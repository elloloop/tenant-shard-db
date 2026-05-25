// shared_index CRUD: add_shared through get_shared_entries_for_node.
//
// IMPORTANT: shared_index is a HINT — authoritative ACLs live in the
// per-tenant canonical_store. Never gate access checks on these rows
// alone.

package globalstore

import (
	"context"
	"fmt"
)

// AddShared upserts a (user_id, source_tenant, node_id) row with the
// given permission. Uses INSERT OR REPLACE so re-sharing with a new
// permission level just updates the row.
func (g *GlobalStore) AddShared(ctx context.Context, userID, sourceTenant, nodeID, permission string) error {
	_, err := g.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO shared_index
		     (user_id, source_tenant, node_id, permission, shared_at)
		 VALUES (?, ?, ?, ?, ?)`,
		userID, sourceTenant, nodeID, permission, g.now(),
	)
	if err != nil {
		return fmt.Errorf("globalstore: add shared (%q,%q,%q): %w", userID, sourceTenant, nodeID, err)
	}
	return nil
}

// RemoveShared deletes one specific (user_id, source_tenant, node_id)
// row. Returns true iff the row existed.
func (g *GlobalStore) RemoveShared(ctx context.Context, userID, sourceTenant, nodeID string) (bool, error) {
	res, err := g.db.ExecContext(ctx,
		`DELETE FROM shared_index
		 WHERE user_id = ? AND source_tenant = ? AND node_id = ?`,
		userID, sourceTenant, nodeID,
	)
	if err != nil {
		return false, fmt.Errorf("globalstore: remove shared (%q,%q,%q): %w", userID, sourceTenant, nodeID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n > 0, nil
}

// ListSharedToUser returns all shared_index rows for a grantee, newest
// first. limit <= 0 -> unlimited (SQLite -1).
func (g *GlobalStore) ListSharedToUser(ctx context.Context, userID string, limit, offset int) ([]*SharedEntry, error) {
	if limit <= 0 {
		limit = -1
	}
	rows, err := g.db.QueryContext(ctx,
		`SELECT user_id, source_tenant, node_id, permission, shared_at
		 FROM shared_index WHERE user_id = ?
		 ORDER BY shared_at DESC LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: list shared to %q: %w", userID, err)
	}
	defer rows.Close()
	return scanSharedRows(rows)
}

// SharedCursor anchors a keyset seek over the shared_index stream for a
// grantee (ADR-029). The effective sort is (shared_at DESC, source_tenant
// DESC, node_id DESC) — (source_tenant, node_id) uniquely identifies a
// shared node — so the seek resumes strictly after this tuple. The tuple
// shape is shared with the per-tenant node_access source so the API layer
// can merge both under one unified cursor.
type SharedCursor struct {
	SharedAt     int64
	SourceTenant string
	NodeID       string
}

// ListSharedToUserPaged is ListSharedToUser with a keyset cursor
// (ADR-029): up to `limit` rows for the grantee ordered (shared_at DESC,
// source_tenant DESC, node_id DESC), seeking strictly after `cursor` when
// non-nil.
func (g *GlobalStore) ListSharedToUserPaged(ctx context.Context, userID string, limit int, cursor *SharedCursor) ([]*SharedEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	q := `SELECT user_id, source_tenant, node_id, permission, shared_at
	      FROM shared_index WHERE user_id = ?`
	args := []any{userID}
	// Keyset seek (ADR-029): resume strictly after the cursor tuple in the
	// effective DESC order. The expanded disjunction avoids relying on
	// SQLite row-value comparison.
	if cursor != nil {
		q += ` AND (shared_at < ?` +
			` OR (shared_at = ? AND source_tenant < ?)` +
			` OR (shared_at = ? AND source_tenant = ? AND node_id < ?))`
		args = append(args,
			cursor.SharedAt,
			cursor.SharedAt, cursor.SourceTenant,
			cursor.SharedAt, cursor.SourceTenant, cursor.NodeID)
	}
	q += ` ORDER BY shared_at DESC, source_tenant DESC, node_id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := g.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("globalstore: list shared paged to %q: %w", userID, err)
	}
	defer rows.Close()
	return scanSharedRows(rows)
}

// ListSharedFromNode returns all shared_index rows for a (source_tenant,
// node_id), newest first.
func (g *GlobalStore) ListSharedFromNode(ctx context.Context, sourceTenant, nodeID string) ([]*SharedEntry, error) {
	rows, err := g.db.QueryContext(ctx,
		`SELECT user_id, source_tenant, node_id, permission, shared_at
		 FROM shared_index WHERE source_tenant = ? AND node_id = ?
		 ORDER BY shared_at DESC`,
		sourceTenant, nodeID,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: list shared from (%q,%q): %w", sourceTenant, nodeID, err)
	}
	defer rows.Close()
	return scanSharedRows(rows)
}

// CleanupStaleShared deletes every shared_index row referring to a
// (source_tenant, node_id) tuple. Called from the Applier when the
// underlying node is deleted. Returns the row count removed.
func (g *GlobalStore) CleanupStaleShared(ctx context.Context, sourceTenant, nodeID string) (int64, error) {
	res, err := g.db.ExecContext(ctx,
		`DELETE FROM shared_index WHERE source_tenant = ? AND node_id = ?`,
		sourceTenant, nodeID,
	)
	if err != nil {
		return 0, fmt.Errorf("globalstore: cleanup stale shared (%q,%q): %w", sourceTenant, nodeID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n, nil
}

// RemoveAllSharedForUser deletes every shared_index row whose grantee
// matches userID. Used by GDPR account deletion.
func (g *GlobalStore) RemoveAllSharedForUser(ctx context.Context, userID string) (int64, error) {
	res, err := g.db.ExecContext(ctx,
		`DELETE FROM shared_index WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		return 0, fmt.Errorf("globalstore: remove all shared for %q: %w", userID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n, nil
}

// scanSharedRows is the common rows->slice helper for shared_index
// queries.
func scanSharedRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]*SharedEntry, error) {
	out := []*SharedEntry{}
	for rows.Next() {
		var s SharedEntry
		if err := rows.Scan(&s.UserID, &s.SourceTenant, &s.NodeID, &s.Permission, &s.SharedAt); err != nil {
			return nil, fmt.Errorf("globalstore: scan shared row: %w", err)
		}
		out = append(out, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globalstore: iterate shared rows: %w", err)
	}
	return out, nil
}

// RevokeUserAccess deletes the (tenant_id, user_id) membership AND
// every shared_index row where the user is the grantee within that
// tenant, in a single transaction.
func (g *GlobalStore) RevokeUserAccess(ctx context.Context, tenantID, userID string) (*RevokeResult, error) {
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("globalstore: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	memRes, err := tx.ExecContext(ctx,
		`DELETE FROM tenant_members WHERE tenant_id = ? AND user_id = ?`,
		tenantID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: revoke (delete member): %w", err)
	}
	memN, err := memRes.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("globalstore: rows affected: %w", err)
	}

	sharedRes, err := tx.ExecContext(ctx,
		`DELETE FROM shared_index WHERE user_id = ? AND source_tenant = ?`,
		userID, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: revoke (delete shared): %w", err)
	}
	sharedN, err := sharedRes.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("globalstore: rows affected: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("globalstore: commit revoke: %w", err)
	}
	return &RevokeResult{
		TenantID:          tenantID,
		UserID:            userID,
		MembershipRemoved: memN > 0,
		SharedRemoved:     sharedN,
	}, nil
}
