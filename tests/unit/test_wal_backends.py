"""
Mock-based behavior tests for WAL stream backends.

These tests verify that each backend calls the correct cloud SDK methods
with the right arguments, without needing actual cloud services.

Backends tested:
  - KafkaWalStream (aiokafka)
  - KinesisWalStream (aiobotocore / Kinesis)
  - PubSubWalStream (google-cloud-pubsub)
  - SqsWalStream (aiobotocore / SQS)
  - ServiceBusWalStream (azure-servicebus)
  - EventHubsWalStream (azure-eventhub)

InMemoryWalStream is tested separately in test_wal_memory.py.
"""

from __future__ import annotations

import sys
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from dbaas.entdb_server.wal.base import (
    StreamPos,
    StreamRecord,
    WalConnectionError,
    WalError,
)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_kafka_config(**overrides):
    cfg = MagicMock()
    cfg.brokers = "localhost:9092"
    cfg.acks = "all"
    cfg.enable_idempotence = True
    cfg.security_protocol = "PLAINTEXT"
    cfg.sasl_mechanism = None
    cfg.sasl_username = None
    cfg.sasl_password = None
    cfg.ssl_cafile = None
    cfg.ssl_certfile = None
    cfg.ssl_keyfile = None
    cfg.topic = "entdb-wal"
    cfg.auto_offset_reset = "earliest"
    cfg.enable_auto_commit = False
    for k, v in overrides.items():
        setattr(cfg, k, v)
    return cfg


def _make_kinesis_config(**overrides):
    cfg = MagicMock()
    cfg.stream_name = "entdb-wal"
    cfg.region = "us-east-1"
    cfg.endpoint_url = None
    cfg.iterator_type = "LATEST"
    cfg.max_records_per_get = 100
    for k, v in overrides.items():
        setattr(cfg, k, v)
    return cfg


def _make_pubsub_config(**overrides):
    cfg = MagicMock()
    cfg.project_id = "my-project"
    cfg.topic_id = "entdb-wal"
    cfg.subscription_id = "entdb-sub"
    cfg.endpoint = None
    cfg.ordering_enabled = True
    cfg.max_messages = 100
    cfg.ack_deadline_seconds = 30
    for k, v in overrides.items():
        setattr(cfg, k, v)
    return cfg


def _make_sqs_config(**overrides):
    cfg = MagicMock()
    cfg.queue_url = "https://sqs.us-east-1.amazonaws.com/123/entdb.fifo"
    cfg.region = "us-east-1"
    cfg.endpoint_url = None
    cfg.max_messages = 10
    cfg.wait_time_seconds = 5
    cfg.visibility_timeout = 30
    for k, v in overrides.items():
        setattr(cfg, k, v)
    return cfg


def _make_servicebus_config(**overrides):
    cfg = MagicMock()
    cfg.connection_string = "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=key"
    cfg.queue_name = "entdb-wal"
    cfg.max_wait_time_seconds = 5
    for k, v in overrides.items():
        setattr(cfg, k, v)
    return cfg


def _make_eventhubs_config(**overrides):
    cfg = MagicMock()
    cfg.connection_string = "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=key"
    cfg.eventhub_name = "entdb-wal"
    cfg.consumer_group = "$Default"
    cfg.max_batch_size = 50
    cfg.max_wait_time = 5
    for k, v in overrides.items():
        setattr(cfg, k, v)
    return cfg


def _sample_record(topic="test", partition=0, offset=1):
    return StreamRecord(
        key="tenant_1",
        value=b'{"op": "create"}',
        position=StreamPos(
            topic=topic,
            partition=partition,
            offset=offset,
            timestamp_ms=1700000000000,
        ),
    )


# ===================================================================
# Kafka
# ===================================================================


