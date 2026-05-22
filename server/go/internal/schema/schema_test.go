// Tests for the schema package. Covers JSON round-trip, freeze
// semantics, fingerprint determinism, and field-id <-> name lookup.

package schema

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// pythonSampleJSON is the cross-language fixture for the SDK / server
// schema JSON contract. Two node types (User, Task) plus one edge type
// (AssignedTo). Sorted-key, compact-separator canonical form.
const pythonSampleJSON = `{
  "node_types": [
    {
      "type_id": 1,
      "name": "User",
      "fields": [
        {"field_id": 1, "name": "email", "kind": "str", "required": true, "indexed": true, "unique": true, "pii": true},
        {"field_id": 2, "name": "display_name", "kind": "str", "searchable": true},
        {"field_id": 3, "name": "status", "kind": "enum", "enum_values": ["active", "inactive", "banned"]}
      ],
      "default_acl": [{"principal": "role:admin", "permission": "admin"}],
      "data_policy": "personal",
      "subject_field": "email"
    },
    {
      "type_id": 2,
      "name": "Task",
      "fields": [
        {"field_id": 1, "name": "title", "kind": "str", "required": true, "searchable": true},
        {"field_id": 2, "name": "owner_id", "kind": "ref", "ref_type_id": 1},
        {"field_id": 3, "name": "priority", "kind": "int", "indexed": true},
        {"field_id": 4, "name": "tag", "kind": "str"}
      ],
      "composite_unique": [{"name": "owner_title", "field_ids": [2, 1]}]
    }
  ],
  "edge_types": [
    {
      "edge_id": 10,
      "name": "AssignedTo",
      "from_type_id": 2,
      "to_type_id": 1,
      "props": [
        {"field_id": 1, "name": "role", "kind": "enum", "enum_values": ["primary", "reviewer"]}
      ],
      "on_subject_exit": "both"
    }
  ]
}`

// pythonReferenceFingerprint is the expected fingerprint for pythonSampleJSON.
// Keep in sync with the cross-language fixture; if the canonical encoder
// changes the two must be regenerated together.
const pythonReferenceFingerprint = "sha256:5822499c748bb30ab35473658f8000f9cba89250d6bafdeabc6ca833fcc85fe2"

func TestLoadFromJSON_Sample(t *testing.T) {
	r, err := LoadFromJSON([]byte(pythonSampleJSON))
	if err != nil {
		t.Fatalf("LoadFromJSON: %v", err)
	}

	user := r.NodeTypeByName("User")
	if user == nil {
		t.Fatal("User node type missing")
	}
	if user.TypeID != 1 {
		t.Errorf("User.TypeID = %d, want 1", user.TypeID)
	}
	if got := r.NodeTypeByID(1); got != user {
		t.Errorf("NodeTypeByID(1) returned different pointer than ByName")
	}
	if dp := r.DataPolicyOf(1); dp != DataPolicyPersonal {
		t.Errorf("DataPolicyOf(1) = %q, want %q", dp, DataPolicyPersonal)
	}
	if sf := r.SubjectField(1); sf != "email" {
		t.Errorf("SubjectField(1) = %q, want email", sf)
	}

	if got := r.UniqueFieldIDs(1); len(got) != 1 || got[0] != 1 {
		t.Errorf("UniqueFieldIDs(1) = %v, want [1]", got)
	}
	if got := r.IndexedFieldIDs(1); len(got) != 0 {
		// "email" is indexed AND unique → excluded from indexed list.
		t.Errorf("IndexedFieldIDs(1) = %v, want []", got)
	}
	if got := r.SearchableFieldIDs(1); len(got) != 1 || got[0] != 2 {
		t.Errorf("SearchableFieldIDs(1) = %v, want [2]", got)
	}
	if got := r.PIIFields(1); len(got) != 1 || got[0] != "email" {
		t.Errorf("PIIFields(1) = %v, want [email]", got)
	}

	task := r.NodeTypeByName("Task")
	if task == nil {
		t.Fatal("Task node type missing")
	}
	cu := r.CompositeUnique(2)
	if len(cu) != 1 || cu[0].Name != "owner_title" || len(cu[0].FieldIDs) != 2 {
		t.Errorf("CompositeUnique(2) = %+v", cu)
	}

	edge := r.EdgeTypeByName("AssignedTo")
	if edge == nil {
		t.Fatal("AssignedTo edge type missing")
	}
	if edge.EdgeID != 10 || edge.FromTypeID != 2 || edge.ToTypeID != 1 {
		t.Errorf("AssignedTo: %+v", edge)
	}
	if edge.OnSubjectExit != OnSubjectExitBoth {
		t.Errorf("OnSubjectExit = %q, want both", edge.OnSubjectExit)
	}
}

