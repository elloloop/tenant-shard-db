# RPC Port Spec — `entdb.v1.EntDBService/CreateTenant`

> Implementation: `server/go/internal/api/create_tenant.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407 — Python → Go server port. Source of truth: Go handler at
`server/go/internal/api/create_tenant.go`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:118` (rpc), `:865-878` (messages).

`CreateTenantRequest`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `actor` | 1 | `string` | Wire-claimed actor. **UNTRUSTED** — see Auth. Required, non-empty (`server/go/internal/api/create_tenant.go`). |
| `tenant_id` | 2 | `string` | New tenant identifier. Required (`:2320-2321`). MUST also pass `_validate_filesystem_id` rules (`[A-Za-z0-9_-]+`) because it becomes a filename: `data_dir/tenant_<id>.db` (`server/go/internal/store/`). |
| `name` | 3 | `string` | Display name. Required, non-empty (`:2322-2323`). No charset constraint. |
| `region` | 4 | `string` | Optional region pin (e.g. `eu-west-1`). Empty → server's `served_region`, falling back to `"us-east-1"` for single-region back-compat (`:2331-2334`). |

`CreateTenantResponse`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `success` | 1 | `bool` | True on insert. False on uniqueness/quota errors (`:2347, :2354, :2360`). |
| `tenant` | 2 | `TenantDetail` | Populated only on success. Built by `_tenant_dict_to_proto(tenant)` from the `global_store.create_tenant` return dict. |
| `error` | 3 | `string` | Free-text error. Prefixed `"Tenant already exists: "` for UNIQUE collisions (`:2356`). |

Important: error reporting uses **response fields**, not `context.abort`, for the
business-logic errors (duplicate, generic exception). `INVALID_ARGUMENT` is the
only condition that uses `context.abort` (`:2319, :2321, :2323`).

## Auth (admin scope; trusted-actor)

- **Authentication required.** This RPC is NOT in the unauth bypass set. Caller
  must present a valid principal via `AuthInterceptor` (session, API key, or
  OAuth/JWT) — same as every other tenant-admin RPC.
