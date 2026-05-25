# ADR-031: Self-describing, name-free schema

**Status:** Proposed
**Decided:** 2026-05-25
**Tags:** schema, proto, wire-contract, event-sourcing, sdk, integrity

**Implementation:** empty-boot registry (`server/go/cmd/entdb-server/`);
`register_schema` WAL op + applier materialization
(`server/go/internal/apply/`); runtime-mutable registry + name-free type
system + fingerprint (`server/go/internal/schema/`); `SchemaDescriptor`
on `ExecuteAtomicRequest` (`proto/entdb/v1/entdb.proto`); SDK
attach-on-write (`sdk/python/entdb_sdk/`, `sdk/go/entdb/`).

## Decision

The server learns an application's schema from the **writes themselves**,
and the schema is expressed purely in **`field_id` coordinates** — never
names. Concretely:

1. **Empty boot.** The server imports nothing at startup — no schema, no
   tenant/user data. It always boots with an empty, runtime-mutable
   schema registry. (Snapshot save/load is explicitly out of scope here;
   see "Deferred".)
2. **Self-describing writes.** A client attaches the definitions of the
   types a batch touches to `ExecuteAtomicRequest`. When the client's
   `schema_fingerprint` differs from the server's, the handler prepends a
   `register_schema` op to the WAL event, **before** the data ops, in the
   same event/transaction.
3. **The applier materializes schema.** Per [ADR-016](016-handlers-append-applier-writes.md)
   only the applier writes SQLite. It applies `register_schema`
   establish-or-reject, registering the type in the (process-wide,
   mutable) registry and creating the per-tenant expression indexes
   ([ADR-023](023-declarative-query-indexes.md)/[ADR-030](030-composite-unique-constraints.md))
   before the data ops it guards.
4. **Name-free everywhere.** At rest (SQLite) and on the wire, schema and
   data are keyed by `field_id` / `type_id` only
   ([ADR-018](018-field-id-keyed-payloads.md)). The server never stores,
   transmits, or translates a field name, type name, or constraint name.
   Names live only in the client's proto.
5. **Deterministic replay.** Because `register_schema` is in the WAL, a
   server rebuilds its registry + indexes by replaying the log — the
   property a boot-loaded snapshot cannot provide.

### Why name-free (ids at rest and on the wire)

[ADR-018](018-field-id-keyed-payloads.md) already makes the proto field
*number* the storage key: node rows store `payload_json` keyed by
`field_id`, and the expression indexes read `json_extract(payload_json,
'$."<field_id>"')`. Everything the server does — index DDL, uniqueness,
composite tuples, type/required validation, filters, CAS preconditions —
needs only the `field_id`. The name is a client ergonomic, resolved by
the SDK's `register_proto_schema` map. Carrying names server-side bought
nothing and created two failure modes: a name-keyed payload that the
server had to translate (and rejected when it had no registry), and a
fingerprint that depended on names two clients had to agree on.

This ADR makes name-free a hard invariant:

- `schema.FieldDef`/`NodeTypeDef`/`EdgeTypeDef` carry **no `name`**. Types
  are identified by `type_id`/`edge_id`; fields by `field_id`.
- The cross-language schema JSON contract emits ids + attributes only.
- The **fingerprint** is computed over the name-free canonical form, so
  client and server derive the same hash without ever sharing names.
- Composite-unique constraints are identified by their **`field_id`
  tuple**; the index name suffix is derived from the tuple (not a
  constraint name), amending [ADR-030](030-composite-unique-constraints.md)'s
  `idx_unique_t<type>_c<name>` to a tuple-derived suffix.
- The legacy **name-keyed payload / filter / precondition** server paths
  are removed. Clients send `field_id`-keyed payloads, `field_id` filters,
  and `field_id` CAS preconditions (the proto already calls `field_id`
  "the preferred and schema-less-safe coordinate"). `GetSchema` returns
  ids + attributes; humans see names by resolving against the proto
  client-side (SDK / console), never via the server.

### Conflict policy: establish-or-reject

A `register_schema` for a `type_id`:

- **absent** → register it;
- **present and byte-identical** (canonical-JSON identity, the same notion
  the fingerprint uses) → no-op;
