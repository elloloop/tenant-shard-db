"""
Base protocol for object storage.

All storage backends implement three operations:
  - put(key, data) -- upload a blob
  - get(key) -- download a blob
  - list_objects(prefix) -- list blobs by prefix
"""
from __future__ import annotations

import logging
from abc import abstractmethod
from typing import TYPE_CHECKING, Protocol, runtime_checkable

if TYPE_CHECKING:
    from ..config import ServerConfig


logger = logging.getLogger(__name__)


class ObjectMeta:
    """Metadata about a stored object."""

    __slots__ = ("key", "size_bytes", "last_modified")

    def __init__(self, key: str, size_bytes: int, last_modified: int) -> None:
        self.key = key
        self.size_bytes = size_bytes
        self.last_modified = last_modified  # Unix ms


@runtime_checkable
class ObjectStore(Protocol):
    """Protocol for object storage backends."""

    @abstractmethod
    async def connect(self) -> None: ...

    @abstractmethod
    async def close(self) -> None: ...

    @abstractmethod
    async def put(
        self,
        key: str,
        data: bytes,
        content_type: str = "application/octet-stream",
        storage_class: str | None = None,
    ) -> None: ...

    @abstractmethod
    async def get(self, key: str) -> bytes: ...

    @abstractmethod
    async def list_objects(self, prefix: str) -> list[ObjectMeta]: ...


def create_object_store(config: ServerConfig) -> ObjectStore:
    """Factory function to create object store from configuration."""
    from ..config import StorageBackend

    if config.storage_backend == StorageBackend.S3:
        from .s3 import S3ObjectStore

        return S3ObjectStore(config.s3)
    elif config.storage_backend == StorageBackend.AZURE_BLOB:
        from .azure_blob import AzureBlobObjectStore

        return AzureBlobObjectStore(config.azure_blob)
    elif config.storage_backend == StorageBackend.GCS:
        from .gcs import GcsObjectStore

        return GcsObjectStore(config.gcs)
    elif config.storage_backend == StorageBackend.LOCAL_FS:
        from .local_fs import LocalFsObjectStore

        return LocalFsObjectStore(config.local_storage_path)
    else:
        raise ValueError(f"Unsupported storage backend: {config.storage_backend}")
