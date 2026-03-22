"""Unit tests for object storage configurations."""

import pytest

from dbaas.entdb_server.config import (
    AzureBlobConfig,
    GcsConfig,
    StorageBackend,
)


@pytest.mark.unit
class TestStorageBackendEnum:
    def test_all_backends(self):
        assert StorageBackend.S3.value == "s3"
        assert StorageBackend.AZURE_BLOB.value == "azure_blob"
        assert StorageBackend.GCS.value == "gcs"
        assert StorageBackend.LOCAL_FS.value == "local"


@pytest.mark.unit
class TestAzureBlobConfig:
    def test_defaults(self):
        config = AzureBlobConfig()
        assert config.container_name == "entdb-storage"
        assert config.connection_string == ""

    def test_from_env(self, monkeypatch):
        monkeypatch.setenv(
            "AZURE_BLOB_CONNECTION_STRING", "DefaultEndpointsProtocol=https;AccountName=test"
        )
        monkeypatch.setenv("AZURE_BLOB_CONTAINER", "my-container")
        config = AzureBlobConfig.from_env()
        assert "test" in config.connection_string
        assert config.container_name == "my-container"


@pytest.mark.unit
class TestGcsConfig:
    def test_defaults(self):
        config = GcsConfig()
        assert config.bucket_name == "entdb-storage"
        assert config.project_id == ""

    def test_from_env(self, monkeypatch):
        monkeypatch.setenv("GCS_PROJECT_ID", "my-project")
        monkeypatch.setenv("GCS_BUCKET", "my-bucket")
        config = GcsConfig.from_env()
        assert config.project_id == "my-project"
        assert config.bucket_name == "my-bucket"

    def test_gcp_project_fallback(self, monkeypatch):
        monkeypatch.setenv("GCP_PROJECT_ID", "fallback")
        config = GcsConfig.from_env()
        assert config.project_id == "fallback"


@pytest.mark.unit
class TestLocalFsStore:
    @pytest.mark.asyncio
    async def test_put_get_list(self, tmp_path):
        from dbaas.entdb_server.storage.local_fs import LocalFsObjectStore

        store = LocalFsObjectStore(str(tmp_path))
        await store.connect()

        await store.put("test/file1.txt", b"hello world")
        await store.put("test/file2.txt", b"goodbye")

        data = await store.get("test/file1.txt")
        assert data == b"hello world"

        objects = await store.list_objects("test/")
        assert len(objects) == 2
        assert objects[0].key == "test/file1.txt"
        assert objects[0].size_bytes == 11

        await store.close()

    @pytest.mark.asyncio
    async def test_get_missing_raises(self, tmp_path):
        from dbaas.entdb_server.storage.local_fs import LocalFsObjectStore

        store = LocalFsObjectStore(str(tmp_path))
        await store.connect()
        with pytest.raises(FileNotFoundError):
            await store.get("nonexistent")

    @pytest.mark.asyncio
    async def test_list_empty_prefix(self, tmp_path):
        from dbaas.entdb_server.storage.local_fs import LocalFsObjectStore

        store = LocalFsObjectStore(str(tmp_path))
        await store.connect()
        objects = await store.list_objects("nothing/")
        assert objects == []
