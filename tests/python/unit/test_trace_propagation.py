"""Unit tests for W3C trace-context propagation in the Python SDK.

ADR-033 §6: the SDK injects the caller's active OpenTelemetry span as a
``traceparent`` gRPC metadata header so the server continues the trace.
Injection is a no-op without an active span. Because every RPC funnels
through ``_GrpcClient._build_metadata``, injecting there covers all
methods uniformly.
"""

from __future__ import annotations

import pytest

from entdb_sdk._tracing import inject_trace_context

# OpenTelemetry is the SDK's OPTIONAL [tracing] extra; skip these tests
# when it isn't installed (e.g. a minimal CI unit-test env) rather than
# failing collection. The no-otel behaviour (injection is a no-op) is the
# SDK's contract and is covered by the SDK shipping without this extra.
pytest.importorskip("opentelemetry.sdk.trace")

from opentelemetry import trace  # noqa: E402  (after importorskip guard)
from opentelemetry.sdk.trace import TracerProvider  # noqa: E402


def _tracer() -> trace.Tracer:
    # A bare provider mints valid sampled span contexts; no exporter is
    # needed because the SDK only PROPAGATES (the server exports).
    return TracerProvider().get_tracer("entdb-sdk-test")


def test_inject_adds_traceparent_for_active_span() -> None:
    with _tracer().start_as_current_span("client-call") as span:
        md: list[tuple[str, str]] = []
        inject_trace_context(md)
        keys = {k for k, _ in md}
        assert "traceparent" in keys, f"traceparent not injected: {md}"
        trace_id_hex = format(span.get_span_context().trace_id, "032x")
        traceparent = next(v for k, v in md if k == "traceparent")
        assert trace_id_hex in traceparent, f"{traceparent!r} missing {trace_id_hex}"


def test_inject_is_noop_without_active_span() -> None:
    md: list[tuple[str, str]] = []
    inject_trace_context(md)
    assert md == [], f"traceparent must not be injected without an active span: {md}"


def test_inject_preserves_existing_metadata() -> None:
    with _tracer().start_as_current_span("client-call"):
        md: list[tuple[str, str]] = [("authorization", "Bearer tok")]
        inject_trace_context(md)
        keys = {k for k, _ in md}
        assert "authorization" in keys, "existing auth header dropped"
        assert "traceparent" in keys, "traceparent not added alongside auth"


def test_build_metadata_injects_traceparent() -> None:
    """_build_metadata (the per-RPC chokepoint) wires injection so all
    methods propagate the trace."""
    from entdb_sdk._grpc_client import GrpcClient

    client = GrpcClient.__new__(GrpcClient)
    client._api_key = None  # type: ignore[attr-defined]
    with _tracer().start_as_current_span("client-call"):
        md = client._build_metadata()
        assert any(k == "traceparent" for k, _ in md), f"_build_metadata omitted traceparent: {md}"
