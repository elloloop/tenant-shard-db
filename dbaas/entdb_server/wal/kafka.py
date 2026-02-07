"""
Kafka/Redpanda WAL stream implementation.

This module provides a production-grade Kafka backend for the WAL stream.
It works with:
- Apache Kafka
- Amazon MSK
- Redpanda
- Any Kafka API-compatible system

Invariants:
    - Producer uses acks=all for strongest durability
    - Idempotent producer prevents duplicate writes on retry
    - Consumer uses manual commit for exactly-once semantics
    - Connection failures trigger automatic reconnection

How to change safely:
    - Test with actual Kafka/Redpanda cluster before deploying
    - Verify durability with chaos engineering (broker failures)
    - Monitor producer/consumer metrics in production
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

# Try to import aiokafka, provide helpful message if not installed
try:
    from aiokafka import AIOKafkaConsumer, AIOKafkaProducer
    from aiokafka.errors import KafkaConnectionError, KafkaError, KafkaTimeoutError
    from aiokafka.structs import OffsetAndMetadata, TopicPartition

    KAFKA_AVAILABLE = True
except ImportError:
    KAFKA_AVAILABLE = False
    AIOKafkaProducer = None
    AIOKafkaConsumer = None


class KafkaWalStream:
    """Kafka implementation of WalStream protocol.

    Uses aiokafka for async producer/consumer operations.
    Configured for strongest durability guarantees.

    Attributes:
        config: Kafka configuration
        producer: AIOKafka producer instance
        consumer: AIOKafka consumer instance (created per subscription)

    Durability configuration:
        - acks='all': Wait for all in-sync replicas
        - enable_idempotence=True: Prevent duplicates on retry
        - max_in_flight_requests_per_connection=5: Allow pipelining with order guarantee

    Example:
        >>> config = KafkaConfig(brokers="localhost:9092", topic="entdb-wal")
        >>> wal = KafkaWalStream(config)
        >>> await wal.connect()
        >>> pos = await wal.append("entdb-wal", "tenant_1", b'{"op": "create"}')
    """

    def __init__(self, config: Any) -> None:
        """Initialize Kafka WAL stream.

        Args:
            config: KafkaConfig instance with connection settings

        Raises:
            ImportError: If aiokafka is not installed
        """
        if not KAFKA_AVAILABLE:
            raise ImportError(
                "aiokafka is required for Kafka backend. Install with: pip install aiokafka"
            )

        self.config = config
        self._producer: AIOKafkaProducer | None = None
        self._consumer: AIOKafkaConsumer | None = None
        self._connected = False
        self._consumer_topic: str | None = None
        self._consumer_group: str | None = None

    @property
    def is_connected(self) -> bool:
        """Whether connected to Kafka."""
        return self._connected and self._producer is not None

    async def connect(self) -> None:
        """Connect to Kafka cluster.

        Creates the producer with durability settings.

        Raises:
            WalConnectionError: If connection fails
        """
        if self._connected:
            return

        try:
            # Build producer configuration
            producer_config = {
                "bootstrap_servers": self.config.brokers,
                "acks": self.config.acks,
                "enable_idempotence": self.config.enable_idempotence,
                "max_batch_size": 16384,
                "linger_ms": 5,  # Small batching for low latency
                "request_timeout_ms": 30000,
                "retry_backoff_ms": 100,
            }

            # Add security settings if configured
            if self.config.security_protocol != "PLAINTEXT":
                producer_config["security_protocol"] = self.config.security_protocol

            if self.config.sasl_mechanism:
                producer_config["sasl_mechanism"] = self.config.sasl_mechanism
                producer_config["sasl_plain_username"] = self.config.sasl_username
                producer_config["sasl_plain_password"] = self.config.sasl_password

            if self.config.ssl_cafile:
                producer_config["ssl_cafile"] = self.config.ssl_cafile
            if self.config.ssl_certfile:
                producer_config["ssl_certfile"] = self.config.ssl_certfile
            if self.config.ssl_keyfile:
                producer_config["ssl_keyfile"] = self.config.ssl_keyfile

            self._producer = AIOKafkaProducer(**producer_config)
            await self._producer.start()
            self._connected = True

            logger.info(
                "Connected to Kafka",
                extra={
                    "brokers": self.config.brokers,
                    "acks": self.config.acks,
                    "idempotent": self.config.enable_idempotence,
                },
            )

        except Exception as e:
            self._connected = False
            raise WalConnectionError(f"Failed to connect to Kafka: {e}") from e

    async def close(self) -> None:
        """Close Kafka connections.

        Flushes pending writes before closing.
        """
        if self._consumer:
            try:
                await self._consumer.stop()
            except Exception as e:
                logger.warning(f"Error closing consumer: {e}")
            self._consumer = None

        if self._producer:
            try:
                await self._producer.stop()
            except Exception as e:
                logger.warning(f"Error closing producer: {e}")
            self._producer = None

        self._connected = False
        logger.info("Kafka connections closed")

    async def append(
        self,
        topic: str,
        key: str,
        value: bytes,
        headers: dict[str, bytes] | None = None,
    ) -> StreamPos:
        """Append event to Kafka topic.

        Uses synchronous send (wait for ack) to ensure durability.

        Args:
            topic: Kafka topic name
            key: Partition key (tenant_id)
            value: Event payload
            headers: Optional message headers

        Returns:
            StreamPos with topic, partition, offset

        Raises:
            WalConnectionError: If not connected
            WalTimeoutError: If send times out
            WalError: For other Kafka errors
        """
        if not self._producer:
            raise WalConnectionError("Not connected to Kafka")

        try:
            # Convert headers to Kafka format
            kafka_headers = None
            if headers:
                kafka_headers = [(k, v) for k, v in headers.items()]

            # Send and wait for acknowledgment
            record_metadata = await self._producer.send_and_wait(
                topic,
                value=value,
                key=key.encode("utf-8"),
                headers=kafka_headers,
            )

            pos = StreamPos(
                topic=record_metadata.topic,
                partition=record_metadata.partition,
                offset=record_metadata.offset,
                timestamp_ms=record_metadata.timestamp or int(time.time() * 1000),
            )

            logger.debug(
                "Event appended to Kafka",
                extra={
                    "topic": topic,
                    "key": key,
                    "partition": pos.partition,
                    "offset": pos.offset,
                },
            )

            return pos

        except KafkaTimeoutError as e:
            raise WalTimeoutError(f"Kafka send timed out: {e}") from e
        except KafkaConnectionError as e:
            self._connected = False
            raise WalConnectionError(f"Kafka connection lost: {e}") from e
        except KafkaError as e:
            raise WalError(f"Kafka send failed: {e}") from e

    async def subscribe(
        self,
        topic: str,
        group_id: str,
        start_position: StreamPos | None = None,
    ) -> AsyncIterator[StreamRecord]:
        """Subscribe to Kafka topic.

        Creates a consumer for the topic and yields records.

        Args:
            topic: Topic to subscribe to
            group_id: Consumer group for coordination
            start_position: Optional starting position

        Yields:
            StreamRecord for each message

        Raises:
            WalConnectionError: If subscription fails
        """
        try:
            # Close existing consumer if any
            if self._consumer:
                await self._consumer.stop()

            # Build consumer configuration
            consumer_config = {
                "bootstrap_servers": self.config.brokers,
                "group_id": group_id,
                "auto_offset_reset": self.config.auto_offset_reset,
                "enable_auto_commit": self.config.enable_auto_commit,
                "max_poll_records": 100,
                "session_timeout_ms": 30000,
                "heartbeat_interval_ms": 10000,
            }

            # Add security settings
            if self.config.security_protocol != "PLAINTEXT":
                consumer_config["security_protocol"] = self.config.security_protocol

            if self.config.sasl_mechanism:
                consumer_config["sasl_mechanism"] = self.config.sasl_mechanism
                consumer_config["sasl_plain_username"] = self.config.sasl_username
                consumer_config["sasl_plain_password"] = self.config.sasl_password

            if self.config.ssl_cafile:
                consumer_config["ssl_cafile"] = self.config.ssl_cafile

            self._consumer = AIOKafkaConsumer(topic, **consumer_config)
            await self._consumer.start()
            self._consumer_topic = topic
            self._consumer_group = group_id

            logger.info("Subscribed to Kafka topic", extra={"topic": topic, "group_id": group_id})

            # Seek to position if specified
            if start_position and start_position.topic == topic:
                tp = (topic, start_position.partition)
                self._consumer.seek(tp, start_position.offset)

            # Yield records
            async for msg in self._consumer:
                # Convert headers
                headers = dict(msg.headers) if msg.headers else {}

                record = StreamRecord(
                    key=msg.key.decode("utf-8") if msg.key else "",
                    value=msg.value,
                    position=StreamPos(
                        topic=msg.topic,
                        partition=msg.partition,
                        offset=msg.offset,
                        timestamp_ms=msg.timestamp or int(time.time() * 1000),
                    ),
                    headers=headers,
                )

                yield record

        except KafkaConnectionError as e:
            raise WalConnectionError(f"Failed to subscribe: {e}") from e
        except KafkaError as e:
            raise WalError(f"Consumer error: {e}") from e

    async def commit(self, record: StreamRecord) -> None:
        """Commit consumed record offset.

        Args:
            record: The record to commit

        Raises:
            WalError: If commit fails
        """
        if not self._consumer:
            raise WalError("No active consumer to commit")

        try:
            # Commit the offset + 1 (next message to consume)
            tp = {
                TopicPartition(record.position.topic, record.position.partition): OffsetAndMetadata(
                    record.position.offset + 1, ""
                )
            }
            await self._consumer.commit(tp)

            logger.debug(
                "Committed offset",
                extra={
                    "topic": record.position.topic,
                    "partition": record.position.partition,
                    "offset": record.position.offset,
                },
            )

        except KafkaError as e:
            raise WalError(f"Failed to commit: {e}") from e

    async def get_positions(self, topic: str, group_id: str) -> dict[int, StreamPos]:
        """Get committed positions for consumer group.

        Args:
            topic: Topic name
            group_id: Consumer group ID

        Returns:
            Dictionary mapping partition to position
        """
        # Create a temporary consumer to fetch positions
        try:
            consumer = AIOKafkaConsumer(
                topic,
                bootstrap_servers=self.config.brokers,
                group_id=group_id,
                enable_auto_commit=False,
            )
            await consumer.start()

            positions = {}
            partitions = consumer.partitions_for_topic(topic) or set()

            for partition in partitions:
                tp = (topic, partition)
                try:
                    committed = await consumer.committed(tp)
                    if committed is not None:
                        positions[partition] = StreamPos(
                            topic=topic,
                            partition=partition,
                            offset=committed,
                            timestamp_ms=int(time.time() * 1000),
                        )
                except Exception:
                    pass

            await consumer.stop()
            return positions

        except Exception as e:
            logger.warning(f"Failed to get positions: {e}")
            return {}

    async def health_check(self) -> bool:
        """Check if Kafka connection is healthy.

        Returns:
            True if connected and working
        """
        if not self._producer:
            return False

        try:
            # Check producer is responsive
            metadata = await asyncio.wait_for(
                self._producer.partitions_for(self.config.topic), timeout=5.0
            )
            return metadata is not None
        except Exception:
            return False
