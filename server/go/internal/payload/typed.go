// Typed payload translation (ADR-028).
//
// EntValue is the lossless wire carrier that replaces google.protobuf.Struct
// for payload values: its oneof holds a real int64, so INTEGER / TIMESTAMP
// fields preserve the full range that Struct's IEEE-754 double could not
// (Bug C). These functions are the ingress/egress counterparts of
// StructToPayload / PayloadToStruct and produce/consume the same id-keyed
// internal payload (map[uint32]any), so the applier and store are unchanged.
//
// Scope: scalar kinds round-trip losslessly. JSON and list kinds use the
// json_value variant (structpb.Value) and inherit JSON's dynamically-typed,
// double-backed numbers — callers needing a lossless integer use an INTEGER
// field, not a JSON/list field (ADR-028 tradeoff). Lossless list_int is a
// tracked follow-up.

package payload

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/structpb"
)

// TypedToPayload converts a wire map<uint32, EntValue> into the id-keyed
// internal payload. The oneof already carries the concrete type, so int64
// is preserved exactly — no float64 round-trip, no safe-integer guard.
// An empty/unset EntValue (or nil) maps to an explicit null.
func TypedToPayload(m map[uint32]*pb.EntValue) (map[uint32]any, error) {
	out := make(map[uint32]any, len(m))
	for id, v := range m {
		gv, err := entValueToGo(v)
		if err != nil {
			return nil, err
		}
		out[id] = gv
	}
	return out, nil
}

func entValueToGo(v *pb.EntValue) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch k := v.GetV().(type) {
	case *pb.EntValue_IntValue:
		return k.IntValue, nil
	case *pb.EntValue_DoubleValue:
		return k.DoubleValue, nil
	case *pb.EntValue_BoolValue:
		return k.BoolValue, nil
	case *pb.EntValue_StringValue:
		return k.StringValue, nil
	case *pb.EntValue_BytesValue:
		return k.BytesValue, nil
	case *pb.EntValue_JsonValue:
		return k.JsonValue.AsInterface(), nil
	case nil:
		return nil, nil // oneof unset == explicit null
	default:
		return nil, errs.Errorf(codes.InvalidArgument,
			"payload: unsupported EntValue variant %T", k)
	}
}

// PayloadToTyped converts an id-keyed internal payload (as read from
// payload_json, where the store decodes numbers with UseNumber so int64
// survives as json.Number) into the wire map<uint32, EntValue>. The schema
// disambiguates the on-disk JSON representation per field kind; schema-less
// values fall back to Go-type inference.
func PayloadToTyped(reg *schema.Registry, nodeTypeName string, p map[uint32]any) (map[uint32]*pb.EntValue, error) {
	out := make(map[uint32]*pb.EntValue, len(p))
	nt := lookupNodeType(reg, nodeTypeName)
	for id, raw := range p {
		var f *schema.FieldDef
		if nt != nil {
			f = nt.GetFieldByID(id)
		}
		ev, err := goToEntValue(f, raw)
		if err != nil {
			return nil, err
		}
		out[id] = ev
	}
	return out, nil
}

func goToEntValue(f *schema.FieldDef, raw any) (*pb.EntValue, error) {
	if raw == nil {
		return &pb.EntValue{}, nil
	}
	if f == nil {
		return inferEntValue(raw)
	}
	switch f.Kind {
	case schema.KindInteger, schema.KindTimestamp:
		n, err := anyToInt64(raw)
		if err != nil {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: field %q (%s): %v", f.Name, f.Kind, err)
		}
		return &pb.EntValue{V: &pb.EntValue_IntValue{IntValue: n}}, nil
	case schema.KindFloat:
		d, err := anyToFloat64(raw)
		if err != nil {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: field %q (float): %v", f.Name, err)
		}
		return &pb.EntValue{V: &pb.EntValue_DoubleValue{DoubleValue: d}}, nil
	case schema.KindBoolean:
		b, ok := raw.(bool)
		if !ok {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: field %q (bool): got %T", f.Name, raw)
		}
		return &pb.EntValue{V: &pb.EntValue_BoolValue{BoolValue: b}}, nil
	case schema.KindBytes:
		b, err := anyToBytes(raw)
		if err != nil {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: field %q (bytes): %v", f.Name, err)
		}
		return &pb.EntValue{V: &pb.EntValue_BytesValue{BytesValue: b}}, nil
	case schema.KindString, schema.KindEnum, schema.KindReference:
		s, ok := raw.(string)
		if !ok {
			return nil, errs.Errorf(codes.InvalidArgument,
				"payload: field %q (%s): got %T", f.Name, f.Kind, raw)
		}
		return &pb.EntValue{V: &pb.EntValue_StringValue{StringValue: s}}, nil
	default:
		// KindJSON + list kinds: dynamically typed; carried as a JSON
		// value (numbers are double-backed — ADR-028 tradeoff).
		return jsonEntValue(raw)
	}
}

