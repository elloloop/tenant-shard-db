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

// newApplierOn builds a second applier over the fixture's WAL+store under a
// distinct group id (independent offset), optionally with an injected clock
// for deterministic gauge-value assertions. nowFn nil => real time.
func (f *fixture) newApplierOn(t *testing.T, groupID string, nowFn func() int64) *apply.Applier {
	t.Helper()
	a, err := apply.New(apply.Options{
		Store:       f.store,
		Global:      f.global,
		Consumer:    f.wal,
		Topic:       testTopic,
		GroupID:     groupID,
		PollTimeout: 25 * time.Millisecond,
		NowFn:       nowFn,
	})
	if err != nil {
		t.Fatalf("apply.New(%s): %v", groupID, err)
	}
	return a
}

// runUntilDone runs a's Run loop in a goroutine and registers cleanup that
// cancels and joins it. Returns the done channel so a halting applier can be
// awaited.
func runInBackground(t *testing.T, a *apply.Applier) (cancel func(), done chan error) {
	t.Helper()
	ctx, cancelF := context.WithCancel(context.Background())
	done = make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	t.Cleanup(func() {
		cancelF()
		<-done
	})
	return cancelF, done
}

// TestApplier_GaugeValuesAndUnits pins the metric VALUES and the
// millis->seconds conversion (#653 coverage). A silent /1000 typo or a
// missing idle-clear would corrupt dashboards/alerts; the in-memory State
// assertions elsewhere can't catch that. Uses a fixed clock so the gauge
// values are exact.
func TestApplier_GaugeValuesAndUnits(t *testing.T) {
	const fixedMs = int64(1_700_000_123_000) // divisible by 1000 -> exact seconds
	const wantSec = float64(fixedMs) / 1000.0

	f := newFixture(t) // provides wal+store+global; its default applier is NOT started here
	a := f.newApplierOn(t, "gauge-group", func() int64 { return fixedMs })

	f.appendEvent(t, makeCreateEvent(testTenant, "k-gauge", "node-gauge"))
	runInBackground(t, a)
	f.waitForIdempKey(t, testTenant, "k-gauge")
	waitFor(t, 2*time.Second, "batch to finalise", func() bool {
		return a.State().ApplyingSinceMilli == 0
	})

	// last-progress gauge is the commit timestamp in SECONDS.
	if got := testutil.ToFloat64(metrics.ApplierLastProgressCollector()); got != wantSec {
		t.Fatalf("last_progress gauge = %v, want %v (millis %d / 1000)", got, wantSec, fixedMs)
	}
	// in-progress gauge is cleared to 0 once the batch finalises (idle).
	if got := testutil.ToFloat64(metrics.ApplierApplyInProgressSinceCollector()); got != 0 {
		t.Fatalf("apply_in_progress_since gauge = %v, want 0 (idle)", got)
	}
}

// TestApplier_MultiTenantBatchProgress pins per-record vs per-batch counter
// semantics under the PARALLEL apply path (#653 coverage): a single poll
// batch spanning two tenants increments records_applied once per record but
// batches_applied exactly once, and the batch-level in-flight marker clears.
// NOT parallel — reads process-global counters.
func TestApplier_MultiTenantBatchProgress(t *testing.T) {
	f := newFixture(t)
	const tenantB = "tenant_b"
	if err := f.store.OpenTenant(context.Background(), tenantB); err != nil {
		t.Fatalf("OpenTenant(%s): %v", tenantB, err)
	}

	beforeRecords := testutil.ToFloat64(metrics.ApplierRecordsAppliedCollector())
	beforeBatches := testutil.ToFloat64(metrics.ApplierBatchesAppliedCollector())

	// Four records across two tenants, all appended before the applier
	// starts, so they arrive in ONE poll batch (single partition, batchSize
	// 32). Two distinct tenants exercises the cross-tenant parallel workers.
	f.appendEvent(t, makeCreateEvent(testTenant, "a1", "na1"))
	f.appendEvent(t, makeCreateEvent(testTenant, "a2", "na2"))
	f.appendEvent(t, makeCreateEvent(tenantB, "b1", "nb1"))
	f.appendEvent(t, makeCreateEvent(tenantB, "b2", "nb2"))

	f.runApplierUntilApplied(t) // default applier: MaxApplyConcurrency = GOMAXPROCS -> parallel
	f.waitForIdempKey(t, testTenant, "a2")
	f.waitForIdempKey(t, tenantB, "b2")
	waitFor(t, 2*time.Second, "batch to finalise", func() bool {
		return f.applier.State().ApplyingSinceMilli == 0
	})

	if got := testutil.ToFloat64(metrics.ApplierRecordsAppliedCollector()) - beforeRecords; got != 4 {
		t.Fatalf("records_applied delta = %v, want 4 (one per record)", got)
	}
	if got := testutil.ToFloat64(metrics.ApplierBatchesAppliedCollector()) - beforeBatches; got != 1 {
		t.Fatalf("batches_applied delta = %v, want 1 (once per batch, not per record/tenant)", got)
	}
}

