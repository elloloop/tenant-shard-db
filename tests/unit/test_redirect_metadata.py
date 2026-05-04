"""
Server-side trailing metadata for tenant redirect hints.

When a server rejects a tenant it doesn't own (multi-node sharding),
it returns ``UNAVAILABLE`` with the human-readable hint
``(try node X)`` baked into the message. To let SDKs *act* on that
hint reliably (rather than parsing English), the server also
attaches the owning node id as a trailing metadata header
``entdb-redirect-node: <node_id>``.

These tests assert the trailer is set when, and only when, the
sharding layer can name an owner.
"""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock

import grpc
import pytest

from dbaas.entdb_server.api.grpc_server import EntDBServicer
from dbaas.entdb_server.sharding import ShardingConfig


@pytest.mark.asyncio
async def test_check_tenant_sets_redirect_trailer_when_owner_known():
    """Server attaches entdb-redirect-node trailer when registry knows the owner."""
    sharding = ShardingConfig(
        node_id="node-a",
        assigned_tenants=frozenset({"tenant-a"}),
        tenant_registry={"tenant-b": "node-b"},
    )
    servicer = EntDBServicer(
        wal=MagicMock(),
        canonical_store=MagicMock(),
        schema_registry=MagicMock(),
        sharding=sharding,
    )

    context = AsyncMock()
    context.abort = AsyncMock()
    context.set_trailing_metadata = MagicMock(return_value=None)

    await servicer._check_tenant("tenant-b", context)

    # Trailer must be set BEFORE abort so the client can read it
    # off the failed call.
    context.set_trailing_metadata.assert_called_once()
    sent = context.set_trailing_metadata.call_args[0][0]
    # Trailer is a sequence of (key, value) tuples.
    pairs = list(sent)
    assert ("entdb-redirect-node", "node-b") in pairs

    context.abort.assert_awaited_once()
    code, msg = context.abort.call_args[0]
    assert code == grpc.StatusCode.UNAVAILABLE
    assert "node-b" in msg


@pytest.mark.asyncio
async def test_check_tenant_no_trailer_when_owner_unknown():
    """No trailer when the registry doesn't know which node owns the tenant."""
    sharding = ShardingConfig(
        node_id="node-a",
        assigned_tenants=frozenset({"tenant-a"}),
        tenant_registry=None,
    )
    servicer = EntDBServicer(
        wal=MagicMock(),
        canonical_store=MagicMock(),
        schema_registry=MagicMock(),
        sharding=sharding,
    )

    context = AsyncMock()
    context.abort = AsyncMock()
    context.set_trailing_metadata = MagicMock(return_value=None)

    await servicer._check_tenant("tenant-z", context)

    context.set_trailing_metadata.assert_not_called()
    context.abort.assert_awaited_once()


@pytest.mark.asyncio
async def test_check_tenant_no_trailer_when_assigned_locally():
    """A tenant we own goes through with no abort and no trailer."""
    sharding = ShardingConfig(
        node_id="node-a",
        assigned_tenants=frozenset({"tenant-a"}),
        tenant_registry={"tenant-a": "node-a"},
    )
    servicer = EntDBServicer(
        wal=MagicMock(),
        canonical_store=MagicMock(),
        schema_registry=MagicMock(),
        sharding=sharding,
    )

    context = AsyncMock()
    context.abort = AsyncMock()
    context.set_trailing_metadata = MagicMock(return_value=None)

    await servicer._check_tenant("tenant-a", context)

    context.abort.assert_not_awaited()
    context.set_trailing_metadata.assert_not_called()
