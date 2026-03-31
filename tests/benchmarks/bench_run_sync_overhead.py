"""
Benchmark: _run_sync metrics overhead on the SQLite hot path.

Measures the cost of the metrics instrumentation in _run_sync, which
is called on EVERY SQLite operation (get_node, create_node, etc.).

The optimization eliminates per-call overhead when metrics are disabled
(the default) by:
  1. Hoisting the `record_sqlite_op` import to module level
  2. Guarding the entire metrics block behind a fast `metrics_enabled()` check
  3. Caching the read/write classification per function object

Run:
  pytest tests/benchmarks/bench_run_sync_overhead.py -v
"""

from __future__ import annotations

import asyncio
import time

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore


@pytest.fixture
def store(tmp_path):
    """Create a CanonicalStore with a temp directory."""
    return CanonicalStore(data_dir=str(tmp_path), wal_mode=True)


@pytest.fixture
def ready_store(store):
    """Store with a tenant and some seed data."""
    loop = asyncio.get_event_loop()
    loop.run_until_complete(store.initialize_tenant("perf"))
    # Seed 100 nodes so reads are realistic
    for i in range(100):
        loop.run_until_complete(
            store.create_node(
                tenant_id="perf",
                type_id=1,
                payload={"n": i},
                owner_actor="user:1",
                node_id=f"seed-{i:04d}",
            )
        )
    return store


class TestRunSyncOverhead:
    """Quantify the per-call overhead of _run_sync metrics instrumentation."""

    def test_bench_get_node_latency(self, benchmark, ready_store):
        """Benchmark single get_node call — dominated by _run_sync overhead
        when the SQLite query itself is sub-microsecond (indexed PK lookup)."""

        def run():
            return asyncio.get_event_loop().run_until_complete(
                ready_store.get_node("perf", "seed-0050")
            )

        node = benchmark(run)
        assert node is not None

    def test_bench_1000_get_node_calls(self, benchmark, ready_store):
        """1000 sequential get_node calls — multiplies the per-call overhead.

        This is the scenario where eliminating the metrics overhead matters
        most: high-frequency reads in a tight loop (e.g. GetNodes RPC with
        many IDs, or the applier reading existing nodes for update_node).
        """
        loop = asyncio.get_event_loop()
        ids = [f"seed-{i:04d}" for i in range(100)]

        def run():
            for _ in range(10):  # 10 rounds x 100 ids = 1000 calls
                for nid in ids:
                    loop.run_until_complete(ready_store.get_node("perf", nid))

        benchmark(run)

    def test_bench_create_node_latency(self, benchmark, ready_store):
        """Single create_node — includes _run_sync overhead on write path."""
        counter = [0]

        def run():
            counter[0] += 1
            return asyncio.get_event_loop().run_until_complete(
                ready_store.create_node(
                    tenant_id="perf",
                    type_id=1,
                    payload={"x": counter[0]},
                    owner_actor="user:bench",
                )
            )

        node = benchmark(run)
        assert node is not None

    def test_bench_check_idempotency_miss(self, benchmark, ready_store):
        """Idempotency check (miss) — called on every applier event."""
        counter = [0]

        def run():
            counter[0] += 1
            return asyncio.get_event_loop().run_until_complete(
                ready_store.check_idempotency("perf", f"nonexistent-{counter[0]}")
            )

        result = benchmark(run)
        assert result is False

    def test_metrics_enabled_overhead_direct(self, benchmark):
        """Measure raw cost of the metrics_enabled() guard itself.

        This should be negligible (~20-50ns per call) — confirms the guard
        is cheaper than what it replaces.
        """
        from dbaas.entdb_server.metrics import metrics_enabled

        def run():
            total = 0
            for _ in range(100_000):
                if metrics_enabled():
                    total += 1
            return total

        result = benchmark(run)
        assert result == 0  # metrics not initialized in test
