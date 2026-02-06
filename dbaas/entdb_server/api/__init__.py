"""
API module for EntDB server.

This module provides the external interface via gRPC.

For HTTP/REST access, use the entdb-gateway sidecar project
which demonstrates SDK usage and provides a web-based data browser.
See examples/entdb-gateway/

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
