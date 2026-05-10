// SPDX-License-Identifier: AGPL-3.0-only

// Tests for RemoveTenantMember (EPIC #407, Wave 2). The Go port closes
// the auth-parity gap against Python — see RemoveTenantMember.md
// "Auth (admin only; trusted-actor)".

package api_test

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// seedTenantWithMembers creates `tenantID` and inserts the supplied
// (user_id, role) memberships. Helper for RemoveTenantMember tests.
func seedTenantWithMembers(t *testing.T, gs *globalstore.GlobalStore, tenantID string, members map[string]string) {
	t.Helper()
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, tenantID, tenantID, "us-east-1"); err != nil {
		t.Fatalf("CreateTenant(%q): %v", tenantID, err)
	}
	for uid, role := range members {
		if err := gs.AddTenantMember(ctx, tenantID, uid, role); err != nil {
			t.Fatalf("AddTenantMember(%q,%q,%q): %v", tenantID, uid, role, err)
		}
	}
}

// TestRemoveTenantMember_AdminHappyPath: an admin caller removes a
// regular member; the row is deleted and Success is true. Mirrors
// test_tenant_registry.py:462-479 (Python happy path), with the added
// admin-role enforcement that Go layers on top.
func TestRemoveTenantMember_AdminHappyPath(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	seedTenantWithMembers(t, gs, "acme", map[string]string{
		"alice": "owner",
		"carol": "admin",
		"bob":   "member",
	})

	srv := api.New(api.WithGlobalStore(gs))

	// Trusted actor on ctx is admin "carol"; the wire actor matches.
	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodOAuth,
		Subject: "user:carol",
	})

	resp, err := srv.RemoveTenantMember(ctx, &pb.TenantMemberRequest{
		Actor:    "user:carol",
		TenantId: "acme",
		UserId:   "bob",
	})
	if err != nil {
		t.Fatalf("RemoveTenantMember: unexpected error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("RemoveTenantMember: success=false, want true (resp=%+v)", resp)
	}
	if resp.GetError() != "" {
		t.Fatalf("RemoveTenantMember: error=%q, want empty", resp.GetError())
	}

	// Verify bob is gone, owner + admin remain.
	members, err := gs.GetTenantMembers(context.Background(), "acme")
	if err != nil {
		t.Fatalf("GetTenantMembers: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("members after delete: got %d, want 2 (rows=%+v)", len(members), members)
	}
	for _, m := range members {
		if m.UserID == "bob" {
			t.Fatalf("RemoveTenantMember: bob still present (%+v)", m)
		}
	}
}

// TestRemoveTenantMember_NonAdminDenied: a regular member trying to
// remove another member is rejected with PERMISSION_DENIED. This is
// the auth-parity-gap fix — Python lets this succeed (latent privilege
// escalation; spec §"Auth").
func TestRemoveTenantMember_NonAdminDenied(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	seedTenantWithMembers(t, gs, "acme", map[string]string{
		"alice": "owner",
		"eve":   "member",
		"bob":   "member",
	})

	srv := api.New(api.WithGlobalStore(gs))

	// Trusted actor: eve, who is just a "member" — not admin/owner.
	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodOAuth,
		Subject: "user:eve",
	})

	resp, err := srv.RemoveTenantMember(ctx, &pb.TenantMemberRequest{
		Actor:    "user:eve",
		TenantId: "acme",
		UserId:   "bob",
	})
	if err == nil {
		t.Fatalf("RemoveTenantMember: expected PERMISSION_DENIED, got resp=%+v", resp)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("RemoveTenantMember: not a grpc status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("RemoveTenantMember: code=%v, want PermissionDenied", st.Code())
	}

	// Bob must still be present — no row was deleted.
	members, err := gs.GetTenantMembers(context.Background(), "acme")
	if err != nil {
		t.Fatalf("GetTenantMembers: %v", err)
	}
	bobPresent := false
	for _, m := range members {
		if m.UserID == "bob" {
			bobPresent = true
			break
		}
	}
	if !bobPresent {
		t.Fatalf("RemoveTenantMember: denied, but bob was deleted anyway (rows=%+v)", members)
	}
}

// TestRemoveTenantMember_LastOwnerBlocked: removing the only "owner"
// returns success=false with the canonical "last owner" error string.
// Mirrors test_tenant_registry.py:481-495.
func TestRemoveTenantMember_LastOwnerBlocked(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	seedTenantWithMembers(t, gs, "acme", map[string]string{
		"alice": "owner",
		"bob":   "member",
	})

	srv := api.New(api.WithGlobalStore(gs))

	// alice is the sole owner — and is calling. As owner, she passes
	// the admin gate.
	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodOAuth,
		Subject: "user:alice",
	})

	resp, err := srv.RemoveTenantMember(ctx, &pb.TenantMemberRequest{
		Actor:    "user:alice",
		TenantId: "acme",
		UserId:   "alice",
	})
	if err != nil {
		t.Fatalf("RemoveTenantMember: unexpected error: %v", err)
	}
	if resp.GetSuccess() {
		t.Fatalf("RemoveTenantMember: success=true, want false (resp=%+v)", resp)
	}
	if !strings.Contains(strings.ToLower(resp.GetError()), "last owner") {
		t.Fatalf("RemoveTenantMember: error=%q, want substring %q", resp.GetError(), "last owner")
	}

	// alice must still be present.
	members, err := gs.GetTenantMembers(context.Background(), "acme")
	if err != nil {
		t.Fatalf("GetTenantMembers: %v", err)
	}
	alicePresent := false
	for _, m := range members {
		if m.UserID == "alice" {
			alicePresent = true
			break
		}
	}
	if !alicePresent {
		t.Fatalf("RemoveTenantMember: last owner was deleted anyway (rows=%+v)", members)
	}
}

// TestRemoveTenantMember_IdempotentNonMember: removing a user who is
// not a member returns success=false with "Member not found" — gRPC
// code OK (idempotent no-op). Mirrors test_tenant_registry.py:513-526.
func TestRemoveTenantMember_IdempotentNonMember(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	seedTenantWithMembers(t, gs, "acme", map[string]string{
		"alice": "owner",
	})

	srv := api.New(api.WithGlobalStore(gs))

	// admin caller bypasses the role gate via the system: prefix on
	// the trusted Identity.
	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodOAuth,
		Subject: "system:admin",
	})

	resp, err := srv.RemoveTenantMember(ctx, &pb.TenantMemberRequest{
		Actor:    "system:admin",
		TenantId: "acme",
		UserId:   "ghost",
	})
	if err != nil {
		t.Fatalf("RemoveTenantMember: unexpected error: %v", err)
	}
	if resp.GetSuccess() {
		t.Fatalf("RemoveTenantMember: success=true, want false (resp=%+v)", resp)
	}
	if !strings.Contains(strings.ToLower(resp.GetError()), "not found") {
		t.Fatalf("RemoveTenantMember: error=%q, want substring %q", resp.GetError(), "not found")
	}
}
