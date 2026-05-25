// Compatibility engine for entdb-schema check / diff.
//
// Compares two *Registry
// instances and reports every per-rule difference, classifying each as
// breaking (block PR) or non-breaking (informational) per the rules
// documented on each ChangeKind constant.
//
// The Breaking flag on each Change is derived from Kind via the
// breakingKinds lookup table at the bottom of this file. This keeps the
// on-disk JSON output stable even if a rule is re-classified later (a
// reviewer can grep the table to see classification).
//
// Determinism: Check returns changes sorted by (Path, Kind) so callers
// get byte-identical output for byte-identical inputs. The order is
// part of the contract — CI diff comments depend on it.

package schema

import (
	"fmt"
	"sort"
	"strings"
)

// ChangeKind enumerates the classes of schema diff the compat engine
// can detect. New values may be added at the end; do not reorder
// existing entries — String() / JSON output depends on the textual
// constant name, not the integer value, but stability of the names is
// part of the contract too.
type ChangeKind int

const (
	ChangeKindUnknown ChangeKind = iota

	// Node-level
	ChangeKindNodeAdded
	ChangeKindNodeRemoved         // non-breaking (ADR-032: removal is loosening; reuse is the break — see TYPE_ID_REUSED)
	ChangeKindNodeRenamed         // RETIRED (ADR-031: registry is name-free) — never emitted
	ChangeKindTypeIDChanged       // RETIRED (ADR-031: name-based detection gone) — never emitted
	ChangeKindSubjectFieldChanged // BREAKING — GDPR routing changes
	ChangeKindDataPolicyTightened // non-breaking — encryption tier strengthens
	ChangeKindDataPolicyLoosened  // BREAKING — historical data leaks downward
	ChangeKindTypeIDReused        // BREAKING (ADR-032) — a reserved/removed type_id reappears as a live type

	// Field-level
	ChangeKindFieldAdded
	ChangeKindFieldRemoved           // non-breaking (ADR-032: removal is loosening; reuse is the break — see FIELD_ID_REUSED)
	ChangeKindFieldIDChanged         // RETIRED (ADR-031: name-based detection gone) — never emitted
	ChangeKindFieldKindChanged       // BREAKING — type-coercion of stored bytes is unsafe
	ChangeKindFieldRenamed           // RETIRED (ADR-031: registry is name-free) — never emitted
	ChangeKindFieldRequiredTightened // BREAKING — existing rows that omit the field now fail
	ChangeKindFieldRequiredLoosened  // non-breaking — looser is always safe
	ChangeKindFieldUniqueAdded       // BREAKING — historical data may already violate
	ChangeKindFieldUniqueRemoved     // non-breaking
	ChangeKindFieldDeprecated        // non-breaking — informational marker
	ChangeKindFieldUndeprecated      // non-breaking
	ChangeKindFieldRefTypeChanged    // BREAKING — dangling references
	ChangeKindFieldPIIToggled        // non-breaking — informational, but flagged
	ChangeKindFieldIDReused          // BREAKING (ADR-032) — a reserved/removed field_id reappears as a live field
	ChangeKindFieldIndexedAdded      // non-breaking (ADR-032) — query index is additive
	ChangeKindFieldIndexedRemoved    // non-breaking (ADR-032) — dropping a query index loosens
	ChangeKindFieldSearchableAdded   // non-breaking (ADR-032) — FTS index is additive
	ChangeKindFieldSearchableRemoved // non-breaking (ADR-032) — dropping FTS loosens

	// Enum-level (per field)
	ChangeKindEnumValueAdded     // non-breaking — append-only
	ChangeKindEnumValueRemoved   // BREAKING — historic events carry old integer
	ChangeKindEnumValueReordered // BREAKING — integer reuse changes semantics

	// Composite-unique (per node type)
	ChangeKindCompositeUniqueAdded   // BREAKING — existing rows may violate
	ChangeKindCompositeUniqueRemoved // non-breaking
	ChangeKindCompositeUniqueChanged // BREAKING — index re-build can fail

	// Edge-level
	ChangeKindEdgeAdded
	ChangeKindEdgeRemoved              // non-breaking (ADR-032: removal is loosening; reuse is the break — see EDGE_ID_REUSED)
	ChangeKindEdgeFromTypeChanged      // BREAKING — edge semantics shift
	ChangeKindEdgeToTypeChanged        // BREAKING — edge semantics shift
	ChangeKindEdgeUniquePerFromAdded   // BREAKING — historical edges may already violate
	ChangeKindEdgeUniquePerFromRemoved // non-breaking
	ChangeKindOnSubjectExitChanged     // BREAKING — GDPR delete semantics shift
	ChangeKindEdgeIDReused             // BREAKING (ADR-032) — a reserved/removed edge_id reappears as a live edge
)

