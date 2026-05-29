// SPDX-License-Identifier: AGPL-3.0-only

package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func tracedCtx(t *testing.T) (context.Context, trace.TraceID, trace.SpanID) {
	t.Helper()
	traceID, err := trace.TraceIDFromHex("11112222333344445555666677778888")
	if err != nil {
		t.Fatalf("trace id: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("1111222233334444")
	if err != nil {
		t.Fatalf("span id: %v", err)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled})
	return trace.ContextWithSpanContext(context.Background(), sc), traceID, spanID
}

// TestTraceHandlerStampsTraceAndSpanID verifies request-path logs carry
// trace_id/span_id when the ctx is traced (ADR-033 §3) — the property
// that ties logs to traces with no call-site changes.
func TestTraceHandlerStampsTraceAndSpanID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(&traceHandler{Handler: slog.NewJSONHandler(&buf, nil)})

	ctx, traceID, spanID := tracedCtx(t)
	logger.InfoContext(ctx, "internal error sanitized for client", "op", "GetNode")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, buf.String())
	}
	if got := rec["trace_id"]; got != traceID.String() {
		t.Errorf("trace_id = %v, want %s", got, traceID)
	}
	if got := rec["span_id"]; got != spanID.String() {
		t.Errorf("span_id = %v, want %s", got, spanID)
	}
}

func TestTraceHandlerNoStampWithoutSpan(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(&traceHandler{Handler: slog.NewJSONHandler(&buf, nil)})
	logger.InfoContext(context.Background(), "no span here")
	if strings.Contains(buf.String(), "trace_id") {
		t.Errorf("trace_id must not be stamped without an active span: %s", buf.String())
	}
}

// TestInitInstallsW3CPropagator verifies Init wires the global W3C
// propagator so an incoming traceparent is extracted into a valid span
// context — the entry point the gRPC StatsHandler relies on (ADR-033 §1).
func TestInitInstallsW3CPropagator(t *testing.T) {
	shutdown, err := Init(context.Background(), Config{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	carrier := propagation.MapCarrier{
		"traceparent": "00-11112222333344445555666677778888-1111222233334444-01",
	}
	ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		t.Fatal("global propagator did not extract a valid span context from an incoming traceparent")
	}
	if got := sc.TraceID().String(); got != "11112222333344445555666677778888" {
		t.Errorf("extracted trace id = %s", got)
	}
	if !sc.IsSampled() {
		t.Error("extracted span context should be sampled (flags=01)")
	}
}
