"""
Mock-based behavior tests for storage backends.

These tests verify that each backend calls the correct cloud SDK methods
with the right arguments, without needing actual cloud services.

Backends tested:
  - S3ObjectStore (aiobotocore / S3)
  - AzureBlobObjectStore (azure-storage-blob)
  - GcsObjectStore (google-cloud-storage)

LocalFsObjectStore is tested separately in test_storage_config.py
with real filesystem operations.
"""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock, patch

import pytest

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_s3_config(**overrides):
    cfg = MagicMock()
    cfg.bucket = "entdb-storage"
    cfg.region = "us-east-1"
    cfg.endpoint_url = None
    cfg.access_key_id = None
    cfg.secret_access_key = None
    for k, v in overrides.items():
        setattr(cfg, k, v)
    return cfg


def _make_azure_blob_config(**overrides):
    cfg = MagicMock()
    cfg.connection_string = "DefaultEndpointsProtocol=https;AccountName=test"
    cfg.container_name = "entdb-container"
    for k, v in overrides.items():
        setattr(cfg, k, v)
    return cfg


def _make_gcs_config(**overrides):
    cfg = MagicMock()
    cfg.project_id = "my-project"
    cfg.bucket_name = "entdb-bucket"
    for k, v in overrides.items():
        setattr(cfg, k, v)
    return cfg


# ===================================================================
# S3
# ===================================================================


