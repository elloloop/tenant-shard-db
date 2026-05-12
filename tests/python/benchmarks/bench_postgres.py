# SPDX-License-Identifier: AGPL-3.0-only
"""Postgres-side wire-level bench, mirroring ``bench_entdb.py``.

Each ``test_pg_*`` case targets one row of the EntDB↔Postgres mapping
table from issue #487. Group names match ``bench_entdb.py`` so the
report doc can pair the runs directly:

    point-read | batched-read | filtered-read |
    single-write | multi-op-write | update |
    edge-fanout | reverse-edge-fanout | traversal |
    fulltext | mailbox-list | health

If Postgres is not reachable (``pg_conn`` fixture skips) every test
here is skipped at collection — the EntDB-only Tier 1 run on a dev box
without Docker still produces useful numbers.

The bench is **sync** (``cur.execute(...)`` not ``await``), for the
same reason ``bench_entdb.py`` is sync: pytest-benchmark's measurement
loop is sync, and ``asyncio.run`` per round adds enough overhead to
dominate the operations we want to measure.
"""

from __future__ import annotations

import json
import uuid

import pytest

# Constants are duplicated from ``conftest`` rather than imported, so
# pytest collection works whether the bench dir is loaded as a package
# or via a plain ``pytest path/to``.
BENCH_TENANT = "bench"
TASK_TYPE_ID = 2
ASSIGNED_TO_EDGE_ID = 100

pytestmark = pytest.mark.benchmark


# ---------------------------------------------------------------------------
# 1 — Health (SELECT 1)
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="health")
def test_pg_health(pg_conn, benchmark) -> None:
    def call() -> None:
        with pg_conn.cursor() as cur:
            cur.execute("SELECT 1")
            cur.fetchone()

    benchmark(call)


# ---------------------------------------------------------------------------
# 2 — GetNode → SELECT WHERE tenant_id=$1 AND id=$2
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="point-read")
def test_pg_get_node(pg_conn, corpus, benchmark) -> None:
    ids = [row["node_id"] for row in corpus]
    counter = {"i": 0}

    def call() -> None:
        i = counter["i"]
        counter["i"] = (i + 1) % len(ids)
        with pg_conn.cursor() as cur:
            cur.execute(
                "SELECT type_id, payload FROM nodes WHERE tenant_id=%s AND id=%s",
                (BENCH_TENANT, ids[i]),
            )
            cur.fetchone()

    benchmark(call)


# ---------------------------------------------------------------------------
# 3 — GetNodes (batched, 10) → SELECT WHERE id = ANY($1)
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="batched-read")
def test_pg_get_nodes_batch(pg_conn, corpus, benchmark) -> None:
    ids = [row["node_id"] for row in corpus]
    batch_size = 10
    counter = {"i": 0}

    def call() -> None:
        i = counter["i"]
        counter["i"] = (i + batch_size) % max(1, len(ids) - batch_size)
        batch = ids[i : i + batch_size]
        with pg_conn.cursor() as cur:
            cur.execute(
                "SELECT id, type_id, payload FROM nodes WHERE tenant_id=%s AND id = ANY(%s)",
                (BENCH_TENANT, batch),
            )
            cur.fetchall()

    benchmark(call)


# ---------------------------------------------------------------------------
# 4 — QueryNodes (filtered, limit=10) → SELECT WHERE type_id=$ ORDER BY created_at DESC LIMIT
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="filtered-read")
def test_pg_query_nodes(pg_conn, benchmark) -> None:
    def call() -> None:
        with pg_conn.cursor() as cur:
            cur.execute(
                "SELECT id, payload FROM nodes "
                "WHERE tenant_id=%s AND type_id=%s "
                "ORDER BY created_at DESC LIMIT %s",
                (BENCH_TENANT, TASK_TYPE_ID, 10),
            )
            cur.fetchall()

    benchmark(call)


# ---------------------------------------------------------------------------
# 5 — ExecuteAtomic (create_node, single op) → BEGIN; INSERT; COMMIT
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="single-write")
def test_pg_execute_atomic_create_node(pg_conn, benchmark) -> None:
    def call() -> None:
        nid = f"pg-bench-write-{uuid.uuid4().hex[:16]}"
        payload = json.dumps({"title": "bench write", "description": "x" * 200})
        with pg_conn.cursor() as cur:
            cur.execute(
                "INSERT INTO nodes (tenant_id, id, type_id, payload) "
                "VALUES (%s, %s, %s, %s::jsonb)",
                (BENCH_TENANT, nid, TASK_TYPE_ID, payload),
            )
        pg_conn.commit()

    benchmark(call)


# ---------------------------------------------------------------------------
# 6 — ExecuteAtomic (create_node + create_edge, multi-op)
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="multi-op-write")
def test_pg_execute_atomic_create_node_and_edge(pg_conn, corpus, benchmark) -> None:
    target = corpus[0]["node_id"]

    def call() -> None:
        nid = f"pg-bench-mn-{uuid.uuid4().hex[:16]}"
        payload = json.dumps({"title": "multi", "description": "y" * 100})
        with pg_conn.cursor() as cur:
            cur.execute(
                "INSERT INTO nodes (tenant_id, id, type_id, payload) "
                "VALUES (%s, %s, %s, %s::jsonb)",
                (BENCH_TENANT, nid, TASK_TYPE_ID, payload),
            )
            cur.execute(
                "INSERT INTO edges (tenant_id, from_id, edge_type, to_id) VALUES (%s, %s, %s, %s)",
                (BENCH_TENANT, nid, ASSIGNED_TO_EDGE_ID, target),
            )
        pg_conn.commit()

    benchmark(call)


