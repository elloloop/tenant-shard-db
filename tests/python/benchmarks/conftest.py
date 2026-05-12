# SPDX-License-Identifier: AGPL-3.0-only
"""Pytest fixtures for the EntDB-vs-Postgres benchmark suite.

This module is the single boot/teardown point for both backends:

* ``entdb_stub`` — sync gRPC stub against a Go ``entdb-server`` subprocess
  (built and managed exactly like ``tests/python/integration/conftest.py``,
  but with the in-memory WAL — the bench is wire-level, not durability-
  level). The bench-only tenant + schema are seeded once via the
  ``contract`` seed profile and a synthetic corpus is then materialised
  through ``ExecuteAtomic`` so reads have something to find.
* ``pg_conn`` — a ``psycopg`` (v3) connection against a Postgres
  container that the harness brings up if ``ENTDB_BENCH_PG_DSN`` is not
  already set. Schema + indexes are created on first use, then a corpus
  matching the EntDB seed is inserted. The fixture is session-scoped
  so the round-trip is amortised across every ``bench_postgres`` test.

The corpus is deterministic (fixed RNG seed) and identical on both
sides: 1000 ``Task``-shaped nodes + 5000 fan-out edges per node so the
edge/traversal queries have realistic data. The same row contents are
INSERTed into Postgres in the schema-fixture step.

Pytest-benchmark groups are matched between the two modules so the
report doc can pair them directly:

    point-read | batched-read | filtered-read |
    single-write | multi-op-write | update |
    edge-fanout | reverse-edge-fanout | traversal |
    fulltext | mailbox-list | health

If a backend is not reachable the corresponding tests are skipped at
collection — the bench is not gated on Postgres being present, so a
plain ``pytest tests/python/benchmarks/`` run on a dev machine without
Docker still produces the EntDB-side numbers.
"""

from __future__ import annotations

import hashlib
import json
import logging
import os
import random
import socket
import subprocess
import time
import uuid
from collections.abc import Iterator
from pathlib import Path

import pytest

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Constants — kept in lock-step with server/go/internal/testseed/seed.go
# ---------------------------------------------------------------------------

BENCH_TENANT = "bench"
# ``system:bench`` actor bypasses ACL on both reads AND writes
# (server/go/internal/acl/actor.go:111 — system actors skip capability
# checks; server/go/internal/api/execute_atomic.go:585 — system/admin
# actors skip tenant-membership). Postgres pays no equivalent cost; the
# symmetric "skip auth on both sides" choice keeps the comparison
# apples-to-apples on both read and write paths. The previous bench
# actor ``user:alice`` still ran the ACL filter on reads (it returned
# trivial results because no canonical ACL store is wired, but
# ``acl.NewFilter(...)`` allocation + the FilterReadable round-trip
# was real overhead). See ``docs/benchmarks/benchmarks.md`` Caveats.
BENCH_ACTOR = "system:bench"

# The ``contract`` seed profile registers User=1 / Task=2 / AssignedTo=100.
# Task is the closest-fit corpus shape (has title/description, no
# unique constraint, no enum).
TASK_TYPE_ID = 2
USER_TYPE_ID = 1
ASSIGNED_TO_EDGE_ID = 100

# Fail-loud threshold on the seed: if fewer than this fraction of the
# expected nodes land, the bench is running against an empty / partial
# corpus and the numbers it reports are meaningless. The previous
# ``logger.warning``-only behaviour let a hard seed failure produce a
# green CI run on a near-empty DB (issue #491 item 1).
SEED_MIN_FRACTION = 0.10

CORPUS_SIZE = int(os.environ.get("ENTDB_BENCH_CORPUS", "1000"))
CORPUS_EDGES_PER_NODE = int(os.environ.get("ENTDB_BENCH_EDGE_FANOUT", "5"))

# Stable RNG so the two backends see byte-identical inputs.
CORPUS_SEED = 0x4E54444250470000


# ---------------------------------------------------------------------------
# Go entdb-server subprocess (re-uses the integration build cache)
# ---------------------------------------------------------------------------

_BINARY_ENV = "ENTDB_GO_BINARY"


def _free_port() -> int:
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


