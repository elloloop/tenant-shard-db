# GetEdgesTo — Go port spec

EPIC: #407. Inverse fan-in counterpart of `GetEdgesFrom`. Returns edges whose
`to_node_id` matches the requested node, scoped to a single tenant. Read-only,
non-mutating, no WAL append.

## Wire contract

- Service: `entdb.v1.EntDBService`
- Method: `GetEdgesTo(GetEdgesRequest) returns (GetEdgesResponse)` —
  `proto/entdb/v1/entdb.proto:67`
- Request (`GetEdgesRequest`, proto descriptor in `entdb_pb2.py:28`):
  - `context.RequestContext` — `tenant_id` (1), `actor` (2), `trace_id` (3)
  - `node_id` (2, string) — TARGET node whose inbound edges are wanted
  - `edge_type_id` (3, int32) — `0` ⇒ all types; nonzero ⇒ filter
  - `limit` (4, int32) — `0` ⇒ default 100 (Python handler line 1436)
  - `offset` (5, int32) — currently UNUSED in Python (see Open questions)
- Response (`GetEdgesResponse`):
  - `edges` (1, repeated `Edge`)
  - `has_more` (2, bool) — `true` iff store returned `> limit` rows
- `Edge` proto fields used: `tenant_id`, `edge_type_id`, `from_node_id`,
  `to_node_id`, `props` (google.protobuf.Struct), `created_at` (int64 ms).
  `props_json` (field 5) is reserved/legacy and MUST NOT be set.

## Auth

- Capability: `CoreCapability.READ`, registered in
  `server/python/entdb_server/auth/capability_registry.py:67`
  (`DEFAULT_OP_REQUIREMENTS["GetEdgesTo"] = READ`).
- Tenant ownership: handler calls `_check_tenant(tenant_id, ctx)`
  (`api/grpc_server.py:1428`). Rejects with `UNAVAILABLE` + trailing
  metadata `entdb-redirect-node` if the shard does not own the tenant
  (`grpc_server.py:362–395`); rejects with `FAILED_PRECONDITION` on
  region-pin mismatch (`grpc_server.py:400–410`).
- Trusted actor: per CLAUDE.md invariant and PR #168, the handler MUST
  use `_trusted_actor(request.context.actor)` (the
  `AuthInterceptor`-attached identity) for any future per-edge ACL
  check. The current Python handler does NOT yet wire ACL filtering into
  `get_edges_to`; the Go port should keep the trust boundary in place
  even though it is currently a no-op (see test
  `test_privilege_escalation.py:421` comment — same property pinned for
  `GetEdgesFrom`; the symmetric test for `GetEdgesTo` is OPEN).

## Side effects

- None. Read-only — no WAL append, no SQLite mutation, no global_store
  write. Does NOT bypass the WAL invariant (CLAUDE.md §1) because no
  state changes.
- Metrics: `record_grpc_request("GetEdgesTo", "ok"|"error", elapsed)`
  (`grpc_server.py:1449,1452`).
- Logging: `logger.error("GetEdgesTo failed: ...", exc_info=True)` on
  exception path (`grpc_server.py:1453`).
- Cross-tenant fan-in: NOT a concern at the storage layer. Each tenant
  has its own SQLite (`canonical_store.py:1068` PRIMARY KEY includes
  `tenant_id`; `idx_edges_to` at `canonical_store.py:1076`). Edges are
  per-tenant rows; an edge "spanning tenants" is structurally
  impossible — both endpoints share the row's `tenant_id`. The query
  filters `WHERE tenant_id = ? AND to_node_id = ?`
  (`canonical_store.py:2673,2679`), so fan-in cannot leak across tenant
  boundaries even if a caller forges a `node_id` that exists in another
  tenant.

## Error contract

Python today returns `GetEdgesResponse(edges=[])` on ANY internal
exception (`grpc_server.py:1454`) — i.e., it swallows errors and emits
an empty result. The Go port SHOULD preserve this swallow-and-empty
behaviour for parity, with the following pinned aborts BEFORE the
try-block reaches the store:

