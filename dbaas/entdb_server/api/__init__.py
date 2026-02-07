"""
API module for EntDB server.

This module provides the external interface via gRPC.

For HTTP/REST access, use the EntDB Console which provides
a web UI and REST API for data browsing (similar to phpMyAdmin).
See console/

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
