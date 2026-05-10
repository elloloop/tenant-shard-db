// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// applyDelegateAccess dispatches a "delegate_access" op.
//
// CRITICAL FIX (PLAN.md §6.4 item 1): the Python applier has no
// dispatch branch for admin_delegate_access events; the WAL event is
// silently dropped on replay. The Python tests only pass because the
// handler also writes node_access directly. The Go applier closes that
// gap by materialising the grant in the same row shape that
// share_node uses.
//
// Semantically delegate_access is identical to share_node — the
// "delegating actor must already hold the cap" check happens at the
// gRPC handler / acl checker layer (W1.9), not here. Storing the grant
// is the applier's only job.
//
// op shape: same as share_node, but the granted_by field is recorded
// as event.actor (the delegator).
func (a *Applier) applyDelegateAccess(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any, res *Result) error {
	nodeID := stringField(op, "node_id")
	actorID := stringField(op, "actor_id")
	if nodeID == "" || actorID == "" {
		return fmt.Errorf("%w: delegate_access missing node_id/actor_id", ErrPoisonEvent)
	}
	actorType := stringField(op, "actor_type")
	if actorType == "" {
		actorType = "user"
	}
	perm := stringField(op, "permission")
	typeID := intField(op, "type_id")

	coreCaps := intSliceField(op, "core_caps")
	extCaps := intSliceField(op, "ext_cap_ids")
	coreJSON, err := json.Marshal(coreCaps)
	if err != nil {
		return fmt.Errorf("apply delegate_access: marshal core_caps: %w", err)
	}
	extJSON, err := json.Marshal(extCaps)
	if err != nil {
		return fmt.Errorf("apply delegate_access: marshal ext_caps: %w", err)
	}
	var expires sql.NullInt64
	if v := int64Field(op, "expires_at"); v > 0 {
		expires = sql.NullInt64{Int64: v, Valid: true}
	}
	now := ev.TsMs
	if now == 0 {
		now = a.now()
	}
	conn := tx.Conn()
	if _, err := conn.ExecContext(ctx, `
		INSERT OR REPLACE INTO node_access
		    (node_id, actor_id, actor_type, permission, granted_by,
		     granted_at, expires_at, type_id, core_caps_json, ext_cap_ids_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nodeID, actorID, actorType, perm, ev.Actor,
		now, expires, typeID, string(coreJSON), string(extJSON),
	); err != nil {
		return fmt.Errorf("apply delegate_access: insert: %w", err)
	}
	if userID := stringField(op, "user_id"); userID != "" {
		src := stringField(op, "source_tenant")
		if src == "" {
			src = ev.TenantID
		}
		res.SharedAdded = append(res.SharedAdded, SharedRef{
			UserID: userID, SourceTenant: src, NodeID: nodeID, Permission: perm,
		})
	}
	return nil
}
