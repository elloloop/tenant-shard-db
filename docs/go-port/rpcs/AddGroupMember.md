# AddGroupMember — Go Port Spec

EPIC #407. Inverse of `RemoveGroupMember`. Reference Python handler:
`server/python/entdb_server/api/grpc_server.py:1946-1984`. Proto:
`proto/entdb/v1/entdb.proto:103, 768-778`.

## Wire contract

Request `entdb.v1.GroupMemberRequest` (proto:768):
- `RequestContext context` — `tenant_id`, `actor` (UNTRUSTED — actor on
  the wire is replaced by the trusted identity at the boundary).
- `string group_id` — opaque group identifier. Convention is
  `"group:<name>"` (see `tests/python/unit/test_acl_v2.py:264, 271`).
  No server-side validation today; an empty `group_id` will INSERT a
  literal empty-string key. Go port: same parity, but flag.
- `string member_actor_id` — the principal being added. May itself be a
  group id (nested groups: `test_acl_v2.py:271, 421`). Convention is
  `"user:<id>"` or `"group:<id>"`.
- `string role` — free-form. Empty defaults to `"member"` at the
  handler (grpc_server.py:1962). No enum validation; anything goes.

Response `GroupMemberResponse` (proto:775):
- `bool success` — `true` on apply, `false` on any caught exception.
- `string error` — `str(e)` on failure (grpc_server.py:1984). gRPC
  status remains `OK` even on failure — failure is signalled
  in-band, not via status code (parity quirk; see Open Questions).

Idempotency: backed by `INSERT OR REPLACE INTO group_users`
(canonical_store.py:3505-3513). Same `(group_id, member_actor_id)`
re-added overwrites `role` and `joined_at`. Duplicate adds DO NOT
error — Go port must match.

## Auth

- Tenant existence: `_check_tenant(tenant_id, context)`
  (grpc_server.py:1954, 362).
