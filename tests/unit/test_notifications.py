"""
Unit tests for notifications and read_cursors tables in canonical store.

Tests cover:
- Create notification, get by user, unread only
- Mark read (single + all)
- Unread count
- Batch create (1000 entries, verify timing < 50ms)
- Read cursors: update, get, unread channels
- Pagination
"""

import tempfile
import time

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore

TENANT = "tenant_1"


class TestNotifications:
    """Tests for notification CRUD operations."""

    @pytest.fixture
    def data_dir(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            yield tmpdir

    @pytest.fixture
    async def store(self, data_dir):
        s = CanonicalStore(data_dir)
        await s.initialize_tenant(TENANT)
        yield s
        s.close_all()

    # ── Create & Get ──────────────────────────────────────────────────

    @pytest.mark.asyncio
    async def test_create_notification_returns_id(self, store):
        """create_notification returns a UUID string."""
        nid = await store.create_notification(TENANT, "alice", "mention", "node_1")
        assert isinstance(nid, str)
        assert len(nid) == 36  # UUID format

    @pytest.mark.asyncio
    async def test_get_notifications_returns_created(self, store):
        """Created notification appears in get_notifications."""
        await store.create_notification(TENANT, "alice", "mention", "node_1", snippet="Hello")
        items = await store.get_notifications(TENANT, "alice")
        assert len(items) == 1
        assert items[0]["user_id"] == "alice"
        assert items[0]["type"] == "mention"
        assert items[0]["node_id"] == "node_1"
        assert items[0]["snippet"] == "Hello"
        assert items[0]["read"] == 0

    @pytest.mark.asyncio
    async def test_get_notifications_filters_by_user(self, store):
        """Notifications are scoped to the requested user."""
        await store.create_notification(TENANT, "alice", "mention", "n1")
        await store.create_notification(TENANT, "bob", "reply", "n2")
        alice_items = await store.get_notifications(TENANT, "alice")
        bob_items = await store.get_notifications(TENANT, "bob")
        assert len(alice_items) == 1
        assert len(bob_items) == 1
        assert alice_items[0]["user_id"] == "alice"
        assert bob_items[0]["user_id"] == "bob"

    @pytest.mark.asyncio
    async def test_get_notifications_unread_only(self, store):
        """unread_only=True filters out read notifications."""
        nid = await store.create_notification(TENANT, "alice", "mention", "n1")
        await store.create_notification(TENANT, "alice", "reply", "n2")
        await store.mark_notification_read(TENANT, nid)

        all_items = await store.get_notifications(TENANT, "alice")
        unread = await store.get_notifications(TENANT, "alice", unread_only=True)
        assert len(all_items) == 2
        assert len(unread) == 1
        assert unread[0]["type"] == "reply"

    @pytest.mark.asyncio
    async def test_get_notifications_ordered_newest_first(self, store):
        """Notifications are ordered by created_at DESC."""
        # Insert with small time gaps to ensure ordering
        await store.create_notification(TENANT, "alice", "mention", "n1", snippet="first")
        time.sleep(0.002)
        await store.create_notification(TENANT, "alice", "reply", "n2", snippet="second")
        items = await store.get_notifications(TENANT, "alice")
        assert items[0]["snippet"] == "second"
        assert items[1]["snippet"] == "first"

    @pytest.mark.asyncio
    async def test_get_notifications_empty_for_unknown_user(self, store):
        """No notifications for a user who has none."""
        items = await store.get_notifications(TENANT, "nobody")
        assert items == []

    @pytest.mark.asyncio
    async def test_create_notification_without_snippet(self, store):
        """Snippet is optional and defaults to None."""
        await store.create_notification(TENANT, "alice", "like", "n1")
        items = await store.get_notifications(TENANT, "alice")
        assert items[0]["snippet"] is None

    # ── Mark Read ─────────────────────────────────────────────────────

    @pytest.mark.asyncio
    async def test_mark_notification_read(self, store):
        """Marking a notification read sets read=1."""
        nid = await store.create_notification(TENANT, "alice", "mention", "n1")
        result = await store.mark_notification_read(TENANT, nid)
        assert result is True
        items = await store.get_notifications(TENANT, "alice")
        assert items[0]["read"] == 1

    @pytest.mark.asyncio
    async def test_mark_notification_read_nonexistent(self, store):
        """Marking a nonexistent notification returns False."""
        result = await store.mark_notification_read(TENANT, "does-not-exist")
        assert result is False

    @pytest.mark.asyncio
    async def test_mark_notification_read_idempotent(self, store):
        """Marking an already-read notification is idempotent."""
        nid = await store.create_notification(TENANT, "alice", "mention", "n1")
        await store.mark_notification_read(TENANT, nid)
        result = await store.mark_notification_read(TENANT, nid)
        # SQLite UPDATE sets read=1 again; rowcount is 1 because the row exists
        assert result is True

    @pytest.mark.asyncio
    async def test_mark_all_read(self, store):
        """mark_all_read marks all unread notifications for a user."""
        await store.create_notification(TENANT, "alice", "mention", "n1")
        await store.create_notification(TENANT, "alice", "reply", "n2")
        await store.create_notification(TENANT, "alice", "like", "n3")

        count = await store.mark_all_read(TENANT, "alice")
        assert count == 3

        unread = await store.get_notifications(TENANT, "alice", unread_only=True)
        assert len(unread) == 0

    @pytest.mark.asyncio
    async def test_mark_all_read_only_affects_target_user(self, store):
        """mark_all_read does not affect other users."""
        await store.create_notification(TENANT, "alice", "mention", "n1")
        await store.create_notification(TENANT, "bob", "mention", "n2")

        await store.mark_all_read(TENANT, "alice")

        bob_unread = await store.get_notifications(TENANT, "bob", unread_only=True)
        assert len(bob_unread) == 1

    @pytest.mark.asyncio
    async def test_mark_all_read_returns_zero_when_none(self, store):
        """mark_all_read returns 0 when no unread notifications exist."""
        count = await store.mark_all_read(TENANT, "alice")
        assert count == 0

    # ── Unread Count ──────────────────────────────────────────────────

    @pytest.mark.asyncio
    async def test_unread_count(self, store):
        """get_unread_count returns correct count."""
        await store.create_notification(TENANT, "alice", "mention", "n1")
        await store.create_notification(TENANT, "alice", "reply", "n2")
        count = await store.get_unread_count(TENANT, "alice")
        assert count == 2

    @pytest.mark.asyncio
    async def test_unread_count_decrements_on_read(self, store):
        """Unread count decrements when a notification is marked read."""
        nid = await store.create_notification(TENANT, "alice", "mention", "n1")
        await store.create_notification(TENANT, "alice", "reply", "n2")
        await store.mark_notification_read(TENANT, nid)
        count = await store.get_unread_count(TENANT, "alice")
        assert count == 1

    @pytest.mark.asyncio
    async def test_unread_count_zero_for_unknown_user(self, store):
        """Unread count is 0 for user with no notifications."""
        count = await store.get_unread_count(TENANT, "nobody")
        assert count == 0

    # ── Pagination ────────────────────────────────────────────────────

    @pytest.mark.asyncio
    async def test_pagination_limit(self, store):
        """Limit parameter restricts result count."""
        for i in range(10):
            await store.create_notification(TENANT, "alice", "mention", f"n{i}")
        page = await store.get_notifications(TENANT, "alice", limit=3)
        assert len(page) == 3

    @pytest.mark.asyncio
    async def test_pagination_offset(self, store):
        """Offset parameter skips results."""
        for i in range(10):
            await store.create_notification(TENANT, "alice", "mention", f"n{i}")
        all_items = await store.get_notifications(TENANT, "alice", limit=10)
        page2 = await store.get_notifications(TENANT, "alice", limit=3, offset=3)
        assert len(page2) == 3
        # Page 2 items should match items 3-5 from the full list
        assert [p["id"] for p in page2] == [a["id"] for a in all_items[3:6]]

    @pytest.mark.asyncio
    async def test_pagination_beyond_end(self, store):
        """Offset beyond total returns empty list."""
        await store.create_notification(TENANT, "alice", "mention", "n1")
        page = await store.get_notifications(TENANT, "alice", limit=10, offset=100)
        assert page == []

    # ── Batch Create ──────────────────────────────────────────────────

    @pytest.mark.asyncio
    async def test_batch_create_notifications(self, store):
        """batch_create_notifications inserts all entries."""
        entries = [{"user_id": f"user_{i}", "type": "mention", "node_id": "n1"} for i in range(50)]
        count = await store.batch_create_notifications(TENANT, entries)
        assert count == 50

        # Spot-check a specific user
        items = await store.get_notifications(TENANT, "user_0")
        assert len(items) == 1

    @pytest.mark.asyncio
    async def test_batch_create_with_snippets(self, store):
        """Batch entries can include optional snippets."""
        entries = [
            {"user_id": "alice", "type": "mention", "node_id": "n1", "snippet": "Hi!"},
            {"user_id": "bob", "type": "reply", "node_id": "n2"},
        ]
        count = await store.batch_create_notifications(TENANT, entries)
        assert count == 2
        alice = await store.get_notifications(TENANT, "alice")
        assert alice[0]["snippet"] == "Hi!"
        bob = await store.get_notifications(TENANT, "bob")
        assert bob[0]["snippet"] is None

    @pytest.mark.asyncio
    async def test_batch_create_1000_entries_performance(self, store):
        """1000 notifications created in a single batch under 50ms."""
        entries = [
            {"user_id": f"user_{i}", "type": "at_everyone", "node_id": "broadcast_1"}
            for i in range(1000)
        ]
        start = time.perf_counter()
        count = await store.batch_create_notifications(TENANT, entries)
        elapsed_ms = (time.perf_counter() - start) * 1000
        assert count == 1000
        assert elapsed_ms < 50, f"Batch insert took {elapsed_ms:.1f}ms, expected < 50ms"

    @pytest.mark.asyncio
    async def test_batch_create_empty(self, store):
        """Batch create with empty list returns 0."""
        count = await store.batch_create_notifications(TENANT, [])
        assert count == 0


class TestReadCursors:
    """Tests for read cursor operations."""

    @pytest.fixture
    def data_dir(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            yield tmpdir

    @pytest.fixture
    async def store(self, data_dir):
        s = CanonicalStore(data_dir)
        await s.initialize_tenant(TENANT)
        yield s
        s.close_all()

    # ── Update & Get ──────────────────────────────────────────────────

    @pytest.mark.asyncio
    async def test_update_and_get_read_cursor(self, store):
        """update_read_cursor sets a cursor, get_read_cursor retrieves it."""
        await store.update_read_cursor(TENANT, "alice", "channel_general")
        cursor_ts = await store.get_read_cursor(TENANT, "alice", "channel_general")
        assert cursor_ts is not None
        assert isinstance(cursor_ts, int)
        # Should be recent (within last second)
        now_ms = int(time.time() * 1000)
        assert abs(now_ms - cursor_ts) < 2000

    @pytest.mark.asyncio
    async def test_get_read_cursor_nonexistent(self, store):
        """get_read_cursor returns None for non-existent cursor."""
        cursor_ts = await store.get_read_cursor(TENANT, "alice", "no_such_channel")
        assert cursor_ts is None

    @pytest.mark.asyncio
    async def test_update_read_cursor_upsert(self, store):
        """Updating a cursor twice overwrites the timestamp."""
        await store.update_read_cursor(TENANT, "alice", "channel_general")
        first_ts = await store.get_read_cursor(TENANT, "alice", "channel_general")
        time.sleep(0.002)
        await store.update_read_cursor(TENANT, "alice", "channel_general")
        second_ts = await store.get_read_cursor(TENANT, "alice", "channel_general")
        assert second_ts >= first_ts

    @pytest.mark.asyncio
    async def test_read_cursors_scoped_per_user_channel(self, store):
        """Each (user, channel) pair has its own cursor."""
        await store.update_read_cursor(TENANT, "alice", "ch1")
        await store.update_read_cursor(TENANT, "alice", "ch2")
        await store.update_read_cursor(TENANT, "bob", "ch1")

        alice_ch1 = await store.get_read_cursor(TENANT, "alice", "ch1")
        alice_ch2 = await store.get_read_cursor(TENANT, "alice", "ch2")
        bob_ch1 = await store.get_read_cursor(TENANT, "bob", "ch1")

        assert alice_ch1 is not None
        assert alice_ch2 is not None
        assert bob_ch1 is not None

    # ── Unread Channels ───────────────────────────────────────────────

    @pytest.mark.asyncio
    async def test_get_unread_channels_with_no_cursor(self, store):
        """Channels with notifications but no cursor show as unread."""
        await store.create_notification(TENANT, "alice", "mention", "ch_general")
        await store.create_notification(TENANT, "alice", "reply", "ch_general")
        await store.create_notification(TENANT, "alice", "mention", "ch_random")

        unread = await store.get_unread_channels(TENANT, "alice")
        by_channel = {r["channel_id"]: r["unread_count"] for r in unread}
        assert by_channel["ch_general"] == 2
        assert by_channel["ch_random"] == 1

    @pytest.mark.asyncio
    async def test_get_unread_channels_after_cursor_update(self, store):
        """Updating read cursor clears unread count for that channel."""
        await store.create_notification(TENANT, "alice", "mention", "ch_general")
        time.sleep(0.002)
        await store.update_read_cursor(TENANT, "alice", "ch_general")

        unread = await store.get_unread_channels(TENANT, "alice")
        channel_ids = [r["channel_id"] for r in unread]
        assert "ch_general" not in channel_ids

    @pytest.mark.asyncio
    async def test_get_unread_channels_empty(self, store):
        """No unread channels when user has no notifications."""
        unread = await store.get_unread_channels(TENANT, "alice")
        assert unread == []

    @pytest.mark.asyncio
    async def test_get_unread_channels_partial_read(self, store):
        """Cursor only clears notifications created before the cursor time."""
        await store.create_notification(TENANT, "alice", "mention", "ch_general")
        time.sleep(0.002)
        await store.update_read_cursor(TENANT, "alice", "ch_general")
        time.sleep(0.002)
        # New notification after cursor should appear as unread
        await store.create_notification(TENANT, "alice", "reply", "ch_general")

        unread = await store.get_unread_channels(TENANT, "alice")
        by_channel = {r["channel_id"]: r["unread_count"] for r in unread}
        assert by_channel.get("ch_general") == 1
