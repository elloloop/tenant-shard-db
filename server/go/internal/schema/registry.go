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
// when the type_id collides with an already-registered type.
var ErrDuplicateRegistration = errors.New("schema: duplicate registration")

// ErrSchemaConflict is returned by RegisterOrVerifyNode /
// RegisterOrVerifyEdge under the establish-or-reject policy when a type
// is already registered under the same type_id but with a DIFFERENT
// definition. The errs package maps this to FAILED_PRECONDITION at the
// gRPC boundary. Online evolution / ALTER is out of scope, so a
// conflicting redefinition is always rejected.
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
	nodes map[int32]*NodeTypeDef
	edges map[int32]*EdgeTypeDef
	// reservedTypeIDs / reservedEdgeIDs are the schema-level tombstone
	// lists (ADR-032). They are pure metadata consulted by the compat
	// checker; the runtime registry never reads them on the hot path.
	reservedTypeIDs []int32
	reservedEdgeIDs []int32
}

// clone returns a shallow copy of s with fresh top-level maps. The
// values (*NodeTypeDef / *EdgeTypeDef) are shared with the parent
// snapshot — they are never mutated in place (establish-or-reject never
// edits an existing type), so sharing them is safe and keeps a
// registration O(types) rather than O(fields).
func (s *snapshot) clone() *snapshot {
	out := &snapshot{
		nodes:           make(map[int32]*NodeTypeDef, len(s.nodes)+1),
		edges:           make(map[int32]*EdgeTypeDef, len(s.edges)+1),
		reservedTypeIDs: s.reservedTypeIDs,
		reservedEdgeIDs: s.reservedEdgeIDs,
	}
	for k, v := range s.nodes {
		out.nodes[k] = v
	}
	for k, v := range s.edges {
		out.edges[k] = v
	}
	return out
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
		nodes: map[int32]*NodeTypeDef{},
		edges: map[int32]*EdgeTypeDef{},
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
// registry is frozen, ErrDuplicateRegistration if the type_id is already
// taken, or a validation error if nt is malformed.
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
		return fmt.Errorf("%w: cannot register node type_id %d", ErrFrozen, nt.TypeID)
	}
	cur := r.load()
	if _, ok := cur.nodes[nt.TypeID]; ok {
		return fmt.Errorf(
			"%w: type_id %d already registered",
			ErrDuplicateRegistration, nt.TypeID,
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
		return fmt.Errorf("%w: cannot register edge type_id %d", ErrFrozen, et.EdgeID)
	}
	cur := r.load()
	if _, ok := cur.edges[et.EdgeID]; ok {
		return fmt.Errorf(
			"%w: edge_id %d already registered",
			ErrDuplicateRegistration, et.EdgeID,
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
	r.snap.Store(next)
}

// publishEdge builds a fresh snapshot from cur with et added and stores
// it atomically. Caller MUST hold r.mu.
func (r *Registry) publishEdge(cur *snapshot, et *EdgeTypeDef) {
	next := cur.clone()
	next.edges[et.EdgeID] = et
	r.snap.Store(next)
}

// RegisterOrVerifyNode implements the establish-or-reject policy for
// SELF-DESCRIBING WRITES. Types are identified solely by type_id
// (name-free, ADR-031):
//
//   - absent (type_id not registered): register nt and recompute the
//     fingerprint. Returns (registered=true, nil).
//   - present and byte-identical to the stored definition: no-op.
//     Returns (false, nil) — idempotent.
//   - present but DIFFERENT: reject with ErrSchemaConflict.
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

	if byID, hasID := cur.nodes[nt.TypeID]; hasID {
		if nodeDefEqual(byID, nt) {
			return false, nil // idempotent no-op
		}
		return false, fmt.Errorf(
			"%w: node type_id %d differs from the registered definition",
			ErrSchemaConflict, nt.TypeID)
	}

	// Absent → establish. Frozen registries cannot grow.
	if r.frozen.Load() {
		return false, fmt.Errorf("%w: cannot register node type_id %d", ErrFrozen, nt.TypeID)
	}
	r.publishNode(cur, nt)
	if err := r.recomputeFingerprintLocked(); err != nil {
		return true, err
	}
	return true, nil
}

// RegisterOrVerifyEdge is the edge counterpart of RegisterOrVerifyNode.
// Same establish-or-reject semantics, keyed by edge_id.
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

	if byID, hasID := cur.edges[et.EdgeID]; hasID {
		if edgeDefEqual(byID, et) {
			return false, nil // idempotent no-op
		}
		return false, fmt.Errorf(
			"%w: edge_id %d differs from the registered definition",
			ErrSchemaConflict, et.EdgeID)
	}

	if r.frozen.Load() {
		return false, fmt.Errorf("%w: cannot register edge type_id %d", ErrFrozen, et.EdgeID)
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

// NodeType returns the node type by id (int / int32 / int64 / uint32).
// Returns nil if not found. Lock-free. Name-free per ADR-031: types are
// addressed only by their numeric id.
func (r *Registry) NodeType(id any) *NodeTypeDef {
	s := r.load()
	switch v := id.(type) {
	case int32:
		return s.nodes[v]
	case int:
		return s.nodes[int32(v)]
	case int64:
		return s.nodes[int32(v)]
	case uint32:
		return s.nodes[int32(v)]
	}
	return nil
}

// NodeTypeByID is the explicit-id helper.
func (r *Registry) NodeTypeByID(id int32) *NodeTypeDef { return r.load().nodes[id] }

// Empty reports whether the registry holds no node or edge types. With the
// empty-boot model (ADR-031) the registry is always non-nil but may be
// genuinely empty before any self-describing write has registered a type.
// An empty registry is the SCHEMA-LESS mode: handlers tolerate unknown
// type_ids and id-keyed (digit) coordinates exactly as they did for the
// pre-ADR-031 nil registry (the issue #545 escape hatch). A nil receiver
// reports empty.
func (r *Registry) Empty() bool {
	if r == nil {
		return true
	}
	s := r.load()
	return len(s.nodes) == 0 && len(s.edges) == 0
}

// EdgeType is the edge counterpart of NodeType.
func (r *Registry) EdgeType(id any) *EdgeTypeDef {
	s := r.load()
	switch v := id.(type) {
	case int32:
		return s.edges[v]
	case int:
		return s.edges[int32(v)]
	case int64:
		return s.edges[int32(v)]
	case uint32:
		return s.edges[int32(v)]
	}
	return nil
}

// EdgeTypeByID is the explicit-id helper.
func (r *Registry) EdgeTypeByID(id int32) *EdgeTypeDef { return r.load().edges[id] }

// ReservedTypeIDs returns the schema-level tombstone list of removed
// node-type ids (ADR-032). The returned slice is the snapshot's own
// backing array; callers must not mutate it.
func (r *Registry) ReservedTypeIDs() []int32 { return r.load().reservedTypeIDs }

// ReservedEdgeIDs returns the schema-level tombstone list of removed
// edge-type ids (ADR-032).
func (r *Registry) ReservedEdgeIDs() []int32 { return r.load().reservedEdgeIDs }

// setReservedIDs publishes the schema-level reserved type/edge id lists
// onto the current snapshot. Used by the JSON loader; callers must hold
// no special lock since it runs during single-threaded load.
func (r *Registry) setReservedIDs(typeIDs, edgeIDs []int32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	next := r.load().clone()
	next.reservedTypeIDs = typeIDs
	next.reservedEdgeIDs = edgeIDs
	r.snap.Store(next)
}

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

// PIIFieldIDs returns the field_ids of PII-marked, non-deprecated fields
// on the type. Returns nil for unknown types. Name-free per ADR-031.
func (r *Registry) PIIFieldIDs(typeID int32) []uint32 {
	n := r.load().nodes[typeID]
	if n == nil {
		return nil
	}
	out := make([]uint32, 0)
	for i := range n.Fields {
		f := &n.Fields[i]
		if f.PII && !f.Deprecated {
			out = append(out, f.FieldID)
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

// SubjectFieldID returns the subject field_id for a node type, or
// (0, false) if unset / unknown. Name-free per ADR-031.
func (r *Registry) SubjectFieldID(typeID int32) (uint32, bool) {
	n := r.load().nodes[typeID]
	if n == nil || n.SubjectField == nil {
		return 0, false
	}
	return *n.SubjectField, true
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
				"edge_id=%d references unknown from_type_id %d",
				e.EdgeID, e.FromTypeID,
			))
		}
		if _, ok := s.nodes[e.ToTypeID]; !ok {
			errs = append(errs, fmt.Sprintf(
				"edge_id=%d references unknown to_type_id %d",
				e.EdgeID, e.ToTypeID,
			))
		}
	}
	for _, n := range s.nodes {
		for i := range n.Fields {
			f := &n.Fields[i]
			if f.RefTypeID != nil {
				if _, ok := s.nodes[*f.RefTypeID]; !ok {
					errs = append(errs, fmt.Sprintf(
						"field_id %d in node type_id %d references unknown type_id %d",
						f.FieldID, n.TypeID, *f.RefTypeID,
					))
				}
			}
		}
	}
	return errs
}
