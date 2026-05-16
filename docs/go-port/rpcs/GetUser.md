# RPC Port Spec — `entdb.v1.EntDBService/GetUser`

> Implementation: `server/go/internal/api/get_user.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407 — Python → Go server port. Source of truth: Go handler at
`server/go/internal/api/get_user.go`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:113` (rpc), `:816-824` (request/response),
`:793-800` (`UserInfo`).

`GetUserRequest`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `actor` | 1 | `string` | UNTRUSTED on the wire. Required for argument validation, but the auth interceptor overrides it with the authenticated identity (CLAUDE.md trusted-actor invariant; commit `fece3fb`). Empty → `INVALID_ARGUMENT`. |
| `user_id` | 2 | `string` | Required. Empty → `INVALID_ARGUMENT`. Bare ID (e.g. `"alice"`), NOT the principal form `"user:alice"`. |

`GetUserResponse`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `found` | 1 | `bool` | False when the user_id is not in `user_registry` AND in the catch-all error branch (`server/go/internal/api/get_user.go` — internal errors are swallowed and reported as `found=false`). |
| `user`  | 2 | `UserInfo` | Populated only when `found=true`. `UserInfo{user_id, email, name, status, created_at, updated_at}`; `created_at`/`updated_at` are `int64` unix seconds. |

This is a unary RPC. No streaming, no `RequestContext`, no `tenant_id` —
GetUser is global, not per-tenant.

## Auth (self only? admin? trusted-actor)

- **Authenticated, but unrestricted.** The handler enforces no role/permission
  beyond "actor is non-empty". Docstring at `server/go/internal/api/get_user.go`: *"Available
  to any authenticated actor."* Pinned by `test_user_registry.py:193-245` and
  `test_grpc_contract.py:396-416`.
- **Wire `actor` is UNTRUSTED.** Per CLAUDE.md and commit `fece3fb` the Go
  handler MUST resolve identity from gRPC metadata via the auth interceptor
  (mirror `auth/auth_interceptor.py:92` `get_authoritative_actor`). The wire
  field exists only so the contract test suite (which runs without an auth
  interceptor — `test_grpc_contract.py:394`) can drive the validation branch.
- **NOT in `UNAUTHENTICATED_METHODS`** (`auth_interceptor.py:157-184`); the
  caller must present valid credentials in production.
- **No `Permission` check, no ACL, no admin-only gate.** Any logged-in actor
  may read any user's profile. If EPIC #407 wants to tighten this to
  "self-or-admin", that is a NEW behavior and needs its own ticket — do NOT
  smuggle it into the port.
- Trusted-actor self-check helper exists at `server/go/internal/api/get_user.go`
  (`_is_self_or_admin`) but is intentionally NOT called from `GetUser`.

## Side effects (read on global_store)

**None — pure read.** No WAL append, no SQLite write, no metrics label
beyond the histogram. The WAL invariant in CLAUDE.md does not apply: this
RPC mutates nothing.

In-order narration of the Python handler:

1. `start := time.perf_counter()` for histogram timing.
2. Guard: if `self.global_store is None` → `context.abort(UNIMPLEMENTED, "User registry not configured")` (`server/go/internal/api/get_user.go`). Pinned by `test_user_registry.py:433-444`.
3. Validate `request.actor != ""` → else `INVALID_ARGUMENT` (`:2163`).
4. Validate `request.user_id != ""` → else `INVALID_ARGUMENT` (`:2165`).
5. `user := global_store.get_user(user_id)` — single SQLite SELECT against the global DB: `SELECT * FROM user_registry WHERE user_id = ?` (`server/go/internal/globalstore/`). Async wrapper around a thread-pool sync call.
6. If `user is None`: record `("GetUser","ok",elapsed)`; return `GetUserResponse{found:false}`.
7. Else: record `("GetUser","ok",elapsed)`; return `GetUserResponse{found:true, user: _user_dict_to_proto(user)}` (`server/go/internal/api/get_user.go-…`).
8. Catch-all `except Exception`: log + record `("GetUser","error",elapsed)`; return `found:false` (`:2179-2182`). The Go port should preserve "swallow into found=false" only for genuinely unexpected errors; abort-driven control flow (`UNIMPLEMENTED`/`INVALID_ARGUMENT`) must propagate.

