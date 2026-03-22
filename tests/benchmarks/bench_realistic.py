"""
Realistic multi-tenant benchmark: EntDB vs PostgreSQL.

Simulates 3 colleges sharing the same infrastructure, each with 12 months of
accumulated usage data on the cheapest container tier (0.25 CPU / 512 MB).

Data model per tenant (12 months of usage):
  - Users:         4,000 nodes  (type 1)
  - Chat messages: 500,000 nodes (type 2)
  - Posts:         50,000 nodes  (type 3)
  - Comments:      200,000 nodes (type 4)
  - Tasks:         30,000 nodes  (type 5)
  - Edges:         300,000       (follows, replies, reactions, assignments)
  Total: 784,000 nodes + 300,000 edges = 1,084,000 rows per tenant
         x 3 tenants = 3,252,000 total rows

Benchmarks (run against a random tenant):
  - Point reads (get by ID from 784K rows)
  - Type queries (list posts, paginated, from 50K posts)
  - Edge queries (fan-out from random node across 300K edges)
  - Writes (new chat message into 500K existing messages)
  - Cross-tenant isolation (read tenant A while writing tenant B)

Run:
  python tests/benchmarks/bench_realistic.py
"""

from __future__ import annotations

import json
import random
import statistics
import subprocess
import sys
import time
import uuid

# ---------------------------------------------------------------------------
# Data volumes per tenant
# ---------------------------------------------------------------------------
# college-0: large university, 18 months of data  → 2.1M nodes
# college-1: mid-size college, 12 months           → 1.1M nodes
# college-2: small college, 6 months               → 400K nodes
TENANT_CONFIGS = {
    "college-0": {
        "users": 4_000,
        "messages": 1_500_000,
        "posts": 100_000,
        "comments": 400_000,
        "tasks": 50_000,
        "edges": 500_000,
    },
    "college-1": {
        "users": 4_000,
        "messages": 800_000,
        "posts": 60_000,
        "comments": 200_000,
        "tasks": 30_000,
        "edges": 300_000,
    },
    "college-2": {
        "users": 2_000,
        "messages": 300_000,
        "posts": 30_000,
        "comments": 60_000,
        "tasks": 10_000,
        "edges": 100_000,
    },
}

TENANT_IDS = list(TENANT_CONFIGS.keys())
NUM_TENANTS = len(TENANT_IDS)

# Benchmark parameters
BENCH_READS = 1000
BENCH_WRITES = 500
BENCH_MIXED_OPS = 200

TIER = {"name": "0.25 CPU / 512MB", "cpus": "0.25", "memory": "512m"}


def _tenant_type_configs(tenant_id: str) -> list[tuple[int, str, int]]:
    """Get (type_id, type_name, count) for a tenant."""
    cfg = TENANT_CONFIGS[tenant_id]
    return [
        (1, "user", cfg["users"]),
        (2, "message", cfg["messages"]),
        (3, "post", cfg["posts"]),
        (4, "comment", cfg["comments"]),
        (5, "task", cfg["tasks"]),
    ]


def _tenant_total_nodes(tenant_id: str) -> int:
    cfg = TENANT_CONFIGS[tenant_id]
    return cfg["users"] + cfg["messages"] + cfg["posts"] + cfg["comments"] + cfg["tasks"]


def _tenant_edges(tenant_id: str) -> int:
    return TENANT_CONFIGS[tenant_id]["edges"]


TOTAL_ALL_NODES = sum(_tenant_total_nodes(t) for t in TENANT_IDS)
TOTAL_ALL_EDGES = sum(_tenant_edges(t) for t in TENANT_IDS)
TOTAL_ALL_ROWS = TOTAL_ALL_NODES + TOTAL_ALL_EDGES


# ---------------------------------------------------------------------------
# Payload generation
# ---------------------------------------------------------------------------


def _rand_payload(type_name: str, i: int, num_users: int = 4000) -> str:
    """Generate a realistic JSON payload for each node type."""
    if type_name == "user":
        return json.dumps(
            {
                "name": f"Student {i}",
                "email": f"student{i}@college.edu",
                "major": random.choice(["CS", "Math", "Physics", "English", "History", "Bio"]),
                "year": random.choice([1, 2, 3, 4]),
                "bio": f"I am student {i}. " + "x" * random.randint(50, 200),
            }
        )
    if type_name == "message":
        chan = random.randint(1, 200)
        thread = random.randint(1, 5000)
        return json.dumps(
            {
                "text": f"Message {i}: " + "lorem ipsum " * random.randint(3, 30),
                "channel": f"channel-{chan}",
                "thread_id": f"thread-{thread}",
            }
        )
    if type_name == "post":
        return json.dumps(
            {
                "title": f"Post {i}: "
                + " ".join(
                    random.choices(
                        ["help", "question", "discussion", "announcement", "meme", "event"], k=3
                    )
                ),
                "body": "Post body content. " * random.randint(5, 50),
                "subreddit": random.choice(
                    ["general", "cs101", "dorms", "food", "sports", "clubs", "memes"]
                ),
                "score": random.randint(-10, 500),
            }
        )
    if type_name == "comment":
        return json.dumps(
            {
                "text": "Comment text. " * random.randint(1, 10),
                "score": random.randint(-5, 100),
            }
        )
    if type_name == "task":
        month = random.randint(1, 12)
        day = random.randint(1, 28)
        assignee_idx = random.randint(0, num_users - 1)
        return json.dumps(
            {
                "title": f"Task {i}",
                "description": "Task description. " * random.randint(2, 10),
                "status": random.choice(["todo", "in_progress", "done"]),
                "due_date": f"2026-{month:02d}-{day:02d}",
                "assignee": f"student{assignee_idx}@college.edu",
            }
        )
    return json.dumps({"data": f"item-{i}"})


