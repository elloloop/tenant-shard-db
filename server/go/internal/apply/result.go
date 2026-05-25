// SPDX-License-Identifier: AGPL-3.0-only

package apply

import "github.com/elloloop/tenant-shard-db/server/go/internal/wal"

// Status is the receipt status returned from ExecuteAtomic (see
// docs/go-port/shared/applier.md "Receipt construction"). APPLIED
// means the event committed; SKIPPED means the idempotency-key probe
// inside the txn matched a prior row; FAILED is the catch-all error
// path; FAILED_PRECONDITION is the CAS-miss path that aborts the
// batch but advances the WAL offset.
type Status uint8

const (
	// StatusApplied is the happy-path receipt status.
	StatusApplied Status = iota
	// StatusSkipped means the idempotency-key was already recorded.
	StatusSkipped
	// StatusFailed means the apply errored. Result.Err carries the cause.
	StatusFailed
	// StatusFailedPrecondition means a conditional UpdateNodeOp's
	// precondition did not match observed state. Result.Precondition
	// carries the typed failure detail. See GitHub issue #500.
	StatusFailedPrecondition
	// StatusUniqueViolation means a create/update op tripped a declared
	// single-field or composite unique constraint. Result.UniqueViolation
	// carries the structured ALREADY_EXISTS detail. Like
	// FAILED_PRECONDITION it aborts the batch but advances the WAL
	// offset (deterministic outcome). See issue #566.
	StatusUniqueViolation
)

// String returns the wire form ("APPLIED" / "SKIPPED" / "FAILED" /
// "FAILED_PRECONDITION").
func (s Status) String() string {
	switch s {
	case StatusApplied:
		return "APPLIED"
	case StatusSkipped:
		return "SKIPPED"
	case StatusFailed:
		return "FAILED"
	case StatusFailedPrecondition:
		return "FAILED_PRECONDITION"
	case StatusUniqueViolation:
		return "UNIQUE_VIOLATION"
	default:
		return "UNKNOWN"
	}
}

// Result is the outcome of applying one event.
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

	// Precondition carries the typed failure detail when Status is
	// StatusFailedPrecondition. Lifted into the gRPC
	// ExecuteAtomicResponse.precondition_failure by the handler when
	// wait_applied=true; otherwise served from the idempotency cache
	// via GetReceiptStatus.
	Precondition *PreconditionFailure

	// UniqueViolation carries the structured ALREADY_EXISTS detail when
	// Status is StatusUniqueViolation. The handler lifts it into a gRPC
	// ALREADY_EXISTS status; the idempotency cache replays it on retry.
	UniqueViolation *UniqueViolation
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
