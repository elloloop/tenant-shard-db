package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// SearchNodes runs an FTS5 MATCH query over the searchable fields of
// typeID. Returns up to limit nodes ordered by FTS rank. Lazy-creates
// the FTS table on first call.
//
// query is a raw FTS5 match expression. The string is bound as a
// parameter — no concatenation — so SQL injection is impossible at the
// FTS level. (FTS5 itself parses the match expression and rejects
// malformed input; that's an FTS error, not a SQL injection vector.)
func (s *CanonicalStore) SearchNodes(ctx context.Context, tenantID string, typeID int32, query string, searchableFieldIDs []uint32, limit, offset int) ([]*Node, error) {
	return s.searchNodes(ctx, tenantID, typeID, query, searchableFieldIDs, "", limit, offset)
}

// SearchMailboxNodes runs the same FTS5 MATCH as SearchNodes but scopes
// the result to a single user's mailbox: only USER_MAILBOX nodes with
// target_user_id == targetUser are returned. targetUser must be
// non-empty (the api layer guarantees this).
func (s *CanonicalStore) SearchMailboxNodes(ctx context.Context, tenantID, targetUser string, typeID int32, query string, searchableFieldIDs []uint32, limit, offset int) ([]*Node, error) {
	if targetUser == "" {
		return nil, nil
	}
	return s.searchNodes(ctx, tenantID, typeID, query, searchableFieldIDs, targetUser, limit, offset)
}

// searchNodes is the shared FTS5 search core. When mailboxUser != "" the
// JOIN is constrained to that user's USER_MAILBOX nodes; otherwise it
// behaves exactly as the pre-#568 SearchNodes (no mailbox predicate).
func (s *CanonicalStore) searchNodes(ctx context.Context, tenantID string, typeID int32, query string, searchableFieldIDs []uint32, mailboxUser string, limit, offset int) ([]*Node, error) {
	if len(searchableFieldIDs) == 0 {
		return nil, nil
	}
	if err := s.EnsureFTSIndex(ctx, tenantID, typeID, searchableFieldIDs); err != nil {
		return nil, err
	}
	db, err := s.readDB(tenantID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	tableName := fmt.Sprintf("fts_t%d", typeID)
	// Mailbox scope (#568, ADR-020): a USER_MAILBOX node never leaks into a
	// tenant-scoped search. mailboxUser set -> only that user's mailbox
	// nodes; empty -> exclude every mailbox node.
	mailboxPredicate := ""
	args := []any{tenantID}
	if mailboxUser != "" {
		mailboxPredicate = " AND n.target_user_id = ? AND n.storage_mode = ?"
		args = append(args, mailboxUser, int32(StorageModeUserMailbox))
	} else {
		mailboxPredicate = " AND n.storage_mode <> ?"
		args = append(args, int32(StorageModeUserMailbox))
	}
	args = append(args, query, limit, offset)
	q := fmt.Sprintf(`
		SELECT n.tenant_id, n.node_id, n.type_id, n.payload_json,
		       n.created_at, n.updated_at, n.owner_actor, n.acl_blob,
		       n.storage_mode, n.target_user_id
		FROM %s AS fts
		JOIN nodes AS n ON n.node_id = fts.node_id AND n.tenant_id = ?%s
		WHERE %s MATCH ?
		ORDER BY fts.rank
		LIMIT ? OFFSET ?`, tableName, mailboxPredicate, tableName,
	)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: SearchNodes: %w", err)
	}
	defer rows.Close()
	var out []*Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
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
// virtual table. payload is the field-id-keyed map; missing fields are
// inserted as empty strings (FTS5 requires the column to be present).
//
// This opens its own write transaction. The applier instead uses
// FTSInsertConn so the index row commits atomically with the node row.
func (s *CanonicalStore) FTSInsert(ctx context.Context, tenantID string, typeID int32, nodeID string, payload map[string]any, searchableFieldIDs []uint32) error {
	if len(searchableFieldIDs) == 0 {
		return nil
	}
	return s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		return FTSInsertConn(ctx, conn, typeID, nodeID, payload, searchableFieldIDs)
	})
}

// FTSInsertConn writes one node's FTS row on an existing connection (the
// applier's BatchTxn connection), so the index is committed in the same
// transaction as the node insert. The caller must have already ensured
// the virtual table exists (EnsureFTSIndex).
func FTSInsertConn(ctx context.Context, conn *sql.Conn, typeID int32, nodeID string, payload map[string]any, searchableFieldIDs []uint32) error {
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
	if _, err := conn.ExecContext(ctx, q, values...); err != nil {
		return fmt.Errorf("store: FTSInsert: %w", err)
	}
	return nil
}

// FTSDelete removes a node's row from the per-type FTS5 virtual table.
func (s *CanonicalStore) FTSDelete(ctx context.Context, tenantID string, typeID int32, nodeID string) error {
	return s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		return FTSDeleteConn(ctx, conn, typeID, nodeID)
	})
}

// FTSDeleteConn removes a node's FTS row on an existing connection. The
// caller must guard with a type-has-searchable check, because the DELETE
// errors if the per-type virtual table was never created.
func FTSDeleteConn(ctx context.Context, conn *sql.Conn, typeID int32, nodeID string) error {
	if _, err := conn.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM fts_t%d WHERE node_id = ?`, typeID),
		nodeID,
	); err != nil {
		return fmt.Errorf("store: FTSDelete: %w", err)
	}
	return nil
}