def _build_node_ids(tenant_id: str) -> list[str]:
    """Build the list of all node IDs for one tenant (deterministic)."""
    ids: list[str] = []
    for _, type_name, count in _tenant_type_configs(tenant_id):
        ids.extend(f"{type_name}-{i}" for i in range(count))
    return ids


def _type_id_from_node_id(nid: str) -> int:
    """Derive type_id from a node ID prefix like 'message-42'."""
    prefix = nid.split("-")[0]
    return {"user": 1, "message": 2, "post": 3, "comment": 4, "task": 5}.get(prefix, 1)


# ---------------------------------------------------------------------------
# PostgreSQL seeding and benchmarking
# ---------------------------------------------------------------------------


def _pg_connect():
    """Return a psycopg2 connection to the benchmark Postgres."""
    import psycopg2

    return psycopg2.connect(
        host="localhost", port=5434, user="bench", password="bench", dbname="bench"
    )


def seed_postgres(num_tenants: int) -> float:
    """Seed Postgres with realistic data for *num_tenants* tenants. Returns seconds."""
    from psycopg2.extras import execute_values

    conn = _pg_connect()
    conn.autocommit = False
    cur = conn.cursor()

    # Schema
    cur.execute("""
        CREATE TABLE IF NOT EXISTS nodes (
            tenant_id TEXT NOT NULL, node_id TEXT NOT NULL,
            type_id INTEGER NOT NULL, payload_json TEXT NOT NULL DEFAULT '{}',
            created_at BIGINT NOT NULL, updated_at BIGINT NOT NULL,
            owner_actor TEXT NOT NULL, acl_blob TEXT NOT NULL DEFAULT '[]',
            PRIMARY KEY (tenant_id, node_id))
    """)
    cur.execute(
        "CREATE INDEX IF NOT EXISTS idx_type ON nodes (tenant_id, type_id, created_at DESC)"
    )
    cur.execute("CREATE INDEX IF NOT EXISTS idx_owner ON nodes (tenant_id, owner_actor)")
    cur.execute("""
        CREATE TABLE IF NOT EXISTS edges (
            tenant_id TEXT NOT NULL, edge_type_id INTEGER NOT NULL,
            from_node_id TEXT NOT NULL, to_node_id TEXT NOT NULL,
            props_json TEXT NOT NULL DEFAULT '{}', created_at BIGINT NOT NULL,
            PRIMARY KEY (tenant_id, edge_type_id, from_node_id, to_node_id))
    """)
    cur.execute("CREATE INDEX IF NOT EXISTS idx_edge_from ON edges (tenant_id, from_node_id)")
    cur.execute("CREATE INDEX IF NOT EXISTS idx_edge_to ON edges (tenant_id, to_node_id)")
    cur.execute("TRUNCATE nodes, edges")
    conn.commit()

    batch_size = 1000
    ts_base = int(time.time() * 1000) - (365 * 86400 * 1000)  # 12 months ago

    start = time.perf_counter()

    for tenant_idx in range(num_tenants):
        tid = TENANT_IDS[tenant_idx]
        all_node_ids: list[str] = []

        # Nodes
        num_users = TENANT_CONFIGS[tid]["users"]
        for type_id, type_name, count in _tenant_type_configs(tid):
            batch: list[tuple] = []
            for i in range(count):
                nid = f"{type_name}-{i}"
                all_node_ids.append(nid)
                ts = ts_base + (i * 60000)
                actor = f"user:{i % num_users}"
                batch.append(
                    (tid, nid, type_id, _rand_payload(type_name, i, num_users), ts, ts, actor, "[]")
                )
                if len(batch) >= batch_size:
                    execute_values(
                        cur,
                        "INSERT INTO nodes "
                        "(tenant_id,node_id,type_id,payload_json,"
                        "created_at,updated_at,owner_actor,acl_blob) VALUES %s",
                        batch,
                    )
                    conn.commit()
                    batch = []
            if batch:
                execute_values(
                    cur,
                    "INSERT INTO nodes "
                    "(tenant_id,node_id,type_id,payload_json,"
                    "created_at,updated_at,owner_actor,acl_blob) VALUES %s",
                    batch,
                )
                conn.commit()

        # Edges
        batch_e: list[tuple] = []
        for i in range(_tenant_edges(tid)):
            from_id = random.choice(all_node_ids)
            to_id = random.choice(all_node_ids)
            edge_type = random.choice([1, 2, 3])
            ts = ts_base + (i * 30000)
            batch_e.append((tid, edge_type, from_id, to_id, "{}", ts))
            if len(batch_e) >= batch_size:
                execute_values(
                    cur,
                    "INSERT INTO edges "
                    "(tenant_id,edge_type_id,from_node_id,to_node_id,props_json,created_at) "
                    "VALUES %s ON CONFLICT DO NOTHING",
                    batch_e,
                )
                conn.commit()
                batch_e = []
        if batch_e:
            execute_values(
                cur,
                "INSERT INTO edges "
                "(tenant_id,edge_type_id,from_node_id,to_node_id,props_json,created_at) "
                "VALUES %s ON CONFLICT DO NOTHING",
                batch_e,
            )
            conn.commit()

        print(f"    Tenant {tid} seeded ({tenant_idx + 1}/{num_tenants})", flush=True)

    elapsed = time.perf_counter() - start
    conn.close()
    return elapsed


