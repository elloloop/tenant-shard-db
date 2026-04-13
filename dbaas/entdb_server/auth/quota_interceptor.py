"""Three-layer rate limit interceptor.

Implements the quota model frozen in ``docs/decisions/quotas.md``:

1. **Phase 1 — Per-tenant monthly quota**. Durable, read from
   :class:`GlobalStore` with a short in-process cache so the hot path is
   a dict lookup on most requests.
2. **Phase 2 — Per-tenant token bucket**. In-memory fair-share RPS
   enforcement so a single tenant cannot starve a shared server.
3. **Phase 3 — Per-user token bucket**. Same algorithm keyed by
   ``(tenant_id, actor_id)`` to limit blast radius of a compromised
   credential.

Behaviour:
    - Requests that are not ``ExecuteAtomic`` pass through unchanged.
    - Tenants with no config, or ``max_writes_per_month == 0``, skip the
      monthly check. Likewise ``max_rps_sustained == 0`` / per-user
      limits == 0 skip the bucket checks.
    - Check order is fixed: monthly → tenant RPS → per-user RPS. The
      first denial wins. Soft monthly overage is a warning log that
      still falls through to the RPS checks.
    - On errors looking up quota state, the interceptor fails open: log
      and allow the request.

Invariants:
    - The interceptor never blocks writes when the quota backend is
      unavailable. Billing drift is preferable to outage.
    - Usage counters are incremented by the Applier post-apply, not by
      this interceptor. This interceptor is read-only w.r.t. durable
      state; the in-memory token buckets are the only mutable state.
    - Token-bucket state is per-process and in-memory only. A restart
      resets everyone's buckets; the ADR says this is acceptable
      because it grants one free burst, not an unlimited one.

How to change safely:
    - Keep ``intercept_service`` async. Blocking calls inside it will
      starve the gRPC event loop.
    - Cache TTLs should stay short (seconds) — long TTLs let tenants
      burst well past their limit before the cache refreshes.
    - The per-user bucket dict is evicted when it exceeds
      ``_USER_BUCKET_MAX_SIZE`` entries so it cannot grow unbounded.
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

# Cap on the per-user bucket dict. When exceeded we evict the oldest
# buckets (LRU by last update time) to bound memory. 10k entries is
# ~1MB worst-case, negligible compared to a request's memory cost.
_USER_BUCKET_MAX_SIZE = 10_000


class _TokenBucket:
    """Continuous-refill token bucket with an asyncio lock.

    Used for both the per-tenant (Phase 2) and per-user (Phase 3)
    rate-limit layers. The lock ensures concurrent ``try_consume`` calls
    from multiple async tasks cannot double-spend tokens.

    Attributes:
        capacity: Maximum tokens the bucket can hold (burst size).
        refill_per_second: Steady-state refill rate in tokens/second.
        tokens: Current token count (float for sub-token precision).
        last_update: Monotonic timestamp of the last refill calculation.
    """

    def __init__(self, capacity: float, refill_per_second: float):
        self.capacity = capacity
        self.refill_per_second = refill_per_second
        self.tokens = capacity
        self.last_update = time.monotonic()
        self.lock = asyncio.Lock()

    async def try_consume(self, n: float) -> tuple[bool, float]:
        """Attempt to consume ``n`` tokens.

        Returns ``(allowed, wait_seconds)``. When ``allowed`` is False,
        ``wait_seconds`` is an estimate of how long the caller should
        wait before enough tokens are available.
        """
        async with self.lock:
            now = time.monotonic()
            elapsed = now - self.last_update
            self.tokens = min(self.capacity, self.tokens + elapsed * self.refill_per_second)
            self.last_update = now
            if self.tokens >= n:
                self.tokens -= n
                return True, 0.0
            shortfall = n - self.tokens
            if self.refill_per_second <= 0:
                return False, float("inf")
            wait_s = shortfall / self.refill_per_second
            return False, wait_s


class QuotaInterceptor(grpc.aio.ServerInterceptor):
    """gRPC interceptor that enforces all three quota layers.

    Usage::

        interceptor = QuotaInterceptor(global_store)
        server = grpc.aio.server(interceptors=[auth_interceptor, interceptor])

    Order matters: the auth interceptor must run first so the quota
    interceptor can trust the tenant identifier on the request and read
    the authenticated actor out of the context-var.
    """

    def __init__(
        self,
        global_store: Any,
        *,
        config_ttl_s: float = _CONFIG_TTL_S,
        usage_ttl_s: float = _USAGE_TTL_S,
        user_bucket_max_size: int = _USER_BUCKET_MAX_SIZE,
    ) -> None:
        self._global_store = global_store
        self._cache = _QuotaCache(config_ttl_s=config_ttl_s, usage_ttl_s=usage_ttl_s)
        # Token-bucket state. These dicts are the only mutable state
        # on the interceptor. Both are scoped to the interceptor
        # instance so tests get isolated state.
        self._tenant_buckets: dict[str, _TokenBucket] = {}
        self._user_buckets: dict[tuple[str, str], _TokenBucket] = {}
        self._user_bucket_max_size = user_bucket_max_size

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
                allowed, retry_after_s, reason = await interceptor._check_all(tenant_id, n_ops)
                if not allowed:
                    # Attach Retry-After as trailing metadata so the SDK
                    # can surface it on RateLimitError.
                    try:
                        await context.set_trailing_metadata((("retry-after", str(retry_after_s)),))
                    except Exception:  # pragma: no cover - defensive
                        pass
                    await context.abort(
                        grpc.StatusCode.RESOURCE_EXHAUSTED,
                        f"rate limit exceeded: {reason}",
                    )
            return await handler.unary_unary(request, context) if handler.unary_unary else None

        # We only wrap unary-unary handlers; streaming RPCs are not
        # currently metered. If a streaming RPC starts appearing in
        # _ENFORCED_METHODS the wrapper above needs to be extended.
        #
        # Note: grpc.aio does NOT provide its own
        # unary_unary_rpc_method_handler constructor — we use the
        # top-level grpc.unary_unary_rpc_method_handler, which works
        # for both sync and async handler functions.
        if handler.unary_unary is not None:
            return grpc.unary_unary_rpc_method_handler(
                wrapper,
                request_deserializer=handler.request_deserializer,
                response_serializer=handler.response_serializer,
            )
        return handler

    async def _check(self, tenant_id: str, n_ops: int) -> tuple[bool, int, str]:
        """Backward-compatible alias for :meth:`_check_all`.

        Phase 1 only exposed ``_check`` as a single entry point that
        handled the monthly quota. Phase 2 / Phase 3 introduce the
        extra layers, but the public contract (``allowed, retry, reason``)
        is unchanged, so existing tests and callers that already hold a
        reference to ``_check`` still work.
        """
        return await self._check_all(tenant_id, n_ops)

    async def _check_all(self, tenant_id: str, n_ops: int) -> tuple[bool, int, str]:
        """Run all three layers in order. First denial wins.

        Returns ``(allowed, retry_after_seconds, reason_if_denied)``.
        """
        # Load config once per request; cached so this is usually a
        # single dict lookup.
        try:
            config = await self._cache.get_config(
                tenant_id, lambda: self._global_store.get_quota_config(tenant_id)
            )
        except Exception:
            logger.warning("quota config lookup failed for %s", tenant_id, exc_info=True)
            return True, 0, ""

        # Phase 1 — monthly quota.
        allowed, retry_after, reason = await self._check_monthly(tenant_id, n_ops, config)
        if not allowed:
            return False, retry_after, reason

        # Phase 2 — per-tenant token bucket.
        allowed, retry_after, reason = await self._check_rps(tenant_id, n_ops, config)
        if not allowed:
            return False, retry_after, reason

        # Phase 3 — per-user token bucket.
        allowed, retry_after, reason = await self._check_user_rps(tenant_id, n_ops, config)
        if not allowed:
            return False, retry_after, reason

        return True, 0, ""

    async def _check_monthly(
        self, tenant_id: str, n_ops: int, config: dict | None
    ) -> tuple[bool, int, str]:
        """Check the Phase 1 monthly write quota.

        Fails open (allows) on any backend error so a flapping
        ``global_store`` cannot take down writes.
        """
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
        reason = (
            f"monthly write quota exceeded: "
            f"{current}/{limit} writes this period, +{n_ops} would exceed"
        )
        if not config.get("hard_enforce", False):
            logger.warning("tenant %s over quota (soft enforce): %s", tenant_id, reason)
            return True, 0, ""
        return False, _seconds_until_next_period(), reason

    async def _check_rps(
        self, tenant_id: str, n_ops: int, config: dict | None
    ) -> tuple[bool, int, str]:
        """Phase 2: per-tenant token bucket."""
        sustained = int((config or {}).get("max_rps_sustained", 0) or 0)
        if sustained <= 0:
            # Unlimited — drop any cached bucket so a live config change
            # that removes the limit immediately stops consuming tokens.
            self._tenant_buckets.pop(tenant_id, None)
            return True, 0, ""
        burst = int((config or {}).get("max_rps_burst", 0) or 0)
        if burst <= 0:
            burst = sustained

        bucket = self._tenant_buckets.get(tenant_id)
        if bucket is None or bucket.capacity != burst or bucket.refill_per_second != sustained:
            # Rebuild the bucket when the config changes. The previous
            # bucket (if any) is dropped so the new limits take effect
            # immediately on the next request.
            bucket = _TokenBucket(capacity=float(burst), refill_per_second=float(sustained))
            self._tenant_buckets[tenant_id] = bucket

        allowed, wait_s = await bucket.try_consume(float(n_ops))
        if allowed:
            return True, 0, ""
        retry_after = max(1, int(wait_s + 0.999)) if wait_s != float("inf") else 60
        reason = f"tenant RPS bucket empty: {n_ops} tokens requested, wait ~{retry_after}s"
        return False, retry_after, reason

    async def _check_user_rps(
        self, tenant_id: str, n_ops: int, config: dict | None
    ) -> tuple[bool, int, str]:
        """Phase 3: per-user token bucket.

        Skipped when there is no trusted identity (e.g. a no-auth test
        deployment) because we refuse to key off untrusted client input.
        """
        sustained = int((config or {}).get("max_rps_per_user_sustained", 0) or 0)
        if sustained <= 0:
            return True, 0, ""

        try:
            from .auth_interceptor import get_current_identity
        except Exception:  # pragma: no cover - defensive
            return True, 0, ""

        actor_id = get_current_identity()
        if not actor_id:
            # No trusted identity — skip the per-user check. The
            # per-tenant bucket already protects the box.
            return True, 0, ""

        burst = int((config or {}).get("max_rps_per_user_burst", 0) or 0)
        if burst <= 0:
            burst = sustained

        key = (tenant_id, actor_id)
        bucket = self._user_buckets.get(key)
        if bucket is None or bucket.capacity != burst or bucket.refill_per_second != sustained:
            self._evict_user_buckets_if_needed()
            bucket = _TokenBucket(capacity=float(burst), refill_per_second=float(sustained))
            self._user_buckets[key] = bucket

        allowed, wait_s = await bucket.try_consume(float(n_ops))
        if allowed:
            return True, 0, ""
        retry_after = max(1, int(wait_s + 0.999)) if wait_s != float("inf") else 60
        reason = (
            f"per-user RPS bucket empty for {actor_id}: "
            f"{n_ops} tokens requested, wait ~{retry_after}s"
        )
        return False, retry_after, reason

    def _evict_user_buckets_if_needed(self) -> None:
        """Drop old user buckets when the dict exceeds the size cap.

        LRU-style: evict the quarter of entries with the oldest
        ``last_update`` timestamps. Cheap (O(n log n) in the unlikely
        case we hit the cap) and bounded.
        """
        if len(self._user_buckets) < self._user_bucket_max_size:
            return
        target_size = max(1, self._user_bucket_max_size * 3 // 4)
        # Sort keys by last_update ascending (oldest first) and drop
        # enough to bring the dict down to target_size.
        ordered = sorted(
            self._user_buckets.items(),
            key=lambda kv: kv[1].last_update,
        )
        drop = len(self._user_buckets) - target_size
        for key, _ in ordered[:drop]:
            self._user_buckets.pop(key, None)
        logger.info(
            "evicted %d per-user token buckets (size %d -> %d)",
            drop,
            len(self._user_buckets) + drop,
            len(self._user_buckets),
        )

    def invalidate(self, tenant_id: str) -> None:
        """Invalidate the cache for a tenant. Used by tests and after
        config / usage changes that need to take effect immediately.

        Also drops any token buckets keyed under this tenant so a config
        change picks up on the very next request.
        """
        self._cache.invalidate(tenant_id)
        self._tenant_buckets.pop(tenant_id, None)
        for key in [k for k in self._user_buckets if k[0] == tenant_id]:
            self._user_buckets.pop(key, None)


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
    "_TokenBucket",
]
