"""Tests for Phase 2 (tenant RPS) and Phase 3 (per-user RPS) quotas.

Covers the extension of the quota interceptor with in-memory token
buckets, the per-user eviction path, the multi-layer check ordering,
the applier post-apply increment hook, and the new ``GetTenantQuota``
RPC surface.

These tests intentionally stay off the gRPC wire — they call
``_check_all`` / ``_check_rps`` / ``_check_user_rps`` directly and
use in-process fakes. Wire-level coverage lives in the integration
suite and the existing ``test_quotas.py`` for Phase 1.
"""

from __future__ import annotations

import asyncio
import time
from unittest.mock import AsyncMock, MagicMock

import pytest

from dbaas.entdb_server.auth.auth_interceptor import (
    set_current_identity,
)
from dbaas.entdb_server.auth.quota_interceptor import (
    QuotaInterceptor,
    _next_calendar_month_start_ms,
    _TokenBucket,
)
from dbaas.entdb_server.global_store import GlobalStore, _calendar_month_start_ms

# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture
def gs(tmp_path):
    store = GlobalStore(str(tmp_path))
    try:
        yield store
    finally:
        store.close()


@pytest.fixture
def interceptor(gs):
    # Short TTLs so tests don't accidentally hit a stale cache.
    return QuotaInterceptor(gs, config_ttl_s=0.01, usage_ttl_s=0.01)


# ---------------------------------------------------------------------------
# _TokenBucket primitive
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bucket_consume_partial_allowed():
    b = _TokenBucket(capacity=10, refill_per_second=5)
    ok, wait = await b.try_consume(3)
    assert ok is True
    assert wait == 0.0
    assert b.tokens == pytest.approx(7, abs=0.1)


@pytest.mark.asyncio
async def test_bucket_consume_more_than_capacity_denied():
    b = _TokenBucket(capacity=5, refill_per_second=1)
    ok, wait = await b.try_consume(10)
    assert ok is False
    assert wait > 0


@pytest.mark.asyncio
async def test_bucket_refill_over_time_restores_tokens():
    b = _TokenBucket(capacity=10, refill_per_second=100)
    ok, _ = await b.try_consume(10)
    assert ok is True
    await asyncio.sleep(0.05)  # ~5 tokens
    ok, _ = await b.try_consume(2)
    assert ok is True


@pytest.mark.asyncio
async def test_bucket_sequential_consumes_drain():
    b = _TokenBucket(capacity=6, refill_per_second=0.001)
    for _ in range(6):
        ok, _ = await b.try_consume(1)
        assert ok is True
    ok, _ = await b.try_consume(1)
    assert ok is False


@pytest.mark.asyncio
async def test_bucket_concurrent_consumes_no_overspend():
    b = _TokenBucket(capacity=100, refill_per_second=0.001)

    async def take():
        return await b.try_consume(1)

    results = await asyncio.gather(*(take() for _ in range(150)))
    allowed = sum(1 for ok, _ in results if ok)
    assert allowed == 100


# ---------------------------------------------------------------------------
# Phase 2 — per-tenant RPS
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_tenant_rps_unlimited_passes_through(interceptor, gs):
    # No config row -> unlimited
    allowed, retry, reason = await interceptor._check_all("t-unknown", 100)
    assert allowed is True
    assert retry == 0
    assert reason == ""
    assert "t-unknown" not in interceptor._tenant_buckets


@pytest.mark.asyncio
async def test_tenant_rps_zero_sustained_unlimited(interceptor, gs):
    await gs.set_quota_config("t1", max_writes_per_month=0, max_rps_sustained=0)
    allowed, retry, reason = await interceptor._check_all("t1", 50)
    assert allowed is True


@pytest.mark.asyncio
async def test_tenant_rps_burst_zero_falls_back_to_sustained(interceptor, gs):
    await gs.set_quota_config("t1", max_rps_sustained=5, max_rps_burst=0)
    allowed, _, _ = await interceptor._check_all("t1", 5)
    assert allowed is True
    bucket = interceptor._tenant_buckets["t1"]
    assert bucket.capacity == 5
    assert bucket.refill_per_second == 5


