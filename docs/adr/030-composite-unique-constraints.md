# ADR-030: Composite (multi-field) unique constraints

**Status:** Accepted
**Decided:** 2026-05-25
**Tags:** schema, proto, indexing, integrity, sdk
**Implementation:** `(entdb.node).composite_unique` proto option; schema registry (`server/go/internal/schema/`); composite expression index + violation translation (`server/go/internal/store/`); applier enforcement (`server/go/internal/apply/`); SDK error parsing (`sdk/go/entdb/`, `sdk/python/entdb_sdk/`). Issue #566.

## Decision

A node type may declare one or more **composite (multi-field) unique
constraints**: a named set of two-or-more fields whose tuple of values
must be unique across all nodes of that type within a tenant. They are
declared in proto, resolved to `field_id` tuples at schema load, backed
by a SQLite UNIQUE expression index over the `json_extract` tuple, and
enforced atomically by the applier inside the WAL apply transaction. A
violation surfaces as a gRPC `ALREADY_EXISTS` carrying a structured
detail string both SDKs parse into a typed `UniqueConstraintError`.

This extends single-field uniqueness ([ADR-025](025-single-shape-sdk-api.md):
`(entdb.field).unique = true`) to the multi-field case without adding a
second enforcement mechanism — the same expression-index pattern, the
same applier-write-path, the same typed error.

### Proto declaration

`(entdb.node).composite_unique` is a `repeated UniqueConstraint`
message-level option (field number 24 on `NodeOpts`; field 23 / the old
`keys` is reserved). Additive and backward-compatible — existing schemas
omit it.

```proto
message OAuthIdentity {
  option (entdb.node).type_id = 201;
  option (entdb.node).composite_unique = {
    name: "provider_user_id"
    fields: ["provider", "provider_user_id"]
  };

  string provider         = 1 [(entdb.field) = { required: true }];
  string provider_user_id = 2 [(entdb.field) = { required: true }];
  string email            = 3 [(entdb.field) = { unique: true }];
}
```

`UniqueConstraint { repeated string fields; string name; }` carries
proto field **names**; the SDK codegen resolves each to its stable
`field_id` ([ADR-018](018-field-id-keyed-payloads.md)) at registration
time and a typo fails loudly there rather than silently dropping the
constraint. `name` is optional — when omitted the SDK derives a stable
name from the field ids. A constraint must reference **at least two**
fields; single-field uniqueness stays on `(entdb.field).unique = true`
(one way per operation, [ADR-025](025-single-shape-sdk-api.md)).

### Schema registry

`CompositeUniqueDef { Name string; FieldIDs []uint32 }` lives on
`NodeTypeDef.CompositeUnique`, round-trips through the schema JSON
contract (`composite_unique` key), and contributes to the schema
fingerprint. `Validate` enforces: non-empty name, ≥2 distinct field ids,
every field id known on the type, unique constraint names, and unique
field-id signatures (no two constraints over the same field set).
`Registry.CompositeUnique(type_id)` exposes them in declaration order.

### Index DDL

For each constraint the store mints a partial UNIQUE expression index:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_t201_cprovider_user_id
  ON nodes(tenant_id,
           json_extract(payload_json, '$."1"'),
           json_extract(payload_json, '$."2"'))
  WHERE type_id = 201;
