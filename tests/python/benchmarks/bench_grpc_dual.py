# SPDX-License-Identifier: AGPL-3.0-only
"""
Dual-backend gRPC wire-level benchmarks (Phase 4B of EPIC #407).

These benchmarks intentionally exercise the gRPC wire path so the same
test code can measure either the Python in-process servicer or the Go
``entdb-server`` subprocess. Switch backends with the
``ENTDB_SERVER_TARGET`` env var:

    python -m pytest tests/python/benchmarks/bench_grpc_dual.py --benchmark-only
    ENTDB_SERVER_TARGET=go python -m pytest tests/python/benchmarks/bench_grpc_dual.py --benchmark-only

All cases are marked ``cross_backend`` so the benchmark conftest knows
to run them against both targets (rather than skipping them under the
Go target like the Python-internals benches).

Implementation notes:

* Uses the **synchronous** gRPC stub (``grpc.insecure_channel`` +
  ``EntDBServiceStub`` from ``entdb_pb2_grpc``) rather than ``grpc.aio``
  so pytest-benchmark can wrap each call directly without re-entering an
  event loop. The Python ``live_server`` fixture runs the server on
  pytest-asyncio's loop; the Go ``live_server`` is a subprocess. Both
  speak plain HTTP/2 gRPC, so a sync client is fine for both.
* The channel is opened once per test and reused across rounds.
* Read-path RPCs (``GetNode``, ``GetNodes``, ``QueryNodes``, ``Health``)
  measure mean per-call wall-time. Writes (``ExecuteAtomic`` with
  ``wait_applied=True``) measure the full WAL append + applier ack
  round-trip.

Caveats:

* The in-memory WAL is used on both backends; Kafka/Redpanda topology
  is intentionally not in the loop. Comparing against a Kafka deployment
  is out of scope for Phase 4B.
* The Python backend runs in-process inside the test runner, sharing
  CPU with the client. The Go backend runs as a subprocess. Resource
  contention is therefore subtly different — small differences in
  wall-time should not be over-interpreted.
"""

from __future__ import annotations

import uuid

import grpc
import pytest
from google.protobuf.struct_pb2 import Struct

# Re-use the contract suite's seed constants so the in-process Python
# fixture and the Go subprocess both have a seeded node we can target.
from tests.python.integration.conftest import ALICE, SEED_NODE_ID, TENANT

# Every bench in this module runs against both Python and Go.
pytestmark = pytest.mark.cross_backend


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _ctx():
    from entdb_server.api.generated import entdb_pb2 as pb

    return pb.RequestContext(tenant_id=TENANT, actor=ALICE)


@pytest.fixture
def stub(grpc_endpoint):
    """Sync EntDBService stub bound to the live server."""
    from entdb_server.api.generated.entdb_pb2_grpc import EntDBServiceStub

    channel = grpc.insecure_channel(grpc_endpoint)
    # Wait for the channel to be ready so the first benchmark call
    # doesn't pay connection-establishment cost. 5s is generous; both
    # backends should be up well before this fixture runs.
    grpc.channel_ready_future(channel).result(timeout=5.0)
    try:
        yield EntDBServiceStub(channel)
    finally:
        channel.close()


# ---------------------------------------------------------------------------
# Read-path benches
# ---------------------------------------------------------------------------


class TestReadPath:
    """Wire-level read benchmarks against the seeded node."""

    def test_get_node_p50(self, benchmark, stub):
        from entdb_server.api.generated import entdb_pb2 as pb

        req = pb.GetNodeRequest(context=_ctx(), type_id=1, node_id=SEED_NODE_ID)

        def _call():
            resp = stub.GetNode(req)
            assert resp.found
            return resp

        benchmark(_call)

    def test_get_nodes_batch10(self, benchmark, stub):
        """Batched lookup; mostly-miss case mirrors realistic traffic."""
        from entdb_server.api.generated import entdb_pb2 as pb

        ids = [SEED_NODE_ID] + [f"missing-{i}" for i in range(9)]
        req = pb.GetNodesRequest(context=_ctx(), type_id=1, node_ids=ids)

        def _call():
            resp = stub.GetNodes(req)
            # Server returns the found nodes in `nodes` and the rest in
            # `missing_ids`. We seeded one node so at least one is found.
            assert len(resp.nodes) >= 1 or len(resp.missing_ids) >= 9
            return resp

        benchmark(_call)

    def test_query_nodes(self, benchmark, stub):
        from entdb_server.api.generated import entdb_pb2 as pb

        req = pb.QueryNodesRequest(context=_ctx(), type_id=1, limit=10)

        def _call():
            resp = stub.QueryNodes(req)
            assert any(n.node_id == SEED_NODE_ID for n in resp.nodes)
            return resp

        benchmark(_call)

    def test_health(self, benchmark, stub):
        """Tiny RPC — measures the channel + grpc dispatch overhead."""
        from entdb_server.api.generated import entdb_pb2 as pb

        req = pb.HealthRequest()

        def _call():
            try:
                return stub.Health(req)
            except grpc.RpcError as exc:
                # Some Go-wave revisions still return UNIMPLEMENTED for
                # Health; both code paths complete an RTT so we keep
                # benchmarking.
                if exc.code() != grpc.StatusCode.UNIMPLEMENTED:
                    raise

        benchmark(_call)


# ---------------------------------------------------------------------------
# Write-path benches (WAL append + applier ack)
# ---------------------------------------------------------------------------


class TestWritePath:
    """Wire-level write benchmarks; each call appends a unique node."""

    def test_create_node_wait_applied(self, benchmark, stub):
        """Full write round-trip: gRPC -> WAL -> applier -> ack."""
        from entdb_server.api.generated import entdb_pb2 as pb

        def _call():
            node_id = f"bench-{uuid.uuid4().hex[:12]}"
            data = Struct()
            data.update({"1": f"{node_id}@example.com", "2": node_id})
            req = pb.ExecuteAtomicRequest(
                context=_ctx(),
                idempotency_key=f"bench-write-{node_id}",
                operations=[
                    pb.Operation(create_node=pb.CreateNodeOp(type_id=1, id=node_id, data=data))
                ],
                wait_applied=True,
                wait_timeout_ms=5000,
            )
            resp = stub.ExecuteAtomic(req)
            assert resp.success, f"write failed: {resp.error}"
            return resp

        benchmark(_call)
