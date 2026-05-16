# RevokeAccess — Go Port Spec

> Implementation: `server/go/internal/api/revoke_access.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407. Counterpart to `ShareNode`. Removes a single direct grant
on a node for one actor (user/group/tenant principal). Reference Python
handler: `server/go/internal/api/revoke_access.go`.
Proto: `proto/entdb/v1/entdb.proto:97, 746-755`.

## Wire contract

Request `entdb.v1.RevokeAccessRequest` (proto:746):
- `RequestContext context` — tenant_id + actor (UNTRUSTED; see Auth).
- `string node_id` — node whose grant is being removed.
- `string actor_id` — fully-qualified principal whose grant to drop.
  Convention: `"user:<id>"`, `"group:<id>"`, or `"tenant:<id>"`. The
  handler does not parse/validate the prefix; it is matched verbatim
  against `node_access.actor_id` (server/go/internal/store/).

Response `RevokeAccessResponse` (proto:752):
- `bool found` — `true` iff a row was deleted from `node_access`. False
  for both "no such grant existed" AND on permission/error paths (see
  Error contract).
- `string error` — populated on PermissionError and on any caught
  exception. Empty on the happy path regardless of `found`.

Idempotency: revoking a non-existent grant returns `found=false,
error=""` and is **not** an error. See pinned test
`test_revoke_nonexistent_returns_false`
((legacy Python unit test, removed in Phase 4D)).

## Auth (caller must own/share-grant the node; trusted-actor)

- Tenant existence: `_check_tenant(request.context.tenant_id, ctx)`
  (server/go/internal/api/revoke_access.go, 362).
- Trusted actor: `_trusted_actor(request.context.actor)` — the payload
  actor is OVERWRITTEN by `AuthInterceptor.get_authoritative_actor`
  (server/go/internal/api/revoke_access.go, 418). The Go port MUST read identity from the
  gRPC metadata via the AuthInterceptor equivalent and ignore the
  request-payload actor for all auth decisions. Privilege-escalation
  guard pinned by `tests/python/integration/test_privilege_escalation.py`
  (RevokeAccess included in the matrix).
- Capability check: `_check_capability(tenant_id, trusted_actor,
  node_id, type_id=0, "RevokeAccess", ctx)` (server/go/internal/api/revoke_access.go).
  Required capability is `CoreCapability.ADMIN`
  (`server/go/internal/auth/capability_registry.go`,
  pinned by (legacy Python unit test, removed in Phase 4D)).
- ADMIN is satisfied by node ownership OR by an explicit ADMIN grant
  on the node (the share path that ShareNode itself can install with
  `core_caps=[ADMIN]`). Tenant-admin role also passes via the
  capability registry — match Python behavior, do not tighten.
- On `PermissionError`: emit `record_grpc_request("RevokeAccess",
  "denied", ...)` and return `RevokeAccessResponse{found:false,
  error:str(perm_err)}` — NOT a gRPC status error
  (server/go/internal/api/revoke_access.go). Preserve this for client compatibility.

## Side effects (WAL append: revocation; mailbox cleanup)

**INVARIANT VIOLATION IN PYTHON (carry-forward risk).** The current
Python handler writes directly to per-tenant SQLite via
`canonical_store.revoke_access` → `_sync_revoke_access` (DELETE on
`node_access`, server/go/internal/store/). It does **not** append
a `TransactionEvent` to the WAL. This means revocations are silently
**lost on rebuild from the WAL** — a documented violation of the
"all writes go through the WAL" invariant in `CLAUDE.md`.

Go-port decision (recommended for EPIC #407): **fix this on port.**
Append a WAL event with op type `revoke_access` (mirroring
`admin_revoke_access` at server/go/internal/apply/applier.go, but scoped to a single
node + actor rather than a user-wide sweep). Schema:

```
TransactionEvent.ops = [{
    "type": "revoke_access",
    "node_id": <string>,
    "actor_id": <string>,
}]
```

Apply path (Go applier): `DELETE FROM node_access WHERE node_id=? AND
actor_id=?`; `rowcount > 0` becomes `found`. The handler must wait
for the receipt to be APPLIED before returning so `found` reflects
post-apply state — same fence pattern other writes already use.

Cross-tenant `shared_index` cleanup (currently inline in handler at
server/go/internal/api/revoke_access.go) MUST move into the applier’s post-apply
hook, mirroring `_update_shared_index_on_revoke`
(server/go/internal/apply/applier.go). Behavior to preserve:
- If `actor_id` starts with `"group:"`: enumerate group members via
  `get_group_members(tenant_id, actor_id)` and remove each member’s
  `shared_index` row for `(tenant_id, node_id)`.
- Otherwise: a single `global_store.remove_shared(actor_id,
  tenant_id, node_id)` call (server/go/internal/globalstore/).
- Errors from shared_index cleanup are **logged and swallowed**
  (warning level, not fatal). Handler still returns `found=true`.
  This is intentional: `shared_index` is a hint cache, not
  authoritative (server/go/internal/globalstore/).

ACL inheritance: revoking a parent automatically blocks children
through `acl_inherit` chains — no extra work needed; the cascade is
implicit in the access-check path. Pinned by
`test_revoke_parent_blocks_children`
((legacy Python unit test, removed in Phase 4D)).

Mailbox cleanup: there is no per-node mailbox row tied to ACL grants,
so no direct mailbox DELETE is required. The brief mentions "mailbox
cleanup side effect" — confirmed scope is `shared_index` (the
"shared with me" mailbox view), not `mailbox_items`. Matches
ListSharedWithMe semantics.

## Error contract

All errors are returned as `RevokeAccessResponse{found:false,
error:<string>}` — never as a gRPC status error (server/go/internal/api/revoke_access.go-
1875). Three observable paths:
1. `PermissionError` → `error=str(perm_err)`, metric label `denied`.
2. Any other exception → `error=str(e)`, metric label `error`,
   `logger.error(..., exc_info=True)`.
3. Happy path → `error=""`, metric label `ok`. `found` is `true` iff
   at least one row was deleted.

`_check_tenant` may raise; it propagates into the generic `except
Exception` and surfaces as `error=...`. Match this; do not convert to
`NOT_FOUND`.

## Shared Go package deps

- `internal/auth` — `AuthInterceptor.AuthoritativeActor(ctx)`,
  `CheckTenant`, `CheckCapability(tenant, actor, nodeID, typeID,
  rpcName)`, `CapabilityRegistry` seeded with
  `RevokeAccess → CoreCapability.ADMIN`.
- `internal/wal` — `Wal.Append(event)` + receipt wait (re-uses the
  ExecuteAtomic write fence).
- `internal/apply` — applier op handler `revoke_access` writing to
  per-tenant SQLite; post-apply hook calling shared-index cleanup.
- `internal/canonical` — typed access to `node_access`,
  `get_group_members`, `acl_inherit` (read-only here; revoke is just
  a DELETE on `node_access`).
- `internal/global` — `GlobalStore.RemoveShared(userID, tenant,
  nodeID)` + `CleanupStaleShared`.
- `internal/metrics` — `RecordGrpcRequest("RevokeAccess", label,
  duration)` with labels `ok|denied|error`.
- `internal/proto` (generated) — `entdb.v1.RevokeAccessRequest`,
  `RevokeAccessResponse`.

## Other-RPC deps (ShareNode counterpart)

- **ShareNode** (`server/go/internal/api/revoke_access.go`, proto:96) is the inverse.
  Both write to the same `node_access` row keyed by
  `(node_id, actor_id)`. ShareNode upserts; RevokeAccess deletes by
  exact `actor_id`. They MUST round-trip cleanly: any
  `(node_id, actor_id)` ShareNode can install, RevokeAccess must
  remove. `permission` / `core_caps` / `ext_cap_ids` / `expires_at`
  on the share row are not needed by RevokeAccess (whole row is
  dropped) — but the Go port should reuse the same canonical
  `actor_id` normalization (whatever ShareNode writes verbatim into
  `node_access.actor_id` must match here byte-for-byte).
- **AddGroupMember / RemoveGroupMember** affect membership snapshots
  used during shared_index fan-out. Eventual-consistency window is
  acceptable; matches Python.
- **RevokeAllUserAccess** (proto:1000-1006) is the user-wide sweep
  (admin op, WAL-backed via `admin_revoke_access`). Distinct surface;
  do NOT collapse into RevokeAccess.
- **TransferOwnership** can change who is the owner-by-default ADMIN;
  RevokeAccess auth re-checks per call so no special coordination.

## Contract tests pinning behavior (file:line)

- `tests/python/integration/test_grpc_contract.py:351-361` — happy
  path: idempotent on seed node (`found` may be true or false).
- (legacy Python unit test, removed in Phase 4D)
  (`test_revoke_removes_access`) — share then revoke removes access.
- (legacy Python unit test, removed in Phase 4D)
  (`test_revoke_nonexistent_returns_false`) — revoke without prior
  grant returns false, no error.
- (legacy Python unit test, removed in Phase 4D)
  (`test_revoke_parent_blocks_children`) — ACL inheritance cascade.
- (legacy Python unit test, removed in Phase 4D) — RevokeAccess
  requires `CoreCapability.ADMIN`.
- (legacy Python unit test, removed in Phase 4D) — SDK
  `scope.revoke()` routes through gRPC with the expected kwargs.
- Go SDK tests already pin transport behavior:
  `sdk/go/entdb/transport_extras_test.go:239-275` (RemovesGrant +
  IsIdempotent) and `:358-362` (Scope routing).
- Privilege-escalation matrix in
  `tests/python/integration/test_privilege_escalation.py` covers
  RevokeAccess (trusted-actor enforcement).

## Implementation outline

```go
func (s *Server) RevokeAccess(ctx context.Context,
    req *pb.RevokeAccessRequest) (*pb.RevokeAccessResponse, error) {
    start := time.Now()
    defer func() { /* metrics elsewhere */ }()

    if err := s.auth.CheckTenant(ctx, req.Context.TenantId); err != nil {
        return errResp(err, "error", start)
    }
    actor := s.auth.AuthoritativeActor(ctx) // ignore req.Context.Actor
    if err := s.auth.CheckCapability(ctx, req.Context.TenantId, actor,
        req.NodeId, 0, "RevokeAccess"); err != nil {
        if errors.Is(err, auth.ErrPermission) {
            metrics.Record("RevokeAccess", "denied", start)
            return &pb.RevokeAccessResponse{Found: false,
                Error: err.Error()}, nil
        }
        return errResp(err, "error", start)
    }

    evt := wal.NewEvent(req.Context.TenantId, []wal.Op{{
        Type: "revoke_access",
        NodeID: req.NodeId, ActorID: req.ActorId,
    }})
    receipt, err := s.wal.AppendAndWait(ctx, evt)
    if err != nil { return errResp(err, "error", start) }

    found := receipt.RowsAffected > 0
    metrics.Record("RevokeAccess", "ok", start)
    return &pb.RevokeAccessResponse{Found: found}, nil
}
```

Applier op handler (sketch):
```go
case "revoke_access":
    res, _ := tx.Exec(`DELETE FROM node_access
        WHERE node_id=? AND actor_id=?`, op.NodeID, op.ActorID)
    n, _ := res.RowsAffected()
    receipt.RowsAffected = n
    if n > 0 { go s.cleanupSharedIndexOnRevoke(tenant, op) }
