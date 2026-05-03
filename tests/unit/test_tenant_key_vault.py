"""
Unit tests for ``TenantKeyVault`` — the SQLite-backed wrap-and-stash
store for per-tenant data-encryption keys (DEKs).

These tests pin the durability and tamper-resistance contracts that the
vault is the single source of truth for. Anything that breaks one of
these properties is a security-relevant regression — the test suite
should fail loudly before such a change merges.
"""

from __future__ import annotations

import sqlite3
from pathlib import Path

import pytest

from dbaas.entdb_server.crypto.tenant_key_vault import (
    TenantKeyAlreadyProvisionedError,
    TenantKeyVault,
    TenantKeyVaultError,
    TenantShreddedError,
)

_MASTER_A = b"\xa1" * 32
_MASTER_B = b"\xb2" * 32
_DEK = b"\x42" * 32


@pytest.fixture
def vault_path(tmp_path: Path) -> Path:
    return tmp_path / "vault.db"


@pytest.fixture
def vault(vault_path: Path) -> TenantKeyVault:
    return TenantKeyVault(str(vault_path), _MASTER_A)


# ══════════════════════════════════════════════════════════════════════
# 1. Round-trip
# ══════════════════════════════════════════════════════════════════════


class TestProvisionAndGet:
    def test_provision_then_get_returns_same_dek(self, vault: TenantKeyVault) -> None:
        vault.provision("acme", _DEK)
        assert vault.get("acme") == _DEK

    def test_get_unknown_tenant_raises_keyerror(self, vault: TenantKeyVault) -> None:
        with pytest.raises(KeyError, match="acme"):
            vault.get("acme")

    def test_provision_twice_raises(self, vault: TenantKeyVault) -> None:
        vault.provision("acme", _DEK)
        with pytest.raises(TenantKeyAlreadyProvisionedError):
            vault.provision("acme", b"\x99" * 32)

    def test_each_tenant_gets_independent_dek(self, vault: TenantKeyVault) -> None:
        vault.provision("acme", b"\x01" * 32)
        vault.provision("globex", b"\x02" * 32)
        assert vault.get("acme") == b"\x01" * 32
        assert vault.get("globex") == b"\x02" * 32

    def test_provision_rejects_wrong_dek_length(self, vault: TenantKeyVault) -> None:
        with pytest.raises(ValueError, match="32 bytes"):
            vault.provision("acme", b"\x01" * 31)


# ══════════════════════════════════════════════════════════════════════
# 2. Master-KEK validation
# ══════════════════════════════════════════════════════════════════════


class TestMasterKekValidation:
    def test_constructor_rejects_short_kek(self, vault_path: Path) -> None:
        with pytest.raises(ValueError, match="32 bytes"):
            TenantKeyVault(str(vault_path), b"\x00" * 16)

    def test_rewrap_rejects_short_kek(self, vault: TenantKeyVault) -> None:
        with pytest.raises(ValueError, match="32 bytes"):
            vault.rewrap_with_new_master(b"\x00" * 16)


# ══════════════════════════════════════════════════════════════════════
# 3. Shred semantics
# ══════════════════════════════════════════════════════════════════════


class TestShred:
    def test_shred_then_get_raises_shredded_error(self, vault: TenantKeyVault) -> None:
        vault.provision("acme", _DEK)
        assert vault.shred("acme") is True
        with pytest.raises(TenantShreddedError):
            vault.get("acme")

    def test_shred_is_idempotent(self, vault: TenantKeyVault) -> None:
        vault.provision("acme", _DEK)
        assert vault.shred("acme") is True
        # Second call returns True (row exists) but does not raise.
        assert vault.shred("acme") is True
        with pytest.raises(TenantShreddedError):
            vault.get("acme")

    def test_shred_unknown_tenant_returns_false(self, vault: TenantKeyVault) -> None:
        # No row → nothing to shred. We return False so the caller can
        # surface "shred-on-nonexistent-tenant" if that matters to them.
        assert vault.shred("never-existed") is False

    def test_provision_after_shred_raises(self, vault: TenantKeyVault) -> None:
        vault.provision("acme", _DEK)
        vault.shred("acme")
        # Re-provisioning a shredded tenant must NOT silently succeed —
        # otherwise a bug in tenant-creation could resurrect a shredded
        # tenant's row, defeating the durability guarantee.
        with pytest.raises(TenantShreddedError):
            vault.provision("acme", b"\x99" * 32)

    def test_shredded_row_is_durable_across_new_vault_instances(
        self, vault: TenantKeyVault, vault_path: Path
    ) -> None:
        vault.provision("acme", _DEK)
        vault.shred("acme")

        # Open the same SQLite file with a fresh vault — the shred must
        # be honoured. This is the property that makes the shred
        # actually meaningful for compliance: the data is unrecoverable
        # even after a process restart.
        vault2 = TenantKeyVault(str(vault_path), _MASTER_A)
        with pytest.raises(TenantShreddedError):
            vault2.get("acme")
        assert vault2.is_shredded("acme") is True

    def test_check_constraint_enforced(self, vault: TenantKeyVault) -> None:
        # The schema's CHECK constraint says wrapped_key NULL ↔
        # shredded_at non-NULL. Smoke-test it by trying to violate it
        # directly via SQL — if a future migration drops the CHECK we
        # want the test to scream.
        with sqlite3.connect(vault._db_path) as conn, pytest.raises(sqlite3.IntegrityError):
            conn.execute(
                "INSERT INTO tenant_keys (tenant_id, wrapped_key, "
                "created_at, shredded_at) VALUES (?, NULL, 0, NULL)",
                ("bogus",),
            )


