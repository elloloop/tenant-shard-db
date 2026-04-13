"""
Unit tests for applier halt-on-failure semantics and the asyncio
event-loop offload of synchronous SQLite work.

These tests cover two critical bug fixes in
``dbaas/entdb_server/apply/applier.py``:

1. WAL offsets must NEVER advance past a failed event. Production must
   halt and surface the failure rather than silently dropping data.
2. Long batch transactions must run inside the canonical_store
   ThreadPoolExecutor so the gRPC asyncio loop is never blocked.
"""

from __future__ import annotations

import json

import pytest

from dbaas.entdb_server.apply.applier import (
    Applier,
    ApplyResult,
    MailboxFanoutConfig,
    TransactionEvent,
)
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.wal.memory import InMemoryWalStream


def _event_bytes(
    tenant_id: str,
    idempotency_key: str,
    node_id: str | None = None,
) -> bytes:
    event = {
        "tenant_id": tenant_id,
        "actor": "user:1",
        "idempotency_key": idempotency_key,
        "ops": [
            {
                "op": "create_node",
                "type_id": 1,
                "id": node_id or f"n-{idempotency_key}",
                "data": {"k": idempotency_key},
            }
        ],
    }
    return json.dumps(event).encode()


# ────────────────────────────────────────────────────────────────────
# Bug 1 — run_in_executor offload
# ────────────────────────────────────────────────────────────────────


@pytest.mark.unit
class TestSqliteOffload:
    """Sync SQLite work in _apply_tenant_batch / apply_event must run
    in canonical_store._executor, not the asyncio event loop."""

    @pytest.mark.asyncio
    async def test_apply_tenant_batch_uses_executor(self, tmp_path, mocker):
        """A batch apply must call _executor.submit at least once."""
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        await wal.append("t", "k", _event_bytes("tenant-1", "ev-1", "node-1"))
        await wal.append("t", "k", _event_bytes("tenant-1", "ev-2", "node-2"))

        store = CanonicalStore(str(tmp_path))
        applier = Applier(
            wal=wal,
            canonical_store=store,
            topic="t",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=10,
        )

        # Spy on the executor's submit method
        submit_spy = mocker.spy(store._executor, "submit")

        records = await wal.poll_batch("t", "grp", max_records=10, timeout_ms=50)
        items = [(r, r.value_json()) for r in records]
        await applier._apply_tenant_batch("tenant-1", items)

        assert submit_spy.call_count >= 1, (
            "Expected canonical_store._executor.submit to be called when "
            "applying a batch — sync SQLite work must not run on the "
            "asyncio event loop"
        )

        store.close_all()

    @pytest.mark.asyncio
    async def test_apply_event_uses_executor(self, tmp_path, mocker):
        """A single-event apply must offload to the executor."""
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        store = CanonicalStore(str(tmp_path))
        applier = Applier(
            wal=wal,
            canonical_store=store,
            topic="t",
            fanout_config=MailboxFanoutConfig(enabled=False),
        )

        submit_spy = mocker.spy(store._executor, "submit")

        event = TransactionEvent.from_dict(
            json.loads(_event_bytes("tenant-1", "ev-solo", "node-solo").decode())
        )
        result = await applier.apply_event(event)
        assert result.success

        assert submit_spy.call_count >= 1, (
            "Expected canonical_store._executor.submit to be called when "
            "apply_event runs — sync SQLite work must not run on the loop"
        )

        store.close_all()


# ────────────────────────────────────────────────────────────────────
# Bug 2 — halt on failure / never advance past a failed event
# ────────────────────────────────────────────────────────────────────


def _install_failing_apply_event(applier: Applier, fail_keys: set[str]) -> None:
    """Patch ``apply_event`` to fail for events with idempotency_key in
    ``fail_keys`` while still advancing for the rest.
    """
    orig = applier.apply_event

    async def maybe_fail(event: TransactionEvent) -> ApplyResult:
        if event.idempotency_key in fail_keys:
            return ApplyResult(success=False, event=event, error="injected failure")
        return await orig(event)

    applier.apply_event = maybe_fail  # type: ignore[method-assign]


@pytest.mark.unit
class TestSingleRecordHaltOnError:
    @pytest.mark.asyncio
    async def test_single_failure_does_not_advance_offset(self, tmp_path):
        """In _start_single, a failed event must not be committed and
        the loop must halt."""
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        await wal.append("t", "k", _event_bytes("tenant-1", "ev-bad", "node-bad"))

        store = CanonicalStore(str(tmp_path))
        applier = Applier(
            wal=wal,
            canonical_store=store,
            topic="t",
            group_id="grp",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=1,
        )

        _install_failing_apply_event(applier, {"ev-bad"})

        await applier.start()
        assert applier._running is False

        # The failed event must NOT have been committed to the consumer
        # group — a fresh poll on the same group must see it again.
        replay = await wal.poll_batch("t", "grp", max_records=10, timeout_ms=50)
        assert len(replay) == 1, "Failed event must remain uncommitted so it is redelivered"

        store.close_all()

    @pytest.mark.asyncio
    async def test_single_success_still_commits(self, tmp_path):
        """Regression guard — apply_event still works on the happy
        path after the executor refactor.
        """
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        store = CanonicalStore(str(tmp_path))
        applier = Applier(
            wal=wal,
            canonical_store=store,
            topic="t",
            group_id="grp",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=1,
        )

        event = TransactionEvent.from_dict(
            json.loads(_event_bytes("tenant-1", "ev-ok", "node-ok").decode())
        )
        result = await applier.apply_event(event)
        assert result.success is True
        assert not result.skipped

        node = await store.get_node("tenant-1", "node-ok")
        assert node is not None

        store.close_all()


