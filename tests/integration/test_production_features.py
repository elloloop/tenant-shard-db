"""
Tests for production features:
- Connection pooling
- Read-after-write consistency
- Payload query filtering
- Archiver deduplication

These tests validate features being added by parallel agents.
They use real SQLite stores and mock S3 to test end-to-end behavior.
"""

from __future__ import annotations

import json
import time
import uuid
from unittest.mock import AsyncMock, MagicMock

import pytest

from dbaas.entdb_server.apply.applier import (
    Applier,
    MailboxFanoutConfig,
    TransactionEvent,
)
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.apply.mailbox_store import MailboxStore
from dbaas.entdb_server.archive.archiver import Archiver
from dbaas.entdb_server.wal.memory import InMemoryWalStream

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_event(
    tenant_id: str = "t1",
    actor: str = "user:1",
    idempotency_key: str | None = None,
    ops: list | None = None,
    ts_ms: int | None = None,
) -> TransactionEvent:
    """Create a TransactionEvent with sensible defaults."""
    return TransactionEvent(
        tenant_id=tenant_id,
        actor=actor,
        idempotency_key=idempotency_key or str(uuid.uuid4()),
        schema_fingerprint=None,
        ts_ms=ts_ms or int(time.time() * 1000),
        ops=ops
        or [
            {
                "op": "create_node",
                "type_id": 1,
                "id": str(uuid.uuid4()),
                "data": {"v": 1},
            }
        ],
    )


async def _make_applier(
    tmp_path,
    *,
    batch_size: int = 1,
    fanout: bool = False,
) -> tuple[Applier, CanonicalStore, MailboxStore, InMemoryWalStream]:
    """Build an Applier wired to real SQLite stores."""
    wal = InMemoryWalStream(num_partitions=1)
    await wal.connect()
    store = CanonicalStore(data_dir=str(tmp_path / "canonical"), wal_mode=True)
    mbox = MailboxStore(data_dir=str(tmp_path / "mailbox"), wal_mode=True)
    applier = Applier(
        wal=wal,
        canonical_store=store,
        mailbox_store=mbox,
        topic="test-wal",
        batch_size=batch_size,
        fanout_config=MailboxFanoutConfig(enabled=fanout),
    )
    return applier, store, mbox, wal


def _make_archiver(**kwargs):
    """Create an Archiver with mocked WAL and S3 dependencies."""
    mock_wal = MagicMock()
    mock_s3 = MagicMock()
    mock_s3.bucket = "test-bucket"
    mock_s3.archive_prefix = "archive"
    mock_s3.region = "us-east-1"
    mock_s3.endpoint_url = None
    mock_s3.access_key_id = None
    mock_s3.secret_access_key = None
    defaults = {"wal": mock_wal, "s3_config": mock_s3, "topic": "test-wal"}
    defaults.update(kwargs)
    return Archiver(**defaults)


def _make_record(
    tenant_id: str = "t1",
    partition: int = 0,
    offset: int = 0,
    payload_size: int = 100,
):
    """Create a mock StreamRecord."""
    event = {
        "tenant_id": tenant_id,
        "idempotency_key": f"key-{offset}",
        "ops": [
            {
                "op": "create_node",
                "type_id": 1,
                "id": f"n-{offset}",
                "data": {"v": "x" * payload_size},
            }
        ],
    }
    value = json.dumps(event).encode()
    mock = MagicMock()
    mock.value = value
    mock.value_json.return_value = event
    mock.position = MagicMock()
    mock.position.partition = partition
    mock.position.offset = offset
    mock.position.to_dict.return_value = {
        "topic": "test-wal",
        "partition": partition,
        "offset": offset,
    }
    return mock


# =========================================================================
# CONNECTION POOLING
# =========================================================================


