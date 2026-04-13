# Quota & rate-limit decisions

Frozen architectural decisions about rate limiting, quotas, and fair-share enforcement. Newest first.

---

## 2026-04-13: Three-layer rate limit model, implement Phase 1 (monthly quota) first

**Status:** frozen
**Decided:** 2026-04-13
**Tags:** quotas, rate-limits, billing, noisy-neighbor, sdk
**Supersedes:** none
**Superseded by:** none

### Decision

EntDB enforces rate limits in **three additive layers**, implemented in phases so each layer is independently shippable and reuses the same infrastructure:

1. **Phase 1 — Per-tenant monthly quota** (billing enforcement). Durable counters in `global_store`, updated post-apply, checked with a cached lookup. Soft by default, hard opt-in per plan.
2. **Phase 2 — Per-tenant token bucket** (noisy-neighbor / QoS). In-process per-server, additive extension of Phase 1's config and interceptor.
3. **Phase 3 — Per-user token bucket** (credential abuse protection). Reuses Phase 2's bucket machinery keyed by `(tenant_id, actor_id)`.

**This decision freezes Phase 1 and commits to the three-phase plan.**

**Phase 1 surface:**

- `TenantQuotaConfig` proto message stored in `global_store.tenant_quotas`, fields: `tenant_id`, `max_writes_per_month`, `hard_enforce`. Missing / zero values mean "unlimited".
- `TenantUsage` proto message stored in `global_store.tenant_usage`, fields: `tenant_id`, `period_start_ms`, `writes_count`. Calendar-month reset.
- `QuotaInterceptor` runs in the gRPC server after `AuthInterceptor`. On `ExecuteAtomic` requests, checks `writes_count + len(event.ops) > max_writes_per_month` and returns `RESOURCE_EXHAUSTED` + `Retry-After` when `hard_enforce=true`.
- Applier calls `global_store.increment_usage(tenant_id, n_ops)` after a successful `ExecuteAtomic` commit (fire-and-forget, errors logged but not raised).
- `GetTenantQuota(tenant_id)` RPC returns the current config + usage snapshot for dashboards.
- SDK surfaces `RateLimitError` with `retry_after_ms` parsed from trailing metadata. Both Python and Go SDKs gain a typed error.
- Calendar-month reset: on first request of a new period, `reset_period(tenant_id)` zeroes the counter and advances `period_start_ms`.

**Design rules (all phases):**

1. **Meter writes, not requests.** Reads are not counted. A batch of 10 ops = 10 writes (prevents batch gaming).
2. **Post-apply increment, not pre-apply.** The counter records work that actually succeeded. Slight lag (up to one request) is acceptable for billing.
3. **Soft default, hard opt-in.** Most tenants hit quota as a warning signal, not a hard stop. Production-tier plans with strict contracts can opt into `hard_enforce=true`.
4. **RESOURCE_EXHAUSTED + Retry-After.** Same gRPC status and metadata shape across all three phases. SDKs write one handler that works for quota limits, token-bucket throttle, and per-user abuse — they differ only in the `Retry-After` value.
5. **Shared `TenantQuotaConfig` struct grows over time.** Phase 2 adds `max_rps_sustained`, `max_rps_burst`. Phase 3 adds `max_rps_per_user_sustained`, `max_rps_per_user_burst`. All additive — no wire-breaking changes.

### Context

Multi-tenant SaaS needs rate limiting for three independent reasons: billing enforcement (plan contracts), noisy-neighbor protection (one tenant can't starve the box), and abuse protection (a compromised key shouldn't DOS the service). These are often conflated but have different storage, consistency, and enforcement point requirements.

The three-layer model separates them cleanly:
- Layer 1 (monthly) lives in durable storage, billing-accurate, tolerates lag.
- Layer 2 (burst RPS) lives in memory, hot-path, fair-share.
- Layer 3 (per-user) reuses Layer 2 machinery with a different key.

Implementing Layer 1 first gives us the billing story and proves out the shared infrastructure (config, cache, interceptor, SDK error) that Layers 2 and 3 inherit without rework.

### Alternatives considered

