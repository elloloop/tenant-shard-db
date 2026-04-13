// Package entdb provides a Go client SDK for the EntDB multi-tenant graph database.
package entdb

import (
	"fmt"
	"strconv"

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
