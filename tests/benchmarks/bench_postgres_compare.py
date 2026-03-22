"""
PostgreSQL vs EntDB (SQLite) comparison benchmark.

Runs equivalent CRUD operations against both PostgreSQL and SQLite
under the same container resource constraints.

Run:
  python tests/benchmarks/bench_postgres_compare.py

Requires Docker.
"""
from __future__ import annotations

import json
import subprocess
import sys
import textwrap
import time

TIERS = [
    {"name": "shared-cpu-1x (256MB)", "cpus": "0.25", "memory": "256m"},
    {"name": "shared-cpu-1x (1GB)", "cpus": "1", "memory": "1g"},
    {"name": "performance-2x (2GB)", "cpus": "2", "memory": "2g"},
]

BATCH_SIZES = [1, 10, 50]
NUM_EVENTS = 500

# ---------------------------------------------------------------------------
# SQLite benchmark (runs inside entdb-bench container)
# ---------------------------------------------------------------------------

SQLITE_SCRIPT = textwrap.dedent("""\
import asyncio, json, sys, time
from dbaas.entdb_server.apply.applier import TransactionEvent
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.wal.memory import InMemoryWalStream

def make_event(i):
    return json.dumps({
        "tenant_id": "bench", "actor": f"user:{i%10}",
        "idempotency_key": f"event-{i}", "schema_fingerprint": None,
        "ts_ms": int(time.time()*1000),
        "ops": [{"op":"create_node","type_id":1,"id":f"node-{i}",
                 "data":{"name":f"Item {i}","body":"x"*200}}],
    }).encode()

async def run(n, bs):
    wal = InMemoryWalStream(num_partitions=1)
    await wal.connect()
    store = CanonicalStore(data_dir="/tmp/bench", wal_mode=True)
    await store.initialize_tenant("bench")
    for i in range(n):
        await wal.append("t", "bench", make_event(i))

    applied = 0
    start = time.perf_counter()
    while applied < n:
        records = await wal.poll_batch(topic="t", group_id="b", max_records=bs, timeout_ms=50)
        if not records:
            continue
        if bs <= 1:
            for r in records:
                e = TransactionEvent.from_dict(r.value_json(), r.position)
                await store.create_node(tenant_id=e.tenant_id, type_id=1,
                    payload=e.ops[0].get("data",{}), owner_actor=e.actor,
                    node_id=e.ops[0].get("id"), created_at=e.ts_ms)
                applied += 1
        else:
            evts = [TransactionEvent.from_dict(r.value_json(), r.position) for r in records]
            with store.batch_transaction("bench") as conn:
                for e in evts:
                    store.create_node_raw(conn, tenant_id=e.tenant_id, type_id=1,
                        payload=e.ops[0].get("data",{}), owner_actor=e.actor,
                        node_id=e.ops[0].get("id"), created_at=e.ts_ms)
                    applied += 1
        await wal.commit(records[-1])

    elapsed = time.perf_counter() - start

    # Read benchmark
    read_start = time.perf_counter()
    for i in range(min(n, 200)):
        await store.get_node("bench", f"node-{i}")
    read_elapsed = time.perf_counter() - read_start
    reads = min(n, 200)

    await wal.close()
    print(json.dumps({
        "write_throughput": round(n/elapsed, 1),
        "write_latency_ms": round((elapsed/n)*1000, 3),
        "read_throughput": round(reads/read_elapsed, 1),
        "read_latency_ms": round((read_elapsed/reads)*1000, 3),
    }))

asyncio.run(run(int(sys.argv[1]), int(sys.argv[2])))
""")

# ---------------------------------------------------------------------------
# PostgreSQL benchmark (runs psycopg2 against a Postgres container)
# ---------------------------------------------------------------------------

