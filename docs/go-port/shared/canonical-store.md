# CanonicalStore — Go Port Spec

EPIC #407. Per-tenant SQLite materialized view of the WAL. Source of
truth for every read RPC. Rebuildable from the WAL — no data must
exist here that is not also in an event.

Python source: `server/python/entdb_server/apply/canonical_store.py`
(5345 LOC, single class `CanonicalStore`). This spec captures the
contract a Go port must preserve, not a line-for-line translation.

CLAUDE.md invariant #4: **per-tenant SQLite isolation** — each tenant
has its own DB file, no cross-tenant queries inside a single SQLite
transaction. Cross-tenant state (users, tenants, memberships) lives in
`global_store` (separate spec).

## Schema

One physical SQLite file per tenant: `{data_dir}/tenant_{tenant_id}.db`.
Strict id validation at `canonical_store.py:138-162` —
`tenant_id` must match `[A-Za-z0-9_-]{1,128}` or open is rejected
(prevents path-collision data leak). Same alphabet enforced for any id
that becomes a filename.

Mailbox + public files share the `nodes`/`edges` schema (storage modes
TENANT/USER_MAILBOX/PUBLIC, `canonical_store.py:732-892`):

- `tenant_{id}.db` — TENANT mode (default).
- `{tenant_id}/user_{user_id}.db` — USER_MAILBOX (per-user, lazy create).
- `public.db` — singleton PUBLIC.

DDL is a single `executescript` at `canonical_store.py:1037-1200` plus
typed-capability ALTERs at `:1211-1224`. Tables (all keyed by
`tenant_id` first):

