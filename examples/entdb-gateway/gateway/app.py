"""
FastAPI application factory for EntDB HTTP Gateway.

This module creates the main FastAPI app with:
- CORS configuration for frontend
- SDK client lifecycle management
- API routes
- Static file serving for frontend
"""

import os
from contextlib import asynccontextmanager
from pathlib import Path
from typing import AsyncGenerator

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles

from .config import Settings
from .routes import router
from .sdk_client import SdkClientManager


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    """Manage SDK client lifecycle."""
    settings = Settings()
    manager = SdkClientManager(settings)

    await manager.start()
    app.state.sdk_manager = manager

    yield

    await manager.stop()


def create_app() -> FastAPI:
    """Create and configure the FastAPI application."""
    settings = Settings()

    app = FastAPI(
        title="EntDB HTTP Gateway",
        description="REST API gateway for EntDB, with data browsing frontend",
        version="1.0.0",
        lifespan=lifespan,
    )

    # CORS for frontend
    app.add_middleware(
        CORSMiddleware,
        allow_origins=settings.cors_origins,
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    # API routes
    app.include_router(router, prefix="/api/v1")

    # Health endpoint at root
    @app.get("/health")
    async def health():
        return {"status": "healthy", "service": "entdb-gateway"}

    # Serve frontend static files in production
    frontend_dir = Path(__file__).parent.parent / "frontend" / "dist"
    if frontend_dir.exists():
        app.mount("/", StaticFiles(directory=str(frontend_dir), html=True), name="frontend")

    return app


# Default app instance
app = create_app()
