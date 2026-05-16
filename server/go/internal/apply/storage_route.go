// SPDX-License-Identifier: AGPL-3.0-only

package apply

// storageMode classifies an op by which physical store(s) it writes.
// Used by the post-commit fan-out hook (shared_index maintenance) to
// decide whether to update globalstore alongside the per-tenant write.
//
// In every per-tenant write also goes through the canonical
// store; the only meaningful split is whether the op also produces a
// shared_index hint or a globalstore membership change.
type storageMode uint8

const (
	// storageCanonical: writes only the per-tenant SQLite (canonical_store).
	storageCanonical storageMode = iota
	// storageCanonicalAndShared: writes per-tenant SQLite AND maintains
	// the global_store.shared_index hint (cross-tenant share/revoke).
	storageCanonicalAndShared
	// storageGlobalOnly: writes globalstore only (tenant membership ops).
	storageGlobalOnly
)

// classifyOp returns the storage mode for a single op. The applier
// dispatches every op against the canonical store first; this hint just
// drives post-commit fan-out side effects.
func classifyOp(op map[string]any) storageMode {
	switch opTypeOf(op) {
	case OpShareNode, OpRevokeAccess, OpDelegateAccess, OpAdminRevokeAccess:
		return storageCanonicalAndShared
	case OpAddTenantMember, OpRemoveTenantMember, OpChangeMemberRole, OpSetLegalHold:
		return storageGlobalOnly
	default:
		return storageCanonical
	}
}
