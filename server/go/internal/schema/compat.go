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
	ChangeKindNodeRemoved         // BREAKING — WAL events referencing the type become unreplayable
	ChangeKindNodeRenamed         // non-breaking — type_id is the on-disk key
	ChangeKindTypeIDChanged       // BREAKING — same name, different type_id == new type
	ChangeKindSubjectFieldChanged // BREAKING — GDPR routing changes
	ChangeKindDataPolicyTightened // non-breaking — encryption tier strengthens
	ChangeKindDataPolicyLoosened  // BREAKING — historical data leaks downward

	// Field-level
	ChangeKindFieldAdded
	ChangeKindFieldRemoved           // BREAKING — historic rows still carry the field
	ChangeKindFieldIDChanged         // BREAKING — field_id reassignment corrupts existing data
	ChangeKindFieldKindChanged       // BREAKING — type-coercion of stored bytes is unsafe
	ChangeKindFieldRenamed           // non-breaking — field_id is the on-disk key (CLAUDE.md invariant #6)
	ChangeKindFieldRequiredTightened // BREAKING — existing rows that omit the field now fail
	ChangeKindFieldRequiredLoosened  // non-breaking — looser is always safe
	ChangeKindFieldUniqueAdded       // BREAKING — historical data may already violate
	ChangeKindFieldUniqueRemoved     // non-breaking
	ChangeKindFieldDeprecated        // non-breaking — informational marker
	ChangeKindFieldUndeprecated      // non-breaking
	ChangeKindFieldRefTypeChanged    // BREAKING — dangling references
	ChangeKindFieldPIIToggled        // non-breaking — informational, but flagged

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
	ChangeKindEdgeRemoved              // BREAKING — historical edges orphaned
	ChangeKindEdgeFromTypeChanged      // BREAKING — edge semantics shift
	ChangeKindEdgeToTypeChanged        // BREAKING — edge semantics shift
	ChangeKindEdgeUniquePerFromAdded   // BREAKING — historical edges may already violate
	ChangeKindEdgeUniquePerFromRemoved // non-breaking
	ChangeKindOnSubjectExitChanged     // BREAKING — GDPR delete semantics shift
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
var breakingKinds = map[ChangeKind]bool{
	ChangeKindNodeRemoved:            true,
	ChangeKindTypeIDChanged:          true,
	ChangeKindSubjectFieldChanged:    true,
	ChangeKindDataPolicyLoosened:     true,
	ChangeKindFieldRemoved:           true,
	ChangeKindFieldIDChanged:         true,
	ChangeKindFieldKindChanged:       true,
	ChangeKindFieldRequiredTightened: true,
	ChangeKindFieldUniqueAdded:       true,
	ChangeKindFieldRefTypeChanged:    true,
	ChangeKindEnumValueRemoved:       true,
	ChangeKindEnumValueReordered:     true,
	ChangeKindCompositeUniqueAdded:   true,
	ChangeKindCompositeUniqueChanged: true,
	ChangeKindEdgeRemoved:            true,
	ChangeKindEdgeFromTypeChanged:    true,
	ChangeKindEdgeToTypeChanged:      true,
	ChangeKindEdgeUniquePerFromAdded: true,
	ChangeKindOnSubjectExitChanged:   true,
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
	out := make(map[int32]*NodeTypeDef, len(r.nodes))
	for id, n := range r.nodes {
		out[id] = n
	}
	return out
}

