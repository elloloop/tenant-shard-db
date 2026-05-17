// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// Options configures a new Applier. Required fields: Store, Consumer,
// Topic, GroupID. Global is required for wal.ScopeGlobal records; an
// applier without Global treats those records as poison.
type Options struct {
	Store    *store.CanonicalStore
	Global   *globalstore.GlobalStore
	Consumer wal.Consumer
	Topic    string
	GroupID  string

	// BatchSize is the max records PollBatch returns per iteration.
	// Defaults to 32 (matches the Python applier batch_size default).
	BatchSize int

	// PollTimeout is how long PollBatch blocks waiting for records.
	// Defaults to 100ms.
	PollTimeout time.Duration

	// NowFn returns the current Unix-millis. Injectable for deterministic
	// tests. nil -> time.Now().UnixMilli.
	NowFn func() int64

	// FanoutHook is an optional post-commit hook for tests /
	// notification wiring. Best-effort; errors do not fail the apply.
	FanoutHook func(ctx context.Context, ev *Event, res *Result)

	// HaltOnPoison toggles halt-on-error behaviour. Defaults to true
	// (production parity with Python halt_on_error=True). Tests that
	// want to explore skip-and-continue paths can flip it; the contract
	// pinned by docs/go-port/shared/applier.md is that production must
	// halt.
	HaltOnPoison *bool
}

// Applier is the WAL consumer that materialises TransactionEvents into
// per-tenant SQLite. Single consumer goroutine per (topic, group_id)
// (Python parity); per-tenant pool deferred to a follow-up.
//
// Concurrency: Run is meant to be called once per Applier. Stop is
// safe to call from any goroutine and is idempotent. Replay is safe to
// call concurrently with Run if and only if it operates against a
// different tenant DB (the per-tenant write mutex from store enforces
// the rest).
type Applier struct {
	store    *store.CanonicalStore
	global   *globalstore.GlobalStore
	consumer wal.Consumer
	topic    string
	groupID  string

	batchSize    int
	pollTimeout  time.Duration
	nowFn        func() int64
	fanoutHook   func(ctx context.Context, ev *Event, res *Result)
	haltOnPoison bool

	// running is non-zero when the Run loop has started. Used by
	// Stop to skip the wait-for-loop-exit path when Run never started.
	running  atomic.Bool
	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// New constructs an Applier. Required: Store, Consumer, Topic, GroupID.
// Returns an error if any required dependency is nil/empty.
func New(opts Options) (*Applier, error) {
	if opts.Store == nil {
		return nil, errors.New("apply: Options.Store is required")
	}
	if opts.Consumer == nil {
		return nil, errors.New("apply: Options.Consumer is required")
	}
	if opts.Topic == "" {
		return nil, errors.New("apply: Options.Topic is required")
	}
	if opts.GroupID == "" {
		return nil, errors.New("apply: Options.GroupID is required")
	}
	batch := opts.BatchSize
	if batch <= 0 {
		batch = 32
	}
	pollTO := opts.PollTimeout
	if pollTO <= 0 {
		pollTO = 100 * time.Millisecond
	}
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = func() int64 { return time.Now().UnixMilli() }
	}
	halt := true
	if opts.HaltOnPoison != nil {
		halt = *opts.HaltOnPoison
	}
	return &Applier{
		store:        opts.Store,
		global:       opts.Global,
		consumer:     opts.Consumer,
		topic:        opts.Topic,
		groupID:      opts.GroupID,
		batchSize:    batch,
		pollTimeout:  pollTO,
		nowFn:        nowFn,
		fanoutHook:   opts.FanoutHook,
		haltOnPoison: halt,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}, nil
}

func (a *Applier) now() int64 { return a.nowFn() }

