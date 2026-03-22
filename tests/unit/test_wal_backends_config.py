"""Unit tests for WAL backend configurations."""

import pytest

from dbaas.entdb_server.config import (
    ServiceBusConfig,
    SqsConfig,
    WalBackend,
)


@pytest.mark.unit
class TestWalBackendEnum:
    def test_all_backends(self):
        assert WalBackend.KAFKA.value == "kafka"
        assert WalBackend.KINESIS.value == "kinesis"
        assert WalBackend.PUBSUB.value == "pubsub"
        assert WalBackend.SQS.value == "sqs"
        assert WalBackend.SERVICEBUS.value == "servicebus"
        assert WalBackend.LOCAL.value == "local"


@pytest.mark.unit
class TestSqsConfig:
    def test_defaults(self):
        config = SqsConfig()
        assert config.queue_url == ""
        assert config.region == "us-east-1"
        assert config.max_messages == 10

    def test_from_env(self, monkeypatch):
        monkeypatch.setenv(
            "SQS_QUEUE_URL",
            "https://sqs.us-east-1.amazonaws.com/123/entdb.fifo",
        )
        monkeypatch.setenv("SQS_REGION", "us-west-2")
        config = SqsConfig.from_env()
        assert "123" in config.queue_url
        assert config.region == "us-west-2"

    def test_localstack_endpoint(self, monkeypatch):
        monkeypatch.setenv("SQS_ENDPOINT", "http://localhost:4566")
        config = SqsConfig.from_env()
        assert config.endpoint_url == "http://localhost:4566"


@pytest.mark.unit
class TestServiceBusConfig:
    def test_defaults(self):
        config = ServiceBusConfig()
        assert config.queue_name == "entdb-wal"
        assert config.max_messages == 20

    def test_from_env(self, monkeypatch):
        monkeypatch.setenv(
            "SERVICEBUS_CONNECTION_STRING",
            "Endpoint=sb://test.servicebus.windows.net/;"
            "SharedAccessKeyName=key;SharedAccessKey=val",
        )
        monkeypatch.setenv("SERVICEBUS_QUEUE_NAME", "my-queue")
        config = ServiceBusConfig.from_env()
        assert "test.servicebus" in config.connection_string
        assert config.queue_name == "my-queue"


@pytest.mark.unit
class TestLocalBackend:
    def test_local_creates_memory_stream(self):
        # Just verify the enum value exists and maps correctly
        assert WalBackend("local") == WalBackend.LOCAL
