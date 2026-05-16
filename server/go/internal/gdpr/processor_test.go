package gdpr_test

import (
	"context"
	"errors"
	"testing"

	entcrypto "github.com/elloloop/tenant-shard-db/server/go/internal/crypto"
	"github.com/elloloop/tenant-shard-db/server/go/internal/gdpr"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

func TestProcessorCryptoShredsPersonalTenantAfterGraceExpiry(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	master := gdprTestMaster(0x51)
	gs, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true, NowFn: func() int64 { return 1000 }})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })
	vault, err := entcrypto.NewTenantKeyVault(ctx, entcrypto.TenantKeyVaultOptions{DB: gs.DB(), MasterKey: master})
	if err != nil {
		t.Fatalf("NewTenantKeyVault: %v", err)
	}
	keys, err := entcrypto.NewKeyManager(master, vault)
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	cs, err := store.New(store.Options{
		RootDir:            dir,
		WALMode:            true,
		KeyManager:         keys,
		EncryptionRequired: true,
	})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := gs.CreateTenantWithOwner(ctx, "personal", "Alice Personal", "", "alice"); err != nil {
		t.Fatalf("CreateTenantWithOwner: %v", err)
	}
	if _, err := cs.CreateNodeRaw(ctx, "personal", store.NodeInput{
		NodeID:     "secret-node",
		TypeID:     7,
		OwnerActor: "user:alice",
		Payload:    map[string]any{"1": "private"},
	}); err != nil {
		t.Fatalf("CreateNodeRaw: %v", err)
	}
	if _, err := gs.QueueDeletion(ctx, "alice", 0); err != nil {
		t.Fatalf("QueueDeletion: %v", err)
	}

	processor, err := gdpr.New(gdpr.Options{
		Global: gs,
		Store:  cs,
		Keys:   keys,
		NowFn:  func() int64 { return 1000 },
	})
	if err != nil {
		t.Fatalf("gdpr.New: %v", err)
	}
	summary, err := processor.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if summary.EntriesCompleted != 1 || summary.PersonalTenants != 1 || summary.TenantKeysShredded != 1 {
		t.Fatalf("summary = %+v, want one completed personal shred", summary)
	}
	if _, err := keys.TenantKey(ctx, "personal"); !errors.Is(err, entcrypto.ErrTenantShredded) {
		t.Fatalf("TenantKey after deletion = %v, want ErrTenantShredded", err)
	}
	if err := cs.OpenTenant(ctx, "personal"); !errors.Is(err, entcrypto.ErrTenantShredded) {
		t.Fatalf("OpenTenant after deletion = %v, want ErrTenantShredded", err)
	}
	entry, err := gs.GetDeletionEntry(ctx, "alice")
	if err != nil {
		t.Fatalf("GetDeletionEntry: %v", err)
	}
	if entry == nil || entry.Status != "completed" {
		t.Fatalf("deletion entry = %+v, want completed", entry)
	}
	user, err := gs.GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user == nil || user.Status != "deleted" {
		t.Fatalf("user = %+v, want deleted", user)
	}
	tenant, err := gs.GetTenant(ctx, "personal")
	if err != nil {
		t.Fatalf("GetTenant: %v", err)
	}
	if tenant == nil || tenant.Status != "deleted" {
		t.Fatalf("tenant = %+v, want deleted", tenant)
	}
	members, err := gs.GetUserTenants(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUserTenants: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("memberships after deletion = %+v, want none", members)
	}
}

func gdprTestMaster(fill byte) []byte {
	key := make([]byte, entcrypto.KeyLength)
	for i := range key {
		key[i] = fill
	}
	return key
}
