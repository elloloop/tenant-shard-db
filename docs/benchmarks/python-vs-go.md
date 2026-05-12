# Python vs Go server: dual-backend benchmark comparison

**Status:** Phase 4B of EPIC #407 (pre-Python-deletion sanity check).
**Methodology:** `tests/python/benchmarks/bench_grpc_dual.py` runs the
same gRPC wire-level test code against both backends, switched via the
`ENTDB_SERVER_TARGET={python,go}` env var. Raw JSON is committed under
`.benchmarks/{python,go}/`.

The bench is intentionally focused on the **wire path**: a sync gRPC
client (one channel per test, reused across rounds) calls the server's
RPC and pytest-benchmark times the round-trip. Both backends use the
in-memory WAL so Kafka/Redpanda topology is excluded from the
comparison (see Caveats).

## Hardware / environment

Two runs are committed so trends are visible across environments:

| Tag                    | OS / CPU                              | Resource limits          | Container       |
|------------------------|---------------------------------------|--------------------------|-----------------|
| `_docker_constrained`  | Debian 12 in Docker (host: macOS)     | `--cpus=1 --memory=512m` | `golang:1.25`   |
| `_host_macos_arm64`    | macOS 25.2 / Apple Silicon            | none                     | host process    |

The `_docker_constrained` numbers are the reproducible "small CI
runner" emulation that informs the Phase 4D decision; the `_host_*`
numbers are the local-dev sanity check. The CI workflow
(`.github/workflows/benchmarks.yml`) runs the constrained variant on
Linux runners — there's no virtualization-layer distortion there
(unlike Docker Desktop on macOS).

To reproduce:

```bash
# host (no limits)
tests/python/benchmarks/run_dual_backend.sh

# constrained (--cpus=1 --memory=512m inside a linux container)
tests/python/benchmarks/run_dual_backend.sh --constrained
```

## Results — Docker constrained (`--cpus=1 --memory=512m`, 10 rounds)

Median (p50) and 95th-percentile latency per RPC, microseconds:

| RPC                                | Python p50 | Python p95 | Go p50 | Go p95 | Go/Py p50 | Narrative                                                |
|------------------------------------|-----------:|-----------:|-------:|-------:|----------:|----------------------------------------------------------|
| `Health`                           |    250.3   |    334.2   |  124.5 |  145.9 |   **0.50x** | Go cuts plain-channel dispatch overhead in half.        |
| `GetNode` (single)                 |    407.5   |    490.0   |  177.0 |  340.2 |   **0.43x** | Go ~2.3x faster on single point reads.                  |
| `QueryNodes` (limit=10)            |    407.8   |    611.1   |  200.1 |  283.2 |   **0.49x** | Go ~2x faster; p95 gap is wider.                         |
| `GetNodes` (batch of 10, 1 hit)    |   1117.0   |   1513.7   |  297.2 |  523.2 |   **0.27x** | Largest read-side gap; Go ~3.7x faster.                  |
| `ExecuteAtomic` (write + applied)  |   1271.1   |   2527.2   |  279.5 |  435.7 |   **0.22x** | Write path: Go ~4.5x faster end-to-end.                  |

## Results — Host (macOS arm64, 30 rounds, no resource limits)

| RPC                                | Python p50 | Python p95 | Go p50 | Go p95 | Go/Py p50 |
|------------------------------------|-----------:|-----------:|-------:|-------:|----------:|
| `Health`                           |    172.7   |    537.2   |   69.8 |  170.6 |   **0.40x** |
| `GetNode` (single)                 |    307.0   |    449.1   |  116.6 |  152.6 |   **0.38x** |
| `QueryNodes` (limit=10)            |    348.7   |    855.4   |  207.5 |  647.0 |   **0.59x** |
| `GetNodes` (batch of 10, 1 hit)    |    957.6   |   1432.3   |  256.5 |  623.9 |   **0.27x** |
| `ExecuteAtomic` (write + applied)  |    955.2   |   1350.7   |  168.8 |  568.6 |   **0.18x** |

Both environments show the **same shape**: Go is 2-5x faster on
every operation, with the widest gap on the write path (which exercises
the WAL append + applier consume loop + ack) and on batched reads.

## Summary

* Go is faster than Python on every measured wire-level operation —
  no surprises, no Go regressions vs Python.
* The Python applier + Python gRPC handler combination is the dominant
  cost on the write path; the Go server's single-threaded scheduler
  plus pure-Go SQLite (`modernc.org/sqlite`) clears the same path in
  ~25% of the time.
* No metric is dramatically **slower** on Go. **Not a Phase 4 blocker.**

## Caveats

* **In-memory WAL only.** Kafka/Redpanda fan-in is not in the loop
  here. A WAL-backed comparison needs a separate matrix (Redpanda + a
  durability assertion); deferred until Phase 4D's go/no-go.
* **Same-host client + server.** The client runs on the test runner,
  the server on the same Linux box (Docker) or the same macOS host.
  Network latency is loopback; cloud latency will dominate over the
  Python/Go gap on real deployments.
* **`Health` may be `UNIMPLEMENTED` on some Go waves.** The bench
  treats `UNIMPLEMENTED` as a successful RTT and keeps timing — both
  backends pay the same dispatch cost in that case.
* **macOS Docker Desktop adds a VM layer.** The host numbers above are
  on Apple Silicon natively; the constrained numbers go through Docker
  Desktop's lightweight VM, which adds variance not present on a
  Linux CI runner. CI-collected numbers (artifact uploads via the
  `Benchmarks` workflow) are the canonical comparison surface.
* **Python-internals benchmarks are not in this comparison.** The
  legacy pytest-benchmark suite (`bench_acl.py`, `bench_crud.py`,
  `bench_batch_applier.py`, …) measures `CanonicalStore` and
  `Applier` Python objects directly. There is no Go equivalent of
  those entry points, so the bench conftest auto-skips them when
  `ENTDB_SERVER_TARGET=go`. The wire-level comparison
  (`bench_grpc_dual.py`) is the only suite that's apples-to-apples
  cross-backend.

## Raw data

Each pytest-benchmark JSON includes `machine_info.entdb_server_target`
indicating which backend produced it. The naming convention is:

```
.benchmarks/<target>/<UTC-timestamp>_<env-tag>.json
```

Where `<env-tag>` is `docker_constrained` (CI-equivalent) or
`host_<os>_<arch>` (local dev).
