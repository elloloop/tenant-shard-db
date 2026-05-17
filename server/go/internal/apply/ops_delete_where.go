// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"fmt"
	"strings"

	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// DeleteWhere server-side cost ceiling. The single-applier invariant
// (CLAUDE.md §3 — one applier goroutine per tenant) means a runaway
// predicate that matches a million rows would pin the tenant's applier
// for the duration of the delete, starving every other write. We cap
// the per-op delete count so a sweeper degrades to "many small sweeps"
// instead of "one unbounded stall". A caller that needs to drain a
// large backlog polls until a sweep deletes zero rows (the standard
// TTL-sweeper loop). GitHub issue #504, "Server-side cost ceiling".
const (
	// deleteWhereDefaultLimit is used when the op requests limit <= 0.
	deleteWhereDefaultLimit = 1000
	// deleteWhereMaxLimit is the hard ceiling — the requested limit is
	// clamped to this regardless of how large a value the caller sent.
	deleteWhereMaxLimit = 10000
)

// applyDeleteWhere dispatches a "delete_where" op (GitHub issue #504):
// a single-RPC predicate-based sweeper. It deletes every node of
// op["type_id"] whose field-id-keyed payload matches ALL of the
// op["where"] predicates, capped best-effort by op["limit"].
//
// Idempotency is the standard whole-batch applied_events memoization
// the applier already enforces around every event (issue #500): a
// retry with the same idempotency key is a no-op replay because the
// event's idempotency row is recorded in the same BatchTxn. We do NOT
// memoize a cached row count — issue #504 scopes the response to
// "applied, no count" for v1, so a replay simply re-reports APPLIED
// with zero deleted ids (the rows are already gone). This is
// deterministic on a from-scratch rebuild too: the first replay of
// this event deletes the matching rows; any later replay of the SAME
// event sees the idempotency row and skips before reaching this op.
//
// The predicate primitives reuse the issue-#501 range operators via
// store.QueryFilter / store.CompileQueryFilters, so no new SQL-
// construction path is introduced and the injection-safe
// json_extract(payload_json, '$."<field_id>"') <op> ? shape is shared
// verbatim with QueryNodes.
//
// Cascade parity with applyDeleteNode: for every matched node we remove
// its edges, node_visibility, node_access and acl_inherit rows before
// deleting the node row, all inside the caller's BatchTxn so the sweep
// is atomic with the rest of the event.
func (a *Applier) applyDeleteWhere(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any, res *Result) error {
	typeID := intField(op, "type_id")
	if typeID == 0 {
		return fmt.Errorf("%w: delete_where missing type_id", ErrPoisonEvent)
	}

	filters, err := parseDeleteWhereFilters(op)
	if err != nil {
		return err
	}
	if len(filters) == 0 {
		// An unconditional bulk delete is too dangerous to express
		// implicitly — the handler rejects the empty-predicate case at
		// ingress; reaching here means a malformed WAL record.
		return fmt.Errorf("%w: delete_where requires at least one predicate", ErrPoisonEvent)
	}

	limit := int(intField(op, "limit"))
	if limit <= 0 {
		limit = deleteWhereDefaultLimit
	}
	if limit > deleteWhereMaxLimit {
		limit = deleteWhereMaxLimit
	}

	clauses, params := store.CompileQueryFilters(filters)

	conn := tx.Conn()

	// Select the matching ids first (best-effort LIMIT), then cascade-
	// delete each. We materialise the id list rather than issuing a
	// single `DELETE ... WHERE ... LIMIT` because we must also clean up
	// the four dependent tables per node, exactly like applyDeleteNode.
	selectSQL := fmt.Sprintf(`
		SELECT node_id FROM nodes
		WHERE tenant_id = ? AND type_id = ? AND %s
		ORDER BY node_id
		LIMIT ?`,
		strings.Join(clauses, " AND "),
	)
	selectArgs := make([]any, 0, len(params)+3)
	selectArgs = append(selectArgs, ev.TenantID, typeID)
	selectArgs = append(selectArgs, params...)
	selectArgs = append(selectArgs, limit)

	rows, err := conn.QueryContext(ctx, selectSQL, selectArgs...)
	if err != nil {
		return fmt.Errorf("apply delete_where: select: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			_ = rows.Close()
			return fmt.Errorf("apply delete_where: scan: %w", scanErr)
		}
		ids = append(ids, id)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		_ = rows.Close()
		return fmt.Errorf("apply delete_where: rows: %w", rowsErr)
	}
	_ = rows.Close()

	for _, nodeID := range ids {
		if _, err := conn.ExecContext(ctx,
			`DELETE FROM edges WHERE tenant_id = ? AND (from_node_id = ? OR to_node_id = ?)`,
			ev.TenantID, nodeID, nodeID,
		); err != nil {
			return fmt.Errorf("apply delete_where: edges: %w", err)
		}
		if _, err := conn.ExecContext(ctx,
			`DELETE FROM node_visibility WHERE tenant_id = ? AND node_id = ?`,
			ev.TenantID, nodeID,
		); err != nil {
			return fmt.Errorf("apply delete_where: visibility: %w", err)
		}
		if _, err := conn.ExecContext(ctx,
			`DELETE FROM node_access WHERE node_id = ?`, nodeID,
		); err != nil {
			return fmt.Errorf("apply delete_where: node_access: %w", err)
		}
		if _, err := conn.ExecContext(ctx,
			`DELETE FROM acl_inherit WHERE node_id = ?`, nodeID,
		); err != nil {
			return fmt.Errorf("apply delete_where: acl_inherit: %w", err)
		}
		if _, err := conn.ExecContext(ctx,
			`DELETE FROM nodes WHERE tenant_id = ? AND node_id = ?`,
			ev.TenantID, nodeID,
		); err != nil {
			return fmt.Errorf("apply delete_where: nodes: %w", err)
		}
		res.DeletedNodeIDs = append(res.DeletedNodeIDs, nodeID)
	}
	return nil
}