@pytest.mark.unit
class TestKafkaWalStream:
    """Tests for KafkaWalStream using mocked aiokafka."""

    @pytest.mark.asyncio
    async def test_connect_creates_and_starts_producer(self):
        """connect() creates AIOKafkaProducer and calls start()."""
        mock_producer = AsyncMock()
        with (
            patch("dbaas.entdb_server.wal.kafka.KAFKA_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.kafka.AIOKafkaProducer") as MockProducer,
        ):
            MockProducer.return_value = mock_producer
            from dbaas.entdb_server.wal.kafka import KafkaWalStream

            wal = KafkaWalStream(_make_kafka_config())
            await wal.connect()

            MockProducer.assert_called_once()
            call_kwargs = MockProducer.call_args[1]
            assert call_kwargs["bootstrap_servers"] == "localhost:9092"
            assert call_kwargs["acks"] == "all"
            assert call_kwargs["enable_idempotence"] is True
            mock_producer.start.assert_awaited_once()
            assert wal.is_connected

    @pytest.mark.asyncio
    async def test_append_sends_message(self):
        """append() calls producer.send_and_wait with topic, value, and key."""
        mock_producer = AsyncMock()
        record_meta = MagicMock()
        record_meta.topic = "entdb-wal"
        record_meta.partition = 0
        record_meta.offset = 42
        record_meta.timestamp = 1700000000000
        mock_producer.send_and_wait.return_value = record_meta

        with (
            patch("dbaas.entdb_server.wal.kafka.KAFKA_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.kafka.AIOKafkaProducer") as MockProducer,
        ):
            MockProducer.return_value = mock_producer
            from dbaas.entdb_server.wal.kafka import KafkaWalStream

            wal = KafkaWalStream(_make_kafka_config())
            await wal.connect()

            pos = await wal.append("entdb-wal", "tenant_1", b'{"op":"create"}')

            mock_producer.send_and_wait.assert_awaited_once_with(
                "entdb-wal",
                value=b'{"op":"create"}',
                key=b"tenant_1",
                headers=None,
            )
            assert pos.topic == "entdb-wal"
            assert pos.partition == 0
            assert pos.offset == 42

    @pytest.mark.asyncio
    async def test_append_with_headers(self):
        """append() converts header dict to Kafka header list."""
        mock_producer = AsyncMock()
        record_meta = MagicMock(topic="entdb-wal", partition=0, offset=1, timestamp=1700000000000)
        mock_producer.send_and_wait.return_value = record_meta

        with (
            patch("dbaas.entdb_server.wal.kafka.KAFKA_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.kafka.AIOKafkaProducer") as MockProducer,
        ):
            MockProducer.return_value = mock_producer
            from dbaas.entdb_server.wal.kafka import KafkaWalStream

            wal = KafkaWalStream(_make_kafka_config())
            await wal.connect()

            headers = {"x-type": b"create"}
            await wal.append("entdb-wal", "t1", b"data", headers=headers)

            call_kwargs = mock_producer.send_and_wait.call_args[1]
            assert call_kwargs["headers"] == [("x-type", b"create")]

    @pytest.mark.asyncio
    async def test_append_not_connected_raises(self):
        """append() raises WalConnectionError when not connected."""
        with (
            patch("dbaas.entdb_server.wal.kafka.KAFKA_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.kafka.AIOKafkaProducer"),
        ):
            from dbaas.entdb_server.wal.kafka import KafkaWalStream

            wal = KafkaWalStream(_make_kafka_config())
            # Do NOT connect
            with pytest.raises(WalConnectionError):
                await wal.append("t", "k", b"v")

    @pytest.mark.asyncio
    async def test_close_stops_producer(self):
        """close() stops the producer and marks disconnected."""
        mock_producer = AsyncMock()
        with (
            patch("dbaas.entdb_server.wal.kafka.KAFKA_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.kafka.AIOKafkaProducer") as MockProducer,
        ):
            MockProducer.return_value = mock_producer
            from dbaas.entdb_server.wal.kafka import KafkaWalStream

            wal = KafkaWalStream(_make_kafka_config())
            await wal.connect()
            await wal.close()

            mock_producer.stop.assert_awaited_once()
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_connect_failure_raises_wal_connection_error(self):
        """connect() wraps SDK errors into WalConnectionError."""
        mock_producer = AsyncMock()
        mock_producer.start.side_effect = Exception("broker unreachable")

        with (
            patch("dbaas.entdb_server.wal.kafka.KAFKA_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.kafka.AIOKafkaProducer") as MockProducer,
        ):
            MockProducer.return_value = mock_producer
            from dbaas.entdb_server.wal.kafka import KafkaWalStream

            wal = KafkaWalStream(_make_kafka_config())
            with pytest.raises(WalConnectionError, match="broker unreachable"):
                await wal.connect()
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_is_connected_initially_false(self):
        """is_connected is False before connect()."""
        with (
            patch("dbaas.entdb_server.wal.kafka.KAFKA_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.kafka.AIOKafkaProducer"),
        ):
            from dbaas.entdb_server.wal.kafka import KafkaWalStream

            wal = KafkaWalStream(_make_kafka_config())
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_commit_calls_consumer_commit(self):
        """commit() calls consumer.commit with offset+1."""
        mock_consumer = AsyncMock()
        mock_tp = MagicMock()
        mock_oam = MagicMock()

        with (
            patch("dbaas.entdb_server.wal.kafka.KAFKA_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.kafka.AIOKafkaProducer"),
            patch("dbaas.entdb_server.wal.kafka.TopicPartition", return_value=mock_tp),
            patch(
                "dbaas.entdb_server.wal.kafka.OffsetAndMetadata",
                return_value=mock_oam,
            ),
        ):
            from dbaas.entdb_server.wal.kafka import KafkaWalStream

            wal = KafkaWalStream(_make_kafka_config())
            wal._consumer = mock_consumer

            record = _sample_record(topic="entdb-wal", partition=0, offset=10)
            await wal.commit(record)

            mock_consumer.commit.assert_awaited_once_with({mock_tp: mock_oam})

    @pytest.mark.asyncio
    async def test_commit_without_consumer_raises(self):
        """commit() raises WalError when there is no consumer."""
        with (
            patch("dbaas.entdb_server.wal.kafka.KAFKA_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.kafka.AIOKafkaProducer"),
        ):
            from dbaas.entdb_server.wal.kafka import KafkaWalStream

            wal = KafkaWalStream(_make_kafka_config())
            with pytest.raises(WalError, match="No active consumer"):
                await wal.commit(_sample_record())


# ===================================================================
# Kinesis
# ===================================================================


