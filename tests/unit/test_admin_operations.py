"""
Unit tests for admin operations (Issue #90, ADR-003).

Covers the four admin operations:

* ``transfer_user_content`` — reassign ownership of all nodes from
  one user to another (employee offboarding).
* ``delegate_access`` — grant another user temporary access to all
  of from_user's nodes.
* ``set_legal_hold`` — toggle a tenant between ``active`` and
  ``legal_hold``. Legal hold blocks deletes at the gRPC layer.
* ``revoke_all_user_access`` (previously ``revoke_user_access``) —
  remove all node_access grants, group memberships, and shared_index
  entries for a user in a tenant.

Each operation is exercised at two layers:

1. ``CanonicalStore`` / ``GlobalStore`` directly — verifying the
   SQL side effects and audit log writes.
2. ``EntDBServicer`` handlers — verifying request validation,
   admin-only enforcement, and error propagation.

Plus an end-to-end test that asserts ``legal_hold`` blocks a
``DeleteNodeOp`` through ``ExecuteAtomic``.
"""

from __future__ import annotations

import json
import tempfile
import time
import uuid
from unittest.mock import AsyncMock, MagicMock

import grpc
import pytest
from google.protobuf.struct_pb2 import Struct

from dbaas.entdb_server.api.generated import (
    CreateNodeOp,
    DelegateAccessRequest,
    DeleteNodeOp,
    ExecuteAtomicRequest,
    LegalHoldRequest,
    Operation,
    RequestContext,
    RevokeAllUserAccessRequest,
    TransferUserContentRequest,
)
from dbaas.entdb_server.api.grpc_server import EntDBServicer
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.global_store import GlobalStore

TENANT = "tenant-admin-ops"
ALICE = "user:alice"
BOB = "user:bob"
CAROL = "user:carol"
ADMIN = "user:admin"


# ── Fixtures ────────────────────────────────────────────────────────


@pytest.fixture
def store(tmp_path):
    s = CanonicalStore(data_dir=tmp_path)
    # Initialize tenant DB synchronously.
    with s._get_connection(TENANT, create=True) as conn:
        s._create_schema(conn)
    return s


@pytest.fixture
def global_store():
    with tempfile.TemporaryDirectory() as tmpdir:
        gs = GlobalStore(tmpdir)
        yield gs
        gs.close()


# ── Helpers ─────────────────────────────────────────────────────────


def _create_node(store, owner=ALICE, tenant_id=TENANT, acl=None):
    nid = str(uuid.uuid4())
    now = int(time.time() * 1000)
    node = store._sync_create_node(
        tenant_id,
        1,  # type_id
        {"title": "test"},
        owner,
        nid,
        acl or [],
        now,
    )
    return node.node_id


class _AbortError(BaseException):
    pass


class _FakeContext:
    def __init__(self) -> None:
        self.aborted = False
        self.abort_code: grpc.StatusCode | None = None
        self.abort_message: str | None = None

    async def abort(self, code, message):
        self.aborted = True
        self.abort_code = code
        self.abort_message = message
        raise _AbortError(f"[{code}] {message}")


def _make_servicer(canonical_store, global_store=None):
    wal = MagicMock()
    pos = MagicMock()
    pos.__str__ = MagicMock(return_value="0:0:0")
    wal.append = AsyncMock(return_value=pos)

    schema_registry = MagicMock()
    schema_registry.fingerprint = ""

    return EntDBServicer(
        wal=wal,
        canonical_store=canonical_store,
        schema_registry=schema_registry,
        global_store=global_store,
    )


async def _bootstrap_tenant(
    gs: GlobalStore,
    tenant_id: str = TENANT,
    status: str = "active",
    members: dict[str, str] | None = None,
) -> None:
    await gs.create_tenant(tenant_id, f"Tenant {tenant_id}")
    if status != "active":
        await gs.set_tenant_status(tenant_id, status)
    if members:
        for user_id, role in members.items():
            await gs.add_member(tenant_id, user_id, role=role)


