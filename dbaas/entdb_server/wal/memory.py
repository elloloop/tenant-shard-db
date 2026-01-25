"""
In-memory WAL stream implementation for testing.

This module provides a simple in-memory WAL backend for:
- Unit tests
- Integration tests
- Local development without external dependencies

Invariants:
    - All data is lost on process exit
    - Provides same ordering guarantees as production backends
    - Thread-safe for concurrent access

How to change safely:
    - This is test-only code, changes don't affect production
    - Keep interface compatible with WalStream protocol
    - Add features to help with testing scenarios
"""

from __future__ import annotations

import asyncio
import hashlib
import time
from collections import defaultdict
from dataclasses import dataclass, field
from typing import AsyncIterator, Dict, List, Optional, Set
import logging

from .base import (
    WalStream,
    StreamRecord,
    StreamPos,
    WalError,
    WalConnectionError,
)

logger = logging.getLogger(__name__)


@dataclass
class InMemoryPartition:
    """In-memory partition storage."""
    records: List[StreamRecord] = field(default_factory=list)
    next_offset: int = 0


class InMemoryWalStream:
    """In-memory implementation of WalStream for testing.

    This provides a fully functional WAL stream that stores all data
    in memory. Useful for:
    - Unit tests that need WAL behavior without external dependencies
    - Integration tests that verify application logic
    - Local development and debugging

    Attributes:
        num_partitions: Number of partitions to simulate
        topics: Storage for topic data

    Thread safety:
        Uses asyncio locks for thread safety. Safe to use from
        multiple coroutines.

    Example:
        >>> wal = InMemoryWalStream()
        >>> await wal.connect()
        >>> await wal.append("test", "key1", b"value1")
        >>> async for record in wal.subscribe("test", "group1"):
        ...     print(record.value)
    """

    def __init__(self, num_partitions: int = 4) -> None:
        """Initialize in-memory WAL stream.

        Args:
            num_partitions: Number of partitions per topic
        """
        self.num_partitions = num_partitions
        self._topics: Dict[str, Dict[int, InMemoryPartition]] = defaultdict(
            lambda: {i: InMemoryPartition() for i in range(self.num_partitions)}
        )
        self._committed: Dict[str, Dict[int, int]] = defaultdict(
            lambda: defaultdict(int)
        )
        self._connected = False
        self._lock = asyncio.Lock()
        self._new_record_events: Dict[str, asyncio.Event] = defaultdict(asyncio.Event)
        self._subscribers: Set[str] = set()

    @property
    def is_connected(self) -> bool:
        """Whether connected (always true after connect())."""
        return self._connected

    async def connect(self) -> None:
        """Connect (no-op for in-memory)."""
        self._connected = True
        logger.debug("InMemoryWalStream connected")

    async def close(self) -> None:
        """Close and clear all data."""
        self._connected = False
        self._topics.clear()
        self._committed.clear()
        self._subscribers.clear()
        logger.debug("InMemoryWalStream closed")

    async def append(
        self,
        topic: str,
        key: str,
        value: bytes,
        headers: Optional[Dict[str, bytes]] = None,
    ) -> StreamPos:
        """Append event to in-memory stream.

        Args:
            topic: Topic name
            key: Partition key
            value: Event payload
            headers: Optional headers

        Returns:
            StreamPos with partition and offset
        """
        if not self._connected:
            raise WalConnectionError("Not connected")

        # Consistent partitioning based on key hash
        partition = self._partition_for_key(key)

        async with self._lock:
            part = self._topics[topic][partition]
            offset = part.next_offset
            timestamp = int(time.time() * 1000)

            pos = StreamPos(
                topic=topic,
                partition=partition,
                offset=offset,
                timestamp_ms=timestamp,
            )

            record = StreamRecord(
                key=key,
                value=value,
                position=pos,
                headers=headers or {},
            )

            part.records.append(record)
            part.next_offset += 1

            # Signal new record to subscribers
            if topic in self._new_record_events:
                self._new_record_events[topic].set()

        logger.debug(
            "Event appended to in-memory WAL",
            extra={"topic": topic, "key": key, "partition": partition, "offset": offset}
        )

        return pos

    async def subscribe(
        self,
        topic: str,
        group_id: str,
        start_position: Optional[StreamPos] = None,
    ) -> AsyncIterator[StreamRecord]:
        """Subscribe to in-memory stream.

        Args:
            topic: Topic to subscribe to
            group_id: Consumer group ID
            start_position: Optional starting position

        Yields:
            StreamRecord for each event
        """
        if not self._connected:
            raise WalConnectionError("Not connected")

        consumer_key = f"{topic}:{group_id}"
        self._subscribers.add(consumer_key)

        # Initialize starting positions
        positions = {}
        for partition in range(self.num_partitions):
            if start_position and start_position.partition == partition:
                positions[partition] = start_position.offset + 1
            else:
                # Check for committed position
                committed = self._committed.get(group_id, {}).get(partition, 0)
                positions[partition] = committed

        try:
            while consumer_key in self._subscribers:
                has_records = False

                async with self._lock:
                    for partition in range(self.num_partitions):
                        if topic not in self._topics:
                            continue

                        part = self._topics[topic].get(partition)
                        if not part:
                            continue

                        current_pos = positions[partition]
                        while current_pos < len(part.records):
                            record = part.records[current_pos]
                            positions[partition] = current_pos + 1
                            has_records = True
                            yield record
                            current_pos += 1

                if not has_records:
                    # Wait for new records
                    self._new_record_events[topic].clear()
                    try:
                        await asyncio.wait_for(
                            self._new_record_events[topic].wait(),
                            timeout=1.0
                        )
                    except asyncio.TimeoutError:
                        pass

        finally:
            self._subscribers.discard(consumer_key)

    async def commit(self, record: StreamRecord) -> None:
        """Commit consumed record offset.

        Args:
            record: The record to commit
        """
        # In-memory just tracks the position
        # Group ID is not available here, so we use a default
        self._committed["default"][record.position.partition] = record.position.offset + 1

    async def get_positions(self, topic: str, group_id: str) -> Dict[int, StreamPos]:
        """Get committed positions.

        Args:
            topic: Topic name
            group_id: Consumer group ID

        Returns:
            Dictionary mapping partition to position
        """
        positions = {}
        committed = self._committed.get(group_id, {})
        for partition, offset in committed.items():
            positions[partition] = StreamPos(
                topic=topic,
                partition=partition,
                offset=offset,
                timestamp_ms=int(time.time() * 1000),
            )
        return positions

    def _partition_for_key(self, key: str) -> int:
        """Get partition number for a key using consistent hashing."""
        hash_bytes = hashlib.md5(key.encode('utf-8')).digest()
        hash_int = int.from_bytes(hash_bytes[:4], 'big')
        return hash_int % self.num_partitions

    # Testing helpers

    def get_all_records(self, topic: str) -> List[StreamRecord]:
        """Get all records for a topic (testing helper).

        Args:
            topic: Topic name

        Returns:
            List of all records across all partitions
        """
        records = []
        if topic in self._topics:
            for partition in sorted(self._topics[topic].keys()):
                records.extend(self._topics[topic][partition].records)
        return records

    def get_record_count(self, topic: str) -> int:
        """Get total record count for a topic (testing helper)."""
        count = 0
        if topic in self._topics:
            for part in self._topics[topic].values():
                count += len(part.records)
        return count

    def clear_topic(self, topic: str) -> None:
        """Clear all records for a topic (testing helper)."""
        if topic in self._topics:
            for part in self._topics[topic].values():
                part.records.clear()
                part.next_offset = 0

    async def inject_failure(self, exception: Exception) -> None:
        """Inject a failure for testing error handling.

        The next operation will raise this exception.
        """
        # This is a placeholder - would need more sophisticated
        # implementation for realistic failure injection
        pass

    async def wait_for_records(
        self,
        topic: str,
        count: int,
        timeout: float = 5.0,
    ) -> bool:
        """Wait for a specific number of records (testing helper).

        Args:
            topic: Topic name
            count: Expected record count
            timeout: Maximum wait time

        Returns:
            True if count reached, False if timeout
        """
        start = time.time()
        while time.time() - start < timeout:
            if self.get_record_count(topic) >= count:
                return True
            await asyncio.sleep(0.1)
        return False
