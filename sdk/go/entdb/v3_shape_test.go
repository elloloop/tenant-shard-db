package entdb

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/testpb"
)

// ProductSKU is a hand-rolled token that mirrors what the
// protoc-gen-entdb-keys codegen plugin would emit for the Product
// message's “sku“ field:
//
//	string sku = 1 [(entdb.field).unique = true];
//
// Type-parameterised as UniqueKey[string] so “GetByKey(ctx, scope,
// ProductSKU, 12345)“ is a compile-time error.
var ProductSKU = UniqueKey[string]{TypeID: 201, FieldID: 1, Name: "sku"}

// TestSingleShapeFlow exercises the canonical end-to-end shape of
// the 2026-04-14 SDK v0.3 API: one Create, one Update, one Delete,
// one Edge op, one generic Get, and one typed GetByKey — all going
// through the same typed surface user code sees.
func TestSingleShapeFlow(t *testing.T) {
	ctx := context.Background()

	// Create the SDK's fake transport and a bound scope.
	transport := &mockTransport{
		commitResp: &CommitResult{
			Success:        true,
			CreatedNodeIDs: []string{"node-1"},
			Applied:        true,
		},
		getNodeResp: &Node{
			TenantID: "acme",
			NodeID:   "node-1",
			TypeID:   201,
			Payload: map[string]any{
				"1": "WIDGET-1",
				"2": "Widget",
				"3": int64(999),
			},
		},
		getNodeByKeyResp: &Node{
			TenantID: "acme",
			NodeID:   "node-1",
			TypeID:   201,
			Payload:  map[string]any{"1": "WIDGET-1"},
		},
		queryResp: []*Node{
			{
				TenantID: "acme",
				NodeID:   "node-1",
				TypeID:   201,
				Payload:  map[string]any{"1": "WIDGET-1", "2": "Widget"},
			},
		},
	}
	client := newClientWithTransport("localhost:50051", transport)
	scope := client.Tenant("acme").Actor(UserActor("alice"))

	// ── Plan.Create ────────────────────────────────────────────
	plan := scope.Plan()
	p := testpb.NewProductMsg()
	p.SetFields("WIDGET-1", "Widget", 999)
	alias := plan.Create(p)
	if alias == "" {
		t.Fatal("Plan.Create returned empty alias")
	}

	// ── Plan.Update (partial patch; only price_cents set) ─────
	patch := testpb.NewProductMsg()
	patch.SetFields("", "", 1299)
	plan.Update("node-1", patch)

	// ── Plan.Delete (generic witness, no instance needed) ─────
	Delete[*testpb.Product](plan, "old-node")

	// ── Edge ops (generic witness) ────────────────────────────
	EdgeCreate[*testpb.PurchaseEdge](plan, "user-1", "node-1")
	EdgeDelete[*testpb.PurchaseEdge](plan, "user-1", "stale-node")

	result, err := plan.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if !result.Success {
		t.Fatal("Commit not successful")
	}

	// Assert the transport recorded the expected ops shape.
	ops := transport.lastCommitOperations
	if len(ops) != 5 {
		t.Fatalf("committed ops = %d, want 5", len(ops))
	}

	// op[0] = Create
	if ops[0].Type != OpCreateNode {
		t.Errorf("ops[0].Type = %v, want OpCreateNode", ops[0].Type)
	}
	if ops[0].TypeID != 201 {
		t.Errorf("ops[0].TypeID = %d, want 201", ops[0].TypeID)
	}
	if ops[0].Data["1"] != "WIDGET-1" {
		t.Errorf("ops[0].Data[1] = %v, want WIDGET-1", ops[0].Data["1"])
	}
	if ops[0].Data["2"] != "Widget" {
		t.Errorf("ops[0].Data[2] = %v, want Widget", ops[0].Data["2"])
	}
	if ops[0].Data["3"] != int64(999) {
		t.Errorf("ops[0].Data[3] = %v, want int64(999)", ops[0].Data["3"])
	}

	// op[1] = Update — only price_cents
	if ops[1].Type != OpUpdateNode {
		t.Errorf("ops[1].Type = %v, want OpUpdateNode", ops[1].Type)
	}
	if ops[1].NodeID != "node-1" {
		t.Errorf("ops[1].NodeID = %q", ops[1].NodeID)
	}
	if ops[1].TypeID != 201 {
		t.Errorf("ops[1].TypeID = %d, want 201", ops[1].TypeID)
	}
	if ops[1].Patch["3"] != int64(1299) {
		t.Errorf("ops[1].Patch[3] = %v, want 1299", ops[1].Patch["3"])
	}
	if _, ok := ops[1].Patch["1"]; ok {
		t.Errorf("ops[1].Patch should not contain unset sku field")
	}

	// op[2] = Delete
	if ops[2].Type != OpDeleteNode {
		t.Errorf("ops[2].Type = %v, want OpDeleteNode", ops[2].Type)
	}
	if ops[2].TypeID != 201 {
		t.Errorf("ops[2].TypeID = %d, want 201", ops[2].TypeID)
	}
	if ops[2].NodeID != "old-node" {
		t.Errorf("ops[2].NodeID = %q, want old-node", ops[2].NodeID)
	}

	// op[3] = EdgeCreate — edge_id = 301
	if ops[3].Type != OpCreateEdge {
		t.Errorf("ops[3].Type = %v, want OpCreateEdge", ops[3].Type)
	}
	if ops[3].EdgeTypeID != 301 {
		t.Errorf("ops[3].EdgeTypeID = %d, want 301", ops[3].EdgeTypeID)
	}
	if ops[3].FromNodeID != "user-1" || ops[3].ToNodeID != "node-1" {
		t.Errorf("ops[3] endpoints = %q -> %q", ops[3].FromNodeID, ops[3].ToNodeID)
	}

	// op[4] = EdgeDelete
	if ops[4].Type != OpDeleteEdge {
		t.Errorf("ops[4].Type = %v, want OpDeleteEdge", ops[4].Type)
	}
	if ops[4].EdgeTypeID != 301 {
		t.Errorf("ops[4].EdgeTypeID = %d, want 301", ops[4].EdgeTypeID)
	}

	// ── Generic Get ────────────────────────────────────────────
	got, err := Get[*testpb.Product](ctx, scope, "node-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.SKU() != "WIDGET-1" {
		t.Errorf("got.SKU() = %q, want WIDGET-1", got.SKU())
	}
	if got.Name() != "Widget" {
		t.Errorf("got.Name() = %q, want Widget", got.Name())
	}
	if got.PriceCents() != 999 {
		t.Errorf("got.PriceCents() = %d, want 999", got.PriceCents())
	}
	if transport.lastGetTypeID != 201 {
		t.Errorf("transport.lastGetTypeID = %d, want 201", transport.lastGetTypeID)
	}

	// ── GetByKey via typed token ───────────────────────────────
	byKey, err := GetByKey(ctx, scope, ProductSKU, "WIDGET-1")
	if err != nil {
		t.Fatalf("GetByKey: %v", err)
	}
	if byKey == nil {
		t.Fatal("GetByKey returned nil node")
	}
	if transport.lastGetByKeyTypeID != 201 {
		t.Errorf("lastGetByKeyTypeID = %d, want 201", transport.lastGetByKeyTypeID)
	}
	if transport.lastGetByKeyFieldID != 1 {
		t.Errorf("lastGetByKeyFieldID = %d, want 1", transport.lastGetByKeyFieldID)
	}
	if transport.lastGetByKeyValue == nil || transport.lastGetByKeyValue.GetStringValue() != "WIDGET-1" {
		t.Errorf("lastGetByKeyValue = %v, want string WIDGET-1", transport.lastGetByKeyValue)
	}

	// ── Generic Query ──────────────────────────────────────────
	results, err := Query[*testpb.Product](ctx, scope,
		map[string]any{"price_cents": map[string]any{"$gte": 500}})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Query results = %d, want 1", len(results))
	}
	if results[0].SKU() != "WIDGET-1" {
		t.Errorf("results[0].SKU() = %q, want WIDGET-1", results[0].SKU())
	}
	if transport.lastQueryTypeID != 201 {
		t.Errorf("lastQueryTypeID = %d, want 201", transport.lastQueryTypeID)
	}
}

