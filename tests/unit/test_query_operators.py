"""
Unit tests for the MongoDB-style query-operator translator introduced
by the 2026-04-14 SDK v0.3 decision.

Covers:

- All eight scalar / list / pattern operators (``$eq``, ``$ne``, ``$gt``,
  ``$gte``, ``$lt``, ``$lte``, ``$in``, ``$nin``, ``$like``, ``$between``).
- Boolean composition (``$and``, ``$or``) at the top level.
- Field-name → field-id translation via the schema registry.
- Rejection of unknown fields, unknown operators, and malformed shapes.
- SQL-injection safety — parameterisation on values, safe-identifier
  validation on field names, and numeric interpolation on field ids.
- End-to-end execution against a real CanonicalStore with a registered
  schema so operator pushdown actually matches the rows it should.

The tests exercise both the pure translator (``compile_query_filter``)
and the integrated ``CanonicalStore.query_nodes`` path.
"""

from __future__ import annotations

import tempfile

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.apply.query_filter import (
    QueryFilterError,
    compile_query_filter,
)
from dbaas.entdb_server.schema.registry import (
    get_registry,
    reset_registry,
)
from dbaas.entdb_server.schema.types import NodeTypeDef, field

TENANT = "t-qo"
TYPE_PRODUCT = 201


@pytest.fixture
def schema_registry():
    """Register a ``Product`` node type and hand the registry to tests."""
    reset_registry()
    reg = get_registry()
    reg.register_node_type(
        NodeTypeDef(
            type_id=TYPE_PRODUCT,
            name="Product",
            fields=(
                field(1, "sku", "str", unique=True),
                field(2, "name", "str"),
                field(3, "price_cents", "int"),
                field(4, "status", "str"),
                field(5, "tags", "str"),
            ),
        )
    )
    yield reg
    reset_registry()


@pytest.fixture
async def seeded_store(schema_registry):
    """CanonicalStore pre-populated with ten products for pushdown tests."""
    with tempfile.TemporaryDirectory() as tmpdir:
        store = CanonicalStore(tmpdir, wal_mode=False)
        await store.initialize_tenant(TENANT)

        products = [
            {"1": f"sku-{i:02d}", "2": f"Widget {i}", "3": 100 * i, "4": "active"}
            for i in range(1, 11)
        ]
        # Mark products 5 and 6 as archived for $ne / $nin tests.
        products[4]["4"] = "archived"
        products[5]["4"] = "archived"
        for i, p in enumerate(products):
            await store.create_node(
                tenant_id=TENANT,
                type_id=TYPE_PRODUCT,
                payload=p,
                owner_actor="u:test",
                node_id=f"n-{i + 1:02d}",
                created_at=1_000 + i,
            )
        yield store


# ══════════════════════════════════════════════════════════════════
# 1. Pure translator — shape and SQL fragment assertions
# ══════════════════════════════════════════════════════════════════


class TestCompileQueryFilterBasic:
    def test_empty_filter_returns_empty(self, schema_registry):
        sql, params = compile_query_filter(None, TYPE_PRODUCT, schema_registry)
        assert sql == ""
        assert params == []

    def test_scalar_shorthand_equality(self, schema_registry):
        sql, params = compile_query_filter({"status": "active"}, TYPE_PRODUCT, schema_registry)
        assert '$."4"' in sql
        assert " = ?" in sql
        assert params == ["active"]

    def test_scalar_none_is_is_null(self, schema_registry):
        sql, params = compile_query_filter({"status": None}, TYPE_PRODUCT, schema_registry)
        assert "IS NULL" in sql
        assert params == []

    def test_eq_operator_form(self, schema_registry):
        sql, params = compile_query_filter(
            {"status": {"$eq": "active"}}, TYPE_PRODUCT, schema_registry
        )
        assert " = ?" in sql
        assert params == ["active"]

    def test_ne_operator(self, schema_registry):
        sql, params = compile_query_filter(
            {"status": {"$ne": "archived"}}, TYPE_PRODUCT, schema_registry
        )
        assert " != ?" in sql
        assert params == ["archived"]

    def test_gt_gte_lt_lte(self, schema_registry):
        for op, sql_op in [("$gt", ">"), ("$gte", ">="), ("$lt", "<"), ("$lte", "<=")]:
            sql, params = compile_query_filter(
                {"price_cents": {op: 500}}, TYPE_PRODUCT, schema_registry
            )
            assert f" {sql_op} ?" in sql
            assert params == [500]

    def test_in_operator(self, schema_registry):
        sql, params = compile_query_filter(
            {"status": {"$in": ["active", "pending", "queued"]}},
            TYPE_PRODUCT,
            schema_registry,
        )
        assert "IN (?, ?, ?)" in sql
        assert params == ["active", "pending", "queued"]

    def test_nin_operator(self, schema_registry):
        sql, params = compile_query_filter(
            {"status": {"$nin": ["archived"]}},
            TYPE_PRODUCT,
            schema_registry,
        )
        assert "NOT IN (?)" in sql
        assert params == ["archived"]

    def test_like_operator(self, schema_registry):
        sql, params = compile_query_filter(
            {"name": {"$like": "Widget%"}}, TYPE_PRODUCT, schema_registry
        )
        assert " LIKE ?" in sql
        assert params == ["Widget%"]

    def test_between_operator(self, schema_registry):
        sql, params = compile_query_filter(
            {"price_cents": {"$between": [200, 600]}},
            TYPE_PRODUCT,
            schema_registry,
        )
        assert "BETWEEN ? AND ?" in sql
        assert params == [200, 600]


