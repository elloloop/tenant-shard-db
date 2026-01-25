"""
Write-Ahead Log (WAL) Stream abstraction for EntDB.

This module provides a pluggable WAL backend interface supporting:
- Kafka/Redpanda (recommended for production)
- AWS Kinesis
- In-memory (for testing)

The WAL stream is the source of truth for all writes. SQLite databases
are derived views that can be rebuilt from the stream.

Invariants:
    - append() returns only after durable storage is confirmed
    - Events are totally ordered per tenant (partition by tenant_id)
    - Consumers receive events in order within a partition
    - Failed appends must not result in partial writes

How to change safely:
    - New backends must implement WalStream protocol
    - Test with chaos engineering (network partitions, crashes)
    - Verify durability guarantees match production requirements
"""

from .base import (
    StreamPos,
    StreamRecord,
    WalConnectionError,
    WalError,
    WalStream,
    WalTimeoutError,
    create_wal_stream,
)
from .kafka import KafkaWalStream
from .kinesis import KinesisWalStream
from .memory import InMemoryWalStream

__all__ = [
    # Protocol and types
    "WalStream",
    "StreamRecord",
    "StreamPos",
    "WalError",
    "WalConnectionError",
    "WalTimeoutError",
    # Factory
    "create_wal_stream",
    # Implementations
    "KafkaWalStream",
    "KinesisWalStream",
    "InMemoryWalStream",
]
