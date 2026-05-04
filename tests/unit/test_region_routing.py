"""
Region pinning rejection in EntDBServicer._check_tenant.

When a tenant is pinned to region X but the request lands on a server
configured to serve region Y, _check_tenant must abort with
FAILED_PRECONDITION (the mismatch is permanent for this node, not a
transient unavailability).

Single-region back-compat: if the server has no served_region configured
(None), region-based rejection must not fire — existing single-region
deployments keep working.
"""

from __future__ import annotations

import tempfile
from unittest.mock import AsyncMock, MagicMock

import grpc
import pytest

from dbaas.entdb_server.api.grpc_server import EntDBServicer
from dbaas.entdb_server.global_store import GlobalStore


@pytest.fixture
async def global_store_with_tenants():
    with tempfile.TemporaryDirectory() as tmpdir:
        store = GlobalStore(tmpdir)
        await store.create_tenant("t-us", "US Co", region="us-east-1")
        await store.create_tenant("t-eu", "EU Co", region="eu-west-1")
        yield store
        store.close()


def _make_servicer(global_store, served_region: str | None) -> EntDBServicer:
    return EntDBServicer(
        wal=MagicMock(),
        canonical_store=MagicMock(),
        schema_registry=MagicMock(),
        global_store=global_store,
        served_region=served_region,
    )


class TestRegionRejection:
    @pytest.mark.asyncio
    async def test_rejects_tenant_from_other_region(self, global_store_with_tenants):
        """Server pinned to us-east-1 must reject tenants pinned to eu-west-1."""
        servicer = _make_servicer(global_store_with_tenants, served_region="us-east-1")
        context = AsyncMock()
        context.abort = AsyncMock()

        await servicer._check_tenant("t-eu", context)

        context.abort.assert_awaited_once()
        code, msg = context.abort.call_args[0]
        assert code == grpc.StatusCode.FAILED_PRECONDITION
        assert "region" in msg.lower()
        assert "eu-west-1" in msg
        assert "us-east-1" in msg

    @pytest.mark.asyncio
    async def test_allows_tenant_in_same_region(self, global_store_with_tenants):
        """Server pinned to us-east-1 must allow tenants pinned to us-east-1."""
        servicer = _make_servicer(global_store_with_tenants, served_region="us-east-1")
        context = AsyncMock()
        context.abort = AsyncMock()

        await servicer._check_tenant("t-us", context)

        context.abort.assert_not_awaited()

    @pytest.mark.asyncio
    async def test_no_served_region_accepts_all(self, global_store_with_tenants):
        """When served_region is None, region check is disabled (single-region back-compat)."""
        servicer = _make_servicer(global_store_with_tenants, served_region=None)
        context = AsyncMock()
        context.abort = AsyncMock()

        await servicer._check_tenant("t-eu", context)
        await servicer._check_tenant("t-us", context)

        context.abort.assert_not_awaited()

    @pytest.mark.asyncio
    async def test_unknown_tenant_does_not_crash_region_check(self, global_store_with_tenants):
        """Region check must not blow up on an unknown tenant.

        Authn/authz checks downstream are responsible for rejecting unknown
        tenants — _check_tenant only enforces routing, so it should pass
        through cleanly when the tenant lookup returns None.
        """
        servicer = _make_servicer(global_store_with_tenants, served_region="us-east-1")
        context = AsyncMock()
        context.abort = AsyncMock()

        await servicer._check_tenant("does-not-exist", context)

        context.abort.assert_not_awaited()
