// Exception-class-name -> gRPC code translation table.
//
// Source of truth: docs/go-port/shared/error-mapping.md, "Exception -> code
// mapping".
//
// The applier and store packages return errors that this package can classify
// via FromPythonException (string-keyed for the in-band path) and via
// FromGoError (sentinel-keyed). Handlers should prefer building errors with
// errs.Errorf or returning a sentinel directly; this map exists for the
// applier's swallow / re-raise decision and for cross-language contract
// tests that replay error fixtures by class name.

package errs

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// pythonExceptionCodes is the canonical exception-class-name -> gRPC code
// table. Keys are the class names exactly as they appear in the source
// (`raise FooError(...)`); values are the gRPC code the server eventually
// emits. See error-mapping.md for the full catalogue.
var pythonExceptionCodes = map[string]codes.Code{
	// AccessDeniedError -> PERMISSION_DENIED.
	"AccessDeniedError": codes.PermissionDenied,

	// TenantNotFoundError -> NOT_FOUND
	// (via _check_tenant existence-leak guard).
	"TenantNotFoundError": codes.NotFound,

	// IdempotencyViolationError -> ALREADY_EXISTS.
	"IdempotencyViolationError": codes.AlreadyExists,

	// UniqueConstraintError -> ALREADY_EXISTS.
	"UniqueConstraintError": codes.AlreadyExists,

	// CompositeUniqueConstraintError -> ALREADY_EXISTS.
	"CompositeUniqueConstraintError": codes.AlreadyExists,

	// QueryFilterError -> INVALID_ARGUMENT.
	"QueryFilterError": codes.InvalidArgument,

	// CompatibilityError -> FAILED_PRECONDITION
	// (when surfaced through admin path).
	"CompatibilityError": codes.FailedPrecondition,

	// RegistryFrozenError -> FAILED_PRECONDITION.
	"RegistryFrozenError": codes.FailedPrecondition,

	// DuplicateRegistrationError -> FAILED_PRECONDITION.
	"DuplicateRegistrationError": codes.FailedPrecondition,

	// AuthenticationError -> UNAUTHENTICATED (via auth interceptor).
	"AuthenticationError": codes.Unauthenticated,

	// SessionError -> UNAUTHENTICATED.
	"SessionError": codes.Unauthenticated,

	// ApiKeyError -> UNAUTHENTICATED.
	"ApiKeyError": codes.Unauthenticated,

	// AuthError -> UNAUTHENTICATED.
	"AuthError": codes.Unauthenticated,

	// CryptoShreddedError -> FAILED_PRECONDITION (tenant crypto-shredded; permanent).
	"CryptoShreddedError": codes.FailedPrecondition,

	// TenantShreddedError -> FAILED_PRECONDITION.
	"TenantShreddedError": codes.FailedPrecondition,

	// TenantKeyAlreadyProvisionedError -> ALREADY_EXISTS.
	"TenantKeyAlreadyProvisionedError": codes.AlreadyExists,

	// WalConnectionError. Swallowed -> OK with empty/error response. The
	// Go port may upgrade to UNAVAILABLE behind a feature flag; until
	// then this row documents the status. See error-mapping.md row.
	"WalConnectionError": codes.Unavailable,

	// WalTimeoutError. Same swallow note as WalConnectionError.
	"WalTimeoutError": codes.Unavailable,

	// WalSerializationError. Reported as in-band INTERNAL on
	// ExecuteAtomicResponse.error_code.
	"WalSerializationError": codes.Internal,

	// SchemaValidationError reported in-band on ExecuteAtomicResponse,
	// NOT a gRPC code. Mapped to InvalidArgument for cross-language
	// contract tests that drive this path outside ExecuteAtomic.
	"SchemaValidationError": codes.InvalidArgument,

	// ValidationError / SchemaFingerprintMismatch. Reported in-band on
	// ExecuteAtomicResponse. Same note as SchemaValidationError.
	"ValidationError":           codes.InvalidArgument,
	"SchemaFingerprintMismatch": codes.FailedPrecondition,

	// PermissionError (builtin) swallowed inside admin paths.
	// Route to PermissionDenied.
	"PermissionError": codes.PermissionDenied,
}

// FromPythonException returns the gRPC code the server emits (or would emit,
// modulo swallow patterns) for the given exception class name. Returns
// codes.Unknown if the class name is not in the table; callers should treat
// Unknown as "swallow / in-band".
//
// Cross-language contract tests use this to replay error fixtures by class
// name: the test harness sends the exception class name over the wire and
// the server's classifier produces the corresponding status code.
func FromPythonException(name string) codes.Code {
	if c, ok := pythonExceptionCodes[name]; ok {
		return c
	}
	return codes.Unknown
}

// FromGoError unwraps any error to its gRPC code. This is the inverse of
// Errorf and the entry point for the swallow chokepoint -- handlers should
// route through this rather than calling status.Code directly so the
// chokepoint can be augmented later (metric labels, logging, etc.) without
// touching every handler.
//
// Behavior:
//   - nil -> codes.OK
//   - *codeError -> the embedded code
//   - status.Status-bearing error -> the embedded code
//   - anything else -> codes.Unknown
func FromGoError(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	return status.Code(err)
}