// changeKindName is the textual constant used in JSON output and text
// summaries. Kept stable across releases — downstream CI parsers grep
// these names.
var changeKindName = map[ChangeKind]string{
	ChangeKindUnknown:                  "UNKNOWN",
	ChangeKindNodeAdded:                "NODE_ADDED",
	ChangeKindNodeRemoved:              "NODE_REMOVED",
	ChangeKindNodeRenamed:              "NODE_RENAMED",
	ChangeKindTypeIDChanged:            "TYPE_ID_CHANGED",
	ChangeKindSubjectFieldChanged:      "SUBJECT_FIELD_CHANGED",
	ChangeKindDataPolicyTightened:      "DATA_POLICY_TIGHTENED",
	ChangeKindDataPolicyLoosened:       "DATA_POLICY_LOOSENED",
	ChangeKindFieldAdded:               "FIELD_ADDED",
	ChangeKindFieldRemoved:             "FIELD_REMOVED",
	ChangeKindFieldIDChanged:           "FIELD_ID_CHANGED",
	ChangeKindFieldKindChanged:         "FIELD_KIND_CHANGED",
	ChangeKindFieldRenamed:             "FIELD_RENAMED",
	ChangeKindFieldRequiredTightened:   "FIELD_REQUIRED_TIGHTENED",
	ChangeKindFieldRequiredLoosened:    "FIELD_REQUIRED_LOOSENED",
	ChangeKindFieldUniqueAdded:         "FIELD_UNIQUE_ADDED",
	ChangeKindFieldUniqueRemoved:       "FIELD_UNIQUE_REMOVED",
	ChangeKindFieldDeprecated:          "FIELD_DEPRECATED",
	ChangeKindFieldUndeprecated:        "FIELD_UNDEPRECATED",
	ChangeKindFieldRefTypeChanged:      "FIELD_REF_TYPE_CHANGED",
	ChangeKindFieldPIIToggled:          "FIELD_PII_TOGGLED",
	ChangeKindTypeIDReused:             "TYPE_ID_REUSED",
	ChangeKindFieldIDReused:            "FIELD_ID_REUSED",
	ChangeKindEdgeIDReused:             "EDGE_ID_REUSED",
	ChangeKindFieldIndexedAdded:        "FIELD_INDEXED_ADDED",
	ChangeKindFieldIndexedRemoved:      "FIELD_INDEXED_REMOVED",
	ChangeKindFieldSearchableAdded:     "FIELD_SEARCHABLE_ADDED",
	ChangeKindFieldSearchableRemoved:   "FIELD_SEARCHABLE_REMOVED",
	ChangeKindEnumValueAdded:           "ENUM_VALUE_ADDED",
	ChangeKindEnumValueRemoved:         "ENUM_VALUE_REMOVED",
	ChangeKindEnumValueReordered:       "ENUM_VALUE_REORDERED",
	ChangeKindCompositeUniqueAdded:     "COMPOSITE_UNIQUE_ADDED",
	ChangeKindCompositeUniqueRemoved:   "COMPOSITE_UNIQUE_REMOVED",
	ChangeKindCompositeUniqueChanged:   "COMPOSITE_UNIQUE_CHANGED",
	ChangeKindEdgeAdded:                "EDGE_ADDED",
	ChangeKindEdgeRemoved:              "EDGE_REMOVED",
	ChangeKindEdgeFromTypeChanged:      "EDGE_FROM_TYPE_CHANGED",
	ChangeKindEdgeToTypeChanged:        "EDGE_TO_TYPE_CHANGED",
	ChangeKindEdgeUniquePerFromAdded:   "EDGE_UNIQUE_PER_FROM_ADDED",
	ChangeKindEdgeUniquePerFromRemoved: "EDGE_UNIQUE_PER_FROM_REMOVED",
	ChangeKindOnSubjectExitChanged:     "ON_SUBJECT_EXIT_CHANGED",
}

// String returns the textual name for the change kind. Stable contract.
func (k ChangeKind) String() string {
	if s, ok := changeKindName[k]; ok {
		return s
	}
	return fmt.Sprintf("CHANGEKIND(%d)", int(k))
}

// MarshalJSON emits the textual constant name so JSON output is
// reviewable as-is.
func (k ChangeKind) MarshalJSON() ([]byte, error) {
	return []byte("\"" + k.String() + "\""), nil
}

