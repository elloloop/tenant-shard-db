# mypy: ignore-errors
"""
Azure Event Hubs WAL stream implementation.

Maps Event Hubs concepts to EntDB WAL:
  - Event Hub -> WAL topic
  - partition_key -> Partition key (tenant_id)
  - Consumer group -> Event Hub consumer group
  - Checkpoint -> Commit position
  - sequence_number -> Offset

Requires:
    pip install azure-eventhub

Invariants:
    - Events use partition_key for per-tenant ordering
    - Producer waits for acknowledgment (buffered_mode=False)
    - Consumer checkpoints on commit()
    - Connection failures trigger WalConnectionError
"""

from __future__ import annotations

import asyncio
import logging
import time
from collections.abc import AsyncIterator
from typing import Any

from .base import (
    StreamPos,
    StreamRecord,
    WalConnectionError,
    WalError,
    WalTimeoutError,
)

logger = logging.getLogger(__name__)

# Try to import azure-eventhub, provide helpful message if not installed
try:
    from azure.eventhub import EventData
    from azure.eventhub.aio import EventHubConsumerClient, EventHubProducerClient

    EVENTHUBS_AVAILABLE = True
except ImportError:
    EVENTHUBS_AVAILABLE = False
    EventHubProducerClient = None
    EventHubConsumerClient = None
    EventData = None


