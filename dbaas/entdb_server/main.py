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
import os
import signal
import sys
from pathlib import Path

from .api import GrpcServer
from .api.grpc_server import EntDBServicer
from .apply import Applier, CanonicalStore
from .archive import Archiver
from .config import ServerConfig, WalBackend
from .global_store import GlobalStore
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


def _normalize_for_server(entry: dict) -> dict:
    """Normalize an SDK-produced ``to_dict`` entry for the server's
    ``from_dict``.

    SDK and server use different enum-serialization conventions:
    the SDK writes ``data_policy: "PERSONAL"`` (the enum *name*) while
    the server's ``DataPolicy`` enum has values ``"personal"`` (the
    lowercase form). Same story for ``subject_exit``. We lowercase
    these on the way in so the roundtrip works without changing
    either side's enum surface.
    """
    out = dict(entry)
    for key in ("data_policy", "on_subject_exit", "subject_exit"):
        v = out.get(key)
        if isinstance(v, str) and v.isupper():
            out[key] = v.lower()
    return out


def _legacy_node_to_dict_form(nt: dict) -> dict:
    """Translate a legacy hand-written YAML node entry (``id``,
    ``kind``, ...) into the ``NodeTypeDef.from_dict`` shape
    (``field_id``, etc.)."""
    fields_out = []
    for f in nt.get("fields", []):
        fd: dict = {
            "field_id": f["id"],
            "name": f["name"],
            "kind": f["kind"],
        }
        if f.get("required"):
            fd["required"] = True
        if f.get("enum_values") or f.get("values"):
            fd["enum_values"] = f.get("enum_values") or f.get("values")
        if f.get("indexed"):
            fd["indexed"] = True
        if f.get("searchable"):
            fd["searchable"] = True
        if f.get("unique"):
            fd["unique"] = True
        fields_out.append(fd)
    return {
        "type_id": nt["type_id"],
        "name": nt["name"],
        "fields": fields_out,
    }