# ---------------------------------------------------------------------------
# 7 — ExecuteAtomic (update_node) → UPDATE ... SET payload=$1
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="update")
def test_pg_execute_atomic_update_node(pg_conn, corpus, benchmark) -> None:
    ids = [row["node_id"] for row in corpus]
    counter = {"i": 0}

    def call() -> None:
        i = counter["i"]
        counter["i"] = (i + 1) % len(ids)
        patch = json.dumps({"description": f"updated-{i}"})
        with pg_conn.cursor() as cur:
            cur.execute(
                "UPDATE nodes SET payload = payload || %s::jsonb WHERE tenant_id=%s AND id=%s",
                (patch, BENCH_TENANT, ids[i]),
            )
        pg_conn.commit()

    benchmark(call)


# ---------------------------------------------------------------------------
# 8 — GetEdgesFrom → SELECT WHERE from_id=$1
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="edge-fanout")
def test_pg_get_edges_from(pg_conn, corpus, benchmark) -> None:
    ids = [row["node_id"] for row in corpus]
    counter = {"i": 0}

    def call() -> None:
        i = counter["i"]
        counter["i"] = (i + 1) % len(ids)
        with pg_conn.cursor() as cur:
            cur.execute(
                "SELECT to_id, edge_type, props FROM edges "
                "WHERE tenant_id=%s AND from_id=%s LIMIT %s",
                (BENCH_TENANT, ids[i], 20),
            )
            cur.fetchall()

    benchmark(call)


# ---------------------------------------------------------------------------
# 9 — GetEdgesTo → SELECT WHERE to_id=$1 (uses edges_to_idx)
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="reverse-edge-fanout")
def test_pg_get_edges_to(pg_conn, corpus, benchmark) -> None:
    ids = [row["node_id"] for row in corpus]
    counter = {"i": 0}

    def call() -> None:
        i = counter["i"]
        counter["i"] = (i + 1) % len(ids)
        with pg_conn.cursor() as cur:
            cur.execute(
                "SELECT from_id, edge_type, props FROM edges "
                "WHERE tenant_id=%s AND to_id=%s LIMIT %s",
                (BENCH_TENANT, ids[i], 20),
            )
            cur.fetchall()

    benchmark(call)


# ---------------------------------------------------------------------------
# 10 — GetConnectedNodes (depth=1) → JOIN edges and nodes
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="traversal")
def test_pg_get_connected_nodes(pg_conn, corpus, benchmark) -> None:
    ids = [row["node_id"] for row in corpus]
    counter = {"i": 0}

    def call() -> None:
        i = counter["i"]
        counter["i"] = (i + 1) % len(ids)
        with pg_conn.cursor() as cur:
            cur.execute(
                "SELECT n.id, n.type_id, n.payload FROM edges e "
                "JOIN nodes n ON n.tenant_id=e.tenant_id AND n.id=e.to_id "
                "WHERE e.tenant_id=%s AND e.from_id=%s LIMIT %s",
                (BENCH_TENANT, ids[i], 20),
            )
            cur.fetchall()

    benchmark(call)


# ---------------------------------------------------------------------------
# 11 — SearchMailbox → tsvector @@ plainto_tsquery
#
# Indicative-only (Postgres FTS does stemming, EntDB does substring).
# Reported side-by-side without a north-star target.
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="fulltext")
def test_pg_search_mailbox(pg_conn, benchmark) -> None:
    def call() -> None:
        with pg_conn.cursor() as cur:
            cur.execute(
                "SELECT id, payload FROM nodes WHERE tenant_id=%s AND "
                "to_tsvector('simple', "
                "  coalesce(payload->>'title','') || ' ' || "
                "  coalesce(payload->>'description','')) "
                "@@ plainto_tsquery('simple', %s) "
                "LIMIT %s",
                (BENCH_TENANT, "bench", 20),
            )
            cur.fetchall()

    benchmark(call)


# ---------------------------------------------------------------------------
# 12 — GetMailbox (owner-scoped recent list)
# ---------------------------------------------------------------------------


@pytest.mark.benchmark(group="mailbox-list")
def test_pg_get_mailbox(pg_conn, benchmark) -> None:
    counter = {"i": 0}

    def call() -> None:
        # Cycle through the 25 owners the corpus generates so the
        # planner sees variety.
        i = counter["i"]
        counter["i"] = (i + 1) % 25
        owner = f"user-{i}"
        with pg_conn.cursor() as cur:
            cur.execute(
                "SELECT id, payload FROM nodes "
                "WHERE tenant_id=%s AND type_id=%s AND payload->>'owner'=%s "
                "ORDER BY created_at DESC LIMIT %s",
                (BENCH_TENANT, TASK_TYPE_ID, owner, 20),
            )
            cur.fetchall()

    benchmark(call)
