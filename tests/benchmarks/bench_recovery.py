"""
Recovery strategy performance benchmarks.

Run: pytest tests/benchmarks/bench_recovery.py -v --benchmark-only
"""

from __future__ import annotations

import asyncio
import sqlite3
import time

from dbaas.entdb_server.tools.recovery_strategy import (
    RecoveryStrategy,
)

# --- Mock providers for benchmarks ---


class BenchSnapshotProvider:
    """Creates a real SQLite database for benchmarking."""

    def __init__(self, num_seed_rows: int = 0):
        self._seed_rows = num_seed_rows

    async def find_latest_snapshot(self, tenant_id: str) -> dict | None:
        return {"s3_key": "bench-snapshot.db.gz", "last_stream_pos": "bench:0:0"}

    async def restore_snapshot(self, snapshot: dict, db_path: str) -> None:
        conn = sqlite3.connect(db_path)
        conn.execute(
            "CREATE TABLE IF NOT EXISTS nodes "
            "(tenant_id TEXT, node_id TEXT, type_id INTEGER, payload_json TEXT, "
            "created_at INTEGER, updated_at INTEGER, owner_actor TEXT, acl_blob TEXT)"
        )
        conn.execute(
            "CREATE TABLE IF NOT EXISTS edges "
            "(tenant_id TEXT, edge_type_id INTEGER, from_node_id TEXT, "
            "to_node_id TEXT, props_json TEXT, created_at INTEGER)"
        )
        conn.execute(
            "CREATE TABLE IF NOT EXISTS node_visibility "
            "(tenant_id TEXT, node_id TEXT, principal TEXT)"
        )
        conn.execute(
            "CREATE TABLE IF NOT EXISTS applied_events "
            "(tenant_id TEXT, idempotency_key TEXT, stream_pos TEXT, applied_at INTEGER)"
        )

        # Seed rows to simulate a real snapshot
        for i in range(self._seed_rows):
            conn.execute(
                "INSERT INTO nodes VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
                (
                    "bench",
                    f"node-{i}",
                    1,
                    f'{{"name": "Item {i}"}}',
                    int(time.time() * 1000),
                    int(time.time() * 1000),
                    "bench-user",
                    "[]",
                ),
            )

        conn.commit()
        conn.close()


class BenchKafkaProvider:
    """Simulates Kafka replay with configurable event count."""

    def __init__(self, events: int = 100):
        self._events = events

    async def check_kafka_coverage(self, topic: str, start_pos: str | None) -> bool:
        return True

    async def replay_from_kafka(
        self, topic: str, start_pos: str | None, db_path: str, timeout_seconds: int
    ) -> tuple[int, str | None]:
        # Simulate applying events to SQLite
        conn = sqlite3.connect(db_path)
        for i in range(self._events):
            conn.execute(
                "INSERT INTO nodes VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
                (
                    "bench",
                    f"kafka-node-{i}",
                    1,
                    f'{{"name": "Kafka Item {i}"}}',
                    int(time.time() * 1000),
                    int(time.time() * 1000),
                    "bench-user",
                    "[]",
                ),
            )
        conn.commit()
        conn.close()
        return (self._events, f"{topic}:0:{self._events}")


class BenchArchiveProvider:
    """Simulates archive replay."""

    def __init__(self, events: int = 100):
        self._events = events

    async def replay_from_archive(
        self, tenant_id: str, start_pos: str | None, db_path: str
    ) -> tuple[int, str | None]:
        conn = sqlite3.connect(db_path)
        for i in range(self._events):
            conn.execute(
                "INSERT INTO nodes VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
                (
                    tenant_id,
                    f"archive-node-{i}",
                    1,
                    f'{{"name": "Archive Item {i}"}}',
                    int(time.time() * 1000),
                    int(time.time() * 1000),
                    "bench-user",
                    "[]",
                ),
            )
        conn.commit()
        conn.close()
        return (self._events, f"bench:0:{self._events}")


# --- Plan benchmarks ---


class TestPlanBenchmarks:
    """Benchmark recovery plan creation."""

    def test_bench_plan_creation(self, benchmark):
        """Time to create a recovery plan."""
        strategy = RecoveryStrategy(
            snapshot_provider=BenchSnapshotProvider(),
            kafka_provider=BenchKafkaProvider(),
            archive_provider=BenchArchiveProvider(),
        )

        def run():
            return asyncio.get_event_loop().run_until_complete(strategy.plan("bench-tenant"))

        plan = benchmark(run)
        assert len(plan.tiers) == 3


# --- Execute benchmarks ---


class TestExecuteBenchmarks:
    """Benchmark full recovery execution."""

    def test_bench_snapshot_only_recovery(self, benchmark, tmp_path):
        """Recovery with snapshot only (no WAL replay)."""
        strategy = RecoveryStrategy(
            snapshot_provider=BenchSnapshotProvider(num_seed_rows=1000),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
            verify_after_recovery=False,
        )

        counter = [0]

        def run():
            db_path = str(tmp_path / f"bench-{counter[0]}.db")
            counter[0] += 1
            return asyncio.get_event_loop().run_until_complete(
                strategy.execute("bench-tenant", db_path)
            )

        result = benchmark(run)
        assert result.success

    def test_bench_snapshot_plus_kafka_100_events(self, benchmark, tmp_path):
        """Recovery: snapshot + 100 Kafka events."""
        strategy = RecoveryStrategy(
            snapshot_provider=BenchSnapshotProvider(num_seed_rows=100),
            kafka_provider=BenchKafkaProvider(events=100),
            archive_provider=None,
            archive_replay_enabled=False,
            verify_after_recovery=False,
        )

        counter = [0]

        def run():
            db_path = str(tmp_path / f"bench-{counter[0]}.db")
            counter[0] += 1
            return asyncio.get_event_loop().run_until_complete(
                strategy.execute("bench-tenant", db_path)
            )

        result = benchmark(run)
        assert result.success
        assert result.total_events_replayed == 100

    def test_bench_snapshot_plus_kafka_1000_events(self, benchmark, tmp_path):
        """Recovery: snapshot + 1000 Kafka events."""
        strategy = RecoveryStrategy(
            snapshot_provider=BenchSnapshotProvider(num_seed_rows=100),
            kafka_provider=BenchKafkaProvider(events=1000),
            archive_provider=None,
            archive_replay_enabled=False,
            verify_after_recovery=False,
        )

        counter = [0]

        def run():
            db_path = str(tmp_path / f"bench-{counter[0]}.db")
            counter[0] += 1
            return asyncio.get_event_loop().run_until_complete(
                strategy.execute("bench-tenant", db_path)
            )

        result = benchmark(run)
        assert result.success

    def test_bench_kafka_fallback_to_archive(self, benchmark, tmp_path):
        """Recovery: Kafka fails, falls back to archive."""
        strategy = RecoveryStrategy(
            snapshot_provider=BenchSnapshotProvider(),
            kafka_provider=BenchKafkaProvider(events=0),
            archive_provider=BenchArchiveProvider(events=500),
            verify_after_recovery=False,
        )
        # Make Kafka report no coverage
        strategy._kafka._has_coverage = False  # type: ignore

        counter = [0]

        def run():
            db_path = str(tmp_path / f"bench-{counter[0]}.db")
            counter[0] += 1
            return asyncio.get_event_loop().run_until_complete(
                strategy.execute("bench-tenant", db_path)
            )

        result = benchmark(run)
        assert result.success