```

## Open questions / risks (idempotency: revoke-not-granted)

1. **WAL parity vs. Python bug-for-bug.** Python skips the WAL today.
   Decision needed: port the bug (fast, identical behavior) or fix on
   port (correct, slight contract drift — `found` will require
   waiting for apply). Recommendation: **fix.** Tests pin only
   eventual `found` semantics, not the path.
2. **Idempotency on revoke-not-granted.** Python returns
   `found=false, error=""`. After WAL-ification, replays of the same
   `idempotency_key` must collapse to one DELETE; second call returns
   `found=false`. Confirm the Go applier treats a 0-row DELETE as a
   normal apply (not an error) so `error` stays empty.
3. **Group membership timing.** `shared_index` cleanup enumerates
   group members at apply time. If a member is added between
   ShareNode and RevokeAccess for a group grant, their shared_index
   row may have been seeded by ShareNode but not removed here.
   Acceptable per current design (`shared_index` is a hint), but
   document it.
4. **Error string stability.** Tests don’t pin the exact error
   string, but SDK-side error wrapping does string-contains in some
   places. Match Python `str(perm_err)` style ("permission denied: …").
5. **Concurrent ShareNode + RevokeAccess on same `(node, actor)`.**
   WAL serialization removes the race. Last-writer-wins by
   stream_pos; verify both ops share the same partition key
   (likely `tenant_id`) so ordering is deterministic.
6. **`actor_id` normalization.** Python does no normalization. If
   ShareNode wrote `"User:Bob"` and RevokeAccess sends `"user:bob"`,
   no rows match. Preserve this strictness on port; do NOT
   case-fold.
