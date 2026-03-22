"""
EntDB (full stack) vs PostgreSQL — End-to-End Benchmark.

Compares what the CLIENT experiences:
  EntDB:    SDK → gRPC → Kafka WAL → ACK (writes)
            SDK → gRPC → SQLite → response (reads)
  Postgres: psycopg2 → SQL → Postgres → ACK/response

Run:
  # Start stack first: docker compose up -d --build server
  # Start PG: docker run -d --name bench-pg -p 5433:5432 -e POSTGRES_USER=bench -e POSTGRES_PASSWORD=bench -e POSTGRES_DB=bench postgres:16
  python tests/benchmarks/bench_entdb_vs_pg.py
"""
from __future__ import annotations

import asyncio
import json
import statistics
import sys
import time
import uuid

NUM_EVENTS = 200
BATCH_SIZES = [1, 10, 50]


# ---------------------------------------------------------------------------
# EntDB benchmark (full stack via SDK)
# ---------------------------------------------------------------------------

async def bench_entdb_writes(num: int) -> dict:
    """Benchmark EntDB writes through the full gRPC → Kafka → ACK path."""
    import grpc

    from entdb_sdk._generated import entdb_pb2, entdb_pb2_grpc

    channel = grpc.insecure_channel("localhost:50051")
    stub = entdb_pb2_grpc.EntDBServiceStub(channel)

    tenant_id = f"bench-{uuid.uuid4().hex[:8]}"
    latencies = []

    for i in range(num):
        ctx = entdb_pb2.RequestContext(
            tenant_id=tenant_id,
            actor=f"user:{i % 10}",
        )
        operations = [
            entdb_pb2.Operation(
                create_node=entdb_pb2.CreateNodeOp(
                    type_id=1,
                    data_json=json.dumps({"name": f"Item {i}", "body": "x" * 200}),
                ),
            )
        ]

        request = entdb_pb2.ExecuteAtomicRequest(
            context=ctx,
            idempotency_key=f"bench-write-{i}-{uuid.uuid4().hex[:8]}",
            operations=operations,
        )

        start = time.perf_counter()
        try:
            stub.ExecuteAtomic(request)
        except grpc.RpcError:
            continue
        elapsed = time.perf_counter() - start
        latencies.append(elapsed)

    channel.close()

    if not latencies:
        return {"throughput": 0, "p50_ms": 0, "p99_ms": 0, "avg_ms": 0}

    total = sum(latencies)
    return {
        "throughput": round(len(latencies) / total, 1),
        "avg_ms": round(statistics.mean(latencies) * 1000, 2),
        "p50_ms": round(statistics.median(latencies) * 1000, 2),
        "p99_ms": round(sorted(latencies)[int(len(latencies) * 0.99)] * 1000, 2),
    }


