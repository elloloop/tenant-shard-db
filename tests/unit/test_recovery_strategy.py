"""
Unit tests for tiered recovery strategy.

Tests cover:
- Recovery plan construction based on available providers
- Tier ordering (snapshot -> Kafka WAL -> S3 archive)
- Execute paths: success, fallback, partial availability
- Idempotency tracking and result metadata
"""

from __future__ import annotations

import sqlite3

import pytest

from dbaas.entdb_server.tools.recovery_strategy import (
    RecoveryStrategy,
    RecoveryTier,
)

# ---------------------------------------------------------------------------
# Mock providers
# ---------------------------------------------------------------------------


class MockSnapshotProvider:
    def __init__(self, snapshot: dict | None = None):
        self._snapshot = snapshot

    async def find_latest_snapshot(self, tenant_id: str) -> dict | None:
        return self._snapshot

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
        conn.commit()
        conn.close()


class MockKafkaProvider:
    def __init__(self, has_coverage: bool = True, events_to_replay: int = 5):
        self._has_coverage = has_coverage
        self._events = events_to_replay

    async def check_kafka_coverage(self, topic: str, start_pos: str | None) -> bool:
        return self._has_coverage

    async def replay_from_kafka(
        self,
        topic: str,
        start_pos: str | None,
        db_path: str,
        timeout_seconds: int,
    ) -> tuple[int, str | None]:
        return (self._events, f"{topic}:0:{self._events + 100}")


class MockArchiveProvider:
    def __init__(self, events_to_replay: int = 10):
        self._events = events_to_replay

    async def replay_from_archive(
        self,
        tenant_id: str,
        start_pos: str | None,
        db_path: str,
    ) -> tuple[int, str | None]:
        return (self._events, f"entdb-wal:0:{self._events + 50}")


# ---------------------------------------------------------------------------
# Plan-construction tests
# ---------------------------------------------------------------------------


@pytest.mark.unit
class TestRecoveryPlan:
    """Tests for recovery plan construction."""

    @pytest.mark.asyncio
    async def test_plan_with_snapshot_and_kafka(self):
        """Snapshot + Kafka enabled yields [SNAPSHOT, KAFKA_WAL]."""
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "snap-1"}),
            kafka_provider=MockKafkaProvider(),
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )

        plan = await strategy.build_plan("tenant_1")

        assert RecoveryTier.SNAPSHOT in plan.tiers
        assert RecoveryTier.KAFKA_WAL in plan.tiers
        assert RecoveryTier.S3_ARCHIVE not in plan.tiers

    @pytest.mark.asyncio
    async def test_plan_with_all_tiers(self):
        """All providers available yields [SNAPSHOT, KAFKA_WAL, S3_ARCHIVE]."""
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "snap-1"}),
            kafka_provider=MockKafkaProvider(),
            archive_provider=MockArchiveProvider(),
            kafka_replay_enabled=True,
            archive_replay_enabled=True,
        )

        plan = await strategy.build_plan("tenant_1")

        assert plan.tiers == [
            RecoveryTier.SNAPSHOT,
            RecoveryTier.KAFKA_WAL,
            RecoveryTier.S3_ARCHIVE,
        ]

    @pytest.mark.asyncio
    async def test_plan_kafka_disabled(self):
        """kafka_replay_enabled=False excludes KAFKA_WAL."""
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "snap-1"}),
            kafka_provider=MockKafkaProvider(),
            archive_provider=MockArchiveProvider(),
            kafka_replay_enabled=False,
            archive_replay_enabled=True,
        )

        plan = await strategy.build_plan("tenant_1")

        assert RecoveryTier.KAFKA_WAL not in plan.tiers
        assert RecoveryTier.SNAPSHOT in plan.tiers
        assert RecoveryTier.S3_ARCHIVE in plan.tiers

    @pytest.mark.asyncio
    async def test_plan_archive_disabled(self):
        """archive_replay_enabled=False excludes S3_ARCHIVE."""
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot={"s3_key": "snap-1"}),
            kafka_provider=MockKafkaProvider(),
            archive_provider=MockArchiveProvider(),
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )

        plan = await strategy.build_plan("tenant_1")

        assert RecoveryTier.S3_ARCHIVE not in plan.tiers
        assert RecoveryTier.SNAPSHOT in plan.tiers
        assert RecoveryTier.KAFKA_WAL in plan.tiers

    @pytest.mark.asyncio
    async def test_plan_no_snapshot(self):
        """No snapshot available skips SNAPSHOT tier."""
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(snapshot=None),
            kafka_provider=MockKafkaProvider(),
            archive_provider=MockArchiveProvider(),
            kafka_replay_enabled=True,
            archive_replay_enabled=True,
        )

        plan = await strategy.build_plan("tenant_1")

        assert RecoveryTier.SNAPSHOT not in plan.tiers
        assert RecoveryTier.KAFKA_WAL in plan.tiers
        assert RecoveryTier.S3_ARCHIVE in plan.tiers

    @pytest.mark.asyncio
    async def test_plan_nothing_available(self):
        """No providers at all yields empty tiers."""
        strategy = RecoveryStrategy(
            snapshot_provider=None,
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )

        plan = await strategy.build_plan("tenant_1")

        assert plan.tiers == []


