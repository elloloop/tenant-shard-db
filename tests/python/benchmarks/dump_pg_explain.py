# SPDX-License-Identifier: AGPL-3.0-only
"""Dump ``EXPLAIN ANALYZE`` for each Postgres bench query.

Run after the bench (via ``run_bench.sh --tier 2``) so reviewers can
audit that the right indexes are being used. Issue #487 open question
#10 — "Should we record the Postgres EXPLAIN ANALYZE output for each
bench query in CI, so reviewers can confirm the right index is used?"
Answer: yes, cheap and high-value, dumps to artifacts.

Usage:
    python dump_pg_explain.py postgresql://bench:bench@127.0.0.1:55432/bench
"""

from __future__ import annotations

import sys

import psycopg

QUERIES: list[tuple[str, str, tuple]] = [
    (
        "GetNode (point-read)",
        "SELECT type_id, payload FROM nodes WHERE tenant_id=%s AND id=%s",
        ("bench", "bench-node-000000"),
    ),
    (
        "GetNodes (batched-read)",
        "SELECT id, type_id, payload FROM nodes WHERE tenant_id=%s AND id = ANY(%s)",
        ("bench", [f"bench-node-{i:06d}" for i in range(10)]),
    ),
    (
        "QueryNodes (filtered-read)",
        "SELECT id, payload FROM nodes WHERE tenant_id=%s AND type_id=%s "
        "ORDER BY created_at DESC LIMIT %s",
        ("bench", 2, 10),
    ),
    (
        "GetEdgesFrom (edge-fanout)",
        "SELECT to_id, edge_type, props FROM edges WHERE tenant_id=%s AND from_id=%s LIMIT %s",
        ("bench", "bench-node-000000", 20),
    ),
    (
        "GetEdgesTo (reverse-edge-fanout)",
        "SELECT from_id, edge_type, props FROM edges WHERE tenant_id=%s AND to_id=%s LIMIT %s",
        ("bench", "bench-node-000000", 20),
    ),
    (
        "GetConnectedNodes (traversal)",
        "SELECT n.id, n.type_id, n.payload FROM edges e "
        "JOIN nodes n ON n.tenant_id=e.tenant_id AND n.id=e.to_id "
        "WHERE e.tenant_id=%s AND e.from_id=%s LIMIT %s",
        ("bench", "bench-node-000000", 20),
    ),
    (
        "SearchMailbox (fulltext)",
        "SELECT id, payload FROM nodes WHERE tenant_id=%s AND "
        "to_tsvector('simple', "
        "coalesce(payload->>'title','') || ' ' || coalesce(payload->>'description','')) "
        "@@ plainto_tsquery('simple', %s) LIMIT %s",
        ("bench", "bench", 20),
    ),
    (
        "GetMailbox (mailbox-list)",
        "SELECT id, payload FROM nodes "
        "WHERE tenant_id=%s AND type_id=%s AND payload->>'owner'=%s "
        "ORDER BY created_at DESC LIMIT %s",
        ("bench", 2, "user-0", 20),
    ),
]


def _corpus_present(conn: psycopg.Connection) -> bool:
    """Return True if the bench corpus appears to be loaded.

    Checks both that the schema exists and that ``bench-node-000000``
    (hardcoded into the ``QUERIES`` list) is actually present — without
    it every plan reports ``rows=0`` and the dump is misleading (issue
    #491 item 7).
    """
    try:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT 1 FROM nodes WHERE tenant_id=%s AND id=%s",
                ("bench", "bench-node-000000"),
            )
            return cur.fetchone() is not None
    except psycopg.errors.UndefinedTable:
        return False
    except Exception:  # noqa: BLE001
        return False


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: dump_pg_explain.py <DSN>", file=sys.stderr)
        return 2
    dsn = sys.argv[1]
    with psycopg.connect(dsn) as conn:
        if not _corpus_present(conn):
            print(
                "ERROR: bench corpus not found in this Postgres instance.\n"
                "  Expected row (tenant_id='bench', id='bench-node-000000') is missing,\n"
                "  or the `nodes` table doesn't exist yet.\n"
                "\n"
                "  Run the bench first so the seed fixture populates the corpus, e.g.:\n"
                "    tests/python/benchmarks/run_bench.sh --tier 2\n"
                "  or, if the compose stack is already up:\n"
                "    pytest tests/python/benchmarks/bench_postgres.py --benchmark-only -q\n"
                "\n"
                "  Without the seed, every EXPLAIN ANALYZE plan reports rows=0 and is\n"
                "  not useful for confirming index usage.",
                file=sys.stderr,
            )
            return 1

        for title, sql, params in QUERIES:
            print(f"\n========== {title} ==========\n")
            try:
                with conn.cursor() as cur:
                    cur.execute("EXPLAIN (ANALYZE, BUFFERS, VERBOSE) " + sql, params)
                    for (line,) in cur.fetchall():
                        print(line)
            except Exception as exc:  # noqa: BLE001
                print(f"(EXPLAIN failed: {exc})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