# ════════════════════════════════════════════════════════════════════
# CanonicalStore: transfer_user_content
# ════════════════════════════════════════════════════════════════════


class TestTransferUserContent:
    async def test_transfers_all_alice_nodes(self, store):
        n1 = _create_node(store, owner=ALICE)
        n2 = _create_node(store, owner=ALICE)
        n3 = _create_node(store, owner=BOB)  # unrelated

        result = await store.transfer_user_content(
            TENANT,
            ALICE,
            BOB,
            actor=ADMIN,
        )

        assert result["transferred"] == 2
        assert result["tenant_id"] == TENANT
        assert result["from"] == ALICE
        assert result["to"] == BOB

        for nid in (n1, n2, n3):
            node = await store.get_node(TENANT, nid)
            assert node is not None
            assert node.owner_actor == BOB

    async def test_returns_zero_when_no_nodes(self, store):
        result = await store.transfer_user_content(
            TENANT,
            ALICE,
            BOB,
            actor=ADMIN,
        )
        assert result["transferred"] == 0

    async def test_audit_log_records_transfer(self, store):
        _create_node(store, owner=ALICE)
        _create_node(store, owner=ALICE)
        await store.transfer_user_content(
            TENANT,
            ALICE,
            BOB,
            actor=ADMIN,
        )
        entries = await store.get_audit_log(TENANT, limit=10)
        matches = [e for e in entries if e["action"] == "transfer_content"]
        assert len(matches) == 1
        meta = json.loads(matches[0]["metadata"])
        assert meta["from_user"] == ALICE
        assert meta["to_user"] == BOB
        assert meta["transferred"] == 2
        assert matches[0]["actor_id"] == ADMIN

    async def test_visibility_index_updated(self, store):
        nid = _create_node(store, owner=ALICE)
        await store.transfer_user_content(
            TENANT,
            ALICE,
            BOB,
            actor=ADMIN,
        )
        # BOB should now have visibility on the transferred node.
        accessible = await store.can_access(TENANT, nid, [BOB])
        assert accessible is True


# ════════════════════════════════════════════════════════════════════
# CanonicalStore: delegate_access
# ════════════════════════════════════════════════════════════════════


class TestDelegateAccess:
    async def test_grants_read_on_all_nodes(self, store):
        n1 = _create_node(store, owner=ALICE)
        n2 = _create_node(store, owner=ALICE)
        _create_node(store, owner=CAROL)  # not alice's

        expires = int(time.time() * 1000) + 3600_000
        result = await store.delegate_access(
            TENANT,
            ALICE,
            BOB,
            "read",
            expires,
            actor=ADMIN,
        )

        assert result["delegated"] == 2
        assert result["expires_at"] == expires

        # BOB should now see alice's nodes via shared-with-me
        shared = await store.list_shared_with_me(TENANT, [BOB])
        shared_ids = {n.node_id for n in shared}
        assert n1 in shared_ids
        assert n2 in shared_ids

    async def test_expires_at_is_stored(self, store):
        nid = _create_node(store, owner=ALICE)
        expires = int(time.time() * 1000) + 60_000
        await store.delegate_access(
            TENANT,
            ALICE,
            BOB,
            "read",
            expires,
            actor=ADMIN,
        )
        with store._get_connection(TENANT) as conn:
            row = conn.execute(
                "SELECT expires_at, permission FROM node_access WHERE node_id = ? AND actor_id = ?",
                (nid, BOB),
            ).fetchone()
        assert row is not None
        assert row["expires_at"] == expires
        assert row["permission"] == "read"

    async def test_permanent_delegation_has_null_expiry(self, store):
        nid = _create_node(store, owner=ALICE)
        await store.delegate_access(
            TENANT,
            ALICE,
            BOB,
            "read",
            None,
            actor=ADMIN,
        )
        with store._get_connection(TENANT) as conn:
            row = conn.execute(
                "SELECT expires_at FROM node_access WHERE node_id = ? AND actor_id = ?",
                (nid, BOB),
            ).fetchone()
        assert row["expires_at"] is None

    async def test_audit_logged(self, store):
        _create_node(store, owner=ALICE)
        await store.delegate_access(
            TENANT,
            ALICE,
            BOB,
            "write",
            None,
            actor=ADMIN,
        )
        entries = await store.get_audit_log(TENANT, limit=10)
        matches = [e for e in entries if e["action"] == "delegate_access"]
        assert len(matches) == 1
        meta = json.loads(matches[0]["metadata"])
        assert meta["from_user"] == ALICE
        assert meta["to_user"] == BOB
        assert meta["permission"] == "write"
        assert meta["delegated"] == 1

    async def test_expired_grants_are_filtered(self, store):
        _create_node(store, owner=ALICE)
        past = int(time.time() * 1000) - 10_000
        await store.delegate_access(
            TENANT,
            ALICE,
            BOB,
            "read",
            past,
            actor=ADMIN,
        )
        shared = await store.list_shared_with_me(TENANT, [BOB])
        assert shared == []


