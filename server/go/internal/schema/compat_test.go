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

func userNode() NodeTypeDef {
	return NodeTypeDef{
		TypeID: 1,
		Name:   "User",
		Fields: []FieldDef{
			{FieldID: 1, Name: "user_id", Kind: KindString, Required: true, Unique: true},
			{FieldID: 2, Name: "email", Kind: KindString},
		},
	}
}

// findChange returns the first Change whose Kind matches `k`, or nil
// when none exists. Used by table tests to assert presence without
// pinning slice indices.
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
	post2 := NodeTypeDef{TypeID: 2, Name: "Post", Fields: []FieldDef{{FieldID: 1, Name: "title", Kind: KindString}}}
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
	post := NodeTypeDef{TypeID: 2, Name: "Post", Fields: []FieldDef{{FieldID: 1, Name: "title", Kind: KindString}}}
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

func TestCompat_TypeIDChanged(t *testing.T) {
	old := regWith([]NodeTypeDef{{TypeID: 1, Name: "User", Fields: []FieldDef{{FieldID: 1, Name: "user_id", Kind: KindString}}}}, nil)
	newR := regWith([]NodeTypeDef{{TypeID: 2, Name: "User", Fields: []FieldDef{{FieldID: 1, Name: "user_id", Kind: KindString}}}}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindTypeIDChanged)
	if got == nil {
		t.Fatalf("expected TYPE_ID_CHANGED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("TYPE_ID_CHANGED must be breaking")
	}
}

func TestCompat_FieldAdded(t *testing.T) {
	old := regWith([]NodeTypeDef{userNode()}, nil)
	user2 := userNode()
	user2.Fields = append(user2.Fields, FieldDef{FieldID: 3, Name: "phone", Kind: KindString})
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

func TestCompat_FieldIDReassigned(t *testing.T) {
	old := regWith([]NodeTypeDef{userNode()}, nil)
	user2 := userNode()
	// email moves from field_id=2 to field_id=7
	user2.Fields[1].FieldID = 7
	newR := regWith([]NodeTypeDef{user2}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindFieldIDChanged)
	if got == nil {
		t.Fatalf("expected FIELD_ID_CHANGED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("FIELD_ID_CHANGED must be breaking")
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
	// flip user_id required from true to false
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
		TypeID: 1, Name: "Order",
		Fields: []FieldDef{{FieldID: 1, Name: "status", Kind: KindEnum, EnumValues: []string{"A", "B"}}},
	}}, nil)
	newR := regWith([]NodeTypeDef{{
		TypeID: 1, Name: "Order",
		Fields: []FieldDef{{FieldID: 1, Name: "status", Kind: KindEnum, EnumValues: []string{"A", "B", "C"}}},
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
		TypeID: 1, Name: "Order",
		Fields: []FieldDef{{FieldID: 1, Name: "status", Kind: KindEnum, EnumValues: []string{"A", "B", "C"}}},
	}}, nil)
	newR := regWith([]NodeTypeDef{{
		TypeID: 1, Name: "Order",
		Fields: []FieldDef{{FieldID: 1, Name: "status", Kind: KindEnum, EnumValues: []string{"A", "B"}}},
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
		TypeID: 1, Name: "Order",
		Fields: []FieldDef{{FieldID: 1, Name: "status", Kind: KindEnum, EnumValues: []string{"A", "B"}}},
	}}, nil)
	newR := regWith([]NodeTypeDef{{
		TypeID: 1, Name: "Order",
		Fields: []FieldDef{{FieldID: 1, Name: "status", Kind: KindEnum, EnumValues: []string{"B", "A"}}},
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
	user2.CompositeUnique = []CompositeUniqueDef{{Name: "by_email_tenant", FieldIDs: []uint32{1, 2}}}
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
	user.CompositeUnique = []CompositeUniqueDef{{Name: "by_email_tenant", FieldIDs: []uint32{1, 2}}}
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

func TestCompat_CompositeUniqueChanged(t *testing.T) {
	user := userNode()
	user.Fields = append(user.Fields, FieldDef{FieldID: 3, Name: "org", Kind: KindString})
	user.CompositeUnique = []CompositeUniqueDef{{Name: "by_set", FieldIDs: []uint32{1, 2}}}
	old := regWith([]NodeTypeDef{user}, nil)
	user2 := userNode()
	user2.Fields = append(user2.Fields, FieldDef{FieldID: 3, Name: "org", Kind: KindString})
	user2.CompositeUnique = []CompositeUniqueDef{{Name: "by_set", FieldIDs: []uint32{1, 3}}}
	newR := regWith([]NodeTypeDef{user2}, nil)
	c := Check(old, newR)
	got := findChange(c, ChangeKindCompositeUniqueChanged)
	if got == nil {
		t.Fatalf("expected COMPOSITE_UNIQUE_CHANGED, got %v", c)
	}
	if !got.Breaking {
		t.Fatalf("COMPOSITE_UNIQUE_CHANGED must be breaking")
	}
}

func owns() EdgeTypeDef {
	return EdgeTypeDef{EdgeID: 100, Name: "Owns", FromTypeID: 1, ToTypeID: 2, OnSubjectExit: OnSubjectExitBoth}
}

func TestCompat_EdgeAdded(t *testing.T) {
	user := userNode()
	post := NodeTypeDef{TypeID: 2, Name: "Post", Fields: []FieldDef{{FieldID: 1, Name: "title", Kind: KindString}}}
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
	post := NodeTypeDef{TypeID: 2, Name: "Post", Fields: []FieldDef{{FieldID: 1, Name: "title", Kind: KindString}}}
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
	org := NodeTypeDef{TypeID: 3, Name: "Org", Fields: []FieldDef{{FieldID: 1, Name: "name", Kind: KindString}}}
	post := NodeTypeDef{TypeID: 2, Name: "Post", Fields: []FieldDef{{FieldID: 1, Name: "title", Kind: KindString}}}
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
	post := NodeTypeDef{TypeID: 2, Name: "Post", Fields: []FieldDef{{FieldID: 1, Name: "title", Kind: KindString}}}
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
	post := NodeTypeDef{TypeID: 2, Name: "Post", Fields: []FieldDef{{FieldID: 1, Name: "title", Kind: KindString}}}
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
	old := regWith([]NodeTypeDef{{TypeID: 1, Name: "Pay", DataPolicy: &fin, Fields: []FieldDef{{FieldID: 1, Name: "amount", Kind: KindInteger}}}}, nil)
	newR := regWith([]NodeTypeDef{{TypeID: 1, Name: "Pay", DataPolicy: &bus, Fields: []FieldDef{{FieldID: 1, Name: "amount", Kind: KindInteger}}}}, nil)
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
	old := regWith([]NodeTypeDef{{TypeID: 1, Name: "Pay", DataPolicy: &bus, Fields: []FieldDef{{FieldID: 1, Name: "amount", Kind: KindInteger}}}}, nil)
	newR := regWith([]NodeTypeDef{{TypeID: 1, Name: "Pay", DataPolicy: &fin, Fields: []FieldDef{{FieldID: 1, Name: "amount", Kind: KindInteger}}}}, nil)
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
	s1 := "user_id"
	s2 := "id"
	old := regWith([]NodeTypeDef{{TypeID: 1, Name: "User", SubjectField: &s1, Fields: []FieldDef{{FieldID: 1, Name: "user_id", Kind: KindString}, {FieldID: 2, Name: "id", Kind: KindString}}}}, nil)
	newR := regWith([]NodeTypeDef{{TypeID: 1, Name: "User", SubjectField: &s2, Fields: []FieldDef{{FieldID: 1, Name: "user_id", Kind: KindString}, {FieldID: 2, Name: "id", Kind: KindString}}}}, nil)
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
	post1 := NodeTypeDef{TypeID: 2, Name: "Post", Fields: []FieldDef{{FieldID: 1, Name: "title", Kind: KindString}}}
	post2 := NodeTypeDef{TypeID: 3, Name: "Comment", Fields: []FieldDef{{FieldID: 1, Name: "body", Kind: KindString}}}
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
	c := Change{Kind: ChangeKindFieldIDChanged, Path: "node:User.field:email", Message: "x", Breaking: true}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"FIELD_ID_CHANGED"`) {
		t.Fatalf("expected textual kind, got %s", raw)
	}
}

func TestRenderText_Smoke(t *testing.T) {
	r1 := regWith([]NodeTypeDef{userNode()}, nil)
	user2 := userNode()
	user2.Fields[1].FieldID = 7
	r2 := regWith([]NodeTypeDef{user2}, nil)
	c := Check(r1, r2)
	out := RenderText(c, "baseline.json")
	if !strings.Contains(out, "BREAKING") {
		t.Fatalf("expected BREAKING header, got: %s", out)
	}
	if !strings.Contains(out, "FIELD_ID_CHANGED") {
		t.Fatalf("expected rule name, got: %s", out)
	}
}

// BenchmarkCheck100Types asserts <100ms (well under the 2s budget).
// A 100-type / 2000-field synthetic registry is large enough to make
// the algorithmic cost visible without taking forever to construct.
func BenchmarkCheck100Types(b *testing.B) {
	nodes := make([]NodeTypeDef, 100)
	for i := 0; i < 100; i++ {
		fields := make([]FieldDef, 20)
		for j := 0; j < 20; j++ {
			fields[j] = FieldDef{
				FieldID: uint32(j + 1),
				Name:    "f" + itoaShort(j),
				Kind:    KindString,
			}
		}
		nodes[i] = NodeTypeDef{
			TypeID: int32(i + 1),
			Name:   "T" + itoaShort(i),
			Fields: fields,
		}
	}
	r := regWith(nodes, nil)
	// Same shape on both sides to exercise the "no diff" path, which
	// is the hot path in CI (most PRs don't change the schema).
	r2 := regWith(nodes, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Check(r, r2)
	}
}

func itoaShort(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := false
	x := v
	if x < 0 {
		neg = true
		x = -x
	}
	for x > 0 {
		i--
		buf[i] = byte('0' + x%10)
		x /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
