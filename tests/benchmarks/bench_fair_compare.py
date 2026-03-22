"""
Fair comparison: EntDB vs PostgreSQL — same container specs.

Both database containers get identical CPU/memory limits.
Kafka runs separately (unconstrained, like any managed service would).
Client runs on the host for both.

EntDB:    Client → gRPC → EntDB container (CPU/mem limited) → Kafka (separate)
Postgres: Client → psycopg2 → Postgres container (same CPU/mem limits)

Run:
  python tests/benchmarks/bench_fair_compare.py
"""

from __future__ import annotations

import json
import statistics
import subprocess
import sys
import time
import uuid

TIERS = [
    {"name": "0.25 CPU / 512MB", "cpus": "0.25", "memory": "512m"},
    {"name": "1 CPU / 1GB", "cpus": "1", "memory": "1g"},
    {"name": "2 CPU / 2GB", "cpus": "2", "memory": "2g"},
]

ACKS_MODES = ["all", "1"]

NUM_WRITES = 1000
NUM_READS = 1000


# ---------------------------------------------------------------------------
# Infrastructure management
# ---------------------------------------------------------------------------


def stop_all():
    """Stop all benchmark containers and network."""
    for name in [
        "bench-entdb",
        "bench-rp0",
        "bench-rp1",
        "bench-rp2",
        "bench-minio",
        "bench-minio-init",
        "bench-pg",
    ]:
        subprocess.run(["docker", "rm", "-f", name], capture_output=True)
    subprocess.run(["docker", "network", "rm", "bench-net"], capture_output=True)


def start_kafka_cluster():
    """Start 3-node Redpanda cluster with replication factor 3."""
    # Create network
    subprocess.run(["docker", "network", "create", "bench-net"], capture_output=True)

    seeds = "bench-rp0:33145"
    image = "docker.redpanda.com/redpandadata/redpanda:v23.3.5"

    for node_id in range(3):
        name = f"bench-rp{node_id}"
        kafka_port = 19092 + node_id  # 19092, 19093, 19094
        subprocess.run(["docker", "rm", "-f", name], capture_output=True)

        cmd = [
            "docker",
            "run",
            "-d",
            "--name",
            name,
            "--network",
            "bench-net",
            "-p",
            f"{kafka_port}:9092",
            image,
            "redpanda",
            "start",
            "--smp=1",
            "--memory=256M",
            "--reserve-memory=0M",
            "--overprovisioned",
            f"--node-id={node_id}",
            "--kafka-addr=PLAINTEXT://0.0.0.0:9092",
            f"--advertise-kafka-addr=PLAINTEXT://{name}:9092",
            "--rpc-addr=0.0.0.0:33145",
            f"--advertise-rpc-addr={name}:33145",
        ]
        if node_id > 0:
            cmd.append(f"--seeds={seeds}")

        subprocess.run(cmd, capture_output=True)

    # Wait for cluster to form
    print("  Waiting for 3-node cluster...", end="", flush=True)
    for _attempt in range(45):
        r = subprocess.run(
            ["docker", "exec", "bench-rp0", "rpk", "cluster", "health"],
            capture_output=True,
            text=True,
        )
        if r.returncode == 0 and "true" in r.stdout.lower():
            break
        time.sleep(1)
    else:
        print(" TIMEOUT")
        r = subprocess.run(
            ["docker", "exec", "bench-rp0", "rpk", "cluster", "health"],
            capture_output=True,
            text=True,
        )
        print(f"  Health output: {r.stdout.strip()}")
        return False

    # Create topic with replication factor 3
    subprocess.run(
        [
            "docker",
            "exec",
            "bench-rp0",
            "rpk",
            "topic",
            "create",
            "entdb-bench",
            "-r",
            "3",
            "-p",
            "3",
        ],
        capture_output=True,
    )

    print(" OK (3 brokers, RF=3)")
    return True


