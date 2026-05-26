// Package testpb provides proto-annotated test messages for the
// entdb Go SDK's internal tests. The messages are built at package
// init time by programmatically constructing a FileDescriptorProto
// with “(entdb.node)“ and “(entdb.edge)“ options encoded as
// raw wire-format bytes on the MessageOptions, then passing it
// through protodesc.NewFile to get a real FileDescriptor we can
// dynamicpb.NewMessage from.
//
// Doing this in Go code (rather than running protoc during the
// test build) avoids a toolchain dependency and lets the SDK tests
// run in a bare “go test ./...“ with no side-channel codegen.
package testpb

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Product is a dynamicpb message descriptor for a test Product
// node type:
//
//	message Product {
//	    option (entdb.node) = { type_id: 201 };
//	    string sku         = 1 [(entdb.field).unique = true];
//	    string name        = 2;
//	    int64  price_cents = 3;
//	}
var ProductDesc protoreflect.MessageDescriptor

// PurchaseEdgeDesc describes a test edge type:
//
//	message PurchaseEdge {
//	    option (entdb.edge) = { edge_id: 301 };
//	}
var PurchaseEdgeDesc protoreflect.MessageDescriptor

// NotAnEntityDesc describes a message with no EntDB annotations,
// used by negative tests.
var NotAnEntityDesc protoreflect.MessageDescriptor

// RichDocDesc is a node type exercising EVERY FieldOpts + NodeOpts
// attribute the wire SchemaFieldDef / SchemaNodeTypeDef carries. It is
// the regression fixture for the gap discovered in issue #604 (and the
// 8 similar omissions the same diff fixes): without it the Go SDK's
// auto-attach silently drops ref_type_id, pii, deprecated, description,
// data_policy, subject_field, legal_basis (per node), etc.
//
//	message RichDoc {
//	    option (entdb.node) = {
//	        type_id: 401,
//	        data_policy: DATA_POLICY_BUSINESS,
//	        subject_field: "owner_id",
//	        legal_basis: "user-consent",
//	        description: "Rich doc; all NodeOpts attributes",
//	        deprecated: true,
//	    };
//	    string title        = 1 [(entdb.field).description = "the title"];
//	    string body         = 2;
//	    string owner_id     = 3 [(entdb.field) = { kind: "ref", ref_type_id: 201 }];
//	    string author_email = 4 [(entdb.field) = { pii: true, deprecated: true, description: "scrub on anonymise" }];
//	}
var RichDocDesc protoreflect.MessageDescriptor

// RichEdgeDesc is an edge type exercising EVERY EdgeOpts attribute the
// wire SchemaEdgeTypeDef carries (excluding from_type_id/to_type_id,
// which are derived from the proto's from/to field types — a separate
// fix tracked outside this regression set).
//
//	message RichEdge {
//	    option (entdb.edge) = {
//	        edge_id: 501,
//	        unique_per_from: true,
//	        data_policy: DATA_POLICY_AUDIT,
//	        on_subject_exit: SUBJECT_EXIT_TO,
//	        description: "Rich edge; all EdgeOpts attributes",
//	        deprecated: true,
//	    };
//	}
var RichEdgeDesc protoreflect.MessageDescriptor

// OAuthIdentityDesc is a node type carrying a composite unique
// constraint (ADR-030 / issue #566):
//
//	message OAuthIdentity {
//	    option (entdb.node) = {
//	        type_id: 202
//	        composite_unique: { name: "provider_user_id"
//	                            fields: ["provider", "provider_user_id"] }
//	    };
//	    string provider         = 1;
//	    string provider_user_id = 2;
//	}
var OAuthIdentityDesc protoreflect.MessageDescriptor

