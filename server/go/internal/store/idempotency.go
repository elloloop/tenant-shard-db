package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// CheckIdempotency reports whether (tenant_id, request_id) has already
// been recorded. Mirrors canonical_store.py:_sync_check_idempotency
// (1379).
func (s *CanonicalStore) CheckIdempotency(ctx context.Context, tenantID, requestID string) (bool, error) {
	db, err := s.db(tenantID)
	if err != nil {
		return false, err
	}
	var one int
	err = db.QueryRowContext(ctx,
		`SELECT 1 FROM applied_events WHERE tenant_id = ? AND idempotency_key = ?`,
		tenantID, requestID,
	).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("store: CheckIdempotency: %w", err)
	}
	return true, nil
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
			INSERT INTO applied_events (tenant_id, idempotency_key, stream_pos, applied_at)
			VALUES (?, ?, ?, ?)`,
			tenantID, requestID, streamPos, now,
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
// open BatchTxn. Mirrors the Python pattern at canonical_store.py:1409
// where the applier passes its conn into _sync_record_applied_event.
func (s *CanonicalStore) RecordIdempotencyTx(ctx context.Context, b *BatchTxn, requestID, streamPos string) error {
	_, err := b.conn.ExecContext(ctx, `
		INSERT INTO applied_events (tenant_id, idempotency_key, stream_pos, applied_at)
		VALUES (?, ?, ?, ?)`,
		b.tenantID, requestID, streamPos, s.now(),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: %s/%s", ErrIdempotencyViolation, b.tenantID, requestID)
		}
		return fmt.Errorf("store: RecordIdempotencyTx: %w", err)
	}
	return nil
}
