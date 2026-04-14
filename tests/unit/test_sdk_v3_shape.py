"""Canonical end-to-end test for the SDK v0.3 single-shape API.

This file is the executable form of the example block in
``docs/decisions/sdk_api.md``. Every method on the v0.3 surface is
exercised here: ``Plan.create`` from a proto message,
``Scope.get_by_key`` via a typed ``UniqueKey`` token, the partial
``Plan.update(node_id, msg)`` semantics, and ``Plan.delete(NodeType,
node_id)`` with a proto class as the type witness. The test also
exercises the ``UniqueConstraintError`` typed exception path so
that the wire-level translation from ALREADY_EXISTS is covered.

The transport is mocked rather than spinning up a real gRPC server —
the focus here is the SDK shape, not the server. Server-side
correctness is exercised separately by the unit + integration suite
under ``tests/unit/test_unique_keys.py`` and friends.
"""

from __future__ import annotations

from unittest.mock import AsyncMock

import grpc
import pytest

from sdk.entdb_sdk import (
    Actor,
    DbClient,
    Mailbox,
    Public,
    Tenant,
    UniqueConstraintError,
    UniqueKey,
    register_proto_schema,
)
from sdk.entdb_sdk._grpc_client import GrpcCommitResult, GrpcReceipt, Node
from sdk.entdb_sdk.registry import get_registry, reset_registry
from tests._test_schemas import test_schema_pb2 as ts


# ── Codegen-shaped sidecar ──────────────────────────────────────────
#
# In production these tokens come from the ``protoc-gen-entdb-keys``
# plugin in a generated ``test_schema_entdb.py`` file. We hand-build
# the same shape inline here so the test does not depend on running
# the codegen.
class ProductKeys:
    sku = UniqueKey[str](type_id=9001, field_id=1, name="sku")


class CategoryKeys:
    slug = UniqueKey[str](type_id=9002, field_id=1, name="slug")


# ── Fixtures ────────────────────────────────────────────────────────


@pytest.fixture(autouse=True)
def _registered_schema():
    reset_registry()
    register_proto_schema(ts)
    yield
    reset_registry()


def _make_node_response(node_id: str = "n1", payload: dict | None = None) -> Node:
    return Node(
        tenant_id="acme",
        node_id=node_id,
        type_id=9001,
        payload=payload or {"sku": "WIDGET-1", "name": "Widget One", "price_cents": 999},
        created_at=1_700_000_000_000,
        updated_at=1_700_000_000_000,
        owner_actor="user:alice",
        acl=[],
    )


def _success_commit(created_id: str = "n1") -> GrpcCommitResult:
    return GrpcCommitResult(
        success=True,
        receipt=GrpcReceipt(
            tenant_id="acme",
            idempotency_key="k1",
            stream_position="0:0:1",
        ),
        created_node_ids=[created_id],
        applied=True,
        error=None,
    )


@pytest.fixture
def db():
    """A ``DbClient`` with a mocked gRPC transport.

    All methods used in the tests below are mocked to return fixture
    payloads. ``execute_atomic`` is mocked separately per test where
    we need to swap out the response for a duplicate-detection
    scenario.
    """
    client = DbClient("localhost:50051")
    client._connected = True
    client.registry = get_registry()
    client._grpc = AsyncMock()
    client._grpc.execute_atomic = AsyncMock(return_value=_success_commit())
    client._grpc.get_node = AsyncMock(return_value=_make_node_response())
    client._grpc.get_node_by_key = AsyncMock(return_value=_make_node_response())
    return client


# ── 1. The canonical end-to-end flow ────────────────────────────────


