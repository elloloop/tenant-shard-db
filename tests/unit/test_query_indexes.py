"""
Unit tests for declarative non-unique expression indexes via
``(entdb.field).indexed = true``.

Covers:
1. ``_ensure_query_indexes`` creates the expected non-unique indexes.
2. Queries on indexed vs non-indexed fields return identical results.
3. Unique fields get ``idx_unique_t*`` indexes, indexed fields get
   ``idx_query_t*`` indexes — names differ.
4. A field with both ``unique`` and ``indexed`` gets only the unique
   index (unique is a superset).
5. Cache behaviour — second call is a no-op.
6. Idempotency — calling twice produces no errors and one index.
7. ``_ensure_field_indexes`` creates both kinds in one call.
8. ``get_indexed_field_ids`` returns only indexed-not-unique fields.
"""

from __future__ import annotations

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.schema.registry import (
    get_registry,
    reset_registry,
)
from dbaas.entdb_server.schema.types import NodeTypeDef, field

TENANT = "tenant-qi-tests"
TYPE_PRODUCT = 201

# Field ids used in the test schema.
FIELD_SKU = 1
FIELD_NAME = 3
FIELD_PRICE = 4
FIELD_STATUS = 5
FIELD_CATEGORY = 6


# ── Fixtures ────────────────────────────────────────────────────────


@pytest.fixture(autouse=True)
def registered_schema():
    """Register a ``Product`` type with a mix of unique, indexed, and
    plain fields so every test can exercise all combinations.
    """
    reset_registry()
    reg = get_registry()
    product = NodeTypeDef(
        type_id=TYPE_PRODUCT,
        name="Product",
        fields=(
            field(FIELD_SKU, "sku", "str", unique=True),
            field(FIELD_NAME, "name", "str"),
            field(FIELD_PRICE, "price_cents", "int", indexed=True),
            field(FIELD_STATUS, "status", "str", indexed=True),
            field(FIELD_CATEGORY, "category", "str"),
        ),
    )
    reg.register_node_type(product)
    yield reg
    reset_registry()


@pytest.fixture
def store(tmp_path):
    s = CanonicalStore(data_dir=str(tmp_path))
    with s._get_connection(TENANT, create=True) as conn:
        s._create_schema(conn)
    return s


def _get_index_names(conn, table="nodes") -> set[str]:
    """Return the set of user-defined index names on *table*."""
    rows = conn.execute(
        "SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = ?",
        (table,),
    ).fetchall()
    return {r[0] if isinstance(r, tuple) else r["name"] for r in rows}


# ════════════════════════════════════════════════════════════════════
# 1. _ensure_query_indexes creates expected indexes
# ════════════════════════════════════════════════════════════════════


class TestEnsureQueryIndexes:
    def test_creates_non_unique_indexes(self, store):
        with store._get_connection(TENANT) as conn:
            store._ensure_query_indexes(conn, TENANT, TYPE_PRODUCT, [FIELD_PRICE, FIELD_STATUS])
            names = _get_index_names(conn)
        assert f"idx_query_t{TYPE_PRODUCT}_f{FIELD_PRICE}" in names
        assert f"idx_query_t{TYPE_PRODUCT}_f{FIELD_STATUS}" in names

    def test_index_allows_duplicates(self, store):
        """Non-unique indexes must NOT reject duplicate values."""
        with store._get_connection(TENANT) as conn:
            store._ensure_query_indexes(conn, TENANT, TYPE_PRODUCT, [FIELD_PRICE])
            # Insert two rows with the same price — should succeed.
            conn.execute(
                "INSERT INTO nodes (tenant_id, node_id, type_id, payload_json, "
                "owner_actor, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
                (TENANT, "p1", TYPE_PRODUCT, '{"4": 999}', "user:test", 0, 0),
            )
            conn.execute(
                "INSERT INTO nodes (tenant_id, node_id, type_id, payload_json, "
                "owner_actor, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
                (TENANT, "p2", TYPE_PRODUCT, '{"4": 999}', "user:test", 0, 0),
            )
            # Both rows present.
            rows = conn.execute(
                "SELECT node_id FROM nodes WHERE tenant_id = ? AND type_id = ?",
                (TENANT, TYPE_PRODUCT),
            ).fetchall()
            ids = {r[0] if isinstance(r, tuple) else r["node_id"] for r in rows}
        assert ids == {"p1", "p2"}

    def test_empty_field_list_is_noop(self, store):
        with store._get_connection(TENANT) as conn:
            store._ensure_query_indexes(conn, TENANT, TYPE_PRODUCT, [])
            names = _get_index_names(conn)
        assert not any(n.startswith("idx_query_") for n in names)

    def test_introspect_index_via_pragma(self, store):
        """Verify the index exists via PRAGMA index_list."""
        with store._get_connection(TENANT) as conn:
            store._ensure_query_indexes(conn, TENANT, TYPE_PRODUCT, [FIELD_PRICE])
            index_list = conn.execute("PRAGMA index_list(nodes)").fetchall()
        idx_names = [r[1] if isinstance(r, tuple) else r["name"] for r in index_list]
        assert f"idx_query_t{TYPE_PRODUCT}_f{FIELD_PRICE}" in idx_names


