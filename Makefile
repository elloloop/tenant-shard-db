# EntDB Makefile
# ================
#
# Common development tasks for EntDB.
#
# Quick start:
#   make help        Show every target (grouped by purpose)
#   make ci          Run the FULL local pre-push gate (every suite)
#   make dev         Start the local dev environment in Docker
#
# `make ci` is the canonical pre-push command. It runs every suite this
# repo expects to be green: all FOUR Go modules (server, Go SDK,
# tests/contract, codegen plugin), the Python integration + unit
# suites, the Docker e2e stack, and ruff. Per CLAUDE.md, run it clean
# before every `git push`.

.PHONY: help dev stop logs test e2e e2e-logs e2e-keep build clean proto \
        schema-snapshot schema-breaking \
        ci verify check \
        test-go test-server test-sdk-go test-contract test-tools \
        test-python test-python-integration test-python-unit \
        test-e2e lint

# -----------------------------------------------------------------------------
# Configurable knobs
# -----------------------------------------------------------------------------

# Schema-evolution compat gate (ADR-032). SCHEMA_BASELINE is the committed
# lock file; SCHEMA_CURRENT is the snapshot regenerated from the current
# proto. Override on the command line, e.g.
#   make schema-breaking SCHEMA_CURRENT=build/new-snapshot.json
# The invocation is IDENTICAL to the CI step — same `breaking` verb, same
# exit-code contract — so local dev and CI never diverge.
SCHEMA_BIN      ?= entdb-schema
SCHEMA_BASELINE ?= schema.lock.json
SCHEMA_CURRENT  ?= build/current-snapshot.json
SCHEMA_SERVER   ?= localhost:50051

# =============================================================================
# Help
# =============================================================================

# Default target — list every available target, grouped by purpose, with
# a one-line description for each. The grouping mirrors the sections in
# this Makefile.
help:
	@echo "EntDB Make targets"
	@echo "=================="
	@echo ""
	@echo "Verification (the pre-push gate):"
	@echo "  make ci                          Run EVERY suite: 4 Go modules + Python + e2e + lint"
	@echo "                                   Aliases: make verify, make check"
	@echo ""
	@echo "Granular test targets (composable; ci runs them all):"
	@echo "  make test-go                     All four Go modules in one shot"
	@echo "    make test-server                 server/go (canonical entdb-server)"
	@echo "    make test-sdk-go                 sdk/go/entdb (the published Go SDK)"
	@echo "    make test-contract               tests/contract (cross-language schema parity)"
	@echo "    make test-tools                  tools/protoc-gen-entdb-keys (codegen plugin)"
	@echo "  make test-python                 Python integration + unit"
	@echo "    make test-python-integration     115 integration tests vs a live Go server (~2 min)"
	@echo "    make test-python-unit            449 fast unit tests (~1 s)"
	@echo "  make test-e2e                    Docker e2e stack (redpanda + server + 25 tests, ~1-2 min)"
	@echo "  make lint                        ruff check + format check"
	@echo ""
	@echo "Development:"
	@echo "  make dev                         Start dev environment (server + console on :8080)"
	@echo "  make stop                        Stop dev environment"
	@echo "  make logs                        Tail dev environment logs"
	@echo ""
	@echo "Build & codegen:"
	@echo "  make build                       Build all Docker images"
	@echo "  make proto                       Regenerate protobuf stubs (Go + Python)"
	@echo ""
	@echo "Schema-evolution gate (ADR-032):"
	@echo "  make schema-snapshot             Regenerate the current schema snapshot from a running server"
	@echo "  make schema-breaking             Compare current vs baseline; exit non-zero on breaking change"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean                       Remove all containers and volumes"
	@echo ""
	@echo "Legacy:"
	@echo "  make test                        Containerised unit-test subset (prefer 'make ci')"
	@echo "  make e2e / e2e-logs / e2e-keep   Direct e2e entrypoints (test-e2e is the same)"

