"""
Unit tests for canonical tenant SQLite store.

Tests cover:
- Node CRUD operations
- Edge CRUD operations
- Idempotency checking
- Visibility management
"""

import tempfile

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore


class TestCanonicalStore:
    """Tests for CanonicalStore."""

    @pytest.fixture
    def data_dir(self):
        """Create temporary data directory."""
        with tempfile.TemporaryDirectory() as tmpdir:
            yield tmpdir

    @pytest.fixture
    def store(self, data_dir):
        """Create store for tenant."""
        return CanonicalStore(data_dir, wal_mode=False)

    @pytest.mark.asyncio
    async def test_create_node(self, store):
        """Create node stores data."""
        tenant_id = "tenant_1"
        node_id = await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "Test User", "email": "test@example.com"},
            owner_actor="user:alice",
            principals=["user:alice", "tenant:*"],
        )

        assert node_id is not None

        # Retrieve the node
        node = await store.get_node(tenant_id, node_id)
        assert node is not None
        assert node["payload"]["name"] == "Test User"
        assert node["owner_actor"] == "user:alice"

    @pytest.mark.asyncio
    async def test_create_node_idempotent(self, store):
        """Same idempotency key returns same result."""
        tenant_id = "tenant_1"

        node_id1 = await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "Test"},
            owner_actor="user:alice",
            idempotency_key="create_user_1",
        )

        # Second call with same idempotency key
        node_id2 = await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "Different"},
            owner_actor="user:alice",
            idempotency_key="create_user_1",
        )

        assert node_id1 == node_id2

    @pytest.mark.asyncio
    async def test_update_node(self, store):
        """Update node modifies payload."""
        tenant_id = "tenant_1"

        node_id = await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "Original"},
            owner_actor="user:alice",
        )

        await store.update_node(
            tenant_id=tenant_id,
            node_id=node_id,
            payload={"name": "Updated"},
        )

        node = await store.get_node(tenant_id, node_id)
        assert node["payload"]["name"] == "Updated"

    @pytest.mark.asyncio
    async def test_delete_node_soft_delete(self, store):
        """Delete performs soft delete."""
        tenant_id = "tenant_1"

        node_id = await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "Test"},
            owner_actor="user:alice",
        )

        await store.delete_node(tenant_id, node_id)

        # Node should not be returned normally
        node = await store.get_node(tenant_id, node_id)
        assert node is None

        # But exists in DB with deleted flag
        node = await store.get_node(tenant_id, node_id, include_deleted=True)
        assert node is not None
        assert node["deleted"] is True

    @pytest.mark.asyncio
    async def test_query_nodes_by_type(self, store):
        """Query nodes filters by type."""
        tenant_id = "tenant_1"

        # Create nodes of different types
        await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "User 1"},
            owner_actor="user:alice",
        )
        await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "User 2"},
            owner_actor="user:alice",
        )
        await store.create_node(
            tenant_id=tenant_id,
            type_id=2,
            payload={"title": "Task 1"},
            owner_actor="user:alice",
        )

        users = await store.query_nodes(tenant_id, type_id=1)
        assert len(users) == 2

        tasks = await store.query_nodes(tenant_id, type_id=2)
        assert len(tasks) == 1

    @pytest.mark.asyncio
    async def test_create_edge(self, store):
        """Create edge links nodes."""
        tenant_id = "tenant_1"

        # Create nodes
        user_id = await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "User"},
            owner_actor="user:alice",
        )
        task_id = await store.create_node(
            tenant_id=tenant_id,
            type_id=2,
            payload={"title": "Task"},
            owner_actor="user:alice",
        )

        # Create edge
        edge_id = await store.create_edge(
            tenant_id=tenant_id,
            edge_type_id=100,
            from_id=task_id,
            to_id=user_id,
            owner_actor="user:alice",
        )

        assert edge_id is not None

        # Query edges
        edges = await store.get_edges_from(tenant_id, task_id)
        assert len(edges) == 1
        assert edges[0]["to_id"] == user_id

    @pytest.mark.asyncio
    async def test_get_edges_to(self, store):
        """Get edges pointing to a node."""
        tenant_id = "tenant_1"

        user_id = await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "User"},
            owner_actor="user:alice",
        )
        task1_id = await store.create_node(
            tenant_id=tenant_id,
            type_id=2,
            payload={"title": "Task 1"},
            owner_actor="user:alice",
        )
        task2_id = await store.create_node(
            tenant_id=tenant_id,
            type_id=2,
            payload={"title": "Task 2"},
            owner_actor="user:alice",
        )

        # Assign both tasks to user
        await store.create_edge(
            tenant_id=tenant_id,
            edge_type_id=100,
            from_id=task1_id,
            to_id=user_id,
            owner_actor="user:alice",
        )
        await store.create_edge(
            tenant_id=tenant_id,
            edge_type_id=100,
            from_id=task2_id,
            to_id=user_id,
            owner_actor="user:alice",
        )

        edges = await store.get_edges_to(tenant_id, user_id)
        assert len(edges) == 2

    @pytest.mark.asyncio
    async def test_delete_edge(self, store):
        """Delete edge removes relationship."""
        tenant_id = "tenant_1"

        user_id = await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "User"},
            owner_actor="user:alice",
        )
        task_id = await store.create_node(
            tenant_id=tenant_id,
            type_id=2,
            payload={"title": "Task"},
            owner_actor="user:alice",
        )

        edge_id = await store.create_edge(
            tenant_id=tenant_id,
            edge_type_id=100,
            from_id=task_id,
            to_id=user_id,
            owner_actor="user:alice",
        )

        await store.delete_edge(tenant_id, edge_id)

        edges = await store.get_edges_from(tenant_id, task_id)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_check_idempotency(self, store):
        """Check idempotency returns stored result."""
        tenant_id = "tenant_1"

        # First operation
        result = await store.check_idempotency(tenant_id, "op_1")
        assert result is None

        # Record the operation
        await store.record_idempotency(tenant_id, "op_1", {"node_id": "abc123"})

        # Second check returns stored result
        result = await store.check_idempotency(tenant_id, "op_1")
        assert result == {"node_id": "abc123"}

    @pytest.mark.asyncio
    async def test_set_visibility(self, store):
        """Set visibility updates node principals."""
        tenant_id = "tenant_1"

        node_id = await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "Test"},
            owner_actor="user:alice",
            principals=["user:alice"],
        )

        # Update visibility
        await store.set_visibility(
            tenant_id=tenant_id,
            node_id=node_id,
            principals=["user:alice", "user:bob", "role:admin"],
        )

        visibility = await store.get_visibility(tenant_id, node_id)
        assert set(visibility) == {"user:alice", "user:bob", "role:admin"}

    @pytest.mark.asyncio
    async def test_tenant_isolation(self, store):
        """Different tenants have isolated data."""
        # Create node in tenant_1
        await store.create_node(
            tenant_id="tenant_1",
            type_id=1,
            payload={"name": "Tenant 1 User"},
            owner_actor="user:alice",
        )

        # Create node in tenant_2
        await store.create_node(
            tenant_id="tenant_2",
            type_id=1,
            payload={"name": "Tenant 2 User"},
            owner_actor="user:bob",
        )

        # Query each tenant
        t1_nodes = await store.query_nodes("tenant_1", type_id=1)
        t2_nodes = await store.query_nodes("tenant_2", type_id=1)

        assert len(t1_nodes) == 1
        assert len(t2_nodes) == 1
        assert t1_nodes[0]["payload"]["name"] == "Tenant 1 User"
        assert t2_nodes[0]["payload"]["name"] == "Tenant 2 User"
