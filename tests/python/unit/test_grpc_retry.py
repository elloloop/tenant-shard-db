# SPDX-License-Identifier: AGPL-3.0-only
"""
Unit tests for the Python SDK's transient-retry policy
(``GrpcClient._retry`` and its helpers in ``_grpc_client.py``).

Covers the three properties added for issue #143 (PERF-7):

  * full-jitter exponential backoff (no thundering herd),
  * a per-call wall-clock retry budget,
  * a DEADLINE_EXCEEDED read-method allowlist (writes such as
    ExecuteAtomic must NEVER be retried on a timeout).

The jitter source is injected as a seeded ``random.Random`` so the
backoff durations asserted here are deterministic. ``asyncio.sleep``
is monkeypatched so the tests don't actually wait.
"""

from __future__ import annotations

import random

import grpc
import pytest

from entdb_sdk._grpc_client import (
    _DEADLINE_RETRYABLE_METHODS,
    GrpcClient,
    _full_jitter_backoff,
    _is_retryable,
    _short_method,
)

# ── Fake gRPC error / multicallable ─────────────────────────────────


class FakeRpcError(grpc.RpcError):
    """Minimal stand-in for grpc.aio.AioRpcError.

    ``_retry`` only needs ``.code()`` and (via
    ``extract_redirect_node``) ``.trailing_metadata()``.
    """

    def __init__(self, code: grpc.StatusCode) -> None:
        self._code = code

    def code(self) -> grpc.StatusCode:
        return self._code

    def trailing_metadata(self):  # noqa: D401 - matches grpc API
        return []


class FakeCall:
    """An awaitable bound-multicallable substitute.

    Raises ``FakeRpcError(code)`` for the first ``fail_n`` calls,
    then returns ``"ok"``. Records how many times it was invoked.
    Exposes ``_method`` so ``_short_method`` resolves the allowlist
    key just like a real grpc.aio multicallable.
    """

    def __init__(self, code: grpc.StatusCode, fail_n: int, method: str) -> None:
        self._code = code
        self._fail_n = fail_n
        self._method = f"/entdb.v1.EntDBService/{method}"
        self.calls = 0

    async def __call__(self, *args, **kwargs):
        self.calls += 1
        if self.calls <= self._fail_n:
            raise FakeRpcError(self._code)
        return "ok"


def _make_client(**kw) -> GrpcClient:
    # Seeded RNG -> deterministic jitter. No real network.
    return GrpcClient(retry_rng=random.Random(1234), **kw)


# ── _short_method ───────────────────────────────────────────────────


class TestShortMethod:
    def test_strips_path(self):
        fn = FakeCall(grpc.StatusCode.OK, 0, "GetNode")
        assert _short_method(fn) == "GetNode"

    def test_bytes_method(self):
        fn = FakeCall(grpc.StatusCode.OK, 0, "ExecuteAtomic")
        fn._method = fn._method.encode()
        assert _short_method(fn) == "ExecuteAtomic"

    def test_unknown_callable_returns_empty(self):
        assert _short_method(object()) == ""


# ── _is_retryable ───────────────────────────────────────────────────


class TestIsRetryable:
    def test_unavailable_retryable_for_reads_and_writes(self):
        for m in ("GetNode", "ExecuteAtomic", "ShareNode", "DeleteUser"):
            assert _is_retryable(grpc.StatusCode.UNAVAILABLE, m) is True

    def test_deadline_exceeded_retryable_only_for_read_allowlist(self):
        for m in ("GetNode", "QueryNodes", "Health", "WaitForOffset", "ExportUserData"):
            assert _is_retryable(grpc.StatusCode.DEADLINE_EXCEEDED, m) is True
        for m in ("ExecuteAtomic", "ShareNode", "DeleteUser", "CreateTenant"):
            assert _is_retryable(grpc.StatusCode.DEADLINE_EXCEEDED, m) is False

    def test_terminal_codes_never_retryable(self):
        for c in (
            grpc.StatusCode.NOT_FOUND,
            grpc.StatusCode.PERMISSION_DENIED,
            grpc.StatusCode.INVALID_ARGUMENT,
            grpc.StatusCode.ALREADY_EXISTS,
            grpc.StatusCode.INTERNAL,
        ):
            assert _is_retryable(c, "GetNode") is False

    def test_execute_atomic_not_in_read_allowlist(self):
        assert "ExecuteAtomic" not in _DEADLINE_RETRYABLE_METHODS
        assert "ShareNode" not in _DEADLINE_RETRYABLE_METHODS
        assert "GetNode" in _DEADLINE_RETRYABLE_METHODS


