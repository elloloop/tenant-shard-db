// SPDX-License-Identifier: AGPL-3.0-only

// Keyset cursor pagination for GetEdgesFrom/GetEdgesTo (ADR-029 / #580).

package api_test

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

func TestGetEdgesFrom_KeysetPagesAllEdges(t *testing.T) {
	t.Parallel()
	srv, cs := newEdgesServer(t, "tenant-edge-keyset")
	ctx := context.Background()
	const n = 25
	for i := 0; i < n; i++ {
		if _, err := cs.CreateEdge(ctx, "tenant-edge-keyset", store.EdgeInput{
			EdgeTypeID: 7, FromNodeID: "src", ToNodeID: fmt.Sprintf("dst-%03d", i),
		}); err != nil {
			t.Fatalf("CreateEdge: %v", err)
		}
	}

	seen := map[string]bool{}
	token := ""
	for pages := 0; ; pages++ {
		if pages > 1000 {
			t.Fatal("pagination did not terminate")
		}
		resp, err := srv.GetEdgesFrom(ctx, &pb.GetEdgesRequest{
			Context:    &pb.RequestContext{TenantId: "tenant-edge-keyset", Actor: "user:alice"},
			NodeId:     "src",
			EdgeTypeId: 7,
			PageSize:   10,
			PageToken:  token,
		})
		if err != nil {
			t.Fatalf("GetEdgesFrom: %v", err)
		}
		for _, e := range resp.GetEdges() {
			if seen[e.GetToNodeId()] {
				t.Fatalf("edge to %s returned on more than one page", e.GetToNodeId())
			}
			seen[e.GetToNodeId()] = true
		}
		token = resp.GetNextPageToken()
		if token == "" {
			break
		}
	}
	if len(seen) != n {
		t.Fatalf("paged %d edges, want %d — silent truncation (Bug A class)", len(seen), n)
	}
}

func TestGetEdgesFrom_KeysetRejectsCrossQueryToken(t *testing.T) {
	t.Parallel()
	srv, cs := newEdgesServer(t, "tenant-edge-xq")
	ctx := context.Background()
	for i := 0; i < 12; i++ {
		if _, err := cs.CreateEdge(ctx, "tenant-edge-xq", store.EdgeInput{
			EdgeTypeID: 7, FromNodeID: "src", ToNodeID: fmt.Sprintf("dst-%03d", i),
		}); err != nil {
			t.Fatalf("CreateEdge: %v", err)
		}
	}
	first, err := srv.GetEdgesFrom(ctx, &pb.GetEdgesRequest{
		Context: &pb.RequestContext{TenantId: "tenant-edge-xq", Actor: "user:alice"},
		NodeId:  "src", EdgeTypeId: 7, PageSize: 5,
	})
	if err != nil {
		t.Fatalf("GetEdgesFrom: %v", err)
	}
	if first.GetNextPageToken() == "" {
		t.Fatal("expected a next_page_token")
	}
	// Present the token under a different edge_type_id → fingerprint mismatch.
	_, err = srv.GetEdgesFrom(ctx, &pb.GetEdgesRequest{
		Context:    &pb.RequestContext{TenantId: "tenant-edge-xq", Actor: "user:alice"},
		NodeId:     "src",
		EdgeTypeId: 9,
		PageSize:   5,
		PageToken:  first.GetNextPageToken(),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("cross-query token: code = %s, want InvalidArgument (%v)", status.Code(err), err)
	}
}

func TestGetEdgesFrom_KeysetRejectsTokenWithOffset(t *testing.T) {
	t.Parallel()
	srv, cs := newEdgesServer(t, "tenant-edge-off")
	ctx := context.Background()
	for i := 0; i < 12; i++ {
		if _, err := cs.CreateEdge(ctx, "tenant-edge-off", store.EdgeInput{
			EdgeTypeID: 7, FromNodeID: "src", ToNodeID: fmt.Sprintf("dst-%03d", i),
		}); err != nil {
			t.Fatalf("CreateEdge: %v", err)
		}
	}
	first, _ := srv.GetEdgesFrom(ctx, &pb.GetEdgesRequest{
		Context: &pb.RequestContext{TenantId: "tenant-edge-off", Actor: "user:alice"},
		NodeId:  "src", EdgeTypeId: 7, PageSize: 5,
	})
	_, err := srv.GetEdgesFrom(ctx, &pb.GetEdgesRequest{
		Context: &pb.RequestContext{TenantId: "tenant-edge-off", Actor: "user:alice"},
		NodeId:  "src", EdgeTypeId: 7, PageSize: 5, Offset: 5,
		PageToken: first.GetNextPageToken(),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("token+offset: code = %s, want InvalidArgument (%v)", status.Code(err), err)
	}
}
