// Package payload is the gRPC-boundary translator between name-keyed
// google.protobuf.Struct payloads (the wire) and id-keyed map[uint32]any
// payloads (storage / WAL events).
//
// Per CLAUDE.md invariant #6 ("field IDs, not field names, on disk")
// translation happens at exactly two points: ingress in the gRPC handler
// layer (StructToPayload, FilterNamesToIDs) and egress that converts a
// stored id-keyed JSON blob back to names for SDKs that need it
// (PayloadJSONToNames). The egress to *structpb.Struct (PayloadToStruct)
// is a zero-translation wrap — the wire stays id-keyed and the SDK does
// the id→name lookup on the client side.
//
// Spec: docs/go-port/shared/payload-translation.md.
package payload

import (
	"encoding/base64"
	"strconv"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/structpb"
)

// StructToPayload translates a wire google.protobuf.Struct into the
// id-keyed internal payload representation map[uint32]any. Used on the
// ingress side of CreateNode / UpdateNode.
//
// Wire shape (per CLAUDE.md invariant #6):
// The wire payload is keyed by stringified field_ids ("1", "2", ...).
// The SDKs do the name→id translation client-side from the proto
// descriptor (where fd.Number() IS the field_id per ADR-006). Storage
// and WAL events deal exclusively with id-keyed maps; renames are
// metadata-only, so storage cannot key on names.
//
// Behavior:
//
//   - nil / empty s -> empty map (never nil) so downstream JSON
//     marshalling stays deterministic.
//   - reg == nil OR nodeTypeName == "" OR registry has no entry for
//     nodeTypeName -> schema-less ingress: only digit-string keys are
//     accepted. Name-keyed payloads on an unregistered type are
//     rejected with INVALID_ARGUMENT (rather than silently dropped)
//     so the client learns immediately that its SDK is misconfigured
//     or out of date.
//   - Schema-aware path:
//   - Digit-only key matching a known field_id is kept as-is.
//   - String key matching a field name is translated to its id
//     (back-compat for pre-fix SDKs; new SDKs send id-keyed and
//     skip this path).
//   - Unknown name keys are dropped silently (Python parity:
//     ingress is permissive so legacy clients sending deprecated
//     fields don't break writes).
//   - Field-kind coercion (only when schema is known):
//   - BYTES: structpb has no bytes type; the wire carries base64
//     strings. Decode to []byte for storage.
//   - TIMESTAMP / INTEGER: structpb represents numbers as float64.
//     Coerce to int64 on ingress so disk values are integers.
//   - JSON: top-level passthrough. Nested *structpb.Struct values
//     are converted to map[string]any once at this layer; the
//     translator never recurses into them.
//
// Order independence is a property of map iteration; do not rely on
// Struct.Fields insertion order anywhere downstream.
func StructToPayload(reg *schema.Registry, nodeTypeName string, s *structpb.Struct) (map[uint32]any, error) {
	if s == nil || len(s.GetFields()) == 0 {
		return map[uint32]any{}, nil
	}

	// Schema-less ingress: the wire is required to be id-keyed because
	// without a registered schema the server has no name→id mapping.
	// Silently dropping name keys (the prior behavior) caused silent
	// data loss for SDKs that emitted name-keyed payloads.
	nt := lookupNodeType(reg, nodeTypeName)
	if nt == nil {
		out := make(map[uint32]any, len(s.GetFields()))
		var firstName string
		for k, v := range s.GetFields() {
			if id, ok := parseFieldID(k); ok {
				out[id] = v.AsInterface()
				continue
			}
			if firstName == "" {
				firstName = k
			}
		}
		if firstName != "" {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload for type_id with name %q (not in schema registry) "+
					"contains name-keyed field %q; wire payload must be "+
					"id-keyed (e.g. {\"1\": value}). Either register the "+
					"schema for this type or upgrade the SDK to one that "+
					"pre-translates names to field_ids client-side "+
					"(CLAUDE.md invariant #6).",
				nodeTypeName, firstName)
		}
		return out, nil
	}

	// Build name->FieldDef and id->FieldDef indices once per call. The
	// registry caches these post-Freeze for the hot path; we keep our
	// own copy here so the kind-coercion lookup is O(1).
	nameToField := make(map[string]*schema.FieldDef, len(nt.Fields))
	idToField := make(map[uint32]*schema.FieldDef, len(nt.Fields))
	for i := range nt.Fields {
		f := &nt.Fields[i]
		nameToField[f.Name] = f
		idToField[f.FieldID] = f
	}

	out := make(map[uint32]any, len(s.GetFields()))
	for key, value := range s.GetFields() {
		// Pre-translated digit keys: a caller (e.g. typed Go SDK) may
		// already have done id translation. If the digit matches a
		// known field id, keep it; otherwise the key is unknown on
		// this schema and is dropped.
		if id, ok := parseFieldID(key); ok {
			if f := idToField[id]; f != nil {
				coerced, err := coerceForKind(f, value)
				if err != nil {
					return nil, err
				}
				out[id] = coerced
				continue
			}
			// Digit but not a known id: drop silently (forward-compat
			// scenarios are handled on egress, not ingress).
			continue
		}

		f := nameToField[key]
		if f == nil {
			// Unknown name: drop silently. Matches Python
			// name_to_id_keys (ingress is permissive).
			continue
		}
		coerced, err := coerceForKind(f, value)
		if err != nil {
			return nil, err
		}
		out[f.FieldID] = coerced
	}
	return out, nil
}