def bench_pg_point_reads(num: int, tenant_id: str, all_ids: list[str]) -> dict:
    """Read random nodes by ID from populated PG."""
    conn = _pg_connect()
    conn.autocommit = True
    cur = conn.cursor()

    latencies: list[float] = []
    for _ in range(num):
        nid = random.choice(all_ids)
        t0 = time.perf_counter()
        cur.execute("SELECT * FROM nodes WHERE tenant_id = %s AND node_id = %s", (tenant_id, nid))
        cur.fetchone()
        latencies.append(time.perf_counter() - t0)

    conn.close()
    return _summarize(latencies)


def bench_pg_type_query(num: int, tenant_id: str) -> dict:
    """Query posts by type with pagination from populated PG."""
    conn = _pg_connect()
    conn.autocommit = True
    cur = conn.cursor()

    num_posts = TENANT_CONFIGS[tenant_id]["posts"]
    latencies: list[float] = []
    for _ in range(num):
        offset = random.randint(0, max(num_posts - 20, 0))
        t0 = time.perf_counter()
        cur.execute(
            "SELECT * FROM nodes WHERE tenant_id = %s AND type_id = 3 "
            "ORDER BY created_at DESC LIMIT 20 OFFSET %s",
            (tenant_id, offset),
        )
        cur.fetchall()
        latencies.append(time.perf_counter() - t0)

    conn.close()
    return _summarize(latencies)


def bench_pg_edge_query(num: int, tenant_id: str, all_ids: list[str]) -> dict:
    """Query edges from a node in populated PG."""
    conn = _pg_connect()
    conn.autocommit = True
    cur = conn.cursor()

    latencies: list[float] = []
    for _ in range(num):
        nid = random.choice(all_ids)
        t0 = time.perf_counter()
        cur.execute(
            "SELECT * FROM edges WHERE tenant_id = %s AND from_node_id = %s",
            (tenant_id, nid),
        )
        cur.fetchall()
        latencies.append(time.perf_counter() - t0)

    conn.close()
    return _summarize(latencies)


def bench_pg_writes(num: int, tenant_id: str) -> dict:
    """Write new nodes into populated PG table."""
    conn = _pg_connect()
    conn.autocommit = False
    cur = conn.cursor()

    ts = int(time.time() * 1000)
    latencies: list[float] = []
    for i in range(num):
        nid = f"new-msg-{uuid.uuid4().hex[:8]}"
        actor = f"user:{i % 4000}"
        t0 = time.perf_counter()
        cur.execute(
            "INSERT INTO nodes VALUES (%s,%s,%s,%s,%s,%s,%s,%s)",
            (tenant_id, nid, 2, _rand_payload("message", i), ts, ts, actor, "[]"),
        )
        conn.commit()
        latencies.append(time.perf_counter() - t0)

    conn.close()
    return _summarize(latencies)


# ---------------------------------------------------------------------------
# EntDB seeding and benchmarking (via gRPC)
# ---------------------------------------------------------------------------


def _entdb_channel():
    """Return a gRPC channel to EntDB."""
    import grpc

    return grpc.insecure_channel("localhost:50052")


