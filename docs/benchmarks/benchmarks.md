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

**Parity is enforced by `docker-compose.bench.yml`, not by the CI
runner.** Both the local `run_bench.sh --tier 2` flow and the CI
`Tier 2` workflow bring up the compose stack, which pins
`entdb-server` and `postgres:17-alpine` to `cpus=1.0` / `mem_limit=512m`
each. The CI job dumps the inspected NanoCpus + Memory limits before
running the benches so a reviewer can confirm the cgroup is actually
applied (the previous GHA-service-container path declared
`postgres:17-alpine` with no resource options, which meant the
container ran with the runner's full 7GB / 2-CPU budget — "parity" in
name only, fixed in issue #491 item 5).

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

**Shape note — `update` (row #7).** EntDB's `UpdateNodeOp` sends a
client-side `patch` that the server merges into the existing payload
field-by-field (no full-row rewrite at the storage layer; only changed
field-id slots touch SQLite). Postgres uses `payload = payload ||
$1::jsonb` which is a JSONB merge at SQL level but is **internally a
full-row rewrite** — Postgres has no in-place update on the heap, so
every UPDATE writes a new row version and the old one is reclaimed by
VACUUM. The two are wire-equivalent ("merge this patch into the row")
but storage-divergent. We picked the JSONB-merge form on the Postgres
side rather than a full `SET payload = $1` rewrite because it matches
the wire intent — but a reader comparing the absolute write cost should
be aware that EntDB's edge here is partly a JSON-on-disk vs row-on-heap
asymmetry, not pure runtime speed.

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
thrashes immediately at `--memory=256m`". The original PR #489 table
measured this on a 1k-node / 5k-edge corpus — a fair-game starting
point but, as the PR review (issue #491 item 6) called out, a trivial
working set fits in any memory budget and the table only proved "PG
doesn't crash at 256m on a trivial workload."

Re-measured at **10x** the corpus — 10k nodes / 100k edges — using
`tests/python/benchmarks/measure_pg_memory_floor.py`:

| `--memory` | Cold boot   | Workload (500r + 100w on 10k/100k) | Status |
|------------|-------------|------------------------------------|--------|
| 256m       | ok (1.3s)   | 0.27s                              | ok     |
| 384m       | ok (1.2s)   | 0.22s                              | ok     |
| 512m       | ok (1.2s)   | 0.23s                              | ok     |
| 768m       | ok (1.2s)   | 0.23s                              | ok     |
| 1024m      | ok (1.2s)   | 0.22s                              | ok     |

Even at 10× the original corpus, `postgres:17-alpine` is unfazed at
256m on this dev box (macOS / arm64, Docker Desktop, ~3.8GB Docker VM
ceiling — see the script for the exact methodology). The 256m row
shows a slightly higher workload time because the 10k bulk-load fills
the shared-buffers ring; once it's warm subsequent rows are equivalent.
No OOM kills, no connection drops, no plan flips visible in the
EXPLAIN dumps.

Methodology: `tests/python/benchmarks/measure_pg_memory_floor.py`
brings up `postgres:17-alpine` under `--cpus=1 --memory=<step>m`,
creates the documented schema + indexes, bulk-loads 10k Task-shaped
nodes + 100k fan-out edges (10 per node), then runs 500 mixed read
transactions (point / batched / filtered / edge-fanout / FTS) and 100
write transactions. Pass = no `OOMKilled`, no connection drop,
workload completes. Run the script yourself to reproduce on a
different machine:

```bash
python tests/python/benchmarks/measure_pg_memory_floor.py 256 384 512 768 1024
```

**Pinned Tier 2 floor: 512m.** Rationale, restated against the
10k/100k evidence:

- Empirical minimum on this dev box was still **256m** — neither the
  1k nor the 10k corpus pushed PG into distress.
- Linux CI runners (GH-hosted `ubuntu-latest`, 7GB total RAM, no
  dedicated cgroup) have less page-cache headroom than a 16-32GB dev
  box; pinning at 256m there would risk flake from one bad GC pause
  or a noisy-neighbour eviction. The CI run is the headline number
  this doc cites — flake there is more expensive than 256MB.
- 512m gives us 2x headroom over the empirical floor and matches the
  EntDB Tier 1 budget exactly (apples-to-apples on the absolute number).

If a future workload (real ACL graph, concurrent clients, 10x larger
corpus again) pushes the floor higher, re-run the script and update
the table. The measurement script is **not** invoked by the bench
harness or CI — it's a one-shot doc-evidence tool.

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
- **No ACL filtering — symmetric on reads and writes.** EntDB's
  `GetNode` runs through Permission-checking middleware; EntDB writes
  also run a tenant-membership + role check in `ExecuteAtomic`. The
  Postgres equivalents are raw `SELECT` / `INSERT` / `UPDATE` with no
  authz hop. To keep the comparison apples-to-apples on **both** the
  read and write paths, the bench runs as the `system:bench` actor —
  `system:*` actors bypass both the read-side ACL filter
  (`server/go/internal/api/query_nodes.go:applyQueryACLFilter`, system
  short-circuit at L242) and the write-side membership gate
  (`server/go/internal/api/execute_atomic.go:checkTenantWriteAccess`,
  system short-circuit at L585). Previously the bench actor was
  `user:alice`, which bypassed nothing on the write side and forced
  `acl.NewFilter(...)` allocation on the read side even though no
  canonical ACL store was wired. ACL overhead is measured separately in
  Tier 1 — that's the right place for it, not here.
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

## Baseline drift on label-gated tiers

Both tiers feed `github-action-benchmark` and write to a `gh-pages`
data path (`dev/bench/tier1` and `dev/bench/tier2`). Tier 1 runs on
every PR + push to `main` so its series is dense — the
`alert-threshold: 150%` comparison fires against the immediately
preceding run, typically the previous day. Tier 2 is heavier and runs
only on push-to-`main`, on tags, and on PRs labelled `bench:postgres`.

The gappy-series footgun: `alert-threshold` compares each new data
point to the **immediately previous one in the series**, not to a
rolling median. If the Tier 2 series goes 6 weeks between push-to-main
runs (because no one shipped a Tier-2-interesting change), then a 50%
regression on the next run compares against 6-week-old numbers — not
the most-recent week's main-branch performance. A real regression
introduced gradually across that window can slip under the 150% gate
without firing.

Workarounds (none yet shipped — call out in a follow-up issue if/when
the drift bites):

1. **Run Tier 2 nightly on `main` from a separate workflow.** Cron a
   `workflow_dispatch` with `tier: 2`; the series gets a data point a
   day even when no PR triggers it. The regression-alert window stays
   tight.
2. **Tighten the Tier 2 threshold** (e.g. `alert-threshold: 120%`) so
   smaller regressions fire — but a noisy Tier 2 run on a noisy CI
   runner already flirts with ±15% on writes, so this trades false
   positives for false negatives.
3. **Switch to a comparison strategy that uses the median of the last
   N runs**, not the previous point. `github-action-benchmark` does
   not support this out of the box; would need a custom analyser.

Tier 1 is not exposed to this problem on the cadence we ship at
(commits land daily). Document the limitation rather than silently
shipping a misleading regression alert.

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
