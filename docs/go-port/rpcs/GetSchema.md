# GetSchema — Go Port Spec

> Implementation: `server/go/internal/api/get_schema.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407. Reference Python: `server/go/internal/api/get_schema.go`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:79` (rpc), `:590-596` (request), `:598-607` (response).

Request `GetSchemaRequest`:
- `int32 type_id = 1` — optional. When non-zero, response is filtered to that node type plus edge types touching it.
- `string tenant_id = 2` — optional. Used only as the data-driven fallback key when the registry is empty.

Response `GetSchemaResponse`:
- field 1 RESERVED (`schema_json`); do not reuse.
- `string fingerprint = 2` — `sha256:<hex>` from the in-memory registry, or `""` when registry is unfrozen / empty.
- `google.protobuf.Struct schema = 3` — `{node_types: [...], edge_types: [...]}`. Empty registry + empty tenant_id => zero-valued Struct.

The RPC is read-only and idempotent. There is no `RequestContext` field (unlike most other RPCs); auth still applies via interceptor metadata.

## Auth (Permission, trusted-actor handling)

The Python handler does NOT call `_check_tenant`, has no Permission check, and never reads `request.actor` (no such field). All access control comes from the auth interceptor:

- Auth interceptor extracts trusted actor from session/JWT/API-key in metadata. See CLAUDE.md invariant #3 + `auth/` package.
- Anonymous callers: Python currently allows them (no explicit gate). Go port SHOULD preserve this — `GetSchema` is treated as effectively public metadata, since the registry is process-global and contains no tenant data.
- The optional `tenant_id` is used only to read distinct type IDs from that tenant's SQLite. The Go port MUST NOT trust `request.tenant_id` for authorization decisions; it is a hint that gates a read against the caller's already-authorized scope. If the auth interceptor produced a tenant binding that disagrees with `request.tenant_id`, follow project policy (cross-tenant read => permission_denied).
- No `request.actor` field exists in `GetSchemaRequest`; the "ignore client-claimed actor" rule (commit `fece3fb`) is structurally satisfied.

## Side effects