- `UNAVAILABLE` — tenant not owned by this node (with
  `entdb-redirect-node` trailer when known).
- `FAILED_PRECONDITION` — tenant pinned to another region.

After `_check_tenant` succeeds, no further `context.Abort` paths exist;
any storage error becomes `edges=[]` + metric label `error`. Note:
this empty-on-error policy is debated (see Open questions).

## Shared Go package deps

- `internal/auth` — `CheckTenant(ctx, tenantID)` mirroring
  `_check_tenant` (shared with every other tenant-scoped RPC; reused
  from GetEdgesFrom port).
- `internal/auth` — `TrustedActor(ctx, claimed)` reading the
  interceptor `ContextVar` equivalent; same helper as GetEdgesFrom.
- `internal/canonical` — `GetEdgesTo(ctx, tenantID, nodeID,
  edgeTypeID *int32) ([]Edge, error)` SQLite read; query identical to
  `_sync_get_edges_to` (`canonical_store.py:2662–2693`).
- `internal/proto/convert` — `EdgeToProto(Edge) *pb.Edge` (shared with
  GetEdgesFrom; converts `props map[string]any` → `*structpb.Struct`).
  Reuse `_dict_to_struct` equivalent.
- `internal/metrics` — `RecordGrpcRequest(method, status, elapsed)`.
- `internal/grpcutil` — limit clamp helper (default 100, `has_more`
  computation: `len(edges) > limit`).

## Other-RPC deps (overlap with GetEdgesFrom)

`GetEdgesTo` is the symmetric twin of `GetEdgesFrom`
(`grpc_server.py:1384`). Shared code surface:

- Same request/response proto messages (`GetEdgesRequest`,
  `GetEdgesResponse`).
- Same capability mapping (`READ`).
- Same `_check_tenant` ingress check.
- Same `Edge` → proto conversion path.
- Same limit/has_more semantics (`limit or 100`, slice `edges[:limit]`,
  `has_more = len(edges) > limit`).
- Differs only in the SQL predicate: `from_node_id = ?` vs
  `to_node_id = ?` and uses index `idx_edges_to`
  (`canonical_store.py:1076`) instead of the `from`-side index.

Recommended: implement a single private `getEdgesByEndpoint(direction
{from|to}, ...)` helper in the Go server, called by both RPCs, and
land both in the same PR (CLAUDE.md cross-reference: SDK-side
`grpc_transport_test.go:669,694` already does this for the client).

## Contract tests pinning behavior (file:line)

- `tests/python/integration/test_grpc_contract.py:248–254` — happy
  path: empty seed ⇒ `r.edges == []` (response is success, not error).
- `tests/python/unit/test_canonical_store.py:331–370` — store-level:
  two edges to one target ⇒ length 2.
- `tests/python/integration/test_crud_comprehensive.py:317–325` —
  store-level fan-in count.
- `tests/python/integration/test_workplace_chat.py:233,253,284,294,
  315,370,749,798,960` — `edge_type_id` filtering on member fan-in.
- `tests/python/integration/test_workplace_posts.py:396,444,454,469,
  490,501,514,534,573,574,693` — fan-in for upvotes/replies.
- `tests/python/integration/test_workplace_tasks.py:318,386,402,452,
  473,485,522,534,576,646,704,938` — project/task assignment fan-in.
- `tests/python/integration/test_multi_node_sharding.py:1132` — fan-in
  inside the multi-shard fixture (covers `_check_tenant` redirect
  trailer indirectly).
- `tests/python/integration/test_batch_applier.py:1223` — empty
  fan-in for non-existent `to_node_id` (`"ghost"`).
- `tests/python/integration/test_privilege_escalation.py:132,421` —
  pins that the handler does NOT trust `context.actor` for authz; the
  `GetEdgesTo`-specific test is currently absent and SHOULD be added
  in the Go port (parity with line 429 `test_get_edges_from_does_not_
  use_claimed_actor_for_authz`).
- `sdk/go/entdb/grpc_transport_test.go:694–712` — SDK-side wire shape:
  one inbound edge, `from_node_id`/`to_node_id`/`edge_type_id` round
  trip.
