// user_registry CRUD. Mirrors the Python helpers at
// server/python/entdb_server/global_store.py:336 (create_user) through
// :449 (set_user_status). user_id is bare ("alice"), never prefixed
// (`user:alice`). Translation happens at the gRPC boundary.

package globalstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"google.golang.org/grpc/codes"
)

// CreateUser inserts a new user_registry row. Returns ErrAlreadyExists
// (codes.AlreadyExists) on duplicate user_id or duplicate email.
//
// Mirrors `_sync_create_user` (global_store.py:352).
func (g *GlobalStore) CreateUser(ctx context.Context, userID, email, name string) (*User, error) {
	now := g.now()
	_, err := g.db.ExecContext(ctx,
		`INSERT INTO user_registry (user_id, email, name, status, created_at, updated_at)
		 VALUES (?, ?, ?, 'active', ?, ?)`,
		userID, email, name, now, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errs.Errorf(codes.AlreadyExists,
				"globalstore: user %q already exists (or email %q taken)", userID, email)
		}
		return nil, fmt.Errorf("globalstore: create user %q: %w", userID, err)
	}
	return &User{
		UserID:    userID,
		Email:     email,
		Name:      name,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetUser returns the user, or (nil, nil) if not present. The "no
// error" sentinel mirrors the Python `dict | None` contract — callers
// decide whether to upgrade missing-row to NOT_FOUND.
func (g *GlobalStore) GetUser(ctx context.Context, userID string) (*User, error) {
	row := g.db.QueryRowContext(ctx,
		`SELECT user_id, email, name, status, created_at, updated_at
		 FROM user_registry WHERE user_id = ?`,
		userID,
	)
	var u User
	if err := row.Scan(&u.UserID, &u.Email, &u.Name, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("globalstore: get user %q: %w", userID, err)
	}
	return &u, nil
}

// UserUpdates carries the optional patch for UpdateUser. Nil pointer
// fields are skipped (matches the Python kwargs whitelist of
// {email, name, status} at global_store.py:397).
type UserUpdates struct {
	Email  *string
	Name   *string
	Status *string
}

// UpdateUser applies a partial update. Returns true iff a row existed.
// Returns ErrAlreadyExists if the new email collides with another row.
//
// Mirrors `_sync_update_user` (global_store.py:396).
func (g *GlobalStore) UpdateUser(ctx context.Context, userID string, upd UserUpdates) (bool, error) {
	cols := []string{}
	args := []any{}
	if upd.Email != nil {
		cols = append(cols, "email = ?")
		args = append(args, *upd.Email)
	}
	if upd.Name != nil {
		cols = append(cols, "name = ?")
		args = append(args, *upd.Name)
	}
	if upd.Status != nil {
		cols = append(cols, "status = ?")
		args = append(args, *upd.Status)
	}
	if len(cols) == 0 {
		return false, nil
	}
	cols = append(cols, "updated_at = ?")
	args = append(args, g.now())
	args = append(args, userID)

	q := "UPDATE user_registry SET "
	for i, c := range cols {
		if i > 0 {
			q += ", "
		}
		q += c
	}
	q += " WHERE user_id = ?"

	res, err := g.db.ExecContext(ctx, q, args...)
	if err != nil {
		if isUniqueViolation(err) {
			return false, errs.Errorf(codes.AlreadyExists,
				"globalstore: update user %q would collide on email", userID)
		}
		return false, fmt.Errorf("globalstore: update user %q: %w", userID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n > 0, nil
}

// SetUserStatus is a thin wrapper over UpdateUser{Status: ...}. Kept as
// a distinct method so the gRPC layer can audit status transitions
// without parsing a struct field. Mirrors `_sync_set_user_status`
// (global_store.py:442).
func (g *GlobalStore) SetUserStatus(ctx context.Context, userID, status string) (bool, error) {
	now := g.now()
	res, err := g.db.ExecContext(ctx,
		`UPDATE user_registry SET status = ?, updated_at = ? WHERE user_id = ?`,
		status, now, userID,
	)
	if err != nil {
		return false, fmt.Errorf("globalstore: set user %q status: %w", userID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n > 0, nil
}

// ListUsers returns up to limit users with status, ordered by
// created_at (ASC, the Python default). limit <= 0 means "no upper
// bound"; SQLite accepts -1 as unlimited.
//
// Mirrors `_sync_list_users` (global_store.py:426).
func (g *GlobalStore) ListUsers(ctx context.Context, status string, limit, offset int) ([]*User, error) {
	if limit <= 0 {
		limit = -1
	}
	rows, err := g.db.QueryContext(ctx,
		`SELECT user_id, email, name, status, created_at, updated_at
		 FROM user_registry WHERE status = ?
		 ORDER BY created_at LIMIT ? OFFSET ?`,
		status, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: list users: %w", err)
	}
	defer rows.Close()
	out := []*User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.UserID, &u.Email, &u.Name, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("globalstore: scan user: %w", err)
		}
		out = append(out, &u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globalstore: iterate users: %w", err)
	}
	return out, nil
}

// DeleteUser is a soft delete: it sets status='deleted' (Python uses
// the same status enum at global_store.py:33). Returns true iff the row
// existed.
func (g *GlobalStore) DeleteUser(ctx context.Context, userID string) (bool, error) {
	return g.SetUserStatus(ctx, userID, "deleted")
}
