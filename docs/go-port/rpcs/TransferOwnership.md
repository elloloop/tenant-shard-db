# TransferOwnership — Go Port Spec

EPIC #407. Reference Python handler:
`server/python/entdb_server/api/grpc_server.py:2030-2049`. Proto:
`proto/entdb/v1/entdb.proto:108-109, 780-789`. Capability declaration:
`server/python/entdb_server/auth/capability_registry.py:75`.

> NOTE: the Python handler is thin and intentionally under-enforced.
> Go port MUST close the gaps in "Open questions / risks" before
> ship. Behaviour pinned by contract tests is preserved verbatim;
> the rest is hardened.

## Wire contract

Request `entdb.v1.TransferOwnershipRequest` (proto:780):
- `RequestContext context` — tenant_id, actor (UNTRUSTED, see Auth),
  trace_id.
- `string node_id` — node whose `owner_actor` is being reassigned.
- `string new_owner` — fully-qualified principal string
  (`"user:alice"`, `"group:admins"`, or `"system"`). The Python store
  writes whatever string it receives; there is no validation today
  (canonical_store.py:3631-3650). Go port MUST validate the prefix
  matches `Actor.user(...)` / `Actor.group(...)` (CLAUDE.md §"Actors")
  and reject empty strings with `INVALID_ARGUMENT`.

Response `TransferOwnershipResponse` (proto:786):
- `bool found` — `true` iff the row existed and was updated
  (`cursor.rowcount > 0`, canonical_store.py:3650). For a missing
  node, returns `found=false` (no error string). Pinned by
  `test_acl_v2.py:534-536`.
- `string error` — set ONLY on exception path
  (grpc_server.py:2049). Successful "node not found" returns
  `(found=false, error="")`.

## Auth (current owner only; trusted-actor)

Today the handler ONLY calls `_check_tenant` (grpc_server.py:2038)
and trusts whatever caller asks. This is a known gap.

Go port MUST enforce, before any mutation:
1. `_check_tenant(tenant_id)` — tenant exists and is not archived
   (grpc_server.py:362).
2. `trustedActor := AuthInterceptor.GetAuthoritativeActor(ctx)` —
   IGNORE `request.context.actor`. Same fix as commit fece3fb
   ("ignore client-claimed actor in gRPC handlers"). Pinned by
   `tests/python/integration/test_privilege_escalation.py`.
3. `_check_tenant_access(tenant_id, trustedActor, require_write=true)`
   — caller must be a writing tenant member.
4. Capability check: `TransferOwnership` requires `CORE_CAP_ADMIN`
   on the node (capability_registry.py:75). Use
   `_check_capability(tenant_id, trustedActor, node_id, type_id,
   "TransferOwnership")`, mirroring `ShareNode`
   (grpc_server.py:1782-1793). On `PermissionError` return
   `(found=false, error="permission denied: …")` — no gRPC abort,
   string-error in the response is the existing convention for ACL
   ops (`ShareNode`/`RevokeAccess`).
5. Effective rule: ADMIN includes the current owner (owner is
   implicitly ADMIN per `_sync_can_access`). Non-owner admins (system
   actor, ACL-granted ADMIN) can also transfer; this matches existing
   `ShareNode` semantics and is intentional.

`new_owner` does NOT need to be an existing tenant member today —
the field is a free-form principal string. Go port should warn-log
when `new_owner` resolves to a user that is not a member of the
tenant; do not reject (would break test_grpc_contract.py:387-393
which uses `BOB` without membership setup).

## Side effects

### WAL append (REQUIRED — gap vs CLAUDE.md §1)

Python today writes SQLite directly (canonical_store.py:3637-3650),
violating the invariant "every mutation goes through the WAL". Go
port MUST:
- Append a new `OwnershipTransferOp` (or reuse a generic
  `AdminOp.transfer_ownership`) inside a `TransactionEvent` to the
  WAL via the shared `wal.Append` package.
- Have the `Applier` apply it: `UPDATE nodes SET owner_actor=?` and
  refresh `node_visibility` (mirrors `_update_visibility` call at
  canonical_store.py:3643-3649).
