"""
Tests for bug fixes: path traversal, _check_tenant await, health check,
query_nodes pagination, _run_sync delegation, and rate limiter bounds.
"""

import tempfile
from unittest.mock import AsyncMock, MagicMock

import grpc
import pytest

from dbaas.entdb_server.api.rate_limiter import RateLimitInterceptor
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.storage.local_fs import LocalFsObjectStore

# ── Fix 1: Path traversal ────────────────────────────────────────────


class TestPathTraversal:
    """LocalFsObjectStore must reject keys that escape the base directory."""

    @pytest.fixture
    def store(self, tmp_path):
        s = LocalFsObjectStore(str(tmp_path))
        return s

    @pytest.mark.asyncio
    async def test_path_traversal_put_blocked(self, store):
        with pytest.raises(ValueError, match="Path traversal detected"):
            await store.put("../../etc/passwd", b"evil")

    @pytest.mark.asyncio
    async def test_path_traversal_get_blocked(self, store):
        with pytest.raises(ValueError, match="Path traversal detected"):
            await store.get("../../../etc/shadow")

    @pytest.mark.asyncio
    async def test_path_traversal_list_blocked(self, store):
        with pytest.raises(ValueError, match="Path traversal detected"):
            await store.list_objects("../../")

    @pytest.mark.asyncio
    async def test_path_traversal_normal_key_works(self, store):
        await store.connect()
        await store.put("tenant/data.bin", b"hello")
        data = await store.get("tenant/data.bin")
        assert data == b"hello"

        items = await store.list_objects("tenant/")
        assert len(items) == 1
        assert items[0].key == "tenant/data.bin"


# ── Fix 2: _check_tenant await ──────────────────────────────────────


class TestCheckTenant:
    """_check_tenant must await context.abort() in grpc.aio."""

    @pytest.mark.asyncio
    async def test_check_tenant_rejects_unassigned(self):
        from dbaas.entdb_server.api.grpc_server import EntDBServicer
        from dbaas.entdb_server.sharding import ShardingConfig

        sharding = ShardingConfig(
            node_id="node-a",
            assigned_tenants=frozenset({"tenant-a"}),
        )

        servicer = EntDBServicer(
            wal=MagicMock(),
            canonical_store=MagicMock(),
            mailbox_store=MagicMock(),
            schema_registry=MagicMock(),
            sharding=sharding,
        )

        context = AsyncMock()
        context.abort = AsyncMock()

        await servicer._check_tenant("tenant-b", context)

        context.abort.assert_awaited_once()
        args = context.abort.call_args
        assert args[0][0] == grpc.StatusCode.UNAVAILABLE
        assert "tenant-b" in args[0][1]

    @pytest.mark.asyncio
    async def test_check_tenant_allows_assigned(self):
        from dbaas.entdb_server.api.grpc_server import EntDBServicer
        from dbaas.entdb_server.sharding import ShardingConfig

        sharding = ShardingConfig(
            node_id="node-a",
            assigned_tenants=frozenset({"tenant-a"}),
        )

        servicer = EntDBServicer(
            wal=MagicMock(),
            canonical_store=MagicMock(),
            mailbox_store=MagicMock(),
            schema_registry=MagicMock(),
            sharding=sharding,
        )

        context = AsyncMock()
        context.abort = AsyncMock()

        await servicer._check_tenant("tenant-a", context)

        context.abort.assert_not_awaited()


# ── Fix 3: Health healthy in multi-node ──────────────────────────────


class TestHealthMultiNode:
    """Health RPC must not treat info keys (node_id, assigned_tenants) as unhealthy."""

    @pytest.mark.asyncio
    async def test_health_healthy_in_multi_node(self):
        from dbaas.entdb_server.api.grpc_server import EntDBServicer
        from dbaas.entdb_server.sharding import ShardingConfig

        sharding = ShardingConfig(
            node_id="node-a",
            assigned_tenants=frozenset({"tenant-a", "tenant-b"}),
        )

        wal = MagicMock()
        wal.is_connected = True

        servicer = EntDBServicer(
            wal=wal,
            canonical_store=MagicMock(),
            mailbox_store=MagicMock(),
            schema_registry=MagicMock(),
            sharding=sharding,
        )

        from dbaas.entdb_server.api.generated import HealthRequest

        context = AsyncMock()
        response = await servicer.Health(HealthRequest(), context)

        assert response.healthy is True
        assert response.components["node_id"] == "node-a"
        assert response.components["wal"] == "healthy"
        assert response.components["storage"] == "healthy"


# ── Fix 4: query_nodes filter + pagination ───────────────────────────


