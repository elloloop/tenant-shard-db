# Go Port Spec — `globalstore` (cross-tenant SQLite)

EPIC #407 component spec. Source of truth in Python:
`server/python/entdb_server/global_store.py` (1333 lines, single class
`GlobalStore`). The Go reimplementation lives at
`server/go/internal/globalstore/`.

This is the only place cross-tenant state may be read or written
(CLAUDE.md invariant #4). It owns its own SQLite file at
`{data_dir}/global.db`, distinct from per-tenant `canonical_store` files.

## Schema

All tables live in one file (`global.db`), all timestamps are integer
Unix epoch **seconds** unless suffixed `_ms`. Schema is created
idempotently on startup; column additions use `ALTER TABLE ... ADD
COLUMN` guarded by `PRAGMA table_info` (see `_migrate_tenant_quotas`,
`_migrate_tenant_registry_region` at `global_store.py:187,280`).

| Table | Purpose | PK | Notes |
|---|---|---|---|
| `user_registry` | identity registry | `user_id` | `email UNIQUE`, `status` ∈ {active, suspended, pending_deletion, deleted}, `created_at`, `updated_at` (`global_store.py:203`) |
| `tenant_registry` | tenant metadata + region pin | `tenant_id` | `status` ∈ {active, archived, legal_hold, deleted}, `region` defaults `us-east-1` (`global_store.py:212,217`) |
| `tenant_members` | user→tenant mapping with role | `(tenant_id, user_id)` | `role` is free-text (`owner`, `admin`, `member`); secondary index on `user_id` (`global_store.py:220-228`) |
| `shared_index` | cross-tenant share hint | `(user_id, source_tenant, node_id)` | `permission` text; **hint-only**, authoritative ACLs live in canonical_store (`global_store.py:230,755`) |
| `deletion_queue` | GDPR right-to-erasure scheduler | `user_id` | `requested_at`, `execute_at`, `export_path`, `status` ∈ {pending, completed} (`global_store.py:239`) |
| `legal_holds` | per-tenant legal hold records | `(tenant_id, held_by)` | informational; the `tenant_registry.status='legal_hold'` flag is what gates writes (`global_store.py:247`) |
| `tenant_quotas` | rate-limit / monthly write config | `tenant_id` | monthly cap + Phase-2/3 token-bucket columns (`global_store.py:255`) |
| `tenant_usage` | rolling write counters | `tenant_id` | `period_start_ms` is start-of-UTC-month in **milliseconds** (`global_store.py:266`) |

Identifier conventions:

- `user_id` — bare id, e.g. `"alice"`. This is what's stored in
  `user_registry.user_id`, `tenant_members.user_id`,
  `shared_index.user_id`, `deletion_queue.user_id`.
- `tenant_id` — bare id, e.g. `"acme"`.
- Mailbox routing: there is **no** `mailbox_index` or `region_pinning`
  table. Region is a column on `tenant_registry`. The legacy mailbox
  store is gone (see `grpc_server.py:1461,1483,1610` — RPCs
  deprecated, return empty).

## Operations

Async public methods that thread through a single-thread executor (one
SQLite connection) via `_run_sync`. Each `async def foo` has a paired
`_sync_foo` (`global_store.py:312`).

**Reads:**

- `get_user(user_id)` → row or `None` (`global_store.py:371`)
- `list_users(status='active', limit, offset)` (`global_store.py:411`)
- `get_tenant(tenant_id)` (`global_store.py:489`)
- `list_tenants(status='active')` — no pagination (`global_store.py:504`)
- `get_members(tenant_id)` (`global_store.py:598`)
- `get_user_tenants(user_id)` (`global_store.py:614`)
- `is_member(tenant_id, user_id)` (`global_store.py:648`)
- `get_shared_with_me(user_id, limit, offset)` (`global_store.py:713`)
- `get_shared_entries_for_node(source_tenant, node_id)` (`global_store.py:774`)
- `get_pending_deletions()` / `get_executable_deletions(now)` /
  `get_deletion_entry(user_id)` (`global_store.py:850,865,887`)
- `get_legal_holds(tenant_id)` / `is_under_legal_hold(tenant_id)` (`global_store.py:1021,1040`)
- `get_quota_config(tenant_id)` / `get_usage(tenant_id)` (`global_store.py:1193,1227`)

**Writes:**

- `create_user(user_id, email, name)` — UNIQUE on email; raises `IntegrityError` on duplicate (`global_store.py:336`)
- `update_user(user_id, **{email,name,status})` (`global_store.py:386`)
- `set_user_status(user_id, status)` (`global_store.py:434`)
- `create_tenant(tenant_id, name, region='us-east-1')` (`global_store.py:453`)
- `set_tenant_status(tenant_id, status)` (`global_store.py:520`)
- `set_legal_hold(tenant_id, enabled, actor)` — flips
  `tenant_registry.status` between `legal_hold` and `active` (`global_store.py:528`)
- `add_member` / `remove_member` / `change_role` (`global_store.py:558,582,630`)
- `add_shared` (`INSERT OR REPLACE`) / `remove_shared` /
  `cleanup_stale_shared(source_tenant, node_id)` /
  `remove_all_shared_for_user(user_id)` (`global_store.py:670,697,751,736`)
- `queue_deletion(user_id, grace_days=30)` /
  `cancel_deletion(user_id)` /
  `mark_deletion_completed(user_id)` (`global_store.py:800,834,898`)
- `remove_all_memberships_for_user(user_id)` (`global_store.py:910`)
- `transfer_user_content(tenant_id, from_user, to_user)` —
  ensures `to_user` is a member; **does not** touch canonical_store
  (`global_store.py:921`)
- `set_legal_hold_record(tenant_id, held_by, reason)` /
  `remove_legal_hold(tenant_id, held_by)` — `legal_holds` table only;
  separate from `set_legal_hold` (`global_store.py:967,1001`)
- `revoke_user_access(tenant_id, user_id)` — deletes membership +
  scoped shared_index in one txn (`global_store.py:1061`)
- `set_quota_config(...)` (UPSERT) (`global_store.py:1102`)
- `increment_usage(tenant_id, n_writes)` — calendar-month rollover
  baked in (`global_store.py:1256`)
- `reset_period(tenant_id)` (`global_store.py:1310`)

## WAL coupling

**`GlobalStore` has no WAL.** All writes are direct SQLite operations
inside the single connection's autocommit/manual-txn model. This is a
documented carve-out from CLAUDE.md invariant #1 ("all writes go
through the WAL"); the Python implementation also reflects this.

Coupling to the per-tenant WAL pipeline:

1. **Applier** consumes `TransactionEvent`s for one tenant and, as a
   side-effect, writes to `global_store.shared_index` for `share` /
   `revoke` / `delete_node` events
   (`apply/applier.py:1709,1716,1745,1751,1772`). It also calls
   `increment_usage` after each successful commit
   (`apply/applier.py:1319`). These are **best-effort**: errors are
   logged, not raised — the WAL has already been durable.
2. **Handlers** (gRPC) write directly to `global_store` for ops that
   have no per-tenant analogue: `CreateUser`, `CreateTenant`,
   `AddTenantMember`, `RemoveTenantMember`, `ChangeMemberRole`,
   `UpdateUser`, `ArchiveTenant`, GDPR delete-account, legal hold
   admin ops (see `grpc_server.py:2127,2335,2477,2534,2637,2429,2950`
   and `admin_handlers.py:42,130,156`).
3. **Quotas/usage** are read by `quota_interceptor`
   (`auth/quota_interceptor.py:278,314`).

**Recovery story:** because there's no WAL, `global.db` is itself the
durable record for cross-tenant state. SQLite `journal_mode=WAL` +
`synchronous=NORMAL` (`global_store.py:172-174`) is the only crash
safety. There is no rebuild-from-events path. **This is an open
question for the Go port** (see "Open questions").

## Identifier translation

CLAUDE.md key pattern: `user_id "alice"` (bare) vs
`tenant_principal "user:alice"` (prefixed). `GlobalStore` deals
**exclusively in bare ids** — every column named `user_id` stores
`alice`, never `user:alice`.

Translation happens at the gRPC boundary in
`grpc_server.py`: helpers like `_extract_user_id` strip `user:`
(`grpc_server.py:413,1584,2299`). The Go port must enforce the same
contract: `globalstore` package types take `UserID string` (bare); any
prefix-stripping happens in the gRPC layer (server/go/internal/api).

Group actors (`group:admins`) never appear as a `user_id` in
`shared_index`; the Applier expands groups via
`canonical_store.get_group_members` and inserts one row per resolved
member (`apply/applier.py:1707`).

## Concurrency

Single SQLite connection guarded by a single-thread `ThreadPoolExecutor`
(`global_store.py:132`). All `async` methods funnel through
`_run_sync` (`global_store.py:312`), serializing writes. WAL mode is
enabled so concurrent reads from sibling processes (e.g. tests) are
allowed, but the in-process model is "one writer, no contention".

Go port choice: a `*sql.DB` with `SetMaxOpenConns(1)`, or a
hand-rolled mutex around a `*sql.Conn`. Either preserves the
"no two goroutines touch SQLite simultaneously" property and avoids
`SQLITE_BUSY` retry loops in hot paths. Prefer `MaxOpenConns(1)` —
matches the Python invariant exactly.

## Go design

```
server/go/internal/globalstore/
    globalstore.go        // type GlobalStore + constructor + close
    schema.go             // CREATE TABLE statements + migrations
    users.go              // user_registry CRUD
    tenants.go            // tenant_registry CRUD
    members.go            // tenant_members CRUD + is_member
    shared.go             // shared_index ops
    deletion.go           // deletion_queue ops (GDPR)
    legalhold.go          // legal_holds + status flag
    quota.go              // tenant_quotas + tenant_usage
    encryption.go         // open_encrypted_connection adapter
    globalstore_test.go   // table-driven parity tests
```

Sketch:

```go
type GlobalStore struct {
    db        *sql.DB           // MaxOpenConns(1)
    dataDir   string
    busyMS    int
    walMode   bool
    encCfg    *EncryptionConfig // optional
    nowFn     func() int64      // injectable for tests
}

func New(opts Options) (*GlobalStore, error)
func (g *GlobalStore) Close() error

// ctx-first signatures everywhere — Go convention, no _run_sync wrapper needed
func (g *GlobalStore) CreateUser(ctx context.Context, id, email, name string) (*pb.UserInfo, error)
func (g *GlobalStore) GetTenant(ctx context.Context, id string) (*pb.TenantInfo, error)
// ...
```

Return typed protobuf messages directly (`pb.UserInfo`,
`pb.TenantInfo`, `pb.Membership`) rather than `map[string]any`; the
Python `dict` returns are an artifact of dynamic typing.

Errors: domain sentinels in `internal/errs` —
`ErrUserNotFound`, `ErrTenantNotFound`, `ErrEmailTaken`,
`ErrMembershipExists`, `ErrAlreadyQueued`. Map SQLite `UNIQUE`
violations at the boundary, never leak `sqlite3.IntegrityError`
analogues.

## Dependencies

- `server/go/internal/pb/entdb/v1` — generated proto types
  (`UserInfo`, `TenantInfo`, `Member`, `Membership`).
- `server/go/internal/errs` — typed errors.
- `database/sql` + `modernc.org/sqlite` (pure-Go) or
  `mattn/go-sqlite3` (cgo). Match whatever `canonical_store` chooses.
- `server/go/internal/encryption` — encrypted connection helper,
  parity with Python `encryption.derive_global_key` /
  `open_encrypted_connection` (`global_store.py:80,156`).
- `time` for epoch-second helpers; a `nowFn` field for test injection.

No dependency on `wal/`, `applier/`, or `canonical_store`. Other
components depend on `globalstore`, never the reverse.

## RPCs touching this

From `grpc_server.py` and `admin_handlers.py` (audit complete via
grep at line refs above):

**Reads:**
`ListTenants`, `GetTenant`, `ListUsers`, `GetUser`,
`GetTenantMembers`, `GetUserTenants`, `ListSharedWithMe`,
`Health` (indirectly via tenant existence check at line 401, 503),
`ExecuteAtomic` (membership check at 599), `GetUsage` (3133-3134).

**Writes:**
`CreateUser`, `UpdateUser`, `CreateTenant` (also `add_member` for
creator), `ArchiveTenant`, `AddTenantMember`, `RemoveTenantMember`,
`ChangeMemberRole`, `ShareNode` (Applier path),
`RevokeAccess` (Applier path + `admin_handlers.revoke_user_access`),
`TransferOwnership` (`admin_handlers.transfer_user_content`),
`SetLegalHold` / admin legal-hold endpoints,
`DeleteAccount` / `CancelAccountDeletion` /
`SuspendUser` / `ReactivateUser` (`grpc_server.py:2950-3093`),
`SetQuotaConfig` (admin).

GDPR worker (`gdpr_worker.py:113-173`): scans deletion_queue,
fans out to per-tenant deletion, then clears memberships, shared,
and marks status=`deleted`.

## Test surface

Existing Python tests that establish parity expectations (port to Go):

- `tests/python/unit/test_user_registry.py` — user CRUD, email uniqueness, status transitions.
- `tests/python/unit/test_tenant_registry.py` — tenant CRUD, region default.
- `tests/python/unit/test_tenant_roles.py` — membership + role changes.
- `tests/python/unit/test_listtenants_auth.py` — `ListTenants` RBAC; reads `get_user_tenants`.
- `tests/python/unit/test_cross_tenant_read.py` — invariant #4 enforcement.
- `tests/python/unit/test_tenant_key_vault.py` — encryption parity.
- `tests/python/integration/test_region_pinning.py` — `region` column round-trip.
- `tests/python/integration/test_privilege_escalation.py` — actor / `user:` boundary.
- Cross-implementation contract tests in `tests/contract/` should
  exercise: GDPR queue, legal hold gating of writes, quota
  enforcement, share/revoke side-effects.

New Go tests (`globalstore_test.go`):

- Schema migration: open old-shape DB (no `region`, no Phase-2 quota
  columns), assert ALTER ran.
- Concurrent caller goroutines vs `MaxOpenConns(1)` — no
  `SQLITE_BUSY`, FIFO-ish.
- Calendar-month rollover in `IncrementUsage` (inject `nowFn`).
- `RevokeUserAccess` atomicity — both deletes succeed or neither.

## Open questions / risks

1. **Recovery / durability.** The Python store relies entirely on
   `global.db` being durable. Snapshot/Archive (server/python/.../snapshot.py)
   does **not** capture `global.db`. For the Go port, decide:
   (a) keep status quo and document it as a non-replayable substrate,
   (b) emit a parallel global-WAL stream, or
   (c) snapshot `global.db` to S3 alongside per-tenant snapshots.
   Recommend (c) — minimal new code, preserves the "S3 Object Lock is
   the audit trail" invariant.
2. **`shared_index` is a hint.** Python comments call it
   "not authoritative" (`global_store.py:755`). Authoritative ACLs
   live in canonical_store. The Go port must preserve this — never
   gate access checks on `shared_index` alone.
3. **`set_legal_hold` vs `legal_holds` table double-write.** Two
   independent code paths exist (`set_legal_hold` flips
   `tenant_registry.status`; `set_legal_hold_record` inserts into
   `legal_holds`). Today they're called from different RPCs and can
   drift. Consider unifying in Go behind one `SetLegalHold(record)`
   API.
4. **No transactional boundary across stores.** A failure between
   `global_store.create_tenant` and `global_store.add_member` (creator
   becomes owner, `grpc_server.py:2335,2344`) leaves a tenant with no
   owner. Wrap related ops in a single SQLite transaction in Go
   (currently each `_sync_*` opens its own implicit txn).
5. **Mailbox routing has no table.** Several RPCs (`SearchMailbox`,
   `GetMailbox`, `ListMailboxUsers`) are deprecated stubs. Confirm
   the Go port can drop them entirely (proto deprecated?) before
   building.
6. **`tenant_usage` rollover races.** Two concurrent
   `IncrementUsage` at month-boundary may both insert / both update.
   Python is safe via single-thread executor; Go must keep
   `MaxOpenConns(1)` or wrap rollover in a transaction with
   `ON CONFLICT` semantics.