```

Same shape as the single-field unique index
([ADR-025](025-single-shape-sdk-api.md)) and the query index
([ADR-023](023-declarative-query-indexes.md)), with two differences:

1. The index spans the **tuple** of `json_extract` expressions, in
   declaration order.
2. The name prefix is `idx_unique_t<type>_c<constraint>` (the `c`
   distinguishes composite from the single-field `_f<field>` form).

The index name is parseable back to its coordinates, and that round-trip
is load-bearing — see "Violation translation".

### Enforcement is in the applier, on the txn connection

Per [ADR-016](016-handlers-append-applier-writes.md) only the applier
writes SQLite. The applier ensures the unique / composite / query
indexes for a type **on the BatchTxn's own connection, before** the
`INSERT`/`UPDATE`, so the constraint fires synchronously inside the same
transaction. Running the DDL on a separate handle would deadlock against
the BatchTxn's held writer lock; the in-txn ensure path therefore does
NOT use the process-local index cache (a batch that creates an index and
then rolls back — because the very INSERT it guards trips the
constraint — drops the index with the txn, and a cache hit would then
skip re-creation and silently leave the constraint unenforced).
`CREATE ... IF NOT EXISTS` against an already-committed index is a cheap
metadata check, so re-running it per first-touch is correct and
inexpensive.

This also closes a pre-existing gap: [ADR-023](023-declarative-query-indexes.md)
described the applier ensuring unique + query indexes before each write,
but the WAL apply path did not actually do so. It does now, for
single-field unique, composite unique, and query indexes alike.

### Violation translation and the wire contract

A constraint trip is an **expected, deterministic outcome** — replaying
the same event against the same materialised state always reproduces it
— so it is handled exactly like a CAS precondition miss
([ADR-016](016-handlers-append-applier-writes.md), issue #500): the
applier rolls back the batch, memoizes the structured detail in the
idempotency cache with status `UNIQUE_VIOLATION`, and advances the WAL
offset **without halting** the consumer. The `ExecuteAtomic` handler
(when `wait_applied`) lifts the memoized detail into a gRPC
`ALREADY_EXISTS` status; a retry with the same idempotency key replays
the identical typed error.

`modernc.org/sqlite` reports an expression-index UNIQUE violation as
``UNIQUE constraint failed: index '<index_name>'``. The index name is
the authoritative source of *which* declared constraint fired, so the
store parses it (rather than guessing from the payload), resolves the
coordinates via the registry, and renders the colliding values from the
field-id-keyed payload.

The emitted detail strings are **verbatim wire contract** — pinned by
the SDK parsers (`parseUniqueConstraintFromStatus` /
`_parse_composite_unique_constraint_detail`):

```
single-field:
  Unique constraint violation: tenant=<t> type_id=<T> field_id=<F> value=<repr> already exists
composite:
  Composite unique constraint violation: tenant=<t> type_id=<T> constraint='<name>' fields=[<F>, ...] values=[<repr>, ...] already exists
