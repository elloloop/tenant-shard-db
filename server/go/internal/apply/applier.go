// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
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
	// Defaults to 32.
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
	// (production must halt). Tests that want to explore
	// skip-and-continue paths can flip it; the contract is pinned by
	// docs/go-port/shared/applier.md.
	HaltOnPoison *bool

	// MaxApplyConcurrency caps how many distinct tenants' records the
	// Run loop applies in parallel within a single poll batch (#140
	// PERF-4). Records for the SAME tenant are always applied serially
	// in offset order by a single worker, and offsets are committed in
	// strict batch order along the contiguous fully-applied prefix, so
	// this knob never weakens per-tenant ordering or the
	// single-writer-per-tenant invariant (ADR-016) — it only bounds the
	// goroutine fan-out across independent tenant SQLite files.
	//
	// 0 (default) -> runtime.GOMAXPROCS(0). 1 -> fully serial (the
	// pre-#140 behaviour, kept as an escape hatch). Values >0 cap the
	// worker pool; the cap never exceeds the number of distinct tenants
	// in the batch.
	MaxApplyConcurrency int
}

// Applier is the WAL consumer that materialises TransactionEvents into
// per-tenant SQLite. One consumer per (topic, group_id): it polls the
// WAL serially and commits offsets serially in batch order. Within a
// poll batch it applies records for DISTINCT tenants in parallel (#140
// PERF-4) — see processBatch for the invariant-preservation argument.
// This does not change the consumer-group model ADR-016 describes
// ("one consumer per partition; events apply serially within a
// partition"): the consumer-group offset is still committed strictly
// in offset order along the contiguous fully-applied prefix, and
// records for any single tenant still apply serially in offset order.
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
	maxApplyConc int

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
	maxConc := opts.MaxApplyConcurrency
	if maxConc <= 0 {
		maxConc = runtime.GOMAXPROCS(0)
	}
	if maxConc < 1 {
		maxConc = 1
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
		maxApplyConc: maxConc,
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
		if len(records) == 0 {
			continue
		}
		if err := a.processBatch(ctx, records); err != nil {
			return err
		}
	}
}

// processBatch applies one poll batch with cross-tenant parallelism
// (#140 PERF-4) while preserving every consistency invariant the
// single-threaded loop guaranteed:
//
//   - Per-tenant ordering: records are partitioned by tenant_id and
//     each tenant's records are applied SERIALLY in batch (== offset)
//     order by exactly one worker goroutine. Two records for the same
//     tenant never run concurrently.
//   - Single-writer-per-tenant (ADR-016): distinct tenants own
//     independent SQLite files; workers only ever touch their own
//     tenant. The store's per-tenant write mutex remains the backstop
//     but is never contended by the applier itself.
//   - Monotonic, gap-free offset commit: results are FINALISED
//     (consumer-group commit + persisted per-tenant offset + fan-out)
//     strictly in original batch order, stopping at the first
//     halt-on-poison failure. Because PollBatch returns records in
//     offset order within each partition, finalising in batch order
//     advances each partition's committed offset only along its
//     contiguous fully-applied prefix — a later tenant finishing first
//     never advances an offset past an earlier un-applied record.
//
// On a halt-on-poison failure, workers for other tenants may already
// have applied later records. Their SQLite writes are durable but
// their offsets are NOT committed, so a restart re-delivers them and
// the in-txn idempotency probe SKIPs them — identical to the
// pre-#140 "dropped on the floor, re-delivered after restart"
// contract, just generalised across tenants.
func (a *Applier) processBatch(ctx context.Context, records []Record) error {
	n := len(records)

	// Decode the tenant routing key for every record up front, in
	// order. Decode failures are deferred to applyRecord (which maps
	// them to ErrPoisonEvent) so the halt semantics are unchanged; we
	// route those records to a stable synthetic key so they still
	// apply serially relative to each other.
	results := make([]Result, n)
	type recRef struct {
		idx int
		rec Record
	}
	groups := make(map[string][]recRef)
	order := make([]string, 0, 8) // distinct tenants, first-seen order
	for i, rec := range records {
		key := routeKey(rec)
		if _, seen := groups[key]; !seen {
			order = append(order, key)
		}
		groups[key] = append(groups[key], recRef{idx: i, rec: rec})
	}

	// applyChain applies one tenant's records serially in batch (offset)
	// order. Under halt-on-poison it STOPS at the first failing record
	// in this tenant's own stream — preserving the exact pre-#140
	// per-tenant semantics ("subsequent records dropped on the floor;
	// re-delivered after restart"). Records left unapplied keep their
	// zero-value Result; finalizeBatch halts at the failing record in
	// batch order before it could ever inspect them. Cross-tenant
	// speculative apply past a poison in a DIFFERENT tenant can still
	// happen (different worker, no shared stop) and is safe by the
	// idempotent-replay invariant (ADR-016).
	applyChain := func(refs []recRef) {
		for _, r := range refs {
			res := a.applyRecord(ctx, r.rec)
			results[r.idx] = res
			if res.Err != nil && a.haltOnPoison {
				return
			}
		}
	}

	// Fast path: a single tenant in the batch -> apply serially in this
	// goroutine, behaving exactly like the old serial loop.
	if len(order) <= 1 {
		for _, key := range order {
			applyChain(groups[key])
		}
		return a.finalizeBatch(ctx, records, results)
	}

	// Bound the worker pool by the smaller of the configured cap and
	// the number of distinct tenants.
	workers := a.maxApplyConc
	if workers > len(order) {
		workers = len(order)
	}

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for _, key := range order {
		key := key
		sem <- struct{}{}
		wg.Add(1)
		go func(refs []recRef) {
			defer wg.Done()
			defer func() { <-sem }()
			applyChain(refs)
		}(groups[key])
	}
	wg.Wait()

	return a.finalizeBatch(ctx, records, results)
}

