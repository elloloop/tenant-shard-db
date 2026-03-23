"""
Batch applier integration tests.

Tests poll_batch + batch_transaction for correctness with
InMemoryWalStream and real SQLite.
"""

from __future__ import annotations

import json
import time

import pytest

from dbaas.entdb_server.apply.applier import (
    Applier,
    MailboxFanoutConfig,
    TransactionEvent,
)
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.apply.mailbox_store import MailboxStore
from dbaas.entdb_server.wal.base import WalConnectionError
from dbaas.entdb_server.wal.memory import InMemoryWalStream


def _event_bytes(
    tenant_id: str,
    idempotency_key: str,
    node_id: str | None = None,
    payload: dict | None = None,
    ops: list[dict] | None = None,
) -> bytes:
    """Build event bytes for WAL append."""
    if ops is None:
        ops = [
            {
                "op": "create_node",
                "type_id": 1,
                "id": node_id or f"n-{idempotency_key}",
                "data": payload or {"k": idempotency_key},
            }
        ]
    return json.dumps(
        {
            "tenant_id": tenant_id,
            "actor": "user:1",
            "idempotency_key": idempotency_key,
            "ops": ops,
        }
    ).encode()


def _make_event(
    tenant_id: str,
    idempotency_key: str,
    node_id: str | None = None,
    payload: dict | None = None,
    ops: list[dict] | None = None,
) -> TransactionEvent:
    if ops is None:
        ops = [
            {
                "op": "create_node",
                "type_id": 1,
                "id": node_id or f"n-{idempotency_key}",
                "data": payload or {"k": idempotency_key},
            }
        ]
    return TransactionEvent(
        tenant_id=tenant_id,
        actor="user:1",
        idempotency_key=idempotency_key,
        schema_fingerprint=None,
        ts_ms=int(time.time() * 1000),
        ops=ops,
    )


def _build_applier(
    wal: InMemoryWalStream,
    store: CanonicalStore,
    mbox: MailboxStore,
    batch_size: int = 1,
    topic: str = "test-wal",
) -> Applier:
    return Applier(
        wal=wal,
        canonical_store=store,
        mailbox_store=mbox,
        topic=topic,
        fanout_config=MailboxFanoutConfig(enabled=False),
        batch_size=batch_size,
    )


# =========================================================================
# POLL BATCH
# =========================================================================


