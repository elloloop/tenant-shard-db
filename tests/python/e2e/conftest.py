# SPDX-License-Identifier: AGPL-3.0-only
"""
E2E test fixtures for EntDB.

These tests require docker compose to be running with the full stack
(see docker-compose.e2e.yml or docker-compose.e2e.go.yml). They drive
the gRPC server via the Python SDK — the legacy HTTP/REST fixtures
were retired in Wave 8 of EPIC #407 when the dead `test_full_flow.py`
was ported to gRPC.

Fixtures:

* ``grpc_target``    — ``"<host>:<port>"`` string for the server under
                       test. Resolves ``ENTDB_HOST``/``ENTDB_PORT``
                       (defaults: ``server:50051`` inside the
                       e2e-tests container, ``localhost:50051`` for
                       host-side ad-hoc runs).
* ``server_target``  — ``"python"`` or ``"go"``, set from
                       ``ENTDB_SERVER_TARGET``. Tests can branch on
                       this when behaviour legitimately differs (the
                       only such case today is the crash-recovery
                       test, which uses a different compose file).
* ``db_client``      — session-scoped, connected ``DbClient`` with the
                       e2e_schema_pb2 schema registered.
* ``tenant_id``      — module-scoped tenant id (the pre-seeded
                       ``e2e-test`` for Python+Go) for tests that
                       don't need isolation.
* ``actor``          — the canonical e2e actor string (matches the
                       Go-server ``-seed-profile=e2e`` runner user).
* ``scope``          — function-scoped ``ActorScope`` bound to
                       ``tenant_id`` + ``actor``.
* ``fresh_tenant``   — function-scoped fixture that creates a new
                       tenant via ``CreateTenant`` and returns its id.
                       Used by the multi-tenant isolation test.
* ``restart_server`` — async callable that ``docker compose restart``s
                       the server container and waits for the gRPC
                       endpoint to come back. Used by the crash-
                       recovery test.

The legacy ``ENTDB_E2E_TESTS=1`` gate has been removed — the harness
(run-e2e.sh) is the only invoker and it always wants the e2e tests
to run.
"""

from __future__ import annotations

import asyncio
import os
import subprocess
import sys
import time
from collections.abc import AsyncGenerator, Awaitable, Callable, Generator

import pytest
import pytest_asyncio

# gRPC client is function-scoped. A session-scoped DbClient would be
# cheaper but pytest-asyncio's default function-scoped event loop
# tears the loop down between tests, and ``grpc.aio`` channels are
# bound to the loop they were created on — yielding "attached to a
# different loop" errors. A per-test channel is the path of least
# surprise; ``insecure_channel`` + Connect is sub-millisecond on a
# warm server.

# Make `import e2e_schema_pb2` resolve when pytest is invoked from
# the repo root or from inside the e2e-tests container.
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

# These imports require the SDK to be importable; the e2e-tests
# container Dockerfile pip-installs entdb-sdk so that's fine.
import e2e_schema_pb2 as pb  # noqa: E402

from entdb_sdk import DbClient, register_proto_schema  # noqa: E402

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

HOST = os.environ.get("ENTDB_HOST", "server")
PORT = int(os.environ.get("ENTDB_PORT", "50051"))
TENANT = os.environ.get("ENTDB_TENANT", "e2e-test")
TLS_CA = os.environ.get("ENTDB_TLS_CA", "")
ACTOR = "user:e2e-runner"
ADMIN_ACTOR = "system:e2e-admin"

# Path inside the e2e-tests container that's bind-mounted to the host's
# compose file location — populated by docker-compose.e2e.yml so that
# the crash-recovery test can `docker compose restart` the server.
COMPOSE_FILE_IN_CONTAINER = "/compose/docker-compose.yml"


# ---------------------------------------------------------------------------
# Simple identifiers
# ---------------------------------------------------------------------------


@pytest.fixture(scope="session")
def grpc_target() -> str:
    return f"{HOST}:{PORT}"


@pytest.fixture(scope="session")
def server_target() -> str:
    """Which server implementation we're driving.

    Always ``"go"`` since Phase 4D of EPIC #407 retired the Python server.
    Kept as a fixture so older tests that branched on this value keep
    compiling; the legacy ``"python"`` branches are now unreachable.
    """
    return os.environ.get("ENTDB_SERVER_TARGET", "go")


@pytest.fixture(scope="session")
def actor() -> str:
    return ACTOR


@pytest.fixture(scope="module")
def tenant_id() -> str:
    """The pre-seeded e2e tenant. Stable for the module — most tests
    don't need a private tenant and seeding-on-boot already covers
    the Go target. The Python server auto-creates tenants on first
    write."""
    return TENANT


# ---------------------------------------------------------------------------
# SDK client (session-scoped, reused across tests)
# ---------------------------------------------------------------------------


def _wait_for_grpc(host: str, port: int, timeout: int = 60) -> None:
    """Poll TCP until the server's listener is up. The SDK's own
    retry loop covers the gRPC handshake; we just want a fast-fail
    when the container is missing entirely."""
    import socket

    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            with socket.create_connection((host, port), timeout=1):
                return
        except (TimeoutError, OSError):
            time.sleep(1)
    raise RuntimeError(f"gRPC server at {host}:{port} not reachable after {timeout}s")


_SCHEMA_REGISTERED = False
_BOOT_WAITED = False