func init() {
	fd := buildFile()
	ProductDesc = fd.Messages().ByName("Product")
	PurchaseEdgeDesc = fd.Messages().ByName("PurchaseEdge")
	NotAnEntityDesc = fd.Messages().ByName("NotAnEntity")
	OAuthIdentityDesc = fd.Messages().ByName("OAuthIdentity")
	RichDocDesc = fd.Messages().ByName("RichDoc")
	RichEdgeDesc = fd.Messages().ByName("RichEdge")
	if ProductDesc == nil || PurchaseEdgeDesc == nil || NotAnEntityDesc == nil ||
		OAuthIdentityDesc == nil || RichDocDesc == nil || RichEdgeDesc == nil {
		panic("testpb: missing test message descriptors")
	}
}

// NewProduct returns a fresh *dynamicpb.Message for the Product type.
// Callers populate it via the proto reflection API (or the helper
// SetProductFields).
func NewProduct() *dynamicpb.Message {
	return dynamicpb.NewMessage(ProductDesc)
}

// NewPurchaseEdge returns a fresh *dynamicpb.Message for the
// PurchaseEdge type.
func NewPurchaseEdge() *dynamicpb.Message {
	return dynamicpb.NewMessage(PurchaseEdgeDesc)
}

// NewNotAnEntity returns a fresh non-entity message.
func NewNotAnEntity() *dynamicpb.Message {
	return dynamicpb.NewMessage(NotAnEntityDesc)
}

// ── Concrete type wrappers for generic-witness tests ────────────────
//
// The Delete[T] / EdgeCreate[T] / EdgeDelete[T] free functions rely
// on the Go language rule that a typed nil pointer of a generated
// proto struct returns a valid descriptor from ProtoReflect(). A
// plain *dynamicpb.Message has NO such property — it carries the
// descriptor at runtime. To exercise the generic-witness path in
// tests without depending on protoc, we wrap the dynamicpb
// descriptor in a small concrete type whose zero-value pointer can
// be ProtoReflected.

// Product is a concrete proto.Message wrapper around the Product
// descriptor. It exists purely so “entdb.Delete[*testpb.Product]“
// compiles and runs against a typed witness without needing a
// generated .pb.go file.
type Product struct {
	msg *dynamicpb.Message
}

// NewProductMsg returns a fresh writable *Product.
func NewProductMsg() *Product {
	return &Product{msg: dynamicpb.NewMessage(ProductDesc)}
}

// ProtoReflect implements proto.Message. A nil *Product returns the
// Product descriptor's zero message — the path Delete[*Product]
// relies on. A non-nil *Product with a nil inner dynamic message
// lazily constructs one so “new(Product).ProtoReflect().Set(...)“
// round-trips through the wrapper.
func (p *Product) ProtoReflect() protoreflect.Message {
	if p == nil {
		return dynamicpb.NewMessage(ProductDesc).ProtoReflect()
	}
	if p.msg == nil {
		p.msg = dynamicpb.NewMessage(ProductDesc)
	}
	return p.msg.ProtoReflect()
}

// Reset implements proto.Message.
func (p *Product) Reset() { *p = Product{msg: dynamicpb.NewMessage(ProductDesc)} }

// String implements proto.Message.
func (p *Product) String() string {
	if p == nil || p.msg == nil {
		return ""
	}
	return p.msg.String()
}

// SetFields is a convenience for populating sku/name/price_cents.
func (p *Product) SetFields(sku, name string, priceCents int64) {
	if p.msg == nil {
		p.msg = dynamicpb.NewMessage(ProductDesc)
	}
	SetProductFields(p.msg, sku, name, priceCents)
}

// SKU returns the sku field.
func (p *Product) SKU() string { return GetProductSKU(p.msg) }

// Name returns the name field.
func (p *Product) Name() string { return GetProductName(p.msg) }

// PriceCents returns the price_cents field.
func (p *Product) PriceCents() int64 { return GetProductPriceCents(p.msg) }

// PurchaseEdge is the edge counterpart to Product.
type PurchaseEdge struct {
	msg *dynamicpb.Message
}

// NewPurchaseEdgeMsg returns a fresh *PurchaseEdge.
func NewPurchaseEdgeMsg() *PurchaseEdge {
	return &PurchaseEdge{msg: dynamicpb.NewMessage(PurchaseEdgeDesc)}
}

