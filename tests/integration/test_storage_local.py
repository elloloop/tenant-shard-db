"""
Local filesystem storage integration tests.

Tests all ObjectStore operations with real filesystem.
"""

from __future__ import annotations

import asyncio
import os

import pytest

from dbaas.entdb_server.storage.local_fs import LocalFsObjectStore


@pytest.fixture
async def obj_store(tmp_path):
    store = LocalFsObjectStore(base_path=str(tmp_path / "storage"))
    await store.connect()
    yield store
    await store.close()


# =========================================================================
# PUT / GET
# =========================================================================


@pytest.mark.integration
class TestLocalFsPutGet:
    """Tests for put and get roundtrip operations."""

    @pytest.mark.asyncio
    async def test_put_and_get_roundtrip(self, obj_store):
        await obj_store.put("file.txt", b"hello world")
        data = await obj_store.get("file.txt")
        assert data == b"hello world"

    @pytest.mark.asyncio
    async def test_put_overwrites_existing(self, obj_store):
        await obj_store.put("file.txt", b"first")
        await obj_store.put("file.txt", b"second")
        data = await obj_store.get("file.txt")
        assert data == b"second"

    @pytest.mark.asyncio
    async def test_get_nonexistent_raises(self, obj_store):
        with pytest.raises(FileNotFoundError):
            await obj_store.get("does-not-exist.txt")

    @pytest.mark.asyncio
    async def test_put_creates_directories(self, obj_store):
        await obj_store.put("a/b/c/deep.txt", b"nested")
        data = await obj_store.get("a/b/c/deep.txt")
        assert data == b"nested"

    @pytest.mark.asyncio
    async def test_put_with_nested_key_path(self, obj_store):
        await obj_store.put("tenants/t1/snapshots/snap.db", b"snapshot-data")
        data = await obj_store.get("tenants/t1/snapshots/snap.db")
        assert data == b"snapshot-data"

    @pytest.mark.asyncio
    async def test_put_binary_data(self, obj_store):
        binary = bytes(range(256))
        await obj_store.put("binary.bin", binary)
        data = await obj_store.get("binary.bin")
        assert data == binary

    @pytest.mark.asyncio
    async def test_put_large_file(self, obj_store):
        large = b"x" * (1024 * 1024)  # 1 MB
        await obj_store.put("large.bin", large)
        data = await obj_store.get("large.bin")
        assert len(data) == 1024 * 1024

    @pytest.mark.asyncio
    async def test_put_empty_file(self, obj_store):
        await obj_store.put("empty.txt", b"")
        data = await obj_store.get("empty.txt")
        assert data == b""

    @pytest.mark.asyncio
    async def test_get_returns_exact_bytes(self, obj_store):
        original = b"\x00\x01\xff\xfe\x80"
        await obj_store.put("exact.bin", original)
        data = await obj_store.get("exact.bin")
        assert data == original

    @pytest.mark.asyncio
    async def test_content_type_parameter_accepted(self, obj_store):
        # content_type is accepted but local FS ignores it
        await obj_store.put("typed.json", b'{"a": 1}', content_type="application/json")
        data = await obj_store.get("typed.json")
        assert data == b'{"a": 1}'


# =========================================================================
# LIST
# =========================================================================


