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

// poolEntry is one tenant's pooled handle.
//
// Two distinct *sql.DB handles point at the same physical tenant file:
//
//   - db      — the single-connection WRITE/DDL handle. SetMaxOpenConns(1)
//     plus the per-tenant writeMu serialize every writer onto one
//     connection (ADR-016: only the applier writes SQLite, and it does
//     so under BEGIN IMMEDIATE on this handle). DDL (lazy index/FTS
//     creation) and AdminDB ad-hoc access also use this handle so they
//     observe the writer's exact view.
//   - readDB  — a read-only POOLED handle (mode=ro + query_only,
//     SetMaxOpenConns(N)). Pure query methods (GetNode, QueryNodes,
//     …) use this so same-tenant reads run concurrently against
//     SQLite WAL snapshots instead of serializing behind the single
//     write connection. readDB is physically incapable of writing
//     (driver opens it SQLITE_OPEN_READONLY), so it cannot violate
//     the single-writer invariant even by mistake.
//
// readDB MAY be nil when WAL mode is disabled (see pool.walMode): a
// read-only connection cannot create the -wal/-shm sidecars, and the
// rollback-journal mode does not allow a reader concurrent with a
// writer anyway, so the split would give nothing and risk SQLITE_BUSY.
// In that case readDBOrWrite() falls back to the write handle and
// behaviour is exactly as before this change.
//
// ADR-026: the split is OFF BY DEFAULT (--read-pool-size=1, the single
// shared connection) — landed dark pending idle-tenant eviction
// (canonical-store OQ-2). It is opt-IN: set --read-pool-size>1 to
// enable. Correctness does not depend on the default: ADR-026
// condition 1 moved the WaitForOffset offset broadcast to AFTER
// BatchTxn.Commit (see offset.go / txn.go) unconditionally, so a
// read-after-write fenced by WaitForOffset is only woken once the
// write is committed and visible on every connection, including this
// read pool when enabled. That latent regression is closed
// regardless of --read-pool-size, not merely "preserved".
type poolEntry struct {
	db *sql.DB
	// readDB is the read-only pooled handle. nil => use db for reads
	// (non-WAL mode). Never written through.
	readDB *sql.DB
	// writeMu serializes writers on this tenant's DB. Readers do NOT
	// take it; SQLite WAL mode + per-connection isolation handles them.
	// This matches canonical_store.py:_get_tenant_lock (642) which only
	// guards the single-pooled-writer path in Python.
	writeMu sync.Mutex
}

// reader returns the handle pure-read methods should use: the pooled
// read-only handle when present, else the single write handle. Callers
// MUST only issue SELECTs through the returned handle.
func (e *poolEntry) reader() *sql.DB {
	if e.readDB != nil {
		return e.readDB
	}
	return e.db
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

	// readPoolSize is SetMaxOpenConns(N) for the per-tenant read-only
	// handle. <=1 disables the split (reads keep using the single write
	// connection — identical to pre-#137 behaviour).
	readPoolSize int

	// keyManager switches tenant DB opens to SQLCipher.
	keyManager *entcrypto.KeyManager

	// encryptionRequired rejects plaintext files and missing key config.
	encryptionRequired bool
}

