"""Unit tests for gRPC authentication."""

from unittest.mock import MagicMock

import pytest

from dbaas.entdb_server.api.auth import ApiKeyInterceptor
from dbaas.entdb_server.config import GrpcConfig


@pytest.mark.unit
class TestApiKeyInterceptor:
    def test_valid_key_passes(self):
        interceptor = ApiKeyInterceptor(frozenset({"key-1", "key-2"}))
        continuation = MagicMock(return_value="handler")
        details = MagicMock()
        details.method = "/entdb.EntDBService/ExecuteAtomic"
        details.invocation_metadata = [("authorization", "Bearer key-1")]
        result = interceptor.intercept_service(continuation, details)
        continuation.assert_called_once()
        assert result == "handler"

    def test_invalid_key_rejected(self):
        interceptor = ApiKeyInterceptor(frozenset({"key-1"}))
        continuation = MagicMock()
        details = MagicMock()
        details.method = "/entdb.EntDBService/ExecuteAtomic"
        details.invocation_metadata = [("authorization", "Bearer wrong-key")]
        result = interceptor.intercept_service(continuation, details)
        continuation.assert_not_called()
        assert result is not None

    def test_missing_key_rejected(self):
        interceptor = ApiKeyInterceptor(frozenset({"key-1"}))
        continuation = MagicMock()
        details = MagicMock()
        details.method = "/entdb.EntDBService/ExecuteAtomic"
        details.invocation_metadata = []
        interceptor.intercept_service(continuation, details)
        continuation.assert_not_called()

    def test_health_check_bypasses_auth(self):
        interceptor = ApiKeyInterceptor(frozenset({"key-1"}))
        continuation = MagicMock(return_value="handler")
        details = MagicMock()
        details.method = "/entdb.EntDBService/Health"
        details.invocation_metadata = []  # No key
        result = interceptor.intercept_service(continuation, details)
        continuation.assert_called_once()
        assert result == "handler"

    def test_raw_key_without_bearer(self):
        interceptor = ApiKeyInterceptor(frozenset({"raw-key"}))
        continuation = MagicMock(return_value="handler")
        details = MagicMock()
        details.method = "/entdb.EntDBService/GetNode"
        details.invocation_metadata = [("authorization", "raw-key")]
        result = interceptor.intercept_service(continuation, details)
        continuation.assert_called_once()
        assert result == "handler"

    def test_empty_keys_rejects_all(self):
        interceptor = ApiKeyInterceptor(frozenset())
        continuation = MagicMock()
        details = MagicMock()
        details.method = "/entdb.EntDBService/GetNode"
        details.invocation_metadata = [("authorization", "Bearer anything")]
        interceptor.intercept_service(continuation, details)
        continuation.assert_not_called()


@pytest.mark.unit
class TestAuthConfig:
    def test_auth_disabled_by_default(self):
        config = GrpcConfig()
        assert config.auth_enabled is False
        assert config.auth_api_keys == frozenset()

    def test_auth_from_env(self, monkeypatch):
        monkeypatch.setenv("AUTH_ENABLED", "true")
        monkeypatch.setenv("AUTH_API_KEYS", "key1,key2,key3")
        config = GrpcConfig.from_env()
        assert config.auth_enabled is True
        assert config.auth_api_keys == frozenset({"key1", "key2", "key3"})

    def test_tls_config(self, monkeypatch):
        monkeypatch.setenv("GRPC_TLS_CERT", "/path/to/cert.pem")
        monkeypatch.setenv("GRPC_TLS_KEY", "/path/to/key.pem")
        config = GrpcConfig.from_env()
        assert config.tls_cert_file == "/path/to/cert.pem"
        assert config.tls_key_file == "/path/to/key.pem"