PG_BENCH_SCRIPT = textwrap.dedent("""\
import json, sys, time, uuid
import psycopg2
from psycopg2.extras import execute_values

n = int(sys.argv[1])
bs = int(sys.argv[2])

conn = psycopg2.connect(host="localhost", port=5433, user="bench", password="bench", dbname="bench")
conn.autocommit = False
cur = conn.cursor()

# Create schema matching EntDB's canonical store
cur.execute('''
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
''')
cur.execute('CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes (tenant_id, type_id)')
cur.execute('TRUNCATE nodes')
conn.commit()

# Write benchmark
ts = int(time.time() * 1000)
applied = 0
start = time.perf_counter()

if bs <= 1:
    for i in range(n):
        cur.execute(
            "INSERT INTO nodes (tenant_id, node_id, type_id, payload_json, created_at, updated_at, owner_actor, acl_blob) VALUES (%s,%s,%s,%s,%s,%s,%s,%s)",
            ("bench", f"node-{i}", 1, json.dumps({"name": f"Item {i}", "body": "x"*200}), ts, ts, f"user:{i%10}", "[]")
        )
        conn.commit()
        applied += 1
else:
    batch = []
    for i in range(n):
        batch.append(("bench", f"node-{i}", 1, json.dumps({"name": f"Item {i}", "body": "x"*200}), ts, ts, f"user:{i%10}", "[]"))
        if len(batch) >= bs:
            execute_values(cur,
                "INSERT INTO nodes (tenant_id, node_id, type_id, payload_json, created_at, updated_at, owner_actor, acl_blob) VALUES %s",
                batch)
            conn.commit()
            applied += len(batch)
            batch = []
    if batch:
        execute_values(cur,
            "INSERT INTO nodes (tenant_id, node_id, type_id, payload_json, created_at, updated_at, owner_actor, acl_blob) VALUES %s",
            batch)
        conn.commit()
        applied += len(batch)

write_elapsed = time.perf_counter() - start

# Read benchmark
read_start = time.perf_counter()
reads = min(n, 200)
for i in range(reads):
    cur.execute("SELECT * FROM nodes WHERE tenant_id = %s AND node_id = %s", ("bench", f"node-{i}"))
    cur.fetchone()
read_elapsed = time.perf_counter() - read_start

conn.close()
print(json.dumps({
    "write_throughput": round(n/write_elapsed, 1),
    "write_latency_ms": round((write_elapsed/n)*1000, 3),
    "read_throughput": round(reads/read_elapsed, 1),
    "read_latency_ms": round((read_elapsed/reads)*1000, 3),
}))
""")


def build_images():
    """Build benchmark images."""
    print("Building EntDB benchmark image...")
    subprocess.run(
        ["docker", "build", "-t", "entdb-bench", "-f", "-", "."],
        input="FROM python:3.11-slim-bookworm\nWORKDIR /app\nCOPY pyproject.toml .\nCOPY dbaas/ dbaas/\nCOPY sdk/ sdk/\nRUN pip install --no-cache-dir -e '.[server]' 2>/dev/null\n",
        capture_output=True, text=True,
        cwd="/Users/arun/projects/opensource/tenant-shard-db",
    )
    print("Done.")


def start_postgres(tier: dict) -> str:
    """Start a Postgres container with resource limits. Returns container ID."""
    # Stop any existing bench-pg
    subprocess.run(["docker", "rm", "-f", "bench-pg"], capture_output=True)

    cmd = [
        "docker", "run", "-d",
        "--name", "bench-pg",
        "--cpus", tier["cpus"],
        "--memory", tier["memory"],
        "-e", "POSTGRES_USER=bench",
        "-e", "POSTGRES_PASSWORD=bench",
        "-e", "POSTGRES_DB=bench",
        "-p", "5433:5432",
        # Tune for benchmarks
        "-e", "POSTGRES_INITDB_ARGS=--data-checksums",
        "postgres:16-bookworm",
        "-c", "shared_buffers=64MB",
        "-c", "fsync=on",
        "-c", "synchronous_commit=on",
        "-c", "max_connections=20",
    ]
    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode != 0:
        print(f"Failed to start Postgres: {result.stderr}")
        return ""

    # Wait for Postgres to be ready
    for _ in range(30):
        check = subprocess.run(
            ["docker", "exec", "bench-pg", "pg_isready", "-U", "bench"],
            capture_output=True, text=True,
        )
        if check.returncode == 0:
            return result.stdout.strip()
        time.sleep(1)

    print("Postgres failed to start in 30s")
    return ""


def stop_postgres():
    subprocess.run(["docker", "rm", "-f", "bench-pg"], capture_output=True)


def run_sqlite_bench(tier: dict, batch_size: int) -> dict | None:
    """Run SQLite benchmark in constrained container."""
    cmd = [
        "docker", "run", "--rm",
        "--cpus", tier["cpus"],
        "--memory", tier["memory"],
        "-v", "/Users/arun/projects/opensource/tenant-shard-db:/app:ro",
        "-w", "/app",
        "entdb-bench",
        "python", "-c", SQLITE_SCRIPT, str(NUM_EVENTS), str(batch_size),
    ]
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=120)
        if result.returncode != 0:
            return None
        return json.loads(result.stdout.strip().split("\n")[-1])
    except Exception:
        return None


def run_postgres_bench(batch_size: int) -> dict | None:
    """Run Postgres benchmark (client runs on host, PG in container)."""
    try:
        result = subprocess.run(
            [sys.executable, "-c", PG_BENCH_SCRIPT, str(NUM_EVENTS), str(batch_size)],
            capture_output=True, text=True, timeout=120,
            cwd="/Users/arun/projects/opensource/tenant-shard-db",
        )
        if result.returncode != 0:
            print(f"    PG error: {result.stderr.strip()[-200:]}")
            return None
        return json.loads(result.stdout.strip().split("\n")[-1])
    except Exception as e:
        print(f"    PG exception: {e}")
        return None