class TestCanonicalFlow:
    @pytest.mark.asyncio
    async def test_create_lookup_update_delete(self, db):
        """The single-shape v0.3 API: one method per operation."""
        scope = db.tenant("acme").actor(Actor.user("alice"))

        # Create — only the proto, nothing else.
        plan = scope.plan()
        plan.create(ts.Product(sku="WIDGET-1", name="Widget One", price_cents=999))
        result = await plan.commit(wait_applied=True)
        assert result.success
        node_id = result.created_node_ids[0]

        # Lookup by node id — proto class as the type witness.
        fetched = await scope.get(ts.Product, node_id)
        assert fetched is not None
        assert fetched.payload["sku"] == "WIDGET-1"

        # Lookup by typed unique-key token — codegen sidecar.
        fetched = await scope.get_by_key(ProductKeys.sku, "WIDGET-1")
        assert fetched is not None
        # The mocked transport returns the same Node fixture regardless
        # of the lookup key, but we can verify the SDK called the
        # transport with the right (type_id, field_id, value) triple.
        call_kwargs = db._grpc.get_node_by_key.call_args.kwargs
        assert call_kwargs["type_id"] == 9001
        assert call_kwargs["field_id"] == 1
        assert call_kwargs["value"] == "WIDGET-1"

        # Update — partial: only fields explicitly set on the message
        # become the patch, everything else is left untouched.
        plan2 = scope.plan()
        plan2.update(node_id, ts.Product(price_cents=1499))
        await plan2.commit(wait_applied=True)
        last_call = db._grpc.execute_atomic.call_args
        ops = last_call.kwargs["operations"]
        assert ops[0]["update_node"]["patch"] == {"price_cents": 1499}
        assert ops[0]["update_node"]["type_id"] == 9001
        assert ops[0]["update_node"]["id"] == node_id

        # Delete — type witness via proto class.
        plan3 = scope.plan()
        plan3.delete(ts.Product, node_id)
        await plan3.commit(wait_applied=True)
        last_call = db._grpc.execute_atomic.call_args
        ops = last_call.kwargs["operations"]
        assert ops[0]["delete_node"]["type_id"] == 9001
        assert ops[0]["delete_node"]["id"] == node_id

    @pytest.mark.asyncio
    async def test_duplicate_create_raises_typed_unique_constraint_error(self, db):
        """A duplicate unique value surfaces as ``UniqueConstraintError``.

        The wire path: server sends back a gRPC ``ALREADY_EXISTS``
        with a structured ``"type_id=<int> field_id=<int> value=<repr>
        already exists"`` detail string. The SDK's
        ``GrpcClient.execute_atomic`` parser recovers the integer
        type / field ids and the original Python value into a typed
        exception that callers can catch by class, not by
        string-matching. Here we drive that parser directly by
        mocking the gRPC stub one layer below the SDK boundary, so
        the entire parser path is exercised end to end.
        """
        from sdk.entdb_sdk._grpc_client import GrpcClient

        class _FakeRpcError(grpc.RpcError):
            def code(self):
                return grpc.StatusCode.ALREADY_EXISTS

            def details(self):
                return (
                    "Unique constraint violation: type_id=9001 field_id=1 "
                    "value='WIDGET-1' already exists"
                )

        # Build a real GrpcClient and patch its low-level retry/stub.
        real_grpc = GrpcClient(host="localhost", port=50051)
        real_grpc._stub = AsyncMock()
        real_grpc._retry = AsyncMock(side_effect=_FakeRpcError())
        db._grpc = real_grpc

        scope = db.tenant("acme").actor(Actor.user("alice"))
        plan = scope.plan()
        plan.create(ts.Product(sku="WIDGET-1", name="Dup"))

        with pytest.raises(UniqueConstraintError) as exc_info:
            await plan.commit(wait_applied=True)

        err = exc_info.value
        assert err.tenant_id == "acme"
        assert err.type_id == 9001
        assert err.field_id == 1
        assert err.value == "WIDGET-1"


# ── 2. Storage descriptors are the only way to pick a storage mode ──


