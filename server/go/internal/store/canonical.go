package store

import (
	"context"
	"database/sql"
	"sync"
	"time"

	entcrypto "github.com/elloloop/tenant-shard-db/server/go/internal/crypto"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
)

// Options configures a CanonicalStore at construction time. Zero values
// produce production defaults (WAL on, 5s busy_timeout, time.Now clock).
type Options struct {
	// RootDir is the directory tenant DB files (tenant_<id>.db) live in.
	// Required.
	RootDir string

	// BusyTimeout is the SQLite busy_timeout. 0 -> 5s default.
	BusyTimeout time.Duration

	// WALMode toggles PRAGMA journal_mode=WAL. true in production.
	WALMode bool

	// NowFn returns the current Unix-epoch millisecond. Injectable so
	// tests can drive timestamps deterministically. nil -> time.Now.
	NowFn func() int64

	// Registry is the schema.Registry used for lazy index creation
	// (UniqueFieldIDs / IndexedFieldIDs / SearchableFieldIDs lookups).
	// May be nil; methods that need it will skip index creation when
	// nil, which is what tests want when they exercise raw CRUD paths.
	Registry *schema.Registry

	// KeyManager enables SQLCipher encryption for tenant database files.
	// When set, OpenTenant derives/unwraps the per-tenant DEK and opens
	// tenant_<id>.db through SQLCipher.
	KeyManager *entcrypto.KeyManager

	// EncryptionRequired refuses to construct a store unless KeyManager
	// is configured. It also rejects pre-existing plaintext tenant files.
	EncryptionRequired bool
}

// CanonicalStore is the per-tenant SQLite materialized view of the WAL.
// One *sql.DB per tenant_id, lazy-opened on first use, single-writer
// per tenant (matches the Python parity pattern at canonical_store.py
// :683-717 + :642).
//
// Method receivers are value-stable; the struct is safe for concurrent
// goroutine use. Writers hold the per-tenant write mutex (see
// poolEntry.writeMu); readers do not — SQLite WAL mode handles them.
type CanonicalStore struct {
	pool       *pool
	registry   *schema.Registry
	nowFn      func() int64
	indexCache *indexCache

	// offsetMu guards offsetCond / appliedOffsets. Reads and writes of
	// the offset map both go through this mutex.
	offsetMu        sync.Mutex
	offsetCond      *sync.Cond
	appliedOffsets  map[string]int64
	closed          bool
	closeOnceClosed sync.Once
}

// New constructs a CanonicalStore. Caller must Close. RootDir is created
// (recursively) if it does not exist.
func New(opts Options) (*CanonicalStore, error) {
	p, err := newPool(poolOptions{
		rootDir:            opts.RootDir,
		busyTimeout:        opts.BusyTimeout,
		walMode:            opts.WALMode,
		keyManager:         opts.KeyManager,
		encryptionRequired: opts.EncryptionRequired,
	})
	if err != nil {
		return nil, err
	}
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = func() int64 { return time.Now().UnixMilli() }
	}
	cs := &CanonicalStore{
		pool:           p,
		registry:       opts.Registry,
		nowFn:          nowFn,
		indexCache:     newIndexCache(),
		appliedOffsets: map[string]int64{},
	}
	cs.offsetCond = sync.NewCond(&cs.offsetMu)
	return cs, nil
}

// OpenTenant lazily creates / opens the tenant_<id>.db file, runs the
// schema DDL, and applies the production PRAGMAs. Idempotent: a second
// call for the same tenant_id is a fast O(1) lookup.
func (s *CanonicalStore) OpenTenant(ctx context.Context, tenantID string) error {
	_, err := s.pool.open(ctx, tenantID)
	return err
}

// CloseTenant closes the per-tenant DB handle. Idempotent.
func (s *CanonicalStore) CloseTenant(tenantID string) error {
	return s.pool.close(tenantID)
}

// Close releases all per-tenant DB handles. Idempotent. After Close,
// any in-flight WaitForOffset wakes up (returns ctx.Err() or the
// dedicated wake) so callers don't deadlock at shutdown.
func (s *CanonicalStore) Close() error {
	var err error
	s.closeOnceClosed.Do(func() {
		s.offsetMu.Lock()
		s.closed = true
		s.offsetMu.Unlock()
		s.offsetCond.Broadcast()
		err = s.pool.closeAll()
	})
	return err
}

// db returns the opened *sql.DB for tenantID or an error if the tenant
// has not been opened. Read-side helper used by every Get/Query method.
func (s *CanonicalStore) db(tenantID string) (*sql.DB, error) {
	e, err := s.pool.get(tenantID)
	if err != nil {
		return nil, err
	}
	return e.db, nil
}

// dbAuto returns the opened *sql.DB for tenantID, lazy-opening if
// needed. Used by writers / applier paths that legitimately create the
// tenant on first touch.
func (s *CanonicalStore) dbAuto(ctx context.Context, tenantID string) (*sql.DB, *poolEntry, error) {
	e, err := s.pool.open(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	return e.db, e, nil
}

// now returns the current Unix-epoch millisecond via the injected clock.
func (s *CanonicalStore) now() int64 { return s.nowFn() }

// Registry exposes the configured schema.Registry, or nil if none was
// configured. Used by callers that need to look up field metadata
// outside of the lazy-index hook.
func (s *CanonicalStore) Registry() *schema.Registry { return s.registry }

// AdminDB returns the per-tenant *sql.DB handle for tenantID. The
// tenant must already be open (call OpenTenant first). Intended ONLY
// for the apply package's acl-readers adapter (server/go/internal/apply
// /acladapter.go) — it needs ad-hoc reads against node_access /
// group_users that don't have a dedicated public method on this store.
//
// Production code outside the apply package should use the typed
// helpers (GetNode, QueryNodes, …); reaching for AdminDB elsewhere is
// a code smell.
func (s *CanonicalStore) AdminDB(tenantID string) (*sql.DB, error) {
	return s.db(tenantID)
}

// TenantDBPath returns the absolute tenant SQLite file path. It is used
// by compliance workers and tests that need to inspect on-disk state.
func (s *CanonicalStore) TenantDBPath(tenantID string) (string, error) {
	if err := validateTenantID(tenantID); err != nil {
		return "", err
	}
	return s.pool.dbPath(tenantID), nil
}
