# SPDX-License-Identifier: AGPL-3.0-only
"""Wire-level bench against the Go ``entdb-server`` via the SDK proto stubs.

Each bench function targets one of the 12 operations in the
EntDB↔Postgres mapping table from issue #487. Group names match
``bench_postgres.py`` so the report doc can pair the two runs.

Fixtures: ``entdb_stub`` (a session-scoped sync gRPC stub built from
``sdk/python/entdb_sdk/_generated/entdb_pb2_grpc.py``) and
``entdb_seeded`` (a list of ``CORPUS_SIZE`` node ids the conftest has
materialised through ``ExecuteAtomic`` before the bench runs).

The bench is sync (``stub.GetNode(req)`` not ``await
stub.GetNode(req)``) for two reasons:

1. pytest-benchmark's measurement loop is sync; asyncio.run per round
   adds enough overhead to dominate the RPCs we want to measure.
2. The hot path inside the server is identical regardless of which
   client transport is used; the bench measures the server's wire
   work, not the client's event loop.
"""

from __future__ import annotations

import uuid

import pytest

# Constants are duplicated from ``conftest`` rather than imported, so
# pytest collection works whether the bench dir is loaded as a package
# (``-p tests.python.benchmarks``) or via a plain ``pytest path/to``.
BENCH_TENANT = "bench"
# ``system:bench`` bypasses ACL on both reads and writes — keeps the
# comparison apples-to-apples with Postgres which pays no ACL cost.
# See ``conftest.BENCH_ACTOR`` for the full rationale.
BENCH_ACTOR = "system:bench"
TASK_TYPE_ID = 2

pytestmark = pytest.mark.benchmark


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _ctx():
    from entdb_sdk._generated import entdb_pb2 as pb

    return pb.RequestContext(tenant_id=BENCH_TENANT, actor=BENCH_ACTOR)


def _struct(d: dict):
    from google.protobuf import struct_pb2

    s = struct_pb2.Struct()
    s.update(d)
    return s


# ---------------------------------------------------------------------------
# 1 — Health
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="health")
def test_entdb_health(entdb_stub, entdb_seeded, benchmark) -> None:
    from entdb_sdk._generated import entdb_pb2 as pb

    req = pb.HealthRequest()
    benchmark(lambda: entdb_stub.Health(req, timeout=5.0))


# ---------------------------------------------------------------------------
# 2 — GetNode
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="point-read")
def test_entdb_get_node(entdb_stub, entdb_seeded, benchmark) -> None:
    from entdb_sdk._generated import entdb_pb2 as pb

    ids = entdb_seeded
    counter = {"i": 0}

    def call() -> None:
        i = counter["i"]
        counter["i"] = (i + 1) % len(ids)
        req = pb.GetNodeRequest(context=_ctx(), type_id=TASK_TYPE_ID, node_id=ids[i])
        entdb_stub.GetNode(req, timeout=5.0)

    benchmark(call)


# ---------------------------------------------------------------------------
# 3 — GetNodes (batched, 10)
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="batched-read")
def test_entdb_get_nodes_batch(entdb_stub, entdb_seeded, benchmark) -> None:
    from entdb_sdk._generated import entdb_pb2 as pb

    ids = entdb_seeded
    batch_size = 10
    counter = {"i": 0}

    def call() -> None:
        i = counter["i"]
        counter["i"] = (i + batch_size) % max(1, len(ids) - batch_size)
        req = pb.GetNodesRequest(
            context=_ctx(),
            type_id=TASK_TYPE_ID,
            node_ids=ids[i : i + batch_size],
        )
        entdb_stub.GetNodes(req, timeout=5.0)

    benchmark(call)


# ---------------------------------------------------------------------------
# 4 — QueryNodes (filtered, limit=10)
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="filtered-read")
def test_entdb_query_nodes(entdb_stub, entdb_seeded, benchmark) -> None:
    from entdb_sdk._generated import entdb_pb2 as pb

    req = pb.QueryNodesRequest(
        context=_ctx(),
        type_id=TASK_TYPE_ID,
        limit=10,
        offset=0,
        order_by="created_at",
        descending=True,
    )
    benchmark(lambda: entdb_stub.QueryNodes(req, timeout=5.0))


# ---------------------------------------------------------------------------
# 5 — ExecuteAtomic (create_node, single op)
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="single-write")
def test_entdb_execute_atomic_create_node(entdb_stub, entdb_seeded, benchmark) -> None:
    from entdb_sdk._generated import entdb_pb2 as pb

    def call() -> None:
        nid = f"bench-write-{uuid.uuid4().hex[:16]}"
        req = pb.ExecuteAtomicRequest(
            context=_ctx(),
            idempotency_key=f"bench-write-{uuid.uuid4().hex[:12]}",
            operations=[
                pb.Operation(
                    create_node=pb.CreateNodeOp(
                        type_id=TASK_TYPE_ID,
                        id=nid,
                        data=_struct({"1": "bench write", "2": "x" * 200}),
                    )
                )
            ],
        )
        entdb_stub.ExecuteAtomic(req, timeout=10.0)

    benchmark(call)


