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
SEED_RECEIPT = "seed-1"

# Contract-suite schema (ADR-031). The server now boots schema-less; the
# harness bootstraps tenant + users + schema + the seed node through the
# real API, exactly like a production client — there is NO boot seed.
#
# This is the name-free schema the old testseed.RegisterContractSchema
# loaded, kept in lock-step with:
#   - tests/python/integration/test_composite_unique.py (OAuthIdentity 201)
#   - tests/python/integration/test_grpc_contract.py    (User type 1)
#   - server/go/internal/schema (name-free FieldKind wire strings)
#
# User (type_id 1): field 1 = email, field 2 = name.
# OAuthIdentity (type_id 201): 1 = provider, 2 = provider_user_id,
#   3 = email (single-field unique), composite_unique on the (1,2) tuple.
CONTRACT_USER_TYPE_ID = 1
CONTRACT_OAUTH_TYPE_ID = 201


def _contract_schema_descriptor():
    """Build the name-free contract SchemaDescriptor (ADR-031).

    Lives here (not in the SDK) so the harness exercises the same wire
    surface a hand-rolled client would: id-keyed types, id-keyed fields,
    composite-unique by field-id tuple, no names anywhere.
    """
    from entdb_sdk._generated import entdb_pb2 as pb

    user = pb.SchemaNodeTypeDef(
        type_id=CONTRACT_USER_TYPE_ID,
        fields=[
            pb.SchemaFieldDef(field_id=1, kind="str"),
            pb.SchemaFieldDef(field_id=2, kind="str"),
        ],
    )
    oauth = pb.SchemaNodeTypeDef(
        type_id=CONTRACT_OAUTH_TYPE_ID,
        fields=[
            pb.SchemaFieldDef(field_id=1, kind="str"),
            pb.SchemaFieldDef(field_id=2, kind="str"),
            pb.SchemaFieldDef(field_id=3, kind="str", unique=True),
        ],
        composite_unique=[pb.SchemaCompositeUniqueDef(field_ids=[1, 2])],
    )
    return pb.SchemaDescriptor(node_types=[user, oauth])


def _contract_schema_fingerprint() -> str:
    """The name-free fingerprint of the contract schema.

    Computed with the SAME canonicaliser the server and SDK use so a write
    that carries the descriptor lands the schema, and a later read sees a
    matching GetSchema fingerprint. Pinned here as a constant-by-construction
    rather than hand-copied so a schema edit can't silently drift.
    """
    import hashlib
    import json

    def _field(f) -> dict:
        out: dict = {"field_id": f.field_id, "kind": f.kind}
        if f.unique:
            out["unique"] = True
        return out

    def _node(n) -> dict:
        out: dict = {
            "type_id": n.type_id,
            "fields": [_field(f) for f in n.fields],
        }
        if n.composite_unique:
            out["composite_unique"] = [{"field_ids": list(c.field_ids)} for c in n.composite_unique]
        return out

    desc = _contract_schema_descriptor()
    doc = {
        "node_types": [_node(n) for n in desc.node_types],
        "edge_types": [],
    }
    canon = json.dumps(doc, sort_keys=True, separators=(",", ":"))
    return "sha256:" + hashlib.sha256(canon.encode("utf-8")).hexdigest()


