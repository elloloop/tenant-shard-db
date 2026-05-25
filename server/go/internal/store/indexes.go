package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
)

// indexCache is the process-local cache of (db_path, type_id) ->
// {ensured-index-kinds}. Keyed by the physical DB path rather than
// tenant_id so any future re-introduction of shared physical files
// only emits one CREATE INDEX per type per file.
type indexCache struct {
	mu            sync.Mutex
	uniqueDone    map[indexKey]struct{}
	queryDone     map[indexKey]struct{}
	compositeDone map[indexKey]struct{}
	ftsDone       map[indexKey]struct{}
}

type indexKey struct {
	dbPath string
	typeID int32
}

func newIndexCache() *indexCache {
	return &indexCache{
		uniqueDone:    map[indexKey]struct{}{},
		queryDone:     map[indexKey]struct{}{},
		compositeDone: map[indexKey]struct{}{},
		ftsDone:       map[indexKey]struct{}{},
	}
}

// EnsureUniqueIndex creates a partial unique expression index for each
// (type_id, field_id) pair on demand. Idempotent at both the SQL level
// (IF NOT EXISTS) and the in-memory cache level.
//
// Index name:
//
//	idx_unique_t<type>_f<field>
//	  ON nodes(tenant_id, json_extract(payload_json, '$."<field>"'))
//	  WHERE type_id = <type>
//
// type_id and field_id come from the schema registry and are constrained
// to integers — interpolating them into the DDL is safe (no user input).
func (s *CanonicalStore) EnsureUniqueIndex(ctx context.Context, tenantID string, typeID int32, fieldIDs []uint32) error {
	if len(fieldIDs) == 0 {
		return nil
	}
	db, err := s.db(tenantID)
	if err != nil {
		return err
	}
	key := indexKey{dbPath: s.pool.dbPath(tenantID), typeID: typeID}
	s.indexCache.mu.Lock()
	if _, ok := s.indexCache.uniqueDone[key]; ok {
		s.indexCache.mu.Unlock()
		return nil
	}
	s.indexCache.mu.Unlock()

	for _, fid := range fieldIDs {
		stmt := fmt.Sprintf(
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_t%d_f%d `+
				`ON nodes(tenant_id, json_extract(payload_json, '$."%d"')) `+
				`WHERE type_id = %d`,
			typeID, fid, fid, typeID,
		)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("store: create unique index t%d_f%d: %w", typeID, fid, err)
		}
	}
	s.indexCache.mu.Lock()
	s.indexCache.uniqueDone[key] = struct{}{}
	s.indexCache.mu.Unlock()
	return nil
}

// EnsureCompositeUniqueIndex creates partial composite-unique expression
// indexes. Each constraint is identified by its field_ids tuple
// (name-free, ADR-031); the index suffix is derived from that tuple.
func (s *CanonicalStore) EnsureCompositeUniqueIndex(ctx context.Context, tenantID string, typeID int32, constraints []CompositeUnique) error {
	if len(constraints) == 0 {
		return nil
	}
	db, err := s.db(tenantID)
	if err != nil {
		return err
	}
	key := indexKey{dbPath: s.pool.dbPath(tenantID), typeID: typeID}
	s.indexCache.mu.Lock()
	if _, ok := s.indexCache.compositeDone[key]; ok {
		s.indexCache.mu.Unlock()
		return nil
	}
	s.indexCache.mu.Unlock()

	for _, c := range constraints {
		safe := compositeIndexSuffix(c.FieldIDs)
		extracts := make([]string, 0, len(c.FieldIDs))
		for _, f := range c.FieldIDs {
			extracts = append(extracts, fmt.Sprintf(`json_extract(payload_json, '$."%d"')`, f))
		}
		stmt := fmt.Sprintf(
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_t%d_c%s `+
				`ON nodes(tenant_id, %s) WHERE type_id = %d`,
			typeID, safe, strings.Join(extracts, ", "), typeID,
		)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("store: create composite unique index t%d_c%s: %w", typeID, safe, err)
		}
	}
	s.indexCache.mu.Lock()
	s.indexCache.compositeDone[key] = struct{}{}
	s.indexCache.mu.Unlock()
	return nil
}

// EnsureQueryIndex creates non-unique partial expression indexes for
// fields declared (entdb.field).indexed = true.
func (s *CanonicalStore) EnsureQueryIndex(ctx context.Context, tenantID string, typeID int32, fieldIDs []uint32) error {
	if len(fieldIDs) == 0 {
		return nil
	}
	db, err := s.db(tenantID)
	if err != nil {
		return err
	}
	key := indexKey{dbPath: s.pool.dbPath(tenantID), typeID: typeID}
	s.indexCache.mu.Lock()
	if _, ok := s.indexCache.queryDone[key]; ok {
		s.indexCache.mu.Unlock()
		return nil
	}
	s.indexCache.mu.Unlock()

	for _, fid := range fieldIDs {
		stmt := fmt.Sprintf(
			`CREATE INDEX IF NOT EXISTS idx_query_t%d_f%d `+
				`ON nodes(tenant_id, json_extract(payload_json, '$."%d"')) `+
				`WHERE type_id = %d`,
			typeID, fid, fid, typeID,
		)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("store: create query index t%d_f%d: %w", typeID, fid, err)
		}
	}
	s.indexCache.mu.Lock()
	s.indexCache.queryDone[key] = struct{}{}
	s.indexCache.mu.Unlock()
	return nil
}

