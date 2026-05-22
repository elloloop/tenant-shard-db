// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
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
// Two implementations ship:
//
//   - MemoryOAuthValidator (this file) -- a static kid -> key map used by
//     tests and the no-auth dev server. HS256 / RS256 / ES256.
//   - JWKSValidator (jwks.go) -- production OIDC: real JWKS fetched over
//     the network with caching and key rotation, OIDC discovery, and
//     Google / Microsoft / Okta presets. RS256 / ES256.
//
// The algorithm-confusion guards (a JWKS RSA/EC key may never verify an
// HS* token, and vice versa) are enforced by both.
type OAuthValidator interface {
	Validate(ctx context.Context, token string) (map[string]any, error)
}

// JWKKey is one entry in an in-memory JWKS used by the in-memory
// validator. Exactly one of HS256Secret / RS256Public / ES256Public is
// populated; the validator picks the algorithm based on the JWT header.
//
// This is deliberately simpler than the JWK spec -- no kty, no crv, no
// PEM parsing -- because the in-memory validator is only used by tests
// and dev-mode servers. Production OAuth (real JWKS fetched over the
// network) lives in jwks.go.
type JWKKey struct {
	// Alg is "HS256", "RS256", or "ES256".
	Alg string
	// HS256Secret is the shared secret for HS256 verification.
	// Populated iff Alg == "HS256".
	HS256Secret []byte
	// RS256Public is the RSA public key for RS256 verification.
	// Populated iff Alg == "RS256".
	RS256Public *rsa.PublicKey
	// ES256Public is the ECDSA P-256 public key for ES256
	// verification. Populated iff Alg == "ES256".
	ES256Public *ecdsa.PublicKey
}

// MemoryOAuthValidator is the in-memory OAuthValidator used by tests
// and the no-auth dev server. It accepts a fixed issuer/audience pair
// and a static map of kid -> JWKKey. Concurrent reads are safe; writes
// (AddKey) take a write lock.
//
// It supports HS256, RS256, and ES256. Production OIDC (network JWKS,
// discovery, provider presets) lives in JWKSValidator (jwks.go).
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
func (v *MemoryOAuthValidator) Validate(_ context.Context, token string) (map[string]any, error) {
	hdr, claims, signed, sigB, err := parseJWT(token)
	if err != nil {
		return nil, err
	}
	if hdr.Kid == "" {
		return nil, unauthenticatedf("JWT missing 'kid' header")
	}
	if hdr.Alg != "HS256" && hdr.Alg != "RS256" && hdr.Alg != "ES256" {
		// Reject "none" explicitly along with everything else we
		// don't understand.
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
	case "ES256":
		if key.ES256Public == nil {
			return nil, unauthenticatedf("kid %q has no ES256 public key", hdr.Kid)
		}
		if err := verifyES256(key.ES256Public, signed, sigB); err != nil {
			return nil, unauthenticatedf("invalid JWT signature: %v", err)
		}
	}

	if err := verifyStandardClaims(claims, v.now(), v.issuer, v.audience); err != nil {
		return nil, err
	}
	return claims, nil
}

// jwtHeader is the decoded JOSE header. typ is parsed but unused -- some
// IdPs omit it and the spec makes it optional.
type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

