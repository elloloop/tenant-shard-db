# CancelUserDeletion — Go Port Spec

> Implementation: `server/go/internal/api/cancel_user_deletion.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407. Reverses a pending GDPR right-to-erasure request during the
grace window. Reference Python handler:
`server/go/internal/api/cancel_user_deletion.go`. Proto:
`proto/entdb/v1/entdb.proto:139, 1052-1060`. Backing store:
`server/go/internal/globalstore/` (`cancel_deletion`)
and `:434-450` (`set_user_status`). Counterpart RPC `DeleteUser`:
`server/go/internal/api/cancel_user_deletion.go`.

## Wire contract

Request `entdb.v1.CancelUserDeletionRequest` (proto:1052):
- `string actor` — UNTRUSTED payload. See Auth.
- `string user_id` — required. Identifies the row in the global
  `deletion_queue` table (server/go/internal/globalstore/) and the corresponding
  row in `user_registry`. Plain user id, NOT a `tenant_principal`
  (`user:` prefix is stripped at the boundary).

No grace-window argument, no idempotency key, no tenant scoping —
deletion entries are global-registry rows.

Response `entdb.v1.CancelUserDeletionResponse` (proto:1057):
- `bool success` — mirrors `cancel_deletion`'s rowcount: `True` iff a
  `pending` deletion row existed and was deleted
  (server/go/internal/globalstore/; server/go/internal/api/cancel_user_deletion.go).
- `string error` — populated only by the in-band catch-all
  (server/go/internal/api/cancel_user_deletion.go).

Note: success=false carries NO error text when the row simply was not
in `pending` state (e.g. already completed, or never queued). The Go
port MUST replicate this — do NOT promote "no pending row" to
`NotFound` or `FailedPrecondition`; integration test
`tests/python/integration/test_grpc_contract.py:677-681` asserts the
"happy" mode on a freshly queued deletion using the response body, not
status codes.

## Auth (self within grace window / admin; trusted-actor)

- Empty `actor` → `INVALID_ARGUMENT "actor is required"`
  (server/go/internal/api/cancel_user_deletion.go).
- Empty `user_id` → `INVALID_ARGUMENT "user_id is required"`
  (server/go/internal/api/cancel_user_deletion.go).
- `global_store` unset → `UNIMPLEMENTED "User registry not
  configured"` (server/go/internal/api/cancel_user_deletion.go).
- Authorization gate: `_is_self_or_admin(actor, user_id)`
  (server/go/internal/api/cancel_user_deletion.go, definition at :2071-2086). Trusted-actor
  pattern — privilege decision uses `get_authoritative_actor(actor)`
  from `auth/auth_interceptor.py:92`, NOT `request.actor`. This is the
  post-#168 invariant: a malicious caller cannot forge `actor=
  "admin:root"` in the payload.
  - admin / system → trusted starts with `system:`, `admin:`, or
    equals `__system__`.
  - self → trusted equals `"user:" + user_id` or equals `user_id`.
  - else → `PERMISSION_DENIED "CancelUserDeletion requires the user
    themselves or an admin actor"`.
- "Within grace window" is enforced **only** by the `cancel_deletion`
  store call: it `DELETE`s rows `WHERE status = 'pending'`. If the
  applier worker has already flipped the row to `completed`, the
  cancel is a no-op (`success=false`). There is no time-based check
  in the handler — the worker's pass over `get_executable_deletions`
  (server/go/internal/globalstore/) is the de-facto deadline.

## Side effects (global WAL append; flip pending → active)

The Go handler appends a global-scope `user_deletion_canceled` WAL
event and waits for the applier. The user registry and `deletion_queue`
are no longer direct-write carve-outs.

What the handler actually does (server/go/internal/api/cancel_user_deletion.go):
1. Pre-read the pending `deletion_queue` row. If none exists, return the
   Python-compatible in-band no-op response.
2. Append `op="user_deletion_canceled"` with `user_id` and `updated_at`.
3. The applier hard-deletes the pending row and flips
   `user_registry.status` from `pending_deletion` back to `active` in one
   globalstore transaction.
   The status flip is **conditional** on cancel succeeding; if the
   row was already gone, status is left untouched.
3. `record_grpc_request("CancelUserDeletion", "ok"|"error", elapsed)`
   (server/go/internal/api/cancel_user_deletion.go, 3008).

