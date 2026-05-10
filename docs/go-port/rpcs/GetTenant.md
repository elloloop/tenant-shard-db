# GetTenant — Go Port Spec

EPIC #407. Reference Python: `server/python/entdb_server/api/grpc_server.py:2362-2395`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:119` (rpc), `:880-883` (request), `:885-888` (response), `:853-863` (`TenantDetail`).

Request `GetTenantRequest`:
- `string actor = 1` — REQUIRED. Non-empty string check at handler boundary (`grpc_server.py:2376-2377`). Per project security policy (commit `fece3fb`), the handler MUST NOT trust the client-supplied actor for authorization; the trusted principal comes from the auth interceptor. The field is still validated for non-emptiness so the wire contract matches every other registry RPC.
- `string tenant_id = 2` — REQUIRED. Non-empty (`:2378-2379`).

Response `GetTenantResponse`:
- `bool found = 1` — `true` iff the tenant row exists in `global_store`'s `tenant_registry` table.
- `TenantDetail tenant = 2` — populated only when `found == true`. Fields: `tenant_id`, `name`, `status`, `created_at` (unix seconds), `region` (e.g. `"us-east-1"`).

The RPC is read-only and idempotent. There is no `RequestContext` field. Lookups hit a process-global SQLite (the registry), so the call is region-agnostic — confirmed by `tests/python/integration/test_region_pinning.py:97-106` (an EU-served instance can read a US-pinned tenant's metadata).

## Auth (member-only? admin? trusted-actor)

Effectively public-within-authenticated-callers. The Python handler performs NO permission check:
- It does NOT call `_check_tenant` (contrast `GetTenantQuota` at `grpc_server.py:3130`).
- It does NOT consult `_get_member_role` (`:2290-2296`).
- It does NOT distinguish member vs non-member; any caller who passes the empty-string gates gets the row.

This is intentional and pinned by tests: `tests/python/unit/test_tenant_registry.py:307-321` calls `GetTenant` with an arbitrary `user:alice` actor and expects the response without setting up any membership. The region-pinning test at `tests/python/integration/test_region_pinning.py:99-106` further pins that an actor (`user:bob`) reading another tenant's row (`t-us`) succeeds — i.e., cross-tenant metadata reads are allowed.

Trusted-actor handling for the Go port:
- Auth interceptor extracts the trusted principal from session/JWT/API-key metadata (CLAUDE.md invariant #3).
- `request.actor` is validated for emptiness ONLY. Do not derive identity from it; do not compare it against the trusted principal here (other handlers e.g. `GetTenantQuota` do that via `_check_tenant`, but `GetTenant` deliberately does not).
- Anonymous callers: rejected by the auth interceptor before reaching the handler. The handler itself does not care.

Open: see "Open questions" below — exposing `created_at`/`region` to non-members is arguably a small information leak; the Go port preserves parity by default.

## Side effects (read on global_store)

None — read-only.

- WAL: not appended (CLAUDE.md invariant #1 satisfied trivially; reads bypass the WAL).
- `global_store.get_tenant(tenant_id)` (`server/python/entdb_server/global_store.py:489-502`): single `SELECT * FROM tenant_registry WHERE tenant_id = ?` against the global SQLite, wrapped via `_run_sync` to keep the gRPC `aio` thread non-blocking (CLAUDE.md invariant #3).
- per-tenant `canonical_store`: untouched. Cross-tenant boundary (CLAUDE.md invariant #4) is not crossed because the registry lives in `global_store`'s own SQLite.
- quotas / rate limits: not enforced in the handler; rely on interceptor.
- metrics: emits `record_grpc_request("GetTenant", "ok"|"error", elapsed)` (`:2384`, `:2387`, `:2393`).

## Error contract

The Python handler swallows all unexpected exceptions and returns `GetTenantResponse(found=False)` with `OK` status (`:2392-2395`). Go MUST preserve this — the contract test at `tests/python/integration/test_grpc_contract.py:466-471` only requires `r.found is False` for the not-found case, and the unit test at `tests/python/unit/test_tenant_registry.py:323-334` does likewise. There is no negative test asserting `Internal`.

| grpc.Code | Trigger |
|---|---|
| `OK` (found=true) | `tenant_id` exists in registry. |
| `OK` (found=false) | `tenant_id` does not exist, OR any handler exception (caught + logged at error). |
| `INVALID_ARGUMENT` | `actor == ""` (`:2376-2377`); `tenant_id == ""` (`:2378-2379`). Pinned by `test_grpc_contract.py:472-476`. |
| `UNIMPLEMENTED` | `self.global_store is None` — registry not configured (`:2370-2374`). |
| `UNAUTHENTICATED` | Auth interceptor only (no creds / bad token). Not from handler. |
| `PERMISSION_DENIED` | Not emitted by this handler. (Membership is not checked.) |
| `UNAVAILABLE` / `FAILED_PRECONDITION` | Not emitted; `GetTenant` is registry-global so the region/redirect machinery does not fire. |
| `INTERNAL` | NOT returned. SQLite/global_store errors are caught at `:2392-2395` and degraded to `found=False`. |

The argument-validation paths use `context.abort(...)` which raises and yields the gRPC status to the client; the outer `try` re-catches via `except Exception` and would otherwise mask them — but `AbortError` propagates because it isn't subclassed from base `Exception` in the way that gets caught here (`grpc.aio` raises `AbortError` which subclasses `Exception`). In Python, the abort path actually does pass through the catch-all and may end as `found=False` rather than `INVALID_ARGUMENT`; this is a known wart preserved by the contract test (`test_grpc_contract.py:472-476` accepts either `INVALID_ARGUMENT` or a successful `found=False` per the test runner's "invalid_argument" mode). Go port: emit `INVALID_ARGUMENT` cleanly via `status.Error(codes.InvalidArgument, …)` and short-circuit before the recover block.

## Shared Go package deps

- `internal/globalstore` — Go port of `global_store.GlobalStore`: needs `GetTenant(ctx, tenantID string) (*Tenant, error)` returning `nil, nil` for not-found, mirroring `_sync_get_tenant` (`global_store.py:497-502`). Backed by the global SQLite file (CLAUDE.md "Per-tenant SQLite isolation" — `global_store` is the cross-tenant exception).
- `internal/pb` — generated proto types `pb.GetTenantRequest`, `pb.GetTenantResponse`, `pb.TenantDetail` from `proto/entdb/v1/entdb.proto`.
- `internal/conv` (or inline) — `tenantDictToProto` analogue of `_tenant_dict_to_proto` (`grpc_server.py:2270-2278`): plain field copy, default-empty/zero on missing keys.
- `internal/metrics` — `RecordGRPCRequest("GetTenant", "ok"|"error", duration)` mirroring `record_grpc_request`.
- `internal/logging` — error-level structured logger for the catch-all path (`:2394`).
- `google.golang.org/grpc/codes` + `status` — for `INVALID_ARGUMENT` and `UNIMPLEMENTED` returns.

## Other-RPC deps

None at runtime. `GetTenant` reads state populated by `CreateTenant` (`grpc_server.py:2310-2360`) and may be invalidated by `ArchiveTenant` (`:2397+`). The Go port can implement `GetTenant` standalone provided `internal/globalstore.GetTenant` is wired up. SDK callers consume the response shape (`sdk/python/entdb_sdk/_grpc_client.py` via `GetTenantResponse` / `TenantDetail`, see `tests/python/unit/test_tenant_registry.py:804-823`).

## Contract tests pinning behavior (file:line)

- `tests/python/integration/test_grpc_contract.py:460-465` — happy path: `actor=ALICE, tenant_id=TENANT`, mode `"happy"`, check `r.found is True and r.tenant.tenant_id == TENANT`.
- `tests/python/integration/test_grpc_contract.py:466-471` — not-found: `tenant_id="ghost"`, mode `"not_found"`, check `r.found is False`.
- `tests/python/integration/test_grpc_contract.py:472-476` — empty actor: mode `"invalid_argument"`. (Note wart re. exception swallowing under `:2392-2395`; see "Error contract".)
- `tests/python/unit/test_tenant_registry.py:307-321` — happy unit-level path; pins `name` and `tenant_id` round-trip.
- `tests/python/unit/test_tenant_registry.py:323-334` — `found=False` for missing tenant; no exception raised.
- `tests/python/unit/test_tenant_registry.py:804-823` — SDK-side: pins `region` field surfaces through `GetTenantResponse.tenant.region`.
- `tests/python/integration/test_region_pinning.py:97-106` — pins that the registry is region-global: any server can read any tenant's row.

No test pins the "missing global_store => UNIMPLEMENTED" path; preserve it as defensive code.

## Implementation outline

```go
func (s *Server) GetTenant(ctx context.Context, req *pb.GetTenantRequest) (*pb.GetTenantResponse, error)
```

1. `start := time.Now()`. Defer metrics emission. Default status label `"ok"`; switch to `"error"` only inside the recover/error path.
2. If `s.globalStore == nil`: return `status.Error(codes.Unimplemented, "Tenant registry not configured")` (`:2370-2374`). Emit metric `"error"`.
3. Validate inputs BEFORE any try/recover:
   - If `req.GetActor() == ""`: return `status.Error(codes.InvalidArgument, "actor is required")`. Emit metric `"error"`. (`:2376-2377`)
   - If `req.GetTenantId() == ""`: return `status.Error(codes.InvalidArgument, "tenant_id is required")`. Emit metric `"error"`. (`:2378-2379`)
4. Wrap the lookup body in a `defer recover()` so panics + unexpected errors degrade to `found=false` per Python parity (`:2392-2395`):
   ```go
   defer func() {
       if r := recover(); r != nil {
           log.Error("GetTenant panicked", "err", r)
           resp = &pb.GetTenantResponse{Found: false}; err = nil
           recordMetric("error")
       }
   }()
   ```
5. Call `tenant, lookupErr := s.globalStore.GetTenant(ctx, req.GetTenantId())` (`global_store.py:489-495`).
6. On `lookupErr != nil`: log at error level, return `&pb.GetTenantResponse{Found: false}, nil`, emit metric `"error"`. Do NOT propagate as `Internal` — Python parity (`:2392-2395`).
7. On `tenant == nil`: emit metric `"ok"`, return `&pb.GetTenantResponse{Found: false}, nil` (`:2383-2385`).
8. Otherwise build `TenantDetail`:
   ```go
   td := &pb.TenantDetail{
       TenantId:  tenant.TenantID,
       Name:      tenant.Name,
       Status:    tenant.Status,
       CreatedAt: tenant.CreatedAt,
       Region:    tenant.Region,
   }
   ```
   (mirrors `_tenant_dict_to_proto` `:2270-2278`; missing fields default to zero values).
9. Emit metric `"ok"`. Return `&pb.GetTenantResponse{Found: true, Tenant: td}, nil` (`:2387-2391`).
10. Do NOT call any `_check_tenant` equivalent. Do NOT consult membership tables. Do NOT inspect the trusted-actor binding for cross-tenant gating; this is intentional parity (see "Auth").

## Open questions / risks

- **Cross-tenant metadata leak**: `GetTenant` exposes `region`, `created_at`, and `status` of any tenant to any authenticated caller, regardless of membership. Likely a latent privacy issue — `region` reveals data-residency choices and `status` reveals lifecycle (active/archived). The region-pinning test (`test_region_pinning.py:97-106`) explicitly pins this behavior, so the Go port preserves it; flag as a follow-up for product/security review post-port. Possible future fix: restrict to members + admins, mirroring `GetTenantMembers`.
- **Argument-validation vs catch-all interaction**: in Python, `context.abort` raises `AbortError`, which is caught by the outer `except Exception` at `:2392`, masking the `INVALID_ARGUMENT` status with `found=False`. The Go port should emit `INVALID_ARGUMENT` cleanly (return BEFORE the defer/recover block), which is strictly stricter than Python and still passes the contract test at `test_grpc_contract.py:472-476` (the test runner accepts the gRPC status). Document this as a small parity improvement.
- **`UNIMPLEMENTED` for missing registry**: no test pins this. Could be silently dropped, but it is defensive against partial deployments — keep it.
- **`actor` field shape**: the Python tests pass `"user:alice"` strings. CLAUDE.md "Key Patterns" notes Actors should use `Actor.user("bob")` not raw strings — but `GetTenantRequest.actor` is on-the-wire `string`, and the handler's only check is non-emptiness. Go port: keep as `string`, no parsing.
- **`status` field stringly-typed**: `TenantDetail.status` is `string` ("active", "archived"). Consider adding an enum in a future proto revision; for the port, keep as-is.
- **No pagination / batching equivalent**: callers needing bulk tenant lookup use `ListUserTenants` / membership RPCs; do not introduce a `BatchGetTenant` here without a separate RPC.
- **Metric name**: Python uses `"GetTenant"`. Go must emit identical label so existing dashboards keep working.
