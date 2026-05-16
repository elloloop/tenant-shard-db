# ADR-022: FTS5-backed full-text search via `(entdb.field).searchable`

**Status:** Accepted
**Decided:** 2026-04-19
**Tags:** search, indexing, performance, schema, proto
**Implementation:** Go server FTS5 path under `server/go/internal/store/`; SDK `Search` API in both Python and Go SDKs

## Decision

EntDB uses SQLite FTS5 virtual tables for full-text search. A field
opts into FTS by setting `(entdb.field).searchable = true` in proto.
The applier creates one FTS5 virtual table per node type that has at
least one searchable field, lazily on first write or search. FTS
mutations happen inside the applier's apply transaction alongside
the regular `nodes` writes — they are a side effect of applying WAL
events, not direct writes from handlers, so the WAL-source-of-truth
invariant ([ADR-016](016-handlers-append-applier-writes.md)) is
preserved.

A new `SearchNodes` RPC performs the search; results are ACL-
filtered server-side (same as `QueryNodes`).

### Schema option

The `FieldOpts.searchable` flag (field number 2 in
`entdb_options.proto`) opts a field into FTS. Only string (TEXT)
fields are eligible; non-string fields with `searchable: true` are
rejected with a warning at schema registration time.

```proto
message Product {
  option (entdb.node) = { type_id: 201 };
  string name = 3 [(entdb.field) = { searchable: true, indexed: true }];
  string desc = 6 [(entdb.field).searchable = true];
  int32  price = 4;
}
```

### FTS5 virtual table design

```sql
CREATE VIRTUAL TABLE IF NOT EXISTS fts_t{type_id} USING fts5(
  node_id UNINDEXED,
  f3,
  f6,
  tokenize='porter unicode61'
);
```

Key properties:

- **`node_id UNINDEXED`** — the `node_id` column is stored in the
  FTS table but excluded from tokenization. On match, `node_id` is
  used to JOIN against the `nodes` table to fetch the full row.
- **`tokenize='porter unicode61'`** — Porter stemming + Unicode-
  aware tokenization. "running" matches "run"; accented characters
  are handled correctly.
- **Column names are `f{field_id}`** — matching the field-ID-not-
  name convention ([ADR-018](018-field-id-keyed-payloads.md)).
- **One table per type** — each type has different searchable
  fields, so they need different column sets. Keeps per-type FTS
  indexes small.

### Lazy creation

FTS tables are created lazily on first write or search for a type,
same pattern as unique expression indexes
([ADR-025](025-single-shape-sdk-api.md)) and query indexes
([ADR-023](023-declarative-query-indexes.md)). The
`CREATE VIRTUAL TABLE IF NOT EXISTS` clause makes this idempotent
across restarts. A process-local cache avoids repeated DDL.

### Write path

FTS mutations happen inside the applier's apply transaction,
alongside the regular `nodes` table writes:

- **Create:**
  `INSERT INTO fts_t{type_id}(node_id, f3, f6) VALUES (?, ?, ?)`
- **Update:** if any searchable field changed, delete-then-insert.
- **Delete:** `DELETE FROM fts_t{type_id} WHERE node_id = ?`

### Query path

```sql
SELECT n.* FROM fts_t{type_id} AS fts
JOIN nodes AS n ON n.node_id = fts.node_id AND n.tenant_id = ?
WHERE fts_t{type_id} MATCH ?
ORDER BY fts.rank
LIMIT ? OFFSET ?
```

Results are ACL-filtered server-side, identically to `QueryNodes`.

### FTS5 query syntax

The `query` field accepts standard FTS5 match expressions:

- Simple terms: `widget` (matches stemmed forms).
- Phrases: `"blue widget"` (exact phrase).
- Boolean: `widget AND NOT gadget`, `widget OR gizmo`.
- Prefix: `wid*` (prefix match).
- Column filters: `f3:widget` (match in specific field).

### Storage cost

FTS5 stores the inverted index plus a copy of the searchable text
content. For typical text fields (~100 words per node), the FTS
index adds roughly 50-100 bytes per word per node. A type with 100K
nodes and ~200 words of searchable text per node uses ~8-16 MB of
FTS index. Acceptable given the per-tenant SQLite isolation model.

### Not in scope

- Relevance scoring / highlighting / snippets in the response.
- FTS on non-string fields.
- Changes to the existing `QueryNodes` filter path.

### SDK surface

- **Python:** `scope.search(Product, "widget", limit=50, offset=0)`
- **Go:** `entdb.Search[*shop.Product](ctx, scope, "widget")`

Both call the `SearchNodes` RPC.