# ════════════════════════════════════════════════════════════════════
# 2. Queries on indexed vs non-indexed fields — same results
# ════════════════════════════════════════════════════════════════════


class TestQueryResultsIdentical:
    def _seed(self, store):
        """Insert a few rows for querying."""
        with store._get_connection(TENANT) as conn:
            for i, (price, status, cat) in enumerate(
                [
                    (100, "active", "electronics"),
                    (200, "active", "electronics"),
                    (100, "draft", "books"),
                    (300, "active", "books"),
                ]
            ):
                conn.execute(
                    "INSERT INTO nodes (tenant_id, node_id, type_id, payload_json, "
                    "owner_actor, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
                    (
                        TENANT,
                        f"p{i}",
                        TYPE_PRODUCT,
                        f'{{"{FIELD_PRICE}": {price}, "{FIELD_STATUS}": "{status}", "{FIELD_CATEGORY}": "{cat}"}}',
                        "user:test",
                        0,
                        0,
                    ),
                )

    @pytest.mark.asyncio
    async def test_filter_on_indexed_field(self, store):
        self._seed(store)
        # Create the query index first.
        with store._get_connection(TENANT) as conn:
            store._ensure_query_indexes(conn, TENANT, TYPE_PRODUCT, [FIELD_PRICE])
        # Query on indexed field.
        reg = get_registry()
        nodes_indexed = await store.query_nodes(
            TENANT,
            TYPE_PRODUCT,
            filter_json={"price_cents": 100},
            schema_registry=reg,
        )
        # Query on non-indexed field.
        nodes_plain = await store.query_nodes(
            TENANT,
            TYPE_PRODUCT,
            filter_json={"category": "electronics"},
            schema_registry=reg,
        )
        assert len(nodes_indexed) == 2
        assert len(nodes_plain) == 2

    @pytest.mark.asyncio
    async def test_results_same_with_or_without_index(self, store):
        """Query results must be identical regardless of index existence."""
        self._seed(store)
        reg = get_registry()

        # Query before creating index.
        before = await store.query_nodes(
            TENANT,
            TYPE_PRODUCT,
            filter_json={"status": "active"},
            schema_registry=reg,
        )

        # Create index.
        with store._get_connection(TENANT) as conn:
            store._ensure_query_indexes(conn, TENANT, TYPE_PRODUCT, [FIELD_STATUS])

        # Query after creating index.
        after = await store.query_nodes(
            TENANT,
            TYPE_PRODUCT,
            filter_json={"status": "active"},
            schema_registry=reg,
        )
        assert {n.node_id for n in before} == {n.node_id for n in after}


# ════════════════════════════════════════════════════════════════════
# 3. Unique vs non-unique index names differ
# ════════════════════════════════════════════════════════════════════


class TestIndexNameScheme:
    def test_unique_and_indexed_names_differ(self, store):
        with store._get_connection(TENANT) as conn:
            store._ensure_unique_indexes(conn, TENANT, TYPE_PRODUCT, [FIELD_SKU])
            store._ensure_query_indexes(conn, TENANT, TYPE_PRODUCT, [FIELD_PRICE])
            names = _get_index_names(conn)
        assert f"idx_unique_t{TYPE_PRODUCT}_f{FIELD_SKU}" in names
        assert f"idx_query_t{TYPE_PRODUCT}_f{FIELD_PRICE}" in names
        # No cross-contamination.
        assert f"idx_query_t{TYPE_PRODUCT}_f{FIELD_SKU}" not in names
        assert f"idx_unique_t{TYPE_PRODUCT}_f{FIELD_PRICE}" not in names


# ════════════════════════════════════════════════════════════════════
# 4. unique=true superset — no redundant query index
# ════════════════════════════════════════════════════════════════════


