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
# Builder stage - install dependencies
# =============================================================================
FROM base AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libffi-dev \
    && rm -rf /var/lib/apt/lists/*

# Create virtual environment
RUN python -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

# Copy requirements and install dependencies
COPY pyproject.toml ./
RUN pip install --no-cache-dir --upgrade pip setuptools wheel && \
    pip install --no-cache-dir \
    aiokafka>=0.10.0 \
    aiobotocore>=2.11.0 \
    aiohttp>=3.9.0 \
    grpcio>=1.60.0 \
    grpcio-tools>=1.60.0 \
    uvicorn>=0.27.0 \
    fastapi>=0.109.0 \
    pydantic>=2.5.0 \
    pydantic-settings>=2.0.0 \
    pyyaml>=6.0.0 \
    aiofiles>=23.2.0

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
ENV HTTP_ENABLED="true"
ENV HTTP_BIND="0.0.0.0:8081"
ENV DATA_DIR="/var/lib/entdb"
ENV LOG_LEVEL="INFO"
ENV LOG_FORMAT="json"

# Expose ports
EXPOSE 50051 8081

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8081/v1/health || exit 1

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
COPY dbaas/ /app/dbaas/
COPY sdk/ /app/sdk/
COPY playground/*.py /app/playground/

# Copy built frontend
COPY --from=frontend-builder /build/playground/frontend/dist /app/playground/static

# Switch to non-root user
USER entdb

# Set Python path (include /app/sdk for entdb_sdk imports)
ENV PYTHONPATH="/app:/app/sdk"

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

# Install test dependencies
RUN pip install --no-cache-dir \
    pytest>=7.4.0 \
    pytest-asyncio>=0.23.0 \
    pytest-cov>=4.1.0 \
    pytest-timeout>=2.2.0 \
    httpx>=0.26.0

# Copy all code
COPY . /app/

# Set Python path
ENV PYTHONPATH="/app"

# Default command runs tests
CMD ["pytest", "tests/", "-v", "--cov=dbaas", "--cov=sdk"]