// ftsCreateStmt builds the CREATE VIRTUAL TABLE statement for typeID.
func ftsCreateStmt(typeID int32, fieldIDs []uint32) string {
	cols := make([]string, 0, len(fieldIDs))
	for _, f := range fieldIDs {
		cols = append(cols, fmt.Sprintf("f%d", f))
	}
	return fmt.Sprintf(
		`CREATE VIRTUAL TABLE IF NOT EXISTS fts_t%d USING fts5(`+
			`node_id UNINDEXED, %s, tokenize='porter unicode61')`,
		typeID, strings.Join(cols, ", "),
	)
}

// EnsureFTSIndex creates an FTS5 virtual table for typeID with one
// column per searchable field_id. tokenize='porter unicode61' for
// stemming + Unicode handling. Uses a pooled connection — callers that
// already hold an open BatchTxn write transaction must use
// EnsureFTSIndexConn instead, or the DDL deadlocks on the SQLite write
// lock the open txn already holds.
func (s *CanonicalStore) EnsureFTSIndex(ctx context.Context, tenantID string, typeID int32, fieldIDs []uint32) error {
	if len(fieldIDs) == 0 {
		return nil
	}
	db, err := s.db(tenantID)
	if err != nil {
		return err
	}
	key := indexKey{dbPath: s.pool.dbPath(tenantID), typeID: typeID}
	s.indexCache.mu.Lock()
	if _, ok := s.indexCache.ftsDone[key]; ok {
		s.indexCache.mu.Unlock()
		return nil
	}
	s.indexCache.mu.Unlock()

	if _, err := db.ExecContext(ctx, ftsCreateStmt(typeID, fieldIDs)); err != nil {
		return fmt.Errorf("store: create FTS table t%d: %w", typeID, err)
	}
	s.indexCache.mu.Lock()
	s.indexCache.ftsDone[key] = struct{}{}
	s.indexCache.mu.Unlock()
	return nil
}

// EnsureFTSIndexConn creates the FTS5 virtual table for typeID on the
// supplied connection — used by the applier so the DDL runs inside the
// same BEGIN IMMEDIATE transaction that holds the per-tenant write lock,
// avoiding the cross-connection deadlock EnsureFTSIndex would hit.
// Idempotent: the in-memory ftsDone cache short-circuits repeats.
func (s *CanonicalStore) EnsureFTSIndexConn(ctx context.Context, conn *sql.Conn, tenantID string, typeID int32, fieldIDs []uint32) error {
	if len(fieldIDs) == 0 {
		return nil
	}
	key := indexKey{dbPath: s.pool.dbPath(tenantID), typeID: typeID}
	s.indexCache.mu.Lock()
	if _, ok := s.indexCache.ftsDone[key]; ok {
		s.indexCache.mu.Unlock()
		return nil
	}
	s.indexCache.mu.Unlock()

	if _, err := conn.ExecContext(ctx, ftsCreateStmt(typeID, fieldIDs)); err != nil {
		return fmt.Errorf("store: create FTS table t%d: %w", typeID, err)
	}
	s.indexCache.mu.Lock()
	s.indexCache.ftsDone[key] = struct{}{}
	s.indexCache.mu.Unlock()
	return nil
}

// CompositeUnique is the input shape for EnsureCompositeUniqueIndex.
// Mirrors schema.CompositeUniqueDef but is decoupled so this package
// can be exercised in tests without a full schema.Registry. Name-free
// (ADR-031): a composite constraint is identified by its FieldIDs tuple.
type CompositeUnique struct {
	// FieldIDs are the field_ids forming the composite key, in
	// declaration order.
	FieldIDs []uint32
}

