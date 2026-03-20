# Console — Build and Test Locally

Build, lint, and run Playwright tests for the EntDB Console (data browser). Update GitHub issue.

## Steps

### 0. Check for context

If `.claude/issue` exists, read the issue for context.

### 1. Install dependencies

```bash
cd console/frontend && npm install && cd ../..
cd tests/playwright && npm install && cd ../..
```

### 2. Lint

```bash
cd console/frontend && npm run lint && cd ../..
```

Don't fail on warnings, only errors.

### 3. Build

```bash
cd console/frontend && npm run build && cd ../..
```

If it fails, try to fix and retry once.

### 4. Start the full stack (needed for Playwright)

```bash
docker compose up -d --build
```

Wait for console to be healthy:
```bash
until curl -sf http://localhost:8080/health > /dev/null; do sleep 5; done
```

### 5. Run Playwright tests

```bash
cd tests/playwright && npx playwright test --project=console && cd ../..
```

Capture output including pass/fail counts.

### 6. Tear down

```bash
docker compose down -v
```

### 7. Update GitHub issue

```
gh issue comment <number> --body "### Console — Local Dev Results

**Lint**: Passed / Failed
**Build**: Passed / Failed
**Playwright**: X passed / X failed

<details>
<summary>Test output</summary>

\`\`\`
<last 50 lines>
\`\`\`
</details>

_Run at $(date -u +%Y-%m-%dT%H:%M:%SZ)_"
```

### 8. Report

Summarize. If passed, suggest `/console-docker` or `/pr`. If failed, offer to fix.
