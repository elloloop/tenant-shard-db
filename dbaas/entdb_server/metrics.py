"""
Prometheus metrics for EntDB.

Exposes counters and histograms for:
- gRPC request count and latency
- Applier events processed/skipped/errors
- WAL append latency
- SQLite operation count

Disabled by default. Enable with METRICS_ENABLED=true.
Metrics served on METRICS_PORT (default 9090).
"""

from __future__ import annotations

import logging
import time
from collections.abc import Generator
from contextlib import contextmanager
from typing import Any

logger = logging.getLogger(__name__)

try:
    from prometheus_client import Counter, Gauge, Histogram, start_http_server

    PROMETHEUS_AVAILABLE = True
except ImportError:
    PROMETHEUS_AVAILABLE = False


# Metrics (created lazily when enabled)
_metrics: dict[str, Any] = {}


def init_metrics(port: int = 9090) -> bool:
    """Initialize and start Prometheus metrics server."""
    if not PROMETHEUS_AVAILABLE:
        logger.warning("prometheus_client not installed, metrics disabled")
        return False

    global _metrics
    _metrics = {
        "grpc_requests": Counter(
            "entdb_grpc_requests_total",
            "Total gRPC requests",
            ["method", "status"],
        ),
        "grpc_latency": Histogram(
            "entdb_grpc_latency_seconds",
            "gRPC request latency",
            ["method"],
            buckets=(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 5.0),
        ),
        "applier_events": Counter(
            "entdb_applier_events_total",
            "Applier events processed",
            ["status"],  # applied, skipped, error
        ),
        "applier_lag": Gauge(
            "entdb_applier_lag_seconds",
            "Applier lag behind WAL head",
        ),
        "wal_appends": Counter(
            "entdb_wal_appends_total",
            "WAL append operations",
            ["status"],
        ),
        "wal_append_latency": Histogram(
            "entdb_wal_append_latency_seconds",
            "WAL append latency",
            buckets=(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.5, 1.0),
        ),
        "sqlite_operations": Counter(
            "entdb_sqlite_operations_total",
            "SQLite operations",
            ["operation"],  # read, write
        ),
        "active_tenants": Gauge(
            "entdb_active_tenants",
            "Number of active tenants",
        ),
    }

    start_http_server(port)
    logger.info("Prometheus metrics server started", extra={"port": port})
    return True


def record_grpc_request(method: str, status: str, duration: float) -> None:
    """Record a gRPC request."""
    if "grpc_requests" in _metrics:
        _metrics["grpc_requests"].labels(method=method, status=status).inc()
        _metrics["grpc_latency"].labels(method=method).observe(duration)


def record_applier_event(status: str) -> None:
    """Record an applier event (applied/skipped/error)."""
    if "applier_events" in _metrics:
        _metrics["applier_events"].labels(status=status).inc()


def record_wal_append(status: str, duration: float) -> None:
    """Record a WAL append."""
    if "wal_appends" in _metrics:
        _metrics["wal_appends"].labels(status=status).inc()
        _metrics["wal_append_latency"].observe(duration)


def record_sqlite_op(operation: str) -> None:
    """Record a SQLite operation."""
    if "sqlite_operations" in _metrics:
        _metrics["sqlite_operations"].labels(operation=operation).inc()


def set_active_tenants(count: int) -> None:
    """Set the number of active tenants."""
    if "active_tenants" in _metrics:
        _metrics["active_tenants"].set(count)


@contextmanager
def track_latency(metric_name: str, **labels: str) -> Generator[None, None, None]:
    """Context manager to track operation latency."""
    start = time.perf_counter()
    try:
        yield
    finally:
        duration = time.perf_counter() - start
        if metric_name in _metrics:
            _metrics[metric_name].labels(**labels).observe(duration)
