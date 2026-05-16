# Applier — Go port spec (shared)

EPIC #407 — Python → Go server port. The Applier is the WAL consumer that
materialises `TransactionEvent`s into per-tenant SQLite. CLAUDE.md flags
it as the most intricate piece to port. This spec captures the contract
the Go implementation must satisfy.

Reference Python implementation:
[`server/go/internal/apply/applier.go`](../../../server/go/internal/apply/applier.go).

Target Go package: `server/go/internal/apply/` (new). Sibling packages
referenced below: `server/go/internal/wal/`, `server/go/internal/store/`,
`server/go/internal/pb/`, `server/go/internal/errs/`,
`server/go/internal/telemetry/`.

---

## Roles & responsibilities

The Applier is a long-running consumer with five jobs, in order:

1. **Consume** records from the WAL via `WalStream.Subscribe` /
   `PollBatch` (`applier.py:418-468`). One Applier instance consumes one
   `(topic, group_id)` pair; a node may run several with disjoint
   `assigned_tenants` filters (`applier.py:483-489`).
2. **Validate + dispatch by op-type**. Inspect `event.ops`, classify
   storage mode (`_event_storage_mode`, `applier.py:629-667`), open the
   correct physical SQLite file (tenant / mailbox / public —
   `_open_event_connection`, `applier.py:669-691`), then for each op
   call the matching write helper on `CanonicalStore` (the giant
   `if/elif` ladder in `_sync_apply_event_body`, `applier.py:929-1248`).
3. **Write SQLite atomically**. All ops in one event execute inside
   a single `BEGIN IMMEDIATE` … `COMMIT` block opened via
   `canonical_store.batch_transaction()` (`canonical_store.py:1264-1289`).
   The `applied_events` row is inserted in the same transaction
   (`applier.py:1250-1262`). Idempotency check happens **inside** the
   transaction before any mutation (`applier.py:921-927`).
4. **Advance the WAL offset**. Only on success and only per partition
   that fully succeeded (`applier.py:541-557`). Read the partition-
   commit logic carefully — it is the load-bearing correctness hook.
5. **Notify offset waiters / build receipts**. After commit, call
   `canonical_store.update_applied_offset(tenant_id, stream_pos)`
   (`applier.py:446-449`, `applier.py:597-602`) which wakes
   `wait_for_offset` (`canonical_store.py:478-512`). The
   `(stream_pos, status)` pair is what `ExecuteAtomic` returns to the
   caller as a receipt.

Side responsibilities, in declining importance:

- **Mailbox fanout** after commit (`applier.py:1361-1368`,
  `_fanout_node`, `applier.py:1615-1668`). Writes notification rows in
  the tenant DB; fanout failure is logged but never fails the apply.
- **Shared-index maintenance** for cross-tenant share/revoke/group-
  member changes (`applier.py:1687-1845`). All best-effort against
  `GlobalStore`.
- **Quota counter increment** (`_increment_usage_safe`,
  `applier.py:1306-1325`). Strictly fire-and-forget.
- **Lazy index/FTS bootstrap** — first write of a new type creates the
  unique/query/composite/FTS expression indexes (`applier.py:946-954`,
  `1001-1012`).

---

## Determinism contract

`tests/python/integration/test_wal_replay_determinism.py` is the bedrock
contract. Three properties are asserted:

1. **Round-trip determinism** (`test_wal_replay_determinism.py:179-382`).
   Drive a fixed sequence of mixed ops (create_node × 3, update_node,
   create_edge, delete_edge, admin_transfer_content, create_edge after
   transfer), `_canonical_dump` the state, wipe SQLite, replay from
   offset 0 with a fresh applier and a new `group_id`, dump again. The
   two dumps **must compare equal as Python dicts**, including
   `payload_json` byte-for-byte (`:128-136`).
2. **Idempotency on replay** (`:385-440`). Two appends of the same
   `idempotency_key` produce **exactly one** node. Detection is the
   `SELECT 1 FROM applied_events WHERE tenant_id=? AND
   idempotency_key=?` probe inside the transaction
   (`applier.py:766-772`, `:921-927`).
3. **Halt on poison** (`:443-549`). A `USER_MAILBOX` create_node with no
   `target_user_id` raises `ValidationError` in
   `_event_storage_mode`. With `halt_on_error=True` (production
   default) the applier:
   - applies the preceding good event,
   - does NOT advance the WAL past the poisoned record,
   - does NOT apply any subsequent record.

