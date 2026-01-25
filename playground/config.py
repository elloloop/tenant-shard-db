"""
Configuration for EntDB Playground.
"""

from pydantic import Field
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Playground configuration."""

    # EntDB server
    entdb_host: str = Field(default="localhost")
    entdb_port: int = Field(default=50051)

    # Playground settings
    host: str = Field(default="0.0.0.0")
    port: int = Field(default=8081)

    # Sandbox tenant - all playground data goes here
    sandbox_tenant: str = Field(default="playground")
    sandbox_actor: str = Field(default="playground:user")

    # Auto-cleanup TTL (seconds) - 0 = no auto-cleanup
    cleanup_ttl: int = Field(default=3600, description="Auto-delete data after N seconds (0=disabled)")

    # CORS
    cors_origins: list[str] = Field(
        default=["http://localhost:3000", "http://localhost:5173", "http://localhost:8080"],
    )

    model_config = {"env_prefix": "PLAYGROUND_"}