async def bench_entdb_reads(num: int, tenant_id: str = "playground") -> dict:
    """Benchmark EntDB reads through gRPC → SQLite path."""
    import grpc

    from entdb_sdk._generated import entdb_pb2, entdb_pb2_grpc

    channel = grpc.insecure_channel("localhost:50051")
    stub = entdb_pb2_grpc.EntDBServiceStub(channel)

    # First create some nodes to read
    write_tenant = f"bench-read-{uuid.uuid4().hex[:8]}"
    node_ids = []
    for i in range(min(num, 50)):
        ctx = entdb_pb2.RequestContext(
            tenant_id=write_tenant,
            actor="bench-user",
        )
        operations = [
            entdb_pb2.Operation(
                create_node=entdb_pb2.CreateNodeOp(
                    type_id=1,
                    data_json=json.dumps({"name": f"Read Item {i}"}),
                ),
            )
        ]
        request = entdb_pb2.ExecuteAtomicRequest(
            context=ctx,
            idempotency_key=f"bench-seed-{i}-{uuid.uuid4().hex[:8]}",
            operations=operations,
        )
        try:
            resp = stub.ExecuteAtomic(request)
            if resp.created_node_ids:
                node_ids.append(resp.created_node_ids[0])
        except grpc.RpcError:
            pass

    # Wait for applier to process
    time.sleep(1)

    if not node_ids:
        channel.close()
        return {"throughput": 0, "p50_ms": 0, "p99_ms": 0, "avg_ms": 0}

    # Now benchmark reads
    latencies = []
    for i in range(num):
        node_id = node_ids[i % len(node_ids)]
        ctx = entdb_pb2.RequestContext(
            tenant_id=write_tenant,
            actor="bench-user",
        )
        request = entdb_pb2.GetNodeRequest(
            context=ctx,
            type_id=1,
            node_id=node_id,
        )
        start = time.perf_counter()
        try:
            stub.GetNode(request)
        except grpc.RpcError:
            continue
        elapsed = time.perf_counter() - start
        latencies.append(elapsed)

    channel.close()

    if not latencies:
        return {"throughput": 0, "p50_ms": 0, "p99_ms": 0, "avg_ms": 0}

    total = sum(latencies)
    return {
        "throughput": round(len(latencies) / total, 1),
        "avg_ms": round(statistics.mean(latencies) * 1000, 2),
        "p50_ms": round(statistics.median(latencies) * 1000, 2),
        "p99_ms": round(sorted(latencies)[int(len(latencies) * 0.99)] * 1000, 2),
    }


# ---------------------------------------------------------------------------
# PostgreSQL benchmark (direct SQL)
# ---------------------------------------------------------------------------

def bench_pg_writes_single(num: int) -> dict:
    """Benchmark Postgres writes — one INSERT + COMMIT per event."""
    import psycopg2

    conn = psycopg2.connect(host="localhost", port=5433, user="bench", password="bench", dbname="bench")
    conn.autocommit = False
    cur = conn.cursor()

    cur.execute("""
        CREATE TABLE IF NOT EXISTS nodes (
            tenant_id TEXT NOT NULL,
            node_id TEXT NOT NULL,
            type_id INTEGER NOT NULL,
            payload_json TEXT NOT NULL DEFAULT '{}',
            created_at BIGINT NOT NULL,
            updated_at BIGINT NOT NULL,
            owner_actor TEXT NOT NULL,
            acl_blob TEXT NOT NULL DEFAULT '[]',
            PRIMARY KEY (tenant_id, node_id)
        )
    """)
    cur.execute("CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes (tenant_id, type_id)")
    cur.execute("TRUNCATE nodes")
    conn.commit()

    ts = int(time.time() * 1000)
    tid = f"pg-bench-{uuid.uuid4().hex[:8]}"
    latencies = []

    for i in range(num):
        start = time.perf_counter()
        cur.execute(
            "INSERT INTO nodes VALUES (%s,%s,%s,%s,%s,%s,%s,%s)",
            (tid, f"node-{i}", 1, json.dumps({"name": f"Item {i}", "body": "x" * 200}),
             ts, ts, f"user:{i % 10}", "[]"),
        )
        conn.commit()
        elapsed = time.perf_counter() - start
        latencies.append(elapsed)

    conn.close()

    total = sum(latencies)
    return {
        "throughput": round(len(latencies) / total, 1),
        "avg_ms": round(statistics.mean(latencies) * 1000, 2),
        "p50_ms": round(statistics.median(latencies) * 1000, 2),
        "p99_ms": round(sorted(latencies)[int(len(latencies) * 0.99)] * 1000, 2),
    }


