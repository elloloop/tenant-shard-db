// Package payload is the gRPC-boundary translator between id-keyed
// google.protobuf.Struct payloads (the wire) and id-keyed map[uint32]any
// payloads (storage / WAL events).
//
// NAME-FREE per ADR-031 (and CLAUDE.md invariant #6, "field IDs not
// names on disk"): the server never stores, transmits, or translates a
// field name. Wire payloads, filters, and CAS preconditions are keyed by
// field_id (decimal-string keys, e.g. {"1": value}); the SDK resolves
// names client-side from the proto descriptor. The translation that
// remains here is purely field-KIND coercion (BYTES <-> base64,
// TIMESTAMP/INTEGER number narrowing) driven by the registry looked up
// by type_id.
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
// NAME-FREE per ADR-031: the wire payload MUST be keyed by stringified
// field_ids ("1", "2", ...). The SDK resolves names to field_ids
// client-side from the proto descriptor (fd.Number() IS the field_id per
// ADR-006). Non-digit (name) keys are rejected with INVALID_ARGUMENT —
// the server never translates a name.
//
// Behavior:
//
//   - nil / empty s -> empty map (never nil) so downstream JSON
//     marshalling stays deterministic.
//   - A non-digit key is INVALID_ARGUMENT (the SDK is misconfigured or
//     out of date).
//   - A digit key matching a known field_id (when reg+typeID resolve a
//     type) is kind-coerced; a digit key not known on the type is kept
//     as-is (ingress is permissive — a future-schema field round-trips).
//   - Schema-less (reg == nil OR typeID unknown): digit keys pass
//     through with no kind coercion.
//   - Field-kind coercion (only when the type is known):
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
func StructToPayload(reg *schema.Registry, typeID int32, s *structpb.Struct) (map[uint32]any, error) {
	if s == nil || len(s.GetFields()) == 0 {
		return map[uint32]any{}, nil
	}

	nt := lookupNodeType(reg, typeID)

	out := make(map[uint32]any, len(s.GetFields()))
	for key, value := range s.GetFields() {
		id, ok := parseFieldID(key)
		if !ok {
			// Name-free invariant: a non-digit key means the SDK did not
			// translate names to field_ids. Reject rather than silently
			// drop so the misconfiguration surfaces immediately.
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload contains non-field_id key %q; wire payload must be "+
					"id-keyed (e.g. {\"1\": value}) — the SDK pre-translates "+
					"names to field_ids client-side (ADR-031 / CLAUDE.md "+
					"invariant #6).", key)
		}
		if nt != nil {
			if f := nt.GetFieldByID(id); f != nil {
				coerced, err := coerceForKind(f, value)
				if err != nil {
					return nil, err
				}
				out[id] = coerced
				continue
			}
			// Digit but not a known id: keep as-is (forward-compat — a
			// field from a newer schema revision is preserved verbatim).
			out[id] = value.AsInterface()
			continue
		}
		// Schema-less: id-keyed passthrough, no kind coercion.
		out[id] = value.AsInterface()
	}
	return out, nil
}

// PayloadToStruct wraps an id-keyed payload for the wire.
//
// The wire stays id-keyed (zero-translation egress). typeID is consumed
// only for kind-aware coercion in the reverse direction:
//
//   - BYTES: storage holds []byte (or pre-base64 string); structpb has
//     no bytes type, so we encode to base64 string on the wire.
//   - TIMESTAMP / INTEGER: storage holds int64; structpb represents
//     numbers as float64.
//
// When typeID == 0 or the registry has no entry, the egress is a pure
// passthrough — values go through structpb.NewValue as-is.
//
// The returned Struct has stringified field_id keys ("1", "2", ...).
func PayloadToStruct(reg *schema.Registry, typeID int32, p map[uint32]any) (*structpb.Struct, error) {
	out := &structpb.Struct{Fields: make(map[string]*structpb.Value, len(p))}
	if len(p) == 0 {
		return out, nil
	}

	nt := lookupNodeType(reg, typeID)

	for id, raw := range p {
		key := strconv.FormatUint(uint64(id), 10)

		var v *structpb.Value
		var err error
		if nt != nil {
			if f := nt.GetFieldByID(id); f != nil {
				v, err = encodeForKind(f, raw)
			} else {
				// Unknown id on egress is preserved verbatim
				// (forward-compat).
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

// FilterToIDs normalises a filter dict (used by QueryNodes /
// delete_where) into the id-keyed form the canonical store expects.
//
// NAME-FREE per ADR-031: filter keys are field_ids (decimal strings).
// A non-digit (name) key is INVALID_ARGUMENT — the server never resolves
// a name. A digit key that the schema does not recognise (when a type is
// resolved) is INVALID_ARGUMENT, because filtering on a field that does
// not exist would silently change the result set.
//
// The input is map[string]any, not *structpb.Struct (the gRPC layer
// unwraps a Struct before calling).
func FilterToIDs(reg *schema.Registry, typeID int32, filter map[string]any) (map[uint32]any, error) {
	if len(filter) == 0 {
		return map[uint32]any{}, nil
	}

	nt := lookupNodeType(reg, typeID)
	out := make(map[uint32]any, len(filter))
	for key, value := range filter {
		id, ok := parseFieldID(key)
		if !ok {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: filter key %q is not a field_id; filters are "+
					"id-keyed (ADR-031 — the SDK pre-translates names)", key)
		}
		if nt != nil && nt.GetFieldByID(id) == nil {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: filter references unknown field_id %d on type_id %d", id, typeID)
		}
		out[id] = value
	}
	return out, nil
}

// --- internal helpers ---------------------------------------------------

// lookupNodeType is the single resolver this package uses. typeID == 0 or
// a nil registry returns nil (schema-less). Name-free per ADR-031:
// resolution is by type_id only.
func lookupNodeType(reg *schema.Registry, typeID int32) *schema.NodeTypeDef {
	if reg == nil || typeID == 0 {
		return nil
	}
	return reg.NodeTypeByID(typeID)
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
// present, value null" from "key absent" (the applier silently no-ops
// on null).
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
				"payload: field_id %d (bytes) requires base64 string, got %T",
				f.FieldID, v.Kind)
		}
		decoded, err := base64.StdEncoding.DecodeString(s.StringValue)
		if err != nil {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: field_id %d (bytes) is not valid base64: %v", f.FieldID, err)
		}
		return decoded, nil
	case schema.KindTimestamp, schema.KindInteger:
		n, ok := v.Kind.(*structpb.Value_NumberValue)
		if !ok {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: field_id %d (%s) requires a number, got %T",
				f.FieldID, f.Kind, v.Kind)
		}
		// structpb numbers are float64. Reject values outside the
		// safe-integer range (2^53) to keep replay deterministic.
		if n.NumberValue > 1<<53 || n.NumberValue < -(1<<53) {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: field_id %d (%s) value %v exceeds safe integer range",
				f.FieldID, f.Kind, n.NumberValue)
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
				"payload: field_id %d (enum) requires a string, got %T",
				f.FieldID, v.Kind)
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
				"payload: field_id %d (bytes) stored as %T, expected []byte or string",
				f.FieldID, raw)
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
