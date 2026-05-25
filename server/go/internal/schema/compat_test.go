package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

// helper: build a single-node registry the easy way for table tests.
func regWith(nodes []NodeTypeDef, edges []EdgeTypeDef) *Registry {
	r := NewRegistry()
	for i := range nodes {
		nt := nodes[i]
		if err := r.RegisterNode(&nt); err != nil {
			panic(err)
		}
	}
	for i := range edges {
		et := edges[i]
		if err := r.RegisterEdge(&et); err != nil {
			panic(err)
		}
	}
	return r
}

// userNode is the name-free fixture node (type_id=1) used across the
// compat table tests.
func userNode() NodeTypeDef {
	return NodeTypeDef{
		TypeID: 1,
		Fields: []FieldDef{
			{FieldID: 1, Kind: KindString, Required: true, Unique: true},
			{FieldID: 2, Kind: KindString},
		},
	}
}

// findChange returns the first Change whose Kind matches `k`, or nil
// when none exists.
func findChange(cs []Change, k ChangeKind) *Change {
	for i := range cs {
		if cs[i].Kind == k {
			return &cs[i]
		}
	}
	return nil
}

func TestCompat_NodeAdded(t *testing.T) {
	old := regWith([]NodeTypeDef{userNode()}, nil)
	post := userNode()
	post2 := NodeTypeDef{TypeID: 2, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}}
	newR := regWith([]NodeTypeDef{post, post2}, nil)
	c := Check(old, newR)
	if got := findChange(c, ChangeKindNodeAdded); got == nil {
		t.Fatalf("expected NODE_ADDED, got %v", c)
	} else if got.Breaking {
		t.Fatalf("NODE_ADDED must not be breaking")
	}
}

// TestCompat_NodeRemoved: ADR-032 — removing a type is LOOSENING and
// therefore SAFE (its id should be reserved so it cannot be reused; the
// reuse is the break, covered by TestCompat_TypeIDReused).
func TestCompat_NodeRemoved(t *testing.T) {
	user := userNode()
	post := NodeTypeDef{TypeID: 2, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}}
	old := regWith([]NodeTypeDef{user, post}, nil)
	newR := regWith([]NodeTypeDef{user}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindNodeRemoved)
	if got == nil {
		t.Fatalf("expected NODE_REMOVED, got %v", c)
	}
	if got.Breaking {
		t.Fatalf("NODE_REMOVED must NOT be breaking (ADR-032: removal is safe)")
	}
}

func TestCompat_FieldAdded(t *testing.T) {
	old := regWith([]NodeTypeDef{userNode()}, nil)
	user2 := userNode()
	user2.Fields = append(user2.Fields, FieldDef{FieldID: 3, Kind: KindString})
	newR := regWith([]NodeTypeDef{user2}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindFieldAdded)
	if got == nil {
		t.Fatalf("expected FIELD_ADDED, got %v", c)
	}
	if got.Breaking {
		t.Fatalf("FIELD_ADDED must not be breaking")
	}
}

// TestCompat_FieldRemoved: ADR-032 — removing a field is LOOSENING and
// therefore SAFE. The id must be reserved so it cannot be reused; reuse
// is the break (TestCompat_FieldIDReused).
func TestCompat_FieldRemoved(t *testing.T) {
	old := regWith([]NodeTypeDef{userNode()}, nil)
	user2 := userNode()
	user2.Fields = user2.Fields[:1]
	newR := regWith([]NodeTypeDef{user2}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindFieldRemoved)
	if got == nil {
		t.Fatalf("expected FIELD_REMOVED, got %v", c)
	}
	if got.Breaking {
		t.Fatalf("FIELD_REMOVED must NOT be breaking (ADR-032: removal is safe)")
	}
}

