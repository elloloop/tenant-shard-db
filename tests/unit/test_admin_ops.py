"""
Unit tests for admin operations (Issues #90, #91, #92).

Covers:
- transfer_user_content: ownership transfer at global_store level
- Legal holds: set, remove, query, is_under_legal_hold
- revoke_user_access: membership removal + shared_index cleanup
- Idempotency of all operations
- WAL-first semantics for TransferUserContent and DelegateAccess
  (Bug 3 in PR description): both gRPC handlers must append to the
  WAL rather than writing directly to canonical_store.
"""

from __future__ import annotations

import json
import tempfile
import time
import uuid
from unittest.mock import AsyncMock, MagicMock

import pytest

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


async def _bootstrap_tenant(
    gs: GlobalStore,
    tenant_id: str = TENANT,
    members: dict[str, str] | None = None,
) -> None:
    await gs.create_tenant(tenant_id, f"Tenant {tenant_id}")
    if members:
        for user_id, role in members.items():
            await gs.add_member(tenant_id, user_id, role=role)


# ════════════════════════════════════════════════════════════════════
# Issue #90: transfer_user_content (global_store)
# ════════════════════════════════════════════════════════════════════


class TestTransferUserContentGlobalStore:
    async def test_transfer_creates_membership_for_new_user(self, global_store):
        """Transfer to a user not in the tenant creates membership."""
        await _bootstrap_tenant(global_store, members={"alice": "member"})
        result = await global_store.transfer_user_content(TENANT, ALICE, BOB)
        assert result["membership_created"] is True
        assert await global_store.is_member(TENANT, BOB) is True

    async def test_transfer_does_not_duplicate_existing_member(self, global_store):
        """Transfer to an existing member does not create a new row."""
        await _bootstrap_tenant(global_store, members={ALICE: "member", BOB: "member"})
        result = await global_store.transfer_user_content(TENANT, ALICE, BOB)
        assert result["membership_created"] is False
        members = await global_store.get_members(TENANT)
        bob_rows = [m for m in members if m["user_id"] == BOB]
        assert len(bob_rows) == 1

    async def test_transfer_returns_correct_fields(self, global_store):
        """Result dict contains expected keys and values."""
        await _bootstrap_tenant(global_store, members={"alice": "member"})
        result = await global_store.transfer_user_content(TENANT, ALICE, BOB)
        assert result["tenant_id"] == TENANT
        assert result["from_user"] == ALICE
        assert result["to_user"] == BOB

    async def test_transfer_idempotent(self, global_store):
        """Calling transfer twice for the same pair is safe."""
        await _bootstrap_tenant(global_store, members={"alice": "member"})
        r1 = await global_store.transfer_user_content(TENANT, ALICE, BOB)
        r2 = await global_store.transfer_user_content(TENANT, ALICE, BOB)
        assert r1["membership_created"] is True
        assert r2["membership_created"] is False


# ════════════════════════════════════════════════════════════════════
# Issue #90: transfer_user_content (canonical_store -- ownership)
# ════════════════════════════════════════════════════════════════════


class TestTransferOwnershipCanonicalStore:
    async def test_transfer_changes_owner_actor(self, store):
        """Nodes previously owned by from_user are now owned by to_user."""
        n1 = _create_node(store, owner=ALICE)
        n2 = _create_node(store, owner=ALICE)
        _create_node(store, owner=BOB)  # should remain BOB

        result = await store.transfer_user_content(TENANT, ALICE, BOB, actor=ADMIN)
        assert result["transferred"] == 2

        for nid in (n1, n2):
            node = await store.get_node(TENANT, nid)
            assert node.owner_actor == BOB

    async def test_transfer_zero_when_no_nodes(self, store):
        """Transfer with no owned nodes returns transferred=0."""
        result = await store.transfer_user_content(TENANT, ALICE, BOB, actor=ADMIN)
        assert result["transferred"] == 0


# ════════════════════════════════════════════════════════════════════
# Issue #91: Legal holds
# ════════════════════════════════════════════════════════════════════


