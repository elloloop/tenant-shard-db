# Shared Port Spec — Cross-Implementation Test Harness

EPIC #407 — Python -> Go server port. The Go server MUST pass the **same**
Python contract tests that gate the Python server today. This spec
defines how the existing pytest suite is reused against a Go binary
without forking the test code.

## Reading list (sources)

- `tests/python/integration/test_grpc_contract.py` — 41 RPC × ~70 cases
  (CONTRACT_CASES list around `:195`, fixture `live_server` at `:84`).
- `tests/python/integration/test_wal_replay_determinism.py` — replay,
  idempotency, halt-on-poison (3 P0 properties).
- `tests/python/integration/test_concurrent_applier_reads.py` — reader
  vs applier safety (`:60-`).
- `tests/python/unit/test_grpc_wire_format.py` — id-keyed payload,
  4 MiB cap, op-order independence (`:73-` `server_with_default_limits`).
- `tests/python/integration/test_privilege_escalation.py` — 34 cases,
  asserts handlers consult `get_authoritative_actor`, not request body.
- Python entrypoint: `server/python/entdb_server/main.py:1`.
- Python config (env vars): `server/python/entdb_server/config.py:125`
  (gRPC), `:454` (storage), `:708` (WAL backend selection).

## Current Python harness — how the test server is started

Today, **there is no `conftest.py` for `tests/python/integration/`**
(verified: only `tests/python/benchmarks/conftest.py` and
`tests/python/e2e/conftest.py` exist). Each test that needs a live gRPC
server constructs one in-process with its own fixture:

- **Contract suite** — `live_server` fixture
  (`test_grpc_contract.py:84-171`): builds `SchemaRegistry`,
  `GlobalStore` on a `TemporaryDirectory`, `InMemoryWalStream`,
  `CanonicalStore(wal_mode=False)`, an `Applier` task with
  `MailboxFanoutConfig(enabled=False)` and `batch_size=1`, then a bare
  `grpc.aio.server()` bound to a free 127.0.0.1 port (`_free_port`,
  `:56`). Auth interceptor is **not installed** — handlers run with no
  `_current_identity` set; happy-path RPCs trust whatever
  `RequestContext.actor` claims.
- **Wire-format suite** — `server_with_default_limits`
  (`test_grpc_wire_format.py:73-`): same shape as above but uses the
  bare `grpc_aio.server()` (no 50 MiB receive override) so the
  oversized-payload test exercises the protocol-default 4 MiB cap.
- **WAL replay & concurrent reader suites** — no gRPC at all. They
  drive `Applier` + `CanonicalStore` directly against
  `InMemoryWalStream` (`test_wal_replay_determinism.py:39`,
  `test_concurrent_applier_reads.py:35`). These are **applier-internal**
  contracts, not wire contracts; the Go port runs them via a separate
  Go-native test (see "Per-RPC gating" below).
- **Privilege-escalation suite** — `EntDBServicer` is constructed
  in-process with mock collaborators (`test_privilege_escalation.py:62`)
  and called through a `_FakeContext` (`:79`). The trusted identity is
  injected via `set_current_identity` (`auth_interceptor.py`), so this
  test never touches a socket.

Tear-down is per-fixture: `server.stop(grace=0)`, `applier.stop()`,
`applier_task.cancel()`, `wal.close()`, `global_store.close()`,
`TemporaryDirectory` exits.

## What needs to change

Add `tests/python/integration/conftest.py` with a single
`grpc_endpoint` fixture (returns `"127.0.0.1:<port>"`). Existing
fixtures that build their own in-process server are refactored to call
this fixture instead. Selection is via env var:

```
ENTDB_SERVER_TARGET=python   # default, in-process Python (today's path)
ENTDB_SERVER_TARGET=go       # subprocess: $ENTDB_GO_BINARY
```

Behaviour:

- `python` -> existing in-process spin-up moves into the conftest;
  fixtures now `yield port` from there.
- `go` -> `subprocess.Popen([$ENTDB_GO_BINARY, ...flags])` bound to a
  `_free_port()`-allocated TCP port; conftest waits for `Health` to
  return `healthy=true` (10 s timeout, 50 ms poll), yields, then
  `proc.terminate()` + `proc.wait(5)` on teardown. stdout/stderr
  captured to a per-test file under `pytest`'s `tmp_path` and printed
  on failure.
- Seed data (the `seeded-node` row) is created by the conftest after
  spin-up via the same `ExecuteAtomic` call that lives in
  `test_grpc_contract.py:131-154` today — moves with the fixture.
