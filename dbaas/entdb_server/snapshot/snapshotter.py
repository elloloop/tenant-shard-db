# mypy: ignore-errors
"""
SQLite snapshotter for EntDB.

The Snapshotter periodically creates backups of tenant SQLite databases
and uploads them to S3. This enables:
- Fast restore (load snapshot + replay recent events)
- Point-in-time recovery
- Disaster recovery

Snapshot format:
    s3://<bucket>/snapshots/tenant=<id>/ts=<unix_ms>.sqlite.gz

Each snapshot includes a manifest:
    s3://<bucket>/snapshots/tenant=<id>/ts=<unix_ms>.manifest.json

Manifest contains:
    - snapshot_ts: When snapshot was taken
    - last_stream_pos: Last applied stream position
    - schema_fingerprint: Schema version
    - checksum: SHA-256 of compressed database

Invariants:
    - Snapshots are atomic (use SQLite backup API)
    - Only consistent snapshots are uploaded
    - Manifests are written after successful upload
    - Failed snapshots are cleaned up

How to change safely:
    - Add new manifest fields, don't remove existing ones
    - Test restore with old snapshots before format changes
    - Maintain backward compatibility in restore tool
"""

from __future__ import annotations

import asyncio
import gzip
import hashlib
import json
import logging
import shutil
import sqlite3
import tempfile
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from ..apply.canonical_store import CanonicalStore

logger = logging.getLogger(__name__)

# Try to import aiobotocore for S3
try:
    from aiobotocore.session import get_session

    S3_AVAILABLE = True
except ImportError:
    S3_AVAILABLE = False
    get_session = None


@dataclass
class SnapshotInfo:
    """Information about a snapshot.

    Attributes:
        tenant_id: Tenant identifier
        snapshot_ts: Snapshot timestamp (Unix ms)
        last_stream_pos: Last applied stream position
        schema_fingerprint: Schema version hash
        checksum: SHA-256 of compressed database
        size_bytes: Compressed size in bytes
        s3_key: S3 object key
    """

    tenant_id: str
    snapshot_ts: int
    last_stream_pos: str | None
    schema_fingerprint: str | None
    checksum: str
    size_bytes: int
    s3_key: str


