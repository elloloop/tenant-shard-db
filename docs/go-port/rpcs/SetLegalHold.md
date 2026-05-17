# RPC Port Spec — `entdb.v1.EntDBService/SetLegalHold`

> Implementation: `server/go/internal/api/set_legal_hold.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407 — Python → Go server port. Source of truth: Go handler at
`server/go/internal/api/set_legal_hold.go`.

## Wire contract (set vs clear; scope)

Proto: `proto/entdb/v1/entdb.proto:132` (rpc), `:982-992` (messages).

`LegalHoldRequest`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `actor` | 1 | `string` | Caller actor string. **Untrusted** — overwritten by `AuthInterceptor` trusted identity (see Auth). |
| `tenant_id` | 2 | `string` | Required. Empty → `INVALID_ARGUMENT` (`server/go/internal/api/set_legal_hold.go`). |
| `enabled` | 3 | `bool` | `true` = set hold, `false` = clear hold. There is no separate "Clear" RPC — the same RPC toggles. |

`LegalHoldResponse`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `success` | 1 | `bool` | True iff the tenant row was updated. False if tenant not found. |
| `status` | 2 | `string` | New `tenant_registry.status` literal: `"legal_hold"` or `"active"` (`server/go/internal/api/set_legal_hold.go`, pinned by `(legacy Python unit test, removed),624`). |
| `error` | 3 | `string` | Human-readable error on `success=false` (e.g. `"Tenant not found"`). |

**Scope.** The RPC mutates exactly one column: `tenant_registry.status` for one
`tenant_id`. Toggling between `"active"` ⇄ `"legal_hold"` only. It does NOT
touch the parallel `legal_holds` table (that table — `server/go/internal/globalstore/`
with `(tenant_id, held_by, reason, created_at)` — is currently a separate
ledger consumed by `audit/compliance.py` and is **not** populated by this
handler today). The Go port MUST preserve this minimal scope (issue #408
tracks merging the two).

## Auth (compliance officer; trusted-actor)

Method **is not** in `AuthInterceptor.UNAUTHENTICATED_METHODS`. The interceptor
populates the trusted identity ContextVar before the handler runs.

Authorization is delegated to `_require_admin_or_owner(tenant_id, actor, ctx,
"SetLegalHold")` (`server/go/internal/api/set_legal_hold.go`, helper at `:2656-2690`). It:

1. Calls `get_authoritative_actor(actor)` — payload `actor` is *replaced* by
   the interceptor's trusted identity. A client claiming
   `actor: "system:admin"` while authenticated as `user:eve` is downgraded to
   `user:eve` (`server/go/internal/api/set_legal_hold.go`). Pinned by
   `tests/python/integration/test_privilege_escalation.py` (whole file pattern).
2. Empty trusted actor → `INVALID_ARGUMENT "actor is required"` (`:2674-2675`).
3. `_is_admin_or_system(trusted_actor)` (system:* / __system__ / admin:*) → ok.
4. Otherwise looks up `tenant_members.role`; only `owner` or `admin` may
   proceed. Anything else (member / viewer / guest / non-member) →
   `PERMISSION_DENIED "SetLegalHold requires admin or owner role"`.
   Pinned by `test_admin_operations.py:626-638` (member rejected with
   `PERMISSION_DENIED`).

There is no dedicated "compliance officer" role today. If/when one lands
(EPIC #407 punt-list), it must be allowed alongside `admin`/`owner` here.
`global_store == None` short-circuits to `UNIMPLEMENTED "Tenant registry not
configured"` (`server/go/internal/api/set_legal_hold.go`) **before** the auth check — Go port
should preserve that ordering.

## Side effects (WAL; flag in canonical_store/global_store; downstream RPCs check hold)

Sequence (`server/go/internal/api/set_legal_hold.go`):

1. `global_store.set_legal_hold(tenant_id, enabled, trusted_actor)` →
   `_sync_set_tenant_status` UPDATEs `tenant_registry.status` to
   `"legal_hold"` or `"active"` (`server/go/internal/globalstore/`). Returns False if
   tenant row missing — handler returns `success=false, error="Tenant not
   found"` (`:2830-2832`). Note: no separate `legal_holds`-table insert here.
2. `canonical_store.append_audit(...)` (`:2839-2851`) writes one tenant-scoped
   audit row with `action="set_legal_hold"`, `target_type="tenant"`,
   `target_id=tenant_id`, `metadata={"enabled": bool, "status": new_status}`.
   **Best effort** — failure is logged at WARN and *does not* roll back the
   status flip (`:2852-2853`). The `tenant_registry` row is the authoritative
   record. Pinned by `test_admin_operations.py:640-656`.

**WAL invariant deviation (must address in Go port).** CLAUDE.md §1 says
"every mutation MUST be appended to the WAL". The Python handler does NOT
call `wal.append()` — the status mutation lives only in the global-store
SQLite. This means a global-store rebuild from WAL replay would lose the
hold. Tracked as a known gap; the Go port SHOULD add a
`TransactionEvent.ops` entry (e.g. `admin_set_legal_hold`) and apply via
`Applier.apply_event()`. See Open questions.

