// Tests for ChangeMemberRole. Behavioural pins are documented in
// docs/go-port/rpcs/ChangeMemberRole.md and the Python contract suite
// (tests/python/unit/test_tenant_registry.py:585-686). The Go-side
// drift this file enforces is the last-owner demotion protection
// (PLAN.md §6) — the Python handler does NOT have this and the Go
// port adds it.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// changeRoleFixture seeds a tenant with the canonical 3-member shape
// used across these tests:
//
//	alice -> owner
//	bob   -> admin
//	carol -> member
//
// Returns the wired Server and the underlying GlobalStore so individual
// tests can re-seed extra rows or reach into the store for assertions.
func changeRoleFixture(t *testing.T) (*api.Server, *globalstore.GlobalStore, context.Context) {
	t.Helper()
	gs := newGlobalStore(t)
	ctx := context.Background()

	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := gs.AddTenantMember(ctx, "acme", "alice", "owner"); err != nil {
		t.Fatalf("AddTenantMember alice: %v", err)
	}
	if err := gs.AddTenantMember(ctx, "acme", "bob", "admin"); err != nil {
		t.Fatalf("AddTenantMember bob: %v", err)
	}
	if err := gs.AddTenantMember(ctx, "acme", "carol", "member"); err != nil {
		t.Fatalf("AddTenantMember carol: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))
	return srv, gs, ctx
}

// TestChangeMemberRole_AdminPromotesMember pins the happy path: an
// existing tenant admin (bob) promotes a member (carol) to admin.
func TestChangeMemberRole_AdminPromotesMember(t *testing.T) {
	t.Parallel()

	srv, gs, ctx := changeRoleFixture(t)

	resp, err := srv.ChangeMemberRole(ctx, &pb.ChangeMemberRoleRequest{
		Actor:    "user:bob",
		TenantId: "acme",
		UserId:   "carol",
		NewRole:  "admin",
	})
	if err != nil {
		t.Fatalf("ChangeMemberRole: unexpected error: %v", err)
	}
	if resp == nil || !resp.GetSuccess() {
		t.Fatalf("ChangeMemberRole: success=false; resp=%+v", resp)
	}

	// Verify the row actually moved.
	members, err := gs.GetTenantMembers(ctx, "acme")
	if err != nil {
		t.Fatalf("GetTenantMembers: %v", err)
	}
	got := ""
	for _, m := range members {
		if m.UserID == "carol" {
			got = m.Role
		}
	}
	if got != "admin" {
		t.Fatalf("carol role = %q, want %q", got, "admin")
	}
}

// TestChangeMemberRole_AdminDemotesAdmin pins the demote path and
// confirms admin role grants the privilege (not just owner — Go
// widens the Python "owner-only" rule).
func TestChangeMemberRole_AdminDemotesAdmin(t *testing.T) {
	t.Parallel()

	srv, gs, ctx := changeRoleFixture(t)

	// Add a second admin so bob can demote one without touching himself.
	if err := gs.AddTenantMember(ctx, "acme", "dave", "admin"); err != nil {
		t.Fatalf("AddTenantMember dave: %v", err)
	}

	resp, err := srv.ChangeMemberRole(ctx, &pb.ChangeMemberRoleRequest{
		Actor:    "user:bob",
		TenantId: "acme",
		UserId:   "dave",
		NewRole:  "member",
	})
	if err != nil {
		t.Fatalf("ChangeMemberRole: unexpected error: %v", err)
	}
	if resp == nil || !resp.GetSuccess() {
		t.Fatalf("ChangeMemberRole: success=false; resp=%+v", resp)
	}

	members, err := gs.GetTenantMembers(ctx, "acme")
	if err != nil {
		t.Fatalf("GetTenantMembers: %v", err)
	}
	got := ""
	for _, m := range members {
		if m.UserID == "dave" {
			got = m.Role
		}
	}
	if got != "member" {
		t.Fatalf("dave role = %q, want %q", got, "member")
	}
}

// TestChangeMemberRole_NonAdminDenied pins the negative path: a plain
// member (carol) attempting to mutate roles is rejected with
// PERMISSION_DENIED. Mirrors the Python pin
// `test_change_role_by_member_denied`.
func TestChangeMemberRole_NonAdminDenied(t *testing.T) {
	t.Parallel()

	srv, _, ctx := changeRoleFixture(t)

	_, err := srv.ChangeMemberRole(ctx, &pb.ChangeMemberRoleRequest{
		Actor:    "user:carol",
		TenantId: "acme",
		UserId:   "bob",
		NewRole:  "member",
	})
	if err == nil {
		t.Fatalf("ChangeMemberRole: expected PERMISSION_DENIED, got nil")
	}
	if got := errs.Code(err); got != codes.PermissionDenied {
		t.Fatalf("ChangeMemberRole: code = %v, want PermissionDenied (err=%v)", got, err)
	}
}

// TestChangeMemberRole_LastOwnerDemotionBlocked pins the Go-side drift
// from PLAN.md §6: demoting the sole owner of a tenant must surface
// FAILED_PRECONDITION rather than silently bricking admin access.
//
// Setup: only alice is an owner. A system actor (which bypasses the
// tenant-membership privilege check) attempts to demote alice — the
// guard MUST still fire because the protection is independent of the
// caller's privilege.
func TestChangeMemberRole_LastOwnerDemotionBlocked(t *testing.T) {
	t.Parallel()

	srv, gs, ctx := changeRoleFixture(t)

	_, err := srv.ChangeMemberRole(ctx, &pb.ChangeMemberRoleRequest{
		Actor:    "system:admin",
		TenantId: "acme",
		UserId:   "alice",
		NewRole:  "member",
	})
	if err == nil {
		t.Fatalf("ChangeMemberRole: expected FAILED_PRECONDITION, got nil")
	}
	if got := errs.Code(err); got != codes.FailedPrecondition {
		t.Fatalf("ChangeMemberRole: code = %v, want FailedPrecondition (err=%v)", got, err)
	}

	// Sanity: alice is still an owner.
	members, err := gs.GetTenantMembers(ctx, "acme")
	if err != nil {
		t.Fatalf("GetTenantMembers: %v", err)
	}
	got := ""
	for _, m := range members {
		if m.UserID == "alice" {
			got = m.Role
		}
	}
	if got != "owner" {
		t.Fatalf("alice role = %q, want %q (last-owner guard must not mutate)", got, "owner")
	}
}