class Snapshotter:
    """Creates and manages SQLite snapshots.

    The Snapshotter runs as a background loop that:
    1. Lists tenant databases
    2. Checks if snapshot is needed (time/event threshold)
    3. Creates consistent snapshot using SQLite backup API
    4. Compresses and uploads to S3
    5. Writes manifest for restore

    Attributes:
        canonical_store: CanonicalStore for database access
        s3_config: S3 configuration
        schema_fingerprint: Current schema version

    Example:
        >>> snapshotter = Snapshotter(canonical_store, s3_config)
        >>> await snapshotter.start()  # Runs until stopped
    """

    def __init__(
        self,
        canonical_store: CanonicalStore,
        s3_config: Any,
        schema_fingerprint: str | None = None,
        interval_seconds: int = 3600,
        min_events_since_last: int = 1000,
        compression: str = "gzip",
        max_concurrent: int = 4,
    ) -> None:
        """Initialize the snapshotter.

        Args:
            canonical_store: CanonicalStore instance
            s3_config: S3Config instance
            schema_fingerprint: Current schema fingerprint
            interval_seconds: Interval between snapshot checks
            min_events_since_last: Minimum events to trigger snapshot
            compression: Compression algorithm ("gzip" or "none")
            max_concurrent: Maximum concurrent snapshot uploads
        """
        self.canonical_store = canonical_store
        self.s3_config = s3_config
        self.schema_fingerprint = schema_fingerprint
        self.interval_seconds = interval_seconds
        self.min_events_since_last = min_events_since_last
        self.compression = compression
        self.max_concurrent = max_concurrent

        self._running = False
        self._snapshot_count = 0
        self._s3_client = None
        self._session = None
        self._semaphore = asyncio.Semaphore(max_concurrent)

    async def start(self) -> None:
        """Start the snapshotter loop."""
        if self._running:
            logger.warning("Snapshotter already running")
            return

        if not S3_AVAILABLE:
            logger.error("aiobotocore not available, snapshotter disabled")
            return

        self._running = True
        logger.info(
            "Starting snapshotter",
            extra={
                "bucket": self.s3_config.bucket,
                "interval_seconds": self.interval_seconds,
            },
        )

        # Initialize S3 client
        await self._init_s3_client()

        try:
            while self._running:
                await self._snapshot_cycle()
                await asyncio.sleep(self.interval_seconds)

        except asyncio.CancelledError:
            logger.info("Snapshotter cancelled")
        except Exception as e:
            logger.error(f"Snapshotter error: {e}", exc_info=True)
        finally:
            self._running = False
            await self._close_s3_client()

    async def stop(self) -> None:
        """Stop the snapshotter loop."""
        self._running = False
        logger.info("Stopping snapshotter")

    async def _init_s3_client(self) -> None:
        """Initialize S3 client."""
        self._session = get_session()

        client_kwargs = {
            "region_name": self.s3_config.region,
        }

        if self.s3_config.endpoint_url:
            client_kwargs["endpoint_url"] = self.s3_config.endpoint_url

        if self.s3_config.access_key_id:
            client_kwargs["aws_access_key_id"] = self.s3_config.access_key_id
            client_kwargs["aws_secret_access_key"] = self.s3_config.secret_access_key

        self._s3_ctx = self._session.create_client("s3", **client_kwargs)
        self._s3_client = await self._s3_ctx.__aenter__()

    async def _close_s3_client(self) -> None:
        """Close S3 client."""
        if self._s3_client:
            await self._s3_ctx.__aexit__(None, None, None)
            self._s3_client = None

    async def _snapshot_cycle(self) -> None:
        """Run one snapshot cycle for all tenants."""
        # List tenant databases
        data_dir = self.canonical_store.data_dir
        if not data_dir.exists():
            return

        tenant_ids = []
        for db_file in data_dir.glob("tenant_*.db"):
            # Extract tenant ID from filename
            tenant_id = db_file.stem.replace("tenant_", "")
            tenant_ids.append(tenant_id)

        if not tenant_ids:
            return

        logger.info(f"Starting snapshot cycle for {len(tenant_ids)} tenants")

        # Snapshot each tenant (with concurrency limit)
        tasks = [self._snapshot_tenant(tenant_id) for tenant_id in tenant_ids]
        await asyncio.gather(*tasks, return_exceptions=True)

    async def _snapshot_tenant(self, tenant_id: str) -> SnapshotInfo | None:
        """Create snapshot for a single tenant.

        Args:
            tenant_id: Tenant identifier

        Returns:
            SnapshotInfo if snapshot was created, None otherwise
        """
        async with self._semaphore:
            try:
                # Check if snapshot is needed
                if not await self._should_snapshot(tenant_id):
                    return None

                return await self._create_snapshot(tenant_id)

            except Exception as e:
                logger.error(f"Failed to snapshot tenant {tenant_id}: {e}", exc_info=True)
                return None

    async def _should_snapshot(self, tenant_id: str) -> bool:
        """Check if tenant needs a new snapshot."""
        # Get latest snapshot info
        latest = await self._get_latest_snapshot(tenant_id)
        if not latest:
            return True  # No snapshot exists

        # Check time since last snapshot
        age_seconds = (time.time() * 1000 - latest.snapshot_ts) / 1000
        # Check events since last snapshot
        # (This would require tracking event count, simplified for now)
        return age_seconds >= self.interval_seconds

    async def _create_snapshot(self, tenant_id: str) -> SnapshotInfo | None:
        """Create and upload a snapshot.

        Args:
            tenant_id: Tenant identifier

        Returns:
            SnapshotInfo if successful
        """
        db_path = self.canonical_store.get_db_path(tenant_id)
        if not db_path.exists():
            return None

        snapshot_ts = int(time.time() * 1000)

        # Create temporary file for snapshot
        with tempfile.NamedTemporaryFile(suffix=".db", delete=False) as tmp_file:
            tmp_path = Path(tmp_file.name)

        try:
            # Use SQLite backup API for consistent snapshot
            await asyncio.get_event_loop().run_in_executor(
                None,
                self._backup_database,
                str(db_path),
                str(tmp_path),
            )

            # Get last stream position
            last_stream_pos = await self.canonical_store.get_last_applied_position(tenant_id)

            # Compress if needed
            if self.compression == "gzip":
                compressed_path = tmp_path.with_suffix(".db.gz")
                await asyncio.get_event_loop().run_in_executor(
                    None,
                    self._compress_file,
                    str(tmp_path),
                    str(compressed_path),
                )
                upload_path = compressed_path
            else:
                upload_path = tmp_path

            # Compute checksum
            checksum = await asyncio.get_event_loop().run_in_executor(
                None,
                self._compute_checksum,
                str(upload_path),
            )

            # Build S3 key
            extension = ".sqlite.gz" if self.compression == "gzip" else ".sqlite"
            s3_key = (
                f"{self.s3_config.snapshot_prefix}/tenant={tenant_id}/ts={snapshot_ts}{extension}"
            )

            # Upload to S3
            file_size = upload_path.stat().st_size
            with open(upload_path, "rb") as f:
                await self._s3_client.put_object(
                    Bucket=self.s3_config.bucket,
                    Key=s3_key,
                    Body=f.read(),
                    ContentType="application/x-sqlite3",
                )

            # Create snapshot info
            snapshot_info = SnapshotInfo(
                tenant_id=tenant_id,
                snapshot_ts=snapshot_ts,
                last_stream_pos=last_stream_pos,
                schema_fingerprint=self.schema_fingerprint,
                checksum=checksum,
                size_bytes=file_size,
                s3_key=s3_key,
            )

            # Upload manifest
            manifest_key = s3_key.replace(extension, ".manifest.json")
            manifest = {
                "tenant_id": tenant_id,
                "snapshot_ts": snapshot_ts,
                "last_stream_pos": last_stream_pos,
                "schema_fingerprint": self.schema_fingerprint,
                "checksum": checksum,
                "size_bytes": file_size,
                "s3_key": s3_key,
            }
            await self._s3_client.put_object(
                Bucket=self.s3_config.bucket,
                Key=manifest_key,
                Body=json.dumps(manifest, indent=2).encode("utf-8"),
                ContentType="application/json",
            )

            self._snapshot_count += 1
            logger.info(
                "Created snapshot",
                extra={
                    "tenant_id": tenant_id,
                    "snapshot_ts": snapshot_ts,
                    "size_bytes": file_size,
                    "s3_key": s3_key,
                },
            )

            return snapshot_info

        finally:
            # Clean up temporary files
            if tmp_path.exists():
                tmp_path.unlink()
            compressed_path = tmp_path.with_suffix(".db.gz")
            if compressed_path.exists():
                compressed_path.unlink()

    def _backup_database(self, source_path: str, dest_path: str) -> None:
        """Create consistent database backup using SQLite backup API."""
        source_conn = sqlite3.connect(source_path)
        dest_conn = sqlite3.connect(dest_path)

        try:
            source_conn.backup(dest_conn)
        finally:
            source_conn.close()
            dest_conn.close()

    def _compress_file(self, source_path: str, dest_path: str) -> None:
        """Compress file with gzip."""
        with open(source_path, "rb") as f_in, gzip.open(dest_path, "wb") as f_out:
            shutil.copyfileobj(f_in, f_out)

    def _compute_checksum(self, file_path: str) -> str:
        """Compute SHA-256 checksum of file."""
        sha256 = hashlib.sha256()
        with open(file_path, "rb") as f:
            for chunk in iter(lambda: f.read(8192), b""):
                sha256.update(chunk)
        return f"sha256:{sha256.hexdigest()}"

    async def _get_latest_snapshot(self, tenant_id: str) -> SnapshotInfo | None:
        """Get the latest snapshot for a tenant."""
        try:
            prefix = f"{self.s3_config.snapshot_prefix}/tenant={tenant_id}/"
            response = await self._s3_client.list_objects_v2(
                Bucket=self.s3_config.bucket,
                Prefix=prefix,
            )

            manifests = [
                obj for obj in response.get("Contents", []) if obj["Key"].endswith(".manifest.json")
            ]

            if not manifests:
                return None

            # Get the most recent manifest
            latest = max(manifests, key=lambda x: x["LastModified"])

            # Download and parse manifest
            response = await self._s3_client.get_object(
                Bucket=self.s3_config.bucket,
                Key=latest["Key"],
            )
            content = await response["Body"].read()
            manifest = json.loads(content.decode("utf-8"))

            return SnapshotInfo(
                tenant_id=manifest["tenant_id"],
                snapshot_ts=manifest["snapshot_ts"],
                last_stream_pos=manifest.get("last_stream_pos"),
                schema_fingerprint=manifest.get("schema_fingerprint"),
                checksum=manifest["checksum"],
                size_bytes=manifest["size_bytes"],
                s3_key=manifest["s3_key"],
            )

        except Exception as e:
            logger.warning(f"Failed to get latest snapshot for {tenant_id}: {e}")
            return None

    async def snapshot_now(self, tenant_id: str) -> SnapshotInfo | None:
        """Create a snapshot immediately.

        Args:
            tenant_id: Tenant to snapshot

        Returns:
            SnapshotInfo if successful
        """
        if not self._s3_client:
            await self._init_s3_client()

        try:
            return await self._create_snapshot(tenant_id)
        finally:
            if not self._running:
                await self._close_s3_client()

    @property
    def stats(self) -> dict[str, Any]:
        """Get snapshotter statistics."""
        return {
            "running": self._running,
            "snapshot_count": self._snapshot_count,
        }


