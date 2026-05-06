package main

import (
	"strings"
	"testing"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

func TestNodesList_NumericType(t *testing.T) {
	resetFlags()
	out, _, restore := captureOutput()
	defer restore()
	svc := startFake(t, &fakeCLIServer{
		queryResp: &pb.QueryNodesResponse{
			Nodes: []*pb.Node{
				{
					NodeId:     "n-1",
					TypeId:     201,
					OwnerActor: "user:alice",
					CreatedAt:  1_700_000_000_000,
					Payload:    mustStruct(t, map[string]any{"1": "WIDGET-1"}),
				},
			},
		},
	})

	if err := runNodesList("acme", "201", 50, 0); err != nil {
		t.Fatalf("runNodesList: %v", err)
	}
	if svc.queryReq.GetTypeId() != 201 {
		t.Errorf("server saw type_id = %d, want 201", svc.queryReq.GetTypeId())
	}
	if svc.queryReq.GetContext().GetTenantId() != "acme" {
		t.Errorf("server saw tenant = %q", svc.queryReq.GetContext().GetTenantId())
	}
	if svc.queryReq.GetContext().GetActor() != "user:alice" {
		t.Errorf("server saw actor = %q", svc.queryReq.GetContext().GetActor())
	}
	if svc.queryReq.GetLimit() != 50 {
		t.Errorf("limit = %d", svc.queryReq.GetLimit())
	}
	got := out.String()
	for _, want := range []string{"n-1", "201", "user:alice", "WIDGET-1"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q: %s", want, got)
		}
	}
}

func TestNodesList_NameResolution(t *testing.T) {
	resetFlags()
	_, _, restore := captureOutput()
	defer restore()
	svc := startFake(t, &fakeCLIServer{
		schemaResp: &pb.GetSchemaResponse{
			Schema: mustStruct(t, map[string]any{
				"node_types": []any{
					map[string]any{"type_id": float64(202), "name": "Product"},
				},
			}),
		},
		queryResp: &pb.QueryNodesResponse{},
	})

	if err := runNodesList("acme", "Product", 10, 0); err != nil {
		t.Fatalf("runNodesList: %v", err)
	}
	if svc.queryReq.GetTypeId() != 202 {
		t.Errorf("expected type_id resolved to 202, got %d", svc.queryReq.GetTypeId())
	}
}

func TestNodesGet_FoundJSON(t *testing.T) {
	resetFlags()
	out, _, restore := captureOutput()
	defer restore()
	svc := startFake(t, &fakeCLIServer{
		getNodeResp: &pb.GetNodeResponse{
			Found: true,
			Node:  &pb.Node{NodeId: "n-1", TypeId: 201, Payload: mustStruct(t, map[string]any{"1": "X"})},
		},
	})

	if err := runNodesGet("acme", "n-1", "201"); err != nil {
		t.Fatalf("runNodesGet: %v", err)
	}
	if svc.getNodeReq.GetNodeId() != "n-1" {
		t.Errorf("server saw node_id = %q", svc.getNodeReq.GetNodeId())
	}
	if !strings.Contains(out.String(), "\"node_id\"") || !strings.Contains(out.String(), "\"n-1\"") {
		t.Errorf("expected JSON node_id field; got %s", out.String())
	}
}

func TestNodesGet_NotFoundIsError(t *testing.T) {
	resetFlags()
	_, _, restore := captureOutput()
	defer restore()
	startFake(t, &fakeCLIServer{getNodeResp: &pb.GetNodeResponse{Found: false}})

	err := runNodesGet("acme", "missing", "201")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v", err)
	}
}