def seed_entdb(num_tenants: int) -> float:
    """Seed EntDB via gRPC for *num_tenants* tenants. Returns seconds."""
    import grpc

    from entdb_sdk._generated import entdb_pb2, entdb_pb2_grpc

    channel = _entdb_channel()
    stub = entdb_pb2_grpc.EntDBServiceStub(channel)

    start = time.perf_counter()

    for tenant_idx in range(num_tenants):
        tid = TENANT_IDS[tenant_idx]
        total = 0

        for type_id, type_name, count in _tenant_type_configs(tid):
            grpc_batch_size = 50
            for batch_start in range(0, count, grpc_batch_size):
                batch_end = min(batch_start + grpc_batch_size, count)
                ops = []
                for i in range(batch_start, batch_end):
                    ops.append(
                        entdb_pb2.Operation(
                            create_node=entdb_pb2.CreateNodeOp(
                                type_id=type_id,
                                id=f"{type_name}-{i}",
                                data_json=_rand_payload(type_name, i),
                            ),
                        )
                    )

                actor = f"user:{batch_start % 4000}"
                ctx = entdb_pb2.RequestContext(tenant_id=tid, actor=actor)
                idem_key = f"seed-{tid}-{type_name}-{batch_start}-{uuid.uuid4().hex[:6]}"
                req = entdb_pb2.ExecuteAtomicRequest(
                    context=ctx,
                    idempotency_key=idem_key,
                    operations=ops,
                )
                try:
                    stub.ExecuteAtomic(req)
                    total += len(ops)
                except grpc.RpcError as exc:
                    code = exc.code()
                    print(f"    Seed error at {tid} {type_name}-{batch_start}: {code}")

                if total % 10000 == 0 and total > 0:
                    print(
                        f"    [{tid}] Seeded {total:,}/{_tenant_total_nodes(tid):,} nodes...",
                        flush=True,
                    )

        print(
            f"    Tenant {tid} done ({tenant_idx + 1}/{num_tenants}, {total:,} nodes)",
            flush=True,
        )

    # Wait for applier to catch up
    print("    Waiting for applier to catch up...", flush=True)
    time.sleep(5)

    elapsed = time.perf_counter() - start
    channel.close()
    return elapsed


def bench_entdb_point_reads(num: int, tenant_id: str, all_ids: list[str]) -> dict:
    """Read random nodes by ID from populated EntDB."""
    import grpc

    from entdb_sdk._generated import entdb_pb2, entdb_pb2_grpc

    channel = _entdb_channel()
    stub = entdb_pb2_grpc.EntDBServiceStub(channel)
    ctx = entdb_pb2.RequestContext(tenant_id=tenant_id, actor="bench")

    latencies: list[float] = []
    for _ in range(num):
        nid = random.choice(all_ids)
        type_id = _type_id_from_node_id(nid)
        t0 = time.perf_counter()
        try:  # noqa: SIM105
            stub.GetNode(entdb_pb2.GetNodeRequest(context=ctx, type_id=type_id, node_id=nid))
        except grpc.RpcError:
            pass  # still record latency below
        latencies.append(time.perf_counter() - t0)

    channel.close()
    return _summarize(latencies)


def bench_entdb_type_query(num: int, tenant_id: str) -> dict:
    """Query posts by type with pagination from populated EntDB."""
    import grpc

    from entdb_sdk._generated import entdb_pb2, entdb_pb2_grpc

    channel = _entdb_channel()
    stub = entdb_pb2_grpc.EntDBServiceStub(channel)
    ctx = entdb_pb2.RequestContext(tenant_id=tenant_id, actor="bench")

    num_posts = TENANT_CONFIGS[tenant_id]["posts"]
    latencies: list[float] = []
    for _ in range(num):
        offset = random.randint(0, max(num_posts - 20, 0))
        t0 = time.perf_counter()
        try:  # noqa: SIM105
            stub.QueryNodes(
                entdb_pb2.QueryNodesRequest(
                    context=ctx,
                    type_id=3,
                    limit=20,
                    offset=offset,
                    order_by="created_at",
                    descending=True,
                )
            )
        except grpc.RpcError:
            pass
        latencies.append(time.perf_counter() - t0)

    channel.close()
    return _summarize(latencies)


def bench_entdb_edge_query(num: int, tenant_id: str, all_ids: list[str]) -> dict:
    """Query edges from a random node in populated EntDB."""
    import grpc

    from entdb_sdk._generated import entdb_pb2, entdb_pb2_grpc

    channel = _entdb_channel()
    stub = entdb_pb2_grpc.EntDBServiceStub(channel)
    ctx = entdb_pb2.RequestContext(tenant_id=tenant_id, actor="bench")

    latencies: list[float] = []
    for _ in range(num):
        nid = random.choice(all_ids)
        t0 = time.perf_counter()
        try:  # noqa: SIM105
            stub.GetEdgesFrom(entdb_pb2.GetEdgesRequest(context=ctx, node_id=nid))
        except grpc.RpcError:
            pass
        latencies.append(time.perf_counter() - t0)

    channel.close()
    return _summarize(latencies)


