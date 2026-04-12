"""
Unit tests for the EntDB encryption-at-rest crypto module.

Tests cover:
- KeyManager: derivation determinism, uniqueness, rotation, shred
- open_encrypted_db: fallback mode and PRAGMA application
- crypto_shred_tenant: key destruction and file deletion
- HKDF correctness
- Input validation and edge cases
- Thread-safety of concurrent key derivations
"""

from __future__ import annotations

import os
import tempfile
import threading
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

from dbaas.entdb_server.crypto.crypto_shred import crypto_shred_tenant
from dbaas.entdb_server.crypto.encrypted_connection import (
    is_sqlcipher_available,
    open_encrypted_db,
)
from dbaas.entdb_server.crypto.key_manager import KeyManager

# A valid 32-byte master key (64 hex chars)
MASTER_KEY_HEX = "a" * 64
MASTER_KEY_BYTES = bytes.fromhex(MASTER_KEY_HEX)


# ---------------------------------------------------------------------------
# KeyManager: derivation determinism
# ---------------------------------------------------------------------------


class TestKeyManagerDerivation:
    """Tests for KeyManager.derive_tenant_key determinism and uniqueness."""

    def test_derive_tenant_key_is_deterministic(self):
        """Same master key and tenant ID always produce the same derived key."""
        km = KeyManager(MASTER_KEY_BYTES)
        k1 = km.derive_tenant_key("tenant-abc")
        k2 = km.derive_tenant_key("tenant-abc")
        assert k1 == k2

    def test_different_tenant_ids_produce_different_keys(self):
        """Different tenant IDs produce different derived keys."""
        km = KeyManager(MASTER_KEY_BYTES)
        ka = km.derive_tenant_key("tenant-a")
        kb = km.derive_tenant_key("tenant-b")
        assert ka != kb

    def test_derived_key_is_32_bytes(self):
        """Derived keys are 32 bytes (256 bits)."""
        km = KeyManager(MASTER_KEY_BYTES)
        key = km.derive_tenant_key("tenant-1")
        assert len(key) == 32
        assert isinstance(key, bytes)

    def test_hex_string_master_key_accepted(self):
        """KeyManager accepts a hex-encoded master key string."""
        km = KeyManager(MASTER_KEY_HEX)
        key = km.derive_tenant_key("tenant-x")
        assert len(key) == 32

    def test_hex_and_bytes_master_key_produce_same_result(self):
        """Hex-string and bytes master key produce identical derivations."""
        km_hex = KeyManager(MASTER_KEY_HEX)
        km_bytes = KeyManager(MASTER_KEY_BYTES)
        assert km_hex.derive_tenant_key("t1") == km_bytes.derive_tenant_key("t1")

    def test_special_characters_in_tenant_id(self):
        """Tenant IDs with special characters produce valid 32-byte keys."""
        km = KeyManager(MASTER_KEY_BYTES)
        key = km.derive_tenant_key("org/proj:tenant@special#1")
        assert len(key) == 32

    def test_empty_tenant_id_produces_valid_key(self):
        """An empty tenant ID still produces a valid derived key."""
        km = KeyManager(MASTER_KEY_BYTES)
        key = km.derive_tenant_key("")
        assert len(key) == 32


# ---------------------------------------------------------------------------
# KeyManager: HKDF correctness
# ---------------------------------------------------------------------------


class TestKeyManagerHKDF:
    """Verify HKDF is used correctly under the hood."""

    def test_derivation_uses_hkdf_sha256(self):
        """The derivation path uses HKDF with SHA-256."""
        from cryptography.hazmat.primitives.hashes import SHA256
        from cryptography.hazmat.primitives.kdf.hkdf import HKDF

        info = b"entdb-tenant-key:tenant-hkdf-test"
        hkdf = HKDF(algorithm=SHA256(), length=32, salt=None, info=info)
        expected = hkdf.derive(MASTER_KEY_BYTES)

        km = KeyManager(MASTER_KEY_BYTES)
        actual = km.derive_tenant_key("tenant-hkdf-test")
        assert actual == expected

    def test_different_master_keys_produce_different_tenant_keys(self):
        """Two different master keys yield different derived keys for the same tenant."""
        km1 = KeyManager(b"\x01" * 32)
        km2 = KeyManager(b"\x02" * 32)
        assert km1.derive_tenant_key("t") != km2.derive_tenant_key("t")


# ---------------------------------------------------------------------------
# KeyManager: input validation
# ---------------------------------------------------------------------------


