"""
EntDB Server - Main entry point.

This module starts the EntDB server with all components:
- gRPC server (primary API)
- Applier loop (WAL -> SQLite)
- Archiver loop (WAL -> S3)
- Snapshotter loop (SQLite -> S3)

For HTTP/REST access, use the EntDB Console which provides
a web UI and REST API for data browsing (similar to phpMyAdmin).
See console/

Usage:
    python -m entdb_server.main

Configuration is entirely via environment variables.
See config.py for all available settings.

Invariants:
    - Server waits for WAL connection before accepting requests
    - Graceful shutdown waits for in-flight operations
    - All components share the same schema registry

How to change safely:
    - Add new components with enable/disable flags
    - Test shutdown sequence thoroughly
    - Monitor startup time in production
"""

from __future__ import annotations

import asyncio
import logging
import signal
import sys
from pathlib import Path

from .api import GrpcServer
from .api.grpc_server import EntDBServicer
from .apply import Applier, CanonicalStore, MailboxStore
from .archive import Archiver
from .config import ServerConfig, WalBackend
from .schema import freeze_registry, get_registry
from .snapshot import Snapshotter
from .wal import WalStream, create_wal_stream

logger = logging.getLogger(__name__)


def setup_logging(config: ServerConfig) -> None:
    """Configure logging based on configuration.

    Args:
        config: Server configuration
    """
    level = getattr(logging, config.observability.log_level.upper(), logging.INFO)

    if config.observability.log_format == "json":
        # Use JSON formatter
        try:
            import json_log_formatter

            formatter = json_log_formatter.JSONFormatter()
        except ImportError:
            # Fallback to simple JSON
            formatter = logging.Formatter(
                '{"time":"%(asctime)s","level":"%(levelname)s","logger":"%(name)s","message":"%(message)s"}'
            )
    else:
        formatter = logging.Formatter("%(asctime)s - %(name)s - %(levelname)s - %(message)s")

    handler = logging.StreamHandler()
    handler.setFormatter(formatter)

    root_logger = logging.getLogger()
    root_logger.setLevel(level)
    root_logger.handlers = [handler]

    # Reduce noise from libraries
    logging.getLogger("aiokafka").setLevel(logging.WARNING)
    logging.getLogger("botocore").setLevel(logging.WARNING)
    logging.getLogger("aiobotocore").setLevel(logging.WARNING)


