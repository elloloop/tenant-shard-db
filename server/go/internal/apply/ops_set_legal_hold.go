// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"fmt"
)

// applySetLegalHold dispatches a "set_legal_hold" op. PLAN.md §6.1
// borderline case: the legal-hold flag lives in globalstore but is
// consulted on per-tenant writes. Recording the event in the WAL keeps
// the audit history reconstructible even if globalstore is restored
// from a different snapshot.
//
// The applier writes the legal_holds row in globalstore (post-commit
// of the per-tenant txn — the per-tenant txn has nothing to do for
// this op, but we still write the applied_events row inside it for
// idempotency parity). Best-effort against globalstore: failures are
// surfaced as a halt-on-poison error.
//
// op shape:
//
//	{
//	  "op":      "set_legal_hold",
//	  "held_by": "user:...",
//	  "reason":  "...",
//	  "clear":   bool, // true => ClearLegalHold
//	}
func (a *Applier) applySetLegalHold(ctx context.Context, ev *Event, op map[string]any) error {
	if a.global == nil {
		// Global store not wired; skip silently. This is the same
		// trade-off the Python server makes — if globalstore isn't
		// available, set_legal_hold is a no-op.
		return nil
	}
	heldBy := stringField(op, "held_by")
	if heldBy == "" {
		return fmt.Errorf("%w: set_legal_hold missing held_by", ErrPoisonEvent)
	}
	if boolField(op, "clear") {
		if _, err := a.global.ClearLegalHold(ctx, ev.TenantID, heldBy); err != nil {
			return fmt.Errorf("apply set_legal_hold: clear: %w", err)
		}
		return nil
	}
	reason := stringField(op, "reason")
	if _, err := a.global.SetLegalHold(ctx, ev.TenantID, heldBy, reason); err != nil {
		return fmt.Errorf("apply set_legal_hold: set: %w", err)
	}
	return nil
}
