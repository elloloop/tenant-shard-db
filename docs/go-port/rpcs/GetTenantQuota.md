# GetTenantQuota — Go Port Spec

EPIC #407. Reference Python: `server/python/entdb_server/api/grpc_server.py:3106-3163`.
ADR: `docs/decisions/quotas.md` (2026-04-13, three-layer rate-limit model).

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:142` (rpc), `:1063-1068` (request), `:1070-1085` (response).

`GetTenantQuotaRequest`:
- `string actor = 1` — claimed caller; **MUST be ignored** for authz (commit `fece3fb`). Used only for legacy log lines.
- `string tenant_id = 2` — required; empty triggers `INVALID_ARGUMENT`.

`GetTenantQuotaResponse` exposes the union of all three rate-limit phases (additive, single response shape per ADR):
- `tenant_id` (1) — echoes request.
- **Phase 1 — monthly write quota (durable, calendar-month):**
  - `int64 max_writes_per_month = 2` — 0 = unlimited.
  - `int64 writes_used = 3` — current period counter from `tenant_usage.writes_count`.
  - `int64 period_start_ms = 4` — from `tenant_usage.period_start_ms` (UTC start-of-month at first increment).
  - `int64 period_end_ms = 5` — **computed locally** as `_next_calendar_month_start_ms()`; do NOT read from DB. This keeps dashboards correct for tenants with no writes this period (`grpc_server.py:3137-3141`).
- **Phase 2 — per-tenant token bucket (in-memory, ops):**
  - `int32 max_rps_sustained = 6`, `int32 max_rps_burst = 7`.
- **Phase 3 — per-user token bucket (in-memory, ops, keyed by `(tenant_id, actor)`):**
  - `int32 max_rps_per_user_sustained = 8`, `int32 max_rps_per_user_burst = 9`.
- `bool hard_enforce = 10` — false ⇒ overage logged-but-allowed; true ⇒ `RESOURCE_EXHAUSTED`.

Storage units: writes are op-count (a 10-op `ExecuteAtomic` = 10 writes). RPS values are ops/sec, not requests/sec. Storage bytes are NOT exposed via this RPC — only writes / ops / monthly counters per ADR §"Meter writes, not requests".

## Auth (member / admin; trusted-actor)

- Gate: `_require_admin_or_owner(tenant_id, request.actor, ctx, "GetTenantQuota")` (`grpc_server.py:3129-3131`, definition `:2656`). The Python helper resolves the **trusted** actor from interceptor metadata (session / JWT / API-key) and rejects callers who are not `admin` or `owner` on the tenant.
- `request.actor` from the wire is a **hint only** — privilege-escalation tests pin that a non-admin sending `actor="system:root"` is rejected (`tests/python/integration/test_privilege_escalation.py:368-378`).
- Members (non-admin/non-owner) ⇒ `PERMISSION_DENIED` (`tests/python/integration/test_grpc_contract.py:609-613`).
- Anonymous / unauthenticated ⇒ `UNAUTHENTICATED` from the auth interceptor before this handler runs.
- The Go port MUST extract the trusted actor from `context.Context` (set by the auth interceptor) and pass it to the admin/owner check; never trust the proto `actor` field for authz.

## Side effects

Read-only. Specifically:
- WAL: NOT appended (CLAUDE.md invariant #1 — reads bypass WAL).
- canonical_store: untouched (no per-tenant SQLite read).
- global_store reads only:
  - `get_quota_config(tenant_id)` ⇒ `tenant_quotas` row (`global_store.py:1193`). Returns `None` for unknown tenants.
  - `get_usage(tenant_id)` ⇒ `tenant_usage` row (`global_store.py:1227`). Returns a synthesized `{period_start_ms: _calendar_month_start_ms(), writes_count: 0}` for tenants with no row yet (`global_store.py:1244-1247`).
- Quota counters / rate-limit buckets: this RPC is **not metered** by `QuotaInterceptor` — `_ENFORCED_METHODS = {"ExecuteAtomic"}` (`auth/quota_interceptor.py:127`). It also does not warm the quota cache (the cache is populated lazily by the interceptor on writes).
- Cache freshness: dashboards see DB-fresh values, NOT cached values. The interceptor's TTL cache (`_CONFIG_TTL_S=30`, `_USAGE_TTL_S=10`, `auth/quota_interceptor.py:61-62`) is used for enforcement only; admin reads bypass it.

## Error contract

| gRPC status | Trigger | Source |
|---|---|---|
| `OK` | normal path; unknown tenant returns zero-valued response | `grpc_server.py:3144-3157`; `tests/python/unit/test_quota_phase2_phase3.py:518-531` |
| `INVALID_ARGUMENT` | `request.tenant_id == ""` | `grpc_server.py:3127-3128`; `tests/python/integration/test_grpc_contract.py:604-608` |
| `UNIMPLEMENTED` | server started without a `global_store` (quota registry not configured) | `grpc_server.py:3122-3126` |
| `PERMISSION_DENIED` | trusted actor lacks admin/owner on tenant; covers wire-claimed-admin escalation | `grpc_server.py:3129`; `tests/python/integration/test_privilege_escalation.py:374-378`, `test_grpc_contract.py:609-613` |
| `UNAUTHENTICATED` | no trusted identity (interceptor) | `auth/` interceptor, before handler |
| `INTERNAL` | unexpected exception (logged, re-raised) | `grpc_server.py:3158-3163` |

Unknown-tenant **does not** return `NOT_FOUND` — by ADR design it returns zeros so dashboards render an "unlimited" badge without a special-case branch.

## Shared Go package deps (quotas)

The Go port should land these under `server/go/`:
- `internal/quota/` — config & usage types mirroring Python rows:
  - `Config{TenantID, MaxWritesPerMonth, MaxRPSSustained, MaxRPSBurst, MaxRPSPerUserSustained, MaxRPSPerUserBurst, HardEnforce}`.
  - `Usage{TenantID, PeriodStartMS, WritesCount}`.
  - `Period.NextCalendarMonthStartMS()` and `Period.CalendarMonthStartMS()` (UTC) — see `auth/quota_interceptor.py:65-71`.
- `internal/globalstore/` — `GetQuotaConfig(ctx, tenantID) (*Config, error)`, `GetUsage(ctx, tenantID) (*Usage, error)` (must return zero-valued Usage with current month-start when no row exists, mirroring `global_store.py:1244-1247`).
- `internal/auth/` — `RequireAdminOrOwner(ctx, tenantID, rpcName) (trustedActor string, err error)` — single shared helper used by all admin RPCs. Source of trust: `auth.TrustedActorFromContext(ctx)` (set by the auth interceptor); proto `actor` is ignored.
- `internal/grpcerr/` — `InvalidArgument`, `PermissionDenied`, `Unimplemented`, `Internal` constructors with consistent log fields.
- `internal/quota/interceptor` (already needed for `ExecuteAtomic`): exposes `Period.NextCalendarMonthStartMS()` so this handler shares the exact computation.

## Other-RPC deps

- Every **write** RPC (currently `ExecuteAtomic` only — `auth/quota_interceptor.py:127`) consumes Phase-1 quota via the interceptor and a post-apply increment in the Applier. The Go applier MUST call the equivalent of `global_store.increment_usage(tenant_id, n_ops)` after a successful commit (CLAUDE.md invariant #1; ADR §"Post-apply increment, not pre-apply"). Failures are fire-and-forget (logged, not raised).
- Phase 2 / 3 token-bucket consumption also happens in the interceptor on writes; this RPC reports config but does not read live bucket state — the bucket is in-memory per server (`auth/quota_interceptor.py:135`) and intentionally not exposed.
- `SetTenantQuota` / admin write RPC (when added) MUST invalidate `QuotaInterceptor` cache for the tenant after writing (`invalidate(tenant_id)`, `auth/quota_interceptor.py:435`).
- Reads (`GetNode`, `QueryNodes`, etc.) do NOT consume quota — ADR §"Meter writes, not requests".

## Contract tests pinning behavior (file:line)

- `tests/python/unit/test_quota_phase2_phase3.py:481-514` — happy path: returns config + usage, all 9 numeric fields and `hard_enforce` populated; `period_start_ms == _calendar_month_start_ms()`, `period_end_ms == _next_calendar_month_start_ms()`.
- `tests/python/unit/test_quota_phase2_phase3.py:517-531` — unknown tenant: zero-valued response, `period_end_ms` still computed.
- `tests/python/unit/test_quota_phase2_phase3.py:534-549` — non-admin caller: handler propagates the admin-gate's `PermissionError`.
- `tests/python/unit/test_quota_phase2_phase3.py:552-568` — `period_end_ms` is strictly after `now` and strictly after `period_start_ms` (calendar-month math invariant).
- `tests/python/integration/test_privilege_escalation.py:368-378` — wire-claimed admin actor with non-admin trusted identity ⇒ `PERMISSION_DENIED`. Pins the "ignore client-claimed actor" rule for this RPC.
- `tests/python/integration/test_grpc_contract.py:597-613` — three-row matrix: happy (admin), `INVALID_ARGUMENT` (empty tenant_id), `PERMISSION_DENIED` (member actor).

The Go port MUST be wired into the cross-impl contract harness once `tests/contract/` lands so these same matrix rows run against the Go server.

## Implementation outline

```go
// server/go/internal/api/get_tenant_quota.go
func (s *EntDBServer) GetTenantQuota(
    ctx context.Context, req *entdbv1.GetTenantQuotaRequest,
) (*entdbv1.GetTenantQuotaResponse, error) {
    start := time.Now()
    defer func() { metrics.RecordGRPC("GetTenantQuota", start) }()

    if s.globalStore == nil {
        return nil, grpcerr.Unimplemented("Quota registry not configured")
    }
    if req.GetTenantId() == "" {
        return nil, grpcerr.InvalidArgument("tenant_id is required")
    }
    // Trusted actor only; req.Actor is ignored (privilege-escalation guard).
    if _, err := s.auth.RequireAdminOrOwner(ctx, req.GetTenantId(), "GetTenantQuota"); err != nil {
        return nil, err // PERMISSION_DENIED or UNAUTHENTICATED
    }

    cfg, err := s.globalStore.GetQuotaConfig(ctx, req.GetTenantId())
    if err != nil { return nil, grpcerr.Internal(err) }
    usage, err := s.globalStore.GetUsage(ctx, req.GetTenantId())
    if err != nil { return nil, grpcerr.Internal(err) }

    // period_end is computed locally — do NOT trust a stale DB row.
    periodEnd := quota.NextCalendarMonthStartMS()

    // cfg may be nil (unknown tenant) — emit zero-valued response.
    var c quota.Config
    if cfg != nil { c = *cfg }
    return &entdbv1.GetTenantQuotaResponse{
        TenantId:               req.GetTenantId(),
        MaxWritesPerMonth:      c.MaxWritesPerMonth,
        WritesUsed:             usage.WritesCount,
        PeriodStartMs:          usage.PeriodStartMS,
        PeriodEndMs:            periodEnd,
        MaxRpsSustained:        c.MaxRPSSustained,
        MaxRpsBurst:            c.MaxRPSBurst,
        MaxRpsPerUserSustained: c.MaxRPSPerUserSustained,
        MaxRpsPerUserBurst:     c.MaxRPSPerUserBurst,
        HardEnforce:            c.HardEnforce,
    }, nil
}
```

Notes:
- All `int64`/`int32` are emitted as 0 when source values are missing, matching Python `int(... or 0)` coalescing (`grpc_server.py:3146-3154`).
- `usage` from `GetUsage` MUST never be nil — the Go store synthesizes `{period_start_ms: CalendarMonthStartMS(), writes_count: 0}` for unknown tenants.
- Use the same logger / metric labels (`"GetTenantQuota"`, `"ok"|"error"`) for parity with Python dashboards.

## Open questions / risks

- **Counter freshness vs reservation.** Python uses post-apply increment + 10s usage TTL on the enforcement path. Admin reads here bypass the cache, so a dashboard can briefly show `writes_used < interceptor's view`. Acceptable for billing display; document the lag in console UI.
- **Eventual consistency between WAL and counter.** A crash between `Applier.commit` and `increment_usage` leaves the counter off by one event (ADR §Operational notes). The Go port should preserve the same trade-off — do NOT add a 2-phase commit; reconcilers (future) can replay the WAL to fix drift.
- **Calendar-month boundary.** If a request lands within seconds of the month rollover, `period_start_ms` (from row) and `period_end_ms` (computed now) can briefly straddle different months. Consider computing both from a single `time.Now()` snapshot to keep the response internally consistent — Python currently does not, and the contract test only checks `period_end_ms > period_start_ms`.
- **Storage / egress quotas.** Not in Phase 1; ADR §"What this makes easy" notes adding new metered dimensions follows the same pattern. Reserve future proto field numbers (≥11) before any new fields land — coordinate with proto reviewers.
- **Per-server bucket state not exposed.** Phase 2/3 token-bucket *current* fill levels live in-process (`auth/quota_interceptor.py:135`) and are intentionally not in the response. If the Go server fleet scales horizontally, dashboard "remaining burst" numbers can never be accurate without Redis (ADR rejected this for Phase 1). Document as known-limitation.
- **Trusted-actor source in Go.** Confirm the Go auth interceptor places the trusted actor under a typed context key (e.g. `auth.contextKey`) before this RPC is wired up — otherwise `RequireAdminOrOwner` cannot enforce the privilege-escalation contract.
- **`UNIMPLEMENTED` semantics.** Python returns `UNIMPLEMENTED` when `global_store` is missing. In Go, a server built without a global store is a misconfiguration, not a runtime condition — consider failing startup instead, and only keeping the `UNIMPLEMENTED` branch for a deliberately-disabled mode (feature flag).
