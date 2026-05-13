// SPDX-License-Identifier: AGPL-3.0-only

package apply_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// preconditionOp builds an update_node op with a CAS precondition keyed
// on a single field_id. equals is passed through unboxed (matching the
// handler-side translation).
func preconditionOp(id string, typeID int32, patch map[string]any, fieldID string, fieldName string, equals any) map[string]any {
	op := map[string]any{
		"op":      string(apply.OpUpdateNode),
		"id":      id,
		"type_id": typeID,
		"patch":   patch,
		"precondition": map[string]any{
			"field":    fieldName,
			"field_id": fieldID,
			"equals":   equals,
		},
	}
	return op
}

// TestApplier_PreconditionMet — happy path: observed matches expected.
// Both the update patch AND a side-effect create_node commit; the
// idempotency cache records APPLIED.
func TestApplier_PreconditionMet(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	// Seed: token node with consumed_at=0 (field_id "2" on disk).
	seed := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "seed",
		Ops: []map[string]any{mkCreateNode("tok1", 1, map[string]any{"2": float64(0)})},
	}
	f.appendEvent(t, seed)

	// CAS consume + audit side-effect.
	ev := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "consume1",
		Ops: []map[string]any{
			preconditionOp("tok1", 1, map[string]any{"2": float64(1700000000)},
				"2", "consumed_at", float64(0)),
			mkCreateNode("audit1", 2, map[string]any{"1": "refresh_consumed"}),
		},
	}
	f.appendEvent(t, ev)

	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "consume1")

	rec, err := f.store.CheckIdempotencyStatus(context.Background(), testTenant, "consume1")
	if err != nil {
		t.Fatalf("CheckIdempotencyStatus: %v", err)
	}
	if rec.Status != store.IdempotencyStatusApplied {
		t.Fatalf("status=%q want APPLIED", rec.Status)
	}
	// Side-effect node committed.
	if _, err := f.store.GetNode(context.Background(), testTenant, "audit1"); err != nil {
		t.Fatalf("audit node missing: %v", err)
	}
	// Token field updated.
	n, err := f.store.GetNode(context.Background(), testTenant, "tok1")
	if err != nil {
		t.Fatalf("token missing: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(n.PayloadJSON), &payload); err != nil {
		t.Fatalf("payload decode: %v", err)
	}
	if payload["2"] != float64(1700000000) {
		t.Fatalf("consumed_at=%v want 1700000000", payload["2"])
	}
}

// TestApplier_PreconditionFailedAbortsBatch — observed does not match
// expected. The applier MUST: (1) abort every op in the batch
// (including side-effect ops), (2) memoize FAILED_PRECONDITION in the
// idempotency cache, (3) NOT halt the consumer loop, (4) advance the
// WAL offset so a fresh applier rebuilding from the WAL produces the
// same outcome.
func TestApplier_PreconditionFailedAbortsBatch(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	// Seed: token already consumed.
	seed := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "seed",
		Ops: []map[string]any{mkCreateNode("tok1", 1, map[string]any{"2": float64(99)})},
	}
	f.appendEvent(t, seed)

	// CAS consume expecting consumed_at=0 but actual=99 -> fail.
	ev := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "consume1",
		Ops: []map[string]any{
			preconditionOp("tok1", 1, map[string]any{"2": float64(1700000000)},
				"2", "consumed_at", float64(0)),
			mkCreateNode("audit1", 2, map[string]any{"1": "refresh_consumed"}),
		},
	}
	f.appendEvent(t, ev)

	// Run applier — it MUST NOT halt; this is the
	// not-poison-but-expected-error branch.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.applier.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	f.waitForIdempKey(t, testTenant, "consume1")

	rec, err := f.store.CheckIdempotencyStatus(context.Background(), testTenant, "consume1")
	if err != nil {
		t.Fatalf("CheckIdempotencyStatus: %v", err)
	}
	if rec.Status != store.IdempotencyStatusFailedPrecondition {
		t.Fatalf("status=%q want FAILED_PRECONDITION", rec.Status)
	}
	if rec.FailureJSON == "" {
		t.Fatalf("failure_json is empty; want JSON-encoded PreconditionFailure")
	}
	// Decode + verify coordinates.
	var pf apply.PreconditionFailure
	if err := json.Unmarshal([]byte(rec.FailureJSON), &pf); err != nil {
		t.Fatalf("decode failure_json: %v", err)
	}
	if pf.OpIndex != 0 {
		t.Fatalf("OpIndex=%d want 0", pf.OpIndex)
	}
	if pf.Field != "consumed_at" {
		t.Fatalf("Field=%q want consumed_at", pf.Field)
	}
	if pf.Observed != float64(99) {
		t.Fatalf("Observed=%v want 99", pf.Observed)
	}
	if pf.Expected != float64(0) {
		t.Fatalf("Expected=%v want 0", pf.Expected)
	}
	if !pf.FieldPresent {
		t.Fatalf("FieldPresent=false want true (consumed_at was set to 99)")
	}
	// Side-effect node MUST NOT have committed.
	if _, err := f.store.GetNode(context.Background(), testTenant, "audit1"); err == nil {
		t.Fatalf("audit node committed despite precondition failure — hard-fail invariant violated")
	}
	// Token payload MUST NOT have been mutated.
	n, err := f.store.GetNode(context.Background(), testTenant, "tok1")
	if err != nil {
		t.Fatalf("token missing: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(n.PayloadJSON), &payload); err != nil {
		t.Fatalf("payload decode: %v", err)
	}
	if payload["2"] != float64(99) {
		t.Fatalf("consumed_at=%v want 99 (unchanged)", payload["2"])
	}
}