- `tests/python/benchmarks/bench_crud.py:344–350` — perf budget; Go
  port should match or beat.

## Implementation outline

```go
func (s *Server) GetEdgesTo(ctx context.Context, req *pb.GetEdgesRequest) (*pb.GetEdgesResponse, error) {
    start := time.Now()
    tenantID := req.GetContext().GetTenantId()

    // Ingress checks — may abort with UNAVAILABLE / FAILED_PRECONDITION.
    if err := s.checkTenant(ctx, tenantID); err != nil {
        metrics.RecordGrpcRequest("GetEdgesTo", "error", time.Since(start))
        return nil, err
    }
    _ = auth.TrustedActor(ctx, req.GetContext().GetActor()) // pin trust boundary

    var edgeType *int32
    if t := req.GetEdgeTypeId(); t != 0 {
        edgeType = &t
    }

    edges, err := s.canonical.GetEdgesTo(ctx, tenantID, req.GetNodeId(), edgeType)
    if err != nil {
        s.log.Error("GetEdgesTo failed", "err", err)
        metrics.RecordGrpcRequest("GetEdgesTo", "error", time.Since(start))
        return &pb.GetEdgesResponse{}, nil // parity: swallow + empty
    }

    limit := int(req.GetLimit())
    if limit == 0 { limit = 100 }
    hasMore := len(edges) > limit
    if hasMore { edges = edges[:limit] }

    out := make([]*pb.Edge, 0, len(edges))
    for _, e := range edges { out = append(out, convert.EdgeToProto(e)) }
    metrics.RecordGrpcRequest("GetEdgesTo", "ok", time.Since(start))
    return &pb.GetEdgesResponse{Edges: out, HasMore: hasMore}, nil
}
```

SQLite query in `internal/canonical`:

```sql
-- with edge_type filter
SELECT tenant_id, edge_type_id, from_node_id, to_node_id, props_json, created_at
FROM edges
WHERE tenant_id = ? AND to_node_id = ? AND edge_type_id = ?;
-- without
SELECT ... FROM edges WHERE tenant_id = ? AND to_node_id = ?;
```

Use `idx_edges_to(tenant_id, to_node_id)` (`canonical_store.py:1076`).

Run on a dedicated read-side worker pool (mirror Python's `_run_sync`
thread-pool semantics; CLAUDE.md §3 forbids `ThreadPoolExecutor`-style
sync/async bridging in the request path itself, but per-tenant SQLite
calls SHOULD use a bounded pool so a slow query cannot stall the
loop).

## Open questions / risks

1. `offset` field is silently ignored — Python and the Go SDK both
   pass it through but the server does not paginate. Decision needed:
   honour `offset` with `LIMIT ? OFFSET ?` or formally deprecate the
   field. Pinning current behaviour (no-op) is the safe Phase-1
   choice.
2. `has_more` is computed AFTER fetching all matching rows from SQLite
   then slicing in memory — for high-fan-in targets (e.g. a popular
   post with 1M upvotes) this is O(N) memory per request. Decision:
   keep parity for v1; file follow-up to push `LIMIT limit + 1` into
   SQL.
3. Empty-on-error swallow is hostile to clients (looks identical to
   "no edges"). Should the Go port instead return
   `codes.Internal`? Recommend: keep parity, but emit a structured log
   + metric label so operators can distinguish.
4. No per-edge ACL filtering today. When CLAUDE.md's capability
   model is extended to edges, fan-in (`GetEdgesTo`) is the harder
   case: a target node owner may not have READ on the source nodes.
   Spec the filter at the canonical_store layer, not the handler, so
   GetEdgesFrom and GetEdgesTo share enforcement.
5. `created_at` precision: Python emits ms (int64). Confirm Go port
   uses the same unit, not `time.Time`/RFC3339.
6. Missing privilege-escalation test for `GetEdgesTo` (parity with
   `test_get_edges_from_does_not_use_claimed_actor_for_authz` at
   `test_privilege_escalation.py:429`) — add as part of port PR.
