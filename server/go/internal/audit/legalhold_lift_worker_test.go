// SPDX-License-Identifier: AGPL-3.0-only

// Durability + retry + metrics tests for the legal-hold-lift worker
// (EPIC #511 Gap 1, ADR-015). These prove the rework's core promise:
// the lift survives a server restart (it runs purely from the persisted
// queue, NOT a request-scoped goroutine) and a transient S3 error is
// retried on the next tick. The shared-object / idempotent / retention
// correctness lives in legalhold_lift_test.go and is unchanged.

package audit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
)

// fakeLiftQueue is an in-memory stand-in for the durable
// globalstore-backed queue. The worker reads/dequeues/counts purely
// through this — a fresh instance with a pre-populated queue models a
// server that crashed AFTER the release was committed and restarted with
// no in-memory state.
type fakeLiftQueue struct {
	pending  map[string]int64 // tenant_id -> enqueued_at
	held     map[string]bool  // tenant_id -> still under legal hold
	getErr   error            // injected GetLegalHoldLiftQueue error
	getCalls int
}

func newFakeLiftQueue() *fakeLiftQueue {
	return &fakeLiftQueue{pending: map[string]int64{}, held: map[string]bool{}}
}

func (q *fakeLiftQueue) GetLegalHoldLiftQueue(context.Context) ([]LiftQueueEntry, error) {
	q.getCalls++
	if q.getErr != nil {
		return nil, q.getErr
	}
	out := make([]LiftQueueEntry, 0, len(q.pending))
	for tid, at := range q.pending {
		out = append(out, LiftQueueEntry{TenantID: tid, EnqueuedAt: at})
	}
	return out, nil
}

func (q *fakeLiftQueue) DequeueLegalHoldLift(_ context.Context, tenantID string) (bool, error) {
	_, ok := q.pending[tenantID]
	delete(q.pending, tenantID)
	return ok, nil
}

func (q *fakeLiftQueue) CountLegalHoldLiftQueue(context.Context) (int, error) {
	return len(q.pending), nil
}

func (q *fakeLiftQueue) IsLegalHoldSet(_ context.Context, tenantID string) (bool, error) {
	return q.held[tenantID], nil
}

// liftMetric reads one legal-hold-lift collector value by metric name.
func liftMetric(t *testing.T, name string) float64 {
	t.Helper()
	for _, c := range metrics.LegalHoldLiftCollectors() {
		ch := make(chan *prometheus.Desc, 1)
		c.Describe(ch)
		close(ch)
		matched := false
		for d := range ch {
			if d != nil && strings.Contains(d.String(), name) {
				matched = true
			}
		}
		if matched {
			return testutil.ToFloat64(c)
		}
	}
	t.Fatalf("metric %q not found in LegalHoldLiftCollectors", name)
	return 0
}

// TestLiftWorker_DurableRestart proves the lift survives a server
// restart. The queue is pre-populated (modelling a release that was
// committed durably) and the worker is constructed FRESH with zero
// in-memory state — no request-scoped goroutine, no detached sweep. The
// worker completes the lift purely from the persisted queue + live S3.
// The SetLegalHold OFF path no longer spawns a goroutine at all (asserted
// structurally below by TestLiftWorker_NoGoroutineInOFFPath).
func TestLiftWorker_DurableRestart(t *testing.T) {
	ctx := context.Background()

	// Live S3 still carries the held object from before the "crash".
	s3 := newLiftFakeS3()
	s3.seed("wal/entdb-wal/0/0-1.jsonl.gz", gzJSONL(t, "acme"), true)
	s3.seed("wal/entdb-wal/0/2-3.jsonl.gz", gzJSONL(t, "acme"), true)
	store := NewS3ObjectLockStore(s3, "audit-bucket", "")

	// Durable queue as it would be AFTER the release committed but
	// BEFORE any sweep ran (server then crashed/restarted).
	q := newFakeLiftQueue()
	q.pending["acme"] = 1700000000

	// Fresh worker — the ONLY state it has is the persisted queue.
	w, err := NewLiftWorker(LiftWorkerOptions{Queue: q, Lifter: store})
	if err != nil {
		t.Fatalf("NewLiftWorker: %v", err)
	}

	sum, err := w.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if sum.TenantsScanned != 1 || sum.TenantsCompleted != 1 || sum.TenantsRetained != 0 {
		t.Fatalf("summary = %+v; want 1 scanned/1 completed/0 retained", sum)
	}
	// Every previously-held object's hold is now OFF, purely from the
	// persisted queue + live S3 (no in-process trigger).
	for k, o := range s3.objects {
		if o.legalHoldOn {
			t.Fatalf("object %q still ON after durable-restart lift", k)
		}
	}
	assertRetentionIntact(t, s3)
	// Row removed only after the full success.
	if _, ok := q.pending["acme"]; ok {
		t.Fatal("queue row not removed after successful lift")
	}
}

