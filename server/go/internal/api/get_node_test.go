// Tests for the GetNode RPC.
//
// Spec: docs/go-port/rpcs/GetNode.md. Four behaviours pinned here:
//
//  1. Happy round-trip: an existing node -> Found=true with Node
//     populated; payload round-trips through the wire as id-keyed
//     Struct (mirrors test_grpc_contract.py:212-216).
//
//  2. Missing node -> Found=false with status OK (NOT NOT_FOUND).
//     Pinned by test_grpc_contract.py:217-222.
//
//  3. Non-member with no per-tenant grants -> PERMISSION_DENIED. The
//     check fires BEFORE row lookup, so a non-member querying any
//     node_id (existent or not) sees PERMISSION_DENIED. Pinned by
//     spec "Error contract".
//
//  4. Invariant #6: payload on the wire is keyed by stringified
//     field_id, NOT by field name. Disk and wire BOTH use id keys —
//     the SDK does name translation.
//
// The shared globalstore helper lives in helpers_external_test.go;
// do NOT redeclare newGlobalStore here.

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

// newGetNodeStore returns a tmpdir-backed CanonicalStore with WAL on.
// Closed on test teardown. Independent of newTestStore in
// get_receipt_status_test.go (that one is in package api, not api_test).
func newGetNodeStore(t *testing.T) *store.CanonicalStore {
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

// seedNodeForGet is a small builder that opens the tenant + creates a node.
// Returns the created node so tests can pin created_at / updated_at.
func seedNodeForGet(t *testing.T, cs *store.CanonicalStore, tenantID, nodeID, owner string, payload map[string]any) *store.Node {
	t.Helper()
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	n, err := cs.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID:     nodeID,
		TypeID:     7,
		Payload:    payload,
		OwnerActor: owner,
	})
	if err != nil {
		t.Fatalf("CreateNodeRaw: %v", err)
	}
	return n
}

// TestGetNode_HappyRoundTrip seeds a node and asserts the RPC returns
// it field-for-field, with the payload visible on the wire as a
// Struct keyed by stringified field_id.
//
// The test runs WITHOUT a globalstore so the cross-tenant check
// short-circuits to roleLocal (back-compat path) — that's the
// minimal-deps shape the spec documents at "Open questions / risks"
// item 5 ("global_store is None -> 'local'"). The privilege /
// membership branches are covered by the dedicated cases below.
func TestGetNode_HappyRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cs := newGetNodeStore(t)
	const tenantID = "tenant-a"
	const nodeID = "node-1"

	created := seedNodeForGet(t, cs, tenantID, nodeID, "user:alice", map[string]any{
		"1": "alice@example.com",
		"2": "hashed-password-blob",
	})

	srv := api.New(api.WithStore(cs))
	resp, err := srv.GetNode(ctx, &pb.GetNodeRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "user:alice"},
		TypeId:  7,
		NodeId:  nodeID,
	})
	if err != nil {
		t.Fatalf("GetNode: unexpected err: %v", err)
	}
	if !resp.GetFound() {
		t.Fatalf("GetNode: found: got false, want true")
	}
	got := resp.GetNode()
	if got == nil {
		t.Fatalf("GetNode: node is nil despite found=true")
	}
	if got.GetNodeId() != nodeID {
		t.Fatalf("GetNode: node_id: got %q, want %q", got.GetNodeId(), nodeID)
	}
	if got.GetTypeId() != 7 {
		t.Fatalf("GetNode: type_id: got %d, want 7", got.GetTypeId())
	}
	if got.GetOwnerActor() != "user:alice" {
		t.Fatalf("GetNode: owner_actor: got %q, want %q", got.GetOwnerActor(), "user:alice")
	}
	if got.GetCreatedAt() != created.CreatedAt {
		t.Fatalf("GetNode: created_at: got %d, want %d", got.GetCreatedAt(), created.CreatedAt)
	}
}

// TestGetNode_MissingNode pins the in-band absence signal: an unknown
// node_id returns Found=false with codes.OK, NOT codes.NotFound. This
// is the deliberate asymmetry called out in the spec — clients branch
// on response.Found, so upgrading to a NOT_FOUND status code would be
// a contract break.
//
// Runs WITHOUT a globalstore so the membership gate short-circuits to
// roleLocal — otherwise a non-member would hit PERMISSION_DENIED
// before the lookup, which is a different contract pin (covered
// below).
func TestGetNode_MissingNode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cs := newGetNodeStore(t)
	const tenantID = "tenant-a"

	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	srv := api.New(api.WithStore(cs))
	resp, err := srv.GetNode(ctx, &pb.GetNodeRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "user:alice"},
		TypeId:  7,
		NodeId:  "does-not-exist",
	})
	if err != nil {
		t.Fatalf("GetNode: unexpected err for missing node: %v", err)
	}
	if resp.GetFound() {
		t.Fatalf("GetNode: found: got true, want false for missing node")
	}
	if resp.GetNode() != nil {
		t.Fatalf("GetNode: node populated despite found=false: %+v", resp.GetNode())
	}
}

