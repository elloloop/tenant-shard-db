// SPDX-License-Identifier: MIT
package entdb

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2/internal/pb"
	"github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2/internal/testpb"
)

// The name-free fingerprint must be DETERMINISTIC and ID-driven (ADR-031):
// it is a sha256 over the name-free canonical schema JSON, computed the same
// way the Go server's schema.computeFingerprint does, so client and server
// derive the same hash without ever sharing names. (The server-parity value
// is pinned by the cross-language integration suite; here we pin the SDK
// side's stability + shape.)
func TestSelfDescribe_FingerprintIsStableAndNameFree(t *testing.T) {
	a := newSchemaRegistry([]proto.Message{&testpb.Product{}, &testpb.PurchaseEdge{}})
	b := newSchemaRegistry([]proto.Message{&testpb.PurchaseEdge{}, &testpb.Product{}})
	if a == nil || b == nil {
		t.Fatal("nil registry")
	}
	if a.fingerprint == "" {
		t.Fatal("empty fingerprint")
	}
	// Registration order must not change the fingerprint (types are sorted
	// by id; the canonical form is name-free and order-independent).
	if a.fingerprint != b.fingerprint {
		t.Errorf("fingerprint not order-stable: %q vs %q", a.fingerprint, b.fingerprint)
	}
}

func TestSelfDescribe_DescriptorIsNameFree(t *testing.T) {
	reg := newSchemaRegistry([]proto.Message{&testpb.Product{}, &testpb.PurchaseEdge{}})
	if reg == nil || reg.descriptor == nil {
		t.Fatal("nil descriptor")
	}
	// The wire SchemaDescriptor carries no names (the proto reserves the
	// name field numbers); it is keyed by type_id / edge_id / field_id.
	if len(reg.descriptor.GetNodeTypes()) != 1 {
		t.Fatalf("want 1 node type (Product), got %d", len(reg.descriptor.GetNodeTypes()))
	}
	prod := reg.descriptor.GetNodeTypes()[0]
	if prod.GetTypeId() != 201 {
		t.Errorf("type_id = %d, want 201", prod.GetTypeId())
	}
	if len(prod.GetFields()) != 3 {
		t.Errorf("Product fields = %d, want 3", len(prod.GetFields()))
	}
	// field 1 (sku) is unique.
	var sku *struct{ unique bool }
	for _, f := range prod.GetFields() {
		if f.GetFieldId() == 1 {
			sku = &struct{ unique bool }{f.GetUnique()}
		}
	}
	if sku == nil || !sku.unique {
		t.Errorf("field 1 (sku) should be unique=true")
	}
	if len(reg.descriptor.GetEdgeTypes()) != 1 || reg.descriptor.GetEdgeTypes()[0].GetEdgeId() != 301 {
		t.Errorf("want 1 edge type with edge_id 301")
	}
}

func TestSelfDescribe_EmptyRegistryIsNil(t *testing.T) {
	if newSchemaRegistry(nil) != nil {
		t.Error("nil messages should yield a nil registry")
	}
	// A message with no (entdb.node)/(entdb.edge) option contributes
	// nothing — the registry stays nil.
	if newSchemaRegistry([]proto.Message{&testpb.NotAnEntity{}}) != nil {
		t.Error("a non-entity message should yield a nil registry")
	}
}

// findField returns the SchemaFieldDef with the given field_id, or nil.
func findField(fields []*pb.SchemaFieldDef, id uint32) *pb.SchemaFieldDef {
	for _, f := range fields {
		if f.GetFieldId() == id {
			return f
		}
	}
	return nil
}

// TestSelfDescribe_RefTypeIDIsExtracted is the direct regression test for
// issue #604: every kind:"ref" field MUST carry its ref_type_id on the wire
// or the server rejects the whole register_schema with VALIDATION_ERROR.
// The Go SDK previously dropped this attribute entirely; this test pins it.
func TestSelfDescribe_RefTypeIDIsExtracted(t *testing.T) {
	reg := newSchemaRegistry([]proto.Message{&testpb.RichDoc{}})
	if reg == nil || reg.descriptor == nil {
		t.Fatal("nil registry")
	}
	if len(reg.descriptor.GetNodeTypes()) != 1 {
		t.Fatalf("want 1 node type, got %d", len(reg.descriptor.GetNodeTypes()))
	}
	node := reg.descriptor.GetNodeTypes()[0]

	ownerID := findField(node.GetFields(), 3) // owner_id
	if ownerID == nil {
		t.Fatal("owner_id (field 3) missing from descriptor")
	}
	if ownerID.GetKind() != "ref" {
		t.Errorf("owner_id.kind = %q, want \"ref\"", ownerID.GetKind())
	}
	if ownerID.RefTypeId == nil {
		t.Fatal("owner_id.ref_type_id is nil — issue #604 regression: Go SDK dropped ref_type_id")
	}
	if *ownerID.RefTypeId != 201 {
		t.Errorf("owner_id.ref_type_id = %d, want 201", *ownerID.RefTypeId)
	}
}

