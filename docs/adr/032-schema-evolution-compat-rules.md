# ADR-032: Schema-evolution compatibility rules + a customer-runnable gate

**Status:** Proposed
**Decided:** 2026-05-25
**Tags:** schema, proto, wire-contract, sdk, tooling, ci, integrity

**Implementation:** compat classifier
(`server/go/internal/schema/compat.go`); reserved-id tombstones
(`server/go/internal/schema/types.go`, `loader.go`, `registry.go`); the
`breaking` subcommand (`server/go/cmd/entdb-schema/breaking.go`);
distribution (`.github/workflows/release.yml`, `Dockerfile.schema`);
recipes (`Makefile`, `docs/recipes/buf.yaml.template`,
`docs/guides/schema-lockdown.md`).

## Decision

An EntDB application team must be able to catch a dangerous schema change
**before deploy**, with a SINGLE command run **identically** in local dev
and in CI — the runtime-schema analogue of `buf breaking`. We ship that as
the `entdb-schema breaking` verb over the existing compat classifier, and
we fix the classification matrix to one governing principle:

> **LOOSENING is safe; TIGHTENING (and identity reuse / data-corruption)
> is breaking.**

The tool reuses the existing `entdb-schema` CLI (no buf plugin). It
operates on **name-free** schema snapshots (ids only) per
[ADR-031](031-self-describing-name-free-schema.md) and exits **non-zero**
on any breaking change.

## The rule matrix

A change is classified against the committed baseline snapshot. The
machine-readable classification table is `breakingKinds` in
`compat.go`; flipping a row there is the audit trail.

### BREAKING (tightening / identity / data-corrupting)

| Change | Kind | Why |
| --- | --- | --- |
| add `unique` (false→true) | `FIELD_UNIQUE_ADDED` | historical rows may already collide; `CREATE UNIQUE INDEX` fails like Postgres |
| add `required` (false→true) | `FIELD_REQUIRED_TIGHTENED` | existing rows that omit the field now fail validation |
| change field `kind` | `FIELD_KIND_CHANGED` | type-coercion of stored bytes is unsafe |
| change / reuse a `field_id` | `FIELD_ID_REUSED` (reuse of a reserved id) / remove+add | the field number is the on-disk key ([ADR-018](018-field-id-keyed-payloads.md)); reuse re-reads old rows as a different field |
| reuse a removed/reserved `field_id` | `FIELD_ID_REUSED` | a tombstoned id brought back to life corrupts historic rows |
| add a `composite_unique` constraint | `COMPOSITE_UNIQUE_ADDED` | existing tuples may already violate ([ADR-030](030-composite-unique-constraints.md)) |
| change a composite's field-set | remove+add (tuple IS identity) | the new tuple's index re-build can fail |
| change / reuse a `type_id` / `edge_id` | `TYPE_ID_REUSED` / `EDGE_ID_REUSED` (reuse) / remove+add | type identity is the partition/replay key; reuse orphans or mis-routes history |
| change an edge's from/to type | `EDGE_FROM_TYPE_CHANGED` / `EDGE_TO_TYPE_CHANGED` | edge semantics shift; historical edges become invalid |

Other breaks the engine already enforced and keeps: `SUBJECT_FIELD_CHANGED`,
`DATA_POLICY_LOOSENED`, `FIELD_REF_TYPE_CHANGED`, `ENUM_VALUE_REMOVED`,
`ENUM_VALUE_REORDERED`, `EDGE_UNIQUE_PER_FROM_ADDED`, `ON_SUBJECT_EXIT_CHANGED`.

### SAFE (loosening / additive)

| Change | Kind | Why |
| --- | --- | --- |
| drop `unique` | `FIELD_UNIQUE_REMOVED` | strictly looser |
| drop `required` | `FIELD_REQUIRED_LOOSENED` | strictly looser |
| add / remove `indexed` | `FIELD_INDEXED_ADDED` / `_REMOVED` | query index is additive metadata ([ADR-023](023-declarative-query-indexes.md)) |
| add / remove `searchable` | `FIELD_SEARCHABLE_ADDED` / `_REMOVED` | FTS index is additive metadata ([ADR-022](022-fts5-full-text-search.md)) |
| add a new field (new `field_id`) | `FIELD_ADDED` | additive |
| remove a field (id reserved) | `FIELD_REMOVED` | loosening — old rows simply stop being read for that id; reuse is the break |
| add a new type | `NODE_ADDED` / `EDGE_ADDED` | additive |
| remove a type (id reserved) | `NODE_REMOVED` / `EDGE_REMOVED` | loosening; reuse is the break |
| drop a `composite_unique` | `COMPOSITE_UNIQUE_REMOVED` | strictly looser |

### Reserved ids — why removal is safe

Removal is safe **because the freed id is then off-limits**. This is the
proto `reserved` discipline. The snapshot carries explicit tombstones:

- `reserved_field_ids` on a node/edge type (`types.go`),
- `reserved_type_ids` / `reserved_edge_ids` at the schema root (`loader.go`).

These are `omitempty`, so a schema that doesn't reserve emits no extra
keys and its **fingerprint is unchanged** (the same name-free canonical
form ADR-031 hashes). The classifier turns a baseline-reserved id that
reappears as **live** into the corresponding `*_REUSED` BREAKING change
instead of a benign add. A live id that is also reserved is rejected at
load time as a definition error.

