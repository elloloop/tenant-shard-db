// SPDX-License-Identifier: AGPL-3.0-only

package apply_test

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// deleteWhereOp builds a delete_where op-dict in the on-WAL shape the
// handler produces: field-id-keyed predicates with stable string
// operator tokens.
func deleteWhereOp(typeID int32, limit int, preds ...map[string]any) map[string]any {
	where := make([]any, 0, len(preds))
	for _, p := range preds {
		where = append(where, p)
	}
	return map[string]any{
		"op":      string(apply.OpDeleteWhere),
		"type_id": typeID,
		"where":   where,
		"limit":   limit,
	}
}

func pred(fieldID int, op string, value any) map[string]any {
	return map[string]any{"field_id": fieldID, "op": op, "value": value}
}

// TestApplier_DeleteWhere_PredicateMatchDeletesOnlyMatching is the
// core #504 contract: a predicate sweep removes exactly the matching
// nodes of the requested type and leaves everything else intact.
func TestApplier_DeleteWhere_PredicateMatchDeletesOnlyMatching(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	// Seed type 1 = WebAuthn challenges with field_id "2" = expires_at.
	// expired1 / expired2 are stale; fresh1 is still valid; other1 is a
	// different type that must never be touched.
	seed := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "seed",
		Ops: []map[string]any{
			mkCreateNode("expired1", 1, map[string]any{"2": float64(100)}),
			mkCreateNode("expired2", 1, map[string]any{"2": float64(200)}),
			mkCreateNode("fresh1", 1, map[string]any{"2": float64(9000)}),
			mkCreateNode("other1", 2, map[string]any{"2": float64(100)}),
		},
	}
	f.appendEvent(t, seed)

	// Sweep: delete type-1 nodes where expires_at < 1000.
	ev := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "sweep1",
		Ops: []map[string]any{
			deleteWhereOp(1, 0, pred(2, "lt", float64(1000))),
		},
	}
	f.appendEvent(t, ev)

	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "sweep1")

	// Matching nodes gone.
	for _, id := range []string{"expired1", "expired2"} {
		if _, err := f.store.GetNode(context.Background(), testTenant, id); err == nil {
			t.Fatalf("node %q should have been swept", id)
		}
	}
	// Non-matching + different-type nodes survive.
	for _, id := range []string{"fresh1", "other1"} {
		if _, err := f.store.GetNode(context.Background(), testTenant, id); err != nil {
			t.Fatalf("node %q must survive the sweep: %v", id, err)
		}
	}

	rec, err := f.store.CheckIdempotencyStatus(context.Background(), testTenant, "sweep1")
	if err != nil {
		t.Fatalf("CheckIdempotencyStatus: %v", err)
	}
	if rec.Status != store.IdempotencyStatusApplied {
		t.Fatalf("status=%q want APPLIED", rec.Status)
	}
}

// TestApplier_DeleteWhere_IdempotentReapply pins issue #500 interaction:
// re-appending the SAME sweep event (same idempotency key) is a no-op
// replay — it must not error, must not delete the now-recreated node,
// and the cached status stays APPLIED.
func TestApplier_DeleteWhere_IdempotentReapply(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	seed := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "seed",
		Ops: []map[string]any{
			mkCreateNode("tok1", 1, map[string]any{"2": float64(100)}),
		},
	}
	f.appendEvent(t, seed)

	sweep := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "sweep-once",
		Ops: []map[string]any{
			deleteWhereOp(1, 0, pred(2, "lt", float64(1000))),
		},
	}
	f.appendEvent(t, sweep)

	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "sweep-once")

	if _, err := f.store.GetNode(context.Background(), testTenant, "tok1"); err == nil {
		t.Fatalf("tok1 should have been swept on first apply")
	}

	// Recreate the same node id, then replay the SAME sweep event. The
	// idempotency memo must short-circuit before the op runs, leaving
	// the recreated node intact.
	recreate := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "recreate",
		Ops: []map[string]any{
			mkCreateNode("tok1", 1, map[string]any{"2": float64(100)}),
		},
	}
	f.appendEvent(t, recreate)
	f.waitForIdempKey(t, testTenant, "recreate")

	f.appendEvent(t, sweep) // same idempotency key "sweep-once"
	// Give the applier a beat to process the replayed record.
	f.waitForIdempKey(t, testTenant, "sweep-once")

	if _, err := f.store.GetNode(context.Background(), testTenant, "tok1"); err != nil {
		t.Fatalf("recreated tok1 must survive an idempotent sweep replay: %v", err)
	}
	rec, err := f.store.CheckIdempotencyStatus(context.Background(), testTenant, "sweep-once")
	if err != nil {
		t.Fatalf("CheckIdempotencyStatus: %v", err)
	}
	if rec.Status != store.IdempotencyStatusApplied {
		t.Fatalf("status=%q want APPLIED", rec.Status)
	}
}

