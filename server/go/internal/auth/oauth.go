// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// OAuthValidator validates an OAuth/OIDC bearer JWT and returns the
// decoded claims. The interceptor calls Validate exactly once per
// request, with the value of the Authorization header after the leading
// "Bearer " has been stripped.
//
// Implementations must:
//
//   - Verify signature, expiry, issuer, and audience.
//   - Return an error wrapping codes.Unauthenticated on any failure.
//
// Today only the in-memory implementation below is shipped. Real JWKS
// rotation, network discovery, and ES256 are tracked separately -- see
// docs/go-port/shared/auth-interceptor.md "Open questions / risks"
// items "JWKS rotation thundering herd" and "sub collisions across
// providers".
type OAuthValidator interface {
	Validate(ctx context.Context, token string) (map[string]any, error)
}

// JWKKey is one entry in an in-memory JWKS used by the in-memory
// validator. Exactly one of HS256Secret or RS256Public is populated; the
// validator picks the algorithm based on the JWT header.
//
// This is deliberately simpler than the JWK spec -- no kty, no crv, no
// PEM parsing -- because the in-memory validator is only used by tests
// and dev-mode servers. Production OAuth (real JWKS fetched over the
// network) is Phase 2.
type JWKKey struct {
	// Alg is "HS256" or "RS256". ES256 is tracked separately for a
	// follow-up.
	Alg string
	// HS256Secret is the shared secret for HS256 verification.
	// Populated iff Alg == "HS256".
	HS256Secret []byte
	// RS256Public is the RSA public key for RS256 verification.
	// Populated iff Alg == "RS256".
	RS256Public *rsa.PublicKey
}

// MemoryOAuthValidator is the in-memory OAuthValidator used by tests
// and the no-auth dev server. It accepts a fixed issuer/audience pair
// and a static map of kid -> JWKKey. Concurrent reads are safe; writes
// (AddKey) take a write lock.
//
// It supports HS256 and RS256. ES256 is deferred to Phase 2 with the
// rest of the production OAuth surface -- see
// docs/go-port/shared/auth-interceptor.md "Open questions / risks".
type MemoryOAuthValidator struct {
	issuer   string
	audience string

	mu   sync.RWMutex
	keys map[string]JWKKey

	// Now lets tests pin a synthetic clock. nil -> time.Now.
	Now func() time.Time
}

// NewMemoryOAuthValidator returns an in-memory validator pinned to the
// given issuer and audience. Add keys with AddKey before validating.
func NewMemoryOAuthValidator(issuer, audience string) *MemoryOAuthValidator {
	return &MemoryOAuthValidator{
		issuer:   issuer,
		audience: audience,
		keys:     make(map[string]JWKKey),
	}
}

// AddKey registers a verification key under a kid. The kid in the JWT
// header is used to pick the key, mirroring real JWKS lookup but
// without any of the network plumbing.
func (v *MemoryOAuthValidator) AddKey(kid string, key JWKKey) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.keys[kid] = key
}

func (v *MemoryOAuthValidator) now() time.Time {
	if v.Now != nil {
		return v.Now()
	}
	return time.Now()
}

// Validate parses, verifies, and decodes a compact-serialisation JWT.
// Returns the decoded claims on success. On any failure (malformed,
// unsupported algorithm, unknown kid, bad signature, expired, wrong
// issuer/audience) it returns an error that wraps
// codes.Unauthenticated.
//
// Mirrors OAuthValidator.validate_token in
func (v *MemoryOAuthValidator) Validate(_ context.Context, token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, unauthenticatedf("malformed JWT: expected 3 segments, got %d", len(parts))
	}
	headerB, err := decodeSegment(parts[0])
	if err != nil {
		return nil, unauthenticatedf("malformed JWT header: %v", err)
	}
	payloadB, err := decodeSegment(parts[1])
	if err != nil {
		return nil, unauthenticatedf("malformed JWT payload: %v", err)
	}
	sigB, err := decodeSegment(parts[2])
	if err != nil {
		return nil, unauthenticatedf("malformed JWT signature: %v", err)
	}

	var hdr struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerB, &hdr); err != nil {
		return nil, unauthenticatedf("malformed JWT header: %v", err)
	}
	if hdr.Kid == "" {
		return nil, unauthenticatedf("JWT missing 'kid' header")
	}
	if hdr.Alg != "HS256" && hdr.Alg != "RS256" {
		// Reject "none" explicitly along with everything else we
		// don't understand. ES256 is deferred to Phase 2.
		return nil, unauthenticatedf("unsupported JWT algorithm: %q", hdr.Alg)
	}

	v.mu.RLock()
	key, ok := v.keys[hdr.Kid]
	v.mu.RUnlock()
	if !ok {
		return nil, unauthenticatedf("no JWKS key found for kid %q", hdr.Kid)
	}
	if key.Alg != hdr.Alg {
		// Algorithm-confusion guard: if a key is registered as RS256
		// but the JWT claims HS256, reject. Otherwise an attacker
		// could sign a token with the RSA public bytes treated as an
		// HMAC secret.
		return nil, unauthenticatedf("algorithm mismatch: token=%s, key=%s", hdr.Alg, key.Alg)
	}

	signed := []byte(parts[0] + "." + parts[1])
	switch hdr.Alg {
	case "HS256":
		if !verifyHS256(key.HS256Secret, signed, sigB) {
			return nil, unauthenticatedf("invalid JWT signature")
		}
	case "RS256":
		if key.RS256Public == nil {
			return nil, unauthenticatedf("kid %q has no RS256 public key", hdr.Kid)
		}
		if err := verifyRS256(key.RS256Public, signed, sigB); err != nil {
			return nil, unauthenticatedf("invalid JWT signature: %v", err)
		}
	}

	var claims map[string]any
	if err := json.Unmarshal(payloadB, &claims); err != nil {
		return nil, unauthenticatedf("malformed JWT claims: %v", err)
	}

	now := v.now()
	if exp, ok := numericClaim(claims, "exp"); ok {
		if now.Unix() >= exp {
			return nil, unauthenticatedf("token has expired")
		}
	}
	if nbf, ok := numericClaim(claims, "nbf"); ok {
		if now.Unix() < nbf {
			return nil, unauthenticatedf("token not yet valid")
		}
	}
	if v.issuer != "" {
		iss, _ := claims["iss"].(string)
		if iss != v.issuer {
			return nil, unauthenticatedf("invalid issuer: got %q, want %q", iss, v.issuer)
		}
	}
	if v.audience != "" {
		if !audienceMatches(claims["aud"], v.audience) {
			return nil, unauthenticatedf("invalid audience: want %q", v.audience)
		}
	}

	return claims, nil
}

