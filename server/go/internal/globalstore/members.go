// tenant_members CRUD: add_member through change_role. Composite PK is
// (tenant_id, user_id); a secondary index on user_id makes
// GetUserTenants cheap.

package globalstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"google.golang.org/grpc/codes"
)

// AddTenantMember inserts a (tenant_id, user_id, role) row. Empty role
// defaults to "member" (matches Python). Returns ErrAlreadyExists if
// the membership already exists — Python lets the IntegrityError
// bubble; the Go boundary code wants a typed error.
func (g *GlobalStore) AddTenantMember(ctx context.Context, tenantID, userID, role string) error {
	if role == "" {
		role = "member"
	}
	now := g.now()
	_, err := g.db.ExecContext(ctx,
		`INSERT INTO tenant_members (tenant_id, user_id, role, joined_at)
		 VALUES (?, ?, ?, ?)`,
		tenantID, userID, role, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return errs.Errorf(codes.AlreadyExists,
				"globalstore: %q is already a member of %q", userID, tenantID)
		}
		return fmt.Errorf("globalstore: add member (%q,%q): %w", tenantID, userID, err)
	}
	return nil
}

// RemoveTenantMember deletes the (tenant_id, user_id) row. Returns
// true iff a row was removed.
func (g *GlobalStore) RemoveTenantMember(ctx context.Context, tenantID, userID string) (bool, error) {
	res, err := g.db.ExecContext(ctx,
		`DELETE FROM tenant_members WHERE tenant_id = ? AND user_id = ?`,
		tenantID, userID,
	)
	if err != nil {
		return false, fmt.Errorf("globalstore: remove member (%q,%q): %w", tenantID, userID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n > 0, nil
}

// GetTenantMembers lists all members of a tenant, ordered by joined_at.
func (g *GlobalStore) GetTenantMembers(ctx context.Context, tenantID string) ([]*Member, error) {
	rows, err := g.db.QueryContext(ctx,
		`SELECT tenant_id, user_id, role, joined_at
		 FROM tenant_members WHERE tenant_id = ? ORDER BY joined_at`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: list tenant members: %w", err)
	}
	defer rows.Close()
	out := []*Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.TenantID, &m.UserID, &m.Role, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("globalstore: scan member: %w", err)
		}
		out = append(out, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globalstore: iterate members: %w", err)
	}
	return out, nil
}

// GetUserTenants lists all tenants a user belongs to, ordered by
// joined_at.
func (g *GlobalStore) GetUserTenants(ctx context.Context, userID string) ([]*Member, error) {
	rows, err := g.db.QueryContext(ctx,
		`SELECT tenant_id, user_id, role, joined_at
		 FROM tenant_members WHERE user_id = ? ORDER BY joined_at`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: list user tenants: %w", err)
	}
	defer rows.Close()
	out := []*Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.TenantID, &m.UserID, &m.Role, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("globalstore: scan member: %w", err)
		}
		out = append(out, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globalstore: iterate user tenants: %w", err)
	}
	return out, nil
}

// ChangeMemberRole updates the role for an existing membership. Returns
// true iff the row existed.
func (g *GlobalStore) ChangeMemberRole(ctx context.Context, tenantID, userID, role string) (bool, error) {
	res, err := g.db.ExecContext(ctx,
		`UPDATE tenant_members SET role = ? WHERE tenant_id = ? AND user_id = ?`,
		role, tenantID, userID,
	)
	if err != nil {
		return false, fmt.Errorf("globalstore: change role (%q,%q): %w", tenantID, userID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n > 0, nil
}

// IsMember is the hot-path membership check used by ExecuteAtomic and
// ListTenants.
func (g *GlobalStore) IsMember(ctx context.Context, tenantID, userID string) (bool, error) {
	var one int
	err := g.db.QueryRowContext(ctx,
		`SELECT 1 FROM tenant_members WHERE tenant_id = ? AND user_id = ?`,
		tenantID, userID,
	).Scan(&one)
	if err != nil {
		// sql.ErrNoRows is the "not a member" signal.
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("globalstore: is_member (%q,%q): %w", tenantID, userID, err)
	}
	return true, nil
}

// RemoveAllMembershipsForUser deletes every tenant_members row whose
// user_id matches. Returns the number of rows deleted. Used by GDPR
// account deletion.
func (g *GlobalStore) RemoveAllMembershipsForUser(ctx context.Context, userID string) (int64, error) {
	res, err := g.db.ExecContext(ctx,
		`DELETE FROM tenant_members WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		return 0, fmt.Errorf("globalstore: remove all memberships for %q: %w", userID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n, nil
}

// TransferUserContent ensures `toUser` is a member of `tenantID` (adds
// with role='member' if absent) and reports whether a new membership
// was created. Note: this only touches tenant_members; the per-tenant
// node ownership transfer is a separate call.
func (g *GlobalStore) TransferUserContent(ctx context.Context, tenantID, fromUser, toUser string) (*TransferResult, error) {
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("globalstore: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existing int
	err = tx.QueryRowContext(ctx,
		`SELECT 1 FROM tenant_members WHERE tenant_id = ? AND user_id = ?`,
		tenantID, toUser,
	).Scan(&existing)
	created := false
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("globalstore: probe member (%q,%q): %w", tenantID, toUser, err)
		}
		// Insert the new membership.
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO tenant_members (tenant_id, user_id, role, joined_at)
			 VALUES (?, ?, 'member', ?)`,
			tenantID, toUser, g.now(),
		); err != nil {
			return nil, fmt.Errorf("globalstore: insert transfer member: %w", err)
		}
		created = true
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("globalstore: commit transfer: %w", err)
	}
	return &TransferResult{
		TenantID:          tenantID,
		FromUser:          fromUser,
		ToUser:            toUser,
		MembershipCreated: created,
	}, nil
}
