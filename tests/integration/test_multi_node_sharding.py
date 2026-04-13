"""
Multi-node sharding integration tests.

Tests applier tenant filtering and multi-node isolation using
real SQLite and InMemoryWalStream.
"""

from __future__ import annotations

import asyncio
import json
import time

import pytest

from dbaas.entdb_server.apply.applier import (
    Applier,
    MailboxFanoutConfig,
    TransactionEvent,
)
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.sharding import ShardingConfig
from dbaas.entdb_server.wal.memory import InMemoryWalStream


def _make_event(
    tenant_id: str,
    idempotency_key: str,
    node_id: str | None = None,
    payload: dict | None = None,
) -> TransactionEvent:
    """Helper to create a TransactionEvent."""
    ops = [
        {
            "op": "create_node",
            "type_id": 1,
            "id": node_id or f"n-{idempotency_key}",
            "data": payload or {"key": idempotency_key},
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


def _event_bytes(tenant_id: str, idempotency_key: str, node_id: str | None = None) -> bytes:
    """Helper to build event bytes for WAL append."""
    return json.dumps(
        {
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
    ).encode()


def _build_applier(
    wal: InMemoryWalStream,
    store: CanonicalStore,
    assigned_tenants: frozenset[str] | None = None,
    topic: str = "test-wal",
    batch_size: int = 1,
    group_id: str = "test-group",
) -> Applier:
    return Applier(
        wal=wal,
        canonical_store=store,
        topic=topic,
        group_id=group_id,
        fanout_config=MailboxFanoutConfig(enabled=False),
        assigned_tenants=assigned_tenants,
        batch_size=batch_size,
    )


# =========================================================================
# APPLIER TENANT FILTERING
# =========================================================================


@pytest.mark.integration
class TestApplierTenantFiltering:
    """Tests that applier correctly filters events by assigned tenant set."""

    @pytest.mark.asyncio
    async def test_assigned_tenant_is_processed(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"t1"}))

        event = _make_event("t1", "e1")
        result = await applier.apply_event(event)
        assert result.success
        assert not result.skipped
        assert "n-e1" in result.created_nodes

    @pytest.mark.asyncio
    async def test_unassigned_tenant_is_skipped_via_process_record(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"t1"}))

        await wal.append("test-wal", "other", _event_bytes("other", "e2"))
        records = await wal.poll_batch("test-wal", "test-group", max_records=1)
        assert len(records) == 1
        result = await applier._process_record(records[0])
        assert result.skipped

    @pytest.mark.asyncio
    async def test_skipped_events_advance_offset(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"t1"}))

        # Append two events: one skipped, one processed
        await wal.append("test-wal", "other", _event_bytes("other", "skip-1"))
        await wal.append("test-wal", "t1", _event_bytes("t1", "process-1"))

        records = await wal.poll_batch("test-wal", "test-group", max_records=10)
        assert len(records) == 2
        r1 = await applier._process_record(records[0])
        r2 = await applier._process_record(records[1])
        assert r1.skipped
        assert r2.success and not r2.skipped

    @pytest.mark.asyncio
    async def test_empty_assigned_tenants_accepts_all(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        # None means all accepted
        applier = _build_applier(wal, store, assigned_tenants=None)

        for tid in ["t1", "t2", "t3"]:
            event = _make_event(tid, f"ev-{tid}")
            r = await applier.apply_event(event)
            assert r.success and not r.skipped

    @pytest.mark.asyncio
    async def test_multiple_tenants_assigned(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"t1", "t2", "t3"}))

        for tid in ["t1", "t2", "t3"]:
            event = _make_event(tid, f"ev-{tid}")
            r = await applier.apply_event(event)
            assert r.success
            assert not r.skipped

    @pytest.mark.asyncio
    async def test_tenant_not_in_set_is_skipped(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"t1"}))

        await wal.append("test-wal", "t2", _event_bytes("t2", "blocked-1"))
        records = await wal.poll_batch("test-wal", "test-group", max_records=1)
        r = await applier._process_record(records[0])
        assert r.skipped

    @pytest.mark.asyncio
    async def test_skipped_count_increments(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"t1"}))

        for i in range(5):
            await wal.append("test-wal", "other", _event_bytes("other", f"skip-{i}"))

        records = await wal.poll_batch("test-wal", "test-group", max_records=10)
        for rec in records:
            await applier._process_record(rec)

        assert applier._skipped_count == 5

    @pytest.mark.asyncio
    async def test_stats_include_skipped_count(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"t1"}))

        await wal.append("test-wal", "other", _event_bytes("other", "s1"))
        records = await wal.poll_batch("test-wal", "test-group", max_records=1)
        await applier._process_record(records[0])

        stats = applier.stats
        assert "skipped_count" in stats
        assert stats["skipped_count"] == 1

    @pytest.mark.asyncio
    async def test_100_events_mixed_tenants(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"mine"}))

        for i in range(100):
            tid = "mine" if i % 2 == 0 else "other"
            await wal.append("test-wal", tid, _event_bytes(tid, f"ev-{i}", node_id=f"n-{i}"))

        records = await wal.poll_batch("test-wal", "test-group", max_records=200)
        processed = 0
        skipped = 0
        for rec in records:
            r = await applier._process_record(rec)
            if r.skipped:
                skipped += 1
            else:
                processed += 1

        assert processed == 50
        assert skipped == 50

    @pytest.mark.asyncio
    async def test_batch_mode_with_filtering(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"mine"}), batch_size=10)

        for i in range(10):
            tid = "mine" if i < 5 else "other"
            await wal.append("test-wal", tid, _event_bytes(tid, f"b-{i}", node_id=f"bn-{i}"))

        records = await wal.poll_batch("test-wal", "test-group", max_records=20)
        for rec in records:
            await applier._process_record(rec)

        assert applier._skipped_count == 5

    @pytest.mark.asyncio
    async def test_filter_does_not_affect_event_ordering(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"mine"}))

        # Interleave mine/other
        await wal.append("test-wal", "mine", _event_bytes("mine", "first", node_id="first"))
        await wal.append("test-wal", "other", _event_bytes("other", "middle"))
        await wal.append("test-wal", "mine", _event_bytes("mine", "last", node_id="last"))

        records = await wal.poll_batch("test-wal", "test-group", max_records=10)
        created = []
        for rec in records:
            r = await applier._process_record(rec)
            if not r.skipped:
                created.extend(r.created_nodes)

        assert created == ["first", "last"]

    @pytest.mark.asyncio
    async def test_filter_with_single_tenant(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"only-one"}))

        event = _make_event("only-one", "single-ev")
        r = await applier.apply_event(event)
        assert r.success and not r.skipped

    @pytest.mark.asyncio
    async def test_reassign_tenants_restart(self, tmp_path):
        """Simulates tenant reassignment by creating a new applier with different set."""
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)

        # First applier: only t1
        applier1 = _build_applier(wal, store, assigned_tenants=frozenset({"t1"}))
        e1 = _make_event("t1", "phase-1", node_id="p1")
        r1 = await applier1.apply_event(e1)
        assert r1.success and not r1.skipped

        # Second applier: only t2 (simulates reassignment)
        applier2 = _build_applier(wal, store, assigned_tenants=frozenset({"t2"}))
        e2 = _make_event("t2", "phase-2", node_id="p2")
        r2 = await applier2.apply_event(e2)
        assert r2.success and not r2.skipped

        # Verify both nodes exist in their respective tenant DBs
        assert await store.get_node("t1", "p1") is not None
        assert await store.get_node("t2", "p2") is not None

    @pytest.mark.asyncio
    async def test_applier_process_record_assigns_stream_pos(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"t1"}))

        await wal.append("test-wal", "t1", _event_bytes("t1", "with-pos", node_id="wpos"))
        records = await wal.poll_batch("test-wal", "test-group", max_records=1)
        r = await applier._process_record(records[0])
        assert r.success
        assert r.event.stream_pos is not None