- Block the handler on `WaitForOffset` if `wait_applied=true` is
  signalled via context — TransferOwnership has no such field today,
  but the Go port should default to **synchronous-apply** (the
  current Python store call is synchronous-equivalent because it
  bypasses the WAL).

Add a new op variant under `TransactionEvent.ops` in the proto and a
matching handler in `Applier.apply_event()`. Do NOT shoehorn this
into `UpdateNodeOp` — `owner_actor` is not a payload field and is
not in `field_mask`.

### ACL changes

On apply:
- `nodes.owner_actor` row mutated (canonical_store.py:3639-3641).
- `node_visibility` rebuilt for that node with the NEW owner as a
  principal (canonical_store.py:3643-3649). The OLD owner is dropped
  unless they retain access via an ACL grant — verified by
  `test_acl_v2.py:528-532` (ALICE can access pre-transfer; BOB can
  access post-transfer; no assertion that ALICE loses access — but
  the visibility rebuild makes that the de-facto outcome).
- The node's existing `acl_blob` is NOT modified. Explicit grants
  to the previous owner survive (this is by design — if an admin
  shared the node back to the old owner, they keep that grant).

### Mailbox

No mailbox effects. TransferOwnership touches `nodes` only.
`MailboxItem`s keyed by `user_id` are untouched. New owner does NOT
receive a notification (out of scope).

### Cross-tenant `global_store.shared_index`

Python TransferOwnership does NOT touch `shared_index`. Pre-existing
share rows pointing at this node remain valid (the new owner having
a redundant share row is harmless; old owner's inbox-of-shares is
independent of ownership). Go port: no `global_store` writes.

Cross-tenant transfer is NOT supported — `new_owner` is resolved in
the requesting tenant's namespace. Use `TransferUserContent` for
bulk reassignment. Reject `new_owner` strings with tenant qualifiers.

## Error contract

| Condition                                  | Response                                                                  |
|--------------------------------------------|---------------------------------------------------------------------------|
| Tenant unknown / archived                  | gRPC `NOT_FOUND` from `_check_tenant` (matches existing pattern)          |
| Empty `node_id` or empty `new_owner`       | `INVALID_ARGUMENT` (Go port hardens; Python passes through)               |
| Caller lacks ADMIN on node                 | `(found=false, error="permission denied: …")` (mirrors ShareNode)         |
| Node does not exist                        | `(found=false, error="")` — pinned by `test_acl_v2.py:534-536`            |
| Internal failure (SQLite, WAL append)      | `(found=false, error=str(exc))` — matches Python catch-all (line 2049)    |

The handler MUST NOT raise gRPC status codes for the "not found"
case — the contract test asserts `r.found is True` for the happy
path (`test_grpc_contract.py:392`) and `False` for missing rows;
both via the message body, not status.

## Shared Go package deps

- `internal/auth` — `AuthInterceptor.GetAuthoritativeActor`,
  `_trusted_actor` equivalent.
- `internal/auth/capability` — `CapabilityRegistry`, op→cap map
  (mirrors `capability_registry.py`).
- `internal/store/canonical` — per-tenant SQLite handle pool, the
  `transfer_ownership` sync impl, `update_visibility` helper.
- `internal/wal` — `Append(event)`. Backend-agnostic
  (Kafka/Kinesis/SQS/memory).
- `internal/apply` — `Applier.Apply(event)`. New case for the
  ownership-transfer op.
- `internal/observability/metrics` — `record_grpc_request` →
  `metrics.RecordGRPC("TransferOwnership", outcome, dur)`.
- `internal/actor` — `Actor.User(...)` / `Actor.Group(...)` parser
  for `new_owner` validation.

## Other-RPC deps

- `ExecuteAtomic` — shares the WAL append + apply path; the Go port
  should reuse the same `wal.Append` and `Applier` machinery.
- `WaitForOffset` — if a future revision adds `wait_applied`, this
  is the synchronisation primitive.
- `ShareNode` / `RevokeAccess` — share the capability-check pattern
  (`grpc_server.py:1782-1793`). Copy the same scaffolding.
- `TransferUserContent` (proto:`TransferUserContentRequest`) — the
  bulk, cross-user variant. Distinct semantics: bulk reassign by
  old-owner identity, not per-node.

## Contract tests pinning behavior (file:line)

