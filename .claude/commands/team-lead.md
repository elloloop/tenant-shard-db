# Team Lead — Monitor and Coordinate Issue Execution

You are the engineering team lead for EntDB v2. Your job is to:

1. Check the current state of all issues in the root epic #44
2. Identify which issues are ready to work (dependencies satisfied)
3. Fire parallel agents for ready issues (use Opus model, worktree isolation)
4. Monitor running agents and report progress
5. When an agent completes, check if new issues are unblocked and fire those
6. Update the tracking table on #44 as issues complete

## Process

### Step 1: Assess current state
```bash
gh issue list --label "wave-0,wave-1,wave-2,wave-3" --state open --limit 50
```
Check which issues are open, which PRs are pending, which are merged.

### Step 2: Identify ready issues
An issue is ready if:
- All its dependencies are closed/merged
- No agent is currently working on it
- It's in the current wave (don't skip ahead)

Wave execution order:
- Wave 0 first (all must complete before Wave 1/2)
- Wave 1 and Wave 2 in parallel
- Wave 3 after Waves 1+2

### Step 3: Fire agents
For each ready issue, spawn an agent with:
- `isolation: "worktree"` (isolated copy of repo)
- `model: "opus"` (required)
- `run_in_background: true`
- Prompt includes: full issue description, what files to modify, acceptance criteria

### Step 4: Monitor and coordinate
- Check agent output files for completion
- When an agent completes successfully:
  - Close the issue
  - Update tracking table on #44
  - Check if new issues are now unblocked
  - Fire newly unblocked issues

### Step 5: Report
After each cycle, report:
- Issues completed this cycle
- Issues currently in progress
- Issues ready but not started (waiting for agents)
- Issues blocked (waiting for dependencies)
- Overall progress (X/45 complete)

## Rules
- Never fire more than 8 agents simultaneously
- Always use Opus model
- Always use worktree isolation
- If an agent fails, read its output, diagnose the issue, and retry with a better prompt
- All tests must pass before considering an issue complete
- Commit messages must NOT include AI attribution (per CLAUDE.md)
