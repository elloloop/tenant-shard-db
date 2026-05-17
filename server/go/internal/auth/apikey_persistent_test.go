// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

// fakeAPIKeyStore is an in-memory APIKeyStore for unit-testing the
// persistent manager without global.db. now() lets tests drive expiry.
type fakeAPIKeyStore struct {
	rows    map[string]*StoredAPIKey
	revoked map[string]bool
	now     func() int64
	failOn  string // if set, the named method returns an error
}

func newFakeStore() *fakeAPIKeyStore {
	return &fakeAPIKeyStore{
		rows:    map[string]*StoredAPIKey{},
		revoked: map[string]bool{},
		now:     func() int64 { return 1_000_000 },
	}
}

func (f *fakeAPIKeyStore) PutAPIKey(_ context.Context, k StoredAPIKey) error {
	if f.failOn == "put" {
		return errors.New("boom")
	}
	if _, ok := f.rows[k.KeyID]; ok {
		return errors.New("duplicate key_id")
	}
	cp := k
	cp.Scopes = append([]string(nil), k.Scopes...)
	f.rows[k.KeyID] = &cp
	return nil
}

func (f *fakeAPIKeyStore) RevokeAPIKey(_ context.Context, keyID string) (bool, error) {
	if f.failOn == "revoke" {
		return false, errors.New("boom")
	}
	if _, ok := f.rows[keyID]; !ok || f.revoked[keyID] {
		return false, nil
	}
	f.revoked[keyID] = true
	return true, nil
}

func (f *fakeAPIKeyStore) ListActiveAPIKeys(_ context.Context, now int64) ([]StoredAPIKey, error) {
	if f.failOn == "list" {
		return nil, errors.New("db unavailable")
	}
	if now == 0 {
		now = f.now()
	}
	out := []StoredAPIKey{}
	for id, r := range f.rows {
		if f.revoked[id] {
			continue
		}
		if r.ExpiresAt != 0 && r.ExpiresAt <= now {
			continue
		}
		out = append(out, *r)
	}
	return out, nil
}

