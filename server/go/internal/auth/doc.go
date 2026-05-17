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
//	apikey.go argon2id hashing + in-memory APIKeyManager.
//	apikey_persistent.go PersistentAPIKeyManager (durable, rotatable).
//	session.go SessionManager: per-session TTL, per-user
//	                concurrent-session cap, request-checked
//	                revocation list, over a pluggable SessionStore
//	                (in-memory default; Redis seam documented).
//	errors.go UNAUTHENTICATED / PERMISSION_DENIED /
//	                RESOURCE_EXHAUSTED wrappers.
//
// Production OAuth/OIDC ships here (JWKSValidator: network JWKS fetch +
// caching + key rotation, OIDC discovery, Google/Microsoft/Okta
// presets), wired into the server via the -oauth-issuer / -jwks-url /
// -oauth-audience / -oauth-provider flags on cmd/entdb-server.
//
// API keys are hashed with argon2id (golang.org/x/crypto/argon2) in a
// PHC-format string with a per-key random salt; verification is a
// constant-time compare of the derived key. The PersistentAPIKeyManager
// stores those hashes in global.db (via the APIKeyStore interface, so
// auth never imports globalstore) and supports rotation: multiple
// simultaneously-active keys for a documented migration window (issue
// the new key, flip clients over, then revoke the old key_id). It is
// wired into cmd/entdb-server behind the --api-key-auth flag.
//
// Session management is production-shaped: the SessionStore interface
// in session.go is the seam for sharing the session table and
// revocation list across replicas (Redis, or a stateless signed-token
// scheme). The default in-process store is correct for single-node and
// tests. The quota interceptor is tracked separately; this package
// ships the production OAuth validators, the argon2id API-key managers
// (in-memory + persistent), and the production session manager, plus
// the trusted-actor plumbing.
package auth
