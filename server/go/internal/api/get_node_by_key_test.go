// Tests for the GetNodeByKey RPC (W2 — EPIC #407).
//
// Spec: docs/go-port/rpcs/GetNodeByKey.md. The four cases pinned here
// match the contract bullets the spec calls out:
//
//  1. Happy round-trip: a node carrying a unique value resolves and
//     returns Found=true with the wire shape preserved (mirrors
//     test_unique_keys.py:597-623, test_grpc_contract.py:614-626).
//
//  2. Missing key: no row matches the (type, field, value) tuple ->
//     Found=false, codes.OK. NOT codes.NotFound — preserves the
//     Python implementation contract over the unique_keys.md
//     decision text (mirrors test_unique_keys.py:625-640).
//
//  3. ACL denied: a row exists but its acl_blob carries an explicit
//     DENY for the trusted actor -> codes.PermissionDenied. The deny
//     is the contract pin for the "key resolved but actor blocked"
//     branch of the spec's error contract.
//
//  4. Composite key: multi-field unique tuple resolves via the
//     store-level GetNodeByCompositeKey helper. The RPC itself does
//     NOT yet expose composite keys (spec "Open questions / risks"
//     #1) — this test drives the store API directly so the
//     ExecuteAtomic pre-check has a typed entry point ready.

package api_test

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/tenant"
)

const (
	gnbkTenant = "acme"
	gnbkRegion = "us-east-1"
	gnbkNodeID = "alice-id"
	gnbkType   = int32(7)
	gnbkField  = int32(1)
)

// newCanonicalStore opens a tmpdir-backed store + opens one tenant. We
// don't reach for a Registry — GetNodeByKey relies on the unique
// expression index lazily created by the applier, but the SELECT works
// without it (degrades to a scan); pure read tests don't need the
// index for correctness.
func newCanonicalStore(t *testing.T, tenantID string) *store.CanonicalStore {
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

// seedNode writes a node row keyed by field_id (CLAUDE.md invariant
// #6) so the unique-index SELECT lights it up.
func seedNode(t *testing.T, cs *store.CanonicalStore, tenantID, nodeID, ownerActor string, payload map[string]any, acl []store.ACLEntry) {
	t.Helper()
	if _, err := cs.CreateNodeRaw(context.Background(), tenantID, store.NodeInput{
		NodeID:     nodeID,
		TypeID:     gnbkType,
		Payload:    payload,
		OwnerActor: ownerActor,
		ACL:        acl,
	}); err != nil {
		t.Fatalf("CreateNodeRaw: %v", err)
	}
}

// stringValue wraps a string in google.protobuf.Value the same way an
// SDK transport would.
func stringValue(t *testing.T, s string) *structpb.Value {
	t.Helper()
	v, err := structpb.NewValue(s)
	if err != nil {
		t.Fatalf("structpb.NewValue: %v", err)
	}
	return v
}

// TestGetNodeByKey_HappyRoundTrip seeds a User node with email=alice@…
// and asserts the lookup hits, returns Found=true, and preserves the
// node id + payload field id keying (`{"1": "alice@example.com"}`).
func TestGetNodeByKey_HappyRoundTrip(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, gnbkTenant, "Acme", gnbkRegion); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	cs := newCanonicalStore(t, gnbkTenant)
	seedNode(t, cs, gnbkTenant, gnbkNodeID, "user:alice",
		map[string]any{"1": "alice@example.com"}, nil)

	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithStore(cs),
		api.WithSharding(&tenant.Sharding{NodeID: "n1"}),
		api.WithRegion(gnbkRegion),
	)

	resp, err := srv.GetNodeByKey(ctx, &pb.GetNodeByKeyRequest{
		TenantId: gnbkTenant,
		Actor:    "user:alice",
		TypeId:   gnbkType,
		FieldId:  gnbkField,
		Value:    stringValue(t, "alice@example.com"),
	})
	if err != nil {
		t.Fatalf("GetNodeByKey: unexpected err: %v", err)
	}
	if !resp.GetFound() {
		t.Fatalf("GetNodeByKey: Found=false, want true")
	}
	got := resp.GetNode()
	if got == nil {
		t.Fatalf("GetNodeByKey: Node is nil despite Found=true")
	}
	if got.GetNodeId() != gnbkNodeID {
		t.Fatalf("GetNodeByKey: NodeId=%q, want %q", got.GetNodeId(), gnbkNodeID)
	}
	if got.GetTypeId() != gnbkType {
		t.Fatalf("GetNodeByKey: TypeId=%d, want %d", got.GetTypeId(), gnbkType)
	}
	if got.GetTenantId() != gnbkTenant {
		t.Fatalf("GetNodeByKey: TenantId=%q, want %q", got.GetTenantId(), gnbkTenant)
	}
	if got.GetOwnerActor() != "user:alice" {
		t.Fatalf("GetNodeByKey: OwnerActor=%q, want %q", got.GetOwnerActor(), "user:alice")
	}
	pl := got.GetPayload()
	if pl == nil {
		t.Fatalf("GetNodeByKey: Payload is nil")
	}
	v := pl.GetFields()["1"]
	if v == nil || v.GetStringValue() != "alice@example.com" {
		t.Fatalf("GetNodeByKey: payload[1]=%v, want alice@example.com", v)
	}
}

