"""
Batch applier benchmarks.

Compares throughput of the applier with different batch sizes
using InMemoryWalStream and real SQLite.

Run:
  pytest tests/benchmarks/bench_batch_applier.py -v -s --benchmark-disable
"""

from __future__ import annotations

import json
import time

import pytest

from dbaas.entdb_server.apply.applier import TransactionEvent
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.wal.memory import InMemoryWalStream


def _make_event(tenant_id: str, i: int, payload_size: int = 200) -> bytes:
    """Create a realistic WAL event as bytes."""
    event = {
        "tenant_id": tenant_id,
        "actor": f"user:{i % 10}",
        "idempotency_key": f"event-{i}",
        "schema_fingerprint": None,
        "ts_ms": int(time.time() * 1000),
        "ops": [
            {
                "op": "create_node",
                "type_id": 1,
                "id": f"node-{i}",
                "data": {"name": f"Item {i}", "body": "x" * payload_size},
            }
        ],
    }
    return json.dumps(event).encode()


async def _run_applier_benchmark(
    data_dir: str,
    num_events: int,
    batch_size: int,
    poll_timeout_ms: int = 50,
) -> dict:
    """Run the applier with given batch_size and measure throughput.

    Returns dict with timing results.
    """
    wal = InMemoryWalStream(num_partitions=1)
    await wal.connect()

    store = CanonicalStore(data_dir=data_dir, wal_mode=True)
    await store.initialize_tenant("bench")

    # Pre-load all events into the WAL
    topic = "bench-wal"
    for i in range(num_events):
        await wal.append(topic, "bench", _make_event("bench", i))

    # Benchmark: poll_batch + apply with batch_transaction
    # batch_size=1: each event gets its own BEGIN/COMMIT (baseline)
    # batch_size>1: all events in one BEGIN/COMMIT (amortized fsync)
    applied = 0
    start = time.perf_counter()

    while applied < num_events:
        records = await wal.poll_batch(
            topic=topic,
            group_id="bench-applier",
            max_records=batch_size,
            timeout_ms=poll_timeout_ms,
        )
        if not records:
            continue

        # Parse all events in the batch
        events = []
        for record in records:
            data = record.value_json()
            events.append(TransactionEvent.from_dict(data, record.position))

        if batch_size <= 1:
            # Single-event mode: each gets its own transaction
            for event in events:
                await store.create_node(
                    tenant_id=event.tenant_id,
                    type_id=event.ops[0]["type_id"],
                    payload=event.ops[0].get("data", {}),
                    owner_actor=event.actor,
                    node_id=event.ops[0].get("id"),
                    created_at=event.ts_ms,
                )
                applied += 1
        else:
            # Batch mode: one transaction for all events
            with store.batch_transaction("bench") as conn:
                for event in events:
                    store.create_node_raw(
                        conn,
                        tenant_id=event.tenant_id,
                        type_id=event.ops[0]["type_id"],
                        payload=event.ops[0].get("data", {}),
                        owner_actor=event.actor,
                        node_id=event.ops[0].get("id"),
                        created_at=event.ts_ms,
                    )
                    applied += 1

        # Commit last record in batch
        await wal.commit(records[-1])

    elapsed = time.perf_counter() - start

    # Verify
    node = await store.get_node("bench", "node-0")
    assert node is not None, "First node should exist"
    last_node = await store.get_node("bench", f"node-{num_events - 1}")
    assert last_node is not None, f"Last node (node-{num_events - 1}) should exist"

    await wal.close()

    return {
        "batch_size": batch_size,
        "num_events": num_events,
        "elapsed_sec": round(elapsed, 3),
        "throughput": round(num_events / elapsed, 1),
        "avg_latency_ms": round((elapsed / num_events) * 1000, 3),
    }


