"""
Tests for cron cycle #3 fixes:
  Server.stop() cleanup of partially-started components.
"""

import asyncio
from unittest.mock import AsyncMock

import pytest

# ── Server.stop() cleans up partial startup ──────────────────


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
