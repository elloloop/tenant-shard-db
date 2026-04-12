"""Tests for WAL-based audit compliance module.

Covers:
- S3 Object Lock configuration
- WAL audit trail export (JSON + CSV)
- Filtering by tenant, actor, time range
- Segment parsing edge cases
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Any

import pytest

from dbaas.entdb_server.audit.compliance import _parse_segment, export_audit_trail
from dbaas.entdb_server.audit.s3_lock import S3ObjectLockConfig, configure_s3_object_lock

# ── Fixtures ─────────────────────────────────────────────────────────


@dataclass
class FakeObjectMeta:
    key: str
    size_bytes: int = 0
    last_modified: int = 0


class FakeObjectStore:
    """In-memory object store for testing."""

    def __init__(self, objects: dict[str, bytes] | None = None):
        self._objects = objects or {}

    async def list_objects(self, prefix: str) -> list[FakeObjectMeta]:
        return [FakeObjectMeta(key=k) for k in sorted(self._objects) if k.startswith(prefix)]

    async def get(self, key: str) -> bytes:
        return self._objects[key]


def _make_wal_segment(*events: dict[str, Any]) -> bytes:
    return "\n".join(json.dumps(e) for e in events).encode()


def _make_event(
    tenant_id: str = "t1",
    actor: str = "user:alice",
    ts_ms: int = 1000,
    ops: list[dict] | None = None,
) -> dict[str, Any]:
    return {
        "tenant_id": tenant_id,
        "actor": actor,
        "idempotency_key": f"key-{ts_ms}",
        "schema_fingerprint": "sha256:abc",
        "ts_ms": ts_ms,
        "ops": ops or [{"op": "create_node", "type_id": 101, "as": "n1"}],
    }


# ── S3 Object Lock ──────────────────────────────────────────────────


class TestS3ObjectLockConfig:
    def test_defaults(self):
        cfg = S3ObjectLockConfig()
        assert cfg.mode == "COMPLIANCE"
        assert cfg.retention_days == 2190
        assert cfg.enable_cloudtrail is True

    def test_custom(self):
        cfg = S3ObjectLockConfig(mode="GOVERNANCE", retention_days=365)
        assert cfg.mode == "GOVERNANCE"
        assert cfg.retention_days == 365


class TestConfigureS3ObjectLock:
    @pytest.mark.asyncio
    async def test_applies_lock_config(self):
        calls = []

        class FakeS3:
            async def put_object_lock_configuration(self, **kwargs):
                calls.append(kwargs)

        result = await configure_s3_object_lock(FakeS3(), "my-bucket")
        assert result["status"] == "applied"
        assert result["bucket"] == "my-bucket"
        assert result["mode"] == "COMPLIANCE"
        assert result["retention_days"] == 2190
        assert len(calls) == 1
        assert calls[0]["Bucket"] == "my-bucket"
        lock = calls[0]["ObjectLockConfiguration"]
        assert lock["Rule"]["DefaultRetention"]["Mode"] == "COMPLIANCE"
        assert lock["Rule"]["DefaultRetention"]["Days"] == 2190

    @pytest.mark.asyncio
    async def test_custom_config(self):
        calls = []

        class FakeS3:
            async def put_object_lock_configuration(self, **kwargs):
                calls.append(kwargs)

        cfg = S3ObjectLockConfig(mode="GOVERNANCE", retention_days=365)
        result = await configure_s3_object_lock(FakeS3(), "bucket", cfg)
        assert result["mode"] == "GOVERNANCE"
        assert result["retention_days"] == 365

    @pytest.mark.asyncio
    async def test_skips_unsupported_client(self):
        result = await configure_s3_object_lock(object(), "bucket")
        assert result["status"] == "skipped"


# ── WAL Audit Export ────────────────────────────────────────────────


class TestExportAuditTrail:
    @pytest.mark.asyncio
    async def test_exports_json(self):
        store = FakeObjectStore(
            {
                "archive/seg-001": _make_wal_segment(
                    _make_event(ts_ms=1000),
                    _make_event(ts_ms=2000, actor="user:bob"),
                ),
            }
        )
        result = await export_audit_trail(store, prefix="archive")
        entries = json.loads(result)
        assert len(entries) == 2
        assert entries[0]["timestamp_ms"] == 1000
        assert entries[1]["actor"] == "user:bob"

    @pytest.mark.asyncio
    async def test_exports_csv(self):
        store = FakeObjectStore(
            {
                "archive/seg-001": _make_wal_segment(_make_event()),
            }
        )
        result = await export_audit_trail(store, prefix="archive", format="csv")
        lines = result.strip().split("\n")
        assert len(lines) == 2
        assert "timestamp_ms" in lines[0]

    @pytest.mark.asyncio
    async def test_filter_by_tenant(self):
        store = FakeObjectStore(
            {
                "archive/seg-001": _make_wal_segment(
                    _make_event(tenant_id="t1"),
                    _make_event(tenant_id="t2"),
                ),
            }
        )
        result = await export_audit_trail(store, prefix="archive", tenant_id="t1")
        entries = json.loads(result)
        assert len(entries) == 1
        assert entries[0]["tenant_id"] == "t1"

    @pytest.mark.asyncio
    async def test_filter_by_actor(self):
        store = FakeObjectStore(
            {
                "archive/seg-001": _make_wal_segment(
                    _make_event(actor="user:alice"),
                    _make_event(actor="user:bob"),
                ),
            }
        )
        result = await export_audit_trail(store, prefix="archive", actor="user:bob")
        entries = json.loads(result)
        assert len(entries) == 1
        assert entries[0]["actor"] == "user:bob"

    @pytest.mark.asyncio
    async def test_filter_by_time_range(self):
        store = FakeObjectStore(
            {
                "archive/seg-001": _make_wal_segment(
                    _make_event(ts_ms=1000),
                    _make_event(ts_ms=2000),
                    _make_event(ts_ms=3000),
                ),
            }
        )
        result = await export_audit_trail(store, prefix="archive", since_ms=1500, until_ms=2500)
        entries = json.loads(result)
        assert len(entries) == 1
        assert entries[0]["timestamp_ms"] == 2000

    @pytest.mark.asyncio
    async def test_multiple_segments_sorted(self):
        store = FakeObjectStore(
            {
                "archive/seg-002": _make_wal_segment(_make_event(ts_ms=3000)),
                "archive/seg-001": _make_wal_segment(_make_event(ts_ms=1000)),
            }
        )
        result = await export_audit_trail(store, prefix="archive")
        entries = json.loads(result)
        assert entries[0]["timestamp_ms"] == 1000
        assert entries[1]["timestamp_ms"] == 3000

    @pytest.mark.asyncio
    async def test_empty_store(self):
        store = FakeObjectStore({})
        result = await export_audit_trail(store, prefix="archive")
        assert json.loads(result) == []

    @pytest.mark.asyncio
    async def test_multiple_ops_per_event(self):
        store = FakeObjectStore(
            {
                "archive/seg-001": _make_wal_segment(
                    _make_event(
                        ops=[
                            {"op": "create_node", "type_id": 101, "as": "n1"},
                            {"op": "create_edge", "type_id": 201},
                        ]
                    ),
                ),
            }
        )
        result = await export_audit_trail(store, prefix="archive")
        entries = json.loads(result)
        assert len(entries) == 2
        assert entries[0]["operation"] == "create_node"
        assert entries[1]["operation"] == "create_edge"

    @pytest.mark.asyncio
    async def test_corrupt_segment_skipped(self):
        store = FakeObjectStore(
            {
                "archive/seg-001": b"not json\n",
                "archive/seg-002": _make_wal_segment(_make_event()),
            }
        )
        result = await export_audit_trail(store, prefix="archive")
        entries = json.loads(result)
        assert len(entries) == 1


class TestParseSegment:
    def test_empty_lines_skipped(self):
        entries: list[dict] = []
        _parse_segment(b"\n\n\n", entries, None, None, None, None)
        assert entries == []

    def test_malformed_json_skipped(self):
        entries: list[dict] = []
        raw = b'{"valid": true}\nnot json\n{"also": true}'
        _parse_segment(raw, entries, None, None, None, None)
        assert len(entries) == 0  # no ops in those events

    def test_event_without_ops(self):
        entries: list[dict] = []
        raw = json.dumps({"tenant_id": "t1", "actor": "a", "ts_ms": 1}).encode()
        _parse_segment(raw, entries, None, None, None, None)
        assert entries == []
