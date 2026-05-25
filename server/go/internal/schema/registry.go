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

// ErrSchemaConflict is returned by RegisterOrVerifyNode /
// RegisterOrVerifyEdge under the establish-or-reject policy when a type
// is already registered under the same identity (type_id or name) but
// with a DIFFERENT definition. The errs package maps this to
// FAILED_PRECONDITION at the gRPC boundary. Online evolution / ALTER is
// out of scope, so a conflicting redefinition is always rejected.
var ErrSchemaConflict = errors.New("schema: conflicting type definition")

// snapshot is the immutable container of the registry's maps. It is
// swapped atomically on every successful mutation so reads are
// lock-free: a reader loads the current *snapshot once and indexes into
// it without holding a lock, while a concurrent writer publishes a fresh
// snapshot built from a clone of the previous one. A reader holding an
// older snapshot keeps observing a consistent, self-coherent view (the
// classic copy-on-write read/write split).
//
// This is the property that makes SELF-DESCRIBING WRITES safe: the
// applier's schema op registers types at runtime (mutating the registry
// while gRPC read handlers run concurrently) without a process-wide
// read lock on the hot path.
type snapshot struct {
	nodes       map[int32]*NodeTypeDef
	nodesByName map[string]*NodeTypeDef
	edges       map[int32]*EdgeTypeDef
	edgesByName map[string]*EdgeTypeDef

	// Per-type field name<->id maps. Always populated (lazily-empty-safe)
	// so the translate.go lookups can stay lock-free in both frozen and
	// runtime-mutable modes.
	fieldNameToID map[int32]map[string]uint32
	fieldIDToName map[int32]map[uint32]string
}

// clone returns a shallow copy of s with fresh top-level maps. The
// values (*NodeTypeDef / *EdgeTypeDef and the inner field maps) are
// shared with the parent snapshot — they are never mutated in place
// (establish-or-reject never edits an existing type), so sharing them is
// safe and keeps a registration O(types) rather than O(fields).
func (s *snapshot) clone() *snapshot {
	out := &snapshot{
		nodes:         make(map[int32]*NodeTypeDef, len(s.nodes)+1),
		nodesByName:   make(map[string]*NodeTypeDef, len(s.nodesByName)+1),
		edges:         make(map[int32]*EdgeTypeDef, len(s.edges)+1),
		edgesByName:   make(map[string]*EdgeTypeDef, len(s.edgesByName)+1),
		fieldNameToID: make(map[int32]map[string]uint32, len(s.fieldNameToID)+1),
		fieldIDToName: make(map[int32]map[uint32]string, len(s.fieldIDToName)+1),
	}
	for k, v := range s.nodes {
		out.nodes[k] = v
	}
	for k, v := range s.nodesByName {
		out.nodesByName[k] = v
	}
	for k, v := range s.edges {
		out.edges[k] = v
	}
	for k, v := range s.edgesByName {
		out.edgesByName[k] = v
	}
	for k, v := range s.fieldNameToID {
		out.fieldNameToID[k] = v
	}
	for k, v := range s.fieldIDToName {
		out.fieldIDToName[k] = v
	}
	return out
}

// buildFieldMaps returns the name<->id translation maps for a single
// node type. Used when a node is added to a snapshot.
func buildFieldMaps(n *NodeTypeDef) (map[string]uint32, map[uint32]string) {
	nm := make(map[string]uint32, len(n.Fields))
	im := make(map[uint32]string, len(n.Fields))
	for i := range n.Fields {
		f := &n.Fields[i]
		nm[f.Name] = f.FieldID
		im[f.FieldID] = f.Name
	}
	return nm, im
}

// Registry is the process-wide singleton schema store.
//
// Two modes share one type:
//
//   - Boot-snapshot mode: types are registered before serving and the
//     registry is Freeze()'d. After Freeze, mutation is rejected
//     (ErrFrozen). Unit tests rely on this immutability guarantee.
//   - Runtime-mutable mode (SELF-DESCRIBING WRITES): the registry is
//     never frozen; RegisterOrVerifyNode/Edge register types as writes
//     arrive (establish-or-reject), and the WAL replay rebuilds the
//     same set on boot. The fingerprint is recomputed after every
//     successful mutation.
//
// Reads are always lock-free in both modes: every read loads the
// current immutable *snapshot via an atomic pointer and indexes into it.
// Mutations serialize on mu, build a new snapshot from a clone, and
// publish it atomically.
type Registry struct {
	mu sync.Mutex // held only during registration / freeze

	snap atomic.Pointer[snapshot]

	frozen      atomic.Bool
	fingerprint atomic.Pointer[string]
}

