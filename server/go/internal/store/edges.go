package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// Edge is the in-Go representation of one row of the `edges` table.
// PropsJSON is opaque (caller responsibility); created_at is unix ms.
type Edge struct {
	TenantID      string
	EdgeTypeID    int32
	FromNodeID    string
	ToNodeID      string
	PropsJSON     string
	PropagatesACL bool
	CreatedAt     int64
}

// EdgeInput is the input shape of CreateEdge.
type EdgeInput struct {
	EdgeTypeID    int32
	FromNodeID    string
	ToNodeID      string
	Props         map[string]any
	PropagatesACL bool
	CreatedAt     int64 // 0 -> store.now()
}

// GetEdgesFrom returns outgoing edges from nodeID. If edgeTypeID is
// non-nil it filters by edge type; otherwise returns every edge type.
// Mirrors canonical_store.py:_sync_get_edges_from (2609).
func (s *CanonicalStore) GetEdgesFrom(ctx context.Context, tenantID, nodeID string, edgeTypeID *int32, limit int) ([]*Edge, error) {
	return s.getEdges(ctx, tenantID, nodeID, edgeTypeID, true, limit)
}

// GetEdgesTo returns incoming edges to nodeID. Mirrors
// canonical_store.py:_sync_get_edges_to (2662).
func (s *CanonicalStore) GetEdgesTo(ctx context.Context, tenantID, nodeID string, edgeTypeID *int32, limit int) ([]*Edge, error) {
	return s.getEdges(ctx, tenantID, nodeID, edgeTypeID, false, limit)
}

func (s *CanonicalStore) getEdges(ctx context.Context, tenantID, nodeID string, edgeTypeID *int32, outgoing bool, limit int) ([]*Edge, error) {
	db, err := s.db(tenantID)
	if err != nil {
		return nil, err
	}
	col := "from_node_id"
	if !outgoing {
		col = "to_node_id"
	}
	q := fmt.Sprintf(
		`SELECT tenant_id, edge_type_id, from_node_id, to_node_id,
		        props_json, propagates_acl, created_at
		 FROM edges WHERE tenant_id = ? AND %s = ?`, col,
	)
	args := []any{tenantID, nodeID}
	if edgeTypeID != nil {
		q += ` AND edge_type_id = ?`
		args = append(args, *edgeTypeID)
	}
	q += ` ORDER BY created_at DESC`
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: getEdges: %w", err)
	}
	defer rows.Close()
	var out []*Edge
	for rows.Next() {
		e := &Edge{}
		var propagates int
		if err := rows.Scan(&e.TenantID, &e.EdgeTypeID, &e.FromNodeID, &e.ToNodeID,
			&e.PropsJSON, &propagates, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: getEdges scan: %w", err)
		}
		e.PropagatesACL = propagates != 0
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: getEdges rows: %w", err)
	}
	return out, nil
}

// CreateEdge inserts (or replaces) an edge row. INSERT OR REPLACE
// matches canonical_store.py:_sync_create_edge (2476).
func (s *CanonicalStore) CreateEdge(ctx context.Context, tenantID string, in EdgeInput) (*Edge, error) {
	if in.FromNodeID == "" || in.ToNodeID == "" {
		return nil, fmt.Errorf("store: CreateEdge: from/to node ids required")
	}
	props := in.Props
	if props == nil {
		props = map[string]any{}
	}
	propsStr, err := json.Marshal(props)
	if err != nil {
		return nil, fmt.Errorf("store: marshal props: %w", err)
	}
	now := in.CreatedAt
	if now == 0 {
		now = s.now()
	}
	propagates := 0
	if in.PropagatesACL {
		propagates = 1
	}
	err = s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, `
			INSERT OR REPLACE INTO edges
			    (tenant_id, edge_type_id, from_node_id, to_node_id,
			     props_json, propagates_acl, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			tenantID, in.EdgeTypeID, in.FromNodeID, in.ToNodeID,
			string(propsStr), propagates, now,
		)
		if err != nil {
			return fmt.Errorf("store: insert edge: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &Edge{
		TenantID:      tenantID,
		EdgeTypeID:    in.EdgeTypeID,
		FromNodeID:    in.FromNodeID,
		ToNodeID:      in.ToNodeID,
		PropsJSON:     string(propsStr),
		PropagatesACL: in.PropagatesACL,
		CreatedAt:     now,
	}, nil
}

// DeleteEdge removes an edge by (edge_type_id, from, to). Returns
// ErrEdgeNotFound if no row was deleted. Mirrors
// canonical_store.py:_sync_delete_edge (2564).
func (s *CanonicalStore) DeleteEdge(ctx context.Context, tenantID string, edgeTypeID int32, fromNodeID, toNodeID string) error {
	return s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		res, err := conn.ExecContext(ctx, `
			DELETE FROM edges
			WHERE tenant_id = ? AND edge_type_id = ?
			  AND from_node_id = ? AND to_node_id = ?`,
			tenantID, edgeTypeID, fromNodeID, toNodeID,
		)
		if err != nil {
			return fmt.Errorf("store: delete edge: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return fmt.Errorf("%w: %s/%d/%s->%s", ErrEdgeNotFound, tenantID, edgeTypeID, fromNodeID, toNodeID)
		}
		return nil
	})
}
