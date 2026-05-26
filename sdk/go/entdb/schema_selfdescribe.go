// SPDX-License-Identifier: MIT
package entdb

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2/internal/pb"
)

// schemaRegistry is the client-side, NAME-FREE schema the Go SDK rides on
// self-describing writes (ADR-031). It is built once from the proto
// message types the caller registers via [WithSchema] and exposes two
// derived artefacts that stay in lock-step:
//
//   - Descriptor: the wire pb.SchemaDescriptor attached to ExecuteAtomic
//     when the server has not yet confirmed this schema's fingerprint.
//   - Fingerprint: the sha256 over the name-free canonical JSON, computed
//     with the SAME omitempty rules and sort-keys/compact encoding the Go
//     server's schema.computeFingerprint uses — so client and server
//     derive the same hash without ever sharing names.
//
// A nil/empty registry means the SDK sends no descriptor and no
// fingerprint, leaving the server schema-less (the issue #545 path).
type schemaRegistry struct {
	descriptor  *pb.SchemaDescriptor
	fingerprint string
}

// newSchemaRegistry builds the name-free registry from the descriptors of
// the given proto message types. Messages without an (entdb.node) /
// (entdb.edge) option are skipped (mirrors register_proto_schema). Returns
// nil when nothing registered, so callers can treat "no schema" uniformly.
func newSchemaRegistry(msgs []proto.Message) *schemaRegistry {
	if len(msgs) == 0 {
		return nil
	}
	var (
		nodes []*pb.SchemaNodeTypeDef
		edges []*pb.SchemaEdgeTypeDef
		// canonNodes/canonEdges mirror the descriptor in the name-free
		// JSON shape the fingerprint hashes (kept in lock-step with the
		// descriptor so the client-computed fingerprint equals the one the
		// server derives from the same descriptor).
		canonNodes []map[string]any
		canonEdges []map[string]any
	)
	seenNode := map[int32]struct{}{}
	seenEdge := map[int32]struct{}{}
	for _, m := range msgs {
		if m == nil {
			continue
		}
		md := m.ProtoReflect().Descriptor()
		if typeID, err := typeIDFromMessage(m); err == nil && typeID != 0 {
			if _, dup := seenNode[typeID]; dup {
				continue
			}
			seenNode[typeID] = struct{}{}
			nodes = append(nodes, nodeDescriptorFor(md, typeID))
			canonNodes = append(canonNodes, canonNodeFor(md, typeID))
			continue
		}
		if edgeID, err := edgeIDFromMessage(m); err == nil && edgeID != 0 {
			if _, dup := seenEdge[edgeID]; dup {
				continue
			}
			seenEdge[edgeID] = struct{}{}
			edges = append(edges, edgeDescriptorFor(md, edgeID))
			canonEdges = append(canonEdges, canonEdgeFor(md, edgeID))
		}
	}
	if len(nodes) == 0 && len(edges) == 0 {
		return nil
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].GetTypeId() < nodes[j].GetTypeId() })
	sort.Slice(edges, func(i, j int) bool { return edges[i].GetEdgeId() < edges[j].GetEdgeId() })
	sort.Slice(canonNodes, func(i, j int) bool {
		return canonNodes[i]["type_id"].(int64) < canonNodes[j]["type_id"].(int64)
	})
	sort.Slice(canonEdges, func(i, j int) bool {
		return canonEdges[i]["edge_id"].(int64) < canonEdges[j]["edge_id"].(int64)
	})

	return &schemaRegistry{
		descriptor: &pb.SchemaDescriptor{NodeTypes: nodes, EdgeTypes: edges},
		fingerprint: nameFreeFingerprint(map[string]any{
			"node_types": canonNodes,
			"edge_types": canonEdges,
		}),
	}
}

