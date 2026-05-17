// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// JWKSValidator is the production OAuthValidator. It validates RS256 /
// ES256 OIDC bearer tokens against a JWKS document fetched over the
// network, with the document cached in-process and refreshed lazily on a
// kid miss (key rotation) or after a TTL.
//
// HS256 is intentionally NOT accepted here: a JWKS endpoint only ever
// publishes asymmetric public keys, so an HS256 token reaching this
// validator is the classic algorithm-confusion attack (signing a token
// with the RSA/EC public bytes treated as an HMAC secret). It is
// rejected outright, the same guard MemoryOAuthValidator enforces.
//
// Concurrency: the cached key set is guarded by a RWMutex; refreshes are
// coalesced through a single-flight latch keyed by JWKS URL so a burst
// of requests on an unknown kid triggers exactly one network fetch (the
// "thundering herd" risk called out in
// docs/go-port/shared/auth-interceptor.md).
type JWKSValidator struct {
	issuer   string
	audience string
	jwksURL  string

	httpClient *http.Client
	// cacheTTL bounds how long a fetched JWKS is trusted before a
	// proactive refresh; a kid miss forces a refresh regardless.
	cacheTTL time.Duration

	mu        sync.RWMutex
	keys      map[string]parsedJWK
	fetchedAt time.Time

	// flight serialises concurrent refreshes for this validator so a
	// kid miss does not fan out N network calls. Mirrors the
	// singleflight pattern without pulling in golang.org/x/sync.
	flightMu sync.Mutex
	flight   *refreshCall

	// Now lets tests pin a synthetic clock for exp/nbf and the cache
	// TTL. nil -> time.Now.
	Now func() time.Time
}

// parsedJWK is a verification key materialised from one JWKS entry. The
// alg is pinned at parse time so the verify path can enforce the
// algorithm-confusion guard (a token's header alg must match the key's
// declared alg, never cross RSA<->EC<->HMAC).
type parsedJWK struct {
	alg string
	rsa *rsa.PublicKey
	ec  *ecdsa.PublicKey
}

type refreshCall struct {
	done chan struct{}
	err  error
}

// JWKSOptions configures a JWKSValidator. Exactly one of JWKSURL or
// IssuerDiscovery must drive key discovery: pass JWKSURL to skip the
// .well-known round-trip, or leave it empty and let NewJWKSValidator run
// OIDC discovery against Issuer.
type JWKSOptions struct {
	// Issuer is the expected `iss` claim AND, when JWKSURL is empty,
	// the OIDC issuer whose .well-known/openid-configuration is
	// fetched to discover jwks_uri. Required.
	Issuer string
	// Audience is the expected `aud` claim. Empty disables the
	// audience check (dev only; production MUST set it).
	Audience string
	// JWKSURL, when set, is used verbatim and discovery is skipped.
	JWKSURL string
	// HTTPClient is used for discovery + JWKS fetches. nil -> a client
	// with a 10s timeout.
	HTTPClient *http.Client
	// CacheTTL bounds how long a fetched JWKS is trusted before a
	// proactive refresh. Zero -> 15 minutes. A kid miss always forces
	// an immediate refresh regardless of TTL.
	CacheTTL time.Duration
}

// oidcDiscovery is the subset of the OpenID Provider Metadata
// (RFC 8414 / OpenID Connect Discovery 1.0) we consume.
type oidcDiscovery struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