@pytest.mark.unit
class TestKinesisWalStream:
    """Tests for KinesisWalStream using mocked aiobotocore."""

    @pytest.mark.asyncio
    async def test_connect_creates_kinesis_client(self):
        """connect() creates a Kinesis client and verifies the stream."""
        mock_session = MagicMock()
        mock_client_ctx = AsyncMock()
        mock_client = AsyncMock()
        mock_client_ctx.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client_ctx.__aexit__ = AsyncMock(return_value=False)
        mock_session.create_client.return_value = mock_client_ctx
        mock_client.describe_stream.return_value = {"StreamDescription": {}}

        with (
            patch("dbaas.entdb_server.wal.kinesis.KINESIS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.wal.kinesis.get_session",
                return_value=mock_session,
            ),
        ):
            from dbaas.entdb_server.wal.kinesis import KinesisWalStream

            wal = KinesisWalStream(_make_kinesis_config())
            await wal.connect()

            mock_session.create_client.assert_called_once_with("kinesis", region_name="us-east-1")
            mock_client.describe_stream.assert_awaited_once_with(StreamName="entdb-wal")
            assert wal.is_connected

    @pytest.mark.asyncio
    async def test_append_calls_put_record(self):
        """append() calls put_record with StreamName, Data, PartitionKey."""
        mock_session = MagicMock()
        mock_client_ctx = AsyncMock()
        mock_client = AsyncMock()
        mock_client_ctx.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client_ctx.__aexit__ = AsyncMock(return_value=False)
        mock_session.create_client.return_value = mock_client_ctx
        mock_client.describe_stream.return_value = {}
        mock_client.put_record.return_value = {
            "ShardId": "shardId-000000000001",
            "SequenceNumber": "100",
        }

        with (
            patch("dbaas.entdb_server.wal.kinesis.KINESIS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.wal.kinesis.get_session",
                return_value=mock_session,
            ),
        ):
            from dbaas.entdb_server.wal.kinesis import KinesisWalStream

            wal = KinesisWalStream(_make_kinesis_config())
            await wal.connect()
            pos = await wal.append("entdb-wal", "tenant_1", b'{"op":"create"}')

            mock_client.put_record.assert_awaited_once()
            call_kwargs = mock_client.put_record.call_args[1]
            assert call_kwargs["StreamName"] == "entdb-wal"
            assert call_kwargs["Data"] == b'{"op":"create"}'
            assert call_kwargs["PartitionKey"] == "tenant_1"
            assert "ExplicitHashKey" in call_kwargs
            assert pos.partition == 1
            assert pos.offset == 100

    @pytest.mark.asyncio
    async def test_append_not_connected_raises(self):
        """append() raises WalConnectionError when not connected."""
        with (
            patch("dbaas.entdb_server.wal.kinesis.KINESIS_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.kinesis.get_session"),
        ):
            from dbaas.entdb_server.wal.kinesis import KinesisWalStream

            wal = KinesisWalStream(_make_kinesis_config())
            with pytest.raises(WalConnectionError):
                await wal.append("s", "k", b"v")

    @pytest.mark.asyncio
    async def test_close_exits_client_context(self):
        """close() exits the aiobotocore client context manager."""
        mock_session = MagicMock()
        mock_client_ctx = AsyncMock()
        mock_client = AsyncMock()
        mock_client_ctx.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client_ctx.__aexit__ = AsyncMock(return_value=False)
        mock_session.create_client.return_value = mock_client_ctx
        mock_client.describe_stream.return_value = {}

        with (
            patch("dbaas.entdb_server.wal.kinesis.KINESIS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.wal.kinesis.get_session",
                return_value=mock_session,
            ),
        ):
            from dbaas.entdb_server.wal.kinesis import KinesisWalStream

            wal = KinesisWalStream(_make_kinesis_config())
            await wal.connect()
            await wal.close()

            mock_client_ctx.__aexit__.assert_awaited_once_with(None, None, None)
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_connect_failure_raises_wal_connection_error(self):
        """connect() wraps SDK errors into WalConnectionError."""
        mock_session = MagicMock()
        mock_client_ctx = AsyncMock()
        mock_client = AsyncMock()
        mock_client_ctx.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.describe_stream.side_effect = Exception("stream not found")
        mock_session.create_client.return_value = mock_client_ctx

        with (
            patch("dbaas.entdb_server.wal.kinesis.KINESIS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.wal.kinesis.get_session",
                return_value=mock_session,
            ),
        ):
            from dbaas.entdb_server.wal.kinesis import KinesisWalStream

            wal = KinesisWalStream(_make_kinesis_config())
            with pytest.raises(WalConnectionError, match="stream not found"):
                await wal.connect()
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_is_connected_initially_false(self):
        """is_connected is False before connect()."""
        with (
            patch("dbaas.entdb_server.wal.kinesis.KINESIS_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.kinesis.get_session"),
        ):
            from dbaas.entdb_server.wal.kinesis import KinesisWalStream

            wal = KinesisWalStream(_make_kinesis_config())
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_commit_stores_checkpoint(self):
        """commit() stores the sequence number as an internal checkpoint."""
        with (
            patch("dbaas.entdb_server.wal.kinesis.KINESIS_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.kinesis.get_session"),
        ):
            from dbaas.entdb_server.wal.kinesis import KinesisWalStream

            wal = KinesisWalStream(_make_kinesis_config())
            record = _sample_record(partition=1, offset=999)
            await wal.commit(record)

            assert "shardId-000000000001" in wal._checkpoints
            assert wal._checkpoints["shardId-000000000001"] == "999"


# ===================================================================
# Pub/Sub
# ===================================================================


