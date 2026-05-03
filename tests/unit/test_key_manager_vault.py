"""
Unit tests for ``KeyManager`` running in vault mode.

These tests pin the integration contract between ``KeyManager`` and
``TenantKeyVault``:

- The migration path on first vault access seeds the row with the
  legacy HKDF-derived DEK so existing SQLite tenant files keep
  opening with the same key after the operator turns the vault on.
- Crypto-shred is durable across ``KeyManager`` instances (i.e.
  process restarts) when the vault is configured.
- Master-key rotation in vault mode rewraps rows in place rather than
  changing per-tenant DEKs, so SQLite tenant files do NOT need
  re-encryption.
- Derived-mode behaviour (no vault) is unchanged — backward
  compatibility for existing deployments.
"""

from __future__ import annotations

from pathlib import Path

import pytest

from dbaas.entdb_server.crypto.key_manager import (
    CryptoShreddedError,
    KeyManager,
)
from dbaas.entdb_server.crypto.tenant_key_vault import TenantKeyVault

_MASTER_A = b"\xa1" * 32
_MASTER_B = b"\xb2" * 32


@pytest.fixture
def vault_path(tmp_path: Path) -> Path:
    return tmp_path / "vault.db"


# ══════════════════════════════════════════════════════════════════════
# 1. Migration safety — first access seeds with legacy HKDF-derived key
# ══════════════════════════════════════════════════════════════════════


class TestMigrationSeed:
    def test_first_vault_access_returns_legacy_dek(self, vault_path: Path) -> None:
        """The DEK returned on first vault access must equal the legacy
        HKDF-derived DEK so existing tenant SQLite files keep opening.
        """
        # 1. Capture what the legacy (no-vault) KeyManager would have
        #    produced for this tenant.
        legacy = KeyManager(_MASTER_A).derive_tenant_key("acme")

        # 2. Now enable the vault and access the same tenant for the
        #    first time. The DEK must match exactly — the migration
        #    seeds the vault row with the legacy derivation.
        vault = TenantKeyVault(str(vault_path), _MASTER_A)
        km = KeyManager(_MASTER_A, vault=vault)
        assert km.derive_tenant_key("acme") == legacy

    def test_provisioned_row_is_persisted(self, vault_path: Path) -> None:
        vault = TenantKeyVault(str(vault_path), _MASTER_A)
        km = KeyManager(_MASTER_A, vault=vault)
        dek = km.derive_tenant_key("acme")

        # A second KeyManager + Vault instance opening the same path
        # returns the same DEK (i.e. the row was written, not just
        # cached in the first KeyManager's memory).
        vault2 = TenantKeyVault(str(vault_path), _MASTER_A)
        km2 = KeyManager(_MASTER_A, vault=vault2)
        assert km2.derive_tenant_key("acme") == dek


# ══════════════════════════════════════════════════════════════════════
# 2. Durable shred — survives a fresh KeyManager
# ══════════════════════════════════════════════════════════════════════


class TestDurableShred:
    def test_shred_persists_across_new_keymanager(self, vault_path: Path) -> None:
        vault = TenantKeyVault(str(vault_path), _MASTER_A)
        km = KeyManager(_MASTER_A, vault=vault)
        km.derive_tenant_key("acme")
        km.shred_tenant("acme")

        # Fresh KeyManager + Vault — simulates a process restart.
        vault2 = TenantKeyVault(str(vault_path), _MASTER_A)
        km2 = KeyManager(_MASTER_A, vault=vault2)
        with pytest.raises(CryptoShreddedError, match="acme"):
            km2.derive_tenant_key("acme")
        assert km2.is_shredded("acme") is True

    def test_shred_unaccessed_tenant_seeds_then_tombstones(self, vault_path: Path) -> None:
        """Shredding a tenant that was never accessed in this process
        must still leave a durable tombstone — otherwise an attacker
        with write access to the SQLite file could revert state and
        re-derive the DEK.
        """
        vault = TenantKeyVault(str(vault_path), _MASTER_A)
        km = KeyManager(_MASTER_A, vault=vault)
        km.shred_tenant("never-accessed")

        # New process. Even though 'never-accessed' was never
        # explicitly accessed before shred, the shred must hold.
        vault2 = TenantKeyVault(str(vault_path), _MASTER_A)
        km2 = KeyManager(_MASTER_A, vault=vault2)
        with pytest.raises(CryptoShreddedError):
            km2.derive_tenant_key("never-accessed")

    def test_cryptoshredded_error_is_runtimeerror_subclass(self) -> None:
        # Backward compatibility: existing callers do
        # `except RuntimeError:` against the old in-memory shred.
        # CryptoShreddedError must remain catchable that way.
        assert issubclass(CryptoShreddedError, RuntimeError)


