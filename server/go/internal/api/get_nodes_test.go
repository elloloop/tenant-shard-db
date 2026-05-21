// Tests for the GetNodes RPC (W2 — EPIC #407).
//
// Spec: docs/go-port/rpcs/GetNodes.md. Coverage targets the contract
// pins enumerated there:
//
//   - Empty node_ids -> nodes=[] missing_ids=[] OK (Python parity).
//   - Single id round-trip: existing id surfaces as a Node; unknown id
//     surfaces in missing_ids.
//   - Multi-id round-trip: multiple existing ids preserve request order
//     (gaps closed); duplicates are NOT deduped.
//   - Mixed missing + visible + denied: missing-or-denied are merged
//     into missing_ids; the cross-tenant denied case must NOT be
//     distinguishable from not-found (spec "Auth").
//   - Oversize batch (> maxBatchNodeIDs) -> RESOURCE_EXHAUSTED before
//     any I/O. Hardening delta vs Python (no Python equivalent).
//   - Unknown tenant -> NOT_FOUND from the tenant gate
//     (CheckTenant / globalstore.GetTenant).

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// newStoreWithTenant builds a fresh CanonicalStore rooted at t.TempDir()
// and pre-opens tenantID. Closed automatically on test teardown.
func newStoreWithTenant(t *testing.T, tenantID string) *store.CanonicalStore {
	t.Helper()
	cs, err := store.New(store.Options{
		RootDir: t.TempDir(),
		WALMode: true,
	})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	if err := cs.OpenTenant(context.Background(), tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	return cs
}

// seedNodeForGetNodes is a tiny helper to put one node into the store with a
// deterministic owner so the cross-tenant filter tests can vary by
// whether the actor matches owner_actor.
func seedNodeForGetNodes(t *testing.T, cs *store.CanonicalStore, tenantID, nodeID, owner string) {
	t.Helper()
	_, err := cs.CreateNodeRaw(context.Background(), tenantID, store.NodeInput{
		NodeID:     nodeID,
		TypeID:     1,
		Payload:    map[string]any{"1": nodeID + "-payload"},
		OwnerActor: owner,
	})
	if err != nil {
		t.Fatalf("seedNodeForGetNodes %s: %v", nodeID, err)
	}
}

// TestGetNodes_EmptyList pins the trivial "no work" branch:
// nodes=[] missing_ids=[] OK. No store I/O — but tenant gate +
// cross-tenant role check still run.
func TestGetNodes_EmptyList(t *testing.T) {
	t.Parallel()
	const tenantID = "acme"

	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(context.Background(), tenantID, "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := gs.AddTenantMember(context.Background(), tenantID, "alice", "owner"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}
	cs := newStoreWithTenant(t, tenantID)

	srv := api.New(api.WithGlobalStore(gs), api.WithStore(cs))
	resp, err := srv.GetNodes(context.Background(), &pb.GetNodesRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "user:alice"},
	})
	if err != nil {
		t.Fatalf("GetNodes: %v", err)
	}
	if len(resp.GetNodes()) != 0 {
		t.Fatalf("nodes: got %d, want 0", len(resp.GetNodes()))
	}
	if len(resp.GetMissingIds()) != 0 {
		t.Fatalf("missing_ids: got %d, want 0", len(resp.GetMissingIds()))
	}
}

// TestGetNodes_SingleHit pins the single-id round-trip path:
// existing -> nodes has one entry, missing_ids is empty.
func TestGetNodes_SingleHit(t *testing.T) {
	t.Parallel()
	const tenantID = "acme"

	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(context.Background(), tenantID, "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := gs.AddTenantMember(context.Background(), tenantID, "alice", "owner"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}
	cs := newStoreWithTenant(t, tenantID)
	seedNodeForGetNodes(t, cs, tenantID, "n1", "user:alice")

	srv := api.New(api.WithGlobalStore(gs), api.WithStore(cs))
	resp, err := srv.GetNodes(context.Background(), &pb.GetNodesRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "user:alice"},
		NodeIds: []string{"n1"},
	})
	if err != nil {
		t.Fatalf("GetNodes: %v", err)
	}
	if len(resp.GetNodes()) != 1 {
		t.Fatalf("nodes: got %d, want 1", len(resp.GetNodes()))
	}
	if got := resp.GetNodes()[0].GetNodeId(); got != "n1" {
		t.Fatalf("node_id: got %q, want n1", got)
	}
	if len(resp.GetMissingIds()) != 0 {
		t.Fatalf("missing_ids: got %v, want []", resp.GetMissingIds())
	}
}

