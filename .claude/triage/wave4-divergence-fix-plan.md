# Wave 4 — Go vs Python contract divergence: triage + fix plan

Source-of-truth run: `ENTDB_SERVER_TARGET=go python -m pytest tests/python/integration/test_grpc_contract.py -v` on `738b3be` (post Wave 3 flip). 39 FAILED, 31 PASSED, 1 SKIPPED (Python-only).

## Headline finding

**All 39 failures collapse to ONE root cause.** The Go subprocess harness never seeds the contract-fixture state (tenant `acme`, users `alice`/`bob`, memberships, registry, seed node). The Python in-process fixture seeds via direct `GlobalStore`/`CanonicalStore` writes + a follow-up `ExecuteAtomic` seed call (`tests/python/integration/conftest.py:275-348`). The Go path (`_start_go_server` at `conftest.py:350-416`) starts an empty `entdb-server` binary and yields the port — no equivalent seed step.

Every failing case is downstream of "empty global store" / "empty canonical store" / "registry has no types":

- "tenant \"acme\" does not exist" (NOT_FOUND from `tenant.CheckTenant` at `server/go/internal/tenant/check.go:140`): 25 cases.
- "User not found" / "Tenant not found" in-band error string (no users/tenants in globalstore): 5 cases.
- `r.found is True` / `any(u.user_id == "alice"...)` check failures (RPC succeeded, returned empty/found=false): 6 cases.
- `AddTenantMember`/`ChangeMemberRole`/`RemoveTenantMember` returning PERMISSION_DENIED (alice not a member, so not owner): 3 cases — same root cause.
- `SearchNodes-invalid_argument`: tenant gate fires before the empty-query check, so we get NOT_FOUND instead of INVALID_ARGUMENT — masked by missing tenant.
- `WaitForOffset-happy`: DEADLINE_EXCEEDED because no offset 0 has been observed on tenant `acme` (no tenant SQLite initialized, no WAL events).
- `GetTenantQuota-permission_denied`: expected PERMISSION_DENIED (bob is non-admin), got NOT_FOUND (tenant missing) — same root cause.

The harness contract at `docs/go-port/shared/test-harness.md:96-117` explicitly says the Go binary MUST honour `--seed-tenant acme` (creating tenant + alice as owner + bob as member, registering the User/Task/AssignedTo schema). The Go binary at `server/go/cmd/entdb-server/main.go:29-34` declares only `--addr`, `--data-dir`, `--wal-backend`, `--wal-topic`, `--wal-group` — `--seed-tenant` (and the other harness-required flags: `--auth-disabled`, `--mailbox-fanout`, `--batch-size`, `--ready-probe`) are not wired. The harness in `conftest.py:367-377` similarly only passes `--addr`, `--data-dir`, `--wal-backend memory`.

## Root-cause buckets

### Bucket 1 — Missing test-only seed path on the Go server (all 39 failures)

**Affected RPCs / cases (all 39):**

