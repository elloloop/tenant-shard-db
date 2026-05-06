package main

import (
	"strings"
	"testing"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

func TestEdges_OutDirection(t *testing.T) {
	resetFlags()
	out, _, restore := captureOutput()
	defer restore()
	svc := startFake(t, &fakeCLIServer{
		edgesFromRsp: &pb.GetEdgesResponse{
			Edges: []*pb.Edge{
				{FromNodeId: "u-1", ToNodeId: "n-1", EdgeTypeId: 301, CreatedAt: 1_700_000_000_000},
			},
		},
	})

	if err := runEdges("acme", "u-1", "out", "301", 50); err != nil {
		t.Fatalf("runEdges: %v", err)
	}
	if svc.edgesFromReq == nil {
		t.Fatal("server didn't see GetEdgesFrom")
	}
	if svc.edgesFromReq.GetNodeId() != "u-1" || svc.edgesFromReq.GetEdgeTypeId() != 301 {
		t.Errorf("request mismatch: %+v", svc.edgesFromReq)
	}
	got := out.String()
	for _, want := range []string{"u-1", "n-1", "301"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q: %s", want, got)
		}
	}
}

func TestEdges_InDirection(t *testing.T) {
	resetFlags()
	_, _, restore := captureOutput()
	defer restore()
	svc := startFake(t, &fakeCLIServer{
		edgesToRsp: &pb.GetEdgesResponse{
			Edges: []*pb.Edge{{FromNodeId: "x", ToNodeId: "n-1", EdgeTypeId: 302}},
		},
	})

	if err := runEdges("acme", "n-1", "in", "302", 50); err != nil {
		t.Fatalf("runEdges: %v", err)
	}
	if svc.edgesToReq == nil {
		t.Fatal("server didn't see GetEdgesTo")
	}
	if svc.edgesFromReq != nil {
		t.Fatal("server saw GetEdgesFrom for an --in request")
	}
	if svc.edgesToReq.GetNodeId() != "n-1" || svc.edgesToReq.GetEdgeTypeId() != 302 {
		t.Errorf("request mismatch: %+v", svc.edgesToReq)
	}
}
