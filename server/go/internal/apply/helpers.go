// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// refreshVisibility clears and re-populates node_visibility for a node.
// Mirrors store/visibility.go updateVisibilityWithConn but lives in the
// apply package because it runs on a *sql.Conn handed out by an open
// BatchTxn (the store helper takes the same shape but is unexported).
func refreshVisibility(ctx context.Context, conn *sql.Conn, tenantID, nodeID, ownerActor string, acl []aclEntry) error {
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM node_visibility WHERE tenant_id = ? AND node_id = ?`,
		tenantID, nodeID,
	); err != nil {
		return fmt.Errorf("apply: clear visibility: %w", err)
	}
	if ownerActor != "" {
		if _, err := conn.ExecContext(ctx,
			`INSERT INTO node_visibility (tenant_id, node_id, principal) VALUES (?, ?, ?)`,
			tenantID, nodeID, ownerActor,
		); err != nil {
			return fmt.Errorf("apply: insert owner visibility: %w", err)
		}
	}
	for _, e := range acl {
		if e.Principal == "" || e.Principal == ownerActor {
			continue
		}
		if _, err := conn.ExecContext(ctx,
			`INSERT OR IGNORE INTO node_visibility (tenant_id, node_id, principal) VALUES (?, ?, ?)`,
			tenantID, nodeID, e.Principal,
		); err != nil {
			return fmt.Errorf("apply: insert acl visibility: %w", err)
		}
	}
	return nil
}

// searchableFieldIDs returns the searchable field_ids for typeID from
// the store's schema registry, or nil when no registry is wired or the
// type has no searchable fields.
func (a *Applier) searchableFieldIDs(typeID int32) []uint32 {
	reg := a.store.Registry()
	if reg == nil {
		return nil
	}
	return reg.SearchableFieldIDs(typeID)
}

// indexNodeFTS writes (or re-writes) a node's FTS5 row inside the open
// BatchTxn connection so the search index commits atomically with the
// node row. No-op when the type declares no searchable fields.
//
// payload is the field-id-keyed map already destined for the node row.
// The FTS virtual table is lazily created (idempotent CREATE) on the
// SAME connection as the open BatchTxn (EnsureFTSIndexConn): running the
// DDL on a separate pooled connection would deadlock against the write
// lock the in-flight BEGIN IMMEDIATE already holds.
func (a *Applier) indexNodeFTS(ctx context.Context, conn *sql.Conn, tenantID string, typeID int32, nodeID string, payload map[string]any) error {
	fids := a.searchableFieldIDs(typeID)
	if len(fids) == 0 {
		return nil
	}
	if err := a.store.EnsureFTSIndexConn(ctx, conn, tenantID, typeID, fids); err != nil {
		return err
	}
	return store.FTSInsertConn(ctx, conn, typeID, nodeID, payload, fids)
}

// deindexNodeFTS removes a node's FTS5 row inside the open BatchTxn
// connection. No-op when the type declares no searchable fields (so the
// virtual table may not exist).
func (a *Applier) deindexNodeFTS(ctx context.Context, conn *sql.Conn, tenantID string, typeID int32, nodeID string) error {
	fids := a.searchableFieldIDs(typeID)
	if len(fids) == 0 {
		return nil
	}
	if err := a.store.EnsureFTSIndexConn(ctx, conn, tenantID, typeID, fids); err != nil {
		return err
	}
	return store.FTSDeleteConn(ctx, conn, typeID, nodeID)
}

// isNoRows is a tiny shim for errors.Is(err, sql.ErrNoRows). Kept as a
// helper because the create_edge cycle check is the only call site and
// the inline expression hurts readability.
func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
