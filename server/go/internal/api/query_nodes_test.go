// Tests for the QueryNodes RPC (W2 — EPIC #407).
//
// Spec: docs/go-port/rpcs/QueryNodes.md.
//
// Six branches pinned here:
//
//  1. Equality filter on a payload field returns the matching subset.
//  2. Pagination + sort: descending order_by + offset returns disjoint
//     pages.
//  3. ACL post-filter excludes nodes the cross-tenant caller cannot
//     read; nodes with a direct grant survive.
//  4. Unsupported FilterOp (GTE) -> codes.InvalidArgument. This is the
//     deliberate behaviour FIX over Python's silent exception swallow
//     called out in the spec "Open questions" §5.
//  5. Inlined-operator value (Op=EQ, Value={"$gte": ...}) -> codes.
//     InvalidArgument. Same parity-fix rationale as (4).
//  6. Sort by node_id ascending returns rows in lexical order.
//
// Tests live in `package api_test` per the W2 task brief.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// queryNodesFixture wires a fresh store, globalstore, and registry.
// The tenant is created in the globalstore (so checkTenant passes) and
// the canonical-store side is opened too. Three User-typed nodes are
// seeded with distinct payload values for filter/sort/page tests.
type queryNodesFixture struct {
	t        *testing.T
	cs       *store.CanonicalStore
	registry *schema.Registry
	srv      *api.Server
	tenantID string
}

func newQueryNodesFixture(t *testing.T) *queryNodesFixture {
	t.Helper()

	cs, err := store.New(store.Options{RootDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	const tenantID = "tenant-q"
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(ctx, tenantID, "Tenant Q", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	reg := schema.NewRegistry()
	if err := reg.RegisterNode(&schema.NodeTypeDef{
		TypeID: 1,
		Name:   "User",
		Fields: []schema.FieldDef{
			{FieldID: 1, Name: "email", Kind: schema.KindString},
			{FieldID: 2, Name: "age", Kind: schema.KindInteger},
		},
	}); err != nil {
		t.Fatalf("registry register: %v", err)
	}
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("registry freeze: %v", err)
	}

	srv := api.New(
		api.WithStore(cs),
		api.WithGlobalStore(gs),
		api.WithSchemaRegistry(reg),
	)

	return &queryNodesFixture{
		t:        t,
		cs:       cs,
		registry: reg,
		srv:      srv,
		tenantID: tenantID,
	}
}

func (f *queryNodesFixture) seedNode(id, owner, email string, age int64) {
	f.t.Helper()
	_, err := f.cs.CreateNodeRaw(context.Background(), f.tenantID, store.NodeInput{
		NodeID:     id,
		TypeID:     1,
		OwnerActor: owner,
		Payload:    map[string]any{"1": email, "2": age},
	})
	if err != nil {
		f.t.Fatalf("CreateNodeRaw %s: %v", id, err)
	}
}

func (f *queryNodesFixture) seedNodeACL(id, owner, email string, age int64, acl []store.ACLEntry) {
	f.t.Helper()
	_, err := f.cs.CreateNodeRaw(context.Background(), f.tenantID, store.NodeInput{
		NodeID:     id,
		TypeID:     1,
		OwnerActor: owner,
		Payload:    map[string]any{"1": email, "2": age},
		ACL:        acl,
	})
	if err != nil {
		f.t.Fatalf("CreateNodeRaw %s: %v", id, err)
	}
}

// mustValue is a tiny structpb constructor for filter values.
func mustValue(t *testing.T, v any) *structpb.Value {
	t.Helper()
	out, err := structpb.NewValue(v)
	if err != nil {
		t.Fatalf("structpb.NewValue(%v): %v", v, err)
	}
	return out
}

// TestQueryNodes_EqualityFilter pins the filter-by-name path: a single
// EQ filter on `email` (resolved to field_id 1 by FilterNamesToIDs)
// returns only the matching subset.
func TestQueryNodes_EqualityFilter(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)

	// owner is the same actor on every node so the same-tenant ACL
	// filter does not gate them out.
	f.seedNode("n1", "user:alice", "alice@example.com", 30)
	f.seedNode("n2", "user:alice", "bob@example.com", 25)
	f.seedNode("n3", "user:alice", "alice@example.com", 40)

	resp, err := f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
		TypeId:  1,
		Filters: []*pb.FieldFilter{
			{Field: "email", Op: pb.FilterOp_EQ, Value: mustValue(t, "alice@example.com")},
		},
	})
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	if len(resp.GetNodes()) != 2 {
		t.Fatalf("nodes: got %d, want 2 (alice@example.com matches n1 + n3)", len(resp.GetNodes()))
	}
	for _, n := range resp.GetNodes() {
		if n.GetNodeId() != "n1" && n.GetNodeId() != "n3" {
			t.Fatalf("unexpected node_id %q in result", n.GetNodeId())
		}
	}
}

