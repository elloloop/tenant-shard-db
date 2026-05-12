# EntDB Dual-Stack Parity Suite — Phase 4A (EPIC #407)

Pre-deletion sanity check for the Python -> Go server port. Boots the
Python and Go gRPC servers side-by-side on independent Kafka clusters,
replays the same multi-RPC sequences against each, and asserts the
resulting SQLite + ACL state is semantically equivalent.

## What it covers

The cross-impl contract harness (#428) and e2e suites (#478, #479)
already give us per-RPC parity (70/70 contract + 22/22 e2e under both
targets). This suite fills the remaining gap:

* **Stateful sequence parity** — multi-RPC traffic patterns (CRUD,
  edges, share/revoke, idempotent retry, concurrent writers, restart)
  produce equivalent end-state on both servers.
* **WAL replay parity** — write -> `docker compose restart` -> read.
  Both servers must rebuild the same SQLite view from the WAL.

## When to run it

This suite is **transitional**. Run it locally before any release that
touches the Go server, and especially before Phase 4D when
`server/python/` is deleted. It does **not** have a CI job — adding
one would impose ~5 minutes of CI runtime for parity assertions that
are already covered by contract + e2e at the RPC level.

After Phase 4D deletes `server/python/`, the Python server image
build will fail and this suite becomes unrunnable. **Delete
`tests/python/parity/` in the same commit** — track it in the Phase
4D cleanup checklist.

## Running locally

```bash
bash tests/python/parity/run-parity.sh
# Useful flags:
#   --no-cache   force a clean container build
#   --logs       dump server logs on failure
#   --keep       keep containers up after the run (for debug)
```

The script spins up Redpanda x2, MinIO, both server containers, and
the `parity-tests` runner. Total runtime on a warm cache is ~3-5
minutes; first run incurs ~5-8 minutes of image build.

## Architecture

```
parity-tests ──┬── server-python:50051  ── redpanda-py
               └── server-go:50051      ── redpanda-go
```

Both servers use the same e2e schema (`User`/`Product`/`Order` +
`Purchased`/`PlacedOrder`/`OrderContains` edges, type IDs 8001-8003,
edge IDs 8101-8103). Each test allocates a private tenant via
`CreateTenant` on the Go server (which has its global store enabled);
the Python server auto-creates tenants on first write (its global
store is disabled in dev mode).

Each server reads its own dedicated Redpanda cluster. **This matters**:
if both servers consumed from the same Kafka topic, the harness would
be testing replay-determinism (replaying the same WAL twice into two
appliers) rather than parity (two servers, two independent WALs,
identical responses). Independent clusters force each server to
generate its own WAL entries from its own ingress path, which is what
parity actually means.

## Divergence policy

The normalisation helpers in `conftest.py` strip values that
legitimately differ between independent servers:

* `node_id`, `id` (receipt ID), generated UUIDs — replaced with
  `<UUID>` placeholders.
* `created_at`, `updated_at`, `applied_at`, `ts_ms` — wall-clock
  timestamps, stripped.
* `stream_position`, `offset` — independent WALs => independent
  offsets, stripped.
* `tenant_id` — per-test random tenant ids, stripped.

**Everything else must match exactly** after normalisation. If a
parity scenario fails it indicates a real semantic divergence between
the Python and Go implementations and is a **Phase 4 blocker** —
either fix the Go server before deleting the Python one or document
the divergence as an intentional behaviour change.

## Files

* `docker-compose.parity.yml` — two-stack topology with independent
  Kafka clusters.
* `Dockerfile.parity` — parity-tests runner image; mirrors the e2e
  runner but pinned at the parity test directory.
* `conftest.py` — `python_client` / `go_client` fixtures, per-test
  `parity_tenant`, normalisation + snapshot helpers, `restart_servers`.
* `test_parity_scenarios.py` — six parity scenarios.
* `run-parity.sh` — one-shot driver.
* `e2e_schema_pb2.py` — generated proto module, copied from
  `tests/python/e2e/` at container-build time (kept in lock-step via
  the Dockerfile, not duplicated in version control).
