// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
)

// oauthConfig is the operator-facing OAuth/OIDC configuration parsed
// from the -oauth-* flags. When provider is set it seeds issuer / JWKS
// URL from a known IdP preset; explicit -oauth-issuer / -jwks-url always
// win over the preset.
type oauthConfig struct {
	provider string // "" | google | microsoft | okta
	issuer   string
	jwksURL  string
	audience string
}

// enabled reports whether the operator asked for production OIDC auth.
// Any one of provider / issuer / jwks-url being set turns it on.
func (c oauthConfig) enabled() bool {
	return strings.TrimSpace(c.provider) != "" ||
		strings.TrimSpace(c.issuer) != "" ||
		strings.TrimSpace(c.jwksURL) != ""
}

// buildOAuthValidator resolves the flag set into a production
// JWKSValidator. Resolution order:
//
//  1. If -oauth-provider names a known preset, seed issuer + JWKS URL
//     from it (Google has both; Microsoft/Okta are tenant/org-specific
//     so the preset only marks them as discovery-driven and the
//     operator MUST pass -oauth-issuer).
//  2. Explicit -oauth-issuer / -jwks-url override the preset.
//  3. With a JWKS URL, discovery is skipped. Without one, OIDC discovery
//     runs against the issuer (<issuer>/.well-known/openid-configuration).
//
// The returned validator has already warmed its JWKS cache, so a
// misconfigured issuer fails here at boot rather than on the first
// request.
func buildOAuthValidator(ctx context.Context, c oauthConfig) (*auth.JWKSValidator, error) {
	issuer := strings.TrimSpace(c.issuer)
	jwksURL := strings.TrimSpace(c.jwksURL)

	if name := strings.TrimSpace(c.provider); name != "" {
		preset, ok := auth.PresetProvider(name)
		if !ok {
			return nil, fmt.Errorf("--oauth-provider %q is not recognised (want one of %s)",
				name, strings.Join(auth.KnownProviders(), ", "))
		}
		if issuer == "" {
			issuer = preset.Issuer
		}
		if jwksURL == "" {
			jwksURL = preset.JWKSURL
		}
		if issuer == "" {
			// Microsoft / Okta: tenant/org-specific issuer the preset
			// cannot know. Force the operator to supply it.
			return nil, fmt.Errorf("--oauth-provider %q is tenant/org-specific; also pass --oauth-issuer with the concrete issuer URL", name)
		}
	}

	if issuer == "" {
		return nil, fmt.Errorf("OAuth requires --oauth-issuer (or a --oauth-provider preset that supplies one)")
	}

	v, err := auth.NewJWKSValidator(ctx, auth.JWKSOptions{
		Issuer:   issuer,
		Audience: strings.TrimSpace(c.audience),
		JWKSURL:  jwksURL,
	})
	if err != nil {
		return nil, err
	}
	return v, nil
}

// oauthSummary is a one-line, secret-free description for the boot log.
func oauthSummary(c oauthConfig, v *auth.JWKSValidator) string {
	prov := strings.TrimSpace(c.provider)
	if prov == "" {
		prov = "custom"
	}
	aud := strings.TrimSpace(c.audience)
	if aud == "" {
		aud = "(unchecked — set --oauth-audience in production)"
	}
	return fmt.Sprintf("provider=%s issuer=%s jwks=%s audience=%s", prov, c.issuerOrPreset(), v.JWKSURL(), aud)
}

// issuerOrPreset reports the effective issuer for logging, falling back
// to the preset name when the operator relied purely on -oauth-provider.
func (c oauthConfig) issuerOrPreset() string {
	if s := strings.TrimSpace(c.issuer); s != "" {
		return s
	}
	if name := strings.TrimSpace(c.provider); name != "" {
		if p, ok := auth.PresetProvider(name); ok && p.Issuer != "" {
			return p.Issuer
		}
	}
	return "(discovery)"
}
