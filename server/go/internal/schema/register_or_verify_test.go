package schema

import (
	"errors"
	"testing"
)

func userNodeDef(emailUnique bool) *NodeTypeDef {
	return &NodeTypeDef{
		TypeID: 1,
		Name:   "User",
		Fields: []FieldDef{
			{FieldID: 1, Name: "email", Kind: KindString, Unique: emailUnique},
			{FieldID: 2, Name: "name", Kind: KindString},
		},
	}
}

// TestRegisterOrVerifyNode_Establish registers an absent type, recomputes
// the fingerprint, and exposes the type to the read accessors.
func TestRegisterOrVerifyNode_Establish(t *testing.T) {
	r := NewRegistry()
	if r.Fingerprint() != "" {
		t.Fatalf("fresh registry fingerprint = %q, want empty", r.Fingerprint())
	}
	registered, err := r.RegisterOrVerifyNode(userNodeDef(true))
	if err != nil {
		t.Fatalf("RegisterOrVerifyNode: %v", err)
	}
	if !registered {
		t.Fatalf("registered = false, want true (type was absent)")
	}
	if r.NodeTypeByID(1) == nil {
		t.Fatalf("type 1 not visible after establish")
	}
	if r.NodeTypeByName("User") == nil {
		t.Fatalf("type User not visible by name after establish")
	}
	if got := r.UniqueFieldIDs(1); len(got) != 1 || got[0] != 1 {
		t.Fatalf("UniqueFieldIDs(1) = %v, want [1]", got)
	}
	if r.Fingerprint() == "" {
		t.Fatalf("fingerprint not recomputed after establish")
	}
	// Field translation maps must be populated (lock-free read path).
	if id, ok := r.FieldIDByNameForType(1, "email"); !ok || id != 1 {
		t.Fatalf("FieldIDByNameForType(1, email) = (%d, %v), want (1, true)", id, ok)
	}
}

// TestRegisterOrVerifyNode_IdenticalNoOp re-registers a byte-identical
// type: no-op, fingerprint unchanged, registered=false.
func TestRegisterOrVerifyNode_IdenticalNoOp(t *testing.T) {
	r := NewRegistry()
	if _, err := r.RegisterOrVerifyNode(userNodeDef(true)); err != nil {
		t.Fatalf("first RegisterOrVerifyNode: %v", err)
	}
	fp1 := r.Fingerprint()

	registered, err := r.RegisterOrVerifyNode(userNodeDef(true))
	if err != nil {
		t.Fatalf("second RegisterOrVerifyNode: %v", err)
	}
	if registered {
		t.Fatalf("registered = true on an identical re-register, want false")
	}
	if r.Fingerprint() != fp1 {
		t.Fatalf("fingerprint changed on identical re-register: %q -> %q", fp1, r.Fingerprint())
	}
}

// TestRegisterOrVerifyNode_Conflict rejects a divergent redefinition of
// the same type_id and leaves the registered type unchanged.
func TestRegisterOrVerifyNode_Conflict(t *testing.T) {
	r := NewRegistry()
	if _, err := r.RegisterOrVerifyNode(userNodeDef(true)); err != nil {
		t.Fatalf("first RegisterOrVerifyNode: %v", err)
	}
	fp1 := r.Fingerprint()

	_, err := r.RegisterOrVerifyNode(userNodeDef(false)) // email no longer unique
	if err == nil {
		t.Fatalf("conflicting RegisterOrVerifyNode: expected error, got nil")
	}
	if !errors.Is(err, ErrSchemaConflict) {
		t.Fatalf("error = %v, want ErrSchemaConflict", err)
	}
	// Unchanged: email still unique, fingerprint stable.
	if got := r.UniqueFieldIDs(1); len(got) != 1 || got[0] != 1 {
		t.Fatalf("after conflict UniqueFieldIDs(1) = %v, want [1]", got)
	}
	if r.Fingerprint() != fp1 {
		t.Fatalf("fingerprint changed after a rejected conflict")
	}
}

// TestRegisterOrVerifyNode_NameTypeIDMismatch rejects the same name under
// a different type_id (and vice versa) as a conflict.
func TestRegisterOrVerifyNode_NameTypeIDMismatch(t *testing.T) {
	r := NewRegistry()
	if _, err := r.RegisterOrVerifyNode(userNodeDef(true)); err != nil {
		t.Fatalf("first RegisterOrVerifyNode: %v", err)
	}
	// Same name "User", different type_id.
	rename := userNodeDef(true)
	rename.TypeID = 9
	if _, err := r.RegisterOrVerifyNode(rename); !errors.Is(err, ErrSchemaConflict) {
		t.Fatalf("name reused under new type_id: err = %v, want ErrSchemaConflict", err)
	}
}

// TestRegisterOrVerifyNode_FrozenRejectsNew pins that a frozen registry
// rejects an establish but still answers "identical" for a known type
// (the verify-only fast path).
func TestRegisterOrVerifyNode_FrozenRejectsNew(t *testing.T) {
	r := NewRegistry()
	if err := r.RegisterNode(userNodeDef(true)); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}
	if _, err := r.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	// Identical type on a frozen registry: verify-only no-op, no error.
	registered, err := r.RegisterOrVerifyNode(userNodeDef(true))
	if err != nil {
		t.Fatalf("verify identical on frozen registry: %v", err)
	}
	if registered {
		t.Fatalf("registered = true verifying on a frozen registry")
	}
	// New type on a frozen registry: ErrFrozen.
	other := &NodeTypeDef{TypeID: 2, Name: "Doc", Fields: []FieldDef{{FieldID: 1, Name: "title", Kind: KindString}}}
	if _, err := r.RegisterOrVerifyNode(other); !errors.Is(err, ErrFrozen) {
		t.Fatalf("establish on frozen registry: err = %v, want ErrFrozen", err)
	}
}

// TestRegisterOrVerifyEdge_Establish covers the edge counterpart and the
// on_subject_exit default normalisation.
func TestRegisterOrVerifyEdge_Establish(t *testing.T) {
	r := NewRegistry()
	et := &EdgeTypeDef{EdgeID: 1, Name: "Follows", FromTypeID: 1, ToTypeID: 1}
	registered, err := r.RegisterOrVerifyEdge(et)
	if err != nil {
		t.Fatalf("RegisterOrVerifyEdge: %v", err)
	}
	if !registered {
		t.Fatalf("registered = false, want true")
	}
	got := r.EdgeTypeByID(1)
	if got == nil {
		t.Fatalf("edge 1 not registered")
	}
	if got.OnSubjectExit != OnSubjectExitBoth {
		t.Fatalf("OnSubjectExit = %q, want %q (default)", got.OnSubjectExit, OnSubjectExitBoth)
	}
	// Re-registering the same edge (with the default normalised) is a no-op.
	registered, err = r.RegisterOrVerifyEdge(&EdgeTypeDef{EdgeID: 1, Name: "Follows", FromTypeID: 1, ToTypeID: 1})
	if err != nil {
		t.Fatalf("identical edge re-register: %v", err)
	}
	if registered {
		t.Fatalf("registered = true on identical edge re-register")
	}
}
