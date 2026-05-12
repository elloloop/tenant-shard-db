# ShareNode owner-share semantics — Phase 4A.1 analysis

EPIC #407 / Wave 4A. Decide whether Go's "owner can share" is the
correct semantic to ship before Python is deleted. **Analysis only.**

## 1. Findings — what each implementation actually does

### Python `ShareNode` handler

`server/python/entdb_server/api/grpc_server.py:1746-1826` calls four
gates in order:

1. `_check_tenant` (sharding/region)
2. `_trusted_actor` (rebind to interceptor identity)
3. `_check_tenant_access(require_write=True)` — must be tenant
   owner/admin/member (NOT *node* owner — this is tenant-role).
4. `_check_capability(tenant, actor, node_id, type_id, "ShareNode")`
   at `grpc_server.py:1783-1790`.

`_check_capability` (`grpc_server.py:299-360`):

- Line 322: short-circuits on `actor.startswith("system:")` /
  `"admin:"` / `"__system__"` via `_is_admin_or_system`.
- Line 324-331: looks up required cap → `CoreCapability.ADMIN` for
  `"ShareNode"` (`auth/capability_registry.py:73`).
- Line 334-339: calls `canonical_store.get_acl_grants(...)` — which
  reads **only** `node_access` rows (`canonical_store.py:3099-3149`).
  Node ownership is NOT consulted.
- Line 340-360: iterates grants, looks for one whose typed caps
  satisfy ADMIN; otherwise raises `PermissionError`.

**Net: Python denies owner-share unless the owner happens to also
hold an explicit `node_access` row with ADMIN.** Owners don't get
self-grants automatically (see `_sync_create_node` at
`canonical_store.py:1297-1356` — owner is recorded in
`nodes.owner_actor` and `node_visibility`, but no `node_access`
row is written).

There IS an owner short-circuit, but it lives in
`canonical_store._sync_can_access` (`canonical_store.py:2920`) and in
`AclManager.check_permission` (`apply/acl.py:240-241`, pinned by
`tests/python/unit/test_acl.py:110-118`). Both are reached by READ
paths, not by the ShareNode handler. The handler-level
`_check_capability` and the data-layer `_sync_can_access` are two
parallel ACL paths and they DISAGREE on owner semantics. This is a
latent bug in Python.

### Go `ShareNode` handler

`server/go/internal/api/share_node.go:96-254` (active code):

1. `s.checkTenant` (gate).
2. `auth.Authoritative` (rebind).
3. Required-arg validation.
4. **ACL pre-check at lines 157-186**: system/admin short-circuit;
   otherwise fetch the node via `s.store.GetNode` and require
   `node.OwnerActor == trusted.String()`. Non-owner → soft-fail.
5. WAL append.

The handler comment at lines 30-33 is explicit: *"the full Python
flow (\_check\_capability with CoreCapability.ADMIN and the share-
grant fall-through) is deferred to a follow-up once the acl.Checker
is wired into Server — until then, this handler enforces the
strictly-narrower owner-only branch."*

**Net: Go allows owner-share, but is strictly narrower than what
Python+acl.Checker together would do** — it does NOT yet accept a
non-owner user with an explicit ADMIN grant as a valid sharer.

### Go `acl.Checker` (built, but NOT wired into ShareNode)

`server/go/internal/acl/checker.go:139-255`:

- Line 140-142: system actors short-circuit.
- Line 146-149: reads `NodeMeta` (owner + type_id).
- Line 167-173: **owner short-circuit** — if the resolved actor set
  contains `meta.OwnerActor`, allow.
- Line 175-184: looks up required cap; if none registered, allow.
- Line 186-235: iterate `node_access` grants, deny-pass first, then
  allow-pass.
- Line 237-249: cross-tenant fallback via `CrossTenantGrantReader`.

When wired into ShareNode, this Checker would accept owner-share AND
non-owner-with-ADMIN-grant — i.e. the "industry-standard" semantic.

### Test surface

- `tests/python/integration/test_grpc_contract.py:201-212` — happy
  path uses `actor=ADMIN` (`"system:admin"`), which short-circuits in
  Python's `_is_admin_or_system`. **The contract test does NOT
  exercise owner-share at all.** This is why both impls pass.
- `server/go/internal/api/share_node_test.go:107-161` —
  `TestShareNode_OwnerHappyPath`: alice (owner) shares with bob,
  expects `success=true`. This is the new Go pin.
- `tests/python/unit/test_acl.py:110-118, 170-183, 185-188` — pin the
  `AclManager.check_permission` owner-as-admin semantic (data layer,
  not handler).
- `tests/python/unit/test_capability_registry.py:66` — pins
  `ShareNode → CoreCapability.ADMIN`. Says nothing about ownership.
