# ADR-029: Keyset cursor pagination for reads (`page_token` / `next_page_token`)

**Status:** Accepted — design frozen 2026-05-23. **QueryNodes server
implemented** (#564): keyset cursor with `page_size` / `page_token` →
`next_page_token`, fingerprint-bound tokens, seek-not-skip continuation.
The characterization test
`tests/python/integration/test_query_range.py::test_query_does_not_silently_truncate`
is now green (un-xfailed). **`GetEdgesFrom`/`GetEdgesTo`** (#580): keyset over
`(created_at, edge_type_id, peer_node_id)` + both-SDK auto-follow.
**`ListUsers`** (#580): keyset over `(created_at, user_id)` + Python
auto-follow (the Go SDK does not expose ListUsers); also fixed its
swallow (#573-class) to surface store faults.
**`ListSharedWithMe`** (#580): UNIFIED keyset over
`(timestamp, source_tenant, node_id)` spanning BOTH merged sources —
per-tenant `node_access` (keyed on `granted_at`) and global `shared_index`
(keyed on `shared_at`) — with both-SDK auto-follow. See "Resolved:
`ListSharedWithMe` — unified merge-cursor" below.
**`SearchNodes`** (#580): RESOLVED as offset-paged, NOT keyset — FTS5
`rank` is not a stable keyset column. See "Resolved: `SearchNodes` —
offset-paged ranked search (FTS carve-out)" below.
**`GetConnectedNodes`** (#580): RESOLVED as an intentionally bounded
traversal, NOT cursor-paginated. See "Resolved: `GetConnectedNodes` —
bounded traversal, not cursor-paginated" below.
**Decided:** 2026-05-23
**Tags:** api, pagination, query, sdk, consistency, read-path
**Complements:** [ADR-023](023-declarative-query-indexes.md) (declarative
query indexes — the index that backs `order_by` also backs keyset seeks)
and [ADR-025](025-single-shape-sdk-api.md) (single-shape SDK API — the
cursor lives behind the same helpers).
**Implementation:** QueryNodes done — `proto/entdb/v1/entdb.proto`,
`server/go/internal/api/{pagetoken,query_nodes}.go`,
`server/go/internal/store/nodes.go`. Pending — `sdk/go/entdb/`,
`sdk/python/entdb_sdk/` (auto-follow), and
`server/go/internal/api/{get_edges_from,get_edges_to,search_nodes,list_*}.go`.
Both SDKs ship together.

## Decision

Reads adopt **keyset (seek) cursor pagination** in the
[AIP-158](https://google.aip.dev/158) shape:

- Requests carry `page_size` and `page_token`.
- Responses carry `next_page_token` (empty when the last page is reached).
- The token is **opaque** to clients and encodes a **keyset cursor**: the
  `(order_by value, node_id)` tuple of the last row returned, plus the
  sort direction and a fingerprint of the query (type_id + filters +
  order_by) so a token cannot be replayed against a different query.
- Continuation is a seek, not a skip:
  `WHERE (order_by_col, node_id) > (cursor)` (or `<` for descending),
  `ORDER BY order_by_col, node_id`, `LIMIT page_size`. `node_id` (unique)
  is always appended as the final sort key so the ordering is a **total
  order** and the cursor is unambiguous.
- Applies to `QueryNodes`, `GetEdgesFrom`/`GetEdgesTo`, and the `List*`
  RPCs (including `ListSharedWithMe` via a unified merge-cursor). Three
  reads are carve-outs, resolved below: `SearchNodes` (offset-paged ranked
  search) and `GetConnectedNodes` (bounded BFS traversal) are NOT keyset.

`page_size` defaults to 100 (unchanged) and is clamped to `MaxPageSize`
(1000). The point is no longer the per-page cap — it is that **the client
can always retrieve the remainder via `next_page_token`.**

**SDK helpers auto-follow the cursor.** `List*` / `query` helpers loop
`next_page_token` to return the complete set by default (or expose a lazy
iterator / async generator for streaming). A read of N rows returns N rows,
never a silent prefix.

The legacy `offset` field is retained for backward compatibility but
**deprecated**; new code uses the cursor.

### Resolved: `SearchNodes` — offset-paged ranked search (FTS carve-out)

`SearchNodes` is a **carve-out: it is offset-paged, not keyset-paged.**
FTS5 `rank` (bm25) is computed by the `MATCH` operator at query time and
is **not a column you can filter in a `WHERE` seek** — there is no
`WHERE rank > :cursor` that the index can serve, so a keyset cursor over
relevance order is impossible. Materialising `(rank, node_id)` into a
sortable column was considered and rejected: rank is query-dependent (it
changes with the match expression), so it cannot be precomputed per row.

The deeper point is that **deep-paging relevance-ranked results is an
anti-pattern**: search is "top-N by relevance," and page 50 of a ranked
result set is noise. So `SearchNodes` keeps `limit`/`offset` (now with
`page_size` as the AIP-158 alias that takes precedence over `limit`), adds
an **exact `has_more`** (the server asks the store for `limit+1` rows and
trims the probe), and has **no `next_page_token`**. The SDK `search`
helpers expose `page_size` + `offset` and **do NOT auto-follow to
completion** — unlike `query`/`edges`/`shared-with-me`, which do. This is
the one read where returning a bounded top-N prefix is the correct
contract, not a defect.

### Resolved: `ListSharedWithMe` — unified merge-cursor

`ListSharedWithMe` merges two sources — the per-tenant `node_access` index
(ordered by `granted_at`) and the global cross-tenant `shared_index`
(ordered by `shared_at`). Both timestamps are integer columns, and
`(source_tenant, node_id)` uniquely identifies a shared node, so the two
streams share **one total order**: `(timestamp DESC, source_tenant DESC,
node_id DESC)`. A single keyset cursor over that tuple drives BOTH
sources: each is queried `WHERE (ts, source_tenant, node_id) < cursor
ORDER BY ts DESC, source_tenant DESC, node_id DESC LIMIT page_size+1`, the
two result sets are merged, deduped by `(source_tenant, node_id)` (a node
in both sources collapses to its higher-timestamp position), the top
`page_size` are returned, and the cursor for the next page is the tuple of
the last merged row. Fetching `page_size+1` from **each** source is
sufficient because the globally-next `page_size+1` distinct tuples are a
subset of the union of each source's own next `page_size+1` tuples — so
`has_more` and the page are exact. The token is fingerprint-bound to the
recipient + tenant and is mutually exclusive with the deprecated `offset`.
Both SDKs auto-follow this cursor to completion.

### Resolved: `GetConnectedNodes` — bounded traversal, not cursor-paginated

`GetConnectedNodes` is a **BFS graph traversal**, not an ordered scan over
a single index, so a keyset cursor is **ill-defined**: the "last row" of a
page is not a stable seek anchor for the next, because the frontier is
recomputed from the edge set, ACL-pruned per step, and deduped across
fan-in. It stays a deliberately **bounded read**: it returns at most
`limit` reachable, ACL-visible nodes within `MaxConnectedDepth`, with an
**exact `has_more`** (the traversal collects one probe node beyond `limit`
to distinguish "page full" from "more exist"). There is no
`next_page_token`; callers needing the full neighbourhood raise `limit`.
This is documented in the handler header and is not a truncation defect —
the traversal is explicitly bounded by contract.

## Context

Bug A: reads silently truncate. `QueryNodes` defaults to
`defaultQueryLimit = 100` (`query_nodes.go:56`); the Go SDK's `QueryNodes`
transport has **no limit/offset/cursor parameter at all**
(`sdk/go/entdb/client.go:37`), and the Python `list_*` / `query` helpers
default to `limit=100` and never paginate. So `ListPasskeyCredentials`,
`list_users`, etc. return 100 of N with no error and no way to get the
rest. `MaxPageSize=1000` is irrelevant because no caller sets a limit, and
even at 1000 there is no cursor to go further.

A standard database never silently truncates: `SELECT` returns all matching
rows, and the modern best practice for stable, scalable chunking is keyset
(seek) pagination, not `LIMIT/OFFSET`. For a gRPC API the established
convention is AIP-158 `page_token` / `next_page_token`. EntDB's read model
is an ordered SQLite table (a materialized view of the WAL); `node_id` is
unique and `order_by` over stable columns already exists — keyset is a
clean fit and is stable under the append-only write model.

## Invariants (must hold)

1. **Total order for the cursor.** The effective sort is
   `(order_by_col, node_id)`; `node_id` is unique, so no two rows share a
   cursor position and continuation can never skip or duplicate a row, even
   under concurrent inserts/deletes between pages.
2. **Token integrity.** `page_token` encodes the query fingerprint; a token
   presented with a different `type_id` / filters / `order_by` is rejected
   with `INVALID_ARGUMENT` rather than silently returning wrong rows.
3. **No silent truncation.** A response that omits rows MUST set a
   non-empty `next_page_token`. The SDK helpers MUST follow it to
   completion unless the caller explicitly opts into manual paging.

## Alternatives considered

- **Offset-loop in the SDK helpers** (page via the existing `offset`).
  Rejected — a short fix: `OFFSET` is O(n) to skip and, under concurrent
  writes, skips or duplicates rows across pages. Violates the "no short
  fixes" / standard-DB-best-practice bar.
- **Just raise `defaultQueryLimit` / `MaxPageSize`.** Rejected — still a
  hard cap, doesn't scale past it, and large single responses collide with
  the 4 MiB gRPC message limit and server memory.
- **Unbounded streaming response (server streams all rows).** Rejected for
  the unary read RPCs — unbounded message/memory; a separate streaming RPC
  could be added later but is out of scope for fixing the truncation.
- **Keep returning a prefix, document the cap.** Rejected — the defect is
  precisely that callers don't know rows were dropped.

## Consequences

**Locks in:**
- The read contract is `page_size` + `page_token` → `next_page_token`, with
  an opaque keyset token over `(order_by, node_id)`.
- `order_by` columns must be indexable / totally orderable with `node_id`
  as tiebreaker (ties into ADR-023 declarative indexes).
- SDK `List*`/`query` helpers return the complete set by default.

**Makes easy:**
- Correct, stable pagination over arbitrarily large result sets.
- Cheap deep pages (seek, not skip).

**Makes harder / tradeoffs:**
- Additive proto + server + both-SDK work, and token encode/decode +
  validation logic.
- Keyset cannot jump to an arbitrary page number (no "page 7 of 200"); only
  next/continue. Accepted — random-access paging is not a requirement and
  is exactly what makes `OFFSET` slow and unstable.
- Changing a query's filters/sort invalidates an in-flight token (by
  design, invariant 2).

**Failure modes:**
- Token presented against a mismatched query → `INVALID_ARGUMENT`.
- Caller uses both `offset` and `page_token` → `INVALID_ARGUMENT` (pick one;
  `offset` is deprecated).

## References

- [AIP-158](https://google.aip.dev/158) — pagination shape adopted.
- [ADR-023](023-declarative-query-indexes.md) — indexes backing `order_by`
  also back keyset seeks.
- [ADR-025](025-single-shape-sdk-api.md) — the cursor lives behind the
  single-shape SDK helpers.
- [ADR-005](005-event-sourcing-wal.md) — the read model is an ordered
  materialized view, which makes keyset stable.
- Bug A characterization test (landed `xfail`):
  `tests/python/integration/test_query_range.py::test_query_does_not_silently_truncate`.
- Related: [ADR-028](028-typed-payload-wire-values.md) (the sibling
  data-integrity fix frozen the same day).
