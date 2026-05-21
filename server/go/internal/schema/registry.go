package schema

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// ErrFrozen is returned when a mutation is attempted on a frozen
// registry. The errs package maps this to FAILED_PRECONDITION at the
// gRPC boundary.
var ErrFrozen = errors.New("schema: registry is frozen")

// ErrDuplicateRegistration is returned by RegisterNode/RegisterEdge
// when the type_id or name collides with an already-registered type.
var ErrDuplicateRegistration = errors.New("schema: duplicate registration")

// Registry is the process-wide singleton populated at boot, frozen
// before serving.
//
// After Freeze() returns, all read methods are lock-free — the
// applier and gRPC handlers hold a *Registry reference without
// further synchronization.
type Registry struct {
	mu sync.Mutex // held only during registration / freeze

	nodes       map[int32]*NodeTypeDef
	nodesByName map[string]*NodeTypeDef
	edges       map[int32]*EdgeTypeDef
	edgesByName map[string]*EdgeTypeDef

	// Per-type field name<->id maps, populated lazily on first lookup
	// after Freeze. Translation lives in translate.go.
	fieldNameToID map[int32]map[string]uint32
	fieldIDToName map[int32]map[uint32]string

	frozen      atomic.Bool
	fingerprint atomic.Pointer[string]
}

// NewRegistry returns an empty, mutable registry. Production code paths
// take *Registry as a constructor argument (so tests can inject
// fixtures without touching package-level state).
func NewRegistry() *Registry {
	return &Registry{
		nodes:       map[int32]*NodeTypeDef{},
		nodesByName: map[string]*NodeTypeDef{},
		edges:       map[int32]*EdgeTypeDef{},
		edgesByName: map[string]*EdgeTypeDef{},
	}
}

// Frozen reports whether Freeze has been called.
func (r *Registry) Frozen() bool { return r.frozen.Load() }

// Fingerprint returns the post-freeze schema fingerprint, e.g.
// "sha256:abc123…". Returns the empty string before Freeze.
//
// The actual SHA computation lives in fingerprint.go; this accessor is
// here so registry.go owns the public surface.
func (r *Registry) Fingerprint() string {
	if p := r.fingerprint.Load(); p != nil {
		return *p
	}
	return ""
}

// RegisterNode adds nt to the registry. Returns ErrFrozen if the
// registry is frozen, ErrDuplicateRegistration if the type_id or name
// is already taken, or a validation error if nt is malformed.
//
// nt is validated before insertion; on error the registry is unchanged.
func (r *Registry) RegisterNode(nt *NodeTypeDef) error {
	if nt == nil {
		return errors.New("schema: nil NodeTypeDef")
	}
	if err := nt.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen.Load() {
		return fmt.Errorf("%w: cannot register node type %q", ErrFrozen, nt.Name)
	}
	if existing, ok := r.nodes[nt.TypeID]; ok {
		return fmt.Errorf(
			"%w: type_id %d already registered as %q",
			ErrDuplicateRegistration, nt.TypeID, existing.Name,
		)
	}
	if existing, ok := r.nodesByName[nt.Name]; ok {
		return fmt.Errorf(
			"%w: node type name %q already registered with type_id %d",
			ErrDuplicateRegistration, nt.Name, existing.TypeID,
		)
	}
	r.nodes[nt.TypeID] = nt
	r.nodesByName[nt.Name] = nt
	return nil
}

// RegisterEdge adds et to the registry. Same error semantics as
// RegisterNode. Edge from_type_id / to_type_id need not be registered
// yet — the cross-reference check runs in ValidateAll.
func (r *Registry) RegisterEdge(et *EdgeTypeDef) error {
	if et == nil {
		return errors.New("schema: nil EdgeTypeDef")
	}
	if err := et.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen.Load() {
		return fmt.Errorf("%w: cannot register edge type %q", ErrFrozen, et.Name)
	}
	if existing, ok := r.edges[et.EdgeID]; ok {
		return fmt.Errorf(
			"%w: edge_id %d already registered as %q",
			ErrDuplicateRegistration, et.EdgeID, existing.Name,
		)
	}
	if existing, ok := r.edgesByName[et.Name]; ok {
		return fmt.Errorf(
			"%w: edge type name %q already registered with edge_id %d",
			ErrDuplicateRegistration, et.Name, existing.EdgeID,
		)
	}
	if et.OnSubjectExit == "" {
		et.OnSubjectExit = OnSubjectExitBoth
	}
	r.edges[et.EdgeID] = et
	r.edgesByName[et.Name] = et
	return nil
}

