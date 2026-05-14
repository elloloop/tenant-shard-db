// SPDX-License-Identifier: AGPL-3.0-only

package globalstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

// TenantCreatedApply is the full global row state carried by a
// tenant_created WAL op.
type TenantCreatedApply struct {
	TenantID    string
	Name        string
	Region      string
	OwnerUserID string
	CreatedAt   int64
}

// UserApply is the full user_registry row state carried by user create
// and update WAL ops.
type UserApply struct {
	UserID    string
	Email     string
	Name      string
	Status    string
	CreatedAt int64
	UpdatedAt int64
}

// MemberApply is the full tenant_members row state carried by member
// create / role-change WAL ops.
type MemberApply struct {
	TenantID string
	UserID   string
	Role     string
	JoinedAt int64
}

// DeletionApply is the full deletion_queue row state carried by a
// user_deletion_scheduled WAL op.
type DeletionApply struct {
	UserID      string
	RequestedAt int64
	ExecuteAt   int64
	Status      string
}

// LegalHoldApply is the state carried by a legal_hold_set WAL op.
type LegalHoldApply struct {
	TenantID  string
	HeldBy    string
	Reason    string
	CreatedAt int64
	Enabled   bool
}

// ErrApplyConflict marks a deterministic global WAL conflict: the event
// was valid, but another event already materialized incompatible state.
var ErrApplyConflict = fmt.Errorf("%w: global apply conflict", errs.ErrAlreadyExists)

