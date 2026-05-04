package entdb

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Composite (multi-field) unique-constraint error parsing — mirrors
// the Python SDK's `_parse_composite_unique_constraint_detail`.
// The server emits ALREADY_EXISTS with the structured detail::
//
//	Composite unique constraint violation: type_id=<int>
//	constraint='<name>' fields=[<int>, ...] values=[<repr>, ...]
//	already exists
//
// `parseUniqueConstraintFromStatus` must populate ConstraintName,
// FieldIDs, Values, and TypeID on the returned typed error so callers
// can write `errors.As(err, &uce)` and discriminate composite vs.
// single-field violations via `uce.IsComposite()`.

func TestParseCompositeUniqueConstraint_AlreadyExists(t *testing.T) {
	detail := "Composite unique constraint violation: tenant=t1 type_id=201 " +
		"constraint='provider_user_id' fields=[1, 2] values=['google', 'uid-1'] " +
		"already exists"
	st := status.New(codes.AlreadyExists, detail)
	uce := parseUniqueConstraintFromStatus(st.Err(), "t1")
	if uce == nil {
		t.Fatalf("expected *UniqueConstraintError, got nil")
	}
	if !uce.IsComposite() {
		t.Fatalf("IsComposite() = false; want true")
	}
	if uce.ConstraintName != "provider_user_id" {
		t.Errorf("ConstraintName = %q, want provider_user_id", uce.ConstraintName)
	}
	if uce.TypeID != 201 {
		t.Errorf("TypeID = %d, want 201", uce.TypeID)
	}
	if len(uce.FieldIDs) != 2 || uce.FieldIDs[0] != 1 || uce.FieldIDs[1] != 2 {
		t.Errorf("FieldIDs = %v, want [1 2]", uce.FieldIDs)
	}
	if len(uce.Values) != 2 {
		t.Fatalf("Values = %v, want length 2", uce.Values)
	}
	// Values come through as the raw Python repr — typed-error
	// callers see them as strings for diagnostic logging.
	if !strings.Contains(uce.Values[0].(string), "google") {
		t.Errorf("Values[0] = %v, missing 'google'", uce.Values[0])
	}
}

func TestParseCompositeUniqueConstraint_FallsBackOnSingleField(t *testing.T) {
	// Single-field detail must NOT be parsed as composite — it
	// keeps ConstraintName empty so `IsComposite()` returns false.
	detail := "Unique constraint violation: type_id=101 field_id=1 " +
		"value='alice@example.com' already exists"
	st := status.New(codes.AlreadyExists, detail)
	uce := parseUniqueConstraintFromStatus(st.Err(), "t1")
	if uce == nil {
		t.Fatalf("expected *UniqueConstraintError, got nil")
	}
	if uce.IsComposite() {
		t.Errorf("single-field detail parsed as composite: %+v", uce)
	}
	if uce.ConstraintName != "" {
		t.Errorf("ConstraintName = %q; want empty for single-field", uce.ConstraintName)
	}
}

func TestUniqueConstraintError_ErrorsAs_Composite(t *testing.T) {
	// Callers should be able to discriminate via errors.As + the
	// IsComposite helper without depending on a separate type.
	detail := "Composite unique constraint violation: type_id=201 " +
		"constraint='ck' fields=[1, 2] values=['a', 'b'] already exists"
	st := status.New(codes.AlreadyExists, detail)
	var src error = st.Err()
	wrapped := parseUniqueConstraintFromStatus(src, "t1")

	var got *UniqueConstraintError
	if !errors.As(wrapped, &got) {
		t.Fatalf("errors.As failed to extract *UniqueConstraintError")
	}
	if !got.IsComposite() {
		t.Fatalf("expected composite; got %+v", got)
	}
}

func TestParseCompositeUniqueConstraint_NotAlreadyExists(t *testing.T) {
	st := status.New(codes.Internal, "nope")
	if uce := parseUniqueConstraintFromStatus(st.Err(), "t1"); uce != nil {
		t.Errorf("expected nil for non-AlreadyExists, got %+v", uce)
	}
}