def _repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def _hash_go_sources(server_go_dir: Path) -> str:
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
    override = os.environ.get(_BINARY_ENV)
    if override:
        p = Path(override)
        if not p.is_file():
            raise RuntimeError(f"{_BINARY_ENV}={override!r} is not a file")
        return p

    repo_root = _repo_root()
    server_go_dir = repo_root / "server" / "go"
    if not server_go_dir.is_dir():
        raise RuntimeError(f"server/go missing at {server_go_dir}")

    sha = _hash_go_sources(server_go_dir)
    bin_dir = repo_root / ".go-bin"
    bin_dir.mkdir(exist_ok=True)
    bin_path = bin_dir / f"entdb-server-{sha}"
    if bin_path.is_file():
        return bin_path

    logger.info("bench-conftest: building Go entdb-server (sha=%s)", sha)
    proc = subprocess.run(
        ["go", "build", "-o", str(bin_path), "./cmd/entdb-server"],
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


def _wait_for_grpc_ready_sync(addr: str, timeout: float = 30.0) -> None:
    """Sync poll loop suitable for use from a sync fixture."""
    import grpc

    from entdb_sdk._generated import entdb_pb2 as pb
    from entdb_sdk._generated.entdb_pb2_grpc import EntDBServiceStub

    deadline = time.monotonic() + timeout
    last_err: Exception | None = None
    while time.monotonic() < deadline:
        try:
            channel = grpc.insecure_channel(addr)
            stub = EntDBServiceStub(channel)
            stub.Health(pb.HealthRequest(), timeout=1.0)
            channel.close()
            return
        except grpc.RpcError as exc:
            if exc.code() == grpc.StatusCode.UNIMPLEMENTED:  # type: ignore[attr-defined]
                return
            last_err = exc
        except Exception as exc:  # noqa: BLE001
            last_err = exc
        time.sleep(0.1)
    raise RuntimeError(f"gRPC server at {addr} not ready in {timeout}s: {last_err!r}")


@pytest.fixture(scope="session")
def entdb_endpoint(tmp_path_factory) -> Iterator[str]:
    """Yield ``host:port`` of a live Go ``entdb-server`` for the bench.

    Honours ``ENTDB_BENCH_ENDPOINT`` so the harness can point at an
    already-running container (the way ``run_bench.sh --tier=2`` does)
    without spawning a second subprocess.
    """
    override = os.environ.get("ENTDB_BENCH_ENDPOINT")
    if override:
        _wait_for_grpc_ready_sync(override, timeout=30.0)
        yield override
        return

    binary = _build_go_binary()
    data_dir = tmp_path_factory.mktemp("entdb-bench-data")
    log_path = tmp_path_factory.mktemp("entdb-bench-logs") / "server.log"

    last_err: Exception | None = None
    proc: subprocess.Popen | None = None
    port: int | None = None
    for _ in range(3):
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
            BENCH_TENANT,
        ]
        log_fh = open(log_path, "ab", buffering=0)  # noqa: SIM115
        proc = subprocess.Popen(
            cmd,
            stdout=log_fh,
            stderr=subprocess.STDOUT,
            env={**os.environ, "ENTDB_LOG_LEVEL": "warn"},
        )
        try:
            _wait_for_grpc_ready_sync(f"127.0.0.1:{port}", timeout=15.0)
            break
        except Exception as exc:  # noqa: BLE001
            last_err = exc
            proc.terminate()
            try:
                proc.wait(timeout=2.0)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=2.0)
            proc = None
    if proc is None or port is None:
        raise RuntimeError(
            f"failed to start Go entdb-server after 3 attempts: {last_err!r}; logs at {log_path}"
        )

    try:
        yield f"127.0.0.1:{port}"
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5.0)
        except subprocess.TimeoutExpired:
            proc.kill()


@pytest.fixture(scope="session")
def entdb_stub(entdb_endpoint: str):
    """Session-scoped sync ``EntDBServiceStub``.

    The bench is intentionally sync: pytest-benchmark drives a tight
    measurement loop and ``asyncio.run`` per round adds enough overhead
    to dominate the RPCs we're trying to measure. One channel per
    session keeps the connect cost out of the hot loop.
    """
    import grpc

    from entdb_sdk._generated.entdb_pb2_grpc import EntDBServiceStub

    channel = grpc.insecure_channel(entdb_endpoint)
    stub = EntDBServiceStub(channel)
    yield stub
    channel.close()


# ---------------------------------------------------------------------------
# Corpus
# ---------------------------------------------------------------------------


