# UpdateUser — Go Port Spec

EPIC #407. Mutates a row in the global user registry. Reference Python
handler: `server/python/entdb_server/api/grpc_server.py:2184-2229`.
Proto: `proto/entdb/v1/entdb.proto:114, 826-838`. Backing store:
`server/python/entdb_server/global_store.py:386-409`.

## Wire contract

Request `entdb.v1.UpdateUserRequest` (proto:826):
- `string actor` — UNTRUSTED payload. See Auth.
- `string user_id` — required. Identifies the row in
  `global_store.user_registry`. Not a `tenant_principal` (no `user:`
  prefix).
- `string email`, `string name`, `string status` — partial-update
  fields. **Truthiness gates each one** (grpc_server.py:2209-2214):
  empty string == "do not update". There is **no `FieldMask`** and no
  way to clear a field to empty. Go port MUST replicate truthy-only
  semantics; do not switch to proto3 presence/`optional` without a
  proto change. Status is a free-form string (`active`, `suspended`,
  …) — no enum is enforced server-side, but only `email`/`name`/
  `status` are whitelisted in `_sync_update_user`
  (global_store.py:397). Unknown fields silently no-op (filtered by
  the whitelist).

Partial vs full: **partial only**. Any subset of the three mutable
fields may be sent. If all three are empty, the handler short-circuits
and returns `success=false, error="No fields to update"`
(grpc_server.py:2216-2218) — this is **not an INVALID_ARGUMENT abort**;
it is an in-band `UpdateUserResponse`.

Immutable on this RPC: `user_id`, `created_at`. `updated_at` is set
server-side by `_sync_update_user` (global_store.py:401).

Response `UpdateUserResponse` (proto:835):
- `bool success` — true iff one row was updated.
- `string error` — populated for the in-band failures
  ("No fields to update", "User not found", and the catch-all
  `str(e)`).

## Auth

- `request.actor` REQUIRED (`INVALID_ARGUMENT` if empty,
  grpc_server.py:2198-2199). `request.user_id` REQUIRED
  (`INVALID_ARGUMENT`, grpc_server.py:2200-2201).
- Trusted-actor pattern: `_is_self_or_admin` calls
  `get_authoritative_actor(actor)` (grpc_server.py:2071-2086,
  auth/auth_interceptor.py:92) and ignores the request payload for
  the privilege decision. This is the post-#168 invariant — Go port
  MUST resolve the trusted identity via the AuthInterceptor
  equivalent (gRPC metadata / context value) and NEVER trust the
  payload `actor` for the auth check.
- Authorization rule (grpc_server.py:2202-2206):
  - admin / system → allowed (`trusted` starts with `system:`,
    `admin:`, or equals `__system__`).
  - self → allowed when `trusted == "user:" + user_id` or
    `trusted == user_id`.
  - else → `PERMISSION_DENIED` with message
    `"UpdateUser requires the user themselves or admin actor"`.
- Tenant scoping: **none**. UpdateUser writes to the global registry,
  not a per-tenant SQLite. No `_check_tenant` is called.
- Configuration gate: if `self.global_store` is unset, abort
  `UNIMPLEMENTED "User registry not configured"`
  (grpc_server.py:2192-2196).

## Side effects

- Writes a single `UPDATE user_registry SET … WHERE user_id = ?` to the
  global SQLite via `_sync_update_user` (global_store.py:404-409),
  off-loaded from the asyncio loop by `_run_sync` (thread pool).
- **NOT WAL-routed today.** The user registry lives in `global_store`
  and is not event-sourced through `TransactionEvent` /
  `Applier.apply_event`. This is a deliberate carve-out for cross-
  tenant identity rows; per CLAUDE.md the per-tenant data plane is
  WAL-sourced, but the global registry tables are direct SQLite. Go
  port MUST preserve this — do **not** route `UpdateUser` through
  `wal.append`. If event-sourcing the global registry is desired, it
  is a separate epic and a proto change.
- Metrics: `record_grpc_request("UpdateUser", "ok"|"error", elapsed)`
  (grpc_server.py:2217, 2222, 2227). Note "ok" is recorded even for
  in-band failures (no-fields, not-found) because no abort fires.
- No audit-log write. Compliance trail for global-registry mutations
  is not in scope of this RPC; `audit/compliance.py` exports the WAL,
  which does not contain user-registry edits.

## Error contract

| Condition                     | Channel                  | Status / value                                        |
|-------------------------------|--------------------------|-------------------------------------------------------|
| `global_store` unset          | `context.abort`          | `UNIMPLEMENTED "User registry not configured"`        |
| empty `actor`                 | `context.abort`          | `INVALID_ARGUMENT "actor is required"`                |
| empty `user_id`               | `context.abort`          | `INVALID_ARGUMENT "user_id is required"`              |
| not self, not admin           | `context.abort`          | `PERMISSION_DENIED "UpdateUser requires the user themselves or admin actor"` |
| no mutable fields set         | in-band response         | `success=false, error="No fields to update"`          |
| user not found                | in-band response         | `success=false, error="User not found"`               |
| unhandled exception           | in-band response         | `success=false, error=str(e)`; metric label `error`   |

Go port note: in Go we'd normally promote the catch-all to
`codes.Internal` — **do not**, until contract tests are loosened. The
Python handler returns `error=str(e)` in-band on the same response
type, and contract tests assert success/failure on the response body
not on status codes for these arms.

## Shared Go package deps

