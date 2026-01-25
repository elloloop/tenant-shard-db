"""
End-to-end tests for full EntDB flow.

Tests cover:
- Complete write/read cycle
- WAL to SQLite materialization
- Crash recovery
- Multi-tenant isolation
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


class TestFullFlow:
    """End-to-end tests for complete flow."""

    @pytest.mark.asyncio
    async def test_create_and_read_node(
        self,
        http_base_url: str,
        test_tenant_id: str,
        test_actor: str,
    ):
        """Create a node and read it back."""
        async with httpx.AsyncClient(base_url=http_base_url) as client:
            # Create node
            create_response = await client.post(
                "/v1/atomic",
                json={
                    "tenant_id": test_tenant_id,
                    "actor": test_actor,
                    "idempotency_key": "e2e_create_1",
                    "operations": [
                        {
                            "op": "create_node",
                            "alias": "user1",
                            "type_id": 1,
                            "payload": {
                                "email": "e2e@example.com",
                                "name": "E2E Test User",
                            },
                        }
                    ],
                },
            )
            assert create_response.status_code == 200
            result = create_response.json()
            node_id = result["results"][0]["node_id"]

            # Wait for applier to process
            await asyncio.sleep(1)

            # Read node back
            get_response = await client.get(
                f"/v1/nodes/{node_id}",
                params={"tenant_id": test_tenant_id, "actor": test_actor},
            )
            assert get_response.status_code == 200
            node = get_response.json()

            assert node["id"] == node_id
            assert node["payload"]["email"] == "e2e@example.com"
            assert node["payload"]["name"] == "E2E Test User"

    @pytest.mark.asyncio
    async def test_idempotent_create(
        self,
        http_base_url: str,
        test_tenant_id: str,
        test_actor: str,
    ):
        """Same idempotency key returns same result."""
        async with httpx.AsyncClient(base_url=http_base_url) as client:
            request_body = {
                "tenant_id": test_tenant_id,
                "actor": test_actor,
                "idempotency_key": "e2e_idempotent_test",
                "operations": [
                    {
                        "op": "create_node",
                        "alias": "user1",
                        "type_id": 1,
                        "payload": {"email": "idempotent@example.com"},
                    }
                ],
            }

            # First call
            response1 = await client.post("/v1/atomic", json=request_body)
            assert response1.status_code == 200
            node_id1 = response1.json()["results"][0]["node_id"]

            # Second call with same idempotency key
            response2 = await client.post("/v1/atomic", json=request_body)
            assert response2.status_code == 200
            node_id2 = response2.json()["results"][0]["node_id"]

            # Should return same node ID
            assert node_id1 == node_id2

    @pytest.mark.asyncio
    async def test_create_node_with_edge(
        self,
        http_base_url: str,
        test_tenant_id: str,
        test_actor: str,
    ):
        """Create nodes and edge in single transaction."""
        async with httpx.AsyncClient(base_url=http_base_url) as client:
            response = await client.post(
                "/v1/atomic",
                json={
                    "tenant_id": test_tenant_id,
                    "actor": test_actor,
                    "idempotency_key": "e2e_edge_test",
                    "operations": [
                        {
                            "op": "create_node",
                            "alias": "user1",
                            "type_id": 1,
                            "payload": {"email": "user@example.com", "name": "User"},
                        },
                        {
                            "op": "create_node",
                            "alias": "task1",
                            "type_id": 2,
                            "payload": {"title": "Test Task", "description": "Do something"},
                        },
                        {
                            "op": "create_edge",
                            "edge_type_id": 100,
                            "from_id": "$task1.id",
                            "to_id": "$user1.id",
                        },
                    ],
                },
            )
            assert response.status_code == 200
            results = response.json()["results"]

            user_id = results[0]["node_id"]
            task_id = results[1]["node_id"]
            _ = results[2]["edge_id"]  # edge_id captured but not used in assertions

            await asyncio.sleep(1)

            # Query edges from task
            edges_response = await client.get(
                f"/v1/nodes/{task_id}/edges/out",
                params={"tenant_id": test_tenant_id, "actor": test_actor},
            )
            assert edges_response.status_code == 200
            edges = edges_response.json()

            assert len(edges) == 1
            assert edges[0]["to_id"] == user_id

    @pytest.mark.asyncio
    async def test_query_nodes_by_type(
        self,
        http_base_url: str,
        test_tenant_id: str,
        test_actor: str,
    ):
        """Query nodes by type."""
        async with httpx.AsyncClient(base_url=http_base_url) as client:
            # Create multiple nodes
            for i in range(3):
                await client.post(
                    "/v1/atomic",
                    json={
                        "tenant_id": test_tenant_id,
                        "actor": test_actor,
                        "idempotency_key": f"e2e_query_test_{i}",
                        "operations": [
                            {
                                "op": "create_node",
                                "alias": "node",
                                "type_id": 1,
                                "payload": {"email": f"user{i}@example.com"},
                            }
                        ],
                    },
                )

            await asyncio.sleep(1)

            # Query all type 1 nodes
            response = await client.get(
                "/v1/nodes",
                params={
                    "tenant_id": test_tenant_id,
                    "actor": test_actor,
                    "type_id": 1,
                },
            )
            assert response.status_code == 200
            nodes = response.json()

            assert len(nodes) >= 3

    @pytest.mark.asyncio
    async def test_update_node(
        self,
        http_base_url: str,
        test_tenant_id: str,
        test_actor: str,
    ):
        """Update node payload."""
        async with httpx.AsyncClient(base_url=http_base_url) as client:
            # Create
            create_response = await client.post(
                "/v1/atomic",
                json={
                    "tenant_id": test_tenant_id,
                    "actor": test_actor,
                    "idempotency_key": "e2e_update_test_create",
                    "operations": [
                        {
                            "op": "create_node",
                            "alias": "user1",
                            "type_id": 1,
                            "payload": {"email": "original@example.com", "name": "Original"},
                        }
                    ],
                },
            )
            node_id = create_response.json()["results"][0]["node_id"]

            await asyncio.sleep(1)

            # Update
            await client.post(
                "/v1/atomic",
                json={
                    "tenant_id": test_tenant_id,
                    "actor": test_actor,
                    "idempotency_key": "e2e_update_test_update",
                    "operations": [
                        {
                            "op": "update_node",
                            "node_id": node_id,
                            "payload": {"email": "updated@example.com", "name": "Updated"},
                        }
                    ],
                },
            )

            await asyncio.sleep(1)

            # Verify
            get_response = await client.get(
                f"/v1/nodes/{node_id}",
                params={"tenant_id": test_tenant_id, "actor": test_actor},
            )
            node = get_response.json()

            assert node["payload"]["email"] == "updated@example.com"
            assert node["payload"]["name"] == "Updated"

    @pytest.mark.asyncio
    async def test_delete_node(
        self,
        http_base_url: str,
        test_tenant_id: str,
        test_actor: str,
    ):
        """Delete node (soft delete)."""
        async with httpx.AsyncClient(base_url=http_base_url) as client:
            # Create
            create_response = await client.post(
                "/v1/atomic",
                json={
                    "tenant_id": test_tenant_id,
                    "actor": test_actor,
                    "idempotency_key": "e2e_delete_test_create",
                    "operations": [
                        {
                            "op": "create_node",
                            "alias": "user1",
                            "type_id": 1,
                            "payload": {"email": "todelete@example.com"},
                        }
                    ],
                },
            )
            node_id = create_response.json()["results"][0]["node_id"]

            await asyncio.sleep(1)

            # Delete
            await client.post(
                "/v1/atomic",
                json={
                    "tenant_id": test_tenant_id,
                    "actor": test_actor,
                    "idempotency_key": "e2e_delete_test_delete",
                    "operations": [
                        {
                            "op": "delete_node",
                            "node_id": node_id,
                        }
                    ],
                },
            )

            await asyncio.sleep(1)

            # Verify deleted
            get_response = await client.get(
                f"/v1/nodes/{node_id}",
                params={"tenant_id": test_tenant_id, "actor": test_actor},
            )
            # Should return 404 or empty
            assert get_response.status_code in [404, 200]
            if get_response.status_code == 200:
                assert get_response.json().get("deleted") is True

    @pytest.mark.asyncio
    async def test_multi_tenant_isolation(
        self,
        http_base_url: str,
        test_actor: str,
    ):
        """Different tenants are isolated."""
        import uuid

        tenant1 = f"tenant_iso_1_{uuid.uuid4().hex[:8]}"
        tenant2 = f"tenant_iso_2_{uuid.uuid4().hex[:8]}"

        async with httpx.AsyncClient(base_url=http_base_url) as client:
            # Create in tenant 1
            await client.post(
                "/v1/atomic",
                json={
                    "tenant_id": tenant1,
                    "actor": test_actor,
                    "idempotency_key": "iso_test_1",
                    "operations": [
                        {
                            "op": "create_node",
                            "alias": "user1",
                            "type_id": 1,
                            "payload": {"email": "tenant1@example.com"},
                        }
                    ],
                },
            )

            # Create in tenant 2
            await client.post(
                "/v1/atomic",
                json={
                    "tenant_id": tenant2,
                    "actor": test_actor,
                    "idempotency_key": "iso_test_2",
                    "operations": [
                        {
                            "op": "create_node",
                            "alias": "user1",
                            "type_id": 1,
                            "payload": {"email": "tenant2@example.com"},
                        }
                    ],
                },
            )

            await asyncio.sleep(1)

            # Query tenant 1
            t1_response = await client.get(
                "/v1/nodes",
                params={"tenant_id": tenant1, "actor": test_actor, "type_id": 1},
            )
            t1_nodes = t1_response.json()

            # Query tenant 2
            t2_response = await client.get(
                "/v1/nodes",
                params={"tenant_id": tenant2, "actor": test_actor, "type_id": 1},
            )
            t2_nodes = t2_response.json()

            # Each tenant should only see their own data
            t1_emails = {n["payload"]["email"] for n in t1_nodes}
            t2_emails = {n["payload"]["email"] for n in t2_nodes}

            assert "tenant1@example.com" in t1_emails
            assert "tenant2@example.com" not in t1_emails
            assert "tenant2@example.com" in t2_emails
            assert "tenant1@example.com" not in t2_emails

    @pytest.mark.asyncio
    async def test_health_check(self, http_base_url: str):
        """Health endpoint returns OK."""
        async with httpx.AsyncClient(base_url=http_base_url) as client:
            response = await client.get("/health")
            assert response.status_code == 200
            health = response.json()
            assert health["status"] == "healthy"

    @pytest.mark.asyncio
    async def test_schema_endpoint(self, http_base_url: str):
        """Schema endpoint returns types."""
        async with httpx.AsyncClient(base_url=http_base_url) as client:
            response = await client.get("/v1/schema")
            assert response.status_code == 200
            schema = response.json()
            assert "node_types" in schema
            assert "edge_types" in schema
            assert "fingerprint" in schema


class TestCrashRecovery:
    """Tests for crash recovery scenarios."""

    @pytest.mark.asyncio
    async def test_recovery_after_restart(
        self,
        http_base_url: str,
        test_tenant_id: str,
        test_actor: str,
        docker_compose,
    ):
        """Data survives server restart."""
        import subprocess

        async with httpx.AsyncClient(base_url=http_base_url) as client:
            # Create node
            create_response = await client.post(
                "/v1/atomic",
                json={
                    "tenant_id": test_tenant_id,
                    "actor": test_actor,
                    "idempotency_key": "recovery_test",
                    "operations": [
                        {
                            "op": "create_node",
                            "alias": "user1",
                            "type_id": 1,
                            "payload": {"email": "recovery@example.com"},
                        }
                    ],
                },
            )
            node_id = create_response.json()["results"][0]["node_id"]

            await asyncio.sleep(2)

        # Restart server (not WAL)
        compose_file = os.path.join(os.path.dirname(__file__), "..", "..", "docker-compose.yml")
        subprocess.run(
            ["docker-compose", "-f", compose_file, "restart", "dbaas"],
            check=True,
            capture_output=True,
        )

        # Wait for restart
        await asyncio.sleep(10)

        # Verify data still exists
        async with httpx.AsyncClient(base_url=http_base_url) as client:
            get_response = await client.get(
                f"/v1/nodes/{node_id}",
                params={"tenant_id": test_tenant_id, "actor": test_actor},
            )
            assert get_response.status_code == 200
            node = get_response.json()
            assert node["payload"]["email"] == "recovery@example.com"
