"""
Container-constrained benchmarks.

Runs the batch applier benchmark inside Docker containers with
resource limits matching common cloud tiers (Fly.io, Railway, etc).

Run:
  python tests/benchmarks/bench_container.py

This script is run directly (not via pytest) because it orchestrates
Docker containers externally.
"""
from __future__ import annotations

import json
import subprocess
import sys
import textwrap

# Fly.io tiers to benchmark
TIERS = [
    {"name": "shared-cpu-1x (256MB)", "cpus": "0.25", "memory": "256m", "label": "free"},
    {"name": "shared-cpu-1x (512MB)", "cpus": "0.25", "memory": "512m", "label": "small"},
    {"name": "shared-cpu-1x (1GB)", "cpus": "1", "memory": "1g", "label": "medium"},
    {"name": "performance-2x (2GB)", "cpus": "2", "memory": "2g", "label": "standard"},
    {"name": "performance-4x (4GB)", "cpus": "4", "memory": "4g", "label": "large"},
]

BATCH_SIZES = [1, 10, 50]
NUM_EVENTS = 500

# Inline benchmark script that runs inside the container
BENCH_SCRIPT = textwrap.dedent("""\
import asyncio
import json
import sys
import time

from dbaas.entdb_server.apply.applier import TransactionEvent
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.wal.memory import InMemoryWalStream


def make_event(i, payload_size=200):
    return json.dumps({
        "tenant_id": "bench",
        "actor": f"user:{i % 10}",
        "idempotency_key": f"event-{i}",
        "schema_fingerprint": None,
        "ts_ms": int(time.time() * 1000),
        "ops": [{
            "op": "create_node",
            "type_id": 1,
            "id": f"node-{i}",
            "data": {"name": f"Item {i}", "body": "x" * payload_size},
        }],
    }).encode()


async def run(num_events, batch_size):
    wal = InMemoryWalStream(num_partitions=1)
    await wal.connect()

    store = CanonicalStore(data_dir="/tmp/bench-data", wal_mode=True)
    await store.initialize_tenant("bench")

    topic = "bench-wal"
    for i in range(num_events):
        await wal.append(topic, "bench", make_event(i))

    applied = 0
    start = time.perf_counter()

    while applied < num_events:
        records = await wal.poll_batch(
            topic=topic,
            group_id="bench",
            max_records=batch_size,
            timeout_ms=50,
        )
        if not records:
            continue

        if batch_size <= 1:
            for record in records:
                data = record.value_json()
                event = TransactionEvent.from_dict(data, record.position)
                await store.create_node(
                    tenant_id=event.tenant_id,
                    type_id=event.ops[0]["type_id"],
                    payload=event.ops[0].get("data", {}),
                    owner_actor=event.actor,
                    node_id=event.ops[0].get("id"),
                    created_at=event.ts_ms,
                )
                applied += 1
        else:
            events = []
            for record in records:
                data = record.value_json()
                events.append(TransactionEvent.from_dict(data, record.position))

            with store.batch_transaction("bench") as conn:
                for event in events:
                    store.create_node_raw(
                        conn,
                        tenant_id=event.tenant_id,
                        type_id=event.ops[0]["type_id"],
                        payload=event.ops[0].get("data", {}),
                        owner_actor=event.actor,
                        node_id=event.ops[0].get("id"),
                        created_at=event.ts_ms,
                    )
                    applied += 1

        await wal.commit(records[-1])

    elapsed = time.perf_counter() - start
    await wal.close()

    return {
        "throughput": round(num_events / elapsed, 1),
        "elapsed": round(elapsed, 3),
        "latency_ms": round((elapsed / num_events) * 1000, 3),
    }


num_events = int(sys.argv[1])
batch_size = int(sys.argv[2])
result = asyncio.run(run(num_events, batch_size))
print(json.dumps(result))
""")


def build_bench_image():
    """Build a minimal Docker image for benchmarking."""
    print("Building benchmark image...")
    dockerfile = textwrap.dedent("""\
    FROM python:3.11-slim-bookworm
    WORKDIR /app
    COPY pyproject.toml .
    COPY dbaas/ dbaas/
    COPY sdk/ sdk/
    RUN pip install --no-cache-dir -e ".[server]" 2>/dev/null
    """)

    result = subprocess.run(
        ["docker", "build", "-t", "entdb-bench", "-f", "-", "."],
        input=dockerfile,
        capture_output=True,
        text=True,
        cwd="/Users/arun/projects/opensource/tenant-shard-db",
    )
    if result.returncode != 0:
        print(f"Build failed:\n{result.stderr}")
        sys.exit(1)
    print("Image built.\n")