None. Specifically:
- WAL: not appended. Read-only RPC.
- canonical_store: read-only — `get_distinct_type_ids` (`apply/server/go/internal/store/`) and `get_distinct_edge_type_ids` (`:4103`). Both are `SELECT … GROUP BY` against the per-tenant SQLite (`:4063-4074`, `:4087-4101`) wrapped via `_run_sync` (CLAUDE.md invariant #4).
- global_store: untouched.
- quotas / rate limits: not enforced in the handler; rely on interceptor.
- schema registry: read-only (`schema_registry.to_dict()` `schema/registry.py:463-477`; `.fingerprint` property `:103-106`).

## Error contract (exhaustive grpc-code → trigger pairs)

The Python handler is unusual: it swallows ALL errors and returns `GetSchemaResponse(fingerprint="")` with `OK` status (`server/go/internal/api/get_schema.go`). The Go port MUST preserve this contract — the contract test (`tests/python/integration/test_grpc_contract.py:208`) only requires `r.fingerprint != "" or r.HasField("schema")`, so an empty-but-OK response is the documented degraded path.

| grpc.Code | Trigger |
|---|---|
| `OK` | Always returned by the Python handler, including under exception. Go MUST match. |
| `Unauthenticated` | Auth interceptor only (no creds / bad token). Not from handler. |
| `PermissionDenied` | Auth interceptor only (cross-tenant `tenant_id` outside caller scope, if policy enforces). Not from handler. |
| `Unavailable` / `FailedPrecondition` | Interceptor-level (e.g. shard-redirect via `entdb-redirect-node` trailer). `GetSchema` does NOT call `_check_tenant`, so it does NOT emit the redirect trailer — Go port should match. |
| `Internal` | NOT returned. Distinct-types SQLite errors are caught and logged at DEBUG (`:1665-1666`); outer catch-all at `:1686-1689`. |

Implication for Go: catch panics + every error from `canonical_store` / registry, log, and degrade. Do not propagate to `status.Error`.

## Shared Go package deps (one-line rationale each)

- `internal/schema` — port of `SchemaRegistry`: holds `NodeTypeDef`/`EdgeTypeDef`, exposes `ToMap() map[string]any`, `Fingerprint() string`, freeze semantics (`schema/registry.py:88-106`, `:420-461`).
- `internal/canonicalstore` — Go port of `canonical_store`: needs `GetDistinctTypeIDs(ctx, tenantID)` and `GetDistinctEdgeTypeIDs(ctx, tenantID)` returning `[]struct{TypeID int32; Count int64}` (`apply/server/go/internal/store/`, `:4103`).
- `internal/structpb` (or stdlib `structpb` from `google.golang.org/protobuf/types/known/structpb`) — for converting `map[string]any` → `*structpb.Struct`, mirroring `_dict_to_struct` (`server/go/internal/api/get_schema.go`).
- `internal/metrics` — `RecordGRPCRequest("GetSchema", "ok"|"error", duration)` mirroring `record_grpc_request` (`:1681`, `:1687`).
- `internal/logging` — structured logger; debug-level for fallback failure (`:1666`), error-level for outer catch (`:1688`).

## Other-RPC deps

None at runtime. The fingerprint returned here is consumed by `ExecuteAtomic` (proto field `schema_fingerprint`, `internal/pb/entdb.pb.go:490`) for client-side schema-pinning, but `ExecuteAtomic` reads its own copy from the registry. Go port can implement `GetSchema` standalone.

## Contract tests pinning behavior (file:line)

- `tests/python/integration/test_grpc_contract.py:203-209` — happy path: `tenant_id=TENANT`, mode "happy", check `r.fingerprint != "" or r.HasField("schema")`. This is the SOLE behavioral pin; Go port must satisfy it via either path (frozen registry => fingerprint set; empty registry => Struct populated from canonical_store).
- `sdk/go/entdb/cmd/entdbctl/cmd_schema_test.go:36` — Go SDK CLI test asserting response shape (consumer-side).
- `sdk/go/entdb/cmd/entdbctl/testutil_test.go:32-60` — fake server signature pins the proto contract.
- `sdk/go/entdb/cmd/entdb-console/upstream_stub.go:36-37` — console upstream pin.

No unit test exercises the handler directly; behavior is pinned only by the contract test above and by SDK consumer tests.

## Implementation outline (numbered Go handler steps)

```go
func (s *Server) GetSchema(ctx context.Context, req *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error)
```

1. `start := time.Now()`. Defer metrics emission with status label `"ok"` by default; recover from panic and switch to `"error"` (mirrors `:1681`/`:1687`).
2. Wrap the entire body in a `func() (*pb.GetSchemaResponse, error)` closure with a `defer recover()`. On any error or panic: log at error level, return `&pb.GetSchemaResponse{Fingerprint: ""}, nil` — no status error (`:1686-1689`).
3. Snapshot the schema: `schemaMap := s.schemaRegistry.ToMap()` and `fingerprint := s.schemaRegistry.Fingerprint()`. `ToMap` MUST sort `node_types` / `edge_types` by ID for fingerprint determinism (`schema/registry.py:470-477`).
4. `hasRegistryTypes := len(schemaMap["node_types"].([]any)) > 0 || len(schemaMap["edge_types"].([]any)) > 0`.
5. If `!hasRegistryTypes && req.TenantId != ""`: call `canonicalStore.GetDistinctTypeIDs(ctx, req.TenantId)` and `GetDistinctEdgeTypeIDs(ctx, req.TenantId)`. On error: log at DEBUG and skip (`:1665-1666`) — leave `schemaMap` empty. On success: rebuild `schemaMap` as in `:1645-1664`:
   - `node_types[i] = {type_id, name: fmt.Sprintf("Type_%d", id), fields: []}`
   - `edge_types[i] = {edge_id, name: fmt.Sprintf("Edge_%d", id), from_type_id: 0, to_type_id: 0, props: []}`
6. If `req.TypeId != 0`: filter in place (`:1668-1679`):
   - `node_types`: keep where `type_id == req.TypeId`.
   - `edge_types`: keep where `from_type_id == req.TypeId || to_type_id == req.TypeId`.
   Note: filtering happens AFTER the data-driven fallback, so a fallback-built schema (with `from_type_id=0`) will only retain edge types when `req.TypeId == 0`.
7. Convert `schemaMap` → `*structpb.Struct` via `structpb.NewStruct(schemaMap)`. If conversion errors (non-JSON-able value), log error and degrade per step 2.
8. Return `&pb.GetSchemaResponse{Schema: structVal, Fingerprint: fingerprint}`. When the registry is unfrozen `fingerprint == ""`, matching Python's `self.schema_registry.fingerprint or ""` (`:1684`).
9. Emit metric `RecordGRPCRequest("GetSchema", "ok", elapsed)` on the success path before return.

## Open questions / risks

- **Auth posture**: Python silently allows anonymous calls. Confirm with EPIC owners whether Go should require an authenticated principal (consensus reading of CLAUDE.md security commit `fece3fb` suggests yes for everything else, but `GetSchema` is metadata-only). Default: keep parity, allow anonymous, document.
- **Cross-tenant `tenant_id`**: Handler reads from `req.TenantId`'s SQLite without checking caller's tenant scope. This leaks distinct `type_id` integers across tenants. Likely a latent bug. Go port should add a `_check_tenant`-equivalent gate when `req.TenantId != ""` and the caller is bound to a different tenant — but flag this as a behavior change vs. Python and gate it behind a feature flag for the contract-test transition.
- **Empty registry + non-zero `type_id` + empty `tenant_id`**: Python returns the zero-value Struct after filtering. Go must replicate (no panic on `nil` slices).
- **Struct conversion of integer keys**: `to_dict()` emits `type_id` as Go `int`/Python `int`. `structpb.NewStruct` requires float64 for numbers — confirm round-trip. The Python side returns it via `Struct.update(d)` which floats integers; clients (incl. SDKs) already handle this. Match it.
- **Fingerprint determinism**: Go `ToMap` MUST sort by ID and use deterministic JSON (no map iteration order) — fingerprint is `sha256:<hex>` of canonical JSON (`registry.py:448-461`). Cross-language fingerprint parity is required so existing Python-issued fingerprints validate against a Go-served registry; verify via a contract test against a fixed test registry.
- **Metric name**: Python uses `"GetSchema"`. Go must emit identical label so existing dashboards keep working.
- **Schema fallback mismatch**: Data-driven fallback yields `from_type_id=0,to_type_id=0` placeholders (`:1658-1660`); under `type_id` filter (step 6) a non-zero filter drops all edges. Documented as-is — likely benign but surfaces in alerts if anyone queries with both flags. No fix in port.
