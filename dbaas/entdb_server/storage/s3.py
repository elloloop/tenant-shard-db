# mypy: ignore-errors
"""
S3-compatible object storage backend.

Works with AWS S3, Cloudflare R2, MinIO, and any S3-compatible service.
"""
from __future__ import annotations

import logging
from typing import Any

from .base import ObjectMeta

logger = logging.getLogger(__name__)

try:
    from aiobotocore.session import get_session

    S3_AVAILABLE = True
except ImportError:
    S3_AVAILABLE = False
    get_session = None


class S3ObjectStore:
    """S3-compatible object storage."""

    def __init__(self, config: Any) -> None:
        if not S3_AVAILABLE:
            raise ImportError("aiobotocore is required for S3 storage. pip install aiobotocore")
        self._config = config
        self._client = None
        self._session = None
        self._ctx = None

    async def connect(self) -> None:
        self._session = get_session()
        kwargs = {"region_name": self._config.region}
        if self._config.endpoint_url:
            kwargs["endpoint_url"] = self._config.endpoint_url
        if self._config.access_key_id:
            kwargs["aws_access_key_id"] = self._config.access_key_id
            kwargs["aws_secret_access_key"] = self._config.secret_access_key
        self._ctx = self._session.create_client("s3", **kwargs)
        self._client = await self._ctx.__aenter__()
        logger.info("Connected to S3", extra={"bucket": self._config.bucket})

    async def close(self) -> None:
        if self._client:
            await self._ctx.__aexit__(None, None, None)
            self._client = None

    async def put(
        self,
        key: str,
        data: bytes,
        content_type: str = "application/octet-stream",
        storage_class: str | None = None,
    ) -> None:
        kwargs = {
            "Bucket": self._config.bucket,
            "Key": key,
            "Body": data,
            "ContentType": content_type,
        }
        if storage_class and storage_class != "STANDARD":
            kwargs["StorageClass"] = storage_class
        await self._client.put_object(**kwargs)

    async def get(self, key: str) -> bytes:
        response = await self._client.get_object(
            Bucket=self._config.bucket,
            Key=key,
        )
        return await response["Body"].read()

    async def list_objects(self, prefix: str) -> list[ObjectMeta]:
        response = await self._client.list_objects_v2(
            Bucket=self._config.bucket,
            Prefix=prefix,
        )
        results = []
        for obj in response.get("Contents", []):
            results.append(
                ObjectMeta(
                    key=obj["Key"],
                    size_bytes=obj["Size"],
                    last_modified=int(obj["LastModified"].timestamp() * 1000),
                )
            )
        return results
