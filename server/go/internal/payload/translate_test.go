// Tests for payload translation. Mirrors the asserted behavior in
// tests/python/unit/test_grpc_wire_format.py — wire payload is
// id-keyed, unknown names dropped, field order does not matter,
// schema-less mode passes through.

package payload

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"google.golang.org/protobuf/types/known/structpb"
)

// fixtureRegistry seeds a minimal frozen registry with a User-like type.
// Field IDs are deliberately non-sequential so order independence and
// id-vs-name correctness are observable in test output.
func fixtureRegistry(t *testing.T) *schema.Registry {
	t.Helper()
	reg := schema.NewRegistry()
	if err := reg.RegisterNode(&schema.NodeTypeDef{
		TypeID: 1,
		Name:   "User",
		Fields: []schema.FieldDef{
			{FieldID: 1, Name: "email", Kind: schema.KindString},
			{FieldID: 2, Name: "name", Kind: schema.KindString},
			{FieldID: 3, Name: "age", Kind: schema.KindInteger},
			{FieldID: 4, Name: "active", Kind: schema.KindBoolean},
			{FieldID: 5, Name: "avatar", Kind: schema.KindBytes},
			{FieldID: 6, Name: "created_at", Kind: schema.KindTimestamp},
			{FieldID: 7, Name: "metadata", Kind: schema.KindJSON},
			{FieldID: 8, Name: "status", Kind: schema.KindEnum, EnumValues: []string{"active", "banned"}},
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

// TestStructToPayload_KeysAreFieldIDs is the central CLAUDE.md
// invariant #6 test: marshal a Struct keyed by field NAMES and assert
// the resulting map keys are field_ids (uint32), not names.
func TestStructToPayload_KeysAreFieldIDs(t *testing.T) {
	reg := fixtureRegistry(t)
	s := mustStruct(t, map[string]any{
		"email": "alice@example.com",
		"name":  "Alice",
	})
	got, err := StructToPayload(reg, "User", s)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	if v, ok := got[1]; !ok || v != "alice@example.com" {
		t.Fatalf("expected got[1]=email value, got %v (%T), full=%v", v, v, got)
	}
	if v, ok := got[2]; !ok || v != "Alice" {
		t.Fatalf("expected got[2]=name value, got %v (%T), full=%v", v, v, got)
	}
	// No string keys leaked through (the map type forbids that, but
	// also assert that the size is exactly 2 — no extras).
	if len(got) != 2 {
		t.Fatalf("expected 2 entries (id-keyed), got %d: %v", len(got), got)
	}
}

func TestStructToPayload_DropsUnknownFields(t *testing.T) {
	reg := fixtureRegistry(t)
	s := mustStruct(t, map[string]any{
		"email":       "drop@example.com",
		"ghost_field": "should be dropped",
	})
	got, err := StructToPayload(reg, "User", s)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only known field after drop, got %v", got)
	}
	if _, ok := got[1]; !ok {
		t.Fatalf("expected email (id=1) in result, got %v", got)
	}
}

func TestStructToPayload_EmptyStructIsEmptyMap(t *testing.T) {
	reg := fixtureRegistry(t)
	got, err := StructToPayload(reg, "User", &structpb.Struct{})
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil map for empty struct")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
	got2, err := StructToPayload(reg, "User", nil)
	if err != nil {
		t.Fatalf("StructToPayload(nil): %v", err)
	}
	if len(got2) != 0 {
		t.Fatalf("expected empty map for nil struct, got %v", got2)
	}
}

func TestStructToPayload_DigitKeyPassthrough(t *testing.T) {
	reg := fixtureRegistry(t)
	// Pre-translated digit keys: id 1 is known (email), id 99 is not.
	s := mustStruct(t, map[string]any{
		"1":  "via@digit.com",
		"99": "unknown id, drop",
	})
	got, err := StructToPayload(reg, "User", s)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	if v, ok := got[1]; !ok || v != "via@digit.com" {
		t.Fatalf("expected digit-key passthrough, got %v", got)
	}
	if _, ok := got[99]; ok {
		t.Fatalf("unknown digit id should have been dropped, got %v", got)
	}
}

func TestStructToPayload_BytesBase64Coercion(t *testing.T) {
	reg := fixtureRegistry(t)
	original := []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01, 0x02}
	encoded := base64.StdEncoding.EncodeToString(original)
	s := mustStruct(t, map[string]any{"avatar": encoded})

	got, err := StructToPayload(reg, "User", s)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	v, ok := got[5].([]byte)
	if !ok {
		t.Fatalf("expected []byte for avatar, got %T (%v)", got[5], got[5])
	}
	if !bytes.Equal(v, original) {
		t.Fatalf("bytes round-trip mismatch: got %x want %x", v, original)
	}
}

func TestStructToPayload_BytesInvalidBase64IsInvalidArgument(t *testing.T) {
	reg := fixtureRegistry(t)
	s := mustStruct(t, map[string]any{"avatar": "***not base64***"})
	_, err := StructToPayload(reg, "User", s)
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
		"age":        42.0,
		"created_at": 1715000000000.0,
	})
	got, err := StructToPayload(reg, "User", s)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	if v, ok := got[3].(int64); !ok || v != 42 {
		t.Fatalf("expected age=int64(42), got %T(%v)", got[3], got[3])
	}
	if v, ok := got[6].(int64); !ok || v != 1715000000000 {
		t.Fatalf("expected created_at=int64(...), got %T(%v)", got[6], got[6])
	}
}