# ════════════════════════════════════════════════════════════════════
# GlobalStore: set_legal_hold
# ════════════════════════════════════════════════════════════════════


class TestSetLegalHold:
    async def test_enables_legal_hold(self, global_store):
        await _bootstrap_tenant(global_store)
        updated = await global_store.set_legal_hold(TENANT, True, actor=ADMIN)
        assert updated is True
        tenant = await global_store.get_tenant(TENANT)
        assert tenant["status"] == "legal_hold"

    async def test_disables_legal_hold(self, global_store):
        await _bootstrap_tenant(global_store, status="legal_hold")
        updated = await global_store.set_legal_hold(TENANT, False, actor=ADMIN)
        assert updated is True
        tenant = await global_store.get_tenant(TENANT)
        assert tenant["status"] == "active"

    async def test_returns_false_for_unknown_tenant(self, global_store):
        updated = await global_store.set_legal_hold(
            "ghost",
            True,
            actor=ADMIN,
        )
        assert updated is False


# ════════════════════════════════════════════════════════════════════
# CanonicalStore: revoke_user_access
# ════════════════════════════════════════════════════════════════════


class TestRevokeUserAccess:
    async def test_removes_node_access_grants(self, store):
        n1 = _create_node(store, owner=ALICE)
        n2 = _create_node(store, owner=ALICE)
        await store.share_node(TENANT, n1, BOB, "read", ALICE)
        await store.share_node(TENANT, n2, BOB, "write", ALICE)

        result = await store.revoke_user_access(TENANT, BOB, actor=ADMIN)
        assert result["revoked_grants"] == 2

        with store._get_connection(TENANT) as conn:
            rows = conn.execute(
                "SELECT 1 FROM node_access WHERE actor_id = ?",
                (BOB,),
            ).fetchall()
        assert rows == []

    async def test_removes_group_memberships(self, store):
        await store.add_group_member(TENANT, "group:friends", BOB)
        await store.add_group_member(TENANT, "group:team", BOB)
        await store.add_group_member(TENANT, "group:team", CAROL)

        result = await store.revoke_user_access(TENANT, BOB, actor=ADMIN)
        assert result["revoked_groups"] == 2

        with store._get_connection(TENANT) as conn:
            rows = conn.execute(
                "SELECT 1 FROM group_users WHERE member_actor_id = ?",
                (BOB,),
            ).fetchall()
        assert rows == []
        # CAROL's membership untouched
        carol_rows = conn = store._get_connection(TENANT)
        with carol_rows as conn:
            remain = conn.execute(
                "SELECT member_actor_id FROM group_users WHERE group_id = 'group:team'",
            ).fetchall()
        assert any(r[0] == CAROL for r in remain)

    async def test_removes_visibility_index(self, store):
        n1 = _create_node(store, owner=ALICE)
        await store.share_node(TENANT, n1, BOB, "read", ALICE)
        await store.revoke_user_access(TENANT, BOB, actor=ADMIN)

        with store._get_connection(TENANT) as conn:
            rows = conn.execute(
                "SELECT 1 FROM node_visibility WHERE tenant_id = ? AND principal = ?",
                (TENANT, BOB),
            ).fetchall()
        assert rows == []

    async def test_audit_logged(self, store):
        await store.revoke_user_access(TENANT, BOB, actor=ADMIN)
        entries = await store.get_audit_log(TENANT, limit=10)
        matches = [e for e in entries if e["action"] == "revoke_user_access"]
        assert len(matches) == 1
        meta = json.loads(matches[0]["metadata"])
        assert meta["user_id"] == BOB
        assert matches[0]["actor_id"] == ADMIN

    async def test_returns_zero_counts_when_nothing_to_revoke(self, store):
        result = await store.revoke_user_access(
            TENANT,
            "user:ghost",
            actor=ADMIN,
        )
        assert result["revoked_grants"] == 0
        assert result["revoked_groups"] == 0


