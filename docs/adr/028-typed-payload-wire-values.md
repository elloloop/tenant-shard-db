# ADR-028: Typed, `field_id`-keyed payload wire values (retire `google.protobuf.Struct`)

**Status:** Accepted — design frozen 2026-05-23; **implementation pending.**
Characterization tests are already landed (RED, skipped) pinning the
post-fix contract:
`server/go/internal/payload/translate_test.go::TestPayload_Int64Spectrum_BugC`
and `sdk/go/entdb/wire_test.go::TestPayloadRoundTrip_Int64Spectrum_BugC`.
**Decided:** 2026-05-23
**Tags:** wire-format, payload, proto, schema, data-integrity, migration
**Complements:** [ADR-006](006-proto-schema-definition.md) (proto is the
type system) and [ADR-018](018-field-id-keyed-payloads.md) (payloads keyed
by `field_id`) — this ADR changes the *value encoding* only; the
`field_id` keying and rename-free property of ADR-018 are preserved and
in fact strengthened (native numeric keys).
**Implementation:** pending — `proto/entdb/v1/entdb.proto` (payload fields),
`server/go/internal/payload/translate.go`, `sdk/go/entdb/wire.go`,
`sdk/python/entdb_sdk/client.py`. Both SDKs ship together.

## Decision

Node/edge payloads stop crossing the wire as `google.protobuf.Struct` and
instead use a **typed, `field_id`-keyed map**:

```proto
message EntValue {
  oneof v {
    int64                  int_value    = 1;  // INTEGER / TIMESTAMP — lossless
    double                 double_value = 2;  // DOUBLE
    bool                   bool_value   = 3;  // BOOLEAN
    string                 string_value = 4;  // STRING / ENUM
    bytes                  bytes_value  = 5;  // BYTES — no base64-in-a-string
    google.protobuf.Struct json_value   = 6;  // JSON (genuinely dynamic)
  }
  // absence of `v` == explicit null.
}

// field_id -> typed value. The map key IS the proto field number / field_id
// (ADR-018), now native uint32 instead of a stringified JSON key.
map<uint32, EntValue> typed_data = 8;   // CreateNodeOp / Node
map<uint32, EntValue> typed_patch = 9;  // UpdateNodeOp
// ... symmetric additions for edge props.
```

Integer payload values are carried as a real `int64`, so the full int64
range round-trips exactly. The server's safe-integer guard
(`translate.go:347`, reject `> 2^53`) and the float64 guess-and-coerce
dance (`coerceForKind` / `encodeForKind`) are **removed** — the wire no
longer lies about the type, so the server no longer has to reconstruct it.

At rest, the applier MUST persist integer values losslessly (store as a
JSON integer / `json.Number`, never a float64) so `payload_json` keeps the
exact value the typed wire delivered. Concretely this requires a SHARED
canonical-number decode (`json.Decoder` + `UseNumber`, normalizing
`json.Number` → `int64` if integral else `float64`) applied CONSISTENTLY
at every JSON boundary — `wal.DecodeEvent`, the `applyUpdateNode` merge
read, and every payload_json egress decode — because `applyUpdateNode`
CAS does `reflect.DeepEqual(store-decoded, event-decoded)` and any
representation mismatch breaks CAS / DeleteWhere / FTS.