- **Trusted-actor invariant (MANDATORY).** The wire `request.actor` is reduced
  to a server-trusted principal via `self._trusted_actor(request.actor)`
  (`server/go/internal/api/create_tenant.go`). The Go port MUST mirror this: ignore the wire
  `actor` for any decision and read the principal from the auth interceptor's
  per-RPC context. The fix that introduced this is the privilege-escalation
  remediation in commit `fece3fb` ("Fix privilege escalation: ignore client-
  claimed actor in gRPC handlers"). Pinned by `_trusted_actor` definition at
  `server/go/internal/api/create_tenant.go` and the per-handler comment at `:2325-2328`.
- **Admin scope.** Today, no explicit admin gate runs inside `CreateTenant` —
  any authenticated caller can create a tenant and is auto-recorded as `owner`.
  EPIC #407 owners SHOULD decide whether to add an explicit `is_admin` gate in
  the Go port (consistent with `_is_admin_or_system` at `:2053-2090`). Keeping
  parity with Python = no gate; see Open questions.
- **Owner assignment.** Trusted actor → `_actor_user_id` (`:2298-2302`, strips
  `user:` prefix) → `global_store.add_member(tenant_id, uid, role="owner")`.
  This binding cannot be forged because it derives from the trusted principal.

## Side effects

In-order narration of the Python handler:

1. Guard: `self.global_store` exists, else `abort(UNIMPLEMENTED, "Tenant
   registry not configured")` (`:2312-2316`).
2. Validate non-empty `actor`, `tenant_id`, `name`
   (`abort(INVALID_ARGUMENT, …)`).
3. Resolve trusted actor (`:2329`).
4. Resolve region: `request.region or self.served_region or "us-east-1"`
   (`:2334`).
5. `await self.global_store.create_tenant(tenant_id, name, region=region)` —
   `INSERT INTO tenant_registry (tenant_id, name, status='active', created_at,
   region)` on the **global** SQLite (`server/go/internal/globalstore/`). UNIQUE on
   `tenant_id` enforces idempotence.
6. `await self.canonical_store.initialize_tenant(tenant_id)` — opens
   `data_dir/tenant_<id>.db` with `create=True` and runs `_create_schema(conn)`
   (`server/go/internal/store/`). This **creates the SQLite file** and
   bootstraps the per-tenant tables (nodes, edges, applied_events, etc.).
7. `await self.global_store.add_member(tenant_id, creator_uid, role="owner")`
   (`server/go/internal/globalstore/`).
8. Metrics: `record_grpc_request("CreateTenant", "ok"|"error", elapsed)`.

**WAL-append parity (CRITICAL — see Open questions).** The Python handler does
**not** currently append to the WAL. This violates CLAUDE.md invariant #1 ("All
writes go through the WAL") for the tenant-registry plane and means a global-
store rebuild from the WAL would silently drop tenants. The Go port MUST treat
this as a deliberate design question (see #407): either (a) add a
`TenantCreated` op to `TransactionEvent.ops` and apply it via `Applier`, or (b)
explicitly document the global-store SQLite as a non-WAL-backed control plane.
Recommendation: (a). Pattern to follow: `admin_handlers.py:55-60`.

**Quota init.** No quota row is created today. The Go port should initialize
the per-tenant quota row (rate-limit bucket, storage cap) at this point if the
Go quota subsystem requires it; current Python lazy-creates on first use.

**Region pinning.** The `region` column is the data-plane gate enforced by
`_check_tenant` (`server/go/internal/api/create_tenant.go`): a request landing on a server with
`served_region != tenant.region` is rejected `FAILED_PRECONDITION`. Pinned by
`tests/python/integration/test_region_pinning.py:108-134`.

## Error contract

| gRPC code / response shape | Trigger |
|----------------------------|---------|
| `INVALID_ARGUMENT` (abort) | `actor`, `tenant_id`, or `name` empty (`:2319, :2321, :2323`). |
| `UNIMPLEMENTED` (abort) | `global_store` not configured (`:2313`). |
| `success=false, error="Tenant already exists: …"` | UNIQUE constraint on `tenant_registry.tenant_id` (`:2352-2357`). NOT `ALREADY_EXISTS` — Python returns `OK` + `success=false`. Go port MUST preserve this (contract test pins `success is False` and `"already exists" in error`, (legacy Python unit test, removed in Phase 4D)). |
| `success=false, error=<exc str>` | Any other exception (`:2358-2360`). |
| Filesystem errors from `initialize_tenant` | Today bubble as generic exceptions → `success=false`. Go port should map `EEXIST` (idempotent — see Implementation) and surface `INTERNAL` for `EROFS`/`ENOSPC`. |
| Quota | No quota check today. If Go adds one, return `success=false, error="quota exceeded: …"`; do NOT abort with `RESOURCE_EXHAUSTED` unless the contract is updated. |

The trusted-actor handling does **not** raise — it silently rebinds. Never
return `PERMISSION_DENIED` from this handler unless an admin gate is added.

## Shared Go package deps

Each lives under `server/go/internal/...` unless noted.

- `pb` (`server/go/internal/pb/entdbv1`) — generated `CreateTenantRequest`,
  `CreateTenantResponse`, `TenantDetail`. Required.
- `globalstore` — `CreateTenant(ctx, id, name, region) (*Tenant, error)`,
  `AddMember(ctx, tenantID, userID, role)` (mirrors `server/go/internal/globalstore/,558`).
  Required.
- `canonicalstore` — `InitializeTenant(ctx, tenantID) error` (mirrors
  `server/go/internal/store/`). Must be idempotent on retry. Required.
- `auth` — provides the trusted principal via `auth.PrincipalFromContext(ctx)`
  (Go equivalent of `_trusted_actor`). Required.
- `region` — package-level `ServedRegion string` (mirrors
  `server/go/internal/api/create_tenant.go`). Required.
- `metrics` — `RecordGRPCRequest(method, status, dur)`. Required.
- `errs` — sentinel `ErrTenantExists` returned by `globalstore.CreateTenant`
  on UNIQUE collision; handler maps to `success=false, error="Tenant already
  exists: …"`. Required.
- `wal` (recommended, see Open questions) — for emitting a `TenantCreated`
  event if invariant #1 is honored.

NOT used: `apply.Applier` (no per-tenant event yet), `acl`, `schema`,
`crypto`, `audit`, `quota` (today).

## Other-RPC deps

- **Downstream — `AddTenantMember`** (`server/go/internal/api/create_tenant.go`): uses the
  same `global_store.add_member` path. `CreateTenant` already invokes it
  internally for the creator-as-owner step, so the Go port of
  `AddTenantMember` should share the `globalstore.AddMember` primitive
  (extract a single `(tenantID, userID, role)` helper used by both). The
  duplicate-member error string (`"Member already exists in this tenant"`,
  `:2486`) is the parallel of CreateTenant's "Tenant already exists" — same
  pattern, do not diverge.
- **Downstream — `GetTenant`** (`server/go/internal/api/create_tenant.go-…`) reads what
  `CreateTenant` wrote; ports together for a coherent contract test surface.
- **Downstream — every data-plane RPC** depends on `_check_tenant`
  (`:400-415`) reading the `region` column populated here.
- **Independent of**: WAL append, schema registry, ACL — those tenants are
  hydrated lazily on first data-plane call.

## Contract tests pinning behavior

- (legacy Python unit test, removed in Phase 4D) — happy path:
  `success=True`, `tenant.tenant_id`, `tenant.name`, `tenant.status="active"`,
  `canonical_store.initialize_tenant` awaited once with the id, creator
  recorded as `owner` member. **The Go server must satisfy these assertions
  verbatim once stubs are swapped.**
- (legacy Python unit test, removed in Phase 4D) — duplicate tenant_id
  yields `success=False` with `"already exists"` in `error` (NOT a gRPC abort).
- `tests/python/integration/test_grpc_contract.py:477-487` — over a live
  channel: happy path with `actor=ADMIN, tenant_id="newtenant"` returns
  `success=True`; empty `tenant_id` triggers `INVALID_ARGUMENT`.
- `tests/python/integration/test_region_pinning.py:62-79` — `region` round-
  trips through `global_store` and into `CreateTenantResponse.tenant.region`.
- `tests/python/integration/test_region_pinning.py:81-106` — empty `region`
  defaults to the server's `served_region`; US server pins `us-east-1`, EU
  server pins `eu-west-1`.
- `tests/python/integration/test_region_pinning.py:108-134` — region pin then
  drives `_check_tenant` rejection with `FAILED_PRECONDITION` on the wrong
  server (downstream pin; ensures `CreateTenant` populated the column).
- `(legacy Python unit test, removed)-...` (`TestSDKCreateTenantRegion`)
  — SDK-level region propagation (informational; SDK port, not server).

## Implementation outline (Go handler)

```go
// server/go/internal/api/create_tenant.go
func (s *EntDBServer) CreateTenant(ctx context.Context, req *pb.CreateTenantRequest) (*pb.CreateTenantResponse, error) {
    start := time.Now()
    status := "ok"
    defer func() { metrics.RecordGRPCRequest("CreateTenant", status, time.Since(start)) }()
```

1. Guard `s.globalStore != nil`; else `status.Error(codes.Unimplemented, "Tenant registry not configured")`.
2. Validate `req.Actor`, `req.TenantId`, `req.Name` non-empty → `codes.InvalidArgument`.
3. Validate `req.TenantId` against `validateFilesystemID` (mirror
   `_validate_filesystem_id`) BEFORE creating any DB row, to prevent partial
   state where the registry row exists but the SQLite filename is illegal.
4. `principal := auth.PrincipalFromContext(ctx)` — trusted; ignore `req.Actor`.
5. `region := req.Region; if region == "" { region = s.servedRegion }; if region == "" { region = "us-east-1" }`.
6. Preflight duplicate lookup via `globalStore.GetTenant(ctx, req.TenantId)`.
   - Existing row → return `ALREADY_EXISTS` before appending a second event.
   - Lookup error → `INTERNAL`.
7. Append a global-scope WAL event keyed by `__global__` with
   `op="tenant_created"` carrying `tenant_id`, `name`, `region`,
   `owner_user_id`, and `created_at`.
8. Wait for the applier to materialize that offset. The global applier
   calls `globalstore.ApplyTenantCreated`, which inserts the registry row
   and owner membership in one SQLite transaction.
9. Build `TenantDetail` from the event payload via a single conversion
    helper (mirror `_tenant_dict_to_proto`). Return `Success: true`.

Filesystem side effects remain lazy and replay-safe: no per-tenant SQLite
file is created by `CreateTenant`; the canonical store opens/creates it on
first data-plane use.

## Open questions / risks (replay determinism with FS init)

- **Global WAL reconstruction.** *Closed in the Go port.* `CreateTenant`
  now emits `tenant_created` and the applier writes registry + owner rows
  through `globalstore.ApplyTenantCreated`, so replay against a fresh
  `global.db` reconstructs onboarding state.
- **Replay determinism for filesystem init.** If `initialize_tenant` is
  driven by the `Applier` on WAL replay across a fresh disk, the SQLite
  file MUST end up at the same path with the same schema version. Risk:
  if `_create_schema` ever depends on a non-deterministic input
  (e.g. server clock, schema-registry state), replays diverge.
  Mitigation: `_create_schema` is currently DDL-only and deterministic
  — keep it that way; gate any future dynamic content behind a schema
  version embedded in the WAL event.
- **`tenant_id` charset.** Python validates only at `_get_db_path` time,
  inside `initialize_tenant`. A bad id fails AFTER the registry INSERT,
  leaving an orphan row. Go port should validate up front (step 3 above).
- **Quota init.** Today none. If the Go port adds quota, decide whether
  failure to provision quota rolls back the tenant create or leaves it
  unprovisioned (handler currently has no rollback path).
- **Admin gate.** Should `CreateTenant` require `_is_admin_or_system`?
  Today it does not — any authenticated caller can create unlimited
  tenants. This is a multi-tenant abuse vector and should be tracked
  alongside the port (separate issue, not blocking).
- **Cross-region `region` value.** The handler trusts `req.Region`
  unconditionally. There is no allow-list (`AllowedRegions`). A caller
  on the US server can pin `region="apac-south-1"`, then no server can
  serve the tenant. Add a regions allow-list in the Go port.
- **`tenant.region` is immutable today** (no UpdateTenant RPC for region).
  Document this so future ports do not assume mutability.
