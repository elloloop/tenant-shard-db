"""
Unit tests for SQLCipher encryption integration.

Tests cover:
- Key derivation determinism and uniqueness
- EncryptionConfig defaults and validation
- CanonicalStore with encryption disabled (existing behavior)
- CanonicalStore with encryption enabled but no sqlcipher (warning fallback)
- GlobalStore with encryption enabled but no sqlcipher (warning fallback)
- Crypto-shred (file deletion)
- Encryption module helper functions
"""

from __future__ import annotations

import logging
import os
import tempfile
from pathlib import Path
from unittest.mock import patch

import pytest

from dbaas.entdb_server.config import EncryptionConfig
from dbaas.entdb_server.encryption import (
    derive_global_key,
    derive_tenant_key,
    is_sqlcipher_available,
    open_encrypted_connection,
    shred_tenant,
)


# ── Key derivation tests ─────────────────────────────────────────────


class TestKeyDerivation:
    """Tests for the HMAC-SHA256 key derivation."""

    def test_deterministic_same_inputs(self):
        """Same master key and tenant ID always produce the same key."""
        key1 = derive_tenant_key("master-secret", "tenant-abc")
        key2 = derive_tenant_key("master-secret", "tenant-abc")
        assert key1 == key2

    def test_different_tenants_different_keys(self):
        """Different tenant IDs produce different keys."""
        key_a = derive_tenant_key("master-secret", "tenant-a")
        key_b = derive_tenant_key("master-secret", "tenant-b")
        assert key_a != key_b

    def test_different_master_keys_different_results(self):
        """Different master keys produce different keys for the same tenant."""
        key1 = derive_tenant_key("key-one", "tenant-x")
        key2 = derive_tenant_key("key-two", "tenant-x")
        assert key1 != key2

    def test_key_is_hex_string(self):
        """Derived key is a 64-character hex string (SHA-256 output)."""
        key = derive_tenant_key("master", "tenant-1")
        assert len(key) == 64
        assert all(c in "0123456789abcdef" for c in key)

    def test_global_key_deterministic(self):
        """Global key derivation is deterministic."""
        k1 = derive_global_key("master")
        k2 = derive_global_key("master")
        assert k1 == k2

    def test_global_key_differs_from_tenant_key(self):
        """Global key is distinct from any tenant key."""
        gk = derive_global_key("master")
        tk = derive_tenant_key("master", "global.db")
        assert gk != tk

    def test_empty_tenant_id(self):
        """Empty tenant ID still produces a valid key."""
        key = derive_tenant_key("master", "")
        assert len(key) == 64

    def test_special_characters_in_tenant_id(self):
        """Tenant IDs with special characters produce valid keys."""
        key = derive_tenant_key("master", "tenant/with:special@chars")
        assert len(key) == 64


# ── EncryptionConfig tests ───────────────────────────────────────────


class TestEncryptionConfig:
    """Tests for EncryptionConfig dataclass."""

    def test_defaults(self):
        """Default config has encryption disabled."""
        cfg = EncryptionConfig()
        assert cfg.enabled is False
        assert cfg.master_key == ""
        assert cfg.key_derivation == "hkdf"

    def test_enabled_with_key(self):
        """Config with encryption enabled and a master key is valid."""
        cfg = EncryptionConfig(enabled=True, master_key="secret123")
        cfg.validate()  # Should not raise

    def test_enabled_without_key_raises(self):
        """Enabling encryption without a master key raises ValueError."""
        cfg = EncryptionConfig(enabled=True, master_key="")
        with pytest.raises(ValueError, match="ENTDB_MASTER_KEY"):
            cfg.validate()

    def test_disabled_without_key_ok(self):
        """Disabled encryption does not require a master key."""
        cfg = EncryptionConfig(enabled=False, master_key="")
        cfg.validate()  # Should not raise

    def test_from_env_defaults(self):
        """from_env with no env vars returns disabled config."""
        env = os.environ.copy()
        for k in ("ENTDB_ENCRYPTION_ENABLED", "ENTDB_MASTER_KEY", "ENTDB_KEY_DERIVATION"):
            env.pop(k, None)
        with patch.dict(os.environ, env, clear=True):
            cfg = EncryptionConfig.from_env()
            assert cfg.enabled is False

    def test_from_env_enabled(self):
        """from_env reads encryption settings from environment."""
        env = {
            "ENTDB_ENCRYPTION_ENABLED": "true",
            "ENTDB_MASTER_KEY": "my-secret-key",
            "ENTDB_KEY_DERIVATION": "hkdf",
        }
        with patch.dict(os.environ, env, clear=False):
            cfg = EncryptionConfig.from_env()
            assert cfg.enabled is True
            assert cfg.master_key == "my-secret-key"
            assert cfg.key_derivation == "hkdf"

    def test_frozen(self):
        """EncryptionConfig is frozen (immutable)."""
        cfg = EncryptionConfig()
        with pytest.raises(AttributeError):
            cfg.enabled = True  # type: ignore[misc]