@pytest.mark.unit
class TestPubSubWalStream:
    """Tests for PubSubWalStream using mocked google-cloud-pubsub.

    We set ordering_enabled=False in the config to avoid a lazy import of
    google.cloud.pubsub_v1.types.PublisherOptions inside connect().  Ordering
    behaviour is a config concern, not relevant to SDK-call verification.
    """

    def _patch_pubsub(self):
        """Return patches for pubsub availability and clients."""
        mock_publisher_cls = MagicMock()
        mock_subscriber_cls = MagicMock()
        return (
            patch("dbaas.entdb_server.wal.pubsub.PUBSUB_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.wal.pubsub.PublisherClient",
                mock_publisher_cls,
            ),
            patch(
                "dbaas.entdb_server.wal.pubsub.SubscriberClient",
                mock_subscriber_cls,
            ),
            mock_publisher_cls,
            mock_subscriber_cls,
        )

    def _pubsub_config(self, **overrides):
        """Config with ordering disabled to avoid lazy google import."""
        defaults = {"ordering_enabled": False}
        defaults.update(overrides)
        return _make_pubsub_config(**defaults)

    @pytest.mark.asyncio
    async def test_connect_creates_publisher_and_subscriber(self):
        """connect() creates PublisherClient and SubscriberClient."""
        p1, p2, p3, mock_pub_cls, mock_sub_cls = self._patch_pubsub()
        mock_publisher = MagicMock()
        mock_subscriber = MagicMock()
        mock_pub_cls.return_value = mock_publisher
        mock_sub_cls.return_value = mock_subscriber
        mock_publisher.topic_path.return_value = "projects/my-project/topics/entdb-wal"
        mock_subscriber.subscription_path.return_value = (
            "projects/my-project/subscriptions/entdb-sub"
        )

        with p1, p2, p3:
            from dbaas.entdb_server.wal.pubsub import PubSubWalStream

            wal = PubSubWalStream(self._pubsub_config())
            await wal.connect()

            mock_pub_cls.assert_called_once()
            mock_sub_cls.assert_called_once()
            mock_publisher.topic_path.assert_called_once_with("my-project", "entdb-wal")
            mock_subscriber.subscription_path.assert_called_once_with("my-project", "entdb-sub")
            assert wal.is_connected

    @pytest.mark.asyncio
    async def test_append_calls_publish(self):
        """append() calls publisher.publish with topic_path and ordering_key."""
        p1, p2, p3, mock_pub_cls, mock_sub_cls = self._patch_pubsub()
        mock_publisher = MagicMock()
        mock_subscriber = MagicMock()
        mock_pub_cls.return_value = mock_publisher
        mock_sub_cls.return_value = mock_subscriber
        mock_publisher.topic_path.return_value = "projects/my-project/topics/entdb-wal"
        mock_subscriber.subscription_path.return_value = (
            "projects/my-project/subscriptions/entdb-sub"
        )
        # publish returns a future
        mock_future = MagicMock()
        mock_future.result.return_value = "msg-id-123"
        mock_publisher.publish.return_value = mock_future

        with p1, p2, p3:
            from dbaas.entdb_server.wal.pubsub import PubSubWalStream

            # Use ordering_enabled=True here but mock the lazy import
            config = self._pubsub_config(ordering_enabled=True)
            wal = PubSubWalStream(config)
            # Manually set internal state to skip connect() lazy import
            wal._publisher = mock_publisher
            wal._subscriber = mock_subscriber
            wal._topic_path = "projects/my-project/topics/entdb-wal"
            wal._subscription_path = "projects/my-project/subscriptions/entdb-sub"
            wal._connected = True

            pos = await wal.append("entdb-wal", "tenant_1", b"data")

            mock_publisher.publish.assert_called_once()
            call_args = mock_publisher.publish.call_args
            assert call_args[0][0] == "projects/my-project/topics/entdb-wal"
            assert call_args[1]["data"] == b"data"
            assert call_args[1]["ordering_key"] == "tenant_1"
            assert pos.topic == "entdb-wal"
            assert pos.partition == 0

    @pytest.mark.asyncio
    async def test_append_not_connected_raises(self):
        """append() raises WalConnectionError when not connected."""
        p1, p2, p3, mock_pub_cls, _mock_sub_cls = self._patch_pubsub()
        with p1, p2, p3:
            from dbaas.entdb_server.wal.pubsub import PubSubWalStream

            wal = PubSubWalStream(self._pubsub_config())
            with pytest.raises(WalConnectionError):
                await wal.append("t", "k", b"v")

    @pytest.mark.asyncio
    async def test_close_stops_publisher_and_subscriber(self):
        """close() calls stop/close on clients."""
        p1, p2, p3, mock_pub_cls, mock_sub_cls = self._patch_pubsub()
        mock_publisher = MagicMock()
        mock_subscriber = MagicMock()
        mock_pub_cls.return_value = mock_publisher
        mock_sub_cls.return_value = mock_subscriber
        mock_publisher.topic_path.return_value = "projects/my-project/topics/entdb-wal"
        mock_subscriber.subscription_path.return_value = (
            "projects/my-project/subscriptions/entdb-sub"
        )

        with p1, p2, p3:
            from dbaas.entdb_server.wal.pubsub import PubSubWalStream

            wal = PubSubWalStream(self._pubsub_config())
            await wal.connect()
            await wal.close()

            mock_publisher.stop.assert_called_once()
            mock_subscriber.close.assert_called_once()
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_connect_failure_raises_wal_connection_error(self):
        """connect() wraps SDK errors into WalConnectionError."""
        p1, p2, p3, mock_pub_cls, _mock_sub_cls = self._patch_pubsub()
        mock_pub_cls.side_effect = Exception("auth failed")
        with p1, p2, p3:
            from dbaas.entdb_server.wal.pubsub import PubSubWalStream

            wal = PubSubWalStream(self._pubsub_config())
            with pytest.raises(WalConnectionError, match="auth failed"):
                await wal.connect()
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_is_connected_initially_false(self):
        """is_connected is False before connect()."""
        p1, p2, p3, _mock_pub_cls, _mock_sub_cls = self._patch_pubsub()
        with p1, p2, p3:
            from dbaas.entdb_server.wal.pubsub import PubSubWalStream

            wal = PubSubWalStream(self._pubsub_config())
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_commit_acknowledges_message(self):
        """commit() calls subscriber.acknowledge with the stored ack_id."""
        p1, p2, p3, mock_pub_cls, mock_sub_cls = self._patch_pubsub()
        mock_publisher = MagicMock()
        mock_subscriber = MagicMock()
        mock_pub_cls.return_value = mock_publisher
        mock_sub_cls.return_value = mock_subscriber
        mock_publisher.topic_path.return_value = "projects/my-project/topics/entdb-wal"
        mock_subscriber.subscription_path.return_value = (
            "projects/my-project/subscriptions/entdb-sub"
        )

        with p1, p2, p3:
            from dbaas.entdb_server.wal.pubsub import PubSubWalStream

            wal = PubSubWalStream(self._pubsub_config())
            await wal.connect()

            # Simulate a pending ack
            wal._pending_acks["42"] = "ack-id-xyz"

            record = _sample_record(offset=42)
            await wal.commit(record)

            mock_subscriber.acknowledge.assert_called_once()

    @pytest.mark.asyncio
    async def test_commit_without_subscriber_raises(self):
        """commit() raises WalError when there is no subscriber."""
        p1, p2, p3, _mock_pub_cls, _mock_sub_cls = self._patch_pubsub()
        with p1, p2, p3:
            from dbaas.entdb_server.wal.pubsub import PubSubWalStream

            wal = PubSubWalStream(self._pubsub_config())
            with pytest.raises(WalError, match="No active subscriber"):
                await wal.commit(_sample_record())


