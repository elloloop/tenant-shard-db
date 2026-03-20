"""
Archiver performance benchmarks.

Run: pytest tests/benchmarks/bench_archiver.py -v --benchmark-only
"""
from __future__ import annotations

import json
import time
from unittest.mock import MagicMock

from dbaas.entdb_server.archive.archiver import Archiver

# --- Helpers ---

def make_event(tenant_id: str = "bench-tenant", i: int = 0, payload_size: int = 200) -> dict:
    """Create a realistic WAL event."""
    return {
        "tenant_id": tenant_id,
        "idempotency_key": f"idem-{i}",
        "schema_fingerprint": "sha256:abc123",
        "actor": "bench-user",
        "ts_ms": int(time.time() * 1000),
        "ops": [
            {
                "op": "create_node",
                "type_id": 1,
                "id": f"node-{i}",
                "data": {"name": f"Benchmark Item {i}", "payload": "x" * payload_size},
            }
        ],
    }


def make_record(event: dict, partition: int = 0, offset: int = 0):
    """Create a mock StreamRecord."""
    value = json.dumps(event).encode("utf-8")
    mock = MagicMock()
    mock.value = value
    mock.value_json.return_value = event
    mock.position = MagicMock()
    mock.position.partition = partition
    mock.position.offset = offset
    mock.position.to_dict.return_value = {"topic": "bench", "partition": partition, "offset": offset}
    return mock


# --- Serialization Benchmarks ---

class TestSerializationBenchmarks:
    """Benchmark segment serialization and compression."""

    def _build_archiver(self):
        mock_wal = MagicMock()
        mock_s3 = MagicMock()
        mock_s3.bucket = "bench-bucket"
        mock_s3.archive_prefix = "archive"
        mock_s3.region = "us-east-1"
        mock_s3.endpoint_url = None
        mock_s3.access_key_id = None
        mock_s3.secret_access_key = None
        return Archiver(wal=mock_wal, s3_config=mock_s3, topic="bench")

    def test_bench_serialize_100_events_gzip(self, benchmark):
        """Serialize and gzip-compress 100 events."""
        archiver = self._build_archiver()
        events = [
            {
                "event": make_event(i=i),
                "position": {"topic": "bench", "partition": 0, "offset": i},
                "checksum": f"sha256:{'a' * 64}",
                "archived_at": int(time.time() * 1000),
            }
            for i in range(100)
        ]
        result = benchmark(archiver._serialize_segment, events)
        assert len(result) > 0

    def test_bench_serialize_1000_events_gzip(self, benchmark):
        """Serialize and gzip-compress 1000 events."""
        archiver = self._build_archiver()
        events = [
            {
                "event": make_event(i=i),
                "position": {"topic": "bench", "partition": 0, "offset": i},
                "checksum": f"sha256:{'a' * 64}",
                "archived_at": int(time.time() * 1000),
            }
            for i in range(1000)
        ]
        result = benchmark(archiver._serialize_segment, events)
        assert len(result) > 0

    def test_bench_serialize_1000_events_no_compression(self, benchmark):
        """Serialize 1000 events without compression."""
        archiver = self._build_archiver()
        archiver.compression = "none"
        events = [
            {
                "event": make_event(i=i),
                "position": {"topic": "bench", "partition": 0, "offset": i},
                "checksum": f"sha256:{'a' * 64}",
                "archived_at": int(time.time() * 1000),
            }
            for i in range(1000)
        ]
        result = benchmark(archiver._serialize_segment, events)
        assert len(result) > 0

    def test_bench_serialize_large_payload_events(self, benchmark):
        """Serialize 100 events with 10KB payloads."""
        archiver = self._build_archiver()
        events = [
            {
                "event": make_event(i=i, payload_size=10000),
                "position": {"topic": "bench", "partition": 0, "offset": i},
                "checksum": f"sha256:{'a' * 64}",
                "archived_at": int(time.time() * 1000),
            }
            for i in range(100)
        ]
        result = benchmark(archiver._serialize_segment, events)
        assert len(result) > 0


# --- Checksum Benchmarks ---

