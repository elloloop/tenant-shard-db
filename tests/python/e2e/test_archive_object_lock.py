# SPDX-License-Identifier: AGPL-3.0-only
"""
EntDB Archive E2E — WAL -> S3 Object Lock round-trip (EPIC #511, ADR-015).

Runs only in the dedicated archive stack (docker-compose.archive.yml):
Redpanda + MinIO (Object-Lock COMPLIANCE bucket) + entdb-server with
``-archive-enabled`` and a Prometheus ``/metrics`` endpoint.

What this asserts (success criteria 1, 2, 5 from issue #511):

1. Writing nodes through the SDK lands events on the Kafka WAL.
2. The archive sidecar continuously copies those events into S3 with
   ``ObjectLockMode=COMPLIANCE`` and a retain-until date.
3. The archived object is gzip JSONL and round-trips back to the
   events we wrote (tenant id is present in the decoded payload).
4. The ``entdb_archive_lag_events`` Prometheus gauge is exported and
   drains back to 0 once the archiver has caught up — the operator
   alert signal named in ADR-015's failure-modes section.
5. Legal-hold lift (EPIC #511 Gap 1): with the tenant under legal hold
   the archiver writes objects with ``LegalHold=ON``; an explicit
   ``SetLegalHold`` OFF durably enqueues a lift and the crash-durable
   background worker (NOT a request-scoped goroutine) then lifts the
   hold on those already-written objects while the
   ``ObjectLockMode=COMPLIANCE`` retention stays exactly as written
   (immutable — ADR-015). The ``entdb_legal_hold_lift_pending`` gauge is
   exported and drains to 0 once the worker has finished — the operator
   signal the old detached goroutine never provided.
"""

from __future__ import annotations

import gzip
import io
import json
import os
import time

import pytest

# This module only runs inside the dedicated archive stack
# (docker-compose.archive.yml / Dockerfile.archive). The fast 22-case
# e2e image does NOT install boto3/requests and has no MinIO, so skip
# the whole module cleanly when it's collected by the main suite.
boto3 = pytest.importorskip("boto3", reason="archive e2e stack only (needs boto3)")
requests = pytest.importorskip("requests", reason="archive e2e stack only (needs requests)")
if not os.environ.get("ENTDB_S3_ENDPOINT"):
    pytest.skip(
        "archive e2e stack only — set ENTDB_S3_ENDPOINT (see "
        "docker-compose.archive.yml / run-archive-e2e.sh)",
        allow_module_level=True,
    )

import e2e_schema_pb2 as pb  # noqa: E402
from botocore.config import Config  # noqa: E402

S3_ENDPOINT = os.environ.get("ENTDB_S3_ENDPOINT", "http://minio:9000")
S3_BUCKET = os.environ.get("ENTDB_S3_BUCKET", "entdb-wal-archive")
METRICS_URL = os.environ.get("ENTDB_METRICS_URL", "http://server:9090/metrics")
TENANT = os.environ.get("ENTDB_TENANT", "e2e-test")


@pytest.fixture(scope="module")
def s3():
    return boto3.client(
        "s3",
        endpoint_url=S3_ENDPOINT,
        aws_access_key_id=os.environ.get("AWS_ACCESS_KEY_ID", "entdb"),
        aws_secret_access_key=os.environ.get("AWS_SECRET_ACCESS_KEY", "entdb-secret"),
        region_name=os.environ.get("AWS_DEFAULT_REGION", "us-east-1"),
        config=Config(s3={"addressing_style": "path"}, signature_version="s3v4"),
    )


def _list_archive_objects(s3) -> list[dict]:
    out: list[dict] = []
    paginator = s3.get_paginator("list_objects_v2")
    for page in paginator.paginate(Bucket=S3_BUCKET, Prefix="wal/"):
        out.extend(page.get("Contents", []) or [])
    return out


def _scrape_metric_lines(name: str) -> list[str]:
    resp = requests.get(METRICS_URL, timeout=10)
    resp.raise_for_status()
    return [
        line
        for line in resp.text.splitlines()
        if line.startswith(name + "{") or line.startswith(name + " ")
    ]


