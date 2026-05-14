# Payload Translation Boundary — Go Port Spec

EPIC #407, Python→Go server port. Pins CLAUDE.md invariant #6: **payloads are
stored keyed by `field_id`**, not by name. Translation happens **only at the
gRPC boundary**. Storage, WAL events, and the applier deal exclusively with
id-keyed maps.

Reference (Python):
- `server/python/entdb_server/schema/field_id_translation.py:75` — `name_to_id_keys`
- `server/python/entdb_server/schema/field_id_translation.py:133` — `id_to_name_keys`
- `server/python/entdb_server/schema/field_id_translation.py:184` — `translate_payload_json_to_names`
- `server/python/entdb_server/schema/field_id_translation.py:215` — `translate_filter_name_to_id`
- `server/python/entdb_server/api/grpc_server.py:149` — `_struct_to_dict` / `_dict_to_struct`
- `server/python/entdb_server/api/grpc_server.py:846` — ingress translation in `_convert_operations`
- `server/python/entdb_server/api/grpc_server.py:1693` — `_node_to_proto` (egress, no translation)
- `tests/python/unit/test_grpc_wire_format.py` — pinned wire contract

---

## Encoding

Payloads on the wire are `google.protobuf.Struct`. On disk and in WAL events
they are `map[uint32]any` — the key is the field's stable numeric `field_id`
(positive `int`, ≤ 65535; see `server/python/entdb_server/schema/types.py:194`).

- **Key shape on the wire**: `Struct.fields` keyed by **numeric strings**
  (`"1"`, `"2"`). Per `_node_to_proto` the server returns the on-disk dict
  verbatim through `_dict_to_struct`; the keys are stringified `field_id`s.
  See `tests/python/unit/test_grpc_wire_format.py:172` (`assert keys == {"1", "2"}`).
- **Key shape on disk / in events**: prefer `map[uint32]any` in Go (no
  string round-trip). Stringify only at the protobuf boundary because
  `structpb.Struct` requires `map[string]*Value`.
- **Value types preserved**: string, int (as float64 — `Struct` has no int
  type; document this loss-of-precision caveat), float, bool, list, nested
  map, null. `bytes` and `timestamp` are encoded per `FieldKind` rules
  (`schema/types.py:54`): `bytes` → base64 string in JSON; `timestamp` →
  Unix milliseconds as integer.
- **Null / optional**: an absent field is omitted from the map entirely. An
  explicit JSON `null` is stored as `nil`; the schema validator treats it as
  "not provided" (see `FieldDef.validate_value`, `schema/types.py:212`).
- **Empty payload**: `Struct{}` round-trips to an empty `map[uint32]any{}`
  (and back). `tests/python/unit/test_grpc_wire_format.py:320` pins this.
- **Unknown field names**: dropped silently on ingress. `field_id_translation.py:122`.
- **Unknown field ids**: kept verbatim on egress so future-schema rows are
  not lossy. `field_id_translation.py:175`.

---

## Boundary

Translation happens at **exactly two points** inside the gRPC handler layer.
Nowhere else.

1. **Ingress** (`_convert_operations`, `grpc_server.py:830`): name-keyed
   `Struct` → `map[uint32]any` via `name_to_id_keys`. Applies to
   `create_node.data`, `update_node.patch`, and any filter dict on
   `QueryNodes` (`translate_filter_name_to_id`).
2. **Egress** (`_node_to_proto`, `grpc_server.py:1693`): the on-disk
   id-keyed payload is wrapped in a `Struct` **without translation**. The
   wire stays id-keyed; SDKs translate to names on the client side using
   their own registry.

Forbidden:
- The applier (`apply/applier.py`) MUST NOT translate. It receives id-keyed
  events and writes id-keyed rows.
- The canonical store (`apply/canonical_store.py`) MUST NOT translate.
  Persistence is `field_id` only.
- WAL serializers MUST NOT translate. Events on the WAL are id-keyed.

Rationale: translation is a function of `(payload, type_id, schema)`. The
schema lives in the registry, which is a request-time dependency; allowing
inner layers to translate would make WAL replay schema-version-dependent and
break invariant #1 (WAL is source of truth).

---

## Test contract

`tests/python/unit/test_grpc_wire_format.py` is the cross-implementation
contract. The Go server MUST satisfy every test (transcribed to Go test
infrastructure, same protocol-level assertions).

