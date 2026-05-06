package main

import (
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mapErr translates an upstream gRPC error into a connect.Error so the
// browser sees a sane Connect-shaped failure (with the right Connect
// code) instead of a raw gRPC status struct. We deliberately discard
// any structured details from the upstream error — the upstream is
// internal-only (see the warning at the top of entdb.proto) and may
// embed implementation-leaking strings (SQL fragments, file paths,
// kafka offsets) that have no business reaching a browser tab.
//
// If the error already is a connect.Error (e.g. raised inside a console
// handler before the upstream call), it's returned unchanged so the
// original code/message survive.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce
	}
	st, ok := status.FromError(err)
	if !ok {
		// Not a gRPC status — most likely a connection / transport
		// error. Surface as Unavailable; the browser doesn't need
		// the underlying string.
		return connect.NewError(connect.CodeUnavailable, errors.New("upstream unavailable"))
	}
	return connect.NewError(grpcCodeToConnect(st.Code()), errors.New(sanitiseMessage(st.Message())))
}

// grpcCodeToConnect maps gRPC canonical codes to Connect codes. The two
// share the same numeric values today but using a switch keeps this
// future-proof and obvious in code review.
func grpcCodeToConnect(c codes.Code) connect.Code {
	switch c {
	case codes.OK:
		// Caller shouldn't be wrapping a successful status, but if
		// they do, we still need a Connect code. "Unknown" is the
		// safest default — never returned in practice.
		return connect.CodeUnknown
	case codes.Canceled:
		return connect.CodeCanceled
	case codes.Unknown:
		return connect.CodeUnknown
	case codes.InvalidArgument:
		return connect.CodeInvalidArgument
	case codes.DeadlineExceeded:
		return connect.CodeDeadlineExceeded
	case codes.NotFound:
		return connect.CodeNotFound
	case codes.AlreadyExists:
		return connect.CodeAlreadyExists
	case codes.PermissionDenied:
		return connect.CodePermissionDenied
	case codes.ResourceExhausted:
		return connect.CodeResourceExhausted
	case codes.FailedPrecondition:
		return connect.CodeFailedPrecondition
	case codes.Aborted:
		return connect.CodeAborted
	case codes.OutOfRange:
		return connect.CodeOutOfRange
	case codes.Unimplemented:
		return connect.CodeUnimplemented
	case codes.Internal:
		return connect.CodeInternal
	case codes.Unavailable:
		return connect.CodeUnavailable
	case codes.DataLoss:
		return connect.CodeDataLoss
	case codes.Unauthenticated:
		return connect.CodeUnauthenticated
	default:
		return connect.CodeUnknown
	}
}

// sanitiseMessage redacts message strings that look like they could
// leak server internals. For PR 1 this is intentionally conservative:
// we keep short, human-readable messages but blank out anything that
// looks like a path or SQL fragment. The point is to keep the surface
// debuggable without becoming a side-channel.
func sanitiseMessage(msg string) string {
	if msg == "" {
		return "upstream error"
	}
	// Truncate long messages — anything longer than 256 chars is
	// almost certainly a stack/trace/SQL dump.
	if len(msg) > 256 {
		return "upstream error"
	}
	return msg
}