**Tamper-evidence.** The CLAUDE.md §2 audit trail belongs to the WAL +
S3 Object Lock (COMPLIANCE), not a separate hash-chained table. Once the
WAL gap above is closed, S3 Object Lock provides the immutable record of
the hold being set/cleared — do NOT add a parallel hash-chain audit table.

**Downstream enforcement.** The flag is consulted by
`_check_tenant_access(..., op_kind="delete")` at `server/go/internal/api/set_legal_hold.go`,
which aborts with `FAILED_PRECONDITION "Tenant '<id>' is on legal hold;
deletes are not allowed"`. `ExecuteAtomic` calls this with op-kind derived
per-operation (`server/go/internal/api/set_legal_hold.go`); a `delete_node` / `delete_edge`
under hold raises FAILED_PRECONDITION while `create_node` / read RPCs
succeed. Pinned by `test_admin_operations.py:664-703` and
`test_tenant_roles.py:380-432, 610-635`.

## Error contract

| Condition | Code | Body |
|---|---|---|
| `global_store` not configured | `UNIMPLEMENTED` | `"Tenant registry not configured"` |
| `tenant_id == ""` | `INVALID_ARGUMENT` | `"tenant_id is required"` (pinned `test_admin_operations.py:860-868`, `test_grpc_contract.py:579-583`) |
| Trusted actor empty | `INVALID_ARGUMENT` | `"actor is required"` |
| Caller is non-owner / non-admin tenant member | `PERMISSION_DENIED` | `"SetLegalHold requires admin or owner role"` |
| Tenant row missing | **OK** | `LegalHoldResponse{success=false, error="Tenant not found"}` (NOT a gRPC error — pinned by `_check_tenant_access` not being called here) |
| Unexpected exception | **OK** | `LegalHoldResponse{success=false, error=str(e)}` (`:2862`); existing `RpcError` is re-raised (`:2859`) |

`record_grpc_request("SetLegalHold", "ok"/"error", elapsed)` is called on
every exit (`:2831, 2855, 2858`). Go port must wire equivalent metric.

## Shared Go package deps

| Python | Go package (port) | Used by |
|---|---|---|
| `global_store.set_legal_hold`, `get_tenant`, `get_member_role` | `internal/store/global` | handler core |
| `canonical_store.append_audit` | `internal/store/canonical` | best-effort audit row |
| `auth.auth_interceptor.get_authoritative_actor`, `get_current_identity` | `internal/auth` | trusted-actor binding |
| `record_grpc_request` | `internal/metrics` | latency/status histogram |
| (future) `wal.Append` for `admin_set_legal_hold` op | `internal/wal` | close CLAUDE.md §1 gap |
| (future) `audit/compliance.export_audit_trail` | `internal/audit/compliance` | only on export, not in-handler |

`legal_holds` table (`server/go/internal/globalstore/`) and the
`set_legal_hold_record` / `is_under_legal_hold` / `get_legal_holds` /
`remove_legal_hold` family (`server/go/internal/globalstore/`,
(legacy Python unit test, removed in Phase 4D)) are NOT touched by this
handler today. They live in the same package but the Go port should keep
them as a separate concern (court-order ledger) until #408 unifies them.

## Other-RPC deps (DeleteUser, ArchiveTenant must check)

Status is checked **only** inside `_check_tenant_access`. RPCs that route
through it pick up legal-hold semantics for free:

- `ExecuteAtomic` — per-op `op_kind` is "delete" for `delete_node`/`delete_edge`
  (`server/go/internal/api/set_legal_hold.go`). Hold blocks. Pinned by
  `test_admin_operations.py:664-703`.
- `DeleteNode` / `DeleteEdge` standalone RPCs — same path.
- `ArchiveTenant` (`server/go/internal/api/set_legal_hold.go`) — does NOT call
  `_check_tenant_access`; it calls `set_tenant_status("archived")` directly.
  **Gap:** archiving over a legal_hold silently overwrites the status to
  `"archived"` and removes hold protection. Go port MUST add an explicit
  `if status == "legal_hold": FAILED_PRECONDITION` guard in `ArchiveTenant`
  before the status flip. (Issue: open.)
- `DeleteUser` (`server/go/internal/api/set_legal_hold.go`) — does NOT call
  `_check_tenant_access` (no tenant_id on the request — it's user-global,
  GDPR scope). The user can be queued for erasure even if every tenant
  they belong to is on legal_hold. **Gap.** Go port should iterate
  `global_store.get_user_tenants(user_id)` and refuse the queue if any
  tenant is `legal_hold` (or any has a row in `legal_holds` once #408 is
  done). See Open questions.
- `CancelUserDeletion`, `ProcessDeletions` — read-only / cleanup; no hold
  check today.

## Contract tests pinning behavior

