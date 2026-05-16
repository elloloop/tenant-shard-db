// SPDX-License-Identifier: AGPL-3.0-only

package acl

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Registry holds the proto-derived capability mappings and implication
// closures. Built once at server startup; immutable thereafter.
//
// Accepts JSON input mirroring SchemaRegistry.to_json; a future
// follow-up will wire it directly off proto descriptors.
type Registry struct {
	types       map[int32]*TypeCapabilities
	defaultType *TypeCapabilities
}

// TypeCapabilities is per-type capability metadata after build. Mirrors
// the Python TypeCapabilities dataclass at capability_registry.py:137-151.
type TypeCapabilities struct {
	TypeID            int32
	ExtensionEnumName string
	ExtCapNames       map[ExtCapID]string
	Mappings          []CapabilityMapping
	Implications      []CapabilityImplication

	// Precomputed transitive closures, including built-in core rules.
	implicationClosureCore map[CoreCapability]map[CoreCapability]struct{}
	implicationClosureExt  map[ExtCapID]map[ExtCapID]struct{}
	extImpliesCore         map[ExtCapID]map[CoreCapability]struct{}
}

// CapabilityMapping is one row of a type's op → capability table. Exactly
// one of RequiredCore / RequiredExt is non-zero.
type CapabilityMapping struct {
	Op           string
	RequiredCore CoreCapability // CoreCapUnspecified == not set
	RequiredExt  ExtCapID       // 0 == not set
	Field        string         // "" == any
	FieldValue   string         // "" == any
	ChildType    string         // "" == any
}

// Specificity returns a score used to pick the best matching mapping.
// Higher is more specific. Mirrors CapabilityMapping.specificity at
// capability_registry.py:113-119.
func (m CapabilityMapping) Specificity() int {
	score := 0
	if m.Field != "" {
		score++
	}
	if m.FieldValue != "" {
		score++
	}
	if m.ChildType != "" {
		score++
	}
	return score
}

// CapabilityImplication is a single implication rule for a type. Either
// Core or Ext is set (exclusive). The implied set may mix core and ext.
type CapabilityImplication struct {
	Core        CoreCapability // CoreCapUnspecified == not the source
	Ext         ExtCapID       // 0 == not the source
	ImpliesCore []CoreCapability
	ImpliesExt  []ExtCapID
}

// DefaultOpRequirements is the built-in op → core-cap table applied
// when a type has no explicit override. Mirrors DEFAULT_OP_REQUIREMENTS
// at capability_registry.py:61-76. CreateNode is intentionally absent —
// that is a tenant-level check, not per-node.
var DefaultOpRequirements = map[string]CoreCapability{
	"GetNode":           CoreCapRead,
	"GetNodes":          CoreCapRead,
	"GetNodeByKey":      CoreCapRead,
	"QueryNodes":        CoreCapRead,
	"GetEdgesFrom":      CoreCapRead,
	"GetEdgesTo":        CoreCapRead,
	"GetConnectedNodes": CoreCapRead,
	"UpdateNode":        CoreCapEdit,
	"DeleteNode":        CoreCapDelete,
	"CreateEdge":        CoreCapEdit,
	"DeleteEdge":        CoreCapEdit,
	"ShareNode":         CoreCapAdmin,
	"RevokeAccess":      CoreCapAdmin,
	"TransferOwnership": CoreCapAdmin,
}

// NewRegistry returns an empty registry whose only known type is the
// implicit "type 0" — used for callers that haven't registered an
// explicit type. Built-in core implications are applied to type 0 too.
func NewRegistry() *Registry {
	r := &Registry{
		types:       map[int32]*TypeCapabilities{},
		defaultType: &TypeCapabilities{TypeID: 0},
	}
	buildClosures(r.defaultType)
	return r
}

// RegisterType installs a TypeCapabilities for typeID. Idempotent: a
// second call replaces the previous registration for the same id.
// Closures are recomputed from CoreImplications + the per-type
// implications list.
func (r *Registry) RegisterType(tc *TypeCapabilities) {
	if tc == nil {
		return
	}
	if tc.ExtCapNames == nil {
		tc.ExtCapNames = map[ExtCapID]string{}
	}
	buildClosures(tc)
	r.types[tc.TypeID] = tc
}

// GetType returns the TypeCapabilities for typeID, or the implicit
// type-0 default when the id is unknown. Mirrors get_type at
// capability_registry.py:244.
func (r *Registry) GetType(typeID int32) *TypeCapabilities {
	if tc, ok := r.types[typeID]; ok {
		return tc
	}
	return r.defaultType
}

