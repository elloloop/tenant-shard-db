// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// isNoRows is a tiny shim for errors.Is(err, sql.ErrNoRows). Kept as a
// helper because the create_edge cycle check is the only call site and
// the inline expression hurts readability.
func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
