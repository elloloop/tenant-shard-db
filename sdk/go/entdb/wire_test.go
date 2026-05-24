package entdb

import (
	"math"
	"testing"
)

// TestPayloadRoundTrip_Int64Spectrum_BugC is the Go SDK regression for
// Bug C (#563): a payload integer must survive the typed encode/decode
// round-trip (mapToTyped -> typedToMap) with value AND int64 type intact,
// across the full int64 range. The typed EntValue carrier (ADR-028)
// replaces the lossy google.protobuf.Struct path. (The Struct path stays
// lossy by design — asserted in TestPayloadRoundTrip_StructStillLossy.)
func TestPayloadRoundTrip_Int64Spectrum_BugC(t *testing.T) {
	cases := []int64{
		(1 << 53) + 1,          // 9007199254740993 — first unsafe odd int
		10_000_000_000_000_001, // 10^16 + 1
		(1 << 62) + 1,
		math.MaxInt64 - 1,
		math.MaxInt64,
		math.MinInt64,
		-(1 << 53) - 1,
	}
	for _, want := range cases {
		typed := mapToTyped(map[string]any{"1": want})
		out := typedToMap(typed)
		got, ok := out["1"].(int64)
		if !ok {
			t.Errorf("field 1: wrote int64(%d), read back %v (%T) — typed path must preserve int64",
				want, out["1"], out["1"])
			continue
		}
		if got != want {
			t.Errorf("field 1: wrote %d, read back %d — int64 corrupted (Bug C)", want, got)
		}
	}
}
