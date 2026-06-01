// SPDX-License-Identifier: AGPL-3.0-only

// Audit test for FINDING #643 (#7) — stored type-confusion (kind asymmetry) — FIXED.
//
// Before the fix, ingress UNDER-validated field kinds while the read path
// STRICTLY asserted them: coerceForKind handled only Bytes/Timestamp/Integer/
// JSON/Enum (Boolean/String/Float/Reference fell through to a permissive
// `default: v.AsInterface()`), and TypedToPayload did no kind validation at
// all. A wrong-typed value therefore slipped past CreateNode/UpdateNode and
// persisted, then permanently broke every schema-aware read of that node
// (PayloadToTyped errored, aborting the whole QueryNodes page — a durable
// tenant-wide DoS).
//
// Fix (this change):
//   - coerceForKind now rejects a wrong wire type for STRING/REFERENCE/BOOLEAN/
//     FLOAT with INVALID_ARGUMENT, exactly as it already did for BYTES/INTEGER/
//     TIMESTAMP/ENUM — strict typing on the write path (chosen philosophy).
//   - goToEntValue (read) degrades a kind-mismatch to the value's inferred type
//     instead of failing, so an already-poisoned row (or a typed-path mismatch)
//     stays readable and never bricks QueryNodes.
//
// The fixtureRegistry / mustStruct helpers and the schema fixture (field_id 4
// == KindBoolean, field_id 1/2 == KindString) live in translate_test.go in
// this same (white-box) package and are reused here. The earlier
// "_DemonstratesFinding7" tests that pinned the buggy asymmetry were removed
// when the fix landed; this gate enforces the round-trip invariant.

package payload

import (
	"errors"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// TestKindConfusion_SecureBehavior_Finding7 enforces the invariant: anything
// the ingress translators ACCEPT must round-trip through PayloadToTyped without
// error — equivalently, ingress must REJECT a wrong-kind value for
// Boolean/String/Float/Reference with an INVALID_ARGUMENT-class error. It uses
// only existing exported functions and accepts either fix shape:
//
//	(a) ingress rejects the wrong-kind value up front, OR
//	(b) ingress accepts and the read path then accepts it too (degrades).
func TestKindConfusion_SecureBehavior_Finding7(t *testing.T) {
	reg := fixtureRegistry(t)

	// --- Legacy Struct ingress path ------------------------------------
	// field_id 4 == KindBoolean; a string is the wrong kind.
	boolSlotWrongType := mustStruct(t, map[string]any{"4": "not a bool"})
	stored, err := StructToPayload(reg, 1, boolSlotWrongType)
	if err != nil {
		// Fix shape (a): ingress rejects up front.
		if !errors.Is(err, errs.ErrInvalidArgument) {
			t.Fatalf("ingress rejection must be INVALID_ARGUMENT-class, got %v", err)
		}
	} else {
		// Fix shape (b): ingress accepted, so the read MUST succeed.
		if _, rerr := PayloadToTyped(reg, 1, stored); rerr != nil {
			t.Fatalf("round-trip invariant violated: ingress accepted a value "+
				"the read path then rejected: %v", rerr)
		}
	}

	// --- field_id 1 == KindString; a number is the wrong kind ----------
	strSlotWrongType := mustStruct(t, map[string]any{"1": 12345.0})
	stored2, err2 := StructToPayload(reg, 1, strSlotWrongType)
	if err2 != nil {
		if !errors.Is(err2, errs.ErrInvalidArgument) {
			t.Fatalf("ingress rejection must be INVALID_ARGUMENT-class, got %v", err2)
		}
	} else {
		if _, rerr := PayloadToTyped(reg, 1, stored2); rerr != nil {
			t.Fatalf("round-trip invariant violated for string field: %v", rerr)
		}
	}

	// --- Typed EntValue ingress path -----------------------------------
	// A StringValue aimed at field_id 4 (KindBoolean). TypedToPayload takes no
	// schema, so the round-trip is satisfied by the read-path heal: the value
	// is read back as its inferred type rather than failing.
	typedWire := map[uint32]*pb.EntValue{
		4: {V: &pb.EntValue_StringValue{StringValue: "not a bool"}},
	}
	storedTyped, terr := TypedToPayload(typedWire)
	if terr != nil {
		if !errors.Is(terr, errs.ErrInvalidArgument) {
			t.Fatalf("typed ingress rejection must be INVALID_ARGUMENT-class, got %v", terr)
		}
	} else {
		if _, rerr := PayloadToTyped(reg, 1, storedTyped); rerr != nil {
			t.Fatalf("round-trip invariant violated on the typed path: ingress "+
				"accepted a value the read path then rejected: %v", rerr)
		}
	}
}

// TestKindConfusion_IngressRejectsWrongKind_Finding7 pins the chosen strict
// philosophy directly: the legacy Struct ingress rejects a wrong-typed value
// for each previously-unguarded scalar kind, just like BYTES/INTEGER already do.
func TestKindConfusion_IngressRejectsWrongKind_Finding7(t *testing.T) {
	reg := fixtureRegistry(t)
	cases := []struct {
		name  string
		field string
		value any
	}{
		{"string into boolean field", "4", "not a bool"},
		{"number into string field", "1", 12345.0},
		{"number into string field (field 2)", "2", 6789.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := StructToPayload(reg, 1, mustStruct(t, map[string]any{tc.field: tc.value}))
			if err == nil {
				t.Fatalf("ingress accepted a wrong-kind value for field %s; want InvalidArgument", tc.field)
			}
			if !errors.Is(err, errs.ErrInvalidArgument) {
				t.Fatalf("ingress rejection must be INVALID_ARGUMENT-class, got %v", err)
			}
		})
	}
}

// TestKindConfusion_ReadHealsPoisonedRow_Finding7 pins the read-side heal: a
// value that does not match the declared kind (modelling an already-poisoned
// row written before the ingress fix) is degraded to its inferred type rather
// than failing the read — so QueryNodes is not bricked for the whole type.
func TestKindConfusion_ReadHealsPoisonedRow_Finding7(t *testing.T) {
	reg := fixtureRegistry(t)
	// field_id 4 is KindBoolean; simulate a poisoned stored payload holding a
	// string there (bypassing ingress, as a pre-fix row would).
	poisoned := map[uint32]any{4: "not a bool"}
	typed, err := PayloadToTyped(reg, 1, poisoned)
	if err != nil {
		t.Fatalf("read of a poisoned row must not error (it must degrade): %v", err)
	}
	ev, ok := typed[4]
	if !ok || ev.GetStringValue() != "not a bool" {
		t.Fatalf("poisoned boolean field should read back as its inferred string value, got %+v", typed[4])
	}
}