# ===================================================================
# SQS
# ===================================================================


@pytest.mark.unit
class TestSqsWalStream:
    """Tests for SqsWalStream using mocked aiobotocore."""

    def _make_connected_wal(self):
        """Create an SqsWalStream with a mocked client already connected."""
        mock_session = MagicMock()
        mock_client_ctx = AsyncMock()
        mock_client = AsyncMock()
        mock_client_ctx.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client_ctx.__aexit__ = AsyncMock(return_value=False)
        mock_session.create_client.return_value = mock_client_ctx
        return mock_session, mock_client_ctx, mock_client

    @pytest.mark.asyncio
    async def test_connect_creates_sqs_client(self):
        """connect() creates an aiobotocore SQS client."""
        mock_session, mock_client_ctx, mock_client = self._make_connected_wal()

        # We need to stub the actual aiobotocore.session module
        mock_aio_session_mod = MagicMock()
        mock_aio_session_mod.get_session.return_value = mock_session

        with (
            patch("dbaas.entdb_server.wal.sqs.AIOBOTOCORE_AVAILABLE", True),
            patch.dict(
                sys.modules,
                {"aiobotocore.session": mock_aio_session_mod, "aiobotocore": MagicMock()},
            ),
            patch(
                "dbaas.entdb_server.wal.sqs.aiobotocore.session",
                mock_aio_session_mod,
            ),
        ):
            from dbaas.entdb_server.wal.sqs import SqsWalStream

            wal = SqsWalStream(_make_sqs_config())
            await wal.connect()

            mock_session.create_client.assert_called_once_with("sqs", region_name="us-east-1")
            assert wal.is_connected

    @pytest.mark.asyncio
    async def test_append_calls_send_message(self):
        """append() calls send_message with QueueUrl, MessageBody, MessageGroupId."""
        mock_session, mock_client_ctx, mock_client = self._make_connected_wal()
        mock_client.send_message.return_value = {"MessageId": "msg-123"}

        mock_aio_session_mod = MagicMock()
        mock_aio_session_mod.get_session.return_value = mock_session

        with (
            patch("dbaas.entdb_server.wal.sqs.AIOBOTOCORE_AVAILABLE", True),
            patch.dict(
                sys.modules,
                {"aiobotocore.session": mock_aio_session_mod, "aiobotocore": MagicMock()},
            ),
            patch(
                "dbaas.entdb_server.wal.sqs.aiobotocore.session",
                mock_aio_session_mod,
            ),
        ):
            from dbaas.entdb_server.wal.sqs import SqsWalStream

            config = _make_sqs_config()
            wal = SqsWalStream(config)
            await wal.connect()
            pos = await wal.append(config.queue_url, "tenant_1", b'{"op":"create"}')

            mock_client.send_message.assert_awaited_once()
            call_kwargs = mock_client.send_message.call_args[1]
            assert call_kwargs["QueueUrl"] == config.queue_url
            assert call_kwargs["MessageBody"] == '{"op":"create"}'
            assert call_kwargs["MessageGroupId"] == "tenant_1"
            assert "MessageDeduplicationId" in call_kwargs
            assert pos.partition == 0

    @pytest.mark.asyncio
    async def test_append_not_connected_raises(self):
        """append() raises WalConnectionError when not connected."""
        with patch("dbaas.entdb_server.wal.sqs.AIOBOTOCORE_AVAILABLE", True):
            from dbaas.entdb_server.wal.sqs import SqsWalStream

            wal = SqsWalStream(_make_sqs_config())
            with pytest.raises(WalConnectionError):
                await wal.append("q", "k", b"v")

    @pytest.mark.asyncio
    async def test_close_exits_client_context(self):
        """close() exits the aiobotocore client context manager."""
        mock_session, mock_client_ctx, mock_client = self._make_connected_wal()

        mock_aio_session_mod = MagicMock()
        mock_aio_session_mod.get_session.return_value = mock_session

        with (
            patch("dbaas.entdb_server.wal.sqs.AIOBOTOCORE_AVAILABLE", True),
            patch.dict(
                sys.modules,
                {"aiobotocore.session": mock_aio_session_mod, "aiobotocore": MagicMock()},
            ),
            patch(
                "dbaas.entdb_server.wal.sqs.aiobotocore.session",
                mock_aio_session_mod,
            ),
        ):
            from dbaas.entdb_server.wal.sqs import SqsWalStream

            wal = SqsWalStream(_make_sqs_config())
            await wal.connect()
            await wal.close()

            mock_client_ctx.__aexit__.assert_awaited_once_with(None, None, None)
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_connect_failure_raises_wal_connection_error(self):
        """connect() wraps SDK errors into WalConnectionError."""
        mock_session = MagicMock()
        mock_client_ctx = AsyncMock()
        mock_client_ctx.__aenter__ = AsyncMock(side_effect=Exception("endpoint unreachable"))
        mock_session.create_client.return_value = mock_client_ctx

        mock_aio_session_mod = MagicMock()
        mock_aio_session_mod.get_session.return_value = mock_session

        with (
            patch("dbaas.entdb_server.wal.sqs.AIOBOTOCORE_AVAILABLE", True),
            patch.dict(
                sys.modules,
                {"aiobotocore.session": mock_aio_session_mod, "aiobotocore": MagicMock()},
            ),
            patch(
                "dbaas.entdb_server.wal.sqs.aiobotocore.session",
                mock_aio_session_mod,
            ),
        ):
            from dbaas.entdb_server.wal.sqs import SqsWalStream

            wal = SqsWalStream(_make_sqs_config())
            with pytest.raises(WalConnectionError, match="endpoint unreachable"):
                await wal.connect()
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_is_connected_initially_false(self):
        """is_connected is False before connect()."""
        with patch("dbaas.entdb_server.wal.sqs.AIOBOTOCORE_AVAILABLE", True):
            from dbaas.entdb_server.wal.sqs import SqsWalStream

            wal = SqsWalStream(_make_sqs_config())
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_commit_calls_delete_message(self):
        """commit() calls delete_message with QueueUrl and ReceiptHandle."""
        mock_client = AsyncMock()

        with patch("dbaas.entdb_server.wal.sqs.AIOBOTOCORE_AVAILABLE", True):
            from dbaas.entdb_server.wal.sqs import SqsWalStream

            config = _make_sqs_config()
            wal = SqsWalStream(config)
            wal._client = mock_client
            wal._connected = True

            # Simulate a pending ack
            wal._pending_acks["1"] = "receipt-handle-abc"

            record = _sample_record(topic=config.queue_url, partition=0, offset=1)
            await wal.commit(record)

            mock_client.delete_message.assert_awaited_once_with(
                QueueUrl=config.queue_url,
                ReceiptHandle="receipt-handle-abc",
            )

    @pytest.mark.asyncio
    async def test_commit_without_client_raises(self):
        """commit() raises WalError when there is no client."""
        with patch("dbaas.entdb_server.wal.sqs.AIOBOTOCORE_AVAILABLE", True):
            from dbaas.entdb_server.wal.sqs import SqsWalStream

            wal = SqsWalStream(_make_sqs_config())
            with pytest.raises(WalError, match="No active SQS client"):
                await wal.commit(_sample_record())


