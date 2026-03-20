"""
Unit tests for ArchiverConfig and RecoveryConfig.

Tests cover:
- Default values for all configuration fields
- Environment variable overrides via from_env()
- Disabled archiver behavior (flush_mode override)
- Recovery tier toggle settings
"""

from __future__ import annotations

import pytest

from dbaas.entdb_server.config import ArchiverConfig, RecoveryConfig, ServerConfig


@pytest.mark.unit
class TestArchiverConfig:
    """Tests for ArchiverConfig defaults and env-based construction."""

    def test_defaults(self):
        """Default ArchiverConfig has sensible production values."""
        config = ArchiverConfig()
        assert config.enabled is True
        assert config.flush_mode == "batched"
        assert config.flush_interval_seconds == 60
        assert config.max_segment_events == 10000
        assert config.min_segment_events == 1
        assert config.compression == "gzip"
        assert config.s3_storage_class == "STANDARD"
        assert config.deduplicate is True

    def test_disabled_via_env(self, monkeypatch):
        """ARCHIVER_ENABLED=false forces flush_mode to disabled."""
        monkeypatch.setenv("ARCHIVER_ENABLED", "false")
        config = ArchiverConfig.from_env()
        assert config.enabled is False
        assert config.flush_mode == "disabled"

    def test_individual_flush_mode(self, monkeypatch):
        """ARCHIVE_FLUSH_MODE=individual sets flush_mode."""
        monkeypatch.setenv("ARCHIVE_FLUSH_MODE", "individual")
        config = ArchiverConfig.from_env()
        assert config.flush_mode == "individual"

    def test_batched_with_custom_interval(self, monkeypatch):
        """Custom flush interval is respected."""
        monkeypatch.setenv("ARCHIVE_FLUSH_SECONDS", "14400")
        config = ArchiverConfig.from_env()
        assert config.flush_interval_seconds == 14400

    def test_custom_storage_class(self, monkeypatch):
        """S3 storage class can be overridden."""
        monkeypatch.setenv("ARCHIVE_S3_STORAGE_CLASS", "INFREQUENT_ACCESS")
        config = ArchiverConfig.from_env()
        assert config.s3_storage_class == "INFREQUENT_ACCESS"

    def test_min_segment_events(self, monkeypatch):
        """Minimum segment events threshold is configurable."""
        monkeypatch.setenv("ARCHIVE_MIN_SEGMENT_EVENTS", "100")
        config = ArchiverConfig.from_env()
        assert config.min_segment_events == 100


@pytest.mark.unit
class TestRecoveryConfig:
    """Tests for RecoveryConfig defaults and env-based construction."""

    def test_defaults(self):
        """Default RecoveryConfig enables all tiers."""
        config = RecoveryConfig()
        assert config.kafka_replay_enabled is True
        assert config.archive_replay_enabled is True
        assert config.kafka_replay_timeout_seconds == 300
        assert config.verify_after_recovery is True

    def test_kafka_disabled(self, monkeypatch):
        """RECOVERY_KAFKA_REPLAY=false disables Kafka tier."""
        monkeypatch.setenv("RECOVERY_KAFKA_REPLAY", "false")
        config = RecoveryConfig.from_env()
        assert config.kafka_replay_enabled is False

    def test_archive_disabled(self, monkeypatch):
        """RECOVERY_ARCHIVE_REPLAY=false disables archive tier."""
        monkeypatch.setenv("RECOVERY_ARCHIVE_REPLAY", "false")
        config = RecoveryConfig.from_env()
        assert config.archive_replay_enabled is False

    def test_custom_timeout(self, monkeypatch):
        """Kafka replay timeout is configurable."""
        monkeypatch.setenv("RECOVERY_KAFKA_TIMEOUT", "600")
        config = RecoveryConfig.from_env()
        assert config.kafka_replay_timeout_seconds == 600