// parseDeleteWhereFilters decodes the WAL op-dict "where" list into the
// store's id-keyed [store.QueryFilter] slice. The handler resolves the
// developer-facing field name to a stable field_id at ingress (reusing
// the QueryNodes name->id path), so by the time the predicate is on the
// WAL it is already schema-less: each entry is
// {"field_id": <number>, "op": <string>, "value": <scalar>}.
//
// A structurally invalid entry (missing/zero field_id, unknown op) is a
// poison event — the handler validated the shape before the append, so
// reaching an invalid entry here means a malformed record.
func parseDeleteWhereFilters(op map[string]any) ([]store.QueryFilter, error) {
	raw, ok := op["where"].([]any)
	if !ok {
		// Tolerate the JSON-decoded "[]map[string]any" shape too.
		if alt, ok2 := op["where"].([]map[string]any); ok2 {
			raw = make([]any, len(alt))
			for i, m := range alt {
				raw[i] = m
			}
		} else {
			return nil, nil
		}
	}
	out := make([]store.QueryFilter, 0, len(raw))
	for _, entry := range raw {
		m, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: delete_where predicate is not an object", ErrPoisonEvent)
		}
		fid := intField(m, "field_id")
		if fid <= 0 {
			return nil, fmt.Errorf("%w: delete_where predicate missing field_id", ErrPoisonEvent)
		}
		qop, ok := deleteWhereOpFromString(stringField(m, "op"))
		if !ok {
			return nil, fmt.Errorf("%w: delete_where predicate has unsupported op %q",
				ErrPoisonEvent, stringField(m, "op"))
		}
		out = append(out, store.QueryFilter{
			FieldID: uint32(fid),
			Op:      qop,
			Value:   m["value"],
		})
	}
	return out, nil
}

// deleteWhereOpFromString maps the internal op-dict operator token to
// the store's comparison-operator enum. The token set is the issue-#501
// six-operator subset (eq/ne/lt/le/gt/ge) — the handler normalises both
// the wire FilterOp enum and the MongoDB-style "$op" inline keys down
// to these tokens before the WAL append.
func deleteWhereOpFromString(s string) (store.QueryFilterOp, bool) {
	switch s {
	case "eq", "$eq", "":
		return store.QueryFilterEq, true
	case "ne", "$ne":
		return store.QueryFilterNe, true
	case "lt", "$lt":
		return store.QueryFilterLt, true
	case "le", "lte", "$lte":
		return store.QueryFilterLe, true
	case "gt", "$gt":
		return store.QueryFilterGt, true
	case "ge", "gte", "$gte":
		return store.QueryFilterGe, true
	}
	return 0, false
}
