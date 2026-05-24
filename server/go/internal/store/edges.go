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
func (s *CanonicalStore) GetEdgesFrom(ctx context.Context, tenantID, nodeID string, edgeTypeID *int32, limit int) ([]*Edge, error) {
	return s.getEdges(ctx, tenantID, nodeID, edgeTypeID, true, limit, nil)
}

// GetEdgesTo returns incoming edges to nodeID.
func (s *CanonicalStore) GetEdgesTo(ctx context.Context, tenantID, nodeID string, edgeTypeID *int32, limit int) ([]*Edge, error) {
	return s.getEdges(ctx, tenantID, nodeID, edgeTypeID, false, limit, nil)
}

// EdgeCursor anchors a keyset seek over the edge list (ADR-029). The
// effective sort is (created_at DESC, edge_type_id DESC, peer DESC) where
// peer is to_node_id for outgoing edges and from_node_id for incoming —
// a total order, since (edge_type_id, peer) uniquely identifies an edge
// for a fixed source/target node. The seek resumes strictly after this
// tuple.
type EdgeCursor struct {
	CreatedAt  int64
	EdgeTypeID int32
	PeerNodeID string
}

// EdgeCursorFrom returns the keyset anchor for e in the given direction
// (outgoing ⇒ peer is to_node_id; incoming ⇒ from_node_id).
func EdgeCursorFrom(e *Edge, outgoing bool) EdgeCursor {
	peer := e.ToNodeID
	if !outgoing {
		peer = e.FromNodeID
	}
	return EdgeCursor{CreatedAt: e.CreatedAt, EdgeTypeID: e.EdgeTypeID, PeerNodeID: peer}
}

// GetEdgesFromPaged is GetEdgesFrom with a keyset cursor (ADR-029).
func (s *CanonicalStore) GetEdgesFromPaged(ctx context.Context, tenantID, nodeID string, edgeTypeID *int32, limit int, cursor *EdgeCursor) ([]*Edge, error) {
	return s.getEdges(ctx, tenantID, nodeID, edgeTypeID, true, limit, cursor)
}

// GetEdgesToPaged is GetEdgesTo with a keyset cursor (ADR-029).
func (s *CanonicalStore) GetEdgesToPaged(ctx context.Context, tenantID, nodeID string, edgeTypeID *int32, limit int, cursor *EdgeCursor) ([]*Edge, error) {
	return s.getEdges(ctx, tenantID, nodeID, edgeTypeID, false, limit, cursor)
}

func (s *CanonicalStore) getEdges(ctx context.Context, tenantID, nodeID string, edgeTypeID *int32, outgoing bool, limit int, cursor *EdgeCursor) ([]*Edge, error) {
	db, err := s.readDB(tenantID)
	if err != nil {
		return nil, err
	}
	col := "from_node_id"
	peer := "to_node_id"
	if !outgoing {
		col = "to_node_id"
		peer = "from_node_id"
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
	// Keyset seek (ADR-029): resume strictly after the cursor tuple in the
	// effective DESC order (created_at, edge_type_id, peer). The expanded
	// disjunction avoids relying on SQLite row-value comparison.
	if cursor != nil {
		q += fmt.Sprintf(
			` AND (created_at < ? OR (created_at = ? AND edge_type_id < ?)`+
				` OR (created_at = ? AND edge_type_id = ? AND %s < ?))`, peer)
		args = append(args,
			cursor.CreatedAt,
			cursor.CreatedAt, cursor.EdgeTypeID,
			cursor.CreatedAt, cursor.EdgeTypeID, cursor.PeerNodeID)
	}
	// Total order so the keyset cursor is unambiguous (peer uniquely
	// identifies an edge for a fixed source/target node).
	q += fmt.Sprintf(` ORDER BY created_at DESC, edge_type_id DESC, %s DESC`, peer)
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

// CreateEdge inserts (or replaces) an edge row.
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
// ErrEdgeNotFound if no row was deleted.
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