**Scope is not just stored payloads.** The same `EntValue` (or an
int64-bearing form) MUST also carry the scalar wire-value fields that are
`google.protobuf.Value`/`Struct` today, or int64 corrupts in the
query/lookup/CAS paths (issue #572): `FieldFilter.value` (QueryNodes),
`GetNodeByKeyRequest.value`, and `UpdateNodePrecondition.equals` — plus
the SDKs' `toProtoValue`/filter encoders, which must stop routing int64
through `float64`. Server-side reads of these fields must not collapse to
`float64` via `AsInterface()`.

## Context

Bug C: payloads cross the wire as `google.protobuf.Struct`
(`entdb.proto:249` `data = 7`, plus `patch`, edge props). `Struct` numbers
are IEEE-754 **doubles** by the protobuf spec, so any int64 above 2^53
corrupts **at the wire boundary**, in both SDKs and the server:

- `2^53 + 1` silently rounds to `2^53` and slips *past* the
  reject-`> 2^53` guard, so it is stored corrupted;
- larger magnitudes (`10^16+1`, `2^62+1`, `MaxInt64`) are rejected
  outright — a value the client legitimately wrote cannot be stored;
- the schema-less / JSON path has no guard at all, so the whole range
  corrupts silently (the customer's spectrum probe).

A standard database carries `BIGINT` as a genuine 64-bit integer because
the type is known and the protocol is typed; none routes it through a
float. EntDB already claims this end-to-end (ADR-006/018) — the defect is
that the wire chose a schemaless, double-backed envelope for
statically-typed data. No SDK-only fix is possible; the carrier is wrong.

## Migration (dual-read / dual-write; both SDKs ship together)

Breaking wire changes are forbidden to land abruptly, so:

1. **Additive proto:** add `typed_data` / `typed_patch` (new field numbers)
   alongside the existing `Struct data = 7` / `patch`, which are marked
   `deprecated` but kept.
2. **Server ingress (dual-read):** prefer `typed_data` when present;
   otherwise fall back to the legacy `Struct data`. A request MUST NOT set
   both for the same op (reject `INVALID_ARGUMENT` if it does).
3. **Server egress (dual-write, transition window):** populate `typed_data`
   always; also populate the legacy `Struct data` so pre-upgrade SDKs keep
   reading — EXCEPT integer values outside the float64-safe range, which
   the legacy field cannot represent and which are therefore only present
   in `typed_data` (an old SDK reading such a node sees the legacy field
   absent/clamped; documented as the known limitation that motivated the
   upgrade).
4. **SDKs:** send `typed_data`; on read, prefer `typed_data`, fall back to
   the legacy field for old servers. Python + Go in the same PR
   (per the repo's SDK-parity rule).
5. **Deprecation:** after a release window with both SDKs published and
   servers upgraded, drop the legacy `Struct` payload fields in a
   subsequent major.

## Alternatives considered

- **String-encode integer fields inside the existing `Struct`** (protobuf's
  int64-as-JSON-string convention). Rejected — a short fix: stringly-typed,
  schema-dependent (the server must know each field is INTEGER to know to
  parse it), and it *breaks schema-less numeric-field-id mode* (can't tell
  an int field from a genuine string field without a schema). Violates the
  "no short fixes" bar.
- **Raise the safe-integer guard / accept the float64 loss.** Rejected —
  it is silent data corruption; raising a threshold doesn't make doubles
  hold int64.
- **Keep `Struct`, add a parallel typed sidecar map only for ints.**
  Rejected — two carriers for one concept; doubles the test matrix and the
  ingress ambiguity ADR-018 already warned about.
- **Separate `field_id` registry decoupled from proto numbers.** Already
  rejected by ADR-018; unchanged here.

## Consequences

**Locks in:**
- The payload value carrier is a typed `oneof`; `field_id` keys are native
  `uint32` (ADR-018 keying preserved, stringification dropped).
- INTEGER/TIMESTAMP fields support the full int64 range, losslessly, end to
  end — wire, applier, and `payload_json` at rest.
- The reject-`> 2^53` guard and the float64 coercion code are deleted.

**Makes easy:**
- Correct large integers (nanosecond timestamps, byte quotas, MaxInt64
  sentinels) — the class of value that silently corrupted before.
- Native `bytes` (drops the base64-in-a-string hack).
- Schema-less mode is now *lossless* for integers (the value is
  self-describing on the wire; no schema needed to preserve type).

**Makes harder / tradeoffs:**
- A breaking wire change with a real migration window and dual-read/write
  complexity (above). Worth it; this is a data-integrity defect.
- `json_value` (the JSON field kind) still uses `google.protobuf.Struct`
  and is therefore still double-backed *inside* a JSON blob. Accepted: JSON
  is dynamically typed by definition; callers who need a lossless int use
  an INTEGER field, not a JSON sub-value. Documented, not hidden.
- Hand-rolled clients must construct `EntValue` oneofs (already "use an
  SDK" per ADR-018).

**Failure modes:**
- A request sets both `typed_data` and legacy `data` for one op →
  `INVALID_ARGUMENT` (no silent precedence surprise).
- An old SDK reads a node whose integer exceeds the float64-safe range →
  the legacy `Struct` field cannot carry it; the value lives only in
  `typed_data`. This is the explicit incompatibility the upgrade resolves.

## References

- [ADR-006](006-proto-schema-definition.md) — proto as the type system;
  this ADR makes the wire obey it for values.
- [ADR-018](018-field-id-keyed-payloads.md) — `field_id` keying + rename-free
  property; preserved, with native numeric keys.
- [ADR-016](016-handlers-append-applier-writes.md) — applier is the only
  writer; it must persist int64 losslessly to `payload_json`.
- Bug C characterization tests (landed RED/skipped):
  `server/go/internal/payload/translate_test.go::TestPayload_Int64Spectrum_BugC`,
  `sdk/go/entdb/wire_test.go::TestPayloadRoundTrip_Int64Spectrum_BugC`.
- Related: [ADR-029](029-keyset-cursor-pagination.md) (the sibling
  read-contract fix frozen the same day).
