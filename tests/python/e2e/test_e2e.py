# SPDX-License-Identifier: AGPL-3.0-only
"""
EntDB End-to-End Test Suite — node/edge/atomic operations.

These tests assert the SDK v0.3 single-shape API (proto messages
everywhere) against the running gRPC server. They were originally a
standalone script (``python test_e2e.py``); Wave 8 of EPIC #407
re-shaped them as pytest tests so pytest discovers them alongside the
ported ``test_full_flow.py``.

Coverage (12 tests):

* Node ops: create_single_node, create_multiple_nodes,
  query_nodes_by_type, query_products, get_node_by_id, update_node,
  delete_node
* Edge ops: create_edges, get_edges_from_node, delete_edge
* Advanced: large_payload, concurrent_transactions

The ``scope`` fixture (from conftest.py) is a session-shared
``ActorScope`` bound to (``e2e-test``, ``user:e2e-runner``) — the
seeded tenant + actor on the Go target, auto-created on the Python
target.

Environment overrides:
    ENTDB_HOST   (default: server inside the e2e-tests container)
    ENTDB_PORT   (default: 50051)
    ENTDB_TENANT (default: e2e-test)
    ENTDB_SERVER_TARGET   python | go
"""

from __future__ import annotations

import asyncio
import time

import e2e_schema_pb2 as pb

# pytest asyncio_mode = auto → no @pytest.mark.asyncio decorators.


# =============================================================================
# Node operations
# =============================================================================


async def test_create_single_node(scope) -> None:
    """Create a single node."""
    plan = scope.plan()
    plan.create(
        pb.User(
            email="alice@example.com",
            name="Alice Smith",
            age=30,
            created_at=int(time.time() * 1000),
        ),
        as_="alice",
    )
    result = await plan.commit(wait_applied=True)
    assert result.success, f"commit failed: {result.error}"
    assert len(result.created_node_ids) == 1


async def test_create_multiple_nodes(scope) -> None:
    """Create multiple nodes in one transaction."""
    plan = scope.plan()
    plan.create(pb.User(email="bob@example.com", name="Bob Jones", age=25), as_="bob")
    plan.create(
        pb.Product(
            sku=f"LAPTOP-{int(time.time() * 1000)}",
            name="Gaming Laptop",
            price=1299.99,
            category="electronics",
        ),
        as_="laptop",
    )
    plan.create(
        pb.Product(
            sku=f"SHIRT-{int(time.time() * 1000)}",
            name="Blue T-Shirt",
            price=29.99,
            category="clothing",
        ),
        as_="shirt",
    )

    result = await plan.commit(wait_applied=True)
    assert result.success, f"commit failed: {result.error}"
    assert len(result.created_node_ids) == 3


async def test_query_nodes_by_type(scope) -> None:
    """Query nodes by type."""
    nodes = await scope.query(pb.User, limit=100)
    assert len(nodes) >= 1
    for node in nodes:
        assert node.type_id == 8001
        assert "email" in node.payload
        assert "name" in node.payload


async def test_query_products(scope) -> None:
    """Query product nodes."""
    nodes = await scope.query(pb.Product, limit=100)
    assert len(nodes) >= 1
    for node in nodes:
        assert node.type_id == 8002
        assert "sku" in node.payload
        assert "price" in node.payload


async def test_get_node_by_id(scope) -> None:
    """Get a specific node by ID."""
    plan = scope.plan()
    plan.create(pb.User(email="david@example.com", name="David"), as_="david")
    result = await plan.commit(wait_applied=True)
    assert result.success
    node_id = result.created_node_ids[0]

    node = await scope.get(pb.User, node_id)
    assert node is not None
    assert node.payload["email"] == "david@example.com"
    assert node.payload["name"] == "David"


async def test_update_node(scope) -> None:
    """Update a node."""
    plan = scope.plan()
    plan.create(pb.User(email="frank@example.com", name="Frank", age=40), as_="frank")
    result = await plan.commit(wait_applied=True)
    assert result.success
    frank_id = result.created_node_ids[0]

    plan2 = scope.plan()
    plan2.update(frank_id, pb.User(age=41, active=False))
    result2 = await plan2.commit(wait_applied=True)
    assert result2.success, f"update failed: {result2.error}"

    node = await scope.get(pb.User, frank_id)
    assert node is not None
    assert node.payload["age"] == 41
    assert node.payload["active"] is False


async def test_delete_node(scope) -> None:
    """Delete a node."""
    plan = scope.plan()
    plan.create(pb.User(email="grace@example.com", name="Grace"), as_="grace")
    result = await plan.commit(wait_applied=True)
    assert result.success
    grace_id = result.created_node_ids[0]

    plan2 = scope.plan()
    plan2.delete(pb.User, grace_id)
    result2 = await plan2.commit(wait_applied=True)
    assert result2.success, f"delete failed: {result2.error}"

    node = await scope.get(pb.User, grace_id)
    assert node is None


