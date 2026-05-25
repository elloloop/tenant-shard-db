// Package schema defines the node/edge type system, the registry
// singleton populated at boot and frozen before serving, and the
// lookup primitives the applier and gRPC handlers consult on every
// request.
//
// Spec: docs/go-port/shared/schema-registry.md.
//
// The single source of truth for type metadata is the proto
// descriptor surface (CLAUDE.md invariant #5); JSON is the bootstrap
// channel used when the Go server does not import the proto module
// directly. Fingerprint() is byte-stable for the same registry
// contents across all implementations.
package schema

import (
	"fmt"
)

// FieldKind enumerates the storage-and-validation kinds a FieldDef
// can take. Wire values are the strings the SDK ships in the JSON
// schema export and the proto descriptor surface.
type FieldKind string

const (
	KindString    FieldKind = "str"
	KindInteger   FieldKind = "int"
	KindFloat     FieldKind = "float"
	KindBoolean   FieldKind = "bool"
	KindTimestamp FieldKind = "timestamp"
	KindJSON      FieldKind = "json"
	KindBytes     FieldKind = "bytes"
	KindEnum      FieldKind = "enum"
	KindReference FieldKind = "ref"
	KindListStr   FieldKind = "list_str"
	KindListInt   FieldKind = "list_int"
	KindListRef   FieldKind = "list_ref"
)

// Valid reports whether k is one of the declared FieldKind values.
func (k FieldKind) Valid() bool {
	switch k {
	case KindString, KindInteger, KindFloat, KindBoolean,
		KindTimestamp, KindJSON, KindBytes, KindEnum,
		KindReference, KindListStr, KindListInt, KindListRef:
		return true
	}
	return false
}

// AclPermission is the permission type for AclEntry. ACL evaluation
// lives in the acl package, not here.
type AclPermission string

const (
	PermissionRead   AclPermission = "read"
	PermissionWrite  AclPermission = "write"
	PermissionDelete AclPermission = "delete"
	PermissionAdmin  AclPermission = "admin"
)

// OnSubjectExit drives GDPR edge-cleanup behaviour when a data subject
// is deleted/anonymized.
type OnSubjectExit string

const (
	OnSubjectExitFrom OnSubjectExit = "from"
	OnSubjectExitTo   OnSubjectExit = "to"
	OnSubjectExitBoth OnSubjectExit = "both"
)

// DataPolicy is the data-classification tier for a node or edge type.
// Wire values are stable and round-trip through the JSON contract.
type DataPolicy string

const (
	DataPolicyPersonal   DataPolicy = "personal"
	DataPolicyBusiness   DataPolicy = "business"
	DataPolicyFinancial  DataPolicy = "financial"
	DataPolicyAudit      DataPolicy = "audit"
	DataPolicyEphemeral  DataPolicy = "ephemeral"
	DataPolicyHealthcare DataPolicy = "healthcare"
)

// AclEntry is one entry in NodeTypeDef.DefaultAcl.
type AclEntry struct {
	Principal  string        `json:"principal"`
	Permission AclPermission `json:"permission"`
}

// FieldDef describes a single field on a node or edge type. JSON field
// names use snake_case as defined by the schema JSON contract.
//
// Per ADR-031 the type system is NAME-FREE: a field is identified solely
// by its FieldID (the proto field number per ADR-018). The human-facing
// name lives only in the client's proto and is resolved by the SDK.
//
// Invariants enforced by Validate:
//   - FieldID in [1, 65535]
//   - EnumValues non-empty when Kind == KindEnum
//   - RefTypeID set when Kind == KindReference
type FieldDef struct {
	FieldID     uint32    `json:"field_id"`
	Kind        FieldKind `json:"kind"`
	Required    bool      `json:"required,omitempty"`
	Default     any       `json:"default,omitempty"`
	EnumValues  []string  `json:"enum_values,omitempty"`
	RefTypeID   *int32    `json:"ref_type_id,omitempty"`
	Indexed     bool      `json:"indexed,omitempty"`
	Searchable  bool      `json:"searchable,omitempty"`
	Deprecated  bool      `json:"deprecated,omitempty"`
	Description string    `json:"description,omitempty"`
	PII         bool      `json:"pii,omitempty"`
	Unique      bool      `json:"unique,omitempty"`
}