class TestLegalHolds:
    async def test_set_legal_hold(self, global_store):
        """Setting a legal hold records it."""
        await _bootstrap_tenant(global_store)
        result = await global_store.set_legal_hold_record(TENANT, "court-xyz", "Litigation pending")
        assert result["tenant_id"] == TENANT
        assert result["held_by"] == "court-xyz"
        assert result["reason"] == "Litigation pending"

    async def test_is_under_legal_hold_true(self, global_store):
        """Tenant with a hold returns True."""
        await _bootstrap_tenant(global_store)
        await global_store.set_legal_hold_record(TENANT, "court-xyz", "Litigation pending")
        assert await global_store.is_under_legal_hold(TENANT) is True

    async def test_is_under_legal_hold_false_when_no_holds(self, global_store):
        """Tenant with no holds returns False."""
        await _bootstrap_tenant(global_store)
        assert await global_store.is_under_legal_hold(TENANT) is False

    async def test_remove_legal_hold(self, global_store):
        """Removing a hold makes is_under_legal_hold return False."""
        await _bootstrap_tenant(global_store)
        await global_store.set_legal_hold_record(TENANT, "court-xyz", "Litigation pending")
        removed = await global_store.remove_legal_hold(TENANT, "court-xyz")
        assert removed is True
        assert await global_store.is_under_legal_hold(TENANT) is False

    async def test_remove_nonexistent_hold_returns_false(self, global_store):
        """Removing a hold that doesn't exist returns False."""
        await _bootstrap_tenant(global_store)
        removed = await global_store.remove_legal_hold(TENANT, "nobody")
        assert removed is False

    async def test_multiple_holds_on_same_tenant(self, global_store):
        """Multiple authorities can hold the same tenant."""
        await _bootstrap_tenant(global_store)
        await global_store.set_legal_hold_record(TENANT, "court-a", "Case A")
        await global_store.set_legal_hold_record(TENANT, "court-b", "Case B")
        holds = await global_store.get_legal_holds(TENANT)
        assert len(holds) == 2
        held_by_set = {h["held_by"] for h in holds}
        assert held_by_set == {"court-a", "court-b"}

    async def test_removing_one_hold_keeps_others(self, global_store):
        """Removing one hold does not affect other holds."""
        await _bootstrap_tenant(global_store)
        await global_store.set_legal_hold_record(TENANT, "court-a", "Case A")
        await global_store.set_legal_hold_record(TENANT, "court-b", "Case B")
        await global_store.remove_legal_hold(TENANT, "court-a")
        assert await global_store.is_under_legal_hold(TENANT) is True
        holds = await global_store.get_legal_holds(TENANT)
        assert len(holds) == 1
        assert holds[0]["held_by"] == "court-b"

    async def test_set_legal_hold_idempotent(self, global_store):
        """Setting the same hold twice does not create duplicates."""
        await _bootstrap_tenant(global_store)
        await global_store.set_legal_hold_record(TENANT, "court-a", "Case A")
        await global_store.set_legal_hold_record(TENANT, "court-a", "Case A")
        holds = await global_store.get_legal_holds(TENANT)
        assert len(holds) == 1

    async def test_get_legal_holds_empty(self, global_store):
        """get_legal_holds returns empty list for tenant with no holds."""
        await _bootstrap_tenant(global_store)
        holds = await global_store.get_legal_holds(TENANT)
        assert holds == []


# ════════════════════════════════════════════════════════════════════
# Issue #92: revoke_user_access (global_store)
# ════════════════════════════════════════════════════════════════════