- Suites that don't talk to gRPC (replay, concurrent applier,
  privilege-escalation) are **not** parameterised by target. They stay
  Python-only and a Go-native equivalent lives under `tests/go/`
  (separate file, not in scope here).

## Go binary contract

The harness expects `$ENTDB_GO_BINARY` to honour these flags / env
vars. Defaults must match the Python in-process fixture so the same
contract assertions hold.

| Flag / env                    | Required | Notes                                                       |
| ----------------------------- | -------- | ----------------------------------------------------------- |
| `--listen 127.0.0.1:<port>`   | yes      | Mirrors `GRPC_BIND` (`config.py:125`).                      |
| `--wal-backend memory`        | yes      | In-memory WAL — equivalent of `InMemoryWalStream`.          |
| `--data-dir <path>`           | yes      | Per-tenant SQLite dir; harness passes a `tmp_path`.         |
| `--auth-disabled`             | yes      | No `AuthInterceptor` installed; matches today's fixture.    |
| `--mailbox-fanout=false`      | yes      | Mirrors `MailboxFanoutConfig(enabled=False)`.               |
| `--batch-size 1`              | yes      | Mirrors `Applier(..., batch_size=1)`.                       |
| `--max-message-bytes`         | no       | Omit -> 4 MiB default (wire-format test depends on this).   |
| `--seed-tenant acme`          | yes      | Pre-creates tenant + alice (owner) + bob (member).          |
| `--ready-probe`               | yes      | Process exits non-zero if `Health` not servable in 5 s.     |
| `ENTDB_LOG_LEVEL`             | no       | Defaults to `info`; harness sets `debug` on `-v`.           |

The binary MUST NOT require any other env var (no Kafka brokers, no
S3, no KMS). Anything else the test fixture needs (schema registration)
is done over the wire after start-up — the registry is global to the
process and the fixture registers `User`, `Task`, `AssignedTo` via
`GetSchema` + an internal `RegisterSchema` admin RPC (already covered
by Python in-process today; for Go the binary must pre-register the
same registry when `--seed-tenant` is set, since that registry is what
the contract suite assumes — see `_build_registry` at
`test_grpc_contract.py:64`).

## CI matrix

Add to `.github/workflows/ci.yml` the `integration-tests` job (around
`ci.yml:113-`):

```
integration-tests:
  strategy:
    fail-fast: false
    matrix:
      server: [python, go]
  env:
    ENTDB_SERVER_TARGET: ${{ matrix.server }}
```

For `server: go`:

1. `actions/setup-go@v5` with the version pinned in
   `sdk/go/entdb/go.mod` (Go SDK already exists).
2. Build step: `go build -o /tmp/entdb-server ./server/go/cmd/entdb-server`
   (this path is the **first deliverable** of the Go port — until it
   exists the matrix leg is `continue-on-error: true`, see Phase 0
   below).
3. `ENTDB_GO_BINARY=/tmp/entdb-server` exported into the test step.
4. Cache: `actions/cache@v4` keyed on
   `hashFiles('server/go/**/*.go','server/go/go.sum')` -> caches both
   `~/.cache/go-build` and `~/go/pkg/mod`.

Gating: the `all-checks` aggregator (`ci.yml:225-`) MUST require the
`server: go` leg only after Phase 2 (parity reached). Phase 0 / 1 leave
it at `continue-on-error: true` so red Go runs don't block Python PRs.

Tests skipped on the Go side use `pytest.mark.skipif(...)` driven by
the registry below — they show as `skipped`, not failures, in the
matrix output.

## Per-RPC gating

Test source MUST stay implementation-agnostic. Gating lives in **one**
file: `tests/python/integration/_go_parity.py` — a registry of RPC
names known to be implemented by the Go binary at `HEAD`:

```
GO_IMPLEMENTED: frozenset[str] = frozenset({
    "Health",
    # add as RPCs land
})
```

`conftest.py` exposes a single helper:

```
def requires_target(rpc: str) -> pytest.MarkDecorator:
    if os.environ.get("ENTDB_SERVER_TARGET") == "go" and rpc not in GO_IMPLEMENTED:
        return pytest.mark.skip(reason=f"{rpc}: not yet implemented in Go")
    return pytest.mark.usefixtures()  # no-op
```

The contract test (`test_grpc_contract.py`) is parametrised over
`CONTRACT_CASES`; the param-id is the `rpc` field. A
`pytest_collection_modifyitems` hook in conftest reads the case's `rpc`
out of the param-id and applies `requires_target(rpc)` automatically.
**No edits to `CONTRACT_CASES`**, no skip decorators sprinkled across
71 cases, no fork of the test file. Adding an RPC to the Go side is a
one-line change to `GO_IMPLEMENTED`.