@pytest.mark.integration
class TestLocalFsList:
    """Tests for list_objects operations."""

    @pytest.mark.asyncio
    async def test_list_empty_prefix(self, obj_store):
        result = await obj_store.list_objects("nonexistent/")
        assert result == []

    @pytest.mark.asyncio
    async def test_list_with_matching_files(self, obj_store):
        await obj_store.put("data/a.txt", b"a")
        await obj_store.put("data/b.txt", b"b")
        await obj_store.put("data/c.txt", b"c")

        result = await obj_store.list_objects("data/")
        assert len(result) == 3

    @pytest.mark.asyncio
    async def test_list_returns_correct_metadata(self, obj_store):
        await obj_store.put("meta/file.txt", b"hello")
        result = await obj_store.list_objects("meta/")
        assert len(result) == 1
        assert result[0].key == "meta/file.txt"
        assert result[0].size_bytes == 5
        assert result[0].last_modified > 0

    @pytest.mark.asyncio
    async def test_list_with_nested_directories(self, obj_store):
        await obj_store.put("root/sub1/a.txt", b"a")
        await obj_store.put("root/sub2/b.txt", b"b")
        await obj_store.put("root/c.txt", b"c")

        result = await obj_store.list_objects("root/")
        assert len(result) == 3
        keys = {r.key for r in result}
        assert "root/sub1/a.txt" in keys
        assert "root/sub2/b.txt" in keys
        assert "root/c.txt" in keys

    @pytest.mark.asyncio
    async def test_list_ordering_alphabetical(self, obj_store):
        for name in ["c.txt", "a.txt", "b.txt"]:
            await obj_store.put(f"sorted/{name}", b"x")

        result = await obj_store.list_objects("sorted/")
        keys = [r.key for r in result]
        assert keys == sorted(keys)

    @pytest.mark.asyncio
    async def test_list_after_delete(self, obj_store):
        await obj_store.put("del/a.txt", b"a")
        await obj_store.put("del/b.txt", b"b")

        # Manually delete the file
        os.remove(os.path.join(obj_store._base, "del", "a.txt"))

        result = await obj_store.list_objects("del/")
        assert len(result) == 1
        assert result[0].key == "del/b.txt"

    @pytest.mark.asyncio
    async def test_list_with_many_files(self, obj_store):
        for i in range(100):
            await obj_store.put(f"many/file-{i:03d}.txt", f"data-{i}".encode())

        result = await obj_store.list_objects("many/")
        assert len(result) == 100

    @pytest.mark.asyncio
    async def test_list_with_similar_prefixes(self, obj_store):
        await obj_store.put("prefix/data.txt", b"1")
        await obj_store.put("prefix-extra/data.txt", b"2")

        result = await obj_store.list_objects("prefix/")
        assert len(result) == 1
        assert result[0].key == "prefix/data.txt"

    @pytest.mark.asyncio
    async def test_list_returns_only_files(self, obj_store):
        await obj_store.put("files/sub/a.txt", b"a")
        # The directory "files/sub/" should not appear in results
        result = await obj_store.list_objects("files/")
        for r in result:
            assert not r.key.endswith("/")

    @pytest.mark.asyncio
    async def test_list_across_nested_structures(self, obj_store):
        await obj_store.put("deep/l1/l2/l3/file.txt", b"deep")
        await obj_store.put("deep/l1/file.txt", b"shallow")

        result = await obj_store.list_objects("deep/")
        assert len(result) == 2
        keys = {r.key for r in result}
        assert "deep/l1/l2/l3/file.txt" in keys
        assert "deep/l1/file.txt" in keys


# =========================================================================
# EDGE CASES
# =========================================================================


@pytest.mark.integration
class TestLocalFsEdgeCases:
    """Tests for edge cases and unusual inputs."""

    @pytest.mark.asyncio
    async def test_concurrent_put_same_key(self, obj_store):
        async def write(data: bytes):
            await obj_store.put("concurrent.txt", data)

        await asyncio.gather(
            write(b"writer-1"),
            write(b"writer-2"),
        )
        # One of them should win
        data = await obj_store.get("concurrent.txt")
        assert data in (b"writer-1", b"writer-2")

    @pytest.mark.asyncio
    async def test_put_with_special_chars_in_key(self, obj_store):
        await obj_store.put("special-chars_file.v2.txt", b"special")
        data = await obj_store.get("special-chars_file.v2.txt")
        assert data == b"special"

    @pytest.mark.asyncio
    async def test_put_get_with_unicode_content(self, obj_store):
        text = b"Hello world"
        await obj_store.put("unicode.txt", text)
        data = await obj_store.get("unicode.txt")
        assert data.decode("utf-8") == "Hello world"

    @pytest.mark.asyncio
    async def test_connect_creates_base_directory(self, tmp_path):
        new_path = str(tmp_path / "brand" / "new" / "dir")
        store = LocalFsObjectStore(base_path=new_path)
        await store.connect()
        assert os.path.isdir(new_path)

    @pytest.mark.asyncio
    async def test_close_is_idempotent(self, obj_store):
        await obj_store.close()
        await obj_store.close()  # Should not raise

    @pytest.mark.asyncio
    async def test_operations_after_close_still_work(self, tmp_path):
        """LocalFsObjectStore.close() is a no-op, so operations still work."""
        store = LocalFsObjectStore(base_path=str(tmp_path / "after-close"))
        await store.connect()
        await store.close()
        # Put still works because close is a no-op for local FS
        await store.put("post-close.txt", b"still works")
        data = await store.get("post-close.txt")
        assert data == b"still works"

    @pytest.mark.asyncio
    async def test_very_long_key_path(self, obj_store):
        # Create a key with many segments
        segments = "/".join(f"seg{i}" for i in range(20))
        key = f"{segments}/file.txt"
        await obj_store.put(key, b"deep nesting")
        data = await obj_store.get(key)
        assert data == b"deep nesting"

    @pytest.mark.asyncio
    async def test_put_zero_bytes(self, obj_store):
        await obj_store.put("zero.bin", b"")
        data = await obj_store.get("zero.bin")
        assert data == b""
        result = await obj_store.list_objects("zero")
        assert len(result) == 1
        assert result[0].size_bytes == 0

    @pytest.mark.asyncio
    async def test_list_with_no_trailing_slash(self, obj_store):
        await obj_store.put("notrail/file.txt", b"x")
        # Without trailing slash still works because parent dir match
        result = await obj_store.list_objects("notrail")
        # The file should be found via the prefix matching
        keys = {r.key for r in result}
        assert "notrail/file.txt" in keys

    @pytest.mark.asyncio
    async def test_storage_class_parameter_ignored(self, obj_store):
        await obj_store.put("sc.txt", b"data", storage_class="GLACIER")
        data = await obj_store.get("sc.txt")
        assert data == b"data"


