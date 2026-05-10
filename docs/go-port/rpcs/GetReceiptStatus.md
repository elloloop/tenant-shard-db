# GetReceiptStatus — Go port spec

EPIC: #407 (Python -> Go server reimplementation).

`entdb.v1.EntDBService.GetReceiptStatus` is the read-side companion to
`ExecuteAtomic`. Clients poll it with the `idempotency_key` they supplied at
write time to learn whether the WAL event has been materialized into the
tenant's SQLite view yet. It is a pure read against `applied_events`, never
touches the WAL, and never mutates state.

## Wire contract

- Proto: `proto/entdb/v1/entdb.proto:52` (rpc), `:375-389` (messages + enum).
- Request `GetReceiptStatusRequest`:
  - `RequestContext context = 1` — `tenant_id`, `actor`, `trace_id`. `actor` is UNTRUSTED (see Auth).
  - `string idempotency_key = 2` — the same key the caller passed to `ExecuteAtomic`.
- Response `GetReceiptStatusResponse`:
  - `ReceiptStatus status = 1` — `UNKNOWN(0) | PENDING(1) | APPLIED(2) | FAILED(3)`.
  - `string error = 2` — populated only when `status == UNKNOWN` (handler-internal exception, see Error contract).
- The Python handler **never aborts** on application-level failure; it returns
  `UNKNOWN` + `error`. Only `_check_tenant` may abort (UNAVAILABLE / FAILED_PRECONDITION).
  Go must preserve this asymmetry — contract tests assert it.

## Auth (Permission, trusted-actor)

Per server/python/entdb_server/api/grpc_server.py:946-971 there is **no**
permission check, no `Permission` enum lookup, no node-level ACL evaluation,
and no use of `_resolve_trusted_actor`. The handler:

1. Calls `_check_tenant(tenant_id, ctx)` (grpc_server.py:362-410) — sharding
   ownership + region pinning. This is the only ingress gate.
2. Reads `applied_events` for `(tenant_id, idempotency_key)`.

`request.context.actor` is read off the wire but **never used** for
authorization. The Go port MUST NOT add a permission check here — doing so
would tighten the contract and break SDK polling flows that pass
`actor="system"` (sdk/python/entdb_sdk/_grpc_client.py:863).

