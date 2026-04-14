"""Regression tests for Plan double-commit guard and FieldDef type validation.

Issue 1: Plan.commit() could be called multiple times, replaying all
         operations a second time. Plan.commit() now raises
         ``RuntimeError`` on a second call.

Issue 2: FieldDef.validate_value() previously skipped type checks.
         The type checks are still exercised here directly against
         FieldDef, since the SDK v0.3 ``Plan.create`` API takes proto
         messages whose typed fields make most "wrong type" cases
         impossible at the call site (the proto runtime rejects the
         construction itself).
"""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock

import pytest

from sdk.entdb_sdk import register_proto_schema
from sdk.entdb_sdk._grpc_client import GrpcCommitResult, GrpcReceipt
from sdk.entdb_sdk.client import DbClient, Plan
from sdk.entdb_sdk.errors import ValidationError
from sdk.entdb_sdk.registry import get_registry, reset_registry
from sdk.entdb_sdk.schema import FieldDef, FieldKind
from tests._test_schemas import test_schema_pb2 as ts

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _field(fid: int, name: str, kind_str: str, **kwargs) -> FieldDef:
    kind_map = {
        "str": FieldKind.STRING,
        "int": FieldKind.INTEGER,
        "float": FieldKind.FLOAT,
        "bool": FieldKind.BOOLEAN,
        "timestamp": FieldKind.TIMESTAMP,
        "enum": FieldKind.ENUM,
        "json": FieldKind.JSON,
        "bytes": FieldKind.BYTES,
        "ref": FieldKind.REFERENCE,
        "list_str": FieldKind.LIST_STRING,
        "list_int": FieldKind.LIST_INT,
        "list_ref": FieldKind.LIST_REF,
    }
    return FieldDef(field_id=fid, name=name, kind=kind_map[kind_str], **kwargs)


@pytest.fixture(autouse=True)
def _registered_schema():
    """Register the test_schema proto types for the duration of each test."""
    reset_registry()
    register_proto_schema(ts)
    yield
    reset_registry()


@pytest.fixture
def mock_grpc_client():
    client = MagicMock()
    client._host = "localhost"
    client._port = 50051
    client.connect = AsyncMock()
    client.close = AsyncMock()
    client.execute_atomic = AsyncMock(
        return_value=GrpcCommitResult(
            success=True,
            receipt=GrpcReceipt(tenant_id="t1", idempotency_key="k1", stream_position="1"),
            created_node_ids=["node_1"],
            applied=True,
            error=None,
        )
    )
    return client


def _make_db(mock_grpc):
    db = DbClient.__new__(DbClient)
    db._grpc = mock_grpc
    db._connected = True
    db.registry = get_registry()
    db._last_offsets = {}
    db._schema_fingerprint = None
    return db


# ---------------------------------------------------------------------------
# Issue 1: Plan double-commit guard
# ---------------------------------------------------------------------------


class TestPlanDoubleCommitGuard:
    """Plan.commit() must raise RuntimeError when called a second time."""

    @pytest.mark.asyncio
    async def test_second_commit_raises(self, mock_grpc_client):
        db = _make_db(mock_grpc_client)
        plan = Plan(db, "t1", "user:1")
        plan.create(ts.Product(sku="p1", name="One", price_cents=100))

        result = await plan.commit()
        assert result.success

        with pytest.raises(RuntimeError, match="already been committed"):
            await plan.commit()

    @pytest.mark.asyncio
    async def test_empty_plan_double_commit_raises(self, mock_grpc_client):
        db = _make_db(mock_grpc_client)
        plan = Plan(db, "t1", "user:1")

        result = await plan.commit()
        assert result.success

        with pytest.raises(RuntimeError, match="already been committed"):
            await plan.commit()

    @pytest.mark.asyncio
    async def test_create_after_commit_raises(self, mock_grpc_client):
        db = _make_db(mock_grpc_client)
        plan = Plan(db, "t1", "user:1")
        plan.create(ts.Product(sku="p1", name="One", price_cents=100))
        await plan.commit()

        with pytest.raises(RuntimeError, match="already been committed"):
            plan.create(ts.Product(sku="p2", name="Two", price_cents=200))

    @pytest.mark.asyncio
    async def test_update_after_commit_raises(self, mock_grpc_client):
        db = _make_db(mock_grpc_client)
        plan = Plan(db, "t1", "user:1")
        plan.create(ts.Product(sku="p1", name="One", price_cents=100))
        await plan.commit()

        with pytest.raises(RuntimeError, match="already been committed"):
            plan.update("node_1", ts.Product(name="Updated"))

    @pytest.mark.asyncio
    async def test_delete_after_commit_raises(self, mock_grpc_client):
        db = _make_db(mock_grpc_client)
        plan = Plan(db, "t1", "user:1")
        plan.create(ts.Product(sku="p1", name="One", price_cents=100))
        await plan.commit()

        with pytest.raises(RuntimeError, match="already been committed"):
            plan.delete(ts.Product, "node_1")

    @pytest.mark.asyncio
    async def test_edge_create_after_commit_raises(self, mock_grpc_client):
        db = _make_db(mock_grpc_client)
        plan = Plan(db, "t1", "user:1")
        plan.create(ts.Product(sku="p1", name="One", price_cents=100))
        await plan.commit()

        with pytest.raises(RuntimeError, match="already been committed"):
            plan.edge_create(ts.BelongsTo, "node_1", "node_2")

    @pytest.mark.asyncio
    async def test_edge_delete_after_commit_raises(self, mock_grpc_client):
        db = _make_db(mock_grpc_client)
        plan = Plan(db, "t1", "user:1")
        plan.create(ts.Product(sku="p1", name="One", price_cents=100))
        await plan.commit()

        with pytest.raises(RuntimeError, match="already been committed"):
            plan.edge_delete(ts.BelongsTo, "node_1", "node_2")

    @pytest.mark.asyncio
    async def test_first_commit_still_works(self, mock_grpc_client):
        """Ensure the guard doesn't break normal single-commit usage."""
        db = _make_db(mock_grpc_client)
        plan = Plan(db, "t1", "user:1")
        plan.create(ts.Product(sku="p1", name="One", price_cents=100))

        result = await plan.commit()
        assert result.success
        assert result.created_node_ids == ["node_1"]
        mock_grpc_client.execute_atomic.assert_awaited_once()


