"""
Tests for cron cycle #3 fixes:
  1. FTS5 query sanitisation in MailboxStore.search()
  2. Server.stop() cleanup of partially-started components
"""

import asyncio
import tempfile
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from dbaas.entdb_server.apply.mailbox_store import MailboxStore


# ── Fix 1: FTS5 query sanitisation ──────────────────────────────────


class TestFts5QuerySanitisation:
    """MailboxStore.search() must sanitise user input before FTS5 MATCH."""

    @pytest.fixture
    def data_dir(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            yield tmpdir

    @pytest.fixture
    def store(self, data_dir):
        return MailboxStore(data_dir)

    # -- _sanitize_fts5_query unit tests --

    def test_plain_words_quoted(self):
        result = MailboxStore._sanitize_fts5_query("hello world")
        assert result == '"hello" "world"'

    def test_embedded_double_quotes_escaped(self):
        result = MailboxStore._sanitize_fts5_query('say "hi" now')
        assert result == '"say" """hi""" "now"'

    def test_fts5_operators_neutralised(self):
        """FTS5 boolean operators like AND, OR, NOT, NEAR must be quoted."""
        result = MailboxStore._sanitize_fts5_query("hello AND world OR NOT foo")
        assert result == '"hello" "AND" "world" "OR" "NOT" "foo"'

    def test_special_chars_neutralised(self):
        """Characters with FTS5 meaning (*, ^, :, -, +) are quoted."""
        result = MailboxStore._sanitize_fts5_query("prefix* col:value -excluded")
        tokens = result.split()
        assert len(tokens) == 3
        assert tokens[0] == '"prefix*"'
        assert tokens[1] == '"col:value"'
        assert tokens[2] == '"-excluded"'

    def test_empty_query(self):
        result = MailboxStore._sanitize_fts5_query("")
        assert result == '""'

    def test_whitespace_only_query(self):
        result = MailboxStore._sanitize_fts5_query("   ")
        assert result == '""'

    def test_parentheses_neutralised(self):
        result = MailboxStore._sanitize_fts5_query("(hello) OR (world)")
        tokens = result.split()
        assert len(tokens) == 3
        assert tokens[0] == '"(hello)"'
        assert tokens[1] == '"OR"'
        assert tokens[2] == '"(world)"'

    # -- Integration tests: search with malicious input --

    @pytest.mark.asyncio
    async def test_search_with_fts5_operators_does_not_crash(self, store):
        """Queries containing FTS5 operators should not crash."""
        await store.add_item(
            tenant_id="t1",
            user_id="u1",
            source_type_id=1,
            source_node_id="n1",
            snippet="hello world test",
        )

        # These would crash or behave unexpectedly without sanitisation
        dangerous_queries = [
            'hello AND world',
            'hello OR world',
            'NOT hello',
            'hello NEAR world',
            'hello NEAR/2 world',
            '"unbalanced quote',
            'col:value',
            '(hello OR world) AND test',
            'hello*',
            ') OR rowid > 0 --',
            '""',
            '""; DROP TABLE fts_mailbox; --',
        ]

        for query in dangerous_queries:
            # Should not raise any exception
            results = await store.search("t1", "u1", query)
            # Results may be empty or non-empty; the point is no crash
            assert isinstance(results, list), f"Query {query!r} returned non-list"

    @pytest.mark.asyncio
    async def test_search_still_finds_results_after_sanitisation(self, store):
        """Normal search queries should still return correct results."""
        await store.add_item(
            tenant_id="t1",
            user_id="u1",
            source_type_id=1,
            source_node_id="n1",
            snippet="meeting tomorrow at 3pm with alice",
        )
        await store.add_item(
            tenant_id="t1",
            user_id="u1",
            source_type_id=1,
            source_node_id="n2",
            snippet="lunch plans for friday",
        )

        results = await store.search("t1", "u1", "meeting")
        assert len(results) == 1
        assert results[0].item.source_node_id == "n1"

        results = await store.search("t1", "u1", "friday")
        assert len(results) == 1
        assert results[0].item.source_node_id == "n2"

    @pytest.mark.asyncio
    async def test_search_multi_word_still_works(self, store):
        """Multi-word queries should match items containing all words."""
        await store.add_item(
            tenant_id="t1",
            user_id="u1",
            source_type_id=1,
            source_node_id="n1",
            snippet="important project meeting notes",
        )
        await store.add_item(
            tenant_id="t1",
            user_id="u1",
            source_type_id=1,
            source_node_id="n2",
            snippet="casual meeting about lunch",
        )

        # Both words must appear (FTS5 implicit AND for quoted tokens)
        results = await store.search("t1", "u1", "project meeting")
        assert len(results) == 1
        assert results[0].item.source_node_id == "n1"


# ── Fix 2: Server.stop() cleans up partial startup ──────────────────


class TestServerStopPartialStartup:
    """Server.stop() must clean up components even when _running is False."""

    @pytest.mark.asyncio
    async def test_stop_cleans_up_wal_when_running_is_false(self):
        """If start() failed after WAL connect, stop() must still close WAL."""
        from dbaas.entdb_server.main import Server

        server = Server.__new__(Server)
        server._running = False
        server._tasks = []
        server._shutdown_event = asyncio.Event()

        # Simulate partial startup: WAL was connected but _running never set
        mock_wal = AsyncMock()
        server.wal = mock_wal
        server.canonical_store = None
        server.mailbox_store = None
        server.servicer = None
        server.grpc_server = None
        server.applier = None
        server.archiver = None
        server.snapshotter = None

        await server.stop()

        # WAL.close() must have been called
        mock_wal.close.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_stop_cleans_up_grpc_when_running_is_false(self):
        """If start() failed after gRPC start, stop() must still stop gRPC."""
        from dbaas.entdb_server.main import Server

        server = Server.__new__(Server)
        server._running = False
        server._tasks = []
        server._shutdown_event = asyncio.Event()

        mock_wal = AsyncMock()
        mock_grpc = AsyncMock()
        server.wal = mock_wal
        server.canonical_store = None
        server.mailbox_store = None
        server.servicer = None
        server.grpc_server = mock_grpc
        server.applier = None
        server.archiver = None
        server.snapshotter = None

        await server.stop()

        mock_grpc.stop.assert_awaited_once()
        mock_wal.close.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_stop_handles_all_none_components(self):
        """stop() must not crash when all components are None (very early failure)."""
        from dbaas.entdb_server.main import Server

        server = Server.__new__(Server)
        server._running = False
        server._tasks = []
        server._shutdown_event = asyncio.Event()
        server.wal = None
        server.canonical_store = None
        server.mailbox_store = None
        server.servicer = None
        server.grpc_server = None
        server.applier = None
        server.archiver = None
        server.snapshotter = None

        # Should not raise
        await server.stop()

    @pytest.mark.asyncio
    async def test_stop_cancels_tasks_and_cleans_components(self):
        """Full cleanup: cancel tasks, stop applier/archiver/snapshotter, close WAL."""
        from dbaas.entdb_server.main import Server

        server = Server.__new__(Server)
        server._running = True
        server._shutdown_event = asyncio.Event()

        mock_wal = AsyncMock()
        mock_grpc = AsyncMock()
        mock_applier = AsyncMock()
        mock_archiver = AsyncMock()
        mock_snapshotter = AsyncMock()

        server.wal = mock_wal
        server.grpc_server = mock_grpc
        server.applier = mock_applier
        server.archiver = mock_archiver
        server.snapshotter = mock_snapshotter
        server.canonical_store = None
        server.mailbox_store = None
        server.servicer = None

        # Create a fake task
        async def noop():
            await asyncio.sleep(999)

        task = asyncio.create_task(noop())
        server._tasks = [task]

        await server.stop()

        assert task.cancelled()
        mock_applier.stop.assert_awaited_once()
        mock_archiver.stop.assert_awaited_once()
        mock_snapshotter.stop.assert_awaited_once()
        mock_grpc.stop.assert_awaited_once()
        mock_wal.close.assert_awaited_once()
        assert server._running is False
