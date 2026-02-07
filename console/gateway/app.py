"""
FastAPI application factory for EntDB Console.

This module creates the main FastAPI app with:
- CORS configuration for frontend
- Console gRPC client lifecycle management
- Read-only API routes
- Static file serving for frontend
"""

from contextlib import asynccontextmanager
from pathlib import Path
from typing import AsyncGenerator

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles

from .config import Settings
from .console_client import ConsoleClient
from .routes import router


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    """Manage Console client lifecycle."""
    settings = Settings()
    client = ConsoleClient(host=settings.entdb_host, port=settings.entdb_port)

    await client.connect()
    app.state.console_client = client
    app.state.settings = settings

    yield

    await client.close()


def create_app() -> FastAPI:
    """Create and configure the FastAPI application."""
    settings = Settings()

    app = FastAPI(
        title="EntDB Console",
        description=(
            "Read-only web interface for browsing EntDB data. "
            "All write operations must go through the SDK."
        ),
        version="1.0.0",
        lifespan=lifespan,
    )

    # CORS for frontend
    app.add_middleware(
        CORSMiddleware,
        allow_origins=settings.cors_origins,
        allow_credentials=True,
        allow_methods=["GET", "OPTIONS"],  # Read-only
        allow_headers=["*"],
    )

    # API routes
    app.include_router(router, prefix="/api/v1")

    # Health endpoint at root
    @app.get("/health")
    async def health():
        return {"status": "healthy", "service": "entdb-console", "mode": "read-only"}

    # Serve frontend static files
    # Check Docker path first, then local dev path
    docker_static_dir = Path("/app/console/static")
    local_static_dir = Path(__file__).parent.parent / "frontend" / "dist"

    static_dir = docker_static_dir if docker_static_dir.exists() else local_static_dir
    if static_dir.exists():
        app.mount("/", StaticFiles(directory=str(static_dir), html=True), name="frontend")

    return app


# Default app instance
app = create_app()
