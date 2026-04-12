"""
Unit tests for the GDPR engine (Issue #103, ADR-004).

These tests exercise the anonymization routine, export routine,
deletion queue, deletion worker, and server handlers.

Layers covered:
    - ``CanonicalStore.anonymize_user_in_tenant``
    - ``CanonicalStore.export_user_data_for_tenant``
    - ``CanonicalStore.delete_tenant_database``
    - ``GlobalStore`` deletion queue helpers
    - ``GdprDeletionWorker.run_once``
    - ``EntDBServicer`` handlers: DeleteUser / ExportUserData /
      FreezeUser / CancelUserDeletion

Every data_policy path is exercised and every edge on_subject_exit
direction is asserted.
"""

from __future__ import annotations

import json
import time
import uuid
from unittest.mock import AsyncMock, MagicMock

import grpc
import pytest

from dbaas.entdb_server.api.generated import (
    CancelUserDeletionRequest,
    DeleteUserRequest,
    ExportUserDataRequest,
    FreezeUserRequest,
)
from dbaas.entdb_server.api.grpc_server import EntDBServicer
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.data_policy import DataPolicy
from dbaas.entdb_server.gdpr_worker import GdprDeletionWorker
from dbaas.entdb_server.global_store import GlobalStore
from dbaas.entdb_server.schema import (
    EdgeTypeDef,
    NodeTypeDef,
    OnSubjectExit,
    SchemaRegistry,
    field,
)

TENANT = "tenant-gdpr"
ALICE = "user:alice"
BOB = "user:bob"
ADMIN = "user:admin"


# ── Schema fixtures ────────────────────────────────────────────────


def build_registry() -> SchemaRegistry:
    """Registry covering every data_policy branch and every
    on_subject_exit direction."""
    reg = SchemaRegistry()

    reg.register_node_type(
        NodeTypeDef(
            type_id=10,
            name="JournalEntry",  # PERSONAL
            fields=(
                field(1, "title", "str", pii=False),
                field(2, "body", "str", pii=True),
            ),
            data_policy=DataPolicy.PERSONAL,
        )
    )
    reg.register_node_type(
        NodeTypeDef(
            type_id=11,
            name="Task",  # BUSINESS
            fields=(
                field(1, "title", "str", pii=False),
                field(2, "assignee_email", "str", pii=True),
                field(3, "priority", "int"),
            ),
            data_policy=DataPolicy.BUSINESS,
        )
    )
    reg.register_node_type(
        NodeTypeDef(
            type_id=12,
            name="Invoice",  # FINANCIAL
            fields=(
                field(1, "amount", "int"),
                field(2, "customer_email", "str", pii=True),
            ),
            data_policy=DataPolicy.FINANCIAL,
            legal_basis="Companies Act 2006 s.386",
        )
    )
    reg.register_node_type(
        NodeTypeDef(
            type_id=13,
            name="AuditEntry",  # AUDIT
            fields=(
                field(1, "action", "str", pii=False),
                field(2, "actor_email", "str", pii=True),
            ),
            data_policy=DataPolicy.AUDIT,
            legal_basis="SOX 404",
        )
    )
    reg.register_node_type(
        NodeTypeDef(
            type_id=14,
            name="SessionToken",  # EPHEMERAL
            fields=(field(1, "token", "str", pii=True),),
            data_policy=DataPolicy.EPHEMERAL,
        )
    )
    reg.register_node_type(
        NodeTypeDef(
            type_id=15,
            name="PerformanceReview",  # BUSINESS + subject_field
            fields=(
                field(1, "employee_id", "str", pii=True),
                field(2, "score", "int"),
            ),
            data_policy=DataPolicy.BUSINESS,
            subject_field="employee_id",
        )
    )
    reg.register_node_type(
        NodeTypeDef(
            type_id=20,
            name="User",
            fields=(field(1, "email", "str", pii=True),),
            data_policy=DataPolicy.PERSONAL,
        )
    )

    reg.register_edge_type(
        EdgeTypeDef(
            edge_id=100,
            name="AssignedTo",
            from_type=11,
            to_type=20,
            data_policy=DataPolicy.BUSINESS,
            on_subject_exit=OnSubjectExit.BOTH,
        )
    )
    reg.register_edge_type(
        EdgeTypeDef(
            edge_id=101,
            name="FollowsUser",
            from_type=20,
            to_type=20,
            data_policy=DataPolicy.PERSONAL,
            on_subject_exit=OnSubjectExit.FROM,
        )
    )
    reg.register_edge_type(
        EdgeTypeDef(
            edge_id=102,
            name="MentionsUser",
            from_type=11,
            to_type=20,
            data_policy=DataPolicy.PERSONAL,
            on_subject_exit=OnSubjectExit.TO,
        )
    )
    reg.freeze()
    return reg