// Freeze computes the deterministic fingerprint and flips the frozen
// flag atomically. Subsequent RegisterNode/RegisterEdge calls return
// ErrFrozen. Calling Freeze twice returns ErrFrozen on the second
// call.
//
// After Freeze returns, the per-type field name<->id maps are
// pre-built so translate.FieldIDByName / translate.FieldNameByID
// stay lock-free on the hot path.
func (r *Registry) Freeze() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen.Load() {
		return "", fmt.Errorf("%w: already frozen", ErrFrozen)
	}
	fp, err := r.computeFingerprint()
	if err != nil {
		return "", err
	}
	// Pre-build translation maps before flipping frozen so reads are
	// lock-free post-freeze.
	r.fieldNameToID = make(map[int32]map[string]uint32, len(r.nodes))
	r.fieldIDToName = make(map[int32]map[uint32]string, len(r.nodes))
	for tid, n := range r.nodes {
		nm := make(map[string]uint32, len(n.Fields))
		im := make(map[uint32]string, len(n.Fields))
		for i := range n.Fields {
			f := &n.Fields[i]
			nm[f.Name] = f.FieldID
			im[f.FieldID] = f.Name
		}
		r.fieldNameToID[tid] = nm
		r.fieldIDToName[tid] = im
	}
	r.fingerprint.Store(&fp)
	r.frozen.Store(true)
	return fp, nil
}

// NodeType returns the node type by id (int / int32) or name (string).
// Returns nil if not found. Lock-free after Freeze.
//
// Callers that already have a typed argument should prefer the explicit
// helpers below for compile-time safety.
func (r *Registry) NodeType(idOrName any) *NodeTypeDef {
	switch v := idOrName.(type) {
	case int32:
		return r.nodes[v]
	case int:
		return r.nodes[int32(v)]
	case int64:
		return r.nodes[int32(v)]
	case uint32:
		return r.nodes[int32(v)]
	case string:
		return r.nodesByName[v]
	}
	return nil
}

// NodeTypeByID is the explicit-id helper.
func (r *Registry) NodeTypeByID(id int32) *NodeTypeDef { return r.nodes[id] }

// NodeTypeByName is the explicit-name helper.
func (r *Registry) NodeTypeByName(name string) *NodeTypeDef { return r.nodesByName[name] }

// EdgeType is the edge counterpart of NodeType.
func (r *Registry) EdgeType(idOrName any) *EdgeTypeDef {
	switch v := idOrName.(type) {
	case int32:
		return r.edges[v]
	case int:
		return r.edges[int32(v)]
	case int64:
		return r.edges[int32(v)]
	case uint32:
		return r.edges[int32(v)]
	case string:
		return r.edgesByName[v]
	}
	return nil
}

// EdgeTypeByID is the explicit-id helper.
func (r *Registry) EdgeTypeByID(id int32) *EdgeTypeDef { return r.edges[id] }

// EdgeTypeByName is the explicit-name helper.
func (r *Registry) EdgeTypeByName(name string) *EdgeTypeDef { return r.edgesByName[name] }

// NodeTypes returns the registered node types in deterministic order
// (sorted by type_id). The returned slice is a copy; callers may
// mutate it freely.
func (r *Registry) NodeTypes() []*NodeTypeDef {
	out := make([]*NodeTypeDef, 0, len(r.nodes))
	ids := make([]int32, 0, len(r.nodes))
	for id := range r.nodes {
		ids = append(ids, id)
	}
	sortInt32(ids)
	for _, id := range ids {
		out = append(out, r.nodes[id])
	}
	return out
}

// EdgeTypes is the edge counterpart of NodeTypes.
func (r *Registry) EdgeTypes() []*EdgeTypeDef {
	out := make([]*EdgeTypeDef, 0, len(r.edges))
	ids := make([]int32, 0, len(r.edges))
	for id := range r.edges {
		ids = append(ids, id)
	}
	sortInt32(ids)
	for _, id := range ids {
		out = append(out, r.edges[id])
	}
	return out
}

