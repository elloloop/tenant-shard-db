// Tests for the ListSharedWithMe RPC (W2 — EPIC #407).
//
// The behaviours pinned here mirror the contract in
// docs/go-port/rpcs/ListSharedWithMe.md.
// Contract tests are at:
//   - tests/python/integration/test_grpc_contract.py:339-350 (smoke)
//
// We deliberately seed via the typed store / globalstore helpers
// rather than the gRPC ShareNode handler — ShareNode hasn't
// landed yet, and the RPC under test is read-only.

package api_test

import (
	"context"
	"testing"
	"time"

	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
)

// newCanonicalStoreForTest builds a CanonicalStore rooted at t.TempDir()
// with WAL on, and registers a teardown hook. This is the per-test-file
// helper because helpers_external_test.go intentionally only exposes a
// globalstore helper today — adding a second one would have to go in
// helpers_external_test.go which is out of scope for this PR.
func newCanonicalStoreForTest(t *testing.T) *store.CanonicalStore {
	t.Helper()
	cs, err := store.New(store.Options{
		RootDir: t.TempDir(),
		WALMode: true,
	})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// seedSharedNode is a small builder: it creates a node owned by
// `owner` in `tenantID`, then inserts a non-deny `node_access` grant
// for `grantee`. This is the fully-loaded same-tenant share path —
// what `ShareNode(tenant=t, recipient=u, node=n)` would produce when
// the recipient lives in the same tenant as the node.
func seedSharedNode(t *testing.T, cs *store.CanonicalStore, tenantID, nodeID, owner, grantee, perm string) {
	t.Helper()
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant(%q): %v", tenantID, err)
	}
	if _, err := cs.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID:     nodeID,
		TypeID:     1,
		Payload:    map[string]any{"1": "doc-" + nodeID},
		OwnerActor: owner,
	}); err != nil {
		t.Fatalf("CreateNodeRaw(%q,%q): %v", tenantID, nodeID, err)
	}
	if err := cs.ShareNode(ctx, tenantID, store.ShareNodeInput{
		NodeID:     nodeID,
		ActorID:    grantee,
		ActorType:  "user",
		Permission: perm,
		GrantedBy:  owner,
	}); err != nil {
		t.Fatalf("ShareNode(%q,%q,%q): %v", tenantID, nodeID, grantee, err)
	}
}

// seedSharedNodeExpired creates the same shape as seedSharedNode but
// with an `expires_at` already in the past. Used to pin the "expired
// grants are filtered" assertion.
func seedSharedNodeExpired(t *testing.T, cs *store.CanonicalStore, tenantID, nodeID, owner, grantee, perm string, expiresAt int64) {
	t.Helper()
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant(%q): %v", tenantID, err)
	}
	if _, err := cs.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID:     nodeID,
		TypeID:     1,
		Payload:    map[string]any{"1": "expired-" + nodeID},
		OwnerActor: owner,
	}); err != nil {
		t.Fatalf("CreateNodeRaw(%q,%q): %v", tenantID, nodeID, err)
	}
	if err := cs.ShareNode(ctx, tenantID, store.ShareNodeInput{
		NodeID:     nodeID,
		ActorID:    grantee,
		ActorType:  "user",
		Permission: perm,
		GrantedBy:  owner,
		ExpiresAt:  expiresAt,
	}); err != nil {
		t.Fatalf("ShareNode(expired %q,%q,%q): %v", tenantID, nodeID, grantee, err)
	}
}

// makeReq is a tiny request builder so each test reads as the wire
// payload it exercises.
func makeReq(tenantID, actor string, limit, offset int32) *pb.ListSharedWithMeRequest {
	return &pb.ListSharedWithMeRequest{
		Context: &pb.RequestContext{
			TenantId: tenantID,
			Actor:    actor,
		},
		Limit:  limit,
		Offset: offset,
	}
}

