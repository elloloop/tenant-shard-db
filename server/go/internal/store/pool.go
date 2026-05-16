package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	entcrypto "github.com/elloloop/tenant-shard-db/server/go/internal/crypto"

	// modernc.org/sqlite registers the "sqlite" driver via init().
	_ "modernc.org/sqlite"
)

// validateTenantID is the Go port of canonical_store.py:_validate_filesystem_id
// (138-162). The id MUST match [A-Za-z0-9_-]{1,128}; anything else is
// rejected outright. The cost of a false reject (an exotic id) is much
// smaller than the cost of a false accept (two ids collapsing to the
// same file and leaking data across tenants — CLAUDE.md invariant #4).
func validateTenantID(tenantID string) error {
	if tenantID == "" {
		return fmt.Errorf("%w: empty tenant_id", ErrInvalidTenantID)
	}
	if len(tenantID) > 128 {
		return fmt.Errorf("%w: tenant_id length %d exceeds 128", ErrInvalidTenantID, len(tenantID))
	}
	for i := 0; i < len(tenantID); i++ {
		c := tenantID[i]
		ok := (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_'
		if !ok {
			return fmt.Errorf("%w: tenant_id contains forbidden character %q", ErrInvalidTenantID, c)
		}
	}
	return nil
}

// poolEntry is one tenant's pooled handle: the *sql.DB plus the
// per-tenant writer mutex. Mirrors the Python pattern of
// (pooled connection + per-tenant threading.Lock).
type poolEntry struct {
	db *sql.DB
	// writeMu serializes writers on this tenant's DB. Readers do NOT
	// take it; SQLite WAL mode + per-connection isolation handles them.
	// This matches canonical_store.py:_get_tenant_lock (642) which only
	// guards the single-pooled-writer path in Python.
	writeMu sync.Mutex
}

// pool is the per-tenant connection registry. Keyed by tenant_id;
// each entry holds an opened *sql.DB pinned to a single connection
// (single-writer SQLite — see canonical-store.md "Concurrency").
type pool struct {
	mu      sync.RWMutex
	entries map[string]*poolEntry

	// rootDir is the directory tenant DBs live in; created at New() time.
	rootDir string

	// busyTimeout is the SQLite busy_timeout applied via DSN. 0 -> 5s.
	busyTimeout time.Duration

	// walMode toggles PRAGMA journal_mode=WAL. true in production.
	walMode bool

	// keyManager switches tenant DB opens to SQLCipher.
	keyManager *entcrypto.KeyManager

	// encryptionRequired rejects plaintext files and missing key config.
	encryptionRequired bool
}

type poolOptions struct {
	rootDir            string
	busyTimeout        time.Duration
	walMode            bool
	keyManager         *entcrypto.KeyManager
	encryptionRequired bool
}

// newPool returns a pool ready to lazy-open tenant DBs. The rootDir is
// created with 0o755 if it does not already exist.
func newPool(opts poolOptions) (*pool, error) {
	if opts.rootDir == "" {
		return nil, fmt.Errorf("store: rootDir is required")
	}
	if opts.encryptionRequired && opts.keyManager == nil {
		return nil, fmt.Errorf("store: encryption required but no key manager configured")
	}
	if err := os.MkdirAll(opts.rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("store: mkdir %q: %w", opts.rootDir, err)
	}
	if opts.busyTimeout <= 0 {
		opts.busyTimeout = 5 * time.Second
	}
	return &pool{
		entries:            map[string]*poolEntry{},
		rootDir:            opts.rootDir,
		busyTimeout:        opts.busyTimeout,
		walMode:            opts.walMode,
		keyManager:         opts.keyManager,
		encryptionRequired: opts.encryptionRequired,
	}, nil
}

// dbPath returns the absolute path of tenant_<id>.db inside the pool's
// rootDir. Mirrors canonical_store.py _get_db_path (683).
func (p *pool) dbPath(tenantID string) string {
	return filepath.Join(p.rootDir, "tenant_"+tenantID+".db")
}

// open returns the pooled entry for tenantID, creating it on first use.
// Schema is initialized and PRAGMAs applied on first open. Subsequent
// calls are O(1) read-locked map lookups.
//
// MaxOpenConns(1) is non-negotiable — it matches the Python pooled-
// single-connection model and avoids INSERT-OR-REPLACE racing under
// concurrent readers (canonical-store.md "Open questions" item 5).
func (p *pool) open(ctx context.Context, tenantID string) (*poolEntry, error) {
	if err := validateTenantID(tenantID); err != nil {
		return nil, err
	}

	// Fast path: existing entry.
	p.mu.RLock()
	if e, ok := p.entries[tenantID]; ok {
		p.mu.RUnlock()
		return e, nil
	}
	p.mu.RUnlock()

	// Slow path: open + initSchema under write lock.
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock.
	if e, ok := p.entries[tenantID]; ok {
		return e, nil
	}

	path := p.dbPath(tenantID)
	driver, dsn, err := p.openParams(ctx, tenantID, path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if p.walMode {
		if _, err := db.ExecContext(ctx, `PRAGMA journal_mode = WAL`); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("store: set journal_mode=WAL: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, `PRAGMA synchronous = NORMAL`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: set synchronous=NORMAL: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA cache_size = -64000`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: set cache_size: %w", err)
	}

	if err := initSchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	e := &poolEntry{db: db}
	p.entries[tenantID] = e
	return e, nil
}

func (p *pool) openParams(ctx context.Context, tenantID, path string) (driver, dsn string, err error) {
	if p.keyManager == nil {
		return "sqlite", fmt.Sprintf("file:%s?_pragma=busy_timeout(%d)", path, p.busyTimeout.Milliseconds()), nil
	}
	encrypted, hasDatabase, err := entcrypto.SQLiteFileEncryptionStatus(path)
	if err != nil {
		return "", "", err
	}
	if hasDatabase && !encrypted {
		return "", "", fmt.Errorf("store: refusing to open unencrypted tenant DB %q with encryption configured", path)
	}
	key, err := p.keyManager.TenantKey(ctx, tenantID)
	if err != nil {
		return "", "", fmt.Errorf("store: tenant key %q: %w", tenantID, err)
	}
	dsn, err = entcrypto.SQLCipherDSN(path, key, p.busyTimeout)
	if err != nil {
		return "", "", err
	}
	return entcrypto.SQLCipherDriverName, dsn, nil
}

// get returns an opened entry for tenantID without creating it. Returns
// ErrTenantNotOpen if no entry exists. Used by hot-path read methods
// that should not silently lazy-create a tenant DB.
func (p *pool) get(tenantID string) (*poolEntry, error) {
	if err := validateTenantID(tenantID); err != nil {
		return nil, err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	e, ok := p.entries[tenantID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTenantNotOpen, tenantID)
	}
	return e, nil
}

// close releases the pooled connection for tenantID, if any. Idempotent.
func (p *pool) close(tenantID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.entries[tenantID]
	if !ok {
		return nil
	}
	delete(p.entries, tenantID)
	return e.db.Close()
}

// closeAll releases every pooled connection. Idempotent. Errors are
// joined into one return value so a single bad close does not mask the
// others.
func (p *pool) closeAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var firstErr error
	for tid, e := range p.entries {
		if err := e.db.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("store: close tenant %q: %w", tid, err)
		}
		delete(p.entries, tid)
	}
	return firstErr
}