# ── Fixtures ───────────────────────────────────────────────────────


@pytest.fixture
def store(tmp_path):
    s = CanonicalStore(data_dir=tmp_path)
    with s._get_connection(TENANT, create=True) as conn:
        s._create_schema(conn)
    return s


@pytest.fixture
def registry():
    return build_registry()


@pytest.fixture
def global_store(tmp_path):
    gs = GlobalStore(str(tmp_path / "gs"))
    yield gs
    gs.close()


# ── Helpers ────────────────────────────────────────────────────────


def _mk_node(store, type_id, payload, owner, tenant=TENANT):
    node_id = str(uuid.uuid4())
    node = store._sync_create_node(
        tenant, type_id, payload, owner, node_id, [], int(time.time() * 1000)
    )
    return node.node_id


def _mk_edge(store, edge_type_id, from_id, to_id, props=None, tenant=TENANT):
    with store._get_connection(tenant) as conn:
        conn.execute(
            "INSERT INTO edges (tenant_id, edge_type_id, from_node_id, to_node_id, "
            "props_json, propagates_acl, created_at) VALUES (?, ?, ?, ?, ?, 0, ?)",
            (
                tenant,
                edge_type_id,
                from_id,
                to_id,
                json.dumps(props or {}),
                int(time.time() * 1000),
            ),
        )


def _get_node(store, node_id, tenant=TENANT):
    with store._get_connection(tenant) as conn:
        row = conn.execute(
            "SELECT * FROM nodes WHERE tenant_id = ? AND node_id = ?",
            (tenant, node_id),
        ).fetchone()
        return dict(row) if row else None


def _list_edges(store, tenant=TENANT):
    with store._get_connection(tenant) as conn:
        rows = conn.execute("SELECT * FROM edges WHERE tenant_id = ?", (tenant,)).fetchall()
        return [dict(r) for r in rows]


# ── anon_id helper ─────────────────────────────────────────────────


def test_anon_id_is_deterministic():
    a = CanonicalStore.anon_id(ALICE, "salt1")
    b = CanonicalStore.anon_id(ALICE, "salt1")
    assert a == b
    assert a.startswith("user:anon-")
    assert len(a) == len("user:anon-") + 12


def test_anon_id_differs_across_salts():
    a = CanonicalStore.anon_id(ALICE, "salt1")
    b = CanonicalStore.anon_id(ALICE, "salt2")
    assert a != b


def test_anon_id_differs_across_users():
    a = CanonicalStore.anon_id(ALICE, "salt1")
    b = CanonicalStore.anon_id(BOB, "salt1")
    assert a != b


# ── Anonymization: node policies ───────────────────────────────────


def test_personal_nodes_are_deleted(store, registry):
    nid = _mk_node(store, 10, {"title": "secret", "body": "very personal"}, ALICE)
    result = store._sync_anonymize_user_in_tenant(TENANT, ALICE, registry, "salt")
    assert result["deleted_nodes"] == 1
    assert result["anonymized_nodes"] == 0
    assert _get_node(store, nid) is None


def test_business_nodes_are_anonymized(store, registry):
    nid = _mk_node(
        store, 11, {"title": "Fix bug", "assignee_email": "alice@a.com", "priority": 2}, ALICE
    )
    result = store._sync_anonymize_user_in_tenant(TENANT, ALICE, registry, "salt")
    assert result["anonymized_nodes"] == 1
    assert result["deleted_nodes"] == 0
    row = _get_node(store, nid)
    assert row is not None
    payload = json.loads(row["payload_json"])
    assert payload["title"] == "Fix bug"  # pii=false preserved
    assert payload["assignee_email"] == ""  # pii=true scrubbed
    assert payload["priority"] == 2
    assert row["owner_actor"] == CanonicalStore.anon_id(ALICE, "salt")


