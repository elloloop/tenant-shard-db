"""
Unit tests for tenant role-based and status-based access control in
the gRPC server (Issue #56).

These tests focus on the ``_check_tenant_access`` helper and its
integration with ``ExecuteAtomic``:

Roles
-----
* owner / admin / member  -> read + write
* viewer / guest          -> read only (writes rejected)
* non-member              -> all rejected (PERMISSION_DENIED)
* system actors           -> bypass all role and status checks

Tenant status
-------------
* active     -> all operations allowed
* archived   -> reads ok, writes rejected (FAILED_PRECONDITION)
* legal_hold -> reads + creates ok, deletes rejected (FAILED_PRECONDITION)
* deleted    -> all operations rejected (NOT_FOUND)

Backward compatibility
----------------------
* When ``GlobalStore`` is None the helper is a no-op so existing
  deployments without a registry continue to work.
"""

from __future__ import annotations

import tempfile
from unittest.mock import AsyncMock, MagicMock

import grpc
import pytest
from google.protobuf.struct_pb2 import Struct

from dbaas.entdb_server.api.generated import (
    CreateNodeOp,
    DeleteNodeOp,
    ExecuteAtomicRequest,
    Operation,
    RequestContext,
)
from dbaas.entdb_server.api.grpc_server import EntDBServicer
from dbaas.entdb_server.global_store import GlobalStore

# ── Test scaffolding ─────────────────────────────────────────────────


class _AbortError(BaseException):
    """Mimics ``grpc.aio.AbortError`` for the fake context.

    Inherits from ``BaseException`` so it propagates through generic
    ``except Exception`` blocks in the gRPC handler, matching the
    behaviour observed in production with the real gRPC server.
    """


class _FakeContext:
    """Minimal stand-in for ``grpc_aio.ServicerContext``."""

    def __init__(self) -> None:
        self.aborted = False
        self.abort_code: grpc.StatusCode | None = None
        self.abort_message: str | None = None

    async def abort(self, code: grpc.StatusCode, message: str) -> None:
        self.aborted = True
        self.abort_code = code
        self.abort_message = message
        raise _AbortError(f"[{code}] {message}")


def _make_servicer(global_store: GlobalStore | None) -> EntDBServicer:
    """Build an ``EntDBServicer`` with mocked WAL and stores."""
    wal = MagicMock()
    pos = MagicMock()
    pos.__str__ = MagicMock(return_value="0:0:0")
    wal.append = AsyncMock(return_value=pos)

    canonical_store = MagicMock()
    canonical_store.initialize_tenant = AsyncMock()
    canonical_store.wait_for_offset = AsyncMock(return_value=True)

    mailbox_store = MagicMock()

    schema_registry = MagicMock()
    schema_registry.fingerprint = ""  # disable fingerprint check

    return EntDBServicer(
        wal=wal,
        canonical_store=canonical_store,
        mailbox_store=mailbox_store,
        schema_registry=schema_registry,
        global_store=global_store,
    )


def _create_request(
    tenant_id: str,
    actor: str,
    op_type: str = "create",
) -> ExecuteAtomicRequest:
    """Build an ExecuteAtomicRequest containing one create or delete op."""
    if op_type == "delete":
        op = Operation(delete_node=DeleteNodeOp(type_id=1, id="node-1"))
    else:
        op = Operation(create_node=CreateNodeOp(type_id=1, data=Struct()))
    return ExecuteAtomicRequest(
        context=RequestContext(tenant_id=tenant_id, actor=actor),
        idempotency_key="key-1",
        operations=[op],
    )


@pytest.fixture
def global_store():
    with tempfile.TemporaryDirectory() as tmpdir:
        gs = GlobalStore(tmpdir)
        yield gs
        gs.close()


async def _bootstrap_tenant(
    gs: GlobalStore,
    tenant_id: str = "t1",
    status: str = "active",
    members: dict[str, str] | None = None,
) -> None:
    """Create a tenant in the registry with optional members and status."""
    await gs.create_tenant(tenant_id, f"Tenant {tenant_id}")
    if status != "active":
        await gs.set_tenant_status(tenant_id, status)
    if members:
        for user_id, role in members.items():
            await gs.add_member(tenant_id, user_id, role=role)


# ── _check_tenant_access: role enforcement ───────────────────────────


