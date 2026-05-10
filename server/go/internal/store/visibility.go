package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// updateVisibilityWithConn refreshes the node_visibility index for one
// node. Mirrors canonical_store.py:_update_visibility (2785). Called
// by writers (CreateNodeRaw, TransferOwnership, transfer_user_content)
// inside their transaction.
//
// Behaviour:
//
//   - Clear all existing visibility rows for (tenant_id, node_id).
//   - Insert one row for the owner.
//   - Insert one row per ACL principal (if not == owner).
func updateVisibilityWithConn(ctx context.Context, conn *sql.Conn, tenantID, nodeID, ownerActor string, acl []ACLEntry) error {
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM node_visibility WHERE tenant_id = ? AND node_id = ?`,
		tenantID, nodeID,
	); err != nil {
		return fmt.Errorf("store: clear visibility: %w", err)
	}
	if ownerActor != "" {
		if _, err := conn.ExecContext(ctx,
			`INSERT INTO node_visibility (tenant_id, node_id, principal) VALUES (?, ?, ?)`,
			tenantID, nodeID, ownerActor,
		); err != nil {
			return fmt.Errorf("store: insert owner visibility: %w", err)
		}
	}
	for _, e := range acl {
		if e.Principal == "" || e.Principal == ownerActor {
			continue
		}
		if _, err := conn.ExecContext(ctx,
			`INSERT OR IGNORE INTO node_visibility (tenant_id, node_id, principal) VALUES (?, ?, ?)`,
			tenantID, nodeID, e.Principal,
		); err != nil {
			return fmt.Errorf("store: insert acl visibility: %w", err)
		}
	}
	return nil
}

// AddVisibility inserts one (tenant, node, principal) row. Idempotent
// (INSERT OR IGNORE). Direct data-access helper; ACL semantics live in
// W1.9.
func (s *CanonicalStore) AddVisibility(ctx context.Context, tenantID, nodeID, principal string) error {
	return s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx,
			`INSERT OR IGNORE INTO node_visibility (tenant_id, node_id, principal) VALUES (?, ?, ?)`,
			tenantID, nodeID, principal,
		)
		if err != nil {
			return fmt.Errorf("store: add visibility: %w", err)
		}
		return nil
	})
}

// GetVisibleNodeIDs returns the subset of nodeIDs the actor can see by
// owner_actor or node_visibility (including the tenant:* wildcard).
// Used by read handlers to post-filter results before egress.
//
// This is the data-access slice of the ACL post-filter pattern at
// canonical_store.py:_sync_get_visible_nodes (2715). Full ACL semantics
// (typed capabilities, group resolution, inheritance) live in W1.9.
func (s *CanonicalStore) GetVisibleNodeIDs(ctx context.Context, tenantID string, actorIDs []string, nodeIDs []string) (map[string]struct{}, error) {
	if len(nodeIDs) == 0 {
		return map[string]struct{}{}, nil
	}
	if len(actorIDs) == 0 {
		return map[string]struct{}{}, nil
	}
	db, err := s.db(tenantID)
	if err != nil {
		return nil, err
	}
	// Build the parameterized IN clauses. tenant:* is included as a
	// visibility-only wildcard (matches Python "tenant:*" handling at
	// _sync_can_access:2887).
	visIDs := make([]string, 0, len(actorIDs)+1)
	visIDs = append(visIDs, actorIDs...)
	visIDs = append(visIDs, "tenant:*")

	nodePh := strings.Repeat("?,", len(nodeIDs))
	nodePh = nodePh[:len(nodePh)-1]
	actorPh := strings.Repeat("?,", len(actorIDs))
	actorPh = actorPh[:len(actorPh)-1]
	visPh := strings.Repeat("?,", len(visIDs))
	visPh = visPh[:len(visPh)-1]

	q := fmt.Sprintf(`
		SELECT DISTINCT n.node_id FROM nodes n
		LEFT JOIN node_visibility v
		    ON v.tenant_id = n.tenant_id AND v.node_id = n.node_id
		WHERE n.tenant_id = ? AND n.node_id IN (%s)
		  AND ( n.owner_actor IN (%s)
		     OR v.principal IN (%s) )`,
		nodePh, actorPh, visPh,
	)
	args := make([]any, 0, 1+len(nodeIDs)+len(actorIDs)+len(visIDs))
	args = append(args, tenantID)
	for _, id := range nodeIDs {
		args = append(args, id)
	}
	for _, a := range actorIDs {
		args = append(args, a)
	}
	for _, v := range visIDs {
		args = append(args, v)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: GetVisibleNodeIDs: %w", err)
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: GetVisibleNodeIDs scan: %w", err)
		}
		out[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: GetVisibleNodeIDs rows: %w", err)
	}
	return out, nil
}

// ListSharedWithMe returns nodes the actor (or any of their groups) has
// an explicit, non-deny, non-expired node_access entry on. Mirrors
// canonical_store.py:_sync_list_shared_with_me (3444).
//
// Pagination uses limit + offset. Cursor-based pagination can layer on
// top later (returning offset as the cursor is the simplest contract).
func (s *CanonicalStore) ListSharedWithMe(ctx context.Context, tenantID string, actorIDs []string, limit, offset int) ([]*Node, error) {
	if len(actorIDs) == 0 {
		return nil, nil
	}
	db, err := s.db(tenantID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	ph := strings.Repeat("?,", len(actorIDs))
	ph = ph[:len(ph)-1]
	q := fmt.Sprintf(`
		SELECT DISTINCT n.tenant_id, n.node_id, n.type_id, n.payload_json,
		                n.created_at, n.updated_at, n.owner_actor, n.acl_blob
		FROM nodes n
		JOIN node_access na ON na.node_id = n.node_id
		WHERE n.tenant_id = ?
		  AND na.actor_id IN (%s)
		  AND na.permission != 'deny'
		  AND (na.expires_at IS NULL OR na.expires_at > ?)
		ORDER BY na.granted_at DESC
		LIMIT ? OFFSET ?`, ph,
	)
	args := make([]any, 0, 4+len(actorIDs))
	args = append(args, tenantID)
	for _, a := range actorIDs {
		args = append(args, a)
	}
	args = append(args, s.now(), limit, offset)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: ListSharedWithMe: %w", err)
	}
	defer rows.Close()
	var out []*Node
	for rows.Next() {
		n := &Node{}
		if err := rows.Scan(&n.TenantID, &n.NodeID, &n.TypeID, &n.PayloadJSON,
			&n.CreatedAt, &n.UpdatedAt, &n.OwnerActor, &n.ACLJSON); err != nil {
			return nil, fmt.Errorf("store: ListSharedWithMe scan: %w", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: ListSharedWithMe rows: %w", err)
	}
	return out, nil
}
