// SPDX-License-Identifier: AGPL-3.0-only

// Tests for the CreateTenant RPC. Pins the Wave-2 contract:
//
//   - Admin happy path → Success=true, registry row + owner membership.
//   - Non-admin trusted actor → PERMISSION_DENIED (Wave-2 admin gate).
//   - Duplicate tenant_id → ALREADY_EXISTS.
//   - Empty/invalid tenant_id (and other required fields) → INVALID_ARGUMENT.
//
// Spec: docs/go-port/rpcs/CreateTenant.md.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// adminCtx returns a context carrying a trusted admin Identity, the way
// the auth interceptor would after authenticating an admin caller. Tests
// that call the handler directly need to plumb this in themselves
// because no interceptor runs in the unit-test path.
func adminCtx() context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodSession,
		Subject: "admin:root",
	})
}

// userCtx returns a context carrying a trusted *non-admin* Identity. The
// admin gate must reject this even if the wire `actor` claims admin.
func userCtx() context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodSession,
		Subject: "user:alice",
	})
}

// TestCreateTenant_AdminHappyPath: admin caller, all required fields
// populated, region pin applied. Verifies the response shape and the
// side effects on globalstore (registry row + owner membership).
func TestCreateTenant_AdminHappyPath(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs), api.WithRegion("eu-west-1"))

	resp, err := srv.CreateTenant(adminCtx(), &pb.CreateTenantRequest{
		Actor:    "admin:root",
		TenantId: "acme",
		Name:     "Acme Corp",
		Region:   "us-east-1",
	})
	if err != nil {
		t.Fatalf("CreateTenant: unexpected err: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("CreateTenant: success=false, error=%q", resp.GetError())
	}
	if got := resp.GetTenant().GetTenantId(); got != "acme" {
		t.Fatalf("tenant_id: got %q, want %q", got, "acme")
	}
	if got := resp.GetTenant().GetName(); got != "Acme Corp" {
		t.Fatalf("name: got %q, want %q", got, "Acme Corp")
	}
	if got := resp.GetTenant().GetStatus(); got != "active" {
		t.Fatalf("status: got %q, want %q", got, "active")
	}
	if got := resp.GetTenant().GetRegion(); got != "us-east-1" {
		t.Fatalf("region: got %q, want %q", got, "us-east-1")
	}
	if resp.GetTenant().GetCreatedAt() <= 0 {
		t.Fatalf("created_at: got %d, want >0", resp.GetTenant().GetCreatedAt())
	}

	// Side effects: registry row exists and creator was recorded as owner.
	got, err := gs.GetTenant(context.Background(), "acme")
	if err != nil || got == nil {
		t.Fatalf("GetTenant: err=%v tenant=%v", err, got)
	}
	if got.Region != "us-east-1" {
		t.Fatalf("registry region: got %q, want %q", got.Region, "us-east-1")
	}
	members, err := gs.GetTenantMembers(context.Background(), "acme")
	if err != nil {
		t.Fatalf("GetTenantMembers: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("members: got %d, want 1 (%+v)", len(members), members)
	}
	// The admin's bare ID (without the "admin:" prefix) is what's stored
	// as the user_id, mirroring _actor_user_id in the Python source.
	if members[0].UserID != "root" {
		t.Fatalf("owner user_id: got %q, want %q", members[0].UserID, "root")
	}
	if members[0].Role != "owner" {
		t.Fatalf("owner role: got %q, want %q", members[0].Role, "owner")
	}
}

// TestCreateTenant_RegionDefaultsToServedRegion: empty req.region falls
// back to s.region. Pinned by the Python region-pin contract test at
// tests/python/integration/test_region_pinning.py:81-106.
func TestCreateTenant_RegionDefaultsToServedRegion(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs), api.WithRegion("eu-west-1"))

	resp, err := srv.CreateTenant(adminCtx(), &pb.CreateTenantRequest{
		Actor:    "admin:root",
		TenantId: "acme",
		Name:     "Acme Corp",
	})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if got := resp.GetTenant().GetRegion(); got != "eu-west-1" {
		t.Fatalf("region: got %q, want %q (server's served region)", got, "eu-west-1")
	}
}