# ---------------------------------------------------------------------------
# Issue 2: FieldDef.validate_value() type checking (still standalone)
# ---------------------------------------------------------------------------


class TestFieldDefTypeValidation:
    """FieldDef.validate_value() must reject wrong types."""

    def test_string_rejects_int(self):
        f = _field(1, "name", "str")
        ok, err = f.validate_value(123)
        assert not ok
        assert "string" in err

    def test_string_accepts_string(self):
        f = _field(1, "name", "str")
        ok, err = f.validate_value("hello")
        assert ok

    def test_int_rejects_string(self):
        f = _field(1, "count", "int")
        ok, err = f.validate_value("five")
        assert not ok
        assert "integer" in err

    def test_int_rejects_bool(self):
        f = _field(1, "count", "int")
        ok, err = f.validate_value(True)
        assert not ok
        assert "integer" in err

    def test_int_accepts_int(self):
        f = _field(1, "count", "int")
        ok, err = f.validate_value(42)
        assert ok

    def test_float_rejects_string(self):
        f = _field(1, "score", "float")
        ok, err = f.validate_value("3.14")
        assert not ok
        assert "number" in err

    def test_float_rejects_bool(self):
        f = _field(1, "score", "float")
        ok, err = f.validate_value(True)
        assert not ok

    def test_float_accepts_int(self):
        f = _field(1, "score", "float")
        ok, err = f.validate_value(100)
        assert ok

    def test_float_accepts_float(self):
        f = _field(1, "score", "float")
        ok, err = f.validate_value(3.14)
        assert ok

    def test_bool_rejects_int(self):
        f = _field(1, "active", "bool")
        ok, err = f.validate_value(1)
        assert not ok
        assert "boolean" in err

    def test_bool_rejects_string(self):
        f = _field(1, "active", "bool")
        ok, err = f.validate_value("true")
        assert not ok

    def test_bool_accepts_bool(self):
        f = _field(1, "active", "bool")
        ok, err = f.validate_value(False)
        assert ok

    def test_timestamp_rejects_negative(self):
        f = _field(1, "ts", "timestamp")
        ok, err = f.validate_value(-1)
        assert not ok
        assert "timestamp" in err

    def test_timestamp_rejects_string(self):
        f = _field(1, "ts", "timestamp")
        ok, err = f.validate_value("2024-01-01")
        assert not ok

    def test_timestamp_accepts_zero(self):
        f = _field(1, "ts", "timestamp")
        ok, err = f.validate_value(0)
        assert ok

    def test_timestamp_accepts_positive(self):
        f = _field(1, "ts", "timestamp")
        ok, err = f.validate_value(1700000000)
        assert ok

    def test_enum_rejects_int(self):
        f = _field(1, "status", "enum", enum_values=("a", "b"))
        ok, err = f.validate_value(1)
        assert not ok
        assert "string" in err

    def test_enum_rejects_invalid_value(self):
        f = _field(1, "status", "enum", enum_values=("a", "b"))
        ok, err = f.validate_value("c")
        assert not ok

    def test_enum_accepts_valid(self):
        f = _field(1, "status", "enum", enum_values=("a", "b"))
        ok, err = f.validate_value("a")
        assert ok

    def test_json_rejects_string(self):
        f = _field(1, "meta", "json")
        ok, err = f.validate_value("not json")
        assert not ok
        assert "dict or list" in err

    def test_json_accepts_dict(self):
        f = _field(1, "meta", "json")
        ok, err = f.validate_value({"key": "val"})
        assert ok

    def test_json_accepts_list(self):
        f = _field(1, "meta", "json")
        ok, err = f.validate_value([1, 2, 3])
        assert ok

    def test_ref_rejects_string(self):
        f = _field(1, "owner", "ref")
        ok, err = f.validate_value("user:1")
        assert not ok

    def test_ref_rejects_missing_keys(self):
        f = _field(1, "owner", "ref")
        ok, err = f.validate_value({"type_id": 1})
        assert not ok
        assert "type_id" in err and "id" in err

    def test_ref_accepts_valid(self):
        f = _field(1, "owner", "ref")
        ok, err = f.validate_value({"type_id": 1, "id": "u1"})
        assert ok

    def test_list_str_rejects_non_list(self):
        f = _field(1, "tags", "list_str")
        ok, err = f.validate_value("tag1,tag2")
        assert not ok

    def test_list_str_rejects_int_element(self):
        f = _field(1, "tags", "list_str")
        ok, err = f.validate_value(["ok", 123])
        assert not ok
        assert "tags[1]" in err

    def test_list_str_accepts_valid(self):
        f = _field(1, "tags", "list_str")
        ok, err = f.validate_value(["a", "b"])
        assert ok

    def test_list_int_rejects_bool_element(self):
        f = _field(1, "nums", "list_int")
        ok, err = f.validate_value([1, True])
        assert not ok

    def test_list_int_accepts_valid(self):
        f = _field(1, "nums", "list_int")
        ok, err = f.validate_value([1, 2, 3])
        assert ok

    def test_none_allowed_when_not_required(self):
        f = _field(1, "name", "str")
        ok, err = f.validate_value(None)
        assert ok

    def test_none_rejected_when_required(self):
        f = _field(1, "name", "str", required=True)
        ok, err = f.validate_value(None)
        assert not ok
        assert "required" in err


