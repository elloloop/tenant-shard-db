# GetNode — Go Port Spec

> Implementation: `server/go/internal/api/get_node.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407. Single-node read by `node_id` within one tenant. Read-only.
Python source of truth: `server/go/internal/api/get_node.go`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:55` (RPC), `:394-407` (messages), `:454-472` (`Node`).

```
rpc GetNode(GetNodeRequest) returns (GetNodeResponse);

GetNodeRequest  { RequestContext context = 1; int32 type_id = 2;
                  string node_id = 3; string after_offset = 10;
                  int32 wait_timeout_ms = 11; }
GetNodeResponse { Node node = 1; bool found = 2; }
```

`Node.payload` is a `google.protobuf.Struct` keyed by **field_id strings**
(invariant #6), e.g. `{"1": "alice@example.com", "2": "$2a$..."}`. The
SDK does id→name translation; the server MUST NOT translate. Pinned by
(legacy Python unit test, removed in Phase 4D) and the wire
test (legacy Python unit test, removed in Phase 4D).

`type_id` is present on the request but the Python handler does **not**
use it to filter the lookup — `canonical_store.get_node(tenant, node_id)`
is keyed only on `(tenant_id, node_id)`. See "Open questions".

`after_offset` + `wait_timeout_ms` provide read-after-write consistency
by waiting for the WAL applier to reach a stream position before reading
(default 30s when `wait_timeout_ms == 0`). Implementation:
`server/go/internal/api/get_node.go` → `_wait_for_offset` (`:937-944`) →
`canonical_store.wait_for_offset`.

## Auth

1. **Trusted actor only.** `request.context.actor` is **untrusted on the
   wire**. Handler immediately rebinds via `_trusted_actor`
   (`server/go/internal/api/get_node.go`) which calls
   `auth.auth_interceptor.get_authoritative_actor` (a `ContextVar` set by
   `AuthInterceptor`). Payload-supplied `system:admin` from a regular
   user is downgraded to the interceptor identity — pinned by
   `tests/python/integration/test_privilege_escalation.py:186-209`.

2. **Tenant residency.** `_check_tenant` (`server/go/internal/api/get_node.go`):
   - sharding registry: tenant not owned by this node → `UNAVAILABLE`
     with `entdb-redirect-node` trailer.
   - region pinning: tenant region ≠ served region →
     `FAILED_PRECONDITION` (no retry).

3. **Membership / cross-tenant grant.** `_check_cross_tenant_read`
   (`server/go/internal/api/get_node.go`):
   - no `global_store` → returns `"local"` (back-compat).
   - `system:*` / `__system__` → `"member"`.
   - `is_member(tenant, user_id)` → `"member"`.
   - else if any `node_access` row in tenant for this actor → `"cross_tenant"`.
   - else `PERMISSION_DENIED`.

4. **Per-node ACL (cross-tenant only).** When role is `"cross_tenant"`,
   re-check the specific `node_id` against `canonical_store.can_access`
   (`server/go/internal/api/get_node.go`, `server/go/internal/store/`). Failure →
   `PERMISSION_DENIED`.

5. **Capability mapping.** `GetNode → CoreCapability.READ`. Fixed at
   `server/go/internal/auth/capability_registry.go`. Pinned by
   (legacy Python unit test, removed in Phase 4D) and
   (legacy Python unit test, removed in Phase 4D). Note: the current
   Python `GetNode` does **not** consult `required_for_op` at the handler
   level — capability enforcement is via the membership/ACL chain above.
   Go port should preserve this exact shape (do not add a new capability
   gate without an issue).

## Side effects

**None.** Read-only. No `wal.append`, no SQLite writes. Only:
- `canonical_store.get_node` (per-tenant SQLite read, invariant #4).
- `canonical_store.resolve_actor_groups` + `can_access` (per-tenant SQLite
  reads) on the cross-tenant path.
- `global_store.is_member` / `get_tenant` (global SQLite read) via
  `_check_cross_tenant_read` and `_check_tenant`.
- `canonical_store.wait_for_offset` (WAL apply barrier; passive).
- `record_grpc_request("GetNode", "ok"|"error", duration)` metric.

Per-tenant SQLite isolation is preserved: every `canonical_store` call is
scoped by `tenant_id` and uses `_get_connection(tenant_id)`
(`server/go/internal/store/`).

## Error contract — NOT_FOUND vs PERMISSION_DENIED

The handler returns ordered checks; **first failure wins**:

1. `_check_tenant` → `UNAVAILABLE` (wrong shard) or `FAILED_PRECONDITION`
   (region pin). With redirect trailer when applicable.
2. `_check_cross_tenant_read` → `PERMISSION_DENIED` for non-members
   without a node_access grant. **This fires before the row lookup**, so
   a non-member querying a non-existent node sees `PERMISSION_DENIED`,
   not `found=false`.
3. `canonical_store.get_node` returns `None` → response with `found=False`,
   gRPC status `OK`. **No abort.** Pinned by
   `tests/python/integration/test_grpc_contract.py:217-222` (mode
   `not_found`, `r.found is False`).
4. Cross-tenant role + missing per-node grant → `PERMISSION_DENIED`
   (`server/go/internal/api/get_node.go`). Note: the row exists at this point, so
   "node missing" cannot be distinguished from "no grant" by an attacker
   who lacks tenant membership — that is intentional.

**Quirk to preserve.** The handler wraps the body in `try/except Exception`
(`:1061-1064`): any uncaught exception (including the `_AbortError` raised
by `context.abort` in tests) is logged and converted to
`GetNodeResponse(found=False)`. In real `grpc.aio` runtime, `context.abort`
terminates the call **before** this catch fires, so clients see the abort
status — but unit tests that mock context observe `found=False` plus a
recorded `ctx.abort_code`. Pinned by
`tests/python/integration/test_privilege_escalation.py:196-209`.

The Go port should:
- Use a real abort path (`status.Errorf`) — not a try/catch swallow. The
  catch is a Python defensive artifact; aborts in `grpc-go` are
  terminal and the swallow is unreachable.
- Keep the metric labels: `{"GetNode","ok"}` for both happy and
  not-found, `{"GetNode","error"}` only for unexpected exceptions.

## Shared Go package deps

(All under `server/go/internal/...` per the planned layout — paths are
proposals, replace with whatever EPIC #407 freezes.)

- `auth.AuthoritativeActor(ctx) string` — equivalent to
  `get_authoritative_actor`. Pulls from a request-scoped value set by
  the auth interceptor.
- `tenancy.CheckTenant(ctx, tenantID) error` — sharding redirect +
  region pin. Returns a `status.Status` with the
  `entdb-redirect-node` trailer attached when applicable.
- `tenancy.CheckCrossTenantRead(ctx, tenantID, actor) (Role, error)` —
  returns `RoleLocal | RoleMember | RoleCrossTenant` or
  `PERMISSION_DENIED`.
- `canonical.Store.GetNode(ctx, tenantID, nodeID) (*Node, error)` —
  per-tenant SQLite read. Returns `(nil, nil)` for missing.
- `canonical.Store.ResolveActorGroups`, `canonical.Store.CanAccess`.
- `canonical.Store.WaitForOffset(ctx, tenantID, pos, timeout) (bool, error)`.
- `wire.NodeToProto(n *canonical.Node) *pb.Node` — must produce the
  id-keyed `Struct` payload. Single source of truth so `GetNode`,
  `GetNodes`, `QueryNodes`, `GetNodeByKey` agree.
- `metrics.RecordGRPCRequest(method, status string, dur time.Duration)`.

## Other-RPC deps

- **`GetNodeByKey`** (`server/go/internal/api/get_node.go`) delegates to `GetNode`
  after a unique-index lookup. The Go `GetNode` must remain the single
  authoritative read entry-point so ACL/cross-tenant/redirect logic is
  not duplicated. `GetNodeByKey` should call the **internal** Go
  `GetNode` (same package, not via the gRPC stub) once both are ported.
- **`GetNodes`** (`server/go/internal/api/get_node.go`) is the batch sibling and
  shares `_check_cross_tenant_read` + `wait_for_offset` + the wire
  payload encoding. Port them together to keep behaviour aligned.

## Contract tests pinning behavior

- `tests/python/integration/test_grpc_contract.py:212-216` — happy path:
  `found=True`, `node.node_id == SEED_NODE_ID`.
- `tests/python/integration/test_grpc_contract.py:217-222` — not_found
  returns `found=False` with no abort, status OK.
- (legacy Python unit test, removed in Phase 4D) — id-keyed
  payload on the wire (invariant #6).
- (legacy Python unit test, removed in Phase 4D) — wire round-trip
  via real `grpc.aio` stub.
- `tests/python/integration/test_privilege_escalation.py:186-209` —
  payload `actor` claim ignored; trusted identity wins;
  `PERMISSION_DENIED` for non-member.
- `(legacy Python unit test, removed), 412` —
  `GetNode → CoreCapability.READ`.
- (legacy Python unit test, removed in Phase 4D) — capability lookup.
- `tests/python/integration/test_redirect_cache.py:12-59` — sharding
  redirect via `entdb-redirect-node` trailer.
- `(legacy Python unit test, removed), 67` — auth interceptor binds
  trusted actor for `/entdb.EntDBService/GetNode`.
- (legacy Python unit test, removed in Phase 4D) — per-tenant rate-limit
  buckets keyed on `GetNode` method name.
- `(legacy Python unit test, removed), 112-121` — metric labels
  (`method="GetNode"`).

## Implementation outline

```go
func (s *EntDBServer) GetNode(
    ctx context.Context, req *pb.GetNodeRequest,
) (*pb.GetNodeResponse, error) {
    start := time.Now()
    defer func() { metrics.RecordGRPCRequest("GetNode", status, time.Since(start)) }()

    if err := s.tenancy.CheckTenant(ctx, req.Context.TenantId); err != nil {
        return nil, err // UNAVAILABLE / FAILED_PRECONDITION (with trailer)
    }
    actor := auth.AuthoritativeActor(ctx) // ignore req.Context.Actor

    role, err := s.tenancy.CheckCrossTenantRead(ctx, req.Context.TenantId, actor)
    if err != nil { return nil, err } // PERMISSION_DENIED

    if req.AfterOffset != "" {
        timeout := 30 * time.Second
        if req.WaitTimeoutMs > 0 { timeout = time.Duration(req.WaitTimeoutMs) * time.Millisecond }
        _, _ = s.canonical.WaitForOffset(ctx, req.Context.TenantId, req.AfterOffset, timeout)
    }

    node, err := s.canonical.GetNode(ctx, req.Context.TenantId, req.NodeId)
    if err != nil { return nil, status.Errorf(codes.Internal, "...") }
    if node == nil { return &pb.GetNodeResponse{Found: false}, nil }

    if role == tenancy.RoleCrossTenant {
        ids, _ := s.canonical.ResolveActorGroups(ctx, req.Context.TenantId, actor)
        ok, _ := s.canonical.CanAccess(ctx, req.Context.TenantId, req.NodeId, ids)
        if !ok {
            return nil, status.Error(codes.PermissionDenied,
                "Actor does not have access to this node")
        }
    }
    return &pb.GetNodeResponse{Found: true, Node: wire.NodeToProto(node)}, nil
}
```

## Open questions / risks

1. **`type_id` is unused.** Request carries `type_id` but the Python
   read ignores it; a node with `node_id="x"` of type 7 is returned
   even when `type_id=1` was requested. Preserve the bug-for-bug for
   contract test parity, but file a follow-up to either enforce
   `node.type_id == req.type_id` or remove the field.
2. **Exception swallow.** The Python `try/except` at `:1061-1064`
   masks errors as `found=False`. In Go we should return real errors
   (`Internal` for unexpected failures); contract tests only assert the
   abort/found-false pair on auth failures, which Go raises as real
   `PermissionDenied`. Confirm none of the unit tests assert
   `found=False` on a non-auth exception path.
3. **Cross-tenant role uses two queries.** `_check_cross_tenant_read`
   already calls `resolve_actor_groups` + `has_node_access`, then the
   handler **redoes** `resolve_actor_groups` + `can_access` for the
   specific node. The Go port should pass `actor_ids` through the role
   result to avoid the second resolve.
4. **`wait_for_offset` failure is silent.** Python ignores the bool
   return of `_wait_for_offset` for `GetNode`; if the offset never
   applies, the read returns stale (or `found=False`). Decision to
   preserve unless EPIC #407 says otherwise.
5. **`global_store is None` → `"local"`.** Back-compat path skips all
   membership checks. Ensure Go test fixtures construct a
   `global_store` for any test that asserts auth, or replicate the
   skip behaviour exactly.
6. **No-auth deployments.** When `AuthInterceptor` did not run,
   `get_authoritative_actor` falls back to the request-supplied actor.
   Go port must mirror this for unit-test ergonomics; document the
   expected production deployment requires the interceptor.
