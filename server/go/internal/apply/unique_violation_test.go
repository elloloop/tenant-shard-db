// SPDX-License-Identifier: AGPL-3.0-only

package apply_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// newFixtureWithRegistry builds the same in-memory WAL + store + applier
// wiring as newFixture, but with a schema.Registry so the applier
// ensures unique/composite indexes and translates a constraint trip
// into the structured ALREADY_EXISTS detail (issue #566).
func newFixtureWithRegistry(t *testing.T, reg *schema.Registry) *fixture {
	t.Helper()
	w := wal.NewInMemory(1)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}
	dir := t.TempDir()
	cs, err := store.New(store.Options{RootDir: dir, WALMode: true, Registry: reg})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	gs, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })

	a, err := apply.New(apply.Options{
		Store:       cs,
		Global:      gs,
		Consumer:    w,
		Topic:       testTopic,
		GroupID:     testGroupID,
		PollTimeout: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}
	if err := cs.OpenTenant(context.Background(), testTenant); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	return &fixture{t: t, wal: w, store: cs, global: gs, applier: a}
}

func compositeRegistry(t *testing.T) *schema.Registry {
	t.Helper()
	reg := schema.NewRegistry()
	nt := &schema.NodeTypeDef{
		TypeID: 201,
		Fields: []schema.FieldDef{
			{FieldID: 1, Kind: schema.KindString},
			{FieldID: 2, Kind: schema.KindString},
			{FieldID: 3, Kind: schema.KindInteger},
		},
		CompositeUnique: []schema.CompositeUniqueDef{
			{FieldIDs: []uint32{1, 2}},
		},
	}
	if err := reg.RegisterNode(nt); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	return reg
}

// TestApplier_CompositeUniqueViolation drives a duplicate composite-key
// create through the full WAL→applier path and asserts the violation is
// memoized as UNIQUE_VIOLATION with the verbatim structured detail —
// without halting the consumer (a third, distinct create still applies).
func TestApplier_CompositeUniqueViolation(t *testing.T) {
	t.Parallel()
	f := newFixtureWithRegistry(t, compositeRegistry(t))

	first := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "c1", TsMs: 1700000000000,
		Ops: []map[string]any{mkCreateNode("id1", 201, map[string]any{"1": "google", "2": "uid-1"})},
	}
	dup := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "c2", TsMs: 1700000000001,
		Ops: []map[string]any{mkCreateNode("id2", 201, map[string]any{"1": "google", "2": "uid-1"})},
	}
	third := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "c3", TsMs: 1700000000002,
		Ops: []map[string]any{mkCreateNode("id3", 201, map[string]any{"1": "github", "2": "uid-9"})},
	}
	f.appendEvent(t, first)
	f.appendEvent(t, dup)
	f.appendEvent(t, third)

	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "c3")

	// The duplicate must be memoized as UNIQUE_VIOLATION with the
	// verbatim detail string.
	rec, err := f.store.CheckIdempotencyStatus(context.Background(), testTenant, "c2")
	if err != nil {
		t.Fatalf("CheckIdempotencyStatus: %v", err)
	}
	if !rec.Present || rec.Status != store.IdempotencyStatusUniqueViolation {
		t.Fatalf("dup idempotency = %+v, want UNIQUE_VIOLATION", rec)
	}
	var uv struct {
		Detail string `json:"Detail"`
	}
	if err := json.Unmarshal([]byte(rec.FailureJSON), &uv); err != nil {
		t.Fatalf("failure_json decode: %v", err)
	}
	want := "Composite unique constraint violation: tenant=tenant_a type_id=201 " +
		"constraint='(1,2)' fields=[1, 2] values=['google', 'uid-1'] already exists"
	if uv.Detail != want {
		t.Fatalf("detail mismatch:\n got=%q\nwant=%q", uv.Detail, want)
	}

	// The duplicate row must NOT have been written.
	if _, err := f.store.GetNode(context.Background(), testTenant, "id2"); err == nil {
		t.Fatalf("duplicate node id2 was written despite the constraint")
	}
	// The consumer did not halt: the third, distinct create applied.
	if _, err := f.store.GetNode(context.Background(), testTenant, "id3"); err != nil {
		t.Fatalf("third create did not apply (consumer halted?): %v", err)
	}
	// The first (winning) row is present.
	if _, err := f.store.GetNode(context.Background(), testTenant, "id1"); err != nil {
		t.Fatalf("winning row id1 missing: %v", err)
	}
}

// TestApplier_CompositeUniqueViolation_BigInt pins ADR-028 lossless
// rendering through the full applier path: a composite key whose value
// is an int64 above 2^53 must surface in the detail as the exact integer
// literal.
func TestApplier_CompositeUniqueViolation_BigInt(t *testing.T) {
	t.Parallel()
	f := newFixtureWithRegistry(t, compositeRegistry(t))

	const big int64 = 9223372036854775807
	first := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "b1", TsMs: 1700000000000,
		Ops: []map[string]any{mkCreateNode("b-id1", 201, map[string]any{"1": "google", "2": big})},
	}
	dup := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "b2", TsMs: 1700000000001,
		Ops: []map[string]any{mkCreateNode("b-id2", 201, map[string]any{"1": "google", "2": big})},
	}
	f.appendEvent(t, first)
	f.appendEvent(t, dup)

	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "b2")

	rec, err := f.store.CheckIdempotencyStatus(context.Background(), testTenant, "b2")
	if err != nil {
		t.Fatalf("CheckIdempotencyStatus: %v", err)
	}
	if !rec.Present || rec.Status != store.IdempotencyStatusUniqueViolation {
		t.Fatalf("dup idempotency = %+v, want UNIQUE_VIOLATION", rec)
	}
	if !strings.Contains(rec.FailureJSON, "9223372036854775807") {
		t.Fatalf("big int not rendered losslessly: %s", rec.FailureJSON)
	}
	if strings.Contains(rec.FailureJSON, "e+") || strings.Contains(rec.FailureJSON, "E+") {
		t.Fatalf("big int rendered in scientific notation: %s", rec.FailureJSON)
	}
}
