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


def main() -> None:
    if len(sys.argv) != 2:
        print("usage: dump_pg_explain.py <DSN>", file=sys.stderr)
        sys.exit(2)
    dsn = sys.argv[1]
    with psycopg.connect(dsn) as conn:
        for title, sql, params in QUERIES:
            print(f"\n========== {title} ==========\n")
            try:
                with conn.cursor() as cur:
                    cur.execute("EXPLAIN (ANALYZE, BUFFERS, VERBOSE) " + sql, params)
                    for (line,) in cur.fetchall():
                        print(line)
            except Exception as exc:
                print(f"(EXPLAIN failed: {exc})")


if __name__ == "__main__":
    main()