class TestCompileQueryFilterComposition:
    def test_and_top_level(self, schema_registry):
        sql, params = compile_query_filter(
            {
                "$and": [
                    {"status": "active"},
                    {"price_cents": {"$gte": 300}},
                ]
            },
            TYPE_PRODUCT,
            schema_registry,
        )
        assert " AND " in sql
        assert params == ["active", 300]

    def test_or_top_level(self, schema_registry):
        sql, params = compile_query_filter(
            {
                "$or": [
                    {"status": "active"},
                    {"status": "pending"},
                ]
            },
            TYPE_PRODUCT,
            schema_registry,
        )
        assert " OR " in sql
        assert params == ["active", "pending"]

    def test_nested_and_or(self, schema_registry):
        sql, params = compile_query_filter(
            {
                "$or": [
                    {
                        "$and": [
                            {"status": "active"},
                            {"price_cents": {"$gte": 300}},
                        ]
                    },
                    {"status": "pending"},
                ]
            },
            TYPE_PRODUCT,
            schema_registry,
        )
        assert " OR " in sql
        assert " AND " in sql
        assert params == ["active", 300, "pending"]

    def test_multiple_operators_on_one_field(self, schema_registry):
        sql, params = compile_query_filter(
            {"price_cents": {"$gte": 100, "$lt": 500}},
            TYPE_PRODUCT,
            schema_registry,
        )
        # The two operators are joined with AND.
        assert sql.count("?") == 2
        assert params == [100, 500]


# ══════════════════════════════════════════════════════════════════
# 2. Validation failures — rejected shapes
# ══════════════════════════════════════════════════════════════════


class TestQueryFilterRejection:
    def test_unknown_field_rejected(self, schema_registry):
        with pytest.raises(QueryFilterError, match="Unknown field"):
            compile_query_filter({"does_not_exist": 1}, TYPE_PRODUCT, schema_registry)

    def test_unknown_type_rejected(self, schema_registry):
        with pytest.raises(QueryFilterError, match="unknown type_id"):
            compile_query_filter({"name": "x"}, 9_999, schema_registry)

    def test_unknown_operator_rejected(self, schema_registry):
        with pytest.raises(QueryFilterError, match="Unknown operator"):
            compile_query_filter({"name": {"$regex": "foo"}}, TYPE_PRODUCT, schema_registry)

    def test_operator_without_dollar_rejected(self, schema_registry):
        with pytest.raises(QueryFilterError, match="must start with"):
            compile_query_filter({"name": {"eq": "foo"}}, TYPE_PRODUCT, schema_registry)

    def test_empty_operator_dict_rejected(self, schema_registry):
        with pytest.raises(QueryFilterError, match="Empty operator"):
            compile_query_filter({"name": {}}, TYPE_PRODUCT, schema_registry)

    def test_in_requires_list(self, schema_registry):
        with pytest.raises(QueryFilterError, match="non-empty list"):
            compile_query_filter({"status": {"$in": "active"}}, TYPE_PRODUCT, schema_registry)

    def test_in_rejects_empty_list(self, schema_registry):
        with pytest.raises(QueryFilterError, match="non-empty list"):
            compile_query_filter({"status": {"$in": []}}, TYPE_PRODUCT, schema_registry)

    def test_between_requires_pair(self, schema_registry):
        with pytest.raises(QueryFilterError, match=r"\[lo, hi\]"):
            compile_query_filter(
                {"price_cents": {"$between": [1]}},
                TYPE_PRODUCT,
                schema_registry,
            )

    def test_like_requires_string(self, schema_registry):
        with pytest.raises(QueryFilterError, match="string pattern"):
            compile_query_filter({"name": {"$like": 42}}, TYPE_PRODUCT, schema_registry)

    def test_top_level_scalar_operator_rejected(self, schema_registry):
        with pytest.raises(QueryFilterError, match="Top-level operator"):
            compile_query_filter({"$eq": "foo"}, TYPE_PRODUCT, schema_registry)

    def test_and_requires_list(self, schema_registry):
        with pytest.raises(QueryFilterError, match=r"\$and"):
            compile_query_filter(
                {"$and": {"status": "active"}},
                TYPE_PRODUCT,
                schema_registry,
            )


