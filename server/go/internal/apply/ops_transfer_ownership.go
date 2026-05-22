// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// applyTransferOwnership dispatches a "transfer_ownership" op.
// Restores the WAL-first invariant flagged in PLAN.md §6.1.
//
// Refreshes node_visibility for the new owner so the visibility index
// stays consistent (PLAN.md §6.4 item 3 calls out the
// transfer_user_content visibility-drift bug; this branch fixes it for
// single-node transfers).
//
// op shape:
//
//	{"op":"transfer_ownership","node_id":"<id>","new_owner":"user:..."}
func (a *Applier) applyTransferOwnership(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any) error {
	nodeID := stringField(op, "node_id")
	newOwner := stringField(op, "new_owner")
	if nodeID == "" || newOwner == "" {
		return fmt.Errorf("%w: transfer_ownership missing node_id/new_owner", ErrPoisonEvent)
	}
	conn := tx.Conn()

	var aclJSON string
	err := conn.QueryRowContext(ctx,
		`SELECT acl_blob FROM nodes WHERE tenant_id = ? AND node_id = ?`,
		ev.TenantID, nodeID,
	).Scan(&aclJSON)
	if errors.Is(err, sql.ErrNoRows) {
		// Missing target: no-op; halt only on infra errors.
		return nil
	}
	if err != nil {
		return fmt.Errorf("apply transfer_ownership: read acl: %w", err)
	}
	var acl []aclEntry
	if aclJSON != "" {
		_ = json.Unmarshal([]byte(aclJSON), &acl)
	}
	if _, err := conn.ExecContext(ctx,
		`UPDATE nodes SET owner_actor = ? WHERE tenant_id = ? AND node_id = ?`,
		newOwner, ev.TenantID, nodeID,
	); err != nil {
		return fmt.Errorf("apply transfer_ownership: update owner: %w", err)
	}
	if err := refreshVisibility(ctx, conn, ev.TenantID, nodeID, newOwner, acl); err != nil {
		return err
	}
	return nil
}