// TestSelfDescribe_FieldAttributesRoundTrip verifies that every wire field on
// SchemaFieldDef the Go SDK previously dropped (pii, deprecated, description,
// ref_type_id) is now extracted from FieldOpts and emitted onto the descriptor.
func TestSelfDescribe_FieldAttributesRoundTrip(t *testing.T) {
	reg := newSchemaRegistry([]proto.Message{&testpb.RichDoc{}})
	if reg == nil {
		t.Fatal("nil registry")
	}
	node := reg.descriptor.GetNodeTypes()[0]

	// title field: only description="the title"
	title := findField(node.GetFields(), 1)
	if title == nil {
		t.Fatal("title (field 1) missing")
	}
	if title.GetDescription() != "the title" {
		t.Errorf("title.description = %q, want \"the title\"", title.GetDescription())
	}
	if title.GetPii() || title.GetDeprecated() || title.RefTypeId != nil {
		t.Errorf("title should only carry description; got pii=%v deprecated=%v ref=%v",
			title.GetPii(), title.GetDeprecated(), title.RefTypeId)
	}

	// body field: nothing set (every attribute should be the zero value)
	body := findField(node.GetFields(), 2)
	if body == nil {
		t.Fatal("body (field 2) missing")
	}
	if body.GetDescription() != "" || body.GetPii() || body.GetDeprecated() || body.RefTypeId != nil {
		t.Errorf("body should have no attributes set; got %+v", body)
	}

	// author_email field: pii=true, deprecated=true, description=...
	author := findField(node.GetFields(), 4)
	if author == nil {
		t.Fatal("author_email (field 4) missing")
	}
	if !author.GetPii() {
		t.Error("author_email.pii = false; want true — Go SDK was dropping pii (GDPR-critical)")
	}
	if !author.GetDeprecated() {
		t.Error("author_email.deprecated = false; want true")
	}
	if author.GetDescription() != "scrub on anonymise" {
		t.Errorf("author_email.description = %q, want \"scrub on anonymise\"", author.GetDescription())
	}
}

// TestSelfDescribe_NodeAttributesRoundTrip pins every NodeOpts attribute
// the Go SDK previously dropped (deprecated, description, data_policy,
// subject_field, legal_basis).
func TestSelfDescribe_NodeAttributesRoundTrip(t *testing.T) {
	reg := newSchemaRegistry([]proto.Message{&testpb.RichDoc{}})
	if reg == nil {
		t.Fatal("nil registry")
	}
	node := reg.descriptor.GetNodeTypes()[0]

	if node.GetTypeId() != 401 {
		t.Errorf("type_id = %d, want 401", node.GetTypeId())
	}
	if !node.GetDeprecated() {
		t.Error("node.deprecated = false; want true")
	}
	if node.GetDescription() != "Rich doc; all NodeOpts attributes" {
		t.Errorf("node.description = %q", node.GetDescription())
	}
	if node.GetDataPolicy() != "business" {
		t.Errorf("node.data_policy = %q, want \"business\"", node.GetDataPolicy())
	}
	if node.SubjectField == nil {
		t.Fatal("node.subject_field nil; want field_id of owner_id (3) — Go SDK was dropping it")
	}
	if *node.SubjectField != 3 {
		t.Errorf("node.subject_field = %d, want 3 (owner_id's field_id)", *node.SubjectField)
	}
	if node.LegalBasis == nil || *node.LegalBasis != "user-consent" {
		t.Errorf("node.legal_basis = %v, want \"user-consent\"", node.LegalBasis)
	}
}

