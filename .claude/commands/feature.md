# Feature — Design and Plan a Feature in an Existing App

Talk to the user to understand the feature, audit the existing EntDB codebase, figure out what needs changes vs what's new, create the issue tree, and set up work so each piece can be built independently.

## Phase 1: Understand the Feature

Have a conversation with the user:
- What does this feature do? (one-sentence summary)
- Who is it for? (SDK users, console users, server operators)
- What's the user journey? (step by step)

## Phase 2: Audit the Existing Codebase

Before designing anything, understand what already exists:

### Scan proto definition
```bash
cat dbaas/entdb_server/api/proto/entdb.proto | head -100
```
Note existing RPCs and message types.

### Scan server modules
```bash
ls dbaas/entdb_server/
ls dbaas/entdb_server/apply/
ls dbaas/entdb_server/wal/
ls dbaas/entdb_server/schema/
```

### Scan SDK
```bash
ls sdk/entdb_sdk/
```

### Scan console
```bash
ls console/frontend/src/pages/
ls console/gateway/
```

### Scan playground
```bash
ls playground/frontend/src/
```

### Check tests
```bash
ls tests/unit/ tests/integration/ tests/e2e/ tests/playwright/ 2>/dev/null
```

Build a map of what exists and what's affected by the new feature.

## Phase 3: Design the Feature

Categorize every piece of work:

| Category | Example | How it's handled |
|---|---|---|
| **Proto change** | Add new RPC to `entdb.proto` | Update proto, regenerate, new issue |
| **Schema change** | New node/edge type support | Update schema types, compat check |
| **Server change** | New gRPC handler | Change in `dbaas/entdb_server/`, new issue |
| **SDK change** | New client method | Change in `sdk/entdb_sdk/`, new issue |
| **Console change** | New page or view | Change in `console/`, new issue |
| **Playground change** | New interactive feature | Change in `playground/`, new issue |

### Dependency analysis

1. Proto/schema changes come first — always
2. Server changes depend on proto
3. SDK changes depend on proto
4. Console/Playground changes depend on server API being available
5. Playwright tests cover the full flow

**Present the plan to the user before creating issues.**

## Phase 4: Create the Issue Tree

### 1. Create the feature root issue

```bash
gh issue create --title "<feature title>" --body "$(cat <<'EOF'
## Feature
<what it does, who it's for, user journey>

## Existing codebase context
<what already exists that this feature touches>

## Changes needed
### Proto/Schema
<changes to proto or schema system>
### Server
<gRPC handlers, applier, WAL, storage changes>
### SDK
<client method changes>
### Console
<UI changes>
### Playwright Tests
<what to test end-to-end>

## Execution order
<waves>
EOF
)" --assignee @me --label feature
```

### 2. Create sub-issues with full context chains (no sibling knowledge)

### 3. Update root issue with tracking table

## Phase 5: Report

Tell the user the full issue tree, dependencies, and where to start.

## Important
- Proto changes are backward-compatible only (add, never remove RPCs)
- Schema evolution must pass compat checks
- Every sub-issue must include Playwright test requirements where applicable
- Two issues on the SAME directory = sequential
- Two issues on DIFFERENT directories = parallel
