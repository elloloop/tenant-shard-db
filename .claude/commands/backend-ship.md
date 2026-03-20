# Backend — Full Ship Pipeline

Run the complete backend pipeline for a GitHub issue. Every step updates the issue.

## Arguments

$ARGUMENTS should be a GitHub issue number.
If not provided, read from `.claude/issue`. If that doesn't exist, ask.

## Pipeline

Execute in order, stop on failure:

### 1. Setup
- `gh issue view <number>` — read the issue body AND all comments
- Read the context chain to understand the full feature scope
- Check out or create the issue branch (branch from main)
- Save issue number to `.claude/issue`

### 2. Implement
- From the issue context, understand what server changes are needed
- Implement in `dbaas/entdb_server/` and/or `sdk/entdb_sdk/`
- Write unit tests in `tests/unit/`
- Write integration tests in `tests/integration/` if applicable

### 3. Local dev (same as `/backend-dev`)
- `pip install -e ".[dev]"`
- `ruff check .` and `ruff format --check .`
- `mypy dbaas sdk --exclude '(_generated|api/generated)'`
- `pytest tests/unit -v --cov=dbaas --cov=sdk`
- `pytest tests/integration -v`
- Comment on issue
- **Stop if failed** — offer to fix

### 4. Principal engineer review (same as `/review`)
- Read every file you wrote or changed
- Check: security, error handling, performance, correctness, tests, scope
- Fix any issues found
- Comment on issue with review results
- **Stop if unfixable issues**

### 5. Docker integration (same as `/backend-docker`)
- `docker compose build && up -d`
- Health checks on server, console, playground
- `make e2e` if available
- `docker compose down -v`
- Comment on issue
- **Stop if failed** — offer to fix

### 6. Create PR (same as `/pr`)
- Push branch
- Create PR with `Closes #<number>`
- Comment on issue with PR link

### 7. Final summary

```
gh issue comment <number> --body "### Backend Ship Complete

| Step | Status |
|---|---|
| Lint (ruff) | Passed |
| Type check (mypy) | Passed |
| Unit tests | X passed |
| Integration tests | X passed |
| Coverage | X% |
| Docker integration | Passed / Skipped |
| PR | #<pr> |

Ready for review."
```

## Scope rules

- Only modify files relevant to the issue
- Server changes go in `dbaas/entdb_server/`
- SDK changes go in `sdk/entdb_sdk/`
- Tests go in `tests/`
- Never hand-edit generated code in `_generated/` or `api/generated/`
- If you need a proto change, STOP and run `/blocked`

## Other rules
- Stop on any failure, offer to fix
- Each step updates issue independently
- Never merge PR automatically
