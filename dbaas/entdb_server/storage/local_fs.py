"""
Local filesystem object storage backend.

Stores objects as files on disk. No external dependencies.
Perfect for development and single-machine deployments.
"""

from __future__ import annotations

import logging
from pathlib import Path

from .base import ObjectMeta

logger = logging.getLogger(__name__)


class LocalFsObjectStore:
    """Local filesystem storage backend."""

    def __init__(self, base_path: str = "/var/lib/entdb/storage") -> None:
        self._base = Path(base_path)

    async def connect(self) -> None:
        self._base.mkdir(parents=True, exist_ok=True)
        logger.info("Using local filesystem storage", extra={"path": str(self._base)})

    async def close(self) -> None:
        pass  # Nothing to close

    async def put(
        self,
        key: str,
        data: bytes,
        content_type: str = "application/octet-stream",
        storage_class: str | None = None,
    ) -> None:
        path = self._base / key
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)

    async def get(self, key: str) -> bytes:
        path = self._base / key
        if not path.exists():
            raise FileNotFoundError(f"Object not found: {key}")
        return path.read_bytes()

    async def list_objects(self, prefix: str) -> list[ObjectMeta]:
        prefix_path = self._base / prefix
        base_dir = prefix_path.parent if not prefix_path.is_dir() else prefix_path
        if not base_dir.exists():
            return []

        results = []
        for path in sorted(base_dir.rglob("*")):
            if path.is_file():
                rel = str(path.relative_to(self._base))
                if rel.startswith(prefix):
                    stat = path.stat()
                    results.append(
                        ObjectMeta(
                            key=rel,
                            size_bytes=stat.st_size,
                            last_modified=int(stat.st_mtime * 1000),
                        )
                    )
        return results
