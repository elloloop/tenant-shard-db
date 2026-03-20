# Console — Full Ship Pipeline

Run the complete console pipeline for a GitHub issue. Every step updates the issue.

## Arguments

$ARGUMENTS should be a GitHub issue number.
If not provided, read from `.claude/issue`. If that doesn't exist, ask.

## Pipeline

Execute in order, stop on failure:

### 1. Setup
- `gh issue view <number>` — read issue body AND all comments
- Read the context chain
- Check out or create the issue branch
- Save issue number to `.claude/issue`

### 2. Implement
- From the issue context, understand what console changes are needed
- Frontend changes go in `console/frontend/src/`
- Gateway changes go in `console/gateway/`
- Write Playwright tests in `tests/playwright/console/`

### 3. Local dev (same as `/console-dev`)
- Install deps, lint, build
- Start stack with Docker Compose
- Run Playwright tests
- Comment on issue
- **Stop if failed**

### 4. Principal engineer review (same as `/review`)
- Read every changed file
- Check: security, UX, accessibility, error handling, tests
- Fix any issues
- Comment on issue

### 5. Docker E2E (same as `/console-docker`)
- Full Docker Compose build
- Seed test data
- Run Playwright against containers
- Tear down
- Comment on issue
- **Stop if failed**

### 6. Create PR (same as `/pr`)
- Push branch, create PR with `Closes #<number>`
- Comment on issue

### 7. Final summary

```
gh issue comment <number> --body "### Console Ship Complete

| Step | Status |
|---|---|
| Lint | Passed |
| Build | Passed |
| Playwright | X passed |
| Docker E2E | Passed / Skipped |
| PR | #<pr> |

Ready for review."
```

## Scope rules

- Frontend changes: `console/frontend/src/`
- Gateway changes: `console/gateway/`
- Tests: `tests/playwright/console/`
- Never modify server code — if you need a new API endpoint, run `/blocked`
- Never modify playground code

## Other rules
- Stop on any failure, offer to fix
- Each step updates issue independently
- Never merge PR automatically
