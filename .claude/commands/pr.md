# Create Pull Request

Create a pull request linking the GitHub issue, with test results and a clear description.

## Steps

### 1. Get context

- Read issue number from `.claude/issue`
- `gh issue view <number>` — get title, body, check for parent issue
- Get current branch name
- `git log main..HEAD --oneline` — all commits
- `git diff main...HEAD --stat` — changed files

### 2. Detect parent issue

Look in the issue body for `Parent: #<number>`.

### 3. Rebase on main

```bash
git fetch origin
git rebase origin/main
```

If conflicts in generated code (`_generated/`, `api/generated/`), prefer incoming main changes. Re-run build after rebase.

### 4. Push branch

```bash
git push -u origin <branch-name> --force-with-lease
```

### 5. Create PR

```bash
gh pr create --title "<concise title under 70 chars>" --body "$(cat <<'EOF'
## Summary

<2-4 bullet points describing changes and why>

## Changes

<key files/areas changed>

## Test results

- **Lint (ruff)**: Passed / Not run
- **Type check (mypy)**: Passed / Not run
- **Unit tests (pytest)**: Passed / Not run
- **Integration tests**: Passed / Not run
- **Playwright tests**: Passed / Not run
- **Docker E2E**: Passed / Not run

## Test plan

- [ ] <specific things to verify>

Closes #<issue-number>
EOF
)"
```

### 6. Update the issue

```
gh issue comment <number> --body "### PR Created

PR: #<pr-number> — <pr-title>
Branch: \`<branch-name>\`
Changed files: <count>"
```

### 7. Update parent issue (if sub-issue)

```
gh issue comment <parent-number> --body "### Update: #<number>
PR created: #<pr-number>
Status: ready for review"
```

### 8. Report

Show PR URL. Suggest deploy as next step.

## Important
- Always include `Closes #<issue-number>` in PR body
- Don't include AI attribution in commits or PR
- Review the diff before creating
