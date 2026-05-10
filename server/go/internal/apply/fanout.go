// SPDX-License-Identifier: AGPL-3.0-only

package apply

import "context"

// fanout runs best-effort post-commit notifications. Mirrors
// applier.py:_fanout_node (1615-1668) and the broader
// "notifications + shared_index" hooks at applier.py:1361-1368.
//
// Per docs/go-port/shared/applier.md "Open questions / risks" item 8,
// fanout intentionally runs OUTSIDE the per-tenant transaction. A crash
// between commit and fanout drops notifications — accepted because
// notifications are best-effort and the canonical write already
// succeeded. The Go port preserves this trade-off.
//
// Wave 1 keeps notifications a stub (server/python/entdb_server has
// already deprecated the per-user mailbox tables; see
// store/notifications.go). The hook exists so Wave-2 RPCs that grow
// real notification semantics have a single place to plug in.
func (a *Applier) fanout(ctx context.Context, ev *Event, res *Result) {
	if a.fanoutHook != nil {
		a.fanoutHook(ctx, ev, res)
	}
	// shared_index maintenance is the only mandatory post-commit hook
	// today. It belongs here (and not inside the txn) for the same
	// reason as notification fanout: best-effort against a different
	// physical store, must not block the per-tenant write.
	a.maintainSharedIndex(ctx, res)
}
