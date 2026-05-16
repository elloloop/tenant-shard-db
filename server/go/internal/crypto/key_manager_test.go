package crypto

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
)

func TestKeyManagerDerivedKeysAreDeterministicAndCopied(t *testing.T) {
	ctx := context.Background()
	master := testMaster(0x11)

	first, err := NewKeyManager(master, nil)
	if err != nil {
		t.Fatalf("NewKeyManager first: %v", err)
	}
	second, err := NewKeyManager(master, nil)
	if err != nil {
		t.Fatalf("NewKeyManager second: %v", err)
	}

	acme1, err := first.TenantKey(ctx, "acme")
	if err != nil {
		t.Fatalf("TenantKey acme first: %v", err)
	}
	acme2, err := second.TenantKey(ctx, "acme")
	if err != nil {
		t.Fatalf("TenantKey acme second: %v", err)
	}
	if !bytes.Equal(acme1, acme2) {
		t.Fatalf("same master+tenant produced different keys")
	}

	globex, err := first.TenantKey(ctx, "globex")
	if err != nil {
		t.Fatalf("TenantKey globex: %v", err)
	}
	if bytes.Equal(acme1, globex) {
		t.Fatalf("different tenants produced the same key")
	}

	acme1[0] ^= 0xff
	acme3, err := first.TenantKey(ctx, "acme")
	if err != nil {
		t.Fatalf("TenantKey acme cached: %v", err)
	}
	if bytes.Equal(acme1, acme3) {
		t.Fatalf("caller mutation leaked into the key cache")
	}
	if got := first.CachedTenantIDs(); len(got) != 2 || got[0] != "acme" || got[1] != "globex" {
		t.Fatalf("CachedTenantIDs = %v, want [acme globex]", got)
	}
}

func TestParseMasterKeyHex(t *testing.T) {
	master := testMaster(0x22)
	encoded := hex.EncodeToString(master)

	got, err := ParseMasterKeyHex(encoded)
	if err != nil {
		t.Fatalf("ParseMasterKeyHex valid: %v", err)
	}
	if !bytes.Equal(got, master) {
		t.Fatalf("ParseMasterKeyHex returned wrong key")
	}

	if _, err := ParseMasterKeyHex("not-hex"); err == nil {
		t.Fatalf("ParseMasterKeyHex accepted invalid hex")
	}
	short := hex.EncodeToString(master[:KeyLength-1])
	if _, err := ParseMasterKeyHex(short); !errors.Is(err, ErrInvalidMasterKey) {
		t.Fatalf("ParseMasterKeyHex short: got %v, want ErrInvalidMasterKey", err)
	}
}

func TestKeyManagerVaultSeedsAndPersistsKeysInGlobalDB(t *testing.T) {
	ctx := context.Background()
	master := testMaster(0x33)
	gs := testGlobalStore(t)

	vault, err := NewTenantKeyVault(ctx, TenantKeyVaultOptions{DB: gs.DB(), MasterKey: master})
	if err != nil {
		t.Fatalf("NewTenantKeyVault first: %v", err)
	}
	manager, err := NewKeyManager(master, vault)
	if err != nil {
		t.Fatalf("NewKeyManager first: %v", err)
	}
	seeded, err := manager.TenantKey(ctx, "acme")
	if err != nil {
		t.Fatalf("TenantKey seeded: %v", err)
	}

	vault2, err := NewTenantKeyVault(ctx, TenantKeyVaultOptions{DB: gs.DB(), MasterKey: master})
	if err != nil {
		t.Fatalf("NewTenantKeyVault second: %v", err)
	}
	manager2, err := NewKeyManager(master, vault2)
	if err != nil {
		t.Fatalf("NewKeyManager second: %v", err)
	}
	reopened, err := manager2.TenantKey(ctx, "acme")
	if err != nil {
		t.Fatalf("TenantKey reopened: %v", err)
	}
	if !bytes.Equal(seeded, reopened) {
		t.Fatalf("vault did not persist the tenant key")
	}

	row, err := vault2.GetRow(ctx, "acme")
	if err != nil {
		t.Fatalf("GetRow: %v", err)
	}
	if row == nil || row.TenantID != "acme" || len(row.WrappedKey) <= nonceLength || row.ShreddedAtMS.Valid {
		t.Fatalf("unexpected vault row: %+v", row)
	}
	if bytes.Contains(row.WrappedKey, seeded) {
		t.Fatalf("wrapped vault blob contains the plaintext DEK")
	}
}