func TestRoundTrip(t *testing.T) {
	r, err := LoadFromJSON([]byte(pythonSampleJSON))
	if err != nil {
		t.Fatalf("LoadFromJSON: %v", err)
	}
	out, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	r2, err := LoadFromJSON(out)
	if err != nil {
		t.Fatalf("LoadFromJSON (round-trip): %v\n%s", err, out)
	}
	if _, err := r.Freeze(); err != nil {
		t.Fatalf("Freeze r: %v", err)
	}
	if _, err := r2.Freeze(); err != nil {
		t.Fatalf("Freeze r2: %v", err)
	}
	if r.Fingerprint() != r2.Fingerprint() {
		t.Errorf(
			"fingerprint changed across round-trip:\n  before=%s\n   after=%s",
			r.Fingerprint(), r2.Fingerprint(),
		)
	}
}

func TestFingerprint_DeterministicAcrossLoads(t *testing.T) {
	r1, err := LoadFromJSON([]byte(pythonSampleJSON))
	if err != nil {
		t.Fatalf("load 1: %v", err)
	}
	r2, err := LoadFromJSON([]byte(pythonSampleJSON))
	if err != nil {
		t.Fatalf("load 2: %v", err)
	}
	fp1, err := r1.Freeze()
	if err != nil {
		t.Fatalf("freeze 1: %v", err)
	}
	fp2, err := r2.Freeze()
	if err != nil {
		t.Fatalf("freeze 2: %v", err)
	}
	if fp1 != fp2 {
		t.Errorf("fingerprint differs across loads: %s vs %s", fp1, fp2)
	}
	if !strings.HasPrefix(fp1, "sha256:") {
		t.Errorf("fingerprint missing sha256: prefix: %s", fp1)
	}
}

func TestFingerprint_PythonByteParity(t *testing.T) {
	// This test pins byte-for-byte parity with the expected reference
	// fingerprint for the shared fixture. If it breaks, the canonical
	// encoder in fingerprint.go has drifted from the sort-keys,
	// no-whitespace compact JSON format.
	r, err := LoadFromJSON([]byte(pythonSampleJSON))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	fp, err := r.Freeze()
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if fp != pythonReferenceFingerprint {
		t.Errorf(
			"fingerprint mismatch:\n  got  = %s\n  want = %s",
			fp, pythonReferenceFingerprint,
		)
	}
}

func TestFingerprint_ChangesOnFieldAdd(t *testing.T) {
	r1, err := LoadFromJSON([]byte(pythonSampleJSON))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	fp1, err := r1.Freeze()
	if err != nil {
		t.Fatalf("freeze 1: %v", err)
	}

	r2, err := LoadFromJSON([]byte(pythonSampleJSON))
	if err != nil {
		t.Fatalf("load 2: %v", err)
	}
	user := r2.NodeTypeByName("User")
	user.Fields = append(user.Fields, FieldDef{
		FieldID: 99, Name: "extra", Kind: KindString,
	})
	fp2, err := r2.Freeze()
	if err != nil {
		t.Fatalf("freeze 2: %v", err)
	}
	if fp1 == fp2 {
		t.Errorf("fingerprint unchanged after adding field: %s", fp1)
	}
}

func TestFingerprint_ChangesOnFieldRename(t *testing.T) {
	r1, err := LoadFromJSON([]byte(pythonSampleJSON))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	fp1, err := r1.Freeze()
	if err != nil {
		t.Fatalf("freeze 1: %v", err)
	}

	r2, err := LoadFromJSON([]byte(pythonSampleJSON))
	if err != nil {
		t.Fatalf("load 2: %v", err)
	}
	user := r2.NodeTypeByName("User")
	user.Fields[0].Name = "email_addr"
	// Rebuild byName map entry not needed — Freeze rebuilds the
	// translation maps from scratch and the fingerprint is computed
	// over the fields directly.
	fp2, err := r2.Freeze()
	if err != nil {
		t.Fatalf("freeze 2: %v", err)
	}
	if fp1 == fp2 {
		t.Errorf("fingerprint unchanged after renaming field: %s", fp1)
	}
}

