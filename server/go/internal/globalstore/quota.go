// tenant_quotas + tenant_usage CRUD. Mirrors the Python helpers at
// through :1333 (reset_period).
//
// IMPORTANT: tenant_usage.period_start_ms is the start of the current
// UTC calendar month in MILLISECONDS. Every other timestamp in
// globalstore is Unix-epoch SECONDS — the spec calls this out at
// docs/go-port/shared/global-store.md "all timestamps are integer Unix
// epoch seconds unless suffixed `_ms`".
//
// IncrementUsage is the hot-path counter. It folds the calendar-month
// rollover into a single transaction so concurrent callers cannot
// race; combined with MaxOpenConns(1) on the connection pool, two
// IncrementUsage calls cannot interleave.

package globalstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// calendarMonthStartMs returns the Unix-millisecond timestamp of the
// start of the UTC calendar month containing `t`.
func calendarMonthStartMs(t time.Time) int64 {
	t = t.UTC()
	start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start.UnixMilli()
}

// nowForUsage returns (now-second, current-month-start-ms) using the
// store's injected clock. Tests that drive the calendar-rollover path
// inject NowFn to advance Time.
func (g *GlobalStore) nowForUsage() (int64, int64) {
	now := g.now()
	t := time.Unix(now, 0).UTC()
	return now, calendarMonthStartMs(t)
}

// SetTenantQuota upserts the tenant_quotas row.
func (g *GlobalStore) SetTenantQuota(ctx context.Context, q QuotaConfig) (*QuotaConfig, error) {
	now := g.now()
	hard := int64(0)
	if q.HardEnforce {
		hard = 1
	}
	_, err := g.db.ExecContext(ctx,
		`INSERT INTO tenant_quotas
		     (tenant_id, max_writes_per_month, hard_enforce,
		      max_rps_sustained, max_rps_burst,
		      max_rps_per_user_sustained, max_rps_per_user_burst,
		      updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(tenant_id) DO UPDATE SET
		     max_writes_per_month       = excluded.max_writes_per_month,
		     hard_enforce               = excluded.hard_enforce,
		     max_rps_sustained          = excluded.max_rps_sustained,
		     max_rps_burst              = excluded.max_rps_burst,
		     max_rps_per_user_sustained = excluded.max_rps_per_user_sustained,
		     max_rps_per_user_burst     = excluded.max_rps_per_user_burst,
		     updated_at                 = excluded.updated_at`,
		q.TenantID,
		q.MaxWritesPerMonth,
		hard,
		q.MaxRPSSustained,
		q.MaxRPSBurst,
		q.MaxRPSPerUserSustained,
		q.MaxRPSPerUserBurst,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: set quota %q: %w", q.TenantID, err)
	}
	out := q
	out.UpdatedAt = now
	return &out, nil
}

// GetTenantQuota returns the row, or (nil, nil) if no config has been
// set. A missing row is treated as "unlimited".
func (g *GlobalStore) GetTenantQuota(ctx context.Context, tenantID string) (*QuotaConfig, error) {
	row := g.db.QueryRowContext(ctx,
		`SELECT tenant_id, max_writes_per_month, hard_enforce,
		        max_rps_sustained, max_rps_burst,
		        max_rps_per_user_sustained, max_rps_per_user_burst,
		        updated_at
		 FROM tenant_quotas WHERE tenant_id = ?`,
		tenantID,
	)
	var q QuotaConfig
	var hard int64
	if err := row.Scan(
		&q.TenantID,
		&q.MaxWritesPerMonth,
		&hard,
		&q.MaxRPSSustained,
		&q.MaxRPSBurst,
		&q.MaxRPSPerUserSustained,
		&q.MaxRPSPerUserBurst,
		&q.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("globalstore: get quota %q: %w", tenantID, err)
	}
	q.HardEnforce = hard != 0
	return &q, nil
}

