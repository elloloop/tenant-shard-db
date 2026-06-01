package crypto

import (
	"bytes"
	"context"
	"testing"
)

// seedDerivedVaultRow inserts a legacy derived-key vault row (key_origin
// 'derived') wrapping derivedKey, modelling a tenant provisioned before #638.
func seedDerivedVaultRow(t *testing.T, v *TenantKeyVault, tenantID string, derivedKey []byte) {
	t.Helper()
	wrapped, err := v.wrap(derivedKey, tenantID)
	if err != nil {
		t.Fatalf("wrap derived key: %v", err)
	}
	if _, err := v.db.ExecContext(context.Background(),
		`INSERT INTO tenant_key_vault (tenant_id, wrapped_key, created_at_ms, key_origin) VALUES (?, ?, ?, ?)`,
		tenantID, wrapped, v.nowFn().UnixMilli(), keyOriginDerived,
	); err != nil {
		t.Fatalf("seed derived vault row: %v", err)
	}
}

func newVaultMgr(t *testing.T, fill byte) (context.Context, []byte, *TenantKeyVault, *KeyManager) {
	t.Helper()
	ctx := context.Background()
	master := testMaster(fill)
	db := testVaultDB(t)
	vault, err := NewTenantKeyVault(ctx, TenantKeyVaultOptions{DB: db, MasterKey: master})
	if err != nil {
		t.Fatalf("NewTenantKeyVault: %v", err)
	}
	mgr, err := NewKeyManager(master, vault)
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	return ctx, master, vault, mgr
}

// TestMigrateDerivedTenants_RekeysToRandom is the core migration test: a
// legacy derived-key tenant is re-keyed to a fresh random key that is no
// longer re-derivable from masterKey+tenantID, and the DB re-key is driven
// with the correct (old, new) key pair.
func TestMigrateDerivedTenants_RekeysToRandom(t *testing.T) {
	ctx, _, vault, mgr := newVaultMgr(t, 0xa1)
	const tenantID = "legacy"

	derivedKey, err := mgr.deriveTenantKey(tenantID)
	if err != nil {
		t.Fatalf("deriveTenantKey: %v", err)
	}
	seedDerivedVaultRow(t, vault, tenantID, derivedKey)

	cands, err := mgr.DerivedTenantIDs(ctx)
	if err != nil {
		t.Fatalf("DerivedTenantIDs: %v", err)
	}
	if len(cands) != 1 || cands[0] != tenantID {
		t.Fatalf("candidates = %v, want [%q]", cands, tenantID)
	}

	var gotOld, gotNew []byte
	calls := 0
	rekeyDB := func(_ context.Context, id string, oldKey, newKey []byte) error {
		calls++
		if id != tenantID {
			t.Errorf("rekeyDB tenant = %q, want %q", id, tenantID)
		}
		gotOld = append([]byte(nil), oldKey...)
		gotNew = append([]byte(nil), newKey...)
		return nil
	}

	report, err := mgr.MigrateDerivedTenants(ctx, rekeyDB)
	if err != nil {
		t.Fatalf("MigrateDerivedTenants: %v", err)
	}
	if calls != 1 {
		t.Fatalf("rekeyDB called %d times, want 1", calls)
	}
	if len(report.Rekeyed) != 1 || report.Rekeyed[0] != tenantID {
		t.Fatalf("report.Rekeyed = %v, want [%q]", report.Rekeyed, tenantID)
	}
	if len(report.Recovered) != 0 {
		t.Fatalf("report.Recovered = %v, want empty", report.Recovered)
	}
	if !bytes.Equal(gotOld, derivedKey) {
		t.Fatalf("rekeyDB old key = %x, want the derived key %x", gotOld, derivedKey)
	}
	if bytes.Equal(gotNew, derivedKey) {
		t.Fatalf("rekeyDB new key equals the derived key; not random")
	}

	// The vault now hands out the new random key, marked 'random'.
	cur, err := vault.Get(ctx, tenantID)
	if err != nil {
		t.Fatalf("vault.Get after migrate: %v", err)
	}
	if !bytes.Equal(cur, gotNew) {
		t.Fatalf("vault current key %x != rekeyDB new key %x", cur, gotNew)
	}
	if org, _ := vault.keyOriginOf(ctx, tenantID); org != keyOriginRandom {
		t.Fatalf("key_origin = %q, want %q", org, keyOriginRandom)
	}
	// Irreversibility: master+tenantID no longer derives the live key.
	rederived, _ := mgr.deriveTenantKey(tenantID)
	if bytes.Equal(rederived, cur) {
		t.Fatalf("post-migration key is still re-derivable from masterKey+tenantID")
	}
	// No longer a candidate.
	if cands2, _ := mgr.DerivedTenantIDs(ctx); len(cands2) != 0 {
		t.Fatalf("tenant still a candidate after migration: %v", cands2)
	}
}