- No Python integration/unit test exists that calls `ShareNode` with
  a `user:` owner actor and asserts success. (Confirmed by grep over
  `tests/python/`.)
- `tests/python/parity/test_parity_scenarios.py` — does not exist on
  this branch. The divergence was observed in #481's parity sweep.

## 2. Why the contract test passes both

The contract case at `test_grpc_contract.py:201-212` builds the
request with `context.actor = "system:admin"`. In Python this hits
the `_is_admin_or_system` branch at `grpc_server.py:322` and
short-circuits before `get_acl_grants` runs. In Go the same actor
hits the `trusted.IsSystem() || trusted.IsAdmin()` short-circuit at
`share_node.go:157`. Neither implementation evaluates the
owner-vs-grant question for this case.

The only place owner-share is actually tested is the new Go unit
test `TestShareNode_OwnerHappyPath`, which has no Python equivalent.
Therefore: no contract regression today, but the moment a parity
test calls `ShareNode` with a `user:` owner actor, Python denies and
Go allows.

## 3. Scenario walkthrough

For each scenario: `P` = Python today, `Go` = Go handler today,
`Want` = recommended ship semantic, with reasoning.

| # | Scenario | P | Go | Want | Reasoning |
|---|----------|---|----|----|-----------|
| 1 | Alice (owner) shares her own node with Bob | DENY | ALLOW | **ALLOW** | Owner already controls the node by every other axis (delete, transfer, edit). Denying re-share is theatre; they can transfer ownership and re-share through that path. |
| 2 | Tenant admin (`system:`/`admin:` actor) shares any node | ALLOW | ALLOW | **ALLOW** | Required for break-glass + admin tooling. Both impls agree. |
| 3 | User with explicit ADMIN grant on a node shares it | ALLOW | DENY | **ALLOW** | The whole point of granting ADMIN is to delegate management, including re-share. Go's current narrowing breaks this — must fix before Python sunsets. |
| 4 | User with only READ tries to share | DENY | DENY | **DENY** | Both agree. Trivial. |
| 5 | Service account (`service:` actor) shares | DENY in P unless granted; DENY in Go (not in admin allowlist) | DENY | **DENY by default, ALLOW with ADMIN grant** | Service accounts must be authorised the same way as users — explicit grant. The Go `IsAdmin` allowlist accepts `admin:` not `service:`. Correct. |
| 6 | Former owner (ownership transferred) shares | DENY | DENY (owner check fails after transfer) | **DENY** | `TransferOwnership` rewrites `nodes.owner_actor`. Both impls correctly stop honouring the old owner. |
| 7 | Non-owner user with ADMIN grant (no ownership) shares | ALLOW | DENY | **ALLOW** | Same as #3. The ADMIN capability is meaningless if it doesn't permit re-share. |

Security analysis of "owner can share":
- **Confidentiality**: owner already sees every field; sharing READ
  doesn't expose anything they don't already control.
- **Integrity**: owner can edit/delete arbitrarily; share doesn't
  add an integrity surface.
- **Availability/blast radius**: a compromised owner account can
  exfiltrate to any recipient. But a compromised owner can ALSO
  email/screenshot the data, transfer ownership, or
  `RevokeAllUserAccess` everyone else. Restricting share doesn't
  meaningfully shrink the attack surface.