// inferEntValue maps a schema-less value to the closest EntValue variant.
// Integers (int64 / integral json.Number) stay lossless; everything else
// follows the Go type.
func inferEntValue(raw any) (*pb.EntValue, error) {
	switch x := raw.(type) {
	case int64:
		return &pb.EntValue{V: &pb.EntValue_IntValue{IntValue: x}}, nil
	case int:
		return &pb.EntValue{V: &pb.EntValue_IntValue{IntValue: int64(x)}}, nil
	case int32:
		return &pb.EntValue{V: &pb.EntValue_IntValue{IntValue: int64(x)}}, nil
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return &pb.EntValue{V: &pb.EntValue_IntValue{IntValue: n}}, nil
		}
		d, err := x.Float64()
		if err != nil {
			return nil, errs.Errorf(codes.InvalidArgument, "payload: bad number %q", x.String())
		}
		return &pb.EntValue{V: &pb.EntValue_DoubleValue{DoubleValue: d}}, nil
	case float64:
		return &pb.EntValue{V: &pb.EntValue_DoubleValue{DoubleValue: x}}, nil
	case bool:
		return &pb.EntValue{V: &pb.EntValue_BoolValue{BoolValue: x}}, nil
	case string:
		return &pb.EntValue{V: &pb.EntValue_StringValue{StringValue: x}}, nil
	case []byte:
		return &pb.EntValue{V: &pb.EntValue_BytesValue{BytesValue: x}}, nil
	default:
		return jsonEntValue(raw)
	}
}

func jsonEntValue(raw any) (*pb.EntValue, error) {
	jv, err := structpb.NewValue(normalizeForStruct(raw))
	if err != nil {
		return nil, errs.Errorf(codes.InvalidArgument, "payload: json value: %v", err)
	}
	return &pb.EntValue{V: &pb.EntValue_JsonValue{JsonValue: jv}}, nil
}

// normalizeForStruct makes a value acceptable to structpb.NewValue, which
// rejects json.Number and integer Go types. Recurses into maps/slices.
func normalizeForStruct(raw any) any {
	switch x := raw.(type) {
	case json.Number:
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	case int64:
		return float64(x)
	case int:
		return float64(x)
	case int32:
		return float64(x)
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, v := range x {
			m[k] = normalizeForStruct(v)
		}
		return m
	case []any:
		s := make([]any, len(x))
		for i, v := range x {
			s[i] = normalizeForStruct(v)
		}
		return s
	default:
		return x
	}
}

func anyToInt64(v any) (int64, error) {
	switch x := v.(type) {
	case int64:
		return x, nil
	case int:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case json.Number:
		return x.Int64() // lossless: parses the decimal string
	case float64:
		// Legacy Struct path: only float64-safe integers are representable.
		if x == float64(int64(x)) {
			return int64(x), nil
		}
		return 0, fmt.Errorf("non-integral float64 %v", x)
	case string:
		return strconv.ParseInt(x, 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", v)
	}
}

func anyToFloat64(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case json.Number:
		return x.Float64()
	case int64:
		return float64(x), nil
	case int:
		return float64(x), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

func anyToBytes(v any) ([]byte, error) {
	switch x := v.(type) {
	case []byte:
		return x, nil
	case string:
		// On disk, bytes are stored base64 (JSON has no bytes type).
		return base64.StdEncoding.DecodeString(x)
	default:
		return nil, fmt.Errorf("cannot convert %T to bytes", v)
	}
}
