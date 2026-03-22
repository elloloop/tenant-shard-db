# mypy: ignore-errors
"""
Azure Blob Storage backend.

Requires: pip install azure-storage-blob
"""

from __future__ import annotations

import logging
from typing import Any

from .base import ObjectMeta

logger = logging.getLogger(__name__)

try:
    from azure.storage.blob.aio import BlobServiceClient

    AZURE_BLOB_AVAILABLE = True
except ImportError:
    AZURE_BLOB_AVAILABLE = False
    BlobServiceClient = None


class AzureBlobObjectStore:
    """Azure Blob Storage implementation."""

    def __init__(self, config: Any) -> None:
        if not AZURE_BLOB_AVAILABLE:
            raise ImportError(
                "azure-storage-blob is required for Azure Blob storage. "
                "pip install azure-storage-blob"
            )
        self._config = config
        self._client = None

    async def connect(self) -> None:
        self._client = BlobServiceClient.from_connection_string(self._config.connection_string)
        logger.info(
            "Connected to Azure Blob Storage",
            extra={"container": self._config.container_name},
        )

    async def close(self) -> None:
        if self._client:
            await self._client.close()
            self._client = None

    async def put(
        self,
        key: str,
        data: bytes,
        content_type: str = "application/octet-stream",
        storage_class: str | None = None,
    ) -> None:
        container = self._client.get_container_client(self._config.container_name)
        blob = container.get_blob_client(key)
        await blob.upload_blob(data, overwrite=True, content_type=content_type)

    async def get(self, key: str) -> bytes:
        container = self._client.get_container_client(self._config.container_name)
        blob = container.get_blob_client(key)
        stream = await blob.download_blob()
        return await stream.readall()

    async def list_objects(self, prefix: str) -> list[ObjectMeta]:
        container = self._client.get_container_client(self._config.container_name)
        results = []
        async for blob in container.list_blobs(name_starts_with=prefix):
            results.append(
                ObjectMeta(
                    key=blob.name,
                    size_bytes=blob.size,
                    last_modified=int(blob.last_modified.timestamp() * 1000)
                    if blob.last_modified
                    else 0,
                )
            )
        return results