# ════════════════════════════════════════════════════════════════════
# gRPC handler: TransferUserContent
# ════════════════════════════════════════════════════════════════════


class TestTransferUserContentHandler:
    async def test_owner_can_transfer(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            members={"admin": "owner"},
        )
        _create_node(store, owner=ALICE)
        _create_node(store, owner=ALICE)

        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        resp = await servicer.TransferUserContent(
            TransferUserContentRequest(
                actor=ADMIN,
                tenant_id=TENANT,
                from_user=ALICE,
                to_user=BOB,
            ),
            ctx,
        )
        assert resp.success is True
        assert resp.transferred == 2

    async def test_member_cannot_transfer(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            members={"bob": "member"},
        )
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer.TransferUserContent(
                TransferUserContentRequest(
                    actor=BOB,
                    tenant_id=TENANT,
                    from_user=ALICE,
                    to_user=CAROL,
                ),
                ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED

    async def test_viewer_cannot_transfer(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            members={"bob": "viewer"},
        )
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer.TransferUserContent(
                TransferUserContentRequest(
                    actor=BOB,
                    tenant_id=TENANT,
                    from_user=ALICE,
                    to_user=CAROL,
                ),
                ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED

    async def test_system_actor_can_transfer(self, store, global_store):
        await _bootstrap_tenant(global_store)
        _create_node(store, owner=ALICE)
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()

        resp = await servicer.TransferUserContent(
            TransferUserContentRequest(
                actor="system:admin",
                tenant_id=TENANT,
                from_user=ALICE,
                to_user=BOB,
            ),
            ctx,
        )
        assert resp.success is True
        assert resp.transferred == 1


# ════════════════════════════════════════════════════════════════════
# gRPC handler: DelegateAccess
# ════════════════════════════════════════════════════════════════════


class TestDelegateAccessHandler:
    async def test_admin_can_delegate(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            members={"admin": "admin"},
        )
        _create_node(store, owner=ALICE)
        _create_node(store, owner=ALICE)

        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        resp = await servicer.DelegateAccess(
            DelegateAccessRequest(
                actor=ADMIN,
                tenant_id=TENANT,
                from_user=ALICE,
                to_user=BOB,
                permission="read",
                expires_at=int(time.time() * 1000) + 3600_000,
            ),
            ctx,
        )
        assert resp.success is True
        assert resp.delegated == 2

    async def test_non_admin_cannot_delegate(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            members={"bob": "member"},
        )
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer.DelegateAccess(
                DelegateAccessRequest(
                    actor=BOB,
                    tenant_id=TENANT,
                    from_user=ALICE,
                    to_user=CAROL,
                    permission="read",
                ),
                ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED

    async def test_default_permission_is_read(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            members={"admin": "owner"},
        )
        _create_node(store, owner=ALICE)
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        await servicer.DelegateAccess(
            DelegateAccessRequest(
                actor=ADMIN,
                tenant_id=TENANT,
                from_user=ALICE,
                to_user=BOB,
                # no permission set
            ),
            ctx,
        )
        # The handler appends an admin_delegate_access WAL event with
        # ``permission`` defaulted to ``"read"``. The Applier (not the
        # handler) is responsible for materializing the grant into
        # SQLite, so we verify the WAL event content rather than the
        # canonical store row — the wal in this test is a mock.
        servicer.wal.append.assert_awaited()
        call_args = servicer.wal.append.call_args
        payload = json.loads(call_args.kwargs["value"].decode())
        assert payload["ops"][0]["op"] == "admin_delegate_access"
        assert payload["ops"][0]["permission"] == "read"


# ════════════════════════════════════════════════════════════════════
# gRPC handler: SetLegalHold
# ════════════════════════════════════════════════════════════════════


class TestSetLegalHoldHandler:
    async def test_owner_can_enable_hold(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            members={"admin": "owner"},
        )
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        resp = await servicer.SetLegalHold(
            LegalHoldRequest(actor=ADMIN, tenant_id=TENANT, enabled=True),
            ctx,
        )
        assert resp.success is True
        assert resp.status == "legal_hold"
        tenant = await global_store.get_tenant(TENANT)
        assert tenant["status"] == "legal_hold"

    async def test_disable_hold_returns_to_active(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            status="legal_hold",
            members={"admin": "owner"},
        )
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        resp = await servicer.SetLegalHold(
            LegalHoldRequest(actor=ADMIN, tenant_id=TENANT, enabled=False),
            ctx,
        )
        assert resp.success is True
        assert resp.status == "active"

    async def test_member_cannot_set_hold(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            members={"bob": "member"},
        )
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        with pytest.raises(_AbortError):
            await servicer.SetLegalHold(
                LegalHoldRequest(actor=BOB, tenant_id=TENANT, enabled=True),
                ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED

    async def test_set_hold_appends_audit_entry(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            members={"admin": "owner"},
        )
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        await servicer.SetLegalHold(
            LegalHoldRequest(actor=ADMIN, tenant_id=TENANT, enabled=True),
            ctx,
        )
        entries = await store.get_audit_log(TENANT, limit=10)
        matches = [e for e in entries if e["action"] == "set_legal_hold"]
        assert len(matches) == 1
        meta = json.loads(matches[0]["metadata"])
        assert meta["enabled"] is True
        assert meta["status"] == "legal_hold"


# ════════════════════════════════════════════════════════════════════
# legal_hold blocks deletes (end-to-end through ExecuteAtomic)
# ════════════════════════════════════════════════════════════════════


class TestLegalHoldBlocksDeletes:
    async def test_legal_hold_rejects_delete_node(self, store, global_store):
        """Verify _check_tenant_access already rejects deletes under legal_hold."""
        await _bootstrap_tenant(
            global_store,
            status="legal_hold",
            members={"alice": "owner"},
        )
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()

        request = ExecuteAtomicRequest(
            context=RequestContext(tenant_id=TENANT, actor=ALICE),
            idempotency_key="k-delete",
            operations=[
                Operation(delete_node=DeleteNodeOp(type_id=1, id="node-x")),
            ],
        )
        with pytest.raises(_AbortError):
            await servicer.ExecuteAtomic(request, ctx)
        assert ctx.abort_code == grpc.StatusCode.FAILED_PRECONDITION

    async def test_legal_hold_allows_create_node(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            status="legal_hold",
            members={"alice": "owner"},
        )
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()

        request = ExecuteAtomicRequest(
            context=RequestContext(tenant_id=TENANT, actor=ALICE),
            idempotency_key="k-create",
            operations=[
                Operation(create_node=CreateNodeOp(type_id=1, data=Struct())),
            ],
        )
        resp = await servicer.ExecuteAtomic(request, ctx)
        assert resp.success is True


# ════════════════════════════════════════════════════════════════════
# gRPC handler: RevokeAllUserAccess
# ════════════════════════════════════════════════════════════════════


class TestRevokeAllUserAccessHandler:
    async def test_admin_can_revoke(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            members={"admin": "admin"},
        )
        nid = _create_node(store, owner=ALICE)
        await store.share_node(TENANT, nid, BOB, "read", ALICE)
        await store.add_group_member(TENANT, "group:x", BOB)

        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        resp = await servicer.RevokeAllUserAccess(
            RevokeAllUserAccessRequest(
                actor=ADMIN,
                tenant_id=TENANT,
                user_id=BOB,
            ),
            ctx,
        )
        assert resp.success is True
        assert resp.revoked_grants == 1
        assert resp.revoked_groups == 1

    async def test_non_admin_cannot_revoke(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            members={"bob": "member"},
        )
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        with pytest.raises(_AbortError):
            await servicer.RevokeAllUserAccess(
                RevokeAllUserAccessRequest(
                    actor=BOB,
                    tenant_id=TENANT,
                    user_id=CAROL,
                ),
                ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED

    async def test_cleans_up_shared_index(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            members={"admin": "owner"},
        )
        # Seed shared_index directly so we can verify cleanup.
        await global_store.add_shared(BOB, TENANT, "node-1", "read")
        await global_store.add_shared(BOB, TENANT, "node-2", "read")
        await global_store.add_shared(BOB, "other-tenant", "node-3", "read")

        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        resp = await servicer.RevokeAllUserAccess(
            RevokeAllUserAccessRequest(
                actor=ADMIN,
                tenant_id=TENANT,
                user_id=BOB,
            ),
            ctx,
        )
        assert resp.success is True
        assert resp.revoked_shared == 2

        # Only the other-tenant entry should remain.
        remaining = await global_store.get_shared_with_me(BOB)
        assert len(remaining) == 1
        assert remaining[0]["source_tenant"] == "other-tenant"

    async def test_missing_user_id_rejected(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            members={"admin": "owner"},
        )
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        with pytest.raises(_AbortError):
            await servicer.RevokeAllUserAccess(
                RevokeAllUserAccessRequest(
                    actor=ADMIN,
                    tenant_id=TENANT,
                    user_id="",
                ),
                ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.INVALID_ARGUMENT


# ════════════════════════════════════════════════════════════════════
# Permissions: missing actor and input validation
# ════════════════════════════════════════════════════════════════════


class TestPermissionsAndValidation:
    async def test_transfer_requires_actor(self, store, global_store):
        await _bootstrap_tenant(global_store)
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        with pytest.raises(_AbortError):
            await servicer.TransferUserContent(
                TransferUserContentRequest(
                    actor="",
                    tenant_id=TENANT,
                    from_user=ALICE,
                    to_user=BOB,
                ),
                ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.INVALID_ARGUMENT

    async def test_transfer_requires_from_user(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            members={"admin": "owner"},
        )
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        with pytest.raises(_AbortError):
            await servicer.TransferUserContent(
                TransferUserContentRequest(
                    actor=ADMIN,
                    tenant_id=TENANT,
                    from_user="",
                    to_user=BOB,
                ),
                ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.INVALID_ARGUMENT

    async def test_delegate_requires_to_user(self, store, global_store):
        await _bootstrap_tenant(
            global_store,
            members={"admin": "owner"},
        )
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        with pytest.raises(_AbortError):
            await servicer.DelegateAccess(
                DelegateAccessRequest(
                    actor=ADMIN,
                    tenant_id=TENANT,
                    from_user=ALICE,
                    to_user="",
                ),
                ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.INVALID_ARGUMENT

    async def test_legal_hold_requires_tenant_id(self, store, global_store):
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        with pytest.raises(_AbortError):
            await servicer.SetLegalHold(
                LegalHoldRequest(actor=ADMIN, tenant_id="", enabled=True),
                ctx,
            )
        assert ctx.abort_code == grpc.StatusCode.INVALID_ARGUMENT
