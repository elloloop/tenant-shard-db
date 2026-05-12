# SPDX-License-Identifier: AGPL-3.0-only
"""
Benchmark configuration and system spec capture.

Automatically collects and prints machine specs at the start of any
benchmark run so results are reproducible and comparable.

Also wires the ``ENTDB_SERVER_TARGET`` env var into the benchmark
suite (Phase 4B of EPIC #407): when set to ``go`` the dual-backend
:func:`live_server` fixture launches the Go ``entdb-server`` subprocess
and benchmarks measure the wire path against it; when unset/``python``
the in-process Python servicer is used. Python-internal benchmarks
(those that hit ``CanonicalStore`` / ``Applier`` Python objects
directly, not the gRPC wire) are auto-skipped when target=go since
their implementation does not exist on the Go side.
"""

from __future__ import annotations

import os
import platform
import subprocess

import pytest


def _get_cpu_info() -> dict:
    """Collect CPU information."""
    info = {
        "processor": platform.processor() or "unknown",
        "arch": platform.machine(),
    }

    system = platform.system()
    if system == "Darwin":
        try:
            brand = subprocess.check_output(
                ["sysctl", "-n", "machdep.cpu.brand_string"], text=True
            ).strip()
            cores_physical = subprocess.check_output(
                ["sysctl", "-n", "hw.physicalcpu"], text=True
            ).strip()
            cores_logical = subprocess.check_output(
                ["sysctl", "-n", "hw.logicalcpu"], text=True
            ).strip()
            info["brand"] = brand
            info["cores_physical"] = int(cores_physical)
            info["cores_logical"] = int(cores_logical)
        except (subprocess.CalledProcessError, FileNotFoundError):
            pass
    elif system == "Linux":
        try:
            with open("/proc/cpuinfo") as f:
                for line in f:
                    if line.startswith("model name"):
                        info["brand"] = line.split(":")[1].strip()
                        break
            cores = os.cpu_count()
            info["cores_logical"] = cores or 0
        except FileNotFoundError:
            pass

    return info


def _get_memory_info() -> dict:
    """Collect memory information."""
    info = {}
    system = platform.system()

    if system == "Darwin":
        try:
            mem_bytes = subprocess.check_output(["sysctl", "-n", "hw.memsize"], text=True).strip()
            info["total_gb"] = round(int(mem_bytes) / (1024**3), 1)
        except (subprocess.CalledProcessError, FileNotFoundError):
            pass
    elif system == "Linux":
        try:
            with open("/proc/meminfo") as f:
                for line in f:
                    if line.startswith("MemTotal"):
                        kb = int(line.split()[1])
                        info["total_gb"] = round(kb / (1024**2), 1)
                        break
        except FileNotFoundError:
            pass

    return info


def _get_disk_info() -> dict:
    """Collect disk type information."""
    info = {}
    system = platform.system()

    if system == "Darwin":
        try:
            # Check if running on SSD
            output = subprocess.check_output(
                ["system_profiler", "SPNVMeDataType", "-detailLevel", "mini"],
                text=True,
                timeout=5,
            )
            if "NVMe" in output or "Apple" in output:
                info["type"] = "NVMe SSD"
            else:
                info["type"] = "SSD"
        except (subprocess.CalledProcessError, FileNotFoundError, subprocess.TimeoutExpired):
            info["type"] = "unknown"
    elif system == "Linux":
        try:
            # Check /sys/block for rotation
            for dev in os.listdir("/sys/block"):
                rotational_path = f"/sys/block/{dev}/queue/rotational"
                if os.path.exists(rotational_path):
                    with open(rotational_path) as f:
                        is_rotational = f.read().strip() == "1"
                    info["type"] = "HDD" if is_rotational else "SSD"
                    break
        except (OSError, FileNotFoundError):
            info["type"] = "unknown"

    return info


def _get_sqlite_version() -> str:
    """Get SQLite version."""
    import sqlite3

    return sqlite3.sqlite_version


