# ADR-033: Accept incoming W3C trace context; correlate logs, traces, and metrics

**Status:** Accepted
**Decided:** 2026-05-29
**Tags:** observability, tracing, logging, metrics, grpc, wire-contract, sdk

**Implementation:** observability bootstrap
(`server/go/internal/observability/`); gRPC `StatsHandler` +
trace-correlated slog default (`server/go/cmd/entdb-server/main.go`);
WAL trace-context headers (`server/go/internal/wal/wal.go`,
`internal/api/execute_atomic.go`, `internal/apply/applier.go`); metric
exemplars (`server/go/internal/metrics/grpc.go`); SDK injection
(`sdk/go/entdb`, `sdk/python/entdb_sdk`).

## Context

The server emitted Prometheus metrics (`entdb_grpc_*`, behind
`--metrics-addr`) but had **no distributed tracing and no
trace-correlated logging**. OpenTelemetry was present only as transitive
`// indirect` deps; the auth interceptor read gRPC metadata for auth
headers only, never `traceparent`; logging was the stdlib `log` package
(plaintext, no trace id); and a `RequestContext.trace_id` proto field
existed but no handler ever read it. An operator could not take a
`traceparent` from an upstream service and follow that request through
EntDB's logs, traces, and metrics.

## Decision

EntDB **accepts incoming [W3C Trace Context](https://www.w3.org/TR/trace-context/)**
(`traceparent` / `tracestate`) on every RPC and threads the resulting
trace id through **all three** observability signals: logs, traces, and
metrics. The standard `traceparent` gRPC metadata header is the source of
truth; the `RequestContext.trace_id` proto field is **superseded** (see
"Superseded proto field" below).

### 1. Extraction + propagation (the entry point)

- A single global propagator is installed at boot:
  `propagation.NewCompositeTextMapPropagator(TraceContext{}, Baggage{})`.
- The gRPC server installs `grpc.StatsHandler(otelgrpc.NewServerHandler())`.
  otelgrpc runs the global propagator over incoming metadata, so an
  upstream `traceparent` becomes the **parent** of EntDB's server span;
  when absent, EntDB starts a fresh root. Either way a valid
  `SpanContext` lives on the request `ctx` and flows down the existing
  ctx-first call path (handler → store → SQLite) with no extra plumbing.
- This is wired as a `StatsHandler`, **before** the auth interceptors,
  so the span wraps authentication and an auth rejection is still traced.

### 2. Traces (export is opt-in, correlation is always on)

- A real `sdktrace.TracerProvider` is **always** configured (sampler
  `ParentBased(AlwaysSample)`), so a valid trace id is available for log
  and metric correlation **even when nothing is exported**.
- Span **export** is opt-in: an OTLP/gRPC exporter is attached only when
  `--otlp-endpoint` (or the standard `OTEL_EXPORTER_OTLP_ENDPOINT` env)
  is set. With no endpoint there is no exporter and no network egress —
  spans are created and carry the trace id but go nowhere. This keeps the
  default build side-effect-free (matches `--metrics-addr` defaulting
  off) while making "turn on tracing" a one-flag change.
- A `resource` with `service.name=entdb-server` (+ version) attributes
  every span.

### 3. Logs (structured, trace-correlated)

- A structured `slog` handler is set as the process default at boot
  (`--log-format=text|json`, default `text` to preserve dev output). The
  handler wraps the base handler and, for every record carrying a ctx
  with a valid span, adds `trace_id` and `span_id` attributes.
- Because the existing request-path logging already uses the
  `slog.*Context` variants (`internal/errs/sanitize.go`,
  `internal/audit/*`), setting the default handler is sufficient to make
  those lines carry the trace id — no call-site churn. Boot/lifecycle
  `log.Printf` lines run with no request ctx and are intentionally left
  as-is (nothing to correlate).

### 4. Metrics (trace exemplars)

- `metrics.RecordGRPCRequest` becomes **ctx-aware**; when the ctx carries
  a sampled span it records a `trace_id` **exemplar** on the
  `entdb_grpc_latency_seconds` histogram (`ObserveWithExemplar`). Per-RPC
  status semantics stay in the handlers (e.g. ExecuteAtomic's in-band
  `INTERNAL` outcome), so metrics recording stays per-handler rather than
  moving to an interceptor that can only see the gRPC status.
- `/metrics` is served with OpenMetrics enabled so exemplars are exposed
  to scrapers that support them.

### 5. Trace context survives the async WAL boundary

The write path is async: a handler appends to the WAL and the applier
writes SQLite later, on the boot ctx (not the request ctx) — so an
in-process span chain would end at `ExecuteAtomic`. To keep the trace
continuous through apply:

- `ExecuteAtomic` injects the current `traceparent`/`tracestate` into the
  WAL `Record.Headers` map (alongside the existing
  `HeaderIdempotencyKey`), using the global propagator.
- The applier extracts that context from `rec.Headers` and starts the
  per-record apply span **linked to / as a child of** the originating
  request's trace, so apply-side spans, logs, and metrics correlate back
  to the request that produced the event.

Headers are metadata-only; they do **not** change the WAL `Event` JSON
body, so the cross-impl contract byte layout (ADR-006/ADR-018) and replay
determinism are unaffected.

### 6. SDK injection (both SDKs, one PR)

For an end-to-end trace to exist, callers must emit `traceparent`. Both
the Go (`sdk/go/entdb`) and Python (`sdk/python/entdb_sdk`) SDKs inject
the active span's W3C `traceparent` into outgoing gRPC metadata via a
client interceptor. If the host app has no active OTel span, injection is
a no-op (no dependency forced on SDK consumers). Shipped together per the
"both SDKs in one PR" rule, each with mock unit tests + a live-server
integration test.

### 7. Trace context is NOT persisted in SQLite (decided 2026-05-29)

The trace id is intentionally **not** stored in SQLite — not on business
rows and not in the `applied_events` bookkeeping. Tracing/observability
is not data: the durable provenance link already lives in the WAL record
headers (and therefore the S3 archive, ADR-015), and live debugging is
served by spans (trace backend) + `trace_id` on logs + metric exemplars.
SQLite stays data + indexes only. This is the standard-DB posture and
avoids coupling observability into the data model or the replayed,
fingerprinted event body.

## Superseded proto field

`RequestContext.trace_id` (proto field 3) predates this work and was
never read by the server. W3C `traceparent` metadata is now the
mechanism. The field is left in place (removing it is a breaking wire
change per ADR-032) but documented as superseded; the server does not
read it. A future cleanup may reserve the id.

## Alternatives considered

- **Honor `RequestContext.trace_id` instead of `traceparent`.** Rejected:
  non-standard, no propagation format, no parent/sampling semantics,
  doesn't interop with upstream services or collectors.
- **otelgrpc interceptor instead of `StatsHandler`.** `StatsHandler` is
  the maintained, lower-overhead path and is what otelgrpc documents;
  the interceptor path is legacy.
- **Centralize metrics in one interceptor.** Rejected: loses
  handler-specific status (in-band `INTERNAL`, skipped, etc.) that an
  interceptor cannot observe.
- **Always export spans.** Rejected: forces an OTLP endpoint and network
  egress on every deployment; export is opt-in, correlation is not.

## Consequences

- Operators get true request-level traceability across logs, traces, and
  metrics from an incoming `traceparent`, with a single flag to ship
  spans to any OTLP collector (Tempo, Jaeger, etc.).
- OTel modules move from `// indirect` to direct requires; `otel/sdk` and
  an OTLP exporter are added.
- Default behavior is unchanged for anyone who sets no new flags (no
  exporter, text logs) — except logs now carry `trace_id`/`span_id` when
  a request is traced, which is additive.
