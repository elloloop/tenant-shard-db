"""
E2E test fixtures for EntDB.

These tests require docker-compose to be running with the full stack.
"""

import asyncio
import os
import pytest
import subprocess
import time
from typing import Generator

# Skip E2E tests if not in E2E mode
E2E_ENABLED = os.environ.get("ENTDB_E2E_TESTS", "0") == "1"

pytestmark = pytest.mark.skipif(
    not E2E_ENABLED,
    reason="E2E tests disabled. Set ENTDB_E2E_TESTS=1 to enable."
)


def wait_for_service(host: str, port: int, timeout: int = 60) -> bool:
    """Wait for a service to become available."""
    import socket

    start = time.time()
    while time.time() - start < timeout:
        try:
            with socket.create_connection((host, port), timeout=1):
                return True
        except (socket.error, socket.timeout):
            time.sleep(1)
    return False


@pytest.fixture(scope="session")
def docker_compose() -> Generator[None, None, None]:
    """Start docker-compose for E2E tests."""
    if not E2E_ENABLED:
        yield
        return

    compose_file = os.path.join(
        os.path.dirname(__file__),
        "..", "..",
        "docker-compose.yml"
    )

    # Start services
    subprocess.run(
        ["docker-compose", "-f", compose_file, "up", "-d"],
        check=True,
        capture_output=True,
    )

    try:
        # Wait for services
        assert wait_for_service("localhost", 50051, timeout=60), "gRPC not ready"
        assert wait_for_service("localhost", 8080, timeout=60), "HTTP not ready"
        yield
    finally:
        # Stop services
        subprocess.run(
            ["docker-compose", "-f", compose_file, "down", "-v"],
            check=True,
            capture_output=True,
        )


@pytest.fixture
def grpc_channel(docker_compose):
    """Create gRPC channel to server."""
    try:
        import grpc
    except ImportError:
        pytest.skip("grpcio not installed")

    channel = grpc.insecure_channel("localhost:50051")
    yield channel
    channel.close()


@pytest.fixture
def http_base_url(docker_compose) -> str:
    """HTTP base URL for API."""
    return "http://localhost:8080"


@pytest.fixture
def test_tenant_id() -> str:
    """Generate unique tenant ID for test isolation."""
    import uuid
    return f"test_tenant_{uuid.uuid4().hex[:8]}"


@pytest.fixture
def test_actor() -> str:
    """Test actor identity."""
    return "user:e2e_test_user"

