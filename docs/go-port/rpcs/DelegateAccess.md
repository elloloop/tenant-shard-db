# RPC Port Spec — `entdb.v1.EntDBService/DelegateAccess`

EPIC #407. Source of truth: handler at `server/python/entdb_server/api/grpc_server.py:2747-2806`;
WAL helper at `server/python/entdb_server/api/admin_handlers.py:70-115`;
storage (legacy direct-write) at `server/python/entdb_server/apply/canonical_store.py:3774-3870`.

Bulk-share admin op: grants `to_user` a permission on **every node owned by
`from_user`**, optionally time-bounded. Unlike `ShareNode` (per-node;
caller must hold ADMIN on the target), DelegateAccess is tenant-admin-scoped
and operates by `owner_actor` lookup at apply time.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:131` (rpc), `:966-980` (messages).

`DelegateAccessRequest`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `actor` | 1 | `string` | Caller identity. **Untrusted** — overridden by the AuthInterceptor's authoritative actor (`grpc_server.py:2762-2764`). |
| `tenant_id` | 2 | `string` | Required (`grpc_server.py:2756-2757`). |
| `from_user` | 3 | `string` | Content owner whose nodes are being delegated. Required (`:2758-2759`). Bare user_id, NOT prefixed `user:` — matches `nodes.owner_actor` storage shape. |
| `to_user` | 4 | `string` | Recipient. Required (`:2760-2761`). Same shape as `from_user`. |
| `permission` | 5 | `string` | Legacy permission string. Defaults to `"read"` when empty (`:2766`). Persisted verbatim into `node_access.permission` by the applier. **No typed `core_caps` equivalent** on this RPC today — track for v2. |
| `expires_at` | 6 | `int64` | Epoch-ms. `0` -> `nil` -> `NULL` (permanent) (`:2767`). Past values are persisted but filtered at read by `list_shared_with_me`, pinned by `test_admin_operations.py:302-314`. |

`DelegateAccessResponse`: `{ bool success = 1; int32 delegated = 2; int64 expires_at = 3; string error = 4; }`.

`delegated` is the **count of nodes owned by `from_user` at handler-call
time** (`grpc_server.py:2786-2795`) — a SELECT before the WAL event is
applied, so an estimate, not a post-apply count. Pinned by
`test_admin_ops.py:438`. `expires_at` echoes request; `0` when permanent
(`grpc_server.py:2799`). SDK consumers depend on this shape
(`sdk/go/entdb/admin_test.go:291-333`).

## Auth (delegator must hold the right; trusted-actor)

1. `_check_tenant(tenant_id)` (`grpc_server.py:362-410`, at `:2755`) —
   sharding + region pinning. Wrong owner: `UNAVAILABLE` +
   `entdb-redirect-node` trailer. Region mismatch: `FAILED_PRECONDITION`.
2. Required-field guards (`:2756-2761`) — empty `tenant_id`/`from_user`/
   `to_user` -> `INVALID_ARGUMENT` abort.
3. `_require_admin_or_owner(...)` (`grpc_server.py:2656-2690`, at `:2762`)
   — the only permission gate. Rebinds wire `actor` to the trusted
   identity (`AuthInterceptor.get_authoritative_actor`); system actors
   short-circuit; otherwise the global-store `tenant_members.role` must be
   `owner` or `admin`. **The delegator does NOT need any per-node grant**
   — admin/owner is sufficient to bulk-delegate any user's content. That
   is the "right" the delegator must hold.
4. Privilege-escalation guard: client-supplied `actor` is discarded.
   Regression pinned by commit `fece3fb`.

`from_user`/`to_user` are NOT validated against tenant membership.

## Side effects (WAL append; ACL grant with delegation marker)

WAL-first. The handler appends an `admin_delegate_access` event via
`handle_delegate_access` (`admin_handlers.py:70-115`):

```json
{ "tenant_id": "...", "actor": "<trusted>", "idempotency_key": "admin-delegate-<uuid4>",
  "ts_ms": <ms>, "ops": [{ "op": "admin_delegate_access",
                           "from_user": "...", "to_user": "...",
                           "permission": "read|write|...", "expires_at": <ms|null> }] }
```

The handler MUST NOT call `canonical_store.delegate_access()` directly.
Pinned by `test_admin_ops.py:436-437` (`assert "delegate_access" not in cs.write_calls`).

**KNOWN GAP — fix in Go port.** `apply/applier.py:1231-1248` only
dispatches `admin_transfer_content` and `admin_revoke_access`. **No
`admin_delegate_access` branch exists** — the WAL event is
appended-and-ignored on the gRPC path. The
`test_admin_operations.py:514-536` test passes via the legacy direct-write
`canonical_store.delegate_access` route, NOT the WAL handler. Go port
MUST add a `case "admin_delegate_access"` mirroring `_sync_delegate_access`
(`canonical_store.py:3774-3810`):

```sql
SELECT node_id FROM nodes WHERE tenant_id = ? AND owner_actor = ?;
-- for each row:
INSERT OR REPLACE INTO node_access
  (node_id, actor_id, actor_type, permission, granted_by, granted_at, expires_at)