// NewRegistry returns an empty, mutable registry. Production code paths
// take *Registry as a constructor argument (so tests can inject
// fixtures without touching package-level state).
func NewRegistry() *Registry {
	r := &Registry{}
	r.snap.Store(&snapshot{
		nodes:         map[int32]*NodeTypeDef{},
		nodesByName:   map[string]*NodeTypeDef{},
		edges:         map[int32]*EdgeTypeDef{},
		edgesByName:   map[string]*EdgeTypeDef{},
		fieldNameToID: map[int32]map[string]uint32{},
		fieldIDToName: map[int32]map[uint32]string{},
	})
	return r
}

// load returns the current immutable snapshot. Lock-free; never nil
// after NewRegistry.
func (r *Registry) load() *snapshot { return r.snap.Load() }

// Frozen reports whether Freeze has been called.
func (r *Registry) Frozen() bool { return r.frozen.Load() }

// Fingerprint returns the current schema fingerprint, e.g.
// "sha256:abc123…". In boot-snapshot mode this is set by Freeze; in
// runtime-mutable mode it is recomputed after each successful
// registration. Returns the empty string before the first fingerprint
// is computed.
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
	cur := r.load()
	if existing, ok := cur.nodes[nt.TypeID]; ok {
		return fmt.Errorf(
			"%w: type_id %d already registered as %q",
			ErrDuplicateRegistration, nt.TypeID, existing.Name,
		)
	}
	if existing, ok := cur.nodesByName[nt.Name]; ok {
		return fmt.Errorf(
			"%w: node type name %q already registered with type_id %d",
			ErrDuplicateRegistration, nt.Name, existing.TypeID,
		)
	}
	r.publishNode(cur, nt)
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
	cur := r.load()
	if existing, ok := cur.edges[et.EdgeID]; ok {
		return fmt.Errorf(
			"%w: edge_id %d already registered as %q",
			ErrDuplicateRegistration, et.EdgeID, existing.Name,
		)
	}
	if existing, ok := cur.edgesByName[et.Name]; ok {
		return fmt.Errorf(
			"%w: edge type name %q already registered with edge_id %d",
			ErrDuplicateRegistration, et.Name, existing.EdgeID,
		)
	}
	if et.OnSubjectExit == "" {
		et.OnSubjectExit = OnSubjectExitBoth
	}
	r.publishEdge(cur, et)
	return nil
}

// publishNode builds a fresh snapshot from cur with nt added and stores
// it atomically. Caller MUST hold r.mu.
func (r *Registry) publishNode(cur *snapshot, nt *NodeTypeDef) {
	next := cur.clone()
	next.nodes[nt.TypeID] = nt
	next.nodesByName[nt.Name] = nt
	nm, im := buildFieldMaps(nt)
	next.fieldNameToID[nt.TypeID] = nm
	next.fieldIDToName[nt.TypeID] = im
	r.snap.Store(next)
}

// publishEdge builds a fresh snapshot from cur with et added and stores
// it atomically. Caller MUST hold r.mu.
func (r *Registry) publishEdge(cur *snapshot, et *EdgeTypeDef) {
	next := cur.clone()
	next.edges[et.EdgeID] = et
	next.edgesByName[et.Name] = et
	r.snap.Store(next)
}