# ══════════════════════════════════════════════════════════════════
# 3. SQL-injection safety
# ══════════════════════════════════════════════════════════════════


class TestSqlInjectionSafety:
    def test_injection_in_field_value_is_parameterised(self, schema_registry):
        """A literal ``'); DROP TABLE nodes; --`` on the value side is
        bound as a SQL parameter — never interpolated into the SQL."""
        sql, params = compile_query_filter(
            {"name": "'); DROP TABLE nodes; --"},
            TYPE_PRODUCT,
            schema_registry,
        )
        assert "DROP TABLE" not in sql
        assert params == ["'); DROP TABLE nodes; --"]

    def test_injection_in_field_name_rejected(self, schema_registry):
        """Field names are resolved through the schema registry so a
        malicious name is rejected before it touches any SQL."""
        with pytest.raises(QueryFilterError, match="Unknown field"):
            compile_query_filter(
                {'"; DROP TABLE nodes; --': "x"},
                TYPE_PRODUCT,
                schema_registry,
            )

    def test_injection_without_registry_rejected(self):
        """In registry-less mode the translator still rejects anything
        that isn't a safe identifier."""
        with pytest.raises(QueryFilterError, match="Invalid field name"):
            compile_query_filter(
                {'"; DROP TABLE nodes; --': "x"},
                TYPE_PRODUCT,
                None,
            )

    def test_digit_key_allowed_without_registry(self):
        """Digit-keyed filters (already-translated field ids) work
        without a registry — this keeps the legacy id-keyed call path
        usable during migrations."""
        sql, params = compile_query_filter({"2": "x"}, TYPE_PRODUCT, None)
        assert '$."2"' in sql
        assert params == ["x"]


# ══════════════════════════════════════════════════════════════════
# 4. End-to-end — real CanonicalStore + real index semantics
# ══════════════════════════════════════════════════════════════════


class TestQueryNodesE2E:
    @pytest.mark.asyncio
    async def test_eq_filter_returns_matching_rows(self, seeded_store):
        results = await seeded_store.query_nodes(
            tenant_id=TENANT,
            type_id=TYPE_PRODUCT,
            filter_json={"status": "active"},
            schema_registry=get_registry(),
            limit=100,
        )
        # 10 products total, 2 archived ⇒ 8 active.
        assert len(results) == 8

    @pytest.mark.asyncio
    async def test_ne_filter(self, seeded_store):
        results = await seeded_store.query_nodes(
            tenant_id=TENANT,
            type_id=TYPE_PRODUCT,
            filter_json={"status": {"$ne": "archived"}},
            schema_registry=get_registry(),
            limit=100,
        )
        assert len(results) == 8

    @pytest.mark.asyncio
    async def test_gte_price(self, seeded_store):
        results = await seeded_store.query_nodes(
            tenant_id=TENANT,
            type_id=TYPE_PRODUCT,
            filter_json={"price_cents": {"$gte": 500}},
            schema_registry=get_registry(),
            limit=100,
        )
        # prices are 100, 200, ..., 1000 — six rows meet >= 500.
        assert len(results) == 6

    @pytest.mark.asyncio
    async def test_between_price(self, seeded_store):
        results = await seeded_store.query_nodes(
            tenant_id=TENANT,
            type_id=TYPE_PRODUCT,
            filter_json={"price_cents": {"$between": [200, 500]}},
            schema_registry=get_registry(),
            limit=100,
        )
        # 200, 300, 400, 500 ⇒ four rows.
        assert len(results) == 4

    @pytest.mark.asyncio
    async def test_in_and_like(self, seeded_store):
        results = await seeded_store.query_nodes(
            tenant_id=TENANT,
            type_id=TYPE_PRODUCT,
            filter_json={
                "$and": [
                    {"status": {"$in": ["active", "queued"]}},
                    {"name": {"$like": "Widget%"}},
                ]
            },
            schema_registry=get_registry(),
            limit=100,
        )
        assert len(results) == 8
        for r in results:
            assert r.payload["4"] == "active"

    @pytest.mark.asyncio
    async def test_or_composition(self, seeded_store):
        results = await seeded_store.query_nodes(
            tenant_id=TENANT,
            type_id=TYPE_PRODUCT,
            filter_json={
                "$or": [
                    {"price_cents": {"$lte": 200}},
                    {"price_cents": {"$gte": 900}},
                ]
            },
            schema_registry=get_registry(),
            limit=100,
        )
        # prices 100, 200, 900, 1000 ⇒ four rows.
        assert len(results) == 4

    @pytest.mark.asyncio
    async def test_unknown_field_raises_at_runtime(self, seeded_store):
        with pytest.raises(QueryFilterError):
            await seeded_store.query_nodes(
                tenant_id=TENANT,
                type_id=TYPE_PRODUCT,
                filter_json={"nope": "value"},
                schema_registry=get_registry(),
            )