func TestPersistentAPIKeyManager_IssueValidateRoundTrip(t *testing.T) {
	store := newFakeStore()
	m := NewPersistentAPIKeyManager(store)
	m.Now = func() time.Time { return time.Unix(1_000_000, 0) }

	id, err := m.Issue(context.Background(), "super-secret", "deploy-bot", []string{"read", "write"}, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if id == "" {
		t.Fatal("Issue returned empty key_id")
	}
	// The stored row must carry an argon2id hash, never the plaintext.
	row := store.rows[id]
	if row == nil {
		t.Fatal("Issue did not persist a row")
	}
	if row.Hash == "super-secret" || !hasArgon2Prefix(row.Hash) {
		t.Fatalf("stored hash not argon2id: %q", row.Hash)
	}

	info, err := m.Validate(context.Background(), "super-secret")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if info.Name != "deploy-bot" {
		t.Errorf("Name = %q, want deploy-bot", info.Name)
	}
	if len(info.Scopes) != 2 {
		t.Errorf("Scopes = %v, want 2", info.Scopes)
	}
	if info.KeyID != id {
		t.Errorf("KeyID = %q, want %q", info.KeyID, id)
	}
}

func TestPersistentAPIKeyManager_UnknownAndEmpty(t *testing.T) {
	m := NewPersistentAPIKeyManager(newFakeStore())
	if _, err := m.Validate(context.Background(), ""); errs.Code(err) != codes.Unauthenticated {
		t.Errorf("empty key code = %v, want Unauthenticated", errs.Code(err))
	}
	if _, err := m.Validate(context.Background(), "ghost"); errs.Code(err) != codes.Unauthenticated {
		t.Errorf("unknown key code = %v, want Unauthenticated", errs.Code(err))
	}
}

// TestPersistentAPIKeyManager_RotationOverlap is the documented
// migration window end-to-end against the persistence layer: issue new,
// both validate, revoke old, only new validates.
func TestPersistentAPIKeyManager_RotationOverlap(t *testing.T) {
	store := newFakeStore()
	m := NewPersistentAPIKeyManager(store)
	ctx := context.Background()

	oldID, err := m.Issue(ctx, "old-secret", "svc", []string{"read"}, 0)
	if err != nil {
		t.Fatalf("issue old: %v", err)
	}
	newID, err := m.Issue(ctx, "new-secret", "svc", []string{"read"}, 0)
	if err != nil {
		t.Fatalf("issue new: %v", err)
	}
	if oldID == newID {
		t.Fatal("distinct secrets produced same key_id")
	}

	// Overlap: both keys valid.
	if _, err := m.Validate(ctx, "old-secret"); err != nil {
		t.Fatalf("old key should validate during overlap: %v", err)
	}
	if _, err := m.Validate(ctx, "new-secret"); err != nil {
		t.Fatalf("new key should validate during overlap: %v", err)
	}

	// Retire the old key.
	flipped, err := m.Revoke(ctx, oldID)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if !flipped {
		t.Fatal("Revoke reported no active key flipped")
	}
	// Idempotent: re-revoking returns false.
	if again, _ := m.Revoke(ctx, oldID); again {
		t.Fatal("second Revoke should be a no-op (false)")
	}

	if _, err := m.Validate(ctx, "old-secret"); err == nil {
		t.Fatal("revoked old key must fail")
	}
	if _, err := m.Validate(ctx, "new-secret"); err != nil {
		t.Fatalf("new key must survive old-key revocation: %v", err)
	}
}

func TestPersistentAPIKeyManager_ExpiryEnforced(t *testing.T) {
	store := newFakeStore()
	m := NewPersistentAPIKeyManager(store)
	clock := int64(1_000_000)
	store.now = func() int64 { return clock }
	m.Now = func() time.Time { return time.Unix(clock, 0) }
	ctx := context.Background()

	if _, err := m.Issue(ctx, "short-lived", "bot", []string{"read"}, clock+100); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := m.Validate(ctx, "short-lived"); err != nil {
		t.Fatalf("key valid before expiry: %v", err)
	}
	clock += 200 // advance past expiry
	if _, err := m.Validate(ctx, "short-lived"); err == nil {
		t.Fatal("expired key must fail")
	}
}

func TestPersistentAPIKeyManager_ScopeEnforcement(t *testing.T) {
	store := newFakeStore()
	m := NewPersistentAPIKeyManager(store)
	ctx := context.Background()

	if _, err := m.Issue(ctx, "ro", "reader", []string{"read"}, 0); err != nil {
		t.Fatalf("issue ro: %v", err)
	}
	if _, err := m.Issue(ctx, "admin", "ops", []string{"read", "write", "admin"}, 0); err != nil {
		t.Fatalf("issue admin: %v", err)
	}

	ro, err := m.Validate(ctx, "ro")
	if err != nil {
		t.Fatalf("validate ro: %v", err)
	}
	if len(ro.Scopes) != 1 || ro.Scopes[0] != "read" {
		t.Fatalf("ro scopes = %v, want [read]", ro.Scopes)
	}
	admin, err := m.Validate(ctx, "admin")
	if err != nil {
		t.Fatalf("validate admin: %v", err)
	}
	if len(admin.Scopes) != 3 {
		t.Fatalf("admin scopes = %v, want 3", admin.Scopes)
	}
}

// TestPersistentAPIKeyManager_StoreFailureNotAnInvalidKey ensures a DB
// outage surfaces as a distinct UNAUTHENTICATED message (availability
// problem), not as a silent "invalid API key" that would mask the
// outage.
func TestPersistentAPIKeyManager_StoreFailureNotAnInvalidKey(t *testing.T) {
	store := newFakeStore()
	store.failOn = "list"
	m := NewPersistentAPIKeyManager(store)
	_, err := m.Validate(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error when store fails")
	}
	if errs.Code(err) != codes.Unauthenticated {
		t.Errorf("code = %v, want Unauthenticated", errs.Code(err))
	}
	if got := err.Error(); !strings.Contains(got, "store unavailable") {
		t.Errorf("error = %q, want it to mention store unavailability", got)
	}
}

func hasArgon2Prefix(s string) bool { return strings.Contains(s, "$argon2id$v=19$") }
