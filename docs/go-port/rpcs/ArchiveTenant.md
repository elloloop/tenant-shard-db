# RPC Port Spec — `entdb.v1.EntDBService/ArchiveTenant`

EPIC #407 — Python → Go server port. Source of truth: Python handler at
`server/python/entdb_server/api/grpc_server.py:2397-2440`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:120` (rpc), `:890-898` (messages).

`ArchiveTenantRequest`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `actor` | 1 | `string` | Wire-claimed identity. **UNTRUSTED** — must be replaced with `AuthInterceptor` authoritative actor before any privilege decision (see Auth). Required (`INVALID_ARGUMENT` when empty). |
| `tenant_id` | 2 | `string` | Tenant to archive. Required. No format validation today; treat opaque. |

`ArchiveTenantResponse`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `success` | 1 | `bool` | True iff tenant existed AND status update applied. |
| `error` | 2 | `string` | Human-readable error when `success=false`. Currently set to `"Tenant not found"` (missing) or `str(e)` (caught exception). Free-form; do not parse. |

Note: there is no `RequestContext` wrapper on this RPC — `actor` is a top-level
field. The Go port must NOT introduce one (wire compat).

## Auth (admin only; trusted-actor)

CLAUDE.md mandates: handlers replace `request.actor` with the interceptor-bound
trusted identity before any policy check. Python sequence
(`grpc_server.py:2419-2427`):

1. `trusted_actor = self._trusted_actor(request.actor)` —
   delegates to `auth.auth_interceptor.get_authoritative_actor` which returns
   the `ContextVar` value set by `AuthInterceptor` if present, else falls back
   to the wire string (no-auth deployments / unit tests only).
2. `actor_uid = self._actor_user_id(trusted_actor)` strips `user:` prefix.
3. `if not self._is_admin_or_system(trusted_actor):` — i.e. trusted prefix is
   `system:`, `admin:`, or equals `__system__` (`grpc_server.py:2053-2069`).
4. Otherwise look up `_get_member_role(tenant_id, actor_uid)`; only `"owner"`
   may archive. Anything else → `PERMISSION_DENIED`.

The Go port MUST mirror exactly this order — privilege string check first
(short-circuit for system/admin), membership lookup only for non-admin
callers. The interceptor-derived actor is the single source of truth; the
wire `actor` field is rebound and never reused for authz.

`ArchiveTenant` is NOT in `AuthInterceptor.UNAUTHENTICATED_METHODS`
(`auth/auth_interceptor.py:157-162`); standard auth path applies.

## Side effects (WAL append; archive workflow handoff; data export to cold storage)

**Current Python behaviour (gap, see Open questions):** the handler performs
ONE side effect — `self.global_store.set_tenant_status(tenant_id, "archived")`
(`grpc_server.py:2429`, impl at `global_store.py:520-554`). That is a direct
SQLite UPDATE on `tenant_registry.status` in the global store DB. Subsequent
behavioural impact is enforced lazily by `_check_tenant_access`
(`grpc_server.py:518-522`): writes against an archived tenant return
`FAILED_PRECONDITION`; reads still succeed. No legal-hold side effects.

**What the handler does NOT do today** (intentional flags for the Go port):

- **No WAL append.** The status flip is a direct global-store write,
  bypassing the event log. This violates CLAUDE.md invariant #1 ("All writes
  go through the WAL") for the per-tenant case but `global_store` is a
  cross-tenant store with its own SQLite — see Open questions.
- **No archiver handoff.** The `Archiver` subsystem
  (`server/python/entdb_server/archive/archiver.py`) is a *WAL-stream* archiver
  (Kafka/Kinesis → S3 segments); it is NOT triggered by `ArchiveTenant`. Name
  collision only.
- **No cold-storage export.** No S3 `StorageClass` transition, no per-tenant
  SQLite snapshot to Glacier, no compaction.
- **No member revocation, no API-key revocation, no shard reassignment.**

**Go-port target behaviour** (proposal, requires product sign-off):
1. Validate inputs.
2. Resolve trusted actor; authorize.
3. Append a `TenantStatusChanged{tenant_id, from, to: "archived", actor, ts}`
   event to the **global-store WAL stream** (new event type — see Open
   questions for whether global_store should have its own WAL or piggy-back
   on a control-plane stream).
4. `Applier` consumes the event and updates `tenant_registry.status`.
5. Optionally enqueue a cold-storage workflow (out-of-band worker) that
   compacts the tenant's SQLite + recent WAL segments to `STANDARD_IA` or
   `GLACIER` per `s3_config.s3_storage_class`. Reversible until the workflow
   completes; one-way after.
6. Return `success` reflecting the WAL append result, NOT the materialized
   view (consistent with other write RPCs).

## Error contract

| Condition | gRPC code | Path |
|-----------|-----------|------|
| `global_store` not configured | `UNIMPLEMENTED` | `grpc_server.py:2406-2409` |
| Empty `actor` | `INVALID_ARGUMENT` | `:2411-2412` |
| Empty `tenant_id` | `INVALID_ARGUMENT` | `:2413-2414` |
| Non-owner, non-admin/system caller | `PERMISSION_DENIED` | `:2421-2427` |
| Tenant id not found in registry | `OK` + `success=false`, `error="Tenant not found"` | `:2431-2433` |
| Any other exception | `OK` + `success=false`, `error=str(e)` | `:2437-2440` |

Note the asymmetry: auth/validation failures abort with a gRPC status code;
post-auth lookup misses and unexpected errors return `OK` with
`success=false`. The Go port MUST preserve this — the contract test
explicitly asserts the happy path returns `success=True` and the empty-actor
case returns `INVALID_ARGUMENT`
(`tests/python/integration/test_grpc_contract.py:683-693`).

`record_grpc_request("ArchiveTenant", "ok"|"error", elapsed)` is emitted on
every exit (`grpc_server.py:2432, 2435, 2438`); Go port must wire equivalent
metrics.

## Shared Go package deps (`archive/` subsystem, wal, store)

| Python module | Go package (proposed) | Use |
|---------------|-----------------------|-----|
| `entdb_server/global_store.py` | `internal/globalstore` | `SetTenantStatus(ctx, tenantID, status) (bool, error)`; `GetMemberRole(ctx, tenantID, userID) (string, error)` |
| `entdb_server/auth/auth_interceptor.py` | `internal/auth` | `GetAuthoritativeActor(ctx)` resolves the trusted actor from the gRPC context (replaces Python `ContextVar`). |
| `entdb_server/api/grpc_server.py` (helpers) | `internal/grpcsvc/actor` | `TrustedActor`, `ActorUserID`, `IsAdminOrSystem` helpers — shared across all admin handlers. |
| `entdb_server/archive/archiver.py` | `internal/archive` (existing for WAL→S3) | **Not used by this RPC today.** Spec'd here only to document the name collision. |
| `entdb_server/wal/*` | `internal/wal` | Future: append `TenantStatusChanged` event when invariant #1 is properly extended to global_store. |
| `entdb_server/api/generated` (proto stubs) | `gen/go/entdb/v1` | `ArchiveTenantRequest`, `ArchiveTenantResponse`. |
| metrics (`record_grpc_request`) | `internal/metrics` | Per-RPC counter+histogram. |

No dependency on per-tenant `canonical_store`, `Applier`, `SchemaRegistry`,
or any per-tenant SQLite. This RPC operates entirely on the global store.

## Other-RPC deps

`ArchiveTenant` shares helpers and side-effect surface with:

- `CreateTenant` (`grpc_server.py:~2300`) — registers the tenant; archive is
  the inverse of "active".
- `GetTenant` (`grpc_server.py:2360-2395`) — read path; consumers poll
  status here to observe archive completion.
- `SetLegalHold` (`grpc_server.py:~2820-2845`) — also flips
  `tenant_registry.status` via `set_legal_hold` → reuses
  `_sync_set_tenant_status`. The two states (`archived`, `legal_hold`) are
  mutually exclusive in the schema (single `status` column) — see Open
  questions.
- `AddTenantMember` / `RemoveTenantMember` — share `_get_member_role`.
- All write RPCs (`PutNode`, `PutEdge`, `Commit`, `DeleteNode`, …) gated by
  `_check_tenant_access` (`grpc_server.py:518-522`) which is the consumer of
  the `archived` flag.

## Contract tests pinning behaviour (file:line)

- `tests/python/integration/test_grpc_contract.py:683-688` — happy path:
  admin actor, seed tenant id → `success=True`.
- `tests/python/integration/test_grpc_contract.py:690-693` — empty actor →
  `INVALID_ARGUMENT`.
- `tests/python/unit/test_tenant_registry.py:347-362` — owner can archive;
  post-call `tenant.status == "archived"` verified via `get_tenant`.
- `tests/python/unit/test_tenant_registry.py:364-377` — non-owner member
  receives `PERMISSION_DENIED`.
- `tests/python/unit/test_tenant_registry.py:379-391` — `system:admin`
  actor succeeds without membership.
- `tests/python/unit/test_tenant_roles.py:324-339` — archived tenant still
  serves reads.
- `tests/python/unit/test_tenant_roles.py:342-360` — archived tenant rejects
  writes with `FAILED_PRECONDITION` (downstream invariant).

The Go port suite must add a contract test that reproduces each of the
seven cases above through the Go gRPC server using the same proto wire.

## Implementation outline (sync vs async archive; how reversal works)

**Synchronous portion (in handler, must complete before response):**
1. Validate `actor` + `tenant_id` non-empty.
2. Resolve `trustedActor`; reject non-admin non-owner.
3. Update `tenant_registry.status = 'archived'` via `globalstore.SetTenantStatus`.
4. Return `success=true` (or `success=false, error="Tenant not found"` if
   the row was missing).

The Python implementation is fully synchronous — the entire archive is just
a SQLite UPDATE. There is no background workflow today.

**Asynchronous portion (proposed for Go port, not in Python today):**
- Emit a `tenant.archived` event onto a control-plane WAL stream.
- A worker reacts: snapshots the tenant's SQLite shard, uploads to S3 with
  the configured `s3_storage_class` (e.g. `STANDARD_IA`), and writes a
  `tenant_archive_manifest` row pointing at the snapshot.
- The worker is idempotent (event includes a UUID; manifest insert is
  `INSERT OR IGNORE`).

**Reversal.** There is no `UnarchiveTenant` RPC today. Reversal in the
Python implementation is "manually flip status back to `active` via direct
DB access" — there is no proto surface for it. For the Go port, two options:

- **(A) Idempotent state machine.** Add `RestoreTenant` RPC; allowed only
  while the cold-storage workflow has not yet transitioned the snapshot to
  `GLACIER` (or any non-immediate-retrieval class). Owner/admin only.
- **(B) Hard cutoff.** Once the async workflow completes, archive is
  one-way. Restore is a support-team operation outside the API surface.

Option (A) is required if console UX wants an "undo" affordance; (B) is
simpler and matches typical "soft-delete window" semantics. Decision is
deferred — see Open questions.

## Open questions / risks (legal-hold interaction, GDPR collision)

1. **Legal-hold collision.** `tenant_registry.status` is a single column;
   `archived` and `legal_hold` cannot coexist. If `ArchiveTenant` is
   invoked on a tenant currently in `legal_hold`, the Python handler
   silently overwrites the hold status — a compliance bug. Go port should
   reject with `FAILED_PRECONDITION` when current status is `legal_hold`,
   or store legal-hold separately from lifecycle status. Pin behaviour with
   a new contract test before changing.
2. **No WAL append (CLAUDE.md invariant #1).** `set_tenant_status` writes
   directly to global-store SQLite. Since global_store is cross-tenant, it
   does not flow through per-tenant WALs — but the invariant still implies
   *some* event log for replay/audit. Decision needed: introduce a control-
   plane WAL topic, or document global_store as an explicit exception.
3. **GDPR collision.** A tenant under a pending `DeleteUser` grace period
   may have outstanding GDPR obligations (right-to-erasure timer). Archiving
   the tenant must not pause that timer; the Go port should verify the
   delete worker still scans archived tenants. Currently undefined.
4. **No member / API-key revocation on archive.** Members of an archived
   tenant retain their grants; only writes are blocked. If product intent
   is "archive ⇒ frozen and inaccessible", the Go port must additionally
   revoke active sessions and API keys for the tenant.
5. **Error leak via `str(e)`.** The catch-all path leaks Python exception
   strings to the wire (`grpc_server.py:2440`). Go port should map to a
   generic `"internal error"` and rely on server-side logs + trace IDs.
6. **No idempotency.** Re-archiving an already-archived tenant returns
   `success=true` (UPDATE matches a row). That is fine, but pin it
   explicitly in the Go contract suite.
7. **Shard reassignment.** Archived tenants still occupy a shard slot.
   Open: should archive trigger eviction from the shard map?
