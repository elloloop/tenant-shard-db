# ADR-018: Payloads are keyed by `field_id` on the wire and on disk; the proto field number IS the `field_id`

**Status:** Accepted
**Decided:** 2026-05-15
**Tags:** wire-format, storage, schema, payload, proto
**Implementation:** PR #507 / v1.12.2 — SDKs pre-translate names→ids client-side; server rejects schemaless name-keyed payloads with `INVALID_ARGUMENT`

## Decision

Node payloads are stored as JSON keyed by stringified `field_id`s:

```json
{"1": "alice@example.com", "2": "Alice", "3": 30}
```

The wire format is the same shape. Field names (`email`, `name`,
`age`) appear only in the SDK's user-facing API — they're translated
to numeric `field_id`s at the SDK boundary before the request leaves
the client, and translated back from `field_id`s on read for
ergonomic user code.

**Critical equivalence:** the **proto field number** and the EntDB
**`field_id`** are the same number. `string email = 1;` makes the
EntDB field_id 1. There is no separate registration step that
assigns `field_id`s; the proto descriptor IS the registration.

Both SDKs (Python + Go) read `fd.number` from the proto descriptor
and emit id-keyed payloads. The server's ingress translator
(`server/go/internal/payload/translate.go`) enforces this:

- Schema-aware ingress: accepts id-keyed; accepts name-keyed for
  backwards compatibility (translates names→ids using the local
  registry).
- Schema-less ingress (registry has no entry for the type): accepts
  id-keyed only; rejects any name-keyed entry with `INVALID_ARGUMENT`.

The applier writes the id-keyed payload to SQLite verbatim. The
egress path on read is symmetric: the server returns id-keyed; the
SDK translates back to name-keyed for user code.

## Context

EntDB inherits protobuf's rename-free property. Renaming
`string email = 1;` to `string emailAddress = 1;` is metadata
only — proto consumers built against the old name keep working
because the wire/disk format keys on `1`, not `email`. For that
property to hold end-to-end in EntDB, **every layer below the SDK
must key on the number, not the name**.

The Python SDK historically emitted name-keyed payloads
(`{"email": "..."}`) and relied on server-side translation. The Go
server's schema-less translator silently dropped non-digit keys,
which on an unregistered type produced silent data loss
(`payload = {}` stored, every field lost without an error). PR #507
closed both halves: SDKs pre-translate names→ids client-side, server
rejects schemaless name-keyed payloads loudly instead of dropping
them.

The "schemaless" path matters because EntDB supports types the
server doesn't know about (the registry is populated lazily for
many deployments). Even for unregistered types, the wire is
id-keyed and the storage is id-keyed — the schemaless path doesn't
disable invariants, it just disables name→id translation server-side.

## Alternatives considered

- **Name-keyed payload on the wire** (the Python-era default).
  Rejected. Defeats the rename-free property — renames become
  full-table data migrations. Also: requires the server to know
  every type's schema to do translation, which conflicts with
  schemaless operation.

- **Mixed name+id wire (server accepts either, normalizes).**
  Rejected. Ambiguous: a future field literally named `"1"` would
  collide with the digit-passthrough heuristic. Also doubles the
  test matrix on every boundary.

- **Separate `field_id` registration table independent of proto
  numbering.** Rejected. Requires a second source of truth for
  field identities; the proto descriptor already has unique numbers
  per field. ADR-006 establishes proto as the type system; this
  ADR is the data-format consequence of that decision.

- **Server-side name→id translation only (Python-era pattern).**
  Rejected. Requires the server to have the schema for every type
  it sees. Either you mandate schema registration (breaks
  schemaless), or you silently drop unknown fields (the v1.12.0/1
  bug PR #507 fixed).

## Consequences

**What this locks in:**

- Wire payloads have stringified-numeric keys (`"1"`, `"2"`, …).
  Any non-digit key on a schemaless type's request returns
  `INVALID_ARGUMENT`.
- On-disk SQLite `payload_json` columns store stringified-numeric
  keys verbatim from the WAL event.
- SDKs do the name↔id translation. The Python SDK uses
  `_names_to_ids(fd.number, ...)` (`sdk/python/entdb_sdk/client.py`);
  the Go SDK uses generated proto serialization which already
  produces id-keyed output.
- Renaming a proto field is a code-only change. No data migration.
- Adding a field is additive: protoc assigns the next number, the
  SDK ships, the server stores values under the new id without
  needing any registration round-trip.

**What this makes easy:**

- Schema evolution without data rewrites.
- Forward/backward compatibility — old SDKs ignore unknown ids,
  new SDKs see them; the wire format is stable in shape.
- A schemaless mode that doesn't sacrifice the rename-free property.

**What this makes harder:**

- Hand-rolled clients (curl, grpcurl) can't easily compose payloads
  — they need the proto descriptor or a schema dump to know what
  field id maps to what user-facing name. This is intentional; the
  project's stance is "use an SDK" (see top-level README).
- Removing a field requires retiring the number (don't reuse it
  for a different field). `entdb-schema check` flags reuses.

**Failure modes:**

- SDK predates the v1.12.2 pre-translation fix and sends name-keyed
  payload to a schemaless type: server rejects with
  `INVALID_ARGUMENT` (loud, no silent data loss).
- Two proto files register the same `type_id` with different
  field-number assignments: protoc errors at compile, `entdb-schema
  check` flags at CI.
- A field is renamed at the proto level *and* its number is
  reassigned: client-side translation maps the new name to the new
  number; old data on disk under the old number is now orphaned.
  Don't do this; `entdb-schema check --baseline` catches it.
- A buggy SDK builds payload by reading `fd.name` instead of
  `fd.number`: the payload looks valid on the wire but won't match
  any registered field on the server's ingress. Detected by ingress
  validation.

## References

- [ADR-006](006-proto-schema-definition.md) — proto as the type
  system definition; this ADR is the data-format consequence.
- [ADR-016](016-handlers-append-applier-writes.md) — writes flow
  handler → WAL → applier → SQLite, all id-keyed.
- PR [#507](https://github.com/elloloop/tenant-shard-db/pull/507) /
  release v1.12.2 — closed the silent-data-loss path; this ADR
  records the design that PR implements.
- `server/go/internal/payload/translate.go` — boundary translator
  (ingress: optional name→id translation when schema known; reject
  on schemaless name keys).
- `sdk/python/entdb_sdk/client.py:_names_to_ids` — Python
  client-side translation.
- `sdk/go/entdb/` — Go SDK; uses generated proto serialization,
  which is id-keyed by construction.