// Validate checks the field invariants. Returns an error rather than
// panicking so loader callers can collect every violation in a single
// pass. Fields are identified by field_id (name-free, ADR-031).
func (f *FieldDef) Validate() error {
	if f.FieldID == 0 {
		return fmt.Errorf("field_id must be positive, got %d", f.FieldID)
	}
	if f.FieldID > 65535 {
		return fmt.Errorf("field_id must be <= 65535, got %d", f.FieldID)
	}
	if !f.Kind.Valid() {
		return fmt.Errorf("invalid field kind %q for field_id %d", f.Kind, f.FieldID)
	}
	if f.Kind == KindEnum && len(f.EnumValues) == 0 {
		return fmt.Errorf("enum_values required for ENUM field_id %d", f.FieldID)
	}
	if f.Kind == KindReference && f.RefTypeID == nil {
		return fmt.Errorf("ref_type_id required for REFERENCE field_id %d", f.FieldID)
	}
	return nil
}

// CompositeUniqueDef declares a multi-field unique constraint. Per
// ADR-031 a composite constraint is identified solely by its FieldIDs
// tuple — there is no constraint name. The index suffix is derived from
// the tuple (see store.compositeIndexSuffix).
type CompositeUniqueDef struct {
	FieldIDs []uint32 `json:"field_ids"`
}

// Validate enforces the CompositeUniqueDef invariants.
func (c *CompositeUniqueDef) Validate() error {
	if len(c.FieldIDs) < 2 {
		return fmt.Errorf(
			"CompositeUniqueDef must reference at least 2 fields, got %d",
			len(c.FieldIDs),
		)
	}
	seen := make(map[uint32]struct{}, len(c.FieldIDs))
	for _, fid := range c.FieldIDs {
		if _, dup := seen[fid]; dup {
			return fmt.Errorf(
				"CompositeUniqueDef %s has duplicate field_id %d",
				signature(c.FieldIDs), fid,
			)
		}
		seen[fid] = struct{}{}
	}
	return nil
}

// NodeTypeDef describes a node type in the schema registry. Per ADR-031
// the type is identified solely by its TypeID — there is no type name.
type NodeTypeDef struct {
	TypeID int32 `json:"type_id"`
	// Fields is always emitted (even when empty) — the JSON contract
	// always includes the "fields" key.
	Fields          []FieldDef           `json:"fields"`
	Deprecated      bool                 `json:"deprecated,omitempty"`
	Description     string               `json:"description,omitempty"`
	DefaultACL      []AclEntry           `json:"default_acl,omitempty"`
	DataPolicy      *DataPolicy          `json:"data_policy,omitempty"`
	SubjectField    *uint32              `json:"subject_field,omitempty"`
	LegalBasis      *string              `json:"legal_basis,omitempty"`
	CompositeUnique []CompositeUniqueDef `json:"composite_unique,omitempty"`
	// ReservedFieldIDs is the tombstone list for removed field_ids. Per
	// ADR-032 removing a field is SAFE only when its id is then reserved
	// here so it can never be reused for a different field — the same
	// guarantee proto's native `reserved` gives. The compat checker treats
	// a live field_id that appears in this list (i.e. a removed/reserved
	// id brought back to life) as a BREAKING reuse. Emitted only when
	// non-empty so it does not perturb the fingerprint of schemas that
	// don't reserve.
	ReservedFieldIDs []uint32 `json:"reserved_field_ids,omitempty"`
}

// Validate checks NodeTypeDef invariants: positive type_id, no duplicate
// field_ids, and composite_unique constraints reference known fields
// with unique field-id signatures (the tuple IS the constraint identity,
// ADR-031).
func (n *NodeTypeDef) Validate() error {
	if n.TypeID <= 0 {
		return fmt.Errorf("type_id must be positive, got %d", n.TypeID)
	}

	fieldIDs := make(map[uint32]struct{}, len(n.Fields))
	for i := range n.Fields {
		f := &n.Fields[i]
		if err := f.Validate(); err != nil {
			return fmt.Errorf("node type_id %d: %w", n.TypeID, err)
		}
		if _, dup := fieldIDs[f.FieldID]; dup {
			return fmt.Errorf("duplicate field_id %d in node type_id %d", f.FieldID, n.TypeID)
		}
		fieldIDs[f.FieldID] = struct{}{}
	}

	for _, rid := range n.ReservedFieldIDs {
		if _, live := fieldIDs[rid]; live {
			return fmt.Errorf(
				"node type_id %d declares field_id %d both live and reserved",
				n.TypeID, rid,
			)
		}
	}

	if len(n.CompositeUnique) > 0 {
		seenSig := make(map[string]struct{}, len(n.CompositeUnique))
		for i := range n.CompositeUnique {
			cu := &n.CompositeUnique[i]
			if err := cu.Validate(); err != nil {
				return fmt.Errorf("node type_id %d: %w", n.TypeID, err)
			}
			for _, fid := range cu.FieldIDs {
				if _, ok := fieldIDs[fid]; !ok {
					return fmt.Errorf(
						"composite_unique %s on node type_id %d references unknown field_id %d",
						signature(cu.FieldIDs), n.TypeID, fid,
					)
				}
			}
			sig := signature(cu.FieldIDs)
			if _, dup := seenSig[sig]; dup {
				return fmt.Errorf(
					"duplicate composite_unique constraint on node type_id %d: fields %s already declared",
					n.TypeID, sig,
				)
			}
			seenSig[sig] = struct{}{}
		}
	}
	return nil
}

