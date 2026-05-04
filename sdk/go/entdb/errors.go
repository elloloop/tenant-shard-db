// Package entdb provides a Go client SDK for the EntDB multi-tenant graph database.
package entdb

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// EntDBError is the base error type for all EntDB SDK errors.
type EntDBError struct {
	Message string
	Code    string
	Details map[string]any
}

func (e *EntDBError) Error() string {
	return fmt.Sprintf("entdb %s: %s", e.Code, e.Message)
}

// ConnectionError indicates a failure to connect to the EntDB server.
type ConnectionError struct {
	EntDBError
	Address string
}

// NewConnectionError creates a new ConnectionError.
func NewConnectionError(message, address string) *ConnectionError {
	return &ConnectionError{
		EntDBError: EntDBError{
			Message: message,
			Code:    "CONNECTION_ERROR",
			Details: map[string]any{"address": address},
		},
		Address: address,
	}
}

// ValidationError indicates a payload validation failure.
type ValidationError struct {
	EntDBError
	FieldName string
	Errors    []string
}

// NewValidationError creates a new ValidationError.
func NewValidationError(message string, fieldName string, errors []string) *ValidationError {
	return &ValidationError{
		EntDBError: EntDBError{
			Message: message,
			Code:    "VALIDATION_ERROR",
			Details: map[string]any{"field": fieldName, "errors": errors},
		},
		FieldName: fieldName,
		Errors:    errors,
	}
}

// NotFoundError indicates a missing resource.
type NotFoundError struct {
	EntDBError
	ResourceType string
	ResourceID   string
}

// NewNotFoundError creates a new NotFoundError.
func NewNotFoundError(message, resourceType, resourceID string) *NotFoundError {
	return &NotFoundError{
		EntDBError: EntDBError{
			Message: message,
			Code:    "NOT_FOUND",
			Details: map[string]any{
				"resource_type": resourceType,
				"resource_id":   resourceID,
			},
		},
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}
}

// AccessDeniedError indicates an authorization failure.
type AccessDeniedError struct {
	EntDBError
	Actor              string
	ResourceID         string
	RequiredPermission string
}

// NewAccessDeniedError creates a new AccessDeniedError.
func NewAccessDeniedError(message, actor, resourceID, requiredPermission string) *AccessDeniedError {
	return &AccessDeniedError{
		EntDBError: EntDBError{
			Message: message,
			Code:    "ACCESS_DENIED",
			Details: map[string]any{
				"actor":               actor,
				"resource_id":         resourceID,
				"required_permission": requiredPermission,
			},
		},
		Actor:              actor,
		ResourceID:         resourceID,
		RequiredPermission: requiredPermission,
	}
}

// TransactionError indicates an atomic transaction failure.
type TransactionError struct {
	EntDBError
	IdempotencyKey string
}

// NewTransactionError creates a new TransactionError.
func NewTransactionError(message, idempotencyKey string) *TransactionError {
	return &TransactionError{
		EntDBError: EntDBError{
			Message: message,
			Code:    "TRANSACTION_ERROR",
			Details: map[string]any{"idempotency_key": idempotencyKey},
		},
		IdempotencyKey: idempotencyKey,
	}
}

// RateLimitError indicates the caller was throttled by one of the three
// quota layers (monthly write quota, per-tenant RPS, per-user RPS). It
// mirrors the Python SDK's RateLimitError so callers can write a single
// handler that covers all three phases of the rate-limit model frozen in
// docs/decisions/quotas.md.
//
// The server sends:
//   - gRPC status code RESOURCE_EXHAUSTED
//   - a "retry-after" trailing metadata entry (seconds, decimal string)
//   - a human-readable status message
//
// Callers should treat a non-zero RetryAfterMs as a hint — the underlying
// reason may be a burst-RPS throttle (sub-second wait) or a monthly quota
// exhaustion (wait until the next calendar month). The typical retry
// strategy is exponential backoff capped at RetryAfterMs.
type RateLimitError struct {
	EntDBError
	// RetryAfterMs is the server-suggested wait in milliseconds before
	// retrying. Zero when the server did not provide one.
	RetryAfterMs int64
	// Limit is the quota limit that was hit (monthly writes, or RPS).
	// Zero when the server did not surface a numeric limit.
	Limit int64
	// Used is the consumed amount in the current period. Zero when the
	// server did not surface a numeric usage count.
	Used int64
}

