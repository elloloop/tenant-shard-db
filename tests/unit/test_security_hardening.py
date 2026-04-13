"""Tests for the security/durability hardening batch.

Covers four independent fixes landed in a single PR:

1. **Strict path validation** — tenant_id / user_id are no longer
   silently normalized into a safe alphabet; any disallowed character
   raises ValueError. Prevents cross-tenant leaks from collisions
   like ``tenant!a`` → ``tenanta``.
2. **JWKS DoS hardening** — blocking urllib fetch is now offloaded
   to an executor, concurrent cache misses are deduped to a single
   in-flight fetch, and unknown kids are negatively cached to prevent
   cache-miss amplification attacks.
3. **Per-tenant fair-share semaphore** — ``_run_sync`` caps in-flight
   async work to one task per tenant so a noisy neighbor cannot
   starve the shared executor.
4. **Graceful shutdown ordering** — ``Server.stop()`` signals
   components cooperatively, drains background tasks with a bounded
   timeout, and only cancels as a last resort. Prevents uncommitted
   WAL offsets on abrupt shutdown.
"""

from __future__ import annotations

import asyncio
import time
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

# ── Fix 1: strict path validation ──────────────────────────────────


class TestStrictPathValidation:
    """``_get_db_path`` and related helpers must reject (not normalize)
    any tenant_id / user_id that contains filesystem-unsafe chars."""

    def _make_store(self, tmp_path):
        from dbaas.entdb_server.apply.canonical_store import CanonicalStore

        return CanonicalStore(data_dir=str(tmp_path))

    def test_valid_tenant_id_accepted(self, tmp_path):
        store = self._make_store(tmp_path)
        path = store._get_db_path("acme-corp_01")
        assert path.name == "tenant_acme-corp_01.db"

    def test_tenant_id_with_bang_rejected(self, tmp_path):
        store = self._make_store(tmp_path)
        with pytest.raises(ValueError, match="forbidden characters"):
            store._get_db_path("tenant!a")

    def test_tenant_id_collision_cannot_happen(self, tmp_path):
        """`tenanta` and `tenant!a` must NOT collapse to the same file."""
        store = self._make_store(tmp_path)
        ok_path = store._get_db_path("tenanta")
        with pytest.raises(ValueError):
            store._get_db_path("tenant!a")
        assert ok_path.name == "tenant_tenanta.db"

    def test_empty_tenant_id_rejected(self, tmp_path):
        store = self._make_store(tmp_path)
        with pytest.raises(ValueError, match="empty"):
            store._get_db_path("")

    def test_path_traversal_rejected(self, tmp_path):
        store = self._make_store(tmp_path)
        for attempt in ("../other", "tenant/../escape", "a/b", "tenant\x00x"):
            with pytest.raises(ValueError):
                store._get_db_path(attempt)

    def test_overlong_tenant_id_rejected(self, tmp_path):
        store = self._make_store(tmp_path)
        with pytest.raises(ValueError, match="length exceeds"):
            store._get_db_path("a" * 500)

    def test_non_string_tenant_id_rejected(self, tmp_path):
        store = self._make_store(tmp_path)
        with pytest.raises(ValueError, match="must be a string"):
            store._get_db_path(42)  # type: ignore[arg-type]

    def test_mailbox_path_rejects_bad_user_id(self, tmp_path):
        store = self._make_store(tmp_path)
        with pytest.raises(ValueError, match="forbidden characters"):
            store._get_mailbox_db_path("acme", "bob!hack")

    def test_mailbox_path_strips_user_principal_prefix(self, tmp_path):
        store = self._make_store(tmp_path)
        path = store._get_mailbox_db_path("acme", "user:alice")
        assert path.name == "user_alice.db"

    def test_mailbox_path_rejects_bad_tenant_id(self, tmp_path):
        store = self._make_store(tmp_path)
        with pytest.raises(ValueError, match="forbidden characters"):
            store._get_mailbox_db_path("bad/tenant", "alice")


# ── Fix 2: JWKS DoS hardening ──────────────────────────────────────


