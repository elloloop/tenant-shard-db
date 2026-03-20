# Console — Docker Compose E2E Test

Build with Docker Compose, run the full stack, test the Console with Playwright, tear down. Update GitHub issue.

## Steps

### 1. Build

```bash
docker compose build --no-cache
```

### 2. Start

```bash
docker compose up -d
```

Wait for all services to be healthy (timeout 120s):
- Server: gRPC health check on port 50051
- Console: `http://localhost:8080/health`
- Playground: `http://localhost:8081/health`

### 3. Seed test data

Use the SDK to create test nodes and edges so Playwright has data to browse:

```bash
python -c "
import asyncio
from entdb_sdk.client import DbClient

async def seed():
    async with DbClient('localhost:50051', tenant_id='playwright') as db:
        plan = db.plan()
        plan.create(1, {'name': 'Test Node', 'status': 'active'}, as_='node1')
        await plan.commit(wait_applied=True)
        print('Test data seeded')

asyncio.run(seed())
"
```

### 4. Run Playwright against running containers

```bash
cd tests/playwright && npx playwright test && cd ../..
```

### 5. Capture logs

```bash
docker compose logs --tail=30
```

### 6. Tear down

```bash
docker compose down -v
```
Always run, even if tests fail.

### 7. Update GitHub issue

```
gh issue comment <number> --body "### Console — Docker E2E Results

**Docker build**: Passed / Failed
**All services healthy**: Passed / Failed
**Playwright E2E**: X passed / X failed

<details>
<summary>Container logs</summary>

\`\`\`
<logs>
\`\`\`
</details>

_Run at $(date -u +%Y-%m-%dT%H:%M:%SZ)_"
```

### 8. Report

If passed, suggest `/pr`. If failed, show errors and offer to fix.