// finalizeBatch commits offsets + runs fan-out for an already-applied
// batch, strictly in batch order, stopping at the first halt-on-poison
// failure (the contiguous-prefix commit rule). Records past a halted
// failure are intentionally left uncommitted for redelivery.
func (a *Applier) finalizeBatch(ctx context.Context, records []Record, results []Result) error {
	for i := range records {
		rec := records[i]
		res := results[i]
		if res.Err != nil {
			if a.haltOnPoison {
				// Do NOT advance the offset. The consumer group's
				// committed position stays at the last successful
				// record; records[i:] are left uncommitted and will
				// be re-delivered after restart.
				return res.Err
			}
			// skip-and-continue: still commit the offset and move on.
		}
		// ADR-027 invariant 3 (load-bearing): the WAL/consumer-group
		// offset commit and the persisted per-tenant offset advance
		// below are the ONLY places these offsets move forward, and
		// they run here in the single serial finalizeBatch loop, in
		// strict batch order, returning at the first poisoned record.
		// That ordered early-return is the entire gap-free
		// contiguous-prefix guarantee. Calling consumer.Commit (or
		// store.UpdateAppliedOffset for the consumer-group position)
		// from a parallel apply worker, a fan-out hook, or any path
		// other than this serial loop lets a faster later-tenant
		// worker advance the offset past an earlier un-applied record
		// and BREAKS gap-freeness. Do not move offset commits out of
		// this loop.
		if err := a.consumer.Commit(ctx, a.groupID, rec); err != nil {
			return fmt.Errorf("apply: commit offset: %w", err)
		}
		if res.Status == StatusApplied || res.Status == StatusSkipped || res.Status == StatusFailedPrecondition || res.Status == StatusUniqueViolation {
			if err := a.store.UpdateAppliedOffset(ctx, res.TenantID, rec.Position.Topic, rec.Position.Partition, rec.Position.Offset); err != nil {
				// Persisting the offset is best-effort; the in-memory
				// tracker has already been updated by the in-txn
				// UpdateAppliedOffsetTx call. Surface the error so the
				// supervisor can act.
				return fmt.Errorf("apply: persist offset: %w", err)
			}
		}
		// Best-effort post-commit fan-out + shared_index.
		a.fanout(ctx, decodeOrZero(rec), &res)
	}
	return nil
}