NO touch to: `canonical_store`, `wal`, `applier`, `acl`, `capability_registry`,
`schema_registry`, `crypto`, `audit`, `quota`. Importing any of these in the
Go handler is a smell.

## Error contract (NOT_FOUND vs PERMISSION_DENIED)

| gRPC code | Trigger | Pin |
|-----------|---------|-----|
| `OK` + `found=true` | user exists | `test_grpc_contract.py:396-400` |
| `OK` + `found=false` | user does not exist (the **NOT_FOUND signal is in-band**, not a status code) | `test_grpc_contract.py:402-406`, `test_user_registry.py:219-231` |
| `INVALID_ARGUMENT` | `actor==""` or `user_id==""` | `test_grpc_contract.py:407-416`, `test_user_registry.py:233-245` |
| `UNIMPLEMENTED` | server started without a global store backing | `test_user_registry.py:433-444` |
| `UNAUTHENTICATED` | missing/invalid credentials — emitted by the auth interceptor, NOT by this handler | `auth_interceptor.py:186+` |
| `PERMISSION_DENIED` | **never emitted by this handler.** Reading another user's profile is allowed; do NOT introduce a self/admin gate during the port. | — |
| `OK` + `found=false` (swallow) | uncaught internal error | `server/go/internal/api/get_user.go` (no test pin; preserve to avoid leaking stack frames). The Go port should additionally `log.Error` with the original error. |

Note the deliberate asymmetry: missing user is `OK + found=false` (treated as
a successful absence, like `GetNode` per `docs/go-port/rpcs/GetNodes.md`), not
`NOT_FOUND`. Do NOT "fix" this in Go — it would break clients that branch on
`found`.

## Shared Go package deps

Each is a new package under `server/go/internal/...`.

- `pb` (`server/go/internal/pb/entdbv1`) — generated `GetUserRequest`, `GetUserResponse`, `UserInfo`, servicer interface. Required.
- `globalstore` — `GetUser(ctx, userID string) (*User, error)` returning `(nil, nil)` on miss to mirror Python `None`. Mirrors `server/go/internal/globalstore/`. Required.
- `metrics` — `RecordGRPCRequest(method, status string, dur time.Duration)` (mirrors `metrics.py`). Required.
- `auth` — exposes `AuthoritativeActor(ctx) (string, error)` reading from incoming metadata. The handler does not USE the resolved actor for an authorization decision, but the interceptor must still run; the package is a transitive dep via the server bootstrap, not a direct import in `getuser.go`.
- `errs` — helpers like `errs.InvalidArgument(field string)` and `errs.Unimplemented(msg string)` for consistent `status.Error` construction. Required.

NOT used: `wal`, `apply`, `canonicalstore`, `acl`, `capability`, `schema`,
`crypto`, `audit`, `quota`, `sharding`. Importing any of them is a smell.

## Other-RPC deps

- `CreateUser` (`server/go/internal/api/get_user.go`) — populates the `user_registry` row that GetUser reads. The Go port for `CreateUser` MUST land before or with this RPC, otherwise the happy-path contract test cannot seed `alice`.
- `UpdateUser` (`:2184-…`) — mutates the row that GetUser returns. Not a hard dep, but the port order should keep them adjacent.
- `ListUsers` (`:2230-…`) — shares `_user_dict_to_proto` and the same SQL table. Co-port to share the helper.
- `GetUserTenants` (`:2572-2599`) — separate cross-table join (`tenant_memberships`); explicitly NOT this RPC. Avoid conflating.
- `Health` — independent.

## Contract tests pinning behavior (file:line)