func diffNodes(oldR, newR *Registry) []Change {
	var out []Change
	oldByID := nodesByID(oldR)
	newByID := nodesByID(newR)

	// First, identify name pairs that exist on both sides with different
	// type_ids. These get a single canonical TYPE_ID_CHANGED entry and
	// must be suppressed from the id-keyed remove/add passes below
	// (otherwise we triple-count: NODE_REMOVED + NODE_ADDED +
	// TYPE_ID_CHANGED for the same logical operation).
	oldByName := map[string]*NodeTypeDef{}
	for _, n := range oldByID {
		oldByName[n.Name] = n
	}
	newByName := map[string]*NodeTypeDef{}
	for _, n := range newByID {
		newByName[n.Name] = n
	}
	// suppressedOldIDs / suppressedNewIDs hold the type_ids that should
	// NOT produce a NODE_REMOVED / NODE_ADDED event because they're
	// already accounted for by a TYPE_ID_CHANGED match on the name.
	suppressedOldIDs := map[int32]struct{}{}
	suppressedNewIDs := map[int32]struct{}{}
	for name, oldNode := range oldByName {
		newNode, ok := newByName[name]
		if !ok {
			continue
		}
		if oldNode.TypeID != newNode.TypeID {
			suppressedOldIDs[oldNode.TypeID] = struct{}{}
			suppressedNewIDs[newNode.TypeID] = struct{}{}
		}
	}

	// Sort old IDs for deterministic traversal.
	oldIDs := make([]int32, 0, len(oldByID))
	for id := range oldByID {
		oldIDs = append(oldIDs, id)
	}
	sortInt32(oldIDs)
	for _, id := range oldIDs {
		oldNode := oldByID[id]
		newNode, ok := newByID[id]
		if !ok {
			if _, suppress := suppressedOldIDs[id]; suppress {
				// Name still present in new under a different type_id —
				// reported as TYPE_ID_CHANGED below.
				continue
			}
			out = append(out, Change{
				Kind:     ChangeKindNodeRemoved,
				Path:     fmt.Sprintf("node:%s", oldNode.Name),
				OldValue: oldNode.Name,
				Message:  fmt.Sprintf("node type %q (type_id=%d) removed", oldNode.Name, oldNode.TypeID),
			})
			continue
		}
		out = append(out, diffNodeBody(oldNode, newNode)...)
	}

	newIDs := make([]int32, 0, len(newByID))
	for id := range newByID {
		newIDs = append(newIDs, id)
	}
	sortInt32(newIDs)
	for _, id := range newIDs {
		if _, ok := oldByID[id]; ok {
			continue
		}
		if _, suppress := suppressedNewIDs[id]; suppress {
			// Name existed before under a different type_id — reported
			// as TYPE_ID_CHANGED below.
			continue
		}
		newNode := newByID[id]
		out = append(out, Change{
			Kind:     ChangeKindNodeAdded,
			Path:     fmt.Sprintf("node:%s", newNode.Name),
			NewValue: newNode.Name,
			Message:  fmt.Sprintf("node type %q (type_id=%d) added", newNode.Name, newNode.TypeID),
		})
	}

	// Emit TYPE_ID_CHANGED — a node with the same name on both sides
	// but a different type_id. This is materially different from
	// "remove-then-add" because users typically don't intend type-id
	// drift; we surface it as its own breaking rule and suppress the
	// id-keyed remove/add for the same name (above).
	for name, oldNode := range oldByName {
		newNode, ok := newByName[name]
		if !ok {
			continue
		}
		if oldNode.TypeID != newNode.TypeID {
			out = append(out, Change{
				Kind:     ChangeKindTypeIDChanged,
				Path:     fmt.Sprintf("node:%s", name),
				OldValue: oldNode.TypeID,
				NewValue: newNode.TypeID,
				Message: fmt.Sprintf(
					"node type %q changed type_id from %d to %d", name, oldNode.TypeID, newNode.TypeID,
				),
			})
		}
	}
	return out
}

func diffNodeBody(oldNode, newNode *NodeTypeDef) []Change {
	var out []Change
	base := fmt.Sprintf("node:%s", newNode.Name)

	if oldNode.Name != newNode.Name {
		out = append(out, Change{
			Kind:     ChangeKindNodeRenamed,
			Path:     base,
			OldValue: oldNode.Name,
			NewValue: newNode.Name,
			Message:  fmt.Sprintf("node type renamed from %q to %q (type_id=%d)", oldNode.Name, newNode.Name, newNode.TypeID),
		})
	}

	if !subjectFieldEqual(oldNode.SubjectField, newNode.SubjectField) {
		out = append(out, Change{
			Kind:     ChangeKindSubjectFieldChanged,
			Path:     base,
			OldValue: derefStr(oldNode.SubjectField),
			NewValue: derefStr(newNode.SubjectField),
			Message: fmt.Sprintf(
				"node type %q changed subject_field from %q to %q",
				newNode.Name, derefStr(oldNode.SubjectField), derefStr(newNode.SubjectField),
			),
		})
	}

	out = append(out, diffDataPolicy(oldNode.DataPolicy, newNode.DataPolicy, base, newNode.Name)...)

	// Fields
	out = append(out, diffFields(oldNode, newNode)...)

	// Composite unique
	out = append(out, diffCompositeUnique(oldNode, newNode, base)...)

	return out
}

