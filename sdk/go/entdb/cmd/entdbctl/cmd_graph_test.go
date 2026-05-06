package main

import (
	"strings"
	"testing"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

func TestGraph_Table(t *testing.T) {
	resetFlags()
	out, _, restore := captureOutput()
	defer restore()
	svc := startFake(t, &fakeCLIServer{
		connResp: &pb.GetConnectedNodesResponse{
			Nodes: []*pb.Node{
				{NodeId: "n-2", TypeId: 201, OwnerActor: "user:bob"},
			},
			HasMore: true,
		},
	})

	if err := runGraph("acme", "n-1", "", 25); err != nil {
		t.Fatalf("runGraph: %v", err)
	}
	if svc.connReq.GetNodeId() != "n-1" {
		t.Errorf("server saw node_id = %q", svc.connReq.GetNodeId())
	}
	if svc.connReq.GetLimit() != 25 {
		t.Errorf("limit = %d", svc.connReq.GetLimit())
	}
	got := out.String()
	if !strings.Contains(got, "n-2") || !strings.Contains(got, "user:bob") {
		t.Errorf("output missing connected node info: %s", got)
	}
	if !strings.Contains(got, "more results") {
		t.Errorf("expected has_more hint; got %s", got)
	}
}
