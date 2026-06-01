// SPDX-License-Identifier: AGPL-3.0-only

// Security-audit tests for the read-surface authorization gaps.
//
// Two findings are pinned here:
//
//   #639 (synthesis rank #3, HIGH) USER_MAILBOX privacy bypass — FIXED.
//      GetNode / GetNodes / QueryNodes / SearchNodes took target_user off the
//      wire and confined the read to that user's mailbox with NO check that
//      the trusted caller is that user, so any tenant member could read
//      another user's private mailbox. Fixed by authorizeMailboxScope
//      (mailbox_authz.go), called in all four handlers before the store read.
//      The gates below are now active.
//
//   #640 (synthesis rank #4, HIGH) registry-read RPCs resolve then DISCARD the
//      trusted actor (get_tenant_members.go:80 does `_ = auth.Authoritative(
//      ...)`) with no authz gate. Any authenticated low-privilege caller dumps
//      any tenant's roster / any user's PII / the whole user registry. now
//      gated (this change) — the gates below are active.
//
// These tests stand up their own server wiring (newAuthzTestServer) so we own
// the globalstore handle and can register tenant members — the shared
// newSearchTestServer does not expose the globalstore it builds.
//
// Tests are package api_test; they reuse seedMailboxNode / searchTypeID from
// the sibling _test.go files.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// newAuthzTestServer mirrors newSearchTestServer but RETURNS the globalstore
// so the test can register tenant members (the membership row is what
// classifies an actor as roleMember in checkCrossTenantRead). The schema is
// identical to the search fixture (typeID=searchTypeID, two searchable string
// fields) so seedMailboxNode works unchanged.
//
// Note: with no auth interceptor on ctx, auth.Authoritative returns the
// wire-claimed actor verbatim, so the wire `actor` field drives role
// classification directly in these tests — exactly the production path when a
// caller authenticates as a real low-privilege user.
func newAuthzTestServer(t *testing.T) (*api.Server, *store.CanonicalStore, *globalstore.GlobalStore, string) {
	t.Helper()
	gs := newGlobalStore(t)
	ctx := context.Background()
	const tenantID = "acme"
	if _, err := gs.CreateTenant(ctx, tenantID, "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	reg := schema.NewRegistry()
	if err := reg.RegisterNode(&schema.NodeTypeDef{
		TypeID: searchTypeID,
		Fields: []schema.FieldDef{
			{FieldID: 1, Kind: schema.KindString, Searchable: true},
			{FieldID: 2, Kind: schema.KindString, Searchable: true},
		},
	}); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}

	cs, err := store.New(store.Options{
		RootDir:  t.TempDir(),
		WALMode:  true,
		Registry: reg,
	})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	// A WAL producer so write RPCs (ShareNode) reach their handler body
	// rather than the optional-deps guard; read RPCs ignore it.
	producer := wal.NewInMemory(0)
	if err := producer.Connect(ctx); err != nil {
		t.Fatalf("producer.Connect: %v", err)
	}
	t.Cleanup(func() { _ = producer.Close(ctx) })

	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithStore(cs),
		api.WithSchemaRegistry(reg),
		api.WithWALProducer(producer),
	)
	return srv, cs, gs, tenantID
}

// addMember registers userID as a plain "member" of tenantID. A member is
// classified roleMember by checkCrossTenantRead, which is the privilege level
// the #639 bypass required (a non-member with no grant is rejected at the
// cross-tenant gate before the mailbox read even runs).
func addMember(t *testing.T, gs *globalstore.GlobalStore, tenantID, userID string) {
	t.Helper()
	if err := gs.AddTenantMember(context.Background(), tenantID, userID, "member"); err != nil {
		t.Fatalf("AddTenantMember(%q): %v", userID, err)
	}
}

// -----------------------------------------------------------------------
// Finding #639 — USER_MAILBOX privacy bypass (FIXED). authorizeMailboxScope
// gates every mailbox-scoped read: only the target user or an admin/system
// actor may use target_user; an ordinary member naming another user is denied.
// -----------------------------------------------------------------------

