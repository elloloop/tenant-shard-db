// Tests for GetEdgesFrom RPC.
//
// Contract pins (from docs/go-port/rpcs/GetEdgesFrom.md):
//
//   - Empty seed: test_grpc_contract.py:241-247 — call succeeds,
//                     edges == [].
//   - Single edge: happy round-trip; props are preserved on the wire.
//   - Multiple edges: every stored outgoing edge is returned (no
//                     ACL filter on destinations — preserved parity gap;
//                     see GetEdgesFrom.md §"Auth").
//   - Missing source: unknown from_node_id returns empty edges, no
//                     error (no NOT_FOUND special-case).
//   - Type filter: edge_type_id != 0 narrows the result set;
//                     edge_type_id == 0 returns every type.
//   - Internal error: store fault (tenant DB never opened) collapses
//                     to an empty response with codes.OK.
//
// The shared globalstore helper lives in helpers_external_test.go;
// do NOT redeclare newGlobalStore here.

package api_test

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/tenant"
)

// newEdgesStore opens a fresh, tmpdir-backed CanonicalStore. WAL on so
// concurrent SQLite reads don't serialise.
func newEdgesStore(t *testing.T) *store.CanonicalStore {
	t.Helper()
	cs, err := store.New(store.Options{
		RootDir: t.TempDir(),
		WALMode: true,
	})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// newEdgesServer wires globalstore (so the tenant gate sees a
// registered tenant) and a CanonicalStore so the handler can read.
// Returns the constructed server, the store handle (for seeding edges),
// and the tenant id used.
func newEdgesServer(t *testing.T, tenantID string) (*api.Server, *store.CanonicalStore) {
	t.Helper()
	cs := newEdgesStore(t)
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, tenantID, "T", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithStore(cs),
		api.WithSharding(&tenant.Sharding{NodeID: "node-a"}),
	)
	return srv, cs
}

func edgesFromReq(tenantID, nodeID string) *pb.GetEdgesRequest {
	return &pb.GetEdgesRequest{
		Context: &pb.RequestContext{TenantId: tenantID},
		NodeId:  nodeID,
	}
}

// TestGetEdgesFrom_EmptyTenantReturnsNoEdges: no edges seeded -> empty
// list, no error. Mirrors test_grpc_contract.py:241-247.
func TestGetEdgesFrom_EmptyTenantReturnsNoEdges(t *testing.T) {
	t.Parallel()
	srv, _ := newEdgesServer(t, "tenant-empty")

	resp, err := srv.GetEdgesFrom(context.Background(), edgesFromReq("tenant-empty", "src"))
	if err != nil {
		t.Fatalf("GetEdgesFrom: unexpected error: %v", err)
	}
	if got := len(resp.GetEdges()); got != 0 {
		t.Fatalf("edges len: got %d, want 0", got)
	}
	if resp.GetHasMore() {
		t.Fatalf("has_more: got true, want false")
	}
}

// TestGetEdgesFrom_SingleEdgeRoundTrip: one outgoing edge -> returned
// with all wire fields populated.
func TestGetEdgesFrom_SingleEdgeRoundTrip(t *testing.T) {
	t.Parallel()
	srv, cs := newEdgesServer(t, "tenant-single")
	ctx := context.Background()

	if _, err := cs.CreateEdge(ctx, "tenant-single", store.EdgeInput{
		EdgeTypeID: 7,
		FromNodeID: "src",
		ToNodeID:   "dst",
		Props:      map[string]any{"1": "hello"},
	}); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	resp, err := srv.GetEdgesFrom(ctx, edgesFromReq("tenant-single", "src"))
	if err != nil {
		t.Fatalf("GetEdgesFrom: %v", err)
	}
	if len(resp.GetEdges()) != 1 {
		t.Fatalf("edges len: got %d, want 1", len(resp.GetEdges()))
	}
	e := resp.GetEdges()[0]
	if e.GetTenantId() != "tenant-single" {
		t.Fatalf("tenant_id: %q", e.GetTenantId())
	}
	if e.GetEdgeTypeId() != 7 {
		t.Fatalf("edge_type_id: %d", e.GetEdgeTypeId())
	}
	if e.GetFromNodeId() != "src" || e.GetToNodeId() != "dst" {
		t.Fatalf("from/to: %q->%q", e.GetFromNodeId(), e.GetToNodeId())
	}
	if e.GetProps() == nil {
		t.Fatalf("props: nil, want populated Struct")
	}
	v, ok := e.GetProps().GetFields()["1"]
	if !ok || v.GetStringValue() != "hello" {
		t.Fatalf("props[1]: got %v, want \"hello\"", v)
	}
}

// TestGetEdgesFrom_MultipleEdgesAllReturned pins the no-ACL-filter
// parity gap. The handler returns every stored outgoing edge,
// including destinations the caller has no READ on. Tightening this
// requires a contract change — see GetEdgesFrom.md §"Auth".
func TestGetEdgesFrom_MultipleEdgesAllReturned(t *testing.T) {
	t.Parallel()
	srv, cs := newEdgesServer(t, "tenant-multi")
	ctx := context.Background()

	for _, dst := range []string{"d1", "d2", "d3"} {
		if _, err := cs.CreateEdge(ctx, "tenant-multi", store.EdgeInput{
			EdgeTypeID: 1,
			FromNodeID: "src",
			ToNodeID:   dst,
		}); err != nil {
			t.Fatalf("CreateEdge %s: %v", dst, err)
		}
	}

	resp, err := srv.GetEdgesFrom(ctx, edgesFromReq("tenant-multi", "src"))
	if err != nil {
		t.Fatalf("GetEdgesFrom: %v", err)
	}
	if len(resp.GetEdges()) != 3 {
		t.Fatalf("edges len: got %d, want 3 (no ACL filter expected)", len(resp.GetEdges()))
	}
	got := map[string]bool{}
	for _, e := range resp.GetEdges() {
		got[e.GetToNodeId()] = true
	}
	for _, want := range []string{"d1", "d2", "d3"} {
		if !got[want] {
			t.Fatalf("missing dst %q in result: %v", want, got)
		}
	}
}