def test_financial_nodes_are_retained_and_anonymized(store, registry):
    nid = _mk_node(store, 12, {"amount": 500, "customer_email": "alice@a.com"}, ALICE)
    result = store._sync_anonymize_user_in_tenant(TENANT, ALICE, registry, "salt")
    assert result["anonymized_nodes"] == 1
    row = _get_node(store, nid)
    assert row is not None
    payload = json.loads(row["payload_json"])
    assert payload["amount"] == 500
    assert payload["customer_email"] == ""


def test_audit_nodes_are_retained_and_anonymized(store, registry):
    nid = _mk_node(store, 13, {"action": "login", "actor_email": "alice@a.com"}, ALICE)
    result = store._sync_anonymize_user_in_tenant(TENANT, ALICE, registry, "salt")
    assert result["anonymized_nodes"] == 1
    row = _get_node(store, nid)
    assert row is not None
    payload = json.loads(row["payload_json"])
    assert payload["action"] == "login"
    assert payload["actor_email"] == ""


def test_ephemeral_nodes_are_deleted(store, registry):
    nid = _mk_node(store, 14, {"token": "abc"}, ALICE)
    result = store._sync_anonymize_user_in_tenant(TENANT, ALICE, registry, "salt")
    assert result["deleted_nodes"] == 1
    assert _get_node(store, nid) is None


def test_subject_field_nodes_are_anonymized(store, registry):
    """Performance review is OWNED by admin but ABOUT alice (subject_field)."""
    nid = _mk_node(store, 15, {"employee_id": ALICE, "score": 9}, ADMIN)
    result = store._sync_anonymize_user_in_tenant(TENANT, ALICE, registry, "salt")
    assert result["anonymized_nodes"] == 1
    row = _get_node(store, nid)
    assert row is not None
    payload = json.loads(row["payload_json"])
    # subject_field rewritten to anon id even though it's also pii=true
    assert payload["employee_id"] == CanonicalStore.anon_id(ALICE, "salt")
    assert row["owner_actor"] == ADMIN


def test_unrelated_nodes_are_untouched(store, registry):
    _ = _mk_node(store, 11, {"title": "A", "assignee_email": "a@a"}, ALICE)
    n_bob = _mk_node(store, 11, {"title": "B", "assignee_email": "b@b"}, BOB)
    store._sync_anonymize_user_in_tenant(TENANT, ALICE, registry, "salt")
    row = _get_node(store, n_bob)
    assert row is not None
    assert row["owner_actor"] == BOB
    payload = json.loads(row["payload_json"])
    assert payload["assignee_email"] == "b@b"


def test_counts_are_accurate(store, registry):
    _mk_node(store, 10, {"title": "p", "body": "x"}, ALICE)  # deleted
    _mk_node(store, 14, {"token": "t"}, ALICE)  # deleted
    _mk_node(store, 11, {"title": "t1", "assignee_email": "e"}, ALICE)  # anon
    _mk_node(store, 12, {"amount": 1, "customer_email": "e"}, ALICE)  # anon
    _mk_node(store, 11, {"title": "bob task", "assignee_email": "b"}, BOB)  # untouched
    result = store._sync_anonymize_user_in_tenant(TENANT, ALICE, registry, "salt")
    assert result["deleted_nodes"] == 2
    assert result["anonymized_nodes"] == 2


# ── Anonymization: edges ───────────────────────────────────────────


def test_edge_on_subject_exit_both_anonymizes_to_direction(store, registry):
    task = _mk_node(store, 11, {"title": "t", "assignee_email": "x"}, BOB)
    _mk_edge(store, 100, task, ALICE)  # BUSINESS + BOTH
    result = store._sync_anonymize_user_in_tenant(TENANT, ALICE, registry, "salt")
    assert result["anonymized_edges"] == 1
    edges = _list_edges(store)
    assert len(edges) == 1
    anon = CanonicalStore.anon_id(ALICE, "salt")
    assert edges[0]["to_node_id"] == anon


def test_edge_on_subject_exit_both_handles_from_direction(store, registry):
    bob_node = _mk_node(store, 20, {"email": "b@b"}, BOB)
    _mk_edge(store, 100, ALICE, bob_node)
    store._sync_anonymize_user_in_tenant(TENANT, ALICE, registry, "salt")
    edges = _list_edges(store)
    assert len(edges) == 1
    anon = CanonicalStore.anon_id(ALICE, "salt")
    assert edges[0]["from_node_id"] == anon