- `tests/python/unit/test_acl_v2.py:525-536` — happy path returns
  `True`, ALICE→BOB owner change makes the node visible to BOB; a
  missing `node_id` returns `False`.
- `tests/python/integration/test_grpc_contract.py:385-393` — gRPC
  wire happy path: `TransferOwnershipRequest(context, SEED_NODE_ID,
  BOB)` ⇒ `r.found is True`.
- `tests/python/unit/test_admin_ops.py:127-144` — adjacent
  `transfer_user_content` tests; useful as a sanity reference for
  what BULK ownership transfer looks like (do NOT confuse with this
  RPC).
- `server/python/entdb_server/auth/capability_registry.py:75` —
  fixes `TransferOwnership: CoreCapability.ADMIN` for the
  capability table.

No existing test asserts: WAL append occurred, old owner loses
visibility, or non-admin caller is rejected. The Go port adds those
tests under `tests/contract/` as part of EPIC #407.

## Implementation outline

```go
func (s *Server) TransferOwnership(
    ctx context.Context, req *pb.TransferOwnershipRequest,
) (*pb.TransferOwnershipResponse, error) {
    start := time.Now()
    defer metrics.RecordGRPC("TransferOwnership", &outcome, start)

    // 1. Validate.
    if req.NodeId == "" || req.NewOwner == "" {
        return nil, status.Error(codes.InvalidArgument, "node_id/new_owner required")
    }
    if _, err := actor.Parse(req.NewOwner); err != nil {
        return nil, status.Error(codes.InvalidArgument, err.Error())
    }
    // 2. Auth.
    if err := s.checkTenant(ctx, req.Context.TenantId); err != nil { return nil, err }
    trusted := auth.TrustedActor(ctx)
    if err := s.checkTenantAccess(ctx, req.Context.TenantId, trusted, true); err != nil {
        return nil, err
    }
    if err := s.checkCapability(ctx, req.Context.TenantId, trusted,
        req.NodeId, /*typeId*/0, "TransferOwnership"); err != nil {
        return &pb.TransferOwnershipResponse{Found: false, Error: err.Error()}, nil
    }
    // 3. WAL append.
    evt := wal.NewTransactionEvent(req.Context.TenantId, trusted,
        wal.TransferOwnershipOp{NodeId: req.NodeId, NewOwner: req.NewOwner})
    if err := s.wal.Append(ctx, evt); err != nil {
        return &pb.TransferOwnershipResponse{Found: false, Error: err.Error()}, nil
    }
    // 4. Synchronous apply (or wait-for-offset).
    found, err := s.applier.WaitTransferOwnership(ctx, evt.Offset)
    if err != nil { return &pb.TransferOwnershipResponse{Found: false, Error: err.Error()}, nil }
    return &pb.TransferOwnershipResponse{Found: found}, nil
}
```

`Applier.applyTransferOwnership`: `UPDATE nodes SET owner_actor=?
WHERE tenant_id=? AND node_id=?`; if rowcount > 0, refresh
`node_visibility`. Return `rowcount > 0` to the caller via the
`WaitTransferOwnership` channel.

## Open questions / risks

1. **Atomicity across tenants?** N/A — single-tenant by construction.
   Cross-tenant ownership transfer would need 2PC/saga between
   tenant shards; out of scope. Reject tenant-qualified `new_owner`.
2. **WAL gap.** Python bypasses the WAL — replay would silently
   lose ownership transfers (CLAUDE.md §1 violation). Go port MUST
   close this. Add a replay-rebuild test that asserts owner state.
3. **Old owner self-access.** Owner is implicitly ADMIN, so ALICE
   loses access on transfer (no explicit ACL row to retain). Confirm
   product intent; alternative is auto-grant old-owner READ.
4. **Old owner's outgoing shares.** If ALICE shared with CHARLIE,
   CHARLIE retains access post-transfer until BOB calls RevokeAccess.
5. **Mailbox notification?** Decide if BOB gets a "you now own X"
   item. Out of scope for parity.
6. **Owner-only vs ADMIN-only.** Spec title says "current owner";
   capability_registry.py:75 says ADMIN. Go port follows the
   capability table (ADMIN) — that is the pinned source of truth.
