// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"fmt"
)

// applyAdminRevokeAccess dispatches an "admin_revoke_access" op.
//
// PLAN.md §6.4 item 2: revokes every grant + group membership AND
// every visibility row for the user, matching the behaviour the
// RevokeAllUserAccess handler intends to deliver.
//
// op shape:
//
//	{"op":"admin_revoke_access","user_id":"<id>"}
func (a *Applier) applyAdminRevokeAccess(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any) error {
	userID := stringField(op, "user_id")
	if userID == "" {
		return fmt.Errorf("%w: admin_revoke_access missing user_id", ErrPoisonEvent)
	}
	conn := tx.Conn()
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM node_access WHERE actor_id = ?`, userID,
	); err != nil {
		return fmt.Errorf("apply admin_revoke_access: node_access: %w", err)
	}
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM group_users WHERE member_actor_id = ?`, userID,
	); err != nil {
		return fmt.Errorf("apply admin_revoke_access: group_users: %w", err)
	}
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM node_visibility WHERE tenant_id = ? AND principal = ?`,
		ev.TenantID, userID,
	); err != nil {
		return fmt.Errorf("apply admin_revoke_access: node_visibility: %w", err)
	}
	return nil
}
