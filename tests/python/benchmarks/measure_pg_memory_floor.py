# SPDX-License-Identifier: AGPL-3.0-only
"""Measure the Postgres memory floor at a 10k-node / 100k-edge corpus.

Issue #491 item 6 — the original floor table in
``docs/benchmarks/benchmarks.md`` was measured on a 1k-node / 5k-edge
corpus and showed ``pg`` happy from 256m to 1024m. The reviewer's point
was that a tiny working set fits in any RAM budget; the table proves
"PG doesn't crash at 256m on a trivial workload," not "PG runs
reliably at 256m on a realistic one."

This script re-measures at 10x the corpus size:

* 10000 ``Task``-shaped nodes (matches the EntDB seed shape)
* 100000 fan-out edges (10 per node)
* 500 mixed read / 100 mixed write transactions after warmup

For each ``--memory`` step it spins up ``postgres:17-alpine`` under
``--cpus=1 --memory=<step>m``, creates the documented bench schema +
indexes, bulk-loads the corpus, then drives the same query shapes
``dump_pg_explain.py`` records.

Pass criteria mirror the original measurement so the new and old rows
are directly comparable:

* No ``OOMKilled``
* Connection never drops (we'd see ``OperationalError``)
* Workload completes (no soft time-out)

This script is **not** invoked by the bench harness — it's a one-shot
that produces the table in ``docs/benchmarks/benchmarks.md``. Run it on
the host whose floor you want to document; the result is dev-machine
specific (see the doc caveat about CI runner page-cache headroom).

Usage::

    python measure_pg_memory_floor.py 256 384 512 768 1024

Each step prints a row of ``| memory | boot | workload time | status |``
in markdown-table form so the doc can be updated by copy-paste.
"""

from __future__ import annotations

import json
import subprocess
import sys
import time
import uuid

# Match the conftest fixture so the schema is identical.
BENCH_TENANT = "bench"
TASK_TYPE_ID = 2
ASSIGNED_TO_EDGE_ID = 100

CORPUS_NODES = 10_000
EDGES_PER_NODE = 10  # → 100_000 edges total
READ_TXNS = 500
WRITE_TXNS = 100


SCHEMA_SQL = """
DROP TABLE IF EXISTS edges;
DROP TABLE IF EXISTS nodes;

CREATE TABLE nodes (
    tenant_id  TEXT  NOT NULL,
    id         TEXT  NOT NULL,
    type_id    INT   NOT NULL,
    payload    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX nodes_type_idx     ON nodes(tenant_id, type_id, created_at DESC);
CREATE INDEX nodes_payload_gin  ON nodes USING GIN (payload jsonb_path_ops);
CREATE INDEX nodes_fts_idx      ON nodes USING GIN (
    to_tsvector('simple',
        coalesce(payload->>'title','') || ' ' || coalesce(payload->>'description','')));

CREATE TABLE edges (
    tenant_id  TEXT  NOT NULL,
    from_id    TEXT  NOT NULL,
    edge_type  INT   NOT NULL,
    to_id      TEXT  NOT NULL,
    props      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, from_id, edge_type, to_id)
);

CREATE INDEX edges_to_idx ON edges(tenant_id, to_id, edge_type);
"""


def _free_port() -> int:
    import socket

    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


def _wait_pg(dsn: str, timeout: float = 30.0) -> bool:
    import psycopg

    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        try:
            with psycopg.connect(dsn, connect_timeout=2) as conn:
                conn.execute("SELECT 1")
            return True
        except Exception:  # noqa: BLE001
            time.sleep(0.5)
    return False


def _corpus_payload(i: int) -> str:
    subjects = [
        "Refactor applier batching",
        "Investigate WAL replay flake",
        "Pin franz-go version",
        "Backfill schema validation",
        "Reduce p99 latency",
        "Improve SDK error messages",
        "Bench harness rewrite",
        "Multi-tenant ACL audit",
    ]
    body = f"node-{i} body-token-{i:08d}-" + "x" * 64 + f" owner=user-{i % 25}"
    return json.dumps(
        {
            "title": subjects[i % len(subjects)],
            "description": body,
            "owner": f"user-{i % 25}",
        }
    )