class TestChecksumBenchmarks:
    """Benchmark checksum computation."""

    def _build_archiver(self):
        mock_wal = MagicMock()
        mock_s3 = MagicMock()
        mock_s3.bucket = "bench-bucket"
        mock_s3.archive_prefix = "archive"
        mock_s3.region = "us-east-1"
        mock_s3.endpoint_url = None
        mock_s3.access_key_id = None
        mock_s3.secret_access_key = None
        return Archiver(wal=mock_wal, s3_config=mock_s3, topic="bench")

    def test_bench_checksum_small_event(self, benchmark):
        """SHA-256 checksum for small event (~500 bytes)."""
        archiver = self._build_archiver()
        data = json.dumps(make_event(payload_size=200)).encode()
        result = benchmark(archiver._compute_checksum, data)
        assert result.startswith("sha256:")

    def test_bench_checksum_large_event(self, benchmark):
        """SHA-256 checksum for large event (~100KB)."""
        archiver = self._build_archiver()
        data = json.dumps(make_event(payload_size=100000)).encode()
        result = benchmark(archiver._compute_checksum, data)
        assert result.startswith("sha256:")


# --- S3 Key Generation Benchmarks ---

class TestS3KeyBenchmarks:
    """Benchmark S3 key generation."""

    def _build_archiver(self):
        mock_wal = MagicMock()
        mock_s3 = MagicMock()
        mock_s3.bucket = "bench-bucket"
        mock_s3.archive_prefix = "archive"
        mock_s3.region = "us-east-1"
        mock_s3.endpoint_url = None
        mock_s3.access_key_id = None
        mock_s3.secret_access_key = None
        return Archiver(wal=mock_wal, s3_config=mock_s3, topic="bench")

    def test_bench_s3_key_generation(self, benchmark):
        """Generate S3 key for archive segment."""
        archiver = self._build_archiver()
        result = benchmark(archiver._build_s3_key, "tenant-1", 0, 0, 9999)
        assert "tenant=tenant-1" in result


# --- Compression Ratio Analysis ---

class TestCompressionRatio:
    """Measure compression ratios for different data patterns."""

    def _build_archiver(self):
        mock_wal = MagicMock()
        mock_s3 = MagicMock()
        mock_s3.bucket = "bench-bucket"
        mock_s3.archive_prefix = "archive"
        mock_s3.region = "us-east-1"
        mock_s3.endpoint_url = None
        mock_s3.access_key_id = None
        mock_s3.secret_access_key = None
        return Archiver(wal=mock_wal, s3_config=mock_s3, topic="bench")

    def test_compression_ratio_typical_events(self):
        """Measure compression ratio for typical JSON events."""
        archiver = self._build_archiver()
        events = [
            {
                "event": make_event(i=i),
                "position": {"topic": "bench", "partition": 0, "offset": i},
                "checksum": f"sha256:{'a' * 64}",
                "archived_at": int(time.time() * 1000),
            }
            for i in range(1000)
        ]

        archiver.compression = "none"
        raw = archiver._serialize_segment(events)

        archiver.compression = "gzip"
        compressed = archiver._serialize_segment(events)

        ratio = len(compressed) / len(raw)
        print(f"\nCompression ratio: {ratio:.2%} ({len(raw)} → {len(compressed)} bytes)")
        print(f"Space saved: {(1 - ratio):.1%}")
        assert ratio < 1.0  # Compression should reduce size


# --- Cost Model ---

