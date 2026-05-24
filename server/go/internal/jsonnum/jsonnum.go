// SPDX-License-Identifier: AGPL-3.0-only

// Package jsonnum provides a canonical JSON decode that preserves the
// full int64 range (ADR-028 / Bug C #563). Go's default
// json.Unmarshal into interface{} collapses every number to float64,
// which silently corrupts int64 values above 2^53. This package decodes
// with json.Decoder.UseNumber() and then normalizes json.Number to a
// real int64 (when the value is integral) or float64 otherwise.
//
// It MUST be used consistently at every JSON payload boundary —
// wal.DecodeEvent, the applier's read-modify-write merge, and every
// payload_json egress decode — so that values compared across those
// boundaries (e.g. update_node CAS via reflect.DeepEqual) share one
// representation. Mixing this with a plain float64 decode reintroduces
// the type-mismatch bugs it exists to prevent.
package jsonnum

import (
	"bytes"
	"encoding/json"
)

// Decode unmarshals a JSON object into map[string]any with integers
// preserved as int64 (not float64). A nil/empty input yields an empty,
// non-nil map so callers can range over it unconditionally.
func Decode(data []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	for k, v := range m {
		m[k] = Normalize(v)
	}
	return m, nil
}

// Normalize recursively rewrites json.Number to int64 (when the value is
// an exact integer) or float64 (otherwise), and recurses through maps
// and slices. Non-number values pass through unchanged. This is the
// canonical in-memory representation for decoded payload values.
func Normalize(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	case map[string]any:
		for k, val := range x {
			x[k] = Normalize(val)
		}
		return x
	case []any:
		for i, val := range x {
			x[i] = Normalize(val)
		}
		return x
	default:
		return v
	}
}
