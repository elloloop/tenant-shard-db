---
name: freeze-decision
description: Record a frozen architectural/design decision in docs/decisions/, detect contradictions with existing decisions, and surface them before writing.
---

# freeze-decision

When the user says "freeze this decision", "lock this in", "record this decision", or similar phrases, invoke this skill to write a structured decision record.

## What this skill does

1. **Determines the topic** — the subject area this decision affects (e.g. `storage`, `auth`, `fanout`, `schema`, `api`, `billing`, `compliance`).
2. **Reads existing decisions** in `docs/decisions/{topic}.md` and any related topic files.
3. **Checks for contradictions** — scans existing decisions and reasons about whether the new decision conflicts with any prior one.
4. **Writes the new decision** with structured metadata. If there's a contradiction, asks the user whether to supersede the old decision (mark it as superseded and continue) or drop the new one.
5. **Updates an index** at `docs/decisions/INDEX.md` listing all frozen decisions with a one-line summary.

## File layout

```
docs/decisions/
  INDEX.md               — one-line summary of every frozen decision
  {topic}.md             — all decisions for a single topic, newest first
```

Each topic file contains one or more decision entries, separated by `---`. When a new decision on the same topic is frozen, append it to the top of the topic file with a link to any prior decision it supersedes.

## Decision entry format

```markdown
## {YYYY-MM-DD}: {short imperative title}

**Status:** frozen | superseded
**Decided:** {YYYY-MM-DD}
**Tags:** {comma-separated}
**Supersedes:** {link to prior decision, or "none"}
**Superseded by:** {link, filled in when this is later replaced}

### Decision

{One or two short paragraphs stating what was decided, in present tense.}

### Context

{Why this decision was needed. The problem or question that prompted it.}

### Alternatives considered

- **Option A:** {short description}. Rejected because {reason}.
- **Option B:** {short description}. Rejected because {reason}.

### Consequences

{What this decision locks in. Tradeoffs accepted. Things this makes easy or hard.}

### References

- Conversation thread: {session id or date}
- Related decisions: {links}
- Code: {file paths, if implementation followed}
```

## Contradiction detection

Before writing a new decision, read `docs/decisions/{topic}.md` and any obviously-related topic files. For each existing frozen decision, ask:

1. **Does the new decision reverse a prior decision?** (e.g. "drafts live in user.db" vs "drafts live in tenant.db")
2. **Does it change an invariant that was previously locked in?** (e.g. "edges are unidirectional" vs "edges can go either way")
3. **Does it introduce a mechanism that a prior decision explicitly rejected?** (e.g. "add built-in drafts primitive" after "do not add built-in drafts primitive")

If any of these are true, report the contradiction clearly:

```
Contradiction detected:
- New decision: {summary of new}
- Conflicts with: docs/decisions/{topic}.md#{YYYY-MM-DD}: {title of old}
- Specifically: {one-sentence explanation}

To proceed, choose one:
  A) Supersede — mark the old decision as superseded, write the new one.
  B) Drop new — abandon the new decision, keep the old one.
  C) Edit — clarify the new decision so the contradiction is resolved.
```

Wait for user confirmation before writing when a contradiction exists.

When superseding, update the old entry's `Status:` to `superseded` and its `Superseded by:` to point at the new entry's anchor. The old entry stays in the file (never delete decisions — they're historical record).

## INDEX.md format

```markdown
# Frozen Decisions — Index

## Topics

### storage
- **2026-04-13** — [Use immutable storage mode, no built-in drafts primitive](storage.md#2026-04-13-use-immutable-storage-mode-no-built-in-drafts-primitive) — frozen
- **2026-04-11** — [Per-tenant SQLite default, opt-in user mailbox](storage.md#2026-04-11-per-tenant-sqlite-default) — superseded by 2026-04-13

### auth
- **2026-04-12** — [Auth identity via ContextVar, never trust request.actor](auth.md#2026-04-12-auth-identity-via-contextvar) — frozen

### fanout
- ...
```

Index is rebuilt from topic files on each freeze (don't try to maintain it incrementally).

## When NOT to freeze

Do not freeze:
- **Implementation details** that can change without affecting architecture (e.g. "use grpc-go v1.80.0")
- **Ephemeral decisions** scoped to a single PR
- **Personal preferences** without architectural consequences
- **Things already documented** in CLAUDE.md as invariants (those are already "frozen" at the repo level)

Freeze only **architectural choices** with lasting consequences: data model, API shape, consistency semantics, storage layout, security posture, ownership boundaries, failure modes.

If unsure whether a decision is freezeworthy, ask the user before writing.

## Example invocation

User: "let's freeze this — storage mode is immutable at node creation, no built-in drafts primitive, apps handle drafts via ACL flips"