# ADR-014: Decision records home & CLAUDE.md scope

**Status:** Accepted
**Decided:** 2026-05-14
**Supersedes:** none
**Superseded by:** none
**Tags:** process, documentation
**Implementation:** _this commit_

## Decision

All EntDB design decisions live in **`docs/adr/`** as numbered ADRs.
There is exactly one home; the parallel `docs/decisions/` folder is
deprecated and its entries migrate into `docs/adr/` opportunistically
(when a discussion touches them).

`CLAUDE.md` contains **agent execution rules only**: the local CI
workflow, release process, directory map, testing commands, code-style
hints, and pointers to where things live. CLAUDE.md may **reference**
ADRs by number and one-line summary, but never restates the body of a
decision. When CLAUDE.md and an ADR disagree, the ADR wins — CLAUDE.md
must be corrected.

The 6 "Architecture Invariants" currently embedded in CLAUDE.md §
"Architecture Invariants" (#1 WAL is source of truth, #2 WAL is the
audit log, #3 single applier goroutine, #4 per-tenant SQLite
isolation, #5 proto is the type system, #6 field-ids on disk) get
lifted to ADR-015 through ADR-020 in follow-up commits, each
evaluated for current relevance during the move. Until that migration
is complete, the CLAUDE.md section is read-only — edits must land as
the corresponding new ADR, not as a CLAUDE.md change.

### ADR template

Every new ADR uses this header:

```markdown
# ADR-NNN: Short title

**Status:** Accepted | Superseded | Proposed | Deprecated
**Decided:** YYYY-MM-DD
**Supersedes:** ADR-MMM (if any), or `none`
**Superseded by:** ADR-PPP (if any), or `none`
**Tags:** comma, separated, kebab-case
**Implementation:** PR #NNN / commit SHA / "_this commit_"

## Decision

(1-3 sentences. The rule itself, no rationale.)

## Context

(Why we're locking this now. What problem it solves. Cite real prior
art / incidents if any.)

## Alternatives considered

(- Option N: rejected because ...)

## Consequences

**What this locks in:** (mechanism, code path, on-disk shape)
**What this makes easy:** (capabilities unlocked)
**What this makes harder:** (deliberate tradeoffs)
**Failure modes:** (how the contract can be observably violated)

## References

(Conversation logs, related ADRs, implementation links, prior issues.)
```

### Status lifecycle

```
Proposed -> Accepted -> Superseded
                    \-> Deprecated
```

- **Proposed**: discussion draft, not load-bearing.
- **Accepted**: in force; code MUST honor it.
- **Superseded**: replaced by a later ADR; keep the file for history,
  link forward from `Superseded by:`.
- **Deprecated**: no longer in force AND nothing replaces it (the
  problem went away). Keep the file so audit trails resolve.

A superseded ADR is **not deleted**. The supersede chain is the
project's design history.

## Context

The repo accumulated three locations for design decisions: the older
numbered `docs/adr/` (1-13), the newer dated `docs/decisions/`
(10 entries), and the "Architecture Invariants" section of CLAUDE.md
(6 invariants embedded as 5-15 line decision bodies). The audit on
`chore/consistency-audit` surfaced three locked-vs-locked
contradictions — frozen decisions that contradict each other because
each was written without knowing the other existed.

Embedding decisions in CLAUDE.md compounds the problem: CLAUDE.md is
edited frequently as agent workflows evolve, and edits to the
invariants section will drift out of sync with both `decisions/` and
the code. CLAUDE.md ships in every Claude session prompt, so the cost
of an undetected drift is high — agents will treat the embedded
restatement as canonical and ignore the actual ADR.

This ADR consolidates the three homes into one, sets a single
templated format, and confines CLAUDE.md to execution-only content
that doesn't need design-decision provenance.

## Alternatives considered

- **Keep both `docs/adr/` and `docs/decisions/`, define a split**
  (e.g. ADRs for cross-cutting architecture, decisions/ for narrower
  module-level calls). Rejected: the split has to be drawn somewhere
  and authors will misclassify; the dual home is what caused
  contradictions 1-3 in the first place.

- **Move everything to `docs/decisions/`, deprecate `docs/adr/`.**
  Rejected: the "ADR" name is widely recognized industry vocabulary
  (Architectural Decision Record). New contributors look for `adr/`
  first. The dated supersede chain from `decisions/` is the better
  template; the folder name from `adr/` is the better convention.

- **Leave invariants in CLAUDE.md, just stop adding new ones there.**
  Rejected: drift is already present (CLAUDE.md #2 contradicts ADR-001
  and ADR-011). Without a forcing function to move the existing ones,
  the contradictions persist.

- **Embed decisions in code comments next to the relevant package.**
  Rejected: decisions outlive individual code paths. Decisions get
  superseded; code gets rewritten. Coupling them puts the
  supersede chain in `git log` instead of in a navigable index.

## Consequences

**What this locks in:**

- The canonical decision corpus is `docs/adr/NNN-*.md`. New ADRs use
  the next available number.
- CLAUDE.md is execution-only. The "Architecture Invariants" section
  stays read-only until it's migrated to ADR-015-020; agents and
  humans MUST land changes to those decisions as ADR commits, not as
  CLAUDE.md edits.
- A superseding ADR updates the predecessor's `Superseded by:` line
  in the same commit. The chain is bidirectional.
- The 10 entries in `docs/decisions/` are valid until migrated. The
  migration happens lazily — when a discussion or fix touches a
  given decision, it moves into `docs/adr/` with a new number, the
  supersede chain is updated, and the old `decisions/` file is
  removed.

**What this makes easy:**

- Agents and humans have one place to look for design rationale.
- Supersede chains are navigable from the file's frontmatter, both
  forward and backward.
- The `Status` field tells a reader at a glance whether an ADR is in
  force, history, or proposal.
- Discovering contradictions becomes a mechanical check
  (`grep -l 'Status:.*Accepted' docs/adr/` + cross-link audit).

**What this makes harder:**

- Two-step migration: ADRs first, code/comments second. CLAUDE.md
  pointers must stay in sync as ADRs land.
- Writing a new ADR has more ceremony than dropping a paragraph in
  CLAUDE.md. Intentional — the ceremony forces the decision to be
  framed with alternatives and consequences rather than a one-liner.

**Failure modes:**

- A CLAUDE.md edit silently changes the meaning of a referenced ADR.
  Detected by: a contradiction audit (this one was the prompt). Future
  audits should grep CLAUDE.md for embedded rule language ("MUST",
  "MUST NOT", "do not", "always", "never") and flag anything that
  isn't a workflow rule.
- A new ADR lands without updating its predecessor's `Superseded by:`
  line. Detected by: backward-link audit. Could be enforced by a
  pre-commit hook later (out of scope here).
- The migration from `docs/decisions/` to `docs/adr/` stalls
  partway, leaving the corpus split again. Mitigation: complete
  ADR-015 through ADR-020 (the 6 invariants) within the same
  multi-PR effort that opened this ADR. Existing `decisions/`
  entries can migrate as discussions touch them, but the CLAUDE.md
  invariants must clear out fast.

## References

- Audit findings on `chore/consistency-audit` 2026-05-14 — 3
  locked-vs-locked contradictions (`storage.md` vs `sdk_api.md`,
  CLAUDE.md invariant #2 vs ADR-001/ADR-011, ADR-001 vs `storage.md`
  on per-user mailbox files).
- Existing ADRs: `docs/adr/001-storage-architecture.md` through
  `docs/adr/013-accessibility-compliance.md`.
- Existing decisions: `docs/decisions/INDEX.md`.
- CLAUDE.md "Architecture Invariants" section (pre-migration).