// RegisterOrVerifyNode implements the establish-or-reject policy for
// SELF-DESCRIBING WRITES:
//
//   - absent (neither type_id nor name registered): register nt and
//     recompute the fingerprint. Returns (registered=true, nil).
//   - present and byte-identical to the stored definition: no-op.
//     Returns (false, nil) — idempotent.
//   - present but DIFFERENT (or a partial identity collision, e.g. the
//     same type_id under a different name): reject with ErrSchemaConflict.
//
// Works on a runtime-mutable (non-frozen) registry. Calling it on a
// frozen registry returns ErrFrozen UNLESS the type is already present
// and identical (a frozen registry can still answer "yes, identical" as
// a pure read — that keeps a fingerprint-mismatch retry that re-sends an
// already-known schema from spuriously failing).
func (r *Registry) RegisterOrVerifyNode(nt *NodeTypeDef) (registered bool, err error) {
	if nt == nil {
		return false, errors.New("schema: nil NodeTypeDef")
	}
	if err := nt.Validate(); err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cur := r.load()

	byID, hasID := cur.nodes[nt.TypeID]
	byName, hasName := cur.nodesByName[nt.Name]

	if hasID || hasName {
		// Identity collision. Both must resolve to the SAME stored type,
		// and that stored type must be byte-identical to nt; anything
		// else is a conflict (establish-or-reject; no ALTER).
		if hasID && hasName && byID == byName && nodeDefEqual(byID, nt) {
			return false, nil // idempotent no-op
		}
		switch {
		case hasID && !nodeDefEqual(byID, nt):
			return false, fmt.Errorf(
				"%w: node type_id %d is registered as %q which differs from the supplied %q",
				ErrSchemaConflict, nt.TypeID, byID.Name, nt.Name)
		case hasName && (!hasID || byName.TypeID != nt.TypeID):
			return false, fmt.Errorf(
				"%w: node name %q is registered with type_id %d which differs from the supplied type_id %d",
				ErrSchemaConflict, nt.Name, byName.TypeID, nt.TypeID)
		default:
			return false, fmt.Errorf(
				"%w: node type %q (type_id %d) differs from the registered definition",
				ErrSchemaConflict, nt.Name, nt.TypeID)
		}
	}

	// Absent → establish. Frozen registries cannot grow.
	if r.frozen.Load() {
		return false, fmt.Errorf("%w: cannot register node type %q", ErrFrozen, nt.Name)
	}
	r.publishNode(cur, nt)
	if err := r.recomputeFingerprintLocked(); err != nil {
		return true, err
	}
	return true, nil
}

// RegisterOrVerifyEdge is the edge counterpart of RegisterOrVerifyNode.
// Same establish-or-reject semantics.
func (r *Registry) RegisterOrVerifyEdge(et *EdgeTypeDef) (registered bool, err error) {
	if et == nil {
		return false, errors.New("schema: nil EdgeTypeDef")
	}
	if et.OnSubjectExit == "" {
		et.OnSubjectExit = OnSubjectExitBoth
	}
	if err := et.Validate(); err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cur := r.load()

	byID, hasID := cur.edges[et.EdgeID]
	byName, hasName := cur.edgesByName[et.Name]

	if hasID || hasName {
		if hasID && hasName && byID == byName && edgeDefEqual(byID, et) {
			return false, nil // idempotent no-op
		}
		switch {
		case hasID && !edgeDefEqual(byID, et):
			return false, fmt.Errorf(
				"%w: edge_id %d is registered as %q which differs from the supplied %q",
				ErrSchemaConflict, et.EdgeID, byID.Name, et.Name)
		case hasName && (!hasID || byName.EdgeID != et.EdgeID):
			return false, fmt.Errorf(
				"%w: edge name %q is registered with edge_id %d which differs from the supplied edge_id %d",
				ErrSchemaConflict, et.Name, byName.EdgeID, et.EdgeID)
		default:
			return false, fmt.Errorf(
				"%w: edge type %q (edge_id %d) differs from the registered definition",
				ErrSchemaConflict, et.Name, et.EdgeID)
		}
	}

	if r.frozen.Load() {
		return false, fmt.Errorf("%w: cannot register edge type %q", ErrFrozen, et.Name)
	}
	r.publishEdge(cur, et)
	if err := r.recomputeFingerprintLocked(); err != nil {
		return true, err
	}
	return true, nil
}

// recomputeFingerprintLocked recomputes and stores the schema
// fingerprint from the current snapshot. Caller MUST hold r.mu (so the
// snapshot it canonicalises is the one just published). Used by the
// runtime-mutable RegisterOrVerify* path; the frozen path computes the
// fingerprint once in Freeze.
func (r *Registry) recomputeFingerprintLocked() error {
	fp, err := r.computeFingerprint()
	if err != nil {
		return err
	}
	r.fingerprint.Store(&fp)
	return nil
}

// Freeze computes the deterministic fingerprint and flips the frozen
// flag atomically. Subsequent RegisterNode/RegisterEdge calls return
// ErrFrozen. Calling Freeze twice returns ErrFrozen on the second
// call.
//
// After Freeze returns, the per-type field name<->id maps are already
// present in the snapshot (every publishNode builds them), so
// translate.FieldIDByName / translate.FieldNameByID stay lock-free in
// both modes.
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
	r.fingerprint.Store(&fp)
	r.frozen.Store(true)
	return fp, nil
}

