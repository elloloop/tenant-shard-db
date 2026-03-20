# Split Issue into Sub-Issues

Take a parent issue and split it into component sub-issues for EntDB. Each sub-issue carries the full context chain but has NO knowledge of sibling issues.

## Arguments

$ARGUMENTS: `<component1> [component2] ...`

Components: `proto`, `server`, `sdk`, `console`, `playground`, `tests`

Examples:
- `/split server console` → creates: server + console + playwright tests
- `/split proto server sdk console` → creates all components
- `/split server` → creates: server only

If no arguments, ask the user.

## Steps

### 1. Build the full context chain

Read the current issue from `.claude/issue`:
```bash
gh issue view <number>
```
Walk UP the parent chain until root.

### 2. Create component sub-issues

For each component, create a sub-issue with full context chain + component-specific scope:

**Proto sub-issue:**
- Scope: `dbaas/entdb_server/api/proto/entdb.proto`
- Run `./scripts/generate_proto.sh` after changes
- Must pass schema compat check

**Server sub-issue:**
- Scope: `dbaas/entdb_server/`
- Tests: `tests/unit/`, `tests/integration/`
- Must pass `ruff check .`, `mypy dbaas sdk`, `pytest tests/unit -v`

**SDK sub-issue:**
- Scope: `sdk/entdb_sdk/`
- Tests: `tests/unit/test_sdk_*.py`
- Must pass `pytest tests/unit -v`

**Console sub-issue:**
- Scope: `console/frontend/`, `console/gateway/`
- Tests: `tests/playwright/console/`
- Must pass `npm run build` (in console/frontend/) and Playwright tests

**Playground sub-issue:**
- Scope: `playground/frontend/`
- Tests: `tests/playwright/playground/`
- Must pass `npm run build` and Playwright tests

**Tests sub-issue:**
- Scope: `tests/e2e/`, `tests/playwright/`
- Covers end-to-end validation of the full feature

### 3. Update parent issue with tracking table

```
gh issue comment <current> --body "### Split into sub-issues

| Component | Issue | Directory | Status |
|---|---|---|---|
| Proto | #<N> | dbaas/entdb_server/api/proto/ | Not started |
| Server | #<N> | dbaas/entdb_server/ | Not started |
| SDK | #<N> | sdk/entdb_sdk/ | Not started |
| Console | #<N> | console/ | Not started |
| Playwright | #<N> | tests/playwright/ | Not started |

**Order**: Proto first → Server/SDK in parallel → Console/Playground → Playwright tests"
```

### 4. Report

Show the breakdown and execution order.

## Important
- Every sub-issue includes full context chain but NO sibling knowledge
- Proto changes must merge first (other components need generated code)
- Server and SDK can be parallel after proto
- Console/Playground depend on server API being stable
- Playwright tests come last to validate everything