class TestRevokeUserAccessGlobalStore:
    async def test_revoke_removes_membership(self, global_store):
        """Revoking a user removes them from tenant_members."""
        await _bootstrap_tenant(global_store, members={"bob": "member"})
        result = await global_store.revoke_user_access(TENANT, "bob")
        assert result["membership_removed"] is True
        assert await global_store.is_member(TENANT, "bob") is False

    async def test_revoke_clears_shared_index(self, global_store):
        """Revoking removes shared_index entries for the tenant."""
        await _bootstrap_tenant(global_store, members={"bob": "member"})
        await global_store.add_shared("bob", TENANT, "node-1", "read")
        await global_store.add_shared("bob", TENANT, "node-2", "write")
        await global_store.add_shared("bob", "other-tenant", "node-3", "read")

        result = await global_store.revoke_user_access(TENANT, "bob")
        assert result["shared_removed"] == 2

        remaining = await global_store.get_shared_with_me("bob")
        assert len(remaining) == 1
        assert remaining[0]["source_tenant"] == "other-tenant"

    async def test_revoke_idempotent(self, global_store):
        """Revoking twice is safe -- second call reports nothing removed."""
        await _bootstrap_tenant(global_store, members={"bob": "member"})
        r1 = await global_store.revoke_user_access(TENANT, "bob")
        r2 = await global_store.revoke_user_access(TENANT, "bob")
        assert r1["membership_removed"] is True
        assert r2["membership_removed"] is False
        assert r2["shared_removed"] == 0

    async def test_revoke_nonexistent_user_safe(self, global_store):
        """Revoking a user not in the tenant returns zeros."""
        await _bootstrap_tenant(global_store)
        result = await global_store.revoke_user_access(TENANT, "ghost")
        assert result["membership_removed"] is False
        assert result["shared_removed"] == 0


# ════════════════════════════════════════════════════════════════════
# Issue #92: revoke_user_access (canonical_store -- ACL entries)
# ════════════════════════════════════════════════════════════════════


class TestRevokeUserAccessCanonicalStore:
    async def test_revoke_clears_acl_entries(self, store):
        """All node_access grants for the user are removed."""
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

    async def test_revoke_clears_group_memberships(self, store):
        """Group memberships for the user are removed."""
        await store.add_group_member(TENANT, "group:team", BOB)
        await store.add_group_member(TENANT, "group:friends", BOB)

        result = await store.revoke_user_access(TENANT, BOB, actor=ADMIN)
        assert result["revoked_groups"] == 2


# ════════════════════════════════════════════════════════════════════
# Bug 3: WAL-first wiring for TransferUserContent + DelegateAccess
# ════════════════════════════════════════════════════════════════════
#
# Both gRPC handlers used to bypass the WAL by calling canonical_store
# methods directly. These tests pin the new behavior:
#
# 1. TransferUserContent appends an ``admin_transfer_content`` event.
# 2. DelegateAccess appends an ``admin_delegate_access`` event.
# 3. Neither handler mutates canonical_store directly — the Applier is
#    the only legitimate writer (verified by replacing canonical_store
#    with a guard that records write attempts).


class _RecordingCanonicalStore(CanonicalStore):
    """CanonicalStore subclass that records mutating method calls.

    Used to assert that the gRPC admin handlers do *not* call write
    methods on the store directly — all mutations must go through the
    WAL (and thence through the Applier).
    """

    def __init__(self, data_dir):
        super().__init__(data_dir=data_dir)
        self.write_calls: list[str] = []

    async def transfer_user_content(self, *args, **kwargs):  # type: ignore[override]
        self.write_calls.append("transfer_user_content")
        return {"transferred": 0}

    async def delegate_access(self, *args, **kwargs):  # type: ignore[override]
        self.write_calls.append("delegate_access")
        return {"delegated": 0, "expires_at": None}


async def _setup_servicer_with_wal_spy(tmp_path, gs_dir):
    from dbaas.entdb_server.api.grpc_server import EntDBServicer

    gs = GlobalStore(gs_dir)
    await gs.create_tenant(TENANT, "T")
    await gs.add_member(TENANT, "admin", role="owner")

    cs = _RecordingCanonicalStore(data_dir=tmp_path)
    with cs._get_connection(TENANT, create=True) as conn:
        cs._create_schema(conn)
    # Seed two nodes owned by alice so the count-by-query in the
    # handler returns a non-zero number.
    nid_a = str(uuid.uuid4())
    nid_b = str(uuid.uuid4())
    now = int(time.time() * 1000)
    cs._sync_create_node(TENANT, 1, {}, ALICE, nid_a, [], now)
    cs._sync_create_node(TENANT, 1, {}, ALICE, nid_b, [], now)

    wal = MagicMock()
    pos = MagicMock()
    pos.__str__ = MagicMock(return_value="0:0:0")
    wal.append = AsyncMock(return_value=pos)

    schema_registry = MagicMock()
    schema_registry.fingerprint = ""

    servicer = EntDBServicer(
        wal=wal,
        canonical_store=cs,
        schema_registry=schema_registry,
        global_store=gs,
    )
    return servicer, cs, wal, gs


