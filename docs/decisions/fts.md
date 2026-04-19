# Full-text search

## 2026-04-19: FTS5-backed full-text search via (entdb.field).searchable

**Status:** frozen
**Decided:** 2026-04-19
**Tags:** search, indexing, performance, schema, proto

### Context

EntDB needs full-text search across node payloads. Users want to search
free-text fields (product names, descriptions, messages) with stemming,
phrase matching, and relevance ranking -- capabilities that JSON
`json_extract` + `LIKE` cannot provide efficiently.

### Decision

Use SQLite FTS5 virtual tables, one per node type that has at least one
field with `(entdb.field).searchable = true`.

#### Schema option

The existing `FieldOpts.searchable` (field number 2 in
`entdb_options.proto`) opts a field into FTS. Only string (TEXT) fields
are eligible; non-string fields with `searchable: true` are rejected
with a warning at schema registration time.

```proto
message Product {
  option (entdb.node) = { type_id: 201 };
  string name = 3 [(entdb.field) = { searchable: true, indexed: true }];
  string desc = 6 [(entdb.field).searchable = true];
  int32  price = 4;
}
```

#### FTS5 virtual table design

```sql
CREATE VIRTUAL TABLE IF NOT EXISTS fts_t{type_id} USING fts5(
  node_id UNINDEXED,
  f3,
  f6,
  tokenize='porter unicode61'
);
```

Key properties:

- **`node_id UNINDEXED`** -- the node_id column is stored in the FTS
  table but excluded from tokenization. On match, node_id is used to
  JOIN against the `nodes` table to fetch the full row.

- **`tokenize='porter unicode61'`** -- Porter stemming + Unicode-aware
  tokenization. "running" matches "run"; accented characters are
  handled correctly.

- **Column names are `f{field_id}`** -- matching the field-ID-not-name
  convention. `node_id` is stored UNINDEXED so it can be retrieved from
  match results without being tokenized.

- **One table per type** -- each type has different searchable fields,
  so they need different column sets. Keeps per-type FTS indexes small.

#### Lazy creation

FTS tables are created lazily on first write or search for a type, same
pattern as unique expression indexes. The `CREATE VIRTUAL TABLE IF NOT
EXISTS` clause makes this idempotent across restarts. A process-local
cache (`_fts_table_cache`) avoids repeated DDL.

#### Write path

FTS mutations happen inside the applier's `_apply_event_with_conn`
transaction, alongside the regular `nodes` table writes:

- **Create**: `INSERT INTO fts_t{type_id}(node_id, f3, f6) VALUES (?, ?, ?)`
- **Update**: if any searchable field changed, delete-then-insert
- **Delete**: `DELETE FROM fts_t{type_id} WHERE node_id = ?`

This respects the "all writes go through the WAL" invariant -- FTS
mutations are a side effect of applying WAL events, not direct writes
from handlers.

#### Query path

A new `SearchNodes` RPC performs the search:

```sql
SELECT n.* FROM fts_t{type_id} AS fts
JOIN nodes AS n ON n.node_id = fts.node_id AND n.tenant_id = ?
WHERE fts_t{type_id} MATCH ?
ORDER BY fts.rank
LIMIT ? OFFSET ?
```

Results are ACL-filtered server-side (same as QueryNodes).

#### FTS5 query syntax

The `query` field accepts standard FTS5 match expressions:

- Simple terms: `widget` (matches stemmed forms)
- Phrases: `"blue widget"` (exact phrase)
- Boolean: `widget AND NOT gadget`, `widget OR gizmo`
- Prefix: `wid*` (prefix match)
- Column filters: `f3:widget` (match in specific field)

#### Storage cost

FTS5 stores the inverted index plus a copy of the searchable text
content. For typical text fields (~100 words per node), the FTS index
adds roughly 50-100 bytes per word per node. A type with 100K nodes
and ~200 words of searchable text per node uses ~8-16 MB of FTS index.
This is acceptable given the per-tenant SQLite isolation model.

### Not included

- Relevance scoring / highlighting / snippets in the response
- FTS on non-string fields
- Changes to the existing `query_nodes` / `query_filter.py` path

### SDK surface

**Python:** `scope.search(Product, "widget", limit=50, offset=0)`
**Go:** `entdb.Search[*shop.Product](ctx, scope, "widget")`

Both call the `SearchNodes` RPC.
