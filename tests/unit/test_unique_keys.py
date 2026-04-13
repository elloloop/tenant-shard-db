"""
Unit tests for the unique-keys / secondary-lookup-keys feature
(2026-04-13 unique_keys decision).

Covers four layers:

1. CanonicalStore / applier — per-tenant ``node_keys`` table writes,
   cascade cleanup on delete, collision detection, cross-tenant
   isolation, and mailbox / public storage modes.
2. gRPC servicer — pre-validate fast-fail via ``ALREADY_EXISTS`` and
   the new ``GetNodeByKey`` handler.
3. Python SDK — ``Plan.create(..., keys=...)``, ``scope.get_by_key``,
   ``UniqueConstraintError``.
4. Proto / schema registry — ``NodeOpts.keys`` round-trip.
"""

from __future__ import annotations

import tempfile
from unittest.mock import AsyncMock, MagicMock

import grpc
import pytest
from google.protobuf.struct_pb2 import Struct

from dbaas.entdb_server.api.generated import (
    CreateNodeOp,
    ExecuteAtomicRequest,
    GetNodeByKeyRequest,
    Operation,
    RequestContext,
)
from dbaas.entdb_server.api.grpc_server import EntDBServicer
from dbaas.entdb_server.apply.applier import (
    Applier,
    MailboxFanoutConfig,
    TransactionEvent,
)
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.global_store import GlobalStore
from dbaas.entdb_server.wal.memory import InMemoryWalStream

TENANT = "tenant-uk-tests"
ALICE = "user:alice"
BOB = "user:bob"


# ── Fixtures ────────────────────────────────────────────────────────


@pytest.fixture
def store(tmp_path):
    s = CanonicalStore(data_dir=str(tmp_path))
    with s._get_connection(TENANT, create=True) as conn:
        s._create_schema(conn)
    return s


@pytest.fixture
def global_store():
    with tempfile.TemporaryDirectory() as tmpdir:
        gs = GlobalStore(tmpdir)
        yield gs
        gs.close()


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


async def _bootstrap_tenant(
    gs: GlobalStore,
    tenant_id: str = TENANT,
    members: dict[str, str] | None = None,
) -> None:
    await gs.create_tenant(tenant_id, f"Tenant {tenant_id}")
    if members:
        for user_id, role in members.items():
            await gs.add_member(tenant_id, user_id, role=role)


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


def _create_event(
    tenant_id: str,
    idempotency_key: str,
    *,
    type_id: int = 101,
    node_id: str | None = None,
    data: dict | None = None,
    keys: dict[str, str] | None = None,
    actor: str = ALICE,
) -> TransactionEvent:
    op: dict = {
        "op": "create_node",
        "type_id": type_id,
        "id": node_id or f"n-{idempotency_key}",
        "data": data or {"k": idempotency_key},
    }
    if keys:
        op["keys"] = keys
    return TransactionEvent.from_dict(
        {
            "tenant_id": tenant_id,
            "actor": actor,
            "idempotency_key": idempotency_key,
            "ops": [op],
        }
    )


async def _make_applier(store) -> Applier:
    wal = InMemoryWalStream(num_partitions=1)
    await wal.connect()
    return Applier(
        wal=wal,
        canonical_store=store,
        topic="t",
        fanout_config=MailboxFanoutConfig(enabled=False),
    )


# ════════════════════════════════════════════════════════════════════
# 1-12. Applier / canonical store
# ════════════════════════════════════════════════════════════════════