// TestListSharedWithMe_Empty: a caller with zero shares gets a
// well-formed empty response — `nodes=[]`, `has_more=false`. Pinned
// by spec error-contract row "Caller has no shares".
func TestListSharedWithMe_Empty(t *testing.T) {
	t.Parallel()
	cs := newCanonicalStoreForTest(t)
	if err := cs.OpenTenant(context.Background(), "acme"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	srv := api.New(api.WithStore(cs))
	resp, err := srv.ListSharedWithMe(context.Background(), makeReq("acme", "user:alice", 0, 0))
	if err != nil {
		t.Fatalf("ListSharedWithMe: %v", err)
	}
	if resp == nil {
		t.Fatalf("nil response")
	}
	if len(resp.GetNodes()) != 0 {
		t.Fatalf("nodes: got %d, want 0", len(resp.GetNodes()))
	}
	if resp.GetHasMore() {
		t.Fatalf("has_more: got true, want false")
	}
}

// TestListSharedWithMe_SingleSameTenant: a caller with exactly one
// non-expired, non-deny grant in the same tenant sees that one node.
func TestListSharedWithMe_SingleSameTenant(t *testing.T) {
	t.Parallel()
	cs := newCanonicalStoreForTest(t)
	seedSharedNode(t, cs, "acme", "n1", "user:owner", "user:alice", "read")

	srv := api.New(api.WithStore(cs))
	resp, err := srv.ListSharedWithMe(context.Background(), makeReq("acme", "user:alice", 0, 0))
	if err != nil {
		t.Fatalf("ListSharedWithMe: %v", err)
	}
	if got, want := len(resp.GetNodes()), 1; got != want {
		t.Fatalf("nodes: got %d, want %d (resp=%+v)", got, want, resp)
	}
	if got, want := resp.GetNodes()[0].GetNodeId(), "n1"; got != want {
		t.Fatalf("node_id: got %q, want %q", got, want)
	}
	if got, want := resp.GetNodes()[0].GetTenantId(), "acme"; got != want {
		t.Fatalf("tenant_id: got %q, want %q", got, want)
	}
}

// TestListSharedWithMe_MultipleSameTenant: several non-deny grants
// for the caller in one tenant all surface. We don't pin order (the
// cross-tenant merge can affect it anyway), only set membership.
func TestListSharedWithMe_MultipleSameTenant(t *testing.T) {
	t.Parallel()
	cs := newCanonicalStoreForTest(t)
	for _, id := range []string{"n1", "n2", "n3"} {
		seedSharedNode(t, cs, "acme", id, "user:owner", "user:alice", "read")
	}
	// Noise: a node shared with someone else MUST NOT leak.
	seedSharedNode(t, cs, "acme", "n-other", "user:owner", "user:bob", "read")

	srv := api.New(api.WithStore(cs))
	resp, err := srv.ListSharedWithMe(context.Background(), makeReq("acme", "user:alice", 0, 0))
	if err != nil {
		t.Fatalf("ListSharedWithMe: %v", err)
	}
	if got, want := len(resp.GetNodes()), 3; got != want {
		t.Fatalf("nodes: got %d, want %d", got, want)
	}
	got := map[string]bool{}
	for _, n := range resp.GetNodes() {
		got[n.GetNodeId()] = true
	}
	for _, want := range []string{"n1", "n2", "n3"} {
		if !got[want] {
			t.Fatalf("missing node %q from response: %v", want, got)
		}
	}
	if got["n-other"] {
		t.Fatalf("response leaked another grantee's node: n-other present")
	}
}

// TestListSharedWithMe_MultipleTenantsCrossTenant: shares from
// multiple foreign tenants surface via the global shared_index, are
// resolved through cross-tenant store.GetNode, and union with same-
// tenant grants.
func TestListSharedWithMe_MultipleTenantsCrossTenant(t *testing.T) {
	t.Parallel()
	cs := newCanonicalStoreForTest(t)
	gs := newGlobalStore(t)

	// Register tenants in globalstore so the per-handler tenant gate
	// passes; the gate dereferences `gs.GetTenant(tenant_id)` and
	// raises NOT_FOUND otherwise.
	for _, tn := range []string{"alice-tenant", "acme", "globex"} {
		if _, err := gs.CreateTenant(context.Background(), tn, tn, ""); err != nil {
			t.Fatalf("CreateTenant(%q): %v", tn, err)
		}
	}

	// Same-tenant grant — caller's home tenant is "alice-tenant".
	seedSharedNode(t, cs, "alice-tenant", "home-1", "user:owner", "user:alice", "read")

	// Foreign tenant 1 — "acme" — owns a node shared cross-tenant
	// with alice. seedSharedNode opens the tenant and inserts both the
	// node and the per-tenant node_access row; for the cross-tenant
	// path we additionally write the shared_index hint.
	seedSharedNode(t, cs, "acme", "remote-acme-1", "user:bob", "user:alice", "read")
	if err := gs.AddShared(context.Background(), "user:alice", "acme", "remote-acme-1", "read"); err != nil {
		t.Fatalf("AddShared(acme): %v", err)
	}

	// Foreign tenant 2 — "globex".
	seedSharedNode(t, cs, "globex", "remote-globex-1", "user:carol", "user:alice", "read")
	if err := gs.AddShared(context.Background(), "user:alice", "globex", "remote-globex-1", "read"); err != nil {
		t.Fatalf("AddShared(globex): %v", err)
	}

	// Bob is shared something else but is NOT alice; alice MUST NOT
	// see bob's shares.
	seedSharedNode(t, cs, "acme", "bob-only", "user:bob", "user:bob", "read")
	if err := gs.AddShared(context.Background(), "user:bob", "acme", "bob-only", "read"); err != nil {
		t.Fatalf("AddShared(bob): %v", err)
	}

	srv := api.New(
		api.WithStore(cs),
		api.WithGlobalStore(gs),
	)
	// alice's home tenant is "alice-tenant"; cross-tenant rows surface
	// regardless of which tenant the caller is currently bound to.
	resp, err := srv.ListSharedWithMe(context.Background(), makeReq("alice-tenant", "user:alice", 0, 0))
	if err != nil {
		t.Fatalf("ListSharedWithMe: %v", err)
	}

	// Build a (tenant, node) set for assertion clarity.
	type key struct{ t, n string }
	got := map[key]bool{}
	for _, n := range resp.GetNodes() {
		got[key{n.GetTenantId(), n.GetNodeId()}] = true
	}
	want := []key{
		{"alice-tenant", "home-1"},
		{"acme", "remote-acme-1"},
		{"globex", "remote-globex-1"},
	}
	for _, w := range want {
		if !got[w] {
			t.Fatalf("missing (tenant=%s, node=%s) from response. got=%v", w.t, w.n, got)
		}
	}
	if got[key{"acme", "bob-only"}] {
		t.Fatalf("response leaked bob's share into alice's results")
	}
	if n := len(resp.GetNodes()); n != 3 {
		t.Fatalf("nodes: got %d, want 3 (resp=%+v)", n, resp)
	}
}

// TestListSharedWithMe_ExpiredGrantsFiltered: a same-tenant grant
// whose expires_at is in the past must NOT surface. The per-tenant
// query filters `expires_at IS NULL OR expires_at > now()`.
//
// Note: cross-tenant `shared_index` has NO expires_at column today
// (spec "Open questions / risks" #3), so this case only covers the
// per-tenant path.
func TestListSharedWithMe_ExpiredGrantsFiltered(t *testing.T) {
	t.Parallel()
	cs := newCanonicalStoreForTest(t)

	// Active grant — must surface.
	seedSharedNode(t, cs, "acme", "active", "user:owner", "user:alice", "read")
	// Expired grant — was valid until 5s before unix epoch, definitely
	// in the past.
	pastMS := int64(time.Now().Add(-1 * time.Hour).UnixMilli())
	seedSharedNodeExpired(t, cs, "acme", "expired", "user:owner", "user:alice", "read", pastMS)

	srv := api.New(api.WithStore(cs))
	resp, err := srv.ListSharedWithMe(context.Background(), makeReq("acme", "user:alice", 0, 0))
	if err != nil {
		t.Fatalf("ListSharedWithMe: %v", err)
	}
	if got, want := len(resp.GetNodes()), 1; got != want {
		t.Fatalf("nodes: got %d, want %d", got, want)
	}
	if got, want := resp.GetNodes()[0].GetNodeId(), "active"; got != want {
		t.Fatalf("node_id: got %q, want %q (expired grant should be filtered)", got, want)
	}
}

// TestListSharedWithMe_Pagination: with limit=2 and offset=0 we get
// 2 rows out of 3 same-tenant grants and `has_more=true`; with
// limit=2 and offset=2 we get the third row and `has_more=false`.
// This test pins the steady-state pagination contract for a single
// index — the cross-tenant overlap caveat is documented in spec
// "Pagination semantics" but is hard to assert on without a stable
// merge order, so we keep this case scoped to one index.
func TestListSharedWithMe_Pagination(t *testing.T) {
	t.Parallel()
	cs := newCanonicalStoreForTest(t)
	for _, id := range []string{"n1", "n2", "n3"} {
		seedSharedNode(t, cs, "acme", id, "user:owner", "user:alice", "read")
	}

	srv := api.New(api.WithStore(cs))

	// Page 1: limit=2, offset=0. Expect 2 nodes; has_more=true since
	// len(nodes) >= limit.
	page1, err := srv.ListSharedWithMe(context.Background(), makeReq("acme", "user:alice", 2, 0))
	if err != nil {
		t.Fatalf("ListSharedWithMe page1: %v", err)
	}
	if got, want := len(page1.GetNodes()), 2; got != want {
		t.Fatalf("page1 nodes: got %d, want %d", got, want)
	}
	if !page1.GetHasMore() {
		t.Fatalf("page1 has_more: got false, want true (len(nodes)>=limit)")
	}

	// Page 2: limit=2, offset=2. Expect the remaining 1 node;
	// has_more=false.
	page2, err := srv.ListSharedWithMe(context.Background(), makeReq("acme", "user:alice", 2, 2))
	if err != nil {
		t.Fatalf("ListSharedWithMe page2: %v", err)
	}
	if got, want := len(page2.GetNodes()), 1; got != want {
		t.Fatalf("page2 nodes: got %d, want %d", got, want)
	}
	if page2.GetHasMore() {
		t.Fatalf("page2 has_more: got true, want false")
	}

	// Across both pages, every seeded node appeared exactly once.
	seen := map[string]int{}
	for _, n := range page1.GetNodes() {
		seen[n.GetNodeId()]++
	}
	for _, n := range page2.GetNodes() {
		seen[n.GetNodeId()]++
	}
	for _, id := range []string{"n1", "n2", "n3"} {
		if seen[id] != 1 {
			t.Fatalf("node %q appeared %d times across both pages, want 1", id, seen[id])
		}
	}
}
