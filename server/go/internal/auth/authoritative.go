// SPDX-License-Identifier: AGPL-3.0-only

package auth

import "context"

// Authoritative is the SINGLE choke point for the trusted-actor pattern.
// Every handler that performs authorization MUST call this once at the
// top, with the actor claimed in the request payload, and rebind the
// local variable to the result.
//
// The behavioural pin is tests/python/integration/test_grpc_contract.py
// (privilege-escalation cases).
//
// Resolution rules, in order:
//
//  1. If the interceptor populated a trusted Identity on ctx, that
//     Identity wins -- regardless of what the request payload claims.
//     This is the privilege-escalation fix: a malicious caller can
//     authenticate as themselves and still claim
//     actor: "system:admin" in the request body, and we MUST NOT
//     honour the body claim.
//
//     The trusted Subject is normalised to an Actor:
//     - If it already starts with user:, system:, or admin:, ParseActor
//     gives us the right kind verbatim.
//     - Otherwise it is wrapped as user:<subject>. A JWT
//     sub: "alice" becomes user:alice; a session for
//     user_id: "user:alice" stays user:alice.
//
//  2. If no trusted Identity is on ctx (interceptor disabled, unit tests,
//     or no-auth deployment mode), claimed is returned unchanged. This
//     is the documented fallback. A production server MUST run the
//     interceptor; without it, every handler trusts the wire payload,
//     which is exactly the privilege-escalation hole this package
//     exists to plug. See docs/go-port/shared/auth-interceptor.md
//     "Trusted-actor contract" final paragraph.
//
// Special case: the literal "__system__" bootstrap actor used by the Applier
// never appears on the wire. The Applier constructs that actor directly via a
// dedicated constant rather than routing it through Authoritative.
//
// Defence in depth: subsequent helpers (admin checks, tenant-access
// checks, cross-tenant-read checks) also re-call Authoritative to
// re-derive the trusted actor from ctx -- a belt-and-suspenders pattern
// to guard against any path that skips the top-of-handler call.
func Authoritative(ctx context.Context, claimed Actor) Actor {
	id, ok := IdentityFromContext(ctx)
	if !ok {
		return claimed
	}
	// Subject may already be a prefixed actor string ("user:alice",
	// "system:gdpr-worker") -- common when the credential carrier is a
	// session for a tenant_principal value. Try ParseActor first so we
	// preserve the kind.
	if parsed := ParseActor(id.Subject); parsed.Kind() == KindUser ||
		parsed.Kind() == KindSystem ||
		parsed.Kind() == KindAdmin {
		return parsed
	}
	// No recognised prefix -- wrap as user:<subject>. Handles the JWT
	// sub: "alice" -> user:alice case.
	return User(id.Subject)
}