class TestApplierUniqueKeys:
    @pytest.mark.asyncio
    async def test_create_node_with_keys_inserts_node_keys_row(self, store):
        applier = await _make_applier(store)
        event = _create_event(
            TENANT,
            "ev-1",
            node_id="alice",
            keys={"email": "alice@example.com"},
        )
        result = await applier.apply_event(event)
        assert result.success

        resolved = await store.get_node_by_key(TENANT, 101, "email", "alice@example.com")
        assert resolved == "alice"

    @pytest.mark.asyncio
    async def test_create_duplicate_key_raises_unique_constraint(self, store):
        applier = await _make_applier(store)
        ev1 = _create_event(
            TENANT,
            "ev-a",
            node_id="alice",
            keys={"email": "alice@example.com"},
        )
        ev2 = _create_event(
            TENANT,
            "ev-b",
            node_id="bob",
            keys={"email": "alice@example.com"},
        )
        r1 = await applier.apply_event(ev1)
        assert r1.success

        r2 = await applier.apply_event(ev2)
        assert not r2.success
        assert "UNIQUE" in r2.error.upper() or "unique" in r2.error.lower()

    @pytest.mark.asyncio
    async def test_duplicate_key_transaction_rolled_back(self, store):
        """A failed key insert must also roll back the node row."""
        applier = await _make_applier(store)
        ev1 = _create_event(
            TENANT,
            "ev-a",
            node_id="alice",
            keys={"email": "alice@example.com"},
        )
        ev2 = _create_event(
            TENANT,
            "ev-b",
            node_id="bob-dup",
            keys={"email": "alice@example.com"},
        )
        await applier.apply_event(ev1)
        r2 = await applier.apply_event(ev2)
        assert not r2.success

        # bob-dup must not be present — the IntegrityError rolled
        # the whole batch_transaction back.
        leftover = await store.get_node(TENANT, "bob-dup")
        assert leftover is None

    @pytest.mark.asyncio
    async def test_update_node_replaces_keys(self, store):
        applier = await _make_applier(store)
        ev1 = _create_event(
            TENANT,
            "ev-a",
            node_id="alice",
            keys={"email": "alice@example.com"},
        )
        r1 = await applier.apply_event(ev1)
        assert r1.success

        update_event = TransactionEvent.from_dict(
            {
                "tenant_id": TENANT,
                "actor": ALICE,
                "idempotency_key": "ev-upd",
                "ops": [
                    {
                        "op": "update_node",
                        "type_id": 101,
                        "id": "alice",
                        "patch": {},
                        "keys": {"email": "alice2@example.com"},
                    }
                ],
            }
        )
        r2 = await applier.apply_event(update_event)
        assert r2.success

        assert (await store.get_node_by_key(TENANT, 101, "email", "alice@example.com")) is None
        assert (await store.get_node_by_key(TENANT, 101, "email", "alice2@example.com")) == "alice"

    @pytest.mark.asyncio
    async def test_update_with_colliding_key_fails_and_keeps_old(self, store):
        applier = await _make_applier(store)
        await applier.apply_event(
            _create_event(TENANT, "ev-a", node_id="alice", keys={"email": "alice@example.com"})
        )
        await applier.apply_event(
            _create_event(TENANT, "ev-b", node_id="bob", keys={"email": "bob@example.com"})
        )

        update_event = TransactionEvent.from_dict(
            {
                "tenant_id": TENANT,
                "actor": ALICE,
                "idempotency_key": "ev-upd",
                "ops": [
                    {
                        "op": "update_node",
                        "type_id": 101,
                        "id": "bob",
                        "patch": {},
                        "keys": {"email": "alice@example.com"},  # colliding
                    }
                ],
            }
        )
        r = await applier.apply_event(update_event)
        assert not r.success
        # bob still holds the original key
        assert (await store.get_node_by_key(TENANT, 101, "email", "bob@example.com")) == "bob"

    @pytest.mark.asyncio
    async def test_delete_node_cascades_node_keys(self, store):
        applier = await _make_applier(store)
        await applier.apply_event(
            _create_event(
                TENANT,
                "ev-a",
                node_id="alice",
                keys={"email": "alice@example.com", "external_id": "ext-42"},
            )
        )
        assert (await store.get_node_by_key(TENANT, 101, "email", "alice@example.com")) == "alice"

        delete_event = TransactionEvent.from_dict(
            {
                "tenant_id": TENANT,
                "actor": ALICE,
                "idempotency_key": "ev-del",
                "ops": [{"op": "delete_node", "type_id": 101, "id": "alice"}],
            }
        )
        rd = await applier.apply_event(delete_event)
        assert rd.success

        assert (await store.get_node_by_key(TENANT, 101, "email", "alice@example.com")) is None
        assert (await store.get_node_by_key(TENANT, 101, "external_id", "ext-42")) is None

    @pytest.mark.asyncio
    async def test_get_node_by_key_unknown_key_is_none(self, store):
        assert (await store.get_node_by_key(TENANT, 101, "email", "nobody@example.com")) is None

    @pytest.mark.asyncio
    async def test_get_node_by_key_is_tenant_scoped(self, store, tmp_path):
        other = "tenant-other"
        with store._get_connection(other, create=True) as conn:
            store._create_schema(conn)
        applier = await _make_applier(store)
        # Same key value, different tenants → independent.
        await applier.apply_event(
            _create_event(
                TENANT,
                "ev-a",
                node_id="alice",
                keys={"email": "alice@example.com"},
            )
        )
        await applier.apply_event(
            _create_event(
                other,
                "ev-b",
                node_id="bob",
                keys={"email": "alice@example.com"},
            )
        )
        assert (await store.get_node_by_key(TENANT, 101, "email", "alice@example.com")) == "alice"
        assert (await store.get_node_by_key(other, 101, "email", "alice@example.com")) == "bob"

    @pytest.mark.asyncio
    async def test_multiple_keys_per_node(self, store):
        applier = await _make_applier(store)
        r = await applier.apply_event(
            _create_event(
                TENANT,
                "ev-a",
                node_id="alice",
                keys={"email": "alice@example.com", "external_id": "ext-42"},
            )
        )
        assert r.success
        assert (await store.get_node_by_key(TENANT, 101, "email", "alice@example.com")) == "alice"
        assert (await store.get_node_by_key(TENANT, 101, "external_id", "ext-42")) == "alice"

    @pytest.mark.asyncio
    async def test_keys_work_in_user_mailbox_storage(self, store):
        applier = await _make_applier(store)
        event = TransactionEvent.from_dict(
            {
                "tenant_id": TENANT,
                "actor": ALICE,
                "idempotency_key": "ev-mbx",
                "ops": [
                    {
                        "op": "create_node",
                        "type_id": 101,
                        "id": "private-1",
                        "data": {"k": "v"},
                        "storage_mode": "USER_MAILBOX",
                        "target_user_id": "alice",
                        "keys": {"token": "t-private"},
                    }
                ],
            }
        )
        r = await applier.apply_event(event)
        assert r.success
        # Mailbox nodes live in the mailbox file — verify the
        # node_keys row was materialised inside that physical DB.
        with store._get_mailbox_connection(TENANT, "alice") as conn:
            row = conn.execute(
                "SELECT node_id FROM node_keys "
                "WHERE type_id = ? AND key_name = ? AND key_value = ?",
                (101, "token", "t-private"),
            ).fetchone()
        assert row is not None
        assert (row[0] if isinstance(row, tuple) else row["node_id"]) == "private-1"

    @pytest.mark.asyncio
    async def test_keys_work_in_public_storage(self, store):
        applier = await _make_applier(store)
        event = TransactionEvent.from_dict(
            {
                "tenant_id": TENANT,
                "actor": ALICE,
                "idempotency_key": "ev-pub",
                "ops": [
                    {
                        "op": "create_node",
                        "type_id": 101,
                        "id": "pub-1",
                        "data": {"k": "v"},
                        "storage_mode": "PUBLIC",
                        "keys": {"slug": "hello-world"},
                    }
                ],
            }
        )
        r = await applier.apply_event(event)
        assert r.success
        with store._get_public_connection() as conn:
            row = conn.execute(
                "SELECT node_id FROM node_keys "
                "WHERE type_id = ? AND key_name = ? AND key_value = ?",
                (101, "slug", "hello-world"),
            ).fetchone()
        assert row is not None
        assert (row[0] if isinstance(row, tuple) else row["node_id"]) == "pub-1"