class Server:
    """EntDB Server orchestrator.

    Manages the lifecycle of all server components:
    - WAL connection
    - gRPC server
    - Background loops (applier, archiver, snapshotter)

    Attributes:
        config: Server configuration
        wal: WAL stream instance
        canonical_store: Tenant SQLite store
        mailbox_store: Per-user mailbox store
        servicer: gRPC service implementation

    Example:
        >>> server = Server()
        >>> await server.start()
        >>> # Server is running
        >>> await server.stop()
    """

    def __init__(self, config: ServerConfig | None = None) -> None:
        """Initialize the server.

        Args:
            config: Optional server configuration (loaded from env if not provided)
        """
        self.config = config or ServerConfig.from_env()
        self._running = False
        self._shutdown_event = asyncio.Event()

        # Components (initialized in start())
        self.wal: WalStream | None = None
        self.canonical_store: CanonicalStore | None = None
        self.mailbox_store: MailboxStore | None = None
        self.servicer: EntDBServicer | None = None
        self.grpc_server: GrpcServer | None = None
        self.applier: Applier | None = None
        self.archiver: Archiver | None = None
        self.snapshotter: Snapshotter | None = None

        # Background tasks
        self._tasks: list[asyncio.Task] = []

    async def start(self) -> None:
        """Start the server and all components."""
        if self._running:
            logger.warning("Server already running")
            return

        logger.info("Starting EntDB server")
        self.config.log_config()

        try:
            # Ensure data directory exists
            data_dir = Path(self.config.storage.data_dir)
            data_dir.mkdir(parents=True, exist_ok=True)
            mailbox_dir = data_dir / "mailboxes"
            mailbox_dir.mkdir(parents=True, exist_ok=True)

            # Initialize schema registry
            registry = get_registry()
            # In production, types would be registered here
            # For now, freeze with empty registry
            try:
                fingerprint = freeze_registry()
                logger.info(f"Schema registry frozen, fingerprint: {fingerprint}")
            except Exception:
                logger.warning("Schema registry already frozen")
                fingerprint = registry.fingerprint

            # Initialize WAL stream
            self.wal = create_wal_stream(self.config)
            await self.wal.connect()
            logger.info("WAL stream connected")

            # Initialize stores
            self.canonical_store = CanonicalStore(
                data_dir=str(data_dir),
                wal_mode=self.config.storage.wal_mode,
                busy_timeout_ms=self.config.storage.busy_timeout_ms,
                cache_size_pages=self.config.storage.cache_size_pages,
            )

            self.mailbox_store = MailboxStore(
                data_dir=str(mailbox_dir),
                wal_mode=self.config.storage.wal_mode,
                busy_timeout_ms=self.config.storage.busy_timeout_ms,
            )

            # Initialize servicer
            self.servicer = EntDBServicer(
                wal=self.wal,
                canonical_store=self.canonical_store,
                mailbox_store=self.mailbox_store,
                schema_registry=registry,
                topic=self.config.kafka.topic
                if self.config.wal_backend == WalBackend.KAFKA
                else self.config.kinesis.stream_name,
            )

            # Start gRPC server
            host, port = self.config.grpc.bind_address.split(":")
            self.grpc_server = GrpcServer(
                servicer=self.servicer,
                host=host,
                port=int(port),
                max_workers=self.config.grpc.max_workers,
                reflection_enabled=self.config.grpc.reflection_enabled,
            )
            await self.grpc_server.start()

            # Start applier
            topic = (
                self.config.kafka.topic
                if self.config.wal_backend == WalBackend.KAFKA
                else self.config.kinesis.stream_name
            )
            self.applier = Applier(
                wal=self.wal,
                canonical_store=self.canonical_store,
                mailbox_store=self.mailbox_store,
                topic=topic,
                group_id=self.config.kafka.consumer_group
                if self.config.wal_backend == WalBackend.KAFKA
                else "entdb-applier",
                schema_fingerprint=fingerprint,
            )
            applier_task = asyncio.create_task(self.applier.start())
            self._tasks.append(applier_task)

            # Start archiver if enabled
            if self.config.archiver.enabled:
                self.archiver = Archiver(
                    wal=self.wal,
                    s3_config=self.config.s3,
                    topic=topic,
                    group_id="entdb-archiver",
                    flush_interval_seconds=self.config.archiver.flush_interval_seconds,
                    max_segment_size_bytes=self.config.archiver.max_segment_size_bytes,
                    max_segment_events=self.config.archiver.max_segment_events,
                    compression=self.config.archiver.compression,
                )
                archiver_task = asyncio.create_task(self.archiver.start())
                self._tasks.append(archiver_task)

            # Start snapshotter if enabled
            if self.config.snapshot.enabled:
                self.snapshotter = Snapshotter(
                    canonical_store=self.canonical_store,
                    s3_config=self.config.s3,
                    schema_fingerprint=fingerprint,
                    interval_seconds=self.config.snapshot.interval_seconds,
                    min_events_since_last=self.config.snapshot.min_events_since_last,
                    compression=self.config.snapshot.compression,
                    max_concurrent=self.config.snapshot.max_concurrent,
                )
                snapshotter_task = asyncio.create_task(self.snapshotter.start())
                self._tasks.append(snapshotter_task)

            self._running = True
            logger.info("EntDB server started successfully")

            # Wait for shutdown signal
            await self._shutdown_event.wait()

        except Exception as e:
            logger.error(f"Server startup failed: {e}", exc_info=True)
            await self.stop()
            raise

    async def stop(self) -> None:
        """Stop the server gracefully."""
        if not self._running:
            return

        logger.info("Stopping EntDB server")

        # Stop background tasks
        for task in self._tasks:
            task.cancel()

        if self._tasks:
            await asyncio.gather(*self._tasks, return_exceptions=True)

        # Stop components
        if self.applier:
            await self.applier.stop()

        if self.archiver:
            await self.archiver.stop()

        if self.snapshotter:
            await self.snapshotter.stop()

        if self.grpc_server:
            await self.grpc_server.stop()

        if self.wal:
            await self.wal.close()

        self._running = False
        logger.info("EntDB server stopped")

    def request_shutdown(self) -> None:
        """Request graceful shutdown."""
        self._shutdown_event.set()


def main() -> None:
    """Main entry point."""
    # Load configuration
    try:
        config = ServerConfig.from_env()
    except ValueError as e:
        print(f"Configuration error: {e}", file=sys.stderr)
        sys.exit(1)

    # Setup logging
    setup_logging(config)

    # Create server
    server = Server(config)

    # Setup signal handlers
    loop = asyncio.new_event_loop()
    asyncio.set_event_loop(loop)

    def handle_signal(sig: int) -> None:
        logger.info(f"Received signal {sig}, initiating shutdown")
        server.request_shutdown()

    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(sig, handle_signal, sig)

    # Run server
    try:
        loop.run_until_complete(server.start())
    except KeyboardInterrupt:
        pass
    finally:
        loop.run_until_complete(server.stop())
        loop.close()


if __name__ == "__main__":
    main()
