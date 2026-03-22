"""
Tiered recovery strategy for EntDB.

Recovery priority order:
  1. Snapshot — restore latest SQLite backup from S3 (always first)
  2. Kafka WAL — replay events from snapshot position to present
  3. S3 Archive — replay archived events (last resort, if Kafka expired)

The strategy first restores the most recent snapshot, then fills the gap
between the snapshot and the present using the fastest available source.
Kafka is preferred because it's faster (direct stream) and has the freshest
data. The S3 archive is only used if Kafka retention has expired past the
snapshot position.

Invariants:
    - Recovery is idempotent (all event application checks idempotency keys)
    - Each tier is attempted independently — failure in one falls through
    - The strategy never modifies Kafka or S3 data
    - Recovery reports exactly which tiers were used

How to change safely:
    - Add new tiers by extending RecoveryTier enum
    - Test each tier independently with mocked dependencies
"""

from __future__ import annotations

import logging
import time
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, Protocol

logger = logging.getLogger(__name__)


class RecoveryTier(Enum):
    """Recovery data source tiers, in priority order."""

    SNAPSHOT = "snapshot"
    KAFKA_WAL = "kafka_wal"
    S3_ARCHIVE = "s3_archive"


@dataclass
class TierResult:
    """Result of attempting a single recovery tier.

    Attributes:
        tier: Which tier was attempted
        success: Whether the tier succeeded
        events_replayed: Number of events applied from this tier
        final_stream_pos: Stream position after this tier
        error: Error message if failed
        skipped: Whether the tier was skipped (disabled or unavailable)
        skip_reason: Why the tier was skipped
    """

    tier: RecoveryTier
    success: bool
    events_replayed: int = 0
    final_stream_pos: str | None = None
    error: str | None = None
    skipped: bool = False
    skip_reason: str | None = None


@dataclass
class RecoveryPlan:
    """Computed plan for recovering a tenant.

    Attributes:
        tenant_id: Tenant to recover
        tiers: Ordered list of tiers to attempt
        snapshot_key: S3 key of the snapshot to restore (if found)
        snapshot_stream_pos: Last stream position in the snapshot
    """

    tenant_id: str
    tiers: list[RecoveryTier] = field(default_factory=list)
    snapshot_key: str | None = None
    snapshot_stream_pos: str | None = None


@dataclass
class RecoveryResult:
    """Full result of the tiered recovery operation.

    Attributes:
        success: Whether recovery succeeded
        tier_results: Results for each tier attempted
        snapshot_used: S3 key of snapshot that was restored
        total_events_replayed: Total events applied across all tiers
        final_stream_pos: Final stream position after recovery
        duration_ms: Total recovery duration in milliseconds
        error: Error message if failed
    """

    success: bool
    tier_results: list[TierResult] = field(default_factory=list)
    snapshot_used: str | None = None
    total_events_replayed: int = 0
    final_stream_pos: str | None = None
    duration_ms: int = 0
    error: str | None = None


class SnapshotProvider(Protocol):
    """Protocol for finding and restoring snapshots."""

    async def find_latest_snapshot(self, tenant_id: str) -> dict[str, Any] | None:
        """Find the latest snapshot for a tenant. Returns manifest dict or None."""
        ...

    async def restore_snapshot(self, snapshot: dict[str, Any], db_path: str) -> None:
        """Download and restore a snapshot to the given database path."""
        ...


class KafkaReplayProvider(Protocol):
    """Protocol for replaying events from Kafka."""

    async def check_kafka_coverage(self, topic: str, start_pos: str | None) -> bool:
        """Check if Kafka still has events from the given position."""
        ...

    async def replay_from_kafka(
        self, topic: str, start_pos: str | None, db_path: str, timeout_seconds: int
    ) -> tuple[int, str | None]:
        """Replay events from Kafka. Returns (events_replayed, final_pos)."""
        ...


class ArchiveReplayProvider(Protocol):
    """Protocol for replaying events from S3 archive."""

    async def replay_from_archive(
        self, tenant_id: str, start_pos: str | None, db_path: str
    ) -> tuple[int, str | None]:
        """Replay events from archive. Returns (events_replayed, final_pos)."""
        ...


