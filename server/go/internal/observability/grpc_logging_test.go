// SPDX-License-Identifier: AGPL-3.0-only

package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// withBufferedTraceLogger installs a JSON slog default wrapped in the
// trace handler (so trace_id/span_id are stamped) and returns the buffer
// + a restore func.
func withBufferedTraceLogger(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(&traceHandler{Handler: slog.NewJSONHandler(&buf, nil)}))
	return &buf, func() { slog.SetDefault(prev) }
}

func TestAccessLogUnaryInterceptor_LogsMethodStatusLatencyAndTraceID(t *testing.T) {
	buf, restore := withBufferedTraceLogger(t)
	defer restore()

	ctx, traceID, spanID := tracedCtx(t)
	_, err := AccessLogUnaryInterceptor()(
		ctx, "req",
		&grpc.UnaryServerInfo{FullMethod: "/entdb.v1.EntDBService/GetNode"},
		func(context.Context, any) (any, error) { return "ok", nil },
	)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("access log not JSON: %v\n%s", err, buf.String())
	}
	if rec["msg"] != "grpc request" {
		t.Errorf("msg = %v", rec["msg"])
	}
	if rec["method"] != "/entdb.v1.EntDBService/GetNode" {
		t.Errorf("method = %v", rec["method"])
	}
	if rec["grpc_code"] != "OK" {
		t.Errorf("grpc_code = %v, want OK", rec["grpc_code"])
	}
	if _, ok := rec["latency_ms"]; !ok {
		t.Error("missing latency_ms")
	}
	if rec["trace_id"] != traceID.String() {
		t.Errorf("trace_id = %v, want %s", rec["trace_id"], traceID)
	}
	if rec["span_id"] != spanID.String() {
		t.Errorf("span_id = %v, want %s", rec["span_id"], spanID)
	}
}

func TestAccessLogUnaryInterceptor_ErrorStatusLogsWarn(t *testing.T) {
	buf, restore := withBufferedTraceLogger(t)
	defer restore()

	_, _ = AccessLogUnaryInterceptor()(
		context.Background(), "req",
		&grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(context.Context, any) (any, error) { return nil, status.Error(codes.NotFound, "nope") },
	)
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, buf.String())
	}
	if rec["grpc_code"] != "NotFound" {
		t.Errorf("grpc_code = %v, want NotFound", rec["grpc_code"])
	}
	if rec["level"] != "WARN" {
		t.Errorf("level = %v, want WARN for a non-OK status", rec["level"])
	}
}

func TestAccessLogUnaryInterceptor_SkipsHealthProbes(t *testing.T) {
	buf, restore := withBufferedTraceLogger(t)
	defer restore()

	called := false
	_, _ = AccessLogUnaryInterceptor()(
		context.Background(), "req",
		&grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"},
		func(context.Context, any) (any, error) { called = true; return "ok", nil },
	)
	if !called {
		t.Error("handler must still run for health probes")
	}
	if buf.Len() != 0 {
		t.Errorf("health probes must not be access-logged, got: %s", buf.String())
	}
}