// TestGetNode_MailboxBypass_SecureBehavior_Finding3 verifies the single-node
// read path: a non-owner member is denied, the owner and admin still read it.
func TestGetNode_MailboxBypass_SecureBehavior_Finding3(t *testing.T) {
	srv, cs, gs, tenantID := newAuthzTestServer(t)
	ctx := context.Background()
	addMember(t, gs, tenantID, "alice")
	addMember(t, gs, tenantID, "bob")
	seedMailboxNode(t, cs, tenantID, "bob", "mb", "secret", "bob private body")

	// A non-owner member MUST NOT read another user's mailbox node.
	resp, err := srv.GetNode(ctx, &pb.GetNodeRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: "user:alice"},
		NodeId:     "mb",
		TargetUser: "bob",
	})
	if err != nil {
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("alice -> bob mailbox: want PermissionDenied or Found=false, got err %v", err)
		}
	} else if resp.GetFound() {
		t.Fatal("alice -> bob mailbox: privacy bypass still open (Found=true)")
	}

	// The owner still reads their own mailbox node.
	own, err := srv.GetNode(ctx, &pb.GetNodeRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: "user:bob"},
		NodeId:     "mb",
		TargetUser: "bob",
	})
	if err != nil || !own.GetFound() {
		t.Fatalf("owner bob must still read his mailbox node: found=%v err=%v", own.GetFound(), err)
	}

	// An admin still can (system/admin bypass is intentional).
	adm, err := srv.GetNode(ctx, &pb.GetNodeRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: "system:test"},
		NodeId:     "mb",
		TargetUser: "bob",
	})
	if err != nil || !adm.GetFound() {
		t.Fatalf("admin must still read the mailbox node: found=%v err=%v", adm.GetFound(), err)
	}
}

// TestGetNodes_MailboxBypass_SecureBehavior_Finding3 verifies the batch path.
func TestGetNodes_MailboxBypass_SecureBehavior_Finding3(t *testing.T) {
	srv, cs, gs, tenantID := newAuthzTestServer(t)
	ctx := context.Background()
	addMember(t, gs, tenantID, "alice")
	addMember(t, gs, tenantID, "bob")
	seedMailboxNode(t, cs, tenantID, "bob", "mb", "secret", "bob private body")

	// Non-owner member: bob's node must NOT appear. Either reported missing
	// or the call is denied outright.
	resp, err := srv.GetNodes(ctx, &pb.GetNodesRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: "user:alice"},
		NodeIds:    []string{"mb"},
		TargetUser: "bob",
	})
	if err != nil {
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("alice -> bob mailbox batch: want PermissionDenied or missing, got err %v", err)
		}
	} else {
		for _, n := range resp.GetNodes() {
			if n.GetNodeId() == "mb" {
				t.Fatal("alice -> bob mailbox batch: privacy bypass still open (leaked mb)")
			}
		}
	}

	// Owner still gets it.
	own, err := srv.GetNodes(ctx, &pb.GetNodesRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: "user:bob"},
		NodeIds:    []string{"mb"},
		TargetUser: "bob",
	})
	if err != nil || len(own.GetNodes()) != 1 {
		t.Fatalf("owner bob must still read his mailbox node in batch: nodes=%d err=%v", len(own.GetNodes()), err)
	}
}

// TestSearchNodes_MailboxBypass_SecureBehavior_Finding3 verifies the FTS path.
func TestSearchNodes_MailboxBypass_SecureBehavior_Finding3(t *testing.T) {
	srv, cs, gs, tenantID := newAuthzTestServer(t)
	ctx := context.Background()
	addMember(t, gs, tenantID, "alice")
	addMember(t, gs, tenantID, "bob")
	seedMailboxNode(t, cs, tenantID, "bob", "mb", "subject", "keyword bobsecret")

	// Non-owner member: must see nothing from bob's mailbox.
	resp, err := srv.SearchNodes(ctx, &pb.SearchNodesRequest{
		TenantId:   tenantID,
		Actor:      "user:alice",
		TypeId:     searchTypeID,
		Query:      "keyword",
		TargetUser: "bob",
	})
	if err != nil {
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("alice -> bob mailbox search: want PermissionDenied or empty, got err %v", err)
		}
	} else if len(resp.GetNodes()) != 0 {
		t.Fatalf("alice -> bob mailbox search: privacy bypass still open (%d nodes)", len(resp.GetNodes()))
	}

	// Owner still finds their own node.
	own, err := srv.SearchNodes(ctx, &pb.SearchNodesRequest{
		TenantId:   tenantID,
		Actor:      "user:bob",
		TypeId:     searchTypeID,
		Query:      "keyword",
		TargetUser: "bob",
	})
	if err != nil || len(own.GetNodes()) != 1 {
		t.Fatalf("owner bob must still find his mailbox node: nodes=%d err=%v", len(own.GetNodes()), err)
	}
}