// UniqueFieldIDs returns ids of fields declared unique on the node
// type. Returns nil for unknown types. Excludes deprecated fields.
func (r *Registry) UniqueFieldIDs(typeID int32) []uint32 {
	n := r.nodes[typeID]
	if n == nil {
		return nil
	}
	out := make([]uint32, 0)
	for i := range n.Fields {
		f := &n.Fields[i]
		if f.Unique && !f.Deprecated {
			out = append(out, f.FieldID)
		}
	}
	return out
}

// CompositeUnique returns the composite-unique constraints for a node
// type (in declaration order). Returns nil for unknown types.
func (r *Registry) CompositeUnique(typeID int32) []CompositeUniqueDef {
	n := r.nodes[typeID]
	if n == nil {
		return nil
	}
	out := make([]CompositeUniqueDef, len(n.CompositeUnique))
	copy(out, n.CompositeUnique)
	return out
}

// IndexedFieldIDs returns ids of fields declared indexed but not
// unique (unique fields already get a unique expression index).
func (r *Registry) IndexedFieldIDs(typeID int32) []uint32 {
	n := r.nodes[typeID]
	if n == nil {
		return nil
	}
	out := make([]uint32, 0)
	for i := range n.Fields {
		f := &n.Fields[i]
		if f.Indexed && !f.Unique && !f.Deprecated {
			out = append(out, f.FieldID)
		}
	}
	return out
}

// SearchableFieldIDs returns ids of STRING fields declared searchable.
// Non-string searchable fields are silently excluded (the warning is
// emitted at registration time by the SDK codegen).
func (r *Registry) SearchableFieldIDs(typeID int32) []uint32 {
	n := r.nodes[typeID]
	if n == nil {
		return nil
	}
	out := make([]uint32, 0)
	for i := range n.Fields {
		f := &n.Fields[i]
		if f.Searchable && !f.Deprecated && f.Kind == KindString {
			out = append(out, f.FieldID)
		}
	}
	return out
}

// PIIFields returns the names of PII-marked, non-deprecated fields on
// the type. Returns nil for unknown types.
func (r *Registry) PIIFields(typeID int32) []string {
	n := r.nodes[typeID]
	if n == nil {
		return nil
	}
	out := make([]string, 0)
	for i := range n.Fields {
		f := &n.Fields[i]
		if f.PII && !f.Deprecated {
			out = append(out, f.Name)
		}
	}
	return out
}

// DataPolicyOf returns the data policy for a node type, defaulting to
// PERSONAL when unset. Returns the zero value for unknown types —
// callers that need a distinguished error should check NodeTypeByID
// first.
func (r *Registry) DataPolicyOf(typeID int32) DataPolicy {
	n := r.nodes[typeID]
	if n == nil {
		return ""
	}
	if n.DataPolicy != nil {
		return *n.DataPolicy
	}
	return DataPolicyPersonal
}

// SubjectField returns the subject field name for a node type, or
// empty string if unset / unknown.
func (r *Registry) SubjectField(typeID int32) string {
	n := r.nodes[typeID]
	if n == nil || n.SubjectField == nil {
		return ""
	}
	return *n.SubjectField
}

// ValidateAll cross-references edge from_type_id / to_type_id against
// registered nodes, and verifies every FieldDef.RefTypeID points at a
// registered node.
func (r *Registry) ValidateAll() []string {
	var errs []string
	for _, e := range r.edges {
		if _, ok := r.nodes[e.FromTypeID]; !ok {
			errs = append(errs, fmt.Sprintf(
				"edge %q (edge_id=%d) references unknown from_type_id %d",
				e.Name, e.EdgeID, e.FromTypeID,
			))
		}
		if _, ok := r.nodes[e.ToTypeID]; !ok {
			errs = append(errs, fmt.Sprintf(
				"edge %q (edge_id=%d) references unknown to_type_id %d",
				e.Name, e.EdgeID, e.ToTypeID,
			))
		}
	}
	for _, n := range r.nodes {
		for i := range n.Fields {
			f := &n.Fields[i]
			if f.RefTypeID != nil {
				if _, ok := r.nodes[*f.RefTypeID]; !ok {
					errs = append(errs, fmt.Sprintf(
						"field %q in node type %q references unknown type_id %d",
						f.Name, n.Name, *f.RefTypeID,
					))
				}
			}
		}
	}
	return errs
}
