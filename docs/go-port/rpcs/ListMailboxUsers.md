# RPC Port Spec — `entdb.v1.EntDBService/ListMailboxUsers`

> Implementation: `server/go/internal/api/list_mailbox_users.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407 — Python -> Go server port. Source of truth: Go handler at
`server/go/internal/api/list_mailbox_users.go`.

**Status: deprecated stub.** The legacy per-user mailbox SQLite store has been
removed. The handler is retained for proto compatibility and unconditionally
returns an empty `user_ids` list. The Go port MUST replicate this stub
verbatim — adding a real implementation is a contract change and out of scope
for #407.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:85` (rpc), `:621-629` (messages).

`ListMailboxUsersRequest`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `tenant_id` | 1 | `string` | Required for tenant routing. No `RequestContext`, no `actor` field on this message. |

`ListMailboxUsersResponse`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `user_ids` | 1 | `repeated string` | Always empty in current contract. Must be returned as `[]`, NOT a nil/null repeated. Pinned by `test_grpc_contract.py:286`. |

Full method name on the wire: `/entdb.v1.EntDBService/ListMailboxUsers`
(see `server/go/internal/api/generated/entdb_pb2_grpc.go`,
`sdk/go/entdb/internal/pb/entdb_grpc.pb.go:70`).

## Auth (trusted-actor; admin scope?)

- **Authenticated.** NOT in
  `AuthInterceptor.UNAUTHENTICATED_METHODS`
  (`server/go/internal/auth/interceptor.go` — that frozenset
  contains only `Health` and `grpc.health.v1.Health/Check`). The auth
  interceptor runs and a missing/invalid identity aborts before the handler
  is reached.
- **No admin scope required.** The handler does not call
  `_is_admin_or_system`, `_check_tenant_access`, or any ACL helper. Any
  authenticated identity that the tenant-routing check accepts may invoke it.
- **Trusted-actor invariant: N/A.** `ListMailboxUsersRequest` carries no
  `actor` and no `RequestContext`. There is nothing on the wire to forge,
  so the CLAUDE.md rule "wire `request.actor` is UNTRUSTED" has no surface
  here — but the Go handler MUST NOT introduce any actor parameter or trust
  any metadata-derived identity for authorization (the empty-list contract
  is identity-independent today).
- **Tenant routing.** The handler does call `_check_tenant(request.tenant_id, context)`
  (`server/go/internal/api/list_mailbox_users.go`), which performs the standard sharding-owner /
  region-pinning checks (`server/go/internal/api/list_mailbox_users.go`). This is the only
  pre-condition gate.
- **Rate limiting.** Goes through the standard `RateLimitInterceptor`
  (no bypass). Go port must keep this RPC inside the tenant bucket like
  every other tenant-scoped RPC.

## Side effects (read-only)

**None.** The handler:
1. Awaits `_check_tenant(request.tenant_id, context)` — may abort with
   `UNAVAILABLE` or `FAILED_PRECONDITION` (see error contract). This call
   may read from `global_store.get_tenant()` for region pinning
   (`server/go/internal/api/list_mailbox_users.go`), but is a side-effect-free read.
2. Returns `ListMailboxUsersResponse(user_ids=[])`.

No WAL append, no SQLite read, no `canonical_store` access, no `global_store`
write, no quota charge, no schema lookup, no audit log entry. This RPC MUST
NOT touch the WAL — even reads of tenant existence go through `global_store`,
not `wal.append`.

Note: the current Python handler does NOT call `record_grpc_request` (unlike
most other handlers). The Go port SHOULD record metrics for observability,
but this is an additive divergence, not a contract break.

## Error contract

| gRPC code | Trigger | Source |
|-----------|---------|--------|
| `OK` | Always, for any authenticated caller whose tenant routes to this node. Response is `{user_ids: []}`. | Pinned by `test_grpc_contract.py:281-287` (mode `happy`). |
| `UNAUTHENTICATED` | Auth interceptor rejects bad/missing credentials before handler runs. | `auth_interceptor.py` — generic, not handler-specific. |
| `UNAVAILABLE` | Tenant is owned by another node (sharding). Trailer `entdb-redirect-node` carries the owner. | `server/go/internal/api/list_mailbox_users.go`. |
| `FAILED_PRECONDITION` | Tenant pinned to a different region than the node serves. | `server/go/internal/api/list_mailbox_users.go`. |
| `RESOURCE_EXHAUSTED` | Per-tenant rate limit exceeded. | `RateLimitInterceptor` — generic. |
| `INTERNAL` | Unhandled panic. The current Python stub has no `try/except` and would let exceptions propagate; in practice none can fire after `_check_tenant`. Go port: `defer recover()` -> `status.Errorf(codes.Internal, ...)`. | n/a |

The handler MUST NOT abort with `INVALID_ARGUMENT`, `PERMISSION_DENIED`, or
`NOT_FOUND`. An empty `tenant_id` is currently NOT validated at this RPC
(it falls through `_check_tenant` and returns an empty list); the Go port
MUST preserve this behavior to keep contract-test parity unless EPIC #407
explicitly tightens it.

## Shared Go package deps

Each is a package under `server/go/internal/...` unless noted.

- `pb` (`server/go/internal/pb/entdbv1`) — generated `ListMailboxUsersRequest`,
  `ListMailboxUsersResponse`, servicer interface. Required.
