# EntDB Server Dockerfile
#
# This builds a production-ready container for the EntDB server.
# The same Dockerfile is used for:
# - Local development (docker compose)
# - CI builds
# - Production deployment (ECS/Kubernetes)
#
# Build:
#   docker build -t entdb-server .
#
# Run:
#   docker run -p 50051:50051 entdb-server

# =============================================================================
# Base stage with Python and dependencies
# =============================================================================
FROM python:3.11-slim-bookworm AS base

# Prevent Python from writing bytecode and buffering stdout/stderr
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

# Create non-root user for security
RUN groupadd --gid 1000 entdb && \
    useradd --uid 1000 --gid 1000 --shell /bin/bash --create-home entdb

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# =============================================================================
# Builder stage - install dependencies via uv from pyproject.toml
# =============================================================================
FROM base AS builder

# Pull pinned uv binary from the official distroless image. uv resolves
# and installs the dependency tree from pyproject.toml extras, so the
# image deps cannot drift from the declared deps.
COPY --from=ghcr.io/astral-sh/uv:0.5.14 /uv /uvx /usr/local/bin/

# Build deps for any wheels that need compilation (cryptography, grpcio
# on uncommon archs). Most wheels are prebuilt.
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libffi-dev \
    && rm -rf /var/lib/apt/lists/*

ENV UV_LINK_MODE=copy \
    UV_COMPILE_BYTECODE=1 \
    UV_PYTHON_DOWNLOADS=never

RUN uv venv /opt/venv
ENV VIRTUAL_ENV=/opt/venv \
    PATH="/opt/venv/bin:$PATH"

# server/python/pyproject.toml is the single source of truth for runtime
# deps. `uv pip compile` resolves the project + named extras into a flat
# requirement list, which `uv pip install` then installs. The entdb
# package itself is NOT installed here — modules are imported from /app
# via PYTHONPATH at the per-stage level, so a code change does not
# invalidate this dependency layer.
COPY server/python/pyproject.toml ./server-pyproject.toml
COPY README.md ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv pip compile server-pyproject.toml \
        -o /tmp/requirements.txt && \
    uv pip install --no-cache -r /tmp/requirements.txt

# =============================================================================
# Server stage - production image
# =============================================================================
FROM base AS server

ARG VERSION=dev
ARG COMMIT=unknown

LABEL org.opencontainers.image.title="EntDB Server" \
      org.opencontainers.image.description="Tenant-sharded graph database with nodes and edges" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.source="https://github.com/elloloop/tenant-shard-db" \
      org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.opencontainers.image.vendor="elloloop"

# Copy virtual environment from builder
COPY --from=builder /opt/venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

# Copy application code. entdb_server and entdb_sdk land as siblings
# directly under /app so PYTHONPATH=/app exposes both top-level packages.
COPY server/python/entdb_server/ /app/entdb_server/
COPY sdk/python/entdb_sdk/ /app/entdb_sdk/
COPY VERSION /app/VERSION

# Create data directory
RUN mkdir -p /var/lib/entdb && chown -R entdb:entdb /var/lib/entdb

# Switch to non-root user
USER entdb

# Set Python path: entdb_server and entdb_sdk are siblings under /app.
ENV PYTHONPATH="/app"

# Default configuration
ENV GRPC_BIND="0.0.0.0:50051"
ENV DATA_DIR="/var/lib/entdb"
ENV LOG_LEVEL="INFO"
ENV LOG_FORMAT="json"

# Expose ports
EXPOSE 50051

# Health check via the standard gRPC Health Checking Protocol
# (grpc.health.v1.Health/Check). The probe script ships in the image
# alongside the server — no separate binary download.
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD python -m entdb_server.healthcheck --addr=localhost:50051 || exit 1

# Default command
CMD ["python", "-m", "entdb_server.main"]

# =============================================================================
# SDK stage - for sample applications
# =============================================================================
FROM base AS sdk

# Copy virtual environment from builder
COPY --from=builder /opt/venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

# Copy all code including examples
COPY . /app/

# entdb_sdk and entdb_server are siblings under /app (the SDK stage uses
# `COPY . /app/`, so the workspace tree lands at /app — but at runtime
# we only need the package roots discoverable). Adding the package
# parents to PYTHONPATH lets `import entdb_sdk` / `import entdb_server`
# work in sample apps without each app having to set its own PYTHONPATH.
ENV PYTHONPATH="/app:/app/sdk/python:/app/server/python"

# Switch to non-root user
USER entdb

# Default command for SDK stage
CMD ["python", "-c", "import entdb_sdk; print(f'EntDB SDK {entdb_sdk.__version__} ready')"]

# =============================================================================
# entdb-console (Go) frontend builder — builds the React SPA that gets
# embedded into the Go binary via //go:embed. Must run before the Go
# build stage so frontend/dist exists at compile time.
# =============================================================================
FROM node:20-slim AS entdb-console-frontend-builder

WORKDIR /build

COPY sdk/go/entdb/cmd/entdb-console/frontend/package*.json sdk/go/entdb/cmd/entdb-console/frontend/
RUN cd sdk/go/entdb/cmd/entdb-console/frontend && \
    (npm ci 2>/dev/null || npm install)

COPY sdk/go/entdb/cmd/entdb-console/frontend/ sdk/go/entdb/cmd/entdb-console/frontend/
RUN cd sdk/go/entdb/cmd/entdb-console/frontend && npm run build

# =============================================================================
# entdb-console (Go) Go builder — compiles the single binary with the
# SPA already baked into frontend/dist by the previous stage.
# =============================================================================
FROM golang:1.25-bookworm AS entdb-console-builder

WORKDIR /src

# Cache modules layer separately from source.
COPY sdk/go/entdb/go.mod sdk/go/entdb/go.sum ./sdk/go/entdb/
RUN cd sdk/go/entdb && go mod download

# Bring in the rest of the Go SDK source.
COPY sdk/go/entdb/ ./sdk/go/entdb/

# Replace the placeholder dist/ with the real built SPA.
COPY --from=entdb-console-frontend-builder \
    /build/sdk/go/entdb/cmd/entdb-console/frontend/dist \
    ./sdk/go/entdb/cmd/entdb-console/frontend/dist

RUN cd sdk/go/entdb && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /entdb-console ./cmd/entdb-console

# =============================================================================
# entdb-console runtime — distroless static, just the binary.
# =============================================================================
FROM gcr.io/distroless/static-debian12:nonroot AS entdb-console

ARG VERSION=dev
ARG COMMIT=unknown

LABEL org.opencontainers.image.title="EntDB Console (Go)" \
      org.opencontainers.image.description="Single-binary read-only console + embedded React SPA" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.source="https://github.com/elloloop/tenant-shard-db" \
      org.opencontainers.image.licenses="MIT"

COPY --from=entdb-console-builder /entdb-console /entdb-console

# Default flags. Override via ENTDB_CONSOLE_ADDR / ENTDB_UPSTREAM /
# ENTDB_API_KEY environment variables.
ENV ENTDB_CONSOLE_ADDR="0.0.0.0:8080" \
    ENTDB_UPSTREAM="server:50051"

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/entdb-console"]

# =============================================================================
# Test stage - for running tests
# =============================================================================
FROM builder AS test

# Install dev tooling on top of the runtime venv. The dev extras lived
# in the old monolithic root pyproject; after the workspace split they
# are pinned inline here.
RUN --mount=type=cache,target=/root/.cache/uv \
    uv pip install --no-cache \
        "pytest>=8.0.0" \
        "pytest-asyncio>=0.23.0" \
        "pytest-cov>=4.1.0" \
        "pytest-timeout>=2.2.0" \
        "ruff>=0.15.0" \
        "mypy>=1.8.0" \
        "httpx>=0.26.0" \
        "grpcio-tools>=1.60.0"

# Copy all code
COPY . /app/

# Set Python path: server/python and sdk/python expose the top-level
# entdb_server and entdb_sdk packages.
ENV PYTHONPATH="/app:/app/server/python:/app/sdk/python"

# Default command runs tests
CMD ["pytest", "tests/python/", "-v", "--cov=entdb_server", "--cov=entdb_sdk"]
