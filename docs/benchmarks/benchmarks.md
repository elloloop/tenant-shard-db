# EntDB benchmarks — Postgres comparison

**Status:** Issue #487 / PR #489. Replaces the now-stale
`python-vs-go.md` (Phase 4B; the Python server was deleted in Phase 4D,
commit 8d07f5f).

EntDB is a graph DB on top of per-tenant SQLite + a WAL. Without an
external anchor we can only see EntDB-vs-itself trends. Postgres is the
natural north-star: it's the relational workload everyone reaches for,
the wire-level RTT on a single point read is the de-facto floor we
should be aiming at, and "ratio vs Postgres" is a number a reader can
calibrate against instantly.

This doc covers what we measure, how we measure it, the two-tier
strategy, and the per-RPC north-star targets. **It is not a "beat
Postgres" claim** — beating Postgres on a graph workload it has to JSON-
decode is an eventual target. The immediate output is a number we can
stare at on every PR.

## Two-tier strategy

| Tier | Containers | What it answers | CI trigger |
|------|------------|-----------------|------------|
| **Tier 1 — EntDB smallest** | `entdb-server` only at `--cpus=1 --memory=512m` | Is EntDB regressing against itself? | Every PR, push to main, tag, `workflow_dispatch` |
| **Tier 2 — Parity** | `entdb-server` + `postgres:17-alpine`, both at `--cpus=1 --memory=512m` | Is EntDB competitive with Postgres at the same resource budget? | Push to main, tags, PR with `bench:postgres` label, `workflow_dispatch` |

The split is deliberate. Tier 1 is the "every PR" gate — fast,
zero-Docker, runs in ~15s on a CI runner. Tier 2 is heavier (compose
stack, two backends, EXPLAIN dump) so it gates on a label or a push
to main; most PRs don't change anything that would affect the Postgres
comparison.

Both tiers write pytest-benchmark JSON to `.benchmarks/`:

```
.benchmarks/entdb-go/<UTC-timestamp>_tier1_constrained.json
.benchmarks/entdb-go/<UTC-timestamp>_tier2_parity.json
.benchmarks/postgres/<UTC-timestamp>_tier2_parity.json
```

## Operation mapping table

EntDB is a graph DB; Postgres is relational. For each EntDB RPC we
benchmark there is a closest semantically-equivalent Postgres workload
and a small set of indexes that makes the comparison fair.
**Equivalent** here means "same client-observable behaviour for the
same input shape": same payload returned, same number of round-trips,
same durability level (Postgres `synchronous_commit=on` by default;
EntDB acks after WAL apply).

| # | EntDB RPC | Postgres equivalent | Bench group | North-star ratio (EntDB p50 / Postgres p50) |
|---|---|---|---|---|
| 1 | `Health` | `SELECT 1` | `health` | 1.0 |
| 2 | `GetNode(id, tenant)` | `SELECT … WHERE tenant_id=$1 AND id=$2` | `point-read` | within 1.30x |
| 3 | `GetNodes(ids[], tenant)` | `SELECT … WHERE id = ANY($1)` | `batched-read` | within 1.20x |
| 4 | `QueryNodes(type_id, …)` | `SELECT … WHERE type_id=$ ORDER BY created_at DESC LIMIT` | `filtered-read` | within 1.50x |
| 5 | `ExecuteAtomic(create_node)` | `BEGIN; INSERT; COMMIT;` | `single-write` | within 2.0x (long-term: 1.30x) |
| 6 | `ExecuteAtomic(create_node + create_edge)` | `BEGIN; INSERT; INSERT; COMMIT;` | `multi-op-write` | within 1.30x |
| 7 | `ExecuteAtomic(update_node)` | `UPDATE … SET payload = …` | `update` | within 1.50x |
| 8 | `GetEdgesFrom(node_id)` | `SELECT … FROM edges WHERE from_id=$1` | `edge-fanout` | **beat** Postgres (ratio ≤ 1.0) |
| 9 | `GetEdgesTo(node_id)` | `SELECT … FROM edges WHERE to_id=$1` (uses `edges_to_idx`) | `reverse-edge-fanout` | **beat** Postgres (ratio ≤ 1.0) |
| 10 | `GetConnectedNodes(node_id, depth=1)` | `SELECT … FROM edges JOIN nodes ON …` | `traversal` | within 1.50x |
| 11 | `SearchMailbox(query)` | `tsvector @@ plainto_tsquery(…)` | `fulltext` | not a target (different semantics) |
| 12 | `GetMailbox(user_id, limit)` | `SELECT … WHERE owner=$ ORDER BY created_at DESC LIMIT` | `mailbox-list` | within 1.50x |

