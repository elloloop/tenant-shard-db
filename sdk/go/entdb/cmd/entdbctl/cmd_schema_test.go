package main

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

func mustStruct(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatalf("structpb.NewStruct: %v", err)
	}
	return s
}

func TestSchemaShow_Table(t *testing.T) {
	resetFlags()
	out, _, restore := captureOutput()
	defer restore()
	schema := mustStruct(t, map[string]any{
		"node_types": []any{
			map[string]any{"type_id": float64(201), "name": "Product"},
			map[string]any{"type_id": float64(202), "name": "User"},
		},
		"edge_types": []any{
			map[string]any{"edge_id": float64(301), "name": "OWNS"},
		},
	})
	svc := startFake(t, &fakeCLIServer{
		schemaResp: &pb.GetSchemaResponse{
			Fingerprint: "abc123",
			Schema:      schema,
		},
	})

	if err := runSchemaShow("acme"); err != nil {
		t.Fatalf("runSchemaShow: %v", err)
	}
	if svc.schemaReq.GetTenantId() != "acme" {
		t.Errorf("server saw tenant_id = %q", svc.schemaReq.GetTenantId())
	}
	got := out.String()
	for _, want := range []string{"abc123", "Product", "201", "User", "OWNS", "301", "node", "edge"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q: %s", want, got)
		}
	}
}