// breakingKinds maps each ChangeKind to its breaking-or-not classification.
// A nil entry means non-breaking. Reviewers: flipping a classification
// here is the audit trail.
// Per ADR-032 the governing principle is LOOSENING is safe, TIGHTENING
// is breaking. Type/field/edge *removal* is loosening (safe) — the
// breaking move is reusing the freed id, captured by the *_REUSED kinds.
// FIELD_ID_CHANGED / TYPE_ID_CHANGED are retired under ADR-031 (name-free,
// so a "change" is a remove+add); they are listed here for the historical
// audit trail but are never emitted.
var breakingKinds = map[ChangeKind]bool{
	ChangeKindTypeIDChanged:          true, // retired (ADR-031); never emitted
	ChangeKindTypeIDReused:           true,
	ChangeKindSubjectFieldChanged:    true,
	ChangeKindDataPolicyLoosened:     true,
	ChangeKindFieldIDChanged:         true, // retired (ADR-031); never emitted
	ChangeKindFieldIDReused:          true,
	ChangeKindFieldKindChanged:       true,
	ChangeKindFieldRequiredTightened: true,
	ChangeKindFieldUniqueAdded:       true,
	ChangeKindFieldRefTypeChanged:    true,
	ChangeKindEnumValueRemoved:       true,
	ChangeKindEnumValueReordered:     true,
	ChangeKindCompositeUniqueAdded:   true,
	ChangeKindCompositeUniqueChanged: true,
	ChangeKindEdgeFromTypeChanged:    true,
	ChangeKindEdgeToTypeChanged:      true,
	ChangeKindEdgeUniquePerFromAdded: true,
	ChangeKindOnSubjectExitChanged:   true,
	ChangeKindEdgeIDReused:           true,
}

// IsBreaking reports whether the kind is classified as a breaking
// change.
func (k ChangeKind) IsBreaking() bool { return breakingKinds[k] }

// Change is one row in the compat engine's output. JSON shape is part
// of the contract — see docs/guides/schema-lockdown.md.
type Change struct {
	Kind     ChangeKind `json:"kind"`
	Path     string     `json:"path"`
	OldValue any        `json:"old_value,omitempty"`
	NewValue any        `json:"new_value,omitempty"`
	Message  string     `json:"message"`
	Breaking bool       `json:"breaking"`
}

// String renders a single change for human-readable output.
func (c Change) String() string {
	tag := "[OK]      "
	if c.Breaking {
		tag = "[BREAKING]"
	}
	return fmt.Sprintf("%s %-28s %s — %s", tag, c.Kind.String(), c.Path, c.Message)
}

// Check compares two registries and returns the list of detected
// changes, in deterministic order (sorted by Path then Kind). Neither
// registry is mutated. Either argument may be nil — a nil registry is
// treated as empty (so Check(nil, new) reports every type as added,
// and Check(old, nil) reports every type as removed).
func Check(oldR, newR *Registry) []Change {
	var changes []Change
	changes = append(changes, diffNodes(oldR, newR)...)
	changes = append(changes, diffEdges(oldR, newR)...)
	// Stable order: Path first, then Kind name.
	sort.SliceStable(changes, func(i, j int) bool {
		if changes[i].Path != changes[j].Path {
			return changes[i].Path < changes[j].Path
		}
		return changes[i].Kind.String() < changes[j].Kind.String()
	})
	// Stamp Breaking from the classification table so callers always
	// see the canonical value regardless of how the diff functions
	// initialise it.
	for i := range changes {
		changes[i].Breaking = changes[i].Kind.IsBreaking()
	}
	return changes
}

// HasBreaking reports whether any change in cs is breaking.
func HasBreaking(cs []Change) bool {
	for _, c := range cs {
		if c.Breaking {
			return true
		}
	}
	return false
}

// CountBreaking returns the number of breaking changes.
func CountBreaking(cs []Change) int {
	n := 0
	for _, c := range cs {
		if c.Breaking {
			n++
		}
	}
	return n
}

// -----------------------------------------------------------------------------
// Node diff
// -----------------------------------------------------------------------------

func nodesByID(r *Registry) map[int32]*NodeTypeDef {
	if r == nil {
		return map[int32]*NodeTypeDef{}
	}
	s := r.load()
	out := make(map[int32]*NodeTypeDef, len(s.nodes))
	for id, n := range s.nodes {
		out[id] = n
	}
	return out
}

