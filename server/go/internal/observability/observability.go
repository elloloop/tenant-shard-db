// SPDX-License-Identifier: AGPL-3.0-only

// Package observability bootstraps the EntDB server's tracing, logging,
// and metric-correlation pipeline per ADR-033.
//
// The headline capability is: accept an incoming W3C `traceparent` on
// every RPC and thread the resulting trace id through all three
// observability signals.
//
//   - A global W3C TraceContext+Baggage propagator is installed so the
//     gRPC StatsHandler (wired in cmd/entdb-server) parses an upstream
//     traceparent into the request context.
//   - A real TracerProvider is ALWAYS configured (so a valid trace id is
//     available for log/metric correlation), but span EXPORT is opt-in:
//     an OTLP/gRPC exporter is attached only when an endpoint is set.
//   - A structured slog default is installed whose handler stamps
//     trace_id/span_id onto every record carrying a traced context.
//
// Init is side-effect-free with respect to the network unless an OTLP
// endpoint is configured — mirroring how --metrics-addr defaults off.
package observability

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// ServiceName is the service.name resource attribute attached to every
// span and reported to collectors.
const ServiceName = "entdb-server"

// Config configures the observability bootstrap. The zero value is
// valid: no exporter, text logs at info level to stderr.
type Config struct {
	// ServiceVersion is the service.version resource attribute. Empty is
	// fine (attribute omitted).
	ServiceVersion string

	// OTLPEndpoint, when non-empty, attaches an OTLP/gRPC span exporter
	// pointing at this host:port (or URL). Empty disables span export;
	// the standard OTEL_EXPORTER_OTLP_ENDPOINT env var is consulted as a
	// fallback so the flag and the env both work.
	OTLPEndpoint string

	// OTLPInsecure disables transport security for the OTLP exporter
	// (plaintext gRPC). For local collectors only.
	OTLPInsecure bool

	// LogFormat selects the base slog handler: "json" or "text"
	// (default "text").
	LogFormat string

	// LogLevel is the minimum slog level (default slog.LevelInfo).
	LogLevel slog.Level

	// LogWriter is where logs are written (default os.Stderr).
	LogWriter io.Writer
}

// Init installs the global propagator, tracer provider, and structured
// logger, and returns a shutdown function that flushes and closes the
// span pipeline. Init is safe to call once at process start.
//
// The returned shutdown function is always non-nil (a no-op when nothing
// needs flushing), so callers can defer it unconditionally.
func Init(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	// Logging first so any later setup failures are structured.
	setupLogger(cfg)

	// W3C propagator: makes an incoming traceparent the parent of our
	// server span, and lets us inject it into outgoing calls / WAL
	// headers. Composite with Baggage for general OTel interop.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	res := resource.NewSchemaless(serviceAttrs(cfg)...)

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		// ParentBased(AlwaysSample): honor an upstream sampling decision
		// when present, otherwise sample roots. Trace ids are available
		// for log/metric correlation regardless of the sampling decision
		// or whether an exporter is attached.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	}

	endpoint := strings.TrimSpace(cfg.OTLPEndpoint)
	envEndpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) +
		strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"))
	var exporter sdktrace.SpanExporter
	if endpoint != "" || envEndpoint != "" {
		exporter, err = newOTLPExporter(ctx, endpoint, cfg.OTLPInsecure)
		if err != nil {
			return func(context.Context) error { return nil }, fmt.Errorf("observability: otlp exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exporter))
		slog.Info("tracing: OTLP span export enabled", "endpoint", endpointOrEnv(endpoint, envEndpoint), "insecure", cfg.OTLPInsecure)
	} else {
		slog.Info("tracing: incoming traceparent honored; span export OFF (no --otlp-endpoint)")
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)

	return func(shutdownCtx context.Context) error {
		// tp.Shutdown flushes the batch processor and closes the
		// exporter underneath it.
		return tp.Shutdown(shutdownCtx)
	}, nil
}

// newOTLPExporter builds an OTLP/gRPC trace exporter. When endpoint is
// empty the exporter falls back to the standard OTEL_EXPORTER_OTLP_*
// environment variables.
func newOTLPExporter(ctx context.Context, endpoint string, insecure bool) (sdktrace.SpanExporter, error) {
	expOpts := []otlptracegrpc.Option{}
	if endpoint != "" {
		expOpts = append(expOpts, otlptracegrpc.WithEndpoint(endpoint))
	}
	if insecure {
		expOpts = append(expOpts, otlptracegrpc.WithInsecure())
	}
	return otlptracegrpc.New(ctx, expOpts...)
}

func serviceAttrs(cfg Config) []attribute.KeyValue {
	attrs := []attribute.KeyValue{attribute.String("service.name", ServiceName)}
	if v := strings.TrimSpace(cfg.ServiceVersion); v != "" {
		attrs = append(attrs, attribute.String("service.version", v))
	}
	return attrs
}

func endpointOrEnv(flagEndpoint, envEndpoint string) string {
	if flagEndpoint != "" {
		return flagEndpoint
	}
	return "env:OTEL_EXPORTER_OTLP_ENDPOINT"
}
