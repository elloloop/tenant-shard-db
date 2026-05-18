// SPDX-License-Identifier: AGPL-3.0-only

// Durable legal-hold-lift worker (EPIC #511 Gap 1, ADR-015).
//
// Why this exists: SetLegalHold OFF used to launch the archive sweep as
// a detached fire-and-forget goroutine off the RPC path. That was NOT
// crash-durable — a server restart / S3 outage / >timeout left a
// released tenant's objects stuck LegalHold=ON with no retry and no
// signal, which silently breaks GDPR right-to-erasure-after-release. For
// a compliance feature that is unacceptable.
//
// The trigger is now durable: ApplyLegalHoldSet (the WAL-driven global
// apply step that clears the legal_holds row + flips
// tenant_registry.status) ALSO upserts a legal_hold_lift_queue row in
// the SAME transaction. This worker — mirroring the GDPR
// deletion_queue worker (internal/gdpr/processor.go) — drains that queue
// on an interval, running the existing idempotent/resumable paginated
// sweep for each queued tenant. On full success the row is deleted; on
// ANY failure / partial / per-run timeout the row is left for the next
// tick (retry/resume). The per-run timeout is therefore safe: the row
// persists and the next tick continues exactly where state left off
// (every sweep decision is derived from live S3 + globalstore state, so
// there is no progress cursor to corrupt).
//
// Correctness of the sweep itself is unchanged (per-partition
// shared-object safety, COMPLIANCE retention never touched, GDPR
// precedence, MinIO Content-MD5) — only the trigger + execution model
// changed (detached goroutine -> durable queue + retrying worker).

package audit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
)

// LiftQueueStore is the slice of globalstore the lift worker needs. Kept
// as an interface so the worker is unit-testable with an in-memory fake
// and so this package does not hard-depend on globalstore.
type LiftQueueStore interface {
	// GetLegalHoldLiftQueue returns every pending-lift entry, oldest first.
	GetLegalHoldLiftQueue(ctx context.Context) ([]LiftQueueEntry, error)
	// DequeueLegalHoldLift removes a tenant's pending-lift row after a
	// fully-successful sweep.
	DequeueLegalHoldLift(ctx context.Context, tenantID string) (bool, error)
	// CountLegalHoldLiftQueue returns the current queue depth (the
	// entdb_legal_hold_lift_pending gauge source).
	CountLegalHoldLiftQueue(ctx context.Context) (int, error)
	// IsLegalHoldSet backs the per-object co-tenant guard: a shared
	// archive object keeps its hold while ANY tenant it carries is still
	// under legal hold.
	IsLegalHoldSet(ctx context.Context, tenantID string) (bool, error)
}

// LiftQueueEntry is one queued tenant whose archive legal hold must be
// lifted. Mirrors globalstore.LegalHoldLiftQueueEntry without the
// package dependency.
type LiftQueueEntry struct {
	TenantID   string
	EnqueuedAt int64
}

// LegalHoldLifter is the sweep seam (implemented by *S3ObjectLockStore).
// Kept as an interface so the worker is unit-testable with a fake S3.
type LegalHoldLifter interface {
	LiftLegalHoldForReleasedTenant(ctx context.Context, tenantID string, stillHeld TenantHeldFunc) (LiftSummary, error)
}

// LiftWorkerOptions configures a LiftWorker.
type LiftWorkerOptions struct {
	// Queue is the durable globalstore-backed pending-lift queue.
	Queue LiftQueueStore
	// Lifter runs the paginated S3 sweep for a released tenant.
	Lifter LegalHoldLifter
	// RunTimeout bounds a single tenant's sweep. Zero => 30m. A
	// timed-out sweep simply leaves the queue row for the next tick (the
	// sweep is resumable), so the bound never corrupts state.
	RunTimeout time.Duration
}

// liftRunTimeoutDefault bounds one tenant's sweep per tick. The sweep is
// idempotent/resumable and the queue row survives a timeout, so the next
// tick resumes — the bound only stops a single stuck S3 endpoint from
// blocking the whole queue forever.
const liftRunTimeoutDefault = 30 * time.Minute

// LiftWorker drains the durable legal_hold_lift_queue: for each queued
// tenant it runs the resumable archive sweep, deletes the row only on
// full success, and publishes the pending/completed/error metrics.
type LiftWorker struct {
	queue      LiftQueueStore
	lifter     LegalHoldLifter
	runTimeout time.Duration
}

