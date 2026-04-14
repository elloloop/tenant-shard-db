"""
Unit tests for the unique-field / GetNodeByKey feature
(2026-04-14 SDK v0.3 decision, supersedes the 2026-04-13 ``node_keys``
table design).

Covers three layers:

1. CanonicalStore / applier — lazy unique expression indexes,
   duplicate rejection at apply time, lookup-by-key via the same
   index, tenant isolation.
2. gRPC servicer — ``ExecuteAtomic`` fast-fail pre-check,
   ``GetNodeByKey`` handler with typed ``(field_id, value)`` request.
3. Proto — ``CreateNodeOp`` no longer carries a ``keys`` map,
   ``FieldOpts.unique`` round-trips, ``GetNodeByKeyRequest`` carries
   ``field_id`` + ``google.protobuf.Value``.

The old ``node_keys`` table / ``keys={"email": ...}`` argument / string
``key_name`` lookup are all gone. The user-facing contract is: declare
``(entdb.field).unique = true`` in proto, write the value via the
regular payload, and look it up with ``(field_id, value)``.
"""

from __future__ import annotations

import tempfile
from unittest.mock import AsyncMock, MagicMock

import grpc
import pytest
from google.protobuf.struct_pb2 import Struct, Value

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
    UniqueConstraintError,
    _parse_unique_index_name,
)
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.global_store import GlobalStore
from dbaas.entdb_server.schema.registry import (
    SchemaRegistry,
    get_registry,
    reset_registry,
)
from dbaas.entdb_server.schema.types import NodeTypeDef, field
from dbaas.entdb_server.wal.memory import InMemoryWalStream

TENANT = "tenant-uk-tests"
ALICE = "user:alice"
BOB = "user:bob"
TYPE_USER = 101

# Field ids that the tests use — these match the ``field`` calls in the
# ``registered_schema`` fixture below.
FIELD_EMAIL = 1
FIELD_EXTERNAL_ID = 2


# ── Fixtures ────────────────────────────────────────────────────────


@pytest.fixture(autouse=True)
def registered_schema():
    """Register a ``User`` type with two unique fields for every test.

    Autouse because every test in this module depends on the applier
    being able to see the schema when it decides which indexes to
    create. ``reset_registry`` keeps the global singleton from leaking
    state between tests.
    """
    reset_registry()
    reg = get_registry()
    user = NodeTypeDef(
        type_id=TYPE_USER,
        name="User",
        fields=(
            field(FIELD_EMAIL, "email", "str", unique=True),
            field(FIELD_EXTERNAL_ID, "external_id", "str", unique=True),
            field(3, "name", "str"),
        ),
    )
    reg.register_node_type(user)
    yield reg
    reset_registry()


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


def _make_servicer(canonical_store, global_store=None, schema_registry=None):
    wal = MagicMock()
    pos = MagicMock()
    pos.__str__ = MagicMock(return_value="0:0:0")
    wal.append = AsyncMock(return_value=pos)

    # Default to the real registry so ``get_unique_field_ids`` works
    # for the gRPC pre-check path.
    reg = schema_registry if schema_registry is not None else get_registry()
    # Tests don't freeze the registry, so expose a blank fingerprint
    # attribute on a wrapper object to satisfy the servicer contract.
    reg_view: SchemaRegistry | MagicMock
    if hasattr(reg, "fingerprint"):
        reg_view = reg
    else:
        reg_view = MagicMock()
        reg_view.fingerprint = ""
    # Callers that pass a real ``SchemaRegistry`` need a fingerprint
    # attribute that doesn't blow up the schema check.
    if getattr(reg_view, "fingerprint", None) is None:
        try:
            reg_view._fingerprint = ""  # type: ignore[attr-defined]
        except Exception:
            pass

    return EntDBServicer(
        wal=wal,
        canonical_store=canonical_store,
        schema_registry=reg_view,
        global_store=global_store,
    )


