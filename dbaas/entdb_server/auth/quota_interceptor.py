"""Per-tenant monthly quota interceptor.

Phase 1 of the three-layer rate-limit model frozen in
``docs/decisions/quotas.md``. Enforces ``max_writes_per_month`` on
``ExecuteAtomic`` requests. Reads the config / usage from
:class:`GlobalStore` with a short in-process cache so the hot path
is a single dict lookup on most requests.

Behaviour:
    - Requests that are not ``ExecuteAtomic`` pass through unchanged.
    - Tenants with no config, or ``max_writes_per_month == 0``, are
      unlimited (no-op).
    - When ``hard_enforce=False`` (default), over-quota requests get a
      warning log but are allowed. Billing can charge for overage.
    - When ``hard_enforce=True``, over-quota requests are rejected with
      ``RESOURCE_EXHAUSTED`` and a ``Retry-After`` trailer giving the
      number of seconds until the next calendar-month period start.
    - On errors looking up quota state (e.g. ``GlobalStore`` flaps), the
      interceptor fails open: log and allow the request.

Invariants:
    - The interceptor never blocks writes when the quota backend is
      unavailable. Billing drift is preferable to outage.
    - Usage counters are incremented by the Applier post-apply, not by
      this interceptor. This interceptor is read-only.

How to change safely:
    - Keep ``intercept_service`` async. Blocking calls inside it will
      starve the gRPC event loop.
    - Cache TTLs should stay short (seconds) — long TTLs let tenants
      burst well past their limit before the cache refreshes.
"""

from __future__ import annotations

import asyncio
import datetime as _dt
import logging
import time
from collections.abc import Awaitable, Callable
from typing import Any

import grpc

logger = logging.getLogger(__name__)


# Default cache TTLs (seconds). Short on purpose: the whole point is to
# avoid hitting global_store on every request, not to batch updates.
_CONFIG_TTL_S = 30.0
_USAGE_TTL_S = 10.0


def _next_calendar_month_start_ms() -> int:
    now = _dt.datetime.now(tz=_dt.timezone.utc)
    if now.month == 12:
        nxt = _dt.datetime(now.year + 1, 1, 1, tzinfo=_dt.timezone.utc)
    else:
        nxt = _dt.datetime(now.year, now.month + 1, 1, tzinfo=_dt.timezone.utc)
    return int(nxt.timestamp() * 1000)


