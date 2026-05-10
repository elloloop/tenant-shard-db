# RPC Port Spec — `entdb.v1.EntDBService/RevokeAllUserAccess`

EPIC #407 — Python → Go server port. Source of truth: Python handler at
`server/python/entdb_server/api/grpc_server.py:2864-2921`.

Use case: **emergency offboarding**. An admin (or compliance officer) yanks
every form of access a user has inside one tenant in a single shot — direct
ACL grants, group memberships, visibility rows, and the cross-tenant
`shared_index` rows that point back at this tenant.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:133` (rpc), `:994-1006` (messages).

`RevokeAllUserAccessRequest`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `actor` | 1 | `string` | Caller-claimed actor. **UNTRUSTED** — overridden by `AuthInterceptor`. See Auth. |
| `tenant_id` | 2 | `string` | Required. Empty → `INVALID_ARGUMENT`. |
| `user_id` | 3 | `string` | Required. Empty → `INVALID_ARGUMENT`. Format is the canonical actor string (e.g. `user:alice`), matching `node_access.actor_id`. |

Note: this RPC does **not** use `RequestContext` — it carries `actor` and
`tenant_id` as flat fields like the rest of the admin/GDPR family
(`TransferUserContent`, `DelegateAccess`, `SetLegalHold`, `DeleteUser`).

`RevokeAllUserAccessResponse`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `success` | 1 | `bool` | True iff every step succeeded. False with `error` set on caught exception (auth/`_check_tenant` aborts re-raise). |
| `revoked_grants` | 2 | `int32` | `DELETE FROM node_access WHERE actor_id = ?` rowcount. |
| `revoked_groups` | 3 | `int32` | `DELETE FROM group_users WHERE member_actor_id = ?` rowcount. |
| `revoked_shared` | 4 | `int32` | Count of `global_store.shared_index` rows where `source_tenant == tenant_id` removed for this user. |
| `error` | 5 | `string` | Human-readable; only populated on the swallow path (non-abort exceptions). |

## Auth (admin / compliance; trusted-actor)

1. `_check_tenant(tenant_id)` — sharding owner check (`grpc_server.py:362`); aborts `FAILED_PRECONDITION` with `entdb-redirect-node` trailer if this node does not own the tenant.
2. Required-field validation BEFORE the auth check, in this order: `tenant_id`, `user_id` (`grpc_server.py:2873-2876`).
3. `_require_admin_or_owner(tenant_id, request.actor, ctx, "RevokeAllUserAccess")` — `grpc_server.py:2656-2690`:
   - Trusted actor comes from `auth_interceptor.get_authoritative_actor(...)` (the metadata-derived identity), **not** `request.actor`. The wire `actor` is decorative; clients claiming `system:admin` while authenticated as `user:eve` are downgraded to `user:eve`. Pinned by CLAUDE.md "All writes go through the WAL" + the trusted-actor invariant fix (commit `fece3fb`).
   - System actors (`system:*`) bypass; otherwise the actor's `tenant_member.role` must be `owner` or `admin`. Anything else → `PERMISSION_DENIED`.
4. No per-node ACL check — this is a tenant-wide bulk op, gated only by tenant-admin role.

The Go port MUST keep validation order identical so the contract
`INVALID_ARGUMENT` cases at `test_grpc_contract.py:593-595` and `test_admin_operations.py:781-797` keep passing.

## Side effects (many WAL events; mailbox cleanup; cross-tenant)

Python today (current behavior, pinned by tests):

1. `canonical_store.revoke_user_access(tenant_id, user_id, actor)` — `canonical_store.py:3871-3952`. Single `BEGIN IMMEDIATE` txn on the tenant SQLite:
   - `DELETE FROM node_access WHERE actor_id = ?` → `revoked_grants`.
   - `DELETE FROM group_users WHERE member_actor_id = ?` → `revoked_groups`.
   - `DELETE FROM node_visibility WHERE tenant_id = ? AND principal = ?` (visibility cleanup; not counted).
   - `append_audit(action="revoke_user_access", target=user_id, metadata={revoked_grants, revoked_groups})`.
2. Cross-tenant `shared_index` cleanup against `global_store` (`grpc_server.py:2891-2907`):
   - `get_shared_with_me(user_id, limit=10000)` then `remove_shared(user_id, source_tenant, node_id)` for each row whose `source_tenant == request.tenant_id`. Other-tenant rows are preserved (pinned: `test_admin_operations.py:776-779`).
   - Wrapped in `try/except` — global_store errors are logged at `WARN` and **do not** fail the RPC. `revoked_shared` is whatever was successfully removed before the exception.
3. Metric `record_grpc_request("RevokeAllUserAccess", "ok"|"error", elapsed)`.

**WAL invariant gap (Go port MUST fix).** Per CLAUDE.md §1, every mutation
must go through the WAL. Python today writes directly to SQLite via
`canonical_store.revoke_user_access` and only uses the WAL for the audit
trail. On a full WAL rebuild the revocation is **lost** — the deleted
`node_access` rows would be reapplied. The applier already has an
`admin_revoke_access` op (`apply/applier.py:1240-1245`) but it only handles
`node_visibility`, not `node_access` / `group_users`. The Go port should:

- Append a `TransactionEvent` containing one `admin_revoke_access` op with `{user_id, tenant_id}` and let the applier (extended) do the deletes.
- Mirror the `TransferUserContent` pattern at `grpc_server.py:2711-2715` ("WAL-first: append an admin_transfer_content event so the ownership change survives a full WAL rebuild").
- The applier extension is in scope for this ticket — broaden `admin_revoke_access` to also delete from `node_access` and `group_users` and return rowcounts so the handler can surface tallies.

The cross-tenant `shared_index` cleanup is on `global_store` (a different
SQLite); it has its own append-only journal and is best handled by emitting
a per-row `global_admin_remove_shared` event into the global WAL (or
keeping the current direct call until the global WAL is in scope —
document the gap).

## Error contract

| gRPC code | Trigger | Source |
|-----------|---------|--------|
| `OK` | Happy path. `success=true`. | `grpc_server.py:2909-2915` |
| `INVALID_ARGUMENT` | `tenant_id` empty / `user_id` empty / `actor` empty (resolves through `_require_admin_or_owner`). | `:2873-2876`, `:2675` |
| `PERMISSION_DENIED` | Caller is not tenant `owner`/`admin` and not a `system:*` actor. | `:2685-2689` |
| `FAILED_PRECONDITION` | Tenant not owned by this node — trailer `entdb-redirect-node` set. | `:362-411` |
| `OK` (with `success=false`, `error=…`) | Any other Python exception (e.g. SQLite `OperationalError`). The handler swallows and returns `RevokeAllUserAccessResponse(success=False, error=str(e))`. | `:2916-2921` |

The Go port should preserve the swallow behavior on non-RPC errors so
existing SDK callers (which check `resp.success` rather than the gRPC
status) keep working — see `sdk/go/entdb/admin.go:117-126` and
`sdk/python/entdb_sdk/_grpc_client.py:2500-2535`.

## Shared Go package deps

- `pb` (`server/go/internal/pb/entdbv1`) — generated request/response types.
- `auth` — `GetAuthoritativeActor(ctx) string` and the `system:*` predicate (mirrors `auth/auth_interceptor.py`).
- `tenantroles` — `GetMemberRole(ctx, tenantID, userID) (string, error)` (mirrors `_get_member_role`).
- `wal` — `Append(ctx, TransactionEvent) (Receipt, error)`. **Required** to fix the WAL invariant gap.
- `apply` — extended `admin_revoke_access` op handler; returns `(grants, groups int)` rowcounts via the applied-events table or a side-channel.
- `canonicalstore` — only via the applier; the handler MUST NOT write SQLite directly.
- `globalstore` — `GetSharedWithMe`, `RemoveShared` (mirrors `global_store.py:670-726`).
- `audit` — `Append(action="revoke_user_access", target_type="user", target_id, metadata)` (separate from the WAL; backed by S3 Object Lock per CLAUDE.md §2).
- `sharding` — for `_check_tenant`.
- `metrics` — `RecordGRPCRequest`.
- `errs` — common abort helpers.

NOT used: `acl`, `capability_registry`, `crypto`, `quota`, `schema`. ACL is
not consulted (admin override). Quota is not charged (admin op).

## Other-RPC deps

- **`RevokeAccess`** (`grpc_server.py:1828`, spec TBD): per-node revoke. `RevokeAllUserAccess` is the bulk-form generalization; it does NOT call `RevokeAccess` internally — it issues raw `DELETE` against `node_access`. The two diverge in audit metadata and in the per-node visibility recompute. Port `RevokeAccess` first; reuse only the SQL fragment, not the handler.
- **`RemoveTenantMember`** (`grpc_server.py:2492`): admins frequently call `RevokeAllUserAccess` *then* `RemoveTenantMember` to fully evict a user. The two are independent; share no state. Order matters because `_require_admin_or_owner` checks membership — call `RevokeAllUserAccess` first while the offender is still a member if you also want their member role.
- **`FreezeUser`** (`grpc_server.py` — `FreezeUserRequest` at proto `:1029`): GDPR-flavored "stop accepting writes from this user" without deleting access. Frequently chained with `RevokeAllUserAccess` for hard offboarding. Independent code path; port separately.
- **`DeleteUser`** (`grpc_server.py:2925`): GDPR right-to-erasure. `RevokeAllUserAccess` is **not** a substitute — `DeleteUser` queues data deletion after a grace period, while this RPC is access-only and immediate.

## Contract tests pinning behavior

- `tests/python/integration/test_grpc_contract.py:584-596` — happy path (`actor=ADMIN, tenant_id=TENANT, user_id="bob"` → `success=true`) and `INVALID_ARGUMENT` for empty `user_id`. Runs over a real gRPC channel. Go server must pass verbatim.
- `tests/python/unit/test_admin_operations.py:711-734` — `test_admin_can_revoke`: shares a node with BOB, adds BOB to a group, expects `revoked_grants=1, revoked_groups=1` after RPC. Pins the rowcount semantics.
- `tests/python/unit/test_admin_operations.py:735-751` — `test_non_admin_cannot_revoke`: non-member caller → `PERMISSION_DENIED`. Pins auth gate.
- `tests/python/unit/test_admin_operations.py:753-779` — `test_cleans_up_shared_index`: seeds 3 `shared_index` rows (2 in TENANT, 1 in `other-tenant`), expects `revoked_shared=2` and the `other-tenant` row preserved. Pins cross-tenant scoping.
- `tests/python/unit/test_admin_operations.py:781-797` — empty `user_id` → `INVALID_ARGUMENT` with `_AbortError`. Pins validation order.
- `tests/python/unit/test_admin_ops.py:223-264` — store-level: idempotent (second call returns 0), tenant-scoped, non-existent user returns 0 grants/groups. Pin for the applier extension.
- `tests/python/unit/test_admin_operations.py:347-415` — `revoke_user_access` audit-trail tests: `append_audit(action="revoke_user_access", ...)` is called; metadata contains tallies. Go port must emit the same audit row.
- `sdk/go/entdb/admin_test.go:367-388` — Go SDK contract: response fields `RevokedGrants/RevokedGroups/RevokedShared` round-trip via `RevokeAllResult`. The server's wire format is what this test fakes; do not change tags.

## Implementation outline (atomicity, partial failure)

```go
// server/go/internal/api/admin_revoke_all.go
func (s *EntDBServer) RevokeAllUserAccess(
    ctx context.Context, req *pb.RevokeAllUserAccessRequest,
) (*pb.RevokeAllUserAccessResponse, error) {
    start := time.Now()
    defer func() { metrics.RecordGRPCRequest("RevokeAllUserAccess", outcome, time.Since(start)) }()
```

1. `s.checkTenant(ctx, req.TenantId)` — sharding redirect; abort `FAILED_PRECONDITION` with trailer.
2. Validate: `req.TenantId != ""`, `req.UserId != ""`. `INVALID_ARGUMENT` on empty.
3. `trustedActor, err := s.requireAdminOrOwner(ctx, req.TenantId, req.Actor, "RevokeAllUserAccess")`. **Ignore `req.Actor` for authorization** — only the metadata-derived identity counts.
4. **WAL append (single event, atomic on SQLite side):**
   ```go
   ev := wal.TransactionEvent{
       TenantID: req.TenantId,
       Actor:    trustedActor,
       IdempotencyKey: deriveKey(req),  // see open-question below
       Ops: []wal.Op{{Type: "admin_revoke_access", UserID: req.UserId}},
   }
   receipt, err := s.wal.Append(ctx, ev)
   ```
   On WAL failure → return `INTERNAL`, no partial state. The applier handles the deletes inside one `BEGIN IMMEDIATE` (already the pattern; extend it from `apply/applier.py:1240-1245` to also delete from `node_access` and `group_users` and return tallies).
5. `s.wal.WaitForApplied(ctx, receipt, timeout=5s)` so the response can quote rowcounts. If timeout, return `success=true, revoked_grants=0, revoked_groups=0, error="applied wait timed out"` — preserves Python's "best-effort tally" feel.
6. Read tallies from the applier's per-event result channel (or, if absent, query `node_access`/`group_users` row-deltas via the audit row).
7. **Cross-tenant cleanup** (best-effort, do NOT fail the RPC):
   - `entries := s.globalStore.GetSharedWithMe(ctx, req.UserId, 10000, 0)`.
   - For each `e` where `e.SourceTenant == req.TenantId`: `s.globalStore.RemoveShared(ctx, req.UserId, req.TenantId, e.NodeID)`. Increment `revokedShared` only on success.
   - Catch all errors → `log.Warn("shared_index cleanup failed on RevokeAllUserAccess", "err", err)`. Do not propagate.
8. `s.audit.Append(...)` — S3 Object Lock entry. (Python does this inside `revoke_user_access`; in Go put it after the WAL applies so the audit row reflects realized state.)
9. `outcome = "ok"`; return `&pb.RevokeAllUserAccessResponse{Success: true, RevokedGrants: g, RevokedGroups: gr, RevokedShared: rs}`.

**Atomicity boundaries:**
- Tenant-SQLite deletes: atomic (single applier txn).
- Cross-tenant `shared_index` deletes: NOT atomic with the tenant deletes — they live in a different SQLite (`global_store`). Partial failure leaves `shared_index` rows behind; an admin can re-run the RPC, which is idempotent.
- Idempotency: re-running the same WAL event (same `idempotency_key`) is a no-op via the `applied_events` dedupe in the applier. `revoked_grants/groups` will be `0` on retry — matches Python.

**Partial-failure modes:**
- WAL append succeeds, applier slow → `success=true` with zero tallies (caveat above).
- WAL append fails → no SQLite change, `INTERNAL` returned, retry-safe.
- `global_store` unavailable → tenant deletes still applied, `revoked_shared=0`, RPC succeeds with a `WARN` log.

## Open questions / risks

- **Legal hold blocks?** `SetLegalHold` (proto `:980-991`) flags a tenant as on-hold. Today `RevokeAllUserAccess` does NOT check the hold flag — an admin can revoke access even while the tenant is preserving evidence. Decide:
  - Option A (status quo): access revocation is independent of legal hold — only **deletion** is blocked. Document this clearly. Recommended.
  - Option B: block when held, return `FAILED_PRECONDITION`. Risk: blocks emergency offboarding during litigation, which is when you most need it. Reject.
  - Option C: still revoke, but emit an enhanced audit row with `legal_hold_active=true`. Cheap and forensics-friendly. Recommended add-on.
- **Idempotency key derivation.** Request has no `idempotency_key` field. Server must synthesize (`sha256(tenant_id|user_id|trusted_actor|day_bucket)`?) so retries dedupe but a same-day rerun by a different admin still produces a fresh event. Confirm with EPIC #407.
- **Tally fidelity vs. Python.** Python returns rowcounts from the synchronous `revoke_user_access` call. Go's WAL-first path makes this asynchronous. If an SDK consumer treats `revoked_grants > 0` as "definitely happened," fine; but `==0` is now ambiguous (zero matches vs. apply pending). Surface a `pending=true` boolean? Or always `WaitForApplied` with a generous timeout? Decide per #407.
- **Group fan-out.** Today the handler removes the user from `group_users` but does NOT iterate groups they were in to revoke node-level grants held *via group membership*. The visibility cleanup (`node_visibility WHERE principal=user`) catches this in materialized form, but if visibility was never recomputed for a stale node, the user could retain phantom access until the next ACL recompute. Confirm whether the Go port should explicitly recompute or trust the lazy path. Leaning toward an explicit recompute job triggered by the `admin_revoke_access` event.
- **Cross-tenant cleanup at scale.** Hard cap of `limit=10000` shared rows (`grpc_server.py:2895`). Users who were shared > 10k nodes won't be fully cleaned in one call. Page through, or raise the cap with a streaming variant. Track as follow-up.
- **No actor metadata in `revoked_shared` audit.** The current code does not append a `global_store` audit row for the per-row removals. Add one in the Go port — important for compliance reconstruction.
- **Race with concurrent `ShareNode`.** Between WAL append and applier execution a fresh share could land. Applier's `BEGIN IMMEDIATE` serializes within tenant scope, so the new share will either land before the revoke (and be deleted) or after (and survive). Document the "after" case so admins know to FreezeUser the user before RevokeAll if they need a hard barrier.