Wire-format suite (`test_grpc_wire_format.py`): the four invariants
are RPC-agnostic protocol properties — they run unconditionally on
both targets once `Health` works (the suite calls `ExecuteAtomic` and
`GetNode`, so those two RPCs must be in `GO_IMPLEMENTED` before this
suite runs against Go; until then, mark the whole module
`pytestmark = requires_target("ExecuteAtomic")`).

## Phase progression

Aligned with the EPIC #407 wave plan:

- **Wave 0 — harness** (this doc): land conftest + `_go_parity.py` +
  CI matrix (with `continue-on-error` on the Go leg). Python tests must
  remain green; Go leg is allowed to be all-skipped.
- **Wave 1 — Health is the proof-of-life**: `Health` RPC in Go,
  `GO_IMPLEMENTED = {"Health"}`. CI matrix Go leg now runs **one**
  contract case green; everything else skipped. This is the smallest
  signal that subprocess spin-up, port allocation, ready-probe, and
  metadata propagation all work end-to-end.
- **Wave 2 — read RPCs** (`GetNode`, `GetNodes`, `GetSchema`,
  `QueryNodes`, ...): added to `GO_IMPLEMENTED` one PR at a time. Each
  PR ships the Go handler **and** the registry entry in the same diff.
- **Wave 3 — write RPCs** (`ExecuteAtomic` and friends): unlocks the
  wire-format suite against Go.
- **Wave 4 — flip gate**: when `GO_IMPLEMENTED` covers all 41 RPCs,
  drop `continue-on-error` on the Go matrix leg in `ci.yml` and add
  it to the `all-checks` `needs:` list. Parity reached.

## Dependencies

- **Python harness**: `pytest>=8.0.0`, `pytest-asyncio>=0.23.0`,
  `grpcio>=1.60.0` (already in `ci.yml:96-99`). No new pip deps.
- **Go binary**: `server/go/cmd/entdb-server/main.go` (does not exist
  yet — Wave 1 deliverable). Must be a single static binary; no `cgo`
  required for the in-memory WAL + temp SQLite test mode (use
  `modernc.org/sqlite` to keep `CGO_ENABLED=0`).
- **CI runner**: stock `ubuntu-latest`. No Docker for the matrix
  (subprocess only); the existing `e2e-tests` job covers the Docker
  path and stays Python-only.

## Open questions / risks

1. **Go build cache** — first-run `go build` on a cold cache is ~40 s
   on `ubuntu-latest`; caching `~/.cache/go-build` brings it to ~5 s.
   If the Go server grows large deps (e.g. `confluent-kafka-go`), we
   may need a separate `go-build` job that uploads the binary as an
   artifact and the test job downloads it.
2. **Subprocess port allocation** — `_free_port()`'s bind-then-close
   trick (`test_grpc_contract.py:56`) has a TOCTOU window: another
   process can grab the port between close and the Go subprocess'
   `Listen`. Acceptable for serial CI; under `pytest-xdist -n auto`
   we will see flakes. Mitigation: add a 3-attempt retry around
   subprocess spin-up in conftest (rebind on `address already in use`).
3. **Auth-disabled mode** — `--auth-disabled` is a test-only flag.
   Risk: it ships in production builds. Mitigation: gate behind a
   build tag (`//go:build testharness`) so a release binary lacks the
   flag entirely. The CI test build adds `-tags=testharness`; the
   release workflow does not.
4. **Diverging behaviour triage** — when one impl passes and the other
   fails on the same case, the failure is almost certainly Go-side
   (Python is the reference). Convention: open a Go-port bug tagged
   `port-divergence`, link the failing case-id, and **do not** weaken
   the Python assertion. The contract is the contract; the Go binary
   moves to it, never the reverse.
5. **In-process vs subprocess perf gap** — Go subprocess adds ~80 ms
   start-up per test session; multiplied by ~80 cases that's not free.
   Use a **session-scoped** `grpc_endpoint` fixture for the contract
   suite (Python's existing fixture is already session-shaped via the
   single seeded node) and reset state between cases via tenant scope,
   not by restarting the process.
6. **Schema registration on the Go side** — the Python fixture mutates
   a process-global `_global_registry` (`test_grpc_contract.py:87-91`).
   The Go binary cannot share that process. Open question: do we
   expose a `RegisterSchema` admin RPC, or hard-code the contract
   registry into the `--seed-tenant` path? Leaning toward the latter
   for Wave 1 (less surface), promote to an RPC in Wave 3 once write
   paths are in.
