// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"fmt"
)

// applyDeleteNode dispatches a "delete_node" op. Mirrors
// applier.py:1109-1156. Cascades edges, visibility, node_access and
// acl_inherit before deleting the node row.
func (a *Applier) applyDeleteNode(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any, aliases nodeAliasMap, res *Result) error {
	nodeID := aliases.resolveRef(stringField(op, "id"))
	if nodeID == "" {
		return fmt.Errorf("%w: delete_node missing id", ErrPoisonEvent)
	}
	conn := tx.Conn()
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM edges WHERE tenant_id = ? AND (from_node_id = ? OR to_node_id = ?)`,
		ev.TenantID, nodeID, nodeID,
	); err != nil {
		return fmt.Errorf("apply delete_node: edges: %w", err)
	}
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM node_visibility WHERE tenant_id = ? AND node_id = ?`,
		ev.TenantID, nodeID,
	); err != nil {
		return fmt.Errorf("apply delete_node: visibility: %w", err)
	}
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM node_access WHERE node_id = ?`, nodeID,
	); err != nil {
		return fmt.Errorf("apply delete_node: node_access: %w", err)
	}
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM acl_inherit WHERE node_id = ?`, nodeID,
	); err != nil {
		return fmt.Errorf("apply delete_node: acl_inherit: %w", err)
	}
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM nodes WHERE tenant_id = ? AND node_id = ?`,
		ev.TenantID, nodeID,
	); err != nil {
		return fmt.Errorf("apply delete_node: nodes: %w", err)
	}
	res.DeletedNodeIDs = append(res.DeletedNodeIDs, nodeID)
	return nil
}
