# ListSharedWithMe — Go Port Spec

> Implementation: `server/go/internal/api/list_shared_with_me.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407. Reference Python handler:
`server/go/internal/api/list_shared_with_me.go`. Proto:
`proto/entdb/v1/entdb.proto:100, 757-766`.

The RPC answers "what has been shared TO me?" — the inverse of `ShareNode`'s
direction. It aggregates two indexes:
1. **per-tenant `node_access`** — same-tenant grants where caller is the grantee.
2. **global `shared_index`** — cross-tenant grants written by `ShareNode`
   into `global_store` (one row per recipient × source-tenant × node).

## Wire contract

Request `entdb.v1.ListSharedWithMeRequest` (proto:757):
- `RequestContext context` (tenant_id, actor — actor is UNTRUSTED on wire).
- `int32 limit` — page size; `0` means "use server default 100"
  (server/go/internal/api/list_shared_with_me.go).
- `int32 offset` — pagination offset; `0` means start.

Response `ListSharedWithMeResponse` (proto:763):
- `repeated Node nodes` — union of (a) same-tenant nodes the caller (or
  any of caller's resolved groups) has a non-`deny` grant on, plus (b)
  nodes from OTHER tenants where `shared_index.user_id == trusted_actor`.
  Cross-tenant entries are de-duplicated against per-tenant results by
  `(tenant_id, node_id)` (server/go/internal/api/list_shared_with_me.go, 1919-1928).
- `bool has_more` — `len(nodes) >= limit` (server/go/internal/api/list_shared_with_me.go). NB: this
  is a Python bug-compatible signal — it does NOT account for the
  cross-tenant append step which can push `len(nodes)` past `limit`. Go
  port must preserve.

Pagination semantics:
- `limit`/`offset` are applied **independently** to each index and then
  merged. So at offset=100 you may see up to `2*limit` nodes, and pages
  can overlap. Documented as a known limitation.
- Order: per-tenant rows ordered by `na.granted_at DESC`
  (server/go/internal/store/), cross-tenant entries ordered by
  `shared_at DESC` (server/go/internal/globalstore/), and the merge is
  per-tenant-first then cross-tenant — NOT a global sort.

## Auth (caller is implicit recipient; trusted-actor)

- Tenant existence: `_check_tenant` (server/go/internal/api/list_shared_with_me.go, 362).
- Trusted actor: `actor = self._trusted_actor(request.context.actor)`
  (server/go/internal/api/list_shared_with_me.go, 418-437). The recipient identity is **never**
  taken from the request payload; it is read from the
  `AuthInterceptor`'s `ContextVar` via `get_authoritative_actor`. This
  is the key invariant — the caller is the implicit recipient, and
  letting the request claim a different recipient would let `user:eve`
  enumerate `user:alice`'s shares.
- No capability check (read-of-own-shares is implicitly authorised by
  identity). Verify against (legacy Python unit test, removed in Phase 4D)
  during port — likely no row, parity is "any authenticated tenant
  member may call".
- Group expansion: `canonical_store.resolve_actor_groups(tenant_id,
  actor)` returns `[actor, group:a, group:b, ...]` — the per-tenant
  query then matches any of those (server/go/internal/api/list_shared_with_me.go,
  server/go/internal/store/).
- Cross-tenant lookup uses **only the bare actor string** (no group
  expansion) — `global_store.get_shared_with_me(user_id=actor, ...)`
  (server/go/internal/api/list_shared_with_me.go). Cross-tenant group-share fan-out happens
  at write time (`ShareNode` expands group → members and writes one
  `shared_index` row per member). See
  (legacy Python unit test, removed in Phase 4D).

## Side effects (read; cross-tenant — items shared from other tenants)

- Read-only on disk. No WAL append, no SQLite write, no audit row.
- **Reads cross-tenant SQLite databases.** For each `shared_index`
  entry from a foreign `source_tenant`, the handler calls
  `canonical_store.get_node(source_tenant, node_id)`
  (server/go/internal/api/list_shared_with_me.go). This is the ONE allowed cross-tenant
  read in EntDB and only legal because the `shared_index` row IS the
  authorisation token — its existence means `ShareNode` already wrote
  a `node_access` row in the source tenant. Per CLAUDE.md invariant 4,
  no SQLite transaction crosses tenants; each `get_node` opens its own
  per-tenant connection.
- Stale `shared_index` entries (source tenant unloaded, node deleted,
  permission revoked but global index lagging) are silently skipped via
  bare `except` (server/go/internal/api/list_shared_with_me.go). Go port must match — these
  are eventual-consistency artefacts, not errors.
- `global_store` failure logs a warning and falls through to per-tenant
  results only (server/go/internal/api/list_shared_with_me.go) — degrades gracefully.
- Metric: `record_grpc_request("ListSharedWithMe", "ok"|"error", elapsed)`
  (server/go/internal/api/list_shared_with_me.go, 1942).

## Error contract

| Condition | Python behaviour | Go port |
|---|---|---|
| Tenant missing/disabled | `_check_tenant` aborts `NOT_FOUND` / `FAILED_PRECONDITION` (with `entdb-redirect-node` trailer if owned elsewhere) | match |
| Forged actor in payload | overwritten by trusted actor; payload value is ignored (no error) | match |
| Caller has no shares | OK, `nodes=[]`, `has_more=false` | match |
| `global_store` unavailable | logged warning; return per-tenant results only | match |
| Stale `shared_index` row (source tenant unloaded, node deleted) | silently skipped | match |
| Per-tenant SQLite error | bare `except`: log + return `nodes=[]` with status OK (server/go/internal/api/list_shared_with_me.go) — error mode swallowed | preserve for v1 parity; flag for review |
| `limit < 0` / `offset < 0` | passed straight to SQLite (negative LIMIT = unlimited in SQLite) — undefined behaviour | Go port: clamp to `[0, maxLimit]`, `maxLimit=1000` |

There is no per-RPC `PERMISSION_DENIED`: identity authorises the read,
and "no shares" is a valid empty success.

## Shared Go package deps (acl, mailbox, global_store)

- `internal/auth` — `GetAuthoritativeActor(ctx)`. The caller IS the
  implicit recipient; no group/capability check needed.
- `internal/tenant` — `CheckTenant(ctx, tenantID)` (with redirect-trailer
  semantics from `_check_tenant`).
- `internal/canonical` (the per-tenant SQLite store) —
  `ResolveActorGroups(ctx, tenantID, actor) []string`,
  `ListSharedWithMe(ctx, tenantID, actorIDs, limit, offset) []*Node`,
  `GetNode(ctx, tenantID, nodeID) *Node` (used cross-tenant).
- `internal/globalstore` — `GetSharedWithMe(ctx, userID, limit, offset)
  []SharedEntry` where `SharedEntry{UserID, SourceTenant, NodeID,
  Permission, SharedAt}`. Backed by the global SQLite (`shared_index`
  table, schema in `server/go/internal/globalstore/`).
- `internal/wire` — `NodeToProto` (id-keyed payload per CLAUDE.md
  invariant 6).
- `internal/metrics` — `RecordGRPCRequest`.
- (Mailbox is NOT a dep here — the listing is over `shared_index`,
  not the mailbox. See "Other-RPC deps" for the historical confusion.)

## Other-RPC deps (overlap with GetMailbox)

- **`ShareNode` writes both indexes**: same-tenant grants write
  `node_access` (used by per-tenant query); cross-tenant grants ALSO
  write `shared_index` rows via `global_store.add_shared`. Group shares
  are fanned out: one `shared_index` row per (member, source_tenant,
  node).
- **`RevokeAccess` removes both**: deletes `node_access` AND
  `global_store.remove_shared` (server/go/internal/api/list_shared_with_me.go). If the
  global delete fails, this RPC will keep returning the revoked node
  until the next ShareNode → applier loop. Same eventual-consistency
  caveat applies.
- **Overlap with `GetMailbox`**: `GetMailbox` lists mailbox **messages**
  (the per-tenant message-passing primitive); `ListSharedWithMe` lists
  **nodes shared via ACL grants**. They are independent surfaces but
  often confused because both answer "things addressed to me". The Go
  port should NOT share an internal helper between the two — different
  storage (`mailbox_messages` table vs `shared_index` + `node_access`),
  different write paths.
- **Capability mapping**: see `test_capability_registry.py` — confirm at
  port time. No capability is required by the handler; identity alone
  authorises.
- All read RPCs share the same auth/tenant helpers; spec lives in
  `docs/go-port/shared/auth.md` and `shared/tenant.md`.

## Contract tests pinning behavior (file:line)

- (legacy Python unit test, removed in Phase 4D) — same-tenant index:
  shared-nodes union, deny exclusion, group-share inclusion via
  `resolve_actor_groups`.
- (legacy Python unit test, removed in Phase 4D) — cross-tenant
  aggregation: `global_store.get_shared_with_me` returns rows from
  multiple `source_tenant`s; per-tenant + cross-tenant union must
  surface all of them.
- (legacy Python unit test, removed in Phase 4D) — group share
  cross-tenant: a `ShareNode` to `group:eng` writes one
  `shared_index` row per group member, so each member sees the node
  on their own `ListSharedWithMe`.
- (legacy Python unit test, removed in Phase 4D) (file header
  reference) and the id-keyed payload assertions — `nodes[*].payload`
  on the wire is field-id-keyed, NOT name-keyed.
- `tests/python/integration/test_grpc_contract.py:339-350` — happy-path
  smoke: well-formed response after a `ShareNode`; check is permissive
  because the in-memory test fixture may or may not have wired
  `global_store`.

Go port adds (`tests/contract/`):
- offset>0 with both indexes populated does NOT skip cross-tenant rows
  (current bug-compat: each index is paginated independently).
- `has_more=true` when per-tenant alone fills the page.
- forged actor in payload returns the trusted actor's shares only.

## Implementation outline

```go
func (s *Server) ListSharedWithMe(ctx context.Context, req *pb.ListSharedWithMeRequest) (*pb.ListSharedWithMeResponse, error) {
    start := time.Now()
    statusLabel := "ok"
    defer func() { metrics.RecordGRPCRequest("ListSharedWithMe", statusLabel, time.Since(start)) }()

    if err := s.checkTenant(ctx, req.Context.TenantId); err != nil { statusLabel = "error"; return nil, err }
    actor := auth.GetAuthoritativeActor(ctx)              // ignore req.Context.Actor

    limit := req.Limit
    if limit <= 0 { limit = 100 }
    if limit > 1000 { limit = 1000 }                       // hardening delta vs Python
    offset := req.Offset
    if offset < 0 { offset = 0 }

    actorIDs, err := s.store.ResolveActorGroups(ctx, req.Context.TenantId, actor)
    if err != nil { statusLabel = "error"; return &pb.ListSharedWithMeResponse{}, nil } // parity: swallow

    // 1. Per-tenant
    nodes, _ := s.store.ListSharedWithMe(ctx, req.Context.TenantId, actorIDs, limit, offset)

    // 2. Cross-tenant from global shared_index
    seen := make(map[string]struct{}, len(nodes))
    for _, n := range nodes { seen[n.TenantID+"\x00"+n.NodeID] = struct{}{} }
    if s.globalStore != nil {
        entries, gerr := s.globalStore.GetSharedWithMe(ctx, actor, limit, offset)
        if gerr != nil {
            log.Warn().Err(gerr).Msg("shared_index read failed in ListSharedWithMe")
        } else {
            for _, e := range entries {
                key := e.SourceTenant + "\x00" + e.NodeID
                if _, dup := seen[key]; dup { continue }
                n, err := s.store.GetNode(ctx, e.SourceTenant, e.NodeID)
                if err != nil || n == nil { continue }   // stale entry; skip silently
                nodes = append(nodes, n)
                seen[key] = struct{}{}
            }
        }
    }

    out := make([]*pb.Node, 0, len(nodes))
    for _, n := range nodes { out = append(out, wire.NodeToProto(n)) }
    return &pb.ListSharedWithMeResponse{Nodes: out, HasMore: int32(len(nodes)) >= limit}, nil
}
```

Notes:
- `grpc.aio` parity: Go's grpc-go is request-per-goroutine; no async
  bridging needed. SQLite calls go through the canonical-store pool.
- Cross-tenant `GetNode` calls fan out serially in Python; consider
  bounded parallelism (sem=8) only if benchmarks demand it. Behaviour
  is independent of order, so safe to parallelise.
- Per CLAUDE.md invariant 4, never open a single SQLite transaction
  spanning two tenants. Each `GetNode` opens its own connection.

## Open questions / risks (pagination; expired/revoked filter)

1. **Pagination is broken-by-design**: `limit`/`offset` apply
   independently to per-tenant and cross-tenant indexes, then merge.
   Page boundaries do not line up. Two options: (a) parity for v1,
   document; (b) implement a unified sort key (e.g. opaque cursor over
   `(shared_at, source_tenant, node_id)`). Recommend (a) + follow-up
   issue. EPIC #407 owner to confirm.
2. **`has_more` is misleading**: `len(nodes) >= limit` after the
   cross-tenant append over-reports `true`. Match Python bug for v1.
3. **Expired/revoked filter**: per-tenant query already filters
   `expires_at` and `permission != 'deny'`
   (server/go/internal/store/). Cross-tenant `shared_index` has NO
   `expires_at` column — an expired share lingers in the global index
   until `RevokeAccess` runs. Go port should match (parity), but file
   issue: add `expires_at` to `shared_index` and filter at
   `get_shared_with_me`.
4. **Stale-entry storm**: if a source tenant is offline,
   `get_node` returns nil and we skip. A user with hundreds of stale
   entries pays N round-trips per call. Consider a periodic GC of
   `shared_index` rows whose `source_tenant` no longer resolves.
5. **No capability gate**: any authenticated tenant member can list
   their own shares. Confirm with EPIC #407 — should there be a
   `READ` capability requirement for symmetry with other read RPCs?
6. **`type_id` filter**: callers may want "show me shared Documents
   only". Not in proto today; track as a v2 feature.
