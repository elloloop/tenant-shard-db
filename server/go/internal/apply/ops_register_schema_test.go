// SPDX-License-Identifier: AGPL-3.0-only

package apply_test

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// mkRegisterSchema builds a register_schema op carrying a single User
// node type (type_id=1) whose email field is unique. The shape mirrors
// the cross-language schema JSON contract (snake_case keys) so the
// applier's decodeSchemaOp round-trips it into schema.NodeTypeDef.
func mkRegisterSchema(emailUnique bool) map[string]any {
	return map[string]any{
		"op": string(apply.OpRegisterSchema),
		"node_types": []any{
			map[string]any{
				"type_id": 1,
				"name":    "User",
				"fields": []any{
					map[string]any{"field_id": 1, "name": "email", "kind": "str", "unique": emailUnique},
					map[string]any{"field_id": 2, "name": "name", "kind": "str"},
				},
			},
		},
	}
}

// TestApplyRegisterSchema_RegistersAndIndexes drives a register_schema +
// create_node event through the full WAL→applier path against an EMPTY,
// non-frozen registry and asserts the type is registered, the unique
// index is created, and a duplicate-email create is memoized as a
// UNIQUE_VIOLATION (proving the index bites).
func TestApplyRegisterSchema_RegistersAndIndexes(t *testing.T) {
	t.Parallel()
	reg := schema.NewRegistry() // empty, NOT frozen
	f := newFixtureWithRegistry(t, reg)

	first := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "s1", TsMs: 1700000000000,
		Ops: []map[string]any{
			mkRegisterSchema(true),
			mkCreateNode("n1", 1, map[string]any{"1": "alice@example.com"}),
		},
	}
	dup := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "s2", TsMs: 1700000000001,
		Ops: []map[string]any{
			mkRegisterSchema(true), // identical schema -> idempotent no-op
			mkCreateNode("n2", 1, map[string]any{"1": "alice@example.com"}),
		},
	}
	f.appendEvent(t, first)
	f.appendEvent(t, dup)

	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "s2")

	// Registry now holds the User type with a unique email field, rebuilt
	// from the WAL register_schema op.
	if reg.NodeTypeByID(1) == nil {
		t.Fatalf("type 1 not registered by the schema op")
	}
	if got := reg.UniqueFieldIDs(1); len(got) != 1 || got[0] != 1 {
		t.Fatalf("UniqueFieldIDs(1) = %v, want [1]", got)
	}

	// First create won; the duplicate-email create was memoized as a
	// UNIQUE_VIOLATION (the unique index the schema op created fired).
	rec, err := f.store.CheckIdempotencyStatus(context.Background(), testTenant, "s2")
	if err != nil {
		t.Fatalf("CheckIdempotencyStatus: %v", err)
	}
	if !rec.Present || rec.Status != store.IdempotencyStatusUniqueViolation {
		t.Fatalf("dup idempotency = %+v, want UNIQUE_VIOLATION", rec)
	}
	if _, err := f.store.GetNode(context.Background(), testTenant, "n1"); err != nil {
		t.Fatalf("winning row n1 missing: %v", err)
	}
	if _, err := f.store.GetNode(context.Background(), testTenant, "n2"); err == nil {
		t.Fatalf("duplicate row n2 written despite the unique constraint")
	}
}

// TestApplyRegisterSchema_Conflict pins establish-or-reject in the
// applier: a register_schema op whose definition diverges from the
// already-registered type is memoized as FAILED_PRECONDITION (a
// deterministic outcome) WITHOUT halting the consumer, and the registered
// type is unchanged.
func TestApplyRegisterSchema_Conflict(t *testing.T) {
	t.Parallel()
	reg := schema.NewRegistry()
	f := newFixtureWithRegistry(t, reg)

	establish := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "e1", TsMs: 1700000000000,
		Ops: []map[string]any{
			mkRegisterSchema(true),
			mkCreateNode("c1", 1, map[string]any{"1": "a@example.com"}),
		},
	}
	conflict := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "e2", TsMs: 1700000000001,
		Ops: []map[string]any{
			mkRegisterSchema(false), // email NOT unique -> conflict
			mkCreateNode("c2", 1, map[string]any{"1": "b@example.com"}),
		},
	}
	// A third, well-formed event proves the consumer did not halt.
	after := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "e3", TsMs: 1700000000002,
		Ops: []map[string]any{
			mkCreateNode("c3", 1, map[string]any{"1": "c@example.com"}),
		},
	}
	f.appendEvent(t, establish)
	f.appendEvent(t, conflict)
	f.appendEvent(t, after)

	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "e3")

	rec, err := f.store.CheckIdempotencyStatus(context.Background(), testTenant, "e2")
	if err != nil {
		t.Fatalf("CheckIdempotencyStatus: %v", err)
	}
	if !rec.Present || rec.Status != store.IdempotencyStatusFailedPrecondition {
		t.Fatalf("conflict idempotency = %+v, want FAILED_PRECONDITION", rec)
	}
	// The conflicting event aborted: its data op (c2) did not commit.
	if _, err := f.store.GetNode(context.Background(), testTenant, "c2"); err == nil {
		t.Fatalf("conflict event data op c2 committed despite the reject")
	}
	// The registered type is unchanged (email still unique).
	if got := reg.UniqueFieldIDs(1); len(got) != 1 || got[0] != 1 {
		t.Fatalf("after conflict UniqueFieldIDs(1) = %v, want [1]", got)
	}
	// The consumer did not halt: the third event applied.
	if _, err := f.store.GetNode(context.Background(), testTenant, "c3"); err != nil {
		t.Fatalf("post-conflict event c3 did not apply (consumer halted?): %v", err)
	}
}