// nodeDescriptorFor builds a name-free pb.SchemaNodeTypeDef from a message
// descriptor. composite_unique is identified by its field_ids tuple only
// (ADR-031); names are never carried. All NodeOpts attributes the server's
// canonical FieldDef/NodeTypeDef structs serialise are emitted here so
// the client-computed fingerprint matches the server's.
func nodeDescriptorFor(md protoreflect.MessageDescriptor, typeID int32) *pb.SchemaNodeTypeDef {
	out := &pb.SchemaNodeTypeDef{TypeId: typeID}
	out.Fields = fieldDescriptorsFor(md)
	nopts := readNodeOpts(md)
	if nopts.deprecated {
		out.Deprecated = true
	}
	if nopts.description != "" {
		out.Description = nopts.description
	}
	if dp := dataPolicyName(nopts.dataPolicy); dp != "" {
		out.DataPolicy = dp
	}
	if nopts.subjectField != "" {
		if fid, ok := protoFieldNameToID(md, nopts.subjectField); ok {
			out.SubjectField = &fid
		}
	}
	if nopts.legalBasis != "" {
		out.LegalBasis = &nopts.legalBasis
	}
	if cu, err := extractCompositeUnique(md); err == nil {
		for _, c := range cu {
			ids := c["field_ids"].([]int64)
			fids := make([]uint32, 0, len(ids))
			for _, id := range ids {
				fids = append(fids, uint32(id))
			}
			out.CompositeUnique = append(out.CompositeUnique, &pb.SchemaCompositeUniqueDef{FieldIds: fids})
		}
	}
	return out
}

// edgeDescriptorFor builds a name-free pb.SchemaEdgeTypeDef from a message
// descriptor. EdgeOpts attributes (deprecated, description, data_policy,
// unique_per_from, on_subject_exit) are carried; from_type_id/to_type_id
// are NOT in EdgeOpts and must be resolved separately from the edge
// message's `from`/`to` field types (tracked as a follow-up).
func edgeDescriptorFor(md protoreflect.MessageDescriptor, edgeID int32) *pb.SchemaEdgeTypeDef {
	eopts := readEdgeOpts(md)
	out := &pb.SchemaEdgeTypeDef{
		EdgeId:        edgeID,
		Props:         fieldDescriptorsFor(md),
		OnSubjectExit: subjectExitName(eopts.onSubjectExit),
	}
	if eopts.uniquePerFrom {
		out.UniquePerFrom = true
	}
	if eopts.deprecated {
		out.Deprecated = true
	}
	if eopts.description != "" {
		out.Description = eopts.description
	}
	if dp := dataPolicyName(eopts.dataPolicy); dp != "" {
		out.DataPolicy = dp
	}
	return out
}

// protoFieldNameToID resolves a proto field NAME to its numeric field_id
// within md. Used to lower NodeOpts.subject_field (a string name in the
// proto option) to a name-free wire field_id (ADR-031).
func protoFieldNameToID(md protoreflect.MessageDescriptor, name string) (uint32, bool) {
	fds := md.Fields()
	for i := 0; i < fds.Len(); i++ {
		fd := fds.Get(i)
		if string(fd.Name()) == name {
			return uint32(fd.Number()), true
		}
	}
	return 0, false
}

func fieldDescriptorsFor(md protoreflect.MessageDescriptor) []*pb.SchemaFieldDef {
	fds := md.Fields()
	out := make([]*pb.SchemaFieldDef, 0, fds.Len())
	for i := 0; i < fds.Len(); i++ {
		fd := fds.Get(i)
		fopts := readFieldOpts(fd)
		f := &pb.SchemaFieldDef{
			FieldId: uint32(fd.Number()),
			Kind:    fieldKindFor(fd, fopts.kindOverride),
		}
		if fopts.required {
			f.Required = true
		}
		if fopts.indexed {
			f.Indexed = true
		}
		if fopts.searchable {
			f.Searchable = true
		}
		if fopts.unique {
			f.Unique = true
		}
		if fopts.pii {
			f.Pii = true
		}
		if fopts.deprecated {
			f.Deprecated = true
		}
		if fopts.description != "" {
			f.Description = fopts.description
		}
		if fopts.hasRefType {
			rt := fopts.refTypeID
			f.RefTypeId = &rt
		}
		if fopts.enumValues != "" {
			for _, ev := range splitEnumValues(fopts.enumValues) {
				f.EnumValues = append(f.EnumValues, ev)
			}
		}
		out = append(out, f)
	}
	return out
}

