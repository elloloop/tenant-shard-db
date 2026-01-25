"""
API module for EntDB server.

This module provides the external interfaces:
- gRPC server (primary API)
- HTTP server (optional REST API)

Both servers share the same underlying service logic.

Invariants:
    - All operations require tenant_id and actor
    - Writes go through WAL before acknowledgment
    - Reads come from SQLite materialized views

How to change safely:
    - gRPC changes must be backward compatible
    - Add new RPC methods, don't modify existing ones
    - HTTP endpoints should match gRPC semantics
"""

from .grpc_server import GrpcServer
from .http_server import create_http_app

__all__ = [
    "GrpcServer",
    "create_http_app",
]
