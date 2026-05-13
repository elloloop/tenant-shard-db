package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Idempotency status values stored on applied_events.status.
const (
	// IdempotencyStatusApplied is the legacy success record. Any row
	// pre-dating the GitHub-issue-#500 CAS migration also reads as
	// APPLIED via the column default.
	IdempotencyStatusApplied = "APPLIED"

	// IdempotencyStatusFailedPrecondition is the memoized
	// precondition-miss record. The accompanying failure_json column
	// holds the structured failure detail (op_index, field, expected,
	// observed, field_present) as JSON so a retry with the same idem
	// key replays the cached failure without re-evaluating the
	// predicate against possibly-changed state. See GitHub issue #500.
	IdempotencyStatusFailedPrecondition = "FAILED_PRECONDITION"
)

// IdempotencyRecord is the result of CheckIdempotencyStatus. Present is
// false when no row exists for (tenant_id, idempotency_key). On
// FAILED_PRECONDITION rows FailureJSON carries the JSON-encoded
// apply.PreconditionFailure payload.
type IdempotencyRecord struct {
	Present     bool
	Status      string
	FailureJSON string
}

// CheckIdempotency reports whether (tenant_id, request_id) has already
// been recorded. Mirrors canonical_store.py:_sync_check_idempotency
// (1379).
//
// Pre-CAS callers that just want a boolean "have we seen this idem
// key" answer remain on this method. Callers that need to distinguish
// the cached-failure case (so a retry replays the same typed error)
// should use CheckIdempotencyStatus instead.
func (s *CanonicalStore) CheckIdempotency(ctx context.Context, tenantID, requestID string) (bool, error) {
	rec, err := s.CheckIdempotencyStatus(ctx, tenantID, requestID)
	if err != nil {
		return false, err
	}
	return rec.Present, nil
}

// CheckIdempotencyStatus returns the full applied_events row coordinates
// for (tenant_id, request_id) — Present + Status + FailureJSON. Used by
// the applier's in-txn idempotency probe (so cached
// FAILED_PRECONDITION rows short-circuit to the memoized failure
// branch) and by the handler's GetReceiptStatus path.
func (s *CanonicalStore) CheckIdempotencyStatus(ctx context.Context, tenantID, requestID string) (IdempotencyRecord, error) {
	db, err := s.db(tenantID)
	if err != nil {
		return IdempotencyRecord{}, err
	}
	var status string
	var failure sql.NullString
	err = db.QueryRowContext(ctx,
		`SELECT status, failure_json FROM applied_events WHERE tenant_id = ? AND idempotency_key = ?`,
		tenantID, requestID,
	).Scan(&status, &failure)
	if errors.Is(err, sql.ErrNoRows) {
		return IdempotencyRecord{Present: false}, nil
	}
	if err != nil {
		return IdempotencyRecord{}, fmt.Errorf("store: CheckIdempotencyStatus: %w", err)
	}
	rec := IdempotencyRecord{Present: true, Status: status}
	if failure.Valid {
		rec.FailureJSON = failure.String
	}
	return rec, nil
}

// RecordIdempotency inserts a (tenant_id, request_id, stream_pos) row
// in applied_events. Returns ErrIdempotencyViolation when the key has
// already been recorded. Mirrors canonical_store.py:_sync_record_applied_event
// (1409).
//
// The applier calls this inside its BatchTxn (see RecordIdempotencyTx).
func (s *CanonicalStore) RecordIdempotency(ctx context.Context, tenantID, requestID, streamPos string) error {
	now := s.now()
	return s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, `
			INSERT INTO applied_events (tenant_id, idempotency_key, stream_pos, applied_at, status)
			VALUES (?, ?, ?, ?, ?)`,
			tenantID, requestID, streamPos, now, IdempotencyStatusApplied,
		)
		if err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: %s/%s", ErrIdempotencyViolation, tenantID, requestID)
			}
			return fmt.Errorf("store: RecordIdempotency: %w", err)
		}
		return nil
	})
}

// RecordIdempotencyTx records the idempotency key inside an already-
// open BatchTxn with status=APPLIED. Mirrors the Python pattern at
// canonical_store.py:1409 where the applier passes its conn into
// _sync_record_applied_event.
func (s *CanonicalStore) RecordIdempotencyTx(ctx context.Context, b *BatchTxn, requestID, streamPos string) error {
	_, err := b.conn.ExecContext(ctx, `
		INSERT INTO applied_events (tenant_id, idempotency_key, stream_pos, applied_at, status)
		VALUES (?, ?, ?, ?, ?)`,
		b.tenantID, requestID, streamPos, s.now(), IdempotencyStatusApplied,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: %s/%s", ErrIdempotencyViolation, b.tenantID, requestID)
		}
		return fmt.Errorf("store: RecordIdempotencyTx: %w", err)
	}
	return nil
}

// RecordIdempotencyFailure inserts a FAILED_PRECONDITION row keyed by
// (tenant_id, requestID). Called by the applier's failure-memoization
// path after a conditional UpdateNodeOp precondition miss; the call
// runs in its OWN transaction (the apply-batch txn was rolled back) so
// the WAL offset still advances and a subsequent retry with the same
// idem key replays the cached failure rather than re-evaluating.
//
// failureJSON is opaque to the store — the caller (apply package)
// owns the encoding.
func (s *CanonicalStore) RecordIdempotencyFailure(ctx context.Context, tenantID, requestID, streamPos, failureJSON string) error {
	now := s.now()
	return s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, `
			INSERT INTO applied_events (tenant_id, idempotency_key, stream_pos, applied_at, status, failure_json)
			VALUES (?, ?, ?, ?, ?, ?)`,
			tenantID, requestID, streamPos, now, IdempotencyStatusFailedPrecondition, failureJSON,
		)
		if err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: %s/%s", ErrIdempotencyViolation, tenantID, requestID)
			}
			return fmt.Errorf("store: RecordIdempotencyFailure: %w", err)
		}
		return nil
	})
}
