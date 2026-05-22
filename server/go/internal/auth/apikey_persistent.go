// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"fmt"
	"time"
)

// StoredAPIKey is the auth-package view of one persisted API-key row.
// It is decoupled from globalstore.APIKeyRecord so this package does not
// import globalstore (keeps the dependency edge auth <- callers, never
// auth -> globalstore) and so the manager is unit-testable with a fake
// store.
type StoredAPIKey struct {
	KeyID     string
	Name      string
	Hash      string // argon2id PHC string
	Scopes    []string
	ExpiresAt int64 // Unix seconds; 0 == never
}

// APIKeyStore is the persistence contract the PersistentAPIKeyManager
// depends on. *globalstore.GlobalStore satisfies it structurally via an
// adapter (see GlobalStoreAPIKeys); tests use an in-memory fake.
//
// PutAPIKey/RevokeAPIKey carry the rotation surface: issue a new row,
// flip clients over, then revoke the old key_id. ListActiveAPIKeys is
// the validation hot path -- it returns every key that is status=active
// AND unexpired, which is exactly the set that must all stay acceptable
// during a rotation migration window.
type APIKeyStore interface {
	PutAPIKey(ctx context.Context, k StoredAPIKey) error
	RevokeAPIKey(ctx context.Context, keyID string) (bool, error)
	ListActiveAPIKeys(ctx context.Context, now int64) ([]StoredAPIKey, error)
}

// PersistentAPIKeyManager is the production APIKeyManager: keys live in
// global.db, hashed with argon2id, scoped, revocable, and rotatable
// (multiple simultaneously-active keys for a documented migration
// window).
//
// Validate fetches the active set from the store and compares the
// presented secret against each entry's argon2id hash with a
// constant-time compare. The candidate set is naturally small (one
// rotation window's worth of keys); per-key salts make a hash-table
// lookup impossible by construction, so the linear scan is intentional
// and bounded.
//
// Concurrency: safe for concurrent use as long as the backing store is
// (globalstore pins MaxOpenConns(1), so it is).
type PersistentAPIKeyManager struct {
	store APIKeyStore

	// Now lets tests pin a synthetic clock. nil -> time.Now.
	Now func() time.Time
}

// NewPersistentAPIKeyManager wires a manager onto a durable store.
func NewPersistentAPIKeyManager(store APIKeyStore) *PersistentAPIKeyManager {
	return &PersistentAPIKeyManager{store: store}
}

func (m *PersistentAPIKeyManager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

// Issue mints a new key: it hashes the plaintext with argon2id, derives
// a stable key_id, and persists the row as status=active. The plaintext
// is the caller's responsibility to surface once and then forget --
// globalstore only ever stores the hash. expiresAt is an absolute
// Unix-second deadline; pass 0 for "never".
//
// Issue is the "create the new key" half of rotation: call it to mint
// the replacement, hand the new secret to the client, leave the old key
// active until the cutover completes, then Revoke the old key_id.
func (m *PersistentAPIKeyManager) Issue(ctx context.Context, plaintext, name string, scopes []string, expiresAt int64) (keyID string, err error) {
	if plaintext == "" {
		return "", fmt.Errorf("auth: Issue requires a non-empty plaintext key")
	}
	encoded, err := hashAPIKey(plaintext)
	if err != nil {
		return "", err
	}
	keyID = keyIDFor(plaintext)
	if err := m.store.PutAPIKey(ctx, StoredAPIKey{
		KeyID:     keyID,
		Name:      name,
		Hash:      encoded,
		Scopes:    append([]string(nil), scopes...),
		ExpiresAt: expiresAt,
	}); err != nil {
		return "", err
	}
	return keyID, nil
}

// Revoke retires a key by key_id. Returns true iff an active key was
// flipped to revoked. This is the "retire the old key" half of the
// rotation migration window; after this returns the old secret is dead
// while the new one (issued earlier) keeps working uninterrupted.
func (m *PersistentAPIKeyManager) Revoke(ctx context.Context, keyID string) (bool, error) {
	return m.store.RevokeAPIKey(ctx, keyID)
}

// Validate authenticates a presented API-key secret against the active
// set. Revoked or expired keys are never returned by the store query, so
// a hit here is by construction a live key. The argon2id verification is
// constant-time per candidate.
func (m *PersistentAPIKeyManager) Validate(ctx context.Context, plaintext string) (APIKeyInfo, error) {
	if plaintext == "" {
		return APIKeyInfo{}, unauthenticatedf("missing API key")
	}
	active, err := m.store.ListActiveAPIKeys(ctx, m.now().Unix())
	if err != nil {
		// A store failure must not leak as "invalid key" (that would be
		// an availability bug masquerading as auth). Surface it as an
		// internal-ish UNAUTHENTICATED with a distinct message; the
		// interceptor maps everything from this package to the gRPC
		// status, and we never want a transient DB blip to look like a
		// brute-force miss.
		return APIKeyInfo{}, unauthenticatedf("API key store unavailable: %v", err)
	}
	for _, k := range active {
		ok, verr := VerifyAPIKeyHash(plaintext, k.Hash)
		if verr != nil || !ok {
			continue
		}
		return APIKeyInfo{
			KeyID:  k.KeyID,
			Name:   k.Name,
			Scopes: append([]string(nil), k.Scopes...),
		}, nil
	}
	return APIKeyInfo{}, unauthenticatedf("invalid API key")
}