async def test_wal_archives_to_s3_object_lock_compliance(s3, scope) -> None:
    """End-to-end: SDK write -> Kafka WAL -> S3 Object Lock archive."""
    # 1. Write a batch of nodes. Each commit is one WAL event; the
    #    archive batch size is small (8) so the sidecar flushes quickly.
    written_emails = set()
    for i in range(12):
        plan = scope.plan()
        email = f"archive-{i}-{time.time_ns()}@example.com"
        plan.create(pb.User(email=email, name=f"Archive User {i}"))
        result = await plan.commit(wait_applied=True)
        assert result.success, f"commit {i} failed: {result.error}"
        written_emails.add(email)

    # 2. Poll S3 until the archiver has flushed at least one object.
    #    Retention is short (1 day) in the test bucket; the archiver
    #    polls every ~1s and the batch interval is small.
    deadline = time.time() + 90
    objects: list[dict] = []
    while time.time() < deadline:
        objects = _list_archive_objects(s3)
        if objects:
            break
        time.sleep(2)
    assert objects, (
        "no archive objects under wal/ after 90s — archiver did not flush WAL events to S3"
    )

    # Object key format: wal/<topic>/<partition>/<start>-<end>.jsonl.gz
    key = objects[0]["Key"]
    assert key.startswith("wal/") and key.endswith(".jsonl.gz"), f"bad key {key!r}"

    # 3. Object Lock COMPLIANCE + retain-until must be set (ADR-015:
    #    even root cannot delete within the retention window).
    head = s3.head_object(Bucket=S3_BUCKET, Key=key)
    assert head.get("ObjectLockMode") == "COMPLIANCE", (
        f"ObjectLockMode={head.get('ObjectLockMode')!r}, want COMPLIANCE"
    )
    assert head.get("ObjectLockRetainUntilDate") is not None, (
        "ObjectLockRetainUntilDate missing — object is not retention-locked"
    )
    meta = head.get("Metadata", {})
    # S3 lowercases user metadata keys; the archiver writes entdb-* keys.
    assert meta.get("entdb-format", "").startswith("wal-jsonl"), (
        f"unexpected archive format metadata: {meta!r}"
    )
    assert int(meta.get("entdb-record-count", "0")) >= 1

    # 4. The archived body is gzip JSONL and decodes back to our events.
    body = s3.get_object(Bucket=S3_BUCKET, Key=key)["Body"].read()
    with gzip.GzipFile(fileobj=io.BytesIO(body)) as gz:
        lines = [ln for ln in gz.read().decode().splitlines() if ln.strip()]
    assert lines, f"archive object {key!r} decoded to zero JSONL lines"
    decoded = [json.loads(ln) for ln in lines]
    for rec in decoded:
        assert rec["topic"].startswith("entdb-wal-archive-e2e")
        assert "offset" in rec and "partition" in rec
    # The tenant id rides the WAL record key / event value.
    blob = json.dumps(decoded)
    assert TENANT in blob, "archived events do not reference the e2e tenant"

    # 5. Prometheus archive-lag gauge is exported and drains to 0 once
    #    the archiver has caught up. We just wrote 12 events that all
    #    archived above, so steady-state lag is 0.
    lag_lines: list[str] = []
    drained = False
    deadline = time.time() + 60
    while time.time() < deadline:
        lag_lines = _scrape_metric_lines("entdb_archive_lag_events")
        if lag_lines and all(float(ln.rsplit(" ", 1)[1]) == 0.0 for ln in lag_lines):
            drained = True
            break
        time.sleep(3)
    assert lag_lines, (
        "entdb_archive_lag_events not exported on /metrics — archive lag metric missing"
    )
    assert drained, f"archive lag did not drain to 0; last sample: {lag_lines!r}"

    # Sanity: the writes counter advances past our batch. Poll rather
    # than one-shot — the final archive batch may still be mid-flush
    # when the lag gauge first reads 0 for the other partitions.
    writes: list[str] = []
    deadline = time.time() + 30
    while time.time() < deadline:
        writes = _scrape_metric_lines("entdb_archive_writes_total")
        if writes and float(writes[0].rsplit(" ", 1)[1]) >= 12.0:
            break
        time.sleep(2)
    assert writes, "entdb_archive_writes_total not exported"
    assert float(writes[0].rsplit(" ", 1)[1]) >= 12.0, (
        f"entdb_archive_writes_total={writes[0]!r}, want >= 12"
    )


def _legal_hold_status(s3, key: str) -> str | None:
    """Return the object's S3 Object Lock legal-hold status, or None
    when no legal-hold configuration is present (never held)."""
    try:
        resp = s3.get_object_legal_hold(Bucket=S3_BUCKET, Key=key)
    except Exception:  # noqa: BLE001 — boto raises when no legal hold set
        return None
    return resp.get("LegalHold", {}).get("Status")