class TestRoleEnforcement:
    """Direct tests of ``_check_tenant_access``."""

    async def test_owner_can_write(self, global_store):
        await _bootstrap_tenant(global_store, members={"alice": "owner"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        role = await servicer._check_tenant_access(
            "t1", "user:alice", ctx, require_write=True, op_kind="write",
        )
        assert role == "owner"
        assert ctx.aborted is False

    async def test_admin_can_write(self, global_store):
        await _bootstrap_tenant(global_store, members={"alice": "admin"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        role = await servicer._check_tenant_access(
            "t1", "user:alice", ctx, require_write=True, op_kind="write",
        )
        assert role == "admin"

    async def test_member_can_write(self, global_store):
        await _bootstrap_tenant(global_store, members={"bob": "member"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        role = await servicer._check_tenant_access(
            "t1", "user:bob", ctx, require_write=True, op_kind="write",
        )
        assert role == "member"

    async def test_viewer_cannot_write(self, global_store):
        await _bootstrap_tenant(global_store, members={"viv": "viewer"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer._check_tenant_access(
                "t1", "user:viv", ctx, require_write=True, op_kind="write",
            )
        assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED

    async def test_viewer_can_read(self, global_store):
        await _bootstrap_tenant(global_store, members={"viv": "viewer"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        role = await servicer._check_tenant_access(
            "t1", "user:viv", ctx, require_write=False, op_kind="read",
        )
        assert role == "viewer"

    async def test_guest_cannot_write(self, global_store):
        await _bootstrap_tenant(global_store, members={"gus": "guest"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer._check_tenant_access(
                "t1", "user:gus", ctx, require_write=True, op_kind="write",
            )
        assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED

    async def test_guest_can_read(self, global_store):
        await _bootstrap_tenant(global_store, members={"gus": "guest"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        role = await servicer._check_tenant_access(
            "t1", "user:gus", ctx, require_write=False, op_kind="read",
        )
        assert role == "guest"

    async def test_non_member_rejected(self, global_store):
        await _bootstrap_tenant(global_store, members={"alice": "owner"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer._check_tenant_access(
                "t1", "user:eve", ctx, require_write=False, op_kind="read",
            )
        assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED

    async def test_non_member_rejected_for_write(self, global_store):
        await _bootstrap_tenant(global_store, members={"alice": "owner"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer._check_tenant_access(
                "t1", "user:eve", ctx, require_write=True, op_kind="write",
            )
        assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED

    async def test_system_prefix_bypasses_role_check(self, global_store):
        await _bootstrap_tenant(global_store, members={"alice": "owner"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        role = await servicer._check_tenant_access(
            "t1", "system:applier", ctx, require_write=True, op_kind="delete",
        )
        assert role == "system"
        assert ctx.aborted is False

    async def test_underscore_system_bypasses(self, global_store):
        await _bootstrap_tenant(global_store, members={"alice": "owner"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        role = await servicer._check_tenant_access(
            "t1", "__system__", ctx, require_write=True, op_kind="delete",
        )
        assert role == "system"


# ── _check_tenant_access: status enforcement ─────────────────────────


class TestStatusEnforcement:
    """Tests for tenant status-based gating."""

    async def test_active_allows_writes(self, global_store):
        await _bootstrap_tenant(global_store, members={"alice": "owner"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        role = await servicer._check_tenant_access(
            "t1", "user:alice", ctx, require_write=True, op_kind="write",
        )
        assert role == "owner"

    async def test_archived_allows_reads(self, global_store):
        await _bootstrap_tenant(
            global_store, status="archived", members={"alice": "owner"},
        )
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        role = await servicer._check_tenant_access(
            "t1", "user:alice", ctx, require_write=False, op_kind="read",
        )
        assert role == "owner"

    async def test_archived_rejects_writes(self, global_store):
        await _bootstrap_tenant(
            global_store, status="archived", members={"alice": "owner"},
        )
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer._check_tenant_access(
                "t1", "user:alice", ctx, require_write=True, op_kind="write",
            )
        assert ctx.abort_code == grpc.StatusCode.FAILED_PRECONDITION

    async def test_archived_rejects_deletes(self, global_store):
        await _bootstrap_tenant(
            global_store, status="archived", members={"alice": "owner"},
        )
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer._check_tenant_access(
                "t1", "user:alice", ctx, require_write=True, op_kind="delete",
            )
        assert ctx.abort_code == grpc.StatusCode.FAILED_PRECONDITION

    async def test_legal_hold_allows_reads(self, global_store):
        await _bootstrap_tenant(
            global_store, status="legal_hold", members={"alice": "owner"},
        )
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        role = await servicer._check_tenant_access(
            "t1", "user:alice", ctx, require_write=False, op_kind="read",
        )
        assert role == "owner"

    async def test_legal_hold_allows_creates(self, global_store):
        await _bootstrap_tenant(
            global_store, status="legal_hold", members={"alice": "owner"},
        )
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        role = await servicer._check_tenant_access(
            "t1", "user:alice", ctx, require_write=True, op_kind="write",
        )
        assert role == "owner"

    async def test_legal_hold_rejects_deletes(self, global_store):
        await _bootstrap_tenant(
            global_store, status="legal_hold", members={"alice": "owner"},
        )
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer._check_tenant_access(
                "t1", "user:alice", ctx, require_write=True, op_kind="delete",
            )
        assert ctx.abort_code == grpc.StatusCode.FAILED_PRECONDITION

    async def test_deleted_rejects_reads(self, global_store):
        await _bootstrap_tenant(
            global_store, status="deleted", members={"alice": "owner"},
        )
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer._check_tenant_access(
                "t1", "user:alice", ctx, require_write=False, op_kind="read",
            )
        assert ctx.abort_code == grpc.StatusCode.NOT_FOUND

    async def test_deleted_rejects_writes(self, global_store):
        await _bootstrap_tenant(
            global_store, status="deleted", members={"alice": "owner"},
        )
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer._check_tenant_access(
                "t1", "user:alice", ctx, require_write=True, op_kind="write",
            )
        assert ctx.abort_code == grpc.StatusCode.NOT_FOUND

    async def test_unknown_tenant_rejected(self, global_store):
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer._check_tenant_access(
                "ghost", "user:alice", ctx, require_write=False, op_kind="read",
            )
        assert ctx.abort_code == grpc.StatusCode.NOT_FOUND


# ── Backward compatibility ───────────────────────────────────────────


class TestBackwardCompatibility:
    """When GlobalStore is unset, role/status checks must be no-ops."""

    async def test_no_global_store_skips_check(self):
        servicer = _make_servicer(global_store=None)
        ctx = _FakeContext()

        role = await servicer._check_tenant_access(
            "any-tenant",
            "user:nobody",
            ctx,
            require_write=True,
            op_kind="delete",
        )
        assert role == "system"
        assert ctx.aborted is False

    async def test_execute_atomic_no_global_store(self):
        servicer = _make_servicer(global_store=None)
        ctx = _FakeContext()
        request = _create_request("t1", "user:nobody", op_type="create")

        response = await servicer.ExecuteAtomic(request, ctx)
        assert response.success is True
        assert ctx.aborted is False


# ── ExecuteAtomic integration ────────────────────────────────────────


class TestExecuteAtomicAccessControl:
    """End-to-end role + status enforcement through ExecuteAtomic."""

    async def test_execute_atomic_owner_create(self, global_store):
        await _bootstrap_tenant(global_store, members={"alice": "owner"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.ExecuteAtomic(
            _create_request("t1", "user:alice", "create"), ctx,
        )
        assert response.success is True

    async def test_execute_atomic_member_create(self, global_store):
        await _bootstrap_tenant(global_store, members={"bob": "member"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.ExecuteAtomic(
            _create_request("t1", "user:bob", "create"), ctx,
        )
        assert response.success is True

    async def test_execute_atomic_viewer_rejected(self, global_store):
        await _bootstrap_tenant(global_store, members={"viv": "viewer"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer.ExecuteAtomic(
                _create_request("t1", "user:viv", "create"), ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED

    async def test_execute_atomic_guest_rejected(self, global_store):
        await _bootstrap_tenant(global_store, members={"gus": "guest"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer.ExecuteAtomic(
                _create_request("t1", "user:gus", "create"), ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED

    async def test_execute_atomic_non_member_rejected(self, global_store):
        await _bootstrap_tenant(global_store, members={"alice": "owner"})
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer.ExecuteAtomic(
                _create_request("t1", "user:eve", "create"), ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED

    async def test_execute_atomic_system_actor(self, global_store):
        # No member rows; system bypass should still allow the call.
        await _bootstrap_tenant(global_store)
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.ExecuteAtomic(
            _create_request("t1", "system:applier", "create"), ctx,
        )
        assert response.success is True

    async def test_execute_atomic_archived_rejects_create(self, global_store):
        await _bootstrap_tenant(
            global_store, status="archived", members={"alice": "owner"},
        )
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer.ExecuteAtomic(
                _create_request("t1", "user:alice", "create"), ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.FAILED_PRECONDITION

    async def test_execute_atomic_legal_hold_allows_create(self, global_store):
        await _bootstrap_tenant(
            global_store, status="legal_hold", members={"alice": "owner"},
        )
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.ExecuteAtomic(
            _create_request("t1", "user:alice", "create"), ctx,
        )
        assert response.success is True

    async def test_execute_atomic_legal_hold_rejects_delete(self, global_store):
        await _bootstrap_tenant(
            global_store, status="legal_hold", members={"alice": "owner"},
        )
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer.ExecuteAtomic(
                _create_request("t1", "user:alice", "delete"), ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.FAILED_PRECONDITION

    async def test_execute_atomic_deleted_tenant_rejected(self, global_store):
        await _bootstrap_tenant(
            global_store, status="deleted", members={"alice": "owner"},
        )
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer.ExecuteAtomic(
                _create_request("t1", "user:alice", "create"), ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.NOT_FOUND