def start_minio():
    """Start MinIO for EntDB snapshots/archives."""
    subprocess.run(["docker", "rm", "-f", "bench-minio", "bench-minio-init"], capture_output=True)
    import os

    subprocess.run(
        [
            "docker",
            "run",
            "-d",
            "--name",
            "bench-minio",
            "--network",
            "bench-net",
            "-e",
            "MINIO_ROOT_USER=minioadmin",
            "-e",
            "MINIO_ROOT_PASSWORD=minioadmin",
            "minio/minio:RELEASE.2024-01-18T22-51-28Z",
            "server",
            "/data",
            "--console-address",
            ":9001",
        ],
        capture_output=True,
        env=dict(os.environ),
    )
    time.sleep(2)
    # Create bucket
    subprocess.run(
        [
            "docker",
            "run",
            "--rm",
            "--network",
            "bench-net",
            "minio/mc:RELEASE.2024-01-18T07-03-39Z",
            "bash",
            "-c",
            "mc alias set m http://bench-minio:9000 minioadmin minioadmin && mc mb --ignore-existing m/entdb-storage",
        ],
        capture_output=True,
    )


def start_entdb(tier: dict, acks: str = "all"):
    """Start EntDB server with resource limits against 3-broker cluster."""
    subprocess.run(["docker", "rm", "-f", "bench-entdb"], capture_output=True)

    brokers = "bench-rp0:9092,bench-rp1:9092,bench-rp2:9092"
    # Use a unique topic per acks mode to avoid cross-contamination
    topic = f"entdb-bench-acks-{acks}"

    # Create topic with RF=3 if it doesn't exist
    subprocess.run(
        [
            "docker",
            "exec",
            "bench-rp0",
            "rpk",
            "topic",
            "create",
            topic,
            "-r",
            "3",
            "-p",
            "3",
        ],
        capture_output=True,
    )

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
            tier["cpus"],
            "--memory",
            tier["memory"],
            "-p",
            "50052:50051",
            "-e",
            "GRPC_BIND=0.0.0.0:50051",
            "-e",
            "WAL_BACKEND=kafka",
            "-e",
            f"KAFKA_BROKERS={brokers}",
            "-e",
            f"KAFKA_TOPIC={topic}",
            "-e",
            f"KAFKA_CONSUMER_GROUP=bench-applier-{acks}",
            "-e",
            f"KAFKA_ACKS={acks}",
            "-e",
            f"KAFKA_ENABLE_IDEMPOTENCE={'true' if acks == 'all' else 'false'}",
            "-e",
            "APPLIER_BATCH_SIZE=10",
            "-e",
            "S3_BUCKET=entdb-storage",
            "-e",
            "S3_ENDPOINT=http://bench-minio:9000",
            "-e",
            "S3_REGION=us-east-1",
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
            "-e",
            "LOG_FORMAT=text",
            "-e",
            "SCHEMA_FILE=/app/schema.yaml",
            "-v",
            "/Users/arun/projects/opensource/tenant-shard-db/schema.yaml:/app/schema.yaml:ro",
            "entdb-server-bench",
        ],
        capture_output=True,
    )

    # Wait for gRPC
    for _ in range(30):
        try:
            import grpc

            ch = grpc.insecure_channel("localhost:50052")
            grpc.channel_ready_future(ch).result(timeout=2)
            ch.close()
            return True
        except Exception:
            time.sleep(1)
    print("EntDB failed to start")
    # Print logs
    r = subprocess.run(
        ["docker", "logs", "bench-entdb", "--tail", "20"], capture_output=True, text=True
    )
    print(r.stdout[-500:] if r.stdout else "")
    print(r.stderr[-500:] if r.stderr else "")
    return False


def start_postgres(tier: dict):
    """Start Postgres with same resource limits as EntDB."""
    subprocess.run(["docker", "rm", "-f", "bench-pg"], capture_output=True)
    # Scale shared_buffers based on available memory
    mem = tier["memory"]
    shared_buf = "32MB" if mem in ("256m", "512m") else "64MB"

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
            tier["cpus"],
            "--memory",
            tier["memory"],
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
            f"shared_buffers={shared_buf}",
            "-c",
            "fsync=on",
            "-c",
            "synchronous_commit=on",
        ],
        capture_output=True,
    )

    for _ in range(30):
        r = subprocess.run(
            ["docker", "exec", "bench-pg", "pg_isready", "-U", "bench"],
            capture_output=True,
        )
        if r.returncode == 0:
            return True
        time.sleep(1)
    print("Postgres failed to start")
    return False