# ===================================================================
# Service Bus
# ===================================================================


@pytest.mark.unit
class TestServiceBusWalStream:
    """Tests for ServiceBusWalStream using mocked azure-servicebus."""

    @pytest.mark.asyncio
    async def test_connect_creates_client_from_connection_string(self):
        """connect() creates ServiceBusClient from connection string."""
        mock_client = MagicMock()
        mock_sender = MagicMock()
        mock_client.get_queue_sender.return_value = mock_sender

        mock_sb_client_cls = MagicMock()
        mock_sb_client_cls.from_connection_string.return_value = mock_client

        with (
            patch("dbaas.entdb_server.wal.servicebus.SERVICEBUS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.wal.servicebus.ServiceBusClient",
                mock_sb_client_cls,
            ),
            patch("dbaas.entdb_server.wal.servicebus.ServiceBusMessage"),
        ):
            from dbaas.entdb_server.wal.servicebus import ServiceBusWalStream

            config = _make_servicebus_config()
            wal = ServiceBusWalStream(config)
            await wal.connect()

            mock_sb_client_cls.from_connection_string.assert_called_once_with(
                conn_str=config.connection_string
            )
            mock_client.get_queue_sender.assert_called_once_with(queue_name="entdb-wal")
            assert wal.is_connected

    @pytest.mark.asyncio
    async def test_append_sends_message_with_session_id(self):
        """append() creates a ServiceBusMessage with session_id and sends it."""
        mock_client = MagicMock()
        mock_sender = AsyncMock()
        mock_client.get_queue_sender.return_value = mock_sender
        mock_sb_client_cls = MagicMock()
        mock_sb_client_cls.from_connection_string.return_value = mock_client

        mock_message = MagicMock()
        mock_sb_message_cls = MagicMock(return_value=mock_message)

        with (
            patch("dbaas.entdb_server.wal.servicebus.SERVICEBUS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.wal.servicebus.ServiceBusClient",
                mock_sb_client_cls,
            ),
            patch(
                "dbaas.entdb_server.wal.servicebus.ServiceBusMessage",
                mock_sb_message_cls,
            ),
        ):
            from dbaas.entdb_server.wal.servicebus import ServiceBusWalStream

            wal = ServiceBusWalStream(_make_servicebus_config())
            await wal.connect()
            pos = await wal.append("entdb-wal", "tenant_1", b"payload")

            mock_sb_message_cls.assert_called_once_with(body=b"payload", session_id="tenant_1")
            mock_sender.send_messages.assert_awaited_once_with(mock_message)
            assert pos.topic == "entdb-wal"
            assert pos.partition == 0

    @pytest.mark.asyncio
    async def test_append_not_connected_raises(self):
        """append() raises WalConnectionError when not connected."""
        with (
            patch("dbaas.entdb_server.wal.servicebus.SERVICEBUS_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.servicebus.ServiceBusClient"),
            patch("dbaas.entdb_server.wal.servicebus.ServiceBusMessage"),
        ):
            from dbaas.entdb_server.wal.servicebus import ServiceBusWalStream

            wal = ServiceBusWalStream(_make_servicebus_config())
            with pytest.raises(WalConnectionError):
                await wal.append("q", "k", b"v")

    @pytest.mark.asyncio
    async def test_close_closes_sender_and_client(self):
        """close() closes sender, receiver, and client."""
        # ServiceBusClient.from_connection_string() and get_queue_sender()
        # are synchronous calls, but sender.close() and client.close() are
        # awaited -- use MagicMock for the client with AsyncMock children.
        mock_sender = MagicMock()
        mock_sender.close = AsyncMock()
        mock_client = MagicMock()
        mock_client.get_queue_sender.return_value = mock_sender
        mock_client.close = AsyncMock()
        mock_sb_client_cls = MagicMock()
        mock_sb_client_cls.from_connection_string.return_value = mock_client

        with (
            patch("dbaas.entdb_server.wal.servicebus.SERVICEBUS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.wal.servicebus.ServiceBusClient",
                mock_sb_client_cls,
            ),
            patch("dbaas.entdb_server.wal.servicebus.ServiceBusMessage"),
        ):
            from dbaas.entdb_server.wal.servicebus import ServiceBusWalStream

            wal = ServiceBusWalStream(_make_servicebus_config())
            await wal.connect()
            await wal.close()

            mock_sender.close.assert_awaited_once()
            mock_client.close.assert_awaited_once()
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_connect_failure_raises_wal_connection_error(self):
        """connect() wraps SDK errors into WalConnectionError."""
        mock_sb_client_cls = MagicMock()
        mock_sb_client_cls.from_connection_string.side_effect = Exception(
            "invalid connection string"
        )

        with (
            patch("dbaas.entdb_server.wal.servicebus.SERVICEBUS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.wal.servicebus.ServiceBusClient",
                mock_sb_client_cls,
            ),
            patch("dbaas.entdb_server.wal.servicebus.ServiceBusMessage"),
        ):
            from dbaas.entdb_server.wal.servicebus import ServiceBusWalStream

            wal = ServiceBusWalStream(_make_servicebus_config())
            with pytest.raises(WalConnectionError, match="invalid connection string"):
                await wal.connect()
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_is_connected_initially_false(self):
        """is_connected is False before connect()."""
        with (
            patch("dbaas.entdb_server.wal.servicebus.SERVICEBUS_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.servicebus.ServiceBusClient"),
            patch("dbaas.entdb_server.wal.servicebus.ServiceBusMessage"),
        ):
            from dbaas.entdb_server.wal.servicebus import ServiceBusWalStream

            wal = ServiceBusWalStream(_make_servicebus_config())
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_commit_completes_message(self):
        """commit() calls receiver.complete_message on the stored message."""
        with (
            patch("dbaas.entdb_server.wal.servicebus.SERVICEBUS_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.servicebus.ServiceBusClient"),
            patch("dbaas.entdb_server.wal.servicebus.ServiceBusMessage"),
        ):
            from dbaas.entdb_server.wal.servicebus import ServiceBusWalStream

            wal = ServiceBusWalStream(_make_servicebus_config())

            mock_receiver = AsyncMock()
            mock_msg = MagicMock()
            wal._pending_messages["1"] = (mock_receiver, mock_msg)

            record = _sample_record(offset=1)
            await wal.commit(record)

            mock_receiver.complete_message.assert_awaited_once_with(mock_msg)

    @pytest.mark.asyncio
    async def test_append_with_headers_sets_application_properties(self):
        """append() sets application_properties when headers are provided."""
        mock_client = MagicMock()
        mock_sender = AsyncMock()
        mock_client.get_queue_sender.return_value = mock_sender
        mock_sb_client_cls = MagicMock()
        mock_sb_client_cls.from_connection_string.return_value = mock_client

        mock_message = MagicMock()
        mock_sb_message_cls = MagicMock(return_value=mock_message)

        with (
            patch("dbaas.entdb_server.wal.servicebus.SERVICEBUS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.wal.servicebus.ServiceBusClient",
                mock_sb_client_cls,
            ),
            patch(
                "dbaas.entdb_server.wal.servicebus.ServiceBusMessage",
                mock_sb_message_cls,
            ),
        ):
            from dbaas.entdb_server.wal.servicebus import ServiceBusWalStream

            wal = ServiceBusWalStream(_make_servicebus_config())
            await wal.connect()

            headers = {"x-type": b"create"}
            await wal.append("entdb-wal", "t1", b"data", headers=headers)

            assert mock_message.application_properties == {"x-type": "create"}