// TestQueryNodes_MailboxBypass_SecureBehavior_Finding3 verifies the query path.
// QueryNodes also ran a per-row ACL post-filter, but the explicit
// authorizeMailboxScope gate now denies a cross-user mailbox query outright so
// the boundary does not depend on that filter.
func TestQueryNodes_MailboxBypass_SecureBehavior_Finding3(t *testing.T) {
	srv, cs, gs, tenantID := newAuthzTestServer(t)
	ctx := context.Background()
	addMember(t, gs, tenantID, "alice")
	addMember(t, gs, tenantID, "bob")
	seedMailboxNode(t, cs, tenantID, "bob", "mb", "subject", "bob body")

	// Non-owner member: denied or empty, never bob's node.
	resp, err := srv.QueryNodes(ctx, &pb.QueryNodesRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: "user:alice"},
		TypeId:     searchTypeID,
		TargetUser: "bob",
		Limit:      100,
	})
	if err != nil {
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("alice -> bob mailbox query: want PermissionDenied or empty, got err %v", err)
		}
	} else {
		for _, n := range resp.GetNodes() {
			if n.GetNodeId() == "mb" {
				t.Fatal("alice -> bob mailbox query: privacy bypass still open (leaked mb)")
			}
		}
	}

	// Owner still queries their own mailbox.
	own, err := srv.QueryNodes(ctx, &pb.QueryNodesRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: "user:bob"},
		TypeId:     searchTypeID,
		TargetUser: "bob",
		Limit:      100,
	})
	if err != nil {
		t.Fatalf("owner bob mailbox query: unexpected err: %v", err)
	}
	found := false
	for _, n := range own.GetNodes() {
		if n.GetNodeId() == "mb" {
			found = true
		}
	}
	if !found {
		t.Fatal("owner bob must still see his mailbox node via QueryNodes")
	}
}

// TestGetNodeByKey_MailboxBypass_SecureBehavior_Finding3 closes the 5th
// mailbox-read path surfaced by adversarial review of the #639 fix:
// GetNodeByKey resolves a node by unique-key value (e.g. an email) with NO
// target_user scope and no storage_mode filter, and its only gate
// (checkNodeACL) passes on the empty ACL a mailbox node carries. The fix makes
// USER_MAILBOX nodes invisible to this RPC entirely, mirroring the plain by-id
// read. A non-owner member naming a mailbox node's key must NOT get the
// payload; a non-mailbox node by key is still resolvable.
func TestGetNodeByKey_MailboxBypass_SecureBehavior_Finding3(t *testing.T) {
	srv, cs, gs, tenantID := newAuthzTestServer(t)
	ctx := context.Background()
	addMember(t, gs, tenantID, "alice")
	addMember(t, gs, tenantID, "bob")
	// bob's private mailbox node; field 1 holds the (guessable) key value.
	seedMailboxNode(t, cs, tenantID, "bob", "mb", "bob-private-key", "bob private body")

	// alice (member, not bob, not admin) tries to resolve bob's mailbox node
	// by its key value — must NOT get it.
	resp, err := srv.GetNodeByKey(ctx, &pb.GetNodeByKeyRequest{
		TenantId: tenantID,
		Actor:    "user:alice",
		TypeId:   searchTypeID,
		FieldId:  1,
		Value:    structpb.NewStringValue("bob-private-key"),
	})
	if err != nil {
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("alice GetNodeByKey -> bob mailbox: want Found=false or PermissionDenied, got err %v", err)
		}
	} else if resp.GetFound() {
		t.Fatalf("GetNodeByKey leaked bob's mailbox node to alice (node_id=%q)", resp.GetNode().GetNodeId())
	}

	// Mailbox nodes are invisible to by-key for everyone (no target_user scope
	// on this RPC) — even the owner reaches their mailbox via the scoped reads.
	bobResp, err := srv.GetNodeByKey(ctx, &pb.GetNodeByKeyRequest{
		TenantId: tenantID, Actor: "user:bob", TypeId: searchTypeID, FieldId: 1,
		Value: structpb.NewStringValue("bob-private-key"),
	})
	if err == nil && bobResp.GetFound() {
		t.Fatal("GetNodeByKey must not expose USER_MAILBOX nodes even to the owner; use the target_user-scoped reads")
	}

	// Regression guard: a non-mailbox (tenant) node IS still resolvable by key.
	if _, err := cs.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID: "tn", TypeID: searchTypeID, OwnerActor: "user:alice",
		Payload: map[string]any{"1": "tenant-key", "2": "body"},
	}); err != nil {
		t.Fatalf("CreateNodeRaw(tenant node): %v", err)
	}
	tnResp, err := srv.GetNodeByKey(ctx, &pb.GetNodeByKeyRequest{
		TenantId: tenantID, Actor: "user:alice", TypeId: searchTypeID, FieldId: 1,
		Value: structpb.NewStringValue("tenant-key"),
	})
	if err != nil || !tnResp.GetFound() || tnResp.GetNode().GetNodeId() != "tn" {
		t.Fatalf("non-mailbox node must still be resolvable by key: found=%v err=%v", tnResp.GetFound(), err)
	}
}

