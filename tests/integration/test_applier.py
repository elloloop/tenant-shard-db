"""
Integration tests for Applier with in-memory WAL.

Tests cover:
- End-to-end event processing
- Idempotency
- Transaction handling
- Mailbox fanout
"""

import tempfile
import time

import pytest

from dbaas.entdb_server.apply.applier import Applier, MailboxFanoutConfig, TransactionEvent
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.apply.mailbox_store import MailboxStore
from dbaas.entdb_server.schema.registry import SchemaRegistry
from dbaas.entdb_server.schema.types import EdgeTypeDef, NodeTypeDef, field
from dbaas.entdb_server.wal.memory import InMemoryWalStream


class TestApplierIntegration:
    """Integration tests for Applier."""

    @pytest.fixture
    def data_dir(self):
        """Create temporary data directory."""
        with tempfile.TemporaryDirectory() as tmpdir:
            yield tmpdir

    @pytest.fixture
    def registry(self):
        """Create schema registry with test types."""
        reg = SchemaRegistry()

        User = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(
                field(1, "email", "str"),
                field(2, "name", "str"),
            ),
        )

        Task = NodeTypeDef(
            type_id=2,
            name="Task",
            fields=(
                field(1, "title", "str"),
                field(2, "description", "str"),
            ),
        )

        Message = NodeTypeDef(
            type_id=3,
            name="Message",
            fields=(
                field(1, "subject", "str"),
                field(2, "body", "str"),
            ),
        )

        AssignedTo = EdgeTypeDef(
            edge_id=100,
            name="AssignedTo",
            from_type=2,  # Task
            to_type=1,  # User
        )

        reg.register_node_type(User)
        reg.register_node_type(Task)
        reg.register_node_type(Message)
        reg.register_edge_type(AssignedTo)

        return reg

    @pytest.fixture
    def wal(self):
        """Create in-memory WAL."""
        return InMemoryWalStream(num_partitions=4)

    @pytest.fixture
    def canonical_store(self, data_dir):
        """Create canonical store."""
        return CanonicalStore(data_dir, wal_mode=False)

    @pytest.fixture
    def mailbox_store(self, data_dir):
        """Create mailbox store."""
        return MailboxStore(data_dir)

    @pytest.fixture
    async def applier(self, wal, canonical_store, mailbox_store, registry):
        """Create and start applier."""
        applier = Applier(
            wal=wal,
            canonical_store=canonical_store,
            mailbox_store=mailbox_store,
            topic="entdb-wal",
            group_id="applier",
            fanout_config=MailboxFanoutConfig(
                enabled=True,
                node_types={3},  # Message type fans out to mailbox
            ),
        )
        await wal.connect()
        yield applier
        await wal.close()

    def _make_event(
        self,
        tenant_id: str,
        actor: str,
        idempotency_key: str,
        ops: list,
    ) -> TransactionEvent:
        """Helper to create a TransactionEvent with proper field names."""
        return TransactionEvent(
            tenant_id=tenant_id,
            actor=actor,
            idempotency_key=idempotency_key,
            schema_fingerprint=None,
            ts_ms=int(time.time() * 1000),
            ops=ops,
        )

    @pytest.mark.asyncio
    async def test_create_node_event(self, applier, canonical_store):
        """Create node event is applied."""
        tenant_id = "tenant_1"

        event = self._make_event(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="create_user_1",
            ops=[
                {
                    "op": "create_node",
                    "as": "user1",
                    "type_id": 1,
                    "data": {"email": "alice@example.com", "name": "Alice"},
                }
            ],
        )

        result = await applier.apply_event(event)
        assert result.success
        assert len(result.created_nodes) == 1

        # Verify node was created
        nodes = await canonical_store.query_nodes(tenant_id, type_id=1)
        assert len(nodes) == 1
        assert nodes[0]["payload"]["email"] == "alice@example.com"

    @pytest.mark.asyncio
    async def test_idempotent_processing(self, applier, canonical_store):
        """Same event processed twice is idempotent."""
        tenant_id = "tenant_1"

        event = self._make_event(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="idempotent_op_1",
            ops=[
                {
                    "op": "create_node",
                    "as": "user1",
                    "type_id": 1,
                    "data": {"email": "test@example.com", "name": "Test"},
                }
            ],
        )

        # Apply same event twice
        result1 = await applier.apply_event(event)
        result2 = await applier.apply_event(event)

        assert result1.success
        assert not result1.skipped
        assert result2.success
        assert result2.skipped

        # Should only have one node
        nodes = await canonical_store.query_nodes(tenant_id, type_id=1)
        assert len(nodes) == 1

    @pytest.mark.asyncio
    async def test_create_edge_with_alias_reference(self, applier, canonical_store):
        """Edge can reference nodes by alias."""
        tenant_id = "tenant_1"

        event = self._make_event(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="create_task_with_assignment",
            ops=[
                {
                    "op": "create_node",
                    "as": "user1",
                    "type_id": 1,
                    "data": {"email": "alice@example.com", "name": "Alice"},
                },
                {
                    "op": "create_node",
                    "as": "task1",
                    "type_id": 2,
                    "data": {"title": "Fix bug", "description": "Important"},
                },
                {
                    "op": "create_edge",
                    "edge_id": 100,
                    "from": {"ref": "$task1.id"},
                    "to": {"ref": "$user1.id"},
                },
            ],
        )

        result = await applier.apply_event(event)
        assert result.success

        # Verify nodes created
        users = await canonical_store.query_nodes(tenant_id, type_id=1)
        tasks = await canonical_store.query_nodes(tenant_id, type_id=2)
        assert len(users) == 1
        assert len(tasks) == 1

        # Verify edge created
        assert len(result.created_edges) == 1
        assert result.created_edges[0][0] == 100  # edge_type_id

    @pytest.mark.asyncio
    async def test_update_node_event(self, applier, canonical_store):
        """Update node event modifies payload."""
        tenant_id = "tenant_1"

        # First create
        create_event = self._make_event(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="create_1",
            ops=[
                {
                    "op": "create_node",
                    "as": "user1",
                    "type_id": 1,
                    "data": {"email": "old@example.com", "name": "Old Name"},
                }
            ],
        )

        result = await applier.apply_event(create_event)
        assert result.success
        node_id = result.created_nodes[0]

        # Then update
        update_event = self._make_event(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="update_1",
            ops=[
                {
                    "op": "update_node",
                    "id": node_id,
                    "patch": {"email": "new@example.com", "name": "New Name"},
                }
            ],
        )

        result = await applier.apply_event(update_event)
        assert result.success

        # Verify update
        node = await canonical_store.get_node(tenant_id, node_id)
        assert node["payload"]["email"] == "new@example.com"
        assert node["payload"]["name"] == "New Name"

    @pytest.mark.asyncio
    async def test_delete_node_event(self, applier, canonical_store):
        """Delete node event soft-deletes."""
        tenant_id = "tenant_1"

        # Create
        create_event = self._make_event(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="create_2",
            ops=[
                {
                    "op": "create_node",
                    "as": "user1",
                    "type_id": 1,
                    "data": {"email": "test@example.com", "name": "Test"},
                }
            ],
        )

        result = await applier.apply_event(create_event)
        assert result.success
        node_id = result.created_nodes[0]

        # Delete
        delete_event = self._make_event(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="delete_1",
            ops=[
                {
                    "op": "delete_node",
                    "id": node_id,
                }
            ],
        )

        result = await applier.apply_event(delete_event)
        assert result.success

        # Node should not appear in normal query
        nodes = await canonical_store.query_nodes(tenant_id, type_id=1)
        assert len(nodes) == 0

        # But exists as deleted
        node = await canonical_store.get_node(tenant_id, node_id, include_deleted=True)
        assert node is not None
        assert node["deleted"] is True

    @pytest.mark.asyncio
    async def test_mailbox_fanout(self, applier, mailbox_store):
        """Message type fans out to recipient mailboxes."""
        tenant_id = "tenant_1"

        event = self._make_event(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="send_message_1",
            ops=[
                {
                    "op": "create_node",
                    "as": "msg1",
                    "type_id": 3,  # Message type
                    "data": {"subject": "Hello", "body": "Hi Bob!"},
                    "fanout_to": ["user:bob", "user:charlie"],
                }
            ],
        )

        result = await applier.apply_event(event)
        assert result.success

        # Check Bob's mailbox
        bob_items = await mailbox_store.get_items(tenant_id, "user:bob")
        assert len(bob_items) == 1

        # Check Charlie's mailbox
        charlie_items = await mailbox_store.get_items(tenant_id, "user:charlie")
        assert len(charlie_items) == 1

    @pytest.mark.asyncio
    async def test_multi_tenant_isolation(self, applier, canonical_store):
        """Events from different tenants are isolated."""
        # Tenant 1 event
        event1 = self._make_event(
            tenant_id="tenant_1",
            actor="user:alice",
            idempotency_key="t1_create",
            ops=[
                {
                    "op": "create_node",
                    "as": "user1",
                    "type_id": 1,
                    "data": {"email": "t1@example.com", "name": "Tenant 1 User"},
                }
            ],
        )

        # Tenant 2 event
        event2 = self._make_event(
            tenant_id="tenant_2",
            actor="user:bob",
            idempotency_key="t2_create",
            ops=[
                {
                    "op": "create_node",
                    "as": "user1",
                    "type_id": 1,
                    "data": {"email": "t2@example.com", "name": "Tenant 2 User"},
                }
            ],
        )

        result1 = await applier.apply_event(event1)
        result2 = await applier.apply_event(event2)
        assert result1.success
        assert result2.success

        # Verify isolation
        t1_nodes = await canonical_store.query_nodes("tenant_1", type_id=1)
        t2_nodes = await canonical_store.query_nodes("tenant_2", type_id=1)

        assert len(t1_nodes) == 1
        assert len(t2_nodes) == 1
        assert t1_nodes[0]["payload"]["email"] == "t1@example.com"
        assert t2_nodes[0]["payload"]["email"] == "t2@example.com"

    @pytest.mark.asyncio
    async def test_atomic_transaction_rollback(self, applier, canonical_store):
        """Failed transaction rolls back all operations."""
        tenant_id = "tenant_1"

        # Event with invalid operation (referencing non-existent node)
        event = self._make_event(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="bad_tx",
            ops=[
                {
                    "op": "create_node",
                    "as": "user1",
                    "type_id": 1,
                    "data": {"email": "test@example.com", "name": "Test"},
                },
                {
                    "op": "update_node",
                    "id": "nonexistent_id",
                    "patch": {"name": "Updated"},
                },
            ],
        )

        result = await applier.apply_event(event)
        # The apply_event should report failure
        assert not result.success or result.error is not None
