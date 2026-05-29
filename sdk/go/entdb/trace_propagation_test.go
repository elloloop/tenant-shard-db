// SPDX-License-Identifier: MIT

package entdb

import (
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func captureOutgoingMD(t *testing.T, ctx context.Context) metadata.MD {
	t.Helper()
	var got metadata.MD
	invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		got, _ = metadata.FromOutgoingContext(ctx)
		return nil
	}
	if err := traceparentUnaryInterceptor()(ctx, "/entdb.v1.EntDBService/GetNode", nil, nil, nil, invoker); err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}
	return got
}

// TestTraceparentUnaryInterceptor_InjectsActiveSpan verifies the SDK
// injects the caller's active W3C trace context as a traceparent header
// (ADR-033 §6).
func TestTraceparentUnaryInterceptor_InjectsActiveSpan(t *testing.T) {
	traceID, err := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("trace id: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("0123456789abcdef")
	if err != nil {
		t.Fatalf("span id: %v", err)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	got := captureOutgoingMD(t, ctx)
	tp := got.Get("traceparent")
	if len(tp) == 0 {
		t.Fatal("traceparent header not injected for an active span")
	}
	if !strings.Contains(tp[0], traceID.String()) {
		t.Errorf("traceparent %q does not carry trace id %s", tp[0], traceID)
	}
	if !strings.Contains(tp[0], spanID.String()) {
		t.Errorf("traceparent %q does not carry span id %s", tp[0], spanID)
	}
}

// TestTraceparentUnaryInterceptor_NoSpanNoHeader verifies the SDK forces
// no tracing behaviour on callers without an active span: nothing is
// injected, so non-OpenTelemetry consumers are unaffected.
func TestTraceparentUnaryInterceptor_NoSpanNoHeader(t *testing.T) {
	got := captureOutgoingMD(t, context.Background())
	if len(got.Get("traceparent")) != 0 {
		t.Errorf("traceparent must not be injected without an active span; got %v", got.Get("traceparent"))
	}
}

// TestTraceparentUnaryInterceptor_PreservesExistingMetadata verifies the
// injected header is added alongside (not replacing) existing outgoing
// metadata such as the auth header.
func TestTraceparentUnaryInterceptor_PreservesExistingMetadata(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("aaaaaaaaaaaaaaaabbbbbbbbbbbbbbbb")
	spanID, _ := trace.SpanIDFromHex("cccccccccccccccc")
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled})
	ctx := metadata.NewOutgoingContext(
		trace.ContextWithSpanContext(context.Background(), sc),
		metadata.Pairs("authorization", "Bearer tok"),
	)
	got := captureOutgoingMD(t, ctx)
	if len(got.Get("authorization")) == 0 {
		t.Error("existing authorization header was dropped")
	}
	if len(got.Get("traceparent")) == 0 {
		t.Error("traceparent was not added alongside existing metadata")
	}
}
