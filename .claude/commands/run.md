# Run — Execute Issues in Parallel

Execute a wave of issues in parallel using sub-agents in isolated worktrees.

## Arguments

$ARGUMENTS: root issue number, or `next` to auto-detect the next wave.
- `/run 100` → read issue #100's tracking table, execute the next unblocked wave
- `/run 100 wave 2` → execute specifically wave 2

## Steps

### 1. Read the root issue

```bash
gh issue view <number> --comments
```

Parse the tracking table. Identify sub-issues, their status, and dependencies.

### 2. Determine the next wave

Find the next wave where all dependencies are completed and merged.

### 3. Verify prerequisites

- Check that dependency issues have merged PRs
- Pull latest main: `git checkout main && git pull`
- Verify proto generated code exists if needed

### 4. Launch parallel agents

For each issue in the wave, launch a sub-agent with `isolation: "worktree"`:

**Determine the right ship command from issue title prefix:**
- `[Proto]` → update proto, generate, commit, PR
- `[Server]` → run `/backend-ship <issue-number>`
- `[SDK]` → implement SDK changes, test, PR
- `[Console]` → run `/console-ship <issue-number>`
- `[Playwright]` → write and run Playwright tests, PR

**Agent instructions include:**
- Full issue context
- EntDB-specific scope rules (only modify files in assigned directory)
- Test requirements (pytest for Python, Playwright for frontends)
- Self-review checklist
- PR creation with `Closes #<number>`

Launch ALL agents in the wave simultaneously. Run them in the background.

### 5. Collect results and verify test coverage

As agents complete:
- Check PRs were created
- Verify test files exist proportional to source changes
- Flag any agents with missing tests

### 6. Update root issue

```
gh issue comment <root> --body "### Wave <N> Complete

| Issue | Component | Status | PR | Tests |
|---|---|---|---|---|
| #<issue> | <type> | status | #<pr> | src/test count |

**Next**: merge PRs, then run next wave"
```

### 7. Report

Show results, PR links, and next wave instructions.

## Execution Model

```
Wave 0: [Proto] changes → 1 agent
  ↓ merge to main, regenerate

Wave 1: [Server], [SDK] → 2 agents in parallel
  ↓ merge to main

Wave 2: [Console], [Playground] → 2 agents in parallel
  ↓ merge to main

Wave 3: [Playwright] end-to-end tests → 1 agent
  ↓ merge

Done.
```

## Important
- Verify previous wave is merged before running next
- Each agent runs in isolated worktree
- Proto waves merge before server/SDK waves
- Server waves merge before console/playground waves
- Never auto-merge PRs
