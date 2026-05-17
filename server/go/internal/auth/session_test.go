// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

func TestMemorySessionManager_RoundTrip(t *testing.T) {
	m := NewMemorySessionManager(time.Hour)
	tok, err := m.Create("user:alice", map[string]any{"ip": "127.0.0.1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	info, err := m.Validate(context.Background(), tok)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if info.UserID != "user:alice" {
		t.Errorf("UserID = %q", info.UserID)
	}
	if info.Metadata["ip"] != "127.0.0.1" {
		t.Errorf("Metadata = %v", info.Metadata)
	}
}

func TestMemorySessionManager_RejectsUnknown(t *testing.T) {
	m := NewMemorySessionManager(time.Hour)
	_, err := m.Validate(context.Background(), "nope")
	if err == nil || errs.Code(err) != codes.Unauthenticated {
		t.Fatalf("want UNAUTHENTICATED, got %v", err)
	}
}

func TestMemorySessionManager_Revoke(t *testing.T) {
	m := NewMemorySessionManager(time.Hour)
	tok, _ := m.Create("u", nil)
	m.Revoke(tok)
	if _, err := m.Validate(context.Background(), tok); err == nil {
		t.Fatal("expected revoked session to fail")
	}
}

func TestMemorySessionManager_Expired(t *testing.T) {
	m := NewMemorySessionManager(time.Hour)
	tok, err := m.CreateWithExpiry("u", nil, time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("CreateWithExpiry: %v", err)
	}
	if _, err := m.Validate(context.Background(), tok); err == nil {
		t.Fatal("expected expired session to fail")
	}
}

// --- Per-user concurrent-session cap (#88 / E9.3) ---

func TestMemorySessionManager_PerUserCap_RejectsNew(t *testing.T) {
	m := NewSessionManager(time.Hour, 2, nil)

	t1, err := m.Create("alice", nil)
	if err != nil {
		t.Fatalf("session 1: %v", err)
	}
	if _, err := m.Create("alice", nil); err != nil {
		t.Fatalf("session 2: %v", err)
	}

	// Third session for alice must be rejected with ResourceExhausted.
	_, err = m.Create("alice", nil)
	if err == nil {
		t.Fatal("expected the over-cap session to be rejected")
	}
	if errs.Code(err) != codes.ResourceExhausted {
		t.Fatalf("want ResourceExhausted, got %v (%v)", errs.Code(err), err)
	}

	// A different user is unaffected by alice's cap.
	if _, err := m.Create("bob", nil); err != nil {
		t.Fatalf("bob should not be capped by alice: %v", err)
	}

	// Existing sessions still validate -- reject-new must not have
	// evicted an established session.
	if _, err := m.Validate(context.Background(), t1); err != nil {
		t.Fatalf("first session should still be valid: %v", err)
	}
}

func TestMemorySessionManager_PerUserCap_RevokeFreesSlot(t *testing.T) {
	m := NewSessionManager(time.Hour, 1, nil)

	t1, err := m.Create("alice", nil)
	if err != nil {
		t.Fatalf("session 1: %v", err)
	}
	if _, err := m.Create("alice", nil); errs.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected cap to reject 2nd session, got %v", err)
	}

	// Revoking the first session must free the slot.
	m.Revoke(t1)
	if _, err := m.Create("alice", nil); err != nil {
		t.Fatalf("slot should be free after revoke: %v", err)
	}
}

func TestMemorySessionManager_PerUserCap_ExpiredDoesNotCount(t *testing.T) {
	clock := time.Now()
	m := NewSessionManager(0, 1, nil)
	m.Now = func() time.Time { return clock }

	// An already-expired session must not permanently consume the
	// user's single slot.
	if _, err := m.CreateWithExpiry("alice", nil, clock.Add(-time.Minute)); err != nil {
		t.Fatalf("CreateWithExpiry: %v", err)
	}
	if _, err := m.CreateWithExpiry("alice", nil, clock.Add(time.Hour)); err != nil {
		t.Fatalf("expired session should not occupy the cap slot: %v", err)
	}
}

func TestMemorySessionManager_PerUserCap_Unlimited(t *testing.T) {
	m := NewMemorySessionManager(time.Hour) // maxPerUser == 0 -> unlimited
	for i := 0; i < 50; i++ {
		if _, err := m.Create("alice", nil); err != nil {
			t.Fatalf("unlimited cap should allow session %d: %v", i, err)
		}
	}
}

