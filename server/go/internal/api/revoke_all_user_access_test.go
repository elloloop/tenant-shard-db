// SPDX-License-Identifier: AGPL-3.0-only

// Tests for RevokeAllUserAccess. Pin the WAL-first
// restoration: the handler MUST emit an admin_revoke_access event so
// the applier can drain node_access + group_users + node_visibility on
// replay. See docs/go-port/rpcs/RevokeAllUserAccess.md "WAL invariant
// gap (Go port MUST fix)".

package api_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// revokeAllFixture wires the api.Server + WAL + applier + stores
// together. The applier runs in a background goroutine so the WAL
// event the handler appends is materialized into per-tenant SQLite
// before the test asserts on the final state.
type revokeAllFixture struct {
	t       *testing.T
	srv     *api.Server
	store   *store.CanonicalStore
	global  *globalstore.GlobalStore
	walImpl *wal.InMemory
	applier *apply.Applier
}

func newRevokeAllFixture(t *testing.T) *revokeAllFixture {
	t.Helper()
	dir := t.TempDir()

	cs, err := store.New(store.Options{RootDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	gs, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })

	w := wal.NewInMemory(1)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}

	a, err := apply.New(apply.Options{
		Store:       cs,
		Global:      gs,
		Consumer:    w,
		Topic:       "entdb-wal", // matches the handler's hard-coded topic.
		GroupID:     "applier-test",
		PollTimeout: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	srv := api.New(
		api.WithStore(cs),
		api.WithGlobalStore(gs),
		api.WithWALProducer(w),
	)

	return &revokeAllFixture{
		t:       t,
		srv:     srv,
		store:   cs,
		global:  gs,
		walImpl: w,
		applier: a,
	}
}

// openTenant opens the per-tenant SQLite so the seed helpers below
// can write directly.
func (f *revokeAllFixture) openTenant(tenantID string) {
	f.t.Helper()
	if err := f.store.OpenTenant(context.Background(), tenantID); err != nil {
		f.t.Fatalf("OpenTenant(%q): %v", tenantID, err)
	}
}

// seedGrant inserts a node_access row for (node_id, actor_id) with
// "read" permission. The test uses ShareNode which is the public
// idempotent upsert; node existence is not enforced at the row level.
func (f *revokeAllFixture) seedGrant(tenantID, nodeID, actorID string) {
	f.t.Helper()
	err := f.store.ShareNode(context.Background(), tenantID, store.ShareNodeInput{
		NodeID:     nodeID,
		ActorID:    actorID,
		Permission: "read",
		GrantedBy:  "user:owner",
	})
	if err != nil {
		f.t.Fatalf("ShareNode: %v", err)
	}
}

// seedGroupMember inserts a group_users row.
func (f *revokeAllFixture) seedGroupMember(tenantID, groupID, memberActorID string) {
	f.t.Helper()
	if err := f.store.AddGroupMember(context.Background(), tenantID, groupID, memberActorID, "member"); err != nil {
		f.t.Fatalf("AddGroupMember: %v", err)
	}
}

// countAccess returns the (node_access, group_users) row counts for
// userID inside tenantID. Used to assert pre/post state.
func (f *revokeAllFixture) countAccess(tenantID, userID string) (int, int) {
	f.t.Helper()
	db, err := f.store.AdminDB(tenantID)
	if err != nil {
		f.t.Fatalf("AdminDB: %v", err)
	}
	var grants, groups int
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM node_access WHERE actor_id = ?`, userID,
	).Scan(&grants); err != nil {
		f.t.Fatalf("scan node_access: %v", err)
	}
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM group_users WHERE member_actor_id = ?`, userID,
	).Scan(&groups); err != nil {
		f.t.Fatalf("scan group_users: %v", err)
	}
	return grants, groups
}

