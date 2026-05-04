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
#   docker run -p 50051:50051 -p 8081:8081 entdb-server

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
# Frontend builder - Build React frontends
# =============================================================================
FROM node:20-slim AS frontend-builder

WORKDIR /build

# Build Console frontend
COPY console/frontend/package*.json console/frontend/
RUN cd console/frontend && npm install --frozen-lockfile 2>/dev/null || npm install

COPY console/frontend/ console/frontend/
RUN cd console/frontend && npm run build

# Build Playground frontend
COPY playground/frontend/package*.json playground/frontend/
RUN cd playground/frontend && npm install --frozen-lockfile 2>/dev/null || npm install

COPY playground/frontend/ playground/frontend/
RUN cd playground/frontend && npm run build

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

# pyproject.toml is the single source of truth for runtime deps.
# `uv pip compile` resolves the named extras into a flat requirement
# list, which `uv pip install` then installs. The entdb package itself
# is NOT installed here — modules are imported from /app via PYTHONPATH
# at the per-stage level, so a code change does not invalidate this
# dependency layer.
COPY pyproject.toml README.md ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv pip compile pyproject.toml \
        --extra server --extra console --extra playground \
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
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.vendor="elloloop"

# Copy virtual environment from builder
COPY --from=builder /opt/venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

# Copy application code
COPY dbaas/ /app/dbaas/
COPY sdk/ /app/sdk/
COPY VERSION /app/VERSION

# Create data directory
RUN mkdir -p /var/lib/entdb && chown -R entdb:entdb /var/lib/entdb

# Switch to non-root user
USER entdb

# Set Python path (include /app/sdk for entdb_sdk imports)
ENV PYTHONPATH="/app:/app/sdk"

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
    CMD python -m dbaas.entdb_server.healthcheck --addr=localhost:50051 || exit 1

# Default command
CMD ["python", "-m", "dbaas.entdb_server.main"]

# =============================================================================
# SDK stage - for sample applications
# =============================================================================
FROM base AS sdk

# Copy virtual environment from builder
COPY --from=builder /opt/venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

# Copy all code including examples
COPY . /app/

# Set Python path
ENV PYTHONPATH="/app"

# Switch to non-root user
USER entdb

# Default command for SDK stage
CMD ["python", "-c", "print('EntDB SDK ready')"]

# =============================================================================
# Console stage - Read-only data browser
# =============================================================================
FROM base AS console

# Copy virtual environment from builder
COPY --from=builder /opt/venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

# Copy application code
COPY dbaas/ /app/dbaas/
COPY sdk/ /app/sdk/
COPY console/gateway/ /app/console/gateway/
COPY console/__init__.py /app/console/

# Copy built frontend
COPY --from=frontend-builder /build/console/frontend/dist /app/console/static

# Switch to non-root user
USER entdb

# Set Python path (include /app/sdk for entdb_sdk imports)
ENV PYTHONPATH="/app:/app/sdk"

# Console configuration
ENV CONSOLE_HOST="0.0.0.0"
ENV CONSOLE_PORT="8080"
ENV CONSOLE_ENTDB_HOST="server"
ENV CONSOLE_ENTDB_PORT="50051"
ENV CONSOLE_DEFAULT_TENANT_ID="default"

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Default command
CMD ["uvicorn", "console.gateway.app:app", "--host", "0.0.0.0", "--port", "8080"]

# =============================================================================
# Playground stage - Interactive SDK sandbox
# =============================================================================
FROM base AS playground

# Copy virtual environment from builder
COPY --from=builder /opt/venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

# Copy application code
COPY playground/*.py /app/playground/

# Copy built frontend
COPY --from=frontend-builder /build/playground/frontend/dist /app/playground/static

# Switch to non-root user
USER entdb

# Set Python path
ENV PYTHONPATH="/app"

# Playground configuration
ENV PLAYGROUND_HOST="0.0.0.0"
ENV PLAYGROUND_PORT="8081"
ENV PLAYGROUND_ENTDB_HOST="server"
ENV PLAYGROUND_ENTDB_PORT="50051"
ENV PLAYGROUND_SANDBOX_TENANT="playground"

# Expose port
EXPOSE 8081

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8081/health || exit 1

# Default command
CMD ["uvicorn", "playground.app:app", "--host", "0.0.0.0", "--port", "8081"]

# =============================================================================
# Test stage - for running tests
# =============================================================================
FROM builder AS test

# Install dev extras (pytest, mypy, grpcio-tools, etc.) on top of the
# runtime venv. Source of truth is pyproject.toml [dev] extras.
RUN --mount=type=cache,target=/root/.cache/uv \
    uv pip compile pyproject.toml --extra dev -o /tmp/dev-requirements.txt && \
    uv pip install --no-cache -r /tmp/dev-requirements.txt

# Copy all code
COPY . /app/

# Set Python path
ENV PYTHONPATH="/app"

# Default command runs tests
CMD ["pytest", "tests/", "-v", "--cov=dbaas", "--cov=sdk"]