// ProtoReflect implements proto.Message.
func (e *PurchaseEdge) ProtoReflect() protoreflect.Message {
	if e == nil || e.msg == nil {
		return dynamicpb.NewMessage(PurchaseEdgeDesc).ProtoReflect()
	}
	return e.msg.ProtoReflect()
}

// Reset implements proto.Message.
func (e *PurchaseEdge) Reset() { *e = PurchaseEdge{msg: dynamicpb.NewMessage(PurchaseEdgeDesc)} }

// String implements proto.Message.
func (e *PurchaseEdge) String() string { return "" }

// NotAnEntity is a concrete message with no entdb annotations.
type NotAnEntity struct {
	msg *dynamicpb.Message
}

// NewNotAnEntityMsg returns a fresh *NotAnEntity.
func NewNotAnEntityMsg() *NotAnEntity {
	return &NotAnEntity{msg: dynamicpb.NewMessage(NotAnEntityDesc)}
}

// ProtoReflect implements proto.Message.
func (n *NotAnEntity) ProtoReflect() protoreflect.Message {
	if n == nil || n.msg == nil {
		return dynamicpb.NewMessage(NotAnEntityDesc).ProtoReflect()
	}
	return n.msg.ProtoReflect()
}

// Reset implements proto.Message.
func (n *NotAnEntity) Reset() { *n = NotAnEntity{msg: dynamicpb.NewMessage(NotAnEntityDesc)} }

// String implements proto.Message.
func (n *NotAnEntity) String() string { return "" }

// RichDoc is a concrete proto.Message wrapper around RichDocDesc — used
// by the regression test set covering #604 (ref_type_id) and every
// adjacent FieldOpts / NodeOpts attribute the Go SDK had been silently
// dropping (pii, deprecated, description, data_policy, subject_field,
// legal_basis).
type RichDoc struct {
	msg *dynamicpb.Message
}

// NewRichDocMsg returns a fresh writable *RichDoc.
func NewRichDocMsg() *RichDoc {
	return &RichDoc{msg: dynamicpb.NewMessage(RichDocDesc)}
}

// ProtoReflect implements proto.Message.
func (r *RichDoc) ProtoReflect() protoreflect.Message {
	if r == nil || r.msg == nil {
		return dynamicpb.NewMessage(RichDocDesc).ProtoReflect()
	}
	return r.msg.ProtoReflect()
}

// Reset implements proto.Message.
func (r *RichDoc) Reset() { *r = RichDoc{msg: dynamicpb.NewMessage(RichDocDesc)} }

// String implements proto.Message.
func (r *RichDoc) String() string { return "" }

// RichEdge is the edge-side regression fixture covering EdgeOpts
// attributes (unique_per_from, on_subject_exit, data_policy,
// deprecated, description) that the Go SDK had been hardcoding away.
type RichEdge struct {
	msg *dynamicpb.Message
}

// NewRichEdgeMsg returns a fresh writable *RichEdge.
func NewRichEdgeMsg() *RichEdge {
	return &RichEdge{msg: dynamicpb.NewMessage(RichEdgeDesc)}
}

// ProtoReflect implements proto.Message.
func (e *RichEdge) ProtoReflect() protoreflect.Message {
	if e == nil || e.msg == nil {
		return dynamicpb.NewMessage(RichEdgeDesc).ProtoReflect()
	}
	return e.msg.ProtoReflect()
}

// Reset implements proto.Message.
func (e *RichEdge) Reset() { *e = RichEdge{msg: dynamicpb.NewMessage(RichEdgeDesc)} }

// String implements proto.Message.
func (e *RichEdge) String() string { return "" }

