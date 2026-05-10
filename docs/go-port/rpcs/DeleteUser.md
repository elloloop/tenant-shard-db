# DeleteUser — Go Port Spec

EPIC #407. GDPR right-to-erasure. Queues a user for deletion with a
grace period; a background worker performs the actual erasure.
Python source of truth: `server/python/entdb_server/api/grpc_server.py:2925-2981`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:136` (RPC), `:1010-1025` (messages).

```
rpc DeleteUser(DeleteUserRequest) returns (DeleteUserResponse);

DeleteUserRequest  { string actor = 1; string user_id = 2; int32 grace_days = 3; }
DeleteUserResponse { bool success = 1; int64 requested_at = 2;
                     int64 execute_at = 3; string status = 4; string error = 5; }
```

`grace_days <= 0` is normalized to **30** (`grpc_server.py:2965`).
`status` values returned today: `"pending"`. Worker transitions write
back to `global_store` rows; the gRPC reply only ever sees `"pending"`
on the queueing path (see `_sync_queue_deletion`, `global_store.py:815-822`).

`requested_at` / `execute_at` are unix-seconds (int64) populated by
`global_store.queue_deletion` (`global_store.py:800-822`).

Note: the request is **flat** (no `RequestContext`), unlike most
ingress RPCs. `actor` is therefore the only client-supplied identity
field — see Auth.

## Auth (self via consent / admin / compliance; trusted-actor)

1. **Trusted actor.** Handler MUST rebind `request.actor` through
   `_trusted_actor` (`grpc_server.py:418-437`) before any authorization
   check. Untrusted client claims are dropped per the choke-point fix
   (commit `fece3fb`, applied to all GDPR RPCs in the Go port).
2. **Self-or-admin.** `_is_self_or_admin(actor, user_id)` gates entry
   (`grpc_server.py:2944-2948`). Two acceptable principals:
   - The user themselves (`actor == "user:<user_id>"`), satisfying
     GDPR Art. 17 self-service erasure consent.
   - An admin / compliance principal (system actors with the `admin`
     role; same predicate used by `FreezeUser` and `ExportUserData`).
3. **Argument validation.** `actor` and `user_id` empty → `INVALID_ARGUMENT`
   (`grpc_server.py:2940-2943`). `global_store` not configured →
   `UNIMPLEMENTED`.
4. Failure mode: non-self / non-admin → `PERMISSION_DENIED` via
   `context.abort` (pinned by `tests/python/unit/test_gdpr_engine.py:646-654`
   and `tests/python/integration/test_grpc_contract.py:671-676`).

## Side effects

The handler is intentionally **lightweight**: it queues, it does not
erase. The actual deletion pipeline runs in `gdpr_worker.py`.

Synchronous path (`grpc_server.py:2950-2967`):
1. `global_store.get_user(user_id)` → `None` returns
   `success=False, error="User not found"` (no abort, OK status code).
   Pinned: `test_gdpr_engine.py:658-662`.
2. `global_store.get_deletion_entry(user_id)` → if an entry with
   `status == "pending"` exists, return its existing
   `requested_at`/`execute_at` unchanged (idempotent re-queue).
   Pinned: `test_gdpr_engine.py:637-643`.
3. `global_store.queue_deletion(user_id, grace_days)` inserts into the
   `deletion_queue` table (`global_store.py:239-246, 815-822`).
4. `global_store.set_user_status(user_id, "pending_deletion")`.

WAL append: **none on the queueing path today.** The Python handler
writes directly to `global_store` (which has its own SQLite — see
CLAUDE.md invariant #4). The Go port MUST preserve this: cross-tenant
membership and deletion state live in the global store, not per-tenant
WALs. The actual erasure events emitted by the worker (anonymize_user,
delete_tenant, remove_membership) do go through per-tenant WALs via
`canonical_store.anonymize_user_in_tenant` and
`canonical_store.delete_tenant_database` (`gdpr_worker.py:135-167`).

Tombstone vs. hard delete:
- **Per-tenant nodes/edges**: anonymized in-place (PII fields hashed
  with salt, `owner_actor` rewritten); rows remain so referential
  integrity holds. This is the "tombstone" — the WAL retains the
  anonymize op but **not raw PII** (CLAUDE.md invariant #2; salt lives
  in `gdpr_worker.salt`).
- **Sole-owner ("personal") tenants**: SQLite file is dropped via
  `canonical_store.delete_tenant_database` and tenant status set to
  `"deleted"` (`gdpr_worker.py:138-152`).
- **Global rows**: memberships, shared edges, then user status →
  `"deleted"`, queue entry → `"completed"` (`gdpr_worker.py:170-173`).

Legal hold: there is **no legal-hold check in the current Python
DeleteUser handler**. Tenant-level legal hold is enforced in
`_check_tenant_access` for *node/edge* deletes
(`grpc_server.py:524, 453, 755`) but not at GDPR queueing time.
The Go port SHOULD add the check explicitly: before queueing, walk
`global_store.get_user_tenants(user_id)` and reject with
`FAILED_PRECONDITION` if any returns `is_under_legal_hold(tenant_id)`
true (`global_store.py:1040-1056`). Flag this in the issue — do not
silently change behavior; gate on a config flag and add a contract
test before flipping default.

Durable crypto-shred: the personal-tenant path drops the SQLite file,
which removes the data-encryption-key handle held by
`crypto/crypto_shred` (`encryption.py:147-149`); for non-personal
tenants the per-user payload remains encrypted under the tenant DEK
and is anonymized in-place. No separate per-user shred is performed.

## Error contract

| Condition                              | Code                  | Pinned by |
|----------------------------------------|-----------------------|-----------|
| `global_store` not configured          | `UNIMPLEMENTED`       | `grpc_server.py:2939` |
| `actor` empty                          | `INVALID_ARGUMENT`    | `grpc_server.py:2940-2941` |
| `user_id` empty                        | `INVALID_ARGUMENT`    | `grpc_server.py:2942-2943` |
| Caller is not self and not admin       | `PERMISSION_DENIED`   | `test_gdpr_engine.py:646-654`, `test_grpc_contract.py:671-676` |
| User does not exist                    | `OK` + `success=false`, `error="User not found"` | `test_gdpr_engine.py:658-662` |
| Pending entry already exists           | `OK` + `success=true`, `status="pending"`, same timestamps | `test_gdpr_engine.py:637-643` |
| Generic exception                      | `OK` + `success=false`, `error=str(e)` | `grpc_server.py:2976-2981` |

Notes:
- "User not found" returns OK, not `NOT_FOUND`. The Go port MUST keep
  this for SDK compatibility.
- Legal-hold rejection (when added) → `FAILED_PRECONDITION` with a
  message naming the held tenant. New behavior; ship behind flag.

## Shared Go package deps

- `gdpr/` (new) — port of `gdpr_worker.py`. Owns the deletion state
  machine, anonymization, sole-tenant-drop logic, and the
  `tenant_principal` translation (`user:<id>`).
- `audit/` — `audit/compliance.go` (port of `audit/compliance.py`),
  `audit/s3_lock.go` (port of `audit/s3_lock.py`). DeleteUser does
  not write audit records itself; the worker emits events that S3
  Object Lock retains (CLAUDE.md invariant #2).
- `crypto/keys/` — DEK lifecycle for sole-tenant crypto-shred via
  `crypto_shred_tenant` (`encryption.py:147-149`).
- `wal/` — used only indirectly via per-tenant `canonical_store`
  ops in the worker (anonymize / delete_tenant). DeleteUser handler
  itself does not append.
- `store/` — `global_store.go` interface: `GetUser`,
  `GetDeletionEntry`, `QueueDeletion`, `SetUserStatus`,
  `IsUnderLegalHold`, `GetUserTenants`.

## Other-RPC deps

- **`CancelUserDeletion`** (`grpc_server.py:2983-3010`,
  `proto:139`): inverse during grace window; deletes from
  `deletion_queue` and flips user back to `"active"`. MUST be
  implemented in the same milestone — pinned by
  `test_gdpr_engine.py:664-672`.
- **`ExportUserData`** (`proto:1027-1037`): GDPR Art. 20 portability.
  Often called immediately before DeleteUser by SDKs; share the
  same self-or-admin gate.
- **`FreezeUser`** (`proto:1039-1049`): suspends without erasure;
  shares the auth helper. Worker SHOULD skip frozen users or treat
  freeze as a soft hold (current Python: no interaction — flag).
- `SetLegalHold` (tenant-level, `grpc_server.py:2820-2840`): produces
  the precondition the new legal-hold gate would read.

## Contract tests pinning behavior

- `tests/python/unit/test_gdpr_engine.py:625-633` — happy path,
  `status=="pending"`, user marked `pending_deletion`.
- `tests/python/unit/test_gdpr_engine.py:636-643` — idempotency:
  same `execute_at` returned on repeat call.
- `tests/python/unit/test_gdpr_engine.py:646-654` — non-self /
  non-admin → `PERMISSION_DENIED` abort.
- `tests/python/unit/test_gdpr_engine.py:657-662` — unknown user →
  `success=False`, no abort.
- `tests/python/unit/test_gdpr_engine.py:664-672` — Cancel after
  Delete restores status to `"active"`.
- `tests/python/integration/test_grpc_contract.py:665-670` —
  end-to-end happy path over real gRPC channel.
- `tests/python/integration/test_grpc_contract.py:671-676` —
  end-to-end `permission_denied` for foreign actor.

The Go port MUST keep all seven green via the contract test harness
in `tests/contract/` (cross-impl).

## Implementation outline

State machine (rows in `global_store.deletion_queue`,
`global_store.py:239-246`):

```
                     +-------------------+
                     | (no row)          |
                     +---------+---------+
                               | DeleteUser
                               v