@pytest_asyncio.fixture
async def db_client(grpc_target: str) -> AsyncGenerator[DbClient, None]:
    """Function-scoped DbClient with the e2e schema registered.

    Mirrors the connect-with-retry loop in ``test_e2e.py`` so a slow
    server boot (Go target, fresh image build) doesn't flake the run.
    The schema registration + TCP-ready wait happen at most once per
    pytest session (cached via module-level flags) so the per-test
    cost is just a fresh ``grpc.aio.insecure_channel``.
    """
    global _SCHEMA_REGISTERED, _BOOT_WAITED
    if not _SCHEMA_REGISTERED:
        register_proto_schema(pb)
        _SCHEMA_REGISTERED = True
    if not _BOOT_WAITED:
        _wait_for_grpc(HOST, PORT, timeout=60)
        _BOOT_WAITED = True

    client_kwargs = {}
    if TLS_CA:
        import grpc

        with open(TLS_CA, "rb") as f:
            client_kwargs = {
                "secure": True,
                "credentials": grpc.ssl_channel_credentials(root_certificates=f.read()),
            }

    client = DbClient(grpc_target, **client_kwargs)
    last_exc: Exception | None = None
    for _ in range(30):
        try:
            await client.connect()
            break
        except Exception as exc:  # noqa: BLE001
            last_exc = exc
            await asyncio.sleep(2)
    else:
        raise RuntimeError(f"could not connect to {grpc_target}: {last_exc!r}")

    try:
        yield client
    finally:
        await client.close()


@pytest.fixture
def scope(db_client: DbClient, tenant_id: str, actor: str):
    """ActorScope bound to (tenant_id, actor) for the test."""
    return db_client.tenant(tenant_id).actor(actor)


# ---------------------------------------------------------------------------
# Fresh tenant (for isolation tests)
# ---------------------------------------------------------------------------


@pytest_asyncio.fixture
async def fresh_tenant(db_client: DbClient, server_target: str) -> str:
    """Return a brand-new tenant id.

    The Python server runs in dev-mode with the global store disabled
    (``Global store disabled (auth+quotas off)`` at boot) so CreateTenant
    is rejected and tenants are auto-created on first write — just
    minting a fresh id is enough. The Go server runs with the global
    store enabled, so we explicitly CreateTenant + AddTenantMember as
    a trusted system actor.
    """
    import uuid

    tid = f"e2e-iso-{uuid.uuid4().hex[:8]}"

    if server_target == "go":
        create_result = await db_client.create_tenant(
            tenant_id=tid,
            name=f"E2E Iso {tid}",
            actor=ADMIN_ACTOR,
        )
        if not create_result.get("success"):
            raise RuntimeError(f"CreateTenant({tid}) failed: {create_result.get('error')}")
        await db_client.add_tenant_member(
            tenant_id=tid,
            user_id="e2e-runner",
            role="owner",
            actor=ADMIN_ACTOR,
        )

    return tid


# ---------------------------------------------------------------------------
# Crash-recovery helper
# ---------------------------------------------------------------------------


@pytest.fixture
def restart_server(
    server_target: str,
) -> Callable[[], Awaitable[None]]:
    """Return an async callable that restarts the server container
    and waits for its gRPC port to come back.

    The compose file is bind-mounted into the e2e-tests container at
    ``/compose/docker-compose.yml`` (see the compose files), and the
    docker CLI is available because the Dockerfile installs it and
    the host's docker socket is mounted at ``/var/run/docker.sock``.
    """

    async def _restart() -> None:
        if not os.path.exists("/var/run/docker.sock"):
            pytest.skip(
                "restart_server requires /var/run/docker.sock — "
                "this path is only mounted when run via run-e2e.sh"
            )
        if not os.path.exists(COMPOSE_FILE_IN_CONTAINER):
            pytest.skip(
                f"restart_server requires {COMPOSE_FILE_IN_CONTAINER} — "
                "compose file bind-mount is missing"
            )

        # Run synchronously in a thread to avoid blocking the event loop.
        def _cmd() -> None:
            subprocess.run(
                [
                    "docker",
                    "compose",
                    "-f",
                    COMPOSE_FILE_IN_CONTAINER,
                    "restart",
                    "server",
                ],
                check=True,
                capture_output=True,
            )

        await asyncio.to_thread(_cmd)

        # Wait for the listener to come back. The Go server starts
        # immediately; the Python server takes longer because the
        # Kafka consumer has to rejoin its group and replay.
        await asyncio.to_thread(_wait_for_grpc, HOST, PORT, 60)
        # Give the applier a beat to materialise any unwritten WAL
        # entries before the caller asserts on read state.
        await asyncio.sleep(5 if server_target == "python" else 2)

    return _restart


# ---------------------------------------------------------------------------
# Back-compat: keep the gRPC channel name some legacy tests still ref.
# ---------------------------------------------------------------------------


@pytest.fixture
def grpc_channel(grpc_target: str) -> Generator[object, None, None]:
    """Raw gRPC channel for low-level tests (none today). Kept for
    parity with the old fixture name; prefer ``db_client`` / ``scope``."""
    try:
        import grpc
    except ImportError:
        pytest.skip("grpcio not installed")

    if TLS_CA:
        with open(TLS_CA, "rb") as f:
            channel = grpc.secure_channel(
                grpc_target,
                grpc.ssl_channel_credentials(root_certificates=f.read()),
            )
    else:
        channel = grpc.insecure_channel(grpc_target)
    try:
        yield channel
    finally:
        channel.close()
