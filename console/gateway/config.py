"""
Configuration for EntDB Console.

Uses pydantic-settings for environment variable loading.
"""

from pydantic import Field
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Console configuration loaded from environment."""

    # EntDB server connection
    entdb_host: str = Field(default="localhost", description="EntDB gRPC server host")
    entdb_port: int = Field(default=50051, description="EntDB gRPC server port")

    # Default tenant (can be overridden per-request)
    default_tenant_id: str = Field(default="default", description="Default tenant ID")

    # Console settings
    host: str = Field(default="0.0.0.0", description="Console bind host")
    port: int = Field(default=8080, description="Console bind port")

    # CORS settings
    cors_origins: list[str] = Field(
        default=["http://localhost:3000", "http://localhost:5173"],
        description="Allowed CORS origins",
    )

    # Connection pool
    max_connections: int = Field(default=10, description="Max SDK connections")
    connection_timeout: float = Field(default=30.0, description="Connection timeout seconds")

    # Pagination defaults
    default_page_size: int = Field(default=50, description="Default items per page")
    max_page_size: int = Field(default=200, description="Maximum items per page")

    model_config = {"env_prefix": "CONSOLE_"}

    @property
    def entdb_endpoint(self) -> str:
        """Full EntDB gRPC endpoint."""
        return f"{self.entdb_host}:{self.entdb_port}"