# ════════════════════════════════════════════════════════════════════
# 13-17. gRPC server — pre-validate + GetNodeByKey handler
# ════════════════════════════════════════════════════════════════════


class TestUniqueKeysGrpcHandlers:
    @pytest.mark.asyncio
    async def test_execute_atomic_duplicate_key_returns_already_exists(self, store, global_store):
        await _bootstrap_tenant(global_store, members={"alice": "owner"})
        # Pre-seed: directly insert the key row so pre-validate trips.
        with store._get_connection(TENANT) as conn:
            conn.execute(
                "INSERT INTO node_keys (tenant_id, type_id, node_id, "
                "key_name, key_value, created_at) VALUES (?, ?, ?, ?, ?, ?)",
                (TENANT, 101, "preexisting", "email", "alice@example.com", 0),
            )
            conn.commit()

        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()

        create_op = CreateNodeOp(type_id=101, id="alice", data=Struct())
        create_op.keys["email"] = "alice@example.com"
        req = ExecuteAtomicRequest(
            context=RequestContext(tenant_id=TENANT, actor=ALICE),
            idempotency_key="k-dup",
            operations=[Operation(create_node=create_op)],
        )
        with pytest.raises(_AbortError):
            await servicer.ExecuteAtomic(req, ctx)
        assert ctx.abort_code == grpc.StatusCode.ALREADY_EXISTS

    @pytest.mark.asyncio
    async def test_race_applier_catches_integrity_error(self, store):
        """Two creates with the same key — one wins, the other fails
        at apply time even if pre-validate passed."""
        applier = await _make_applier(store)
        ev1 = _create_event(TENANT, "ev-r1", node_id="n1", keys={"sku": "ABC-1"})
        ev2 = _create_event(TENANT, "ev-r2", node_id="n2", keys={"sku": "ABC-1"})
        r1 = await applier.apply_event(ev1)
        r2 = await applier.apply_event(ev2)
        # One wins, one loses — exactly one success.
        assert (r1.success, r2.success) in ((True, False), (False, True))

    @pytest.mark.asyncio
    async def test_get_node_by_key_returns_node(self, store, global_store):
        await _bootstrap_tenant(global_store, members={"alice": "owner"})
        # Insert a node directly and its key row.
        now = 1_700_000_000_000
        node = store._sync_create_node(TENANT, 101, {"name": "Alice"}, ALICE, "alice", [], now)
        assert node.node_id == "alice"
        with store._get_connection(TENANT) as conn:
            store.insert_node_keys(conn, TENANT, 101, "alice", {"email": "alice@example.com"}, now)
            conn.commit()

        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        resp = await servicer.GetNodeByKey(
            GetNodeByKeyRequest(
                tenant_id=TENANT,
                actor=ALICE,
                type_id=101,
                key_name="email",
                key_value="alice@example.com",
            ),
            ctx,
        )
        assert resp.found is True
        assert resp.node.node_id == "alice"

    @pytest.mark.asyncio
    async def test_get_node_by_key_not_found(self, store, global_store):
        await _bootstrap_tenant(global_store, members={"alice": "owner"})
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        resp = await servicer.GetNodeByKey(
            GetNodeByKeyRequest(
                tenant_id=TENANT,
                actor=ALICE,
                type_id=101,
                key_name="email",
                key_value="nobody@example.com",
            ),
            ctx,
        )
        assert resp.found is False

    @pytest.mark.asyncio
    async def test_get_node_by_key_enforces_tenant_check(self, store, global_store):
        """An unknown tenant still returns found=False (handler does
        not leak anything about whether the key exists or not)."""
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        resp = await servicer.GetNodeByKey(
            GetNodeByKeyRequest(
                tenant_id="tenant-does-not-exist",
                actor=ALICE,
                type_id=101,
                key_name="email",
                key_value="x@y.z",
            ),
            ctx,
        )
        assert resp.found is False