The ratio targets are encoded in `tests/python/benchmarks/conftest.py`
(`RATIO_TARGETS` dict) and attached to each pytest-benchmark item as
`user_properties.ratio_target` so a downstream report tool can read
them without parsing the test source. A ratio of `1.00` means
"match-or-beat"; anything `> 1.00` is "within Nx of Postgres".

These are **starting positions, not contracts**. The first run sets
the baseline; each PR that improves a number updates the doc. If your
local runs miss a target, that's a measurement to record — not a number
to fudge.

## Postgres schema and indexes

Mirrors EntDB's `(id, type_id, payload-jsonb)` shape closely enough to
be semantically equivalent for the operations we measure. Defined in
`tests/python/benchmarks/conftest.py` (`pg_conn` fixture):

```sql
CREATE TABLE nodes (
    tenant_id  TEXT  NOT NULL,
    id         TEXT  NOT NULL,
    type_id    INT   NOT NULL,
    payload    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX nodes_type_idx    ON nodes(tenant_id, type_id, created_at DESC);
CREATE INDEX nodes_payload_gin ON nodes USING GIN (payload jsonb_path_ops);
CREATE INDEX nodes_fts_idx     ON nodes USING GIN (
    to_tsvector('simple',
        coalesce(payload->>'title','') || ' ' || coalesce(payload->>'description',''))
);

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
```

The indexes are the **minimum required** for the bench queries to use
an index at all. We do not tune Postgres beyond defaults (no
`shared_buffers` override, no `work_mem` bump, no `synchronous_commit=off`).
Justification:

1. EntDB ships with its current defaults. Tuning Postgres but not
   tuning EntDB would be unfair to Postgres on a recency basis (Postgres
   has 25+ years of default-tuning Stockholm syndrome) and unfair to
   EntDB on every other axis.
2. Anyone running this benchmark themselves will use Postgres defaults;
   we want the public number to match what they'll see.
3. Indexes are not "tuning" — they are the minimum required for the
   query to use an index at all. A Postgres user benchmarking these
   workloads would create them.

A future issue can add a "tuned" tier showing the gap; it lives in a
separate column with a "tuned" superscript. The headline number is
defaults.

## Postgres memory floor — empirical

Issue #487 asks: how small can Postgres actually run the bench
workload? Community lore says "Postgres often fails to start or
thrashes immediately at `--memory=256m`". On `postgres:17-alpine` with
this bench's 1k-node + 5k-edge corpus, that lore is **stale**:

| `--memory` | Cold boot | Bench-shaped workload (1k seed + 500 reads + 100 writes) |
|------------|-----------|----------------------------------------------------------|
| 256m       | ok        | ok (~0.40s)                                              |
| 384m       | ok        | ok (~0.41s)                                              |
| 512m       | ok        | ok (~0.41s)                                              |
| 768m       | ok        | ok (~0.42s)                                              |
| 1024m      | ok        | ok (~0.42s)                                              |

Methodology: `tests/python/benchmarks/dump_pg_explain.py` exercises the
same query shapes the bench measures (point reads, edge fanout,
filtered list, FTS, batched writes). At each `--memory` step the script
brings up `postgres:17-alpine` under `--cpus=1`, creates the documented
schema + indexes, bulk-loads the 1k-node / 5k-edge corpus, then runs
500 read transactions and 100 write transactions. Pass = no
`OOMKilled`, no connection drop, workload completes in <5s.

**Pinned Tier 2 floor: 512m.** Rationale:

- Empirical minimum on this dev box was **256m** — well under the 768m
  / 1g the issue speculated.
- Linux CI runners (GH-hosted `ubuntu-latest`, 7GB total RAM, no
  dedicated cgroup) have less page-cache headroom than a 32GB dev box;
  pinning at 256m there would risk flake from one bad GC pause.
- 512m gives us 2x headroom over the empirical floor and matches the
  EntDB Tier 1 budget exactly (apples-to-apples on the absolute number).

If a future workload (large corpus, concurrent clients, real ACL graph)
pushes the floor higher, re-run the methodology and update the table.

## Reproduce locally

```bash
# Tier 1 — EntDB only, host subprocess.
tests/python/benchmarks/run_bench.sh --tier 1

# Tier 2 — bring up the compose stack, run both backends.
tests/python/benchmarks/run_bench.sh --tier 2

# Same as --tier 2.
tests/python/benchmarks/run_bench.sh --with-postgres

# Tier 2 with a different memory budget (e.g. to verify the floor).
tests/python/benchmarks/run_bench.sh --tier 2 --memory 256m

# Bench with more rounds (better precision, slower).
BENCH_ROUNDS=30 tests/python/benchmarks/run_bench.sh --tier 1
```