# =========================================================================
# SNAPSHOT OPERATIONS
# =========================================================================


@pytest.mark.integration
class TestLocalFsSnapshotPatterns:
    """Tests simulating snapshot storage patterns."""

    @pytest.mark.asyncio
    async def test_store_and_retrieve_snapshot(self, obj_store):
        snap_data = b"SQLite database bytes here"
        await obj_store.put("snapshots/t1/snap-001.db", snap_data)
        data = await obj_store.get("snapshots/t1/snap-001.db")
        assert data == snap_data

    @pytest.mark.asyncio
    async def test_list_snapshots_for_tenant(self, obj_store):
        for i in range(5):
            await obj_store.put(f"snapshots/t1/snap-{i:03d}.db", f"snap-{i}".encode())

        result = await obj_store.list_objects("snapshots/t1/")
        assert len(result) == 5

    @pytest.mark.asyncio
    async def test_snapshots_isolated_by_tenant(self, obj_store):
        await obj_store.put("snapshots/t1/snap.db", b"t1-data")
        await obj_store.put("snapshots/t2/snap.db", b"t2-data")

        t1 = await obj_store.list_objects("snapshots/t1/")
        t2 = await obj_store.list_objects("snapshots/t2/")
        assert len(t1) == 1
        assert len(t2) == 1
        assert t1[0].key != t2[0].key

    @pytest.mark.asyncio
    async def test_snapshot_manifest_json(self, obj_store):
        import json

        manifest = {"tenant_id": "t1", "last_stream_pos": "wal:0:100", "nodes": 50}
        await obj_store.put(
            "snapshots/t1/manifest.json",
            json.dumps(manifest).encode(),
        )
        data = await obj_store.get("snapshots/t1/manifest.json")
        parsed = json.loads(data)
        assert parsed["nodes"] == 50

    @pytest.mark.asyncio
    async def test_overwrite_snapshot(self, obj_store):
        await obj_store.put("snapshots/t1/latest.db", b"v1")
        await obj_store.put("snapshots/t1/latest.db", b"v2")
        data = await obj_store.get("snapshots/t1/latest.db")
        assert data == b"v2"

    @pytest.mark.asyncio
    async def test_snapshot_size_tracking(self, obj_store):
        data = b"x" * 10240
        await obj_store.put("snapshots/t1/big.db", data)
        result = await obj_store.list_objects("snapshots/t1/")
        assert result[0].size_bytes == 10240

    @pytest.mark.asyncio
    async def test_archive_segment_storage(self, obj_store):
        for i in range(10):
            await obj_store.put(
                f"archive/t1/segment-{i:04d}.jsonl",
                f'{{"event": {i}}}\n'.encode(),
            )
        result = await obj_store.list_objects("archive/t1/")
        assert len(result) == 10

    @pytest.mark.asyncio
    async def test_mixed_content_types(self, obj_store):
        await obj_store.put("files/data.json", b'{"a":1}', content_type="application/json")
        await obj_store.put("files/data.bin", b"\x00\x01", content_type="application/octet-stream")
        await obj_store.put("files/data.txt", b"hello", content_type="text/plain")

        result = await obj_store.list_objects("files/")
        assert len(result) == 3


