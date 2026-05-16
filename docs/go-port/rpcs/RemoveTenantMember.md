# RPC Port Spec — `entdb.v1.EntDBService/RemoveTenantMember`

> Implementation: `server/go/internal/api/remove_tenant_member.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407 — Python → Go server port. Source of truth: Python handler at
`server/go/internal/api/remove_tenant_member.go`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:124` (rpc), `:909-919` (messages).

Request — `TenantMemberRequest` (shared with `AddTenantMember`):
| Field | Tag | Type | Required | Notes |
|------|-----|------|----------|------|
| `actor` | 1 | `string` | yes | Caller, UNTRUSTED. Format `"user:<id>"` or `"system:<name>"`. Rebound via `_trusted_actor` before any decision. |
| `tenant_id` | 2 | `string` | yes | Tenant whose member is being removed. |
| `user_id` | 3 | `string` | yes | User to drop. NOT a `tenant_principal` ("user:alice") — bare id ("alice"). |
| `role` | 4 | `string` | no | IGNORED by this RPC (request type is reused from `AddTenantMember`). Go port MUST NOT read it. |

Response — `TenantMemberResponse`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `success` | 1 | `bool` | True iff a row was deleted. False on "not found" / "last owner". |
| `error` | 2 | `string` | Human-readable, lowercase-substring-stable: `"Member not found"`, `"Cannot remove the last owner of a tenant"`. Tests match `"not found"` / `"last owner"` (`test_tenant_registry.py:495,526`). DO NOT rephrase. |

## Auth (admin only; trusted-actor)

