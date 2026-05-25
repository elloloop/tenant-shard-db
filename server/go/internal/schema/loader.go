// Loader and serialiser for the cross-language schema JSON contract.
//
// The schema JSON document has two top-level keys, "node_types" and
// "edge_types", each a list sorted by id. LoadFromJSON consumes that
// exact shape; (*Registry).MarshalJSON re-emits it.
//
// Field-by-field parity is enforced by the cross-language round-trip
// in tests/python/integration/test_grpc_contract.py.

package schema

import (
	"encoding/json"
	"fmt"
)

// schemaFile is the wire shape of the to_dict() / from_dict() document.
// It is the on-disk envelope for both the JSON loader and the
// fingerprint canonicaliser.
type schemaFile struct {
	NodeTypes []NodeTypeDef `json:"node_types"`
	EdgeTypes []EdgeTypeDef `json:"edge_types"`
}

// LoadFromJSON parses the SchemaRegistry JSON shape into a fresh,
// mutable *Registry. The returned registry is not yet
// frozen — callers (typically server boot) call Freeze() once all
// supplemental registrations are done.
//
// Validation runs on every type before insertion, so a malformed type
// surfaces a typed error here rather than at first request time.
func LoadFromJSON(data []byte) (*Registry, error) {
	var f schemaFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("schema: parse JSON: %w", err)
	}
	r := NewRegistry()
	for i := range f.NodeTypes {
		nt := f.NodeTypes[i]
		if err := r.RegisterNode(&nt); err != nil {
			return nil, fmt.Errorf("schema: register node %q: %w", nt.Name, err)
		}
	}
	for i := range f.EdgeTypes {
		et := f.EdgeTypes[i]
		if et.OnSubjectExit == "" {
			et.OnSubjectExit = OnSubjectExitBoth
		}
		if err := r.RegisterEdge(&et); err != nil {
			return nil, fmt.Errorf("schema: register edge %q: %w", et.Name, err)
		}
	}
	return r, nil
}

// MarshalJSON emits the registry in canonical order (node_types sorted
// by type_id, edge_types sorted by edge_id). The output is suitable
// for round-tripping through LoadFromJSON and as input to the
// fingerprint canonicaliser.
//
// MarshalJSON is also implicitly called by encoding/json so callers can
// json.Marshal(reg) directly.
func (r *Registry) MarshalJSON() ([]byte, error) {
	doc := r.toFile()
	return json.Marshal(doc)
}

// toFile is the shared lowering used by both MarshalJSON and the
// fingerprint canonicaliser. Types are sorted by id. Nil Fields / Props
// slices are normalised to empty slices so the output emits `[]` not
// `null` (the schema JSON contract always emits the array, even when
// empty).
func (r *Registry) toFile() schemaFile {
	s := r.load()
	doc := schemaFile{
		NodeTypes: make([]NodeTypeDef, 0, len(s.nodes)),
		EdgeTypes: make([]EdgeTypeDef, 0, len(s.edges)),
	}
	ids := make([]int32, 0, len(s.nodes))
	for id := range s.nodes {
		ids = append(ids, id)
	}
	sortInt32(ids)
	for _, id := range ids {
		n := *s.nodes[id]
		if n.Fields == nil {
			n.Fields = []FieldDef{}
		}
		doc.NodeTypes = append(doc.NodeTypes, n)
	}
	eids := make([]int32, 0, len(s.edges))
	for id := range s.edges {
		eids = append(eids, id)
	}
	sortInt32(eids)
	for _, id := range eids {
		e := *s.edges[id]
		if e.Props == nil {
			e.Props = []FieldDef{}
		}
		// on_subject_exit is always emitted (defaults to "both").
		if e.OnSubjectExit == "" {
			e.OnSubjectExit = OnSubjectExitBoth
		}
		doc.EdgeTypes = append(doc.EdgeTypes, e)
	}
	return doc
}

func sortInt32(s []int32) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