class _FakeContext:
    async def abort(self, code, message):
        raise AssertionError(f"unexpected abort: {code} {message}")


class TestAdminHandlersGoThroughWal:
    async def test_transfer_user_content_appends_admin_transfer_event(self, tmp_path):
        from dbaas.entdb_server.api.generated import TransferUserContentRequest

        with tempfile.TemporaryDirectory() as gs_dir:
            servicer, cs, wal, gs = await _setup_servicer_with_wal_spy(tmp_path, gs_dir)
            try:
                resp = await servicer.TransferUserContent(
                    TransferUserContentRequest(
                        actor=ADMIN,
                        tenant_id=TENANT,
                        from_user=ALICE,
                        to_user=BOB,
                    ),
                    _FakeContext(),
                )
                assert resp.success is True
                # The handler must have appended exactly one WAL event
                # whose op type is admin_transfer_content.
                wal.append.assert_awaited_once()
                payload_bytes = wal.append.call_args.kwargs["value"]
                event = json.loads(payload_bytes.decode())
                assert event["ops"][0]["op"] == "admin_transfer_content"
                assert event["ops"][0]["from_user"] == ALICE
                assert event["ops"][0]["to_user"] == BOB
                # And the handler must NOT have mutated canonical_store
                # directly — the WAL is the only source of truth.
                assert "transfer_user_content" not in cs.write_calls
                # The response counts nodes still owned by from_user
                # at the time of the call (the Applier will materialize
                # the rename later).
                assert resp.transferred == 2
            finally:
                gs.close()

    async def test_delegate_access_appends_admin_delegate_event(self, tmp_path):
        from dbaas.entdb_server.api.generated import DelegateAccessRequest

        with tempfile.TemporaryDirectory() as gs_dir:
            servicer, cs, wal, gs = await _setup_servicer_with_wal_spy(tmp_path, gs_dir)
            try:
                expires = int(time.time() * 1000) + 3600_000
                resp = await servicer.DelegateAccess(
                    DelegateAccessRequest(
                        actor=ADMIN,
                        tenant_id=TENANT,
                        from_user=ALICE,
                        to_user=BOB,
                        permission="write",
                        expires_at=expires,
                    ),
                    _FakeContext(),
                )
                assert resp.success is True
                wal.append.assert_awaited_once()
                payload_bytes = wal.append.call_args.kwargs["value"]
                event = json.loads(payload_bytes.decode())
                op = event["ops"][0]
                assert op["op"] == "admin_delegate_access"
                assert op["from_user"] == ALICE
                assert op["to_user"] == BOB
                assert op["permission"] == "write"
                assert op["expires_at"] == expires
                # Direct delegate_access must NOT have been called.
                assert "delegate_access" not in cs.write_calls
                assert resp.delegated == 2
                assert resp.expires_at == expires
            finally:
                gs.close()

    async def test_delegate_access_default_permission_is_read(self, tmp_path):
        from dbaas.entdb_server.api.generated import DelegateAccessRequest

        with tempfile.TemporaryDirectory() as gs_dir:
            servicer, cs, wal, gs = await _setup_servicer_with_wal_spy(tmp_path, gs_dir)
            try:
                resp = await servicer.DelegateAccess(
                    DelegateAccessRequest(
                        actor=ADMIN,
                        tenant_id=TENANT,
                        from_user=ALICE,
                        to_user=BOB,
                        # permission omitted
                    ),
                    _FakeContext(),
                )
                assert resp.success is True
                wal.append.assert_awaited_once()
                event = json.loads(wal.append.call_args.kwargs["value"].decode())
                assert event["ops"][0]["permission"] == "read"
            finally:
                gs.close()