// waitDrained polls until both node_access and group_users counts for
// userID drop to zero, or the deadline fires. The applier is async so
// the handler returns before the deletes have landed.
func (f *revokeAllFixture) waitDrained(tenantID, userID string) {
	f.t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		g, gr := f.countAccess(tenantID, userID)
		if g == 0 && gr == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	g, gr := f.countAccess(tenantID, userID)
	f.t.Fatalf("waitDrained: applier never drained user=%q (grants=%d groups=%d)", userID, g, gr)
}

// seedTenantMembership seeds a tenant_members row for callers whose
// authority comes from member-role, not actor prefix.
func (f *revokeAllFixture) seedTenantMembership(tenantID, userID, role string) {
	f.t.Helper()
	if _, err := f.global.CreateTenant(context.Background(), tenantID, tenantID, "us-east-1"); err != nil {
		// CreateTenant may fail if already exists in a prior helper —
		// ignore the error and continue.
		_ = err
	}
	if err := f.global.AddTenantMember(context.Background(), tenantID, userID, role); err != nil {
		f.t.Fatalf("AddTenantMember(%q,%q,%q): %v", tenantID, userID, role, err)
	}
}

// TestRevokeAllUserAccess_AdminHappyPath: an admin: trusted actor
// revokes a user with grants and group memberships. The WAL event
// is appended, the applier drains node_access + group_users, and the
// response tallies match.
func TestRevokeAllUserAccess_AdminHappyPath(t *testing.T) {
	t.Parallel()
	f := newRevokeAllFixture(t)
	const tenantID = "acme"
	const target = "user:bob"
	f.openTenant(tenantID)
	if _, err := f.global.CreateTenant(context.Background(), tenantID, tenantID, "us-east-1"); err != nil {
		t.Fatalf("CreateTenant(%q): %v", tenantID, err)
	}

	// Seed two grants and two group memberships for bob.
	f.seedGrant(tenantID, "doc1", target)
	f.seedGrant(tenantID, "doc2", target)
	f.seedGroupMember(tenantID, "g-eng", target)
	f.seedGroupMember(tenantID, "g-design", target)

	// Sanity: pre-revoke counts are non-zero.
	if g, gr := f.countAccess(tenantID, target); g != 2 || gr != 2 {
		t.Fatalf("pre-revoke counts: grants=%d groups=%d, want 2,2", g, gr)
	}

	// Cross-tenant shared_index: 2 rows in acme + 1 in another tenant.
	// The handler must clean only the acme rows.
	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodOAuth,
		Subject: "admin:root",
	})
	if err := f.global.AddShared(ctx, target, tenantID, "doc1", "read"); err != nil {
		t.Fatalf("AddShared acme/doc1: %v", err)
	}
	if err := f.global.AddShared(ctx, target, tenantID, "doc2", "read"); err != nil {
		t.Fatalf("AddShared acme/doc2: %v", err)
	}
	if err := f.global.AddShared(ctx, target, "other-tenant", "doc3", "read"); err != nil {
		t.Fatalf("AddShared other-tenant/doc3: %v", err)
	}

	resp, err := f.srv.RevokeAllUserAccess(ctx, &pb.RevokeAllUserAccessRequest{
		Actor:    "admin:root",
		TenantId: tenantID,
		UserId:   target,
	})
	if err != nil {
		t.Fatalf("RevokeAllUserAccess: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("RevokeAllUserAccess: success=false, err=%q", resp.GetError())
	}
	if got, want := resp.GetRevokedGrants(), int32(2); got != want {
		t.Errorf("RevokedGrants=%d, want %d", got, want)
	}
	if got, want := resp.GetRevokedGroups(), int32(2); got != want {
		t.Errorf("RevokedGroups=%d, want %d", got, want)
	}
	if got, want := resp.GetRevokedShared(), int32(2); got != want {
		t.Errorf("RevokedShared=%d, want %d (other-tenant row must survive)", got, want)
	}

	// Wait for the applier to process the WAL event, then assert the
	// per-tenant SQLite is fully drained for bob.
	f.waitDrained(tenantID, target)

	// Other-tenant shared_index row must survive.
	rows, err := f.global.ListSharedToUser(ctx, target, 100, 0)
	if err != nil {
		t.Fatalf("ListSharedToUser: %v", err)
	}
	if len(rows) != 1 || rows[0].SourceTenant != "other-tenant" {
		t.Fatalf("ListSharedToUser: got %+v, want exactly 1 row in other-tenant", rows)
	}
}