// Run is the consumer loop. It blocks until ctx is cancelled, Stop is
// called, or a poison event halts the consumer (when halt-on-poison is
// true). On halt-on-poison the offset is NOT advanced past the failing
// record so a restart re-attempts it.
func (a *Applier) Run(ctx context.Context) error {
	if !a.running.CompareAndSwap(false, true) {
		return ErrApplierClosed
	}
	defer close(a.doneCh)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-a.stopCh:
			return nil
		default:
		}

		records, err := a.consumer.PollBatch(ctx, a.topic, a.groupID, a.batchSize, a.pollTimeout)
		if err != nil {
			// ctx cancellation surfaces here as a wrapped error; treat
			// as graceful shutdown.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return fmt.Errorf("apply: poll batch: %w", err)
		}
		for i, rec := range records {
			res := a.applyRecord(ctx, rec)
			if res.Err != nil {
				if a.haltOnPoison {
					// Do NOT advance offset; the consumer group's
					// committed position stays at the last successful
					// record. Subsequent records (records[i+1:]) are
					// dropped on the floor — they'll be re-delivered
					// after restart.
					_ = i
					return res.Err
				}
				// skip-and-continue: still commit the offset (mirrors
				// Python halt_on_error=False) and move on.
			}
			if err := a.consumer.Commit(ctx, a.groupID, rec); err != nil {
				return fmt.Errorf("apply: commit offset: %w", err)
			}
			if res.Status == StatusApplied || res.Status == StatusSkipped || res.Status == StatusFailedPrecondition {
				if err := a.store.UpdateAppliedOffset(ctx, res.TenantID, rec.Position.Topic, rec.Position.Partition, rec.Position.Offset); err != nil {
					// Persisting the offset is best-effort; the in-
					// memory tracker has already been updated by the
					// in-txn UpdateAppliedOffsetTx call. Surface the
					// error so the supervisor can act.
					return fmt.Errorf("apply: persist offset: %w", err)
				}
			}
			// Best-effort post-commit fan-out + shared_index.
			a.fanout(ctx, decodeOrZero(rec), &res)
		}
	}
}

// Stop signals Run to return and waits for it to exit. Safe to call
// concurrently and idempotent.
func (a *Applier) Stop() {
	a.stopOnce.Do(func() { close(a.stopCh) })
	if a.running.Load() {
		<-a.doneCh
	}
}

// applyRecord is the per-record entry point. Decodes the WAL record,
// opens a BatchTxn, dispatches every op, and finalises with the
// idempotency probe (inside-txn check). Mirrors applier.py:1349-1392.
func (a *Applier) applyRecord(ctx context.Context, rec Record) Result {
	res := Result{Position: rec.Position}
	ev, err := wal.DecodeEvent(rec.Value)
	if err != nil {
		res.Status = StatusFailed
		res.Err = fmt.Errorf("%w: %s", ErrPoisonEvent, err.Error())
		return res
	}
	res.TenantID = ev.TenantID
	var applyErr error
	switch ev.Scope {
	case "", wal.ScopeTenant:
		applyErr = a.applyEvent(ctx, &ev, &res)
	case wal.ScopeGlobal:
		applyErr = a.applyGlobalEvent(ctx, &ev, &res)
	default:
		applyErr = fmt.Errorf("%w: unknown event scope %q", ErrPoisonEvent, ev.Scope)
	}
	if applyErr != nil {
		res.Status = StatusFailed
		res.Err = applyErr
		return res
	}
	return res
}