The Go applier MUST replicate these byte-level outcomes. Specifically:

- `payload_json` is stored as the JSON encoding of the input payload
  dict. Use a JSON encoder that emits the same key order / spacing as
  Python's `json.dumps(obj)` with default args. **Open question:** the
  determinism test compares `payload_json` strings — the Go encoder
  must produce identical output (no whitespace, sort-by-insertion or
  sort-by-key matching Python). See **Open questions / risks** below.
- The `applied_events` row uses `int(time.time() * 1000)` for
  `applied_at` (`applier.py:891`, `:1260`). This is **not** part of
  the determinism dump (the dump only reads nodes + edges) but
  `WaitForOffset` consumers depend on it.
- Generated node IDs (when an op omits `id`) come from
  `canonical_store.create_node_raw`. The determinism test pins ids
  via `op["id"]` so the Go port doesn't need byte-identical UUIDs —
  but it MUST honour caller-supplied ids verbatim.

Halt semantics, precisely (`applier.py:427-443`, `:519-572`):

- A failed apply MUST NOT call `wal.commit(record)`.
- A failed apply MUST set `_running = false` when `halt_on_error` is
  true. The loop returns; the caller (supervisor) decides whether to
  surface or restart.
- In the batched path, partition-level commit is gated on every record
  in that partition succeeding (`applier.py:541-557`). A failure on
  partition 3 must not block commits on partitions 0–2.

---

## Concurrency model

Python's applier is a single asyncio task per `(topic, group_id)`. The
synchronous SQLite work is offloaded to a `ThreadPoolExecutor` owned by
the `CanonicalStore` (`canonical_store.py:417-425`):

- `apply_event` builds the `TransactionEvent`, then calls
  `loop.run_in_executor(self.canonical_store._executor,
  self._sync_apply_event_body, event)` (`applier.py:1349-1354`).
- `_apply_tenant_batch` does the same with
  `_sync_apply_tenant_batch_body` (`applier.py:586-592`).
- The executor is shared with all read RPCs. Per-tenant locks
  (`canonical_store.py:410-412`) and per-tenant async semaphores
  (`canonical_store.py:413-416`) serialise same-tenant writers and
  cap fair-share of the pool.

Reader/writer interaction is exercised by
`tests/python/integration/test_concurrent_applier_reads.py`. The
contract (`:104-136`, `:139-`):

- Readers running `get_node` / `get_edges_from` / `get_nodes_by_type`
  in tight loops MUST NEVER observe a half-applied transaction. An
  edge whose `to_node_id` resolves to a missing node is a violation.
- Per-event atomicity is delivered by the single SQLite `BEGIN
  IMMEDIATE` … `COMMIT` block. `BEGIN IMMEDIATE` takes a RESERVED
  lock; readers in WAL mode keep going against the pre-commit
  snapshot.
- No deadlocks under bursts of 100 writes × 10 readers × 2× pool
  workers.

**Go implications:**

- Python's GIL hides ordering bugs that Go preemption will surface.
  The test is harder to satisfy in Go; budget for it.
- The natural Go equivalent of `run_in_executor` is a bounded worker
  pool of goroutines, each owning an `*sql.DB` with `MaxOpenConns=1`
  per tenant DB file (SQLite is serialised internally anyway).
- Per-tenant batching: keep the `_apply_tenant_batch` design — group
  records by `tenant_id`, apply each tenant's slice in one
  transaction, commit per-partition only on full success
  (`applier.py:575-614`). Mixed-storage-mode batches must fall back
  to single-event dispatch (`applier.py:743-751`).

---

## Recovery tiers

Recovery is **not** part of the steady-state Applier. It runs at boot
or on tenant rebuild via
`server/go/internal/tools/recovery_strategy.go` (`RecoveryTier`,
`:38-43`):

1. **Tier 1 — SNAPSHOT**: Find the latest `s3://…/snapshots/tenant=…/
   ts=…sqlite.gz` (`snapshot/snapshotter.py:1-50`). Decompress, restore
   to `data_dir/<tenant>.db`, read manifest's `last_stream_pos` and
   resume the WAL from that offset.
2. **Tier 2 — KAFKA_WAL**: Drive the Applier from the snapshot's
   `last_stream_pos` forward. This is just the steady-state apply
   loop (`recovery_strategy.py:391-414`).