// Error implements the error interface.
func (e *RateLimitError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = "rate limit exceeded"
	}
	if e.RetryAfterMs > 0 {
		return fmt.Sprintf("entdb %s: %s (retry after %dms)", e.Code, msg, e.RetryAfterMs)
	}
	return fmt.Sprintf("entdb %s: %s", e.Code, msg)
}

// NewRateLimitError constructs a RateLimitError with the given message
// and server-suggested retry window (in milliseconds).
func NewRateLimitError(message string, retryAfterMs int64) *RateLimitError {
	return &RateLimitError{
		EntDBError: EntDBError{
			Message: message,
			Code:    "RATE_LIMITED",
			Details: map[string]any{"retry_after_ms": retryAfterMs},
		},
		RetryAfterMs: retryAfterMs,
	}
}

// parseRateLimitFromStatus converts a gRPC status + trailing metadata
// pair into a RateLimitError when the status code is RESOURCE_EXHAUSTED.
// Returns nil for other status codes so callers can chain-check their
// error mapping. The "retry-after" metadata key is parsed as a decimal
// integer number of seconds (matching the server's QuotaInterceptor)
// and exposed on the error as milliseconds so the Go SDK's retry
// backoff math can stay in a single unit.
func parseRateLimitFromStatus(err error, trailer metadata.MD) *RateLimitError {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.ResourceExhausted {
		return nil
	}
	var retryAfterMs int64
	if values := trailer.Get("retry-after"); len(values) > 0 {
		if secs, parseErr := strconv.ParseInt(values[0], 10, 64); parseErr == nil && secs >= 0 {
			retryAfterMs = secs * 1000
		}
	}
	return NewRateLimitError(st.Message(), retryAfterMs)
}

// UniqueConstraintError indicates that a create/update operation
// violated a declared unique field (2026-04-14 SDK v0.3 decision,
// supersedes 2026-04-13 unique_keys). The server surfaces this via
// gRPC ALREADY_EXISTS, with a status message identifying the
// colliding field_id + value. parseUniqueConstraintFromStatus turns
// those responses into a typed error so callers can write
//
//	if uce, ok := err.(*entdb.UniqueConstraintError); ok {
//	    // fetch the existing node via entdb.GetByKey(ctx, scope, ...)
//	}
//
// Fields mirror the server-side payload:
//
//   - TypeID / FieldID identify the (type, unique field) pair.
//   - Value holds the colliding value as the Go scalar the caller
//     originally passed through the wire.
type UniqueConstraintError struct {
	EntDBError
	TenantID string
	TypeID   int32
	FieldID  int32
	Value    any
	// ConstraintName is non-empty when the violation came from a
	// composite (multi-field) `(entdb.node).composite_unique`
	// constraint. Single-field violations leave it empty and use
	// `FieldID` / `Value` instead.
	ConstraintName string
	// FieldIDs / Values mirror the composite constraint coordinates.
	// Both are nil for single-field violations.
	FieldIDs []int32
	Values   []any
}

// IsComposite reports whether this is a composite (multi-field)
// unique-constraint violation as opposed to a single-field one.
func (e *UniqueConstraintError) IsComposite() bool {
	return e != nil && e.ConstraintName != ""
}

// NewUniqueConstraintError builds a typed UniqueConstraintError from
// the colliding-field coordinates. The message is set from the
// arguments so callers can log it directly.
func NewUniqueConstraintError(tenantID string, typeID, fieldID int32, value any) *UniqueConstraintError {
	msg := fmt.Sprintf(
		"unique constraint violation: tenant=%s type_id=%d field_id=%d value=%v already exists",
		tenantID, typeID, fieldID, value,
	)
	return &UniqueConstraintError{
		EntDBError: EntDBError{
			Message: msg,
			Code:    "UNIQUE_CONSTRAINT",
			Details: map[string]any{
				"tenant_id": tenantID,
				"type_id":   typeID,
				"field_id":  fieldID,
				"value":     value,
			},
		},
		TenantID: tenantID,
		TypeID:   typeID,
		FieldID:  fieldID,
		Value:    value,
	}
}