@pytest.mark.asyncio
async def test_tenant_rps_over_burst_denied(interceptor, gs):
    await gs.set_quota_config("t1", max_rps_sustained=1, max_rps_burst=3)
    # Burn the bucket down
    allowed, _, _ = await interceptor._check_all("t1", 3)
    assert allowed is True
    # Next op should be denied with a positive retry-after
    allowed, retry, reason = await interceptor._check_all("t1", 3)
    assert allowed is False
    assert retry >= 1
    assert "tenant RPS" in reason


@pytest.mark.asyncio
async def test_tenant_rps_independent_buckets(interceptor, gs):
    await gs.set_quota_config("t1", max_rps_sustained=1, max_rps_burst=2)
    await gs.set_quota_config("t2", max_rps_sustained=1, max_rps_burst=2)

    allowed, _, _ = await interceptor._check_all("t1", 2)
    assert allowed is True
    allowed, _, _ = await interceptor._check_all("t1", 2)
    assert allowed is False
    # t2 is independent
    allowed, _, _ = await interceptor._check_all("t2", 2)
    assert allowed is True


@pytest.mark.asyncio
async def test_tenant_rps_config_change_rebuilds_bucket(interceptor, gs):
    await gs.set_quota_config("t1", max_rps_sustained=1, max_rps_burst=2)
    await interceptor._check_all("t1", 2)  # drain

    # Tenant raises their limits; invalidate so the cache sees the new config
    await gs.set_quota_config("t1", max_rps_sustained=100, max_rps_burst=200)
    interceptor.invalidate("t1")

    allowed, _, _ = await interceptor._check_all("t1", 50)
    assert allowed is True
    assert interceptor._tenant_buckets["t1"].capacity == 200


@pytest.mark.asyncio
async def test_tenant_rps_refill_time_restores_tokens(interceptor, gs):
    await gs.set_quota_config("t1", max_rps_sustained=100, max_rps_burst=100)
    await interceptor._check_all("t1", 100)  # drain
    allowed, _, _ = await interceptor._check_all("t1", 5)
    assert allowed is False
    await asyncio.sleep(0.2)  # ~20 tokens refilled
    allowed, _, _ = await interceptor._check_all("t1", 5)
    assert allowed is True


# ---------------------------------------------------------------------------
# Phase 3 — per-user RPS
# ---------------------------------------------------------------------------


@pytest.fixture
def with_identity():
    """Set a trusted identity on the auth context-var for the test.

    Each pytest-asyncio test runs in its own asyncio Task which in turn
    runs in a fresh ``contextvars`` copy, so mutations made here are
    automatically scoped to the test — no explicit teardown needed.
    (Trying to reset via a token from ``set_current_identity`` fails
    with ``ValueError: Token was created in a different Context`` when
    fixture setup and test run in different contexts.)
    """

    def _set(identity: str | None):
        set_current_identity(identity)

    yield _set
    # Explicitly clear at the end, which is a no-op if we were in a
    # fresh context (the common case under pytest-asyncio).
    set_current_identity(None)


@pytest.mark.asyncio
async def test_per_user_no_identity_skipped(interceptor, gs):
    await gs.set_quota_config("t1", max_rps_per_user_sustained=1, max_rps_per_user_burst=2)
    # No trusted identity context-var set at all.
    allowed, _, _ = await interceptor._check_all("t1", 10)
    assert allowed is True
    assert len(interceptor._user_buckets) == 0