class TestQueryNodesFilterPagination:
    """Filter must be applied BEFORE pagination so limit/offset work correctly."""

    @pytest.fixture
    def store(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            yield CanonicalStore(tmpdir, wal_mode=False)

    @pytest.mark.asyncio
    async def test_query_nodes_filter_with_pagination(self, store):
        tenant = "t1"
        await store.initialize_tenant(tenant)

        # Create 10 nodes: 5 with status=active, 5 with status=inactive
        for i in range(10):
            status = "active" if i % 2 == 0 else "inactive"
            await store.create_node(
                tenant_id=tenant,
                type_id=1,
                payload={"status": status, "idx": i},
                owner_actor="user:test",
                created_at=1000 + i,
            )

        # Query with filter + limit=2, offset=0 — should get first 2 active nodes
        results = await store.query_nodes(
            tenant_id=tenant,
            type_id=1,
            filter_json={"status": "active"},
            limit=2,
            offset=0,
            descending=False,
        )
        assert len(results) == 2
        for r in results:
            assert r.payload["status"] == "active"

        # Query with filter + limit=2, offset=2 — should get next 2 active nodes
        results2 = await store.query_nodes(
            tenant_id=tenant,
            type_id=1,
            filter_json={"status": "active"},
            limit=2,
            offset=2,
            descending=False,
        )
        assert len(results2) == 2
        for r in results2:
            assert r.payload["status"] == "active"

        # No overlap between pages
        ids_page1 = {r.node_id for r in results}
        ids_page2 = {r.node_id for r in results2}
        assert ids_page1.isdisjoint(ids_page2)

    @pytest.mark.asyncio
    async def test_query_nodes_filter_returns_correct_count(self, store):
        tenant = "t2"
        await store.initialize_tenant(tenant)

        # Create 6 nodes: 2 matching, 4 non-matching
        for i in range(6):
            color = "red" if i < 2 else "blue"
            await store.create_node(
                tenant_id=tenant,
                type_id=1,
                payload={"color": color},
                owner_actor="user:test",
            )

        # With large limit, should get exactly 2 matching nodes
        results = await store.query_nodes(
            tenant_id=tenant,
            type_id=1,
            filter_json={"color": "red"},
            limit=100,
            offset=0,
        )
        assert len(results) == 2


# ── Fix 5: tenant_exists uses executor ───────────────────────────────


class TestTenantExistsUsesExecutor:
    """Async methods must delegate to _run_sync to avoid blocking the event loop."""

    @pytest.mark.asyncio
    async def test_tenant_exists_uses_executor(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            store = CanonicalStore(tmpdir, wal_mode=False)
            # Spy on _run_sync to verify it's called
            original_run_sync = store._run_sync

            call_log = []

            async def tracking_run_sync(fn, *args):
                call_log.append(fn.__name__)
                return await original_run_sync(fn, *args)

            store._run_sync = tracking_run_sync

            result = await store.tenant_exists("nonexistent")
            assert result is False
            assert "_sync_tenant_exists" in call_log

    @pytest.mark.asyncio
    async def test_get_stats_uses_executor(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            store = CanonicalStore(tmpdir, wal_mode=False)
            await store.initialize_tenant("t1")

            original_run_sync = store._run_sync
            call_log = []

            async def tracking_run_sync(fn, *args):
                call_log.append(fn.__name__)
                return await original_run_sync(fn, *args)

            store._run_sync = tracking_run_sync

            stats = await store.get_stats("t1")
            assert "_sync_get_stats" in call_log
            assert "nodes" in stats


# ── Fix 6: Rate limiter bounded memory ──────────────────────────────


class TestRateLimiterMaxTenants:
    """Rate limiter must evict oldest buckets when max_tenants is exceeded."""

    def _make_details(self, method, tenant_id=None):
        details = MagicMock()
        details.method = method
        metadata = []
        if tenant_id:
            metadata.append(("x-tenant-id", tenant_id))
        details.invocation_metadata = metadata
        return details

    def test_rate_limiter_max_tenants_bounded(self):
        interceptor = RateLimitInterceptor(rate=100.0, burst=10, max_tenants=5)
        continuation = MagicMock(return_value="handler")

        # Add 5 tenants
        for i in range(5):
            details = self._make_details("/entdb.EntDBService/GetNode", f"tenant-{i}")
            interceptor.intercept_service(continuation, details)

        assert len(interceptor._buckets) == 5

        # Adding a 6th tenant should evict the oldest
        details = self._make_details("/entdb.EntDBService/GetNode", "tenant-new")
        interceptor.intercept_service(continuation, details)

        assert len(interceptor._buckets) <= 5
        assert "tenant-new" in interceptor._buckets
        # tenant-0 (oldest) should have been evicted
        assert "tenant-0" not in interceptor._buckets

    def test_rate_limiter_existing_tenant_no_eviction(self):
        interceptor = RateLimitInterceptor(rate=100.0, burst=10, max_tenants=3)
        continuation = MagicMock(return_value="handler")

        for i in range(3):
            details = self._make_details("/entdb.EntDBService/GetNode", f"tenant-{i}")
            interceptor.intercept_service(continuation, details)

        # Accessing an existing tenant should NOT evict
        details = self._make_details("/entdb.EntDBService/GetNode", "tenant-1")
        interceptor.intercept_service(continuation, details)

        assert len(interceptor._buckets) == 3
        assert "tenant-0" in interceptor._buckets