class TestKeyManagerValidation:
    """Tests for master key validation."""

    def test_empty_master_key_string_raises(self):
        """An empty master key string raises ValueError."""
        with pytest.raises(ValueError, match="must not be empty"):
            KeyManager("")

    def test_invalid_hex_string_raises(self):
        """A non-hex master key string raises ValueError."""
        with pytest.raises(ValueError, match="valid hex"):
            KeyManager("not-hex-at-all")

    def test_wrong_length_bytes_raises(self):
        """A master key that is not 32 bytes raises ValueError."""
        with pytest.raises(ValueError, match="32 bytes"):
            KeyManager(b"\x00" * 16)

    def test_wrong_length_hex_raises(self):
        """A hex string that decodes to wrong length raises ValueError."""
        with pytest.raises(ValueError, match="32 bytes"):
            KeyManager("aa" * 16)  # 16 bytes, not 32


# ---------------------------------------------------------------------------
# KeyManager: master key rotation
# ---------------------------------------------------------------------------


class TestKeyManagerRotation:
    """Tests for master key rotation."""

    def test_rotation_produces_new_tenant_keys(self):
        """After rotation, the same tenant ID yields a different derived key."""
        km = KeyManager(MASTER_KEY_BYTES)
        old_key = km.derive_tenant_key("tenant-rot")

        new_master = b"\xbb" * 32
        rotation_map = km.rotate_master_key(MASTER_KEY_BYTES, new_master)

        new_key = km.derive_tenant_key("tenant-rot")
        assert old_key != new_key
        assert rotation_map["tenant-rot"] == (old_key, new_key)

    def test_rotation_wrong_old_key_raises(self):
        """Rotation with incorrect old_key raises ValueError."""
        km = KeyManager(MASTER_KEY_BYTES)
        with pytest.raises(ValueError, match="does not match"):
            km.rotate_master_key(b"\xff" * 32, b"\xcc" * 32)

    def test_rotation_invalid_new_key_raises(self):
        """Rotation with a new_key of wrong length raises ValueError."""
        km = KeyManager(MASTER_KEY_BYTES)
        with pytest.raises(ValueError, match="32 bytes"):
            km.rotate_master_key(MASTER_KEY_BYTES, b"\x00" * 16)

    def test_rotation_skips_shredded_tenants(self):
        """Rotation does not re-derive keys for shredded tenants."""
        km = KeyManager(MASTER_KEY_BYTES)
        km.derive_tenant_key("t-keep")
        km.derive_tenant_key("t-shred")
        km.shred_tenant("t-shred")

        new_master = b"\xcc" * 32
        rotation_map = km.rotate_master_key(MASTER_KEY_BYTES, new_master)
        assert "t-keep" in rotation_map
        assert "t-shred" not in rotation_map


# ---------------------------------------------------------------------------
# KeyManager: shred
# ---------------------------------------------------------------------------


class TestKeyManagerShred:
    """Tests for crypto-shred of key material."""

    def test_shred_removes_cached_key(self):
        """After shredding, the tenant key is no longer in the cache."""
        km = KeyManager(MASTER_KEY_BYTES)
        km.derive_tenant_key("t-bye")
        assert "t-bye" in km.cached_tenant_ids

        km.shred_tenant("t-bye")
        assert "t-bye" not in km.cached_tenant_ids

    def test_shred_blocks_future_derivation(self):
        """After shredding, derive_tenant_key raises RuntimeError."""
        km = KeyManager(MASTER_KEY_BYTES)
        km.derive_tenant_key("t-gone")
        km.shred_tenant("t-gone")

        with pytest.raises(RuntimeError, match="crypto-shredded"):
            km.derive_tenant_key("t-gone")

    def test_is_shredded(self):
        """is_shredded returns True only after shredding."""
        km = KeyManager(MASTER_KEY_BYTES)
        assert not km.is_shredded("t-x")
        km.shred_tenant("t-x")
        assert km.is_shredded("t-x")

    def test_shred_idempotent(self):
        """Calling shred_tenant twice does not raise."""
        km = KeyManager(MASTER_KEY_BYTES)
        km.shred_tenant("t-double")
        km.shred_tenant("t-double")  # no error
        assert km.is_shredded("t-double")


# ---------------------------------------------------------------------------
# KeyManager: concurrent key derivations
# ---------------------------------------------------------------------------


