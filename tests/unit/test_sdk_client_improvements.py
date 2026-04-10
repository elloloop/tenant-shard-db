"""Tests for SDK improvements: filter, trace_id, api_key, timeout, retry, connection check."""

from __future__ import annotations

import json
from unittest.mock import AsyncMock, MagicMock, patch

import grpc
import pytest

from sdk.entdb_sdk._grpc_client import GrpcClient, GrpcCommitResult, GrpcReceipt
from sdk.entdb_sdk.client import DbClient, Plan
from sdk.entdb_sdk.errors import ConnectionError
from sdk.entdb_sdk.schema import FieldDef, FieldKind, NodeTypeDef


def _make_field(id: int, name: str, kind_str: str, **kwargs) -> FieldDef:
    """Helper to create field definitions."""
    kind_map = {
        "str": FieldKind.STRING,
        "int": FieldKind.INTEGER,
        "bool": FieldKind.BOOLEAN,
    }
    return FieldDef(
        field_id=id,
        name=name,
        kind=kind_map.get(kind_str, FieldKind.STRING),
        **kwargs,
    )


@pytest.fixture
def task_type():
    """Task type definition for testing."""
    return NodeTypeDef(
        type_id=2,
        name="Task",
        fields=(
            _make_field(1, "title", "str"),
            _make_field(2, "done", "bool"),
        ),
    )


@pytest.fixture
def mock_grpc_client():
    """Create a mock GrpcClient with all methods as AsyncMock."""
    client = MagicMock(spec=GrpcClient)
    client._host = "localhost"
    client._port = 50051
    client.connect = AsyncMock()
    client.close = AsyncMock()

    # Default return values for gRPC methods
    client.get_node = AsyncMock(return_value=None)
    client.get_nodes = AsyncMock(return_value=([], []))
    client.query_nodes = AsyncMock(return_value=([], False))
    client.get_edges_from = AsyncMock(return_value=([], False))
    client.get_edges_to = AsyncMock(return_value=([], False))
    client.search_mailbox = AsyncMock(return_value=[])
    client.health = AsyncMock(return_value={"healthy": True, "version": "1.0", "components": {}})
    client.execute_atomic = AsyncMock(
        return_value=GrpcCommitResult(
            success=True,
            receipt=GrpcReceipt(tenant_id="t1", idempotency_key="key1", stream_position="1"),
            created_node_ids=["node_1"],
            applied=True,
            error=None,
        )
    )
    client.get_receipt_status = AsyncMock(return_value="APPLIED")

    return client


def _make_db_client(mock_grpc: MagicMock, **kwargs) -> DbClient:
    """Create a DbClient wired to a mock GrpcClient."""
    db = DbClient.__new__(DbClient)
    db._grpc = mock_grpc
    db._connected = True

    # Set up a mock registry
    registry = MagicMock()
    registry.fingerprint = "test-fingerprint"
    db.registry = registry
    return db


# ---------------------------------------------------------------------------
# Fix 1 & 6: query with filter, order_by, descending
# ---------------------------------------------------------------------------


class TestQueryFilter:
    """Tests for query with typed filter dict."""

    @pytest.mark.asyncio
    async def test_query_passes_filter_dict(self, task_type, mock_grpc_client):
        """Filter dict is passed as-is to gRPC client."""
        db = _make_db_client(mock_grpc_client)
        filter_dict = {"status": "active", "priority": 1}

        await db.query(task_type, "t1", "user:1", filter=filter_dict)

        mock_grpc_client.query_nodes.assert_awaited_once()
        call_kwargs = mock_grpc_client.query_nodes.call_args.kwargs
        assert call_kwargs["filter"] == filter_dict

    @pytest.mark.asyncio
    async def test_query_empty_filter(self, task_type, mock_grpc_client):
        """No filter results in empty dict."""
        db = _make_db_client(mock_grpc_client)

        await db.query(task_type, "t1", "user:1")

        call_kwargs = mock_grpc_client.query_nodes.call_args.kwargs
        assert call_kwargs["filter"] == {}

    @pytest.mark.asyncio
    async def test_query_filter_with_pagination(self, task_type, mock_grpc_client):
        """Filter works alongside limit and offset."""
        db = _make_db_client(mock_grpc_client)

        await db.query(task_type, "t1", "user:1", filter={"done": True}, limit=10, offset=5)

        call_kwargs = mock_grpc_client.query_nodes.call_args.kwargs
        assert call_kwargs["filter"] == {"done": True}
        assert call_kwargs["limit"] == 10
        assert call_kwargs["offset"] == 5

    @pytest.mark.asyncio
    async def test_query_order_by_default(self, task_type, mock_grpc_client):
        """Default order_by is 'created_at' descending."""
        db = _make_db_client(mock_grpc_client)

        await db.query(task_type, "t1", "user:1")

        call_kwargs = mock_grpc_client.query_nodes.call_args.kwargs
        assert call_kwargs["order_by"] == "created_at"
        assert call_kwargs["descending"] is True

    @pytest.mark.asyncio
    async def test_query_custom_ordering(self, task_type, mock_grpc_client):
        """Custom order_by and descending are passed through."""
        db = _make_db_client(mock_grpc_client)

        await db.query(task_type, "t1", "user:1", order_by="updated_at", descending=False)

        call_kwargs = mock_grpc_client.query_nodes.call_args.kwargs
        assert call_kwargs["order_by"] == "updated_at"
        assert call_kwargs["descending"] is False