func diffFields(oldNode, newNode *NodeTypeDef) []Change {
	var out []Change
	oldFields := map[uint32]*FieldDef{}
	oldFieldsByName := map[string]*FieldDef{}
	for i := range oldNode.Fields {
		f := &oldNode.Fields[i]
		oldFields[f.FieldID] = f
		oldFieldsByName[f.Name] = f
	}
	newFields := map[uint32]*FieldDef{}
	newFieldsByName := map[string]*FieldDef{}
	for i := range newNode.Fields {
		f := &newNode.Fields[i]
		newFields[f.FieldID] = f
		newFieldsByName[f.Name] = f
	}

	// Removed fields (field_id present in old, absent in new) AND the
	// same name not present under a different id.
	oldIDs := make([]uint32, 0, len(oldFields))
	for id := range oldFields {
		oldIDs = append(oldIDs, id)
	}
	sortUint32(oldIDs)
	for _, id := range oldIDs {
		of := oldFields[id]
		nf, sameID := newFields[id]
		nfByName, sameName := newFieldsByName[of.Name]
		switch {
		case sameID && nf.Name == of.Name:
			// Same id, same name → check body diff.
			out = append(out, diffFieldBody(newNode.Name, of, nf)...)
		case sameID && nf.Name != of.Name:
			// Same id, different name → field rename. Field IDs are the
			// on-disk key (CLAUDE.md invariant #6), so the rename is a
			// safe, non-breaking metadata change. Emit FIELD_RENAMED and
			// still run the body diff so genuine kind/required/etc.
			// transitions on top of the rename still surface.
			out = append(out, Change{
				Kind:     ChangeKindFieldRenamed,
				Path:     fmt.Sprintf("node:%s.field:%s", newNode.Name, nf.Name),
				OldValue: of.Name,
				NewValue: nf.Name,
				Message: fmt.Sprintf(
					"field_id %d on %q renamed from %q to %q (field_id is the on-disk key — non-breaking)",
					id, newNode.Name, of.Name, nf.Name,
				),
			})
			out = append(out, diffFieldBody(newNode.Name, of, nf)...)
		case !sameID && sameName:
			// Field with same name exists under different id → that's
			// the canonical "FIELD_ID_CHANGED" break.
			out = append(out, Change{
				Kind:     ChangeKindFieldIDChanged,
				Path:     fmt.Sprintf("node:%s.field:%s", newNode.Name, of.Name),
				OldValue: of.FieldID,
				NewValue: nfByName.FieldID,
				Message: fmt.Sprintf(
					"field %q on %q changed field_id from %d to %d",
					of.Name, newNode.Name, of.FieldID, nfByName.FieldID,
				),
			})
			out = append(out, diffFieldBody(newNode.Name, of, nfByName)...)
		default:
			// Field removed.
			out = append(out, Change{
				Kind:     ChangeKindFieldRemoved,
				Path:     fmt.Sprintf("node:%s.field:%s", newNode.Name, of.Name),
				OldValue: of.Name,
				Message: fmt.Sprintf(
					"field %q (field_id=%d) removed from %q", of.Name, of.FieldID, newNode.Name,
				),
			})
		}
	}

	// Added fields (in new, not previously present by id and not the
	// new home of a renamed-id field).
	newIDs := make([]uint32, 0, len(newFields))
	for id := range newFields {
		newIDs = append(newIDs, id)
	}
	sortUint32(newIDs)
	for _, id := range newIDs {
		nf := newFields[id]
		if of, ok := oldFields[id]; ok && of.Name == nf.Name {
			continue
		}
		// Suppress spurious FIELD_ADDED when the field_id already
		// existed under a different name (handled above as
		// FIELD_RENAMED) — otherwise the renamed-to name would
		// double-count as both a rename and a new add.
		if _, idExisted := oldFields[id]; idExisted {
			continue
		}
		if _, oldHasName := oldFieldsByName[nf.Name]; oldHasName {
			continue
		}
		out = append(out, Change{
			Kind:     ChangeKindFieldAdded,
			Path:     fmt.Sprintf("node:%s.field:%s", newNode.Name, nf.Name),
			NewValue: nf.Name,
			Message: fmt.Sprintf(
				"field %q (field_id=%d, kind=%s) added to %q",
				nf.Name, nf.FieldID, nf.Kind, newNode.Name,
			),
		})
	}
	return out
}