// TestLiftWorker_RetryAfterTransientS3Error proves retry/resume: a
// transient S3 List error on the first tick leaves the queue row in
// place (no dequeue, error metric bumped); the next tick succeeds and
// removes the row. This is the durability guarantee the old detached
// goroutine lacked.
func TestLiftWorker_RetryAfterTransientS3Error(t *testing.T) {
	ctx := context.Background()

	s3 := newLiftFakeS3()
	s3.seed("wal/entdb-wal/0/0-1.jsonl.gz", gzJSONL(t, "acme"), true)
	store := NewS3ObjectLockStore(s3, "audit-bucket", "")

	q := newFakeLiftQueue()
	q.pending["acme"] = 1700000000

	w, err := NewLiftWorker(LiftWorkerOptions{Queue: q, Lifter: store})
	if err != nil {
		t.Fatalf("NewLiftWorker: %v", err)
	}

	errsBefore := liftMetric(t, "entdb_legal_hold_lift_errors_total")

	// Tick 1: transient List failure.
	s3.listErr = errors.New("s3: throttled (transient)")
	sum1, err := w.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce tick1: %v", err)
	}
	if sum1.TenantsCompleted != 0 || sum1.TenantsRetained != 1 {
		t.Fatalf("tick1 summary = %+v; want 0 completed/1 retained", sum1)
	}
	if _, ok := q.pending["acme"]; !ok {
		t.Fatal("queue row removed despite a failed sweep — retry would be impossible")
	}
	if s3.objects["wal/entdb-wal/0/0-1.jsonl.gz"].legalHoldOn != true {
		t.Fatal("object hold cleared despite a failed list — must stay ON for retry")
	}
	if got := liftMetric(t, "entdb_legal_hold_lift_errors_total"); got != errsBefore+1 {
		t.Fatalf("errors_total = %v; want %v after one failed tick", got, errsBefore+1)
	}

	// Tick 2: S3 recovers, the SAME persisted row drives a successful sweep.
	s3.listErr = nil
	sum2, err := w.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce tick2: %v", err)
	}
	if sum2.TenantsCompleted != 1 || sum2.TenantsRetained != 0 {
		t.Fatalf("tick2 summary = %+v; want 1 completed/0 retained", sum2)
	}
	if _, ok := q.pending["acme"]; ok {
		t.Fatal("queue row not removed after the retry succeeded")
	}
	if s3.objects["wal/entdb-wal/0/0-1.jsonl.gz"].legalHoldOn {
		t.Fatal("object hold still ON after the successful retry tick")
	}
	assertRetentionIntact(t, s3)
}

// TestLiftWorker_Metrics proves the pending gauge tracks the durable
// queue depth and the completed counter advances per fully-swept tenant.
func TestLiftWorker_Metrics(t *testing.T) {
	ctx := context.Background()

	s3 := newLiftFakeS3()
	s3.seed("wal/entdb-wal/0/0-1.jsonl.gz", gzJSONL(t, "t1"), true)
	s3.seed("wal/entdb-wal/0/2-3.jsonl.gz", gzJSONL(t, "t2"), true)
	store := NewS3ObjectLockStore(s3, "audit-bucket", "")

	q := newFakeLiftQueue()
	q.pending["t1"] = 1700000001
	q.pending["t2"] = 1700000002

	w, err := NewLiftWorker(LiftWorkerOptions{Queue: q, Lifter: store})
	if err != nil {
		t.Fatalf("NewLiftWorker: %v", err)
	}

	completedBefore := liftMetric(t, "entdb_legal_hold_lift_completed_total")

	if _, err := w.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if got := liftMetric(t, "entdb_legal_hold_lift_pending"); got != 0 {
		t.Fatalf("pending gauge = %v; want 0 after both tenants swept", got)
	}
	if got := liftMetric(t, "entdb_legal_hold_lift_completed_total"); got != completedBefore+2 {
		t.Fatalf("completed_total = %v; want %v (+2)", got, completedBefore+2)
	}
}

