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

.PHONY: help dev stop test e2e e2e-logs build clean proto

# Default target
help:
	@echo "EntDB Development Commands"
	@echo "=========================="
	@echo ""
	@echo "Development:"
	@echo "  make dev          Start development environment (server, console, playground)"
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
	@echo "  Console:    http://localhost:8080"
	@echo "  Playground: http://localhost:8081"
	@echo "  gRPC:       localhost:50051"
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
	./tests/e2e/run-e2e.sh

e2e-logs:
	@echo "Running end-to-end tests (with logs on failure)..."
	./tests/e2e/run-e2e.sh --logs

e2e-keep:
	@echo "Running end-to-end tests (keeping containers)..."
	./tests/e2e/run-e2e.sh --keep

# =============================================================================
# Build
# =============================================================================

build:
	@echo "Building Docker images..."
	docker compose build
	docker compose -f tests/e2e/docker-compose.e2e.yml build

proto:
	@echo "Regenerating protobuf files..."
	./scripts/generate_proto.sh

# =============================================================================
# Cleanup
# =============================================================================

clean:
	@echo "Cleaning up..."
	docker compose down -v --remove-orphans
	docker compose -f tests/e2e/docker-compose.e2e.yml down -v --remove-orphans
	@echo "Done"
