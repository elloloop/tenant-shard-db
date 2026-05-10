# GetEdgesFrom — Go port spec

EPIC #407. Mirror of Python `EntDBServicer.GetEdgesFrom`
(`server/python/entdb_server/api/grpc_server.py:1384-1418`). Reads the
outgoing edges of a node from per-tenant SQLite. Pure read; no WAL
writes.

## Wire contract

- Service: `entdb.v1.EntDBService`
- Method: `GetEdgesFrom`, full name `/entdb.v1.EntDBService/GetEdgesFrom`
  (`sdk/go/entdb/internal/pb/entdb_grpc.pb.go:63`).
- Proto: `proto/entdb/v1/entdb.proto:64` —
  `rpc GetEdgesFrom(GetEdgesRequest) returns (GetEdgesResponse);`
- `GetEdgesRequest` (`proto/entdb/v1/entdb.proto:476-486`):
  - `RequestContext context = 1` — `tenant_id` (required), `actor`
    (advisory; do NOT trust — see Auth), `trace_id`.
  - `string node_id = 2` — source node; matched against
    `edges.from_node_id`. Empty string returns zero rows (no special
    error in Python today).
  - `int32 edge_type_id = 3` — optional filter; treat any falsy value
    (0) as "no filter" to match Python (`grpc_server.py:1393`).
  - `int32 limit = 4` — defaults to 100 when 0/unset
    (`grpc_server.py:1400`).
  - `int32 offset = 5` — declared in proto but **not honoured by the
    Python handler today**; see Open questions.
- `GetEdgesResponse` (`proto/entdb/v1/entdb.proto:488-491`):
  - `repeated Edge edges` — each `Edge` (`proto/.../entdb.proto:493-507`)
    carries `tenant_id`, `edge_type_id`, `from_node_id`, `to_node_id`,
    `created_at`, `props` (`google.protobuf.Struct`). Field 5
    (`props_json`) is reserved.
  - `bool has_more` — Python sets it iff `len(edges) > limit` *before*
    truncation (`grpc_server.py:1410-1414`). Note: store returns the
    full result set (no SQL `LIMIT`), the handler slices in Python.

## Auth

- **Tenant ownership / region pinning.** First action is
  `await self._check_tenant(request.context.tenant_id, context)`
  (`grpc_server.py:1392`, helper at `:362-410`). On a miss the handler
  aborts `UNAVAILABLE` with `entdb-redirect-node` trailer (sharding) or
  `FAILED_PRECONDITION` (region pin). The Go port must reproduce both
  paths and the trailer key exactly — Go SDK redirect cache reads it
  (`sdk/go/entdb/redirect_cache.go`).
- **Capability.** `capability_registry.DEFAULT_OP_REQUIREMENTS` maps
  `"GetEdgesFrom" → CoreCapability.READ`
  (`server/python/entdb_server/auth/capability_registry.py:66`). The
  Python handler does NOT currently invoke a per-node ACL check before
  fan-out — it returns every edge stored for the tenant + node. The Go
  port should match this for parity in v1; do not add an ACL filter
  without a contract change.
- **Does ACL filter destination nodes the caller can't see?** **No.**
  The handler returns edges to nodes the caller may have no `READ` on.
  This is a known parity gap pinned by the privilege-escalation test
  (see below) which deliberately accepts the weaker property. EPIC #407
  decisions: keep parity; track ACL filtering as a follow-up.
- **Trusted actor.** `request.context.actor` is advisory only. The
  handler does not currently rebind to `_trusted_actor()`
  (`grpc_server.py:418-437`); the privilege-escalation test pins that
  the claimed actor must not be used for any authz decision the
  handler reaches (it currently reaches none). Go port: read trusted
  identity from the auth interceptor metadata, ignore
  `request.context.actor` for any future authz, plumb the trusted
  actor into observability fields only.

## Side effects

None. Read-only. **No WAL append, no SQLite writes, no state
mutation.** Confirms architecture invariant 1 in `CLAUDE.md` — this
RPC is on the egress path; never call `wal.append` here. Permitted
side effects: prometheus counter
`record_grpc_request("GetEdgesFrom", "ok"|"error", elapsed)`
(`grpc_server.py:1413,1416`) and structured logs.