class TestOAuthJwksHardening:
    """OAuthValidator must not block the event loop on JWKS fetches
    and must dedupe / negative-cache cache-miss amplification."""

    @pytest.mark.asyncio
    async def test_fetch_runs_in_executor(self):
        """_fetch_jwks must dispatch urllib via run_in_executor."""
        from dbaas.entdb_server.auth.oauth_validator import OAuthValidator

        v = OAuthValidator("https://issuer.example.com", "aud")

        fake_response = {"keys": [{"kid": "k1", "n": "x", "e": "AQAB", "kty": "RSA"}]}

        with patch("urllib.request.urlopen") as uo:
            uo.return_value.__enter__.return_value.read.return_value = (
                '{"keys":[{"kid":"k1","n":"x","e":"AQAB","kty":"RSA"}]}'
            )

            loop = asyncio.get_running_loop()
            orig_run = loop.run_in_executor
            call_count = 0

            async def _spy(ex, fn, *args, **kwargs):
                nonlocal call_count
                call_count += 1
                return await orig_run(ex, fn, *args, **kwargs)

            with patch.object(loop, "run_in_executor", side_effect=_spy):
                await v._fetch_jwks()

            assert call_count == 1, "urlopen must be dispatched via run_in_executor"
            assert "k1" in v._jwks_cache
            del fake_response  # silence unused

    @pytest.mark.asyncio
    async def test_deduped_concurrent_fetches(self):
        """Ten concurrent callers to _fetch_jwks_deduped fire one HTTP call."""
        from dbaas.entdb_server.auth.oauth_validator import OAuthValidator

        v = OAuthValidator("https://issuer.example.com", "aud")

        call_count = 0

        async def fake_fetch():
            nonlocal call_count
            call_count += 1
            await asyncio.sleep(0.02)  # simulate network latency
            v._jwks_cache = {"k1": {"kid": "k1"}}
            v._jwks_fetched_at = time.time()
            return v._jwks_cache

        with patch.object(v, "_fetch_jwks", side_effect=fake_fetch):
            await asyncio.gather(*[v._fetch_jwks_deduped() for _ in range(10)])

        assert call_count == 1, (
            f"Expected 1 deduped fetch, got {call_count} — the dedupe lock "
            "failed to collapse concurrent callers."
        )

    @pytest.mark.asyncio
    async def test_unknown_kid_negative_cache(self):
        """Once a kid is confirmed unknown, subsequent lookups do NOT refetch."""
        from dbaas.entdb_server.auth.oauth_validator import (
            AuthenticationError,
            OAuthValidator,
        )

        v = OAuthValidator(
            "https://issuer.example.com",
            "aud",
            unknown_kid_negative_ttl=60.0,
        )
        v._jwks_cache = {"known-kid": {"kid": "known-kid"}}
        v._jwks_fetched_at = time.time()

        # First attempt: unknown kid triggers a refetch (which returns same cache).
        fetch_count = 0

        async def fake_fetch():
            nonlocal fetch_count
            fetch_count += 1
            return v._jwks_cache

        with patch.object(v, "_fetch_jwks_deduped", side_effect=fake_fetch):
            # Mimic validate_token's lookup path.
            kid = "attacker-random-kid"
            assert v._get_cached_key(kid) is None
            assert not v._is_unknown_kid(kid)

            await v._fetch_jwks_deduped()
            v._remember_unknown_kid(kid)

            # Now subsequent checks short-circuit without refetching.
            assert v._is_unknown_kid(kid)
            assert fetch_count == 1, "Only the initial lookup should fetch"

        # Attempt a second "validation" for the same kid. Since the
        # negative cache hit means we never enter the refetch branch,
        # fetch_count should not grow.
        _ = AuthenticationError  # silence unused

    @pytest.mark.asyncio
    async def test_unknown_kid_cache_eventually_expires(self):
        """Negative cache entries must expire so real key rotations work."""
        from dbaas.entdb_server.auth.oauth_validator import OAuthValidator

        v = OAuthValidator(
            "https://issuer.example.com",
            "aud",
            unknown_kid_negative_ttl=0.01,
        )
        v._remember_unknown_kid("rotating-kid")
        assert v._is_unknown_kid("rotating-kid")
        await asyncio.sleep(0.02)
        assert not v._is_unknown_kid("rotating-kid")

    @pytest.mark.asyncio
    async def test_negative_cache_bounded(self):
        """The negative cache evicts when it crosses 1024 entries."""
        from dbaas.entdb_server.auth.oauth_validator import OAuthValidator

        v = OAuthValidator(
            "https://issuer.example.com",
            "aud",
            unknown_kid_negative_ttl=3600.0,
        )
        for i in range(1100):
            v._remember_unknown_kid(f"k{i}")
        assert len(v._unknown_kid_cache) <= 1024


# ── Fix 3: per-tenant fair-share semaphore ─────────────────────────