// SetProductFields populates the sku / name / price_cents fields on
// a Product dynamic message.
func SetProductFields(m *dynamicpb.Message, sku, name string, priceCents int64) {
	mr := m.ProtoReflect()
	fields := mr.Descriptor().Fields()
	if sku != "" {
		mr.Set(fields.ByName("sku"), protoreflect.ValueOfString(sku))
	}
	if name != "" {
		mr.Set(fields.ByName("name"), protoreflect.ValueOfString(name))
	}
	if priceCents != 0 {
		mr.Set(fields.ByName("price_cents"), protoreflect.ValueOfInt64(priceCents))
	}
}

// GetProductSKU reads the sku field from a dynamic Product message.
func GetProductSKU(m *dynamicpb.Message) string {
	mr := m.ProtoReflect()
	return mr.Get(mr.Descriptor().Fields().ByName("sku")).String()
}

// GetProductName reads the name field.
func GetProductName(m *dynamicpb.Message) string {
	mr := m.ProtoReflect()
	return mr.Get(mr.Descriptor().Fields().ByName("name")).String()
}

// GetProductPriceCents reads the price_cents field.
func GetProductPriceCents(m *dynamicpb.Message) int64 {
	mr := m.ProtoReflect()
	return mr.Get(mr.Descriptor().Fields().ByName("price_cents")).Int()
}