# ════════════════════════════════════════════════════════════════════
# 18-21. Python SDK — Plan / scope / errors
# ════════════════════════════════════════════════════════════════════


class TestSdkUniqueKeys:
    def test_plan_create_keys_are_serialized(self):
        from entdb_sdk.client import Plan
        from entdb_sdk.schema import FieldDef, FieldKind, NodeTypeDef

        client = MagicMock()
        client.registry = None

        plan = Plan(client=client, tenant_id="t", actor="user:a")
        node_type = NodeTypeDef(
            type_id=101,
            name="User",
            fields=(FieldDef(field_id=1, name="email", kind=FieldKind.STRING),),
        )
        plan.create(node_type, {"email": "a@x.z"}, keys={"email": "a@x.z"})
        op = plan._operations[0]["create_node"]
        assert op["keys"] == {"email": "a@x.z"}

    def test_plan_update_keys_are_serialized(self):
        from entdb_sdk.client import Plan
        from entdb_sdk.schema import FieldDef, FieldKind, NodeTypeDef

        client = MagicMock()
        plan = Plan(client=client, tenant_id="t", actor="user:a")
        node_type = NodeTypeDef(
            type_id=101,
            name="User",
            fields=(FieldDef(field_id=1, name="email", kind=FieldKind.STRING),),
        )
        plan.update(node_type, "n1", {"email": "b@x.z"}, keys={"email": "b@x.z"})
        op = plan._operations[0]["update_node"]
        assert op["keys"] == {"email": "b@x.z"}

    def test_unique_constraint_error_carries_fields(self):
        from entdb_sdk.errors import UniqueConstraintError

        err = UniqueConstraintError(
            "collision",
            tenant_id="t1",
            type_id=101,
            key_name="email",
            key_value="a@x.z",
        )
        assert err.tenant_id == "t1"
        assert err.type_id == 101
        assert err.key_name == "email"
        assert err.key_value == "a@x.z"
        assert err.code == "UNIQUE_CONSTRAINT"
        assert err.details["key_name"] == "email"

    def test_unique_constraint_error_is_entdb_error(self):
        from entdb_sdk.errors import EntDbError, UniqueConstraintError

        err = UniqueConstraintError("x", tenant_id="t")
        assert isinstance(err, EntDbError)

    def test_unique_constraint_error_exported_from_init(self):
        import entdb_sdk

        assert hasattr(entdb_sdk, "UniqueConstraintError")
        assert "UniqueConstraintError" in entdb_sdk.__all__


