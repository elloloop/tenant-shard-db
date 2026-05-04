package entdb

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
	"github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/testpb"
)

// TestPayloadRoundTrip_IDKeyed_HydratesProtoFields verifies that a
// field-id-keyed wire payload (the contract per CLAUDE.md invariant
// #6 and the marshal.go:342 comment) round-trips into the typed
// proto message via Get[T] without requiring any name fallback.
//
// This is the regression test for the PR-D fix: the server now sends
// id-keyed payloads on read RPCs, and the Go SDK marshal layer must
// resolve those ids to proto field numbers without dropping data.
func TestPayloadRoundTrip_IDKeyed_HydratesProtoFields(t *testing.T) {
	// Build an id-keyed payload exactly as the server now writes it
	// onto the wire. Keys are "1" (sku), "2" (name), "3" (price_cents).
	payload, err := structpb.NewStruct(map[string]any{
		"1": "WIDGET-1",
		"2": "Widget Deluxe",
		"3": float64(1499), // structpb stores numerics as float64
	})
	if err != nil {
		t.Fatalf("payload: %v", err)
	}

	svc := &fakeServer{
		getNodeResp: &pb.GetNodeResponse{
			Found: true,
			Node: &pb.Node{
				TenantId:   "acme",
				NodeId:     "node-1",
				TypeId:     201, // testpb.Product type_id
				OwnerActor: "user:alice",
				CreatedAt:  1000,
				UpdatedAt:  2000,
				Payload:    payload,
			},
		},
	}
	tr := startFakeServer(t, svc)
	client := newClientWithTransport("bufnet", tr)
	scope := client.Tenant("acme").Actor(UserActor("alice"))

	got, err := Get[*testpb.Product](context.Background(), scope, "node-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}

	// Verify the typed proto fields are populated from id-keyed input.
	if got.SKU() != "WIDGET-1" {
		t.Errorf("SKU = %q, want WIDGET-1", got.SKU())
	}
	if got.Name() != "Widget Deluxe" {
		t.Errorf("Name = %q, want Widget Deluxe", got.Name())
	}
	if got.PriceCents() != 1499 {
		t.Errorf("PriceCents = %d, want 1499", got.PriceCents())
	}
}

// TestPayloadRoundTrip_QueryReturnsTypedSlice mirrors the GetNode
// case for Query[T]: id-keyed wire payloads must hydrate the typed
// slice without any name-key fallback.
func TestPayloadRoundTrip_QueryReturnsTypedSlice(t *testing.T) {
	mk := func(sku, name string, price float64) *structpb.Struct {
		s, err := structpb.NewStruct(map[string]any{
			"1": sku,
			"2": name,
			"3": price,
		})
		if err != nil {
			t.Fatalf("payload: %v", err)
		}
		return s
	}

	svc := &fakeServer{
		queryResp: &pb.QueryNodesResponse{
			Nodes: []*pb.Node{
				{
					TenantId: "acme",
					NodeId:   "n1",
					TypeId:   201,
					Payload:  mk("A", "Apple", 100),
				},
				{
					TenantId: "acme",
					NodeId:   "n2",
					TypeId:   201,
					Payload:  mk("B", "Banana", 200),
				},
			},
		},
	}
	tr := startFakeServer(t, svc)
	client := newClientWithTransport("bufnet", tr)
	scope := client.Tenant("acme").Actor(UserActor("alice"))

	got, err := Query[*testpb.Product](context.Background(), scope, nil)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d products, want 2", len(got))
	}

	skus := map[string]string{got[0].SKU(): got[0].Name(), got[1].SKU(): got[1].Name()}
	if skus["A"] != "Apple" {
		t.Errorf("expected A=Apple, got %v", skus)
	}
	if skus["B"] != "Banana" {
		t.Errorf("expected B=Banana, got %v", skus)
	}
}
