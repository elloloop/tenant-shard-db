// Package errs is the typed-error chokepoint for the Go gRPC server.
//
// Every handler that needs to emit a gRPC status code MUST do so through this
// package, so the Python -> Go contract stays in one place. The exported
// sentinels each implement GRPCStatus() *status.Status, so they round-trip
// through google.golang.org/grpc/status.FromError and
// google.golang.org/grpc/status.Code without the caller needing to know the
// concrete type.
//
// Spec: docs/go-port/shared/error-mapping.md (canonical mapping of every
// status code emitted by the server). Per-RPC specs in
// docs/go-port/rpcs/*.md reference this file for code semantics; do not
// duplicate the mapping there.
//
// Out-of-scope (deliberate parity with Python):
//
//   - google.rpc.ErrorInfo / google.rpc.RetryInfo proto details. The Python
//     server does not emit these; adding them would de-sync the contract.
//   - codes.Unknown. If a contract test ever observes Unknown it is a P0
//     bug -- see error-mapping.md "Open questions / risks" item 4.
package errs

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// codeError is the concrete sentinel type returned by newCode. It carries a
// gRPC code plus an optional message and implements GRPCStatus so
// status.Code(err) and status.FromError(err) work without type-asserting on
// the concrete type.
//
// errors.Is(err, sentinel) returns true for any codeError that shares the
// same code as sentinel -- this lets handlers wrap a sentinel with a
// formatted message via Errorf or fmt.Errorf("...: %w", ErrFoo) and still
// match the sentinel in tests.
type codeError struct {
	code codes.Code
	msg  string
}

// newCode constructs a sentinel with no message. Used at package level.
func newCode(c codes.Code) *codeError {
	return &codeError{code: c}
}

// Error implements error. Empty message degrades to the lowercase code name
// (mirrors what status.Errorf produces when given an empty format string).
func (e *codeError) Error() string {
	if e.msg == "" {
		return e.code.String()
	}
	return e.msg
}

// GRPCStatus implements the interface status.FromError checks for. Returning
// a *status.Status here means callers can do
//
//	return errs.ErrNotFound
//
// directly from a handler and the gRPC runtime will produce a
// status.Status{Code: NotFound, Message: ...} without any extra wrapping.
func (e *codeError) GRPCStatus() *status.Status {
	return status.New(e.code, e.Error())
}

// Is reports whether target is a codeError sharing this error's code.
//
// This is what makes errors.Is(err, errs.ErrPermission) work after the
// caller has wrapped the sentinel with a custom message via Errorf:
//
//	err := errs.Errorf(codes.PermissionDenied, "actor %q is not admin", a)
//	errors.Is(err, errs.ErrPermission) // true
func (e *codeError) Is(target error) bool {
	t, ok := target.(*codeError)
	if !ok {
		return false
	}
	return e.code == t.code
}