@pytest.mark.unit
class TestBatchHaltOnError:
    @pytest.mark.asyncio
    async def test_batch_partial_failure_no_offset_advances(self, tmp_path):
        """In a batch of 5 events where event 3 fails, none of the 5
        offsets advance."""
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(5):
            await wal.append("t", "k", _event_bytes("tenant-1", f"ev-{i}", f"node-{i}"))

        store = CanonicalStore(str(tmp_path))
        applier = Applier(
            wal=wal,
            canonical_store=store,
            topic="t",
            group_id="grp",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=10,
            poll_timeout_ms=50,
        )

        # Force the batch path to take the per-record fallback by
        # making _apply_tenant_batch always raise.
        async def always_raise(tenant_id, items):
            raise RuntimeError("inject batch failure")

        applier._apply_tenant_batch = always_raise  # type: ignore[method-assign]

        # Make the 3rd record (idempotency_key ev-2) fail in fallback.
        _install_failing_apply_event(applier, {"ev-2"})

        await applier.start()
        assert applier._running is False

        # No record in this single partition should have been committed
        # because at least one record failed.
        replay = await wal.poll_batch("t", "grp", max_records=10, timeout_ms=50)
        assert len(replay) == 5, (
            "All 5 records must be redelivered — partition with a failed "
            f"event must not be committed (got {len(replay)})"
        )

        store.close_all()

    @pytest.mark.asyncio
    async def test_batch_success_still_commits(self, tmp_path):
        """Regression guard — a clean batch advances the offset."""
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(3):
            await wal.append("t", "k", _event_bytes("tenant-1", f"ok-{i}", f"node-ok-{i}"))

        store = CanonicalStore(str(tmp_path))
        applier = Applier(
            wal=wal,
            canonical_store=store,
            topic="t",
            group_id="grp",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=10,
            poll_timeout_ms=50,
        )

        # Stop after the first batch
        orig = applier._apply_tenant_batch

        async def stop_after(tenant_id, items):
            await orig(tenant_id, items)
            applier._running = False

        applier._apply_tenant_batch = stop_after  # type: ignore[method-assign]

        await applier.start()

        replay = await wal.poll_batch("t", "grp", max_records=10, timeout_ms=50)
        assert len(replay) == 0, "Successful batch should advance offsets"

        store.close_all()

    @pytest.mark.asyncio
    async def test_halt_on_error_false_continues(self, tmp_path):
        """halt_on_error=False is a test-only escape hatch: the batch
        loop must NOT flip _running after a failure, but still must
        skip committing the failed record's partition.
        """
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()
        for i in range(2):
            await wal.append("t", "k", _event_bytes("tenant-1", f"ev-{i}", f"node-{i}"))

        store = CanonicalStore(str(tmp_path))
        applier = Applier(
            wal=wal,
            canonical_store=store,
            topic="t",
            group_id="grp",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=10,
            poll_timeout_ms=50,
            halt_on_error=False,
        )

        # Force the batch fallback path
        async def always_raise(tenant_id, items):
            raise RuntimeError("inject batch failure")

        applier._apply_tenant_batch = always_raise  # type: ignore[method-assign]
        _install_failing_apply_event(applier, {"ev-0"})

        # Run only one batch iteration by stopping after the first poll
        orig_poll = wal.poll_batch
        call_count = {"n": 0}

        async def poll_once_then_stop(*args, **kwargs):
            call_count["n"] += 1
            if call_count["n"] > 1:
                applier._running = False
                return []
            return await orig_poll(*args, **kwargs)

        wal.poll_batch = poll_once_then_stop  # type: ignore[method-assign]

        await applier.start()

        # Restore the real poll_batch for the assertion replay below
        wal.poll_batch = orig_poll  # type: ignore[method-assign]

        # halt_on_error=False means the loop did not raise/return; it
        # was stopped by our test harness, not by the failure. Critical
        # property: the failed-record partition was NOT committed.
        assert applier._running is False  # we stopped it externally
        replay = await wal.poll_batch("t", "grp", max_records=10, timeout_ms=50)
        assert len(replay) == 2, (
            f"halt_on_error=False still must not commit failed records (got {len(replay)})"
        )

        store.close_all()
