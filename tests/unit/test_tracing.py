"""
Unit tests for OpenTelemetry tracing module.

Tests cover:
- Graceful behavior when opentelemetry is not installed
- No-op span behavior without initialization
- Tracer returns None without initialization
- ObservabilityConfig tracing defaults
- ObservabilityConfig tracing from environment variables
"""

import os
from unittest import mock

import pytest

from dbaas.entdb_server import tracing
from dbaas.entdb_server.config import ObservabilityConfig
from dbaas.entdb_server.tracing import get_tracer, span


@pytest.fixture(autouse=True)
def _reset_tracer():
    """Reset global tracer before each test."""
    original = tracing._tracer
    tracing._tracer = None
    yield
    tracing._tracer = original


class TestInitWithoutOtel:
    """Test graceful degradation when opentelemetry is missing."""

    def test_init_without_otel(self):
        """init_tracing returns False when opentelemetry is not available."""
        with mock.patch.object(tracing, "OTEL_AVAILABLE", False):
            result = tracing.init_tracing()
            assert result is False


class TestSpanNoopWithoutInit:
    """Test that span is a no-op when tracing is not initialized."""

    def test_span_noop_without_init(self):
        """span() yields None when tracer is not initialized."""
        with span("test-span", key="value") as s:
            assert s is None


class TestGetTracerNoneWithoutInit:
    """Test that get_tracer returns None without initialization."""

    def test_get_tracer_none_without_init(self):
        """get_tracer() returns None when tracing is not initialized."""
        assert get_tracer() is None


class TestConfigDefaults:
    """Test ObservabilityConfig tracing defaults."""

    def test_config_defaults(self):
        """ObservabilityConfig has correct tracing defaults."""
        config = ObservabilityConfig()
        assert config.trace_enabled is False
        assert config.trace_sampling_rate == 0.1
        assert config.trace_exporter == "console"
        assert config.trace_endpoint is None


class TestConfigFromEnv:
    """Test ObservabilityConfig tracing from environment variables."""

    def test_config_from_env(self):
        """ObservabilityConfig.from_env() reads tracing env vars."""
        env = {
            "TRACE_ENABLED": "true",
            "TRACE_SAMPLING_RATE": "0.5",
            "TRACE_EXPORTER": "otlp",
            "TRACE_ENDPOINT": "http://collector:4317",
        }
        with mock.patch.dict(os.environ, env, clear=False):
            config = ObservabilityConfig.from_env()
            assert config.trace_enabled is True
            assert config.trace_sampling_rate == 0.5
            assert config.trace_exporter == "otlp"
            assert config.trace_endpoint == "http://collector:4317"
