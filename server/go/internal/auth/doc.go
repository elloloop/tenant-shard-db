// Package auth is the gRPC authentication / trusted-actor chokepoint for
// the Go server. Ported from server/python/entdb_server/auth/, EPIC #407.
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
//	actor.go          Actor type + Kind enum + prefix constructors.
//	identity.go       Identity struct + context.Context plumbing.
//	authoritative.go  Authoritative() -- the SINGLE trusted-actor chokepoint.
//	interceptor.go    Unary + Stream gRPC interceptors; Health bypass list.
//	oauth.go          In-memory OAuthValidator (HS256 / RS256).
//	apikey.go         In-memory APIKeyManager.
//	session.go        In-memory SessionManager.
//	errors.go         UNAUTHENTICATED / PERMISSION_DENIED wrappers.
//
// Wave 1 ships only the in-memory validators; production OAuth (real
// JWKS rotation, network discovery), Redis-backed sessions, and the
// quota interceptor are deferred to Phase 2.
package auth