# ── _full_jitter_backoff ────────────────────────────────────────────


class _FixedFractionRandom(random.Random):
    """random.Random whose ``random()`` always returns ``frac``.

    ``Random.uniform(a, b)`` is ``a + (b - a) * self.random()``, so
    pinning ``random()`` pins the jitter draw without monkeypatching
    ``uniform`` (which trips ARG lint on the unused signature).
    """

    def __init__(self, frac: float) -> None:
        super().__init__()
        self._frac = frac

    def random(self) -> float:  # noqa: D401 - matches Random API
        return self._frac


class TestFullJitterBackoff:
    def test_bounded_by_ceiling(self):
        # Worst case (rng -> ~1.0) is essentially the per-attempt
        # ceiling: uniform(0, c) with random()=frac yields c*frac.
        rng = _FixedFractionRandom(1.0)
        for attempt, ceiling in ((0, 0.1), (1, 0.2), (2, 0.4), (3, 0.8)):
            assert _full_jitter_backoff(attempt, rng) == pytest.approx(ceiling)

    def test_capped_at_max_delay(self):
        rng = _FixedFractionRandom(1.0)
        # attempt 10 would be 0.1*1024=102.4s unjittered; must cap.
        assert _full_jitter_backoff(10, rng) == pytest.approx(5.0)

    def test_zero_fraction_is_zero(self):
        rng = _FixedFractionRandom(0.0)
        assert _full_jitter_backoff(5, rng) == 0.0

    def test_is_randomised_not_fixed(self):
        # Distinct seeds -> distinct draws: proves jitter is applied
        # (the pre-#143 code used a fixed 0.1*2**n with no jitter).
        a = _full_jitter_backoff(3, random.Random(1))
        b = _full_jitter_backoff(3, random.Random(2))
        assert a != b
        assert 0.0 <= a < 0.8
        assert 0.0 <= b < 0.8


# ── _retry: end-to-end behaviour ────────────────────────────────────


