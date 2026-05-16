# RPC Port Spec — `entdb.v1.EntDBService/Health`

> Implementation: `server/go/internal/api/health.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407 — Python → Go server port. Source of truth: Go handler at
`server/go/internal/api/health.go`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:76` (rpc), `:582-588` (messages).

`HealthRequest` — empty message. No fields. No `RequestContext`, no actor,
no tenant_id. The probe is server-global, not per-tenant.

`HealthResponse`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `healthy` | 1 | `bool` | True iff `wal == "healthy"` AND `storage == "healthy"` (other component keys are informational, do NOT gate `healthy`). |
| `version` | 2 | `string` | Server semver. Python returns hard-coded `"1.0.0"` (see `server/go/internal/api/health.go`). Go port should read from build-stamped var (`-ldflags -X main.version=...`); contract test only requires non-empty (`test_grpc_contract.py:201`). |
| `components` | 3 | `map<string,string>` | Always contains `wal` and `storage`. Adds `node_id` and `assigned_tenants` only when sharding is multi-node. |

## Auth

- **No authentication required.** `/entdb.EntDBService/Health` is in
  `AuthInterceptor.UNAUTHENTICATED_METHODS` (`auth/auth_interceptor.py:157-162`).
  Pinned by (legacy Python unit test, removed in Phase 4D).
- **No rate-limit.** Bypassed by `RateLimitInterceptor`
  ((legacy Python unit test, removed in Phase 4D)).
- **No `Permission` check.** Handler does not consult `acl` or
  `capability_registry`.
- **Trusted-actor invariant: N/A.** Request carries no actor field, so the
  CLAUDE.md "wire `request.actor` is UNTRUSTED" rule has nothing to ignore here.
  Go port must NOT add any `request.context` parsing — there is no `context`.

## Side effects

**None.** Strictly read-only, in-memory introspection. In-order narration:

1. `start := time.Now()` for metrics timing.
2. Probe `wal.IsConnected()` (or treat as healthy if backend lacks the method —
   matches Python `hasattr(self.wal, "is_connected")` shim at
   `server/go/internal/api/health.go`). Catch any error → `components["wal"] = "unknown"`.
3. Set `components["storage"] = "healthy"` unconditionally. (Python does not
   actually probe SQLite; Go port should preserve this — adding a real probe
   would be a behavior change; track as future work.)
4. If `sharding != nil && sharding.IsMultiNode()`:
   - `components["node_id"] = sharding.NodeID`
   - `components["assigned_tenants"] = strings.Join(sortedSlice(sharding.AssignedTenants), ",")`
5. Compute `healthy = components["wal"]=="healthy" && components["storage"]=="healthy"`
   (the `node_id`/`assigned_tenants` keys MUST NOT count against health — this
   is the regression pinned by `test_cron_fixes.py:116-147`).
6. Record metric `record_grpc_request("Health", "ok"|"error", elapsed)`.
7. Return `HealthResponse{healthy, version, components}`.

No WAL append, no `canonical_store` write, no `global_store` read, no quota
charge, no schema lookup.

## Error contract

| gRPC code | Trigger |
|-----------|---------|
| `OK` | Always, in practice. Even when `wal.IsConnected()` panics, Python catches and returns `OK` with `components["wal"]="unknown"` (`server/go/internal/api/health.go`). |
| `INTERNAL` | Only if a non-recoverable panic escapes the inner `try` (Python re-raises at `server/go/internal/api/health.go`). Go port: `defer recover() → status.Errorf(codes.Internal, ...)` and increment `record_grpc_request("Health","error",…)`. |
| `UNAVAILABLE` | Server not started — surfaced by transport, never by the handler. |

The handler MUST NOT abort with `UNAUTHENTICATED`, `PERMISSION_DENIED`, or
`INVALID_ARGUMENT`. `HealthRequest` is empty so there is nothing to validate.

## Shared Go package deps

Each is a new package under `server/go/internal/...` unless noted.

- `pb` (`server/go/internal/pb/entdbv1`) — generated `HealthRequest`, `HealthResponse`, servicer interface. Required.
- `wal` — `IsConnected() bool` interface (mirrors `wal/base.py:337`). Optional method; absence means "assume healthy". Used in step 2.
- `sharding` — `Config{NodeID string; AssignedTenants []string; IsMultiNode() bool}` (mirrors `sharding.py`). Optional; nil-safe.
- `metrics` — `RecordGRPCRequest(method, status string, dur time.Duration)` (mirrors `metrics.py:103`). Required.
- `version` — package-level `Version string` set via `-ldflags`. Required.

