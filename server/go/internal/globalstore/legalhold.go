// legal_holds CRUD. Mirrors the Python helpers at
// through :1057 (is_under_legal_hold).
//
// Two concepts collide here:
//
//   - tenant_registry.status='legal_hold' is the *gating* flag that the
//     gRPC layer reads when deciding whether to admit a delete.
//   - legal_holds is the *audit/informational* table that records who
//     placed the hold, when, and why.
//
// They're independent today (the Python global_store has both
// `set_legal_hold` and `set_legal_hold_record`); see the Open Question
// in docs/go-port/shared/global-store.md about unifying them.

package globalstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SetLegalHold inserts a legal_holds row for (tenant_id, held_by). It
// is INSERT OR IGNORE — re-running with the same key is a no-op.
// Mirrors `_sync_set_legal_hold_record` (global_store.py:984).
func (g *GlobalStore) SetLegalHold(ctx context.Context, tenantID, heldBy, reason string) (*LegalHold, error) {
	now := g.now()
	_, err := g.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO legal_holds (tenant_id, held_by, reason, created_at)
		 VALUES (?, ?, ?, ?)`,
		tenantID, heldBy, reason, now,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: set legal hold (%q,%q): %w", tenantID, heldBy, err)
	}
	return &LegalHold{
		TenantID:  tenantID,
		HeldBy:    heldBy,
		Reason:    reason,
		CreatedAt: now,
	}, nil
}

// ClearLegalHold removes a (tenant_id, held_by) row. Returns true iff
// a row existed. Mirrors `_sync_remove_legal_hold` (global_store.py:1013).
func (g *GlobalStore) ClearLegalHold(ctx context.Context, tenantID, heldBy string) (bool, error) {
	res, err := g.db.ExecContext(ctx,
		`DELETE FROM legal_holds WHERE tenant_id = ? AND held_by = ?`,
		tenantID, heldBy,
	)
	if err != nil {
		return false, fmt.Errorf("globalstore: clear legal hold (%q,%q): %w", tenantID, heldBy, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n > 0, nil
}

// GetLegalHold lists every legal_holds row for a tenant, ordered by
// created_at. Mirrors `_sync_get_legal_holds` (global_store.py:1032).
func (g *GlobalStore) GetLegalHold(ctx context.Context, tenantID string) ([]*LegalHold, error) {
	rows, err := g.db.QueryContext(ctx,
		`SELECT tenant_id, held_by, reason, created_at
		 FROM legal_holds WHERE tenant_id = ? ORDER BY created_at`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: get legal holds %q: %w", tenantID, err)
	}
	defer rows.Close()
	out := []*LegalHold{}
	for rows.Next() {
		var h LegalHold
		if err := rows.Scan(&h.TenantID, &h.HeldBy, &h.Reason, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("globalstore: scan legal hold: %w", err)
		}
		out = append(out, &h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globalstore: iterate legal holds: %w", err)
	}
	return out, nil
}

// IsLegalHoldSet reports whether *any* legal_holds row exists for the
// tenant. Mirrors `_sync_is_under_legal_hold` (global_store.py:1051).
func (g *GlobalStore) IsLegalHoldSet(ctx context.Context, tenantID string) (bool, error) {
	var one int
	err := g.db.QueryRowContext(ctx,
		`SELECT 1 FROM legal_holds WHERE tenant_id = ? LIMIT 1`,
		tenantID,
	).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("globalstore: is_under_legal_hold %q: %w", tenantID, err)
	}
	return true, nil
}

// LegalHoldLiftQueueEntry is one pending-lift row: a tenant whose hold
// was explicitly released and whose already-archived S3 objects still
// need their Object Lock legal hold cleared.
type LegalHoldLiftQueueEntry struct {
	TenantID   string
	EnqueuedAt int64
}

// enqueueLegalHoldLiftTx upserts a pending-lift row INSIDE the caller's
// transaction. It is called from ApplyLegalHoldSet on the durable
// release path so the pending lift is committed atomically with clearing
// the legal_holds row + flipping tenant_registry.status — a crash AFTER
// the release still has the lift recorded. ON CONFLICT keeps the
// earliest enqueued_at so a re-release does not reset the age. EPIC #511
// Gap 1.
func enqueueLegalHoldLiftTx(ctx context.Context, tx *sql.Tx, tenantID string, enqueuedAt int64) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO legal_hold_lift_queue (tenant_id, enqueued_at)
		 VALUES (?, ?)
		 ON CONFLICT(tenant_id) DO NOTHING`,
		tenantID, enqueuedAt,
	); err != nil {
		return fmt.Errorf("globalstore: enqueue legal-hold lift %q: %w", tenantID, err)
	}
	return nil
}

// GetLegalHoldLiftQueue returns every pending-lift entry, oldest first.
// The background lift worker drains this on its tick. Reading purely
// from the durable table is what makes the lift survive a server
// restart (no in-memory trigger state).
func (g *GlobalStore) GetLegalHoldLiftQueue(ctx context.Context) ([]*LegalHoldLiftQueueEntry, error) {
	rows, err := g.db.QueryContext(ctx,
		`SELECT tenant_id, enqueued_at
		 FROM legal_hold_lift_queue
		 ORDER BY enqueued_at, tenant_id`,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: list legal-hold lift queue: %w", err)
	}
	defer rows.Close()
	out := []*LegalHoldLiftQueueEntry{}
	for rows.Next() {
		var e LegalHoldLiftQueueEntry
		if err := rows.Scan(&e.TenantID, &e.EnqueuedAt); err != nil {
			return nil, fmt.Errorf("globalstore: scan legal-hold lift row: %w", err)
		}
		out = append(out, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globalstore: iterate legal-hold lift rows: %w", err)
	}
	return out, nil
}

// DequeueLegalHoldLift removes a tenant's pending-lift row. The worker
// calls this only after the sweep for that tenant has fully completed;
// any failure / partial run leaves the row so the next tick retries.
// Returns true iff a row existed.
func (g *GlobalStore) DequeueLegalHoldLift(ctx context.Context, tenantID string) (bool, error) {
	res, err := g.db.ExecContext(ctx,
		`DELETE FROM legal_hold_lift_queue WHERE tenant_id = ?`,
		tenantID,
	)
	if err != nil {
		return false, fmt.Errorf("globalstore: dequeue legal-hold lift %q: %w", tenantID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n > 0, nil
}

// CountLegalHoldLiftQueue returns the number of pending-lift rows. Used
// to publish the entdb_legal_hold_lift_pending gauge.
func (g *GlobalStore) CountLegalHoldLiftQueue(ctx context.Context) (int, error) {
	var n int
	if err := g.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM legal_hold_lift_queue`,
	).Scan(&n); err != nil {
		return 0, fmt.Errorf("globalstore: count legal-hold lift queue: %w", err)
	}
	return n, nil
}
