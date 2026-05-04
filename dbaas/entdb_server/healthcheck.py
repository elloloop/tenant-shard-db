"""
Healthcheck CLI — calls the standard gRPC Health Checking RPC.

Mirrors ``grpc_health_probe`` so we don't need to download a Go binary
into the image just to call ``grpc.health.v1.Health/Check``. Used by
the Docker ``HEALTHCHECK`` instruction.

Exit codes:
    0  — service is SERVING
    1  — service is not serving, channel failed, or RPC errored

Usage:
    python -m dbaas.entdb_server.healthcheck [--addr HOST:PORT] [--service NAME]

Defaults: ``--addr=localhost:50051``, ``--service=``  (empty = default service).
"""

from __future__ import annotations

import argparse
import sys

import grpc
from grpc_health.v1 import health_pb2, health_pb2_grpc


def main() -> int:
    parser = argparse.ArgumentParser(description="gRPC Health Check probe")
    parser.add_argument("--addr", default="localhost:50051", help="gRPC server host:port")
    parser.add_argument(
        "--service",
        default="",
        help="Service name to probe (empty = default service)",
    )
    parser.add_argument("--timeout", type=float, default=5.0, help="RPC timeout in seconds")
    args = parser.parse_args()

    try:
        channel = grpc.insecure_channel(args.addr)
        stub = health_pb2_grpc.HealthStub(channel)
        resp = stub.Check(
            health_pb2.HealthCheckRequest(service=args.service),
            timeout=args.timeout,
        )
    except grpc.RpcError as exc:
        print(f"healthcheck: RPC failed: {exc}", file=sys.stderr)
        return 1
    except Exception as exc:
        print(f"healthcheck: {exc}", file=sys.stderr)
        return 1

    if resp.status == health_pb2.HealthCheckResponse.SERVING:
        return 0

    status_name = health_pb2.HealthCheckResponse.ServingStatus.Name(resp.status)
    print(f"healthcheck: status={status_name}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