VALUES (?, to_user, 'user', permission, granted_by, now, expires_at);
```

`granted_by` is the trusted actor and is the **delegation marker** — no
separate `delegated_by` column. Auditors distinguish a normal share from a
delegation via the audit-log entry `action = "delegate_access"`
(`canonical_store.py:3850`). Consider a typed `grant_kind` column in v2.

`actor_type` is hard-coded `"user"` — no group/service delegation.
Cross-tenant: not supported. Mailbox fanout: none.

## Error contract

| gRPC code | Trigger |
|-----------|---------|
| `OK` + `success=true, delegated=N, expires_at=...` | Event appended to WAL. `N` is owner-count at request time. |
| `OK` + `success=false, error=...` | Catch-all for non-gRPC exceptions (`grpc_server.py:2801-2806`). |
| `INVALID_ARGUMENT` (abort) | Missing `tenant_id` / `from_user` / `to_user` (`:2756-2761`); also missing trusted actor (`:2675`). Pinned by `test_admin_operations.py:841-858`. |
| `PERMISSION_DENIED` (abort) | Caller is not admin/owner (`grpc_server.py:2685-2689`). Pinned by `test_admin_operations.py:538-557`. |
| `UNAVAILABLE` (abort) | Tenant not served by this node; trailer `entdb-redirect-node` carries owner. |
| `FAILED_PRECONDITION` (abort) | Region pinning mismatch. |
| `UNAUTHENTICATED` | Auth interceptor only; never raised by the handler. |

`INTERNAL` is intentionally avoided — unexpected failures route through the
soft-fail shape `OK + success=false`. Go port MUST keep this; SDK clients
parse the response field, not the status code (`sdk/go/entdb/admin.go:95-104`).

## Shared Go package deps

All under `server/go/internal/...`.

- `pb` — generated `DelegateAccessRequest`/`Response`.
- `auth` — `GetAuthoritativeActor(req.Actor) string`.
- `globalstore` — `GetMemberRole(tenantID, userID) (string, error)`.
- `wal` — `Append(ctx, topic, key, *TransactionEvent) (StreamPos, error)`.
- `apply` — applier with new `admin_delegate_access` op handler. **NEW.**
- `store` — `CountNodesByOwner(tenantID, owner) (int, error)` for `delegated` pre-count (`grpc_server.py:2787-2793`).
- `metrics` — `RecordGRPCRequest("DelegateAccess", outcome, dur)`.
- `sharding` — `_check_tenant`.
- `idgen` — UUID4 for `idempotency_key`.

NOT used: `crypto`, `audit` (WAL is the audit log, CLAUDE.md inv. #2),
`quota`, `mailbox`, `acl`/`CapabilityRegistry` (no typed caps on this RPC).

## Other-RPC deps (ShareNode similar)

- **`ShareNode`** (`grpc_server.py:1746-1826`) — per-node share; uses
  `_check_capability` (ADMIN on target) instead of `_require_admin_or_owner`.
  Shares the `node_access` table; Go port should share the writer.
- **`TransferUserContent`** (`grpc_server.py:2692-2745`) — same WAL-first
  pattern via `handle_transfer_user_content`. Its `admin_transfer_content`
  branch (`applier.py:1231-1238`) is the template for the missing
  `admin_delegate_access` branch.
- **`RevokeAllUserAccess`** — `admin_revoke_access` branch
  (`applier.py:1240-1245`). **Does NOT target delegations** — deletes from
  `node_visibility`, not `node_access`. See open questions.
- **`ListSharedWithMe`** — reader of delegated grants; filters `expires_at`.
  Pinned at `test_admin_operations.py:241-244`.
- **`ExecuteAtomic`** — WAL envelope template (`grpc_server.py:790`).

## Contract tests pinning behavior (file:line)

- `tests/python/unit/test_admin_ops.py:408-441` — WAL-first: handler
  appends `admin_delegate_access` with all fields; does NOT call
  `canonical_store.delegate_access`; `resp.delegated == 2`,
  `resp.expires_at == request.expires_at`.
- `tests/python/unit/test_admin_ops.py:443-464` — empty `permission`
  defaults to `"read"` in the WAL payload.
- `tests/python/unit/test_admin_operations.py:222-244` — bulk grant via
  direct path: 2 alice-owned nodes -> bob via `list_shared_with_me`;
  carol-owned excluded. Go applier branch must reproduce.
- `tests/python/unit/test_admin_operations.py:246-264` — `expires_at`
  persisted; permission stored verbatim.
- `tests/python/unit/test_admin_operations.py:266-281` — permanent
  delegation -> `expires_at IS NULL`.
- `tests/python/unit/test_admin_operations.py:283-300` — audit row
  `action="delegate_access"`, metadata has `from_user`, `to_user`,
  `permission`, `delegated`.
- `tests/python/unit/test_admin_operations.py:302-314` — past `expires_at`
  filtered at read time.
- `tests/python/unit/test_admin_operations.py:514-536` — handler happy
  path; `delegated=2`.
- `tests/python/unit/test_admin_operations.py:538-557` — non-admin ->
  `PERMISSION_DENIED`.
- `tests/python/unit/test_admin_operations.py:559-586` — default
  `"read"` reaches the WAL event.
- `tests/python/unit/test_admin_operations.py:841-858` — empty `to_user`
  -> `INVALID_ARGUMENT`.
- `tests/python/integration/test_grpc_contract.py:761-764` — DelegateAccess
  is **explicitly excluded** from the cross-RPC contract sweep. Add a
  contract case in EPIC #407 closeout.
- `sdk/go/entdb/admin_test.go:291-316` — SDK propagates `expires_at`.
- `sdk/go/entdb/admin_test.go:318-333` — permanent: zero `expires_at` on wire.

## Implementation outline

```go
// server/go/internal/api/delegate_access.go
func (s *EntDBServer) DelegateAccess(ctx context.Context, req *pb.DelegateAccessRequest) (*pb.DelegateAccessResponse, error) {
    start := time.Now()
    if err := s.checkTenant(ctx, req.TenantId); err != nil { return nil, err }
    if req.TenantId == "" || req.FromUser == "" || req.ToUser == "" {
        return nil, status.Error(codes.InvalidArgument, "tenant_id/from_user/to_user required")
    }
    actor, err := s.requireAdminOrOwner(ctx, req.TenantId, req.Actor, "DelegateAccess")
    if err != nil { return nil, err }

    permission := req.Permission
    if permission == "" { permission = "read" }
    var expiresAt *int64
    if req.ExpiresAt != 0 { v := req.ExpiresAt; expiresAt = &v }

    ev := wal.NewEvent(req.TenantId, actor, "admin-delegate-"+uuid.NewString(), []wal.Op{{
        "op":"admin_delegate_access", "from_user":req.FromUser, "to_user":req.ToUser,
        "permission":permission, "expires_at":expiresAt,
    }})
    if _, err := s.wal.Append(ctx, s.topic, req.TenantId, ev); err != nil {
        metrics.RecordGRPCRequest("DelegateAccess", "error", time.Since(start))
        return &pb.DelegateAccessResponse{Success: false, Error: err.Error()}, nil
    }
    delegated, _ := s.store.CountNodesByOwner(ctx, req.TenantId, req.FromUser)
    metrics.RecordGRPCRequest("DelegateAccess", "ok", time.Since(start))
    out := &pb.DelegateAccessResponse{Success: true, Delegated: int32(delegated)}
    if expiresAt != nil { out.ExpiresAt = *expiresAt }
    return out, nil
}
```

Applier branch (`server/go/internal/apply/applier.go`):

```go
case "admin_delegate_access":
    rows := tx.QueryRows("SELECT node_id FROM nodes WHERE tenant_id=? AND owner_actor=?", tenantID, op.FromUser)
    for _, r := range rows {
        tx.Exec(`INSERT OR REPLACE INTO node_access
                 (node_id, actor_id, actor_type, permission, granted_by, granted_at, expires_at)
                 VALUES (?, ?, 'user', ?, ?, ?, ?)`,
                r.NodeID, op.ToUser, op.Permission, event.Actor, event.TsMs, op.ExpiresAt)
    }
