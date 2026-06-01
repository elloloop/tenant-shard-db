// SPDX-License-Identifier: AGPL-3.0-only

package store

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"

	entcrypto "github.com/elloloop/tenant-shard-db/server/go/internal/crypto"
)

// RekeyTenantDB re-encrypts tenantID's SQLite database so it ends encrypted
// under newKey. It implements crypto.RekeyDBFunc for the #638 derived-key
// migration and is idempotent + crash-safe:
//
//   - if the DB is already readable with newKey it is a no-op (a previous run
//     re-keyed the file but may have crashed before the vault was finalized);
//   - otherwise it opens with oldKey and runs SQLCipher PRAGMA rekey;
//   - a missing/empty database file is a no-op (a tenant provisioned in the
//     vault but never written — the file will be created under newKey on the
//     first write).
//
// It first drops any pooled handle for the tenant so the re-key runs on a
// fresh, exclusive connection. Run this as an offline migration step with the
// server stopped: the store must otherwise be idle for the tenant while its
// database file is rewritten.
func (s *CanonicalStore) RekeyTenantDB(ctx context.Context, tenantID string, oldKey, newKey []byte) error {
	if s.pool.keyManager == nil {
		return fmt.Errorf("store: RekeyTenantDB requires encryption-at-rest (no key manager configured)")
	}
	if len(oldKey) != entcrypto.KeyLength || len(newKey) != entcrypto.KeyLength {
		return fmt.Errorf("store: RekeyTenantDB keys must be %d bytes", entcrypto.KeyLength)
	}
	if err := s.CloseTenant(tenantID); err != nil {
		return fmt.Errorf("store: close tenant %q before rekey: %w", tenantID, err)
	}
	path := s.pool.dbPath(tenantID)
	_, hasDatabase, err := entcrypto.SQLiteFileEncryptionStatus(path)
	if err != nil {
		return err
	}
	if !hasDatabase {
		// Nothing on disk yet; the first write will create the file under the
		// new key, so there is nothing to re-encrypt.
		return nil
	}
	// Idempotency: if newKey already opens the DB, a previous run re-keyed it
	// (and only the vault finalize was lost) — treat as done.
	if ok, err := s.dbOpensWith(ctx, path, newKey); err != nil {
		return err
	} else if ok {
		return nil
	}
	return s.rekeyFile(ctx, path, oldKey, newKey)
}

// dbOpensWith reports whether the SQLCipher database at path is readable with
// key (i.e. key is its current encryption key). A read is forced so SQLCipher
// actually applies and verifies the key.
func (s *CanonicalStore) dbOpensWith(ctx context.Context, path string, key []byte) (bool, error) {
	dsn, err := entcrypto.SQLCipherDSN(path, key, s.pool.busyTimeout)
	if err != nil {
		return false, err
	}
	db, err := sql.Open(entcrypto.SQLCipherDriverName, dsn)
	if err != nil {
		return false, fmt.Errorf("store: open %q for probe: %w", path, err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master`).Scan(&n); err != nil {
		// Wrong key (or otherwise unreadable) — not openable with this key.
		return false, nil
	}
	return true, nil
}

// rekeyFile opens path with oldKey and runs SQLCipher PRAGMA rekey to newKey
// on a single dedicated connection.
func (s *CanonicalStore) rekeyFile(ctx context.Context, path string, oldKey, newKey []byte) error {
	dsn, err := entcrypto.SQLCipherDSN(path, oldKey, s.pool.busyTimeout)
	if err != nil {
		return err
	}
	db, err := sql.Open(entcrypto.SQLCipherDriverName, dsn)
	if err != nil {
		return fmt.Errorf("store: open %q for rekey: %w", path, err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("store: rekey conn %q: %w", path, err)
	}
	defer func() { _ = conn.Close() }()

	// Confirm oldKey actually opens the DB before mutating it.
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master`).Scan(&n); err != nil {
		return fmt.Errorf("store: rekey %q: database not readable with the supplied current key: %w", path, err)
	}
	// Flush WAL frames into the main database so the rekey covers every page.
	if _, err := conn.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return fmt.Errorf("store: rekey %q: checkpoint: %w", path, err)
	}
	// SQLCipher PRAGMA rekey re-encrypts the whole database in place; it is
	// atomic (journalled), so a crash leaves the file fully on the old or fully
	// on the new key, never partially — which is what makes the migration
	// safe to resume.
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`PRAGMA rekey = "x'%s'"`, hex.EncodeToString(newKey))); err != nil {
		return fmt.Errorf("store: rekey %q: %w", path, err)
	}
	return nil
}