@pytest.mark.asyncio
async def test_per_user_two_users_independent(interceptor, gs, with_identity):
    await gs.set_quota_config("t1", max_rps_per_user_sustained=1, max_rps_per_user_burst=2)

    with_identity("alice")
    allowed, _, _ = await interceptor._check_all("t1", 2)
    assert allowed is True
    # Alice exhausted
    allowed, _, _ = await interceptor._check_all("t1", 2)
    assert allowed is False

    # Bob is independent
    with_identity("bob")
    allowed, _, _ = await interceptor._check_all("t1", 2)
    assert allowed is True


@pytest.mark.asyncio
async def test_per_user_sustained_zero_unlimited(interceptor, gs, with_identity):
    await gs.set_quota_config(
        "t1",
        max_rps_sustained=100,
        max_rps_burst=100,
        max_rps_per_user_sustained=0,
    )
    with_identity("alice")
    allowed, _, _ = await interceptor._check_all("t1", 50)
    assert allowed is True
    assert len(interceptor._user_buckets) == 0


@pytest.mark.asyncio
async def test_per_user_enforced_separately_from_tenant(interceptor, gs, with_identity):
    # Tenant allows plenty; per-user is tight.
    await gs.set_quota_config(
        "t1",
        max_rps_sustained=1000,
        max_rps_burst=1000,
        max_rps_per_user_sustained=1,
        max_rps_per_user_burst=2,
    )
    with_identity("alice")
    allowed, _, _ = await interceptor._check_all("t1", 2)
    assert allowed is True
    allowed, retry, reason = await interceptor._check_all("t1", 2)
    assert allowed is False
    assert "per-user" in reason
    assert retry >= 1


@pytest.mark.asyncio
async def test_per_user_eviction_at_cap(gs, with_identity):
    interceptor = QuotaInterceptor(
        gs,
        config_ttl_s=60,
        usage_ttl_s=60,
        user_bucket_max_size=20,
    )
    await gs.set_quota_config(
        "t1",
        max_rps_per_user_sustained=1,
        max_rps_per_user_burst=10,
    )
    # Create 25 distinct users > cap of 20; triggers eviction back to 15.
    for i in range(25):
        with_identity(f"user{i}")
        await interceptor._check_all("t1", 1)
    assert len(interceptor._user_buckets) <= 20
    # And strictly less than 25, meaning eviction actually ran.
    assert len(interceptor._user_buckets) < 25


# ---------------------------------------------------------------------------
# Layer ordering
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_monthly_hard_over_beats_rps(interceptor, gs):
    await gs.set_quota_config(
        "t1",
        max_writes_per_month=1,
        hard_enforce=True,
        max_rps_sustained=1000,
        max_rps_burst=1000,
    )
    await gs.increment_usage("t1", 10)  # already over
    interceptor.invalidate("t1")
    allowed, retry, reason = await interceptor._check_all("t1", 1)
    assert allowed is False
    assert "monthly" in reason
    # Monthly denial gives a retry-after until the next calendar month.
    assert retry > 60  # at least a minute


@pytest.mark.asyncio
async def test_soft_monthly_falls_through_to_rps(interceptor, gs):
    await gs.set_quota_config(
        "t1",
        max_writes_per_month=1,
        hard_enforce=False,  # soft
        max_rps_sustained=1,
        max_rps_burst=1,
    )
    await gs.increment_usage("t1", 100)
    interceptor.invalidate("t1")
    # First call uses the 1-token bucket; allowed
    allowed, _, _ = await interceptor._check_all("t1", 1)
    assert allowed is True
    # Second call hits the RPS bucket, not monthly (soft logged)
    allowed, _, reason = await interceptor._check_all("t1", 1)
    assert allowed is False
    assert "tenant RPS" in reason


@pytest.mark.asyncio
async def test_all_unlimited_passes(interceptor, gs):
    await gs.set_quota_config("t1")
    allowed, retry, reason = await interceptor._check_all("t1", 1000)
    assert allowed is True
    assert retry == 0