// TestGetEdgesFrom_MissingSourceReturnsEmpty: an unknown from_node_id
// returns an empty edge list with no error (no NOT_FOUND).
func TestGetEdgesFrom_MissingSourceReturnsEmpty(t *testing.T) {
	t.Parallel()
	srv, cs := newEdgesServer(t, "tenant-miss")
	ctx := context.Background()
	if _, err := cs.CreateEdge(ctx, "tenant-miss", store.EdgeInput{
		EdgeTypeID: 1, FromNodeID: "src", ToNodeID: "dst",
	}); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	resp, err := srv.GetEdgesFrom(ctx, edgesFromReq("tenant-miss", "ghost"))
	if err != nil {
		t.Fatalf("GetEdgesFrom: %v", err)
	}
	if len(resp.GetEdges()) != 0 {
		t.Fatalf("edges len: got %d, want 0 for missing source", len(resp.GetEdges()))
	}
}

// TestGetEdgesFrom_EdgeTypeFilter pins both branches: edge_type_id == 0
// returns every type, edge_type_id != 0 narrows.
func TestGetEdgesFrom_EdgeTypeFilter(t *testing.T) {
	t.Parallel()
	srv, cs := newEdgesServer(t, "tenant-filter")
	ctx := context.Background()

	if _, err := cs.CreateEdge(ctx, "tenant-filter", store.EdgeInput{
		EdgeTypeID: 1, FromNodeID: "src", ToNodeID: "d1",
	}); err != nil {
		t.Fatalf("CreateEdge t1: %v", err)
	}
	if _, err := cs.CreateEdge(ctx, "tenant-filter", store.EdgeInput{
		EdgeTypeID: 2, FromNodeID: "src", ToNodeID: "d2",
	}); err != nil {
		t.Fatalf("CreateEdge t2: %v", err)
	}

	// edge_type_id == 0 -> no filter, returns both.
	respAll, err := srv.GetEdgesFrom(ctx, edgesFromReq("tenant-filter", "src"))
	if err != nil {
		t.Fatalf("GetEdgesFrom (all): %v", err)
	}
	if len(respAll.GetEdges()) != 2 {
		t.Fatalf("unfiltered edges: got %d, want 2", len(respAll.GetEdges()))
	}

	// edge_type_id == 2 -> only the type-2 edge.
	respFilt, err := srv.GetEdgesFrom(ctx, &pb.GetEdgesRequest{
		Context:    &pb.RequestContext{TenantId: "tenant-filter"},
		NodeId:     "src",
		EdgeTypeId: 2,
	})
	if err != nil {
		t.Fatalf("GetEdgesFrom (filtered): %v", err)
	}
	if len(respFilt.GetEdges()) != 1 {
		t.Fatalf("filtered edges: got %d, want 1", len(respFilt.GetEdges()))
	}
	if respFilt.GetEdges()[0].GetEdgeTypeId() != 2 {
		t.Fatalf("filtered edge type: got %d, want 2", respFilt.GetEdges()[0].GetEdgeTypeId())
	}
}

// TestGetEdgesFrom_NotOpenTenant_LazyOpensAndReadsData: a registered
// tenant whose per-tenant SQLite handle has NOT been opened in this
// process must not be reported as a false-empty edge set. The store is a
// materialized view of the WAL (ADR-016); "not opened" only means the
// applier hasn't materialized the tenant in-process yet, so the handler
// lazy-opens it and returns the edges already persisted to disk.
//
// Two phases over the SAME RootDir prove this: phase 1 seeds an edge
// through a store that opens the tenant, then closes it; phase 2 serves
// GetEdgesFrom from a FRESH store where the tenant was never opened.
func TestGetEdgesFrom_NotOpenTenant_LazyOpensAndReadsData(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	const tenantID = "tenant-reopen"

	// Phase 1: seed one edge, then close the store.
	seedCS, err := store.New(store.Options{RootDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("store.New (seed): %v", err)
	}
	if err := seedCS.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant (seed): %v", err)
	}
	if _, err := seedCS.CreateEdge(ctx, tenantID, store.EdgeInput{
		EdgeTypeID: 7,
		FromNodeID: "src",
		ToNodeID:   "dst",
	}); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}
	if err := seedCS.Close(); err != nil {
		t.Fatalf("seed store Close: %v", err)
	}

	// Phase 2: a fresh store over the same dir — tenant NOT opened.
	cs, err := store.New(store.Options{RootDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("store.New (fresh): %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(ctx, tenantID, "T", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithStore(cs),
		api.WithSharding(&tenant.Sharding{NodeID: "node-a"}),
	)

	resp, err := srv.GetEdgesFrom(ctx, edgesFromReq(tenantID, "src"))
	if err != nil {
		t.Fatalf("GetEdgesFrom on not-open tenant: want OK, got %v", err)
	}
	if got := len(resp.GetEdges()); got != 1 {
		t.Fatalf("edges: got %d, want 1 — lazy-open must read the persisted "+
			"edge, not a false-empty result", got)
	}
}
