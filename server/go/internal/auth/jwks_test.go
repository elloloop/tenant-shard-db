// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

// --- JWKS document helpers -------------------------------------------------

func rsaJWK(kid string, pub *rsa.PublicKey) jwkEntry {
	eBytes := big.NewInt(int64(pub.E)).Bytes()
	return jwkEntry{
		Kty: "RSA",
		Kid: kid,
		Alg: "RS256",
		Use: "sig",
		N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(eBytes),
	}
}

func ecJWK(kid string, pub *ecdsa.PublicKey) jwkEntry {
	return jwkEntry{
		Kty: "EC",
		Kid: kid,
		Alg: "ES256",
		Use: "sig",
		Crv: "P-256",
		X:   base64.RawURLEncoding.EncodeToString(pub.X.Bytes()),
		Y:   base64.RawURLEncoding.EncodeToString(pub.Y.Bytes()),
	}
}

func jwksJSON(t *testing.T, keys ...jwkEntry) []byte {
	t.Helper()
	b, err := json.Marshal(jwksDoc{Keys: keys})
	if err != nil {
		t.Fatalf("marshal jwks: %v", err)
	}
	return b
}

// --- Tests -----------------------------------------------------------------

func TestJWKSValidator_RS256_Valid(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&hits, 1)
		_, _ = w.Write(jwksJSON(t, rsaJWK("kid-rsa", &priv.PublicKey)))
	}))
	defer srv.Close()

	v, err := NewJWKSValidator(context.Background(), JWKSOptions{
		Issuer:   "https://issuer.test",
		Audience: "entdb",
		JWKSURL:  srv.URL,
	})
	if err != nil {
		t.Fatalf("NewJWKSValidator: %v", err)
	}
	tok, _ := SignRS256ForTest(priv, "kid-rsa", map[string]any{
		"sub": "alice",
		"iss": "https://issuer.test",
		"aud": "entdb",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	claims, err := v.Validate(context.Background(), tok)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims["sub"] != "alice" {
		t.Errorf("sub = %v, want alice", claims["sub"])
	}
	// One fetch at construction; cached afterward.
	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Errorf("JWKS fetched %d times, want 1 (cache miss)", got)
	}
}

func TestJWKSValidator_ES256_Valid(t *testing.T) {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(jwksJSON(t, ecJWK("kid-ec", &priv.PublicKey)))
	}))
	defer srv.Close()

	v, err := NewJWKSValidator(context.Background(), JWKSOptions{
		Issuer:  "https://issuer.test",
		JWKSURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("NewJWKSValidator: %v", err)
	}
	tok, _ := SignES256ForTest(priv, "kid-ec", map[string]any{
		"sub": "bob",
		"iss": "https://issuer.test",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	claims, err := v.Validate(context.Background(), tok)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims["sub"] != "bob" {
		t.Errorf("sub = %v, want bob", claims["sub"])
	}
}

func TestJWKSValidator_CachesAcrossRequests(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&hits, 1)
		_, _ = w.Write(jwksJSON(t, rsaJWK("k1", &priv.PublicKey)))
	}))
	defer srv.Close()

	v, err := NewJWKSValidator(context.Background(), JWKSOptions{
		Issuer:  "https://issuer.test",
		JWKSURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("NewJWKSValidator: %v", err)
	}
	for i := 0; i < 20; i++ {
		tok, _ := SignRS256ForTest(priv, "k1", map[string]any{
			"sub": "alice",
			"iss": "https://issuer.test",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		if _, err := v.Validate(context.Background(), tok); err != nil {
			t.Fatalf("Validate #%d: %v", i, err)
		}
	}
	// 20 validations, known kid each time -> still just the warm-up
	// fetch.
	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Errorf("JWKS fetched %d times, want 1 (cached)", got)
	}
}

func TestJWKSValidator_KeyRotationRefetchesOnKidMiss(t *testing.T) {
	oldKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	newKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	var rotated atomic.Bool
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&hits, 1)
		if rotated.Load() {
			// After rotation the IdP serves the new kid only.
			_, _ = w.Write(jwksJSON(t, rsaJWK("kid-new", &newKey.PublicKey)))
			return
		}
		_, _ = w.Write(jwksJSON(t, rsaJWK("kid-old", &oldKey.PublicKey)))
	}))
	defer srv.Close()

	v, err := NewJWKSValidator(context.Background(), JWKSOptions{
		Issuer:  "https://issuer.test",
		JWKSURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("NewJWKSValidator: %v", err)
	}

	// Old key validates with the warm cache (1 fetch so far).
	oldTok, _ := SignRS256ForTest(oldKey, "kid-old", map[string]any{
		"sub": "alice", "iss": "https://issuer.test",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if _, err := v.Validate(context.Background(), oldTok); err != nil {
		t.Fatalf("pre-rotation Validate: %v", err)
	}

	// IdP rotates. A token signed by the new key has an unknown kid;
	// the validator must refetch and then succeed.
	rotated.Store(true)
	newTok, _ := SignRS256ForTest(newKey, "kid-new", map[string]any{
		"sub": "alice", "iss": "https://issuer.test",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if _, err := v.Validate(context.Background(), newTok); err != nil {
		t.Fatalf("post-rotation Validate (should refetch): %v", err)
	}
	if got := atomic.LoadInt64(&hits); got != 2 {
		t.Errorf("JWKS fetched %d times, want 2 (warm-up + rotation refetch)", got)
	}

	// The refreshed cache still serves the new kid without another
	// fetch.
	if _, err := v.Validate(context.Background(), newTok); err != nil {
		t.Fatalf("second post-rotation Validate: %v", err)
	}
	if got := atomic.LoadInt64(&hits); got != 2 {
		t.Errorf("JWKS fetched %d times after re-validate, want 2 (cached)", got)
	}
}

func TestJWKSValidator_KidMissRefreshIsCoalesced(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	var hits int64
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt64(&hits, 1)
		if n >= 2 {
			// Block concurrent refetches so they pile up on the
			// singleflight latch instead of racing past it.
			<-release
		}
		_, _ = w.Write(jwksJSON(t, rsaJWK("kid-1", &key.PublicKey)))
	}))
	defer srv.Close()

	v, err := NewJWKSValidator(context.Background(), JWKSOptions{
		Issuer:  "https://issuer.test",
		JWKSURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("NewJWKSValidator: %v", err)
	}

	// Unknown kid -> every concurrent caller triggers refreshOnce; only
	// one network call should be in flight.
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok, _ := SignRS256ForTest(key, "ghost-kid", map[string]any{
				"sub": "x", "iss": "https://issuer.test",
				"exp": time.Now().Add(time.Hour).Unix(),
			})
			_, _ = v.Validate(context.Background(), tok)
		}()
	}
	// Give the goroutines time to converge on the latch, then release.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	// Warm-up fetch (1) + at most one coalesced refetch for the burst.
	if got := atomic.LoadInt64(&hits); got > 2 {
		t.Errorf("JWKS fetched %d times, want <=2 (singleflight coalesced the burst)", got)
	}
}

