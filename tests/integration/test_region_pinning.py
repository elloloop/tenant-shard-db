"""
End-to-end integration test for tenant region pinning.

Exercises the full flow: CreateTenant with a region → tenant persisted →
data-plane RPC against a server in a *different* region rejected with
FAILED_PRECONDITION → same tenant on a server in the *matching* region
passes the routing check.

Tests use a real GlobalStore (SQLite) plus a real EntDBServicer; the
canonical_store is mocked since this test is about routing, not graph
ops.
"""

from __future__ import annotations

import tempfile
from unittest.mock import AsyncMock, MagicMock

import grpc
import pytest

from dbaas.entdb_server.api.generated import (
    CreateTenantRequest,
    GetTenantRequest,
)
from dbaas.entdb_server.api.grpc_server import EntDBServicer
from dbaas.entdb_server.global_store import GlobalStore


@pytest.fixture
async def two_region_setup():
    """Yield a (global_store, us_servicer, eu_servicer) triple sharing one global registry."""
    with tempfile.TemporaryDirectory() as tmpdir:
        store = GlobalStore(tmpdir)

        canonical = MagicMock()
        canonical.initialize_tenant = AsyncMock()

        us_servicer = EntDBServicer(
            wal=MagicMock(),
            canonical_store=canonical,
            schema_registry=MagicMock(),
            global_store=store,
            served_region="us-east-1",
        )
        eu_servicer = EntDBServicer(
            wal=MagicMock(),
            canonical_store=canonical,
            schema_registry=MagicMock(),
            global_store=store,
            served_region="eu-west-1",
        )

        yield store, us_servicer, eu_servicer
        store.close()


@pytest.mark.integration
class TestRegionPinningE2E:
    @pytest.mark.asyncio
    async def test_create_tenant_persists_region(self, two_region_setup):
        """CreateTenant accepts region and the value reaches the global store."""
        _, us_servicer, _ = two_region_setup

        ctx = AsyncMock()
        ctx.abort = AsyncMock()
        resp = await us_servicer.CreateTenant(
            CreateTenantRequest(
                actor="user:alice",
                tenant_id="t-eu",
                name="EU Co",
                region="eu-west-1",
            ),
            ctx,
        )

        assert resp.success is True
        assert resp.tenant.region == "eu-west-1"

    @pytest.mark.asyncio
    async def test_create_tenant_empty_region_defaults_to_servers_region(self, two_region_setup):
        """Empty region in CreateTenantRequest defaults to the server's served_region."""
        _, us_servicer, eu_servicer = two_region_setup

        ctx = AsyncMock()
        ctx.abort = AsyncMock()
        await us_servicer.CreateTenant(
            CreateTenantRequest(actor="user:alice", tenant_id="t-us", name="US Co"),
            ctx,
        )
        await eu_servicer.CreateTenant(
            CreateTenantRequest(actor="user:bob", tenant_id="t-eu", name="EU Co"),
            ctx,
        )

        # Read back via the EU server (registry is global, GetTenant doesn't
        # go through _check_tenant).
        us_resp = await eu_servicer.GetTenant(
            GetTenantRequest(actor="user:alice", tenant_id="t-us"), ctx
        )
        eu_resp = await eu_servicer.GetTenant(
            GetTenantRequest(actor="user:bob", tenant_id="t-eu"), ctx
        )
        assert us_resp.tenant.region == "us-east-1"
        assert eu_resp.tenant.region == "eu-west-1"

    @pytest.mark.asyncio
    async def test_data_plane_rejected_for_wrong_region(self, two_region_setup):
        """A data-plane request for an EU tenant against a US server is rejected."""
        _, us_servicer, _ = two_region_setup

        ctx = AsyncMock()
        ctx.abort = AsyncMock()
        await us_servicer.CreateTenant(
            CreateTenantRequest(
                actor="user:alice",
                tenant_id="t-eu",
                name="EU Co",
                region="eu-west-1",
            ),
            ctx,
        )

        # Now hit _check_tenant directly with the EU tenant on the US servicer.
        ctx2 = AsyncMock()
        ctx2.abort = AsyncMock()
        await us_servicer._check_tenant("t-eu", ctx2)

        ctx2.abort.assert_awaited_once()
        code, msg = ctx2.abort.call_args[0]
        assert code == grpc.StatusCode.FAILED_PRECONDITION
        assert "eu-west-1" in msg
        assert "us-east-1" in msg

    @pytest.mark.asyncio
    async def test_data_plane_accepted_in_matching_region(self, two_region_setup):
        """The same tenant routed to its own region passes _check_tenant."""
        _, us_servicer, eu_servicer = two_region_setup

        ctx = AsyncMock()
        ctx.abort = AsyncMock()
        await us_servicer.CreateTenant(
            CreateTenantRequest(
                actor="user:alice",
                tenant_id="t-eu",
                name="EU Co",
                region="eu-west-1",
            ),
            ctx,
        )

        ctx2 = AsyncMock()
        ctx2.abort = AsyncMock()
        await eu_servicer._check_tenant("t-eu", ctx2)
        ctx2.abort.assert_not_awaited()