func diffFieldBody(nodeName string, of, nf *FieldDef) []Change {
	var out []Change
	base := fmt.Sprintf("node:%s.field:%s", nodeName, nf.Name)
	if of.Kind != nf.Kind {
		out = append(out, Change{
			Kind:     ChangeKindFieldKindChanged,
			Path:     base,
			OldValue: string(of.Kind),
			NewValue: string(nf.Kind),
			Message: fmt.Sprintf(
				"field %q on %q changed kind from %s to %s", nf.Name, nodeName, of.Kind, nf.Kind,
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
					"field %q on %q became required (existing rows that omit it will fail validation)",
					nf.Name, nodeName,
				),
			})
		} else {
			out = append(out, Change{
				Kind:     ChangeKindFieldRequiredLoosened,
				Path:     base,
				OldValue: of.Required,
				NewValue: nf.Required,
				Message:  fmt.Sprintf("field %q on %q no longer required", nf.Name, nodeName),
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
					"field %q on %q became unique (historical data may already violate)",
					nf.Name, nodeName,
				),
			})
		} else {
			out = append(out, Change{
				Kind:     ChangeKindFieldUniqueRemoved,
				Path:     base,
				OldValue: of.Unique,
				NewValue: nf.Unique,
				Message:  fmt.Sprintf("field %q on %q no longer unique", nf.Name, nodeName),
			})
		}
	}
	if of.Deprecated != nf.Deprecated {
		kind := ChangeKindFieldUndeprecated
		msg := fmt.Sprintf("field %q on %q un-deprecated", nf.Name, nodeName)
		if nf.Deprecated {
			kind = ChangeKindFieldDeprecated
			msg = fmt.Sprintf("field %q on %q marked deprecated", nf.Name, nodeName)
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
				"field %q on %q pii flag toggled (%v → %v)", nf.Name, nodeName, of.PII, nf.PII,
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
				"field %q on %q changed ref_type_id from %v to %v",
				nf.Name, nodeName, derefInt32(of.RefTypeID), derefInt32(nf.RefTypeID),
			),
		})
	}
	// Enum value comparisons (only meaningful when kind == enum).
	if nf.Kind == KindEnum || of.Kind == KindEnum {
		out = append(out, diffEnumValues(nodeName, nf.Name, of.EnumValues, nf.EnumValues)...)
	}
	return out
}

func diffEnumValues(nodeName, fieldName string, oldVals, newVals []string) []Change {
	var out []Change
	base := fmt.Sprintf("node:%s.field:%s.enum", nodeName, fieldName)
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
				"enum values for %q on %q reordered (integer reuse — old WAL events now resolve to a different value)",
				fieldName, nodeName,
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
					"enum value %q removed from %s.%s", v, nodeName, fieldName,
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
					"enum value %q added to %s.%s", v, nodeName, fieldName,
				),
			})
		}
	}
	return out
}