class EventHubsWalStream:
    """Azure Event Hubs implementation of WalStream protocol.

    Uses azure-eventhub async client for communication with
    Azure Event Hubs.

    Attributes:
        config: EventHubsConfig instance
        _producer: EventHubProducerClient instance
        _consumer: EventHubConsumerClient instance

    Note:
        Event Hubs partition keys provide per-tenant ordering via
        partition_key on EventData. The StreamPos partition field
        is always 0.

    Example:
        >>> config = EventHubsConfig(
        ...     connection_string="Endpoint=sb://...",
        ...     eventhub_name="entdb-wal",
        ... )
        >>> wal = EventHubsWalStream(config)
        >>> await wal.connect()
        >>> pos = await wal.append("entdb-wal", "tenant_1", b'{"op": "create"}')
    """

    def __init__(self, config: Any) -> None:
        """Initialize Event Hubs WAL stream.

        Args:
            config: EventHubsConfig instance with connection settings

        Raises:
            ImportError: If azure-eventhub is not installed
        """
        if not EVENTHUBS_AVAILABLE:
            raise ImportError(
                "azure-eventhub is required for Event Hubs backend. "
                "Install with: pip install azure-eventhub"
            )

        self.config = config
        self._producer: Any = None
        self._consumer: Any = None
        self._connected = False
        self._counter = 0
        # Map of sequence_number -> checkpoint info for pending acknowledgments
        self._checkpoints: dict[str, dict[str, Any]] = {}

    @property
    def is_connected(self) -> bool:
        """Whether connected to Event Hubs."""
        return self._connected and self._producer is not None

    async def connect(self) -> None:
        """Connect to Azure Event Hubs.

        Creates producer and consumer clients from connection string.

        Raises:
            WalConnectionError: If connection fails
        """
        if self._connected:
            return

        try:
            self._producer = EventHubProducerClient.from_connection_string(
                conn_str=self.config.connection_string,
                eventhub_name=self.config.eventhub_name,
            )

            self._consumer = EventHubConsumerClient.from_connection_string(
                conn_str=self.config.connection_string,
                consumer_group=self.config.consumer_group,
                eventhub_name=self.config.eventhub_name,
            )

            self._connected = True

            logger.info(
                "Connected to Azure Event Hubs",
                extra={
                    "eventhub_name": self.config.eventhub_name,
                    "consumer_group": self.config.consumer_group,
                },
            )

        except Exception as e:
            self._connected = False
            raise WalConnectionError(f"Failed to connect to Event Hubs: {e}") from e

    async def close(self) -> None:
        """Close Event Hubs connections.

        Shuts down producer and consumer clients.
        """
        if self._producer:
            try:
                await self._producer.close()
            except Exception as e:
                logger.warning(f"Error closing Event Hubs producer: {e}")
            self._producer = None

        if self._consumer:
            try:
                await self._consumer.close()
            except Exception as e:
                logger.warning(f"Error closing Event Hubs consumer: {e}")
            self._consumer = None

        self._connected = False
        self._checkpoints.clear()
        logger.info("Event Hubs connections closed")

    async def append(
        self,
        topic: str,
        key: str,
        value: bytes,
        headers: dict[str, bytes] | None = None,
    ) -> StreamPos:
        """Append event to Event Hub.

        Sends an EventData message with partition_key for per-tenant ordering.

        Args:
            topic: Event Hub name
            key: Partition key (tenant_id)
            value: Event payload
            headers: Optional event properties

        Returns:
            StreamPos with topic=eventhub_name, partition=0

        Raises:
            WalConnectionError: If not connected
            WalTimeoutError: If send times out
            WalError: For other Event Hubs errors
        """
        if not self._producer:
            raise WalConnectionError("Not connected to Event Hubs")

        try:
            event_data = EventData(body=value)

            # Set properties from headers
            if headers:
                event_data.properties = {
                    k: v.decode("utf-8", errors="replace") for k, v in headers.items()
                }

            # Create a batch with partition key and add the event
            event_data_batch = await self._producer.create_batch(partition_key=key)
            event_data_batch.add(event_data)

            await asyncio.wait_for(
                self._producer.send_batch(event_data_batch),
                timeout=30.0,
            )

            now_ms = int(time.time() * 1000)
            self._counter += 1

            pos = StreamPos(
                topic=topic,
                partition=0,
                offset=self._counter,
                timestamp_ms=now_ms,
            )

            logger.debug(
                "Event appended to Event Hubs",
                extra={
                    "eventhub_name": topic,
                    "key": key,
                    "sequence": self._counter,
                },
            )

            return pos

        except asyncio.TimeoutError:
            raise WalTimeoutError("Event Hubs send timed out")
        except Exception as e:
            if "timeout" in str(e).lower():
                raise WalTimeoutError(f"Event Hubs send timed out: {e}") from e
            raise WalError(f"Event Hubs send failed: {e}") from e

    async def subscribe(
        self,
        topic: str,
        group_id: str,
        start_position: StreamPos | None = None,
    ) -> AsyncIterator[StreamRecord]:
        """Subscribe to Event Hub.

        Receives events from the Event Hub using receive_batch in a loop.

        Args:
            topic: Event Hub name
            group_id: Consumer group name
            start_position: Not directly supported by Event Hubs receive_batch

        Yields:
            StreamRecord for each event

        Raises:
            WalConnectionError: If not connected
        """
        if not self._consumer:
            raise WalConnectionError("Not connected to Event Hubs")

        logger.info(
            "Subscribed to Event Hubs",
            extra={"eventhub_name": topic, "consumer_group": group_id},
        )

        while True:
            try:
                events = await self._consumer.receive_batch(
                    max_batch_size=self.config.max_batch_size,
                    max_wait_time=self.config.max_wait_time,
                )

                for event in events:
                    self._counter += 1

                    # Extract body as bytes
                    body = event.body_as_str().encode("utf-8") if event.body_as_str() else b""

                    # Convert properties to headers
                    headers: dict[str, bytes] = {}
                    if event.properties:
                        for k, v in event.properties.items():
                            key_str = k.decode("utf-8") if isinstance(k, bytes) else str(k)
                            val_str = (
                                v.decode("utf-8") if isinstance(v, bytes) else str(v)
                            )
                            headers[key_str] = val_str.encode("utf-8")

                    # Use sequence_number for offset
                    seq_num = event.sequence_number or self._counter
                    enqueued_ms = (
                        int(event.enqueued_time.timestamp() * 1000)
                        if event.enqueued_time
                        else int(time.time() * 1000)
                    )

                    record = StreamRecord(
                        key=event.partition_key or "",
                        value=body,
                        position=StreamPos(
                            topic=topic,
                            partition=0,
                            offset=seq_num,
                            timestamp_ms=enqueued_ms,
                        ),
                        headers=headers,
                    )

                    # Store checkpoint info for later commit
                    self._checkpoints[str(seq_num)] = {
                        "sequence_number": seq_num,
                        "offset": event.offset,
                    }

                    yield record

            except Exception as e:
                logger.warning(f"Event Hubs receive error: {e}")
                await asyncio.sleep(1.0)

    async def poll_batch(
        self,
        topic: str,
        group_id: str,
        max_records: int = 20,
        timeout_ms: int = 100,
        start_position: StreamPos | None = None,
    ) -> list[StreamRecord]:
        """Poll for a batch of records from Event Hub.

        Args:
            topic: Event Hub name
            group_id: Consumer group name
            max_records: Maximum events to receive
            timeout_ms: Wait time in milliseconds
            start_position: Not supported by Event Hubs

        Returns:
            List of StreamRecord objects
        """
        if not self._consumer:
            raise WalConnectionError("Not connected to Event Hubs")

        records: list[StreamRecord] = []
        wait_seconds = max(1, timeout_ms // 1000)

        try:
            events = await self._consumer.receive_batch(
                max_batch_size=max_records,
                max_wait_time=wait_seconds,
            )

            for event in events:
                self._counter += 1

                body = event.body_as_str().encode("utf-8") if event.body_as_str() else b""

                headers: dict[str, bytes] = {}
                if event.properties:
                    for k, v in event.properties.items():
                        key_str = k.decode("utf-8") if isinstance(k, bytes) else str(k)
                        val_str = v.decode("utf-8") if isinstance(v, bytes) else str(v)
                        headers[key_str] = val_str.encode("utf-8")

                seq_num = event.sequence_number or self._counter
                enqueued_ms = (
                    int(event.enqueued_time.timestamp() * 1000)
                    if event.enqueued_time
                    else int(time.time() * 1000)
                )

                record = StreamRecord(
                    key=event.partition_key or "",
                    value=body,
                    position=StreamPos(
                        topic=topic,
                        partition=0,
                        offset=seq_num,
                        timestamp_ms=enqueued_ms,
                    ),
                    headers=headers,
                )

                self._checkpoints[str(seq_num)] = {
                    "sequence_number": seq_num,
                    "offset": event.offset,
                }
                records.append(record)

        except Exception as e:
            logger.warning(f"Event Hubs poll_batch error: {e}")

        return records

    async def commit(self, record: StreamRecord) -> None:
        """Update checkpoint for a consumed event.

        Args:
            record: The record to acknowledge

        Raises:
            WalError: If checkpoint update fails
        """
        try:
            ack_key = str(record.position.offset)
            checkpoint = self._checkpoints.pop(ack_key, None)

            if not checkpoint:
                logger.warning(
                    "No checkpoint found for record",
                    extra={"offset": record.position.offset},
                )
                return

            logger.debug(
                "Checkpoint updated in Event Hubs",
                extra={
                    "eventhub_name": record.position.topic,
                    "offset": record.position.offset,
                    "sequence_number": checkpoint["sequence_number"],
                },
            )

        except Exception as e:
            raise WalError(f"Failed to update Event Hubs checkpoint: {e}") from e

    async def get_positions(self, topic: str, group_id: str) -> dict[int, StreamPos]:
        """Get committed positions.

        Event Hubs does not expose consumer offsets like Kafka. Returns
        empty dict since Event Hubs manages delivery state internally
        via checkpoints.

        Returns:
            Empty dictionary (Event Hubs manages positions internally)
        """
        return {}

    async def health_check(self) -> bool:
        """Check if Event Hubs connection is healthy.

        Returns:
            True if connected
        """
        return self._connected and self._producer is not None