// audienceMatches accepts either a string aud or a list-of-strings aud,
// matching the OIDC spec and what jwt.decode does in Python.
func audienceMatches(raw any, want string) bool {
	switch v := raw.(type) {
	case string:
		return v == want
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}

// numericClaim extracts a numeric claim (exp, nbf, iat) as int64. JSON
// numbers come out of encoding/json as float64, so cast through that.
func numericClaim(claims map[string]any, name string) (int64, bool) {
	raw, ok := claims[name]
	if !ok {
		return 0, false
	}
	switch v := raw.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	default:
		return 0, false
	}
}

// decodeSegment decodes a base64url-without-padding JWT segment.
func decodeSegment(s string) ([]byte, error) {
	if pad := len(s) % 4; pad != 0 {
		s += strings.Repeat("=", 4-pad)
	}
	return base64.URLEncoding.DecodeString(s)
}

// verifyHS256 returns true iff sig is a valid HMAC-SHA256 of signed
// under the given secret. Uses hmac.Equal for constant-time comparison.
func verifyHS256(secret, signed, sig []byte) bool {
	mac := hmac.New(sha256.New, secret)
	mac.Write(signed)
	return hmac.Equal(mac.Sum(nil), sig)
}

// verifyRS256 verifies an RS256 (RSA-PKCS#1 v1.5 with SHA-256) signature.
func verifyRS256(pub *rsa.PublicKey, signed, sig []byte) error {
	h := sha256.Sum256(signed)
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, h[:], sig); err != nil {
		return errors.New("rsa verify failed")
	}
	return nil
}

// SignHS256ForTest is a tiny helper that produces a compact-serialisation
// HS256 JWT given the raw header and claims. It exists so the package's
// own unit tests can mint tokens without pulling in a third-party JWT
// library; production code MUST NOT call it.
//
// Exported with a "ForTest" suffix so misuse is loud at the call site.
// Returns the compact JWT string.
func SignHS256ForTest(secret []byte, kid string, claims map[string]any) (string, error) {
	hdr := map[string]any{"alg": "HS256", "kid": kid, "typ": "JWT"}
	hdrB, err := json.Marshal(hdr)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	payloadB, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	encHdr := base64.RawURLEncoding.EncodeToString(hdrB)
	encPayload := base64.RawURLEncoding.EncodeToString(payloadB)
	signed := encHdr + "." + encPayload
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signed))
	sig := mac.Sum(nil)
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// SignRS256ForTest mints an RS256 JWT for the package's own tests. As
// with SignHS256ForTest, production code MUST NOT call it.
func SignRS256ForTest(priv *rsa.PrivateKey, kid string, claims map[string]any) (string, error) {
	hdr := map[string]any{"alg": "RS256", "kid": kid, "typ": "JWT"}
	hdrB, err := json.Marshal(hdr)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	payloadB, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	encHdr := base64.RawURLEncoding.EncodeToString(hdrB)
	encPayload := base64.RawURLEncoding.EncodeToString(payloadB)
	signed := encHdr + "." + encPayload
	h := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(nil, priv, crypto.SHA256, h[:])
	if err != nil {
		return "", fmt.Errorf("sign rs256: %w", err)
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}
