# ADR-023: Declarative non-unique expression indexes via `(entdb.field).indexed`

**Status:** Accepted
**Decided:** 2026-04-19
**Tags:** indexing, performance, schema, proto
**Implementation:** `server/go/internal/store/` (lazy query-index creation alongside unique indexes)

## Decision

Fields annotated with `(entdb.field).indexed = true` in proto get a
non-unique SQLite expression index on the `nodes` table, created
lazily on first write for the type. This accelerates `QueryNodes`
filters on high-cardinality or frequently-queried fields without
requiring any changes to the query path.

### Proto declaration

```proto
message Product {
  option (entdb.node) = { type_id: 201 };
  string sku         = 1 [(entdb.field).unique = true];
  string name        = 3;
  int32  price_cents = 4 [(entdb.field).indexed = true];
  string status      = 5 [(entdb.field).indexed = true];
}
```

### Index DDL

For each `(type_id, field_id)` pair where `indexed = true` and
`unique` is not also set:

```sql
CREATE INDEX IF NOT EXISTS idx_query_t201_f4
  ON nodes(tenant_id, json_extract(payload_json, '$."4"'))
  WHERE type_id = 201;
```

Same shape as the unique-index pattern in
[ADR-025](025-single-shape-sdk-api.md), with two differences:

1. The index is non-unique (`CREATE INDEX`, not
   `CREATE UNIQUE INDEX`).
2. The name prefix is `idx_query_` instead of `idx_unique_`.

### `unique` implies `indexed`

A unique expression index IS an index. Fields with `unique = true`
already have a unique expression index that SQLite's planner uses
for equality and range lookups. Adding `indexed = true` on a field
that already has `unique = true` is harmless but redundant — the
schema registry's `GetIndexedFieldIDs()` excludes fields that are
already `unique`, so no duplicate non-unique index is created.

### Lazy creation mechanism

Indexes are created lazily on the first `CreateNode` or
`UpdateNode` operation for a type whose schema declares indexed
fields. The applier ensures both unique and query indexes before
applying the write operation. A per-process cache keyed by
`(physical_db_path, type_id)` runs the `CREATE INDEX IF NOT EXISTS`
DDL at most once per type per process lifetime.

### Query planner — no changes needed

`QueryNodes` compiles MongoDB-style operators to SQL WHERE
fragments of the form
`json_extract(payload_json, '$."<field_id>"') <op> ?`. This
expression matches the indexed expression exactly, so SQLite's
query planner automatically uses the index when it estimates a
benefit. No query-path changes, no `ANALYZE`, and no query hints
are required.

### Storage and write-amplification cost

Each expression index costs approximately 30-50 bytes per row (a
B-tree entry for `(tenant_id, extracted_value, rowid)`). Every
`INSERT` and `UPDATE` on an indexed field updates the index as
well. This is the standard SQLite write-amplification tradeoff and
is explicitly opt-in via the proto annotation — there is no
automatic indexing.

## Context

[ADR-025](025-single-shape-sdk-api.md) (the v0.3 SDK ADR) deferred
query-time indexes:

> Indexes for query-time filtering are explicitly out of scope for
> this ADR. Without an index, every filtered query is a tenant+type
> scan — no worse than today, but no better either. Adding
> declarative `(entdb.field).indexed = true` and lazy non-unique
> expression-index creation is a separate decision for later if the
> scan cost actually bites.

The scan cost has bitten on workloads with >10k nodes per type
filtered on a single status or price field. This ADR closes that
gap with the simplest possible mechanism: same lazy-index pattern,
same cache, same idempotent DDL, just non-unique instead of unique.

## Alternatives considered

- **Mandatory index hints in the query API.** Rejected. Forces the
  application to think about physical layout at every call site.
  The proto annotation is the right place — schema-author owns
  index decisions, query author writes natural filters.
- **Automatic indexing of every payload field.** Rejected. Write
  amplification scales with field count; an unused index costs
  storage and slows writes for no benefit. Opt-in keeps the cost
  obvious in the proto.
- **Build a separate inverted-index table for query filters.**
  Rejected. Same fan-out cost as automatic indexing, plus a second
  consistency story between `nodes` and the index table. SQLite
  expression indexes are the simpler tool.
- **Composite indexes (multi-field).** Deferred. Phase 1 supports
  single-field expression indexes. Composite filters that walk
  multiple `(entdb.field).indexed` fields still benefit because
  SQLite will pick the most selective single-column index. Native
  composite-index support is a future extension if hot multi-field
  queries appear.
- **Drop the lazy pattern; create all indexes eagerly at boot.**
  Rejected. Lazy creation keeps cold-start fast and doesn't pay for
  types that aren't actively written. The idempotent DDL is cheap
  on subsequent calls.

## Consequences

**What this locks in:**

- `FieldOpts.indexed` (field number 3 in `entdb_options.proto`) is
  the stable schema annotation for declaring non-unique indexes.
- The schema-registry method `GetIndexedFieldIDs(type_id)` returns
  field ids that are `indexed = true` but NOT `unique = true`.
- The applier creates `idx_query_t<type>_f<field>` indexes lazily
  on the first `CreateNode` or `UpdateNode` for a type.
- A unified "ensure field indexes" entry point in the applier
  covers both unique and query index creation.

**What this makes easy:**

- Speeding up filtered queries on hot fields by adding one proto
  annotation.
- No server restart needed — indexes are created lazily on first
  write.
- No query-path changes needed — SQLite handles it.

**What this makes harder:**

- Dropping an indexed field requires manual `DROP INDEX` (same as
  unique indexes — see
  [ADR-025](025-single-shape-sdk-api.md#consequences)).
- Each additional index increases write latency marginally (~5-15
  µs per indexed field per write on NVMe).

**Failure modes:**

- The lazy DDL runs on every applier process startup. A process
  that processes only updates (no creates) on a type whose schema
  changed to add `indexed: true` won't trigger index creation
  until the next create. Mitigation: the ensure-index call hooks
  both `CreateNode` and `UpdateNode`.
- A query reads from `nodes` between when the schema is registered
  and when the lazy index is first created. The query works
  (returns correct results) but does a full scan. Acceptable —
  not a correctness issue, just a perf transient on the very first
  query for that type+field pair.

## References

- [ADR-025](025-single-shape-sdk-api.md) — the v0.3 SDK ADR that
  deferred query indexes; this ADR closes that deferral.
- [ADR-022](022-fts5-full-text-search.md) — same lazy-index pattern
  applied to FTS5 virtual tables.
- [ADR-018](018-field-id-keyed-payloads.md) — field-ID-keyed
  payloads. Indexed expressions use `'$."<field_id>"'`, never the
  proto field name.
- Files this commit removes content from:
  - `docs/decisions/query_indexes.md` — the query-index design is
    captured here; the file is deleted in the same commit.