class TestStorageDescriptors:
    @pytest.mark.asyncio
    async def test_default_storage_is_tenant(self, db):
        scope = db.tenant("acme").actor(Actor.user("alice"))
        plan = scope.plan()
        plan.create(ts.Product(sku="p1", name="A"))
        # Storage routing fields should NOT be present for default
        # tenant-mode storage — the absence is the signal.
        op = plan._plan._operations[0]["create_node"]
        assert "storage_mode" not in op
        assert "target_user_id" not in op

    @pytest.mark.asyncio
    async def test_mailbox_storage_routes_to_user_db(self, db):
        scope = db.tenant("acme").actor(Actor.user("alice"))
        plan = scope.plan()
        plan.create(
            ts.Product(sku="p1", name="A"),
            storage=Mailbox(user_id="alice"),
        )
        op = plan._plan._operations[0]["create_node"]
        assert op["storage_mode"] == "USER_MAILBOX"
        assert op["target_user_id"] == "alice"

    @pytest.mark.asyncio
    async def test_public_storage_routes_to_public_db(self, db):
        scope = db.tenant("acme").actor(Actor.user("alice"))
        plan = scope.plan()
        plan.create(
            ts.Product(sku="p1", name="A"),
            storage=Public(),
        )
        op = plan._plan._operations[0]["create_node"]
        assert op["storage_mode"] == "PUBLIC"
        assert "target_user_id" not in op

    @pytest.mark.asyncio
    async def test_explicit_tenant_storage_is_default(self, db):
        scope = db.tenant("acme").actor(Actor.user("alice"))
        plan = scope.plan()
        plan.create(ts.Product(sku="p1", name="A"), storage=Tenant())
        op = plan._plan._operations[0]["create_node"]
        assert "storage_mode" not in op


# ── 3. UniqueKey is constructed by codegen, not by the user ────────


class TestUniqueKeyToken:
    def test_token_carries_type_field_and_name(self):
        key = ProductKeys.sku
        assert key.type_id == 9001
        assert key.field_id == 1
        assert key.name == "sku"

    def test_token_is_hashable_and_frozen(self):
        # ``@dataclass(frozen=True)`` — token can be used as a dict key.
        key1 = UniqueKey[str](type_id=9001, field_id=1, name="sku")
        key2 = UniqueKey[str](type_id=9001, field_id=1, name="sku")
        assert key1 == key2
        d = {key1: "value"}
        assert d[key2] == "value"

    @pytest.mark.asyncio
    async def test_get_by_key_rejects_non_unique_key_arg(self, db):
        scope = db.tenant("acme").actor(Actor.user("alice"))
        with pytest.raises(TypeError, match="UniqueKey"):
            # Passing a string instead of a UniqueKey is the exact
            # mistake the v0.3 single-shape API exists to prevent.
            await scope.get_by_key("sku", "WIDGET-1")  # type: ignore[arg-type]


# ── 4. The deleted methods stay deleted ──────────────────────────────


class TestDeletedSurface:
    """Regression tests asserting the v0.2 surface is gone.

    If any of these start passing in the future, the single-shape
    invariant has been violated.
    """

    def test_create_with_acl_is_gone(self):
        from sdk.entdb_sdk.client import Plan

        assert not hasattr(Plan, "create_with_acl")

    def test_create_in_mailbox_is_gone(self):
        from sdk.entdb_sdk.client import Plan

        assert not hasattr(Plan, "create_in_mailbox")

    def test_create_in_public_is_gone(self):
        from sdk.entdb_sdk.client import Plan

        assert not hasattr(Plan, "create_in_public")

    def test_create_with_keys_is_gone(self):
        from sdk.entdb_sdk.client import Plan

        assert not hasattr(Plan, "create_with_keys")

    def test_update_with_keys_is_gone(self):
        from sdk.entdb_sdk.client import Plan

        assert not hasattr(Plan, "update_with_keys")

    def test_plan_create_rejects_dict_payload(self, db):
        from sdk.entdb_sdk.client import Plan

        plan = Plan(db, tenant_id="t", actor="user:a")
        # The old shape — NodeTypeDef + dict — is no longer accepted.
        with pytest.raises(TypeError, match="proto message instance"):
            plan.create({"sku": "p1"})

    def test_plan_delete_rejects_node_type_def(self, db):
        from sdk.entdb_sdk.client import Plan
        from sdk.entdb_sdk.schema import NodeTypeDef
        from sdk.entdb_sdk.schema import field as fld

        plan = Plan(db, tenant_id="t", actor="user:a")
        nt = NodeTypeDef(type_id=99, name="X", fields=(fld(1, "name", "str"),))
        with pytest.raises(TypeError, match="proto message class"):
            plan.delete(nt, "n1")

    def test_plan_update_rejects_dict_patch(self, db):
        from sdk.entdb_sdk.client import Plan

        plan = Plan(db, tenant_id="t", actor="user:a")
        with pytest.raises(TypeError, match="proto message instance"):
            plan.update("n1", {"name": "X"})  # type: ignore[arg-type]
