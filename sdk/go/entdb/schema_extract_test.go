package entdb

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/testpb"
)

// fdsFromTestPB wraps the in-package testpb file descriptor into
// a ``FileDescriptorSet`` suitable for ``ExtractSchemaJSON``. This
// is the same format ``protoc --descriptor_set_out=FILE`` writes.
func fdsFromTestPB(t *testing.T) *descriptorpb.FileDescriptorSet {
	t.Helper()
	fp := testpb.ProductDesc.ParentFile()
	fdp := protodesc.ToFileDescriptorProto(fp)
	return &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fdp},
	}
}

// ── Top-level shape ─────────────────────────────────────────────────

func TestExtractSchemaJSON_NilSetIsRejected(t *testing.T) {
	if _, err := ExtractSchemaJSON(nil); err == nil {
		t.Fatal("expected error for nil descriptor set")
	}
}

func TestExtractSchemaJSON_EmptySetReturnsEmptyTypes(t *testing.T) {
	out, err := ExtractSchemaJSON(&descriptorpb.FileDescriptorSet{})
	if err != nil {
		t.Fatalf("ExtractSchemaJSON: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	schema := env["schema"].(map[string]any)
	if got := len(schema["node_types"].([]any)); got != 0 {
		t.Errorf("want 0 node_types, got %d", got)
	}
	if got := len(schema["edge_types"].([]any)); got != 0 {
		t.Errorf("want 0 edge_types, got %d", got)
	}
}

// ── Node + edge extraction from the testpb file ─────────────────────

func TestExtractSchemaJSON_ExtractsNodesAndEdgesFromTestPB(t *testing.T) {
	out, err := ExtractSchemaJSON(fdsFromTestPB(t))
	if err != nil {
		t.Fatalf("ExtractSchemaJSON: %v", err)
	}

	var env map[string]any
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	schema := env["schema"].(map[string]any)

	nodes := schema["node_types"].([]any)
	if len(nodes) != 1 {
		t.Fatalf("want 1 node, got %d", len(nodes))
	}
	prod := nodes[0].(map[string]any)
	if prod["name"] != "Product" {
		t.Errorf("name = %q, want Product", prod["name"])
	}
	if int(prod["type_id"].(float64)) != 201 {
		t.Errorf("type_id = %v, want 201", prod["type_id"])
	}

	// NotAnEntity has no (entdb.node) — must NOT appear in node_types.
	for _, n := range nodes {
		if n.(map[string]any)["name"] == "NotAnEntity" {
			t.Error("NotAnEntity (no entdb annotation) should be skipped, but appeared in node_types")
		}
	}

	edges := schema["edge_types"].([]any)
	if len(edges) != 1 {
		t.Fatalf("want 1 edge, got %d", len(edges))
	}
	edge := edges[0].(map[string]any)
	if int(edge["edge_id"].(float64)) != 301 {
		t.Errorf("edge_id = %v, want 301", edge["edge_id"])
	}
}

// ── Field extraction: kinds + annotations ───────────────────────────

func TestExtractSchemaJSON_FieldKindsMappedFromProtoTypes(t *testing.T) {
	out, _ := ExtractSchemaJSON(fdsFromTestPB(t))
	var env map[string]any
	_ = json.Unmarshal(out, &env)
	prod := env["schema"].(map[string]any)["node_types"].([]any)[0].(map[string]any)

	fields := prod["fields"].([]any)
	if len(fields) != 3 {
		t.Fatalf("want 3 fields on Product, got %d", len(fields))
	}

	byName := map[string]map[string]any{}
	for _, f := range fields {
		fm := f.(map[string]any)
		byName[fm["name"].(string)] = fm
	}

	// sku: TYPE_STRING → kind "str"; (entdb.field).unique=true → unique:true
	sku := byName["sku"]
	if sku["kind"] != "str" {
		t.Errorf("sku.kind = %q, want str", sku["kind"])
	}
	if sku["unique"] != true {
		t.Errorf("sku.unique = %v, want true", sku["unique"])
	}
	if int(sku["field_id"].(float64)) != 1 {
		t.Errorf("sku.field_id = %v, want 1", sku["field_id"])
	}

	// name: TYPE_STRING, no annotations → kind str, no unique key emitted
	name := byName["name"]
	if name["kind"] != "str" {
		t.Errorf("name.kind = %q, want str", name["kind"])
	}
	if _, ok := name["unique"]; ok {
		t.Error("name.unique should be omitted when not set")
	}

	// price_cents: TYPE_INT64 → kind "int"
	price := byName["price_cents"]
	if price["kind"] != "int" {
		t.Errorf("price_cents.kind = %q, want int", price["kind"])
	}
}

// ── Output is deterministic (stable order across runs) ──────────────

func TestExtractSchemaJSON_OutputIsDeterministic(t *testing.T) {
	fds := fdsFromTestPB(t)
	a, err := ExtractSchemaJSON(fds)
	if err != nil {
		t.Fatalf("ExtractSchemaJSON: %v", err)
	}
	b, err := ExtractSchemaJSON(fds)
	if err != nil {
		t.Fatalf("ExtractSchemaJSON: %v", err)
	}
	if string(a) != string(b) {
		t.Errorf("non-deterministic output between two calls — schema.lock.json must round-trip cleanly through git diff")
	}
}

// ── Round-trip with a multi-file descriptor set ─────────────────────

func TestExtractSchemaJSON_HandlesMultipleFilesInSet(t *testing.T) {
	// Two copies of the same file in a set should still produce
	// only one (Product, PurchaseEdge) — protodesc.NewFiles
	// rejects duplicates, so we test with two distinct files
	// (one with Product, one with a renamed copy).
	fp := testpb.ProductDesc.ParentFile()
	fdp1 := protodesc.ToFileDescriptorProto(fp)
	fdp2 := proto.Clone(fdp1).(*descriptorpb.FileDescriptorProto)
	fdp2.Name = proto.String("internal/testpb/testpb_other.proto")
	fdp2.Package = proto.String("entdb.testpb_other")

	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fdp1, fdp2},
	}

	out, err := ExtractSchemaJSON(fds)
	if err != nil {
		t.Fatalf("ExtractSchemaJSON: %v", err)
	}
	var env map[string]any
	_ = json.Unmarshal(out, &env)

	// Both files share the same type_id=201 and edge_id=301. They
	// land twice in the output (the SDK doesn't deduplicate; that's
	// the customer's problem if their proto packages collide). What
	// we verify here is that walking ranges over BOTH files rather
	// than stopping at the first.
	nodes := env["schema"].(map[string]any)["node_types"].([]any)
	edges := env["schema"].(map[string]any)["edge_types"].([]any)
	if len(nodes) != 2 {
		t.Errorf("want 2 nodes (one per file), got %d", len(nodes))
	}
	if len(edges) != 2 {
		t.Errorf("want 2 edges (one per file), got %d", len(edges))
	}
}
