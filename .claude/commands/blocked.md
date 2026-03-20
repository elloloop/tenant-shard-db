# Blocked — Escalate a Missing Dependency

A feature agent has discovered it needs something that doesn't exist yet — a proto RPC, a shared utility, a schema type, etc. This skill stops work and escalates.

**RULE: Feature agents must NEVER create things outside their own assigned directory. If something is missing, call `/blocked`.**

## Arguments

$ARGUMENTS: description of what's needed.
Example: `/blocked Need a GetNodesByOwner RPC to filter nodes by owner_actor`

## Steps

### 1. Document the blocker

Identify exactly what's missing:
- **Missing proto RPC**: a gRPC method that doesn't exist in `entdb.proto`
- **Missing schema type**: a node/edge type or field kind not supported
- **Missing SDK method**: a client method that doesn't exist
- **Missing gateway endpoint**: a REST endpoint not exposed by the console gateway
- **Missing shared component**: a React component needed by console/playground

### 2. Check if it already exists

```bash
grep -r "<keyword>" dbaas/entdb_server/api/proto/ sdk/entdb_sdk/ console/gateway/ console/frontend/src/
```

If it exists but the agent didn't know, report where it is and continue work.

### 3. If truly missing — STOP and escalate

Comment on the current issue:

```
gh issue comment <current-issue> --body "### Blocked

**Needs**: <what's missing>
**Why**: <why this feature needs it>
**Scope**: <proto change / schema change / SDK method / gateway endpoint>

Work on this issue is paused until the dependency is resolved."
```

### 4. Escalate to parent issue

```
gh issue comment <parent-issue> --body "### Dependency needed

**Reported by**: #<current-issue>
**Needs**: <what's missing>
**Why**: <why it's needed>
**Suggested scope**: <what component needs to change>

This blocks progress on #<current-issue>."
```

### 5. Report

Tell the user what's missing and suggest `/resolve <parent-issue#>`.

## What agents must NEVER do when blocked

- Create a proto RPC themselves (that's the contract owner's job)
- Add a schema type outside the schema system
- Import from another feature/module directly
- Create workarounds or mocks
- Continue with stubs and "fix later"