def _seconds_until_next_period() -> int:
    now_ms = int(time.time() * 1000)
    return max(1, (_next_calendar_month_start_ms() - now_ms) // 1000)


class _QuotaCache:
    """Tiny TTL cache for quota config and usage rows."""

    def __init__(self, config_ttl_s: float = _CONFIG_TTL_S, usage_ttl_s: float = _USAGE_TTL_S):
        self._config_ttl = config_ttl_s
        self._usage_ttl = usage_ttl_s
        self._config: dict[str, tuple[float, dict | None]] = {}
        self._usage: dict[str, tuple[float, dict]] = {}
        self._lock = asyncio.Lock()

    async def get_config(
        self, tenant_id: str, loader: Callable[[], Awaitable[dict | None]]
    ) -> dict | None:
        now = time.monotonic()
        hit = self._config.get(tenant_id)
        if hit is not None and now - hit[0] < self._config_ttl:
            return hit[1]
        async with self._lock:
            hit = self._config.get(tenant_id)
            if hit is not None and now - hit[0] < self._config_ttl:
                return hit[1]
            value = await loader()
            self._config[tenant_id] = (now, value)
            return value

    async def get_usage(self, tenant_id: str, loader: Callable[[], Awaitable[dict]]) -> dict:
        now = time.monotonic()
        hit = self._usage.get(tenant_id)
        if hit is not None and now - hit[0] < self._usage_ttl:
            return hit[1]
        async with self._lock:
            hit = self._usage.get(tenant_id)
            if hit is not None and now - hit[0] < self._usage_ttl:
                return hit[1]
            value = await loader()
            self._usage[tenant_id] = (now, value)
            return value

    def invalidate(self, tenant_id: str) -> None:
        self._config.pop(tenant_id, None)
        self._usage.pop(tenant_id, None)

    def clear(self) -> None:
        self._config.clear()
        self._usage.clear()


# Methods that this interceptor enforces quotas on. Reads pass through.
_ENFORCED_METHODS = frozenset({"ExecuteAtomic"})


class QuotaInterceptor(grpc.aio.ServerInterceptor):
    """gRPC interceptor that enforces per-tenant monthly write quotas.

    Usage::

        interceptor = QuotaInterceptor(global_store)
        server = grpc.aio.server(interceptors=[auth_interceptor, interceptor])

    Order matters: the auth interceptor must run first so the quota
    interceptor can trust the tenant identifier on the request.
    """

    def __init__(
        self,
        global_store: Any,
        *,
        config_ttl_s: float = _CONFIG_TTL_S,
        usage_ttl_s: float = _USAGE_TTL_S,
    ) -> None:
        self._global_store = global_store
        self._cache = _QuotaCache(config_ttl_s=config_ttl_s, usage_ttl_s=usage_ttl_s)

    async def intercept_service(
        self,
        continuation: Callable[[grpc.HandlerCallDetails], Awaitable[grpc.RpcMethodHandler | None]],
        handler_call_details: grpc.HandlerCallDetails,
    ) -> grpc.RpcMethodHandler | None:
        method = handler_call_details.method or ""
        rpc_name = method.rsplit("/", 1)[-1]
        if rpc_name not in _ENFORCED_METHODS:
            return await continuation(handler_call_details)

        handler = await continuation(handler_call_details)
        if handler is None:
            return None

        interceptor = self

        async def wrapper(request: Any, context: grpc.aio.ServicerContext) -> Any:
            tenant_id = _extract_tenant_id(request)
            n_ops = _extract_ops_count(request)
            if tenant_id and n_ops > 0:
                allowed, retry_after_s, reason = await interceptor._check(tenant_id, n_ops)
                if not allowed:
                    # Attach Retry-After as trailing metadata so the SDK
                    # can surface it on RateLimitError.
                    try:
                        await context.set_trailing_metadata((("retry-after", str(retry_after_s)),))
                    except Exception:  # pragma: no cover - defensive
                        pass
                    await context.abort(
                        grpc.StatusCode.RESOURCE_EXHAUSTED,
                        f"monthly write quota exceeded: {reason}",
                    )
            return await handler.unary_unary(request, context) if handler.unary_unary else None

        # We only wrap unary-unary handlers; streaming RPCs are not
        # currently metered. If a streaming RPC starts appearing in
        # _ENFORCED_METHODS the wrapper above needs to be extended.
        if handler.unary_unary is not None:
            return grpc.aio.unary_unary_rpc_method_handler(
                wrapper,
                request_deserializer=handler.request_deserializer,
                response_serializer=handler.response_serializer,
            )
        return handler

    async def _check(self, tenant_id: str, n_ops: int) -> tuple[bool, int, str]:
        """Return ``(allowed, retry_after_seconds, reason_if_denied)``.

        Fails open (allows) on any backend error so a flapping
        ``global_store`` cannot take down writes.
        """
        try:
            config = await self._cache.get_config(
                tenant_id, lambda: self._global_store.get_quota_config(tenant_id)
            )
        except Exception:
            logger.warning("quota config lookup failed for %s", tenant_id, exc_info=True)
            return True, 0, ""

        if config is None or config.get("max_writes_per_month", 0) <= 0:
            return True, 0, ""  # unlimited

        try:
            usage = await self._cache.get_usage(
                tenant_id, lambda: self._global_store.get_usage(tenant_id)
            )
        except Exception:
            logger.warning("quota usage lookup failed for %s", tenant_id, exc_info=True)
            return True, 0, ""

        limit = int(config["max_writes_per_month"])
        current = int(usage.get("writes_count", 0))
        projected = current + n_ops
        if projected <= limit:
            return True, 0, ""

        # Over quota. Soft mode: warn and allow. Hard mode: reject.
        reason = f"{current}/{limit} writes this period, +{n_ops} would exceed"
        if not config.get("hard_enforce", False):
            logger.warning("tenant %s over quota (soft enforce): %s", tenant_id, reason)
            return True, 0, ""
        return False, _seconds_until_next_period(), reason

    def invalidate(self, tenant_id: str) -> None:
        """Invalidate the cache for a tenant. Used by tests and after
        config / usage changes that need to take effect immediately."""
        self._cache.invalidate(tenant_id)


def _extract_tenant_id(request: Any) -> str | None:
    """Pull a tenant_id out of an ExecuteAtomic (or similar) request.

    The request message has ``context.tenant_id`` in current protos.
    Older protos have ``tenant_id`` at the top level. Handle both.
    """
    ctx = getattr(request, "context", None)
    if ctx is not None:
        tid = getattr(ctx, "tenant_id", None)
        if tid:
            return tid
    return getattr(request, "tenant_id", None) or None


def _extract_ops_count(request: Any) -> int:
    """Count the number of ops in an ExecuteAtomic request.

    We meter per op, not per request, so a batch of 10 creates counts
    as 10 writes. Prevents a trivial batch-gaming attack on the quota.
    """
    ops = getattr(request, "operations", None)
    if ops is None:
        ops = getattr(request, "ops", None)
    try:
        return len(ops) if ops is not None else 0
    except TypeError:
        return 0


__all__ = [
    "QuotaInterceptor",
    "_QuotaCache",
]
