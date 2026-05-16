# RPC Port Spec — `entdb.v1.EntDBService/GetConnectedNodes`

> Implementation: `server/go/internal/api/get_connected_nodes.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407 — Python → Go server port. Source of truth: Python handler at
`server/go/internal/api/get_connected_nodes.go`. Storage layer at
`server/go/internal/store/` (sync core)
and `:3424-3438` (async wrapper).

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:91` (rpc), `:707-718` (messages).

`GetConnectedNodesRequest`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `context` | 1 | `RequestContext` | tenant_id + actor (UNTRUSTED — see Auth). |
| `node_id` | 2 | `string` | Source node — the "from" side of the edge join. Required (empty string is a vacuous query, returns `[]`). |
| `edge_type_id` | 3 | `int32` | Single edge-type filter. Mandatory; there is no "all edge types" sentinel. |
| `limit` | 4 | `int32` | `<= 0` → coerce to `100` (`server/go/internal/api/get_connected_nodes.go`). No upper cap today (open question below). |
| `offset` | 5 | `int32` | `<= 0` → coerce to `0` (`server/go/internal/api/get_connected_nodes.go`). |

`GetConnectedNodesResponse`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `nodes` | 1 | `repeated Node` | Connected nodes ordered by `n.created_at DESC` (`server/go/internal/store/, 3358, 3405`). Payload is **field-id-keyed** (`{"1": "..."}`), not name-keyed — see (legacy Python unit test, removed in Phase 4D). |
| `has_more` | 2 | `bool` | `len(nodes) == limit` (`server/go/internal/api/get_connected_nodes.go`). Heuristic only — not a true "more rows exist" signal; clients must page until they get a short page. |

**Direction: outbound only.** Python's storage path joins `edges e ON e.to_node_id = n.node_id` filtered by `e.from_node_id = node_id`
(`server/go/internal/store/, 3338-3341, 3400-3404`). To traverse the inverse direction, callers use a different RPC; this one does NOT honour a `direction` param.

**Depth: 1 (single hop).** This is NOT a multi-hop traversal — the SQL is a single `nodes JOIN edges` and never recurses. There is no `depth` field on the request and no cycle handling in the handler (the storage layer simply has no recursion to cycle on). Multi-hop discovery is a client concern; see Open Questions.

## Auth