@pytest.mark.unit
class TestS3ObjectStore:
    """Tests for S3ObjectStore using mocked aiobotocore."""

    def _setup_s3_mocks(self):
        """Create mocked aiobotocore session and client."""
        mock_session = MagicMock()
        mock_client_ctx = AsyncMock()
        mock_client = AsyncMock()
        mock_client_ctx.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client_ctx.__aexit__ = AsyncMock(return_value=False)
        mock_session.create_client.return_value = mock_client_ctx
        return mock_session, mock_client_ctx, mock_client

    @pytest.mark.asyncio
    async def test_connect_creates_s3_client(self):
        """connect() creates an aiobotocore S3 client."""
        mock_session, _mock_ctx, _mock_client = self._setup_s3_mocks()

        with (
            patch("dbaas.entdb_server.storage.s3.S3_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.s3.get_session",
                return_value=mock_session,
            ),
        ):
            from dbaas.entdb_server.storage.s3 import S3ObjectStore

            store = S3ObjectStore(_make_s3_config())
            await store.connect()

            mock_session.create_client.assert_called_once_with(
                "s3", region_name="us-east-1"
            )

    @pytest.mark.asyncio
    async def test_connect_with_endpoint_url(self):
        """connect() passes endpoint_url when configured (e.g., for MinIO/R2)."""
        mock_session, _mock_ctx, _mock_client = self._setup_s3_mocks()

        with (
            patch("dbaas.entdb_server.storage.s3.S3_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.s3.get_session",
                return_value=mock_session,
            ),
        ):
            from dbaas.entdb_server.storage.s3 import S3ObjectStore

            config = _make_s3_config(
                endpoint_url="http://localhost:9000",
                access_key_id="minioadmin",
                secret_access_key="minioadmin",
            )
            store = S3ObjectStore(config)
            await store.connect()

            call_kwargs = mock_session.create_client.call_args[1]
            assert call_kwargs["endpoint_url"] == "http://localhost:9000"
            assert call_kwargs["aws_access_key_id"] == "minioadmin"
            assert call_kwargs["aws_secret_access_key"] == "minioadmin"

    @pytest.mark.asyncio
    async def test_put_calls_put_object(self):
        """put() calls put_object with Bucket, Key, Body."""
        mock_session, _mock_ctx, mock_client = self._setup_s3_mocks()

        with (
            patch("dbaas.entdb_server.storage.s3.S3_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.s3.get_session",
                return_value=mock_session,
            ),
        ):
            from dbaas.entdb_server.storage.s3 import S3ObjectStore

            store = S3ObjectStore(_make_s3_config())
            await store.connect()
            await store.put("snapshots/2024/data.bin", b"binary-data")

            mock_client.put_object.assert_awaited_once_with(
                Bucket="entdb-storage",
                Key="snapshots/2024/data.bin",
                Body=b"binary-data",
                ContentType="application/octet-stream",
            )

    @pytest.mark.asyncio
    async def test_put_with_storage_class(self):
        """put() passes StorageClass when it is not STANDARD."""
        mock_session, _mock_ctx, mock_client = self._setup_s3_mocks()

        with (
            patch("dbaas.entdb_server.storage.s3.S3_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.s3.get_session",
                return_value=mock_session,
            ),
        ):
            from dbaas.entdb_server.storage.s3 import S3ObjectStore

            store = S3ObjectStore(_make_s3_config())
            await store.connect()
            await store.put(
                "archive/old.bin",
                b"data",
                storage_class="GLACIER",
            )

            call_kwargs = mock_client.put_object.call_args[1]
            assert call_kwargs["StorageClass"] == "GLACIER"

    @pytest.mark.asyncio
    async def test_put_standard_storage_class_omitted(self):
        """put() does NOT include StorageClass when it is STANDARD."""
        mock_session, _mock_ctx, mock_client = self._setup_s3_mocks()

        with (
            patch("dbaas.entdb_server.storage.s3.S3_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.s3.get_session",
                return_value=mock_session,
            ),
        ):
            from dbaas.entdb_server.storage.s3 import S3ObjectStore

            store = S3ObjectStore(_make_s3_config())
            await store.connect()
            await store.put("key", b"data", storage_class="STANDARD")

            call_kwargs = mock_client.put_object.call_args[1]
            assert "StorageClass" not in call_kwargs

    @pytest.mark.asyncio
    async def test_get_calls_get_object_and_reads_body(self):
        """get() calls get_object and reads the Body stream."""
        mock_session, _mock_ctx, mock_client = self._setup_s3_mocks()
        mock_body = AsyncMock()
        mock_body.read.return_value = b"file-content"
        mock_client.get_object.return_value = {"Body": mock_body}

        with (
            patch("dbaas.entdb_server.storage.s3.S3_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.s3.get_session",
                return_value=mock_session,
            ),
        ):
            from dbaas.entdb_server.storage.s3 import S3ObjectStore

            store = S3ObjectStore(_make_s3_config())
            await store.connect()
            result = await store.get("snapshots/data.bin")

            mock_client.get_object.assert_awaited_once_with(
                Bucket="entdb-storage",
                Key="snapshots/data.bin",
            )
            mock_body.read.assert_awaited_once()
            assert result == b"file-content"

    @pytest.mark.asyncio
    async def test_list_objects_calls_list_objects_v2(self):
        """list_objects() calls list_objects_v2 with Prefix."""
        mock_session, _mock_ctx, mock_client = self._setup_s3_mocks()
        mock_last_modified = MagicMock()
        mock_last_modified.timestamp.return_value = 1700000000.0
        mock_client.list_objects_v2.return_value = {
            "Contents": [
                {
                    "Key": "snapshots/a.bin",
                    "Size": 1024,
                    "LastModified": mock_last_modified,
                },
                {
                    "Key": "snapshots/b.bin",
                    "Size": 2048,
                    "LastModified": mock_last_modified,
                },
            ]
        }

        with (
            patch("dbaas.entdb_server.storage.s3.S3_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.s3.get_session",
                return_value=mock_session,
            ),
        ):
            from dbaas.entdb_server.storage.s3 import S3ObjectStore

            store = S3ObjectStore(_make_s3_config())
            await store.connect()
            results = await store.list_objects("snapshots/")

            mock_client.list_objects_v2.assert_awaited_once_with(
                Bucket="entdb-storage",
                Prefix="snapshots/",
            )
            assert len(results) == 2
            assert results[0].key == "snapshots/a.bin"
            assert results[0].size_bytes == 1024
            assert results[1].key == "snapshots/b.bin"

    @pytest.mark.asyncio
    async def test_list_objects_empty_bucket(self):
        """list_objects() returns empty list when no Contents key."""
        mock_session, _mock_ctx, mock_client = self._setup_s3_mocks()
        mock_client.list_objects_v2.return_value = {}

        with (
            patch("dbaas.entdb_server.storage.s3.S3_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.s3.get_session",
                return_value=mock_session,
            ),
        ):
            from dbaas.entdb_server.storage.s3 import S3ObjectStore

            store = S3ObjectStore(_make_s3_config())
            await store.connect()
            results = await store.list_objects("empty/")
            assert results == []

    @pytest.mark.asyncio
    async def test_close_exits_client_context(self):
        """close() exits the aiobotocore client context."""
        mock_session, mock_ctx, _mock_client = self._setup_s3_mocks()

        with (
            patch("dbaas.entdb_server.storage.s3.S3_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.s3.get_session",
                return_value=mock_session,
            ),
        ):
            from dbaas.entdb_server.storage.s3 import S3ObjectStore

            store = S3ObjectStore(_make_s3_config())
            await store.connect()
            await store.close()

            mock_ctx.__aexit__.assert_awaited_once_with(None, None, None)
            assert store._client is None