def test_edge_on_subject_exit_from_ignores_to_direction(store, registry):
    """FollowsUser (edge_id=101, on_subject_exit=FROM) — when alice is TO,
    the edge should be left alone."""
    bob_node = _mk_node(store, 20, {"email": "b@b"}, BOB)
    _mk_edge(store, 101, bob_node, ALICE)
    store._sync_anonymize_user_in_tenant(TENANT, ALICE, registry, "salt")
    edges = _list_edges(store)
    assert len(edges) == 1
    assert edges[0]["to_node_id"] == ALICE


def test_edge_on_subject_exit_from_deletes_when_user_is_from(store, registry):
    bob_node = _mk_node(store, 20, {"email": "b@b"}, BOB)
    _mk_edge(store, 101, ALICE, bob_node)
    store._sync_anonymize_user_in_tenant(TENANT, ALICE, registry, "salt")
    # PERSONAL edge policy → delete
    edges = _list_edges(store)
    assert len(edges) == 0


def test_edge_on_subject_exit_to_deletes_when_user_is_to(store, registry):
    task = _mk_node(store, 11, {"title": "t", "assignee_email": "x"}, BOB)
    _mk_edge(store, 102, task, ALICE)
    store._sync_anonymize_user_in_tenant(TENANT, ALICE, registry, "salt")
    edges = _list_edges(store)
    assert len(edges) == 0


def test_edge_on_subject_exit_to_ignores_from_direction(store, registry):
    bob_node = _mk_node(store, 20, {"email": "b@b"}, BOB)
    _mk_edge(store, 102, ALICE, bob_node)
    store._sync_anonymize_user_in_tenant(TENANT, ALICE, registry, "salt")
    edges = _list_edges(store)
    assert len(edges) == 1  # untouched


# ── Export user data ───────────────────────────────────────────────


def test_export_includes_owned_nodes(store, registry):
    nid = _mk_node(store, 11, {"title": "Own", "assignee_email": "e", "priority": 1}, ALICE)
    data = store._sync_export_user_data(TENANT, ALICE, registry)
    ids = [n["node_id"] for n in data["nodes"]]
    assert nid in ids


def test_export_includes_subject_field_match(store, registry):
    nid = _mk_node(store, 15, {"employee_id": ALICE, "score": 7}, ADMIN)
    data = store._sync_export_user_data(TENANT, ALICE, registry)
    ids = [n["node_id"] for n in data["nodes"]]
    assert nid in ids


def test_export_excludes_audit_records(store, registry):
    nid = _mk_node(store, 13, {"action": "x", "actor_email": "a@a"}, ALICE)
    data = store._sync_export_user_data(TENANT, ALICE, registry)
    ids = [n["node_id"] for n in data["nodes"]]
    assert nid not in ids


def test_export_excludes_unrelated_nodes(store, registry):
    _mk_node(store, 11, {"title": "bob", "assignee_email": "b", "priority": 1}, BOB)
    data = store._sync_export_user_data(TENANT, ALICE, registry)
    assert data["nodes"] == []


def test_export_respects_multiple_policies(store, registry):
    _mk_node(store, 10, {"title": "j", "body": "b"}, ALICE)
    _mk_node(store, 11, {"title": "t", "assignee_email": "e"}, ALICE)
    _mk_node(store, 12, {"amount": 1, "customer_email": "e"}, ALICE)
    _mk_node(store, 13, {"action": "x", "actor_email": "e"}, ALICE)
    data = store._sync_export_user_data(TENANT, ALICE, registry)
    policies = sorted(n["data_policy"] for n in data["nodes"])
    assert "audit" not in policies
    assert "personal" in policies
    assert "business" in policies
    assert "financial" in policies


# ── delete_tenant_database ─────────────────────────────────────────


def test_delete_tenant_database_removes_file(store):
    path = store.get_db_path(TENANT)
    assert path.exists()
    ok = store.delete_tenant_database(TENANT)
    assert ok is True
    assert not path.exists()


