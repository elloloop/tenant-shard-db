"""
Unit tests for applier fanout migration.

Verifies that _fanout_node writes notifications to the canonical store's
per-tenant SQLite (notifications table) instead of the legacy mailbox_store.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any
from unittest.mock import AsyncMock, MagicMock

import pytest

from dbaas.entdb_server.apply.applier import (
    Applier,
    MailboxFanoutConfig,
)
from dbaas.entdb_server.apply.canonical_store import CanonicalStore, Node
from dbaas.entdb_server.apply.mailbox_store import MailboxStore
from dbaas.entdb_server.wal.memory import InMemoryWalStream


@dataclass
class FakeEvent:
    """Minimal event for _fanout_node tests."""

    tenant_id: str = "t1"
    ts_ms: int = 1000
    idempotency_key: str = "ik-1"
    ops: list = field(default_factory=list)


def _make_node(
    node_id: str = "node-1",
    type_id: int = 1,
    payload: dict | None = None,
    acl: list | None = None,
) -> Node:
    """Build a Node with sensible defaults."""
    return Node(
        tenant_id="t1",
        node_id=node_id,
        type_id=type_id,
        payload=payload or {"title": "Hello"},
        acl=acl or [],
    )


async def _make_applier(tmp_path, tenant_id: str = "t1") -> Applier:
    """Build an Applier wired to real canonical store and real mailbox store.

    Initializes the tenant database so tables exist before tests run.
    """
    wal = InMemoryWalStream(num_partitions=1)
    store = CanonicalStore(str(tmp_path / "canonical"))
    mbox = MailboxStore(str(tmp_path / "mailbox"))
    await store.initialize_tenant(tenant_id)
    applier = Applier(
        wal=wal,
        canonical_store=store,
        mailbox_store=mbox,
        topic="t",
        fanout_config=MailboxFanoutConfig(enabled=True),
    )
    return applier


@pytest.mark.unit
class TestFanoutWritesToCanonicalStore:
    """_fanout_node must write to canonical_store.batch_create_notifications."""

    @pytest.mark.asyncio
    async def test_fanout_creates_notifications_in_canonical_store(self, tmp_path):
        """Fanout with a single recipient creates one notification row."""
        applier = await _make_applier(tmp_path)
        event = FakeEvent(tenant_id="t1")
        node = _make_node(acl=[{"principal": "user:alice"}])
        op: dict[str, Any] = {"op": "create_node"}

        await applier._fanout_node(event, node, op)

        # Query the canonical store's notifications table
        notifications = await applier.canonical_store.get_notifications("t1", "user:alice")
        assert len(notifications) == 1
        assert notifications[0]["node_id"] == "node-1"
        assert notifications[0]["user_id"] == "user:alice"

        applier.canonical_store.close_all()

    @pytest.mark.asyncio
    async def test_fanout_does_not_call_mailbox_store(self, tmp_path):
        """After migration, mailbox_store.add_item must NOT be called."""
        applier = await _make_applier(tmp_path)
        applier.mailbox_store = MagicMock(spec=MailboxStore)
        applier.mailbox_store.add_item = AsyncMock()

        event = FakeEvent(tenant_id="t1")
        node = _make_node(acl=[{"principal": "user:bob"}])
        op: dict[str, Any] = {"op": "create_node"}

        await applier._fanout_node(event, node, op)

        applier.mailbox_store.add_item.assert_not_called()
        applier.canonical_store.close_all()

    @pytest.mark.asyncio
    async def test_fanout_batch_of_10_inserts_all(self, tmp_path):
        """A node with 10 ACL recipients produces 10 notification rows."""
        applier = await _make_applier(tmp_path)
        acl = [{"principal": f"user:u{i}"} for i in range(10)]
        event = FakeEvent(tenant_id="t1")
        node = _make_node(acl=acl)
        op: dict[str, Any] = {"op": "create_node"}

        await applier._fanout_node(event, node, op)

        # Check each user got exactly one notification
        total = 0
        for i in range(10):
            notifs = await applier.canonical_store.get_notifications("t1", f"user:u{i}")
            assert len(notifs) == 1, f"user:u{i} should have 1 notification"
            total += len(notifs)
        assert total == 10

        applier.canonical_store.close_all()

    @pytest.mark.asyncio
    async def test_fanout_deduplicates_recipients(self, tmp_path):
        """If the same user appears in both fanout_to and ACL, only one notification."""
        applier = await _make_applier(tmp_path)
        event = FakeEvent(tenant_id="t1")
        node = _make_node(acl=[{"principal": "user:dup"}])
        op: dict[str, Any] = {"op": "create_node", "fanout_to": ["user:dup"]}

        await applier._fanout_node(event, node, op)

        notifs = await applier.canonical_store.get_notifications("t1", "user:dup")
        assert len(notifs) == 1

        applier.canonical_store.close_all()

    @pytest.mark.asyncio
    async def test_fanout_skips_non_user_principals(self, tmp_path):
        """Principals that don't start with 'user:' are skipped."""
        applier = await _make_applier(tmp_path)
        event = FakeEvent(tenant_id="t1")
        node = _make_node(
            acl=[
                {"principal": "role:admin"},
                {"principal": "group:eng"},
                {"principal": "user:valid"},
            ]
        )
        op: dict[str, Any] = {"op": "create_node"}

        await applier._fanout_node(event, node, op)

        # Only user:valid should have a notification
        valid_notifs = await applier.canonical_store.get_notifications("t1", "user:valid")
        assert len(valid_notifs) == 1

        applier.canonical_store.close_all()

    @pytest.mark.asyncio
    async def test_fanout_notification_contains_snippet(self, tmp_path):
        """Notification snippet is derived from the node payload."""
        applier = await _make_applier(tmp_path)
        event = FakeEvent(tenant_id="t1")
        node = _make_node(
            payload={"title": "Meeting Notes", "body": "Discuss Q4 goals"},
            acl=[{"principal": "user:charlie"}],
        )
        op: dict[str, Any] = {"op": "create_node"}

        await applier._fanout_node(event, node, op)

        notifs = await applier.canonical_store.get_notifications("t1", "user:charlie")
        assert len(notifs) == 1
        snippet = notifs[0].get("snippet", "")
        assert "Meeting Notes" in snippet
        assert "Discuss Q4 goals" in snippet

        applier.canonical_store.close_all()

    @pytest.mark.asyncio
    async def test_fanout_no_recipients_is_noop(self, tmp_path):
        """If there are no user: recipients, nothing is written."""
        applier = await _make_applier(tmp_path)
        # Mock batch_create_notifications to verify it's not called
        applier.canonical_store.batch_create_notifications = AsyncMock()

        event = FakeEvent(tenant_id="t1")
        node = _make_node(acl=[{"principal": "role:admin"}])
        op: dict[str, Any] = {"op": "create_node"}

        await applier._fanout_node(event, node, op)

        applier.canonical_store.batch_create_notifications.assert_not_called()

    @pytest.mark.asyncio
    async def test_fanout_error_is_logged_not_raised(self, tmp_path):
        """If batch_create_notifications fails, exception is caught and logged."""
        applier = await _make_applier(tmp_path)
        applier.canonical_store.batch_create_notifications = AsyncMock(
            side_effect=RuntimeError("db locked")
        )

        event = FakeEvent(tenant_id="t1")
        node = _make_node(acl=[{"principal": "user:fail"}])
        op: dict[str, Any] = {"op": "create_node"}

        # Should not raise
        await applier._fanout_node(event, node, op)

    @pytest.mark.asyncio
    async def test_fanout_from_fanout_to_field(self, tmp_path):
        """Recipients listed in op['fanout_to'] get notifications."""
        applier = await _make_applier(tmp_path)
        event = FakeEvent(tenant_id="t1")
        node = _make_node(acl=[])
        op: dict[str, Any] = {
            "op": "create_node",
            "fanout_to": ["user:x", "user:y"],
        }

        await applier._fanout_node(event, node, op)

        for uid in ["user:x", "user:y"]:
            notifs = await applier.canonical_store.get_notifications("t1", uid)
            assert len(notifs) == 1

        applier.canonical_store.close_all()
