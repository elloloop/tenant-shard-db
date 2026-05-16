# RPC Port Spec — `entdb.v1.EntDBService/GetMailbox`

> Implementation: `server/go/internal/api/get_mailbox.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407 — Python -> Go server port. Source of truth: Python handler at
`server/go/internal/api/get_mailbox.go`.

> Status: **deprecated stub.** The legacy per-user, cross-tenant mailbox
> SQLite store has been removed. Fanout now writes to the per-tenant
> `notifications` table in the canonical store
> (`apply/server/go/internal/store/`, `:4531`). `GetMailbox` is retained
> for proto compatibility and contract pinning, and MUST return an empty,
> well-formed response. Do NOT reintroduce a cross-tenant SQLite read here
> in the Go port — see "Side effects" and "Open questions" below.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:73` (rpc), `:539-557` (request/response),
`:559-578` (`MailboxItem`).

`GetMailboxRequest`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `context` | 1 | `RequestContext` | Standard envelope. `context.tenant_id` IS used (passed to `_check_tenant`). `context.actor` is wire-untrusted (see Auth). |
| `user_id` | 2 | `string` | Mailbox owner. Bare id (e.g. `"alice"`), NOT `"user:alice"` — translation lives at the gRPC boundary per CLAUDE.md. **Currently ignored** by the stub. |
| `source_type_id` | 3 | `int32` | Filter. Ignored by stub. |
| `thread_id` | 4 | `string` | Filter. Ignored by stub. |
| `unread_only` | 5 | `bool` | Filter. Ignored by stub. |
| `limit` | 6 | `int32` | Pagination. Ignored. |
| `offset` | 7 | `int32` | Pagination. Ignored. |

`GetMailboxResponse`:
| Field | Tag | Type | Stub value |
|------|-----|------|------|
| `items` | 1 | `repeated MailboxItem` | Always empty slice (NOT nil — pinned by `list(r.items) == []`). |
| `unread_count` | 2 | `int32` | Always `0`. |
| `has_more` | 3 | `bool` | Always `false`. Note: error path in Python omits `has_more` (default-zero), which is wire-equivalent to `false` — Go port should set it explicitly to `false` on both paths. |

`MailboxItem` shape (for type registration / forward-compat — never emitted by
the stub): `item_id`, `ref_id`, `source_type_id`, `source_node_id`,
`thread_id`, `ts (int64)`, `snippet (8)`, `state (Struct, 10)`,
`metadata (Struct, 11)`. Tags 7 and 9 are **reserved** (old JSON string
fields) — Go generated code MUST keep the reservation.

## Auth (mailbox owner verification via trusted-actor)

- **Authenticated method.** `/entdb.EntDBService/GetMailbox` is NOT in
  `AuthInterceptor.UNAUTHENTICATED_METHODS`
  (`auth/auth_interceptor.py:157-162`). Without an interceptor populating
  the `ContextVar`, `_trusted_actor` falls back to the wire actor — but
  the stub does not call `_trusted_actor` at all.
- **Owner verification: NOT performed by the current stub.** A live
  implementation would resolve `_trusted_actor(ctx.actor)`
  (`server/go/internal/api/get_mailbox.go`) and reject when it does not match
  `_actor_user_id(...) == request.user_id` (`:412-416`). The Go port MUST
  keep parity with the stub: do NOT add owner checks that diverge from
  Python observable behavior.
- **Tenant guard runs.** `_check_tenant(request.context.tenant_id, context)`
  (`server/go/internal/api/get_mailbox.go`) is the only auth-shaped check. It enforces:
  - sharding ownership (UNAVAILABLE + `entdb-redirect-node` trailer when
    another node owns the tenant — `:387-394`),
  - region pinning (FAILED_PRECONDITION when `tenant.region != served_region`
    — `:400-410`).
- **No `Permission`/ACL check.** The stub bypasses
  `_check_tenant_access` and the capability registry entirely.
- **Trusted-actor invariant (CLAUDE.md §"actor is wire-untrusted"):** the
  stub never trusts `request.context.actor` because it never reads it.
  The Go port MUST preserve this — do NOT log, branch on, or echo back
  the wire actor without first running it through the trusted-actor
  resolver.

