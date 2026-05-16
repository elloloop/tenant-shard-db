# SearchMailbox — Go port spec

> Implementation: `server/go/internal/api/search_mailbox.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC: #407 (Python -> Go server port)
RPC: `entdb.v1.EntDBService/SearchMailbox`
Python handler: `server/go/internal/api/search_mailbox.go`
Proto: `proto/entdb/v1/entdb.proto:70` (service entry), `:511-537` (messages)

## Wire contract

Unary RPC. Request `SearchMailboxRequest` (proto field IDs preserved):

| field | id | type | meaning |
|------|----|------|---------|
| context | 1 | RequestContext | tenant_id, actor, trace_id |
| user_id | 2 | string | mailbox owner (cross-tenant scope) |
| query | 3 | string | FTS5 match expression |
| source_type_ids | 4 | repeated int32 | optional type filter |
| limit | 5 | int32 | page size |
| offset | 6 | int32 | pagination offset |

Response `SearchMailboxResponse`:

| field | id | type |
|------|----|------|
| results | 1 | repeated MailboxSearchResult |
| has_more | 2 | bool |

`MailboxSearchResult`: `item` (MailboxItem), `rank` (float32, FTS5 bm25), `highlights` (string).

Status today: handler is a deprecated stub. It validates tenant ownership and returns
`SearchMailboxResponse{results=[], has_more=false}` regardless of input
(`server/go/internal/api/search_mailbox.go`). The legacy per-user mailbox SQLite store was removed; fanout now
writes to a per-tenant `notifications` table (`server/go/internal/store/`,
`server/go/internal/store/`). The Go port MUST preserve this stub semantics for v1 parity, then
the FTS5-backed implementation lands behind the same RPC in a follow-up.

## Auth (whose mailbox? trusted-actor)

- Wire `request.context.actor` is UNTRUSTED. Always rebind via `_trusted_actor` equivalent,
  which prefers the `AuthInterceptor` `ContextVar` and only falls back to the wire field for
  no-auth deployments and unit tests (`server/go/internal/api/search_mailbox.go`).
- `request.user_id` selects WHICH mailbox to read; it is NOT identity. Authorization rule
  (when implementation lands): `trusted_actor` MUST be either (a) the same user as
  `user_id`, or (b) hold tenant-admin / system role on `context.tenant_id`. Anything else =>
  `PERMISSION_DENIED`.
- Tenant gate runs first via `_check_tenant` (`server/go/internal/api/search_mailbox.go`): rejects with
  `UNAVAILABLE` + `entdb-redirect-node` trailer if this node is not the shard owner;
  rejects with `FAILED_PRECONDITION` if the tenant is region-pinned elsewhere.
- "Mailbox is cross-tenant for a user" applies at the data layer (notifications can come
  from any tenant a user belongs to), but the RPC itself is scoped to a single
  `context.tenant_id` — the caller must drive the cross-tenant fanout client-side. The Go
  port keeps that contract: one tenant per call.

## Side effects

None today (stub returns empty). Future FTS5-backed impl is read-only:

- Reads from per-tenant SQLite only — no cross-tenant SQLite transaction
  (per CLAUDE.md invariant 4). Cross-user-tenant aggregation is the caller's job.
- Uses FTS5 virtual tables `fts_t{type_id}`, lazily created on first read/write
  (`server/go/internal/store/`, `docs/decisions/fts.md:37-70`).
- Joins FTS rows against `notifications` (per-tenant) keyed by `node_id`.
- NOT a write path — never appends to the WAL. (Mailbox writes happen via the fanout
  applier, which IS WAL-driven; see invariant 1.)

## Error contract (exhaustive)

Today (stub):

- Tenant not owned by this node => `UNAVAILABLE` + trailer `entdb-redirect-node=<owner>`
  (`server/go/internal/api/search_mailbox.go`).
- Tenant region-pinned elsewhere => `FAILED_PRECONDITION` (`server/go/internal/api/search_mailbox.go`).
- Any other exception inside the handler is swallowed: returns `SearchMailboxResponse{results=[]}`
  with `has_more` defaulted to false, logged at ERROR (`server/go/internal/api/search_mailbox.go`).
  The Go port MUST replicate the swallow-and-return-empty behavior — a contract test pins it.

For the future FTS5-backed impl:

- Authorization fail => `PERMISSION_DENIED`.
- Malformed FTS5 query => `INVALID_ARGUMENT` with "fts5: syntax error" detail.
- `limit < 0` or `offset < 0` => `INVALID_ARGUMENT`.
- Unknown `source_type_ids` => silently filtered out (no error), matches `SearchNodes`.
- SQLite I/O error => `INTERNAL`.

## Shared Go package deps

- `internal/auth` — `TrustedActor(ctx, wireActor) string`, mirrors `_trusted_actor`.
- `internal/sharding` — `IsMine(tenantID)`, `Owner(tenantID)`, redirect-trailer helper.
- `internal/region` — `ServedRegion()`, tenant-region pin check.
- `internal/store/canonical` — per-tenant SQLite handle pool.
- `internal/fts` (NEW subsystem) — wraps SQLite FTS5 access:
  - `EnsureTable(conn, tenantID, typeID int32, fieldIDs []int32) error`
  - `Search(conn, tenantID, typeID int32, query string, limit, offset int) ([]Hit, error)`
  - Hit: `{NodeID string, Rank float32, Highlights string}`
  - Lazy DDL with process-local cache (mirrors `_fts_table_cache`,
    `server/go/internal/store/`, `:2025-2037`).
- `internal/proto/structpb` — for response payload Struct conversion (when impl lands).
- `internal/metrics` — `RecordGRPCRequest("SearchMailbox", outcome, latency)`.

## Other-RPC deps (overlap)

- **GetMailbox** (`server/go/internal/api/search_mailbox.go`) — same stub pattern, same auth gate, same
  swallow-and-return-empty error policy. Implement together; share the auth helper.
- **SearchNodes** (`server/go/internal/store/`, proto `:SearchNodesRequest`) — the
  "real" FTS5 RPC. Future SearchMailbox is structurally `SearchNodes` with a fixed
  type-set (notification source types) and an extra `user_id` predicate. Reuse the
  `internal/fts` package; do NOT fork.
- **ListMailboxUsers** — also a deprecated stub returning empty; same family.
- **AuthInterceptor** — populates the trusted-actor `ContextVar`; required for the
  authz check.

## Contract tests pinning behavior

- `tests/python/integration/test_grpc_contract.py:267-273` — happy path, asserts
  `results == [] and has_more == False`. Go port MUST satisfy this byte-for-byte.
- `tests/python/integration/test_grpc_contract.py:282-287` — `ListMailboxUsers` empty
  stub (sister assertion).
- `tests/python/integration/test_grpc_contract.py:288-295` — `ListTenants` returns
  `PERMISSION_DENIED` when no auth interceptor; documents the trusted-actor contract
  the Go port must reproduce.
- No test currently exercises tenant-redirect / region-pin paths for SearchMailbox
  specifically — copy the patterns from `_check_tenant` tests in
  `tests/python/integration/test_sharding_*` and add equivalents.

## Implementation outline

```go
func (s *EntDBServer) SearchMailbox(
    ctx context.Context, req *entdbv1.SearchMailboxRequest,
) (*entdbv1.SearchMailboxResponse, error) {
    start := time.Now()
    defer func() { metrics.RecordGRPC("SearchMailbox", outcome, time.Since(start)) }()

    if err := s.checkTenant(ctx, req.GetContext().GetTenantId()); err != nil {
        return nil, err  // UNAVAILABLE / FAILED_PRECONDITION already typed
    }

    // v1 stub parity with Python handler.
    return &entdbv1.SearchMailboxResponse{Results: nil, HasMore: false}, nil
}
```

When FTS5 impl lands (follow-up issue):

1. `actor := auth.TrustedActor(ctx, req.Context.Actor)`.
2. Authz: `actor == "user:"+req.UserId` OR `s.isTenantAdmin(actor, tenantID)` else
   `PERMISSION_DENIED`.
3. Validate `limit >= 0`, `offset >= 0` else `INVALID_ARGUMENT`.
4. Acquire per-tenant SQLite handle.
5. For each source type id (or default notification type set), call
   `fts.Search(conn, tenantID, typeID, req.Query, limit+1, offset)`. If
   `source_type_ids` empty, query the union of registered notification types.
6. Join hit `node_id`s against `notifications` to build `MailboxItem`s; keep
   FTS `rank` and `highlights` (use `snippet()`/`highlight()` FTS5 aux funcs).
7. ACL filter server-side (mirrors `SearchNodes` ACL trim).
8. Trim to `limit`; set `has_more = len(hits) > limit`.
9. Wrap unexpected errors: log at ERROR, return empty response (Python swallow
   parity is REQUIRED until contract tests are updated to expect typed errors).

## Open questions / risks

- **Stub vs real**: Issue #407 should clarify whether the Go port lands the
  v1 stub only, or also implements the FTS5-backed version. Default: stub now,
  FTS5 in a follow-up — matches "behavioral parity first" rule.
- **Cross-tenant fanout**: a user belongs to many tenants; today the RPC is
  single-tenant. Confirm there is no hidden caller relying on cross-tenant
  results (grep for SDK callers in `sdk/python/entdb_sdk` and
  `sdk/go/entdb`).
- **Highlights/rank shape**: `MailboxSearchResult.highlights` is a single
  string — Python never populated it. Define the format (FTS5 `snippet(...)`
  output, `<mark>...</mark>` tags?) before any client depends on it.
- **Rank type**: proto says `float`; FTS5 bm25 in SQLite returns float64.
  Cast at the boundary; document loss of precision is acceptable.
- **Error swallow**: returning `OK` with empty results for an internal failure
  is a misleading metric. Once the FTS5 impl ships, flip to `INTERNAL` and
  update the contract test in the same PR.
- **Empty `query`**: Python stub accepts it; future impl should treat empty
  query as `INVALID_ARGUMENT` (FTS5 MATCH on empty string is an error anyway).
