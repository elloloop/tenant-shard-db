"""
CRUD operation benchmarks for EntDB CanonicalStore.

Benchmarks cover the core database operations against real SQLite:
- Node create, read, update, delete
- Edge create, read, delete
- Idempotency checks
- Query by type with pagination
- Graph traversal (edges from/to)
- Visibility-filtered queries
- Bulk operations
- Concurrent multi-tenant access

Run:
  pytest tests/benchmarks/bench_crud.py -v --benchmark-only
  pytest tests/benchmarks/bench_crud.py -v --benchmark-columns=mean,stddev,rounds,iterations
"""
from __future__ import annotations

import asyncio

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore

# --- Fixtures ---

@pytest.fixture
def store(tmp_path):
    """Create a CanonicalStore with a temp directory."""
    return CanonicalStore(data_dir=str(tmp_path), wal_mode=True)


@pytest.fixture
def initialized_store(store):
    """Store with tenant already initialized."""
    asyncio.get_event_loop().run_until_complete(store.initialize_tenant("bench"))
    return store


@pytest.fixture
def populated_store(initialized_store):
    """Store with 1000 nodes and 500 edges pre-populated."""
    loop = asyncio.get_event_loop()
    node_ids = []
    for i in range(1000):
        node = loop.run_until_complete(
            initialized_store.create_node(
                tenant_id="bench",
                type_id=(i % 5) + 1,  # 5 different types
                payload={"name": f"Item {i}", "index": i, "tags": ["a", "b"]},
                owner_actor=f"user:{i % 10}",
                node_id=f"node-{i:04d}",
                acl=[{"principal": f"user:{i % 10}", "permission": "read"}],
            )
        )
        node_ids.append(node.node_id)

    # Create edges between consecutive nodes
    for i in range(500):
        loop.run_until_complete(
            initialized_store.create_edge(
                tenant_id="bench",
                edge_type_id=1,
                from_node_id=node_ids[i],
                to_node_id=node_ids[i + 1],
                props={"weight": i * 0.1},
            )
        )

    return initialized_store, node_ids


# --- Node Create Benchmarks ---

class TestNodeCreateBenchmarks:
    """Benchmark node creation operations."""

    def test_bench_create_node_simple(self, benchmark, initialized_store):
        """Create a single node with minimal payload."""
        counter = [0]

        def run():
            counter[0] += 1
            return asyncio.get_event_loop().run_until_complete(
                initialized_store.create_node(
                    tenant_id="bench",
                    type_id=1,
                    payload={"name": f"Node {counter[0]}"},
                    owner_actor="bench-user",
                )
            )

        node = benchmark(run)
        assert node.node_id is not None

    def test_bench_create_node_large_payload(self, benchmark, initialized_store):
        """Create a node with a large payload (~5KB)."""
        counter = [0]

        def run():
            counter[0] += 1
            return asyncio.get_event_loop().run_until_complete(
                initialized_store.create_node(
                    tenant_id="bench",
                    type_id=1,
                    payload={
                        "name": f"Large Node {counter[0]}",
                        "description": "x" * 4000,
                        "tags": [f"tag-{j}" for j in range(50)],
                        "metadata": {f"key-{j}": f"val-{j}" for j in range(20)},
                    },
                    owner_actor="bench-user",
                )
            )

        node = benchmark(run)
        assert node.node_id is not None

    def test_bench_create_node_with_acl(self, benchmark, initialized_store):
        """Create a node with ACL entries (triggers visibility index update)."""
        counter = [0]

        def run():
            counter[0] += 1
            return asyncio.get_event_loop().run_until_complete(
                initialized_store.create_node(
                    tenant_id="bench",
                    type_id=1,
                    payload={"name": f"ACL Node {counter[0]}"},
                    owner_actor="bench-user",
                    acl=[
                        {"principal": "user:1", "permission": "read"},
                        {"principal": "user:2", "permission": "write"},
                        {"principal": "group:admins", "permission": "admin"},
                    ],
                )
            )

        node = benchmark(run)
        assert node.acl is not None

    def test_bench_create_100_nodes_batch(self, benchmark, initialized_store):
        """Create 100 nodes sequentially (simulates bulk ingest)."""
        counter = [0]

        def run():
            loop = asyncio.get_event_loop()
            nodes = []
            for i in range(100):
                counter[0] += 1
                n = loop.run_until_complete(
                    initialized_store.create_node(
                        tenant_id="bench",
                        type_id=1,
                        payload={"name": f"Batch {counter[0]}", "i": i},
                        owner_actor="bench-user",
                    )
                )
                nodes.append(n)
            return nodes

        nodes = benchmark(run)
        assert len(nodes) == 100