# ---------------------------------------------------------------------------
# Benchmarks
# ---------------------------------------------------------------------------


def bench_entdb_writes(num: int) -> dict:
    """Write through EntDB gRPC → Kafka → ACK."""
    import grpc

    from entdb_sdk._generated import entdb_pb2, entdb_pb2_grpc

    channel = grpc.insecure_channel("localhost:50052")
    stub = entdb_pb2_grpc.EntDBServiceStub(channel)

    tid = f"bench-{uuid.uuid4().hex[:8]}"
    latencies = []

    for i in range(num):
        ctx = entdb_pb2.RequestContext(tenant_id=tid, actor="bench")
        ops = [
            entdb_pb2.Operation(
                create_node=entdb_pb2.CreateNodeOp(
                    type_id=1,
                    data_json=json.dumps({"name": f"Item {i}", "body": "x" * 200}),
                ),
            )
        ]
        req = entdb_pb2.ExecuteAtomicRequest(
            context=ctx,
            idempotency_key=f"w-{i}-{uuid.uuid4().hex[:6]}",
            operations=ops,
        )
        start = time.perf_counter()
        try:
            stub.ExecuteAtomic(req)
        except grpc.RpcError as e:
            print(f"    gRPC error: {e.code()} {e.details()}")
            continue
        latencies.append(time.perf_counter() - start)

    channel.close()
    return _summarize(latencies, num)


def bench_entdb_reads(num: int) -> dict:
    """Read through EntDB gRPC → SQLite."""
    import grpc

    from entdb_sdk._generated import entdb_pb2, entdb_pb2_grpc

    channel = grpc.insecure_channel("localhost:50052")
    stub = entdb_pb2_grpc.EntDBServiceStub(channel)

    # Seed
    tid = f"bench-r-{uuid.uuid4().hex[:8]}"
    node_ids = []
    for i in range(min(num, 50)):
        ctx = entdb_pb2.RequestContext(tenant_id=tid, actor="bench")
        ops = [
            entdb_pb2.Operation(
                create_node=entdb_pb2.CreateNodeOp(
                    type_id=1,
                    data_json=json.dumps({"name": f"Seed {i}"}),
                ),
            )
        ]
        try:
            resp = stub.ExecuteAtomic(
                entdb_pb2.ExecuteAtomicRequest(
                    context=ctx,
                    idempotency_key=f"seed-{i}-{uuid.uuid4().hex[:6]}",
                    operations=ops,
                )
            )
            if resp.created_node_ids:
                node_ids.append(resp.created_node_ids[0])
        except grpc.RpcError:
            pass

    time.sleep(2)  # Wait for applier

    if not node_ids:
        return {"throughput": 0, "avg_ms": 0, "p50_ms": 0, "p99_ms": 0}

    latencies = []
    for i in range(num):
        ctx = entdb_pb2.RequestContext(tenant_id=tid, actor="bench")
        req = entdb_pb2.GetNodeRequest(
            context=ctx,
            type_id=1,
            node_id=node_ids[i % len(node_ids)],
        )
        start = time.perf_counter()
        try:
            stub.GetNode(req)
        except grpc.RpcError:
            continue
        latencies.append(time.perf_counter() - start)

    channel.close()
    return _summarize(latencies, num)


def bench_pg_writes(num: int) -> dict:
    """Write directly to Postgres."""
    import psycopg2

    conn = psycopg2.connect(
        host="localhost", port=5434, user="bench", password="bench", dbname="bench"
    )
    conn.autocommit = False
    cur = conn.cursor()

    cur.execute("""
        CREATE TABLE IF NOT EXISTS nodes (
            tenant_id TEXT NOT NULL, node_id TEXT NOT NULL,
            type_id INTEGER NOT NULL, payload_json TEXT NOT NULL DEFAULT '{}',
            created_at BIGINT NOT NULL, updated_at BIGINT NOT NULL,
            owner_actor TEXT NOT NULL, acl_blob TEXT NOT NULL DEFAULT '[]',
            PRIMARY KEY (tenant_id, node_id))
    """)
    cur.execute("CREATE INDEX IF NOT EXISTS idx_type ON nodes (tenant_id, type_id)")
    cur.execute("TRUNCATE nodes")
    conn.commit()

    tid = f"pg-{uuid.uuid4().hex[:8]}"
    ts = int(time.time() * 1000)
    latencies = []

    for i in range(num):
        start = time.perf_counter()
        cur.execute(
            "INSERT INTO nodes VALUES (%s,%s,%s,%s,%s,%s,%s,%s)",
            (
                tid,
                f"node-{i}",
                1,
                json.dumps({"name": f"Item {i}", "body": "x" * 200}),
                ts,
                ts,
                f"user:{i % 10}",
                "[]",
            ),
        )
        conn.commit()
        latencies.append(time.perf_counter() - start)

    conn.close()
    return _summarize(latencies, num)