- `tenantroute` — `CheckTenant(ctx, tenantID) error` mirroring
  `_check_tenant` (`server/go/internal/api/list_mailbox_users.go`). Must surface `UNAVAILABLE`
  with `entdb-redirect-node` trailer and `FAILED_PRECONDITION` for
  region-pin mismatch. Shared with every tenant-scoped RPC. Spec it in
  `docs/go-port/shared/tenant-route.md` (separate ticket).
- `metrics` — `RecordGRPCRequest(method, status, dur)` (mirrors
  `metrics.py:103`). Optional for parity, recommended.

NOT used and MUST NOT be imported by this handler: `auth`, `acl`, `apply`,
`canonicalstore`, `globalstore` (direct), `schema`, `wal`, `errs`, `quota`,
`crypto`, `audit`. Importing any signals scope creep (e.g. someone trying
to "implement" the deprecated mailbox-user enumeration — don't).

## Other-RPC deps

- `GetMailbox` (`server/go/internal/api/list_mailbox_users.go+`) — companion deprecated stub, also
  returns empty (`test_grpc_contract.py:274-279`). Same lifecycle: ported as
  a stub, not a real implementation. This RPC's spec parallels `GetMailbox`'s
  spec; keep both consistent.
- `SearchMailbox` (`server/go/internal/api/list_mailbox_users.go`, `test_grpc_contract.py:265-273`) — third
  member of the deprecated mailbox triad; same stub treatment.

`ListMailboxUsers` itself is a leaf RPC (no fan-out, no internal RPC calls).
Can be ported in isolation once `tenantroute` and the auth interceptor are
landed.

## Contract tests pinning behavior (file:line)

- `tests/python/integration/test_grpc_contract.py:281-287` — happy path:
  `ListMailboxUsersRequest(tenant_id="acme")` returns
  `ListMailboxUsersResponse` with `user_ids == []`. This test runs over a
  real gRPC channel without an auth interceptor wired (see harness setup
  at `test_grpc_contract.py:1-80`), so the Go server passes it once
  `_check_tenant` is satisfied. **The Go server must pass this test
  verbatim.**
- No dedicated unit tests for this RPC exist (deliberate — it's a stub).
  If the Go port adds metrics, add a unit test asserting the metric label
  but do not add behavioural tests beyond the contract test above.
- `sdk/go/entdb/cmd/entdb-console/frontend/src/gen/entdb_connect.ts:189-194`
  — generated client; non-behavioural, do not edit.
- `sdk/go/entdb/internal/pb/entdb_grpc.pb.go:70` — generated full-method
  constant; non-behavioural.

## Implementation outline

```go
// server/go/internal/api/list_mailbox_users.go
func (s *EntDBServer) ListMailboxUsers(
    ctx context.Context,
    req *pb.ListMailboxUsersRequest,
) (*pb.ListMailboxUsersResponse, error) {
    start := time.Now()
    status := "ok"
    defer func() {
        if r := recover(); r != nil {
            status = "error"
            metrics.RecordGRPCRequest("ListMailboxUsers", status, time.Since(start))
            panic(r)
        }
        metrics.RecordGRPCRequest("ListMailboxUsers", status, time.Since(start))
    }()

    if err := s.tenantRoute.CheckTenant(ctx, req.GetTenantId()); err != nil {
        status = "error"
        return nil, err // already a status.Error from tenantRoute
    }

    // Deprecated stub — legacy per-user mailbox SQLite store has been removed.
    // See docs/go-port/rpcs/ListMailboxUsers.md and
    // server/go/internal/api/list_mailbox_users.go.
    return &pb.ListMailboxUsersResponse{UserIds: []string{}}, nil
}
```

Wire `/entdb.v1.EntDBService/ListMailboxUsers` into the standard
authenticated + rate-limited interceptor chain (no bypass set). Do NOT
add to any unauth-bypass list.

## Open questions / risks

- **Should the Go port resurrect a real implementation?** The CLAUDE.md
  invariants would require: (a) a `mailbox_users` view materialized by the
  `Applier` from WAL events, NOT a side-table; (b) per-tenant SQLite
  isolation; (c) field-id storage. This is not in EPIC #407 scope — flag
  for a follow-up ticket if product still wants the listing.
- **Empty `tenant_id` handling.** The Python stub does not reject it. If
  EPIC #407 introduces a global "validate tenant_id" middleware, this RPC
  will start returning `INVALID_ARGUMENT` for empty input — confirm
  desired behavior before tightening.
- **Metrics divergence.** Adding `record_grpc_request` calls in the Go port
  is a net-positive but technically a behavioural divergence (the Python
  handler is missing them; see `server/go/internal/api/list_mailbox_users.go`). Recommend
  adding them in Go AND backfilling Python in a separate cleanup PR so the
  two implementations stay aligned.
- **Authentication strictness.** The contract test runs without an auth
  interceptor and expects `OK`. Production deployments DO have the auth
  interceptor — confirm with EPIC #407 owner whether the Go contract suite
  should additionally test the auth-required path.
- **Trailer behavior on redirect.** The Python `_check_tenant` sets
  `entdb-redirect-node` synchronously on `grpc.aio` contexts; the Go port
  must mirror this via `grpc.SetTrailer(ctx, metadata.Pairs(...))` BEFORE
  returning the `UNAVAILABLE` error — or SDK redirect caches will miss.
  Cross-link to `docs/go-port/shared/tenant-route.md`.