| Table | Primary key | Purpose |
|---|---|---|
| `schema_version` | `version` | migration tracking |
| `nodes` | `(tenant_id, node_id)` | node rows; `payload_json` keyed by **field_id** (invariant #6); `acl_blob` legacy JSON |
| `edges` | `(tenant_id, edge_type_id, from_node_id, to_node_id)` | typed edges with `props_json`, `propagates_acl` |
| `node_visibility` | `(tenant_id, node_id, principal)` | denormalized index for principal → visible nodes |
| `applied_events` | `UNIQUE (tenant_id, idempotency_key)` | applier dedup |
| `node_access` | `(node_id, actor_id)` | typed-capability ACL grants (`type_id`, `core_caps_json`, `ext_cap_ids_json`) |
| `group_users` | `(group_id, member_actor_id)` | nested group membership |
| `acl_inherit` | `(node_id, inherit_from)` | structural ACL inheritance |
| `notifications` | `id` | per-user mailbox |
| `read_cursors` | `(user_id, channel_id)` | last-read timestamps |
| `type_metadata` | `type_id` | data_policy + PII fields cache |
| `audit_log` | `event_id` | hash-chained audit (now superseded by S3 Object Lock — invariant #2) |

Indexes (DDL `:1059-1186`): `idx_nodes_type`, `idx_nodes_owner`,
`idx_nodes_updated`, `idx_edges_from/to/type`,
`idx_visibility_principal`, `idx_applied_events_key`,
`idx_access_actor`, `idx_group_users_member`, `idx_inherit_from`,
`idx_notif_user`, `idx_audit_time`, `idx_audit_actor`.

**Lazy expression indexes** (per-type, idempotent `IF NOT EXISTS`,
process-local cache `:439-451`):

- `idx_unique_t<type>_f<field>` — `CREATE UNIQUE INDEX … ON nodes(tenant_id, json_extract(payload_json, '$."<field>"')) WHERE type_id = <type>` (`_ensure_unique_indexes` `:1721`). Replaces the deleted `node_keys` table — see `docs/decisions/unique_keys.md` (superseded section).
- `idx_unique_composite_t<type>_…` — multi-field composite unique (`:1809`).
- `idx_query_t<type>_f<field>` — non-unique twin for `(entdb.field).indexed = true` (`_ensure_query_indexes` `:1876`, `docs/decisions/query_indexes.md`).
- `fts_t<type>` — FTS5 virtual table, `tokenize='porter unicode61'`, `node_id UNINDEXED`, columns `f<field_id>` (`_ensure_fts_table` `:1993`, `docs/decisions/fts.md`).

## Write surface

Only the Applier writes. Every `_sync_*` write helper takes either
`tenant_id` (opens its own connection + transaction) or `conn`
(participates in an outer `batch_transaction`). The Applier path uses
`batch_transaction` (`:1264-1289`) so an entire `TransactionEvent.ops`
list is one `BEGIN IMMEDIATE … COMMIT`; partial application is
impossible by construction.

Methods called by `Applier.apply_event` (see `apply/applier.py`):

- `create_node_raw(conn, …)` `:1291` — inserts `nodes` row, calls `_ensure_field_indexes` `:1942` first, calls `_update_visibility` `:2785`, FTS insert via `_sync_fts_insert` `:2039`.
- `_sync_update_node` `:1612` — partial payload merge, FTS delete-then-insert if any searchable field changed.
- `_sync_delete_node` `:1688` — cascades to `edges`, `node_visibility`, `node_access`, `acl_inherit`, FTS.
- `_sync_create_edge` / `_sync_delete_edge` `:2476` / `:2564` — `INSERT OR REPLACE` on edges; storage-mode hierarchy enforced by `_validate_edge_direction` `:989`.
- `_sync_share_node` / `_sync_revoke_access` / `_sync_delegate_access` / `_sync_revoke_user_access` / `_sync_transfer_ownership` / `_sync_transfer_user_content` `:2962`–`:3774`.
- `_sync_add_group_member` / `_sync_remove_group_member` `:3497` / `:3533`.
- `_sync_create_notification` / `_sync_batch_create_notifications` / `_sync_mark_notification_read` / `_sync_mark_all_read` / `_sync_update_read_cursor` `:4518`–`:4759`.
- `_sync_anonymize_user_in_tenant` `:4890` (GDPR), `_sync_delete_user_drafts` `:893`.
- `_sync_record_applied_event` `:1409` + `_sync_check_idempotency` `:1379` (idempotency on every batch).
- `_sync_append_audit` `:4170` — legacy hash-chained audit; **do not extend** (invariant #2 — S3 Object Lock supersedes).

**Field-id payload contract.** `payload_json` is `{"<field_id_int>": value, …}`. Rename of a proto field is free; rename of a `field_id` is breaking. Translation to/from names happens at the SDK boundary only. JSON columns: `payload_json`, `acl_blob`, `props_json`, `core_caps_json`, `ext_cap_ids_json`, `pii_fields`, audit `metadata`.

**Lazy index hook.** Before `CreateNode`/`UpdateNode` the Applier calls `_ensure_field_indexes` once per `(db_path, type_id)`; the cache `:439-451` is process-local, the `IF NOT EXISTS` clauses keep it correct across restarts.

## Read surface

Called directly from gRPC handlers (`api/grpc_server.py`) — all async,
all dispatched via `_run_sync`. None of these read paths take an outer
connection; each opens-or-borrows the pooled tenant connection.

| Async method | Sync impl | Used by RPC |
|---|---|---|
| `get_node` `:2326` | `_sync_get_node` `:2305` | `GetNode` |
| `get_nodes_by_type` `:2372` | `_sync_get_nodes_by_type` `:2340` | `GetNodes` |
| `query_nodes` `:2450` | `_sync_query_nodes` `:2394` | `QueryNodes` (uses `compile_query_filter` from `apply/query_filter.py` — Mongo-style operator allow-list, parameterised SQL fragment) |
| `search_nodes` `:2157` | `_sync_search_nodes` `:2101` | `SearchNodes` (FTS5 `MATCH … ORDER BY rank`) |
| `get_node_by_key` `:2215` | `_sync_get_node_by_key` `:2177` | unique-key lookup; planner picks `idx_unique_t…` automatically |
| `get_node_by_composite_key` `:2270` | `_sync_get_node_by_composite_key` `:2229` | composite unique |
| `get_visible_nodes` `:2761` | `_sync_get_visible_nodes` `:2715` | ACL-pre-filtered enumeration via `node_visibility` JOIN |
| `get_edges_from` `:2642` | `_sync_get_edges_from` `:2609` | `GetEdgesFrom` |
| `get_edges_to` `:2695` | `_sync_get_edges_to` `:2662` | `GetEdgesTo` |
| `get_connected_nodes` (via `_sync_get_connected_nodes` `:3267`) | | `GetConnectedNodes` |
| `_sync_get_edges_and_nodes` `:3387` | | bulk fetch |
| `list_shared_with_me` `:3481` | `_sync_list_shared_with_me` `:3444` | `ListSharedWithMe` |
| `_sync_get_acl_grants` `:3099` | | ACL inspection |
| `list_node_access_for_group` `:3609` | `_sync_list_node_access_for_group` `:3592` | console |
| `_sync_has_node_access` / `_sync_can_access` `:3956` / `:2867` | | ACL post-filter |
| `_sync_get_notifications` / `_sync_get_unread_count` / `_sync_get_unread_channels` `:4562` / `:4675` / `:4826` | | mailbox/notifications |

**ACL post-filter pattern.** `node_visibility` is an optimization for
"enumerate what I can see"; full ACL evaluation (typed capabilities,
groups, inheritance) runs through `_sync_can_access` `:2867` and the
group-resolution helper `_sync_resolve_actor_groups` `:2828` (recursive,
`_ACL_MAX_DEPTH = 10`). Read RPCs that return lists call `_can_access`
per row before emitting (server-side, never trust the client).

**Pagination.** Every list endpoint takes `limit`, `offset`. Order
keys are validated against an allow-list (e.g. `_sync_query_nodes`
`:2415-2417`) — no user string is ever interpolated into SQL.

**Read-after-write consistency.** `wait_for_offset` `:478` (and
`update_applied_offset` `:463`) — `asyncio.Condition` per tenant; the
applier signals after each batch. Read RPCs that pass
`after_offset` + `wait_timeout_ms` block until the materialized view
catches up (default 30 s).

## Concurrency

- **WAL mode + busy_timeout.** `PRAGMA journal_mode=WAL`, `synchronous=NORMAL`, `busy_timeout=5000ms`, `cache_size=-64000` (~64 MiB) — `_get_connection` `:711-717`. SQLite WAL mode allows one writer + many readers without blocking readers.
- **One pooled connection per file.** `self._connections` keyed by physical path (`:683`, `:783`). `check_same_thread=False`, `isolation_level=None` (autocommit; explicit `BEGIN IMMEDIATE`/`COMMIT`).
- **Per-tenant `threading.Lock`** (`_get_tenant_lock` `:642`) serializes same-tenant SQLite calls so the single pooled connection never sees overlapping `execute()`s from different threads. Acquired BEFORE the cache check (`:690`) to avoid orphaned-FD races on first open.
- **Per-tenant `asyncio.Semaphore(1)`** (`_get_tenant_semaphore` `:605`) — fair-share guard at the asyncio layer so a single noisy tenant cannot occupy all 32 thread-pool workers and starve others. Keyed by `(tenant_id, loop_id)` for test isolation.
- **Thread pool.** `ThreadPoolExecutor(max_workers=min(32, cpu+4))` `:420-425`, `thread_name_prefix="entdb-sqlite"`. All `_sync_*` calls dispatched via `_run_sync` `:536`.
- **Single-writer guarantee.** `BEGIN IMMEDIATE` in `batch_transaction` `:1283` — the per-tenant lock + WAL mode mean readers proceed concurrently while the applier holds the writer slot.
- **Applier replay behavior.** During replay the Applier opens one `batch_transaction` per WAL batch; reads in flight see the pre-batch snapshot, then the post-batch snapshot atomically. Pinned by `tests/python/integration/test_concurrent_applier_reads.py` (no half-applied transaction visible; no deadlock; final reads see every committed write).

## Go design

Proposed location: **`server/go/internal/store/`** (currently only
`server/go/README.md` exists). Split:

```
server/go/internal/store/
  canonical.go        — type CanonicalStore (per-tenant ops)
  schema.go           — DDL + lazy index/FTS creation
  pool.go             — connection pool, per-tenant locks, executor
  txn.go              — BatchTransaction wrapper (BEGIN IMMEDIATE)
  visibility.go       — _update_visibility helpers
  acl_engine.go       — typed-capability evaluation (mirrors :2823+)
  fts.go              — _ensure_fts_table + search
  indexes.go          — _ensure_unique/query/composite indexes
  notifications.go    — mailbox + read cursors
  audit.go            — legacy hash-chain (frozen — see invariant #2)
  errors.go           — TenantNotFoundError, IdempotencyViolationError
```

**Driver choice.** Two viable options:

| | `modernc.org/sqlite` | `mattn/go-sqlite3` |
|---|---|---|
| CGO | no (pure Go transpile of SQLite) | yes |
| FTS5 | yes (built in) | yes (with `sqlite_fts5` build tag) |
| `json_extract` | yes | yes |
| Cross-compile | trivial — single binary, alpine-friendly | needs CC per target; complicates Docker multi-arch |
| Performance | ~10–20 % slower than CGO on hot paths | fastest |
| Encryption (SEE/SQLCipher) | no | via SQLCipher fork |
| Licence | BSD-3 | MIT |
| Maturity | production at large scale (cznic/ql lineage) | de-facto Go standard |

Recommendation: **`modernc.org/sqlite`** for the default build —
matches the "single static binary, no CGO toolchain" Go-server posture
and removes Docker cross-compile pain. Keep the driver behind an
interface (`store.Driver`) so a `mattn` build tag can be added if
encryption-at-rest (SQLCipher) re-enters scope.

**Type sketch:**

```go
type CanonicalStore struct {
    dataDir       string
    walMode       bool
    busyTimeout   time.Duration
    cacheSizeKB   int
    encryption    *EncryptionConfig
    pool          *connPool       // path -> *sql.DB (pooled at driver level)
    tenantMu      *keyedMutex     // per-tenant serializer
    appliedOffset *offsetTracker  // tenant -> stream pos + sync.Cond
    indexCache    *indexCache     // (path, type_id) -> set
}

func (s *CanonicalStore) GetNode(ctx context.Context, tenantID, nodeID string) (*Node, error)
func (s *CanonicalStore) QueryNodes(ctx context.Context, q QueryNodesArgs) ([]*Node, error)
func (s *CanonicalStore) BatchTxn(ctx context.Context, tenantID string, fn func(*Tx) error) error
// …
```

Async-via-thread-pool dance from Python disappears — Go's
`database/sql` gives us native goroutines and a per-`*sql.DB`
connection pool. We still need the per-tenant mutex if we keep one
`*sql.DB` per tenant DB path with `SetMaxOpenConns(1)` (matches the
Python single-pooled-connection model and serializes writers without
WAL contention surprises). Alternative: `SetMaxOpenConns(N)` + rely on
SQLite's WAL — needs benchmarking against the Python baseline before
choosing.

## Dependencies

- **proto/pb** — `entdb/v1` generated types: `Node`, `Edge`, `Operation`, `TransactionEvent`, `RequestContext`, FieldOpts. Imported as `pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb/entdb/v1"`.
- **schema** — `NodeTypeDef`, `EdgeTypeDef`, `SchemaRegistry` (Go port of `schema/registry.py` + `schema/types.py`). Provides `GetUniqueFieldIDs`, `GetIndexedFieldIDs`, `GetSearchableFieldIDs`, `GetCompositeUnique` per type.
- **acl** — `Permission` enum + typed `CoreCap`/`ExtCap` (Go port of `auth/permissions.py`). Used by `node_access` writes and `_sync_can_access` evaluation.
- **errs** — typed errors: `TenantNotFound`, `IdempotencyViolation`, `UniqueConstraintError`, `ACLDenied`. Mapped to gRPC codes at the handler boundary.
- **query_filter** — Go port of `apply/query_filter.py` (Mongo-style operator allow-list → SQL fragment). Same operator set, same parameterisation rules.
- **metrics** — Prometheus counters for `sqlite_op{type=read|write}` (mirrors `_record_sqlite_op` `:83-84`).

## RPCs touching this

**Read RPCs (every one calls a `_sync_*` here):**
`GetNode`, `GetNodes`, `QueryNodes`, `SearchNodes`, `GetNodeByKey`
(spec yet to be added — uses `get_node_by_key`), `GetEdgesFrom`,
`GetEdgesTo`, `GetConnectedNodes`, `ListSharedWithMe`, `GetMailbox`,
`SearchMailbox`, `ListMailboxUsers`, `GetReceiptStatus`,
`GetTenantMembers`, `WaitForOffset`.

**Write RPCs (all funnel through `ExecuteAtomic` → WAL → Applier →
`batch_transaction` here):** `ExecuteAtomic`, `ShareNode`,
`RevokeAccess`, `TransferOwnership`, `AddTenantMember`,
`ChangeMemberRole`, `AddGroupMember`, `RemoveGroupMember`,
`CreateTenant`, `ArchiveTenant`. (`CreateUser`, `UpdateUser`, etc. hit
`global_store`, not this store — separate spec.)

See per-RPC files in `docs/go-port/rpcs/` for handler-side detail.

## Test surface

**Fixture pattern for Go integration tests:**

```go
func newTestStore(t *testing.T) *store.CanonicalStore {
    dir := t.TempDir()
    s, err := store.Open(dir, store.Options{WAL: true, BusyTimeout: 5 * time.Second})
    require.NoError(t, err)
    t.Cleanup(func() { s.Close() })
    return s
}
```

In-memory mode (`:memory:` per tenant) is **not** supported because
the per-tenant file-path identity is load-bearing for the pool; tests
use `t.TempDir()` instead — same shape as `tests/python/integration/`
fixtures.

Behaviours to pin (Go ports of existing Python tests):

- `tests/python/unit/test_canonical_store.py` — schema creation, basic CRUD, idempotency dedup.
- `tests/python/unit/test_canonical_store_concurrency.py` — per-tenant lock correctness, no FD leaks under racing opens.
- `tests/python/unit/test_canonical_store_perf.py` — sanity perf budgets (Go SHOULD beat Python on every line).
- `tests/python/integration/test_concurrent_applier_reads.py` — readers + applier contract: no half-applied transaction observable, no deadlocks, every committed write visible post-test.

Cross-implementation contract tests live in `tests/contract/` (per
`CLAUDE.md` project layout); the Go store must pass the same fixtures.

## Open questions / risks

1. **Driver licensing + CGO.** `modernc.org/sqlite` removes CGO at the cost of ~15 % perf on hot paths and no SQLCipher. If encryption-at-rest is required (`encryption.py`, `derive_tenant_key`), we must either (a) keep `mattn/go-sqlite3` + SQLCipher, (b) implement application-level field encryption above the driver, or (c) defer encryption to filesystem-level (LUKS/EBS) and drop SQLCipher. Decision needed before GA.
2. **Per-tenant file lifecycle.** Python lazy-creates files on first write and never closes them until process exit. With many idle tenants this leaks FDs. Go port should add an LRU eviction (`evict_mailbox_connection` `:955` is the only existing eviction hook). Threshold + policy TBD.
3. **FTS5 availability.** `modernc.org/sqlite` ships FTS5 by default; `mattn/go-sqlite3` requires the `sqlite_fts5` build tag — easy to forget. Add a startup self-test (`SELECT fts5_version()`) that aborts boot on a missing FTS5.
4. **`busy_timeout` tuning.** 5 s is the Python default; under bursty multi-tenant load with one `*sql.DB` per file we may need higher. Bench against the integration suite before locking in.
5. **`SetMaxOpenConns(1)` vs N.** Python serializes via threading.Lock; Go could trust WAL and let N readers proceed without an explicit mutex. Risk: `INSERT OR REPLACE` semantics + lazy DDL inside a transaction need verification under concurrent readers. Decide via benchmark, not vibes.
6. **type_id ignored in `GetNode`.** Already noted in `docs/go-port/rpcs/GetNode.md` — wire field is present but unused. Fix (filter on `type_id`) or formally delete from the proto in v2; tracked separately from this spec.
7. **Audit log table.** `audit_log` + hash chain still ships in DDL but invariant #2 says S3 Object Lock supersedes. Go port keeps the table for backwards-compat read-only queries; **do not extend**, do not write new audit rows from the Go applier.
8. **Pool key by physical path.** Path is a string; on macOS case-insensitive FS this is a footgun. Validator alphabet excludes upper/lower collisions in IDs but not in `data_dir`. Document the assumption (`data_dir` must be on a case-sensitive FS) or canonicalize via `filepath.EvalSymlinks` at open.
