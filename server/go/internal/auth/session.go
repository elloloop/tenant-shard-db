// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// SessionManager validates an opaque session token (the value of the
// x-session-token header) and returns the associated user_id.
type SessionManager interface {
	Validate(ctx context.Context, token string) (SessionInfo, error)
}

// SessionInfo is the metadata surfaced for a successfully validated
// session. UserID is what the interceptor uses as Identity.Subject.
//
// Mirrors the dict returned by SessionManager.validate_session in
// server/python/entdb_server/auth/session_manager.py:110-138.
type SessionInfo struct {
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
	Metadata  map[string]any
}

// MemorySessionManager is the in-memory SessionManager used by tests
// and the no-auth dev server. It enforces a TTL per session and
// supports revocation. It does NOT implement the per-user max-sessions
// cap from the Python implementation -- that's not needed for unit
// tests or the W1.5 contract pin and is easy to add later.
//
// For production behind a load balancer this needs to become Redis or
// stateless signed tokens; flagged in
// docs/go-port/shared/auth-interceptor.md "Open questions / risks".
type MemorySessionManager struct {
	defaultTTL time.Duration

	mu       sync.RWMutex
	sessions map[string]*sessionEntry

	// Now lets tests pin a synthetic clock. nil -> time.Now.
	Now func() time.Time
}

type sessionEntry struct {
	userID    string
	createdAt time.Time
	expiresAt time.Time
	metadata  map[string]any
	revoked   bool
}

// NewMemorySessionManager returns a new in-memory session manager. ttl
// is the default lifetime for sessions created via Create; pass 0 to
// require an explicit ExpiresAt on every CreateWithExpiry call.
func NewMemorySessionManager(ttl time.Duration) *MemorySessionManager {
	return &MemorySessionManager{
		defaultTTL: ttl,
		sessions:   make(map[string]*sessionEntry),
	}
}

func (m *MemorySessionManager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

// Create issues a new session for userID with the manager's default
// TTL. Returns the opaque token the client should put in the
// x-session-token header.
func (m *MemorySessionManager) Create(userID string, metadata map[string]any) (string, error) {
	return m.CreateWithExpiry(userID, metadata, m.now().Add(m.defaultTTL))
}

// CreateWithExpiry issues a session with an explicit absolute expiry.
// Used by tests that need to mint already-expired tokens.
func (m *MemorySessionManager) CreateWithExpiry(userID string, metadata map[string]any, expiresAt time.Time) (string, error) {
	tok, err := randomToken(32)
	if err != nil {
		return "", err
	}
	now := m.now()
	entry := &sessionEntry{
		userID:    userID,
		createdAt: now,
		expiresAt: expiresAt,
		metadata:  metadata,
	}
	m.mu.Lock()
	m.sessions[tok] = entry
	m.mu.Unlock()
	return tok, nil
}

// Revoke marks a session as revoked. Subsequent Validate calls return
// an UNAUTHENTICATED error.
func (m *MemorySessionManager) Revoke(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.sessions[token]; ok {
		e.revoked = true
	}
}

// Validate looks up the token, checks revocation and expiry, and
// returns the session info.
func (m *MemorySessionManager) Validate(_ context.Context, token string) (SessionInfo, error) {
	if token == "" {
		return SessionInfo{}, unauthenticatedf("missing session token")
	}
	m.mu.RLock()
	entry, ok := m.sessions[token]
	m.mu.RUnlock()
	if !ok || entry == nil {
		return SessionInfo{}, unauthenticatedf("invalid session token")
	}
	if entry.revoked {
		return SessionInfo{}, unauthenticatedf("session has been revoked")
	}
	if !entry.expiresAt.IsZero() && !m.now().Before(entry.expiresAt) {
		// Lazy-evict so the map doesn't grow unboundedly under churn.
		m.mu.Lock()
		delete(m.sessions, token)
		m.mu.Unlock()
		return SessionInfo{}, unauthenticatedf("session has expired")
	}
	// Defensive copy of metadata so callers can't poison the store.
	var md map[string]any
	if entry.metadata != nil {
		md = make(map[string]any, len(entry.metadata))
		for k, v := range entry.metadata {
			md[k] = v
		}
	}
	return SessionInfo{
		UserID:    entry.userID,
		CreatedAt: entry.createdAt,
		ExpiresAt: entry.expiresAt,
		Metadata:  md,
	}, nil
}

func randomToken(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
