// SPDX-License-Identifier: AGPL-3.0-only

// Tests for the GetConnectedNodes RPC (W2 — EPIC #407).
//
// Behaviour pinned here:
//   - Single-hop traversal returns every connected outbound node, ordered
//     by created_at DESC (mirrors test_grpc_contract.py:255-266).
//   - Cycle protection: a graph with a back-edge a→b→a does NOT cause
//     a→a to surface; visited-set keeps the source out of the result.
//   - Depth limit: BFS does not cross beyond MaxConnectedDepth (= 1
//     today), so b's outbound children are NOT yielded by a query
//     anchored at a (depth-2 ports remain a future EPIC concern; this
//     test guards against accidental recursion).
//   - ACL prune: a child node owned by a different user with no
//     visibility row is NOT returned, even though the edge exists.
//   - Unknown tenant (gate failure) propagates as a gRPC error rather
//     than a swallowed empty list — the Go port deliberately tightens
//     the "swallow everything" behaviour, per spec "Error
//     contract" recommendation.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/tenant"
)

// connectedTestServer wires a Server with a globalstore (so checkTenant
// passes for the "acme" tenant) plus a real CanonicalStore (for the
// graph traversal). Returns the server, the store handle, and a context.
func connectedTestServer(t *testing.T) (*api.Server, *store.CanonicalStore, context.Context) {
	t.Helper()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	cs, err := store.New(store.Options{RootDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	if err := cs.OpenTenant(ctx, "acme"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithStore(cs),
		api.WithSharding(&tenant.Sharding{NodeID: "n1"}),
		api.WithRegion("us-east-1"),
	)
	return srv, cs, ctx
}

// nodesByID materialises the response into a map for order-insensitive
// assertions. Tests that care about ordering check the slice directly.
func nodesByID(resp *pb.GetConnectedNodesResponse) map[string]*pb.Node {
	out := map[string]*pb.Node{}
	for _, n := range resp.GetNodes() {
		out[n.GetNodeId()] = n
	}
	return out
}

// TestGetConnectedNodes_SingleHop: source node with two outbound edges
// of the requested type returns both children. Both children are owned
// by the same actor as the source, so the ACL filter does not prune.
func TestGetConnectedNodes_SingleHop(t *testing.T) {
	t.Parallel()
	srv, cs, ctx := connectedTestServer(t)

	// Build: src -> child1, src -> child2, all owned by alice.
	mustCreateNode(t, cs, "acme", "src", 1, "user:alice")
	mustCreateNode(t, cs, "acme", "child1", 1, "user:alice")
	mustCreateNode(t, cs, "acme", "child2", 1, "user:alice")
	mustCreateEdge(t, cs, "acme", 7, "src", "child1")
	mustCreateEdge(t, cs, "acme", 7, "src", "child2")

	resp, err := srv.GetConnectedNodes(ctx, &pb.GetConnectedNodesRequest{
		Context:    &pb.RequestContext{TenantId: "acme", Actor: "user:alice"},
		NodeId:     "src",
		EdgeTypeId: 7,
		Limit:      50,
	})
	if err != nil {
		t.Fatalf("GetConnectedNodes: unexpected error: %v", err)
	}
	got := nodesByID(resp)
	if len(got) != 2 {
		t.Fatalf("got %d nodes, want 2: ids=%v", len(got), keys(got))
	}
	if _, ok := got["child1"]; !ok {
		t.Errorf("missing child1 (ids=%v)", keys(got))
	}
	if _, ok := got["child2"]; !ok {
		t.Errorf("missing child2 (ids=%v)", keys(got))
	}
	// Source must not appear in the result (cycle/self exclusion).
	if _, ok := got["src"]; ok {
		t.Errorf("source node leaked into result")
	}
}

// TestGetConnectedNodes_DepthLimitHonored: a 2-hop graph
// (src -> a -> grandchild) yields only `a`, not `grandchild`, because
// the BFS depth cap is 1 (the Python single-JOIN contract).
func TestGetConnectedNodes_DepthLimitHonored(t *testing.T) {
	t.Parallel()
	srv, cs, ctx := connectedTestServer(t)

	mustCreateNode(t, cs, "acme", "src", 1, "user:alice")
	mustCreateNode(t, cs, "acme", "a", 1, "user:alice")
	mustCreateNode(t, cs, "acme", "grandchild", 1, "user:alice")
	mustCreateEdge(t, cs, "acme", 7, "src", "a")
	mustCreateEdge(t, cs, "acme", 7, "a", "grandchild")

	resp, err := srv.GetConnectedNodes(ctx, &pb.GetConnectedNodesRequest{
		Context:    &pb.RequestContext{TenantId: "acme", Actor: "user:alice"},
		NodeId:     "src",
		EdgeTypeId: 7,
	})
	if err != nil {
		t.Fatalf("GetConnectedNodes: unexpected error: %v", err)
	}
	got := nodesByID(resp)
	if _, ok := got["a"]; !ok {
		t.Errorf("missing direct child a (ids=%v)", keys(got))
	}
	if _, ok := got["grandchild"]; ok {
		t.Errorf("grandchild leaked across depth limit (ids=%v)", keys(got))
	}
	if len(got) != 1 {
		t.Fatalf("got %d nodes, want 1 (only direct child): ids=%v", len(got), keys(got))
	}
}

// TestGetConnectedNodes_CycleProtection: a back-edge child -> src must
// not cause src to be re-yielded or the BFS to loop forever. With the
// depth cap at 1, this is more of a structural guard, but the visited
// set is the primitive that makes a future depth raise safe.
func TestGetConnectedNodes_CycleProtection(t *testing.T) {
	t.Parallel()
	srv, cs, ctx := connectedTestServer(t)

	mustCreateNode(t, cs, "acme", "src", 1, "user:alice")
	mustCreateNode(t, cs, "acme", "child", 1, "user:alice")
	mustCreateEdge(t, cs, "acme", 7, "src", "child")
	mustCreateEdge(t, cs, "acme", 7, "child", "src") // back-edge

	resp, err := srv.GetConnectedNodes(ctx, &pb.GetConnectedNodesRequest{
		Context:    &pb.RequestContext{TenantId: "acme", Actor: "user:alice"},
		NodeId:     "src",
		EdgeTypeId: 7,
	})
	if err != nil {
		t.Fatalf("GetConnectedNodes: unexpected error: %v", err)
	}
	got := nodesByID(resp)
	if _, ok := got["src"]; ok {
		t.Fatalf("src re-yielded via back-edge (ids=%v)", keys(got))
	}
	if _, ok := got["child"]; !ok {
		t.Fatalf("missing child (ids=%v)", keys(got))
	}
	if len(got) != 1 {
		t.Fatalf("got %d nodes, want 1: ids=%v", len(got), keys(got))
	}
}

// TestGetConnectedNodes_ACLFilterPrunesChild: bob owns the source and
// has been ACL'd onto it; one child is owned by bob (visible), the
// other by carol with no visibility (denied). Only the visible child
// must be returned.
func TestGetConnectedNodes_ACLFilterPrunesChild(t *testing.T) {
	t.Parallel()
	srv, cs, ctx := connectedTestServer(t)

	// src owned by bob; child_visible owned by bob; child_denied owned
	// by carol with no ACL grant for bob.
	mustCreateNode(t, cs, "acme", "src", 1, "user:bob")
	mustCreateNode(t, cs, "acme", "child_visible", 1, "user:bob")
	mustCreateNode(t, cs, "acme", "child_denied", 1, "user:carol")
	mustCreateEdge(t, cs, "acme", 7, "src", "child_visible")
	mustCreateEdge(t, cs, "acme", 7, "src", "child_denied")

	resp, err := srv.GetConnectedNodes(ctx, &pb.GetConnectedNodesRequest{
		Context:    &pb.RequestContext{TenantId: "acme", Actor: "user:bob"},
		NodeId:     "src",
		EdgeTypeId: 7,
	})
	if err != nil {
		t.Fatalf("GetConnectedNodes: unexpected error: %v", err)
	}
	got := nodesByID(resp)
	if _, ok := got["child_visible"]; !ok {
		t.Errorf("missing child_visible (ids=%v)", keys(got))
	}
	if _, ok := got["child_denied"]; ok {
		t.Errorf("child_denied leaked past ACL filter (ids=%v)", keys(got))
	}
	if len(got) != 1 {
		t.Fatalf("got %d nodes, want 1 (only the bob-owned child): ids=%v",
			len(got), keys(got))
	}
}

// TestGetConnectedNodes_SourceNotAccessibleReturnsEmpty: the actor has
// no read access to the source node — the handler must short-circuit
// to []. Spec: source-gate — "do NOT leak existence via a different code".
func TestGetConnectedNodes_SourceNotAccessibleReturnsEmpty(t *testing.T) {
	t.Parallel()
	srv, cs, ctx := connectedTestServer(t)

	// src owned by alice; bob has no visibility. child is alice-owned
	// (would also be denied to bob, but the source gate fires first).
	mustCreateNode(t, cs, "acme", "src", 1, "user:alice")
	mustCreateNode(t, cs, "acme", "child", 1, "user:alice")
	mustCreateEdge(t, cs, "acme", 7, "src", "child")

	resp, err := srv.GetConnectedNodes(ctx, &pb.GetConnectedNodesRequest{
		Context:    &pb.RequestContext{TenantId: "acme", Actor: "user:bob"},
		NodeId:     "src",
		EdgeTypeId: 7,
	})
	if err != nil {
		t.Fatalf("GetConnectedNodes: unexpected error: %v", err)
	}
	if len(resp.GetNodes()) != 0 {
		t.Fatalf("expected empty result (source-gate prune), got %d nodes",
			len(resp.GetNodes()))
	}
	if resp.GetHasMore() {
		t.Errorf("HasMore should be false on empty result")
	}
}

// TestGetConnectedNodes_UnknownTenantPropagatesError: the tenant gate
// returns NOT_FOUND for an unknown tenant; the handler must propagate it
// (spec: "deliberate behavior tightening" vs. Python's swallow).
func TestGetConnectedNodes_UnknownTenantPropagatesError(t *testing.T) {
	t.Parallel()
	srv, _, ctx := connectedTestServer(t)

	resp, err := srv.GetConnectedNodes(ctx, &pb.GetConnectedNodesRequest{
		Context:    &pb.RequestContext{TenantId: "ghost", Actor: "user:alice"},
		NodeId:     "src",
		EdgeTypeId: 7,
	})
	if err == nil {
		t.Fatalf("expected error for unknown tenant, got resp=%v", resp)
	}
	if got := errs.Code(err); got != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (err=%v)", got, err)
	}
}

// --- helpers ----------------------------------------------------------

func mustCreateNode(t *testing.T, cs *store.CanonicalStore, tenantID, nodeID string, typeID int32, owner string) {
	t.Helper()
	if _, err := cs.CreateNodeRaw(context.Background(), tenantID, store.NodeInput{
		NodeID:     nodeID,
		TypeID:     typeID,
		OwnerActor: owner,
	}); err != nil {
		t.Fatalf("CreateNodeRaw(%s): %v", nodeID, err)
	}
}

func mustCreateEdge(t *testing.T, cs *store.CanonicalStore, tenantID string, edgeTypeID int32, from, to string) {
	t.Helper()
	if _, err := cs.CreateEdge(context.Background(), tenantID, store.EdgeInput{
		EdgeTypeID: edgeTypeID,
		FromNodeID: from,
		ToNodeID:   to,
	}); err != nil {
		t.Fatalf("CreateEdge(%s->%s): %v", from, to, err)
	}
}

func keys(m map[string]*pb.Node) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
