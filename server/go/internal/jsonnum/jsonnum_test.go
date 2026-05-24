// SPDX-License-Identifier: AGPL-3.0-only

package jsonnum

import (
	"encoding/json"
	"math"
	"testing"
)

func TestDecode_Int64Spectrum(t *testing.T) {
	cases := []int64{
		0, 1, -1,
		(1 << 53) + 1,          // 9007199254740993 — first unsafe odd int
		10_000_000_000_000_001, // 10^16 + 1
		(1 << 62) + 1,
		math.MaxInt64 - 1,
		math.MaxInt64,
		math.MinInt64,
		-(1 << 53) - 1,
	}
	for _, want := range cases {
		raw, err := json.Marshal(map[string]any{"v": want})
		if err != nil {
			t.Fatalf("marshal %d: %v", want, err)
		}
		m, err := Decode(raw)
		if err != nil {
			t.Fatalf("Decode %d: %v", want, err)
		}
		got, ok := m["v"].(int64)
		if !ok {
			t.Errorf("%d: got %T (%v), want int64", want, m["v"], m["v"])
			continue
		}
		if got != want {
			t.Errorf("int64 round-trip: wrote %d, got %d", want, got)
		}
	}
}

func TestDecode_FloatStaysFloat(t *testing.T) {
	m, err := Decode([]byte(`{"f": 3.5, "i": 7, "s": "x", "b": true}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if _, ok := m["f"].(float64); !ok {
		t.Errorf("f: got %T, want float64", m["f"])
	}
	if v, ok := m["i"].(int64); !ok || v != 7 {
		t.Errorf("i: got %T(%v), want int64(7)", m["i"], m["i"])
	}
	if m["s"] != "x" || m["b"] != true {
		t.Errorf("scalar passthrough wrong: %v %v", m["s"], m["b"])
	}
}

func TestDecode_EmptyIsNonNilMap(t *testing.T) {
	for _, in := range [][]byte{nil, {}, []byte("   ")} {
		m, err := Decode(in)
		if err != nil {
			t.Fatalf("Decode(%q): %v", in, err)
		}
		if m == nil {
			t.Errorf("Decode(%q): got nil map, want empty non-nil", in)
		}
	}
}

func TestNormalize_NestedBigInt(t *testing.T) {
	// json.Number nested in a map/slice normalizes to int64.
	raw := []byte(`{"outer": {"n": 9007199254740993}, "list": [9007199254740993, 1.5]}`)
	m, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	outer := m["outer"].(map[string]any)
	if outer["n"].(int64) != (1<<53)+1 {
		t.Errorf("nested map int: got %v", outer["n"])
	}
	list := m["list"].([]any)
	if list[0].(int64) != (1<<53)+1 {
		t.Errorf("nested slice int: got %v", list[0])
	}
	if _, ok := list[1].(float64); !ok {
		t.Errorf("nested slice float: got %T", list[1])
	}
}
