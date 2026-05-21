// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"fmt"
)

// applyAdminTransferContent dispatches an "admin_transfer_content" op.
// Also refreshes the visibility for every owner-row that changed hands
// (the original applier did not do this).
//
// op shape:
//
//	{
//	  "op": "admin_transfer_content",
//	  "from_user": "user:alice",
//	  "to_user": "user:bob",
//	}
func (a *Applier) applyAdminTransferContent(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any) error {
	fromUser := stringField(op, "from_user")
	toUser := stringField(op, "to_user")
	if fromUser == "" || toUser == "" {
		return fmt.Errorf("%w: admin_transfer_content missing from/to_user", ErrPoisonEvent)
	}
	now := ev.TsMs
	if now == 0 {
		now = a.now()
	}
	conn := tx.Conn()
	if _, err := conn.ExecContext(ctx, `
		UPDATE nodes SET owner_actor = ?, updated_at = ?
		WHERE tenant_id = ? AND owner_actor = ?`,
		toUser, now, ev.TenantID, fromUser,
	); err != nil {
		return fmt.Errorf("apply admin_transfer_content: update: %w", err)
	}
	// Refresh visibility for every node whose owner changed: replace
	// fromUser with toUser in node_visibility (closes the
	// transfer_user_content visibility-drift bug).
	if _, err := conn.ExecContext(ctx, `
		UPDATE OR IGNORE node_visibility SET principal = ?
		WHERE tenant_id = ? AND principal = ?`,
		toUser, ev.TenantID, fromUser,
	); err != nil {
		return fmt.Errorf("apply admin_transfer_content: refresh visibility: %w", err)
	}
	// Drop any duplicates that the UPDATE OR IGNORE skipped past.
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM node_visibility WHERE tenant_id = ? AND principal = ?`,
		ev.TenantID, fromUser,
	); err != nil {
		return fmt.Errorf("apply admin_transfer_content: cleanup visibility: %w", err)
	}
	return nil
}