func TestStructToPayload_TimestampOutOfRange(t *testing.T) {
	reg := fixtureRegistry(t)
	// 2^54 — outside structpb safe integer range.
	s := mustStruct(t, map[string]any{"created_at": float64(1) * (1 << 54)})
	_, err := StructToPayload(reg, "User", s)
	if err == nil || !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("expected INVALID_ARGUMENT for out-of-range timestamp, got %v", err)
	}
}

func TestStructToPayload_JSONNestedMapPassthrough(t *testing.T) {
	reg := fixtureRegistry(t)
	s := mustStruct(t, map[string]any{
		"metadata": map[string]any{
			"key": "value",
			"nested": map[string]any{
				"deep": float64(1),
			},
		},
	})
	got, err := StructToPayload(reg, "User", s)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	m, ok := got[7].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for metadata, got %T", got[7])
	}
	if m["key"] != "value" {
		t.Fatalf("expected metadata.key=value, got %v", m["key"])
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
	s := mustStruct(t, map[string]any{"status": "active"})
	got, err := StructToPayload(reg, "User", s)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	if got[8] != "active" {
		t.Fatalf("expected status=active, got %v", got[8])
	}
}

func TestStructToPayload_FieldOrderIndependent(t *testing.T) {
	reg := fixtureRegistry(t)
	s1 := mustStruct(t, map[string]any{"email": "x@example.com", "name": "X"})
	s2 := mustStruct(t, map[string]any{"name": "X", "email": "x@example.com"})

	g1, err := StructToPayload(reg, "User", s1)
	if err != nil {
		t.Fatalf("s1: %v", err)
	}
	g2, err := StructToPayload(reg, "User", s2)
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
	got, err := PayloadToStruct(reg, "User", in)
	if err != nil {
		t.Fatalf("PayloadToStruct: %v", err)
	}
	keys := got.GetFields()
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys on the wire, got %d (%v)", len(keys), keys)
	}
	if v, ok := keys["1"]; !ok || v.GetStringValue() != "alice@example.com" {
		t.Fatalf("expected keys[\"1\"]=email, got %v", keys)
	}
	if v, ok := keys["2"]; !ok || v.GetStringValue() != "Alice" {
		t.Fatalf("expected keys[\"2\"]=name, got %v", keys)
	}
}

func TestPayloadToStruct_BytesEncodedAsBase64(t *testing.T) {
	reg := fixtureRegistry(t)
	original := []byte{0xca, 0xfe, 0xba, 0xbe}
	got, err := PayloadToStruct(reg, "User", map[uint32]any{5: original})
	if err != nil {
		t.Fatalf("PayloadToStruct: %v", err)
	}
	v := got.GetFields()["5"].GetStringValue()
	if v != base64.StdEncoding.EncodeToString(original) {
		t.Fatalf("expected base64 encoded bytes, got %q", v)
	}
}

func TestPayloadToStruct_TimestampInt64IsNumberValue(t *testing.T) {
	reg := fixtureRegistry(t)
	got, err := PayloadToStruct(reg, "User", map[uint32]any{6: int64(1715000000000)})
	if err != nil {
		t.Fatalf("PayloadToStruct: %v", err)
	}
	v := got.GetFields()["6"].GetNumberValue()
	if v != 1715000000000.0 {
		t.Fatalf("expected number 1715000000000, got %v", v)
	}
}

