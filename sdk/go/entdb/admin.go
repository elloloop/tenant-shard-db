package entdb

import "context"

// Admin groups EntDB's cross-tenant identity operations: creating
// tenants and users in the global registry, managing memberships,
// and bulk ACL admin (transfer, delegate, revoke).
//
// These calls cross tenant boundaries — they take ``actor`` as a
// raw string (typically ``"system:admin"`` or a tenant
// owner/admin actor) rather than a scoped Actor. They live on
// ``DbClient`` rather than ``Scope`` because most of them either
// operate above any single tenant (CreateTenant, CreateUser,
// GetUserTenants) or want the raw ``user_id`` form to match the
// global registry's identifier shape.
//
// Get a handle via [DbClient.Admin]:
//
//	admin := client.Admin()
//	tenant, err := admin.CreateTenant(ctx, "system:admin", "acme", "Acme Corp")
//	user, err := admin.CreateUser(ctx, "system:admin", "alice", "alice@acme.test", "Alice")
//	err = admin.AddTenantMember(ctx, "system:admin", "acme", "alice", "member")
//
// Like the rest of the SDK, every admin write goes through the WAL
// — they are auditable and replayable, not direct SQLite mutations.
type Admin struct {
	client *DbClient
}

// Admin returns a typed handle for the cross-tenant admin RPCs.
// See [Admin] for the full surface.
func (c *DbClient) Admin() *Admin {
	return &Admin{client: c}
}

// CreateTenant registers a new tenant in the global registry. The
// new tenant has no members; call [Admin.AddTenantMember] to invite
// users. ``actor`` must be ``"system:admin"`` (or another principal
// with the admin capability).
//
// Re-creating a tenant that already exists raises an underlying
// uniqueness error from the global SQLite — there is no "create or
// get" form by design.
//
// Pass [WithRegion] to pin the tenant to a specific geographic
// region (e.g. ``WithRegion("eu-west-1")``); when omitted the
// server defaults the tenant to its own served region. The pinned
// region is reflected on [TenantDetail.Region].
func (a *Admin) CreateTenant(ctx context.Context, actor, tenantID, name string, opts ...CreateTenantOption) (*TenantDetail, error) {
	return a.client.transport.CreateTenant(ctx, actor, tenantID, name, opts...)
}

// CreateUser registers a new user in the global registry. ``email``
// is unique across all tenants — re-using an email raises an
// AlreadyExists error from the server.
func (a *Admin) CreateUser(ctx context.Context, actor, userID, email, name string) (*UserInfo, error) {
	return a.client.transport.CreateUser(ctx, actor, userID, email, name)
}

// AddTenantMember adds ``userID`` to ``tenantID`` with the given
// ``role`` (``"owner"`` / ``"admin"`` / ``"member"`` /
// ``"viewer"``). Both the user and tenant must exist.
func (a *Admin) AddTenantMember(ctx context.Context, actor, tenantID, userID, role string) error {
	return a.client.transport.AddTenantMember(ctx, actor, tenantID, userID, role)
}

// RemoveTenantMember drops a membership row. It does NOT reassign
// nodes the user owns or revoke their direct ACL grants — for that
// see [Admin.TransferUserContent] and [Admin.RevokeAllUserAccess].
// Combine all three for a complete off-board.
func (a *Admin) RemoveTenantMember(ctx context.Context, actor, tenantID, userID string) error {
	return a.client.transport.RemoveTenantMember(ctx, actor, tenantID, userID)
}

// ChangeMemberRole promotes or demotes a member. The server treats
// roles as opaque strings — capability mappings are configured on
// the server side, not enforced here.
func (a *Admin) ChangeMemberRole(ctx context.Context, actor, tenantID, userID, newRole string) error {
	return a.client.transport.ChangeMemberRole(ctx, actor, tenantID, userID, newRole)
}

// GetTenantMembers returns every membership row for ``tenantID`` —
// who is in this tenant, with what role.
func (a *Admin) GetTenantMembers(ctx context.Context, actor, tenantID string) ([]TenantMember, error) {
	return a.client.transport.GetTenantMembers(ctx, actor, tenantID)
}