// PayloadToStruct wraps an id-keyed payload for the wire.
//
// The wire stays id-keyed (per the spec's "zero-translation egress"
// rule). nodeTypeName is consumed only for kind-aware coercion in the
// reverse direction:
//
//   - BYTES: storage holds []byte (or pre-base64 string); structpb has
//     no bytes type, so we encode to base64 string on the wire.
//   - TIMESTAMP / INTEGER: storage holds int64; structpb represents
//     numbers as float64.
//
// When nodeTypeName == "" or the registry has no entry, the egress is a
// pure passthrough — values go through structpb.NewValue as-is.
//
// The returned Struct has stringified field_id keys ("1", "2", ...).
func PayloadToStruct(reg *schema.Registry, nodeTypeName string, p map[uint32]any) (*structpb.Struct, error) {
	out := &structpb.Struct{Fields: make(map[string]*structpb.Value, len(p))}
	if len(p) == 0 {
		return out, nil
	}

	nt := lookupNodeType(reg, nodeTypeName)

	for id, raw := range p {
		key := strconv.FormatUint(uint64(id), 10)

		var v *structpb.Value
		var err error
		if nt != nil {
			if f := nt.GetFieldByID(id); f != nil {
				v, err = encodeForKind(f, raw)
			} else {
				// Unknown id on egress is preserved verbatim
				// (forward-compat) — matches Python id_to_name_keys.
				v, err = newValue(raw)
			}
		} else {
			v, err = newValue(raw)
		}
		if err != nil {
			return nil, err
		}
		out.Fields[key] = v
	}
	return out, nil
}

