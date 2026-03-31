"""
Benchmark: StreamRecord.value_json() caching.

Measures the cost saved by caching the parsed JSON dict on StreamRecord
so that repeated calls to value_json() on the same record avoid
redundant json.loads()+bytes.decode() work.

In the applier hot path, value_json() is called:
  1. In _start_batched to read tenant_id for grouping  (line 299)
  2. In _process_record for actual event parsing        (line 709)
  3. Potentially again in the error-handling fallback    (line 737)

With caching, call #2 and #3 are free dict lookups (~50ns vs ~5us).

Run:
  pytest tests/benchmarks/bench_value_json_cache.py -v
"""

from __future__ import annotations

import json
import time

import pytest

from dbaas.entdb_server.wal.base import StreamPos, StreamRecord
from dbaas.entdb_server.wal.base import _SENTINEL  # noqa: F401  -- cache reset marker


def _make_record(payload_size: int = 10) -> StreamRecord:
    """Build a realistic WAL record with a TransactionEvent payload."""
    event = {
        "tenant_id": "tenant_perf_123",
        "actor": "user:42",
        "idempotency_key": "idem-0001",
        "schema_fingerprint": "sha256:abc123",
        "ts_ms": int(time.time() * 1000),
        "ops": [
            {
                "op": "create_node",
                "type_id": 101,
                "id": f"node-{i:04d}",
                "data": {
                    "title": f"Task #{i}",
                    "description": "A moderately sized payload field " * 3,
                    "priority": i % 5,
                    "tags": ["urgent", "backend", "perf"],
                },
                "acl": [
                    {"principal": "user:42", "role": "owner"},
                    {"principal": "user:99", "role": "viewer"},
                ],
            }
            for i in range(payload_size)
        ],
    }
    value_bytes = json.dumps(event).encode("utf-8")
    pos = StreamPos(topic="entdb-wal", partition=0, offset=0, timestamp_ms=0)
    return StreamRecord(key="tenant_perf_123", value=value_bytes, position=pos)


def _reset_cache(record: StreamRecord) -> None:
    """Reset the value_json cache to simulate a fresh record."""
    record._cached_json = _SENTINEL


class TestValueJsonCacheBenchmark:
    """Quantify the win from caching value_json() on StreamRecord."""

    def test_bench_single_call(self, benchmark):
        """Single value_json() call -- baseline cost of one parse."""
        record = _make_record(payload_size=5)

        def run():
            # Reset cache to measure cold parse
            _reset_cache(record)
            return record.value_json()

        result = benchmark(run)
        assert result["tenant_id"] == "tenant_perf_123"

    def test_bench_repeated_calls_cached(self, benchmark):
        """3 repeated value_json() calls on the same record (simulates applier).

        With caching, calls 2 and 3 are essentially free.
        """
        record = _make_record(payload_size=5)

        def run():
            # Reset cache to simulate a fresh record arriving
            _reset_cache(record)
            d1 = record.value_json()  # cold parse
            d2 = record.value_json()  # cached
            d3 = record.value_json()  # cached
            return d1, d2, d3

        d1, d2, d3 = benchmark(run)
        # All three should return the same dict object (identity, not just equality)
        assert d1 is d2
        assert d2 is d3

    def test_bench_1000_records_double_access(self, benchmark):
        """1000 records, each accessed twice (batch grouping + processing).

        This is the realistic applier scenario: poll_batch returns N records,
        the applier reads value_json() to extract tenant_id for grouping,
        then reads it again inside _apply_tenant_batch / _process_record.
        """
        records = [_make_record(payload_size=3) for _ in range(1000)]

        def run():
            for rec in records:
                _reset_cache(rec)
            total = 0
            for rec in records:
                d = rec.value_json()  # first call: parse
                total += d["ts_ms"]
                d2 = rec.value_json()  # second call: cached
                total += d2["ts_ms"]
            return total

        benchmark(run)

    def test_cache_identity(self):
        """Verify cached result is the same object (not a copy)."""
        record = _make_record(payload_size=2)
        first = record.value_json()
        second = record.value_json()
        assert first is second

    def test_cache_correctness(self):
        """Verify cached JSON matches a fresh parse."""
        record = _make_record(payload_size=2)
        cached = record.value_json()
        fresh = json.loads(record.value.decode("utf-8"))
        assert cached == fresh

    def test_manual_before_after_comparison(self):
        """Print explicit before/after timing for the README.

        Not a pytest-benchmark test; uses raw timeit for clarity.
        """
        record = _make_record(payload_size=5)
        value_bytes = record.value
        iterations = 50_000

        # "Before" -- simulate uncached: decode + json.loads every call
        t0 = time.perf_counter()
        for _ in range(iterations):
            json.loads(value_bytes.decode("utf-8"))
        uncached_s = time.perf_counter() - t0

        # "After" -- cached: first call parses, rest are dict lookups
        t0 = time.perf_counter()
        for _ in range(iterations):
            _reset_cache(record)
            record.value_json()  # parse (cold)
            record.value_json()  # cached
            record.value_json()  # cached
        cached_s = time.perf_counter() - t0
        # cached_s covers 3x iterations but only 1x parse + 2x cache hit

        uncached_per_call_us = (uncached_s / iterations) * 1e6
        # For fair comparison: "3 calls uncached" vs "3 calls cached"
        uncached_3x_s = uncached_s * 3
        speedup = uncached_3x_s / cached_s if cached_s > 0 else float("inf")

        print(f"\n--- value_json() cache benchmark ({iterations} iterations) ---")
        print(f"  Payload size: ~{len(value_bytes)} bytes")
        print(f"  Uncached (1 call):  {uncached_per_call_us:.2f} us/call")
        print(f"  3x uncached total:  {uncached_3x_s * 1e3:.1f} ms")
        print(f"  3x cached total:    {cached_s * 1e3:.1f} ms  (1 parse + 2 cache hits)")
        print(f"  Speedup (3 calls):  {speedup:.1f}x")
        print(f"  Estimated saving:   {(uncached_3x_s - cached_s) * 1e3:.1f} ms over {iterations} records")

        # Sanity: caching 3 calls should be faster than 3 uncached calls
        assert cached_s < uncached_3x_s, "Caching should be faster than 3x uncached"
