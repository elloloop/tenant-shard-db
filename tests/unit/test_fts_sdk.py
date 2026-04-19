"""
SDK tests for FTS search (2026-04-19 decision).

Tests that ``scope.search(NodeType, query)`` correctly delegates to
the gRPC client and returns results or empty list.
"""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock

import pytest

from dbaas.entdb_server.schema.registry import get_registry, reset_registry
from dbaas.entdb_server.schema.types import NodeTypeDef, field
from sdk.entdb_sdk.client import DbClient

TYPE_PRODUCT = 201


@pytest.fixture(autouse=True)
def registered_schema():
    reset_registry()
    reg = get_registry()
    product = NodeTypeDef(
        type_id=TYPE_PRODUCT,
        name="Product",
        fields=(
            field(1, "sku", "str", unique=True),
            field(3, "name", "str", searchable=True),
            field(6, "desc", "str", searchable=True),
        ),
    )
    reg.register_node_type(product)
    yield reg
    reset_registry()


@pytest.fixture
def product_type():
    return get_registry().get_node_type(TYPE_PRODUCT)


@pytest.mark.asyncio
async def test_client_search_nodes_delegates(product_type):
    """DbClient.search_nodes delegates to GrpcClient.search_nodes."""
    fake_node = MagicMock()
    fake_node.node_id = "p1"

    client = DbClient.__new__(DbClient)
    client._connected = True
    client._grpc = MagicMock()
    client._grpc.search_nodes = AsyncMock(return_value=[fake_node])

    results = await client.search_nodes(
        node_type=product_type,
        tenant_id="t1",
        actor="user:alice",
        query="widget",
        limit=10,
        offset=0,
    )

    assert len(results) == 1
    assert results[0].node_id == "p1"
    client._grpc.search_nodes.assert_awaited_once()
    call_kwargs = client._grpc.search_nodes.call_args
    assert call_kwargs.kwargs["query"] == "widget"
    assert call_kwargs.kwargs["type_id"] == TYPE_PRODUCT


@pytest.mark.asyncio
async def test_client_search_nodes_empty_result(product_type):
    """DbClient.search_nodes returns empty list for no matches."""
    client = DbClient.__new__(DbClient)
    client._connected = True
    client._grpc = MagicMock()
    client._grpc.search_nodes = AsyncMock(return_value=[])

    results = await client.search_nodes(
        node_type=product_type,
        tenant_id="t1",
        actor="user:alice",
        query="nonexistent",
    )

    assert results == []
