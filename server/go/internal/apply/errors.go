// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"errors"
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

// Sentinel errors returned by the applier. Each wraps an internal/errs
// sentinel so the gRPC layer maps them to the appropriate status codes
// (see docs/go-port/shared/error-mapping.md).
var (
	// ErrPoisonEvent signals an event the applier refuses to apply
	// because it is structurally malformed (missing required fields,
	// unknown op-type, etc.). Halts the consumer.
	ErrPoisonEvent = fmt.Errorf("%w: poison event", errs.ErrInvalidArgument)

	// ErrUnknownOpType signals an op-type the applier does not know
	// about. A typo in a handler that raced ahead of the applier is the
	// usual cause; failing closed avoids silent data loss.
	ErrUnknownOpType = fmt.Errorf("%w: unknown op type", errs.ErrInvalidArgument)

	// ErrApplierClosed signals Run was called on a stopped applier (or
	// Stop was called and Run subsequently returned).
	ErrApplierClosed = fmt.Errorf("%w: applier closed", errs.ErrFailedPrecondition)

	// ErrPreconditionFailed is the sentinel returned when a conditional
	// UpdateNodeOp precondition did not match observed state. Unlike
	// ErrPoisonEvent this is an EXPECTED outcome of CAS — the applier
	// aborts the batch, memoizes the failure in the idempotency cache,
	// and advances the WAL offset (no halt). The handler converts the
	// sentinel to a typed gRPC FailedPrecondition response carrying the
	// op_index / field / expected / observed coordinates via the typed
	// wrapper below. See GitHub issue #500.
	ErrPreconditionFailed = fmt.Errorf("%w: precondition failed", errs.ErrFailedPrecondition)

	// ErrSchemaConflict is the sentinel returned when a register_schema
	// op (SELF-DESCRIBING WRITES) supplies a type whose identity
	// (type_id or name) is already registered with a DIFFERENT
	// definition. Establish-or-reject: a type absent is registered, a
	// type identical is a no-op, a type that conflicts is rejected. Like
	// ErrPreconditionFailed this is an EXPECTED, DETERMINISTIC outcome
	// (re-applying the same event against the same registry always
	// reproduces it), NOT a poison: the applier aborts the batch,
	// memoizes the conflict in the idempotency cache with status
	// FAILED_PRECONDITION, and advances the WAL offset without halting.
	// Online evolution / ALTER is out of scope, so a conflicting
	// redefinition never silently overwrites the registered type.
	ErrSchemaConflict = fmt.Errorf("%w: schema conflict", errs.ErrFailedPrecondition)

	// ErrUniqueViolation is the sentinel returned when a create/update
	// op trips a declared single-field or composite unique constraint.
	// Like ErrPreconditionFailed this is an EXPECTED, deterministic
	// outcome (re-applying the same event against the same materialised
	// state always reproduces it), NOT a poison: the applier aborts the
	// batch, memoizes the structured detail in the idempotency cache
	// with status UNIQUE_VIOLATION, and advances the WAL offset without
	// halting. The ExecuteAtomic handler lifts the memoized detail into
	// a gRPC ALREADY_EXISTS status so the SDKs parse it as a typed
	// UniqueConstraintError. See issue #566 and the composite-unique ADR.
	ErrUniqueViolation = fmt.Errorf("%w: unique constraint violation", errs.ErrAlreadyExists)
)

// UniqueViolation is the typed wrapper carried alongside
// ErrUniqueViolation. Detail is the fully-formatted ALREADY_EXISTS
// string the store built (see store.BuildUniqueViolationDetail) — it is
// memoized verbatim and replayed to the client, so the wire format is
// owned entirely by the store and this struct just transports it.
type UniqueViolation struct {
	// Detail is the structured ALREADY_EXISTS message the SDK parsers
	// consume. Format is pinned by the SDK contract tests.
	Detail string
}

// Error implements error.
func (e *UniqueViolation) Error() string { return "entdb: " + e.Detail }

// Unwrap allows errors.Is(err, ErrUniqueViolation) on the typed wrapper.
func (e *UniqueViolation) Unwrap() error { return ErrUniqueViolation }

// AsUniqueViolation extracts a *UniqueViolation from err via errors.As,
// returning nil when the chain has no such value.
func AsUniqueViolation(err error) *UniqueViolation {
	var uv *UniqueViolation
	if errors.As(err, &uv) {
		return uv
	}
	return nil
}

// PreconditionFailure is the typed wrapper carried alongside
// ErrPreconditionFailed. It captures the coordinates of a CAS miss in a
// form the gRPC handler can lift directly into the wire
// PreconditionFailure proto message.
//
// The struct deliberately uses any for Expected / Observed — the
// applier reads the patch payload as JSON-decoded map[string]any so
// the values are already in the same shape the structpb encoder
// expects on the egress side. JSON-canonical comparison (see
// preconditionMatches in ops_update_node.go) keeps numeric and string
// equality consistent across JSON round-trips.
type PreconditionFailure struct {
	OpIndex  int
	Field    string
	Expected any
	Observed any
	// FieldPresent is false when the node payload had no key for the
	// resolved field_id. Distinguishes "field missing" from "field
	// stored as JSON null" so the typed error in the SDK can carry the
	// distinction without re-fetching state.
	FieldPresent bool
}

// Error implements error. The message is intentionally compact —
// callers that need structured detail unwrap via errors.As.
func (e *PreconditionFailure) Error() string {
	return fmt.Sprintf(
		"entdb: precondition failed at op_index=%d field=%q expected=%v observed=%v",
		e.OpIndex, e.Field, e.Expected, e.Observed,
	)
}

// Unwrap allows errors.Is(err, ErrPreconditionFailed) on a typed
// failure wrapper.
func (e *PreconditionFailure) Unwrap() error { return ErrPreconditionFailed }

// AsPreconditionFailure extracts a *PreconditionFailure from err via
// errors.As, returning nil when the chain has no such value.
func AsPreconditionFailure(err error) *PreconditionFailure {
	var pf *PreconditionFailure
	if errors.As(err, &pf) {
		return pf
	}
	return nil
}

// SchemaConflict is the typed wrapper carried alongside
// ErrSchemaConflict. Detail is a human-readable description of the
// conflicting type identity (which type_id / name disagreed). It is
// memoized verbatim so a same-idempotency-key retry replays the
// identical failure.
type SchemaConflict struct {
	Detail string
}

// Error implements error.
func (e *SchemaConflict) Error() string { return "entdb: " + e.Detail }

// Unwrap allows errors.Is(err, ErrSchemaConflict) on the typed wrapper.
func (e *SchemaConflict) Unwrap() error { return ErrSchemaConflict }

// AsSchemaConflict extracts a *SchemaConflict from err via errors.As,
// returning nil when the chain has no such value.
func AsSchemaConflict(err error) *SchemaConflict {
	var sc *SchemaConflict
	if errors.As(err, &sc) {
		return sc
	}
	return nil
}
