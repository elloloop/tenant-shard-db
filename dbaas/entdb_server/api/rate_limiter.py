"""
gRPC rate limiter interceptor.

Token bucket per tenant. Configurable via env vars:
    RATE_LIMIT_ENABLED=true
    RATE_LIMIT_RPS=100          # requests per second per tenant
    RATE_LIMIT_BURST=200        # burst capacity
"""

from __future__ import annotations

import logging
import time
from collections import defaultdict
from threading import Lock

import grpc

logger = logging.getLogger(__name__)


class TokenBucket:
    """Thread-safe token bucket for rate limiting."""

    def __init__(self, rate: float, burst: int) -> None:
        self._rate = rate
        self._burst = burst
        self._tokens = float(burst)
        self._last_refill = time.monotonic()
        self._lock = Lock()

    def consume(self) -> bool:
        with self._lock:
            now = time.monotonic()
            elapsed = now - self._last_refill
            self._tokens = min(self._burst, self._tokens + elapsed * self._rate)
            self._last_refill = now
            if self._tokens >= 1:
                self._tokens -= 1
                return True
            return False


class RateLimitInterceptor(grpc.ServerInterceptor):
    """Per-tenant rate limiter."""

    UNLIMITED_METHODS = frozenset(
        {
            "/entdb.EntDBService/Health",
            "/grpc.health.v1.Health/Check",
        }
    )

    def __init__(self, rate: float = 100.0, burst: int = 200) -> None:
        self._rate = rate
        self._burst = burst
        self._buckets: dict[str, TokenBucket] = defaultdict(lambda: TokenBucket(rate, burst))

    def intercept_service(self, continuation, handler_call_details):
        method = handler_call_details.method
        if method in self.UNLIMITED_METHODS:
            return continuation(handler_call_details)

        # Extract tenant from metadata
        metadata = dict(handler_call_details.invocation_metadata or [])
        tenant_id = metadata.get("x-tenant-id", "unknown")

        if not self._buckets[tenant_id].consume():

            def _rate_limited(request, context):
                context.abort(
                    grpc.StatusCode.RESOURCE_EXHAUSTED,
                    f"Rate limit exceeded for tenant {tenant_id}",
                )

            return grpc.unary_unary_rpc_method_handler(_rate_limited)

        return continuation(handler_call_details)