// FilterNamesToIDs translates a name-keyed filter dict (used by
// QueryNodes) into the id-keyed form the canonical store expects.
//
// Differs from StructToPayload in two ways:
//
//   - The input is map[string]any, not *structpb.Struct (the gRPC layer
//     unwraps a Struct before calling).
//   - Unknown names are an error, not a silent drop. A QueryNodes
//     filter mentioning a field that does not exist on the schema is a
//     client bug and should surface as INVALID_ARGUMENT — silently
//     dropping it would change the result set.
//
// Mirrors translate_filter_name_to_id in the Python port, with the
// stricter unknown-key behavior the Go server enforces (see
// docs/go-port/shared/payload-translation.md "QueryNodes filter").
func FilterNamesToIDs(reg *schema.Registry, nodeTypeName string, filter map[string]any) (map[uint32]any, error) {
	if len(filter) == 0 {
		return map[uint32]any{}, nil
	}

	nt := lookupNodeType(reg, nodeTypeName)
	if nt == nil {
		// Schema-less: digit-only keys parse to uint32; non-digit keys
		// are an error because we cannot resolve them without a schema.
		out := make(map[uint32]any, len(filter))
		for k, v := range filter {
			id, ok := parseFieldID(k)
			if !ok {
				return nil, errs.Errorf(codes.InvalidArgument,
					"payload: cannot translate filter key %q without a schema", k)
			}
			out[id] = v
		}
		return out, nil
	}

	out := make(map[uint32]any, len(filter))
	for key, value := range filter {
		if id, ok := parseFieldID(key); ok {
			if nt.GetFieldByID(id) == nil {
				return nil, errs.Errorf(codes.InvalidArgument,
					"payload: filter references unknown field_id %d on type %q", id, nodeTypeName)
			}
			out[id] = value
			continue
		}
		f := nt.GetField(key)
		if f == nil {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: filter references unknown field %q on type %q", key, nodeTypeName)
		}
		out[f.FieldID] = value
	}
	return out, nil
}

// PayloadJSONToNames converts an id-keyed payload (e.g. as fetched from
// the canonical store) to a name-keyed map[string]any suitable for
// JSON-friendly egress paths (audit exports, the console JSON view).
//
// Unknown ids are preserved verbatim (as their decimal string) so a
// row written by a future schema revision is not lossy on read.
// Unknown ids preserved as decimal-string keys is identical to the
// Python id_to_name_keys fallthrough.
//
// When reg / nodeTypeName resolves to no schema, every id is rendered
// as its decimal string.
func PayloadJSONToNames(reg *schema.Registry, nodeTypeName string, p map[uint32]any) (map[string]any, error) {
	out := make(map[string]any, len(p))
	if len(p) == 0 {
		return out, nil
	}

	nt := lookupNodeType(reg, nodeTypeName)
	for id, value := range p {
		var key string
		if nt != nil {
			if f := nt.GetFieldByID(id); f != nil {
				key = f.Name
			} else {
				key = strconv.FormatUint(uint64(id), 10)
			}
		} else {
			key = strconv.FormatUint(uint64(id), 10)
		}
		out[key] = value
	}
	return out, nil
}

// --- internal helpers ---------------------------------------------------

// lookupNodeType is the single resolver this package uses. Empty
// nodeTypeName or nil registry returns nil (schema-less).
func lookupNodeType(reg *schema.Registry, nodeTypeName string) *schema.NodeTypeDef {
	if reg == nil || nodeTypeName == "" {
		return nil
	}
	return reg.NodeTypeByName(nodeTypeName)
}