# ---------------------------------------------------------------------------
# Fix 2: trace_id
# ---------------------------------------------------------------------------


class TestTraceId:
    """Tests for trace_id propagation."""

    @pytest.mark.asyncio
    async def test_auto_generated_trace_id(self, task_type, mock_grpc_client):
        """trace_id is auto-generated when not provided."""
        db = _make_db_client(mock_grpc_client)

        await db.get(task_type, "node_1", "t1", "user:1")

        call_kwargs = mock_grpc_client.get_node.call_args.kwargs
        assert call_kwargs["trace_id"] != ""
        # Should be a valid UUID-like string
        assert len(call_kwargs["trace_id"]) == 36

    @pytest.mark.asyncio
    async def test_custom_trace_id(self, task_type, mock_grpc_client):
        """Custom trace_id is passed through to gRPC."""
        db = _make_db_client(mock_grpc_client)

        await db.get(task_type, "node_1", "t1", "user:1", trace_id="my-trace-123")

        call_kwargs = mock_grpc_client.get_node.call_args.kwargs
        assert call_kwargs["trace_id"] == "my-trace-123"

    @pytest.mark.asyncio
    async def test_trace_id_in_plan(self, task_type, mock_grpc_client):
        """Plan accepts and passes trace_id on commit."""
        db = _make_db_client(mock_grpc_client)

        plan = Plan(db, "t1", "user:1", trace_id="plan-trace-456")
        plan.create(task_type, {"title": "Test", "done": False})
        await plan.commit()

        call_kwargs = mock_grpc_client.execute_atomic.call_args.kwargs
        assert call_kwargs["trace_id"] == "plan-trace-456"


# ---------------------------------------------------------------------------
# Fix 3: api_key
# ---------------------------------------------------------------------------


class TestApiKey:
    """Tests for API key authentication."""

    def test_api_key_stored_in_grpc_client(self):
        """api_key is stored in GrpcClient."""
        grpc = GrpcClient(host="localhost", port=50051, api_key="test-key-123")
        assert grpc._api_key == "test-key-123"

    def test_api_key_in_metadata(self):
        """api_key produces Bearer metadata."""
        grpc = GrpcClient(host="localhost", port=50051, api_key="secret-key")
        metadata = grpc._build_metadata()
        assert ("authorization", "Bearer secret-key") in metadata

    def test_no_api_key_empty_metadata(self):
        """No api_key means empty metadata list."""
        grpc = GrpcClient(host="localhost", port=50051)
        metadata = grpc._build_metadata()
        assert metadata == []

    def test_db_client_passes_api_key_to_grpc(self):
        """DbClient passes api_key to GrpcClient constructor."""
        db = DbClient("localhost:50051", api_key="my-api-key")
        assert db._grpc._api_key == "my-api-key"


# ---------------------------------------------------------------------------
# Fix 4: per-call timeout
# ---------------------------------------------------------------------------


class TestTimeout:
    """Tests for per-call timeout."""

    @pytest.mark.asyncio
    async def test_default_timeout(self, task_type, mock_grpc_client):
        """Default timeout is passed when none specified."""
        db = _make_db_client(mock_grpc_client)

        await db.get(task_type, "node_1", "t1", "user:1")

        call_kwargs = mock_grpc_client.get_node.call_args.kwargs
        # None is passed, meaning GrpcClient will use its default (30s)
        assert call_kwargs["timeout"] is None

    @pytest.mark.asyncio
    async def test_custom_timeout(self, task_type, mock_grpc_client):
        """Custom timeout is passed to gRPC layer."""
        db = _make_db_client(mock_grpc_client)

        await db.get(task_type, "node_1", "t1", "user:1", timeout=5.0)

        call_kwargs = mock_grpc_client.get_node.call_args.kwargs
        assert call_kwargs["timeout"] == 5.0

    @pytest.mark.asyncio
    async def test_timeout_on_query(self, task_type, mock_grpc_client):
        """Timeout is passed through on query calls."""
        db = _make_db_client(mock_grpc_client)

        await db.query(task_type, "t1", "user:1", timeout=10.0)

        call_kwargs = mock_grpc_client.query_nodes.call_args.kwargs
        assert call_kwargs["timeout"] == 10.0


# ---------------------------------------------------------------------------
# Fix 5: retry logic
# ---------------------------------------------------------------------------


