// SPDX-License-Identifier: MIT

package entdb

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// w3cPropagator injects the caller's active span as W3C trace context.
// The concrete TraceContext propagator is used (not the OpenTelemetry
// global) so the SDK propagates whenever the caller has an active span,
// regardless of whether they configured a global propagator.
var w3cPropagator = propagation.TraceContext{}

// traceparentUnaryInterceptor injects the caller's active OpenTelemetry
// span context as a W3C traceparent (+ tracestate) into the outgoing
// gRPC metadata so the server — and, via the WAL, the applier —
// continue the same trace (ADR-033 §6).
//
// It is a no-op when the caller has no active span, so it forces no
// tracing dependency or behavior on SDK consumers who don't use
// OpenTelemetry.
func traceparentUnaryInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		if ok {
			md = md.Copy()
		} else {
			md = metadata.MD{}
		}
		w3cPropagator.Inject(ctx, &metadataCarrier{md: md})
		ctx = metadata.NewOutgoingContext(ctx, md)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// metadataCarrier adapts gRPC metadata.MD to propagation.TextMapCarrier
// so the W3C propagator can write traceparent/tracestate into it.
type metadataCarrier struct{ md metadata.MD }

func (c *metadataCarrier) Get(key string) string {
	vals := c.md.Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func (c *metadataCarrier) Set(key, value string) { c.md.Set(key, value) }

func (c *metadataCarrier) Keys() []string {
	out := make([]string, 0, len(c.md))
	for k := range c.md {
		out = append(out, k)
	}
	return out
}
