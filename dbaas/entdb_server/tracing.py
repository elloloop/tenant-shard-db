"""
OpenTelemetry tracing for EntDB.

Provides distributed tracing across gRPC calls, WAL operations,
and SQLite queries. Disabled when opentelemetry is not installed.

Enable:
    pip install entdb[tracing]
    TRACE_ENABLED=true
    TRACE_SAMPLING_RATE=0.1
    TRACE_EXPORTER=otlp  # or jaeger, console
    TRACE_ENDPOINT=http://localhost:4317
"""

from __future__ import annotations

import logging
from contextlib import contextmanager
from typing import Any

logger = logging.getLogger(__name__)

try:
    from opentelemetry import trace
    from opentelemetry.sdk.resources import Resource
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import BatchSpanProcessor

    OTEL_AVAILABLE = True
except ImportError:
    OTEL_AVAILABLE = False
    trace = None

_tracer = None


def init_tracing(
    service_name: str = "entdb",
    sampling_rate: float = 0.1,
    exporter: str = "console",
    endpoint: str | None = None,
) -> bool:
    """Initialize OpenTelemetry tracing."""
    global _tracer

    if not OTEL_AVAILABLE:
        logger.warning("opentelemetry not installed, tracing disabled")
        return False

    from opentelemetry.sdk.trace.sampling import TraceIdRatioBased

    resource = Resource.create({"service.name": service_name})
    sampler = TraceIdRatioBased(sampling_rate)
    provider = TracerProvider(resource=resource, sampler=sampler)

    if exporter == "otlp" and endpoint:
        try:
            from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter

            span_exporter = OTLPSpanExporter(endpoint=endpoint)
            provider.add_span_processor(BatchSpanProcessor(span_exporter))
        except ImportError:
            logger.warning("OTLP exporter not available, falling back to console")
            exporter = "console"

    if exporter == "console":
        from opentelemetry.sdk.trace.export import ConsoleSpanExporter

        provider.add_span_processor(BatchSpanProcessor(ConsoleSpanExporter()))

    trace.set_tracer_provider(provider)
    _tracer = trace.get_tracer("entdb")
    logger.info("Tracing initialized", extra={"exporter": exporter, "sampling_rate": sampling_rate})
    return True


@contextmanager
def span(name: str, **attributes: Any):
    """Create a trace span. No-op when tracing is not initialized."""
    if _tracer:
        with _tracer.start_as_current_span(name, attributes=attributes) as s:
            yield s
    else:
        yield None


def get_tracer():
    """Get the global tracer (or None)."""
    return _tracer
