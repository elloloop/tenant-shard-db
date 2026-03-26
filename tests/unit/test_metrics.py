"""
Unit tests for Prometheus metrics module.

Tests cover:
- Graceful behavior when prometheus_client is not installed
- No-crash behavior when metrics are not initialized
- Metric recording functions
- Latency tracking context manager
"""

import time
from unittest import mock

import pytest

from dbaas.entdb_server import metrics
from dbaas.entdb_server.metrics import (
    init_metrics,
    record_applier_event,
    record_grpc_request,
    record_wal_append,
    set_active_tenants,
    track_latency,
)


@pytest.fixture(autouse=True)
def _clear_metrics():
    """Reset metrics dict before each test."""
    original = metrics._metrics.copy()
    metrics._metrics.clear()
    yield
    metrics._metrics.clear()
    metrics._metrics.update(original)


class TestInitWithoutPrometheus:
    """Test graceful degradation when prometheus_client is missing."""

    def test_init_without_prometheus(self):
        """init_metrics returns False when prometheus_client is not available."""
        with mock.patch.object(metrics, "PROMETHEUS_AVAILABLE", False):
            result = init_metrics(port=9999)
            assert result is False


class TestRecordGrpcRequest:
    """Test gRPC request recording when metrics are not initialized."""

    def test_record_grpc_request_noop_when_not_initialized(self):
        """record_grpc_request does not crash when metrics dict is empty."""
        # Should be a no-op, no exception
        record_grpc_request(method="GetNode", status="ok", duration=0.05)


class TestRecordApplierEvent:
    """Test applier event recording."""

    def test_record_applier_event_noop_when_not_initialized(self):
        """record_applier_event does not crash when metrics dict is empty."""
        record_applier_event(status="applied")

    def test_record_applier_event_with_mock_counter(self):
        """record_applier_event calls inc() on the counter."""
        mock_counter = mock.MagicMock()
        metrics._metrics["applier_events"] = mock_counter
        record_applier_event(status="skipped")
        mock_counter.labels.assert_called_once_with(status="skipped")
        mock_counter.labels.return_value.inc.assert_called_once()


class TestRecordWalAppend:
    """Test WAL append recording."""

    def test_record_wal_append_noop_when_not_initialized(self):
        """record_wal_append does not crash when metrics dict is empty."""
        record_wal_append(status="ok", duration=0.01)

    def test_record_wal_append_with_mock(self):
        """record_wal_append calls inc() and observe()."""
        mock_counter = mock.MagicMock()
        mock_histogram = mock.MagicMock()
        metrics._metrics["wal_appends"] = mock_counter
        metrics._metrics["wal_append_latency"] = mock_histogram
        record_wal_append(status="ok", duration=0.042)
        mock_counter.labels.assert_called_once_with(status="ok")
        mock_counter.labels.return_value.inc.assert_called_once()
        mock_histogram.observe.assert_called_once_with(0.042)


class TestSetActiveTenants:
    """Test active tenants gauge."""

    def test_set_active_tenants_noop_when_not_initialized(self):
        """set_active_tenants does not crash when metrics dict is empty."""
        set_active_tenants(count=5)

    def test_set_active_tenants_with_mock(self):
        """set_active_tenants calls set() on the gauge."""
        mock_gauge = mock.MagicMock()
        metrics._metrics["active_tenants"] = mock_gauge
        set_active_tenants(count=42)
        mock_gauge.set.assert_called_once_with(42)


class TestTrackLatency:
    """Test latency tracking context manager."""

    def test_track_latency_noop_when_not_initialized(self):
        """track_latency does not crash when metrics dict is empty."""
        with track_latency("grpc_latency", method="GetNode"):
            time.sleep(0.001)

    def test_track_latency_with_mock(self):
        """track_latency calls observe() with elapsed duration."""
        mock_histogram = mock.MagicMock()
        metrics._metrics["grpc_latency"] = mock_histogram
        with track_latency("grpc_latency", method="GetNode"):
            time.sleep(0.01)
        mock_histogram.labels.assert_called_once_with(method="GetNode")
        args = mock_histogram.labels.return_value.observe.call_args
        observed_duration = args[0][0]
        assert observed_duration >= 0.005  # at least 5ms (generous lower bound)