# =============================================================================
# Verification — the canonical pre-push gate.
#
# Per CLAUDE.md ("Always run CI locally before pushing"), the full suite
# is the union of FOUR Go modules (server, Go SDK, tests/contract,
# tools/protoc-gen-entdb-keys), the Python integration + unit suites, the
# Docker e2e stack, and ruff lint/format. CLAUDE.md's documented CI list
# happens to omit tests/contract + tools/protoc-gen-entdb-keys; this
# aggregator keeps them in the loop so a schema/proto change cannot
# silently break them.
# =============================================================================

# Run every suite in order; stop on the first failure. ~4-5 min warm.
# Requires: Go toolchain, Python venv with grpcio==1.78.x, uvx, Docker.
ci: test-go test-python test-e2e lint
	@echo ""
	@echo "All suites green - safe to push."

# Aliases — pick whichever verb fits muscle memory; they all run ci.
verify: ci
check: ci

# -----------------------------------------------------------------------------
# Go suites — four separate go.mod modules
# -----------------------------------------------------------------------------

# Run all four Go modules. Composes the per-module targets below so
# `make ci` can depend on a single name while devs can still drill down.
test-go: test-server test-sdk-go test-contract test-tools

# server/go: the canonical Go server (entdb-server, entdb-schema,
# entdb-console binaries + every server-side internal/ package). The
# bulk of the runtime + WAL apply + schema-registry + payload + ACL.
# ~30 s on a warm cache.
test-server:
	@echo "=== test-server: server/go vet + test ==="
	cd server/go && go vet ./... && go test ./...

# sdk/go/entdb: the published Go SDK module
# (`go get github.com/elloloop/tenant-shard-db/sdk/go/entdb`). Carries
# its own checked-in pb stubs + the self-describing-write transport
# (ADR-031). ~10 s.
test-sdk-go:
	@echo "=== test-sdk-go: sdk/go/entdb vet + test ==="
	cd sdk/go/entdb && go vet ./... && go test ./...

# tests/contract: cross-language schema-snapshot parity tests — round-
# trip + self-consistency against the entdb-schema CLI. SEPARATE go.mod,
# NOT covered by `cd server/go && go test ./...`. A schema/proto change
# can pass the documented CI list and still break this; that's exactly
# what happened during ADR-031 (the name-ful fixture stopped parsing).
# ~2 s.
test-contract:
	@echo "=== test-contract: tests/contract go test ==="
	cd tests/contract && go test ./...

# tools/protoc-gen-entdb-keys: the protoc plugin that generates the
# typed UniqueKey[T] tokens for the SDKs (ADR-025). Separate go.mod,
# also not in the documented CI list. ~3 s.
test-tools:
	@echo "=== test-tools: tools/protoc-gen-entdb-keys go test ==="
	cd tools/protoc-gen-entdb-keys && go test ./...

# -----------------------------------------------------------------------------
# Python suites
# -----------------------------------------------------------------------------

# Both Python suites (integration + unit). Does NOT run the Docker e2e
# stack — that's `test-e2e`, separated because it needs Docker.
test-python: test-python-integration test-python-unit

# 115 integration tests against a LIVE Go server. The conftest builds
# entdb-server, boots it schema-less (ADR-031: no boot seed), then
# bootstraps tenant + users + schema through the gRPC API before tests
# — exactly how a real client provisions. ~2 min.
# Requires: a Python venv with grpcio==1.78.x to match the pinned stubs
# in sdk/python/entdb_sdk/_generated/.
test-python-integration:
	@echo "=== test-python-integration: pytest tests/python/integration ==="
	python -m pytest tests/python/integration/ -q

# 449 fast Python unit tests (no server boot, no Docker). ~1 s.
test-python-unit:
	@echo "=== test-python-unit: pytest tests/python/unit ==="
	python -m pytest tests/python/unit/ -q

# -----------------------------------------------------------------------------
# Docker e2e
# -----------------------------------------------------------------------------

# Spin up redpanda + entdb-server in containers and run the 25-case e2e
# suite (Kafka WAL, crash recovery, S3 Object Lock). Requires Docker.
# ~1-2 min. Alias for the existing `e2e` target so `ci` can compose
# under a consistent `test-*` naming.
test-e2e: e2e

# -----------------------------------------------------------------------------
# Lint / format
# -----------------------------------------------------------------------------