// ApplyTenantCreated idempotently materializes a tenant registry row
// and the creator's owner membership in a single globalstore
// transaction.
func (g *GlobalStore) ApplyTenantCreated(ctx context.Context, in TenantCreatedApply) (*Tenant, error) {
	if in.Region == "" {
		in.Region = DefaultRegion
	}
	if in.CreatedAt == 0 {
		in.CreatedAt = g.now()
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("globalstore: apply tenant_created begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tenant_registry (tenant_id, name, status, created_at, region)
		VALUES (?, ?, 'active', ?, ?)`,
		in.TenantID, in.Name, in.CreatedAt, in.Region,
	); err != nil {
		if isUniqueViolation(err) {
			match, merr := tenantCreatedRowMatchesTx(ctx, tx, in)
			if merr != nil {
				return nil, merr
			}
			if !match {
				return nil, fmt.Errorf("%w: tenant %q already exists", ErrApplyConflict, in.TenantID)
			}
		} else {
			return nil, fmt.Errorf("globalstore: apply tenant_created tenant: %w", err)
		}
	}
	if in.OwnerUserID != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO tenant_members (tenant_id, user_id, role, joined_at)
			VALUES (?, ?, 'owner', ?)`,
			in.TenantID, in.OwnerUserID, in.CreatedAt,
		); err != nil {
			if isUniqueViolation(err) {
				match, merr := memberRowMatchesTx(ctx, tx, MemberApply{
					TenantID: in.TenantID,
					UserID:   in.OwnerUserID,
					Role:     "owner",
					JoinedAt: in.CreatedAt,
				})
				if merr != nil {
					return nil, merr
				}
				if !match {
					return nil, fmt.Errorf("%w: owner %q already exists in tenant %q", ErrApplyConflict, in.OwnerUserID, in.TenantID)
				}
			} else {
				return nil, fmt.Errorf("globalstore: apply tenant_created owner: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("globalstore: apply tenant_created commit: %w", err)
	}
	return &Tenant{
		TenantID:  in.TenantID,
		Name:      in.Name,
		Status:    "active",
		CreatedAt: in.CreatedAt,
		Region:    in.Region,
	}, nil
}

// ApplyUserCreated idempotently materializes a user_registry row.
func (g *GlobalStore) ApplyUserCreated(ctx context.Context, in UserApply) (*User, error) {
	normalizeUserApply(g, &in)
	if _, err := g.db.ExecContext(ctx, `
		INSERT INTO user_registry (user_id, email, name, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		in.UserID, in.Email, in.Name, in.Status, in.CreatedAt, in.UpdatedAt,
	); err != nil {
		if isUniqueViolation(err) {
			existing, getErr := g.GetUser(ctx, in.UserID)
			if getErr != nil {
				return nil, getErr
			}
			if existing != nil && userApplyMatches(existing, in) {
				return existing, nil
			}
			return nil, fmt.Errorf("%w: user %q or email %q already exists", ErrApplyConflict, in.UserID, in.Email)
		}
		return nil, fmt.Errorf("globalstore: apply user_created: %w", err)
	}
	return &User{
		UserID:    in.UserID,
		Email:     in.Email,
		Name:      in.Name,
		Status:    in.Status,
		CreatedAt: in.CreatedAt,
		UpdatedAt: in.UpdatedAt,
	}, nil
}

// ApplyUserUpdated applies the complete user_registry row carried in
// the WAL. Missing rows are deterministic precondition failures; exact
// duplicate rows are treated as replay success.
func (g *GlobalStore) ApplyUserUpdated(ctx context.Context, in UserApply) error {
	normalizeUserApply(g, &in)
	res, err := g.db.ExecContext(ctx, `
		UPDATE user_registry
		SET email = ?, name = ?, status = ?, updated_at = ?
		WHERE user_id = ?`,
		in.Email, in.Name, in.Status, in.UpdatedAt, in.UserID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: email %q already exists", ErrApplyConflict, in.Email)
		}
		return fmt.Errorf("globalstore: apply user_updated: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("globalstore: apply user_updated rows: %w", err)
	}
	if n == 0 {
		existing, getErr := g.GetUser(ctx, in.UserID)
		if getErr != nil {
			return getErr
		}
		if existing != nil && userApplyMatches(existing, in) {
			return nil
		}
		return fmt.Errorf("%w: user %q not found", errs.ErrFailedPrecondition, in.UserID)
	}
	return nil
}

// ApplyMemberAdded idempotently inserts a tenant_members row.
func (g *GlobalStore) ApplyMemberAdded(ctx context.Context, in MemberApply) error {
	normalizeMemberApply(g, &in)
	_, err := g.db.ExecContext(ctx, `
		INSERT INTO tenant_members (tenant_id, user_id, role, joined_at)
		VALUES (?, ?, ?, ?)`,
		in.TenantID, in.UserID, in.Role, in.JoinedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			match, merr := memberRowMatches(ctx, g.db, in)
			if merr != nil {
				return merr
			}
			if match {
				return nil
			}
			return fmt.Errorf("%w: user %q already member of tenant %q", ErrApplyConflict, in.UserID, in.TenantID)
		}
		return fmt.Errorf("globalstore: apply member_added: %w", err)
	}
	return nil
}

// ApplyMemberRemoved idempotently deletes a tenant_members row.
func (g *GlobalStore) ApplyMemberRemoved(ctx context.Context, tenantID, userID string) error {
	if _, err := g.db.ExecContext(ctx,
		`DELETE FROM tenant_members WHERE tenant_id = ? AND user_id = ?`,
		tenantID, userID,
	); err != nil {
		return fmt.Errorf("globalstore: apply member_removed: %w", err)
	}
	return nil
}

// ApplyMemberRoleChanged applies the role carried in the WAL. Missing
// rows are deterministic precondition failures; exact duplicate rows
// are treated as replay success.
func (g *GlobalStore) ApplyMemberRoleChanged(ctx context.Context, in MemberApply) error {
	normalizeMemberApply(g, &in)
	res, err := g.db.ExecContext(ctx, `
		UPDATE tenant_members SET role = ?
		WHERE tenant_id = ? AND user_id = ?`,
		in.Role, in.TenantID, in.UserID,
	)
	if err != nil {
		return fmt.Errorf("globalstore: apply member_role_changed: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("globalstore: apply member_role_changed rows: %w", err)
	}
	if n == 0 {
		match, merr := memberRoleMatches(ctx, g.db, in.TenantID, in.UserID, in.Role)
		if merr != nil {
			return merr
		}
		if match {
			return nil
		}
		return fmt.Errorf("%w: member %q not found in tenant %q", errs.ErrFailedPrecondition, in.UserID, in.TenantID)
	}
	return nil
}

// ApplyTenantArchived idempotently flips tenant_registry.status to
// archived. Missing rows are a no-op on replay.
func (g *GlobalStore) ApplyTenantArchived(ctx context.Context, tenantID string) error {
	if _, err := g.db.ExecContext(ctx,
		`UPDATE tenant_registry SET status = 'archived' WHERE tenant_id = ?`,
		tenantID,
	); err != nil {
		return fmt.Errorf("globalstore: apply tenant_archived: %w", err)
	}
	return nil
}

// ApplyLegalHoldSet materializes the legal_holds audit row and the
// tenant_registry gating status in one globalstore transaction.
func (g *GlobalStore) ApplyLegalHoldSet(ctx context.Context, in LegalHoldApply) error {
	if in.CreatedAt == 0 {
		in.CreatedAt = g.now()
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("globalstore: apply legal_hold_set begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	status := "active"
	if in.Enabled {
		status = "legal_hold"
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO legal_holds (tenant_id, held_by, reason, created_at)
			VALUES (?, ?, ?, ?)`,
			in.TenantID, in.HeldBy, in.Reason, in.CreatedAt,
		); err != nil {
			return fmt.Errorf("globalstore: apply legal_hold_set insert: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM legal_holds WHERE tenant_id = ? AND held_by = ?`,
			in.TenantID, in.HeldBy,
		); err != nil {
			return fmt.Errorf("globalstore: apply legal_hold_set clear: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE tenant_registry SET status = ? WHERE tenant_id = ?`,
		status, in.TenantID,
	); err != nil {
		return fmt.Errorf("globalstore: apply legal_hold_set status: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("globalstore: apply legal_hold_set commit: %w", err)
	}
	return nil
}

// ApplyUserDeletionScheduled idempotently materializes the deletion
// queue row and pending_deletion user status.
func (g *GlobalStore) ApplyUserDeletionScheduled(ctx context.Context, in DeletionApply) error {
	if in.RequestedAt == 0 {
		in.RequestedAt = g.now()
	}
	if in.ExecuteAt == 0 {
		in.ExecuteAt = in.RequestedAt
	}
	if in.Status == "" {
		in.Status = "pending"
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("globalstore: apply user_deletion_scheduled begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO deletion_queue (user_id, requested_at, execute_at, status)
		VALUES (?, ?, ?, ?)`,
		in.UserID, in.RequestedAt, in.ExecuteAt, in.Status,
	); err != nil {
		return fmt.Errorf("globalstore: apply user_deletion_scheduled queue: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE user_registry SET status = 'pending_deletion', updated_at = ? WHERE user_id = ?`,
		in.RequestedAt, in.UserID,
	); err != nil {
		return fmt.Errorf("globalstore: apply user_deletion_scheduled status: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("globalstore: apply user_deletion_scheduled commit: %w", err)
	}
	return nil
}

// ApplyUserDeletionCanceled idempotently removes a pending deletion
// queue row and restores the user status to active when a row existed.
func (g *GlobalStore) ApplyUserDeletionCanceled(ctx context.Context, userID string, updatedAt int64) error {
	if updatedAt == 0 {
		updatedAt = g.now()
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("globalstore: apply user_deletion_canceled begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx,
		`DELETE FROM deletion_queue WHERE user_id = ? AND status = 'pending'`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("globalstore: apply user_deletion_canceled delete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("globalstore: apply user_deletion_canceled rows: %w", err)
	}
	if n > 0 {
		if _, err := tx.ExecContext(ctx,
			`UPDATE user_registry SET status = 'active', updated_at = ? WHERE user_id = ?`,
			updatedAt, userID,
		); err != nil {
			return fmt.Errorf("globalstore: apply user_deletion_canceled status: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("globalstore: apply user_deletion_canceled commit: %w", err)
	}
	return nil
}

// ApplyUserFrozen idempotently sets user_registry.status.
func (g *GlobalStore) ApplyUserFrozen(ctx context.Context, userID, status string, updatedAt int64) error {
	if updatedAt == 0 {
		updatedAt = g.now()
	}
	if status == "" {
		status = "active"
	}
	if _, err := g.db.ExecContext(ctx,
		`UPDATE user_registry SET status = ?, updated_at = ? WHERE user_id = ?`,
		status, updatedAt, userID,
	); err != nil {
		return fmt.Errorf("globalstore: apply user_frozen: %w", err)
	}
	return nil
}

// ApplyAccessTransferred idempotently ensures the recipient is a
// tenant member. Per-tenant node ownership changes are handled by the
// tenant-scoped admin_transfer_content op.
func (g *GlobalStore) ApplyAccessTransferred(ctx context.Context, tenantID, toUser string, joinedAt int64) error {
	if joinedAt == 0 {
		joinedAt = g.now()
	}
	if _, err := g.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO tenant_members (tenant_id, user_id, role, joined_at)
		VALUES (?, ?, 'member', ?)`,
		tenantID, toUser, joinedAt,
	); err != nil {
		return fmt.Errorf("globalstore: apply access_transferred: %w", err)
	}
	return nil
}

// ApplyAccessRevoked idempotently removes shared_index rows for a user
// within one source tenant. It deliberately does not remove
// tenant_members; RevokeAllUserAccess revokes grants and shares, not
// tenant membership.
func (g *GlobalStore) ApplyAccessRevoked(ctx context.Context, tenantID, userID string) error {
	if _, err := g.db.ExecContext(ctx,
		`DELETE FROM shared_index WHERE user_id = ? AND source_tenant = ?`,
		userID, tenantID,
	); err != nil {
		return fmt.Errorf("globalstore: apply access_revoked: %w", err)
	}
	return nil
}

func normalizeUserApply(g *GlobalStore, in *UserApply) {
	now := g.now()
	if in.Status == "" {
		in.Status = "active"
	}
	if in.CreatedAt == 0 {
		in.CreatedAt = now
	}
	if in.UpdatedAt == 0 {
		in.UpdatedAt = in.CreatedAt
	}
}

func normalizeMemberApply(g *GlobalStore, in *MemberApply) {
	if in.Role == "" {
		in.Role = "member"
	}
	if in.JoinedAt == 0 {
		in.JoinedAt = g.now()
	}
}

func tenantCreatedRowMatchesTx(ctx context.Context, tx *sql.Tx, in TenantCreatedApply) (bool, error) {
	var name, status, region string
	var createdAt int64
	err := tx.QueryRowContext(ctx, `
		SELECT name, status, created_at, region
		FROM tenant_registry
		WHERE tenant_id = ?`,
		in.TenantID,
	).Scan(&name, &status, &createdAt, &region)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("globalstore: apply tenant_created existing tenant: %w", err)
	}
	return name == in.Name &&
		status == "active" &&
		createdAt == in.CreatedAt &&
		region == in.Region, nil
}

type memberRowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func memberRowMatches(ctx context.Context, q memberRowQuerier, in MemberApply) (bool, error) {
	var role string
	var joinedAt int64
	err := q.QueryRowContext(ctx, `
		SELECT role, joined_at
		FROM tenant_members
		WHERE tenant_id = ? AND user_id = ?`,
		in.TenantID, in.UserID,
	).Scan(&role, &joinedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("globalstore: apply member existing row: %w", err)
	}
	return role == in.Role && joinedAt == in.JoinedAt, nil
}

func memberRowMatchesTx(ctx context.Context, tx *sql.Tx, in MemberApply) (bool, error) {
	return memberRowMatches(ctx, tx, in)
}

func memberRoleMatches(ctx context.Context, q memberRowQuerier, tenantID, userID, role string) (bool, error) {
	var existing string
	err := q.QueryRowContext(ctx,
		`SELECT role FROM tenant_members WHERE tenant_id = ? AND user_id = ?`,
		tenantID, userID,
	).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("globalstore: apply member role existing row: %w", err)
	}
	return existing == role, nil
}

func userApplyMatches(u *User, in UserApply) bool {
	return u.UserID == in.UserID &&
		u.Email == in.Email &&
		u.Name == in.Name &&
		u.Status == in.Status &&
		u.CreatedAt == in.CreatedAt &&
		u.UpdatedAt == in.UpdatedAt
}
