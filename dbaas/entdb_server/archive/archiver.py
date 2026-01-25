"""
WAL stream archiver for EntDB.

The Archiver consumes events from the WAL stream and writes them
to S3 in compacted segment files. This provides:
- Long-term durability beyond stream retention
- Ability to rebuild databases from archive
- Compliance and audit trail

Archive format:
    s3://<bucket>/archive/tenant=<id>/partition=<p>/from=<offset>/to=<offset>.jsonl.gz

Each line in the archive is a JSON object:
    {"event": {...}, "position": {...}, "checksum": "sha256:..."}

Invariants:
    - Archives are append-only and immutable
    - Each event is archived exactly once (idempotent)
    - Archives contain checksums for integrity verification
    - Segment files are compressed with gzip

How to change safely:
    - Archive format changes require version field
    - Never modify existing archive files
    - Test restore from archive before format changes
"""

from __future__ import annotations

import asyncio
import gzip
import hashlib
import io
import json
import logging
import time
from dataclasses import dataclass, field
from typing import Any

from ..wal.base import StreamRecord, WalStream

logger = logging.getLogger(__name__)

# Try to import aiobotocore for S3
try:
    from aiobotocore.session import get_session

    S3_AVAILABLE = True
except ImportError:
    S3_AVAILABLE = False
    get_session = None


@dataclass
class ArchiveSegment:
    """Represents an archive segment file.

    Attributes:
        tenant_id: Tenant identifier
        partition: Stream partition
        from_offset: First event offset in segment
        to_offset: Last event offset in segment
        event_count: Number of events in segment
        size_bytes: Compressed size in bytes
        s3_key: S3 object key
        created_at: Creation timestamp
    """

    tenant_id: str
    partition: int
    from_offset: int
    to_offset: int
    event_count: int
    size_bytes: int
    s3_key: str
    created_at: int


@dataclass
class PendingSegment:
    """Segment being built before flush."""

    tenant_id: str
    partition: int
    from_offset: int
    events: list[dict[str, Any]] = field(default_factory=list)
    size_estimate: int = 0