@pytest.mark.asyncio
async def test_tenant_rps_then_user_rps_ordering(interceptor, gs, with_identity):
    await gs.set_quota_config(
        "t1",
        max_rps_sustained=1,
        max_rps_burst=2,
        max_rps_per_user_sustained=1,
        max_rps_per_user_burst=1000,
    )
    with_identity("alice")
    # 2 ops succeed (both buckets have room)
    allowed, _, _ = await interceptor._check_all("t1", 2)
    assert allowed is True
    # Tenant bucket now empty; per-user still has room.
    # The tenant bucket denial wins.
    allowed, _, reason = await interceptor._check_all("t1", 1)
    assert allowed is False
    assert "tenant RPS" in reason


# ---------------------------------------------------------------------------
# Applier post-apply increment hook
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_applier_increment_usage_hook_called():
    """The hook must call global_store.increment_usage with n_ops."""
    from dbaas.entdb_server.apply.applier import Applier

    mock_gs = MagicMock()
    mock_gs.increment_usage = AsyncMock(return_value={"writes_count": 3})

    applier = Applier.__new__(Applier)
    applier.global_store = mock_gs

    await applier._increment_usage_safe("t1", 3)
    mock_gs.increment_usage.assert_awaited_once_with("t1", 3)


@pytest.mark.asyncio
async def test_applier_increment_usage_swallows_errors():
    """A failed increment must NOT raise — apply must still succeed."""
    from dbaas.entdb_server.apply.applier import Applier

    mock_gs = MagicMock()
    mock_gs.increment_usage = AsyncMock(side_effect=RuntimeError("boom"))

    applier = Applier.__new__(Applier)
    applier.global_store = mock_gs

    # Must not raise.
    await applier._increment_usage_safe("t1", 5)
    mock_gs.increment_usage.assert_awaited_once()


@pytest.mark.asyncio
async def test_applier_increment_usage_noop_when_no_global_store():
    from dbaas.entdb_server.apply.applier import Applier

    applier = Applier.__new__(Applier)
    applier.global_store = None
    # Would raise if it tried to call .increment_usage on None.
    await applier._increment_usage_safe("t1", 5)


@pytest.mark.asyncio
async def test_applier_increment_usage_zero_ops_noop():
    from dbaas.entdb_server.apply.applier import Applier

    mock_gs = MagicMock()
    mock_gs.increment_usage = AsyncMock()

    applier = Applier.__new__(Applier)
    applier.global_store = mock_gs
    await applier._increment_usage_safe("t1", 0)
    mock_gs.increment_usage.assert_not_called()


# ---------------------------------------------------------------------------
# Config round-trip
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_set_quota_config_round_trips_rps_fields(gs):
    await gs.set_quota_config(
        "t1",
        max_writes_per_month=100,
        hard_enforce=True,
        max_rps_sustained=50,
        max_rps_burst=100,
        max_rps_per_user_sustained=5,
        max_rps_per_user_burst=10,
    )
    cfg = await gs.get_quota_config("t1")
    assert cfg["max_rps_sustained"] == 50
    assert cfg["max_rps_burst"] == 100
    assert cfg["max_rps_per_user_sustained"] == 5
    assert cfg["max_rps_per_user_burst"] == 10
    assert cfg["hard_enforce"] is True


@pytest.mark.asyncio
async def test_set_quota_config_defaults_zero(gs):
    await gs.set_quota_config("t1", max_writes_per_month=100)
    cfg = await gs.get_quota_config("t1")
    assert cfg["max_rps_sustained"] == 0
    assert cfg["max_rps_burst"] == 0
    assert cfg["max_rps_per_user_sustained"] == 0
    assert cfg["max_rps_per_user_burst"] == 0


# ---------------------------------------------------------------------------
# GetTenantQuota RPC
# ---------------------------------------------------------------------------


class _FakeCtx:
    """Minimal async servicer context fake for the handler path."""

    def __init__(self):
        self.aborted: tuple | None = None

    async def abort(self, code, detail):  # pragma: no cover - raises
        self.aborted = (code, detail)
        raise RuntimeError(f"abort {code}: {detail}")

    async def set_trailing_metadata(self, md):
        return None