def _legacy_edge_to_dict_form(et: dict) -> dict:
    """Translate a legacy hand-written YAML edge entry into the
    ``EdgeTypeDef.from_dict`` shape."""
    edge_dict: dict = {
        "edge_id": et["edge_id"],
        "name": et["name"],
        "from_type_id": et["from_type"],
        "to_type_id": et["to_type"],
    }
    if et.get("props"):
        edge_dict["props"] = [
            {"field_id": p["id"], "name": p["name"], "kind": p["kind"]} for p in et["props"]
        ]
    return edge_dict


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
        self.global_store: GlobalStore | None = None
        self.servicer: EntDBServicer | None = None
        self.grpc_server: GrpcServer | None = None
        self.applier: Applier | None = None
        self.archiver: Archiver | None = None
        self.snapshotter: Snapshotter | None = None

        # Background tasks
        self._tasks: list[asyncio.Task] = []

    @staticmethod
    def _load_schema_file(registry: object, schema_file: str) -> None:
        """Load node/edge types from a YAML or JSON schema file into the registry."""
        from .schema.types import EdgeTypeDef, NodeTypeDef

        path = Path(schema_file)
        if not path.exists():
            logger.warning(f"Schema file not found: {schema_file}")
            return

        raw = path.read_text()
        if path.suffix in (".yaml", ".yml"):
            # ``yaml`` is only required for the legacy YAML-format
            # path; lazy-import so JSON schema files (the canonical
            # ``entdb-schema snapshot`` output) load even in
            # environments that don't have PyYAML installed.
            import yaml

            data = yaml.safe_load(raw)
        else:
            import json

            data = json.loads(raw)

        if not data:
            return

        # Accept the wrapped form produced by ``entdb-schema snapshot``
        # (``{"version": 1, "fingerprint": "...", "schema": {...}}``) so
        # the CLI's output is directly bootable without an extra
        # ``jq '.schema'`` step. Plain top-level form keeps working.
        if "schema" in data and "node_types" not in data:
            data = data["schema"]

        # Two input shapes are accepted:
        #
        # 1. ``to_dict``-form (what ``entdb-schema snapshot`` produces and
        #    what ``NodeTypeDef.from_dict`` expects natively): ``field_id``,
        #    ``from_type_id``/``to_type_id``, kinds as ``"str"``/``"int"``.
        #
        # 2. Legacy hand-written-YAML form: ``id``, ``from_type``/``to_type``,
        #    kinds the same. Kept working so existing deployments that
        #    check a hand-edited ``schema.yaml`` into git don't break.
        #
        # Detection is per-field/per-edge via the presence of the
        # canonical key.
        node_map: dict[int, NodeTypeDef] = {}

        for nt in data.get("node_types", []):
            if nt.get("fields") and "field_id" in nt["fields"][0]:
                node_dict = _normalize_for_server(nt)
            else:
                node_dict = _legacy_node_to_dict_form(nt)
            node_type = NodeTypeDef.from_dict(node_dict)
            registry.register_node_type(node_type)  # type: ignore[attr-defined]
            node_map[node_type.type_id] = node_type
            logger.info(f"Registered node type: {node_type.name} (id={node_type.type_id})")

        for et in data.get("edge_types", []):
            if "from_type_id" in et:
                edge_dict = _normalize_for_server(et)
            else:
                edge_dict = _legacy_edge_to_dict_form(et)
            edge_type = EdgeTypeDef.from_dict(edge_dict)
            registry.register_edge_type(edge_type)  # type: ignore[attr-defined]
            logger.info(f"Registered edge type: {edge_type.name} (id={edge_type.edge_id})")

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

            # Initialize schema registry
            registry = get_registry()

            # Load schema from file if configured
            if self.config.schema_file:
                self._load_schema_file(registry, self.config.schema_file)

            try:
                fingerprint = freeze_registry()
                logger.info(f"Schema registry frozen, fingerprint: {fingerprint}")
            except Exception:
                logger.warning("Schema registry already frozen")
                fingerprint = registry.fingerprint or ""

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

            # Global store for cross-tenant metadata (users, tenants,
            # memberships, quotas). Required for the QuotaInterceptor,
            # the tenant-access check, and the Applier's post-apply
            # usage-increment hook.
            #
            # Only instantiated when auth or quotas are actually enabled
            # on this deployment. The no-auth / no-quota path (E2E,
            # dev-mode, tests) leaves global_store as None so existing
            # code paths that check ``if self.global_store is None`` to
            # bypass tenant-existence checks keep working.
            enable_global_store = (
                self.config.grpc.auth_enabled or os.getenv("ENTDB_QUOTAS_ENABLED") == "true"
            )
            if enable_global_store:
                try:
                    self.global_store = GlobalStore(
                        data_dir=str(data_dir),
                        wal_mode=self.config.storage.wal_mode,
                        busy_timeout_ms=self.config.storage.busy_timeout_ms,
                        encryption_config=self.config.encryption,
                    )
                    logger.info("Global store initialised at %s", data_dir)
                except Exception as exc:
                    logger.warning("Global store initialisation failed: %s", exc)
                    self.global_store = None
            else:
                self.global_store = None
                logger.info(
                    "Global store disabled (auth+quotas off). Set auth_enabled=true or "
                    "ENTDB_QUOTAS_ENABLED=true to enable."
                )

            # Log warning for local mode
            if self.config.wal_backend == WalBackend.LOCAL:
                logger.warning(
                    "Running in LOCAL mode — WAL is in-memory only. "
                    "Data is NOT durable across restarts. Use snapshots for recovery."
                )

            # Determine topic based on WAL backend
            if self.config.wal_backend == WalBackend.KAFKA:
                topic = self.config.kafka.topic
            elif self.config.wal_backend == WalBackend.KINESIS:
                topic = self.config.kinesis.stream_name
            elif self.config.wal_backend == WalBackend.PUBSUB:
                topic = self.config.pubsub.topic_id
            elif self.config.wal_backend == WalBackend.SQS:
                topic = self.config.sqs.queue_url
            elif self.config.wal_backend == WalBackend.SERVICEBUS:
                topic = self.config.servicebus.queue_name
            elif self.config.wal_backend == WalBackend.EVENTHUBS:
                topic = self.config.eventhubs.eventhub_name
            elif self.config.wal_backend == WalBackend.LOCAL:
                topic = "entdb-wal"
            else:
                topic = "entdb-wal"

            # Initialize servicer
            self.servicer = EntDBServicer(
                wal=self.wal,
                canonical_store=self.canonical_store,
                schema_registry=registry,
                topic=topic,
                sharding=self.config.sharding,
                global_store=self.global_store,
                served_region=os.getenv("ENTDB_REGION") or None,
            )

            # Start gRPC server
            host, port = self.config.grpc.bind_address.split(":")
            self.grpc_server = GrpcServer(
                servicer=self.servicer,
                host=host,
                port=int(port),
                max_workers=self.config.grpc.max_workers,
                reflection_enabled=self.config.grpc.reflection_enabled,
                auth_enabled=self.config.grpc.auth_enabled,
                auth_api_keys=self.config.grpc.auth_api_keys,
                tls_cert_file=self.config.grpc.tls_cert_file,
                tls_key_file=self.config.grpc.tls_key_file,
                global_store=self.global_store,
            )
            await self.grpc_server.start()

            # Start Prometheus metrics if enabled
            if self.config.observability.metrics_enabled:
                from .metrics import init_metrics

                init_metrics(self.config.observability.metrics_port)

            # Start OpenTelemetry tracing if enabled
            if self.config.observability.trace_enabled:
                from .tracing import init_tracing

                init_tracing(
                    sampling_rate=self.config.observability.trace_sampling_rate,
                    exporter=self.config.observability.trace_exporter,
                    endpoint=self.config.observability.trace_endpoint,
                )

            # Start applier
            if self.config.wal_backend == WalBackend.KAFKA:
                group_id = self.config.kafka.consumer_group
            elif self.config.wal_backend == WalBackend.PUBSUB:
                group_id = self.config.pubsub.subscription_id
            elif self.config.wal_backend == WalBackend.EVENTHUBS:
                group_id = self.config.eventhubs.consumer_group
            else:
                group_id = "entdb-applier"

            self.applier = Applier(
                wal=self.wal,
                canonical_store=self.canonical_store,
                topic=topic,
                group_id=group_id,
                schema_fingerprint=fingerprint,
                batch_size=self.config.applier.batch_size,
                poll_timeout_ms=self.config.kafka.poll_timeout_ms
                if self.config.wal_backend == WalBackend.KAFKA
                else 100,
                assigned_tenants=self.config.sharding.assigned_tenants,
                global_store=self.global_store,
            )
            applier_task = asyncio.create_task(self.applier.start())
            self._tasks.append(applier_task)

            # Start archiver if enabled (uses its own WAL instance to avoid
            # sharing the consumer with the applier)
            if (
                self.config.archiver.enabled
                and self.config.archiver.flush_mode != "disabled"
                and self.config.wal_backend != WalBackend.LOCAL
            ):
                self._archiver_wal = create_wal_stream(self.config)
                await self._archiver_wal.connect()
                self.archiver = Archiver(
                    wal=self._archiver_wal,
                    s3_config=self.config.s3,
                    topic=topic,
                    group_id="entdb-archiver",
                    flush_interval_seconds=self.config.archiver.flush_interval_seconds,
                    max_segment_size_bytes=self.config.archiver.max_segment_size_bytes,
                    max_segment_events=self.config.archiver.max_segment_events,
                    compression=self.config.archiver.compression,
                    flush_mode=self.config.archiver.flush_mode,
                    min_segment_events=self.config.archiver.min_segment_events,
                    s3_storage_class=self.config.archiver.s3_storage_class,
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

    # How long to wait for background tasks to drain after the
    # cooperative .stop() flag is set, before falling back to cancel.
    # 30s is enough for an in-flight Kafka poll (max ~1s) plus a
    # large batch apply + offset commit, with headroom.
    _GRACEFUL_STOP_TIMEOUT_S: float = 30.0

    async def stop(self) -> None:
        """Stop the server gracefully.

        Shutdown sequence (order matters for durability):

        1. **Signal** every component with a cooperative ``stop()``
           flag so their background loops exit at the next checkpoint.
           The Applier in particular must finish its current batch
           and commit the WAL offset before being torn down — if we
           ``task.cancel()`` it mid-batch, events land in SQLite but
           the offset doesn't commit to Kafka, and replaying from the
           old offset on restart causes duplicate apply.
        2. **Drain** the background tasks with a bounded timeout.
           The Applier, archiver, and snapshotter all observe the
           stop flag, finish their current work, and return cleanly
           from their own coroutines. We ``gather()`` them to wait.
        3. **Cancel** only the tasks that are still running after the
           drain timeout. These are pathological — stuck in a poll
           or a lock — and cannot be recovered gracefully. We cancel
           them as the last resort, accepting the data-loss risk
           rather than hanging the shutdown forever.
        4. **Close** WAL / global store / canonical store resources.

        Partial-startup paths (tests that build Server via
        ``__new__`` or that fail in the middle of ``start()``) are
        handled by ``getattr`` + None-checks throughout.
        """
        logger.info("Stopping EntDB server")

        # 1. Signal cooperative stop on every component that has one.
        #    This must happen BEFORE we touch ``self._tasks`` so the
        #    loops observe the flag on their next iteration.
        if self.applier:
            try:
                await self.applier.stop()
            except Exception:  # pragma: no cover - defensive
                logger.warning("applier.stop() failed", exc_info=True)
        if self.archiver:
            try:
                await self.archiver.stop()
            except Exception:  # pragma: no cover - defensive
                logger.warning("archiver.stop() failed", exc_info=True)
        if self.snapshotter:
            try:
                await self.snapshotter.stop()
            except Exception:  # pragma: no cover - defensive
                logger.warning("snapshotter.stop() failed", exc_info=True)
        if self.grpc_server:
            # Stop accepting new RPCs now so in-flight requests drain.
            try:
                await self.grpc_server.stop()
            except Exception:  # pragma: no cover - defensive
                logger.warning("grpc_server.stop() failed", exc_info=True)

        # 2. Drain background tasks with a bounded timeout. Tasks
        #    like the Applier's main loop poll for WAL batches and
        #    exit when their ``_running`` flag flips False.
        if self._tasks:
            try:
                await asyncio.wait_for(
                    asyncio.gather(*self._tasks, return_exceptions=True),
                    timeout=self._GRACEFUL_STOP_TIMEOUT_S,
                )
            except asyncio.TimeoutError:
                # 3. Fallback: cancel anything that didn't drain in time.
                stuck = [t for t in self._tasks if not t.done()]
                logger.warning(
                    "graceful shutdown timed out after %ss; cancelling %d stuck tasks",
                    self._GRACEFUL_STOP_TIMEOUT_S,
                    len(stuck),
                )
                for task in stuck:
                    task.cancel()
                await asyncio.gather(*self._tasks, return_exceptions=True)

        if hasattr(self, "_archiver_wal") and self._archiver_wal:
            await self._archiver_wal.close()

        if self.wal:
            await self.wal.close()

        # Use getattr so Server instances built via ``__new__`` (e.g. in
        # tests that exercise partial-startup paths) do not crash when
        # the attribute is missing.
        global_store = getattr(self, "global_store", None)
        if global_store:
            try:
                global_store.close()
            except Exception:  # pragma: no cover - defensive
                logger.warning("global store close failed", exc_info=True)

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