+---------------+   Cancel     +-----------+
|   cancelled   | <----------- | scheduled | <-- (re-DeleteUser is no-op)
| (row removed) |              +-----+-----+
+---------------+                    | execute_at <= now
                                     v
                              +--------------+
                              |   executing  |   (worker tick)
                              +------+-------+
                                     | success
                                     v
                              +--------------+
                              |   completed  |
                              +--------------+
```

Handler pseudocode (Go):

```go
func (s *Server) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
    if s.globalStore == nil { return nil, status.Error(codes.Unimplemented, "User registry not configured") }
    if req.Actor == ""   { return nil, status.Error(codes.InvalidArgument, "actor is required") }
    if req.UserId == ""  { return nil, status.Error(codes.InvalidArgument, "user_id is required") }
    actor := s.trustedActor(req.Actor)                                    // ContextVar equivalent
    if !s.isSelfOrAdmin(actor, req.UserId) {
        return nil, status.Error(codes.PermissionDenied,
            "DeleteUser requires the user themselves or an admin actor")
    }
    user, _ := s.globalStore.GetUser(ctx, req.UserId)
    if user == nil { return &pb.DeleteUserResponse{Success: false, Error: "User not found"}, nil }
    if existing, _ := s.globalStore.GetDeletionEntry(ctx, req.UserId);
       existing != nil && existing.Status == "pending" {
        return &pb.DeleteUserResponse{Success: true, RequestedAt: existing.RequestedAt,
            ExecuteAt: existing.ExecuteAt, Status: "pending"}, nil
    }
    // (optional, gated) legal-hold gate: walk GetUserTenants → IsUnderLegalHold
    grace := req.GraceDays; if grace <= 0 { grace = 30 }
    entry, err := s.globalStore.QueueDeletion(ctx, req.UserId, grace)
    if err != nil { return &pb.DeleteUserResponse{Success: false, Error: err.Error()}, nil }
    _ = s.globalStore.SetUserStatus(ctx, req.UserId, "pending_deletion")
    return &pb.DeleteUserResponse{Success: true, RequestedAt: entry.RequestedAt,
        ExecuteAt: entry.ExecuteAt, Status: "pending"}, nil
}
```

Worker (separate goroutine, port of `gdpr_worker.py`):
poll `GetExecutableDeletions(now)`, for each user run anonymize /
sole-tenant-drop / global-cleanup pipeline, then
`MarkDeletionCompleted`. Per-tenant ops go through the WAL via
existing `canonical_store` Go ports — DeleteUser itself never appends.

## Open questions / risks

1. **GDPR 30-day deadline.** Art. 17 requires erasure "without undue
   delay." The current 30-day grace is a *cancellation* window and
   may exceed the deadline if the worker is offline. Risk: silent
   non-compliance. Mitigation: alert if `now - execute_at > 7d` in
   `executable_deletions`. Decide before GA.
2. **Idempotency on the failure path.** A re-queue after a worker
   crash mid-pipeline currently returns the *original* timestamps
   (handler thinks it's pending); but `deletion_queue.status` may
   have been left in a partial state (no `executing` row today —
   pipeline is run-to-completion in-process). The Go port should
   add an `executing` status with a worker lease + heartbeat to
   make crash recovery deterministic.
3. **Audit immutability.** Worker-emitted anonymize/delete events
   are appended to per-tenant WALs which terminate in S3 Object
   Lock (COMPLIANCE mode). Confirm the Object Lock retention
   horizon ≥ statutory audit window before flipping any tenant
   to "GDPR mode." `audit/s3_lock.go` MUST refuse retention < N
   days configured.
4. **Legal-hold gate not enforced at queue time** (Python). The
   Go port should add it (FAILED_PRECONDITION) and pin a contract
   test, but ship behind a config flag to keep parity tests green
   on day-zero of cutover.
5. **Self-only via consent receipt.** Today self-action requires
   only that the trusted actor match the user_id. Stronger: require
   a recent consent receipt (signed by the user's session key) so
   a hijacked session can't auto-erase. Out of scope for the Go
   port v1 but flag in the spec.
6. **Personal-tenant heuristic.** Sole-membership ⇒ drop tenant.
   If membership state is racing with `RemoveMember`, the worker
   may drop a tenant that gained a new member between `get_members`
   and the file unlink. Lock the tenant row before the decision.
