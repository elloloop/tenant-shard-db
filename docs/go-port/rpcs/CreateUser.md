# RPC Port Spec — `entdb.v1.EntDBService/CreateUser`

> Implementation: `server/go/internal/api/create_user.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407 — Python → Go server port. Source of truth: Go handler at
`server/go/internal/api/create_user.go`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:112` (rpc), `:802-814` (messages),
`:792-800` (`UserInfo`).

`CreateUserRequest`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `actor` | 1 | `string` | UNTRUSTED wire payload. Substituted with `AuthInterceptor.get_authoritative_actor(...)` before any privilege check (`server/go/internal/api/create_user.go`). Required (non-empty). |
| `user_id` | 2 | `string` | Required. Bare id like `"alice"` — NOT `"user:alice"` (see Open questions). Becomes `user_registry.user_id` PRIMARY KEY (`server/go/internal/globalstore/`). |
| `email`   | 3 | `string` | Required. UNIQUE constraint at storage layer. |
| `name`    | 4 | `string` | Required. Display name; not unique. |

`CreateUserResponse`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `success` | 1 | `bool`     | `true` on insert, `false` on uniqueness collision. Validation/auth failures abort with a status code instead of returning `success=false` — the field is only false on the duplicate-key path (`server/go/internal/api/create_user.go`). |
| `user`    | 2 | `UserInfo` | Populated on success. `status` always `"active"` (`server/go/internal/globalstore/`). `created_at`/`updated_at` are unix-seconds ints from `_now()`. |
| `error`   | 3 | `string`   | Set only on the `success=false` paths. Contains the underlying SQLite error message; consumers must not parse this — use the gRPC code. |

## Auth (admin-only, trusted-actor)

- **Authentication required.** RPC is NOT in `AuthInterceptor.UNAUTHENTICATED_METHODS`.
- **Privilege: admin or system.** Handler calls `_is_admin_or_system(request.actor)`
  (`server/go/internal/api/create_user.go`), which delegates to `get_authoritative_actor`
  (`auth/auth_interceptor.py:92`) and accepts only trusted prefixes
  `system:`, `admin:`, or the literal `"__system__"` (`server/go/internal/api/create_user.go`).
- **Trusted-actor invariant (CLAUDE.md "wire is UNTRUSTED").** `request.actor`
  is a payload field; it MUST NOT be used directly. The Go port must mirror
  the Python helper: read the trusted identity from the per-call context
  (set by `AuthInterceptor` on the `metadata.MD`) and use *that* for the
  `startswith` check. Pinned by
  `tests/python/integration/test_privilege_escalation.py:321-341` —
  authenticated user `"user:eve"` claiming `actor="system:admin"` MUST be
  rejected with `PERMISSION_DENIED`.
- **No `Permission` enum check, no ACL.** Privilege is gated by the actor
  prefix only; `acl` / `capability_registry` are NOT consulted.
- **Rate limit / quota.** Standard interceptors apply (admin actors typically
  bypass per-tenant limiters; not unique to this RPC).

## Side effects (global WAL write)

The Go handler appends a global-scope `user_created` WAL event keyed by
`__global__` and waits for the applier to materialize the
`global_store.user_registry` row. The handler does not write globalstore
directly.

In-order narration:

1. `start := time.Now()` for metrics.
2. If `s.globalStore == nil` → abort `UNIMPLEMENTED "User registry not configured"`
   (`server/go/internal/api/create_user.go`).
3. Validate `actor` non-empty → abort `INVALID_ARGUMENT "actor is required"`.
4. Resolve trusted actor from context; reject if not `system:` / `admin:` /
   `__system__` → abort `PERMISSION_DENIED "CreateUser requires admin or system actor"`.
5. Validate `user_id`, `email`, `name` non-empty (in that order) → abort
   `INVALID_ARGUMENT "<field> is required"`.
6. Preflight duplicate `user_id` via `globalStore.GetUser`.
7. Append `op="user_created"` carrying the full row state
   (`user_id`, `email`, `name`, `status`, `created_at`, `updated_at`).