def bench_pg_writes_batched(num: int, batch_size: int) -> dict:
    """Benchmark Postgres writes — batched INSERTs."""
    import psycopg2
    from psycopg2.extras import execute_values

    conn = psycopg2.connect(host="localhost", port=5433, user="bench", password="bench", dbname="bench")
    conn.autocommit = False
    cur = conn.cursor()

    cur.execute("""
        CREATE TABLE IF NOT EXISTS nodes (
            tenant_id TEXT NOT NULL,
            node_id TEXT NOT NULL,
            type_id INTEGER NOT NULL,
            payload_json TEXT NOT NULL DEFAULT '{}',
            created_at BIGINT NOT NULL,
            updated_at BIGINT NOT NULL,
            owner_actor TEXT NOT NULL,
            acl_blob TEXT NOT NULL DEFAULT '[]',
            PRIMARY KEY (tenant_id, node_id)
        )
    """)
    cur.execute("TRUNCATE nodes")
    conn.commit()

    ts = int(time.time() * 1000)
    tid = f"pg-batch-{uuid.uuid4().hex[:8]}"

    start = time.perf_counter()
    batch = []
    for i in range(num):
        batch.append((tid, f"node-{i}", 1,
                       json.dumps({"name": f"Item {i}", "body": "x" * 200}),
                       ts, ts, f"user:{i % 10}", "[]"))
        if len(batch) >= batch_size:
            execute_values(cur,
                "INSERT INTO nodes (tenant_id,node_id,type_id,payload_json,created_at,updated_at,owner_actor,acl_blob) VALUES %s",
                batch)
            conn.commit()
            batch = []
    if batch:
        execute_values(cur,
            "INSERT INTO nodes (tenant_id,node_id,type_id,payload_json,created_at,updated_at,owner_actor,acl_blob) VALUES %s",
            batch)
        conn.commit()

    elapsed = time.perf_counter() - start
    conn.close()

    return {
        "throughput": round(num / elapsed, 1),
        "avg_ms": round((elapsed / num) * 1000, 2),
        "p50_ms": 0,  # Can't measure per-event in batch mode
        "p99_ms": 0,
    }


