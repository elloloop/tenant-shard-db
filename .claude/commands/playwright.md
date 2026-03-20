# Playwright — Run Playwright Tests

Run Playwright tests against the EntDB Console and Playground. Requires the full stack to be running.

## Arguments

$ARGUMENTS: optional project filter.
- `/playwright` → run all Playwright tests
- `/playwright console` → run only console tests
- `/playwright playground` → run only playground tests
- `/playwright --headed` → run in headed mode for debugging

## Steps

### 1. Check if stack is running

```bash
curl -sf http://localhost:8080/health > /dev/null 2>&1 && echo "Console: up" || echo "Console: down"
curl -sf http://localhost:8081/health > /dev/null 2>&1 && echo "Playground: up" || echo "Playground: down"
```

If stack is not running:
```bash
docker compose up -d --build
# Wait for health
until curl -sf http://localhost:8080/health > /dev/null; do sleep 5; done
until curl -sf http://localhost:8081/health > /dev/null; do sleep 5; done
```

### 2. Install Playwright dependencies

```bash
cd tests/playwright
npm install
npx playwright install --with-deps chromium
```

### 3. Run tests

```bash
cd tests/playwright

# All tests
npx playwright test

# Or filtered by project
npx playwright test --project=console
npx playwright test --project=playground

# Headed mode for debugging
npx playwright test --headed
```

### 4. View results

If tests fail:
```bash
npx playwright show-report
```

### 5. Update GitHub issue (if applicable)

```
gh issue comment <number> --body "### Playwright Test Results

**Console tests**: X passed / X failed
**Playground tests**: X passed / X failed

<details>
<summary>Test output</summary>

\`\`\`
<output>
\`\`\`
</details>

_Run at $(date -u +%Y-%m-%dT%H:%M:%SZ)_"
```

### 6. Report

Summarize results. If failures, offer to fix or debug with `--headed` mode.
