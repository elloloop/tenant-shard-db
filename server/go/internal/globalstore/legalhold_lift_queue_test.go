// SPDX-License-Identifier: AGPL-3.0-only

// Tests for the durable legal_hold_lift_queue (EPIC #511 Gap 1,
// ADR-015). The critical assertion is that ApplyLegalHoldSet enqueues
// the pending lift IN THE SAME transaction that clears the hold + flips
// tenant_registry.status — so a crash after an explicit release still
// has the lift durably recorded. ON never enqueues; the queue CRUD
// round-trips; re-release keeps the earliest enqueued_at.

package globalstore_test

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
)

func TestApplyLegalHoldSet_ReleaseEnqueuesLiftAtomically(t *testing.T) {
	clock := int64(1700000000)
	gs := newStore(t, func() int64 { return clock })
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	// ON: hold set, registry flips to legal_hold, queue stays EMPTY.
	if err := gs.ApplyLegalHoldSet(ctx, globalstore.LegalHoldApply{
		TenantID: "acme", HeldBy: "admin:root", Reason: "x",
		CreatedAt: clock, Enabled: true,
	}); err != nil {
		t.Fatalf("ApplyLegalHoldSet(on): %v", err)
	}
	if held, _ := gs.IsLegalHoldSet(ctx, "acme"); !held {
		t.Fatal("hold not set after enable")
	}
	if q, err := gs.GetLegalHoldLiftQueue(ctx); err != nil || len(q) != 0 {
		t.Fatalf("queue after ON = %+v err=%v; want empty", q, err)
	}

	// OFF (explicit release): hold cleared, registry back to active,
	// AND a pending-lift row exists — all from the one apply step.
	clock = 1700000050
	if err := gs.ApplyLegalHoldSet(ctx, globalstore.LegalHoldApply{
		TenantID: "acme", HeldBy: "admin:root", Reason: "",
		CreatedAt: clock, Enabled: false,
	}); err != nil {
		t.Fatalf("ApplyLegalHoldSet(off): %v", err)
	}
	if held, _ := gs.IsLegalHoldSet(ctx, "acme"); held {
		t.Fatal("hold still set after release")
	}
	tnt, _ := gs.GetTenant(ctx, "acme")
	if tnt == nil || tnt.Status != "active" {
		t.Fatalf("registry status = %v; want active", tnt)
	}
	q, err := gs.GetLegalHoldLiftQueue(ctx)
	if err != nil {
		t.Fatalf("GetLegalHoldLiftQueue: %v", err)
	}
	if len(q) != 1 || q[0].TenantID != "acme" || q[0].EnqueuedAt != clock {
		t.Fatalf("lift queue = %+v; want [{acme %d}]", q, clock)
	}
}

func TestLegalHoldLiftQueue_CRUDRoundTrip(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()

	if n, err := gs.CountLegalHoldLiftQueue(ctx); err != nil || n != 0 {
		t.Fatalf("empty count = %d err=%v; want 0", n, err)
	}
	if ok, err := gs.DequeueLegalHoldLift(ctx, "ghost"); err != nil || ok {
		t.Fatalf("dequeue missing = %v err=%v; want false/nil", ok, err)
	}

	// Enqueue via the only real path: an explicit release
	// (ApplyLegalHoldSet enabled=false) of a registered tenant.
	for _, tt := range []struct {
		tid string
		at  int64
	}{{"t-b", 200}, {"t-a", 100}, {"t-c", 300}} {
		if _, err := gs.CreateTenant(ctx, tt.tid, tt.tid, "us-east-1"); err != nil {
			t.Fatalf("CreateTenant %s: %v", tt.tid, err)
		}
		if err := gs.ApplyLegalHoldSet(ctx, globalstore.LegalHoldApply{
			TenantID: tt.tid, HeldBy: "admin:root", CreatedAt: tt.at, Enabled: false,
		}); err != nil {
			t.Fatalf("enqueue %s: %v", tt.tid, err)
		}
	}
	q, err := gs.GetLegalHoldLiftQueue(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// Oldest-first ordering (enqueued_at, then tenant_id).
	if len(q) != 3 || q[0].TenantID != "t-a" || q[1].TenantID != "t-b" || q[2].TenantID != "t-c" {
		t.Fatalf("ordering = %+v; want t-a,t-b,t-c by enqueued_at", q)
	}
	if n, _ := gs.CountLegalHoldLiftQueue(ctx); n != 3 {
		t.Fatalf("count = %d; want 3", n)
	}
	if ok, err := gs.DequeueLegalHoldLift(ctx, "t-b"); err != nil || !ok {
		t.Fatalf("dequeue t-b = %v err=%v; want true", ok, err)
	}
	if n, _ := gs.CountLegalHoldLiftQueue(ctx); n != 2 {
		t.Fatalf("count after dequeue = %d; want 2", n)
	}
}

func TestApplyLegalHoldSet_ReReleaseKeepsEarliestEnqueuedAt(t *testing.T) {
	clock := int64(1700000000)
	gs := newStore(t, func() int64 { return clock })
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	off := func(at int64) {
		if err := gs.ApplyLegalHoldSet(ctx, globalstore.LegalHoldApply{
			TenantID: "acme", HeldBy: "admin:root", CreatedAt: at, Enabled: false,
		}); err != nil {
			t.Fatalf("ApplyLegalHoldSet(off @%d): %v", at, err)
		}
	}
	off(1700000010)
	off(1700000099) // re-release later — must NOT bump enqueued_at
	q, err := gs.GetLegalHoldLiftQueue(ctx)
	if err != nil || len(q) != 1 {
		t.Fatalf("queue = %+v err=%v; want one row", q, err)
	}
	if q[0].EnqueuedAt != 1700000010 {
		t.Fatalf("enqueued_at = %d; want earliest 1700000010 (re-release must not reset age)",
			q[0].EnqueuedAt)
	}
}