// uniqueIndexDDL returns the CREATE UNIQUE INDEX statement for a
// single-field unique constraint. Shared by the lazy (separate-conn)
// and the in-txn ensure paths so the index name + expression stay
// byte-identical.
func uniqueIndexDDL(typeID int32, fid uint32) string {
	return fmt.Sprintf(
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_t%d_f%d `+
			`ON nodes(tenant_id, json_extract(payload_json, '$."%d"')) `+
			`WHERE type_id = %d`,
		typeID, fid, fid, typeID,
	)
}

// compositeUniqueIndexDDL returns the CREATE UNIQUE INDEX statement for
// a composite constraint. Mirrors EnsureCompositeUniqueIndex exactly.
func compositeUniqueIndexDDL(typeID int32, c CompositeUnique) string {
	safe := compositeIndexSuffix(c.FieldIDs)
	extracts := make([]string, 0, len(c.FieldIDs))
	for _, f := range c.FieldIDs {
		extracts = append(extracts, fmt.Sprintf(`json_extract(payload_json, '$."%d"')`, f))
	}
	return fmt.Sprintf(
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_t%d_c%s `+
			`ON nodes(tenant_id, %s) WHERE type_id = %d`,
		typeID, safe, strings.Join(extracts, ", "), typeID,
	)
}

// queryIndexDDL returns the non-unique query-index statement.
func queryIndexDDL(typeID int32, fid uint32) string {
	return fmt.Sprintf(
		`CREATE INDEX IF NOT EXISTS idx_query_t%d_f%d `+
			`ON nodes(tenant_id, json_extract(payload_json, '$."%d"')) `+
			`WHERE type_id = %d`,
		typeID, fid, fid, typeID,
	)
}

// EnsureFieldIndexesTx creates the unique / composite-unique / query
// expression indexes for typeID inside an already-open BatchTxn, using
// the txn's own connection. This is the applier's hook (ADR-023 / the
// composite-unique ADR): the unique indexes MUST exist on the write
// connection BEFORE the INSERT so the constraint fires synchronously
// within the same transaction. Running the DDL on a separate handle
// (as the lazy Ensure*Index methods do) would deadlock against the
// BatchTxn's held writer lock.
//
// Deliberately NOT cached in indexCache: a batch that creates an index
// and then rolls back (e.g. because the INSERT it guards trips the very
// constraint just created) drops the index along with the txn. A cache
// hit on the next batch would then skip re-creation and silently leave
// the constraint unenforced. CREATE ... IF NOT EXISTS against an
// already-committed index is a cheap metadata-only check, so re-running
// the DDL per first-touch is correct and inexpensive. FTS tables are
// NOT created here — the applier maintains those via fts.go.
//
// No-ops when no schema.Registry is configured (raw-CRUD test paths).
func (s *CanonicalStore) EnsureFieldIndexesTx(ctx context.Context, tx *BatchTxn, typeID int32) error {
	if s.registry == nil {
		return nil
	}
	conn := tx.Conn()
	for _, fid := range s.registry.UniqueFieldIDs(typeID) {
		if _, err := conn.ExecContext(ctx, uniqueIndexDDL(typeID, fid)); err != nil {
			return fmt.Errorf("store: create unique index (tx) t%d_f%d: %w", typeID, fid, err)
		}
	}
	for _, c := range s.registry.CompositeUnique(typeID) {
		cu := CompositeUnique{FieldIDs: c.FieldIDs}
		if _, err := conn.ExecContext(ctx, compositeUniqueIndexDDL(typeID, cu)); err != nil {
			return fmt.Errorf("store: create composite unique index (tx) t%d_c%s: %w",
				typeID, compositeIndexSuffix(c.FieldIDs), err)
		}
	}
	for _, fid := range s.registry.IndexedFieldIDs(typeID) {
		if _, err := conn.ExecContext(ctx, queryIndexDDL(typeID, fid)); err != nil {
			return fmt.Errorf("store: create query index (tx) t%d_f%d: %w", typeID, fid, err)
		}
	}
	return nil
}

// ensureFieldIndexes is the convenience wrapper used by writers before
// CreateNode / UpdateNode. Skips silently when no schema.Registry is
// configured.
func (s *CanonicalStore) ensureFieldIndexes(ctx context.Context, tenantID string, typeID int32) error {
	if s.registry == nil {
		return nil
	}
	if uniq := s.registry.UniqueFieldIDs(typeID); len(uniq) > 0 {
		if err := s.EnsureUniqueIndex(ctx, tenantID, typeID, uniq); err != nil {
			return err
		}
	}
	if comps := s.registry.CompositeUnique(typeID); len(comps) > 0 {
		cu := make([]CompositeUnique, 0, len(comps))
		for _, c := range comps {
			cu = append(cu, CompositeUnique{FieldIDs: c.FieldIDs})
		}
		if err := s.EnsureCompositeUniqueIndex(ctx, tenantID, typeID, cu); err != nil {
			return err
		}
	}
	if idx := s.registry.IndexedFieldIDs(typeID); len(idx) > 0 {
		if err := s.EnsureQueryIndex(ctx, tenantID, typeID, idx); err != nil {
			return err
		}
	}
	if fts := s.registry.SearchableFieldIDs(typeID); len(fts) > 0 {
		if err := s.EnsureFTSIndex(ctx, tenantID, typeID, fts); err != nil {
			return err
		}
	}
	return nil
}
