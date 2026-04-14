"""
Integration tests for SDK client data types and plan builder.

Tests cover:
- Plan building (SDK v0.3 single-shape API: proto messages everywhere)
- Node and edge data classes
"""

from unittest.mock import MagicMock

import pytest

from sdk.entdb_sdk import register_proto_schema
from sdk.entdb_sdk.client import Edge, Node, Plan
from sdk.entdb_sdk.registry import get_registry, reset_registry
from tests._test_schemas import test_schema_pb2 as ts


@pytest.fixture(autouse=True)
def _registered_schema():
    """Register the v0.3 test_schema for the duration of each test."""
    reset_registry()
    register_proto_schema(ts)
    yield
    reset_registry()


def _mock_client() -> MagicMock:
    client = MagicMock()
    client.registry = get_registry()
    return client


class TestPlanBuilder:
    """Tests for Plan builder using SDK v0.3 proto-message single-shape API."""

    def test_plan_create_node(self):
        """Plan can create nodes from a proto message."""
        client = _mock_client()
        plan = Plan(client, tenant_id="t1", actor="user:alice")

        plan.create(
            ts.Product(sku="WIDGET-1", name="Widget", price_cents=100),
            as_="product1",
        )

        assert len(plan._operations) == 1
        assert "create_node" in plan._operations[0]
        assert plan._operations[0]["create_node"]["type_id"] == 9001
        assert plan._operations[0]["create_node"]["as"] == "product1"

    def test_plan_multiple_operations(self):
        """Plan can hold multiple operations."""
        client = _mock_client()
        plan = Plan(client, tenant_id="t1", actor="user:alice")

        plan.create(ts.Product(sku="p1", name="A"), as_="p1")
        plan.create(ts.Category(slug="cat-1", name="One"), as_="c1")

        assert len(plan._operations) == 2

    def test_plan_update_node(self):
        """Plan can update nodes — only the set fields become the patch."""
        client = _mock_client()
        plan = Plan(client, tenant_id="t1", actor="user:alice")

        plan.update("node_123", ts.Product(name="Updated Name"))

        assert len(plan._operations) == 1
        op = plan._operations[0]
        assert "update_node" in op
        assert op["update_node"]["type_id"] == 9001
        assert op["update_node"]["id"] == "node_123"
        assert op["update_node"]["patch"] == {"name": "Updated Name"}

    def test_plan_delete_node(self):
        """Plan can delete nodes — type witness is the proto class."""
        client = _mock_client()
        plan = Plan(client, tenant_id="t1", actor="user:alice")

        plan.delete(ts.Product, "node_123")

        assert len(plan._operations) == 1
        op = plan._operations[0]
        assert "delete_node" in op
        assert op["delete_node"]["type_id"] == 9001
        assert op["delete_node"]["id"] == "node_123"

    def test_plan_generates_idempotency_key(self):
        """Plan generates an idempotency key if not provided."""
        client = _mock_client()
        plan = Plan(client, tenant_id="t1", actor="user:alice")

        plan.create(ts.Product(sku="p1", name="X"))

        assert plan._idempotency_key is not None
        assert len(plan._idempotency_key) > 0

    def test_plan_uses_provided_idempotency_key(self):
        """Plan uses the caller-provided idempotency key when given."""
        client = _mock_client()
        plan = Plan(client, tenant_id="t1", actor="user:alice", idempotency_key="my-key")

        assert plan._idempotency_key == "my-key"


class TestNodeDataclass:
    """Tests for Node dataclass."""

    def test_node_creation(self):
        """Node can be created with data."""
        node = Node(
            tenant_id="tenant_1",
            node_id="node_123",
            type_id=1,
            payload={"email": "test@example.com"},
            owner_actor="user:alice",
            created_at=1234567890,
            updated_at=1234567890,
        )

        assert node.node_id == "node_123"
        assert node.type_id == 1
        assert node.payload["email"] == "test@example.com"


class TestEdgeDataclass:
    """Tests for Edge dataclass."""

    def test_edge_creation(self):
        """Edge can be created with data."""
        edge = Edge(
            tenant_id="tenant_1",
            edge_type_id=100,
            from_node_id="node_1",
            to_node_id="node_2",
            props={"weight": 1},
            created_at=1234567890,
        )

        assert edge.from_node_id == "node_1"
        assert edge.to_node_id == "node_2"
        assert edge.props["weight"] == 1
