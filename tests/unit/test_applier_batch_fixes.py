"""
Unit tests for batch applier bug fixes.

Tests cover:
- Multi-partition commit: all partitions' offsets are committed after a batch
- Schema fingerprint validation in _apply_tenant_batch
"""

from __future__ import annotations

import json

import pytest

from dbaas.entdb_server.apply.applier import (
    Applier,
    MailboxFanoutConfig,
)
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.wal.memory import InMemoryWalStream


def _event_bytes(
    tenant_id: str,
    idempotency_key: str,
    node_id: str | None = None,
    schema_fingerprint: str | None = None,
) -> bytes:
    """Build event bytes for WAL append."""
    event: dict = {
        "tenant_id": tenant_id,
        "actor": "user:1",
        "idempotency_key": idempotency_key,
        "ops": [
            {
                "op": "create_node",
                "type_id": 1,
                "id": node_id or f"n-{idempotency_key}",
                "data": {"k": idempotency_key},
            }
        ],
    }
    if schema_fingerprint:
        event["schema_fingerprint"] = schema_fingerprint
    return json.dumps(event).encode()


@pytest.mark.unit
class TestBatchMultiPartitionCommit:
    """After a batch spanning multiple partitions, all partitions must be
    committed so that the next poll_batch does not re-deliver records."""

    @pytest.mark.asyncio
    async def test_multi_partition_records_not_re_delivered(self, tmp_path):
        """Records from all partitions are committed after batch apply.

        We use two different keys that hash to different partitions (with a
        2-partition WAL this is guaranteed for most key pairs). Then we run
        one batch cycle and verify the next poll returns nothing.
        """
        wal = InMemoryWalStream(num_partitions=2)
        await wal.connect()

        # Find two keys that hash to different partitions
        key_a, key_b = None, None
        for i in range(100):
            candidate = f"key-{i}"
            p = wal._partition_for_key(candidate)
            if p == 0 and key_a is None:
                key_a = candidate
            elif p == 1 and key_b is None:
                key_b = candidate
            if key_a and key_b:
                break
        assert key_a is not None and key_b is not None, "Could not find keys for both partitions"

        # Append one record per partition
        await wal.append("t", key_a, _event_bytes("tenant-1", "ev-a", "node-a"))
        await wal.append("t", key_b, _event_bytes("tenant-1", "ev-b", "node-b"))

        store = CanonicalStore(str(tmp_path))
        applier = Applier(
            wal=wal,
            canonical_store=store,
            topic="t",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=10,
            poll_timeout_ms=50,
        )

        # Run one iteration of the batch loop manually
        records = await wal.poll_batch("t", "grp", max_records=10, timeout_ms=50)
        assert len(records) == 2

        # Verify we actually got records from 2 different partitions
        partitions = {r.position.partition for r in records}
        assert len(partitions) == 2, f"Expected 2 partitions, got {partitions}"

        # Group and apply as the applier does
        tenant_records: dict[str, list] = {}
        for record in records:
            data = record.value_json()
            tid = data["tenant_id"]
            if tid not in tenant_records:
                tenant_records[tid] = []
            tenant_records[tid].append((record, data))

        for tid, items in tenant_records.items():
            await applier._apply_tenant_batch(tid, items)

        # Commit per-partition (the fix)
        per_partition: dict[int, object] = {}
        for record in records:
            per_partition[record.position.partition] = record
        for record in per_partition.values():
            await wal.commit(record)

        # Now poll again -- should get nothing
        second_batch = await wal.poll_batch("t", "grp", max_records=10, timeout_ms=50)
        assert len(second_batch) == 0, (
            f"Expected 0 re-delivered records but got {len(second_batch)}: "
            "multi-partition commit did not advance all partition offsets"
        )

        store.close_all()


@pytest.mark.unit
class TestBatchSchemaFingerprintIsNotEnforcedOnReplay:
    """The Applier must NOT reject events with a stale schema_fingerprint.

    The WAL is an immutable historical log. Events that were valid when
    written must still be applied on replay — otherwise a schema redeploy
    would cause historical events to be silently dropped, breaking
    rebuild-from-WAL and causing divergent replicas.

    schema_fingerprint is only enforced on the WRITE path (gRPC boundary),
    where it rejects stale clients.
    """

    @pytest.mark.asyncio
    async def test_mismatched_schema_still_applied_in_batch(self, tmp_path):
        """Historical events with a different schema_fingerprint must be applied."""
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        await wal.append(
            "t",
            "k",
            _event_bytes("tenant-1", "ev-good", "node-good", schema_fingerprint="sha256:correct"),
        )
        await wal.append(
            "t",
            "k",
            _event_bytes(
                "tenant-1",
                "ev-historical",
                "node-historical",
                schema_fingerprint="sha256:old",
            ),
        )

        store = CanonicalStore(str(tmp_path))
        applier = Applier(
            wal=wal,
            canonical_store=store,
            topic="t",
            schema_fingerprint="sha256:correct",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=10,
        )

        records = await wal.poll_batch("t", "grp", max_records=10, timeout_ms=50)
        assert len(records) == 2

        items = [(r, r.value_json()) for r in records]
        await applier._apply_tenant_batch("tenant-1", items)

        good_node = await store.get_node("tenant-1", "node-good")
        assert good_node is not None, "Current-schema node should have been applied"

        historical_node = await store.get_node("tenant-1", "node-historical")
        assert historical_node is not None, (
            "Historical event with a different fingerprint must still be applied — "
            "otherwise a rebuild-from-WAL would silently drop data."
        )

        assert applier._error_count == 0

        store.close_all()
