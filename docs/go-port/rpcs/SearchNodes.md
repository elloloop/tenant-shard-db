# SearchNodes — Go port spec

> Implementation: `server/go/internal/api/search_nodes.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC: #407 (Python -> Go server port)
RPC: `entdb.v1.EntDBService/SearchNodes`
Python handler: `/Users/arun/projects/opensource/tenant-shard-db/server/go/internal/api/search_nodes.go`
Proto: `/Users/arun/projects/opensource/tenant-shard-db/proto/entdb/v1/entdb.proto:148` (service entry), `:1124-1136` (messages)
Decision: `/Users/arun/projects/opensource/tenant-shard-db/docs/decisions/fts.md`

## Wire contract (query string, filter, ranking, pagination)

Unary RPC. `SearchNodesRequest` is a flat message (NOT wrapped in `RequestContext` — `tenant_id` and `actor` live at the top level, unlike most RPCs):

| field   | id | type   | meaning                                                                        |
|---------|----|--------|--------------------------------------------------------------------------------|
| tenant_id | 1 | string | tenant scope; mandatory                                                       |
| actor    | 2 | string | wire-claimed actor — UNTRUSTED, rebound via `_trusted_actor`                   |
| type_id  | 3 | int32  | node type whose FTS5 virtual table to query                                    |
| query    | 4 | string | FTS5 MATCH expression (AND/OR/NOT, phrase `"..."`, prefix `wid*`, `f3:term`)   |
| limit    | 5 | int32  | page size, defaults to 50 when zero (`server/go/internal/api/search_nodes.go`)                    |
| offset   | 6 | int32  | pagination offset, defaults to 0 (`server/go/internal/api/search_nodes.go`)                       |

`SearchNodesResponse`:

| field | id | type            |
|-------|----|-----------------|
| nodes | 1  | repeated Node   |

No `has_more`, no `total_count`, no per-row `rank`/`highlights` in this RPC — those are explicitly out of scope per `docs/decisions/fts.md:117-122`. Ordering is FTS5 `bm25` ascending (best match first) via `ORDER BY fts.rank` (`server/go/internal/store/`). The Go port MUST preserve the response shape exactly: a single repeated `Node` field, same proto field numbers.

Pagination: stateless `LIMIT/OFFSET` against the joined query — no cursor token. Callers wanting "next page" must re-issue with `offset += len(nodes)`. `Node.payload` is emitted as `google.protobuf.Struct` keyed by **field-id strings** (not field names) per CLAUDE.md invariant 6 — translation to names is the SDK's job.

## Auth (per-tenant; ACL post-filter; trusted-actor)

1. `_check_tenant(tenant_id, ctx)` (`server/go/internal/api/search_nodes.go`, called at `:1146`) is the first gate:
   - Returns `UNAVAILABLE` + trailer `entdb-redirect-node=<owner>` if shard not owned by this node.
   - Returns `FAILED_PRECONDITION` if the tenant is region-pinned elsewhere.
2. `actor = self._trusted_actor(request.actor)` (`:1173`, helper at `:418-437`) prefers the AuthInterceptor `ContextVar` over the wire-claimed string. The Go port MUST NOT trust `request.actor` directly — it is purely a fallback for no-auth dev/test. Per commit fece3fb the wire actor is treated as untrusted for all gRPC handlers.
3. **No pre-FTS authz**: `SearchNodes` does NOT require admin or membership before running the FTS5 query. Anyone whose tenant gate passes can MATCH against the FTS index. The privacy boundary is the per-row ACL post-filter, not a coarse RPC-level check.
4. `_check_cross_tenant_read(tenant_id, trusted_actor, ctx)` (`:561` / called at `:1174`) classifies the actor as `"member"` or `"cross_tenant"`. **Only when role is `cross_tenant`** does the handler ACL-trim the result set (`:1179-1192`):
   - `actor_ids = canonical_store.resolve_actor_groups(tenant_id, trusted_actor)` expands the trusted actor into its group memberships.
   - For each candidate node, `canonical_store.can_access(tenant_id, node_id, actor_ids)` is checked; non-accessible rows are dropped silently.
5. Implication for parity: an in-tenant member sees the unfiltered FTS result set (including nodes they have no ACL grant on). This matches `QueryNodes` semantics — both treat ACL as a cross-tenant-only filter today. The Go port MUST replicate this exactly; do NOT tighten it without a separate decision doc.
6. Per-tenant SQLite isolation (CLAUDE.md invariant 4): the FTS5 virtual table `fts_t{type_id}` lives in the SAME per-tenant SQLite file as the `nodes` table. Cross-tenant search is impossible by construction.

## Side effects (read on FTS5 + canonical_store)

- Read-only RPC — does NOT append to the WAL. (FTS5 mutations are applied inside `Applier._apply_event_with_conn`, see `server/go/internal/apply/applier.go,1108,1130` and `docs/decisions/fts.md:73-83`.)
- **Lazy DDL**: `_ensure_fts_table` (`server/go/internal/store/`) issues `CREATE VIRTUAL TABLE IF NOT EXISTS fts_t{type_id} USING fts5(node_id UNINDEXED, f{fid1}, f{fid2}, ..., tokenize='porter unicode61')` on first read or write. Cached in `self._fts_table_cache: set[(tenant_id, type_id)]` (`server/go/internal/store/`). The Go port MUST replicate this lazy + cached DDL behavior — first-call latency includes the CREATE.
- **Single SQL JOIN**: `SELECT n.* FROM fts_t{type_id} AS fts JOIN nodes AS n ON n.node_id = fts.node_id AND n.tenant_id = ? WHERE fts_t{type_id} MATCH ? ORDER BY fts.rank LIMIT ? OFFSET ?` (`server/go/internal/store/`).
- Empty `searchable_field_ids` => returns `[]` immediately, no SQL issued (`server/go/internal/store/`). This is the path covered by the contract test "type 1 has no searchable fields".
- Metrics: `record_grpc_request("SearchNodes", "ok"|"error", elapsed)` at `:1208,1211`. Go port wires same labels.
- ACL post-filter (cross-tenant only) issues additional reads against `acl` / group-membership tables via `resolve_actor_groups` + `can_access` — N+1 in row count. Acceptable per Python parity; optimize later if needed.

## Error contract (INVALID_ARGUMENT for bad query)

| condition                                              | gRPC status              | source                          |
|--------------------------------------------------------|--------------------------|---------------------------------|
| `query.strip() == ""`                                  | `INVALID_ARGUMENT` "query must not be empty" | `server/go/internal/api/search_nodes.go` |
| `len(query) > 1000`                                    | `INVALID_ARGUMENT` "query must be under 1000 characters" | `:1153-1158` |
| Tenant not owned by this node                          | `UNAVAILABLE` + redirect trailer | `:362-395`              |
| Tenant region-pinned elsewhere                         | `FAILED_PRECONDITION`    | `:405-410`                      |
| Type with no searchable fields                         | `OK` with `nodes: []`     | `:1161` + `server/go/internal/store/` |
| Malformed FTS5 syntax (e.g. unbalanced quotes)         | swallowed -> `OK` with `nodes: []` (logged ERROR) | `:1210-1213` |
| SQLite I/O error                                       | swallowed -> `OK` with `nodes: []` (logged ERROR) | `:1210-1213` |

The blanket `except Exception` at `:1210` is a known Python wart: ANY error after the validation gates becomes an empty-result `OK`. Contract tests (below) pin this behavior. The Go port MUST preserve swallow-to-empty for parity in v1; `INTERNAL`-status errors can be introduced only with a contract-test update in the same PR.

`limit < 0` / `offset < 0`: Python does NOT validate these; SQLite errors and the swallow path returns empty. Go port should match — do NOT add a stricter check.

## Shared Go package deps (fts subsystem)

- `internal/auth` — `TrustedActor(ctx, wireActor) string` (mirrors `_trusted_actor`, `server/go/internal/api/search_nodes.go`).
- `internal/sharding` — `IsMine(tenantID)`, `Owner(tenantID)`, redirect-trailer helper for `_check_tenant`.
- `internal/region` — `ServedRegion()` + tenant-region pin lookup.
- `internal/store/canonical` — per-tenant SQLite handle pool; surfaces `Conn(tenantID)`.
- `internal/store/canonical/acl` — `ResolveActorGroups(conn, tenantID, actor) []string`, `CanAccess(conn, tenantID, nodeID, actorIDs) bool` mirroring `server/go/internal/store/`.
- `internal/schema` — `Registry.SearchableFieldIDs(typeID int32) []int32` mirroring `get_searchable_field_ids` (`schema/registry.py:311`).
- `internal/fts` (NEW subsystem, shared with future `SearchMailbox`):
  - `EnsureTable(conn *sql.Conn, tenantID string, typeID int32, fieldIDs []int32) error` — idempotent `CREATE VIRTUAL TABLE IF NOT EXISTS`, process-local cache keyed by `(tenantID, typeID)`.
  - `Search(conn, tenantID, typeID, query string, limit, offset int) ([]NodeRow, error)` — runs the JOIN, returns `nodes`-table rows in rank order.
  - Cache type: `sync.Map[ftsKey]struct{}` to mirror Python `_fts_table_cache` (`server/go/internal/store/`, `:2025-2037`).
- `internal/proto/structpb` — `MapToStruct(map[string]any) *structpb.Struct` for `Node.payload`.
- `internal/metrics` — `RecordGRPC("SearchNodes", outcome, latency)`.

**SQLite driver**: must be the same one used by `GetNode`/`QueryNodes` (one driver across the canonical store). FTS5 is a compile-time SQLite feature — the chosen driver build MUST include it (see Open questions).

## Other-RPC deps (SearchMailbox sibling)

- `SearchMailbox` (`docs/go-port/rpcs/SearchMailbox.md`) — same FTS5 family. Its future real impl is structurally `SearchNodes` with a fixed type-set (notification source types) plus a `user_id` predicate. Both MUST share `internal/fts`; do not fork.
- `QueryNodes` (`docs/go-port/rpcs/QueryNodes.md`) — sibling read RPC. Identical ACL-post-filter pattern; reuse the same `acl.CanAccess` helper. Verify both go-port specs land the same shared helpers.
- `GetNode`/`GetNodes` — same per-tenant SQLite handle pool; same `Node` row shape.
- `ExecuteAtomic` — produces the FTS index rows (writer side via `Applier`); behavioral parity test for `SearchNodes` requires `ExecuteAtomic` writes to be visible ((legacy Python unit test, removed in Phase 4D) writes via the applier, then reads via `SearchNodes`).
- `AuthInterceptor` — populates the trusted-actor `ContextVar`; mandatory dependency for the trusted-actor rebind (commit fece3fb).

## Contract tests pinning behavior (file:line)

- `/Users/arun/projects/opensource/tenant-shard-db/tests/python/integration/test_grpc_contract.py:627-632` — empty query => `INVALID_ARGUMENT`. Go port MUST satisfy byte-for-byte.
- `/Users/arun/projects/opensource/tenant-shard-db/tests/python/integration/test_grpc_contract.py:633-641` — `type_id=1` has no searchable fields => `OK` with `nodes == []`. The "no searchable fields short-circuit" is a load-bearing contract.
- `/Users/arun/projects/opensource/tenant-shard-db/(legacy Python unit test, removed)` — empty-query `INVALID_ARGUMENT` (unit-level coverage).
- `/Users/arun/projects/opensource/tenant-shard-db/(legacy Python unit test, removed)` — happy path: write `Product` via the applier with `desc="findme"`, search `query="widget"`, expect exactly one node with `node_id == "p1"` (FTS Porter stemming + multi-field index).
- `/Users/arun/projects/opensource/tenant-shard-db/(legacy Python unit test, removed)` (full file) — additional assertions on phrase / prefix / column-filter syntax and update/delete propagation through the applier.
- `/Users/arun/projects/opensource/tenant-shard-db/(legacy Python unit test, removed)` — `SearchNodes` payload Struct format is field-id-keyed (CLAUDE.md invariant 6); regression target.

Cross-implementation contract tests under `tests/contract/` are a placeholder per CLAUDE.md ("Project Structure"); when the Go server lands, port the four cases above into the polyglot suite first.

## Implementation outline

```go
func (s *EntDBServer) SearchNodes(
    ctx context.Context, req *entdbv1.SearchNodesRequest,
) (resp *entdbv1.SearchNodesResponse, err error) {
    start := time.Now()
    outcome := "ok"
    defer func() { metrics.RecordGRPC("SearchNodes", outcome, time.Since(start)) }()

    // 1. Tenant gate (UNAVAILABLE / FAILED_PRECONDITION).
    if err := s.checkTenant(ctx, req.GetTenantId()); err != nil {
        outcome = "error"; return nil, err
    }

    // 2. Validate query (INVALID_ARGUMENT before anything else touches storage).
    q := strings.TrimSpace(req.GetQuery())
    if q == "" {
        outcome = "error"
        return nil, status.Error(codes.InvalidArgument, "query must not be empty")
    }
    if len(q) > 1000 {
        outcome = "error"
        return nil, status.Error(codes.InvalidArgument, "query must be under 1000 characters")
    }

    // 3. Schema lookup: empty searchable set => empty response, no SQL.
    typeID := req.GetTypeId()
    fids := s.schema.SearchableFieldIDs(typeID)
    if len(fids) == 0 {
        return &entdbv1.SearchNodesResponse{}, nil
    }

    // 4. Open per-tenant conn and run the FTS5 JOIN. Lazy DDL inside.
    limit := int(req.GetLimit()); if limit == 0 { limit = 50 }
    offset := int(req.GetOffset())
    rows, ferr := s.fts.Search(ctx, req.GetTenantId(), typeID, q, limit, offset)
    if ferr != nil {
        // PARITY: swallow + return empty + log ERROR (do NOT surface INTERNAL yet).
        s.log.Error("SearchNodes failed", "err", ferr)
        return &entdbv1.SearchNodesResponse{}, nil
    }

    // 5. ACL post-filter only when the actor is cross-tenant.
    actor := auth.TrustedActor(ctx, req.GetActor())
    role, err := s.checkCrossTenantRead(ctx, req.GetTenantId(), actor)
    if err != nil { outcome = "error"; return nil, err }
    if role == auth.RoleCrossTenant {
        actorIDs, _ := s.acl.ResolveActorGroups(ctx, req.GetTenantId(), actor)
        kept := rows[:0]
        for _, n := range rows {
            ok, _ := s.acl.CanAccess(ctx, req.GetTenantId(), n.NodeID, actorIDs)
            if ok { kept = append(kept, n) }
        }
        rows = kept
    }

    // 6. Convert to proto.
    out := make([]*entdbv1.Node, 0, len(rows))
    for _, n := range rows { out = append(out, n.ToProto()) }
    return &entdbv1.SearchNodesResponse{Nodes: out}, nil
}
```

Notes on parity:
- Order of operations matches Python exactly: tenant -> validate query -> searchable lookup -> SQL -> ACL trim -> proto convert. Swapping any two breaks at least one contract test.
- Convert payload via field-id-keyed Struct; do NOT translate to field names server-side.
- Default `limit = 50` only when caller passes 0; negative limits flow into SQLite (which errors), then the swallow path returns empty.

## Open questions / risks (Go FTS5 driver choice, parity with Python)

- **SQLite driver / FTS5 build**: `mattn/go-sqlite3` requires the `sqlite_fts5` build tag; pure-Go `modernc.org/sqlite` ships FTS5 in default builds and avoids cgo (preferred for cross-compile + supply-chain). Decision needed before `internal/fts` lands; document in a follow-up `docs/decisions/go-sqlite-driver.md`. Whichever driver is picked MUST also be used by `GetNode`/`QueryNodes`/`Applier` — one driver across the canonical store.
- **Tokenizer parity**: Python uses `tokenize='porter unicode61'`. Both candidate Go drivers expose the same FTS5 tokenizers, but verify behavior on accented input + Porter stemming with a tokenizer-level test (compare Go output against `pytest (legacy Python unit test, removed)` corpus).
- **Lazy DDL race**: two concurrent first-time searches on the same `(tenant, type)` could both run `CREATE VIRTUAL TABLE IF NOT EXISTS` — idempotent in SQLite but the cache update needs `sync.Map.LoadOrStore`. Python is single-process per tenant SQLite + GIL-serialized; Go can have true concurrency.
- **Error swallow semantics**: returning `OK` with `nodes: []` for malformed FTS syntax is a misleading metric — operators can't distinguish "no matches" from "user typo". Log volume is the only signal today. Flip to `INVALID_ARGUMENT` (with detail "fts5: syntax error") only in a follow-up that updates the contract test in the same PR.
- **ACL trim cost**: cross-tenant searches do an N-row N+1 query loop. Acceptable for parity; revisit with a JOIN-based ACL filter once `QueryNodes` does the same (keep both RPCs in lockstep).
- **`limit`/`offset` validation**: Python lets negative values reach SQLite. Go drivers may differ in error shape; `internal/fts` should normalize to "swallow-and-empty" so the contract is preserved.
- **bm25 stability**: `ORDER BY fts.rank` is `bm25(fts_t{type_id})` ascending. SQLite's bm25 weights default to 1.0 per column; if a follow-up tunes per-column weights, both Python and Go must change together (contract: "best match first" only — no exact-rank assertions exist today).
- **Concurrency with writers**: FTS index is updated inside the applier's per-event SQLite transaction. Search reads see consistent snapshots only if WAL-mode SQLite is used (it is, in Python). Confirm Go canonical_store opens with `_journal_mode=WAL`.
- **Empty-but-whitespace query**: Python `query.strip()` rejects `"   "` as empty. Go must call `strings.TrimSpace` before the empty-check, not just compare to `""`.
- **Wire-actor exposure**: `SearchNodesRequest` carries `actor` at the top level (not inside `RequestContext`). The trusted-actor rebind is identical, but logging must avoid leaking `request.actor` as identity (use the rebound `actor` only).
