// Tests for the deprecated ListMailboxUsers stub. Behavioural parity
// with Python is pinned by tests/python/integration/test_grpc_contract.py:281-287
// (the cross-language contract suite); this file covers the two
// branches the Python handler has after _check_tenant:
//
//  1. valid tenant -> OK with an empty user_ids list.
//  2. unknown tenant -> NOT_FOUND from the tenant.CheckTenant gate.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// TestListMailboxUsers_EmptyForValidTenant pins the deprecated-stub
// contract: a tenant that passes the gate returns an empty user_ids
// list. The slice must be non-nil — matches Python's
// `ListMailboxUsersResponse(user_ids=[])` repeated-field default.
func TestListMailboxUsers_EmptyForValidTenant(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.ListMailboxUsers(ctx, &pb.ListMailboxUsersRequest{TenantId: "acme"})
	if err != nil {
		t.Fatalf("ListMailboxUsers: unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("ListMailboxUsers: response is nil")
	}
	if resp.UserIds == nil {
		t.Fatalf("ListMailboxUsers: UserIds is nil; want empty slice (contract pin)")
	}
	if len(resp.UserIds) != 0 {
		t.Fatalf("ListMailboxUsers: UserIds = %v; want []", resp.UserIds)
	}
}

// TestListMailboxUsers_UnknownTenantNotFound: when globalstore is wired
// and the tenant row is absent, tenant.CheckTenant returns NOT_FOUND
// and the handler must propagate it verbatim.
func TestListMailboxUsers_UnknownTenantNotFound(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.ListMailboxUsers(context.Background(), &pb.ListMailboxUsersRequest{TenantId: "ghost"})
	if err == nil {
		t.Fatalf("ListMailboxUsers: expected NOT_FOUND, got nil")
	}
	if got := errs.Code(err); got != codes.NotFound {
		t.Fatalf("ListMailboxUsers: code = %v, want NotFound (err=%v)", got, err)
	}
}
