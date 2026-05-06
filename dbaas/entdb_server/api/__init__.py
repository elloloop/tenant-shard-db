"""
API module for EntDB server.

This module provides the external interface via gRPC.

For a web-based data browser, use the standalone `entdb-console`
Go binary in `sdk/go/entdb/cmd/entdb-console/`.

Invariants:
    - All operations require tenant_id and actor
    - Writes go through WAL before acknowledgment
    - Reads come from SQLite materialized views

How to change safely:
    - gRPC changes must be backward compatible
    - Add new RPC methods, don't modify existing ones
"""

from .grpc_server import GrpcServer

__all__ = [
    "GrpcServer",
]
