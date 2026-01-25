"""
Integration tests for SDK client with mock server.

Tests cover:
- Plan building
- Node and edge operations
- Query execution
"""

from unittest.mock import AsyncMock, MagicMock

import pytest

from sdk.entdb_sdk.client import DbClient, Edge, Node, Plan
from sdk.entdb_sdk.schema import EdgeTypeDef, FieldDef, FieldKind, NodeTypeDef


def make_field(id: int, name: str, kind_str: str, **kwargs) -> FieldDef:
    """Helper to create field definitions."""
    kind_map = {
        "str": FieldKind.STRING,
        "int": FieldKind.INTEGER,
        "bool": FieldKind.BOOLEAN,
    }
    return FieldDef(field_id=id, name=name, kind=kind_map.get(kind_str, FieldKind.STRING), **kwargs)


class TestPlanBuilder:
    """Tests for Plan builder."""

    @pytest.fixture
    def client(self):
        """Create mock client."""
        client = MagicMock(spec=DbClient)
        client.tenant_id = "tenant_1"
        client.actor = "user:alice"
        return client

    @pytest.fixture
    def user_type(self):
        """User type definition."""
        return NodeTypeDef(
            type_id=1,
            name="User",
            fields=(
                make_field(1, "email", "str"),
                make_field(2, "name", "str"),
            ),
        )

    @pytest.fixture
    def task_type(self):
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
    def edge_type(self):
        """AssignedTo edge type."""
        return EdgeTypeDef(
            edge_id=100,
            name="AssignedTo",
            from_type=2,
            to_type=1,
        )

    def test_plan_create_node(self, client, user_type):
        """Plan can create nodes."""
        plan = Plan(client)

        plan.create(
            user_type,
            {
                "email": "test@example.com",
                "name": "Test User",
            },
            alias="user1",
        )

        assert len(plan._operations) == 1
        assert plan._operations[0]["op"] == "create_node"
        assert plan._operations[0]["alias"] == "user1"

    def test_plan_multiple_operations(self, client, user_type, task_type, edge_type):
        """Plan can have multiple operations."""
        plan = Plan(client)

        plan.create(user_type, {"email": "a@b.com", "name": "A"}, alias="user1")
        plan.create(task_type, {"title": "Task 1", "done": False}, alias="task1")
        plan.link(edge_type, "$task1.id", "$user1.id")

        assert len(plan._operations) == 3

    def test_plan_update_node(self, client, user_type):
        """Plan can update nodes."""
        plan = Plan(client)

        plan.update("node_123", {"name": "Updated Name"})

        assert len(plan._operations) == 1
        assert plan._operations[0]["op"] == "update_node"
        assert plan._operations[0]["node_id"] == "node_123"

    def test_plan_delete_node(self, client):
        """Plan can delete nodes."""
        plan = Plan(client)

        plan.delete("node_123")

        assert len(plan._operations) == 1
        assert plan._operations[0]["op"] == "delete_node"

    def test_plan_with_idempotency_key(self, client, user_type):
        """Plan can set idempotency key."""
        plan = Plan(client, idempotency_key="unique_key_123")

        plan.create(user_type, {"email": "test@example.com"})

        assert plan._idempotency_key == "unique_key_123"

    def test_plan_generates_idempotency_key(self, client, user_type):
        """Plan generates idempotency key if not provided."""
        plan = Plan(client)

        plan.create(user_type, {"email": "test@example.com"})

        # Key should be generated
        assert plan._idempotency_key is not None
        assert len(plan._idempotency_key) > 0


class TestNodeDataclass:
    """Tests for Node dataclass."""

    def test_node_creation(self):
        """Node can be created with data."""
        node = Node(
            id="node_123",
            type_id=1,
            tenant_id="tenant_1",
            payload={"email": "test@example.com"},
            owner_actor="user:alice",
            created_at=1234567890,
        )

        assert node.id == "node_123"
        assert node.type_id == 1
        assert node.payload["email"] == "test@example.com"

    def test_node_get_field(self):
        """Node can get field value."""
        node = Node(
            id="node_123",
            type_id=1,
            tenant_id="tenant_1",
            payload={"email": "test@example.com", "name": "Test"},
            owner_actor="user:alice",
        )

        assert node.get("email") == "test@example.com"
        assert node.get("name") == "Test"
        assert node.get("missing") is None
        assert node.get("missing", "default") == "default"


class TestEdgeDataclass:
    """Tests for Edge dataclass."""

    def test_edge_creation(self):
        """Edge can be created with data."""
        edge = Edge(
            id="edge_456",
            edge_type_id=100,
            from_id="node_1",
            to_id="node_2",
            tenant_id="tenant_1",
            owner_actor="user:alice",
        )

        assert edge.id == "edge_456"
        assert edge.from_id == "node_1"
        assert edge.to_id == "node_2"


class TestDbClientMock:
    """Tests for DbClient with mocked transport."""

    @pytest.fixture
    def mock_transport(self):
        """Create mock transport."""
        transport = AsyncMock()
        transport.execute_atomic = AsyncMock(return_value={"results": [{"node_id": "created_123"}]})
        transport.get_node = AsyncMock(
            return_value={
                "id": "node_123",
                "type_id": 1,
                "payload": {"email": "test@example.com"},
                "owner_actor": "user:alice",
            }
        )
        transport.query_nodes = AsyncMock(
            return_value=[
                {"id": "node_1", "type_id": 1, "payload": {}},
                {"id": "node_2", "type_id": 1, "payload": {}},
            ]
        )
        return transport

    @pytest.mark.asyncio
    async def test_client_execute_plan(self, mock_transport):
        """Client can execute plan."""
        client = DbClient(
            transport=mock_transport,
            tenant_id="tenant_1",
            actor="user:alice",
        )

        user_type = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(make_field(1, "email", "str"),),
        )

        result = await client.atomic(
            lambda plan: plan.create(user_type, {"email": "test@example.com"})
        )

        mock_transport.execute_atomic.assert_called_once()
        assert result is not None

    @pytest.mark.asyncio
    async def test_client_get_node(self, mock_transport):
        """Client can get node."""
        client = DbClient(
            transport=mock_transport,
            tenant_id="tenant_1",
            actor="user:alice",
        )

        node = await client.get("node_123")

        mock_transport.get_node.assert_called_once()
        assert node.id == "node_123"

    @pytest.mark.asyncio
    async def test_client_query_nodes(self, mock_transport):
        """Client can query nodes."""
        client = DbClient(
            transport=mock_transport,
            tenant_id="tenant_1",
            actor="user:alice",
        )

        nodes = await client.query(type_id=1)

        mock_transport.query_nodes.assert_called_once()
        assert len(nodes) == 2
