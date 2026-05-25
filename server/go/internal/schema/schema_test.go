// Tests for the schema package. Covers JSON round-trip, freeze
// semantics, fingerprint determinism, and id-keyed lookup. NAME-FREE per
// ADR-031: the registry, JSON contract, and fingerprint carry no names.

package schema

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// pythonSampleJSON is the cross-language fixture for the SDK / server
// schema JSON contract. Two node types (1, 2) plus one edge type (10).
// Sorted-key, compact-separator canonical form. Name-free (ADR-031):
// types/fields are identified by id only.
const pythonSampleJSON = `{
  "node_types": [
    {
      "type_id": 1,
      "fields": [
        {"field_id": 1, "kind": "str", "required": true, "indexed": true, "unique": true, "pii": true},
        {"field_id": 2, "kind": "str", "searchable": true},
        {"field_id": 3, "kind": "enum", "enum_values": ["active", "inactive", "banned"]}
      ],
      "default_acl": [{"principal": "role:admin", "permission": "admin"}],
      "data_policy": "personal",
      "subject_field": 1
    },
    {
      "type_id": 2,
      "fields": [
        {"field_id": 1, "kind": "str", "required": true, "searchable": true},
        {"field_id": 2, "kind": "ref", "ref_type_id": 1},
        {"field_id": 3, "kind": "int", "indexed": true},
        {"field_id": 4, "kind": "str"}
      ],
      "composite_unique": [{"field_ids": [2, 1]}]
    }
  ],
  "edge_types": [
    {
      "edge_id": 10,
      "from_type_id": 2,
      "to_type_id": 1,
      "props": [
        {"field_id": 1, "kind": "enum", "enum_values": ["primary", "reviewer"]}
      ],
      "on_subject_exit": "both"
    }
  ]
}`

// pythonReferenceFingerprint is the expected fingerprint for
// pythonSampleJSON in its name-free form (ADR-031). Keep in sync with the
// cross-language fixture; if the canonical encoder changes the two must
// be regenerated together.
const pythonReferenceFingerprint = "sha256:6179549ada4a93571edd51dfea150747c5c25d76ece723b8347ec7d9329b7c7a"