func TestJWKSValidator_RejectsAlgConfusion_HS256AtJWKS(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(jwksJSON(t, rsaJWK("kid-1", &priv.PublicKey)))
	}))
	defer srv.Close()

	v, err := NewJWKSValidator(context.Background(), JWKSOptions{
		Issuer:  "https://issuer.test",
		JWKSURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("NewJWKSValidator: %v", err)
	}
	// Attacker forges an HS256 token for a kid that the JWKS publishes
	// as RSA. A naive verifier that fed the RSA public bytes to HMAC
	// would accept it; we must reject before any signature math.
	tok, _ := SignHS256ForTest([]byte("rsa-public-bytes-as-hmac-secret"), "kid-1", map[string]any{
		"sub": "attacker", "iss": "https://issuer.test",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	_, err = v.Validate(context.Background(), tok)
	if err == nil || errs.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected UNAUTHENTICATED alg-confusion rejection, got err=%v code=%v", err, errs.Code(err))
	}
}

func TestJWKSValidator_RejectsNoneAlg(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(jwksJSON(t, rsaJWK("kid-1", &priv.PublicKey)))
	}))
	defer srv.Close()

	v, err := NewJWKSValidator(context.Background(), JWKSOptions{
		Issuer:  "https://issuer.test",
		JWKSURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("NewJWKSValidator: %v", err)
	}
	// header {"alg":"none","kid":"kid-1"}, empty payload, empty sig.
	tok := "eyJhbGciOiJub25lIiwia2lkIjoia2lkLTEifQ.e30."
	if _, err := v.Validate(context.Background(), tok); err == nil {
		t.Fatal("expected rejection of alg=none at JWKS validator")
	}
}

