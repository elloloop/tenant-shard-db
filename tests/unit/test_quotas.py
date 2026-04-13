"""Tests for Phase 1 of the quota system (monthly write quota).

Covers:
    - GlobalStore quota config CRUD
    - GlobalStore usage counter + monthly reset
    - Quota interceptor cache TTL
    - Quota interceptor soft vs hard enforcement
    - Quota interceptor fail-open on backend errors
    - Calendar-month rollover

Does not yet cover gRPC-server integration (that happens when the
interceptor is wired into the servicer in a follow-up).
"""

from __future__ import annotations

import asyncio
import time
from unittest.mock import AsyncMock, MagicMock

import pytest

from dbaas.entdb_server.auth.quota_interceptor import (
    _extract_ops_count,
    _extract_tenant_id,
    _next_calendar_month_start_ms,
    _QuotaCache,
    _seconds_until_next_period,
)
from dbaas.entdb_server.global_store import GlobalStore, _calendar_month_start_ms

# ── GlobalStore quota tables ─────────────────────────────────────────


@pytest.fixture
def gs(tmp_path):
    """Fresh global store for each test."""
    store = GlobalStore(str(tmp_path))
    try:
        yield store
    finally:
        store.close()


@pytest.mark.asyncio
async def test_quota_config_default_missing(gs):
    """A tenant with no config row returns None (unlimited)."""
    assert await gs.get_quota_config("t1") is None


@pytest.mark.asyncio
async def test_quota_config_set_and_read(gs):
    cfg = await gs.set_quota_config("t1", max_writes_per_month=10_000, hard_enforce=True)
    assert cfg["tenant_id"] == "t1"
    assert cfg["max_writes_per_month"] == 10_000
    assert cfg["hard_enforce"] is True

    read_back = await gs.get_quota_config("t1")
    assert read_back is not None
    assert read_back["max_writes_per_month"] == 10_000
    assert read_back["hard_enforce"] is True


@pytest.mark.asyncio
async def test_quota_config_upsert(gs):
    """Setting a config twice overwrites the previous row."""
    await gs.set_quota_config("t1", max_writes_per_month=100)
    await gs.set_quota_config("t1", max_writes_per_month=200, hard_enforce=True)
    cfg = await gs.get_quota_config("t1")
    assert cfg["max_writes_per_month"] == 200
    assert cfg["hard_enforce"] is True


@pytest.mark.asyncio
async def test_usage_default_zero_for_new_tenant(gs):
    usage = await gs.get_usage("t-new")
    assert usage["writes_count"] == 0
    assert usage["period_start_ms"] == _calendar_month_start_ms()


@pytest.mark.asyncio
async def test_increment_usage_creates_row(gs):
    result = await gs.increment_usage("t1", 5)
    assert result["writes_count"] == 5

    usage = await gs.get_usage("t1")
    assert usage["writes_count"] == 5


@pytest.mark.asyncio
async def test_increment_usage_accumulates(gs):
    await gs.increment_usage("t1", 3)
    await gs.increment_usage("t1", 7)
    await gs.increment_usage("t1", 10)
    usage = await gs.get_usage("t1")
    assert usage["writes_count"] == 20


@pytest.mark.asyncio
async def test_increment_usage_zero_is_noop(gs):
    await gs.increment_usage("t1", 5)
    await gs.increment_usage("t1", 0)
    await gs.increment_usage("t1", -3)
    usage = await gs.get_usage("t1")
    assert usage["writes_count"] == 5


@pytest.mark.asyncio
async def test_calendar_month_rollover_resets_counter(gs):
    """Simulate a rollover by writing a stale period_start_ms directly
    and then calling increment_usage."""
    with gs._get_connection() as conn:
        conn.execute(
            "INSERT INTO tenant_usage (tenant_id, period_start_ms, writes_count, updated_at) "
            "VALUES (?, ?, ?, ?)",
            ("t1", 0, 10_000, 0),  # period_start_ms=0 is way in the past
        )

    result = await gs.increment_usage("t1", 1)
    assert result["writes_count"] == 1  # reset, not 10_001
    assert result["period_start_ms"] == _calendar_month_start_ms()


@pytest.mark.asyncio
async def test_reset_period_zeroes_counter(gs):
    await gs.increment_usage("t1", 500)
    usage = await gs.get_usage("t1")
    assert usage["writes_count"] == 500

    await gs.reset_period("t1")
    usage = await gs.get_usage("t1")
    assert usage["writes_count"] == 0
    assert usage["period_start_ms"] == _calendar_month_start_ms()


# ── _QuotaCache ──────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_cache_fetches_on_miss():
    loads = 0

    async def loader():
        nonlocal loads
        loads += 1
        return {"max_writes_per_month": 100}

    cache = _QuotaCache(config_ttl_s=60.0)
    value1 = await cache.get_config("t1", loader)
    value2 = await cache.get_config("t1", loader)
    assert value1 == value2
    assert loads == 1  # second call served from cache


@pytest.mark.asyncio
async def test_cache_refreshes_after_ttl():
    loads = 0

    async def loader():
        nonlocal loads
        loads += 1
        return {"max_writes_per_month": 100}

    cache = _QuotaCache(config_ttl_s=0.01)  # 10ms
    await cache.get_config("t1", loader)
    await asyncio.sleep(0.02)
    await cache.get_config("t1", loader)
    assert loads == 2


@pytest.mark.asyncio
async def test_cache_separate_keys():
    loads: list[str] = []

    async def loader_factory(tenant):
        async def loader():
            loads.append(tenant)
            return {"tenant": tenant}

        return loader

    cache = _QuotaCache(config_ttl_s=60.0)
    t1_loader = await loader_factory("t1")
    t2_loader = await loader_factory("t2")
    await cache.get_config("t1", t1_loader)
    await cache.get_config("t2", t2_loader)
    assert loads == ["t1", "t2"]