## Context

EntDB needs full-text search across node payloads. Users want to
search free-text fields (product names, descriptions, messages) with
stemming, phrase matching, and relevance ranking — capabilities
that `json_extract` + `LIKE` cannot provide efficiently.

SQLite ships FTS5 as a built-in module with stemming, prefix match,
phrase queries, and Okapi BM25 ranking. Reusing it avoids running a
second search service for the per-tenant database model.

## Alternatives considered

- **External search engine (Elasticsearch / OpenSearch / Meilisearch).**
  Rejected for the default deployment. Would multiply operational
  cost (a second stateful service, a second consistency story
  between WAL and search index), pull the audit-and-replay
  invariants of the WAL out of scope for search, and require
  per-tenant ACL filtering on a system EntDB doesn't control. The
  per-tenant SQLite model already isolates search indexes by
  tenant for free.
- **Postgres-style trigram (`pg_trgm`) on JSON-extracted text.**
  Rejected. SQLite does not ship trigram natively; implementing it
  in a custom module is more code than FTS5 with worse linguistic
  features (no stemming, no Unicode normalization, no phrase
  match).
- **Application-layer search via brute-force `LIKE` scan.** Rejected
  beyond toy scale: every search is O(rows) and pays
  `json_extract` per row.
- **FTS over the whole payload JSON.** Rejected — would tokenize
  field IDs (`"1"`, `"2"`) and other JSON noise, and return matches
  on irrelevant fields. The opt-in per-field model is more precise
  and lets the schema author choose what is meaningfully
  searchable.
- **Relevance scores / highlights / snippets in the first version.**
  Deferred. The RPC returns the same `Node` shape as `QueryNodes`,
  with FTS5's BM25 ordering applied server-side. Adding `rank` /
  `highlights` to the wire is a future extension if the use case
  shows up.

## Consequences

**What this locks in:**

- `FieldOpts.searchable` (field number 2 in `entdb_options.proto`)
  is the stable schema annotation for opting fields into FTS.
- Per-type FTS5 virtual tables named `fts_t{type_id}` with columns
  `node_id UNINDEXED, f{field_id}, …` and `tokenize='porter
  unicode61'`. Renaming the convention would require a data
  migration.
- FTS mutations are applied inside the applier transaction, never
  directly from handlers.
- `SearchNodes` RPC takes `(tenant_id, type_id, query, limit,
  offset)` and returns repeated `Node` ordered by FTS5 rank.
- ACL filtering is applied at query time, same as `QueryNodes`.

**What this makes easy:**

- Adding search to a field = one proto annotation + regen + write.
  No server change, no migration.
- Multi-language support via the `unicode61` tokenizer (accent
  folding, case folding) plus Porter stemming for English-shaped
  text.

**What this makes harder:**

- FTS on non-string fields. The schema registry rejects the
  annotation; numeric/boolean fields need a different mechanism
  (range query through `QueryNodes`).
- Cross-type search. Each type has its own FTS table; "search
  everything for `widget`" requires N queries client-side. Could be
  unified at the RPC layer in a future extension if the use case
  appears.
- FTS5 build dependency. The Go SQLite driver must compile or
  embed FTS5. `mattn/go-sqlite3` needs the `sqlite_fts5` build tag;
  `modernc.org/sqlite` ships it in default builds (preferred for
  cross-compile + supply-chain reasons). Driver choice is locked
  in by [ADR-001](001-storage-architecture.md).

**Failure modes:**

- A searchable field is renamed in proto. The FTS column name is
  `f{field_id}`, which doesn't change with rename, so search keeps
  working. (Field IDs are stable;
  [ADR-018](018-field-id-keyed-payloads.md).)
- An update changes a searchable field but the applier crashes
  before the FTS row is updated. On WAL replay the applier reapplies
  the event from scratch and the FTS row is reinserted.

## References

- [ADR-018](018-field-id-keyed-payloads.md) — field-ID-keyed
  payloads. FTS column names follow the same convention.
- [ADR-016](016-handlers-append-applier-writes.md) — handlers append
  to WAL; applier writes SQLite (and FTS5). FTS is never written
  from a handler.
- [ADR-023](023-declarative-query-indexes.md) — lazy-index pattern
  shared with FTS table creation.
- [ADR-025](025-single-shape-sdk-api.md) — `Search` is exposed via
  the single-shape SDK surface; no parallel `SearchWithACL` /
  `SearchByType` variants.
- Files this commit removes content from:
  - `docs/decisions/fts.md` — the FTS5 design is captured here; the
    file is deleted in the same commit.