def main():
    # Check psycopg2
    try:
        import psycopg2  # noqa: F401
    except ImportError:
        print("Installing psycopg2-binary...")
        subprocess.run([sys.executable, "-m", "pip", "install", "psycopg2-binary"],
                       capture_output=True)

    build_images()

    results = []

    for tier in TIERS:
        print(f"\n{'='*70}")
        print(f"TIER: {tier['name']} (cpus={tier['cpus']}, mem={tier['memory']})")
        print(f"{'='*70}")

        # Start Postgres with same limits
        print("Starting PostgreSQL...")
        start_postgres(tier)
        time.sleep(2)  # Extra settle time

        tier_result = {"tier": tier["name"]}

        for bs in BATCH_SIZES:
            print(f"\n  batch_size={bs}:")

            # SQLite
            sys.stdout.write("    SQLite:   ")
            sys.stdout.flush()
            sq = run_sqlite_bench(tier, bs)
            if sq:
                print(f"writes={sq['write_throughput']:>8.1f}/sec  reads={sq['read_throughput']:>8.1f}/sec")
                tier_result[f"sqlite_bs{bs}"] = sq
            else:
                print("FAILED")
                tier_result[f"sqlite_bs{bs}"] = {}

            # Postgres
            sys.stdout.write("    Postgres: ")
            sys.stdout.flush()
            pg = run_postgres_bench(bs)
            if pg:
                print(f"writes={pg['write_throughput']:>8.1f}/sec  reads={pg['read_throughput']:>8.1f}/sec")
                tier_result[f"pg_bs{bs}"] = pg
            else:
                print("FAILED")
                tier_result[f"pg_bs{bs}"] = {}

        results.append(tier_result)
        stop_postgres()

    # Summary table
    print(f"\n\n{'='*90}")
    print("SQLITE vs POSTGRESQL — WRITE THROUGHPUT (events/sec)")
    print(f"{'='*90}")
    print(f"{'Tier':<28}", end="")
    for bs in BATCH_SIZES:
        print(f" {'SQLite bs='+str(bs):>14} {'PG bs='+str(bs):>14}", end="")
    print()
    print("-" * 90)

    for res in results:
        print(f"{res['tier']:<28}", end="")
        for bs in BATCH_SIZES:
            sq = res.get(f"sqlite_bs{bs}", {}).get("write_throughput", 0)
            pg = res.get(f"pg_bs{bs}", {}).get("write_throughput", 0)
            print(f" {sq:>10.0f}/sec {pg:>10.0f}/sec", end="")
        print()

    print(f"\n{'='*90}")
    print("SQLITE vs POSTGRESQL — READ THROUGHPUT (reads/sec)")
    print(f"{'='*90}")
    print(f"{'Tier':<28}", end="")
    for _bs in BATCH_SIZES:
        print(f" {'SQLite':>14} {'PG':>14}", end="")
    print()
    print("-" * 90)

    for res in results:
        print(f"{res['tier']:<28}", end="")
        for bs in BATCH_SIZES:
            sq = res.get(f"sqlite_bs{bs}", {}).get("read_throughput", 0)
            pg = res.get(f"pg_bs{bs}", {}).get("read_throughput", 0)
            print(f" {sq:>10.0f}/sec {pg:>10.0f}/sec", end="")
        print()

    print(f"\n{'='*90}")
    print("WINNER PER TIER (best batch size, write throughput)")
    print(f"{'='*90}")
    for res in results:
        best_sq = max(res.get(f"sqlite_bs{bs}", {}).get("write_throughput", 0) for bs in BATCH_SIZES)
        best_pg = max(res.get(f"pg_bs{bs}", {}).get("write_throughput", 0) for bs in BATCH_SIZES)
        if best_sq > 0 and best_pg > 0:
            ratio = best_sq / best_pg
            winner = "SQLite" if ratio > 1 else "PostgreSQL"
            print(f"  {res['tier']:<28} SQLite={best_sq:>8.0f}  PG={best_pg:>8.0f}  → {winner} {max(ratio, 1/ratio):.1f}x faster")
        else:
            print(f"  {res['tier']:<28} incomplete data")

    # Save
    with open("/Users/arun/projects/opensource/tenant-shard-db/tests/benchmarks/pg-compare-results.json", "w") as f:
        json.dump(results, f, indent=2)
    print("\nRaw results saved to tests/benchmarks/pg-compare-results.json")


if __name__ == "__main__":
    main()