# ===================================================================
# Azure Blob
# ===================================================================


@pytest.mark.unit
class TestAzureBlobObjectStore:
    """Tests for AzureBlobObjectStore using mocked azure-storage-blob."""

    @pytest.mark.asyncio
    async def test_connect_creates_client_from_connection_string(self):
        """connect() creates BlobServiceClient from connection string."""
        mock_client = MagicMock()
        mock_cls = MagicMock()
        mock_cls.from_connection_string.return_value = mock_client

        with (
            patch(
                "dbaas.entdb_server.storage.azure_blob.AZURE_BLOB_AVAILABLE", True
            ),
            patch(
                "dbaas.entdb_server.storage.azure_blob.BlobServiceClient",
                mock_cls,
            ),
        ):
            from dbaas.entdb_server.storage.azure_blob import AzureBlobObjectStore

            config = _make_azure_blob_config()
            store = AzureBlobObjectStore(config)
            await store.connect()

            mock_cls.from_connection_string.assert_called_once_with(
                config.connection_string
            )

    @pytest.mark.asyncio
    async def test_put_uploads_blob(self):
        """put() uploads data to the correct container and key."""
        mock_client = MagicMock()
        mock_container = MagicMock()
        mock_blob = AsyncMock()
        mock_client.get_container_client.return_value = mock_container
        mock_container.get_blob_client.return_value = mock_blob

        mock_cls = MagicMock()
        mock_cls.from_connection_string.return_value = mock_client

        with (
            patch(
                "dbaas.entdb_server.storage.azure_blob.AZURE_BLOB_AVAILABLE", True
            ),
            patch(
                "dbaas.entdb_server.storage.azure_blob.BlobServiceClient",
                mock_cls,
            ),
        ):
            from dbaas.entdb_server.storage.azure_blob import AzureBlobObjectStore

            store = AzureBlobObjectStore(_make_azure_blob_config())
            await store.connect()
            await store.put("snapshots/data.bin", b"content")

            mock_client.get_container_client.assert_called_with("entdb-container")
            mock_container.get_blob_client.assert_called_with("snapshots/data.bin")
            mock_blob.upload_blob.assert_awaited_once_with(
                b"content",
                overwrite=True,
                content_type="application/octet-stream",
            )

    @pytest.mark.asyncio
    async def test_put_with_custom_content_type(self):
        """put() passes content_type through to upload_blob."""
        mock_client = MagicMock()
        mock_container = MagicMock()
        mock_blob = AsyncMock()
        mock_client.get_container_client.return_value = mock_container
        mock_container.get_blob_client.return_value = mock_blob

        mock_cls = MagicMock()
        mock_cls.from_connection_string.return_value = mock_client

        with (
            patch(
                "dbaas.entdb_server.storage.azure_blob.AZURE_BLOB_AVAILABLE", True
            ),
            patch(
                "dbaas.entdb_server.storage.azure_blob.BlobServiceClient",
                mock_cls,
            ),
        ):
            from dbaas.entdb_server.storage.azure_blob import AzureBlobObjectStore

            store = AzureBlobObjectStore(_make_azure_blob_config())
            await store.connect()
            await store.put("f.json", b"{}", content_type="application/json")

            call_kwargs = mock_blob.upload_blob.call_args
            assert call_kwargs[1]["content_type"] == "application/json"

    @pytest.mark.asyncio
    async def test_get_downloads_blob(self):
        """get() downloads and returns the blob content."""
        mock_client = MagicMock()
        mock_container = MagicMock()
        mock_blob = AsyncMock()
        mock_stream = AsyncMock()
        mock_stream.readall.return_value = b"downloaded-content"
        mock_blob.download_blob.return_value = mock_stream
        mock_client.get_container_client.return_value = mock_container
        mock_container.get_blob_client.return_value = mock_blob

        mock_cls = MagicMock()
        mock_cls.from_connection_string.return_value = mock_client

        with (
            patch(
                "dbaas.entdb_server.storage.azure_blob.AZURE_BLOB_AVAILABLE", True
            ),
            patch(
                "dbaas.entdb_server.storage.azure_blob.BlobServiceClient",
                mock_cls,
            ),
        ):
            from dbaas.entdb_server.storage.azure_blob import AzureBlobObjectStore

            store = AzureBlobObjectStore(_make_azure_blob_config())
            await store.connect()
            result = await store.get("snapshots/data.bin")

            mock_container.get_blob_client.assert_called_with("snapshots/data.bin")
            mock_blob.download_blob.assert_awaited_once()
            mock_stream.readall.assert_awaited_once()
            assert result == b"downloaded-content"

    @pytest.mark.asyncio
    async def test_list_objects_lists_blobs_with_prefix(self):
        """list_objects() calls list_blobs with name_starts_with prefix."""
        mock_client = MagicMock()
        mock_container = MagicMock()

        mock_last_modified = MagicMock()
        mock_last_modified.timestamp.return_value = 1700000000.0

        blob1 = MagicMock()
        blob1.name = "snapshots/a.bin"
        blob1.size = 100
        blob1.last_modified = mock_last_modified

        blob2 = MagicMock()
        blob2.name = "snapshots/b.bin"
        blob2.size = 200
        blob2.last_modified = mock_last_modified

        # Simulate async iterator
        async def _async_iter(*_args, **_kwargs):
            for b in [blob1, blob2]:
                yield b

        mock_container.list_blobs = _async_iter

        mock_client.get_container_client.return_value = mock_container

        mock_cls = MagicMock()
        mock_cls.from_connection_string.return_value = mock_client

        with (
            patch(
                "dbaas.entdb_server.storage.azure_blob.AZURE_BLOB_AVAILABLE", True
            ),
            patch(
                "dbaas.entdb_server.storage.azure_blob.BlobServiceClient",
                mock_cls,
            ),
        ):
            from dbaas.entdb_server.storage.azure_blob import AzureBlobObjectStore

            store = AzureBlobObjectStore(_make_azure_blob_config())
            await store.connect()
            results = await store.list_objects("snapshots/")

            assert len(results) == 2
            assert results[0].key == "snapshots/a.bin"
            assert results[0].size_bytes == 100
            assert results[1].key == "snapshots/b.bin"

    @pytest.mark.asyncio
    async def test_close_closes_client(self):
        """close() calls client.close()."""
        mock_client = AsyncMock()
        mock_cls = MagicMock()
        mock_cls.from_connection_string.return_value = mock_client

        with (
            patch(
                "dbaas.entdb_server.storage.azure_blob.AZURE_BLOB_AVAILABLE", True
            ),
            patch(
                "dbaas.entdb_server.storage.azure_blob.BlobServiceClient",
                mock_cls,
            ),
        ):
            from dbaas.entdb_server.storage.azure_blob import AzureBlobObjectStore

            store = AzureBlobObjectStore(_make_azure_blob_config())
            await store.connect()
            await store.close()

            mock_client.close.assert_awaited_once()
            assert store._client is None