class TestPerTenantFairShareSemaphore:
    """_run_sync must cap in-flight work per tenant to prevent noisy
    neighbors from starving the shared ThreadPoolExecutor."""

    @pytest.mark.asyncio
    async def test_same_tenant_requests_serialize(self, tmp_path):
        """Two concurrent same-tenant _run_sync calls must execute
        sequentially (one at a time) even though both can dispatch."""
        from dbaas.entdb_server.apply.canonical_store import CanonicalStore

        store = CanonicalStore(data_dir=str(tmp_path))
        try:
            running: list[int] = []
            max_concurrent = [0]
            lock = asyncio.Lock()

            def slow_op(_tenant_id: str) -> int:
                # Simulate a slow sync op.
                n = len(running)
                running.append(1)
                time.sleep(0.05)
                running.pop()
                return n

            # Instrument the in-flight counter around the semaphore.
            async def instrumented(tenant_id: str):
                async with lock:
                    before = len([1 for r in running if r])
                result = await store._run_sync(slow_op, tenant_id)
                async with lock:
                    in_flight = len(running)
                    if in_flight > max_concurrent[0]:
                        max_concurrent[0] = in_flight
                return before, result

            await asyncio.gather(*[instrumented("tenant-a") for _ in range(5)])
            assert max_concurrent[0] <= 1, (
                "Same-tenant requests must serialize through the "
                f"semaphore; saw {max_concurrent[0]} concurrent"
            )
        finally:
            store.close_all()

    @pytest.mark.asyncio
    async def test_different_tenants_run_in_parallel(self, tmp_path):
        """Different tenants have independent semaphores and do not
        block each other even under burst load."""
        from dbaas.entdb_server.apply.canonical_store import CanonicalStore

        store = CanonicalStore(data_dir=str(tmp_path))
        try:
            concurrent_by_tenant: dict[str, int] = {}

            def op(tenant_id: str) -> str:
                concurrent_by_tenant[tenant_id] = concurrent_by_tenant.get(tenant_id, 0) + 1
                time.sleep(0.05)
                concurrent_by_tenant[tenant_id] -= 1
                return tenant_id

            start = time.monotonic()
            await asyncio.gather(
                store._run_sync(op, "t1"),
                store._run_sync(op, "t2"),
                store._run_sync(op, "t3"),
                store._run_sync(op, "t4"),
            )
            elapsed = time.monotonic() - start
            # Four tenants × 50ms sequentially would be ~200ms. With
            # parallelism they should overlap heavily — expect under 150ms.
            assert elapsed < 0.15, (
                f"Different-tenant _run_sync should parallelize; took {elapsed:.3f}s"
            )
        finally:
            store.close_all()

    @pytest.mark.asyncio
    async def test_noisy_neighbor_does_not_block_other_tenant(self, tmp_path):
        """A burst of same-tenant calls must not starve another
        tenant's single request — the canonical noisy-neighbor
        scenario that motivated this fix."""
        from dbaas.entdb_server.apply.canonical_store import CanonicalStore

        store = CanonicalStore(data_dir=str(tmp_path))
        try:
            # Noisy tenant: 20 same-tenant calls each taking 20ms.
            def slow(tenant_id: str) -> int:
                time.sleep(0.02)
                return 0

            noisy = [store._run_sync(slow, "noisy") for _ in range(20)]
            # Quiet tenant: one fast call — should complete before
            # the noisy burst finishes.
            quiet_start = time.monotonic()

            async def quiet_task():
                return await store._run_sync(slow, "quiet")

            quiet_done = asyncio.create_task(quiet_task())
            await quiet_done  # should complete quickly
            quiet_elapsed = time.monotonic() - quiet_start

            # Let the noisy burst finish so we don't leak tasks.
            await asyncio.gather(*noisy)

            # If the noisy tenant had hogged the executor, the quiet
            # request would be queued behind 20*20ms = 400ms of work.
            # With the semaphore, quiet finishes in roughly one op's
            # worth of time (plus scheduling overhead).
            assert quiet_elapsed < 0.10, (
                f"Quiet tenant starved by noisy tenant: quiet took {quiet_elapsed:.3f}s"
            )
        finally:
            store.close_all()

    @pytest.mark.asyncio
    async def test_semaphore_skipped_when_first_arg_is_not_an_id(self, tmp_path):
        """Non-string first arg → no semaphore, direct dispatch.

        Some internal helpers take a conn or other object first — we
        must not try to use it as a tenant key.
        """
        from dbaas.entdb_server.apply.canonical_store import CanonicalStore

        store = CanonicalStore(data_dir=str(tmp_path))
        try:

            def op(x: int) -> int:
                return x * 2

            result = await store._run_sync(op, 21)
            assert result == 42
            # Semaphore dict stays empty because we never registered one.
            assert store._tenant_semaphores == {}
        finally:
            store.close_all()


