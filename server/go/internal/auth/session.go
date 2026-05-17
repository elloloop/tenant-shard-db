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
type SessionInfo struct {
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
	Metadata  map[string]any
}

// Session is the persisted server-side record for one issued token. It
// is the unit a SessionStore stores and returns. The token itself is
// the store's primary key and is NOT carried inside the struct.
type Session struct {
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
	Metadata  map[string]any
}

// SessionStore is the pluggable persistence seam behind
// MemorySessionManager. It owns two pieces of state:
//
//  1. the live session table (token -> Session), and
//  2. the revocation list -- the set of tokens that must be rejected
//     on the very next request regardless of TTL.
//
// The default implementation (MemorySession Store) keeps both in
// process memory. Production deployments behind a load balancer need
// the state shared across replicas: implement this interface against
// Redis (or a stateless signed-token scheme that needs no live table
// but still consults a small revocation set). The seam is deliberately
// narrow so a Redis adapter is a thin translation layer:
//
//	Put          -> SET session:<tok> <json> EX <ttl>
//	Get          -> GET session:<tok>
//	Delete       -> DEL session:<tok>
//	UserTokens   -> SMEMBERS user_sessions:<uid> (maintained by Put/Delete)
//	Revoke       -> SADD revoked + SET revoked:<tok> EX <ttl> (self-expiring)
//	IsRevoked    -> EXISTS revoked:<tok>
//
// A SessionStore implementation MUST be safe for concurrent use.
//
// Revocation outlives the session entry on purpose: a token can be
// revoked, its session lazily evicted on expiry, and a replayed
// request with the same token must still be rejected as revoked rather
// than merely "unknown" -- so IsRevoked does not depend on Get
// returning a live session.
type SessionStore interface {
	// Put stores (or replaces) the session for token.
	Put(ctx context.Context, token string, s Session) error
	// Get returns the session for token. ok is false when absent.
	Get(ctx context.Context, token string) (s Session, ok bool, err error)
	// Delete removes the session for token (used by lazy expiry).
	// It MUST NOT clear an existing revocation entry for the token.
	Delete(ctx context.Context, token string) error
	// ActiveTokens returns the tokens currently held for userID. The
	// caller filters expired entries; the store need not.
	ActiveTokens(ctx context.Context, userID string) ([]string, error)
	// Revoke records token in the revocation list. Subsequent
	// IsRevoked calls for token MUST return true. Idempotent.
	Revoke(ctx context.Context, token string) error
	// IsRevoked reports whether token is on the revocation list.
	IsRevoked(ctx context.Context, token string) (bool, error)
}

// MemorySessionManager is the in-memory SessionManager used by tests
// and the no-auth dev server. It enforces a per-session TTL, a
// per-user concurrent-session cap, and a revocation list that is
// consulted on every authenticated request (not only lazily on
// expiry).
//
// Persistence is delegated to a SessionStore (memorySessionStore by
// default). For production behind a load balancer, construct it with a
// Redis-backed SessionStore so revocation and the session table are
// shared across replicas. See SessionStore's doc comment for the Redis
// mapping.
type MemorySessionManager struct {
	defaultTTL time.Duration

	// maxPerUser caps concurrent (unexpired, non-revoked) sessions per
	// user. 0 means unlimited. When a Create would exceed the cap the
	// new session is REJECTED with a ResourceExhausted error rather
	// than silently evicting the oldest one -- evicting the oldest
	// would let an attacker who can mint sessions push a legitimate
	// user off their own login without any signal. Reject-new makes
	// the limit visible to the caller.
	maxPerUser int

	store SessionStore

	// Now lets tests pin a synthetic clock. nil -> time.Now.
	Now func() time.Time
}

// NewMemorySessionManager returns a new in-memory session manager. ttl
// is the default lifetime for sessions created via Create; pass 0 to
// require an explicit ExpiresAt on every CreateWithExpiry call.
//
// The returned manager has no per-user session cap (unlimited) and a
// fresh in-memory store. Use NewSessionManager for production wiring
// with a cap and/or an external (Redis) store.
func NewMemorySessionManager(ttl time.Duration) *MemorySessionManager {
	return NewSessionManager(ttl, 0, nil)
}