class Archiver:
    """Archives WAL events to S3.

    The Archiver runs as a background loop that:
    1. Consumes events from the WAL stream
    2. Buffers events into segments per tenant/partition
    3. Flushes segments to S3 when size/time threshold is reached

    Attributes:
        wal: WAL stream to consume from
        s3_config: S3 configuration
        topic: WAL topic name
        group_id: Consumer group ID

    Example:
        >>> archiver = Archiver(wal, s3_config, topic="entdb-wal")
        >>> await archiver.start()  # Runs until stopped
    """

    def __init__(
        self,
        wal: WalStream,
        s3_config: Any,
        topic: str = "entdb-wal",
        group_id: str = "entdb-archiver",
        flush_interval_seconds: int = 60,
        max_segment_size_bytes: int = 100 * 1024 * 1024,
        max_segment_events: int = 10000,
        compression: str = "gzip",
    ) -> None:
        """Initialize the archiver.

        Args:
            wal: WAL stream to consume from
            s3_config: S3Config instance
            topic: WAL topic name
            group_id: Consumer group ID
            flush_interval_seconds: Interval between flushes
            max_segment_size_bytes: Maximum segment size
            max_segment_events: Maximum events per segment
            compression: Compression algorithm ("gzip" or "none")
        """
        self.wal = wal
        self.s3_config = s3_config
        self.topic = topic
        self.group_id = group_id
        self.flush_interval_seconds = flush_interval_seconds
        self.max_segment_size_bytes = max_segment_size_bytes
        self.max_segment_events = max_segment_events
        self.compression = compression

        self._running = False
        self._pending_segments: dict[str, PendingSegment] = {}  # key = tenant:partition
        self._last_flush_time = time.time()
        self._archived_count = 0
        self._s3_client = None
        self._session = None

    async def start(self) -> None:
        """Start the archiver loop."""
        if self._running:
            logger.warning("Archiver already running")
            return

        if not S3_AVAILABLE:
            logger.error("aiobotocore not available, archiver disabled")
            return

        self._running = True
        logger.info(
            "Starting archiver",
            extra={
                "topic": self.topic,
                "group_id": self.group_id,
                "bucket": self.s3_config.bucket,
            },
        )

        # Initialize S3 client
        await self._init_s3_client()

        # Start background flush task
        flush_task = asyncio.create_task(self._flush_loop())

        try:
            async for record in self.wal.subscribe(self.topic, self.group_id):
                if not self._running:
                    break

                await self._process_record(record)
                await self.wal.commit(record)

        except asyncio.CancelledError:
            logger.info("Archiver cancelled")
        except Exception as e:
            logger.error(f"Archiver error: {e}", exc_info=True)
        finally:
            self._running = False
            flush_task.cancel()

            # Final flush
            await self._flush_all()

            # Close S3 client
            await self._close_s3_client()

    async def stop(self) -> None:
        """Stop the archiver loop."""
        self._running = False
        logger.info("Stopping archiver")

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

    async def _process_record(self, record: StreamRecord) -> None:
        """Process a single WAL record."""
        try:
            event_data = record.value_json()
            tenant_id = event_data.get("tenant_id", "unknown")
            partition = record.position.partition

            # Create archive entry
            archive_entry = {
                "event": event_data,
                "position": record.position.to_dict(),
                "checksum": self._compute_checksum(record.value),
                "archived_at": int(time.time() * 1000),
            }

            # Add to pending segment
            segment_key = f"{tenant_id}:{partition}"
            if segment_key not in self._pending_segments:
                self._pending_segments[segment_key] = PendingSegment(
                    tenant_id=tenant_id,
                    partition=partition,
                    from_offset=record.position.offset,
                )

            segment = self._pending_segments[segment_key]
            segment.events.append(archive_entry)
            segment.size_estimate += len(record.value)

            # Check if segment should be flushed
            if (
                segment.size_estimate >= self.max_segment_size_bytes
                or len(segment.events) >= self.max_segment_events
            ):
                await self._flush_segment(segment_key)

        except Exception as e:
            logger.error(f"Error processing record for archive: {e}", exc_info=True)

    async def _flush_loop(self) -> None:
        """Background loop for periodic flushes."""
        while self._running:
            await asyncio.sleep(self.flush_interval_seconds)
            await self._flush_all()

    async def _flush_all(self) -> None:
        """Flush all pending segments."""
        segment_keys = list(self._pending_segments.keys())
        for key in segment_keys:
            await self._flush_segment(key)

    async def _flush_segment(self, segment_key: str) -> ArchiveSegment | None:
        """Flush a pending segment to S3.

        Args:
            segment_key: Key for the pending segment

        Returns:
            ArchiveSegment if flushed, None if empty
        """
        if segment_key not in self._pending_segments:
            return None

        segment = self._pending_segments.pop(segment_key)
        if not segment.events:
            return None

        try:
            # Build S3 key
            to_offset = segment.events[-1]["position"]["offset"]
            s3_key = self._build_s3_key(
                segment.tenant_id,
                segment.partition,
                segment.from_offset,
                to_offset,
            )

            # Serialize events
            content = self._serialize_segment(segment.events)

            # Upload to S3
            await self._s3_client.put_object(
                Bucket=self.s3_config.bucket,
                Key=s3_key,
                Body=content,
                ContentType="application/x-gzip"
                if self.compression == "gzip"
                else "application/x-ndjson",
            )

            self._archived_count += len(segment.events)

            archive_segment = ArchiveSegment(
                tenant_id=segment.tenant_id,
                partition=segment.partition,
                from_offset=segment.from_offset,
                to_offset=to_offset,
                event_count=len(segment.events),
                size_bytes=len(content),
                s3_key=s3_key,
                created_at=int(time.time() * 1000),
            )

            logger.info(
                "Flushed archive segment",
                extra={
                    "tenant_id": segment.tenant_id,
                    "partition": segment.partition,
                    "events": len(segment.events),
                    "size_bytes": len(content),
                    "s3_key": s3_key,
                },
            )

            return archive_segment

        except Exception as e:
            logger.error(f"Failed to flush segment: {e}", exc_info=True)
            # Put segment back for retry
            self._pending_segments[segment_key] = segment
            return None

    def _build_s3_key(
        self,
        tenant_id: str,
        partition: int,
        from_offset: int,
        to_offset: int,
    ) -> str:
        """Build S3 object key for archive segment."""
        extension = ".jsonl.gz" if self.compression == "gzip" else ".jsonl"
        return (
            f"{self.s3_config.archive_prefix}/"
            f"tenant={tenant_id}/"
            f"partition={partition}/"
            f"from={from_offset:020d}_to={to_offset:020d}{extension}"
        )

    def _serialize_segment(self, events: list[dict[str, Any]]) -> bytes:
        """Serialize events to JSONL format with optional compression."""
        lines = [json.dumps(e, separators=(",", ":")) + "\n" for e in events]
        content = "".join(lines).encode("utf-8")

        if self.compression == "gzip":
            buf = io.BytesIO()
            with gzip.GzipFile(fileobj=buf, mode="wb") as gz:
                gz.write(content)
            return buf.getvalue()

        return content

    def _compute_checksum(self, data: bytes) -> str:
        """Compute SHA-256 checksum of data."""
        return f"sha256:{hashlib.sha256(data).hexdigest()}"

    @property
    def stats(self) -> dict[str, Any]:
        """Get archiver statistics."""
        pending_events = sum(len(s.events) for s in self._pending_segments.values())
        return {
            "running": self._running,
            "archived_count": self._archived_count,
            "pending_segments": len(self._pending_segments),
            "pending_events": pending_events,
        }