3. **Tier 3 — S3_ARCHIVE**: When records have aged out of Kafka, pull
   them from `archive/archiver.py` (`:300-385`), feed them into the
   Applier as if they came from the WAL, then transition to Tier 2.

The Go applier needs a `Replay(ctx, tenantID, fromPos)` entry point
distinct from `Run(ctx)` so the recovery harness can drive batches
without a live WAL subscription. Snapshot manifests carry the resume
offset — Tier 1 must hand it to Tier 2 atomically.

The Snapshotter itself uses SQLite's online backup API
(`snapshotter.py:88-117`) — out of scope for this spec but the Applier
must not hold a write transaction across snapshot start, otherwise
backup latency spikes.

---

## Receipt construction

`ExecuteAtomic` (`api/grpc_server.py:620-826`) is the only RPC that
produces a write receipt. Flow:

1. Handler validates the request, builds a `TransactionEvent`, calls
   `wal.append(...)` and gets back a `StreamPos`.
2. Handler waits via `_wait_for_offset(tenant, stream_pos, timeout)`
   (`grpc_server.py:806`, `:937-944`) which delegates to
   `canonical_store.wait_for_offset` (`canonical_store.py:478-512`).
3. The Applier processes the record asynchronously, commits SQLite,
   then calls `update_applied_offset` (`applier.py:446-449`,
   `:597-602`) which `notify_all`s the waiter.
4. Handler returns `ExecuteAtomicResponse{stream_pos, status,
   error_trail?}` (`grpc_server.py:806-819`).

The receipt's three observable fields:

- `stream_pos` — `topic:partition:offset` string from
  `StreamPos.__str__` (decoded by `_parse_stream_offset`,
  `canonical_store.py:90-100`).
- `status` — APPLIED / SKIPPED / FAILED. SKIPPED is set when the
  idempotency check inside the transaction hits an existing row
  (`applier.py:925-927`).
- `error_trail` — populated from `ApplyResult.error` on failures
  (`applier.py:289`, `:1387-1392`). Today it's the stringified
  exception. Go port should categorise (see **Open questions** below).

`GetReceiptStatus` (`grpc_server.py:946-968`) is a separate RPC that
just looks up the `applied_events` row by `idempotency_key`. The
Applier does not need to do anything special for it beyond writing the
row inside the apply transaction.

`WaitForOffset` is documented in
[`docs/go-port/rpcs/WaitForOffset.md`](../rpcs/WaitForOffset.md).

---

## Field-id payload encoding

Per CLAUDE.md invariant 6: payloads are stored keyed by `field_id`
(stringified ints) on disk, e.g. `{"1": "alice@example.com", "2":
"Alice"}`. The Applier sees field-id-keyed payloads in the WAL — no
translation happens here. Translation between names and ids is the
gRPC boundary's job (`api/grpc_server.py`).

The Applier touches `data` / `patch` keyed by field-id strings in:

- `create_node_raw` (`applier.py:782-791`, `:956-1012`) — passed
  through to `CanonicalStore.create_node_raw` and dumped as
  `payload_json`.
- `update_node` patch merge (`applier.py:805-813`, `:1029-1073`) —
  the patch is `dict.update`d onto the existing payload by
  string key; field-id renames are therefore free.
- Unique-constraint violation reporting (`applier.py:967-998`,
  `:1075-1102`) — `dup_value = data.get(str(dup_field_id))` looks up
  the colliding value by stringified field-id.
- FTS indexing (`applier.py:1001-1012`, `:1108-1120`) — accepts the
  field-id-keyed payload directly.

**Go port:** the wire `data` field is a `google.protobuf.Struct` or
JSON `map<string, Value>`. Decode it as `map[string]any` and pass
through unchanged. The on-disk form is `json.Marshal` of that map.
This is the determinism-critical encoding — see open questions.

---

## Go design

Proposed package layout (`server/go/internal/apply/`):

```
apply/
  applier.go          // type Applier, Run(ctx), Replay(ctx, ...)
  event.go            // type TransactionEvent (mirrors applier.py:204-269)
  result.go           // type ApplyResult (applier.py:272-290)
  storage_route.go    // _event_storage_mode + _open_event_connection
  ops_node.go         // create/update/delete_node handlers
  ops_edge.go         // create/delete_edge + ACL inheritance
  ops_admin.go        // admin_transfer_content, admin_revoke_access
  alias.go            // _resolve_ref / _resolve_node_ref
  errors.go           // ApplierError, ValidationError, UniqueConstraintError
  fanout.go           // _fanout_node + _generate_snippet
  shared_index.go     // shared-index hooks (best-effort)
```