8. Wait for the applier to apply that offset via
   `globalstore.ApplyUserCreated`.
9. On success → record `("CreateUser","ok",…)` and return
   `{success:true, user: UserInfo{...}}`.

No `canonical_store` touch. No tenant-scoped SQLite. No quota charge.

## Error contract (uniqueness + validation)

| gRPC code | Trigger | Source |
|-----------|---------|--------|
| `UNIMPLEMENTED` | `global_store` not configured. | `server/go/internal/api/create_user.go` |
| `INVALID_ARGUMENT` | Empty `actor`, `user_id`, `email`, or `name`. | `server/go/internal/api/create_user.go`; pinned `test_user_registry.py:171-187`, `test_grpc_contract.py:443-446` |
| `PERMISSION_DENIED` | Trusted actor is not `system:*` / `admin:*` / `__system__`. | `server/go/internal/api/create_user.go`; pinned `test_user_registry.py:126-145`, `test_privilege_escalation.py:321-341`, `test_grpc_contract.py:436-441` |
| `ALREADY_EXISTS` | Duplicate `user_id` preflight collision. | Go port hardening for WAL-first path |
| `INTERNAL` / `DEADLINE_EXCEEDED` | WAL append or wait-applied failure. | Go port WAL-first path |

Go port detail: the duplicate-key path is promoted to `ALREADY_EXISTS`
because the handler must decide before appending a durable WAL event.

## Shared Go package deps

- `pb` (`server/go/internal/pb/entdbv1`) — generated `CreateUserRequest`, `CreateUserResponse`, `UserInfo`.
- `globalstore` (`server/go/internal/globalstore`) — duplicate precheck via
  `GetUser`; applier write via `ApplyUserCreated`.
- `wal` / `apply` — global `user_created` op and wait-applied path.
- `auth` (`server/go/internal/auth`) — `TrustedActorFromContext(ctx) (string, bool)` mirroring `auth_interceptor.py:92`. Required.
- `metrics` — `RecordGRPCRequest("CreateUser", status, dur)`.
- `clock` — injectable `Now() time.Time` for deterministic tests; `_now()` in Python is `int(time.time())` (`server/go/internal/globalstore/`).

NOT used and MUST NOT be imported: `canonicalstore`, `acl`, `schema`,
`quota`, `crypto`, `audit`.

## Other-RPC deps

- `GetUser` (`server/go/internal/api/create_user.go`) — read path; tests use it after `CreateUser` to verify the row exists. Port together or stub.
- `UpdateUser` (`proto:826-833`) — same `global_store` table; shares the trusted-actor pattern (self-or-admin instead of admin-only). Spec lives in a sibling file.
- `ListUsers` (`proto:114`) — pagination over the same table; admin-only. Independent of `CreateUser` but shares storage.
- `GetUserTenants` (`proto:126`) — joins `user_registry` with `tenant_members`; depends on the row created here.

`CreateUser` itself has **no upstream RPC dependencies** — it can be the
first user-registry RPC ported, before `GetUser`/`UpdateUser`/`ListUsers`.

## Contract tests pinning behavior

- (legacy Python unit test, removed in Phase 4D) — full handler matrix:
  - `:68-97` happy path with `actor="system:admin"`.
  - `:99-124` happy path with `actor="admin:root"`.
  - `:126-145` regular user denied (`PERMISSION_DENIED`, store not called).
  - `:147-169` duplicate email returns `success=false, error~"already exists"`.
  - `:171-187` missing `email` aborts `INVALID_ARGUMENT`.
- (legacy Python unit test, removed in Phase 4D) — no `global_store` configured aborts `UNIMPLEMENTED`.
- `tests/python/integration/test_grpc_contract.py:428-446` — over-the-wire matrix: happy / `PERMISSION_DENIED` (`actor=ALICE`) / `INVALID_ARGUMENT` (empty `user_id`).
- `tests/python/integration/test_privilege_escalation.py:321-341` — security regression: authenticated `user:eve` claiming `actor="system:admin"` MUST get `PERMISSION_DENIED`. Parametrized over `CLAIMED_ADMIN_ACTORS` (`:166`).