# --- Node Read Benchmarks ---

class TestNodeReadBenchmarks:
    """Benchmark node read operations."""

    def test_bench_get_node_by_id(self, benchmark, populated_store):
        """Read a single node by ID."""
        store, node_ids = populated_store

        def run():
            return asyncio.get_event_loop().run_until_complete(
                store.get_node("bench", "node-0500")
            )

        node = benchmark(run)
        assert node is not None
        assert node.node_id == "node-0500"

    def test_bench_get_node_miss(self, benchmark, populated_store):
        """Read a non-existent node (should return None)."""
        store, _ = populated_store

        def run():
            return asyncio.get_event_loop().run_until_complete(
                store.get_node("bench", "nonexistent-node")
            )

        result = benchmark(run)
        assert result is None

    def test_bench_get_nodes_by_type_100(self, benchmark, populated_store):
        """Query 100 nodes by type_id."""
        store, _ = populated_store

        def run():
            return asyncio.get_event_loop().run_until_complete(
                store.get_nodes_by_type("bench", type_id=1, limit=100)
            )

        nodes = benchmark(run)
        assert len(nodes) > 0

    def test_bench_get_nodes_by_type_paginated(self, benchmark, populated_store):
        """Query nodes with pagination (offset=50, limit=20)."""
        store, _ = populated_store

        def run():
            return asyncio.get_event_loop().run_until_complete(
                store.get_nodes_by_type("bench", type_id=1, limit=20, offset=50)
            )

        nodes = benchmark(run)
        assert len(nodes) <= 20


# --- Node Update Benchmarks ---

class TestNodeUpdateBenchmarks:
    """Benchmark node update operations."""

    def test_bench_update_node_small_patch(self, benchmark, populated_store):
        """Update a single field on a node."""
        store, _ = populated_store
        counter = [0]

        def run():
            counter[0] += 1
            return asyncio.get_event_loop().run_until_complete(
                store.update_node(
                    "bench", "node-0100", {"name": f"Updated {counter[0]}"}
                )
            )

        node = benchmark(run)
        assert node is not None

    def test_bench_update_node_large_patch(self, benchmark, populated_store):
        """Update multiple fields on a node."""
        store, _ = populated_store
        counter = [0]

        def run():
            counter[0] += 1
            return asyncio.get_event_loop().run_until_complete(
                store.update_node(
                    "bench",
                    "node-0200",
                    {
                        "name": f"Big Update {counter[0]}",
                        "description": "x" * 2000,
                        "extra_tags": ["new", "updated"],
                        "revision": counter[0],
                    },
                )
            )

        node = benchmark(run)
        assert node is not None


# --- Node Delete Benchmarks ---

class TestNodeDeleteBenchmarks:
    """Benchmark node deletion (including cascade to edges and visibility)."""

    def test_bench_delete_node_no_edges(self, benchmark, initialized_store):
        """Delete a node with no edges."""
        loop = asyncio.get_event_loop()
        counter = [0]

        # Pre-create nodes to delete
        nodes_to_delete = []
        for i in range(200):
            n = loop.run_until_complete(
                initialized_store.create_node(
                    tenant_id="bench",
                    type_id=1,
                    payload={"name": f"Deletable {i}"},
                    owner_actor="user",
                )
            )
            nodes_to_delete.append(n.node_id)

        def run():
            if counter[0] < len(nodes_to_delete):
                nid = nodes_to_delete[counter[0]]
                counter[0] += 1
                return asyncio.get_event_loop().run_until_complete(
                    initialized_store.delete_node("bench", nid)
                )
            return False

        benchmark(run)


# --- Edge Benchmarks ---