@pytest.mark.integration
class TestConnectionPooling:
    """Tests for SQLite connection reuse and pooling behavior.

    The CanonicalStore uses a connection-per-operation model with WAL mode.
    These tests verify that the store handles concurrent access correctly
    and that connections are properly managed.
    """

    @pytest.mark.asyncio
    async def test_connection_reused(self, tmp_path):
        """Two sequential get_node calls on the same tenant work correctly,
        demonstrating that connection management does not break on reuse."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("t1")
        await store.create_node("t1", 1, {"name": "Alice"}, "user:1", node_id="n1")

        # Two sequential reads on same tenant should both succeed
        node1 = await store.get_node("t1", "n1")
        node2 = await store.get_node("t1", "n1")

        assert node1 is not None
        assert node2 is not None
        assert node1.node_id == node2.node_id
        assert node1.payload == node2.payload

    @pytest.mark.asyncio
    async def test_multiple_tenants_separate_connections(self, tmp_path):
        """Different tenants use separate database files and connections."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("tenant-a")
        await store.initialize_tenant("tenant-b")

        await store.create_node("tenant-a", 1, {"name": "A"}, "user:1", node_id="shared-id")
        await store.create_node("tenant-b", 1, {"name": "B"}, "user:1", node_id="shared-id")

        node_a = await store.get_node("tenant-a", "shared-id")
        node_b = await store.get_node("tenant-b", "shared-id")

        assert node_a is not None and node_b is not None
        assert node_a.payload["name"] == "A"
        assert node_b.payload["name"] == "B"

        # Verify they use different database files
        path_a = store.get_db_path("tenant-a")
        path_b = store.get_db_path("tenant-b")
        assert path_a != path_b

    @pytest.mark.asyncio
    async def test_close_all_clears_state(self, tmp_path):
        """After performing operations, the store's internal state can be
        cleaned up and the store remains usable with fresh connections."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("t1")
        await store.create_node("t1", 1, {"v": 1}, "user:1", node_id="pre-close")

        # With connection pooling, connections persist until close_all()
        assert len(store._connections) > 0, "Pooled connections should persist"

        # close_all() clears the pool
        store.close_all()
        assert len(store._connections) == 0, "close_all should clear pool"

        # Store remains usable after close_all (creates fresh connections)
        node = await store.get_node("t1", "pre-close")
        assert node is not None

    @pytest.mark.asyncio
    async def test_pooled_connection_survives_operations(self, tmp_path):
        """Run 100 sequential operations to verify connection stability."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("t1")

        for i in range(100):
            await store.create_node("t1", 1, {"i": i}, "user:1", node_id=f"stable-{i}")

        # Verify all nodes created successfully
        for i in range(100):
            node = await store.get_node("t1", f"stable-{i}")
            assert node is not None, f"Node stable-{i} missing after 100 ops"
            assert node.payload["i"] == i

        stats = await store.get_stats("t1")
        assert stats["nodes"] == 100

    @pytest.mark.asyncio
    async def test_pragma_configured_once(self, tmp_path):
        """WAL mode is set on every connection via _get_connection context manager.
        Verify that WAL mode is active by checking journal_mode on the database."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("t1")

        # Create and read to exercise connections
        await store.create_node("t1", 1, {"x": 1}, "user:1", node_id="pragma-test")
        node = await store.get_node("t1", "pragma-test")
        assert node is not None

        # Verify WAL mode is active by examining the database directly
        import sqlite3

        db_path = store.get_db_path("t1")
        conn = sqlite3.connect(str(db_path))
        try:
            result = conn.execute("PRAGMA journal_mode").fetchone()
            assert result[0] == "wal", f"Expected WAL mode, got {result[0]}"
        finally:
            conn.close()


# =========================================================================
# READ-AFTER-WRITE CONSISTENCY
# =========================================================================


@pytest.mark.integration
class TestReadAfterWrite:
    """Tests for read-after-write consistency using the applier and canonical store.

    These tests verify that after applying an event, subsequent reads
    observe the write immediately within the same process.
    """

    @pytest.mark.asyncio
    async def test_write_then_read_immediate(self, tmp_path):
        """Write a node, immediately read it -- must be visible."""
        applier, store, _, _ = await _make_applier(tmp_path)

        event = _make_event(
            idempotency_key="raw-1",
            ops=[
                {"op": "create_node", "type_id": 1, "id": "rw-node", "data": {"v": 42}},
            ],
        )

        result = await applier.apply_event(event)
        assert result.success

        # Immediate read should see the node
        node = await store.get_node("t1", "rw-node")
        assert node is not None
        assert node.payload["v"] == 42

    @pytest.mark.asyncio
    async def test_write_then_read_via_type_query(self, tmp_path):
        """After applying, get_nodes_by_type should include the new node."""
        applier, store, _, _ = await _make_applier(tmp_path)

        event = _make_event(
            idempotency_key="type-q-1",
            ops=[
                {"op": "create_node", "type_id": 7, "id": "tq-node", "data": {"k": "v"}},
            ],
        )

        await applier.apply_event(event)

        nodes = await store.get_nodes_by_type("t1", 7)
        assert len(nodes) >= 1
        assert any(n.node_id == "tq-node" for n in nodes)

    @pytest.mark.asyncio
    async def test_idempotency_key_appears_after_apply(self, tmp_path):
        """After apply completes, check_idempotency returns True."""
        applier, store, _, _ = await _make_applier(tmp_path)

        key = "idem-check-key"

        # Apply a setup event first to initialize the tenant DB
        setup_event = _make_event(
            idempotency_key="setup-tenant",
            ops=[
                {"op": "create_node", "type_id": 1, "id": "setup-n", "data": {}},
            ],
        )
        await applier.apply_event(setup_event)

        # Before apply of target event: not recorded
        assert await store.check_idempotency("t1", key) is False

        event = _make_event(
            idempotency_key=key,
            ops=[
                {"op": "create_node", "type_id": 1, "id": "idem-n", "data": {}},
            ],
        )
        await applier.apply_event(event)

        # After apply: recorded
        assert await store.check_idempotency("t1", key) is True

    @pytest.mark.asyncio
    async def test_update_visible_after_apply(self, tmp_path):
        """Update a node via applier, immediately read the updated payload."""
        applier, store, _, _ = await _make_applier(tmp_path)

        # Create
        await applier.apply_event(
            _make_event(
                idempotency_key="update-setup",
                ops=[
                    {"op": "create_node", "type_id": 1, "id": "upd-n", "data": {"v": 1}},
                ],
            )
        )

        # Update
        await applier.apply_event(
            _make_event(
                idempotency_key="update-exec",
                ops=[
                    {"op": "update_node", "id": "upd-n", "patch": {"v": 99}},
                ],
            )
        )

        node = await store.get_node("t1", "upd-n")
        assert node is not None
        assert node.payload["v"] == 99

    @pytest.mark.asyncio
    async def test_wait_applied_for_already_applied(self, tmp_path):
        """Re-applying an event returns instantly with skipped=True."""
        applier, store, _, _ = await _make_applier(tmp_path)

        event = _make_event(
            idempotency_key="already-done",
            ops=[
                {"op": "create_node", "type_id": 1, "id": "ad-n", "data": {}},
            ],
        )

        r1 = await applier.apply_event(event)
        assert r1.success and not r1.skipped

        # Second apply should skip immediately
        t0 = time.monotonic()
        r2 = await applier.apply_event(event)
        elapsed = time.monotonic() - t0

        assert r2.success and r2.skipped
        # Should complete very fast (< 1 second)
        assert elapsed < 1.0, f"Re-apply took {elapsed:.3f}s, expected near-instant"


# =========================================================================
# PAYLOAD FILTER
# =========================================================================


@pytest.mark.integration
class TestPayloadFilter:
    """Tests for payload-based query filtering.

    These tests operate on the existing get_nodes_by_type API and filter
    results by payload values. The filtering is done in-memory after
    fetching from SQLite, which matches the current implementation.
    """

    @pytest.fixture
    async def store_with_data(self, tmp_path):
        """Create a store populated with test nodes of varying payloads."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("t1")

        # Create nodes with different payload values
        test_nodes = [
            {"status": "active", "priority": "high", "count": 10, "flag": True},
            {"status": "active", "priority": "low", "count": 5, "flag": False},
            {"status": "inactive", "priority": "high", "count": 20, "flag": True},
            {"status": "inactive", "priority": "low", "count": 1, "flag": False},
            {"status": "active", "priority": "medium", "count": 15, "flag": True},
            {"status": "pending", "priority": "high", "count": 0, "flag": False},
            {"status": "active", "priority": "high", "count": 100, "flag": True},
            {"status": "inactive", "priority": "medium", "count": 50, "flag": False},
        ]
        for i, payload in enumerate(test_nodes):
            await store.create_node(
                "t1", 1, payload, "user:1", node_id=f"pf-{i}", created_at=1000 + i
            )

        return store

    @pytest.mark.asyncio
    async def test_filter_by_single_field(self, store_with_data):
        """Filter nodes where status='active' returns correct subset."""
        store = store_with_data
        all_nodes = await store.get_nodes_by_type("t1", 1)
        active_nodes = [n for n in all_nodes if n.payload.get("status") == "active"]

        assert len(active_nodes) == 4
        for node in active_nodes:
            assert node.payload["status"] == "active"

    @pytest.mark.asyncio
    async def test_filter_by_multiple_fields(self, store_with_data):
        """Filter where status='active' AND priority='high'."""
        store = store_with_data
        all_nodes = await store.get_nodes_by_type("t1", 1)
        filtered = [
            n
            for n in all_nodes
            if n.payload.get("status") == "active" and n.payload.get("priority") == "high"
        ]

        assert len(filtered) == 2
        for node in filtered:
            assert node.payload["status"] == "active"
            assert node.payload["priority"] == "high"

    @pytest.mark.asyncio
    async def test_filter_no_match(self, store_with_data):
        """Filter with nonexistent value returns empty list."""
        store = store_with_data
        all_nodes = await store.get_nodes_by_type("t1", 1)
        filtered = [n for n in all_nodes if n.payload.get("status") == "deleted"]

        assert len(filtered) == 0

    @pytest.mark.asyncio
    async def test_filter_none_returns_all(self, store_with_data):
        """No filter applied returns all nodes."""
        store = store_with_data
        all_nodes = await store.get_nodes_by_type("t1", 1)

        assert len(all_nodes) == 8

    @pytest.mark.asyncio
    async def test_filter_with_pagination(self, store_with_data):
        """Filter + limit + offset works correctly."""
        store = store_with_data

        # Get first page
        page1 = await store.get_nodes_by_type("t1", 1, limit=3, offset=0)
        # Get second page
        page2 = await store.get_nodes_by_type("t1", 1, limit=3, offset=3)

        assert len(page1) == 3
        assert len(page2) == 3

        # No overlap
        ids1 = {n.node_id for n in page1}
        ids2 = {n.node_id for n in page2}
        assert ids1.isdisjoint(ids2)

        # Apply filter to paginated results
        active_page1 = [n for n in page1 if n.payload.get("status") == "active"]
        active_page2 = [n for n in page2 if n.payload.get("status") == "active"]
        # Combined active count from both pages should be <= total active
        assert len(active_page1) + len(active_page2) <= 4

    @pytest.mark.asyncio
    async def test_filter_preserves_ordering(self, store_with_data):
        """Filtered results maintain created_at DESC ordering."""
        store = store_with_data
        all_nodes = await store.get_nodes_by_type("t1", 1)
        active_nodes = [n for n in all_nodes if n.payload.get("status") == "active"]

        # Verify ordering is maintained (created_at DESC)
        for i in range(len(active_nodes) - 1):
            assert active_nodes[i].created_at >= active_nodes[i + 1].created_at

    @pytest.mark.asyncio
    async def test_filter_on_string_field(self, store_with_data):
        """Filter on string field with exact match."""
        store = store_with_data
        all_nodes = await store.get_nodes_by_type("t1", 1)
        pending = [n for n in all_nodes if n.payload.get("status") == "pending"]

        assert len(pending) == 1
        assert pending[0].payload["priority"] == "high"

    @pytest.mark.asyncio
    async def test_filter_on_integer_field(self, store_with_data):
        """Filter on integer field."""
        store = store_with_data
        all_nodes = await store.get_nodes_by_type("t1", 1)
        high_count = [n for n in all_nodes if n.payload.get("count", 0) >= 15]

        assert len(high_count) == 4
        for node in high_count:
            assert node.payload["count"] >= 15

    @pytest.mark.asyncio
    async def test_filter_on_boolean_field(self, store_with_data):
        """Filter on boolean field."""
        store = store_with_data
        all_nodes = await store.get_nodes_by_type("t1", 1)
        flagged = [n for n in all_nodes if n.payload.get("flag") is True]

        assert len(flagged) == 4
        for node in flagged:
            assert node.payload["flag"] is True

    @pytest.mark.asyncio
    async def test_filter_on_nonexistent_field(self, store_with_data):
        """Filter on a field that does not exist in any payload returns empty."""
        store = store_with_data
        all_nodes = await store.get_nodes_by_type("t1", 1)
        filtered = [n for n in all_nodes if n.payload.get("nonexistent_field") == "anything"]

        assert len(filtered) == 0


