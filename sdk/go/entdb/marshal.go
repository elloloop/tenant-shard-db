package entdb

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"
)

// Extension field numbers from sdk/entdb_sdk/proto/entdb_options.proto.
//
// The Go SDK deliberately does NOT import the generated entdb_options
// package — doing so would create a cyclic dependency between the
// SDK and every generated schema package that uses these extensions.
// Instead we decode the raw wire format of the message's
// “MessageOptions“ to pull the type_id / edge_id out of the
// embedded NodeOpts / EdgeOpts message.
const (
	// (entdb.node) message option — NodeOpts at extension field 50100.
	extNodeOpts = 50100
	// (entdb.edge) message option — EdgeOpts at extension field 50101.
	extEdgeOpts = 50101

	// NodeOpts.type_id / EdgeOpts.edge_id are both proto field 1.
	nodeOptsTypeIDField = 1
	edgeOptsEdgeIDField = 1
)

// typeIDFromMessage reads the “(entdb.node).type_id“ from a
// message's proto descriptor. Returns an error if the message is not
// annotated as an EntDB node type.
func typeIDFromMessage(msg proto.Message) (int32, error) {
	if msg == nil {
		return 0, fmt.Errorf("entdb: nil message")
	}
	desc := msg.ProtoReflect().Descriptor()
	return extractInt32FromOptions(desc, extNodeOpts, nodeOptsTypeIDField, "node")
}

// edgeIDFromMessage reads the “(entdb.edge).edge_id“ from a
// message's proto descriptor. Returns an error if the message is not
// annotated as an EntDB edge type.
func edgeIDFromMessage(msg proto.Message) (int32, error) {
	if msg == nil {
		return 0, fmt.Errorf("entdb: nil message")
	}
	desc := msg.ProtoReflect().Descriptor()
	return extractInt32FromOptions(desc, extEdgeOpts, edgeOptsEdgeIDField, "edge")
}

// extractInt32FromOptions walks the raw-wire-format encoding of the
// message's “MessageOptions“ looking for extension “extNum“ (a
// length-prefixed submessage) and returns field “innerField“ (an
// int32 varint) from inside it. We do this by hand because importing
// the generated extension descriptor would pull the entdb_options
// package into every SDK user.
func extractInt32FromOptions(desc protoreflect.MessageDescriptor, extNum, innerField int32, kind string) (int32, error) {
	opts := desc.Options()
	if opts == nil {
		return 0, fmt.Errorf("entdb: message %q has no (entdb.%s) option", desc.FullName(), kind)
	}
	raw, err := proto.Marshal(opts)
	if err != nil {
		return 0, fmt.Errorf("entdb: marshal options for %q: %w", desc.FullName(), err)
	}
	inner, ok := findLengthDelimited(raw, uint64(extNum))
	if !ok {
		return 0, fmt.Errorf("entdb: message %q has no (entdb.%s) option", desc.FullName(), kind)
	}
	v, ok := findVarint(inner, uint64(innerField))
	if !ok {
		return 0, fmt.Errorf("entdb: message %q (entdb.%s) missing field %d", desc.FullName(), kind, innerField)
	}
	return int32(v), nil
}

// findLengthDelimited scans a protobuf wire-format byte stream
// looking for a length-delimited (wire type 2) field with the given
// field number and returns its payload. Used to pull the nested
// NodeOpts / EdgeOpts submessage out of MessageOptions.
func findLengthDelimited(buf []byte, fieldNum uint64) ([]byte, bool) {
	for len(buf) > 0 {
		tag, n := decodeVarint(buf)
		if n == 0 {
			return nil, false
		}
		buf = buf[n:]
		wire := tag & 0x7
		num := tag >> 3
		switch wire {
		case 0: // varint
			_, n = decodeVarint(buf)
			if n == 0 {
				return nil, false
			}
			buf = buf[n:]
		case 1: // 64-bit fixed
			if len(buf) < 8 {
				return nil, false
			}
			buf = buf[8:]
		case 2: // length-delimited
			ln, n := decodeVarint(buf)
			if n == 0 {
				return nil, false
			}
			buf = buf[n:]
			if uint64(len(buf)) < ln {
				return nil, false
			}
			if num == fieldNum {
				return buf[:ln], true
			}
			buf = buf[ln:]
		case 5: // 32-bit fixed
			if len(buf) < 4 {
				return nil, false
			}
			buf = buf[4:]
		default:
			return nil, false
		}
	}
	return nil, false
}