async def list_archive_segments(
    s3_config: Any,
    tenant_id: str,
    partition: int | None = None,
) -> list[ArchiveSegment]:
    """List archive segments for a tenant.

    Args:
        s3_config: S3 configuration
        tenant_id: Tenant identifier
        partition: Optional partition filter

    Returns:
        List of archive segments
    """
    if not S3_AVAILABLE:
        return []

    session = get_session()
    client_kwargs = {"region_name": s3_config.region}
    if s3_config.endpoint_url:
        client_kwargs["endpoint_url"] = s3_config.endpoint_url

    async with session.create_client("s3", **client_kwargs) as s3:
        prefix = f"{s3_config.archive_prefix}/tenant={tenant_id}/"
        if partition is not None:
            prefix += f"partition={partition}/"

        segments = []
        paginator = s3.get_paginator("list_objects_v2")

        async for page in paginator.paginate(Bucket=s3_config.bucket, Prefix=prefix):
            for obj in page.get("Contents", []):
                key = obj["Key"]
                # Parse segment info from key
                # Format: .../from=<offset>_to=<offset>.jsonl.gz
                try:
                    parts = key.split("/")
                    filename = parts[-1]
                    partition_part = parts[-2]

                    # Extract partition
                    part_num = int(partition_part.split("=")[1])

                    # Extract offsets from filename
                    base = filename.replace(".jsonl.gz", "").replace(".jsonl", "")
                    offset_parts = base.split("_")
                    from_offset = int(offset_parts[0].split("=")[1])
                    to_offset = int(offset_parts[1].split("=")[1])

                    segments.append(
                        ArchiveSegment(
                            tenant_id=tenant_id,
                            partition=part_num,
                            from_offset=from_offset,
                            to_offset=to_offset,
                            event_count=0,  # Unknown without reading
                            size_bytes=obj["Size"],
                            s3_key=key,
                            created_at=int(obj["LastModified"].timestamp() * 1000),
                        )
                    )
                except Exception:
                    continue

        return sorted(segments, key=lambda s: (s.partition, s.from_offset))
