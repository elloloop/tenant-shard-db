# Backend — Deploy to Staging

Deploy the EntDB backend to staging and run smoke tests. Update GitHub issue.

## Steps

### 1. Detect deployment method

| Config | Platform | Command |
|---|---|---|
| `fly.toml` | Fly.io | `fly deploy` |
| k8s manifests in `k8s/` or `deploy/` | Kubernetes | `kubectl apply -n staging` |
| `docker-compose.staging.yml` | Docker on remote | SSH + docker compose |
| GitHub Actions deploy workflow | GH Actions | `gh workflow run deploy.yml -f environment=staging` |

If nothing found, ask the user.

### 2. Deploy

Run the appropriate deploy command. Capture endpoint URL.

### 3. Wait for deployment

Poll the health endpoint until it responds (timeout 5 minutes):
```bash
# gRPC health check
python -c "
import grpc
ch = grpc.insecure_channel('<host>:50051')
grpc.channel_ready_future(ch).result(timeout=30)
print('Server healthy')
"
```

### 4. Smoke tests

```bash
# gRPC smoke test
python -c "
from entdb_sdk._generated import entdb_pb2, entdb_pb2_grpc
import grpc
ch = grpc.insecure_channel('<host>:50051')
stub = entdb_pb2_grpc.EntDBServiceStub(ch)
resp = stub.Health(entdb_pb2.HealthRequest())
print(f'Health: {resp}')
resp = stub.ListTenants(entdb_pb2.ListTenantsRequest())
print(f'Tenants: {resp}')
"
```

### 5. Update GitHub issue

```
gh issue comment <number> --body "### Backend — Staging Deployment

**URL**: <staging-url>
**Health**: Passed / Failed
**Smoke tests**: Passed / Failed

_Deployed at $(date -u +%Y-%m-%dT%H:%M:%SZ)_"
```

### 6. Report

Show staging URL. If passed, PR is ready for review.

## Important
- Never deploy to production
- If credentials needed, ask user
- Don't log secrets