// TestMigrateDerivedTenants_Idempotent: a second run is a no-op (the tenant is
// already 'random').
func TestMigrateDerivedTenants_Idempotent(t *testing.T) {
	ctx, _, vault, mgr := newVaultMgr(t, 0xb2)
	const tenantID = "legacy"
	dk, _ := mgr.deriveTenantKey(tenantID)
	seedDerivedVaultRow(t, vault, tenantID, dk)

	noop := func(_ context.Context, _ string, _, _ []byte) error { return nil }
	if _, err := mgr.MigrateDerivedTenants(ctx, noop); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	calls := 0
	report, err := mgr.MigrateDerivedTenants(ctx, func(_ context.Context, _ string, _, _ []byte) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if calls != 0 || len(report.Rekeyed) != 0 || len(report.Recovered) != 0 {
		t.Fatalf("second migrate not a no-op: calls=%d report=%+v", calls, report)
	}
}

// TestMigrateDerivedTenants_ResumesInterrupted: a tenant left in 'rekeying'
// state (a prior run crashed after staging) is completed on re-run using the
// staged previous/new key pair, and the DB re-key is driven idempotently.
func TestMigrateDerivedTenants_ResumesInterrupted(t *testing.T) {
	ctx, _, vault, mgr := newVaultMgr(t, 0xc3)
	const tenantID = "interrupted"
	oldKey, _ := mgr.deriveTenantKey(tenantID)
	seedDerivedVaultRow(t, vault, tenantID, oldKey)

	// Simulate a crash mid-migration: stage a new key (origin -> 'rekeying',
	// prev_wrapped_key = old) but never complete.
	stagedNew, err := newRandomKey()
	if err != nil {
		t.Fatalf("newRandomKey: %v", err)
	}
	if err := vault.stageRekey(ctx, tenantID, stagedNew); err != nil {
		t.Fatalf("stageRekey: %v", err)
	}
	if org, _ := vault.keyOriginOf(ctx, tenantID); org != keyOriginRekeying {
		t.Fatalf("post-stage origin = %q, want %q", org, keyOriginRekeying)
	}

	var gotOld, gotNew []byte
	report, err := mgr.MigrateDerivedTenants(ctx, func(_ context.Context, _ string, o, n []byte) error {
		gotOld = append([]byte(nil), o...)
		gotNew = append([]byte(nil), n...)
		return nil
	})
	if err != nil {
		t.Fatalf("resume migrate: %v", err)
	}
	if len(report.Recovered) != 1 || report.Recovered[0] != tenantID {
		t.Fatalf("report.Recovered = %v, want [%q]", report.Recovered, tenantID)
	}
	if len(report.Rekeyed) != 0 {
		t.Fatalf("report.Rekeyed = %v, want empty (this was a resume)", report.Rekeyed)
	}
	// Resume must re-key from the staged OLD key to the staged NEW key.
	if !bytes.Equal(gotOld, oldKey) {
		t.Fatalf("resume old key = %x, want staged previous %x", gotOld, oldKey)
	}
	if !bytes.Equal(gotNew, stagedNew) {
		t.Fatalf("resume new key = %x, want staged new %x", gotNew, stagedNew)
	}
	// Finalized: origin 'random', prev cleared, current == staged new.
	if org, _ := vault.keyOriginOf(ctx, tenantID); org != keyOriginRandom {
		t.Fatalf("post-resume origin = %q, want %q", org, keyOriginRandom)
	}
	cur, err := vault.Get(ctx, tenantID)
	if err != nil {
		t.Fatalf("vault.Get after resume: %v", err)
	}
	if !bytes.Equal(cur, stagedNew) {
		t.Fatalf("post-resume current key %x != staged new %x", cur, stagedNew)
	}
}

// TestMigrateDerivedTenants_RequiresVault: a vault-less KeyManager cannot
// durably re-key, so migration is refused rather than silently no-op.
func TestMigrateDerivedTenants_RequiresVault(t *testing.T) {
	mgr, err := NewKeyManager(testMaster(0xd4), nil)
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	if _, err := mgr.MigrateDerivedTenants(context.Background(),
		func(_ context.Context, _ string, _, _ []byte) error { return nil }); err == nil {
		t.Fatal("MigrateDerivedTenants with no vault: expected an error, got nil")
	}
}