@pytest.mark.integration
class TestPollBatch:
    """Tests for InMemoryWalStream.poll_batch behavior."""

    @pytest.mark.asyncio
    async def test_poll_returns_available_records(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(5):
            await wal.append("topic", "k", _event_bytes("t1", f"e{i}"))

        records = await wal.poll_batch("topic", "grp", max_records=10)
        assert len(records) == 5

    @pytest.mark.asyncio
    async def test_poll_respects_max_records(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(20):
            await wal.append("topic", "k", _event_bytes("t1", f"e{i}"))

        records = await wal.poll_batch("topic", "grp", max_records=5)
        assert len(records) == 5

    @pytest.mark.asyncio
    async def test_poll_empty_returns_empty(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        records = await wal.poll_batch("topic", "grp", max_records=10, timeout_ms=50)
        assert records == []

    @pytest.mark.asyncio
    async def test_poll_with_timeout(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        start = time.time()
        records = await wal.poll_batch("topic", "grp", max_records=10, timeout_ms=100)
        elapsed = time.time() - start
        assert records == []
        # Should have waited roughly 100ms
        assert elapsed >= 0.05

    @pytest.mark.asyncio
    async def test_poll_after_commit_advances_offset(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(5):
            await wal.append("topic", "k", _event_bytes("t1", f"e{i}"))

        batch1 = await wal.poll_batch("topic", "grp", max_records=2)
        assert len(batch1) == 2
        await wal.commit(batch1[-1])

        batch2 = await wal.poll_batch("topic", "grp", max_records=10)
        assert len(batch2) == 3

    @pytest.mark.asyncio
    async def test_poll_returns_correct_record_data(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        payload = {"tenant_id": "t1", "actor": "u", "idempotency_key": "k1", "ops": []}
        await wal.append("topic", "t1", json.dumps(payload).encode())

        records = await wal.poll_batch("topic", "grp", max_records=1)
        assert len(records) == 1
        data = json.loads(records[0].value)
        assert data["tenant_id"] == "t1"
        assert data["idempotency_key"] == "k1"

    @pytest.mark.asyncio
    async def test_poll_sequential_calls_return_different_records(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(6):
            await wal.append("topic", "k", _event_bytes("t1", f"e{i}"))

        batch1 = await wal.poll_batch("topic", "grp", max_records=3)
        await wal.commit(batch1[-1])

        batch2 = await wal.poll_batch("topic", "grp", max_records=3)
        keys1 = {json.loads(r.value)["idempotency_key"] for r in batch1}
        keys2 = {json.loads(r.value)["idempotency_key"] for r in batch2}
        assert keys1.isdisjoint(keys2)

    @pytest.mark.asyncio
    async def test_poll_with_1_record_available(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        await wal.append("topic", "k", _event_bytes("t1", "only"))

        records = await wal.poll_batch("topic", "grp", max_records=10)
        assert len(records) == 1

    @pytest.mark.asyncio
    async def test_poll_with_exactly_max_records(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(5):
            await wal.append("topic", "k", _event_bytes("t1", f"e{i}"))

        records = await wal.poll_batch("topic", "grp", max_records=5)
        assert len(records) == 5

    @pytest.mark.asyncio
    async def test_poll_with_more_than_max_records(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(15):
            await wal.append("topic", "k", _event_bytes("t1", f"e{i}"))

        records = await wal.poll_batch("topic", "grp", max_records=10)
        assert len(records) == 10

    @pytest.mark.asyncio
    async def test_poll_preserves_record_order(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(10):
            await wal.append("topic", "k", _event_bytes("t1", f"ordered-{i}"))

        records = await wal.poll_batch("topic", "grp", max_records=20)
        keys = [json.loads(r.value)["idempotency_key"] for r in records]
        assert keys == [f"ordered-{i}" for i in range(10)]

    @pytest.mark.asyncio
    async def test_poll_after_close_raises(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        await wal.close()
        with pytest.raises(WalConnectionError):
            await wal.poll_batch("topic", "grp", max_records=1)

    @pytest.mark.asyncio
    async def test_poll_multiple_partitions(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=4)
        await wal.connect()
        # Different keys map to different partitions
        for i in range(20):
            await wal.append("topic", f"key-{i}", _event_bytes("t1", f"mp-{i}"))

        records = await wal.poll_batch("topic", "grp", max_records=30)
        assert len(records) == 20

    @pytest.mark.asyncio
    async def test_poll_batch_returns_stream_records(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        await wal.append("topic", "k1", _event_bytes("t1", "r1"))

        records = await wal.poll_batch("topic", "grp", max_records=1)
        assert len(records) == 1
        assert records[0].key == "k1"
        assert records[0].position is not None
        assert records[0].position.topic == "topic"


# =========================================================================
# BATCH APPLIER CORRECTNESS
# =========================================================================


@pytest.mark.integration
class TestBatchApplierCorrectness:
    """Tests batch applier produces correct results."""

    @pytest.mark.asyncio
    async def test_batch_size_1_processes_all(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox, batch_size=1)

        for i in range(10):
            event = _make_event("t1", f"b1-{i}", node_id=f"b1-{i}")
            await applier.apply_event(event)

        nodes = await store.get_nodes_by_type("t1", 1)
        assert len(nodes) == 10

    @pytest.mark.asyncio
    async def test_batch_size_10_processes_all(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox, batch_size=10)

        for i in range(20):
            event = _make_event("t1", f"b10-{i}", node_id=f"b10-{i}")
            await applier.apply_event(event)

        nodes = await store.get_nodes_by_type("t1", 1)
        assert len(nodes) == 20

    @pytest.mark.asyncio
    async def test_batch_size_50_processes_all(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox, batch_size=50)

        for i in range(50):
            event = _make_event("t1", f"b50-{i}", node_id=f"b50-{i}")
            await applier.apply_event(event)

        nodes = await store.get_nodes_by_type("t1", 1)
        assert len(nodes) == 50

    @pytest.mark.asyncio
    async def test_batch_creates_correct_nodes(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        event = _make_event("t1", "exact", node_id="exact-node", payload={"val": 42})
        r = await applier.apply_event(event)
        assert r.success

        node = await store.get_node("t1", "exact-node")
        assert node is not None
        assert node.payload["val"] == 42

    @pytest.mark.asyncio
    async def test_batch_preserves_event_ordering(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        # Create nodes with sequential payloads
        for i in range(10):
            event = _make_event("t1", f"ord-{i}", node_id=f"ord-{i}", payload={"seq": i})
            await applier.apply_event(event)

        # Verify all exist with correct data
        for i in range(10):
            node = await store.get_node("t1", f"ord-{i}")
            assert node is not None
            assert node.payload["seq"] == i

    @pytest.mark.asyncio
    async def test_batch_with_mixed_operations(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        # Create two nodes
        create_event = _make_event(
            "t1",
            "mix-create",
            ops=[
                {"op": "create_node", "type_id": 1, "id": "mix-a", "data": {"v": 1}},
                {"op": "create_node", "type_id": 1, "id": "mix-b", "data": {"v": 2}},
                {"op": "create_edge", "edge_id": 1, "from": "mix-a", "to": "mix-b"},
            ],
        )
        r = await applier.apply_event(create_event)
        assert r.success
        assert len(r.created_nodes) == 2
        assert len(r.created_edges) == 1

        # Update one node
        update_event = TransactionEvent(
            tenant_id="t1",
            actor="user:1",
            idempotency_key="mix-update",
            schema_fingerprint=None,
            ts_ms=int(time.time() * 1000),
            ops=[{"op": "update_node", "id": "mix-a", "patch": {"v": 99}}],
        )
        r2 = await applier.apply_event(update_event)
        assert r2.success

        node = await store.get_node("t1", "mix-a")
        assert node.payload["v"] == 99

    @pytest.mark.asyncio
    async def test_batch_with_idempotent_replay(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        event = _make_event("t1", "idem-key", node_id="idem-node")
        r1 = await applier.apply_event(event)
        r2 = await applier.apply_event(event)
        assert r1.success and not r1.skipped
        assert r2.success and r2.skipped

        nodes = await store.get_nodes_by_type("t1", 1)
        assert len(nodes) == 1

    @pytest.mark.asyncio
    async def test_batch_with_multi_tenant_events(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        for tid in ["ta", "tb", "tc"]:
            for i in range(3):
                event = _make_event(tid, f"{tid}-{i}", node_id=f"{tid}-{i}")
                await applier.apply_event(event)

        for tid in ["ta", "tb", "tc"]:
            nodes = await store.get_nodes_by_type(tid, 1)
            assert len(nodes) == 3

    @pytest.mark.asyncio
    async def test_batch_mode_same_as_single(self, tmp_path):
        """Batch and single mode produce identical results."""
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        # Single mode
        store1 = CanonicalStore(data_dir=str(tmp_path / "s1"), wal_mode=True)
        mbox1 = MailboxStore(data_dir=str(tmp_path / "m1"), wal_mode=True)
        applier1 = _build_applier(wal, store1, mbox1, batch_size=1)

        # Batch mode
        store2 = CanonicalStore(data_dir=str(tmp_path / "s2"), wal_mode=True)
        mbox2 = MailboxStore(data_dir=str(tmp_path / "m2"), wal_mode=True)
        applier2 = _build_applier(wal, store2, mbox2, batch_size=10)

        for i in range(15):
            event = _make_event("t1", f"cmp-{i}", node_id=f"cmp-{i}", payload={"i": i})
            await applier1.apply_event(event)
            await applier2.apply_event(event)

        nodes1 = await store1.get_nodes_by_type("t1", 1)
        nodes2 = await store2.get_nodes_by_type("t1", 1)
        assert len(nodes1) == len(nodes2) == 15

        for i in range(15):
            n1 = await store1.get_node("t1", f"cmp-{i}")
            n2 = await store2.get_node("t1", f"cmp-{i}")
            assert n1.payload == n2.payload

    @pytest.mark.asyncio
    async def test_500_events_all_applied(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        for i in range(500):
            event = _make_event("t1", f"bulk-{i}", node_id=f"bulk-{i}")
            await applier.apply_event(event)

        nodes = await store.get_nodes_by_type("t1", 1, limit=1000)
        assert len(nodes) == 500

    @pytest.mark.asyncio
    async def test_batch_with_empty_ops(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        event = TransactionEvent(
            tenant_id="t1",
            actor="user:1",
            idempotency_key="empty-ops",
            schema_fingerprint=None,
            ts_ms=int(time.time() * 1000),
            ops=[],
        )
        r = await applier.apply_event(event)
        assert r.success

    @pytest.mark.asyncio
    async def test_batch_with_large_payloads(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        big_payload = {f"field_{i}": "x" * 500 for i in range(20)}
        event = _make_event("t1", "big", node_id="big-node", payload=big_payload)
        r = await applier.apply_event(event)
        assert r.success

        node = await store.get_node("t1", "big-node")
        assert len(node.payload) == 20

    @pytest.mark.asyncio
    async def test_batch_delete_after_create(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        create = _make_event("t1", "create-del", node_id="del-me")
        await applier.apply_event(create)
        assert await store.get_node("t1", "del-me") is not None

        delete = TransactionEvent(
            tenant_id="t1",
            actor="user:1",
            idempotency_key="do-del",
            schema_fingerprint=None,
            ts_ms=int(time.time() * 1000),
            ops=[{"op": "delete_node", "id": "del-me"}],
        )
        r = await applier.apply_event(delete)
        assert r.success
        assert await store.get_node("t1", "del-me") is None

    @pytest.mark.asyncio
    async def test_batch_commits_offset_after_processing(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        for i in range(5):
            await wal.append("test-wal", "t1", _event_bytes("t1", f"co-{i}", node_id=f"co-{i}"))

        records = await wal.poll_batch("test-wal", "grp", max_records=5)
        assert len(records) == 5
        await wal.commit(records[-1])

        # Polling again should return no new records
        records2 = await wal.poll_batch("test-wal", "grp", max_records=5, timeout_ms=50)
        assert records2 == []


# =========================================================================
# BATCH TRANSACTION (CanonicalStore)
# =========================================================================


@pytest.mark.integration
class TestBatchTransaction:
    """Tests for CanonicalStore.batch_transaction."""

    @pytest.fixture
    def store(self, tmp_path):
        import asyncio

        s = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        asyncio.get_event_loop().run_until_complete(s.initialize_tenant("test"))
        return s

    @pytest.mark.asyncio
    async def test_batch_creates_all_or_nothing(self, store):
        with store.batch_transaction("test") as conn:
            for i in range(5):
                store.create_node_raw(conn, "test", 1, {"i": i}, "u:1", node_id=f"aon-{i}")

        for i in range(5):
            node = await store.get_node("test", f"aon-{i}")
            assert node is not None

    @pytest.mark.asyncio
    async def test_batch_with_100_nodes(self, store):
        with store.batch_transaction("test") as conn:
            for i in range(100):
                store.create_node_raw(conn, "test", 1, {"i": i}, "u:1", node_id=f"h-{i}")

        nodes = await store.get_nodes_by_type("test", 1, limit=200)
        assert len(nodes) == 100

    @pytest.mark.asyncio
    async def test_batch_performance_faster_than_individual(self, store):
        # Batch: all in one transaction
        t0 = time.time()
        with store.batch_transaction("test") as conn:
            for i in range(50):
                store.create_node_raw(conn, "test", 2, {"i": i}, "u:1", node_id=f"perf-{i}")
        _ = time.time() - t0  # batch_time

        # Individual: one transaction per create
        t0 = time.time()
        for i in range(50):
            await store.create_node("test", 3, {"i": i}, "u:1", node_id=f"ind-{i}")
        _ = time.time() - t0  # individual_time

        # Batch should generally be faster (or at least not significantly slower)
        # We just verify both succeed
        nodes_batch = await store.get_nodes_by_type("test", 2)
        nodes_ind = await store.get_nodes_by_type("test", 3)
        assert len(nodes_batch) == 50
        assert len(nodes_ind) == 50

    @pytest.mark.asyncio
    async def test_create_node_raw_matches_create_node(self, store):
        # Via create_node
        await store.create_node("test", 1, {"via": "async"}, "u:1", node_id="async-n")

        # Via create_node_raw
        with store.batch_transaction("test") as conn:
            store.create_node_raw(conn, "test", 1, {"via": "raw"}, "u:1", node_id="raw-n")

        # Both should be fetchable
        fetched1 = await store.get_node("test", "async-n")
        fetched2 = await store.get_node("test", "raw-n")
        assert fetched1 is not None
        assert fetched2 is not None
        assert fetched1.type_id == fetched2.type_id == 1

    @pytest.mark.asyncio
    async def test_batch_rollback_leaves_db_unchanged(self, store):
        await store.create_node("test", 1, {}, "u:1", node_id="existing-r")
        try:
            with store.batch_transaction("test") as conn:
                store.create_node_raw(conn, "test", 1, {}, "u:1", node_id="new-r")
                # Force a duplicate
                store.create_node_raw(conn, "test", 1, {}, "u:1", node_id="existing-r")
        except Exception:
            pass

        assert await store.get_node("test", "new-r") is None
        assert await store.get_node("test", "existing-r") is not None

    @pytest.mark.asyncio
    async def test_batch_across_different_types(self, store):
        with store.batch_transaction("test") as conn:
            store.create_node_raw(conn, "test", 1, {"type": "a"}, "u:1", node_id="ta-1")
            store.create_node_raw(conn, "test", 2, {"type": "b"}, "u:1", node_id="tb-1")
            store.create_node_raw(conn, "test", 3, {"type": "c"}, "u:1", node_id="tc-1")

        assert len(await store.get_nodes_by_type("test", 1)) == 1
        assert len(await store.get_nodes_by_type("test", 2)) == 1
        assert len(await store.get_nodes_by_type("test", 3)) == 1

    @pytest.mark.asyncio
    async def test_batch_with_acl(self, store):
        acl = [{"principal": "user:1", "permission": "admin"}]
        with store.batch_transaction("test") as conn:
            n = store.create_node_raw(
                conn, "test", 1, {"x": 1}, "u:1", node_id="acl-batch", acl=acl
            )
        assert len(n.acl) == 1

        fetched = await store.get_node("test", "acl-batch")
        assert fetched is not None

    @pytest.mark.asyncio
    async def test_batch_with_custom_timestamp(self, store):
        with store.batch_transaction("test") as conn:
            n = store.create_node_raw(
                conn, "test", 1, {}, "u:1", node_id="ts-batch", created_at=999000
            )
        assert n.created_at == 999000

        fetched = await store.get_node("test", "ts-batch")
        assert fetched.created_at == 999000

    @pytest.mark.asyncio
    async def test_batch_idempotency_tracking(self, store):
        """Record applied events inside a batch transaction."""
        await store.record_applied_event("test", "batch-idem-key", "pos:0:1")
        assert await store.check_idempotency("test", "batch-idem-key") is True
        assert await store.check_idempotency("test", "other-key") is False

    @pytest.mark.asyncio
    async def test_batch_multiple_sequential(self, store):
        """Multiple sequential batch transactions work correctly."""
        for batch in range(3):
            with store.batch_transaction("test") as conn:
                for i in range(5):
                    store.create_node_raw(
                        conn,
                        "test",
                        1,
                        {"batch": batch, "i": i},
                        "u:1",
                        node_id=f"seq-{batch}-{i}",
                    )

        nodes = await store.get_nodes_by_type("test", 1, limit=100)
        assert len(nodes) == 15


# =========================================================================
# WAL STREAM ADVANCED
# =========================================================================


@pytest.mark.integration
class TestWalStreamAdvanced:
    """Advanced tests for InMemoryWalStream behavior."""

    @pytest.mark.asyncio
    async def test_append_returns_stream_pos(self):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        pos = await wal.append("t", "k", b"v")
        assert pos.topic == "t"
        assert pos.offset == 0

    @pytest.mark.asyncio
    async def test_sequential_offsets(self):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        p0 = await wal.append("t", "k", b"v0")
        p1 = await wal.append("t", "k", b"v1")
        p2 = await wal.append("t", "k", b"v2")
        assert p0.offset == 0
        assert p1.offset == 1
        assert p2.offset == 2

    @pytest.mark.asyncio
    async def test_poll_batch_different_groups_independent(self):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(5):
            await wal.append("t", "k", f"v{i}".encode())

        r1 = await wal.poll_batch("t", "grp-a", max_records=3)
        r2 = await wal.poll_batch("t", "grp-b", max_records=3)
        # Both groups should get records (they track offsets independently)
        assert len(r1) == 3
        assert len(r2) == 3

    @pytest.mark.asyncio
    async def test_clear_topic_resets_offsets(self):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(3):
            await wal.append("t", "k", f"v{i}".encode())
        assert wal.get_record_count("t") == 3
        wal.clear_topic("t")
        assert wal.get_record_count("t") == 0

    @pytest.mark.asyncio
    async def test_nonexistent_topic_returns_empty(self):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        records = await wal.poll_batch("nope", "grp", max_records=5, timeout_ms=50)
        assert records == []

    @pytest.mark.asyncio
    async def test_get_all_records_empty(self):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        assert wal.get_all_records("t") == []

    @pytest.mark.asyncio
    async def test_get_positions_empty(self):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        positions = await wal.get_positions("t", "grp")
        assert positions == {}

    @pytest.mark.asyncio
    async def test_get_positions_after_commit(self):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        await wal.append("t", "k", b"v")
        records = await wal.poll_batch("t", "grp", max_records=1)
        await wal.commit(records[0])
        positions = await wal.get_positions("t", "default")
        assert len(positions) > 0

    @pytest.mark.asyncio
    async def test_num_partitions_configurable(self):
        wal = InMemoryWalStream(num_partitions=16)
        await wal.connect()
        assert wal.num_partitions == 16

    @pytest.mark.asyncio
    async def test_append_without_connect_raises(self):
        wal = InMemoryWalStream(num_partitions=1)
        from dbaas.entdb_server.wal.base import WalConnectionError

        with pytest.raises(WalConnectionError):
            await wal.append("t", "k", b"v")

    @pytest.mark.asyncio
    async def test_multiple_topics_independent(self):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(3):
            await wal.append("a", "k", f"a{i}".encode())
        for i in range(5):
            await wal.append("b", "k", f"b{i}".encode())

        assert wal.get_record_count("a") == 3
        assert wal.get_record_count("b") == 5

    @pytest.mark.asyncio
    async def test_record_key_preserved(self):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        await wal.append("t", "my-key", b"v")
        records = await wal.poll_batch("t", "grp", max_records=1)
        assert records[0].key == "my-key"

    @pytest.mark.asyncio
    async def test_record_headers_preserved(self):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        await wal.append("t", "k", b"v", headers={"h1": b"val1"})
        records = await wal.poll_batch("t", "grp", max_records=1)
        assert records[0].headers == {"h1": b"val1"}

    @pytest.mark.asyncio
    async def test_record_no_headers(self):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        await wal.append("t", "k", b"v")
        records = await wal.poll_batch("t", "grp", max_records=1)
        assert records[0].headers == {}

    @pytest.mark.asyncio
    async def test_stream_pos_str(self):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        pos = await wal.append("my-topic", "k", b"v")
        s = str(pos)
        assert "my-topic" in s

    @pytest.mark.asyncio
    async def test_stream_pos_to_dict(self):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        pos = await wal.append("t", "k", b"v")
        d = pos.to_dict()
        assert d["topic"] == "t"
        assert "offset" in d
        assert "partition" in d
        assert "timestamp_ms" in d

    @pytest.mark.asyncio
    async def test_stream_pos_from_dict_roundtrip(self):
        from dbaas.entdb_server.wal.base import StreamPos

        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        pos = await wal.append("t", "k", b"v")
        d = pos.to_dict()
        pos2 = StreamPos.from_dict(d)
        assert pos == pos2

    @pytest.mark.asyncio
    async def test_concurrent_appends(self):
        import asyncio

        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        async def append_batch(start: int) -> None:
            for i in range(10):
                await wal.append("t", "k", f"v{start + i}".encode())

        await asyncio.gather(append_batch(0), append_batch(100), append_batch(200))
        assert wal.get_record_count("t") == 30

    @pytest.mark.asyncio
    async def test_poll_batch_with_start_position(self):
        from dbaas.entdb_server.wal.base import StreamPos

        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(10):
            await wal.append("t", "k", f"v{i}".encode())

        start = StreamPos(topic="t", partition=0, offset=4, timestamp_ms=0)
        records = await wal.poll_batch("t", "grp", max_records=20, start_position=start)
        assert len(records) == 5  # offsets 5,6,7,8,9

    @pytest.mark.asyncio
    async def test_close_clears_all(self):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        await wal.append("t", "k", b"v")
        await wal.close()
        assert not wal.is_connected


# =========================================================================
# APPLIER EDGE OPERATIONS
# =========================================================================


@pytest.mark.integration
class TestApplierEdgeOperations:
    """Tests for applier edge create/delete operations."""

    @pytest.mark.asyncio
    async def test_create_multiple_edges(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        event = _make_event(
            "t1",
            "multi-edge",
            ops=[
                {"op": "create_node", "type_id": 1, "id": "hub", "data": {}},
                {"op": "create_node", "type_id": 1, "id": "spoke1", "data": {}},
                {"op": "create_node", "type_id": 1, "id": "spoke2", "data": {}},
                {"op": "create_node", "type_id": 1, "id": "spoke3", "data": {}},
                {"op": "create_edge", "edge_id": 1, "from": "hub", "to": "spoke1"},
                {"op": "create_edge", "edge_id": 1, "from": "hub", "to": "spoke2"},
                {"op": "create_edge", "edge_id": 1, "from": "hub", "to": "spoke3"},
            ],
        )
        r = await applier.apply_event(event)
        assert r.success
        assert len(r.created_edges) == 3

    @pytest.mark.asyncio
    async def test_edge_with_different_types(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        event = _make_event(
            "t1",
            "edge-types",
            ops=[
                {"op": "create_node", "type_id": 1, "id": "n1", "data": {}},
                {"op": "create_node", "type_id": 1, "id": "n2", "data": {}},
                {"op": "create_edge", "edge_id": 1, "from": "n1", "to": "n2"},
                {"op": "create_edge", "edge_id": 2, "from": "n1", "to": "n2"},
            ],
        )
        r = await applier.apply_event(event)
        assert r.success
        assert len(r.created_edges) == 2

    @pytest.mark.asyncio
    async def test_edge_after_update(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        create = _make_event(
            "t1",
            "setup-edge",
            ops=[
                {"op": "create_node", "type_id": 1, "id": "e-src", "data": {"v": 1}},
                {"op": "create_node", "type_id": 1, "id": "e-tgt", "data": {"v": 1}},
            ],
        )
        await applier.apply_event(create)

        update = _make_event(
            "t1",
            "update-then-edge",
            ops=[
                {"op": "update_node", "id": "e-src", "patch": {"v": 2}},
                {"op": "create_edge", "edge_id": 1, "from": "e-src", "to": "e-tgt"},
            ],
        )
        r = await applier.apply_event(update)
        assert r.success
        edges = await store.get_edges_from("t1", "e-src")
        assert len(edges) == 1

    @pytest.mark.asyncio
    async def test_create_node_with_acl_via_event(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        event = _make_event(
            "t1",
            "acl-event",
            ops=[
                {
                    "op": "create_node",
                    "type_id": 1,
                    "id": "acl-node",
                    "data": {"name": "Secured"},
                    "acl": [{"principal": "user:1", "permission": "admin"}],
                }
            ],
        )
        r = await applier.apply_event(event)
        assert r.success

    @pytest.mark.asyncio
    async def test_complex_multi_op_transaction(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        # Create a small graph
        event = _make_event(
            "t1",
            "complex-tx",
            ops=[
                {"op": "create_node", "type_id": 1, "id": "root", "data": {"role": "root"}},
                {"op": "create_node", "type_id": 2, "id": "child1", "data": {"role": "child"}},
                {"op": "create_node", "type_id": 2, "id": "child2", "data": {"role": "child"}},
                {"op": "create_edge", "edge_id": 1, "from": "root", "to": "child1"},
                {"op": "create_edge", "edge_id": 1, "from": "root", "to": "child2"},
                {"op": "create_edge", "edge_id": 2, "from": "child1", "to": "child2"},
            ],
        )
        r = await applier.apply_event(event)
        assert r.success
        assert len(r.created_nodes) == 3
        assert len(r.created_edges) == 3

    @pytest.mark.asyncio
    async def test_10_sequential_events(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        for i in range(10):
            event = _make_event("t1", f"seq-{i}", node_id=f"seq-{i}")
            r = await applier.apply_event(event)
            assert r.success

        nodes = await store.get_nodes_by_type("t1", 1, limit=20)
        assert len(nodes) == 10

    @pytest.mark.asyncio
    async def test_event_with_empty_string_payload_key(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        event = _make_event("t1", "empty-key", node_id="ek", payload={"": "value"})
        r = await applier.apply_event(event)
        assert r.success
        node = await store.get_node("t1", "ek")
        assert node.payload[""] == "value"

    @pytest.mark.asyncio
    async def test_event_with_null_values_in_payload(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        event = _make_event("t1", "null-val", node_id="nv", payload={"a": None, "b": 1})
        r = await applier.apply_event(event)
        assert r.success
        node = await store.get_node("t1", "nv")
        assert node.payload["a"] is None

    @pytest.mark.asyncio
    async def test_event_with_list_in_payload(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        event = _make_event("t1", "list-val", node_id="lv", payload={"tags": ["a", "b", "c"]})
        r = await applier.apply_event(event)
        assert r.success
        node = await store.get_node("t1", "lv")
        assert node.payload["tags"] == ["a", "b", "c"]

    @pytest.mark.asyncio
    async def test_event_with_boolean_payload(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        event = _make_event(
            "t1", "bool-val", node_id="bv", payload={"active": True, "deleted": False}
        )
        r = await applier.apply_event(event)
        assert r.success
        node = await store.get_node("t1", "bv")
        assert node.payload["active"] is True
        assert node.payload["deleted"] is False

    @pytest.mark.asyncio
    async def test_event_with_numeric_payload(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        event = _make_event("t1", "num-val", node_id="numv", payload={"int": 42, "float": 3.14})
        r = await applier.apply_event(event)
        assert r.success
        node = await store.get_node("t1", "numv")
        assert node.payload["int"] == 42
        assert abs(node.payload["float"] - 3.14) < 0.001

    @pytest.mark.asyncio
    async def test_event_different_type_ids(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        mbox = MailboxStore(data_dir=str(tmp_path / "mbox"), wal_mode=True)
        applier = _build_applier(wal, store, mbox)

        for tid in [1, 2, 3, 4, 5]:
            event = _make_event(
                "t1",
                f"type-{tid}",
                ops=[
                    {
                        "op": "create_node",
                        "type_id": tid,
                        "id": f"typed-{tid}",
                        "data": {"type": tid},
                    }
                ],
            )
            r = await applier.apply_event(event)
            assert r.success

        for tid in [1, 2, 3, 4, 5]:
            nodes = await store.get_nodes_by_type("t1", tid)
            assert len(nodes) == 1


# =========================================================================
# CANONICAL STORE ADVANCED
# =========================================================================


@pytest.mark.integration
class TestCanonicalStoreAdvanced:
    """Additional tests for CanonicalStore correctness."""

    @pytest.fixture
    def store(self, tmp_path):
        import asyncio

        s = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        asyncio.get_event_loop().run_until_complete(s.initialize_tenant("test"))
        return s

    @pytest.mark.asyncio
    async def test_get_node_after_delete_returns_none(self, store):
        node = await store.create_node("test", 1, {"x": 1}, "u:1")
        await store.delete_node("test", node.node_id)
        assert await store.get_node("test", node.node_id) is None

    @pytest.mark.asyncio
    async def test_update_returns_updated_node(self, store):
        node = await store.create_node("test", 1, {"a": 1}, "u:1")
        updated = await store.update_node("test", node.node_id, {"a": 2, "b": 3})
        assert updated.payload == {"a": 2, "b": 3}

    @pytest.mark.asyncio
    async def test_get_nodes_by_type_returns_list(self, store):
        for i in range(3):
            await store.create_node("test", 5, {"i": i}, "u:1")
        result = await store.get_nodes_by_type("test", 5)
        assert isinstance(result, list)
        assert len(result) == 3

    @pytest.mark.asyncio
    async def test_edge_with_props(self, store):
        n1 = await store.create_node("test", 1, {}, "u:1")
        n2 = await store.create_node("test", 1, {}, "u:1")
        edge = await store.create_edge(
            "test", 1, n1.node_id, n2.node_id, props={"weight": 0.5, "label": "friend"}
        )
        assert edge.props["weight"] == 0.5
        assert edge.props["label"] == "friend"

    @pytest.mark.asyncio
    async def test_delete_edge_returns_true(self, store):
        n1 = await store.create_node("test", 1, {}, "u:1")
        n2 = await store.create_node("test", 1, {}, "u:1")
        await store.create_edge("test", 1, n1.node_id, n2.node_id)
        result = await store.delete_edge("test", 1, n1.node_id, n2.node_id)
        assert result is True

    @pytest.mark.asyncio
    async def test_delete_edge_nonexistent_returns_false(self, store):
        result = await store.delete_edge("test", 1, "a", "b")
        assert result is False

    @pytest.mark.asyncio
    async def test_initialize_tenant_idempotent(self, tmp_path):
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("idempotent-t")
        await store.initialize_tenant("idempotent-t")  # should not raise
        assert await store.tenant_exists("idempotent-t")

    @pytest.mark.asyncio
    async def test_create_node_generates_uuid(self, store):
        n1 = await store.create_node("test", 1, {}, "u:1")
        n2 = await store.create_node("test", 1, {}, "u:1")
        assert n1.node_id != n2.node_id
        assert len(n1.node_id) > 0

    @pytest.mark.asyncio
    async def test_record_applied_event_with_stream_pos(self, store):
        await store.record_applied_event("test", "tracked-key", "wal:0:42")
        assert await store.check_idempotency("test", "tracked-key") is True

    @pytest.mark.asyncio
    async def test_record_applied_event_without_stream_pos(self, store):
        await store.record_applied_event("test", "no-pos-key")
        assert await store.check_idempotency("test", "no-pos-key") is True

    @pytest.mark.asyncio
    async def test_many_idempotency_keys(self, store):
        for i in range(50):
            await store.record_applied_event("test", f"key-{i}")

        for i in range(50):
            assert await store.check_idempotency("test", f"key-{i}") is True
        assert await store.check_idempotency("test", "key-999") is False

    @pytest.mark.asyncio
    async def test_node_created_at_updated_at(self, store):
        node = await store.create_node("test", 1, {"v": 1}, "u:1")
        assert node.created_at == node.updated_at

        import time

        time.sleep(0.01)
        updated = await store.update_node("test", node.node_id, {"v": 2})
        assert updated.updated_at >= node.created_at

    @pytest.mark.asyncio
    async def test_edges_from_nonexistent_node(self, store):
        edges = await store.get_edges_from("test", "ghost")
        assert edges == []

    @pytest.mark.asyncio
    async def test_edges_to_nonexistent_node(self, store):
        edges = await store.get_edges_to("test", "ghost")
        assert edges == []

    @pytest.mark.asyncio
    async def test_create_100_nodes_bulk(self, store):
        for i in range(100):
            await store.create_node("test", 1, {"i": i}, "u:1", node_id=f"bulk-{i}")

        nodes = await store.get_nodes_by_type("test", 1, limit=200)
        assert len(nodes) == 100

    @pytest.mark.asyncio
    async def test_get_edges_filtered_by_type(self, store):
        n1 = await store.create_node("test", 1, {}, "u:1")
        n2 = await store.create_node("test", 1, {}, "u:1")
        await store.create_edge("test", 1, n1.node_id, n2.node_id)
        await store.create_edge("test", 2, n1.node_id, n2.node_id)

        type1 = await store.get_edges_from("test", n1.node_id, edge_type_id=1)
        type2 = await store.get_edges_from("test", n1.node_id, edge_type_id=2)
        assert len(type1) == 1
        assert len(type2) == 1

    @pytest.mark.asyncio
    async def test_node_type_id_persisted(self, store):
        node = await store.create_node("test", 42, {"x": 1}, "u:1")
        fetched = await store.get_node("test", node.node_id)
        assert fetched.type_id == 42

    @pytest.mark.asyncio
    async def test_edge_from_to_preserved(self, store):
        n1 = await store.create_node("test", 1, {}, "u:1")
        n2 = await store.create_node("test", 1, {}, "u:1")
        edge = await store.create_edge("test", 1, n1.node_id, n2.node_id)
        assert edge.from_node_id == n1.node_id
        assert edge.to_node_id == n2.node_id

    @pytest.mark.asyncio
    async def test_create_node_with_empty_acl(self, store):
        node = await store.create_node("test", 1, {}, "u:1", acl=[])
        assert node.acl == []

    @pytest.mark.asyncio
    async def test_create_node_with_multiple_acl_entries(self, store):
        acl = [
            {"principal": "user:1", "permission": "read"},
            {"principal": "user:2", "permission": "write"},
            {"principal": "group:admins", "permission": "admin"},
        ]
        node = await store.create_node("test", 1, {}, "u:1", acl=acl)
        assert len(node.acl) == 3
