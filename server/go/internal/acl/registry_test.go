// SPDX-License-Identifier: AGPL-3.0-only

package acl_test

import (
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/acl"
)

// TestRequiredForOpDefaults pins the DEFAULT_OP_REQUIREMENTS table.
func TestRequiredForOpDefaults(t *testing.T) {
	r := acl.NewRegistry()
	cases := []struct {
		op   string
		want acl.CoreCapability
	}{
		{"GetNode", acl.CoreCapRead},
		{"GetNodes", acl.CoreCapRead},
		{"QueryNodes", acl.CoreCapRead},
		{"GetEdgesFrom", acl.CoreCapRead},
		{"GetConnectedNodes", acl.CoreCapRead},
		{"UpdateNode", acl.CoreCapEdit},
		{"DeleteNode", acl.CoreCapDelete},
		{"CreateEdge", acl.CoreCapEdit},
		{"DeleteEdge", acl.CoreCapEdit},
		{"ShareNode", acl.CoreCapAdmin},
		{"RevokeAccess", acl.CoreCapAdmin},
		{"TransferOwnership", acl.CoreCapAdmin},
	}
	for _, c := range cases {
		core, ext := r.RequiredForOp(0, c.op, acl.OpQuery{})
		if core != c.want || ext != 0 {
			t.Fatalf("RequiredForOp(%q) = (%v, %v), want (%v, 0)", c.op, core, ext, c.want)
		}
	}
}

func TestRequiredForOpUnknownReturnsZero(t *testing.T) {
	r := acl.NewRegistry()
	core, ext := r.RequiredForOp(0, "UnknownOp", acl.OpQuery{})
	if core != acl.CoreCapUnspecified || ext != 0 {
		t.Fatalf("unknown op = (%v, %v), want (Unspecified, 0)", core, ext)
	}
}

// TestRequiredForOpFieldOverride covers field-level gating per
// docs/decisions/acl.md "Field-level permission gating" example.
func TestRequiredForOpFieldOverride(t *testing.T) {
	r := acl.NewRegistry()
	r.RegisterType(&acl.TypeCapabilities{
		TypeID: 7,
		Mappings: []acl.CapabilityMapping{
			// Override: UpdateNode on field "status" requires ext-cap 11.
			{Op: "UpdateNode", Field: "status", RequiredExt: 11},
			// Even more specific: status="merged" requires ext 22.
			{Op: "UpdateNode", Field: "status", FieldValue: "merged", RequiredExt: 22},
		},
	})
	// No-field fall-through to default EDIT.
	core, ext := r.RequiredForOp(7, "UpdateNode", acl.OpQuery{})
	if core != acl.CoreCapEdit || ext != 0 {
		t.Fatalf("UpdateNode no field = (%v, %v), want (Edit, 0)", core, ext)
	}
	// Field=status -> ext 11.
	core, ext = r.RequiredForOp(7, "UpdateNode", acl.OpQuery{Field: "status"})
	if ext != 11 {
		t.Fatalf("UpdateNode field=status = (%v, %v), want (_, 11)", core, ext)
	}
	// Field=status, value=merged -> ext 22 (more specific wins).
	core, ext = r.RequiredForOp(7, "UpdateNode", acl.OpQuery{Field: "status", FieldValue: "merged"})
	if ext != 22 {
		t.Fatalf("UpdateNode status=merged = (%v, %v), want (_, 22)", core, ext)
	}
}

func TestCheckGrantUnspecifiedRequirementAllows(t *testing.T) {
	r := acl.NewRegistry()
	if !r.CheckGrant(nil, nil, acl.CoreCapUnspecified, 0, 0) {
		t.Fatalf("CheckGrant with no requirement should allow")
	}
}