def run_benchmark(tier: dict, batch_size: int, num_events: int) -> dict | None:
    """Run benchmark inside a constrained container."""
    cmd = [
        "docker", "run", "--rm",
        "--cpus", tier["cpus"],
        "--memory", tier["memory"],
        "-v", "/Users/arun/projects/opensource/tenant-shard-db:/app:ro",
        "-w", "/app",
        "entdb-bench",
        "python", "-c", BENCH_SCRIPT, str(num_events), str(batch_size),
    ]

    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=120,
        )
        if result.returncode != 0:
            print(f"  ERROR: {result.stderr.strip()[-200:]}")
            return None

        # Parse the last line as JSON
        output = result.stdout.strip().split("\n")[-1]
        return json.loads(output)

    except subprocess.TimeoutExpired:
        print("  TIMEOUT (>120s)")
        return None
    except json.JSONDecodeError:
        print(f"  Parse error: {result.stdout.strip()[-200:]}")
        return None


def main():
    build_bench_image()

    # Collect results
    all_results = []

    for tier in TIERS:
        print(f"--- {tier['name']} (cpus={tier['cpus']}, mem={tier['memory']}) ---")
        tier_results = {"tier": tier["name"], "label": tier["label"]}

        for bs in BATCH_SIZES:
            sys.stdout.write(f"  batch_size={bs:>3}... ")
            sys.stdout.flush()

            r = run_benchmark(tier, bs, NUM_EVENTS)
            if r:
                tier_results[f"bs{bs}"] = r
                print(f"{r['throughput']:>8.1f} events/sec  ({r['latency_ms']:.3f}ms/event)")
            else:
                tier_results[f"bs{bs}"] = {"throughput": 0, "elapsed": 0, "latency_ms": 0}
                print("FAILED")

        all_results.append(tier_results)
        print()

    # Print summary table
    print("=" * 90)
    print(f"CONTAINER BENCHMARK RESULTS ({NUM_EVENTS} events)")
    print("=" * 90)
    print(f"{'Tier':<30} {'CPU':>5} {'RAM':>5}", end="")
    for bs in BATCH_SIZES:
        print(f" {'bs=' + str(bs):>14}", end="")
    print(f" {'Best Speedup':>14}")
    print("-" * 90)

    for res in all_results:
        tier_info = next(t for t in TIERS if t["name"] == res["tier"])
        print(f"{res['tier']:<30} {tier_info['cpus']:>5} {tier_info['memory']:>5}", end="")

        baseline = res.get("bs1", {}).get("throughput", 1) or 1
        best_throughput = baseline

        for bs in BATCH_SIZES:
            r = res.get(f"bs{bs}", {})
            tp = r.get("throughput", 0)
            best_throughput = max(best_throughput, tp)
            print(f" {tp:>10.1f}/sec", end="")

        speedup = best_throughput / baseline if baseline > 0 else 0
        print(f" {speedup:>12.1f}x")

    print("=" * 90)

    # Print capacity assessment
    print("\nCAPACITY vs COLLEGE WORKLOAD (500 writes/sec peak)")
    print("-" * 60)
    for res in all_results:
        best = max(
            res.get(f"bs{bs}", {}).get("throughput", 0)
            for bs in BATCH_SIZES
        )
        headroom = best / 500 if best > 0 else 0
        status = "OK" if headroom >= 1.5 else "TIGHT" if headroom >= 1.0 else "NO"
        print(f"  {res['tier']:<30} {best:>8.0f}/sec  {headroom:>5.1f}x headroom  [{status}]")

    # Save raw results
    with open("/Users/arun/projects/opensource/tenant-shard-db/tests/benchmarks/container-results.json", "w") as f:
        json.dump(all_results, f, indent=2)
    print("\nRaw results saved to tests/benchmarks/container-results.json")


if __name__ == "__main__":
    main()