// NewJWKSValidator builds a production validator. If opts.JWKSURL is
// empty it performs OIDC discovery against opts.Issuer
// (<issuer>/.well-known/openid-configuration) to resolve jwks_uri, and
// verifies the discovered issuer matches (a discovery document that
// claims a different issuer is an attack and is rejected). It then warms
// the JWKS cache with one fetch so the first real request is fast and
// boot fails loudly on a misconfigured issuer.
func NewJWKSValidator(ctx context.Context, opts JWKSOptions) (*JWKSValidator, error) {
	if strings.TrimSpace(opts.Issuer) == "" {
		return nil, fmt.Errorf("jwks validator: Issuer is required")
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	ttl := opts.CacheTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}

	jwksURL := strings.TrimSpace(opts.JWKSURL)
	if jwksURL == "" {
		disc, err := discoverOIDC(ctx, httpClient, opts.Issuer)
		if err != nil {
			return nil, err
		}
		if disc.Issuer != "" && disc.Issuer != opts.Issuer {
			return nil, fmt.Errorf("oidc discovery: document issuer %q does not match configured issuer %q", disc.Issuer, opts.Issuer)
		}
		if disc.JWKSURI == "" {
			return nil, fmt.Errorf("oidc discovery: %s did not return a jwks_uri", opts.Issuer)
		}
		jwksURL = disc.JWKSURI
	}

	v := &JWKSValidator{
		issuer:     opts.Issuer,
		audience:   opts.Audience,
		jwksURL:    jwksURL,
		httpClient: httpClient,
		cacheTTL:   ttl,
		keys:       map[string]parsedJWK{},
	}
	// Warm the cache so a bad issuer/JWKS URL fails at boot, not on the
	// first request.
	if err := v.refresh(ctx); err != nil {
		return nil, fmt.Errorf("jwks validator: initial JWKS fetch: %w", err)
	}
	return v, nil
}

func (v *JWKSValidator) now() time.Time {
	if v.Now != nil {
		return v.Now()
	}
	return time.Now()
}

// JWKSURL returns the resolved JWKS endpoint (post-discovery). Useful
// for boot logging.
func (v *JWKSValidator) JWKSURL() string { return v.jwksURL }

// Validate implements OAuthValidator. It parses the JWT, resolves the
// kid against the cached JWKS (refreshing once on a miss to pick up
// rotated keys), verifies the signature with the algorithm pinned by the
// JWKS entry, and enforces exp / nbf / iss / aud.
func (v *JWKSValidator) Validate(ctx context.Context, token string) (map[string]any, error) {
	hdr, claims, signed, sigB, err := parseJWT(token)
	if err != nil {
		return nil, err
	}
	if hdr.Kid == "" {
		return nil, unauthenticatedf("JWT missing 'kid' header")
	}
	switch hdr.Alg {
	case "RS256", "ES256":
		// asymmetric -- a JWKS can verify these.
	default:
		// HS256 / none / anything else. An HS256 token at a JWKS
		// endpoint is the algorithm-confusion attack.
		return nil, unauthenticatedf("unsupported JWT algorithm for JWKS validation: %q", hdr.Alg)
	}

	key, ok := v.lookupKey(hdr.Kid)
	if !ok {
		// Unknown kid: the IdP may have rotated signing keys. Force a
		// single coalesced refresh and retry once.
		if err := v.refreshOnce(ctx); err != nil {
			return nil, unauthenticatedf("no JWKS key for kid %q and refresh failed: %v", hdr.Kid, err)
		}
		key, ok = v.lookupKey(hdr.Kid)
		if !ok {
			return nil, unauthenticatedf("no JWKS key found for kid %q", hdr.Kid)
		}
	}
	if key.alg != hdr.Alg {
		// Algorithm-confusion guard: the JWKS entry pins the alg; a
		// token claiming a different alg for the same kid is rejected.
		return nil, unauthenticatedf("algorithm mismatch: token=%s, key=%s", hdr.Alg, key.alg)
	}

	switch hdr.Alg {
	case "RS256":
		if key.rsa == nil {
			return nil, unauthenticatedf("kid %q is not an RSA key", hdr.Kid)
		}
		if err := verifyRS256(key.rsa, signed, sigB); err != nil {
			return nil, unauthenticatedf("invalid JWT signature: %v", err)
		}
	case "ES256":
		if key.ec == nil {
			return nil, unauthenticatedf("kid %q is not an EC key", hdr.Kid)
		}
		if err := verifyES256(key.ec, signed, sigB); err != nil {
			return nil, unauthenticatedf("invalid JWT signature: %v", err)
		}
	}

	if err := verifyStandardClaims(claims, v.now(), v.issuer, v.audience); err != nil {
		return nil, err
	}
	return claims, nil
}