// TestShareNode_RejectsMailboxNode_Finding3 is the root-cause prevention: a
// USER_MAILBOX node must not enter the ACL/sharing system at all, so even its
// owner cannot share it. This stops a mailbox node from ever acquiring a
// node_access / shared_index row (which would egress its metadata via
// ListSharedWithMe, incl. cross-tenant).
func TestShareNode_RejectsMailboxNode_Finding3(t *testing.T) {
	srv, cs, gs, tenantID := newAuthzTestServer(t)
	ctx := context.Background()
	addMember(t, gs, tenantID, "bob")
	// bob OWNS the mailbox node (so the ACL owner short-circuit passes); the
	// share must still be refused because it is a USER_MAILBOX node.
	seedMailboxNode(t, cs, tenantID, "bob", "mb", "subj", "body")

	resp, err := srv.ShareNode(ctx, &pb.ShareNodeRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: "user:bob"},
		NodeId:     "mb",
		ActorId:    "user:carol",
		Permission: "read",
	})
	if err != nil {
		t.Fatalf("ShareNode(mailbox): unexpected gRPC error: %v", err)
	}
	if resp.GetSuccess() {
		t.Fatal("ShareNode must refuse to share a USER_MAILBOX node (#639 root cause)")
	}
}

// TestListSharedWithMe_ExcludesMailbox_Finding3 closes the cross-tenant
// shared-with-me path: even if a mailbox node already has a cross-tenant
// shared_index row (e.g. from before the ShareNode rejection above), the
// recipient must not see it. The per-tenant SQL already excludes mailbox
// nodes; this pins the cross-tenant resolution path (store.GetNode) too.
func TestListSharedWithMe_ExcludesMailbox_Finding3(t *testing.T) {
	srv, cs, gs, tenantID := newAuthzTestServer(t)
	ctx := context.Background()
	// bob's private mailbox node in the source tenant.
	seedMailboxNode(t, cs, tenantID, "bob", "ma", "subj", "body")
	// Simulate a pre-existing cross-tenant shared_index row pointing eve at
	// bob's mailbox node (the now-prevented share).
	if err := gs.AddShared(ctx, "user:eve", tenantID, "ma", "read"); err != nil {
		t.Fatalf("AddShared: %v", err)
	}

	resp, err := srv.ListSharedWithMe(ctx, &pb.ListSharedWithMeRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "user:eve"},
		Limit:   100,
	})
	if err != nil {
		t.Fatalf("ListSharedWithMe(eve): %v", err)
	}
	for _, n := range resp.GetNodes() {
		if n.GetNodeId() == "ma" {
			t.Fatal("ListSharedWithMe leaked bob's USER_MAILBOX node to eve via the cross-tenant shared_index (#639)")
		}
	}
}

// -----------------------------------------------------------------------
// Finding #640 (FIXED): registry-read RPCs are now gated. GetUser /
// GetUserTenants require self-or-admin; GetTenantMembers requires
// membership-or-admin; ListUsers requires an admin/system actor.
// -----------------------------------------------------------------------