- Trusted actor: **NOT extracted today.** The Python handler does
  *not* call `_trusted_actor` — it never reads `request.context.actor`
  at all. This is a privilege-escalation gap: any caller can mutate
  any group in any tenant they can reach. The Go port MUST close this
  gap (see CLAUDE.md §3 and commit `fece3fb` "Fix privilege escalation:
  ignore client-claimed actor in gRPC handlers"). Required Go behaviour:
    1. `trusted := auth.GetAuthoritativeActor(ctx)` — read from gRPC
       metadata, ignore `request.context.actor`.
    2. Authorise: caller must (a) be tenant admin/system, OR (b) hold
       `OWNER` or admin role on the group. Today no group-ownership
       table exists; the simplest v1 rule is **tenant admin only**,
       with a follow-up issue for group-level admin.
- Capability: not registered in `RPC_TO_CAPABILITY` (no entry found in
  `server/python/entdb_server/auth/`). Add `CoreCapability.MANAGE_ACL`
  (or new `MANAGE_GROUPS`) and pin via a unit test mirroring
  `tests/python/unit/test_capability_registry.py:62`.

## Side effects

**Current Python (NON-CONFORMANT to CLAUDE.md §1):**
1. `canonical_store.add_group_member` writes directly to SQLite
   (grpc_server.py:1958, canonical_store.py:3497-3531). **Bypasses the
   WAL.** A rebuild-from-WAL drops the membership row silently.
2. Cascade: enumerate `list_node_access_for_group(tenant, group_id)`
   and call `global_store.add_shared(member, tenant, node_id, perm)`
   for each entry (grpc_server.py:1965-1976,
   canonical_store.py:3592-3627). Failures here are logged at WARN and
   swallowed (grpc_server.py:1977-1978).
3. Metric `record_grpc_request("AddGroupMember", "ok"|"error", elapsed)`
   (grpc_server.py:1979, 1982).

**Required Go behaviour (must fix the WAL bypass):**
1. Build a `TransactionEvent` with op
   `{"type": "add_group_member", "group_id": ..., "member_actor_id":
   ..., "role": ...}` and call `wal.Append(ctx, tenantID, event)`.
2. The applier (`apply/applier.go`) handles the new op-type by
   invoking `canonicalStore.AddGroupMember` and the existing
   `_update_shared_index_on_group_member_add` cascade
   (applier.py:1782-1813). Move the cascade out of the handler and
   into the applier so a WAL replay reconstructs `shared_index`.
3. Return `GroupMemberResponse{success: true}` after the WAL append
   succeeds (do NOT block on apply — the read-after-write fence is
   the caller's responsibility via `WaitForOffset`, parity with
   `ExecuteAtomic`).

ACL recompute consequences (must be preserved):
- Adding a user to a group transitively grants them every node-access
  the group already holds (`test_acl_v2.py:412-431`,
  `test_shared_index.py:241-259`).
- Adding a *group* as a member (nested groups) makes its members
  inherit access (`test_acl_v2.py:271, 421`).
- `resolve_actor_groups` is recursive; cycles must terminate (parity
  with `_sync_resolve_actor_groups`).

## Error contract

| Condition | Python | Go port |
|---|---|---|
| Tenant missing/disabled | `_check_tenant` aborts `NOT_FOUND` / `FAILED_PRECONDITION` | match |
| Caller not authorised | (no check today — vulnerability) | `PERMISSION_DENIED` |
| Empty `group_id` or `member_actor_id` | accepted, INSERTs blank row | reject `INVALID_ARGUMENT` (hardening) |
| Same `(group_id, member)` re-added | `success=true`, role overwritten | match (idempotent) |
| Cascade `add_shared` failure | logged WARN, `success=true` | match — cascade is best-effort |
| WAL append failure | n/a (no WAL today) | `UNAVAILABLE`; do NOT touch SQLite |
| Internal error | bare `except`, return `success=false, error=str(e)`, status OK | match in v1; flag for review |

Status-code parity quirk: Python returns `OK` with `success=false` on
internal error rather than a non-OK gRPC status. Match for v1; the
mainline error path stays in-band.

## Shared Go package deps

- `internal/auth` — `GetAuthoritativeActor(ctx)`, admin/role checks.
- `internal/tenant` — `CheckTenant`.
- `internal/wal` — `Append(ctx, tenantID, TransactionEvent)`. Backends
  per `server/python/entdb_server/wal/` (Kafka, Kinesis, SQS, memory).
- `internal/apply` — `Applier.ApplyEvent` dispatches the new
  `add_group_member` op-type; cascade lives here.
- `internal/canonical` (a.k.a. `internal/store`) —
  `AddGroupMember(tenantID, groupID, memberActorID, role)`,
  `ListNodeAccessForGroup(tenantID, groupID)`,
  `ResolveActorGroups(tenantID, actor)`.
- `internal/global` — `AddShared(userID, sourceTenant, nodeID, perm)`
  for the cascade (global_store.py:670-690).
- `internal/metrics` — `RecordGRPCRequest`.
- `internal/acl/groups` — pure helpers for nested-group resolution
  (cycle-safe BFS).

## Other-RPC deps

- `RemoveGroupMember` (proto:106, grpc_server.py:1986-…) is the exact
  inverse: same wire shape, `DELETE` on `group_users`, cascade-removes
  shared_index entries that were granted *only* via this group
  (applier.py:1815-…). Spec the WAL op as `remove_group_member`.
- `ShareNode` / `RevokeAccess` populate `node_access` rows that
  `list_node_access_for_group` reads — adding a member after a share
  must back-fill `shared_index`; adding before means the next
  `ShareNode` cascade picks them up via `get_group_members`
  (`test_shared_index.py:241-259`).
- Contract tests order `AddGroupMember` before `RemoveGroupMember`
  intentionally (`test_grpc_contract.py:374`).

## Contract tests pinning behavior

- `tests/python/integration/test_grpc_contract.py:362-373` — happy
  path, `success == true`, payload shape pinned.
- `tests/python/unit/test_acl_v2.py:578-591` — add → resolve includes
  group; remove → resolve excludes; remove-nonexistent returns false.
- `tests/python/unit/test_acl_v2.py:264-279` — group resolution and
  nested-group expansion.
- `tests/python/unit/test_acl_v2.py:412-431` — adding member to a
  group transitively grants the group's existing node access.
- `tests/python/unit/test_acl_v2.py:476` — group ACL participates in
  `can_access`.
- `tests/python/unit/test_shared_index.py:241-259` — group share
  expands to per-member `shared_index` entries.
- `tests/python/unit/test_cross_tenant_read.py:366-404` — group
  membership across tenant boundary for cross-tenant reads.
- `tests/python/unit/test_admin_operations.py:369-371, 719` — admin
  ops invoke `add_group_member` directly.

Go port adds (`tests/contract/`): WAL-append observed before SQLite
write; replay-from-empty-WAL reconstructs membership; trusted-actor
substitution rejects forged admin claim; idempotent re-add overwrites
role; cascade back-fills `shared_index`.

## Implementation outline

```go
func (s *Server) AddGroupMember(ctx context.Context, req *pb.GroupMemberRequest) (*pb.GroupMemberResponse, error) {
    start := time.Now()
    st := "ok"
    defer func() { metrics.RecordGRPCRequest("AddGroupMember", st, time.Since(start)) }()

    if err := s.checkTenant(ctx, req.Context.TenantId); err != nil { st = "error"; return nil, err }
    trusted := auth.GetAuthoritativeActor(ctx) // ignore req.Context.Actor
    if err := s.authz.RequireGroupAdmin(ctx, req.Context.TenantId, trusted, req.GroupId); err != nil {
        st = "error"; return nil, err // PERMISSION_DENIED
    }
    if req.GroupId == "" || req.MemberActorId == "" {
        st = "error"; return nil, status.Error(codes.InvalidArgument, "group_id and member_actor_id required")
    }
    role := req.Role
    if role == "" { role = "member" }

    ev := wal.NewEvent(req.Context.TenantId, trusted, []wal.Op{{
        Type:          "add_group_member",
        GroupID:       req.GroupId,
        MemberActorID: req.MemberActorId,
        Role:          role,
    }})
    if _, err := s.wal.Append(ctx, req.Context.TenantId, ev); err != nil {
        st = "error"
        return &pb.GroupMemberResponse{Success: false, Error: err.Error()}, nil // parity: in-band
    }
    return &pb.GroupMemberResponse{Success: true}, nil
}
```

Applier handler (sketch):
```go
case "add_group_member":
    if err := store.AddGroupMember(ctx, tid, op.GroupID, op.MemberActorID, op.Role); err != nil { return err }
    // Cascade — best effort, do not fail the apply.
    entries, _ := store.ListNodeAccessForGroup(ctx, tid, op.GroupID)
    for _, e := range entries {
        _ = global.AddShared(ctx, op.MemberActorID, tid, e.NodeID, e.Permission)
    }
```

Notes:
- Cascade runs in the applier so a WAL replay reconstructs
  `shared_index` (CLAUDE.md §1).
- `INSERT OR REPLACE` semantics keep replay idempotent.
- No `WaitForOffset` inside the handler — caller fences via
  `after_offset` on subsequent reads.

## Open questions / risks

1. **Auth model is undefined.** No "group owner" or "group admin"
   concept exists in the schema today. v1 rule = tenant admin only;
   file follow-up to introduce per-group admin (proto field on group
   creation, new `group_admins` table). Until then, this RPC is
   strictly an admin op.
2. **WAL bypass in Python is a known parity-vs-correctness conflict.**
   The Go port MUST route through the WAL (CLAUDE.md §1) even though
   that is technically a behaviour change. Coordinate with EPIC #407
   so the contract test for "membership survives rebuild" lands at
   the same time as the Go server, otherwise Python CI will fail.
3. **Idempotency on `INSERT OR REPLACE`** silently overwrites `role`
   and `joined_at`. Consider whether re-add should be a no-op when
   role matches, or always bump `joined_at`. Recommend: always bump
   to match Python.
4. **Nested groups + cycles.** Adding `group:a` as member of
   `group:b` and then `group:b` as member of `group:a` is currently
   accepted; `_sync_resolve_actor_groups` must guard with a `visited`
   set. Pin a contract test (`tests/contract/group_cycle.go`) — the
   Go BFS must terminate.
5. **Cascade error swallowing.** `add_shared` failures are WARN-
   logged. Under WAL replay, a transient global_store outage could
   leak missing `shared_index` rows. Add an idempotent
   reconcile job (`shared_index_rebuild`) — out of scope for v1.
6. **In-band error vs gRPC status.** Returning `OK / success=false`
   conflicts with idiomatic Go gRPC. Match for v1, file deprecation
   issue for v2 to switch to `codes.Internal`.
7. **No capability registry entry.** `tests/python/unit/test_capability_registry.py`
   has no row for `AddGroupMember`. Add one (`CoreCapability.MANAGE_ACL`
   or new `MANAGE_GROUPS`) before merging the Go port.
8. **`group_id` / `member_actor_id` validation.** Empty strings,
   `"user:"` (no id), or trailing whitespace are all currently
   accepted. Recommend `INVALID_ARGUMENT` on empty; defer richer
   validation to a separate hardening PR.
