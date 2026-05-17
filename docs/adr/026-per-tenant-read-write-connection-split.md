# ADR-026: Per-tenant read/write SQLite connection split

**Status:** Accepted — implemented (PR #536, issue #137 / PERF-1); the
conditions in §Conditions are met. **Landed dark:** the split ships
**off by default** (`--read-pool-size=1`); enabling it in production is
an explicit operator opt-in, gated on idle-tenant eviction
(canonical-store open-question 2). The root-cause broadcast-ordering
fix (condition 1) is unconditional and always active.
**Decided:** 2026-05-17
**Tags:** store, concurrency, performance, sqlite, read-after-write
**Implementation:** `server/go/internal/store/{pool,canonical}.go`;
SELECT-only methods in `nodes.go, access.go, acl.go, visibility.go,
edges.go, offset.go, fts.go, idempotency.go`; `--read-pool-size` flag
in `server/go/cmd/entdb-server/main.go`. Tracked by PR #536.

## Decision

Each per-tenant SQLite database is opened with **two** `*sql.DB`
handles:

- The existing **write handle** — `SetMaxOpenConns(1)` + `writeMu` +
  `BEGIN IMMEDIATE`, unchanged byte-for-byte. It remains the sole
  writer. [ADR-016](016-handlers-append-applier-writes.md) (only the
  applier writes SQLite) is fully preserved.
- A new **read handle** — a pooled, physically read-only connection
  set (`SetMaxOpenConns(readPoolSize)`), opened with SQLite
  `mode=ro` + `PRAGMA query_only=ON` on the modernc path and
  `&mode=ro&_query_only=true` on the SQLCipher path (the per-tenant
  key is still supplied).

Pure-SELECT store methods route to the read handle; every
`INSERT/UPDATE/DELETE` and all DDL stay on the write handle. The split
activates only when `WALMode && readPoolSize > 1`; otherwise the read
handle is nil and reads fall back to the single write handle (behaviour
identical to pre-#137).

## Context

`pool.go` opened each tenant `*sql.DB` with `SetMaxOpenConns(1)`,
shared by the applier and all readers. The applier holds that one
connection for the whole `BatchTxn` (`BEGIN IMMEDIATE` … `COMMIT`), so
**same-tenant reads serialise** — behind each other and behind any
in-flight write. SQLite WAL mode natively supports one writer plus many
concurrent readers, so this leaves throughput on the floor.
`docs/go-port/shared/canonical-store.md` open-question 5 marked the
`SetMaxOpenConns(1)` vs N decision **benchmark-gated** ("decide via
benchmark, not vibes") and flagged the specific hazard: `INSERT OR
REPLACE` + lazy DDL under concurrent readers.

Measured (Apple M4 Max, same-tenant parallel `GetNode`,
reproduced independently): **~2.8× read throughput**
(≈10.4 µs/op → ≈3.7 µs/op), stable across runs. The win is real and
the design direction is sound.

Two correctness facts were verified against driver source and the
apply path, and they hold:

- The read handle is **physically read-only** on both drivers
  (`mode=ro` honoured at the VFS; `query_only` parsed to
  `PRAGMA query_only=1`). Concurrent readers therefore cannot race
  `INSERT OR REPLACE` — they cannot write at all. Open-question-5's
  hazard is genuinely closed.
- The applier reads its own uncommitted writes **only** through
  `tx.Conn()` (the write-txn connection), never through a re-routed
  SELECT method. No in-flight uncommitted row is ever read via the
  read pool inside an apply transaction.

## The correctness regression this unmasks (first-class consequence)

The split removes an **implicit connection-serialisation** that was
masking a pre-existing latent race in the read-after-write path:

`applyEvent` calls `UpdateAppliedOffsetTx`, which executes
`offsetCond.Broadcast()` (waking `WaitForOffset(N)` waiters) **before**
`tx.Commit()`. The code itself notes a strictly-correct implementation
would defer the notify to commit.

- **Pre-#536:** a woken reader used the single write connection, which
  the applier's `BatchTxn` held until post-`COMMIT` cleanup. The
  reader physically blocked until the write committed, then saw the
  data. The early broadcast was harmless — the connection bottleneck
  masked it.
- **Post-#536:** the woken reader uses an independent read connection
  and runs its `SELECT` immediately. If it lands in the
  broadcast→commit window, its WAL snapshot **excludes the uncommitted
  write** → the client reads its own confirmed write back as **stale
  or `Found=false`**.

This is **not an edge case**: the Python SDK auto-attaches the last
write's offset as `after_offset` on every subsequent
`GetNode`/`GetNodeByKey`/`GetNodes`, and those handlers do a bare
`WaitForOffset` with **no idempotency poll**. It is the default
read-your-write path. CI is green only because write RPCs
(`ExecuteAtomic`) additionally poll an idempotency record that commits
in the *same* `BatchTxn` as the data, and every integration/e2e test
uses that protected write path; the exposed read path has no test.

## Conditions (met before merge)

1. **Move `offsetCond.Broadcast()` to after `BatchTxn.Commit()`**
   (root-cause fix; also fixes the pre-existing latent bug).
   Unconditional and always active regardless of `--read-pool-size`.
2. **Add a regression test**: a write driven through the WAL→applier
   path, fenced by `after_offset`/`WaitForOffset`, asserting the
   re-routed `GetNode` observes the write under concurrent apply. It
   must fail on the pre-fix branch and pass after condition 1.
3. **Accurate rollout posture.** The split ships **off by default**
   (`--read-pool-size=1`) — a deliberate dark landing. Read-after-write
   correctness is fixed by condition 1, but the resource envelope
   (extra FDs + page cache per tenant, no idle-tenant eviction —
   canonical-store OQ-2) is unresolved, so the split is an explicit
   operator opt-in pending that work. Flag help and PR narrative state
   off-by-default / opt-in, and that read-your-writes via
   `WaitForOffset` is *weakened then re-fixed by condition 1* — not
   "preserved".

## Alternatives considered

- **Keep `SetMaxOpenConns(1)`.** Rejected — leaves a measured ~2.8×
  same-tenant read win unrealised; the open question explicitly asked
  for the benchmark, which now exists.
- **Route only `after_offset`-fenced reads back to the write handle**
  (keep the masking). Rejected — that defeats the perf win on exactly
  the reads people fence, and preserves the latent bug instead of
  fixing it.
- **Ship the split now, fix the broadcast race later.** Rejected — it
  is a live, default-path read-after-write correctness regression; the
  fix is small and must land with it.

## Consequences

**Locks in:** per-tenant resource cost rises — each materialised read
connection is an extra OS file descriptor and carries its own
`PRAGMA cache_size = -64000` (up to 64 MiB page cache). With no
idle-tenant eviction (canonical-store.md open-question 2 is still
unresolved — handles live for the process lifetime), worst case is
~9 FDs and up to several hundred MiB page-cache **per active tenant**.

**Makes easy:** concurrent same-tenant reads (dashboards, fan-out
queries, list endpoints) no longer queue behind each other or behind
the applier.

**Makes harder / follow-ups (tracked in this ADR, non-blocking):**

- Pair with the long-deferred idle-tenant LRU eviction
  (canonical-store.md OQ-2) before large-tenant-count GA; reconsider
  the default pool size and per-read-conn `cache_size`.
- `go test -race ./internal/api/` hangs on a pre-existing
  `transferFixture` nil-channel send (unrelated to this change).
  "Full CI green under `-race`" cannot be claimed until that fixture
  is fixed; track separately.

**Failure modes:** if condition 1 is not met, the documented
read-after-write regression is live by default. With condition 1 met,
`WaitForOffset(N)` signals only once offset N's data is committed and
visible on any connection, closing the window for both the read pool
and the pre-existing latent path.

## References

- [ADR-016](016-handlers-append-applier-writes.md) — only the applier
  writes SQLite; the write path is untouched by this ADR.
- `docs/go-port/shared/canonical-store.md` — open-question 5
  (read/write split, benchmark-gated) and open-question 2 (idle-tenant
  eviction).
- Issue #137 (PERF-1); PR #536.
