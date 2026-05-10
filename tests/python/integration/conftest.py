# SPDX-License-Identifier: AGPL-3.0-only
"""Cross-implementation test harness for the EntDB integration suite.

This conftest is the single switch-point for running the same Python
contract tests against either the in-process Python ``EntDBServicer``
(default) or the Go ``entdb-server`` subprocess (Wave 2+ of EPIC #407).

Selection is via the ``ENTDB_SERVER_TARGET`` env var:

* ``python`` (default) — today's path. Each test that needs a gRPC
  endpoint pulls :func:`live_server`, which constructs the in-process
  servicer + applier + in-memory WAL, seeds a tenant and one node,
  and yields the bound port.
* ``go`` — builds (or reuses) ``server/go/cmd/entdb-server``, launches
  it as a subprocess on a free port with the test-only flags
  documented in ``docs/go-port/shared/test-harness.md``, polls the
  ``Health`` RPC until the server answers, and yields the bound port.
  Per-RPC gating is enforced by :func:`pytest_collection_modifyitems`:
  contract cases whose RPC isn't in
  :data:`tests.python.integration._go_parity.GO_IMPLEMENTED` are
  skipped, not failed. Wave 2 PRs flip RPCs from skipped to runnable
  one at a time.

The fixture interface is identical across targets: tests receive an
``int`` port number, connect to ``127.0.0.1:<port>``, and assert wire
behaviour. No test body changes when adding the Go target.
"""

from __future__ import annotations

import asyncio
import hashlib
import logging
import os
import socket
import subprocess
import tempfile
import time
from collections.abc import AsyncIterator
from pathlib import Path

import grpc
import pytest
from grpc import aio as grpc_aio

from tests.python.integration._go_parity import GO_IMPLEMENTED

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Target selection
# ---------------------------------------------------------------------------

_TARGET_ENV = "ENTDB_SERVER_TARGET"
_BINARY_ENV = "ENTDB_GO_BINARY"
_VALID_TARGETS = ("python", "go")


