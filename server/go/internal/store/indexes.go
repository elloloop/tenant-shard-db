package store

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// indexCache is the process-local cache of (db_path, type_id) ->
// {ensured-index-kinds}. Mirrors canonical_store.py:439-451 plus the
// FTS table cache (:447). Keyed by the physical DB path rather than
// tenant_id so mailbox / public stores (which multiplex tenants onto
// one file in Python) only emit one CREATE INDEX per type per file.
//
// keeps one DB per tenant so the path-vs-tenant_id distinction
// is moot, but mirroring the Python shape simplifies any future
// re-introduction of shared physical files.
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
// Mirrors canonical_store.py:_ensure_unique_indexes (1721). Index name:
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
// indexes. Each constraint is (constraint_name, field_ids...). Mirrors
// canonical_store.py:_ensure_composite_unique_indexes (1800).
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
		safe := safeIdent(c.Name)
		if safe == "" {
			parts := make([]string, 0, len(c.FieldIDs))
			for _, f := range c.FieldIDs {
				parts = append(parts, fmt.Sprintf("%d", f))
			}
			safe = "f" + strings.Join(parts, "_")
		}
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
// fields declared (entdb.field).indexed = true. Mirrors
// canonical_store.py:_ensure_query_indexes (1876).
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

// EnsureFTSIndex creates an FTS5 virtual table for typeID with one
// column per searchable field_id. Mirrors canonical_store.py:
// _ensure_fts_table (1993). tokenize='porter unicode61' for stemming +
// Unicode handling.
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

	cols := make([]string, 0, len(fieldIDs))
	for _, f := range fieldIDs {
		cols = append(cols, fmt.Sprintf("f%d", f))
	}
	stmt := fmt.Sprintf(
		`CREATE VIRTUAL TABLE IF NOT EXISTS fts_t%d USING fts5(`+
			`node_id UNINDEXED, %s, tokenize='porter unicode61')`,
		typeID, strings.Join(cols, ", "),
	)
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("store: create FTS table t%d: %w", typeID, err)
	}
	s.indexCache.mu.Lock()
	s.indexCache.ftsDone[key] = struct{}{}
	s.indexCache.mu.Unlock()
	return nil
}

// CompositeUnique is the input shape for EnsureCompositeUniqueIndex.
// Mirrors schema.CompositeUniqueDef but is decoupled so this package
// can be exercised in tests without a full schema.Registry.
type CompositeUnique struct {
	// Name is the constraint name (becomes the index suffix). Must be
	// a valid SQL identifier or it is sanitized.
	Name string
	// FieldIDs are the field_ids forming the composite key, in
	// declaration order.
	FieldIDs []uint32
}

// safeIdent strips characters that are not valid in a SQL identifier.
// Mirrors canonical_store.py:1862.
func safeIdent(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '_':
			b.WriteByte(c)
		default:
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

// ensureFieldIndexes is the convenience wrapper used by writers before
// CreateNode / UpdateNode. Mirrors canonical_store.py:_ensure_field_indexes
// (1942). Skips silently when no schema.Registry is configured.
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
			cu = append(cu, CompositeUnique{Name: c.Name, FieldIDs: c.FieldIDs})
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