NOT used by this RPC and MUST NOT be imported: `auth`, `acl`, `apply`,
`canonicalstore`, `globalstore`, `schema`, `errs`, `quota`, `crypto`, `audit`.
Importing any of them is a code-smell signal that the handler is doing too much.

## Other-RPC deps

None. Health is a leaf RPC. Independent of every other RPC; can be ported in
isolation as the first ticket of the Go port.

Note: this is a **separate** RPC from `grpc.health.v1.Health/Check`, the
standard probe protocol used by Docker `HEALTHCHECK` and k8s. The Go port
must register both:
- `entdb.v1.EntDBService/Health` (this spec)
- `grpc.health.v1.Health` (use `google.golang.org/grpc/health` package + `health.NewServer()`, set status `SERVING` for `""` — mirrors `server/go/internal/api/health.go`, pinned by (legacy Python unit test, removed in Phase 4D)).

## Contract tests pinning behavior

- `tests/python/integration/test_grpc_contract.py:195-202` — happy-path: empty request, `healthy=true`, non-empty `version`. Runs over a real gRPC channel. **The Go server must pass this test verbatim once the Python stubs are swapped for the Go binary.**
- (legacy Python unit test, removed in Phase 4D) — multi-node Health stays healthy: `node_id` / `assigned_tenants` info-keys don't drag `healthy` to false. Regression guard.
- (legacy Python unit test, removed in Phase 4D) — auth interceptor lets `/entdb.EntDBService/Health` through with no metadata.
- (legacy Python unit test, removed in Phase 4D) — rate limiter bypasses Health even when tenant bucket is empty.
- (legacy Python unit test, removed in Phase 4D) — standard `grpc.health.v1.Health/Check` is wired and returns `SERVING`.
- (legacy Python unit test, removed in Phase 4D), `test_jwt_auth.py:360` — additional bypass pins for the JWT auth path.
- `tests/python/e2e/test_full_flow.py:404-411` — HTTP `/health` returns `{"status":"healthy"}` (separate gateway, out-of-scope for this RPC).

## Implementation outline (Go handler)

```go
// server/go/internal/api/health.go
func (s *EntDBServer) Health(ctx context.Context, _ *pb.HealthRequest) (*pb.HealthResponse, error) {
    start := time.Now()
    defer func() {
        status := "ok"
        if r := recover(); r != nil { status = "error"; panic(r) }
        metrics.RecordGRPCRequest("Health", status, time.Since(start))
    }()
```

1. Initialize `components := map[string]string{}`.
2. Probe WAL: if `s.wal` implements `interface{ IsConnected() bool }`, call it inside a `func() { defer recover() ... }()` and set `components["wal"]` to `"healthy"`, `"unhealthy"`, or `"unknown"`. Otherwise `"healthy"`.
3. `components["storage"] = "healthy"` (unconditional; matches Python).
4. If `s.sharding != nil && s.sharding.IsMultiNode()`:
   - `components["node_id"] = s.sharding.NodeID`
   - sort `s.sharding.AssignedTenants` ascending, join with `","`, assign to `components["assigned_tenants"]`.
5. `healthy := components["wal"] == "healthy" && components["storage"] == "healthy"`.
6. Return `&pb.HealthResponse{Healthy: healthy, Version: version.Version, Components: components}, nil`.
7. Wire `/entdb.EntDBService/Health` into the unauth-bypass set in the Go auth interceptor and rate-limit interceptor (separate ticket — track in `docs/go-port/shared/auth.md` once written).

## Open questions / risks

- **Hard-coded `version="1.0.0"`** in Python is a known wart. Go port should
  fix by reading from `runtime/debug.ReadBuildInfo()` or `-ldflags -X`. Confirm
  with EPIC #407 owner that this is desired, not a contract break.
- **`storage="healthy"` is a lie** — Python never opens an SQLite cursor.
  Decide whether the Go port should add a real probe (`PRAGMA quick_check`)
  per assigned tenant. Risk: a real probe on N tenants could time out
  health checks under load. Recommended: keep Python behavior, file a
  follow-up ticket for a real probe behind a flag.
- **WAL `IsConnected()` semantics differ across backends** (`memory`, `kafka`,
  `kinesis`, `sqs`, `pubsub`, `eventhubs`, `servicebus`). Some are pure
  in-memory flags, some return cached state without a live ping. Go port must
  preserve the cheap, non-blocking contract — never round-trip to the broker
  inside Health. Otherwise the Docker HEALTHCHECK can starve.
- **Concurrency**: Python builds a fresh `dict` each call, no shared state.
  Go port must do the same — do not memoize `components` across calls,
  because `assigned_tenants` can change at runtime via SIGHUP.
- **Metrics label cardinality** is fixed (`Health` × {`ok`,`error`}); no risk.
