# Resolve — Handle a Blocked Dependency

A feature agent has reported a missing dependency via `/blocked`. This skill figures out if the dependency already exists, creates the right issue to fulfill it, and sets up the dependency chain.

## Arguments

$ARGUMENTS: parent issue number.
Example: `/resolve 100`

## Steps

### 1. Read all blocker reports

```bash
gh issue view <parent> --comments
```

Find all "Dependency needed" comments. Collect what's needed and who needs it.

### 2. Deduplicate and analyze

Group blockers by what they need. Multiple sub-issues might need the same thing.

### 3. Check if it already exists

```bash
grep -r "<keyword>" dbaas/entdb_server/api/proto/ sdk/entdb_sdk/ console/gateway/
```

If found, comment on the blocked issue and unblock.

### 4. Determine the right resolution

| What's missing | Resolution |
|---|---|
| Proto RPC or message | Create a contract update issue |
| Schema type or field kind | Create a schema system issue |
| SDK client method | Create an SDK issue |
| Gateway REST endpoint | Create a console gateway issue |
| Shared React component | Create a console/playground issue |

### 5. Create resolution issue(s)

```bash
gh issue create --title "[<type>] <what's needed>" --body "$(cat <<'EOF'
## Context Chain
<walk up from parent to root>

---

## Task
<what needs to be created/updated>

### Requested by
- #<blocked-issue>: needs <X> because <reason>

### Acceptance criteria
- [ ] <thing> exists and is available
- [ ] Tests pass
- [ ] Merged to main

Parent: #<parent>
EOF
)" --assignee @me
```

### 6. Update blocked issues

```
gh issue comment <blocked-issue> --body "### Dependency tracked

The missing dependency has been tracked as #<resolution-issue>.
This issue remains blocked until #<resolution-issue> is merged to main.

Depends on: #<resolution-issue>"
```

### 7. Report

Show what was created and the new execution order.

## Important
- Always check if the dependency already exists before creating new issues
- If the same thing is needed by multiple features, create ONE resolution issue
- Blocked issues must NOT continue until the resolution is merged to main
