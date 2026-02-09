"""
EntDB Playground - Simple data writer for experimenting with EntDB.

All data goes to a 'playground' tenant visible in Console.

Usage:
    uvicorn playground.app:app --port 8081
"""

from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles

from entdb_sdk import DbClient
from entdb_sdk.registry import SchemaRegistry

from .config import Settings
from .routes import router
from .schema import ALL_EDGE_TYPES, ALL_NODE_TYPES


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
        description="Simple data writer for experimenting with EntDB.",
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

    @app.get("/api")
    async def api_info():
        return {
            "service": "entdb-playground",
            "sandbox_tenant": settings.sandbox_tenant,
            "endpoints": {
                "create_node": "POST /api/v1/nodes",
                "create_edge": "POST /api/v1/edges",
            },
        }

    @app.get("/health")
    async def health():
        return {"status": "healthy", "service": "entdb-playground"}

    # Serve frontend static files
    docker_static_dir = Path("/app/playground/static")
    local_static_dir = Path(__file__).parent / "frontend" / "dist"

    static_dir = docker_static_dir if docker_static_dir.exists() else local_static_dir
    if static_dir.exists():
        app.mount("/", StaticFiles(directory=str(static_dir), html=True), name="frontend")

    return app


app = create_app()