class TestRetry:
    """Tests for retry logic on transient gRPC errors."""

    @pytest.mark.asyncio
    async def test_retries_on_unavailable(self):
        """Retries on UNAVAILABLE and succeeds."""
        grpc_client = GrpcClient(host="localhost", port=50051, max_retries=3)

        call_count = 0

        async def flaky_call(*args, **kwargs):
            nonlocal call_count
            call_count += 1
            if call_count < 3:
                error = grpc.RpcError()
                error.code = lambda: grpc.StatusCode.UNAVAILABLE
                raise error
            return "success"

        # Patch asyncio.sleep to avoid delays in tests
        with patch("sdk.entdb_sdk._grpc_client.asyncio.sleep", new_callable=AsyncMock):
            result = await grpc_client._retry(flaky_call)

        assert result == "success"
        assert call_count == 3

    @pytest.mark.asyncio
    async def test_no_retry_on_invalid_argument(self):
        """Does not retry on non-transient errors like INVALID_ARGUMENT."""
        grpc_client = GrpcClient(host="localhost", port=50051, max_retries=3)

        call_count = 0

        async def failing_call(*args, **kwargs):
            nonlocal call_count
            call_count += 1
            error = grpc.RpcError()
            error.code = lambda: grpc.StatusCode.INVALID_ARGUMENT
            raise error

        with (
            pytest.raises(grpc.RpcError),
            patch("sdk.entdb_sdk._grpc_client.asyncio.sleep", new_callable=AsyncMock),
        ):
            await grpc_client._retry(failing_call)

        assert call_count == 1  # No retries

    @pytest.mark.asyncio
    async def test_exponential_backoff(self):
        """Backoff increases exponentially between retries."""
        grpc_client = GrpcClient(host="localhost", port=50051, max_retries=3)

        call_count = 0

        async def always_unavailable(*args, **kwargs):
            nonlocal call_count
            call_count += 1
            error = grpc.RpcError()
            error.code = lambda: grpc.StatusCode.UNAVAILABLE
            raise error

        sleep_durations: list[float] = []

        async def mock_sleep(duration: float) -> None:
            sleep_durations.append(duration)

        with (
            pytest.raises(grpc.RpcError),
            patch("sdk.entdb_sdk._grpc_client.asyncio.sleep", side_effect=mock_sleep),
        ):
            await grpc_client._retry(always_unavailable)

        # Verify exponential backoff: 0.1, 0.2, 0.4
        assert len(sleep_durations) == 3
        assert sleep_durations[0] == pytest.approx(0.1)
        assert sleep_durations[1] == pytest.approx(0.2)
        assert sleep_durations[2] == pytest.approx(0.4)

    @pytest.mark.asyncio
    async def test_max_retries_exceeded(self):
        """Raises after max retries exhausted."""
        grpc_client = GrpcClient(host="localhost", port=50051, max_retries=2)

        call_count = 0

        async def always_fails(*args, **kwargs):
            nonlocal call_count
            call_count += 1
            error = grpc.RpcError()
            error.code = lambda: grpc.StatusCode.DEADLINE_EXCEEDED
            raise error

        with (
            pytest.raises(grpc.RpcError),
            patch("sdk.entdb_sdk._grpc_client.asyncio.sleep", new_callable=AsyncMock),
        ):
            await grpc_client._retry(always_fails)

        # 1 initial + 2 retries = 3 total
        assert call_count == 3


# ---------------------------------------------------------------------------
# Fix 7: connection status check
# ---------------------------------------------------------------------------


class TestConnectionCheck:
    """Tests for connection status enforcement."""

    @pytest.mark.asyncio
    async def test_error_before_connect(self, task_type):
        """Raises ConnectionError when not connected."""
        db = DbClient("localhost:50051")
        # Not connected yet

        with pytest.raises(ConnectionError) as exc_info:
            await db.get(task_type, "node_1", "t1", "user:1")

        assert "Not connected" in str(exc_info.value)
        assert exc_info.value.address == "localhost:50051"

    @pytest.mark.asyncio
    async def test_works_after_connect(self, task_type, mock_grpc_client):
        """Methods work when connected."""
        db = _make_db_client(mock_grpc_client)
        # _connected = True from _make_db_client

        # Should not raise
        result = await db.get(task_type, "node_1", "t1", "user:1")
        assert result is None  # Mock returns None

    @pytest.mark.asyncio
    async def test_error_after_close(self, task_type, mock_grpc_client):
        """Raises ConnectionError after close is called."""
        db = _make_db_client(mock_grpc_client)

        # Close the connection
        await db.close()

        with pytest.raises(ConnectionError):
            await db.get(task_type, "node_1", "t1", "user:1")

    def test_atomic_requires_connection(self, task_type, mock_grpc_client):
        """atomic() requires connection."""
        db = DbClient("localhost:50051")
        # Not connected

        with pytest.raises(ConnectionError):
            db.atomic("t1", "user:1")

    @pytest.mark.asyncio
    async def test_health_requires_connection(self, mock_grpc_client):
        """health() requires connection."""
        db = DbClient("localhost:50051")

        with pytest.raises(ConnectionError):
            await db.health()

    @pytest.mark.asyncio
    async def test_execute_requires_connection(self, mock_grpc_client):
        """_execute() requires connection."""
        db = DbClient("localhost:50051")

        with pytest.raises(ConnectionError):
            await db._execute(
                tenant_id="t1",
                actor="user:1",
                operations=[],
                idempotency_key="key",
            )
