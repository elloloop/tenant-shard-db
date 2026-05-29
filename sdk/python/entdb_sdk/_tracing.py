"""W3C trace-context propagation for the EntDB Python SDK (ADR-033 §6).

Injects the caller's active OpenTelemetry span as ``traceparent`` (and
``tracestate``) gRPC metadata so the server continues the same trace.

This is a *no-op* when OpenTelemetry is not installed or when there is no
active span, so SDK consumers are never forced to depend on
OpenTelemetry. Install the optional extra to enable propagation::

    pip install "entdb-sdk[tracing]"
"""

from __future__ import annotations

try:
    from opentelemetry.propagate import inject as _otel_inject

    _OTEL_AVAILABLE = True
except Exception:  # pragma: no cover - exercised by the no-otel path
    _OTEL_AVAILABLE = False


def inject_trace_context(
    metadata: list[tuple[str, str]],
) -> list[tuple[str, str]]:
    """Append W3C ``traceparent``/``tracestate`` for the active span.

    Mutates and returns ``metadata``. No-op when OpenTelemetry is absent
    or no span is active (the carrier comes back empty). Uses the global
    propagator, which OpenTelemetry defaults to W3C tracecontext +
    baggage.
    """
    if not _OTEL_AVAILABLE:
        return metadata
    carrier: dict[str, str] = {}
    try:
        _otel_inject(carrier)
    except Exception:  # pragma: no cover - defensive: never break a call
        return metadata
    for key, value in carrier.items():
        metadata.append((key, value))
    return metadata