- **Authentication required.** Standard `AuthInterceptor` path; method is **not** in `UNAUTHENTICATED_METHODS`.
- **Trusted-actor invariant** (CLAUDE.md issue #168): handler immediately rebinds the actor at `server/go/internal/api/get_connected_nodes.go` via `self._trusted_actor(request.context.actor)` (defined `:418-437`), which delegates to `get_authoritative_actor` — the `ContextVar` set by `AuthInterceptor` ALWAYS wins over `request.context.actor`. Go port MUST mirror: never trust the wire-actor for any downstream check.
- **Group expansion**: `canonical_store.resolve_actor_groups(tenant_id, trusted_actor)` (`server/go/internal/api/get_connected_nodes.go`) returns the principal's transitive group IDs. The result list is what flows into `actor_ids` for ACL filtering.
- **Per-node ACL filter across the traversal**, two-stage:
  1. **Source gate** (`server/go/internal/store/`): if the actor cannot access `node_id`, return `[]` immediately. Pinned by (legacy Python unit test, removed in Phase 4D).
  2. **Target filter**, branches on the edge type's `propagates_acl` flag (`server/go/internal/store/`):
     - `propagates_acl=1` → "fast path": every connected node is visible **except** those with an explicit `node_access.permission='deny'` for any of `actor_ids` (`server/go/internal/store/`). Pinned by `test_acl_v2.py:496-507`.
     - `propagates_acl=0` → "slow path": each child must independently satisfy `owner_actor IN actor_ids` OR exist in `node_visibility` for the principal OR have a non-deny `node_access` row (`server/go/internal/store/`).
- **System actor bypass**: when `SYSTEM_ACTOR` (server-internal service identity) appears in `actor_ids`, ACL is skipped entirely (`server/go/internal/store/`). Pinned by `test_acl_v2.py:517-522`. Go port: keep this — it is how `Applier`-driven re-fanouts read graph state.
- **No capability/Permission check**. The RPC is read-only; the gate is "read access on source" + "not denied on target". There is no `Permission.READ_GRAPH`-style check.
- **Tenant gate**: `await self._check_tenant(...)` (`server/go/internal/api/get_connected_nodes.go`) — verifies tenant exists and is not suspended. Failure surfaces as `PermissionDenied` from the helper (Go port should preserve this code).

## Side effects

**Read-only.** No WAL append, no SQLite write, no `global_store` mutation, no quota debit, no audit-log entry. Architecturally invariant per CLAUDE.md §1: this RPC has no business writing the WAL.

**Potentially heavy.** The slow path (`propagates_acl=0`) executes a `nodes JOIN edges` plus three correlated `EXISTS` subqueries against `node_visibility` and `node_access`, each parameterised with the full `actor_ids` set. For a hot user in many groups against a fan-out parent with 100k children this can scan a lot. The handler does NOT charge a quota and does NOT timeout the SQL — Go port should add a `context.Context` deadline pass-through to the SQL driver (Python lacks this) but MUST NOT add a behavior change beyond cancellation.

In-order narration the Go handler should preserve:
1. `start := time.Now()`.
2. `_check_tenant(ctx, request.context.tenant_id)` — tenant existence/status gate.
3. `trusted := trusted_actor(request.context.actor)` (read from interceptor ContextVar equivalent — `auth.AuthoritativeActor(ctx)`).
4. `actorIDs := canonicalStore.ResolveActorGroups(ctx, tenantID, trusted)`.
5. `limit := request.Limit; if limit <= 0 { limit = 100 }`.
6. `offset := request.Offset; if offset < 0 { offset = 0 }`.
7. `nodes, err := canonicalStore.GetConnectedNodes(ctx, tenantID, request.NodeId, request.EdgeTypeId, actorIDs, limit, offset)`.
8. `record_grpc_request("GetConnectedNodes", "ok"|"error", elapsed)` — both arms.
9. Return `&pb.GetConnectedNodesResponse{Nodes: protoNodes, HasMore: len(nodes) == limit}`.

## Error contract

| gRPC code | Trigger | Notes |
|-----------|---------|-------|
| `OK` | Happy path, including empty result. Source-not-accessible returns `OK` with `nodes=[]` — do NOT leak existence via a different code. |
| `OK` (degraded) | **Any** handler exception falls into the broad `except Exception` at `server/go/internal/api/get_connected_nodes.go` and returns `GetConnectedNodesResponse(nodes=[])`. This is the **current Python contract** — pinned by `tests/python/integration/test_grpc_contract.py:255-266` (happy seed-empty case returns `nodes=[]` cleanly). The contract test does NOT explicitly require swallowing internal errors, but Go port should preserve "empty list on internal error" to match observed client behavior; log at error severity. |
| `PERMISSION_DENIED` | Raised by `_check_tenant` for unknown / suspended tenant — surfaces from the helper, gets caught by the broad `except` and rewritten to empty list in Python. Recommended Go port: let `PermissionDenied` propagate (do NOT swallow tenant-gate errors); only swallow the SQL/decode errors. Track as a **deliberate behavior tightening** in the EPIC. |
| `UNAUTHENTICATED` | Auth interceptor — never reached by handler. |
| `INVALID_ARGUMENT` | Currently never raised. `node_id=""` and `edge_type_id=0` both yield empty result via SQL filtering, not a 4xx. Preserve. |

## Shared Go package deps

New packages under `server/go/internal/...` unless noted.

- `pb` (`server/go/internal/pb/entdbv1`) — generated `GetConnectedNodesRequest`, `GetConnectedNodesResponse`, `Node`, `RequestContext`. Required.
- `auth` — `AuthoritativeActor(ctx) string` (mirrors `auth_interceptor.get_authoritative_actor`). Required.
- `canonicalstore` — must export `ResolveActorGroups(ctx, tenantID, actor) ([]string, error)`, `GetConnectedNodes(ctx, tenantID, nodeID string, edgeTypeID int32, actorIDs []string, limit, offset int32) ([]Node, error)`, plus internal `canAccess` helper (`server/go/internal/store/`). The Go SQL builder MUST mirror the two SQL branches (propagates / does-not-propagate) — see Implementation outline.
- `tenantgate` — `CheckTenant(ctx, tenantID) error` (mirrors `_check_tenant` at `server/go/internal/api/get_connected_nodes.go`).
- `metrics` — `RecordGRPCRequest("GetConnectedNodes", status, dur)`.
- `proto` (internal) — `nodeToProto(n Node) *pb.Node` mirroring `_node_to_proto` (`server/go/internal/api/get_connected_nodes.go`); payload must be **id-keyed** (`test_payload_wire_format.py:13-25`).

NOT used and MUST NOT be imported: `wal`, `apply`, `audit`, `quota`, `crypto`. Read-only RPC.

## Other-RPC deps

- Shares the SQL infrastructure used by `GetEdgesFrom` / `GetEdgesTo` (the `edges` table join + `from_node_id` / `to_node_id` indexing). `GetConnectedNodes` is essentially `GetEdgesFrom` followed by an `IN (target_ids)` `nodes` lookup, fused into a single SQL — there is no shared Go function today, but the Go port should extract `edgesFromQuery(...)` so all three RPCs build identical join plans. See companion specs `GetEdgesFrom.md` and `GetEdgesTo.md` (TODO under same EPIC).
- Shares ACL filter primitives with `ListSharedWithMe`, `GetNode`, `QueryNodes` — `node_access` deny-check, `node_visibility` lookup, `owner_actor` match. Factor into `canonicalstore/acl.go`.
- Depends on `ResolveActorGroups` semantics also used by `GetNode`, `Share/RevokeAccess`, `ListSharedWithMe`. Port that helper FIRST.

## Contract tests pinning behavior

- `tests/python/integration/test_grpc_contract.py:255-266` — happy path over real gRPC channel: empty graph returns `nodes=[]`. The Go server must pass this verbatim once stubs are swapped.
- (legacy Python unit test, removed in Phase 4D) — propagates_acl=1, source shared, returns both children.
- (legacy Python unit test, removed in Phase 4D) — explicit deny on a child filters it out even when the edge propagates.
- (legacy Python unit test, removed in Phase 4D) — no access to source → empty result (does NOT leak existence).
- (legacy Python unit test, removed in Phase 4D) — `SYSTEM_ACTOR` bypasses ACL completely.
- (legacy Python unit test, removed in Phase 4D) — pagination with limit=5, offset=5 returns disjoint pages.
- `tests/python/benchmarks/bench_acl.py:104, 114` — perf budget for hot/cold paths (30/31 children); Go port should beat or match.
- (legacy Python unit test, removed in Phase 4D) — SDK delegate signature (`tenant_id`, `actor`, `node_id`, `edge_type_id`); kwargs locked.
- (legacy Python unit test, removed in Phase 4D) — payload is field-id-keyed on the wire, not name-keyed. Mandatory regression guard.

## Implementation outline

```go
// server/go/internal/api/get_connected_nodes.go
func (s *EntDBServer) GetConnectedNodes(ctx context.Context, req *pb.GetConnectedNodesRequest) (*pb.GetConnectedNodesResponse, error) {
    start := time.Now()
    status := "ok"
    defer func() { metrics.RecordGRPCRequest("GetConnectedNodes", status, time.Since(start)) }()

    if err := s.tenantGate.CheckTenant(ctx, req.Context.TenantId); err != nil {
        status = "error"; return nil, err // PERMISSION_DENIED — propagate, do NOT swallow
    }
    trusted := auth.AuthoritativeActor(ctx, req.Context.Actor)
    actorIDs, err := s.canon.ResolveActorGroups(ctx, req.Context.TenantId, trusted)
    if err != nil { status = "error"; return &pb.GetConnectedNodesResponse{}, nil } // match Python swallow

    limit := req.Limit; if limit <= 0 { limit = 100 }
    offset := req.Offset; if offset < 0 { offset = 0 }

    nodes, err := s.canon.GetConnectedNodes(ctx, req.Context.TenantId, req.NodeId, req.EdgeTypeId, actorIDs, limit, offset)
    if err != nil { status = "error"; log.Error(...); return &pb.GetConnectedNodesResponse{}, nil }

    out := make([]*pb.Node, len(nodes))
    for i, n := range nodes { out[i] = nodeToProto(n) }
    return &pb.GetConnectedNodesResponse{Nodes: out, HasMore: int32(len(nodes)) == limit}, nil
}
```

Storage `GetConnectedNodes` mirrors `server/go/internal/store/` exactly:
1. If `slices.Contains(actorIDs, SystemActor)` → run unfiltered SQL (`server/go/internal/store/`).
2. Source gate: `canAccess(tenantID, nodeID, actorIDs)` — empty list short-circuit.
3. Lookup `propagates_acl` for the edge type via the seed `from_node_id` join (`server/go/internal/store/`). Fallback to `false` when no edge row exists.
4. Branch the SQL: fast path uses `NOT EXISTS deny`; slow path uses `(owner OR visibility OR non-deny access)` triple-EXISTS. Use named parameters / `sqlx.In` to expand `actorIDs` placeholders rather than naive string concat.
5. Map rows → `Node` via `Node.from_row` equivalent — payload deserialised from `payload_json` blob, ACL from `acl_blob`. Must remain field-id-keyed.

## Open questions / risks

- **No upper bound on `limit`.** A pathological client can request `limit=2_000_000_000`. Python relies on SQLite's int handling; Go must clamp (`min(limit, 1000)` is the convention used elsewhere). Verify against benchmark expectations before clamping — clamping changes wire behavior and needs an EPIC-tracked decision.
- **`has_more` is a heuristic.** When `len(nodes) == limit` and the underlying graph happens to have exactly `limit` matches, `has_more=true` lies. Document; do not fix in this RPC (would require an extra `LIMIT n+1` round-trip and break the contract test).
- **Cycle handling: N/A at depth=1**, but if the EPIC adds a `depth` field later, the SQL becomes recursive and needs a visited set. Track separately; do NOT add depth>1 in the Go port without a fresh proto rev.
- **Result-size memory cap.** `nodes JOIN edges` materialises every row into Go heap before serialisation. For `limit=100` this is fine; if the cap is raised, switch to a streaming server-side cursor + `grpc.ServerStream` (would require a proto break — out of scope here).
- **Slow-path fan-out** with a user in 200+ groups expands into a 600+ placeholder SQL. SQLite's parser cap is ~32k params per statement; risk surfaces only at extreme scale, but Go port should batch when `len(actorIDs) > 500`.
- **Payload-key contract drift.** `_node_to_proto` emits `_dict_to_struct(n.payload)` — if `n.payload` is name-keyed in storage (legacy rows), the wire becomes name-keyed and silently breaks the Go SDK. Add a Go-side guard: assert keys are numeric strings; log+alert otherwise.
- **`PermissionDenied` swallowing.** Python's broad `except` masks tenant-gate failures as empty results. Recommended Go behavior: propagate `PermissionDenied`; coordinate with EPIC #407 owner because clients may rely on the empty-list signal.
