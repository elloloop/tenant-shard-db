# Console — Deploy to Staging

Deploy the EntDB Console to staging and run smoke tests. Update GitHub issue.

## Steps

### 1. Detect deployment platform

The console is typically deployed as part of the full EntDB stack via Docker Compose or Kubernetes.

| Config | Platform | Command |
|---|---|---|
| `fly.toml` | Fly.io | `fly deploy` |
| k8s manifests | Kubernetes | `kubectl apply -n staging` |
| `docker-compose.staging.yml` | Docker on remote | SSH + docker compose |
| GitHub Actions | GH Actions | `gh workflow run deploy.yml` |

If no config found, ask the user.

### 2. Deploy

Run the platform-specific deploy command. Capture the deployment URL.

### 3. Wait for deployment

```bash
until curl -sf <url>/health > /dev/null; do sleep 5; done
```

### 4. Smoke tests

Run Playwright smoke tests against the staging URL:
```bash
cd tests/playwright && CONSOLE_URL=<staging-url> npx playwright test --project=console --grep @smoke && cd ../..
```

If no `@smoke` tagged tests, do basic checks:
```bash
curl -f <url>/health
curl -f <url>/api/schema
```

### 5. Update GitHub issue

```
gh issue comment <number> --body "### Console — Staging Deployment

**URL**: <staging-url>
**Health**: Passed / Failed
**Smoke tests**: Passed / Failed

_Deployed at $(date -u +%Y-%m-%dT%H:%M:%SZ)_"
```

### 6. Report

Show staging URL and test results.
