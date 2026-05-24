package entdb

import (
	"math"
	"testing"
)

// TestPayloadRoundTrip_Int64Spectrum_BugC characterizes the int64
// data-integrity landmine (Bug C) at the Go SDK boundary. A payload
// integer value must survive the encode/decode round-trip
// (mapToStruct -> structToMap) with both its value AND its int64 type
// intact, across the full int64 range.
//
// Today the SDK lowers payloads into a google.protobuf.Struct, whose
// numbers are IEEE-754 doubles (wire.go: structpb.NewStruct / AsMap), so
// every integer comes back as a float64 and values above 2^53 lose
// precision (e.g. 2^53+1 -> 2^53). The fix (a typed field_id->Value wire
// carrier; ADR pending) must make these round-trip exactly as int64.
//
// LANDED RED on purpose (Bug C characterization). Skipped so CI stays
// green; remove the t.Skip when the typed-value carrier lands.
func TestPayloadRoundTrip_Int64Spectrum_BugC(t *testing.T) {
	t.Skip("Bug C: int64 payload corruption via google.protobuf.Struct (double-backed); un-skip when the typed field_id->Value wire carrier lands — ADR pending")
	cases := []int64{
		(1 << 53) + 1,          // 9007199254740993 — first unsafe odd int
		10_000_000_000_000_001, // 10^16 + 1
		(1 << 62) + 1,
		math.MaxInt64 - 1,
		math.MaxInt64,
		-(1 << 53) - 1,
	}
	for _, want := range cases {
		s, err := mapToStruct(map[string]any{"1": want})
		if err != nil {
			t.Errorf("mapToStruct(%d): %v", want, err)
			continue
		}
		out := structToMap(s)
		got, ok := out["1"].(int64)
		if !ok {
			t.Errorf("field 1: wrote int64(%d), read back %v (%T) — SDK must preserve the int64 type, not coerce to float64 (Bug C)",
				want, out["1"], out["1"])
			continue
		}
		if got != want {
			t.Errorf("field 1: wrote %d, read back %d — int64 corrupted via Struct (Bug C)", want, got)
		}
	}
}