// parseJWT splits a compact-serialisation JWT, base64url-decodes the
// three segments, and unmarshals the header + claims. It performs NO
// signature or claim verification -- callers do that. signed is the
// exact bytes the signature covers (header.payload).
func parseJWT(token string) (hdr jwtHeader, claims map[string]any, signed, sig []byte, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return hdr, nil, nil, nil, unauthenticatedf("malformed JWT: expected 3 segments, got %d", len(parts))
	}
	headerB, e := decodeSegment(parts[0])
	if e != nil {
		return hdr, nil, nil, nil, unauthenticatedf("malformed JWT header: %v", e)
	}
	payloadB, e := decodeSegment(parts[1])
	if e != nil {
		return hdr, nil, nil, nil, unauthenticatedf("malformed JWT payload: %v", e)
	}
	sigB, e := decodeSegment(parts[2])
	if e != nil {
		return hdr, nil, nil, nil, unauthenticatedf("malformed JWT signature: %v", e)
	}
	if e := json.Unmarshal(headerB, &hdr); e != nil {
		return hdr, nil, nil, nil, unauthenticatedf("malformed JWT header: %v", e)
	}
	if e := json.Unmarshal(payloadB, &claims); e != nil {
		return hdr, nil, nil, nil, unauthenticatedf("malformed JWT claims: %v", e)
	}
	return hdr, claims, []byte(parts[0] + "." + parts[1]), sigB, nil
}

// verifyStandardClaims enforces exp / nbf / iss / aud. Empty wantIssuer
// or wantAudience skips that check (dev mode). now is the validator's
// clock so tests can pin it.
func verifyStandardClaims(claims map[string]any, now time.Time, wantIssuer, wantAudience string) error {
	if exp, ok := numericClaim(claims, "exp"); ok {
		if now.Unix() >= exp {
			return unauthenticatedf("token has expired")
		}
	}
	if nbf, ok := numericClaim(claims, "nbf"); ok {
		if now.Unix() < nbf {
			return unauthenticatedf("token not yet valid")
		}
	}
	if wantIssuer != "" {
		iss, _ := claims["iss"].(string)
		if iss != wantIssuer {
			return unauthenticatedf("invalid issuer: got %q, want %q", iss, wantIssuer)
		}
	}
	if wantAudience != "" {
		if !audienceMatches(claims["aud"], wantAudience) {
			return unauthenticatedf("invalid audience: want %q", wantAudience)
		}
	}
	return nil
}

// audienceMatches accepts either a string aud or a list-of-strings aud,
// per the OIDC spec.
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

// verifyES256 verifies an ES256 (ECDSA on P-256 with SHA-256) signature.
//
// The JWS ES256 signature is the fixed-width R || S concatenation (RFC
// 7518 sec 3.4), each 32 bytes for P-256 -- NOT the ASN.1 DER form that
// ecdsa.VerifyASN1 expects. We split it and verify with ecdsa.Verify.
// A signature of the wrong length is rejected outright; this also slams
// the door on the curve-confusion family of attacks.
func verifyES256(pub *ecdsa.PublicKey, signed, sig []byte) error {
	if pub.Curve != elliptic.P256() {
		return errors.New("ES256 key is not on the P-256 curve")
	}
	const keyBytes = 32 // P-256 field size
	if len(sig) != 2*keyBytes {
		return fmt.Errorf("ES256 signature is %d bytes, want %d (R||S)", len(sig), 2*keyBytes)
	}
	r := new(big.Int).SetBytes(sig[:keyBytes])
	s := new(big.Int).SetBytes(sig[keyBytes:])
	h := sha256.Sum256(signed)
	if !ecdsa.Verify(pub, h[:], r, s) {
		return errors.New("ecdsa verify failed")
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

// SignES256ForTest mints an ES256 JWT (ECDSA P-256, SHA-256) with the
// JWS-mandated fixed-width R||S signature. Tests / dev only -- production
// code MUST NOT call it.
func SignES256ForTest(priv *ecdsa.PrivateKey, kid string, claims map[string]any) (string, error) {
	hdr := map[string]any{"alg": "ES256", "kid": kid, "typ": "JWT"}
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
	r, s, err := ecdsa.Sign(cryptorand.Reader, priv, h[:])
	if err != nil {
		return "", fmt.Errorf("sign es256: %w", err)
	}
	const keyBytes = 32
	sig := make([]byte, 2*keyBytes)
	r.FillBytes(sig[:keyBytes])
	s.FillBytes(sig[keyBytes:])
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}