Postgres results land in `.benchmarks/postgres/`; EntDB results in
`.benchmarks/entdb-go/`. Tier 2 also dumps `EXPLAIN ANALYZE` for each
bench query to `.benchmarks/postgres/<ts>_tier2_parity_explain.txt` so
reviewers can audit that the right indexes are used.

## Excluded from the comparison

EntDB-only RPCs with no fair Postgres mapping (44 RPCs total in
`proto/entdb/v1/entdb.proto`; ~12 mapped above, ~32 excluded):

- ACL graph — `ShareNode`, `RevokeAccess`, `DelegateAccess`,
  `ListSharedWithMe`. You can model it with a `permissions` table but
  the comparison stops being meaningful.
- Admin / GDPR — `TransferOwnership`, `TransferUserContent`,
  `SetLegalHold`, `RevokeAllUserAccess`. Multi-statement procedures
  that no real-world app would write the same way EntDB does.
- Tenancy / IAM control plane — `CreateUser`, `CreateTenant`,
  `ArchiveTenant`, `AddTenantMember`, `ChangeMemberRole`,
  `GetTenantMembers`, `GetUserTenants`, `AddGroupMember`,
  `RemoveGroupMember`.
- EntDB-internal — `WaitForOffset`, `GetReceiptStatus`, `GetSchema`,
  `ListTenants`, `ListMailboxUsers`.

These continue to be benched against the Go server in Tier 1 with no
external reference.

## Caveats

- **Graph vs relational.** Postgres can model a graph but isn't
  optimised for it the way EntDB is. The mapping table is the best
  honest equivalence; row #11 (full-text) is indicative only — Postgres
  FTS does stemming, EntDB does substring matching.
- **Single-tenant bench.** All 12 RPCs are exercised against one tenant;
  Postgres's `tenant_id` column is a no-op partition with one value.
  Multi-tenant Postgres performance is a separate question
  (schema-per-tenant, table-per-tenant, RLS, partitioning by
  `tenant_id`).
- **No ACL filtering.** EntDB's `GetNode` runs through
  Permission-checking middleware; the Postgres equivalent is a raw
  `SELECT`. The bench skips EntDB's ACL middleware to keep the
  comparison fair on the storage hop. ACL overhead is measured
  separately in Tier 1.
- **In-memory WAL on EntDB.** The bench uses the in-memory WAL
  backend; Postgres uses its real WAL. This favours EntDB on write
  latency. A Kafka-WAL variant is future work.
- **Same-host loopback.** Both DBs are on `localhost`; no real network
  latency. Favours both equally but makes absolute numbers optimistic
  for cloud deployments.
- **`psycopg` connection overhead** is excluded by reusing one
  connection per test (matches the gRPC channel reuse on the EntDB
  side).
- **Cold cache vs warm.** The bench runs after a warmup phase; both
  DBs are measured warm. Cold-start numbers are out of scope.
- **No idempotency replay.** EntDB's `ExecuteAtomic` supports
  idempotency keys; Postgres has no equivalent. The bench does not
  exercise idempotent retry on either side.
- **JSONB encode/decode.** Both sides pay JSON cost. EntDB pays it
  twice on writes (client → field-id payload → SQLite blob); Postgres
  pays it once (client → JSONB binary). Slight unfairness toward EntDB
  on write.

## Open questions tracked elsewhere

- SQLite-direct bench (would tell us the WAL+applier overhead).
- pgvector / FoundationDB / DGraph / Neo4j comparisons.
- Tuned-small Postgres tier (`shared_buffers=64MB`, `work_mem=1MB`).
- Multi-tenant Postgres partitioning bench.

All separate issues — this one is Postgres-defaults only.

## Raw data

```
.benchmarks/entdb-go/<UTC-timestamp>_tier1_constrained.json
.benchmarks/entdb-go/<UTC-timestamp>_tier2_parity.json
.benchmarks/postgres/<UTC-timestamp>_tier2_parity.json
.benchmarks/postgres/<UTC-timestamp>_tier2_parity_explain.txt
```

Each pytest-benchmark JSON records `machine_info` (OS / arch / Python),
the per-RPC group, and `user_properties.ratio_target` so a downstream
tool can render the side-by-side comparison table without parsing the
test source.
