// Package entdb provides a Go client SDK for the EntDB multi-tenant graph database.
package entdb

import "fmt"

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