| RPC | Mode | Failure surface |
|---|---|---|
| GetNode | happy, not_found | abort NOT_FOUND `tenant "acme" does not exist` |
| GetNodes | happy | abort NOT_FOUND |
| QueryNodes | happy | abort NOT_FOUND |
| GetEdgesFrom | happy | abort NOT_FOUND |
| GetEdgesTo | happy | abort NOT_FOUND |
| GetConnectedNodes | happy | abort NOT_FOUND |
| SearchMailbox | happy | abort NOT_FOUND |
| GetMailbox | happy | abort NOT_FOUND |
| ListMailboxUsers | happy | abort NOT_FOUND |
| WaitForOffset | happy | abort DEADLINE_EXCEEDED (no events on tenant) |
| GetReceiptStatus | happy0, happy1 | abort NOT_FOUND |
| ShareNode | happy | abort NOT_FOUND |
| RevokeAccess | happy | abort NOT_FOUND |
| AddGroupMember | happy | abort NOT_FOUND |
| RemoveGroupMember | happy | abort NOT_FOUND |
| TransferOwnership | happy | abort NOT_FOUND |
| ListSharedWithMe | happy | abort NOT_FOUND |
| GetUser | happy | response `found=false` (alice not in globalstore) |
| ListUsers | happy | response `users=[]` (no users) |
| UpdateUser | happy | response `success=false, error="User not found"` |
| GetTenant | happy | response `found=false` (acme not in globalstore) |
| GetTenantMembers | happy | response `members=[]` |
| GetUserTenants | happy | response `memberships=[]` |
| AddTenantMember | happy | abort PERMISSION_DENIED (alice not owner/admin) |
| ChangeMemberRole | happy | abort PERMISSION_DENIED |
| RemoveTenantMember | happy | abort PERMISSION_DENIED |
| TransferUserContent | happy | abort NOT_FOUND |
| SetLegalHold | happy | abort NOT_FOUND |
| RevokeAllUserAccess | happy | abort NOT_FOUND |
| GetTenantQuota | happy | abort NOT_FOUND |
| GetTenantQuota | permission_denied | abort NOT_FOUND (expected PERMISSION_DENIED) |
| GetNodeByKey | not_found | abort NOT_FOUND (expected `found=false`) |
| SearchNodes | invalid_argument | abort NOT_FOUND (expected INVALID_ARGUMENT) |
| SearchNodes | happy | abort NOT_FOUND |
| FreezeUser | happy0 | response `success=false, error="User not found"` |
| DeleteUser | happy | response `success=false, error="User not found"` |
| ArchiveTenant | happy | response `success=false, error="Tenant not found"` |

**Why "one bucket":** in 36 of 39 cases the Go handler is correct — it's enforcing the gate it should enforce against an empty global/canonical store. The remaining 3 (`SearchNodes-invalid_argument`, `WaitForOffset-happy`, `GetTenantQuota-permission_denied`) look like "wrong status code" at first glance but inspection shows the same root cause: with seeding present, the validation/membership check that the test expects would run first and produce the expected code.

**Spec / handler references:**

- Python seed in `tests/python/integration/conftest.py:293-300` (creates tenant, users alice + bob, memberships) and `:240-272` (ExecuteAtomic seed for `seeded-node`).
- Python registry build at `tests/python/integration/conftest.py:217-237` (User/Task/AssignedTo).
- Go `tenant.CheckTenant` at `server/go/internal/tenant/check.go:140-142` (returns NOT_FOUND when global store has no row).
- Go `server/go/cmd/entdb-server/main.go:29-34` (flags wired — no `--seed-tenant`).
- Harness contract at `docs/go-port/shared/test-harness.md:96-117`.
- Harness conftest at `tests/python/integration/conftest.py:367-377` (only passes addr/data-dir/wal-backend).

**Fix — one PR:**

1. Add a `--seed-tenant <id>` flag (and a fixed-shape companion: tenant `<id>`, user `alice` as owner, user `bob` as member, registry types User(1)/Task(2)/AssignedTo(100), seed node `seeded-node` of type 1 with payload `{1: "seeded@example.com", 2: "Seeded"}`) to `server/go/cmd/entdb-server/main.go`. This must run BEFORE the gRPC listener accepts traffic, and BEFORE the applier starts (so the seed node either lands via direct canonical write under the same idempotency key the Python seed uses, or via an internal "seed WAL event" replayed at startup — the former is simpler and Wave-1-correct because the canonical store is the read-side of the WAL).
2. Make sure schema fingerprint matches Python by registering the same three types via `schema.Registry` — `server/go/internal/schema` already has the registry; thread it through `api.WithSchemaRegistry(...)` and into `--seed-tenant` setup.
3. Update `tests/python/integration/conftest.py:367-377` (`_start_go_server`) to pass `--seed-tenant acme`. While here, align with the harness spec by also passing `--auth-disabled`, `--mailbox-fanout=false`, `--batch-size=1` if those flags are needed for parity (today the Go binary defaults to single-applier in-memory which already matches Python, so these may be no-ops; check before adding).
4. The seed must also write the `idempotency_key="seed-1"` receipt so `GetReceiptStatus-happy0` (expects APPLIED for `seed-1`) passes. The Python seed achieves this implicitly because `ExecuteAtomic` records the receipt; in Go, replicate by either (a) running an actual `ExecuteAtomic`-equivalent path internally during seed, or (b) inserting the receipt row + node row + offset row directly into the per-tenant SQLite. Option (a) reuses real code paths and is preferred.

