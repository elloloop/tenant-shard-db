// SPDX-License-Identifier: AGPL-3.0-only

package apply

import "github.com/elloloop/tenant-shard-db/server/go/internal/wal"

// Status mirrors the receipt status the Python applier returns from
// ExecuteAtomic (see docs/go-port/shared/applier.md "Receipt
// construction"). APPLIED means the event committed; SKIPPED means the
// idempotency-key probe inside the txn matched a prior row;
// FAILED is the catch-all error path.
type Status uint8

const (
	// StatusApplied is the happy-path receipt status.
	StatusApplied Status = iota
	// StatusSkipped means the idempotency-key was already recorded.
	StatusSkipped
	// StatusFailed means the apply errored. Result.Err carries the cause.
	StatusFailed
)

// String returns the wire form ("APPLIED" / "SKIPPED" / "FAILED").
// Mirrors the Python ApplyResult.status enum values.
func (s Status) String() string {
	switch s {
	case StatusApplied:
		return "APPLIED"
	case StatusSkipped:
		return "SKIPPED"
	case StatusFailed:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

// Result is the outcome of applying one event. Mirrors the Python
// ApplyResult struct at server/python/entdb_server/apply/applier.py:272.
//
// Position is the WAL position the event came from; on a successful
// apply this is the receipt the SDK returns to clients.
//
// CreatedNodes / CreatedEdges / DeletedNodeIDs let the post-commit
// fan-out hook (mailbox notifications, shared-index maintenance) act on
// what changed without re-reading the database.
type Result struct {
	Position       wal.StreamPos
	Status         Status
	Err            error
	TenantID       string
	CreatedNodes   []string
	CreatedEdges   []EdgeRef
	DeletedNodeIDs []string

	// SharedAdded / SharedRemoved record the (user_id, source_tenant,
	// node_id) tuples whose grants changed in this event. Best-effort
	// post-commit; consumed by shared-index maintenance.
	SharedAdded   []SharedRef
	SharedRemoved []SharedRef
}

// EdgeRef is the in-result representation of a created edge. Used for
// post-commit notifications.
type EdgeRef struct {
	EdgeTypeID int32
	FromNodeID string
	ToNodeID   string
}

// SharedRef identifies one shared_index hint row. Cross-tenant grants
// produce these; same-tenant grants do not.
type SharedRef struct {
	UserID       string
	SourceTenant string
	NodeID       string
	Permission   string
}
