# mypy: ignore-errors
"""
Google Cloud Pub/Sub WAL stream implementation.

Maps Pub/Sub concepts to EntDB WAL:
  - Pub/Sub Topic -> WAL topic
  - Ordering key -> Partition key (tenant_id)
  - Subscription -> Consumer group
  - Acknowledge -> Commit

Requires:
    pip install google-cloud-pubsub

Invariants:
    - Messages use ordering keys for per-tenant ordering
    - Publisher waits for server acknowledgment (durable write)
    - Messages are acknowledged on commit() for exactly-once semantics
    - Connection failures trigger appropriate error types
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

# Try to import google-cloud-pubsub, provide helpful message if not installed
try:
    from google.api_core.exceptions import DeadlineExceeded, GoogleAPICallError, NotFound
    from google.cloud.pubsub_v1 import PublisherClient, SubscriberClient
    from google.cloud.pubsub_v1.types import PullRequest

    PUBSUB_AVAILABLE = True
except ImportError:
    PUBSUB_AVAILABLE = False
    PublisherClient = None
    SubscriberClient = None
    PullRequest = None
    DeadlineExceeded = None
    NotFound = None
    GoogleAPICallError = None


class PubSubWalStream:
    """Google Cloud Pub/Sub implementation of WalStream protocol.

    Uses google-cloud-pubsub synchronous clients wrapped in asyncio executors
    for async compatibility.

    Attributes:
        config: PubSubConfig instance
        publisher: Pub/Sub publisher client
        subscriber: Pub/Sub subscriber client

    Note:
        Pub/Sub does not have partitions like Kafka. Ordering keys are used
        to provide per-tenant ordering guarantees. The StreamPos partition
        field is always 0.

    Example:
        >>> config = PubSubConfig(project_id="my-project", topic_id="entdb-wal")
        >>> wal = PubSubWalStream(config)
        >>> await wal.connect()
        >>> pos = await wal.append("entdb-wal", "tenant_1", b'{"op": "create"}')
    """

    def __init__(self, config: Any) -> None:
        """Initialize Pub/Sub WAL stream.

        Args:
            config: PubSubConfig instance with connection settings

        Raises:
            ImportError: If google-cloud-pubsub is not installed
        """
        if not PUBSUB_AVAILABLE:
            raise ImportError(
                "google-cloud-pubsub is required for Pub/Sub backend. "
                "Install with: pip install google-cloud-pubsub"
            )

        self.config = config
        self._publisher: PublisherClient | None = None
        self._subscriber: SubscriberClient | None = None
        self._connected = False
        self._topic_path: str | None = None
        self._subscription_path: str | None = None
        # Map of ack_id -> message for pending acknowledgments
        self._pending_acks: dict[str, str] = {}

    @property
    def is_connected(self) -> bool:
        """Whether connected to Pub/Sub."""
        return self._connected and self._publisher is not None

    async def connect(self) -> None:
        """Connect to Google Cloud Pub/Sub.

        Creates publisher and subscriber clients.

        Raises:
            WalConnectionError: If connection fails
        """
        if self._connected:
            return

        try:
            # Build client options
            client_options = {}
            if self.config.endpoint:
                client_options["api_endpoint"] = self.config.endpoint

            # Create publisher with ordering enabled
            publisher_options = {}
            if self.config.ordering_enabled:
                from google.cloud.pubsub_v1.types import PublisherOptions as PubOptions

                publisher_options["publisher_options"] = PubOptions(
                    enable_message_ordering=True,
                )

            if client_options:
                from google.api_core.client_options import ClientOptions

                opts = ClientOptions(**client_options)
                self._publisher = PublisherClient(client_options=opts, **publisher_options)
                self._subscriber = SubscriberClient(client_options=opts)
            else:
                self._publisher = PublisherClient(**publisher_options)
                self._subscriber = SubscriberClient()

            # Build resource paths
            self._topic_path = self._publisher.topic_path(
                self.config.project_id, self.config.topic_id
            )
            self._subscription_path = self._subscriber.subscription_path(
                self.config.project_id, self.config.subscription_id
            )

            self._connected = True

            logger.info(
                "Connected to Pub/Sub",
                extra={
                    "project": self.config.project_id,
                    "topic": self.config.topic_id,
                    "subscription": self.config.subscription_id,
                    "endpoint": self.config.endpoint or "GCP",
                },
            )

        except Exception as e:
            self._connected = False
            raise WalConnectionError(f"Failed to connect to Pub/Sub: {e}") from e

    async def close(self) -> None:
        """Close Pub/Sub connections.

        Shuts down publisher and subscriber clients.
        """
        if self._publisher:
            try:
                self._publisher.stop()
            except Exception as e:
                logger.warning(f"Error closing publisher: {e}")
            self._publisher = None

        if self._subscriber:
            try:
                self._subscriber.close()
            except Exception as e:
                logger.warning(f"Error closing subscriber: {e}")
            self._subscriber = None

        self._connected = False
        self._pending_acks.clear()
        logger.info("Pub/Sub connections closed")

    async def append(
        self,
        topic: str,
        key: str,
        value: bytes,
        headers: dict[str, bytes] | None = None,
    ) -> StreamPos:
        """Append event to Pub/Sub topic.

        Publishes a message with ordering key for per-tenant ordering.

        Args:
            topic: Topic ID (used to build full topic path)
            key: Ordering key (tenant_id)
            value: Event payload
            headers: Optional message attributes

        Returns:
            StreamPos with topic, partition=0, offset=publish_time

        Raises:
            WalConnectionError: If not connected
            WalTimeoutError: If publish times out
            WalError: For other Pub/Sub errors
        """
        if not self._publisher:
            raise WalConnectionError("Not connected to Pub/Sub")

        try:
            # Build topic path (use configured topic if topic matches topic_id)
            topic_path = self._topic_path
            if topic != self.config.topic_id:
                topic_path = self._publisher.topic_path(self.config.project_id, topic)

            # Convert headers to string attributes (Pub/Sub attributes are string->string)
            attributes = {}
            if headers:
                attributes = {k: v.decode("utf-8", errors="replace") for k, v in headers.items()}

            # Build publish kwargs
            publish_kwargs: dict[str, Any] = {
                "data": value,
            }
            if attributes:
                publish_kwargs.update(attributes)

            if self.config.ordering_enabled:
                publish_kwargs["ordering_key"] = key

            # Publish and wait for acknowledgment
            loop = asyncio.get_event_loop()
            future = self._publisher.publish(topic_path, **publish_kwargs)

            # Wait for the future to resolve (with timeout)
            message_id = await asyncio.wait_for(
                loop.run_in_executor(None, future.result),
                timeout=30.0,
            )

            now_ms = int(time.time() * 1000)

            pos = StreamPos(
                topic=topic,
                partition=0,  # Pub/Sub has no partitions
                offset=now_ms,  # Use publish time as offset
                timestamp_ms=now_ms,
            )

            logger.debug(
                "Event appended to Pub/Sub",
                extra={
                    "topic": topic,
                    "key": key,
                    "message_id": message_id,
                },
            )

            return pos

        except asyncio.TimeoutError:
            raise WalTimeoutError("Pub/Sub publish timed out")
        except NotFound as e:
            raise WalConnectionError(f"Pub/Sub topic not found: {e}") from e
        except GoogleAPICallError as e:
            raise WalError(f"Pub/Sub publish failed: {e}") from e

    async def subscribe(
        self,
        topic: str,
        group_id: str,
        start_position: StreamPos | None = None,
    ) -> AsyncIterator[StreamRecord]:
        """Subscribe to Pub/Sub topic.

        Uses synchronous pull wrapped in asyncio executor to consume messages.

        Args:
            topic: Topic ID
            group_id: Subscription ID (used as consumer group equivalent)
            start_position: Optional starting position (limited support in Pub/Sub)

        Yields:
            StreamRecord for each message

        Raises:
            WalConnectionError: If subscription fails
        """
        if not self._subscriber:
            raise WalConnectionError("Not connected to Pub/Sub")

        try:
            # Build subscription path
            subscription_path = self._subscriber.subscription_path(self.config.project_id, group_id)

            logger.info(
                "Subscribed to Pub/Sub",
                extra={"topic": topic, "subscription": group_id},
            )

            loop = asyncio.get_event_loop()

            # Poll for messages continuously
            while True:
                try:
                    pull_request = PullRequest(
                        subscription=subscription_path,
                        max_messages=self.config.max_messages,
                    )

                    def _pull(req: PullRequest = pull_request) -> Any:
                        return self._subscriber.pull(
                            request=req,
                            timeout=self.config.ack_deadline_seconds,
                        )

                    response = await loop.run_in_executor(None, _pull)

                    for received_message in response.received_messages:
                        msg = received_message.message
                        ack_id = received_message.ack_id

                        # Convert attributes to headers
                        headers = {k: v.encode("utf-8") for k, v in msg.attributes.items()}

                        # Extract ordering key as record key
                        record_key = msg.ordering_key or ""

                        # Use publish_time for timestamp
                        publish_time_ms = int(msg.publish_time.timestamp() * 1000)

                        record = StreamRecord(
                            key=record_key,
                            value=msg.data,
                            position=StreamPos(
                                topic=topic,
                                partition=0,
                                offset=publish_time_ms,
                                timestamp_ms=publish_time_ms,
                            ),
                            headers=headers,
                        )

                        # Store ack_id for later acknowledgment via commit()
                        self._pending_acks[str(publish_time_ms)] = ack_id

                        yield record

                except DeadlineExceeded:
                    # No messages available within timeout, continue polling
                    pass

                # Small delay to avoid tight polling when no messages
                await asyncio.sleep(0.2)

        except NotFound as e:
            raise WalConnectionError(f"Pub/Sub subscription not found: {e}") from e
        except GoogleAPICallError as e:
            raise WalError(f"Pub/Sub subscription error: {e}") from e

    async def poll_batch(
        self,
        topic: str,
        group_id: str,
        max_records: int = 20,
        timeout_ms: int = 100,
        start_position: StreamPos | None = None,
    ) -> list[StreamRecord]:
        """Poll for a batch of records from Pub/Sub.

        Uses a single synchronous pull call (via executor) instead of
        delegating to subscribe(), which loops forever and would cause
        poll_batch to hang when fewer than max_records are available.

        Args:
            topic: Topic ID
            group_id: Subscription ID
            max_records: Maximum messages to pull
            timeout_ms: Timeout in milliseconds
            start_position: Not supported

        Returns:
            List of StreamRecord objects (may be empty)
        """
        if not self._subscriber:
            raise WalConnectionError("Not connected to Pub/Sub")

        records: list[StreamRecord] = []
        try:
            subscription_path = self._subscriber.subscription_path(self.config.project_id, group_id)
            timeout_seconds = max(1, timeout_ms // 1000)

            pull_request = PullRequest(
                subscription=subscription_path,
                max_messages=max_records,
            )

            loop = asyncio.get_event_loop()

            def _pull():
                return self._subscriber.pull(
                    request=pull_request,
                    timeout=timeout_seconds,
                )

            response = await loop.run_in_executor(None, _pull)

            for received_message in response.received_messages:
                msg = received_message.message
                ack_id = received_message.ack_id

                headers = {k: v.encode("utf-8") for k, v in msg.attributes.items()}
                record_key = msg.ordering_key or ""
                publish_time_ms = int(msg.publish_time.timestamp() * 1000)

                record = StreamRecord(
                    key=record_key,
                    value=msg.data,
                    position=StreamPos(
                        topic=topic,
                        partition=0,
                        offset=publish_time_ms,
                        timestamp_ms=publish_time_ms,
                    ),
                    headers=headers,
                )

                self._pending_acks[str(publish_time_ms)] = ack_id
                records.append(record)

        except DeadlineExceeded:
            # No messages available within timeout
            pass
        except NotFound as e:
            raise WalConnectionError(f"Pub/Sub subscription not found: {e}") from e
        except GoogleAPICallError as e:
            raise WalError(f"Pub/Sub poll error: {e}") from e

        return records

    async def commit(self, record: StreamRecord) -> None:
        """Acknowledge a consumed message.

        Args:
            record: The record to acknowledge

        Raises:
            WalError: If acknowledgment fails
        """
        if not self._subscriber:
            raise WalError("No active subscriber to commit")

        try:
            # Look up ack_id from pending acks
            ack_key = str(record.position.offset)
            ack_id = self._pending_acks.pop(ack_key, None)

            if not ack_id:
                logger.warning(
                    "No pending ack found for record",
                    extra={"offset": record.position.offset},
                )
                return

            # Build subscription path
            subscription_path = self._subscription_path

            # Acknowledge the message
            loop = asyncio.get_event_loop()
            await loop.run_in_executor(
                None,
                lambda: self._subscriber.acknowledge(
                    request={
                        "subscription": subscription_path,
                        "ack_ids": [ack_id],
                    }
                ),
            )

            logger.debug(
                "Acknowledged message",
                extra={
                    "topic": record.position.topic,
                    "offset": record.position.offset,
                },
            )

        except GoogleAPICallError as e:
            raise WalError(f"Failed to acknowledge: {e}") from e

    async def get_positions(self, topic: str, group_id: str) -> dict[int, StreamPos]:
        """Get committed positions.

        Pub/Sub does not expose consumer offsets like Kafka. Returns empty
        dict since Pub/Sub manages delivery state internally via ack/nack.

        Returns:
            Empty dictionary (Pub/Sub manages positions internally)
        """
        return {}

    async def health_check(self) -> bool:
        """Check if Pub/Sub connection is healthy.

        Returns:
            True if connected and topic exists
        """
        if not self._publisher or not self._topic_path:
            return False

        try:
            loop = asyncio.get_event_loop()
            await asyncio.wait_for(
                loop.run_in_executor(
                    None,
                    lambda: self._publisher.get_topic(request={"topic": self._topic_path}),
                ),
                timeout=5.0,
            )
            return True
        except Exception:
            return False
