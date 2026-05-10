// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"sync"
	"time"
)

// APIKeyManager validates an opaque API key (the value of the
// x-api-key header) and returns the key's metadata. The interceptor
// calls Validate once per request when the API-key header is present.
type APIKeyManager interface {
	Validate(ctx context.Context, key string) (APIKeyInfo, error)
}

// APIKeyInfo is the metadata surfaced for a successfully validated API
// key. The Name is what the interceptor uses as the Identity.Subject;
// the Scopes flow through to AuthContext.Scopes for the quota
// interceptor and any future scope-gated RPC.
//
// Mirrors the dict returned by ApiKeyManager.validate_key in
// server/python/entdb_server/auth/api_key_manager.py:124-158.
type APIKeyInfo struct {
	KeyID  string
	Name   string
	Scopes []string
}

// MemoryAPIKeyManager is the in-memory APIKeyManager used by tests and
// the no-auth dev server. Keys are stored as SHA-256 hashes -- no
// plaintext secret is retained -- to match the Python implementation's
// disk-write surface. Validation uses subtle.ConstantTimeCompare on the
// hash to avoid timing leaks (TODO carried over from
// api_key_manager.py:23).
type MemoryAPIKeyManager struct {
	mu      sync.RWMutex
	byHash  map[string]*apiKeyEntry
	byKeyID map[string]*apiKeyEntry
}

type apiKeyEntry struct {
	keyID     string
	name      string
	scopes    []string
	hash      []byte
	expiresAt time.Time // zero means "never"
	revoked   bool
}

// NewMemoryAPIKeyManager returns an empty in-memory key store. Add keys
// with Add.
func NewMemoryAPIKeyManager() *MemoryAPIKeyManager {
	return &MemoryAPIKeyManager{
		byHash:  make(map[string]*apiKeyEntry),
		byKeyID: make(map[string]*apiKeyEntry),
	}
}

// Add registers a plaintext key. The plaintext is hashed and dropped;
// callers retain it themselves (or print it once and forget it, like
// the real key-create flow). The returned key_id matches the
// "ek_<hex>" format the Python store hands out and lets callers
// Revoke later without remembering the plaintext.
//
// keyID is generated from the hash so the same plaintext is idempotent
// across calls; the test surface doesn't need a separate ID generator.
func (m *MemoryAPIKeyManager) Add(plaintext, name string, scopes []string) string {
	hash := sha256Sum(plaintext)
	keyID := "ek_" + hex.EncodeToString(hash[:8])
	entry := &apiKeyEntry{
		keyID:  keyID,
		name:   name,
		scopes: append([]string(nil), scopes...),
		hash:   hash,
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byHash[string(hash)] = entry
	m.byKeyID[keyID] = entry
	return keyID
}

// Revoke marks a key as revoked. Subsequent Validate calls return an
// UNAUTHENTICATED error.
func (m *MemoryAPIKeyManager) Revoke(keyID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.byKeyID[keyID]; ok {
		e.revoked = true
		// Drop from the hash index so Validate fails fast with a
		// constant-time miss, matching api_key_manager.py:222.
		delete(m.byHash, string(e.hash))
	}
}

// Validate looks up the plaintext key, checks revocation and expiry,
// and returns the key info. The lookup itself is a hash-table read;
// the constant-time compare here is defence-in-depth -- map lookup is
// already O(1) but does allocate, so the timing channel is small.
func (m *MemoryAPIKeyManager) Validate(_ context.Context, plaintext string) (APIKeyInfo, error) {
	if plaintext == "" {
		return APIKeyInfo{}, unauthenticatedf("missing API key")
	}
	hash := sha256Sum(plaintext)
	m.mu.RLock()
	entry, ok := m.byHash[string(hash)]
	m.mu.RUnlock()
	if !ok || entry == nil {
		return APIKeyInfo{}, unauthenticatedf("invalid API key")
	}
	// Belt-and-suspenders constant-time compare against the stored
	// hash. If the map index hash collides (it won't with SHA-256 on
	// the universe of inputs we see, but the assertion is cheap) this
	// catches it.
	if subtle.ConstantTimeCompare(entry.hash, hash) != 1 {
		return APIKeyInfo{}, unauthenticatedf("invalid API key")
	}
	if entry.revoked {
		return APIKeyInfo{}, unauthenticatedf("API key has been revoked")
	}
	if !entry.expiresAt.IsZero() && !time.Now().Before(entry.expiresAt) {
		return APIKeyInfo{}, unauthenticatedf("API key has expired")
	}
	return APIKeyInfo{
		KeyID:  entry.keyID,
		Name:   entry.name,
		Scopes: append([]string(nil), entry.scopes...),
	}, nil
}

func sha256Sum(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}
