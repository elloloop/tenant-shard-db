# Shared Port Spec — Schema Registry

EPIC #407 — Python → Go server port. Source of truth:
`server/python/entdb_server/schema/` (the whole package). Behavioural pins:
`(legacy Python unit test, removed in Phase 4D)`, `test_schema_validator.py`,
`test_compat.py`, `test_composite_unique_schema.py`,
`test_schema_cli_e2e.py`. The registry is a *server-wide singleton*,
populated at boot, frozen before serving — every privileged RPC under
`docs/go-port/rpcs/` consults it for type IDs, field IDs, validation, and
data-policy decisions.

CLAUDE.md invariant #5: **proto is the type system.** The Go port MUST
populate the registry from the same proto descriptors the Python side
already uses — no parallel codegen, no hand-maintained type tables.

## Concepts

`NodeTypeDef` (`schema/types.py:427-642`) is a stable type definition with:

- `type_id` (1..2^31, `:483-486`) — canonical identifier; never reused
  even after deprecation.
- `name` — label only; renames are free because storage is keyed by id.
- `fields` — tuple of `FieldDef`; duplicate `field_id` or duplicate name
  rejected at construction (`:491-498`).
- `data_policy` (`DataPolicy` enum, defaults `None` → `PERSONAL` at
  runtime, `registry.py:241-250`) — drives encryption tier and GDPR.
- `subject_field` — name of the field holding the data-subject id.
- `default_acl` — applied when a node is created without an explicit ACL.
- `composite_unique` — tuple of `CompositeUniqueDef`
  (`types.py:373-424`); each entry is `(name, (field_id, ...))` with
  ≥2 distinct field ids, names unique per type, no two composites
  sharing the same field set.

`FieldDef` (`types.py:143-309`) carries `field_id` (1..65535,
`:192-195`), `kind` (`FieldKind`: `str|int|float|bool|timestamp|json|`
`bytes|enum|ref|list_str|list_int|list_ref`), and four schema bits the
applier and query planner read directly: `required`, `unique`, `indexed`,
`searchable`. Plus `pii`, `deprecated`, `enum_values` (append-only,
`compat.py:573-584`), `ref_type_id`.

`EdgeTypeDef` (`types.py:644-798`) is unidirectional with
`from_type_id`/`to_type_id` (`:701-713`), `props` (own `FieldDef`s),
`unique_per_from`, `on_subject_exit` (`from|to|both`, drives GDPR edge
cleanup, `registry.py:371-389`), and its own `data_policy`.

Registry helpers the rest of the server depends on:

- `get_node_type(int|str)` / `get_edge_type(...)` — id-or-name lookup.
- `get_unique_field_ids(type_id)` (`registry.py:252-268`) — drives lazy
  `CREATE UNIQUE INDEX` in the applier (`docs/decisions/sdk_api.md`).
- `get_composite_unique_constraints(type_id)` (`:270-290`) — same, multi-
  field.
- `get_indexed_field_ids(type_id)` (`:292-309`) — non-unique indexes;
  `unique` is a superset of `indexed`, so unique fields are excluded
  here.
- `get_searchable_field_ids(type_id)` (`:311-331`) — only `STRING` kind;
  feeds FTS5 index creation.
- `get_pii_fields(type_id)`, `get_data_policy(type_id)`,
  `get_subject_field(type_id)` — GDPR / compliance.

## Lifecycle

The registry is a **process-wide singleton**, mutable during boot,
frozen exactly once before serving. There is **no per-tenant variant**;
all tenants share one schema (`registry.py:46-48,554-585`).

Boot sequence as Python runs it today (`server/go/cmd/entdb-server/main.go`):

1. `registry = get_registry()` — lazily creates the global on first call.
2. Optional `_load_schema_file(registry, path)` — reads a JSON file
   (the format produced by `SchemaRegistry.to_json`, `:479-488`) and
   calls `register_node_type` / `register_edge_type` for each entry.
   This is how the SDK-emitted schema (via
   `entdb_sdk.codegen.register_proto_schema`,
   `sdk/python/entdb_sdk/codegen.py:431-510`) reaches the server in
   environments where the server doesn't import the proto module
   directly.
3. `freeze_registry()` (`registry.py:573-585`) — computes a SHA-256
   fingerprint over the canonical `to_dict()` (`:448-461`) and flips
   `_frozen = True`. After freeze, `register_node_type` raises
   `RegistryFrozenError` (`:122-125`).

