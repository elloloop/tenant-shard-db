# RPC Port Spec — `entdb.v1.EntDBService/ChangeMemberRole`

> Implementation: `server/go/internal/api/change_member_role.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407 — Python → Go server port. Source of truth: Python handler at
`server/go/internal/api/change_member_role.go`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:127` (rpc), `:939-949` (messages).

`ChangeMemberRoleRequest`:
| Field       | Tag | Type   | Notes                                                                 |
|-------------|-----|--------|-----------------------------------------------------------------------|
| `actor`     | 1   | string | Caller principal, e.g. `user:alice` or `system:admin`. UNTRUSTED on the wire — see Auth. Required (`server/go/internal/api/change_member_role.go`). |
| `tenant_id` | 2   | string | Tenant whose membership is being modified. Required (`:2617-2618`).   |
| `user_id`   | 3   | string | Bare user id (no `user:` prefix) of the member being changed. Required (`:2619-2620`). |
| `new_role`  | 4   | string | Target role. Free-form string in current Python (`owner`, `admin`, `member` are observed in tests); no enum validation server-side. Required (`:2621-2622`). |

`ChangeMemberRoleResponse`:
| Field     | Tag | Type   | Notes                                                                 |
|-----------|-----|--------|-----------------------------------------------------------------------|
| `success` | 1   | bool   | True iff the row was updated (`SQL UPDATE rowcount > 0`).             |
| `error`   | 2   | string | Set when `success=false`. Currently `"Member not found"` for the only soft-failure path (`:2645`). Other faults surface as gRPC status codes, not as `error` strings. |

Note: there is no `RequestContext`/`idempotency_key` on this message; rebuild semantics rely on the membership row being a last-writer-wins UPDATE.

## Auth (admin; last-admin demotion protection; trusted-actor)

- **Trusted-actor rebind (mandatory).** Handler MUST replace `request.actor`
  with `_trusted_actor(request.actor)` before any privilege check
  (`server/go/internal/api/change_member_role.go`). The implementation lives at `:418-437` and
  delegates to `auth/auth_interceptor.py::get_authoritative_actor`. In Go
  this is a `ContextVar` equivalent: read the authoritative actor from
  `context.Context` (set by an auth interceptor); fall back to `request.actor`
  only when no interceptor ran (no-auth deployments / unit tests). Any
  privilege check that consults `request.actor` directly is a CVE-class bug
  (cf. commit `fece3fb`).
- **Authorization rule.** Allowed iff (a) trusted actor passes
  `_is_admin_or_system` (i.e. `system:*` or admin role in the global user
  registry, `:2053-2064`), OR (b) trusted actor's role in `tenant_id` is
  exactly `"owner"` (`:2629-2635`). `admin` role is NOT sufficient — pinned
  by `test_change_role_by_admin_denied` (see below).