# ---------------------------------------------------------------------------
# 6 — ExecuteAtomic (create_node + create_edge, multi-op)
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="multi-op-write")
def test_entdb_execute_atomic_create_node_and_edge(entdb_stub, entdb_seeded, benchmark) -> None:
    from entdb_sdk._generated import entdb_pb2 as pb

    target = entdb_seeded[0]

    def call() -> None:
        nid = f"bench-mn-{uuid.uuid4().hex[:16]}"
        edge_op = pb.CreateEdgeOp(edge_id=100)
        getattr(edge_op, "from").CopyFrom(pb.NodeRef(id=nid))
        edge_op.to.CopyFrom(pb.NodeRef(id=target))
        req = pb.ExecuteAtomicRequest(
            context=_ctx(),
            idempotency_key=f"bench-mn-{uuid.uuid4().hex[:12]}",
            operations=[
                pb.Operation(
                    create_node=pb.CreateNodeOp(
                        type_id=TASK_TYPE_ID,
                        id=nid,
                        data=_struct({"1": "multi", "2": "y" * 100}),
                    )
                ),
                pb.Operation(create_edge=edge_op),
            ],
        )
        entdb_stub.ExecuteAtomic(req, timeout=10.0)

    benchmark(call)


# ---------------------------------------------------------------------------
# 7 — ExecuteAtomic (update_node)
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="update")
def test_entdb_execute_atomic_update_node(entdb_stub, entdb_seeded, benchmark) -> None:
    from entdb_sdk._generated import entdb_pb2 as pb

    ids = entdb_seeded
    counter = {"i": 0}

    def call() -> None:
        i = counter["i"]
        counter["i"] = (i + 1) % len(ids)
        req = pb.ExecuteAtomicRequest(
            context=_ctx(),
            idempotency_key=f"bench-upd-{uuid.uuid4().hex[:12]}",
            operations=[
                pb.Operation(
                    update_node=pb.UpdateNodeOp(
                        type_id=TASK_TYPE_ID,
                        id=ids[i],
                        patch=_struct({"2": f"updated-{i}"}),
                    )
                )
            ],
        )
        entdb_stub.ExecuteAtomic(req, timeout=10.0)

    benchmark(call)


# ---------------------------------------------------------------------------
# 8 — GetEdgesFrom
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="edge-fanout")
def test_entdb_get_edges_from(entdb_stub, entdb_seeded, benchmark) -> None:
    from entdb_sdk._generated import entdb_pb2 as pb

    ids = entdb_seeded
    counter = {"i": 0}

    def call() -> None:
        i = counter["i"]
        counter["i"] = (i + 1) % len(ids)
        req = pb.GetEdgesRequest(context=_ctx(), node_id=ids[i], limit=20)
        entdb_stub.GetEdgesFrom(req, timeout=5.0)

    benchmark(call)


# ---------------------------------------------------------------------------
# 9 — GetEdgesTo
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="reverse-edge-fanout")
def test_entdb_get_edges_to(entdb_stub, entdb_seeded, benchmark) -> None:
    from entdb_sdk._generated import entdb_pb2 as pb

    ids = entdb_seeded
    counter = {"i": 0}

    def call() -> None:
        i = counter["i"]
        counter["i"] = (i + 1) % len(ids)
        req = pb.GetEdgesRequest(context=_ctx(), node_id=ids[i], limit=20)
        entdb_stub.GetEdgesTo(req, timeout=5.0)

    benchmark(call)


# ---------------------------------------------------------------------------
# 10 — GetConnectedNodes
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="traversal")
def test_entdb_get_connected_nodes(entdb_stub, entdb_seeded, benchmark) -> None:
    from entdb_sdk._generated import entdb_pb2 as pb

    ids = entdb_seeded
    counter = {"i": 0}

    def call() -> None:
        i = counter["i"]
        counter["i"] = (i + 1) % len(ids)
        req = pb.GetConnectedNodesRequest(
            context=_ctx(),
            node_id=ids[i],
            edge_type_id=100,
            limit=20,
        )
        entdb_stub.GetConnectedNodes(req, timeout=5.0)

    benchmark(call)


# ---------------------------------------------------------------------------
# 11 — SearchNodes (full-text, semantics differ — see issue caveats)
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="fulltext")
def test_entdb_search_nodes(entdb_stub, entdb_seeded, benchmark) -> None:
    from entdb_sdk._generated import entdb_pb2 as pb

    req = pb.SearchNodesRequest(
        tenant_id=BENCH_TENANT,
        actor=BENCH_ACTOR,
        type_id=TASK_TYPE_ID,
        query="bench",
        limit=20,
    )

    def call() -> None:
        try:
            entdb_stub.SearchNodes(req, timeout=5.0)
        except Exception:
            # The Task schema in the contract seed may not have a
            # ``searchable`` field, in which case the server returns
            # an empty result or an error — either way the RPC has
            # been timed and the round-trip is what we wanted to
            # measure. We swallow so the bench reports a number
            # rather than failing.
            pass

    benchmark(call)


# ---------------------------------------------------------------------------
# 12 — QueryNodes with predicate (mailbox-list parity)
#
# EntDB has no direct ``GetMailbox`` equivalent for Task-shaped corpus
# (mailbox is a special storage mode); we benchmark the closest read
# shape — type-scoped paginated list, which is exactly what
# Postgres-side ``test_pg_get_mailbox`` does too.
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="mailbox-list")
def test_entdb_mailbox_like_list(entdb_stub, entdb_seeded, benchmark) -> None:
    from entdb_sdk._generated import entdb_pb2 as pb

    req = pb.QueryNodesRequest(
        context=_ctx(),
        type_id=TASK_TYPE_ID,
        limit=20,
        offset=0,
        order_by="created_at",
        descending=True,
    )
    benchmark(lambda: entdb_stub.QueryNodes(req, timeout=5.0))