// findVarint scans a protobuf wire-format byte stream for a varint
// (wire type 0) field with the given field number and returns its
// value. Used to pull “type_id“/“edge_id“ out of the inner
// NodeOpts/EdgeOpts message.
func findVarint(buf []byte, fieldNum uint64) (uint64, bool) {
	for len(buf) > 0 {
		tag, n := decodeVarint(buf)
		if n == 0 {
			return 0, false
		}
		buf = buf[n:]
		wire := tag & 0x7
		num := tag >> 3
		switch wire {
		case 0:
			v, n := decodeVarint(buf)
			if n == 0 {
				return 0, false
			}
			buf = buf[n:]
			if num == fieldNum {
				return v, true
			}
		case 1:
			if len(buf) < 8 {
				return 0, false
			}
			buf = buf[8:]
		case 2:
			ln, n := decodeVarint(buf)
			if n == 0 {
				return 0, false
			}
			buf = buf[n:]
			if uint64(len(buf)) < ln {
				return 0, false
			}
			buf = buf[ln:]
		case 5:
			if len(buf) < 4 {
				return 0, false
			}
			buf = buf[4:]
		default:
			return 0, false
		}
	}
	return 0, false
}

// decodeVarint returns the varint at the head of buf and the number
// of bytes it consumed, or (0, 0) on error.
func decodeVarint(buf []byte) (uint64, int) {
	var v uint64
	var shift uint
	for i, b := range buf {
		if i >= 10 {
			return 0, 0
		}
		v |= uint64(b&0x7f) << shift
		if b < 0x80 {
			return v, i + 1
		}
		shift += 7
	}
	return 0, 0
}

// marshalForWire converts a proto message into the (type_id,
// field-id-keyed payload) pair that the EntDB wire format expects.
//
// Fields are identified by their numeric proto field id on disk
// (e.g. “"1"“, “"2"“) rather than their proto name, per the
// "field IDs, not field names, on disk" invariant in CLAUDE.md.
//
// For create ops we want every set field. For update ops we want
// ONLY the fields the caller explicitly set. proto3 scalars don't
// carry presence, so “Range“ iterates only (a) non-zero scalars
// and (b) explicitly-set “optional“ fields and messages. That
// matches the intuitive "patch = set fields" semantics for update,
// and is a no-op difference for create (a freshly-constructed
// message populated by the developer has exactly the set fields).
func marshalForWire(msg proto.Message) (int32, map[string]any, error) {
	typeID, err := typeIDFromMessage(msg)
	if err != nil {
		return 0, nil, err
	}
	payload, err := payloadFromMessage(msg)
	if err != nil {
		return 0, nil, err
	}
	return typeID, payload, nil
}

// marshalSetFieldsForWire is the Update counterpart to
// [marshalForWire]. In proto3 the distinction only matters for
// “optional“ fields and submessages, but we keep the two entry
// points separate so callers read as intent-documenting.
func marshalSetFieldsForWire(msg proto.Message) (int32, map[string]any, error) {
	return marshalForWire(msg)
}

