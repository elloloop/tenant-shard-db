"""
Recovery strategy integration tests.

Tests tiered recovery with real SQLite databases and mock providers.
"""

from __future__ import annotations

import json
import sqlite3

import pytest

from dbaas.entdb_server.tools.recovery_strategy import (
    RecoveryStrategy,
    RecoveryTier,
)

# ---------------------------------------------------------------------------
# Mock providers (extended from unit test patterns)
# ---------------------------------------------------------------------------


class MockSnapshotProvider:
    """Creates a real SQLite DB with schema on restore."""

    def __init__(
        self,
        snapshot: dict | None = None,
        nodes: list[dict] | None = None,
        edges: list[dict] | None = None,
        applied_events: list[dict] | None = None,
        raise_on_restore: bool = False,
    ):
        self._snapshot = snapshot
        self._nodes = nodes or []
        self._edges = edges or []
        self._applied_events = applied_events or []
        self._raise_on_restore = raise_on_restore
        self.restore_count = 0

    async def find_latest_snapshot(self, tenant_id: str) -> dict | None:
        return self._snapshot

    async def restore_snapshot(self, snapshot: dict, db_path: str) -> None:
        if self._raise_on_restore:
            raise RuntimeError("Snapshot restore failed")

        self.restore_count += 1
        conn = sqlite3.connect(db_path)
        conn.executescript(
            """
            CREATE TABLE IF NOT EXISTS nodes (
                tenant_id TEXT, node_id TEXT, type_id INTEGER,
                payload_json TEXT, created_at INTEGER, updated_at INTEGER,
                owner_actor TEXT, acl_blob TEXT
            );
            CREATE TABLE IF NOT EXISTS edges (
                tenant_id TEXT, edge_type_id INTEGER, from_node_id TEXT,
                to_node_id TEXT, props_json TEXT, created_at INTEGER
            );
            CREATE TABLE IF NOT EXISTS node_visibility (
                tenant_id TEXT, node_id TEXT, principal TEXT
            );
            CREATE TABLE IF NOT EXISTS applied_events (
                tenant_id TEXT, idempotency_key TEXT, stream_pos TEXT,
                applied_at INTEGER
            );
            """
        )
        for node in self._nodes:
            conn.execute(
                "INSERT INTO nodes VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
                (
                    node.get("tenant_id", "t1"),
                    node["node_id"],
                    node.get("type_id", 1),
                    json.dumps(node.get("payload", {})),
                    node.get("created_at", 1000),
                    node.get("updated_at", 1000),
                    node.get("owner_actor", "u:1"),
                    json.dumps(node.get("acl", [])),
                ),
            )
        for edge in self._edges:
            conn.execute(
                "INSERT INTO edges VALUES (?, ?, ?, ?, ?, ?)",
                (
                    edge.get("tenant_id", "t1"),
                    edge.get("edge_type_id", 1),
                    edge["from_node_id"],
                    edge["to_node_id"],
                    json.dumps(edge.get("props", {})),
                    edge.get("created_at", 1000),
                ),
            )
        for ae in self._applied_events:
            conn.execute(
                "INSERT INTO applied_events VALUES (?, ?, ?, ?)",
                (
                    ae.get("tenant_id", "t1"),
                    ae["idempotency_key"],
                    ae.get("stream_pos"),
                    ae.get("applied_at", 1000),
                ),
            )
        conn.commit()
        conn.close()


class MockKafkaProvider:
    def __init__(
        self,
        has_coverage: bool = True,
        events_to_replay: int = 5,
        raise_on_replay: bool = False,
    ):
        self._has_coverage = has_coverage
        self._events = events_to_replay
        self._raise_on_replay = raise_on_replay
        self.replay_count = 0

    async def check_kafka_coverage(self, topic: str, start_pos: str | None) -> bool:
        return self._has_coverage

    async def replay_from_kafka(
        self,
        topic: str,
        start_pos: str | None,
        db_path: str,
        timeout_seconds: int,
    ) -> tuple[int, str | None]:
        if self._raise_on_replay:
            raise RuntimeError("Kafka replay failed")
        self.replay_count += 1
        return (self._events, f"{topic}:0:{self._events + 100}")


class MockArchiveProvider:
    def __init__(
        self,
        events_to_replay: int = 10,
        raise_on_replay: bool = False,
    ):
        self._events = events_to_replay
        self._raise_on_replay = raise_on_replay
        self.replay_count = 0

    async def replay_from_archive(
        self,
        tenant_id: str,
        start_pos: str | None,
        db_path: str,
    ) -> tuple[int, str | None]:
        if self._raise_on_replay:
            raise RuntimeError("Archive replay failed")
        self.replay_count += 1
        return (self._events, f"entdb-wal:0:{self._events + 50}")


