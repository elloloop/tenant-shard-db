"""
EntDB Playground - Interactive SDK simulator.

A write-enabled sandbox for developers to experiment with EntDB.
All data goes to a 'playground' tenant visible in Console.

Features:
- Create, update, delete nodes
- Create, delete edges
- Atomic transactions
- SDK code generation for each operation
- Auto-cleanup (TTL-based)

Usage:
    uvicorn playground.app:app --port 8081
"""

from contextlib import asynccontextmanager
from typing import AsyncGenerator

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from entdb_sdk import DbClient
from entdb_sdk.registry import SchemaRegistry

from .config import Settings
from .routes import router
from .schema import ALL_NODE_TYPES, ALL_EDGE_TYPES


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    """Manage SDK client lifecycle."""
    settings = Settings()

    # Create registry with playground schema
    registry = SchemaRegistry()
    for node_type in ALL_NODE_TYPES:
        registry.register_node_type(node_type)
    for edge_type in ALL_EDGE_TYPES:
        registry.register_edge_type(edge_type)

    # Connect SDK client
    endpoint = f"{settings.entdb_host}:{settings.entdb_port}"
    client = DbClient(endpoint, registry=registry)
    await client.connect()

    app.state.db_client = client
    app.state.settings = settings

    yield

    await client.close()


def create_app() -> FastAPI:
    """Create the Playground FastAPI app."""
    settings = Settings()

    app = FastAPI(
        title="EntDB Playground",
        description=(
            "Interactive sandbox for experimenting with EntDB. "
            "All writes go to the 'playground' tenant, visible in Console. "
            "Each response includes equivalent SDK code."
        ),
        version="1.0.0",
        lifespan=lifespan,
    )

    # CORS
    app.add_middleware(
        CORSMiddleware,
        allow_origins=settings.cors_origins,
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    # API routes
    app.include_router(router, prefix="/api/v1")

    # Root info
    @app.get("/")
    async def root():
        return {
            "service": "entdb-playground",
            "description": "Interactive SDK simulator for EntDB",
            "sandbox_tenant": settings.sandbox_tenant,
            "console_url": "http://localhost:8080",
            "endpoints": {
                "schema": "/api/v1/schema",
                "create_node": "POST /api/v1/nodes",
                "update_node": "PATCH /api/v1/nodes",
                "delete_node": "DELETE /api/v1/nodes",
                "create_edge": "POST /api/v1/edges",
                "delete_edge": "DELETE /api/v1/edges",
                "atomic": "POST /api/v1/atomic",
            },
            "tips": [
                "View created data in Console at http://localhost:8080",
                "Use tenant_id='playground' in Console to see sandbox data",
                "Each response includes equivalent SDK code",
            ],
        }

    @app.get("/health")
    async def health():
        return {"status": "healthy", "service": "entdb-playground"}

    return app


app = create_app()