func TestRevokeAllUserAccess_WALEventCarriesSharedIndexCleanup(t *testing.T) {
	t.Parallel()
	f := newRevokeAllFixture(t)
	const tenantID = "acme"
	const target = "user:bob"
	f.openTenant(tenantID)
	if _, err := f.global.CreateTenant(context.Background(), tenantID, tenantID, "us-east-1"); err != nil {
		t.Fatalf("CreateTenant(%q): %v", tenantID, err)
	}
	if err := f.global.AddShared(context.Background(), target, tenantID, "doc1", "read"); err != nil {
		t.Fatalf("AddShared: %v", err)
	}

	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodOAuth,
		Subject: "admin:root",
	})
	if _, err := f.srv.RevokeAllUserAccess(ctx, &pb.RevokeAllUserAccessRequest{
		Actor:    "admin:root",
		TenantId: tenantID,
		UserId:   target,
	}); err != nil {
		t.Fatalf("RevokeAllUserAccess: %v", err)
	}

	if got := f.walImpl.GetRecordCount("entdb-wal"); got != 1 {
		t.Fatalf("WAL records = %d, want 1 tenant event with paired ops", got)
	}
	records, err := f.walImpl.PollBatch(context.Background(), "entdb-wal", "revoke-shape-drain", 10, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("PollBatch records = %d, want 1", len(records))
	}
	ev, err := wal.DecodeEvent(records[0].Value)
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	if ev.Scope == wal.ScopeGlobal {
		t.Fatalf("event scope = global, want tenant-scoped paired event")
	}
	if ev.TenantID != tenantID {
		t.Fatalf("event tenant = %q, want %q", ev.TenantID, tenantID)
	}
	if len(ev.Ops) != 2 {
		t.Fatalf("ops = %d, want admin_revoke_access + access_revoked", len(ev.Ops))
	}
	if got, _ := ev.Ops[0]["op"].(string); got != string(apply.OpAdminRevokeAccess) {
		t.Fatalf("first op = %q, want %s", got, apply.OpAdminRevokeAccess)
	}
	if got, _ := ev.Ops[1]["op"].(string); got != string(apply.OpAccessRevoked) {
		t.Fatalf("second op = %q, want %s", got, apply.OpAccessRevoked)
	}
	if got, _ := ev.Ops[1]["tenant_id"].(string); got != tenantID {
		t.Fatalf("access_revoked tenant_id = %q, want %q", got, tenantID)
	}
	if got, _ := ev.Ops[1]["user_id"].(string); got != target {
		t.Fatalf("access_revoked user_id = %q, want %q", got, target)
	}
}

// TestRevokeAllUserAccess_NonAdminDenied: a regular member calling
// against another user is rejected with PERMISSION_DENIED. No WAL
// event is appended, no SQLite state changes.
func TestRevokeAllUserAccess_NonAdminDenied(t *testing.T) {
	t.Parallel()
	f := newRevokeAllFixture(t)
	const tenantID = "acme"
	const target = "user:bob"
	f.openTenant(tenantID)

	f.seedGrant(tenantID, "doc1", target)
	f.seedGroupMember(tenantID, "g-eng", target)

	// Mallory is a "member" — not owner/admin.
	f.seedTenantMembership(tenantID, "mallory", "member")
	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodOAuth,
		Subject: "user:mallory",
	})

	resp, err := f.srv.RevokeAllUserAccess(ctx, &pb.RevokeAllUserAccessRequest{
		Actor:    "user:mallory",
		TenantId: tenantID,
		UserId:   target,
	})
	if err == nil {
		t.Fatalf("RevokeAllUserAccess: expected PERMISSION_DENIED, got resp=%+v", resp)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("RevokeAllUserAccess: not a grpc status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("RevokeAllUserAccess: code=%v, want PermissionDenied", st.Code())
	}

	// State must be untouched: bob's grant + group still present.
	if g, gr := f.countAccess(tenantID, target); g != 1 || gr != 1 {
		t.Errorf("post-deny counts: grants=%d groups=%d, want 1,1", g, gr)
	}
	// And nothing landed in the WAL.
	if n := f.walImpl.GetRecordCount("entdb-wal"); n != 0 {
		t.Errorf("WAL records after deny: got %d, want 0", n)
	}
}