// parseFieldID parses a digit-only string into a uint32 within the
// FieldDef bound (1-65535). Returns false for empty / non-digit / 0 /
// out-of-range. Mirrors the field_id invariant on FieldDef.Validate.
func parseFieldID(s string) (uint32, bool) {
	if s == "" {
		return 0, false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil || n == 0 || n > 65535 {
		return 0, false
	}
	return uint32(n), true
}

// coerceForKind converts a *structpb.Value into the canonical Go type
// for a field's Kind. Returns the raw AsInterface() result for kinds
// that need no coercion. Returns INVALID_ARGUMENT on a structurally
// wrong wire value (e.g. BYTES field carrying a non-string).
//
// Nil or NullValue is preserved as nil so callers can distinguish "key
// present, value null" from "key absent" — matches the Python contract
// where FieldDef.validate_value treats None as "not provided" (and the
// applier silently no-ops on it).
func coerceForKind(f *schema.FieldDef, v *structpb.Value) (any, error) {
	if v == nil {
		return nil, nil
	}
	if _, ok := v.Kind.(*structpb.Value_NullValue); ok {
		return nil, nil
	}
	switch f.Kind {
	case schema.KindBytes:
		s, ok := v.Kind.(*structpb.Value_StringValue)
		if !ok {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: field %q (bytes) requires base64 string, got %T",
				f.Name, v.Kind)
		}
		decoded, err := base64.StdEncoding.DecodeString(s.StringValue)
		if err != nil {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: field %q (bytes) is not valid base64: %v", f.Name, err)
		}
		return decoded, nil
	case schema.KindTimestamp, schema.KindInteger:
		n, ok := v.Kind.(*structpb.Value_NumberValue)
		if !ok {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: field %q (%s) requires a number, got %T",
				f.Name, f.Kind, v.Kind)
		}
		// structpb numbers are float64. Reject values outside the
		// safe-integer range (2^53) to keep replay deterministic.
		if n.NumberValue > 1<<53 || n.NumberValue < -(1<<53) {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: field %q (%s) value %v exceeds safe integer range",
				f.Name, f.Kind, n.NumberValue)
		}
		return int64(n.NumberValue), nil
	case schema.KindJSON:
		// Top-level passthrough. *structpb.Struct unwraps to
		// map[string]any (recursively, but only at the value level —
		// the translator never recurses into nested struct keys).
		return v.AsInterface(), nil
	case schema.KindEnum:
		s, ok := v.Kind.(*structpb.Value_StringValue)
		if !ok {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: field %q (enum) requires a string, got %T",
				f.Name, v.Kind)
		}
		return s.StringValue, nil
	default:
		return v.AsInterface(), nil
	}
}

// encodeForKind is the egress counterpart of coerceForKind. Storage
// values for BYTES are []byte; structpb has no bytes type so we encode
// to base64 string. TIMESTAMP / INTEGER values may be int64 on disk
// but structpb represents numbers as float64; structpb.NewValue handles
// numeric conversion.
func encodeForKind(f *schema.FieldDef, raw any) (*structpb.Value, error) {
	if raw == nil {
		return structpb.NewNullValue(), nil
	}
	switch f.Kind {
	case schema.KindBytes:
		switch b := raw.(type) {
		case []byte:
			return structpb.NewStringValue(base64.StdEncoding.EncodeToString(b)), nil
		case string:
			// Already base64-encoded by an earlier write path; pass
			// through verbatim.
			return structpb.NewStringValue(b), nil
		default:
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: field %q (bytes) stored as %T, expected []byte or string",
				f.Name, raw)
		}
	default:
		return newValue(raw)
	}
}

// newValue is a shim around structpb.NewValue that accepts the int64 /
// uint32 / int / float forms the on-disk layer produces and returns a
// gRPC-status-bearing error on failure (rather than the raw protobuf
// error). Mirrors errs.* sentinels for the gRPC boundary.
func newValue(raw any) (*structpb.Value, error) {
	// structpb.NewValue is strict about input types — it does not
	// accept int64 directly (only float64, int, bool, string, []byte
	// without base64, []any, map[string]any, nil). Normalize the
	// numeric forms storage might use.
	switch n := raw.(type) {
	case int64:
		return structpb.NewNumberValue(float64(n)), nil
	case int32:
		return structpb.NewNumberValue(float64(n)), nil
	case uint32:
		return structpb.NewNumberValue(float64(n)), nil
	case uint64:
		return structpb.NewNumberValue(float64(n)), nil
	case []byte:
		// Schema-less egress: a raw []byte has no kind hint. Encode as
		// base64 so the value is JSON-safe; the SDK that wrote it must
		// know it's bytes.
		return structpb.NewStringValue(base64.StdEncoding.EncodeToString(n)), nil
	}
	v, err := structpb.NewValue(raw)
	if err != nil {
		return nil, errs.Errorf(codes.InvalidArgument,
			"payload: cannot encode value of type %T: %v", raw, err)
	}
	return v, nil
}