// GetUsage returns the current-period usage row, or a zero-initialized
// usage with the current calendar-month period_start_ms if no row
// exists (callers treat missing as "new tenant, unused"). Not persisted.
func (g *GlobalStore) GetUsage(ctx context.Context, tenantID string) (*Usage, error) {
	row := g.db.QueryRowContext(ctx,
		`SELECT tenant_id, period_start_ms, writes_count, updated_at
		 FROM tenant_usage WHERE tenant_id = ?`,
		tenantID,
	)
	var u Usage
	if err := row.Scan(&u.TenantID, &u.PeriodStartMs, &u.WritesCount, &u.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, periodStart := g.nowForUsage()
			return &Usage{
				TenantID:      tenantID,
				PeriodStartMs: periodStart,
				WritesCount:   0,
				UpdatedAt:     0,
			}, nil
		}
		return nil, fmt.Errorf("globalstore: get usage %q: %w", tenantID, err)
	}
	return &u, nil
}

// IncrementUsage adds `nWrites` to the rolling write counter, with
// calendar-month rollover folded in. nWrites <= 0 is a no-op that returns
// the current usage.
//
// Atomicity: this runs in a single transaction, but combined with the
// pool's MaxOpenConns(1) two callers can't interleave even on the read
// path; serialized at the connection level rather than relying on
// SQLite optimistic concurrency.
func (g *GlobalStore) IncrementUsage(ctx context.Context, tenantID string, nWrites int64) (*Usage, error) {
	if nWrites <= 0 {
		return g.GetUsage(ctx, tenantID)
	}
	now, periodStart := g.nowForUsage()

	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("globalstore: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		curStart int64
		curCount int64
	)
	err = tx.QueryRowContext(ctx,
		`SELECT period_start_ms, writes_count FROM tenant_usage WHERE tenant_id = ?`,
		tenantID,
	).Scan(&curStart, &curCount)
	if errors.Is(err, sql.ErrNoRows) {
		// First write of this tenant.
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO tenant_usage (tenant_id, period_start_ms, writes_count, updated_at)
			 VALUES (?, ?, ?, ?)`,
			tenantID, periodStart, nWrites, now,
		); err != nil {
			return nil, fmt.Errorf("globalstore: insert usage %q: %w", tenantID, err)
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("globalstore: commit usage: %w", err)
		}
		return &Usage{
			TenantID:      tenantID,
			PeriodStartMs: periodStart,
			WritesCount:   nWrites,
			UpdatedAt:     now,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("globalstore: read usage %q: %w", tenantID, err)
	}

	// Rollover detection: if the stored period predates the current
	// month, reset the counter to nWrites and bump period_start_ms.
	newCount := curCount + nWrites
	newStart := curStart
	if curStart < periodStart {
		newCount = nWrites
		newStart = periodStart
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE tenant_usage
		 SET period_start_ms = ?, writes_count = ?, updated_at = ?
		 WHERE tenant_id = ?`,
		newStart, newCount, now, tenantID,
	); err != nil {
		return nil, fmt.Errorf("globalstore: update usage %q: %w", tenantID, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("globalstore: commit usage: %w", err)
	}
	return &Usage{
		TenantID:      tenantID,
		PeriodStartMs: newStart,
		WritesCount:   newCount,
		UpdatedAt:     now,
	}, nil
}

// ResetUsage forces a fresh period for a tenant (writes_count=0,
// period_start_ms=current month). Used by tests and ops.
func (g *GlobalStore) ResetUsage(ctx context.Context, tenantID string) (*Usage, error) {
	now, periodStart := g.nowForUsage()
	_, err := g.db.ExecContext(ctx,
		`INSERT INTO tenant_usage (tenant_id, period_start_ms, writes_count, updated_at)
		 VALUES (?, ?, 0, ?)
		 ON CONFLICT(tenant_id) DO UPDATE SET
		     period_start_ms = excluded.period_start_ms,
		     writes_count    = 0,
		     updated_at      = excluded.updated_at`,
		tenantID, periodStart, now,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: reset usage %q: %w", tenantID, err)
	}
	return &Usage{
		TenantID:      tenantID,
		PeriodStartMs: periodStart,
		WritesCount:   0,
		UpdatedAt:     now,
	}, nil
}
