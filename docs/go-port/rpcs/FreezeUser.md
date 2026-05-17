# FreezeUser — Go Port Spec

> Implementation: `server/go/internal/api/freeze_user.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407. GDPR Article 18 ("restrict processing"): toggle a user
between `active` and `frozen`. Freeze blocks user-initiated mutations
but preserves all data (vs `DeleteUser` which tombstones, vs
`RevokeAllUserAccess` which removes ACL grants). Python handler:
`server/go/internal/api/freeze_user.go`. Proto:
`proto/entdb/v1/entdb.proto:138, 1039-1050`.

## Wire contract

Request `entdb.v1.FreezeUserRequest` (proto:1039-1044):
- `string actor` — UNTRUSTED; see Auth. Trusted id from gRPC metadata.
- `string user_id` — bare user id (`"alice"`), NOT a principal
  (`"user:alice"`). Handler compares both forms (server/go/internal/api/freeze_user.go).
- `bool enabled` — `true` → status `frozen`; `false` → `active`.

Response `FreezeUserResponse` (proto:1046-1050):
- `bool success` — `true` on apply, `false` on "user not found" and
  on caught exception paths.
- `string status` — the post-write status string (`"frozen"` or
  `"active"`). Empty on the not-found / error paths.
- `string error` — populated on not-found (`"User not found"`) and on
  any caught non-gRPC exception. Empty on success.

Idempotency: freezing an already-frozen user (or unfreezing an
already-active user) is a no-op write that still returns
`success=true, status=<new>` — `set_user_status` UPDATEs
unconditionally, returns `True` iff a row matched (global_store.py:
442-447). Pinned by `test_unfreeze_user_handler`
(test_gdpr_engine.py:721-731) which calls FreezeUser twice.

## Auth (compliance / admin; trusted-actor)

- Identity: handler calls `_is_self_or_admin(request.actor,
  request.user_id)` (server/go/internal/api/freeze_user.go, 2071-2086) which **discards
  the payload actor** and uses
  `auth_interceptor.get_authoritative_actor(actor)` (line 2083). Go
  port MUST read trusted identity from gRPC context metadata, never
  trust `request.actor`. Privilege-escalation matrix in
  `tests/python/integration/test_privilege_escalation.py` covers this
  family.
- Allowed callers (line 2084-2086):
  - any actor whose trusted id starts with `system:` or `admin:`
  - the literal `__system__`
  - the user themselves (trusted id == `user:<user_id>` or
    `<user_id>`)
- Pre-checks (in order, each `await context.abort` on failure):
  1. `self.global_store` must be configured →
     `UNIMPLEMENTED "User registry not configured"` (server/go/internal/api/freeze_user.go:
     3080-3081).
  2. `request.actor` non-empty → `INVALID_ARGUMENT "actor is required"`
     (3082-3083).
  3. `request.user_id` non-empty → `INVALID_ARGUMENT "user_id is
     required"` (3084-3085).
  4. self-or-admin check → `PERMISSION_DENIED "FreezeUser requires
     the user themselves or an admin actor"` (3086-3090).

There is **no tenant scope** on this RPC — freeze is a global-store
operation, mirroring `DeleteUser` (server/go/internal/api/freeze_user.go). Do not
add `_check_tenant_access` on the port.

## Side effects (WAL append; flag in global_store; subsequent auth checks reject mutating ops)

The Go port appends a global `user_frozen` WAL op and waits for the
applier to update `user_registry.status`:

```
TransactionEvent.ops = [{
    "op": "user_frozen",
    "user_id": <string>,
    "status": "frozen" | "active",
}]
```

Applier: `UPDATE user_registry SET status=?, updated_at=? WHERE
user_id=?`; `rowcount > 0` → `success`. Wait for receipt APPLIED
before returning.

The freeze flag is `user_registry.status` (server/go/internal/globalstore/).
Consulted on every mutating RPC by `_check_tenant_access(...,
require_write=True)` (server/go/internal/api/freeze_user.go): if status is `frozen`
or `pending_deletion`, abort `PERMISSION_DENIED "User '<id>' is
frozen and cannot write"`. Reads pass `require_write=False` — see
`test_frozen_user_can_still_read` (test_gdpr_engine.py:754-767).