def _collect_specs() -> dict:
    """Collect all system specs."""
    return {
        "os": f"{platform.system()} {platform.release()}",
        "os_version": platform.version(),
        "python": platform.python_version(),
        "sqlite": _get_sqlite_version(),
        "cpu": _get_cpu_info(),
        "memory": _get_memory_info(),
        "disk": _get_disk_info(),
    }


def _format_specs(specs: dict) -> str:
    """Format specs as a readable string."""
    cpu = specs["cpu"]
    mem = specs["memory"]
    disk = specs["disk"]

    lines = [
        "",
        "=" * 70,
        "BENCHMARK SYSTEM SPECS",
        "=" * 70,
        f"  OS:       {specs['os']}",
        f"  Python:   {specs['python']}",
        f"  SQLite:   {specs['sqlite']}",
    ]

    if "brand" in cpu:
        lines.append(f"  CPU:      {cpu['brand']}")
    if "cores_physical" in cpu:
        lines.append(
            f"  Cores:    {cpu['cores_physical']} physical / {cpu['cores_logical']} logical"
        )
    elif "cores_logical" in cpu:
        lines.append(f"  Cores:    {cpu['cores_logical']} logical")

    if "total_gb" in mem:
        lines.append(f"  Memory:   {mem['total_gb']} GB")

    if "type" in disk:
        lines.append(f"  Disk:     {disk['type']}")

    lines.append("=" * 70)
    lines.append("")

    return "\n".join(lines)


@pytest.fixture(scope="session", autouse=True)
def print_system_specs():
    """Print system specs once at the start of the benchmark session."""
    specs = _collect_specs()
    print(_format_specs(specs))
    yield specs


def pytest_benchmark_update_machine_info(config, machine_info):
    """Hook: inject system specs into benchmark JSON output."""
    specs = _collect_specs()
    machine_info["system"] = specs["os"]
    machine_info["python"] = specs["python"]
    machine_info["sqlite"] = specs["sqlite"]

    cpu = specs["cpu"]
    if "brand" in cpu:
        machine_info["cpu_brand"] = cpu["brand"]
    if "cores_physical" in cpu:
        machine_info["cpu_cores_physical"] = cpu["cores_physical"]
        machine_info["cpu_cores_logical"] = cpu["cores_logical"]

    mem = specs["memory"]
    if "total_gb" in mem:
        machine_info["memory_gb"] = mem["total_gb"]

    disk = specs["disk"]
    if "type" in disk:
        machine_info["disk_type"] = disk["type"]

    # Record which server backend the benchmark targeted. This is what
    # makes Python-vs-Go comparisons in .benchmarks/{python,go}/ JSON
    # files self-describing.
    machine_info["entdb_server_target"] = os.environ.get("ENTDB_SERVER_TARGET", "python").lower()


# ---------------------------------------------------------------------------
# Dual-backend live_server fixture (Phase 4B of EPIC #407)
# ---------------------------------------------------------------------------
#
# The pytest-benchmark suite has historically exercised Python internals
# (``CanonicalStore``, ``Applier``, etc.) directly. For Phase 4B we add
# a ``live_server`` fixture so wire-level benchmarks
# (``bench_grpc_dual.py``) can run against either the in-process Python
# servicer or the Go ``entdb-server`` subprocess.
#
# The Python server is hosted on a **background thread** with its own
# asyncio event loop so the test body can stay synchronous and use a
# blocking gRPC client. (pytest-benchmark calls a sync callable; mixing
# a sync client with an in-thread async server deadlocks.) The Go
# backend is a real subprocess; we delegate to the integration conftest
# for the build + spawn logic since that's the canonical implementation.


def _entdb_server_target() -> str:
    target = os.environ.get("ENTDB_SERVER_TARGET", "python").lower()
    if target not in ("python", "go"):
        raise RuntimeError(f"ENTDB_SERVER_TARGET={target!r} is not one of ('python', 'go').")
    return target