func (v *JWKSValidator) lookupKey(kid string) (parsedJWK, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	k, ok := v.keys[kid]
	return k, ok
}

// refreshOnce coalesces concurrent refreshes: the first caller performs
// the fetch, every other caller that arrives while it is in flight waits
// on the same result instead of issuing its own network call. This is
// the singleflight behaviour the spec calls for, implemented with a
// latch so we don't add a dependency.
func (v *JWKSValidator) refreshOnce(ctx context.Context) error {
	v.flightMu.Lock()
	if v.flight != nil {
		call := v.flight
		v.flightMu.Unlock()
		select {
		case <-call.done:
			return call.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	call := &refreshCall{done: make(chan struct{})}
	v.flight = call
	v.flightMu.Unlock()

	call.err = v.refresh(ctx)

	v.flightMu.Lock()
	v.flight = nil
	v.flightMu.Unlock()
	close(call.done)
	return call.err
}

// refresh fetches the JWKS document and atomically swaps the cached key
// set. A fetch that yields zero usable keys is treated as an error so we
// don't blow away a good cache with an empty one.
func (v *JWKSValidator) refresh(ctx context.Context) error {
	keys, err := fetchJWKS(ctx, v.httpClient, v.jwksURL)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return fmt.Errorf("jwks: %s returned no usable keys", v.jwksURL)
	}
	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = v.now()
	v.mu.Unlock()
	return nil
}

// MaybeRefresh proactively refreshes if the cache is older than the TTL.
// The interceptor does not call this on the hot path -- the kid-miss
// refresh covers correctness; this exists for an optional background
// ticker an operator may wire up later. Kept un-wired for now.
func (v *JWKSValidator) MaybeRefresh(ctx context.Context) error {
	v.mu.RLock()
	stale := v.now().Sub(v.fetchedAt) >= v.cacheTTL
	v.mu.RUnlock()
	if !stale {
		return nil
	}
	return v.refreshOnce(ctx)
}

// discoverOIDC fetches <issuer>/.well-known/openid-configuration and
// returns the parsed metadata. The well-known path is appended to the
// issuer with exactly one slash, per OpenID Connect Discovery 1.0 sec 4.
func discoverOIDC(ctx context.Context, c *http.Client, issuer string) (oidcDiscovery, error) {
	url := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return oidcDiscovery{}, fmt.Errorf("oidc discovery: build request: %w", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return oidcDiscovery{}, fmt.Errorf("oidc discovery: GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return oidcDiscovery{}, fmt.Errorf("oidc discovery: GET %s: status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return oidcDiscovery{}, fmt.Errorf("oidc discovery: read body: %w", err)
	}
	var d oidcDiscovery
	if err := json.Unmarshal(body, &d); err != nil {
		return oidcDiscovery{}, fmt.Errorf("oidc discovery: decode %s: %w", url, err)
	}
	return d, nil
}

// jwksDoc / jwkEntry mirror the JWK Set structure (RFC 7517). Only the
// fields needed to materialise RSA / EC verification keys are decoded.
type jwksDoc struct {
	Keys []jwkEntry `json:"keys"`
}

type jwkEntry struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	// RSA
	N string `json:"n"`
	E string `json:"e"`
	// EC
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// fetchJWKS GETs the JWKS document and materialises every RSA/EC signing
// key into a kid-indexed map. Entries that are malformed or of an
// unsupported type are skipped (an IdP may publish encryption keys or
// future algorithms alongside the signing keys we use); the caller
// errors only if NOTHING usable came back.
func fetchJWKS(ctx context.Context, c *http.Client, jwksURL string) (map[string]parsedJWK, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("jwks: build request: %w", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jwks: GET %s: %w", jwksURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks: GET %s: status %d", jwksURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("jwks: read body: %w", err)
	}
	var doc jwksDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("jwks: decode %s: %w", jwksURL, err)
	}

	out := make(map[string]parsedJWK, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kid == "" {
			continue // we route by kid; an entry without one is unusable
		}
		// Skip explicit encryption keys; we only verify signatures.
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		switch k.Kty {
		case "RSA":
			pub, err := parseRSAJWK(k)
			if err != nil {
				continue
			}
			alg := k.Alg
			if alg == "" {
				alg = "RS256"
			}
			if alg != "RS256" {
				continue // only RS256 supported
			}
			out[k.Kid] = parsedJWK{alg: alg, rsa: pub}
		case "EC":
			pub, err := parseECJWK(k)
			if err != nil {
				continue
			}
			alg := k.Alg
			if alg == "" {
				alg = "ES256"
			}
			if alg != "ES256" {
				continue // only ES256 (P-256) supported
			}
			out[k.Kid] = parsedJWK{alg: alg, ec: pub}
		default:
			continue
		}
	}
	return out, nil
}

func parseRSAJWK(k jwkEntry) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("rsa jwk: bad modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("rsa jwk: bad exponent: %w", err)
	}
	if len(nBytes) == 0 || len(eBytes) == 0 {
		return nil, fmt.Errorf("rsa jwk: empty modulus or exponent")
	}
	e := new(big.Int).SetBytes(eBytes)
	if !e.IsInt64() || e.Int64() <= 0 {
		return nil, fmt.Errorf("rsa jwk: implausible exponent")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(e.Int64()),
	}, nil
}

