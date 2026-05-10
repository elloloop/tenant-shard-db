// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// applyUpdateNode dispatches an "update_node" op. Mirrors
// applier.py:1020-1107. The patch is field-id-keyed and merged onto the
// existing payload by string key (rename-free per CLAUDE.md invariant
// #6). storage_mode is immutable — any attempt to set it is a
// poison-event.
func (a *Applier) applyUpdateNode(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any, aliases nodeAliasMap) error {
	patch := mapField(op, "patch")
	if patch == nil {
		patch = map[string]any{}
	}
	if _, ok := patch["storage_mode"]; ok {
		return fmt.Errorf("%w: storage_mode is immutable", ErrPoisonEvent)
	}
	nodeID := aliases.resolveRef(stringField(op, "id"))
	if nodeID == "" {
		return fmt.Errorf("%w: update_node missing id", ErrPoisonEvent)
	}

	conn := tx.Conn()
	row := conn.QueryRowContext(ctx,
		`SELECT payload_json FROM nodes WHERE tenant_id = ? AND node_id = ?`,
		ev.TenantID, nodeID,
	)
	var existingJSON string
	if err := row.Scan(&existingJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Mirror the Python applier: missing target is a no-op.
			return nil
		}
		return fmt.Errorf("apply update_node: read existing: %w", err)
	}
	merged := map[string]any{}
	if existingJSON != "" {
		if err := json.Unmarshal([]byte(existingJSON), &merged); err != nil {
			return fmt.Errorf("apply update_node: parse existing payload: %w", err)
		}
	}
	for k, v := range patch {
		merged[k] = v
	}
	mergedJSON, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("apply update_node: marshal merged: %w", err)
	}
	now := ev.TsMs
	if now == 0 {
		now = a.now()
	}
	if _, err := conn.ExecContext(ctx,
		`UPDATE nodes SET payload_json = ?, updated_at = ? WHERE tenant_id = ? AND node_id = ?`,
		string(mergedJSON), now, ev.TenantID, nodeID,
	); err != nil {
		return fmt.Errorf("apply update_node: update: %w", err)
	}
	return nil
}
