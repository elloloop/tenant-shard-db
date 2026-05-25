// SPDX-License-Identifier: MIT
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
// a “FileDescriptorSet“ suitable for “ExtractSchemaJSON“. This
// is the same format “protoc --descriptor_set_out=FILE“ writes.
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
	// Product (201) + OAuthIdentity (202).
	if len(nodes) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(nodes))
	}
	prod := nodes[0].(map[string]any) // sorted by type_id, Product (201) first
	// NAME-FREE (ADR-031): no type name is emitted — identity is type_id.
	if _, ok := prod["name"]; ok {
		t.Errorf("node carried a name key %v; ADR-031 schema JSON is name-free", prod["name"])
	}
	if int(prod["type_id"].(float64)) != 201 {
		t.Errorf("type_id = %v, want 201", prod["type_id"])
	}
	// All emitted type_ids are the annotated ones; NotAnEntity (no
	// (entdb.node)) must be absent. Name-free, so assert by id set.
	ids := map[int]bool{}
	for _, n := range nodes {
		ids[int(n.(map[string]any)["type_id"].(float64))] = true
	}
	if !ids[201] || !ids[202] {
		t.Errorf("want type_ids {201,202}, got %v", ids)
	}

	edges := schema["edge_types"].([]any)
	if len(edges) != 1 {
		t.Fatalf("want 1 edge, got %d", len(edges))
	}
	edge := edges[0].(map[string]any)
	if _, ok := edge["name"]; ok {
		t.Errorf("edge carried a name key; ADR-031 schema JSON is name-free")
	}
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

	// NAME-FREE (ADR-031): fields are keyed by field_id, never by name.
	byID := map[int]map[string]any{}
	for _, f := range fields {
		fm := f.(map[string]any)
		if _, ok := fm["name"]; ok {
			t.Errorf("field carried a name key; ADR-031 schema JSON is name-free")
		}
		byID[int(fm["field_id"].(float64))] = fm
	}

	// field 1 (sku): TYPE_STRING → kind "str"; (entdb.field).unique=true
	sku := byID[1]
	if sku["kind"] != "str" {
		t.Errorf("field 1.kind = %q, want str", sku["kind"])
	}
	if sku["unique"] != true {
		t.Errorf("field 1.unique = %v, want true", sku["unique"])
	}

	// field 2 (name): TYPE_STRING, no annotations → kind str, no unique
	name := byID[2]
	if name["kind"] != "str" {
		t.Errorf("field 2.kind = %q, want str", name["kind"])
	}
	if _, ok := name["unique"]; ok {
		t.Error("field 2.unique should be omitted when not set")
	}

	// field 3 (price_cents): TYPE_INT64 → kind "int"
	price := byID[3]
	if price["kind"] != "int" {
		t.Errorf("field 3.kind = %q, want int", price["kind"])
	}
}

// ── Composite unique extraction (ADR-030 / issue #566) ──────────────

func TestExtractSchemaJSON_ExtractsCompositeUnique(t *testing.T) {
	out, err := ExtractSchemaJSON(fdsFromTestPB(t))
	if err != nil {
		t.Fatalf("ExtractSchemaJSON: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	nodes := env["schema"].(map[string]any)["node_types"].([]any)

	// NAME-FREE (ADR-031): find OAuthIdentity (type_id 202) by id.
	var oauth map[string]any
	for _, n := range nodes {
		nm := n.(map[string]any)
		if int(nm["type_id"].(float64)) == 202 {
			oauth = nm
		}
	}
	if oauth == nil {
		t.Fatalf("OAuthIdentity (type_id 202) not extracted; nodes=%v", nodes)
	}

	cuRaw, ok := oauth["composite_unique"]
	if !ok {
		t.Fatalf("composite_unique missing on OAuthIdentity: %v", oauth)
	}
	cu := cuRaw.([]any)
	if len(cu) != 1 {
		t.Fatalf("want 1 composite constraint, got %d: %v", len(cu), cu)
	}
	c := cu[0].(map[string]any)
	// NAME-FREE (ADR-031): a composite constraint is identified solely by
	// its field_ids tuple — no constraint name is emitted.
	if _, ok := c["name"]; ok {
		t.Errorf("composite_unique carried a name key %v; ADR-031 is name-free", c["name"])
	}
	ids := c["field_ids"].([]any)
	if len(ids) != 2 || int(ids[0].(float64)) != 1 || int(ids[1].(float64)) != 2 {
		t.Errorf("field_ids = %v, want [1 2] (resolved from proto field names)", ids)
	}

	// Product (type_id 201, no composite_unique) must NOT carry the key.
	for _, n := range nodes {
		nm := n.(map[string]any)
		if int(nm["type_id"].(float64)) == 201 {
			if _, ok := nm["composite_unique"]; ok {
				t.Error("Product (no composite_unique) should omit the key")
			}
		}
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

	// Both files share the same node/edge ids. They land twice in the
	// output (the SDK doesn't deduplicate; that's the customer's
	// problem if their proto packages collide). What we verify here is
	// that walking ranges over BOTH files rather than stopping at the
	// first. Each file carries two node types (Product + OAuthIdentity)
	// and one edge type (PurchaseEdge).
	nodes := env["schema"].(map[string]any)["node_types"].([]any)
	edges := env["schema"].(map[string]any)["edge_types"].([]any)
	if len(nodes) != 4 {
		t.Errorf("want 4 nodes (two per file), got %d", len(nodes))
	}
	if len(edges) != 2 {
		t.Errorf("want 2 edges (one per file), got %d", len(edges))
	}
}
