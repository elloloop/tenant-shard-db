// Package globalstore implements `global.db`, the cross-tenant SQLite
// store that holds user_registry, tenant_registry, tenant_members,
// shared_index, deletion_queue, legal_holds, tenant_quotas, and
// tenant_usage. It is the only place CLAUDE.md invariant #4 permits
// cross-tenant state to be read or written.
//
// Spec: docs/go-port/shared/global-store.md. Source-of-truth Python:
//
// # Carve-out from invariant #1 ("all writes go through the WAL")
//
// `globalstore` has no WAL. The Python implementation also reflects
// this — `global.db` is the durable record for cross-tenant state.
// Don't add WAL coupling here; the carve-out is documented in the spec
// and in PLAN.md §6.
//
// # Concurrency
//
// MaxOpenConns(1) is non-negotiable: it mirrors the Python
// single-thread executor (one writer, no contention) and avoids
// SQLITE_BUSY retry loops in hot paths. SetMaxIdleConns matches so the
// pool never closes the lone connection between calls.
//
// # Driver
//
// The default driver is modernc.org/sqlite. When Options.EncryptionKey
// is configured, global.db is opened through SQLCipher instead.

package globalstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	entcrypto "github.com/elloloop/tenant-shard-db/server/go/internal/crypto"

	// modernc.org/sqlite registers the "sqlite" driver with database/sql
	// in its init(); a side-effect import is the canonical way to use it.
	_ "modernc.org/sqlite"
)

// Options is the config bag for New. Zero values produce production
// defaults: WAL journal, NORMAL synchronous, 5s busy-timeout, time.Now
// for the clock.
type Options struct {
	// DataDir is the directory the global.db file lives in. Created
	// with 0o755 if it does not exist.
	DataDir string

	// BusyTimeout is the SQLite busy_timeout. 0 -> 5s default.
	BusyTimeout time.Duration

	// WALMode toggles `PRAGMA journal_mode = WAL`. true is the
	// production default; tests can disable it.
	WALMode bool

	// NowFn returns the current Unix-epoch second. Injectable so tests
	// can drive calendar-month rollover deterministically. nil ->
	// time.Now().Unix().
	NowFn func() int64

	// EncryptionKey enables SQLCipher encryption for global.db. It must
	// be exactly 32 bytes when present.
	EncryptionKey []byte

	// EncryptionRequired refuses to open global.db without EncryptionKey
	// and rejects pre-existing plaintext global.db files.
	EncryptionRequired bool
}

// GlobalStore is the cross-tenant SQLite handle.
//
// All methods are safe for concurrent goroutine use; serialization is
// provided by MaxOpenConns(1) on the underlying *sql.DB.
type GlobalStore struct {
	db     *sql.DB
	path   string
	nowFn  func() int64
	closed bool
}

// New opens (or creates) global.db inside opts.DataDir, runs the schema
// migrations, and returns a ready-to-use GlobalStore. Caller must Close.
//
// MaxOpenConns(1) is set unconditionally — see package doc.
func New(opts Options) (*GlobalStore, error) {
	if opts.DataDir == "" {
		return nil, errors.New("globalstore: Options.DataDir is required")
	}
	if opts.EncryptionRequired && len(opts.EncryptionKey) == 0 {
		return nil, errors.New("globalstore: encryption required but no encryption key configured")
	}
	if err := os.MkdirAll(opts.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("globalstore: create data dir %q: %w", opts.DataDir, err)
	}
	busy := opts.BusyTimeout
	if busy <= 0 {
		busy = 5 * time.Second
	}
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = func() int64 { return time.Now().Unix() }
	}
	path := filepath.Join(opts.DataDir, "global.db")

	driver, dsn, err := globalOpenParams(path, busy, opts.EncryptionKey)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("globalstore: open %q: %w", path, err)
	}

	// MaxOpenConns(1) is the parity-with-Python invariant. SetMaxIdleConns
	// matches so the connection survives between calls; ConnMaxLifetime=0
	// keeps it alive for the process lifetime.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if opts.WALMode {
		if _, err := db.ExecContext(ctx, `PRAGMA journal_mode = WAL`); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("globalstore: set journal_mode=WAL: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, `PRAGMA synchronous = NORMAL`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("globalstore: set synchronous=NORMAL: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("globalstore: set foreign_keys=ON: %w", err)
	}

	if err := initSchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &GlobalStore{
		db:    db,
		path:  path,
		nowFn: nowFn,
	}, nil
}

func globalOpenParams(path string, busy time.Duration, encryptionKey []byte) (driver, dsn string, err error) {
	if len(encryptionKey) == 0 {
		return "sqlite", fmt.Sprintf("file:%s?_pragma=busy_timeout(%d)", path, busy.Milliseconds()), nil
	}
	encrypted, hasDatabase, err := entcrypto.SQLiteFileEncryptionStatus(path)
	if err != nil {
		return "", "", err
	}
	if hasDatabase && !encrypted {
		return "", "", fmt.Errorf("globalstore: refusing to open unencrypted global.db %q with encryption configured", path)
	}
	dsn, err = entcrypto.SQLCipherDSN(path, encryptionKey, busy)
	if err != nil {
		return "", "", err
	}
	return entcrypto.SQLCipherDriverName, dsn, nil
}

// Close releases the SQLite connection. Idempotent.
func (g *GlobalStore) Close() error {
	if g == nil || g.closed {
		return nil
	}
	g.closed = true
	return g.db.Close()
}

// DB exposes the underlying *sql.DB for tests that need to assert
// connection-pool stats (e.g. MaxOpenConnections == 1). Production code
// should not reach into this.
func (g *GlobalStore) DB() *sql.DB { return g.db }

// Path returns the absolute path of the global.db file.
func (g *GlobalStore) Path() string { return g.path }

// now returns the current Unix-epoch second via the injected clock.
func (g *GlobalStore) now() int64 { return g.nowFn() }