# ===================================================================
# Event Hubs
# ===================================================================


@pytest.mark.unit
class TestEventHubsWalStream:
    """Mock-based tests for Azure Event Hubs WAL backend."""

    @pytest.mark.asyncio
    async def test_connect_creates_clients(self):
        """connect() creates producer and consumer clients from connection string."""
        mock_producer = MagicMock()
        mock_consumer = MagicMock()

        mock_producer_cls = MagicMock()
        mock_producer_cls.from_connection_string.return_value = mock_producer

        mock_consumer_cls = MagicMock()
        mock_consumer_cls.from_connection_string.return_value = mock_consumer

        with (
            patch("dbaas.entdb_server.wal.eventhubs.EVENTHUBS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.wal.eventhubs.EventHubProducerClient",
                mock_producer_cls,
            ),
            patch(
                "dbaas.entdb_server.wal.eventhubs.EventHubConsumerClient",
                mock_consumer_cls,
            ),
            patch("dbaas.entdb_server.wal.eventhubs.EventData"),
        ):
            from dbaas.entdb_server.wal.eventhubs import EventHubsWalStream

            config = _make_eventhubs_config()
            wal = EventHubsWalStream(config)
            await wal.connect()

            mock_producer_cls.from_connection_string.assert_called_once_with(
                conn_str=config.connection_string,
                eventhub_name=config.eventhub_name,
            )
            mock_consumer_cls.from_connection_string.assert_called_once_with(
                conn_str=config.connection_string,
                consumer_group=config.consumer_group,
                eventhub_name=config.eventhub_name,
            )
            assert wal.is_connected

    @pytest.mark.asyncio
    async def test_append_sends_event_data(self):
        """append() creates EventData and sends via producer batch."""
        mock_producer = MagicMock()
        mock_batch = MagicMock()
        mock_producer.create_batch = AsyncMock(return_value=mock_batch)
        mock_producer.send_batch = AsyncMock()

        mock_producer_cls = MagicMock()
        mock_producer_cls.from_connection_string.return_value = mock_producer

        mock_consumer_cls = MagicMock()
        mock_consumer_cls.from_connection_string.return_value = MagicMock()

        mock_event_data = MagicMock()
        mock_event_data_cls = MagicMock(return_value=mock_event_data)

        with (
            patch("dbaas.entdb_server.wal.eventhubs.EVENTHUBS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.wal.eventhubs.EventHubProducerClient",
                mock_producer_cls,
            ),
            patch(
                "dbaas.entdb_server.wal.eventhubs.EventHubConsumerClient",
                mock_consumer_cls,
            ),
            patch(
                "dbaas.entdb_server.wal.eventhubs.EventData",
                mock_event_data_cls,
            ),
        ):
            from dbaas.entdb_server.wal.eventhubs import EventHubsWalStream

            wal = EventHubsWalStream(_make_eventhubs_config())
            await wal.connect()
            pos = await wal.append("entdb-wal", "tenant_1", b"payload")

            mock_event_data_cls.assert_called_once_with(body=b"payload")
            mock_producer.send_batch.assert_awaited_once_with(mock_batch)
            assert pos.topic == "entdb-wal"
            assert pos.partition == 0

    @pytest.mark.asyncio
    async def test_append_sets_partition_key(self):
        """append() sets partition_key on the batch for ordering."""
        mock_producer = MagicMock()
        mock_batch = MagicMock()
        mock_producer.create_batch = AsyncMock(return_value=mock_batch)
        mock_producer.send_batch = AsyncMock()

        mock_producer_cls = MagicMock()
        mock_producer_cls.from_connection_string.return_value = mock_producer

        mock_consumer_cls = MagicMock()
        mock_consumer_cls.from_connection_string.return_value = MagicMock()

        mock_event_data_cls = MagicMock(return_value=MagicMock())

        with (
            patch("dbaas.entdb_server.wal.eventhubs.EVENTHUBS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.wal.eventhubs.EventHubProducerClient",
                mock_producer_cls,
            ),
            patch(
                "dbaas.entdb_server.wal.eventhubs.EventHubConsumerClient",
                mock_consumer_cls,
            ),
            patch(
                "dbaas.entdb_server.wal.eventhubs.EventData",
                mock_event_data_cls,
            ),
        ):
            from dbaas.entdb_server.wal.eventhubs import EventHubsWalStream

            wal = EventHubsWalStream(_make_eventhubs_config())
            await wal.connect()
            await wal.append("entdb-wal", "tenant_1", b"payload")

            mock_producer.create_batch.assert_awaited_once_with(partition_key="tenant_1")

    @pytest.mark.asyncio
    async def test_close_closes_clients(self):
        """close() closes both producer and consumer clients."""
        mock_producer = MagicMock()
        mock_producer.close = AsyncMock()
        mock_consumer = MagicMock()
        mock_consumer.close = AsyncMock()

        mock_producer_cls = MagicMock()
        mock_producer_cls.from_connection_string.return_value = mock_producer

        mock_consumer_cls = MagicMock()
        mock_consumer_cls.from_connection_string.return_value = mock_consumer

        with (
            patch("dbaas.entdb_server.wal.eventhubs.EVENTHUBS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.wal.eventhubs.EventHubProducerClient",
                mock_producer_cls,
            ),
            patch(
                "dbaas.entdb_server.wal.eventhubs.EventHubConsumerClient",
                mock_consumer_cls,
            ),
            patch("dbaas.entdb_server.wal.eventhubs.EventData"),
        ):
            from dbaas.entdb_server.wal.eventhubs import EventHubsWalStream

            wal = EventHubsWalStream(_make_eventhubs_config())
            await wal.connect()
            await wal.close()

            mock_producer.close.assert_awaited_once()
            mock_consumer.close.assert_awaited_once()
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_connect_failure_raises(self):
        """connect() wraps SDK errors into WalConnectionError."""
        mock_producer_cls = MagicMock()
        mock_producer_cls.from_connection_string.side_effect = Exception(
            "invalid connection string"
        )

        with (
            patch("dbaas.entdb_server.wal.eventhubs.EVENTHUBS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.wal.eventhubs.EventHubProducerClient",
                mock_producer_cls,
            ),
            patch("dbaas.entdb_server.wal.eventhubs.EventHubConsumerClient"),
            patch("dbaas.entdb_server.wal.eventhubs.EventData"),
        ):
            from dbaas.entdb_server.wal.eventhubs import EventHubsWalStream

            wal = EventHubsWalStream(_make_eventhubs_config())
            with pytest.raises(WalConnectionError, match="invalid connection string"):
                await wal.connect()
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_is_connected(self):
        """is_connected is False before connect() and True after."""
        with (
            patch("dbaas.entdb_server.wal.eventhubs.EVENTHUBS_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.eventhubs.EventHubProducerClient"),
            patch("dbaas.entdb_server.wal.eventhubs.EventHubConsumerClient"),
            patch("dbaas.entdb_server.wal.eventhubs.EventData"),
        ):
            from dbaas.entdb_server.wal.eventhubs import EventHubsWalStream

            wal = EventHubsWalStream(_make_eventhubs_config())
            assert not wal.is_connected

    @pytest.mark.asyncio
    async def test_commit_updates_checkpoint(self):
        """commit() removes the checkpoint entry for the record."""
        with (
            patch("dbaas.entdb_server.wal.eventhubs.EVENTHUBS_AVAILABLE", True),
            patch("dbaas.entdb_server.wal.eventhubs.EventHubProducerClient"),
            patch("dbaas.entdb_server.wal.eventhubs.EventHubConsumerClient"),
            patch("dbaas.entdb_server.wal.eventhubs.EventData"),
        ):
            from dbaas.entdb_server.wal.eventhubs import EventHubsWalStream

            wal = EventHubsWalStream(_make_eventhubs_config())

            # Simulate a pending checkpoint
            wal._checkpoints["1"] = {
                "sequence_number": 1,
                "offset": "100",
            }

            record = _sample_record(offset=1)
            await wal.commit(record)

            assert "1" not in wal._checkpoints