func parseECJWK(k jwkEntry) (*ecdsa.PublicKey, error) {
	if k.Crv != "P-256" {
		return nil, fmt.Errorf("ec jwk: unsupported curve %q (only P-256/ES256)", k.Crv)
	}
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("ec jwk: bad x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("ec jwk: bad y: %w", err)
	}
	pub := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}
	if !pub.Curve.IsOnCurve(pub.X, pub.Y) {
		return nil, fmt.Errorf("ec jwk: point is not on P-256")
	}
	return pub, nil
}

// OIDCProvider is a known IdP whose issuer + JWKS URL are well-known, so
// an operator can configure auth with a single -oauth-provider flag plus
// the audience (client id) instead of hand-copying URLs.
type OIDCProvider struct {
	// Issuer is the canonical `iss` value the IdP stamps into tokens.
	Issuer string
	// JWKSURL is the IdP's published JWKS endpoint. When set, the
	// validator skips OIDC discovery and uses it directly.
	JWKSURL string
}

// providerPresets maps a short name to a known IdP. Microsoft's issuer
// is tenant-specific (`https://login.microsoftonline.com/<tenant>/v2.0`)
// so it is intentionally NOT a static preset -- use -oauth-issuer with
// the concrete tenant and let discovery resolve the JWKS URL.
var providerPresets = map[string]OIDCProvider{
	"google": {
		Issuer:  "https://accounts.google.com",
		JWKSURL: "https://www.googleapis.com/oauth2/v3/certs",
	},
	"okta": {
		// Okta is org-specific; the issuer must be supplied by the
		// operator. The preset only records that discovery is the
		// resolution path (no static JWKS URL).
	},
	"microsoft": {
		// Tenant-specific issuer; resolved via discovery from
		// -oauth-issuer. No static JWKS URL.
	},
}

// PresetProvider returns the preset for a known provider name
// ("google", "microsoft", "okta"). The bool is false for an unknown
// name. A returned preset with an empty Issuer means "discovery only --
// the operator must supply -oauth-issuer" (Okta/Microsoft are
// org/tenant-specific).
func PresetProvider(name string) (OIDCProvider, bool) {
	p, ok := providerPresets[strings.ToLower(strings.TrimSpace(name))]
	return p, ok
}

// KnownProviders lists the recognised -oauth-provider preset names.
func KnownProviders() []string {
	return []string{"google", "microsoft", "okta"}
}
