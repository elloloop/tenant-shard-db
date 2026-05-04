package entdb

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// FieldOpts extension field numbers (from
// sdk/entdb_sdk/proto/entdb_options.proto). Decoded by hand from
// MessageOptions to avoid dragging the generated entdb_options Go
// package into every SDK user's binary.
const (
	extFieldOpts = 50102

	fieldOptsRequiredField    = 1
	fieldOptsSearchableField  = 2
	fieldOptsIndexedField     = 3
	fieldOptsEnumValuesField  = 6
	fieldOptsKindOverrideField = 7
	fieldOptsUniqueField      = 13
)

// EdgeOpts has its (from_type, to_type) wired into the message
// descriptor name, not the option — there is no canonical edge-type
// proto on the wire today, so SchemaJSON reports edges with empty
// from_type_id/to_type_id and lets the server fill them from
// elsewhere if needed. Edge property fields work the same way as
// node fields.
const (
	edgeOptsNameField = 2 // string name = 2 in EdgeOpts
)

// ExtractSchemaJSON reads an EntDB schema out of a compiled
// protobuf descriptor set and returns the JSON bytes the
// EntDB server expects in ``SCHEMA_FILE``.
//
// The descriptor set is what ``protoc --descriptor_set_out=FILE
// --include_imports`` produces — the binary
// ``descriptorpb.FileDescriptorSet`` form. Every message annotated
// with ``(entdb.node)`` or ``(entdb.edge)`` becomes a node or edge
// type in the output JSON. Every field on a node or edge gets its
// kind inferred from the proto field type (with the
// ``(entdb.field).kind`` string override winning when set), and
// the ``(entdb.field).{required,indexed,searchable,unique}`` flags
// flow through.
//
// Output is the same wrapped envelope produced by ``entdb-schema
// snapshot`` (Python) — ``{"version": 1, "fingerprint": "...",
// "schema": {...}}`` — so the same server boot path
// (``_load_schema_file``) consumes both.
//
// Errors come back when the descriptor set is malformed or when a
// node has a missing required field id (``(entdb.node).type_id ==
// 0``). Messages with no ``(entdb.node)`` / ``(entdb.edge)`` option
// are silently skipped — same as the Python ``register_proto_schema``
// behaviour.
func ExtractSchemaJSON(fds *descriptorpb.FileDescriptorSet) ([]byte, error) {
	if fds == nil {
		return nil, fmt.Errorf("entdb: nil FileDescriptorSet")
	}
	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, fmt.Errorf("entdb: build files registry: %w", err)
	}

	var (
		nodes []map[string]any
		edges []map[string]any
	)
	var rangeErr error
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		err := walkMessages(fd.Messages(), &nodes, &edges)
		if err != nil {
			rangeErr = err
			return false
		}
		return true
	})
	if rangeErr != nil {
		return nil, rangeErr
	}

	// Stable order — by type_id / edge_id — so the output JSON is
	// deterministic across runs.
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i]["type_id"].(int64) < nodes[j]["type_id"].(int64)
	})
	sort.Slice(edges, func(i, j int) bool {
		return edges[i]["edge_id"].(int64) < edges[j]["edge_id"].(int64)
	})

	// Materialize empty slices so the JSON has ``"node_types": []``
	// instead of ``"node_types": null`` — saves callers one
	// `if x != nil` branch on every consumer.
	if nodes == nil {
		nodes = []map[string]any{}
	}
	if edges == nil {
		edges = []map[string]any{}
	}
	envelope := map[string]any{
		"version":     1,
		"fingerprint": "go-cli-extract",
		"schema": map[string]any{
			"node_types": nodes,
			"edge_types": edges,
		},
	}
	return json.MarshalIndent(envelope, "", "  ")
}

func walkMessages(
	msgs protoreflect.MessageDescriptors,
	nodes *[]map[string]any,
	edges *[]map[string]any,
) error {
	for i := 0; i < msgs.Len(); i++ {
		md := msgs.Get(i)

		nodeID, hasNode := readMessageOptInt(md, extNodeOpts, nodeOptsTypeIDField)
		edgeID, hasEdge := readMessageOptInt(md, extEdgeOpts, edgeOptsEdgeIDField)

		switch {
		case hasNode:
			if nodeID == 0 {
				return fmt.Errorf("entdb: %s has (entdb.node) but type_id is 0", md.FullName())
			}
			*nodes = append(*nodes, buildNodeEntry(md, nodeID))
		case hasEdge:
			if edgeID == 0 {
				return fmt.Errorf("entdb: %s has (entdb.edge) but edge_id is 0", md.FullName())
			}
			*edges = append(*edges, buildEdgeEntry(md, edgeID))
		}

		if err := walkMessages(md.Messages(), nodes, edges); err != nil {
			return err
		}
	}
	return nil
}

func buildNodeEntry(md protoreflect.MessageDescriptor, typeID int32) map[string]any {
	return map[string]any{
		"type_id": int64(typeID),
		"name":    string(md.Name()),
		"fields":  extractFields(md),
	}
}