# ---------------------------------------------------------------------------
# Execution tests
# ---------------------------------------------------------------------------


@pytest.mark.unit
class TestRecoveryExecution:
    """Tests for recovery execution paths."""

    @pytest.mark.asyncio
    async def test_execute_snapshot_then_kafka(self, tmp_path):
        """Snapshot restores, then Kafka replays events."""
        db_path = str(tmp_path / "tenant_1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-1", "last_stream_pos": "entdb-wal:0:50"},
            ),
            kafka_provider=MockKafkaProvider(has_coverage=True, events_to_replay=5),
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )

        result = await strategy.execute("tenant_1", db_path)

        assert result.success
        assert len(result.tier_results) >= 2

        snapshot_tier = result.tier_results[0]
        assert snapshot_tier.tier == RecoveryTier.SNAPSHOT

        kafka_tier = result.tier_results[1]
        assert kafka_tier.tier == RecoveryTier.KAFKA_WAL
        assert kafka_tier.events_replayed == 5

    @pytest.mark.asyncio
    async def test_execute_kafka_fallback_to_archive(self, tmp_path):
        """Kafka has no coverage; falls through to archive tier."""
        db_path = str(tmp_path / "tenant_1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-1", "last_stream_pos": "entdb-wal:0:50"},
            ),
            kafka_provider=MockKafkaProvider(has_coverage=False),
            archive_provider=MockArchiveProvider(events_to_replay=10),
            kafka_replay_enabled=True,
            archive_replay_enabled=True,
        )

        result = await strategy.execute("tenant_1", db_path)

        assert result.success

        # Archive tier should have been used
        tier_names = [tr.tier for tr in result.tier_results]
        assert RecoveryTier.S3_ARCHIVE in tier_names

        archive_tier = next(
            tr for tr in result.tier_results if tr.tier == RecoveryTier.S3_ARCHIVE
        )
        assert archive_tier.events_replayed == 10

    @pytest.mark.asyncio
    async def test_execute_snapshot_only(self, tmp_path):
        """Only snapshot available — no WAL replay."""
        db_path = str(tmp_path / "tenant_1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-1"},
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )

        result = await strategy.execute("tenant_1", db_path)

        assert result.success
        assert len(result.tier_results) == 1
        assert result.tier_results[0].tier == RecoveryTier.SNAPSHOT

    @pytest.mark.asyncio
    async def test_execute_all_tiers_fail(self, tmp_path):
        """No providers available results in failure."""
        db_path = str(tmp_path / "tenant_1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=None,
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )

        result = await strategy.execute("tenant_1", db_path)

        assert not result.success

    @pytest.mark.asyncio
    async def test_execute_idempotent_events(self, tmp_path):
        """TierResult tracks events_replayed correctly."""
        db_path = str(tmp_path / "tenant_1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-1", "last_stream_pos": "entdb-wal:0:50"},
            ),
            kafka_provider=MockKafkaProvider(has_coverage=True, events_to_replay=7),
            archive_provider=None,
            kafka_replay_enabled=True,
            archive_replay_enabled=False,
        )

        result = await strategy.execute("tenant_1", db_path)

        assert result.success
        kafka_tier = next(
            tr for tr in result.tier_results if tr.tier == RecoveryTier.KAFKA_WAL
        )
        assert kafka_tier.events_replayed == 7

    @pytest.mark.asyncio
    async def test_recovery_result_duration(self, tmp_path):
        """Result captures duration_ms."""
        db_path = str(tmp_path / "tenant_1.db")
        strategy = RecoveryStrategy(
            snapshot_provider=MockSnapshotProvider(
                snapshot={"s3_key": "snap-1"},
            ),
            kafka_provider=None,
            archive_provider=None,
            kafka_replay_enabled=False,
            archive_replay_enabled=False,
        )

        result = await strategy.execute("tenant_1", db_path)

        assert result.duration_ms is not None
        assert result.duration_ms >= 0
