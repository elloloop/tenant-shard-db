# EntDB HTTP Gateway

A sidecar service that provides REST API access to EntDB, demonstrating SDK usage and including a web-based data browser.

## Overview

This project shows how to build HTTP APIs on top of EntDB using the Python SDK. It includes:

- **FastAPI Gateway**: REST API that wraps the EntDB SDK
- **React Frontend**: Data browser with graph visualization
- **Docker Compose**: Full development stack

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Browser / Client                        │
└─────────────────────────┬───────────────────────────────────┘
                          │ HTTP/REST
┌─────────────────────────▼───────────────────────────────────┐
│                    HTTP Gateway                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              FastAPI Application                      │   │
│  │  • REST API endpoints (/api/v1/*)                    │   │
│  │  • React SPA frontend (/)                            │   │
│  │  • SDK client pool                                   │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────┬───────────────────────────────────┘
                          │ gRPC
┌─────────────────────────▼───────────────────────────────────┐
│                    EntDB Server                              │
│                    (gRPC only)                               │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Using Docker Compose

```bash
# Start all services
docker-compose up -d

# Open browser
open http://localhost:8080
```

### Development Mode

```bash
# Start EntDB and dependencies
docker-compose up -d entdb redpanda minio minio-init

# Run gateway locally
cd gateway
pip install -r ../requirements.txt
pip install -e ../../sdk  # Install SDK
uvicorn app:app --reload --port 8080

# Run frontend locally
cd ../frontend
npm install
npm run dev
```

## API Endpoints

### Schema

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/schema` | Get full schema |
| GET | `/api/v1/schema/types/{type_id}` | Get type schema |

### Nodes

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/nodes` | List nodes (with pagination) |
| GET | `/api/v1/nodes/{id}` | Get node by ID |
| POST | `/api/v1/nodes` | Create node |
| PATCH | `/api/v1/nodes/{id}` | Update node |
| DELETE | `/api/v1/nodes/{id}` | Delete node |

### Edges

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/nodes/{id}/edges/out` | Get outgoing edges |
| GET | `/api/v1/nodes/{id}/edges/in` | Get incoming edges |
| POST | `/api/v1/edges` | Create edge |

### Search

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/search?q=query` | Full-text search |

### Atomic Operations

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/atomic` | Execute atomic transaction |

### Browse (Frontend Support)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/browse/types` | Get types with counts |
| GET | `/api/v1/browse/graph/{id}` | Get node neighborhood |

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `GATEWAY_ENTDB_HOST` | localhost | EntDB gRPC host |
| `GATEWAY_ENTDB_PORT` | 50051 | EntDB gRPC port |
| `GATEWAY_HOST` | 0.0.0.0 | Gateway bind host |
| `GATEWAY_PORT` | 8080 | Gateway bind port |
| `GATEWAY_DEFAULT_TENANT_ID` | default | Default tenant |
| `GATEWAY_CORS_ORIGINS` | ["http://localhost:3000"] | CORS origins |

## Frontend Features

- **Type Browser**: Navigate node types in sidebar
- **Node List**: Paginated view with search
- **Node Detail**: View payload, metadata, and edges
- **Graph View**: Force-directed graph visualization
- **Search**: Full-text search with results

## SDK Usage Example

This gateway demonstrates how to use the EntDB SDK:

```python
from entdb_sdk import DbClient

async with DbClient(
    endpoint="entdb:50051",
    tenant_id="my_tenant",
    actor="user:alice",
) as client:
    # Get a node
    node = await client.get("node_123")

    # Query nodes by type
    users = await client.query(type_id=1, limit=10)

    # Atomic transaction
    result = await client.atomic(lambda plan: (
        plan.create(type_id=1, payload={"name": "Bob"}, alias="user"),
        plan.create(type_id=2, payload={"title": "Task"}, alias="task"),
        plan.link(edge_type_id=100, from_id="$task.id", to_id="$user.id"),
    ))
```

## Development

### Gateway

```bash
cd gateway
pip install -r ../requirements.txt

# Run with auto-reload
uvicorn app:app --reload

# Run tests
pytest
```

### Frontend

```bash
cd frontend
npm install

# Development server
npm run dev

# Build for production
npm run build

# Type check
npm run lint
```

## Docker Build

```bash
# Build gateway image
docker build -t entdb-gateway .

# Run standalone
docker run -p 8080:8080 \
  -e GATEWAY_ENTDB_HOST=host.docker.internal \
  entdb-gateway
```
