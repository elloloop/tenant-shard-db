// Tests for AddTenantMember. Behavioural parity with the Python handler
// (server/python/entdb_server/api/grpc_server.py:2442-2490) is pinned by
// the cross-language contract suite at
// tests/python/integration/test_grpc_contract.py:511-530 and the unit
// tests at tests/python/unit/test_tenant_registry.py:404-449. This file
// covers the four branches the spec calls out:
//
//   1. Admin happy path -> OK + success=true; row inserted.
//   2. Non-admin caller -> PERMISSION_DENIED.
//   3. Duplicate (tenant_id, user_id) -> OK + success=false, no gRPC error.
//   4. Empty fields -> INVALID_ARGUMENT.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// TestAddTenantMember_AdminHappyPath: a trusted admin: actor adds a
// new member; handler returns OK + success=true and the row lands in
// globalstore.tenant_members.
func TestAddTenantMember_AdminHappyPath(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	// Trusted admin identity attested by the (simulated) interceptor.
	// Wire-claimed actor matches but, per the trusted-actor invariant,
	// would be ignored if it didn't.
	authedCtx := auth.WithIdentity(ctx, auth.Identity{
		Method:  auth.MethodAPIKey,
		Subject: "admin:root",
	})

	resp, err := srv.AddTenantMember(authedCtx, &pb.TenantMemberRequest{
		Actor:    "admin:root",
		TenantId: "acme",
		UserId:   "charlie",
		Role:     "member",
	})
	if err != nil {
		t.Fatalf("AddTenantMember: unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("AddTenantMember: nil response")
	}
	if !resp.GetSuccess() {
		t.Fatalf("AddTenantMember: success=false, error=%q; want success=true", resp.GetError())
	}

	// Confirm the row landed.
	members, err := gs.GetTenantMembers(ctx, "acme")
	if err != nil {
		t.Fatalf("GetTenantMembers: %v", err)
	}
	var found *string
	for _, m := range members {
		if m.UserID == "charlie" {
			r := m.Role
			found = &r
			break
		}
	}
	if found == nil {
		t.Fatalf("GetTenantMembers: charlie row missing; got %+v", members)
	}
	if *found != "member" {
		t.Fatalf("GetTenantMembers: charlie role = %q; want %q", *found, "member")
	}
}

// TestAddTenantMember_NonAdminPermissionDenied: a regular user without
// owner/admin role for the target tenant gets PERMISSION_DENIED. Pins
// the membership-based admin check (grpc_server.py:2468-2474) and
// implicitly the trusted-actor invariant — the wire actor claims
// "system:admin" but the interceptor-attested identity wins.
func TestAddTenantMember_NonAdminPermissionDenied(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	// Bob is a plain member — not owner, not admin.
	if err := gs.AddTenantMember(ctx, "acme", "bob", "member"); err != nil {
		t.Fatalf("seed AddTenantMember: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	// Privilege-escalation regression pin (commit fece3fb): bob's
	// session is bound to user:bob, but the request claims
	// actor=system:admin. The handler MUST ignore the wire claim and
	// authorise as user:bob.
	authedCtx := auth.WithIdentity(ctx, auth.Identity{
		Method:  auth.MethodSession,
		Subject: "user:bob",
	})

	_, err := srv.AddTenantMember(authedCtx, &pb.TenantMemberRequest{
		Actor:    "system:admin", // wire-claimed escalation attempt
		TenantId: "acme",
		UserId:   "charlie",
		Role:     "member",
	})
	if err == nil {
		t.Fatalf("AddTenantMember: expected PERMISSION_DENIED, got nil")
	}
	if got := errs.Code(err); got != codes.PermissionDenied {
		t.Fatalf("AddTenantMember: code = %v, want PermissionDenied (err=%v)", got, err)
	}
}

// TestAddTenantMember_DuplicateSoftFailure: a second AddTenantMember
// for the same (tenant_id, user_id) returns gRPC OK with
// success=false and the canonical error string. Pinned by
// grpc_server.py:2484-2487 and the SDK idempotent-replay contract.
func TestAddTenantMember_DuplicateSoftFailure(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))
	authedCtx := auth.WithIdentity(ctx, auth.Identity{
		Method:  auth.MethodAPIKey,
		Subject: "admin:root",
	})

	req := &pb.TenantMemberRequest{
		Actor:    "admin:root",
		TenantId: "acme",
		UserId:   "charlie",
		Role:     "member",
	}

	// First call: lands the row.
	resp1, err := srv.AddTenantMember(authedCtx, req)
	if err != nil {
		t.Fatalf("AddTenantMember (first): %v", err)
	}
	if !resp1.GetSuccess() {
		t.Fatalf("AddTenantMember (first): success=false, error=%q", resp1.GetError())
	}

	// Second call: duplicate — soft failure.
	resp2, err := srv.AddTenantMember(authedCtx, req)
	if err != nil {
		t.Fatalf("AddTenantMember (duplicate): expected nil error, got %v", err)
	}
	if resp2 == nil {
		t.Fatalf("AddTenantMember (duplicate): nil response")
	}
	if resp2.GetSuccess() {
		t.Fatalf("AddTenantMember (duplicate): success=true; want false")
	}
	const wantMsg = "Member already exists in this tenant"
	if resp2.GetError() != wantMsg {
		t.Fatalf("AddTenantMember (duplicate): error = %q; want %q", resp2.GetError(), wantMsg)
	}
}

// TestAddTenantMember_EmptyFieldsInvalidArgument: each of actor /
// tenant_id / user_id, when empty, surfaces INVALID_ARGUMENT BEFORE
// any identity work. Pinned by test_grpc_contract.py:519-523 (empty
// actor) and grpc_server.py:2456-2461.
func TestAddTenantMember_EmptyFieldsInvalidArgument(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))
	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodAPIKey,
		Subject: "admin:root",
	})

	cases := []struct {
		name string
		req  *pb.TenantMemberRequest
	}{
		{
			name: "empty actor",
			req: &pb.TenantMemberRequest{
				Actor:    "",
				TenantId: "acme",
				UserId:   "charlie",
			},
		},
		{
			name: "empty tenant_id",
			req: &pb.TenantMemberRequest{
				Actor:    "admin:root",
				TenantId: "",
				UserId:   "charlie",
			},
		},
		{
			name: "empty user_id",
			req: &pb.TenantMemberRequest{
				Actor:    "admin:root",
				TenantId: "acme",
				UserId:   "",
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := srv.AddTenantMember(ctx, tt.req)
			if err == nil {
				t.Fatalf("AddTenantMember: expected INVALID_ARGUMENT, got nil")
			}
			if got := errs.Code(err); got != codes.InvalidArgument {
				t.Fatalf("AddTenantMember: code = %v, want InvalidArgument (err=%v)", got, err)
			}
		})
	}
}