No audit-log write. Compliance trail is out of scope; per CLAUDE.md
the WAL + S3 Object Lock supply the immutable record for tenant-plane
mutations, but global-registry mutations sit outside that flow.

## Error contract (FAILED_PRECONDITION when past point of no return)

| Condition                          | Channel             | Status / value                                                                  |
|------------------------------------|---------------------|---------------------------------------------------------------------------------|
| `global_store` unset               | `context.abort`     | `UNIMPLEMENTED "User registry not configured"`                                  |
| empty `actor`                      | `context.abort`     | `INVALID_ARGUMENT "actor is required"`                                          |
| empty `user_id`                    | `context.abort`     | `INVALID_ARGUMENT "user_id is required"`                                        |
| not self, not admin                | `context.abort`     | `PERMISSION_DENIED "CancelUserDeletion requires the user themselves or an admin actor"` |
| no pending row (never queued)      | in-band response    | `success=false, error=""`                                                       |
| already completed (past grace)     | in-band response    | `success=false, error=""`                                                       |
| unhandled exception                | in-band response    | `success=false, error=str(e)`; metric label `error`                             |

**Re: FAILED_PRECONDITION at point-of-no-return.** The Python handler
does NOT raise `FAILED_PRECONDITION` when the deletion has already
executed. It surfaces the same `success=false` body whether the row
never existed or was already swept by the applier. This is a known
ergonomic gap (see Open questions); the Go port MUST preserve current
behavior to satisfy the integration contract test, and a follow-up
epic should introduce a distinct status (`completed` retained as a
tombstone + `FAILED_PRECONDITION` from the handler) — that is a proto
+ store + test change and out of scope here.

## Shared Go package deps

- `entdbserver/auth` — `GetAuthoritativeActor(ctx, payloadActor) string`
  and `IsSelfOrAdmin(ctx, userID string) bool` (port of
  `auth/auth_interceptor.py:92` and `server/go/internal/api/cancel_user_deletion.go`). The
  `AuthInterceptor` unary interceptor stuffs the trusted actor into
  `context.Context` from gRPC metadata.
- `entdbserver/globalstore` — `CancelDeletion(ctx, userID string)
  (bool, error)` (port of `_sync_cancel_deletion`,
  server/go/internal/globalstore/) and `SetUserStatus(ctx, userID, status
  string) (bool, error)` (port of `_sync_set_user_status`).
- `entdbserver/metrics` — `RecordGRPCRequest(method, outcome string,
  elapsed time.Duration)` (port of `record_grpc_request`).
- `entdbpb` — generated `entdb.v1` Go bindings
  (`CancelUserDeletionRequest`, `CancelUserDeletionResponse`).
- `google.golang.org/grpc/status`, `google.golang.org/grpc/codes` for
  aborts.

## Other-RPC deps (DeleteUser counterpart)

- `DeleteUser` (server/go/internal/api/cancel_user_deletion.go) — populates the
  `deletion_queue` row that this RPC removes and flips
  `user_registry.status` to `pending_deletion`. CancelUserDeletion is
  meaningless without a prior `DeleteUser`; the contract test seeds
  state by calling `DeleteUser` first
  (test_gdpr_engine.py:668).
- `FreezeUser` (server/go/internal/api/cancel_user_deletion.go near :3014, proto:139 sibling) — also
  uses `_is_self_or_admin`; share the auth helper.
- `ExportUserData` (server/go/internal/api/cancel_user_deletion.go+) — frequently called before
  `DeleteUser` for GDPR Article 20 + 17 sequencing; not a hard
  prerequisite.
- The async deletion sweeper (`get_executable_deletions` →
  `mark_deletion_completed`, server/go/internal/globalstore/) races with this
  RPC. The Go port MUST keep the same `WHERE status='pending'`
  guard so a partially-executed deletion cannot be cancelled.

## Contract tests pinning behavior (file:line)

- (legacy Python unit test, removed in Phase 4D) — `cancel_deletion`
  during grace removes the queue entry; `get_pending_deletions()` is
  empty afterwards. Pins step-1 semantics.
