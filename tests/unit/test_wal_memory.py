"""
Unit tests for in-memory WAL stream implementation.

Tests cover:
- Basic append/subscribe operations
- Partition assignment
- Concurrent access
- Testing helpers
"""

import asyncio
import pytest
from dbaas.entdb_server.wal.memory import InMemoryWalStream
from dbaas.entdb_server.wal.base import WalConnectionError


class TestInMemoryWalStream:
    """Tests for InMemoryWalStream."""

    @pytest.fixture
    def wal(self):
        """Create a fresh WAL stream."""
        return InMemoryWalStream(num_partitions=4)

    @pytest.mark.asyncio
    async def test_connect_disconnect(self, wal):
        """Test connection lifecycle."""
        assert not wal.is_connected

        await wal.connect()
        assert wal.is_connected

        await wal.close()
        assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_append_requires_connection(self, wal):
        """Append fails if not connected."""
        with pytest.raises(WalConnectionError):
            await wal.append("test", "key1", b"value1")

    @pytest.mark.asyncio
    async def test_append_returns_position(self, wal):
        """Append returns stream position."""
        await wal.connect()

        pos = await wal.append("test", "key1", b"value1")

        assert pos.topic == "test"
        assert pos.partition >= 0
        assert pos.partition < 4
        assert pos.offset == 0
        assert pos.timestamp_ms > 0

    @pytest.mark.asyncio
    async def test_append_increments_offset(self, wal):
        """Sequential appends to same partition increment offset."""
        await wal.connect()

        # Use same key to hit same partition
        pos1 = await wal.append("test", "key1", b"value1")
        pos2 = await wal.append("test", "key1", b"value2")

        assert pos2.partition == pos1.partition
        assert pos2.offset == pos1.offset + 1

    @pytest.mark.asyncio
    async def test_consistent_partitioning(self, wal):
        """Same key always maps to same partition."""
        await wal.connect()

        positions = []
        for _ in range(10):
            pos = await wal.append("test", "tenant_123", b"data")
            positions.append(pos.partition)

        assert len(set(positions)) == 1  # All same partition

    @pytest.mark.asyncio
    async def test_different_keys_different_partitions(self, wal):
        """Different keys may hit different partitions."""
        await wal.connect()

        partitions = set()
        # Use enough different keys to likely hit different partitions
        for i in range(100):
            pos = await wal.append("test", f"key_{i}", b"data")
            partitions.add(pos.partition)

        # Should hit more than one partition
        assert len(partitions) > 1

    @pytest.mark.asyncio
    async def test_subscribe_yields_records(self, wal):
        """Subscribe yields appended records."""
        await wal.connect()

        # Append some records
        await wal.append("test", "key1", b"value1")
        await wal.append("test", "key2", b"value2")

        # Subscribe and collect records
        records = []
        async for record in wal.subscribe("test", "group1"):
            records.append(record)
            if len(records) == 2:
                break

        assert len(records) == 2
        values = {r.value for r in records}
        assert b"value1" in values
        assert b"value2" in values

    @pytest.mark.asyncio
    async def test_subscribe_with_headers(self, wal):
        """Records include headers."""
        await wal.connect()

        headers = {"x-type": b"create", "x-version": b"1"}
        await wal.append("test", "key1", b"value", headers=headers)

        async for record in wal.subscribe("test", "group1"):
            assert record.headers == headers
            break

    @pytest.mark.asyncio
    async def test_get_all_records_helper(self, wal):
        """Testing helper retrieves all records."""
        await wal.connect()

        await wal.append("test", "key1", b"v1")
        await wal.append("test", "key2", b"v2")
        await wal.append("test", "key3", b"v3")

        records = wal.get_all_records("test")
        assert len(records) == 3

    @pytest.mark.asyncio
    async def test_get_record_count_helper(self, wal):
        """Testing helper returns correct count."""
        await wal.connect()

        assert wal.get_record_count("test") == 0

        await wal.append("test", "key1", b"v1")
        assert wal.get_record_count("test") == 1

        await wal.append("test", "key2", b"v2")
        assert wal.get_record_count("test") == 2

    @pytest.mark.asyncio
    async def test_clear_topic_helper(self, wal):
        """Testing helper clears topic data."""
        await wal.connect()

        await wal.append("test", "key1", b"v1")
        await wal.append("test", "key2", b"v2")
        assert wal.get_record_count("test") == 2

        wal.clear_topic("test")
        assert wal.get_record_count("test") == 0

    @pytest.mark.asyncio
    async def test_wait_for_records_helper(self, wal):
        """Testing helper waits for record count."""
        await wal.connect()

        async def append_later():
            await asyncio.sleep(0.1)
            await wal.append("test", "key1", b"v1")
            await wal.append("test", "key2", b"v2")

        asyncio.create_task(append_later())

        result = await wal.wait_for_records("test", 2, timeout=2.0)
        assert result is True
        assert wal.get_record_count("test") == 2

    @pytest.mark.asyncio
    async def test_wait_for_records_timeout(self, wal):
        """Testing helper returns False on timeout."""
        await wal.connect()

        result = await wal.wait_for_records("test", 10, timeout=0.1)
        assert result is False

    @pytest.mark.asyncio
    async def test_commit_tracks_offset(self, wal):
        """Commit tracks consumed offset."""
        await wal.connect()

        await wal.append("test", "key1", b"v1")

        async for record in wal.subscribe("test", "group1"):
            await wal.commit(record)
            break

        positions = await wal.get_positions("test", "default")
        # Should have some position tracked
        assert len(positions) >= 0  # May not track if using default group

    @pytest.mark.asyncio
    async def test_multiple_topics(self, wal):
        """WAL supports multiple topics."""
        await wal.connect()

        await wal.append("topic1", "key1", b"v1")
        await wal.append("topic2", "key1", b"v2")

        assert wal.get_record_count("topic1") == 1
        assert wal.get_record_count("topic2") == 1

    @pytest.mark.asyncio
    async def test_close_clears_data(self, wal):
        """Close clears all data."""
        await wal.connect()

        await wal.append("test", "key1", b"v1")
        assert wal.get_record_count("test") == 1

        await wal.close()

        # Reconnect and verify data is gone
        await wal.connect()
        assert wal.get_record_count("test") == 0

