# ADR-017: Python server retired in favor of Go reimplementation

**Status:** Accepted
**Decided:** 2026-05-12
**Tags:** server, go-port, deprecation, release
**Implementation:** EPIC #407 Phase 4D, commit `8d07f5f` (`server/python/` deletion); release pipeline switched in Phase 4C (commit `709eecf`)

## Decision

The Python gRPC server (`server/python/`) has been deleted. The Go
server (`server/go/`) is the sole production implementation. The
release pipeline publishes the Go binary under the existing
`ghcr.io/elloloop/tenant-shard-db:vX.Y.Z` image tag, so the swap is a
no-op for downstream consumers. SDK clients
(`sdk/python/entdb_sdk/`, `sdk/go/entdb/`) are unchanged and continue
to speak the same proto contract.

## Context

EntDB shipped on a Python `grpc.aio` server through v1.12.x. EPIC
#407 ran a multi-wave Go reimplementation against a cross-impl test
harness to prove behavioral parity before retiring the Python tree.
The evidence ladder cleared:

- **Contract parity (Wave 4):** 70/70 contract tests pass under the
  cross-impl harness driving the Python SDK against both servers.
- **End-to-end parity (Wave 8 + Wave 9):** 22/22 Docker-stack e2e
  cases pass under both targets, including crash-recovery scenarios
  against the Kafka WAL backend.
- **Dual-stack parity (Phase 4A):** 6/6 scenarios passing on a
  single stack running both servers simultaneously.
- **Performance (Phase 4B):** 2-5x faster than the Python server on
  every measured RPC under matched resource limits (single dual-
  backend benchmark suite, PR #483).
- **Release pipeline swap (Phase 4C, PR #484):** the published
  Docker image at `ghcr.io/elloloop/tenant-shard-db:vX.Y.Z` already
  carried the Go binary by v1.13.0 — consumers had been running on
  the Go server in production before the source-tree deletion
  landed.

With every measurable parity gate cleared and the image swap already
proven in production, keeping the Python source tree would have meant
running two implementations through every future schema change and
release cycle. The retirement commit (`8d07f5f`) removed
`server/python/` outright; the Python SDK and SDK contract suite stay
because they exercise the wire contract from the client side, not the
server.

## Alternatives considered

- **Keep both implementations in the tree as parallel backends.**
  Rejected. Every wire-contract change, every WAL op type, every
  schema-registry tweak would have to ship twice and stay parity-
  tested. The cross-impl harness was viable as a one-time migration
  gate, not as ongoing dual-implementation maintenance.
- **Retire Python before parity was fully demonstrated.** Rejected.
  EntDB stores customer data; switching server implementations
  without contract + e2e + perf parity would have risked silent
  behavioral regressions on operations that hadn't yet been
  exercised under the new server.
- **Move Python to a separate "reference implementation" repo.**
  Rejected. Without an active production deploy, the Python server
  would have rotted within one or two release cycles. A frozen
  reference implementation that doesn't match the wire contract is
  worse than no implementation — it misleads readers into thinking
  it's authoritative.

## Consequences

**What this locks in:**

- `server/go/` is the only server implementation. `server/python/`
  is gone from the tree; resurrection would require recreating it
  from scratch (git history retains the snapshot but it's not a
  maintained branch).
- The release pipeline (`.github/workflows/release.yml`) builds and
  pushes the Go server image under the existing
  `ghcr.io/elloloop/tenant-shard-db:vX.Y.Z` tag. The image name and
  proto contract are unchanged across the swap — downstream
  consumers do not need to update Helm charts, compose files, or
  client code.
- The SDK contract suite (`tests/python/integration/`) targets the
  Go server. The Python conftest harness boots `server/go/cmd/entdb-
  server` (or a docker-compose stack for e2e) and drives it through
  the Python SDK over gRPC.

**What this makes easy:**

- A single server codebase to extend, profile, and deploy.
- One source of truth for behavior: when the proto contract and the
  Go server disagree, the bug is in one place.
- Performance work (the 2-5x speedup measured in Phase 4B) is
  realized for every consumer of the published image, automatically.

**What this makes harder:**

- Cross-impl parity testing is no longer available as a regression
  net. Bugs that previously would have been caught by Python-vs-Go
  divergence now have to be caught by direct test coverage on the
  Go server.
- Contributors looking for a Python-server reference must use `git
  log -- server/python/` or `git show <pre-retirement-sha>:server/
  python/...`. The retirement commit (`8d07f5f`) is the cutover
  point.

**Failure modes:**

- A behavior the Python server had that wasn't exercised by the
  cross-impl harness might be missing from the Go server. The
  contract suite (70 cases) and e2e suite (22 cases) covered every
  RPC and the major operational paths, but a silent gap is
  theoretically possible. Mitigated by: deploying the Go image in
  production well before the source deletion, so any regressions
  would have surfaced operationally.

## References

- EPIC #407 — Go server reimplementation; Wave 4 (contract), Wave 8
  + 9 (e2e), Phase 4A (dual-stack), Phase 4B (perf), Phase 4C
  (release), Phase 4D (deletion).
- Commit `8d07f5f` (PR #485) — `server/python/` tree deletion;
  closes EPIC #407.
- Commit `709eecf` (PR #484) — release pipeline publishes Go server
  image under the same `ghcr.io/elloloop/tenant-shard-db` tag.
- Commit `bff7425` (PR #483) — dual-backend benchmark harness; the
  source of the 2-5x perf parity finding.
- Files this commit removes content from:
  - `docs/decisions/python-server-retired.md` — the retirement
    rationale and evidence ladder are captured here; the file is
    deleted in the same commit.
