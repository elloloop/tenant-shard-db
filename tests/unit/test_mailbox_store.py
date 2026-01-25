"""
Unit tests for mailbox SQLite store with FTS.

Tests cover:
- Mailbox item management
- Full-text search
- Marking read/unread
"""

import tempfile
import pytest
from dbaas.entdb_server.apply.mailbox_store import MailboxStore


class TestMailboxStore:
    """Tests for MailboxStore."""

    @pytest.fixture
    def data_dir(self):
        """Create temporary data directory."""
        with tempfile.TemporaryDirectory() as tmpdir:
            yield tmpdir

    @pytest.fixture
    def store(self, data_dir):
        """Create mailbox store."""
        return MailboxStore(data_dir)

    @pytest.mark.asyncio
    async def test_add_mailbox_item(self, store):
        """Add item to user mailbox."""
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            node_type_id=1,
            node_id="node_123",
            preview_text="Hello from Bob",
            metadata={"sender": "user_bob"},
        )

        items = await store.get_items("tenant_1", "user_alice")
        assert len(items) == 1
        assert items[0].node_id == "node_123"
        assert items[0].preview_text == "Hello from Bob"

    @pytest.mark.asyncio
    async def test_mailbox_pagination(self, store):
        """Mailbox supports pagination."""
        # Add multiple items
        for i in range(10):
            await store.add_item(
                tenant_id="tenant_1",
                user_id="user_alice",
                node_type_id=1,
                node_id=f"node_{i}",
                preview_text=f"Message {i}",
            )

        # Get first page
        page1 = await store.get_items("tenant_1", "user_alice", limit=3)
        assert len(page1) == 3

        # Get second page
        page2 = await store.get_items("tenant_1", "user_alice", limit=3, offset=3)
        assert len(page2) == 3

        # Items should be different
        ids1 = {item.node_id for item in page1}
        ids2 = {item.node_id for item in page2}
        assert ids1.isdisjoint(ids2)

    @pytest.mark.asyncio
    async def test_mark_read(self, store):
        """Mark item as read."""
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            node_type_id=1,
            node_id="node_123",
            preview_text="Test message",
        )

        # Initially unread
        items = await store.get_items("tenant_1", "user_alice")
        assert items[0].is_read is False

        # Mark as read
        await store.mark_read("tenant_1", "user_alice", "node_123")

        items = await store.get_items("tenant_1", "user_alice")
        assert items[0].is_read is True

    @pytest.mark.asyncio
    async def test_mark_unread(self, store):
        """Mark item as unread."""
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            node_type_id=1,
            node_id="node_123",
            preview_text="Test message",
        )

        await store.mark_read("tenant_1", "user_alice", "node_123")
        await store.mark_unread("tenant_1", "user_alice", "node_123")

        items = await store.get_items("tenant_1", "user_alice")
        assert items[0].is_read is False

    @pytest.mark.asyncio
    async def test_get_unread_count(self, store):
        """Get count of unread items."""
        for i in range(5):
            await store.add_item(
                tenant_id="tenant_1",
                user_id="user_alice",
                node_type_id=1,
                node_id=f"node_{i}",
                preview_text=f"Message {i}",
            )

        count = await store.get_unread_count("tenant_1", "user_alice")
        assert count == 5

        # Mark some as read
        await store.mark_read("tenant_1", "user_alice", "node_0")
        await store.mark_read("tenant_1", "user_alice", "node_1")

        count = await store.get_unread_count("tenant_1", "user_alice")
        assert count == 3

    @pytest.mark.asyncio
    async def test_search_mailbox(self, store):
        """Full-text search in mailbox."""
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            node_type_id=1,
            node_id="node_1",
            preview_text="Meeting tomorrow at 3pm",
        )
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            node_type_id=1,
            node_id="node_2",
            preview_text="Lunch plans for Friday",
        )
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            node_type_id=1,
            node_id="node_3",
            preview_text="Project meeting notes",
        )

        # Search for "meeting"
        results = await store.search("tenant_1", "user_alice", "meeting")
        assert len(results) == 2

        # Search for "Friday"
        results = await store.search("tenant_1", "user_alice", "Friday")
        assert len(results) == 1
        assert results[0].node_id == "node_2"

    @pytest.mark.asyncio
    async def test_search_no_results(self, store):
        """Search returns empty for no matches."""
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            node_type_id=1,
            node_id="node_1",
            preview_text="Hello world",
        )

        results = await store.search("tenant_1", "user_alice", "nonexistent")
        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_delete_item(self, store):
        """Delete item from mailbox."""
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            node_type_id=1,
            node_id="node_123",
            preview_text="Test message",
        )

        await store.delete_item("tenant_1", "user_alice", "node_123")

        items = await store.get_items("tenant_1", "user_alice")
        assert len(items) == 0

    @pytest.mark.asyncio
    async def test_user_isolation(self, store):
        """Different users have separate mailboxes."""
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            node_type_id=1,
            node_id="node_1",
            preview_text="Alice's message",
        )
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_bob",
            node_type_id=1,
            node_id="node_2",
            preview_text="Bob's message",
        )

        alice_items = await store.get_items("tenant_1", "user_alice")
        bob_items = await store.get_items("tenant_1", "user_bob")

        assert len(alice_items) == 1
        assert len(bob_items) == 1
        assert alice_items[0].preview_text == "Alice's message"
        assert bob_items[0].preview_text == "Bob's message"

    @pytest.mark.asyncio
    async def test_tenant_isolation(self, store):
        """Different tenants have isolated mailboxes."""
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            node_type_id=1,
            node_id="node_1",
            preview_text="Tenant 1 message",
        )
        await store.add_item(
            tenant_id="tenant_2",
            user_id="user_alice",
            node_type_id=1,
            node_id="node_2",
            preview_text="Tenant 2 message",
        )

        t1_items = await store.get_items("tenant_1", "user_alice")
        t2_items = await store.get_items("tenant_2", "user_alice")

        assert len(t1_items) == 1
        assert len(t2_items) == 1
        assert t1_items[0].preview_text == "Tenant 1 message"
        assert t2_items[0].preview_text == "Tenant 2 message"

    @pytest.mark.asyncio
    async def test_filter_by_type(self, store):
        """Filter mailbox by node type."""
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            node_type_id=1,
            node_id="node_1",
            preview_text="Message type 1",
        )
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            node_type_id=2,
            node_id="node_2",
            preview_text="Message type 2",
        )
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            node_type_id=1,
            node_id="node_3",
            preview_text="Another type 1",
        )

        type1_items = await store.get_items("tenant_1", "user_alice", node_type_id=1)
        type2_items = await store.get_items("tenant_1", "user_alice", node_type_id=2)

        assert len(type1_items) == 2
        assert len(type2_items) == 1