# ruff check + format-check, pinned to 0.15.7 to match GitHub CI exactly.
# Fast — runs over the whole tree in under a second.
lint:
	@echo "=== lint: ruff check + format ==="
	uvx ruff@0.15.7 check .
	uvx ruff@0.15.7 format --check .

# =============================================================================
# Development environment
# =============================================================================

# Start the local dev stack (server + entdb-console on :8080) in Docker.
# For non-Docker dev, build and run server/go/cmd/entdb-server directly.
dev:
	@echo "Starting development environment..."
	docker compose up -d --build
	@echo ""
	@echo "Services:"
	@echo "  EntDB Console: http://localhost:8080  (Go binary, embedded SPA)"
	@echo "  gRPC:          localhost:50051"
	@echo ""
	@echo "Use 'make logs' to view logs, 'make stop' to stop"

# Stop the dev stack (keeps volumes; use `make clean` to wipe).
stop:
	docker compose down

# Tail the dev stack logs.
logs:
	docker compose logs -f

# =============================================================================
# Legacy testing entrypoints (kept for backward compatibility)
# =============================================================================

# Containerised unit-test runner (uses the `test` service in
# docker-compose.yml). Predates the granular targets above. For the
# real pre-push gate use `make ci`, not this.
test:
	@echo "Running unit tests (containerised; for full pre-push gate use 'make ci')..."
	docker compose -f docker-compose.yml build test
	docker compose -f docker-compose.yml run --rm test

# The Docker e2e stack. Same as `make test-e2e`; kept under both names
# so existing muscle memory + scripts keep working.
e2e:
	@echo "Running end-to-end tests..."
	./tests/python/e2e/run-e2e.sh

# Like `e2e`, but dumps server logs on failure (for debugging).
e2e-logs:
	@echo "Running end-to-end tests (with logs on failure)..."
	./tests/python/e2e/run-e2e.sh --logs

# Like `e2e`, but keeps containers running after the suite (for debug).
e2e-keep:
	@echo "Running end-to-end tests (keeping containers)..."
	./tests/python/e2e/run-e2e.sh --keep

# =============================================================================
# Build & codegen
# =============================================================================

# Build all Docker images (dev compose + e2e compose).
build:
	@echo "Building Docker images..."
	docker compose build
	docker compose -f tests/python/e2e/docker-compose.e2e.yml build

# Regenerate the protobuf stubs:
#   - Go (server + Go SDK) via buf with the templates in each module
#   - Python via scripts/generate_proto.sh, which uses the LOCAL
#     grpcio-tools so the emitted GRPC_GENERATED_VERSION matches the
#     pinned grpcio in this venv (avoids the 1.80-vs-1.78 generation
#     skew that bit us during ADR-031).
proto:
	@echo "Regenerating Go stubs (server + SDK) via buf..."
	buf generate --template server/go/buf.gen.yaml
	buf generate --template server/go/buf.gen.console.yaml
	buf generate --template sdk/go/entdb/buf.gen.yaml
	@echo "Regenerating Python SDK stubs..."
	./scripts/generate_proto.sh

# =============================================================================
# Schema evolution (ADR-032) — buf-breaking-style runtime-schema gate
# =============================================================================

# Regenerate the current schema snapshot from a running server. Commit
# the output once as your baseline (schema.lock.json), then run
# `schema-breaking` on every change.
schema-snapshot:
	@mkdir -p $(dir $(SCHEMA_CURRENT))
	$(SCHEMA_BIN) snapshot --from-server $(SCHEMA_SERVER) > $(SCHEMA_CURRENT)
	@echo "Wrote $(SCHEMA_CURRENT) (commit it as $(SCHEMA_BASELINE) for the first baseline)"

# The one-line gate. Exits non-zero on any breaking change. Identical
# invocation in local dev and CI — the same `breaking` verb a customer
# runs in their own CI.
schema-breaking:
	$(SCHEMA_BIN) breaking --baseline $(SCHEMA_BASELINE) --from-file $(SCHEMA_CURRENT)

# =============================================================================
# Cleanup
# =============================================================================

# Tear down all containers and volumes from both compose stacks.
clean:
	@echo "Cleaning up..."
	docker compose down -v --remove-orphans
	docker compose -f tests/python/e2e/docker-compose.e2e.yml down -v --remove-orphans
	@echo "Done"
