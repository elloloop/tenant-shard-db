# EntDB Console

Web-based administration and data browsing tool for EntDB, similar to phpMyAdmin or DBeaver.

## Features

- **Data Browser**: Navigate nodes by type, view details, explore relationships
- **Graph Visualization**: Force-directed graph view of node neighborhoods
- **Full-Text Search**: Search across mailbox items
- **REST API**: HTTP endpoints for programmatic access
- **Schema Viewer**: Browse registered types and their fields

## Quick Start

```bash
# Start with docker-compose (includes EntDB server)
docker-compose up -d

# Open browser
open http://localhost:8080
```

Or run standalone connecting to an existing EntDB server:

```bash
# Set EntDB server address
export CONSOLE_ENTDB_HOST=your-entdb-host
export CONSOLE_ENTDB_PORT=50051

# Run console
docker run -p 8080:8080 \
  -e CONSOLE_ENTDB_HOST \
  -e CONSOLE_ENTDB_PORT \
  ghcr.io/your-org/entdb-console
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Browser                              │
│                    React SPA (Vite)                          │
└─────────────────────────┬───────────────────────────────────┘
                          │ HTTP
┌─────────────────────────▼───────────────────────────────────┐
│                    EntDB Console                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              FastAPI Backend                          │   │
│  │  • REST API (/api/v1/*)                              │   │
│  │  • Static file serving (React build)                 │   │
│  │  • SDK connection pool                               │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────┬───────────────────────────────────┘
                          │ gRPC
┌─────────────────────────▼───────────────────────────────────┐
│                    EntDB Server                              │
│                    (gRPC :50051)                             │
└─────────────────────────────────────────────────────────────┘
```

## Screenshots

### Type Browser
Browse all registered node types in the sidebar. Click to view nodes of that type.

### Node List
Paginated list of nodes with payload preview. Filter by type, navigate pages.

### Node Detail
View full payload, metadata, and connected edges. Links to related nodes.

### Graph View
Interactive force-directed graph visualization. Explore node neighborhoods up to 3 levels deep.

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

### Search & Browse

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/search?q=query` | Full-text search |
| GET | `/api/v1/browse/types` | Get types for sidebar |
| GET | `/api/v1/browse/graph/{id}` | Get node neighborhood |

### Atomic Operations

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/atomic` | Execute atomic transaction |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `CONSOLE_ENTDB_HOST` | localhost | EntDB gRPC host |
| `CONSOLE_ENTDB_PORT` | 50051 | EntDB gRPC port |
| `CONSOLE_HOST` | 0.0.0.0 | Console bind host |
| `CONSOLE_PORT` | 8080 | Console bind port |
| `CONSOLE_DEFAULT_TENANT_ID` | default | Default tenant |
| `CONSOLE_CORS_ORIGINS` | ["http://localhost:3000"] | CORS origins |

## Development

### Backend

```bash
cd console
pip install -r requirements.txt
pip install -e ../sdk  # Install EntDB SDK

# Run with auto-reload
uvicorn gateway.app:app --reload --port 8080
```

### Frontend

```bash
cd frontend
npm install

# Development server with hot reload
npm run dev

# Build for production
npm run build
```

### Full Stack

```bash
# Start EntDB and dependencies
docker-compose up -d entdb redpanda minio minio-init

# Run console locally
uvicorn gateway.app:app --reload --port 8080 &
cd frontend && npm run dev
```

## Docker Build

```bash
# Build console image
docker build -t entdb-console .

# Run
docker run -p 8080:8080 \
  -e CONSOLE_ENTDB_HOST=host.docker.internal \
  entdb-console
```

## Project Structure

```
console/
├── gateway/                # FastAPI backend
│   ├── app.py              # Application factory
│   ├── config.py           # Configuration
│   ├── routes.py           # API endpoints
│   └── sdk_client.py       # SDK connection pool
├── frontend/               # React frontend
│   ├── src/
│   │   ├── components/     # Reusable components
│   │   ├── pages/          # Page components
│   │   ├── api.ts          # API client
│   │   └── App.tsx         # Main app
│   └── package.json
├── Dockerfile              # Multi-stage build
├── docker-compose.yml      # Full dev stack
└── requirements.txt        # Python dependencies
```
