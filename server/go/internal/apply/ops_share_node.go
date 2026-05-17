// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// applyShareNode dispatches a "share_node" op. Restores the WAL-first
// invariant flagged in PLAN.md §6.1 — the Python ShareNode handler
// writes node_access directly today, bypassing the WAL. This applier
// branch makes the grant survive a full WAL rebuild.
//
// op shape:
//
//	{
//	  "op": "share_node",
//	  "node_id": "<id>",
//	  "actor_id": "<id>",
//	  "actor_type": "user|group", // default "user"
//	  "permission": "read|write|...", // legacy enum string
//	  "type_id": <int>, // optional, for typed-cap scoping
//	  "core_caps": [<int>, ...], // typed core caps
//	  "ext_cap_ids": [<int>, ...], // typed extension caps
//	  "expires_at": <ms>, // 0 == never
//	  "user_id": "<id>", // optional cross-tenant grantee
//	  "source_tenant": "<id>", // optional cross-tenant src
//	}
func (a *Applier) applyShareNode(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any, res *Result) error {
	nodeID := stringField(op, "node_id")
	actorID := stringField(op, "actor_id")
	if nodeID == "" || actorID == "" {
		return fmt.Errorf("%w: share_node missing node_id/actor_id", ErrPoisonEvent)
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
		return fmt.Errorf("apply share_node: marshal core_caps: %w", err)
	}
	extJSON, err := json.Marshal(extCaps)
	if err != nil {
		return fmt.Errorf("apply share_node: marshal ext_caps: %w", err)
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
		return fmt.Errorf("apply share_node: insert: %w", err)
	}
	// Record cross-tenant share for shared_index maintenance post-commit.
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

// intSliceField extracts a list-of-int field. Tolerates JSON-decoded
// []any whose elements are float64.
func intSliceField(op map[string]any, key string) []int32 {
	out := []int32{}
	raw, ok := op[key].([]any)
	if !ok {
		return out
	}
	for _, v := range raw {
		switch x := v.(type) {
		case float64:
			out = append(out, int32(x))
		case int:
			out = append(out, int32(x))
		case int32:
			out = append(out, x)
		case int64:
			out = append(out, int32(x))
		}
	}
	return out
}

// int64Field extracts an int64-valued field. JSON numbers come through
// as float64.
func int64Field(op map[string]any, key string) int64 {
	switch v := op[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	}
	return 0
}
