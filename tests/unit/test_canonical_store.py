"""
Unit tests for canonical tenant SQLite store.

Tests cover:
- Node CRUD operations
- Edge CRUD operations
- Idempotency checking
- Visibility management
- Node lazy JSON passthrough (read-path optimization)
"""

import json
import tempfile

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore, Node


class TestNodeLazyJson:
    """Tests for Node lazy JSON string passthrough."""

    def test_from_row_skips_json_parsing(self):
        """Node.from_row stores raw JSON without parsing."""
        node = Node.from_row(
            tenant_id="t1",
            node_id="n1",
            type_id=1,
            payload_json='{"name": "Alice"}',
            created_at=1000,
            updated_at=2000,
            owner_actor="user:alice",
            acl_json='[{"principal": "user:alice", "permission": "read"}]',
        )
        # Raw strings should pass through unchanged
        assert node.payload_json == '{"name": "Alice"}'
        assert node.acl_json == '[{"principal": "user:alice", "permission": "read"}]'

        # Parsed forms should NOT be materialized yet
        assert node._payload_parsed is Node._SENTINEL
        assert node._acl_parsed is Node._SENTINEL

    def test_lazy_payload_parsed_on_access(self):
        """Accessing .payload triggers json.loads and caches result."""
        node = Node.from_row(
            tenant_id="t1",
            node_id="n1",
            type_id=1,
            payload_json='{"key": "value"}',
            created_at=0,
            updated_at=0,
            owner_actor="u",
            acl_json="[]",
        )
        # Before access
        assert node._payload_parsed is Node._SENTINEL

        # Access triggers parse
        payload = node.payload
        assert payload == {"key": "value"}
        assert node._payload_parsed is not Node._SENTINEL

        # Second access returns cached value (same object)
        assert node.payload is payload

    def test_lazy_acl_parsed_on_access(self):
        """Accessing .acl triggers json.loads and caches result."""
        node = Node.from_row(
            tenant_id="t1",
            node_id="n1",
            type_id=1,
            payload_json="{}",
            created_at=0,
            updated_at=0,
            owner_actor="u",
            acl_json='[{"principal": "user:x"}]',
        )
        assert node._acl_parsed is Node._SENTINEL
        acl = node.acl
        assert acl == [{"principal": "user:x"}]
        assert node.acl is acl

    def test_constructor_with_dict_payload(self):
        """Constructing with payload dict serializes immediately."""
        node = Node(
            tenant_id="t1",
            node_id="n1",
            type_id=1,
            payload={"a": 1},
            created_at=0,
            updated_at=0,
            owner_actor="u",
        )
        assert node.payload_json == '{"a": 1}'
        assert node.payload == {"a": 1}
        assert node._payload_parsed is not Node._SENTINEL

    def test_constructor_defaults(self):
        """Default payload is {} and default ACL is []."""
        node = Node(tenant_id="t", node_id="n", type_id=1)
        assert node.payload == {}
        assert node.acl == []
        assert node.payload_json == "{}"
        assert node.acl_json == "[]"

    def test_payload_json_matches_json_dumps(self):
        """payload_json from dict construction matches json.dumps output."""
        data = {"x": [1, 2, 3], "y": {"nested": True}}
        node = Node(
            tenant_id="t",
            node_id="n",
            type_id=1,
            payload=data,
            created_at=0,
            updated_at=0,
            owner_actor="u",
        )
        assert json.loads(node.payload_json) == data

    def test_read_path_no_roundtrip(self):
        """Simulates full read path: from_row -> payload_json should be zero-copy."""
        raw_payload = '{"title": "My Post", "body": "Hello world", "views": 42}'
        raw_acl = '[{"principal": "user:bob", "permission": "read"}]'
        node = Node.from_row(
            tenant_id="t1",
            node_id="n1",
            type_id=1,
            payload_json=raw_payload,
            created_at=0,
            updated_at=0,
            owner_actor="u",
            acl_json=raw_acl,
        )
        # The gRPC layer would use payload_json directly:
        assert node.payload_json is raw_payload  # exact same string object
        assert node.acl_json is raw_acl
        # And _payload_parsed was never touched
        assert node._payload_parsed is Node._SENTINEL

    def test_equality(self):
        """Two nodes with same data are equal."""
        n1 = Node(
            tenant_id="t",
            node_id="n",
            type_id=1,
            payload={"a": 1},
            created_at=0,
            updated_at=0,
            owner_actor="u",
        )
        n2 = Node.from_row("t", "n", 1, '{"a": 1}', 0, 0, "u", "[]")
        assert n1 == n2


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


class TestWritePathSingleSerialize:
    """Tests that write-path methods serialize JSON once, not twice."""

    @pytest.fixture
    def data_dir(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            yield tmpdir

    @pytest.fixture
    def store(self, data_dir):
        return CanonicalStore(data_dir, wal_mode=False)

    @pytest.mark.asyncio
    async def test_create_node_raw_returns_prebuilt_json(self, store):
        """create_node_raw returns Node with payload_json/acl_json pre-built,
        not re-serialized from the dict.  _payload_parsed must be SENTINEL
        (i.e. lazy, not eagerly materialized)."""
        tenant_id = "t_raw"
        await store.initialize_tenant(tenant_id)

        payload = {"title": "Hello", "count": 42}
        acl = [{"principal": "user:alice", "permission": "read"}]

        with store.batch_transaction(tenant_id) as conn:
            node = store.create_node_raw(
                conn,
                tenant_id=tenant_id,
                type_id=1,
                payload=payload,
                owner_actor="user:alice",
                node_id="node-1",
                acl=acl,
                created_at=9999,
            )

        # The node must have been constructed via payload_json= kwarg,
        # so _payload_parsed should be SENTINEL (not eagerly parsed).
        assert node._payload_parsed is Node._SENTINEL
        assert node._acl_parsed is Node._SENTINEL

        # The raw JSON strings must match what json.dumps would produce.
        assert json.loads(node.payload_json) == payload
        assert json.loads(node.acl_json) == acl

        # Accessing .payload lazily parses and returns the same data.
        assert node.payload == payload
        assert node.acl == acl

    @pytest.mark.asyncio
    async def test_create_node_returns_prebuilt_json(self, store):
        """Async create_node also returns Node with pre-built JSON strings."""
        tenant_id = "t_async"
        await store.initialize_tenant(tenant_id)

        payload = {"x": [1, 2, 3]}
        acl = [{"principal": "user:bob", "permission": "write"}]

        node = await store.create_node(
            tenant_id=tenant_id,
            type_id=2,
            payload=payload,
            owner_actor="user:bob",
            acl=acl,
        )

        # Should be constructed via payload_json= (SENTINEL means lazy).
        assert node._payload_parsed is Node._SENTINEL
        assert node._acl_parsed is Node._SENTINEL
        assert json.loads(node.payload_json) == payload
        assert json.loads(node.acl_json) == acl

    @pytest.mark.asyncio
    async def test_update_node_returns_prebuilt_json(self, store):
        """update_node returns Node with pre-built payload_json string."""
        tenant_id = "t_upd"
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
            patch={"name": "Patched", "extra": True},
        )

        assert updated is not None
        # Should be constructed via payload_json= (SENTINEL means lazy).
        assert updated._payload_parsed is Node._SENTINEL
        assert updated._acl_parsed is Node._SENTINEL
        assert json.loads(updated.payload_json) == {"name": "Patched", "extra": True}