// TestGetNodeByKey_MissingKey pins the in-band absence: a non-existent
// (type, field, value) tuple yields Found=false, codes.OK. NOT
// codes.NotFound — SDK clients branch on resp.Found.
func TestGetNodeByKey_MissingKey(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, gnbkTenant, "Acme", gnbkRegion); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	cs := newCanonicalStore(t, gnbkTenant)

	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithStore(cs),
		api.WithSharding(&tenant.Sharding{NodeID: "n1"}),
		api.WithRegion(gnbkRegion),
	)

	resp, err := srv.GetNodeByKey(ctx, &pb.GetNodeByKeyRequest{
		TenantId: gnbkTenant,
		Actor:    "user:alice",
		TypeId:   gnbkType,
		FieldId:  gnbkField,
		Value:    stringValue(t, "ghost@example.com"),
	})
	if err != nil {
		t.Fatalf("GetNodeByKey: unexpected err for missing key: %v", err)
	}
	if resp == nil {
		t.Fatalf("GetNodeByKey: nil response for missing key")
	}
	if resp.GetFound() {
		t.Fatalf("GetNodeByKey: Found=true, want false for missing key")
	}
	if resp.GetNode() != nil {
		t.Fatalf("GetNodeByKey: Node populated despite Found=false: %+v", resp.GetNode())
	}
}

// TestGetNodeByKey_ACLDenied seeds a node owned by "user:alice" with
// an explicit DENY on "user:bob". A lookup as "user:bob" returns
// codes.PermissionDenied, distinct from the Found=false miss.
func TestGetNodeByKey_ACLDenied(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, gnbkTenant, "Acme", gnbkRegion); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	cs := newCanonicalStore(t, gnbkTenant)

	seedNode(t, cs, gnbkTenant, gnbkNodeID, "user:alice",
		map[string]any{"1": "alice@example.com"},
		[]store.ACLEntry{{Principal: "user:bob", Permission: "deny"}},
	)

	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithStore(cs),
		api.WithSharding(&tenant.Sharding{NodeID: "n1"}),
		api.WithRegion(gnbkRegion),
	)

	resp, err := srv.GetNodeByKey(ctx, &pb.GetNodeByKeyRequest{
		TenantId: gnbkTenant,
		Actor:    "user:bob",
		TypeId:   gnbkType,
		FieldId:  gnbkField,
		Value:    stringValue(t, "alice@example.com"),
	})
	if err == nil {
		t.Fatalf("GetNodeByKey: expected PermissionDenied, got resp=%+v", resp)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("GetNodeByKey: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("GetNodeByKey: code=%v, want PermissionDenied", st.Code())
	}
}

// TestGetNodeByCompositeKey_StoreLevel exercises the composite-unique
// store helper (no RPC for composite lookup yet — see spec "Open
// questions / risks" #1). Two unique-fields-per-tuple lookup hits;
// non-matching value misses with (nil, nil).
func TestGetNodeByCompositeKey_StoreLevel(t *testing.T) {
	t.Parallel()
	cs := newCanonicalStore(t, gnbkTenant)
	ctx := context.Background()

	// Seed a Task node with a (owner_id=alice, title="finish go port")
	// composite tuple. Field ids 2 and 1 (Python sample).
	seedNode(t, cs, gnbkTenant, "task-1", "user:alice",
		map[string]any{
			"1": "finish go port",
			"2": "alice",
		},
		nil,
	)

	hit, err := cs.GetNodeByCompositeKey(ctx, gnbkTenant, gnbkType,
		[]int32{2, 1}, []any{"alice", "finish go port"})
	if err != nil {
		t.Fatalf("GetNodeByCompositeKey hit: %v", err)
	}
	if hit == nil {
		t.Fatalf("GetNodeByCompositeKey: nil for matching composite key")
	}
	if hit.NodeID != "task-1" {
		t.Fatalf("GetNodeByCompositeKey: NodeID=%q, want task-1", hit.NodeID)
	}
	// Round-trip the payload to be sure the field-id keying survived.
	var pl map[string]any
	if err := json.Unmarshal([]byte(hit.PayloadJSON), &pl); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}
	if pl["2"] != "alice" {
		t.Fatalf("payload[2]=%v, want alice", pl["2"])
	}

	miss, err := cs.GetNodeByCompositeKey(ctx, gnbkTenant, gnbkType,
		[]int32{2, 1}, []any{"alice", "different title"})
	if err != nil {
		t.Fatalf("GetNodeByCompositeKey miss: %v", err)
	}
	if miss != nil {
		t.Fatalf("GetNodeByCompositeKey: got %+v, want nil for non-matching composite key", miss)
	}
}