# ══════════════════════════════════════════════════════════════════════
# 4. Tamper resistance — AAD binds wrap to tenant_id
# ══════════════════════════════════════════════════════════════════════


class TestTamperResistance:
    def test_swapped_tenant_id_fails_authentication(self, vault: TenantKeyVault) -> None:
        """A wrapped row associated with tenant A must not unwrap as B.

        AES-GCM AAD binds the ciphertext to ``tenant_id`` so an attacker
        with write access to the vault SQLite cannot move a wrapped DEK
        from one tenant row to another.
        """
        vault.provision("acme", _DEK)
        # Snapshot acme's wrapped_key and forge a 'globex' row pointing
        # at the same blob.
        with sqlite3.connect(vault._db_path) as conn:
            row = conn.execute(
                "SELECT wrapped_key FROM tenant_keys WHERE tenant_id = ?",
                ("acme",),
            ).fetchone()
            forged_blob = row[0]
            conn.execute(
                "INSERT INTO tenant_keys (tenant_id, wrapped_key, created_at) VALUES (?, ?, ?)",
                ("globex", forged_blob, 0),
            )

        # acme still works.
        assert vault.get("acme") == _DEK
        # globex blows up with an authentication failure rather than
        # silently returning acme's DEK.
        with pytest.raises(TenantKeyVaultError, match="authentication"):
            vault.get("globex")

    def test_byte_flip_in_wrapped_blob_is_detected(self, vault: TenantKeyVault) -> None:
        vault.provision("acme", _DEK)
        with sqlite3.connect(vault._db_path) as conn:
            row = conn.execute(
                "SELECT wrapped_key FROM tenant_keys WHERE tenant_id = ?",
                ("acme",),
            ).fetchone()
            blob = bytearray(row[0])
            # Flip a bit in the ciphertext region (skip the 12-byte nonce).
            blob[15] ^= 0x01
            conn.execute(
                "UPDATE tenant_keys SET wrapped_key = ? WHERE tenant_id = ?",
                (bytes(blob), "acme"),
            )
        # The vault doesn't cache rows; a fresh get re-reads and
        # re-unwraps, so the flip surfaces immediately.
        with pytest.raises(TenantKeyVaultError, match="authentication"):
            vault.get("acme")


# ══════════════════════════════════════════════════════════════════════
# 5. Master-KEK rotation rewraps rows in place
# ══════════════════════════════════════════════════════════════════════


class TestMasterRotation:
    def test_rewrap_keeps_dek_unchanged(self, vault: TenantKeyVault, vault_path: Path) -> None:
        vault.provision("acme", _DEK)
        vault.provision("globex", b"\xee" * 32)

        # Shred one tenant so we can verify it is skipped on rewrap.
        vault.shred("globex")

        rewrapped = vault.rewrap_with_new_master(_MASTER_B)
        # Active rows: 1. Shredded: 0 (skipped).
        assert rewrapped == 1

        # acme's DEK is unchanged from the caller's perspective.
        assert vault.get("acme") == _DEK
        # The shred is preserved — rotation does not resurrect a
        # tombstoned row.
        with pytest.raises(TenantShreddedError):
            vault.get("globex")

        # A brand-new vault opened with the OLD master must now fail to
        # unwrap (the persisted blob is wrapped under MASTER_B).
        old_vault = TenantKeyVault(str(vault_path), _MASTER_A)
        with pytest.raises(TenantKeyVaultError, match="authentication"):
            old_vault.get("acme")

        # And a fresh vault on the NEW master succeeds.
        new_vault = TenantKeyVault(str(vault_path), _MASTER_B)
        assert new_vault.get("acme") == _DEK


# ══════════════════════════════════════════════════════════════════════
# 6. list_tenant_ids honours include_shredded flag
# ══════════════════════════════════════════════════════════════════════


class TestListTenantIds:
    def test_default_excludes_shredded(self, vault: TenantKeyVault) -> None:
        vault.provision("a", b"\x01" * 32)
        vault.provision("b", b"\x02" * 32)
        vault.shred("a")
        assert vault.list_tenant_ids() == ["b"]

    def test_include_shredded_returns_all(self, vault: TenantKeyVault) -> None:
        vault.provision("a", b"\x01" * 32)
        vault.provision("b", b"\x02" * 32)
        vault.shred("a")
        assert sorted(vault.list_tenant_ids(include_shredded=True)) == ["a", "b"]


# ══════════════════════════════════════════════════════════════════════
# 7. Wrapped blob shape
# ══════════════════════════════════════════════════════════════════════


class TestWrappedBlobShape:
    def test_blob_is_60_bytes_for_32_byte_dek(self, vault: TenantKeyVault) -> None:
        # nonce (12) || ct (32) || tag (16) = 60 bytes.
        vault.provision("acme", _DEK)
        with sqlite3.connect(vault._db_path) as conn:
            row = conn.execute(
                "SELECT wrapped_key FROM tenant_keys WHERE tenant_id = ?",
                ("acme",),
            ).fetchone()
            assert len(row[0]) == 60

    def test_each_provision_uses_a_fresh_nonce(self, tmp_path: Path) -> None:
        # Write the same DEK for two different tenants — the wrapped
        # blobs must differ (random nonces). If both blobs were equal a
        # passive observer of the SQLite file could tell that two
        # tenants share a key.
        v = TenantKeyVault(str(tmp_path / "v.db"), _MASTER_A)
        v.provision("a", _DEK)
        v.provision("b", _DEK)
        with sqlite3.connect(v._db_path) as conn:
            blobs = [
                row[0]
                for row in conn.execute("SELECT wrapped_key FROM tenant_keys ORDER BY tenant_id")
            ]
        assert blobs[0] != blobs[1]
