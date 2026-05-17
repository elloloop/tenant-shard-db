// SPDX-License-Identifier: AGPL-3.0-only

// Handler-level tests for the SEC-4 (#135) pagination clamp.
//
// These exercise the real gRPC handlers end-to-end (store + registry
// wired) and assert that an absurd limit (10_000_000) comes back capped
// at api.MaxPageSize while leaving the existing "<=0 => default"
// behaviour untouched. The clamp helper itself is unit-tested in
// pagination_test.go; the store-layer defence-in-depth clamp is pinned
// in internal/store/nodes_query_test.go.

package api_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// TestQueryNodes_OversizedLimitClampedToMax pins the handler clamp:
// 1001 nodes seeded, a 10M limit must return exactly MaxPageSize rows.
func TestQueryNodes_OversizedLimitClampedToMax(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)

	const seeded = api.MaxPageSize + 1
	for i := 0; i < seeded; i++ {
		f.seedNode(fmt.Sprintf("n%04d", i), "user:alice", "alice@example.com", int64(i))
	}

	resp, err := f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
		TypeId:  1,
		Limit:   10_000_000,
	})
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	if got := len(resp.GetNodes()); got != api.MaxPageSize {
		t.Fatalf("oversized limit not clamped: got %d nodes, want %d (%d seeded)",
			got, api.MaxPageSize, seeded)
	}
}

// TestQueryNodes_SmallLimitUnaffected pins that the clamp does NOT
// touch a normal request: a limit well under the ceiling is honoured
// verbatim, and an unset limit still falls back to the default.
func TestQueryNodes_SmallLimitUnaffected(t *testing.T) {
	t.Parallel()
	f := newQueryNodesFixture(t)

	for i := 0; i < 10; i++ {
		f.seedNode(fmt.Sprintf("n%02d", i), "user:alice", "alice@example.com", int64(i))
	}

	// Explicit small limit honoured exactly.
	resp, err := f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
		TypeId:  1,
		Limit:   3,
	})
	if err != nil {
		t.Fatalf("QueryNodes(limit=3): %v", err)
	}
	if got := len(resp.GetNodes()); got != 3 {
		t.Fatalf("small limit not honoured: got %d nodes, want 3", got)
	}

	// Unset limit -> default (100) still applies; 10 seeded rows all
	// come back (proves the clamp didn't hijack the default path).
	resp, err = f.srv.QueryNodes(context.Background(), &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: f.tenantID, Actor: "user:alice"},
		TypeId:  1,
	})
	if err != nil {
		t.Fatalf("QueryNodes(limit unset): %v", err)
	}
	if got := len(resp.GetNodes()); got != 10 {
		t.Fatalf("default-limit path changed: got %d nodes, want 10", got)
	}
}

// TestListUsers_OversizedLimitClampedToMax pins the global-plane clamp:
// MaxPageSize+1 active users seeded, a 10M limit must cap at
// MaxPageSize. This closes the documented "No upper cap on limit"
// parity wart that motivated SEC-4.
func TestListUsers_OversizedLimitClampedToMax(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	const seeded = api.MaxPageSize + 1
	for i := 0; i < seeded; i++ {
		id := fmt.Sprintf("u%05d", i)
		if _, err := gs.CreateUser(ctx, id, id+"@example.com", id); err != nil {
			t.Fatalf("CreateUser(%q): %v", id, err)
		}
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.ListUsers(ctx, &pb.ListUsersRequest{
		Actor: "user:admin",
		Limit: 10_000_000,
	})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if got := len(resp.GetUsers()); got != api.MaxPageSize {
		t.Fatalf("oversized limit not clamped: got %d users, want %d (%d seeded)",
			got, api.MaxPageSize, seeded)
	}
}

// TestListUsers_NegativeLimitCoercedToDefault pins the SEC-4 widening
// of the zero-coercion to <=0: a negative limit (SQLite LIMIT -1 ==
// unbounded) must NOT trigger a full-registry scan. With seeded > 100
// the response must cap at the 100 default, not return everything.
func TestListUsers_NegativeLimitCoercedToDefault(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	const seeded = 150
	for i := 0; i < seeded; i++ {
		id := fmt.Sprintf("u%05d", i)
		if _, err := gs.CreateUser(ctx, id, id+"@example.com", id); err != nil {
			t.Fatalf("CreateUser(%q): %v", id, err)
		}
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.ListUsers(ctx, &pb.ListUsersRequest{
		Actor: "user:admin",
		Limit: -1, // pre-fix: flowed through as SQLite "unlimited"
	})
	if err != nil {
		t.Fatalf("ListUsers(limit=-1): %v", err)
	}
	if got := len(resp.GetUsers()); got != 100 {
		t.Fatalf("negative limit not coerced to default: got %d users, want 100", got)
	}
}