// TestQueryNodes_RangeOperators exercises the comparison operators
// added by issue #501. Each branch seeds the same three rows and
// asserts that the matching subset is returned. CONTAINS / IN are
// covered separately by TestQueryNodes_UnsupportedOperator.
func TestQueryNodes_RangeOperators(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		op      pb.FilterOp
		value   any
		wantIDs []string
	}{
		{"lt", pb.FilterOp_LT, 30, []string{"n2"}},   // age < 30 → n2 (25)
		{"lte", pb.FilterOp_LTE, 30, []string{"n1", "n2"}},
		{"gt", pb.FilterOp_GT, 30, []string{"n3"}}, // age > 30 → n3 (40)
		{"gte", pb.FilterOp_GTE, 30, []string{"n1", "n3"}},
		{"neq", pb.FilterOp_NEQ, 30, []string{"n2", "n3"}},
		{"eq", pb.FilterOp_EQ, 30, []string{"n1"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newQueryNodesFixture(t)
			f.seedNode("n1", "user:alice", "alice@example.com", 30)
			f.seedNode("n2", "user:alice", "bob@example.com", 25)
			f.seedNode("n3", "user:alice", "carol@example.com", 40)

			resp, err := f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
				Context: &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
				TypeId:  1,
				OrderBy: "node_id",
				Filters: []*pb.FieldFilter{
					{Field: "age", Op: tc.op, Value: mustValue(t, tc.value)},
				},
			})
			if err != nil {
				t.Fatalf("QueryNodes: %v", err)
			}
			got := make([]string, 0, len(resp.GetNodes()))
			for _, n := range resp.GetNodes() {
				got = append(got, n.GetNodeId())
			}
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("got %v, want %v", got, tc.wantIDs)
			}
			for i := range tc.wantIDs {
				if got[i] != tc.wantIDs[i] {
					t.Fatalf("got %v, want %v", got, tc.wantIDs)
				}
			}
		})
	}
}

// TestQueryNodes_RangeOperatorsANDed pins the "all filters AND-ed"
// contract: ``age >= 25 AND age < 40`` is expressed as two filters
// on the same field and returns only the rows that satisfy both.
func TestQueryNodes_RangeOperatorsANDed(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)
	f.seedNode("n1", "user:alice", "alice@example.com", 30)
	f.seedNode("n2", "user:alice", "bob@example.com", 25)
	f.seedNode("n3", "user:alice", "carol@example.com", 40)

	resp, err := f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
		TypeId:  1,
		OrderBy: "node_id",
		Filters: []*pb.FieldFilter{
			{Field: "age", Op: pb.FilterOp_GTE, Value: mustValue(t, 25)},
			{Field: "age", Op: pb.FilterOp_LT, Value: mustValue(t, 40)},
		},
	})
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	if len(resp.GetNodes()) != 2 {
		t.Fatalf("got %d nodes, want 2", len(resp.GetNodes()))
	}
	want := map[string]struct{}{"n1": {}, "n2": {}}
	for _, n := range resp.GetNodes() {
		if _, ok := want[n.GetNodeId()]; !ok {
			t.Fatalf("unexpected node %q in result", n.GetNodeId())
		}
	}
}

