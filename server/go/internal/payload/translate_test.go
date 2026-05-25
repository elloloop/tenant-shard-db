// Tests for payload translation. NAME-FREE per ADR-031: the wire payload
// is field_id-keyed (decimal-string keys), a non-digit (name) key is
// rejected, field order does not matter, and the registry (looked up by
// type_id) drives only field-KIND coercion. Schema-less mode passes
// id-keyed values through untouched.

package payload

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"google.golang.org/protobuf/types/known/structpb"
)

// fixtureRegistry seeds a minimal frozen registry with a User-like type
// (type_id=1). Field IDs are deliberately non-sequential so order
// independence and id correctness are observable in test output.
// Name-free per ADR-031: fields carry no names.
func fixtureRegistry(t *testing.T) *schema.Registry {
	t.Helper()
	reg := schema.NewRegistry()
	if err := reg.RegisterNode(&schema.NodeTypeDef{
		TypeID: 1,
		Fields: []schema.FieldDef{
			{FieldID: 1, Kind: schema.KindString},
			{FieldID: 2, Kind: schema.KindString},
			{FieldID: 3, Kind: schema.KindInteger},
			{FieldID: 4, Kind: schema.KindBoolean},
			{FieldID: 5, Kind: schema.KindBytes},
			{FieldID: 6, Kind: schema.KindTimestamp},
			{FieldID: 7, Kind: schema.KindJSON},
			{FieldID: 8, Kind: schema.KindEnum, EnumValues: []string{"active", "banned"}},
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	return reg
}

func mustStruct(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatalf("structpb.NewStruct: %v", err)
	}
	return s
}

// TestStructToPayload_KeysAreFieldIDs is the central ADR-031 / invariant
// #6 test: the wire is field_id-keyed and the resulting map keys are the
// same field_ids (uint32).
func TestStructToPayload_KeysAreFieldIDs(t *testing.T) {
	reg := fixtureRegistry(t)
	s := mustStruct(t, map[string]any{
		"1": "alice@example.com",
		"2": "Alice",
	})
	got, err := StructToPayload(reg, 1, s)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	if v, ok := got[1]; !ok || v != "alice@example.com" {
		t.Fatalf("expected got[1]=field-1 value, got %v (%T), full=%v", v, v, got)
	}
	if v, ok := got[2]; !ok || v != "Alice" {
		t.Fatalf("expected got[2]=field-2 value, got %v (%T), full=%v", v, v, got)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries (id-keyed), got %d: %v", len(got), got)
	}
}

// TestStructToPayload_NameKeyRejected: a non-digit (name) key is
// INVALID_ARGUMENT — the server never translates names (ADR-031).
func TestStructToPayload_NameKeyRejected(t *testing.T) {
	reg := fixtureRegistry(t)
	s := mustStruct(t, map[string]any{
		"1":     "ok@example.com",
		"email": "should reject",
	})
	_, err := StructToPayload(reg, 1, s)
	if err == nil || !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("expected INVALID_ARGUMENT for a name key, got %v", err)
	}
	if !strings.Contains(err.Error(), "email") {
		t.Fatalf("error should name the offending key, got: %v", err)
	}
}

func TestStructToPayload_EmptyStructIsEmptyMap(t *testing.T) {
	reg := fixtureRegistry(t)
	got, err := StructToPayload(reg, 1, &structpb.Struct{})
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil map for empty struct")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
	got2, err := StructToPayload(reg, 1, nil)
	if err != nil {
		t.Fatalf("StructToPayload(nil): %v", err)
	}
	if len(got2) != 0 {
		t.Fatalf("expected empty map for nil struct, got %v", got2)
	}
}

// TestStructToPayload_UnknownDigitKept: a digit key not on the schema is
// kept verbatim (forward-compat — a field from a newer schema revision
// round-trips on write, matching the egress forward-compat rule).
func TestStructToPayload_UnknownDigitKept(t *testing.T) {
	reg := fixtureRegistry(t)
	s := mustStruct(t, map[string]any{
		"1":  "via@digit.com",
		"99": "unknown id, kept",
	})
	got, err := StructToPayload(reg, 1, s)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	if v, ok := got[1]; !ok || v != "via@digit.com" {
		t.Fatalf("expected digit-key coercion, got %v", got)
	}
	if v, ok := got[99]; !ok || v != "unknown id, kept" {
		t.Fatalf("unknown digit id should be kept verbatim, got %v", got)
	}
}

func TestStructToPayload_BytesBase64Coercion(t *testing.T) {
	reg := fixtureRegistry(t)
	original := []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01, 0x02}
	encoded := base64.StdEncoding.EncodeToString(original)
	s := mustStruct(t, map[string]any{"5": encoded})

	got, err := StructToPayload(reg, 1, s)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	v, ok := got[5].([]byte)
	if !ok {
		t.Fatalf("expected []byte for field 5, got %T (%v)", got[5], got[5])
	}
	if !bytes.Equal(v, original) {
		t.Fatalf("bytes round-trip mismatch: got %x want %x", v, original)
	}
}