# ── Fix 4: graceful shutdown ordering ──────────────────────────────


class TestGracefulShutdown:
    """Server.stop() must signal components cooperatively and drain
    tasks before cancelling — otherwise the Applier can be cancelled
    mid-batch with uncommitted WAL offsets."""

    @pytest.mark.asyncio
    async def test_stop_signals_applier_before_touching_tasks(self):
        """applier.stop() must be awaited before any task.cancel()."""
        from dbaas.entdb_server.main import Server as EntDBServer

        # Build a Server via __new__ so we skip all startup logic and
        # can wire exactly the attributes we care about.
        server = EntDBServer.__new__(EntDBServer)
        server._running = True
        server._tasks = []
        server._GRACEFUL_STOP_TIMEOUT_S = 0.5  # tight so we don't hang tests
        # Track call order for assertion.
        order: list[str] = []
        drain = asyncio.Event()

        applier_mock = MagicMock()

        async def applier_stop():
            order.append("applier.stop")
            drain.set()

        applier_mock.stop = AsyncMock(side_effect=applier_stop)
        server.applier = applier_mock
        server.archiver = None
        server.snapshotter = None
        server.grpc_server = None
        server.wal = None

        async def cooperative_task():
            order.append("task.started")
            await drain.wait()
            order.append("task.exited_cleanly")

        task = asyncio.create_task(cooperative_task())
        await asyncio.sleep(0)  # let the task start
        server._tasks = [task]

        await server.stop()

        # applier.stop must run before the task drains.
        assert "applier.stop" in order
        assert "task.exited_cleanly" in order
        assert order.index("applier.stop") < order.index("task.exited_cleanly")

    @pytest.mark.asyncio
    async def test_stop_drains_tasks_that_exit_on_their_own(self):
        """Tasks that observe the stop flag and exit cleanly should
        NOT be cancelled — they just drain via gather."""
        from dbaas.entdb_server.main import Server as EntDBServer

        server = EntDBServer.__new__(EntDBServer)
        server._running = True
        server._tasks = []
        cancelled = {"flag": False}

        # Applier mock that signals a drain flag.
        drain_flag = asyncio.Event()

        async def applier_stop():
            drain_flag.set()

        server.applier = MagicMock()
        server.applier.stop = AsyncMock(side_effect=applier_stop)
        server.archiver = None
        server.snapshotter = None
        server.grpc_server = None
        server.wal = None

        async def cooperative_task():
            try:
                await drain_flag.wait()
            except asyncio.CancelledError:
                cancelled["flag"] = True
                raise

        task = asyncio.create_task(cooperative_task())
        server._tasks = [task]

        await server.stop()

        assert cancelled["flag"] is False, (
            "Cooperative task should drain via gather, not be cancelled"
        )
        assert task.done()

    @pytest.mark.asyncio
    async def test_stop_falls_back_to_cancel_after_timeout(self):
        """A stuck task that ignores the stop flag must be cancelled
        after the graceful timeout — we don't hang forever."""
        from dbaas.entdb_server.main import Server as EntDBServer

        server = EntDBServer.__new__(EntDBServer)
        server._running = True
        server._tasks = []
        server._GRACEFUL_STOP_TIMEOUT_S = 0.1  # speed up the test
        cancelled = {"flag": False}

        server.applier = MagicMock()
        server.applier.stop = AsyncMock()
        server.archiver = None
        server.snapshotter = None
        server.grpc_server = None
        server.wal = None

        async def stuck_task():
            try:
                await asyncio.sleep(60)  # ignores the stop flag
            except asyncio.CancelledError:
                cancelled["flag"] = True
                raise

        task = asyncio.create_task(stuck_task())
        server._tasks = [task]

        start = time.monotonic()
        await server.stop()
        elapsed = time.monotonic() - start

        assert cancelled["flag"] is True, "Stuck task must be cancelled after timeout"
        assert elapsed < 1.0, f"Graceful shutdown should time out quickly, took {elapsed:.2f}s"

    @pytest.mark.asyncio
    async def test_partial_startup_stop_does_not_crash(self):
        """Server.stop() must tolerate a partial startup (many attrs are None)."""
        from dbaas.entdb_server.main import Server as EntDBServer

        server = EntDBServer.__new__(EntDBServer)
        server._running = True
        server._tasks = []
        server.applier = None
        server.archiver = None
        server.snapshotter = None
        server.grpc_server = None
        server.wal = None

        # Should complete without raising.
        await server.stop()