def _start_python_server_on_thread() -> tuple[int, _PythonServerHandle]:
    """Start the in-process Python gRPC server on a background thread.

    Returns ``(port, handle)``; call ``handle.stop()`` to tear down.
    The server runs its own event loop on the thread so the test
    main-thread can issue blocking gRPC calls without deadlocking.
    """
    import asyncio as _asyncio
    import tempfile
    import threading

    from grpc import aio as grpc_aio

    from entdb_server.api.generated import add_EntDBServiceServicer_to_server
    from entdb_server.api.grpc_server import EntDBServicer
    from entdb_server.apply.applier import Applier, MailboxFanoutConfig
    from entdb_server.apply.canonical_store import CanonicalStore
    from entdb_server.global_store import GlobalStore
    from entdb_server.schema import registry as registry_mod
    from entdb_server.schema.types import EdgeTypeDef, NodeTypeDef, field
    from entdb_server.wal.memory import InMemoryWalStream
    from tests.python.integration.conftest import (
        ALICE,
        SEED_NODE_ID,
        TENANT,
        _free_port,
    )

    # Build schema (mirrors integration conftest's _build_python_registry).
    reg = registry_mod.SchemaRegistry()
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

    prev_reg = registry_mod._global_registry
    registry_mod._global_registry = reg

    tmpdir_obj = tempfile.TemporaryDirectory()
    tmpdir = tmpdir_obj.name
    port = _free_port()

    ready = threading.Event()
    stopped = threading.Event()
    state: dict = {}

    def _serve() -> None:
        loop = _asyncio.new_event_loop()
        _asyncio.set_event_loop(loop)

        async def _run() -> None:
            from google.protobuf.struct_pb2 import Struct

            from entdb_server.api.generated import EntDBServiceStub
            from entdb_server.api.generated import entdb_pb2 as pb

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
                group_id="bench-applier",
                fanout_config=MailboxFanoutConfig(enabled=False),
                batch_size=1,
            )
            applier_task = _asyncio.create_task(applier.start())

            servicer = EntDBServicer(
                wal=wal,
                canonical_store=canonical,
                schema_registry=reg,
                topic="entdb-wal",
                global_store=global_store,
            )

            server = grpc_aio.server()
            add_EntDBServiceServicer_to_server(servicer, server)
            server.add_insecure_port(f"127.0.0.1:{port}")
            await server.start()

            # Seed the contract node so reads have something to find.
            async with grpc_aio.insecure_channel(f"127.0.0.1:{port}") as channel:
                stub = EntDBServiceStub(channel)
                seed_data = Struct()
                seed_data.update({"1": "seeded@example.com", "2": "Seeded"})
                seed_req = pb.ExecuteAtomicRequest(
                    context=pb.RequestContext(tenant_id=TENANT, actor=ALICE),
                    idempotency_key="bench-seed-1",
                    operations=[
                        pb.Operation(
                            create_node=pb.CreateNodeOp(type_id=1, id=SEED_NODE_ID, data=seed_data)
                        )
                    ],
                    wait_applied=True,
                    wait_timeout_ms=5000,
                )
                seed_resp = await stub.ExecuteAtomic(seed_req)
                assert seed_resp.success, f"seed failed: {seed_resp.error}"

            state["server"] = server
            state["applier"] = applier
            state["applier_task"] = applier_task
            state["wal"] = wal
            state["global_store"] = global_store
            ready.set()

            # Block until stop() is called.
            while not stopped.is_set():
                await _asyncio.sleep(0.1)

            # Teardown
            await server.stop(grace=0)
            await applier.stop()
            applier_task.cancel()
            try:
                await _asyncio.wait_for(applier_task, timeout=2.0)
            except (_asyncio.CancelledError, _asyncio.TimeoutError):
                pass
            try:
                await wal.close()
            except Exception:
                pass
            global_store.close()

        try:
            loop.run_until_complete(_run())
        finally:
            loop.close()

    thread = threading.Thread(target=_serve, name="entdb-bench-server", daemon=True)
    thread.start()
    # Wait for the seeded server to be ready; 30s is generous.
    if not ready.wait(timeout=30.0):
        stopped.set()
        raise RuntimeError("bench: Python server didn't become ready within 30s")

    # Sanity: client-side readiness as well.
    import grpc as _grpc

    chan = _grpc.insecure_channel(f"127.0.0.1:{port}")
    try:
        _grpc.channel_ready_future(chan).result(timeout=5.0)
    finally:
        chan.close()

    handle = _PythonServerHandle(
        port=port,
        stopped=stopped,
        thread=thread,
        tmpdir_obj=tmpdir_obj,
        prev_reg=prev_reg,
    )
    return port, handle


