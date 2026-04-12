"""Tests for user registry CRUD: gRPC handlers and SDK client methods.

Covers:
- CreateUser: admin/system can create, regular users cannot
- GetUser: any authenticated actor can read, returns None for missing
- UpdateUser: self or admin can update, regular user cannot update others
- ListUsers: pagination, status filter
- Error handling: duplicate user, missing fields, no global_store
- SDK client wrapper methods
"""

from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from dbaas.entdb_server.api.generated import (
    CreateUserRequest,
    GetUserRequest,
    ListUsersRequest,
    UpdateUserRequest,
)
from dbaas.entdb_server.api.grpc_server import EntDBServicer

# ── Helpers ──────────────────────────────────────────────────────────


def _make_global_store():
    """Build a mock GlobalStore with user registry methods."""
    store = AsyncMock()
    store.create_user = AsyncMock()
    store.get_user = AsyncMock()
    store.update_user = AsyncMock()
    store.list_users = AsyncMock()
    return store


def _build_servicer(global_store=None):
    """Build an EntDBServicer with a mock global store."""
    registry = MagicMock()
    registry.fingerprint = "sha256:test"
    return EntDBServicer(
        wal=AsyncMock(),
        canonical_store=AsyncMock(),
        mailbox_store=AsyncMock(),
        schema_registry=registry,
        global_store=global_store,
    )


def _mock_context():
    """Build a mock gRPC context.

    In real gRPC, context.abort raises an exception that terminates the handler.
    Our mock simulates this by raising Exception so the handler stops processing.
    Tests that expect abort should catch the exception or check context.abort.called.
    """
    ctx = AsyncMock()
    ctx.abort = AsyncMock(side_effect=Exception("aborted"))
    return ctx


# ── CreateUser Tests ─────────────────────────────────────────────────


class TestCreateUser:
    """CreateUser RPC handler tests."""

    @pytest.mark.asyncio
    async def test_create_user_as_system_admin(self):
        store = _make_global_store()
        store.create_user.return_value = {
            "user_id": "u1",
            "email": "alice@example.com",
            "name": "Alice",
            "status": "active",
            "created_at": 1000,
            "updated_at": 1000,
        }
        servicer = _build_servicer(global_store=store)
        context = _mock_context()

        request = CreateUserRequest(
            actor="system:admin",
            user_id="u1",
            email="alice@example.com",
            name="Alice",
        )

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            response = await servicer.CreateUser(request, context)

        assert response.success is True
        assert response.user.user_id == "u1"
        assert response.user.email == "alice@example.com"
        assert response.user.name == "Alice"
        assert response.user.status == "active"
        store.create_user.assert_called_once_with("u1", "alice@example.com", "Alice")

    @pytest.mark.asyncio
    async def test_create_user_as_admin_actor(self):
        store = _make_global_store()
        store.create_user.return_value = {
            "user_id": "u2",
            "email": "bob@example.com",
            "name": "Bob",
            "status": "active",
            "created_at": 2000,
            "updated_at": 2000,
        }
        servicer = _build_servicer(global_store=store)
        context = _mock_context()

        request = CreateUserRequest(
            actor="admin:root",
            user_id="u2",
            email="bob@example.com",
            name="Bob",
        )

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            response = await servicer.CreateUser(request, context)

        assert response.success is True
        assert response.user.user_id == "u2"

    @pytest.mark.asyncio
    async def test_create_user_denied_for_regular_user(self):
        store = _make_global_store()
        servicer = _build_servicer(global_store=store)
        context = _mock_context()

        request = CreateUserRequest(
            actor="user:42",
            user_id="u3",
            email="carol@example.com",
            name="Carol",
        )

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            await servicer.CreateUser(request, context)

        # context.abort was called with PERMISSION_DENIED
        context.abort.assert_called()
        assert "admin or system" in str(context.abort.call_args)
        store.create_user.assert_not_called()

    @pytest.mark.asyncio
    async def test_create_user_duplicate_email(self):
        store = _make_global_store()
        import sqlite3

        store.create_user.side_effect = sqlite3.IntegrityError(
            "UNIQUE constraint failed: user_registry.email"
        )
        servicer = _build_servicer(global_store=store)
        context = _mock_context()

        request = CreateUserRequest(
            actor="system:admin",
            user_id="u4",
            email="dup@example.com",
            name="Dup",
        )

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            response = await servicer.CreateUser(request, context)

        assert response.success is False
        assert "already exists" in response.error

    @pytest.mark.asyncio
    async def test_create_user_missing_email(self):
        store = _make_global_store()
        servicer = _build_servicer(global_store=store)
        context = _mock_context()

        request = CreateUserRequest(
            actor="system:admin",
            user_id="u5",
            name="NoEmail",
        )

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            await servicer.CreateUser(request, context)

        context.abort.assert_called()
        assert "email" in str(context.abort.call_args).lower()


