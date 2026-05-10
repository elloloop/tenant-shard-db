// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"fmt"
)

// applyAddGroupMember dispatches an "add_group_member" op. Restores
// the WAL-first invariant flagged in PLAN.md §6.1 — the Python
// AddGroupMember handler writes group_users directly today.
//
// op shape:
//
//	{
//	  "op":              "add_group_member",
//	  "group_id":        "<id>",
//	  "member_actor_id": "<kind:id>",
//	  "role":            "<role>", // default "member"
//	}
func (a *Applier) applyAddGroupMember(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any) error {
	groupID := stringField(op, "group_id")
	memberActorID := stringField(op, "member_actor_id")
	if groupID == "" || memberActorID == "" {
		return fmt.Errorf("%w: add_group_member missing group_id/member_actor_id", ErrPoisonEvent)
	}
	role := stringField(op, "role")
	if role == "" {
		role = "member"
	}
	now := ev.TsMs
	if now == 0 {
		now = a.now()
	}
	conn := tx.Conn()
	if _, err := conn.ExecContext(ctx, `
		INSERT OR REPLACE INTO group_users
		    (group_id, member_actor_id, role, joined_at)
		VALUES (?, ?, ?, ?)`,
		groupID, memberActorID, role, now,
	); err != nil {
		return fmt.Errorf("apply add_group_member: insert: %w", err)
	}
	return nil
}