class TestPlanCreateValidation:
    """SDK v0.3 ``Plan.create`` enforces typing at the proto layer.

    Most "wrong scalar type" mistakes are now impossible to express in
    user code — the proto class constructor rejects them with
    ``TypeError`` before the SDK ever sees them. We still verify that
    the SDK rejects malformed inputs that *do* survive proto
    construction (e.g. an enum value not in the declared set).
    """

    @pytest.mark.asyncio
    async def test_proto_rejects_int_for_string_field(self, mock_grpc_client):
        # Constructing the proto with the wrong scalar fails before
        # the SDK is involved at all. This is the v0.3 invariant: the
        # type system catches type errors, not the SDK.
        with pytest.raises(TypeError):
            ts.Product(sku=12345)  # sku is a string field

    @pytest.mark.asyncio
    async def test_create_rejects_invalid_enum_value(self, mock_grpc_client):
        db = _make_db(mock_grpc_client)
        plan = Plan(db, "t1", "user:1")
        # ``status`` has enum_values draft/active/archived. The proto
        # itself stores any string, so the SDK's NodeTypeDef enum
        # validation is what catches "not-a-status".
        with pytest.raises(ValidationError):
            plan.create(ts.Product(sku="p1", status="not-a-status"))

    @pytest.mark.asyncio
    async def test_create_accepts_valid_payload(self, mock_grpc_client):
        db = _make_db(mock_grpc_client)
        plan = Plan(db, "t1", "user:1")

        plan.create(ts.Product(sku="p1", name="My Product", price_cents=100, status="active"))
        assert len(plan._operations) == 1

    @pytest.mark.asyncio
    async def test_validate_payload_catches_wrong_types(self):
        """NodeTypeDef.validate_payload still catches type errors when
        called directly with a hand-rolled dict (e.g. by codegen tools)."""
        from sdk.entdb_sdk.schema import NodeTypeDef
        from sdk.entdb_sdk.schema import field as fld

        nt = NodeTypeDef(
            type_id=99,
            name="X",
            fields=(
                fld(1, "title", "str"),
                fld(2, "done", "bool"),
            ),
        )
        ok, errors = nt.validate_payload({"title": 999, "done": "nope"})
        assert not ok
        assert any("string" in e for e in errors)
        assert any("boolean" in e for e in errors)