func TestKeyManagerVaultShredIsDurable(t *testing.T) {
	ctx := context.Background()
	master := testMaster(0x44)
	gs := testGlobalStore(t)
	now := fixedTime(1710000000000)

	vault, err := NewTenantKeyVault(ctx, TenantKeyVaultOptions{
		DB:        gs.DB(),
		MasterKey: master,
		NowFn:     now,
	})
	if err != nil {
		t.Fatalf("NewTenantKeyVault first: %v", err)
	}
	manager, err := NewKeyManager(master, vault)
	if err != nil {
		t.Fatalf("NewKeyManager first: %v", err)
	}
	if _, err := manager.TenantKey(ctx, "acme"); err != nil {
		t.Fatalf("TenantKey before shred: %v", err)
	}
	if err := manager.ShredTenant(ctx, "acme"); err != nil {
		t.Fatalf("ShredTenant: %v", err)
	}
	if _, err := manager.TenantKey(ctx, "acme"); !errors.Is(err, ErrTenantShredded) {
		t.Fatalf("TenantKey after shred: got %v, want ErrTenantShredded", err)
	}

	vault2, err := NewTenantKeyVault(ctx, TenantKeyVaultOptions{DB: gs.DB(), MasterKey: master})
	if err != nil {
		t.Fatalf("NewTenantKeyVault second: %v", err)
	}
	manager2, err := NewKeyManager(master, vault2)
	if err != nil {
		t.Fatalf("NewKeyManager second: %v", err)
	}
	shredded, err := manager2.IsShredded(ctx, "acme")
	if err != nil {
		t.Fatalf("IsShredded reopened: %v", err)
	}
	if !shredded {
		t.Fatalf("reopened manager did not see durable shred tombstone")
	}
	if _, err := manager2.TenantKey(ctx, "acme"); !errors.Is(err, ErrTenantShredded) {
		t.Fatalf("TenantKey reopened after shred: got %v, want ErrTenantShredded", err)
	}

	row, err := vault2.GetRow(ctx, "acme")
	if err != nil {
		t.Fatalf("GetRow shredded: %v", err)
	}
	if row == nil || row.WrappedKey != nil || !row.ShreddedAtMS.Valid || row.ShreddedAtMS.Int64 != 1710000000000 {
		t.Fatalf("unexpected shredded row: %+v", row)
	}
}