func TestLoadFromJSON_Sample(t *testing.T) {
	r, err := LoadFromJSON([]byte(pythonSampleJSON))
	if err != nil {
		t.Fatalf("LoadFromJSON: %v", err)
	}

	user := r.NodeTypeByID(1)
	if user == nil {
		t.Fatal("node type_id=1 missing")
	}
	if user.TypeID != 1 {
		t.Errorf("user.TypeID = %d, want 1", user.TypeID)
	}
	if dp := r.DataPolicyOf(1); dp != DataPolicyPersonal {
		t.Errorf("DataPolicyOf(1) = %q, want %q", dp, DataPolicyPersonal)
	}
	if sf, ok := r.SubjectFieldID(1); !ok || sf != 1 {
		t.Errorf("SubjectFieldID(1) = %d,%v, want 1,true", sf, ok)
	}

	if got := r.UniqueFieldIDs(1); len(got) != 1 || got[0] != 1 {
		t.Errorf("UniqueFieldIDs(1) = %v, want [1]", got)
	}
	if got := r.IndexedFieldIDs(1); len(got) != 0 {
		// field_id=1 is indexed AND unique → excluded from indexed list.
		t.Errorf("IndexedFieldIDs(1) = %v, want []", got)
	}
	if got := r.SearchableFieldIDs(1); len(got) != 1 || got[0] != 2 {
		t.Errorf("SearchableFieldIDs(1) = %v, want [2]", got)
	}
	if got := r.PIIFieldIDs(1); len(got) != 1 || got[0] != 1 {
		t.Errorf("PIIFieldIDs(1) = %v, want [1]", got)
	}

	task := r.NodeTypeByID(2)
	if task == nil {
		t.Fatal("node type_id=2 missing")
	}
	cu := r.CompositeUnique(2)
	if len(cu) != 1 || len(cu[0].FieldIDs) != 2 {
		t.Errorf("CompositeUnique(2) = %+v", cu)
	}

	edge := r.EdgeTypeByID(10)
	if edge == nil {
		t.Fatal("edge_id=10 missing")
	}
	if edge.EdgeID != 10 || edge.FromTypeID != 2 || edge.ToTypeID != 1 {
		t.Errorf("edge: %+v", edge)
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
	// The serialised form must be name-free.
	if strings.Contains(string(out), `"name"`) {
		t.Fatalf("registry JSON contains a name key (ADR-031 violation):\n%s", out)
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

func TestFingerprint_ReferenceParity(t *testing.T) {
	// Pins byte-for-byte parity with the expected reference fingerprint
	// for the name-free shared fixture. If it breaks, the canonical
	// encoder in fingerprint.go has drifted from the sort-keys,
	// no-whitespace compact JSON format (or the fixture changed).
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
	user := r2.NodeTypeByID(1)
	user.Fields = append(user.Fields, FieldDef{
		FieldID: 99, Kind: KindString,
	})
	fp2, err := r2.Freeze()
	if err != nil {
		t.Fatalf("freeze 2: %v", err)
	}
	if fp1 == fp2 {
		t.Errorf("fingerprint unchanged after adding field: %s", fp1)
	}
}

func TestFingerprint_ChangesOnFieldKindChange(t *testing.T) {
	// Name-free: there is no rename to fingerprint, but a kind change
	// (a real, attribute-level change) must move the fingerprint.
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
	user := r2.NodeTypeByID(1)
	user.Fields[1].Kind = KindInteger
	fp2, err := r2.Freeze()
	if err != nil {
		t.Fatalf("freeze 2: %v", err)
	}
	if fp1 == fp2 {
		t.Errorf("fingerprint unchanged after changing field kind: %s", fp1)
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
		TypeID: 99,
		Fields: []FieldDef{{FieldID: 1, Kind: KindString}},
	})
	if err == nil || !errors.Is(err, ErrFrozen) {
		t.Errorf("RegisterNode after freeze err = %v, want ErrFrozen", err)
	}

	err = r.RegisterEdge(&EdgeTypeDef{
		EdgeID: 99, FromTypeID: 1, ToTypeID: 2,
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
		TypeID: 1,
		Fields: []FieldDef{{FieldID: 1, Kind: KindString}},
	}
	b := &NodeTypeDef{
		TypeID: 1,
		Fields: []FieldDef{{FieldID: 1, Kind: KindString}},
	}
	if err := r.RegisterNode(a); err != nil {
		t.Fatalf("register A: %v", err)
	}
	err := r.RegisterNode(b)
	if err == nil || !errors.Is(err, ErrDuplicateRegistration) {
		t.Errorf("duplicate type_id err = %v, want ErrDuplicateRegistration", err)
	}
}

func TestNodeType_GenericLookup(t *testing.T) {
	r, err := LoadFromJSON([]byte(pythonSampleJSON))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cases := []struct {
		key  any
		want int32
	}{
		{int32(1), 1},
		{int(1), 1},
		{int64(1), 1},
		{uint32(1), 1},
		{int32(99), 0},
		// A string key is meaningless in the name-free registry — returns nil.
		{"User", 0},
	}
	for _, tc := range cases {
		got := r.NodeType(tc.key)
		var id int32
		if got != nil {
			id = got.TypeID
		}
		if id != tc.want {
			t.Errorf("NodeType(%v) = type_id %d, want %d", tc.key, id, tc.want)
		}
	}
}

func TestValidate_DuplicateFieldID(t *testing.T) {
	bad := &NodeTypeDef{
		TypeID: 1,
		Fields: []FieldDef{
			{FieldID: 1, Kind: KindString},
			{FieldID: 1, Kind: KindString},
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
				TypeID: 1,
				Fields: []FieldDef{
					{FieldID: 1, Kind: KindString},
					{FieldID: 2, Kind: KindString},
				},
				CompositeUnique: []CompositeUniqueDef{{FieldIDs: []uint32{1}}},
			},
		},
		{
			name: "unknown-field-id",
			nt: &NodeTypeDef{
				TypeID: 1,
				Fields: []FieldDef{
					{FieldID: 1, Kind: KindString},
				},
				CompositeUnique: []CompositeUniqueDef{{FieldIDs: []uint32{1, 99}}},
			},
		},
		{
			name: "duplicate-signature",
			nt: &NodeTypeDef{
				TypeID: 1,
				Fields: []FieldDef{
					{FieldID: 1, Kind: KindString},
					{FieldID: 2, Kind: KindString},
				},
				CompositeUnique: []CompositeUniqueDef{
					{FieldIDs: []uint32{1, 2}},
					{FieldIDs: []uint32{2, 1}},
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
		TypeID: 1,
		Fields: []FieldDef{{FieldID: 1, Kind: KindString}},
	}); err != nil {
		t.Fatalf("register user: %v", err)
	}
	// Edge points at unregistered type 2.
	if err := r.RegisterEdge(&EdgeTypeDef{
		EdgeID: 1, FromTypeID: 1, ToTypeID: 2,
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
		{"type_id": 1, "fields": [{"field_id": 1, "kind": "str"}]},
		{"type_id": 2, "fields": [{"field_id": 1, "kind": "str"}]}
	],
	"edge_types": [
		{"edge_id": 1, "from_type_id": 1, "to_type_id": 2, "props": []}
	]}`
	r, err := LoadFromJSON([]byte(body))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	e := r.EdgeTypeByID(1)
	if e == nil {
		t.Fatal("edge missing")
	}
	if e.OnSubjectExit != OnSubjectExitBoth {
		t.Errorf("OnSubjectExit = %q, want both", e.OnSubjectExit)
	}
}
