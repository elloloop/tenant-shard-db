// Tests for payload translation. Mirrors the asserted behavior in
// tests/python/unit/test_grpc_wire_format.py — wire payload is
// id-keyed, unknown names dropped, field order does not matter,
// schema-less mode passes through.

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

// TestPayload_Int64Spectrum_BugC is the regression test for the int64
// data-integrity landmine (Bug C, ADR-028). A typed INTEGER payload value
// MUST survive round-trip exactly across the full int64 range, via the
// typed EntValue carrier (which holds a real int64) — NOT the legacy
// google.protobuf.Struct path, whose IEEE-754 doubles corrupt >2^53.
//
// Two paths are checked, since losslessness must hold at every hop:
//
//  1. Wire round-trip: PayloadToTyped -> TypedToPayload.
//  2. At-rest round-trip: the value marshalled to payload_json and read
//     back the way the store does (json.Decoder + UseNumber, so integers
//     survive as json.Number rather than float64), then PayloadToTyped.
//
// The legacy Struct path remains lossy by design (ADR-028 retires it);
// that is asserted separately in TestPayload_StructPathStillLossy_BugC.
func TestPayload_Int64Spectrum_BugC(t *testing.T) {
	reg := fixtureRegistry(t)
	// field_id 3 == "age", schema.KindInteger.
	cases := []int64{
		(1 << 53) + 1,          // 9007199254740993 — first unsafe odd int
		10_000_000_000_000_001, // 10^16 + 1
		(1 << 62) + 1,          // large but well within int64
		math.MaxInt64 - 1,      // sentinel-adjacent
		math.MaxInt64,          // MaxInt64 sentinel
		-(1 << 53) - 1,         // negative side of the boundary
		math.MinInt64,          // MinInt64 sentinel
	}
	for _, want := range cases {
		// 1) wire round-trip
		ev, err := PayloadToTyped(reg, "User", map[uint32]any{3: want})
		if err != nil {
			t.Errorf("PayloadToTyped(age=%d): %v", want, err)
			continue
		}
		got, err := TypedToPayload(ev)
		if err != nil {
			t.Errorf("TypedToPayload(age=%d): %v", want, err)
			continue
		}
		if got[3] != any(want) {
			t.Errorf("wire round-trip: wrote %d, read back %v (%T)", want, got[3], got[3])
		}

		// 2) at-rest round-trip through payload_json + UseNumber
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
		ev2, err := PayloadToTyped(reg, "User", map[uint32]any{3: decoded["3"]})
		if err != nil {
			t.Errorf("PayloadToTyped(at-rest age=%d): %v", want, err)
			continue
		}
		got2, _ := TypedToPayload(ev2)
		if got2[3] != any(want) {
			t.Errorf("at-rest round-trip: wrote %d, read back %v (%T) — int64 corrupted", want, got2[3], got2[3])
		}
	}
}

// TestPayload_StructPathStillLossy_BugC documents that the retired
// google.protobuf.Struct payload path remains lossy for int64 >2^53 — the
// reason ADR-028 introduces the typed EntValue carrier. Kept as an
// executable note so the contrast is explicit.
func TestPayload_StructPathStillLossy_BugC(t *testing.T) {
	reg := fixtureRegistry(t)
	const want = int64(1)<<53 + 1
	s, err := PayloadToStruct(reg, "User", map[uint32]any{3: want})
	if err != nil {
		t.Fatalf("PayloadToStruct: %v", err)
	}
	got, err := StructToPayload(reg, "User", s)
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

// TestStructToPayload_SchemalessIdKeyedAccepted: name-keyed ingress is
// rejected on an unregistered type, but id-keyed ingress is accepted
// and preserved verbatim. This is the schemaless contract: the SDK
// pre-translates names to field_ids client-side, so the wire is
// always id-keyed even when the server has no schema for the type.
func TestStructToPayload_SchemalessIdKeyedAccepted(t *testing.T) {
	// Empty (but frozen) registry — no types registered.
	reg := schema.NewRegistry()
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	s := mustStruct(t, map[string]any{
		"1": "alice@example.com",
		"2": "Alice",
	})
	got, err := StructToPayload(reg, "Unknown", s)
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

// TestStructToPayload_SchemalessNameKeyedRejected: name-keyed ingress
// on an unregistered type returns INVALID_ARGUMENT. Without a schema
// the server has no name→id mapping; silently dropping the keys
// (the prior behavior) caused silent data loss.
func TestStructToPayload_SchemalessNameKeyedRejected(t *testing.T) {
	reg := schema.NewRegistry()
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	s := mustStruct(t, map[string]any{
		"email": "alice@example.com",
		"name":  "Alice",
	})
	_, err := StructToPayload(reg, "Unknown", s)
	if err == nil {
		t.Fatal("expected INVALID_ARGUMENT, got nil")
	}
	if !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	// The error message should name the offending field so the SDK author
	// can locate the bug.
	if !strings.Contains(err.Error(), "email") && !strings.Contains(err.Error(), "name") {
		t.Fatalf("error should mention offending field, got: %v", err)
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

// Schema-less ingress accepts id-keyed payloads (CLAUDE.md invariant #6:
// the SDKs pre-translate names client-side from the proto descriptor,
// so the wire is always id-keyed). A mixed payload with even one
// name key is rejected — that signals a misconfigured or out-of-date
// SDK and the prior silent-drop behavior caused silent data loss.
func TestStructToPayload_SchemaLessDigitKeyPassthrough(t *testing.T) {
	// Pure id-keyed: accepted, passed through verbatim.
	got, err := StructToPayload(nil, "", mustStruct(t, map[string]any{
		"1": "id-keyed",
		"2": "also-id-keyed",
	}))
	if err != nil {
		t.Fatalf("StructToPayload(schema-less, id-keyed): %v", err)
	}
	if got[1] != "id-keyed" || got[2] != "also-id-keyed" {
		t.Fatalf("expected id-keyed passthrough, got %v", got)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d (%v)", len(got), got)
	}

	// Mixed payload with a name key: rejected.
	_, err = StructToPayload(nil, "", mustStruct(t, map[string]any{
		"1":    "id-keyed",
		"name": "name-keyed-but-no-schema",
	}))
	if err == nil || !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("expected InvalidArgument for mixed payload, got %v", err)
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