### Single goroutine vs per-tenant worker pool

The Python applier is a **single asyncio task** that offloads SQLite
work to a shared pool. Two viable Go shapes:

| Option | Pros | Cons |
|---|---|---|
| **A. Single consumer goroutine + shared worker pool (Python parity)** | Mirrors Python exactly; simplest mental model; easy to satisfy halt-on-error (one place to stop); per-partition commit logic stays simple. | One slow tenant blocks the consume loop until its batch finishes (already the case in Python). |
| **B. Per-tenant worker goroutine, fan-out from consume loop** | Hot tenants can't head-of-line block cold ones; matches Kafka's per-partition consumer model. | Doubles the partition-commit complexity (per-tenant offset bookkeeping); poison-event halt becomes per-tenant, harder to reason about. |

**Recommendation: start with A** for parity with the Python reference
(makes the determinism + halt tests trivially equivalent). Move to B
only if benchmarks show consume-side starvation.

### Locks needed (option A)

- `*sync.Mutex` per tenant for write serialisation, mirroring
  `_tenant_locks` (`canonical_store.py:410-412`). The Go store layer
  owns these, not the Applier.
- `*sync.Cond` (or `chan struct{}` per tenant) for `wait_for_offset`
  fan-out, mirroring `_offset_conditions`
  (`canonical_store.py:413-416`, `:455-512`).
- `*sync.Map[string]string` for `_applied_offsets`
  (`canonical_store.py:429`).
- `_node_alias_map` (`applier.py:375`) is **per-event**; in Go make it
  a local `map[string]string` inside `applyEvent`, not a field. The
  Python field is reset at every event entry (`applier.py:776`,
  `:911`) — bug-prone, don't replicate the global mutable state.

### Context cancellation

Every batch / event apply should respect `ctx.Done()`. SQLite calls
themselves don't honour context — wrap the goroutine entry so that
on cancel the in-flight transaction either completes or rolls back.
Long replays (Tier 2/3 recovery, hours of WAL) MUST be cancellable.

---

## Dependencies

| Go package | Python equivalent | Used for |
|---|---|---|
| `internal/pb` | `entdb.v1` proto + `apply.applier.TransactionEvent` | Op message types, payload Struct decoding |
| `internal/wal` | `wal/base.WalStream` | `Subscribe`, `PollBatch`, `Commit` |
| `internal/store` | `apply.canonical_store.CanonicalStore` | `BatchTransaction`, `CreateNodeRaw`, `EnsureFieldIndexes`, `UpdateAppliedOffset`, `WaitForOffset`, `BatchCreateNotifications` |
| `internal/store/global` | `global_store.GlobalStore` | `IncrementUsage`, `AddShared`, `RemoveShared`, `CleanupStaleShared` |
| `internal/schema` | `schema.registry.SchemaRegistry` | `GetUniqueFieldIDs`, `GetIndexedFieldIDs`, `GetSearchableFieldIDs`, `GetCompositeUniqueConstraints`, `GetDataPolicy` |
| `internal/errs` | `apply.applier.ApplierError` hierarchy | Typed error returns at the package boundary |
| `internal/telemetry` | `metrics.record_applier_event` | Prometheus counters: `applier_events_total{result}` |
| `internal/auth/acl` | `apply.acl.AclManager` | Mostly read-side; applier holds it but doesn't write to it directly |

---

## RPCs that depend on it

- **`ExecuteAtomic`** (`docs/go-port/rpcs/ExecuteAtomic.md`,
  `grpc_server.py:620-826`). Write path: append to WAL, wait for
  Applier to commit, return receipt. Direct dependency on the
  `applied_offsets` notification fan-out.
- **`GetReceiptStatus`** (`docs/go-port/rpcs/GetReceiptStatus.md`,
  `grpc_server.py:946-968`). Reads `applied_events` table. Depends on
  the Applier writing the row inside the apply transaction so a
  successful response implies the data is durably visible.
- **`WaitForOffset`** (`docs/go-port/rpcs/WaitForOffset.md`). Thin
  wrapper around `canonical_store.wait_for_offset`. Depends on the
  Applier calling `update_applied_offset` after every successful
  commit.