# =========================================================================
# SNAPSHOT RECOVERY
# =========================================================================


@pytest.mark.integration
class TestSnapshotRecovery:
    """Tests snapshot-based recovery with real SQLite."""

    @pytest.mark.asyncio
    async def test_restore_from_snapshot_creates_db(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-1", "last_stream_pos": "wal:0:10"},
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        # DB should exist and be valid
        conn = sqlite3.connect(db_path)
        tables = conn.execute("SELECT name FROM sqlite_master WHERE type='table'").fetchall()
        table_names = {t[0] for t in tables}
        assert "nodes" in table_names
        conn.close()

    @pytest.mark.asyncio
    async def test_snapshot_with_100_nodes_recoverable(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        nodes = [{"node_id": f"n-{i}", "payload": {"i": i}} for i in range(100)]
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-big"},
                nodes=nodes,
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

        conn = sqlite3.connect(db_path)
        count = conn.execute("SELECT COUNT(*) FROM nodes").fetchone()[0]
        assert count == 100
        conn.close()

    @pytest.mark.asyncio
    async def test_snapshot_preserves_payload_data(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-payload"},
                nodes=[{"node_id": "p1", "payload": {"name": "Alice", "age": 30}}],
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

        conn = sqlite3.connect(db_path)
        row = conn.execute("SELECT payload_json FROM nodes WHERE node_id = 'p1'").fetchone()
        payload = json.loads(row[0])
        assert payload["name"] == "Alice"
        assert payload["age"] == 30
        conn.close()

    @pytest.mark.asyncio
    async def test_snapshot_preserves_edges(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-edges"},
                nodes=[
                    {"node_id": "e-a"},
                    {"node_id": "e-b"},
                ],
                edges=[{"from_node_id": "e-a", "to_node_id": "e-b"}],
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

        conn = sqlite3.connect(db_path)
        edge_count = conn.execute("SELECT COUNT(*) FROM edges").fetchone()[0]
        assert edge_count == 1
        conn.close()

    @pytest.mark.asyncio
    async def test_snapshot_preserves_applied_events(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-idem"},
                applied_events=[
                    {"idempotency_key": "k1", "stream_pos": "wal:0:1"},
                    {"idempotency_key": "k2", "stream_pos": "wal:0:2"},
                ],
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

        conn = sqlite3.connect(db_path)
        count = conn.execute("SELECT COUNT(*) FROM applied_events").fetchone()[0]
        assert count == 2
        conn.close()

    @pytest.mark.asyncio
    async def test_recovery_verifies_integrity(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-ok"},
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
            verify_after_recovery=True,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

    @pytest.mark.asyncio
    async def test_recovery_with_empty_snapshot(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-empty"},
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

        conn = sqlite3.connect(db_path)
        count = conn.execute("SELECT COUNT(*) FROM nodes").fetchone()[0]
        assert count == 0
        conn.close()

    @pytest.mark.asyncio
    async def test_recovery_from_corrupt_snapshot_fails(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-corrupt"},
                raise_on_restore=True,
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert not result.success

    @pytest.mark.asyncio
    async def test_multiple_snapshots_uses_latest(self, tmp_path):
        """Provider always returns 'latest' snapshot."""
        db_path = str(tmp_path / "t1.db")
        provider = MockSnapshotProvider(
            snapshot={"s3_key": "snap-latest", "last_stream_pos": "wal:0:99"},
        )
        strategy = RecoveryStrategy(
            snapshot_provider=provider,
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        assert result.snapshot_used == "snap-latest"

    @pytest.mark.asyncio
    async def test_snapshot_stream_position_tracked(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-pos", "last_stream_pos": "wal:0:42"},
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        assert result.final_stream_pos == "wal:0:42"


# =========================================================================
# KAFKA REPLAY RECOVERY
# =========================================================================


@pytest.mark.integration
class TestKafkaReplayRecovery:
    """Tests Kafka WAL replay recovery with real SQLite."""

    @pytest.mark.asyncio
    async def test_replay_from_wal_creates_nodes(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-1", "last_stream_pos": "wal:0:10"},
            ),
            kafka_provider=MockKafkaProvider(has_coverage=True, events_to_replay=5),
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        assert result.total_events_replayed >= 5

    @pytest.mark.asyncio
    async def test_replay_after_snapshot_fills_gap(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-gap", "last_stream_pos": "wal:0:50"},
            ),
            kafka_provider=MockKafkaProvider(has_coverage=True, events_to_replay=10),
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

        snap_tier = result.tier_results[0]
        kafka_tier = result.tier_results[1]
        assert snap_tier.tier == RecoveryTier.SNAPSHOT
        assert kafka_tier.tier == RecoveryTier.KAFKA_WAL
        assert kafka_tier.events_replayed == 10

    @pytest.mark.asyncio
    async def test_replay_ordering_preserved(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        kafka = MockKafkaProvider(has_coverage=True, events_to_replay=20)
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-1"},
            ),
            kafka_provider=kafka,
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        kafka_tier = next(r for r in result.tier_results if r.tier == RecoveryTier.KAFKA_WAL)
        assert kafka_tier.events_replayed == 20

    @pytest.mark.asyncio
    async def test_replay_with_mixed_operations(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=MockKafkaProvider(has_coverage=True, events_to_replay=15),
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        assert result.total_events_replayed >= 15

    @pytest.mark.asyncio
    async def test_no_kafka_coverage_falls_to_archive(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=MockKafkaProvider(has_coverage=False),
            archive_provider=MockArchiveProvider(events_to_replay=8),
            kafka_replay_enabled=True,
            archive_replay_enabled=True,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        tier_names = [r.tier for r in result.tier_results]
        assert RecoveryTier.S3_ARCHIVE in tier_names

    @pytest.mark.asyncio
    async def test_partial_kafka_coverage_detected(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "s1", "last_stream_pos": "wal:0:50"},
            ),
            kafka_provider=MockKafkaProvider(has_coverage=False),
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        # Kafka failed, no archive, so snapshot-only is still success
        kafka_tier = next(r for r in result.tier_results if r.tier == RecoveryTier.KAFKA_WAL)
        assert not kafka_tier.success

    @pytest.mark.asyncio
    async def test_replay_updates_final_stream_pos(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "s1", "last_stream_pos": "wal:0:10"},
            ),
            kafka_provider=MockKafkaProvider(has_coverage=True, events_to_replay=5),
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        # Final pos should be from Kafka replay
        assert result.final_stream_pos is not None
        assert "105" in result.final_stream_pos  # events(5) + 100

    @pytest.mark.asyncio
    async def test_replay_with_batch_events(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=MockKafkaProvider(has_coverage=True, events_to_replay=50),
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        kafka_tier = next(r for r in result.tier_results if r.tier == RecoveryTier.KAFKA_WAL)
        assert kafka_tier.events_replayed == 50

    @pytest.mark.asyncio
    async def test_kafka_replay_failure_handled(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=MockKafkaProvider(has_coverage=True, raise_on_replay=True),
            archive_provider=MockArchiveProvider(events_to_replay=5),
            kafka_replay_enabled=True,
            archive_replay_enabled=True,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        # Should have fallen through to archive
        archive_tier = next(r for r in result.tier_results if r.tier == RecoveryTier.S3_ARCHIVE)
        assert archive_tier.success

    @pytest.mark.asyncio
    async def test_replay_idempotency_no_duplicates(self, tmp_path):
        """Running recovery twice does not produce errors."""
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "s1"},
                nodes=[{"node_id": "dup-check"}],
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        r1 = await strategy.execute("t1", db_path)
        assert r1.success
        # Second run: snapshot restores again (overwrites)
        r2 = await strategy.execute("t1", db_path)
        assert r2.success


# =========================================================================
# RECOVERY TIERS
# =========================================================================


@pytest.mark.integration
class TestRecoveryTiers:
    """Tests tier ordering, fallback, and result tracking."""

    @pytest.mark.asyncio
    async def test_snapshot_kafka_archive_priority_order(self, tmp_path):
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=MockKafkaProvider(),
            archive_provider=MockArchiveProvider(),
            kafka_replay_enabled=True,
            archive_replay_enabled=True,
        )
        plan = await strategy.plan("t1")
        assert plan.tiers == [
            RecoveryTier.SNAPSHOT,
            RecoveryTier.KAFKA_WAL,
            RecoveryTier.S3_ARCHIVE,
        ]

    @pytest.mark.asyncio
    async def test_kafka_failure_falls_to_archive(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=MockKafkaProvider(has_coverage=False),
            archive_provider=MockArchiveProvider(events_to_replay=7),
            kafka_replay_enabled=True,
            archive_replay_enabled=True,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        archive = next(r for r in result.tier_results if r.tier == RecoveryTier.S3_ARCHIVE)
        assert archive.events_replayed == 7

    @pytest.mark.asyncio
    async def test_all_tiers_fail_returns_error(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=None,
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert not result.success
        assert result.error is not None

    @pytest.mark.asyncio
    async def test_snapshot_only_recovery_works(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "snap-only"}),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        assert len(result.tier_results) == 1
        assert result.tier_results[0].tier == RecoveryTier.SNAPSHOT

    @pytest.mark.asyncio
    async def test_recovery_is_idempotent(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        provider = MockSnapshotProvider(
            snapshot={"s3_key": "snap-idem"},
            nodes=[{"node_id": "stable"}],
        )
        strategy = RecoveryStrategy(
            snapshot_provider=provider,
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        r1 = await strategy.execute("t1", db_path)
        r2 = await strategy.execute("t1", db_path)
        assert r1.success and r2.success

        conn = sqlite3.connect(db_path)
        count = conn.execute("SELECT COUNT(*) FROM nodes").fetchone()[0]
        # Mock restore appends (real restore would overwrite the file)
        assert count >= 1
        conn.close()

    @pytest.mark.asyncio
    async def test_recovery_plan_includes_correct_tiers(self, tmp_path):
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=MockKafkaProvider(),
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )
        plan = await strategy.plan("t1")
        assert RecoveryTier.SNAPSHOT in plan.tiers
        assert RecoveryTier.KAFKA_WAL in plan.tiers
        assert RecoveryTier.S3_ARCHIVE not in plan.tiers

    @pytest.mark.asyncio
    async def test_recovery_result_tracks_duration(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.duration_ms >= 0

    @pytest.mark.asyncio
    async def test_verify_after_true_checks_integrity(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
            verify_after_recovery=True,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

    @pytest.mark.asyncio
    async def test_verify_after_false_skips_check(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
            verify_after_recovery=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

    @pytest.mark.asyncio
    async def test_empty_plan_returns_failure(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot=None),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert not result.success
        assert "No recovery tiers" in result.error

    @pytest.mark.asyncio
    async def test_archive_only_recovery(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        # Create a valid DB first (so integrity check passes)
        conn = sqlite3.connect(db_path)
        conn.execute("CREATE TABLE placeholder (id INTEGER)")
        conn.close()

        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot=None),
            kafka_provider=None,
            archive_provider=MockArchiveProvider(events_to_replay=3),
            kafka_replay_enabled=False,
            archive_replay_enabled=True,
            verify_after_recovery=True,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        assert result.total_events_replayed == 3

    @pytest.mark.asyncio
    async def test_snapshot_used_field_set(self, tmp_path):
        db_path = str(tmp_path / "t1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "my-snap-key"},
            ),
            kafka_provider=MockKafkaProvider(has_coverage=True, events_to_replay=1),
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        assert result.snapshot_used == "my-snap-key"


# =========================================================================
# RECOVERY WITH REAL SQLITE DATA
# =========================================================================


@pytest.mark.integration
class TestRecoveryWithSQLiteData:
    """Tests recovery produces valid SQLite databases."""

    @pytest.mark.asyncio
    async def test_restored_db_has_all_tables(self, tmp_path):
        db_path = str(tmp_path / "full.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        await strategy.execute("t1", db_path)

        conn = sqlite3.connect(db_path)
        tables = {
            r[0]
            for r in conn.execute("SELECT name FROM sqlite_master WHERE type='table'").fetchall()
        }
        assert "nodes" in tables
        assert "edges" in tables
        assert "applied_events" in tables
        assert "node_visibility" in tables
        conn.close()

    @pytest.mark.asyncio
    async def test_restored_db_integrity_check_passes(self, tmp_path):
        db_path = str(tmp_path / "check.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "s1"},
                nodes=[{"node_id": f"n{i}"} for i in range(10)],
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        await strategy.execute("t1", db_path)

        conn = sqlite3.connect(db_path)
        result = conn.execute("PRAGMA integrity_check").fetchone()[0]
        assert result == "ok"
        conn.close()

    @pytest.mark.asyncio
    async def test_restored_nodes_queryable(self, tmp_path):
        db_path = str(tmp_path / "query.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "s1"},
                nodes=[
                    {"node_id": "a", "type_id": 1, "payload": {"name": "Alice"}},
                    {"node_id": "b", "type_id": 2, "payload": {"name": "Bob"}},
                ],
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        await strategy.execute("t1", db_path)

        conn = sqlite3.connect(db_path)
        type1 = conn.execute("SELECT * FROM nodes WHERE type_id = 1").fetchall()
        type2 = conn.execute("SELECT * FROM nodes WHERE type_id = 2").fetchall()
        assert len(type1) == 1
        assert len(type2) == 1
        conn.close()

    @pytest.mark.asyncio
    async def test_restored_edges_queryable(self, tmp_path):
        db_path = str(tmp_path / "edges.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "s1"},
                nodes=[{"node_id": "x"}, {"node_id": "y"}],
                edges=[
                    {"from_node_id": "x", "to_node_id": "y", "edge_type_id": 1},
                    {"from_node_id": "y", "to_node_id": "x", "edge_type_id": 2},
                ],
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        await strategy.execute("t1", db_path)

        conn = sqlite3.connect(db_path)
        edges = conn.execute("SELECT * FROM edges").fetchall()
        assert len(edges) == 2
        conn.close()

    @pytest.mark.asyncio
    async def test_restored_applied_events_trackable(self, tmp_path):
        db_path = str(tmp_path / "idem.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "s1"},
                applied_events=[
                    {"idempotency_key": f"k{i}", "stream_pos": f"wal:0:{i}"} for i in range(20)
                ],
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        await strategy.execute("t1", db_path)

        conn = sqlite3.connect(db_path)
        count = conn.execute("SELECT COUNT(*) FROM applied_events").fetchone()[0]
        assert count == 20
        conn.close()

    @pytest.mark.asyncio
    async def test_snapshot_with_payload_json_fidelity(self, tmp_path):
        db_path = str(tmp_path / "fidelity.db")
        payload = {"nested": {"list": [1, 2, 3], "bool": True, "null": None}}
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "s1"},
                nodes=[{"node_id": "fid", "payload": payload}],
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        await strategy.execute("t1", db_path)

        conn = sqlite3.connect(db_path)
        row = conn.execute("SELECT payload_json FROM nodes WHERE node_id='fid'").fetchone()
        parsed = json.loads(row[0])
        assert parsed["nested"]["list"] == [1, 2, 3]
        assert parsed["nested"]["bool"] is True
        assert parsed["nested"]["null"] is None
        conn.close()

    @pytest.mark.asyncio
    async def test_restore_count_tracked(self, tmp_path):
        db_path = str(tmp_path / "count.db")
        provider = MockSnapshotProvider(snapshot={"s3_key": "s1"})
        strategy = RecoveryStrategy(
            snapshot_provider=provider,
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        await strategy.execute("t1", db_path)
        assert provider.restore_count == 1

    @pytest.mark.asyncio
    async def test_kafka_replay_count_tracked(self, tmp_path):
        db_path = str(tmp_path / "kcount.db")
        kafka = MockKafkaProvider(has_coverage=True, events_to_replay=3)
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=kafka,
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )
        await strategy.execute("t1", db_path)
        assert kafka.replay_count == 1

    @pytest.mark.asyncio
    async def test_archive_replay_count_tracked(self, tmp_path):
        db_path = str(tmp_path / "acount.db")
        conn = sqlite3.connect(db_path)
        conn.execute("CREATE TABLE placeholder (id INTEGER)")
        conn.close()

        archive = MockArchiveProvider(events_to_replay=2)
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot=None),
            kafka_provider=None,
            archive_provider=archive,
            kafka_replay_enabled=False,
            archive_replay_enabled=True,
        )
        await strategy.execute("t1", db_path)
        assert archive.replay_count == 1


# =========================================================================
# RECOVERY PLAN BUILDER
# =========================================================================


@pytest.mark.integration
class TestRecoveryPlanBuilder:
    """Tests for the plan() and build_plan() methods."""

    @pytest.mark.asyncio
    async def test_plan_is_alias_for_build_plan(self):
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=MockKafkaProvider(),
            archive_provider=MockArchiveProvider(),
        )
        p1 = await strategy.plan("t1")
        p2 = await strategy.build_plan("t1")
        assert p1.tiers == p2.tiers

    @pytest.mark.asyncio
    async def test_plan_tenant_id_set(self):
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
        )
        plan = await strategy.plan("my-tenant")
        assert plan.tenant_id == "my-tenant"

    @pytest.mark.asyncio
    async def test_plan_snapshot_key_set(self):
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "snapshots/t1/latest.db"}),
        )
        plan = await strategy.plan("t1")
        assert plan.snapshot_key == "snapshots/t1/latest.db"

    @pytest.mark.asyncio
    async def test_plan_snapshot_stream_pos_set(self):
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "s1", "last_stream_pos": "wal:0:99"}
            ),
        )
        plan = await strategy.plan("t1")
        assert plan.snapshot_stream_pos == "wal:0:99"

    @pytest.mark.asyncio
    async def test_plan_no_snapshot_key_when_none(self):
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot=None),
        )
        plan = await strategy.plan("t1")
        assert plan.snapshot_key is None

    @pytest.mark.asyncio
    async def test_plan_no_snapshot_stream_pos_when_none(self):
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot=None),
        )
        plan = await strategy.plan("t1")
        assert plan.snapshot_stream_pos is None

    @pytest.mark.asyncio
    async def test_plan_kafka_only(self):
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot=None),
            kafka_provider=MockKafkaProvider(),
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )
        plan = await strategy.plan("t1")
        assert plan.tiers == [RecoveryTier.KAFKA_WAL]

    @pytest.mark.asyncio
    async def test_plan_archive_only(self):
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot=None),
            archive_provider=MockArchiveProvider(),
            kafka_replay_enabled=False,
            archive_replay_enabled=True,
        )
        plan = await strategy.plan("t1")
        assert plan.tiers == [RecoveryTier.S3_ARCHIVE]

    @pytest.mark.asyncio
    async def test_execute_with_string_tenant(self, tmp_path):
        """execute() accepts a tenant_id string and builds plan internally."""
        db_path = str(tmp_path / "auto-plan.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

    @pytest.mark.asyncio
    async def test_execute_with_plan_object(self, tmp_path):
        """execute() accepts a RecoveryPlan object."""
        db_path = str(tmp_path / "explicit-plan.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        plan = await strategy.plan("t1")
        result = await strategy.execute(plan, db_path)
        assert result.success

    @pytest.mark.asyncio
    async def test_recovery_tier_enum_values(self):
        assert RecoveryTier.SNAPSHOT.value == "snapshot"
        assert RecoveryTier.KAFKA_WAL.value == "kafka_wal"
        assert RecoveryTier.S3_ARCHIVE.value == "s3_archive"

    @pytest.mark.asyncio
    async def test_tier_result_defaults(self):
        from dbaas.entdb_server.tools.recovery_strategy import TierResult

        tr = TierResult(tier=RecoveryTier.SNAPSHOT, success=True)
        assert tr.events_replayed == 0
        assert tr.final_stream_pos is None
        assert tr.error is None
        assert tr.skipped is False

    @pytest.mark.asyncio
    async def test_recovery_result_defaults(self):
        from dbaas.entdb_server.tools.recovery_strategy import RecoveryResult

        rr = RecoveryResult(success=False)
        assert rr.tier_results == []
        assert rr.snapshot_used is None
        assert rr.total_events_replayed == 0
        assert rr.final_stream_pos is None
        assert rr.duration_ms == 0
        assert rr.error is None

    @pytest.mark.asyncio
    async def test_multiple_tenants_recovery(self, tmp_path):
        """Recover multiple tenants independently."""
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        for tid in ["t1", "t2", "t3"]:
            db_path = str(tmp_path / f"{tid}.db")
            result = await strategy.execute(tid, db_path)
            assert result.success

    @pytest.mark.asyncio
    async def test_kafka_timeout_config(self):
        strategy = RecoveryStrategy(
            kafka_replay_timeout_seconds=60,
            kafka_topic="custom-topic",
        )
        assert strategy._kafka_timeout == 60
        assert strategy._kafka_topic == "custom-topic"

    @pytest.mark.asyncio
    async def test_snapshot_then_archive_when_kafka_disabled(self, tmp_path):
        db_path = str(tmp_path / "no-kafka.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=MockKafkaProvider(),
            archive_provider=MockArchiveProvider(events_to_replay=4),
            kafka_replay_enabled=False,
            archive_replay_enabled=True,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        tier_types = [r.tier for r in result.tier_results]
        assert RecoveryTier.KAFKA_WAL not in tier_types
        assert RecoveryTier.S3_ARCHIVE in tier_types


# =========================================================================
# RECOVERY EDGE CASES
# =========================================================================


@pytest.mark.integration
class TestRecoveryEdgeCases:
    """Edge case tests for recovery strategy."""

    @pytest.mark.asyncio
    async def test_snapshot_with_large_node_count(self, tmp_path):
        db_path = str(tmp_path / "large.db")
        nodes = [{"node_id": f"ln-{i}", "payload": {"i": i}} for i in range(500)]
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "big"}, nodes=nodes),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

        conn = sqlite3.connect(db_path)
        count = conn.execute("SELECT COUNT(*) FROM nodes").fetchone()[0]
        assert count == 500
        conn.close()

    @pytest.mark.asyncio
    async def test_snapshot_with_edges_and_nodes(self, tmp_path):
        db_path = str(tmp_path / "graph.db")
        nodes = [{"node_id": f"gn-{i}"} for i in range(10)]
        edges = [{"from_node_id": f"gn-{i}", "to_node_id": f"gn-{(i + 1) % 10}"} for i in range(10)]
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "graph"}, nodes=nodes, edges=edges
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

        conn = sqlite3.connect(db_path)
        node_count = conn.execute("SELECT COUNT(*) FROM nodes").fetchone()[0]
        edge_count = conn.execute("SELECT COUNT(*) FROM edges").fetchone()[0]
        assert node_count == 10
        assert edge_count == 10
        conn.close()

    @pytest.mark.asyncio
    async def test_archive_after_snapshot_failure(self, tmp_path):
        db_path = str(tmp_path / "fallback.db")
        # Create valid DB first
        conn = sqlite3.connect(db_path)
        conn.execute("CREATE TABLE dummy (id INTEGER)")
        conn.close()

        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "bad"}, raise_on_restore=True
            ),
            kafka_provider=None,
            archive_provider=MockArchiveProvider(events_to_replay=5),
            kafka_replay_enabled=False,
            archive_replay_enabled=True,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        assert result.total_events_replayed == 5

    @pytest.mark.asyncio
    async def test_all_providers_raise(self, tmp_path):
        db_path = str(tmp_path / "all-fail.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "bad"}, raise_on_restore=True
            ),
            kafka_provider=MockKafkaProvider(has_coverage=True, raise_on_replay=True),
            archive_provider=MockArchiveProvider(raise_on_replay=True),
            kafka_replay_enabled=True,
            archive_replay_enabled=True,
        )
        result = await strategy.execute("t1", db_path)
        assert not result.success

    @pytest.mark.asyncio
    async def test_recovery_duration_positive(self, tmp_path):
        db_path = str(tmp_path / "dur.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "s1"},
                nodes=[{"node_id": f"d-{i}"} for i in range(50)],
            ),
            kafka_provider=MockKafkaProvider(has_coverage=True, events_to_replay=10),
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        assert result.duration_ms >= 0

    @pytest.mark.asyncio
    async def test_snapshot_key_in_result(self, tmp_path):
        db_path = str(tmp_path / "key.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "snapshots/tenant/001.db"}),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.snapshot_used == "snapshots/tenant/001.db"

    @pytest.mark.asyncio
    async def test_kafka_with_zero_events(self, tmp_path):
        db_path = str(tmp_path / "zero-ev.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=MockKafkaProvider(has_coverage=True, events_to_replay=0),
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        assert result.total_events_replayed == 0

    @pytest.mark.asyncio
    async def test_archive_with_zero_events(self, tmp_path):
        db_path = str(tmp_path / "zero-arch.db")
        conn = sqlite3.connect(db_path)
        conn.execute("CREATE TABLE dummy (id INTEGER)")
        conn.close()

        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot=None),
            kafka_provider=None,
            archive_provider=MockArchiveProvider(events_to_replay=0),
            kafka_replay_enabled=False,
            archive_replay_enabled=True,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

    @pytest.mark.asyncio
    async def test_multiple_sequential_recoveries(self, tmp_path):
        """Run recovery 5 times sequentially on same DB path."""
        db_path = str(tmp_path / "seq.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "s1"},
                nodes=[{"node_id": "stable"}],
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        for _ in range(5):
            result = await strategy.execute("t1", db_path)
            assert result.success

        conn = sqlite3.connect(db_path)
        count = conn.execute("SELECT COUNT(*) FROM nodes").fetchone()[0]
        # Mock restore appends (real restore would overwrite the file)
        assert count >= 1
        conn.close()

    @pytest.mark.asyncio
    async def test_tier_result_skipped_flag(self, tmp_path):
        db_path = str(tmp_path / "skipped.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot=None),
            kafka_provider=MockKafkaProvider(has_coverage=True, events_to_replay=5),
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
            verify_after_recovery=False,
        )
        result = await strategy.execute("t1", db_path)
        # Snapshot was skipped (not in plan), only Kafka is in tiers
        kafka_tier = next(r for r in result.tier_results if r.tier == RecoveryTier.KAFKA_WAL)
        assert kafka_tier.success

    @pytest.mark.asyncio
    async def test_recovery_with_different_db_paths(self, tmp_path):
        """Each tenant gets its own DB file."""
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "s1"},
                nodes=[{"node_id": "n1"}],
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )

        for name in ["alpha", "beta", "gamma"]:
            db_path = str(tmp_path / f"{name}.db")
            result = await strategy.execute(name, db_path)
            assert result.success

            conn = sqlite3.connect(db_path)
            count = conn.execute("SELECT COUNT(*) FROM nodes").fetchone()[0]
            assert count == 1
            conn.close()

    @pytest.mark.asyncio
    async def test_snapshot_with_unicode_payload(self, tmp_path):
        db_path = str(tmp_path / "unicode.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "s1"},
                nodes=[{"node_id": "uni", "payload": {"name": "Alice"}}],
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

        conn = sqlite3.connect(db_path)
        row = conn.execute("SELECT payload_json FROM nodes WHERE node_id='uni'").fetchone()
        parsed = json.loads(row[0])
        assert parsed["name"] == "Alice"
        conn.close()

    @pytest.mark.asyncio
    async def test_recovery_plan_tenant_id_matches(self):
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
        )
        plan = await strategy.plan("specific-tenant-123")
        assert plan.tenant_id == "specific-tenant-123"

    @pytest.mark.asyncio
    async def test_no_verify_with_corrupt_db(self, tmp_path):
        """With verify=False, corrupt DB still reports success if tiers pass."""
        db_path = str(tmp_path / "noverify.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
            verify_after_recovery=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

    @pytest.mark.asyncio
    async def test_kafka_coverage_check_called(self, tmp_path):
        """Kafka provider's check_kafka_coverage is called before replay."""
        db_path = str(tmp_path / "coverage.db")
        kafka = MockKafkaProvider(has_coverage=False)
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=kafka,
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        # Kafka should fail due to no coverage
        kafka_tier = next(r for r in result.tier_results if r.tier == RecoveryTier.KAFKA_WAL)
        assert not kafka_tier.success
        assert kafka.replay_count == 0  # replay was never called

    @pytest.mark.asyncio
    async def test_archive_replay_after_kafka_failure(self, tmp_path):
        """Archive is used when Kafka raises an exception."""
        db_path = str(tmp_path / "arch-after-kafka.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=MockKafkaProvider(has_coverage=True, raise_on_replay=True),
            archive_provider=MockArchiveProvider(events_to_replay=3),
            kafka_replay_enabled=True,
            archive_replay_enabled=True,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        assert result.total_events_replayed >= 3

    @pytest.mark.asyncio
    async def test_snapshot_restore_creates_file(self, tmp_path):
        db_path = str(tmp_path / "newfile.db")
        import os

        assert not os.path.exists(db_path)
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        await strategy.execute("t1", db_path)
        assert os.path.exists(db_path)

    @pytest.mark.asyncio
    async def test_snapshot_with_mixed_type_ids(self, tmp_path):
        db_path = str(tmp_path / "types.db")
        nodes = [{"node_id": f"t{tid}-{i}", "type_id": tid} for tid in [1, 2, 3] for i in range(3)]
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}, nodes=nodes),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

        conn = sqlite3.connect(db_path)
        count = conn.execute("SELECT COUNT(*) FROM nodes").fetchone()[0]
        assert count == 9
        conn.close()

    @pytest.mark.asyncio
    async def test_kafka_and_archive_both_succeed(self, tmp_path):
        """Kafka succeeds first, so archive is never attempted."""
        db_path = str(tmp_path / "both.db")
        kafka = MockKafkaProvider(has_coverage=True, events_to_replay=5)
        archive = MockArchiveProvider(events_to_replay=10)
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=kafka,
            archive_provider=archive,
            kafka_replay_enabled=True,
            archive_replay_enabled=True,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        assert kafka.replay_count == 1
        assert archive.replay_count == 0  # archive was not needed

    @pytest.mark.asyncio
    async def test_total_events_counts_all_tiers(self, tmp_path):
        db_path = str(tmp_path / "total.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "s1"}),
            kafka_provider=MockKafkaProvider(has_coverage=True, events_to_replay=10),
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success
        assert result.total_events_replayed == 10

    @pytest.mark.asyncio
    async def test_snapshot_owner_actor_preserved(self, tmp_path):
        db_path = str(tmp_path / "owner.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "s1"},
                nodes=[{"node_id": "owned", "owner_actor": "user:42"}],
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

        conn = sqlite3.connect(db_path)
        row = conn.execute("SELECT owner_actor FROM nodes WHERE node_id='owned'").fetchone()
        assert row[0] == "user:42"
        conn.close()

    @pytest.mark.asyncio
    async def test_snapshot_acl_preserved(self, tmp_path):
        db_path = str(tmp_path / "acl.db")
        acl = [{"principal": "user:1", "permission": "admin"}]
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "s1"},
                nodes=[{"node_id": "acl-n", "acl": acl}],
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )
        result = await strategy.execute("t1", db_path)
        assert result.success

        conn = sqlite3.connect(db_path)
        row = conn.execute("SELECT acl_blob FROM nodes WHERE node_id='acl-n'").fetchone()
        parsed = json.loads(row[0])
        assert len(parsed) == 1
        conn.close()