# =========================================================================
# MULTI-APPLIER ISOLATION
# =========================================================================


@pytest.mark.integration
class TestMultiApplierIsolation:
    """Tests isolation between two appliers processing the same WAL."""

    @pytest.mark.asyncio
    async def test_two_appliers_different_tenants(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)

        applier_a = _build_applier(wal, store, assigned_tenants=frozenset({"ta"}))
        applier_b = _build_applier(wal, store, assigned_tenants=frozenset({"tb"}))

        # Both process their tenant events
        ra = await applier_a.apply_event(_make_event("ta", "ea", node_id="na"))
        rb = await applier_b.apply_event(_make_event("tb", "eb", node_id="nb"))
        assert ra.success and rb.success

    @pytest.mark.asyncio
    async def test_each_creates_correct_nodes(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)

        applier_a = _build_applier(wal, store, assigned_tenants=frozenset({"ta"}))
        applier_b = _build_applier(wal, store, assigned_tenants=frozenset({"tb"}))

        await applier_a.apply_event(_make_event("ta", "ea1", node_id="na1"))
        await applier_a.apply_event(_make_event("ta", "ea2", node_id="na2"))
        await applier_b.apply_event(_make_event("tb", "eb1", node_id="nb1"))

        ta_nodes = await store.get_nodes_by_type("ta", 1)
        tb_nodes = await store.get_nodes_by_type("tb", 1)
        assert len(ta_nodes) == 2
        assert len(tb_nodes) == 1

    @pytest.mark.asyncio
    async def test_no_cross_contamination(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)

        applier_a = _build_applier(wal, store, assigned_tenants=frozenset({"ta"}))
        applier_b = _build_applier(wal, store, assigned_tenants=frozenset({"tb"}))

        await applier_a.apply_event(_make_event("ta", "ea", node_id="shared-id"))
        await applier_b.apply_event(_make_event("tb", "eb", node_id="shared-id"))

        na = await store.get_node("ta", "shared-id")
        nb = await store.get_node("tb", "shared-id")
        assert na.payload["key"] == "ea"
        assert nb.payload["key"] == "eb"

    @pytest.mark.asyncio
    async def test_both_reach_end_of_stream(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)

        for i in range(10):
            tid = "ta" if i % 2 == 0 else "tb"
            await wal.append("test-wal", tid, _event_bytes(tid, f"ev-{i}", node_id=f"n-{i}"))

        applier_a = _build_applier(wal, store, assigned_tenants=frozenset({"ta"}))
        applier_b = _build_applier(wal, store, assigned_tenants=frozenset({"tb"}))

        records = await wal.poll_batch("test-wal", "test-group", max_records=20)
        for rec in records:
            await applier_a._process_record(rec)
            await applier_b._process_record(rec)

        ta_nodes = await store.get_nodes_by_type("ta", 1)
        tb_nodes = await store.get_nodes_by_type("tb", 1)
        assert len(ta_nodes) == 5
        assert len(tb_nodes) == 5

    @pytest.mark.asyncio
    async def test_applier_a_events_not_in_b_sqlite(self, tmp_path):
        store_a = CanonicalStore(data_dir=str(tmp_path / "a"), wal_mode=True)
        store_b = CanonicalStore(data_dir=str(tmp_path / "b"), wal_mode=True)
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        applier_a = _build_applier(wal, store_a, assigned_tenants=frozenset({"ta"}))
        applier_b = _build_applier(wal, store_b, assigned_tenants=frozenset({"tb"}))

        await applier_a.apply_event(_make_event("ta", "ea", node_id="only-a"))
        await applier_b.apply_event(_make_event("tb", "eb", node_id="only-b"))

        # store_a should not have tb's node
        assert await store_a.get_node("ta", "only-a") is not None
        assert not await store_a.tenant_exists("tb")

        # store_b should not have ta's node
        assert await store_b.get_node("tb", "only-b") is not None
        assert not await store_b.tenant_exists("ta")

    @pytest.mark.asyncio
    async def test_concurrent_applier_processing(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)

        applier_a = _build_applier(wal, store, assigned_tenants=frozenset({"ta"}))
        applier_b = _build_applier(wal, store, assigned_tenants=frozenset({"tb"}))

        # Concurrently apply events
        results = await asyncio.gather(
            applier_a.apply_event(_make_event("ta", "ca1", node_id="ca1")),
            applier_b.apply_event(_make_event("tb", "cb1", node_id="cb1")),
            applier_a.apply_event(_make_event("ta", "ca2", node_id="ca2")),
            applier_b.apply_event(_make_event("tb", "cb2", node_id="cb2")),
        )
        assert all(r.success for r in results)

    @pytest.mark.asyncio
    async def test_applier_restart_recovers(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)

        applier1 = _build_applier(wal, store, assigned_tenants=frozenset({"t1"}))
        await applier1.apply_event(_make_event("t1", "before", node_id="before"))

        # "Restart" with a new applier using same store
        applier2 = _build_applier(wal, store, assigned_tenants=frozenset({"t1"}))
        await applier2.apply_event(_make_event("t1", "after", node_id="after"))

        assert await store.get_node("t1", "before") is not None
        assert await store.get_node("t1", "after") is not None

    @pytest.mark.asyncio
    async def test_overlapping_tenants(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)

        applier_a = _build_applier(wal, store, assigned_tenants=frozenset({"shared", "ta"}))
        applier_b = _build_applier(wal, store, assigned_tenants=frozenset({"shared", "tb"}))

        # Both process "shared" tenant
        ra = await applier_a.apply_event(_make_event("shared", "s1", node_id="s1"))
        rb = await applier_b.apply_event(_make_event("shared", "s2", node_id="s2"))
        assert ra.success and rb.success

        nodes = await store.get_nodes_by_type("shared", 1)
        assert len(nodes) == 2

    @pytest.mark.asyncio
    async def test_empty_stream_handling(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"t1"}))

        records = await wal.poll_batch("test-wal", "test-group", max_records=10, timeout_ms=50)
        assert records == []
        assert applier._skipped_count == 0

    @pytest.mark.asyncio
    async def test_large_batch_mixed_tenants(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"mine"}))

        tenants = ["mine", "other1", "other2", "other3"]
        for i in range(40):
            tid = tenants[i % len(tenants)]
            await wal.append("test-wal", tid, _event_bytes(tid, f"lg-{i}", node_id=f"lg-{i}"))

        records = await wal.poll_batch("test-wal", "test-group", max_records=100)
        for rec in records:
            await applier._process_record(rec)

        mine_nodes = await store.get_nodes_by_type("mine", 1)
        assert len(mine_nodes) == 10
        assert applier._skipped_count == 30

    @pytest.mark.asyncio
    async def test_process_record_with_malformed_json(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"t1"}))

        await wal.append("test-wal", "t1", b"not-json")
        records = await wal.poll_batch("test-wal", "test-group", max_records=1)
        r = await applier._process_record(records[0])
        assert not r.success

    @pytest.mark.asyncio
    async def test_process_record_missing_required_fields(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"t1"}))

        await wal.append("test-wal", "t1", json.dumps({"tenant_id": "t1"}).encode())
        records = await wal.poll_batch("test-wal", "test-group", max_records=1)
        r = await applier._process_record(records[0])
        assert not r.success

    @pytest.mark.asyncio
    async def test_filter_with_frozenset_of_many_tenants(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        big_set = frozenset(f"tenant-{i}" for i in range(50))
        applier = _build_applier(wal, store, assigned_tenants=big_set)

        r = await applier.apply_event(_make_event("tenant-0", "big-ev", node_id="big-n"))
        assert r.success and not r.skipped

    @pytest.mark.asyncio
    async def test_skipped_event_does_not_create_tenant_db(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store, assigned_tenants=frozenset({"mine"}))

        await wal.append("test-wal", "other", _event_bytes("other", "nope"))
        records = await wal.poll_batch("test-wal", "test-group", max_records=1)
        await applier._process_record(records[0])

        assert not await store.tenant_exists("other")


# =========================================================================
# SHARDING CONFIG INTEGRATION
# =========================================================================


@pytest.mark.integration
class TestShardingConfigIntegration:
    """Tests for ShardingConfig construction and behavior."""

    def test_from_env_default(self, monkeypatch):
        monkeypatch.delenv("NODE_ID", raising=False)
        monkeypatch.delenv("ASSIGNED_TENANTS", raising=False)
        monkeypatch.delenv("TENANT_REGISTRY", raising=False)
        config = ShardingConfig.from_env()
        assert config.node_id == "default"
        assert config.assigned_tenants == frozenset()

    def test_from_env_with_tenants(self, monkeypatch):
        monkeypatch.setenv("NODE_ID", "node-a")
        monkeypatch.setenv("ASSIGNED_TENANTS", "t1,t2,t3")
        config = ShardingConfig.from_env()
        assert config.assigned_tenants == frozenset({"t1", "t2", "t3"})

    def test_node_id_from_env(self, monkeypatch):
        monkeypatch.setenv("NODE_ID", "custom-node")
        monkeypatch.delenv("ASSIGNED_TENANTS", raising=False)
        config = ShardingConfig.from_env()
        assert config.node_id == "custom-node"

    def test_tenant_registry_from_env(self, monkeypatch):
        monkeypatch.setenv("TENANT_REGISTRY", json.dumps({"t1": "node-a", "t2": "node-b"}))
        monkeypatch.delenv("ASSIGNED_TENANTS", raising=False)
        config = ShardingConfig.from_env()
        assert config.tenant_registry == {"t1": "node-a", "t2": "node-b"}

    def test_is_multi_node_true(self):
        config = ShardingConfig(assigned_tenants=frozenset({"t1"}))
        assert config.is_multi_node is True

    def test_is_multi_node_false(self):
        config = ShardingConfig(assigned_tenants=frozenset())
        assert config.is_multi_node is False

    def test_is_mine_with_assigned_tenants(self):
        config = ShardingConfig(assigned_tenants=frozenset({"t1", "t2"}))
        assert config.is_mine("t1") is True
        assert config.is_mine("t2") is True
        assert config.is_mine("t3") is False

    def test_is_mine_empty_accepts_all(self):
        config = ShardingConfig(assigned_tenants=frozenset())
        assert config.is_mine("any-tenant") is True
        assert config.is_mine("") is True

    def test_config_with_100_tenants(self):
        tenants = frozenset(f"tenant-{i}" for i in range(100))
        config = ShardingConfig(assigned_tenants=tenants)
        assert len(config.assigned_tenants) == 100
        assert config.is_mine("tenant-50") is True
        assert config.is_mine("tenant-200") is False

    def test_get_owner_returns_node(self):
        config = ShardingConfig(
            tenant_registry={"t1": "node-a", "t2": "node-b"},
        )
        assert config.get_owner("t1") == "node-a"
        assert config.get_owner("t2") == "node-b"
        assert config.get_owner("t3") is None

    def test_get_owner_no_registry(self):
        config = ShardingConfig()
        assert config.get_owner("t1") is None

    def test_default_config_accepts_all(self):
        config = ShardingConfig()
        assert config.is_mine("anything") is True
        assert not config.is_multi_node

    def test_invalid_registry_json_ignored(self, monkeypatch):
        monkeypatch.setenv("TENANT_REGISTRY", "not-json{")
        monkeypatch.delenv("ASSIGNED_TENANTS", raising=False)
        config = ShardingConfig.from_env()
        assert config.tenant_registry is None

    def test_from_env_strips_whitespace(self, monkeypatch):
        monkeypatch.setenv("ASSIGNED_TENANTS", " t1 , t2 , t3 ")
        config = ShardingConfig.from_env()
        assert config.assigned_tenants == frozenset({"t1", "t2", "t3"})

    def test_frozen_dataclass_is_immutable(self):
        config = ShardingConfig(node_id="x", assigned_tenants=frozenset({"t1"}))
        with pytest.raises(AttributeError):
            config.node_id = "y"

    def test_is_mine_performance_large_set(self):
        tenants = frozenset(f"tenant-{i}" for i in range(10000))
        config = ShardingConfig(assigned_tenants=tenants)
        # frozenset lookup is O(1)
        assert config.is_mine("tenant-9999") is True
        assert config.is_mine("tenant-99999") is False

    def test_empty_string_tenant_id(self):
        config = ShardingConfig(assigned_tenants=frozenset({""}))
        assert config.is_mine("") is True
        assert config.is_mine("x") is False

    def test_from_env_empty_string(self, monkeypatch):
        monkeypatch.setenv("ASSIGNED_TENANTS", "")
        config = ShardingConfig.from_env()
        assert config.assigned_tenants == frozenset()

    def test_from_env_single_tenant(self, monkeypatch):
        monkeypatch.setenv("ASSIGNED_TENANTS", "only-one")
        config = ShardingConfig.from_env()
        assert config.assigned_tenants == frozenset({"only-one"})

    def test_from_env_trailing_comma(self, monkeypatch):
        monkeypatch.setenv("ASSIGNED_TENANTS", "a,b,")
        config = ShardingConfig.from_env()
        assert config.assigned_tenants == frozenset({"a", "b"})

    def test_from_env_leading_comma(self, monkeypatch):
        monkeypatch.setenv("ASSIGNED_TENANTS", ",a,b")
        config = ShardingConfig.from_env()
        assert config.assigned_tenants == frozenset({"a", "b"})

    def test_registry_with_many_entries(self):
        registry = {f"t{i}": f"node-{i % 3}" for i in range(50)}
        config = ShardingConfig(tenant_registry=registry)
        assert config.get_owner("t0") == "node-0"
        assert config.get_owner("t49") == "node-1"


# =========================================================================
# APPLIER WITH WAL STREAM INTERACTIONS
# =========================================================================


@pytest.mark.integration
class TestApplierWalStreamInteractions:
    """Tests for applier interacting with the WAL stream."""

    @pytest.mark.asyncio
    async def test_wal_append_and_poll(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        await wal.append("topic", "key", b"value")
        records = await wal.poll_batch("topic", "grp", max_records=1)
        assert len(records) == 1

    @pytest.mark.asyncio
    async def test_wal_commit_advances_position(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(3):
            await wal.append("topic", "k", f"v{i}".encode())

        r1 = await wal.poll_batch("topic", "grp", max_records=1)
        await wal.commit(r1[0])
        r2 = await wal.poll_batch("topic", "grp", max_records=1)
        assert r2[0].value != r1[0].value

    @pytest.mark.asyncio
    async def test_wal_get_record_count(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        assert wal.get_record_count("topic") == 0
        for i in range(5):
            await wal.append("topic", "k", f"v{i}".encode())
        assert wal.get_record_count("topic") == 5

    @pytest.mark.asyncio
    async def test_wal_clear_topic(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(5):
            await wal.append("topic", "k", f"v{i}".encode())
        assert wal.get_record_count("topic") == 5
        wal.clear_topic("topic")
        assert wal.get_record_count("topic") == 0

    @pytest.mark.asyncio
    async def test_wal_get_all_records(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(3):
            await wal.append("topic", "k", f"v{i}".encode())
        all_records = wal.get_all_records("topic")
        assert len(all_records) == 3

    @pytest.mark.asyncio
    async def test_wal_multiple_topics(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        await wal.append("topic-a", "k", b"a")
        await wal.append("topic-b", "k", b"b")
        assert wal.get_record_count("topic-a") == 1
        assert wal.get_record_count("topic-b") == 1

    @pytest.mark.asyncio
    async def test_wal_is_connected(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        assert not wal.is_connected
        await wal.connect()
        assert wal.is_connected
        await wal.close()
        assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_wal_partition_routing(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=4)
        await wal.connect()
        # Same key always goes to same partition
        pos1 = await wal.append("t", "stable-key", b"v1")
        pos2 = await wal.append("t", "stable-key", b"v2")
        assert pos1.partition == pos2.partition

    @pytest.mark.asyncio
    async def test_wal_different_keys_may_differ_partitions(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=8)
        await wal.connect()
        partitions = set()
        for i in range(50):
            pos = await wal.append("t", f"key-{i}", f"v{i}".encode())
            partitions.add(pos.partition)
        # With 50 different keys and 8 partitions, we should see multiple partitions
        assert len(partitions) > 1

    @pytest.mark.asyncio
    async def test_wal_stream_pos_attributes(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        pos = await wal.append("my-topic", "my-key", b"val")
        assert pos.topic == "my-topic"
        assert pos.partition >= 0
        assert pos.offset >= 0
        assert pos.timestamp_ms > 0

    @pytest.mark.asyncio
    async def test_wal_record_value_json(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        payload = {"key": "value", "num": 42}
        await wal.append("topic", "k", json.dumps(payload).encode())
        records = await wal.poll_batch("topic", "grp", max_records=1)
        parsed = records[0].value_json()
        assert parsed["key"] == "value"
        assert parsed["num"] == 42

    @pytest.mark.asyncio
    async def test_wal_headers(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        headers = {"content-type": b"application/json"}
        await wal.append("topic", "k", b"val", headers=headers)
        records = await wal.poll_batch("topic", "grp", max_records=1)
        assert records[0].headers["content-type"] == b"application/json"

    @pytest.mark.asyncio
    async def test_wal_wait_for_records(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(5):
            await wal.append("t", "k", f"v{i}".encode())
        reached = await wal.wait_for_records("t", 5, timeout=1.0)
        assert reached is True

    @pytest.mark.asyncio
    async def test_wal_wait_for_records_timeout(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        reached = await wal.wait_for_records("t", 10, timeout=0.1)
        assert reached is False

    @pytest.mark.asyncio
    async def test_applier_apply_creates_tenant_if_needed(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store)

        assert not await store.tenant_exists("new-tenant")
        event = _make_event("new-tenant", "auto-init", node_id="auto-n")
        r = await applier.apply_event(event)
        assert r.success
        assert await store.tenant_exists("new-tenant")

    @pytest.mark.asyncio
    async def test_applier_schema_fingerprint_match(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = Applier(
            wal=wal,
            canonical_store=store,
            topic="test-wal",
            schema_fingerprint="sha256:abc",
            fanout_config=MailboxFanoutConfig(enabled=False),
        )

        event = TransactionEvent(
            tenant_id="t1",
            actor="u:1",
            idempotency_key="fp-match",
            schema_fingerprint="sha256:abc",
            ts_ms=int(time.time() * 1000),
            ops=[{"op": "create_node", "type_id": 1, "id": "fp-n", "data": {}}],
        )
        r = await applier.apply_event(event)
        assert r.success

    @pytest.mark.asyncio
    async def test_applier_schema_fingerprint_mismatch(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = Applier(
            wal=wal,
            canonical_store=store,
            topic="test-wal",
            schema_fingerprint="sha256:expected",
            fanout_config=MailboxFanoutConfig(enabled=False),
        )

        event = TransactionEvent(
            tenant_id="t1",
            actor="u:1",
            idempotency_key="fp-bad",
            schema_fingerprint="sha256:wrong",
            ts_ms=int(time.time() * 1000),
            ops=[{"op": "create_node", "type_id": 1, "id": "fp-bad-n", "data": {}}],
        )
        r = await applier.apply_event(event)
        assert not r.success
        assert "Schema mismatch" in r.error

    @pytest.mark.asyncio
    async def test_applier_stats_tracking(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store)

        stats_before = applier.stats
        assert stats_before["processed_count"] == 0
        assert stats_before["error_count"] == 0
        assert stats_before["running"] is False

        event = _make_event("t1", "stat-ev", node_id="stat-n")
        await applier.apply_event(event)
        # processed_count is only updated via _log_result, not apply_event directly

    @pytest.mark.asyncio
    async def test_applier_stop(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store)

        await applier.stop()
        assert not applier._running

    @pytest.mark.asyncio
    async def test_applier_create_edge_via_event(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store)

        event = TransactionEvent(
            tenant_id="t1",
            actor="u:1",
            idempotency_key="edge-ev",
            schema_fingerprint=None,
            ts_ms=int(time.time() * 1000),
            ops=[
                {"op": "create_node", "type_id": 1, "id": "ea", "data": {}},
                {"op": "create_node", "type_id": 1, "id": "eb", "data": {}},
                {"op": "create_edge", "edge_id": 1, "from": "ea", "to": "eb"},
            ],
        )
        r = await applier.apply_event(event)
        assert r.success
        edges = await store.get_edges_from("t1", "ea")
        assert len(edges) == 1

    @pytest.mark.asyncio
    async def test_applier_delete_node_via_event(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store)

        # Create then delete
        create = _make_event("t1", "del-create", node_id="del-target")
        await applier.apply_event(create)
        assert await store.get_node("t1", "del-target") is not None

        delete_event = TransactionEvent(
            tenant_id="t1",
            actor="u:1",
            idempotency_key="del-exec",
            schema_fingerprint=None,
            ts_ms=int(time.time() * 1000),
            ops=[{"op": "delete_node", "id": "del-target"}],
        )
        r = await applier.apply_event(delete_event)
        assert r.success
        assert await store.get_node("t1", "del-target") is None

    @pytest.mark.asyncio
    async def test_applier_update_node_via_event(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store)

        create = _make_event("t1", "upd-create", node_id="upd-target", payload={"v": 1})
        await applier.apply_event(create)

        update_event = TransactionEvent(
            tenant_id="t1",
            actor="u:1",
            idempotency_key="upd-exec",
            schema_fingerprint=None,
            ts_ms=int(time.time() * 1000),
            ops=[{"op": "update_node", "id": "upd-target", "patch": {"v": 2}}],
        )
        r = await applier.apply_event(update_event)
        assert r.success
        node = await store.get_node("t1", "upd-target")
        assert node.payload["v"] == 2

    @pytest.mark.asyncio
    async def test_transaction_event_from_dict(self, tmp_path):
        data = {
            "tenant_id": "t1",
            "actor": "u:1",
            "idempotency_key": "k1",
            "ops": [{"op": "create_node", "type_id": 1, "data": {}}],
        }
        event = TransactionEvent.from_dict(data)
        assert event.tenant_id == "t1"
        assert event.actor == "u:1"

    @pytest.mark.asyncio
    async def test_transaction_event_from_dict_missing_fields(self, tmp_path):
        with pytest.raises(ValueError, match="Missing required fields"):
            TransactionEvent.from_dict({"tenant_id": "t1"})

    @pytest.mark.asyncio
    async def test_applier_delete_edge_via_event(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store)

        # Create nodes + edge
        create = TransactionEvent(
            tenant_id="t1",
            actor="u:1",
            idempotency_key="de-create",
            schema_fingerprint=None,
            ts_ms=int(time.time() * 1000),
            ops=[
                {"op": "create_node", "type_id": 1, "id": "de-a", "data": {}},
                {"op": "create_node", "type_id": 1, "id": "de-b", "data": {}},
                {"op": "create_edge", "edge_id": 1, "from": "de-a", "to": "de-b"},
            ],
        )
        await applier.apply_event(create)
        assert len(await store.get_edges_from("t1", "de-a")) == 1

        # Delete edge
        del_edge = TransactionEvent(
            tenant_id="t1",
            actor="u:1",
            idempotency_key="de-del",
            schema_fingerprint=None,
            ts_ms=int(time.time() * 1000),
            ops=[{"op": "delete_edge", "edge_id": 1, "from": "de-a", "to": "de-b"}],
        )
        r = await applier.apply_event(del_edge)
        assert r.success
        assert len(await store.get_edges_from("t1", "de-a")) == 0

    @pytest.mark.asyncio
    async def test_applier_multi_op_with_alias(self, tmp_path):
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        applier = _build_applier(wal, store)

        event = TransactionEvent(
            tenant_id="t1",
            actor="u:1",
            idempotency_key="alias-ev",
            schema_fingerprint=None,
            ts_ms=int(time.time() * 1000),
            ops=[
                {
                    "op": "create_node",
                    "type_id": 1,
                    "id": "alias-node",
                    "data": {"x": 1},
                    "as": "myNode",
                },
            ],
        )
        r = await applier.apply_event(event)
        assert r.success
        assert "alias-node" in r.created_nodes


# =========================================================================
# CANONICAL STORE INTEGRATION
# =========================================================================


@pytest.mark.integration
class TestCanonicalStoreIntegration:
    """Additional integration tests for the canonical store."""

    @pytest.fixture
    def store(self, tmp_path):
        s = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        asyncio.get_event_loop().run_until_complete(s.initialize_tenant("test"))
        return s

    @pytest.mark.asyncio
    async def test_create_multiple_types(self, store):
        for t in [1, 2, 3, 4, 5]:
            await store.create_node("test", t, {"type": t}, "u:1")
        for t in [1, 2, 3, 4, 5]:
            nodes = await store.get_nodes_by_type("test", t)
            assert len(nodes) == 1

    @pytest.mark.asyncio
    async def test_pagination_full_coverage(self, store):
        for i in range(30):
            await store.create_node("test", 1, {"i": i}, "u:1")

        all_ids = set()
        for offset in range(0, 30, 10):
            page = await store.get_nodes_by_type("test", 1, limit=10, offset=offset)
            for n in page:
                all_ids.add(n.node_id)
        assert len(all_ids) == 30

    @pytest.mark.asyncio
    async def test_update_multiple_times(self, store):
        node = await store.create_node("test", 1, {"v": 0}, "u:1")
        for i in range(1, 20):
            await store.update_node("test", node.node_id, {"v": i})

        final = await store.get_node("test", node.node_id)
        assert final.payload["v"] == 19

    @pytest.mark.asyncio
    async def test_delete_nonexistent_is_safe(self, store):
        r = await store.delete_node("test", "ghost-id")
        assert r is False

    @pytest.mark.asyncio
    async def test_edge_bidirectional_query(self, store):
        n1 = await store.create_node("test", 1, {}, "u:1")
        n2 = await store.create_node("test", 1, {}, "u:1")
        await store.create_edge("test", 1, n1.node_id, n2.node_id)

        from_edges = await store.get_edges_from("test", n1.node_id)
        to_edges = await store.get_edges_to("test", n2.node_id)
        assert len(from_edges) == 1
        assert len(to_edges) == 1

    @pytest.mark.asyncio
    async def test_multiple_edge_types(self, store):
        n1 = await store.create_node("test", 1, {}, "u:1")
        n2 = await store.create_node("test", 1, {}, "u:1")
        for et in [1, 2, 3]:
            await store.create_edge("test", et, n1.node_id, n2.node_id)

        all_edges = await store.get_edges_from("test", n1.node_id)
        assert len(all_edges) == 3

    @pytest.mark.asyncio
    async def test_tenant_exists_true(self, store):
        assert await store.tenant_exists("test") is True

    @pytest.mark.asyncio
    async def test_tenant_exists_false(self, store):
        assert await store.tenant_exists("nope") is False

    @pytest.mark.asyncio
    async def test_create_with_specified_timestamp(self, store):
        node = await store.create_node("test", 1, {}, "u:1", created_at=555000)
        assert node.created_at == 555000

    @pytest.mark.asyncio
    async def test_get_nodes_empty_tenant(self, store):
        # A newly initialized tenant has no nodes
        nodes = await store.get_nodes_by_type("test", 1)
        assert nodes == []

    @pytest.mark.asyncio
    async def test_idempotency_different_tenants(self, tmp_path):
        store = CanonicalStore(data_dir=str(tmp_path / "multi"), wal_mode=True)
        await store.initialize_tenant("a")
        await store.initialize_tenant("b")

        await store.record_applied_event("a", "same-key")
        assert await store.check_idempotency("a", "same-key") is True
        assert await store.check_idempotency("b", "same-key") is False

    @pytest.mark.asyncio
    async def test_batch_empty_transaction(self, store):
        """Empty batch transaction commits successfully."""
        with store.batch_transaction("test"):
            pass  # no operations
        # Should not raise

    @pytest.mark.asyncio
    async def test_large_batch_nodes(self, store):
        with store.batch_transaction("test") as conn:
            for i in range(200):
                store.create_node_raw(conn, "test", 1, {"i": i}, "u:1", node_id=f"lb-{i}")

        nodes = await store.get_nodes_by_type("test", 1, limit=300)
        assert len(nodes) == 200

    @pytest.mark.asyncio
    async def test_get_node_returns_correct_owner(self, store):
        await store.create_node("test", 1, {}, "user:42", node_id="owned")
        node = await store.get_node("test", "owned")
        assert node.owner_actor == "user:42"

    @pytest.mark.asyncio
    async def test_edge_props_persisted(self, store):
        n1 = await store.create_node("test", 1, {}, "u:1")
        n2 = await store.create_node("test", 1, {}, "u:1")
        await store.create_edge("test", 1, n1.node_id, n2.node_id, props={"w": 0.9})
        edges = await store.get_edges_from("test", n1.node_id)
        assert edges[0].props["w"] == 0.9

    @pytest.mark.asyncio
    async def test_create_same_id_different_tenants(self, tmp_path):
        store = CanonicalStore(data_dir=str(tmp_path / "shared"), wal_mode=True)
        await store.initialize_tenant("x")
        await store.initialize_tenant("y")

        await store.create_node("x", 1, {"from": "x"}, "u:1", node_id="shared")
        await store.create_node("y", 1, {"from": "y"}, "u:1", node_id="shared")

        nx = await store.get_node("x", "shared")
        ny = await store.get_node("y", "shared")
        assert nx.payload["from"] == "x"
        assert ny.payload["from"] == "y"

    @pytest.mark.asyncio
    async def test_concurrent_reads_and_writes(self, store):
        # Create some nodes
        for i in range(10):
            await store.create_node("test", 1, {"i": i}, "u:1", node_id=f"cr-{i}")

        # Read and write concurrently
        results = await asyncio.gather(
            store.get_nodes_by_type("test", 1),
            store.create_node("test", 1, {"i": 99}, "u:1", node_id="cr-99"),
            store.get_node("test", "cr-0"),
        )
        assert len(results[0]) >= 10
        assert results[1] is not None
        assert results[2] is not None

    @pytest.mark.asyncio
    async def test_node_payload_with_nested_objects(self, store):
        payload = {"a": {"b": {"c": {"d": [1, 2, {"e": "deep"}]}}}}
        node = await store.create_node("test", 1, payload, "u:1")
        fetched = await store.get_node("test", node.node_id)
        assert fetched.payload["a"]["b"]["c"]["d"][2]["e"] == "deep"