// payloadFromMessage walks a proto message's set fields and returns
// a map keyed by string field id. The values are converted to Go
// types the JSON wire codec handles natively (string, int64, float64,
// bool, []any, map[string]any).
func payloadFromMessage(msg proto.Message) (map[string]any, error) {
	if msg == nil {
		return nil, fmt.Errorf("entdb: nil message")
	}
	out := make(map[string]any)
	var err error
	msg.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		converted, cerr := protoValueToGo(fd, v)
		if cerr != nil {
			err = cerr
			return false
		}
		out[strconv.Itoa(int(fd.Number()))] = converted
		return true
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// protoValueToGo converts a protoreflect.Value into a Go value the
// SDK wire codec can round-trip.
func protoValueToGo(fd protoreflect.FieldDescriptor, v protoreflect.Value) (any, error) {
	if fd.IsList() {
		list := v.List()
		out := make([]any, 0, list.Len())
		for i := 0; i < list.Len(); i++ {
			gv, err := scalarToGo(fd, list.Get(i))
			if err != nil {
				return nil, err
			}
			out = append(out, gv)
		}
		return out, nil
	}
	if fd.IsMap() {
		m := v.Map()
		out := make(map[string]any, m.Len())
		var err error
		m.Range(func(k protoreflect.MapKey, mv protoreflect.Value) bool {
			gv, cerr := scalarToGo(fd.MapValue(), mv)
			if cerr != nil {
				err = cerr
				return false
			}
			out[fmt.Sprint(k.Interface())] = gv
			return true
		})
		if err != nil {
			return nil, err
		}
		return out, nil
	}
	return scalarToGo(fd, v)
}

// scalarToGo unwraps a single-valued protoreflect.Value into a Go
// value.
func scalarToGo(fd protoreflect.FieldDescriptor, v protoreflect.Value) (any, error) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return v.Bool(), nil
	case protoreflect.StringKind:
		return v.String(), nil
	case protoreflect.BytesKind:
		// Encode bytes as base64 to survive the JSON wire hop —
		// mirrors the Python SDK, which uses base64 for bytes
		// payload fields.
		return base64.StdEncoding.EncodeToString(v.Bytes()), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return int64(v.Int()), nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return v.Int(), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return int64(v.Uint()), nil
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return int64(v.Uint()), nil
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return v.Float(), nil
	case protoreflect.EnumKind:
		return int64(v.Enum()), nil
	case protoreflect.MessageKind, protoreflect.GroupKind:
		// Nested submessages: walk them recursively into a
		// nested field-id map. Callers that want a flat wire
		// payload should keep their types scalar; nested
		// messages are supported for parity with JSON-mode
		// payloads.
		return payloadFromMessage(v.Message().Interface())
	default:
		return nil, fmt.Errorf("entdb: unsupported proto kind %v", fd.Kind())
	}
}

// unmarshalFromWire builds a fresh proto message of type T from a
// field-id-keyed payload returned by the server. T must be a pointer
// to a generated message type (e.g. “*shop.Product“).
func unmarshalFromWire[T proto.Message](raw map[string]any) (T, error) {
	msg := newZeroMessage[T]()
	if err := applyPayloadToMessage(msg, raw); err != nil {
		var zero T
		return zero, err
	}
	return msg, nil
}

// applyPayloadToMessage copies a field-id-keyed payload into the
// given proto message.
func applyPayloadToMessage(msg proto.Message, raw map[string]any) error {
	if msg == nil {
		return fmt.Errorf("entdb: nil message")
	}
	mr := msg.ProtoReflect()
	fields := mr.Descriptor().Fields()
	for key, val := range raw {
		num, err := strconv.Atoi(key)
		if err != nil {
			// Tolerate name-keyed payloads only if they
			// happen to match a field name — the wire
			// format is field-id-keyed, but we accept
			// names for forgiving round-trips (useful for
			// CLI tooling).
			fd := fields.ByName(protoreflect.Name(key))
			if fd == nil {
				continue
			}
			if err := setField(mr, fd, val); err != nil {
				return err
			}
			continue
		}
		fd := fields.ByNumber(protoreflect.FieldNumber(num))
		if fd == nil {
			continue
		}
		if err := setField(mr, fd, val); err != nil {
			return err
		}
	}
	return nil
}

// setField assigns a Go value to a single proto field.
func setField(mr protoreflect.Message, fd protoreflect.FieldDescriptor, val any) error {
	if val == nil {
		return nil
	}
	if fd.IsList() {
		arr, ok := val.([]any)
		if !ok {
			return fmt.Errorf("entdb: field %s expected list, got %T", fd.Name(), val)
		}
		list := mr.Mutable(fd).List()
		for _, item := range arr {
			pv, err := goToProtoValue(fd, item)
			if err != nil {
				return err
			}
			list.Append(pv)
		}
		return nil
	}
	pv, err := goToProtoValue(fd, val)
	if err != nil {
		return err
	}
	mr.Set(fd, pv)
	return nil
}