func TestJWKSValidator_RejectsExpiredAndBadIssuer(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(jwksJSON(t, rsaJWK("k", &priv.PublicKey)))
	}))
	defer srv.Close()

	v, err := NewJWKSValidator(context.Background(), JWKSOptions{
		Issuer:   "https://right.test",
		Audience: "aud",
		JWKSURL:  srv.URL,
	})
	if err != nil {
		t.Fatalf("NewJWKSValidator: %v", err)
	}

	expired, _ := SignRS256ForTest(priv, "k", map[string]any{
		"sub": "alice", "iss": "https://right.test", "aud": "aud",
		"exp": time.Now().Add(-time.Minute).Unix(),
	})
	if _, err := v.Validate(context.Background(), expired); err == nil {
		t.Error("expected expired-token rejection")
	}

	wrongIss, _ := SignRS256ForTest(priv, "k", map[string]any{
		"sub": "alice", "iss": "https://evil.test", "aud": "aud",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if _, err := v.Validate(context.Background(), wrongIss); err == nil {
		t.Error("expected issuer rejection")
	}

	wrongAud, _ := SignRS256ForTest(priv, "k", map[string]any{
		"sub": "alice", "iss": "https://right.test", "aud": "other",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if _, err := v.Validate(context.Background(), wrongAud); err == nil {
		t.Error("expected audience rejection")
	}
}

func TestJWKSValidator_OIDCDiscovery(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	mux := http.NewServeMux()
	var jwksHits int64
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(oidcDiscovery{
			Issuer:  srv.URL, // discovery doc issuer must match configured issuer
			JWKSURI: srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&jwksHits, 1)
		_, _ = w.Write(jwksJSON(t, rsaJWK("disc-kid", &priv.PublicKey)))
	})

	v, err := NewJWKSValidator(context.Background(), JWKSOptions{
		Issuer: srv.URL, // no JWKSURL -> discovery resolves jwks_uri
	})
	if err != nil {
		t.Fatalf("NewJWKSValidator (discovery): %v", err)
	}
	if v.JWKSURL() != srv.URL+"/jwks" {
		t.Errorf("resolved JWKS URL = %q, want %q", v.JWKSURL(), srv.URL+"/jwks")
	}
	tok, _ := SignRS256ForTest(priv, "disc-kid", map[string]any{
		"sub": "alice", "iss": srv.URL,
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if _, err := v.Validate(context.Background(), tok); err != nil {
		t.Fatalf("Validate after discovery: %v", err)
	}
	if atomic.LoadInt64(&jwksHits) != 1 {
		t.Errorf("jwks endpoint hit %d times, want 1", jwksHits)
	}
}

func TestJWKSValidator_DiscoveryIssuerMismatchRejected(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		// A discovery doc that claims a different issuer than the one
		// we asked for is an attack (issuer substitution).
		_ = json.NewEncoder(w).Encode(oidcDiscovery{
			Issuer:  "https://attacker.example",
			JWKSURI: srv.URL + "/jwks",
		})
	})
	_, err := NewJWKSValidator(context.Background(), JWKSOptions{Issuer: srv.URL})
	if err == nil {
		t.Fatal("expected discovery issuer-mismatch rejection")
	}
}

func TestJWKSValidator_RequiresIssuer(t *testing.T) {
	if _, err := NewJWKSValidator(context.Background(), JWKSOptions{JWKSURL: "http://x"}); err == nil {
		t.Fatal("expected error when Issuer is empty")
	}
}

func TestJWKSValidator_BootFailsOnUnreachableJWKS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, err := NewJWKSValidator(context.Background(), JWKSOptions{
		Issuer:  "https://issuer.test",
		JWKSURL: srv.URL,
	})
	if err == nil {
		t.Fatal("expected boot failure on unreachable/erroring JWKS endpoint")
	}
}

func TestPresetProvider(t *testing.T) {
	g, ok := PresetProvider("google")
	if !ok {
		t.Fatal("google preset missing")
	}
	if g.Issuer != "https://accounts.google.com" || g.JWKSURL == "" {
		t.Errorf("google preset = %+v, want issuer+jwks set", g)
	}
	// Microsoft / Okta are tenant/org-specific: recognised, but no
	// static issuer (discovery from operator-supplied -oauth-issuer).
	ms, ok := PresetProvider("microsoft")
	if !ok || ms.Issuer != "" || ms.JWKSURL != "" {
		t.Errorf("microsoft preset = %+v, want recognised-but-discovery-only", ms)
	}
	if _, ok := PresetProvider("not-a-provider"); ok {
		t.Error("unknown provider should not resolve")
	}
	if len(KnownProviders()) != 3 {
		t.Errorf("KnownProviders() = %v, want 3", KnownProviders())
	}
}

// sanity: the helper-built JWKS round-trips through the real parser.
func TestParseJWK_RoundTrip(t *testing.T) {
	rsaPriv, _ := rsa.GenerateKey(rand.Reader, 2048)
	ecPriv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	keys, err := func() (map[string]parsedJWK, error) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(jwksJSON(t,
				rsaJWK("r", &rsaPriv.PublicKey),
				ecJWK("e", &ecPriv.PublicKey),
			))
		}))
		defer srv.Close()
		return fetchJWKS(context.Background(), http.DefaultClient, srv.URL)
	}()
	if err != nil {
		t.Fatalf("fetchJWKS: %v", err)
	}
	if k, ok := keys["r"]; !ok || k.rsa == nil || k.alg != "RS256" {
		t.Errorf("rsa key not parsed: %+v", keys["r"])
	}
	if k, ok := keys["e"]; !ok || k.ec == nil || k.alg != "ES256" {
		t.Errorf("ec key not parsed: %+v", keys["e"])
	}
}