def test_delete_tenant_database_is_idempotent(store):
    store.delete_tenant_database(TENANT)
    assert store.delete_tenant_database(TENANT) is False


# ── GlobalStore deletion queue ─────────────────────────────────────


@pytest.mark.asyncio
async def test_queue_deletion_and_grace_period(global_store):
    await global_store.create_user("u1", "u1@x.com", "U1")
    entry = await global_store.queue_deletion("u1", grace_days=30)
    assert entry["execute_at"] > entry["requested_at"]
    now_before = entry["requested_at"] + 10
    exec1 = await global_store.get_executable_deletions(now=now_before)
    assert exec1 == []
    exec2 = await global_store.get_executable_deletions(now=entry["execute_at"] + 1)
    assert len(exec2) == 1
    assert exec2[0]["user_id"] == "u1"


@pytest.mark.asyncio
async def test_cancel_deletion_during_grace(global_store):
    await global_store.create_user("u2", "u2@x.com", "U2")
    await global_store.queue_deletion("u2", grace_days=30)
    ok = await global_store.cancel_deletion("u2")
    assert ok is True
    pending = await global_store.get_pending_deletions()
    assert pending == []


@pytest.mark.asyncio
async def test_mark_deletion_completed(global_store):
    await global_store.create_user("u3", "u3@x.com", "U3")
    await global_store.queue_deletion("u3", grace_days=30)
    ok = await global_store.mark_deletion_completed("u3")
    assert ok is True
    entry = await global_store.get_deletion_entry("u3")
    assert entry["status"] == "completed"


@pytest.mark.asyncio
async def test_global_cleanup_removes_memberships_and_shared(global_store):
    await global_store.create_user("u4", "u4@x.com", "U4")
    await global_store.create_tenant("t-a", "A")
    await global_store.create_tenant("t-b", "B")
    await global_store.add_member("t-a", "u4", role="member")
    await global_store.add_member("t-b", "u4", role="member")
    await global_store.add_shared("u4", "t-a", "node1", "read")
    removed_m = await global_store.remove_all_memberships_for_user("u4")
    removed_s = await global_store.remove_all_shared_for_user("u4")
    assert removed_m == 2
    assert removed_s == 1
    assert await global_store.get_user_tenants("u4") == []


# ── GdprDeletionWorker.run_once ────────────────────────────────────


@pytest.mark.asyncio
async def test_worker_deletes_personal_tenant(tmp_path, store, registry, global_store):
    await global_store.create_user("alice", "alice@x", "Alice")
    await global_store.create_tenant(TENANT, "Alice's workspace")
    await global_store.add_member(TENANT, "alice", role="owner")
    await global_store.queue_deletion("alice", grace_days=0)
    worker = GdprDeletionWorker(global_store, store, registry, poll_interval=1)
    path = store.get_db_path(TENANT)
    assert path.exists()
    result = await worker.run_once(now=int(time.time()) + 86400)
    assert result["processed"] == 1
    assert not path.exists()
    user = await global_store.get_user("alice")
    assert user["status"] == "deleted"
    entry = await global_store.get_deletion_entry("alice")
    assert entry["status"] == "completed"


@pytest.mark.asyncio
async def test_worker_anonymizes_shared_tenant(tmp_path, store, registry, global_store):
    await global_store.create_user("alice", "a@x", "Alice")
    await global_store.create_user("bob", "b@x", "Bob")
    await global_store.create_tenant(TENANT, "Shared")
    await global_store.add_member(TENANT, "alice", role="member")
    await global_store.add_member(TENANT, "bob", role="owner")
    _mk_node(store, 11, {"title": "t", "assignee_email": "a", "priority": 1}, ALICE)
    _mk_node(store, 11, {"title": "bob", "assignee_email": "b", "priority": 2}, BOB)
    await global_store.queue_deletion("alice", grace_days=0)
    worker = GdprDeletionWorker(global_store, store, registry, poll_interval=1)
    result = await worker.run_once(now=int(time.time()) + 86400)
    assert result["processed"] == 1
    path = store.get_db_path(TENANT)
    assert path.exists()
    with store._get_connection(TENANT) as conn:
        rows = conn.execute("SELECT owner_actor FROM nodes").fetchall()
    owners = sorted([r["owner_actor"] for r in rows])
    anon = CanonicalStore.anon_id(ALICE, "entdb-gdpr-v1")
    assert BOB in owners
    assert anon in owners