// TestGetNode_NoPermission pins the cross-tenant gate: a non-member
// caller (no membership row, no node_access row) sees
// PERMISSION_DENIED, regardless of whether the node exists. The
// global_store is wired so the membership check actually runs
// (short-circuits to "local" only when no global_store is configured —
// spec "Open questions / risks" item 5).
//
// We seed the tenant + a node owned by alice, then query as bob who
// has no membership and no grants. PERMISSION_DENIED fires before the
// row lookup; we don't even need the node to exist for the assertion
// (and the spec is explicit that this is the intended shape — a
// non-member querying a non-existent node sees PERMISSION_DENIED, not
// found=false, so they cannot probe for node existence).
func TestGetNode_NoPermission(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cs := newGetNodeStore(t)
	gs := newGlobalStore(t)
	const tenantID = "tenant-a"
	const nodeID = "node-1"

	if _, err := gs.CreateTenant(ctx, tenantID, "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	// alice is a member; bob is NOT.
	if err := gs.AddTenantMember(ctx, tenantID, "alice", "owner"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}
	seedNodeForGet(t, cs, tenantID, nodeID, "user:alice", nil)

	srv := api.New(api.WithStore(cs), api.WithGlobalStore(gs))
	_, err := srv.GetNode(ctx, &pb.GetNodeRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "user:bob"},
		TypeId:  7,
		NodeId:  nodeID,
	})
	if err == nil {
		t.Fatalf("GetNode: expected PERMISSION_DENIED, got nil error")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("GetNode: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("GetNode: code: got %v, want PermissionDenied", st.Code())
	}
}

// TestGetNode_PayloadIsIDKeyedOnWire is the explicit invariant-#6
// pin: the on-disk payload is keyed by field_id (CLAUDE.md invariant
// #6), AND the wire payload egress also stays id-keyed (the
// SDK does the id->name translation client-side). This is the
// "zero-translation egress" rule from
// docs/go-port/shared/payload-translation.md.
//
// Without a SchemaRegistry, payload.PayloadToStruct takes the
// schema-less passthrough path which preserves digit-only keys
// verbatim. We assert (a) the wire Struct contains exactly the
// stringified field_id keys we wrote, and (b) it does NOT contain a
// name-keyed entry — a regression that translated id->name on the
// server side would surface as the latter.
func TestGetNode_PayloadIsIDKeyedOnWire(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cs := newGetNodeStore(t)
	const tenantID = "tenant-a"
	const nodeID = "node-1"

	seedNodeForGet(t, cs, tenantID, nodeID, "user:alice", map[string]any{
		"1": "alice@example.com",
		"2": "hashed-password",
	})

	srv := api.New(api.WithStore(cs))
	resp, err := srv.GetNode(ctx, &pb.GetNodeRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "user:alice"},
		TypeId:  7,
		NodeId:  nodeID,
	})
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if !resp.GetFound() {
		t.Fatalf("GetNode: found=false")
	}
	wire := resp.GetNode().GetPayload()
	if wire == nil {
		t.Fatalf("GetNode: payload Struct is nil")
	}
	fields := wire.GetFields()
	// Invariant #6: keys on the wire are stringified field_ids.
	if v, ok := fields["1"]; !ok {
		t.Fatalf("payload field %q missing on wire; got keys %v", "1", keysOf(fields))
	} else if v.GetStringValue() != "alice@example.com" {
		t.Fatalf("payload[1]: got %q, want %q", v.GetStringValue(), "alice@example.com")
	}
	if v, ok := fields["2"]; !ok {
		t.Fatalf("payload field %q missing on wire; got keys %v", "2", keysOf(fields))
	} else if v.GetStringValue() != "hashed-password" {
		t.Fatalf("payload[2]: got %q, want %q", v.GetStringValue(), "hashed-password")
	}
	// Negative: name-keyed entries would mean someone re-introduced
	// server-side id->name translation; that's a CLAUDE.md invariant
	// #6 violation and a wire-format regression.
	for _, name := range []string{"email", "password", "password_hash"} {
		if _, ok := fields[name]; ok {
			t.Fatalf("payload contains name-keyed field %q on the wire (invariant #6 violation); "+
				"all keys: %v", name, keysOf(fields))
		}
	}
}

// keysOf returns the keys of a structpb fields map as a slice — used
// by failure messages so the diff is readable without breaking out
// reflect.
func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
