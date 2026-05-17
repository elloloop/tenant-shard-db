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
// directly. The Python SDK's SchemaRegistry serialises to the same
// JSON shape and Fingerprint() is byte-stable across implementations
// for the same registry contents.
package schema

import (
	"errors"
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

// AclPermission mirrors schema/types.py AclPermission. Stored on
// AclEntry; ACL evaluation lives in the acl package, not here.
type AclPermission string

const (
	PermissionRead   AclPermission = "read"
	PermissionWrite  AclPermission = "write"
	PermissionDelete AclPermission = "delete"
	PermissionAdmin  AclPermission = "admin"
)

// OnSubjectExit mirrors schema/types.py OnSubjectExit. Drives GDPR
// edge-cleanup behaviour when a data subject is deleted/anonymized.
type OnSubjectExit string

const (
	OnSubjectExitFrom OnSubjectExit = "from"
	OnSubjectExitTo   OnSubjectExit = "to"
	OnSubjectExitBoth OnSubjectExit = "both"
)

// DataPolicy mirrors entdb_server/data_policy.py DataPolicy. Wire
// values match the Python enum so the JSON contract round-trips.
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

// FieldDef is the Go counterpart of schema/types.py FieldDef. Field
// names match the Python JSON keys (snake_case) so a Python-emitted
// schema file boots a Go server unchanged.
//
// Invariants enforced by Validate:
//   - FieldID in [1, 65535]
//   - Name non-empty
//   - EnumValues non-empty when Kind == KindEnum
//   - RefTypeID set when Kind == KindReference
type FieldDef struct {
	FieldID     uint32    `json:"field_id"`
	Name        string    `json:"name"`
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
// pass (mirrors Python __post_init__ but composable with the loader).
func (f *FieldDef) Validate() error {
	if f.FieldID == 0 {
		return fmt.Errorf("field_id must be positive, got %d", f.FieldID)
	}
	if f.FieldID > 65535 {
		return fmt.Errorf("field_id must be <= 65535, got %d", f.FieldID)
	}
	if f.Name == "" {
		return errors.New("field name cannot be empty")
	}
	if !f.Kind.Valid() {
		return fmt.Errorf("invalid field kind %q for field %q", f.Kind, f.Name)
	}
	if f.Kind == KindEnum && len(f.EnumValues) == 0 {
		return fmt.Errorf("enum_values required for ENUM field %q", f.Name)
	}
	if f.Kind == KindReference && f.RefTypeID == nil {
		return fmt.Errorf("ref_type_id required for REFERENCE field %q", f.Name)
	}
	return nil
}

// CompositeUniqueDef is the Go counterpart of schema/types.py
// CompositeUniqueDef. Wire field names match the Python JSON keys so
// the contract round-trips.
type CompositeUniqueDef struct {
	Name     string   `json:"name"`
	FieldIDs []uint32 `json:"field_ids"`
}

// Validate enforces the same three invariants as Python __post_init__.
func (c *CompositeUniqueDef) Validate() error {
	if c.Name == "" {
		return errors.New("CompositeUniqueDef name cannot be empty")
	}
	if len(c.FieldIDs) < 2 {
		return fmt.Errorf(
			"CompositeUniqueDef %q must reference at least 2 fields, got %d",
			c.Name, len(c.FieldIDs),
		)
	}
	seen := make(map[uint32]struct{}, len(c.FieldIDs))
	for _, fid := range c.FieldIDs {
		if _, dup := seen[fid]; dup {
			return fmt.Errorf(
				"CompositeUniqueDef %q has duplicate field_id %d",
				c.Name, fid,
			)
		}
		seen[fid] = struct{}{}
	}
	return nil
}

// NodeTypeDef is the Go counterpart of schema/types.py NodeTypeDef.
type NodeTypeDef struct {
	TypeID int32  `json:"type_id"`
	Name   string `json:"name"`
	// Fields is always emitted (even when empty) to match Python
	// NodeTypeDef.to_dict, which always includes the "fields" key.
	Fields          []FieldDef           `json:"fields"`
	Deprecated      bool                 `json:"deprecated,omitempty"`
	Description     string               `json:"description,omitempty"`
	DefaultACL      []AclEntry           `json:"default_acl,omitempty"`
	DataPolicy      *DataPolicy          `json:"data_policy,omitempty"`
	SubjectField    *string              `json:"subject_field,omitempty"`
	LegalBasis      *string              `json:"legal_basis,omitempty"`
	CompositeUnique []CompositeUniqueDef `json:"composite_unique,omitempty"`
}

// Validate checks the same invariants as Python __post_init__:
// positive type_id, non-empty name, no duplicate field_ids/names, and
// composite_unique constraints reference known fields with unique
// names and unique field-id signatures.
func (n *NodeTypeDef) Validate() error {
	if n.TypeID <= 0 {
		return fmt.Errorf("type_id must be positive, got %d", n.TypeID)
	}
	if n.Name == "" {
		return errors.New("node type name cannot be empty")
	}

	fieldIDs := make(map[uint32]struct{}, len(n.Fields))
	fieldNames := make(map[string]struct{}, len(n.Fields))
	for i := range n.Fields {
		f := &n.Fields[i]
		if err := f.Validate(); err != nil {
			return fmt.Errorf("node type %q: %w", n.Name, err)
		}
		if _, dup := fieldIDs[f.FieldID]; dup {
			return fmt.Errorf("duplicate field_id %d in node type %q", f.FieldID, n.Name)
		}
		fieldIDs[f.FieldID] = struct{}{}
		if _, dup := fieldNames[f.Name]; dup {
			return fmt.Errorf("duplicate field name %q in node type %q", f.Name, n.Name)
		}
		fieldNames[f.Name] = struct{}{}
	}

	if len(n.CompositeUnique) > 0 {
		seenNames := make(map[string]struct{}, len(n.CompositeUnique))
		seenSig := make(map[string]struct{}, len(n.CompositeUnique))
		for i := range n.CompositeUnique {
			cu := &n.CompositeUnique[i]
			if err := cu.Validate(); err != nil {
				return fmt.Errorf("node type %q: %w", n.Name, err)
			}
			for _, fid := range cu.FieldIDs {
				if _, ok := fieldIDs[fid]; !ok {
					return fmt.Errorf(
						"composite_unique %q on node type %q references unknown field_id %d",
						cu.Name, n.Name, fid,
					)
				}
			}
			if _, dup := seenNames[cu.Name]; dup {
				return fmt.Errorf(
					"duplicate composite_unique name %q on node type %q",
					cu.Name, n.Name,
				)
			}
			seenNames[cu.Name] = struct{}{}
			sig := signature(cu.FieldIDs)
			if _, dup := seenSig[sig]; dup {
				return fmt.Errorf(
					"duplicate composite_unique constraint on node type %q: fields %s already declared",
					n.Name, sig,
				)
			}
			seenSig[sig] = struct{}{}
		}
	}
	return nil
}

// GetField returns the named field (or nil if absent). Linear scan;
// the registry caches per-type name->field maps for hot-path lookups.
func (n *NodeTypeDef) GetField(name string) *FieldDef {
	for i := range n.Fields {
		if n.Fields[i].Name == name {
			return &n.Fields[i]
		}
	}
	return nil
}

// GetFieldByID is the field-id counterpart of GetField.
func (n *NodeTypeDef) GetFieldByID(id uint32) *FieldDef {
	for i := range n.Fields {
		if n.Fields[i].FieldID == id {
			return &n.Fields[i]
		}
	}
	return nil
}

// EdgeTypeDef is the Go counterpart of schema/types.py EdgeTypeDef.
// JSON keys match Python's to_dict (which serialises from_type_id /
// to_type_id, not the NodeTypeDef pointer).
type EdgeTypeDef struct {
	EdgeID        int32       `json:"edge_id"`
	Name          string      `json:"name"`
	FromTypeID    int32       `json:"from_type_id"`
	ToTypeID      int32       `json:"to_type_id"`
	Props         []FieldDef  `json:"props"`
	UniquePerFrom bool        `json:"unique_per_from,omitempty"`
	Deprecated    bool        `json:"deprecated,omitempty"`
	Description   string      `json:"description,omitempty"`
	DataPolicy    *DataPolicy `json:"data_policy,omitempty"`
	// OnSubjectExit is always emitted (defaults to "both"); mirrors
	// Python EdgeTypeDef.to_dict which writes the key unconditionally.
	OnSubjectExit OnSubjectExit `json:"on_subject_exit"`
}

// Validate checks the same invariants as Python __post_init__:
// positive edge_id, non-empty name, no duplicate prop field_ids.
func (e *EdgeTypeDef) Validate() error {
	if e.EdgeID <= 0 {
		return fmt.Errorf("edge_id must be positive, got %d", e.EdgeID)
	}
	if e.Name == "" {
		return errors.New("edge type name cannot be empty")
	}
	propIDs := make(map[uint32]struct{}, len(e.Props))
	for i := range e.Props {
		p := &e.Props[i]
		if err := p.Validate(); err != nil {
			return fmt.Errorf("edge type %q: %w", e.Name, err)
		}
		if _, dup := propIDs[p.FieldID]; dup {
			return fmt.Errorf("duplicate field_id %d in edge type %q", p.FieldID, e.Name)
		}
		propIDs[p.FieldID] = struct{}{}
	}
	if e.OnSubjectExit == "" {
		// Python defaults to BOTH; preserve that on parse but do not
		// rewrite the source field — the loader normalises.
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
