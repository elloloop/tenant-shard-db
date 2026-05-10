// SPDX-License-Identifier: AGPL-3.0-only

// DelegateAccess RPC tests — Wave 2 of EPIC #407.
//
// These three tests pin the contract called out in
// docs/go-port/rpcs/DelegateAccess.md "Implementation outline":
//
//  1. Admin delegates -> grant materialises after the applier picks up
//     the WAL event. THIS is the test that proves the Wave-0
//     silent-drop bug is fixed: the Python applier had no
//     admin_delegate_access dispatch branch, so the WAL event was
//     appended-and-ignored. With the Go applier (W1.10) + this handler
//     (W2) wired through an in-memory WAL + canonical store, the grant
//     becomes a row in node_access on replay -- which is the contract
//     Python silently breaks today.
//
//  2. Non-admin caller -> PERMISSION_DENIED. Closes the wire-payload
//     trust gap (privilege-escalation regression pinned by commit
//     fece3fb).
//
//  3. Expired delegation -> filtered at read time by the visibility
//     joins. Per spec "Open questions" §3 expiration is read-side only
//     today; the materialised row stays put.

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

const (
	delegateTopic   = "entdb-wal-test-delegate"
	delegateGroupID = "applier-delegate-test"
)

// delegateFixture wires the full mutation path -- WAL producer + WAL
// consumer (same in-memory backend) + per-tenant store + globalstore +
// applier -- into one struct the tests can reach for. Cleanup runs via
// t.Cleanup so individual tests don't have to remember to Stop / Close.
type delegateFixture struct {
	t       *testing.T
	wal     *wal.InMemory
	store   *store.CanonicalStore
	global  *globalstore.GlobalStore
	srv     *api.Server
	applier *apply.Applier
	cancel  context.CancelFunc
	done    chan error
}