| Test | Pins |
|---|---|
| `test_node_payload_on_wire_is_id_keyed` (line 172) | `Node.payload.fields` keys are numeric strings (`"1"`, `"2"`); values match originally-supplied content. |
| `test_payload_with_id_keyed_input_round_trips` (line 202) | Partial payloads accepted (no-required-fields type). Absent fields are not synthesized on read. |
| `test_field_order_on_wire_does_not_matter` (line 228) | proto3 field-order independence — two requests with identical content but reversed `Struct.update` order yield byte-equivalent stored state. |
| `test_oversized_message_is_rejected` (line 293) | Request larger than the **default 4 MiB** receive cap returns `RESOURCE_EXHAUSTED`. The handler is never invoked. Production server raises this to 50 MiB but the protocol-default rejection MUST surface as `RESOURCE_EXHAUSTED`. |
| `test_empty_payload_is_tolerated` (line 320) | Empty `Struct` accepted for types with no required fields; round-trips to empty `Struct` on read. |
| `test_unknown_field_name_is_silently_dropped` (line 354) | Unknown name keys are dropped; only known field ids appear on the wire. |

Order independence is a property of proto3 encoding but the test guards
against any handler that silently depends on Go map iteration order or
`structpb` insertion order. The Go implementation must keep this property.

---

## Schema-driven translation

`field_id` is resolved from a name via the **per-tenant schema registry**.

- Python: `SchemaRegistry.get_node_type(type_id)` returns a `NodeTypeDef`
  whose `.fields` carry `(field_id, name, kind)` — see
  `schema/types.py:427`.
- Lookup table built per call: `name → field_id` from `node_type.fields`
  (`field_id_translation.py:107`).
- **Schema-less mode** (Go server, post v1.12.2): when the registry has
  no entry for `type_id`, only **id-keyed** payloads are accepted. A
  payload with any name-keyed entry is rejected with
  `INVALID_ARGUMENT`, since without a schema the server has no
  name→id mapping. This is the post-fix tightening of the Python-era
  silent-passthrough — silently dropping or persisting name-keyed
  fields on a schemaless type violated invariant #6. The SDKs do the
  name→id translation client-side from the proto descriptor (where
  `fd.Number()` IS the `field_id` by ADR-006), so the wire is
  always id-keyed regardless of whether the server has the schema.
- **Pre-translated keys**: a digit-only string that matches a known
  `field_id` is left alone, allowing typed clients (Go SDK) to send
  pre-translated payloads (`field_id_translation.py:114`).

The Go implementation MUST honor all three behaviors.

---

## Go design

Module: `server/go/internal/payload/`.

```go
package payload

import (
    "github.com/elloloop/tenant-shard-db/server/go/internal/schema"
    structpb "google.golang.org/protobuf/types/known/structpb"
)

// PayloadToStruct wraps an id-keyed map for the wire. No name lookup;
// keys are stringified field_ids verbatim. Used by egress.
func PayloadToStruct(typeID uint32, raw map[uint32]any) (*structpb.Struct, error)

// StructToPayload translates a name-keyed (or pre-id-keyed) Struct to the
// id-keyed internal representation. Unknown names dropped, schema-less
// types pass through. Used by ingress.
func StructToPayload(typeID uint32, s *structpb.Struct, reg schema.Registry) (map[uint32]any, error)

// FilterNamesToIDs translates a name-keyed filter dict for QueryNodes.
// Mirrors translate_filter_name_to_id.
func FilterNamesToIDs(typeID uint32, filter map[string]any, reg schema.Registry) (map[uint32]any, error)

// PayloadJSONToNames translates a stored id-keyed JSON blob back to
// name keys (egress JSON path; mirrors translate_payload_json_to_names).
func PayloadJSONToNames(typeID uint32, raw []byte, reg schema.Registry) ([]byte, error)
```

Notes:
- `PayloadToStruct` does **not** depend on the schema. Egress is a
  zero-translation wrap (the wire is id-keyed).
- `StructToPayload` depends on `schema.Registry`. When `reg.GetNodeType(typeID)`
  is nil → pass through. Digit-only keys matching known field ids are kept.
- Use `map[uint32]any` internally (not `map[string]any`). Convert at the
  `structpb` boundary only.