// TestDelete_NonEntityTypePanics verifies that using the generic
// witness pattern on a message type without “(entdb.node)“
// panics at call time.
func TestDelete_NonEntityTypePanics(t *testing.T) {
	transport := &mockTransport{}
	client := newClientWithTransport("localhost:50051", transport)
	plan := client.Tenant("t").Actor(UserActor("alice")).Plan()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for non-entity type")
		}
	}()
	Delete[*testpb.NotAnEntity](plan, "node-x")
}

// TestEdgeCreate_NonEdgeTypePanics verifies the edge witness path
// for the symmetric case.
func TestEdgeCreate_NonEdgeTypePanics(t *testing.T) {
	transport := &mockTransport{}
	client := newClientWithTransport("localhost:50051", transport)
	plan := client.Tenant("t").Actor(UserActor("alice")).Plan()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for non-edge type")
		}
	}()
	EdgeCreate[*testpb.Product](plan, "a", "b")
}

// TestUniqueKey_TypedValueWitness is a compile-time test in
// disguise: the whole point of the typed UniqueKey[T] token is
// that passing the wrong value type to GetByKey does not
// type-check. We can't assert a compile error from inside a test,
// but we can document the expected call shape and exercise the
// happy path with the correct scalar type.
func TestUniqueKey_TypedValueWitness(t *testing.T) {
	ctx := context.Background()
	transport := &mockTransport{getNodeByKeyResp: &Node{NodeID: "n1"}}
	client := newClientWithTransport("localhost:50051", transport)
	scope := client.Tenant("t").Actor(UserActor("alice"))

	// Correct type: UniqueKey[string] with a string value.
	if _, err := GetByKey(ctx, scope, ProductSKU, "WIDGET-1"); err != nil {
		t.Fatalf("GetByKey with correct value type: %v", err)
	}

	// The following WOULD be a compile error if uncommented:
	//
	//   _, _ = GetByKey(ctx, scope, ProductSKU, 12345)
	//
	// because ProductSKU is UniqueKey[string] and 12345 is an int.
}