Indirectly, **every read RPC** that wants read-after-write consistency
(GetNode, QueryNodes, GetEdgesFrom, …) eventually calls
`WaitForOffset`, so anything that breaks the offset notification
breaks the whole read path.

---

## Open questions / risks

1. **JSON encoding determinism.** Python `json.dumps(obj)` with default
   args emits compact JSON with insertion-order keys. Go's
   `encoding/json` sorts map keys alphabetically. The replay test
   compares `payload_json` strings (`test_wal_replay_determinism.py:128-
   136`). **Decision needed:** either (a) change the Python side to
   use sorted keys before the Go port lands, or (b) implement a
   custom encoder on the Go side that preserves wire-order. Option (a)
   is safer; do it as a pre-port migration with a one-shot rewrite of
   existing rows.

2. **SQLite driver choice.** Two contenders:
   - `modernc.org/sqlite` — pure Go, no cgo, easy cross-compile, but
     ~30% slower on write-heavy benchmarks and lags upstream SQLite
     by 1–2 minor versions.
   - `github.com/mattn/go-sqlite3` — cgo binding to upstream SQLite.
     Faster, full feature parity (FTS5, JSON1), but cgo complicates
     static builds and Docker image size.
   Recommendation: **`mattn/go-sqlite3`** for production parity with
   Python's `sqlite3` (which is also a C binding). Accept the cgo
   tax. Revisit if static binary becomes a hard requirement.

3. **Thread safety with `mattn/go-sqlite3`.** `*sql.DB` is goroutine-
   safe but each connection is single-threaded. Must set
   `db.SetMaxOpenConns(1)` per tenant DB or use the shared pool with
   careful lock ordering. The per-tenant lock from canonical_store
   covers this.

4. **Context cancellation through long replays.** Tier-3 archive
   replays can run for minutes. Pattern: chunk archive batches to
   ≤1k events, check `ctx.Err()` between batches, store progress in
   `applied_events` so a cancelled replay resumes idempotently.

5. **Error categorisation.** Today Python lumps everything into
   `ApplyResult.error = str(e)` (`applier.py:1391`). The Go port
   should distinguish:
   - **Poisoned event** (`ValidationError`, `SchemaFingerprintMismatch`,
     malformed JSON) — halt, surface as `INVALID_ARGUMENT` in the
     receipt, never retry.
   - **Constraint violation** (`UniqueConstraintError`,
     `CompositeUniqueConstraintError`) — return `ALREADY_EXISTS` to
     the caller (`grpc_server.py` already does this), do **not**
     halt; this is a normal client-error path, the WAL offset
     should advance.
   - **Transient infra** (SQLite I/O, disk full, lock timeout) —
     halt and let the supervisor restart; do not advance offset.
   The current `halt_on_error=True` catch-all is too coarse; the Go
   port has a chance to fix this. Wire it as a typed-error switch in
   the consume loop.

6. **`acl_inherit` cycle detection** uses a recursive CTE bounded at
   depth 10 (`applier.py:851-870`, `:1193-1212`). Verify
   `mattn/go-sqlite3` supports recursive CTEs (it does, but worth
   checking with an integration test in the port).

7. **`_node_alias_map` global state.** The Python field is reset at
   every event entry but is still an instance attribute, which means
   concurrent events on the same Applier instance would race. In
   Python this is masked by the single-task design. The Go port must
   make this strictly per-event (local var). Document explicitly.

8. **Mailbox fanout outside the transaction.** `_fanout_node`
   (`applier.py:1361-1368`) runs after the apply transaction commits.
   A crash between commit and fanout drops notifications. Today this
   is accepted (notifications are best-effort). Confirm the Go port
   makes the same trade-off explicitly rather than silently.

9. **Quota counter (`_increment_usage_safe`)** is fire-and-forget per
   ADR `docs/decisions/quotas.md`. The Go port should preserve this:
   billing drift is preferable to a write outage. Use a buffered
   channel + best-effort drain on shutdown.

10. **Storage-mode mixed batches.** The batched path explicitly
    rejects mixed-mode events (`applier.py:743-751`) and falls back
    to per-event apply (`applier.py:510-533`). The Go port must
    keep this fallback or the test
    `test_applier_halts_on_poisoned_event` will fail intermittently.
