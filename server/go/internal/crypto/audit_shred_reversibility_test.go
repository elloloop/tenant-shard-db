package crypto

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"testing"

	"golang.org/x/crypto/hkdf"
)

// FINDING #638 (CRITICAL/security): Crypto-shred is reversible — FIXED.
//
//   file: server/go/internal/crypto/key_manager.go (fetchOrProvisionVault)
//         server/go/internal/crypto/tenant_key_vault.go (Provision, rekey)
//
// Before the fix, the per-tenant SQLCipher DEK was
// HKDF-SHA256(masterKey, tenantID) with no salt/random, so a "shredded"
// tenant's key was re-derivable offline from masterKey+tenantID and the at-
// rest data was never actually destroyed. The fix provisions a random 256-bit
// DEK and stores only the master-wrapped copy, so nulling that copy on shred
// is irreversible. Legacy derived-key tenants are migrated by
// MigrateDerivedTenants (see rekey_migration_test.go).
//
// The test below — previously a skipped regression gate — is now active and
// asserts the secure behaviour. The earlier "_DemonstratesFinding1" test that
// pinned the reversible behaviour was removed when the fix landed.

// TestCryptoShred_SecureBehavior_Finding1 asserts the fixed behaviour: a
// tenant DEK is backed by per-tenant random material, so it is NOT
// reproducible from masterKey+tenantID. Two independent provisions of the same
// tenant under the same master key yield DIFFERENT keys, and a pure offline
// HKDF over masterKey+tenantID does not reproduce the working key.
func TestCryptoShred_SecureBehavior_Finding1(t *testing.T) {
	ctx := context.Background()
	master := testMaster(0xc1)
	const tenantID = "acme"

	provision := func() []byte {
		t.Helper()
		db := testVaultDB(t)
		vault, err := NewTenantKeyVault(ctx, TenantKeyVaultOptions{DB: db, MasterKey: master})
		if err != nil {
			t.Fatalf("NewTenantKeyVault: %v", err)
		}
		mgr, err := NewKeyManager(master, vault)
		if err != nil {
			t.Fatalf("NewKeyManager: %v", err)
		}
		key, err := mgr.TenantKey(ctx, tenantID)
		if err != nil {
			t.Fatalf("TenantKey: %v", err)
		}
		return key
	}

	// Two independent provisions of the same tenant under the same master key
	// must NOT produce the same key bytes — the DEK is random now.
	first := provision()
	second := provision()
	if bytes.Equal(first, second) {
		t.Fatalf("two independent provisions of %q under the same master key produced "+
			"identical DEKs (%x); the per-tenant key has no random component, so "+
			"crypto-shred would remain reversible", tenantID, first)
	}

	// A pure offline HKDF over masterKey+tenantID must NOT reproduce the live
	// working key.
	reader := hkdf.New(sha256.New, master, nil, []byte(hkdfInfoPrefix+tenantID))
	offline := make([]byte, KeyLength)
	if _, err := io.ReadFull(reader, offline); err != nil {
		t.Fatalf("offline HKDF: %v", err)
	}
	if bytes.Equal(offline, first) {
		t.Fatalf("offline HKDF over masterKey+tenantID reproduced the working DEK (%x); "+
			"the key is still re-derivable offline and crypto-shred is reversible", offline)
	}
}

// TestProvisionedKeyIsNotMasterDerived pins the core property directly: the
// key the vault hands out for a freshly provisioned tenant is not the HKDF
// derivation of masterKey+tenantID.
func TestProvisionedKeyIsNotMasterDerived(t *testing.T) {
	ctx := context.Background()
	master := testMaster(0x7e)
	const tenantID = "tenant-x"

	db := testVaultDB(t)
	vault, err := NewTenantKeyVault(ctx, TenantKeyVaultOptions{DB: db, MasterKey: master})
	if err != nil {
		t.Fatalf("NewTenantKeyVault: %v", err)
	}
	mgr, err := NewKeyManager(master, vault)
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	provisioned, err := mgr.TenantKey(ctx, tenantID)
	if err != nil {
		t.Fatalf("TenantKey: %v", err)
	}
	derived, err := mgr.deriveTenantKey(tenantID)
	if err != nil {
		t.Fatalf("deriveTenantKey: %v", err)
	}
	if bytes.Equal(provisioned, derived) {
		t.Fatalf("provisioned key equals the master-derived key; provisioning is not random")
	}

	// The freshly provisioned tenant is NOT a re-key candidate (origin random).
	cands, err := mgr.DerivedTenantIDs(ctx)
	if err != nil {
		t.Fatalf("DerivedTenantIDs: %v", err)
	}
	for _, id := range cands {
		if id == tenantID {
			t.Fatalf("freshly provisioned tenant %q reported as a derived-key candidate", tenantID)
		}
	}
}