KNOWN GAP IN PYTHON: the Python handler does NOT enforce admin/owner role.
`AddTenantMember` (`server/go/internal/api/remove_tenant_member.go`) and `ChangeMemberRole`
(`server/go/internal/api/remove_tenant_member.go`) both call `_trusted_actor` + `_is_admin_or_system`
+ `_get_member_role` and abort `PERMISSION_DENIED` for non-admins.
`RemoveTenantMember` skips this check entirely. The contract test at
`tests/python/integration/test_grpc_contract.py:547-551` only asserts the
happy path with `actor=ALICE` (a regular user). This is a latent privilege
escalation (any authenticated caller can remove any member, including the
last non-owner admin) and tracks against commit `fece3fb` ("ignore
client-claimed actor in gRPC handlers").

**Go port MUST close the gap**, mirroring `AddTenantMember`:

```
trusted := s.trustedActor(req.GetActor())          // server/go/internal/api/remove_tenant_member.go
if !s.isAdminOrSystem(trusted) {                   // server/go/internal/api/remove_tenant_member.go
    role, _ := s.getMemberRole(req.TenantId,
        s.actorUserID(trusted))                    // server/go/internal/api/remove_tenant_member.go
    if role != "owner" && role != "admin" {
        return abort(codes.PermissionDenied,
            "Only owner or admin can remove members")
    }
}
```

`actor` from the wire is UNTRUSTED — Go handler MUST rebind to the trusted
actor returned by the auth interceptor (`auth.GetAuthoritativeActor`) before
ANY downstream decision. Pin this with a new contract test
`TestRemoveTenantMember_NonAdminDenied` so the Go port does not regress on
parity with `AddTenantMember`.

This RPC is NOT in `AuthInterceptor.UNAUTHENTICATED_METHODS` — auth runs.
Rate limiter applies normally (per-tenant bucket).

## Side effects (WAL append; ACL cascade revocation; mailbox?)

In-order narration of the Python handler (`server/go/internal/api/remove_tenant_member.go`):

1. `start = perf_counter()` for metrics.
2. Guard: `self.global_store == nil` → abort `UNIMPLEMENTED "Tenant registry not configured"` (`:2500-2504`).
3. Validate `actor`, `tenant_id`, `user_id` non-empty → abort `INVALID_ARGUMENT` (`:2506-2511`).
4. Read all members via `global_store.get_members(tenant_id)` (`:2514`, `server/go/internal/globalstore/`).
5. Single pass: count owners; locate target by `user_id` (`:2515-2521`).
6. If target missing → return `success=false, error="Member not found"`, metric `ok` (`:2523-2525`). NOT an error code — handler treats it as a normal "no-op".
7. If target is owner AND `owner_count <= 1` → return `success=false, error="Cannot remove the last owner of a tenant"`, metric `error` (`:2527-2532`).
8. `removed = await global_store.remove_member(tenant_id, user_id)` → `DELETE FROM tenant_members` returning `rowcount > 0` (`server/go/internal/globalstore/`).
9. Return `success=removed`, metric `ok` (`:2536-2537`).
10. Outer `except`: log + return `success=false, error=str(e)`, metric `error` (`:2538-2541`).

**WAL append:** the Go handler emits global `member_removed` and waits
for the applier to delete the `tenant_members` row. The handler does not
write globalstore directly.

**ACL cascade: Python does NOT cascade.** The Go SDK doc string says so
explicitly (`sdk/go/entdb/admin.go:68-71`): "drops a membership row. It does
NOT reassign nodes the user owns or revoke their direct ACL grants — for
that see `TransferUserContent` and `RevokeAllUserAccess`. Combine all three
for a complete off-board." Go port MUST preserve this — direct grants,
group memberships, `shared_index` rows, and owned nodes survive. The caller
is responsible for chaining the three RPCs.

**Mailbox: no.** This RPC does not touch the mailbox. No `publish_event`,
no notify, no fan-out.

## Error contract (last-admin protection)

| gRPC code | Trigger |
|-----------|---------|
| `OK` (`success=true`) | Member existed and was deleted. |
| `OK` (`success=false, error="Member not found")` | `user_id` not a member of `tenant_id`. NOT an `RpcError`. |
| `OK` (`success=false, error="Cannot remove the last owner …")` | Target is the only owner. **Last-owner protection.** "admin" is NOT protected — only "owner". |
| `INVALID_ARGUMENT` | Empty `actor`, `tenant_id`, or `user_id`. |
| `PERMISSION_DENIED` | (Go port only) caller is not owner/admin/system. |
| `UNIMPLEMENTED` | `global_store` not configured (registry-less deployment). |
| `INTERNAL` | Unexpected exception. Python returns `success=false, error=str(e)` with `OK`; Go port should match for parity but log at `ERROR`. |

Last-admin nuance: only "owner" is protected. A tenant with one "admin" and
no owners has no last-owner guard — the admin can be removed and the tenant
is left with zero privileged members. Python pins this by omission; do NOT
add an "admin" guard in Go without an ADR.

## Shared Go package deps

- `pb` (`server/go/internal/pb/entdbv1`) — `TenantMemberRequest`, `TenantMemberResponse`. Required.
- `globalstore` — `GetMembers(ctx, tenantID) ([]Member, error)`, `RemoveMember(ctx, tenantID, userID) (bool, error)`. Mirrors `server/go/internal/globalstore/`. Required.
- `auth` — `GetAuthoritativeActor(ctx, requestActor) string`, `IsAdminOrSystem(actor) bool`, `ActorUserID(actor) string`. Mirrors `server/go/internal/api/remove_tenant_member.go,2053`. Required.
- `metrics` — `RecordGRPCRequest("RemoveTenantMember", status, dur)`. Required.
- `errs` — wraps `status.Error(codes.X, msg)` with the exact strings above. Required.

NOT used and MUST NOT be imported: `canonicalstore`, `acl`, `schema`,
`quota`, `crypto`, `audit`, `mailbox`. Importing any signals scope creep.

## Other-RPC deps (overlaps with RevokeAllUserAccess)

This RPC is **paired** with two siblings — Go port should land them in the
same PR if practical because callers compose them:

- `AddTenantMember` (`server/go/internal/api/remove_tenant_member.go`) — inverse op; shares request type and auth shape.
- `ChangeMemberRole` (`server/go/internal/api/remove_tenant_member.go`) — alternative to remove+add; shares the owner/admin enforcement pattern.
- `GetTenantMembers` (`server/go/internal/api/remove_tenant_member.go`) — Go port can reuse the same `globalstore.GetMembers` call.
- `RevokeAllUserAccess` (`server/go/internal/api/remove_tenant_member.go`) — **complementary**, NOT a substitute. Strips ACL grants, group memberships, and `shared_index` entries; does NOT delete the membership row. Off-boarding flow = `TransferUserContent` → `RevokeAllUserAccess` → `RemoveTenantMember`. Document this ordering in the Go SDK package doc.
- `TransferUserContent` (`server/go/internal/api/remove_tenant_member.go`) — reassigns nodes owned by the leaving user.

The four-RPC off-board ordering is encoded only in the Go SDK doc comment
today (`sdk/go/entdb/admin.go:68-71`). Mention it in the Go server's package
doc as well.

## Contract tests pinning behavior (file:line)

- `tests/python/integration/test_grpc_contract.py:546-551` — happy path: `RemoveTenantMember{actor=ALICE, tenant_id=TENANT, user_id="bob"}` over a real gRPC channel. The Go server must pass this verbatim.
- `tests/python/integration/test_grpc_contract.py:552-556` — empty `actor` → `INVALID_ARGUMENT`.
- (legacy Python unit test, removed in Phase 4D) — happy path; one owner + one member; remove member; member list shrinks to owner only.
- (legacy Python unit test, removed in Phase 4D) — last-owner protection; `success=false`, `"last owner" in error`.
- (legacy Python unit test, removed in Phase 4D) — owner removable when ≥2 owners exist.
- (legacy Python unit test, removed in Phase 4D) — non-existent member → `success=false`, `"not found" in error.lower()`.
- `sdk/go/entdb/admin_test.go:203-216` — Go SDK transport happy path; the fake server returns `Success=true` and the SDK swallows it. Pins the wire shape.
- `sdk/go/entdb/grpc_transport_test.go:349` — fake-server hook used by SDK tests; ensures the Go SDK transport still calls the right RPC.

New tests Go port should add (close the auth gap):
- `TestRemoveTenantMember_NonAdminDenied` — `actor=user:eve` who is a `member` of `t1` tries to remove `user:bob` → expect `PERMISSION_DENIED`.
- `TestRemoveTenantMember_OwnerCanRemoveAdmin` — owner removes an admin → `success=true`.
- `TestRemoveTenantMember_AdminCanRemoveMember` — admin removes a member → `success=true`.
- `TestRemoveTenantMember_TrustedActorWins` — interceptor sets `user:eve` (member); request claims `actor="system:admin"` → `PERMISSION_DENIED` (forged actor must lose).

## Implementation outline

```go
// server/go/internal/api/tenant_member.go
func (s *EntDBServer) RemoveTenantMember(
    ctx context.Context, req *pb.TenantMemberRequest,
) (*pb.TenantMemberResponse, error) {
    start := time.Now()
    status := "ok"
    defer func() { metrics.RecordGRPCRequest("RemoveTenantMember", status, time.Since(start)) }()

    if s.globalStore == nil {
        status = "error"
        return nil, errs.Abort(codes.Unimplemented, "Tenant registry not configured")
    }
    if req.GetActor() == "" {
        status = "error"; return nil, errs.Abort(codes.InvalidArgument, "actor is required")
    }
    if req.GetTenantId() == "" {
        status = "error"; return nil, errs.Abort(codes.InvalidArgument, "tenant_id is required")
    }
    if req.GetUserId() == "" {
        status = "error"; return nil, errs.Abort(codes.InvalidArgument, "user_id is required")
    }

    trusted := s.auth.TrustedActor(ctx, req.GetActor())
    if !s.auth.IsAdminOrSystem(trusted) {
        role, _ := s.globalStore.GetMemberRole(ctx, req.GetTenantId(), s.auth.ActorUserID(trusted))
        if role != "owner" && role != "admin" {
            status = "error"
            return nil, errs.Abort(codes.PermissionDenied, "Only owner or admin can remove members")
        }
    }

    members, err := s.globalStore.GetMembers(ctx, req.GetTenantId())
    if err != nil { status = "error"; return errResp(err) }
    var target *globalstore.Member; ownerCount := 0
    for i := range members {
        if members[i].Role == "owner" { ownerCount++ }
        if members[i].UserID == req.GetUserId() { target = &members[i] }
    }
    if target == nil {
        return &pb.TenantMemberResponse{Success: false, Error: "Member not found"}, nil
    }
    if target.Role == "owner" && ownerCount <= 1 {
        status = "error"
        return &pb.TenantMemberResponse{Success: false, Error: "Cannot remove the last owner of a tenant"}, nil
    }
    removed, err := s.globalStore.RemoveMember(ctx, req.GetTenantId(), req.GetUserId())
    if err != nil { status = "error"; return errResp(err) }
    return &pb.TenantMemberResponse{Success: removed}, nil
}
```

Notes: keep the get-then-delete in two calls (matches Python; no transaction
needed — last-owner is advisory, race window is acceptable per current
contract). Do NOT wrap in a SQL transaction across `GetMembers` and
`RemoveMember`; SQLite isolation is per-connection and the cost of a busy-
wait on `tenant_members` is not worth it.

## Open questions / risks

- **WAL invariant violation.** Python skips the WAL for membership writes
  (CLAUDE.md Architecture Invariant #1). Decide before merge: port-as-is and
  file an invariant-debt ticket, OR add a `TenantMemberRemoved` op now. The
  former is recommended — diverging from Python during the port multiplies
  contract-test risk.
- **Auth gap fix is a behavior change.** Adding `PERMISSION_DENIED` for
  non-admins WILL break any caller currently relying on the bug. Audit the
  console + Python SDK for unaudited callers; the contract test at
  `test_grpc_contract.py:548` uses `actor=ALICE` (a tenant admin per the
  fixture), so it should still pass.
- **Last-admin not protected.** Only "owner" is guarded. Tenants with admin-
  only privilege can be left with zero privileged members. Confirm with EPIC
  #407 owner whether to extend the guard or ship parity.
- **Race: TOCTOU on owner_count.** Two concurrent `RemoveTenantMember` calls
  for the two only owners can each pass the `owner_count > 1` check, then
  both delete. Result: zero owners. Python has this race; Go port will too
  unless we add a SQLite `BEGIN IMMEDIATE` around the get+delete. Accept
  parity for now; track follow-up.
- **`role` field on the request is silently ignored.** Risk of caller
  confusion (they may set it expecting the row to be filtered). Document in
  the Go SDK that this field is `AddTenantMember`-only; consider proto
  comment update in a separate PR.
- **Metrics status mapping.** Python records `"ok"` for "Member not found"
  but `"error"` for "last owner". Preserve exactly — dashboards depend on
  this label distribution.
- **Cross-region pinning.** `_check_tenant` (region pin) is NOT called by
  this handler. Confirm intentional: tenant registry is global, not region-
  scoped. If multi-region tenants land later, revisit.