class TestBatchApplierThroughput:
    """Compare applier throughput across different batch sizes."""

    NUM_EVENTS = 500

    @pytest.mark.asyncio
    async def test_batch_size_1(self, tmp_path):
        """Baseline: no batching (batch_size=1)."""
        result = await _run_applier_benchmark(str(tmp_path), self.NUM_EVENTS, batch_size=1)
        print(
            f"\n  batch_size=1:   {result['throughput']:>8.1f} events/sec  "
            f"({result['elapsed_sec']:.3f}s, {result['avg_latency_ms']:.3f}ms/event)"
        )

    @pytest.mark.asyncio
    async def test_batch_size_5(self, tmp_path):
        """Batch size 5."""
        result = await _run_applier_benchmark(str(tmp_path), self.NUM_EVENTS, batch_size=5)
        print(
            f"\n  batch_size=5:   {result['throughput']:>8.1f} events/sec  "
            f"({result['elapsed_sec']:.3f}s, {result['avg_latency_ms']:.3f}ms/event)"
        )

    @pytest.mark.asyncio
    async def test_batch_size_10(self, tmp_path):
        """Batch size 10."""
        result = await _run_applier_benchmark(str(tmp_path), self.NUM_EVENTS, batch_size=10)
        print(
            f"\n  batch_size=10:  {result['throughput']:>8.1f} events/sec  "
            f"({result['elapsed_sec']:.3f}s, {result['avg_latency_ms']:.3f}ms/event)"
        )

    @pytest.mark.asyncio
    async def test_batch_size_20(self, tmp_path):
        """Batch size 20."""
        result = await _run_applier_benchmark(str(tmp_path), self.NUM_EVENTS, batch_size=20)
        print(
            f"\n  batch_size=20:  {result['throughput']:>8.1f} events/sec  "
            f"({result['elapsed_sec']:.3f}s, {result['avg_latency_ms']:.3f}ms/event)"
        )

    @pytest.mark.asyncio
    async def test_batch_size_50(self, tmp_path):
        """Batch size 50."""
        result = await _run_applier_benchmark(str(tmp_path), self.NUM_EVENTS, batch_size=50)
        print(
            f"\n  batch_size=50:  {result['throughput']:>8.1f} events/sec  "
            f"({result['elapsed_sec']:.3f}s, {result['avg_latency_ms']:.3f}ms/event)"
        )

    @pytest.mark.asyncio
    async def test_batch_size_100(self, tmp_path):
        """Batch size 100."""
        result = await _run_applier_benchmark(str(tmp_path), self.NUM_EVENTS, batch_size=100)
        print(
            f"\n  batch_size=100: {result['throughput']:>8.1f} events/sec  "
            f"({result['elapsed_sec']:.3f}s, {result['avg_latency_ms']:.3f}ms/event)"
        )


class TestBatchApplierComparison:
    """Full comparison table across batch sizes."""

    @pytest.mark.asyncio
    async def test_full_comparison(self, tmp_path):
        """Run all batch sizes and print comparison table."""
        batch_sizes = [1, 5, 10, 20, 50, 100]
        num_events = 500
        results = []

        for bs in batch_sizes:
            # Each batch size gets its own directory to avoid cross-contamination
            d = tmp_path / f"bs-{bs}"
            d.mkdir()
            r = await _run_applier_benchmark(str(d), num_events, batch_size=bs)
            results.append(r)

        baseline = results[0]["throughput"]

        print(f"\n{'=' * 72}")
        print(f"BATCH APPLIER THROUGHPUT COMPARISON ({num_events} events)")
        print(f"{'=' * 72}")
        print(f"{'Batch Size':>12} {'Throughput':>14} {'Latency':>12} {'Speedup':>10} {'Time':>10}")
        print(f"{'-' * 72}")
        for r in results:
            speedup = r["throughput"] / baseline
            print(
                f"{r['batch_size']:>12} "
                f"{r['throughput']:>10.1f}/sec "
                f"{r['avg_latency_ms']:>9.3f}ms "
                f"{speedup:>9.2f}x "
                f"{r['elapsed_sec']:>9.3f}s"
            )
        print(f"{'=' * 72}")