# =========================================================================
# CONCURRENT AND STRESS PATTERNS
# =========================================================================


@pytest.mark.integration
class TestLocalFsStress:
    """Stress and concurrent access tests."""

    @pytest.mark.asyncio
    async def test_many_small_files(self, obj_store):
        for i in range(200):
            await obj_store.put(f"small/f{i}.txt", f"data-{i}".encode())

        result = await obj_store.list_objects("small/")
        assert len(result) == 200

    @pytest.mark.asyncio
    async def test_concurrent_reads(self, obj_store):
        await obj_store.put("shared.txt", b"shared-data")

        async def read():
            return await obj_store.get("shared.txt")

        results = await asyncio.gather(*[read() for _ in range(20)])
        assert all(r == b"shared-data" for r in results)

    @pytest.mark.asyncio
    async def test_concurrent_different_keys(self, obj_store):
        async def write(key: str, data: bytes):
            await obj_store.put(key, data)

        tasks = [write(f"key-{i}.txt", f"val-{i}".encode()) for i in range(50)]
        await asyncio.gather(*tasks)

        result = await obj_store.list_objects("key-")
        assert len(result) == 50

    @pytest.mark.asyncio
    async def test_rapid_overwrite(self, obj_store):
        for i in range(50):
            await obj_store.put("rapid.txt", f"version-{i}".encode())

        data = await obj_store.get("rapid.txt")
        assert data == b"version-49"

    @pytest.mark.asyncio
    async def test_put_get_various_sizes(self, obj_store):
        sizes = [0, 1, 10, 100, 1000, 10000, 100000]
        for size in sizes:
            key = f"size/{size}.bin"
            data = b"x" * size
            await obj_store.put(key, data)
            retrieved = await obj_store.get(key)
            assert len(retrieved) == size

    @pytest.mark.asyncio
    async def test_list_performance_with_many_files(self, obj_store):
        for i in range(50):
            await obj_store.put(f"perf/file-{i:04d}.txt", b"x")

        result = await obj_store.list_objects("perf/")
        assert len(result) == 50
        # Verify sorting
        keys = [r.key for r in result]
        assert keys == sorted(keys)

    @pytest.mark.asyncio
    async def test_deeply_nested_structure(self, obj_store):
        path = "/".join(f"level{i}" for i in range(10))
        await obj_store.put(f"{path}/leaf.txt", b"deep")
        data = await obj_store.get(f"{path}/leaf.txt")
        assert data == b"deep"

    @pytest.mark.asyncio
    async def test_multiple_stores_same_base(self, tmp_path):
        base = str(tmp_path / "shared-base")
        store1 = LocalFsObjectStore(base_path=base)
        store2 = LocalFsObjectStore(base_path=base)
        await store1.connect()
        await store2.connect()

        await store1.put("from-1.txt", b"one")
        data = await store2.get("from-1.txt")
        assert data == b"one"

    @pytest.mark.asyncio
    async def test_list_returns_last_modified(self, obj_store):
        import time

        await obj_store.put("ts/file.txt", b"data")
        result = await obj_store.list_objects("ts/")
        assert len(result) == 1
        # last_modified should be recent (within last minute)
        now_ms = int(time.time() * 1000)
        assert abs(now_ms - result[0].last_modified) < 60000

    @pytest.mark.asyncio
    async def test_get_after_put_consistency(self, obj_store):
        """Every put is immediately readable."""
        for i in range(30):
            key = f"consist/f{i}.txt"
            data = f"data-{i}".encode()
            await obj_store.put(key, data)
            retrieved = await obj_store.get(key)
            assert retrieved == data

    @pytest.mark.asyncio
    async def test_put_then_list_then_get(self, obj_store):
        """Full lifecycle: put, list, get."""
        for i in range(5):
            await obj_store.put(f"lifecycle/item-{i}.txt", f"content-{i}".encode())

        listed = await obj_store.list_objects("lifecycle/")
        assert len(listed) == 5

        for meta in listed:
            data = await obj_store.get(meta.key)
            assert data is not None
            assert len(data) > 0

    @pytest.mark.asyncio
    async def test_overwrite_changes_size(self, obj_store):
        await obj_store.put("grow.txt", b"short")
        r1 = await obj_store.list_objects("grow")
        assert r1[0].size_bytes == 5

        await obj_store.put("grow.txt", b"much longer content here")
        r2 = await obj_store.list_objects("grow")
        assert r2[0].size_bytes == 24

    @pytest.mark.asyncio
    async def test_separate_prefixes_independent(self, obj_store):
        await obj_store.put("prefix-a/file.txt", b"a")
        await obj_store.put("prefix-b/file.txt", b"b")

        a = await obj_store.list_objects("prefix-a/")
        b = await obj_store.list_objects("prefix-b/")
        assert len(a) == 1
        assert len(b) == 1
        assert a[0].key != b[0].key

    @pytest.mark.asyncio
    async def test_json_roundtrip(self, obj_store):
        import json

        obj = {"users": [{"name": "Alice"}, {"name": "Bob"}], "count": 2}
        data = json.dumps(obj).encode()
        await obj_store.put("data.json", data)
        retrieved = json.loads(await obj_store.get("data.json"))
        assert retrieved == obj

    @pytest.mark.asyncio
    async def test_binary_roundtrip_all_bytes(self, obj_store):
        data = bytes(range(256)) * 10
        await obj_store.put("allbytes.bin", data)
        retrieved = await obj_store.get("allbytes.bin")
        assert retrieved == data

    @pytest.mark.asyncio
    async def test_list_objects_returns_object_meta(self, obj_store):
        from dbaas.entdb_server.storage.base import ObjectMeta

        await obj_store.put("meta-test/x.txt", b"hello")
        result = await obj_store.list_objects("meta-test/")
        assert len(result) == 1
        meta = result[0]
        assert isinstance(meta, ObjectMeta)
        assert meta.key == "meta-test/x.txt"
        assert meta.size_bytes == 5

    @pytest.mark.asyncio
    async def test_concurrent_list_and_put(self, obj_store):
        async def put_files():
            for i in range(10):
                await obj_store.put(f"conc/f{i}.txt", b"x")

        async def list_files():
            return await obj_store.list_objects("conc/")

        await put_files()
        results = await asyncio.gather(list_files(), list_files(), list_files())
        for r in results:
            assert len(r) == 10

    @pytest.mark.asyncio
    async def test_put_get_sequential_keys(self, obj_store):
        for i in range(20):
            await obj_store.put(f"seq/{i:04d}.dat", f"data-{i}".encode())

        for i in range(20):
            data = await obj_store.get(f"seq/{i:04d}.dat")
            assert data == f"data-{i}".encode()

    @pytest.mark.asyncio
    async def test_list_size_varies(self, obj_store):
        await obj_store.put("sizes/small.txt", b"x")
        await obj_store.put("sizes/medium.txt", b"x" * 100)
        await obj_store.put("sizes/large.txt", b"x" * 10000)

        result = await obj_store.list_objects("sizes/")
        sizes = {r.key.split("/")[1]: r.size_bytes for r in result}
        assert sizes["small.txt"] == 1
        assert sizes["medium.txt"] == 100
        assert sizes["large.txt"] == 10000

    @pytest.mark.asyncio
    async def test_put_replaces_content_completely(self, obj_store):
        await obj_store.put("replace.txt", b"original-long-content")
        await obj_store.put("replace.txt", b"short")
        data = await obj_store.get("replace.txt")
        assert data == b"short"

    @pytest.mark.asyncio
    async def test_get_nonexistent_after_connect(self, tmp_path):
        store = LocalFsObjectStore(base_path=str(tmp_path / "fresh"))
        await store.connect()
        with pytest.raises(FileNotFoundError):
            await store.get("missing.txt")

    @pytest.mark.asyncio
    async def test_list_empty_dir_returns_empty(self, obj_store):
        result = await obj_store.list_objects("totally-empty/")
        assert result == []

    @pytest.mark.asyncio
    async def test_put_and_list_single_file(self, obj_store):
        await obj_store.put("single/only.txt", b"alone")
        result = await obj_store.list_objects("single/")
        assert len(result) == 1
        assert result[0].key == "single/only.txt"