type poolOptions struct {
	rootDir            string
	busyTimeout        time.Duration
	walMode            bool
	readPoolSize       int
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
		readPoolSize:       opts.readPoolSize,
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
// The WRITE handle keeps SetMaxOpenConns(1): exactly one connection,
// serialized further by poolEntry.writeMu. This preserves ADR-016
// (only the applier writes SQLite, under BEGIN IMMEDIATE) and the
// single-writer invariant verbatim — nothing about the write path
// changes.
//
// When WAL mode is on and readPoolSize > 1, open() ALSO opens a
// separate read-only pooled handle (mode=ro + PRAGMA query_only) with
// SetMaxOpenConns(readPoolSize). Pure query methods use it so 50
// same-tenant GetNode calls run concurrently against WAL read
// snapshots instead of queueing on the single write connection
// (issue #137 / canonical-store.md "Open questions" item 5 — resolved
// here by physically opening the read handle read-only, so concurrent
// readers cannot race INSERT-OR-REPLACE: they cannot write at all).
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

	// Read-only pooled handle (issue #137). Only meaningful in WAL mode:
	// a read-only connection cannot create the -wal/-shm sidecars, and
	// rollback-journal mode blocks a reader concurrent with a writer, so
	// the split would only add SQLITE_BUSY risk for zero gain. The write
	// handle above has, by this point, created the file and switched it
	// to WAL, so the read-only handle attaches cleanly.
	var readDB *sql.DB
	if p.walMode && p.readPoolSize > 1 {
		rDriver, rDSN, rErr := p.readOpenParams(ctx, tenantID, path)
		if rErr != nil {
			_ = db.Close()
			return nil, rErr
		}
		readDB, rErr = sql.Open(rDriver, rDSN)
		if rErr != nil {
			_ = db.Close()
			return nil, fmt.Errorf("store: open read handle %q: %w", path, rErr)
		}
		readDB.SetMaxOpenConns(p.readPoolSize)
		readDB.SetMaxIdleConns(p.readPoolSize)
		readDB.SetConnMaxLifetime(0)
		// query_only is belt-and-braces over mode=ro: any stray write
		// (there should be none — only SELECTs reach this handle)
		// fails fast instead of silently mutating tenant state.
		if _, rErr = readDB.ExecContext(ctx, `PRAGMA query_only = ON`); rErr != nil {
			_ = readDB.Close()
			_ = db.Close()
			return nil, fmt.Errorf("store: set query_only on read handle: %w", rErr)
		}
		// Bound the WAL the readers pin: NORMAL synchronous is
		// irrelevant for read-only, but cache_size keeps hot pages
		// resident per read connection.
		if _, rErr = readDB.ExecContext(ctx, `PRAGMA cache_size = -64000`); rErr != nil {
			_ = readDB.Close()
			_ = db.Close()
			return nil, fmt.Errorf("store: set cache_size on read handle: %w", rErr)
		}
	}

	e := &poolEntry{db: db, readDB: readDB}
	p.entries[tenantID] = e
	return e, nil
}

// readOpenParams returns the driver + DSN for the read-only pooled
// handle. It mirrors openParams but forces SQLITE_OPEN_READONLY via the
// standard SQLite URI "mode=ro" parameter (honoured by both
// modernc.org/sqlite and the SQLCipher driver, which open with
// SQLITE_OPEN_URI). The encryption key is still required for SQLCipher
// — read-only does not mean unencrypted.
func (p *pool) readOpenParams(ctx context.Context, tenantID, path string) (driver, dsn string, err error) {
	if p.keyManager == nil {
		return "sqlite", fmt.Sprintf(
			"file:%s?mode=ro&_pragma=busy_timeout(%d)&_pragma=query_only(true)",
			path, p.busyTimeout.Milliseconds(),
		), nil
	}
	key, err := p.keyManager.TenantKey(ctx, tenantID)
	if err != nil {
		return "", "", fmt.Errorf("store: tenant key %q: %w", tenantID, err)
	}
	base, err := entcrypto.SQLCipherDSN(path, key, p.busyTimeout)
	if err != nil {
		return "", "", err
	}
	// SQLCipherDSN returns "file:<path>?<params>"; append read-only +
	// query_only. mutecomm/go-sqlcipher opens with SQLITE_OPEN_URI so
	// mode=ro is honoured at the VFS layer, and it also parses the
	// _query_only DSN parameter (sqlite3.go:_query_only).
	return entcrypto.SQLCipherDriverName, base + "&mode=ro&_query_only=true", nil
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

// closeHandles closes the read handle (if any) then the write handle,
// returning the first error. Read handle first so no reader observes a
// torn-down write handle (they are independent connections, but order
// keeps shutdown deterministic).
func (e *poolEntry) closeHandles() error {
	var firstErr error
	if e.readDB != nil {
		if err := e.readDB.Close(); err != nil {
			firstErr = err
		}
	}
	if err := e.db.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// close releases the pooled connections for tenantID, if any. Idempotent.
func (p *pool) close(tenantID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.entries[tenantID]
	if !ok {
		return nil
	}
	delete(p.entries, tenantID)
	return e.closeHandles()
}

// closeAll releases every pooled connection. Idempotent. Errors are
// joined into one return value so a single bad close does not mask the
// others.
func (p *pool) closeAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var firstErr error
	for tid, e := range p.entries {
		if err := e.closeHandles(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("store: close tenant %q: %w", tid, err)
		}
		delete(p.entries, tid)
	}
	return firstErr
}