func newDelegateFixture(t *testing.T) *delegateFixture {
	t.Helper()
	w := wal.NewInMemory(1)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}
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

	a, err := apply.New(apply.Options{
		Store:       cs,
		Global:      gs,
		Consumer:    w,
		Topic:       delegateTopic,
		GroupID:     delegateGroupID,
		PollTimeout: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}

	srv := api.New(
		api.WithStore(cs),
		api.WithGlobalStore(gs),
		api.WithWALProducer(w),
		api.WithWALTopic(delegateTopic),
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	return &delegateFixture{
		t: t, wal: w, store: cs, global: gs,
		srv: srv, applier: a, cancel: cancel, done: done,
	}
}

// seedTenant registers tenantID in the global store so checkTenant
// passes. Region is left blank so region pinning never fires under
// the default WithRegion("") wiring.
func (f *delegateFixture) seedTenant(t *testing.T, tenantID string) {
	t.Helper()
	if _, err := f.global.CreateTenant(context.Background(), tenantID, tenantID, ""); err != nil {
		t.Fatalf("CreateTenant(%q): %v", tenantID, err)
	}
	if err := f.store.OpenTenant(context.Background(), tenantID); err != nil {
		t.Fatalf("OpenTenant(%q): %v", tenantID, err)
	}
}

// addMember inserts (tenantID, userID, role) into tenant_members.
func (f *delegateFixture) addMember(t *testing.T, tenantID, userID, role string) {
	t.Helper()
	if err := f.global.AddTenantMember(context.Background(), tenantID, userID, role); err != nil {
		t.Fatalf("AddTenantMember(%q,%q,%q): %v", tenantID, userID, role, err)
	}
}

// appendCreateNode appends a synthetic create_node event that gives
// `owner` an owned node, then waits for the applier to apply it.
// This is how we seed the "from_user owns N nodes" precondition for
// DelegateAccess; the handler's pre-apply count drives the bulk grant.
func (f *delegateFixture) appendCreateNode(t *testing.T, tenantID, nodeID, owner, idemKey string) {
	t.Helper()
	ev := apply.Event{
		TenantID:       tenantID,
		Actor:          owner,
		IdempotencyKey: idemKey,
		Ops: []map[string]any{{
			"op":      string(apply.OpCreateNode),
			"id":      nodeID,
			"type_id": int32(1),
			"data":    map[string]any{"1": "secret"},
		}},
	}
	value, err := ev.Encode()
	if err != nil {
		t.Fatalf("event.Encode: %v", err)
	}
	headers := map[string][]byte{wal.HeaderIdempotencyKey: []byte(idemKey)}
	if _, err := f.wal.Append(context.Background(), delegateTopic, tenantID, value, headers); err != nil {
		t.Fatalf("wal.Append: %v", err)
	}
	f.waitForIdempKey(t, tenantID, idemKey)
}

// waitForIdempKey polls until idemKey appears in applied_events for
// tenantID, or the deadline fires. Mirrors the apply package's helper.
func (f *delegateFixture) waitForIdempKey(t *testing.T, tenantID, idemKey string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ok, err := f.store.CheckIdempotency(context.Background(), tenantID, idemKey)
		if err != nil {
			t.Fatalf("CheckIdempotency: %v", err)
		}
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("waitForIdempKey: %q never showed up", idemKey)
}

// waitForGrant polls the per-tenant SQLite for a non-empty node_access
// row matching (nodeID, actorID). Returns the row's permission and
// expires_at. Used to assert the applier materialised the WAL event --
// which is exactly the "silent-drop bug closed" assertion.
func (f *delegateFixture) waitForGrant(t *testing.T, tenantID, nodeID, actorID string) (perm string, expiresAt int64) {
	t.Helper()
	db, err := f.store.AdminDB(tenantID)
	if err != nil {
		t.Fatalf("AdminDB: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var p string
		var exp interface{}
		err := db.QueryRowContext(context.Background(),
			`SELECT permission, expires_at FROM node_access
			   WHERE node_id = ? AND actor_id = ?`,
			nodeID, actorID,
		).Scan(&p, &exp)
		if err == nil {
			if v, ok := exp.(int64); ok {
				return p, v
			}
			return p, 0
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("waitForGrant: no node_access row for (%s, %s) within 2s", nodeID, actorID)
	return "", 0
}

// TestDelegateAccess_AdminDelegatesGrantMaterialisesAfterApply is THE
// regression test that proves the Wave-0 silent-drop bug is closed.
//
// Setup: alice (owner) owns two nodes in acme. carol (admin) calls
// DelegateAccess(from=alice, to=bob, permission=read). The handler
// emits two delegate_access ops on the WAL; the applier (W1.10
// dispatch branch) materialises both into node_access rows.
//
// Pinned by docs/go-port/rpcs/DelegateAccess.md §"Open questions" item 1:
// the Python applier silently drops admin_delegate_access events; the
// Go applier MUST close that gap.
func TestDelegateAccess_AdminDelegatesGrantMaterialisesAfterApply(t *testing.T) {
	t.Parallel()
	f := newDelegateFixture(t)

	const tenantID = "acme"
	f.seedTenant(t, tenantID)
	f.addMember(t, tenantID, "alice", "owner")
	f.addMember(t, tenantID, "carol", "admin")

	// Seed two alice-owned nodes through the WAL so the applier
	// materialises them first. The DelegateAccess handler's owner-count
	// SELECT will pick them up.
	f.appendCreateNode(t, tenantID, "doc1", "user:alice", "seed-doc1")
	f.appendCreateNode(t, tenantID, "doc2", "user:alice", "seed-doc2")

	// Trusted ctx: carol (tenant admin).
	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodOAuth,
		Subject: "user:carol",
	})

	resp, err := f.srv.DelegateAccess(ctx, &pb.DelegateAccessRequest{
		Actor:      "user:carol",
		TenantId:   tenantID,
		FromUser:   "user:alice",
		ToUser:     "user:bob",
		Permission: "read",
	})
	if err != nil {
		t.Fatalf("DelegateAccess: unexpected error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("DelegateAccess: success=false, want true (resp=%+v)", resp)
	}
	if resp.GetDelegated() != 2 {
		t.Fatalf("DelegateAccess: delegated=%d, want 2 (the alice-owned-node count)", resp.GetDelegated())
	}
	if resp.GetExpiresAt() != 0 {
		t.Fatalf("DelegateAccess: expires_at=%d, want 0 (permanent grant)", resp.GetExpiresAt())
	}

	// THIS is the bug-closed assertion: the applier must materialise
	// the grant for both nodes. If the applier silently dropped the
	// event (the Python bug), waitForGrant times out.
	for _, nodeID := range []string{"doc1", "doc2"} {
		perm, exp := f.waitForGrant(t, tenantID, nodeID, "user:bob")
		if perm != "read" {
			t.Fatalf("node_access[%s].permission=%q, want %q", nodeID, perm, "read")
		}
		if exp != 0 {
			t.Fatalf("node_access[%s].expires_at=%d, want 0 (permanent)", nodeID, exp)
		}
	}
}

// TestDelegateAccess_NonAdminPermissionDenied: a regular tenant member
// trying to bulk-delegate is rejected with PERMISSION_DENIED. This pins
// the trusted-actor contract: even if the wire actor claims
// "user:eve", the role lookup fires against the trusted Identity --
// closing the wire-payload trust gap from commit fece3fb.
func TestDelegateAccess_NonAdminPermissionDenied(t *testing.T) {
	t.Parallel()
	f := newDelegateFixture(t)

	const tenantID = "acme"
	f.seedTenant(t, tenantID)
	f.addMember(t, tenantID, "alice", "owner")
	f.addMember(t, tenantID, "eve", "member") // not admin/owner

	// Seed a node so we know the denial fires from the role gate, not
	// from a "no nodes to delegate" short-circuit (the handler doesn't
	// short-circuit, but the test stays robust against future tuning).
	f.appendCreateNode(t, tenantID, "doc1", "user:alice", "seed-doc1")

	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodOAuth,
		Subject: "user:eve",
	})

	resp, err := f.srv.DelegateAccess(ctx, &pb.DelegateAccessRequest{
		Actor:      "user:eve",
		TenantId:   tenantID,
		FromUser:   "user:alice",
		ToUser:     "user:bob",
		Permission: "read",
	})
	if err == nil {
		t.Fatalf("DelegateAccess: want PERMISSION_DENIED, got resp=%+v", resp)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("DelegateAccess: not a grpc status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("DelegateAccess: code=%v, want PermissionDenied", st.Code())
	}

	// Defence in depth: the WAL must not have grown a delegate_access
	// op for bob -- and therefore node_access must have no row. We
	// give the (non-existent) applier work a generous beat to be sure.
	time.Sleep(150 * time.Millisecond)
	db, err := f.store.AdminDB(tenantID)
	if err != nil {
		t.Fatalf("AdminDB: %v", err)
	}
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM node_access WHERE actor_id = ?`,
		"user:bob",
	).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("node_access for user:bob: got %d rows, want 0 (PERMISSION_DENIED must NOT WAL-append)", n)
	}
}

// TestDelegateAccess_ExpiredDelegationFilteredAtRead: an admin
// delegates with expires_at in the past. The applier still materialises
// the row (storage is "tombstoned" lazily; spec §"Open questions" §3),
// but the read-side visibility join must filter it out so a
// VisibleNodeIDs(bob) query against the same tenant returns no
// expired-grant rows.
//
// This pins the read-side filter at store/visibility.go:163
// (`expires_at IS NULL OR expires_at > ?`).
func TestDelegateAccess_ExpiredDelegationFilteredAtRead(t *testing.T) {
	t.Parallel()
	f := newDelegateFixture(t)

	const tenantID = "acme"
	f.seedTenant(t, tenantID)
	f.addMember(t, tenantID, "alice", "owner")
	f.addMember(t, tenantID, "carol", "admin")
	f.appendCreateNode(t, tenantID, "doc1", "user:alice", "seed-doc1")

	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodOAuth,
		Subject: "user:carol",
	})

	// A timestamp comfortably in the past.
	expiredMs := time.Now().Add(-1 * time.Hour).UnixMilli()
	resp, err := f.srv.DelegateAccess(ctx, &pb.DelegateAccessRequest{
		Actor:      "user:carol",
		TenantId:   tenantID,
		FromUser:   "user:alice",
		ToUser:     "user:bob",
		Permission: "read",
		ExpiresAt:  expiredMs,
	})
	if err != nil {
		t.Fatalf("DelegateAccess: unexpected error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("DelegateAccess: success=false (resp=%+v)", resp)
	}
	if resp.GetExpiresAt() != expiredMs {
		t.Fatalf("DelegateAccess: expires_at echoed %d, want %d", resp.GetExpiresAt(), expiredMs)
	}

	// The applier still writes the row -- expiration is read-side.
	perm, exp := f.waitForGrant(t, tenantID, "doc1", "user:bob")
	if perm != "read" {
		t.Fatalf("perm=%q, want read", perm)
	}
	if exp != expiredMs {
		t.Fatalf("expires_at=%d, want %d (expiration is materialised, then filtered at read)", exp, expiredMs)
	}

	// Read-side filter: ListSharedWithMe for bob must return ZERO nodes
	// because the only grant is expired (the SQL applies
	// `expires_at IS NULL OR expires_at > now`, store/visibility.go:163).
	// This is the same code path read handlers exercise.
	shared, err := f.store.ListSharedWithMe(context.Background(),
		tenantID, []string{"user:bob"}, 100, 0)
	if err != nil {
		t.Fatalf("ListSharedWithMe: %v", err)
	}
	for _, n := range shared {
		if n.NodeID == "doc1" {
			t.Fatalf("ListSharedWithMe: doc1 surfaced to user:bob despite expired delegation (read-side filter regression)")
		}
	}
}
