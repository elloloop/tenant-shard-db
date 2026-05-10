// legal_holds CRUD. Mirrors the Python helpers at
// server/python/entdb_server/global_store.py:967 (set_legal_hold_record)
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
