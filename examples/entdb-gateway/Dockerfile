# EntDB HTTP Gateway - Multi-stage Dockerfile
# This builds both the frontend and the backend gateway

# ============================================
# Stage 1: Build frontend
# ============================================
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

# Install dependencies
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci --silent

# Copy source and build
COPY frontend/ ./
RUN npm run build

# ============================================
# Stage 2: Python base
# ============================================
FROM python:3.11-slim AS python-base

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /app

# ============================================
# Stage 3: Build gateway
# ============================================
FROM python-base AS gateway-builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Create virtual environment
RUN python -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

# Install dependencies
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

# ============================================
# Stage 4: Production image
# ============================================
FROM python-base AS production

# Copy virtual environment
COPY --from=gateway-builder /opt/venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

# Create non-root user
RUN groupadd -r gateway && useradd -r -g gateway gateway

# Copy gateway code
COPY gateway/ ./gateway/

# Copy built frontend
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

# Set ownership
RUN chown -R gateway:gateway /app

USER gateway

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run with uvicorn
CMD ["uvicorn", "gateway.app:app", "--host", "0.0.0.0", "--port", "8080"]