// GetFieldByID returns the field with the given field_id (or nil if
// absent). Fields are identified by id only (ADR-031).
func (n *NodeTypeDef) GetFieldByID(id uint32) *FieldDef {
	for i := range n.Fields {
		if n.Fields[i].FieldID == id {
			return &n.Fields[i]
		}
	}
	return nil
}

// EdgeTypeDef describes an edge type in the schema registry. Per ADR-031
// the type is identified solely by its EdgeID — there is no edge name.
// JSON keys use from_type_id / to_type_id (integer IDs, not NodeTypeDef
// pointers).
type EdgeTypeDef struct {
	EdgeID        int32       `json:"edge_id"`
	FromTypeID    int32       `json:"from_type_id"`
	ToTypeID      int32       `json:"to_type_id"`
	Props         []FieldDef  `json:"props"`
	UniquePerFrom bool        `json:"unique_per_from,omitempty"`
	Deprecated    bool        `json:"deprecated,omitempty"`
	Description   string      `json:"description,omitempty"`
	DataPolicy    *DataPolicy `json:"data_policy,omitempty"`
	// OnSubjectExit is always emitted (defaults to "both"); the JSON
	// contract writes this key unconditionally.
	OnSubjectExit OnSubjectExit `json:"on_subject_exit"`
	// ReservedFieldIDs is the tombstone list for removed prop field_ids
	// (same semantics as NodeTypeDef.ReservedFieldIDs, per ADR-032).
	ReservedFieldIDs []uint32 `json:"reserved_field_ids,omitempty"`
}

// Validate checks EdgeTypeDef invariants: positive edge_id, no duplicate
// prop field_ids (name-free, ADR-031).
func (e *EdgeTypeDef) Validate() error {
	if e.EdgeID <= 0 {
		return fmt.Errorf("edge_id must be positive, got %d", e.EdgeID)
	}
	propIDs := make(map[uint32]struct{}, len(e.Props))
	for i := range e.Props {
		p := &e.Props[i]
		if err := p.Validate(); err != nil {
			return fmt.Errorf("edge type_id %d: %w", e.EdgeID, err)
		}
		if _, dup := propIDs[p.FieldID]; dup {
			return fmt.Errorf("duplicate field_id %d in edge type_id %d", p.FieldID, e.EdgeID)
		}
		propIDs[p.FieldID] = struct{}{}
	}
	for _, rid := range e.ReservedFieldIDs {
		if _, live := propIDs[rid]; live {
			return fmt.Errorf(
				"edge type_id %d declares field_id %d both live and reserved",
				e.EdgeID, rid,
			)
		}
	}
	return nil
}

// signature renders a sorted-field-id signature for composite-unique
// duplicate detection. Stable across Go versions because we sort.
func signature(ids []uint32) string {
	cp := make([]uint32, len(ids))
	copy(cp, ids)
	// insertion sort (n is small, typically 2-4)
	for i := 1; i < len(cp); i++ {
		for j := i; j > 0 && cp[j-1] > cp[j]; j-- {
			cp[j-1], cp[j] = cp[j], cp[j-1]
		}
	}
	var buf []byte
	buf = append(buf, '(')
	for i, v := range cp {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = appendUint(buf, uint64(v))
	}
	buf = append(buf, ')')
	return string(buf)
}

func appendUint(buf []byte, v uint64) []byte {
	if v == 0 {
		return append(buf, '0')
	}
	var tmp [20]byte
	i := len(tmp)
	for v > 0 {
		i--
		tmp[i] = byte('0' + v%10)
		v /= 10
	}
	return append(buf, tmp[i:]...)
}
