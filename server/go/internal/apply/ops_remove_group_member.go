// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"fmt"
)

// applyRemoveGroupMember dispatches a "remove_group_member" op.
// Restores the WAL-first invariant flagged in PLAN.md §6.1.
//
// op shape:
//
//	{
//	  "op":              "remove_group_member",
//	  "group_id":        "<id>",
//	  "member_actor_id": "<kind:id>",
//	}
func (a *Applier) applyRemoveGroupMember(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any) error {
	groupID := stringField(op, "group_id")
	memberActorID := stringField(op, "member_actor_id")
	if groupID == "" || memberActorID == "" {
		return fmt.Errorf("%w: remove_group_member missing group_id/member_actor_id", ErrPoisonEvent)
	}
	conn := tx.Conn()
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM group_users WHERE group_id = ? AND member_actor_id = ?`,
		groupID, memberActorID,
	); err != nil {
		return fmt.Errorf("apply remove_group_member: delete: %w", err)
	}
	return nil
}
