// protoc-gen-entdb-keys is a protoc plugin that reads
// (entdb.field).unique = true annotations from .proto files and emits
// typed UniqueKey sidecar files for the EntDB SDK in both Python and Go.
//
// See docs/decisions/sdk_api.md for the authoritative SDK surface ADR.
package main

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Extension field numbers from entdb_options.proto. These are the
// contract with the real options file — the plugin resolves them by
// number at runtime, so it works against both the testdata fixture
// copy and the production entdb_options.proto so long as both agree
// on these numbers.
const (
	messageExtNodeNumber = 50100 // (entdb.node) on MessageOptions
	fieldExtFieldNumber  = 50102 // (entdb.field) on FieldOptions

	nodeOptsTypeIDFieldNumber = 1  // NodeOpts.type_id
	fieldOptsUniqueFieldNum   = 13 // FieldOpts.unique
)

// Initialisms that get fully upper-cased when translating snake_case
// field names to PascalCase Go identifiers. This mirrors the style
// used by protoc-gen-go and golang/lint.
var initialisms = map[string]bool{
	"ACL":   true,
	"API":   true,
	"ASCII": true,
	"CPU":   true,
	"CSS":   true,
	"DNS":   true,
	"EOF":   true,
	"GUID":  true,
	"HTML":  true,
	"HTTP":  true,
	"HTTPS": true,
	"ID":    true,
	"IP":    true,
	"JSON":  true,
	"LHS":   true,
	"QPS":   true,
	"RAM":   true,
	"RHS":   true,
	"RPC":   true,
	"SKU":   true,
	"SLA":   true,
	"SMTP":  true,
	"SQL":   true,
	"SSH":   true,
	"TCP":   true,
	"TLS":   true,
	"TTL":   true,
	"UDP":   true,
	"UI":    true,
	"UID":   true,
	"UUID":  true,
	"URI":   true,
	"URL":   true,
	"UTF8":  true,
	"VM":    true,
	"XML":   true,
	"XMPP":  true,
	"XSRF":  true,
	"XSS":   true,
	"YAML":  true,
}

// uniqueField describes one unique-indexed proto field discovered on
// a node-type message.
type uniqueField struct {
	ProtoName string            // e.g. "order_number"
	FieldID   int32             // proto field number
	Kind      protoreflect.Kind // scalar kind
}

// nodeMessage describes one node-type message with at least one
// unique field.
type nodeMessage struct {
	ProtoName    string // the message name as declared in proto ("Product")
	GoIdent      string // PascalCase identifier suitable for Go ("Product")
	PyClassName  string // Python class name ("ProductKeys")
	TypeID       int32
	UniqueFields []uniqueField
}

func main() {
	protogen.Options{}.Run(func(gen *protogen.Plugin) error {
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			if err := processFile(gen, f); err != nil {
				return fmt.Errorf("%s: %w", f.Desc.Path(), err)
			}
		}
		return nil
	})
}

func processFile(gen *protogen.Plugin, f *protogen.File) error {
	nodes, err := collectNodes(f)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return nil
	}

	pyContent := renderPython(f, nodes)
	goContent := renderGo(f, nodes)

	// Emit files next to the protoc-gen-go output (same directory as
	// the .proto source file, source_relative-style).
	baseName := strings.TrimSuffix(f.Desc.Path(), ".proto")

	pyFile := gen.NewGeneratedFile(baseName+"_entdb.py", f.GoImportPath)
	pyFile.P(pyContent)

	goFile := gen.NewGeneratedFile(baseName+"_entdb.go", f.GoImportPath)
	goFile.P(goContent)

	return nil
}

// collectNodes walks every message in the file, keeps the ones
// annotated with (entdb.node), and collects their (entdb.field).unique
// fields. Returns a non-nil error if any unique field is non-scalar
// or otherwise unsupported.
func collectNodes(f *protogen.File) ([]nodeMessage, error) {
	var out []nodeMessage
	for _, m := range f.Messages {
		typeID, isNode := readNodeTypeID(m.Desc.Options())
		if !isNode {
			continue
		}
		var uniques []uniqueField
		for _, field := range m.Fields {
			if !readFieldUnique(field.Desc.Options()) {
				continue
			}
			if err := validateUniqueField(m, field); err != nil {
				return nil, err
			}
			uniques = append(uniques, uniqueField{
				ProtoName: string(field.Desc.Name()),
				FieldID:   int32(field.Desc.Number()),
				Kind:      field.Desc.Kind(),
			})
		}
		if len(uniques) == 0 {
			continue
		}
		protoName := string(m.Desc.Name())
		out = append(out, nodeMessage{
			ProtoName:    protoName,
			GoIdent:      protoName, // top-level message name already PascalCase
			PyClassName:  protoName + "Keys",
			TypeID:       typeID,
			UniqueFields: uniques,
		})
	}
	// Deterministic order: by type_id, then name.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TypeID != out[j].TypeID {
			return out[i].TypeID < out[j].TypeID
		}
		return out[i].ProtoName < out[j].ProtoName
	})
	return out, nil
}