// TestQueryNodes_UnsupportedOperator pins the still-rejected operators.
// CONTAINS and IN remain INVALID_ARGUMENT pending the W1.10 queryfilter
// package; issue #501 explicitly scoped them out.
func TestQueryNodes_UnsupportedOperator(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)
	f.seedNode("n1", "user:alice", "alice@example.com", 30)

	for _, op := range []pb.FilterOp{pb.FilterOp_CONTAINS, pb.FilterOp_IN} {
		op := op
		t.Run(op.String(), func(t *testing.T) {
			t.Parallel()
			_, err := f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
				Context: &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
				TypeId:  1,
				Filters: []*pb.FieldFilter{
					{Field: "age", Op: op, Value: mustValue(t, 25)},
				},
			})
			if err == nil {
				t.Fatalf("expected INVALID_ARGUMENT for %s, got nil", op)
			}
			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("error is not a grpc status: %v", err)
			}
			if st.Code() != codes.InvalidArgument {
				t.Fatalf("code: got %v, want InvalidArgument", st.Code())
			}
		})
	}
}

// TestQueryNodes_PaginationCursor pins the (limit, offset) pagination
// shape. With three nodes and limit=2 we expect two disjoint pages and
// has_more=true on the first, has_more=false on the second.
func TestQueryNodes_PaginationCursor(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)
	f.seedNode("n1", "user:alice", "a@x", 10)
	f.seedNode("n2", "user:alice", "b@x", 20)
	f.seedNode("n3", "user:alice", "c@x", 30)

	first, err := f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
		TypeId:  1,
		Limit:   2,
		OrderBy: "node_id",
	})
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if len(first.GetNodes()) != 2 {
		t.Fatalf("page 1: got %d nodes, want 2", len(first.GetNodes()))
	}
	if !first.GetHasMore() {
		t.Fatalf("page 1: has_more = false, want true (len == limit)")
	}

	second, err := f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
		TypeId:  1,
		Limit:   2,
		Offset:  2,
		OrderBy: "node_id",
	})
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(second.GetNodes()) != 1 {
		t.Fatalf("page 2: got %d nodes, want 1", len(second.GetNodes()))
	}
	if second.GetHasMore() {
		t.Fatalf("page 2: has_more = true, want false (len < limit)")
	}
	// Disjoint pages.
	if first.GetNodes()[0].GetNodeId() == second.GetNodes()[0].GetNodeId() {
		t.Fatalf("pages overlap: %s appears on both pages",
			first.GetNodes()[0].GetNodeId())
	}
}

// TestQueryNodes_SortByNodeID pins the order_by allow-list path.
// Ascending sort on node_id returns rows in lexical order.
func TestQueryNodes_SortByNodeID(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)
	f.seedNode("c", "user:alice", "c@x", 10)
	f.seedNode("a", "user:alice", "a@x", 20)
	f.seedNode("b", "user:alice", "b@x", 30)

	resp, err := f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
		TypeId:  1,
		OrderBy: "node_id",
	})
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	got := make([]string, 0, len(resp.GetNodes()))
	for _, n := range resp.GetNodes() {
		got = append(got, n.GetNodeId())
	}
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len(nodes) = %d, want %d (got %v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("nodes[%d] = %q, want %q (full got=%v)", i, got[i], want[i], got)
		}
	}
}

// TestQueryNodes_ACLPostFilter pins the cross-tenant post-filter
// behaviour. A node owned by user:alice is only visible to user:bob if
// bob has an explicit ACL grant; nodes without such a grant are
// excluded from the response.
func TestQueryNodes_ACLPostFilter(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)
	// n1: alice owner, no extra grants — bob cannot see it.
	f.seedNode("n1", "user:alice", "alice@example.com", 30)
	// n2: alice owner, but bob is granted read access — visible to bob.
	f.seedNodeACL("n2", "user:alice", "bob-grant@example.com", 25,
		[]store.ACLEntry{{Principal: "user:bob", Permission: "read"}})

	resp, err := f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: f.tenantID, Actor: "user:bob"},
		TypeId:  1,
		OrderBy: "node_id",
	})
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	if len(resp.GetNodes()) != 1 {
		ids := make([]string, len(resp.GetNodes()))
		for i, n := range resp.GetNodes() {
			ids[i] = n.GetNodeId()
		}
		t.Fatalf("nodes: got %d (%v), want 1 (only n2 has a grant for bob)",
			len(resp.GetNodes()), ids)
	}
	if got := resp.GetNodes()[0].GetNodeId(); got != "n2" {
		t.Fatalf("nodes[0].node_id = %q, want %q", got, "n2")
	}
}