**Estimated effort: 1 PR.** Scope: ~150 LOC of seed code in `server/go/cmd/entdb-server/main.go` (or a new `server/go/internal/testseed` package), 1 line in conftest, and a passing run of the contract suite.

**Why this fixes the three "wrong status code" cases:**

- `SearchNodes-invalid_argument` (`server/go/internal/api/search_nodes.go:81-93`): `checkTenant` runs at line 81, empty-query check at line 90. With tenant `acme` seeded, line 81 passes and line 90 produces the expected INVALID_ARGUMENT.
- `WaitForOffset-happy` (`server/go/internal/api/wait_for_offset.go:34-77`): the wait target is offset 0 on tenant `acme`. With the seed running `ExecuteAtomic` to create `seeded-node`, the applier records offset 1 on `acme`, so `WaitForOffset(target=0)` returns immediately.
- `GetTenantQuota-permission_denied` (`server/go/internal/api/get_tenant_quota.go` — verify): tenant gate fires first; with seed present, the gate passes and the bob-is-not-admin check returns PERMISSION_DENIED.

### Bucket 2 — None

There is no second bucket. After Bucket 1 is fixed, re-run the contract suite. Any residual failures are new and warrant their own triage — but based on a static read of every failing handler against the Python source-of-truth, no handler-side divergence is independently visible at this commit.

## Falsifying the "one bucket" hypothesis — checks to run

Before landing the PR, do these spot-checks. If any fails, peel into a new bucket:

1. **GetNode-not_found** — once tenant is seeded, GetNode for `does-not-exist` must return `found=false` with codes.OK, not NOT_FOUND. Confirms Go's GetNode handler distinguishes missing-tenant (gate) from missing-node (data).
2. **GetReceiptStatus-happy0** — `idempotency_key="seed-1"` must resolve to `RECEIPT_STATUS_APPLIED`. Verifies the seed wrote a receipt row using the same key Python uses.
3. **WaitForOffset-happy** — Python contract is "no abort; framework verifies well-formed response" (test check is `lambda _r: True`). After seed, `WaitForOffset(stream_position="entdb-wal:0:0", timeout=500ms)` must NOT abort. If Go still aborts because the applier hasn't caught up, the seed must include a synchronous `WaitForOffset(target=1)` step internally before yielding.
4. **GetTenantQuota-permission_denied** — bob must exist as a non-admin member of acme so the quota handler can return PERMISSION_DENIED instead of NOT_FOUND. The seed adds bob with role="member", which is sufficient.
5. **SearchNodes-happy** — type 1 has no searchable fields, so result must be `nodes=[]` with codes.OK. Verifies the registry is wired the same way (no FTS on type 1).
6. **AddGroupMember / RemoveGroupMember happy** — these use `group_id="g-test"` (not a tenant member); they need tenant existence + actor-as-ALICE to be a member. Seed satisfies both.

## Dependency-ordered fix PR list