// NewSessionManager is the full constructor. maxPerUser caps concurrent
// sessions per user (0 = unlimited). store is the persistence backend;
// pass nil to use the default in-memory store.
func NewSessionManager(ttl time.Duration, maxPerUser int, store SessionStore) *MemorySessionManager {
	if store == nil {
		store = newMemorySessionStore()
	}
	if maxPerUser < 0 {
		maxPerUser = 0
	}
	return &MemorySessionManager{
		defaultTTL: ttl,
		maxPerUser: maxPerUser,
		store:      store,
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
//
// If userID is already at the configured per-user cap (counting only
// unexpired, non-revoked sessions) Create returns a ResourceExhausted
// error and no token is issued.
func (m *MemorySessionManager) Create(userID string, metadata map[string]any) (string, error) {
	return m.CreateWithExpiry(userID, metadata, m.now().Add(m.defaultTTL))
}

// CreateWithExpiry issues a session with an explicit absolute expiry.
// Used by tests that need to mint already-expired tokens. The per-user
// cap is enforced here too.
func (m *MemorySessionManager) CreateWithExpiry(userID string, metadata map[string]any, expiresAt time.Time) (string, error) {
	ctx := context.Background()

	if m.maxPerUser > 0 {
		active, err := m.countActive(ctx, userID)
		if err != nil {
			return "", err
		}
		if active >= m.maxPerUser {
			return "", resourceExhaustedf(
				"user %q has reached the maximum of %d concurrent sessions; revoke an existing session before creating a new one",
				userID, m.maxPerUser)
		}
	}

	tok, err := randomToken(32)
	if err != nil {
		return "", err
	}
	now := m.now()
	if err := m.store.Put(ctx, tok, Session{
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: expiresAt,
		Metadata:  metadata,
	}); err != nil {
		return "", err
	}
	return tok, nil
}

// countActive returns the number of unexpired, non-revoked sessions
// held for userID. Expired/revoked tokens are pruned as a side effect
// so the cap reflects genuinely live sessions and stale rows don't
// permanently consume a user's quota.
func (m *MemorySessionManager) countActive(ctx context.Context, userID string) (int, error) {
	toks, err := m.store.ActiveTokens(ctx, userID)
	if err != nil {
		return 0, err
	}
	now := m.now()
	live := 0
	for _, tok := range toks {
		revoked, err := m.store.IsRevoked(ctx, tok)
		if err != nil {
			return 0, err
		}
		if revoked {
			continue
		}
		s, ok, err := m.store.Get(ctx, tok)
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		if !s.ExpiresAt.IsZero() && !now.Before(s.ExpiresAt) {
			_ = m.store.Delete(ctx, tok)
			continue
		}
		live++
	}
	return live, nil
}

// Revoke marks a single session token as revoked. The token is added
// to the revocation list so the very next Validate for it fails, even
// if the underlying session row is later evicted on expiry.
func (m *MemorySessionManager) Revoke(token string) {
	if token == "" {
		return
	}
	ctx := context.Background()
	_ = m.store.Revoke(ctx, token)
}

// RevokeUser revokes every currently-held token for userID. Used for
// "log out everywhere" / forced sign-out (e.g. password change,
// account compromise). Tokens minted after this call are unaffected.
func (m *MemorySessionManager) RevokeUser(userID string) error {
	ctx := context.Background()
	toks, err := m.store.ActiveTokens(ctx, userID)
	if err != nil {
		return err
	}
	for _, tok := range toks {
		if err := m.store.Revoke(ctx, tok); err != nil {
			return err
		}
	}
	return nil
}

// Validate looks up the token, checks the revocation list FIRST (so a
// revoked token is rejected on the next request regardless of TTL),
// then checks expiry, and returns the session info.
func (m *MemorySessionManager) Validate(ctx context.Context, token string) (SessionInfo, error) {
	if token == "" {
		return SessionInfo{}, unauthenticatedf("missing session token")
	}

	// Revocation is checked on EVERY request, before the session
	// lookup, and independent of whether the session row still
	// exists. This is the "immediate token revocation list" contract:
	// a revoked token must fail the next request even if it has not
	// yet expired and even if the session entry was lazily evicted.
	revoked, err := m.store.IsRevoked(ctx, token)
	if err != nil {
		return SessionInfo{}, err
	}
	if revoked {
		return SessionInfo{}, unauthenticatedf("session has been revoked")
	}

	entry, ok, err := m.store.Get(ctx, token)
	if err != nil {
		return SessionInfo{}, err
	}
	if !ok {
		return SessionInfo{}, unauthenticatedf("invalid session token")
	}
	if !entry.ExpiresAt.IsZero() && !m.now().Before(entry.ExpiresAt) {
		// Lazy-evict so the map doesn't grow unboundedly under churn.
		// The revocation entry (if any) is intentionally left intact
		// by Delete's contract.
		_ = m.store.Delete(ctx, token)
		return SessionInfo{}, unauthenticatedf("session has expired")
	}

	// Defensive copy of metadata so callers can't poison the store.
	var md map[string]any
	if entry.Metadata != nil {
		md = make(map[string]any, len(entry.Metadata))
		for k, v := range entry.Metadata {
			md[k] = v
		}
	}
	return SessionInfo{
		UserID:    entry.UserID,
		CreatedAt: entry.CreatedAt,
		ExpiresAt: entry.ExpiresAt,
		Metadata:  md,
	}, nil
}

// memorySessionStore is the default, in-process SessionStore. It is
// the only SessionStore that ships in this binary; the Redis seam is
// documented on SessionStore and tracked in
// docs/go-port/shared/auth-interceptor.md "Open questions / risks".
type memorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
	revoked  map[string]struct{}
}

func newMemorySessionStore() *memorySessionStore {
	return &memorySessionStore{
		sessions: make(map[string]Session),
		revoked:  make(map[string]struct{}),
	}
}

func (s *memorySessionStore) Put(_ context.Context, token string, sess Session) error {
	s.mu.Lock()
	s.sessions[token] = sess
	s.mu.Unlock()
	return nil
}

func (s *memorySessionStore) Get(_ context.Context, token string) (Session, bool, error) {
	s.mu.RLock()
	sess, ok := s.sessions[token]
	s.mu.RUnlock()
	return sess, ok, nil
}

func (s *memorySessionStore) Delete(_ context.Context, token string) error {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
	return nil
}

func (s *memorySessionStore) ActiveTokens(_ context.Context, userID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []string
	for tok, sess := range s.sessions {
		if sess.UserID == userID {
			out = append(out, tok)
		}
	}
	return out, nil
}

func (s *memorySessionStore) Revoke(_ context.Context, token string) error {
	s.mu.Lock()
	s.revoked[token] = struct{}{}
	s.mu.Unlock()
	return nil
}

func (s *memorySessionStore) IsRevoked(_ context.Context, token string) (bool, error) {
	s.mu.RLock()
	_, ok := s.revoked[token]
	s.mu.RUnlock()
	return ok, nil
}

func randomToken(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