| Test | File:line | Pins |
|---|---|---|
| owner enables hold, status=`legal_hold` | (legacy Python unit test, removed in Phase 4D) | happy-path response shape and side effect |
| disable returns to `active` | `:611-624` | toggle semantics |
| member → `PERMISSION_DENIED` | `:626-638` | auth |
| audit row written with `action="set_legal_hold"` | `:640-656` | side-effect (audit) |
| empty `tenant_id` → `INVALID_ARGUMENT` | `:860-868` | validation |
| `legal_hold` blocks `delete_node` via `ExecuteAtomic` | `:664-684` | downstream enforcement |
| `legal_hold` allows `create_node` | `:686-703` | downstream non-blocking |
| reads + creates allowed, deletes rejected | `(legacy Python unit test, removed), 610-635` | role-matrix interaction |
| GlobalStore.set_legal_hold idempotency, status flip | (legacy Python unit test, removed in Phase 4D) | store-level |
| gRPC contract sweep happy + invalid_argument | `tests/python/integration/test_grpc_contract.py:573-583` | wire-level |
| `legal_holds` ledger family (separate from this RPC) | (legacy Python unit test, removed in Phase 4D) | NOT this RPC, but proves the ledger exists |

## Implementation outline

```go
func (s *Server) SetLegalHold(ctx context.Context, req *pb.LegalHoldRequest) (*pb.LegalHoldResponse, error) {
    start := time.Now()
    defer func() { metrics.RecordGRPC("SetLegalHold", status, time.Since(start)) }()

    if s.globalStore == nil {
        return nil, status.Error(codes.Unimplemented, "Tenant registry not configured")
    }
    if req.TenantId == "" {
        return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
    }
    trusted, err := s.requireAdminOrOwner(ctx, req.TenantId, req.Actor, "SetLegalHold")
    if err != nil { return nil, err }

    newStatus := "active"
    if req.Enabled { newStatus = "legal_hold" }

    updated, err := s.globalStore.SetTenantStatus(ctx, req.TenantId, newStatus)
    if err != nil { return nil, status.Errorf(codes.Internal, "%v", err) }
    if !updated {
        return &pb.LegalHoldResponse{Success: false, Error: "Tenant not found"}, nil
    }

    // TODO(#407): WAL-first — append admin_set_legal_hold op so a global-store
    // rebuild reproduces the flag. Until then, parity with Python.
    meta, _ := json.Marshal(map[string]any{"enabled": req.Enabled, "status": newStatus})
    if err := s.canonicalStore.AppendAudit(ctx, audit.Entry{
        TenantID: req.TenantId, ActorID: trusted, Action: "set_legal_hold",
        TargetType: "tenant", TargetID: req.TenantId, Metadata: string(meta),
    }); err != nil {
        log.Warn().Err(err).Msg("audit append failed on SetLegalHold")
    }
    return &pb.LegalHoldResponse{Success: true, Status: newStatus}, nil
}
```

Match Python's exact error strings — they are observed by SDK callers and
by `test_grpc_contract.py`. Keep the audit-failure-is-warning semantics
identical.

## Open questions / risks

1. **WAL gap (CLAUDE.md §1).** Python writes the status flip directly to
   global-store SQLite. A WAL rebuild loses the hold. The Go port should
   close this with a new `TransactionEvent.ops` op
   (`admin_set_legal_hold`). Behavioral test: stop server, drop global
   SQLite, replay WAL, assert tenant is still on hold. Coordinate with #408.
2. **Two ledgers, one concept.** `tenant_registry.status="legal_hold"`
   (toggle, no reason) vs `legal_holds` table (multi-row, `held_by`,
   `reason`, used by `audit/compliance.export_audit_trail`). The RPC
   only touches the first. Decision needed: extend `LegalHoldRequest`
   with `held_by`/`reason` (proto change, breaking) or keep the ledger
   write in a follow-up RPC (`AddLegalHoldRecord`). Recommend: punt to
   a new RPC; keep this one wire-stable.
3. **GDPR DeleteUser interaction (HIGH).** `DeleteUser`
   (`server/go/internal/api/set_legal_hold.go`) queues an erasure with no tenant-status
   check. If `user:alice` belongs to a tenant on legal_hold and Alice
   invokes `DeleteUser`, the grace timer starts and (after grace)
   `ProcessDeletions` will run — destroying data the hold is supposed
   to preserve. **Resolution required before Go port ships:** in
   `DeleteUser`, look up the user's tenants and abort
   `FAILED_PRECONDITION "user belongs to tenant <id> on legal hold"`
   if any are held. Belt-and-suspenders: `ProcessDeletions` should
   re-check at execute time. No Python test pins this today — add one
   in the Go port (`tests/contract/`). This is the GDPR-vs-legal-hold
   precedence question; legal hold wins per
   `audit/compliance.py` semantics.
4. **ArchiveTenant over a hold (MEDIUM).** Same gap as above for
   `ArchiveTenant` — currently silently overrides the status. Add
   explicit `legal_hold` guard.
5. **Compliance-officer role.** Today only owner/admin/system can set
   the hold. If a `compliance` role is added to `tenant_members.role`,
   include it in `_require_admin_or_owner` allowlist.
6. **Idempotency.** Setting hold twice is a no-op at the SQL layer
   (`UPDATE ... SET status = 'legal_hold'` rowcount > 0 either way).
   The audit log will record both. Go port should preserve this — do
   not dedupe — auditors want to see repeated set actions.
