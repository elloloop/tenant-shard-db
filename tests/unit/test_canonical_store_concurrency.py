"""
Concurrency tests for CanonicalStore connection pool.

Regression coverage for the thread-unsafe ``_get_connection`` race that
allowed two threads to simultaneously open a fresh sqlite3 connection
for the same uncached tenant — leaking the orphan FD when one
overwrote the other in ``self._connections``.
"""

from __future__ import annotations

import sqlite3
import threading
from unittest.mock import patch

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore


@pytest.fixture()
def store(tmp_path):
    """CanonicalStore backed by a temp directory."""
    s = CanonicalStore(str(tmp_path))
    yield s
    s.close_all()


def _touch_db(store: CanonicalStore, tenant_id: str) -> None:
    """Ensure the SQLite file exists so ``_get_connection`` will open it."""
    db_path = store._get_db_path(tenant_id)
    db_path.parent.mkdir(parents=True, exist_ok=True)
    sqlite3.connect(str(db_path)).close()


class TestConnectionPoolConcurrency:
    def test_race_single_connection_under_contention(self, store):
        """20 threads racing on one fresh tenant must produce exactly one
        cached connection (no FD leak, no orphans)."""
        tenant = "race-tenant"
        _touch_db(store, tenant)

        n_threads = 20
        barrier = threading.Barrier(n_threads)
        seen: list[sqlite3.Connection] = []
        seen_lock = threading.Lock()
        errors: list[BaseException] = []

        real_connect = sqlite3.connect
        connect_calls = 0
        connect_calls_lock = threading.Lock()

        def counting_connect(*args, **kwargs):
            nonlocal connect_calls
            with connect_calls_lock:
                connect_calls += 1
            return real_connect(*args, **kwargs)

        def worker():
            try:
                barrier.wait()
                with store._get_connection(tenant) as conn, seen_lock:
                    seen.append(conn)
            except BaseException as exc:  # noqa: BLE001
                errors.append(exc)

        with patch("sqlite3.connect", side_effect=counting_connect):
            threads = [threading.Thread(target=worker) for _ in range(n_threads)]
            for t in threads:
                t.start()
            for t in threads:
                t.join()

        assert not errors, f"worker errors: {errors}"
        assert len(seen) == n_threads
        # Exactly one cached connection under cache_key
        assert len(store._connections) == 1
        cached = next(iter(store._connections.values()))
        # Every thread observed the same connection instance
        assert all(c is cached for c in seen)
        # Underlying sqlite3.connect called exactly once despite 20 callers
        assert connect_calls == 1, f"expected 1 sqlite3.connect call, got {connect_calls} (FD leak)"

    def test_different_tenants_each_get_own_connection(self, store):
        """20 threads, 20 distinct tenants — each ends up with its own
        connection and no contention loses any."""
        n = 20
        tenants = [f"tenant-{i}" for i in range(n)]
        for t in tenants:
            _touch_db(store, t)

        barrier = threading.Barrier(n)
        results: dict[str, sqlite3.Connection] = {}
        results_lock = threading.Lock()
        errors: list[BaseException] = []

        def worker(tid: str):
            try:
                barrier.wait()
                with store._get_connection(tid) as conn, results_lock:
                    results[tid] = conn
            except BaseException as exc:  # noqa: BLE001
                errors.append(exc)

        threads = [threading.Thread(target=worker, args=(t,)) for t in tenants]
        for t in threads:
            t.start()
        for t in threads:
            t.join()

        assert not errors, f"worker errors: {errors}"
        assert len(results) == n
        assert len(store._connections) == n
        # All distinct connection objects
        ids = {id(c) for c in results.values()}
        assert len(ids) == n

    def test_repeated_requests_return_same_cached_connection(self, store):
        """After initial open, subsequent calls return the cached
        connection without re-opening."""
        tenant = "repeat-tenant"
        _touch_db(store, tenant)

        with store._get_connection(tenant) as conn1:
            pass
        with store._get_connection(tenant) as conn2:
            pass
        with store._get_connection(tenant) as conn3:
            pass

        assert conn1 is conn2 is conn3
        assert len(store._connections) == 1

    def test_close_all_invalidates_cache_then_reopens_fresh(self, store):
        """After ``close_all()``, a new ``_get_connection`` must open a
        brand-new connection rather than handing out a closed one."""
        tenant = "reopen-tenant"
        _touch_db(store, tenant)

        with store._get_connection(tenant) as conn1:
            pass
        original_id = id(conn1)

        store.close_all()
        assert store._connections == {}

        with store._get_connection(tenant) as conn2:
            # Must be a fresh, working connection
            cur = conn2.execute("SELECT 1")
            assert cur.fetchone()[0] == 1

        assert len(store._connections) == 1
        assert id(conn2) != original_id
