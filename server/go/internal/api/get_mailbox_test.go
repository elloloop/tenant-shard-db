// Tests for the deprecated GetMailbox stub.
//
// The Python contract test at
// tests/python/integration/test_grpc_contract.py:274-280 pins the
// happy-path shape (`items == [] and unread_count == 0`). The Go
// unit tests below pin the same shape plus the tenant-gate
// NOT_FOUND branch which the contract test does not cover for this
// RPC (see GetMailbox.md "Contract tests pinning behavior").

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/tenant"
)

// TestGetMailbox_HappyPath: with a registered tenant, the stub
// returns an empty, well-formed response — `items == []`,
// `unread_count == 0`, `has_more == false`. Mirrors the Python
// contract pin at test_grpc_contract.py:274-280.
func TestGetMailbox_HappyPath(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithSharding(&tenant.Sharding{NodeID: "node-a"}),
	)

	resp, err := srv.GetMailbox(ctx, &pb.GetMailboxRequest{
		Context: &pb.RequestContext{TenantId: "acme"},
		UserId:  "alice",
	})
	if err != nil {
		t.Fatalf("GetMailbox: unexpected err: %v", err)
	}
	if resp == nil {
		t.Fatalf("GetMailbox: nil response")
	}
	if len(resp.GetItems()) != 0 {
		t.Fatalf("GetMailbox: items: got %d, want 0", len(resp.GetItems()))
	}
	if resp.GetUnreadCount() != 0 {
		t.Fatalf("GetMailbox: unread_count: got %d, want 0", resp.GetUnreadCount())
	}
	if resp.GetHasMore() != false {
		t.Fatalf("GetMailbox: has_more: got %v, want false", resp.GetHasMore())
	}
}

// TestGetMailbox_HappyPath_SingleNode: with no globalstore and no
// sharding wired, the gate degrades to single-node-default and the
// stub still returns the empty response. This is the bring-up path
// for the Go server before globalstore is connected.
func TestGetMailbox_HappyPath_SingleNode(t *testing.T) {
	t.Parallel()
	srv := api.New()
	resp, err := srv.GetMailbox(context.Background(), &pb.GetMailboxRequest{
		Context: &pb.RequestContext{TenantId: "acme"},
	})
	if err != nil {
		t.Fatalf("GetMailbox: %v", err)
	}
	if resp == nil || len(resp.GetItems()) != 0 || resp.GetUnreadCount() != 0 || resp.GetHasMore() {
		t.Fatalf("GetMailbox: want empty response, got %+v", resp)
	}
}

// TestGetMailbox_UnknownTenant: when globalstore is wired and the
// tenant does not exist, the tenant gate produces NOT_FOUND and the
// handler MUST propagate it (no swallow — the gate's status is the
// signal the SDK needs).
func TestGetMailbox_UnknownTenant(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)

	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithSharding(&tenant.Sharding{NodeID: "node-a"}),
	)

	resp, err := srv.GetMailbox(context.Background(), &pb.GetMailboxRequest{
		Context: &pb.RequestContext{TenantId: "ghost"},
	})
	if err == nil {
		t.Fatalf("GetMailbox: expected error for unknown tenant, got resp=%+v", resp)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("GetMailbox: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Fatalf("GetMailbox: code: got %v, want NotFound", st.Code())
	}
}

// TestGetMailbox_IgnoresFilters: the stub ignores user_id, filters,
// and pagination. Setting wild values must not affect the response.
func TestGetMailbox_IgnoresFilters(t *testing.T) {
	t.Parallel()
	srv := api.New()
	resp, err := srv.GetMailbox(context.Background(), &pb.GetMailboxRequest{
		Context:      &pb.RequestContext{TenantId: "acme"},
		UserId:       "alice",
		SourceTypeId: 42,
		ThreadId:     "t-1",
		UnreadOnly:   true,
		Limit:        100,
		Offset:       50,
	})
	if err != nil {
		t.Fatalf("GetMailbox: %v", err)
	}
	if len(resp.GetItems()) != 0 || resp.GetUnreadCount() != 0 || resp.GetHasMore() {
		t.Fatalf("GetMailbox: filters leaked into response: %+v", resp)
	}
}