This pairs the SAFE "remove" rows with their BREAKING "reuse" rows: you
may delete freely, but the only way to re-introduce a freed id is a
breaking change a reviewer must see.

## The command (buf-breaking ergonomics)

```bash
entdb-schema breaking --baseline schema.lock.json --from-file new.json
```

`breaking` is the buf-style verb; `check` remains as the equivalent
alias (same engine `schema.Check`, same flags, same exit codes). Both
exit:

- `0` — compatible (non-breaking changes are still printed),
- `1` — one or more BREAKING changes,
- `2` — usage / I/O error.

The current schema is produced **from the customer's own proto**: snapshot
a running server (`--from-server`, which serves the registry the SDK's
self-describing writes established, ADR-031) or re-canonicalise a file
(`--from-file`). The baseline is a plain committed JSON lock file.

## Distribution (runs trivially in BOTH local dev and CI)

The same artifact, the same invocation, everywhere:

- **ghcr image** — `Dockerfile.schema` (distroless static) is built and
  pushed multi-arch per tag by `.github/workflows/release.yml` as
  `ghcr.io/elloloop/tenant-shard-db-schema:vX.Y.Z`. The release smoke
  test asserts the `breaking` verb is wired.
- **Prebuilt binary** — `build-schema-binaries` cross-compiles
  linux/darwin/windows × amd64/arm64 and attaches each archive (+ a
  `.sha256`) to the GitHub Release.
- **`go install`** — works for local dev
  (`…/server/go/cmd/entdb-schema@vX.Y.Z`) but the server module is not
  tagged for external consumers, so the image / release binary are the
  recommended channels.

Recipes: a `make schema-breaking` target (identical to the CI step), a
`buf.yaml` template for the wire-contract `buf breaking` companion, and a
CI-step + local-recipe walkthrough in
`docs/guides/schema-lockdown.md`. `buf breaking` defends the wire;
`entdb-schema breaking` defends the runtime — run both.

## Context

Before this ADR the compat engine classified type/field/edge *removal*
as breaking and had no notion of a reserved id, no `indexed`/`searchable`
diff, and no buf-style verb. That over-flagged the common safe case
(deleting a field you've stopped writing) while under-protecting the real
danger (re-using a freed id later). [ADR-030](030-composite-unique-constraints.md)
added composite-unique detection; [ADR-031](031-self-describing-name-free-schema.md)
made the engine name-free; this ADR completes the matrix and ships the
customer-facing gate.

## Alternatives considered

- **A buf plugin.** Rejected — buf plugins inspect the proto descriptor,
  but the breaks here live in the runtime registry (id identity,
  uniqueness over stored rows, composite tuples, edge endpoints) that the
  wire descriptor does not capture. `entdb-schema` already loads the
  name-free registry; reusing it is lower friction and keeps one engine.
- **Classify removal as breaking (status quo).** Rejected — it punishes
  the safe, common case and conflates "deleted" with "reused". Proto and
  every mature schema tool treat removal-plus-reservation as safe.
- **Infer reservations (treat every removed id as implicitly reserved
  forever).** Rejected as the *enforcement* mechanism — without an
  explicit tombstone in the snapshot the checker cannot tell a deliberate
  re-introduction from a fresh add. Explicit `reserved_*` lists make the
  reuse visible in code review, exactly like proto `reserved`.

## Consequences

**Locks in:**
- The loosen-safe / tighten-breaking principle and the matrix above as
  the classification contract.
- Explicit reserved-id tombstones (`reserved_field_ids`,
  `reserved_type_ids`, `reserved_edge_ids`) as the removal-safety
  mechanism, fingerprint-neutral when unused.
- `entdb-schema breaking` as the customer-runnable gate, `check` as its
  alias.

**Makes easy:**
- One command, identical in local dev and CI, that fails the build on a
  tightening change and passes a loosening one.
- Deleting fields/types freely while a reviewer still sees any id reuse.

**Makes harder / breaking (for this repo's own tooling):**
- `NODE_REMOVED` / `FIELD_REMOVED` / `EDGE_REMOVED` are reclassified from
  breaking to safe; downstream parsers keying on those names as "breaking"
  must update (the `breaking` flag in the JSON output is authoritative).

**Deferred:**
- `--from-descriptors` (load the registry from a `buf build` descriptor
  set) remains reserved (issue #488 open question #6).
- Auto-deriving reserved lists from proto `reserved` during SDK codegen
  (today the customer keeps the proto `reserved` and the snapshot reserved
  list in lockstep by hand / via snapshot regeneration).

## References

- [ADR-018](018-field-id-keyed-payloads.md) — `field_id`/`type_id` are the
  on-disk keys; this matrix is built around protecting that identity.
- [ADR-030](030-composite-unique-constraints.md) — composite-unique
  constraints, whose add/drop this matrix classifies.
- [ADR-031](031-self-describing-name-free-schema.md) — name-free schema;
  the compat tool and the fingerprint operate on ids only, and the
  reserved tombstones are fingerprint-neutral against that canonical form.
- [ADR-022](022-fts5-full-text-search.md) / [ADR-023](023-declarative-query-indexes.md)
  — `searchable` / `indexed`, classified as additive (safe) metadata.
- `docs/guides/schema-lockdown.md` — the customer onboarding + CI/local
  recipes. Issue #488 — the original `entdb-schema` design.