// canonNodeFor / canonEdgeFor / canonFieldFor mirror the descriptor builders
// in the name-free JSON shape the fingerprint hashes. Kept beside the
// descriptor builders so the two never drift — the fingerprint MUST hash the
// exact attributes the descriptor carries (and the server stores).
func canonNodeFor(md protoreflect.MessageDescriptor, typeID int32) map[string]any {
	out := map[string]any{
		"type_id": int64(typeID),
		"fields":  canonFieldsFor(md),
	}
	nopts := readNodeOpts(md)
	if nopts.deprecated {
		out["deprecated"] = true
	}
	if nopts.description != "" {
		out["description"] = nopts.description
	}
	if dp := dataPolicyName(nopts.dataPolicy); dp != "" {
		out["data_policy"] = dp
	}
	if nopts.subjectField != "" {
		if fid, ok := protoFieldNameToID(md, nopts.subjectField); ok {
			out["subject_field"] = int64(fid)
		}
	}
	if nopts.legalBasis != "" {
		out["legal_basis"] = nopts.legalBasis
	}
	if cu, err := extractCompositeUnique(md); err == nil && len(cu) > 0 {
		out["composite_unique"] = cu
	}
	return out
}

func canonEdgeFor(md protoreflect.MessageDescriptor, edgeID int32) map[string]any {
	eopts := readEdgeOpts(md)
	out := map[string]any{
		"edge_id":         int64(edgeID),
		"from_type_id":    int64(0),
		"to_type_id":      int64(0),
		"props":           canonFieldsFor(md),
		"on_subject_exit": subjectExitName(eopts.onSubjectExit),
	}
	if eopts.uniquePerFrom {
		out["unique_per_from"] = true
	}
	if eopts.deprecated {
		out["deprecated"] = true
	}
	if eopts.description != "" {
		out["description"] = eopts.description
	}
	if dp := dataPolicyName(eopts.dataPolicy); dp != "" {
		out["data_policy"] = dp
	}
	return out
}

func canonFieldsFor(md protoreflect.MessageDescriptor) []map[string]any {
	fds := md.Fields()
	out := make([]map[string]any, 0, fds.Len())
	for i := 0; i < fds.Len(); i++ {
		fd := fds.Get(i)
		fopts := readFieldOpts(fd)
		entry := map[string]any{
			"field_id": int64(fd.Number()),
			"kind":     fieldKindFor(fd, fopts.kindOverride),
		}
		if fopts.required {
			entry["required"] = true
		}
		if fopts.enumValues != "" {
			entry["enum_values"] = splitEnumValues(fopts.enumValues)
		}
		if fopts.hasRefType {
			entry["ref_type_id"] = int64(fopts.refTypeID)
		}
		if fopts.indexed {
			entry["indexed"] = true
		}
		if fopts.searchable {
			entry["searchable"] = true
		}
		if fopts.deprecated {
			entry["deprecated"] = true
		}
		if fopts.description != "" {
			entry["description"] = fopts.description
		}
		if fopts.pii {
			entry["pii"] = true
		}
		if fopts.unique {
			entry["unique"] = true
		}
		out = append(out, entry)
	}
	return out
}

func splitEnumValues(s string) []string {
	var out []string
	cur := ""
	for _, c := range s {
		if c == ',' {
			if t := trimSpace(cur); t != "" {
				out = append(out, t)
			}
			cur = ""
			continue
		}
		cur += string(c)
	}
	if t := trimSpace(cur); t != "" {
		out = append(out, t)
	}
	return out
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// nameFreeFingerprint computes sha256 over the canonical (sort-keys,
// no-whitespace) JSON of the name-free schema doc, matching the Go server's
// schema.computeFingerprint. encoding/json with sort_keys is byte-identical
// to the server's canonicalEncode for the ASCII id/attr payloads schemas use.
func nameFreeFingerprint(doc map[string]any) string {
	b := canonicalJSON(doc)
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// canonicalJSON marshals v with object keys sorted and no whitespace —
// the same shape the server's canonicalEncode emits. encoding/json sorts
// map[string]any keys by default; arrays preserve order (callers pre-sort
// node_types / edge_types / fields).
func canonicalJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
