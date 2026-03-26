"""Unit tests for gRPC rate limiter."""

import time
from unittest.mock import MagicMock

import pytest

from dbaas.entdb_server.api.rate_limiter import RateLimitInterceptor, TokenBucket
from dbaas.entdb_server.config import GrpcConfig


@pytest.mark.unit
class TestTokenBucket:
    def test_allows_within_limit(self):
        bucket = TokenBucket(rate=10.0, burst=10)
        for _ in range(10):
            assert bucket.consume() is True

    def test_rejects_over_limit(self):
        bucket = TokenBucket(rate=10.0, burst=5)
        for _ in range(5):
            assert bucket.consume() is True
        assert bucket.consume() is False

    def test_burst_capacity(self):
        bucket = TokenBucket(rate=1.0, burst=50)
        consumed = 0
        for _ in range(50):
            if bucket.consume():
                consumed += 1
        assert consumed == 50
        assert bucket.consume() is False

    def test_refill_over_time(self):
        bucket = TokenBucket(rate=1000.0, burst=5)
        # Exhaust all tokens
        for _ in range(5):
            bucket.consume()
        assert bucket.consume() is False

        # Wait for refill (1000 rps = 1 token per ms)
        time.sleep(0.05)
        assert bucket.consume() is True


@pytest.mark.unit
class TestRateLimitInterceptor:
    def _make_details(self, method, tenant_id=None):
        details = MagicMock()
        details.method = method
        metadata = []
        if tenant_id:
            metadata.append(("x-tenant-id", tenant_id))
        details.invocation_metadata = metadata
        return details

    def test_allows_within_limit(self):
        interceptor = RateLimitInterceptor(rate=100.0, burst=10)
        continuation = MagicMock(return_value="handler")
        details = self._make_details("/entdb.EntDBService/GetNode", "tenant-1")

        for _ in range(10):
            result = interceptor.intercept_service(continuation, details)
            assert result == "handler"

    def test_rejects_over_limit(self):
        interceptor = RateLimitInterceptor(rate=100.0, burst=3)
        continuation = MagicMock(return_value="handler")
        details = self._make_details("/entdb.EntDBService/GetNode", "tenant-1")

        # Exhaust burst
        for _ in range(3):
            interceptor.intercept_service(continuation, details)

        # Next request should be rejected
        result = interceptor.intercept_service(continuation, details)
        assert result is not None
        assert result != "handler"

    def test_per_tenant_isolation(self):
        interceptor = RateLimitInterceptor(rate=100.0, burst=2)
        continuation = MagicMock(return_value="handler")

        details_a = self._make_details("/entdb.EntDBService/GetNode", "tenant-a")
        details_b = self._make_details("/entdb.EntDBService/GetNode", "tenant-b")

        # Exhaust tenant-a
        for _ in range(2):
            interceptor.intercept_service(continuation, details_a)
        result_a = interceptor.intercept_service(continuation, details_a)
        assert result_a != "handler"

        # tenant-b should still work
        result_b = interceptor.intercept_service(continuation, details_b)
        assert result_b == "handler"

    def test_burst_capacity(self):
        interceptor = RateLimitInterceptor(rate=1.0, burst=20)
        continuation = MagicMock(return_value="handler")
        details = self._make_details("/entdb.EntDBService/GetNode", "tenant-1")

        # Should be able to consume full burst
        for _ in range(20):
            result = interceptor.intercept_service(continuation, details)
            assert result == "handler"

        # Next request should be rejected
        result = interceptor.intercept_service(continuation, details)
        assert result != "handler"

    def test_refill_over_time(self):
        interceptor = RateLimitInterceptor(rate=1000.0, burst=2)
        continuation = MagicMock(return_value="handler")
        details = self._make_details("/entdb.EntDBService/GetNode", "tenant-1")

        # Exhaust burst
        for _ in range(2):
            interceptor.intercept_service(continuation, details)
        assert interceptor.intercept_service(continuation, details) != "handler"

        # Wait for refill
        time.sleep(0.05)
        result = interceptor.intercept_service(continuation, details)
        assert result == "handler"

    def test_health_bypasses_limit(self):
        interceptor = RateLimitInterceptor(rate=1.0, burst=1)
        continuation = MagicMock(return_value="handler")

        # Exhaust the bucket for this tenant
        normal_details = self._make_details("/entdb.EntDBService/GetNode", "tenant-1")
        interceptor.intercept_service(continuation, normal_details)
        assert interceptor.intercept_service(continuation, normal_details) != "handler"

        # Health check should bypass
        health_details = self._make_details("/entdb.EntDBService/Health", "tenant-1")
        result = interceptor.intercept_service(continuation, health_details)
        assert result == "handler"

        # grpc health check should also bypass
        grpc_health = self._make_details("/grpc.health.v1.Health/Check", "tenant-1")
        result = interceptor.intercept_service(continuation, grpc_health)
        assert result == "handler"


@pytest.mark.unit
class TestRateLimitConfig:
    def test_config_defaults(self):
        config = GrpcConfig()
        assert config.rate_limit_enabled is False
        assert config.rate_limit_rps == 100.0
        assert config.rate_limit_burst == 200

    def test_config_from_env(self, monkeypatch):
        monkeypatch.setenv("RATE_LIMIT_ENABLED", "true")
        monkeypatch.setenv("RATE_LIMIT_RPS", "50")
        monkeypatch.setenv("RATE_LIMIT_BURST", "100")
        config = GrpcConfig.from_env()
        assert config.rate_limit_enabled is True
        assert config.rate_limit_rps == 50.0
        assert config.rate_limit_burst == 100
