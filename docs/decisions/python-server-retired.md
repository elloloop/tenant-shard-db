# Python server retirement

**Status**: deleted in Phase 4D of EPIC #407.

The Python server (`server/python/`) was retired in favor of the Go
reimplementation (`server/go/`) after sustained functional parity:
- 70/70 contract tests under cross-impl harness (Wave 4)
- 22/22 e2e under both targets (Wave 8 + Wave 9 — Kafka WAL)
- 6/6 dual-stack parity scenarios (Phase 4A)
- 2-5x faster on every measured RPC under matched resource limits (Phase 4B)

The release pipeline (Phase 4C) had already switched to publishing
the Go image under the same `ghcr.io/elloloop/tenant-shard-db:vX.Y.Z`
tag, making the deletion a no-op for downstream consumers.

SDK clients (`sdk/python/entdb_sdk/`, `sdk/go/entdb/`) are unchanged.
