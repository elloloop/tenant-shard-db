# mypy: ignore-errors
"""
Google Cloud Storage (GCS) backend.

Requires: pip install google-cloud-storage
"""

from __future__ import annotations

import asyncio
import logging
from typing import Any

from .base import ObjectMeta

logger = logging.getLogger(__name__)

try:
    from google.cloud import storage as gcs_storage

    GCS_AVAILABLE = True
except ImportError:
    GCS_AVAILABLE = False
    gcs_storage = None


class GcsObjectStore:
    """Google Cloud Storage implementation.

    Uses the synchronous GCS client wrapped in run_in_executor
    because the async GCS client is not mature.
    """

    def __init__(self, config: Any) -> None:
        if not GCS_AVAILABLE:
            raise ImportError(
                "google-cloud-storage is required for GCS storage. pip install google-cloud-storage"
            )
        self._config = config
        self._client = None
        self._bucket = None

    async def connect(self) -> None:
        loop = asyncio.get_event_loop()

        def _connect():
            client = gcs_storage.Client(project=self._config.project_id)
            bucket = client.bucket(self._config.bucket_name)
            return client, bucket

        self._client, self._bucket = await loop.run_in_executor(None, _connect)
        logger.info("Connected to GCS", extra={"bucket": self._config.bucket_name})

    async def close(self) -> None:
        if self._client:
            self._client.close()
            self._client = None
            self._bucket = None

    async def put(
        self,
        key: str,
        data: bytes,
        content_type: str = "application/octet-stream",
        storage_class: str | None = None,
    ) -> None:
        loop = asyncio.get_event_loop()

        def _put():
            blob = self._bucket.blob(key)
            if storage_class:
                blob.storage_class = storage_class
            blob.upload_from_string(data, content_type=content_type)

        await loop.run_in_executor(None, _put)

    async def get(self, key: str) -> bytes:
        loop = asyncio.get_event_loop()

        def _get():
            blob = self._bucket.blob(key)
            return blob.download_as_bytes()

        return await loop.run_in_executor(None, _get)

    async def list_objects(self, prefix: str) -> list[ObjectMeta]:
        loop = asyncio.get_event_loop()

        def _list():
            blobs = self._client.list_blobs(self._bucket, prefix=prefix)
            return [
                ObjectMeta(
                    key=b.name,
                    size_bytes=b.size or 0,
                    last_modified=int(b.updated.timestamp() * 1000) if b.updated else 0,
                )
                for b in blobs
            ]

        return await loop.run_in_executor(None, _list)