func TestCheckGrantCoreImplications(t *testing.T) {
	r := acl.NewRegistry()
	// Holding ADMIN should satisfy READ, COMMENT, EDIT, DELETE.
	for _, need := range []acl.CoreCapability{acl.CoreCapRead, acl.CoreCapComment, acl.CoreCapEdit, acl.CoreCapDelete, acl.CoreCapAdmin} {
		if !r.CheckGrant([]acl.CoreCapability{acl.CoreCapAdmin}, nil, need, 0, 0) {
			t.Fatalf("ADMIN should imply %v", need)
		}
	}
	// Holding READ should satisfy READ only.
	if !r.CheckGrant([]acl.CoreCapability{acl.CoreCapRead}, nil, acl.CoreCapRead, 0, 0) {
		t.Fatalf("READ implies READ")
	}
	if r.CheckGrant([]acl.CoreCapability{acl.CoreCapRead}, nil, acl.CoreCapComment, 0, 0) {
		t.Fatalf("READ should not imply COMMENT")
	}
	if r.CheckGrant([]acl.CoreCapability{acl.CoreCapRead}, nil, acl.CoreCapEdit, 0, 0) {
		t.Fatalf("READ should not imply EDIT")
	}
	// COMMENT implies READ.
	if !r.CheckGrant([]acl.CoreCapability{acl.CoreCapComment}, nil, acl.CoreCapRead, 0, 0) {
		t.Fatalf("COMMENT implies READ")
	}
	// EDIT implies READ + COMMENT.
	if !r.CheckGrant([]acl.CoreCapability{acl.CoreCapEdit}, nil, acl.CoreCapComment, 0, 0) {
		t.Fatalf("EDIT implies COMMENT")
	}
}

func TestCheckGrantExtImpliesCore(t *testing.T) {
	r := acl.NewRegistry()
	r.RegisterType(&acl.TypeCapabilities{
		TypeID: 301,
		Implications: []acl.CapabilityImplication{
			// PR_EXT_CAP_MERGE (1) implies CORE_CAP_EDIT.
			{Ext: 1, ImpliesCore: []acl.CoreCapability{acl.CoreCapEdit}},
		},
	})
	// Holding ext=1 should satisfy CORE_CAP_EDIT (and READ, COMMENT via core closure).
	if !r.CheckGrant(nil, []acl.ExtCapID{1}, acl.CoreCapEdit, 0, 301) {
		t.Fatalf("ext=1 should imply EDIT")
	}
	if !r.CheckGrant(nil, []acl.ExtCapID{1}, acl.CoreCapRead, 0, 301) {
		t.Fatalf("ext=1 -> EDIT -> READ")
	}
	// Cross-type ext does NOT satisfy.
	if r.CheckGrant(nil, []acl.ExtCapID{1}, acl.CoreCapEdit, 0, 999) {
		t.Fatalf("ext=1 in type=999 should not satisfy EDIT (type-scoped)")
	}
}

func TestCheckGrantExtRequiredMatch(t *testing.T) {
	r := acl.NewRegistry()
	r.RegisterType(&acl.TypeCapabilities{TypeID: 1})
	if !r.CheckGrant(nil, []acl.ExtCapID{42}, acl.CoreCapUnspecified, 42, 1) {
		t.Fatalf("ext=42 should match required ext=42")
	}
	if r.CheckGrant(nil, []acl.ExtCapID{1}, acl.CoreCapUnspecified, 42, 1) {
		t.Fatalf("ext=1 should NOT match required ext=42")
	}
}

func TestRegistryLoadJSON(t *testing.T) {
	const blob = `{
        "types": [
            {
                "type_id": 7,
                "extension_enum_name": "TaskExt",
                "ext_cap_names": {"1": "ASSIGN", "2": "CHANGE_STATUS"},
                "mappings": [
                    {"op": "UpdateNode", "field": "assignee_id", "required_ext": 1}
                ],
                "implications": [
                    {"ext": 2, "implies_core": [3]}
                ]
            }
        ]
    }`
	r := acl.NewRegistry()
	if err := r.LoadJSON([]byte(blob)); err != nil {
		t.Fatalf("LoadJSON: %v", err)
	}
	core, ext := r.RequiredForOp(7, "UpdateNode", acl.OpQuery{Field: "assignee_id"})
	if ext != 1 || core != 0 {
		t.Fatalf("RequiredForOp(7,UpdateNode,assignee_id) = (%v,%v), want (_,1)", core, ext)
	}
	// ext=2 (CHANGE_STATUS) implies core EDIT per the JSON.
	if !r.CheckGrant(nil, []acl.ExtCapID{2}, acl.CoreCapEdit, 0, 7) {
		t.Fatalf("CHANGE_STATUS should imply EDIT")
	}
}

func TestKnownTypeIDs(t *testing.T) {
	r := acl.NewRegistry()
	r.RegisterType(&acl.TypeCapabilities{TypeID: 5})
	r.RegisterType(&acl.TypeCapabilities{TypeID: 1})
	r.RegisterType(&acl.TypeCapabilities{TypeID: 3})
	got := r.KnownTypeIDs()
	if len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 5 {
		t.Fatalf("KnownTypeIDs = %v, want [1 3 5]", got)
	}
}
