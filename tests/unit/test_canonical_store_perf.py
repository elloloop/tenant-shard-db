"""
Performance-related tests for CanonicalStore.

Covers:
- ThreadPoolExecutor max_workers default and override
- query_nodes SQL-pushdown for payload filtering
- SQL injection safety
"""

import os

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore

TENANT = "perf-test-tenant"
TYPE_ID = 1


@pytest.fixture()
def store(tmp_path):
    """Create a CanonicalStore backed by a temp directory."""
    return CanonicalStore(str(tmp_path))


@pytest.fixture()
async def seeded_store(store):
    """Store with 120 nodes seeded (field_id-based payloads)."""
    await store.initialize_tenant(TENANT)
    for i in range(120):
        # Payloads use stringified field-ids as keys, matching EntDB v2 storage
        payload = {
            "1": f"name-{i}",  # field_id 1 = name
            "2": f"category-{i % 5}",  # field_id 2 = category (5 distinct values)
            "3": i,  # field_id 3 = rank
        }
        await store.create_node(
            tenant_id=TENANT,
            type_id=TYPE_ID,
            payload=payload,
            owner_actor="user:seed",
        )
    return store


# ── Fix 1: max_workers tests ─────────────────────────────────────────


class TestMaxWorkers:
    def test_default_max_workers_greater_than_one(self):
        """DEFAULT_MAX_WORKERS must be > 1 to avoid single-thread bottleneck."""
        assert CanonicalStore.DEFAULT_MAX_WORKERS > 1

    def test_default_max_workers_formula(self):
        """DEFAULT_MAX_WORKERS matches the stdlib I/O-bound formula."""
        expected = min(32, (os.cpu_count() or 4) + 4)
        assert expected == CanonicalStore.DEFAULT_MAX_WORKERS

    def test_max_workers_kwarg_respected(self, tmp_path):
        """Passing max_workers overrides the default."""
        store = CanonicalStore(str(tmp_path), max_workers=7)
        assert store._max_workers == 7
        assert store._executor._max_workers == 7

    def test_default_kwarg_uses_class_default(self, tmp_path):
        """Omitting max_workers uses DEFAULT_MAX_WORKERS."""
        store = CanonicalStore(str(tmp_path))
        assert store._max_workers == CanonicalStore.DEFAULT_MAX_WORKERS


# ── Fix 2: query_nodes SQL-pushdown tests ────────────────────────────


class TestQueryNodesSQLPushdown:
    @pytest.mark.asyncio
    async def test_filter_returns_matching_nodes(self, seeded_store):
        """Filtering by a field value returns only matching nodes."""
        results = await seeded_store.query_nodes(
            TENANT,
            TYPE_ID,
            filter_json={"2": "category-0"},
            limit=200,
        )
        assert len(results) == 24  # 120 / 5 categories
        for node in results:
            assert node.payload["2"] == "category-0"

    @pytest.mark.asyncio
    async def test_none_filter_returns_all(self, seeded_store):
        """None filter returns all nodes (up to limit)."""
        results = await seeded_store.query_nodes(
            TENANT,
            TYPE_ID,
            filter_json=None,
            limit=200,
        )
        assert len(results) == 120

    @pytest.mark.asyncio
    async def test_filter_unknown_field_returns_empty(self, seeded_store):
        """Filtering on a field that no node has returns empty list."""
        results = await seeded_store.query_nodes(
            TENANT,
            TYPE_ID,
            filter_json={"999": "nonexistent"},
        )
        assert results == []

    @pytest.mark.asyncio
    async def test_filter_multiple_keys_and_semantics(self, seeded_store):
        """Multiple filter keys use AND semantics."""
        results = await seeded_store.query_nodes(
            TENANT,
            TYPE_ID,
            filter_json={"1": "name-0", "2": "category-0"},
            limit=200,
        )
        assert len(results) == 1
        assert results[0].payload["1"] == "name-0"
        assert results[0].payload["2"] == "category-0"

    @pytest.mark.asyncio
    async def test_sql_injection_safety(self, seeded_store):
        """SQL injection in filter values must not break the query."""
        malicious_value = "'; DROP TABLE nodes; --"
        results = await seeded_store.query_nodes(
            TENANT,
            TYPE_ID,
            filter_json={"1": malicious_value},
        )
        # No crash, no results (the injection value matches no node)
        assert results == []
        # Table must still exist — verify by running a normal query
        all_nodes = await seeded_store.query_nodes(
            TENANT,
            TYPE_ID,
            filter_json=None,
            limit=1,
        )
        assert len(all_nodes) == 1

    @pytest.mark.asyncio
    async def test_sql_injection_in_key(self, seeded_store):
        """SQL injection in filter *keys* must be rejected, not executed.

        Under the 2026-04-14 SDK v0.3 query-operator design the
        filter translator validates field names against the schema
        registry (or, in registry-less mode, a safe-identifier regex)
        before they touch the generated SQL. A malicious key is
        rejected with ``QueryFilterError`` rather than silently
        returning no rows.
        """
        from dbaas.entdb_server.apply.query_filter import QueryFilterError

        malicious_key = '"; DROP TABLE nodes; --'
        with pytest.raises(QueryFilterError):
            await seeded_store.query_nodes(
                TENANT,
                TYPE_ID,
                filter_json={malicious_key: "x"},
            )
        # Table must still exist.
        all_nodes = await seeded_store.query_nodes(
            TENANT,
            TYPE_ID,
            filter_json=None,
            limit=1,
        )
        assert len(all_nodes) == 1

    @pytest.mark.asyncio
    async def test_filter_respects_limit_and_offset(self, seeded_store):
        """SQL-pushdown correctly applies LIMIT and OFFSET."""
        # category-0 has 24 nodes; grab page 2 with page_size=10
        page = await seeded_store.query_nodes(
            TENANT,
            TYPE_ID,
            filter_json={"2": "category-0"},
            limit=10,
            offset=10,
        )
        assert len(page) == 10

    @pytest.mark.asyncio
    async def test_filter_with_none_value(self, seeded_store):
        """Filtering for a key whose value is None returns no nodes
        (all seeded nodes have non-null values for every key)."""
        results = await seeded_store.query_nodes(
            TENANT,
            TYPE_ID,
            filter_json={"1": None},
        )
        assert results == []