## Error contract

The Python handler swallows everything: any exception below
`_check_tenant` is logged and returns `GetEdgesResponse(edges=[])`
with no status code (`grpc_server.py:1415-1418`). `_check_tenant`
itself can `context.abort` with `UNAVAILABLE` or
`FAILED_PRECONDITION` and those propagate.

Go port mapping:

- Tenant not on this node → `codes.Unavailable`, message
  `"Tenant '<id>' is not served by this node[ (try node X)]"`,
  trailer `entdb-redirect-node: <owner>` when known.
- Wrong region → `codes.FailedPrecondition`, message
  `"Tenant '<id>' is pinned to region '<R>'; this node serves '<S>'."`.
- Unknown internal failure → mirror Python: log + return empty
  response with no error. (Recommend logging + metric, no
  status-code change, to keep wire parity. Open question below
  whether to upgrade to `Internal`.)
- Empty `node_id` → empty response, no error.
- Negative or zero `limit` → treat as 100.

## Shared Go package deps

- `internal/pb` — generated `entdb.v1` types
  (`sdk/go/entdb/internal/pb/entdb_grpc.pb.go`). Server side will need
  the server stubs in `server/go/internal/pb/...`.
- `shared/canonicalstore` (TBD; place in
  `docs/go-port/shared/canonicalstore.md`) — needs `GetEdgesFrom(ctx,
  tenantID, nodeID, edgeTypeID *int32) ([]Edge, error)`. Python
  reference: `_sync_get_edges_from`
  (`server/python/entdb_server/apply/canonical_store.py:2609-2640`).
  SQL: `SELECT * FROM edges WHERE tenant_id=? AND from_node_id=?`
  with optional `AND edge_type_id=?`. Index used:
  `idx_edges_from(tenant_id, from_node_id)`
  (`canonical_store.py:1075`).
- `shared/auth` — `CheckTenant(ctx, tenantID)` returning ownership +
  region verdict and `TrustedActorFromCtx(ctx)`. Mirrors
  `_check_tenant` and `auth_interceptor.get_authoritative_actor`.
- `shared/sharding` — `IsMine`, `Owner` lookups for the redirect
  trailer.
- `shared/observability` — `RecordGRPCRequest("GetEdgesFrom",
  status, elapsed)`.
- `shared/structconv` — `MapToStruct(map[string]any)
  *structpb.Struct` for `Edge.props` (Python side: `_dict_to_struct`).

## Other-RPC deps

`GetEdgesFrom` and `GetEdgesTo` are byte-for-byte symmetric — same
request / response messages, same auth, same shape, same error
contract. Differences are exactly:

- DB column matched: `from_node_id` vs `to_node_id`.
- canonical_store call: `get_edges_from` vs `get_edges_to`
  (`canonical_store.py:2642`, `:2695`).
- Metric label and log prefix.

Implement both as one private helper parameterised on direction; see
`grpc_server.py:1420-1454` for the symmetric Python implementation.
`GetConnectedNodes` (`grpc_server.py:1700-…`) reuses the same fan-out
under the hood and should share the helper.

## Contract tests pinning behaviour

- `tests/python/integration/test_grpc_contract.py:241-247` — happy
  path: empty seed, `GetEdgesFrom(node_id=SEED_NODE_ID)` returns
  `edges == []` and call succeeds (no abort).
- `tests/python/integration/test_privilege_escalation.py:421-447` —
  `test_get_edges_from_does_not_use_claimed_actor_for_authz`: with
  `_trusted_identity("user:eve")` and forged
  `context.actor="system:admin"`, handler must (a) succeed and (b)
  invoke `canonical_store.get_edges_from` with the trusted
  `tenant_id` (asserted via `cs.get_edges_from.assert_awaited_once`).
- `sdk/go/entdb/grpc_transport_test.go:669-692` — Go SDK happy path:
  request carries `node_id`, `edge_type_id`; response edges echo
  fields 1-7. Pin output struct order.