# ── SQLCipher availability ───────────────────────────────────────────


class TestSqlcipherAvailability:
    """Tests for SQLCipher availability detection."""

    def test_is_sqlcipher_available_returns_bool(self):
        """is_sqlcipher_available returns a boolean."""
        result = is_sqlcipher_available()
        assert isinstance(result, bool)


# ── open_encrypted_connection fallback ───────────────────────────────


class TestOpenEncryptedConnection:
    """Tests for open_encrypted_connection fallback behavior."""

    def test_fallback_without_sqlcipher(self, caplog):
        """Without sqlcipher, falls back to sqlite3 with a warning."""
        with tempfile.TemporaryDirectory() as tmpdir:
            db_path = os.path.join(tmpdir, "test.db")
            with caplog.at_level(logging.WARNING, logger="dbaas.entdb_server.encryption"):
                conn = open_encrypted_connection(db_path, "deadbeef" * 8)
            conn.execute("CREATE TABLE test (id TEXT)")
            conn.execute("INSERT INTO test VALUES ('hello')")
            row = conn.execute("SELECT id FROM test").fetchone()
            assert row[0] == "hello"
            conn.close()
            # If sqlcipher is not available, we expect a warning
            if not is_sqlcipher_available():
                assert any("SQLCipher not available" in r.message for r in caplog.records)

    def test_connection_is_usable(self):
        """Returned connection (even fallback) is fully functional."""
        with tempfile.TemporaryDirectory() as tmpdir:
            db_path = os.path.join(tmpdir, "func.db")
            conn = open_encrypted_connection(db_path, "abcdef" * 10)
            conn.execute("CREATE TABLE t (v INTEGER)")
            conn.execute("INSERT INTO t VALUES (42)")
            val = conn.execute("SELECT v FROM t").fetchone()[0]
            assert val == 42
            conn.close()


# ── CanonicalStore with encryption ───────────────────────────────────


class TestCanonicalStoreEncryption:
    """Tests for CanonicalStore behavior with encryption config."""

    @pytest.fixture
    def data_dir(self):
        """Create temporary data directory."""
        with tempfile.TemporaryDirectory() as tmpdir:
            yield tmpdir

    @pytest.mark.asyncio
    async def test_works_with_encryption_disabled(self, data_dir):
        """CanonicalStore works normally when encryption is disabled."""
        from dbaas.entdb_server.apply.canonical_store import CanonicalStore

        store = CanonicalStore(data_dir=data_dir, encryption_config=EncryptionConfig())
        await store.initialize_tenant("t1")
        node = await store.create_node(
            tenant_id="t1",
            type_id=1,
            payload={"name": "Alice"},
            owner_actor="user:alice",
        )
        assert node.payload == {"name": "Alice"}
        store.close_all()

    @pytest.mark.asyncio
    async def test_works_with_encryption_enabled_no_sqlcipher(self, data_dir, caplog):
        """CanonicalStore with encryption enabled but no sqlcipher logs warning."""
        from dbaas.entdb_server.apply.canonical_store import CanonicalStore

        if is_sqlcipher_available():
            pytest.skip("sqlcipher is available -- fallback path not testable")

        enc_cfg = EncryptionConfig(enabled=True, master_key="test-master-key")
        with caplog.at_level(logging.WARNING):
            store = CanonicalStore(data_dir=data_dir, encryption_config=enc_cfg)
            await store.initialize_tenant("t2")
            node = await store.create_node(
                tenant_id="t2",
                type_id=1,
                payload={"name": "Bob"},
                owner_actor="user:bob",
            )
            assert node.payload == {"name": "Bob"}
            store.close_all()

        # Should have warned about missing sqlcipher
        assert any("SQLCipher not available" in r.message for r in caplog.records)

    @pytest.mark.asyncio
    async def test_different_tenants_get_different_connections(self, data_dir):
        """Each tenant gets its own database file, even with encryption."""
        from dbaas.entdb_server.apply.canonical_store import CanonicalStore

        enc_cfg = EncryptionConfig(enabled=True, master_key="test-key")
        store = CanonicalStore(data_dir=data_dir, encryption_config=enc_cfg)
        await store.initialize_tenant("t-alpha")
        await store.initialize_tenant("t-beta")

        # Each tenant should have its own file
        assert (Path(data_dir) / "tenant_t-alpha.db").exists()
        assert (Path(data_dir) / "tenant_t-beta.db").exists()
        store.close_all()

    def test_encryption_config_defaults_to_disabled(self):
        """CanonicalStore without encryption_config defaults to disabled."""
        with tempfile.TemporaryDirectory() as tmpdir:
            from dbaas.entdb_server.apply.canonical_store import CanonicalStore

            store = CanonicalStore(data_dir=tmpdir)
            assert store.encryption_config.enabled is False
            store.close_all()


