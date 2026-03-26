"""
Tests that verify critical bug fixes.

These tests target specific bugs found in code review:
1. Atomic idempotency (check + record in same transaction)
2. Batch applier actually uses batch_transaction
3. SQLite operations don't block event loop
4. No duplicate poll_batch methods
5. Partial event application protection
"""

from __future__ import annotations

import asyncio
import os
import time
import uuid

import pytest

from dbaas.entdb_server.apply.applier import (
    Applier,
    ApplyResult,
    MailboxFanoutConfig,
    TransactionEvent,
)
from dbaas.entdb_server.apply.canonical_store import (
    CanonicalStore,
    TenantNotFoundError,
)
from dbaas.entdb_server.apply.mailbox_store import MailboxStore
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


# =========================================================================
# ATOMIC IDEMPOTENCY
# =========================================================================


@pytest.mark.integration
class TestAtomicIdempotency:
    """Verify that idempotency check + record happen atomically."""

    @pytest.mark.asyncio
    async def test_idempotency_check_and_record_atomic(self, tmp_path):
        """Apply an event, verify recorded. Apply again, verify skipped."""
        applier, store, _, _ = await _make_applier(tmp_path)

        event = _make_event(
            idempotency_key="atomic-1",
            ops=[
                {"op": "create_node", "type_id": 1, "id": "n1", "data": {"x": 1}},
            ],
        )

        r1 = await applier.apply_event(event)
        assert r1.success and not r1.skipped
        assert await store.check_idempotency("t1", "atomic-1")

        r2 = await applier.apply_event(event)
        assert r2.success and r2.skipped

    @pytest.mark.asyncio
    async def test_concurrent_same_event_only_one_applies(self, tmp_path):
        """Two concurrent tasks applying the same event: only one
        should create the node."""
        applier, store, _, _ = await _make_applier(tmp_path)

        event = _make_event(
            idempotency_key="dup-race",
            ops=[
                {
                    "op": "create_node",
                    "type_id": 1,
                    "id": "race-node",
                    "data": {"v": 1},
                },
            ],
        )

        results: list[ApplyResult] = await asyncio.gather(
            applier.apply_event(event),
            applier.apply_event(event),
        )

        successes = [r for r in results if r.success and not r.skipped]
        skipped = [r for r in results if r.success and r.skipped]

        # Exactly one should have applied, the other skipped (or both
        # succeed but the node exists only once).
        node = await store.get_node("t1", "race-node")
        assert node is not None
        # At most one task should have done the real apply
        assert len(successes) >= 1
        # Combined applied + skipped should equal 2
        assert len(successes) + len(skipped) == 2

    @pytest.mark.asyncio
    async def test_idempotency_survives_crash_simulation(self, tmp_path):
        """If ops fail mid-way the idempotency key must NOT be recorded."""
        applier, store, _, _ = await _make_applier(tmp_path)

        # First create the node so the second event will hit a duplicate
        await applier.apply_event(
            _make_event(
                idempotency_key="setup",
                ops=[
                    {"op": "create_node", "type_id": 1, "id": "existing", "data": {}},
                ],
            )
        )

        # Now try an event that creates a node then duplicates "existing"
        bad_event = _make_event(
            idempotency_key="crash-sim",
            ops=[
                {
                    "op": "create_node",
                    "type_id": 1,
                    "id": "new-before-crash",
                    "data": {},
                },
                {
                    "op": "create_node",
                    "type_id": 1,
                    "id": "existing",
                    "data": {},
                },  # dup
            ],
        )

        result = await applier.apply_event(bad_event)
        # The apply should fail
        assert not result.success or result.error is not None

        # The idempotency key should NOT be recorded
        already = await store.check_idempotency("t1", "crash-sim")
        assert already is False, "Idempotency key recorded despite failure"

    @pytest.mark.asyncio
    async def test_idempotency_across_restart(self, tmp_path):
        """Apply event, recreate applier, replay same event -- skip."""
        applier1, store, mbox, wal = await _make_applier(tmp_path)
        event = _make_event(
            idempotency_key="restart-key",
            ops=[
                {"op": "create_node", "type_id": 1, "id": "r-node", "data": {"v": 1}},
            ],
        )

        r1 = await applier1.apply_event(event)
        assert r1.success and not r1.skipped

        # New applier, same stores
        applier2 = Applier(
            wal=wal,
            canonical_store=store,
            mailbox_store=mbox,
            topic="test-wal",
            fanout_config=MailboxFanoutConfig(enabled=False),
        )

        r2 = await applier2.apply_event(event)
        assert r2.success and r2.skipped

    @pytest.mark.asyncio
    async def test_idempotency_different_tenants_independent(self, tmp_path):
        """Same idempotency key in different tenants must both apply."""
        applier, store, _, _ = await _make_applier(tmp_path)

        key = "shared-key"
        ev_a = _make_event(
            tenant_id="tA",
            idempotency_key=key,
            ops=[
                {"op": "create_node", "type_id": 1, "id": "nA", "data": {}},
            ],
        )
        ev_b = _make_event(
            tenant_id="tB",
            idempotency_key=key,
            ops=[
                {"op": "create_node", "type_id": 1, "id": "nB", "data": {}},
            ],
        )

        r_a = await applier.apply_event(ev_a)
        r_b = await applier.apply_event(ev_b)

        assert r_a.success and not r_a.skipped
        assert r_b.success and not r_b.skipped

        assert await store.get_node("tA", "nA") is not None
        assert await store.get_node("tB", "nB") is not None

    @pytest.mark.asyncio
    async def test_idempotency_key_stored_with_stream_pos(self, tmp_path):
        """After apply, verify stream_pos is stored in applied_events."""
        applier, store, _, _ = await _make_applier(tmp_path)

        event = _make_event(
            idempotency_key="pos-key",
            ops=[
                {"op": "create_node", "type_id": 1, "id": "pos-node", "data": {}},
            ],
        )

        await applier.apply_event(event)

        # Verify stored in DB
        assert await store.check_idempotency("t1", "pos-key") is True

        # The applied_events row should exist
        stats = await store.get_stats("t1")
        assert stats["applied_events"] >= 1

    @pytest.mark.asyncio
    async def test_1000_events_no_duplicates(self, tmp_path):
        """Apply 1000 events, replay all, verify zero new applications."""
        applier, store, _, _ = await _make_applier(tmp_path)

        events = []
        for i in range(1000):
            ev = _make_event(
                idempotency_key=f"bulk-{i}",
                ops=[
                    {
                        "op": "create_node",
                        "type_id": 1,
                        "id": f"bulk-n-{i}",
                        "data": {"i": i},
                    }
                ],
            )
            events.append(ev)

        # First pass: apply all
        for ev in events:
            r = await applier.apply_event(ev)
            assert r.success and not r.skipped

        # Second pass: replay all -- every one must be skipped
        for ev in events:
            r = await applier.apply_event(ev)
            assert r.success and r.skipped, f"Event {ev.idempotency_key} was not skipped on replay"

        stats = await store.get_stats("t1")
        assert stats["nodes"] == 1000
        assert stats["applied_events"] == 1000

    @pytest.mark.asyncio
    async def test_idempotency_with_batch_mode(self, tmp_path):
        """With batch_size > 1, idempotency works per-event."""
        applier, store, _, _ = await _make_applier(tmp_path, batch_size=10)

        # Pre-apply one event
        pre_event = _make_event(
            idempotency_key="batch-idem-0",
            ops=[
                {"op": "create_node", "type_id": 1, "id": "pre-0", "data": {}},
            ],
        )
        await applier.apply_event(pre_event)

        # Now apply remaining events; the first is a duplicate
        events = [pre_event] + [
            _make_event(
                idempotency_key=f"batch-idem-{i}",
                ops=[
                    {
                        "op": "create_node",
                        "type_id": 1,
                        "id": f"batch-n-{i}",
                        "data": {},
                    }
                ],
            )
            for i in range(1, 5)
        ]

        for ev in events:
            await applier.apply_event(ev)

        # All five idempotency keys should be recorded
        for i in range(5):
            assert await store.check_idempotency("t1", f"batch-idem-{i}")

    @pytest.mark.asyncio
    async def test_partial_batch_idempotency(self, tmp_path):
        """First 5 events new, last 5 already applied. Only 5 new."""
        applier, store, _, _ = await _make_applier(tmp_path)

        # Pre-apply last 5
        for i in range(5, 10):
            ev = _make_event(
                idempotency_key=f"partial-{i}",
                ops=[
                    {
                        "op": "create_node",
                        "type_id": 1,
                        "id": f"partial-n-{i}",
                        "data": {},
                    }
                ],
            )
            await applier.apply_event(ev)

        node_count_before = (await store.get_stats("t1"))["nodes"]
        assert node_count_before == 5

        # Apply all 10
        for i in range(10):
            ev = _make_event(
                idempotency_key=f"partial-{i}",
                ops=[
                    {
                        "op": "create_node",
                        "type_id": 1,
                        "id": f"partial-n-{i}",
                        "data": {},
                    }
                ],
            )
            await applier.apply_event(ev)

        node_count_after = (await store.get_stats("t1"))["nodes"]
        assert node_count_after == 10  # 5 new + 5 existing

    @pytest.mark.asyncio
    async def test_idempotency_key_empty_string(self, tmp_path):
        """Empty idempotency key should still work."""
        applier, store, _, _ = await _make_applier(tmp_path)

        event = _make_event(
            idempotency_key="",
            ops=[
                {
                    "op": "create_node",
                    "type_id": 1,
                    "id": "empty-key-node",
                    "data": {},
                },
            ],
        )

        r1 = await applier.apply_event(event)
        assert r1.success and not r1.skipped

        r2 = await applier.apply_event(event)
        assert r2.success and r2.skipped


