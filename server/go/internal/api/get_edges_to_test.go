// Tests for the GetEdgesTo RPC. Spec: docs/go-port/rpcs/GetEdgesTo.md.
//
// Coverage:
//
//   - test_grpc_contract.py:248-254 — empty seed ⇒ edges == [], OK.
//   - fan-in count: two edges to one target ⇒ length 2.
//   - edge_type_id filter narrows the fan-in.
//   - empty fan-in for a missing dst node.
//   - `except Exception` collapse: any internal error degrades to
//     GetEdgesResponse{} with codes.OK.

package api_test

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// newEdgeStore returns a tmpdir-backed CanonicalStore with WAL on,
// pre-opened for tenantID. Closed automatically on test teardown.
func newEdgeStore(t *testing.T, tenantID string) *store.CanonicalStore {
	t.Helper()
	cs, err := store.New(store.Options{
		RootDir: t.TempDir(),
		WALMode: true,
	})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	if err := cs.OpenTenant(context.Background(), tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	return cs
}

// edgesToReq is a small builder so each case reads as the wire payload.
func edgesToReq(tenantID, nodeID string, edgeTypeID, limit int32) *pb.GetEdgesRequest {
	return &pb.GetEdgesRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: "user:alice"},
		NodeId:     nodeID,
		EdgeTypeId: edgeTypeID,
		Limit:      limit,
	}
}

// TestGetEdgesTo_EmptyReturnsNoEdges pins the contract from
// test_grpc_contract.py:248-254: a tenant with no edges yields
// edges=[] / has_more=false / codes.OK.
func TestGetEdgesTo_EmptyReturnsNoEdges(t *testing.T) {
	t.Parallel()
	const tenantID = "tenant-empty"
	cs := newEdgeStore(t, tenantID)

	srv := api.New(api.WithStore(cs))
	resp, err := srv.GetEdgesTo(context.Background(), edgesToReq(tenantID, "n-target", 0, 0))
	if err != nil {
		t.Fatalf("GetEdgesTo: unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("GetEdgesTo: response is nil")
	}
	if got := len(resp.GetEdges()); got != 0 {
		t.Errorf("Edges: got %d, want 0", got)
	}
	if resp.GetHasMore() {
		t.Errorf("HasMore: got true, want false on empty fan-in")
	}
}

// TestGetEdgesTo_SingleEdgeRoundtrip pins the wire shape: one inbound
// edge round-trips with from_node_id / to_node_id / edge_type_id /
// tenant_id intact. Mirrors sdk/go/entdb/grpc_transport_test.go:694-712.
func TestGetEdgesTo_SingleEdgeRoundtrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const tenantID = "tenant-single"
	const target = "post-1"

	cs := newEdgeStore(t, tenantID)
	if _, err := cs.CreateEdge(ctx, tenantID, store.EdgeInput{
		EdgeTypeID: 7,
		FromNodeID: "user-alice",
		ToNodeID:   target,
		Props:      map[string]any{"weight": 0.5},
	}); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	srv := api.New(api.WithStore(cs))
	resp, err := srv.GetEdgesTo(ctx, edgesToReq(tenantID, target, 0, 0))
	if err != nil {
		t.Fatalf("GetEdgesTo: unexpected error: %v", err)
	}
	if got := len(resp.GetEdges()); got != 1 {
		t.Fatalf("Edges: got %d, want 1", got)
	}
	e := resp.GetEdges()[0]
	if got, want := e.GetTenantId(), tenantID; got != want {
		t.Errorf("TenantId = %q, want %q", got, want)
	}
	if got, want := e.GetEdgeTypeId(), int32(7); got != want {
		t.Errorf("EdgeTypeId = %d, want %d", got, want)
	}
	if got, want := e.GetFromNodeId(), "user-alice"; got != want {
		t.Errorf("FromNodeId = %q, want %q", got, want)
	}
	if got, want := e.GetToNodeId(), target; got != want {
		t.Errorf("ToNodeId = %q, want %q", got, want)
	}
	if e.GetProps() == nil {
		t.Errorf("Props: got nil, want non-nil Struct (always emitted)")
	} else if w, ok := e.GetProps().GetFields()["weight"]; !ok {
		t.Errorf("Props missing key 'weight'; have %v", e.GetProps().GetFields())
	} else if w.GetNumberValue() != 0.5 {
		t.Errorf("Props.weight = %v, want 0.5", w.GetNumberValue())
	}
	if resp.GetHasMore() {
		t.Errorf("HasMore: got true, want false (only 1 edge, default limit)")
	}
}

