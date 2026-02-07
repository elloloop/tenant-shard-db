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
        item = await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            source_type_id=1,
            source_node_id="node_123",
            snippet="Hello from Bob",
            metadata={"sender": "user_bob"},
        )

        fetched = await store.get_item("tenant_1", "user_alice", item.item_id)
        assert fetched is not None
        assert fetched.source_node_id == "node_123"
        assert fetched.snippet == "Hello from Bob"

    @pytest.mark.asyncio
    async def test_mailbox_pagination(self, store):
        """Mailbox supports pagination."""
        # Add multiple items
        for i in range(10):
            await store.add_item(
                tenant_id="tenant_1",
                user_id="user_alice",
                source_type_id=1,
                source_node_id=f"node_{i}",
                snippet=f"Message {i}",
            )

        # Get first page
        page1 = await store.list_items("tenant_1", "user_alice", limit=3)
        assert len(page1) == 3

        # Get second page
        page2 = await store.list_items("tenant_1", "user_alice", limit=3, offset=3)
        assert len(page2) == 3

        # Items should be different
        ids1 = {item.item_id for item in page1}
        ids2 = {item.item_id for item in page2}
        assert ids1.isdisjoint(ids2)

    @pytest.mark.asyncio
    async def test_mark_read(self, store):
        """Mark item as read."""
        item = await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            source_type_id=1,
            source_node_id="node_123",
            snippet="Test message",
        )

        # Initially unread
        fetched = await store.get_item("tenant_1", "user_alice", item.item_id)
        assert fetched.state["read"] is False

        # Mark as read
        await store.mark_read("tenant_1", "user_alice", [item.item_id])

        fetched = await store.get_item("tenant_1", "user_alice", item.item_id)
        assert fetched.state["read"] == 1  # json_set stores 1 for true

    @pytest.mark.asyncio
    async def test_update_state(self, store):
        """Update item state."""
        item = await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            source_type_id=1,
            source_node_id="node_123",
            snippet="Test message",
        )

        # Mark as read via update_state
        updated = await store.update_state("tenant_1", "user_alice", item.item_id, {"read": True})
        assert updated is not None
        assert updated.state["read"] is True

        # Mark as unread via update_state
        updated = await store.update_state("tenant_1", "user_alice", item.item_id, {"read": False})
        assert updated is not None
        assert updated.state["read"] is False

    @pytest.mark.asyncio
    async def test_get_unread_count(self, store):
        """Get count of unread items."""
        items = []
        for i in range(5):
            item = await store.add_item(
                tenant_id="tenant_1",
                user_id="user_alice",
                source_type_id=1,
                source_node_id=f"node_{i}",
                snippet=f"Message {i}",
            )
            items.append(item)

        count = await store.get_unread_count("tenant_1", "user_alice")
        assert count == 5

        # Mark some as read
        await store.mark_read("tenant_1", "user_alice", [items[0].item_id, items[1].item_id])

        count = await store.get_unread_count("tenant_1", "user_alice")
        assert count == 3

    @pytest.mark.asyncio
    async def test_search_mailbox(self, store):
        """Full-text search in mailbox."""
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            source_type_id=1,
            source_node_id="node_1",
            snippet="Meeting tomorrow at 3pm",
        )
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            source_type_id=1,
            source_node_id="node_2",
            snippet="Lunch plans for Friday",
        )
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            source_type_id=1,
            source_node_id="node_3",
            snippet="Project meeting notes",
        )

        # Search for "meeting"
        results = await store.search("tenant_1", "user_alice", "meeting")
        assert len(results) == 2

        # Search for "Friday"
        results = await store.search("tenant_1", "user_alice", "Friday")
        assert len(results) == 1
        assert results[0].item.source_node_id == "node_2"

    @pytest.mark.asyncio
    async def test_search_no_results(self, store):
        """Search returns empty for no matches."""
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            source_type_id=1,
            source_node_id="node_1",
            snippet="Hello world",
        )

        results = await store.search("tenant_1", "user_alice", "nonexistent")
        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_delete_item(self, store):
        """Delete item from mailbox."""
        item = await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            source_type_id=1,
            source_node_id="node_123",
            snippet="Test message",
        )

        deleted = await store.delete_item("tenant_1", "user_alice", item.item_id)
        assert deleted is True

        items = await store.list_items("tenant_1", "user_alice")
        assert len(items) == 0

    @pytest.mark.asyncio
    async def test_user_isolation(self, store):
        """Different users have separate mailboxes."""
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            source_type_id=1,
            source_node_id="node_1",
            snippet="Alice's message",
        )
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_bob",
            source_type_id=1,
            source_node_id="node_2",
            snippet="Bob's message",
        )

        alice_items = await store.list_items("tenant_1", "user_alice")
        bob_items = await store.list_items("tenant_1", "user_bob")

        assert len(alice_items) == 1
        assert len(bob_items) == 1
        assert alice_items[0].snippet == "Alice's message"
        assert bob_items[0].snippet == "Bob's message"

    @pytest.mark.asyncio
    async def test_tenant_isolation(self, store):
        """Different tenants have isolated mailboxes."""
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            source_type_id=1,
            source_node_id="node_1",
            snippet="Tenant 1 message",
        )
        await store.add_item(
            tenant_id="tenant_2",
            user_id="user_alice",
            source_type_id=1,
            source_node_id="node_2",
            snippet="Tenant 2 message",
        )

        t1_items = await store.list_items("tenant_1", "user_alice")
        t2_items = await store.list_items("tenant_2", "user_alice")

        assert len(t1_items) == 1
        assert len(t2_items) == 1
        assert t1_items[0].snippet == "Tenant 1 message"
        assert t2_items[0].snippet == "Tenant 2 message"

    @pytest.mark.asyncio
    async def test_filter_by_type(self, store):
        """Filter mailbox by source type."""
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            source_type_id=1,
            source_node_id="node_1",
            snippet="Message type 1",
        )
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            source_type_id=2,
            source_node_id="node_2",
            snippet="Message type 2",
        )
        await store.add_item(
            tenant_id="tenant_1",
            user_id="user_alice",
            source_type_id=1,
            source_node_id="node_3",
            snippet="Another type 1",
        )

        type1_items = await store.list_items("tenant_1", "user_alice", source_type_id=1)
        type2_items = await store.list_items("tenant_1", "user_alice", source_type_id=2)

        assert len(type1_items) == 2
        assert len(type2_items) == 1