// TestGetTenantMembers_NoAuthzGate_SecureBehavior_Finding4 verifies a
// non-member, non-admin caller is denied while a member and an admin still
// read the roster.
func TestGetTenantMembers_NoAuthzGate_SecureBehavior_Finding4(t *testing.T) {
	srv, _, gs, tenantID := newAuthzTestServer(t)
	ctx := context.Background()
	addMember(t, gs, tenantID, "alice")
	addMember(t, gs, tenantID, "bob")

	// Stranger must be denied.
	if _, err := srv.GetTenantMembers(ctx, &pb.GetTenantMembersRequest{
		TenantId: tenantID, Actor: "user:mallory",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("non-member mallory: want PermissionDenied, got %v", err)
	}

	// A member can still read the roster.
	memResp, err := srv.GetTenantMembers(ctx, &pb.GetTenantMembersRequest{
		TenantId: tenantID, Actor: "user:alice",
	})
	if err != nil || len(memResp.GetMembers()) != 2 {
		t.Fatalf("member alice must still read the roster: members=%d err=%v", len(memResp.GetMembers()), err)
	}

	// An admin can still read the roster.
	admResp, err := srv.GetTenantMembers(ctx, &pb.GetTenantMembersRequest{
		TenantId: tenantID, Actor: "system:test",
	})
	if err != nil || len(admResp.GetMembers()) != 2 {
		t.Fatalf("admin must still read the roster: members=%d err=%v", len(admResp.GetMembers()), err)
	}
}

// TestGetUser_NoAuthzGate_SecureBehavior_Finding4 verifies a stranger cannot
// read another user's profile PII; self and admin still can.
func TestGetUser_NoAuthzGate_SecureBehavior_Finding4(t *testing.T) {
	srv, _, gs, _ := newAuthzTestServer(t)
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "bob", "bob@example.com", "Bob Secret"); err != nil {
		t.Fatalf("CreateUser(bob): %v", err)
	}

	if _, err := srv.GetUser(ctx, &pb.GetUserRequest{
		Actor: "user:mallory", UserId: "bob",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("stranger mallory reading bob: want PermissionDenied, got %v", err)
	}

	self, err := srv.GetUser(ctx, &pb.GetUserRequest{Actor: "user:bob", UserId: "bob"})
	if err != nil || !self.GetFound() {
		t.Fatalf("bob must still read his own profile: found=%v err=%v", self.GetFound(), err)
	}

	adm, err := srv.GetUser(ctx, &pb.GetUserRequest{Actor: "system:test", UserId: "bob"})
	if err != nil || !adm.GetFound() {
		t.Fatalf("admin must still read bob's profile: found=%v err=%v", adm.GetFound(), err)
	}
}

// TestGetUserTenants_NoAuthzGate_SecureBehavior_Finding4 verifies a stranger
// cannot enumerate another user's tenant graph; self and admin can.
func TestGetUserTenants_NoAuthzGate_SecureBehavior_Finding4(t *testing.T) {
	srv, _, gs, tenantID := newAuthzTestServer(t)
	ctx := context.Background()
	addMember(t, gs, tenantID, "bob")

	if _, err := srv.GetUserTenants(ctx, &pb.GetUserTenantsRequest{
		Actor: "user:mallory", UserId: "bob",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("stranger mallory reading bob's tenants: want PermissionDenied, got %v", err)
	}

	if _, err := srv.GetUserTenants(ctx, &pb.GetUserTenantsRequest{
		Actor: "user:bob", UserId: "bob",
	}); err != nil {
		t.Fatalf("bob must still read his own tenant memberships: %v", err)
	}

	if _, err := srv.GetUserTenants(ctx, &pb.GetUserTenantsRequest{
		Actor: "system:test", UserId: "bob",
	}); err != nil {
		t.Fatalf("admin must still read bob's tenant memberships: %v", err)
	}
}

// TestListUsers_NoAuthzGate_SecureBehavior_Finding4 verifies ListUsers is
// admin/system only — a non-privileged caller cannot enumerate the registry.
func TestListUsers_NoAuthzGate_SecureBehavior_Finding4(t *testing.T) {
	srv, _, gs, _ := newAuthzTestServer(t)
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "bob", "bob@example.com", "Bob"); err != nil {
		t.Fatalf("CreateUser(bob): %v", err)
	}

	if _, err := srv.ListUsers(ctx, &pb.ListUsersRequest{Actor: "user:mallory", Limit: 10}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("non-admin ListUsers: want PermissionDenied, got %v", err)
	}
	if _, err := srv.ListUsers(ctx, &pb.ListUsersRequest{Actor: "system:test", Limit: 10}); err != nil {
		t.Fatalf("admin ListUsers must succeed: %v", err)
	}
}
