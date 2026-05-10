// SPDX-License-Identifier: AGPL-3.0-only

package auth

import "context"

// Identity is the verified caller's claims as established by the
// interceptor. It mirrors AuthContext in
// server/python/entdb_server/auth/auth_interceptor.py:118-137.
//
// Subject is the raw verified identifier (JWT sub, API-key name, or
// session user_id). It is NOT prefix-normalised here; normalisation to a
// canonical Actor happens once, at the Authoritative() chokepoint, so the
// raw subject stays available for logging and quota labelling.
//
// Scopes is populated only for API-key auth; OAuth and session methods
// leave it nil.
//
// Claims carries raw JWT claims for OAuth; nil/empty for the other two
// methods. It is a generic map so handlers don't take a hard dependency on
// any particular JWT library.
type Identity struct {
	// Method is "oauth" | "api_key" | "session". Constants below match
	// the Python AuthContext.method values verbatim.
	Method   string
	Subject  string
	Scopes   []string
	Claims   map[string]any
	Metadata map[string]any
}

// Auth method names. These strings are part of the public contract --
// metrics and logs key off them, mirroring the Python AuthContext.method
// field.
const (
	MethodOAuth   = "oauth"
	MethodAPIKey  = "api_key"
	MethodSession = "session"
)

// IsZero reports whether the Identity carries no method and no subject.
// Used by Authoritative to detect "no interceptor ran" without having to
// distinguish a missing context value from a populated-but-empty one.
func (i Identity) IsZero() bool {
	return i.Method == "" && i.Subject == ""
}

// contextKey is the unexported type used as the context.Context value key
// for the trusted Identity. Following the documented Go idiom, the type is
// unexported and a singleton zero-value sentinel is held in identityKey;
// no other package can collide on this key. See
// docs/go-port/shared/auth-interceptor.md "Go interface".
type contextKey struct{}

var identityKey = contextKey{}

// WithIdentity returns a new context that carries id as the trusted
// Identity. The interceptor calls this exactly once per request, after
// successful authentication. Tests that need to simulate an authenticated
// request also call this directly -- there is no global ContextVar
// equivalent in Go (and ContextVars are exactly what the Python port is
// trying to leave behind, see
// docs/go-port/shared/auth-interceptor.md "Wire-level mechanics" tail).
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// IdentityFromContext returns the trusted Identity attached to ctx by the
// interceptor. The bool is false (and the Identity is the zero value) when
// no interceptor ran for this request -- a no-auth deployment, a unit test
// that called the handler directly, or a Health RPC on the bypass list.
//
// Mirrors get_current_identity in
// server/python/entdb_server/auth/auth_interceptor.py:66-74, except that
// the absent case returns (Identity{}, false) instead of None so callers
// can use the standard ", ok" idiom.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	v, ok := ctx.Value(identityKey).(Identity)
	if !ok {
		return Identity{}, false
	}
	if v.IsZero() {
		return Identity{}, false
	}
	return v, true
}
