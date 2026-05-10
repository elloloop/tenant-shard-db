# QueryNodes — Go-port spec

EPIC #407. RPC `entdb.v1.EntDBService/QueryNodes` (re-exported via
`console.v1.ConsoleService/QueryNodes`). Python ref:
`server/python/entdb_server/api/grpc_server.py:1293`.

Read-only: list nodes of `type_id` for a tenant, with typed payload
filters, sort, offset-pagination, read-after-write WAL-offset gate.
Cross-tenant actors get an ACL post-filter.

## Wire contract (filter/predicate shape; pagination cursor; sort)

`QueryNodesRequest` (`proto/entdb/v1/entdb.proto:424`): `context`,
`type_id` (req'd), `limit` (default 100), `offset`, `order_by`,
`descending`, `filters`, `after_offset`, `wait_timeout_ms` (default
30s). Field 3 is `reserved` (legacy `filter_json` — Go MUST reject).

`FieldFilter` (`proto/entdb/v1/entdb.proto:675`): `{field, op,
value}`. `FilterOp`: `EQ, NEQ, GT, GTE, LT, LTE, CONTAINS, IN`
(line 681).

Python servicer translates `repeated FieldFilter` into a MongoDB-
style operator dict via `_field_filters_to_filter_dict`
(`grpc_server.py:190`). Two wire shapes MUST round-trip identically:

1. Typed (Go SDK): `Op=GTE, Value=100` → `{"$gte": 100}`.
2. Inlined (Python SDK legacy): `Op=EQ, Value=Struct{"$gte":100}` →
   same `{"$gte": 100}`.

Multiple filters on one field merge into one operator dict (range
splits `GTE`+`LTE`); equality collapses to `$eq` on merge
(`grpc_server.py:218-227`). Op map: `grpc_server.py:178-187`.

`order_by` allow-list `{created_at, updated_at, node_id, type_id}`;
unknown silently falls back to `created_at` (`canonical_store.py:2415`).

`QueryNodesResponse` (`proto/entdb/v1/entdb.proto:448`): `nodes,
total_count, has_more`. `total_count` is unset (0 on wire); `has_more
= len(nodes) == requested_limit` — cheap heuristic, NOT a strict
cursor (`grpc_server.py:1377`). No opaque cursor; pagination is
`(limit, offset)` only.

## Auth (ACL filter on rows; trusted-actor)

1. `_check_tenant(tenant_id)` — confirms tenant exists (`grpc_server.py:1305`).
2. `_trusted_actor(request.context.actor)` — derives the actor from
   the gRPC AuthContext / interceptor metadata, NOT from the
   client-claimed `actor` field. The claimed value is ignored — see
   the privilege-escalation fix in commit `fece3fb`
   (`grpc_server.py:418`, also applied to `GetConnectedNodes`).
3. `_check_cross_tenant_read(tenant_id, trusted_actor)` returns one
   of `"member" | "cross_tenant" | "denied"`
   (`grpc_server.py:561`). `denied` raises `PERMISSION_DENIED`.
4. When role is `cross_tenant`, the result set is post-filtered: each
   node is checked via `canonical_store.can_access(tenant, node_id,
   actor_groups)` and excluded if not accessible
   (`grpc_server.py:1344-1358`). Group membership is resolved via
   `resolve_actor_groups`.

The ACL filter is applied AFTER the SQL query — this means
`limit`/`has_more` reflect the pre-ACL row count, so a cross-tenant
caller can see fewer than `limit` rows on a page that is not the
last page. Document this gotcha in the Go SDK; do NOT change it
without an ADR (changes pagination semantics).

## Side effects (read-only; index hits)

Read-only. No WAL append, no state change, applier not invoked.
Opens a per-tenant SQLite read connection
(`canonical_store.py:2413`). Lazy expression indexes
(`idx_query_t<type>_f<field>`) declared via
`(entdb.field).indexed = true` are created by the WRITE path
(`_ensure_query_indexes` on `CreateNode`/`UpdateNode`); QueryNodes
benefits when the compiled WHERE
(`json_extract(payload_json, '$."<field_id>"') <op> ?`) matches the
indexed expression byte-for-byte
(`docs/decisions/query_indexes.md:25-47`).

`after_offset` triggers `_wait_for_offset` (`grpc_server.py:937`) —
blocks up to `wait_timeout_ms` (default 30s). Go: context-aware wait
on the applier's offset condvar; do NOT busy-poll.

## Error contract

| Condition                                  | Status               | Source |
|--------------------------------------------|----------------------|--------|
| Unknown tenant                             | `NOT_FOUND`          | `_check_tenant` |
| Actor lacks read access to tenant          | `PERMISSION_DENIED`  | `_check_cross_tenant_read` |
| `wait_timeout_ms` elapsed before offset    | `DEADLINE_EXCEEDED`  | `_wait_for_offset` |
| Unknown field name in filter               | `INVALID_ARGUMENT`   | `QueryFilterError` (`query_filter.py:120`) |
| Unknown operator / wrong arg shape         | `INVALID_ARGUMENT`   | `query_filter.py:181-202` |
| Unsupported `FilterOp` enum value          | `INVALID_ARGUMENT`   | `grpc_server.py:216` |
| Unknown `type_id`                          | `INVALID_ARGUMENT`   | `query_filter.py:113` |

NOTE the Python handler currently swallows ALL exceptions and returns
an empty `QueryNodesResponse{nodes: []}` (`grpc_server.py:1379-1382`).
This is a bug — the Go port MUST surface the gRPC status above. Pin
this as a behaviour CHANGE in EPIC #407 and update the contract test
listed below.

## Shared Go package deps

- `internal/auth` — `TrustedActor(ctx)`, `CheckTenant`,
  `CheckCrossTenantRead` (port of `grpc_server.py:362-628`).
- `internal/canonical` — `QueryNodes(tenantID, typeID, filter,
  limit, offset, orderBy, desc) ([]Node, error)`; `CanAccess`,
  `ResolveActorGroups` for the cross-tenant post-filter.
- `internal/queryfilter` — port of
  `server/python/entdb_server/apply/query_filter.py`. Pure SQL
  builder: `Compile(filterDict, typeID, registry) (sqlFrag string,
  params []any, err error)`. Allow-listed operators only; field
  names resolved to `field_id` via the schema registry; values
  always parameter-bound.
- `internal/schema` — `Registry.GetNodeType(typeID)` /
  `GetField(typeID, name) -> FieldID`.
- `internal/applier` — `WaitForOffset(ctx, tenantID, pos)` for
  read-after-write.
- `internal/proto/structpb` — for inlined-operator legacy shape;
  `google.golang.org/protobuf/types/known/structpb`.
- `internal/metrics` — `RecordGRPCRequest("QueryNodes", status, dur)`.

## Other-RPC deps

- Reads state produced by the applier driven by `Begin` /
  `CreateNode` / `UpdateNode` — same per-tenant SQLite as `GetNode` /
  `GetNodes`.
- Shares the cross-tenant gate with `GetConnectedNodes`,
  `GetEdgesFrom`/`To`, `GetNodes`. Port the gate ONCE in
  `internal/auth` and reuse — do not duplicate per RPC.
- `WaitForOffset` is shared infrastructure with `GetNode`,
  `WaitForOffset` RPC, `SearchNodes`.

## Contract tests pinning behaviour

- `tests/python/integration/test_grpc_contract.py:234` — basic
  list-everything wire shape.
- `tests/python/integration/test_privilege_escalation.py:232` —
  client-claimed admin actor MUST be rejected (commit fece3fb).
- `tests/python/unit/test_payload_wire_format.py:219` — response
  payload MUST be id-keyed (`field_id` strings), not name-keyed.
- `tests/python/unit/test_cross_tenant_read.py:255` — cross-tenant
  reader sees only ACL-permitted nodes.
- `tests/python/unit/test_cross_tenant_read.py:288` — no access ⇒
  empty result.
- `tests/python/unit/test_query_operators.py:361` — full
  `TestQueryNodesE2E` battery (`$eq, $ne, $gte, $between, $in,
  $like, $or, $and, $contains, unknown-field`).
- `tests/python/unit/test_query_operators.py:474` — unknown field
  raises (must surface as `INVALID_ARGUMENT` in Go).
- `tests/python/unit/test_canonical_store_perf.py:75` —
  `TestQueryNodesSQLPushdown` confirms filter is pushed to SQL, not
  applied in Python.
- `tests/python/unit/test_cron_fixes.py:153` —
  `TestQueryNodesFilterPagination` (filter + pagination interaction).
- `tests/python/unit/test_capability_registry.py:63` — RPC mapped to
  `CoreCapability.READ`.

A new contract test in `tests/contract/` MUST exercise BOTH wire
shapes (typed `Op=GTE`, inlined `Struct{"$gte": ...}`) against the
Go server to lock in `_field_filters_to_filter_dict` parity.

## Implementation outline

```
QueryNodes(ctx, req):
  start := time.Now()
  defer recordMetric("QueryNodes", status, time.Since(start))

  if err := auth.CheckTenant(ctx, req.Context.TenantId); err != nil { return nil, err }
  actor, err := auth.TrustedActor(ctx)              // ignore req.Context.Actor
  if err != nil { return nil, err }
  role, err := auth.CheckCrossTenantRead(ctx, req.Context.TenantId, actor)
  if err != nil { return nil, err }                  // PERMISSION_DENIED

  if req.AfterOffset != "" {
    timeout := durationOr(req.WaitTimeoutMs, 30*time.Second)
    if err := applier.WaitForOffset(ctx, req.Context.TenantId, req.AfterOffset, timeout); err != nil {
      return nil, err                                // DEADLINE_EXCEEDED
    }
  }

  filterDict, err := fieldFiltersToDict(req.Filters) // mirror grpc_server.py:190
  if err != nil { return nil, status.Error(codes.InvalidArgument, err.Error()) }

  nodes, err := canonical.QueryNodes(ctx, canonical.QueryArgs{
    TenantID: req.Context.TenantId, TypeID: req.TypeId,
    Filter: filterDict, Limit: limitOr(req.Limit, 100), Offset: int(req.Offset),
    OrderBy: orderByOr(req.OrderBy, "created_at"), Descending: req.Descending,
    Registry: s.registry,
  })
  if err != nil { return nil, mapFilterError(err) } // QueryFilterError -> InvalidArgument

  if role == auth.RoleCrossTenant {
    groups, _ := canonical.ResolveActorGroups(ctx, req.Context.TenantId, actor)
    nodes = filterAccessible(ctx, nodes, groups)     // post-filter
  }

  return &pb.QueryNodesResponse{
    Nodes: toProtoNodes(nodes),
    HasMore: len(nodes) == limitOr(req.Limit, 100),
  }, nil
```

SQL builder (`internal/queryfilter`):

- Compile to `(fragment, params)` with ONLY parameter binding for
  values; operator tokens come from a closed map; field-id JSON path
  literals come from the registry — never from user input.
- Mirror `compile_query_filter` recursion for `$and` / `$or`
  (`query_filter.py:266-282`).
- `$contains` MUST escape `%` / `_` / `\` in user input before LIKE
  with `ESCAPE '\\'` (`query_filter.py:215-224`).
- Reject unknown operators and unknown fields with
  `QueryFilterError` (mapped to `INVALID_ARGUMENT`).
- For the legacy inlined-operator shape, the builder accepts a
  `map[string]any` whose values may themselves be operator dicts —
  same recursion handles both shapes.

ACL post-filter:

- Iterate `nodes`; call `canonical.CanAccess` for each. Keep this
  serial (per-tenant SQLite is single-writer; concurrent reads are
  fine but goroutine-per-row is overkill). Stop early if the caller's
  context is cancelled.

## Open questions / risks

1. Full-scan vs index strategy: SQLite's planner picks the lazy
   expression index when the WHERE expression matches
   `json_extract(payload_json, '$."<field_id>"') <op> ?` exactly.
   Any rewrite (CAST, COALESCE) silently disables the index. Add an
   `EXPLAIN QUERY PLAN` regression test in
   `tests/contract/queryfilter/` per declared
   `(entdb.field).indexed = true` field.
2. How the Python applies the index decision: it does NOT — the
   WRITE path creates the index lazily (`_ensure_query_indexes`,
   `docs/decisions/query_indexes.md:42`) and the READ path is
   oblivious. Go must keep this split (index DDL in
   `internal/applier`, NOT in `QueryNodes`). Freshly-created types
   pay one full-scan QueryNodes before the first write — per ADR.
3. Pagination is offset-based; deep pages are O(offset). EPIC #407
   should decide whether to add keyset pagination now or defer.
4. `has_more = (len == limit)` lies on exact-multiple boundaries
   (next page can be empty). ACL post-filter makes it leakier.
5. Python swallows all exceptions to an empty response
   (`grpc_server.py:1379-1382`). Go MUST surface gRPC errors —
   behaviour change; flag in EPIC #407 release notes.
6. `total_count` is unused. Decide: populate via `COUNT(*)` (extra
   query) or remove proto field. Defer; Go leaves it 0 for parity.
7. `order_by` silently downgrades unknown values. Consider
   `INVALID_ARGUMENT` instead — behaviour change; pin current.