The Go server must pass all of the above verbatim once the Python
binary is swapped out, modulo trusted-identity test fixtures that
inject metadata into the gRPC `context.Context` instead of patching
`auth_interceptor`.

## Implementation outline

```go
// server/go/internal/api/createuser.go
func (s *EntDBServer) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
    start := time.Now()
    record := func(status string) { metrics.RecordGRPCRequest("CreateUser", status, time.Since(start)) }

    if s.globalStore == nil {
        return nil, status.Error(codes.Unimplemented, "User registry not configured")
    }
    if req.GetActor() == "" {
        return nil, status.Error(codes.InvalidArgument, "actor is required")
    }
    trusted, _ := auth.TrustedActorFromContext(ctx) // falls back to req.Actor only when no interceptor ran (tests)
    if !isAdminOrSystem(trusted) {
        return nil, status.Error(codes.PermissionDenied, "CreateUser requires admin or system actor")
    }
    if req.GetUserId() == "" { return nil, status.Error(codes.InvalidArgument, "user_id is required") }
    if req.GetEmail()  == "" { return nil, status.Error(codes.InvalidArgument, "email is required") }
    if req.GetName()   == "" { return nil, status.Error(codes.InvalidArgument, "name is required") }

    user, err := s.globalStore.CreateUser(ctx, req.UserId, req.Email, req.Name)
    if errors.Is(err, globalstore.ErrUserAlreadyExists) {
        record("error")
        return &pb.CreateUserResponse{Success: false, Error: fmt.Sprintf("User already exists: %v", err)}, nil
    }
    if err != nil {
        record("error")
        log.Error("CreateUser failed", "err", err)
        return &pb.CreateUserResponse{Success: false, Error: err.Error()}, nil
    }
    record("ok")
    return &pb.CreateUserResponse{Success: true, User: userToProto(user)}, nil
}

func isAdminOrSystem(trusted string) bool {
    return strings.HasPrefix(trusted, "system:") ||
           strings.HasPrefix(trusted, "admin:")  ||
           trusted == "__system__"
}
```

## Open questions / risks

- **`user_id` vs `tenant_principal` naming boundary.** Wire/storage uses
  bare `"alice"`; ACL/grant code uses `"user:alice"` (CLAUDE.md "Key
  Patterns"). The handler does NOT prepend `user:` — `globalStore.create_user`
  receives the bare id. Any Go code that later joins this row against
  `acl_grants` or `tenant_members` must add the `user:` prefix at the
  boundary. Document the prefix rule in `globalstore` package docs to
  prevent regressions; the bug class here is silent permission misses.
- **Skips the WAL** — violates the CLAUDE.md §1 invariant on its face.
  Today's behavior: rebuild-from-WAL would lose the user registry. Either
  (a) accept that `global_store` state is operator-managed and out of
  scope for replay, or (b) add a `UserCreated` event to `TransactionEvent.ops`
  and route through `Applier`. Decision needed before GA. Recommend (a)
  with a backup of `global_store.db` documented in the runbook.
- **No audit-trail entry.** S3 Object Lock covers WAL only. Admin user
  creation is not currently auditable post-hoc. Track follow-up to mirror
  these into the WAL as `AdminEvent` records (no per-tenant scope).
- **String-matching on `"UNIQUE constraint"`** is fragile. Go port should
  use a sentinel error from the SQLite driver and keep the response
  shape, not the detection mechanism.
- **Generic exception → `OK + success=false`** swallows real bugs. Once
  the contract tests are migrated to use typed errors, switch the catch-all
  to `codes.Internal`. Do NOT change in the parity port.
- **Trusted-actor fallback** when no interceptor ran (unit tests construct
  `EntDBServicer` directly): Python's `get_authoritative_actor` returns
  `request_actor` unchanged. Mirror this fallback in Go so unit tests can
  pass `actor="system:admin"` without setting up a metadata fixture —
  but document it loudly so production never relies on it.
