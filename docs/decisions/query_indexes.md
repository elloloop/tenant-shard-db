# Query indexes

## 2026-04-19: Declarative non-unique expression indexes via (entdb.field).indexed

**Status:** frozen
**Decided:** 2026-04-19
**Tags:** indexing, performance, schema, proto

### Decision

Fields annotated with `(entdb.field).indexed = true` in proto get a non-unique expression index on the `nodes` table, created lazily on first write for the type. This accelerates `query_nodes` filters on high-cardinality or frequently-queried fields without requiring any changes to the query path.

#### Proto declaration

```proto
message Product {
  option (entdb.node) = { type_id: 201 };
  string sku         = 1 [(entdb.field).unique = true];
  string name        = 3;
  int32  price_cents = 4 [(entdb.field).indexed = true];
  string status      = 5 [(entdb.field).indexed = true];
}
```

#### Index DDL

For each `(type_id, field_id)` pair where `indexed = true` and `unique` is not also set:

```sql
CREATE INDEX IF NOT EXISTS idx_query_t201_f4
  ON nodes(tenant_id, json_extract(payload_json, '$."4"'))
  WHERE type_id = 201;
```

This mirrors the unique-index pattern from the 2026-04-14 SDK v0.3 decision, with two differences: (1) the index is non-unique (`CREATE INDEX`, not `CREATE UNIQUE INDEX`), and (2) the name prefix is `idx_query_` instead of `idx_unique_`.

#### `unique` implies `indexed`

A unique expression index IS an index. Fields with `unique = true` already have a unique expression index that SQLite's planner uses for equality and range lookups. Adding `indexed = true` on a field that already has `unique = true` is harmless but redundant -- the schema registry's `get_indexed_field_ids()` excludes fields that are already `unique`, so no duplicate non-unique index is created.

#### Lazy creation mechanism

Indexes are created lazily on the first `CreateNode` or `UpdateNode` operation for a type whose schema declares indexed fields. The applier calls `_ensure_field_indexes()` (which dispatches to both `_ensure_unique_indexes` and `_ensure_query_indexes`) before applying the write operation. Each method maintains a per-process `set[tuple[str, int]]` cache keyed by `(physical_db_path, type_id)` so the `CREATE INDEX IF NOT EXISTS` DDL runs at most once per type per process lifetime.

#### Query planner -- no changes needed

The existing `query_filter.py` compiles MongoDB-style operators to SQL WHERE fragments of the form `json_extract(payload_json, '$."<field_id>"') <op> ?`. This expression matches the indexed expression exactly, so SQLite's query planner automatically uses the index when it estimates a benefit. No query-path changes, no `ANALYZE`, and no query hints are required.

### Storage and write-amplification cost

Each expression index costs approximately 30-50 bytes per row (a B-tree entry for `(tenant_id, extracted_value, rowid)`). Every `INSERT` and `UPDATE` on an indexed field updates the index as well. This is the standard SQLite write-amplification tradeoff and is explicitly opt-in via the proto annotation -- there is no automatic indexing.

### Context

The 2026-04-14 SDK v0.3 ADR explicitly deferred query-time indexes:

> Indexes for query-time filtering are **explicitly out of scope** for this ADR. Without an index, every filtered query is a tenant+type scan -- no worse than today, but no better either. Adding declarative `(entdb.field).indexed = true` and lazy non-unique expression-index creation is a separate decision for later if the scan cost actually bites.

The scan cost has bitten on workloads with >10k nodes per type filtered on a single status or price field. This decision closes that gap with the simplest possible mechanism: same lazy-index pattern, same cache, same idempotent DDL, just non-unique instead of unique.

### Consequences

#### What this locks in

- `FieldOpts.indexed` (field number 3 in `entdb_options.proto`) is the stable API for declaring non-unique indexes.
- `SchemaRegistry.get_indexed_field_ids(type_id)` returns field ids that are `indexed = true` but NOT `unique = true`.
- `CanonicalStore._ensure_query_indexes()` creates `idx_query_t<type>_f<field>` indexes.
- `CanonicalStore._ensure_field_indexes()` is the unified entry point for both unique and query index creation.
- The applier calls `_ensure_field_indexes()` before `CreateNode` and `UpdateNode` operations.

#### What this makes easy

- Speeding up filtered queries on hot fields by adding one proto annotation.
- No server restart needed -- indexes are created lazily on first write.
- No query-path changes needed -- SQLite handles it.

#### What this makes harder

- Dropping an indexed field requires manual `DROP INDEX` (same as unique indexes).
- Each additional index increases write latency marginally (~5-15us per indexed field per write on NVMe).

### References

- Supersedes the "explicitly out of scope" note in [sdk_api.md 2026-04-14](sdk_api.md#2026-04-14-single-shape-sdk-api-typed-unique-key-tokens-via-codegen-expression-index-unique-enforcement)
- Follows the lazy-index pattern established in that same ADR for unique indexes