def _run_step(mem_mb: int) -> dict:
    """Run one memory step. Returns row dict for the markdown table."""
    import psycopg

    name = f"pg-floor-{uuid.uuid4().hex[:8]}"
    port = _free_port()
    image = "postgres:17-alpine"

    boot_t0 = time.monotonic()
    proc = subprocess.run(
        [
            "docker",
            "run",
            "-d",
            "--rm",
            "--name",
            name,
            "--cpus",
            "1",
            "--memory",
            f"{mem_mb}m",
            "-e",
            "POSTGRES_PASSWORD=bench",
            "-e",
            "POSTGRES_USER=bench",
            "-e",
            "POSTGRES_DB=bench",
            "-p",
            f"{port}:5432",
            image,
        ],
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        return {
            "mem": mem_mb,
            "boot": "fail",
            "workload": "—",
            "status": f"docker run failed: {proc.stderr.strip()}",
        }

    dsn = f"postgresql://bench:bench@127.0.0.1:{port}/bench"
    try:
        if not _wait_pg(dsn, timeout=60.0):
            return {
                "mem": mem_mb,
                "boot": "fail",
                "workload": "—",
                "status": "did not accept connections in 60s",
            }
        boot_secs = time.monotonic() - boot_t0

        # Schema
        with psycopg.connect(dsn, autocommit=False) as conn:
            with conn.cursor() as cur:
                cur.execute(SCHEMA_SQL)
            conn.commit()

            # Bulk-load 10k nodes
            with conn.cursor() as cur:
                cur.executemany(
                    "INSERT INTO nodes (tenant_id, id, type_id, payload) "
                    "VALUES (%s, %s, %s, %s::jsonb)",
                    [
                        (BENCH_TENANT, f"bench-node-{i:06d}", TASK_TYPE_ID, _corpus_payload(i))
                        for i in range(CORPUS_NODES)
                    ],
                )
            conn.commit()

            # Bulk-load 100k edges (10 per node, wrap)
            edge_rows = [
                (
                    BENCH_TENANT,
                    f"bench-node-{i:06d}",
                    ASSIGNED_TO_EDGE_ID,
                    f"bench-node-{(i + k) % CORPUS_NODES:06d}",
                )
                for i in range(CORPUS_NODES)
                for k in range(1, EDGES_PER_NODE + 1)
            ]
            with conn.cursor() as cur:
                cur.executemany(
                    "INSERT INTO edges (tenant_id, from_id, edge_type, to_id) "
                    "VALUES (%s, %s, %s, %s)",
                    edge_rows,
                )
                cur.execute("ANALYZE nodes")
                cur.execute("ANALYZE edges")
            conn.commit()

            # Workload: 500 reads (mix of point/batched/filtered/edge-fanout) + 100 writes.
            wl_t0 = time.monotonic()
            for i in range(READ_TXNS):
                with conn.cursor() as cur:
                    nid = f"bench-node-{i % CORPUS_NODES:06d}"
                    if i % 5 == 0:
                        cur.execute(
                            "SELECT type_id, payload FROM nodes WHERE tenant_id=%s AND id=%s",
                            (BENCH_TENANT, nid),
                        )
                        cur.fetchone()
                    elif i % 5 == 1:
                        batch = [f"bench-node-{(i + k) % CORPUS_NODES:06d}" for k in range(10)]
                        cur.execute(
                            "SELECT id, payload FROM nodes WHERE tenant_id=%s AND id = ANY(%s)",
                            (BENCH_TENANT, batch),
                        )
                        cur.fetchall()
                    elif i % 5 == 2:
                        cur.execute(
                            "SELECT id, payload FROM nodes WHERE tenant_id=%s AND type_id=%s "
                            "ORDER BY created_at DESC LIMIT 10",
                            (BENCH_TENANT, TASK_TYPE_ID),
                        )
                        cur.fetchall()
                    elif i % 5 == 3:
                        cur.execute(
                            "SELECT to_id, edge_type FROM edges "
                            "WHERE tenant_id=%s AND from_id=%s LIMIT 20",
                            (BENCH_TENANT, nid),
                        )
                        cur.fetchall()
                    else:
                        cur.execute(
                            "SELECT id, payload FROM nodes WHERE tenant_id=%s AND "
                            "to_tsvector('simple', "
                            "coalesce(payload->>'title','') || ' ' || "
                            "coalesce(payload->>'description','')) "
                            "@@ plainto_tsquery('simple', %s) LIMIT 20",
                            (BENCH_TENANT, "bench"),
                        )
                        cur.fetchall()

            for i in range(WRITE_TXNS):
                with conn.cursor() as cur:
                    nid = f"floor-write-{i:06d}-{uuid.uuid4().hex[:6]}"
                    cur.execute(
                        "INSERT INTO nodes (tenant_id, id, type_id, payload) "
                        "VALUES (%s, %s, %s, %s::jsonb)",
                        (BENCH_TENANT, nid, TASK_TYPE_ID, _corpus_payload(i + CORPUS_NODES)),
                    )
                conn.commit()
            wl_secs = time.monotonic() - wl_t0

        # OOM-killed?
        insp = subprocess.run(
            ["docker", "inspect", "--format", "{{.State.OOMKilled}} {{.State.ExitCode}}", name],
            capture_output=True,
            text=True,
        )
        oom = "true" in (insp.stdout or "").lower()
        if oom:
            return {
                "mem": mem_mb,
                "boot": "ok",
                "workload": f"{wl_secs:.2f}s",
                "status": "OOMKilled",
            }
        return {
            "mem": mem_mb,
            "boot": f"ok ({boot_secs:.1f}s)",
            "workload": f"{wl_secs:.2f}s",
            "status": "ok",
        }
    except Exception as exc:  # noqa: BLE001
        return {
            "mem": mem_mb,
            "boot": "ok",
            "workload": "—",
            "status": f"workload failed: {type(exc).__name__}: {exc}",
        }
    finally:
        subprocess.run(["docker", "rm", "-f", name], capture_output=True)


def main() -> int:
    steps = [256, 384, 512, 768, 1024] if len(sys.argv) < 2 else [int(x) for x in sys.argv[1:]]

    print(f"# PG memory-floor — 10k nodes / 100k edges / {READ_TXNS}r + {WRITE_TXNS}w")
    print()
    print("| `--memory` | Cold boot | Workload time | Status |")
    print("|------------|-----------|---------------|--------|")
    for mem in steps:
        row = _run_step(mem)
        print(
            f"| {row['mem']}m | {row['boot']} | {row['workload']} | {row['status']} |", flush=True
        )
    return 0


if __name__ == "__main__":
    sys.exit(main())
