// Package store implements the per-tenant SQLite canonical store
// (materialized view of the WAL) for the Go server reimplementation.
//
// Spec: docs/go-port/shared/canonical-store.md.
//
// CLAUDE.md invariants honoured here:
//
//   - #4 per-tenant SQLite isolation: one *sql.DB per tenant_id, never
//     read/write across tenants in a single SQLite txn.
//   - #6 field_id payloads on disk: payload JSON is keyed by field_id
//     integers; translation lives in the payload package.
//
// This package only implements *data access*. ACL evaluation lives in
// the acl package (W1.9); applier orchestration lives in apply (W1.10).
package store

import (
	"errors"
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

// Sentinel errors returned by store methods. Each wraps an internal/errs
// sentinel so the gRPC layer maps them to the same codes the Python
// server emits.
var (
	// ErrNodeNotFound is returned when GetNode / UpdateNode / DeleteNode
	// targets a node that does not exist. Wraps errs.ErrNotFound.
	ErrNodeNotFound = fmt.Errorf("%w: node not found", errs.ErrNotFound)

	// ErrEdgeNotFound is returned when DeleteEdge targets a missing edge.
	// Wraps errs.ErrNotFound.
	ErrEdgeNotFound = fmt.Errorf("%w: edge not found", errs.ErrNotFound)

	// ErrTenantNotOpen is returned by methods that require an opened
	// tenant DB when the tenant has not been opened yet. Wraps
	// errs.ErrFailedPrecondition.
	ErrTenantNotOpen = fmt.Errorf("%w: tenant not opened", errs.ErrFailedPrecondition)

	// ErrInvalidTenantID is returned when a tenant_id contains
	// filesystem-unsafe characters or exceeds the length limit. Mirrors
	// canonical_store.py:138-162. Wraps errs.ErrInvalidArgument.
	ErrInvalidTenantID = fmt.Errorf("%w: tenant_id must match [A-Za-z0-9_-]{1,128}", errs.ErrInvalidArgument)

	// ErrIdempotencyViolation is returned by RecordIdempotency when the
	// (tenant_id, request_id) tuple already exists. Wraps
	// errs.ErrAlreadyExists.
	ErrIdempotencyViolation = fmt.Errorf("%w: idempotency key already recorded", errs.ErrAlreadyExists)

	// ErrUniqueConstraint is returned when a unique-index INSERT collides
	// with an existing row. Wraps errs.ErrAlreadyExists.
	ErrUniqueConstraint = fmt.Errorf("%w: unique constraint violation", errs.ErrAlreadyExists)
)

// IsNotFound reports whether err (or anything it wraps) is one of the
// store NotFound sentinels.
func IsNotFound(err error) bool {
	return errors.Is(err, errs.ErrNotFound)
}
