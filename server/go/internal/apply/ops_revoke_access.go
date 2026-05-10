// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"fmt"
)

// applyRevokeAccess dispatches a "revoke_access" op. Restores the WAL-
// first invariant flagged in PLAN.md §6.1 — the Python RevokeAccess
// handler deletes node_access directly today.
//
// op shape:
//
//	{
//	  "op":           "revoke_access",
//	  "node_id":      "<id>",
//	  "actor_id":     "<id>",
//	  "user_id":      "<id>",        // optional cross-tenant grantee
//	  "source_tenant":"<id>",        // optional cross-tenant src
//	}
func (a *Applier) applyRevokeAccess(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any, res *Result) error {
	nodeID := stringField(op, "node_id")
	actorID := stringField(op, "actor_id")
	if nodeID == "" || actorID == "" {
		return fmt.Errorf("%w: revoke_access missing node_id/actor_id", ErrPoisonEvent)
	}
	conn := tx.Conn()
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM node_access WHERE node_id = ? AND actor_id = ?`,
		nodeID, actorID,
	); err != nil {
		return fmt.Errorf("apply revoke_access: delete: %w", err)
	}
	if userID := stringField(op, "user_id"); userID != "" {
		src := stringField(op, "source_tenant")
		if src == "" {
			src = ev.TenantID
		}
		res.SharedRemoved = append(res.SharedRemoved, SharedRef{
			UserID: userID, SourceTenant: src, NodeID: nodeID,
		})
	}
	return nil
}
