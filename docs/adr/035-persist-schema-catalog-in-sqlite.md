# ADR-035: Persist the schema catalog in SQLite (registry as a cache, not a WAL rebuild)

**Status:** Accepted
**Decided:** 2026-05-30
**Tags:** schema, storage, recovery, registry, durability

**Implementation:** per-tenant `schema_catalog` table
(`server/go/internal/store/schema.go`); `UpsertCatalogTx` +
`LoadCatalogInto` + `MarshalCatalogDef`
(`server/go/internal/store/catalog.go`); atomic write in the apply txn
(`server/go/internal/apply/ops_register_schema.go`); load-once-per-tenant
on `OpenTenant`/`dbAuto` (`server/go/internal/store/canonical.go`);
GetSchema reduced to a pure registry serializer
(`server/go/internal/api/get_schema.go`). Fixes #624; closes #626.

**Decided refinements vs the original proposal:**
- **Catalog home = per-tenant SQLite** (resolves the open question
  below). `register_schema` is always tenant-scoped and runs inside the
  per-tenant `BatchTxn`, so a per-tenant `schema_catalog` row is atomic
  with the indexes it describes and the `applied_events`/`applied_offsets`
  rows — atomicity a globalstore catalog could not give. The process-wide
  registry is keyed by `type_id`, so it is reconstructed identically by
  replaying each tenant's catalog.
- **Load strategy = lazy, load-once-per-tenant on open.** The catalog is
  replayed into the registry the first time a tenant is opened (apply path
  via `OpenTenant`, read/write paths via `dbAuto`), so the registry warms
  as the server processes traffic post-restart — fixing the #624 read
  failures with **zero boot-time change**. The eager all-tenant boot-scan
  the proposal floated (to warm GetSchema before any traffic) is
  **deliberately deferred**: opening every tenant DB at boot risks
  startup latency + file-descriptor pressure at scale. It is a future
  opt-in flag; the lazy load already makes GetSchema correct as soon as
  any tenant is touched.

**Supersedes/refines:** the boot mechanics of
[ADR-031](031-self-describing-name-free-schema.md) (self-describing,
name-free schema). ADR-031's wire/identity model is unchanged; only
*where the registry lives across a restart* changes.

## Context

EntDB keeps schema in two layers today:

- **Physical schema — durable in SQLite.** The actual per-tenant tables
  and indexes (unique, composite-unique tuple, FTS5, expression/query
  indexes) are created on disk during `register_schema` apply
  (`EnsureFieldIndexesTx`) and survive restart. Stored data is
  `field_id`-keyed (ADR-018), so reads need no name catalog.
- **Logical registry — in-memory only.** `schema.NewRegistry()`
  (`cmd/entdb-server/main.go`) holds the type/field definitions and the
  canonical fingerprint. There is **no SQLite catalog table** for it. It
  boots empty and is populated by replaying `register_schema` WAL ops;
  every write is self-describing (ADR-031), so it self-heals on the next
  write per type.

The in-memory registry backs: the schema fingerprint advertised on
writes, the read-only `GetSchema` RPC, establish-or-reject validation on
writes, and query field translation.

**The gap.** "Rebuilt deterministically from the WAL on every boot" is
only literally true for a *full replay* (the in-memory backend, or a
fresh Kafka consumer group reading from `earliest`). On a **normal Kafka
restart** the applier resumes from the *committed* offset, so already
applied `register_schema` ops are **not** re-delivered — the registry
boots empty and stays empty until the next write re-registers a type. In
that window `GetSchema` returns an empty/degraded response (its SQLite
distinct-type_id fallback is a documented no-op in `api/get_schema.go`).
Data reads are unaffected (`field_id`-keyed; `query_nodes.go` guards its
type check on `!registry.Empty()`), but the schema surface is
restart-fragile and depends on broker retention for correctness rather
than on durable local state — unlike a standard DB's catalog.

## Decision (proposed)

Make **SQLite the durable source of truth for the schema catalog**, and
demote the in-memory registry to a cache loaded from it at boot.

- Persist node/edge type definitions (the registry's canonical,
  `field_id`-keyed form) to a catalog table, written **in the same
  applier transaction** as the `register_schema` op that establishes
  them — so the catalog is atomic with the indexes it describes and
  survives any restart.
  - Tenant-scoped types → per-tenant SQLite; control-plane/global types →
    globalstore. (Open question below.)
- At boot, **load the catalog into the registry** (analogous to how the
  physical indexes are already durable), instead of relying on WAL
  replay to repopulate it.
- `register_schema` apply becomes: upsert catalog row (establish-or-
  reject) + ensure indexes, both in one txn. Replay stays idempotent and
  deterministic (ADR-031) — re-applying an identical type is a no-op
  against the catalog too.
- `GetSchema` reads the registry (now always populated post-restart);
  the dead SQLite-distinct-type_id fallback is removed.

The registry stays in memory for fast, lock-free reads — it is now a
**cache of a durable catalog**, not a structure rebuilt from the WAL.
This keeps ADR-031's self-describing, name-free model and determinism
while removing the restart-fragility and the broker-retention dependence
for schema.

## Alternatives considered

- **Keep WAL-replay rebuild, fix only `GetSchema`** (e.g. implement the
  distinct-type_id fallback). Rejected: it patches the symptom
  (`GetSchema`) without making the catalog durable; establish-or-reject
  validation and the fingerprint are still empty-until-first-write after
  a restart.
- **Always replay the schema prefix from `earliest` on boot.** Rejected:
  re-reading history on every restart is slow, depends on broker
  retention, and conflates schema bootstrap with data replay.
- **Snapshot the registry to a file (.schema-snapshot.json) at boot.**
  Rejected: a side file is a second source of truth that can drift from
  SQLite; the catalog belongs in the same transactional store as the
  indexes it describes.

## Known asymmetry (accepted)

`applyRegisterSchema` mutates the **process-global in-memory registry**
(`RegisterOrVerifyNode/Edge` publish a new snapshot) *before* the
`BatchTxn` commits. If a single multi-type `register_schema` op
establishes an earlier type and then a later type **conflicts**, the
batch rolls back — reverting the earlier type's catalog row and indexes —
but the in-memory registry keeps the earlier type. So the live process
registry can briefly hold a type whose durable state (catalog + indexes)
was rolled back.

This non-transactional registry mutation is **pre-existing** (it predates
this ADR). Promoting the catalog to the durable source of truth makes the
**durable** side the more-correct one: on the next restart the registry is
reloaded from the committed catalog and the phantom type is dropped, and
every write is self-describing so the next write of that type
re-establishes it consistently. The asymmetry is therefore self-healing
and is accepted rather than fixed here; making the in-memory mutation
all-or-nothing (validate every type before any publish) is a separate,
optional hardening. The catalog and the physical indexes never diverge —
only registry-vs-durable, transiently.

## Open questions

- Catalog home for tenant vs global/control-plane types (per-tenant
  SQLite vs globalstore vs both).
- Migration: existing deployments boot empty and self-heal on first
  write today; a one-time backfill (replay-from-earliest into the
  catalog, or lazy population) may be needed so `GetSchema` is correct
  immediately after upgrade without waiting for a write.
- Interaction with the deferred restore-from-archive path
  ([ADR-034](034-restore-from-object-store-archive.md)): archive replay
  rebuilds the catalog for free via the same `register_schema` apply, so
  the two are complementary.