func TestStructToPayload_BytesInvalidBase64IsInvalidArgument(t *testing.T) {
	reg := fixtureRegistry(t)
	s := mustStruct(t, map[string]any{"5": "***not base64***"})
	_, err := StructToPayload(reg, 1, s)
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
	if !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestStructToPayload_TimestampAndIntegerCoerced(t *testing.T) {
	reg := fixtureRegistry(t)
	s := mustStruct(t, map[string]any{
		"3": 42.0,
		"6": 1715000000000.0,
	})
	got, err := StructToPayload(reg, 1, s)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	if v, ok := got[3].(int64); !ok || v != 42 {
		t.Fatalf("expected field3=int64(42), got %T(%v)", got[3], got[3])
	}
	if v, ok := got[6].(int64); !ok || v != 1715000000000 {
		t.Fatalf("expected field6=int64(...), got %T(%v)", got[6], got[6])
	}
}

func TestStructToPayload_TimestampOutOfRange(t *testing.T) {
	reg := fixtureRegistry(t)
	// 2^54 — outside structpb safe integer range.
	s := mustStruct(t, map[string]any{"6": float64(1) * (1 << 54)})
	_, err := StructToPayload(reg, 1, s)
	if err == nil || !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("expected INVALID_ARGUMENT for out-of-range timestamp, got %v", err)
	}
}

// TestPayload_Int64Spectrum_BugC is the regression test for the int64
// data-integrity landmine (Bug C, ADR-028). A typed INTEGER payload value
// MUST survive round-trip exactly across the full int64 range, via the
// typed EntValue carrier (which holds a real int64).
func TestPayload_Int64Spectrum_BugC(t *testing.T) {
	reg := fixtureRegistry(t)
	// field_id 3 == schema.KindInteger.
	cases := []int64{
		(1 << 53) + 1,
		10_000_000_000_000_001,
		(1 << 62) + 1,
		math.MaxInt64 - 1,
		math.MaxInt64,
		-(1 << 53) - 1,
		math.MinInt64,
	}
	for _, want := range cases {
		ev, err := PayloadToTyped(reg, 1, map[uint32]any{3: want})
		if err != nil {
			t.Errorf("PayloadToTyped(field3=%d): %v", want, err)
			continue
		}
		got, err := TypedToPayload(ev)
		if err != nil {
			t.Errorf("TypedToPayload(field3=%d): %v", want, err)
			continue
		}
		if got[3] != any(want) {
			t.Errorf("wire round-trip: wrote %d, read back %v (%T)", want, got[3], got[3])
		}

		raw, err := json.Marshal(map[string]any{"3": want})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		var decoded map[string]any
		if err := dec.Decode(&decoded); err != nil {
			t.Fatalf("decode: %v", err)
		}
		ev2, err := PayloadToTyped(reg, 1, map[uint32]any{3: decoded["3"]})
		if err != nil {
			t.Errorf("PayloadToTyped(at-rest field3=%d): %v", want, err)
			continue
		}
		got2, _ := TypedToPayload(ev2)
		if got2[3] != any(want) {
			t.Errorf("at-rest round-trip: wrote %d, read back %v (%T) — int64 corrupted", want, got2[3], got2[3])
		}
	}
}

// TestPayload_StructPathStillLossy_BugC documents that the retired
// google.protobuf.Struct payload path remains lossy for int64 >2^53.
func TestPayload_StructPathStillLossy_BugC(t *testing.T) {
	reg := fixtureRegistry(t)
	const want = int64(1)<<53 + 1
	s, err := PayloadToStruct(reg, 1, map[uint32]any{3: want})
	if err != nil {
		t.Fatalf("PayloadToStruct: %v", err)
	}
	got, err := StructToPayload(reg, 1, s)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	if got[3] == any(want) {
		t.Fatalf("Struct path unexpectedly lossless for %d — ADR-028 assumes it is not", want)
	}
}

func TestStructToPayload_JSONNestedMapPassthrough(t *testing.T) {
	reg := fixtureRegistry(t)
	s := mustStruct(t, map[string]any{
		"7": map[string]any{
			"key": "value",
			"nested": map[string]any{
				"deep": float64(1),
			},
		},
	})
	got, err := StructToPayload(reg, 1, s)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	m, ok := got[7].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for field 7, got %T", got[7])
	}
	if m["key"] != "value" {
		t.Fatalf("expected field7.key=value, got %v", m["key"])
	}
	nested, ok := m["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map, got %T", m["nested"])
	}
	if nested["deep"] != float64(1) {
		t.Fatalf("nested.deep mismatch: %v", nested["deep"])
	}
}