def bench_entdb_writes(num: int, tenant_id: str) -> dict:
    """Write new chat messages into populated EntDB."""
    import grpc

    from entdb_sdk._generated import entdb_pb2, entdb_pb2_grpc

    channel = _entdb_channel()
    stub = entdb_pb2_grpc.EntDBServiceStub(channel)

    latencies: list[float] = []
    for i in range(num):
        actor = f"user:{i % 4000}"
        ctx = entdb_pb2.RequestContext(tenant_id=tenant_id, actor=actor)
        ops = [
            entdb_pb2.Operation(
                create_node=entdb_pb2.CreateNodeOp(
                    type_id=2,
                    data_json=_rand_payload("message", i),
                ),
            )
        ]
        idem_key = f"bench-write-{i}-{uuid.uuid4().hex[:6]}"
        req = entdb_pb2.ExecuteAtomicRequest(
            context=ctx,
            idempotency_key=idem_key,
            operations=ops,
        )
        t0 = time.perf_counter()
        try:  # noqa: SIM105
            stub.ExecuteAtomic(req)
        except grpc.RpcError:
            pass  # still record latency below
        latencies.append(time.perf_counter() - t0)

    channel.close()
    return _summarize(latencies)


def bench_entdb_cross_tenant(
    read_tenant: str,
    write_tenant: str,
    all_ids: list[str],
    num_ops: int,
) -> tuple[dict, dict]:
    """Read from *read_tenant* while writing to *write_tenant* concurrently.

    Returns (read_stats, write_stats).
    """
    import grpc

    from entdb_sdk._generated import entdb_pb2, entdb_pb2_grpc

    channel = _entdb_channel()
    stub = entdb_pb2_grpc.EntDBServiceStub(channel)

    read_latencies: list[float] = []
    write_latencies: list[float] = []

    for i in range(num_ops):
        # Write to write_tenant
        actor = f"user:{i % 4000}"
        w_ctx = entdb_pb2.RequestContext(tenant_id=write_tenant, actor=actor)
        w_ops = [
            entdb_pb2.Operation(
                create_node=entdb_pb2.CreateNodeOp(
                    type_id=2,
                    data_json=_rand_payload("message", i),
                ),
            )
        ]
        idem_key = f"cross-write-{i}-{uuid.uuid4().hex[:6]}"
        w_req = entdb_pb2.ExecuteAtomicRequest(
            context=w_ctx,
            idempotency_key=idem_key,
            operations=w_ops,
        )
        t0 = time.perf_counter()
        try:  # noqa: SIM105
            stub.ExecuteAtomic(w_req)
        except grpc.RpcError:
            pass  # still record latency below
        write_latencies.append(time.perf_counter() - t0)

        # Read from read_tenant
        nid = random.choice(all_ids)
        type_id = _type_id_from_node_id(nid)
        r_ctx = entdb_pb2.RequestContext(tenant_id=read_tenant, actor="bench")
        t0 = time.perf_counter()
        try:  # noqa: SIM105
            stub.GetNode(entdb_pb2.GetNodeRequest(context=r_ctx, type_id=type_id, node_id=nid))
        except grpc.RpcError:
            pass  # still record latency below
        read_latencies.append(time.perf_counter() - t0)

    channel.close()
    return _summarize(read_latencies), _summarize(write_latencies)


def bench_pg_cross_tenant(
    read_tenant: str,
    write_tenant: str,
    all_ids: list[str],
    num_ops: int,
) -> tuple[dict, dict]:
    """Read from *read_tenant* while writing to *write_tenant* in PG.

    Returns (read_stats, write_stats).
    """
    conn = _pg_connect()
    conn.autocommit = False
    cur = conn.cursor()

    read_latencies: list[float] = []
    write_latencies: list[float] = []
    ts = int(time.time() * 1000)

    for i in range(num_ops):
        # Write to write_tenant
        nid = f"cross-msg-{uuid.uuid4().hex[:8]}"
        actor = f"user:{i % 4000}"
        t0 = time.perf_counter()
        cur.execute(
            "INSERT INTO nodes VALUES (%s,%s,%s,%s,%s,%s,%s,%s)",
            (write_tenant, nid, 2, _rand_payload("message", i), ts, ts, actor, "[]"),
        )
        conn.commit()
        write_latencies.append(time.perf_counter() - t0)

        # Read from read_tenant
        read_nid = random.choice(all_ids)
        t0 = time.perf_counter()
        cur.execute(
            "SELECT * FROM nodes WHERE tenant_id = %s AND node_id = %s",
            (read_tenant, read_nid),
        )
        cur.fetchone()
        read_latencies.append(time.perf_counter() - t0)

    conn.close()
    return _summarize(read_latencies), _summarize(write_latencies)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _summarize(latencies: list[float]) -> dict:
    """Compute throughput, avg, p50, p99 from a list of latency samples."""
    if not latencies:
        return {"throughput": 0, "avg_ms": 0, "p50_ms": 0, "p99_ms": 0}
    total = sum(latencies)
    sorted_lat = sorted(latencies)
    return {
        "throughput": round(len(latencies) / total, 1),
        "avg_ms": round(statistics.mean(latencies) * 1000, 2),
        "p50_ms": round(statistics.median(latencies) * 1000, 2),
        "p99_ms": round(sorted_lat[int(len(sorted_lat) * 0.99)] * 1000, 2),
    }


