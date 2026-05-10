// Tests for the GetUserTenants handler. Behavioural parity is
// pinned by tests/python/integration/test_grpc_contract.py:500-510 and
// tests/python/unit/test_tenant_registry.py:557-582; this file covers
// every branch the Python handler has after the validation gate plus
// the silent-swallow degradation path.

package api_test

import (
	"context"
	"sort"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// TestGetUserTenants_ZeroMemberships pins the empty-list happy path:
// a known user with no memberships returns OK and an empty (but
// non-nil) Memberships slice. Mirrors Python's
// `global_store.get_user_tenants(...)` returning [] (global_store.py:622-628).
func TestGetUserTenants_ZeroMemberships(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.GetUserTenants(context.Background(), &pb.GetUserTenantsRequest{
		Actor:  "user:alice",
		UserId: "alice",
	})
	if err != nil {
		t.Fatalf("GetUserTenants: unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("GetUserTenants: response is nil")
	}
	if resp.Memberships == nil {
		t.Fatalf("GetUserTenants: Memberships is nil; want empty slice")
	}
	if len(resp.Memberships) != 0 {
		t.Fatalf("GetUserTenants: Memberships = %v; want []", resp.Memberships)
	}
}

// TestGetUserTenants_OneMembership pins the single-tenant happy path
// from test_grpc_contract.py:500-505: the response carries one
// TenantMemberInfo with the tenant_id, user_id, role, and joined_at
// populated.
func TestGetUserTenants_OneMembership(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := gs.AddTenantMember(ctx, "acme", "alice", "owner"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))
	resp, err := srv.GetUserTenants(ctx, &pb.GetUserTenantsRequest{
		Actor:  "user:alice",
		UserId: "alice",
	})
	if err != nil {
		t.Fatalf("GetUserTenants: unexpected error: %v", err)
	}
	if got := len(resp.GetMemberships()); got != 1 {
		t.Fatalf("GetUserTenants: len(memberships) = %d; want 1 (resp=%v)", got, resp)
	}
	m := resp.GetMemberships()[0]
	if m.GetTenantId() != "acme" {
		t.Fatalf("TenantId = %q; want %q", m.GetTenantId(), "acme")
	}
	if m.GetUserId() != "alice" {
		t.Fatalf("UserId = %q; want %q", m.GetUserId(), "alice")
	}
	if m.GetRole() != "owner" {
		t.Fatalf("Role = %q; want %q", m.GetRole(), "owner")
	}
	if m.GetJoinedAt() <= 0 {
		t.Fatalf("JoinedAt = %d; want > 0", m.GetJoinedAt())
	}
}

// TestGetUserTenants_MultipleMemberships pins the multi-tenant case
// from test_tenant_registry.py:557-582: alice in two tenants, both
// must be returned.
func TestGetUserTenants_MultipleMemberships(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	gs := newGlobalStore(t)
	for _, tid := range []string{"t1", "t2"} {
		if _, err := gs.CreateTenant(ctx, tid, tid, "us-east-1"); err != nil {
			t.Fatalf("CreateTenant(%q): %v", tid, err)
		}
		if err := gs.AddTenantMember(ctx, tid, "alice", "member"); err != nil {
			t.Fatalf("AddTenantMember(%q): %v", tid, err)
		}
	}

	srv := api.New(api.WithGlobalStore(gs))
	resp, err := srv.GetUserTenants(ctx, &pb.GetUserTenantsRequest{
		Actor:  "user:alice",
		UserId: "alice",
	})
	if err != nil {
		t.Fatalf("GetUserTenants: unexpected error: %v", err)
	}
	got := make([]string, 0, len(resp.GetMemberships()))
	for _, m := range resp.GetMemberships() {
		got = append(got, m.GetTenantId())
	}
	sort.Strings(got)
	want := []string{"t1", "t2"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("tenant_ids = %v; want %v", got, want)
	}
}

// TestGetUserTenants_EmptyActor: empty actor string -> INVALID_ARGUMENT.
// Mirrors grpc_server.py:2586-2587 and is the contract pin from
// test_grpc_contract.py:506-510.
func TestGetUserTenants_EmptyActor(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.GetUserTenants(context.Background(), &pb.GetUserTenantsRequest{
		Actor:  "",
		UserId: "alice",
	})
	if err == nil {
		t.Fatalf("GetUserTenants: expected INVALID_ARGUMENT, got nil")
	}
	if got := errs.Code(err); got != codes.InvalidArgument {
		t.Fatalf("code = %v; want InvalidArgument (err=%v)", got, err)
	}
}

// TestGetUserTenants_EmptyUserID: empty user_id -> INVALID_ARGUMENT.
// Pinned by grpc_server.py:2588-2589. The Python contract suite does
// not currently exercise this branch through gRPC; the spec calls out
// the gap and instructs the Go port to lock it down.
func TestGetUserTenants_EmptyUserID(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.GetUserTenants(context.Background(), &pb.GetUserTenantsRequest{
		Actor:  "user:alice",
		UserId: "",
	})
	if err == nil {
		t.Fatalf("GetUserTenants: expected INVALID_ARGUMENT, got nil")
	}
	if got := errs.Code(err); got != codes.InvalidArgument {
		t.Fatalf("code = %v; want InvalidArgument (err=%v)", got, err)
	}
}

// TestGetUserTenants_NoGlobalStore: when the server has no globalstore
// wired, the handler returns UNIMPLEMENTED with the canonical message.
// Mirrors grpc_server.py:2580-2584.
func TestGetUserTenants_NoGlobalStore(t *testing.T) {
	t.Parallel()
	srv := api.New() // no WithGlobalStore

	_, err := srv.GetUserTenants(context.Background(), &pb.GetUserTenantsRequest{
		Actor:  "user:alice",
		UserId: "alice",
	})
	if err == nil {
		t.Fatalf("GetUserTenants: expected UNIMPLEMENTED, got nil")
	}
	if got := errs.Code(err); got != codes.Unimplemented {
		t.Fatalf("code = %v; want Unimplemented (err=%v)", got, err)
	}
}

// TestGetUserTenants_InternalErrorReturnsEmptyOK pins the unusual but
// canonical degradation contract: any error from the global_store
// query is caught, logged, and collapsed to OK + memberships=[]. We
// trigger the error by closing the underlying SQLite handle out from
// under the running query (a closed *sql.DB returns ErrConnDone).
// Mirrors grpc_server.py:2596-2599.
func TestGetUserTenants_InternalErrorReturnsEmptyOK(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	// Closing the store makes any subsequent QueryContext return
	// sql.ErrConnDone -- the closest we get to "global_store is broken
	// at runtime" without inventing a fault-injection driver.
	if err := gs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	resp, err := srv.GetUserTenants(context.Background(), &pb.GetUserTenantsRequest{
		Actor:  "user:alice",
		UserId: "alice",
	})
	if err != nil {
		t.Fatalf("GetUserTenants: expected OK degradation, got err=%v", err)
	}
	if resp == nil {
		t.Fatalf("GetUserTenants: response is nil; want empty memberships")
	}
	if resp.Memberships == nil {
		t.Fatalf("GetUserTenants: Memberships is nil; want empty slice")
	}
	if len(resp.Memberships) != 0 {
		t.Fatalf("GetUserTenants: Memberships = %v; want [] (degradation pin)", resp.Memberships)
	}
}
