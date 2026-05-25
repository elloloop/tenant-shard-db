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
	if !got.Breaking {
		t.Fatalf("NODE_REMOVED must be breaking")
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
	if !got.Breaking {
		t.Fatalf("FIELD_REMOVED must be breaking")
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
	if !got.Breaking {
		t.Fatalf("EDGE_REMOVED must be breaking")
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
	user2.Fields = user2.Fields[:1] // remove field 2 → breaking
	r2 := regWith([]NodeTypeDef{user2}, nil)
	c := Check(r1, r2)
	out := RenderText(c, "baseline.json")
	if !strings.Contains(out, "BREAKING") {
		t.Fatalf("expected BREAKING header, got: %s", out)
	}
	if !strings.Contains(out, "FIELD_REMOVED") {
		t.Fatalf("expected rule name, got: %s", out)
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
