// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"fmt"
)

// applyDeleteEdge dispatches a "delete_edge" op. Mirrors
// applier.py:1220-1247. Also removes the inherited ACL pointer if any.
func (a *Applier) applyDeleteEdge(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any, aliases nodeAliasMap) error {
	edgeTypeID := intField(op, "edge_id")
	if edgeTypeID == 0 {
		return fmt.Errorf("%w: delete_edge missing edge_id", ErrPoisonEvent)
	}
	from := aliases.resolveNodeRef(stringField(op, "from"))
	to := aliases.resolveNodeRef(stringField(op, "to"))
	if from == "" || to == "" {
		return fmt.Errorf("%w: delete_edge missing from/to", ErrPoisonEvent)
	}
	conn := tx.Conn()
	if _, err := conn.ExecContext(ctx, `
		DELETE FROM edges WHERE tenant_id = ? AND edge_type_id = ?
		  AND from_node_id = ? AND to_node_id = ?`,
		ev.TenantID, edgeTypeID, from, to,
	); err != nil {
		return fmt.Errorf("apply delete_edge: delete: %w", err)
	}
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM acl_inherit WHERE node_id = ? AND inherit_from = ?`,
		to, from,
	); err != nil {
		return fmt.Errorf("apply delete_edge: acl_inherit: %w", err)
	}
	return nil
}
