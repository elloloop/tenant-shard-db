# RemoveGroupMember — Go port spec

> Implementation: `server/go/internal/api/remove_group_member.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407. Mirror of Python `EntDBServicer.RemoveGroupMember`
(`server/go/internal/api/remove_group_member.go`). Inverse
of `AddGroupMember` (`server/go/internal/api/remove_group_member.go`): removes an actor
from an ACL group on per-tenant SQLite, then cascades into the
cross-tenant `shared_index` so `ListSharedWithMe(member)` stops
surfacing nodes whose only path was the group.

## Wire contract

- Service: `entdb.v1.EntDBService`. Full name
  `/entdb.v1.EntDBService/RemoveGroupMember`
  (`sdk/go/entdb/internal/pb/entdb_grpc.pb.go:77`).
- Proto: `proto/entdb/v1/entdb.proto:106` —
  `rpc RemoveGroupMember(GroupMemberRequest) returns (GroupMemberResponse);`
- `GroupMemberRequest` (`proto/entdb/v1/entdb.proto:768-773`):
  - `RequestContext context = 1` — `tenant_id` (required),
    `actor` (advisory; see Auth), `trace_id`.
  - `string group_id = 2` — free-form (e.g. `"group:eng"`).
  - `string member_actor_id = 3` — actor being removed.
  - `string role = 4` — proto-only; **ignored on Remove** (Add
    reads it, `server/go/internal/api/remove_group_member.go`). Accept + discard.
- `GroupMemberResponse` (`proto/entdb/v1/entdb.proto:775-778`):
  - `bool success` — mirrors `found` from
    `canonical_store.remove_group_member`: `true` iff a row was
    deleted (`server/go/internal/api/remove_group_member.go`,
    `apply/server/go/internal/store/`). **Add/Remove asymmetry**:
    Add returns `success=true` on no exception; Remove returns
    `success=false` with empty `error` when the pair did not exist.
  - `string error` — populated only on exception
    (`server/go/internal/api/remove_group_member.go`).

## Auth

- **Tenant / region.** First call is
  `await self._check_tenant(request.context.tenant_id, context)`
  (`server/go/internal/api/remove_group_member.go`, helper `:362-410`). Standard redirect
  semantics: `UNAVAILABLE` + `entdb-redirect-node` trailer on shard
  miss, `FAILED_PRECONDITION` on region mismatch. Reproduce trailer
  key exactly — Go SDK redirect cache reads it.
- **Capability.** `auth/capability_registry.py` does NOT map
  `RemoveGroupMember` to a `CoreCapability`; handler accepts any
  caller passing `_check_tenant`. Known parity gap (Open question 6).
- **Trusted actor.** `request.context.actor` is advisory only.
  Python’s `_trusted_actor()` (`server/go/internal/api/remove_group_member.go`) is NOT
  invoked here. Go port: rebind to trusted actor (auth interceptor
  metadata) for observability; never use `context.actor` for any
  authz (privilege-escalation invariant, commit `fece3fb`).

## Side effects (WAL append; ACL recompute; cascade)

- **WAL append: NONE today.** Violates CLAUDE.md invariant 1.
  Handler writes directly to per-tenant SQLite via
  `canonical_store.remove_group_member`
  (`apply/server/go/internal/store/`; `_sync` does
  `DELETE FROM group_users WHERE group_id=? AND member_actor_id=?`,
  `:3539-3544`). On rebuild the removal is silently lost. Same
  defect on Add. Go port matches for v1 byte-parity; file blocker
  (Open question 1).
- **SQLite write.** One row deleted from `group_users`. Returns
  `found: bool` (rowcount > 0).
- **ACL recompute.** No explicit recompute. `_sync_can_access` /
  `_sync_resolve_actor_groups` read `group_users` live; removal
  takes effect at the next read
  ((legacy Python unit test, removed in Phase 4D)).
- **Cascade — does removed member’s grants get revoked?**
  1. **Direct grants** (`acl_grants` rows where
     `grantee == member_actor_id`) — NOT touched. Use
     `RevokeAccess` / `RevokeAllUserAccess`.
  2. **Group-derived access** (`acl_grants` rows where
     `grantee == group_id`) — NOT removed from the per-tenant ACL;
     the group still has the grant. Instead the handler cleans up
     the cross-tenant `shared_index` projection so
     `ListSharedWithMe(member)` no longer surfaces those nodes:
     - Read group access via
       `canonical_store.list_node_access_for_group(tenant_id,
       group_id)` BEFORE the delete (`server/go/internal/api/remove_group_member.go`) —
       ordering is load-bearing; reading after observes the already
       removed membership edge.
     - On `found=True`, iterate and call
       `global_store.remove_shared(member_actor_id, tenant_id,
       node_id)` (`server/go/internal/api/remove_group_member.go`).
     - Best-effort: cascade failures logged at WARNING and
       swallowed; RPC returns `success=found`
       (`server/go/internal/api/remove_group_member.go`). Skipped entirely when
       `found=False` or `global_store is None` or pre-read failed.
  3. Effect: if the removed member ALSO has a direct grant on the
     same node, the cascade wrongly deletes their `shared_index`
     row (Open question 4).

## Error contract

Python swallows everything below `_check_tenant`: logs and returns
`GroupMemberResponse(success=False, error=str(e))`
(`server/go/internal/api/remove_group_member.go`). `_check_tenant` aborts propagate as
gRPC status. Go port:

- Tenant not on this node → `codes.Unavailable` + trailer
  `entdb-redirect-node: <owner>` when known.
- Wrong region → `codes.FailedPrecondition`, standard region-pin
  message.
- Member not in group → NOT an error;
  `GroupMemberResponse{Success: false, Error: ""}`
  ((legacy Python unit test, removed in Phase 4D),
  `apply/server/go/internal/store/`).
- Empty `group_id` / `member_actor_id` → not validated; deletes
  nothing, returns `success=false`. Do NOT add `InvalidArgument`.
- Cascade failure → log + swallow + return `success=found`.
- Internal failure → `success=false, error=<msg>` with `codes.OK`
  (parity wins until SDK tests are upgraded).

## Shared Go package deps

- `internal/pb` — generated `entdb.v1` types
  (`sdk/go/entdb/internal/pb/entdb_grpc.pb.go`); server stubs in
  `server/go/internal/pb/...`.
- `shared/canonicalstore` (see
  `docs/go-port/shared/canonicalstore.md`):
  - `RemoveGroupMember(ctx, tenantID, groupID, memberActorID
    string) (found bool, err error)` — mirror
    `_sync_remove_group_member` (`server/go/internal/store/`).
  - `ListNodeAccessForGroup(ctx, tenantID, groupID string)
    ([]GroupAccessEntry, error)` — `{node_id, permission}` rows;
    REQUIRED BEFORE the delete (ordering invariant).
- `shared/globalstore.RemoveShared(ctx, userID, sourceTenant,
  nodeID string) error` — mirror `global_store.remove_shared`.
  Idempotent.
- `shared/auth.CheckTenant`, `TrustedActorFromCtx`.
- `shared/sharding.IsMine`, `Owner` (redirect trailer).
- `shared/observability.RecordGRPCRequest("RemoveGroupMember",
  "ok"|"error", elapsed)` (`server/go/internal/api/remove_group_member.go,2026`).

## Other-RPC deps

- `AddGroupMember` (`server/go/internal/api/remove_group_member.go`) — symmetric
  inverse; share helpers. Different cascade direction
  (`global_store.add_shared` vs `remove_shared`). Land in same PR.
- `RevokeAccess` / `RevokeAllUserAccess` — remove direct grants;
  RemoveGroupMember does not. Keep boundary in contract tests.
- `ListSharedWithMe` (`server/go/internal/api/remove_group_member.go`) — consumer of
  the `shared_index` projection cleaned up here; use in the
  cascade contract test.
- `_check_tenant` helper (`server/go/internal/api/remove_group_member.go`) — reuse the
  Go port from `GetEdgesFrom.md`.

## Contract tests pinning behavior (file:line)

- `tests/python/integration/test_grpc_contract.py:374-384` — gRPC
  happy path after Add. Contract intentionally NOT pinning
  `success=true` (`check: lambda _r: True`) due to shared tenant
  fixture; Go port may tighten.
- (legacy Python unit test, removed in Phase 4D) — remove existing
  member: `_sync_resolve_actor_groups` no longer contains group.
- (legacy Python unit test, removed in Phase 4D) — remove
  non-existent returns `False` → wire `success=false`.
- (legacy Python unit test, removed in Phase 4D) —
  `test_group_removal_cascades_access`: removing user from a
  group immediately removes their effective ACL access (read-path
  recompute, not write-time).
- (legacy Python unit test, removed in Phase 4D) —
  `test_remove_group_member_cascades_shared_index`: pins cascade
  ordering and per-member isolation (Bob empties, Carol keeps).
- `sdk/go/entdb/transport_extras_test.go:319-345` — pins request
  shape (`group_id`, `member_actor_id`).
- `sdk/go/entdb/grpc_transport_test.go:311` —
  `fakeServer.RemoveGroupMember` signature.
- `sdk/go/entdb/scope.go:100-103`, `client.go:674-680` — SDK arg
  order `(tenantID, actor, groupID, memberActor)`.

## Implementation outline

```go
func (s *Server) RemoveGroupMember(
    ctx context.Context, req *pb.GroupMemberRequest,
) (*pb.GroupMemberResponse, error) {
    start := time.Now(); outcome := "ok"
    defer func() { obs.RecordGRPCRequest("RemoveGroupMember", outcome, time.Since(start)) }()

    tenantID := req.GetContext().GetTenantId()
    if err := s.checkTenant(ctx, tenantID); err != nil {
        outcome = "error"; return nil, err // Unavailable / FailedPrecondition
    }
    groupID, memberID := req.GetGroupId(), req.GetMemberActorId()

    // 1. Snapshot group access BEFORE delete (cascade source-of-truth).
    var groupAccess []canonicalstore.GroupAccessEntry
    if s.globalStore != nil {
        if e, err := s.cstore.ListNodeAccessForGroup(ctx, tenantID, groupID); err != nil {
            log.Warn(ctx, "shared_index pre-read failed", "err", err)
        } else { groupAccess = e }
    }

    found, err := s.cstore.RemoveGroupMember(ctx, tenantID, groupID, memberID)
    if err != nil {
        outcome = "error"
        log.Error(ctx, "RemoveGroupMember failed", "err", err)
        return &pb.GroupMemberResponse{Success: false, Error: err.Error()}, nil
    }
    // 2. Best-effort cascade; only when found.
    if found && s.globalStore != nil {
        for _, e := range groupAccess {
            if rerr := s.globalStore.RemoveShared(ctx, memberID, tenantID, e.NodeID); rerr != nil {
                log.Warn(ctx, "shared_index cascade failed", "node", e.NodeID, "err", rerr)
            }
        }
    }
    return &pb.GroupMemberResponse{Success: found}, nil
}
```

Pre-read → delete → cascade ordering is load-bearing; reordering
breaks `test_remove_group_member_cascades_shared_index`.

## Open questions / risks (idempotency)

1. **Not in the WAL (invariant 1).** Bypasses `wal.append`;
   rebuild loses it. Blocker for GA: add `RemoveGroupMemberOp` in
   `TransactionEvent.ops` and apply via `Applier.apply_event`.
   Same for AddGroupMember.
2. **Idempotency.** Removing a non-existent (group, member)
   returns `success=false, error=""`. Repeat is safe (no
   exception, no SQL violation). Cascade is idempotent —
   `global_store.remove_shared` is delete-if-exists. Net: this RPC
   is idempotent and safe to retry.
3. **No `idempotency_key`.** Unlike `ExecuteAtomic`, no receipt /
   dedup. Concurrent Add+Remove on the same pair can interleave;
   final state depends on commit order. Revisit when item 1 lands.
4. **Cascade over-deletion.** If the member ALSO has a direct
   grant on a node the group grants, the cascade still deletes
   their `shared_index` row. `ListSharedWithMe` will mis-report.
   Fix requires consulting `acl_grants` first; do NOT add the
   check without a contract test or behaviour diverges.
5. **`role` on Remove.** Proto carries `role` for both, Remove
   ignores it. v1: accept + discard. Future: deprecate via comment
   or split into two messages.
6. **Capability missing.** Any caller passing `_check_tenant` can
   remove anyone from any group in that tenant. Add admin /
   group-admin capability before GA; pin with a
   privilege-escalation test analogous to
   `test_get_edges_from_does_not_use_claimed_actor_for_authz`.
7. **Cascade pre-read race.** Pre-read and membership delete are
   not in one SQLite transaction. Concurrent `ShareNode(group_id)`
   between the two steps: a `ShareNode` that already fired
   `add_shared` for the removed member won’t be caught by the
   cascade. Consider snapshotting under one transaction in the Go
   port if affordable.
