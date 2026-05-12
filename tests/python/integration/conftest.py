# SPDX-License-Identifier: AGPL-3.0-only
"""Integration test harness — Go server subprocess.

Builds (or reuses) ``server/go/cmd/entdb-server``, launches it as a
subprocess on a free port with the test-only flags documented in
``docs/go-port/shared/test-harness.md``, polls the ``Health`` RPC until
the server answers, and yields the bound port.

(Historical: this conftest used to switch between an in-process Python
``EntDBServicer`` and the Go subprocess via ``ENTDB_SERVER_TARGET``. The
Python server was retired in Phase 4D of EPIC #407, so only the Go path
remains.)
"""

from __future__ import annotations

import asyncio
import hashlib
import logging
import os
import socket
import subprocess
import time
from collections.abc import AsyncIterator
from pathlib import Path

import grpc
import pytest
from grpc import aio as grpc_aio

logger = logging.getLogger(__name__)


_BINARY_ENV = "ENTDB_GO_BINARY"


def _free_port() -> int:
    """Bind a TCP socket to port 0 and immediately release it.

    There's an inherent TOCTOU window between close() and the
    consuming process' listen(); acceptable for serial CI. Under
    pytest-xdist this can flake — mitigation lives in the Go-subprocess
    spin-up (3-attempt retry on EADDRINUSE).
    """
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


def _repo_root() -> Path:
    # tests/python/integration/conftest.py -> repo root is three levels up.
    return Path(__file__).resolve().parents[3]


# ---------------------------------------------------------------------------
# Go binary build + subprocess management
# ---------------------------------------------------------------------------


def _hash_go_sources(server_go_dir: Path) -> str:
    """Hash the Go sources so cached builds are reused only when valid."""
    h = hashlib.sha256()
    paths: list[Path] = []
    for ext in ("*.go", "go.sum", "go.mod"):
        paths.extend(server_go_dir.rglob(ext))
    for p in sorted(paths):
        if any(part.startswith(".") for part in p.relative_to(server_go_dir).parts):
            continue
        h.update(str(p.relative_to(server_go_dir)).encode())
        h.update(b"\0")
        h.update(p.read_bytes())
        h.update(b"\0")
    return h.hexdigest()[:16]


def _build_go_binary() -> Path:
    """Build ``server/go/cmd/entdb-server`` and return the path.

    Honours ``$ENTDB_GO_BINARY`` if set (CI exports it after building
    once outside pytest); otherwise compiles to ``./.go-bin/entdb-server-<sha>``
    and reuses the cached artefact across runs in the same checkout.
    """
    override = os.environ.get(_BINARY_ENV)
    if override:
        p = Path(override)
        if not p.is_file():
            raise RuntimeError(
                f"{_BINARY_ENV}={override!r} does not point at a file; "
                f"build the binary or unset the env var to let pytest build it."
            )
        return p

    repo_root = _repo_root()
    server_go_dir = repo_root / "server" / "go"
    if not server_go_dir.is_dir():
        raise RuntimeError(
            f"server/go directory not found at {server_go_dir}; cannot build Go test binary."
        )

    sha = _hash_go_sources(server_go_dir)
    bin_dir = repo_root / ".go-bin"
    bin_dir.mkdir(exist_ok=True)
    bin_path = bin_dir / f"entdb-server-{sha}"
    if bin_path.is_file():
        return bin_path

    logger.info("conftest: building Go entdb-server (sha=%s)", sha)
    cmd = [
        "go",
        "build",
        "-o",
        str(bin_path),
        "./cmd/entdb-server",
    ]
    proc = subprocess.run(
        cmd,
        cwd=server_go_dir,
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        raise RuntimeError(
            f"go build failed (rc={proc.returncode}):\n"
            f"stdout:\n{proc.stdout}\nstderr:\n{proc.stderr}"
        )
    return bin_path


async def _wait_for_grpc_ready(addr: str, timeout: float = 10.0) -> None:
    """Poll the gRPC endpoint until it accepts connections.

    Any RPC response — including ``UNIMPLEMENTED`` — counts as ready.
    """
    from entdb_sdk._generated import entdb_pb2 as pb
    from entdb_sdk._generated.entdb_pb2_grpc import EntDBServiceStub

    deadline = time.monotonic() + timeout
    last_err: Exception | None = None
    while time.monotonic() < deadline:
        try:
            async with grpc_aio.insecure_channel(addr) as channel:
                stub = EntDBServiceStub(channel)
                await stub.Health(pb.HealthRequest(), timeout=1.0)
                return
        except grpc.aio.AioRpcError as exc:
            if exc.code() == grpc.StatusCode.UNIMPLEMENTED:
                return
            last_err = exc
        except Exception as exc:  # pragma: no cover — transient connect errors
            last_err = exc
        await asyncio.sleep(0.05)
    raise RuntimeError(
        f"gRPC server at {addr} did not become ready within {timeout}s (last error: {last_err!r})"
    )


TENANT = "acme"
ALICE = "user:alice"
SEED_NODE_ID = "seeded-node"


async def _start_go_server(tmp_path_factory) -> AsyncIterator[int]:
    """Go subprocess harness — yields the bound port."""
    binary = _build_go_binary()
    data_dir = tmp_path_factory.mktemp("entdb-go-data")
    log_path = tmp_path_factory.mktemp("entdb-go-logs") / "server.log"

    last_err: Exception | None = None
    proc: subprocess.Popen | None = None
    port: int | None = None

    # Three-attempt retry to absorb the TOCTOU window between
    # _free_port() and the subprocess' Listen().
    for _attempt in range(3):
        port = _free_port()
        cmd = [
            str(binary),
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
        ]
        log_fh = open(log_path, "ab", buffering=0)  # noqa: SIM115 - lifetime tied to subprocess
        proc = subprocess.Popen(
            cmd,
            stdout=log_fh,
            stderr=subprocess.STDOUT,
            env={**os.environ, "ENTDB_LOG_LEVEL": "info"},
        )
        try:
            await _wait_for_grpc_ready(f"127.0.0.1:{port}", timeout=10.0)
            break
        except Exception as exc:
            last_err = exc
            proc.terminate()
            try:
                proc.wait(timeout=2.0)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=2.0)
            proc = None
            continue
    if proc is None or port is None:
        raise RuntimeError(
            f"failed to start Go entdb-server after 3 attempts; "
            f"last error: {last_err!r}; logs at {log_path}"
        )

    try:
        yield port
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5.0)
        except subprocess.TimeoutExpired:
            proc.kill()
            try:
                proc.wait(timeout=2.0)
            except subprocess.TimeoutExpired:
                pass


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture
async def live_server(tmp_path_factory):
    """Yield a bound ``int`` port for a live EntDBService endpoint."""
    agen = _start_go_server(tmp_path_factory)
    port = await agen.__anext__()
    try:
        yield port
    finally:
        try:
            await agen.__anext__()
        except StopAsyncIteration:
            pass


@pytest.fixture
def grpc_endpoint(live_server) -> str:
    """``"127.0.0.1:<port>"`` form of :func:`live_server`."""
    return f"127.0.0.1:{live_server}"