// TestCreateTenant_NonAdminPermissionDenied: a trusted user: actor (the
// auth interceptor authenticated them as user:alice) hits the admin gate
// and gets PERMISSION_DENIED — even though the wire payload claims an
// admin: actor. This is the privilege-escalation regression guard.
func TestCreateTenant_NonAdminPermissionDenied(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.CreateTenant(userCtx(), &pb.CreateTenantRequest{
		Actor:    "admin:root", // wire claim is ignored; trusted actor wins
		TenantId: "acme",
		Name:     "Acme Corp",
	})
	if err == nil {
		t.Fatalf("CreateTenant: expected error, got nil")
	}
	if got := status.Code(err); got != codes.PermissionDenied {
		t.Fatalf("code: got %v, want PermissionDenied", got)
	}

	// Side-effect guarantee: no registry row should have been written.
	got, err := gs.GetTenant(context.Background(), "acme")
	if err != nil {
		t.Fatalf("GetTenant: %v", err)
	}
	if got != nil {
		t.Fatalf("registry row leaked despite PERMISSION_DENIED: %+v", got)
	}
}

// TestCreateTenant_DuplicateAlreadyExists: a second CreateTenant with the
// same tenant_id surfaces as ALREADY_EXISTS. (Wave-2 deviates from
// Python's success=false channel for typed-error ergonomics; spec note
// in CreateTenant.md "Error contract" allows the deviation in the Go
// port.)
func TestCreateTenant_DuplicateAlreadyExists(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs), api.WithRegion("us-east-1"))

	if _, err := srv.CreateTenant(adminCtx(), &pb.CreateTenantRequest{
		Actor:    "admin:root",
		TenantId: "acme",
		Name:     "Acme Corp",
	}); err != nil {
		t.Fatalf("first CreateTenant: %v", err)
	}

	_, err := srv.CreateTenant(adminCtx(), &pb.CreateTenantRequest{
		Actor:    "admin:root",
		TenantId: "acme",
		Name:     "Acme Corp v2",
	})
	if err == nil {
		t.Fatalf("CreateTenant: expected ALREADY_EXISTS, got nil")
	}
	if got := status.Code(err); got != codes.AlreadyExists {
		t.Fatalf("code: got %v, want AlreadyExists", got)
	}
}

// TestCreateTenant_InvalidArgument: each of the three required wire
// fields produces INVALID_ARGUMENT when empty. Mirrors the three
// abort sites in Python at grpc_server.py:2318-2323.
func TestCreateTenant_InvalidArgument(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		req  *pb.CreateTenantRequest
	}{
		{
			name: "empty_actor",
			req:  &pb.CreateTenantRequest{TenantId: "acme", Name: "Acme"},
		},
		{
			name: "empty_tenant_id",
			req:  &pb.CreateTenantRequest{Actor: "admin:root", Name: "Acme"},
		},
		{
			name: "empty_name",
			req:  &pb.CreateTenantRequest{Actor: "admin:root", TenantId: "acme"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			gs := newGlobalStore(t)
			srv := api.New(api.WithGlobalStore(gs))
			_, err := srv.CreateTenant(adminCtx(), c.req)
			if err == nil {
				t.Fatalf("CreateTenant: expected INVALID_ARGUMENT, got nil")
			}
			if got := status.Code(err); got != codes.InvalidArgument {
				t.Fatalf("code: got %v, want InvalidArgument", got)
			}
		})
	}
}

// TestCreateTenant_GlobalStoreUnconfigured: no globalstore wired →
// UNIMPLEMENTED. Mirrors Python's `abort(UNIMPLEMENTED, "Tenant
// registry not configured")` at grpc_server.py:2312-2316.
func TestCreateTenant_GlobalStoreUnconfigured(t *testing.T) {
	t.Parallel()
	srv := api.New() // no globalstore

	_, err := srv.CreateTenant(adminCtx(), &pb.CreateTenantRequest{
		Actor:    "admin:root",
		TenantId: "acme",
		Name:     "Acme Corp",
	})
	if err == nil {
		t.Fatalf("CreateTenant: expected UNIMPLEMENTED, got nil")
	}
	if got := status.Code(err); got != codes.Unimplemented {
		t.Fatalf("code: got %v, want Unimplemented", got)
	}
}