```

`fields` / `values` are rendered as Python-literal lists and scalar
values as Python `repr()` (single-quoted strings, bare numerics) so
`ast.literal_eval` round-trips them on the Python side and the Go regex
parses them. Values are rendered from the `jsonnum`-decoded payload, so
an `int64` above 2^53 renders as its exact integer literal, never float
scientific notation — composite keys over big integers compare and
report losslessly ([ADR-028](028-typed-payload-wire-values.md)).

### SDK surface

Both SDKs map composite and single-field violations onto the **one**
`UniqueConstraintError` type (one way per operation,
[ADR-025](025-single-shape-sdk-api.md)). The composite form populates
`constraint_name` / `field_ids` / `values` and `IsComposite()` /
`is_composite` returns true; the single-field form populates
`field_id` / `value`. No `GetByCompositeKey` lookup is added — issue
#566 asks only for enforcement, and a typed composite-key fetch can be a
later additive surface if a use case appears.

## Context

There was no way to declare uniqueness spanning multiple fields. Clients
needing composite uniqueness (e.g. `(provider, provider_user_id)` for
OAuth identities) had to enforce it with a non-atomic query-then-create,
which loses under concurrency: concurrent creates of the same composite
key all pass the dup-check and insert, producing duplicate rows. This is
security-relevant for identity data — two principals claiming the same
external identity. Single-field unique already enforced atomically via a
unique expression index; the natural fix is the same mechanism over a
tuple.

## Alternatives considered

- **Synthesize the composite key into one single-field unique column.**
  The documented pre-#566 workaround. Rejected as the native answer: it
  pushes key derivation and collision-formatting into every client,
  duplicates the value on disk, and can't express the constraint in the
  schema where the integrity rule belongs.
- **Enforce at the gRPC handler with a pre-INSERT existence check.**
  Rejected — it is exactly the racy query-then-create the issue is
  about, and it violates [ADR-016](016-handlers-append-applier-writes.md)
  (handlers append; only the applier writes).
- **A dedicated composite-key table (`(tenant, type, key_hash) → node`).**
  Rejected. A second write target means a second consistency story
  between `nodes` and the key table; the SQLite expression index is one
  atomic INSERT.
- **Halt the consumer on a constraint trip (treat as poison).**
  Rejected. A unique violation is a legitimate, deterministic client
  outcome, not corruption; halting would wedge the WAL on user error.
  The deterministic-failure memoization path (issue #500) already models
  exactly this — reuse it.
- **A new `ReceiptStatus` enum value for the violation.** Rejected as
  unnecessary for the contract: the authoritative typed surface is the
  `ExecuteAtomic` `ALREADY_EXISTS` gRPC status. `GetReceiptStatus`
  surfaces a polled violation as `RECEIPT_STATUS_FAILED` with the detail
  in `error` — additive, no proto break.
- **A typed `GetByCompositeKey` lookup in this ADR.** Deferred. Out of
  scope for #566 (enforcement only); add later if a fetch-by-tuple use
  case appears, without a parallel error path.

## Consequences

**What this locks in:**

- `NodeOpts.composite_unique` (field 24) and the `UniqueConstraint`
  message are the stable schema annotation; field 23 / `keys` stays
  reserved.
- `CompositeUniqueDef` (name + field-id tuple) on `NodeTypeDef`,
  serialised under the `composite_unique` JSON key, part of the
  fingerprint.
- Index name `idx_unique_t<type>_c<constraint>`; the name is parsed back
  to coordinates, so the minting and parsing helpers must stay in
  lock-step (they share `compositeIndexSuffix`).
- The two ALREADY_EXISTS detail formats above are wire contract;
  changing them breaks the shipped SDK parsers.
- The applier's in-txn "ensure field indexes" hook covers single-field
  unique, composite unique, and query indexes before every
  `CreateNode` / `UpdateNode`.

**What this makes easy:**

- Declaring a multi-field integrity rule with one proto annotation,
  enforced atomically under concurrency.
- Catching the violation by exception type (not string match), with the
  constraint name / field ids / colliding values available.

**What this makes harder:**

- Adding a composite constraint to a type that already has duplicate
  tuples is BREAKING — the `CREATE UNIQUE INDEX` fails. The compat
  checker flags `COMPOSITE_UNIQUE_ADDED` / `..._CHANGED` as breaking;
  back-filling / de-duplicating is the operator's responsibility before
  the constraint can be added (same posture as adding single-field
  `unique`).
- Dropping a constraint requires a manual `DROP INDEX` (same as
  single-field unique / query indexes).

**Failure modes:**

- The lazy in-txn DDL runs on the first create/update of a type after
  start-up. A type that only ever sees updates won't have its index
  created until a create — mitigated by ensuring on both `CreateNode`
  and `UpdateNode` ([ADR-023](023-declarative-query-indexes.md) carries
  the same note).
- If `wait_applied` is false (or the client raced the applier), the
  caller polls `GetReceiptStatus`, which reports the violation as
  `RECEIPT_STATUS_FAILED` with the detail in `error`; the
  ALREADY_EXISTS gRPC surface requires the wait_applied path.

## References

- Issue #566 — composite unique constraints request + identity-race
  motivation.
- [ADR-025](025-single-shape-sdk-api.md) — single-field unique via
  expression index; one-way-per-operation rule (one
  `UniqueConstraintError`, no parallel composite lookup).
- [ADR-023](023-declarative-query-indexes.md) — the lazy expression
  index pattern reused here; this ADR also implements the
  ensure-before-write hook ADR-023 described.
- [ADR-022](022-fts5-full-text-search.md) — same lazy-index pattern for
  FTS5 virtual tables.
- [ADR-018](018-field-id-keyed-payloads.md) — field-id-keyed payloads;
  constraints store and index on `'$."<field_id>"'`, never the proto
  name.
- [ADR-028](028-typed-payload-wire-values.md) — lossless int64; composite
  keys over big integers compare and report exactly.
- [ADR-016](016-handlers-append-applier-writes.md) — handlers append,
  applier writes; the deterministic-failure memoization path (issue
  #500) the unique-violation path reuses.
