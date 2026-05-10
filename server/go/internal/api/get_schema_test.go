// Tests for the GetSchema RPC. Spec: docs/go-port/rpcs/GetSchema.md.
//
// The sole contract pin from the Python test suite
// (tests/python/integration/test_grpc_contract.py:208) requires either
// fingerprint != "" or HasField("schema"). The cases below cover both
// the empty-registry path (Struct populated, fingerprint empty) and the
// frozen-registry path (Struct populated AND fingerprint set), plus the
// optional type_id filter and the degraded swallow-errors path.

package api

import (
	"context"
	"strings"
	"testing"

	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
)

// TestGetSchema_NoRegistry covers Server constructed without
// WithSchemaRegistry — should return OK with an empty Struct and empty
// fingerprint, matching Python's behaviour when SchemaRegistry is empty
// and tenant_id is also empty.
func TestGetSchema_NoRegistry(t *testing.T) {
	t.Parallel()
	srv := New()

	resp, err := srv.GetSchema(context.Background(), &pb.GetSchemaRequest{})
	if err != nil {
		t.Fatalf("GetSchema returned error: %v", err)
	}
	if resp == nil {
		t.Fatalf("GetSchema returned nil response")
	}
	if resp.GetFingerprint() != "" {
		t.Errorf("Fingerprint: want \"\", got %q", resp.GetFingerprint())
	}
	if resp.GetSchema() == nil {
		t.Fatalf("Schema struct: want non-nil, got nil")
	}
	fields := resp.GetSchema().GetFields()
	nodeTypes, ok := fields["node_types"]
	if !ok {
		t.Errorf("Schema struct missing key node_types; keys=%v", keysOf(fields))
	} else if lv := nodeTypes.GetListValue(); lv == nil {
		t.Errorf("node_types: want list value, got %v", nodeTypes)
	} else if n := len(lv.GetValues()); n != 0 {
		t.Errorf("node_types: want 0 entries, got %d", n)
	}
	if _, ok := fields["edge_types"]; !ok {
		t.Errorf("Schema struct missing key edge_types; keys=%v", keysOf(fields))
	}
}

// TestGetSchema_EmptyRegistry covers Server with an explicit empty
// (but possibly frozen) Registry. Both an unfrozen-empty and a
// frozen-empty registry must round-trip an empty schema. The frozen
// case sets fingerprint to a non-empty sha256:... string.
func TestGetSchema_EmptyRegistry(t *testing.T) {
	t.Parallel()
	reg := schema.NewRegistry()
	srv := New(WithSchemaRegistry(reg))

	resp, err := srv.GetSchema(context.Background(), &pb.GetSchemaRequest{})
	if err != nil {
		t.Fatalf("GetSchema returned error: %v", err)
	}
	if got := resp.GetFingerprint(); got != "" {
		t.Errorf("Fingerprint (unfrozen): want \"\", got %q", got)
	}
	if s := resp.GetSchema(); s == nil {
		t.Fatalf("Schema struct: want non-nil, got nil")
	} else {
		if lv, ok := s.GetFields()["node_types"]; !ok || lv.GetListValue() == nil ||
			len(lv.GetListValue().GetValues()) != 0 {
			t.Errorf("node_types: want empty list, got %v", lv)
		}
		if lv, ok := s.GetFields()["edge_types"]; !ok || lv.GetListValue() == nil ||
			len(lv.GetListValue().GetValues()) != 0 {
			t.Errorf("edge_types: want empty list, got %v", lv)
		}
	}

	// Freeze and re-run: fingerprint must populate.
	fp, ferr := reg.Freeze()
	if ferr != nil {
		t.Fatalf("Freeze: %v", ferr)
	}
	if !strings.HasPrefix(fp, "sha256:") {
		t.Errorf("Freeze fingerprint: want sha256: prefix, got %q", fp)
	}
	resp2, err := srv.GetSchema(context.Background(), &pb.GetSchemaRequest{})
	if err != nil {
		t.Fatalf("GetSchema after Freeze returned error: %v", err)
	}
	if resp2.GetFingerprint() != fp {
		t.Errorf("Fingerprint (frozen): want %q, got %q", fp, resp2.GetFingerprint())
	}
}

