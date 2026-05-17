// Internal-error sanitization chokepoint (SEC-5, issue #136).
//
// Handlers that wrap an underlying store / DB / driver error into a
// client-visible codes.Internal status used to do:
//
//	return errs.Errorf(codes.Internal, "get user: %v", err)
//
// That ships the raw SQLite driver text, table names and internal file
// paths back to the caller -- an info leak that helps an attacker
// fingerprint the schema and confirm record existence (defeating the
// generic 404 on GetUser). It also defeats the point of the typed
// sentinels, since the wrapped detail is free-form in proto's
// `string error = N`.
//
// errs.Internal and errs.InternalNoCtx replace that pattern: the full
// detail (op label + underlying err) is logged server-side via slog at
// ERROR level, and the client receives only a fixed, opaque message
// (genericInternalMessage). The gRPC code stays codes.Internal so the
// SDK's retry/observability behavior is unchanged.
//
// Typed sentinels (NotFound / AlreadyExists / InvalidArgument /
// PermissionDenied / FailedPrecondition / ...) and their human-meaningful
// messages are deliberately NOT routed through here -- those messages are
// part of the contract and carry no server internals. Only Internal /
// Unknown (wrapped naked errors) are sanitized.
package errs

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
)

// genericInternalMessage is the only thing a client ever sees for a
// sanitized internal error. It carries no schema, path, or driver
// detail. Keep it stable -- contract tests assert on it and changing it
// is a (minor) wire-visible change.
const genericInternalMessage = "internal error"

// Internal logs the full underlying error server-side and returns a
// codes.Internal status carrying only the fixed generic message.
//
// op is a short, static, non-sensitive label for the failing operation
// (e.g. "get user", "wal append") used purely for server-side log
// correlation. It MUST NOT contain request data; pass a literal.
//
// Usage -- replace
//
//	return errs.Errorf(codes.Internal, "get user: %v", err)
//
// with
//
//	return errs.Internal(ctx, "get user", err)
//
// If err is nil, Internal still returns a non-nil codes.Internal error
// (callers only reach this path on a real failure); the log records a
// nil error for traceability.
func Internal(ctx context.Context, op string, err error) error {
	logInternal(ctx, op, err)
	return &codeError{code: codes.Internal, msg: genericInternalMessage}
}

// InternalNoCtx is Internal for the handful of pure helper functions
// that have no context.Context in scope (e.g. proto-conversion helpers).
// It logs without request-scoped attributes; prefer Internal whenever a
// ctx is available.
func InternalNoCtx(op string, err error) error {
	logInternal(context.Background(), op, err)
	return &codeError{code: codes.Internal, msg: genericInternalMessage}
}

// logInternal centralizes the server-side record of a sanitized error so
// the detail is never lost. ERROR level: these are by definition
// unexpected store/DB/driver failures.
func logInternal(ctx context.Context, op string, err error) {
	slog.ErrorContext(ctx, "internal error sanitized for client",
		"op", op,
		"error", err,
	)
}