def bench_pg_reads(num: int) -> dict:
    """Read from Postgres by primary key."""
    import psycopg2

    conn = psycopg2.connect(
        host="localhost", port=5434, user="bench", password="bench", dbname="bench"
    )
    cur = conn.cursor()
    conn.autocommit = False

    tid = f"pg-r-{uuid.uuid4().hex[:8]}"
    ts = int(time.time() * 1000)
    for i in range(min(num, 50)):
        cur.execute(
            "INSERT INTO nodes VALUES (%s,%s,%s,%s,%s,%s,%s,%s)",
            (tid, f"node-{i}", 1, json.dumps({"name": f"Seed {i}"}), ts, ts, "user", "[]"),
        )
    conn.commit()
    conn.autocommit = True

    latencies = []
    for i in range(num):
        nid = f"node-{i % min(num, 50)}"
        start = time.perf_counter()
        cur.execute("SELECT * FROM nodes WHERE tenant_id = %s AND node_id = %s", (tid, nid))
        cur.fetchone()
        latencies.append(time.perf_counter() - start)

    conn.close()
    return _summarize(latencies, num)


def _summarize(latencies: list[float], total: int) -> dict:
    if not latencies:
        return {"throughput": 0, "avg_ms": 0, "p50_ms": 0, "p99_ms": 0}
    s = sum(latencies)
    return {
        "throughput": round(len(latencies) / s, 1),
        "avg_ms": round(statistics.mean(latencies) * 1000, 2),
        "p50_ms": round(statistics.median(latencies) * 1000, 2),
        "p99_ms": round(sorted(latencies)[int(len(latencies) * 0.99)] * 1000, 2),
    }


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def main():
    try:
        import psycopg2  # noqa: F401
    except ImportError:
        subprocess.run(
            [sys.executable, "-m", "pip", "install", "psycopg2-binary"], capture_output=True
        )

    print("=" * 90)
    print("FAIR COMPARISON: EntDB vs PostgreSQL (same container specs)")
    print("  3-node Redpanda cluster (RF=3), testing acks=all and acks=1")
    print("=" * 90)
    print()
    print("  EntDB container:    CPU/mem limited, gRPC server + SQLite")
    print("  Postgres container: same CPU/mem limits, direct SQL")
    print("  Kafka:              3-node Redpanda cluster, replication factor 3")
    print("  Client:             runs on host for both")
    print()

    # Start shared infra
    print("Starting 3-node Redpanda cluster...")
    stop_all()
    if not start_kafka_cluster():
        return
    start_minio()
    print()

    # Build EntDB image once
    print("Building EntDB image...")
    subprocess.run(
        ["docker", "build", "-t", "entdb-server-bench", "--target", "server", "-q", "."],
        capture_output=True,
        cwd="/Users/arun/projects/opensource/tenant-shard-db",
    )
    print("Done.\n")

    results = []

    for tier in TIERS:
        print(f"{'='*80}")
        print(f"TIER: {tier['name']}")
        print(f"{'='*80}")

        tier_result = {"tier": tier["name"]}

        # --- EntDB with acks=all and acks=1 ---
        for acks in ACKS_MODES:
            label = f"entdb_acks_{acks}"
            print(f"  Starting EntDB (acks={acks})...", end="", flush=True)
            if not start_entdb(tier, acks=acks):
                print(" FAILED")
                tier_result[f"{label}_writes"] = {}
                tier_result[f"{label}_reads"] = {}
            else:
                print(" OK")

                print("    writes...", end="", flush=True)
                ew = bench_entdb_writes(NUM_WRITES)
                tier_result[f"{label}_writes"] = ew
                print(
                    f"  {ew['throughput']:>7.1f}/sec  p50={ew['p50_ms']:.2f}ms  p99={ew['p99_ms']:.2f}ms"
                )

                print("    reads... ", end="", flush=True)
                er = bench_entdb_reads(NUM_READS)
                tier_result[f"{label}_reads"] = er
                print(
                    f"  {er['throughput']:>7.1f}/sec  p50={er['p50_ms']:.2f}ms  p99={er['p99_ms']:.2f}ms"
                )

            subprocess.run(["docker", "rm", "-f", "bench-entdb"], capture_output=True)

        # --- Postgres ---
        print("  Starting PG...  ", end="", flush=True)
        if not start_postgres(tier):
            print(" FAILED")
            tier_result["pg_writes"] = {}
            tier_result["pg_reads"] = {}
        else:
            print(" OK")
            try:
                print("    writes...", end="", flush=True)
                pw = bench_pg_writes(NUM_WRITES)
                tier_result["pg_writes"] = pw
                print(
                    f"  {pw['throughput']:>7.1f}/sec  p50={pw['p50_ms']:.2f}ms  p99={pw['p99_ms']:.2f}ms"
                )

                print("    reads... ", end="", flush=True)
                pr = bench_pg_reads(NUM_READS)
                tier_result["pg_reads"] = pr
                print(
                    f"  {pr['throughput']:>7.1f}/sec  p50={pr['p50_ms']:.2f}ms  p99={pr['p99_ms']:.2f}ms"
                )
            except Exception as e:
                print(f" ERROR: {e}")
                tier_result["pg_writes"] = tier_result.get("pg_writes", {})
                tier_result["pg_reads"] = {}

        subprocess.run(["docker", "rm", "-f", "bench-pg"], capture_output=True)
        results.append(tier_result)
        print()

    # Cleanup
    stop_all()

    # Summary — Writes
    print(f"\n{'='*95}")
    print("WRITES (events/sec) — 3-node Redpanda cluster, RF=3, 1000 events")
    print(f"{'='*95}")
    print(
        f"{'Tier':<22} {'acks=all':>12} {'acks=1':>12} {'Postgres':>12}  {'all p50':>8} {'1 p50':>8} {'PG p50':>8}"
    )
    print("-" * 95)
    for r in results:
        ea = r.get("entdb_acks_all_writes", {})
        e1 = r.get("entdb_acks_1_writes", {})
        pw = r.get("pg_writes", {})
        print(
            f"{r['tier']:<22}"
            f" {ea.get('throughput', 0):>8.0f}/sec"
            f" {e1.get('throughput', 0):>8.0f}/sec"
            f" {pw.get('throughput', 0):>8.0f}/sec"
            f"  {ea.get('p50_ms', 0):>6.2f}ms"
            f" {e1.get('p50_ms', 0):>6.2f}ms"
            f" {pw.get('p50_ms', 0):>6.2f}ms"
        )

    # Summary — Reads
    print(f"\n{'='*70}")
    print("READS (queries/sec)")
    print(f"{'='*70}")
    print(f"{'Tier':<22} {'EntDB':>12} {'Postgres':>12}  {'EntDB p50':>10} {'PG p50':>8}")
    print("-" * 70)
    for r in results:
        er = r.get("entdb_acks_all_reads", r.get("entdb_acks_1_reads", {}))
        pr = r.get("pg_reads", {})
        print(
            f"{r['tier']:<22}"
            f" {er.get('throughput', 0):>8.0f}/sec"
            f" {pr.get('throughput', 0):>8.0f}/sec"
            f"  {er.get('p50_ms', 0):>8.2f}ms"
            f" {pr.get('p50_ms', 0):>6.2f}ms"
        )

    with open(
        "/Users/arun/projects/opensource/tenant-shard-db/tests/benchmarks/fair-compare-results.json",
        "w",
    ) as f:
        json.dump(results, f, indent=2)
    print("\nResults saved to tests/benchmarks/fair-compare-results.json")


if __name__ == "__main__":
    main()