- `tests/python/integration/test_grpc_contract.py:396-400` — happy: `actor=ALICE, user_id="alice"` → `found=true`, `user.user_id == "alice"`. Runs over a real gRPC channel. **The Go server must pass this verbatim.**
- `tests/python/integration/test_grpc_contract.py:401-406` — not_found: `user_id="ghost"` → `found=false`, no abort.
- `tests/python/integration/test_grpc_contract.py:407-411` — invalid_argument: empty `actor` aborts.
- `tests/python/integration/test_grpc_contract.py:412-416` — invalid_argument: empty `user_id` aborts.
- (legacy Python unit test, removed in Phase 4D) — happy unit (mock global_store): `_user_dict_to_proto` mapping correctness.
- (legacy Python unit test, removed in Phase 4D) — missing user → `found=false`.
- (legacy Python unit test, removed in Phase 4D) — empty actor → `context.abort` called with `"actor"` in the message.
- (legacy Python unit test, removed in Phase 4D) — `global_store=None` → `UNIMPLEMENTED` "not configured".
- (No PERMISSION_DENIED pin exists, by design.)

## Implementation outline

```go
// server/go/internal/api/getuser.go
func (s *EntDBServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
    start := time.Now()
    status := "ok"
    defer func() { metrics.RecordGRPCRequest("GetUser", status, time.Since(start)) }()

    if s.globalStore == nil {
        status = "error"
        return nil, errs.Unimplemented("User registry not configured")
    }
    if req.GetActor() == "" {
        status = "error"
        return nil, errs.InvalidArgument("actor is required")
    }
    if req.GetUserId() == "" {
        status = "error"
        return nil, errs.InvalidArgument("user_id is required")
    }

    user, err := s.globalStore.GetUser(ctx, req.GetUserId())
    if err != nil {
        log.Error("GetUser failed", "err", err)
        status = "error"
        return &pb.GetUserResponse{Found: false}, nil // mirror Python swallow
    }
    if user == nil {
        return &pb.GetUserResponse{Found: false}, nil
    }
    return &pb.GetUserResponse{Found: true, User: userToProto(user)}, nil
}
```

`userToProto` mirrors `_user_dict_to_proto` (`server/go/internal/api/get_user.go-…`):
copy `user_id`, `email`, `name`, `status`, `created_at`, `updated_at`;
default each to its zero value when the column is NULL.

## Open questions / risks

- **Should auth tighten to self-or-admin?** Python intentionally allows any
  authenticated actor to read any user. The helper `_is_self_or_admin`
  exists but is unused here. Confirm with EPIC #407 owner: the port should
  preserve open-read, OR a NEW spec ticket should authorize the tightening.
  Don't change it implicitly.
- **In-band `found=false` vs `NOT_FOUND` status.** Existing clients (Python
  SDK, Go SDK in `sdk/go/entdb`) branch on `response.Found`. Switching to
  status-code `NOT_FOUND` is a contract break.
- **Error swallowing in step 8.** Returning `found=false` on internal error
  hides backend outages from callers and inflates the `ok` metric. Port
  must preserve for parity, but file a follow-up to surface as `INTERNAL`
  once SDK clients tolerate it.
- **`actor` field staying on the wire.** Per CLAUDE.md, ignored for auth.
  Go port should still validate non-empty so contract tests `407-416` pass,
  but never feed it into an authorization decision.
- **Concurrency / thread-pool bridge.** Python uses `_run_sync` to push the
  SQLite call onto a thread pool. Go uses a real connection pool — confirm
  `globalstore.GetUser` is safe under concurrent callers (use `database/sql`
  with `SetMaxOpenConns`, not raw `*sql.Conn`).
- **Cardinality of metrics labels** is fixed (`GetUser` × {`ok`,`error`});
  no risk.
- **SQL injection surface.** Python uses parameterized `?`. Go must do the
  same — never `fmt.Sprintf` `user_id` into the query.