// TestRevokeAllUserAccess_NoGrantsIdempotent: revoking a user with
// zero grants/groups returns success=true with all-zero tallies. The
// WAL event is still appended (the applier dedupes); the response is
// the idempotent shape.
func TestRevokeAllUserAccess_NoGrantsIdempotent(t *testing.T) {
	t.Parallel()
	f := newRevokeAllFixture(t)
	const tenantID = "acme"
	const target = "user:ghost"
	f.openTenant(tenantID)
	if _, err := f.global.CreateTenant(context.Background(), tenantID, tenantID, "us-east-1"); err != nil {
		t.Fatalf("CreateTenant(%q): %v", tenantID, err)
	}

	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodOAuth,
		Subject: "system:admin",
	})

	resp, err := f.srv.RevokeAllUserAccess(ctx, &pb.RevokeAllUserAccessRequest{
		Actor:    "system:admin",
		TenantId: tenantID,
		UserId:   target,
	})
	if err != nil {
		t.Fatalf("RevokeAllUserAccess: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("RevokeAllUserAccess: success=false, err=%q", resp.GetError())
	}
	if resp.GetRevokedGrants() != 0 || resp.GetRevokedGroups() != 0 || resp.GetRevokedShared() != 0 {
		t.Fatalf("idempotent no-op: tallies=(%d,%d,%d), want all zero",
			resp.GetRevokedGrants(), resp.GetRevokedGroups(), resp.GetRevokedShared())
	}

	// Second call: same shape (still zero, still success). Confirms
	// the applier's per-event dedupe doesn't flip the response code.
	resp2, err := f.srv.RevokeAllUserAccess(ctx, &pb.RevokeAllUserAccessRequest{
		Actor:    "system:admin",
		TenantId: tenantID,
		UserId:   target,
	})
	if err != nil {
		t.Fatalf("RevokeAllUserAccess (retry): %v", err)
	}
	if !resp2.GetSuccess() {
		t.Fatalf("RevokeAllUserAccess (retry): success=false, err=%q", resp2.GetError())
	}
	if resp2.GetRevokedGrants() != 0 || resp2.GetRevokedGroups() != 0 {
		t.Fatalf("retry tallies non-zero: (%d,%d)",
			resp2.GetRevokedGrants(), resp2.GetRevokedGroups())
	}
}

// TestRevokeAllUserAccess_RequiredFields: empty tenant_id / user_id
// surface as INVALID_ARGUMENT before the auth gate. Pinned by
// test_grpc_contract.py:593-595.
func TestRevokeAllUserAccess_RequiredFields(t *testing.T) {
	t.Parallel()
	f := newRevokeAllFixture(t)
	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodOAuth,
		Subject: "admin:root",
	})

	cases := []struct {
		name string
		req  *pb.RevokeAllUserAccessRequest
	}{
		{"empty tenant_id", &pb.RevokeAllUserAccessRequest{Actor: "admin:root", UserId: "user:bob"}},
		{"empty user_id", &pb.RevokeAllUserAccessRequest{Actor: "admin:root", TenantId: "acme"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := f.srv.RevokeAllUserAccess(ctx, tc.req)
			if err == nil {
				t.Fatalf("expected INVALID_ARGUMENT, got nil")
			}
			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("not a grpc status: %v", err)
			}
			if st.Code() != codes.InvalidArgument {
				t.Fatalf("code=%v, want InvalidArgument", st.Code())
			}
		})
	}
}