## Side effects (cross-tenant: global_store reads)

**None today.** Strictly:

1. `start := time.Now()` for the `record_grpc_request` Prometheus
   observation.
2. `await self._check_tenant(...)` — may read `global_store.get_tenant(...)`
   for the region check (`:401`). This is a cross-tenant *metadata* read,
   not a mailbox read. It is the only DB touch on the success path.
3. Return the canned empty response.

**Crucially: NO SQLite reads, NO WAL append.** This RPC is a pure read,
so the WAL invariant ("all writes go through the WAL", CLAUDE.md §1)
does not constrain it. The CLAUDE.md §4 invariant ("per-tenant SQLite
isolation") forbids ad-hoc cross-tenant SQLite scans — the historical
mailbox store violated this and was removed; the Go port MUST NOT
restore it. If a real cross-tenant mailbox is reintroduced, it goes
through `global_store` with its own SQLite, never by joining across
per-tenant DBs.

## Error contract

| Condition | Status | Where pinned |
|----------|--------|--------------|
| Tenant not owned by this node | `UNAVAILABLE` + `entdb-redirect-node` trailer | `server/go/internal/api/get_mailbox.go`; mirrored by `_check_tenant` for every tenant-scoped RPC. |
| Tenant region mismatch | `FAILED_PRECONDITION` | `server/go/internal/api/get_mailbox.go`. |
| Any other exception inside the `try` | **Swallowed.** Returns `GetMailboxResponse(items=[], unread_count=0)` with `has_more` defaulting to `false`. Error is logged with `exc_info=True` and `record_grpc_request("GetMailbox", "error", ...)` is incremented. | `server/go/internal/api/get_mailbox.go`. |

The swallow-and-return-empty pattern is intentional and pinned by the
contract test (which only asserts `items == [] and unread_count == 0`).
The Go port MUST mirror it: a panic/error inside the handler body must
NOT propagate as a gRPC error to the client. Use `defer recover()` +
metric increment.

## Shared Go package deps

- `internal/shard` — `Sharding.IsMine`, `Sharding.GetOwner` for
  `_check_tenant` parity (Python: `dbaas/sharding/...`,
  `server/go/internal/api/get_mailbox.go`).
- `internal/globalstore` — `GetTenant(ctx, tenantID)` for the region
  guard (`server/go/internal/api/get_mailbox.go`).
- `internal/metrics` — `RecordGRPCRequest(method, status, latency)`
  matching Python `record_grpc_request` cardinality
  (`server/go/internal/api/get_mailbox.go,1496`). Status labels: `"ok"`, `"error"`.
- `internal/auth/trusted` — `GetAuthoritativeActor(wireActor)` ContextVar
  equivalent (Python: `auth/auth_interceptor.py`, used via
  `_trusted_actor` `:418-437`). Imported but NOT called by the stub;
  required for any future un-stubbing.
- `gen/entdb/v1` — generated `GetMailboxRequest`, `GetMailboxResponse`,
  `MailboxItem`. Reserved tags 7,9 must survive codegen.

## Other-RPC deps (SearchMailbox, ShareNode, ListMailboxUsers)

- **SearchMailbox** (`server/go/internal/api/get_mailbox.go`, proto `:70`) — same
  deprecated stub shape: `_check_tenant` + empty `SearchMailboxResponse`.
  Port together; share the empty-response helper.
- **ListMailboxUsers** (`server/go/internal/api/get_mailbox.go`, proto `:85`) — also
  a stub. Note: takes `tenant_id` as a top-level field, not
  `RequestContext`. Returns `ListMailboxUsersResponse(user_ids=[])`.
- **ShareNode** (`server/go/internal/api/get_mailbox.go`, proto `:94`) — the *live*
  cross-tenant operation. Goes through `_trusted_actor`,
  `_check_tenant_access`, and writes to `global_store.shared_index`
  (`:1815-1820`). When/if a real mailbox returns, `ShareNode`'s side
  effects are the upstream producer — but that read path will live in
  `ListSharedWithMe` (already wired in the TS client gen), NOT here.
  Treat `ShareNode` as orthogonal for the `GetMailbox` port: same epic,
  unrelated handler.

## Contract tests pinning behavior (file:line)

- `tests/python/integration/test_grpc_contract.py:274-280` — the
  authoritative pin. Builds `GetMailboxRequest(context=_ctx(),
  user_id="alice")`, expects `list(r.items) == [] and r.unread_count == 0`,
  mode `"happy"` (no auth interceptor; goes through anyway because the
  handler has no actor check). Go port MUST pass byte-identical wire
  bytes against this test fixture.
- `tests/python/integration/test_grpc_contract.py:267-272` — sibling
  pin for `SearchMailbox`; same shape.
- `tests/python/integration/test_grpc_contract.py:281-287` — sibling
  pin for `ListMailboxUsers`.

(`grep -rn "GetMailbox" tests/` returns only the file above — no
unit-level coverage. The Go port should add a Go-side unit test that
exercises the `_check_tenant` UNAVAILABLE branch, which the existing
contract test does not cover for this RPC.)

## Implementation outline

```go
func (s *Server) GetMailbox(ctx context.Context, req *pb.GetMailboxRequest) (*pb.GetMailboxResponse, error) {
    start := time.Now()
    defer func() {
        if r := recover(); r != nil {
            s.metrics.RecordGRPCRequest("GetMailbox", "error", time.Since(start))
            log.Error().Interface("panic", r).Msg("GetMailbox failed")
            // swallow — see "Error contract"
        }
    }()
    if err := s.checkTenant(ctx, req.GetContext().GetTenantId()); err != nil {
        // _check_tenant aborts with its own status; propagate.
        s.metrics.RecordGRPCRequest("GetMailbox", "error", time.Since(start))
        return nil, err
    }
    s.metrics.RecordGRPCRequest("GetMailbox", "ok", time.Since(start))
    return &pb.GetMailboxResponse{Items: []*pb.MailboxItem{}, UnreadCount: 0, HasMore: false}, nil
}
```

Notes:
- `Items` is a non-nil empty slice — protobuf-go marshals nil and `[]`
  identically on the wire, but the Python contract test calls
  `list(r.items)` which works for both; still, prefer explicit `[]` for
  symmetry with `server/go/internal/api/get_mailbox.go`.
- `UNAVAILABLE` from `checkTenant` MUST attach the `entdb-redirect-node`
  trailer via `grpc.SetTrailer` BEFORE returning the status — the SDK
  redirect cache (`sdk/go/entdb/redirect_cache.go`) depends on it.
- Do NOT read `req.UserId`, `req.SourceTypeId`, `req.ThreadId`,
  `req.UnreadOnly`, `req.Limit`, `req.Offset`. Pinned by the Python
  stub ignoring them.

## Open questions / risks

1. **Resurrection risk.** The TS client gen
   (`sdk/go/entdb/cmd/entdb-console/frontend/src/gen/entdb_connect.ts:145`)
   still exposes `GetMailbox`. If a console feature starts calling it,
   we will silently see empty results forever. Action: add a
   server-side `log.Warn` (rate-limited) on first call per process, OR
   surface a deprecation header (`grpc-status-details-bin` /
   custom trailer). Decide before the Go port lands.
2. **Should the Go port keep the stub or delete the RPC?** Proto
   removal is a breaking wire change and is out-of-scope for #407.
   Recommendation: keep the stub, mark the proto comment as
   `// DEPRECATED: returns empty; see notifications table.`
3. **Owner-mismatch behavior is undefined.** A future un-stubbing will
   need to decide: `PERMISSION_DENIED` when caller != `user_id`, or
   silent empty? CLAUDE.md leans `PERMISSION_DENIED` (fail closed); the
   current stub leans silent empty. Document the choice when the
   stub is removed.
4. **Cross-tenant storage location.** If a real mailbox returns, it
   MUST live in `global_store` (CLAUDE.md §4) and be fed by a
   `WAL`-driven applier (CLAUDE.md §1). The `notifications` per-tenant
   table is the right home for *intra-tenant* fanout; a
   cross-tenant inbox is a separate problem.
5. **Metrics cardinality.** Python emits `record_grpc_request("GetMailbox",
   "ok"|"error", ...)` — only two status labels. Do not introduce a
   third label (`"deprecated"`, `"empty"`, etc.) in the Go port; it
   breaks dashboards.