@pytest.mark.asyncio
async def test_worker_skips_not_yet_due(store, registry, global_store):
    await global_store.create_user("alice", "a@x", "Alice")
    await global_store.create_tenant(TENANT, "x")
    await global_store.add_member(TENANT, "alice", role="owner")
    await global_store.queue_deletion("alice", grace_days=30)
    worker = GdprDeletionWorker(global_store, store, registry, poll_interval=1)
    result = await worker.run_once(now=int(time.time()) + 60)
    assert result["processed"] == 0


@pytest.mark.asyncio
async def test_worker_is_idempotent(store, registry, global_store):
    await global_store.create_user("alice", "a@x", "Alice")
    await global_store.create_tenant(TENANT, "x")
    await global_store.add_member(TENANT, "alice", role="owner")
    await global_store.queue_deletion("alice", grace_days=0)
    worker = GdprDeletionWorker(global_store, store, registry, poll_interval=1)
    await worker.run_once(now=int(time.time()) + 86400)
    result2 = await worker.run_once(now=int(time.time()) + 86400)
    assert result2["processed"] == 0


# ── Server handlers ────────────────────────────────────────────────


class _AbortError(BaseException):
    pass


class _FakeContext:
    def __init__(self) -> None:
        self.aborted = False
        self.abort_code = None
        self.abort_message = None

    async def abort(self, code, message):
        self.aborted = True
        self.abort_code = code
        self.abort_message = message
        raise _AbortError(f"[{code}] {message}")


def _make_servicer(canonical_store, global_store, registry):
    wal = MagicMock()
    wal.append = AsyncMock()
    servicer = EntDBServicer(
        wal=wal,
        canonical_store=canonical_store,
        mailbox_store=MagicMock(),
        schema_registry=registry,
        global_store=global_store,
    )
    return servicer


@pytest.mark.asyncio
async def test_delete_user_handler_queues_and_marks_pending(store, registry, global_store):
    await global_store.create_user("alice", "a@x", "Alice")
    servicer = _make_servicer(store, global_store, registry)
    req = DeleteUserRequest(actor=ALICE, user_id="alice", grace_days=30)
    resp = await servicer.DeleteUser(req, _FakeContext())
    assert resp.success is True
    assert resp.status == "pending"
    user = await global_store.get_user("alice")
    assert user["status"] == "pending_deletion"


@pytest.mark.asyncio
async def test_delete_user_handler_idempotent_on_existing(store, registry, global_store):
    await global_store.create_user("alice", "a@x", "Alice")
    servicer = _make_servicer(store, global_store, registry)
    req = DeleteUserRequest(actor=ALICE, user_id="alice", grace_days=30)
    r1 = await servicer.DeleteUser(req, _FakeContext())
    r2 = await servicer.DeleteUser(req, _FakeContext())
    assert r1.success and r2.success
    assert r1.execute_at == r2.execute_at


@pytest.mark.asyncio
async def test_delete_user_handler_requires_self_or_admin(store, registry, global_store):
    await global_store.create_user("alice", "a@x", "Alice")
    servicer = _make_servicer(store, global_store, registry)
    req = DeleteUserRequest(actor="user:mallory", user_id="alice")
    ctx = _FakeContext()
    with pytest.raises(_AbortError):
        await servicer.DeleteUser(req, ctx)
    assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED


@pytest.mark.asyncio
async def test_delete_user_handler_unknown_user(store, registry, global_store):
    servicer = _make_servicer(store, global_store, registry)
    req = DeleteUserRequest(actor=ALICE, user_id="alice")
    resp = await servicer.DeleteUser(req, _FakeContext())
    assert resp.success is False


@pytest.mark.asyncio
async def test_cancel_user_deletion_handler(store, registry, global_store):
    await global_store.create_user("alice", "a@x", "Alice")
    servicer = _make_servicer(store, global_store, registry)
    await servicer.DeleteUser(DeleteUserRequest(actor=ALICE, user_id="alice"), _FakeContext())
    resp = await servicer.CancelUserDeletion(
        CancelUserDeletionRequest(actor=ALICE, user_id="alice"), _FakeContext()
    )
    assert resp.success is True
    user = await global_store.get_user("alice")
    assert user["status"] == "active"