// TestGetNodes_SingleMiss pins that an unknown id surfaces in
// missing_ids with status OK — NOT a NOT_FOUND batch error.
func TestGetNodes_SingleMiss(t *testing.T) {
	t.Parallel()
	const tenantID = "acme"

	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(context.Background(), tenantID, "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := gs.AddTenantMember(context.Background(), tenantID, "alice", "owner"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}
	cs := newStoreWithTenant(t, tenantID)

	srv := api.New(api.WithGlobalStore(gs), api.WithStore(cs))
	resp, err := srv.GetNodes(context.Background(), &pb.GetNodesRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "user:alice"},
		NodeIds: []string{"ghost"},
	})
	if err != nil {
		t.Fatalf("GetNodes: %v", err)
	}
	if len(resp.GetNodes()) != 0 {
		t.Fatalf("nodes: got %d, want 0", len(resp.GetNodes()))
	}
	if got := resp.GetMissingIds(); len(got) != 1 || got[0] != "ghost" {
		t.Fatalf("missing_ids: got %v, want [ghost]", got)
	}
}

// TestGetNodes_MultiPreservesOrderAndDuplicates pins two contract
// invariants in one test:
//   - Order: nodes in request order, gaps closed (denied/missing
//     are absent, not nil-padded).
//   - Duplicates: requesting the same id twice yields the node twice
//     (Python iterates verbatim — spec "Batch semantics" #5).
func TestGetNodes_MultiPreservesOrderAndDuplicates(t *testing.T) {
	t.Parallel()
	const tenantID = "acme"

	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(context.Background(), tenantID, "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := gs.AddTenantMember(context.Background(), tenantID, "alice", "owner"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}
	cs := newStoreWithTenant(t, tenantID)
	seedNodeForGetNodes(t, cs, tenantID, "n1", "user:alice")
	seedNodeForGetNodes(t, cs, tenantID, "n2", "user:alice")
	seedNodeForGetNodes(t, cs, tenantID, "n3", "user:alice")

	srv := api.New(api.WithGlobalStore(gs), api.WithStore(cs))
	resp, err := srv.GetNodes(context.Background(), &pb.GetNodesRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "user:alice"},
		// Request order: n3, missing-x, n1, n1 (duplicate), n2.
		NodeIds: []string{"n3", "missing-x", "n1", "n1", "n2"},
	})
	if err != nil {
		t.Fatalf("GetNodes: %v", err)
	}
	want := []string{"n3", "n1", "n1", "n2"}
	if got := nodeIDs(resp.GetNodes()); !equalSlice(got, want) {
		t.Fatalf("nodes order: got %v, want %v", got, want)
	}
	if got := resp.GetMissingIds(); len(got) != 1 || got[0] != "missing-x" {
		t.Fatalf("missing_ids: got %v, want [missing-x]", got)
	}
}