// TestGetEdgesTo_MultipleEdgesFanIn pins the fan-in count contract:
// two edges directed at the same target ⇒ length 2.
func TestGetEdgesTo_MultipleEdgesFanIn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const tenantID = "tenant-fanin"
	const target = "post-popular"

	cs := newEdgeStore(t, tenantID)
	for _, src := range []string{"user-alice", "user-bob", "user-carol"} {
		if _, err := cs.CreateEdge(ctx, tenantID, store.EdgeInput{
			EdgeTypeID: 1,
			FromNodeID: src,
			ToNodeID:   target,
		}); err != nil {
			t.Fatalf("CreateEdge(%s): %v", src, err)
		}
	}
	// One unrelated outbound edge from the target must NOT show up
	// (cross-direction isolation).
	if _, err := cs.CreateEdge(ctx, tenantID, store.EdgeInput{
		EdgeTypeID: 1,
		FromNodeID: target,
		ToNodeID:   "user-dave",
	}); err != nil {
		t.Fatalf("CreateEdge(outbound): %v", err)
	}

	srv := api.New(api.WithStore(cs))
	resp, err := srv.GetEdgesTo(ctx, edgesToReq(tenantID, target, 0, 0))
	if err != nil {
		t.Fatalf("GetEdgesTo: unexpected error: %v", err)
	}
	if got := len(resp.GetEdges()); got != 3 {
		t.Fatalf("Edges: got %d, want 3", got)
	}
	for _, e := range resp.GetEdges() {
		if e.GetToNodeId() != target {
			t.Errorf("ToNodeId = %q, want %q (fan-in must filter by to)",
				e.GetToNodeId(), target)
		}
	}
}

// TestGetEdgesTo_MissingDstReturnsEmpty: a node_id that has no inbound
// edges (here: never created) yields an empty result, NOT an error.
// Also covers the "ghost"-target case from the spec.
func TestGetEdgesTo_MissingDstReturnsEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const tenantID = "tenant-ghost"

	cs := newEdgeStore(t, tenantID)
	// Seed an edge to a different target so the table is non-empty;
	// the lookup must still return [] for the requested ghost.
	if _, err := cs.CreateEdge(ctx, tenantID, store.EdgeInput{
		EdgeTypeID: 1,
		FromNodeID: "src",
		ToNodeID:   "real-target",
	}); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	srv := api.New(api.WithStore(cs))
	resp, err := srv.GetEdgesTo(ctx, edgesToReq(tenantID, "ghost", 0, 0))
	if err != nil {
		t.Fatalf("GetEdgesTo: unexpected error: %v", err)
	}
	if got := len(resp.GetEdges()); got != 0 {
		t.Errorf("Edges: got %d, want 0 for ghost target", got)
	}
	if resp.GetHasMore() {
		t.Errorf("HasMore: got true, want false")
	}
}