class TestUniqueSupersetOfIndexed:
    def test_unique_field_excluded_from_indexed_list(self):
        """``get_indexed_field_ids`` must NOT return fields that already
        have ``unique = true`` — the unique index serves double duty."""
        reg = get_registry()
        result = reg.get_indexed_field_ids(TYPE_PRODUCT)
        # FIELD_SKU is unique — must be absent.
        assert FIELD_SKU not in result
        # FIELD_PRICE and FIELD_STATUS are indexed-only.
        assert FIELD_PRICE in result
        assert FIELD_STATUS in result

    def test_field_with_both_flags_gets_only_unique_index(self):
        """If a field has both ``unique=True`` and ``indexed=True`` the
        schema registry filters it to unique-only, so only
        ``idx_unique_*`` is created — no redundant ``idx_query_*``."""
        reset_registry()
        reg = get_registry()
        both = NodeTypeDef(
            type_id=999,
            name="Both",
            fields=(
                field(1, "code", "str", unique=True, indexed=True),
                field(2, "label", "str"),
            ),
        )
        reg.register_node_type(both)
        assert reg.get_unique_field_ids(999) == [1]
        assert reg.get_indexed_field_ids(999) == []


# ════════════════════════════════════════════════════════════════════
# 5. Cache — second call is a no-op
# ════════════════════════════════════════════════════════════════════


class TestQueryIndexCache:
    def test_second_call_is_noop_via_cache(self, store):
        with store._get_connection(TENANT) as conn:
            store._ensure_query_indexes(conn, TENANT, TYPE_PRODUCT, [FIELD_PRICE])
            assert store._query_index_cache, "expected cache entry"

            executed: list[str] = []

            class _SpyConn:
                def __init__(self, wrapped):
                    self._wrapped = wrapped

                def execute(self, sql, *args, **kwargs):
                    executed.append(sql)
                    return self._wrapped.execute(sql, *args, **kwargs)

            spy = _SpyConn(conn)
            store._ensure_query_indexes(spy, TENANT, TYPE_PRODUCT, [FIELD_PRICE])
        assert not any("CREATE INDEX" in s.upper() for s in executed), (
            f"second call should be a cache hit, but issued: {executed}"
        )


# ════════════════════════════════════════════════════════════════════
# 6. Idempotency — call twice, no error, one index
# ════════════════════════════════════════════════════════════════════


class TestQueryIndexIdempotency:
    def test_restart_safe_via_if_not_exists(self, store):
        with store._get_connection(TENANT) as conn:
            store._ensure_query_indexes(conn, TENANT, TYPE_PRODUCT, [FIELD_PRICE])

        # Simulate restart: clear the cache.
        store._query_index_cache.clear()

        with store._get_connection(TENANT) as conn:
            # Must not raise.
            store._ensure_query_indexes(conn, TENANT, TYPE_PRODUCT, [FIELD_PRICE])
            rows = conn.execute(
                "SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?",
                (f"idx_query_t{TYPE_PRODUCT}_f{FIELD_PRICE}",),
            ).fetchone()
        count = rows[0] if isinstance(rows, tuple) else rows["COUNT(*)"]
        assert count == 1


# ════════════════════════════════════════════════════════════════════
# 7. _ensure_field_indexes creates both kinds
# ════════════════════════════════════════════════════════════════════


class TestEnsureFieldIndexes:
    def test_creates_both_unique_and_query_indexes(self, store):
        with store._get_connection(TENANT) as conn:
            store._ensure_field_indexes(
                conn,
                TENANT,
                TYPE_PRODUCT,
                unique_field_ids=[FIELD_SKU],
                indexed_field_ids=[FIELD_PRICE, FIELD_STATUS],
            )
            names = _get_index_names(conn)
        assert f"idx_unique_t{TYPE_PRODUCT}_f{FIELD_SKU}" in names
        assert f"idx_query_t{TYPE_PRODUCT}_f{FIELD_PRICE}" in names
        assert f"idx_query_t{TYPE_PRODUCT}_f{FIELD_STATUS}" in names

    def test_empty_lists_are_noop(self, store):
        with store._get_connection(TENANT) as conn:
            store._ensure_field_indexes(conn, TENANT, TYPE_PRODUCT, [], [])
            names = _get_index_names(conn)
        assert not any(n.startswith("idx_unique_") or n.startswith("idx_query_") for n in names)


# ════════════════════════════════════════════════════════════════════
# 8. Schema registry — get_indexed_field_ids
# ════════════════════════════════════════════════════════════════════


class TestGetIndexedFieldIds:
    def test_returns_indexed_non_unique_non_deprecated(self):
        reg = get_registry()
        result = reg.get_indexed_field_ids(TYPE_PRODUCT)
        assert sorted(result) == sorted([FIELD_PRICE, FIELD_STATUS])

    def test_excludes_deprecated_fields(self):
        reset_registry()
        reg = get_registry()
        dep = NodeTypeDef(
            type_id=300,
            name="Dep",
            fields=(
                field(1, "a", "str", indexed=True, deprecated=True),
                field(2, "b", "str", indexed=True),
            ),
        )
        reg.register_node_type(dep)
        assert reg.get_indexed_field_ids(300) == [2]

    def test_unknown_type_returns_empty(self):
        reg = get_registry()
        assert reg.get_indexed_field_ids(99999) == []