# ════════════════════════════════════════════════════════════════════
# 22. Proto / schema — NodeOpts.keys round-trip
# ════════════════════════════════════════════════════════════════════


class TestProtoNodeKeySpec:
    def test_node_key_spec_round_trip(self):
        from entdb_sdk._generated import entdb_options_pb2

        opts = entdb_options_pb2.NodeOpts()
        opts.type_id = 101
        spec = opts.keys.add()
        spec.name = "email"
        spec.required = True
        spec2 = opts.keys.add()
        spec2.name = "external_id"
        spec2.required = False

        wire = opts.SerializeToString()
        round = entdb_options_pb2.NodeOpts()
        round.ParseFromString(wire)
        assert len(round.keys) == 2
        assert round.keys[0].name == "email"
        assert round.keys[0].required is True
        assert round.keys[1].name == "external_id"
        assert round.keys[1].required is False

    def test_create_node_op_keys_round_trip(self):
        from dbaas.entdb_server.api.generated import CreateNodeOp

        op = CreateNodeOp(type_id=101, id="n1")
        op.keys["email"] = "alice@example.com"
        op.keys["external_id"] = "ext-42"
        wire = op.SerializeToString()
        r = CreateNodeOp()
        r.ParseFromString(wire)
        assert dict(r.keys) == {
            "email": "alice@example.com",
            "external_id": "ext-42",
        }

    def test_get_node_by_key_default_op_is_read(self):
        from dbaas.entdb_server.auth.capability_registry import (
            DEFAULT_OP_REQUIREMENTS,
            CoreCapability,
        )

        assert DEFAULT_OP_REQUIREMENTS["GetNodeByKey"] == CoreCapability.READ
