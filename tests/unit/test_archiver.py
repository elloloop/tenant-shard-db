"""
Unit tests for archiver logic.

Tests cover serialization, S3 key generation, checksum computation,
flush mode behavior, and segment management.
"""
from __future__ import annotations

import gzip
import json
from unittest.mock import AsyncMock, MagicMock

import pytest

from dbaas.entdb_server.archive.archiver import Archiver


def _make_archiver(**kwargs):
    """Create an Archiver with mocked dependencies."""
    mock_wal = MagicMock()
    mock_s3 = MagicMock()
    mock_s3.bucket = "test-bucket"
    mock_s3.archive_prefix = "archive"
    mock_s3.region = "us-east-1"
    mock_s3.endpoint_url = None
    mock_s3.access_key_id = None
    mock_s3.secret_access_key = None
    defaults = {"wal": mock_wal, "s3_config": mock_s3, "topic": "test-wal"}
    defaults.update(kwargs)
    return Archiver(**defaults)


def _make_record(tenant_id: str = "t1", partition: int = 0, offset: int = 0, payload_size: int = 100):
    """Create a mock StreamRecord."""
    event = {
        "tenant_id": tenant_id,
        "idempotency_key": f"key-{offset}",
        "ops": [{"op": "create_node", "type_id": 1, "id": f"n-{offset}", "data": {"v": "x" * payload_size}}],
    }
    value = json.dumps(event).encode()
    mock = MagicMock()
    mock.value = value
    mock.value_json.return_value = event
    mock.position = MagicMock()
    mock.position.partition = partition
    mock.position.offset = offset
    mock.position.to_dict.return_value = {"topic": "test-wal", "partition": partition, "offset": offset}
    return mock


@pytest.mark.unit
class TestS3KeyGeneration:
    def test_key_format_gzip(self):
        archiver = _make_archiver()
        key = archiver._build_s3_key("tenant-1", 0, 100, 200)
        assert key == "archive/tenant=tenant-1/partition=0/from=00000000000000000100_to=00000000000000000200.jsonl.gz"

    def test_key_format_no_compression(self):
        archiver = _make_archiver(compression="none")
        key = archiver._build_s3_key("tenant-1", 0, 100, 200)
        assert key.endswith(".jsonl")
        assert not key.endswith(".jsonl.gz")

    def test_key_offset_zero_padding(self):
        archiver = _make_archiver()
        key = archiver._build_s3_key("t", 0, 0, 1)
        assert "from=00000000000000000000_to=00000000000000000001" in key

    def test_key_includes_partition(self):
        archiver = _make_archiver()
        key = archiver._build_s3_key("t", 5, 0, 1)
        assert "partition=5" in key


@pytest.mark.unit
class TestSerialization:
    def test_single_event_jsonl(self):
        archiver = _make_archiver(compression="none")
        events = [{"event": {"a": 1}, "position": {"offset": 0}}]
        result = archiver._serialize_segment(events)
        lines = result.decode().strip().split("\n")
        assert len(lines) == 1
        assert json.loads(lines[0])["event"]["a"] == 1

    def test_multiple_events_newline_separated(self):
        archiver = _make_archiver(compression="none")
        events = [{"event": {"i": i}} for i in range(5)]
        result = archiver._serialize_segment(events)
        lines = result.decode().strip().split("\n")
        assert len(lines) == 5

    def test_gzip_compression_reduces_size(self):
        archiver_raw = _make_archiver(compression="none")
        archiver_gz = _make_archiver(compression="gzip")
        events = [{"event": {"data": "x" * 1000}} for _ in range(100)]
        raw = archiver_raw._serialize_segment(events)
        compressed = archiver_gz._serialize_segment(events)
        assert len(compressed) < len(raw)

    def test_gzip_decompressible(self):
        archiver = _make_archiver(compression="gzip")
        events = [{"event": {"hello": "world"}}]
        compressed = archiver._serialize_segment(events)
        decompressed = gzip.decompress(compressed)
        parsed = json.loads(decompressed.decode().strip())
        assert parsed["event"]["hello"] == "world"