- `sdk/go/entdb/cmd/entdb-console/server_test.go:338-345` — console
  proxy must forward `GetEdgesFrom` to the upstream stub unchanged.
- `sdk/go/entdb/cmd/entdbctl/cmd_edges_test.go:27,57` — CLI uses
  `GetEdgesFrom` for the default direction and **must not** call it
  when `--in` is set. The Go server must keep the directionality
  exactly so this test stays valid.
- `tests/python/benchmarks/bench_realistic.py:549` — perf reference
  (latency budget; not a strict pin but informative).

## Implementation outline

```go
func (s *Server) GetEdgesFrom(
    ctx context.Context, req *pb.GetEdgesRequest,
) (*pb.GetEdgesResponse, error) {
    start := time.Now()
    defer func() { obs.RecordGRPCRequest("GetEdgesFrom", outcome, time.Since(start)) }()

    if err := s.checkTenant(ctx, req.GetContext().GetTenantId()); err != nil {
        return nil, err // Unavailable / FailedPrecondition with trailer
    }

    // Ignore req.Context.Actor for authz; trusted identity comes from
    // the auth interceptor (mirror _trusted_actor).
    var edgeTypeFilter *int32
    if etid := req.GetEdgeTypeId(); etid != 0 {
        v := etid
        edgeTypeFilter = &v
    }

    edges, err := s.cstore.GetEdgesFrom(ctx,
        req.GetContext().GetTenantId(), req.GetNodeId(), edgeTypeFilter)
    if err != nil {
        log.Error(ctx, "GetEdgesFrom failed", "err", err)
        return &pb.GetEdgesResponse{}, nil // parity with Python swallow
    }

    limit := int(req.GetLimit())
    if limit <= 0 { limit = 100 }
    hasMore := len(edges) > limit
    if hasMore { edges = edges[:limit] }

    out := make([]*pb.Edge, 0, len(edges))
    for _, e := range edges {
        out = append(out, &pb.Edge{
            TenantId: e.TenantID, EdgeTypeId: e.EdgeTypeID,
            FromNodeId: e.FromNodeID, ToNodeId: e.ToNodeID,
            CreatedAt: e.CreatedAt, Props: structconv.FromMap(e.Props),
        })
    }
    return &pb.GetEdgesResponse{Edges: out, HasMore: hasMore}, nil
}
```

Keep the limit-then-slice in handler (NOT in SQL) until the Python
side moves; otherwise `has_more` semantics drift.

## Open questions / risks

1. **`offset` is declared in the proto but ignored by the Python
   handler.** Python only respects `limit`, slicing in-process. Go
   port should keep ignoring `offset` for parity *or* the contract
   should be updated server-side first. Flag for #407 owner.
2. **No SQL `LIMIT/ORDER BY`.** SQLite returns rows in insertion
   order, slicing happens in Python. Result ordering is not
   contractually defined and large fan-outs read the entire row set
   into memory. Risk of perf regression when Go port matches behaviour
   on hot tenants. Consider adding a deterministic `ORDER BY
   created_at, edge_type_id, to_node_id` in the shared
   `canonicalstore` and pinning a contract test before changing it.
3. **No ACL filtering of destination nodes.** Caller can enumerate
   `to_node_id`s without READ on the targets. The privilege test
   accepts this; product/security must decide before v1 GA.
4. **Error swallowing.** Returning empty list on internal failure
   masks bugs. Recommend introducing `codes.Internal` once the SDK
   tests are updated; until then, parity wins.
5. **`props` Struct conversion.** Python stores props as a JSON dict
   keyed by **field IDs** (invariant 6). Confirm `structconv` mirrors
   `_dict_to_struct` exactly — including how it handles ints, floats,
   nulls, and nested objects — or contract tests will diverge.
6. **`request.context.actor` parity.** The privilege-escalation test
   only pins the negative property. The Go port should still rebind
   to the trusted actor at handler entry to prevent regression when
   the future ACL filter (item 3) lands.