**Read-still-allowed scope.** Freeze gates writes, NOT:
- Pure-read RPCs (`GetNode`, `QueryNodes`, `GetMailbox`,
  `SearchMailbox`, `ListSharedWithMe`, …).
- `ExportUserData` (GDPR Article 20 portability carve-out).
- Admin/system RPCs operating **on** the frozen user (`DeleteUser`,
  `CancelUserDeletion`, `FreezeUser` unfreeze) — those go through
  `_is_self_or_admin`, not `_check_tenant_access`.
- Receipt status / health / schema reads.

No data purged, no ACL grants removed, no mailbox rows deleted.
The user’s nodes remain visible to peers via existing shares.

## Error contract

- `UNIMPLEMENTED` abort: global_store unconfigured (3080-3081).
- `INVALID_ARGUMENT` abort: empty `actor` (3082-3083) or empty
  `user_id` (3084-3085).
- `PERMISSION_DENIED` abort: not self/admin/system (3086-3090).
- `OK` + `{success:false, error:"User not found"}`: user_id unknown
  (`set_user_status` returned False; line 3094-3096).
- `OK` + `{success:true, status:<new>}`: happy path (3097-3098).
- `OK` + `{success:false, error:str(e)}`: any other caught exception
  (3099-3104). gRPC aborts re-raise unchanged via the
  `isinstance(e, grpc.RpcError)` guard at line 3101.

Do NOT promote "User not found" to `NOT_FOUND` — clients pin the
in-band shape. Metrics: `record_grpc_request("FreezeUser", "ok",
dur)` for happy + not-found; `"error"` for caught exception. Aborts
emit no metric (raised before record).

## Shared Go package deps (auth interceptor must consult freeze flag)

- `internal/auth` — `AuthInterceptor.AuthoritativeActor(ctx)`,
  `IsSelfOrAdmin(actor, userID)` mirroring server/go/internal/api/freeze_user.go.
  **Critical:** the auth interceptor’s write-path check (the Go
  equivalent of `_check_tenant_access` at server/go/internal/api/freeze_user.go)
  MUST consult `GlobalStore.GetUser(userID).Status` and reject
  `frozen` / `pending_deletion` for any mutating RPC. This is the
  load-bearing enforcement; FreezeUser only sets the bit.
- `internal/global` — `GlobalStore.SetUserStatus(userID, status)`
  applier-side; `GlobalStore.GetUser(userID)` read-side (used by the
  auth interceptor’s freeze gate).
- `internal/wal` — `Wal.AppendAndWait(event)` for the
  `set_user_status` op (post-fix; not present in Python).
- `internal/apply` — applier op handler `set_user_status` writing to
  the global-store SQLite; idempotent UPDATE.
- `internal/metrics` — `RecordGrpcRequest("FreezeUser", label, dur)`
  with labels `ok|error` (note: PERMISSION_DENIED aborts do NOT emit
  a label — they raise before `record_grpc_request` is reached;
  preserve this).
- `internal/proto` (generated) — `entdb.v1.FreezeUserRequest`,
  `FreezeUserResponse`.

## Other-RPC deps (DeleteUser, RevokeAllUserAccess)

- **DeleteUser** (server/go/internal/api/freeze_user.go, proto:136, 1010-1024)
  calls `set_user_status(user_id, "pending_deletion")` (line 2967).
  `pending_deletion` is treated identically to `frozen` by the auth
  gate (line 552). FreezeUser MUST NOT clobber `pending_deletion →
  active` — Python does not guard this. Go port: reject
  `enabled=false` when current status is `pending_deletion`; require
  `CancelUserDeletion`.
- **CancelUserDeletion** (server/go/internal/api/freeze_user.go, proto:139) is
  the inverse for `pending_deletion → active` (line 3004). Same
  global-store path; same WAL violation; same fix.
- **RevokeAllUserAccess** (proto:133, 994-1006) removes ACL grants
  but does NOT touch `user_registry.status`. Complementary, not
  equivalent: revoke destroys sharing; freeze blocks future writes
  while preserving data.
- **UpdateUser** (proto:914) exposes a `status` field. Go port
  should funnel admin status writes through the same WAL op (or
  drop the field in favor of FreezeUser / CancelUserDeletion).