// TestLiftWorker_SharedObjectCoTenantStillHeld proves the worker
// preserves the existing shared-object safety: a co-tenant still under
// legal hold (via the queue-backed IsLegalHoldSet) keeps the shared
// object ON; the released tenant's row is still removed (its own sweep
// completed without error — nothing more it can do until the co-tenant
// is released too).
func TestLiftWorker_SharedObjectCoTenantStillHeld(t *testing.T) {
	ctx := context.Background()

	s3 := newLiftFakeS3()
	// One archive object carries BOTH tenants (per-partition key).
	s3.seed("wal/entdb-wal/0/0-1.jsonl.gz", gzJSONL(t, "released", "cotenant"), true)
	store := NewS3ObjectLockStore(s3, "audit-bucket", "")

	q := newFakeLiftQueue()
	q.pending["released"] = 1700000000
	q.held["cotenant"] = true // still under hold

	w, err := NewLiftWorker(LiftWorkerOptions{Queue: q, Lifter: store})
	if err != nil {
		t.Fatalf("NewLiftWorker: %v", err)
	}
	if _, err := w.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !s3.objects["wal/entdb-wal/0/0-1.jsonl.gz"].legalHoldOn {
		t.Fatal("shared object lifted while a co-tenant is still held")
	}
	assertRetentionIntact(t, s3)
}

// TestLiftWorker_GetQueueErrorIsTickError proves a queue-read failure
// surfaces from RunOnce (so Run logs + retries next tick) and does NOT
// silently drop pending lifts.
func TestLiftWorker_GetQueueErrorIsTickError(t *testing.T) {
	ctx := context.Background()
	s3 := newLiftFakeS3()
	store := NewS3ObjectLockStore(s3, "audit-bucket", "")
	q := newFakeLiftQueue()
	q.getErr = errors.New("globalstore: db locked")

	w, err := NewLiftWorker(LiftWorkerOptions{Queue: q, Lifter: store})
	if err != nil {
		t.Fatalf("NewLiftWorker: %v", err)
	}
	if _, err := w.RunOnce(ctx); err == nil {
		t.Fatal("RunOnce returned nil on a queue-read failure; want error")
	}
}

// TestLiftWorker_Run_StopsOnContextCancel proves Run honours ctx
// cancellation (graceful shutdown) and a transient RunOnce error does
// not kill the loop.
func TestLiftWorker_Run_StopsOnContextCancel(t *testing.T) {
	s3 := newLiftFakeS3()
	store := NewS3ObjectLockStore(s3, "audit-bucket", "")
	q := newFakeLiftQueue()

	w, err := NewLiftWorker(LiftWorkerOptions{Queue: q, Lifter: store})
	if err != nil {
		t.Fatalf("NewLiftWorker: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx, 10*time.Millisecond) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case rerr := <-done:
		if !errors.Is(rerr, context.Canceled) {
			t.Fatalf("Run returned %v; want context.Canceled", rerr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop within 2s of ctx cancel")
	}
}

// TestNewLiftWorker_RequiresDeps pins the constructor guards.
func TestNewLiftWorker_RequiresDeps(t *testing.T) {
	s3 := newLiftFakeS3()
	store := NewS3ObjectLockStore(s3, "b", "")
	if _, err := NewLiftWorker(LiftWorkerOptions{Lifter: store}); err == nil {
		t.Fatal("NewLiftWorker without Queue: want error")
	}
	if _, err := NewLiftWorker(LiftWorkerOptions{Queue: newFakeLiftQueue()}); err == nil {
		t.Fatal("NewLiftWorker without Lifter: want error")
	}
}