def _generate_corpus(size: int) -> list[dict]:
    """Deterministic 1k-row Task-shaped corpus.

    Each row is a ``{node_id, payload}`` dict where the payload mirrors
    the ``Task`` schema (title / description) and has been padded with
    a fixed-length body so encoded sizes are comparable across the
    two backends.
    """
    rng = random.Random(CORPUS_SEED)
    rows: list[dict] = []
    subjects = [
        "Refactor applier batching",
        "Investigate WAL replay flake",
        "Pin franz-go version",
        "Backfill schema validation",
        "Reduce p99 latency",
        "Improve SDK error messages",
        "Bench harness rewrite",
        "Multi-tenant ACL audit",
    ]
    for i in range(size):
        nid = f"bench-node-{i:06d}"
        subject = subjects[i % len(subjects)]
        body = (
            f"node-{i} body-token-{rng.randrange(10**6):06d}-" + "x" * 64 + f" owner=user-{i % 25}"
        )
        rows.append(
            {
                "node_id": nid,
                "payload": {
                    "title": subject,
                    "description": body,
                    # Used by Postgres-side mailbox-list query (predicate
                    # on ``owner``). EntDB doesn't filter on this in the
                    # bench — it queries by ``type_id``.
                    "owner": f"user-{i % 25}",
                },
            }
        )
    return rows


@pytest.fixture(scope="session")
def corpus() -> list[dict]:
    return _generate_corpus(CORPUS_SIZE)


# ---------------------------------------------------------------------------
# EntDB seeding (one-shot, session-scoped)
# ---------------------------------------------------------------------------


def _to_struct(d: dict):
    """Convert a plain dict to ``google.protobuf.Struct``."""
    from google.protobuf import struct_pb2

    s = struct_pb2.Struct()
    s.update(d)
    return s


@pytest.fixture(scope="session")
def entdb_seeded(entdb_stub, corpus):
    """Materialise the bench corpus through ``ExecuteAtomic``.

    Two RPCs per row would dominate seed time; the seed batches 50
    creates per call. Edges are added in a second pass so the per-row
    create stays a single-statement durable write (matches the
    operation-mapping row #5 baseline).

    Returns the list of created node ids so individual bench cases
    can index into a stable pool without re-seeding.
    """
    from entdb_sdk._generated import entdb_pb2 as pb

    ctx = pb.RequestContext(tenant_id=BENCH_TENANT, actor=BENCH_ACTOR)
    node_ids: list[str] = []
    seed_failures = 0

    BATCH = 50
    for batch_start in range(0, len(corpus), BATCH):
        ops = []
        for row in corpus[batch_start : batch_start + BATCH]:
            ops.append(
                pb.Operation(
                    create_node=pb.CreateNodeOp(
                        type_id=TASK_TYPE_ID,
                        id=row["node_id"],
                        data=_to_struct(row["payload"]),
                    )
                )
            )
            node_ids.append(row["node_id"])
        req = pb.ExecuteAtomicRequest(
            context=ctx,
            idempotency_key=f"bench-seed-{batch_start}-{uuid.uuid4().hex[:6]}",
            operations=ops,
        )
        try:
            entdb_stub.ExecuteAtomic(req, timeout=30.0)
        except Exception as exc:  # noqa: BLE001
            seed_failures += len(ops)
            logger.warning("entdb seed batch %d failed: %r", batch_start, exc)

    # Fail-loud: if too few nodes landed, the bench would run against
    # an empty/partial corpus and produce meaningless numbers (issue
    # #491 item 1). Threshold deliberately generous — re-seeding into a
    # pre-populated data-dir is fine (idempotency-key collisions are
    # expected), but a hard "server not reachable" or "schema not
    # registered" should fail the bench rather than be hidden behind a
    # log line.
    expected = len(corpus)
    landed = expected - seed_failures
    if landed < max(1, int(expected * SEED_MIN_FRACTION)):
        raise RuntimeError(
            f"entdb seed: only {landed}/{expected} node creates succeeded "
            f"(< {SEED_MIN_FRACTION:.0%} threshold); bench would run against "
            f"a near-empty corpus. Check the server log."
        )

    # Edge fan-out — every node gets ``CORPUS_EDGES_PER_NODE`` edges to
    # the next K nodes in the corpus (wraps). Hoist ``edge_ops`` out of
    # the ``if i % BATCH == 0`` guard so a zero-length ``node_ids`` or
    # first-iteration break can't leave it unbound (issue #491 item 2).
    edge_ops: list = []
    for i, nid in enumerate(node_ids):
        if i % BATCH == 0:
            edge_ops = []
        for k in range(1, CORPUS_EDGES_PER_NODE + 1):
            target = node_ids[(i + k) % len(node_ids)]
            edge_op = pb.CreateEdgeOp(edge_id=ASSIGNED_TO_EDGE_ID)
            # ``from`` is a Python keyword — protobuf exposes the field
            # under that exact name, so kwargs unpacking is the only
            # construction form that works.
            getattr(edge_op, "from").CopyFrom(pb.NodeRef(id=nid))
            edge_op.to.CopyFrom(pb.NodeRef(id=target))
            edge_ops.append(pb.Operation(create_edge=edge_op))
        if (i % BATCH == BATCH - 1) or (i == len(node_ids) - 1):
            req = pb.ExecuteAtomicRequest(
                context=ctx,
                idempotency_key=f"bench-seed-edges-{i}-{uuid.uuid4().hex[:6]}",
                operations=edge_ops,
            )
            try:
                entdb_stub.ExecuteAtomic(req, timeout=60.0)
            except Exception as exc:  # noqa: BLE001
                # Edge-seed failures are not as load-bearing as the node
                # seed (edge benches will simply return empty result
                # sets and the bench will still produce numbers for
                # the non-edge cases). Still log loud + bail early.
                logger.error("entdb seed edges batch %d failed: %r", i, exc)
                break

    # Give the applier a moment to drain.
    time.sleep(0.5)
    return node_ids