// --- Request-checked revocation list (#88 / E9.3) ---

// TestMemorySessionManager_RevokeImmediateNextRequest pins the
// "immediate token revocation list (checked on every request)"
// contract: a revoked but non-expired token must fail the very next
// Validate, not only after TTL lapses.
func TestMemorySessionManager_RevokeImmediateNextRequest(t *testing.T) {
	m := NewMemorySessionManager(time.Hour)
	tok, _ := m.Create("alice", nil)

	if _, err := m.Validate(context.Background(), tok); err != nil {
		t.Fatalf("fresh session should validate: %v", err)
	}

	m.Revoke(tok)

	_, err := m.Validate(context.Background(), tok)
	if err == nil {
		t.Fatal("revoked token must fail on the next request")
	}
	if errs.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", errs.Code(err))
	}
}

// Revocation must outlive the session row: even after the entry is
// lazily evicted on expiry, replaying the revoked token must still be
// rejected as revoked rather than merely "invalid".
func TestMemorySessionManager_RevocationOutlivesEntry(t *testing.T) {
	clock := time.Now()
	m := NewMemorySessionManager(0)
	m.Now = func() time.Time { return clock }

	tok, _ := m.CreateWithExpiry("alice", nil, clock.Add(time.Minute))
	m.Revoke(tok)

	// Advance past expiry and validate -- this both reports revoked
	// and (per Validate ordering) never reaches lazy eviction.
	clock = clock.Add(2 * time.Minute)
	_, err := m.Validate(context.Background(), tok)
	if err == nil || errs.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}

	// Even with the row gone, a replay is still rejected as revoked.
	if _, err := m.Validate(context.Background(), tok); err == nil {
		t.Fatal("replayed revoked token must stay rejected")
	}
}

func TestMemorySessionManager_RevokeUser(t *testing.T) {
	m := NewMemorySessionManager(time.Hour)
	a1, _ := m.Create("alice", nil)
	a2, _ := m.Create("alice", nil)
	b1, _ := m.Create("bob", nil)

	if err := m.RevokeUser("alice"); err != nil {
		t.Fatalf("RevokeUser: %v", err)
	}

	for _, tok := range []string{a1, a2} {
		if _, err := m.Validate(context.Background(), tok); err == nil {
			t.Fatalf("alice token %s should be revoked", tok[:8])
		}
	}
	// Bob's session is untouched.
	if _, err := m.Validate(context.Background(), b1); err != nil {
		t.Fatalf("bob's session should survive RevokeUser(alice): %v", err)
	}
}

// --- Pluggable SessionStore seam (#88 / E9.3) ---

// countingStore wraps the default in-memory store and counts calls,
// proving MemorySessionManager drives all persistence through the
// SessionStore interface (the Redis seam).
type countingStore struct {
	inner    SessionStore
	puts     int
	isRevokd int
}

func (c *countingStore) Put(ctx context.Context, t string, s Session) error {
	c.puts++
	return c.inner.Put(ctx, t, s)
}

func (c *countingStore) Get(ctx context.Context, t string) (Session, bool, error) {
	return c.inner.Get(ctx, t)
}
func (c *countingStore) Delete(ctx context.Context, t string) error {
	return c.inner.Delete(ctx, t)
}
func (c *countingStore) ActiveTokens(ctx context.Context, u string) ([]string, error) {
	return c.inner.ActiveTokens(ctx, u)
}
func (c *countingStore) Revoke(ctx context.Context, t string) error {
	return c.inner.Revoke(ctx, t)
}
func (c *countingStore) IsRevoked(ctx context.Context, t string) (bool, error) {
	c.isRevokd++
	return c.inner.IsRevoked(ctx, t)
}

func TestMemorySessionManager_UsesPluggableStore(t *testing.T) {
	cs := &countingStore{inner: newMemorySessionStore()}
	m := NewSessionManager(time.Hour, 0, cs)

	tok, err := m.Create("alice", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cs.puts != 1 {
		t.Fatalf("expected Create to Put once, got %d", cs.puts)
	}
	if _, err := m.Validate(context.Background(), tok); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// Validate must consult the revocation list on every request.
	if cs.isRevokd == 0 {
		t.Fatal("Validate must check IsRevoked on every request")
	}
}
