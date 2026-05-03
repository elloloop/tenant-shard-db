package entdb

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

// Tests for the v1.5 Admin namespace — the 9 cross-tenant identity
// RPCs grouped under client.Admin().
//
// Each test asserts:
//   - the typed Go return shape decodes correctly from a fake response,
//   - the wire request shape (so a future proto edit that moves a
//     field tag trips the test), and
//   - the failure modes that have a typed surface (NotFound,
//     PermissionDenied, server-side success=false → ADMIN_ERROR).

// ── CreateTenant ──────────────────────────────────────────────────────

func TestAdmin_CreateTenant_HappyPath(t *testing.T) {
	svc := &fakeServer{
		createTenantResp: &pb.CreateTenantResponse{
			Success: true,
			Tenant: &pb.TenantDetail{
				TenantId:  "acme",
				Name:      "Acme Corp",
				Status:    "active",
				CreatedAt: 1_700_000_000_000,
			},
		},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	td, err := c.Admin().CreateTenant(context.Background(), "system:admin", "acme", "Acme Corp")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if td.TenantID != "acme" || td.Name != "Acme Corp" || td.Status != "active" {
		t.Errorf("decoded tenant mismatch: %+v", td)
	}
	if svc.createTenantReq.GetActor() != "system:admin" ||
		svc.createTenantReq.GetTenantId() != "acme" ||
		svc.createTenantReq.GetName() != "Acme Corp" {
		t.Errorf("request shape wrong: %+v", svc.createTenantReq)
	}
}

func TestAdmin_CreateTenant_ServerSuccessFalseSurfacesAdminError(t *testing.T) {
	svc := &fakeServer{
		createTenantResp: &pb.CreateTenantResponse{
			Success: false,
			Error:   "tenant 'acme' already exists",
		},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	_, err := c.Admin().CreateTenant(context.Background(), "system:admin", "acme", "Acme")
	var ee *EntDBError
	if !errors.As(err, &ee) {
		t.Fatalf("want *EntDBError, got %T", err)
	}
	if ee.Code != "ADMIN_ERROR" {
		t.Errorf("want code ADMIN_ERROR, got %q", ee.Code)
	}
}

// ── CreateUser ────────────────────────────────────────────────────────

func TestAdmin_CreateUser_HappyPath(t *testing.T) {
	svc := &fakeServer{
		createUserResp: &pb.CreateUserResponse{
			Success: true,
			User: &pb.UserInfo{
				UserId:    "alice",
				Email:     "alice@acme.test",
				Name:      "Alice",
				Status:    "active",
				CreatedAt: 1_700_000_000_000,
				UpdatedAt: 1_700_000_000_000,
			},
		},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	u, err := c.Admin().CreateUser(context.Background(), "system:admin", "alice", "alice@acme.test", "Alice")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.UserID != "alice" || u.Email != "alice@acme.test" || u.Status != "active" {
		t.Errorf("decoded user mismatch: %+v", u)
	}
	if svc.createUserReq.GetEmail() != "alice@acme.test" {
		t.Errorf("request shape wrong: %+v", svc.createUserReq)
	}
}

func TestAdmin_CreateUser_DuplicateEmailIsAlreadyExists(t *testing.T) {
	svc := &fakeServer{
		createUserErr: status.Error(codes.AlreadyExists, "email is taken"),
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	_, err := c.Admin().CreateUser(context.Background(), "system:admin", "alice", "dup@x.test", "Alice")
	if err == nil {
		t.Fatal("expected error for duplicate email")
	}
	// gRPC AlreadyExists routes through the SDK's typed
	// UniqueConstraintError so callers can errors.As on it the same
	// way they would for a duplicate sku on Plan.Commit.
	var uce *UniqueConstraintError
	if !errors.As(err, &uce) {
		t.Fatalf("want *UniqueConstraintError, got %T (%v)", err, err)
	}
}

// ── AddTenantMember / RemoveTenantMember / ChangeMemberRole ───────────

func TestAdmin_AddTenantMember_HappyPath(t *testing.T) {
	svc := &fakeServer{
		addTenantMemberResp: &pb.TenantMemberResponse{Success: true},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	if err := c.Admin().AddTenantMember(context.Background(), "system:admin", "acme", "alice", "member"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}
	if svc.addTenantMemberReq.GetRole() != "member" || svc.addTenantMemberReq.GetUserId() != "alice" {
		t.Errorf("request shape wrong: %+v", svc.addTenantMemberReq)
	}
}

func TestAdmin_RemoveTenantMember_HappyPath(t *testing.T) {
	svc := &fakeServer{
		rmTenantMemberResp: &pb.TenantMemberResponse{Success: true},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	if err := c.Admin().RemoveTenantMember(context.Background(), "system:admin", "acme", "alice"); err != nil {
		t.Fatalf("RemoveTenantMember: %v", err)
	}
	if svc.rmTenantMemberReq.GetUserId() != "alice" {
		t.Errorf("request shape wrong: %+v", svc.rmTenantMemberReq)
	}
}

func TestAdmin_ChangeMemberRole_HappyPath(t *testing.T) {
	svc := &fakeServer{
		changeRoleResp: &pb.ChangeMemberRoleResponse{Success: true},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	if err := c.Admin().ChangeMemberRole(context.Background(), "system:admin", "acme", "alice", "admin"); err != nil {
		t.Fatalf("ChangeMemberRole: %v", err)
	}
	if svc.changeRoleReq.GetNewRole() != "admin" {
		t.Errorf("request shape wrong: %+v", svc.changeRoleReq)
	}
}

// ── GetTenantMembers / GetUserTenants ─────────────────────────────────

func TestAdmin_GetTenantMembers_HappyPath(t *testing.T) {
	svc := &fakeServer{
		getTenantMembersResp: &pb.GetTenantMembersResponse{
			Members: []*pb.TenantMemberInfo{
				{TenantId: "acme", UserId: "alice", Role: "admin", JoinedAt: 100},
				{TenantId: "acme", UserId: "bob", Role: "member", JoinedAt: 200},
			},
		},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	members, err := c.Admin().GetTenantMembers(context.Background(), "system:admin", "acme")
	if err != nil {
		t.Fatalf("GetTenantMembers: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("want 2 members, got %d", len(members))
	}
	if members[0].UserID != "alice" || members[0].Role != "admin" {
		t.Errorf("member[0] = %+v", members[0])
	}
	if members[1].JoinedAt != 200 {
		t.Errorf("joined_at not propagated: %+v", members[1])
	}
}

func TestAdmin_GetUserTenants_HappyPath(t *testing.T) {
	svc := &fakeServer{
		getUserTenantsResp: &pb.GetUserTenantsResponse{
			Memberships: []*pb.TenantMemberInfo{
				{TenantId: "acme", UserId: "alice", Role: "admin"},
				{TenantId: "globex", UserId: "alice", Role: "viewer"},
			},
		},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	tenancies, err := c.Admin().GetUserTenants(context.Background(), "system:admin", "alice")
	if err != nil {
		t.Fatalf("GetUserTenants: %v", err)
	}
	if len(tenancies) != 2 {
		t.Fatalf("want 2 tenancies, got %d", len(tenancies))
	}
	if tenancies[0].TenantID != "acme" || tenancies[1].TenantID != "globex" {
		t.Errorf("tenancy ids = %v", []string{tenancies[0].TenantID, tenancies[1].TenantID})
	}
	if svc.getUserTenantsReq.GetUserId() != "alice" {
		t.Errorf("request shape wrong: %+v", svc.getUserTenantsReq)
	}
}

// ── DelegateAccess ────────────────────────────────────────────────────

func TestAdmin_DelegateAccess_PropagatesExpiry(t *testing.T) {
	svc := &fakeServer{
		delegateResp: &pb.DelegateAccessResponse{
			Success:   true,
			Delegated: 17,
			ExpiresAt: 9_999_000,
		},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	res, err := c.Admin().DelegateAccess(
		context.Background(), "user:admin", "acme",
		"user:alice", "user:bob", "read", 9_999_000,
	)
	if err != nil {
		t.Fatalf("DelegateAccess: %v", err)
	}
	if res.Delegated != 17 || res.ExpiresAtMs != 9_999_000 {
		t.Errorf("decoded result mismatch: %+v", res)
	}
	if svc.delegateReq.GetExpiresAt() != 9_999_000 ||
		svc.delegateReq.GetPermission() != "read" {
		t.Errorf("request shape wrong: %+v", svc.delegateReq)
	}
}

func TestAdmin_DelegateAccess_PermanentDelegationUsesZeroExpiry(t *testing.T) {
	// expires_at=0 means "never expires" — the SDK does not
	// special-case it, the server reads 0 as "no expiry."
	svc := &fakeServer{
		delegateResp: &pb.DelegateAccessResponse{Success: true, Delegated: 5},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	_, err := c.Admin().DelegateAccess(
		context.Background(), "user:admin", "acme",
		"user:alice", "user:bob", "write", 0,
	)
	if err != nil {
		t.Fatalf("DelegateAccess: %v", err)
	}
	if svc.delegateReq.GetExpiresAt() != 0 {
		t.Errorf("permanent delegation should send expires_at=0, got %d", svc.delegateReq.GetExpiresAt())
	}
}

// ── TransferUserContent ───────────────────────────────────────────────

func TestAdmin_TransferUserContent_ReturnsCount(t *testing.T) {
	svc := &fakeServer{
		xferContentResp: &pb.TransferUserContentResponse{
			Success:     true,
			Transferred: 42,
		},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	count, err := c.Admin().TransferUserContent(
		context.Background(), "user:admin", "acme",
		"user:alice", "user:manager",
	)
	if err != nil {
		t.Fatalf("TransferUserContent: %v", err)
	}
	if count != 42 {
		t.Errorf("want 42 transferred, got %d", count)
	}
	if svc.xferContentReq.GetFromUser() != "user:alice" ||
		svc.xferContentReq.GetToUser() != "user:manager" {
		t.Errorf("request shape wrong: %+v", svc.xferContentReq)
	}
}

// ── RevokeAllUserAccess ───────────────────────────────────────────────

func TestAdmin_RevokeAllUserAccess_TalliesAllThreeBuckets(t *testing.T) {
	svc := &fakeServer{
		revokeAllResp: &pb.RevokeAllUserAccessResponse{
			Success:        true,
			RevokedGrants:  17,
			RevokedGroups:  3,
			RevokedShared:  8,
		},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	res, err := c.Admin().RevokeAllUserAccess(
		context.Background(), "user:admin", "acme", "alice",
	)
	if err != nil {
		t.Fatalf("RevokeAllUserAccess: %v", err)
	}
	if res.RevokedGrants != 17 || res.RevokedGroups != 3 || res.RevokedShared != 8 {
		t.Errorf("decoded tally mismatch: %+v", res)
	}
}

// ── DbClient.Admin() returns a usable handle ──────────────────────────

func TestDbClient_AdminAccessor(t *testing.T) {
	mt := &mockTransport{}
	c := newClientWithTransport("addr", mt)
	a := c.Admin()
	if a == nil {
		t.Fatal("Admin() returned nil")
	}
	// Smoke-test that admin methods route through the transport
	// without panicking — full per-method behaviour covered above.
	_, _ = a.CreateTenant(context.Background(), "system:admin", "acme", "Acme")
	_, _ = a.CreateUser(context.Background(), "system:admin", "alice", "a@x.test", "Alice")
	_ = a.AddTenantMember(context.Background(), "system:admin", "acme", "alice", "member")
}
