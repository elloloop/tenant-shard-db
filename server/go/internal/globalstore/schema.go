// Package globalstore owns the cross-tenant SQLite database (`global.db`).
//
// Spec: docs/go-port/shared/global-store.md. This file holds the schema
// DDL and idempotent migrations; CRUD lives in users.go, tenants.go, etc.
//
// Schema is created on every open via initSchema. Column additions guard
// themselves with PRAGMA table_info checks so older databases (predating
// the region column on tenant_registry, or the Phase 2 RPS columns on
// tenant_quotas) survive a Go-server upgrade.

package globalstore

import (
	"context"
	"database/sql"
	"fmt"
)

// schemaDDL is the canonical CREATE TABLE script. Every table is IF NOT
// EXISTS so concurrent first-open from sibling processes (test fixtures)
// is a no-op. Mirrors `_create_schema` in
const schemaDDL = `
CREATE TABLE IF NOT EXISTS user_registry (
    user_id     TEXT PRIMARY KEY,
    email       TEXT UNIQUE NOT NULL,
    name        TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS tenant_registry (
    tenant_id   TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  INTEGER NOT NULL,
    region      TEXT NOT NULL DEFAULT 'us-east-1'
);

CREATE TABLE IF NOT EXISTS tenant_members (
    tenant_id   TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'member',
    joined_at   INTEGER NOT NULL,
    PRIMARY KEY (tenant_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_members_user
    ON tenant_members(user_id);

CREATE TABLE IF NOT EXISTS shared_index (
    user_id       TEXT NOT NULL,
    source_tenant TEXT NOT NULL,
    node_id       TEXT NOT NULL,
    permission    TEXT NOT NULL,
    shared_at     INTEGER NOT NULL,
    PRIMARY KEY (user_id, source_tenant, node_id)
);

CREATE TABLE IF NOT EXISTS deletion_queue (
    user_id       TEXT PRIMARY KEY,
    requested_at  INTEGER NOT NULL,
    execute_at    INTEGER NOT NULL,
    export_path   TEXT,
    status        TEXT NOT NULL DEFAULT 'pending'
);

CREATE TABLE IF NOT EXISTS legal_holds (
    tenant_id   TEXT NOT NULL,
    held_by     TEXT NOT NULL,
    reason      TEXT NOT NULL,
    created_at  INTEGER NOT NULL,
    PRIMARY KEY (tenant_id, held_by)
);

CREATE TABLE IF NOT EXISTS tenant_quotas (
    tenant_id                  TEXT PRIMARY KEY,
    max_writes_per_month       INTEGER NOT NULL DEFAULT 0,
    hard_enforce               INTEGER NOT NULL DEFAULT 0,
    max_rps_sustained          INTEGER NOT NULL DEFAULT 0,
    max_rps_burst              INTEGER NOT NULL DEFAULT 0,
    max_rps_per_user_sustained INTEGER NOT NULL DEFAULT 0,
    max_rps_per_user_burst     INTEGER NOT NULL DEFAULT 0,
    updated_at                 INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS tenant_usage (
    tenant_id       TEXT PRIMARY KEY,
    period_start_ms INTEGER NOT NULL,
    writes_count    INTEGER NOT NULL DEFAULT 0,
    updated_at      INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS api_keys (
    key_id      TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL DEFAULT '',
    name        TEXT NOT NULL,
    hash        TEXT NOT NULL,
    scopes      TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  INTEGER NOT NULL,
    expires_at  INTEGER NOT NULL DEFAULT 0,
    revoked_at  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_api_keys_tenant
    ON api_keys(tenant_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_status
    ON api_keys(status);
`

// initSchema runs schemaDDL and the table-info-guarded ALTERs. Safe to
// call on every open. Errors are wrapped with the failing step so an
// operator can pinpoint a corrupt-DB scenario.
func initSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schemaDDL); err != nil {
		return fmt.Errorf("globalstore: create schema: %w", err)
	}
	if err := migrateTenantRegistryRegion(ctx, db); err != nil {
		return err
	}
	if err := migrateTenantQuotas(ctx, db); err != nil {
		return err
	}
	return nil
}

// migrateTenantRegistryRegion adds the region column to tenant_registry
// when a pre-region database is opened. Mirrors
// `_migrate_tenant_registry_region` in global_store.py:187.
func migrateTenantRegistryRegion(ctx context.Context, db *sql.DB) error {
	cols, err := tableColumns(ctx, db, "tenant_registry")
	if err != nil {
		return err
	}
	if _, ok := cols["region"]; ok {
		return nil
	}
	_, err = db.ExecContext(ctx,
		`ALTER TABLE tenant_registry ADD COLUMN region TEXT NOT NULL DEFAULT 'us-east-1'`,
	)
	if err != nil {
		return fmt.Errorf("globalstore: migrate tenant_registry.region: %w", err)
	}
	return nil
}

// migrateTenantQuotas adds the Phase 2/3 token-bucket columns when a
// pre-Phase-2 database is opened. Mirrors `_migrate_tenant_quotas` in
// global_store.py:280.
func migrateTenantQuotas(ctx context.Context, db *sql.DB) error {
	cols, err := tableColumns(ctx, db, "tenant_quotas")
	if err != nil {
		return err
	}
	for _, col := range []string{
		"max_rps_sustained",
		"max_rps_burst",
		"max_rps_per_user_sustained",
		"max_rps_per_user_burst",
	} {
		if _, ok := cols[col]; ok {
			continue
		}
		stmt := fmt.Sprintf(
			`ALTER TABLE tenant_quotas ADD COLUMN %s INTEGER NOT NULL DEFAULT 0`,
			col,
		)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("globalstore: migrate tenant_quotas.%s: %w", col, err)
		}
	}
	return nil
}

// tableColumns returns the set of column names for a given SQLite table
// via PRAGMA table_info. Returns an empty map (no error) if the table
// does not exist; callers treat that as "no columns to compare against".
func tableColumns(ctx context.Context, db *sql.DB, table string) (map[string]struct{}, error) {
	// PRAGMA does not bind parameters; the table name is a constant in
	// every call site so this is safe (no user input).
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, fmt.Errorf("globalstore: pragma table_info(%s): %w", table, err)
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("globalstore: scan pragma table_info(%s): %w", table, err)
		}
		out[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globalstore: iterate pragma table_info(%s): %w", table, err)
	}
	return out, nil
}
