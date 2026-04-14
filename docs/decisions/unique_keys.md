# Unique keys / secondary lookup keys

Frozen decisions about client-asserted unique constraints and secondary primary keys on nodes. Newest first.

---

## 2026-04-13: Client-asserted unique keys + secondary lookup keys via one `node_keys` table

**Status:** superseded
**Decided:** 2026-04-13
**Tags:** schema, indexing, consistency, sdk, proto
**Supersedes:** none
**Superseded by:** [sdk_api.md — 2026-04-14 Single-shape SDK API](sdk_api.md#2026-04-14-single-shape-sdk-api-typed-unique-key-tokens-via-codegen-expression-index-unique-enforcement) — the user-facing concept (proto declares unique fields, server enforces) survives, but the `node_keys` table is replaced by SQLite unique expression indexes on `nodes(tenant_id, json_extract(payload, '$."<field_id>"'))`, the message-level `keys: [...]` option moves to field-level `(entdb.field).unique = true`, and the `keys={}` parameter on `plan.create` is deleted (the proto field IS the key).

### Decision

EntDB adds a first-class **unique keys** concept for nodes, unifying two features the schema review surfaced:

1. **Client-asserted uniqueness** — the application computes uniqueness (e.g. an email address, a stripe customer id) and passes it at create time. The server enforces it inside the apply transaction and rejects duplicates.
2. **Secondary primary key / lookup key** — the same value is indexed so the application can look up the node by key instead of by server-generated `node_id`.

Both features share one underlying mechanism: a per-tenant `node_keys` table indexed on `(tenant_id, type_id, key_name, key_value)` as a primary key. A single column is both unique and searchable — same thing, two names.

**Declaration site: proto.** Each node type declares its keys in `entdb_options.proto` via a new `keys` option:

```
message User {
  option (entdb.node) = {
    type_id: 101
    keys: [
      { name: "email", required: true }
      { name: "external_id", required: false }
    ]
  };
  string email = 1;
  string name = 2;
  string external_id = 3;
}
```

All keys declared this way are unique within `(tenant_id, type_id, key_name)`. There is no "non-unique" mode in Phase 1 — if you need a non-unique secondary index, use a filter query. This keeps the mechanism simple and rules out ambiguity.

**Client API.** The SDK (Python + Go) exposes:

- `plan.create(node, keys={"email": "alice@example.com", "external_id": "ext-42"})` on write
- `scope.get_by_key(Type, key_name, key_value)` on read

The server-generated `node_id` is still the canonical identifier for edges and ACL. The `keys` are an additional index, not a replacement.

**Consistency model: pre-validate + authoritative validate.** Uniqueness enforcement runs at two points:

1. **Pre-validate at the gRPC boundary** (fast feedback). Before appending the event to the WAL, the servicer does a quick `SELECT node_id FROM node_keys WHERE ...` against the canonical store. If a duplicate already exists, return `ALREADY_EXISTS` immediately. This handles the 99% case where the client is trying to register an email that already exists — no WAL round-trip, no apply lag.
2. **Authoritative validate at apply time** (race-proof). The Applier, inside the same `batch_transaction` that writes the node row, does the same check against `node_keys` and rejects the event if a race committed between the pre-validate and the apply. The write path uses `INSERT OR FAIL` on the `node_keys` primary key for atomicity: if a concurrent insert got there first, the applier sees `IntegrityError` and fails the apply, halting the tenant loop per the existing halt-on-error semantics.

Clients that need strong read-your-write semantics pass `wait_applied=True`, same as every other write. Failed creates surface as typed `UniqueConstraintError` on both SDKs.

**Lookup path.** `GetNodeByKey` is a new gRPC RPC:
- Input: `tenant_id`, `actor`, `type_id`, `key_name`, `key_value`
- Server does `SELECT node_id FROM node_keys WHERE ... LIMIT 1`, then a normal `GetNode(node_id)` (which honors ACL).
- If the key doesn't exist → `NOT_FOUND`
- If the key exists but the actor lacks ACL on the node → `PERMISSION_DENIED` (same as a direct `GetNode`)

**Updates.** When `update_node` changes a field that is also a declared key:
- New value is validated against `node_keys` (pre-validate + authoritative validate, same as create)
- Old `node_keys` row is DELETEd, new row is INSERTed, both inside the same transaction
- If the new value collides, the update is rejected and the old key stays

**Deletes.** `delete_node` cascades to `DELETE FROM node_keys WHERE tenant_id=? AND node_id=?` via the secondary index.

**What is NOT in scope (explicit non-goals):**
- Non-unique secondary indexes — use filter queries
- Multi-valued keys (one key_name mapping to many values) — if you need this, model as edges
- Cross-type uniqueness — each `(tenant_id, type_id, key_name)` namespace is independent
- Cross-tenant uniqueness — always scoped per tenant
- Full-text search — out of scope, separate feature
- Compound keys made of multiple columns — declare separately; combine client-side if needed

### Context

The schema review identified a real gap: to look up a user by email today, the app has to query-all-users-filter-by-email, which is O(n) in the tenant's user count and requires a second round-trip if you want a consistent unique-email constraint. Existing alternatives:

- **Use email as node_id directly.** Works but couples the application's natural key to the server's internal id; makes rename impossible; leaks email into edges/ACL; doesn't help for secondary keys (external_id, username).
- **Store email + a custom index outside EntDB.** Splits consistency between two systems, invites drift.
- **Scan-and-filter via `query_nodes`.** O(n) per lookup, not suitable for hot paths.

A first-class `keys` mechanism inside the node schema is cheap (one new table, one new index, one RPC), solves both the uniqueness and the lookup problem with one feature, and integrates cleanly with the existing proto-first schema design.

### Alternatives considered

- **Enforce uniqueness only client-side.** Rejected. Race conditions between two clients writing the same email are inevitable; the database has to be the authority.
- **Use a separate Redis / etcd cluster for uniqueness locks.** Rejected. Adds an operational dependency and a cross-system consistency story for a feature that fits in one SQLite table.
- **Server-computed uniqueness via a hashing function the server chooses.** Rejected. Applications know their natural keys; hard-coding a hashing convention forces awkward workarounds (email vs email-lowercased, unicode normalization, etc.). Client provides the normalized value, server enforces exactness.
- **Declare keys in SDK code (registry.register_node_type) instead of proto.** Rejected. The proto-first decision (acl.md) establishes proto as the single source of truth for type metadata. Putting `keys` somewhere else splits the schema story.
- **Allow multi-valued keys.** Rejected for Phase 1. Edges already model "many values per node". Adding multi-valued keys blurs the line and invites queries the index can't answer.
- **Rely on `wait_applied=True` only, skip pre-validate.** Rejected. Pre-validate gives fast-fail at request time for the common case (99% of dup creates are from a single client retrying). Applier-side validate handles races. Both are cheap.

### Consequences

**What this locks in:**

- New `node_keys` table in every tenant SQLite file: `PRIMARY KEY (tenant_id, type_id, key_name, key_value)` + `INDEX (tenant_id, node_id)`. Backwards compatible — existing tenants migrate with an idempotent `CREATE TABLE IF NOT EXISTS` on first apply.
- `entdb_options.proto` gets a `NodeKeySpec` message and a `repeated NodeKeySpec keys = ...` field on `NodeOpts`.
- `CreateNodeOp` gains a `map<string, string> keys = <tag>` field (client-provided values).
- New gRPC `GetNodeByKey(GetNodeByKeyRequest) returns (GetNodeByKeyResponse)` RPC.
- Applier enforces uniqueness authoritatively inside `_sync_apply_event_body` create/update branches. Failures propagate as `IntegrityError` → `ApplyResult(success=False)` → halt-on-error loop exits.
- gRPC servicer pre-validates at ingress for fast-fail. This is an optimization, not a correctness guarantee — races still land in the applier.
- SDKs expose `plan.create(..., keys={...})`, `scope.get_by_key(...)`, and a typed `UniqueConstraintError`.
- Capability registry: `GetNodeByKey` requires `CORE_CAP_READ` on the resolved node, same as `GetNode`.

**What this makes easy:**

- Email-to-user, SKU-to-product, external-id-to-entity lookups are a single O(log n) query.
- Idempotent create-or-fetch patterns: `try: create(..., keys={"email": ...}) except UniqueConstraintError: get_by_key(...)`.
- Application migrations from systems with natural primary keys (Postgres, MongoDB unique indexes) are mechanical.
- Audit trails: `node_keys` insert/delete events are captured in the WAL, so uniqueness changes are reproducible.

**What this makes harder:**

- Schema evolution: renaming or dropping a key in proto requires migrating existing `node_keys` rows. The runtime doesn't enforce unknown keys (forward-compat), but retired keys need a one-off cleanup.
- Writes do one extra `SELECT` + up to N `INSERT`s (typically 1–3). At ~10μs each this is negligible, but high-fan-out types with 20+ keys would feel it. Keep the declared key count low.
- Updates that change a keyed field do a DELETE + INSERT inside the transaction. Rare enough to not worry about.
- The secondary index uses storage proportional to `nodes × avg_keys × avg_value_len`. ~50 bytes per key per node. For 10M nodes × 1.5 keys that's ~750MB. Plan accordingly.

**Failure modes:**

- Race between two concurrent creates with the same key → one succeeds, the other gets `UniqueConstraintError` from the applier. Client retries with get_by_key.
- Applier is behind the gRPC boundary → pre-validate might pass stale, but applier catches it. User sees the error via `wait_applied=True` or on a subsequent read.
- Key value longer than a reasonable threshold (say 512 bytes) — rejected at ingress with `INVALID_ARGUMENT` to prevent pathologically large index rows.
- Null / empty key values — rejected at ingress. If a key is declared `required: true` in proto, missing it at create time is also `INVALID_ARGUMENT`.

### References

- Conversation: 2026-04-13 architecture discussion on unique constraints + secondary keys
- Related decisions:
  - [acl.md — 2026-04-13 Typed capability-based permissions](acl.md#2026-04-13-typed-capability-based-permissions-core--per-type-extensions) — proto-first schema establishes where keys are declared
  - [storage.md — 2026-04-13 Immutable storage mode](storage.md#2026-04-13-immutable-storage-mode-no-built-in-drafts-primitive) — node_keys table lives in whichever physical file the node lives in (tenant / mailbox / public)
- Implementation: planned as follow-up PR after `feature/security-and-shutdown-fixes` merges
