package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// SearchNodes runs an FTS5 MATCH query over the searchable fields of
// typeID. Mirrors canonical_store.py:_sync_search_nodes (2101). Returns
// up to limit nodes ordered by FTS rank. Lazy-creates the FTS table on
// first call.
//
// query is a raw FTS5 match expression. The string is bound as a
// parameter — no concatenation — so SQL injection is impossible at the
// FTS level. (FTS5 itself parses the match expression and rejects
// malformed input; that's an FTS error, not a SQL injection vector.)
func (s *CanonicalStore) SearchNodes(ctx context.Context, tenantID string, typeID int32, query string, searchableFieldIDs []uint32, limit, offset int) ([]*Node, error) {
	if len(searchableFieldIDs) == 0 {
		return nil, nil
	}
	if err := s.EnsureFTSIndex(ctx, tenantID, typeID, searchableFieldIDs); err != nil {
		return nil, err
	}
	db, err := s.db(tenantID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	tableName := fmt.Sprintf("fts_t%d", typeID)
	q := fmt.Sprintf(`
		SELECT n.tenant_id, n.node_id, n.type_id, n.payload_json,
		       n.created_at, n.updated_at, n.owner_actor, n.acl_blob
		FROM %s AS fts
		JOIN nodes AS n ON n.node_id = fts.node_id AND n.tenant_id = ?
		WHERE %s MATCH ?
		ORDER BY fts.rank
		LIMIT ? OFFSET ?`, tableName, tableName,
	)
	rows, err := db.QueryContext(ctx, q, tenantID, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("store: SearchNodes: %w", err)
	}
	defer rows.Close()
	var out []*Node
	for rows.Next() {
		n := &Node{}
		if err := rows.Scan(&n.TenantID, &n.NodeID, &n.TypeID, &n.PayloadJSON,
			&n.CreatedAt, &n.UpdatedAt, &n.OwnerActor, &n.ACLJSON); err != nil {
			return nil, fmt.Errorf("store: SearchNodes scan: %w", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: SearchNodes rows: %w", err)
	}
	return out, nil
}

// FTSInsert indexes one node's searchable fields into the per-type FTS5
// virtual table. Mirrors canonical_store.py:_sync_fts_insert (2039).
// Called by the applier on CreateNode. payload is the field-id-keyed
// map; missing fields are inserted as empty strings (FTS5 requires the
// column to be present).
func (s *CanonicalStore) FTSInsert(ctx context.Context, tenantID string, typeID int32, nodeID string, payload map[string]any, searchableFieldIDs []uint32) error {
	if len(searchableFieldIDs) == 0 {
		return nil
	}
	colNames := make([]string, 0, len(searchableFieldIDs))
	placeholders := make([]string, 0, len(searchableFieldIDs))
	values := make([]any, 0, len(searchableFieldIDs)+1)
	values = append(values, nodeID)
	for _, fid := range searchableFieldIDs {
		colNames = append(colNames, fmt.Sprintf("f%d", fid))
		placeholders = append(placeholders, "?")
		v := payload[fmt.Sprintf("%d", fid)]
		if v == nil {
			values = append(values, "")
		} else {
			values = append(values, fmt.Sprintf("%v", v))
		}
	}
	q := fmt.Sprintf(
		`INSERT INTO fts_t%d(node_id, %s) VALUES (?, %s)`,
		typeID, strings.Join(colNames, ", "), strings.Join(placeholders, ", "),
	)
	return s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, q, values...)
		if err != nil {
			return fmt.Errorf("store: FTSInsert: %w", err)
		}
		return nil
	})
}

// FTSDelete removes a node's row from the per-type FTS5 virtual table.
// Mirrors canonical_store.py:_sync_fts_delete (2059).
func (s *CanonicalStore) FTSDelete(ctx context.Context, tenantID string, typeID int32, nodeID string) error {
	return s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx,
			fmt.Sprintf(`DELETE FROM fts_t%d WHERE node_id = ?`, typeID),
			nodeID,
		)
		if err != nil {
			return fmt.Errorf("store: FTSDelete: %w", err)
		}
		return nil
	})
}