@pytest.mark.asyncio
async def test_export_user_data_handler_returns_bundle(store, registry, global_store):
    await global_store.create_user("alice", "a@x", "Alice")
    await global_store.create_tenant(TENANT, "T")
    await global_store.add_member(TENANT, "alice", role="owner")
    _mk_node(store, 11, {"title": "Own", "assignee_email": "e", "priority": 1}, ALICE)
    servicer = _make_servicer(store, global_store, registry)
    resp = await servicer.ExportUserData(
        ExportUserDataRequest(actor=ALICE, user_id="alice"), _FakeContext()
    )
    assert resp.success is True
    bundle = json.loads(resp.export_json)
    assert bundle["user_id"] == "alice"
    tenants = bundle["tenants"]
    assert len(tenants) == 1
    assert len(tenants[0]["nodes"]) == 1


@pytest.mark.asyncio
async def test_export_user_data_handler_requires_auth(store, registry, global_store):
    await global_store.create_user("alice", "a@x", "Alice")
    servicer = _make_servicer(store, global_store, registry)
    ctx = _FakeContext()
    with pytest.raises(_AbortError):
        await servicer.ExportUserData(
            ExportUserDataRequest(actor="user:mallory", user_id="alice"), ctx
        )
    assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED


@pytest.mark.asyncio
async def test_freeze_user_handler(store, registry, global_store):
    await global_store.create_user("alice", "a@x", "Alice")
    servicer = _make_servicer(store, global_store, registry)
    resp = await servicer.FreezeUser(
        FreezeUserRequest(actor=ALICE, user_id="alice", enabled=True), _FakeContext()
    )
    assert resp.success is True
    assert resp.status == "frozen"
    user = await global_store.get_user("alice")
    assert user["status"] == "frozen"


@pytest.mark.asyncio
async def test_unfreeze_user_handler(store, registry, global_store):
    await global_store.create_user("alice", "a@x", "Alice")
    servicer = _make_servicer(store, global_store, registry)
    await servicer.FreezeUser(
        FreezeUserRequest(actor=ALICE, user_id="alice", enabled=True), _FakeContext()
    )
    resp = await servicer.FreezeUser(
        FreezeUserRequest(actor=ALICE, user_id="alice", enabled=False), _FakeContext()
    )
    assert resp.success is True
    assert resp.status == "active"


@pytest.mark.asyncio
async def test_freeze_user_rejects_writes(store, registry, global_store):
    """A frozen user's writes must be rejected by _check_tenant_access."""
    await global_store.create_user("alice", "a@x", "Alice")
    await global_store.create_tenant(TENANT, "T")
    await global_store.add_member(TENANT, "alice", role="member")
    await global_store.set_user_status("alice", "frozen")
    servicer = _make_servicer(store, global_store, registry)
    ctx = _FakeContext()
    with pytest.raises(_AbortError):
        await servicer._check_tenant_access(
            tenant_id=TENANT,
            actor=ALICE,
            context=ctx,
            require_write=True,
            op_kind="write",
        )
    assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED


@pytest.mark.asyncio
async def test_frozen_user_can_still_read(store, registry, global_store):
    await global_store.create_user("alice", "a@x", "Alice")
    await global_store.create_tenant(TENANT, "T")
    await global_store.add_member(TENANT, "alice", role="member")
    await global_store.set_user_status("alice", "frozen")
    servicer = _make_servicer(store, global_store, registry)
    role = await servicer._check_tenant_access(
        tenant_id=TENANT,
        actor=ALICE,
        context=_FakeContext(),
        require_write=False,
    )
    assert role == "member"


# ── Registry helpers ──────────────────────────────────────────────


def test_registry_edge_policy_and_on_subject_exit(registry):
    assert registry.get_edge_data_policy(100) == DataPolicy.BUSINESS
    assert registry.get_edge_on_subject_exit(100) == "both"
    assert registry.get_edge_on_subject_exit(101) == "from"
    assert registry.get_edge_on_subject_exit(102) == "to"


def test_registry_edge_unknown_id_raises(registry):
    with pytest.raises(KeyError):
        registry.get_edge_data_policy(9999)
    with pytest.raises(KeyError):
        registry.get_edge_on_subject_exit(9999)