- **Last-admin / last-owner demotion protection.** **NOT IMPLEMENTED in the
  Python handler.** `change_role` writes unconditionally; nothing prevents
  demoting the sole owner to `member` and leaving the tenant unadminable.
  Go port should NOT add this protection silently — it would break parity
  with the contract test `test_change_role_by_owner` (which lets `alice`,
  the only owner, change `bob`'s role but does not exercise self-demotion).
  See "Open questions" — needs an explicit decision before introducing.
- **Tenant-region pin.** Not applied here. `_check_tenant_access` is not
  called; cross-region calls are not blocked. (Possibly a gap; preserve
  parity for now.)

## Side effects (WAL append; ACL recompute)

In-order narration of the Python handler:

1. `start := time.Now()` for `record_grpc_request` metrics.
2. Guard: `global_store == nil` → `UNIMPLEMENTED` (`:2609-2613`).
3. Argument validation (4× `INVALID_ARGUMENT`, see Error contract).
4. Rebind to trusted actor; compute `actor_uid` via `_actor_user_id`
   (strip `user:` prefix, `:412-416`).
5. If not admin/system, look up caller's role via `_get_member_role`
   (`:2290-2296`, scans `global_store.get_members(tenant_id)`); enforce
   `role == "owner"`.
6. `global_store.change_role(tenant_id, user_id, new_role)` — single
   `UPDATE tenant_members SET role=? WHERE tenant_id=? AND user_id=?`
   (`server/go/internal/globalstore/`). Returns `rowcount > 0`.
7. Map result to `ChangeMemberRoleResponse{success, error}` and emit
   `record_grpc_request("ChangeMemberRole", "ok"|"error", elapsed)`.

**WAL append:** the Go handler emits global `member_role_changed` and
waits for the applier to update `tenant_members`. The handler does not
write globalstore directly.

**ACL recompute: NOT performed.** Roles in `tenant_members` do not feed
the per-tenant `acl` table; permission grants are independent. No cache
invalidation, no broadcast, no schema-registry touch.

**No emails, no audit trail row, no metric beyond `grpc_requests_total`.**

## Error contract

| Condition                                 | gRPC code             | Response body                       | Source line |
|-------------------------------------------|-----------------------|-------------------------------------|-------------|
| `global_store` not configured             | `UNIMPLEMENTED`       | abort                               | `:2609-2613` |
| `actor == ""`                             | `INVALID_ARGUMENT`    | `"actor is required"`               | `:2615-2616` |
| `tenant_id == ""`                         | `INVALID_ARGUMENT`    | `"tenant_id is required"`           | `:2617-2618` |
| `user_id == ""`                           | `INVALID_ARGUMENT`    | `"user_id is required"`             | `:2619-2620` |
| `new_role == ""`                          | `INVALID_ARGUMENT`    | `"new_role is required"`            | `:2621-2622` |
| Caller is not owner / admin / system      | `PERMISSION_DENIED`   | `"Only owner can change member roles"` | `:2632-2635` |
| Member row does not exist                 | `OK` (gRPC)           | `success=false, error="Member not found"` | `:2643-2645` |
| Any other exception                       | `OK` (gRPC)           | `success=false, error=str(e)`; metric label `"error"` | `:2649-2652` |

Note the asymmetry: missing-member is a soft failure on the response;
authz failure is a hard `PERMISSION_DENIED` abort. Go port MUST preserve
both shapes — they are pinned by tests below.

## Shared Go package deps

- `internal/auth` — `TrustedActor(ctx, requestActor) string`,
  `IsAdminOrSystem(ctx, actor) bool`, `ActorUserID(actor) string` (mirror of
  `_trusted_actor`, `_is_admin_or_system`, `_actor_user_id`).
- `internal/globalstore` — `GetMembers(ctx, tenantID)`, `ChangeRole(ctx,
  tenantID, userID, role) (bool, error)`. SQLite-backed, async via
  `errgroup`/explicit txn.
- `internal/metrics` — `RecordGRPCRequest(rpc, status string, dur
  time.Duration)`.
- `internal/grpcerr` — helpers to translate to `status.Errorf` /
  `context.Abort` semantics. Use `codes.InvalidArgument`,
  `codes.PermissionDenied`, `codes.Unimplemented`.
- Generated `entdbv1` package — `ChangeMemberRoleRequest`,
  `ChangeMemberRoleResponse`.

No WAL package needed (see Side effects).

## Other-RPC deps

- **AddTenantMember** — must run first to materialize the row that
  `ChangeMemberRole` updates (`global_store.add_member`). Tested in the
  contract suite ordering at `tests/python/integration/test_grpc_contract.py:520-545`.
- **GetUserTenants / ListTenantMembers** — read paths that surface the new
  role; rely on the same `tenant_members` table.
- **RemoveTenantMember** — sibling write path, same authz rule.
- No coupling to node/edge RPCs or the per-tenant SQLite shards.

## Contract tests pinning behavior (file:line)

Unit (handler-level, in-memory `GlobalStore`):
- (legacy Python unit test, removed in Phase 4D) —
  `TestChangeMemberRoleHandler` class.
  - `:595-614` `test_change_role_by_owner` — happy path; verifies row
    actually updated via `get_members`.
  - `:616-632` `test_change_role_by_member_denied` — `PERMISSION_DENIED`
    when caller is `member`.
  - `:634-650` `test_change_role_by_admin_denied` — `PERMISSION_DENIED`
    when caller is `admin` (NOT `owner`). This is the load-bearing
    "owner-only" pin.
  - `:652-668` `test_change_role_nonexistent_member` —
    `success=false, error~="not found"`, NOT a gRPC abort.
  - `:670-685` `test_change_role_by_system` — `system:admin` bypasses
    membership check entirely.

Integration (over real gRPC channel):
- `tests/python/integration/test_grpc_contract.py:531-538` — happy-path
  smoke (`actor=ALICE` who is owner).
- `:539-545` — empty-actor → `INVALID_ARGUMENT`.

Go port MUST add equivalent table-driven tests in `tests/go/` and the
cross-impl `tests/contract/` harness (per CLAUDE.md project structure)
before swapping the binary.

## Implementation outline

```go
func (s *Server) ChangeMemberRole(
    ctx context.Context, req *entdbv1.ChangeMemberRoleRequest,
) (*entdbv1.ChangeMemberRoleResponse, error) {
    start := time.Now()
    defer func() { /* metrics.RecordGRPCRequest on both paths */ }()

    if s.globalStore == nil {
        return nil, status.Error(codes.Unimplemented, "Tenant registry not configured")
    }
    if req.Actor == ""    { return nil, status.Error(codes.InvalidArgument, "actor is required") }
    if req.TenantId == "" { return nil, status.Error(codes.InvalidArgument, "tenant_id is required") }
    if req.UserId == ""   { return nil, status.Error(codes.InvalidArgument, "user_id is required") }
    if req.NewRole == ""  { return nil, status.Error(codes.InvalidArgument, "new_role is required") }

    trusted := auth.TrustedActor(ctx, req.Actor)            // ContextVar-equivalent
    if !auth.IsAdminOrSystem(ctx, trusted) {
        role, _ := s.getMemberRole(ctx, req.TenantId, auth.ActorUserID(trusted))
        if role != "owner" {
            return nil, status.Error(codes.PermissionDenied, "Only owner can change member roles")
        }
    }

    updated, err := s.globalStore.ChangeRole(ctx, req.TenantId, req.UserId, req.NewRole)
    if err != nil {
        return &entdbv1.ChangeMemberRoleResponse{Success: false, Error: err.Error()}, nil
    }
    if !updated {
        return &entdbv1.ChangeMemberRoleResponse{Success: false, Error: "Member not found"}, nil
    }
    return &entdbv1.ChangeMemberRoleResponse{Success: true}, nil
}
```

Metrics: emit `"ok"` for both updated and not-found (matches Python
`:2644,:2647`); emit `"error"` only on the exception branch.

## Open questions / risks

1. **WAL invariant violation (HIGH).** Per CLAUDE.md §1, all writes go
   through the WAL; this handler does not. Behavior on rebuild-from-WAL
   is silent loss of role changes. Decide before GA whether to (a) add a
   `change_member_role` op on `TransactionEvent.ops` and route through
   the applier, or (b) document `tenant_members` as a non-WAL'd
   bootstrap table and snapshot it separately. Either way: file an
   issue, do not silently fix in the port.
2. **No last-owner protection.** A tenant's sole owner can demote
   themselves to `member` and brick admin access. Not in tests, so
   adding the check would change observable behavior — needs a product
   call and a test update.
3. **`new_role` is unvalidated free-form.** Caller can set
   `new_role="god-emperor"` and `_get_member_role` will return that
   string on the next call. Should we reject anything outside
   `{owner, admin, member}`? Not enforced today.
4. **Trusted-actor in unit tests.** The unit tests pass `actor="user:alice"`
   directly with no interceptor in the chain; the fallback path in
   `_trusted_actor` honours the request value. Go port must keep the
   same fallback or all five unit tests break.
5. **Region pin gap.** `_check_tenant_access` is not invoked. A node
   serving region `us-east` will happily mutate a row for a tenant
   pinned to `eu-west`. Likely a real bug; preserve parity, file a
   follow-up.
6. **Concurrent role changes.** Two simultaneous `ChangeMemberRole`
   calls race on `UPDATE`; last writer wins, no version check. Fine
   today, but document it.