// TestQueryNodes_InlinedOperator pins the inlined-operator shape: a
// Struct value carrying ``$gte`` / ``$lt`` / ... keys fans into one
// store filter per inlined entry. This is the wire shape the Python
// SDK has historically emitted; issue #501 wires the server to accept
// it natively.
func TestQueryNodes_InlinedOperator(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)
	f.seedNode("n1", "user:alice", "a@x", 10)
	f.seedNode("n2", "user:alice", "b@x", 20)
	f.seedNode("n3", "user:alice", "c@x", 30)

	inlined, err := structpb.NewStruct(map[string]any{
		"$gte": float64(15),
		"$lt":  float64(30),
	})
	if err != nil {
		t.Fatalf("structpb.NewStruct: %v", err)
	}

	resp, err := f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
		TypeId:  1,
		OrderBy: "node_id",
		Filters: []*pb.FieldFilter{
			{
				Field: "age",
				Op:    pb.FilterOp_EQ,
				Value: structpb.NewStructValue(inlined),
			},
		},
	})
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	if len(resp.GetNodes()) != 1 || resp.GetNodes()[0].GetNodeId() != "n2" {
		ids := make([]string, len(resp.GetNodes()))
		for i, n := range resp.GetNodes() {
			ids[i] = n.GetNodeId()
		}
		t.Fatalf("got %v, want [n2]", ids)
	}
}

// TestQueryNodes_InlinedOperatorUnknownRejected pins that the inlined
// shape still rejects operators outside the Eq/Ne/Lt/Le/Gt/Ge set —
// the same INVALID_ARGUMENT path as ``$nin`` / ``$between``.
func TestQueryNodes_InlinedOperatorUnknownRejected(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)
	f.seedNode("n1", "user:alice", "a@x", 10)

	inlined, err := structpb.NewStruct(map[string]any{"$nin": float64(5)})
	if err != nil {
		t.Fatalf("structpb.NewStruct: %v", err)
	}

	_, err = f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
		TypeId:  1,
		Filters: []*pb.FieldFilter{
			{
				Field: "age",
				Op:    pb.FilterOp_EQ,
				Value: structpb.NewStructValue(inlined),
			},
		},
	})
	if err == nil {
		t.Fatalf("expected INVALID_ARGUMENT for unsupported inlined op, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error is not a grpc status: %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("code: got %v, want InvalidArgument", st.Code())
	}
}

// TestQueryNodes_UnknownTypeID pins the type-id validation path: when
// the registry is wired and the type_id is not registered, the handler
// surfaces INVALID_ARGUMENT rather than letting the store run the SQL
// against an empty result set.
func TestQueryNodes_UnknownTypeID(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)

	_, err := f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
		TypeId:  999,
	})
	if err == nil {
		t.Fatalf("expected INVALID_ARGUMENT for unknown type_id, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error is not a grpc status: %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("code: got %v, want InvalidArgument", st.Code())
	}
}

// TestQueryNodes_SchemaLessAllowsUnknownTypeID pins the profile=none
// contract used by entdb-server: with no registry wired, QueryNodes
// treats type_id as an opaque storage discriminator instead of rejecting
// it as unknown.
func TestQueryNodes_SchemaLessAllowsUnknownTypeID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cs := newCanonicalStore(t)
	gs := newGlobalStore(t)

	const tenantID = "tenant-schemaless"
	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	if _, err := gs.CreateTenant(ctx, tenantID, "Tenant Schemaless", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if _, err := cs.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID:     "node-508",
		TypeID:     508,
		OwnerActor: "user:alice",
		Payload:    map[string]any{"1": "schemaless"},
	}); err != nil {
		t.Fatalf("CreateNodeRaw: %v", err)
	}

	srv := api.New(
		api.WithStore(cs),
		api.WithGlobalStore(gs),
	)
	resp, err := srv.QueryNodes(ctx, &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "user:alice"},
		TypeId:  508,
	})
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	if got := len(resp.GetNodes()); got != 1 {
		t.Fatalf("nodes: got %d, want 1", got)
	}
	if got := resp.GetNodes()[0].GetNodeId(); got != "node-508" {
		t.Fatalf("node_id: got %q, want node-508", got)
	}
}
