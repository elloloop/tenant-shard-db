# GetUserTenants — Go Port Spec

EPIC #407. Reference Python: `server/python/entdb_server/api/grpc_server.py:2572-2599`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:126` (rpc), `:930-933` (request), `:935-937` (response), `:902-907` (`TenantMemberInfo`).

Request `GetUserTenantsRequest`:
- `string actor = 1` — required (presence-checked at `grpc_server.py:2586-2587`). Field is informational only after commit `fece3fb` ("ignore client-claimed actor"); the trusted identity comes from the auth interceptor. Empty string => `INVALID_ARGUMENT`.
- `string user_id = 2` — required (`:2588-2589`). Bare id, NO `user:` prefix (CLAUDE.md: `user_id` "alice" vs `tenant_principal` "user:alice"). Empty => `INVALID_ARGUMENT`.

Response `GetUserTenantsResponse`:
- `repeated TenantMemberInfo memberships = 1` — one row per tenant the user belongs to. Each row carries `tenant_id`, `user_id`, `role`, `joined_at` (epoch seconds, int64). Order: `joined_at` ascending (`global_store.py:622-628` — `ORDER BY joined_at`). Empty list when the user has no memberships AND when the handler hits the catch-all error path (`:2599`).

Read-only, idempotent. No `RequestContext` field.

## Auth (self-only? admin?)

The Python handler does **no** authorization beyond presence checks. Notable:

- It does NOT call `_check_tenant` (no tenant scope to check — the cross-tenant view IS the point).
- It does NOT compare `request.user_id` against the trusted identity. Any authenticated caller can list memberships of any other user. This is a latent privilege boundary issue similar to the one fixed by `fece3fb` for actor-claiming.
- The `actor` field is presence-validated but its value is never consulted for an authz decision.

Recommended Go posture (flag as a behavior change, gate behind an env / config switch for the contract-test transition):

- Reject when no trusted identity (`PERMISSION_DENIED`, mirroring `ListTenants` at `:1562-1566`).
- Allow when trusted identity is admin (`__system__`, `system:*`, `admin:*`).
- Allow when `trusted == "user:" + req.user_id` (self-read).
- Otherwise `PERMISSION_DENIED`.
- Default for v1 of the Go port: keep Python parity (no extra check) so contract tests pass; document the gap and open a follow-up.

## Side effects (read on global_store; cross-tenant view)

- WAL: not appended. Read-only RPC; CLAUDE.md invariant #1 not engaged.
- canonical_store / per-tenant SQLite: untouched. CLAUDE.md invariant #4 (per-tenant isolation) is structurally satisfied — this RPC reads from the **global** store, which is the sanctioned cross-tenant surface.
- global_store: single read of `tenant_members` table — `SELECT * FROM tenant_members WHERE user_id = ? ORDER BY joined_at` (`global_store.py:622-628`), wrapped via `_run_sync` (`:620`). This produces the cross-tenant view by design.
- Sharding: handler does NOT filter by `_sharding.is_mine(tenant_id)`. In multi-node deployments the global_store is logically global, so memberships across shards are visible from any node. Go port matches.
- Quotas / rate limits: not enforced; rely on interceptor.
- Metrics: `record_grpc_request("GetUserTenants", "ok"|"error", elapsed)` (`:2594`, `:2597`).

## Error contract

| grpc.Code | Trigger |
|---|---|
| `OK` | Happy path; also returned with `memberships=[]` from the catch-all (`:2596-2599`). |
| `INVALID_ARGUMENT` | `actor == ""` (`:2586-2587`); `user_id == ""` (`:2588-2589`). |
| `UNIMPLEMENTED` | `self.global_store is None` (`:2580-2584`, message: "Tenant registry not configured"). |
| `UNAUTHENTICATED` | Auth interceptor only (no/invalid creds). Not from handler today. |
| `PERMISSION_DENIED` | Auth interceptor only today; Go port should add self-vs-admin gate (see Auth). |
| `INTERNAL` | NOT returned. Any exception from `global_store.get_user_tenants` is caught (`:2596-2599`), logged at error, and a `memberships=[]` `OK` response is sent. Go MUST preserve to keep the contract test green. |