func TestPayloadToStruct_SchemaLessPassthrough(t *testing.T) {
	// No registry, no type name — the wire still carries stringified
	// field_id keys and primitive values pass through.
	got, err := PayloadToStruct(nil, "", map[uint32]any{
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
	// Field ID 999 is not on the schema; egress must still emit it
	// (forward-compat).
	got, err := PayloadToStruct(reg, "User", map[uint32]any{
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
	got, err := PayloadToStruct(reg, "User", map[uint32]any{})
	if err != nil {
		t.Fatalf("PayloadToStruct: %v", err)
	}
	if len(got.GetFields()) != 0 {
		t.Fatalf("expected empty Struct, got %v", got)
	}
	got2, err := PayloadToStruct(reg, "User", nil)
	if err != nil {
		t.Fatalf("PayloadToStruct(nil): %v", err)
	}
	if got2 == nil || len(got2.GetFields()) != 0 {
		t.Fatalf("expected empty Struct for nil map, got %v", got2)
	}
}

// --- Round-trip ----------------------------------------------------------

func TestRoundTrip_NameKeyedToIDKeyedToWire(t *testing.T) {
	reg := fixtureRegistry(t)
	in := mustStruct(t, map[string]any{
		"email":  "rt@example.com",
		"name":   "Round Trip",
		"age":    42.0,
		"active": true,
	})
	mid, err := StructToPayload(reg, "User", in)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	out, err := PayloadToStruct(reg, "User", mid)
	if err != nil {
		t.Fatalf("PayloadToStruct: %v", err)
	}
	// The wire should be id-keyed.
	keys := out.GetFields()
	wantKeys := []string{"1", "2", "3", "4"}
	for _, k := range wantKeys {
		if _, ok := keys[k]; !ok {
			t.Fatalf("round-trip missing key %q (got %v)", k, keys)
		}
	}
	if keys["1"].GetStringValue() != "rt@example.com" {
		t.Fatalf("email round-trip mismatch")
	}
	if keys["3"].GetNumberValue() != 42.0 {
		t.Fatalf("age round-trip mismatch")
	}
	if !keys["4"].GetBoolValue() {
		t.Fatalf("active round-trip mismatch")
	}
}

// --- FilterNamesToIDs ----------------------------------------------------

func TestFilterNamesToIDs_TranslatesNames(t *testing.T) {
	reg := fixtureRegistry(t)
	got, err := FilterNamesToIDs(reg, "User", map[string]any{
		"email": "alice@example.com",
		"age":   42,
	})
	if err != nil {
		t.Fatalf("FilterNamesToIDs: %v", err)
	}
	if got[1] != "alice@example.com" {
		t.Fatalf("expected got[1]=email, got %v", got)
	}
	if got[3] != 42 {
		t.Fatalf("expected got[3]=age, got %v", got)
	}
}

func TestFilterNamesToIDs_DigitKeyPassthrough(t *testing.T) {
	reg := fixtureRegistry(t)
	got, err := FilterNamesToIDs(reg, "User", map[string]any{
		"1": "via id",
	})
	if err != nil {
		t.Fatalf("FilterNamesToIDs: %v", err)
	}
	if got[1] != "via id" {
		t.Fatalf("expected got[1]=via id, got %v", got)
	}
}

func TestFilterNamesToIDs_UnknownNameIsInvalidArgument(t *testing.T) {
	reg := fixtureRegistry(t)
	_, err := FilterNamesToIDs(reg, "User", map[string]any{
		"ghost": "x",
	})
	if err == nil {
		t.Fatal("expected error for unknown filter field")
	}
	if !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("expected INVALID_ARGUMENT, got %v", err)
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("expected error to mention field name, got %v", err)
	}
}

func TestFilterNamesToIDs_UnknownDigitIDIsInvalidArgument(t *testing.T) {
	reg := fixtureRegistry(t)
	_, err := FilterNamesToIDs(reg, "User", map[string]any{
		"999": "x",
	})
	if err == nil || !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("expected INVALID_ARGUMENT for unknown digit id, got %v", err)
	}
}

func TestFilterNamesToIDs_SchemaLessRequiresDigitKeys(t *testing.T) {
	got, err := FilterNamesToIDs(nil, "", map[string]any{"1": "x"})
	if err != nil {
		t.Fatalf("FilterNamesToIDs(schema-less, digit): %v", err)
	}
	if got[1] != "x" {
		t.Fatalf("expected schema-less digit passthrough, got %v", got)
	}
	_, err = FilterNamesToIDs(nil, "", map[string]any{"name": "x"})
	if err == nil || !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("schema-less + name key should be INVALID_ARGUMENT, got %v", err)
	}
}

// --- PayloadJSONToNames --------------------------------------------------

func TestPayloadJSONToNames_TranslatesIDsToNames(t *testing.T) {
	reg := fixtureRegistry(t)
	got, err := PayloadJSONToNames(reg, "User", map[uint32]any{
		1: "alice@example.com",
		2: "Alice",
	})
	if err != nil {
		t.Fatalf("PayloadJSONToNames: %v", err)
	}
	if got["email"] != "alice@example.com" {
		t.Fatalf("expected email, got %v", got)
	}
	if got["name"] != "Alice" {
		t.Fatalf("expected name, got %v", got)
	}
}

func TestPayloadJSONToNames_UnknownIDPreservedAsString(t *testing.T) {
	reg := fixtureRegistry(t)
	got, err := PayloadJSONToNames(reg, "User", map[uint32]any{
		1:   "known",
		999: "future",
	})
	if err != nil {
		t.Fatalf("PayloadJSONToNames: %v", err)
	}
	if got["email"] != "known" {
		t.Fatalf("expected email=known, got %v", got)
	}
	if got["999"] != "future" {
		t.Fatalf("expected unknown id preserved as decimal-string key, got %v", got)
	}
}

func TestPayloadJSONToNames_SchemaLessUsesDecimalKeys(t *testing.T) {
	got, err := PayloadJSONToNames(nil, "", map[uint32]any{
		1: "v1",
		2: "v2",
	})
	if err != nil {
		t.Fatalf("PayloadJSONToNames(schema-less): %v", err)
	}
	if got["1"] != "v1" || got["2"] != "v2" {
		t.Fatalf("expected decimal-keyed map, got %v", got)
	}
}

func TestPayloadJSONToNames_EmptyInput(t *testing.T) {
	reg := fixtureRegistry(t)
	got, err := PayloadJSONToNames(reg, "User", map[uint32]any{})
	if err != nil {
		t.Fatalf("PayloadJSONToNames: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil empty map")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %v", got)
	}
}

// PayloadJSONToNames is the inverse of StructToPayload for the
// schema-aware case (modulo proto Value vs Go any boxing).
func TestPayloadJSONToNames_InverseOfStructToPayload(t *testing.T) {
	reg := fixtureRegistry(t)
	in := mustStruct(t, map[string]any{
		"email": "round@example.com",
		"name":  "Round",
	})
	mid, err := StructToPayload(reg, "User", in)
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	out, err := PayloadJSONToNames(reg, "User", mid)
	if err != nil {
		t.Fatalf("PayloadJSONToNames: %v", err)
	}
	if out["email"] != "round@example.com" {
		t.Fatalf("expected email round-trip, got %v", out)
	}
	if out["name"] != "Round" {
		t.Fatalf("expected name round-trip, got %v", out)
	}
}

// --- Schema-less ingress -------------------------------------------------

func TestStructToPayload_SchemaLessDigitKeyPassthrough(t *testing.T) {
	got, err := StructToPayload(nil, "", mustStruct(t, map[string]any{
		"1":    "id-keyed",
		"name": "name-keyed-but-no-schema",
	}))
	if err != nil {
		t.Fatalf("StructToPayload(schema-less): %v", err)
	}
	if got[1] != "id-keyed" {
		t.Fatalf("expected digit passthrough, got %v", got)
	}
	if _, ok := got[0]; ok {
		t.Fatalf("schema-less: name keys should be dropped (no resolver), got %v", got)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d (%v)", len(got), got)
	}
}

func TestStructToPayload_UnknownTypeNameSchemaLess(t *testing.T) {
	reg := fixtureRegistry(t)
	got, err := StructToPayload(reg, "DoesNotExist", mustStruct(t, map[string]any{
		"1": "passthrough",
	}))
	if err != nil {
		t.Fatalf("StructToPayload: %v", err)
	}
	if got[1] != "passthrough" {
		t.Fatalf("expected schema-less passthrough for unknown type, got %v", got)
	}
}