async def _bootstrap_contract(port: int) -> None:
    """Provision the contract fixtures through the live gRPC API.

    The server boots EMPTY (ADR-031) — no tenant, no users, no schema, no
    seed node. This walks the exact sequence a real client would, in order:

      1. CreateTenant("acme"), CreateUser(alice/bob), AddTenantMember
         (alice=owner, bob=member) — then poll GetTenant until the tenant
         materializes (CreateTenant goes through the global WAL/applier).
      2. A self-describing ExecuteAtomic that carries the name-free
         SchemaDescriptor + fingerprint AND creates the contract seed node
         (type 1, id "seeded-node", idempotency_key "seed-1"). The server
         prepends a register_schema WAL op (establish-or-reject) so the
         schema + per-tenant indexes exist before the data op — and a later
         GetReceiptStatus("seed-1") reads APPLIED.
    """
    from entdb_sdk._generated import entdb_pb2 as pb
    from entdb_sdk._generated.entdb_pb2_grpc import EntDBServiceStub

    admin = "system:admin"
    async with grpc_aio.insecure_channel(f"127.0.0.1:{port}") as ch:
        stub = EntDBServiceStub(ch)

        # 1. Tenant + users + memberships.
        r = await stub.CreateTenant(
            pb.CreateTenantRequest(actor=admin, tenant_id=TENANT, name="Acme Corp")
        )
        assert r.success, r
        for uid, email, name in (
            ("alice", "alice@example.com", "Alice"),
            ("bob", "bob@example.com", "Bob"),
        ):
            ru = await stub.CreateUser(
                pb.CreateUserRequest(actor=admin, user_id=uid, email=email, name=name)
            )
            assert ru.success, ru
        for uid, role in (("alice", "owner"), ("bob", "member")):
            rm = await stub.AddTenantMember(
                pb.TenantMemberRequest(actor=admin, tenant_id=TENANT, user_id=uid, role=role)
            )
            assert rm.success, rm

        # Tenant creation flows through the global WAL → applier; poll until
        # it is visible before the first per-tenant write.
        for _ in range(200):
            g = await stub.GetTenant(pb.GetTenantRequest(actor=admin, tenant_id=TENANT))
            if g.found:
                break
            await asyncio.sleep(0.02)
        else:
            raise RuntimeError(f"tenant {TENANT!r} never materialized after CreateTenant")

        # 2. Self-describing write: register the schema + create the seed node
        #    in one atomic event (a leading register_schema op is prepended by
        #    the server because the carried fingerprint differs from empty).
        from google.protobuf import struct_pb2

        data = struct_pb2.Struct()
        # Id-keyed payload (ADR-031 / CLAUDE.md invariant #6): field 1 =
        # email, field 2 = name.
        data.update({"1": "seeded@example.com", "2": "Seeded"})
        req = pb.ExecuteAtomicRequest(
            context=pb.RequestContext(tenant_id=TENANT, actor=ALICE),
            idempotency_key=SEED_RECEIPT,
            schema_fingerprint=_contract_schema_fingerprint(),
            schema=_contract_schema_descriptor(),
            operations=[
                pb.Operation(
                    create_node=pb.CreateNodeOp(
                        type_id=CONTRACT_USER_TYPE_ID, id=SEED_NODE_ID, data=data
                    )
                )
            ],
            wait_applied=True,
            wait_timeout_ms=10000,
        )
        resp = await stub.ExecuteAtomic(req)
        assert resp.success, f"seed write failed: {resp.error} ({resp.error_code})"


async def _start_go_server(tmp_path_factory, *, bootstrap: bool = True) -> AsyncIterator[int]:
    """Go subprocess harness — yields the bound port.

    EMPTY BOOT (ADR-031). The server imports nothing at startup: no schema,
    no tenant/user data, no seed node. The boot-time schema-registry crutch
    AND the data seed (``--seed-profile`` / ``--seed-tenant``) have BOTH been
    removed — there is no production path that loaded either, so neither
    exists here. The schema, tenant, users and seed node are provisioned
    through the gRPC API after boot, exactly like a real client (see
    :func:`_bootstrap_contract`).

    ``bootstrap`` selects whether the contract fixtures are provisioned:
      - ``True`` (default): create tenant ``acme`` + alice/bob users +
        memberships, register the contract schema via a self-describing
        write, and create the seed node + ``seed-1`` receipt. The contract /
        query / precondition / mailbox / composite suites depend on this.
      - ``False``: a bare empty server — used by the issue #545 schema-less
        DeleteWhere regression, which provisions its OWN tenant and writes
        id-keyed payloads against a schema-less server.
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

    if bootstrap:
        # Provision tenant + users + schema + seed node through the API,
        # exactly like a real client (no boot seed exists anymore).
        await _bootstrap_contract(port)

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


@pytest.fixture
async def schemaless_live_server(tmp_path_factory):
    """A live server booted EMPTY with NO contract bootstrap.

    Used by the issue #545 regression: a schema-less server cannot resolve
    a field NAME, so DeleteWhere must be driven by the numeric payload field
    id. No tenant, user, schema or seed node is provisioned — the test
    creates its own tenant and writes id-keyed payloads.
    """
    agen = _start_go_server(tmp_path_factory, bootstrap=False)
    port = await agen.__anext__()
    try:
        yield port
    finally:
        try:
            await agen.__anext__()
        except StopAsyncIteration:
            pass


@pytest.fixture
def schemaless_grpc_endpoint(schemaless_live_server) -> str:
    """``"127.0.0.1:<port>"`` form of :func:`schemaless_live_server`."""
    return f"127.0.0.1:{schemaless_live_server}"
