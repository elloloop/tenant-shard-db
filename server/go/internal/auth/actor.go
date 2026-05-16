// SPDX-License-Identifier: AGPL-3.0-only

package auth

import "strings"

// Kind is the prefix-encoded category of an Actor identity.
//
// Mirrors the prefix scheme documented in
// docs/go-port/shared/auth-interceptor.md "Trusted-actor contract":
//
//   - user:<id> -- a real authenticated user.
//   - system:<svc> -- internal service identity; bypasses tenant-membership
//     checks.
//   - admin:<id> -- operator/admin identity; same bypass as system:.
//   - group:<id> -- only valid in ACL grant subjects, never as a caller.
//
// The literal "__system__" actor used by the Applier as a bootstrap/replay
// identity never appears on the wire and is not modelled here; if a future
// caller needs it, expose a dedicated constant rather than smuggling it
// through Kind.
type Kind int

const (
	// KindUnknown is the zero value. An Actor with KindUnknown carries an
	// ID that did not match any known prefix; treat it as untrusted.
	KindUnknown Kind = iota
	// KindUser is the user:<id> prefix.
	KindUser
	// KindSystem is the system:<svc> prefix.
	KindSystem
	// KindAdmin is the admin:<id> prefix.
	KindAdmin
	// KindGroup is the group:<id> prefix (ACL subjects only).
	KindGroup
)

// String returns the canonical prefix for the kind without the trailing
// colon. Mirrors the Python prefix strings used in
func (k Kind) String() string {
	switch k {
	case KindUser:
		return "user"
	case KindSystem:
		return "system"
	case KindAdmin:
		return "admin"
	case KindGroup:
		return "group"
	default:
		return "unknown"
	}
}

// Actor is a prefix-encoded caller identity. It is a value type whose
// String form (kind:id) is exactly what the Python server stores in the WAL
// and in ACL subjects.
//
// Construct via User / System / Admin / Group. Parse via ParseActor for
// strings coming off the wire or out of the WAL.
//
// The zero value is the "unknown" actor and is never equal to a valid one;
// callers should treat it the way the Python server treats a None
// identity -- no claimed prefix, no privileges.
type Actor struct {
	kind Kind
	id   string
}

// User returns a user:<id> actor. Mirrors Actor.user(id) in the Python SDK.
func User(id string) Actor { return Actor{kind: KindUser, id: id} }

// System returns a system:<id> actor.
func System(id string) Actor { return Actor{kind: KindSystem, id: id} }

// Admin returns an admin:<id> actor.
func Admin(id string) Actor { return Actor{kind: KindAdmin, id: id} }

// Group returns a group:<id> actor. Group actors are only meaningful as
// ACL grant subjects; they MUST NOT be accepted as caller identities by
// the auth interceptor.
func Group(id string) Actor { return Actor{kind: KindGroup, id: id} }

// Kind returns the Actor's kind enum. Use the Is* predicates instead for
// the common case.
func (a Actor) Kind() Kind { return a.kind }

// ID returns the bare identifier (no prefix). For example, User("alice").ID()
// is "alice", not "user:alice".
func (a Actor) ID() string { return a.id }

// IsUser reports whether the actor is a user:<id>.
func (a Actor) IsUser() bool { return a.kind == KindUser }

// IsSystem reports whether the actor is a system:<svc>.
func (a Actor) IsSystem() bool { return a.kind == KindSystem }

// IsAdmin reports whether the actor is an admin:<id>.
func (a Actor) IsAdmin() bool { return a.kind == KindAdmin }

// IsGroup reports whether the actor is a group:<id>. Caller identities
// should never be groups; this predicate exists for ACL-subject parsing.
func (a Actor) IsGroup() bool { return a.kind == KindGroup }

// IsZero reports whether the actor carries no kind and no id. Equivalent
// to a == Actor{}.
func (a Actor) IsZero() bool { return a.kind == KindUnknown && a.id == "" }

// String renders the actor in the canonical wire form, "kind:id". For the
// zero value this returns the empty string -- callers that need a stable
// non-empty representation should range over Kind/ID directly.
func (a Actor) String() string {
	if a.IsZero() {
		return ""
	}
	if a.kind == KindUnknown {
		// Best-effort: surface the raw id we never managed to classify.
		return a.id
	}
	return a.kind.String() + ":" + a.id
}

// ParseActor parses a "kind:id" string into an Actor. Unknown prefixes
// produce an Actor{Kind: KindUnknown, id: s} so the original raw string is
// preserved for logging / error messages; callers can detect this via
// Kind() == KindUnknown.
//
// An empty input returns the zero Actor (IsZero() == true).
func ParseActor(s string) Actor {
	if s == "" {
		return Actor{}
	}
	idx := strings.IndexByte(s, ':')
	if idx <= 0 || idx == len(s)-1 {
		return Actor{kind: KindUnknown, id: s}
	}
	prefix, id := s[:idx], s[idx+1:]
	switch prefix {
	case "user":
		return Actor{kind: KindUser, id: id}
	case "system":
		return Actor{kind: KindSystem, id: id}
	case "admin":
		return Actor{kind: KindAdmin, id: id}
	case "group":
		return Actor{kind: KindGroup, id: id}
	default:
		return Actor{kind: KindUnknown, id: s}
	}
}
