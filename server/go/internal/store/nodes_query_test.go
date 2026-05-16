// Unit tests for the comparison operators on QueryNodes (issue #501).
// The handler-level translation is covered in internal/api; this file
// pins the SQL-emit / scan path at the store layer.

package store_test

import (
	"context"
	"sort"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// seedRangeNodes opens a fresh tenant, registers a query index on
// field_id 2 (the “age“ field used by the tests below), and inserts
// the supplied (id, age) pairs as type_id=1 nodes.
func seedRangeNodes(t *testing.T, cs *store.CanonicalStore, rows map[string]int64) {
	t.Helper()
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, "t1"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	if err := cs.EnsureQueryIndex(ctx, "t1", 1, []uint32{2}); err != nil {
		t.Fatalf("EnsureQueryIndex: %v", err)
	}
	for id, age := range rows {
		if _, err := cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
			NodeID:     id,
			TypeID:     1,
			OwnerActor: "u",
			Payload:    map[string]any{"1": "x", "2": age},
		}); err != nil {
			t.Fatalf("CreateNodeRaw %s: %v", id, err)
		}
	}
}

func ids(nodes []*store.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.NodeID)
	}
	sort.Strings(out)
	return out
}

// TestQueryNodes_ComparisonOperators exercises each operator on the
// indexed “age“ field. Subtests run sequentially because they share
// one tenant.
func TestQueryNodes_ComparisonOperators(t *testing.T) {
	cs := newStore(t)
	seedRangeNodes(t, cs, map[string]int64{"a": 10, "b": 20, "c": 30})

	cases := []struct {
		name    string
		op      store.QueryFilterOp
		value   any
		wantIDs []string
	}{
		{"eq", store.QueryFilterEq, int64(20), []string{"b"}},
		{"ne", store.QueryFilterNe, int64(20), []string{"a", "c"}},
		{"lt", store.QueryFilterLt, int64(20), []string{"a"}},
		{"le", store.QueryFilterLe, int64(20), []string{"a", "b"}},
		{"gt", store.QueryFilterGt, int64(20), []string{"c"}},
		{"ge", store.QueryFilterGe, int64(20), []string{"b", "c"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := cs.QueryNodes(context.Background(), store.QueryNodesArgs{
				TenantID: "t1",
				TypeID:   1,
				Filters: []store.QueryFilter{
					{FieldID: 2, Op: tc.op, Value: tc.value},
				},
				Limit: 100,
			})
			if err != nil {
				t.Fatalf("QueryNodes: %v", err)
			}
			gotIDs := ids(got)
			if len(gotIDs) != len(tc.wantIDs) {
				t.Fatalf("got %v, want %v", gotIDs, tc.wantIDs)
			}
			for i := range tc.wantIDs {
				if gotIDs[i] != tc.wantIDs[i] {
					t.Fatalf("got %v, want %v", gotIDs, tc.wantIDs)
				}
			}
		})
	}
}

// TestQueryNodes_TwoFiltersANDed pins the "all filters AND-ed"
// contract: “age >= 15 AND age < 30“ returns the in-range subset.
func TestQueryNodes_TwoFiltersANDed(t *testing.T) {
	cs := newStore(t)
	seedRangeNodes(t, cs, map[string]int64{"a": 10, "b": 20, "c": 30})
	got, err := cs.QueryNodes(context.Background(), store.QueryNodesArgs{
		TenantID: "t1",
		TypeID:   1,
		Filters: []store.QueryFilter{
			{FieldID: 2, Op: store.QueryFilterGe, Value: int64(15)},
			{FieldID: 2, Op: store.QueryFilterLt, Value: int64(30)},
		},
		Limit: 100,
	})
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	if g := ids(got); len(g) != 1 || g[0] != "b" {
		t.Fatalf("got %v, want [b]", g)
	}
}

// TestQueryNodes_FilterPlusEqualityFilters pins that the legacy
// EqualityFilters map AND-s correctly with the new Filters list. The
// existing equality call shape must keep working unchanged.
func TestQueryNodes_FilterPlusEqualityFilters(t *testing.T) {
	cs := newStore(t)
	seedRangeNodes(t, cs, map[string]int64{"a": 10, "b": 20, "c": 30})
	got, err := cs.QueryNodes(context.Background(), store.QueryNodesArgs{
		TenantID:        "t1",
		TypeID:          1,
		EqualityFilters: map[uint32]any{1: "x"}, // matches every row
		Filters: []store.QueryFilter{
			{FieldID: 2, Op: store.QueryFilterLe, Value: int64(20)},
		},
		Limit: 100,
	})
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	want := []string{"a", "b"}
	if g := ids(got); len(g) != 2 || g[0] != want[0] || g[1] != want[1] {
		t.Fatalf("got %v, want %v", g, want)
	}
}

// TestQueryNodes_UnindexedFieldStillWorks pins that range queries on
// an unindexed payload field still return the correct rows — the SQL
// just falls back to a scan rather than the expression index. This
// matches the documented contract: “indexed: true“ is an optimisation,
// not a correctness requirement.
func TestQueryNodes_UnindexedFieldStillWorks(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, "t1"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	// NOTE: no EnsureQueryIndex — field_id 2 is unindexed here.
	for id, age := range map[string]int64{"a": 10, "b": 20, "c": 30} {
		if _, err := cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
			NodeID:     id,
			TypeID:     1,
			OwnerActor: "u",
			Payload:    map[string]any{"2": age},
		}); err != nil {
			t.Fatalf("CreateNodeRaw %s: %v", id, err)
		}
	}
	got, err := cs.QueryNodes(ctx, store.QueryNodesArgs{
		TenantID: "t1",
		TypeID:   1,
		Filters: []store.QueryFilter{
			{FieldID: 2, Op: store.QueryFilterGt, Value: int64(15)},
		},
		Limit: 100,
	})
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	want := []string{"b", "c"}
	if g := ids(got); len(g) != 2 || g[0] != want[0] || g[1] != want[1] {
		t.Fatalf("got %v, want %v", g, want)
	}
}

// TestQueryNodes_LimitBounds pins that Limit caps the result count
// even when more rows would match the predicate — required for the
// sweeper "batch of N" pattern that motivated issue #501.
func TestQueryNodes_LimitBounds(t *testing.T) {
	cs := newStore(t)
	rows := map[string]int64{}
	for i := 0; i < 10; i++ {
		rows[string(rune('a'+i))] = int64(i)
	}
	seedRangeNodes(t, cs, rows)
	got, err := cs.QueryNodes(context.Background(), store.QueryNodesArgs{
		TenantID: "t1",
		TypeID:   1,
		Filters: []store.QueryFilter{
			{FieldID: 2, Op: store.QueryFilterLt, Value: int64(100)},
		},
		Limit: 3,
	})
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
}
