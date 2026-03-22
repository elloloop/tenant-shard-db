# mypy: ignore-errors
"""
Azure Service Bus WAL stream implementation.

Uses Service Bus queues with sessions for ordered message delivery.

Maps Service Bus concepts to EntDB WAL:
  - Queue/Topic -> WAL topic
  - SessionId -> Partition key (tenant_id)
  - Complete -> Commit/acknowledge
  - SequenceNumber -> Offset

Requires:
    pip install azure-servicebus

Invariants:
    - Messages ordered within a session (per tenant)
    - Messages completed on commit()
    - Connection failures trigger reconnection
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

# Try to import azure-servicebus, provide helpful message if not installed
try:
    from azure.servicebus import ServiceBusMessage
    from azure.servicebus.aio import ServiceBusClient

    SERVICEBUS_AVAILABLE = True
except ImportError:
    SERVICEBUS_AVAILABLE = False
    ServiceBusClient = None
    ServiceBusMessage = None


class ServiceBusWalStream:
    """Azure Service Bus implementation of WalStream protocol.

    Uses azure-servicebus async client for communication with
    Azure Service Bus queues.

    Attributes:
        config: ServiceBusConfig instance
        _client: ServiceBusClient instance

    Note:
        Service Bus sessions provide per-tenant ordering via SessionId.
        The StreamPos partition field is always 0.

    Example:
        >>> config = ServiceBusConfig(
        ...     connection_string="Endpoint=sb://...",
        ...     queue_name="entdb-wal",
        ... )
        >>> wal = ServiceBusWalStream(config)
        >>> await wal.connect()
        >>> pos = await wal.append("entdb-wal", "tenant_1", b'{"op": "create"}')
    """

    def __init__(self, config: Any) -> None:
        """Initialize Service Bus WAL stream.

        Args:
            config: ServiceBusConfig instance with connection settings

        Raises:
            ImportError: If azure-servicebus is not installed
        """
        if not SERVICEBUS_AVAILABLE:
            raise ImportError(
                "azure-servicebus is required for Service Bus backend. "
                "Install with: pip install azure-servicebus"
            )

        self.config = config
        self._client: Any = None
        self._sender: Any = None
        self._connected = False
        self._counter = 0
        # Map of sequence_number -> received message for pending acknowledgments
        self._pending_messages: dict[str, Any] = {}
        self._receiver: Any = None

    @property
    def is_connected(self) -> bool:
        """Whether connected to Service Bus."""
        return self._connected and self._client is not None

    async def connect(self) -> None:
        """Connect to Azure Service Bus.

        Creates a ServiceBusClient from connection string.

        Raises:
            WalConnectionError: If connection fails
        """
        if self._connected:
            return

        try:
            self._client = ServiceBusClient.from_connection_string(
                conn_str=self.config.connection_string,
            )

            # Create a sender for the queue
            self._sender = self._client.get_queue_sender(
                queue_name=self.config.queue_name,
            )

            self._connected = True

            logger.info(
                "Connected to Azure Service Bus",
                extra={
                    "queue_name": self.config.queue_name,
                },
            )

        except Exception as e:
            self._connected = False
            raise WalConnectionError(f"Failed to connect to Service Bus: {e}") from e

    async def close(self) -> None:
        """Close Service Bus connections.

        Shuts down sender, receiver, and client.
        """
        if self._sender:
            try:
                await self._sender.close()
            except Exception as e:
                logger.warning(f"Error closing Service Bus sender: {e}")
            self._sender = None

        if self._receiver:
            try:
                await self._receiver.close()
            except Exception as e:
                logger.warning(f"Error closing Service Bus receiver: {e}")
            self._receiver = None

        if self._client:
            try:
                await self._client.close()
            except Exception as e:
                logger.warning(f"Error closing Service Bus client: {e}")
            self._client = None

        self._connected = False
        self._pending_messages.clear()
        logger.info("Service Bus connections closed")

    async def append(
        self,
        topic: str,
        key: str,
        value: bytes,
        headers: dict[str, bytes] | None = None,
    ) -> StreamPos:
        """Append event to Service Bus queue.

        Sends a message with session_id for per-tenant ordering.

        Args:
            topic: Queue name
            key: Session ID (tenant_id)
            value: Event payload
            headers: Optional application properties

        Returns:
            StreamPos with topic=queue_name, partition=0

        Raises:
            WalConnectionError: If not connected
            WalTimeoutError: If send times out
            WalError: For other Service Bus errors
        """
        if not self._sender:
            raise WalConnectionError("Not connected to Service Bus")

        try:
            # Create message with session ID for ordering
            message = ServiceBusMessage(
                body=value,
                session_id=key,
            )

            # Set application properties from headers
            if headers:
                message.application_properties = {
                    k: v.decode("utf-8", errors="replace") for k, v in headers.items()
                }

            await asyncio.wait_for(
                self._sender.send_messages(message),
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
                "Event appended to Service Bus",
                extra={
                    "queue_name": topic,
                    "key": key,
                    "sequence": self._counter,
                },
            )

            return pos

        except asyncio.TimeoutError:
            raise WalTimeoutError("Service Bus send timed out")
        except Exception as e:
            if "timeout" in str(e).lower():
                raise WalTimeoutError(f"Service Bus send timed out: {e}") from e
            raise WalError(f"Service Bus send failed: {e}") from e

    async def subscribe(
        self,
        topic: str,
        group_id: str,
        start_position: StreamPos | None = None,
    ) -> AsyncIterator[StreamRecord]:
        """Subscribe to Service Bus queue.

        Receives messages from the queue. Uses session-aware receiver
        for ordered delivery.

        Args:
            topic: Queue name
            group_id: Subscription name (used for logging)
            start_position: Not directly supported by Service Bus

        Yields:
            StreamRecord for each message

        Raises:
            WalConnectionError: If not connected
        """
        if not self._client:
            raise WalConnectionError("Not connected to Service Bus")

        logger.info(
            "Subscribed to Service Bus",
            extra={"queue_name": topic, "group_id": group_id},
        )

        while True:
            try:
                # Create a receiver for the queue
                receiver = self._client.get_queue_receiver(
                    queue_name=self.config.queue_name,
                    max_wait_time=self.config.max_wait_time_seconds,
                )

                async with receiver:
                    async for msg in receiver:
                        self._counter += 1

                        # Extract body as bytes
                        body = b"".join(msg.body)

                        # Convert application properties to headers
                        headers: dict[str, bytes] = {}
                        if msg.application_properties:
                            for k, v in msg.application_properties.items():
                                key_str = k.decode("utf-8") if isinstance(k, bytes) else str(k)
                                val_str = v.decode("utf-8") if isinstance(v, bytes) else str(v)
                                headers[key_str] = val_str.encode("utf-8")

                        # Use sequence_number for offset
                        seq_num = msg.sequence_number or self._counter
                        enqueued_ms = (
                            int(msg.enqueued_time_utc.timestamp() * 1000)
                            if msg.enqueued_time_utc
                            else int(time.time() * 1000)
                        )

                        record = StreamRecord(
                            key=msg.session_id or "",
                            value=body,
                            position=StreamPos(
                                topic=topic,
                                partition=0,
                                offset=seq_num,
                                timestamp_ms=enqueued_ms,
                            ),
                            headers=headers,
                        )

                        # Store the message for later completion
                        self._pending_messages[str(seq_num)] = (receiver, msg)

                        yield record

            except Exception as e:
                logger.warning(f"Service Bus receive error: {e}")
                await asyncio.sleep(1.0)

    async def poll_batch(
        self,
        topic: str,
        group_id: str,
        max_records: int = 20,
        timeout_ms: int = 100,
        start_position: StreamPos | None = None,
    ) -> list[StreamRecord]:
        """Poll for a batch of records from Service Bus.

        Args:
            topic: Queue name
            group_id: Not used (Service Bus manages delivery)
            max_records: Maximum messages to receive
            timeout_ms: Wait time in milliseconds
            start_position: Not supported by Service Bus

        Returns:
            List of StreamRecord objects
        """
        if not self._client:
            raise WalConnectionError("Not connected to Service Bus")

        records: list[StreamRecord] = []
        wait_seconds = max(1, timeout_ms // 1000)

        try:
            receiver = self._client.get_queue_receiver(
                queue_name=self.config.queue_name,
                max_wait_time=wait_seconds,
            )

            async with receiver:
                messages = await receiver.receive_messages(
                    max_message_count=max_records,
                    max_wait_time=wait_seconds,
                )

                for msg in messages:
                    self._counter += 1

                    body = b"".join(msg.body)

                    headers: dict[str, bytes] = {}
                    if msg.application_properties:
                        for k, v in msg.application_properties.items():
                            key_str = k.decode("utf-8") if isinstance(k, bytes) else str(k)
                            val_str = v.decode("utf-8") if isinstance(v, bytes) else str(v)
                            headers[key_str] = val_str.encode("utf-8")

                    seq_num = msg.sequence_number or self._counter
                    enqueued_ms = (
                        int(msg.enqueued_time_utc.timestamp() * 1000)
                        if msg.enqueued_time_utc
                        else int(time.time() * 1000)
                    )

                    record = StreamRecord(
                        key=msg.session_id or "",
                        value=body,
                        position=StreamPos(
                            topic=topic,
                            partition=0,
                            offset=seq_num,
                            timestamp_ms=enqueued_ms,
                        ),
                        headers=headers,
                    )

                    self._pending_messages[str(seq_num)] = (receiver, msg)
                    records.append(record)

        except Exception as e:
            logger.warning(f"Service Bus poll_batch error: {e}")

        return records

    async def commit(self, record: StreamRecord) -> None:
        """Complete a consumed message.

        Args:
            record: The record to acknowledge

        Raises:
            WalError: If completion fails
        """
        try:
            ack_key = str(record.position.offset)
            pending = self._pending_messages.pop(ack_key, None)

            if not pending:
                logger.warning(
                    "No pending message found for record",
                    extra={"offset": record.position.offset},
                )
                return

            receiver, msg = pending

            await receiver.complete_message(msg)

            logger.debug(
                "Completed message in Service Bus",
                extra={
                    "queue_name": record.position.topic,
                    "offset": record.position.offset,
                },
            )

        except Exception as e:
            raise WalError(f"Failed to complete Service Bus message: {e}") from e

    async def get_positions(self, topic: str, group_id: str) -> dict[int, StreamPos]:
        """Get committed positions.

        Service Bus does not expose consumer offsets like Kafka. Returns
        empty dict since Service Bus manages delivery state internally
        via message completion.

        Returns:
            Empty dictionary (Service Bus manages positions internally)
        """
        return {}

    async def health_check(self) -> bool:
        """Check if Service Bus connection is healthy.

        Returns:
            True if connected
        """
        return self._connected and self._client is not None
