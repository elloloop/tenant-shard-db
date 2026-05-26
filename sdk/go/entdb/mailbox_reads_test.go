// SPDX-License-Identifier: MIT
package entdb

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2/internal/pb"
	"github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2/internal/testpb"
)

// productPayload is an id-keyed wire payload for testpb.Product
// (sku=1, name=2, price_cents=3).
func productPayload(t *testing.T, sku, name string, price float64) *structpb.Struct {
	t.Helper()
	p, err := structpb.NewStruct(map[string]any{"1": sku, "2": name, "3": price})
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	return p
}

// TestMailbox_GetInMailbox_SetsTargetUser proves GetInMailbox forwards
// target_user on the wire and hydrates the typed result.
func TestMailbox_GetInMailbox_SetsTargetUser(t *testing.T) {
	svc := &fakeServer{
		getNodeResp: &pb.GetNodeResponse{
			Found: true,
			Node: &pb.Node{
				TenantId: "acme", NodeId: "m1", TypeId: 201,
				OwnerActor: "user:alice", Payload: productPayload(t, "SKU1", "Mail", 1),
			},
		},
	}
	tr := startFakeServer(t, svc)
	scope := newClientWithTransport("bufnet", tr).Tenant("acme").Actor(UserActor("alice"))

	got, err := GetInMailbox[*testpb.Product](context.Background(), scope, "alice", "m1")
	if err != nil {
		t.Fatalf("GetInMailbox: %v", err)
	}
	if got == nil || got.SKU() != "SKU1" {
		t.Fatalf("GetInMailbox: bad result %+v", got)
	}
	if svc.getNodeReq.GetTargetUser() != "alice" {
		t.Fatalf("wire target_user = %q, want alice", svc.getNodeReq.GetTargetUser())
	}
}

// TestMailbox_QueryInMailbox_SetsTargetUser proves QueryInMailbox
// forwards target_user on the wire.
func TestMailbox_QueryInMailbox_SetsTargetUser(t *testing.T) {
	svc := &fakeServer{
		queryResp: &pb.QueryNodesResponse{
			Nodes: []*pb.Node{
				{TenantId: "acme", NodeId: "m1", TypeId: 201, OwnerActor: "user:alice", Payload: productPayload(t, "S", "N", 1)},
			},
		},
	}
	tr := startFakeServer(t, svc)
	scope := newClientWithTransport("bufnet", tr).Tenant("acme").Actor(UserActor("alice"))

	got, err := QueryInMailbox[*testpb.Product](context.Background(), scope, "alice", nil)
	if err != nil {
		t.Fatalf("QueryInMailbox: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("QueryInMailbox: want 1 node, got %d", len(got))
	}
	if svc.queryReq.GetTargetUser() != "alice" {
		t.Fatalf("wire target_user = %q, want alice", svc.queryReq.GetTargetUser())
	}
}

// TestMailbox_SearchInMailbox_SetsTargetUser proves SearchInMailbox
// forwards target_user on the wire.
func TestMailbox_SearchInMailbox_SetsTargetUser(t *testing.T) {
	svc := &fakeServer{
		searchResp: &pb.SearchNodesResponse{
			Nodes: []*pb.Node{
				{TenantId: "acme", NodeId: "m1", TypeId: 201, OwnerActor: "user:alice", Payload: productPayload(t, "S", "N", 1)},
			},
		},
	}
	tr := startFakeServer(t, svc)
	scope := newClientWithTransport("bufnet", tr).Tenant("acme").Actor(UserActor("alice"))

	got, err := SearchInMailbox[*testpb.Product](context.Background(), scope, "alice", "widget")
	if err != nil {
		t.Fatalf("SearchInMailbox: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("SearchInMailbox: want 1 node, got %d", len(got))
	}
	if svc.searchReq.GetTargetUser() != "alice" {
		t.Fatalf("wire target_user = %q, want alice", svc.searchReq.GetTargetUser())
	}
	if svc.searchReq.GetQuery() != "widget" {
		t.Fatalf("wire query = %q, want widget", svc.searchReq.GetQuery())
	}
}

// TestMailbox_GetInMailbox_NotFound confirms a not-found mailbox read
// returns the zero value (no leak), mirroring Get.
func TestMailbox_GetInMailbox_NotFound(t *testing.T) {
	svc := &fakeServer{getNodeResp: &pb.GetNodeResponse{Found: false}}
	tr := startFakeServer(t, svc)
	scope := newClientWithTransport("bufnet", tr).Tenant("acme").Actor(UserActor("alice"))

	got, err := GetInMailbox[*testpb.Product](context.Background(), scope, "alice", "missing")
	if err != nil {
		t.Fatalf("GetInMailbox: %v", err)
	}
	if got != nil {
		t.Fatalf("GetInMailbox(not found): want nil, got %+v", got)
	}
}