# =============================================================================
# Edge operations
# =============================================================================


async def test_create_edges(scope) -> None:
    """Create edges between nodes."""
    plan = scope.plan()
    plan.create(pb.User(email="charlie@example.com", name="Charlie"), as_="charlie")
    plan.create(
        pb.Product(
            sku=f"PHONE-{int(time.time() * 1000)}",
            name="Smartphone",
            price=799.99,
            category="electronics",
        ),
        as_="phone",
    )
    plan.create(
        pb.Order(
            order_number=f"ORD-{int(time.time() * 1000)}",
            total=799.99,
            status="pending",
        ),
        as_="order1",
    )

    plan.edge_create(
        pb.Purchased, "$charlie", "$phone", props={"quantity": 1, "price_paid": 799.99}
    )
    plan.edge_create(pb.PlacedOrder, "$charlie", "$order1")
    plan.edge_create(pb.OrderContains, "$order1", "$phone", props={"quantity": 1})

    result = await plan.commit(wait_applied=True)
    assert result.success, f"commit failed: {result.error}"


async def test_get_edges_from_node(scope) -> None:
    """Get outgoing edges from a node."""
    plan = scope.plan()
    plan.create(pb.User(email="eve@example.com", name="Eve"), as_="eve")
    plan.create(
        pb.Product(
            sku=f"BOOK1-{int(time.time() * 1000)}",
            name="Python Book",
            price=49.99,
            category="other",
        ),
        as_="book",
    )
    plan.create(
        pb.Product(
            sku=f"BOOK2-{int(time.time() * 1000)}",
            name="Go Book",
            price=39.99,
            category="other",
        ),
        as_="gobook",
    )
    plan.edge_create(pb.Purchased, "$eve", "$book", props={"quantity": 2, "price_paid": 99.98})
    plan.edge_create(pb.Purchased, "$eve", "$gobook", props={"quantity": 1, "price_paid": 39.99})

    result = await plan.commit(wait_applied=True)
    assert result.success
    eve_id = result.created_node_ids[0]

    edges = await scope.edges_out(eve_id, edge_type=pb.Purchased)
    assert len(edges) == 2

    for edge in edges:
        assert edge.edge_type_id == 8101
        assert edge.from_node_id == eve_id
        assert "quantity" in edge.props


async def test_delete_edge(scope) -> None:
    """Delete an edge."""
    plan = scope.plan()
    plan.create(pb.User(email="henry@example.com", name="Henry"), as_="henry")
    plan.create(
        pb.Product(
            sku=f"MUG-{int(time.time() * 1000)}",
            name="Coffee Mug",
            price=12.99,
            category="other",
        ),
        as_="mug",
    )
    plan.edge_create(pb.Purchased, "$henry", "$mug", props={"quantity": 1})
    result = await plan.commit(wait_applied=True)
    assert result.success
    henry_id = result.created_node_ids[0]
    mug_id = result.created_node_ids[1]

    edges = await scope.edges_out(henry_id, edge_type=pb.Purchased)
    assert len(edges) == 1

    plan2 = scope.plan()
    plan2.edge_delete(pb.Purchased, henry_id, mug_id)
    result2 = await plan2.commit(wait_applied=True)
    assert result2.success, f"edge delete failed: {result2.error}"

    edges = await scope.edges_out(henry_id, edge_type=pb.Purchased)
    assert len(edges) == 0


# =============================================================================
# Advanced
# =============================================================================


async def test_large_payload(scope) -> None:
    """Handle larger payloads (~1KB strings)."""
    large_name = "A" * 1000

    plan = scope.plan()
    plan.create(
        pb.User(email="largeuser@example.com", name=large_name),
        as_="large",
    )
    result = await plan.commit(wait_applied=True)
    assert result.success

    node = await scope.get(pb.User, result.created_node_ids[0])
    assert node is not None
    assert node.payload["name"] == large_name


async def test_concurrent_transactions(scope) -> None:
    """Multiple concurrent transactions all succeed."""

    async def create_user(i: int):
        plan = scope.plan()
        plan.create(
            pb.User(
                email=f"concurrent{i}-{time.time_ns()}@example.com",
                name=f"Concurrent User {i}",
            )
        )
        return await plan.commit(wait_applied=True)

    results = await asyncio.gather(*[create_user(i) for i in range(10)])
    for i, result in enumerate(results):
        assert result.success, f"transaction {i} failed: {result.error}"