```

## Open questions / risks (revocation semantics, expiry)

- **Applier gap is a real bug.** The Python gRPC path appends
  `admin_delegate_access` and the applier silently drops it; the
  integration test at `test_admin_operations.py:514-536` only passes via
  the legacy direct-write path. Fix in both Python and Go; Go first.
- **Revocation semantics.** No symmetric `RevokeDelegation`. Options
  today: wait for `expires_at`; call `RevokeAllUserAccess(to_user)` (too
  broad — kills all shares too); or per-node `RevokeAccess`. None
  surgically reverse a delegation. Track v2 RPC `RevokeDelegation(from, to)`
  deleting `node_access` rows where `actor_id=to_user` AND
  `nodes.owner_actor=from_user`.
- **`expires_at` enforced read-side only.** Expired rows linger in
  `node_access`; readers filter `WHERE expires_at IS NULL OR expires_at > now`.
  No GC sweeper. Track a background pruner.
- **Idempotency.** Fresh `admin-delegate-<uuid4>` per call — retries
  re-run SELECT/INSERT (safe via `INSERT OR REPLACE` but bumps
  `granted_at` and writes a new audit row). Flag for receipt follow-up.
- **`delegated` is a pre-apply estimate.** New nodes created between
  SELECT and apply inflate the actual grant count (and vice versa). Don't
  fix by blocking on apply.
- **No typed `core_caps`.** ShareNode supports typed `CoreCapability`;
  DelegateAccess stores legacy strings. Address in proto v2.
- **Group/service delegation unsupported** (`actor_type` hard-coded `"user"`).
- **Actor-shape mismatch with ShareNode.** ShareNode normalises to
  `user:<id>`; DelegateAccess stores bare. Reads work because comparison
  key is bare. Don't "normalise" without auditing read paths.