class RecoveryStrategy:
    """Implements tiered recovery logic.

    The strategy computes a plan based on available data sources and
    configuration, then executes each tier in order. Each tier fills
    the gap between the current database state and the present.

    Example:
        >>> strategy = RecoveryStrategy(
        ...     snapshot_provider=snapshot_svc,
        ...     kafka_provider=kafka_svc,
        ...     archive_provider=archive_svc,
        ...     kafka_replay_enabled=True,
        ...     archive_replay_enabled=True,
        ... )
        >>> plan = await strategy.plan("tenant-123")
        >>> result = await strategy.execute(plan, "/data/tenant_tenant-123.db")
    """

    def __init__(
        self,
        snapshot_provider: SnapshotProvider | None = None,
        kafka_provider: KafkaReplayProvider | None = None,
        archive_provider: ArchiveReplayProvider | None = None,
        kafka_replay_enabled: bool = True,
        archive_replay_enabled: bool = True,
        kafka_replay_timeout_seconds: int = 300,
        kafka_topic: str = "entdb-wal",
        verify_after_recovery: bool = True,
    ) -> None:
        self._snapshot = snapshot_provider
        self._kafka = kafka_provider
        self._archive = archive_provider
        self._kafka_enabled = kafka_replay_enabled
        self._archive_enabled = archive_replay_enabled
        self._kafka_timeout = kafka_replay_timeout_seconds
        self._kafka_topic = kafka_topic
        self._verify = verify_after_recovery

    async def plan(self, tenant_id: str) -> RecoveryPlan:
        """Analyze available data sources and create a recovery plan.

        Args:
            tenant_id: The tenant to recover

        Returns:
            RecoveryPlan with ordered tiers and snapshot info
        """
        recovery_plan = RecoveryPlan(tenant_id=tenant_id)

        # Tier 1: Always try snapshot first
        if self._snapshot:
            snapshot = await self._snapshot.find_latest_snapshot(tenant_id)
            if snapshot:
                recovery_plan.tiers.append(RecoveryTier.SNAPSHOT)
                recovery_plan.snapshot_key = snapshot.get("s3_key")
                recovery_plan.snapshot_stream_pos = snapshot.get("last_stream_pos")

        # Tier 2: Kafka WAL replay (if enabled and provider available)
        if self._kafka_enabled and self._kafka:
            recovery_plan.tiers.append(RecoveryTier.KAFKA_WAL)

        # Tier 3: S3 Archive replay (last resort)
        if self._archive_enabled and self._archive:
            recovery_plan.tiers.append(RecoveryTier.S3_ARCHIVE)

        logger.info(
            "Recovery plan created",
            extra={
                "tenant_id": tenant_id,
                "tiers": [t.value for t in recovery_plan.tiers],
                "has_snapshot": recovery_plan.snapshot_key is not None,
                "snapshot_pos": recovery_plan.snapshot_stream_pos,
            },
        )

        return recovery_plan

    async def build_plan(self, tenant_id: str) -> RecoveryPlan:
        """Alias for plan() for readability."""
        return await self.plan(tenant_id)

    async def execute(self, plan_or_tenant: RecoveryPlan | str, db_path: str) -> RecoveryResult:
        """Execute the recovery plan tier by tier.

        Args:
            plan_or_tenant: A RecoveryPlan or a tenant_id string.
                If a string is passed, plan() is called automatically.
            db_path: Path to the tenant's SQLite database file

        Returns:
            RecoveryResult with details of each tier attempted
        """
        if isinstance(plan_or_tenant, str):
            plan = await self.plan(plan_or_tenant)
        else:
            plan = plan_or_tenant

        start_time = time.time()
        result = RecoveryResult(success=False)
        current_pos = None

        if not plan.tiers:
            result.error = "No recovery tiers available — no snapshot, Kafka, or archive found"
            result.duration_ms = int((time.time() - start_time) * 1000)
            return result

        for tier in plan.tiers:
            tier_result = await self._execute_tier(tier, plan, db_path, current_pos)
            result.tier_results.append(tier_result)

            if tier_result.success:
                result.total_events_replayed += tier_result.events_replayed
                if tier_result.final_stream_pos:
                    current_pos = tier_result.final_stream_pos

                if tier == RecoveryTier.SNAPSHOT:
                    result.snapshot_used = plan.snapshot_key
                    current_pos = plan.snapshot_stream_pos

                # Snapshot restores the base; WAL/archive tiers fill the gap.
                # If a WAL or archive tier succeeds, recovery is complete.
                if tier in (RecoveryTier.KAFKA_WAL, RecoveryTier.S3_ARCHIVE):
                    result.success = True
                    break

            elif tier_result.skipped:
                continue
            else:
                # Tier failed — try next tier
                logger.warning(
                    f"Recovery tier {tier.value} failed: {tier_result.error}. " "Trying next tier."
                )
                continue

        # If only snapshot was available and succeeded, that's still a success
        if not result.success and len(result.tier_results) > 0:
            snapshot_results = [
                r for r in result.tier_results if r.tier == RecoveryTier.SNAPSHOT and r.success
            ]
            if snapshot_results:
                result.success = True

        # Verify database integrity if requested
        if result.success and self._verify:
            try:
                import sqlite3

                conn = sqlite3.connect(db_path)
                try:
                    cursor = conn.execute("PRAGMA integrity_check")
                    check = cursor.fetchone()[0]
                    if check != "ok":
                        result.success = False
                        result.error = f"Database integrity check failed: {check}"
                finally:
                    conn.close()
            except Exception as e:
                result.success = False
                result.error = f"Database verification failed: {e}"

        result.final_stream_pos = current_pos
        result.duration_ms = int((time.time() - start_time) * 1000)

        logger.info(
            "Recovery complete",
            extra={
                "tenant_id": plan.tenant_id,
                "success": result.success,
                "snapshot_used": result.snapshot_used,
                "total_events": result.total_events_replayed,
                "final_pos": result.final_stream_pos,
                "duration_ms": result.duration_ms,
                "tiers_attempted": [r.tier.value for r in result.tier_results],
            },
        )

        return result

    async def _execute_tier(
        self,
        tier: RecoveryTier,
        plan: RecoveryPlan,
        db_path: str,
        current_pos: str | None,
    ) -> TierResult:
        """Execute a single recovery tier.

        Args:
            tier: Which tier to execute
            plan: The full recovery plan (for snapshot info)
            db_path: Database file path
            current_pos: Current stream position after previous tiers

        Returns:
            TierResult with outcome
        """
        try:
            if tier == RecoveryTier.SNAPSHOT:
                return await self._execute_snapshot(plan, db_path)
            elif tier == RecoveryTier.KAFKA_WAL:
                return await self._execute_kafka(plan, db_path, current_pos)
            elif tier == RecoveryTier.S3_ARCHIVE:
                return await self._execute_archive(plan, db_path, current_pos)
            else:
                return TierResult(tier=tier, success=False, error=f"Unknown tier: {tier}")
        except Exception as e:
            logger.error(f"Tier {tier.value} error: {e}", exc_info=True)
            return TierResult(tier=tier, success=False, error=str(e))

    async def _execute_snapshot(self, plan: RecoveryPlan, db_path: str) -> TierResult:
        """Restore from snapshot."""
        if not self._snapshot or not plan.snapshot_key:
            return TierResult(
                tier=RecoveryTier.SNAPSHOT,
                success=False,
                skipped=True,
                skip_reason="No snapshot available",
            )

        snapshot = await self._snapshot.find_latest_snapshot(plan.tenant_id)
        if not snapshot:
            return TierResult(
                tier=RecoveryTier.SNAPSHOT,
                success=False,
                skipped=True,
                skip_reason="Snapshot not found",
            )

        await self._snapshot.restore_snapshot(snapshot, db_path)

        return TierResult(
            tier=RecoveryTier.SNAPSHOT,
            success=True,
            final_stream_pos=plan.snapshot_stream_pos,
        )

    async def _execute_kafka(
        self,
        plan: RecoveryPlan,
        db_path: str,
        current_pos: str | None,
    ) -> TierResult:
        """Replay from Kafka WAL."""
        if not self._kafka:
            return TierResult(
                tier=RecoveryTier.KAFKA_WAL,
                success=False,
                skipped=True,
                skip_reason="Kafka provider not available",
            )

        # Check if Kafka has data from our position
        start_pos = current_pos or plan.snapshot_stream_pos
        has_data = await self._kafka.check_kafka_coverage(self._kafka_topic, start_pos)

        if not has_data:
            return TierResult(
                tier=RecoveryTier.KAFKA_WAL,
                success=False,
                error="Kafka data does not cover the required position "
                "(retention may have expired)",
            )

        events, final_pos = await self._kafka.replay_from_kafka(
            self._kafka_topic, start_pos, db_path, self._kafka_timeout
        )

        return TierResult(
            tier=RecoveryTier.KAFKA_WAL,
            success=True,
            events_replayed=events,
            final_stream_pos=final_pos,
        )

    async def _execute_archive(
        self,
        plan: RecoveryPlan,
        db_path: str,
        current_pos: str | None,
    ) -> TierResult:
        """Replay from S3 archive."""
        if not self._archive:
            return TierResult(
                tier=RecoveryTier.S3_ARCHIVE,
                success=False,
                skipped=True,
                skip_reason="Archive provider not available",
            )

        start_pos = current_pos or plan.snapshot_stream_pos
        events, final_pos = await self._archive.replay_from_archive(
            plan.tenant_id, start_pos, db_path
        )

        return TierResult(
            tier=RecoveryTier.S3_ARCHIVE,
            success=True,
            events_replayed=events,
            final_stream_pos=final_pos,
        )
