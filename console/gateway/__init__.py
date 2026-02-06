"""
EntDB HTTP Gateway - A sidecar service providing REST API access to EntDB.

This gateway demonstrates:
1. Using the entdb_sdk to connect to the gRPC server
2. Exposing a REST API for web clients
3. Serving a data browsing frontend
"""

from .app import create_app
from .routes import router

__all__ = ["create_app", "router"]