# ---------------------------------------------------------------------------
# Postgres harness
# ---------------------------------------------------------------------------


def _pg_image() -> str:
    return os.environ.get("ENTDB_BENCH_PG_IMAGE", "postgres:17-alpine")


def _docker_available() -> bool:
    try:
        proc = subprocess.run(["docker", "version"], capture_output=True, text=True, timeout=5)
        return proc.returncode == 0
    except Exception:  # noqa: BLE001
        return False


def _pg_dsn_from_env() -> str | None:
    return os.environ.get("ENTDB_BENCH_PG_DSN")


@pytest.fixture(scope="session")
def pg_dsn() -> Iterator[str]:
    """Yield a Postgres DSN for the bench.

    Resolution order:

    1. ``ENTDB_BENCH_PG_DSN`` env var — set by ``run_bench.sh`` when the
       compose stack is already up, by CI when a Postgres service
       container is wired up, etc. Skip the container plumbing.
    2. Docker available → start a single ``postgres:17-alpine``
       container on a free port, set ``shared_buffers`` to default,
       no other tuning, tear down at session end.
    3. Skip Postgres tests with a helpful message.
    """
    dsn = _pg_dsn_from_env()
    if dsn:
        yield dsn
        return

    if not _docker_available():
        pytest.skip("Postgres bench needs Docker or ENTDB_BENCH_PG_DSN; neither found.")

    name = f"entdb-bench-pg-{uuid.uuid4().hex[:8]}"
    port = _free_port()
    image = _pg_image()

    cmd = [
        "docker",
        "run",
        "-d",
        "--rm",
        "--name",
        name,
        "-e",
        "POSTGRES_PASSWORD=bench",
        "-e",
        "POSTGRES_USER=bench",
        "-e",
        "POSTGRES_DB=bench",
        "-p",
        f"{port}:5432",
        image,
    ]
    proc = subprocess.run(cmd, capture_output=True, text=True)
    if proc.returncode != 0:
        pytest.skip(f"failed to start Postgres container: {proc.stderr.strip()}")

    dsn = f"postgresql://bench:bench@127.0.0.1:{port}/bench"

    # Wait for Postgres to accept connections.
    try:
        import psycopg
    except ImportError:
        subprocess.run(["docker", "rm", "-f", name], capture_output=True)
        pytest.skip("psycopg (v3) not installed; pip install 'psycopg[binary]'")

    deadline = time.monotonic() + 30.0
    last_err: Exception | None = None
    while time.monotonic() < deadline:
        try:
            with psycopg.connect(dsn, connect_timeout=2) as conn:
                conn.execute("SELECT 1")
            break
        except Exception as exc:  # noqa: BLE001
            last_err = exc
            time.sleep(0.5)
    else:
        subprocess.run(["docker", "rm", "-f", name], capture_output=True)
        pytest.skip(f"Postgres did not come up in 30s: {last_err!r}")

    try:
        yield dsn
    finally:
        subprocess.run(["docker", "rm", "-f", name], capture_output=True)


