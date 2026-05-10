// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"encoding/json"
	"fmt"
)

// applyCreateNode dispatches a "create_node" op. Mirrors
// server/python/entdb_server/apply/applier.py:929-1017.
//
// The op shape (field-id-keyed payload, per CLAUDE.md invariant #6):
//
//	{
//	  "op":           "create_node",
//	  "id":           "<node_id>",
//	  "as":           "alias",            // optional
//	  "type_id":      <int>,
//	  "data":         {"<fid>": value...}, // field-id keyed
//	  "acl":          [{principal, permission}, ...] // optional
//	}
//
// owner_actor comes from the surrounding event.actor. Per the spec, the
// applier never generates node ids — caller-supplied ids are required
// (the Python applier accepts a missing id and asks canonical_store to
// generate one; the Go port rejects it so determinism doesn't depend on
// the UUID source).
func (a *Applier) applyCreateNode(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any, aliases nodeAliasMap, res *Result) error {
	nodeID := stringField(op, "id")
	if nodeID == "" {
		return fmt.Errorf("%w: create_node missing id", ErrPoisonEvent)
	}
	typeID := intField(op, "type_id")
	if typeID == 0 {
		return fmt.Errorf("%w: create_node missing type_id", ErrPoisonEvent)
	}
	payload := mapField(op, "data")
	if payload == nil {
		payload = map[string]any{}
	}
	// ACL list. Tolerate either []map[string]any (from JSON unmarshal)
	// or already-typed []store.ACLEntry slices supplied by tests.
	var acl []aclEntry
	if raw, ok := op["acl"].([]any); ok {
		for _, r := range raw {
			if m, ok := r.(map[string]any); ok {
				acl = append(acl, aclEntry{
					Principal:  stringField(m, "principal"),
					Permission: stringField(m, "permission"),
				})
			}
		}
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("apply create_node: marshal payload: %w", err)
	}
	aclJSON, err := json.Marshal(orEmpty(acl))
	if err != nil {
		return fmt.Errorf("apply create_node: marshal acl: %w", err)
	}

	now := ev.TsMs
	if now == 0 {
		now = a.now()
	}

	conn := tx.Conn()
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO nodes (tenant_id, node_id, type_id, payload_json,
		                   created_at, updated_at, owner_actor, acl_blob)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ev.TenantID, nodeID, typeID, string(payloadJSON),
		now, now, ev.Actor, string(aclJSON),
	); err != nil {
		return fmt.Errorf("apply create_node: insert: %w", err)
	}
	// Refresh visibility: owner + each ACL principal.
	if err := refreshVisibility(ctx, conn, ev.TenantID, nodeID, ev.Actor, acl); err != nil {
		return err
	}

	if alias := stringField(op, "as"); alias != "" {
		aliases[alias] = nodeID
	}
	res.CreatedNodes = append(res.CreatedNodes, nodeID)
	return nil
}

// aclEntry mirrors store.ACLEntry; we don't import store here because
// the per-op handlers run raw SQL — the store types are only the
// surface area the applier exposes back out (see alias.go).
type aclEntry struct {
	Principal  string `json:"principal"`
	Permission string `json:"permission"`
}

func orEmpty(a []aclEntry) []aclEntry {
	if a == nil {
		return []aclEntry{}
	}
	return a
}