func TestStructToPayload_EnumStringPreserved(t *testing.T) {
	reg := fixtureRegistry(t)
	s := mustStruct(t, map[string]any{"8": "active"})
	got, err := StructToPayload(reg, 1, s)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	if got[8] != "active" {
		t.Fatalf("expected field8=active, got %v", got[8])
	}
}

func TestStructToPayload_FieldOrderIndependent(t *testing.T) {
	reg := fixtureRegistry(t)
	s1 := mustStruct(t, map[string]any{"1": "x@example.com", "2": "X"})
	s2 := mustStruct(t, map[string]any{"2": "X", "1": "x@example.com"})

	g1, err := StructToPayload(reg, 1, s1)
	if err != nil {
		t.Fatalf("s1: %v", err)
	}
	g2, err := StructToPayload(reg, 1, s2)
	if err != nil {
		t.Fatalf("s2: %v", err)
	}
	if g1[1] != g2[1] || g1[2] != g2[2] || len(g1) != len(g2) {
		t.Fatalf("order should not matter; got %v vs %v", g1, g2)
	}
}

// --- PayloadToStruct -----------------------------------------------------

func TestPayloadToStruct_KeysAreStringifiedFieldIDs(t *testing.T) {
	reg := fixtureRegistry(t)
	in := map[uint32]any{
		1: "alice@example.com",
		2: "Alice",
	}
	got, err := PayloadToStruct(reg, 1, in)
	if err != nil {
		t.Fatalf("PayloadToStruct: %v", err)
	}
	keys := got.GetFields()
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys on the wire, got %d (%v)", len(keys), keys)
	}
	if v, ok := keys["1"]; !ok || v.GetStringValue() != "alice@example.com" {
		t.Fatalf("expected keys[\"1\"], got %v", keys)
	}
	if v, ok := keys["2"]; !ok || v.GetStringValue() != "Alice" {
		t.Fatalf("expected keys[\"2\"], got %v", keys)
	}
}

func TestPayloadToStruct_BytesEncodedAsBase64(t *testing.T) {
	reg := fixtureRegistry(t)
	original := []byte{0xca, 0xfe, 0xba, 0xbe}
	got, err := PayloadToStruct(reg, 1, map[uint32]any{5: original})
	if err != nil {
		t.Fatalf("PayloadToStruct: %v", err)
	}
	v := got.GetFields()["5"].GetStringValue()
	if v != base64.StdEncoding.EncodeToString(original) {
		t.Fatalf("expected base64 encoded bytes, got %q", v)
	}
}

// TestStructToPayload_SchemalessIdKeyedAccepted: id-keyed ingress is
// accepted and preserved verbatim even with no schema for the type.
func TestStructToPayload_SchemalessIdKeyedAccepted(t *testing.T) {
	reg := schema.NewRegistry()
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	s := mustStruct(t, map[string]any{
		"1": "alice@example.com",
		"2": "Alice",
	})
	// type_id 7 is unknown to the empty registry → schema-less branch.
	got, err := StructToPayload(reg, 7, s)
	if err != nil {
		t.Fatalf("StructToPayload(schemaless, id-keyed): %v", err)
	}
	if got[1] != "alice@example.com" || got[2] != "Alice" {
		t.Fatalf("expected id-keyed passthrough, got %v", got)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(got), got)
	}
}

// TestStructToPayload_SchemalessNameKeyedRejected: a name key is rejected
// even schema-less (ADR-031 — the wire is always id-keyed).
func TestStructToPayload_SchemalessNameKeyedRejected(t *testing.T) {
	reg := schema.NewRegistry()
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	s := mustStruct(t, map[string]any{
		"email": "alice@example.com",
	})
	_, err := StructToPayload(reg, 7, s)
	if err == nil || !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if !strings.Contains(err.Error(), "email") {
		t.Fatalf("error should mention offending key, got: %v", err)
	}
}

func TestPayloadToStruct_TimestampInt64IsNumberValue(t *testing.T) {
	reg := fixtureRegistry(t)
	got, err := PayloadToStruct(reg, 1, map[uint32]any{6: int64(1715000000000)})
	if err != nil {
		t.Fatalf("PayloadToStruct: %v", err)
	}
	v := got.GetFields()["6"].GetNumberValue()
	if v != 1715000000000.0 {
		t.Fatalf("expected number 1715000000000, got %v", v)
	}
}

