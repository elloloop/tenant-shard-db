"""
Integration tests for SDK client data types and plan builder.

Tests cover:
- Plan building
- Node and edge data classes
"""

import pytest

from sdk.entdb_sdk.client import Edge, Node, Plan
from sdk.entdb_sdk.schema import EdgeTypeDef, FieldDef, FieldKind, NodeTypeDef


def make_field(id: int, name: str, kind_str: str, **kwargs) -> FieldDef:
    """Helper to create field definitions."""
    kind_map = {
        "str": FieldKind.STRING,
        "int": FieldKind.INTEGER,
        "bool": FieldKind.BOOLEAN,
    }
    return FieldDef(field_id=id, name=name, kind=kind_map.get(kind_str, FieldKind.STRING), **kwargs)


@pytest.fixture
def user_type():
    """User type definition."""
    return NodeTypeDef(
        type_id=1,
        name="User",
        fields=(
            make_field(1, "email", "str", required=True),
            make_field(2, "name", "str"),
        ),
    )


@pytest.fixture
def task_type():
    """Task type definition."""
    return NodeTypeDef(
        type_id=2,
        name="Task",
        fields=(
            make_field(1, "title", "str"),
            make_field(2, "done", "bool"),
        ),
    )


@pytest.fixture
def edge_type():
    """AssignedTo edge type."""
    return EdgeTypeDef(
        edge_id=100,
        name="AssignedTo",
        from_type=2,
        to_type=1,
    )


class TestPlanBuilder:
    """Tests for Plan builder."""

    def test_plan_create_node(self, user_type):
        """Plan can create nodes."""
        from unittest.mock import MagicMock

        client = MagicMock()
        plan = Plan(client, tenant_id="t1", actor="user:alice")

        plan.create(user_type, {"email": "test@example.com", "name": "Test User"}, as_="user1")

        assert len(plan._operations) == 1
        assert "create_node" in plan._operations[0]

    def test_plan_multiple_operations(self, user_type, task_type, edge_type):
        """Plan can have multiple operations."""
        from unittest.mock import MagicMock

        client = MagicMock()
        plan = Plan(client, tenant_id="t1", actor="user:alice")

        plan.create(user_type, {"email": "a@b.com", "name": "A"}, as_="user1")
        plan.create(task_type, {"title": "Task 1", "done": False}, as_="task1")

        assert len(plan._operations) == 2

    def test_plan_update_node(self, user_type):
        """Plan can update nodes."""
        from unittest.mock import MagicMock

        client = MagicMock()
        plan = Plan(client, tenant_id="t1", actor="user:alice")

        plan.update(user_type, "node_123", {"name": "Updated Name"})

        assert len(plan._operations) == 1
        assert "update_node" in plan._operations[0]

    def test_plan_delete_node(self, user_type):
        """Plan can delete nodes."""
        from unittest.mock import MagicMock

        client = MagicMock()
        plan = Plan(client, tenant_id="t1", actor="user:alice")

        plan.delete(user_type, "node_123")

        assert len(plan._operations) == 1
        assert "delete_node" in plan._operations[0]

    def test_plan_generates_idempotency_key(self, user_type):
        """Plan generates idempotency key if not provided."""
        from unittest.mock import MagicMock

        client = MagicMock()
        plan = Plan(client, tenant_id="t1", actor="user:alice")

        plan.create(user_type, {"email": "test@example.com"})

        assert plan._idempotency_key is not None
        assert len(plan._idempotency_key) > 0

    def test_plan_uses_provided_idempotency_key(self, user_type):
        """Plan uses provided idempotency key."""
        from unittest.mock import MagicMock

        client = MagicMock()
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
