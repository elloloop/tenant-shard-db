# RPC Port Spec — `entdb.v1.EntDBService/TransferUserContent`

EPIC #407 — Python -> Go server port. Source of truth: Python handler at
`server/python/entdb_server/api/grpc_server.py:2692-2745`.

> Offboarding flow. Reassigns ownership of every node a user owns inside a
> single tenant to a new owner, and ensures the new owner is a tenant member.
> WAL-first: the per-tenant ownership change MUST be appended as an
> `admin_transfer_content` event so it survives WAL rebuild
> (CLAUDE.md invariant 1). Direct canonical-store writes from this handler
> are forbidden and pinned against by `tests/python/unit/test_admin_ops.py:373-407`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:130` (rpc), `:953-958` (request),
`:960-964` (response).

`TransferUserContentRequest` (note: this RPC does NOT use the standard
`RequestContext` envelope — `actor`/`tenant_id` are top-level scalars):

| Field | Tag | Type | Notes |
|------|-----|------|------|
| `actor` | 1 | `string` | Wire-untrusted. MUST be replaced by `_trusted_actor()` (gRPC peer identity) before any auth check. See "Auth". |
| `tenant_id` | 2 | `string` | Required. Validated by `_check_tenant` (`grpc_server.py:362`). |
| `from_user` | 3 | `string` | Required. Owner being offboarded. Bare id form (e.g. `"alice"`) — translation rules per CLAUDE.md. |
| `to_user` | 4 | `string` | Required. New owner. May or may not already be a tenant member (idempotent membership upsert). |

`TransferUserContentResponse`:

| Field | Tag | Type | Semantics |
|------|-----|------|------|
| `success` | 1 | `bool` | `true` on append + bookkeeping success; `false` for non-abort errors (handler returns the response with `error` populated). |
| `transferred` | 2 | `int32` | **Pre-apply count.** SELECT `COUNT(*) FROM nodes WHERE owner_actor = from_user` before append. Reflects nodes still owned at the time of the call; the handler waits for WAL materialization before returning. |
| `error` | 3 | `string` | Non-empty only on caught (non-abort) exceptions; abort cases use gRPC status, not this field. |

## Auth (admin; trusted-actor)

1. `_check_tenant(request.tenant_id)` — rejects unknown/archived tenants and
   wrong-region requests (`grpc_server.py:362`).
2. INVALID_ARGUMENT validation: `tenant_id`, `from_user`, `to_user`
   non-empty (`:2701-2706`).
3. `_require_admin_or_owner(tenant_id, request.actor, ctx, "TransferUserContent")`
   (`:2707-2709`, defined `:2680-2690`):
   - **MUST ignore** `request.actor` for trust — calls `_trusted_actor()`
     to derive the identity from the gRPC peer, then resolves the role via
     `_get_member_role(tenant_id, actor_uid)`.
   - Allowed roles: `owner`, `admin`. Anything else (`member`, `viewer`,
     non-members) -> `PERMISSION_DENIED`.
   - System actors (`system:*`) bypass membership and pass — pinned by
     `test_admin_operations.py:489-505`.
   - Privilege-escalation regression pinned by
     `tests/python/integration/test_privilege_escalation.py:344-365`:
     a client claiming `actor="user:admin"` while the trusted identity is
     `user:eve` MUST `PERMISSION_DENIED` AND MUST NOT append to the WAL.

The Go port MUST keep the trusted-actor lookup inside the same code path as
the WAL append so a refactor cannot accidentally drop the auth check while
keeping the side effect.

## Side effects (potentially many WAL events; bulk ACL changes; mailbox cascades)

Order in the Python handler (`grpc_server.py:2715-2725` ->
`api/admin_handlers.py:27-67`):

1. **WAL append.** Single tenant-scope event carrying both the tenant
   ownership rewrite and the paired global membership upsert:
   ```json
   {"tenant_id": T, "actor": <trusted>,
    "idempotency_key": "admin-transfer-<uuid4>",
    "ts_ms": <now>,
    "ops": [{"op": "admin_transfer_content",
             "from_user": F, "to_user": T2},
            {"op": "access_transferred",
             "tenant_id": T, "from_user": F, "to_user": T2,
             "joined_at": <now>}]}
   ```
   Topic: `self.topic` (default `"entdb-wal"`), key: `tenant_id`.

2. **Applier materialization** (`apply/applier.py:1231-1238`): single
   `UPDATE nodes SET owner_actor=?, updated_at=? WHERE tenant_id=? AND
   owner_actor=?`. Bulk-rewrites ownership in one SQL statement — O(rows) but
   one round-trip per event.

   The Python Applier currently does NOT refresh `node_visibility` on this
   op (the eager `transfer_user_content` path does, via
   `_update_visibility`, `canonical_store.py:3707-3715`). The Go port must
   match the Applier path: add visibility-index refresh inside the applier
   handler so behavior survives rebuild. See "Open questions".

3. **Audit log.** Eager `canonical_store.transfer_user_content` writes
   `action="transfer_content"` (`canonical_store.py:3753-3766`). Under the
   WAL-first handler this path is NOT called (handler does not invoke
   `canonical_store`). Audit trail comes from the WAL itself + S3 Object
   Lock per CLAUDE.md invariant 2 — do NOT add a parallel audit table.

5. **Mailbox / notifications cascade.** Out of scope for this RPC. Nodes
   keep their notification rows (keyed by node, not owner). No fanout is
   triggered; downstream subscribers observing the WAL see the
   `admin_transfer_content` event and can react.

6. **ACL blobs.** Not modified. Existing per-node ACL grants survive — only
   `owner_actor` changes.

## Error contract

| Condition | Status | Where |
|-----------|--------|-------|
| Empty `tenant_id` / `from_user` / `to_user` | `INVALID_ARGUMENT` | `grpc_server.py:2701-2706` |
| Unknown / archived tenant, wrong region | per `_check_tenant` | `:2700` |
| Caller not admin/owner (and not `system:*`) | `PERMISSION_DENIED` | `:2685-2689` |
| Claimed-admin actor with non-admin trusted identity | `PERMISSION_DENIED`, no WAL append | `test_privilege_escalation.py:344-365` |
| Missing WAL/store/globalstore wiring | `UNIMPLEMENTED` | Go WAL-first dependency gate |
| WAL encode/append failure | `success=false`, `error=str(e)` (NOT abort) | `:2740-2745` |
| Wait-applied timeout/failure | gRPC status (`DEADLINE_EXCEEDED` / typed apply failure) | Go WAL-first wait contract |

Note: the count-query failure path is swallowed — `transferred=0` is returned
even if SELECT fails before append.

`grpc.RpcError` / abort exceptions re-raise; everything else is mapped to
the response with `success=false`.

## Shared Go package deps

- `internal/auth` — `TrustedActor(ctx)`, `RequireAdminOrOwner(ctx, tenantID, rpcName)`
  (consolidated form of `_trusted_actor` + `_require_admin_or_owner`).
- `internal/tenant` — `CheckTenant(ctx, tenantID)`.
- `internal/wal` — `WALStream.Append(ctx, topic, key, value)`.
- `internal/store/global` — `TransferUserContent(ctx, tenantID, fromUser, toUser) (membershipCreated bool, err error)`.
- `internal/store/canonical` — read-only `CountOwnedNodes(ctx, tenantID, owner) (int32, error)` for the post-append count. **Must NOT expose a writer call from the handler path.**
- `internal/event` — `BuildAdminTransferContentEvent(tenantID, actor, from, to) []byte` (JSON encoder; idempotency key generator).
- `internal/metrics` — `RecordGRPCRequest("TransferUserContent", outcome, dur)`.
- `internal/applier` — handler for `admin_transfer_content` op (parity with `applier.py:1231-1238`).

## Other-RPC deps (TransferOwnership, RevokeAllUserAccess)

- **`TransferOwnership`** (`proto:109`, `:780-789`): single-node owner change.
  Different code path; uses `RequestContext` envelope and per-node update.
  Shares no implementation with `TransferUserContent` but ports should land
  together since both rewrite `nodes.owner_actor`.
- **`RevokeAllUserAccess`** (`proto:133`, `:994-1006`): the natural follow-up
  call after `TransferUserContent` in the offboarding flow. Emits an
  `admin_revoke_access` WAL event (`admin_handlers.py:143-180`,
  `applier.py:1240-1245`). Together they form the offboarding pair: transfer
  first (so the user owns nothing), then revoke (so the user has no ACL or
  group access). Operators may script this; no server-side coupling.
- **`DelegateAccess`** (`proto:131`): adjacent admin op, same WAL-first
  pattern (`admin_handlers.py:70-115`). Use as a structural template.

## Contract tests pinning behavior (file:line)

- `tests/python/unit/test_admin_operations.py:153-213` — canonical-store
  level: count returned, untouched-owners preserved, audit row, visibility
  index refreshed.
- `tests/python/unit/test_admin_operations.py:426-505` — gRPC handler:
  owner allowed, member denied, viewer denied, `system:*` allowed.
- `tests/python/unit/test_admin_operations.py:806-839` — INVALID_ARGUMENT
  on missing `actor` / `from_user`.
- `tests/python/unit/test_admin_ops.py:88-119` — global-store: membership
  upsert idempotency, returned fields.
- `tests/python/unit/test_admin_ops.py:127-144` — canonical-store ownership
  rewrite + zero-when-empty.
- `tests/python/unit/test_admin_ops.py:373-407` — **WAL-first contract**:
  exactly one append with `op="admin_transfer_content"`, NO direct
  `canonical_store.transfer_user_content` call, response carries pre-apply
  count.
- `tests/python/integration/test_privilege_escalation.py:344-365` —
  trusted-actor: claimed-admin string ignored; PERMISSION_DENIED + no WAL.
- `tests/python/integration/test_grpc_contract.py:558-572` — happy path
  (`success=True`) and INVALID_ARGUMENT on empty `tenant_id`.

## Implementation outline (atomicity? streaming?)

Not streaming — single unary request/response. Suggested Go shape:

```
func (s *Server) TransferUserContent(ctx, req) (*resp, error) {
    if err := s.checkTenant(ctx, req.TenantId); err != nil { return nil, err }
    if req.TenantId == "" || req.FromUser == "" || req.ToUser == "" {
        return nil, status.Error(InvalidArgument, "<field> is required")
    }
    trusted, err := s.auth.RequireAdminOrOwner(ctx, req.TenantId, "TransferUserContent")
    if err != nil { return nil, err }   // PERMISSION_DENIED on non-admin

    n, _ := s.cstore.CountOwnedNodes(ctx, req.TenantId, req.FromUser)
    // One tenant WAL event carries admin_transfer_content + access_transferred.
    payload := event.BuildAdminTransferContentAndAccessTransferred(req.TenantId, trusted, req.FromUser, req.ToUser)
    pos, idem, err := s.wal.Append(ctx, s.topic, req.TenantId, payload)
    if err != nil {
        return &resp{Success: false, Error: err.Error()}, nil
    }
    if err := s.waitForAdminApplied(ctx, req.TenantId, pos.Offset, idem); err != nil { return nil, err }
    return &resp{Success: true, Transferred: n}, nil
}
```

**Atomicity.** There is NO cross-store transaction, but the Go handler now
puts the global membership op in the same tenant WAL event. If the global op
fails before tenant commit, the tenant batch rolls back. If the global write
materializes and tenant commit later fails, replay converges because
`access_transferred` is idempotent.
The Applier's UPDATE is itself atomic per tenant SQLite. Applier
visibility-index refresh, if added, must run inside the same transaction as
the UPDATE.

**Streaming.** Not needed at the RPC layer. Go chunks large owners into
multiple WAL events so each applier transaction stays bounded.

**Idempotency.** Event idempotency key is `admin-transfer-<uuid4>` — every
call produces a NEW key, so retries cause repeat applies. The bulk UPDATE
is naturally idempotent (a second apply finds zero matching rows). Watch
out: visibility-index refresh must also be idempotent.

## Open questions / risks

1. **Cross-tenant.** Hard scoped — `tenant_id` partitions both the WAL key
   and the SQLite database file. No cross-tenant transfer is expressible.
   `from_user`/`to_user` strings collide across tenants but are
   tenant-scoped semantically. Confirm Go port keeps `tenant_id` as the
   WAL partition key.

2. **Large user (10M+ owned nodes).** Go mitigates this by enumerating
   owned node IDs and chunking events. Each applier transaction updates at
   most the configured chunk size.

3. **Visibility-index drift after rebuild.** The eager
   `canonical_store.transfer_user_content` path refreshes
   `node_visibility` per node (`canonical_store.py:3707-3715`). The
   Applier path (the only one used by the handler) does NOT
   (`applier.py:1231-1238`). On full WAL rebuild, the visibility index for
   transferred nodes will be stale until the next ACL touch. **The Go
   Applier port MUST refresh visibility for affected nodes inside the
   same op handler** to close this gap, and a contract test should pin
   it.

5. **Pre-apply count can race concurrent writes.** The count is taken
   before the WAL append. Other handlers can mutate ownership between the
   count and applier materialization, so the returned count is advisory,
   not a post-apply rowcount.

6. **`actor` field is wire-untrusted but required to be non-empty.** The
   non-empty check (`:2701`) runs before `_require_admin_or_owner`
   substitutes the trusted actor. A client sending `actor=""` aborts
   `INVALID_ARGUMENT` even if the trusted identity is a legitimate admin.
   Pinned by `test_admin_operations.py:806-820`. Preserve this quirk.

7. **No legal-hold check.** Legal hold blocks GDPR deletion but does NOT
   block ownership transfer. Confirm with compliance whether this should
   change before the Go cutover.