The catch-all behavior is unusual but pinned: the contract test only checks for tenant presence on the happy path; on failure the empty-OK response is the documented degradation. Match it.

## Shared Go package deps

- `internal/globalstore` — Go port of `GlobalStore`; expose `GetUserTenants(ctx, userID string) ([]Membership, error)` returning rows ordered by `joined_at` (`global_store.py:614-628`). Underlying schema: `tenant_members(tenant_id, user_id, role, joined_at)`.
- `internal/pb` — generated bindings; reuse `*pb.TenantMemberInfo` from `GetTenantMembers` port.
- `internal/grpcutil` — shared helper `MemberDictToProto` / `MembershipToProto` mirroring `_member_dict_to_proto` (`grpc_server.py:2281-2288`); used by `GetTenantMembers`, `AddTenantMember`, `ChangeMemberRole`, `GetUserTenants`.
- `internal/auth` — `GetCurrentIdentity(ctx)` (analogue of `auth_interceptor.get_current_identity`) for the recommended authz gate.
- `internal/metrics` — `RecordGRPCRequest("GetUserTenants", status, dur)`.
- `internal/logging` — error-level on catch-all (`:2598`).

## Other-RPC deps (overlaps with ListTenants)

- **`ListTenants` overlap**: `ListTenants` also calls `global_store.get_user_tenants(user_id)` (`grpc_server.py:1585`) to filter visible tenants for a regular `user:*` caller. Both RPCs MUST resolve identically in the Go port — share one implementation in `internal/globalstore.GetUserTenants` and one mapping helper. Drift between them would make `ListTenants` show tenants that `GetUserTenants` doesn't return (or vice-versa) and break SDK assumptions in `sdk/go/entdb/admin.go:91-92`.
- **`GetTenantMembers`**: dual view (rows-for-one-tenant vs rows-for-one-user). Same `TenantMemberInfo` shape, same `_member_dict_to_proto` helper. Port together.
- **`gdpr_worker`**: `gdpr_worker.py:136` consumes `get_user_tenants` to compute the deletion fan-out. The Go port of GDPR depends on this method having identical semantics (joined_at ordering doesn't matter for set-based fan-out, but presence/absence does).
- **`DeleteUser` / `RevokeAllUserAccess`**: `grpc_server.py:3034` reads memberships during user-wide revocations; same dependency.
- SDK consumer: `sdk/go/entdb/client.go:841-847` (`grpcTransport.GetUserTenants`), `sdk/go/entdb/admin.go:89-92` (`Admin.GetUserTenants`). No semantic change — the Go server must produce bytes the existing Go SDK already accepts.

## Contract tests pinning behavior (file:line)

- `tests/python/integration/test_grpc_contract.py:500-505` — happy path: `actor=ALICE, user_id="alice"`, expect membership for `TENANT`. Go port must satisfy `any(m.tenant_id == TENANT for m in r.memberships)`.
- `tests/python/integration/test_grpc_contract.py:506-510` — `actor=""`, `user_id="alice"` => mode `invalid_argument`.
- `tests/python/unit/test_tenant_registry.py:557-582` — `TestGetUserTenantsHandler.test_get_user_tenants`: alice in two tenants `t1`, `t2`, both must come back. Pins multi-tenant return.
- `sdk/go/entdb/admin_test.go:233`, `:262-276` — `TestAdmin_GetUserTenants_HappyPath`: SDK consumer pins the `[]TenantMember` shape and round-trip through the transport.
- `sdk/go/entdb/grpc_transport_test.go:111-112`, `:376-379` — fake server signature and request/response wiring; pins the proto contract end-to-end.
- `sdk/go/entdb/client_test.go:174` — mock transport ensures the interface remains stable.

No test currently exercises the "user_id missing" branch from gRPC level (only `actor=""` is pinned). Add one in the Go port to lock invariant.

## Implementation outline

```go
func (s *Server) GetUserTenants(ctx context.Context, req *pb.GetUserTenantsRequest) (*pb.GetUserTenantsResponse, error)
```

1. `start := time.Now()`. Defer `metrics.RecordGRPCRequest("GetUserTenants", status, time.Since(start))` with `status` defaulting to `"ok"` and switched to `"error"` on the catch-all path.
2. Guard: `if s.globalStore == nil { return nil, status.Error(codes.Unimplemented, "Tenant registry not configured") }` (`:2580-2584`).
3. Validate: `req.GetActor() == ""` => `codes.InvalidArgument, "actor is required"`. `req.GetUserId() == ""` => `codes.InvalidArgument, "user_id is required"`. Order matters: `actor` first to match Python (`:2586-2589`).
4. (Recommended, default-off behind config) Authz gate via `auth.GetCurrentIdentity(ctx)`:
   - `nil` => `PermissionDenied`.
   - admin (`__system__`, `system:*`, `admin:*`) => allow.
   - `"user:"+req.UserId` (case-sensitive) => allow.
   - otherwise => `PermissionDenied`.
5. Wrap remainder in a closure with `defer recover()`. On panic OR any error from steps 6-7: log at error, set status `"error"`, return `&pb.GetUserTenantsResponse{Memberships: nil}, nil` — preserve Python's empty-OK degradation (`:2596-2599`). Never propagate `Internal`.
6. `rows, err := s.globalStore.GetUserTenants(ctx, req.UserId)`. Rows are pre-sorted by `joined_at ASC` inside the store (mirror `ORDER BY joined_at`).
7. Translate: `memberships := make([]*pb.TenantMemberInfo, 0, len(rows))` then `MembershipToProto(row)` for each — fields `TenantId`, `UserId`, `Role`, `JoinedAt`, missing values default to zero (matches `dict.get(..., "")` / `0` at `:2283-2287`).
8. Return `&pb.GetUserTenantsResponse{Memberships: memberships}, nil`.
9. Status accounting: `INVALID_ARGUMENT` and `UNIMPLEMENTED` paths emit `record_grpc_request(..., "error", ...)` in Python? — actually they short-circuit before the metric line (`:2594`), so no metric is emitted on those. Match Python: only emit metric on the success-or-degraded paths. (Optional: deviate and emit `"error"` on the validation path; document, but be aware existing dashboards count only post-validation traffic.)

## Open questions / risks

- **Privilege boundary**: today any authenticated caller can read any user's tenant set. This leaks tenant membership graphs. Confirm with EPIC owner whether the Go port should land the self-or-admin gate immediately (recommendation: yes, behind `ENTDB_STRICT_TENANT_REGISTRY_AUTHZ=1`, default-off in v1 to preserve contract tests, default-on in v2). Track in a follow-up issue.
- **Empty-OK degradation**: contract pins it, but SREs may misread `OK + memberships=[]` as "user has no tenants" when global_store is broken. Recommendation: keep the response shape, but emit a distinct metric label (e.g. `status="degraded"`) so dashboards can alarm. This is additive, doesn't break contract.
- **Sharding semantics**: `global_store` is logically singleton. In a multi-node deployment, every node must see the same membership view. If global_store is implemented per-node-with-replication, a stale read could return a tenant that this node can't actually serve. Out of scope for this RPC, but document as a precondition for `internal/globalstore`.
- **`actor` field is dead weight**: presence-checked but unused. Should the Go port deprecate it (proto reserved tag)? Not in this port — keeping it preserves wire compat with Python clients that fill it. Flag for a future proto cleanup.
- **`joined_at` units**: Python stores epoch seconds (int) in SQLite. Proto field is `int64`. Confirm the Go global_store port uses the same unit; mismatched ms/s would break `ListSharedWithMe`-style time filters elsewhere.
- **Test gap**: no test pins `user_id=""` rejection through the gRPC contract harness — only `actor=""` is covered. Add one when porting to lock the validation order.
- **Cross-language helper**: `MembershipToProto` will be reused by 4+ RPCs; place it in `internal/grpcutil` (or co-locate with the global_store package) and unit-test it once with table-driven cases — including the missing-field defaults.