func diffNodes(oldR, newR *Registry) []Change {
	var out []Change
	oldByID := nodesByID(oldR)
	newByID := nodesByID(newR)

	// Name-free per ADR-031: nodes are keyed solely by type_id. A node
	// present only in old is removed; present only in new is added; the
	// historical name-based NODE_RENAMED / TYPE_ID_CHANGED detection is
	// gone because the registry no longer carries names.
	oldIDs := make([]int32, 0, len(oldByID))
	for id := range oldByID {
		oldIDs = append(oldIDs, id)
	}
	sortInt32(oldIDs)
	for _, id := range oldIDs {
		oldNode := oldByID[id]
		newNode, ok := newByID[id]
		if !ok {
			out = append(out, Change{
				Kind:     ChangeKindNodeRemoved,
				Path:     fmt.Sprintf("node:%d", oldNode.TypeID),
				OldValue: oldNode.TypeID,
				Message:  fmt.Sprintf("node type_id=%d removed", oldNode.TypeID),
			})
			continue
		}
		out = append(out, diffNodeBody(oldNode, newNode)...)
	}

	oldReservedTypes := int32Set(reservedTypeIDs(oldR))
	newIDs := make([]int32, 0, len(newByID))
	for id := range newByID {
		newIDs = append(newIDs, id)
	}
	sortInt32(newIDs)
	for _, id := range newIDs {
		if _, ok := oldByID[id]; ok {
			continue
		}
		newNode := newByID[id]
		// ADR-032: a baseline-reserved type_id reappearing as a live type
		// is a BREAKING reuse, not a benign add.
		if _, reserved := oldReservedTypes[id]; reserved {
			out = append(out, Change{
				Kind:     ChangeKindTypeIDReused,
				Path:     fmt.Sprintf("node:%d", newNode.TypeID),
				NewValue: newNode.TypeID,
				Message: fmt.Sprintf(
					"type_id=%d was reserved in the baseline but is re-introduced as a live type (id reuse)",
					newNode.TypeID,
				),
			})
			continue
		}
		out = append(out, Change{
			Kind:     ChangeKindNodeAdded,
			Path:     fmt.Sprintf("node:%d", newNode.TypeID),
			NewValue: newNode.TypeID,
			Message:  fmt.Sprintf("node type_id=%d added", newNode.TypeID),
		})
	}
	return out
}

// reservedTypeIDs / reservedEdgeIDs read the schema-level tombstone lists
// off a registry, tolerating a nil registry.
func reservedTypeIDs(r *Registry) []int32 {
	if r == nil {
		return nil
	}
	return r.ReservedTypeIDs()
}

func reservedEdgeIDs(r *Registry) []int32 {
	if r == nil {
		return nil
	}
	return r.ReservedEdgeIDs()
}

