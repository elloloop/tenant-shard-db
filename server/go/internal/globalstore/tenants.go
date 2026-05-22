// tenant_registry CRUD: create_tenant through set_tenant_status /
// set_legal_hold. Region is part of every returned row; we default to
// "us-east-1" when callers don't supply one.

package globalstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"google.golang.org/grpc/codes"
)

// DefaultRegion is the region pin assigned when CreateTenant is called
// with an empty Region.
const DefaultRegion = "us-east-1"

// CreateTenant inserts a new tenant_registry row. Returns
// ErrAlreadyExists on duplicate tenant_id.
//
// Empty `region` is filled with DefaultRegion.
func (g *GlobalStore) CreateTenant(ctx context.Context, tenantID, name, region string) (*Tenant, error) {
	if region == "" {
		region = DefaultRegion
	}
	now := g.now()
	_, err := g.db.ExecContext(ctx,
		`INSERT INTO tenant_registry (tenant_id, name, status, created_at, region)
		 VALUES (?, ?, 'active', ?, ?)`,
		tenantID, name, now, region,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errs.Errorf(codes.AlreadyExists,
				"globalstore: tenant %q already exists", tenantID)
		}
		return nil, fmt.Errorf("globalstore: create tenant %q: %w", tenantID, err)
	}
	return &Tenant{
		TenantID:  tenantID,
		Name:      name,
		Status:    "active",
		CreatedAt: now,
		Region:    region,
	}, nil
}

// CreateTenantWithOwner atomically inserts the tenant_registry row and
// the creator's tenant_members owner row in a single SQLite transaction.
// Either both land or neither does — the orphan-tenant hazard
// (registry row written without an owner if the process crashed between
// the two INSERTs) is closed here.
//
// Returns ErrAlreadyExists on duplicate tenant_id; the membership insert
// is not attempted in that case. Empty `region` is filled with
// DefaultRegion.
func (g *GlobalStore) CreateTenantWithOwner(ctx context.Context, tenantID, name, region, ownerUserID string) (*Tenant, error) {
	if region == "" {
		region = DefaultRegion
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("globalstore: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := g.now()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO tenant_registry (tenant_id, name, status, created_at, region)
		 VALUES (?, ?, 'active', ?, ?)`,
		tenantID, name, now, region,
	); err != nil {
		if isUniqueViolation(err) {
			return nil, errs.Errorf(codes.AlreadyExists,
				"globalstore: tenant %q already exists", tenantID)
		}
		return nil, fmt.Errorf("globalstore: create tenant %q: %w", tenantID, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO tenant_members (tenant_id, user_id, role, joined_at)
		 VALUES (?, ?, 'owner', ?)`,
		tenantID, ownerUserID, now,
	); err != nil {
		return nil, fmt.Errorf("globalstore: add owner member (%q,%q): %w", tenantID, ownerUserID, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("globalstore: commit create tenant %q: %w", tenantID, err)
	}
	return &Tenant{
		TenantID:  tenantID,
		Name:      name,
		Status:    "active",
		CreatedAt: now,
		Region:    region,
	}, nil
}

// GetTenant returns the row, or (nil, nil) if not present.
func (g *GlobalStore) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	row := g.db.QueryRowContext(ctx,
		`SELECT tenant_id, name, status, created_at, region
		 FROM tenant_registry WHERE tenant_id = ?`,
		tenantID,
	)
	var t Tenant
	if err := row.Scan(&t.TenantID, &t.Name, &t.Status, &t.CreatedAt, &t.Region); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("globalstore: get tenant %q: %w", tenantID, err)
	}
	return &t, nil
}

// ListTenants returns all tenants with the given status, ordered by
// created_at. No pagination.
func (g *GlobalStore) ListTenants(ctx context.Context, status string) ([]*Tenant, error) {
	rows, err := g.db.QueryContext(ctx,
		`SELECT tenant_id, name, status, created_at, region
		 FROM tenant_registry WHERE status = ? ORDER BY created_at`,
		status,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: list tenants: %w", err)
	}
	defer rows.Close()
	out := []*Tenant{}
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.TenantID, &t.Name, &t.Status, &t.CreatedAt, &t.Region); err != nil {
			return nil, fmt.Errorf("globalstore: scan tenant: %w", err)
		}
		out = append(out, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globalstore: iterate tenants: %w", err)
	}
	return out, nil
}

// SetTenantStatus updates the status column. Returns true iff a row
// existed.
func (g *GlobalStore) SetTenantStatus(ctx context.Context, tenantID, status string) (bool, error) {
	res, err := g.db.ExecContext(ctx,
		`UPDATE tenant_registry SET status = ? WHERE tenant_id = ?`,
		status, tenantID,
	)
	if err != nil {
		return false, fmt.Errorf("globalstore: set tenant %q status: %w", tenantID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n > 0, nil
}

// ArchiveTenant flips status to 'archived'. Convenience wrapper around
// SetTenantStatus so the gRPC handler can audit "archive" as a
// dedicated event.
func (g *GlobalStore) ArchiveTenant(ctx context.Context, tenantID string) (bool, error) {
	return g.SetTenantStatus(ctx, tenantID, "archived")
}

// SetLegalHoldStatus toggles the tenant_registry.status flag between
// 'legal_hold' and 'active'. This is the *gating* flag — the
// `legal_holds` table (see legalhold.go) records the *informational*
// audit trail.
func (g *GlobalStore) SetLegalHoldStatus(ctx context.Context, tenantID string, enabled bool) (bool, error) {
	status := "active"
	if enabled {
		status = "legal_hold"
	}
	return g.SetTenantStatus(ctx, tenantID, status)
}