// Error implements the error interface. It prefers the server-provided
// status message when one is available, falling back to the typed
// coordinates so the message remains informative.
func (e *UniqueConstraintError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("entdb %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf(
		"entdb %s: tenant=%s type_id=%d field_id=%d value=%v already exists",
		e.Code, e.TenantID, e.TypeID, e.FieldID, e.Value,
	)
}

// parseUniqueConstraintFromStatus converts a gRPC status into a
// typed UniqueConstraintError when the status code is ALREADY_EXISTS.
// Returns nil for other status codes so callers can chain-check
// their error mapping. The server's status message is preserved on
// the typed error so logs stay useful.
//
// The structured “(type_id, field_id, value)“ payload is carried
// on the gRPC status details. When present we decode it into the
// typed fields; otherwise we fall back to a message-only error
// (still typed, just with zero coordinates) so callers can keep
// using “errors.As“.
func parseUniqueConstraintFromStatus(err error, tenantID string) *UniqueConstraintError {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.AlreadyExists {
		return nil
	}
	msg := st.Message()
	uce := &UniqueConstraintError{
		EntDBError: EntDBError{
			Message: msg,
			Code:    "UNIQUE_CONSTRAINT",
			Details: map[string]any{"tenant_id": tenantID},
		},
		TenantID: tenantID,
	}
	if cname, fids, vals := parseCompositeUniqueDetail(msg); cname != "" {
		uce.ConstraintName = cname
		uce.FieldIDs = fids
		uce.Values = vals
		uce.Details["constraint_name"] = cname
		uce.Details["field_ids"] = fids
		uce.Details["values"] = vals
		if tid := parseTypeIDFromDetail(msg); tid != 0 {
			uce.TypeID = tid
			uce.Details["type_id"] = tid
		}
	}
	return uce
}

// compositeUniqueDetailRE matches the server's composite-unique
// ALREADY_EXISTS detail string. The message format is::
//
//	Composite unique constraint violation: type_id=<int>
//	constraint='<name>' fields=[<int>, ...] values=[<repr>, ...]
//	already exists
//
// Single-field violations use a different format (``field_id=<int>``)
// and are intentionally left for future structured-error work.
var compositeUniqueDetailRE = regexp.MustCompile(
	`type_id=(\d+)\s+constraint=([^\s]+)\s+fields=\[([^\]]*)\]\s+values=\[(.*)\]\s+already exists`,
)

func parseCompositeUniqueDetail(detail string) (string, []int32, []any) {
	if detail == "" {
		return "", nil, nil
	}
	m := compositeUniqueDetailRE.FindStringSubmatch(detail)
	if m == nil {
		return "", nil, nil
	}
	rawConstraint := strings.Trim(m[2], "'\"")
	fids := parseIntList(m[3])
	// Values are best-effort: we expose the raw textual repr so
	// callers can log it; we don't attempt to round-trip arbitrary
	// Python repr() into Go scalars.
	vals := splitRepeatedRepr(m[4])
	return rawConstraint, fids, vals
}

func parseTypeIDFromDetail(detail string) int32 {
	re := regexp.MustCompile(`type_id=(\d+)`)
	m := re.FindStringSubmatch(detail)
	if m == nil {
		return 0
	}
	v, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return int32(v)
}

func parseIntList(s string) []int32 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]int32, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		out = append(out, int32(v))
	}
	return out
}

func splitRepeatedRepr(s string) []any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Naive split — values are repr()'d Python scalars (strings
	// quoted, numbers bare). For the typed-error surface we only
	// need them for diagnostic display; callers that need the
	// original Go-typed values should look at the corresponding
	// field on the original payload.
	parts := strings.Split(s, ",")
	out := make([]any, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// SchemaError indicates a schema-related problem.
type SchemaError struct {
	EntDBError
	ExpectedFingerprint string
	ActualFingerprint   string
}

// NewSchemaError creates a new SchemaError.
func NewSchemaError(message, expected, actual string) *SchemaError {
	return &SchemaError{
		EntDBError: EntDBError{
			Message: message,
			Code:    "SCHEMA_ERROR",
			Details: map[string]any{
				"expected_fingerprint": expected,
				"actual_fingerprint":   actual,
			},
		},
		ExpectedFingerprint: expected,
		ActualFingerprint:   actual,
	}
}