Compatibility ratchet: `tools/schema_cli.py` runs
`check_compatibility(old, new)` (`schema/compat.py:144-188`) against a
checked-in baseline JSON before deployment. Breaking changes
(`ChangeKind.is_breaking`, `compat.py:78-97`) — field removal, kind
change, id reuse, enum value removal/reorder, `from_type` change,
making an optional field required, `propagate_share` flip — fail CI.
Allowed: add type, add field, add enum value, deprecate, rename.

Note: a top-level `.schema-snapshot.json` does not currently exist in
the repo; the CLI accepts an explicit `--baseline` path
(`schema_cli.py:179-181`). The Go port should adopt the same convention
and wire the same baseline into CI.

## Ingress consumers

Three consumers read the registry on the hot path; the Go port must
preserve all three contracts.

**Payload translation at the gRPC boundary.** Storage is keyed by
stringified `field_id` (CLAUDE.md invariant #6). `name_to_id_keys` /
`id_to_name_keys` (`schema/field_id_translation.py:1-46`) translate
ingress and egress payloads using the registry's per-type field map.
Unknown names on ingress are dropped silently; unknown ids on egress
are kept as-is (forward compat). See the dedicated payload-translation
spec for details.

**Validation on write.** `NodeTypeDef.validate_payload`
(`types.py:557-581`) and per-field `FieldDef.validate_value`
(`:203-261`) check required, kind, enum membership, and reference
shape before the WAL append in the gRPC servicer. The applier later
re-reads the registry to discover unique/composite-unique/indexed/
searchable field ids and to drive lazy `CREATE INDEX` statements
(`apply/applier.py:939,1046,1107,1129,1560`).

**Query planning.** `SearchNodes` / `QueryNodes` consult
`get_indexed_field_ids` / `get_searchable_field_ids` to decide whether
a filter is index-backed or a full scan. `GetNodeByKey` resolves a
typed `UniqueKey[T]` token (carrying `type_id`, `field_id`) against the
registry's unique-index for the type.

## Go design

Layout: `server/go/internal/schema/`. Single source of truth shared
across the server.

```
server/go/internal/schema/
  types.go         — NodeTypeDef, EdgeTypeDef, FieldDef, FieldKind, OnSubjectExit, AclEntry, CompositeUniqueDef
  registry.go      — Registry struct, Get/Register/Freeze, helper accessors
  loader.go        — LoadFromJSON, LoadFromDescriptors (proto files)
  compat.go        — Check(old, new) → []Change; CompatibilityError
  fingerprint.go   — Fingerprint(reg) string ("sha256:…")
  translate.go     — NameToID / IDToName payload-key translation
```

Proposed shape:

```go
type Registry struct {
    nodes       map[int32]*NodeTypeDef
    nodesByName map[string]*NodeTypeDef
    edges       map[int32]*EdgeTypeDef
    edgesByName map[string]*EdgeTypeDef
    frozen      atomic.Bool
    fingerprint string
    mu          sync.Mutex // only held during registration
}

func (r *Registry) RegisterNode(t *NodeTypeDef) error
func (r *Registry) RegisterEdge(t *EdgeTypeDef) error
func (r *Registry) Freeze() (string, error)
func (r *Registry) Node(idOrName any) *NodeTypeDef
func (r *Registry) Edge(idOrName any) *EdgeTypeDef
func (r *Registry) UniqueFieldIDs(typeID int32) []int32
func (r *Registry) CompositeUnique(typeID int32) []CompositeUniqueDef
func (r *Registry) IndexedFieldIDs(typeID int32) []int32
func (r *Registry) SearchableFieldIDs(typeID int32) []int32
func (r *Registry) PIIFields(typeID int32) []string
func (r *Registry) DataPolicy(typeID int32) DataPolicy
func (r *Registry) SubjectField(typeID int32) string
```

After `Freeze()` returns, all read methods are lock-free (mirrors
`registry.py:69-72`). The frozen guarantee lets the gRPC handlers and
applier hold a `*Registry` reference without further synchronization.

Population path: at boot, `loader.LoadFromDescriptors` walks
`protoreflect.FileDescriptor`s and reads the `entdb.node` / `entdb.field`
options via `protoc-gen-go`-generated extension accessors —
the same proto annotations Python re-parses in
`sdk/python/entdb_sdk/codegen.py:475-510`. **Do not** re-implement
codegen (CLAUDE.md #5). Alternative: `loader.LoadFromJSON` accepts the
existing `to_dict()` JSON shape verbatim, so a Python-emitted schema
file boots a Go server unchanged — this is the cross-language contract
test.

A `DefaultRegistry()` package-level accessor mirrors Python's
`get_registry()` for ergonomics, but production code paths take
`*Registry` as a constructor argument (so tests can inject fixtures
without touching globals).

## Dependencies

- `proto/entdb/v1/entdb.proto` — `Node`, `Edge`, `GetSchemaResponse`,
  `Struct` (carrying field-id-keyed payloads).
- `proto/entdb_options.proto` — `(entdb.node)`, `(entdb.edge)`,
  `(entdb.field)` extensions; the registry reads these to populate
  `NodeTypeDef`/`FieldDef`.
- `internal/errs` — `ErrSchemaFrozen`, `ErrDuplicateRegistration`,
  `ErrUnknownType`, `ErrSchemaIncompatible` (mapped to gRPC
  `FAILED_PRECONDITION` / `INVALID_ARGUMENT`).
- `internal/datapolicy` — `DataPolicy` enum, used by encryption and
  GDPR layers.

The registry has **no** dependency on the WAL, SQLite, or any I/O. It
is pure data + lookup.

## RPCs

- **GetSchema** (`docs/go-port/rpcs/GetSchema.md`) — serializes the
  registry to the wire `GetSchemaResponse`. Read-only; returns the
  fingerprint so clients can detect drift.
- **ExecuteAtomic** — every op in the plan validates against the
  registry (kind, required, enum, ref shape) before the WAL append.
  Reject early with `INVALID_ARGUMENT`; do not let bad payloads reach
  the applier.
- **GetNodeByKey** — resolves the `(type_id, field_id, value)` triple
  via the registry's unique-field bookkeeping; falls back to
  `NOT_FOUND` if the type has no unique field with that id.
- **SearchNodes / QueryNodes** — consult `IndexedFieldIDs` and
  `SearchableFieldIDs` to pick an index-backed plan vs a full scan;
  also rely on `Node(typeID).GetField(name)` for filter-name → field-id
  rewriting when a client sends a name-keyed filter.
- **CreateUser / CreateTenant / admin RPCs** — use `DataPolicy` and
  `SubjectField` to label new nodes for GDPR tracking.

The interceptor stack itself is registry-independent — auth runs first
(`docs/go-port/shared/auth-interceptor.md`), then the handler validates
against the schema.

## Test surface

Go-side parity tests, all reading the same Python-emitted JSON
(`SchemaRegistry.to_json`) so the two implementations cannot drift:

- `internal/schema/registry_test.go` — register / get / freeze /
  duplicate-id / frozen-write rejection (mirrors
  `(legacy Python unit test, removed in Phase 4D)`).
- `internal/schema/compat_test.go` — every `ChangeKind` enum case from
  `compat.py:45-97` produces the expected breaking/non-breaking
  classification (mirrors `test_compat.py`).
- `internal/schema/composite_test.go` — composite-unique constraint
  validation, name uniqueness, ≥2 fields rule (mirrors
  `test_composite_unique_schema.py`).
- `internal/schema/fingerprint_test.go` — Go fingerprint matches
  Python fingerprint byte-for-byte for the same registry contents.
  This is the single most important parity check; CI runs both
  implementations over a shared fixture set and compares.
- `tests/contract/schema_parity_test.go` — load the in-tree proto
  files via `protoreflect`, register via the Go loader, register via
  Python `register_proto_schema`, diff `to_dict()` outputs.

## Open questions / risks

1. **Runtime schema mutation.** Python freezes once at boot and never
   re-opens. The Go port should preserve this — no hot reload, no
   per-RPC override. If a future requirement needs mutable schema, it
   must come with an `RFC` and a migration story for the WAL replay
   path (the applier assumes the registry is stable for the lifetime
   of the process).
2. **Per-tenant schema variants.** Out of scope. The current model is
   one schema per cluster; multi-tenant schema isolation would
   complicate the WAL (events would need a schema fingerprint header)
   and the applier (per-tenant index sets). Flag this for the post-port
   roadmap; do not attempt during the rewrite.
3. **Compatibility ratchet location.** Python's CLI loads an explicit
   `--baseline` path; there is no committed `.schema-snapshot.json`
   today. The Go port should standardise the baseline location
   (probably `proto/.schema-snapshot.json`) and ensure the
   `release.yml` workflow regenerates it on every tag.
4. **Descriptor option re-parsing.** Python serialises `MessageOptions`
   then re-parses through the entdb extension surface
   (`sdk/python/entdb_sdk/codegen.py:475-485`) to recover the typed
   `NodeOpts`/`EdgeOpts`. Go's `proto.GetExtension` does this natively
   — verify the Python-emitted JSON and the Go-emitted JSON match
   bit-for-bit on the same `.proto` source before declaring parity.
5. **Field-id translation drop semantics.** Ingress drops unknown
   names silently (`field_id_translation.py:21-23`). The Go port must
   match — emitting a warning is fine, raising is not. Document this
   explicitly because it is surprising.