@pytest.mark.asyncio
async def test_get_tenant_quota_returns_config_and_usage(gs):
    from dbaas.entdb_server.api.generated import GetTenantQuotaRequest
    from dbaas.entdb_server.api.grpc_server import EntDBServicer

    servicer = EntDBServicer.__new__(EntDBServicer)
    servicer.global_store = gs
    # Bypass the admin check — exercise the read path directly.
    servicer._require_admin_or_owner = AsyncMock(return_value="system:test")

    await gs.set_quota_config(
        "t1",
        max_writes_per_month=1000,
        hard_enforce=True,
        max_rps_sustained=50,
        max_rps_burst=100,
        max_rps_per_user_sustained=5,
        max_rps_per_user_burst=10,
    )
    await gs.increment_usage("t1", 250)

    req = GetTenantQuotaRequest(actor="system:test", tenant_id="t1")
    resp = await servicer.GetTenantQuota(req, _FakeCtx())

    assert resp.tenant_id == "t1"
    assert resp.max_writes_per_month == 1000
    assert resp.writes_used == 250
    assert resp.hard_enforce is True
    assert resp.max_rps_sustained == 50
    assert resp.max_rps_burst == 100
    assert resp.max_rps_per_user_sustained == 5
    assert resp.max_rps_per_user_burst == 10
    assert resp.period_start_ms == _calendar_month_start_ms()
    assert resp.period_end_ms == _next_calendar_month_start_ms()


@pytest.mark.asyncio
async def test_get_tenant_quota_unknown_returns_zeros(gs):
    from dbaas.entdb_server.api.generated import GetTenantQuotaRequest
    from dbaas.entdb_server.api.grpc_server import EntDBServicer

    servicer = EntDBServicer.__new__(EntDBServicer)
    servicer.global_store = gs
    servicer._require_admin_or_owner = AsyncMock(return_value="system:test")

    req = GetTenantQuotaRequest(actor="system:test", tenant_id="unknown")
    resp = await servicer.GetTenantQuota(req, _FakeCtx())
    assert resp.tenant_id == "unknown"
    assert resp.max_writes_per_month == 0
    assert resp.writes_used == 0
    assert resp.period_end_ms == _next_calendar_month_start_ms()


@pytest.mark.asyncio
async def test_get_tenant_quota_requires_admin(gs):
    from dbaas.entdb_server.api.generated import GetTenantQuotaRequest
    from dbaas.entdb_server.api.grpc_server import EntDBServicer

    servicer = EntDBServicer.__new__(EntDBServicer)
    servicer.global_store = gs

    # Make _require_admin_or_owner raise as it would for a non-admin.
    async def _deny(*args, **kwargs):
        raise PermissionError("not admin")

    servicer._require_admin_or_owner = _deny
    req = GetTenantQuotaRequest(actor="user:eve", tenant_id="t1")
    with pytest.raises(PermissionError):
        await servicer.GetTenantQuota(req, _FakeCtx())


@pytest.mark.asyncio
async def test_get_tenant_quota_period_end_is_next_month(gs):
    """period_end_ms should be the start-of-next-calendar-month."""
    from dbaas.entdb_server.api.generated import GetTenantQuotaRequest
    from dbaas.entdb_server.api.grpc_server import EntDBServicer

    servicer = EntDBServicer.__new__(EntDBServicer)
    servicer.global_store = gs
    servicer._require_admin_or_owner = AsyncMock(return_value="system:test")

    await gs.set_quota_config("t1", max_writes_per_month=1)
    req = GetTenantQuotaRequest(actor="system:test", tenant_id="t1")
    resp = await servicer.GetTenantQuota(req, _FakeCtx())
    # Next-month-start must be strictly after now.
    assert resp.period_end_ms > int(time.time() * 1000)
    # And strictly after the period start.
    assert resp.period_end_ms > resp.period_start_ms