class TestEdgeBenchmarks:
    """Benchmark edge operations."""

    def test_bench_create_edge(self, benchmark, populated_store):
        """Create an edge between existing nodes."""
        store, node_ids = populated_store
        counter = [0]

        def run():
            i = counter[0] % (len(node_ids) - 2)
            counter[0] += 1
            return asyncio.get_event_loop().run_until_complete(
                store.create_edge(
                    tenant_id="bench",
                    edge_type_id=2,  # Different type to avoid conflicts
                    from_node_id=node_ids[i],
                    to_node_id=node_ids[i + 2],
                    props={"label": f"edge-{counter[0]}"},
                )
            )

        edge = benchmark(run)
        assert edge.edge_type_id == 2

    def test_bench_get_edges_from(self, benchmark, populated_store):
        """Get outgoing edges from a node."""
        store, node_ids = populated_store

        def run():
            return asyncio.get_event_loop().run_until_complete(
                store.get_edges_from("bench", node_ids[0])
            )

        edges = benchmark(run)
        assert isinstance(edges, list)

    def test_bench_get_edges_to(self, benchmark, populated_store):
        """Get incoming edges to a node."""
        store, node_ids = populated_store

        def run():
            return asyncio.get_event_loop().run_until_complete(
                store.get_edges_to("bench", node_ids[250])
            )

        edges = benchmark(run)
        assert isinstance(edges, list)

    def test_bench_delete_edge(self, benchmark, populated_store):
        """Delete an edge."""
        store, node_ids = populated_store
        counter = [500]  # Start from end to avoid deleting edges we read

        def run():
            counter[0] -= 1
            i = max(counter[0], 0)
            return asyncio.get_event_loop().run_until_complete(
                store.delete_edge("bench", 1, node_ids[i], node_ids[i + 1])
            )

        benchmark(run)


# --- Idempotency Benchmarks ---

class TestIdempotencyBenchmarks:
    """Benchmark idempotency check and record operations."""

    def test_bench_check_idempotency_miss(self, benchmark, populated_store):
        """Check idempotency for a key that doesn't exist (fast path)."""
        store, _ = populated_store
        counter = [0]

        def run():
            counter[0] += 1
            return asyncio.get_event_loop().run_until_complete(
                store.check_idempotency("bench", f"nonexistent-key-{counter[0]}")
            )

        result = benchmark(run)
        assert result is False

    def test_bench_check_idempotency_hit(self, benchmark, populated_store):
        """Check idempotency for a key that exists (dedup path)."""
        store, _ = populated_store
        loop = asyncio.get_event_loop()

        # Record some events first
        for i in range(100):
            loop.run_until_complete(
                store.record_applied_event("bench", f"existing-key-{i}", f"pos:{i}")
            )

        def run():
            return asyncio.get_event_loop().run_until_complete(
                store.check_idempotency("bench", "existing-key-50")
            )

        result = benchmark(run)
        assert result is True

    def test_bench_record_applied_event(self, benchmark, populated_store):
        """Record an applied event (write to applied_events table)."""
        store, _ = populated_store
        counter = [0]

        def run():
            counter[0] += 1
            return asyncio.get_event_loop().run_until_complete(
                store.record_applied_event(
                    "bench", f"bench-key-{counter[0]}", f"topic:0:{counter[0]}"
                )
            )

        benchmark(run)


# --- Multi-Tenant Benchmarks ---

class TestMultiTenantBenchmarks:
    """Benchmark operations across multiple tenants."""

    def test_bench_initialize_tenant(self, benchmark, store):
        """Initialize a new tenant (create SQLite file + schema)."""
        counter = [0]

        def run():
            counter[0] += 1
            return asyncio.get_event_loop().run_until_complete(
                store.initialize_tenant(f"tenant-{counter[0]}")
            )

        benchmark(run)

    def test_bench_crud_across_tenants(self, benchmark, store):
        """Create + read across 10 different tenants."""
        loop = asyncio.get_event_loop()
        for i in range(10):
            loop.run_until_complete(store.initialize_tenant(f"mt-{i}"))

        counter = [0]

        def run():
            counter[0] += 1
            tid = f"mt-{counter[0] % 10}"
            node = loop.run_until_complete(
                store.create_node(tid, 1, {"n": counter[0]}, "user")
            )
            fetched = loop.run_until_complete(
                store.get_node(tid, node.node_id)
            )
            return fetched

        result = benchmark(run)
        assert result is not None


# --- Scale Benchmarks ---

class TestScaleBenchmarks:
    """Benchmark behavior at scale."""

    def test_bench_read_from_large_table(self, benchmark, populated_store):
        """Read a node when table has 1000 rows (index performance)."""
        store, _ = populated_store

        def run():
            return asyncio.get_event_loop().run_until_complete(
                store.get_node("bench", "node-0999")
            )

        node = benchmark(run)
        assert node is not None

    def test_bench_query_type_from_large_table(self, benchmark, populated_store):
        """Query by type from 1000-row table."""
        store, _ = populated_store

        def run():
            return asyncio.get_event_loop().run_until_complete(
                store.get_nodes_by_type("bench", type_id=3, limit=50)
            )

        nodes = benchmark(run)
        assert len(nodes) > 0
