// SPDX-License-Identifier: MIT
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

	fieldOptsRequiredField     = 1
	fieldOptsSearchableField   = 2
	fieldOptsIndexedField      = 3
	fieldOptsEnumValuesField   = 6
	fieldOptsKindOverrideField = 7
	fieldOptsUniqueField       = 13
)

// NodeOpts extension field numbers consumed by the extractor (from
// sdk/entdb_sdk/proto/entdb_options.proto). composite_unique is the
// repeated UniqueConstraint message (ADR-030 / issue #566).
const (
	nodeOptsCompositeUniqueField = 24 // repeated UniqueConstraint composite_unique = 24

	// UniqueConstraint.fields submessage field number. NAME-FREE (ADR-031):
	// the constraint name (field 2) is no longer read — a composite
	// constraint is identified solely by its field_ids tuple.
	uniqueConstraintFieldsField = 1 // repeated string fields = 1
)

// ExtractSchemaJSON reads an EntDB schema out of a compiled
// protobuf descriptor set and returns the JSON bytes the
// EntDB server expects in “SCHEMA_FILE“.
//
// The descriptor set is what “protoc --descriptor_set_out=FILE
// --include_imports“ produces — the binary
// “descriptorpb.FileDescriptorSet“ form. Every message annotated
// with “(entdb.node)“ or “(entdb.edge)“ becomes a node or edge
// type in the output JSON. Every field on a node or edge gets its
// kind inferred from the proto field type (with the
// “(entdb.field).kind“ string override winning when set), and
// the “(entdb.field).{required,indexed,searchable,unique}“ flags
// flow through.
//
// Output is the same wrapped envelope produced by “entdb-schema
// snapshot“ (Python) — “{"version": 1, "fingerprint": "...",
// "schema": {...}}“ — so the same server boot path
// (“_load_schema_file“) consumes both.
//
// Errors come back when the descriptor set is malformed or when a
// node has a missing required field id (“(entdb.node).type_id ==
// 0“). Messages with no “(entdb.node)“ / “(entdb.edge)“ option
// are silently skipped — same as the Python “register_proto_schema“
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
			entry, err := buildNodeEntry(md, nodeID)
			if err != nil {
				return err
			}
			*nodes = append(*nodes, entry)
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

func buildNodeEntry(md protoreflect.MessageDescriptor, typeID int32) (map[string]any, error) {
	// NAME-FREE (ADR-031): the cross-language schema JSON contract emits
	// ids + attributes only — no type name. The name lives in the proto.
	out := map[string]any{
		"type_id": int64(typeID),
		"fields":  extractFields(md),
	}
	cu, err := extractCompositeUnique(md)
	if err != nil {
		return nil, err
	}
	if len(cu) > 0 {
		out["composite_unique"] = cu
	}
	return out, nil
}

// extractCompositeUnique reads NodeOpts.composite_unique (ADR-030).
// Each UniqueConstraint carries proto field NAMES; this resolves them
// to stable field_ids (ADR-018) against the message's field set. A
// name that does not resolve, or a constraint with fewer than two
// fields, is an error so a typo fails at extract time rather than
// silently dropping the integrity rule (mirrors the Python SDK).
func extractCompositeUnique(md protoreflect.MessageDescriptor) ([]map[string]any, error) {
	opts := md.Options()
	if opts == nil {
		return nil, nil
	}
	raw, err := proto.Marshal(opts)
	if err != nil {
		return nil, nil
	}
	nodeOpts, ok := findLengthDelimited(raw, uint64(extNodeOpts))
	if !ok {
		return nil, nil
	}
	entries := findAllLengthDelimited(nodeOpts, uint64(nodeOptsCompositeUniqueField))
	if len(entries) == 0 {
		return nil, nil
	}
	// Build a proto-field-name -> field_id map for resolution.
	nameToID := map[string]uint32{}
	fds := md.Fields()
	for i := 0; i < fds.Len(); i++ {
		fd := fds.Get(i)
		nameToID[string(fd.Name())] = uint32(fd.Number())
	}
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		names := findAllStrings(e, uint64(uniqueConstraintFieldsField))
		if len(names) < 2 {
			return nil, fmt.Errorf(
				"entdb: %s composite_unique must reference at least 2 fields, got %v",
				md.FullName(), names,
			)
		}
		fieldIDs := make([]int64, 0, len(names))
		for _, nm := range names {
			fid, ok := nameToID[nm]
			if !ok {
				return nil, fmt.Errorf(
					"entdb: %s composite_unique references unknown field %q",
					md.FullName(), nm,
				)
			}
			fieldIDs = append(fieldIDs, int64(fid))
		}
		// NAME-FREE (ADR-031): a composite constraint is identified solely
		// by its field_ids tuple; no constraint name is emitted on the wire
		// or in the fingerprint.
		out = append(out, map[string]any{
			"field_ids": fieldIDs,
		})
	}
	return out, nil
}

// findAllStrings returns every length-delimited (string) field matching
// fieldNum, decoded as UTF-8, in wire order — the repeated-string
// counterpart of findString.
func findAllStrings(buf []byte, fieldNum uint64) []string {
	payloads := findAllLengthDelimited(buf, fieldNum)
	out := make([]string, 0, len(payloads))
	for _, p := range payloads {
		out = append(out, string(p))
	}
	return out
}

func buildEdgeEntry(md protoreflect.MessageDescriptor, edgeID int32) map[string]any {
	// NAME-FREE (ADR-031): edge identified by edge_id only. props and
	// on_subject_exit always emitted (the server's name-free edge shape;
	// on_subject_exit defaults "both").
	out := map[string]any{
		"edge_id":         int64(edgeID),
		"from_type_id":    int64(0),
		"to_type_id":      int64(0),
		"props":           extractFields(md),
		"on_subject_exit": "both",
	}
	return out
}

func extractFields(md protoreflect.MessageDescriptor) []map[string]any {
	fds := md.Fields()
	out := make([]map[string]any, 0, fds.Len())
	for i := 0; i < fds.Len(); i++ {
		fd := fds.Get(i)
		fopts := readFieldOpts(fd)
		// NAME-FREE (ADR-031): field identified by field_id only.
		entry := map[string]any{
			"field_id": int64(fd.Number()),
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

// readMessageOptInt returns the int32 at “innerField“ inside the
// “extNum“ extension on the message's options, plus whether the
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

// fieldKindFor maps a proto field type to the EntDB “kind“ string
// the server's “FieldKind.from_str“ accepts. The
// “(entdb.field).kind“ override wins when set — that's how a proto
// “int64“ carrying epoch milliseconds gets stored as “kind:
// "timestamp"“.
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
// length-delimited (string) field with “fieldNum“. Returns the
// decoded UTF-8 string or "" plus false if not found.
func findString(buf []byte, fieldNum uint64) (string, bool) {
	payload, ok := findLengthDelimited(buf, fieldNum)
	if !ok {
		return "", false
	}
	return string(payload), true
}