// TestApplier_PreconditionRetryReplaysCachedFailure — the failure
// branch must be cached so a retry with the SAME idempotency key
// returns the same memoized failure without re-evaluating the
// predicate against possibly-changed state.
func TestApplier_PreconditionRetryReplaysCachedFailure(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	// Seed: token already consumed.
	seed := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "seed",
		Ops: []map[string]any{mkCreateNode("tok1", 1, map[string]any{"2": float64(99)})},
	}
	f.appendEvent(t, seed)

	// First attempt: precondition fails, cached.
	first := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "consume1",
		Ops: []map[string]any{
			preconditionOp("tok1", 1, map[string]any{"2": float64(1700000000)},
				"2", "consumed_at", float64(0)),
		},
	}
	f.appendEvent(t, first)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.applier.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	f.waitForIdempKey(t, testTenant, "consume1")

	// Now mutate the underlying state OUT-OF-BAND (via another WAL
	// event with a different idem key) so the predicate WOULD now
	// match. The retry must still see the cached failure.
	resetEv := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "reset",
		Ops: []map[string]any{{
			"op":      string(apply.OpUpdateNode),
			"id":      "tok1",
			"type_id": int32(1),
			"patch":   map[string]any{"2": float64(0)},
		}},
	}
	f.appendEvent(t, resetEv)
	f.waitForIdempKey(t, testTenant, "reset")

	// Retry with the SAME idempotency key. The first applier still
	// sees this event because the WAL is append-only; the applier's
	// in-txn idempotency probe matches the cached FAILED_PRECONDITION
	// row and replays it.
	retry := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "consume1",
		TsMs: 1700000000123,
		Ops: []map[string]any{
			preconditionOp("tok1", 1, map[string]any{"2": float64(1700000000)},
				"2", "consumed_at", float64(0)),
		},
	}
	f.appendEvent(t, retry)
	// Give the consumer a moment to process the retry — it MUST be a
	// no-op cache hit, not a re-evaluation.
	time.Sleep(200 * time.Millisecond)

	rec, err := f.store.CheckIdempotencyStatus(context.Background(), testTenant, "consume1")
	if err != nil {
		t.Fatalf("CheckIdempotencyStatus: %v", err)
	}
	if rec.Status != store.IdempotencyStatusFailedPrecondition {
		t.Fatalf("retry promoted cached FAILED to %q — must replay cached failure", rec.Status)
	}
	// Verify the token was NOT mutated by the retry.
	n, err := f.store.GetNode(context.Background(), testTenant, "tok1")
	if err != nil {
		t.Fatalf("token missing: %v", err)
	}
	var payload map[string]any
	_ = json.Unmarshal([]byte(n.PayloadJSON), &payload)
	if payload["2"] != float64(0) {
		t.Fatalf("token mutated by retry; consumed_at=%v want 0", payload["2"])
	}
}

// TestApplier_PreconditionRetryAfterSuccess — success branch is also
// cached. A retry with the same idem key after success returns the
// cached APPLIED row even when the precondition would no longer hold
// against current state.
func TestApplier_PreconditionRetryAfterSuccess(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	seed := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "seed",
		Ops: []map[string]any{mkCreateNode("tok1", 1, map[string]any{"2": float64(0)})},
	}
	f.appendEvent(t, seed)

	// First attempt succeeds.
	first := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "consume1",
		Ops: []map[string]any{
			preconditionOp("tok1", 1, map[string]any{"2": float64(1700000000)},
				"2", "consumed_at", float64(0)),
		},
	}
	f.appendEvent(t, first)

	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "consume1")

	// Retry with same key — precondition would now FAIL (consumed_at
	// is no longer 0) but the cache short-circuits to APPLIED.
	retry := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "consume1",
		TsMs: 1700000000123,
		Ops: []map[string]any{
			preconditionOp("tok1", 1, map[string]any{"2": float64(1700000000)},
				"2", "consumed_at", float64(0)),
		},
	}
	f.appendEvent(t, retry)
	time.Sleep(200 * time.Millisecond)

	rec, err := f.store.CheckIdempotencyStatus(context.Background(), testTenant, "consume1")
	if err != nil {
		t.Fatalf("CheckIdempotencyStatus: %v", err)
	}
	if rec.Status != store.IdempotencyStatusApplied {
		t.Fatalf("retry mutated cached APPLIED to %q", rec.Status)
	}
}