// buildFile constructs a FileDescriptor with the three test
// messages and the (entdb.node) / (entdb.edge) / (entdb.field)
// options encoded on the respective descriptor options.
func buildFile() protoreflect.FileDescriptor {
	syntax := "proto3"
	name := "internal/testpb/testpb.proto"
	pkg := "entdb.testpb"

	// Product message.
	productOpts := &descriptorpb.MessageOptions{}
	// (entdb.node) = { type_id: 201 }
	// Extension field 50100, wire type 2 (length-delimited), with a
	// nested NodeOpts{type_id: 201}. NodeOpts.type_id is field 1,
	// varint; value 201.
	nodeOptsInner := encodeVarintField(1, 201)
	productOptsRaw := encodeLengthDelimitedField(50100, nodeOptsInner)
	productOpts.ProtoReflect().SetUnknown(protoreflect.RawFields(productOptsRaw))

	// sku field options: (entdb.field).unique = true.
	// FieldOpts.unique is field 13, varint 1.
	fieldOptsInner := encodeVarintField(13, 1)
	skuFieldOptsRaw := encodeLengthDelimitedField(50102, fieldOptsInner)
	skuOpts := &descriptorpb.FieldOptions{}
	skuOpts.ProtoReflect().SetUnknown(protoreflect.RawFields(skuFieldOptsRaw))

	productMsg := &descriptorpb.DescriptorProto{
		Name:    proto.String("Product"),
		Options: productOpts,
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:     proto.String("sku"),
				Number:   proto.Int32(1),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				JsonName: proto.String("sku"),
				Options:  skuOpts,
			},
			{
				Name:     proto.String("name"),
				Number:   proto.Int32(2),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				JsonName: proto.String("name"),
			},
			{
				Name:     proto.String("price_cents"),
				Number:   proto.Int32(3),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				JsonName: proto.String("priceCents"),
			},
		},
	}

	// PurchaseEdge message — (entdb.edge) = { edge_id: 301 }
	edgeOptsInner := encodeVarintField(1, 301)
	edgeOptsRaw := encodeLengthDelimitedField(50101, edgeOptsInner)
	edgeOpts := &descriptorpb.MessageOptions{}
	edgeOpts.ProtoReflect().SetUnknown(protoreflect.RawFields(edgeOptsRaw))
	edgeMsg := &descriptorpb.DescriptorProto{
		Name:    proto.String("PurchaseEdge"),
		Options: edgeOpts,
	}

	// OAuthIdentity message — (entdb.node) with a composite_unique
	// constraint. NodeOpts is { type_id: 202 (field 1, varint),
	// composite_unique: [...] (field 24, repeated message) }.
	// UniqueConstraint is { fields: [...] (field 1, repeated string),
	// name: "..." (field 2, string) }.
	ucInner := append(
		encodeStringField(1, "provider"),
		encodeStringField(1, "provider_user_id")...,
	)
	ucInner = append(ucInner, encodeStringField(2, "provider_user_id")...)
	oauthNodeOpts := append(
		encodeVarintField(1, 202),
		encodeLengthDelimitedField(24, ucInner)...,
	)
	oauthOptsRaw := encodeLengthDelimitedField(50100, oauthNodeOpts)
	oauthOpts := &descriptorpb.MessageOptions{}
	oauthOpts.ProtoReflect().SetUnknown(protoreflect.RawFields(oauthOptsRaw))
	oauthMsg := &descriptorpb.DescriptorProto{
		Name:    proto.String("OAuthIdentity"),
		Options: oauthOpts,
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:     proto.String("provider"),
				Number:   proto.Int32(1),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				JsonName: proto.String("provider"),
			},
			{
				Name:     proto.String("provider_user_id"),
				Number:   proto.Int32(2),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				JsonName: proto.String("providerUserId"),
			},
		},
	}

	// NotAnEntity — no EntDB annotations.
	notEntityMsg := &descriptorpb.DescriptorProto{
		Name: proto.String("NotAnEntity"),
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:     proto.String("value"),
				Number:   proto.Int32(1),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				JsonName: proto.String("value"),
			},
		},
	}

	// RichDoc — node type that exercises every FieldOpts + NodeOpts
	// attribute the wire SchemaFieldDef / SchemaNodeTypeDef carries.
	// Regression fixture for #604 (ref_type_id) plus pii / deprecated /
	// description per field, and data_policy / subject_field /
	// legal_basis / deprecated / description per node.
	//
	// NodeOpts wire encoding: type_id=401 (field 1, varint),
	// data_policy=BUSINESS (field 6 = 1), subject_field="owner_id"
	// (field 7), legal_basis="user-consent" (field 9),
	// description="Rich doc; all NodeOpts attributes" (field 10),
	// deprecated=true (field 11 = 1).
	richDocNodeOpts := encodeVarintField(1, 401)
	richDocNodeOpts = append(richDocNodeOpts, encodeVarintField(6, 1)...)
	richDocNodeOpts = append(richDocNodeOpts, encodeStringField(7, "owner_id")...)
	richDocNodeOpts = append(richDocNodeOpts, encodeStringField(9, "user-consent")...)
	richDocNodeOpts = append(richDocNodeOpts, encodeStringField(10, "Rich doc; all NodeOpts attributes")...)
	richDocNodeOpts = append(richDocNodeOpts, encodeVarintField(11, 1)...)
	richDocOptsRaw := encodeLengthDelimitedField(50100, richDocNodeOpts)
	richDocOpts := &descriptorpb.MessageOptions{}
	richDocOpts.ProtoReflect().SetUnknown(protoreflect.RawFields(richDocOptsRaw))

	// title field: (entdb.field).description = "the title"
	titleFieldOpts := &descriptorpb.FieldOptions{}
	titleFieldOpts.ProtoReflect().SetUnknown(protoreflect.RawFields(
		encodeLengthDelimitedField(50102, encodeStringField(10, "the title")),
	))

	// owner_id field: (entdb.field) = { kind: "ref", ref_type_id: 201 }
	ownerInner := append(
		encodeStringField(7, "ref"),
		encodeVarintField(8, 201)...,
	)
	ownerFieldOpts := &descriptorpb.FieldOptions{}
	ownerFieldOpts.ProtoReflect().SetUnknown(protoreflect.RawFields(
		encodeLengthDelimitedField(50102, ownerInner),
	))

	// author_email field: pii=true, deprecated=true, description=...
	authorInner := encodeVarintField(4, 1)                                       // pii
	authorInner = append(authorInner, encodeStringField(10, "scrub on anonymise")...) // description
	authorInner = append(authorInner, encodeVarintField(11, 1)...)               // deprecated
	authorFieldOpts := &descriptorpb.FieldOptions{}
	authorFieldOpts.ProtoReflect().SetUnknown(protoreflect.RawFields(
		encodeLengthDelimitedField(50102, authorInner),
	))

	richDocMsg := &descriptorpb.DescriptorProto{
		Name:    proto.String("RichDoc"),
		Options: richDocOpts,
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:     proto.String("title"),
				Number:   proto.Int32(1),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				JsonName: proto.String("title"),
				Options:  titleFieldOpts,
			},
			{
				Name:     proto.String("body"),
				Number:   proto.Int32(2),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				JsonName: proto.String("body"),
			},
			{
				Name:     proto.String("owner_id"),
				Number:   proto.Int32(3),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				JsonName: proto.String("ownerId"),
				Options:  ownerFieldOpts,
			},
			{
				Name:     proto.String("author_email"),
				Number:   proto.Int32(4),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				JsonName: proto.String("authorEmail"),
				Options:  authorFieldOpts,
			},
		},
	}

	// RichEdge — exercises every EdgeOpts attribute. EdgeOpts wire:
	// edge_id=501 (field 1), unique_per_from=true (field 4 = 1),
	// data_policy=AUDIT (field 5 = 3), on_subject_exit=TO (field 6 = 2),
	// description=... (field 9), deprecated=true (field 10 = 1).
	richEdgeOptsInner := encodeVarintField(1, 501)
	richEdgeOptsInner = append(richEdgeOptsInner, encodeVarintField(4, 1)...)
	richEdgeOptsInner = append(richEdgeOptsInner, encodeVarintField(5, 3)...)
	richEdgeOptsInner = append(richEdgeOptsInner, encodeVarintField(6, 2)...)
	richEdgeOptsInner = append(richEdgeOptsInner, encodeStringField(9, "Rich edge; all EdgeOpts attributes")...)
	richEdgeOptsInner = append(richEdgeOptsInner, encodeVarintField(10, 1)...)
	richEdgeOptsRaw := encodeLengthDelimitedField(50101, richEdgeOptsInner)
	richEdgeOpts := &descriptorpb.MessageOptions{}
	richEdgeOpts.ProtoReflect().SetUnknown(protoreflect.RawFields(richEdgeOptsRaw))

	richEdgeMsg := &descriptorpb.DescriptorProto{
		Name:    proto.String("RichEdge"),
		Options: richEdgeOpts,
	}

	fdp := &descriptorpb.FileDescriptorProto{
		Name:        proto.String(name),
		Package:     proto.String(pkg),
		Syntax:      proto.String(syntax),
		MessageType: []*descriptorpb.DescriptorProto{productMsg, edgeMsg, oauthMsg, notEntityMsg, richDocMsg, richEdgeMsg},
	}

	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		panic(fmt.Errorf("testpb: build file descriptor: %w", err))
	}
	return fd
}

// encodeVarintField returns the wire-format bytes for a single
// varint-typed field “(fieldNum, value)“.
func encodeVarintField(fieldNum, value uint64) []byte {
	var out []byte
	tag := (fieldNum << 3) | 0 // wire type 0 = varint
	out = appendVarint(out, tag)
	out = appendVarint(out, value)
	return out
}

// encodeStringField returns the wire-format bytes for a single
// string-typed field “(fieldNum, value)“ (length-delimited).
func encodeStringField(fieldNum uint64, value string) []byte {
	return encodeLengthDelimitedField(fieldNum, []byte(value))
}

// encodeLengthDelimitedField returns the wire-format bytes for a
// single length-delimited field “(fieldNum, payload)“.
func encodeLengthDelimitedField(fieldNum uint64, payload []byte) []byte {
	var out []byte
	tag := (fieldNum << 3) | 2 // wire type 2 = length-delimited
	out = appendVarint(out, tag)
	out = appendVarint(out, uint64(len(payload)))
	out = append(out, payload...)
	return out
}

// appendVarint encodes v as a protobuf varint and appends it to buf.
func appendVarint(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	return append(buf, byte(v))
}