# ===================================================================
# GCS
# ===================================================================


@pytest.mark.unit
class TestGcsObjectStore:
    """Tests for GcsObjectStore using mocked google-cloud-storage."""

    @pytest.mark.asyncio
    async def test_connect_creates_client_and_bucket(self):
        """connect() creates storage.Client and gets the bucket."""
        mock_gcs_client = MagicMock()
        mock_bucket = MagicMock()
        mock_gcs_client.bucket.return_value = mock_bucket

        mock_storage_mod = MagicMock()
        mock_storage_mod.Client.return_value = mock_gcs_client

        with (
            patch("dbaas.entdb_server.storage.gcs.GCS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.gcs.gcs_storage",
                mock_storage_mod,
            ),
        ):
            from dbaas.entdb_server.storage.gcs import GcsObjectStore

            store = GcsObjectStore(_make_gcs_config())
            await store.connect()

            mock_storage_mod.Client.assert_called_once_with(project="my-project")
            mock_gcs_client.bucket.assert_called_once_with("entdb-bucket")
            assert store._client is mock_gcs_client
            assert store._bucket is mock_bucket

    @pytest.mark.asyncio
    async def test_put_uploads_from_string(self):
        """put() calls blob.upload_from_string with data and content_type."""
        mock_gcs_client = MagicMock()
        mock_bucket = MagicMock()
        mock_blob = MagicMock()
        mock_gcs_client.bucket.return_value = mock_bucket
        mock_bucket.blob.return_value = mock_blob

        mock_storage_mod = MagicMock()
        mock_storage_mod.Client.return_value = mock_gcs_client

        with (
            patch("dbaas.entdb_server.storage.gcs.GCS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.gcs.gcs_storage",
                mock_storage_mod,
            ),
        ):
            from dbaas.entdb_server.storage.gcs import GcsObjectStore

            store = GcsObjectStore(_make_gcs_config())
            await store.connect()
            await store.put("snapshots/data.bin", b"content")

            mock_bucket.blob.assert_called_with("snapshots/data.bin")
            mock_blob.upload_from_string.assert_called_once_with(
                b"content", content_type="application/octet-stream"
            )

    @pytest.mark.asyncio
    async def test_put_with_storage_class(self):
        """put() sets blob.storage_class when storage_class is provided."""
        mock_gcs_client = MagicMock()
        mock_bucket = MagicMock()
        mock_blob = MagicMock()
        mock_gcs_client.bucket.return_value = mock_bucket
        mock_bucket.blob.return_value = mock_blob

        mock_storage_mod = MagicMock()
        mock_storage_mod.Client.return_value = mock_gcs_client

        with (
            patch("dbaas.entdb_server.storage.gcs.GCS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.gcs.gcs_storage",
                mock_storage_mod,
            ),
        ):
            from dbaas.entdb_server.storage.gcs import GcsObjectStore

            store = GcsObjectStore(_make_gcs_config())
            await store.connect()
            await store.put("key", b"data", storage_class="COLDLINE")

            assert mock_blob.storage_class == "COLDLINE"

    @pytest.mark.asyncio
    async def test_get_downloads_as_bytes(self):
        """get() calls blob.download_as_bytes and returns the result."""
        mock_gcs_client = MagicMock()
        mock_bucket = MagicMock()
        mock_blob = MagicMock()
        mock_blob.download_as_bytes.return_value = b"file-content"
        mock_gcs_client.bucket.return_value = mock_bucket
        mock_bucket.blob.return_value = mock_blob

        mock_storage_mod = MagicMock()
        mock_storage_mod.Client.return_value = mock_gcs_client

        with (
            patch("dbaas.entdb_server.storage.gcs.GCS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.gcs.gcs_storage",
                mock_storage_mod,
            ),
        ):
            from dbaas.entdb_server.storage.gcs import GcsObjectStore

            store = GcsObjectStore(_make_gcs_config())
            await store.connect()
            result = await store.get("snapshots/data.bin")

            mock_bucket.blob.assert_called_with("snapshots/data.bin")
            mock_blob.download_as_bytes.assert_called_once()
            assert result == b"file-content"

    @pytest.mark.asyncio
    async def test_list_objects_calls_list_blobs(self):
        """list_objects() calls client.list_blobs with bucket and prefix."""
        mock_gcs_client = MagicMock()
        mock_bucket = MagicMock()
        mock_gcs_client.bucket.return_value = mock_bucket

        mock_updated = MagicMock()
        mock_updated.timestamp.return_value = 1700000000.0

        blob1 = MagicMock()
        blob1.name = "snapshots/a.bin"
        blob1.size = 512
        blob1.updated = mock_updated

        blob2 = MagicMock()
        blob2.name = "snapshots/b.bin"
        blob2.size = 1024
        blob2.updated = mock_updated

        mock_gcs_client.list_blobs.return_value = [blob1, blob2]

        mock_storage_mod = MagicMock()
        mock_storage_mod.Client.return_value = mock_gcs_client

        with (
            patch("dbaas.entdb_server.storage.gcs.GCS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.gcs.gcs_storage",
                mock_storage_mod,
            ),
        ):
            from dbaas.entdb_server.storage.gcs import GcsObjectStore

            store = GcsObjectStore(_make_gcs_config())
            await store.connect()
            results = await store.list_objects("snapshots/")

            mock_gcs_client.list_blobs.assert_called_once_with(
                mock_bucket, prefix="snapshots/"
            )
            assert len(results) == 2
            assert results[0].key == "snapshots/a.bin"
            assert results[0].size_bytes == 512
            assert results[1].key == "snapshots/b.bin"
            assert results[1].size_bytes == 1024

    @pytest.mark.asyncio
    async def test_list_objects_empty(self):
        """list_objects() returns empty list when no blobs match."""
        mock_gcs_client = MagicMock()
        mock_bucket = MagicMock()
        mock_gcs_client.bucket.return_value = mock_bucket
        mock_gcs_client.list_blobs.return_value = []

        mock_storage_mod = MagicMock()
        mock_storage_mod.Client.return_value = mock_gcs_client

        with (
            patch("dbaas.entdb_server.storage.gcs.GCS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.gcs.gcs_storage",
                mock_storage_mod,
            ),
        ):
            from dbaas.entdb_server.storage.gcs import GcsObjectStore

            store = GcsObjectStore(_make_gcs_config())
            await store.connect()
            results = await store.list_objects("empty/")
            assert results == []

    @pytest.mark.asyncio
    async def test_close_closes_client(self):
        """close() calls client.close() and clears references."""
        mock_gcs_client = MagicMock()
        mock_bucket = MagicMock()
        mock_gcs_client.bucket.return_value = mock_bucket

        mock_storage_mod = MagicMock()
        mock_storage_mod.Client.return_value = mock_gcs_client

        with (
            patch("dbaas.entdb_server.storage.gcs.GCS_AVAILABLE", True),
            patch(
                "dbaas.entdb_server.storage.gcs.gcs_storage",
                mock_storage_mod,
            ),
        ):
            from dbaas.entdb_server.storage.gcs import GcsObjectStore

            store = GcsObjectStore(_make_gcs_config())
            await store.connect()
            await store.close()

            mock_gcs_client.close.assert_called_once()
            assert store._client is None
            assert store._bucket is None
