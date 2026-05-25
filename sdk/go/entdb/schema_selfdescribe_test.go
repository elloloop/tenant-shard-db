// SPDX-License-Identifier: MIT
package entdb

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/testpb"
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