func TestTenantKeyVaultProvisionGetAndRewrap(t *testing.T) {
	ctx := context.Background()
	oldMaster := testMaster(0x55)
	newMaster := testMaster(0x66)
	dek := testMaster(0x77)
	gs := testGlobalStore(t)

	vault, err := NewTenantKeyVault(ctx, TenantKeyVaultOptions{DB: gs.DB(), MasterKey: oldMaster})
	if err != nil {
		t.Fatalf("NewTenantKeyVault old: %v", err)
	}
	if err := vault.Provision(ctx, "acme", dek); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if err := vault.Provision(ctx, "acme", dek); !errors.Is(err, ErrTenantKeyAlreadyProvisioned) {
		t.Fatalf("duplicate Provision: got %v, want ErrTenantKeyAlreadyProvisioned", err)
	}
	got, err := vault.Get(ctx, "acme")
	if err != nil {
		t.Fatalf("Get old: %v", err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatalf("Get old returned wrong DEK")
	}

	count, err := vault.RewrapWithNewMaster(ctx, newMaster)
	if err != nil {
		t.Fatalf("RewrapWithNewMaster: %v", err)
	}
	if count != 1 {
		t.Fatalf("RewrapWithNewMaster count = %d, want 1", count)
	}
	got, err = vault.Get(ctx, "acme")
	if err != nil {
		t.Fatalf("Get after rewrap: %v", err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatalf("Get after rewrap returned wrong DEK")
	}

	reopenedNew, err := NewTenantKeyVault(ctx, TenantKeyVaultOptions{DB: gs.DB(), MasterKey: newMaster})
	if err != nil {
		t.Fatalf("NewTenantKeyVault new: %v", err)
	}
	got, err = reopenedNew.Get(ctx, "acme")
	if err != nil {
		t.Fatalf("Get reopened new: %v", err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatalf("Get reopened new returned wrong DEK")
	}

	reopenedOld, err := NewTenantKeyVault(ctx, TenantKeyVaultOptions{DB: gs.DB(), MasterKey: oldMaster})
	if err != nil {
		t.Fatalf("NewTenantKeyVault old reopened: %v", err)
	}
	if _, err := reopenedOld.Get(ctx, "acme"); !errors.Is(err, ErrVaultAuthentication) {
		t.Fatalf("Get reopened old: got %v, want ErrVaultAuthentication", err)
	}
}

func TestTenantKeyVaultBindsWrappedKeyToTenantID(t *testing.T) {
	ctx := context.Background()
	master := testMaster(0x78)
	dek := testMaster(0x79)
	gs := testGlobalStore(t)

	vault, err := NewTenantKeyVault(ctx, TenantKeyVaultOptions{DB: gs.DB(), MasterKey: master})
	if err != nil {
		t.Fatalf("NewTenantKeyVault: %v", err)
	}
	if err := vault.Provision(ctx, "acme", dek); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if _, err := gs.DB().ExecContext(ctx,
		`UPDATE tenant_key_vault SET tenant_id = ? WHERE tenant_id = ?`,
		"globex", "acme",
	); err != nil {
		t.Fatalf("rename tenant key row: %v", err)
	}
	if _, err := vault.Get(ctx, "globex"); !errors.Is(err, ErrVaultAuthentication) {
		t.Fatalf("Get renamed row: got %v, want ErrVaultAuthentication", err)
	}
}

func TestTenantKeyVaultRejectsShreddedProvision(t *testing.T) {
	ctx := context.Background()
	master := testMaster(0x88)
	dek := testMaster(0x99)
	gs := testGlobalStore(t)

	vault, err := NewTenantKeyVault(ctx, TenantKeyVaultOptions{DB: gs.DB(), MasterKey: master})
	if err != nil {
		t.Fatalf("NewTenantKeyVault: %v", err)
	}
	if err := vault.Provision(ctx, "acme", dek); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if err := vault.Shred(ctx, "acme"); err != nil {
		t.Fatalf("Shred: %v", err)
	}
	if err := vault.Provision(ctx, "acme", dek); !errors.Is(err, ErrTenantShredded) {
		t.Fatalf("Provision after shred: got %v, want ErrTenantShredded", err)
	}
}

func TestTenantKeyVaultRejectsEmptyTenantID(t *testing.T) {
	ctx := context.Background()
	master := testMaster(0xaa)
	dek := testMaster(0xbb)
	gs := testGlobalStore(t)

	vault, err := NewTenantKeyVault(ctx, TenantKeyVaultOptions{DB: gs.DB(), MasterKey: master})
	if err != nil {
		t.Fatalf("NewTenantKeyVault: %v", err)
	}
	for name, call := range map[string]func() error{
		"Get": func() error {
			_, err := vault.Get(ctx, "")
			return err
		},
		"GetRow": func() error {
			_, err := vault.GetRow(ctx, "")
			return err
		},
		"Provision": func() error {
			return vault.Provision(ctx, "", dek)
		},
		"Shred": func() error {
			return vault.Shred(ctx, "")
		},
	} {
		if err := call(); err == nil {
			t.Fatalf("%s accepted empty tenant_id", name)
		}
	}
}

func testGlobalStore(t *testing.T) *globalstore.GlobalStore {
	t.Helper()
	gs, err := globalstore.New(globalstore.Options{
		DataDir: t.TempDir(),
		WALMode: true,
	})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })
	return gs
}

func testMaster(fill byte) []byte {
	key := make([]byte, KeyLength)
	for i := range key {
		key[i] = fill
	}
	return key
}

func fixedTime(ms int64) func() time.Time {
	return func() time.Time {
		return time.UnixMilli(ms)
	}
}