// TestGetNodes_MissingAndDeniedMergedIntoMissingIds is the
// information-leak guard: a cross-tenant non-member with at least one
// node_access grant on tenantID can read SOME nodes but not others.
// Per the spec, the wire MUST NOT distinguish "you don't have access"
// from "the node doesn't exist" — both cases land in missing_ids.
//
// Setup: alice owns three nodes in acme. mallory is not a member of
// acme but has a node_access grant on n1 (so can_access(n1) is true)
// and no grant on n2. Probing mallory through GetNodes should yield:
//
//   - n1 in nodes (visible cross-tenant).
//   - n2 in missing_ids (cross-tenant denied — no per-id error).
//   - ghost in missing_ids (truly absent).
//
// The two missing reasons are intentionally indistinguishable on the
// wire.
func TestGetNodes_MissingAndDeniedMergedIntoMissingIds(t *testing.T) {
	t.Parallel()
	const tenantID = "acme"

	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(context.Background(), tenantID, "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	// alice IS a member of acme; mallory is NOT.
	if err := gs.AddTenantMember(context.Background(), tenantID, "alice", "owner"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}
	cs := newStoreWithTenant(t, tenantID)
	seedNodeForGetNodes(t, cs, tenantID, "n1", "user:alice")
	seedNodeForGetNodes(t, cs, tenantID, "n2", "user:alice")

	// Grant mallory READ access to n1 only — this is the
	// `node_access` row that flips _check_cross_tenant_read from
	// PERMISSION_DENIED to "cross_tenant" + per-id filtering.
	if err := cs.ShareNode(context.Background(), tenantID, store.ShareNodeInput{
		NodeID:     "n1",
		ActorID:    "user:mallory",
		ActorType:  "user",
		Permission: "read",
		GrantedBy:  "user:alice",
	}); err != nil {
		t.Fatalf("ShareNode n1: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs), api.WithStore(cs))
	resp, err := srv.GetNodes(context.Background(), &pb.GetNodesRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "user:mallory"},
		NodeIds: []string{"n1", "n2", "ghost"},
	})
	if err != nil {
		t.Fatalf("GetNodes: unexpected gRPC error: %v", err)
	}
	// n1 is the only visible node.
	if got := nodeIDs(resp.GetNodes()); len(got) != 1 || got[0] != "n1" {
		t.Fatalf("nodes: got %v, want [n1]", got)
	}
	// Both n2 (denied) and ghost (missing) land in missing_ids —
	// indistinguishable.
	missing := resp.GetMissingIds()
	if !containsAll(missing, []string{"n2", "ghost"}) || len(missing) != 2 {
		t.Fatalf("missing_ids: got %v, want exactly [n2, ghost] in some order", missing)
	}
}

// TestGetNodes_OversizeBatchRejected pins the Go-port hardening
// delta: above maxBatchNodeIDs (1000) the handler aborts
// RESOURCE_EXHAUSTED before any tenant / store I/O. Python has no
// such cap; the cap is documented in the spec "Open Questions" #4.
func TestGetNodes_OversizeBatchRejected(t *testing.T) {
	t.Parallel()
	const tenantID = "acme"

	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(context.Background(), tenantID, "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	cs := newStoreWithTenant(t, tenantID)

	ids := make([]string, 1001) // > 1000 cap
	for i := range ids {
		ids[i] = "id-" + itoa(i)
	}

	srv := api.New(api.WithGlobalStore(gs), api.WithStore(cs))
	_, err := srv.GetNodes(context.Background(), &pb.GetNodesRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "user:alice"},
		NodeIds: ids,
	})
	if err == nil {
		t.Fatalf("expected RESOURCE_EXHAUSTED, got nil")
	}
	if got := status.Code(err); got != codes.ResourceExhausted {
		t.Fatalf("got code %v, want ResourceExhausted", got)
	}
}

// TestGetNodes_OversizeBatchExactlyAtCapAccepted is the boundary
// counterpart: a batch of exactly maxBatchNodeIDs (1000) is
// accepted (cap is EXCLUSIVE-of-greater, INCLUSIVE-of-cap).
func TestGetNodes_OversizeBatchExactlyAtCapAccepted(t *testing.T) {
	t.Parallel()
	const tenantID = "acme"

	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(context.Background(), tenantID, "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := gs.AddTenantMember(context.Background(), tenantID, "alice", "owner"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}
	cs := newStoreWithTenant(t, tenantID)

	ids := make([]string, 1000) // exactly at cap
	for i := range ids {
		ids[i] = "id-" + itoa(i)
	}

	srv := api.New(api.WithGlobalStore(gs), api.WithStore(cs))
	resp, err := srv.GetNodes(context.Background(), &pb.GetNodesRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "user:alice"},
		NodeIds: ids,
	})
	if err != nil {
		t.Fatalf("GetNodes at cap: %v", err)
	}
	// All 1000 ids miss (none seeded) -> all in missing_ids.
	if got := len(resp.GetMissingIds()); got != 1000 {
		t.Fatalf("missing_ids: got %d, want 1000", got)
	}
	if len(resp.GetNodes()) != 0 {
		t.Fatalf("nodes: got %d, want 0", len(resp.GetNodes()))
	}
}

// TestGetNodes_UnknownTenantNotFound pins the tenant-gate behaviour:
// an unknown tenant_id results in codes.NotFound from CheckTenant
// before any read work runs.
func TestGetNodes_UnknownTenantNotFound(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	cs := newStoreWithTenant(t, "acme") // open a different tenant
	srv := api.New(api.WithGlobalStore(gs), api.WithStore(cs))

	_, err := srv.GetNodes(context.Background(), &pb.GetNodesRequest{
		Context: &pb.RequestContext{TenantId: "ghost-tenant", Actor: "user:alice"},
		NodeIds: []string{"n1"},
	})
	if err == nil {
		t.Fatalf("expected NOT_FOUND, got nil")
	}
	if got := status.Code(err); got != codes.NotFound {
		t.Fatalf("got code %v, want NotFound", got)
	}
}

// nodeIDs lives in helpers_external_test.go.

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsAll(haystack, needles []string) bool {
	set := make(map[string]struct{}, len(haystack))
	for _, h := range haystack {
		set[h] = struct{}{}
	}
	for _, n := range needles {
		if _, ok := set[n]; !ok {
			return false
		}
	}
	return true
}

// itoa is a tiny strconv-free integer formatter so the
// oversize-batch tests don't drag in an import for one call site.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