// KnownTypeIDs returns the registered type ids in ascending order. The
// implicit type-0 default is NOT included unless it has been registered
// explicitly via RegisterType.
func (r *Registry) KnownTypeIDs() []int32 {
	ids := make([]int32, 0, len(r.types))
	for id := range r.types {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// OpQuery is the optional set of selectors that narrow a RequiredForOp
// lookup. Empty fields mean "any".
type OpQuery struct {
	Field      string
	FieldValue string
	ChildType  string
}

// RequiredForOp returns the (core, ext) requirement for an op on a
// node of the given type. Exactly one of the returned values is
// non-zero, or both are zero meaning "no requirement — allow".
//
// Mirrors required_for_op at capability_registry.py:250-289. Lookup
// order:
//
//  1. Most-specific type-level mapping matching op + selectors.
//  2. Default op requirement baked into DefaultOpRequirements.
//  3. (CoreCapUnspecified, 0) meaning no requirement.
func (r *Registry) RequiredForOp(typeID int32, op string, q OpQuery) (CoreCapability, ExtCapID) {
	tc := r.GetType(typeID)
	var best *CapabilityMapping
	bestScore := -1
	for i := range tc.Mappings {
		m := &tc.Mappings[i]
		if m.Op != op {
			continue
		}
		if m.Field != "" && m.Field != q.Field {
			continue
		}
		if m.FieldValue != "" && m.FieldValue != q.FieldValue {
			continue
		}
		if m.ChildType != "" && m.ChildType != q.ChildType {
			continue
		}
		score := m.Specificity()
		if score > bestScore {
			best = m
			bestScore = score
		}
	}
	if best != nil {
		return best.RequiredCore, best.RequiredExt
	}
	if def, ok := DefaultOpRequirements[op]; ok {
		return def, 0
	}
	return CoreCapUnspecified, 0
}

// CheckGrant reports whether (grantedCore, grantedExt) satisfies the
// (requiredCore, requiredExt) requirement, honouring implications.
//
// typeID selects which type's extension-capability space to use for
// requiredExt and the type's ext → core closure. grantedExt is always
// interpreted in the scope of the SAME typeID — cross-type extension
// grants never satisfy a check (by construction of the registry).
//
// Mirrors check_grant at capability_registry.py:291-350.
func (r *Registry) CheckGrant(grantedCore []CoreCapability, grantedExt []ExtCapID, requiredCore CoreCapability, requiredExt ExtCapID, typeID int32) bool {
	if requiredCore == CoreCapUnspecified && requiredExt == 0 {
		return true
	}
	tc := r.GetType(typeID)

	grantCore := map[CoreCapability]struct{}{}
	for _, c := range grantedCore {
		if c == CoreCapUnspecified {
			continue
		}
		grantCore[c] = struct{}{}
	}
	grantExt := map[ExtCapID]struct{}{}
	for _, e := range grantedExt {
		if e == 0 {
			continue
		}
		grantExt[e] = struct{}{}
	}

	// Expand grant exts via per-type ext closure AND pull in core caps
	// each ext implies (e.g. PR_EXT_CAP_MERGE → CORE_CAP_EDIT).
	expandedExt := map[ExtCapID]struct{}{}
	for e := range grantExt {
		expandedExt[e] = struct{}{}
		for t := range tc.implicationClosureExt[e] {
			expandedExt[t] = struct{}{}
		}
	}
	promotedCore := map[CoreCapability]struct{}{}
	for e := range expandedExt {
		for c := range tc.extImpliesCore[e] {
			promotedCore[c] = struct{}{}
		}
	}

	// Expand grant cores AFTER folding in promoted-from-ext cores so
	// "ext → EDIT → {READ, COMMENT}" fires correctly.
	expandedCore := map[CoreCapability]struct{}{}
	for c := range grantCore {
		expandedCore[c] = struct{}{}
	}
	for c := range promotedCore {
		expandedCore[c] = struct{}{}
	}
	// Fixed-point would be needed for general DAGs; the closure is
	// already transitive so one pass suffices.
	for c := range expandedCore {
		for t := range tc.implicationClosureCore[c] {
			expandedCore[t] = struct{}{}
		}
	}

	if requiredCore != CoreCapUnspecified {
		_, ok := expandedCore[requiredCore]
		return ok
	}
	if requiredExt != 0 {
		_, ok := expandedExt[requiredExt]
		return ok
	}
	return false
}

// LegacyToCoreCaps is a thin method alias for the package-level helper,
// exposed on Registry for parity with the Python static method
// CapabilityRegistry.legacy_permission_to_core_caps.
func (r *Registry) LegacyToCoreCaps(p Permission) []CoreCapability {
	return LegacyToCoreCaps(p)
}

// buildClosures populates implicationClosureCore / implicationClosureExt
// / extImpliesCore on tc. Mirrors _build_closures at
// capability_registry.py:392-449.
func buildClosures(tc *TypeCapabilities) {
	coreGraph := map[CoreCapability]map[CoreCapability]struct{}{}
	for src, dsts := range CoreImplications {
		s := map[CoreCapability]struct{}{}
		for _, d := range dsts {
			s[d] = struct{}{}
		}
		coreGraph[src] = s
	}
	extGraph := map[ExtCapID]map[ExtCapID]struct{}{}
	extToCore := map[ExtCapID]map[CoreCapability]struct{}{}

	for _, imp := range tc.Implications {
		if imp.Core != CoreCapUnspecified {
			set, ok := coreGraph[imp.Core]
			if !ok {
				set = map[CoreCapability]struct{}{}
				coreGraph[imp.Core] = set
			}
			for _, c := range imp.ImpliesCore {
				set[c] = struct{}{}
			}
			// Implies-ext on a core source is rare and intentionally
			// not modelled: anyone holding imp.Core matches before exts
			// are consulted (capability_registry.py:412-418).
		} else if imp.Ext != 0 {
			set, ok := extGraph[imp.Ext]
			if !ok {
				set = map[ExtCapID]struct{}{}
				extGraph[imp.Ext] = set
			}
			for _, e := range imp.ImpliesExt {
				set[e] = struct{}{}
			}
			if len(imp.ImpliesCore) > 0 {
				cset, ok := extToCore[imp.Ext]
				if !ok {
					cset = map[CoreCapability]struct{}{}
					extToCore[imp.Ext] = cset
				}
				for _, c := range imp.ImpliesCore {
					cset[c] = struct{}{}
				}
			}
		}
	}

	// Fixed-point transitive closure over coreGraph.
	for {
		changed := false
		for src, targets := range coreGraph {
			for t := range targets {
				for tt := range coreGraph[t] {
					if _, ok := targets[tt]; !ok {
						targets[tt] = struct{}{}
						changed = true
					}
				}
			}
			coreGraph[src] = targets
		}
		if !changed {
			break
		}
	}

	// Fixed-point transitive closure over extGraph.
	for {
		changed := false
		for src, targets := range extGraph {
			for t := range targets {
				for tt := range extGraph[t] {
					if _, ok := targets[tt]; !ok {
						targets[tt] = struct{}{}
						changed = true
					}
				}
			}
			extGraph[src] = targets
		}
		if !changed {
			break
		}
	}

	tc.implicationClosureCore = coreGraph
	tc.implicationClosureExt = extGraph
	tc.extImpliesCore = extToCore
}

// RegistryJSON is the JSON wire shape consumed by LoadJSON. Mirrors the
// SchemaRegistry.to_json output for the capability subset.
type RegistryJSON struct {
	Types []TypeJSON `json:"types"`
}

// TypeJSON is the per-type slice of RegistryJSON.
type TypeJSON struct {
	TypeID            int32             `json:"type_id"`
	ExtensionEnumName string            `json:"extension_enum_name,omitempty"`
	ExtCapNames       map[string]string `json:"ext_cap_names,omitempty"` // string keys for JSON
	Mappings          []MappingJSON     `json:"mappings,omitempty"`
	Implications      []ImplicationJSON `json:"implications,omitempty"`
}

// MappingJSON is one CapabilityMapping in JSON form.
type MappingJSON struct {
	Op           string `json:"op"`
	RequiredCore int    `json:"required_core,omitempty"`
	RequiredExt  int    `json:"required_ext,omitempty"`
	Field        string `json:"field,omitempty"`
	FieldValue   string `json:"field_value,omitempty"`
	ChildType    string `json:"child_type,omitempty"`
}

// ImplicationJSON is one CapabilityImplication in JSON form.
type ImplicationJSON struct {
	Core        int   `json:"core,omitempty"`
	Ext         int   `json:"ext,omitempty"`
	ImpliesCore []int `json:"implies_core,omitempty"`
	ImpliesExt  []int `json:"implies_ext,omitempty"`
}

// LoadJSON parses a RegistryJSON blob and registers each TypeJSON as a
// TypeCapabilities. Used at startup once the schema package has emitted
// its proto-derived capability metadata.
func (r *Registry) LoadJSON(data []byte) error {
	var rj RegistryJSON
	if err := json.Unmarshal(data, &rj); err != nil {
		return fmt.Errorf("acl: parse registry json: %w", err)
	}
	for _, tj := range rj.Types {
		tc := &TypeCapabilities{
			TypeID:            tj.TypeID,
			ExtensionEnumName: tj.ExtensionEnumName,
			ExtCapNames:       map[ExtCapID]string{},
		}
		for k, v := range tj.ExtCapNames {
			var id int32
			_, err := fmt.Sscanf(k, "%d", &id)
			if err != nil {
				return fmt.Errorf("acl: ext_cap_names key %q not an int: %w", k, err)
			}
			tc.ExtCapNames[ExtCapID(id)] = v
		}
		for _, m := range tj.Mappings {
			tc.Mappings = append(tc.Mappings, CapabilityMapping{
				Op:           m.Op,
				RequiredCore: CoreCapability(m.RequiredCore),
				RequiredExt:  ExtCapID(m.RequiredExt),
				Field:        m.Field,
				FieldValue:   m.FieldValue,
				ChildType:    m.ChildType,
			})
		}
		for _, imp := range tj.Implications {
			ci := CapabilityImplication{
				Core: CoreCapability(imp.Core),
				Ext:  ExtCapID(imp.Ext),
			}
			for _, c := range imp.ImpliesCore {
				ci.ImpliesCore = append(ci.ImpliesCore, CoreCapability(c))
			}
			for _, e := range imp.ImpliesExt {
				ci.ImpliesExt = append(ci.ImpliesExt, ExtCapID(e))
			}
			tc.Implications = append(tc.Implications, ci)
		}
		r.RegisterType(tc)
	}
	return nil
}
