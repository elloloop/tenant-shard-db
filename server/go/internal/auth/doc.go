// Package auth is the gRPC authentication / trusted-actor chokepoint for
// the Go server.
//
// CLAUDE.md invariant: request.actor / request.context.actor on the wire
// is UNTRUSTED. The verified caller identity is established by the
// Interceptor in this package and parked on the request context; every
// authorization decision MUST consult that trusted identity via
// Authoritative, never the request payload directly. The behavioural
// pin is tests/python/integration/test_privilege_escalation.py (34
// cases across 8 RPCs); the spec is
// docs/go-port/shared/auth-interceptor.md.
//
// Layout:
//
//	actor.go Actor type + Kind enum + prefix constructors.
//	identity.go Identity struct + context.Context plumbing.
//	authoritative.go Authoritative() -- the SINGLE trusted-actor chokepoint.
//	interceptor.go Unary + Stream gRPC interceptors; Health bypass list.
//	oauth.go In-memory OAuthValidator (HS256 / RS256 / ES256).
//	jwks.go Production JWKSValidator: network JWKS fetch +
//	                 caching + key rotation, OIDC discovery,
//	                 Google/Microsoft/Okta presets (RS256 / ES256).
//	apikey.go In-memory APIKeyManager.
//	session.go In-memory SessionManager.
//	errors.go UNAUTHENTICATED / PERMISSION_DENIED wrappers.
//
// Production OAuth/OIDC ships here (JWKSValidator), wired into the
// server via the -oauth-issuer / -jwks-url / -oauth-audience /
// -oauth-provider flags on cmd/entdb-server. Redis-backed sessions, the
// production API-key store, and the quota interceptor are tracked
// separately (#87/#88); this package ships the OAuth validators plus the
// trusted-actor plumbing, with in-memory API-key/session managers for
// tests and dev.
package auth
