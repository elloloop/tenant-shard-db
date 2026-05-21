// SPDX-License-Identifier: AGPL-3.0-only

package acl

// Grant is the in-Go representation of one row of node_access — an ACL
// grant for (NodeID, SubjectActor).
//
// Both the legacy Permission field and the typed Capability fields are
// kept; on read the canonical store returns whichever the row has.
// LegacyToCoreCaps fills the typed fields when the row is legacy-only.
type Grant struct {
	// Subject is the grantee — who the grant applies to.
	Subject Actor
	// NodeID is the node the grant applies to.
	NodeID string
	// TypeID is the node's type id at grant time. Used by Registry to
	// scope ExtCapID interpretation. 0 == "core only / type-agnostic".
	TypeID int32
	// Permission is the legacy enum (still on the wire).
	Permission Permission
	// CoreCaps is the typed core-capability set this grant carries.
	// Empty when the row is legacy-only — call LegacyToCoreCaps to
	// derive.
	CoreCaps []CoreCapability
	// ExtCapIDs is the typed extension-capability set, scoped to TypeID.
	ExtCapIDs []ExtCapID
	// GrantedBy is the actor that authored the grant. Used for audit
	// and for the "delegating actor must already hold the cap" check
	// (W1.10 applier territory; this package does not enforce it).
	GrantedBy Actor
	// GrantedAt is the unix-millis timestamp the grant was authored.
	GrantedAt int64
	// ExpiresAt is the unix-millis expiry; nil means "never expires".
	// Mirrors node_access.expires_at NULLABLE.
	ExpiresAt *int64
}

// IsExpired reports whether the grant has expired relative to now (a
// unix-millis clock reading). A nil ExpiresAt means "never".
func (g Grant) IsExpired(nowMs int64) bool {
	if g.ExpiresAt == nil {
		return false
	}
	return *g.ExpiresAt <= nowMs
}

// IsDeny reports whether this grant is the explicit-negative DENY row.
// DENY rows carry no positive caps; the deny semantics is enforced by
// the Checker two-pass.
func (g Grant) IsDeny() bool { return g.Permission == PermDeny }

// EffectiveCoreCaps returns the typed core-cap set this grant satisfies,
// preferring the explicit CoreCaps field when present and falling back
// to LegacyToCoreCaps(g.Permission) for legacy-only rows.
func (g Grant) EffectiveCoreCaps() []CoreCapability {
	if len(g.CoreCaps) > 0 {
		return g.CoreCaps
	}
	return LegacyToCoreCaps(g.Permission)
}
