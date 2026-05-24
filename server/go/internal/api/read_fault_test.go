// SPDX-License-Identifier: AGPL-3.0-only

// Regression tests for #573: read RPCs must surface a genuine post-open
// store fault as a non-OK status, not mask it as empty+OK (which makes a
// broken store indistinguishable from "no results" — silent data loss).
//
// A fault is injected by dropping the underlying table through a second
// raw connection to the tenant's SQLite file (the store keeps its own
// pool open, so its next query fails with "no such table"). This hits the
// post-open read path specifically — the tenant is already lazy-opened,
// so this is NOT the not-open case (which is correctly lazy-opened).

package api_test

import (
	"context"
	"database/sql"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// dropTenantTable drops a table from a tenant DB via a second connection,
// faulting the store's subsequent reads. The "sqlite" driver (modernc) is
// registered transitively through the store package import.
func dropTenantTable(t *testing.T, cs *store.CanonicalStore, tenantID, table string) {
	t.Helper()
	path, err := cs.TenantDBPath(tenantID)
	if err != nil {
		t.Fatalf("TenantDBPath: %v", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("DROP TABLE " + table); err != nil {
		t.Fatalf("drop table %s: %v", table, err)
	}
}

func TestGetNodes_StoreFaultSurfacesInternal(t *testing.T) {
	t.Parallel()
	const tenantID = "acme"
	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(context.Background(), tenantID, "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := gs.AddTenantMember(context.Background(), tenantID, "alice", "owner"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}
	cs := newStoreWithTenant(t, tenantID)
	seedNodeForGetNodes(t, cs, tenantID, "n1", "user:alice")
	srv := api.New(api.WithGlobalStore(gs), api.WithStore(cs))
	dropTenantTable(t, cs, tenantID, "nodes")

	_, err := srv.GetNodes(context.Background(), &pb.GetNodesRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "user:alice"},
		NodeIds: []string{"n1"},
	})
	// The pre-fix behavior returned all-missing + codes.OK, masking the
	// fault as if the node simply didn't exist. Post-fix: a non-OK status.
	if status.Code(err) != codes.Internal {
		t.Fatalf("GetNodes on broken store: code = %s, want Internal (%v)", status.Code(err), err)
	}
}

func TestQueryNodes_StoreFaultSurfacesInternal(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)
	f.seedNode("n1", "user:alice", "a@x", 1)
	dropTenantTable(t, f.cs, f.tenantID, "nodes")

	_, err := f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
		TypeId:  1,
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("QueryNodes on broken store: code = %s, want Internal (%v)", status.Code(err), err)
	}
}

func TestGetEdgesFrom_StoreFaultSurfacesInternal(t *testing.T) {
	t.Parallel()
	srv, cs := newEdgesServer(t, "tenant-fault-from")
	ctx := context.Background()
	if _, err := cs.CreateEdge(ctx, "tenant-fault-from", store.EdgeInput{
		EdgeTypeID: 7, FromNodeID: "src", ToNodeID: "dst",
	}); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}
	dropTenantTable(t, cs, "tenant-fault-from", "edges")

	_, err := srv.GetEdgesFrom(ctx, edgesFromReq("tenant-fault-from", "src"))
	if status.Code(err) != codes.Internal {
		t.Fatalf("GetEdgesFrom on broken store: code = %s, want Internal (%v)", status.Code(err), err)
	}
}

func TestGetEdgesTo_StoreFaultSurfacesInternal(t *testing.T) {
	t.Parallel()
	srv, cs := newEdgesServer(t, "tenant-fault-to")
	ctx := context.Background()
	if _, err := cs.CreateEdge(ctx, "tenant-fault-to", store.EdgeInput{
		EdgeTypeID: 7, FromNodeID: "src", ToNodeID: "dst",
	}); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}
	dropTenantTable(t, cs, "tenant-fault-to", "edges")

	_, err := srv.GetEdgesTo(ctx, edgesFromReq("tenant-fault-to", "dst"))
	if status.Code(err) != codes.Internal {
		t.Fatalf("GetEdgesTo on broken store: code = %s, want Internal (%v)", status.Code(err), err)
	}
}

func TestGetConnectedNodes_StoreFaultSurfacesInternal(t *testing.T) {
	t.Parallel()
	srv, cs, ctx := connectedTestServer(t)
	// Seed the source as a real node owned by alice so the source-visibility
	// gate passes (owner match), letting the BFS run and hit the dropped
	// edges table — the fault path under test.
	if _, err := cs.CreateNodeRaw(ctx, "acme", store.NodeInput{
		NodeID: "src", TypeID: 1, OwnerActor: "user:alice",
		Payload: map[string]any{"1": "src"},
	}); err != nil {
		t.Fatalf("CreateNodeRaw: %v", err)
	}
	mustCreateEdge(t, cs, "acme", 7, "src", "child")
	dropTenantTable(t, cs, "acme", "edges")

	_, err := srv.GetConnectedNodes(ctx, &pb.GetConnectedNodesRequest{
		Context:    &pb.RequestContext{TenantId: "acme", Actor: "user:alice"},
		NodeId:     "src",
		EdgeTypeId: 7,
		Limit:      50,
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("GetConnectedNodes on broken store: code = %s, want Internal (%v)", status.Code(err), err)
	}
}

func TestSearchNodes_StoreFaultSurfacesInternal(t *testing.T) {
	t.Parallel()
	srv, cs, tenantID := newSearchTestServer(t)
	seedIndexedNode(t, cs, tenantID, "n1", "user:alice",
		map[string]any{"1": "hello", "2": "world"}, nil)
	// Drop the base nodes table the FTS JOIN reads from → genuine scan
	// fault (distinct from the malformed-query InvalidArgument path).
	dropTenantTable(t, cs, tenantID, "nodes")

	_, err := srv.SearchNodes(context.Background(), &pb.SearchNodesRequest{
		TenantId: tenantID,
		Actor:    "user:alice",
		TypeId:   searchTypeID,
		Query:    "hello",
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("SearchNodes on broken store: code = %s, want Internal (%v)", status.Code(err), err)
	}
}
