// SPDX-License-Identifier: AGPL-3.0-only

package apply_test

import (
	"context"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// makeCreateEvent builds a single-op create_node Event for the given
// tenant, idempotency key, and node id.
func makeCreateEvent(tenantID, idempKey, nodeID string) apply.Event {
	return apply.Event{
		TenantID:       tenantID,
		Actor:          "user:svc",
		IdempotencyKey: idempKey,
		TsMs:           1700000000000,
		Ops:            []map[string]any{mkCreateNode(nodeID, 1, map[string]any{"1": "v"})},
	}
}

// waitFor polls cond until it returns true or the deadline fires, failing
// the test with what it was waiting for.
func waitFor(t *testing.T, within time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s", within, what)
}

// TestApplier_ProgressAdvancesOnApply pins that a committed batch advances
// the apply-progress signal (#653): the in-memory State and the Prometheus
// counters both move, and the applier reports a healthy, not-stalled,
// caught-up state afterwards. NOT parallel — it reads process-global
// counters, so it must run without a concurrent applier.
func TestApplier_ProgressAdvancesOnApply(t *testing.T) {
	f := newFixture(t)

	beforeRecords := testutil.ToFloat64(metrics.ApplierRecordsAppliedCollector())
	beforeBatches := testutil.ToFloat64(metrics.ApplierBatchesAppliedCollector())

	ev := makeCreateEvent(testTenant, "k-progress", "node-progress")
	f.appendEvent(t, ev)
	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "k-progress")

	// The applied_events row is written in-txn, before finalizeBatch marks
	// progress; poll until the batch fully finalises (in-flight cleared).
	waitFor(t, 2*time.Second, "batch to finalise", func() bool {
		s := f.applier.State()
		return s.ApplyingSinceMilli == 0 && s.LastProgressMilli > 0
	})

	s := f.applier.State()
	if s.LastProgressMilli == 0 {
		t.Fatalf("last progress never advanced: %+v", s)
	}
	if s.ApplyingSinceMilli != 0 {
		t.Fatalf("applier still reports a batch in-flight after apply: %+v", s)
	}

	if got := testutil.ToFloat64(metrics.ApplierRecordsAppliedCollector()) - beforeRecords; got != 1 {
		t.Fatalf("records_applied delta = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.ApplierBatchesAppliedCollector()) - beforeBatches; got != 1 {
		t.Fatalf("batches_applied delta = %v, want 1", got)
	}

	rd := f.applier.Readiness(time.Now().UnixMilli(), time.Minute)
	if rd.Stalled || rd.State != "idle" {
		t.Fatalf("readiness after apply = %+v, want idle/not-stalled", rd)
	}
}

// TestApplier_StallIsObservable is the regression test for the #650 prod
// failure: a blocked applier (here: the test holds the per-tenant write
// lock so the applier's BeginBatch blocks — faithfully reproducing SQLite
// lock contention on an existing shard) is observably STALLED via the
// readiness signal, and recovers cleanly once unblocked. Without #653 a
// stall was invisible (writes keep ACK'ing, nothing materialises, reads
// time out, 0 restarts).
//
// The applier lifecycle is managed inline (not via runApplierUntilApplied)
// so the deferred teardown ALWAYS releases the lock before waiting on the
// applier goroutine — otherwise a failed assertion would wedge the test
// (the blocked BeginBatch does not observe context cancellation).
func TestApplier_StallIsObservable(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	// Hold the per-tenant write lock so the applier's apply blocks.
	bt, err := f.store.BeginBatch(context.Background(), testTenant)
	if err != nil {
		t.Fatalf("BeginBatch (hold lock): %v", err)
	}
	released := false
	release := func() {
		if !released {
			_ = bt.Rollback()
			released = true
		}
	}
	// ORDERING INVARIANT: release() MUST run before cancel()+<-done. The
	// applier is blocked in BeginBatch on a sync.Mutex, which does NOT
	// observe context cancellation — so cancel() alone cannot unwedge it.
	// Releasing the lock first lets the applier drain and Run return; only
	// then is <-done guaranteed not to hang.
	defer func() {
		release()
		cancel()
		<-done
	}()

	f.appendEvent(t, makeCreateEvent(testTenant, "k-stall", "node-stall"))
	go func() { done <- f.applier.Run(ctx) }()

	// The applier picks up the batch and blocks in BeginBatch; markApplyStart
	// (processBatch entry) runs first, so ApplyingSinceMilli is set.
	waitFor(t, 5*time.Second, "applier to begin applying", func() bool {
		return f.applier.State().ApplyingSinceMilli > 0
	})

	// With a small threshold the aging in-flight batch is reported stalled.
	const threshold = 100 * time.Millisecond
	waitFor(t, 5*time.Second, "applier to be reported stalled", func() bool {
		return f.applier.Readiness(time.Now().UnixMilli(), threshold).Stalled
	})
	if rd := f.applier.Readiness(time.Now().UnixMilli(), threshold); rd.State != "stalled" {
		t.Fatalf("readiness while blocked = %+v, want state=stalled", rd)
	}
	// An idle applier must NOT be flagged stalled — guard against a signal
	// that just trips on elapsed wall-clock. Here a batch IS in-flight, so
	// stalled is correct; the idle case is covered in progress_test.go.

	before := f.applier.State().LastProgressMilli

	// Release the lock; the applier proceeds and recovers.
	release()
	f.waitForIdempKey(t, testTenant, "k-stall")
	waitFor(t, 5*time.Second, "applier to recover (in-flight cleared)", func() bool {
		s := f.applier.State()
		return s.ApplyingSinceMilli == 0 && s.LastProgressMilli >= before
	})
	if rd := f.applier.Readiness(time.Now().UnixMilli(), threshold); rd.Stalled {
		t.Fatalf("readiness after recovery = %+v, want not-stalled", rd)
	}
}
