package store

import (
	"context"
	"database/sql"
	"fmt"
)

// schemaDDL mirrors canonical_store.py:1037-1200 _create_schema. Every
// table is IF NOT EXISTS so that running initSchema twice is a no-op.
//
// Tables omitted from the Wave 1 cut (kept in Python today):
//
//   - notifications, read_cursors: stubs only (notifications.go)
//   - audit_log: superseded by S3 Object Lock (CLAUDE.md invariant #2)
//   - type_metadata: deferred to W1.10 (applier responsibility)
//   - schema_version: kept here for forward-compat with Python read paths
const schemaDDL = `
CREATE TABLE IF NOT EXISTS schema_version (
    version    INTEGER PRIMARY KEY,
    applied_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS nodes (
    tenant_id    TEXT NOT NULL,
    node_id      TEXT NOT NULL,
    type_id      INTEGER NOT NULL,
    payload_json TEXT NOT NULL DEFAULT '{}',
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    owner_actor  TEXT NOT NULL,
    acl_blob     TEXT NOT NULL DEFAULT '[]',
    PRIMARY KEY (tenant_id, node_id)
);
CREATE INDEX IF NOT EXISTS idx_nodes_type    ON nodes(tenant_id, type_id);
CREATE INDEX IF NOT EXISTS idx_nodes_owner   ON nodes(tenant_id, owner_actor);
CREATE INDEX IF NOT EXISTS idx_nodes_updated ON nodes(tenant_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS edges (
    tenant_id      TEXT NOT NULL,
    edge_type_id   INTEGER NOT NULL,
    from_node_id   TEXT NOT NULL,
    to_node_id     TEXT NOT NULL,
    props_json     TEXT NOT NULL DEFAULT '{}',
    propagates_acl INTEGER NOT NULL DEFAULT 0,
    created_at     INTEGER NOT NULL,
    PRIMARY KEY (tenant_id, edge_type_id, from_node_id, to_node_id)
);
CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(tenant_id, from_node_id);
CREATE INDEX IF NOT EXISTS idx_edges_to   ON edges(tenant_id, to_node_id);
CREATE INDEX IF NOT EXISTS idx_edges_type ON edges(tenant_id, edge_type_id);

CREATE TABLE IF NOT EXISTS node_visibility (
    tenant_id TEXT NOT NULL,
    node_id   TEXT NOT NULL,
    principal TEXT NOT NULL,
    PRIMARY KEY (tenant_id, node_id, principal)
);
CREATE INDEX IF NOT EXISTS idx_visibility_principal
    ON node_visibility(tenant_id, principal, node_id);

CREATE TABLE IF NOT EXISTS applied_events (
    tenant_id       TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    stream_pos      TEXT,
    applied_at      INTEGER NOT NULL,
    -- status is one of 'APPLIED' or 'FAILED_PRECONDITION'. Default
    -- 'APPLIED' keeps the legacy success-only rows correct after
    -- the GitHub-issue-#500 (CAS) migration; the FAILED case is the
    -- memoized precondition-miss path so retries with the same idem
    -- key replay the cached failure without re-evaluating.
    status          TEXT NOT NULL DEFAULT 'APPLIED',
    -- failure_json carries a JSON encoding of the apply.PreconditionFailure
    -- struct when status='FAILED_PRECONDITION'; NULL otherwise. The
    -- structured fields (op_index, field, expected, observed,
    -- field_present) survive a round-trip through the idempotency
    -- cache so the wire-side typed error is identical on retry.
    failure_json    TEXT,
    UNIQUE (tenant_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_applied_events_key
    ON applied_events(tenant_id, idempotency_key);

CREATE TABLE IF NOT EXISTS applied_offsets (
    tenant_id  TEXT NOT NULL,
    topic      TEXT NOT NULL,
    partition  INTEGER NOT NULL,
    offset_pos INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (tenant_id, topic, partition)
);

CREATE TABLE IF NOT EXISTS node_access (
    node_id          TEXT NOT NULL,
    actor_id         TEXT NOT NULL,
    actor_type       TEXT NOT NULL DEFAULT 'user',
    permission       TEXT NOT NULL,
    granted_by       TEXT NOT NULL,
    granted_at       INTEGER NOT NULL,
    expires_at       INTEGER DEFAULT NULL,
    type_id          INTEGER NOT NULL DEFAULT 0,
    core_caps_json   TEXT NOT NULL DEFAULT '[]',
    ext_cap_ids_json TEXT NOT NULL DEFAULT '[]',
    PRIMARY KEY (node_id, actor_id)
);
CREATE INDEX IF NOT EXISTS idx_access_actor
    ON node_access(actor_id, node_id);

CREATE TABLE IF NOT EXISTS group_users (
    group_id        TEXT NOT NULL,
    member_actor_id TEXT NOT NULL,
    role            TEXT NOT NULL DEFAULT 'member',
    joined_at       INTEGER NOT NULL,
    PRIMARY KEY (group_id, member_actor_id)
);
CREATE INDEX IF NOT EXISTS idx_group_users_member
    ON group_users(member_actor_id);

CREATE TABLE IF NOT EXISTS acl_inherit (
    node_id      TEXT NOT NULL,
    inherit_from TEXT NOT NULL,
    PRIMARY KEY (node_id, inherit_from)
);
CREATE INDEX IF NOT EXISTS idx_inherit_from
    ON acl_inherit(inherit_from);

INSERT OR IGNORE INTO schema_version (version, applied_at)
    VALUES (1, strftime('%s', 'now') * 1000);
`

// initSchema creates every table the store package needs. Idempotent.
// Mirrors canonical_store.py:_create_schema (1037-1200) minus the
// deprecated/ legacy tables (audit_log, notifications, read_cursors,
// type_metadata) — those are either superseded (CLAUDE.md invariant #2)
// or out-of-scope for Wave 1.
func initSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schemaDDL); err != nil {
		return fmt.Errorf("store: create schema: %w", err)
	}
	// In-place column additions for tenant DBs created before the
	// GitHub-issue-#500 CAS migration. SQLite has no IF NOT EXISTS on
	// ADD COLUMN, so we probe pragma_table_info first.
	if err := addColumnIfMissing(ctx, db, "applied_events", "status",
		`ALTER TABLE applied_events ADD COLUMN status TEXT NOT NULL DEFAULT 'APPLIED'`,
	); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "applied_events", "failure_json",
		`ALTER TABLE applied_events ADD COLUMN failure_json TEXT`,
	); err != nil {
		return err
	}
	return nil
}

// addColumnIfMissing runs alterSQL when the given column is absent
// from the table. Idempotent — re-runs are a no-op on fully-migrated
// databases.
func addColumnIfMissing(ctx context.Context, db *sql.DB, table, column, alterSQL string) error {
	rows, err := db.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return fmt.Errorf("store: probe %s columns: %w", table, err)
	}
	defer rows.Close()
	have := false
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("store: scan %s column: %w", table, err)
		}
		if name == column {
			have = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: iterate %s columns: %w", table, err)
	}
	if have {
		return nil
	}
	if _, err := db.ExecContext(ctx, alterSQL); err != nil {
		return fmt.Errorf("store: add column %s.%s: %w", table, column, err)
	}
	return nil
}
