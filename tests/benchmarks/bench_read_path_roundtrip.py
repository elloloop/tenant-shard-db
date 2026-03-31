"""
Benchmark: Read-path JSON round-trip elimination.

Measures the cost saved by passing raw JSON strings through the read path
instead of json.loads() in canonical_store + json.dumps() in grpc_server.

This validates performance cycle #3: eliminating the
  SQLite TEXT -> json.loads -> dict -> json.dumps -> proto string
round-trip on every node read.

Run:
  pytest tests/benchmarks/bench_read_path_roundtrip.py -v --benchmark-columns=mean,stddev,rounds
"""

from __future__ import annotations

import asyncio
import json

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore, Node


@pytest.fixture
def store(tmp_path):
    return CanonicalStore(data_dir=str(tmp_path), wal_mode=True)


@pytest.fixture
def populated_store(store):
    """Store with 200 nodes, payloads of varying size."""
    loop = asyncio.get_event_loop()
    loop.run_until_complete(store.initialize_tenant("bench"))
    for i in range(200):
        loop.run_until_complete(
            store.create_node(
                tenant_id="bench",
                type_id=1,
                payload={
                    "name": f"User {i}",
                    "email": f"user{i}@example.com",
                    "bio": f"A moderately long biography text for user {i}. " * 5,
                    "settings": {"theme": "dark", "lang": "en", "tz": "UTC"},
                    "tags": ["alpha", "beta", "gamma"],
                    "score": i * 1.5,
                },
                owner_actor=f"user:{i}",
                node_id=f"node-{i:04d}",
                acl=[
                    {"principal": f"user:{i}", "permission": "owner"},
                    {"principal": "role:admin", "permission": "read"},
                    {"principal": "tenant:*", "permission": "read"},
                ],
            )
        )
    return store


class TestReadPathRoundTrip:
    """Benchmark read-path with lazy JSON passthrough vs old json.loads+json.dumps."""

    def test_bench_get_node_read_path(self, benchmark, populated_store):
        """Single get_node + simulated gRPC proto construction."""

        def run():
            node = asyncio.get_event_loop().run_until_complete(
                populated_store.get_node("bench", "node-0050")
            )
            # Simulate what gRPC server does: access payload_json and acl_json
            _ = node.payload_json
            _ = node.acl_json
            return node

        node = benchmark(run)
        assert node is not None

    def test_bench_get_node_old_style_roundtrip(self, benchmark, populated_store):
        """Simulate old behavior: force json.loads then json.dumps."""

        def run():
            node = asyncio.get_event_loop().run_until_complete(
                populated_store.get_node("bench", "node-0050")
            )
            # Old path: parse then re-serialize (what we eliminated)
            _ = json.dumps(node.payload)
            _ = json.dumps(node.acl)
            return node

        node = benchmark(run)
        assert node is not None

    def test_bench_query_100_nodes_read_path(self, benchmark, populated_store):
        """Bulk query 100 nodes + simulated gRPC serialization (new path)."""

        def run():
            nodes = asyncio.get_event_loop().run_until_complete(
                populated_store.get_nodes_by_type("bench", type_id=1, limit=100)
            )
            # Simulate gRPC layer: access raw JSON strings
            for n in nodes:
                _ = n.payload_json
                _ = n.acl_json
            return nodes

        nodes = benchmark(run)
        assert len(nodes) == 100

    def test_bench_query_100_nodes_old_style_roundtrip(self, benchmark, populated_store):
        """Bulk query 100 nodes + old-style json.dumps (simulated old path)."""

        def run():
            nodes = asyncio.get_event_loop().run_until_complete(
                populated_store.get_nodes_by_type("bench", type_id=1, limit=100)
            )
            # Old path: force parse + re-serialize for each node
            for n in nodes:
                _ = json.dumps(n.payload)
                _ = json.dumps(n.acl)
            return nodes

        nodes = benchmark(run)
        assert len(nodes) == 100

    def test_bench_lazy_payload_not_parsed_unless_accessed(self, benchmark, populated_store):
        """Verify that payload is NOT parsed when only payload_json is accessed."""

        def run():
            nodes = asyncio.get_event_loop().run_until_complete(
                populated_store.get_nodes_by_type("bench", type_id=1, limit=100)
            )
            for n in nodes:
                # Only access raw string -- should NOT trigger json.loads
                _ = n.payload_json
                _ = n.acl_json
                # Verify _payload_parsed is still sentinel (not parsed)
                assert n._payload_parsed is Node._SENTINEL
                assert n._acl_parsed is Node._SENTINEL
            return nodes

        nodes = benchmark(run)
        assert len(nodes) == 100
