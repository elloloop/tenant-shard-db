"""
Standard gRPC Health Checking Protocol.

The server must expose ``grpc.health.v1.Health/Check`` so orchestrators
(Docker HEALTHCHECK, k8s livenessProbe, ECS) can probe it via
``grpc_health_probe`` or equivalent. Without this, the only way to
know the server is up was via a (non-existent) HTTP endpoint or by
attempting a full ``EntDBService`` RPC, which can't be done without
auth context in production.
"""

from __future__ import annotations

from unittest.mock import MagicMock

import grpc
import pytest
from grpc_health.v1 import health_pb2, health_pb2_grpc

from dbaas.entdb_server.api.grpc_server import EntDBServicer, GrpcServer


def _make_server(port: int) -> GrpcServer:
    servicer = EntDBServicer(
        wal=MagicMock(),
        canonical_store=MagicMock(),
        schema_registry=MagicMock(),
    )
    return GrpcServer(servicer=servicer, host="127.0.0.1", port=port, max_workers=2)


@pytest.mark.asyncio
async def test_health_servicer_registered_and_serving():
    """The standard Health servicer is wired into the gRPC server.

    Guards against silent regression — if someone removes the
    ``add_HealthServicer_to_server`` call, this test fails before the
    Docker HEALTHCHECK does.
    """
    server = _make_server(port=0)
    await server.start()
    try:
        assert hasattr(server, "_health_servicer"), (
            "GrpcServer.start() must register the standard gRPC Health servicer"
        )
        # The async HealthServicer stores statuses in a private dict
        # keyed by service name; the empty string is the default service
        # (what grpc_health_probe uses with no -service flag).
        statuses = server._health_servicer._server_status
        assert statuses[""] == health_pb2.HealthCheckResponse.SERVING
    finally:
        await server.stop(grace_period=0.1)


@pytest.mark.asyncio
async def test_health_check_via_real_client():
    """End-to-end: open a gRPC channel and call ``Health/Check``.

    Mirrors exactly what ``grpc_health_probe -addr=:50051`` does. If
    this test passes, the Docker HEALTHCHECK works.
    """
    # Pick a high, unlikely-to-conflict port for the real-client probe.
    server = _make_server(port=50099)
    await server.start()
    try:
        async with grpc.aio.insecure_channel("127.0.0.1:50099") as channel:
            stub = health_pb2_grpc.HealthStub(channel)
            resp = await stub.Check(health_pb2.HealthCheckRequest(service=""), timeout=5.0)
            assert resp.status == health_pb2.HealthCheckResponse.SERVING
    finally:
        await server.stop(grace_period=0.1)