async def list_snapshots(
    s3_config: Any,
    tenant_id: str,
) -> list[SnapshotInfo]:
    """List all snapshots for a tenant.

    Args:
        s3_config: S3 configuration
        tenant_id: Tenant identifier

    Returns:
        List of snapshots, newest first
    """
    if not S3_AVAILABLE:
        return []

    session = get_session()
    client_kwargs = {"region_name": s3_config.region}
    if s3_config.endpoint_url:
        client_kwargs["endpoint_url"] = s3_config.endpoint_url

    snapshots = []

    async with session.create_client("s3", **client_kwargs) as s3:
        prefix = f"{s3_config.snapshot_prefix}/tenant={tenant_id}/"
        paginator = s3.get_paginator("list_objects_v2")

        async for page in paginator.paginate(Bucket=s3_config.bucket, Prefix=prefix):
            for obj in page.get("Contents", []):
                if obj["Key"].endswith(".manifest.json"):
                    try:
                        response = await s3.get_object(
                            Bucket=s3_config.bucket,
                            Key=obj["Key"],
                        )
                        content = await response["Body"].read()
                        manifest = json.loads(content.decode("utf-8"))

                        snapshots.append(
                            SnapshotInfo(
                                tenant_id=manifest["tenant_id"],
                                snapshot_ts=manifest["snapshot_ts"],
                                last_stream_pos=manifest.get("last_stream_pos"),
                                schema_fingerprint=manifest.get("schema_fingerprint"),
                                checksum=manifest["checksum"],
                                size_bytes=manifest["size_bytes"],
                                s3_key=manifest["s3_key"],
                            )
                        )
                    except Exception:
                        continue

    return sorted(snapshots, key=lambda s: s.snapshot_ts, reverse=True)
