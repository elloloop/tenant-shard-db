// SPDX-License-Identifier: AGPL-3.0-only

package store_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"path/filepath"
	"testing"

	entcrypto "github.com/elloloop/tenant-shard-db/server/go/internal/crypto"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

func randKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, entcrypto.KeyLength)
	if _, err := rand.Read(k); err != nil {
		t.Fatalf("rand key: %v", err)
	}
	return k
}

// newEncryptedStore builds a CanonicalStore backed by a vault-backed
// KeyManager (real SQLCipher tenant DBs).
func newEncryptedStore(t *testing.T) (*store.CanonicalStore, *entcrypto.KeyManager) {
	t.Helper()
	ctx := context.Background()
	master := make([]byte, entcrypto.KeyLength)
	for i := range master {
		master[i] = 0x5a
	}
	vdb, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "vault.db"))
	if err != nil {
		t.Fatalf("open vault db: %v", err)
	}
	t.Cleanup(func() { _ = vdb.Close() })
	vault, err := entcrypto.NewTenantKeyVault(ctx, entcrypto.TenantKeyVaultOptions{DB: vdb, MasterKey: master})
	if err != nil {
		t.Fatalf("vault: %v", err)
	}
	km, err := entcrypto.NewKeyManager(master, vault)
	if err != nil {
		t.Fatalf("key manager: %v", err)
	}
	cs, err := store.New(store.Options{
		RootDir:            t.TempDir(),
		WALMode:            true,
		KeyManager:         km,
		EncryptionRequired: true,
	})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs, km
}

// dbReadableWith reports whether the SQLCipher DB at path opens with key and
// whether node nodeID is present (data-integrity probe).
func dbReadableWith(t *testing.T, path string, key []byte, nodeID string) (opens bool, hasNode bool) {
	t.Helper()
	dsn, err := entcrypto.SQLCipherDSN(path, key, 0)
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	db, err := sql.Open(entcrypto.SQLCipherDriverName, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master`).Scan(&n); err != nil {
		return false, false // wrong key
	}
	var c int
	if err := db.QueryRow(`SELECT count(*) FROM nodes WHERE node_id = ?`, nodeID).Scan(&c); err != nil {
		return true, false
	}
	return true, c == 1
}

// TestRekeyTenantDB_ReEncryptsAndRevokesOldKey is the crux of finding #638:
// after a re-key the database opens with the NEW key (data intact) and no
// longer opens with the OLD key — so an attacker who recomputes the old
// (derived) key can no longer read the data.
func TestRekeyTenantDB_ReEncryptsAndRevokesOldKey(t *testing.T) {
	ctx := context.Background()
	cs, km := newEncryptedStore(t)
	const tenantID = "acme"

	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	// Write a row so we can prove data survives the re-key.
	if _, err := cs.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID:     "n1",
		TypeID:     1,
		OwnerActor: "user:alice",
		Payload:    map[string]any{"1": "secret-value"},
	}); err != nil {
		t.Fatalf("CreateNodeRaw: %v", err)
	}

	oldKey, err := km.TenantKey(ctx, tenantID)
	if err != nil {
		t.Fatalf("TenantKey: %v", err)
	}
	oldKey = append([]byte(nil), oldKey...)
	newKey := randKey(t)

	path, err := cs.TenantDBPath(tenantID)
	if err != nil {
		t.Fatalf("TenantDBPath: %v", err)
	}

	// Precondition: old key opens it (and has the node); new key does not.
	if opens, hasNode := dbReadableWith(t, path, oldKey, "n1"); !opens || !hasNode {
		t.Fatalf("precondition: oldKey opens=%v hasNode=%v, want true,true", opens, hasNode)
	}
	if opens, _ := dbReadableWith(t, path, newKey, "n1"); opens {
		t.Fatal("precondition: newKey already opens the DB before rekey")
	}

	if err := cs.RekeyTenantDB(ctx, tenantID, oldKey, newKey); err != nil {
		t.Fatalf("RekeyTenantDB: %v", err)
	}

	// After: new key opens it AND the row is intact; old key no longer opens.
	if opens, hasNode := dbReadableWith(t, path, newKey, "n1"); !opens || !hasNode {
		t.Fatalf("after rekey: newKey opens=%v hasNode=%v, want true,true (data must survive)", opens, hasNode)
	}
	if opens, _ := dbReadableWith(t, path, oldKey, "n1"); opens {
		t.Fatal("after rekey: the OLD key still opens the DB — re-key did not revoke it (finding #638 not closed)")
	}
}

// TestRekeyTenantDB_Idempotent: re-running with the same (old,new) pair after
// the DB is already on newKey is a safe no-op (models a crash after the file
// re-key but before the vault finalize).
func TestRekeyTenantDB_Idempotent(t *testing.T) {
	ctx := context.Background()
	cs, km := newEncryptedStore(t)
	const tenantID = "acme"
	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	if _, err := cs.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID: "n1", TypeID: 1, OwnerActor: "user:alice", Payload: map[string]any{"1": "v"},
	}); err != nil {
		t.Fatalf("CreateNodeRaw: %v", err)
	}
	oldKey, _ := km.TenantKey(ctx, tenantID)
	oldKey = append([]byte(nil), oldKey...)
	newKey := randKey(t)

	if err := cs.RekeyTenantDB(ctx, tenantID, oldKey, newKey); err != nil {
		t.Fatalf("first rekey: %v", err)
	}
	// Second call: file is already on newKey -> no-op success.
	if err := cs.RekeyTenantDB(ctx, tenantID, oldKey, newKey); err != nil {
		t.Fatalf("idempotent rekey: %v", err)
	}
	path, _ := cs.TenantDBPath(tenantID)
	if opens, hasNode := dbReadableWith(t, path, newKey, "n1"); !opens || !hasNode {
		t.Fatalf("after idempotent rekey: newKey opens=%v hasNode=%v, want true,true", opens, hasNode)
	}
}

// TestRekeyTenantDB_MissingFileIsNoOp: a tenant provisioned in the vault but
// never written has no DB file, so re-key is a no-op success.
func TestRekeyTenantDB_MissingFileIsNoOp(t *testing.T) {
	ctx := context.Background()
	cs, _ := newEncryptedStore(t)
	if err := cs.RekeyTenantDB(ctx, "never-written", randKey(t), randKey(t)); err != nil {
		t.Fatalf("rekey of absent DB should be a no-op, got: %v", err)
	}
}

// TestRekeyTenantDB_WrongOldKeyFails: if neither the new nor the supplied old
// key opens the DB, re-key fails loudly rather than corrupting anything.
func TestRekeyTenantDB_WrongOldKeyFails(t *testing.T) {
	ctx := context.Background()
	cs, _ := newEncryptedStore(t)
	const tenantID = "acme"
	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	if err := cs.RekeyTenantDB(ctx, tenantID, randKey(t), randKey(t)); err == nil {
		t.Fatal("RekeyTenantDB with a wrong current key: expected an error, got nil")
	}
}
