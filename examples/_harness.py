# SPDX-License-Identifier: AGPL-3.0-only
"""Shared harness for the runnable docs examples.

Every ``examples/*.py`` is BOTH:

  * a standalone script — ``python examples/quickstart.py`` boots a
    throwaway Go ``entdb-server`` and runs the example against it; and
  * a pytest case — ``pytest examples/`` imports ``main`` and asserts
    it completes without raising.

This mirrors the integration conftest (it boots the same
``server/go/cmd/entdb-server`` binary with the ``contract`` seed
profile) but is self-contained so the examples stay runnable outside
the test tree — that is the whole point of "tested examples": the
snippet a reader copies is the snippet CI ran.

The ``contract`` seed profile registers exactly:

  User       type_id 1   (email=1, name=2)
  Task       type_id 2   (title=1, description=2)
  AssignedTo edge_id 100  Task -> User

and seeds tenant ``acme`` with users ``alice`` (owner) / ``bob``
(member). ``examples/example_schema.proto`` mirrors those ids so the
high-level proto-message SDK API works against this server.
"""

from __future__ import annotations

import asyncio
import os
import socket
import subprocess
import sys
import tempfile
import time
from collections.abc import Awaitable, Callable
from contextlib import asynccontextmanager
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
EXAMPLES_DIR = REPO_ROOT / "examples"

# Make ``entdb_sdk`` and the example schema importable when run as a
# bare script (pytest gets these from pyproject pythonpath + this dir).
for extra in (str(REPO_ROOT / "sdk" / "python"), str(EXAMPLES_DIR)):
    if extra not in sys.path:
        sys.path.insert(0, extra)

TENANT = "acme"
ALICE = "user:alice"
BOB = "user:bob"


def _free_port() -> int:
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


def _server_binary() -> str:
    """Prefer ``$ENTDB_GO_BINARY`` (CI builds it once); else build."""
    override = os.environ.get("ENTDB_GO_BINARY")
    if override:
        if not Path(override).is_file():
            raise RuntimeError(
                f"ENTDB_GO_BINARY={override!r} is not a file; build the "
                "server or unset the env var."
            )
        return override

    server_go = REPO_ROOT / "server" / "go"
    if not server_go.is_dir():
        raise RuntimeError(f"server/go not found at {server_go}")
    out = Path(tempfile.gettempdir()) / "entdb-server-examples"
    proc = subprocess.run(
        ["go", "build", "-o", str(out), "./cmd/entdb-server"],
        cwd=server_go,
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        raise RuntimeError(f"go build failed (rc={proc.returncode}):\n{proc.stderr}")
    return str(out)


async def _wait_ready(addr: str, timeout: float = 15.0) -> None:
    import grpc
    from grpc import aio as grpc_aio

    from entdb_sdk._generated import entdb_pb2 as pb
    from entdb_sdk._generated.entdb_pb2_grpc import EntDBServiceStub

    deadline = time.monotonic() + timeout
    last: Exception | None = None
    while time.monotonic() < deadline:
        try:
            async with grpc_aio.insecure_channel(addr) as ch:
                await EntDBServiceStub(ch).Health(pb.HealthRequest(), timeout=1.0)
                return
        except grpc.aio.AioRpcError as exc:
            if exc.code() == grpc.StatusCode.UNIMPLEMENTED:
                return
            last = exc
        except Exception as exc:  # transient connect race
            last = exc
        await asyncio.sleep(0.05)
    raise RuntimeError(f"server at {addr} not ready in {timeout}s: {last!r}")


@asynccontextmanager
async def live_endpoint():
    """Yield ``"127.0.0.1:<port>"`` of a fresh contract-seeded server."""
    binary = _server_binary()
    tmp = tempfile.TemporaryDirectory(prefix="entdb-examples-")
    data_dir = Path(tmp.name)
    log_path = data_dir / "server.log"

    proc: subprocess.Popen | None = None
    port: int | None = None
    last: Exception | None = None
    for _ in range(3):
        port = _free_port()
        log_fh = open(log_path, "ab", buffering=0)  # noqa: SIM115
        proc = subprocess.Popen(
            [
                binary,
                "--addr",
                f"127.0.0.1:{port}",
                "--data-dir",
                str(data_dir),
                "--wal-backend",
                "memory",
                "--seed-profile",
                "contract",
                "--seed-tenant",
                TENANT,
            ],
            stdout=log_fh,
            stderr=subprocess.STDOUT,
            env={**os.environ, "ENTDB_LOG_LEVEL": "info"},
        )
        try:
            await _wait_ready(f"127.0.0.1:{port}")
            break
        except Exception as exc:
            last = exc
            proc.terminate()
            try:
                proc.wait(timeout=2.0)
            except subprocess.TimeoutExpired:
                proc.kill()
            proc = None
    if proc is None or port is None:
        tmp.cleanup()
        raise RuntimeError(f"failed to boot entdb-server: {last!r}")

    try:
        yield f"127.0.0.1:{port}"
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5.0)
        except subprocess.TimeoutExpired:
            proc.kill()
        tmp.cleanup()


def reset_sdk_registry() -> None:
    """Clear the process-wide SDK schema registry.

    ``register_proto_schema`` is not idempotent — a second call for the
    same ``type_id`` raises. Standalone runs get a fresh process so
    this is a no-op there; it matters when several examples share one
    Python process (the pytest collector) or a REPL re-runs one.
    """
    from entdb_sdk.registry import reset_registry

    reset_registry()


def run_example(main: Callable[[str], Awaitable[None]]) -> None:
    """Boot a server, run ``main(endpoint)``, tear down. For ``__main__``."""

    async def _drive() -> None:
        reset_sdk_registry()
        async with live_endpoint() as endpoint:
            await main(endpoint)

    asyncio.run(_drive())
    print("example completed OK")
