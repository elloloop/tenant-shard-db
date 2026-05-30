// SPDX-License-Identifier: AGPL-3.0-only

package apply_test

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// mkRegisterSchemaSearchable builds a register_schema op for a type whose
// single string field is searchable, so create_node triggers FTS5 index
// creation (EnsureFTSIndexConn) inside the apply batch.
func mkRegisterSchemaSearchable() map[string]any {
	return map[string]any{
		"op": string(apply.OpRegisterSchema),
		"node_types": []any{
			map[string]any{
				"type_id": 1,
				"name":    "Doc",
				"fields": []any{
					map[string]any{"field_id": 1, "name": "body", "kind": "str", "searchable": true},
				},
			},
		},
	}
}

// TestApplier_FTSWriteAfterRolledBackBatch is the end-to-end (#629)
// regression: it drives the REAL applier crash sequence rather than the
// store internals.
//
// Batch A registers a searchable type and creates a node (creating the
// fts_t1 virtual table) but its in-batch CAS update misses, so the whole
// batch fails its precondition and rolls back — SQLite reverts the CREATE
// VIRTUAL TABLE. Before the fix, the process-local index cache still
// recorded fts_t1 as present, so Batch B (a valid write of the same type)
// cache-hit, skipped re-creating the table, and the applier crashed with
// "no such table: fts_t1". After the fix, Rollback invalidated the cache
// and Batch B re-creates the table and applies cleanly.
func TestApplier_FTSWriteAfterRolledBackBatch(t *testing.T) {
	t.Parallel()
	reg := schema.NewRegistry()
	f := newFixtureWithRegistry(t, reg)

	batchA := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "rb-a", TsMs: 1700000000000,
		Ops: []map[string]any{
			mkRegisterSchemaSearchable(),
			mkCreateNode("n1", 1, map[string]any{"1": "the quick brown fox"}),
			// CAS update of the just-created n1 expecting the wrong value ->
			// precondition miss -> the batch aborts and rolls back.
			preconditionOp("n1", 1, map[string]any{"1": "changed"}, "1", "body", "WRONG-VALUE"),
		},
	}
	batchB := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "rb-b", TsMs: 1700000000001,
		Ops: []map[string]any{
			mkCreateNode("n2", 1, map[string]any{"1": "the quick brown fox"}),
		},
	}
	f.appendEvent(t, batchA)
	f.appendEvent(t, batchB)

	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "rb-b")

	ctx := context.Background()

	// Batch A rolled back: n1 must NOT exist; memoized as a precondition failure.
	if _, err := f.store.GetNode(ctx, testTenant, "n1"); err == nil {
		t.Fatalf("n1 was written despite the rolled-back batch")
	}
	rec, err := f.store.CheckIdempotencyStatus(ctx, testTenant, "rb-a")
	if err != nil {
		t.Fatalf("CheckIdempotencyStatus rb-a: %v", err)
	}
	if !rec.Present || rec.Status != store.IdempotencyStatusFailedPrecondition {
		t.Fatalf("batch A status = %+v, want FAILED_PRECONDITION", rec)
	}

	// Batch B applied without crashing the applier, and n2 is searchable —
	// proving fts_t1 was RE-CREATED after the rollback, not skipped (#629).
	if _, err := f.store.GetNode(ctx, testTenant, "n2"); err != nil {
		t.Fatalf("n2 missing after a valid write following a rolled-back FTS batch (issue #629): %v", err)
	}
	hits, err := f.store.SearchNodes(ctx, testTenant, 1, "fox", []uint32{1}, 10, 0)
	if err != nil {
		t.Fatalf("SearchNodes (must succeed — proves fts_t1 exists): %v", err)
	}
	if len(hits) != 1 || hits[0].NodeID != "n2" {
		t.Fatalf("FTS search after recovery returned %+v, want [n2]", hits)
	}
}
