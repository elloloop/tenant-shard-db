# EntDB - Production-Grade Database-as-a-Service

EntDB is a production-ready multi-tenant database service built on event sourcing principles. It provides strong durability guarantees, flexible schema evolution, and built-in support for graph data models with nodes and edges.

## Features

- **Event Sourcing Architecture**: WAL (Write-Ahead Log) as source of truth with SQLite as derived views
- **Multi-Tenant Isolation**: Complete data isolation per tenant with per-tenant SQLite databases
- **Graph Data Model**: Nodes and unidirectional edges with typed schemas
- **Schema Evolution**: Protobuf-like evolution with stable numeric IDs and compatibility checking
- **Strong Durability**: Kafka/Kinesis WAL with `acks=all`, S3 archiving, and periodic snapshots
- **Full-Text Search**: SQLite FTS5 for mailbox search
- **ACL System**: Principal-based visibility (user:X, role:X, tenant:*)
- **Idempotent Operations**: Deduplication via idempotency keys
- **gRPC + HTTP APIs**: Both APIs for flexibility
- **Python SDK**: Type-safe client with Plan builder

## Quick Start

### Using Docker Compose

```bash
# Start all services
docker-compose up -d

# Check health
curl http://localhost:8080/health
```

### Install SDK

```bash
pip install entdb-sdk
```

### Basic Usage

```python
from entdb_sdk import DbClient, NodeTypeDef, EdgeTypeDef, field

# Define schema
User = NodeTypeDef(
    type_id=1,
    name="User",
    fields=(
        field(1, "email", "str", required=True),
        field(2, "name", "str"),
    ),
)

Task = NodeTypeDef(
    type_id=2,
    name="Task",
    fields=(
        field(1, "title", "str", required=True),
        field(2, "status", "enum", enum_values=("todo", "doing", "done")),
    ),
)

AssignedTo = EdgeTypeDef(
    edge_id=100,
    name="AssignedTo",
    from_type=2,
    to_type=1,
)

# Use client
async with DbClient(
    endpoint="localhost:50051",
    tenant_id="my_company",
    actor="user:alice",
) as client:
    # Create user and task atomically
    result = await client.atomic(lambda plan: (
        plan.create(User, {"email": "alice@example.com", "name": "Alice"}, alias="alice"),
        plan.create(Task, {"title": "Review PR", "status": "todo"}, alias="task"),
        plan.link(AssignedTo, "$task.id", "$alice.id"),
    ))
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Clients                             │
│              (gRPC / HTTP / Python SDK)                     │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────┐
│                    EntDB Server                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │ gRPC Server  │  │ HTTP Server  │  │ Schema       │       │
│  │   :50051     │  │   :8080      │  │ Registry     │       │
│  └──────┬───────┘  └──────┬───────┘  └──────────────┘       │
│         │                 │                                  │
│  ┌──────▼─────────────────▼─────┐                           │
│  │         WAL Stream           │                           │
│  │    (Kafka / Kinesis)         │                           │
│  └──────────────┬───────────────┘                           │
│                 │                                            │
│  ┌──────────────▼───────────────┐                           │
│  │         Applier              │                           │
│  │  (Idempotent Event Apply)    │                           │
│  └──────────────┬───────────────┘                           │
│                 │                                            │
│  ┌──────────────▼───────────────┐   ┌──────────────┐        │
│  │     Canonical Store          │   │   Mailbox    │        │
│  │    (SQLite per tenant)       │   │  (FTS5)      │        │
│  └──────────────────────────────┘   └──────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

## Documentation

- [Getting Started](docs/getting-started.md)
- [Architecture Overview](docs/README.md)
- [Schema Evolution](docs/schema-evolution.md)
- [Durability Guarantees](docs/durability.md)
- [Deployment Guide](docs/deployment.md)
- [SDK Reference](docs/sdk-reference.md)
- [API Reference](docs/api-reference.md)
- [Operations Guide](docs/operations.md)

## Project Structure

```
.
├── dbaas/
│   └── entdb_server/
│       ├── schema/          # Schema types and registry
│       ├── wal/             # WAL stream abstraction
│       ├── apply/           # Applier and stores
│       ├── api/             # gRPC and HTTP servers
│       ├── archive/         # S3 archiving
│       ├── snapshot/        # SQLite snapshots
│       ├── tools/           # CLI tools
│       └── diagrams/        # Mermaid diagrams
├── sdk/
│   └── entdb_sdk/           # Python SDK
├── examples/
│   └── fastapi_app/         # Sample FastAPI application
├── tests/
│   ├── unit/                # Unit tests
│   ├── integration/         # Integration tests
│   └── e2e/                 # End-to-end tests
├── docs/                    # Documentation
├── .github/
│   └── workflows/           # CI/CD pipelines
├── Dockerfile               # Multi-stage Docker build
├── docker-compose.yml       # Local development stack
└── pyproject.toml           # Python project configuration
```

## Development

### Prerequisites

- Python 3.10+
- Docker and Docker Compose
- (Optional) Protocol Buffers compiler

### Setup

```bash
# Clone repository
git clone https://github.com/your-org/entdb.git
cd entdb

# Create virtual environment
python -m venv venv
source venv/bin/activate

# Install dependencies
pip install -e ".[dev]"

# Start services
docker-compose up -d

# Run tests
pytest tests/unit -v
pytest tests/integration -v
```

### Running E2E Tests

```bash
# Set E2E flag
export ENTDB_E2E_TESTS=1

# Run E2E tests
pytest tests/e2e -v
```

## CI/CD

The project includes GitHub Actions workflows for:

- **CI** (`.github/workflows/ci.yml`):
  - Linting (ruff)
  - Type checking (mypy)
  - Unit tests
  - Integration tests
  - Docker build
  - E2E tests
  - Security scan

- **CD** (`.github/workflows/cd.yml`):
  - Multi-arch Docker image build
  - Push to GHCR
  - PyPI SDK publishing
  - Staging/Production deployment

## Configuration

Environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `ENTDB_GRPC_PORT` | gRPC server port | 50051 |
| `ENTDB_HTTP_PORT` | HTTP server port | 8080 |
| `ENTDB_KAFKA_BROKERS` | Kafka bootstrap servers | localhost:9092 |
| `ENTDB_S3_BUCKET` | S3 archive bucket | - |
| `ENTDB_DATA_DIR` | SQLite data directory | /var/lib/entdb |
| `ENTDB_LOG_LEVEL` | Log level | INFO |

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests
5. Submit a pull request

Please ensure:
- All tests pass
- Code is formatted with ruff
- Type hints are included
- Documentation is updated
