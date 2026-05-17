// SPDX-License-Identifier: AGPL-3.0-only

package acl

import "strings"

// ActorKind enumerates the principal categories valid as ACL grant
// subjects. Includes the cross-tenant "tenant:" addition from
// docs/decisions/acl.md (2026-04-13).
//
// Note this is a wider set than auth.Kind: the auth interceptor accepts
// only user/system/admin as caller identities, while ACL grant subjects
// also include group/role/tenant/service (the latter three never
// authenticate, but they ARE valid grantees).
type ActorKind uint8

const (
	// ActorKindUnknown is the zero value — a parse failure or the empty
	// actor. Treat as untrusted.
	ActorKindUnknown ActorKind = iota
	// ActorKindUser — user:<id>. A specific user.
	ActorKindUser
	// ActorKindGroup — group:<id>. Resolved at check time via group_users.
	ActorKindGroup
	// ActorKindService — service:<id>. A service account.
	ActorKindService
	// ActorKindRole — role:<name>. Reserved; matches() does not expand it.
	ActorKindRole
	// ActorKindTenant — tenant:<id> grantee. <id> may be "*" (every
	// authenticated user in the current tenant) or a specific tenant id
	// (every authenticated user in tenant <id>; cross-tenant per
	// docs/decisions/acl.md 2026-04-13).
	ActorKindTenant
	// ActorKindSystem — system:<svc>. Bypasses capability checks.
	ActorKindSystem
)

// String returns the canonical prefix without the trailing colon.
// Mirrors the prefix strings used by the Python Principal type.
func (k ActorKind) String() string {
	switch k {
	case ActorKindUser:
		return "user"
	case ActorKindGroup:
		return "group"
	case ActorKindService:
		return "service"
	case ActorKindRole:
		return "role"
	case ActorKindTenant:
		return "tenant"
	case ActorKindSystem:
		return "system"
	default:
		return "unknown"
	}
}

// Actor is a value-typed ACL principal. It is intentionally narrower
// than auth.Actor — that type is for caller identities the auth
// interceptor admits, while this type is for grant subjects (which
// includes group/role/service).
//
// The String form is "kind:id", matching the format stored in
// node_access.actor_id and the ACLEntry.grantee proto field.
type Actor struct {
	Kind ActorKind
	ID   string
}

// User returns a user:<id> actor.
func User(id string) Actor { return Actor{Kind: ActorKindUser, ID: id} }

// Group returns a group:<id> actor.
func Group(id string) Actor { return Actor{Kind: ActorKindGroup, ID: id} }

// Service returns a service:<id> actor.
func Service(id string) Actor { return Actor{Kind: ActorKindService, ID: id} }

// Role returns a role:<name> actor. Role expansion is a future concern;
// the current Resolver returns it unchanged.
func Role(name string) Actor { return Actor{Kind: ActorKindRole, ID: name} }

// Tenant returns a tenant:<id> actor. Use TenantWildcard for the
// current-tenant wildcard.
func Tenant(id string) Actor { return Actor{Kind: ActorKindTenant, ID: id} }

// TenantWildcard returns the tenant:* actor — every authenticated user
// in the current tenant. Matches the wildcard handled at
// canonical_store.py:2887 and acl.py:127-129.
func TenantWildcard() Actor { return Actor{Kind: ActorKindTenant, ID: "*"} }

// System returns a system:<svc> actor.
func System(svc string) Actor { return Actor{Kind: ActorKindSystem, ID: svc} }

// IsZero reports whether a is the zero value.
func (a Actor) IsZero() bool { return a.Kind == ActorKindUnknown && a.ID == "" }

// IsUser reports whether the actor is a user:<id>.
func (a Actor) IsUser() bool { return a.Kind == ActorKindUser }

// IsGroup reports whether the actor is a group:<id>.
func (a Actor) IsGroup() bool { return a.Kind == ActorKindGroup }

// IsTenantWildcard reports whether the actor is tenant:*.
func (a Actor) IsTenantWildcard() bool {
	return a.Kind == ActorKindTenant && a.ID == "*"
}

// IsSystem reports whether the actor is a system:<svc>. System actors
// bypass capability checks (mirrors canonical_store.py:2883-2884).
func (a Actor) IsSystem() bool { return a.Kind == ActorKindSystem }

// String renders the actor as "kind:id". For the zero value returns "".
func (a Actor) String() string {
	if a.IsZero() {
		return ""
	}
	if a.Kind == ActorKindUnknown {
		return a.ID
	}
	return a.Kind.String() + ":" + a.ID
}

// ParseActor parses a "kind:id" string. Empty input returns the zero
// Actor. An unknown prefix yields ActorKindUnknown with the raw string
// preserved as ID.
func ParseActor(s string) Actor {
	if s == "" {
		return Actor{}
	}
	idx := strings.IndexByte(s, ':')
	if idx <= 0 || idx == len(s)-1 {
		return Actor{Kind: ActorKindUnknown, ID: s}
	}
	prefix, id := s[:idx], s[idx+1:]
	switch prefix {
	case "user":
		return Actor{Kind: ActorKindUser, ID: id}
	case "group":
		return Actor{Kind: ActorKindGroup, ID: id}
	case "service":
		return Actor{Kind: ActorKindService, ID: id}
	case "role":
		return Actor{Kind: ActorKindRole, ID: id}
	case "tenant":
		return Actor{Kind: ActorKindTenant, ID: id}
	case "system":
		return Actor{Kind: ActorKindSystem, ID: id}
	default:
		return Actor{Kind: ActorKindUnknown, ID: s}
	}
}