// TestGetSchema_PopulatedRegistry pins the round-trip of a real
// registry. The contract pin only requires fingerprint!=""||schema; we
// additionally verify the Struct contents match the registered types so
// the SDK CLI consumer (sdk/go/entdb/cmd/entdbctl/cmd_schema_test.go)
// continues to deserialise without surprises.
func TestGetSchema_PopulatedRegistry(t *testing.T) {
	t.Parallel()
	reg := schema.NewRegistry()
	if err := reg.RegisterNode(&schema.NodeTypeDef{
		TypeID: 1,
		Name:   "User",
		Fields: []schema.FieldDef{
			{FieldID: 1, Name: "email", Kind: schema.KindString, Required: true},
			{FieldID: 2, Name: "display_name", Kind: schema.KindString},
		},
	}); err != nil {
		t.Fatalf("RegisterNode User: %v", err)
	}
	if err := reg.RegisterNode(&schema.NodeTypeDef{
		TypeID: 2,
		Name:   "Task",
		Fields: []schema.FieldDef{
			{FieldID: 1, Name: "title", Kind: schema.KindString, Required: true},
		},
	}); err != nil {
		t.Fatalf("RegisterNode Task: %v", err)
	}
	if err := reg.RegisterEdge(&schema.EdgeTypeDef{
		EdgeID:        10,
		Name:          "AssignedTo",
		FromTypeID:    2,
		ToTypeID:      1,
		OnSubjectExit: schema.OnSubjectExitBoth,
	}); err != nil {
		t.Fatalf("RegisterEdge: %v", err)
	}
	fp, err := reg.Freeze()
	if err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	srv := New(WithSchemaRegistry(reg))

	// Unfiltered: every node + edge present.
	resp, err := srv.GetSchema(context.Background(), &pb.GetSchemaRequest{})
	if err != nil {
		t.Fatalf("GetSchema: %v", err)
	}
	if resp.GetFingerprint() != fp {
		t.Errorf("Fingerprint: want %q, got %q", fp, resp.GetFingerprint())
	}
	nodes := listLen(t, resp, "node_types")
	if nodes != 2 {
		t.Errorf("node_types: want 2, got %d", nodes)
	}
	edges := listLen(t, resp, "edge_types")
	if edges != 1 {
		t.Errorf("edge_types: want 1, got %d", edges)
	}

	// Filtered by type_id=1 (User). User node stays; AssignedTo edge
	// stays (to_type_id == 1); Task node drops.
	resp2, err := srv.GetSchema(context.Background(), &pb.GetSchemaRequest{TypeId: 1})
	if err != nil {
		t.Fatalf("GetSchema TypeId=1: %v", err)
	}
	if got := listLen(t, resp2, "node_types"); got != 1 {
		t.Errorf("filtered node_types: want 1, got %d", got)
	}
	if got := listLen(t, resp2, "edge_types"); got != 1 {
		t.Errorf("filtered edge_types: want 1 (touches type 1), got %d", got)
	}

	// Filtered by type_id=99 (unknown). No nodes, no edges.
	resp3, err := srv.GetSchema(context.Background(), &pb.GetSchemaRequest{TypeId: 99})
	if err != nil {
		t.Fatalf("GetSchema TypeId=99: %v", err)
	}
	if got := listLen(t, resp3, "node_types"); got != 0 {
		t.Errorf("filtered node_types (no match): want 0, got %d", got)
	}
	if got := listLen(t, resp3, "edge_types"); got != 0 {
		t.Errorf("filtered edge_types (no match): want 0, got %d", got)
	}

	// Fingerprint is constant across filtered/unfiltered responses
	// (the Python handler computes it from the unfiltered registry).
	if resp2.GetFingerprint() != fp || resp3.GetFingerprint() != fp {
		t.Errorf("fingerprint changed under filter: %q / %q / %q",
			resp.GetFingerprint(), resp2.GetFingerprint(), resp3.GetFingerprint())
	}
}

// TestGetSchema_ContractPin asserts the sole behavioural pin from the
// Python contract test (test_grpc_contract.py:208): the response
// satisfies `fingerprint != "" || HasField("schema")` for both the
// happy path and the degraded empty-registry path.
func TestGetSchema_ContractPin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		build func() *Server
	}{
		{
			name:  "no_registry",
			build: func() *Server { return New() },
		},
		{
			name: "empty_registry",
			build: func() *Server {
				return New(WithSchemaRegistry(schema.NewRegistry()))
			},
		},
		{
			name: "frozen_registry",
			build: func() *Server {
				reg := schema.NewRegistry()
				_ = reg.RegisterNode(&schema.NodeTypeDef{
					TypeID: 1, Name: "X",
					Fields: []schema.FieldDef{{FieldID: 1, Name: "a", Kind: schema.KindString}},
				})
				if _, err := reg.Freeze(); err != nil {
					t.Fatalf("Freeze: %v", err)
				}
				return New(WithSchemaRegistry(reg))
			},
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			srv := c.build()
			resp, err := srv.GetSchema(context.Background(), &pb.GetSchemaRequest{
				TenantId: "tenant-a",
			})
			if err != nil {
				t.Fatalf("GetSchema: %v", err)
			}
			if resp.GetFingerprint() == "" && resp.GetSchema() == nil {
				t.Fatalf("contract pin violated: fingerprint=%q schema=%v",
					resp.GetFingerprint(), resp.GetSchema())
			}
		})
	}
}

// listLen returns the number of entries under fields[key] when it's a
// ListValue. Fails the test if absent or wrong shape.
func listLen(t *testing.T, resp *pb.GetSchemaResponse, key string) int {
	t.Helper()
	s := resp.GetSchema()
	if s == nil {
		t.Fatalf("schema is nil")
	}
	v, ok := s.GetFields()[key]
	if !ok {
		t.Fatalf("schema missing key %q; have %v", key, keysOf(s.GetFields()))
	}
	lv := v.GetListValue()
	if lv == nil {
		t.Fatalf("schema[%q] is not a ListValue: %v", key, v)
	}
	return len(lv.GetValues())
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
