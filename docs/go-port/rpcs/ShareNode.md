# RPC Port Spec — `entdb.v1.EntDBService/ShareNode`

EPIC #407 — Python -> Go server port. Source of truth: Python handler at
`server/python/entdb_server/api/grpc_server.py:1746-1826`. Storage method:
`server/python/entdb_server/apply/canonical_store.py:2962-3051` (sync +
async wrappers). Cross-tenant index: `apply/applier.py:1689-1726`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:94` (rpc), `:720-744` (messages).

`ShareNodeRequest`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `context` | 1 | `RequestContext` | `tenant_id` + `actor`. Server overrides `actor` with the trusted identity (`grpc_server.py:1763`). |
| `node_id` | 2 | `string` | Target node. Treated as opaque; existence is **not** pre-validated, the auth check carries the existence signal (`PERMISSION_DENIED` if missing-or-no-grant). |
| `actor_id` | 3 | `string` | Recipient principal. Format `user:<id>` / `group:<id>` / `service:<id>` / `tenant:<id>`. Bare `<id>` MUST be normalised to `user:<id>` server-side (`entdb.proto:723-725`). |
| `permission` | 4 | `string` | DEPRECATED string ("read"/"write"/"admin"/"deny"). Persisted verbatim for legacy readers. Default `"read"` when empty (`grpc_server.py:1772`). |
| `actor_type` | 5 | `string` | "user" / "group" / "service". Defaults to `"user"` (`grpc_server.py:1800`). |
| `expires_at` | 6 | `int64` | Epoch-ms. `0` means "never" (treated as `nil`/`NULL`). Past values are persisted but filtered at read time (pinned by `tests/python/unit/test_acl_capabilities.py:151-162`). |
| `type_id` | 7 | `int32` | Node-type-id of target; `0` means "any". Pinned by `test_acl_capabilities.py:60-82`. |
| `core_caps` | 8 | `repeated CoreCapability` | Typed grant — authoritative when set. **This is the `Permission` enum referenced by CLAUDE.md invariant #5.** Use `pb.CoreCapability` enum values, not raw ints. |
| `ext_cap_ids` | 9 | `repeated int32` | Extension capability IDs (per-schema). |

`ShareNodeResponse`: `{ bool success = 1; string error = 2; }` — deliberately
**not** `google.rpc.Status`; the handler returns `success=false, error=<msg>`
on `PermissionError` instead of aborting (`grpc_server.py:1791-1793`). Go
port MUST preserve this — flipping to `status.Errorf` is a contract break.

Back-fill rule (in `canonical_store._sync_share_node` at `:2978-2985`): when
`core_caps` is empty, derive via `CapabilityRegistry.legacy_permission_to_core_caps(permission)`.
Pinned by `test_acl_capabilities.py:101-117` (`"write"` -> READ+COMMENT+EDIT).

## Auth

1. `_check_tenant(tenant_id)` (`grpc_server.py:362-410`) — sharding ownership +
   region pinning. On wrong owner: `UNAVAILABLE` + `entdb-redirect-node`
   trailer. On region mismatch: `FAILED_PRECONDITION`.
2. `_trusted_actor(request.context.actor)` (`grpc_server.py:418-437`) — replaces
   the wire actor with `AuthInterceptor`'s authoritative identity. **The
   request-supplied actor is UNTRUSTED** (CLAUDE.md invariant). Privilege
   escalation regression pinned by commit `fece3fb`.
3. `_check_tenant_access(..., require_write=True)` — must be tenant
   owner/admin/member; viewer/guest -> `PERMISSION_DENIED`; archived tenant
   -> `FAILED_PRECONDITION`; deleted -> `NOT_FOUND`.
4. `_check_capability(tenant, actor, node_id, type_id, "ShareNode")`
   (`grpc_server.py:299-360`) — sharing requires `CoreCapability.ADMIN` on
   the target node. System actors (`system:*`, `__system__`) and tenant
   owners short-circuit. Mapping pinned by
   `tests/python/unit/test_capability_registry.py:66`.

A grant whose `permission == "deny"` with empty caps blocks the grantor too
(`grpc_server.py:341-347`); preserve this.

## Side effects

**CLAUDE.md invariant #1 violation in Python — FIX in Go port.** The current
handler writes directly to SQLite via `canonical_store.share_node()`
(`grpc_server.py:1794-1805`) with **no `wal.append()` call**. This means a
materialized-view rebuild from the WAL silently loses every share grant.

Go port MUST:

1. Build a `TransactionEvent` with a single op `{op: "share_node", node_id,
   actor_id, actor_type, permission, expires_at, type_id, core_caps,
   ext_cap_ids, granted_by: trusted_actor}`. Use the same envelope as
   `ExecuteAtomic` (`grpc_server.py:790`).
2. `wal.Append(ctx, event)` — get back `stream_pos`.
3. Wait for the applier (or apply inline in tests) to materialise into
   `node_access`. Add a new `op_type == "share_node"` branch in
   `apply/applier.go` mirroring `_sync_share_node` (`canonical_store.py:2962`).
4. After commit, the applier calls `_update_shared_index_on_share`
   (`applier.py:1689-1726`): writes `GlobalStore.shared_index` so the
   recipient's `ListSharedWithMe` returns the share. Group `actor_id`s are
   expanded to per-member rows. Cross-tenant: when `actor_id == "tenant:<X>"`
   or recipient lives in another tenant, the row goes into the **global**
   shared_index DB (one DB per cluster, not per tenant).
5. Mailbox fanout: ShareNode does NOT currently fan a notification into the
   recipient's mailbox SQLite. The `MailboxFanoutConfig` path
   (`applier.py:295-1367`) only triggers on `create_node`. **Open question
   below** — confirm whether the Go port should add a mailbox row on share.

There is no idempotency-key dedupe today (the handler doesn't even read
`request.idempotency_key` — none on the proto). Re-issuing the same
ShareNode produces an `INSERT OR REPLACE` that updates `granted_at`
(`canonical_store.py:2989`). Preserve this.

## Error contract

| gRPC code | Trigger |
|-----------|---------|
| `OK` + `success=true` | Grant persisted. |
| `OK` + `success=false, error=...` | `PermissionError` from `_check_capability` (`grpc_server.py:1791-1793`) **or** any unexpected exception (`:1823-1826`). Go port: same — wrap the catch-all and return `success=false`. |
| `UNAVAILABLE` (abort) | Tenant not served by this node. Trailer `entdb-redirect-node` carries owner. |
| `FAILED_PRECONDITION` (abort) | Tenant region pinning mismatch, or tenant `archived`. |
| `PERMISSION_DENIED` (abort) | Caller is not a tenant member, or is a viewer/guest (write op). |
| `NOT_FOUND` (abort) | Tenant marked `deleted`. |
| `INVALID_ARGUMENT` | Reserved. Python doesn't validate `node_id`/`actor_id` shape today; Go port SHOULD reject empty strings and malformed actor prefixes here, but file as a behaviour-tightening ticket — current contract tests don't pin this. |
| `UNAUTHENTICATED` | Auth interceptor; never raised by the handler. |

`INTERNAL` is intentionally avoided; failures route through `success=false`.

## Shared Go package deps

Each lives under `server/go/internal/...` unless noted.

- `pb` (`server/go/internal/pb/entdbv1`) — generated `ShareNodeRequest`,
  `ShareNodeResponse`, `CoreCapability`, `RequestContext`. Required.
- `auth` — `AuthInterceptor` + `GetAuthoritativeActor(ctx, fallback string) string`
  (mirrors `auth/auth_interceptor.py`). Required.
- `acl` — typed-capability registry (`CapabilityRegistry`,
  `LegacyPermissionToCoreCaps`, `RequiredForOp`, `CheckGrant`). Mirrors
  `auth/capability_registry.py`. Required — this is the home of the
  `Permission`/`CoreCapability` typed enum that CLAUDE.md invariant #5
  mandates over raw strings.
- `wal` — `Append(ctx, *TransactionEvent) (StreamPos, error)`. Required to
  fix the invariant violation.
- `apply` — applier with new `share_node` op handler. Required.
- `store` (canonical) — `ShareNode(...)` writing `node_access` row. Required.
  The current Python signature is the target shape (`canonical_store.py:3008-3021`).
- `globalstore` — `AddShared(userID, srcTenant, nodeID, permission)` (mirrors
  `applier.py:1709-1721`). Required for `ListSharedWithMe` to see the share.
- `groups` (or part of `store`) — `GetGroupMembers(tenantID, groupID)` for
  group-actor expansion.
- `metrics` — `RecordGRPCRequest("ShareNode", "ok"|"denied"|"error", dur)`.
- `sharding` — for `_check_tenant`.
- `errs` — typed `PermissionError` so the catch returns `success=false` cleanly.

NOT used and MUST NOT be imported: `crypto`, `audit`, `quota`, `mailbox`
(unless we adopt the open-question fanout below).

## Other-RPC deps

- **`RevokeAccess`** (`grpc_server.py:1828-...`) — inverse op; shares the
  `node_access` table and `_update_shared_index_on_revoke` helper. Port as a
  pair to keep `acl` package internally consistent.
- **`ListSharedWithMe`** — reader of `GlobalStore.shared_index`. Cannot
  observe a share until ShareNode's applier-side index update runs. The
  contract test at `tests/python/integration/test_grpc_contract.py:339-350`
  is deliberately permissive (`check: lambda _r: True`) because of this
  race — Go port should keep both behaviours consistent.
- `AddGroupMember` / `RemoveGroupMember` — feed group expansion in `_update_shared_index_on_share`.
- `ExecuteAtomic` — the WAL envelope shape this RPC must adopt comes from
  here (`grpc_server.py:790`).
- `GetReceiptStatus` — once ShareNode goes through the WAL it gets an
  `idempotency_key`; receipt-status queries become meaningful. Add an
  `idempotency_key` field to a v2 `ShareNodeRequest` (out of scope; track).

## Contract tests pinning behavior

- `tests/python/integration/test_grpc_contract.py:327-338` — happy path: admin
  shares with `user:bob` using legacy `permission="read"`, expects
  `success=true`. **Go port must pass this verbatim.**
- `tests/python/unit/test_capability_registry.py:66` — `("ShareNode", CoreCapability.ADMIN)`.
- `tests/python/unit/test_acl_capabilities.py:60-82` — typed `core_caps` persist verbatim.
- `tests/python/unit/test_acl_capabilities.py:84-98` — typed `ext_cap_ids` persist verbatim.
- `tests/python/unit/test_acl_capabilities.py:101-117` — legacy `"write"` derives `[READ, COMMENT, EDIT]`.
- `tests/python/unit/test_acl_capabilities.py:120-148` — back-fill via `migrate_permissions_to_capabilities`.
- `tests/python/unit/test_acl_capabilities.py:151-162` — past `expires_at` filtered at read.
- `tests/python/unit/test_acl_capabilities.py:165-174` — round-trip with revoke.
- `tests/python/unit/test_shared_index.py:193-208` — `share_node` writes one `shared_index` row per recipient (group expansion covered separately).
- `tests/python/unit/test_acl_v2.py:116`, `:315` — sync-path coverage of typed grants.
- `tests/python/unit/test_admin_operations.py:355-392`, `:718` — share + admin-op interaction.
- `tests/python/unit/test_cross_tenant_read.py:75-105` — share with a `tenant:<X>` recipient lands in cross-tenant index.
- `tests/python/unit/test_sdk_hierarchical.py:241-242` — SDK -> RPC kwargs shape.

## Implementation outline

```go
// server/go/internal/api/share_node.go
func (s *EntDBServer) ShareNode(ctx context.Context, req *pb.ShareNodeRequest) (*pb.ShareNodeResponse, error) {
    start := time.Now()
    if err := s.checkTenant(ctx, req.Context.TenantId); err != nil { return nil, err }
    actor := auth.GetAuthoritativeActor(ctx, req.Context.Actor)
    if err := s.checkTenantAccess(ctx, req.Context.TenantId, actor, true); err != nil { return nil, err }

    if err := s.acl.CheckCapability(ctx, req.Context.TenantId, actor, req.NodeId, int(req.TypeId), "ShareNode"); err != nil {
        metrics.RecordGRPCRequest("ShareNode", "denied", time.Since(start))
        return &pb.ShareNodeResponse{Success: false, Error: err.Error()}, nil
    }

    ev := wal.NewEvent(req.Context.TenantId, []wal.Op{{
        "op": "share_node", "node_id": req.NodeId,
        "actor_id": normalizeActor(req.ActorId), "actor_type": defStr(req.ActorType, "user"),
        "permission": defStr(req.Permission, "read"),
        "expires_at": nilZero(req.ExpiresAt), "type_id": req.TypeId,
        "core_caps": req.CoreCaps, "ext_cap_ids": req.ExtCapIds,
        "granted_by": actor,
    }})
    if _, err := s.wal.Append(ctx, ev); err != nil {
        metrics.RecordGRPCRequest("ShareNode", "error", time.Since(start))
        return &pb.ShareNodeResponse{Success: false, Error: err.Error()}, nil
    }
    metrics.RecordGRPCRequest("ShareNode", "ok", time.Since(start))
    return &pb.ShareNodeResponse{Success: true}, nil
}
```

Applier branch (in `apply/applier.go`, alongside `create_node`):
`case "share_node": store.SyncShareNode(...); applier.UpdateSharedIndexOnShare(...)`.

`ShareNodeResponse` carries no receipt today. When the proto is bumped (#407
follow-up), add `string idempotency_key` to the request and return the
applier `stream_pos` so callers can `WaitForOffset`/`GetReceiptStatus`.

## Open questions / risks

- **WAL invariant fix is a behaviour change.** Today, ShareNode is synchronous-
  to-SQLite; clients that read their own grant immediately get it. After the
  WAL fix, there is an applier lag. Mitigation: inline-apply when
  `wal.Backend == "memory"` (test config), or expose a `WaitForApply`
  knob. Confirm with #407 owner.
- **Mailbox fanout on share.** Prompt assumes a "mailbox-of-recipient side
  effect". Code search shows no current fanout for ShareNode — only
  `create_node` triggers `_fanout_node` (`applier.py:1361-1367`). Decide:
  (a) port as-is (no mailbox row), or (b) add a `share_received` notification
  op as part of the Go port. Recommended: file follow-up; do not silently
  expand scope.
- **Cross-tenant grants**: when `actor_id` is `tenant:<X>` or
  `user:<id>@tenant:<Y>`, today the share lands in the source tenant's
  `node_access` plus `GlobalStore.shared_index`. There is **no** write into
  the recipient tenant's DB (CLAUDE.md invariant #4). Pinned by
  `test_cross_tenant_read.py:75-105`. Preserve.
- **Group expansion races**: a member added to the recipient group AFTER the
  share will not have a `shared_index` row (no back-fill). Known limitation;
  tracked in `test_shared_index.py`. Don't regress.
- **`success=false` vs `status.Errorf`**: keep the soft-fail shape. Do not
  "fix" by switching to gRPC status codes — SDK clients (Python + Go) parse
  the response field.
- **Untyped `permission` string**: the deprecated field is still authoritative
  for storage when `core_caps` is empty. Removing it is a major-version
  break; out of scope for the port.
- **Idempotency**: until the proto carries `idempotency_key`, retries write
  duplicate WAL records that collapse at the applier via `INSERT OR REPLACE`.
  Acceptable but noisy; flag for the receipt-tracking follow-up.
