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
        await store.initialize_tenant(tenant_id)

        node = await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "Test User", "email": "test@example.com"},
            owner_actor="user:alice",
            acl=[
                {"principal": "user:alice", "permission": "read"},
                {"principal": "tenant:*", "permission": "read"},
            ],
        )

        assert node is not None
        assert node.node_id is not None

        # Retrieve the node
        fetched = await store.get_node(tenant_id, node.node_id)
        assert fetched is not None
        assert fetched.payload["name"] == "Test User"
        assert fetched.owner_actor == "user:alice"

    @pytest.mark.asyncio
    async def test_create_node_idempotent(self, store):
        """Idempotency key prevents duplicate processing."""
        tenant_id = "tenant_1"
        await store.initialize_tenant(tenant_id)

        # Record an applied event
        already_applied = await store.check_idempotency(tenant_id, "create_user_1")
        assert already_applied is False

        await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "Test"},
            owner_actor="user:alice",
        )
        await store.record_applied_event(tenant_id, "create_user_1", stream_pos="pos_1")

        # Second check shows event already applied
        already_applied = await store.check_idempotency(tenant_id, "create_user_1")
        assert already_applied is True

    @pytest.mark.asyncio
    async def test_update_node(self, store):
        """Update node modifies payload."""
        tenant_id = "tenant_1"
        await store.initialize_tenant(tenant_id)

        node = await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "Original"},
            owner_actor="user:alice",
        )

        updated = await store.update_node(
            tenant_id=tenant_id,
            node_id=node.node_id,
            patch={"name": "Updated"},
        )

        assert updated is not None
        assert updated.payload["name"] == "Updated"

        fetched = await store.get_node(tenant_id, node.node_id)
        assert fetched.payload["name"] == "Updated"

    @pytest.mark.asyncio
    async def test_delete_node(self, store):
        """Delete node removes it."""
        tenant_id = "tenant_1"
        await store.initialize_tenant(tenant_id)

        node = await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "Test"},
            owner_actor="user:alice",
        )

        deleted = await store.delete_node(tenant_id, node.node_id)
        assert deleted is True

        # Node should not be returned
        fetched = await store.get_node(tenant_id, node.node_id)
        assert fetched is None

    @pytest.mark.asyncio
    async def test_query_nodes_by_type(self, store):
        """Query nodes filters by type."""
        tenant_id = "tenant_1"
        await store.initialize_tenant(tenant_id)

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

        users = await store.get_nodes_by_type(tenant_id, type_id=1)
        assert len(users) == 2

        tasks = await store.get_nodes_by_type(tenant_id, type_id=2)
        assert len(tasks) == 1

    @pytest.mark.asyncio
    async def test_create_edge(self, store):
        """Create edge links nodes."""
        tenant_id = "tenant_1"
        await store.initialize_tenant(tenant_id)

        # Create nodes
        user_node = await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "User"},
            owner_actor="user:alice",
        )
        task_node = await store.create_node(
            tenant_id=tenant_id,
            type_id=2,
            payload={"title": "Task"},
            owner_actor="user:alice",
        )

        # Create edge
        edge = await store.create_edge(
            tenant_id=tenant_id,
            edge_type_id=100,
            from_node_id=task_node.node_id,
            to_node_id=user_node.node_id,
        )

        assert edge is not None

        # Query edges
        edges = await store.get_edges_from(tenant_id, task_node.node_id)
        assert len(edges) == 1
        assert edges[0].to_node_id == user_node.node_id

    @pytest.mark.asyncio
    async def test_get_edges_to(self, store):
        """Get edges pointing to a node."""
        tenant_id = "tenant_1"
        await store.initialize_tenant(tenant_id)

        user_node = await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "User"},
            owner_actor="user:alice",
        )
        task1_node = await store.create_node(
            tenant_id=tenant_id,
            type_id=2,
            payload={"title": "Task 1"},
            owner_actor="user:alice",
        )
        task2_node = await store.create_node(
            tenant_id=tenant_id,
            type_id=2,
            payload={"title": "Task 2"},
            owner_actor="user:alice",
        )

        # Assign both tasks to user
        await store.create_edge(
            tenant_id=tenant_id,
            edge_type_id=100,
            from_node_id=task1_node.node_id,
            to_node_id=user_node.node_id,
        )
        await store.create_edge(
            tenant_id=tenant_id,
            edge_type_id=100,
            from_node_id=task2_node.node_id,
            to_node_id=user_node.node_id,
        )

        edges = await store.get_edges_to(tenant_id, user_node.node_id)
        assert len(edges) == 2

    @pytest.mark.asyncio
    async def test_delete_edge(self, store):
        """Delete edge removes relationship."""
        tenant_id = "tenant_1"
        await store.initialize_tenant(tenant_id)

        user_node = await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "User"},
            owner_actor="user:alice",
        )
        task_node = await store.create_node(
            tenant_id=tenant_id,
            type_id=2,
            payload={"title": "Task"},
            owner_actor="user:alice",
        )

        edge = await store.create_edge(
            tenant_id=tenant_id,
            edge_type_id=100,
            from_node_id=task_node.node_id,
            to_node_id=user_node.node_id,
        )

        deleted = await store.delete_edge(
            tenant_id,
            edge.edge_type_id,
            edge.from_node_id,
            edge.to_node_id,
        )
        assert deleted is True

        edges = await store.get_edges_from(tenant_id, task_node.node_id)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_check_idempotency(self, store):
        """Check idempotency returns whether event was applied."""
        tenant_id = "tenant_1"
        await store.initialize_tenant(tenant_id)

        # First check - not yet applied
        result = await store.check_idempotency(tenant_id, "op_1")
        assert result is False

        # Record the operation
        await store.record_applied_event(tenant_id, "op_1", stream_pos="pos_1")

        # Second check - already applied
        result = await store.check_idempotency(tenant_id, "op_1")
        assert result is True

    @pytest.mark.asyncio
    async def test_visibility(self, store):
        """Visibility index allows filtering by principal."""
        tenant_id = "tenant_1"
        await store.initialize_tenant(tenant_id)

        # Create node with ACL
        await store.create_node(
            tenant_id=tenant_id,
            type_id=1,
            payload={"name": "Test"},
            owner_actor="user:alice",
            acl=[
                {"principal": "user:bob", "permission": "read"},
                {"principal": "role:admin", "permission": "read"},
            ],
        )

        # Owner can see the node
        visible = await store.get_visible_nodes(tenant_id, principal="user:alice")
        assert len(visible) == 1

        # ACL principal can see the node
        visible = await store.get_visible_nodes(tenant_id, principal="user:bob")
        assert len(visible) == 1

        # Non-ACL principal cannot see the node
        visible = await store.get_visible_nodes(tenant_id, principal="user:charlie")
        assert len(visible) == 0

    @pytest.mark.asyncio
    async def test_tenant_isolation(self, store):
        """Different tenants have isolated data."""
        await store.initialize_tenant("tenant_1")
        await store.initialize_tenant("tenant_2")

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
        t1_nodes = await store.get_nodes_by_type("tenant_1", type_id=1)
        t2_nodes = await store.get_nodes_by_type("tenant_2", type_id=1)

        assert len(t1_nodes) == 1
        assert len(t2_nodes) == 1
        assert t1_nodes[0].payload["name"] == "Tenant 1 User"
        assert t2_nodes[0].payload["name"] == "Tenant 2 User"