func TestPayloadToStruct_SchemaLessPassthrough(t *testing.T) {
	got, err := PayloadToStruct(nil, 0, map[uint32]any{
		1: "hello",
		2: float64(3.14),
		3: true,
	})
	if err != nil {
		t.Fatalf("PayloadToStruct(schema-less): %v", err)
	}
	f := got.GetFields()
	if f["1"].GetStringValue() != "hello" {
		t.Fatalf("expected hello, got %v", f["1"])
	}
	if f["2"].GetNumberValue() != 3.14 {
		t.Fatalf("expected 3.14, got %v", f["2"])
	}
	if !f["3"].GetBoolValue() {
		t.Fatalf("expected true, got %v", f["3"])
	}
}

func TestPayloadToStruct_UnknownFieldIDPreservedOnEgress(t *testing.T) {
	reg := fixtureRegistry(t)
	got, err := PayloadToStruct(reg, 1, map[uint32]any{
		1:   "known",
		999: "unknown but kept",
	})
	if err != nil {
		t.Fatalf("PayloadToStruct: %v", err)
	}
	if got.GetFields()["999"].GetStringValue() != "unknown but kept" {
		t.Fatalf("expected unknown id preserved, got %v", got.GetFields())
	}
}

func TestPayloadToStruct_EmptyPayloadIsEmptyStruct(t *testing.T) {
	reg := fixtureRegistry(t)
	got, err := PayloadToStruct(reg, 1, map[uint32]any{})
	if err != nil {
		t.Fatalf("PayloadToStruct: %v", err)
	}
	if len(got.GetFields()) != 0 {
		t.Fatalf("expected empty Struct, got %v", got)
	}
	got2, err := PayloadToStruct(reg, 1, nil)
	if err != nil {
		t.Fatalf("PayloadToStruct(nil): %v", err)
	}
	if got2 == nil || len(got2.GetFields()) != 0 {
		t.Fatalf("expected empty Struct for nil map, got %v", got2)
	}
}

// --- Round-trip ----------------------------------------------------------

func TestRoundTrip_IDKeyedToWire(t *testing.T) {
	reg := fixtureRegistry(t)
	in := mustStruct(t, map[string]any{
		"1": "rt@example.com",
		"2": "Round Trip",
		"3": 42.0,
		"4": true,
	})
	mid, err := StructToPayload(reg, 1, in)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	out, err := PayloadToStruct(reg, 1, mid)
	if err != nil {
		t.Fatalf("PayloadToStruct: %v", err)
	}
	keys := out.GetFields()
	for _, k := range []string{"1", "2", "3", "4"} {
		if _, ok := keys[k]; !ok {
			t.Fatalf("round-trip missing key %q (got %v)", k, keys)
		}
	}
	if keys["1"].GetStringValue() != "rt@example.com" {
		t.Fatalf("field1 round-trip mismatch")
	}
	if keys["3"].GetNumberValue() != 42.0 {
		t.Fatalf("field3 round-trip mismatch")
	}
	if !keys["4"].GetBoolValue() {
		t.Fatalf("field4 round-trip mismatch")
	}
}

// --- FilterToIDs ---------------------------------------------------------

func TestFilterToIDs_IDKeyed(t *testing.T) {
	reg := fixtureRegistry(t)
	got, err := FilterToIDs(reg, 1, map[string]any{
		"1": "alice@example.com",
		"3": 42,
	})
	if err != nil {
		t.Fatalf("FilterToIDs: %v", err)
	}
	if got[1] != "alice@example.com" {
		t.Fatalf("expected got[1], got %v", got)
	}
	if got[3] != 42 {
		t.Fatalf("expected got[3], got %v", got)
	}
}

func TestFilterToIDs_NameKeyIsInvalidArgument(t *testing.T) {
	reg := fixtureRegistry(t)
	_, err := FilterToIDs(reg, 1, map[string]any{
		"email": "x",
	})
	if err == nil || !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("expected INVALID_ARGUMENT for a name key, got %v", err)
	}
	if !strings.Contains(err.Error(), "email") {
		t.Fatalf("expected error to mention key, got %v", err)
	}
}

func TestFilterToIDs_UnknownDigitIDIsInvalidArgument(t *testing.T) {
	reg := fixtureRegistry(t)
	_, err := FilterToIDs(reg, 1, map[string]any{
		"999": "x",
	})
	if err == nil || !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("expected INVALID_ARGUMENT for unknown digit id, got %v", err)
	}
}

func TestFilterToIDs_SchemaLessRequiresDigitKeys(t *testing.T) {
	got, err := FilterToIDs(nil, 0, map[string]any{"1": "x"})
	if err != nil {
		t.Fatalf("FilterToIDs(schema-less, digit): %v", err)
	}
	if got[1] != "x" {
		t.Fatalf("expected schema-less digit passthrough, got %v", got)
	}
	_, err = FilterToIDs(nil, 0, map[string]any{"name": "x"})
	if err == nil || !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("schema-less + name key should be INVALID_ARGUMENT, got %v", err)
	}
}
