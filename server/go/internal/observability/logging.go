// SPDX-License-Identifier: AGPL-3.0-only

package observability

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// setupLogger installs a process-wide structured slog default whose
// handler stamps trace_id/span_id onto every record carrying a traced
// context (ADR-033). Existing slog.*Context call sites
// (internal/errs/sanitize.go, internal/audit/*) gain trace correlation
// with no call-site changes.
func setupLogger(cfg Config) {
	w := cfg.LogWriter
	if w == nil {
		w = os.Stderr
	}
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}

	var base slog.Handler
	if strings.EqualFold(strings.TrimSpace(cfg.LogFormat), "json") {
		base = slog.NewJSONHandler(w, opts)
	} else {
		base = slog.NewTextHandler(w, opts)
	}
	slog.SetDefault(slog.New(&traceHandler{Handler: base}))
}

// traceHandler decorates a base slog.Handler so every record carrying a
// ctx with a valid OpenTelemetry span context is enriched with the
// trace_id and span_id, tying logs to traces and (via exemplars)
// metrics.
type traceHandler struct {
	slog.Handler
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx != nil {
		if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
			r.AddAttrs(
				slog.String("trace_id", sc.TraceID().String()),
				slog.String("span_id", sc.SpanID().String()),
			)
		}
	}
	return h.Handler.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithGroup(name)}
}