// Sentinel errors. One per gRPC code currently emitted by the Python server
// (per docs/go-port/shared/error-mapping.md "Code catalogue"). Codes the
// Python server does not raise (OUT_OF_RANGE, CANCELLED, ABORTED, DATA_LOSS)
// are intentionally not declared here; if a future RPC needs them, add them
// in the same row as the Python source mapping.
//
// ErrInternal and ErrDeadlineExceeded are listed in the spec as "not raised
// by the server today" but are exported here because handlers may need to
// surface them (ExecuteAtomic in-band INTERNAL channel, transport-level
// deadline pass-through). See error-mapping.md notes on these codes.
var (
	// ErrInvalidArgument -> codes.InvalidArgument.
	// Python source: api/grpc_server.py (every required-arg validation site,
	// see error-mapping.md row "INVALID_ARGUMENT").
	ErrInvalidArgument = newCode(codes.InvalidArgument)

	// ErrNotFound -> codes.NotFound.
	// Python source: api/grpc_server.py:505-516 (_check_tenant existence
	// leak guard).
	ErrNotFound = newCode(codes.NotFound)

	// ErrAlreadyExists -> codes.AlreadyExists.
	// Python source: api/grpc_server.py:697,746 (idempotency replay,
	// unique-constraint violations from apply/canonical_store.py).
	ErrAlreadyExists = newCode(codes.AlreadyExists)

	// ErrPermission -> codes.PermissionDenied.
	// Python source: api/grpc_server.py many sites (privilege escalation
	// guard, ACL deny, admin-only checks).
	ErrPermission = newCode(codes.PermissionDenied)

	// ErrFailedPrecondition -> codes.FailedPrecondition.
	// Python source: api/grpc_server.py:520,526 (region-pin mismatch);
	// schema/compat.py CompatibilityError; crypto-shred state.
	ErrFailedPrecondition = newCode(codes.FailedPrecondition)

	// ErrResourceExhausted -> codes.ResourceExhausted.
	// Python source: api/rate_limiter.py:94, auth/quota_interceptor.py:237,
	// gRPC core (oversized message).
	ErrResourceExhausted = newCode(codes.ResourceExhausted)

	// ErrUnauthenticated -> codes.Unauthenticated.
	// Python source: auth/auth_interceptor.py:196,202; api/auth.py:81,131,
	// 139,152,187 (every auth failure path).
	ErrUnauthenticated = newCode(codes.Unauthenticated)

	// ErrUnavailable -> codes.Unavailable.
	// Python source: api/grpc_server.py:392-395 (sharding mismatch; the
	// server attaches an entdb-redirect-node trailer before returning).
	ErrUnavailable = newCode(codes.Unavailable)

	// ErrUnimplemented -> codes.Unimplemented.
	// Python source: api/grpc_server.py many sites (optional-store guards
	// when global_store / user_registry / compliance is None).
	ErrUnimplemented = newCode(codes.Unimplemented)

	// ErrInternal -> codes.Internal.
	// Python source: api/grpc_server.py:824 sets the *string* "INTERNAL" on
	// ExecuteAtomicResponse.error_code; gRPC status stays OK. The Go port
	// keeps that in-band channel for ExecuteAtomic specifically (see
	// error-mapping.md "Swallow patterns" item 3). This sentinel exists for
	// any future RPC that needs codes.Internal as a real status, and for
	// the recover-and-wrap fallback discussed in error-mapping.md "Open
	// questions / risks" item 4.
	ErrInternal = newCode(codes.Internal)

	// ErrDeadlineExceeded -> codes.DeadlineExceeded.
	// Python source: not raised by the server -- surfaces only when the
	// client deadline elapses (transport-level). Exported for completeness
	// and for handlers that want to translate context.DeadlineExceeded.
	ErrDeadlineExceeded = newCode(codes.DeadlineExceeded)
)

// Errorf builds a status-bearing error in one call. Use this in handlers
// instead of status.Errorf so the Python -> Go contract stays in one place.
//
// The returned error implements GRPCStatus() and Is(*codeError), so the
// caller's tests can do errors.Is(err, errs.ErrNotFound).
//
// Empty format strings degrade to the code's name; this matches what
// status.Errorf produces.
func Errorf(c codes.Code, format string, a ...any) error {
	msg := format
	if len(a) > 0 {
		msg = fmt.Sprintf(format, a...)
	}
	if msg == "" {
		msg = c.String()
	}
	return &codeError{code: c, msg: msg}
}

// Code unwraps any error to its gRPC code. Returns codes.OK for nil and
// codes.Unknown for an error that does not carry a status (the standard
// google.golang.org/grpc/status.Code behavior). Provided as a convenience so
// handlers that import errs do not also need to import grpc/status just for
// the code lookup.
func Code(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	return status.Code(err)
}

// PreserveStatusOrSwallow mirrors the Python "re-raise gRPC aborts, swallow
// the rest" pattern documented in docs/go-port/shared/error-mapping.md
// "Swallow patterns" item 1.
//
// Behavior:
//
//   - err == nil -> return nil; dst untouched.
//   - err carries a gRPC status code (anything other than codes.Unknown)
//     -> return err unchanged. The handler should propagate this directly
//     so the SDK observes the original code.
//   - err is a naked Go error (no status, i.e. status.Code(err) ==
//     codes.Unknown) -> write err.Error() into dst via dst.SetError, return
//     nil. The handler then returns its zero/empty response with the gRPC
//     status implicitly OK -- matching the Python ResponseProto(success=
//     false, error=str(e)) shape.
//
// dst MUST be the response proto being returned (it has SetError generated
// by go-protobuf for any message with an `error` string field). If dst is
// nil, the swallow path still returns nil and silently drops the error
// message; callers SHOULD pass a non-nil dst.
func PreserveStatusOrSwallow(err error, dst interface{ SetError(string) }) error {
	if err == nil {
		return nil
	}
	if status.Code(err) != codes.Unknown {
		return err
	}
	if dst != nil {
		dst.SetError(err.Error())
	}
	return nil
}

// As is a convenience: errors.As specialized to *codeError. Returns the
// underlying code and true if err carries one of this package's sentinels
// (or any error built via Errorf). This is mainly useful for tests.
func As(err error) (codes.Code, bool) {
	var ce *codeError
	if errors.As(err, &ce) {
		return ce.code, true
	}
	return codes.OK, false
}