@pytest.mark.unit
class TestChecksum:
    def test_sha256_format(self):
        archiver = _make_archiver()
        result = archiver._compute_checksum(b"test data")
        assert result.startswith("sha256:")
        assert len(result) == 7 + 64  # "sha256:" + 64 hex chars

    def test_deterministic(self):
        archiver = _make_archiver()
        assert archiver._compute_checksum(b"abc") == archiver._compute_checksum(b"abc")

    def test_different_data_different_checksum(self):
        archiver = _make_archiver()
        assert archiver._compute_checksum(b"a") != archiver._compute_checksum(b"b")


@pytest.mark.unit
class TestProcessRecord:
    @pytest.mark.asyncio
    async def test_creates_pending_segment(self):
        archiver = _make_archiver()
        record = _make_record(tenant_id="t1", partition=0, offset=0)
        await archiver._process_record(record)
        assert "t1:0" in archiver._pending_segments

    @pytest.mark.asyncio
    async def test_accumulates_events(self):
        archiver = _make_archiver()
        for i in range(5):
            await archiver._process_record(_make_record(offset=i))
        assert len(archiver._pending_segments["t1:0"].events) == 5

    @pytest.mark.asyncio
    async def test_tracks_size_estimate(self):
        archiver = _make_archiver()
        await archiver._process_record(_make_record(payload_size=500))
        segment = archiver._pending_segments["t1:0"]
        assert segment.size_estimate > 0

    @pytest.mark.asyncio
    async def test_individual_mode_flushes_each(self):
        archiver = _make_archiver(flush_mode="individual")
        archiver._s3_client = AsyncMock()
        archiver._s3_client.put_object = AsyncMock()
        await archiver._process_record(_make_record(offset=0))
        # Segment should have been flushed (removed from pending)
        assert "t1:0" not in archiver._pending_segments

    @pytest.mark.asyncio
    async def test_separate_segments_per_tenant(self):
        archiver = _make_archiver()
        await archiver._process_record(_make_record(tenant_id="a", offset=0))
        await archiver._process_record(_make_record(tenant_id="b", offset=0))
        assert "a:0" in archiver._pending_segments
        assert "b:0" in archiver._pending_segments


@pytest.mark.unit
class TestFlushSegment:
    @pytest.mark.asyncio
    async def test_min_segment_events_blocks_flush(self):
        archiver = _make_archiver(min_segment_events=10)
        archiver._s3_client = AsyncMock()
        # Add 5 events (below min)
        for i in range(5):
            await archiver._process_record(_make_record(offset=i))
        result = await archiver._flush_segment("t1:0")
        assert result is None  # Should not flush
        assert "t1:0" in archiver._pending_segments  # Still pending

    @pytest.mark.asyncio
    async def test_force_flush_ignores_min(self):
        archiver = _make_archiver(min_segment_events=10)
        archiver._s3_client = AsyncMock()
        archiver._s3_client.put_object = AsyncMock()
        for i in range(5):
            await archiver._process_record(_make_record(offset=i))
        result = await archiver._flush_segment("t1:0", force=True)
        assert result is not None

    @pytest.mark.asyncio
    async def test_empty_segment_returns_none(self):
        archiver = _make_archiver()
        result = await archiver._flush_segment("nonexistent")
        assert result is None


@pytest.mark.unit
class TestStats:
    @pytest.mark.asyncio
    async def test_stats_empty(self):
        archiver = _make_archiver()
        stats = archiver.stats
        assert stats["pending_segments"] == 0
        assert stats["pending_events"] == 0
        assert stats["archived_count"] == 0

    @pytest.mark.asyncio
    async def test_stats_with_pending(self):
        archiver = _make_archiver()
        for i in range(3):
            await archiver._process_record(_make_record(offset=i))
        stats = archiver.stats
        assert stats["pending_segments"] == 1
        assert stats["pending_events"] == 3