// routeKey returns the partition key a record's apply work is grouped
// under. It is the event's tenant_id (global events route under the
// synthetic global tenant). A decode failure cannot expose a tenant,
// so such records share one stable synthetic bucket — they still apply
// serially relative to each other and halt-on-poison still fires when
// finalizeBatch reaches them in order.
func routeKey(rec Record) string {
	ev, err := wal.DecodeEvent(rec.Value)
	if err != nil {
		return "\x00poison"
	}
	if ev.TenantID == "" {
		return "\x00poison"
	}
	return string(ev.Scope) + "\x00" + ev.TenantID
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
// idempotency probe (inside-txn check).
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
		// Replay a cached unique-constraint trip identically (issue #566).
		if cachedStatus == store.IdempotencyStatusUniqueViolation {
			res.Status = StatusUniqueViolation
			if cachedFailure.Valid && cachedFailure.String != "" {
				var uv UniqueViolation
				if jerr := json.Unmarshal([]byte(cachedFailure.String), &uv); jerr == nil {
					res.UniqueViolation = &uv
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
			// Unique-constraint trip: same deterministic-failure
			// treatment as a CAS miss — roll the batch back, memoize the
			// structured detail, advance the offset (no halt). The
			// handler lifts the memoized detail into a gRPC
			// ALREADY_EXISTS so the SDK sees a typed
			// UniqueConstraintError (issue #566).
			if uv := AsUniqueViolation(err); uv != nil {
				_ = tx.Rollback()
				committed = true // suppress deferred rollback
				return a.memoizeUniqueViolation(ctx, ev, res, uv)
			}
			// Schema conflict (SELF-DESCRIBING WRITES establish-or-reject):
			// a leading register_schema op supplied a type whose identity
			// is already registered with a different definition. Same
			// deterministic-failure treatment as a CAS miss — roll the
			// batch back, memoize as FAILED_PRECONDITION, advance the
			// offset (no halt). No data op committed.
			if sc := AsSchemaConflict(err); sc != nil {
				_ = tx.Rollback()
				committed = true // suppress deferred rollback
				return a.memoizeSchemaConflict(ctx, ev, res, sc)
			}
			return err
		}
	}

	// Record the applied_events row inside the same txn. v2.2: when
	// any create_node op in the batch tripped a SKIP swallow (issue
	// #599), serialise the index-aligned (CreatedNodes, ExistingNodes)
	// envelope into failure_json so the handler's wait_applied path
	// and any retry replay see the same surface as the first call.
	// Empty failure_json on APPLIED is the legacy "no SKIP happened"
	// signal.
	resultJSON := encodeAppliedResultEnvelope(res)
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO applied_events (tenant_id, idempotency_key, stream_pos, applied_at, status, failure_json)
		VALUES (?, ?, ?, ?, ?, ?)`,
		ev.TenantID, ev.IdempotencyKey, res.Position.String(), a.now(), store.IdempotencyStatusApplied, resultJSON,
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

// memoizeUniqueViolation persists the UNIQUE_VIOLATION row in a fresh
// write-txn and returns nil so the consumer loop advances the offset —
// a constraint trip is a deterministic outcome, exactly like a CAS
// miss. The structured ALREADY_EXISTS detail is stored verbatim in
// failure_json so the handler (and any same-idem-key retry) replays the
// identical typed error. Mirrors memoizePreconditionFailure.
func (a *Applier) memoizeUniqueViolation(ctx context.Context, ev *Event, res *Result, uv *UniqueViolation) error {
	res.Status = StatusUniqueViolation
	res.UniqueViolation = uv
	res.TenantID = ev.TenantID

	failureJSON, err := json.Marshal(uv)
	if err != nil {
		failureJSON = []byte("")
	}
	if err := a.store.RecordIdempotencyOutcome(ctx, ev.TenantID, ev.IdempotencyKey, res.Position.String(), store.IdempotencyStatusUniqueViolation, string(failureJSON)); err != nil {
		// A parallel retry that won the race to memoize first is treated
		// as a successful memoization (same as the precondition path).
		if !errors.Is(err, store.ErrIdempotencyViolation) {
			return fmt.Errorf("apply: memoize unique violation: %w", err)
		}
	}
	if err := a.store.UpdateAppliedOffset(ctx, ev.TenantID, res.Position.Topic, res.Position.Partition, res.Position.Offset); err != nil {
		return fmt.Errorf("apply: unique-violation offset: %w", err)
	}
	return nil
}

// memoizeSchemaConflict persists a FAILED_PRECONDITION row in a fresh
// write-txn for a register_schema establish-or-reject rejection and
// returns nil so the consumer loop advances the offset — a schema
// conflict is a deterministic outcome (re-applying the same schema op
// against the same registry always reproduces it), exactly like a CAS
// miss. The conflict detail is stored verbatim in failure_json so a
// same-idempotency-key retry replays the identical outcome. Mirrors
// memoizePreconditionFailure.
func (a *Applier) memoizeSchemaConflict(ctx context.Context, ev *Event, res *Result, sc *SchemaConflict) error {
	res.Status = StatusFailedPrecondition
	res.TenantID = ev.TenantID
	// Surface the conflict as a precondition failure with the detail on
	// the Field coordinate so GetReceiptStatus / wait_applied callers see
	// FAILED_PRECONDITION; the structured detail is human-readable.
	pf := &PreconditionFailure{Field: sc.Detail}
	res.Precondition = pf

	failureJSON, err := json.Marshal(pf)
	if err != nil {
		failureJSON = []byte("")
	}
	if err := a.store.RecordIdempotencyFailure(ctx, ev.TenantID, ev.IdempotencyKey, res.Position.String(), string(failureJSON)); err != nil {
		if !errors.Is(err, store.ErrIdempotencyViolation) {
			return fmt.Errorf("apply: memoize schema conflict: %w", err)
		}
	}
	if err := a.store.UpdateAppliedOffset(ctx, ev.TenantID, res.Position.Topic, res.Position.Partition, res.Position.Offset); err != nil {
		return fmt.Errorf("apply: schema-conflict offset: %w", err)
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
	case OpRegisterSchema:
		return a.applyRegisterSchema(ctx, tx, ev, op)
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