func diffCompositeUnique(oldNode, newNode *NodeTypeDef, base string) []Change {
	var out []Change
	oldByName := map[string]CompositeUniqueDef{}
	for _, cu := range oldNode.CompositeUnique {
		oldByName[cu.Name] = cu
	}
	newByName := map[string]CompositeUniqueDef{}
	for _, cu := range newNode.CompositeUnique {
		newByName[cu.Name] = cu
	}
	oldNames := make([]string, 0, len(oldByName))
	for k := range oldByName {
		oldNames = append(oldNames, k)
	}
	sort.Strings(oldNames)
	for _, name := range oldNames {
		ocu := oldByName[name]
		ncu, ok := newByName[name]
		if !ok {
			out = append(out, Change{
				Kind:     ChangeKindCompositeUniqueRemoved,
				Path:     fmt.Sprintf("%s.composite_unique:%s", base, name),
				OldValue: ocu.FieldIDs,
				Message: fmt.Sprintf(
					"composite_unique %q on %q removed", name, oldNode.Name,
				),
			})
			continue
		}
		if signature(ocu.FieldIDs) != signature(ncu.FieldIDs) {
			out = append(out, Change{
				Kind:     ChangeKindCompositeUniqueChanged,
				Path:     fmt.Sprintf("%s.composite_unique:%s", base, name),
				OldValue: ocu.FieldIDs,
				NewValue: ncu.FieldIDs,
				Message: fmt.Sprintf(
					"composite_unique %q on %q changed fields from %v to %v",
					name, oldNode.Name, ocu.FieldIDs, ncu.FieldIDs,
				),
			})
		}
	}
	newNames := make([]string, 0, len(newByName))
	for k := range newByName {
		newNames = append(newNames, k)
	}
	sort.Strings(newNames)
	for _, name := range newNames {
		if _, ok := oldByName[name]; ok {
			continue
		}
		ncu := newByName[name]
		out = append(out, Change{
			Kind:     ChangeKindCompositeUniqueAdded,
			Path:     fmt.Sprintf("%s.composite_unique:%s", base, name),
			NewValue: ncu.FieldIDs,
			Message: fmt.Sprintf(
				"composite_unique %q on %q added (fields=%v) — existing rows may violate",
				name, newNode.Name, ncu.FieldIDs,
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
	out := make(map[int32]*EdgeTypeDef, len(r.edges))
	for id, e := range r.edges {
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
				Path:     fmt.Sprintf("edge:%s", oe.Name),
				OldValue: oe.Name,
				Message: fmt.Sprintf(
					"edge type %q (edge_id=%d) removed", oe.Name, oe.EdgeID,
				),
			})
			continue
		}
		out = append(out, diffEdgeBody(oe, ne)...)
	}
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
		out = append(out, Change{
			Kind:     ChangeKindEdgeAdded,
			Path:     fmt.Sprintf("edge:%s", ne.Name),
			NewValue: ne.Name,
			Message: fmt.Sprintf(
				"edge type %q (edge_id=%d) added", ne.Name, ne.EdgeID,
			),
		})
	}
	return out
}

func diffEdgeBody(oe, ne *EdgeTypeDef) []Change {
	var out []Change
	base := fmt.Sprintf("edge:%s", ne.Name)
	if oe.FromTypeID != ne.FromTypeID {
		out = append(out, Change{
			Kind:     ChangeKindEdgeFromTypeChanged,
			Path:     base,
			OldValue: oe.FromTypeID,
			NewValue: ne.FromTypeID,
			Message: fmt.Sprintf(
				"edge %q from_type_id changed from %d to %d", ne.Name, oe.FromTypeID, ne.FromTypeID,
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
				"edge %q to_type_id changed from %d to %d", ne.Name, oe.ToTypeID, ne.ToTypeID,
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
					"edge %q unique_per_from added (historical edges may violate)", ne.Name,
				),
			})
		} else {
			out = append(out, Change{
				Kind:     ChangeKindEdgeUniquePerFromRemoved,
				Path:     base,
				OldValue: oe.UniquePerFrom,
				NewValue: ne.UniquePerFrom,
				Message:  fmt.Sprintf("edge %q unique_per_from removed", ne.Name),
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
				"edge %q on_subject_exit changed from %s to %s (GDPR delete semantics shift)",
				ne.Name, normaliseOnExit(oe.OnSubjectExit), normaliseOnExit(ne.OnSubjectExit),
			),
		})
	}
	return out
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func subjectFieldEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func derefStr(p *string) string {
	if p == nil {
		return ""
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

func diffDataPolicy(oldP, newP *DataPolicy, base, nodeName string) []Change {
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
				"node type %q data_policy tightened from %s to %s", nodeName, a, b,
			),
		}}
	}
	return []Change{{
		Kind:     ChangeKindDataPolicyLoosened,
		Path:     base,
		OldValue: string(a),
		NewValue: string(b),
		Message: fmt.Sprintf(
			"node type %q data_policy loosened from %s to %s — historical rows lose protection",
			nodeName, a, b,
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