// goToProtoValue converts a single Go value coming off the wire into
// a protoreflect.Value suitable for a field of the given kind.
func goToProtoValue(fd protoreflect.FieldDescriptor, val any) (protoreflect.Value, error) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		if b, ok := val.(bool); ok {
			return protoreflect.ValueOfBool(b), nil
		}
	case protoreflect.StringKind:
		if s, ok := val.(string); ok {
			return protoreflect.ValueOfString(s), nil
		}
	case protoreflect.BytesKind:
		if s, ok := val.(string); ok {
			dec, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return protoreflect.Value{}, fmt.Errorf("entdb: field %s bytes decode: %w", fd.Name(), err)
			}
			return protoreflect.ValueOfBytes(dec), nil
		}
		if b, ok := val.([]byte); ok {
			return protoreflect.ValueOfBytes(b), nil
		}
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		n, err := toInt64(val)
		if err == nil {
			return protoreflect.ValueOfInt32(int32(n)), nil
		}
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		n, err := toInt64(val)
		if err == nil {
			return protoreflect.ValueOfInt64(n), nil
		}
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		n, err := toInt64(val)
		if err == nil && n >= 0 {
			return protoreflect.ValueOfUint32(uint32(n)), nil
		}
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		n, err := toInt64(val)
		if err == nil && n >= 0 {
			return protoreflect.ValueOfUint64(uint64(n)), nil
		}
	case protoreflect.FloatKind:
		if f, ok := val.(float64); ok {
			return protoreflect.ValueOfFloat32(float32(f)), nil
		}
	case protoreflect.DoubleKind:
		if f, ok := val.(float64); ok {
			return protoreflect.ValueOfFloat64(f), nil
		}
	case protoreflect.EnumKind:
		n, err := toInt64(val)
		if err == nil {
			return protoreflect.ValueOfEnum(protoreflect.EnumNumber(n)), nil
		}
	}
	return protoreflect.Value{}, fmt.Errorf("entdb: field %s kind %v: cannot convert %T", fd.Name(), fd.Kind(), val)
}

// toInt64 coerces a Go numeric / JSON number value to int64.
func toInt64(v any) (int64, error) {
	switch x := v.(type) {
	case int:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case int64:
		return x, nil
	case uint:
		return int64(x), nil
	case uint32:
		return int64(x), nil
	case uint64:
		return int64(x), nil
	case float32:
		return int64(x), nil
	case float64:
		return int64(x), nil
	case string:
		return strconv.ParseInt(x, 10, 64)
	}
	return 0, fmt.Errorf("entdb: cannot coerce %T to int64", v)
}

// toProtoValue converts a Go scalar into a google.protobuf.Value for
// the GetNodeByKey wire RPC. The value parameter on the wire is
// typed as “google.protobuf.Value“ so one RPC shape covers string,
// int, float, and bool unique fields without a separate oneof.
func toProtoValue(v any) (*structpb.Value, error) {
	switch x := v.(type) {
	case nil:
		return structpb.NewNullValue(), nil
	case string:
		return structpb.NewStringValue(x), nil
	case bool:
		return structpb.NewBoolValue(x), nil
	case int:
		return structpb.NewNumberValue(float64(x)), nil
	case int32:
		return structpb.NewNumberValue(float64(x)), nil
	case int64:
		return structpb.NewNumberValue(float64(x)), nil
	case uint:
		return structpb.NewNumberValue(float64(x)), nil
	case uint32:
		return structpb.NewNumberValue(float64(x)), nil
	case uint64:
		return structpb.NewNumberValue(float64(x)), nil
	case float32:
		return structpb.NewNumberValue(float64(x)), nil
	case float64:
		return structpb.NewNumberValue(x), nil
	case []byte:
		// Match the Python SDK: bytes unique keys are base64
		// encoded on the wire.
		return structpb.NewStringValue(base64.StdEncoding.EncodeToString(x)), nil
	default:
		return nil, fmt.Errorf("entdb: unsupported unique key value type: %T", v)
	}
}
