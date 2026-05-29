// SPDX-License-Identifier: AGPL-3.0-only

package observability

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// accessLogSkip is the set of high-frequency methods omitted from the
// per-request access log to avoid drowning real traffic in health-probe
// noise (orchestrators poll Health every few seconds).
var accessLogSkip = map[string]struct{}{
	"/entdb.v1.EntDBService/Health": {},
	"/grpc.health.v1.Health/Check":  {},
}

// AccessLogUnaryInterceptor emits exactly one structured log line per
// unary RPC, carrying the method, gRPC status, and latency. Because it
// logs with the request context, the trace-correlating slog handler
// (see Init) stamps trace_id/span_id onto the line — so every request
// has a log entry tagged with the same correlation id as its trace and
// metric exemplar (ADR-033).
//
// Install it as the OUTERMOST interceptor so it records the final status
// of every request, including those rejected by auth.
func AccessLogUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, skip := accessLogSkip[info.FullMethod]; skip {
			return handler(ctx, req)
		}
		start := time.Now()
		resp, err := handler(ctx, req)
		logRequest(ctx, info.FullMethod, err, time.Since(start))
		return resp, err
	}
}

// AccessLogStreamInterceptor is the streaming counterpart of
// AccessLogUnaryInterceptor.
func AccessLogStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if _, skip := accessLogSkip[info.FullMethod]; skip {
			return handler(srv, ss)
		}
		start := time.Now()
		err := handler(srv, ss)
		logRequest(ss.Context(), info.FullMethod, err, time.Since(start))
		return err
	}
}

func logRequest(ctx context.Context, method string, err error, elapsed time.Duration) {
	code := status.Code(err)
	level := slog.LevelInfo
	if code != codes.OK {
		level = slog.LevelWarn
	}
	attrs := []slog.Attr{
		slog.String("method", method),
		slog.String("grpc_code", code.String()),
		slog.Int64("latency_ms", elapsed.Milliseconds()),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	// LogAttrs with ctx so the trace handler can attach trace_id/span_id.
	slog.Default().LogAttrs(ctx, level, "grpc request", attrs...)
}
