"""
Base protocol and types for WAL stream abstraction.

This module defines the WalStream protocol that all backends must implement,
along with common types for stream positions, records, and errors.

Invariants:
    - StreamPos uniquely identifies a position in the stream
    - StreamRecord contains the event data plus metadata
    - All backends must provide the same ordering guarantees

How to change safely:
    - Protocol changes require updating all implementations
    - Add new methods as optional with default implementations
    - Version the protocol for backward compatibility
"""

from __future__ import annotations

import json
import logging
from abc import abstractmethod
from collections.abc import AsyncIterator
from dataclasses import dataclass, field
from typing import (
    TYPE_CHECKING,
    Any,
    Protocol,
    runtime_checkable,
)

if TYPE_CHECKING:
    from ..config import ServerConfig

logger = logging.getLogger(__name__)

_SENTINEL = object()  # Cache-miss marker for StreamRecord.value_json()


class WalError(Exception):
    """Base exception for WAL operations."""

    pass


class WalConnectionError(WalError):
    """Connection to WAL backend failed."""

    pass


class WalTimeoutError(WalError):
    """WAL operation timed out."""

    pass


class WalSerializationError(WalError):
    """Failed to serialize/deserialize WAL record."""

    pass


@dataclass(frozen=True)
class StreamPos:
    """Position in the WAL stream.

    This uniquely identifies a position in the stream for:
    - Resuming consumption after restart
    - Acknowledging processed events
    - Archiving checkpoints

    Attributes:
        topic: Topic/stream name
        partition: Partition number (0 for Kinesis shards, mapped internally)
        offset: Offset within partition (Kafka offset or Kinesis sequence number)
        timestamp_ms: Timestamp when the record was written (milliseconds)

    The position is backend-specific but provides a consistent interface.
    """

    topic: str
    partition: int
    offset: int
    timestamp_ms: int

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary for serialization."""
        return {
            "topic": self.topic,
            "partition": self.partition,
            "offset": self.offset,
            "timestamp_ms": self.timestamp_ms,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> StreamPos:
        """Create from dictionary."""
        return cls(
            topic=data["topic"],
            partition=data["partition"],
            offset=data["offset"],
            timestamp_ms=data["timestamp_ms"],
        )

    def __str__(self) -> str:
        return f"{self.topic}:{self.partition}:{self.offset}"


@dataclass
class StreamRecord:
    """A record from the WAL stream.

    Contains the event data plus metadata about its position in the stream.

    Attributes:
        key: Partition key (typically tenant_id)
        value: Event payload (bytes, typically JSON-encoded TransactionEvent)
        position: Position in the stream
        headers: Optional headers/metadata

    Example:
        >>> async for record in wal.subscribe("entdb-wal", "applier"):
        ...     event = json.loads(record.value)
        ...     process_event(event)
        ...     await wal.commit(record)
    """

    key: str
    value: bytes
    position: StreamPos
    headers: dict[str, bytes] = field(default_factory=dict)
    _cached_json: Any = field(default=_SENTINEL, init=False, repr=False, compare=False)

    def value_json(self) -> Any:
        """Parse value as JSON, caching the result.

        The parsed dict is cached so repeated calls (e.g. applier batch
        grouping then per-event processing) avoid redundant json.loads
        on the same bytes.

        Returns:
            Parsed JSON value

        Raises:
            WalSerializationError: If value is not valid JSON
        """
        cached = self._cached_json
        if cached is not _SENTINEL:
            return cached
        try:
            parsed = json.loads(self.value.decode("utf-8"))
        except (json.JSONDecodeError, UnicodeDecodeError) as e:
            raise WalSerializationError(f"Failed to parse record value as JSON: {e}")
        self._cached_json = parsed
        return parsed

    def __str__(self) -> str:
        return f"StreamRecord(key={self.key}, pos={self.position})"


@runtime_checkable
class WalStream(Protocol):
    """Protocol for WAL stream backends.

    This defines the interface that all WAL backends must implement.
    The protocol ensures:
    - Durable append with acknowledgment
    - Ordered consumption within partitions
    - Consumer group coordination

    Durability contract:
        - append() returns only after data is durably stored
        - For Kafka: acks=all, min.insync.replicas >= 2 (production)
        - For Kinesis: PutRecord returns after replication

    Ordering contract:
        - Events with same key (tenant_id) are totally ordered
        - Consumer receives events in order within a partition

    Example:
        >>> wal = KafkaWalStream(config)
        >>> await wal.connect()
        >>> pos = await wal.append("entdb-wal", "tenant_123", event_bytes)
        >>> print(f"Event written at {pos}")
    """

    @abstractmethod
    async def connect(self) -> None:
        """Connect to the WAL backend.

        Must be called before any other operations.

        Raises:
            WalConnectionError: If connection fails
        """
        ...

    @abstractmethod
    async def close(self) -> None:
        """Close connection to the WAL backend.

        Flushes any pending writes and releases resources.
        """
        ...

    @abstractmethod
    async def append(
        self,
        topic: str,
        key: str,
        value: bytes,
        headers: dict[str, bytes] | None = None,
    ) -> StreamPos:
        """Append an event to the stream.

        This is the core write operation. It returns only after the event
        is durably stored (not just buffered).

        Args:
            topic: Topic/stream name
            key: Partition key (typically tenant_id for ordering)
            value: Event payload (bytes)
            headers: Optional headers/metadata

        Returns:
            StreamPos indicating where the event was written

        Raises:
            WalConnectionError: If not connected
            WalTimeoutError: If write times out
            WalError: For other write failures

        Durability:
            Returns only after durable acknowledgment from the backend.
        """
        ...

    @abstractmethod
    async def subscribe(
        self,
        topic: str,
        group_id: str,
        start_position: StreamPos | None = None,
    ) -> AsyncIterator[StreamRecord]:
        """Subscribe to events from the stream.

        Creates a consumer that yields events in order. The consumer
        automatically handles partition assignment and rebalancing.

        Args:
            topic: Topic/stream name to subscribe to
            group_id: Consumer group ID for coordination
            start_position: Optional position to start from (overrides committed)

        Yields:
            StreamRecord objects in order within partitions

        Raises:
            WalConnectionError: If subscription fails
            WalError: For other errors

        Note:
            The caller must call commit() to acknowledge processed events.
        """
        ...

    async def poll_batch(
        self,
        topic: str,
        group_id: str,
        max_records: int = 20,
        timeout_ms: int = 100,
        start_position: StreamPos | None = None,
    ) -> list[StreamRecord]:
        """Poll for a batch of records from the stream.

        Returns whatever records are available right now, up to max_records.
        This enables adaptive batching: during low traffic you get 1 record,
        during high traffic you get max_records. No artificial waiting.

        Args:
            topic: Topic/stream name
            group_id: Consumer group ID
            max_records: Maximum records to return per poll
            timeout_ms: How long to wait for records if none available
            start_position: Optional position to start from

        Returns:
            List of StreamRecord objects (may be empty if no records available)
        """
        # Default implementation collects from subscribe() with a timeout.
        # Backends should override with native batch polling for efficiency.
        records: list[StreamRecord] = []
        try:
            async for record in self.subscribe(topic, group_id, start_position):  # type: ignore[attr-defined]
                records.append(record)
                if len(records) >= max_records:
                    break
        except Exception:
            pass
        return records

    @abstractmethod
    async def commit(self, record: StreamRecord) -> None:
        """Commit a consumed record.

        Acknowledges that the record has been successfully processed.
        The consumer will resume from this position on restart.

        Args:
            record: The record to acknowledge

        Raises:
            WalError: If commit fails
        """
        ...

    @abstractmethod
    async def get_positions(self, topic: str, group_id: str) -> dict[int, StreamPos]:
        """Get committed positions for a consumer group.

        Returns the last committed position for each partition.

        Args:
            topic: Topic name
            group_id: Consumer group ID

        Returns:
            Dictionary mapping partition to last committed position
        """
        ...

    @property
    @abstractmethod
    def is_connected(self) -> bool:
        """Whether currently connected to the backend."""
        ...


def create_wal_stream(config: ServerConfig) -> WalStream:
    """Factory function to create a WAL stream from configuration.

    Args:
        config: Server configuration

    Returns:
        Appropriate WalStream implementation

    Raises:
        ValueError: If backend is not supported
    """
    from ..config import WalBackend
    from .kafka import KafkaWalStream
    from .kinesis import KinesisWalStream

    if config.wal_backend == WalBackend.KAFKA:
        return KafkaWalStream(config.kafka)  # type: ignore[return-value]
    elif config.wal_backend == WalBackend.KINESIS:
        return KinesisWalStream(config.kinesis)  # type: ignore[return-value]
    elif config.wal_backend == WalBackend.PUBSUB:
        from .pubsub import PubSubWalStream

        return PubSubWalStream(config.pubsub)  # type: ignore[return-value]
    elif config.wal_backend == WalBackend.SQS:
        from .sqs import SqsWalStream

        return SqsWalStream(config.sqs)  # type: ignore[return-value]
    elif config.wal_backend == WalBackend.SERVICEBUS:
        from .servicebus import ServiceBusWalStream

        return ServiceBusWalStream(config.servicebus)  # type: ignore[return-value]
    elif config.wal_backend == WalBackend.EVENTHUBS:
        from .eventhubs import EventHubsWalStream

        return EventHubsWalStream(config.eventhubs)  # type: ignore[return-value]
    elif config.wal_backend == WalBackend.LOCAL:
        from .memory import InMemoryWalStream

        return InMemoryWalStream()  # type: ignore[return-value]
    else:
        raise ValueError(f"Unsupported WAL backend: {config.wal_backend}")
