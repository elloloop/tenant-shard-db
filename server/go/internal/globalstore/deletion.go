// deletion_queue CRUD. Mirrors the Python helpers at
// through :908 (mark_deletion_completed). Used by the GDPR worker.

package globalstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"google.golang.org/grpc/codes"
)

// secondsPerDay is the conversion factor used by QueueDeletion's grace
// period. Pulled out so tests can reference it without re-deriving.
const secondsPerDay int64 = 86400

// QueueDeletion schedules a user for deletion after `graceDays`. If
// `graceDays <= 0`, deletion is queued for "now" (matches the Python
// behaviour at global_store.py:817 — `now + grace_days * 86400` with
// no clamp). Returns ErrAlreadyExists if the user is already queued.
func (g *GlobalStore) QueueDeletion(ctx context.Context, userID string, graceDays int) (*DeletionEntry, error) {
	now := g.now()
	executeAt := now + int64(graceDays)*secondsPerDay
	_, err := g.db.ExecContext(ctx,
		`INSERT INTO deletion_queue (user_id, requested_at, execute_at, status)
		 VALUES (?, ?, ?, 'pending')`,
		userID, now, executeAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errs.Errorf(codes.AlreadyExists,
				"globalstore: user %q already queued for deletion", userID)
		}
		return nil, fmt.Errorf("globalstore: queue deletion %q: %w", userID, err)
	}
	return &DeletionEntry{
		UserID:      userID,
		RequestedAt: now,
		ExecuteAt:   executeAt,
		Status:      "pending",
	}, nil
}

// CancelDeletion removes a *pending* deletion_queue row. Returns true
// iff a pending row existed. Already-completed entries are immutable
// (they record the audit fact that erasure ran). Mirrors
// `_sync_cancel_deletion` (global_store.py:842).
func (g *GlobalStore) CancelDeletion(ctx context.Context, userID string) (bool, error) {
	res, err := g.db.ExecContext(ctx,
		`DELETE FROM deletion_queue WHERE user_id = ? AND status = 'pending'`,
		userID,
	)
	if err != nil {
		return false, fmt.Errorf("globalstore: cancel deletion %q: %w", userID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n > 0, nil
}

// GetDeletionQueue returns every pending deletion entry, ordered by
// execute_at ASC. Mirrors `_sync_get_pending_deletions`
// (global_store.py:858).
func (g *GlobalStore) GetDeletionQueue(ctx context.Context) ([]*DeletionEntry, error) {
	rows, err := g.db.QueryContext(ctx,
		`SELECT user_id, requested_at, execute_at, export_path, status
		 FROM deletion_queue WHERE status = 'pending'
		 ORDER BY execute_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: list deletion queue: %w", err)
	}
	defer rows.Close()
	return scanDeletionRows(rows)
}

// GetExecutableDeletions returns pending entries whose execute_at is
// <= `now` (Unix-epoch second). If `now == 0`, the store's clock is
// used. Mirrors `_sync_get_executable_deletions` (global_store.py:876).
func (g *GlobalStore) GetExecutableDeletions(ctx context.Context, now int64) ([]*DeletionEntry, error) {
	if now == 0 {
		now = g.now()
	}
	rows, err := g.db.QueryContext(ctx,
		`SELECT user_id, requested_at, execute_at, export_path, status
		 FROM deletion_queue
		 WHERE status = 'pending' AND execute_at <= ?
		 ORDER BY execute_at`,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: list executable deletions: %w", err)
	}
	defer rows.Close()
	return scanDeletionRows(rows)
}

// GetDeletionEntry returns one user's deletion_queue entry, or
// (nil, nil) if not present.
func (g *GlobalStore) GetDeletionEntry(ctx context.Context, userID string) (*DeletionEntry, error) {
	row := g.db.QueryRowContext(ctx,
		`SELECT user_id, requested_at, execute_at, export_path, status
		 FROM deletion_queue WHERE user_id = ?`,
		userID,
	)
	var d DeletionEntry
	var exportPath sql.NullString
	if err := row.Scan(&d.UserID, &d.RequestedAt, &d.ExecuteAt, &exportPath, &d.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("globalstore: get deletion entry %q: %w", userID, err)
	}
	if exportPath.Valid {
		d.ExportPath = exportPath.String
	}
	return &d, nil
}

// MarkDeletionStarted is reserved for future progress reporting. The
// Python implementation jumps straight from 'pending' to 'completed'
// (global_store.py:902); we keep the symbol so the GDPR worker has an
// explicit hook.
func (g *GlobalStore) MarkDeletionStarted(ctx context.Context, userID string) (bool, error) {
	res, err := g.db.ExecContext(ctx,
		`UPDATE deletion_queue SET status = 'in_progress' WHERE user_id = ? AND status = 'pending'`,
		userID,
	)
	if err != nil {
		return false, fmt.Errorf("globalstore: mark deletion started %q: %w", userID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n > 0, nil
}

// MarkDeletionCompleted flips the row to status='completed'. Mirrors
// `_sync_mark_deletion_completed` (global_store.py:902).
func (g *GlobalStore) MarkDeletionCompleted(ctx context.Context, userID string) (bool, error) {
	res, err := g.db.ExecContext(ctx,
		`UPDATE deletion_queue SET status = 'completed' WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		return false, fmt.Errorf("globalstore: mark deletion completed %q: %w", userID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n > 0, nil
}

func scanDeletionRows(rows *sql.Rows) ([]*DeletionEntry, error) {
	out := []*DeletionEntry{}
	for rows.Next() {
		var d DeletionEntry
		var exportPath sql.NullString
		if err := rows.Scan(&d.UserID, &d.RequestedAt, &d.ExecuteAt, &exportPath, &d.Status); err != nil {
			return nil, fmt.Errorf("globalstore: scan deletion row: %w", err)
		}
		if exportPath.Valid {
			d.ExportPath = exportPath.String
		}
		out = append(out, &d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globalstore: iterate deletion rows: %w", err)
	}
	return out, nil
}