- `nil` / empty `Struct` → empty map (not `nil`) to keep downstream JSON
  marshalling deterministic (`{}` not `null`).

---

## Dependencies

- `google.golang.org/protobuf/types/known/structpb` — wire `Struct`.
- `server/go/internal/schema` — `Registry`, `NodeTypeDef`, `FieldDef`.
- `server/go/internal/errs` — typed errors (`ErrSchemaLess` is **not** an
  error; pass-through is the contract). `RESOURCE_EXHAUSTED` for oversized
  payloads is enforced by the gRPC server option, not by `payload`.
- No dependency on the applier, canonical store, or WAL packages — those
  are downstream and consume the id-keyed map directly.

---

## Test surface

`server/go/internal/payload/payload_test.go` (unit) and
`tests/contract/payload_translation_test.go` (cross-impl contract).

- **Round-trip property**: for any registered type and any name-keyed map
  whose keys are subsets of the schema, `StructToPayload ∘ PayloadToStruct`
  preserves the id-keyed map. Use `testing/quick` or `gopter`.
- **Drop-unknown property**: injecting a synthetic unknown key never
  changes the id-keyed output.
- **Order independence**: shuffle `Struct.Fields` insertion order; assert
  byte-equal id-keyed output.
- **Schema-less pass-through**: with `nil` `NodeTypeDef`, both functions
  are identity (modulo string↔uint32 key shape — only when keys are
  digits).
- **Unknown id preservation**: egress keeps unknown id keys verbatim.
- **Type fidelity**: cover string, int (note float64 round-trip), float,
  bool, list, nested map, null, base64 bytes, timestamp ms.
- **Wire-format mirror**: replay every assertion from
  `tests/python/unit/test_grpc_wire_format.py` against the Go server in
  the contract harness.

---

## Open questions / risks

1. **Unknown field_ids on egress.** Python keeps the raw key (forward
   compat). Go should match — but `map[uint32]any` cannot represent a
   "name-keyed legacy" key. Decision: store unknown ids in the same
   `map[uint32]any` (the id is still numeric); if a legacy *name-keyed*
   row is encountered (digits only fail), surface it via a separate
   `extras map[string]any` on the internal Node struct or run the
   migration helper at read time. Track in a follow-up issue.
2. **Nested `Struct` handling.** `FieldKind.JSON` (`schema/types.py:60`)
   accepts arbitrary nested objects. Translation MUST NOT recurse into
   nested structs — only top-level keys are translated. The Python impl
   only translates the outer dict; Go must do the same.
3. **`bytes` encoding.** `structpb.Value` has no bytes type. Python stores
   bytes as base64 strings inside JSON. Go must follow: encode `[]byte`
   as base64 `string_value` on the wire and decode on ingress when the
   schema field kind is `BYTES`. This is a **schema-aware decode** —
   document that `StructToPayload` uses field kind to coerce string →
   `[]byte` for `BYTES` fields.
4. **`timestamp` encoding.** Unix ms as integer (`schema/types.py:58`).
   `structpb` integer fidelity: values pass as `float64`; safe up to
   2^53. Document the cutoff; reject values outside it on ingress.
5. **Missing schema (chicken-and-egg on first write).** A tenant's first
   `RegisterSchema` op itself carries no payload, but subsequent ops
   before the schema is durably applied will hit
   `get_node_type → nil`. Since v1.12.2 the schema-less ingress
   accepts **only id-keyed payloads** (and rejects any name-keyed
   entry with `INVALID_ARGUMENT`); the SDK does the name→id
   translation client-side from the proto descriptor, so the wire
   is id-keyed regardless of the server's schema state. No
   migration helper is needed: rows stored during the
   schema-not-yet-applied window are already id-keyed and remain
   correct once the schema lands.
6. **Pre-translated digit keys colliding with names.** If a future schema
   names a field literally `"1"`, the digit-key passthrough heuristic
   (`field_id_translation.py:114`) becomes ambiguous. Document and
   forbid digit-only field names at registry validation in Go.
7. **`Struct.NullValue` vs absent key.** Python's `MessageToDict` drops
   `NullValue` to `None`; Go's `structpb.AsMap` returns `nil`. Confirm
   the Go path treats both the same: keep the key with `nil` value, do
   not synthesize, do not drop. Pin with a contract test.