// NodeType returns the node type by id (int / int32) or name (string).
// Returns nil if not found. Lock-free.
//
// Callers that already have a typed argument should prefer the explicit
// helpers below for compile-time safety.
func (r *Registry) NodeType(idOrName any) *NodeTypeDef {
	s := r.load()
	switch v := idOrName.(type) {
	case int32:
		return s.nodes[v]
	case int:
		return s.nodes[int32(v)]
	case int64:
		return s.nodes[int32(v)]
	case uint32:
		return s.nodes[int32(v)]
	case string:
		return s.nodesByName[v]
	}
	return nil
}

// NodeTypeByID is the explicit-id helper.
func (r *Registry) NodeTypeByID(id int32) *NodeTypeDef { return r.load().nodes[id] }

// NodeTypeByName is the explicit-name helper.
func (r *Registry) NodeTypeByName(name string) *NodeTypeDef { return r.load().nodesByName[name] }

// EdgeType is the edge counterpart of NodeType.
func (r *Registry) EdgeType(idOrName any) *EdgeTypeDef {
	s := r.load()
	switch v := idOrName.(type) {
	case int32:
		return s.edges[v]
	case int:
		return s.edges[int32(v)]
	case int64:
		return s.edges[int32(v)]
	case uint32:
		return s.edges[int32(v)]
	case string:
		return s.edgesByName[v]
	}
	return nil
}

// EdgeTypeByID is the explicit-id helper.
func (r *Registry) EdgeTypeByID(id int32) *EdgeTypeDef { return r.load().edges[id] }

// EdgeTypeByName is the explicit-name helper.
func (r *Registry) EdgeTypeByName(name string) *EdgeTypeDef { return r.load().edgesByName[name] }

// NodeTypes returns the registered node types in deterministic order
// (sorted by type_id). The returned slice is a copy; callers may
// mutate it freely.
func (r *Registry) NodeTypes() []*NodeTypeDef {
	s := r.load()
	out := make([]*NodeTypeDef, 0, len(s.nodes))
	ids := make([]int32, 0, len(s.nodes))
	for id := range s.nodes {
		ids = append(ids, id)
	}
	sortInt32(ids)
	for _, id := range ids {
		out = append(out, s.nodes[id])
	}
	return out
}

// EdgeTypes is the edge counterpart of NodeTypes.
func (r *Registry) EdgeTypes() []*EdgeTypeDef {
	s := r.load()
	out := make([]*EdgeTypeDef, 0, len(s.edges))
	ids := make([]int32, 0, len(s.edges))
	for id := range s.edges {
		ids = append(ids, id)
	}
	sortInt32(ids)
	for _, id := range ids {
		out = append(out, s.edges[id])
	}
	return out
}

// UniqueFieldIDs returns ids of fields declared unique on the node
// type. Returns nil for unknown types. Excludes deprecated fields.
func (r *Registry) UniqueFieldIDs(typeID int32) []uint32 {
	n := r.load().nodes[typeID]
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
	n := r.load().nodes[typeID]
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
	n := r.load().nodes[typeID]
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
	n := r.load().nodes[typeID]
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
	n := r.load().nodes[typeID]
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
	n := r.load().nodes[typeID]
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
	n := r.load().nodes[typeID]
	if n == nil || n.SubjectField == nil {
		return ""
	}
	return *n.SubjectField
}

// ValidateAll cross-references edge from_type_id / to_type_id against
// registered nodes, and verifies every FieldDef.RefTypeID points at a
// registered node.
func (r *Registry) ValidateAll() []string {
	s := r.load()
	var errs []string
	for _, e := range s.edges {
		if _, ok := s.nodes[e.FromTypeID]; !ok {
			errs = append(errs, fmt.Sprintf(
				"edge %q (edge_id=%d) references unknown from_type_id %d",
				e.Name, e.EdgeID, e.FromTypeID,
			))
		}
		if _, ok := s.nodes[e.ToTypeID]; !ok {
			errs = append(errs, fmt.Sprintf(
				"edge %q (edge_id=%d) references unknown to_type_id %d",
				e.Name, e.EdgeID, e.ToTypeID,
			))
		}
	}
	for _, n := range s.nodes {
		for i := range n.Fields {
			f := &n.Fields[i]
			if f.RefTypeID != nil {
				if _, ok := s.nodes[*f.RefTypeID]; !ok {
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