func buildEdgeEntry(md protoreflect.MessageDescriptor, edgeID int32) map[string]any {
	out := map[string]any{
		"edge_id":      int64(edgeID),
		"name":         readEdgeName(md, string(md.Name())),
		"from_type_id": int64(0),
		"to_type_id":   int64(0),
	}
	props := extractFields(md)
	if len(props) > 0 {
		out["props"] = props
	}
	return out
}

func extractFields(md protoreflect.MessageDescriptor) []map[string]any {
	fds := md.Fields()
	out := make([]map[string]any, 0, fds.Len())
	for i := 0; i < fds.Len(); i++ {
		fd := fds.Get(i)
		fopts := readFieldOpts(fd)
		entry := map[string]any{
			"field_id": int64(fd.Number()),
			"name":     string(fd.Name()),
			"kind":     fieldKindFor(fd, fopts.kindOverride),
		}
		if fopts.required {
			entry["required"] = true
		}
		if fopts.indexed {
			entry["indexed"] = true
		}
		if fopts.searchable {
			entry["searchable"] = true
		}
		if fopts.unique {
			entry["unique"] = true
		}
		if fopts.enumValues != "" {
			entry["enum_values"] = strings.Split(fopts.enumValues, ",")
		}
		out = append(out, entry)
	}
	return out
}

// readMessageOptInt returns the int32 at ``innerField`` inside the
// ``extNum`` extension on the message's options, plus whether the
// extension was present at all.
func readMessageOptInt(md protoreflect.MessageDescriptor, extNum, innerField int32) (int32, bool) {
	opts := md.Options()
	if opts == nil {
		return 0, false
	}
	raw, err := proto.Marshal(opts)
	if err != nil {
		return 0, false
	}
	inner, ok := findLengthDelimited(raw, uint64(extNum))
	if !ok {
		return 0, false
	}
	v, ok := findVarint(inner, uint64(innerField))
	if !ok {
		// Extension present but field missing — treat as id 0 so
		// the caller surfaces the "type_id is 0" error
		// consistently.
		return 0, true
	}
	return int32(v), true
}

// readEdgeName reads ``EdgeOpts.name`` (proto field 2, string) when
// present; otherwise returns ``fallback`` (the message's short
// name).
func readEdgeName(md protoreflect.MessageDescriptor, fallback string) string {
	opts := md.Options()
	if opts == nil {
		return fallback
	}
	raw, err := proto.Marshal(opts)
	if err != nil {
		return fallback
	}
	inner, ok := findLengthDelimited(raw, uint64(extEdgeOpts))
	if !ok {
		return fallback
	}
	if name, ok := findString(inner, uint64(edgeOptsNameField)); ok && name != "" {
		return name
	}
	return fallback
}

type fieldOptsRaw struct {
	required     bool
	searchable   bool
	indexed      bool
	unique       bool
	enumValues   string
	kindOverride string
}

func readFieldOpts(fd protoreflect.FieldDescriptor) fieldOptsRaw {
	out := fieldOptsRaw{}
	opts := fd.Options()
	if opts == nil {
		return out
	}
	raw, err := proto.Marshal(opts)
	if err != nil {
		return out
	}
	inner, ok := findLengthDelimited(raw, uint64(extFieldOpts))
	if !ok {
		return out
	}
	if v, ok := findVarint(inner, uint64(fieldOptsRequiredField)); ok {
		out.required = v != 0
	}
	if v, ok := findVarint(inner, uint64(fieldOptsSearchableField)); ok {
		out.searchable = v != 0
	}
	if v, ok := findVarint(inner, uint64(fieldOptsIndexedField)); ok {
		out.indexed = v != 0
	}
	if v, ok := findVarint(inner, uint64(fieldOptsUniqueField)); ok {
		out.unique = v != 0
	}
	if s, ok := findString(inner, uint64(fieldOptsEnumValuesField)); ok {
		out.enumValues = s
	}
	if s, ok := findString(inner, uint64(fieldOptsKindOverrideField)); ok {
		out.kindOverride = s
	}
	return out
}

// fieldKindFor maps a proto field type to the EntDB ``kind`` string
// the server's ``FieldKind.from_str`` accepts. The
// ``(entdb.field).kind`` override wins when set — that's how a proto
// ``int64`` carrying epoch milliseconds gets stored as ``kind:
// "timestamp"``.
func fieldKindFor(fd protoreflect.FieldDescriptor, override string) string {
	if override != "" {
		return override
	}
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.StringKind:
		return "str"
	case protoreflect.BytesKind:
		return "bytes"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "int"
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return "float"
	case protoreflect.EnumKind:
		return "enum"
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return "json"
	default:
		return "json"
	}
}

// findString walks the wire-format buffer looking for a
// length-delimited (string) field with ``fieldNum``. Returns the
// decoded UTF-8 string or "" plus false if not found.
func findString(buf []byte, fieldNum uint64) (string, bool) {
	payload, ok := findLengthDelimited(buf, fieldNum)
	if !ok {
		return "", false
	}
	return string(payload), true
}