@pytest.fixture(scope="session")
def pg_conn(pg_dsn: str, corpus: list[dict]):
    """Session-scoped psycopg connection with schema + corpus loaded."""
    try:
        import psycopg
    except ImportError:
        pytest.skip("psycopg (v3) not installed; pip install 'psycopg[binary]'")

    conn = psycopg.connect(pg_dsn, autocommit=False)

    # Schema mirrors the operation-mapping table in issue #487. ``id``
    # is TEXT (not UUID) so we can reuse the same human-readable node
    # ids EntDB uses; ``payload`` is JSONB so containment + GIN apply.
    schema_sql = """
    DROP TABLE IF EXISTS edges;
    DROP TABLE IF EXISTS nodes;

    CREATE TABLE nodes (
        tenant_id  TEXT  NOT NULL,
        id         TEXT  NOT NULL,
        type_id    INT   NOT NULL,
        payload    JSONB NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
        PRIMARY KEY (tenant_id, id)
    );

    CREATE INDEX nodes_type_idx     ON nodes(tenant_id, type_id, created_at DESC);
    CREATE INDEX nodes_payload_gin  ON nodes USING GIN (payload jsonb_path_ops);
    CREATE INDEX nodes_fts_idx      ON nodes USING GIN (
        to_tsvector('simple',
            coalesce(payload->>'title','') || ' ' || coalesce(payload->>'description',''))
    );

    CREATE TABLE edges (
        tenant_id  TEXT  NOT NULL,
        from_id    TEXT  NOT NULL,
        edge_type  INT   NOT NULL,
        to_id      TEXT  NOT NULL,
        props      JSONB NOT NULL DEFAULT '{}'::jsonb,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
        PRIMARY KEY (tenant_id, from_id, edge_type, to_id)
    );

    CREATE INDEX edges_to_idx   ON edges(tenant_id, to_id, edge_type);
    """
    with conn.cursor() as cur:
        cur.execute(schema_sql)
    conn.commit()

    # Bulk-load the corpus + edges. ``COPY`` would be faster but
    # ``executemany`` keeps the seed code identical to the on-the-fly
    # writes the bench measures.
    with conn.cursor() as cur:
        cur.executemany(
            "INSERT INTO nodes (tenant_id, id, type_id, payload) VALUES (%s, %s, %s, %s::jsonb)",
            [
                (BENCH_TENANT, row["node_id"], TASK_TYPE_ID, json.dumps(row["payload"]))
                for row in corpus
            ],
        )
        edge_rows = []
        for i, row in enumerate(corpus):
            for k in range(1, CORPUS_EDGES_PER_NODE + 1):
                target = corpus[(i + k) % len(corpus)]["node_id"]
                edge_rows.append((BENCH_TENANT, row["node_id"], ASSIGNED_TO_EDGE_ID, target))
        cur.executemany(
            "INSERT INTO edges (tenant_id, from_id, edge_type, to_id) VALUES (%s, %s, %s, %s)",
            edge_rows,
        )
        cur.execute("ANALYZE nodes")
        cur.execute("ANALYZE edges")
    conn.commit()

    yield conn

    conn.close()


# ---------------------------------------------------------------------------
# Reporting hook — record ratio targets so the report doc can ingest
# the raw pytest-benchmark JSON without parsing the test source.
# ---------------------------------------------------------------------------


# Group → north-star ratio (EntDB p50 / Postgres p50). ``None`` means
# the operation is reported side-by-side but isn't gated on a target
# (e.g. full-text search — different semantics).
RATIO_TARGETS = {
    "point-read": 1.30,
    "batched-read": 1.20,
    "filtered-read": 1.50,
    "single-write": 2.00,
    "multi-op-write": 1.30,
    "update": 1.50,
    "edge-fanout": 1.00,  # EntDB should match-or-beat Postgres here.
    "reverse-edge-fanout": 1.00,
    "traversal": 1.50,
    "fulltext": None,
    "mailbox-list": 1.50,
    "health": 1.00,
}


def pytest_collection_modifyitems(config, items):
    """Tag each bench item with its group target ratio.

    pytest-benchmark already namespaces tests by ``group``; we attach
    the ratio so a downstream tool (or the eyeball test) can read
    ``items[].user_properties`` from the JSON.
    """
    for item in items:
        marker = item.get_closest_marker("benchmark")
        if marker is None:
            continue
        group = marker.kwargs.get("group")
        target = RATIO_TARGETS.get(group)
        if target is not None:
            item.user_properties.append(("ratio_target", target))