# ── GetUser Tests ────────────────────────────────────────────────────


class TestGetUser:
    """GetUser RPC handler tests."""

    @pytest.mark.asyncio
    async def test_get_existing_user(self):
        store = _make_global_store()
        store.get_user.return_value = {
            "user_id": "u1",
            "email": "alice@example.com",
            "name": "Alice",
            "status": "active",
            "created_at": 1000,
            "updated_at": 1000,
        }
        servicer = _build_servicer(global_store=store)
        context = _mock_context()

        request = GetUserRequest(actor="user:u1", user_id="u1")

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            response = await servicer.GetUser(request, context)

        assert response.found is True
        assert response.user.user_id == "u1"
        assert response.user.email == "alice@example.com"

    @pytest.mark.asyncio
    async def test_get_missing_user(self):
        store = _make_global_store()
        store.get_user.return_value = None
        servicer = _build_servicer(global_store=store)
        context = _mock_context()

        request = GetUserRequest(actor="user:u99", user_id="u99")

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            response = await servicer.GetUser(request, context)

        assert response.found is False

    @pytest.mark.asyncio
    async def test_get_user_requires_actor(self):
        store = _make_global_store()
        servicer = _build_servicer(global_store=store)
        context = _mock_context()

        request = GetUserRequest(user_id="u1")

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            await servicer.GetUser(request, context)

        context.abort.assert_called()
        assert "actor" in str(context.abort.call_args).lower()


# ── UpdateUser Tests ─────────────────────────────────────────────────


class TestUpdateUser:
    """UpdateUser RPC handler tests."""

    @pytest.mark.asyncio
    async def test_update_user_by_self(self):
        store = _make_global_store()
        store.update_user.return_value = True
        servicer = _build_servicer(global_store=store)
        context = _mock_context()

        request = UpdateUserRequest(
            actor="user:u1",
            user_id="u1",
            name="Alice Updated",
        )

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            response = await servicer.UpdateUser(request, context)

        assert response.success is True
        store.update_user.assert_called_once_with("u1", name="Alice Updated")

    @pytest.mark.asyncio
    async def test_update_user_by_admin(self):
        store = _make_global_store()
        store.update_user.return_value = True
        servicer = _build_servicer(global_store=store)
        context = _mock_context()

        request = UpdateUserRequest(
            actor="admin:root",
            user_id="u1",
            status="suspended",
        )

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            response = await servicer.UpdateUser(request, context)

        assert response.success is True
        store.update_user.assert_called_once_with("u1", status="suspended")

    @pytest.mark.asyncio
    async def test_update_user_denied_for_other_user(self):
        store = _make_global_store()
        servicer = _build_servicer(global_store=store)
        context = _mock_context()

        request = UpdateUserRequest(
            actor="user:u2",
            user_id="u1",
            name="Hacked",
        )

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            await servicer.UpdateUser(request, context)

        context.abort.assert_called()
        assert "user themselves or admin" in str(context.abort.call_args)
        store.update_user.assert_not_called()

    @pytest.mark.asyncio
    async def test_update_user_not_found(self):
        store = _make_global_store()
        store.update_user.return_value = False
        servicer = _build_servicer(global_store=store)
        context = _mock_context()

        request = UpdateUserRequest(
            actor="system:admin",
            user_id="u999",
            name="Ghost",
        )

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            response = await servicer.UpdateUser(request, context)

        assert response.success is False
        assert "not found" in response.error.lower()

    @pytest.mark.asyncio
    async def test_update_user_no_fields(self):
        store = _make_global_store()
        servicer = _build_servicer(global_store=store)
        context = _mock_context()

        request = UpdateUserRequest(
            actor="system:admin",
            user_id="u1",
        )

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            response = await servicer.UpdateUser(request, context)

        assert response.success is False
        assert "no fields" in response.error.lower()


# ── ListUsers Tests ──────────────────────────────────────────────────