## Contract tests pinning behavior (file:line)

- (legacy Python unit test, removed in Phase 4D)
  (`test_freeze_user_handler`) — freeze sets status `frozen` and
  `resp.status == "frozen"`.
- (legacy Python unit test, removed in Phase 4D)
  (`test_unfreeze_user_handler`) — toggle to `active`; idempotent.
- (legacy Python unit test, removed in Phase 4D)
  (`test_freeze_user_rejects_writes`) — frozen user’s
  `_check_tenant_access(require_write=True)` aborts
  `PERMISSION_DENIED`. **Load-bearing test for the freeze gate.**
- (legacy Python unit test, removed in Phase 4D)
  (`test_frozen_user_can_still_read`) —
  `_check_tenant_access(require_write=False)` returns the role.
- `tests/python/integration/test_privilege_escalation.py` — covers
  FreezeUser trusted-actor enforcement.

Test gaps to add on port: system/admin actor paths, INVALID_ARGUMENT
paths, "User not found" in-band, `pending_deletion` interaction,
WAL-replay determinism (unlocked by fix-on-port).

## Implementation outline

```go
func (s *Server) FreezeUser(ctx context.Context,
    req *pb.FreezeUserRequest) (*pb.FreezeUserResponse, error) {
    start := time.Now()
    if s.globalStore == nil {
        return nil, status.Error(codes.Unimplemented, "User registry not configured")
    }
    if req.Actor == "" {
        return nil, status.Error(codes.InvalidArgument, "actor is required")
    }
    if req.UserId == "" {
        return nil, status.Error(codes.InvalidArgument, "user_id is required")
    }
    if !s.auth.IsSelfOrAdmin(ctx, req.UserId) {
        return nil, status.Error(codes.PermissionDenied,
            "FreezeUser requires the user themselves or an admin actor")
    }
    newStatus := "active"
    if req.Enabled { newStatus = "frozen" }
    evt := wal.GlobalEvent([]wal.Op{{Type: "set_user_status",
        UserID: req.UserId, Status: newStatus}})
    receipt, err := s.wal.AppendAndWait(ctx, evt)
    if err != nil {
        metrics.Record("FreezeUser", "error", start)
        return &pb.FreezeUserResponse{Error: err.Error()}, nil
    }
    if receipt.RowsAffected == 0 {
        metrics.Record("FreezeUser", "ok", start)
        return &pb.FreezeUserResponse{Error: "User not found"}, nil
    }
    metrics.Record("FreezeUser", "ok", start)
    return &pb.FreezeUserResponse{Success: true, Status: newStatus}, nil
}
```

## Open questions / risks (read-still-allowed scope)

1. **Read scope, precisely.** Reads-allowed is pinned only via the
   `require_write` flag on `_check_tenant_access`. Audit Go port read
   RPCs so none pass `require_write=true` (e.g. mailbox cache-warm
   side effects). Keep "reads allowed" strict.
2. **WAL parity vs. Python bug-for-bug.** Python does not WAL freeze
   events; replays silently un-freeze users. Recommend fix-on-port
   (matches RevokeAccess decision). Tests pin only post-call status.
3. **`pending_deletion` clobber.** Python lets FreezeUser
   `enabled=false` overwrite `pending_deletion` with `active`,
   silently cancelling a scheduled deletion. Recommend Go port
   reject with `FAILED_PRECONDITION "use CancelUserDeletion"`.
4. **Self-unfreeze.** Python allows self-unfreeze via the self arm
   of `_is_self_or_admin`. For GDPR Article 18 this is debatable —
   the regulator may have requested the freeze. Match Python today;
   flag for product/legal.
5. **`ExportUserData` carve-out.** Confirm `ExportUserData`
   (proto:137) never sets `require_write=true`. GDPR Article 20
   portability must remain available under freeze.
6. **Interceptor placement.** The freeze gate lives in
   `_check_tenant_access` per-handler. Consider lifting into the
   auth interceptor so new mutating RPCs inherit the gate — only if
   the interceptor can cheaply resolve "is this RPC a write?" (proto
   annotation or method-name allowlist).