// GetUserTenants returns every tenant the user is a member of —
// where this user has access, with what role.
func (a *Admin) GetUserTenants(ctx context.Context, actor, userID string) ([]TenantMember, error) {
	return a.client.transport.GetUserTenants(ctx, actor, userID)
}

// DelegateAccess grants ``toUser`` the listed ``permission`` on
// every node ``fromUser`` owns in ``tenantID`` — useful for
// vacation cover, audit reviewers, and short-lived support
// sessions. Pass ``expiresAtMs = 0`` for a permanent delegation;
// otherwise the grant becomes invisible to the ACL check after
// that absolute epoch-millis timestamp (no background sweep needed
// — the check evaluates the timestamp at read time).
func (a *Admin) DelegateAccess(ctx context.Context, actor, tenantID, fromUser, toUser, permission string, expiresAtMs int64) (*DelegateResult, error) {
	return a.client.transport.DelegateAccess(ctx, actor, tenantID, fromUser, toUser, permission, expiresAtMs)
}

// TransferUserContent reassigns every node owned by ``fromUser`` in
// ``tenantID`` to ``toUser`` — the classic offboarding flow.
// Server-side this also ensures ``toUser`` is a member of the
// tenant, adding them as ``"member"`` if not. It does NOT revoke
// ``fromUser``'s direct ACL grants on other people's nodes — pair
// with [Admin.RevokeAllUserAccess] for a full off-board. Returns
// the count of transferred nodes.
func (a *Admin) TransferUserContent(ctx context.Context, actor, tenantID, fromUser, toUser string) (int32, error) {
	return a.client.transport.TransferUserContent(ctx, actor, tenantID, fromUser, toUser)
}

// RevokeAllUserAccess emergency-removes every direct grant, group
// membership, and shared-index entry the user has in ``tenantID``,
// and drops their ``tenant_members`` row. After this returns the
// user has zero visibility into the tenant. Does NOT delete or
// anonymize the user's owned nodes — for that, see
// [Admin.TransferUserContent] (offboarding) or the GDPR delete
// flow (right-to-erasure with grace period).
func (a *Admin) RevokeAllUserAccess(ctx context.Context, actor, tenantID, userID string) (*RevokeAllResult, error) {
	return a.client.transport.RevokeAllUserAccess(ctx, actor, tenantID, userID)
}

// ── GDPR user lifecycle (right-to-be-forgotten) ─────────────────────

// DeleteUser schedules deletion of ``userID``. The deletion is not
// immediate — the GDPR worker executes it after a grace period
// (default 30 days; pass ``graceDays > 0`` to override). The user
// enters ``"deletion_pending"`` status right away; reads continue
// to work until the grace expires. Use [Admin.CancelUserDeletion]
// to lift the schedule before then.
//
// Returns the absolute requested-at and execute-at timestamps so
// you can show "your data will be removed at <date>" UI without a
// follow-up read.
func (a *Admin) DeleteUser(ctx context.Context, actor, userID string, graceDays int32) (*DeletionScheduled, error) {
	return a.client.transport.DeleteUser(ctx, actor, userID, graceDays)
}

// CancelUserDeletion lifts a pending deletion request. Idempotent
// — calling it on a user with no pending deletion returns success
// with no observable change.
func (a *Admin) CancelUserDeletion(ctx context.Context, actor, userID string) error {
	return a.client.transport.CancelUserDeletion(ctx, actor, userID)
}

// ExportUserData returns the GDPR Article 20 portability bundle —
// a JSON-serialized export of every node the user owns across all
// tenants. The exact JSON shape is documented server-side; treat
// it as opaque from the SDK's perspective.
func (a *Admin) ExportUserData(ctx context.Context, actor, userID string) (string, error) {
	return a.client.transport.ExportUserData(ctx, actor, userID)
}

// FreezeUser flips ``userID``'s status. With ``enabled = true``
// the user is frozen — their authentication continues to succeed
// but every read or write is rejected until they are unfrozen.
// Their data is preserved (no deletion clock starts). Pass
// ``enabled = false`` to lift the freeze.
//
// Returns the new status string (typically ``"frozen"`` or
// ``"active"``).
func (a *Admin) FreezeUser(ctx context.Context, actor, userID string, enabled bool) (string, error) {
	return a.client.transport.FreezeUser(ctx, actor, userID, enabled)
}