def _server_target() -> str:
    target = os.environ.get(_TARGET_ENV, "python").lower()
    if target not in _VALID_TARGETS:
        raise RuntimeError(
            f"{_TARGET_ENV}={target!r} is not one of {_VALID_TARGETS}; "
            f"set it to 'python' (default) or 'go'."
        )
    return target


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
    """Hash the Go sources so cached builds are reused only when valid.

    Walks ``server/go/**/*.go`` plus ``go.sum``; SHA-256 of the
    sorted (path, content) pairs is stable across runs and changes the
    moment any source bytes do.
    """
    h = hashlib.sha256()
    paths: list[Path] = []
    for ext in ("*.go", "go.sum", "go.mod"):
        paths.extend(server_go_dir.rglob(ext))
    for p in sorted(paths):
        # Skip generated test binaries / build outputs that may live
        # under server/go/ at dev time.
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
    Today's Wave-1 Go server returns ``UNIMPLEMENTED`` for every RPC
    including ``Health``; the subprocess is "up" as soon as it answers
    the wire. Later waves will return ``OK`` from ``Health`` and the
    same loop still works.
    """
    # Local import: tests/python/integration/conftest.py is collected
    # before entdb_server is necessarily importable for non-Python
    # targets, but pyproject.toml's pythonpath always exposes it.
    from entdb_server.api.generated import EntDBServiceStub
    from entdb_server.api.generated import entdb_pb2 as pb

    deadline = time.monotonic() + timeout
    last_err: Exception | None = None
    while time.monotonic() < deadline:
        try:
            async with grpc_aio.insecure_channel(addr) as channel:
                stub = EntDBServiceStub(channel)
                await stub.Health(pb.HealthRequest(), timeout=1.0)
                return
        except grpc.aio.AioRpcError as exc:
            # UNIMPLEMENTED means the server is up and routing.
            if exc.code() == grpc.StatusCode.UNIMPLEMENTED:
                return
            last_err = exc
        except Exception as exc:  # pragma: no cover — transient connect errors
            last_err = exc
        await asyncio.sleep(0.05)
    raise RuntimeError(
        f"gRPC server at {addr} did not become ready within {timeout}s (last error: {last_err!r})"
    )


# ---------------------------------------------------------------------------
# Python in-process server (today's path, extracted from test_grpc_contract.py)
# ---------------------------------------------------------------------------


TENANT = "acme"
ALICE = "user:alice"
SEED_NODE_ID = "seeded-node"


def _build_python_registry():
    from entdb_server.schema.registry import SchemaRegistry
    from entdb_server.schema.types import EdgeTypeDef, NodeTypeDef, field

    reg = SchemaRegistry()
    reg.register_node_type(
        NodeTypeDef(
            type_id=1,
            name="User",
            fields=(field(1, "email", "str"), field(2, "name", "str")),
        )
    )
    reg.register_node_type(
        NodeTypeDef(
            type_id=2,
            name="Task",
            fields=(field(1, "title", "str"), field(2, "description", "str")),
        )
    )
    reg.register_edge_type(EdgeTypeDef(edge_id=100, name="AssignedTo", from_type=2, to_type=1))
    return reg


async def _seed_node(addr: str) -> None:
    """Create the contract suite's seed node via ExecuteAtomic.

    Mirrors the inline seeding that used to live in
    ``test_grpc_contract.py::live_server``. Moving it here keeps every
    target on one code path.
    """
    from google.protobuf.struct_pb2 import Struct

    from entdb_server.api.generated import EntDBServiceStub
    from entdb_server.api.generated import entdb_pb2 as pb

    seed_data = Struct()
    seed_data.update({"1": "seeded@example.com", "2": "Seeded"})
    async with grpc_aio.insecure_channel(addr) as channel:
        stub = EntDBServiceStub(channel)
        req = pb.ExecuteAtomicRequest(
            context=pb.RequestContext(tenant_id=TENANT, actor=ALICE),
            idempotency_key="seed-1",
            operations=[
                pb.Operation(
                    create_node=pb.CreateNodeOp(
                        type_id=1,
                        id=SEED_NODE_ID,
                        data=seed_data,
                    )
                )
            ],
            wait_applied=True,
            wait_timeout_ms=2000,
        )
        seed_resp = await stub.ExecuteAtomic(req)
        assert seed_resp.success, f"seed failed: {seed_resp.error}"


async def _start_python_server() -> AsyncIterator[int]:
    """In-process Python harness — yields the bound port.

    Lifted verbatim from ``test_grpc_contract.py::live_server`` so the
    contract suite keeps the same setup it has today.
    """
    from entdb_server.api.generated import add_EntDBServiceServicer_to_server
    from entdb_server.api.grpc_server import EntDBServicer
    from entdb_server.apply.applier import Applier, MailboxFanoutConfig
    from entdb_server.apply.canonical_store import CanonicalStore
    from entdb_server.global_store import GlobalStore
    from entdb_server.schema import registry as registry_mod
    from entdb_server.wal.memory import InMemoryWalStream

    prev = registry_mod._global_registry
    reg = _build_python_registry()
    registry_mod._global_registry = reg

    with tempfile.TemporaryDirectory() as tmpdir:
        global_store = GlobalStore(tmpdir)
        await global_store.create_tenant(TENANT, "Acme Corp")
        await global_store.create_user("alice", "alice@example.com", "Alice")
        await global_store.create_user("bob", "bob@example.com", "Bob")
        await global_store.add_member(TENANT, "alice", role="owner")
        await global_store.add_member(TENANT, "bob", role="member")

        canonical = CanonicalStore(data_dir=tmpdir, wal_mode=False)
        await canonical.initialize_tenant(TENANT)

        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        applier = Applier(
            wal=wal,
            canonical_store=canonical,
            topic="entdb-wal",
            group_id="contract-applier",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=1,
        )
        applier_task = asyncio.create_task(applier.start())

        servicer = EntDBServicer(
            wal=wal,
            canonical_store=canonical,
            schema_registry=reg,
            topic="entdb-wal",
            global_store=global_store,
        )

        port = _free_port()
        server = grpc_aio.server()
        add_EntDBServiceServicer_to_server(servicer, server)
        server.add_insecure_port(f"127.0.0.1:{port}")
        await server.start()

        try:
            await _seed_node(f"127.0.0.1:{port}")
            yield port
        finally:
            await server.stop(grace=0)
            await applier.stop()
            applier_task.cancel()
            try:
                await asyncio.wait_for(applier_task, timeout=2.0)
            except (asyncio.CancelledError, asyncio.TimeoutError):
                pass
            try:
                await wal.close()
            except Exception:
                pass
            global_store.close()
            registry_mod._global_registry = prev


async def _start_go_server(tmp_path_factory) -> AsyncIterator[int]:
    """Go subprocess harness — yields the bound port.

    Builds (or reuses) the Go binary, picks a free port, launches with
    the test-only flag set from ``docs/go-port/shared/test-harness.md``,
    polls until ready, then terminates on teardown.
    """
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
    """Yield a bound ``int`` port for a live EntDBService endpoint.

    Switches implementation based on :data:`ENTDB_SERVER_TARGET`.
    Tests don't see the difference — they just connect to
    ``127.0.0.1:<port>``.
    """
    target = _server_target()
    agen = _start_python_server() if target == "python" else _start_go_server(tmp_path_factory)
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
    """``"127.0.0.1:<port>"`` form of :func:`live_server`.

    Provided as a convenience for tests that want to construct a
    ``grpc.aio.insecure_channel`` without re-formatting the port.
    """
    return f"127.0.0.1:{live_server}"


# ---------------------------------------------------------------------------
# Per-RPC gating against the Go target
# ---------------------------------------------------------------------------


def _rpc_name_from_param_id(item: pytest.Item) -> str | None:
    """Extract the RPC name from a contract-suite param-id.

    ``test_grpc_contract.py`` parametrises over ``CONTRACT_CASES`` with
    ``ids=[f"{c['rpc']}-{c['mode']}"]``; the RPC name is the prefix
    before the first ``-``. Returns ``None`` for tests that aren't
    contract cases.
    """
    callspec = getattr(item, "callspec", None)
    if callspec is None:
        return None
    case = callspec.params.get("case")
    if not isinstance(case, dict):
        return None
    rpc = case.get("rpc")
    if not isinstance(rpc, str):
        return None
    return rpc


def pytest_collection_modifyitems(config: pytest.Config, items: list[pytest.Item]) -> None:
    """Skip tests that can't run against the configured target.

    Behaviour:

    * Target ``python`` — no-op. Every existing test runs.
    * Target ``go`` — contract cases whose RPC isn't in
      :data:`GO_IMPLEMENTED` are skipped with a precise reason. Tests
      that don't have a contract-case param (i.e. Python-only suites
      like ``test_wal_replay_determinism``, ``test_privilege_escalation``,
      and any helper tests) are also skipped, because they exercise
      Python internals and aren't part of the cross-impl contract.
    """
    target = _server_target()
    if target != "go":
        return

    skip_unimpl = pytest.mark.skip(
        reason="ENTDB_SERVER_TARGET=go: RPC not yet implemented in the Go port"
    )
    skip_python_only = pytest.mark.skip(
        reason="ENTDB_SERVER_TARGET=go: Python-only test (no cross-impl contract)"
    )

    for item in items:
        rpc = _rpc_name_from_param_id(item)
        if rpc is None:
            item.add_marker(skip_python_only)
            continue
        if rpc not in GO_IMPLEMENTED:
            item.add_marker(skip_unimpl)