// TestApplier_DeleteWhere_LimitIsBestEffort verifies the limit caps the
// number deleted per op (Postgres DELETE … LIMIT semantics) — a second
// sweep drains the rest.
func TestApplier_DeleteWhere_LimitIsBestEffort(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	seed := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "seed",
		Ops: []map[string]any{
			mkCreateNode("e1", 1, map[string]any{"2": float64(1)}),
			mkCreateNode("e2", 1, map[string]any{"2": float64(2)}),
			mkCreateNode("e3", 1, map[string]any{"2": float64(3)}),
		},
	}
	f.appendEvent(t, seed)

	sweep1 := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "sweep-lim-1",
		Ops: []map[string]any{
			deleteWhereOp(1, 2, pred(2, "lt", float64(1000))),
		},
	}
	f.appendEvent(t, sweep1)

	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "sweep-lim-1")

	remaining := 0
	for _, id := range []string{"e1", "e2", "e3"} {
		if _, err := f.store.GetNode(context.Background(), testTenant, id); err == nil {
			remaining++
		}
	}
	if remaining != 1 {
		t.Fatalf("after limit=2 sweep: remaining=%d want 1", remaining)
	}

	// Second sweep drains the last row.
	sweep2 := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "sweep-lim-2",
		Ops: []map[string]any{
			deleteWhereOp(1, 2, pred(2, "lt", float64(1000))),
		},
	}
	f.appendEvent(t, sweep2)
	f.waitForIdempKey(t, testTenant, "sweep-lim-2")

	for _, id := range []string{"e1", "e2", "e3"} {
		if _, err := f.store.GetNode(context.Background(), testTenant, id); err == nil {
			t.Fatalf("node %q should be drained after the second sweep", id)
		}
	}
}

// TestApplier_DeleteWhere_CascadesEdges confirms parity with
// applyDeleteNode: a swept node's edges are removed too.
func TestApplier_DeleteWhere_CascadesEdges(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	seed := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "seed",
		Ops: []map[string]any{
			mkCreateNode("a", 1, map[string]any{"2": float64(1)}),
			mkCreateNode("b", 1, map[string]any{"2": float64(9000)}),
			{
				"op":      string(apply.OpCreateEdge),
				"edge_id": int32(5),
				"from":    "a",
				"to":      "b",
			},
		},
	}
	f.appendEvent(t, seed)

	sweep := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "sweep-edge",
		Ops: []map[string]any{
			deleteWhereOp(1, 0, pred(2, "lt", float64(1000))),
		},
	}
	f.appendEvent(t, sweep)

	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "sweep-edge")

	if _, err := f.store.GetNode(context.Background(), testTenant, "a"); err == nil {
		t.Fatalf("node a should have been swept")
	}
	// The edge from a->b must be gone (cascade), so b has no inbound edge.
	edges, err := f.store.GetEdgesTo(context.Background(), testTenant, "b", nil, 100)
	if err != nil {
		t.Fatalf("GetEdgesTo: %v", err)
	}
	if len(edges) != 0 {
		t.Fatalf("edge to b should have cascaded away, got %d edges", len(edges))
	}
}

// TestApplier_DeleteWhere_EmptyPredicateIsPoison guards the
// defense-in-depth path: a malformed WAL record with no predicate is a
// poison event (the handler rejects this at ingress, but the applier
// must not silently nuke a whole type if a bad record slips through).
func TestApplier_DeleteWhere_EmptyPredicateIsPoison(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	seed := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "seed",
		Ops: []map[string]any{
			mkCreateNode("keep", 1, map[string]any{"2": float64(1)}),
		},
	}
	f.appendEvent(t, seed)

	bad := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "bad-sweep",
		Ops: []map[string]any{
			{
				"op":      string(apply.OpDeleteWhere),
				"type_id": int32(1),
				"where":   []any{},
				"limit":   0,
			},
		},
	}
	f.appendEvent(t, bad)

	// halt-on-poison default: the applier loop returns an error and does
	// NOT advance past the poison record. Run once and assert the seed
	// node still exists (the bad op never committed).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.applier.Run(ctx) }()
	f.waitForIdempKey(t, testTenant, "seed")
	cancel()
	<-done

	if _, err := f.store.GetNode(context.Background(), testTenant, "keep"); err != nil {
		t.Fatalf("keep must survive a poison empty-predicate sweep: %v", err)
	}
}
