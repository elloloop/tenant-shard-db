# RPC Port Spec — `entdb.v1.EntDBService/ListUsers`

> Implementation: `server/go/internal/api/list_users.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407 — Python -> Go server port. Source of truth: Python handler at
`server/go/internal/api/list_users.go`.

## Wire contract (filter, pagination)

Proto: `proto/entdb/v1/entdb.proto:115` (rpc), `:840-849` (messages).

`ListUsersRequest`:
| Field | Tag | Type | Default | Notes |
|------|-----|------|---------|------|
| `actor` | 1 | `string` | — | UNTRUSTED wire claim. Required; empty -> `INVALID_ARGUMENT`. NOT used to filter results — only gates the call. See "Auth". |
| `status` | 2 | `string` | `"active"` | Free-form filter; SQL `WHERE status = ?` exact match against `user_registry.status`. Empty string coerces to `"active"` (`server/go/internal/api/list_users.go`). No allow-list — values like `"suspended"`, `"deleted"`, `"frozen"` flow through verbatim. |
| `limit` | 3 | `int32` | `100` | `0` (unset) coerces to `100` (`server/go/internal/api/list_users.go`). NO upper cap is enforced today; Go port should mirror exactly (file follow-up to cap at e.g. 1000). Negative values flow into SQLite `LIMIT ?` and behave as "no limit" — preserve, do not "fix". |
| `offset` | 4 | `int32` | `0` | `0`-coerce only on falsy. Pagination is offset-based, not cursor-based; no stable ordering guarantee beyond `ORDER BY created_at` (`server/go/internal/globalstore/`). |

`ListUsersResponse`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `users` | 1 | `repeated UserInfo` | Each row mapped via `_user_dict_to_proto` (`server/go/internal/api/list_users.go`): `{user_id, email, name, status, created_at, updated_at}` — missing keys -> empty string / `0`. |

No `has_more`, no `total_count`, no `next_page_token`. Caller infers end-of-list
when `len(users) < limit`. Go port MUST NOT add a `has_more` field — that is
a wire change.

## Auth (admin scope; trusted-actor)

- **Authentication required.** `ListUsers` is NOT in
  `AuthInterceptor.UNAUTHENTICATED_METHODS` (`auth/auth_interceptor.py:157-162`).
  The auth interceptor runs and stamps the trusted actor into a `ContextVar`.
- **Admin scope: NO.** Despite the wire field name suggesting otherwise, the
  handler today only checks `request.actor` is non-empty — any authenticated
  caller (including `user:alice`) can list users. The contract test at
  `test_user_registry.py:395` exercises this with `actor="user:u1"` and expects
  success. The handler docstring says "Available to any authenticated actor"
  (`server/go/internal/api/list_users.go`).
- **Trusted-actor invariant: ENFORCE BUT BENIGN.** Per CLAUDE.md and the
  privilege-escalation fix in commit `fece3fb`, every handler MUST rebind
  `actor` to `_trusted_actor(request.actor)` before any privilege-bearing use.
  `ListUsers` does no privilege check today (the wire `actor` is only tested
  for emptiness), so the bug surface is small — but the Go port MUST still:
  1. Read the trusted identity from the auth context (Go equivalent of
     `get_authoritative_actor`).
  2. Use that identity for any audit/log line — never the wire claim.
  3. Validate `request.actor != ""` exactly as Python does, to keep contract
     test `test_grpc_contract.py:423-427` passing.
- **No `Permission` / capability check.** Handler does not consult `acl` or
  `capability_registry`. (Future: should gate on `users.list` admin
  capability — see "Open questions".)
- **Rate limit.** Subject to default per-tenant `RateLimitInterceptor` since
  not in any bypass list. Go port: same.

## Side effects (read on global_store)

**Pure read.** No WAL append, no SQLite write, no `canonical_store` touch
(per-tenant SQLite untouched — this is a global-plane RPC).

In-order narration of the handler (`server/go/internal/api/list_users.go`):

1. `start = time.perf_counter()` for metrics.
2. Guard: `self.global_store is None` -> abort `UNIMPLEMENTED`
   `"User registry not configured"` (`:2239-2243`).
3. Guard: `request.actor == ""` -> abort `INVALID_ARGUMENT` `"actor is required"`
   (`:2245-2246`).
4. Coerce defaults: `status = request.status or "active"`,
   `limit = request.limit or 100`, `offset = request.offset or 0`
   (`:2248-2250`).
5. `await self.global_store.list_users(status=, limit=, offset=)` —
   reads global-plane SQLite via `_run_sync` thread pool
   (`server/go/internal/globalstore/`). SQL: `SELECT * FROM user_registry WHERE
   status = ? ORDER BY created_at LIMIT ? OFFSET ?`.
6. Map each row through `_user_dict_to_proto` (`:2088-2097`).
7. `record_grpc_request("ListUsers", "ok", elapsed)` (`:2260`).
8. Return `ListUsersResponse(users=proto_users)`.

This RPC reads ONLY from `global_store` (the cross-tenant store). Per
CLAUDE.md invariant #4 it must NOT open any per-tenant SQLite handle.

## Error contract

| gRPC code | Trigger | Site |
|-----------|---------|------|
| `OK` | Happy path, including empty result. | `:2261` |
| `OK` (with `users=[]`) | Any unexpected exception in the inner `try`. Python's outer `except Exception` swallows the error, logs it, and returns an empty response with `users=[]` (`:2262-2265`). Go port MUST mirror this swallow-behavior so contract tests pass; track as a wart for follow-up — silently masking DB errors is bad. |
| `INVALID_ARGUMENT` | `actor == ""` (`:2245-2246`). Pinned by `test_grpc_contract.py:423-427`. |
| `UNIMPLEMENTED` | `global_store` is `nil` (server started without user-registry config) (`:2239-2243`). Message verbatim: `"User registry not configured"`. |
| `UNAUTHENTICATED` | Surfaced by `AuthInterceptor`, not the handler — missing/invalid token. |
| `RESOURCE_EXHAUSTED` | Surfaced by `RateLimitInterceptor`. |

The handler MUST NOT abort with `PERMISSION_DENIED` (no scope check today)
nor `NOT_FOUND` (empty list is not an error).

## Shared Go package deps

Each package under `server/go/internal/...` unless noted.

- `pb` (`server/go/internal/pb/entdbv1`) — generated `ListUsersRequest`,
  `ListUsersResponse`, `UserInfo`, servicer interface. Required.
- `globalstore` — `ListUsers(ctx, status string, limit, offset int) ([]User, error)`
  mirroring `global_store.list_users` (`server/go/internal/globalstore/`). Required.
  Must use the global-plane SQLite handle, NOT a tenant handle.
- `metrics` — `RecordGRPCRequest(method, status string, dur time.Duration)`
  (mirrors `metrics.py`). Required.
- `auth` — read trusted actor from `context.Context` (`get_authoritative_actor`
  Go equivalent). Required for the trusted-actor invariant even though the
  result is not currently used for filtering.
- `convert` (or method on servicer) — `userDictToProto` mirroring `:2088-2097`.

NOT used and MUST NOT be imported by this handler: `wal`, `apply`,
`canonicalstore`, `acl`, `capability`, `schema`, `crypto`, `audit`, `quota`.
Importing any of them is a smell.

## Other-RPC deps

- `CreateUser`, `UpdateUser`, `DeleteUser`, `FreezeUser` — write paths that
  populate `user_registry`. `ListUsers` reads what they wrote. The Go port of
  `ListUsers` can be landed BEFORE these write RPCs are ported, provided the
  test fixture seeds rows directly via `globalstore.CreateUser` (mirrors what
  `test_grpc_contract.py` does at module setup).
- `Health` — independent; no ordering constraint.
- No fan-out to other RPCs; this is a leaf read.

## Contract tests pinning behavior (file:line)

- (legacy Python unit test, removed in Phase 4D) —
  `test_list_users_default`: `actor="system:admin"`, no other fields ->
  `global_store.list_users` called with `status="active", limit=100, offset=0`;
  proto users preserve `user_id` order from store.
- (legacy Python unit test, removed in Phase 4D) —
  `test_list_users_with_pagination`: `actor="user:u1"`, `status="suspended"`,
  `limit=10`, `offset=5` -> store called with those exact kwargs; non-admin
  actor is allowed.
- `tests/python/integration/test_grpc_contract.py:417-422` — happy-path:
  `ListUsersRequest(actor=ADMIN, limit=10)` returns `users` containing the
  `user_id="alice"` row over a real gRPC channel.
- `tests/python/integration/test_grpc_contract.py:423-427` — invalid-argument:
  `actor=""` -> `INVALID_ARGUMENT`.
- (No regression test today for `global_store=None` -> `UNIMPLEMENTED`. Go
  port should add one when it ports the handler.)

## Implementation outline

```go
// server/go/internal/api/list_users.go
func (s *EntDBServer) ListUsers(
    ctx context.Context, req *pb.ListUsersRequest,
) (*pb.ListUsersResponse, error) {
    start := time.Now()
    statusLabel := "ok"
    defer func() { metrics.RecordGRPCRequest("ListUsers", statusLabel, time.Since(start)) }()

    if s.globalStore == nil {
        statusLabel = "error"
        return nil, status.Error(codes.Unimplemented, "User registry not configured")
    }
    if req.GetActor() == "" {
        statusLabel = "error"
        return nil, status.Error(codes.InvalidArgument, "actor is required")
    }
    _ = auth.TrustedActor(ctx, req.GetActor()) // invariant: rebind even if unused.

    statusFilter := req.GetStatus()
    if statusFilter == "" { statusFilter = "active" }
    limit := int(req.GetLimit()); if limit == 0 { limit = 100 }
    offset := int(req.GetOffset())

    rows, err := s.globalStore.ListUsers(ctx, statusFilter, limit, offset)
    if err != nil {
        // Mirror Python swallow-behavior: log, return empty list, code OK.
        log.Errorw("ListUsers failed", "err", err)
        statusLabel = "error"
        return &pb.ListUsersResponse{}, nil
    }
    out := make([]*pb.UserInfo, 0, len(rows))
    for _, r := range rows { out = append(out, userToProto(r)) }
    return &pb.ListUsersResponse{Users: out}, nil
}
```

Notes:
1. Bind `_ = auth.TrustedActor(...)` (or drop into a named var used in audit
   logging) so the invariant cannot regress when a future scope check is added.
2. `userToProto` must preserve the empty-string / zero defaults from
   `_user_dict_to_proto` — never panic on missing fields.
3. `globalStore.ListUsers` should use a `context.Context`-aware DB call
   (`db.QueryContext`) so client cancellation cuts the SQL.

## Open questions / risks

- **Missing admin-scope check.** Today any authenticated user can list every
  user in the system — likely a leak. The Go port should preserve this for
  contract parity, then a follow-up ticket should gate on a `users.list`
  capability and update `test_list_users_with_pagination` accordingly. Confirm
  with EPIC #407 owner before adding the scope check in the same PR.
- **Silent error swallow.** Returning `OK` with `users=[]` on DB error
  (`:2262-2265`) hides outages and breaks pagination clients. Mirror today;
  open a follow-up to surface `INTERNAL` once a deprecation window is agreed.
- **No cap on `limit`.** A malicious caller can pass `limit=2_000_000_000`
  and OOM the server. Mirror Python's lack-of-cap for parity, but file a
  ticket to enforce `limit <= 1000` server-side.
- **No stable secondary sort.** `ORDER BY created_at` alone allows ties
  to reorder between pages. Go port: keep as-is for parity; follow-up to add
  `, user_id ASC` as tiebreaker.
- **Status filter is free-form.** No allow-list means typos like
  `status="actve"` silently return zero rows. Document in the SDK; do not
  change server behavior.
- **PII exposure.** `UserInfo` includes `email` and `name`. A scope check
  (above) is the real fix; until then, log-redaction must NOT log responses.