class TestKeyManagerConcurrency:
    """Tests for thread-safety of concurrent key derivations."""

    def test_concurrent_derivations_are_safe(self):
        """Multiple threads deriving keys concurrently produce consistent results."""
        km = KeyManager(MASTER_KEY_BYTES)
        results: dict[str, bytes] = {}
        errors: list[Exception] = []

        def derive(tid: str) -> None:
            try:
                results[tid] = km.derive_tenant_key(tid)
            except Exception as exc:
                errors.append(exc)

        threads = [threading.Thread(target=derive, args=(f"tenant-{i}",)) for i in range(20)]
        for t in threads:
            t.start()
        for t in threads:
            t.join()

        assert not errors
        assert len(results) == 20

        # Verify determinism: re-derive in the main thread
        for tid, key in results.items():
            assert km.derive_tenant_key(tid) == key


# ---------------------------------------------------------------------------
# open_encrypted_db: fallback mode
# ---------------------------------------------------------------------------


class TestOpenEncryptedDbFallback:
    """Tests for open_encrypted_db when SQLCipher is NOT available."""

    def test_fallback_returns_usable_connection(self):
        """Without SQLCipher, returns a standard sqlite3 connection that works."""
        with tempfile.TemporaryDirectory() as tmpdir:
            db_path = os.path.join(tmpdir, "test.db")
            conn = open_encrypted_db(db_path, MASTER_KEY_BYTES)
            conn.execute("CREATE TABLE t (id TEXT, val INTEGER)")
            conn.execute("INSERT INTO t VALUES ('a', 1)")
            row = conn.execute("SELECT val FROM t WHERE id='a'").fetchone()
            assert row[0] == 1
            conn.close()

    def test_fallback_logs_warning(self, caplog):
        """Fallback mode logs a warning about missing SQLCipher."""
        if is_sqlcipher_available():
            pytest.skip("SQLCipher is available; fallback path not testable")

        import logging

        with tempfile.TemporaryDirectory() as tmpdir:
            db_path = os.path.join(tmpdir, "warn.db")
            with caplog.at_level(
                logging.WARNING,
                logger="dbaas.entdb_server.crypto.encrypted_connection",
            ):
                conn = open_encrypted_db(db_path, MASTER_KEY_BYTES)
                conn.close()

            assert any("SQLCipher not available" in r.message for r in caplog.records)


# ---------------------------------------------------------------------------
# open_encrypted_db: PRAGMA key with mock sqlcipher
# ---------------------------------------------------------------------------


class TestOpenEncryptedDbPragma:
    """Tests that PRAGMA key is applied when sqlcipher is available."""

    def test_pragma_key_applied_with_mock_sqlcipher(self):
        """When sqlcipher module is present, PRAGMA key is executed."""
        mock_conn = MagicMock()
        mock_module = MagicMock()
        mock_module.connect.return_value = mock_conn

        with (
            patch(
                "dbaas.entdb_server.crypto.encrypted_connection._sqlcipher_available",
                True,
            ),
            patch(
                "dbaas.entdb_server.crypto.encrypted_connection._sqlcipher_module",
                mock_module,
            ),
        ):
            open_encrypted_db("/tmp/fake.db", MASTER_KEY_BYTES)

        mock_module.connect.assert_called_once()
        # Check that PRAGMA key was executed with the hex key
        calls = [str(c) for c in mock_conn.execute.call_args_list]
        pragma_key_call = calls[0]
        assert "PRAGMA key" in pragma_key_call
        assert MASTER_KEY_BYTES.hex() in pragma_key_call

    def test_pragma_kdf_iter_applied(self):
        """When sqlcipher is available, PRAGMA kdf_iter is also set."""
        mock_conn = MagicMock()
        mock_module = MagicMock()
        mock_module.connect.return_value = mock_conn

        with (
            patch(
                "dbaas.entdb_server.crypto.encrypted_connection._sqlcipher_available",
                True,
            ),
            patch(
                "dbaas.entdb_server.crypto.encrypted_connection._sqlcipher_module",
                mock_module,
            ),
        ):
            open_encrypted_db("/tmp/fake.db", MASTER_KEY_BYTES, kdf_iterations=100_000)

        calls = [str(c) for c in mock_conn.execute.call_args_list]
        assert any("kdf_iter" in c for c in calls)


# ---------------------------------------------------------------------------
# crypto_shred_tenant
# ---------------------------------------------------------------------------