func int32Set(ids []int32) map[int32]struct{} {
	m := make(map[int32]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}

func uint32Set(ids []uint32) map[uint32]struct{} {
	m := make(map[uint32]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}

func diffNodeBody(oldNode, newNode *NodeTypeDef) []Change {
	var out []Change
	base := fmt.Sprintf("node:%d", newNode.TypeID)

	if !subjectFieldEqual(oldNode.SubjectField, newNode.SubjectField) {
		out = append(out, Change{
			Kind:     ChangeKindSubjectFieldChanged,
			Path:     base,
			OldValue: derefUint32(oldNode.SubjectField),
			NewValue: derefUint32(newNode.SubjectField),
			Message: fmt.Sprintf(
				"node type_id=%d changed subject_field from %v to %v",
				newNode.TypeID, derefUint32(oldNode.SubjectField), derefUint32(newNode.SubjectField),
			),
		})
	}

	out = append(out, diffDataPolicy(oldNode.DataPolicy, newNode.DataPolicy, base, newNode.TypeID)...)

	// Fields
	out = append(out, diffFields(oldNode, newNode)...)

	// Composite unique
	out = append(out, diffCompositeUnique(oldNode, newNode, base)...)

	return out
}

func diffFields(oldNode, newNode *NodeTypeDef) []Change {
	var out []Change
	oldFields := map[uint32]*FieldDef{}
	for i := range oldNode.Fields {
		f := &oldNode.Fields[i]
		oldFields[f.FieldID] = f
	}
	newFields := map[uint32]*FieldDef{}
	for i := range newNode.Fields {
		f := &newNode.Fields[i]
		newFields[f.FieldID] = f
	}

	// Name-free per ADR-031: fields are keyed solely by field_id. A
	// field present only in old is removed; only in new is added; present
	// on both → body diff. A field_id reassignment is a remove+add — the
	// historical name-keyed FIELD_ID_CHANGED / FIELD_RENAMED detection is
	// gone because the registry no longer carries field names.
	oldIDs := make([]uint32, 0, len(oldFields))
	for id := range oldFields {
		oldIDs = append(oldIDs, id)
	}
	sortUint32(oldIDs)
	for _, id := range oldIDs {
		of := oldFields[id]
		nf, sameID := newFields[id]
		if sameID {
			out = append(out, diffFieldBody(newNode.TypeID, of, nf)...)
			continue
		}
		out = append(out, Change{
			Kind:     ChangeKindFieldRemoved,
			Path:     fmt.Sprintf("node:%d.field:%d", newNode.TypeID, of.FieldID),
			OldValue: of.FieldID,
			Message: fmt.Sprintf(
				"field_id=%d removed from node type_id=%d", of.FieldID, newNode.TypeID,
			),
		})
	}

	oldReserved := uint32Set(oldNode.ReservedFieldIDs)
	newIDs := make([]uint32, 0, len(newFields))
	for id := range newFields {
		newIDs = append(newIDs, id)
	}
	sortUint32(newIDs)
	for _, id := range newIDs {
		nf := newFields[id]
		if _, ok := oldFields[id]; ok {
			continue
		}
		// ADR-032: a baseline-reserved field_id reappearing as a live
		// field is a BREAKING reuse — historic rows keyed by that id mean
		// something else.
		if _, reserved := oldReserved[id]; reserved {
			out = append(out, Change{
				Kind:     ChangeKindFieldIDReused,
				Path:     fmt.Sprintf("node:%d.field:%d", newNode.TypeID, nf.FieldID),
				NewValue: nf.FieldID,
				Message: fmt.Sprintf(
					"field_id=%d on node type_id=%d was reserved in the baseline but is re-introduced as a live field (id reuse)",
					nf.FieldID, newNode.TypeID,
				),
			})
			continue
		}
		out = append(out, Change{
			Kind:     ChangeKindFieldAdded,
			Path:     fmt.Sprintf("node:%d.field:%d", newNode.TypeID, nf.FieldID),
			NewValue: nf.FieldID,
			Message: fmt.Sprintf(
				"field_id=%d (kind=%s) added to node type_id=%d",
				nf.FieldID, nf.Kind, newNode.TypeID,
			),
		})
	}
	return out
}

func diffFieldBody(typeID int32, of, nf *FieldDef) []Change {
	var out []Change
	base := fmt.Sprintf("node:%d.field:%d", typeID, nf.FieldID)
	if of.Kind != nf.Kind {
		out = append(out, Change{
			Kind:     ChangeKindFieldKindChanged,
			Path:     base,
			OldValue: string(of.Kind),
			NewValue: string(nf.Kind),
			Message: fmt.Sprintf(
				"field_id=%d on node type_id=%d changed kind from %s to %s", nf.FieldID, typeID, of.Kind, nf.Kind,
			),
		})
	}
	if of.Required != nf.Required {
		if !of.Required && nf.Required {
			out = append(out, Change{
				Kind:     ChangeKindFieldRequiredTightened,
				Path:     base,
				OldValue: of.Required,
				NewValue: nf.Required,
				Message: fmt.Sprintf(
					"field_id=%d on node type_id=%d became required (existing rows that omit it will fail validation)",
					nf.FieldID, typeID,
				),
			})
		} else {
			out = append(out, Change{
				Kind:     ChangeKindFieldRequiredLoosened,
				Path:     base,
				OldValue: of.Required,
				NewValue: nf.Required,
				Message:  fmt.Sprintf("field_id=%d on node type_id=%d no longer required", nf.FieldID, typeID),
			})
		}
	}
	if of.Unique != nf.Unique {
		if !of.Unique && nf.Unique {
			out = append(out, Change{
				Kind:     ChangeKindFieldUniqueAdded,
				Path:     base,
				OldValue: of.Unique,
				NewValue: nf.Unique,
				Message: fmt.Sprintf(
					"field_id=%d on node type_id=%d became unique (historical data may already violate)",
					nf.FieldID, typeID,
				),
			})
		} else {
			out = append(out, Change{
				Kind:     ChangeKindFieldUniqueRemoved,
				Path:     base,
				OldValue: of.Unique,
				NewValue: nf.Unique,
				Message:  fmt.Sprintf("field_id=%d on node type_id=%d no longer unique", nf.FieldID, typeID),
			})
		}
	}
	if of.Indexed != nf.Indexed {
		kind := ChangeKindFieldIndexedRemoved
		msg := fmt.Sprintf("field_id=%d on node type_id=%d no longer indexed", nf.FieldID, typeID)
		if nf.Indexed {
			kind = ChangeKindFieldIndexedAdded
			msg = fmt.Sprintf("field_id=%d on node type_id=%d now indexed (additive query index)", nf.FieldID, typeID)
		}
		out = append(out, Change{
			Kind:     kind,
			Path:     base,
			OldValue: of.Indexed,
			NewValue: nf.Indexed,
			Message:  msg,
		})
	}
	if of.Searchable != nf.Searchable {
		kind := ChangeKindFieldSearchableRemoved
		msg := fmt.Sprintf("field_id=%d on node type_id=%d no longer searchable", nf.FieldID, typeID)
		if nf.Searchable {
			kind = ChangeKindFieldSearchableAdded
			msg = fmt.Sprintf("field_id=%d on node type_id=%d now searchable (additive FTS index)", nf.FieldID, typeID)
		}
		out = append(out, Change{
			Kind:     kind,
			Path:     base,
			OldValue: of.Searchable,
			NewValue: nf.Searchable,
			Message:  msg,
		})
	}
	if of.Deprecated != nf.Deprecated {
		kind := ChangeKindFieldUndeprecated
		msg := fmt.Sprintf("field_id=%d on node type_id=%d un-deprecated", nf.FieldID, typeID)
		if nf.Deprecated {
			kind = ChangeKindFieldDeprecated
			msg = fmt.Sprintf("field_id=%d on node type_id=%d marked deprecated", nf.FieldID, typeID)
		}
		out = append(out, Change{
			Kind:     kind,
			Path:     base,
			OldValue: of.Deprecated,
			NewValue: nf.Deprecated,
			Message:  msg,
		})
	}
	if of.PII != nf.PII {
		out = append(out, Change{
			Kind:     ChangeKindFieldPIIToggled,
			Path:     base,
			OldValue: of.PII,
			NewValue: nf.PII,
			Message: fmt.Sprintf(
				"field_id=%d on node type_id=%d pii flag toggled (%v → %v)", nf.FieldID, typeID, of.PII, nf.PII,
			),
		})
	}
	if !refTypeEqual(of.RefTypeID, nf.RefTypeID) {
		out = append(out, Change{
			Kind:     ChangeKindFieldRefTypeChanged,
			Path:     base,
			OldValue: derefInt32(of.RefTypeID),
			NewValue: derefInt32(nf.RefTypeID),
			Message: fmt.Sprintf(
				"field_id=%d on node type_id=%d changed ref_type_id from %v to %v",
				nf.FieldID, typeID, derefInt32(of.RefTypeID), derefInt32(nf.RefTypeID),
			),
		})
	}
	// Enum value comparisons (only meaningful when kind == enum).
	if nf.Kind == KindEnum || of.Kind == KindEnum {
		out = append(out, diffEnumValues(typeID, nf.FieldID, of.EnumValues, nf.EnumValues)...)
	}
	return out
}

func diffEnumValues(typeID int32, fieldID uint32, oldVals, newVals []string) []Change {
	var out []Change
	base := fmt.Sprintf("node:%d.field:%d.enum", typeID, fieldID)
	// Reordering: any value present in both lists at different
	// positions is a reorder (integer reuse).
	oldPos := map[string]int{}
	for i, v := range oldVals {
		oldPos[v] = i
	}
	newPos := map[string]int{}
	for i, v := range newVals {
		newPos[v] = i
	}
	// Reorder detection: any value common to both at a different index.
	reordered := false
	for v, oi := range oldPos {
		if ni, ok := newPos[v]; ok && oi != ni {
			reordered = true
			break
		}
	}
	if reordered {
		out = append(out, Change{
			Kind:     ChangeKindEnumValueReordered,
			Path:     base,
			OldValue: oldVals,
			NewValue: newVals,
			Message: fmt.Sprintf(
				"enum values for field_id=%d on node type_id=%d reordered (integer reuse — old WAL events now resolve to a different value)",
				fieldID, typeID,
			),
		})
	}
	for _, v := range oldVals {
		if _, ok := newPos[v]; !ok {
			out = append(out, Change{
				Kind:     ChangeKindEnumValueRemoved,
				Path:     fmt.Sprintf("%s:%s", base, v),
				OldValue: v,
				Message: fmt.Sprintf(
					"enum value %q removed from node type_id=%d field_id=%d", v, typeID, fieldID,
				),
			})
		}
	}
	for _, v := range newVals {
		if _, ok := oldPos[v]; !ok {
			out = append(out, Change{
				Kind:     ChangeKindEnumValueAdded,
				Path:     fmt.Sprintf("%s:%s", base, v),
				NewValue: v,
				Message: fmt.Sprintf(
					"enum value %q added to node type_id=%d field_id=%d", v, typeID, fieldID,
				),
			})
		}
	}
	return out
}

// diffCompositeUnique compares composite-unique constraints keyed by
// their field_id-tuple signature (name-free, ADR-031). A constraint
// present on one side only is added/removed; because the tuple IS the
// identity, there is no "changed" case anymore — a different tuple is a
// distinct constraint (one removed, one added). The ChangeKind enum
// retains COMPOSITE_UNIQUE_CHANGED for output stability but it is no
// longer emitted.
func diffCompositeUnique(oldNode, newNode *NodeTypeDef, base string) []Change {
	var out []Change
	oldBySig := map[string][]uint32{}
	for _, cu := range oldNode.CompositeUnique {
		oldBySig[signature(cu.FieldIDs)] = cu.FieldIDs
	}
	newBySig := map[string][]uint32{}
	for _, cu := range newNode.CompositeUnique {
		newBySig[signature(cu.FieldIDs)] = cu.FieldIDs
	}
	oldSigs := make([]string, 0, len(oldBySig))
	for k := range oldBySig {
		oldSigs = append(oldSigs, k)
	}
	sort.Strings(oldSigs)
	for _, sig := range oldSigs {
		if _, ok := newBySig[sig]; ok {
			continue
		}
		out = append(out, Change{
			Kind:     ChangeKindCompositeUniqueRemoved,
			Path:     fmt.Sprintf("%s.composite_unique:%s", base, sig),
			OldValue: oldBySig[sig],
			Message: fmt.Sprintf(
				"composite_unique %s on node type_id=%d removed", sig, oldNode.TypeID,
			),
		})
	}
	newSigs := make([]string, 0, len(newBySig))
	for k := range newBySig {
		newSigs = append(newSigs, k)
	}
	sort.Strings(newSigs)
	for _, sig := range newSigs {
		if _, ok := oldBySig[sig]; ok {
			continue
		}
		out = append(out, Change{
			Kind:     ChangeKindCompositeUniqueAdded,
			Path:     fmt.Sprintf("%s.composite_unique:%s", base, sig),
			NewValue: newBySig[sig],
			Message: fmt.Sprintf(
				"composite_unique %s on node type_id=%d added — existing rows may violate",
				sig, newNode.TypeID,
			),
		})
	}
	return out
}

// -----------------------------------------------------------------------------
// Edge diff
// -----------------------------------------------------------------------------

func edgesByID(r *Registry) map[int32]*EdgeTypeDef {
	if r == nil {
		return map[int32]*EdgeTypeDef{}
	}
	s := r.load()
	out := make(map[int32]*EdgeTypeDef, len(s.edges))
	for id, e := range s.edges {
		out[id] = e
	}
	return out
}

func diffEdges(oldR, newR *Registry) []Change {
	var out []Change
	oldByID := edgesByID(oldR)
	newByID := edgesByID(newR)

	oldIDs := make([]int32, 0, len(oldByID))
	for id := range oldByID {
		oldIDs = append(oldIDs, id)
	}
	sortInt32(oldIDs)
	for _, id := range oldIDs {
		oe := oldByID[id]
		ne, ok := newByID[id]
		if !ok {
			out = append(out, Change{
				Kind:     ChangeKindEdgeRemoved,
				Path:     fmt.Sprintf("edge:%d", oe.EdgeID),
				OldValue: oe.EdgeID,
				Message: fmt.Sprintf(
					"edge type edge_id=%d removed", oe.EdgeID,
				),
			})
			continue
		}
		out = append(out, diffEdgeBody(oe, ne)...)
	}
	oldReservedEdges := int32Set(reservedEdgeIDs(oldR))
	newIDs := make([]int32, 0, len(newByID))
	for id := range newByID {
		newIDs = append(newIDs, id)
	}
	sortInt32(newIDs)
	for _, id := range newIDs {
		if _, ok := oldByID[id]; ok {
			continue
		}
		ne := newByID[id]
		// ADR-032: a baseline-reserved edge_id reappearing as a live edge
		// is a BREAKING reuse.
		if _, reserved := oldReservedEdges[id]; reserved {
			out = append(out, Change{
				Kind:     ChangeKindEdgeIDReused,
				Path:     fmt.Sprintf("edge:%d", ne.EdgeID),
				NewValue: ne.EdgeID,
				Message: fmt.Sprintf(
					"edge_id=%d was reserved in the baseline but is re-introduced as a live edge (id reuse)",
					ne.EdgeID,
				),
			})
			continue
		}
		out = append(out, Change{
			Kind:     ChangeKindEdgeAdded,
			Path:     fmt.Sprintf("edge:%d", ne.EdgeID),
			NewValue: ne.EdgeID,
			Message: fmt.Sprintf(
				"edge type edge_id=%d added", ne.EdgeID,
			),
		})
	}
	return out
}

func diffEdgeBody(oe, ne *EdgeTypeDef) []Change {
	var out []Change
	base := fmt.Sprintf("edge:%d", ne.EdgeID)
	if oe.FromTypeID != ne.FromTypeID {
		out = append(out, Change{
			Kind:     ChangeKindEdgeFromTypeChanged,
			Path:     base,
			OldValue: oe.FromTypeID,
			NewValue: ne.FromTypeID,
			Message: fmt.Sprintf(
				"edge edge_id=%d from_type_id changed from %d to %d", ne.EdgeID, oe.FromTypeID, ne.FromTypeID,
			),
		})
	}
	if oe.ToTypeID != ne.ToTypeID {
		out = append(out, Change{
			Kind:     ChangeKindEdgeToTypeChanged,
			Path:     base,
			OldValue: oe.ToTypeID,
			NewValue: ne.ToTypeID,
			Message: fmt.Sprintf(
				"edge edge_id=%d to_type_id changed from %d to %d", ne.EdgeID, oe.ToTypeID, ne.ToTypeID,
			),
		})
	}
	if oe.UniquePerFrom != ne.UniquePerFrom {
		if !oe.UniquePerFrom && ne.UniquePerFrom {
			out = append(out, Change{
				Kind:     ChangeKindEdgeUniquePerFromAdded,
				Path:     base,
				OldValue: oe.UniquePerFrom,
				NewValue: ne.UniquePerFrom,
				Message: fmt.Sprintf(
					"edge edge_id=%d unique_per_from added (historical edges may violate)", ne.EdgeID,
				),
			})
		} else {
			out = append(out, Change{
				Kind:     ChangeKindEdgeUniquePerFromRemoved,
				Path:     base,
				OldValue: oe.UniquePerFrom,
				NewValue: ne.UniquePerFrom,
				Message:  fmt.Sprintf("edge edge_id=%d unique_per_from removed", ne.EdgeID),
			})
		}
	}
	if normaliseOnExit(oe.OnSubjectExit) != normaliseOnExit(ne.OnSubjectExit) {
		out = append(out, Change{
			Kind:     ChangeKindOnSubjectExitChanged,
			Path:     base,
			OldValue: string(normaliseOnExit(oe.OnSubjectExit)),
			NewValue: string(normaliseOnExit(ne.OnSubjectExit)),
			Message: fmt.Sprintf(
				"edge edge_id=%d on_subject_exit changed from %s to %s (GDPR delete semantics shift)",
				ne.EdgeID, normaliseOnExit(oe.OnSubjectExit), normaliseOnExit(ne.OnSubjectExit),
			),
		})
	}
	return out
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func subjectFieldEqual(a, b *uint32) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func derefUint32(p *uint32) any {
	if p == nil {
		return nil
	}
	return *p
}

func refTypeEqual(a, b *int32) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func derefInt32(p *int32) any {
	if p == nil {
		return nil
	}
	return *p
}

func normaliseOnExit(v OnSubjectExit) OnSubjectExit {
	if v == "" {
		return OnSubjectExitBoth
	}
	return v
}

// dataPolicyRank orders data policies from "weakest" (lowest leakage
// risk if downgraded) to "strongest". Used to classify
// tightening vs loosening. The ordering is documented on each constant:
//
//	ephemeral < business < personal < healthcare < financial < audit
//
// A move toward the right is a "tightening" (non-breaking — encryption
// strengthens). A move toward the left is a "loosening" (BREAKING —
// historical rows leak downward).
func dataPolicyRank(p DataPolicy) int {
	switch p {
	case DataPolicyEphemeral:
		return 0
	case DataPolicyBusiness:
		return 1
	case DataPolicyPersonal:
		return 2
	case DataPolicyHealthcare:
		return 3
	case DataPolicyFinancial:
		return 4
	case DataPolicyAudit:
		return 5
	}
	// Unknown / empty — treat as the default (personal) to avoid
	// false-positive breaks on a missing field.
	return 2
}

func diffDataPolicy(oldP, newP *DataPolicy, base string, typeID int32) []Change {
	if oldP == nil && newP == nil {
		return nil
	}
	var a, b DataPolicy
	if oldP != nil {
		a = *oldP
	}
	if newP != nil {
		b = *newP
	}
	if a == b {
		return nil
	}
	ra, rb := dataPolicyRank(a), dataPolicyRank(b)
	if rb > ra {
		return []Change{{
			Kind:     ChangeKindDataPolicyTightened,
			Path:     base,
			OldValue: string(a),
			NewValue: string(b),
			Message: fmt.Sprintf(
				"node type_id=%d data_policy tightened from %s to %s", typeID, a, b,
			),
		}}
	}
	return []Change{{
		Kind:     ChangeKindDataPolicyLoosened,
		Path:     base,
		OldValue: string(a),
		NewValue: string(b),
		Message: fmt.Sprintf(
			"node type_id=%d data_policy loosened from %s to %s — historical rows lose protection",
			typeID, a, b,
		),
	}}
}

func sortUint32(s []uint32) {
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
}

// RenderText writes a human-readable report for cs to a strings.Builder.
// Used by CLI text-format output and PR-comment text.
func RenderText(cs []Change, baselineLabel string) string {
	var sb strings.Builder
	breakingN := CountBreaking(cs)
	nonBreakingN := len(cs) - breakingN
	if breakingN > 0 {
		fmt.Fprintf(&sb, "BREAKING: %d breaking change(s) detected", breakingN)
	} else if len(cs) > 0 {
		fmt.Fprintf(&sb, "OK: %d non-breaking change(s) detected", len(cs))
	} else {
		fmt.Fprintf(&sb, "OK: no schema changes detected")
	}
	if baselineLabel != "" {
		fmt.Fprintf(&sb, " against %s", baselineLabel)
	}
	sb.WriteString("\n\n")
	if breakingN > 0 {
		for _, c := range cs {
			if c.Breaking {
				fmt.Fprintln(&sb, "  "+c.String())
			}
		}
		sb.WriteString("\n")
	}
	if nonBreakingN > 0 {
		fmt.Fprintf(&sb, "Non-breaking changes (informational): %d\n", nonBreakingN)
		for _, c := range cs {
			if !c.Breaking {
				fmt.Fprintln(&sb, "  "+c.String())
			}
		}
	}
	return sb.String()
}
