# ADR-027: Parallel WAL apply across distinct tenants

**Status:** Accepted — implementation (PR #541, issue #140 / PERF-4) is
**gated on the blocking conditions in §Conditions** and MUST NOT merge
until they are met.
**Decided:** 2026-05-17
**Tags:** apply, wal, concurrency, performance, consistency
**Amends:** [ADR-016](016-handlers-append-applier-writes.md) — the
"events apply serially within a partition; no fan-out within the
consumer" clause is narrowed by this ADR (see §Decision).
**Implementation:** `server/go/internal/apply/applier.go`
(`processBatch` / `finalizeBatch` / `routeKey`); `--apply-concurrency`
flag in `server/go/cmd/entdb-server/main.go`. Tracked by PR #541.

## Decision

Within a single WAL poll batch, the applier MAY apply records in
**parallel across distinct tenant route keys**, bounded by
`--apply-concurrency` (default `GOMAXPROCS`; `1` reproduces the old
strictly-serial loop). `routeKey = scope + "\x00" + tenant_id`, so all
records for one tenant — and all global-scope records — each collapse
to a single key and a single worker.

This **amends [ADR-016](016-handlers-append-applier-writes.md)**: its
"events apply serially within a partition … no fan-out within the
consumer" wording is narrowed to *"events for a given tenant route key
apply serially in offset order; fan-out across distinct route keys is
permitted; offset commit remains serial and contiguous-prefix."* The
single-writer-per-tenant-DB and gap-free-offset invariants are
preserved by the mechanism below, not by serial execution.

## Context

The applier applied every record in a poll batch serially in one
goroutine regardless of tenant. Per-tenant SQLite files are fully
independent ([ADR-001](001-storage-architecture.md),
[ADR-014](014-physical-storage-layout.md)) with independent write
mutexes, and the per-record cost is dominated by the SQLite `COMMIT`
fsync, which parallelises trivially across distinct files. So apply
latency scaled with the number of distinct tenants per batch for no
structural reason. This is issue #140 (PERF-4).

Measured (independently reproduced): a real **~2–3.7× speedup at
256–1024-tenant batches**. The PR's "~2.1× at 64 tenants" claim **did
not reproduce** (~1.0–1.3×, within noise); the benchmark harness is
methodologically weak (non-independent timed iterations, busy-wait
barrier) and must not be cited for small-batch numbers.

## Invariants (must hold; verified for PR #541 at review time)

1. **Per-tenant ordering.** One route key → one `groups` entry → one
   worker; `applyChain` consumes that key's records sequentially in
   batch (= partition offset) order. The WAL keys by `tenant_id`, so a
   tenant's records are totally ordered within a partition and
   `PollBatch` returns them in offset order.
2. **Single-writer-per-tenant-DB** ([ADR-016](016-handlers-append-applier-writes.md)
   intent). No two goroutines ever touch the same `tenant_<id>.db`;
   `store.BeginBatch` still takes `poolEntry.writeMu` as an unchanged
   backstop the applier simply never contends.
3. **Gap-free monotonic offset commit.** Only the **serial**
   `finalizeBatch` calls `consumer.Commit` / `UpdateAppliedOffset`,
   strictly in original batch order, and returns at the first
   poisoned record without committing it or anything after it. The
   committed set is therefore exactly the contiguous batch prefix; a
   faster later-tenant worker cannot advance the consumer-group offset
   early because workers never commit. Gap-freeness is provided
   entirely by this ordered early-return — **no apply work may ever
   call `consumer.Commit` from a worker.**
4. **Serialised `global.db` writes.** All cross-tenant/global writes
   funnel through either the single `__global__` route or the serial
   `finalizeBatch`/fanout path. `globalstore` has no write mutex
   (relies on `MaxOpenConns(1)`), so this serialisation is
   **load-bearing**: a future change that parallelises fanout, or
   moves a globalstore write into a parallel tenant txn, silently
   breaks cross-tenant consistency. Stated here as a hard invariant.

## Accepted consequence

On a **poisoned batch**, the intermediate on-disk SQLite state differs
from the old serial loop: speculatively-applied later-tenant records
are materialised before the halt. The **converged post-restart state
is identical** — the consumer-group offset sits at the poison, the
speculative tail is re-delivered and idempotency-SKIPped on replay.
This is acceptable because, per
[ADR-016](016-handlers-append-applier-writes.md), SQLite is a derived
view with no external consumer reading it mid-halt. Recorded here as a
deliberate, accepted behavioural change (not a footnote).

## Conditions (blocking — PR #541 does not merge until all are met)

1. **This ADR** recording the ADR-016 amendment, the four invariants,
   and the accepted consequence. (Done.)
2. **Wire `--apply-concurrency`** in `cmd/entdb-server/main.go`
   (default `GOMAXPROCS`, `1` = serial) so operators have a
   no-redeploy kill-switch for a consistency-sensitive default-on
   change. Today it is only an `Options` field.
3. **Add a same-partition multi-tenant poison test**: two tenants that
   hash to the *same* WAL partition with a poison from tenant B
   sandwiched between tenant A records, asserting contiguous-prefix
   commit and idempotent resume.
4. **Restate the benchmark honestly** in the PR: ~2–3.7× at
   256–1024-tenant batches; **drop the "~2.1× at 64 tenants" claim**
   (did not reproduce).

## Open question (non-blocking; track with an e2e follow-up)

Multi-replica / consumer-group **rebalance** mid-batch: on rebalance,
partitions can move to another applier replica that re-polls from the
last broker-committed offset; in-flight speculative applies become the
other replica's redelivery. This stays idempotent (same SKIP path) and
per-tenant ordering holds **only because per-tenant records never split
across partitions** — but that argument now spans two appliers and is
**untested** (the e2e suite uses a single applier). Safe for today's
single-applier deployment; must be validated before multi-replica
appliers ship.

## Alternatives considered

- **Keep strictly-serial apply.** Rejected — leaves a real 2–3.7×
  at-scale win unrealised for no correctness benefit.
- **Parallelise within a tenant** (multiple workers per tenant DB).
  Rejected — breaks per-tenant ordering and the single-writer
  invariant.
- **Abort the whole batch on first poison** (instead of committing the
  contiguous good prefix). Rejected — would discard already-applied
  independent-tenant work and cost most of the parallelism; the
  contiguous-prefix + idempotent-replay model is strictly better and
  is the existing serial contract generalised across tenants.

## Consequences

**Locks in:** the rule that *commit happens only in the serial
finalize loop, in batch order, halting at the first poison* — this is
the single load-bearing decision; a code comment at the finalize
commit site MUST state that committing from anywhere else breaks
gap-freeness.

**Makes easy:** apply throughput scales with available cores on
many-tenant batches without weakening per-tenant ordering or the
single-writer guarantee.

**Makes harder:** future changes near `global.db` writes or the
commit path must respect invariants 3–4 explicitly; the implicit
serialisation that previously made them safe by construction is gone.

## References

- [ADR-016](016-handlers-append-applier-writes.md) — amended by this
  ADR (fan-out clause narrowed); single-writer + handlers-append
  intent preserved.
- [ADR-001](001-storage-architecture.md),
  [ADR-014](014-physical-storage-layout.md) — per-tenant SQLite files
  are independent, which is what makes cross-tenant parallel apply
  sound.
- [ADR-005](005-event-sourcing-wal.md) — WAL partitioning by
  `tenant_id` underpins per-tenant ordering.
- Issue #140 (PERF-4); PR #541.