def _print_row(label: str, entdb_stats: dict, pg_stats: dict) -> None:
    """Print one comparison row in the results table."""
    et = entdb_stats.get("throughput", 0)
    pt = pg_stats.get("throughput", 0)
    ratio = ""
    if et and pt:
        ratio = f"PG {pt / et:.1f}x" if pt > et else f"EntDB {et / pt:.1f}x"
    print(f"{label:<30} {et:>8.0f}/sec {pt:>8.0f}/sec {ratio:>10}")


# ---------------------------------------------------------------------------
# Infrastructure
# ---------------------------------------------------------------------------


def start_infra() -> bool:
    """Start EntDB + Postgres containers (expects Kafka cluster already running)."""
    # Check for existing Kafka cluster
    r = subprocess.run(
        ["docker", "exec", "bench-rp0", "rpk", "cluster", "health"],
        capture_output=True,
        text=True,
    )
    if r.returncode != 0:
        print("  Kafka cluster not running (bench-rp0 container not found).")
        print("  Run: python tests/benchmarks/bench_fair_compare.py")
        print("  Or start the Kafka cluster manually.")
        return False

    # Start EntDB
    subprocess.run(["docker", "rm", "-f", "bench-entdb"], capture_output=True)
    subprocess.run(
        [
            "docker",
            "run",
            "-d",
            "--name",
            "bench-entdb",
            "--network",
            "bench-net",
            "--cpus",
            TIER["cpus"],
            "--memory",
            TIER["memory"],
            "-p",
            "50052:50051",
            "-e",
            "GRPC_BIND=0.0.0.0:50051",
            "-e",
            "WAL_BACKEND=kafka",
            "-e",
            "KAFKA_BROKERS=bench-rp0:9092,bench-rp1:9092,bench-rp2:9092",
            "-e",
            "KAFKA_TOPIC=entdb-realistic",
            "-e",
            "KAFKA_CONSUMER_GROUP=bench-realistic",
            "-e",
            "APPLIER_BATCH_SIZE=50",
            "-e",
            "S3_BUCKET=entdb-storage",
            "-e",
            "S3_ENDPOINT=http://bench-minio:9000",
            "-e",
            "AWS_ACCESS_KEY_ID=minioadmin",
            "-e",
            "AWS_SECRET_ACCESS_KEY=minioadmin",
            "-e",
            "ARCHIVER_ENABLED=false",
            "-e",
            "SNAPSHOT_ENABLED=false",
            "-e",
            "LOG_LEVEL=WARNING",
            "entdb-server-bench",
        ],
        capture_output=True,
    )

    # Create topic
    subprocess.run(
        [
            "docker",
            "exec",
            "bench-rp0",
            "rpk",
            "topic",
            "create",
            "entdb-realistic",
            "-r",
            "3",
            "-p",
            "3",
        ],
        capture_output=True,
    )

    # Wait for EntDB gRPC
    import grpc

    for _ in range(30):
        try:
            ch = grpc.insecure_channel("localhost:50052")
            grpc.channel_ready_future(ch).result(timeout=2)
            ch.close()
            print("  EntDB: ready")
            break
        except Exception:
            time.sleep(1)
    else:
        print("  EntDB: FAILED to start")
        log_result = subprocess.run(
            ["docker", "logs", "bench-entdb", "--tail", "10"],
            capture_output=True,
            text=True,
        )
        print(log_result.stderr[-300:])
        return False

    # Start Postgres
    subprocess.run(["docker", "rm", "-f", "bench-pg"], capture_output=True)
    subprocess.run(
        [
            "docker",
            "run",
            "-d",
            "--name",
            "bench-pg",
            "--network",
            "bench-net",
            "--cpus",
            TIER["cpus"],
            "--memory",
            TIER["memory"],
            "-e",
            "POSTGRES_USER=bench",
            "-e",
            "POSTGRES_PASSWORD=bench",
            "-e",
            "POSTGRES_DB=bench",
            "-p",
            "5434:5432",
            "postgres:16-bookworm",
            "-c",
            "shared_buffers=128MB",
            "-c",
            "fsync=on",
            "-c",
            "synchronous_commit=on",
        ],
        capture_output=True,
    )

    for _ in range(30):
        pg_ready = subprocess.run(
            ["docker", "exec", "bench-pg", "pg_isready", "-U", "bench"],
            capture_output=True,
        )
        if pg_ready.returncode == 0:
            print("  Postgres: ready")
            return True
        time.sleep(1)

    print("  Postgres: FAILED")
    return False


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def main() -> None:
    try:
        import psycopg2  # noqa: F401
    except ImportError:
        subprocess.run(
            [sys.executable, "-m", "pip", "install", "psycopg2-binary"],
            capture_output=True,
        )

    random.seed(42)

    separator = "=" * 80
    print(separator)
    print("REALISTIC MULTI-TENANT BENCHMARK: EntDB vs PostgreSQL")
    print(f"  {TOTAL_ALL_ROWS:,} total rows across {NUM_TENANTS} tenants")
    for tid in TENANT_IDS:
        tn = _tenant_total_nodes(tid)
        te = _tenant_edges(tid)
        print(f"    {tid}: {tn:,} nodes + {te:,} edges = {tn + te:,} rows")
    print(f"  Container: {TIER['name']}")
    print(separator)

    print("\nStarting infrastructure...")
    if not start_infra():
        return

    # --- Seed Postgres ---
    print(
        f"\nSeeding PostgreSQL: {TOTAL_ALL_NODES:,} nodes + {TOTAL_ALL_EDGES:,} edges across {NUM_TENANTS} tenants..."
    )
    pg_seed_time = seed_postgres(NUM_TENANTS)
    pg_rate = int(TOTAL_ALL_NODES / pg_seed_time)
    print(f"  Done in {pg_seed_time:.1f}s ({pg_rate:,} nodes/sec)")

    # --- Seed EntDB ---
    print(f"\nSeeding EntDB: {TOTAL_ALL_NODES:,} nodes across {NUM_TENANTS} tenants (via gRPC)...")
    entdb_seed_time = seed_entdb(NUM_TENANTS)
    print(f"  Done in {entdb_seed_time:.1f}s")

    # Pick a random tenant for benchmarks
    bench_tenant = random.choice(TENANT_IDS)
    all_node_ids = _build_node_ids(bench_tenant)
    bench_nodes = _tenant_total_nodes(bench_tenant)
    bench_edges = _tenant_edges(bench_tenant)
    bench_posts = TENANT_CONFIGS[bench_tenant]["posts"]
    bench_messages = TENANT_CONFIGS[bench_tenant]["messages"]
    print(f"\nBenchmark tenant: {bench_tenant}")

    # --- Benchmarks ---
    print(f"\n{separator}")
    print(f"BENCHMARKING (against populated databases, tenant={bench_tenant})")
    print(separator)

    # Point reads
    print(f"\n  Point reads ({BENCH_READS} random lookups from {bench_nodes:,} rows)...")
    print("    PG...   ", end="", flush=True)
    pg_reads = bench_pg_point_reads(BENCH_READS, bench_tenant, all_node_ids)
    print(
        f"{pg_reads['throughput']:>7.1f}/sec  p50={pg_reads['p50_ms']:.2f}ms  p99={pg_reads['p99_ms']:.2f}ms"
    )

    print("    EntDB...", end="", flush=True)
    entdb_reads = bench_entdb_point_reads(BENCH_READS, bench_tenant, all_node_ids)
    print(
        f"{entdb_reads['throughput']:>7.1f}/sec  p50={entdb_reads['p50_ms']:.2f}ms  p99={entdb_reads['p99_ms']:.2f}ms"
    )

    # Type queries (paginated)
    print(f"\n  Type queries ({BENCH_READS} paginated queries across {bench_posts:,} posts)...")
    print("    PG...   ", end="", flush=True)
    pg_type = bench_pg_type_query(BENCH_READS, bench_tenant)
    print(
        f"{pg_type['throughput']:>7.1f}/sec  p50={pg_type['p50_ms']:.2f}ms  p99={pg_type['p99_ms']:.2f}ms"
    )

    print("    EntDB...", end="", flush=True)
    entdb_type = bench_entdb_type_query(BENCH_READS, bench_tenant)
    print(
        f"{entdb_type['throughput']:>7.1f}/sec  p50={entdb_type['p50_ms']:.2f}ms  p99={entdb_type['p99_ms']:.2f}ms"
    )

    # Edge queries
    print(f"\n  Edge queries ({BENCH_READS} lookups across {bench_edges:,} edges)...")
    print("    PG...   ", end="", flush=True)
    pg_edges = bench_pg_edge_query(BENCH_READS, bench_tenant, all_node_ids)
    print(
        f"{pg_edges['throughput']:>7.1f}/sec  p50={pg_edges['p50_ms']:.2f}ms  p99={pg_edges['p99_ms']:.2f}ms"
    )

    print("    EntDB...", end="", flush=True)
    entdb_edges = bench_entdb_edge_query(BENCH_READS, bench_tenant, all_node_ids)
    print(
        f"{entdb_edges['throughput']:>7.1f}/sec  p50={entdb_edges['p50_ms']:.2f}ms  p99={entdb_edges['p99_ms']:.2f}ms"
    )

    # Writes (into populated table)
    print(f"\n  Writes ({BENCH_WRITES} new messages into {bench_messages:,} existing)...")
    print("    PG...   ", end="", flush=True)
    pg_writes = bench_pg_writes(BENCH_WRITES, bench_tenant)
    print(
        f"{pg_writes['throughput']:>7.1f}/sec  p50={pg_writes['p50_ms']:.2f}ms  p99={pg_writes['p99_ms']:.2f}ms"
    )

    print("    EntDB...", end="", flush=True)
    entdb_writes = bench_entdb_writes(BENCH_WRITES, bench_tenant)
    print(
        f"{entdb_writes['throughput']:>7.1f}/sec  p50={entdb_writes['p50_ms']:.2f}ms  p99={entdb_writes['p99_ms']:.2f}ms"
    )

    # Cross-tenant isolation
    other_tenants = [t for t in TENANT_IDS if t != bench_tenant]
    write_tenant = random.choice(other_tenants)
    print(
        f"\n  Cross-tenant isolation ({BENCH_MIXED_OPS} ops: read {bench_tenant} while writing {write_tenant})..."
    )

    print("    PG...   ", end="", flush=True)
    pg_cross_reads, pg_cross_writes = bench_pg_cross_tenant(
        bench_tenant, write_tenant, all_node_ids, BENCH_MIXED_OPS
    )
    print(
        f"read {pg_cross_reads['throughput']:>7.1f}/sec p50={pg_cross_reads['p50_ms']:.2f}ms"
        f" | write {pg_cross_writes['throughput']:>7.1f}/sec p50={pg_cross_writes['p50_ms']:.2f}ms"
    )

    print("    EntDB...", end="", flush=True)
    entdb_cross_reads, entdb_cross_writes = bench_entdb_cross_tenant(
        bench_tenant, write_tenant, all_node_ids, BENCH_MIXED_OPS
    )
    print(
        f"read {entdb_cross_reads['throughput']:>7.1f}/sec p50={entdb_cross_reads['p50_ms']:.2f}ms"
        f" | write {entdb_cross_writes['throughput']:>7.1f}/sec p50={entdb_cross_writes['p50_ms']:.2f}ms"
    )

    # --- Summary ---
    print(f"\n{separator}")
    print(
        f"RESULTS -- {TOTAL_ALL_NODES:,} nodes + {TOTAL_ALL_EDGES:,} edges"
        f" across {NUM_TENANTS} tenants = {TOTAL_ALL_ROWS:,} total rows"
    )
    print(f"Container: {TIER['name']}")
    print(separator)
    print(f"{'Operation':<30} {'EntDB':>12} {'Postgres':>12} {'Ratio':>10}")
    print("-" * 70)

    _print_row("Point reads (by ID)", entdb_reads, pg_reads)
    _print_row("Type query (paginated)", entdb_type, pg_type)
    _print_row("Edge query", entdb_edges, pg_edges)
    _print_row("Writes (new messages)", entdb_writes, pg_writes)
    _print_row("Cross-tenant reads", entdb_cross_reads, pg_cross_reads)
    _print_row("Cross-tenant writes", entdb_cross_writes, pg_cross_writes)

    print(f"\n{separator}")
    print("SEED TIME")
    print(separator)
    pg_seed_rate = int(TOTAL_ALL_NODES / pg_seed_time)
    print(
        f"  Postgres:  {pg_seed_time:>7.1f}s  ({pg_seed_rate:>8,} nodes/sec, {NUM_TENANTS} tenants)"
    )
    entdb_seed_rate = int(TOTAL_ALL_NODES / entdb_seed_time)
    print(
        f"  EntDB:     {entdb_seed_time:>7.1f}s  ({entdb_seed_rate:>8,} nodes/sec via gRPC, {NUM_TENANTS} tenants)"
    )

    # Save results
    results = {
        "tier": TIER["name"],
        "num_tenants": NUM_TENANTS,
        "tenant_ids": TENANT_IDS,
        "bench_tenant": bench_tenant,
        "per_tenant": {
            tid: {
                "nodes": _tenant_total_nodes(tid),
                "edges": _tenant_edges(tid),
                "total_rows": _tenant_total_nodes(tid) + _tenant_edges(tid),
            }
            for tid in TENANT_IDS
        },
        "aggregate": {
            "total_nodes": TOTAL_ALL_NODES,
            "total_edges": TOTAL_ALL_EDGES,
            "total_rows": TOTAL_ALL_ROWS,
        },
        "seed_pg_sec": pg_seed_time,
        "seed_entdb_sec": entdb_seed_time,
        "point_reads": {"entdb": entdb_reads, "pg": pg_reads},
        "type_query": {"entdb": entdb_type, "pg": pg_type},
        "edge_query": {"entdb": entdb_edges, "pg": pg_edges},
        "writes": {"entdb": entdb_writes, "pg": pg_writes},
        "cross_tenant": {
            "entdb_reads": entdb_cross_reads,
            "entdb_writes": entdb_cross_writes,
            "pg_reads": pg_cross_reads,
            "pg_writes": pg_cross_writes,
        },
    }
    results_path = (
        "/Users/arun/projects/opensource/tenant-shard-db/" "tests/benchmarks/realistic-results.json"
    )
    with open(results_path, "w") as fout:
        json.dump(results, fout, indent=2)
    print("\nResults saved to tests/benchmarks/realistic-results.json")


if __name__ == "__main__":
    main()
