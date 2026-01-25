"""
Integration tests for Applier with in-memory WAL.

Tests cover:
- End-to-end event processing
- Idempotency
- Transaction handling
- Mailbox fanout
"""

import contextlib
import json
import tempfile

import pytest

from dbaas.entdb_server.apply.applier import Applier, TransactionEvent
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
            wal_stream=wal,
            canonical_store=canonical_store,
            mailbox_store=mailbox_store,
            registry=registry,
            topic="entdb-wal",
            consumer_group="applier",
            mailbox_node_types={3},  # Message type fans out to mailbox
        )
        await wal.connect()
        yield applier
        await wal.close()

    @pytest.mark.asyncio
    async def test_create_node_event(self, wal, applier, canonical_store):
        """Create node event is applied."""
        tenant_id = "tenant_1"

        # Create transaction event
        event = TransactionEvent(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="create_user_1",
            operations=[
                {
                    "op": "create_node",
                    "alias": "user1",
                    "type_id": 1,
                    "payload": {"email": "alice@example.com", "name": "Alice"},
                }
            ],
        )

        # Append to WAL
        await wal.append(
            "entdb-wal",
            tenant_id,
            json.dumps(event.to_dict()).encode(),
        )

        # Process one event
        await applier.process_one()

        # Verify node was created
        nodes = await canonical_store.query_nodes(tenant_id, type_id=1)
        assert len(nodes) == 1
        assert nodes[0]["payload"]["email"] == "alice@example.com"

    @pytest.mark.asyncio
    async def test_idempotent_processing(self, wal, applier, canonical_store):
        """Same event processed twice is idempotent."""
        tenant_id = "tenant_1"

        event = TransactionEvent(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="idempotent_op_1",
            operations=[
                {
                    "op": "create_node",
                    "alias": "user1",
                    "type_id": 1,
                    "payload": {"email": "test@example.com", "name": "Test"},
                }
            ],
        )

        event_bytes = json.dumps(event.to_dict()).encode()

        # Append same event twice
        await wal.append("entdb-wal", tenant_id, event_bytes)
        await wal.append("entdb-wal", tenant_id, event_bytes)

        # Process both
        await applier.process_one()
        await applier.process_one()

        # Should only have one node
        nodes = await canonical_store.query_nodes(tenant_id, type_id=1)
        assert len(nodes) == 1

    @pytest.mark.asyncio
    async def test_create_edge_with_alias_reference(self, wal, applier, canonical_store):
        """Edge can reference nodes by alias."""
        tenant_id = "tenant_1"

        event = TransactionEvent(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="create_task_with_assignment",
            operations=[
                {
                    "op": "create_node",
                    "alias": "user1",
                    "type_id": 1,
                    "payload": {"email": "alice@example.com", "name": "Alice"},
                },
                {
                    "op": "create_node",
                    "alias": "task1",
                    "type_id": 2,
                    "payload": {"title": "Fix bug", "description": "Important"},
                },
                {
                    "op": "create_edge",
                    "edge_type_id": 100,
                    "from_id": "$task1.id",
                    "to_id": "$user1.id",
                },
            ],
        )

        await wal.append(
            "entdb-wal",
            tenant_id,
            json.dumps(event.to_dict()).encode(),
        )

        await applier.process_one()

        # Verify nodes created
        users = await canonical_store.query_nodes(tenant_id, type_id=1)
        tasks = await canonical_store.query_nodes(tenant_id, type_id=2)
        assert len(users) == 1
        assert len(tasks) == 1

        # Verify edge created
        edges = await canonical_store.get_edges_from(tenant_id, tasks[0]["id"])
        assert len(edges) == 1
        assert edges[0]["to_id"] == users[0]["id"]

    @pytest.mark.asyncio
    async def test_update_node_event(self, wal, applier, canonical_store):
        """Update node event modifies payload."""
        tenant_id = "tenant_1"

        # First create
        create_event = TransactionEvent(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="create_1",
            operations=[
                {
                    "op": "create_node",
                    "alias": "user1",
                    "type_id": 1,
                    "payload": {"email": "old@example.com", "name": "Old Name"},
                }
            ],
        )

        await wal.append(
            "entdb-wal",
            tenant_id,
            json.dumps(create_event.to_dict()).encode(),
        )
        await applier.process_one()

        nodes = await canonical_store.query_nodes(tenant_id, type_id=1)
        node_id = nodes[0]["id"]

        # Then update
        update_event = TransactionEvent(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="update_1",
            operations=[
                {
                    "op": "update_node",
                    "node_id": node_id,
                    "payload": {"email": "new@example.com", "name": "New Name"},
                }
            ],
        )

        await wal.append(
            "entdb-wal",
            tenant_id,
            json.dumps(update_event.to_dict()).encode(),
        )
        await applier.process_one()

        # Verify update
        node = await canonical_store.get_node(tenant_id, node_id)
        assert node["payload"]["email"] == "new@example.com"
        assert node["payload"]["name"] == "New Name"

    @pytest.mark.asyncio
    async def test_delete_node_event(self, wal, applier, canonical_store):
        """Delete node event soft-deletes."""
        tenant_id = "tenant_1"

        # Create
        create_event = TransactionEvent(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="create_2",
            operations=[
                {
                    "op": "create_node",
                    "alias": "user1",
                    "type_id": 1,
                    "payload": {"email": "test@example.com", "name": "Test"},
                }
            ],
        )

        await wal.append(
            "entdb-wal",
            tenant_id,
            json.dumps(create_event.to_dict()).encode(),
        )
        await applier.process_one()

        nodes = await canonical_store.query_nodes(tenant_id, type_id=1)
        node_id = nodes[0]["id"]

        # Delete
        delete_event = TransactionEvent(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="delete_1",
            operations=[
                {
                    "op": "delete_node",
                    "node_id": node_id,
                }
            ],
        )

        await wal.append(
            "entdb-wal",
            tenant_id,
            json.dumps(delete_event.to_dict()).encode(),
        )
        await applier.process_one()

        # Node should not appear in normal query
        nodes = await canonical_store.query_nodes(tenant_id, type_id=1)
        assert len(nodes) == 0

        # But exists as deleted
        node = await canonical_store.get_node(tenant_id, node_id, include_deleted=True)
        assert node is not None
        assert node["deleted"] is True

    @pytest.mark.asyncio
    async def test_mailbox_fanout(self, wal, applier, mailbox_store):
        """Message type fans out to recipient mailboxes."""
        tenant_id = "tenant_1"

        event = TransactionEvent(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="send_message_1",
            operations=[
                {
                    "op": "create_node",
                    "alias": "msg1",
                    "type_id": 3,  # Message type
                    "payload": {"subject": "Hello", "body": "Hi Bob!"},
                    "recipients": ["user:bob", "user:charlie"],
                }
            ],
        )

        await wal.append(
            "entdb-wal",
            tenant_id,
            json.dumps(event.to_dict()).encode(),
        )
        await applier.process_one()

        # Check Bob's mailbox
        bob_items = await mailbox_store.get_items(tenant_id, "bob")
        assert len(bob_items) == 1
        assert bob_items[0].preview_text == "Hello"

        # Check Charlie's mailbox
        charlie_items = await mailbox_store.get_items(tenant_id, "charlie")
        assert len(charlie_items) == 1

    @pytest.mark.asyncio
    async def test_multi_tenant_isolation(self, wal, applier, canonical_store):
        """Events from different tenants are isolated."""
        # Tenant 1 event
        event1 = TransactionEvent(
            tenant_id="tenant_1",
            actor="user:alice",
            idempotency_key="t1_create",
            operations=[
                {
                    "op": "create_node",
                    "alias": "user1",
                    "type_id": 1,
                    "payload": {"email": "t1@example.com", "name": "Tenant 1 User"},
                }
            ],
        )

        # Tenant 2 event
        event2 = TransactionEvent(
            tenant_id="tenant_2",
            actor="user:bob",
            idempotency_key="t2_create",
            operations=[
                {
                    "op": "create_node",
                    "alias": "user1",
                    "type_id": 1,
                    "payload": {"email": "t2@example.com", "name": "Tenant 2 User"},
                }
            ],
        )

        await wal.append("entdb-wal", "tenant_1", json.dumps(event1.to_dict()).encode())
        await wal.append("entdb-wal", "tenant_2", json.dumps(event2.to_dict()).encode())

        await applier.process_one()
        await applier.process_one()

        # Verify isolation
        t1_nodes = await canonical_store.query_nodes("tenant_1", type_id=1)
        t2_nodes = await canonical_store.query_nodes("tenant_2", type_id=1)

        assert len(t1_nodes) == 1
        assert len(t2_nodes) == 1
        assert t1_nodes[0]["payload"]["email"] == "t1@example.com"
        assert t2_nodes[0]["payload"]["email"] == "t2@example.com"

    @pytest.mark.asyncio
    async def test_atomic_transaction_rollback(self, wal, applier, canonical_store):
        """Failed transaction rolls back all operations."""
        tenant_id = "tenant_1"

        # Event with invalid operation (referencing non-existent node)
        event = TransactionEvent(
            tenant_id=tenant_id,
            actor="user:alice",
            idempotency_key="bad_tx",
            operations=[
                {
                    "op": "create_node",
                    "alias": "user1",
                    "type_id": 1,
                    "payload": {"email": "test@example.com", "name": "Test"},
                },
                {
                    "op": "update_node",
                    "node_id": "nonexistent_id",
                    "payload": {"name": "Updated"},
                },
            ],
        )

        await wal.append(
            "entdb-wal",
            tenant_id,
            json.dumps(event.to_dict()).encode(),
        )

        # Process should fail or skip
        with contextlib.suppress(Exception):
            await applier.process_one()

        # First operation should be rolled back
        nodes = await canonical_store.query_nodes(tenant_id, type_id=1)
        assert len(nodes) == 0