def _make_create_event(
    idempotency_key: str,
    *,
    node_id: str,
    payload: dict[str, str | int],
    tenant_id: str = TENANT,
    type_id: int = TYPE_USER,
    actor: str = ALICE,
) -> TransactionEvent:
    """Build a ``create_node`` event with id-keyed payload.

    The applier stores payloads keyed by ``field_id`` so the caller
    passes a ready-made id-keyed dict (``{"1": "alice@example.com"}``)
    to avoid coupling the tests to the ingress translator.
    """
    return TransactionEvent.from_dict(
        {
            "tenant_id": tenant_id,
            "actor": actor,
            "idempotency_key": idempotency_key,
            "ops": [
                {
                    "op": "create_node",
                    "type_id": type_id,
                    "id": node_id,
                    "data": payload,
                }
            ],
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
# 1. Applier / canonical store — lazy unique index + enforcement
# ════════════════════════════════════════════════════════════════════


class TestApplierUniqueFieldEnforcement:
    @pytest.mark.asyncio
    async def test_first_create_succeeds_and_lookup_works(self, store):
        applier = await _make_applier(store)
        event = _make_create_event(
            "ev-1",
            node_id="alice",
            payload={str(FIELD_EMAIL): "alice@example.com"},
        )
        result = await applier.apply_event(event)
        assert result.success

        node = await store.get_node_by_key(TENANT, TYPE_USER, FIELD_EMAIL, "alice@example.com")
        assert node is not None
        assert node.node_id == "alice"

    @pytest.mark.asyncio
    async def test_duplicate_create_raises_unique_constraint(self, store):
        applier = await _make_applier(store)
        r1 = await applier.apply_event(
            _make_create_event(
                "ev-a", node_id="alice", payload={str(FIELD_EMAIL): "alice@example.com"}
            )
        )
        assert r1.success

        r2 = await applier.apply_event(
            _make_create_event(
                "ev-b", node_id="bob", payload={str(FIELD_EMAIL): "alice@example.com"}
            )
        )
        assert not r2.success
        assert "unique" in (r2.error or "").lower()

    @pytest.mark.asyncio
    async def test_duplicate_create_rolls_back_node(self, store):
        """A failed create must not leave a half-written ``nodes`` row."""
        applier = await _make_applier(store)
        await applier.apply_event(
            _make_create_event(
                "ev-a", node_id="alice", payload={str(FIELD_EMAIL): "alice@example.com"}
            )
        )
        r2 = await applier.apply_event(
            _make_create_event(
                "ev-b", node_id="bob-dup", payload={str(FIELD_EMAIL): "alice@example.com"}
            )
        )
        assert not r2.success

        leftover = await store.get_node(TENANT, "bob-dup")
        assert leftover is None

    @pytest.mark.asyncio
    async def test_update_to_colliding_value_fails(self, store):
        applier = await _make_applier(store)
        await applier.apply_event(
            _make_create_event(
                "ev-a", node_id="alice", payload={str(FIELD_EMAIL): "alice@example.com"}
            )
        )
        await applier.apply_event(
            _make_create_event("ev-b", node_id="bob", payload={str(FIELD_EMAIL): "bob@example.com"})
        )

        update = TransactionEvent.from_dict(
            {
                "tenant_id": TENANT,
                "actor": ALICE,
                "idempotency_key": "ev-upd",
                "ops": [
                    {
                        "op": "update_node",
                        "type_id": TYPE_USER,
                        "id": "bob",
                        "patch": {str(FIELD_EMAIL): "alice@example.com"},
                    }
                ],
            }
        )
        r = await applier.apply_event(update)
        assert not r.success

        # Bob keeps his original value, alice is untouched.
        alice = await store.get_node_by_key(TENANT, TYPE_USER, FIELD_EMAIL, "alice@example.com")
        assert alice is not None
        assert alice.node_id == "alice"
        bob = await store.get_node_by_key(TENANT, TYPE_USER, FIELD_EMAIL, "bob@example.com")
        assert bob is not None
        assert bob.node_id == "bob"

    @pytest.mark.asyncio
    async def test_update_to_new_value_frees_old(self, store):
        """Changing a unique field must let its old value be reused."""
        applier = await _make_applier(store)
        await applier.apply_event(
            _make_create_event(
                "ev-a", node_id="alice", payload={str(FIELD_EMAIL): "old@example.com"}
            )
        )
        update = TransactionEvent.from_dict(
            {
                "tenant_id": TENANT,
                "actor": ALICE,
                "idempotency_key": "ev-upd",
                "ops": [
                    {
                        "op": "update_node",
                        "type_id": TYPE_USER,
                        "id": "alice",
                        "patch": {str(FIELD_EMAIL): "new@example.com"},
                    }
                ],
            }
        )
        assert (await applier.apply_event(update)).success

        # Reuse the freed value on a brand-new node — must succeed.
        reuse = await applier.apply_event(
            _make_create_event(
                "ev-r", node_id="charlie", payload={str(FIELD_EMAIL): "old@example.com"}
            )
        )
        assert reuse.success

        charlie = await store.get_node_by_key(TENANT, TYPE_USER, FIELD_EMAIL, "old@example.com")
        assert charlie is not None
        assert charlie.node_id == "charlie"

    @pytest.mark.asyncio
    async def test_delete_frees_value_for_reuse(self, store):
        applier = await _make_applier(store)
        await applier.apply_event(
            _make_create_event("ev-a", node_id="alice", payload={str(FIELD_EMAIL): "a@example.com"})
        )
        delete = TransactionEvent.from_dict(
            {
                "tenant_id": TENANT,
                "actor": ALICE,
                "idempotency_key": "ev-del",
                "ops": [{"op": "delete_node", "type_id": TYPE_USER, "id": "alice"}],
            }
        )
        assert (await applier.apply_event(delete)).success

        # Value must be free.
        assert await store.get_node_by_key(TENANT, TYPE_USER, FIELD_EMAIL, "a@example.com") is None

        r = await applier.apply_event(
            _make_create_event(
                "ev-b", node_id="second", payload={str(FIELD_EMAIL): "a@example.com"}
            )
        )
        assert r.success

    @pytest.mark.asyncio
    async def test_get_node_by_key_unknown_value(self, store):
        assert (
            await store.get_node_by_key(TENANT, TYPE_USER, FIELD_EMAIL, "nobody@example.com")
            is None
        )

    @pytest.mark.asyncio
    async def test_get_node_by_key_is_tenant_scoped(self, store):
        other = "tenant-other"
        with store._get_connection(other, create=True) as conn:
            store._create_schema(conn)
        applier = await _make_applier(store)

        # Same value, different tenants.
        await applier.apply_event(
            _make_create_event(
                "ev-a",
                node_id="alice",
                payload={str(FIELD_EMAIL): "same@example.com"},
            )
        )
        await applier.apply_event(
            _make_create_event(
                "ev-b",
                node_id="bob",
                payload={str(FIELD_EMAIL): "same@example.com"},
                tenant_id=other,
            )
        )

        n1 = await store.get_node_by_key(TENANT, TYPE_USER, FIELD_EMAIL, "same@example.com")
        n2 = await store.get_node_by_key(other, TYPE_USER, FIELD_EMAIL, "same@example.com")
        assert n1 is not None and n1.node_id == "alice"
        assert n2 is not None and n2.node_id == "bob"

    @pytest.mark.asyncio
    async def test_multiple_unique_fields_enforced_independently(self, store):
        applier = await _make_applier(store)
        await applier.apply_event(
            _make_create_event(
                "ev-a",
                node_id="alice",
                payload={
                    str(FIELD_EMAIL): "alice@example.com",
                    str(FIELD_EXTERNAL_ID): "ext-42",
                },
            )
        )
        # Collision on ``email``.
        r_email = await applier.apply_event(
            _make_create_event(
                "ev-b",
                node_id="bob",
                payload={
                    str(FIELD_EMAIL): "alice@example.com",
                    str(FIELD_EXTERNAL_ID): "ext-1",
                },
            )
        )
        assert not r_email.success
        # Collision on ``external_id``.
        r_ext = await applier.apply_event(
            _make_create_event(
                "ev-c",
                node_id="carol",
                payload={
                    str(FIELD_EMAIL): "carol@example.com",
                    str(FIELD_EXTERNAL_ID): "ext-42",
                },
            )
        )
        assert not r_ext.success
        # A brand-new row with both values distinct succeeds.
        r_ok = await applier.apply_event(
            _make_create_event(
                "ev-d",
                node_id="dave",
                payload={
                    str(FIELD_EMAIL): "dave@example.com",
                    str(FIELD_EXTERNAL_ID): "ext-7",
                },
            )
        )
        assert r_ok.success

    @pytest.mark.asyncio
    async def test_concurrent_creates_one_wins(self, store):
        """Race semantics — the applier is the single serialiser."""
        applier = await _make_applier(store)
        ev1 = _make_create_event(
            "ev-r1", node_id="n1", payload={str(FIELD_EMAIL): "race@example.com"}
        )
        ev2 = _make_create_event(
            "ev-r2", node_id="n2", payload={str(FIELD_EMAIL): "race@example.com"}
        )
        r1 = await applier.apply_event(ev1)
        r2 = await applier.apply_event(ev2)
        assert (r1.success, r2.success) in ((True, False), (False, True))


# ════════════════════════════════════════════════════════════════════
# 2. Lazy unique-index creation caching
# ════════════════════════════════════════════════════════════════════


class TestLazyIndexCreation:
    def test_first_call_creates_index(self, store):
        with store._get_connection(TENANT) as conn:
            store._ensure_unique_indexes(conn, TENANT, TYPE_USER, [FIELD_EMAIL])
            rows = conn.execute(
                "SELECT name FROM sqlite_master WHERE type = 'index' AND name LIKE 'idx_unique_t%'"
            ).fetchall()
        names = {r[0] if isinstance(r, tuple) else r["name"] for r in rows}
        assert f"idx_unique_t{TYPE_USER}_f{FIELD_EMAIL}" in names

    def test_second_call_is_noop_via_cache(self, store):
        """``_ensure_unique_indexes`` only issues DDL once per process.

        Verified by wrapping the connection in a small spy that
        records every ``execute`` call — the second invocation must
        not issue a ``CREATE UNIQUE INDEX`` statement.
        """
        with store._get_connection(TENANT) as conn:
            store._ensure_unique_indexes(conn, TENANT, TYPE_USER, [FIELD_EMAIL])
            assert store._unique_index_cache, "expected cache entry after first call"

            executed: list[str] = []

            class _SpyConn:
                def __init__(self, wrapped):
                    self._wrapped = wrapped

                def execute(self, sql, *args, **kwargs):
                    executed.append(sql)
                    return self._wrapped.execute(sql, *args, **kwargs)

            spy = _SpyConn(conn)
            store._ensure_unique_indexes(spy, TENANT, TYPE_USER, [FIELD_EMAIL])
        assert not any("CREATE UNIQUE INDEX" in s.upper() for s in executed), (
            f"second call should be a cache hit, but issued: {executed}"
        )

    def test_restart_safe_via_if_not_exists(self, store):
        """Restarting the process clears the cache but the CREATE
        INDEX IF NOT EXISTS is idempotent, so the applier can re-run
        without error."""
        with store._get_connection(TENANT) as conn:
            store._ensure_unique_indexes(conn, TENANT, TYPE_USER, [FIELD_EMAIL])

        # Simulate restart: blow away the process-local cache.
        store._unique_index_cache.clear()

        with store._get_connection(TENANT) as conn:
            # Must not raise — the DDL tolerates the existing index.
            store._ensure_unique_indexes(conn, TENANT, TYPE_USER, [FIELD_EMAIL])
            rows = conn.execute(
                "SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?",
                (f"idx_unique_t{TYPE_USER}_f{FIELD_EMAIL}",),
            ).fetchone()
        count = rows[0] if isinstance(rows, tuple) else rows["COUNT(*)"]
        assert count == 1


# ════════════════════════════════════════════════════════════════════
# 3. Error parsing + typed UniqueConstraintError
# ════════════════════════════════════════════════════════════════════


class TestUniqueConstraintError:
    def test_parse_index_name_from_integrity_message(self):
        msg = "UNIQUE constraint failed: index 'idx_unique_t201_f1'"
        assert _parse_unique_index_name(msg) == (201, 1)

    def test_parse_unrelated_message_returns_none(self):
        assert _parse_unique_index_name("some other error") is None

    def test_error_carries_structured_fields(self):
        err = UniqueConstraintError("t1", 101, 1, "alice@example.com")
        assert err.tenant_id == "t1"
        assert err.type_id == 101
        assert err.field_id == 1
        assert err.value == "alice@example.com"


# ════════════════════════════════════════════════════════════════════
# 4. gRPC surface — ExecuteAtomic pre-check + GetNodeByKey
# ════════════════════════════════════════════════════════════════════


class TestGrpcUniqueFieldHandlers:
    @pytest.mark.asyncio
    async def test_execute_atomic_duplicate_returns_already_exists(self, store, global_store):
        await _bootstrap_tenant(global_store, members={"alice": "owner"})

        applier = await _make_applier(store)
        await applier.apply_event(
            _make_create_event(
                "ev-seed",
                node_id="preexisting",
                payload={str(FIELD_EMAIL): "alice@example.com"},
            )
        )

        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()

        struct = Struct()
        struct.update({"email": "alice@example.com"})
        create_op = CreateNodeOp(type_id=TYPE_USER, id="alice", data=struct)
        req = ExecuteAtomicRequest(
            context=RequestContext(tenant_id=TENANT, actor=ALICE),
            idempotency_key="k-dup",
            operations=[Operation(create_node=create_op)],
        )

        with pytest.raises(_AbortError):
            await servicer.ExecuteAtomic(req, ctx)
        assert ctx.abort_code == grpc.StatusCode.ALREADY_EXISTS

    @pytest.mark.asyncio
    async def test_get_node_by_key_returns_node(self, store, global_store):
        await _bootstrap_tenant(global_store, members={"alice": "owner"})
        applier = await _make_applier(store)
        await applier.apply_event(
            _make_create_event(
                "ev-a",
                node_id="alice",
                payload={str(FIELD_EMAIL): "alice@example.com"},
            )
        )

        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        value = Value(string_value="alice@example.com")
        resp = await servicer.GetNodeByKey(
            GetNodeByKeyRequest(
                tenant_id=TENANT,
                actor=ALICE,
                type_id=TYPE_USER,
                field_id=FIELD_EMAIL,
                value=value,
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
                type_id=TYPE_USER,
                field_id=FIELD_EMAIL,
                value=Value(string_value="nobody@example.com"),
            ),
            ctx,
        )
        assert resp.found is False

    @pytest.mark.asyncio
    async def test_get_node_by_key_unknown_tenant(self, store, global_store):
        servicer = _make_servicer(store, global_store)
        ctx = _FakeContext()
        resp = await servicer.GetNodeByKey(
            GetNodeByKeyRequest(
                tenant_id="tenant-does-not-exist",
                actor=ALICE,
                type_id=TYPE_USER,
                field_id=FIELD_EMAIL,
                value=Value(string_value="x@y.z"),
            ),
            ctx,
        )
        assert resp.found is False


# ════════════════════════════════════════════════════════════════════
# 5. Proto round-trip — retired ``keys`` gone, FieldOpts.unique lives
# ════════════════════════════════════════════════════════════════════


class TestProtoSurface:
    def test_node_opts_has_no_keys_field(self):
        from sdk.entdb_sdk._generated import entdb_options_pb2

        opts = entdb_options_pb2.NodeOpts()
        assert not hasattr(opts, "keys") or not callable(getattr(opts, "keys", None))
        # Explicit: the underlying descriptor should not expose a
        # ``keys`` field (it was reserved) and also should not have
        # ``NodeKeySpec`` defined anywhere in the options module.
        field_names = {f.name for f in opts.DESCRIPTOR.fields}
        assert "keys" not in field_names
        assert not hasattr(entdb_options_pb2, "NodeKeySpec")

    def test_field_opts_has_unique_flag(self):
        from sdk.entdb_sdk._generated import entdb_options_pb2

        opts = entdb_options_pb2.FieldOpts()
        opts.unique = True
        wire = opts.SerializeToString()
        round_ = entdb_options_pb2.FieldOpts()
        round_.ParseFromString(wire)
        assert round_.unique is True

    def test_create_node_op_has_no_keys_field(self):
        op = CreateNodeOp(type_id=TYPE_USER, id="n1")
        field_names = {f.name for f in op.DESCRIPTOR.fields}
        assert "keys" not in field_names

    def test_get_node_by_key_request_uses_field_id_and_value(self):
        req = GetNodeByKeyRequest(
            tenant_id=TENANT,
            actor=ALICE,
            type_id=TYPE_USER,
            field_id=FIELD_EMAIL,
            value=Value(string_value="alice@example.com"),
        )
        wire = req.SerializeToString()
        r2 = GetNodeByKeyRequest()
        r2.ParseFromString(wire)
        assert r2.field_id == FIELD_EMAIL
        assert r2.value.string_value == "alice@example.com"

    def test_get_node_by_key_default_op_is_read(self):
        from dbaas.entdb_server.auth.capability_registry import (
            DEFAULT_OP_REQUIREMENTS,
            CoreCapability,
        )

        assert DEFAULT_OP_REQUIREMENTS["GetNodeByKey"] == CoreCapability.READ