- **present and different** → reject as a deterministic failure
  (`FAILED_PRECONDITION` / surfaced as `ALREADY_EXISTS` for the typed SDK
  error), memoized and non-halting exactly like the CAS-miss and
  unique-violation paths ([ADR-016](016-handlers-append-applier-writes.md)
  issue #500, [ADR-030](030-composite-unique-constraints.md)).

Online schema **evolution** (compatible `ALTER`) is out of scope; v1 is
establish-or-reject. Adding a constraint over existing duplicate data
fails the `CREATE UNIQUE INDEX`, same as Postgres — the operator
de-duplicates first (the `entdb-schema` compat checker gates breaking
changes).

### Fingerprint negotiation (lean steady state)

The schema op is idempotent at the applier, but the client should not pay
the bytes on every write. The client attaches the `SchemaDescriptor` only
when its fingerprint is unconfirmed for this server (first write, or after
a `SCHEMA_MISMATCH`); once the server's returned fingerprint matches, it
omits the descriptor. This reuses the existing `schema_fingerprint` round
trip and is the write-path analogue of the SDK auto-follow in
[ADR-029](029-keyset-cursor-pagination.md).

### Registry is process-wide and runtime-mutable

All tenants share one schema; a tenant is a data-partition boundary, not a
schema boundary. The registry is therefore process-wide. It is no longer
frozen-at-boot: a copy-on-write snapshot behind an atomic pointer keeps
reads lock-free while `register_schema` publishes new snapshots. Per-tenant
**indexes** are still created per tenant in each tenant's SQLite on first
touch (the existing `EnsureFieldIndexesTx` path).

## Context

The server enforced uniqueness/validation/indexes only when its in-memory
registry was populated, and the **only** thing that ever populated it was
a test-only boot seed (`--seed-profile` → `testseed`). There was no
production path — no flag, file, env, or RPC — to load an application's
schema. So on a real server `(entdb.field).unique = true` was inert: the
applier's index-ensure loop read an empty registry and created nothing,
and duplicates were silently accepted. The boot seed also masked this in
tests, which is why CI was green while production enforced nothing. This
ADR removes the crutch and makes the production path the *only* path,
exercised identically by tests.

## Alternatives considered

- **Load a schema snapshot at boot (file/flag).** Rejected as the
  end-state: it is not in the WAL, so replay/new-replica does not
  reproduce it deterministically; it forces a restart per schema change;
  and a global boot file fits neither the event-sourced model nor
  per-deploy schema drift. (Snapshot *as an optimization* over the WAL is
  deferred, not this.)
- **A dedicated `RegisterSchema` RPC.** Same WAL benefit but adds a
  lifecycle step a client can forget; piggybacking on the write cannot be
  forgotten. (The historically-absent `RegisterSchema` is reintroduced
  *as a WAL op*, not a side channel.)
- **Carry the constraint in every data op.** A type-level invariant
  restated per write; a client could send conflicting definitions.
- **Keep names server-side (optional/diagnostic).** Rejected — vestigial
  dead weight that re-introduces the name/id dual coordinate and a
  name-dependent fingerprint. Names are client-only.
- **Per-tenant registries.** Unnecessary: all tenants share one schema.

## Consequences

**Locks in:**
- Empty boot; no `--seed-profile`/`--seed-tenant`; tests bootstrap tenant
  + users + schema through the API, sequentially, like a real client.
- `register_schema` WAL op + the `SchemaDescriptor` on
  `ExecuteAtomicRequest` as the schema-establishment mechanism.
- Name-free type system, JSON contract, fingerprint, and index naming.
- Establish-or-reject; deterministic, memoized, non-halting conflicts.

**Makes easy:**
- An app's `unique`/`composite_unique`/`indexed` actually enforced in
  production, with no provisioning step, surviving restart via replay.
- One code path for tests and production (kills the test-vs-prod divergence).

**Makes harder / breaking:**
- The wire schema representation and the fingerprint change (name-free) —
  both SDKs must ship together ([CLAUDE.md]). Old name-keyed payload /
  filter / precondition paths are removed.
- `GetSchema` no longer returns names; name-facing UX resolves client-side.
- [ADR-030](030-composite-unique-constraints.md) composite index name
  suffix changes from constraint-name to field-id-tuple derived.

**Deferred (not today):**
- Schema **snapshot** save/load as a replay optimization (the WAL stays
  the source of truth).
- Online schema **evolution** (compatible `ALTER`).

## References

- [ADR-006](006-proto-schema-definition.md) — proto is the type system;
  this clarifies it is the *client's* type system, projected to the server
  as id-only coordinates.
- [ADR-016](016-handlers-append-applier-writes.md) — handlers append,
  applier writes; `register_schema` follows this and reuses the
  deterministic-failure memoization (issue #500).
- [ADR-018](018-field-id-keyed-payloads.md) — field-id-keyed payloads;
  extended here to the schema itself.
- [ADR-023](023-declarative-query-indexes.md) / [ADR-030](030-composite-unique-constraints.md)
  — lazy expression indexes + composite unique; index naming amended.
- [ADR-025](025-single-shape-sdk-api.md) — single-shape SDK; fingerprint
  and typed unique errors.
- [ADR-029](029-keyset-cursor-pagination.md) — SDK auto-follow, the
  pattern the fingerprint-omit negotiation mirrors.
