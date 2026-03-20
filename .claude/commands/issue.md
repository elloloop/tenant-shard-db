# Create GitHub Issue and Set Up Workspace

Start a new unit of work. Create a GitHub issue (or sub-issue), set up a branch, and prepare the workspace.

## Arguments

$ARGUMENTS: `<title> [-- <description>] [--parent <issue#>]`

Examples:
- `/issue Add full-text search to console -- Implement FTS across all node types`
- `/issue Implement mailbox pagination --parent 42`

If no arguments provided, ask the user.

## Steps

### 1. Parse arguments

- Title: everything before `--`
- Description: everything between `--` and `--parent` (if present)
- Parent: issue number after `--parent` (if present)

### 2. Build context chain (if sub-issue)

If `--parent` is specified, walk the parent chain:

```bash
gh issue view <parent>
```

Check the parent's body for `Parent: #<number>`. If found, read that too. Repeat until root.

### 3. Create the issue

**Sub-issue (with parent):**
```bash
gh issue create --title "<title>" --body "$(cat <<'EOF'
## Context Chain

<full context from root to parent>

---

## Task

<description>

## Required Test Deliverables

Every sub-issue MUST include regression tests:

- [ ] **Unit tests** (pytest) for new Python functions/methods
- [ ] **Integration tests** for new gRPC endpoints or storage operations
- [ ] **Playwright tests** if the issue adds or modifies a console/playground feature
- [ ] All tests pass: `make test` and `npx playwright test`

Parent: #<parent>
EOF
)" --assignee @me
```

### 4. Update parent (if sub-issue)

```bash
gh issue comment <parent> --body "Sub-issue created: #<new-number> — <title>"
```

### 5. Create branch

```bash
git checkout main && git pull && git checkout -b <number>-<slugified-title>
```

### 6. Post progress comment

```
gh issue comment <number> --body "## Progress

- [x] Issue created
- [x] Branch \`<branch-name>\` created
- [ ] Development
- [ ] Unit/integration tests written
- [ ] Playwright tests written (if UI changes)
- [ ] All tests passing
- [ ] Docker tested
- [ ] PR created

---
_Tracking automated by Claude Code_"
```

### 7. Save state

Write `.claude/issue` with the issue number.

### 8. Report

Show issue URL and branch name. Suggest next steps:
- `/split` if this needs multiple components
- `/contract` to update the proto
- `/backend-dev`, `/console-dev` to start building
