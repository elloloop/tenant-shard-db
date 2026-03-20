# Backend — Docker Compose Integration Test

Build with Docker Compose, start the full EntDB stack, run integration tests, tear down. Update GitHub issue.

## Steps

### 1. Build

```bash
docker compose build --no-cache
```

### 2. Start

```bash
docker compose up -d
```

Wait for services to be healthy. The server has a gRPC health check, console and playground have HTTP health checks:
- Server: `grpc.health.v1.Health/Check` on port 50051
- Console: `http://localhost:8080/health`
- Playground: `http://localhost:8081/health`

Poll until all healthy (timeout 120s — Redpanda and MinIO need time).

### 3. Integration tests

**gRPC server:**
```bash
python -c "
import grpc
from entdb_sdk._generated import entdb_pb2, entdb_pb2_grpc
ch = grpc.insecure_channel('localhost:50051')
stub = entdb_pb2_grpc.EntDBServiceStub(ch)
resp = stub.Health(entdb_pb2.HealthRequest())
print(f'gRPC health: {resp}')
"
```

**Console gateway:**
```bash
curl -f http://localhost:8080/health
curl -f http://localhost:8080/api/schema
```

**Playground:**
```bash
curl -f http://localhost:8081/health
```

**Run E2E tests if available:**
```bash
make e2e
```

### 4. Capture logs

```bash
docker compose logs --tail=30
```

### 5. Tear down

```bash
docker compose down -v
```
Always run, even on failure.

### 6. Update GitHub issue

```
gh issue comment <number> --body "### Backend — Docker Integration Results

**Docker build**: Passed / Failed
**Server healthy**: Passed / Failed
**Console healthy**: Passed / Failed
**Playground healthy**: Passed / Failed
**E2E tests**: Passed / Failed / Skipped

<details>
<summary>Container logs</summary>

\`\`\`
<logs>
\`\`\`
</details>

_Run at $(date -u +%Y-%m-%dT%H:%M:%SZ)_"
```

### 7. Report

If passed, suggest `/playwright` then `/pr`. If failed, show errors.