// validateUniqueField rejects unique annotations on field shapes that
// don't make sense for a unique-value lookup: repeated, map, message,
// group, enum, and proto3 "optional" (explicit-presence) fields.
func validateUniqueField(m *protogen.Message, field *protogen.Field) error {
	qn := fmt.Sprintf("%s.%s", m.Desc.Name(), field.Desc.Name())
	d := field.Desc

	if d.IsList() {
		return fmt.Errorf("%s: (entdb.field).unique is not supported on repeated fields", qn)
	}
	if d.IsMap() {
		return fmt.Errorf("%s: (entdb.field).unique is not supported on map fields", qn)
	}
	if d.ContainingOneof() != nil {
		return fmt.Errorf("%s: (entdb.field).unique is not supported on oneof fields", qn)
	}
	if d.HasOptionalKeyword() {
		return fmt.Errorf("%s: (entdb.field).unique is not supported on proto3 optional fields", qn)
	}
	switch d.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return fmt.Errorf("%s: (entdb.field).unique is not supported on message-typed fields", qn)
	case protoreflect.EnumKind:
		return fmt.Errorf("%s: (entdb.field).unique is not supported on enum-typed fields", qn)
	}
	if _, _, ok := kindToPyGo(d.Kind()); !ok {
		return fmt.Errorf("%s: (entdb.field).unique is not supported on fields of kind %s", qn, d.Kind())
	}
	return nil
}

// readNodeTypeID walks the unknown fields of a MessageOptions message,
// looks for extension number 50100 ((entdb.node)), and inside that
// submessage pulls out NodeOpts.type_id (field 1). Returns (type_id, true)
// if the extension is present.
//
// We do this by-number (instead of using proto.GetExtension) so that
// the plugin doesn't need a compile-time Go import of entdb_options.pb.go
// — it works against any options proto that agrees on the numbers.
func readNodeTypeID(opts protoreflect.ProtoMessage) (int32, bool) {
	if opts == nil {
		return 0, false
	}
	raw, err := proto.Marshal(opts)
	if err != nil || len(raw) == 0 {
		return 0, false
	}
	sub, ok := findRawSubMessage(raw, messageExtNodeNumber)
	if !ok {
		return 0, false
	}
	v, ok := findRawVarint(sub, nodeOptsTypeIDFieldNumber)
	if !ok {
		return 0, false
	}
	return int32(v), true
}

// readFieldUnique returns true if (entdb.field).unique == true is set
// on the given FieldOptions.
func readFieldUnique(opts protoreflect.ProtoMessage) bool {
	if opts == nil {
		return false
	}
	raw, err := proto.Marshal(opts)
	if err != nil || len(raw) == 0 {
		return false
	}
	sub, ok := findRawSubMessage(raw, fieldExtFieldNumber)
	if !ok {
		return false
	}
	v, ok := findRawVarint(sub, fieldOptsUniqueFieldNum)
	if !ok {
		return false
	}
	return v != 0
}

// --- Minimal protobuf wire-format reader --------------------------------
//
// We implement just enough of the wire format to find a length-delimited
// submessage by field number, and to find a varint by field number inside
// a submessage. This lets us read extensions without needing the generated
// Go types for entdb_options.proto.

const (
	wireVarint = 0
	wireI64    = 1
	wireLen    = 2
	wireI32    = 5
)

func findRawSubMessage(buf []byte, wantField int) ([]byte, bool) {
	for len(buf) > 0 {
		tag, n := decodeVarint(buf)
		if n == 0 {
			return nil, false
		}
		buf = buf[n:]
		field := int(tag >> 3)
		wire := int(tag & 7)
		switch wire {
		case wireVarint:
			_, m := decodeVarint(buf)
			if m == 0 {
				return nil, false
			}
			buf = buf[m:]
		case wireI64:
			if len(buf) < 8 {
				return nil, false
			}
			buf = buf[8:]
		case wireI32:
			if len(buf) < 4 {
				return nil, false
			}
			buf = buf[4:]
		case wireLen:
			ln, m := decodeVarint(buf)
			if m == 0 {
				return nil, false
			}
			buf = buf[m:]
			if uint64(len(buf)) < ln {
				return nil, false
			}
			payload := buf[:ln]
			buf = buf[ln:]
			if field == wantField {
				return payload, true
			}
		default:
			return nil, false
		}
	}
	return nil, false
}