# ══════════════════════════════════════════════════════════════════════
# 3. Master-KEK rotation rewraps in place
# ══════════════════════════════════════════════════════════════════════


class TestMasterRotation:
    def test_rotation_in_vault_mode_keeps_dek_stable(self, vault_path: Path) -> None:
        vault = TenantKeyVault(str(vault_path), _MASTER_A)
        km = KeyManager(_MASTER_A, vault=vault)
        original_dek = km.derive_tenant_key("acme")

        rotation_map = km.rotate_master_key(_MASTER_A, _MASTER_B)
        # In vault mode the per-tenant DEK is unchanged, so the
        # returned map is empty (no SQLite rekey needed).
        assert rotation_map == {}

        # The same KeyManager continues to return the original DEK.
        assert km.derive_tenant_key("acme") == original_dek

        # A fresh KeyManager opened with the NEW master also returns
        # the same DEK — the rewrap landed in the persisted vault.
        vault2 = TenantKeyVault(str(vault_path), _MASTER_B)
        km2 = KeyManager(_MASTER_B, vault=vault2)
        assert km2.derive_tenant_key("acme") == original_dek

    def test_rotation_in_derived_mode_changes_deks(self) -> None:
        # Without a vault, rotation re-derives keys (legacy behaviour).
        # Pinned here so a future refactor doesn't accidentally collapse
        # both modes into the rewrap path — derived mode has no row to
        # rewrap, so it MUST go via re-derivation.
        km = KeyManager(_MASTER_A)
        old = km.derive_tenant_key("acme")
        rmap = km.rotate_master_key(_MASTER_A, _MASTER_B)
        assert "acme" in rmap
        old_returned, new_returned = rmap["acme"]
        assert old_returned == old
        assert new_returned != old
        assert km.derive_tenant_key("acme") == new_returned


# ══════════════════════════════════════════════════════════════════════
# 4. Tampered vault row → typed error to derive_tenant_key
# ══════════════════════════════════════════════════════════════════════


class TestCryptoShredOrchestrator:
    """End-to-end via the public crypto_shred_tenant orchestrator.

    The orchestrator is what gdpr_worker / admin handlers call. This
    test confirms vault-mode wiring all the way through that public
    entry point — not just the lower-level KeyManager API.
    """

    @pytest.mark.asyncio
    async def test_orchestrator_durable_shred_in_vault_mode(self, vault_path: Path) -> None:
        from dbaas.entdb_server.crypto.crypto_shred import (
            crypto_shred_tenant,
        )

        vault = TenantKeyVault(str(vault_path), _MASTER_A)
        km = KeyManager(_MASTER_A, vault=vault)
        km.derive_tenant_key("acme")

        result = await crypto_shred_tenant("acme", km)
        assert result["key_destroyed"] is True

        # Spin up a fresh KeyManager + Vault — process restart proxy.
        vault2 = TenantKeyVault(str(vault_path), _MASTER_A)
        km2 = KeyManager(_MASTER_A, vault=vault2)
        with pytest.raises(CryptoShreddedError):
            km2.derive_tenant_key("acme")


class TestTamperedVaultRow:
    def test_swapped_row_surfaces_authentication_error(self, vault_path: Path) -> None:
        import sqlite3

        vault = TenantKeyVault(str(vault_path), _MASTER_A)
        km = KeyManager(_MASTER_A, vault=vault)
        km.derive_tenant_key("acme")

        # Forge a 'globex' row pointing at acme's wrapped blob.
        with sqlite3.connect(str(vault_path)) as conn:
            row = conn.execute(
                "SELECT wrapped_key FROM tenant_keys WHERE tenant_id = ?",
                ("acme",),
            ).fetchone()
            conn.execute(
                "INSERT INTO tenant_keys (tenant_id, wrapped_key, created_at) VALUES (?, ?, 0)",
                ("globex", row[0]),
            )

        # KeyManager surfaces the authentication failure rather than
        # silently returning acme's DEK as globex's.
        from dbaas.entdb_server.crypto.tenant_key_vault import (
            TenantKeyVaultError,
        )

        with pytest.raises(TenantKeyVaultError, match="authentication"):
            km.derive_tenant_key("globex")