def bench_pg_reads(num: int) -> dict:
    """Benchmark Postgres reads — SELECT by primary key."""
    import psycopg2

    conn = psycopg2.connect(host="localhost", port=5433, user="bench", password="bench", dbname="bench")
    cur = conn.cursor()

    # Seed data
    tid = f"pg-read-{uuid.uuid4().hex[:8]}"
    conn.autocommit = False
    ts = int(time.time() * 1000)
    for i in range(min(num, 50)):
        cur.execute(
            "INSERT INTO nodes VALUES (%s,%s,%s,%s,%s,%s,%s,%s)",
            (tid, f"node-{i}", 1, json.dumps({"name": f"Read Item {i}"}),
             ts, ts, "user", "[]"),
        )
    conn.commit()
    conn.autocommit = True

    latencies = []
    for i in range(num):
        nid = f"node-{i % min(num, 50)}"
        start = time.perf_counter()
        cur.execute("SELECT * FROM nodes WHERE tenant_id = %s AND node_id = %s", (tid, nid))
        cur.fetchone()
        elapsed = time.perf_counter() - start
        latencies.append(elapsed)

    conn.close()

    total = sum(latencies)
    return {
        "throughput": round(len(latencies) / total, 1),
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
        import subprocess
        subprocess.run([sys.executable, "-m", "pip", "install", "psycopg2-binary"], capture_output=True)

    print("=" * 80)
    print("EntDB (Full Stack) vs PostgreSQL — End-to-End Benchmark")
    print("=" * 80)
    print()
    print("EntDB path:  SDK → gRPC → Kafka append → ACK (write)")
    print("             SDK → gRPC → SQLite query → response (read)")
    print("Postgres:    psycopg2 → SQL → Postgres → ACK/response")
    print()

    # --- WRITES ---
    print("-" * 80)
    print("WRITES (single event, commit per event)")
    print("-" * 80)

    print("  EntDB...", end="", flush=True)
    entdb_w1 = asyncio.run(bench_entdb_writes(NUM_EVENTS))
    print(f"  {entdb_w1['throughput']:>7.1f}/sec  avg={entdb_w1['avg_ms']:.2f}ms  p50={entdb_w1['p50_ms']:.2f}ms  p99={entdb_w1['p99_ms']:.2f}ms")

    print("  PG...   ", end="", flush=True)
    pg_w1 = bench_pg_writes_single(NUM_EVENTS)
    print(f"  {pg_w1['throughput']:>7.1f}/sec  avg={pg_w1['avg_ms']:.2f}ms  p50={pg_w1['p50_ms']:.2f}ms  p99={pg_w1['p99_ms']:.2f}ms")

    print()
    print("-" * 80)
    print("WRITES (batched)")
    print("-" * 80)

    for bs in [10, 50]:
        print(f"\n  batch_size={bs}:")
        print("    EntDB:  (writes always go through Kafka — batch_size affects applier, not client)")

        pg_wb = bench_pg_writes_batched(NUM_EVENTS, bs)
        print(f"    PG:     {pg_wb['throughput']:>7.1f}/sec  avg={pg_wb['avg_ms']:.2f}ms/event")

    # --- READS ---
    print()
    print("-" * 80)
    print("READS (get by ID)")
    print("-" * 80)

    print("  EntDB...", end="", flush=True)
    entdb_r = asyncio.run(bench_entdb_reads(NUM_EVENTS))
    print(f"  {entdb_r['throughput']:>7.1f}/sec  avg={entdb_r['avg_ms']:.2f}ms  p50={entdb_r['p50_ms']:.2f}ms  p99={entdb_r['p99_ms']:.2f}ms")

    print("  PG...   ", end="", flush=True)
    pg_r = bench_pg_reads(NUM_EVENTS)
    print(f"  {pg_r['throughput']:>7.1f}/sec  avg={pg_r['avg_ms']:.2f}ms  p50={pg_r['p50_ms']:.2f}ms  p99={pg_r['p99_ms']:.2f}ms")

    # --- SUMMARY ---
    print()
    print("=" * 80)
    print("SUMMARY")
    print("=" * 80)
    print(f"{'':20} {'EntDB':>12} {'PostgreSQL':>12} {'Winner':>15}")
    print("-" * 60)

    print(f"{'Write throughput':<20} {entdb_w1['throughput']:>8.0f}/sec {pg_w1['throughput']:>8.0f}/sec", end="")
    if entdb_w1['throughput'] > pg_w1['throughput']:
        print(f"  {'EntDB':>10} {entdb_w1['throughput']/pg_w1['throughput']:.1f}x")
    else:
        print(f"  {'PG':>10} {pg_w1['throughput']/entdb_w1['throughput']:.1f}x")

    print(f"{'Write p50 latency':<20} {entdb_w1['p50_ms']:>7.2f}ms {pg_w1['p50_ms']:>7.2f}ms", end="")
    if entdb_w1['p50_ms'] < pg_w1['p50_ms']:
        print(f"  {'EntDB':>10} {pg_w1['p50_ms']/entdb_w1['p50_ms']:.1f}x")
    else:
        print(f"  {'PG':>10} {entdb_w1['p50_ms']/pg_w1['p50_ms']:.1f}x")

    print(f"{'Read throughput':<20} {entdb_r['throughput']:>8.0f}/sec {pg_r['throughput']:>8.0f}/sec", end="")
    if entdb_r['throughput'] > pg_r['throughput']:
        print(f"  {'EntDB':>10} {entdb_r['throughput']/pg_r['throughput']:.1f}x")
    else:
        print(f"  {'PG':>10} {pg_r['throughput']/entdb_r['throughput']:.1f}x")

    print(f"{'Read p50 latency':<20} {entdb_r['p50_ms']:>7.2f}ms {pg_r['p50_ms']:>7.2f}ms", end="")
    if entdb_r['p50_ms'] < pg_r['p50_ms']:
        print(f"  {'EntDB':>10} {pg_r['p50_ms']/entdb_r['p50_ms']:.1f}x")
    else:
        print(f"  {'PG':>10} {entdb_r['p50_ms']/pg_r['p50_ms']:.1f}x")

    print()
    print("NOTE: EntDB writes go through Kafka (durable WAL) before ACK.")
    print("      PostgreSQL writes go directly to the database.")
    print("      EntDB provides event sourcing, multi-tenant isolation,")
    print("      and recovery-from-stream that PostgreSQL does not.")


if __name__ == "__main__":
    main()
