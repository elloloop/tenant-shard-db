package entdb

import (
	"sort"
	"testing"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

// TestFilterToProto_Equality covers the bare-value shorthand —
// {"sku": "WIDGET-1"} ⇒ one EQ filter with the value as a Struct.
func TestFilterToProto_Equality(t *testing.T) {
	out, err := filterToProto(map[string]any{"sku": "WIDGET-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("want 1 filter, got %d", len(out))
	}
	got := out[0]
	if got.GetField() != "sku" {
		t.Errorf("field = %q, want %q", got.GetField(), "sku")
	}
	if got.GetOp() != pb.FilterOp_EQ {
		t.Errorf("op = %v, want EQ", got.GetOp())
	}
	if got.GetValue().GetStringValue() != "WIDGET-1" {
		t.Errorf("value = %v, want %q", got.GetValue(), "WIDGET-1")
	}
}

// TestFilterToProto_KnownOperators ensures every operator in the
// supported set lands on the wire with the correct FilterOp enum
// value and the correct argument value.
func TestFilterToProto_KnownOperators(t *testing.T) {
	cases := []struct {
		key string
		op  pb.FilterOp
	}{
		{"$eq", pb.FilterOp_EQ},
		{"$ne", pb.FilterOp_NEQ},
		{"$gt", pb.FilterOp_GT},
		{"$gte", pb.FilterOp_GTE},
		{"$lt", pb.FilterOp_LT},
		{"$lte", pb.FilterOp_LTE},
		{"$contains", pb.FilterOp_CONTAINS},
		{"$in", pb.FilterOp_IN},
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			out, err := filterToProto(map[string]any{
				"price": map[string]any{c.key: 100.0},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out) != 1 {
				t.Fatalf("want 1 filter, got %d", len(out))
			}
			if out[0].GetOp() != c.op {
				t.Errorf("op = %v, want %v", out[0].GetOp(), c.op)
			}
			if out[0].GetField() != "price" {
				t.Errorf("field = %q, want price", out[0].GetField())
			}
			if got := out[0].GetValue().GetNumberValue(); got != 100.0 {
				t.Errorf("value = %v, want 100", got)
			}
		})
	}
}

// TestFilterToProto_RangeQueryEmitsTwoFilters verifies that a
// two-bound range like {"price": {"$gte": 100, "$lte": 200}} produces
// exactly two FieldFilters — one per operator — both targeting the
// same field. Order is not guaranteed (Go map iteration is
// randomised), so we sort by op before asserting.
func TestFilterToProto_RangeQueryEmitsTwoFilters(t *testing.T) {
	out, err := filterToProto(map[string]any{
		"price": map[string]any{"$gte": 100.0, "$lte": 200.0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("want 2 filters, got %d", len(out))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].GetOp() < out[j].GetOp()
	})
	if out[0].GetOp() != pb.FilterOp_GTE || out[0].GetValue().GetNumberValue() != 100.0 {
		t.Errorf("first filter = (%v, %v), want (GTE, 100)",
			out[0].GetOp(), out[0].GetValue())
	}
	if out[1].GetOp() != pb.FilterOp_LTE || out[1].GetValue().GetNumberValue() != 200.0 {
		t.Errorf("second filter = (%v, %v), want (LTE, 200)",
			out[1].GetOp(), out[1].GetValue())
	}
}

// TestFilterToProto_UnknownOperatorIsDeterministic exercises the
// pass-through fallback for ops the wire enum cannot represent
// (e.g. $nin, $between, $and). Before the fix this code path was
// non-deterministic — Go's randomised map iteration meant the
// emitted FieldFilter list could mix per-op filters with a stray
// equality fallback depending on iteration order. The fix is to
// detect the unknown-op case up-front and emit a single
// pass-through filter; this test re-runs the conversion many times
// to surface any remaining order-dependent behaviour.
func TestFilterToProto_UnknownOperatorIsDeterministic(t *testing.T) {
	in := map[string]any{
		"price": map[string]any{"$between": []any{100.0, 200.0}, "$gte": 50.0},
	}
	for i := 0; i < 50; i++ {
		out, err := filterToProto(in)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if len(out) != 1 {
			t.Fatalf("iteration %d: want 1 pass-through filter, got %d",
				i, len(out))
		}
		if out[0].GetOp() != pb.FilterOp_EQ {
			t.Errorf("iteration %d: op = %v, want EQ pass-through",
				i, out[0].GetOp())
		}
		if out[0].GetField() != "price" {
			t.Errorf("iteration %d: field = %q, want price",
				i, out[0].GetField())
		}
		// The pass-through value carries the full original subtree
		// so the server can rebuild a MongoDB-style operator dict.
		sv := out[0].GetValue().GetStructValue()
		if sv == nil {
			t.Fatalf("iteration %d: expected struct value, got %v",
				i, out[0].GetValue())
		}
		if _, ok := sv.GetFields()["$between"]; !ok {
			t.Errorf("iteration %d: pass-through struct missing $between key",
				i)
		}
		if _, ok := sv.GetFields()["$gte"]; !ok {
			t.Errorf("iteration %d: pass-through struct missing $gte key",
				i)
		}
	}
}

// TestFilterToProto_EmptyFilterReturnsNil — defensive: a nil/empty
// filter must produce a nil slice so the gRPC layer can omit the
// repeated field rather than send an empty list.
func TestFilterToProto_EmptyFilterReturnsNil(t *testing.T) {
	out, err := filterToProto(nil)
	if err != nil {
		t.Fatalf("nil: unexpected error: %v", err)
	}
	if out != nil {
		t.Errorf("nil: want nil slice, got %v", out)
	}

	out, err = filterToProto(map[string]any{})
	if err != nil {
		t.Fatalf("empty: unexpected error: %v", err)
	}
	if out != nil {
		t.Errorf("empty: want nil slice, got %v", out)
	}
}
