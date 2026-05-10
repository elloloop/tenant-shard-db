// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"errors"
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

// Tenant-membership ops. The Python server writes these directly to
// globalstore today (carve-out from CLAUDE.md invariant #1 documented
// in docs/go-port/shared/global-store.md). The Go applier optionally
// drives globalstore through the WAL too, so a stand-alone WAL replay
// can rebuild membership audit history.
//
// All three branches are no-ops when a.global is nil (globalstore not
// wired in this configuration).

// applyAddTenantMember dispatches an "add_tenant_member" op.
//
// op shape: {"op":"add_tenant_member","user_id":"<id>","role":"<role>"}
func (a *Applier) applyAddTenantMember(ctx context.Context, ev *Event, op map[string]any) error {
	if a.global == nil {
		return nil
	}
	userID := stringField(op, "user_id")
	if userID == "" {
		return fmt.Errorf("%w: add_tenant_member missing user_id", ErrPoisonEvent)
	}
	role := stringField(op, "role")
	if err := a.global.AddTenantMember(ctx, ev.TenantID, userID, role); err != nil {
		// AlreadyExists is idempotent under replay — ignore.
		if errors.Is(err, errs.ErrAlreadyExists) {
			return nil
		}
		return fmt.Errorf("apply add_tenant_member: %w", err)
	}
	return nil
}

// applyRemoveTenantMember dispatches a "remove_tenant_member" op.
//
// op shape: {"op":"remove_tenant_member","user_id":"<id>"}
func (a *Applier) applyRemoveTenantMember(ctx context.Context, ev *Event, op map[string]any) error {
	if a.global == nil {
		return nil
	}
	userID := stringField(op, "user_id")
	if userID == "" {
		return fmt.Errorf("%w: remove_tenant_member missing user_id", ErrPoisonEvent)
	}
	if _, err := a.global.RemoveTenantMember(ctx, ev.TenantID, userID); err != nil {
		return fmt.Errorf("apply remove_tenant_member: %w", err)
	}
	return nil
}

// applyChangeMemberRole dispatches a "change_member_role" op.
//
// op shape: {"op":"change_member_role","user_id":"<id>","role":"<role>"}
func (a *Applier) applyChangeMemberRole(ctx context.Context, ev *Event, op map[string]any) error {
	if a.global == nil {
		return nil
	}
	userID := stringField(op, "user_id")
	role := stringField(op, "role")
	if userID == "" || role == "" {
		return fmt.Errorf("%w: change_member_role missing user_id/role", ErrPoisonEvent)
	}
	if _, err := a.global.ChangeMemberRole(ctx, ev.TenantID, userID, role); err != nil {
		return fmt.Errorf("apply change_member_role: %w", err)
	}
	return nil
}