// NewLiftWorker constructs a LiftWorker. Queue + Lifter are required.
func NewLiftWorker(opts LiftWorkerOptions) (*LiftWorker, error) {
	if opts.Queue == nil {
		return nil, errors.New("audit: lift worker requires a queue store")
	}
	if opts.Lifter == nil {
		return nil, errors.New("audit: lift worker requires a lifter")
	}
	rt := opts.RunTimeout
	if rt <= 0 {
		rt = liftRunTimeoutDefault
	}
	return &LiftWorker{queue: opts.Queue, lifter: opts.Lifter, runTimeout: rt}, nil
}

// Run drains the queue immediately and then every interval until ctx is
// cancelled. Mirrors gdpr.Processor.Run. A RunOnce error is logged and
// the loop continues (a transient S3 outage must not kill the worker —
// the rows persist and the next tick retries); only ctx cancellation
// stops the loop.
func (w *LiftWorker) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("audit: lift worker interval must be positive")
	}
	for {
		if _, err := w.RunOnce(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.ErrorContext(ctx, "audit: legal-hold lift worker tick failed (queue retained; retrying next tick)",
				"error", err)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// LiftWorkerSummary reports one RunOnce pass.
type LiftWorkerSummary struct {
	// TenantsScanned is the number of queued tenants this tick attempted.
	TenantsScanned int
	// TenantsCompleted is the number whose sweep fully succeeded and was
	// dequeued.
	TenantsCompleted int
	// TenantsRetained is the number left in the queue (failure / partial
	// / timeout) for the next tick.
	TenantsRetained int
}

// RunOnce processes every queued tenant once. A per-tenant failure does
// NOT abort the pass — every queued tenant gets a chance and any failing
// row is simply retained for the next tick. The pending gauge is
// refreshed at entry and exit so a scrape between ticks is accurate.
func (w *LiftWorker) RunOnce(ctx context.Context) (*LiftWorkerSummary, error) {
	w.publishPending(ctx)
	entries, err := w.queue.GetLegalHoldLiftQueue(ctx)
	if err != nil {
		return nil, fmt.Errorf("audit: read legal-hold lift queue: %w", err)
	}
	summary := &LiftWorkerSummary{}
	stillHeld := func(c context.Context, tid string) (bool, error) {
		return w.queue.IsLegalHoldSet(c, tid)
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		summary.TenantsScanned++
		if w.processOne(ctx, e.TenantID, stillHeld) {
			summary.TenantsCompleted++
		} else {
			summary.TenantsRetained++
		}
	}
	w.publishPending(ctx)
	return summary, nil
}

// processOne runs the sweep for one tenant and dequeues it only on full
// success. Returns true iff the row was dequeued. Any failure (sweep
// error, per-run timeout, dequeue error) leaves the row for the next
// tick and bumps the error metric.
func (w *LiftWorker) processOne(parent context.Context, tenantID string, stillHeld TenantHeldFunc) bool {
	ctx, cancel := context.WithTimeout(parent, w.runTimeout)
	defer cancel()

	sum, err := w.lifter.LiftLegalHoldForReleasedTenant(ctx, tenantID, stillHeld)
	if err != nil {
		metrics.IncLegalHoldLiftErrors()
		slog.ErrorContext(ctx, "audit: legal-hold lift sweep failed (queue row retained for retry)",
			"tenant_id", tenantID, "error", err,
			"scanned", sum.Scanned, "lifted", sum.Lifted)
		return false
	}
	ok, derr := w.queue.DequeueLegalHoldLift(parent, tenantID)
	if derr != nil {
		// The sweep succeeded but the row could not be removed; leave it
		// — a re-run of the (idempotent) sweep next tick is harmless and
		// the row will be cleared then.
		metrics.IncLegalHoldLiftErrors()
		slog.ErrorContext(ctx, "audit: legal-hold lift dequeue failed (will retry; sweep is idempotent)",
			"tenant_id", tenantID, "error", derr)
		return false
	}
	metrics.IncLegalHoldLiftCompleted()
	slog.InfoContext(ctx, "audit: legal-hold lift sweep complete",
		"tenant_id", tenantID, "dequeued", ok,
		"scanned", sum.Scanned, "held_scanned", sum.HeldScanned,
		"lifted", sum.Lifted, "skipped_still_held", sum.SkippedStillHeld)
	return true
}

// publishPending refreshes the entdb_legal_hold_lift_pending gauge from
// the durable queue depth. A count error is logged but never aborts the
// tick — the gauge is observability, not correctness.
func (w *LiftWorker) publishPending(ctx context.Context) {
	n, err := w.queue.CountLegalHoldLiftQueue(ctx)
	if err != nil {
		slog.WarnContext(ctx, "audit: count legal-hold lift queue for metric failed",
			"error", err)
		return
	}
	metrics.SetLegalHoldLiftPending(n)
}
