// Python exception -> gRPC code translation table.
//
// Source of truth: docs/go-port/shared/error-mapping.md, "Exception -> code
// mapping". Each row below carries a comment with the Python file:line where
// the original exception is raised; if the spec is updated, update both.
//
// The Go port does not re-implement every Python exception class -- the
// applier and store packages return errors that this package can classify
// via FromPythonException (string-keyed for the in-band path) and via
// FromGoError (sentinel-keyed). Handlers should prefer building errors with
// errs.Errorf or returning a sentinel directly; this map exists for the
// applier's swallow / re-raise decision and for cross-language contract
// tests that replay Python error fixtures.

package errs

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// pythonExceptionCodes is the canonical Python-class-name -> gRPC code
// table. Keys are the Python class names exactly as they appear in the
// source (`raise FooError(...)`); values are the gRPC code the Python
// handler eventually emits via context.abort.
//
// Keep this sorted by Python source file for ease of audit against
// error-mapping.md.
var pythonExceptionCodes = map[string]codes.Code{
	// apply/acl.py:44 -- AccessDeniedError -> PERMISSION_DENIED.
	"AccessDeniedError": codes.PermissionDenied,

	// apply/canonical_store.py:165 -- TenantNotFoundError -> NOT_FOUND
	// (via _check_tenant existence-leak guard).
	"TenantNotFoundError": codes.NotFound,

	// apply/canonical_store.py:171 -- IdempotencyViolationError ->
	// ALREADY_EXISTS.
	"IdempotencyViolationError": codes.AlreadyExists,

	// apply/applier.py:114 -- UniqueConstraintError -> ALREADY_EXISTS.
	"UniqueConstraintError": codes.AlreadyExists,

	// apply/applier.py:146 -- CompositeUniqueConstraintError ->
	// ALREADY_EXISTS.
	"CompositeUniqueConstraintError": codes.AlreadyExists,

	// apply/query_filter.py:60 -- QueryFilterError -> INVALID_ARGUMENT
	// (surfaces at api/grpc_server.py:1154-1155).
	"QueryFilterError": codes.InvalidArgument,

	// schema/compat.py:128 -- CompatibilityError -> FAILED_PRECONDITION
	// (when surfaced through admin path).
	"CompatibilityError": codes.FailedPrecondition,

	// schema/registry.py:51 -- RegistryFrozenError -> FAILED_PRECONDITION.
	"RegistryFrozenError": codes.FailedPrecondition,

	// schema/registry.py:57 -- DuplicateRegistrationError ->
	// FAILED_PRECONDITION.
	"DuplicateRegistrationError": codes.FailedPrecondition,

	// auth/oauth_validator.py:44 -- AuthenticationError -> UNAUTHENTICATED
	// (via auth interceptor).
	"AuthenticationError": codes.Unauthenticated,

	// auth/session_manager.py:36 -- SessionError -> UNAUTHENTICATED.
	"SessionError": codes.Unauthenticated,

	// auth/api_key_manager.py:41 -- ApiKeyError -> UNAUTHENTICATED.
	"ApiKeyError": codes.Unauthenticated,

	// api/jwt_auth.py:40 -- AuthError -> UNAUTHENTICATED.
	"AuthError": codes.Unauthenticated,

	// crypto/key_manager.py:59 -- CryptoShreddedError ->
	// FAILED_PRECONDITION (tenant has been crypto-shredded; permanent).
	"CryptoShreddedError": codes.FailedPrecondition,

	// crypto/tenant_key_vault.py:55 -- TenantShreddedError ->
	// FAILED_PRECONDITION.
	"TenantShreddedError": codes.FailedPrecondition,

	// crypto/tenant_key_vault.py:67 -- TenantKeyAlreadyProvisionedError ->
	// ALREADY_EXISTS.
	"TenantKeyAlreadyProvisionedError": codes.AlreadyExists,

	// wal/base.py:47 -- WalConnectionError. Python today swallows -> OK
	// with empty/error response. The Go port may upgrade to UNAVAILABLE
	// behind a feature flag; until then this row documents the Python
	// status, NOT the eventual Go status. See error-mapping.md row.
	"WalConnectionError": codes.Unavailable,

	// wal/base.py:53 -- WalTimeoutError. Same swallow note as
	// WalConnectionError.
	"WalTimeoutError": codes.Unavailable,

	// wal/base.py:59 -- WalSerializationError. Swallowed by Python; Go
	// reports as in-band INTERNAL on ExecuteAtomicResponse.error_code.
	"WalSerializationError": codes.Internal,

	// schema/validator.py:18 -- SchemaValidationError reported in-band on
	// ExecuteAtomicResponse, NOT a gRPC code. Mapped to InvalidArgument
	// here only for cross-language contract tests that drive this path
	// outside ExecuteAtomic.
	"SchemaValidationError": codes.InvalidArgument,

	// apply/applier.py:53,59 -- ValidationError /
	// SchemaFingerprintMismatch. Reported in-band on
	// ExecuteAtomicResponse, NOT a gRPC code. Same note as
	// SchemaValidationError.
	"ValidationError":           codes.InvalidArgument,
	"SchemaFingerprintMismatch": codes.FailedPrecondition,

	// api/grpc_server.py:1791,1849 -- Python builtin PermissionError
	// swallowed inside admin paths. Go: route to PermissionDenied.
	"PermissionError": codes.PermissionDenied,
}

// FromPythonException returns the gRPC code that the Python server emits
// (or would emit, modulo swallow patterns) for the given Python exception
// class name. Returns codes.Unknown if the class name is not in the table;
// callers should treat Unknown as "swallow / in-band" -- mirroring the
// Python broad-except behavior.
//
// Cross-language contract tests use this to replay Python error fixtures:
// the test harness sends the exception class name over the wire and the Go
// server's classifier produces the same status code the Python server
// would have produced.
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
