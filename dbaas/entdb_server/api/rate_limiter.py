"""
gRPC rate limiter interceptor.

Token bucket per tenant. Configurable via env vars:
    RATE_LIMIT_ENABLED=true
    RATE_LIMIT_RPS=100          # requests per second per tenant
    RATE_LIMIT_BURST=200        # burst capacity

Limitation: rate limiting is keyed on x-tenant-id metadata. If a client
omits or forges the header, it falls back to the "unknown" bucket. Proper
per-request rate limiting (e.g. by IP or auth identity) requires a
different architecture such as a sidecar proxy or API gateway.
"""

from __future__ import annotations

import logging
import time
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

    def __init__(self, rate: float = 100.0, burst: int = 200, max_tenants: int = 10000) -> None:
        self._rate = rate
        self._burst = burst
        self._max_tenants = max_tenants
        self._buckets: dict[str, TokenBucket] = {}
        self._access_order: list[str] = []

    def _get_bucket(self, tenant_id: str) -> TokenBucket:
        """Get or create a token bucket for a tenant, evicting oldest if at capacity."""
        if tenant_id in self._buckets:
            return self._buckets[tenant_id]

        # Evict oldest entries if at capacity
        while len(self._buckets) >= self._max_tenants and self._access_order:
            oldest = self._access_order.pop(0)
            self._buckets.pop(oldest, None)

        bucket = TokenBucket(self._rate, self._burst)
        self._buckets[tenant_id] = bucket
        self._access_order.append(tenant_id)
        return bucket

    def intercept_service(self, continuation, handler_call_details):
        method = handler_call_details.method
        if method in self.UNLIMITED_METHODS:
            return continuation(handler_call_details)

        # Extract tenant from metadata
        metadata = dict(handler_call_details.invocation_metadata or [])
        tenant_id = metadata.get("x-tenant-id", "unknown")

        if not self._get_bucket(tenant_id).consume():

            def _rate_limited(request, context):
                context.abort(
                    grpc.StatusCode.RESOURCE_EXHAUSTED,
                    f"Rate limit exceeded for tenant {tenant_id}",
                )

            return grpc.unary_unary_rpc_method_handler(_rate_limited)

        return continuation(handler_call_details)