@pytest.mark.asyncio
class TestRetryLoop:
    async def test_unavailable_retries_then_succeeds(self, monkeypatch):
        slept: list[float] = []

        async def fake_sleep(d):
            slept.append(d)

        monkeypatch.setattr("entdb_sdk._grpc_client.asyncio.sleep", fake_sleep)
        client = _make_client(max_retries=3)
        fn = FakeCall(grpc.StatusCode.UNAVAILABLE, 2, "GetNode")

        result = await client._retry(fn)

        assert result == "ok"
        assert fn.calls == 3  # 2 failures + 1 success
        assert len(slept) == 2  # one backoff per retry
        # Full jitter: every sleep is within its attempt ceiling.
        assert 0.0 <= slept[0] < 0.1
        assert 0.0 <= slept[1] < 0.2

    async def test_exhausts_max_retries_and_raises(self, monkeypatch):
        async def fake_sleep(_d):
            pass

        monkeypatch.setattr("entdb_sdk._grpc_client.asyncio.sleep", fake_sleep)
        client = _make_client(max_retries=2)
        fn = FakeCall(grpc.StatusCode.UNAVAILABLE, 99, "GetNode")

        with pytest.raises(grpc.RpcError):
            await client._retry(fn)
        assert fn.calls == 3  # 1 initial + 2 retries

    async def test_execute_atomic_not_retried_on_deadline_exceeded(self, monkeypatch):
        async def fake_sleep(_d):
            raise AssertionError("must not sleep — write must not retry")

        monkeypatch.setattr("entdb_sdk._grpc_client.asyncio.sleep", fake_sleep)
        client = _make_client(max_retries=5)
        fn = FakeCall(grpc.StatusCode.DEADLINE_EXCEEDED, 99, "ExecuteAtomic")

        with pytest.raises(grpc.RpcError):
            await client._retry(fn)
        # Exactly one attempt: a write timing out is NOT retried.
        assert fn.calls == 1

    async def test_execute_atomic_still_retries_on_unavailable(self, monkeypatch):
        async def fake_sleep(_d):
            pass

        monkeypatch.setattr("entdb_sdk._grpc_client.asyncio.sleep", fake_sleep)
        client = _make_client(max_retries=3)
        fn = FakeCall(grpc.StatusCode.UNAVAILABLE, 1, "ExecuteAtomic")

        result = await client._retry(fn)
        assert result == "ok"
        assert fn.calls == 2  # UNAVAILABLE retried even for a write

    async def test_read_retried_on_deadline_exceeded(self, monkeypatch):
        async def fake_sleep(_d):
            pass

        monkeypatch.setattr("entdb_sdk._grpc_client.asyncio.sleep", fake_sleep)
        client = _make_client(max_retries=3)
        fn = FakeCall(grpc.StatusCode.DEADLINE_EXCEEDED, 1, "GetNode")

        result = await client._retry(fn)
        assert result == "ok"
        assert fn.calls == 2

    async def test_terminal_status_not_retried(self, monkeypatch):
        async def fake_sleep(_d):
            raise AssertionError("must not sleep on a terminal status")

        monkeypatch.setattr("entdb_sdk._grpc_client.asyncio.sleep", fake_sleep)
        client = _make_client(max_retries=5)
        fn = FakeCall(grpc.StatusCode.NOT_FOUND, 99, "GetNode")

        with pytest.raises(grpc.RpcError):
            await client._retry(fn)
        assert fn.calls == 1

    async def test_wall_clock_budget_stops_early(self, monkeypatch):
        # A monotonic clock that jumps 100s per call makes the budget
        # (default 30s) trip on the first backoff decision, so even
        # though max_retries=50 the loop bails after a single retry
        # decision rather than running the full schedule.
        ticks = iter([0.0, 100.0, 200.0, 300.0, 400.0])
        monkeypatch.setattr("entdb_sdk._grpc_client.time.monotonic", lambda: next(ticks))

        async def fake_sleep(_d):
            raise AssertionError("budget should trip before sleeping")

        monkeypatch.setattr("entdb_sdk._grpc_client.asyncio.sleep", fake_sleep)
        client = _make_client(max_retries=50)
        fn = FakeCall(grpc.StatusCode.UNAVAILABLE, 99, "GetNode")

        with pytest.raises(grpc.RpcError):
            await client._retry(fn)
        # 1 initial attempt; budget tripped before the first sleep.
        assert fn.calls == 1

    async def test_custom_budget_allows_more_attempts(self, monkeypatch):
        # With a generous budget the loop runs the full retry count.
        monkeypatch.setattr("entdb_sdk._grpc_client.time.monotonic", lambda: 0.0)

        async def fake_sleep(_d):
            pass

        monkeypatch.setattr("entdb_sdk._grpc_client.asyncio.sleep", fake_sleep)
        client = _make_client(max_retries=4, retry_budget=999.0)
        fn = FakeCall(grpc.StatusCode.UNAVAILABLE, 99, "GetNode")

        with pytest.raises(grpc.RpcError):
            await client._retry(fn)
        assert fn.calls == 5  # 1 initial + 4 retries

    async def test_non_positive_budget_falls_back_to_default(self):
        client = _make_client(retry_budget=0.0)
        assert client._retry_budget == 30.0
        client2 = _make_client(retry_budget=-5.0)
        assert client2._retry_budget == 30.0