1. **PR 1 — Wire `--seed-tenant` into the Go entdb-server binary and into the contract harness.**
   - Files touched:
     - `server/go/cmd/entdb-server/main.go` — add flag, wire seed path before `srv.Serve`.
     - New `server/go/internal/testseed/seed.go` (or inline in main) — applies a deterministic seed: tenant `<id>` + region `""` + status `"active"`; users alice (owner) + bob (member); schema types User(1), Task(2), AssignedTo(100); seed-node id `seeded-node` of type 1 with payload `{1: "seeded@example.com", 2: "Seeded"}` and receipt key `seed-1`.
     - `tests/python/integration/conftest.py:367-377` — append `"--seed-tenant", "acme"` to cmd.
     - Add a Go unit test (`server/go/cmd/entdb-server/main_seed_test.go` or `server/go/internal/testseed/seed_test.go`) asserting seed idempotency and resulting global/canonical-store state.
   - Acceptance:
     - `cd server/go && go vet ./... && go test ./...` passes.
     - `ENTDB_SERVER_TARGET=go python -m pytest tests/python/integration/test_grpc_contract.py -q` is 70 passed / 1 skipped / 0 failed.

   No follow-up PRs are anticipated based on static analysis. If post-PR-1 the suite still has residual failures, triage them as new buckets and land each as its own PR.

**Total PRs: 1.**

## Appendix — failure transcripts (one per distinct surface)

### A — Bare NOT_FOUND from tenant gate (25 cases, identical wire shape)

Representative output (from `GetNode-happy`):

```
status = StatusCode.NOT_FOUND
details = "tenant \"acme\" does not exist"
```

Asserted by test as `mode="happy"` => no abort allowed. Originates from `server/go/internal/tenant/check.go:141`.

### B — In-band `User not found` / `Tenant not found` (4 cases)

Representative (from `UpdateUser-happy`):

```
AssertionError: UpdateUser (happy) response failed check: error: "User not found"
```

Handler returns OK with `success=false, error="..."`. Test check is `r.success is True`. Cases:

- `UpdateUser-happy` — handler at `server/go/internal/api/update_user.go` (verify), expects user `alice` to exist.
- `FreezeUser-happy0` — `server/go/internal/api/freeze_user.go` (verify), expects user `bob`.
- `DeleteUser-happy` — `server/go/internal/api/delete_user.go:126`, expects user `alice`.
- `ArchiveTenant-happy` — `server/go/internal/api/archive_tenant.go:110`, expects tenant `acme`.

### C — Empty response failing existence check (6 cases)

Representative (from `GetUser-happy`):

```
AssertionError: GetUser (happy) response failed check:
```

The test check looks for a specific user/tenant/membership in the response; the response is empty.

- `GetUser-happy` — check `r.found is True and r.user.user_id == "alice"`.
- `ListUsers-happy` — check `any(u.user_id == "alice" for u in r.users)`.
- `GetTenant-happy` — check `r.found is True and r.tenant.tenant_id == TENANT`.
- `GetTenantMembers-happy` — check `any(m.user_id == "alice" for m in r.members)`.
- `GetUserTenants-happy` — check `any(m.tenant_id == TENANT for m in r.memberships)`.

### D — PERMISSION_DENIED from missing membership (3 cases)

Representative (from `AddTenantMember-happy`):

```
status = StatusCode.PERMISSION_DENIED
details = "Only owner or admin can add members"
```

Handlers correctly enforce role check; alice has no role on acme because the membership row was never written.

- `AddTenantMember-happy` — `server/go/internal/api/add_tenant_member.go:86-142`.
- `ChangeMemberRole-happy` — `server/go/internal/api/change_member_role.go:64-131`.
- `RemoveTenantMember-happy` — `server/go/internal/api/remove_tenant_member.go` (verify).

### E — Wrong status code (3 cases, ordering masked by missing tenant)

- `SearchNodes-invalid_argument` — expected INVALID_ARGUMENT, got NOT_FOUND. `search_nodes.go:81` (`checkTenant`) runs before `:90` (empty-query check).
- `GetTenantQuota-permission_denied` — expected PERMISSION_DENIED, got NOT_FOUND. Tenant gate fires before role check.
- (`WaitForOffset-happy` is grouped here for completeness — expected no-abort, got DEADLINE_EXCEEDED. Not strictly a "wrong code"; it's "waited for offset that never arrives because no events".)

All three resolve once the seed is in place and the gate / membership / applier checks evaluate against populated state.