class TestCryptoShredTenant:
    """Tests for the crypto_shred_tenant async function."""

    @pytest.mark.asyncio
    async def test_shred_calls_key_manager_shred(self):
        """crypto_shred_tenant calls key_manager.shred_tenant."""
        km = KeyManager(MASTER_KEY_BYTES)
        km.derive_tenant_key("t-shred-test")

        result = await crypto_shred_tenant("t-shred-test", km)

        assert result["tenant_id"] == "t-shred-test"
        assert result["key_destroyed"] is True
        assert km.is_shredded("t-shred-test")

    @pytest.mark.asyncio
    async def test_shred_with_file_deletion(self):
        """crypto_shred_tenant deletes db files when delete_files=True."""
        km = KeyManager(MASTER_KEY_BYTES)
        km.derive_tenant_key("t-del")

        with tempfile.TemporaryDirectory() as tmpdir:
            # Create fake db files
            for suffix in ("", "-wal", "-shm"):
                (Path(tmpdir) / f"tenant_t-del.db{suffix}").write_text("data")

            result = await crypto_shred_tenant("t-del", km, delete_files=True, data_dir=tmpdir)

            assert result["key_destroyed"] is True
            assert result["files_deleted"] is True
            assert len(result["files_found"]) == 3

            # Files should be gone
            for suffix in ("", "-wal", "-shm"):
                assert not (Path(tmpdir) / f"tenant_t-del.db{suffix}").exists()

    @pytest.mark.asyncio
    async def test_shred_without_file_deletion(self):
        """crypto_shred_tenant with delete_files=False keeps files."""
        km = KeyManager(MASTER_KEY_BYTES)

        with tempfile.TemporaryDirectory() as tmpdir:
            db_file = Path(tmpdir) / "tenant_t-keep-file.db"
            db_file.write_text("data")

            result = await crypto_shred_tenant("t-keep-file", km)

            assert result["key_destroyed"] is True
            assert result["files_deleted"] is False
            assert db_file.exists()  # file still there

    @pytest.mark.asyncio
    async def test_shred_closes_canonical_store_connection(self):
        """crypto_shred_tenant calls close_tenant on the canonical store."""
        km = KeyManager(MASTER_KEY_BYTES)
        mock_store = MagicMock()
        mock_store.close_tenant = MagicMock()

        await crypto_shred_tenant("t-close", km, canonical_store=mock_store)

        mock_store.close_tenant.assert_called_once_with("t-close")

    @pytest.mark.asyncio
    async def test_shred_idempotent(self):
        """Calling crypto_shred_tenant twice for the same tenant is safe."""
        km = KeyManager(MASTER_KEY_BYTES)
        r1 = await crypto_shred_tenant("t-idem", km)
        r2 = await crypto_shred_tenant("t-idem", km)
        assert r1["key_destroyed"] is True
        assert r2["key_destroyed"] is True


# ---------------------------------------------------------------------------
# reencrypt_tenant_db
# ---------------------------------------------------------------------------


class TestReencryptTenantDb:
    """Tests for the static reencrypt_tenant_db helper."""

    def test_reencrypt_without_sqlcipher_logs_warning(self, caplog):
        """Without SQLCipher, reencrypt logs a warning and is a no-op."""
        import logging

        with caplog.at_level(logging.WARNING, logger="dbaas.entdb_server.crypto.key_manager"):
            KeyManager.reencrypt_tenant_db("/tmp/fake.db", MASTER_KEY_BYTES, b"\xcc" * 32)

        assert any("SQLCipher not available" in r.message for r in caplog.records)

    def test_reencrypt_with_mock_sqlcipher(self):
        """With sqlcipher available, PRAGMA key and rekey are executed."""
        import sys

        mock_conn = MagicMock()
        mock_sqlcipher = MagicMock()
        mock_sqlcipher.connect.return_value = mock_conn

        old_key = MASTER_KEY_BYTES
        new_key = b"\xdd" * 32

        # `from pysqlcipher3 import dbapi2` resolves via the parent
        # module's attribute, so set it explicitly.
        parent_mock = MagicMock()
        parent_mock.dbapi2 = mock_sqlcipher

        sys.modules["pysqlcipher3"] = parent_mock
        sys.modules["pysqlcipher3.dbapi2"] = mock_sqlcipher
        try:
            KeyManager.reencrypt_tenant_db("/tmp/test.db", old_key, new_key)
            mock_sqlcipher.connect.assert_called_once_with("/tmp/test.db")
            calls = [str(c) for c in mock_conn.execute.call_args_list]
            assert any("PRAGMA key" in c for c in calls)
            assert any("PRAGMA rekey" in c for c in calls)
        finally:
            del sys.modules["pysqlcipher3"]
            del sys.modules["pysqlcipher3.dbapi2"]