- `entdbserver/auth` — `GetAuthoritativeActor(ctx, payloadActor) string`
  (port of `auth/auth_interceptor.py:92`) and `AuthInterceptor`
  unary interceptor that stuffs the trusted actor into `context.Context`.
- `entdbserver/globalstore` — `UpdateUser(ctx, userID string, fields UserUpdate) (bool, error)` returning `(updated, err)`.
  `UserUpdate` is a struct of `*string` (or three booleans + values)
  to mirror truthy-only gating. Backed by per-process `*sql.DB` over
  the global SQLite file.
- `entdbserver/metrics` — `RecordGRPCRequest(method, outcome string, elapsed time.Duration)` (port of `record_grpc_request`).
- `entdbpb` — generated `entdb.v1` Go bindings (UpdateUserRequest,
  UpdateUserResponse, UserInfo).
- `google.golang.org/grpc/status` and `codes` for aborts.

## Other-RPC deps

- `CreateUser` (grpc_server.py near line 2100) — must run first to
  populate the row that `UpdateUser` mutates. Contract test fixture
  preseeds `alice` and `bob`.
- `GetUser` (grpc_server.py:2120-ish) — used by tests to verify the
  post-update state. Not called by the handler.
- `_is_self_or_admin` shared with TransferUser / RevokeUser /
  DeleteUser style RPCs (grpc_server.py:2944, 2997, 3028, 3086) — Go
  port should expose `auth.IsSelfOrAdmin(ctx, userID)` once and reuse.

## Contract tests pinning behavior

- `tests/python/unit/test_user_registry.py:255-271` — happy self update
  passes only `name=` kwarg (proves truthy-only gating: empty `email`
  and `status` are not forwarded to `update_user`).
- `tests/python/unit/test_user_registry.py:273-290` — admin actor
  (`admin:root`) can update another user; only `status=` is forwarded.
- `tests/python/unit/test_user_registry.py:292-309` — non-admin
  updating another user → `context.abort` with the
  "user themselves or admin" message; store NOT called.
- `tests/python/unit/test_user_registry.py:311-328` — store returns
  False → response `success=false, error contains "not found"`.
- `tests/python/unit/test_user_registry.py:330-345` — no mutable fields
  → `success=false, error contains "no fields"`. **No abort.**
- `tests/python/integration/test_grpc_contract.py:448-453` — happy:
  Alice updating her own row returns `success=true`.
- `tests/python/integration/test_grpc_contract.py:454-458` — Alice
  attempting to update Bob → `PERMISSION_DENIED`.

These seven cases form the Go-port acceptance bar. Wire them into
`tests/contract/` once the Go server speaks `entdb.v1.EntDBService`.

## Implementation outline

```go
func (s *Server) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
    start := time.Now()
    defer func() { metrics.RecordGRPCRequest("UpdateUser", outcome, time.Since(start)) }()

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
            "UpdateUser requires the user themselves or admin actor")
    }

    var u globalstore.UserUpdate
    if req.GetEmail()  != "" { u.Email  = ptr(req.GetEmail())  }
    if req.GetName()   != "" { u.Name   = ptr(req.GetName())   }
    if req.GetStatus() != "" { u.Status = ptr(req.GetStatus()) }
    if u.IsEmpty() {
        return &pb.UpdateUserResponse{Success: false, Error: "No fields to update"}, nil
    }

    updated, err := s.globalStore.UpdateUser(ctx, req.GetUserId(), u)
    if err != nil {
        return &pb.UpdateUserResponse{Success: false, Error: err.Error()}, nil
    }
    if !updated {
        return &pb.UpdateUserResponse{Success: false, Error: "User not found"}, nil
    }
    return &pb.UpdateUserResponse{Success: true}, nil
}
```

`globalstore.UpdateUser` builds the `UPDATE … SET …` dynamically from
the non-nil `*string` fields, sets `updated_at = now()`, and returns
`rowsAffected > 0` (mirror global_store.py:396-409). Use a single
transaction; the table is in the global DB so no per-tenant routing.

## Open questions / risks

- **Concurrent updates / lost writes.** No `version` / `etag` / OCC
  column on `user_registry`. Two concurrent `UpdateUser`s racing on
  the same `user_id` last-writer-wins on a per-column basis. SQLite
  serializes writes, so there is no torn row, but a `name` update
  from caller A and a `status` update from caller B issued
  concurrently both succeed and produce a merged row — which is
  probably the desired behavior, but it is undocumented. Go port
  should preserve last-write-wins (do not silently add OCC).
- **No way to clear a field.** Truthy-only gating means `email=""`
  cannot blank an email. Tracked separately if/when proto adds
  `optional` presence.
- **`status` is unconstrained.** Free-form string; clients can write
  anything. `ListUsers` filters by `status="active"` by default
  (global_store.py:411-424), so a typo silently hides the user from
  default listings. Consider a server-side enum check (proto change,
  separate epic).
- **No audit trail.** Global-registry mutations are not WAL-sourced
  and not S3-Object-Lock-tamper-evident. If GDPR/SOC2 demand an
  immutable record of identity changes, that is a follow-up epic
  (event-source the global registry through a `GlobalEvent` topic).
- **`success=false` vs gRPC status.** Mixing in-band failure flags
  with `context.abort` makes client error handling awkward. Go port
  must keep the split for parity but flag this for a v2 cleanup.
- **Idempotency.** Re-issuing the same `UpdateUser` is naturally
  idempotent (same `SET` produces the same row, `updated_at` ticks).
  No request-ID dedup; acceptable given last-write-wins semantics.
