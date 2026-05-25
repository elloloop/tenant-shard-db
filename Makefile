# EntDB Makefile
# ================
#
# Common development tasks for EntDB.
#
# Usage:
#   make help        Show available targets
#   make dev         Start development environment
#   make test        Run unit tests
#   make e2e         Run end-to-end tests
#   make build       Build Docker images
#

.PHONY: help dev stop test e2e e2e-logs build clean proto schema-snapshot schema-breaking

# Schema-evolution compat gate (ADR-032). SCHEMA_BASELINE is the
# committed lock file; SCHEMA_CURRENT is the snapshot regenerated from
# the current proto. Override on the command line, e.g.
#   make schema-breaking SCHEMA_CURRENT=build/new-snapshot.json
# The invocation is IDENTICAL to the CI step — the same `breaking` verb,
# the same exit-code contract — so local dev and CI never diverge.
SCHEMA_BIN     ?= entdb-schema
SCHEMA_BASELINE ?= schema.lock.json
SCHEMA_CURRENT  ?= build/current-snapshot.json
SCHEMA_SERVER   ?= localhost:50051

# Default target
help:
	@echo "EntDB Development Commands"
	@echo "=========================="
	@echo ""
	@echo "Development:"
	@echo "  make dev          Start development environment (server + entdb-console on :8080)"
	@echo "  make stop         Stop development environment"
	@echo "  make logs         Show development logs"
	@echo ""
	@echo "Testing:"
	@echo "  make test         Run unit tests"
	@echo "  make e2e          Run end-to-end tests"
	@echo "  make e2e-logs     Run e2e tests and show logs on failure"
	@echo ""
	@echo "Build:"
	@echo "  make build        Build all Docker images"
	@echo "  make proto        Regenerate protobuf files"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean        Remove all containers and volumes"
	@echo ""

# =============================================================================
# Development
# =============================================================================

dev:
	@echo "Starting development environment..."
	docker compose up -d --build
	@echo ""
	@echo "Services:"
	@echo "  EntDB Console: http://localhost:8080  (Go binary, embedded SPA)"
	@echo "  gRPC:          localhost:50051"
	@echo ""
	@echo "Use 'make logs' to view logs, 'make stop' to stop"

stop:
	docker compose down

logs:
	docker compose logs -f

# =============================================================================
# Testing
# =============================================================================

test:
	@echo "Running unit tests..."
	docker compose -f docker-compose.yml build test
	docker compose -f docker-compose.yml run --rm test

e2e:
	@echo "Running end-to-end tests..."
	./tests/python/e2e/run-e2e.sh

e2e-logs:
	@echo "Running end-to-end tests (with logs on failure)..."
	./tests/python/e2e/run-e2e.sh --logs

e2e-keep:
	@echo "Running end-to-end tests (keeping containers)..."
	./tests/python/e2e/run-e2e.sh --keep

# =============================================================================
# Build
# =============================================================================

build:
	@echo "Building Docker images..."
	docker compose build
	docker compose -f tests/python/e2e/docker-compose.e2e.yml build

proto:
	@echo "Regenerating Go stubs (server + SDK) via buf..."
	buf generate --template server/go/buf.gen.yaml
	buf generate --template server/go/buf.gen.console.yaml
	buf generate --template sdk/go/entdb/buf.gen.yaml
	@echo "Regenerating Python SDK stubs..."
	./scripts/generate_proto.sh

# =============================================================================
# Schema evolution (ADR-032) — buf-breaking-style runtime-schema gate.
# =============================================================================

# Regenerate the current schema snapshot from a running server. Commit the
# output once as your baseline (schema.lock.json), then run `schema-breaking`
# on every change.
schema-snapshot:
	@mkdir -p $(dir $(SCHEMA_CURRENT))
	$(SCHEMA_BIN) snapshot --from-server $(SCHEMA_SERVER) > $(SCHEMA_CURRENT)
	@echo "Wrote $(SCHEMA_CURRENT) (commit it as $(SCHEMA_BASELINE) for the first baseline)"

# The one-line gate. Exits non-zero on any breaking change. Identical
# invocation in local dev and CI.
schema-breaking:
	$(SCHEMA_BIN) breaking --baseline $(SCHEMA_BASELINE) --from-file $(SCHEMA_CURRENT)

# =============================================================================
# Cleanup
# =============================================================================

clean:
	@echo "Cleaning up..."
	docker compose down -v --remove-orphans
	docker compose -f tests/python/e2e/docker-compose.e2e.yml down -v --remove-orphans
	@echo "Done"