// applyEvent runs the per-event apply transaction. Idempotency check
// happens INSIDE the txn (per docs/go-port/shared/applier.md), before
// any mutation. The applied_events row is recorded in the same txn.
//
// GitHub issue #500: a conditional UpdateNodeOp that misses its
// precondition aborts the batch, memoizes the failure to
// applied_events (status=FAILED_PRECONDITION) in a SEPARATE
// transaction, and returns without halting. The WAL offset still
// advances — the failure is a deterministic outcome of replaying the
// same event against the same materialised state, so a fresh applier
// rebuilding from scratch will produce the same outcome.
func (a *Applier) applyEvent(ctx context.Context, ev *Event, res *Result) error {
	if err := a.store.OpenTenant(ctx, ev.TenantID); err != nil {
		return fmt.Errorf("apply: open tenant %q: %w", ev.TenantID, err)
	}
	tx, err := a.store.BeginBatch(ctx, ev.TenantID)
	if err != nil {
		return fmt.Errorf("apply: begin batch: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// In-txn idempotency probe. Reads the cached status + failure_json
	// so a retry with the same idem key after a memoized
	// FAILED_PRECONDITION replays the same typed failure WITHOUT
	// re-evaluating the predicate against possibly-changed state. See
	// the GitHub-issue-#500 idempotency contract.
	conn := tx.Conn()
	var cachedStatus string
	var cachedFailure sql.NullString
	row := conn.QueryRowContext(ctx,
		`SELECT status, failure_json FROM applied_events WHERE tenant_id = ? AND idempotency_key = ?`,
		ev.TenantID, ev.IdempotencyKey,
	)
	if err := row.Scan(&cachedStatus, &cachedFailure); err == nil {
		// Already recorded. Commit the no-op txn (the probe holds the
		// RESERVED lock; releasing via COMMIT is cheaper than ROLLBACK).
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("apply: commit no-op: %w", err)
		}
		committed = true
		// Replay the memoized failure on a cached precondition miss
		// so callers see the same typed error on retry.
		if cachedStatus == store.IdempotencyStatusFailedPrecondition {
			res.Status = StatusFailedPrecondition
			if cachedFailure.Valid && cachedFailure.String != "" {
				var pf PreconditionFailure
				if jerr := json.Unmarshal([]byte(cachedFailure.String), &pf); jerr == nil {
					res.Precondition = &pf
				}
			}
			return nil
		}
		res.Status = StatusSkipped
		return nil
	} else if !isNoRows(err) {
		return fmt.Errorf("apply: idempotency probe: %w", err)
	}

	// Per-event alias map (per CLAUDE.md / spec; never global).
	aliases := nodeAliasMap{}

	for i, op := range ev.Ops {
		if op == nil {
			continue
		}
		if err := a.dispatch(ctx, tx, ev, op, aliases, res, i); err != nil {
			// CAS miss: roll back the batch txn, memoize the failure
			// in a fresh txn, advance the offset (without halting).
			// This is the GitHub-issue-#500 hard-fail semantics — no
			// op in the batch commits, but the WAL offset advances
			// because the outcome is deterministic on replay.
			if pf := AsPreconditionFailure(err); pf != nil {
				_ = tx.Rollback()
				committed = true // suppress deferred rollback
				return a.memoizePreconditionFailure(ctx, ev, res, pf)
			}
			return err
		}
	}

	// Record the applied_events row inside the same txn.
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO applied_events (tenant_id, idempotency_key, stream_pos, applied_at, status)
		VALUES (?, ?, ?, ?, ?)`,
		ev.TenantID, ev.IdempotencyKey, res.Position.String(), a.now(), store.IdempotencyStatusApplied,
	); err != nil {
		return fmt.Errorf("apply: record applied_events: %w", err)
	}

	// In-txn offset advance so the receipt observer wakes immediately.
	if err := a.store.UpdateAppliedOffsetTx(ctx, tx, res.Position.Topic, res.Position.Partition, res.Position.Offset); err != nil {
		return fmt.Errorf("apply: in-txn offset: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("apply: commit: %w", err)
	}
	committed = true
	res.Status = StatusApplied
	return nil
}

// memoizePreconditionFailure persists the FAILED_PRECONDITION row in a
// fresh write-txn and returns nil so the consumer loop advances the
// offset (CAS miss is a "successful" deterministic outcome, just not a
// commit). The applier's offset-advance happens in
// applyRecord/Run after applyEvent returns; we update the in-memory
// offset tracker here too so receipt waiters wake on
// store.WaitForOffset.
func (a *Applier) memoizePreconditionFailure(ctx context.Context, ev *Event, res *Result, pf *PreconditionFailure) error {
	res.Status = StatusFailedPrecondition
	res.Precondition = pf
	res.TenantID = ev.TenantID

	failureJSON, err := json.Marshal(pf)
	if err != nil {
		// If we can't encode the failure detail we still want to
		// memoize the status so retries are deterministic. Fall
		// through with an empty failureJSON.
		failureJSON = []byte("")
	}
	if err := a.store.RecordIdempotencyFailure(ctx, ev.TenantID, ev.IdempotencyKey, res.Position.String(), string(failureJSON)); err != nil {
		// Idempotency-key collision means another writer (a parallel
		// retry that won the race to memoize) recorded the failure
		// first — treat that as a successful memoization.
		if !errors.Is(err, store.ErrIdempotencyViolation) {
			return fmt.Errorf("apply: memoize precondition failure: %w", err)
		}
	}
	// Advance the persisted per-tenant offset so receipt observers
	// (store.WaitForOffset) wake on the failed batch.
	if err := a.store.UpdateAppliedOffset(ctx, ev.TenantID, res.Position.Topic, res.Position.Partition, res.Position.Offset); err != nil {
		return fmt.Errorf("apply: precondition-failure offset: %w", err)
	}
	return nil
}

// dispatch routes one op to its handler. Unknown op types fail closed
// (halt-on-poison) so a typo in a handler that raced ahead of the
// applier surfaces immediately rather than silently dropping data —
// the lesson the DelegateAccess bug taught us.
//
// opIndex is the zero-based position of the op in the enclosing event;
// only applyUpdateNode currently consumes it (to surface
// PreconditionFailure.OpIndex on a CAS miss) but it is passed to every
// op for forward-compat — future structured errors will benefit.
func (a *Applier) dispatch(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any, aliases nodeAliasMap, res *Result, opIndex int) error {
	switch opTypeOf(op) {
	case OpCreateNode:
		return a.applyCreateNode(ctx, tx, ev, op, aliases, res)
	case OpUpdateNode:
		return a.applyUpdateNode(ctx, tx, ev, op, aliases, opIndex)
	case OpDeleteNode:
		return a.applyDeleteNode(ctx, tx, ev, op, aliases, res)
	case OpDeleteWhere:
		return a.applyDeleteWhere(ctx, tx, ev, op, res)
	case OpCreateEdge:
		return a.applyCreateEdge(ctx, tx, ev, op, aliases, res)
	case OpDeleteEdge:
		return a.applyDeleteEdge(ctx, tx, ev, op, aliases)
	case OpAdminTransferContent:
		return a.applyAdminTransferContent(ctx, tx, ev, op)
	case OpAdminRevokeAccess:
		return a.applyAdminRevokeAccess(ctx, tx, ev, op)
	case OpShareNode:
		return a.applyShareNode(ctx, tx, ev, op, res)
	case OpRevokeAccess:
		return a.applyRevokeAccess(ctx, tx, ev, op, res)
	case OpDelegateAccess:
		return a.applyDelegateAccess(ctx, tx, ev, op, res)
	case OpTransferOwnership:
		return a.applyTransferOwnership(ctx, tx, ev, op)
	case OpAddGroupMember:
		return a.applyAddGroupMember(ctx, tx, ev, op)
	case OpRemoveGroupMember:
		return a.applyRemoveGroupMember(ctx, tx, ev, op)
	case OpSharedIndexCleanup:
		return a.applySharedIndexCleanup(ev, op, res)
	case OpSetLegalHold:
		return a.applySetLegalHold(ctx, ev, op)
	case OpAddTenantMember:
		return a.applyAddTenantMember(ctx, ev, op)
	case OpRemoveTenantMember:
		return a.applyRemoveTenantMember(ctx, ev, op)
	case OpChangeMemberRole:
		return a.applyChangeMemberRole(ctx, ev, op)
	case OpAccessTransferred:
		return a.applyAccessTransferred(ctx, ev, op)
	case OpAccessRevoked:
		return a.applyAccessRevoked(ctx, ev, op)
	default:
		return fmt.Errorf("%w: %q", ErrUnknownOpType, opTypeOf(op))
	}
}

func (a *Applier) applyAccessTransferred(ctx context.Context, ev *Event, op map[string]any) error {
	if a.global == nil {
		return fmt.Errorf("%w: access_transferred without globalstore", ErrPoisonEvent)
	}
	tenantID := stringField(op, "tenant_id")
	if tenantID == "" {
		tenantID = ev.TenantID
	}
	return a.global.ApplyAccessTransferred(ctx, tenantID, stringField(op, "to_user"), int64Field(op, "joined_at"))
}

func (a *Applier) applyAccessRevoked(ctx context.Context, ev *Event, op map[string]any) error {
	if a.global == nil {
		return fmt.Errorf("%w: access_revoked without globalstore", ErrPoisonEvent)
	}
	tenantID := stringField(op, "tenant_id")
	if tenantID == "" {
		tenantID = ev.TenantID
	}
	return a.global.ApplyAccessRevoked(ctx, tenantID, stringField(op, "user_id"))
}

// Replay drives the applier from a specific WAL offset for one tenant.
// Used by the recovery harness (Tier 2 — KAFKA_WAL replay from
// snapshot's last_stream_pos forward). For it shares the same
// consumer/store wiring as Run; the caller is responsible for ensuring
// the consumer group's stored offset is positioned correctly before
// invoking.
//
// For convenience (and to keep the test surface honest), Replay returns
// after a single PollBatch returns 0 records — the in-memory WAL
// signals "drained" by returning an empty slice with nil error.
func (a *Applier) Replay(ctx context.Context, tenantID string, fromPos int64) error {
	// fromPos is reserved for future use (resetting the consumer-group
	// offset before driving). The in-memory WAL we ship with today
	// derives this from a fresh group_id; production backends will
	// implement a Seek primitive.
	_ = fromPos
	for {
		records, err := a.consumer.PollBatch(ctx, a.topic, a.groupID, a.batchSize, 50*time.Millisecond)
		if err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}
		for _, rec := range records {
			res := a.applyRecord(ctx, rec)
			if tenantID != "" && res.TenantID != tenantID {
				// Replay is per-tenant; commit the offset (we still
				// need to advance the consumer group) but skip the
				// post-commit work.
				_ = a.consumer.Commit(ctx, a.groupID, rec)
				continue
			}
			if res.Err != nil {
				return res.Err
			}
			if err := a.consumer.Commit(ctx, a.groupID, rec); err != nil {
				return fmt.Errorf("apply: replay commit offset: %w", err)
			}
			a.fanout(ctx, decodeOrZero(rec), &res)
		}
	}
}

// decodeOrZero returns a decoded Event for fan-out, or a zero-value
// Event when decode fails (the apply path already failed; fan-out is
// best-effort).
func decodeOrZero(rec Record) *Event {
	ev, err := wal.DecodeEvent(rec.Value)
	if err != nil {
		return &Event{}
	}
	return &ev
}
