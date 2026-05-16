# ADR-019: All design decisions live in `docs/adr/`; CLAUDE.md is execution-only

**Status:** Accepted
**Decided:** 2026-05-14
**Tags:** process, documentation
**Implementation:** _this commit_

## Decision

All EntDB design decisions live in **`docs/adr/`** as numbered ADRs.
There is exactly one home; the previous `docs/decisions/` folder has
been deleted and its entries migrated into numbered ADRs (see the
per-topic mapping in "What this locks in" below).

`CLAUDE.md` contains **agent execution rules only**: the local CI
workflow, release process, directory map, testing commands, code-style
hints, and pointers to where things live. CLAUDE.md may **reference**
ADRs by number and one-line summary, but never restates the body of a
decision. When CLAUDE.md and an ADR disagree, the ADR wins — CLAUDE.md
must be corrected.

The 6 "Architecture Invariants" originally embedded in CLAUDE.md §
"Architecture Invariants" (#1 WAL is source of truth, #2 WAL is the
audit log, #3 single applier goroutine, #4 per-tenant SQLite
isolation, #5 proto is the type system, #6 field-ids on disk) have
all been migrated. Final mapping:

- #1 → [ADR-016](016-handlers-append-applier-writes.md)
- #2 → [ADR-015](015-wal-and-s3-object-lock-as-audit-log.md)
- #3 → dropped (analysis showed it restated standard Kafka
  consumer-group semantics, not a project-specific design)
- #4 → already covered by [ADR-001](001-storage-architecture.md);
  CLAUDE.md duplication removed
- #5 → folded into [ADR-006](006-proto-schema-definition.md),
  which already covered proto-as-schema; ADR-006 widened to
  "proto is the type system end-to-end"
- #6 → [ADR-018](018-field-id-keyed-payloads.md)

The "Architecture Invariants" header is gone from CLAUDE.md.
CLAUDE.md now lists ADR pointers under "Architecture decisions" as
an orientation aid; the ADRs themselves are normative.

### ADR template

Every new ADR uses this header:

```markdown
# ADR-NNN: Short title

**Status:** Accepted | Proposed
**Decided:** YYYY-MM-DD
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
Proposed -> Accepted -> (deleted)
```

- **Proposed**: discussion draft, not load-bearing.
- **Accepted**: in force; code MUST honor it.

There are only two states. When a decision is superseded or no longer
in force, the **content is deleted from the repo** — not retained as a
"superseded" file with a forward link. Contradictions are corrosive:
LLMs (and humans) loading a doc set with both the old and new
decisions cannot reliably tell which is current, and the supersede
marker is too easy to miss.

Deletion granularity:

- **Partial supersede:** edit the old ADR to remove the parts that
  no longer apply. The rest of the file stays under its original
  number and `Status: Accepted`.
- **Full supersede:** delete the file entirely. The number is retired
  (do not reuse it; the next ADR uses the next unused number).

The new ADR's `Alternatives considered` section is where rejected /
former approaches live in their proper context (as deliberate
rejections with reasoning), not as scattered "Superseded" banners.

The **commit log is the long-term design history.** `git log --follow
docs/adr/` and `git log -S 'audit_log'` reach the same forensic
content a "Superseded" file would have preserved, without polluting
the working tree.

### Frontmatter changes

Drop the `Supersedes:` / `Superseded by:` fields from the template
above — they're meaningless under the deletion policy. Keep
`Status`, `Decided`, `Tags`, `Implementation`.

When an ADR replaces or removes content from earlier ADRs, the new
ADR's `References` section lists the file(s) it removed content
from, with a one-liner ("removed `audit_log` row from ADR-001 tables
list; removed §Audit logging from ADR-011"). The commit doing the
removal is the authoritative record.

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
  first. The folder name from `adr/` is the better convention.

- **Leave invariants in CLAUDE.md, just stop adding new ones there.**
  Rejected: drift is already present (CLAUDE.md #2 contradicts ADR-001
  and ADR-011). Without a forcing function to move the existing ones,
  the contradictions persist.

- **Embed decisions in code comments next to the relevant package.**
  Rejected: decisions outlive individual code paths. Decisions get
  replaced; code gets rewritten. Coupling them puts the history in
  `git log` instead of in a navigable index.

## Consequences

**What this locks in:**

- The canonical decision corpus is `docs/adr/NNN-*.md`. New ADRs use
  the next available number.
- CLAUDE.md is execution-only. The "Architecture Invariants" section
  has been removed; design decisions live exclusively in `docs/adr/`.
  CLAUDE.md may carry ADR pointers (number + one-line summary) under
  an "Architecture decisions" orientation block, but normative
  content lives in the ADRs.
- When a new ADR removes content from earlier ADRs, the new ADR's
  `References` section names the file(s) it touched and the commit
  doing the removal is the authoritative record (no `Superseded by:`
  banner; the old content is just gone).
- The `docs/decisions/` folder is gone. **Migration complete**:
  every entry that lived there has been moved into `docs/adr/` as a
  numbered ADR (see the per-topic mapping below), and the folder
  itself was deleted along with `INDEX.md`. New design decisions go
  directly into `docs/adr/` under the next free number.

Final mapping from the retired `docs/decisions/` corpus to ADRs:

- `acl.md` → [ADR-003](003-acl-model.md)
- `storage.md` → [ADR-020](020-immutable-storage-mode.md) (with
  ADR-014 owning the physical file layout)
- `quotas.md` → [ADR-024](024-three-layer-rate-limit-model.md)
- `unique_keys.md` → merged into
  [ADR-025](025-single-shape-sdk-api.md) (the original
  `node_keys` design is recorded as a rejected alternative
  there)
- `sdk_api.md` → [ADR-025](025-single-shape-sdk-api.md)
- `query_indexes.md` →
  [ADR-023](023-declarative-query-indexes.md)
- `fts.md` → [ADR-022](022-fts5-full-text-search.md)
- `console.md` → [ADR-021](021-go-console-binary.md)
- `python-server-retired.md` →
  [ADR-017](017-python-server-retired.md)

**What this makes easy:**

- Agents and humans have one place to look for design rationale.
- Every file in `docs/adr/` is currently in force. No contradictions
  to reconcile. LLM context loads cleanly.
- The `Status` field tells a reader at a glance whether an ADR is
  in force or still a proposal.
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
- A new ADR lands without deleting the superseded content from
  earlier ADRs. Detected by: cross-ADR contradiction audit (grep
  for the rejected design term across `docs/adr/`). The deletion
  must land in the same commit as the new ADR.
- A future contributor re-creates `docs/decisions/` (or a new
  parallel folder) to avoid the ADR ceremony. Detected by: presence
  of design decisions outside `docs/adr/`. The migration that
  retired the folder is the precedent — any new decision-doc
  location needs to delete or merge ADR-019 first.

## References

- Audit findings on `chore/consistency-audit` 2026-05-14 — 3
  locked-vs-locked contradictions (`storage.md` vs `sdk_api.md`,
  CLAUDE.md invariant #2 vs ADR-001/ADR-011, ADR-001 vs `storage.md`
  on per-user mailbox files).
- Existing ADRs: `docs/adr/001-storage-architecture.md` through
  `docs/adr/013-accessibility-compliance.md`.
- Retired decisions folder: `docs/decisions/` (deleted; entries
  migrated into numbered ADRs per the mapping above).
- CLAUDE.md "Architecture Invariants" section (pre-migration).