func TestFreeze_PreventsRegistration(t *testing.T) {
	r, err := LoadFromJSON([]byte(pythonSampleJSON))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, err := r.Freeze(); err != nil {
		t.Fatalf("freeze: %v", err)
	}

	err = r.RegisterNode(&NodeTypeDef{
		TypeID: 99, Name: "Late",
		Fields: []FieldDef{{FieldID: 1, Name: "x", Kind: KindString}},
	})
	if err == nil || !errors.Is(err, ErrFrozen) {
		t.Errorf("RegisterNode after freeze err = %v, want ErrFrozen", err)
	}

	err = r.RegisterEdge(&EdgeTypeDef{
		EdgeID: 99, Name: "LateEdge", FromTypeID: 1, ToTypeID: 2,
	})
	if err == nil || !errors.Is(err, ErrFrozen) {
		t.Errorf("RegisterEdge after freeze err = %v, want ErrFrozen", err)
	}

	if _, err := r.Freeze(); err == nil || !errors.Is(err, ErrFrozen) {
		t.Errorf("second Freeze err = %v, want ErrFrozen", err)
	}
}

func TestRegister_DuplicateTypeID(t *testing.T) {
	r := NewRegistry()
	a := &NodeTypeDef{
		TypeID: 1, Name: "A",
		Fields: []FieldDef{{FieldID: 1, Name: "x", Kind: KindString}},
	}
	b := &NodeTypeDef{
		TypeID: 1, Name: "B",
		Fields: []FieldDef{{FieldID: 1, Name: "y", Kind: KindString}},
	}
	if err := r.RegisterNode(a); err != nil {
		t.Fatalf("register A: %v", err)
	}
	err := r.RegisterNode(b)
	if err == nil || !errors.Is(err, ErrDuplicateRegistration) {
		t.Errorf("duplicate type_id err = %v, want ErrDuplicateRegistration", err)
	}
}

func TestRegister_DuplicateName(t *testing.T) {
	r := NewRegistry()
	a := &NodeTypeDef{
		TypeID: 1, Name: "A",
		Fields: []FieldDef{{FieldID: 1, Name: "x", Kind: KindString}},
	}
	b := &NodeTypeDef{
		TypeID: 2, Name: "A",
		Fields: []FieldDef{{FieldID: 1, Name: "y", Kind: KindString}},
	}
	if err := r.RegisterNode(a); err != nil {
		t.Fatalf("register A: %v", err)
	}
	err := r.RegisterNode(b)
	if err == nil || !errors.Is(err, ErrDuplicateRegistration) {
		t.Errorf("duplicate name err = %v, want ErrDuplicateRegistration", err)
	}
}

func TestFieldIDByName_AndInverse(t *testing.T) {
	r, err := LoadFromJSON([]byte(pythonSampleJSON))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Pre-freeze (linear scan path).
	id, ok := r.FieldIDByName("User", "email")
	if !ok || id != 1 {
		t.Errorf("pre-freeze FieldIDByName(User,email) = %d,%v", id, ok)
	}
	name, ok := r.FieldNameByID("User", 2)
	if !ok || name != "display_name" {
		t.Errorf("pre-freeze FieldNameByID(User,2) = %q,%v", name, ok)
	}

	if _, err := r.Freeze(); err != nil {
		t.Fatalf("freeze: %v", err)
	}

	// Post-freeze (map path).
	id, ok = r.FieldIDByName("User", "email")
	if !ok || id != 1 {
		t.Errorf("post-freeze FieldIDByName(User,email) = %d,%v", id, ok)
	}
	id, ok = r.FieldIDByName("User", "status")
	if !ok || id != 3 {
		t.Errorf("post-freeze FieldIDByName(User,status) = %d,%v", id, ok)
	}
	if _, ok := r.FieldIDByName("User", "nope"); ok {
		t.Errorf("FieldIDByName(User,nope) ok = true, want false")
	}
	if _, ok := r.FieldIDByName("Nope", "email"); ok {
		t.Errorf("FieldIDByName(Nope,email) ok = true, want false")
	}

	name, ok = r.FieldNameByID("User", 1)
	if !ok || name != "email" {
		t.Errorf("post-freeze FieldNameByID(User,1) = %q,%v", name, ok)
	}
	if _, ok := r.FieldNameByID("User", 999); ok {
		t.Errorf("FieldNameByID(User,999) ok = true, want false")
	}

	// Type-id-keyed variants.
	id, ok = r.FieldIDByNameForType(1, "email")
	if !ok || id != 1 {
		t.Errorf("FieldIDByNameForType(1,email) = %d,%v", id, ok)
	}
	name, ok = r.FieldNameByIDForType(1, 3)
	if !ok || name != "status" {
		t.Errorf("FieldNameByIDForType(1,3) = %q,%v", name, ok)
	}
}

