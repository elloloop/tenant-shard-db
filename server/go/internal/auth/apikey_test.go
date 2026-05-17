// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

func TestMemoryAPIKeyManager_ValidateRoundTrip(t *testing.T) {
	m := NewMemoryAPIKeyManager()
	keyID := m.Add("plaintext-secret", "ci-bot", []string{"read", "write"})
	if keyID == "" {
		t.Fatal("expected non-empty keyID")
	}
	if !strings.HasPrefix(keyID, "ek_") {
		t.Errorf("keyID = %q, want ek_ prefix", keyID)
	}
	info, err := m.Validate(context.Background(), "plaintext-secret")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if info.Name != "ci-bot" {
		t.Errorf("Name = %q, want ci-bot", info.Name)
	}
	if len(info.Scopes) != 2 || info.Scopes[0] != "read" || info.Scopes[1] != "write" {
		t.Errorf("Scopes = %v, want [read write]", info.Scopes)
	}
	if info.KeyID != keyID {
		t.Errorf("KeyID = %q, want %q", info.KeyID, keyID)
	}
}

func TestMemoryAPIKeyManager_RejectsUnknown(t *testing.T) {
	m := NewMemoryAPIKeyManager()
	m.Add("the-real-key", "bot", nil)
	_, err := m.Validate(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	if errs.Code(err) != codes.Unauthenticated {
		t.Errorf("code = %v, want Unauthenticated", errs.Code(err))
	}
}

func TestMemoryAPIKeyManager_RevokedKeyFails(t *testing.T) {
	m := NewMemoryAPIKeyManager()
	id := m.Add("s", "n", nil)
	m.Revoke(id)
	if _, err := m.Validate(context.Background(), "s"); err == nil {
		t.Fatal("expected revoked key to fail")
	}
}

func TestMemoryAPIKeyManager_EmptyKeyFails(t *testing.T) {
	m := NewMemoryAPIKeyManager()
	if _, err := m.Validate(context.Background(), ""); err == nil {
		t.Fatal("expected empty key to fail")
	}
}

// TestArgon2HashRoundTrip pins the hashing primitive: a fresh hash
// verifies against its own plaintext, a different plaintext does not,
// and the encoded form is a PHC argon2id string (never the plaintext or
// a bare SHA-256 hex).
func TestArgon2HashRoundTrip(t *testing.T) {
	enc, err := hashAPIKey("correct horse battery staple")
	if err != nil {
		t.Fatalf("hashAPIKey: %v", err)
	}
	if !strings.HasPrefix(enc, "$argon2id$v=19$") {
		t.Fatalf("encoded hash = %q, want $argon2id$v=19$ prefix", enc)
	}
	if strings.Contains(enc, "correct horse") {
		t.Fatal("encoded hash leaks plaintext")
	}
	ok, err := VerifyAPIKeyHash("correct horse battery staple", enc)
	if err != nil {
		t.Fatalf("VerifyAPIKeyHash: %v", err)
	}
	if !ok {
		t.Fatal("VerifyAPIKeyHash: correct password did not verify")
	}
	bad, err := VerifyAPIKeyHash("wrong password", enc)
	if err != nil {
		t.Fatalf("VerifyAPIKeyHash (wrong): %v", err)
	}
	if bad {
		t.Fatal("VerifyAPIKeyHash: wrong password verified")
	}
}

// TestArgon2HashSaltIsRandom proves two hashes of the same plaintext
// differ (per-key random salt) yet both verify.
func TestArgon2HashSaltIsRandom(t *testing.T) {
	a, err := hashAPIKey("same-secret")
	if err != nil {
		t.Fatalf("hashAPIKey a: %v", err)
	}
	b, err := hashAPIKey("same-secret")
	if err != nil {
		t.Fatalf("hashAPIKey b: %v", err)
	}
	if a == b {
		t.Fatal("two hashes of the same plaintext are identical -- salt is not random")
	}
	for _, enc := range []string{a, b} {
		ok, err := VerifyAPIKeyHash("same-secret", enc)
		if err != nil || !ok {
			t.Fatalf("salted hash failed to verify: ok=%v err=%v", ok, err)
		}
	}
}

func TestVerifyAPIKeyHash_MalformedRejected(t *testing.T) {
	for _, bad := range []string{
		"",
		"not-a-phc-string",
		"$argon2id$v=19$m=65536,t=1,p=4$onlyfourfields",
		"$bcrypt$v=19$m=65536,t=1,p=4$c2FsdA$aGFzaA",
		"$argon2id$v=99$m=65536,t=1,p=4$c2FsdA$aGFzaA",
	} {
		ok, err := VerifyAPIKeyHash("x", bad)
		if ok {
			t.Errorf("malformed hash %q verified true", bad)
		}
		if err == nil {
			t.Errorf("malformed hash %q returned nil error", bad)
		}
	}
}

func TestMemoryAPIKeyManager_ExpiredKeyFails(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	m := NewMemoryAPIKeyManager()
	m.Now = func() time.Time { return now }
	if _, err := m.AddWithExpiry("k", "bot", nil, now.Add(-time.Second)); err != nil {
		t.Fatalf("AddWithExpiry: %v", err)
	}
	if _, err := m.Validate(context.Background(), "k"); err == nil {
		t.Fatal("expected expired key to fail")
	}
}

func TestMemoryAPIKeyManager_ScopesArePerKey(t *testing.T) {
	m := NewMemoryAPIKeyManager()
	m.Add("ro-key", "reader", []string{"read"})
	m.Add("rw-key", "writer", []string{"read", "write"})

	ro, err := m.Validate(context.Background(), "ro-key")
	if err != nil {
		t.Fatalf("validate ro: %v", err)
	}
	if len(ro.Scopes) != 1 || ro.Scopes[0] != "read" {
		t.Errorf("ro scopes = %v, want [read]", ro.Scopes)
	}
	rw, err := m.Validate(context.Background(), "rw-key")
	if err != nil {
		t.Fatalf("validate rw: %v", err)
	}
	if len(rw.Scopes) != 2 {
		t.Errorf("rw scopes = %v, want 2 entries", rw.Scopes)
	}
	// Mutating the returned slice must not poison the store.
	rw.Scopes[0] = "tampered"
	again, _ := m.Validate(context.Background(), "rw-key")
	if again.Scopes[0] == "tampered" {
		t.Fatal("returned Scopes slice aliases internal state")
	}
}

// TestMemoryAPIKeyManager_RotationOverlap exercises the migration
// window: old + new keys both validate while both are active, then the
// old key is revoked and only the new key works.
func TestMemoryAPIKeyManager_RotationOverlap(t *testing.T) {
	m := NewMemoryAPIKeyManager()
	oldID := m.Add("old-secret", "svc", []string{"read"})
	newID := m.Add("new-secret", "svc", []string{"read"})
	if oldID == newID {
		t.Fatal("distinct plaintexts produced the same key_id")
	}

	// Migration window: both keys accepted.
	if _, err := m.Validate(context.Background(), "old-secret"); err != nil {
		t.Fatalf("old key should still validate during overlap: %v", err)
	}
	if _, err := m.Validate(context.Background(), "new-secret"); err != nil {
		t.Fatalf("new key should validate during overlap: %v", err)
	}

	// Cutover complete: retire the old key.
	m.Revoke(oldID)
	if _, err := m.Validate(context.Background(), "old-secret"); err == nil {
		t.Fatal("old key must fail after revocation")
	}
	if _, err := m.Validate(context.Background(), "new-secret"); err != nil {
		t.Fatalf("new key must keep working after old key revoked: %v", err)
	}
}