// TestApplier_PreconditionMultipleOpsAnyFails — when multiple
// conditional ops live in one batch, any one failing aborts the entire
// batch.
func TestApplier_PreconditionMultipleOpsAnyFails(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	// Seed two tokens — A has consumed_at=0 (predicate matches), B
	// has consumed_at=99 (predicate fails).
	seed := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "seed",
		Ops: []map[string]any{
			mkCreateNode("tokA", 1, map[string]any{"2": float64(0)}),
			mkCreateNode("tokB", 1, map[string]any{"2": float64(99)}),
		},
	}
	f.appendEvent(t, seed)

	ev := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "consume1",
		Ops: []map[string]any{
			preconditionOp("tokA", 1, map[string]any{"2": float64(1700000000)},
				"2", "consumed_at", float64(0)),
			preconditionOp("tokB", 1, map[string]any{"2": float64(1700000000)},
				"2", "consumed_at", float64(0)),
		},
	}
	f.appendEvent(t, ev)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.applier.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	f.waitForIdempKey(t, testTenant, "consume1")

	rec, err := f.store.CheckIdempotencyStatus(context.Background(), testTenant, "consume1")
	if err != nil {
		t.Fatalf("CheckIdempotencyStatus: %v", err)
	}
	if rec.Status != store.IdempotencyStatusFailedPrecondition {
		t.Fatalf("status=%q want FAILED_PRECONDITION", rec.Status)
	}
	var pf apply.PreconditionFailure
	if err := json.Unmarshal([]byte(rec.FailureJSON), &pf); err != nil {
		t.Fatalf("decode failure: %v", err)
	}
	if pf.OpIndex != 1 {
		t.Fatalf("OpIndex=%d want 1 (the failing tokB op)", pf.OpIndex)
	}
	// Neither token was mutated — tokA's predicate matched, but its
	// patch must not commit because tokB's predicate failed.
	for _, id := range []string{"tokA", "tokB"} {
		n, err := f.store.GetNode(context.Background(), testTenant, id)
		if err != nil {
			t.Fatalf("token %s missing: %v", id, err)
		}
		var p map[string]any
		_ = json.Unmarshal([]byte(n.PayloadJSON), &p)
		want := float64(0)
		if id == "tokB" {
			want = float64(99)
		}
		if p["2"] != want {
			t.Fatalf("token %s consumed_at=%v want %v (unchanged)", id, p["2"], want)
		}
	}
}

// TestApplier_PreconditionFailedUnwraps — the sentinel is preserved
// through errors.Is + errors.As so callers can branch on it.
func TestApplier_PreconditionFailedUnwraps(t *testing.T) {
	t.Parallel()
	pf := &apply.PreconditionFailure{OpIndex: 3, Field: "x", Expected: 1, Observed: 2}
	if !errors.Is(pf, apply.ErrPreconditionFailed) {
		t.Fatalf("errors.Is should match ErrPreconditionFailed")
	}
	extracted := apply.AsPreconditionFailure(pf)
	if extracted == nil || extracted.OpIndex != 3 {
		t.Fatalf("AsPreconditionFailure did not unwrap; got %+v", extracted)
	}
}

// TestApplier_PreconditionMissingNode — predicate against a non-
// existent node is treated as a miss with observed=nil and
// field_present=false (it is NOT a poison event).
func TestApplier_PreconditionMissingNode(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ev := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "ghost",
		Ops: []map[string]any{
			preconditionOp("does-not-exist", 1, map[string]any{"2": float64(1)},
				"2", "consumed_at", float64(0)),
		},
	}
	f.appendEvent(t, ev)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.applier.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	f.waitForIdempKey(t, testTenant, "ghost")

	rec, err := f.store.CheckIdempotencyStatus(context.Background(), testTenant, "ghost")
	if err != nil {
		t.Fatalf("CheckIdempotencyStatus: %v", err)
	}
	if rec.Status != store.IdempotencyStatusFailedPrecondition {
		t.Fatalf("status=%q want FAILED_PRECONDITION (missing node should be a miss, not a poison)", rec.Status)
	}
	var pf apply.PreconditionFailure
	_ = json.Unmarshal([]byte(rec.FailureJSON), &pf)
	if pf.FieldPresent {
		t.Fatalf("FieldPresent=true on missing node; want false")
	}
}
