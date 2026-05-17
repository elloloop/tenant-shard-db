package main

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
)

// TestGlobalStoreAPIKeysAdapterEndToEnd exercises the production wiring:
// the globalStoreAPIKeys adapter over a real global.db, driven by the
// PersistentAPIKeyManager. This is the path --api-key-auth installs, so
// it covers issue -> argon2 persist -> validate -> rotation overlap ->
// revoke against the actual SQLite schema.
func TestGlobalStoreAPIKeysAdapterEndToEnd(t *testing.T) {
	dir := t.TempDir()
	gs, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })

	mgr := auth.NewPersistentAPIKeyManager(globalStoreAPIKeys{gs})
	ctx := context.Background()

	oldID, err := mgr.Issue(ctx, "old-secret", "deploy-bot", []string{"read", "write"}, 0)
	if err != nil {
		t.Fatalf("Issue old: %v", err)
	}

	// Stored row must carry an argon2id hash, never the plaintext.
	row, err := gs.GetAPIKey(ctx, oldID)
	if err != nil || row == nil {
		t.Fatalf("GetAPIKey(%s): row=%v err=%v", oldID, row, err)
	}
	if row.Hash == "old-secret" || len(row.Hash) < 20 || row.Hash[:9] != "$argon2id" {
		t.Fatalf("persisted hash not argon2id: %q", row.Hash)
	}

	info, err := mgr.Validate(ctx, "old-secret")
	if err != nil {
		t.Fatalf("Validate old: %v", err)
	}
	if info.Name != "deploy-bot" || len(info.Scopes) != 2 {
		t.Fatalf("validated info = %+v, want deploy-bot/[read write]", info)
	}

	// Rotation: issue the replacement, both keys validate during overlap.
	if _, err := mgr.Issue(ctx, "new-secret", "deploy-bot", []string{"read", "write"}, 0); err != nil {
		t.Fatalf("Issue new: %v", err)
	}
	if _, err := mgr.Validate(ctx, "old-secret"); err != nil {
		t.Fatalf("old key should validate during overlap: %v", err)
	}
	if _, err := mgr.Validate(ctx, "new-secret"); err != nil {
		t.Fatalf("new key should validate during overlap: %v", err)
	}

	// Cutover: revoke the old key; only the new key keeps working.
	flipped, err := mgr.Revoke(ctx, oldID)
	if err != nil || !flipped {
		t.Fatalf("Revoke old: flipped=%v err=%v", flipped, err)
	}
	if _, err := mgr.Validate(ctx, "old-secret"); err == nil {
		t.Fatal("revoked old key must fail")
	}
	if _, err := mgr.Validate(ctx, "new-secret"); err != nil {
		t.Fatalf("new key must survive old-key revocation: %v", err)
	}
}