async def test_legal_hold_lift_on_release(s3, db_client) -> None:
    """EPIC #511 Gap 1: set hold ON -> archive -> release -> the durable
    background worker lifts the hold while COMPLIANCE retention is
    untouched. The release does NOT run the sweep on the RPC goroutine —
    it durably enqueues legal_hold_lift_queue and the crash-durable
    LiftWorker drains it; the entdb_legal_hold_lift_pending gauge drains
    to 0 once done.

    Uses a dedicated, registry-backed tenant (created via CreateTenant so
    it flows WAL -> applier -> globalstore, which SetLegalHold's tenant
    gate requires) and writes through admin:root, the tenant creator.
    """
    tenant = f"lh-lift-{int(time.time())}"
    created = await db_client.create_tenant(tenant, name="Legal-Hold Lift E2E", actor="admin:root")
    assert created.get("success"), f"create_tenant failed: {created!r}"
    lh_scope = db_client.tenant(tenant).actor("admin:root")

    # 1. Put the dedicated tenant under legal hold. The archiver consults
    #    globalstore.IsLegalHoldSet per batch, so writes AFTER this land
    #    in S3 with ObjectLockLegalHoldStatus=ON.
    res = await db_client.set_legal_hold(tenant, enabled=True, actor="admin:root")
    assert res.get("success"), f"set_legal_hold ON failed: {res!r}"

    # 2. Write a fresh batch so the archiver flushes held objects.
    for i in range(12):
        plan = lh_scope.plan()
        plan.create(
            pb.User(
                email=f"hold-{i}-{time.time_ns()}@example.com",
                name=f"Hold User {i}",
            )
        )
        result = await plan.commit(wait_applied=True)
        assert result.success, f"held commit {i} failed: {result.error}"

    # 3. Wait until an archived object carrying THIS tenant has legal
    #    hold ON. The key is per WAL partition, so filter by decoding the
    #    body and matching our tenant id.
    held_key: str | None = None
    retain_before: object = None
    deadline = time.time() + 90
    while time.time() < deadline and not held_key:
        for obj in _list_archive_objects(s3):
            k = obj["Key"]
            if _legal_hold_status(s3, k) != "ON":
                continue
            body = s3.get_object(Bucket=S3_BUCKET, Key=k)["Body"].read()
            with gzip.GzipFile(fileobj=io.BytesIO(body)) as gz:
                blob = gz.read().decode()
            if tenant not in blob:
                continue
            held_key = k
            head = s3.head_object(Bucket=S3_BUCKET, Key=k)
            retain_before = head.get("ObjectLockRetainUntilDate")
            assert head.get("ObjectLockMode") == "COMPLIANCE", (
                f"held object {k!r} not COMPLIANCE: {head.get('ObjectLockMode')!r}"
            )
            assert retain_before is not None, f"held object {k!r} has no retain-until"
            break
        if not held_key:
            time.sleep(2)
    assert held_key, (
        "no archived object for the held tenant reached LegalHold=ON within "
        "90s — archiver did not escalate writes for the held tenant"
    )

    # 4. Explicit release. This does NOT run the sweep on the RPC path:
    #    it durably enqueues a legal_hold_lift_queue row (same globalstore
    #    txn that clears the hold) and returns once the hold is cleared.
    #    The crash-durable LiftWorker drains the queue on its interval.
    res = await db_client.set_legal_hold(tenant, enabled=False, actor="admin:root")
    assert res.get("success"), f"set_legal_hold OFF failed: {res!r}"
    assert res.get("status") == "active", f"unexpected release status: {res!r}"

    # 4b. The durable pending-lift gauge must be exported (operator signal
    #     the old detached goroutine never provided). It may already be 0
    #     if the fast-interval worker swept before this scrape; we only
    #     require the metric to EXIST here and drain to 0 below.
    pending_lines = _scrape_metric_lines("entdb_legal_hold_lift_pending")
    assert pending_lines, (
        "entdb_legal_hold_lift_pending not exported on /metrics — the "
        "durable-lift observability signal is missing"
    )

    # 5. The background worker (NOT a request goroutine) must flip the
    #    previously-held object's legal hold OFF, and its COMPLIANCE
    #    retention must be byte-for-byte unchanged (ADR-015: legal hold is
    #    liftable on release; retention is immutable).
    deadline = time.time() + 90
    lifted = False
    while time.time() < deadline:
        status = _legal_hold_status(s3, held_key)
        if status in (None, "OFF"):
            lifted = True
            break
        time.sleep(2)
    assert lifted, (
        f"legal hold on {held_key!r} did not lift to OFF within 90s — the "
        "durable LiftWorker did not drain the queue"
    )

    head_after = s3.head_object(Bucket=S3_BUCKET, Key=held_key)
    assert head_after.get("ObjectLockMode") == "COMPLIANCE", (
        f"retention mode changed on lift: {head_after.get('ObjectLockMode')!r}"
    )
    assert head_after.get("ObjectLockRetainUntilDate") == retain_before, (
        "Object Lock retain-until changed during legal-hold lift — "
        f"before={retain_before!r} after={head_after.get('ObjectLockRetainUntilDate')!r}; "
        "COMPLIANCE retention MUST be immutable (ADR-015)"
    )

    # 6. Once the worker has finished, the pending gauge drains to 0 —
    #    proving the durable queue emptied (the "no signal" gap closed).
    deadline = time.time() + 60
    drained = False
    while time.time() < deadline:
        lines = _scrape_metric_lines("entdb_legal_hold_lift_pending")
        if lines and all(float(ln.rsplit(" ", 1)[1]) == 0.0 for ln in lines):
            drained = True
            break
        time.sleep(3)
    assert drained, (
        "entdb_legal_hold_lift_pending did not drain to 0 after the lift — "
        "the durable queue worker did not complete"
    )