func TestNodeType_GenericLookup(t *testing.T) {
	r, err := LoadFromJSON([]byte(pythonSampleJSON))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cases := []struct {
		key  any
		want string
	}{
		{int32(1), "User"},
		{int(1), "User"},
		{int64(1), "User"},
		{uint32(1), "User"},
		{"User", "User"},
		{int32(99), ""},
		{"missing", ""},
	}
	for _, tc := range cases {
		got := r.NodeType(tc.key)
		var name string
		if got != nil {
			name = got.Name
		}
		if name != tc.want {
			t.Errorf("NodeType(%v) = %q, want %q", tc.key, name, tc.want)
		}
	}
}

func TestValidate_DuplicateFieldID(t *testing.T) {
	bad := &NodeTypeDef{
		TypeID: 1, Name: "Bad",
		Fields: []FieldDef{
			{FieldID: 1, Name: "a", Kind: KindString},
			{FieldID: 1, Name: "b", Kind: KindString},
		},
	}
	if err := bad.Validate(); err == nil {
		t.Errorf("duplicate field_id should fail")
	}
}

func TestValidate_CompositeUniqueRules(t *testing.T) {
	cases := []struct {
		name string
		nt   *NodeTypeDef
	}{
		{
			name: "single-field-composite",
			nt: &NodeTypeDef{
				TypeID: 1, Name: "X",
				Fields: []FieldDef{
					{FieldID: 1, Name: "a", Kind: KindString},
					{FieldID: 2, Name: "b", Kind: KindString},
				},
				CompositeUnique: []CompositeUniqueDef{{Name: "c", FieldIDs: []uint32{1}}},
			},
		},
		{
			name: "unknown-field-id",
			nt: &NodeTypeDef{
				TypeID: 1, Name: "X",
				Fields: []FieldDef{
					{FieldID: 1, Name: "a", Kind: KindString},
				},
				CompositeUnique: []CompositeUniqueDef{{Name: "c", FieldIDs: []uint32{1, 99}}},
			},
		},
		{
			name: "duplicate-name",
			nt: &NodeTypeDef{
				TypeID: 1, Name: "X",
				Fields: []FieldDef{
					{FieldID: 1, Name: "a", Kind: KindString},
					{FieldID: 2, Name: "b", Kind: KindString},
					{FieldID: 3, Name: "c", Kind: KindString},
				},
				CompositeUnique: []CompositeUniqueDef{
					{Name: "dup", FieldIDs: []uint32{1, 2}},
					{Name: "dup", FieldIDs: []uint32{2, 3}},
				},
			},
		},
		{
			name: "duplicate-signature",
			nt: &NodeTypeDef{
				TypeID: 1, Name: "X",
				Fields: []FieldDef{
					{FieldID: 1, Name: "a", Kind: KindString},
					{FieldID: 2, Name: "b", Kind: KindString},
				},
				CompositeUnique: []CompositeUniqueDef{
					{Name: "ab", FieldIDs: []uint32{1, 2}},
					{Name: "ba", FieldIDs: []uint32{2, 1}},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.nt.Validate(); err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

func TestValidateAll_CrossReferences(t *testing.T) {
	r := NewRegistry()
	if err := r.RegisterNode(&NodeTypeDef{
		TypeID: 1, Name: "User",
		Fields: []FieldDef{{FieldID: 1, Name: "x", Kind: KindString}},
	}); err != nil {
		t.Fatalf("register user: %v", err)
	}
	// Edge points at unregistered type 2.
	if err := r.RegisterEdge(&EdgeTypeDef{
		EdgeID: 1, Name: "E", FromTypeID: 1, ToTypeID: 2,
		OnSubjectExit: OnSubjectExitBoth,
	}); err != nil {
		t.Fatalf("register edge: %v", err)
	}
	errsList := r.ValidateAll()
	if len(errsList) == 0 {
		t.Errorf("ValidateAll should report dangling to_type_id")
	}
}

func TestEdge_OnSubjectExitDefault(t *testing.T) {
	// Loader fills in OnSubjectExit when JSON omits it.
	body := `{"node_types": [
		{"type_id": 1, "name": "A", "fields": [{"field_id": 1, "name": "x", "kind": "str"}]},
		{"type_id": 2, "name": "B", "fields": [{"field_id": 1, "name": "y", "kind": "str"}]}
	],
	"edge_types": [
		{"edge_id": 1, "name": "E", "from_type_id": 1, "to_type_id": 2, "props": []}
	]}`
	r, err := LoadFromJSON([]byte(body))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	e := r.EdgeTypeByName("E")
	if e == nil {
		t.Fatal("edge missing")
	}
	if e.OnSubjectExit != OnSubjectExitBoth {
		t.Errorf("OnSubjectExit = %q, want both", e.OnSubjectExit)
	}
}