Trusted-actor invariant: there is no actor-derived effect. No actor is logged
into the WAL by this RPC (it doesn't write).

## Side effects

None. Pure read of `applied_events` via
`canonical_store.check_idempotency` (server/python/entdb_server/apply/canonical_store.py:1377-1405).
No WAL append, no SQLite write, no global_store write, no fanout.

Cross-cutting: `record_grpc_request("GetReceiptStatus", "ok"|"error", elapsed)`
metric (grpc_server.py:964,967). Go must emit the equivalent label.

## Error contract (exhaustive)

| Cause | Code | Body |
| --- | --- | --- |
| Tenant not owned by this node, owner known | `UNAVAILABLE` + trailer `entdb-redirect-node: <owner>` | aborted; "Tenant '<id>' is not served by this node (try node <owner>)" |
| Tenant not owned by this node, owner unknown | `UNAVAILABLE` | aborted; "Tenant '<id>' is not served by this node" |
| Tenant pinned to region != served_region | `FAILED_PRECONDITION` | aborted; region message |
| Any other exception (SQLite IO, panic in `check_idempotency`) | `OK` | `status=UNKNOWN`, `error=<str(exc)>` |
| Idempotency key not in `applied_events` | `OK` | `status=PENDING`, `error=""` |
| Idempotency key in `applied_events` | `OK` | `status=APPLIED`, `error=""` |

Notes:
- The Python handler's bare `except Exception` (grpc_server.py:966) means
  even programmer errors collapse to `UNKNOWN`. Go should match this for
  parity (recover panics in handler, return `status=UNKNOWN`,
  `error=err.Error()`); resist the urge to return `Internal`.
- `RECEIPT_STATUS_FAILED` is **never produced** by the Python server today —
  the enum value exists for future use (and the Python SDK maps it for
  forward compatibility, _grpc_client.py:878). The Go port should leave it
  unproduced unless EPIC #407 explicitly adds a failed-receipt table.
- `error` field is mutually exclusive with non-UNKNOWN status.

## Shared Go package deps

(Names suggested — actual layout TBD by EPIC #407 shared-pkg spec.)

- `internal/shard` — sharding registry, exposes `IsMine(tenantID) bool` and `Owner(tenantID) (string, bool)`. Mirror of `self._sharding`.
- `internal/region` — served-region check; mirrors `self.served_region` + `global_store.get_tenant`.
- `internal/store/canonical` — `CheckIdempotency(ctx, tenantID, idemKey) (bool, error)` against per-tenant SQLite. Maps to canonical_store.py:1391.
- `internal/metrics` — `RecordGRPC(method, outcome, duration)` (Prometheus histogram, same labels as Python).
- `internal/grpcutil` — `AbortWithRedirect(ctx, code, msg, ownerNode)` helper that sets the `entdb-redirect-node` trailer **before** returning the status (grpc_server.py:387-395 ordering matters; SDKs depend on it).
- Generated proto pkg: `sdk/go/entdb/internal/pb` already exists; the server should use a sibling generated package (e.g. `server/go/internal/pb`).

No crypto, no auth, no schema_registry deps — this RPC is intentionally minimal.

## Other-RPC deps (especially ExecuteAtomic interplay)

- **ExecuteAtomic** (grpc_server.py:~600-828) is the only producer of
  rows in `applied_events`. The applier's `apply_with_idempotency`
  (canonical_store.py:1454-1505) inserts the row atomically with the SQLite
  mutations under `BEGIN IMMEDIATE`. Therefore: a row visible to
  `GetReceiptStatus` ⇒ all materialized writes for that event are also
  visible. Go must preserve this **single-transaction** insert-or-skip — do
  not split idempotency recording from the data write.
- **WaitForOffset** (grpc_server.py:973) is the lower-level synchronization
  primitive used by `ExecuteAtomic`'s `wait_applied=true` path. Clients that
  set `wait_applied` get `applied_status` in the response and typically don't
  need to call `GetReceiptStatus`. `GetReceiptStatus` is the fallback for
  fire-and-forget writers and for retries after a network error where the
  writer never saw the receipt.
- **Applier** (server/python/entdb_server/apply/applier.py) consumes the WAL
  and calls `apply_with_idempotency`. The receipt becomes visible only after
  the applier processes the event — so a `PENDING` here means "WAL accepted,
  applier hasn't caught up" (or "key was never issued"; the two are
  indistinguishable, see Open questions).

## Contract tests pinning behavior (file:line)

- tests/python/integration/test_grpc_contract.py:313-326 — the two cases:
  - `idempotency_key="seed-1"` (issued by the seed `ExecuteAtomic` at line 138-153 with `wait_applied=true`) MUST return `RECEIPT_STATUS_APPLIED`.
  - `idempotency_key="never-issued"` MUST return `RECEIPT_STATUS_PENDING` (NOT `UNKNOWN` — Python collapses both unknown-key and not-yet-applied into `PENDING`).
- The same harness (lines 100-171) builds the server with `Applier` running and `wait_applied=true` on seed, so by the time the receipt query runs the row is in `applied_events`. Go contract tests should reuse this fixture pattern.
- Trailer / redirect contract for `_check_tenant`: covered by sharding tests (search `entdb-redirect-node`); GetReceiptStatus inherits this surface.
- SDK transport-level expectations:
  - sdk/go/entdb/transport_extras_test.go:131-150 — APPLIED decodes to `ReceiptStatusApplied`, `error` empty.
  - sdk/go/entdb/transport_extras_test.go:153-172 — UNKNOWN+error surfaces as `(ReceiptStatusUnknown, "<error>", nil)` — note `nil` Go error because the RPC returned OK.
  - sdk/python/entdb_sdk/_grpc_client.py:875-881 — string-map round trip; FAILED is in the map even though the server doesn't produce it.

## Implementation outline

Go server method, on a `EntDBServiceServer` impl using `grpc-go` (server-side
equivalent of `grpc.aio`; the Python invariant "all handlers async" maps to
"all handlers non-blocking, no goroutine-per-request explosion" in Go — keep
SQLite work behind a bounded worker pool to mirror the Python `_run_sync`
thread pool):

1. `start := time.Now()`; defer `metrics.RecordGRPC("GetReceiptStatus", outcome, time.Since(start))`.
2. `if err := s.checkTenant(ctx, req.GetContext().GetTenantId()); err != nil { return nil, err }` — checkTenant returns a `status.Error` already populated (with the redirect trailer set on `ctx` via `grpc.SetTrailer` BEFORE returning).
3. `applied, err := s.canonical.CheckIdempotency(ctx, tenantID, req.GetIdempotencyKey())`.
4. On `err != nil` (or recovered panic): return `&pb.GetReceiptStatusResponse{ Status: pb.ReceiptStatus_RECEIPT_STATUS_UNKNOWN, Error: err.Error() }, nil`. Mark outcome=`error` for metrics. Do NOT return a gRPC error.
5. Else: return `Status: APPLIED if applied else PENDING`, empty `Error`. outcome=`ok`.
6. Ignore `req.GetContext().GetActor()` entirely.

Connection-pool concurrency: `CheckIdempotency` is a `SELECT 1` on a UNIQUE
`(tenant_id, idempotency_key)` index (canonical_store.py:1100). Use a
read-only connection from the per-tenant pool; do not open a write
transaction.

## Open questions / risks

- **PENDING ambiguity.** Today PENDING means "applier behind" OR "key never
  issued". A misbehaving client could poll forever for a typo. The Go port
  should preserve the existing behavior for parity; logging the key at
  TRACE level is fine, but do not change the wire contract here.
- **No TTL on `applied_events`.** Receipts are queryable forever. If
  retention is added later (compaction), `GetReceiptStatus` will start
  returning `PENDING` for keys that were once `APPLIED`. Out of scope for
  port; flag for the durability/compaction work-stream.
- **`RECEIPT_STATUS_FAILED` is never set.** The proto leaves room for
  durably-failed events (e.g., schema-fingerprint mismatch surfaced
  asynchronously by the applier). Today such failures are surfaced via
  `ExecuteAtomicResponse.error_code` only. The Go port should NOT
  speculatively start emitting FAILED — that would be a contract widening
  not pinned by any test.
- **Untrusted actor in `RequestContext`.** Even though we don't auth on it,
  a future logging change must not echo `actor` into audit logs as if it were
  trusted. Keep the field at the wire boundary, drop it before any
  downstream use.
- **Panic safety.** Python's `except Exception` swallows everything; Go's
  default behavior is to crash the goroutine. Add a defer-recover in the
  handler (or rely on a server-wide unary interceptor) and mirror the
  Python `UNKNOWN`+`error` shape. Otherwise contract tests pass but
  behavior differs under fault injection.
- **Region pinning + idempotency keys are global per tenant.** A tenant
  migrated across regions could see receipts disappear. Tracked separately;
  not introduced by this RPC.
