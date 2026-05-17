# GetNodes — Go Port Spec

> Implementation: `server/go/internal/api/get_nodes.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407. Batch sibling of `GetNode`. Reference Python handler:
`server/go/internal/api/get_nodes.go`. Proto:
`proto/entdb/v1/entdb.proto:58, 409-422`.

## Wire contract

Request `entdb.v1.GetNodesRequest` (proto:409):
- `RequestContext context` (tenant_id, actor — actor is UNTRUSTED)
- `int32 type_id` — present in proto but **not consulted** by the handler;
  Python iterates `node_ids` and trusts whatever `type_id` is on disk
  (server/go/internal/api/get_nodes.go). Go port must preserve this (do NOT add a
  type_id filter — would break callers).
- `repeated string node_ids` — IDs to fetch. Empty list is legal and
  returns empty response.
- `string after_offset`, `int32 wait_timeout_ms` — read-after-write
  fence; default 30s when unset (server/go/internal/api/get_nodes.go).

Response `GetNodesResponse` (proto:419):
- `repeated Node nodes` — found AND access-allowed nodes. Order is
  **insertion order from the for-loop** (server/go/internal/api/get_nodes.go), which
  follows request `node_ids` order **minus** missing/denied entries.
  Effectively: nodes are emitted in request order, but indexes are NOT
  parallel to `node_ids` (denied/missing are absent, not nil-padded).
- `repeated string missing_ids` — IDs that were not-found OR
  access-denied (cross-tenant). The two cases are **indistinguishable**
  on the wire — by design.

Batch semantics summary:
- Order: request order, gaps closed.
- Skip missing: yes, into `missing_ids`.
- All-or-nothing: no. Partial success is the norm.
- Duplicates in `node_ids`: not deduped; each lookup runs independently
  (Python loops verbatim). Go port must match.

## Auth

- Tenant existence: `_check_tenant` (server/go/internal/api/get_nodes.go, 362).
- Trusted actor: `_trusted_actor(request.context.actor)` — actor from
  payload is OVERWRITTEN with `AuthInterceptor.get_authoritative_actor`
  (server/go/internal/api/get_nodes.go, 418, 586-588). This is the privilege-escalation
  fix; the Go port must read the trusted identity from the gRPC
  context/metadata via the AuthInterceptor equivalent and ignore the
  request-payload actor for any auth decision. See
  `tests/python/integration/test_privilege_escalation.py:213-228`.
- Tenant membership / cross-tenant: `_check_cross_tenant_read`
  (server/go/internal/api/get_nodes.go, 561-618). Returns `"local"` (no global_store),
  `"member"`, `"cross_tenant"`, or aborts `PERMISSION_DENIED`.
- Per-node ACL (cross-tenant only): when role is `cross_tenant`, each
  individual node is checked via `canonical_store.can_access(tenant,
  node_id, actor_ids)` (server/go/internal/api/get_nodes.go). Members bypass
  per-node ACL entirely. Denied nodes are folded into `missing_ids`,
  NOT surfaced as PERMISSION_DENIED — this is a deliberate
  information-leak guard: a cross-tenant actor cannot probe whether a
  node exists vs whether they lack access.
- Capability: `READ` ((legacy Python unit test, removed in Phase 4D)).

## Side effects

None on the happy path. Read-only. Metric `record_grpc_request("GetNodes",
"ok"|"error", elapsed)` (server/go/internal/api/get_nodes.go, 1289). No WAL append, no
SQLite write, no audit row. The `after_offset` fence may block up to
`wait_timeout_ms` (default 30s) waiting for the applier to catch up
(server/go/internal/api/get_nodes.go, `_wait_for_offset`).

## Error contract

| Condition | Python behaviour | Go port |
|---|---|---|
| Tenant missing/disabled | `_check_tenant` aborts `NOT_FOUND` / `FAILED_PRECONDITION` | match |
| Actor not member, no node_access | `_check_cross_tenant_read` aborts `PERMISSION_DENIED` (batch-level — fails the whole call before any lookups) | match |
| Claimed-actor mismatch (payload says admin, trusted is user:eve) | `PERMISSION_DENIED` from `_check_cross_tenant_read` after substitution | match — `test_privilege_escalation.py:218-228` |
| Some node_ids not in store | placed in `missing_ids`; call returns OK | match |
| Some node_ids exist but cross-tenant actor lacks ACL | placed in `missing_ids`; call returns OK | match — must NOT distinguish from not-found |
| All node_ids missing | OK, `nodes=[]`, `missing_ids=<all>` | match |
| Empty `node_ids` | OK, both lists empty | match |
| `after_offset` not reached within timeout | currently swallowed by outer `except` and degrades to "all missing" (server/go/internal/api/get_nodes.go) | **see Open Questions** |
| Internal error (SQLite, panic) | bare `except`: log + return `nodes=[], missing_ids=<all request ids>` with status OK (server/go/internal/api/get_nodes.go). Effectively swallows errors as "everything missing" | preserve for parity in v1; flag for review (this masks data-loss bugs) |

Partial-failure mode is the key invariant: **GetNodes never returns a
non-OK gRPC status for per-id failures.** Auth and tenant-existence
errors are batch-level and abort the whole call.

## Shared Go package deps

- `internal/auth` — `AuthInterceptor`, `GetAuthoritativeActor(ctx)`,
  trusted-actor extraction from gRPC metadata.
- `internal/tenant` — `CheckTenant`, `CheckCrossTenantRead` (returns
  `RoleLocal | RoleMember | RoleCrossTenant`).
- `internal/canonical` — `Store.GetNode(tenantID, nodeID)`,
  `Store.CanAccess(tenantID, nodeID, actorIDs)`,
  `Store.ResolveActorGroups(tenantID, actor)`.
- `internal/applier` — `WaitForOffset(tenantID, offset, timeout)`.
- `internal/wire` — `nodeToProto` (the `payload`/`acl` conversion that
  Python does at server/go/internal/api/get_nodes.go; field-IDs preserved on disk
  per CLAUDE.md invariant 6).
- `internal/metrics` — `RecordGRPCRequest`.
- Standard `google.golang.org/grpc/status`, `codes`.

## Other-RPC deps

- Shares helpers with `GetNode` (rpcs/GetNode.md), `QueryNodes`,
  `GetNodeByKey` (which delegates to `GetNode`, server/go/internal/api/get_nodes.go).
- Same auth path as every read RPC; spec the helpers in
  `docs/go-port/shared/auth.md` and `docs/go-port/shared/tenant.md`
  rather than re-deriving here.
- No dependency on write RPCs at runtime, but contract tests seed via
  `ExecuteAtomic`.

## Contract tests pinning behavior

- (legacy Python unit test, removed in Phase 4D) — id-keyed
  payload on the wire (field-IDs, not names).
- (legacy Python unit test, removed in Phase 4D) (file header) and
  cross-tenant filter cases at lines 254-407 — covers
  `cross_tenant` per-node filtering that GetNodes inherits.
- (legacy Python unit test, removed in Phase 4D) — RPC mapped to
  `CoreCapability.READ`.
- `tests/python/integration/test_grpc_contract.py:223-233` — happy-path
  with one hit + one miss returns at least the hit; missing IDs
  tolerated.
- `tests/python/integration/test_privilege_escalation.py:213-228` —
  claimed-admin actor in payload is rejected with `PERMISSION_DENIED`
  when trusted identity is a regular user.

Go port adds (`tests/contract/`): order preservation, duplicate
node_ids, empty list, cross-tenant per-node filtering folds into
`missing_ids` (does NOT abort), `after_offset` fence behaviour.

## Implementation outline

```go
func (s *Server) GetNodes(ctx context.Context, req *pb.GetNodesRequest) (*pb.GetNodesResponse, error) {
    start := time.Now()
    defer func() { metrics.RecordGRPCRequest("GetNodes", status, time.Since(start)) }()

    if err := s.checkTenant(ctx, req.Context.TenantId); err != nil { return nil, err }
    trusted := auth.GetAuthoritativeActor(ctx)              // ignore req.Context.Actor
    role, err := s.checkCrossTenantRead(ctx, req.Context.TenantId, trusted)
    if err != nil { return nil, err }                       // PERMISSION_DENIED batch-level

    if req.AfterOffset != "" {
        timeout := 30 * time.Second
        if req.WaitTimeoutMs > 0 { timeout = time.Duration(req.WaitTimeoutMs) * time.Millisecond }
        _ = s.applier.WaitForOffset(ctx, req.Context.TenantId, req.AfterOffset, timeout)
    }

    var actorIDs []string
    if role == tenant.RoleCrossTenant {
        actorIDs, _ = s.store.ResolveActorGroups(ctx, req.Context.TenantId, trusted)
    }

    // Bounded concurrency for per-id lookups (parity perf, not parity behaviour).
    // Cap = min(len(node_ids), maxFanout=32). Preserve request order on output.
    nodes := make([]*pb.Node, 0, len(req.NodeIds))
    missing := make([]string, 0)
    sem := make(chan struct{}, 32)
    results := make([]*canonical.Node, len(req.NodeIds))
    denied := make([]bool, len(req.NodeIds))
    var wg sync.WaitGroup
    for i, id := range req.NodeIds {
        i, id := i, id
        wg.Add(1); sem <- struct{}{}
        go func() {
            defer func() { <-sem; wg.Done() }()
            n, _ := s.store.GetNode(ctx, req.Context.TenantId, id)
            if n == nil { return }
            if actorIDs != nil {
                ok, _ := s.store.CanAccess(ctx, req.Context.TenantId, id, actorIDs)
                if !ok { denied[i] = true; return }
            }
            results[i] = n
        }()
    }
    wg.Wait()
    for i, id := range req.NodeIds {
        if results[i] == nil || denied[i] { missing = append(missing, id); continue }
        nodes = append(nodes, wire.NodeToProto(results[i]))
    }
    return &pb.GetNodesResponse{Nodes: nodes, MissingIds: missing}, nil
}
```

Notes on performance & limits:
- **Bounded concurrency**: cap at 32 in-flight `GetNode` calls per request
  to avoid SQLite-pool starvation under a 10k-id batch. Python is
  serial; Go can parallelise without changing behaviour because
  per-id lookups are independent.
- **Message-size limit**: the response can blow past gRPC's default 4
  MiB if the batch is large. Set a server-side guardrail
  (`maxBatchNodeIDs`, default 1000) and abort
  `RESOURCE_EXHAUSTED` if `len(req.NodeIds) > maxBatchNodeIDs` BEFORE
  doing any I/O. Python has no such cap — flag as a hardening delta in
  the porting changelog.
- **grpc.aio invariant**: handler must be a method on the async-gRPC
  server (`grpc-go` is async by default; no special action — but no
  goroutine should block on a sync sqlite call without going through
  the canonical store's pool).

## Open questions / risks

1. **`after_offset` timeout swallowed**: the bare `except` at
   server/go/internal/api/get_nodes.go turns an applier-fence timeout into "all ids
   missing" with status OK. Should the Go port match (parity) or
   surface `DEADLINE_EXCEEDED`? Recommend: match for v1, file
   follow-up issue.
2. **`type_id` field is ignored**: proto:411 declares `type_id` but
   the handler never reads it. Either (a) start enforcing it (breaking
   change — needs deprecation cycle) or (b) document as advisory and
   add a contract test that mismatched type_id still returns the node.
   Recommend (b) for v1.
3. **Information leak via timing**: cross-tenant denied vs not-found
   are merged into `missing_ids`, but per-id latency may differ
   (denied requires `can_access` round-trip after `get_node`). Low
   risk; out of scope.
4. **Batch size cap**: Python has none. Go port should add
   `RESOURCE_EXHAUSTED` at e.g. 1000 ids — confirm with EPIC #407 owner.
5. **Duplicate node_ids**: Python emits the same node twice if
   requested twice. Match exactly, or dedupe? Recommend: match.
6. **Per-node ACL cache**: 32-way fan-out hits `can_access` per id.
   For high-cardinality batches consider a single bulk
   `CanAccessMany(tenant, ids, actorIDs)` — out of scope for parity v1.