# ── GlobalStore with encryption ──────────────────────────────────────


class TestGlobalStoreEncryption:
    """Tests for GlobalStore behavior with encryption config."""

    def test_global_store_default_no_encryption(self):
        """GlobalStore without encryption_config defaults to disabled."""
        from dbaas.entdb_server.global_store import GlobalStore

        with tempfile.TemporaryDirectory() as tmpdir:
            store = GlobalStore(data_dir=tmpdir)
            assert store.encryption_config.enabled is False
            store.close()

    def test_global_store_with_encryption_enabled_no_sqlcipher(self, caplog):
        """GlobalStore with encryption but no sqlcipher logs warning."""
        from dbaas.entdb_server.global_store import GlobalStore

        if is_sqlcipher_available():
            pytest.skip("sqlcipher is available -- fallback path not testable")

        enc_cfg = EncryptionConfig(enabled=True, master_key="global-master")
        with tempfile.TemporaryDirectory() as tmpdir:
            with caplog.at_level(logging.WARNING):
                store = GlobalStore(data_dir=tmpdir, encryption_config=enc_cfg)
                # The store should still be usable
                assert (Path(tmpdir) / "global.db").exists()
                store.close()

            assert any("SQLCipher not available" in r.message for r in caplog.records)


# ── Crypto-shred tests ──────────────────────────────────────────────


class TestCryptoShred:
    """Tests for tenant crypto-shredding (file deletion)."""

    def test_shred_removes_db_file(self):
        """shred_tenant removes the tenant .db file."""
        with tempfile.TemporaryDirectory() as tmpdir:
            db_file = Path(tmpdir) / "tenant_t1.db"
            db_file.write_text("dummy data")
            assert db_file.exists()

            result = shred_tenant(tmpdir, "t1")
            assert result is True
            assert not db_file.exists()

    def test_shred_removes_wal_and_shm(self):
        """shred_tenant removes .db, -wal, and -shm files."""
        with tempfile.TemporaryDirectory() as tmpdir:
            for suffix in ("", "-wal", "-shm"):
                (Path(tmpdir) / f"tenant_t2.db{suffix}").write_text("data")

            result = shred_tenant(tmpdir, "t2")
            assert result is True
            for suffix in ("", "-wal", "-shm"):
                assert not (Path(tmpdir) / f"tenant_t2.db{suffix}").exists()

    def test_shred_nonexistent_returns_false(self):
        """shred_tenant returns False if no files exist for the tenant."""
        with tempfile.TemporaryDirectory() as tmpdir:
            result = shred_tenant(tmpdir, "no-such-tenant")
            assert result is False

    def test_shred_sanitizes_tenant_id(self):
        """shred_tenant sanitizes the tenant ID to prevent path traversal."""
        with tempfile.TemporaryDirectory() as tmpdir:
            # A malicious tenant_id with path traversal should be sanitized
            db_file = Path(tmpdir) / "tenant_etcpasswd.db"
            db_file.write_text("safe")

            result = shred_tenant(tmpdir, "../etc/passwd")
            # The sanitized ID is "etcpasswd", so it should match
            assert result is True
            assert not db_file.exists()
