# mypy: ignore-errors
"""
AWS SQS FIFO WAL stream implementation.

Uses SQS FIFO queues for ordered, exactly-once message delivery.
Cheaper than Kafka/MSK for low-to-medium throughput workloads.

Requires FIFO queue (queue name must end in .fifo).

Maps SQS concepts to EntDB WAL:
  - FIFO Queue -> WAL topic
  - MessageGroupId -> Partition key (tenant_id)
  - ReceiptHandle -> Commit handle
  - DeleteMessage -> Acknowledge/commit
  - MessageDeduplicationId -> Idempotency key

Requires:
    pip install aiobotocore (already included in server deps)

Invariants:
    - Messages ordered within a MessageGroupId (per tenant)
    - Exactly-once delivery with content-based deduplication
    - Messages acknowledged on commit() via DeleteMessage
"""

from __future__ import annotations

import asyncio
import hashlib
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

# Try to import aiobotocore, provide helpful message if not installed
try:
    import aiobotocore.session

    AIOBOTOCORE_AVAILABLE = True
except ImportError:
    AIOBOTOCORE_AVAILABLE = False


class SqsWalStream:
    """AWS SQS FIFO implementation of WalStream protocol.

    Uses aiobotocore SQS client for async communication with SQS FIFO queues.

    Attributes:
        config: SqsConfig instance
        _client: aiobotocore SQS client

    Note:
        SQS FIFO queues do not have partitions. MessageGroupId provides
        per-tenant ordering. The StreamPos partition field is always 0.

    Example:
        >>> config = SqsConfig(queue_url="https://sqs.us-east-1.amazonaws.com/123/entdb.fifo")
        >>> wal = SqsWalStream(config)
        >>> await wal.connect()
        >>> pos = await wal.append(config.queue_url, "tenant_1", b'{"op": "create"}')
    """

    def __init__(self, config: Any) -> None:
        """Initialize SQS WAL stream.

        Args:
            config: SqsConfig instance with connection settings

        Raises:
            ImportError: If aiobotocore is not installed
        """
        if not AIOBOTOCORE_AVAILABLE:
            raise ImportError(
                "aiobotocore is required for SQS backend. " "Install with: pip install aiobotocore"
            )

        self.config = config
        self._session: Any = None
        self._client: Any = None
        self._client_ctx: Any = None
        self._connected = False
        self._counter = 0
        # Map of message_id -> receipt_handle for pending acknowledgments
        self._pending_acks: dict[str, str] = {}

    @property
    def is_connected(self) -> bool:
        """Whether connected to SQS."""
        return self._connected and self._client is not None

    async def connect(self) -> None:
        """Connect to AWS SQS.

        Creates an aiobotocore SQS client.

        Raises:
            WalConnectionError: If connection fails
        """
        if self._connected:
            return

        try:
            self._session = aiobotocore.session.get_session()

            client_kwargs: dict[str, Any] = {
                "region_name": self.config.region,
            }
            if self.config.endpoint_url:
                client_kwargs["endpoint_url"] = self.config.endpoint_url

            self._client_ctx = self._session.create_client("sqs", **client_kwargs)
            self._client = await self._client_ctx.__aenter__()
            self._connected = True

            logger.info(
                "Connected to SQS",
                extra={
                    "queue_url": self.config.queue_url,
                    "region": self.config.region,
                    "endpoint": self.config.endpoint_url or "AWS",
                },
            )

        except Exception as e:
            self._connected = False
            raise WalConnectionError(f"Failed to connect to SQS: {e}") from e

    async def close(self) -> None:
        """Close SQS client.

        Releases the aiobotocore client resources.
        """
        if self._client_ctx:
            try:
                await self._client_ctx.__aexit__(None, None, None)
            except Exception as e:
                logger.warning(f"Error closing SQS client: {e}")
            self._client = None
            self._client_ctx = None

        self._connected = False
        self._pending_acks.clear()
        logger.info("SQS connection closed")

    async def append(
        self,
        topic: str,
        key: str,
        value: bytes,
        headers: dict[str, bytes] | None = None,
    ) -> StreamPos:
        """Append event to SQS FIFO queue.

        Sends a message with MessageGroupId for per-tenant ordering and
        MessageDeduplicationId for exactly-once delivery.

        Args:
            topic: Queue URL
            key: MessageGroupId (tenant_id)
            value: Event payload
            headers: Optional message attributes

        Returns:
            StreamPos with topic=queue_url, partition=0

        Raises:
            WalConnectionError: If not connected
            WalTimeoutError: If send times out
            WalError: For other SQS errors
        """
        if not self._client:
            raise WalConnectionError("Not connected to SQS")

        try:
            # Use the configured queue_url as the target
            queue_url = self.config.queue_url or topic

            # Build deduplication ID from content hash
            dedup_id = hashlib.sha256(value).hexdigest()

            # Build message attributes from headers
            msg_attributes: dict[str, Any] = {}
            if headers:
                for k, v in headers.items():
                    msg_attributes[k] = {
                        "StringValue": v.decode("utf-8", errors="replace"),
                        "DataType": "String",
                    }

            send_kwargs: dict[str, Any] = {
                "QueueUrl": queue_url,
                "MessageBody": value.decode("utf-8", errors="replace"),
                "MessageGroupId": key,
                "MessageDeduplicationId": dedup_id,
            }
            if msg_attributes:
                send_kwargs["MessageAttributes"] = msg_attributes

            response = await asyncio.wait_for(
                self._client.send_message(**send_kwargs),
                timeout=30.0,
            )

            now_ms = int(time.time() * 1000)
            self._counter += 1

            pos = StreamPos(
                topic=queue_url,
                partition=0,
                offset=self._counter,
                timestamp_ms=now_ms,
            )

            logger.debug(
                "Event appended to SQS",
                extra={
                    "queue_url": queue_url,
                    "key": key,
                    "message_id": response.get("MessageId"),
                },
            )

            return pos

        except asyncio.TimeoutError:
            raise WalTimeoutError("SQS SendMessage timed out")
        except Exception as e:
            if "timeout" in str(e).lower():
                raise WalTimeoutError(f"SQS send timed out: {e}") from e
            raise WalError(f"SQS SendMessage failed: {e}") from e

    async def subscribe(
        self,
        topic: str,
        group_id: str,
        start_position: StreamPos | None = None,
    ) -> AsyncIterator[StreamRecord]:
        """Subscribe to SQS FIFO queue.

        Continuously polls for messages using ReceiveMessage.

        Args:
            topic: Queue URL
            group_id: Not used by SQS (consumer groups are implicit)
            start_position: Not supported by SQS (messages are delivered in order)

        Yields:
            StreamRecord for each message

        Raises:
            WalConnectionError: If not connected
        """
        if not self._client:
            raise WalConnectionError("Not connected to SQS")

        queue_url = self.config.queue_url or topic

        logger.info(
            "Subscribed to SQS",
            extra={"queue_url": queue_url, "group_id": group_id},
        )

        while True:
            try:
                response = await self._client.receive_message(
                    QueueUrl=queue_url,
                    MaxNumberOfMessages=self.config.max_messages,
                    WaitTimeSeconds=self.config.wait_time_seconds,
                    VisibilityTimeout=self.config.visibility_timeout,
                    AttributeNames=["SentTimestamp"],
                    MessageAttributeNames=["All"],
                )

                messages = response.get("Messages", [])

                for msg in messages:
                    message_id = msg["MessageId"]
                    receipt_handle = msg["ReceiptHandle"]
                    body = msg.get("Body", "")
                    sent_timestamp = int(msg.get("Attributes", {}).get("SentTimestamp", "0"))

                    # Convert message attributes to headers
                    headers: dict[str, bytes] = {}
                    for attr_name, attr_val in msg.get("MessageAttributes", {}).items():
                        str_val = attr_val.get("StringValue", "")
                        headers[attr_name] = str_val.encode("utf-8")

                    self._counter += 1

                    record = StreamRecord(
                        key=msg.get("Attributes", {}).get("MessageGroupId", ""),
                        value=body.encode("utf-8") if isinstance(body, str) else body,
                        position=StreamPos(
                            topic=queue_url,
                            partition=0,
                            offset=self._counter,
                            timestamp_ms=sent_timestamp,
                        ),
                        headers=headers,
                    )

                    # Store receipt handle for later acknowledgment
                    self._pending_acks[message_id] = receipt_handle
                    # Also store by offset for commit() lookup
                    self._pending_acks[str(self._counter)] = receipt_handle

                    yield record

                if not messages:
                    await asyncio.sleep(0.2)

            except Exception as e:
                logger.warning(f"SQS receive error: {e}")
                await asyncio.sleep(1.0)

    async def poll_batch(
        self,
        topic: str,
        group_id: str,
        max_records: int = 10,
        timeout_ms: int = 100,
        start_position: StreamPos | None = None,
    ) -> list[StreamRecord]:
        """Poll for a batch of records from SQS.

        Args:
            topic: Queue URL
            group_id: Not used by SQS
            max_records: Maximum messages to receive (SQS max is 10)
            timeout_ms: Wait time for long polling
            start_position: Not supported by SQS

        Returns:
            List of StreamRecord objects
        """
        if not self._client:
            raise WalConnectionError("Not connected to SQS")

        queue_url = self.config.queue_url or topic

        # SQS limits MaxNumberOfMessages to 10
        batch_size = min(max_records, 10)
        wait_seconds = max(1, timeout_ms // 1000)

        try:
            response = await self._client.receive_message(
                QueueUrl=queue_url,
                MaxNumberOfMessages=batch_size,
                WaitTimeSeconds=wait_seconds,
                VisibilityTimeout=self.config.visibility_timeout,
                AttributeNames=["SentTimestamp"],
                MessageAttributeNames=["All"],
            )
        except Exception as e:
            logger.warning(f"SQS poll_batch error: {e}")
            return []

        records: list[StreamRecord] = []
        for msg in response.get("Messages", []):
            message_id = msg["MessageId"]
            receipt_handle = msg["ReceiptHandle"]
            body = msg.get("Body", "")
            sent_timestamp = int(msg.get("Attributes", {}).get("SentTimestamp", "0"))

            headers: dict[str, bytes] = {}
            for attr_name, attr_val in msg.get("MessageAttributes", {}).items():
                str_val = attr_val.get("StringValue", "")
                headers[attr_name] = str_val.encode("utf-8")

            self._counter += 1

            record = StreamRecord(
                key=msg.get("Attributes", {}).get("MessageGroupId", ""),
                value=body.encode("utf-8") if isinstance(body, str) else body,
                position=StreamPos(
                    topic=queue_url,
                    partition=0,
                    offset=self._counter,
                    timestamp_ms=sent_timestamp,
                ),
                headers=headers,
            )

            self._pending_acks[message_id] = receipt_handle
            self._pending_acks[str(self._counter)] = receipt_handle

            records.append(record)

        return records

    async def commit(self, record: StreamRecord) -> None:
        """Acknowledge a consumed message by deleting it from the queue.

        Args:
            record: The record to acknowledge

        Raises:
            WalError: If deletion fails
        """
        if not self._client:
            raise WalError("No active SQS client to commit")

        try:
            # Look up receipt handle by offset
            ack_key = str(record.position.offset)
            receipt_handle = self._pending_acks.pop(ack_key, None)

            if not receipt_handle:
                logger.warning(
                    "No pending receipt handle found for record",
                    extra={"offset": record.position.offset},
                )
                return

            queue_url = record.position.topic or self.config.queue_url

            await self._client.delete_message(
                QueueUrl=queue_url,
                ReceiptHandle=receipt_handle,
            )

            logger.debug(
                "Deleted message from SQS",
                extra={
                    "queue_url": queue_url,
                    "offset": record.position.offset,
                },
            )

        except Exception as e:
            raise WalError(f"Failed to delete SQS message: {e}") from e

    async def get_positions(self, topic: str, group_id: str) -> dict[int, StreamPos]:
        """Get committed positions.

        SQS does not expose consumer offsets. Returns empty dict since
        SQS manages delivery state internally via visibility timeout
        and message deletion.

        Returns:
            Empty dictionary (SQS manages positions internally)
        """
        return {}

    async def health_check(self) -> bool:
        """Check if SQS connection is healthy.

        Returns:
            True if connected and queue is accessible
        """
        if not self._client or not self.config.queue_url:
            return False

        try:
            await asyncio.wait_for(
                self._client.get_queue_attributes(
                    QueueUrl=self.config.queue_url,
                    AttributeNames=["QueueArn"],
                ),
                timeout=5.0,
            )
            return True
        except Exception:
            return False
