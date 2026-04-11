"""
Write-path JSON serialization benchmark.

Measures the impact of eliminating double json.dumps in create_node_raw
and _sync_create_node. Before this optimization, payload and ACL were
serialized once for the INSERT and a second time in the Node constructor.
Now they are serialized once and the pre-built string is passed via
payload_json=/acl_json= keyword args.

Run:
  pytest tests/benchmarks/bench_write_path_json.py -v -s --benchmark-disable
"""

from __future__ import annotations

import json
import time

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore, Node

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

SMALL_PAYLOAD = {"name": "Alice", "email": "alice@example.com"}
LARGE_PAYLOAD = {
    "name": "Alice",
    "email": "alice@example.com",
    "bio": "x" * 4000,
    "tags": [f"tag-{i}" for i in range(50)],
    "metadata": {f"key-{i}": f"val-{i}" for i in range(30)},
}
ACL_3 = [
    {"principal": "user:alice", "permission": "read"},
    {"principal": "user:bob", "permission": "write"},
    {"principal": "group:admins", "permission": "admin"},
]


def _double_serialize(payload: dict, acl: list) -> tuple[str, str, Node]:
    """Simulate the OLD code path: serialize twice (INSERT + Node ctor)."""
    payload_for_insert = json.dumps(payload)
    acl_for_insert = json.dumps(acl)
    # Node ctor does its own json.dumps internally:
    node = Node(
        tenant_id="t",
        node_id="n",
        type_id=1,
        payload=payload,
        created_at=0,
        updated_at=0,
        owner_actor="u",
        acl=acl,
    )
    return payload_for_insert, acl_for_insert, node


def _single_serialize(payload: dict, acl: list) -> tuple[str, str, Node]:
    """Simulate the NEW code path: serialize once, pass pre-built strings."""
    payload_str = json.dumps(payload)
    acl_str = json.dumps(acl)
    node = Node(
        tenant_id="t",
        node_id="n",
        type_id=1,
        created_at=0,
        updated_at=0,
        owner_actor="u",
        payload_json=payload_str,
        acl_json=acl_str,
    )
    return payload_str, acl_str, node


# ---------------------------------------------------------------------------
# Micro-benchmarks: pure serialization overhead (no SQLite)
# ---------------------------------------------------------------------------


class TestWritePathJsonMicro:
    """Micro-benchmark: json.dumps double vs single serialization."""

    N = 10_000

    def test_small_payload_double(self, benchmark):
        """OLD path: small payload serialized twice."""

        def run():
            for _ in range(self.N):
                _double_serialize(SMALL_PAYLOAD, ACL_3)

        benchmark(run)

    def test_small_payload_single(self, benchmark):
        """NEW path: small payload serialized once."""

        def run():
            for _ in range(self.N):
                _single_serialize(SMALL_PAYLOAD, ACL_3)

        benchmark(run)

    def test_large_payload_double(self, benchmark):
        """OLD path: large (~5 KB) payload serialized twice."""

        def run():
            for _ in range(self.N):
                _double_serialize(LARGE_PAYLOAD, ACL_3)

        benchmark(run)

    def test_large_payload_single(self, benchmark):
        """NEW path: large (~5 KB) payload serialized once."""

        def run():
            for _ in range(self.N):
                _single_serialize(LARGE_PAYLOAD, ACL_3)

        benchmark(run)


# ---------------------------------------------------------------------------
# End-to-end: create_node_raw through real SQLite
# ---------------------------------------------------------------------------


class TestWritePathEndToEnd:
    """End-to-end write path benchmark using create_node_raw."""

    NUM_NODES = 500

    @pytest.mark.asyncio
    async def test_create_node_raw_batch(self, tmp_path):
        """Benchmark create_node_raw batch (current optimized code)."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("bench")

        start = time.perf_counter()
        with store.batch_transaction("bench") as conn:
            for i in range(self.NUM_NODES):
                store.create_node_raw(
                    conn,
                    tenant_id="bench",
                    type_id=1,
                    payload={"name": f"Node {i}", "body": "x" * 200},
                    owner_actor="user:test",
                    node_id=f"node-{i}",
                    acl=ACL_3,
                    created_at=1000,
                )
        elapsed = time.perf_counter() - start

        # Verify
        node = await store.get_node("bench", "node-0")
        assert node is not None
        assert node.payload_json  # pre-built string from create_node_raw

        throughput = self.NUM_NODES / elapsed
        per_node_us = (elapsed / self.NUM_NODES) * 1e6

        print(
            f"\n  create_node_raw x{self.NUM_NODES}: "
            f"{throughput:,.0f} nodes/sec  "
            f"({per_node_us:.1f} us/node, {elapsed:.3f}s total)"
        )

    @pytest.mark.asyncio
    async def test_create_node_async(self, tmp_path):
        """Benchmark create_node (async, individual transactions)."""
        store = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
        await store.initialize_tenant("bench")

        start = time.perf_counter()
        for i in range(self.NUM_NODES):
            await store.create_node(
                tenant_id="bench",
                type_id=1,
                payload={"name": f"Node {i}", "body": "x" * 200},
                owner_actor="user:test",
                node_id=f"anode-{i}",
                acl=ACL_3,
                created_at=1000,
            )
        elapsed = time.perf_counter() - start

        node = await store.get_node("bench", "anode-0")
        assert node is not None

        throughput = self.NUM_NODES / elapsed
        per_node_us = (elapsed / self.NUM_NODES) * 1e6

        print(
            f"\n  create_node (async) x{self.NUM_NODES}: "
            f"{throughput:,.0f} nodes/sec  "
            f"({per_node_us:.1f} us/node, {elapsed:.3f}s total)"
        )


# ---------------------------------------------------------------------------
# Comparison: print summary
# ---------------------------------------------------------------------------


class TestWritePathJsonComparison:
    """Print a direct comparison of old vs new serialization cost."""

    @staticmethod
    def test_comparison():
        """Direct timing comparison of double vs single serialize."""
        import timeit

        n = 50_000

        old_time = timeit.timeit(
            lambda: _double_serialize(LARGE_PAYLOAD, ACL_3),
            number=n,
        )
        new_time = timeit.timeit(
            lambda: _single_serialize(LARGE_PAYLOAD, ACL_3),
            number=n,
        )

        saved_pct = ((old_time - new_time) / old_time) * 100
        saved_per_call_us = ((old_time - new_time) / n) * 1e6

        print(f"\n{'=' * 60}")
        print("WRITE PATH JSON SERIALIZATION (large payload, {n} calls)")
        print(f"{'=' * 60}")
        print(f"  Old (double dumps):  {old_time:.3f}s  ({old_time / n * 1e6:.1f} us/call)")
        print(f"  New (single dumps):  {new_time:.3f}s  ({new_time / n * 1e6:.1f} us/call)")
        print(f"  Saved: {saved_pct:.1f}%  ({saved_per_call_us:.1f} us/call)")
        print(f"{'=' * 60}")

        assert new_time < old_time, "Single serialize should be faster"