- **Single unified layer with one algorithm.** Rejected. Monthly billing and hot-path RPS have fundamentally different consistency models (durable vs in-memory, lag-tolerant vs synchronous). Forcing them into one mechanism makes both worse.
- **Start with Phase 2 (token bucket) first.** Rejected. Without billing enforcement, free-tier tenants can consume unlimited resources. Billing is the revenue-critical layer and should ship first.
- **Redis-backed token buckets from day one.** Rejected as premature. In-process per-server is simple, has no external dependencies, and handles the common case. Redis is a future upgrade when the fleet grows beyond a handful of servers.
- **Meter requests instead of writes.** Rejected. Read volume is typically 10-100x write volume on SaaS workloads. Metering reads either makes the quota meaningless (must be 100x larger) or punishes legitimate read-heavy usage.
- **Count whole `ExecuteAtomic` calls as one write.** Rejected. A batch of 1000 operations in one call would dodge the limit trivially. Must count ops.
- **Rolling 30-day window** (vs calendar month). Rejected for Phase 1 because it complicates the reset logic and doesn't match billing cycles. Can be added as a `counting_strategy` field later.
- **Pre-apply increment** (before the Applier commits). Rejected. Would double-count on failed writes, and the counter would drift from actual successful work. Post-apply is accurate.

### Consequences

**What this locks in:**

- `global_store.tenant_quotas` and `global_store.tenant_usage` tables become part of the global schema. Backfill for existing tenants defaults to `max_writes_per_month=0` (unlimited) to avoid breaking production tenants at rollout.
- The gRPC server gains a `QuotaInterceptor` that runs after `AuthInterceptor`. Order matters — auth must succeed before quota is checked because quotas are per-tenant.
- The Applier has a post-apply hook that increments the usage counter. This is fire-and-forget with error logging — quota failures cannot block apply. If the increment fails (e.g. global_store is down), writes still succeed and the counter drifts briefly. This is acceptable for billing.
- `RESOURCE_EXHAUSTED` + `Retry-After` is the canonical rate-limit response shape for the entire platform, regardless of which layer rejected the request.
- Calendar-month periods mean some tenants may get a brief window at the start of the month where they "reset" to zero and appear to have unlimited headroom. This is intentional — matches billing cycle expectations.
- Phase 2 and Phase 3 reuse `TenantQuotaConfig` (additive fields), `QuotaInterceptor` (new branches), and `RateLimitError` (same shape). No refactor needed when those ship.

**What this makes easy:**

- Tiered pricing is a one-row change in `tenant_quotas`.
- Dashboards query a single RPC (`GetTenantQuota`) for the entire quota view.
- Monitoring: usage / limit ratio is exported as a standard Prometheus metric (`entdb_tenant_quota_usage_ratio{tenant_id=}`).
- Adding new metered dimensions (storage, egress, API key count) follows the same pattern.

**What this makes harder:**

- Tenants on very high plans whose usage never reaches the limit still pay the quota-check cost on every request. Mitigated by the 10-30s cache TTL — the check is an in-memory lookup on most requests, not a DB hit.
- The calendar-month reset means usage reports near month boundaries can be confusing. Mitigated by also exposing `period_start_ms` in the response.
- Tenants with `max_writes_per_month=0` (unlimited) still hit the interceptor code path. The check is a single comparison — negligible cost.

**Operational notes:**

- On server startup, the quota cache is cold. First requests for each tenant do a DB lookup. Expected; acceptable.
- If `global_store` is temporarily unreachable and the cache is stale, the interceptor fails-open (allows the request with a warning log). Better to accept slight over-quota than to stop writes when the quota storage flaps.
- The counter increment happens after the apply commit. On a crash between commit and increment, the counter is off by one event. Not worth fixing.

### References

- Conversation: 2026-04-13 architecture discussion on rate limits, quota layers, and the phase plan
- Related decisions:
  - [storage.md — 2026-04-13 Immutable storage mode](storage.md#2026-04-13-immutable-storage-mode-no-built-in-drafts-primitive) — `global_store` is the durable metadata layer
  - [acl.md — 2026-04-13 Cross-tenant ACL](acl.md#2026-04-13-cross-tenant-acl-via-tenantid-grantee-and-public-write-role) — billing rule (creator-pays) aligns with per-tenant quota
- Implementation: this commit (Phase 1). Phases 2 and 3 are follow-up PRs.
