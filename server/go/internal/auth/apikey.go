// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
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
type APIKeyInfo struct {
	KeyID  string
	Name   string
	Scopes []string
}

// argon2id parameters. These are deliberately constants rather than
// tunables: the encoded PHC string carries the parameters that were used
// for each stored hash, so Verify always uses the per-hash parameters
// and we can raise these defaults later without invalidating old keys.
//
// 64 MiB / 1 pass / parallelism=4 follows the OWASP "first recommended"
// argon2id configuration. API-key validation is on the request hot path,
// so we keep time=1 and lean on memory hardness; a single Validate is
// ~2-4ms on commodity hardware which is acceptable for a credential that
// is presented once per RPC (and typically behind a connection that
// reuses the same key).
const (
	argon2Time    uint32 = 1
	argon2Memory  uint32 = 64 * 1024 // KiB => 64 MiB
	argon2KeyLen  uint32 = 32
	argon2SaltLen        = 16
)

func argon2Threads() uint8 {
	if n := runtime.GOMAXPROCS(0); n > 0 && n < 255 {
		return uint8(n)
	}
	return 4
}

// hashAPIKey returns a PHC-format argon2id string for the plaintext key
// using a fresh random salt. Format:
//
//	$argon2id$v=19$m=65536,t=1,p=4$<b64salt>$<b64hash>
//
// The parameters are embedded so VerifyAPIKeyHash can reproduce the
// derivation even if the package defaults change later.
func hashAPIKey(plaintext string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: generate argon2 salt: %w", err)
	}
	threads := argon2Threads()
	sum := argon2.IDKey([]byte(plaintext), salt, argon2Time, argon2Memory, threads, argon2KeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argon2Memory, argon2Time, threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	), nil
}

// errBadHashFormat marks a stored hash that does not parse. It is never
// surfaced to a client (always remapped to a generic UNAUTHENTICATED) so
// a corrupt row reads as "invalid key", not as an oracle.
var errBadHashFormat = errors.New("auth: malformed argon2 hash")

// VerifyAPIKeyHash reports whether plaintext hashes to encoded. The
// comparison of the derived key against the stored key is
// constant-time (subtle.ConstantTimeCompare); argon2 itself is
// data-independent so the derivation does not leak the secret through
// timing.
func VerifyAPIKeyHash(plaintext, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// "" / "argon2id" / "v=19" / "m=..,t=..,p=.." / salt / hash
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, errBadHashFormat
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errBadHashFormat
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, errBadHashFormat
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, errBadHashFormat
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, errBadHashFormat
	}
	got := argon2.IDKey([]byte(plaintext), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// keyIDFor derives the public, non-secret key identifier. It is a
// truncated argon2id digest under a fixed all-zero salt so the same
// plaintext is idempotent across Add calls (matching the historical
// "ek_<hex>" contract) while never exposing a reversible function of the
// secret -- the truncation plus a separate salt domain from the
// verification hash means the key_id is not a verification oracle.
func keyIDFor(plaintext string) string {
	var idSalt [argon2SaltLen]byte // fixed zero salt: this is an identifier, not a credential
	sum := argon2.IDKey([]byte("keyid:"+plaintext), idSalt[:], 1, 16*1024, 1, 8)
	return "ek_" + base64.RawURLEncoding.EncodeToString(sum)
}

// MemoryAPIKeyManager is the in-memory APIKeyManager used by tests and
// the no-auth dev server. Keys are stored as argon2id PHC strings -- no
// plaintext secret is retained. Validation derives the argon2id digest
// of the presented key and compares it constant-time against the stored
// digest.
//
// It supports the same rotation surface as the persistent manager
// (multiple simultaneously-active keys, Revoke without the plaintext) so
// tests can exercise the migration window without a database.
type MemoryAPIKeyManager struct {
	mu      sync.RWMutex
	byKeyID map[string]*apiKeyEntry

	// Now lets tests pin a synthetic clock. nil -> time.Now.
	Now func() time.Time
}

type apiKeyEntry struct {
	keyID      string
	name       string
	scopes     []string
	encodedHsh string
	expiresAt  time.Time // zero means "never"
	revoked    bool
}

// NewMemoryAPIKeyManager returns an empty in-memory key store. Add keys
// with Add.
func NewMemoryAPIKeyManager() *MemoryAPIKeyManager {
	return &MemoryAPIKeyManager{
		byKeyID: make(map[string]*apiKeyEntry),
	}
}

func (m *MemoryAPIKeyManager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

// Add registers a plaintext key with no expiry. The plaintext is hashed
// (argon2id) and dropped; callers retain it themselves (or print it once
// and forget it, like the real key-create flow). The returned key_id is
// stable for the same plaintext and lets callers Revoke later without
// remembering the secret.
func (m *MemoryAPIKeyManager) Add(plaintext, name string, scopes []string) string {
	id, _ := m.AddWithExpiry(plaintext, name, scopes, time.Time{})
	return id
}

// AddWithExpiry is Add with an absolute expiry (zero == never). Returns
// the key_id and any hashing error. Used by the rotation tests to mint
// already-expired keys.
func (m *MemoryAPIKeyManager) AddWithExpiry(plaintext, name string, scopes []string, expiresAt time.Time) (string, error) {
	encoded, err := hashAPIKey(plaintext)
	if err != nil {
		return "", err
	}
	keyID := keyIDFor(plaintext)
	entry := &apiKeyEntry{
		keyID:      keyID,
		name:       name,
		scopes:     append([]string(nil), scopes...),
		encodedHsh: encoded,
		expiresAt:  expiresAt,
	}
	m.mu.Lock()
	m.byKeyID[keyID] = entry
	m.mu.Unlock()
	return keyID, nil
}

// Revoke marks a key as revoked. Subsequent Validate calls return an
// UNAUTHENTICATED error. Unknown key_id is a no-op.
func (m *MemoryAPIKeyManager) Revoke(keyID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.byKeyID[keyID]; ok {
		e.revoked = true
	}
}

// Validate derives the argon2id digest of the presented key and matches
// it against every stored entry. The match is O(n) in the number of
// active keys because argon2 hashes are salted per key (the historical
// SHA-256 hash-table lookup is not possible with a per-key salt); n is
// the count of keys in a single rotation window, which is small by
// construction.
func (m *MemoryAPIKeyManager) Validate(_ context.Context, plaintext string) (APIKeyInfo, error) {
	if plaintext == "" {
		return APIKeyInfo{}, unauthenticatedf("missing API key")
	}
	m.mu.RLock()
	entries := make([]*apiKeyEntry, 0, len(m.byKeyID))
	for _, e := range m.byKeyID {
		entries = append(entries, e)
	}
	m.mu.RUnlock()

	for _, e := range entries {
		ok, err := VerifyAPIKeyHash(plaintext, e.encodedHsh)
		if err != nil || !ok {
			continue
		}
		if e.revoked {
			return APIKeyInfo{}, unauthenticatedf("API key has been revoked")
		}
		if !e.expiresAt.IsZero() && !m.now().Before(e.expiresAt) {
			return APIKeyInfo{}, unauthenticatedf("API key has expired")
		}
		return APIKeyInfo{
			KeyID:  e.keyID,
			Name:   e.name,
			Scopes: append([]string(nil), e.scopes...),
		}, nil
	}
	return APIKeyInfo{}, unauthenticatedf("invalid API key")
}