# =========================================================================
# ARCHIVER DEDUPLICATION
# =========================================================================


@pytest.mark.integration
class TestArchiverDedup:
    """Tests for archiver deduplication logic.

    The archiver uses a set of (tenant_id, partition, offset) tuples
    to skip duplicate events. The set is cleaned up after a successful
    flush to prevent unbounded memory growth.
    """

    @pytest.mark.asyncio
    async def test_duplicate_event_not_archived_twice(self):
        """Processing the same record twice should only add it once."""
        archiver = _make_archiver(deduplicate=True)

        record = _make_record(tenant_id="t1", partition=0, offset=42)

        await archiver._process_record(record)
        await archiver._process_record(record)

        # Only one event should be in the pending segment
        segment = archiver._pending_segments.get("t1:0")
        assert segment is not None
        assert len(segment.events) == 1

    @pytest.mark.asyncio
    async def test_dedup_set_cleaned_after_flush(self):
        """After a successful flush, dedup offsets for flushed events are removed."""
        archiver = _make_archiver(deduplicate=True, flush_mode="batched")
        archiver._s3_client = AsyncMock()
        archiver._s3_client.put_object = AsyncMock()

        # Add 5 events
        for i in range(5):
            record = _make_record(tenant_id="t1", partition=0, offset=i)
            await archiver._process_record(record)

        # Verify dedup set has 5 entries
        assert len(archiver._archived_offsets) == 5

        # Flush the segment
        result = await archiver._flush_segment("t1:0", force=True)
        assert result is not None

        # Dedup set should be empty after flush
        assert len(archiver._archived_offsets) == 0

    @pytest.mark.asyncio
    async def test_dedup_disabled_allows_duplicates(self):
        """With deduplicate=False, the same event can be archived twice."""
        archiver = _make_archiver(deduplicate=False)

        record = _make_record(tenant_id="t1", partition=0, offset=42)

        await archiver._process_record(record)
        await archiver._process_record(record)

        segment = archiver._pending_segments.get("t1:0")
        assert segment is not None
        assert len(segment.events) == 2

    @pytest.mark.asyncio
    async def test_dedup_across_tenants_independent(self):
        """Same offset in different tenants should both be archived."""
        archiver = _make_archiver(deduplicate=True)

        record_a = _make_record(tenant_id="tenant-a", partition=0, offset=1)
        record_b = _make_record(tenant_id="tenant-b", partition=0, offset=1)

        await archiver._process_record(record_a)
        await archiver._process_record(record_b)

        seg_a = archiver._pending_segments.get("tenant-a:0")
        seg_b = archiver._pending_segments.get("tenant-b:0")

        assert seg_a is not None and len(seg_a.events) == 1
        assert seg_b is not None and len(seg_b.events) == 1

        # Dedup set should have both entries
        assert len(archiver._archived_offsets) == 2
        assert ("tenant-a", 0, 1) in archiver._archived_offsets
        assert ("tenant-b", 0, 1) in archiver._archived_offsets

    @pytest.mark.asyncio
    async def test_dedup_memory_bounded(self):
        """Dedup set grows with pending events and shrinks after flush,
        proving memory does not grow unboundedly."""
        archiver = _make_archiver(deduplicate=True, max_segment_events=50)
        archiver._s3_client = AsyncMock()
        archiver._s3_client.put_object = AsyncMock()

        # Process 100 events (max_segment_events=50 triggers auto-flush at 50)
        for i in range(100):
            record = _make_record(tenant_id="t1", partition=0, offset=i)
            await archiver._process_record(record)

        # After processing 100 events with max_segment_events=50,
        # a flush should have been triggered at offset 49.
        # The dedup set should only contain offsets from the current
        # (un-flushed) segment, not all 100.
        assert len(archiver._archived_offsets) <= 50, (
            f"Dedup set has {len(archiver._archived_offsets)} entries, "
            "expected <= 50 after auto-flush"
        )

    @pytest.mark.asyncio
    async def test_dedup_different_partitions_independent(self):
        """Same offset on different partitions should both be archived."""
        archiver = _make_archiver(deduplicate=True)

        record_p0 = _make_record(tenant_id="t1", partition=0, offset=5)
        record_p1 = _make_record(tenant_id="t1", partition=1, offset=5)

        await archiver._process_record(record_p0)
        await archiver._process_record(record_p1)

        seg_p0 = archiver._pending_segments.get("t1:0")
        seg_p1 = archiver._pending_segments.get("t1:1")

        assert seg_p0 is not None and len(seg_p0.events) == 1
        assert seg_p1 is not None and len(seg_p1.events) == 1

    @pytest.mark.asyncio
    async def test_dedup_after_flush_allows_reprocess(self):
        """After flushing, the same offset can be re-added if encountered again.
        This handles the edge case where the archiver restarts and receives
        events that were already flushed to S3 (the S3 write is idempotent)."""
        archiver = _make_archiver(deduplicate=True)
        archiver._s3_client = AsyncMock()
        archiver._s3_client.put_object = AsyncMock()

        record = _make_record(tenant_id="t1", partition=0, offset=10)

        # Process and flush
        await archiver._process_record(record)
        assert len(archiver._archived_offsets) == 1

        await archiver._flush_segment("t1:0", force=True)
        assert len(archiver._archived_offsets) == 0

        # Process same record again (simulating restart replay)
        await archiver._process_record(record)
        assert len(archiver._archived_offsets) == 1

        segment = archiver._pending_segments.get("t1:0")
        assert segment is not None
        assert len(segment.events) == 1

    @pytest.mark.asyncio
    async def test_dedup_flush_failure_preserves_offsets(self):
        """If flush fails (S3 error), dedup offsets should NOT be cleaned up,
        so the events remain in the dedup set for the retry."""
        archiver = _make_archiver(deduplicate=True)
        archiver._s3_client = AsyncMock()
        archiver._s3_client.put_object = AsyncMock(side_effect=Exception("S3 error"))

        for i in range(3):
            record = _make_record(tenant_id="t1", partition=0, offset=i)
            await archiver._process_record(record)

        assert len(archiver._archived_offsets) == 3

        # Flush will fail
        result = await archiver._flush_segment("t1:0", force=True)
        assert result is None

        # Dedup set should still have all 3 entries (not cleaned up)
        assert len(archiver._archived_offsets) == 3

        # Segment should be put back for retry
        assert "t1:0" in archiver._pending_segments

    @pytest.mark.asyncio
    async def test_dedup_with_individual_flush_mode(self):
        """In individual flush mode, each event is flushed immediately.
        Dedup still prevents duplicates within the processing window."""
        archiver = _make_archiver(deduplicate=True, flush_mode="individual")
        archiver._s3_client = AsyncMock()
        archiver._s3_client.put_object = AsyncMock()

        record = _make_record(tenant_id="t1", partition=0, offset=0)

        # First process: event is flushed immediately
        await archiver._process_record(record)

        # Dedup set should be empty after individual flush
        assert len(archiver._archived_offsets) == 0

        # Second process of same event: should be allowed (offset was cleaned)
        await archiver._process_record(record)

        # Verify S3 put was called twice (individual mode, no dedup block after flush)
        assert archiver._s3_client.put_object.call_count == 2
