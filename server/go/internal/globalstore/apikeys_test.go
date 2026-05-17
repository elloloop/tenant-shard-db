// SPDX-License-Identifier: AGPL-3.0-only

package globalstore_test

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
)

func TestAPIKeyPutGetRoundTrip(t *testing.T) {
	gs := newStore(t, func() int64 { return 1_700_000_000 })
	ctx := context.Background()

	rec := globalstore.APIKeyRecord{
		KeyID:    "ek_abc",
		TenantID: "acme",
		Name:     "ci-bot",
		Hash:     "$argon2id$v=19$m=65536,t=1,p=4$c2FsdHNhbHQ$aGFzaA",
		Scopes:   []string{"read", "write"},
	}
	if err := gs.PutAPIKey(ctx, rec); err != nil {
		t.Fatalf("PutAPIKey: %v", err)
	}

	got, err := gs.GetAPIKey(ctx, "ek_abc")
	if err != nil {
		t.Fatalf("GetAPIKey: %v", err)
	}
	if got == nil {
		t.Fatal("GetAPIKey returned nil for an inserted key")
	}
	if got.Name != "ci-bot" || got.Hash != rec.Hash {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if got.Status != globalstore.APIKeyStatusActive {
		t.Errorf("Status = %q, want active", got.Status)
	}
	if len(got.Scopes) != 2 || got.Scopes[0] != "read" || got.Scopes[1] != "write" {
		t.Errorf("Scopes = %v, want [read write]", got.Scopes)
	}
	if got.CreatedAt != 1_700_000_000 {
		t.Errorf("CreatedAt = %d, want store clock", got.CreatedAt)
	}
}

func TestAPIKeyGetMissingReturnsNil(t *testing.T) {
	gs := newStore(t, nil)
	got, err := gs.GetAPIKey(context.Background(), "ek_nope")
	if err != nil {
		t.Fatalf("GetAPIKey: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing key, got %+v", got)
	}
}

func TestAPIKeyDuplicateRejected(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()
	rec := globalstore.APIKeyRecord{KeyID: "ek_dup", Name: "n", Hash: "h"}
	if err := gs.PutAPIKey(ctx, rec); err != nil {
		t.Fatalf("first PutAPIKey: %v", err)
	}
	if err := gs.PutAPIKey(ctx, rec); err == nil {
		t.Fatal("expected duplicate key_id to be rejected")
	}
}

func TestAPIKeyRequiresKeyIDAndHash(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()
	if err := gs.PutAPIKey(ctx, globalstore.APIKeyRecord{Hash: "h"}); err == nil {
		t.Error("expected error when key_id missing")
	}
	if err := gs.PutAPIKey(ctx, globalstore.APIKeyRecord{KeyID: "ek_x"}); err == nil {
		t.Error("expected error when hash missing")
	}
}

// TestAPIKeyListActiveFiltersRevokedAndExpired is the rotation-window
// contract: only active, unexpired keys come back, so the auth layer
// accepts exactly the keys that should still work.
func TestAPIKeyListActiveFiltersRevokedAndExpired(t *testing.T) {
	clock := int64(2_000_000)
	gs := newStore(t, func() int64 { return clock })
	ctx := context.Background()

	mustPut := func(id string, expires int64) {
		t.Helper()
		if err := gs.PutAPIKey(ctx, globalstore.APIKeyRecord{
			KeyID: id, Name: "svc", Hash: "h-" + id, ExpiresAt: expires,
		}); err != nil {
			t.Fatalf("PutAPIKey %s: %v", id, err)
		}
	}
	mustPut("ek_live", 0)               // never expires
	mustPut("ek_future", clock+10_000)  // not yet expired
	mustPut("ek_expired", clock-1)      // already expired
	mustPut("ek_revoked", 0)            // will be revoked below

	flipped, err := gs.RevokeAPIKey(ctx, "ek_revoked")
	if err != nil {
		t.Fatalf("RevokeAPIKey: %v", err)
	}
	if !flipped {
		t.Fatal("RevokeAPIKey reported no active row flipped")
	}

	active, err := gs.ListActiveAPIKeys(ctx, 0)
	if err != nil {
		t.Fatalf("ListActiveAPIKeys: %v", err)
	}
	got := map[string]bool{}
	for _, k := range active {
		got[k.KeyID] = true
	}
	if !got["ek_live"] || !got["ek_future"] {
		t.Errorf("active set missing live/future keys: %v", got)
	}
	if got["ek_expired"] {
		t.Error("expired key leaked into active set")
	}
	if got["ek_revoked"] {
		t.Error("revoked key leaked into active set")
	}
}

func TestAPIKeyRevokeIsIdempotent(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()
	if err := gs.PutAPIKey(ctx, globalstore.APIKeyRecord{KeyID: "ek_r", Name: "n", Hash: "h"}); err != nil {
		t.Fatalf("PutAPIKey: %v", err)
	}
	first, err := gs.RevokeAPIKey(ctx, "ek_r")
	if err != nil || !first {
		t.Fatalf("first RevokeAPIKey: flipped=%v err=%v", first, err)
	}
	second, err := gs.RevokeAPIKey(ctx, "ek_r")
	if err != nil {
		t.Fatalf("second RevokeAPIKey: %v", err)
	}
	if second {
		t.Fatal("second RevokeAPIKey should report no active row flipped")
	}
	missing, err := gs.RevokeAPIKey(ctx, "ek_does_not_exist")
	if err != nil {
		t.Fatalf("RevokeAPIKey unknown: %v", err)
	}
	if missing {
		t.Fatal("revoking an unknown key should report false")
	}

	got, err := gs.GetAPIKey(ctx, "ek_r")
	if err != nil {
		t.Fatalf("GetAPIKey: %v", err)
	}
	if got.Status != globalstore.APIKeyStatusRevoked || got.RevokedAt == 0 {
		t.Errorf("revoked key state = %+v, want status=revoked & revoked_at set", got)
	}
}

func TestAPIKeyListAndDelete(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()
	for _, id := range []string{"ek_1", "ek_2", "ek_3"} {
		if err := gs.PutAPIKey(ctx, globalstore.APIKeyRecord{
			KeyID: id, TenantID: "t1", Name: "n", Hash: "h",
		}); err != nil {
			t.Fatalf("PutAPIKey %s: %v", id, err)
		}
	}
	// Different tenant -- must not show up in the t1-scoped list.
	if err := gs.PutAPIKey(ctx, globalstore.APIKeyRecord{
		KeyID: "ek_other", TenantID: "t2", Name: "n", Hash: "h",
	}); err != nil {
		t.Fatalf("PutAPIKey other-tenant: %v", err)
	}

	t1, err := gs.ListAPIKeys(ctx, "t1", 0)
	if err != nil {
		t.Fatalf("ListAPIKeys t1: %v", err)
	}
	if len(t1) != 3 {
		t.Fatalf("ListAPIKeys t1 = %d rows, want 3", len(t1))
	}
	all, err := gs.ListAPIKeys(ctx, "", 0)
	if err != nil {
		t.Fatalf("ListAPIKeys all: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("ListAPIKeys all = %d rows, want 4", len(all))
	}

	removed, err := gs.DeleteAPIKey(ctx, "ek_1")
	if err != nil || !removed {
		t.Fatalf("DeleteAPIKey: removed=%v err=%v", removed, err)
	}
	if again, _ := gs.DeleteAPIKey(ctx, "ek_1"); again {
		t.Fatal("second DeleteAPIKey should report false")
	}
	got, err := gs.GetAPIKey(ctx, "ek_1")
	if err != nil {
		t.Fatalf("GetAPIKey after delete: %v", err)
	}
	if got != nil {
		t.Fatal("deleted key still present")
	}
}

// TestAPIKeyPersistenceSurvivesReopen proves durability: keys written
// before Close are still readable after re-opening global.db.
func TestAPIKeyPersistenceSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	gs1, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	if err := gs1.PutAPIKey(ctx, globalstore.APIKeyRecord{
		KeyID: "ek_durable", Name: "svc", Hash: "$argon2id$v=19$m=1$s$h", Scopes: []string{"read"},
	}); err != nil {
		t.Fatalf("PutAPIKey: %v", err)
	}
	if err := gs1.Close(); err != nil {
		t.Fatalf("close 1: %v", err)
	}

	gs2, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("open 2: %v", err)
	}
	t.Cleanup(func() { _ = gs2.Close() })

	got, err := gs2.GetAPIKey(ctx, "ek_durable")
	if err != nil {
		t.Fatalf("GetAPIKey after reopen: %v", err)
	}
	if got == nil || got.Name != "svc" || len(got.Scopes) != 1 {
		t.Fatalf("key did not survive reopen: %+v", got)
	}
}
