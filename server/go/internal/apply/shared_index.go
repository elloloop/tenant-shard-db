// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
)

// maintainSharedIndex applies shared_index updates after a successful
// per-tenant commit.
//
// shared_index is a HINT — authoritative ACLs live in the per-tenant
// canonical store. Best-effort against globalstore: we log + continue
// rather than fail the apply (the per-tenant write already committed).
//
// Returns nil even on globalstore failure so callers don't mistake a
// best-effort hint sync for a hard error.
func (a *Applier) maintainSharedIndex(ctx context.Context, res *Result) {
	if a.global == nil {
		return
	}
	for _, s := range res.SharedAdded {
		_ = a.global.AddShared(ctx, s.UserID, s.SourceTenant, s.NodeID, s.Permission)
	}
	for _, s := range res.SharedRemoved {
		_, _ = a.global.RemoveShared(ctx, s.UserID, s.SourceTenant, s.NodeID)
	}
	if res.TenantID != "" {
		for _, nodeID := range res.DeletedNodeIDs {
			// On node delete every shared_index hint pointing at that
			// node becomes stale.
			_, _ = a.global.CleanupStaleShared(ctx, res.TenantID, nodeID)
		}
	}
}