func findRawVarint(buf []byte, wantField int) (uint64, bool) {
	for len(buf) > 0 {
		tag, n := decodeVarint(buf)
		if n == 0 {
			return 0, false
		}
		buf = buf[n:]
		field := int(tag >> 3)
		wire := int(tag & 7)
		switch wire {
		case wireVarint:
			v, m := decodeVarint(buf)
			if m == 0 {
				return 0, false
			}
			buf = buf[m:]
			if field == wantField {
				return v, true
			}
		case wireI64:
			if len(buf) < 8 {
				return 0, false
			}
			buf = buf[8:]
		case wireI32:
			if len(buf) < 4 {
				return 0, false
			}
			buf = buf[4:]
		case wireLen:
			ln, m := decodeVarint(buf)
			if m == 0 {
				return 0, false
			}
			buf = buf[m:]
			if uint64(len(buf)) < ln {
				return 0, false
			}
			buf = buf[ln:]
		default:
			return 0, false
		}
	}
	return 0, false
}

func decodeVarint(buf []byte) (uint64, int) {
	var v uint64
	var shift uint
	for i, b := range buf {
		if i >= 10 {
			return 0, 0
		}
		v |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return v, i + 1
		}
		shift += 7
	}
	return 0, 0
}

// --- Type mapping --------------------------------------------------------

// kindToPyGo maps a proto scalar kind to its Python and Go element
// type names. Returns ok=false for unsupported kinds.
func kindToPyGo(k protoreflect.Kind) (py, goType string, ok bool) {
	switch k {
	case protoreflect.StringKind:
		return "str", "string", true
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int", "int32", true
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "int", "uint32", true
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int", "int64", true
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "int", "uint64", true
	case protoreflect.FloatKind:
		return "float", "float32", true
	case protoreflect.DoubleKind:
		return "float", "float64", true
	case protoreflect.BoolKind:
		return "bool", "bool", true
	case protoreflect.BytesKind:
		return "bytes", "[]byte", true
	}
	return "", "", false
}

// --- snake_case → PascalCase with initialism handling --------------------

func pascalCase(snake string) string {
	parts := strings.Split(snake, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		upper := strings.ToUpper(p)
		if initialisms[upper] {
			b.WriteString(upper)
			continue
		}
		// Title-case the part: first rune upper, rest lower.
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(strings.ToLower(p[1:]))
	}
	return b.String()
}

// --- Python emitter ------------------------------------------------------

func renderPython(f *protogen.File, nodes []nodeMessage) string {
	var b strings.Builder
	b.WriteString("# Code generated by protoc-gen-entdb-keys. DO NOT EDIT.\n")
	fmt.Fprintf(&b, "# source: %s\n\n", f.Desc.Path())
	b.WriteString("from entdb_sdk import UniqueKey\n")
	for _, n := range nodes {
		b.WriteString("\n\n")
		fmt.Fprintf(&b, "class %s:\n", n.PyClassName)
		for _, uf := range n.UniqueFields {
			pyType, _, _ := kindToPyGo(uf.Kind)
			fmt.Fprintf(&b,
				"    %s = UniqueKey[%s](type_id=%d, field_id=%d, name=%q)\n",
				uf.ProtoName, pyType, n.TypeID, uf.FieldID, uf.ProtoName,
			)
		}
	}
	return b.String()
}

// --- Go emitter ----------------------------------------------------------

func renderGo(f *protogen.File, nodes []nodeMessage) string {
	var b strings.Builder
	b.WriteString("// Code generated by protoc-gen-entdb-keys. DO NOT EDIT.\n")
	fmt.Fprintf(&b, "// source: %s\n\n", f.Desc.Path())
	fmt.Fprintf(&b, "package %s\n\n", f.GoPackageName)
	b.WriteString("import \"github.com/elloloop/tenant-shard-db/sdk/go/entdb\"\n\n")
	b.WriteString("var (\n")
	for i, n := range nodes {
		if i > 0 {
			b.WriteString("\n")
		}
		// Align variable names inside a single block for readability.
		maxName := 0
		names := make([]string, len(n.UniqueFields))
		for j, uf := range n.UniqueFields {
			names[j] = n.GoIdent + pascalCase(uf.ProtoName)
			if l := len(names[j]); l > maxName {
				maxName = l
			}
		}
		for j, uf := range n.UniqueFields {
			_, goType, _ := kindToPyGo(uf.Kind)
			fmt.Fprintf(&b,
				"\t%-*s = entdb.UniqueKey[%s]{TypeID: %d, FieldID: %d, Name: %q}\n",
				maxName, names[j], goType, n.TypeID, uf.FieldID, uf.ProtoName,
			)
		}
	}
	b.WriteString(")\n")
	return b.String()
}
