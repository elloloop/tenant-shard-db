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
//     The trusted Subject is normalised to an Actor, but the privilege
//     class is derived from the *trust anchor* (Identity.Method), NEVER
//     from a prefix the credential itself chose:
//     - A user: prefix is always honoured -- it confers no privilege
//     beyond being that user.
//     - A system: / admin: prefix is honoured ONLY when the Method is one
//     the server itself mints (API key / session, see
//     methodAttestsPrefix). On those carriers a "system:gdpr-worker"
//     subject was set by an operator and is trustworthy.
//     - For externally-asserted methods (OAuth JWT sub/email, mTLS client
//     cert) the subject is attacker-influenceable -- self-service signup,
//     email-as-subject, a cert minted for an unrelated workload -- so any
//     system:/admin:/group: prefix is wrapped down to user:<subject>. A
//     JWT sub "alice" becomes user:alice; a forged sub "admin:root"
//     becomes the powerless user user:admin:root. (Promotion of an
//     OAuth/mTLS principal to system/admin must go through an explicit,
//     operator-configured mapping -- tracked as a follow-up, not a prefix
//     the caller can type.)
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
	parsed := ParseActor(id.Subject)
	// A user: prefix is always safe to honour regardless of method: it
	// names a user and confers no elevated privilege.
	if parsed.Kind() == KindUser {
		return parsed
	}
	// A system: / admin: prefix is honoured ONLY on a server-minted carrier
	// (API key / session). On an externally-asserted carrier (OAuth / mTLS)
	// the subject is attacker-influenceable, so a privileged prefix must NOT
	// elevate -- it is wrapped down to a plain user identity below. group:
	// is never a caller identity and always wraps to a user too.
	if methodAttestsPrefix(id.Method) &&
		(parsed.Kind() == KindSystem || parsed.Kind() == KindAdmin) {
		return parsed
	}
	// Everything else -- a bare subject ("alice"), a forged privileged
	// prefix over OAuth/mTLS ("admin:root"), or a group: subject -- wraps as
	// user:<subject>. The raw subject is kept intact so distinct principals
	// stay distinct (a forged "admin:root" becomes user:"admin:root", which
	// cannot collide with the genuine user:root).
	return User(id.Subject)
}

// methodAttestsPrefix reports whether the authentication method that
// produced an Identity is one EntDB itself mints, and may therefore be
// trusted to carry a privileged (system:/admin:) actor prefix in its
// Subject.
//
// API keys and sessions are issued by the server / operator, so a
// "system:gdpr-worker" subject on one of them was set by us and is
// trustworthy. OAuth (JWT sub/email) and mTLS (client cert) subjects are
// asserted by an external IdP or presented by the caller, who can often
// choose them, so their prefixes confer no privilege. Anything
// unrecognised defaults to untrusted (the safe default).
func methodAttestsPrefix(method string) bool {
	switch method {
	case MethodAPIKey, MethodSession:
		return true
	default:
		return false
	}
}