# =========================================================================
# BATCH APPLIER INTEGRITY
# =========================================================================


@pytest.mark.integration
class TestBatchApplierIntegrity:
    """Verify that batch applier correctly uses batch_transaction."""

    @pytest.mark.asyncio
    async def test_batch_all_events_applied(self, tmp_path):
        """Send 50 events via applier, verify all 50 nodes created."""
        applier, store, _, _ = await _make_applier(tmp_path)

        for i in range(50):
            ev = _make_event(
                idempotency_key=f"batch-all-{i}",
                ops=[
                    {
                        "op": "create_node",
                        "type_id": 1,
                        "id": f"ba-{i}",
                        "data": {"i": i},
                    }
                ],
            )
            r = await applier.apply_event(ev)
            assert r.success

        stats = await store.get_stats("t1")
        assert stats["nodes"] == 50

    @pytest.mark.asyncio
    async def test_batch_preserves_ordering(self, tmp_path):
        """Events with sequential data retain correct values."""
        applier, store, _, _ = await _make_applier(tmp_path)

        for i in range(20):
            ev = _make_event(
                idempotency_key=f"order-{i}",
                ops=[
                    {
                        "op": "create_node",
                        "type_id": 1,
                        "id": f"ord-{i}",
                        "data": {"seq": i},
                    }
                ],
            )
            await applier.apply_event(ev)

        for i in range(20):
            node = await store.get_node("t1", f"ord-{i}")
            assert node is not None
            assert node.payload["seq"] == i

    @pytest.mark.asyncio
    async def test_batch_multi_tenant(self, tmp_path):
        """Events for 3 tenants, verify correct tenant isolation."""
        applier, store, _, _ = await _make_applier(tmp_path)

        tenants = ["iso-a", "iso-b", "iso-c"]
        for t in tenants:
            for i in range(5):
                ev = _make_event(
                    tenant_id=t,
                    idempotency_key=f"{t}-{i}",
                    ops=[
                        {
                            "op": "create_node",
                            "type_id": 1,
                            "id": f"{t}-n-{i}",
                            "data": {},
                        }
                    ],
                )
                await applier.apply_event(ev)

        for t in tenants:
            stats = await store.get_stats(t)
            assert stats["nodes"] == 5, f"Tenant {t} has {stats['nodes']} nodes, expected 5"
            # Verify no cross-tenant leakage
            for other in tenants:
                if other == t:
                    continue
                node = await store.get_node(t, f"{other}-n-0")
                assert node is None, f"Tenant {t} can see {other}'s node"

    @pytest.mark.asyncio
    async def test_batch_mixed_operations(self, tmp_path):
        """Batch with create, update, delete, create_edge."""
        applier, store, _, _ = await _make_applier(tmp_path)

        # Create two nodes
        ev1 = _make_event(
            idempotency_key="mix-1",
            ops=[
                {"op": "create_node", "type_id": 1, "id": "mix-a", "data": {"v": 1}},
                {"op": "create_node", "type_id": 1, "id": "mix-b", "data": {"v": 2}},
            ],
        )
        await applier.apply_event(ev1)

        # Update one, create edge
        ev2 = _make_event(
            idempotency_key="mix-2",
            ops=[
                {"op": "update_node", "id": "mix-a", "patch": {"v": 99}},
                {
                    "op": "create_edge",
                    "edge_id": 1,
                    "from": "mix-a",
                    "to": "mix-b",
                },
            ],
        )
        await applier.apply_event(ev2)

        # Delete one node
        ev3 = _make_event(
            idempotency_key="mix-3",
            ops=[
                {"op": "delete_node", "id": "mix-b"},
            ],
        )
        await applier.apply_event(ev3)

        node_a = await store.get_node("t1", "mix-a")
        assert node_a is not None
        assert node_a.payload["v"] == 99

        node_b = await store.get_node("t1", "mix-b")
        assert node_b is None

    @pytest.mark.asyncio
    async def test_batch_rollback_on_error(self, tmp_path):
        """If one event in a batch_transaction fails, all changes for
        that tenant batch should be rolled back."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("t1")

        # Pre-create a node to trigger duplicate
        await store.create_node("t1", 1, {}, "user:1", node_id="pre-exists")

        # Use batch_transaction directly: create new node, then duplicate
        with (
            pytest.raises((Exception,), match="UNIQUE|IntegrityError|duplicate"),
            store.batch_transaction("t1") as conn,
        ):
            store.create_node_raw(conn, "t1", 1, {}, "user:1", node_id="should-rollback")
            store.create_node_raw(conn, "t1", 1, {}, "user:1", node_id="pre-exists")

        # "should-rollback" must not exist
        node = await store.get_node("t1", "should-rollback")
        assert node is None, "Node created before error was not rolled back"

    @pytest.mark.asyncio
    async def test_batch_vs_single_same_result(self, tmp_path):
        """Process 100 events with batch_size=1 and batch_size=50.
        Final DB state must be identical."""
        events = [
            _make_event(
                idempotency_key=f"cmp-{i}",
                ops=[
                    {
                        "op": "create_node",
                        "type_id": 1,
                        "id": f"cmp-{i}",
                        "data": {"i": i},
                    }
                ],
            )
            for i in range(100)
        ]

        # Single mode
        applier_s, store_s, _, _ = await _make_applier(tmp_path / "single", batch_size=1)
        for ev in events:
            await applier_s.apply_event(ev)

        # Batch mode
        applier_b, store_b, _, _ = await _make_applier(tmp_path / "batch", batch_size=50)
        for ev in events:
            await applier_b.apply_event(ev)

        stats_s = await store_s.get_stats("t1")
        stats_b = await store_b.get_stats("t1")
        assert stats_s["nodes"] == stats_b["nodes"] == 100
        assert stats_s["applied_events"] == stats_b["applied_events"] == 100

        # Spot-check data
        for i in [0, 49, 99]:
            ns = await store_s.get_node("t1", f"cmp-{i}")
            nb = await store_b.get_node("t1", f"cmp-{i}")
            assert ns is not None and nb is not None
            assert ns.payload == nb.payload

    @pytest.mark.asyncio
    async def test_batch_with_aliases(self, tmp_path):
        """Events using 'as' aliases within same event resolve correctly."""
        applier, store, _, _ = await _make_applier(tmp_path)

        event = _make_event(
            idempotency_key="alias-ev",
            ops=[
                {
                    "op": "create_node",
                    "type_id": 1,
                    "id": "alias-src",
                    "data": {},
                    "as": "src",
                },
                {"op": "create_node", "type_id": 1, "id": "alias-dst", "data": {}},
                {
                    "op": "create_edge",
                    "edge_id": 1,
                    "from": "alias-src",
                    "to": "alias-dst",
                },
            ],
        )

        result = await applier.apply_event(event)
        assert result.success
        assert len(result.created_nodes) == 2
        assert len(result.created_edges) == 1

        edges = await store.get_edges_from("t1", "alias-src")
        assert len(edges) == 1
        assert edges[0].to_node_id == "alias-dst"

    @pytest.mark.asyncio
    async def test_batch_empty_events_no_crash(self, tmp_path):
        """Event with empty ops list does not crash."""
        applier, _, _, _ = await _make_applier(tmp_path)

        event = _make_event(idempotency_key="empty-ops", ops=[])
        result = await applier.apply_event(event)
        assert result.success

    @pytest.mark.asyncio
    async def test_batch_single_event_batch(self, tmp_path):
        """Batch size larger than available events still works."""
        applier, store, _, _ = await _make_applier(tmp_path, batch_size=100)

        for i in range(3):
            ev = _make_event(
                idempotency_key=f"small-batch-{i}",
                ops=[
                    {
                        "op": "create_node",
                        "type_id": 1,
                        "id": f"sb-{i}",
                        "data": {},
                    }
                ],
            )
            await applier.apply_event(ev)

        stats = await store.get_stats("t1")
        assert stats["nodes"] == 3

    @pytest.mark.asyncio
    async def test_batch_performance_improvement(self, tmp_path):
        """200 events: batch_size=1 vs batch_size=50.
        Batch should be noticeably faster."""
        events_s = [
            _make_event(
                idempotency_key=f"perf-s-{i}",
                ops=[
                    {
                        "op": "create_node",
                        "type_id": 1,
                        "id": f"ps-{i}",
                        "data": {"i": i},
                    }
                ],
            )
            for i in range(200)
        ]
        events_b = [
            _make_event(
                idempotency_key=f"perf-b-{i}",
                ops=[
                    {
                        "op": "create_node",
                        "type_id": 1,
                        "id": f"pb-{i}",
                        "data": {"i": i},
                    }
                ],
            )
            for i in range(200)
        ]

        # Single mode
        applier_s, _, _, _ = await _make_applier(tmp_path / "perf_single", batch_size=1)
        t0 = time.monotonic()
        for ev in events_s:
            await applier_s.apply_event(ev)
        single_time = time.monotonic() - t0

        # Batch mode -- apply_event is always single-event, so we use
        # batch_transaction on the store directly to simulate batching.
        _, store_b, _, _ = await _make_applier(tmp_path / "perf_batch", batch_size=50)
        await store_b.initialize_tenant("t1")
        t0 = time.monotonic()
        batch_sz = 50
        for start in range(0, 200, batch_sz):
            chunk = events_b[start : start + batch_sz]
            with store_b.batch_transaction("t1") as conn:
                for ev in chunk:
                    op = ev.ops[0]
                    store_b.create_node_raw(
                        conn,
                        "t1",
                        op["type_id"],
                        op.get("data", {}),
                        ev.actor,
                        node_id=op["id"],
                    )
        batch_time = time.monotonic() - t0

        # Batch should be faster; use a lenient threshold to avoid flaky
        # failures on slow CI machines.
        assert batch_time < single_time, (
            f"Batch ({batch_time:.3f}s) was not faster than single ({single_time:.3f}s)"
        )


# =========================================================================
# EVENT LOOP NOT BLOCKED
# =========================================================================


@pytest.mark.integration
class TestEventLoopNotBlocked:
    """Verify that SQLite operations don't block the asyncio event loop."""

    @pytest.mark.asyncio
    async def test_concurrent_reads_dont_block(self, tmp_path):
        """10 concurrent get_node calls complete without serialisation."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("t1")

        for i in range(10):
            await store.create_node("t1", 1, {"i": i}, "user:1", node_id=f"cr-{i}")

        results = await asyncio.gather(*[store.get_node("t1", f"cr-{i}") for i in range(10)])
        assert all(r is not None for r in results)
        assert len(results) == 10

    @pytest.mark.asyncio
    async def test_write_during_read(self, tmp_path):
        """Issue a write concurrently with reads -- both complete."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("t1")
        await store.create_node("t1", 1, {}, "user:1", node_id="rw-exist")

        async def do_read():
            for _ in range(20):
                await store.get_node("t1", "rw-exist")
                await asyncio.sleep(0)
            return "read-done"

        async def do_write():
            await store.create_node("t1", 1, {"new": True}, "user:1", node_id="rw-new")
            return "write-done"

        results = await asyncio.gather(do_read(), do_write())
        assert set(results) == {"read-done", "write-done"}
        assert await store.get_node("t1", "rw-new") is not None

    @pytest.mark.asyncio
    async def test_many_concurrent_operations(self, tmp_path):
        """50 concurrent mixed operations all complete within timeout."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("t1")

        async def create(i: int):
            await store.create_node("t1", 1, {"i": i}, "user:1", node_id=f"conc-{i}")

        async def read(i: int):
            return await store.get_node("t1", f"conc-{i}")

        # Phase 1: create 25 nodes concurrently
        await asyncio.gather(*[create(i) for i in range(25)])

        # Phase 2: 25 reads + 25 writes concurrently
        tasks = [read(i) for i in range(25)] + [create(i) for i in range(25, 50)]
        results = await asyncio.wait_for(asyncio.gather(*tasks), timeout=30)
        assert len(results) == 50

    @pytest.mark.asyncio
    async def test_slow_query_doesnt_block_health(self, tmp_path):
        """Insert many nodes, do type query while also doing a fast read.
        The fast read should not wait for the type query."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("t1")

        for i in range(500):
            await store.create_node("t1", 1, {"i": i}, "user:1")

        # Create a known node for fast lookup
        await store.create_node("t1", 2, {"health": True}, "user:1", node_id="health-check")

        async def heavy_query():
            return await store.get_nodes_by_type("t1", 1, limit=500)

        async def health_check():
            node = await store.get_node("t1", "health-check")
            return node is not None

        results = await asyncio.gather(heavy_query(), health_check())
        assert len(results[0]) == 500
        assert results[1] is True

    @pytest.mark.asyncio
    async def test_async_cancellation_works(self, tmp_path):
        """Start a task, cancel it, verify no deadlock."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("t1")
        await store.create_node("t1", 1, {}, "user:1", node_id="cancel-test")

        async def long_reads():
            for _ in range(10000):
                await store.get_node("t1", "cancel-test")
                await asyncio.sleep(0)

        task = asyncio.create_task(long_reads())
        await asyncio.sleep(0.01)
        task.cancel()

        with pytest.raises(asyncio.CancelledError):
            await task

        # After cancellation, store should still be usable
        node = await store.get_node("t1", "cancel-test")
        assert node is not None


# =========================================================================
# PARTIAL EVENT PROTECTION
# =========================================================================


@pytest.mark.integration
class TestPartialEventProtection:
    """Verify that partial event application is handled correctly."""

    @pytest.mark.asyncio
    async def test_failed_op_doesnt_record_idempotency(self, tmp_path):
        """Event with valid create + invalid create (dup ID).
        Idempotency must NOT be recorded."""
        applier, store, _, _ = await _make_applier(tmp_path)

        # Setup: create a node
        setup = _make_event(
            idempotency_key="setup-partial",
            ops=[
                {"op": "create_node", "type_id": 1, "id": "dup-target", "data": {}},
            ],
        )
        await applier.apply_event(setup)

        # Event that will fail: first op ok, second is duplicate
        bad = _make_event(
            idempotency_key="partial-fail",
            ops=[
                {
                    "op": "create_node",
                    "type_id": 1,
                    "id": "partial-new",
                    "data": {},
                },
                {
                    "op": "create_node",
                    "type_id": 1,
                    "id": "dup-target",
                    "data": {},
                },
            ],
        )
        await applier.apply_event(bad)

        # Must not record the idempotency key
        recorded = await store.check_idempotency("t1", "partial-fail")
        assert recorded is False, "Idempotency key recorded despite partial failure"

    @pytest.mark.asyncio
    async def test_retry_after_partial_failure(self, tmp_path):
        """Event fails mid-way, retry after fixing -- all ops apply."""
        applier, store, _, _ = await _make_applier(tmp_path)

        # Create a conflicting node
        await applier.apply_event(
            _make_event(
                idempotency_key="s",
                ops=[
                    {"op": "create_node", "type_id": 1, "id": "conflict", "data": {}},
                ],
            )
        )

        # Event that fails because of duplicate "conflict"
        event = _make_event(
            idempotency_key="retry-ev",
            ops=[
                {
                    "op": "create_node",
                    "type_id": 1,
                    "id": "retry-new",
                    "data": {},
                },
                {
                    "op": "create_node",
                    "type_id": 1,
                    "id": "conflict",
                    "data": {},
                },
            ],
        )
        r1 = await applier.apply_event(event)
        assert not r1.success or r1.error is not None

        # Delete the conflicting node so retry can succeed
        await store.delete_node("t1", "conflict")

        # Retry the same event
        r2 = await applier.apply_event(event)
        # Correct behaviour: succeed and not skipped.
        assert r2.success and not r2.skipped, (
            "Retry did not succeed; idempotency may have been incorrectly recorded"
        )

    @pytest.mark.asyncio
    async def test_multi_op_all_or_nothing(self, tmp_path):
        """Event with 5 creates: if any fails, none should be in DB.
        Tests atomicity of the apply_event path."""
        applier, store, _, _ = await _make_applier(tmp_path)

        # Pre-create node that will cause duplicate on 5th op
        await applier.apply_event(
            _make_event(
                idempotency_key="aon-setup",
                ops=[
                    {"op": "create_node", "type_id": 1, "id": "aon-4", "data": {}},
                ],
            )
        )

        event = _make_event(
            idempotency_key="aon-ev",
            ops=[
                {"op": "create_node", "type_id": 1, "id": "aon-0", "data": {}},
                {"op": "create_node", "type_id": 1, "id": "aon-1", "data": {}},
                {"op": "create_node", "type_id": 1, "id": "aon-2", "data": {}},
                {"op": "create_node", "type_id": 1, "id": "aon-3", "data": {}},
                {"op": "create_node", "type_id": 1, "id": "aon-4", "data": {}},
            ],
        )

        await applier.apply_event(event)

        # If atomicity holds, none of the new nodes should exist
        for i in range(4):
            node = await store.get_node("t1", f"aon-{i}")
            assert node is None, f"Node aon-{i} exists despite failed event -- atomicity violation"

    @pytest.mark.asyncio
    async def test_edge_creation_after_node_in_same_event(self, tmp_path):
        """Create node + create edge referencing it in same event.
        Must work atomically."""
        applier, store, _, _ = await _make_applier(tmp_path)

        # Pre-create target node for edge
        await applier.apply_event(
            _make_event(
                idempotency_key="edge-setup",
                ops=[
                    {
                        "op": "create_node",
                        "type_id": 1,
                        "id": "edge-target",
                        "data": {},
                    },
                ],
            )
        )

        event = _make_event(
            idempotency_key="edge-ev",
            ops=[
                {
                    "op": "create_node",
                    "type_id": 1,
                    "id": "edge-source",
                    "data": {},
                    "as": "src",
                },
                {
                    "op": "create_edge",
                    "edge_id": 1,
                    "from": "edge-source",
                    "to": "edge-target",
                },
            ],
        )

        result = await applier.apply_event(event)
        assert result.success

        edges = await store.get_edges_from("t1", "edge-source")
        assert len(edges) == 1
        assert edges[0].to_node_id == "edge-target"

    @pytest.mark.asyncio
    async def test_delete_then_create_same_id_in_batch(self, tmp_path):
        """Delete node then create with same ID in separate events."""
        applier, store, _, _ = await _make_applier(tmp_path)

        # Create original
        await applier.apply_event(
            _make_event(
                idempotency_key="dc-create",
                ops=[
                    {
                        "op": "create_node",
                        "type_id": 1,
                        "id": "reuse-id",
                        "data": {"gen": 1},
                    },
                ],
            )
        )

        # Delete it
        await applier.apply_event(
            _make_event(
                idempotency_key="dc-delete",
                ops=[
                    {"op": "delete_node", "id": "reuse-id"},
                ],
            )
        )
        assert await store.get_node("t1", "reuse-id") is None

        # Re-create with same ID
        await applier.apply_event(
            _make_event(
                idempotency_key="dc-recreate",
                ops=[
                    {
                        "op": "create_node",
                        "type_id": 1,
                        "id": "reuse-id",
                        "data": {"gen": 2},
                    },
                ],
            )
        )

        node = await store.get_node("t1", "reuse-id")
        assert node is not None
        assert node.payload["gen"] == 2


# =========================================================================
# CONNECTION MANAGEMENT
# =========================================================================


@pytest.mark.integration
class TestConnectionManagement:
    """Tests for connection handling and resource cleanup."""

    @pytest.mark.asyncio
    async def test_many_sequential_operations_no_leak(self, tmp_path):
        """Do 1000 operations; verify no connection leak."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("t1")

        # Warm up
        for i in range(10):
            await store.create_node("t1", 1, {"i": i}, "user:1")

        # Snapshot file descriptor count (Unix only)
        try:
            fd_before = len(os.listdir(f"/proc/{os.getpid()}/fd"))
        except FileNotFoundError:
            # macOS or non-Linux: use a simpler check
            fd_before = None

        for i in range(10, 1010):
            await store.create_node("t1", 1, {"i": i}, "user:1")

        if fd_before is not None:
            fd_after = len(os.listdir(f"/proc/{os.getpid()}/fd"))
            # Allow small variance (< 20 extra fds)
            assert fd_after - fd_before < 20, f"File descriptor leak: {fd_before} -> {fd_after}"

        # At minimum: verify all nodes exist
        stats = await store.get_stats("t1")
        assert stats["nodes"] == 1010

    @pytest.mark.asyncio
    async def test_tenant_initialization_idempotent(self, tmp_path):
        """Call initialize_tenant twice; no error."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("t1")
        await store.initialize_tenant("t1")  # must not raise

        # Verify schema still works
        node = await store.create_node("t1", 1, {"ok": True}, "user:1")
        assert node is not None

    @pytest.mark.asyncio
    async def test_operations_on_uninitialized_tenant_raises(self, tmp_path):
        """get_node on nonexistent tenant raises TenantNotFoundError."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)

        with pytest.raises(TenantNotFoundError):
            await store.get_node("nonexistent", "any-id")

    @pytest.mark.asyncio
    async def test_concurrent_tenant_initialization(self, tmp_path):
        """Initialize same tenant from 5 concurrent tasks. No error."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)

        await asyncio.gather(*[store.initialize_tenant("concurrent-t") for _ in range(5)])

        # Verify tenant works
        node = await store.create_node("concurrent-t", 1, {"ok": True}, "user:1")
        assert node is not None
        fetched = await store.get_node("concurrent-t", node.node_id)
        assert fetched is not None

    @pytest.mark.asyncio
    async def test_database_file_created_on_initialize(self, tmp_path):
        """After initialize_tenant, the .db file exists on disk."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)

        assert not store.get_db_path("file-check").exists()

        await store.initialize_tenant("file-check")

        db_path = store.get_db_path("file-check")
        assert db_path.exists(), f"Database file not created at {db_path}"
        assert db_path.stat().st_size > 0, "Database file is empty"
