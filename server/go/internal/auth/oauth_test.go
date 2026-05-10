// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

func TestMemoryOAuthValidator_HS256_Valid(t *testing.T) {
	secret := []byte("test-secret-please-do-not-use-in-prod")
	v := NewMemoryOAuthValidator("https://issuer.test", "entdb-test")
	v.AddKey("kid-1", JWKKey{Alg: "HS256", HS256Secret: secret})

	tok, err := SignHS256ForTest(secret, "kid-1", map[string]any{
		"sub": "alice",
		"iss": "https://issuer.test",
		"aud": "entdb-test",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := v.Validate(context.Background(), tok)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims["sub"] != "alice" {
		t.Errorf("sub = %v, want alice", claims["sub"])
	}
}

func TestMemoryOAuthValidator_RS256_Valid(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	v := NewMemoryOAuthValidator("https://issuer.test", "entdb-test")
	v.AddKey("kid-rsa", JWKKey{Alg: "RS256", RS256Public: &priv.PublicKey})

	tok, err := SignRS256ForTest(priv, "kid-rsa", map[string]any{
		"sub": "bob",
		"iss": "https://issuer.test",
		"aud": "entdb-test",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := v.Validate(context.Background(), tok)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims["sub"] != "bob" {
		t.Errorf("sub = %v, want bob", claims["sub"])
	}
}

func TestMemoryOAuthValidator_RejectsExpired(t *testing.T) {
	secret := []byte("s")
	v := NewMemoryOAuthValidator("iss", "aud")
	v.AddKey("k", JWKKey{Alg: "HS256", HS256Secret: secret})
	tok, _ := SignHS256ForTest(secret, "k", map[string]any{
		"sub": "alice",
		"iss": "iss",
		"aud": "aud",
		"exp": time.Now().Add(-time.Minute).Unix(),
	})
	_, err := v.Validate(context.Background(), tok)
	if err == nil {
		t.Fatal("expected expired-token error")
	}
	if errs.Code(err) != codes.Unauthenticated {
		t.Errorf("code = %v, want Unauthenticated", errs.Code(err))
	}
}

func TestMemoryOAuthValidator_RejectsBadSig(t *testing.T) {
	v := NewMemoryOAuthValidator("iss", "aud")
	v.AddKey("k", JWKKey{Alg: "HS256", HS256Secret: []byte("right")})
	tok, _ := SignHS256ForTest([]byte("wrong"), "k", map[string]any{
		"sub": "alice",
		"iss": "iss",
		"aud": "aud",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	_, err := v.Validate(context.Background(), tok)
	if err == nil || errs.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected UNAUTHENTICATED, got err=%v code=%v", err, errs.Code(err))
	}
}

func TestMemoryOAuthValidator_RejectsUnknownKid(t *testing.T) {
	v := NewMemoryOAuthValidator("iss", "aud")
	tok, _ := SignHS256ForTest([]byte("s"), "missing", map[string]any{
		"sub": "alice",
		"iss": "iss",
		"aud": "aud",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	_, err := v.Validate(context.Background(), tok)
	if err == nil || errs.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected UNAUTHENTICATED for unknown kid, got %v", err)
	}
}

func TestMemoryOAuthValidator_RejectsAlgConfusion(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := NewMemoryOAuthValidator("iss", "aud")
	// Key registered as RS256, but token claims HS256 -> reject.
	v.AddKey("k", JWKKey{Alg: "RS256", RS256Public: &priv.PublicKey})
	tok, _ := SignHS256ForTest([]byte("anything"), "k", map[string]any{
		"sub": "alice",
		"iss": "iss",
		"aud": "aud",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if _, err := v.Validate(context.Background(), tok); err == nil {
		t.Fatal("expected algorithm-confusion rejection")
	}
}

func TestMemoryOAuthValidator_RejectsBadIssuerOrAudience(t *testing.T) {
	secret := []byte("s")
	v := NewMemoryOAuthValidator("https://right.test", "right-aud")
	v.AddKey("k", JWKKey{Alg: "HS256", HS256Secret: secret})
	tok, _ := SignHS256ForTest(secret, "k", map[string]any{
		"sub": "alice",
		"iss": "https://wrong.test",
		"aud": "right-aud",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if _, err := v.Validate(context.Background(), tok); err == nil {
		t.Errorf("expected issuer rejection")
	}
	tok, _ = SignHS256ForTest(secret, "k", map[string]any{
		"sub": "alice",
		"iss": "https://right.test",
		"aud": "wrong-aud",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if _, err := v.Validate(context.Background(), tok); err == nil {
		t.Errorf("expected audience rejection")
	}
}

func TestMemoryOAuthValidator_RejectsNoneAlg(t *testing.T) {
	// Synthesise a "alg: none" token by signing with HS256 then
	// rewriting the header. Easier: validator just refuses anything
	// outside HS256/RS256.
	v := NewMemoryOAuthValidator("iss", "aud")
	v.AddKey("k", JWKKey{Alg: "HS256", HS256Secret: []byte("s")})
	// Manually crafted "none"-alg token (header: {"alg":"none","kid":"k"},
	// payload: {}, sig: empty). Use base64 raw-url no padding.
	tok := "eyJhbGciOiJub25lIiwia2lkIjoiayJ9.e30."
	if _, err := v.Validate(context.Background(), tok); err == nil {
		t.Errorf("expected rejection of alg=none")
	}
}