- (legacy Python unit test, removed in Phase 4D) — full handler
  happy path: `DeleteUser(alice)` then `CancelUserDeletion(alice)`
  returns `success=True` and `user_registry.status == "active"`. Pins
  the handler chain (cancel → set_user_status flip).
- `tests/python/integration/test_grpc_contract.py:676-681` — wire-
  level happy: `actor=ALICE, user_id="alice"` returns successfully
  over the gRPC channel. Pins the proto/RPC route registration.
- (legacy Python unit test, removed in Phase 4D) — adjacent
  `mark_deletion_completed` test pins the OTHER terminal state and
  proves `cancel_deletion` MUST NOT touch a `completed` row (the
  `WHERE status='pending'` guard is the boundary).

These four cases form the Go-port acceptance bar; wire them into
`tests/contract/` once the Go server speaks `entdb.v1.EntDBService`.

## Implementation outline

```go
func (s *Server) CancelUserDeletion(ctx context.Context, req *pb.CancelUserDeletionRequest) (*pb.CancelUserDeletionResponse, error) {
    start := time.Now()
    outcome := "ok"
    defer func() { metrics.RecordGRPCRequest("CancelUserDeletion", outcome, time.Since(start)) }()

    if s.globalStore == nil {
        return nil, status.Error(codes.Unimplemented, "User registry not configured")
    }
    if req.GetActor() == "" {
        return nil, status.Error(codes.InvalidArgument, "actor is required")
    }
    if req.GetUserId() == "" {
        return nil, status.Error(codes.InvalidArgument, "user_id is required")
    }
    if !auth.IsSelfOrAdmin(ctx, req.GetUserId()) {
        return nil, status.Error(codes.PermissionDenied,
            "CancelUserDeletion requires the user themselves or an admin actor")
    }

    ok, err := s.globalStore.CancelDeletion(ctx, req.GetUserId())
    if err != nil {
        outcome = "error"
        return &pb.CancelUserDeletionResponse{Success: false, Error: err.Error()}, nil
    }
    if ok {
        if _, err := s.globalStore.SetUserStatus(ctx, req.GetUserId(), "active"); err != nil {
            outcome = "error"
            return &pb.CancelUserDeletionResponse{Success: false, Error: err.Error()}, nil
        }
    }
    return &pb.CancelUserDeletionResponse{Success: ok}, nil
}
```

`globalstore.CancelDeletion` issues `DELETE FROM deletion_queue WHERE
user_id=? AND status='pending'` over the global `*sql.DB` and returns
`rowsAffected > 0`. Single statement — no transaction needed. The
status flip is a separate UPDATE; the brief window between the two
is acceptable (worst case: row deleted, status not flipped, leaves
user in `pending_deletion` until a retry — same as Python today).

## Open questions / risks (cancellation deadline)

- **No explicit deadline.** "Grace window" is implicit: it lasts
  until the applier sweeper flips `status='pending'` to `completed`
  (server/go/internal/globalstore/). There is no time-based check in the
  handler. Risk: in a fast-sweep deployment with `grace_days=0` an
  admin who issues `DeleteUser` then `CancelUserDeletion` ms later
  may race the sweeper and silently fail (`success=false, error=""`).
  Mitigation is purely operational — keep `grace_days >= 1`.
- **No `FAILED_PRECONDITION` for past-deadline.** The handler cannot
  distinguish "never queued" from "already executed"; both surface as
  `success=false, error=""`. Operator UX gap. Fix is a proto change
  (separate epic): add an `ALREADY_EXECUTED` enum or promote to a
  gRPC status code, plus retain a `cancelled`/`completed` tombstone
  in `deletion_queue` instead of hard-deleting on cancel.
- **Step-1/step-2 atomicity.** Closed in the Go global WAL path:
  `ApplyUserDeletionCanceled` removes the pending row and updates
  user status in one SQLite transaction.
- **Trusted-actor coverage.** Unit tests mostly run without the
  AuthInterceptor (FakeContext path). The Go port MUST add a
  contract test that asserts a forged `actor="admin:root"` in the
  payload is REJECTED when metadata says `user:eve` — this is the
  post-#168 invariant and currently only covered indirectly.
- **Idempotency.** Re-issuing the same `CancelUserDeletion` after a
  successful first call returns `success=false, error=""` (row gone).
  Clients that retry on transient failures may observe this and log
  spuriously — no fix needed, just document.
