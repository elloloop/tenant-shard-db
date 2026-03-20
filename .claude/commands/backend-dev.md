# Backend — Build and Test Locally

Build, lint, and test the EntDB Python backend. Update GitHub issue with results.

## Steps

### 0. Check for context

If `.claude/issue` exists, read the issue and its comments for context on what's being built.

### 1. Install dependencies

```bash
pip install -e ".[dev]"
```

Or if using the Makefile and Docker:
```bash
make build
```

### 2. Lint

```bash
ruff check .
ruff format --check .
```

Fix any errors. Don't fail on warnings.

### 3. Type check

```bash
mypy dbaas sdk --exclude '(_generated|api/generated)'
```

### 4. Run unit tests

```bash
pytest tests/unit -v --cov=dbaas --cov=sdk --cov-report=term-missing
```

Capture pass/fail counts and coverage.

### 5. Run integration tests

```bash
pytest tests/integration -v
```

### 6. Update GitHub issue

Read issue number from `.claude/issue`. If it exists:

```
gh issue comment <number> --body "### Backend — Local Dev Results

**Lint (ruff)**: Passed / Failed
**Type check (mypy)**: Passed / Failed
**Unit tests**: X passed / X failed
**Integration tests**: X passed / X failed
**Coverage**: X%

<details>
<summary>Test output</summary>

\`\`\`
<last 50 lines>
\`\`\`
</details>

_Run at $(date -u +%Y-%m-%dT%H:%M:%SZ)_"
```

### 7. Report

Summarize. If passed, suggest `/backend-docker`. If failed, offer to fix.