// TestGetEdgesTo_EdgeTypeFilter pins the workplace-chat / workplace-
// posts contract: passing edge_type_id narrows fan-in to that single
// edge type.
func TestGetEdgesTo_EdgeTypeFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const tenantID = "tenant-filter"
	const target = "post-1"

	cs := newEdgeStore(t, tenantID)
	// Two upvotes (type 1) and one reply (type 2) all point at the
	// same target. The filter must return only the two upvotes.
	for _, src := range []string{"user-alice", "user-bob"} {
		if _, err := cs.CreateEdge(ctx, tenantID, store.EdgeInput{
			EdgeTypeID: 1, // UPVOTE
			FromNodeID: src,
			ToNodeID:   target,
		}); err != nil {
			t.Fatalf("CreateEdge(upvote %s): %v", src, err)
		}
	}
	if _, err := cs.CreateEdge(ctx, tenantID, store.EdgeInput{
		EdgeTypeID: 2, // REPLY
		FromNodeID: "user-carol",
		ToNodeID:   target,
	}); err != nil {
		t.Fatalf("CreateEdge(reply): %v", err)
	}

	srv := api.New(api.WithStore(cs))

	// edge_type_id=1 ⇒ only the two upvotes.
	resp, err := srv.GetEdgesTo(ctx, edgesToReq(tenantID, target, 1, 0))
	if err != nil {
		t.Fatalf("GetEdgesTo type=1: %v", err)
	}
	if got := len(resp.GetEdges()); got != 2 {
		t.Fatalf("Edges (type=1): got %d, want 2", got)
	}
	for _, e := range resp.GetEdges() {
		if e.GetEdgeTypeId() != 1 {
			t.Errorf("EdgeTypeId = %d, want 1 (filter leak)", e.GetEdgeTypeId())
		}
	}

	// edge_type_id=2 ⇒ only the single reply.
	resp2, err := srv.GetEdgesTo(ctx, edgesToReq(tenantID, target, 2, 0))
	if err != nil {
		t.Fatalf("GetEdgesTo type=2: %v", err)
	}
	if got := len(resp2.GetEdges()); got != 1 {
		t.Fatalf("Edges (type=2): got %d, want 1", got)
	}
	if resp2.GetEdges()[0].GetEdgeTypeId() != 2 {
		t.Errorf("EdgeTypeId = %d, want 2", resp2.GetEdges()[0].GetEdgeTypeId())
	}

	// edge_type_id=0 ⇒ unfiltered, all three rows.
	respAll, err := srv.GetEdgesTo(ctx, edgesToReq(tenantID, target, 0, 0))
	if err != nil {
		t.Fatalf("GetEdgesTo type=0: %v", err)
	}
	if got := len(respAll.GetEdges()); got != 3 {
		t.Fatalf("Edges (unfiltered): got %d, want 3", got)
	}

	// edge_type_id=99 (unknown) ⇒ empty.
	respMiss, err := srv.GetEdgesTo(ctx, edgesToReq(tenantID, target, 99, 0))
	if err != nil {
		t.Fatalf("GetEdgesTo type=99: %v", err)
	}
	if got := len(respMiss.GetEdges()); got != 0 {
		t.Errorf("Edges (type=99): got %d, want 0", got)
	}
}

// TestGetEdgesTo_StoreErrorReturnsEmpty pins the swallow-and-empty
// contract: any internal storage fault collapses to GetEdgesResponse{}
// with codes.OK and metric label "error". Driven here by skipping
// OpenTenant so the read-side pool returns ErrTenantNotOpen.
func TestGetEdgesTo_StoreErrorReturnsEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cs, err := store.New(store.Options{RootDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	// Deliberately skip OpenTenant — the read-side pool lookup will
	// error and the handler must collapse that to edges=[].

	srv := api.New(api.WithStore(cs))
	resp, err := srv.GetEdgesTo(ctx, edgesToReq("tenant-unopened", "n", 0, 0))
	if err != nil {
		t.Fatalf("GetEdgesTo: must return OK + empty body, not gRPC error: %v", err)
	}
	if resp == nil {
		t.Fatalf("response is nil")
	}
	if got := len(resp.GetEdges()); got != 0 {
		t.Errorf("Edges: got %d, want 0 on store error", got)
	}
	if resp.GetHasMore() {
		t.Errorf("HasMore: got true, want false on store error")
	}
}
