"""
End-to-end tests for mailbox functionality.

Tests cover:
- Mailbox item creation
- Full-text search
- Read/unread status
"""

import asyncio
import os

import httpx
import pytest

# Skip if not in E2E mode
E2E_ENABLED = os.environ.get("ENTDB_E2E_TESTS", "0") == "1"
pytestmark = pytest.mark.skipif(
    not E2E_ENABLED, reason="E2E tests disabled. Set ENTDB_E2E_TESTS=1 to enable."
)


class TestMailboxE2E:
    """End-to-end tests for mailbox."""

    @pytest.mark.asyncio
    async def test_message_appears_in_mailbox(
        self,
        http_base_url: str,
        test_tenant_id: str,
        test_actor: str,
    ):
        """Sending message adds it to recipient mailbox."""
        async with httpx.AsyncClient(base_url=http_base_url) as client:
            # Create message with recipient
            await client.post(
                "/v1/atomic",
                json={
                    "tenant_id": test_tenant_id,
                    "actor": test_actor,
                    "idempotency_key": "mailbox_test_1",
                    "operations": [
                        {
                            "op": "create_node",
                            "alias": "msg1",
                            "type_id": 3,  # Message type
                            "payload": {
                                "subject": "E2E Test Message",
                                "body": "This is a test message for Bob",
                            },
                            "recipients": ["user:bob"],
                        }
                    ],
                },
            )

            await asyncio.sleep(2)

            # Check Bob's mailbox
            response = await client.get(
                "/v1/mailbox",
                params={
                    "tenant_id": test_tenant_id,
                    "user_id": "bob",
                },
            )
            assert response.status_code == 200
            items = response.json()

            assert len(items) >= 1
            subjects = [item["preview_text"] for item in items]
            assert "E2E Test Message" in subjects

    @pytest.mark.asyncio
    async def test_mailbox_search(
        self,
        http_base_url: str,
        test_tenant_id: str,
        test_actor: str,
    ):
        """Full-text search in mailbox."""
        async with httpx.AsyncClient(base_url=http_base_url) as client:
            # Create messages with different content
            await client.post(
                "/v1/atomic",
                json={
                    "tenant_id": test_tenant_id,
                    "actor": test_actor,
                    "idempotency_key": "mailbox_search_1",
                    "operations": [
                        {
                            "op": "create_node",
                            "alias": "msg1",
                            "type_id": 3,
                            "payload": {
                                "subject": "Meeting tomorrow",
                                "body": "Let's discuss the project",
                            },
                            "recipients": ["user:searcher"],
                        }
                    ],
                },
            )

            await client.post(
                "/v1/atomic",
                json={
                    "tenant_id": test_tenant_id,
                    "actor": test_actor,
                    "idempotency_key": "mailbox_search_2",
                    "operations": [
                        {
                            "op": "create_node",
                            "alias": "msg2",
                            "type_id": 3,
                            "payload": {
                                "subject": "Lunch plans",
                                "body": "Where should we eat?",
                            },
                            "recipients": ["user:searcher"],
                        }
                    ],
                },
            )

            await asyncio.sleep(2)

            # Search for "meeting"
            response = await client.get(
                "/v1/mailbox/search",
                params={
                    "tenant_id": test_tenant_id,
                    "user_id": "searcher",
                    "query": "meeting",
                },
            )
            assert response.status_code == 200
            results = response.json()

            # Should find only the meeting message
            assert len(results) >= 1
            assert any("Meeting" in r.get("preview_text", "") for r in results)

    @pytest.mark.asyncio
    async def test_mark_read_unread(
        self,
        http_base_url: str,
        test_tenant_id: str,
        test_actor: str,
    ):
        """Mark mailbox items as read/unread."""
        async with httpx.AsyncClient(base_url=http_base_url) as client:
            # Create message
            create_response = await client.post(
                "/v1/atomic",
                json={
                    "tenant_id": test_tenant_id,
                    "actor": test_actor,
                    "idempotency_key": "mailbox_read_test",
                    "operations": [
                        {
                            "op": "create_node",
                            "alias": "msg1",
                            "type_id": 3,
                            "payload": {
                                "subject": "Read status test",
                                "body": "Testing read/unread",
                            },
                            "recipients": ["user:reader"],
                        }
                    ],
                },
            )
            node_id = create_response.json()["results"][0]["node_id"]

            await asyncio.sleep(2)

            # Check initial unread count
            response = await client.get(
                "/v1/mailbox/unread_count",
                params={
                    "tenant_id": test_tenant_id,
                    "user_id": "reader",
                },
            )
            initial_count = response.json()["count"]
            assert initial_count >= 1

            # Mark as read
            await client.post(
                "/v1/mailbox/mark_read",
                json={
                    "tenant_id": test_tenant_id,
                    "user_id": "reader",
                    "node_id": node_id,
                },
            )

            # Check unread count decreased
            response = await client.get(
                "/v1/mailbox/unread_count",
                params={
                    "tenant_id": test_tenant_id,
                    "user_id": "reader",
                },
            )
            new_count = response.json()["count"]
            assert new_count < initial_count

    @pytest.mark.asyncio
    async def test_mailbox_pagination(
        self,
        http_base_url: str,
        test_tenant_id: str,
        test_actor: str,
    ):
        """Mailbox supports pagination."""
        async with httpx.AsyncClient(base_url=http_base_url) as client:
            # Create multiple messages
            for i in range(5):
                await client.post(
                    "/v1/atomic",
                    json={
                        "tenant_id": test_tenant_id,
                        "actor": test_actor,
                        "idempotency_key": f"mailbox_page_{i}",
                        "operations": [
                            {
                                "op": "create_node",
                                "alias": "msg",
                                "type_id": 3,
                                "payload": {
                                    "subject": f"Page test message {i}",
                                    "body": f"Message body {i}",
                                },
                                "recipients": ["user:paginator"],
                            }
                        ],
                    },
                )

            await asyncio.sleep(2)

            # Get first page
            response1 = await client.get(
                "/v1/mailbox",
                params={
                    "tenant_id": test_tenant_id,
                    "user_id": "paginator",
                    "limit": 2,
                },
            )
            page1 = response1.json()
            assert len(page1) == 2

            # Get second page
            response2 = await client.get(
                "/v1/mailbox",
                params={
                    "tenant_id": test_tenant_id,
                    "user_id": "paginator",
                    "limit": 2,
                    "offset": 2,
                },
            )
            page2 = response2.json()
            assert len(page2) == 2

            # Pages should have different items
            ids1 = {item["node_id"] for item in page1}
            ids2 = {item["node_id"] for item in page2}
            assert ids1.isdisjoint(ids2)
