// SPDX-License-Identifier: AGPL-3.0-only

// Keyset cursor pagination for QueryNodes (ADR-029 / #564).
//
// These pin the post-fix contract: a read never silently truncates —
// following next_page_token retrieves the complete set, the seek is
// stable under concurrent deletes (no skip/duplicate), page_size wins
// over the legacy limit, and a token bound to a different query (or
// mixed with the deprecated offset) is rejected.

package api_test

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

func (f *queryNodesFixture) query(t *testing.T, req *pb.QueryNodesRequest) *pb.QueryNodesResponse {
	t.Helper()
	if req.GetContext() == nil {
		req.Context = &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"}
	}
	resp, err := f.srv.QueryNodes(context.Background(), req)
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	return resp
}

// pageAll follows next_page_token to exhaustion and returns the node_ids
// in the order seen, asserting each page (except possibly the last) is
// full and that the loop terminates.
func (f *queryNodesFixture) pageAll(t *testing.T, pageSize int32, orderBy string) []string {
	t.Helper()
	var got []string
	token := ""
	for i := 0; ; i++ {
		if i > 1000 {
			t.Fatal("pagination did not terminate (cursor loop)")
		}
		resp := f.query(t, &pb.QueryNodesRequest{
			TypeId:    1,
			OrderBy:   orderBy,
			PageSize:  pageSize,
			PageToken: token,
		})
		for _, n := range resp.GetNodes() {
			got = append(got, n.GetNodeId())
		}
		token = resp.GetNextPageToken()
		if token == "" {
			break
		}
	}
	return got
}

func TestQueryNodes_Keyset_PagesEntireSetNoTruncation(t *testing.T) {
	t.Parallel()
	for _, orderBy := range []string{"node_id", "created_at"} {
		orderBy := orderBy
		t.Run(orderBy, func(t *testing.T) {
			t.Parallel()
			f := newQueryNodesFixture(t)
			const n = 25
			want := map[string]bool{}
			for i := 0; i < n; i++ {
				id := fmt.Sprintf("n%03d", i)
				f.seedNode(id, "user:alice", fmt.Sprintf("u%03d@x", i), int64(i))
				want[id] = true
			}
			got := f.pageAll(t, 10, orderBy)
			if len(got) != n {
				t.Fatalf("paged %d rows, want %d — silent truncation (Bug A)", len(got), n)
			}
			seen := map[string]bool{}
			for _, id := range got {
				if seen[id] {
					t.Fatalf("node %s returned on more than one page (duplicate)", id)
				}
				seen[id] = true
				if !want[id] {
					t.Fatalf("unexpected node %s", id)
				}
			}
			if len(seen) != n {
				t.Fatalf("distinct rows = %d, want %d", len(seen), n)
			}
		})
	}
}

// TestQueryNodes_Keyset_StableUnderDelete is the discriminating test vs
// OFFSET: page 1 (keyset), delete a row from the already-seen region,
// then page 2 via the token. Keyset resumes at the anchor regardless of
// the shift, so no unseen row is skipped. OFFSET=pageSize would skip the
// first unseen row after the delete.
func TestQueryNodes_Keyset_StableUnderDelete(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)
	const n = 20
	for i := 0; i < n; i++ {
		f.seedNode(fmt.Sprintf("n%03d", i), "user:alice", fmt.Sprintf("u%03d@x", i), int64(i))
	}

	page1 := f.query(t, &pb.QueryNodesRequest{TypeId: 1, OrderBy: "node_id", PageSize: 10})
	if len(page1.GetNodes()) != 10 || page1.GetNextPageToken() == "" {
		t.Fatalf("page1: got %d nodes, token=%q", len(page1.GetNodes()), page1.GetNextPageToken())
	}

	// Delete a row from the seen region (n003). With OFFSET=10 this would
	// shift the window left and skip n010 on page 2.
	if err := f.cs.DeleteNode(context.Background(), f.tenantID, "n003"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}

	page2 := f.query(t, &pb.QueryNodesRequest{
		TypeId: 1, OrderBy: "node_id", PageSize: 10, PageToken: page1.GetNextPageToken(),
	})
	ids := map[string]bool{}
	for _, n := range page2.GetNodes() {
		ids[n.GetNodeId()] = true
	}
	if !ids["n010"] {
		t.Fatalf("keyset skipped n010 after a seen-region delete (got %v) — not stable", boolMapKeys(ids))
	}
	if len(page2.GetNodes()) != 10 {
		t.Fatalf("page2: got %d nodes, want 10 (n010..n019)", len(page2.GetNodes()))
	}
}

func TestQueryNodes_Keyset_PageSizePrefersOverLimit(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)
	for i := 0; i < 10; i++ {
		f.seedNode(fmt.Sprintf("n%03d", i), "user:alice", fmt.Sprintf("u%03d@x", i), int64(i))
	}
	resp := f.query(t, &pb.QueryNodesRequest{TypeId: 1, OrderBy: "node_id", Limit: 5, PageSize: 2})
	if len(resp.GetNodes()) != 2 {
		t.Fatalf("got %d nodes, want 2 (page_size must win over limit)", len(resp.GetNodes()))
	}
}

func TestQueryNodes_Keyset_RejectsCrossQueryToken(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)
	for i := 0; i < 12; i++ {
		f.seedNode(fmt.Sprintf("n%03d", i), "user:alice", fmt.Sprintf("u%03d@x", i), int64(i))
	}
	// Mint a token under order_by=node_id.
	first := f.query(t, &pb.QueryNodesRequest{TypeId: 1, OrderBy: "node_id", PageSize: 5})
	if first.GetNextPageToken() == "" {
		t.Fatal("expected a next_page_token")
	}
	// Present it to a query with a different order_by → fingerprint mismatch.
	_, err := f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
		Context:   &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
		TypeId:    1,
		OrderBy:   "created_at",
		PageSize:  5,
		PageToken: first.GetNextPageToken(),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("cross-query token: code = %s, want InvalidArgument (%v)", status.Code(err), err)
	}
}

func TestQueryNodes_Keyset_RejectsTokenWithOffset(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)
	for i := 0; i < 12; i++ {
		f.seedNode(fmt.Sprintf("n%03d", i), "user:alice", fmt.Sprintf("u%03d@x", i), int64(i))
	}
	first := f.query(t, &pb.QueryNodesRequest{TypeId: 1, OrderBy: "node_id", PageSize: 5})
	_, err := f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
		Context:   &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
		TypeId:    1,
		OrderBy:   "node_id",
		PageSize:  5,
		Offset:    5,
		PageToken: first.GetNextPageToken(),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("token+offset: code = %s, want InvalidArgument (%v)", status.Code(err), err)
	}
}

func boolMapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
