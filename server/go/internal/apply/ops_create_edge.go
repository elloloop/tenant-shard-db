// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"encoding/json"
	"fmt"
)

// applyCreateEdge dispatches a "create_edge" op. Mirrors
// applier.py:1159-1217. INSERT OR REPLACE semantics on the edges row so
// re-creating an edge with new props is idempotent.
//
// Edge ACL inheritance: when propagates_acl is true, an acl_inherit row
// is added (cycle-checked via a depth-bounded recursive CTE).
func (a *Applier) applyCreateEdge(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any, aliases nodeAliasMap, res *Result) error {
	edgeTypeID := intField(op, "edge_id")
	if edgeTypeID == 0 {
		return fmt.Errorf("%w: create_edge missing edge_id", ErrPoisonEvent)
	}
	from := aliases.resolveNodeRef(stringField(op, "from"))
	to := aliases.resolveNodeRef(stringField(op, "to"))
	if from == "" || to == "" {
		return fmt.Errorf("%w: create_edge missing from/to", ErrPoisonEvent)
	}
	props := mapField(op, "props")
	if props == nil {
		props = map[string]any{}
	}
	propsJSON, err := json.Marshal(props)
	if err != nil {
		return fmt.Errorf("apply create_edge: marshal props: %w", err)
	}
	propagates := 0
	if boolField(op, "propagates_acl") {
		propagates = 1
	}
	now := ev.TsMs
	if now == 0 {
		now = a.now()
	}
	conn := tx.Conn()
	if _, err := conn.ExecContext(ctx, `
		INSERT OR REPLACE INTO edges
		    (tenant_id, edge_type_id, from_node_id, to_node_id,
		     props_json, propagates_acl, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ev.TenantID, edgeTypeID, from, to, string(propsJSON), propagates, now,
	); err != nil {
		return fmt.Errorf("apply create_edge: insert: %w", err)
	}
	if propagates == 1 {
		// Cycle check: would adding (to inherits-from from) create a
		// cycle? Bounded at depth 10 (canonical_store.py:_ACL_MAX_DEPTH).
		var one int
		err := conn.QueryRowContext(ctx, `
			WITH RECURSIVE chain(nid, depth) AS (
				SELECT ?, 0
				UNION ALL
				SELECT ai.inherit_from, c.depth + 1
				FROM acl_inherit ai
				JOIN chain c ON ai.node_id = c.nid
				WHERE c.depth < 10
			)
			SELECT 1 FROM chain WHERE nid = ? LIMIT 1`,
			from, to,
		).Scan(&one)
		// err == sql.ErrNoRows means "no cycle" — proceed; any other
		// error halts.
		if err != nil && !isNoRows(err) {
			return fmt.Errorf("apply create_edge: cycle check: %w", err)
		}
		if isNoRows(err) {
			if _, err := conn.ExecContext(ctx,
				`INSERT OR IGNORE INTO acl_inherit (node_id, inherit_from) VALUES (?, ?)`,
				to, from,
			); err != nil {
				return fmt.Errorf("apply create_edge: acl_inherit: %w", err)
			}
		}
	}
	res.CreatedEdges = append(res.CreatedEdges, EdgeRef{
		EdgeTypeID: edgeTypeID, FromNodeID: from, ToNodeID: to,
	})
	return nil
}