class TestCostModel:
    """Calculate and compare costs for different archiver configurations."""

    # R2 pricing
    R2_PUT_COST = 4.50 / 1_000_000   # per PUT
    R2_GET_COST = 0.36 / 1_000_000   # per GET
    R2_STORAGE_COST = 0.015           # per GB/month
    R2_EGRESS_COST = 0.0             # FREE

    # S3 pricing
    S3_PUT_COST = 5.00 / 1_000_000
    S3_GET_COST = 0.40 / 1_000_000
    S3_STORAGE_COST = 0.023
    S3_EGRESS_COST = 0.09            # per GB

    def test_cost_batched_60s_flush(self):
        """Cost model: batched mode, 60s flush interval."""
        flushes_per_day = 86400 / 60  # 1440
        flushes_per_month = flushes_per_day * 30

        r2_monthly = flushes_per_month * self.R2_PUT_COST
        s3_monthly = flushes_per_month * self.S3_PUT_COST

        print("\n--- Batched 60s flush (1 partition) ---")
        print(f"PUTs/month: {flushes_per_month:,.0f}")
        print(f"R2 cost/month: ${r2_monthly:.4f}")
        print(f"S3 cost/month: ${s3_monthly:.4f}")

        assert r2_monthly < 1.0  # Should be under $1/month

    def test_cost_batched_4h_flush(self):
        """Cost model: batched mode, 4-hour flush interval."""
        flushes_per_day = 86400 / 14400  # 6
        flushes_per_month = flushes_per_day * 30

        r2_monthly = flushes_per_month * self.R2_PUT_COST
        s3_monthly = flushes_per_month * self.S3_PUT_COST

        print("\n--- Batched 4h flush (1 partition) ---")
        print(f"PUTs/month: {flushes_per_month:,.0f}")
        print(f"R2 cost/month: ${r2_monthly:.6f}")
        print(f"S3 cost/month: ${s3_monthly:.6f}")

        assert r2_monthly < 0.01

    def test_cost_individual_flush(self):
        """Cost model: individual flush (1 PUT per event)."""
        events_per_day = 10000
        events_per_month = events_per_day * 30

        r2_monthly = events_per_month * self.R2_PUT_COST
        s3_monthly = events_per_month * self.S3_PUT_COST

        print("\n--- Individual flush (10k events/day) ---")
        print(f"PUTs/month: {events_per_month:,.0f}")
        print(f"R2 cost/month: ${r2_monthly:.4f}")
        print(f"S3 cost/month: ${s3_monthly:.4f}")

    def test_cost_storage_growth(self):
        """Cost model: archive storage growth over 12 months."""
        events_per_month = 10000 * 30  # 300k events
        avg_event_bytes = 500
        compression_ratio = 0.15  # gzip typically 85% reduction for JSON

        monthly_raw = events_per_month * avg_event_bytes
        monthly_compressed = monthly_raw * compression_ratio
        monthly_gb = monthly_compressed / (1024 ** 3)

        print("\n--- Storage growth (300k events/month) ---")
        print(f"Raw: {monthly_raw / 1024 / 1024:.1f} MB/month")
        print(f"Compressed: {monthly_compressed / 1024 / 1024:.1f} MB/month")

        total_r2 = 0
        total_s3 = 0
        for month in range(1, 13):
            cumulative_gb = monthly_gb * month
            total_r2 += cumulative_gb * self.R2_STORAGE_COST
            total_s3 += cumulative_gb * self.S3_STORAGE_COST

        print(f"R2 storage (12 months cumulative): ${total_r2:.2f}")
        print(f"S3 storage (12 months cumulative): ${total_s3:.2f}")

    def test_cost_recovery_egress(self):
        """Cost model: data transfer during recovery."""
        snapshot_size_gb = 0.5  # 500MB snapshot
        archive_size_gb = 2.0  # 2GB of archive segments

        r2_recovery = 0  # Free egress
        s3_recovery = (snapshot_size_gb + archive_size_gb) * self.S3_EGRESS_COST

        print("\n--- Recovery egress cost ---")
        print(f"Snapshot: {snapshot_size_gb:.1f} GB, Archive: {archive_size_gb:.1f} GB")
        print(f"R2: ${r2_recovery:.2f} (free)")
        print(f"S3: ${s3_recovery:.2f}")

        assert r2_recovery == 0

    def test_cost_comparison_summary(self):
        """Full cost comparison: batched vs individual, R2 vs S3."""
        events_per_day = 10000
        avg_event_bytes = 500
        compression_ratio = 0.15

        configs = {
            "batched_60s": {"flushes_day": 1440, "desc": "Batched (60s)"},
            "batched_4h": {"flushes_day": 6, "desc": "Batched (4h)"},
            "individual": {"flushes_day": events_per_day, "desc": "Individual"},
            "disabled": {"flushes_day": 0, "desc": "Disabled"},
        }

        monthly_storage_gb = (events_per_day * 30 * avg_event_bytes * compression_ratio) / (1024 ** 3)

        print(f"\n{'='*70}")
        print(f"COST COMPARISON: {events_per_day:,} events/day, {avg_event_bytes}B avg")
        print(f"{'='*70}")
        print(f"{'Mode':<20} {'PUTs/mo':>10} {'R2 PUT':>10} {'S3 PUT':>10} {'R2 Store':>10} {'S3 Store':>10}")
        print(f"{'-'*70}")

        for key, cfg in configs.items():
            puts_month = cfg["flushes_day"] * 30
            r2_put = puts_month * self.R2_PUT_COST
            s3_put = puts_month * self.S3_PUT_COST
            r2_store = monthly_storage_gb * self.R2_STORAGE_COST if key != "disabled" else 0
            s3_store = monthly_storage_gb * self.S3_STORAGE_COST if key != "disabled" else 0

            print(f"{cfg['desc']:<20} {puts_month:>10,} ${r2_put:>9.4f} ${s3_put:>9.4f} ${r2_store:>9.4f} ${s3_store:>9.4f}")

        print(f"{'='*70}")
        print("Note: R2 egress is FREE. S3 egress is $0.09/GB.")