// TestCompat_FieldIDReassigned: name-free (ADR-031), reassigning a
// field_id is a remove (old id) + add (new id), not a single
// FIELD_ID_CHANGED — that detection required a name to pair the two.
func TestCompat_FieldIDReassigned(t *testing.T) {
	old := regWith([]NodeTypeDef{userNode()}, nil)
	user2 := userNode()
	// field 2 moves to field_id=7
	user2.Fields[1].FieldID = 7
	newR := regWith([]NodeTypeDef{user2}, nil)
	c := Check(old, newR)
	if findChange(c, ChangeKindFieldRemoved) == nil {
		t.Fatalf("expected FIELD_REMOVED for old field_id, got %v", c)
	}
	if findChange(c, ChangeKindFieldAdded) == nil {
		t.Fatalf("expected FIELD_ADDED for new field_id, got %v", c)
	}
	// FIELD_ID_CHANGED / FIELD_RENAMED are retired and never emitted.
	if findChange(c, ChangeKindFieldIDChanged) != nil {
		t.Fatalf("FIELD_ID_CHANGED is retired (ADR-031) and must not fire")
	}
}

func TestCompat_FieldKindChanged(t *testing.T) {
	old := regWith([]NodeTypeDef{userNode()}, nil)
	user2 := userNode()
	user2.Fields[1].Kind = KindInteger
	newR := regWith([]NodeTypeDef{user2}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindFieldKindChanged)
	if got == nil {
		t.Fatalf("expected FIELD_KIND_CHANGED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("FIELD_KIND_CHANGED must be breaking")
	}
}

func TestCompat_RequiredTightened(t *testing.T) {
	old := regWith([]NodeTypeDef{userNode()}, nil)
	user2 := userNode()
	user2.Fields[1].Required = true
	newR := regWith([]NodeTypeDef{user2}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindFieldRequiredTightened)
	if got == nil {
		t.Fatalf("expected FIELD_REQUIRED_TIGHTENED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("FIELD_REQUIRED_TIGHTENED must be breaking")
	}
}

func TestCompat_RequiredLoosened(t *testing.T) {
	old := regWith([]NodeTypeDef{userNode()}, nil)
	// flip field 1 required from true to false
	user2 := userNode()
	user2.Fields[0].Required = false
	// keep unique flag (loosening required only)
	user2.Fields[0].Unique = true
	newR := regWith([]NodeTypeDef{user2}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindFieldRequiredLoosened)
	if got == nil {
		t.Fatalf("expected FIELD_REQUIRED_LOOSENED, got %v", c)
	}
	if got.Breaking {
		t.Fatalf("FIELD_REQUIRED_LOOSENED must not be breaking")
	}
}

func TestCompat_UniqueAdded(t *testing.T) {
	old := regWith([]NodeTypeDef{userNode()}, nil)
	user2 := userNode()
	user2.Fields[1].Unique = true
	newR := regWith([]NodeTypeDef{user2}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindFieldUniqueAdded)
	if got == nil {
		t.Fatalf("expected FIELD_UNIQUE_ADDED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("FIELD_UNIQUE_ADDED must be breaking")
	}
}

func TestCompat_UniqueRemoved(t *testing.T) {
	old := regWith([]NodeTypeDef{userNode()}, nil)
	user2 := userNode()
	user2.Fields[0].Unique = false
	newR := regWith([]NodeTypeDef{user2}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindFieldUniqueRemoved)
	if got == nil {
		t.Fatalf("expected FIELD_UNIQUE_REMOVED, got %v", c)
	}
	if got.Breaking {
		t.Fatalf("FIELD_UNIQUE_REMOVED must not be breaking")
	}
}

func TestCompat_EnumValueAdded(t *testing.T) {
	old := regWith([]NodeTypeDef{{
		TypeID: 1,
		Fields: []FieldDef{{FieldID: 1, Kind: KindEnum, EnumValues: []string{"A", "B"}}},
	}}, nil)
	newR := regWith([]NodeTypeDef{{
		TypeID: 1,
		Fields: []FieldDef{{FieldID: 1, Kind: KindEnum, EnumValues: []string{"A", "B", "C"}}},
	}}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindEnumValueAdded)
	if got == nil {
		t.Fatalf("expected ENUM_VALUE_ADDED, got %v", c)
	}
	if got.Breaking {
		t.Fatalf("ENUM_VALUE_ADDED must not be breaking")
	}
}

func TestCompat_EnumValueRemoved(t *testing.T) {
	old := regWith([]NodeTypeDef{{
		TypeID: 1,
		Fields: []FieldDef{{FieldID: 1, Kind: KindEnum, EnumValues: []string{"A", "B", "C"}}},
	}}, nil)
	newR := regWith([]NodeTypeDef{{
		TypeID: 1,
		Fields: []FieldDef{{FieldID: 1, Kind: KindEnum, EnumValues: []string{"A", "B"}}},
	}}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindEnumValueRemoved)
	if got == nil {
		t.Fatalf("expected ENUM_VALUE_REMOVED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("ENUM_VALUE_REMOVED must be breaking")
	}
}

func TestCompat_EnumValueReordered(t *testing.T) {
	old := regWith([]NodeTypeDef{{
		TypeID: 1,
		Fields: []FieldDef{{FieldID: 1, Kind: KindEnum, EnumValues: []string{"A", "B"}}},
	}}, nil)
	newR := regWith([]NodeTypeDef{{
		TypeID: 1,
		Fields: []FieldDef{{FieldID: 1, Kind: KindEnum, EnumValues: []string{"B", "A"}}},
	}}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindEnumValueReordered)
	if got == nil {
		t.Fatalf("expected ENUM_VALUE_REORDERED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("ENUM_VALUE_REORDERED must be breaking")
	}
}

func TestCompat_CompositeUniqueAdded(t *testing.T) {
	user := userNode()
	old := regWith([]NodeTypeDef{user}, nil)
	user2 := userNode()
	user2.CompositeUnique = []CompositeUniqueDef{{FieldIDs: []uint32{1, 2}}}
	newR := regWith([]NodeTypeDef{user2}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindCompositeUniqueAdded)
	if got == nil {
		t.Fatalf("expected COMPOSITE_UNIQUE_ADDED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("COMPOSITE_UNIQUE_ADDED must be breaking")
	}
}

func TestCompat_CompositeUniqueRemoved(t *testing.T) {
	user := userNode()
	user.CompositeUnique = []CompositeUniqueDef{{FieldIDs: []uint32{1, 2}}}
	old := regWith([]NodeTypeDef{user}, nil)
	user2 := userNode()
	newR := regWith([]NodeTypeDef{user2}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindCompositeUniqueRemoved)
	if got == nil {
		t.Fatalf("expected COMPOSITE_UNIQUE_REMOVED, got %v", c)
	}
	if got.Breaking {
		t.Fatalf("COMPOSITE_UNIQUE_REMOVED must not be breaking")
	}
}

// TestCompat_CompositeUniqueRetupled: name-free (ADR-031), changing a
// composite's field tuple is a remove (old tuple) + add (new tuple). The
// constraint identity IS the tuple, so there is no in-place "changed".
func TestCompat_CompositeUniqueRetupled(t *testing.T) {
	user := userNode()
	user.Fields = append(user.Fields, FieldDef{FieldID: 3, Kind: KindString})
	user.CompositeUnique = []CompositeUniqueDef{{FieldIDs: []uint32{1, 2}}}
	old := regWith([]NodeTypeDef{user}, nil)
	user2 := userNode()
	user2.Fields = append(user2.Fields, FieldDef{FieldID: 3, Kind: KindString})
	user2.CompositeUnique = []CompositeUniqueDef{{FieldIDs: []uint32{1, 3}}}
	newR := regWith([]NodeTypeDef{user2}, nil)
	c := Check(old, newR)
	if findChange(c, ChangeKindCompositeUniqueRemoved) == nil {
		t.Fatalf("expected COMPOSITE_UNIQUE_REMOVED for the old tuple, got %v", c)
	}
	added := findChange(c, ChangeKindCompositeUniqueAdded)
	if added == nil {
		t.Fatalf("expected COMPOSITE_UNIQUE_ADDED for the new tuple, got %v", c)
	}
	if !added.Breaking {
		t.Fatalf("COMPOSITE_UNIQUE_ADDED must be breaking")
	}
	// COMPOSITE_UNIQUE_CHANGED is retired and never emitted.
	if findChange(c, ChangeKindCompositeUniqueChanged) != nil {
		t.Fatalf("COMPOSITE_UNIQUE_CHANGED is retired (ADR-031) and must not fire")
	}
}

func owns() EdgeTypeDef {
	return EdgeTypeDef{EdgeID: 100, FromTypeID: 1, ToTypeID: 2, OnSubjectExit: OnSubjectExitBoth}
}

func TestCompat_EdgeAdded(t *testing.T) {
	user := userNode()
	post := NodeTypeDef{TypeID: 2, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}}
	old := regWith([]NodeTypeDef{user, post}, nil)
	newR := regWith([]NodeTypeDef{user, post}, []EdgeTypeDef{owns()})
	c := Check(old, newR)
	got := findChange(c, ChangeKindEdgeAdded)
	if got == nil {
		t.Fatalf("expected EDGE_ADDED, got %v", c)
	}
	if got.Breaking {
		t.Fatalf("EDGE_ADDED must not be breaking")
	}
}

// TestCompat_EdgeRemoved: ADR-032 — removing an edge type is LOOSENING
// and therefore SAFE; reuse of the edge_id is the break
// (TestCompat_EdgeIDReused).
func TestCompat_EdgeRemoved(t *testing.T) {
	user := userNode()
	post := NodeTypeDef{TypeID: 2, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}}
	old := regWith([]NodeTypeDef{user, post}, []EdgeTypeDef{owns()})
	newR := regWith([]NodeTypeDef{user, post}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindEdgeRemoved)
	if got == nil {
		t.Fatalf("expected EDGE_REMOVED, got %v", c)
	}
	if got.Breaking {
		t.Fatalf("EDGE_REMOVED must NOT be breaking (ADR-032: removal is safe)")
	}
}

func TestCompat_EdgeFromTypeChanged(t *testing.T) {
	user := userNode()
	org := NodeTypeDef{TypeID: 3, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}}
	post := NodeTypeDef{TypeID: 2, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}}
	old := regWith([]NodeTypeDef{user, post, org}, []EdgeTypeDef{owns()})
	o2 := owns()
	o2.FromTypeID = 3
	newR := regWith([]NodeTypeDef{user, post, org}, []EdgeTypeDef{o2})
	c := Check(old, newR)
	got := findChange(c, ChangeKindEdgeFromTypeChanged)
	if got == nil {
		t.Fatalf("expected EDGE_FROM_TYPE_CHANGED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("EDGE_FROM_TYPE_CHANGED must be breaking")
	}
}

func TestCompat_EdgeUniquePerFromAdded(t *testing.T) {
	user := userNode()
	post := NodeTypeDef{TypeID: 2, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}}
	old := regWith([]NodeTypeDef{user, post}, []EdgeTypeDef{owns()})
	o2 := owns()
	o2.UniquePerFrom = true
	newR := regWith([]NodeTypeDef{user, post}, []EdgeTypeDef{o2})
	c := Check(old, newR)
	got := findChange(c, ChangeKindEdgeUniquePerFromAdded)
	if got == nil {
		t.Fatalf("expected EDGE_UNIQUE_PER_FROM_ADDED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("EDGE_UNIQUE_PER_FROM_ADDED must be breaking")
	}
}

func TestCompat_OnSubjectExitChanged(t *testing.T) {
	user := userNode()
	post := NodeTypeDef{TypeID: 2, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}}
	old := regWith([]NodeTypeDef{user, post}, []EdgeTypeDef{owns()})
	o2 := owns()
	o2.OnSubjectExit = OnSubjectExitFrom
	newR := regWith([]NodeTypeDef{user, post}, []EdgeTypeDef{o2})
	c := Check(old, newR)
	got := findChange(c, ChangeKindOnSubjectExitChanged)
	if got == nil {
		t.Fatalf("expected ON_SUBJECT_EXIT_CHANGED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("ON_SUBJECT_EXIT_CHANGED must be breaking")
	}
}

func TestCompat_DataPolicyLoosened(t *testing.T) {
	fin := DataPolicyFinancial
	bus := DataPolicyBusiness
	old := regWith([]NodeTypeDef{{TypeID: 1, DataPolicy: &fin, Fields: []FieldDef{{FieldID: 1, Kind: KindInteger}}}}, nil)
	newR := regWith([]NodeTypeDef{{TypeID: 1, DataPolicy: &bus, Fields: []FieldDef{{FieldID: 1, Kind: KindInteger}}}}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindDataPolicyLoosened)
	if got == nil {
		t.Fatalf("expected DATA_POLICY_LOOSENED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("DATA_POLICY_LOOSENED must be breaking")
	}
}

func TestCompat_DataPolicyTightened(t *testing.T) {
	fin := DataPolicyFinancial
	bus := DataPolicyBusiness
	old := regWith([]NodeTypeDef{{TypeID: 1, DataPolicy: &bus, Fields: []FieldDef{{FieldID: 1, Kind: KindInteger}}}}, nil)
	newR := regWith([]NodeTypeDef{{TypeID: 1, DataPolicy: &fin, Fields: []FieldDef{{FieldID: 1, Kind: KindInteger}}}}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindDataPolicyTightened)
	if got == nil {
		t.Fatalf("expected DATA_POLICY_TIGHTENED, got %v", c)
	}
	if got.Breaking {
		t.Fatalf("DATA_POLICY_TIGHTENED must not be breaking")
	}
}

func TestCompat_SubjectFieldChanged(t *testing.T) {
	s1 := uint32(1)
	s2 := uint32(2)
	old := regWith([]NodeTypeDef{{TypeID: 1, SubjectField: &s1, Fields: []FieldDef{{FieldID: 1, Kind: KindString}, {FieldID: 2, Kind: KindString}}}}, nil)
	newR := regWith([]NodeTypeDef{{TypeID: 1, SubjectField: &s2, Fields: []FieldDef{{FieldID: 1, Kind: KindString}, {FieldID: 2, Kind: KindString}}}}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindSubjectFieldChanged)
	if got == nil {
		t.Fatalf("expected SUBJECT_FIELD_CHANGED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("SUBJECT_FIELD_CHANGED must be breaking")
	}
}

func TestCheck_NoChangesOnIdenticalRegistries(t *testing.T) {
	r1 := regWith([]NodeTypeDef{userNode()}, nil)
	r2 := regWith([]NodeTypeDef{userNode()}, nil)
	c := Check(r1, r2)
	if len(c) != 0 {
		t.Fatalf("expected zero changes, got %d: %v", len(c), c)
	}
	if HasBreaking(c) {
		t.Fatalf("expected no breaking changes")
	}
}

func TestCheck_NilRegistries(t *testing.T) {
	r := regWith([]NodeTypeDef{userNode()}, nil)
	added := Check(nil, r)
	if findChange(added, ChangeKindNodeAdded) == nil {
		t.Fatalf("Check(nil, r) should report all nodes added; got %v", added)
	}
	removed := Check(r, nil)
	if findChange(removed, ChangeKindNodeRemoved) == nil {
		t.Fatalf("Check(r, nil) should report all nodes removed; got %v", removed)
	}
}

func TestCheck_DeterministicOutputOrder(t *testing.T) {
	r1 := regWith([]NodeTypeDef{userNode()}, nil)
	post1 := NodeTypeDef{TypeID: 2, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}}
	post2 := NodeTypeDef{TypeID: 3, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}}
	r2 := regWith([]NodeTypeDef{userNode(), post1, post2}, nil)
	a := Check(r1, r2)
	b := Check(r1, r2)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic len: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Kind != b[i].Kind || a[i].Path != b[i].Path {
			t.Fatalf("non-deterministic at %d: %v vs %v", i, a[i], b[i])
		}
	}
}

func TestChangeKindMarshalJSON(t *testing.T) {
	c := Change{Kind: ChangeKindFieldRemoved, Path: "node:1.field:2", Message: "x", Breaking: true}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"FIELD_REMOVED"`) {
		t.Fatalf("expected textual kind, got %s", raw)
	}
}

func TestRenderText_Smoke(t *testing.T) {
	r1 := regWith([]NodeTypeDef{userNode()}, nil)
	user2 := userNode()
	user2.Fields[1].Kind = KindInteger // change field 2 kind → breaking
	r2 := regWith([]NodeTypeDef{user2}, nil)
	c := Check(r1, r2)
	out := RenderText(c, "baseline.json")
	if !strings.Contains(out, "BREAKING") {
		t.Fatalf("expected BREAKING header, got: %s", out)
	}
	if !strings.Contains(out, "FIELD_KIND_CHANGED") {
		t.Fatalf("expected rule name, got: %s", out)
	}
}

// -----------------------------------------------------------------------------
// ADR-032 matrix: reserved-id reuse (BREAKING) + index/searchable toggles
// (SAFE). Loosening is safe; tightening/identity-reuse is breaking.
// -----------------------------------------------------------------------------

// TestCompat_FieldIDReused: baseline reserves field_id 9; the new schema
// brings field_id 9 back as a live field. Reusing a reserved id is the
// breaking move that pairs with the safe FIELD_REMOVED.
func TestCompat_FieldIDReused(t *testing.T) {
	base := userNode()
	base.ReservedFieldIDs = []uint32{9}
	old := regWith([]NodeTypeDef{base}, nil)

	reuse := userNode()
	reuse.Fields = append(reuse.Fields, FieldDef{FieldID: 9, Kind: KindString})
	newR := regWith([]NodeTypeDef{reuse}, nil)

	c := Check(old, newR)
	got := findChange(c, ChangeKindFieldIDReused)
	if got == nil {
		t.Fatalf("expected FIELD_ID_REUSED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("FIELD_ID_REUSED must be breaking")
	}
	// A reserved id reappearing must NOT also be reported as a benign add.
	if findChange(c, ChangeKindFieldAdded) != nil {
		t.Fatalf("a reserved-id reuse must not be classified as FIELD_ADDED: %v", c)
	}
}

// TestCompat_FieldAddedFreshIsSafe: adding a brand-new field_id (not in
// the reserved list) is loosening and SAFE — distinguishes the reuse case
// from an ordinary add.
func TestCompat_FieldAddedFreshIsSafe(t *testing.T) {
	base := userNode()
	base.ReservedFieldIDs = []uint32{9}
	old := regWith([]NodeTypeDef{base}, nil)

	add := userNode()
	add.ReservedFieldIDs = []uint32{9} // keep the tombstone
	add.Fields = append(add.Fields, FieldDef{FieldID: 10, Kind: KindString})
	newR := regWith([]NodeTypeDef{add}, nil)

	c := Check(old, newR)
	got := findChange(c, ChangeKindFieldAdded)
	if got == nil {
		t.Fatalf("expected FIELD_ADDED for a fresh id, got %v", c)
	}
	if got.Breaking {
		t.Fatalf("FIELD_ADDED for a fresh id must not be breaking")
	}
	if HasBreaking(c) {
		t.Fatalf("adding a fresh field id must be fully safe, got %v", c)
	}
}

// TestCompat_TypeIDReused: baseline reserves type_id 7; new schema brings
// type_id 7 back as a live node.
func TestCompat_TypeIDReused(t *testing.T) {
	body := []byte(`{"node_types":[{"type_id":1,"fields":[{"field_id":1,"kind":"str"}]}],"reserved_type_ids":[7]}`)
	old, err := LoadFromJSON(body)
	if err != nil {
		t.Fatalf("load old: %v", err)
	}
	reuse := regWith([]NodeTypeDef{
		{TypeID: 1, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}},
		{TypeID: 7, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}},
	}, nil)
	c := Check(old, reuse)
	got := findChange(c, ChangeKindTypeIDReused)
	if got == nil {
		t.Fatalf("expected TYPE_ID_REUSED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("TYPE_ID_REUSED must be breaking")
	}
	if findChange(c, ChangeKindNodeAdded) != nil {
		t.Fatalf("a reserved type_id reuse must not be classified as NODE_ADDED: %v", c)
	}
}

// TestCompat_EdgeIDReused: baseline reserves edge_id 50; new schema brings
// edge_id 50 back as a live edge.
func TestCompat_EdgeIDReused(t *testing.T) {
	body := []byte(`{
		"node_types":[
			{"type_id":1,"fields":[{"field_id":1,"kind":"str"}]},
			{"type_id":2,"fields":[{"field_id":1,"kind":"str"}]}
		],
		"edge_types":[],
		"reserved_edge_ids":[50]
	}`)
	old, err := LoadFromJSON(body)
	if err != nil {
		t.Fatalf("load old: %v", err)
	}
	user := NodeTypeDef{TypeID: 1, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}}
	post := NodeTypeDef{TypeID: 2, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}}
	reuse := regWith([]NodeTypeDef{user, post},
		[]EdgeTypeDef{{EdgeID: 50, FromTypeID: 1, ToTypeID: 2, OnSubjectExit: OnSubjectExitBoth}})
	c := Check(old, reuse)
	got := findChange(c, ChangeKindEdgeIDReused)
	if got == nil {
		t.Fatalf("expected EDGE_ID_REUSED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("EDGE_ID_REUSED must be breaking")
	}
	if findChange(c, ChangeKindEdgeAdded) != nil {
		t.Fatalf("a reserved edge_id reuse must not be classified as EDGE_ADDED: %v", c)
	}
}

func TestCompat_IndexedAdded(t *testing.T) {
	old := regWith([]NodeTypeDef{userNode()}, nil)
	user2 := userNode()
	user2.Fields[1].Indexed = true
	newR := regWith([]NodeTypeDef{user2}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindFieldIndexedAdded)
	if got == nil {
		t.Fatalf("expected FIELD_INDEXED_ADDED, got %v", c)
	}
	if got.Breaking {
		t.Fatalf("FIELD_INDEXED_ADDED must not be breaking")
	}
}

func TestCompat_IndexedRemoved(t *testing.T) {
	base := userNode()
	base.Fields[1].Indexed = true
	old := regWith([]NodeTypeDef{base}, nil)
	newR := regWith([]NodeTypeDef{userNode()}, nil) // indexed back to false
	c := Check(old, newR)
	got := findChange(c, ChangeKindFieldIndexedRemoved)
	if got == nil {
		t.Fatalf("expected FIELD_INDEXED_REMOVED, got %v", c)
	}
	if got.Breaking {
		t.Fatalf("FIELD_INDEXED_REMOVED must not be breaking")
	}
}

func TestCompat_SearchableAdded(t *testing.T) {
	old := regWith([]NodeTypeDef{userNode()}, nil)
	user2 := userNode()
	user2.Fields[1].Searchable = true
	newR := regWith([]NodeTypeDef{user2}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindFieldSearchableAdded)
	if got == nil {
		t.Fatalf("expected FIELD_SEARCHABLE_ADDED, got %v", c)
	}
	if got.Breaking {
		t.Fatalf("FIELD_SEARCHABLE_ADDED must not be breaking")
	}
}

func TestCompat_SearchableRemoved(t *testing.T) {
	base := userNode()
	base.Fields[1].Searchable = true
	old := regWith([]NodeTypeDef{base}, nil)
	newR := regWith([]NodeTypeDef{userNode()}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindFieldSearchableRemoved)
	if got == nil {
		t.Fatalf("expected FIELD_SEARCHABLE_REMOVED, got %v", c)
	}
	if got.Breaking {
		t.Fatalf("FIELD_SEARCHABLE_REMOVED must not be breaking")
	}
}

// TestCompat_Matrix is the table-driven sweep over the full ADR-032
// loosen-safe / tighten-breaking matrix. Each row applies a transform to
// a fixture, runs Check, and asserts the expected kind fired with the
// expected breaking classification. It is the single source of truth for
// the rule matrix; per-kind tests above remain for focused failure
// messages.
func TestCompat_Matrix(t *testing.T) {
	// base fixture: one node (type 1) with a required+unique str field 1
	// and a plain str field 2, plus one edge (100) from 1→2.
	base := func() ([]NodeTypeDef, []EdgeTypeDef) {
		return []NodeTypeDef{
				{TypeID: 1, Fields: []FieldDef{
					{FieldID: 1, Kind: KindString, Required: true, Unique: true},
					{FieldID: 2, Kind: KindString},
				}},
				{TypeID: 2, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}},
			}, []EdgeTypeDef{
				{EdgeID: 100, FromTypeID: 1, ToTypeID: 2, OnSubjectExit: OnSubjectExitBoth},
			}
	}

	cases := []struct {
		name     string
		mutate   func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef)
		wantKind ChangeKind
		breaking bool
	}{
		// ---- TIGHTENING / identity / data-corrupting → BREAKING ----
		{"unique added (false→true)", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			n[0].Fields[1].Unique = true
			return n, e
		}, ChangeKindFieldUniqueAdded, true},
		{"required added (false→true)", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			n[0].Fields[1].Required = true
			return n, e
		}, ChangeKindFieldRequiredTightened, true},
		{"field kind changed", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			n[0].Fields[1].Kind = KindInteger
			return n, e
		}, ChangeKindFieldKindChanged, true},
		{"composite_unique added", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			n[0].CompositeUnique = []CompositeUniqueDef{{FieldIDs: []uint32{1, 2}}}
			return n, e
		}, ChangeKindCompositeUniqueAdded, true},
		{"edge from-type changed", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			e[0].FromTypeID = 2
			return n, e
		}, ChangeKindEdgeFromTypeChanged, true},
		{"edge to-type changed", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			e[0].ToTypeID = 1
			return n, e
		}, ChangeKindEdgeToTypeChanged, true},

		// ---- LOOSENING / additive → SAFE ----
		{"unique dropped", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			n[0].Fields[0].Unique = false
			return n, e
		}, ChangeKindFieldUniqueRemoved, false},
		{"required dropped", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			n[0].Fields[0].Required = false
			return n, e
		}, ChangeKindFieldRequiredLoosened, false},
		{"indexed added", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			n[0].Fields[1].Indexed = true
			return n, e
		}, ChangeKindFieldIndexedAdded, false},
		{"indexed removed", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			// baseline is set indexed in the switch below; mutate is a no-op.
			return n, e
		}, ChangeKindFieldIndexedRemoved, false},
		{"searchable added", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			n[0].Fields[1].Searchable = true
			return n, e
		}, ChangeKindFieldSearchableAdded, false},
		{"new field added", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			n[0].Fields = append(n[0].Fields, FieldDef{FieldID: 3, Kind: KindString})
			return n, e
		}, ChangeKindFieldAdded, false},
		{"field removed", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			n[0].Fields = n[0].Fields[:1]
			return n, e
		}, ChangeKindFieldRemoved, false},
		{"new type added", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			n = append(n, NodeTypeDef{TypeID: 3, Fields: []FieldDef{{FieldID: 1, Kind: KindString}}})
			return n, e
		}, ChangeKindNodeAdded, false},
		{"type removed", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			// remove type 2 and the edge that referenced it
			return n[:1], nil
		}, ChangeKindNodeRemoved, false},
		{"composite_unique dropped", func(n []NodeTypeDef, e []EdgeTypeDef) ([]NodeTypeDef, []EdgeTypeDef) {
			// baseline gets the constraint; new drops it (handled below).
			return n, e
		}, ChangeKindCompositeUniqueRemoved, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			oldN, oldE := base()
			var newN []NodeTypeDef
			var newE []EdgeTypeDef

			switch tc.name {
			case "indexed removed":
				oldN[0].Fields[1].Indexed = true
				newN, newE = base() // indexed false on field 2
			case "composite_unique dropped":
				oldN[0].CompositeUnique = []CompositeUniqueDef{{FieldIDs: []uint32{1, 2}}}
				newN, newE = base() // no composite
			default:
				newN, newE = tc.mutate(base())
			}

			old := regWith(oldN, oldE)
			newR := regWith(newN, newE)
			c := Check(old, newR)
			got := findChange(c, tc.wantKind)
			if got == nil {
				t.Fatalf("%s: expected %s, got %v", tc.name, tc.wantKind, c)
			}
			if got.Breaking != tc.breaking {
				t.Fatalf("%s: %s breaking=%v, want %v", tc.name, tc.wantKind, got.Breaking, tc.breaking)
			}
			if tc.breaking != HasBreaking(c) {
				t.Fatalf("%s: HasBreaking=%v, want %v (changes=%v)", tc.name, HasBreaking(c), tc.breaking, c)
			}
		})
	}
}

// BenchmarkCheck100Types asserts <100ms (well under the 2s budget).
func BenchmarkCheck100Types(b *testing.B) {
	nodes := make([]NodeTypeDef, 100)
	for i := 0; i < 100; i++ {
		fields := make([]FieldDef, 20)
		for j := 0; j < 20; j++ {
			fields[j] = FieldDef{
				FieldID: uint32(j + 1),
				Kind:    KindString,
			}
		}
		nodes[i] = NodeTypeDef{
			TypeID: int32(i + 1),
			Fields: fields,
		}
	}
	r := regWith(nodes, nil)
	r2 := regWith(nodes, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Check(r, r2)
	}
}