// TestApplier_PoisonHaltReportsExited is the integration counterpart to the
// exit-window fix (#653 review): a real poison halt makes Readiness report
// "exited"/Stalled even with stall gating disabled (threshold 0), and the
// partial batch counts only its committed prefix (records_applied for the
// good record, batches_applied NOT incremented because the batch never fully
// committed). NOT parallel — reads process-global counters.
func TestApplier_PoisonHaltReportsExited(t *testing.T) {
	f := newFixture(t)
	beforeRecords := testutil.ToFloat64(metrics.ApplierRecordsAppliedCollector())
	beforeBatches := testutil.ToFloat64(metrics.ApplierBatchesAppliedCollector())

	f.appendEvent(t, makeCreateEvent(testTenant, "good", "n-good"))
	// Poison: a create_node op with no id (mirrors TestApplier_HaltsOnPoison).
	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:svc", IdempotencyKey: "poison",
		Ops: []map[string]any{{"op": "create_node", "type_id": 1}},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.applier.Run(ctx) }()

	var runErr error
	select {
	case runErr = <-done:
	case <-time.After(3 * time.Second):
		cancel()
		<-done
		t.Fatal("applier did not halt within 3s")
	}
	if runErr == nil {
		t.Fatal("expected a poison-halt error from Run")
	}

	// Even with stall gating disabled (threshold 0), an exited applier gates.
	rd := f.applier.Readiness(time.Now().UnixMilli(), 0)
	if rd.State != "exited" || !rd.Stalled {
		t.Fatalf("readiness after poison halt = %+v, want exited/stalled", rd)
	}

	if got := testutil.ToFloat64(metrics.ApplierRecordsAppliedCollector()) - beforeRecords; got != 1 {
		t.Fatalf("records_applied delta = %v, want 1 (only the good record's committed prefix)", got)
	}
	if got := testutil.ToFloat64(metrics.ApplierBatchesAppliedCollector()) - beforeBatches; got != 0 {
		t.Fatalf("batches_applied delta = %v, want 0 (halted batch never fully committed)", got)
	}
}

// TestApplier_ReplaySkipsCountAsProgress pins that a replaying applier
// (re-consuming already-applied records, which the idempotency probe returns
// as StatusSkipped) still advances the progress signal (#653 coverage) — so
// an applier draining a backlog after restart is NOT mistaken for stalled.
// NOT parallel — reads process-global counters.
func TestApplier_ReplaySkipsCountAsProgress(t *testing.T) {
	f := newFixture(t)
	f.appendEvent(t, makeCreateEvent(testTenant, "k-replay", "n-replay"))
	f.runApplierUntilApplied(t) // group testGroupID applies it once
	f.waitForIdempKey(t, testTenant, "k-replay")

	// A fresh consumer group re-consumes from offset 0; the record is already
	// in applied_events, so the in-txn idempotency probe returns Skipped.
	beforeRecords := testutil.ToFloat64(metrics.ApplierRecordsAppliedCollector())
	replay := f.newApplierOn(t, "replay-group", nil)
	runInBackground(t, replay)

	waitFor(t, 3*time.Second, "replay to skip-process the record", func() bool {
		return testutil.ToFloat64(metrics.ApplierRecordsAppliedCollector()) > beforeRecords
	})
	if got := testutil.ToFloat64(metrics.ApplierRecordsAppliedCollector()) - beforeRecords; got < 1 {
		t.Fatalf("records_applied delta = %v, want >=1 (a skipped replay still counts as forward progress)", got)
	}
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
