// SPDX-License-Identifier: MIT
package entdb

import (
	"testing"
)

// TestFiltersToMap_SingleEquality pins that a single Eq filter
// produces the same map shape today's equality call site emits — a
// plain field=value entry, not a nested “{"$eq": v}“ dict.
func TestFiltersToMap_SingleEquality(t *testing.T) {
	got := filtersToMap([]Filter{
		{Field: "sku", Op: FilterEq, Value: "WIDGET-1"},
	})
	want := map[string]any{"sku": "WIDGET-1"}
	if got["sku"] != want["sku"] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestFiltersToMap_RangePair pins the half-open-range shape that
// motivates issue #501: “expires_at < now“ → nested “{"$lt": now}“.
func TestFiltersToMap_RangePair(t *testing.T) {
	got := filtersToMap([]Filter{
		{Field: "expires_at", Op: FilterGe, Value: int64(100)},
		{Field: "expires_at", Op: FilterLt, Value: int64(200)},
	})
	sub, ok := got["expires_at"].(map[string]any)
	if !ok {
		t.Fatalf("expires_at is not a map: %T", got["expires_at"])
	}
	if sub["$gte"] != int64(100) || sub["$lt"] != int64(200) {
		t.Fatalf("got %v, want $gte=100,$lt=200", sub)
	}
}

// TestFiltersToMap_NeAndLt pins that two filters on different fields
// produce two distinct entries — they are AND-ed by the server.
func TestFiltersToMap_NeAndLt(t *testing.T) {
	got := filtersToMap([]Filter{
		{Field: "status", Op: FilterNe, Value: "deleted"},
		{Field: "age", Op: FilterLt, Value: 30},
	})
	if sub, ok := got["status"].(map[string]any); !ok || sub["$ne"] != "deleted" {
		t.Fatalf("status filter: got %v", got["status"])
	}
	if sub, ok := got["age"].(map[string]any); !ok || sub["$lt"] != 30 {
		t.Fatalf("age filter: got %v", got["age"])
	}
}

// TestFiltersToMap_Empty pins that an empty slice yields nil (no
// filter on the wire).
func TestFiltersToMap_Empty(t *testing.T) {
	if got := filtersToMap(nil); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
	if got := filtersToMap([]Filter{}); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}