class TestListUsers:
    """ListUsers RPC handler tests."""

    @pytest.mark.asyncio
    async def test_list_users_default(self):
        store = _make_global_store()
        store.list_users.return_value = [
            {
                "user_id": "u1",
                "email": "a@e.com",
                "name": "A",
                "status": "active",
                "created_at": 1,
                "updated_at": 1,
            },
            {
                "user_id": "u2",
                "email": "b@e.com",
                "name": "B",
                "status": "active",
                "created_at": 2,
                "updated_at": 2,
            },
        ]
        servicer = _build_servicer(global_store=store)
        context = _mock_context()

        request = ListUsersRequest(actor="system:admin")

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            response = await servicer.ListUsers(request, context)

        assert len(response.users) == 2
        assert response.users[0].user_id == "u1"
        assert response.users[1].user_id == "u2"
        store.list_users.assert_called_once_with(status="active", limit=100, offset=0)

    @pytest.mark.asyncio
    async def test_list_users_with_pagination(self):
        store = _make_global_store()
        store.list_users.return_value = []
        servicer = _build_servicer(global_store=store)
        context = _mock_context()

        request = ListUsersRequest(
            actor="user:u1",
            status="suspended",
            limit=10,
            offset=5,
        )

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            response = await servicer.ListUsers(request, context)

        assert len(response.users) == 0
        store.list_users.assert_called_once_with(status="suspended", limit=10, offset=5)


# ── No GlobalStore Tests ─────────────────────────────────────────────


class TestNoGlobalStore:
    """Handlers should fail gracefully when global_store is not configured."""

    @pytest.mark.asyncio
    async def test_create_user_without_global_store(self):
        servicer = _build_servicer(global_store=None)
        context = _mock_context()

        request = CreateUserRequest(
            actor="system:admin",
            user_id="u1",
            email="a@e.com",
            name="A",
        )

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            await servicer.CreateUser(request, context)

        context.abort.assert_called()
        assert "not configured" in str(context.abort.call_args).lower()

    @pytest.mark.asyncio
    async def test_get_user_without_global_store(self):
        servicer = _build_servicer(global_store=None)
        context = _mock_context()

        request = GetUserRequest(actor="user:u1", user_id="u1")

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request"):
            await servicer.GetUser(request, context)

        context.abort.assert_called()
        assert "not configured" in str(context.abort.call_args).lower()


# ── SDK Client Tests ─────────────────────────────────────────────────


class TestSDKUserRegistryClient:
    """Tests for SDK DbClient user registry wrapper methods."""

    @pytest.mark.asyncio
    async def test_sdk_create_user(self):
        from sdk.entdb_sdk.client import DbClient

        client = DbClient("localhost:50051")
        client._connected = True
        client._grpc = AsyncMock()
        client._grpc.create_user = AsyncMock(
            return_value={
                "success": True,
                "error": None,
                "user": {
                    "user_id": "u1",
                    "email": "alice@example.com",
                    "name": "Alice",
                    "status": "active",
                    "created_at": 1000,
                    "updated_at": 1000,
                },
            }
        )

        result = await client.create_user("u1", "alice@example.com", "Alice")
        assert result["success"] is True
        assert result["user"]["user_id"] == "u1"

    @pytest.mark.asyncio
    async def test_sdk_get_user(self):
        from sdk.entdb_sdk.client import DbClient

        client = DbClient("localhost:50051")
        client._connected = True
        client._grpc = AsyncMock()
        client._grpc.get_user = AsyncMock(
            return_value={
                "user_id": "u1",
                "email": "alice@example.com",
                "name": "Alice",
                "status": "active",
                "created_at": 1000,
                "updated_at": 1000,
            }
        )

        result = await client.get_user("u1")
        assert result is not None
        assert result["user_id"] == "u1"

    @pytest.mark.asyncio
    async def test_sdk_get_user_not_found(self):
        from sdk.entdb_sdk.client import DbClient

        client = DbClient("localhost:50051")
        client._connected = True
        client._grpc = AsyncMock()
        client._grpc.get_user = AsyncMock(return_value=None)

        result = await client.get_user("u999")
        assert result is None

    @pytest.mark.asyncio
    async def test_sdk_update_user(self):
        from sdk.entdb_sdk.client import DbClient

        client = DbClient("localhost:50051")
        client._connected = True
        client._grpc = AsyncMock()
        client._grpc.update_user = AsyncMock(
            return_value={
                "success": True,
                "error": None,
            }
        )

        result = await client.update_user("u1", name="Alice Updated")
        assert result["success"] is True

    @pytest.mark.asyncio
    async def test_sdk_list_users(self):
        from sdk.entdb_sdk.client import DbClient

        client = DbClient("localhost:50051")
        client._connected = True
        client._grpc = AsyncMock()
        client._grpc.list_users = AsyncMock(
            return_value=[
                {
                    "user_id": "u1",
                    "email": "a@e.com",
                    "name": "A",
                    "status": "active",
                    "created_at": 1,
                    "updated_at": 1,
                },
                {
                    "user_id": "u2",
                    "email": "b@e.com",
                    "name": "B",
                    "status": "active",
                    "created_at": 2,
                    "updated_at": 2,
                },
            ]
        )

        result = await client.list_users(status="active", limit=50, offset=0)
        assert len(result) == 2
        assert result[0]["user_id"] == "u1"