- **Audit**: every share goes through the WAL (CLAUDE.md invariant
  #1, restored in the Go port). The audit trail captures `granted_by
  = trusted_actor`, so owner-share is fully accountable.

## 4. Comparison to adjacent systems

- **Google Drive / Dropbox / OneDrive**: file owner has implicit
  share rights. Org admins can override.
- **GitHub**: repo owner (or maintainers in an org repo) can add
  collaborators. Owner == admin.
- **Slack / Notion**: workspace admin can share anything; resource
  creators (Notion page owner, Slack channel creator) can share
  their own.
- **Postgres**: object owner has all privileges including `GRANT
  OPTION` for that object by default (`pg_class.relowner`, see
  `GRANT` docs). Owner can re-grant; can revoke their own.
- **AWS IAM resource policies**: resource owner (account that
  created the resource) controls the resource policy unless an SCP
  restricts it.
- **Linux/POSIX**: file owner can `chmod` (the share equivalent).
  Only root can chmod files owned by others.

**Pattern: owner has admin including share. A separate
ADMIN-capability grant on top of ownership is unusual and not
present in any mainstream system.** Python's strict check is an
outlier.

## 5. Recommendation

**Ship Go with the `acl.Checker` wired in (allow owner OR
ADMIN-grantee), not the current narrow owner-only branch.**

Two work items, neither of them "keep Go as-is":

### 5.1 Mandatory before Python deletion

Wire `acl.Checker` into the Go `ShareNode` handler (and the matching
follow-up the handler comment already acknowledges at
`share_node.go:30-33`). The checker exists, has owner short-circuit
at `checker.go:167-173`, and correctly evaluates `node_access`
grants. Replace the inline owner-only check at
`share_node.go:157-186` with `s.acl.Check(ctx, acl.CheckRequest{
OpName: "ShareNode", ... })`.

Result: scenarios 1, 2, 3, 7 all behave correctly; scenarios 4, 5,
6 unchanged.

### 5.2 NOT recommended

Do NOT port Python's `_check_capability` semantic verbatim. It is
inconsistent with the data-layer `_sync_can_access` and with every
adjacent system. It is a latent bug that no production caller has
tripped because real callers go through the admin path (contract
test pattern: `system:admin` actor) or use ExecuteAtomic.

### 5.3 Tests to add in the Go port

- `TestShareNode_AdminGranteeReshare` — alice creates node, grants
  `core_caps=[ADMIN]` to bob (via a pre-seeded `node_access` row),
  bob shares with charlie. Expect `success=true`.
- `TestShareNode_ReadGranteeCannotReshare` — bob with READ shares
  with charlie. Expect `success=false`.
- `TestShareNode_OwnerAfterTransfer` — alice transfers to bob, then
  alice tries to share. Expect `success=false`.

### 5.4 Parity test (`tests/python/parity/`) should pin Go's behaviour as the canonical

When the parity test is rewritten as part of #481, the assertion
should be Go's behaviour for owner-share (allow) and Python's
behaviour should be marked as a known divergence that disappears
when Python is removed. Don't "fix" Python — it's terminal.

## 6. Confidence + risks

**Confidence: high (~90%) that owner-can-share is correct.** The
adjacent-system pattern is uniform; the Python check is provably
inconsistent with its own data-layer ACL (`AclManager` has owner =
ADMIN at `acl.py:240-241`); the WAL audit trail provides
accountability.

**What could change my recommendation:**

1. **A pinned regulatory/compliance test that REQUIRES explicit
   ADMIN-grant-on-top-of-ownership.** Searched
   `tests/python/integration/` and `tests/python/e2e/` for
   "owner.*share" / "share.*owner" — no such test exists. Searched
   `docs/decisions/` — `acl.md:69-71` documents owner short-circuit
   as a feature, not a bug. Risk: low.
2. **An ExecuteAtomic op type that performs ShareNode and goes
   through a stricter check.** Skimmed `grpc_server.py:620-790` —
   ExecuteAtomic doesn't have a share op; ShareNode is the only
   path. Risk: low.
3. **Production traffic data showing every real share is done by
   admin actor, never owner.** Don't have telemetry visibility from
   here, but the contract test pattern (`actor=ADMIN`) suggests
   admin-driven sharing is the dominant production path.
   Even if 100% of today's traffic is admin-driven, the SEMANTIC
   that's correct to ship for new clients is owner-can-share. Risk:
   the Wave 4A divergence may surface in a downstream SDK test
   that's been pinned against Python's strict semantic. Mitigation:
   audit pinned tests one more time when wiring `acl.Checker`.
4. **A different ADMIN-on-tenant model where node owners are NOT
   automatically tenant admins.** Already true: `_check_tenant_access`
   only requires owner/admin/member tenant role for write — node
   ownership is independent of tenant role. So scenario 1 already
   has the tenant-membership gate AND the (current Go) owner gate.
   Layering an ADMIN-grant gate on top adds nothing security-wise
   that the tenant gate doesn't already provide.

**What I have NOT verified:**

- The exact wiring path from `Server` struct to a future `s.acl`
  field — the comment at `share_node.go:30-33` says it's deferred to
  a follow-up but I haven't traced what's blocking it (likely just
  the `acl.Checker` construction wiring in `cmd/entdb-server`).
- Whether the cross-tenant ADMIN grant scenario (caller in tenant
  B, grant on `tenant:B` includes ADMIN) is exercised by any test.
  `checker.go:237-249` only translates a cross-tenant `Permission`
  through `LegacyToCoreCaps`, which maps `"admin"` to
  `[CORE_CAP_ADMIN]`. Should work but isn't pinned.
- Whether `ExtensionCapability` mappings on a user-declared type
  could redefine `ShareNode` to require something other than ADMIN.
  Doesn't matter for this analysis — owner short-circuit fires
  before `RequiredForOp` is consulted (`checker.go:167-184`).

## Summary

Go's *current* implementation is too narrow (rejects scenario 3/7)
but correct on owner-share. Python's check is too strict (rejects
scenario 1). The right ship semantic is what `acl.Checker.Check`
already does: owner OR explicit-ADMIN-grant OR system/admin actor.
Wire `acl.Checker` into the ShareNode handler before Python deletion.