@pytest.mark.asyncio
async def test_cache_invalidate():
    loads = 0

    async def loader():
        nonlocal loads
        loads += 1
        return {"max_writes_per_month": 100}

    cache = _QuotaCache(config_ttl_s=60.0)
    await cache.get_config("t1", loader)
    cache.invalidate("t1")
    await cache.get_config("t1", loader)
    assert loads == 2


# ── Interceptor helpers ──────────────────────────────────────────────


def test_extract_tenant_from_context():
    request = MagicMock()
    request.context.tenant_id = "acme"
    assert _extract_tenant_id(request) == "acme"


def test_extract_tenant_fallback_to_top_level():
    request = MagicMock(spec=["tenant_id"])
    request.tenant_id = "beta"
    assert _extract_tenant_id(request) == "beta"


def test_extract_tenant_none():
    request = MagicMock(spec=[])
    assert _extract_tenant_id(request) is None


def test_extract_ops_count_from_operations():
    request = MagicMock()
    request.operations = [1, 2, 3]
    assert _extract_ops_count(request) == 3


def test_extract_ops_count_empty():
    request = MagicMock(spec=[])
    assert _extract_ops_count(request) == 0


def test_next_period_is_in_the_future():
    next_start = _next_calendar_month_start_ms()
    assert next_start > int(time.time() * 1000)


def test_seconds_until_next_period_positive():
    assert _seconds_until_next_period() > 0


# ── QuotaInterceptor._check behaviour ────────────────────────────────


def _make_interceptor(
    config: dict | None = None, usage: dict | None = None, raise_on_config: bool = False
):
    """Build a QuotaInterceptor backed by an async mock global store."""
    from dbaas.entdb_server.auth.quota_interceptor import QuotaInterceptor

    mock_store = AsyncMock()
    if raise_on_config:
        mock_store.get_quota_config.side_effect = RuntimeError("db down")
    else:
        mock_store.get_quota_config.return_value = config
    mock_store.get_usage.return_value = usage or {"writes_count": 0}
    return QuotaInterceptor(mock_store, config_ttl_s=60.0, usage_ttl_s=60.0)


@pytest.mark.asyncio
async def test_check_unlimited_tenant_passes():
    interceptor = _make_interceptor(config=None)
    allowed, _, _ = await interceptor._check("t1", 100)
    assert allowed is True


@pytest.mark.asyncio
async def test_check_zero_limit_is_unlimited():
    interceptor = _make_interceptor(config={"max_writes_per_month": 0, "hard_enforce": True})
    allowed, _, _ = await interceptor._check("t1", 1_000_000)
    assert allowed is True


@pytest.mark.asyncio
async def test_check_under_limit_passes():
    interceptor = _make_interceptor(
        config={"max_writes_per_month": 100, "hard_enforce": True},
        usage={"writes_count": 50},
    )
    allowed, _, _ = await interceptor._check("t1", 40)
    assert allowed is True


@pytest.mark.asyncio
async def test_check_at_limit_passes():
    interceptor = _make_interceptor(
        config={"max_writes_per_month": 100, "hard_enforce": True},
        usage={"writes_count": 99},
    )
    allowed, _, _ = await interceptor._check("t1", 1)
    assert allowed is True


@pytest.mark.asyncio
async def test_check_over_limit_hard_enforce_rejects():
    interceptor = _make_interceptor(
        config={"max_writes_per_month": 100, "hard_enforce": True},
        usage={"writes_count": 99},
    )
    allowed, retry_after, reason = await interceptor._check("t1", 2)
    assert allowed is False
    assert retry_after > 0
    assert "99/100" in reason


@pytest.mark.asyncio
async def test_check_over_limit_soft_enforce_allows(caplog):
    interceptor = _make_interceptor(
        config={"max_writes_per_month": 100, "hard_enforce": False},
        usage={"writes_count": 99},
    )
    allowed, _, _ = await interceptor._check("t1", 2)
    assert allowed is True
    # Soft-mode overage should produce a warning log.
    assert any("over quota" in rec.message for rec in caplog.records)


@pytest.mark.asyncio
async def test_check_config_lookup_error_fails_open():
    """Backend flap must never block writes."""
    interceptor = _make_interceptor(raise_on_config=True)
    allowed, _, _ = await interceptor._check("t1", 5)
    assert allowed is True


@pytest.mark.asyncio
async def test_check_counts_ops_not_requests():
    """A batch of 10 ops should consume 10 quota slots, not 1."""
    interceptor = _make_interceptor(
        config={"max_writes_per_month": 100, "hard_enforce": True},
        usage={"writes_count": 91},
    )
    # 1 request with 10 ops → 91 + 10 = 101 > 100 → reject
    allowed, _, _ = await interceptor._check("t1", 10)
    assert allowed is False
    # 1 request with 9 ops → 91 + 9 = 100 → accept (at limit)
    allowed, _, _ = await interceptor._check("t1", 9)
    assert allowed is True


# ── RateLimitError ───────────────────────────────────────────────────


def test_rate_limit_error_has_retry_after_and_limit():
    from sdk.entdb_sdk.errors import RateLimitError

    err = RateLimitError("quota exceeded", retry_after_ms=5000, limit=100, used=150)
    assert err.retry_after_ms == 5000
    assert err.limit == 100
    assert err.used == 150
    assert err.code == "RATE_LIMITED"


def test_rate_limit_error_exported_from_package():
    from sdk.entdb_sdk import RateLimitError as Exported

    assert Exported.__name__ == "RateLimitError"
