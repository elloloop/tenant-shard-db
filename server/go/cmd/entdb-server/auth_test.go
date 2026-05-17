// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOAuthConfigEnabled(t *testing.T) {
	cases := []struct {
		name string
		c    oauthConfig
		want bool
	}{
		{"empty", oauthConfig{}, false},
		{"provider", oauthConfig{provider: "google"}, true},
		{"issuer", oauthConfig{issuer: "https://x"}, true},
		{"jwks", oauthConfig{jwksURL: "https://x/jwks"}, true},
		{"audience-only is not enough", oauthConfig{audience: "a"}, false},
	}
	for _, tc := range cases {
		if got := tc.c.enabled(); got != tc.want {
			t.Errorf("%s: enabled() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestBuildOAuthValidator_RejectsUnknownProvider(t *testing.T) {
	_, err := buildOAuthValidator(context.Background(), oauthConfig{provider: "facebook"})
	if err == nil || !strings.Contains(err.Error(), "not recognised") {
		t.Fatalf("err = %v, want unrecognised-provider error", err)
	}
}

func TestBuildOAuthValidator_TenantSpecificProviderNeedsIssuer(t *testing.T) {
	// microsoft/okta presets carry no static issuer; the operator must
	// supply --oauth-issuer.
	for _, p := range []string{"microsoft", "okta"} {
		_, err := buildOAuthValidator(context.Background(), oauthConfig{provider: p})
		if err == nil || !strings.Contains(err.Error(), "--oauth-issuer") {
			t.Errorf("provider=%s err = %v, want must-pass-issuer error", p, err)
		}
	}
}

func TestBuildOAuthValidator_RequiresIssuer(t *testing.T) {
	_, err := buildOAuthValidator(context.Background(), oauthConfig{audience: "a"})
	if err == nil || !strings.Contains(err.Error(), "--oauth-issuer") {
		t.Fatalf("err = %v, want requires-issuer error", err)
	}
}

func TestBuildOAuthValidator_DiscoversAndWarmsCache(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   srv.URL,
			"jwks_uri": srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		eBytes := big.NewInt(int64(priv.PublicKey.E)).Bytes()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{{
				"kty": "RSA", "kid": "k", "alg": "RS256", "use": "sig",
				"n": base64.RawURLEncoding.EncodeToString(priv.PublicKey.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(eBytes),
			}},
		})
	})

	v, err := buildOAuthValidator(context.Background(), oauthConfig{
		issuer:   srv.URL,
		audience: "entdb",
	})
	if err != nil {
		t.Fatalf("buildOAuthValidator: %v", err)
	}
	if v.JWKSURL() != srv.URL+"/jwks" {
		t.Errorf("resolved JWKS URL = %q, want %q", v.JWKSURL(), srv.URL+"/jwks")
	}
}

func TestBuildOAuthValidator_FailsClosedOnBadIssuer(t *testing.T) {
	// Discovery against a dead host must fail at boot, not silently
	// produce an always-reject validator.
	_, err := buildOAuthValidator(context.Background(), oauthConfig{
		issuer: "http://127.0.0.1:0/no-such-issuer",
	})
	if err == nil {
		t.Fatal("expected boot failure for unreachable issuer")
	}
}