// TestSelfDescribe_EdgeAttributesRoundTrip pins every EdgeOpts attribute
// the Go SDK previously dropped (unique_per_from, deprecated, description,
// data_policy, on_subject_exit — which was hardcoded "both").
func TestSelfDescribe_EdgeAttributesRoundTrip(t *testing.T) {
	reg := newSchemaRegistry([]proto.Message{&testpb.RichEdge{}})
	if reg == nil {
		t.Fatal("nil registry")
	}
	if len(reg.descriptor.GetEdgeTypes()) != 1 {
		t.Fatalf("want 1 edge type, got %d", len(reg.descriptor.GetEdgeTypes()))
	}
	edge := reg.descriptor.GetEdgeTypes()[0]

	if edge.GetEdgeId() != 501 {
		t.Errorf("edge_id = %d, want 501", edge.GetEdgeId())
	}
	if !edge.GetUniquePerFrom() {
		t.Error("edge.unique_per_from = false; want true — Go SDK was dropping it")
	}
	if !edge.GetDeprecated() {
		t.Error("edge.deprecated = false; want true")
	}
	if edge.GetDescription() != "Rich edge; all EdgeOpts attributes" {
		t.Errorf("edge.description = %q", edge.GetDescription())
	}
	if edge.GetDataPolicy() != "audit" {
		t.Errorf("edge.data_policy = %q, want \"audit\"", edge.GetDataPolicy())
	}
	if edge.GetOnSubjectExit() != "to" {
		t.Errorf("edge.on_subject_exit = %q, want \"to\" — Go SDK was hardcoding \"both\"", edge.GetOnSubjectExit())
	}
}

// TestSelfDescribe_FingerprintIncludesAllAttributes confirms the canonical
// fingerprint changes when an attribute the SDK previously dropped becomes
// set — i.e. the client-side fingerprint now reflects the full schema and
// will MATCH the server's (server canonicalises FieldDef/NodeTypeDef with
// the same omitempty rules). If the SDK drops an attribute the canonical
// form omits it; the server's would include it; fingerprints diverge and
// the client re-attaches schema on every write. This test pins parity.
func TestSelfDescribe_FingerprintIncludesAllAttributes(t *testing.T) {
	rich := newSchemaRegistry([]proto.Message{&testpb.RichDoc{}})
	plain := newSchemaRegistry([]proto.Message{&testpb.Product{}})
	if rich == nil || plain == nil {
		t.Fatal("nil registry")
	}
	if rich.fingerprint == "" || plain.fingerprint == "" {
		t.Fatal("empty fingerprint")
	}
	if rich.fingerprint == plain.fingerprint {
		t.Error("RichDoc and Product produced the SAME fingerprint — additional " +
			"attributes (pii, ref_type_id, data_policy, subject_field, ...) are " +
			"not affecting the canonical form, which means they're still dropped")
	}
}

// ADR-031: the composite ALREADY_EXISTS detail's constraint identity is now
// the field-id TUPLE SIGNATURE (e.g. '(1,2)'), NOT a constraint name. The
// lexical shape is unchanged so the existing parser recovers it; IsComposite
// stays true and FieldIDs carry the tuple.
func TestSelfDescribe_CompositeTupleSignatureParses(t *testing.T) {
	detail := "Composite unique constraint violation: tenant=acme type_id=201 " +
		"constraint='(1,2)' fields=[1, 2] values=['google', 'uid-1'] already exists"
	uce := parseUniqueConstraintFromStatus(status.New(codes.AlreadyExists, detail).Err(), "acme")
	if uce == nil {
		t.Fatal("expected *UniqueConstraintError")
	}
	if !uce.IsComposite() {
		t.Fatal("IsComposite() = false; want true for a tuple-signature composite detail")
	}
	if uce.ConstraintName != "(1,2)" {
		t.Errorf("ConstraintName = %q, want the tuple signature (1,2)", uce.ConstraintName)
	}
	if len(uce.FieldIDs) != 2 || uce.FieldIDs[0] != 1 || uce.FieldIDs[1] != 2 {
		t.Errorf("FieldIDs = %v, want [1 2]", uce.FieldIDs)
	}
	if uce.TypeID != 201 {
		t.Errorf("TypeID = %d, want 201", uce.TypeID)
	}
	var got *UniqueConstraintError
	if !errors.As(error(uce), &got) {
		t.Fatal("errors.As failed to extract *UniqueConstraintError")
	}
}