class _PythonServerHandle:
    def __init__(
        self,
        port: int,
        stopped,
        thread,
        tmpdir_obj,
        prev_reg,
    ) -> None:
        self.port = port
        self._stopped = stopped
        self._thread = thread
        self._tmpdir_obj = tmpdir_obj
        self._prev_reg = prev_reg

    def stop(self) -> None:
        self._stopped.set()
        self._thread.join(timeout=10.0)
        from entdb_server.schema import registry as registry_mod

        registry_mod._global_registry = self._prev_reg
        self._tmpdir_obj.cleanup()


@pytest.fixture
def live_server(tmp_path_factory):
    """Yield a bound ``int`` port for a live EntDBService endpoint.

    Switches implementation based on the ``ENTDB_SERVER_TARGET`` env var:

    * ``python`` (default) — in-process gRPC servicer with in-memory WAL,
      hosted on a background thread so the (sync) benchmark code path
      doesn't share an event loop with the server.
    * ``go`` — ``server/go/cmd/entdb-server`` subprocess. We delegate to
      the integration conftest for the build+spawn logic.
    """
    target = _entdb_server_target()
    if target == "python":
        port, handle = _start_python_server_on_thread()
        try:
            yield port
        finally:
            handle.stop()
        return

    # Go target: spawn via the integration conftest helper. That helper
    # is an async generator; drive it from a fresh event loop on this
    # thread (the benchmark code path is sync and doesn't conflict with
    # the subprocess server).
    import asyncio as _asyncio

    from tests.python.integration.conftest import _start_go_server

    loop = _asyncio.new_event_loop()
    agen = _start_go_server(tmp_path_factory)
    try:
        port = loop.run_until_complete(agen.__anext__())
        try:
            yield port
        finally:
            try:
                loop.run_until_complete(agen.__anext__())
            except StopAsyncIteration:
                pass
    finally:
        loop.close()


@pytest.fixture
def grpc_endpoint(live_server) -> str:
    """``"127.0.0.1:<port>"`` form of :func:`live_server`."""
    return f"127.0.0.1:{live_server}"


# ---------------------------------------------------------------------------
# Per-backend collection gating
# ---------------------------------------------------------------------------
#
# When ENTDB_SERVER_TARGET=go we cannot run the Python-internal
# benchmarks (they reach into entdb_server.* Python modules with no
# Go equivalent). Skip them with a precise reason rather than letting
# them fail. The dual-backend wire bench (bench_grpc_dual.py) opts in
# explicitly via the ``cross_backend`` mark.


def pytest_configure(config: pytest.Config) -> None:
    config.addinivalue_line(
        "markers",
        "cross_backend: benchmark exercises the gRPC wire path and "
        "runs against both Python and Go backends.",
    )


def pytest_collection_modifyitems(config: pytest.Config, items: list[pytest.Item]) -> None:
    if _entdb_server_target() != "go":
        return
    skip_python_internal = pytest.mark.skip(
        reason="ENTDB_SERVER_TARGET=go: benchmark hits Python internals (no Go equivalent)"
    )
    for item in items:
        # Only the wire-level dual-backend benches opt in via the marker.
        if "cross_backend" in item.keywords:
            continue
        item.add_marker(skip_python_internal)
