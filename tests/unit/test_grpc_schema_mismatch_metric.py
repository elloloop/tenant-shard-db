"""
Regression test: ExecuteAtomic schema mismatch must record metric as "error".

Bug: When a client sends an ExecuteAtomic request with a schema fingerprint
that doesn't match the server's fingerprint, the response correctly returns
success=False with error_code="SCHEMA_MISMATCH", but the Prometheus metric
was recorded as status="ok".  This made schema mismatch errors invisible in
production dashboards.

Fix: Record the metric with status="error" so monitoring alerts fire.
"""

from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from google.protobuf.struct_pb2 import Struct

from dbaas.entdb_server.api.generated import (
    CreateNodeOp,
    ExecuteAtomicRequest,
    Operation,
    RequestContext,
)
from dbaas.entdb_server.api.grpc_server import EntDBServicer


def _build_servicer(server_fingerprint: str) -> EntDBServicer:
    """Build an EntDBServicer with a schema registry that has the given fingerprint."""
    registry = MagicMock()
    registry.fingerprint = server_fingerprint
    return EntDBServicer(
        wal=AsyncMock(),
        canonical_store=AsyncMock(),
        mailbox_store=AsyncMock(),
        schema_registry=registry,
    )


def _build_request(client_fingerprint: str) -> ExecuteAtomicRequest:
    """Build an ExecuteAtomicRequest with the given schema fingerprint."""
    return ExecuteAtomicRequest(
        context=RequestContext(tenant_id="t1", actor="user:1"),
        idempotency_key="key-1",
        schema_fingerprint=client_fingerprint,
        operations=[
            Operation(create_node=CreateNodeOp(type_id=1, data=Struct())),
        ],
    )


class TestExecuteAtomicSchemaMismatchMetric:
    """ExecuteAtomic must record schema mismatch as an error metric."""

    @pytest.mark.asyncio
    async def test_schema_mismatch_records_error_metric(self):
        servicer = _build_servicer(server_fingerprint="sha256:server")
        request = _build_request(client_fingerprint="sha256:stale-client")
        context = AsyncMock()

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request") as mock_record:
            response = await servicer.ExecuteAtomic(request, context)

        # The response must indicate failure
        assert response.success is False
        assert response.error_code == "SCHEMA_MISMATCH"

        # The metric must be recorded as "error", not "ok"
        mock_record.assert_called_once()
        call_args = mock_record.call_args
        assert call_args[0][0] == "ExecuteAtomic"
        assert call_args[0][1] == "error", (
            f"Schema mismatch should be recorded as 'error' in metrics, but got '{call_args[0][1]}'"
        )

    @pytest.mark.asyncio
    async def test_matching_schema_records_ok_metric(self):
        """Sanity check: a successful request still records 'ok'."""
        servicer = _build_servicer(server_fingerprint="sha256:same")
        request = _build_request(client_fingerprint="sha256:same")
        context = AsyncMock()

        # wal.append needs to return a position-like object
        pos_mock = MagicMock()
        pos_mock.__str__ = MagicMock(return_value="0:0:0")
        servicer.wal.append = AsyncMock(return_value=pos_mock)

        with patch("dbaas.entdb_server.api.grpc_server.record_grpc_request") as mock_record:
            response = await servicer.ExecuteAtomic(request, context)

        assert response.success is True
        mock_record.assert_called_once()
        assert mock_record.call_args[0][1] == "ok"
