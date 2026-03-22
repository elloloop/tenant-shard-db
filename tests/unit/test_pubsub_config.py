"""Unit tests for Google Pub/Sub configuration."""

import pytest

from dbaas.entdb_server.config import PubSubConfig, WalBackend


@pytest.mark.unit
class TestPubSubConfig:
    def test_defaults(self):
        config = PubSubConfig()
        assert config.topic_id == "entdb-wal"
        assert config.subscription_id == "entdb-applier"
        assert config.ordering_enabled is True
        assert config.max_messages == 100

    def test_from_env(self, monkeypatch):
        monkeypatch.setenv("PUBSUB_PROJECT_ID", "my-project")
        monkeypatch.setenv("PUBSUB_TOPIC_ID", "my-topic")
        monkeypatch.setenv("PUBSUB_SUBSCRIPTION_ID", "my-sub")
        config = PubSubConfig.from_env()
        assert config.project_id == "my-project"
        assert config.topic_id == "my-topic"
        assert config.subscription_id == "my-sub"

    def test_gcp_project_fallback(self, monkeypatch):
        monkeypatch.setenv("GCP_PROJECT_ID", "fallback-project")
        config = PubSubConfig.from_env()
        assert config.project_id == "fallback-project"

    def test_emulator_endpoint(self, monkeypatch):
        monkeypatch.setenv("PUBSUB_ENDPOINT", "localhost:8085")
        config = PubSubConfig.from_env()
        assert config.endpoint == "localhost:8085"

    def test_wal_backend_enum(self):
        assert WalBackend.PUBSUB.value == "pubsub"
