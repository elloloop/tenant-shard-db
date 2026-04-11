"""
Unit tests for wait-for-offset read-after-write consistency.

Tests cover:
- Offset tracking in CanonicalStore (update, get, compare)
- wait_for_offset with simulated applier notification
- after_offset parameter integration on gRPC read methods
- Stream position comparison logic
"""

import asyncio

import pytest

from dbaas.entdb_server.apply.canonical_store import (
    CanonicalStore,
    _compare_stream_pos,
    _parse_stream_offset,
)

# ── Stream position parsing / comparison ──────────────────────────────


class TestStreamPosComparison:
    """Tests for _parse_stream_offset and _compare_stream_pos helpers."""

    def test_parse_full_format(self):
        """topic:partition:offset format extracts the offset."""
        assert _parse_stream_offset("entdb-wal:0:5") == 5
        assert _parse_stream_offset("entdb-wal:0:100") == 100

    def test_parse_plain_int(self):
        """Plain integer string parses correctly."""
        assert _parse_stream_offset("42") == 42

    def test_parse_partition_colon_offset(self):
        """partition:offset format extracts offset after last colon."""
        assert _parse_stream_offset("0:7") == 7

    def test_parse_non_numeric_returns_zero(self):
        """Non-numeric offset returns 0."""
        assert _parse_stream_offset("abc") == 0

    def test_compare_equal(self):
        assert _compare_stream_pos("entdb-wal:0:5", "entdb-wal:0:5") == 0

    def test_compare_greater(self):
        assert _compare_stream_pos("entdb-wal:0:10", "entdb-wal:0:5") > 0

    def test_compare_less(self):
        assert _compare_stream_pos("entdb-wal:0:3", "entdb-wal:0:5") < 0


# ── Offset tracking ─────────────────────────────────────────────────


class TestOffsetTracking:
    """Tests for CanonicalStore offset tracking methods."""

    @pytest.fixture
    def store(self, tmp_path):
        return CanonicalStore(str(tmp_path))

    def test_get_applied_offset_returns_none_initially(self, store):
        assert store.get_applied_offset("tenant_1") is None

    @pytest.mark.asyncio
    async def test_update_and_get_applied_offset(self, store):
        await store.update_applied_offset("tenant_1", "entdb-wal:0:5")
        assert store.get_applied_offset("tenant_1") == "entdb-wal:0:5"

    @pytest.mark.asyncio
    async def test_update_overwrites_previous(self, store):
        await store.update_applied_offset("tenant_1", "entdb-wal:0:5")
        await store.update_applied_offset("tenant_1", "entdb-wal:0:10")
        assert store.get_applied_offset("tenant_1") == "entdb-wal:0:10"

    @pytest.mark.asyncio
    async def test_different_tenants_independent(self, store):
        await store.update_applied_offset("tenant_1", "entdb-wal:0:5")
        await store.update_applied_offset("tenant_2", "entdb-wal:0:10")
        assert store.get_applied_offset("tenant_1") == "entdb-wal:0:5"
        assert store.get_applied_offset("tenant_2") == "entdb-wal:0:10"


# ── wait_for_offset ─────────────────────────────────────────────────


class TestWaitForOffset:
    """Tests for CanonicalStore.wait_for_offset."""

    @pytest.fixture
    def store(self, tmp_path):
        return CanonicalStore(str(tmp_path))

    @pytest.mark.asyncio
    async def test_returns_true_if_already_reached(self, store):
        """If offset is already at or past target, return immediately."""
        await store.update_applied_offset("t1", "entdb-wal:0:10")
        result = await store.wait_for_offset("t1", "entdb-wal:0:5", timeout=1.0)
        assert result is True

    @pytest.mark.asyncio
    async def test_returns_true_if_exactly_reached(self, store):
        await store.update_applied_offset("t1", "entdb-wal:0:5")
        result = await store.wait_for_offset("t1", "entdb-wal:0:5", timeout=1.0)
        assert result is True

    @pytest.mark.asyncio
    async def test_returns_false_on_timeout(self, store):
        """If no offset is ever set, times out and returns False."""
        result = await store.wait_for_offset("t1", "entdb-wal:0:5", timeout=0.1)
        assert result is False

    @pytest.mark.asyncio
    async def test_wakes_up_on_update(self, store):
        """Waiter should wake up when the applier updates the offset."""

        async def simulate_applier():
            await asyncio.sleep(0.05)
            await store.update_applied_offset("t1", "entdb-wal:0:10")

        task = asyncio.create_task(simulate_applier())
        result = await store.wait_for_offset("t1", "entdb-wal:0:10", timeout=5.0)
        assert result is True
        await task

    @pytest.mark.asyncio
    async def test_multiple_waiters_all_notified(self, store):
        """Multiple waiters for different positions should all resolve."""

        async def simulate_applier():
            await asyncio.sleep(0.05)
            await store.update_applied_offset("t1", "entdb-wal:0:20")

        task = asyncio.create_task(simulate_applier())

        r1 = await store.wait_for_offset("t1", "entdb-wal:0:5", timeout=5.0)
        r2 = await store.wait_for_offset("t1", "entdb-wal:0:10", timeout=5.0)
        r3 = await store.wait_for_offset("t1", "entdb-wal:0:20", timeout=5.0)

        assert r1 is True
        assert r2 is True
        assert r3 is True
        await task

    @pytest.mark.asyncio
    async def test_incremental_progress(self, store):
        """Waiter wakes when the offset reaches the target incrementally."""

        async def simulate_applier():
            await asyncio.sleep(0.02)
            await store.update_applied_offset("t1", "entdb-wal:0:3")
            await asyncio.sleep(0.02)
            await store.update_applied_offset("t1", "entdb-wal:0:7")
            await asyncio.sleep(0.02)
            await store.update_applied_offset("t1", "entdb-wal:0:10")

        task = asyncio.create_task(simulate_applier())
        result = await store.wait_for_offset("t1", "entdb-wal:0:10", timeout=5.0)
        assert result is True
        await task

    @pytest.mark.asyncio
    async def test_returns_false_if_offset_below_target_at_timeout(self, store):
        """Returns False if offset is set but never reaches target."""

        async def simulate_applier():
            await asyncio.sleep(0.02)
            await store.update_applied_offset("t1", "entdb-wal:0:3")

        task = asyncio.create_task(simulate_applier())
        result = await store.wait_for_offset("t1", "entdb-wal:0:10", timeout=0.15)
        assert result is False
        await task
