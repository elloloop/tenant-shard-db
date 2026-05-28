window.BENCHMARK_DATA = {
  "lastUpdate": 1779983069528,
  "repoUrl": "https://github.com/elloloop/tenant-shard-db",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "75d6e712283d730f6670b82936609fbb0859f11a",
          "message": "Fix schemaless CAS preconditions\n\nFixes #525",
          "timestamp": "2026-05-17T00:43:26+01:00",
          "tree_id": "b4192c1e861c0b8afda9c97d86893bca565d6ea8",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/75d6e712283d730f6670b82936609fbb0859f11a"
        },
        "date": 1778977315383,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3387.999700029148,
            "unit": "iter/sec",
            "range": "stddev: 0.000025369112531249446",
            "extra": "mean: 295.1594122016589 usec\nrounds: 1344"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2219.322244088495,
            "unit": "iter/sec",
            "range": "stddev: 0.00003514310524790546",
            "extra": "mean: 450.5880129231585 usec\nrounds: 1393"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1105.3564622296715,
            "unit": "iter/sec",
            "range": "stddev: 0.00008511092165527612",
            "extra": "mean: 904.6855328306025 usec\nrounds: 929"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 814.5844027181297,
            "unit": "iter/sec",
            "range": "stddev: 0.0000811953608881937",
            "extra": "mean: 1.2276198717568982 msec\nrounds: 655"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1968.3799540046336,
            "unit": "iter/sec",
            "range": "stddev: 0.00007619280849049144",
            "extra": "mean: 508.0319975650626 usec\nrounds: 1642"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1971.2150021878338,
            "unit": "iter/sec",
            "range": "stddev: 0.00007743166419283495",
            "extra": "mean: 507.3013338931111 usec\nrounds: 1785"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1863.5156200669217,
            "unit": "iter/sec",
            "range": "stddev: 0.00014300862060160163",
            "extra": "mean: 536.6201330601612 usec\nrounds: 1706"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1988.9872542271678,
            "unit": "iter/sec",
            "range": "stddev: 0.000060453492631752935",
            "extra": "mean: 502.7684304535957 usec\nrounds: 1366"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1664.695481528081,
            "unit": "iter/sec",
            "range": "stddev: 0.00008513181571861864",
            "extra": "mean: 600.7104669269996 usec\nrounds: 257"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1434.6063642525223,
            "unit": "iter/sec",
            "range": "stddev: 0.0000600204354602292",
            "extra": "mean: 697.0553211793629 usec\nrounds: 1152"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2666.644332669093,
            "unit": "iter/sec",
            "range": "stddev: 0.00002970117680495202",
            "extra": "mean: 375.00314074471333 usec\nrounds: 1961"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 152.3238984292626,
            "unit": "iter/sec",
            "range": "stddev: 0.00014173186198351557",
            "extra": "mean: 6.564958028988394 msec\nrounds: 138"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "75d6e712283d730f6670b82936609fbb0859f11a",
          "message": "Fix schemaless CAS preconditions\n\nFixes #525",
          "timestamp": "2026-05-17T00:43:26+01:00",
          "tree_id": "b4192c1e861c0b8afda9c97d86893bca565d6ea8",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/75d6e712283d730f6670b82936609fbb0859f11a"
        },
        "date": 1778977714470,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3078.4426690742744,
            "unit": "iter/sec",
            "range": "stddev: 0.000024737067647124946",
            "extra": "mean: 324.8395723090443 usec\nrounds: 975"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2093.4702006120588,
            "unit": "iter/sec",
            "range": "stddev: 0.00003433001704599973",
            "extra": "mean: 477.6757747531512 usec\nrounds: 1212"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 979.9312077091403,
            "unit": "iter/sec",
            "range": "stddev: 0.0000731115014350763",
            "extra": "mean: 1.0204797970847117 msec\nrounds: 823"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 810.9180201562488,
            "unit": "iter/sec",
            "range": "stddev: 0.00008027424034191275",
            "extra": "mean: 1.2331702775667985 msec\nrounds: 526"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1923.100405342062,
            "unit": "iter/sec",
            "range": "stddev: 0.00007865813693992959",
            "extra": "mean: 519.9936504730391 usec\nrounds: 1688"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1899.0394581411106,
            "unit": "iter/sec",
            "range": "stddev: 0.00007564218368582476",
            "extra": "mean: 526.5820021343094 usec\nrounds: 1874"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2021.7570341706582,
            "unit": "iter/sec",
            "range": "stddev: 0.00006734917841702338",
            "extra": "mean: 494.61927575793413 usec\nrounds: 660"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1948.620513720402,
            "unit": "iter/sec",
            "range": "stddev: 0.00012308194886847818",
            "extra": "mean: 513.1835536775454 usec\nrounds: 1183"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1843.9919552689887,
            "unit": "iter/sec",
            "range": "stddev: 0.00004216033460464479",
            "extra": "mean: 542.3017151146556 usec\nrounds: 344"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1573.150979940929,
            "unit": "iter/sec",
            "range": "stddev: 0.000053052417032279",
            "extra": "mean: 635.6668957721716 usec\nrounds: 1372"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2565.311641875909,
            "unit": "iter/sec",
            "range": "stddev: 0.00002872770090995046",
            "extra": "mean: 389.816185946414 usec\nrounds: 2092"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 146.89227085532684,
            "unit": "iter/sec",
            "range": "stddev: 0.00029517845186113445",
            "extra": "mean: 6.807710127818045 msec\nrounds: 133"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "f7011b8919be0b6233f872f6acdf9376fa16b633",
          "message": "chore: cross-doc consistency audit + cleanup; CLAUDE.md→ADR migration; user-facing docs rewrite (#526)\n\n* docs: ADR-014 — decision records home & CLAUDE.md scope\n\nConsolidates the three current homes for design decisions\n(docs/adr/, docs/decisions/, and the embedded \"Architecture\nInvariants\" section of CLAUDE.md) into one: docs/adr/.\n\n- New ADR template merges the numbered-prefix convention from\n  docs/adr/ with the supersede-chain frontmatter from\n  docs/decisions/.\n- CLAUDE.md is execution-only: workflow, release process,\n  directory map, code-style hints. ADR wins in any conflict.\n- The 6 invariants in CLAUDE.md (#1 WAL source of truth, #2\n  audit log, #3 single applier, #4 per-tenant SQLite, #5 proto\n  types, #6 field-ids on disk) will lift to ADR-015 through\n  ADR-020 in follow-up commits, each evaluated for relevance\n  during the move. Until then the section is read-only.\n- Adds image-tag normalization note (Docker tags strip the\n  leading \"v\" from git tags) to the Releases section.\n\n* docs: amend ADR-014 — superseded content is deleted, not retained\n\nLLMs (and humans) struggle with contradictory content even when the\nold version carries a \"Superseded by\" banner — the marker is too\neasy to miss and the contradiction itself confuses context loading.\n\nThe original ADR-014 followed industry-standard ADR practice (keep\nall ADRs forever, link supersede chains). This amendment overrides\nthat for this repo:\n\n- Status lifecycle collapses to Proposed -> Accepted -> (deleted).\n- Partial supersede: edit the old ADR to remove the no-longer-current\n  parts; the rest stays under its original number.\n- Full supersede: delete the file. Number is retired.\n- The new ADR's \"Alternatives considered\" section captures rejected\n  approaches with reasoning. That's where former designs live in\n  their proper context.\n- The commit log is the long-term design history. git log -S finds\n  any deleted decision.\n\nFrontmatter Supersedes / Superseded by fields are dropped from the\ntemplate — meaningless under deletion. Status drops Superseded and\nDeprecated values for the same reason.\n\n* docs: ADR-015 — WAL + S3 Object Lock is the audit log\n\nResolves the locked-vs-locked contradiction the consistency audit\nsurfaced: CLAUDE.md invariant #2 forbade a hash-chained audit_log\ntable, but ADR-001 and ADR-011 each described that exact table as\naccepted design.\n\nADR-015 commits to the single audit posture: WAL is the event\nstream, S3 Object Lock COMPLIANCE archives it for tamper evidence,\nnothing else duplicates that role.\n\nPer ADR-014's \"delete superseded content\" policy, the contradictory\nmaterial is removed outright (not retained behind a \"Superseded by\"\nbanner that LLMs miss):\n\n- ADR-001: dropped the `audit_log` row from the per-tenant tables\n  list and the \"audit\" suffix in two prose lines.\n- ADR-011: removed the entire \"Audit logging (tamper-evident)\"\n  subsection (CREATE TABLE audit_log with prev_hash chain, what-gets-\n  logged checklist, audit-log retention rules). Replaced with a\n  one-paragraph pointer to ADR-015. Also fixed the \"Audit log chain\n  broken\" alert line and the \"tamper-evident, append-only\"\n  checklist entry to reference ADR-015 instead.\n- ADR-003: replaced \"All logged in audit_log\" with \"All flow through\n  the WAL and are therefore in the audit log\".\n- CLAUDE.md: invariant #2 body replaced with a one-line pointer to\n  ADR-015 (per ADR-014's CLAUDE.md-is-execution-only policy).\n\nThe two known implementation gaps are tracked separately:\n\n- #510: 13 admin RPCs still write directly to globalstore SQLite,\n  bypassing the WAL. Closing that carve-out is the correctness half\n  of ADR-015.\n- #511: S3 Object Lock archive isn't ported from Python yet\n  (audit/s3_lock.go does not exist). Building the archiver is the\n  tamper-evidence half of ADR-015.\n\nADR-015 references both EPICs in its \"Context\" and \"References\"\nsections; the design is locked even though both pieces ship later.\n\n* docs: ADR-016 — handlers append to the WAL; only the applier writes SQLite\n\nLifts CLAUDE.md invariant #1 into its own ADR. Names the actors\n(handlers, applier) and the rule explicitly: handlers can't write\nSQLite, the applier is the only SQLite writer.\n\nThe ADR is written for the destination state (no carve-outs). The\nknown implementation gap — 13 admin RPCs still direct-write to\nglobalstore — is documented in the \"Implementation status\" section\nand tracked in EPIC #510. After #510 lands, the ADR matches reality\nwith no edits.\n\nAlso in this commit:\n\n- ADR-014 title renamed for clarity:\n    \"Decision records home & CLAUDE.md scope\"\n  becomes\n    \"All design decisions live in docs/adr/; CLAUDE.md is execution-only\"\n  The body's references to \"ADR-015 through ADR-020\" updated to\n  match the actual non-sequential migration (ADR-015 = audit log /\n  invariant #2; ADR-016 = write path / invariant #1).\n\n- CLAUDE.md invariant #1 body replaced with a one-line pointer to\n  ADR-016 (per ADR-014's CLAUDE.md-is-execution-only policy).\n\n- CLAUDE.md migration notice no longer cites a fixed ADR-015-020\n  range; invariants migrate in discussion order.\n\n* docs: drop CLAUDE.md invariant #3 (single applier goroutine) — not ADR-worthy\n\nThe rule \"single consumer goroutine per server\" was an implementation\nfact restating standard Kafka consumer-group semantics (one consumer\nper partition, serial within a consumer) plus single-writer SQLite\ncontention avoidance. Neither is a project-specific design decision;\nboth fall out of using Kafka + SQLite correctly.\n\nThe genuine ordering constraint (per-tenant event ordering preserved\nend-to-end) is already implicit in ADR-016 (\"the applier processes\nWAL events in order\") and in Kafka's partition-by-tenant-id keying.\n\nThe original \"Python-parity ordering guarantee\" framing is also\nstale — Python is gone. Keeping a CLAUDE.md invariant with stale\nrationale invites confusion.\n\nChanges:\n\n- CLAUDE.md: delete the \"### 3. Single consumer goroutine for the\n  applier\" block entirely. The numbered list now skips #3; the\n  remaining invariants (#4 #5 #6) keep their original numbers to\n  avoid churn in cross-references.\n- ADR-014: rewrite the migration plan paragraph to acknowledge two\n  outcomes per invariant (lift into ADR or drop). Note invariant #3's\n  drop with the reasoning.\n- ADR-016: replace the \"(ADR-017, forthcoming)\" stub with a direct\n  description of the per-partition serial-apply semantics. No\n  forthcoming ADR; ADR-017 number is just unused (next invariant\n  migration claims it).\n\n* docs: ADR-015 & ADR-016 — reflect that #510 is closed, file #513 follow-up\n\nEPIC #510 (admin-plane WAL routing) landed on main via commit\nd8d0afd. The 13 admin RPCs that previously direct-wrote to globalstore\nnow flow through the WAL via dedicated op types; the applier is the\nsole writer of globalstore.\n\nOne small residual remains: server/go/internal/api/remove_group_member.go:231\nstill calls s.global.RemoveShared directly as a \"best-effort shared_index\ncascade\" after the main WAL append. Filed as #513 — same recipe as\n#510, applied to one cleanup site.\n\nAlso filed:\n- #514 (EPIC): holistic file-strategy ADR. Captures the Q1-Q5\n  open questions about SQLite file proliferation, single-tenant\n  scale, fleet-level scale, tenant mobility, and public.db\n  semantics. Defer-and-track per the (b) recommendation.\n\nADR changes:\n\n- ADR-015 \"Context\" — replace the \"two implementation gaps exist\"\n  framing with \"implementation status: admin-plane closed; S3 OL\n  archive open\". The S3 OL piece (EPIC #511) is the only design-locked\n  but not-yet-shipped half now.\n- ADR-015 \"Failure modes\" — the grep-for-direct-writes diagnostic\n  is updated to say the grep currently has one expected hit\n  (#513), any other hit is a regression.\n- ADR-015 frontmatter Implementation field — list 510 (closed),\n  511 (open), 513 (open).\n- ADR-016 \"Implementation status (and known gap)\" — rename to\n  \"Implementation status\" (no gap section anymore). Rewrite the body\n  to say \"the rule holds across all 44 RPCs\" with one residual\n  tracked in #513.\n- ADR-016 \"Failure modes\" — update the grep audit command to match\n  the current pattern; note the one expected hit.\n- ADR-016 \"References\" — replace forward-looking \"(ADR-017,\n  forthcoming)\" / \"after #510 lands\" language with closed-state\n  references.\n\n* docs: remove all design decisions from CLAUDE.md; complete ADR migration\n\nPer user direction \"never write design decisions in CLAUDE.md again;\nall design lives in docs/adr/.\"\n\nThe \"Architecture Invariants\" section in CLAUDE.md is gone. Its\ncontent has been distributed:\n\n- #1 (write path)              -> ADR-016 (already lifted)\n- #2 (audit log)               -> ADR-015 (already lifted)\n- #3 (single applier goroutine) -> dropped (standard Kafka semantics)\n- #4 (per-tenant SQLite)       -> already covered by ADR-001\n- #5 (proto is the type system) -> folded into ADR-006 (widened)\n- #6 (field IDs on disk)       -> NEW: ADR-018\n\nChanges:\n\n* NEW ADR-018: \"Payloads are keyed by field_id on the wire and on\n  disk; the proto field number IS the field_id.\" Records the design\n  PR #507 / v1.12.2 implemented (SDK pre-translates names->ids;\n  server rejects schemaless name-keyed payloads with INVALID_ARGUMENT).\n  Decision was previously only documented in CLAUDE.md invariant #6.\n\n* ADR-006 rewritten substantially:\n  - Title widened: \"Proto-Based Schema Definition\" -> \"Proto is the\n    type system end-to-end\"\n  - Stale \"entdb generate / lint / check / init\" CLI section replaced\n    with current \"entdb-schema snapshot / check / diff / validate\"\n    (the actual binary at server/go/cmd/entdb-schema/)\n  - Generated-code section rewritten: drop the legacy NodeTypeDef\n    hand-built pattern; show register_proto_schema(my_pb2) for Python\n    and direct &myschema.Task{...} for Go (the actual SDK v0.3 API)\n  - Decision section absorbs CLAUDE.md invariant #5's \"no custom\n    codegen\" rule and broadens to \"proto is the type system end-to-\n    end\" (wire, disk, codegen, registry, SDK types — one descriptor)\n  - References ADR-018 for the on-disk format consequence\n\n* CLAUDE.md:\n  - \"Architecture Invariants (MUST NOT violate)\" section header\n    deleted entirely. Replaced with \"Architecture decisions\" — a\n    list of ADR pointers (number + one-line summary) as orientation\n    only, no normative content\n  - Key Patterns: fixed the stale \"register via the schema RPCs\"\n    claim (there is no RegisterSchema RPC; loader reads\n    .schema-snapshot.json at boot; SDK has its own register_proto_schema)\n\n* ADR-014 narrative updated to record the completed migration with\n  the final mapping (1->ADR-016, 2->ADR-015, 3 dropped, 4 already in\n  ADR-001, 5 folded into ADR-006, 6->ADR-018). Drops the \"section\n  read-only until migration completes\" language since migration IS\n  complete.\n\n* ADR-015 + ADR-016: replaced \"CLAUDE.md invariant #N\" cross-\n  references with self-contained framing.\n\n* docs: rebase cleanup — renumber + reflect #511/#513 closures and new ADR-014\n\nRebased onto origin/main which added PR #515 (commit eda6ba9, v1.12.3):\n1. New ADR-014 \"Physical Storage Layout\" — addresses our deferred\n   file-strategy discussion (#514).\n2. RemoveGroupMember shared_index cascade routed through WAL replay\n   (closes #513).\n3. S3 Object Lock archiver foundation in server/go/internal/audit/\n   (substantial progress on #511; issue still open for full integration).\n\nBranch cleanup:\n\n* My ADR-014 (decision records home + CLAUDE.md scope) renumbered to\n  ADR-019 — their ADR-014 (physical layout) shipped first and was\n  released in v1.12.3. References in ADR-015, ADR-016, ADR-018, and\n  CLAUDE.md updated.\n\n* ADR-015: drop \"S3 OL archive open\" framing — foundation shipped in\n  eda6ba9. Drop \"residual cascade tracked in #513\" — closed in eda6ba9.\n  Implementation line updated.\n\n* ADR-016: drop the \"one residual write path remains\" paragraph —\n  no handler in server/go/internal/api/ writes globalstore directly\n  anymore (verified by grep). Failure-modes diagnostic now expects\n  zero matches. Issue #513 marked closed.\n\n* ADR-001: clean up rebase artifacts. The \"Tables per tenant file\"\n  list still listed notifications + read_cursors + groups (not in Go\n  DDL); the \"Why NOT per-user mailbox files\" section contradicted\n  the new ADR-014 which formally introduces per-user mailbox files.\n  Both removed. The list now reflects the actual DDL in\n  server/go/internal/store/schema.go (added applied_offsets). The\n  Supersedure note reframed: ADR-014 owns physical layout, ADR-001\n  retains the tenant-file-as-boundary decision and table list.\n\n* CLAUDE.md \"Architecture decisions\" pointer list reflects the new\n  ADR-014 (physical layout) and ADR-019 (decision records home).\n  Top-of-file scope note points at ADR-019.\n\n* Stray sed artifact in ADR-016 (\"ADR-019. \\\"one home\\\" policy\")\n  fixed to \"ADR-019's \\\"one home\\\" policy\".\n\n* docs: ADR-003 — merge typed-capability ACL model; delete decisions/acl.md\n\nResolves a locked-vs-locked contradiction: ADR-003 described the old\nstring-permission hierarchy (read/comment/write/share/delete/admin/\ndeny) as Accepted, while decisions/acl.md (2026-04-13, frozen)\nreplaced it with a typed CoreCapability + per-type ExtensionCapability\nmodel that the Go code (server/go/internal/acl/) actually\nimplements.\n\nPer ADR-019's delete-don't-supersede policy:\n\n* ADR-003 rewritten in place to capture both 2026-04-13 acl.md\n  decisions (typed capabilities and cross-tenant `tenant:<id>`\n  grantee). Visibility check order, propagate_share inheritance,\n  cycle detection, group-based sharing, admin-doesn't-grant-data,\n  audit-via-WAL, and GDPR semantics all retained from the original\n  ADR-003 where still correct.\n\n* decisions/acl.md deleted — its content lives in ADR-003 now.\n\n* decisions/INDEX.md \"### acl\" section replaced with a pointer to\n  ADR-003.\n\n* References in decisions/sdk_api.md and decisions/unique_keys.md\n  that linked to acl.md updated to point at ADR-003.\n\n* CLAUDE.md ADR pointer for ADR-003 simplified — no more \"read with\n  decisions/acl.md\" caveat since there's nothing to read alongside.\n\nNotes on what's NOT in this commit:\n\n* docs/go-port/shared/acl.md has nine references to decisions/acl.md\n  and many more dead Python source citations. That whole doc tree is\n  scheduled for its own cleanup pass and is left alone here.\n\n* The legacy `permission: string` wire field stays mapped on read\n  (READ→[CORE_CAP_READ], WRITE→[READ,COMMENT,EDIT], ADMIN→\n  [CORE_CAP_ADMIN]) per the migration path the original acl.md\n  defined. The mapping is in the new ADR-003's \"ACL entry wire\n  format\" section.\n\n* docs: ADR-005 — rewrite for single-topic, multi-backend reality\n\nTwo corrections after verifying against the deleted Python source\nand the shipped Go code:\n\n1. Multi-topic was the wrong design. Original ADR-005 proposed three\n   topics (entdb-events, entdb-global, entdb-fanout); single-topic\n   with scope-tagged events handles the same workloads with simpler\n   operations and is what the Go server actually ships. ADR-005's\n   three-topic sections are removed; replaced with the single-topic\n   design partitioned by tenant_id (with __global__ sentinel for\n   control-plane events). The original \"Alternatives considered\"\n   captures the rejected multi-topic shape with reasoning.\n\n2. Multi-backend support was correctly designed. The Python source\n   (git show 8d07f5f^:server/python/entdb_server/wal/) had 7 backend\n   implementations:\n   - memory.py, kafka.py     (ported to Go)\n   - kinesis.py, pubsub.py, sqs.py, servicebus.py, eventhubs.py (unported)\n   The Go port today ships only memory + kafka. Filed EPIC #518 to\n   port the remaining 5. ADR-005 now lists the full backend\n   inventory with status flags and points at #518.\n\nOther cleanups in the rewrite:\n\n- Drop the entdb-fanout / notification-fanout sections. The Go\n  server doesn't have notifications; the section was aspirational\n  Python-era content.\n- Drop the \"Three topics to manage\" consequence line (no longer\n  applies).\n- Reframe recovery tiers: SQLite is materialized view, WAL is\n  durable, S3 archive is the long-term backstop via ADR-015.\n- Add references to ADR-014 (physical layout), ADR-015 (audit log),\n  ADR-016 (write path).\n- CLAUDE.md ADR-005 pointer updated to describe single-topic +\n  swappable backends accurately, with the #518 EPIC linked.\n\nFiles this commit touches:\n\n- docs/adr/005-event-sourcing-wal.md (full rewrite)\n- CLAUDE.md (ADR-005 pointer line)\n\n* docs: ADR-011 — rewrite to match v1.13.0 shipped reality\n\nPR #515 / commits a68d8e1 + 7660da9 + 6a652cf closed both EPICs I\nfiled earlier today (#519 encryption-at-rest, #520 TLS/mTLS).\nADR-011 was written when this was design intent; updating to point\nat actual shipped code + CLI flags rather than aspirations.\n\nSections updated:\n\n* Encryption at rest — SHIPPED in v1.13.0. Real driver\n  (mutecomm/go-sqlcipher/v4), real cipher (AES-256 + HMAC-SHA512 +\n  PBKDF2-SHA512), real CLI flags (-kms-provider, -kms-key-id,\n  -encryption-required), real KMS providers (file/aws/gcp/azure/vault).\n  Real code paths (server/go/internal/crypto/{key_manager,master_key,\n  sqlcipher,tenant_key_vault}.go).\n\n* Encryption in transit — SHIPPED. Real CLI flags (-tls-cert,\n  -tls-key, -tls-ca, -tls-min-version, -require-tls,\n  -require-client-cert) with SIGHUP cert reload. Real code paths\n  (server/go/cmd/entdb-server/tls.go, server/go/internal/auth/mtls.go).\n  Honest about plaintext default for local dev.\n\n* Crypto-shred for GDPR — SHIPPED. New gdpr/processor.go runs the\n  deletion-queue worker on a configurable interval, calls into the\n  crypto package to wipe the tenant key from tenant_key_vault. CLI\n  flags (-gdpr-worker-enabled, -gdpr-worker-interval,\n  -crypto-shred-delete-files).\n\nSections trimmed for accuracy:\n\n* Monitoring/Prometheus — metrics package records internally, but\n  /metrics HTTP endpoint isn't exposed yet. Marked ⏳ planned.\n* OpenTelemetry tracing — go.opentelemetry.io/otel is a transitive\n  dep but there's no actual instrumentation. Issue #517 referenced.\n* HTTP /health — REMOVED the claim entirely. Server is gRPC-only;\n  use grpc.health.v1.Health/Check + entdb.v1.EntDBService/Health,\n  both of which bypass auth.\n* Per-tenant SQLite snapshots — REMOVED from \"✅ shipped\" claims.\n  WAL replay is the durability mechanism today; snapshots remain\n  an optional optimization not implemented in Go.\n* Rate limiting — marked ⏳ planned (Python's QuotaInterceptor not\n  ported yet).\n* Backup checksums — folded into the WAL archive (ADR-015), since\n  Object Lock + COMPLIANCE retention serves the same purpose.\n\nSections kept:\n* Audit logging — already pointed at ADR-015, no change.\n* Authentication — three credential carriers (API key / bearer /\n  session) + new mTLS subject extraction. Real and shipped.\n* Data residency — region pin is real (in CreateTenant + tenant_registry).\n* Compliance mapping table — references unchanged (controls map\n  to standards; not implementation-dependent).\n\nCLAUDE.md ADR-011 pointer updated to summarize the shipped surface.\n\n* docs: ADR-017 — Python server retired (migrated from decisions/)\n\n* docs: ADR-020 — immutable storage mode (migrated from decisions/storage.md)\n\n* docs: ADR-021 — go-console binary (migrated from decisions/console.md)\n\n* docs: ADR-022 — FTS5 full-text search (migrated from decisions/fts.md)\n\n* docs: ADR-023 — declarative query indexes (migrated from decisions/query_indexes.md)\n\n* docs: ADR-024 — three-layer rate-limit model (migrated from decisions/quotas.md)\n\n* docs: ADR-025 — single-shape SDK API (migrated from decisions/sdk_api.md + unique_keys.md)\n\n* docs: retire docs/decisions/ folder; mark ADR-019 migration complete\n\n* docs: rewrite all user-facing docs for v1.13.0 reality (Phase C)\n\nNine top-level / docs/* files rewritten end-to-end to reflect what\nthe v1.13.0 server actually ships, what the SDKs actually expose,\nand the ADRs that govern each design area.\n\nFiles rewritten:\n\n* README.md — Quick start uses grpcurl (no HTTP /health); install\n  instructions drop the stale `pip install -e ./server/python`; Go SDK\n  install added; project structure matches the actual tree (no\n  server/python/); license table points at server/go/; security/\n  encryption features (SQLCipher, KMS, TLS/mTLS, crypto-shred,\n  Object Lock archive) called out as shipped. New code example uses\n  proto messages + register_proto_schema() and plan.edge_create()\n  per the single-shape SDK API (ADR-006).\n* docs/getting-started.md — protoc-first schema definition; proto\n  messages + register_proto_schema (Python) / generated proto types\n  (Go); plan.edge_create not plan.link; wait_applied not\n  wait_for_applied; edges_in not edge_in; gRPC health probe not\n  HTTP. Onboarding step cross-links to docs/onboarding.md.\n* docs/sdk-reference.md — covers both Python and Go SDKs side-by-\n  side per the single-shape API. Operations table replaces the\n  stale chapter that documented nonexistent methods (plan.link,\n  plan.set_visibility, plan.unlink, client.edge_in/edge_out,\n  client.mark_read, client.unread_count, etc.). ACL section uses\n  the typed-capability model from ADR-003.\n* docs/api-reference.md — replaced the entire HTTP-API\n  fabrication (which never existed in the Go server) with a\n  wire-level reference that points at proto/entdb/v1/entdb.proto\n  as the source of truth. Documents wire conventions: id-keyed\n  payloads (ADR-018), trusted-actor pattern, gRPC status codes,\n  cross-region redirect trailer.\n* docs/operations.md — production checklist; CLI-flag-only config\n  (no ENTDB_* env vars); gRPC-only health probe; metrics record\n  internally but /metrics HTTP endpoint pending; Kafka ops via\n  kafka-console-consumer + kafka-consumer-groups; backup story\n  via WAL + Object Lock archive (no built-in SQLite snapshots —\n  WAL replay is the durability mechanism); python -m\n  dbaas.entdb_server.tools.* invocations removed (those tools\n  are gone with the Python server).\n* docs/deployment.md — Terraform task definition uses container\n  command CLI flags not env vars; removes the broken HTTP target\n  group (server is gRPC-only) and the curl-based healthcheck\n  (distroless image, no shell); env var configuration table dropped.\n* docs/durability.md — write-path diagram matches ADR-016; Kafka\n  config recommendations only (no ENTDB_* env vars); recovery\n  procedures via WAL replay + per-tenant key vault restoration\n  (not python -m dbaas.entdb_server.tools.restore which doesn't\n  exist); failure scenarios reflect crypto-shred semantics.\n* docs/schema-evolution.md — proto-based examples throughout\n  (no NodeTypeDef hand-built objects per ADR-006); entdb-schema\n  CLI subcommands (not the nonexistent entdb generate/lint/init);\n  breaking-change list explicitly enumerates field_id reassign,\n  type_id reuse, enum reorder.\n* docs/COMPLIANCE-ROADMAP.md — converted from speculative\n  \"Phase 0-5\" roadmap to a \"shipped vs needed\" status matrix.\n  Notes that hash-chained audit_log was REJECTED per ADR-015 (not\n  a roadmap item); per-tenant snapshot is intentionally not\n  shipped; SQLCipher + TLS + crypto-shred all marked ✅ since\n  they landed in v1.13.0.\n\nOther characteristics across all files:\n\n* All ADR cross-references use the current numbering (ADR-014\n  is theirs/physical-storage-layout from PR #515; ADR-019 is mine/\n  decision-records-home).\n* All code examples use the single-shape SDK API.\n* All \"EntDB provides X\" claims are verified against the v1.13.0\n  shipped surface (server/go/internal/{crypto,gdpr,audit,auth}/).\n\nDoesn't touch CLAUDE.md (already up to date), the ADRs (covered\nby earlier commits on this branch), or `docs/decisions/` and\n`docs/go-port/` (other agents own those passes).\n\n* chore: remove Python source citations and Wave/Phase 0 annotations from Go source\n\nThe Python server was retired in EPIC #407 Phase 4D (commit 8d07f5f) and the\nhistorical \"Wave N\" / \"Phase 0\" rollout annotations no longer apply. This\nsweep:\n\n- Drops \"Source-of-truth Python: server/python/...\" citation comments.\n- Replaces inline \"mirrors/ported from server/python/.../*.py:NNN\" prose\n  with neutral descriptions of the Go behaviour.\n- Drops \"Wave 2 of the Python -> Go server port (EPIC #407)\" headers and\n  similar Wave-N annotations from RPC handler comments.\n- Drops \"Phase 0\" stub annotations.\n\nNo logic changes — comments only. go vet and go test pass for both the\nserver and the SDK module.\n\n* chore: rewrite server/go/README.md and cmd/entdb-server header for post-Phase-4D reality\n\nThe README still described the Go server as a \"Phase 0 skeleton\" with every\nRPC returning Unimplemented. Post-Phase 4D the Go server is canonical and\nall 44 RPCs are implemented. Rewrite the README to reflect that, and clean\nup the cmd/entdb-server/main.go package doc and flag comments to drop\n\"Wave-1 wiring\" / Python-source citation language.\n\n* docs: bring docs/go-port/rpcs/ specs current — replace Python citations with Go paths\n\nAdd a banner at the top of each RPC spec pointing to the implementation in\nserver/go/internal/api/<rpc>.go and noting that the Python-source citations\nbelow are historical (Python server retired in EPIC #407 Phase 4D, commit\n8d07f5f).\n\nReplace dead references throughout each spec:\n- server/python/entdb_server/api/grpc_server.py -> server/go/internal/api/<rpc>.go\n- server/python/entdb_server/apply/canonical_store.py -> server/go/internal/store/\n- server/python/entdb_server/apply/applier.py -> server/go/internal/apply/applier.go\n- server/python/entdb_server/global_store.py -> server/go/internal/globalstore/\n- server/python/entdb_server/auth/auth_interceptor.py -> server/go/internal/auth/interceptor.go\n- tests/python/unit/test_*.py references dropped (those Python unit tests were\n  retired with the Python server; behavioural pins live in tests/python/integration/\n  which still drive against the Go server).\n\nThe design content of each spec is preserved — only the dead citations are\nrewritten.\n\n* docs: refresh docs/go-port/shared/ specs — Python citations -> Go paths\n\nSame treatment as the per-RPC specs: replace dead Python source references\nwith the corresponding Go paths. Adds a historical banner to test-harness.md\nnoting that the cross-implementation harness has shipped (Phase 4A, ADR-016)\nand the dual-server matrix is gone with the Python server retirement\n(Phase 4D, commit 8d07f5f).\n\nDesign content preserved — only dead citations are rewritten.\n\n* docs: archive docs/go-port/PLAN.md (port complete)\n\nEPIC #407 closed with Phase 4D (commit 8d07f5f); the Go server is canonical.\nThis file documents the original Python -> Go port plan and is kept for\nhistorical reference only. Adds a status banner at the top noting the\narchival.\n\n* docs: review fixes — accurate KMS provider list, missing ADR pointers, post-cleanup wording\n\nPrincipal-review pass surfaced four issues; all addressed here.\n\n1. CLAUDE.md scope banner pointed at deleted `docs/decisions/python-server-retired.md`.\n   Updated to ADR-017.\n\n2. CLAUDE.md \"Architecture decisions\" pointer list was missing 7 of\n   the 25 ADRs (017, 020-025). Added all with one-line summaries.\n\n3. README + ADR-011 + operations.md overstated the KMS provider\n   support — listed \"file / aws / gcp / azure / vault\" as if all five\n   shipped. Verified against `server/go/internal/crypto/master_key.go`:\n   only `file`, `aws`, and `vault` are implemented; `gcp` and `azure`\n   are flag-recognized but error at boot. Docs now state that\n   honestly.\n\n4. Agent B's mechanical Python→Go path replacement left 18 RPC spec\n   docs with \"Source of truth: Python handler at `server/go/...`\"\n   (mismatched label vs path). Fixed across all 18 to \"Go handler\n   at `server/go/...`\".\n\nGo test suite still green after these edits.",
          "timestamp": "2026-05-17T01:41:02+01:00",
          "tree_id": "ebd744316a4d0c9f7cc3345d107e469b8a689a5e",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/f7011b8919be0b6233f872f6acdf9376fa16b633"
        },
        "date": 1778978559082,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3331.249272920882,
            "unit": "iter/sec",
            "range": "stddev: 0.000027429568444672634",
            "extra": "mean: 300.18768277979603 usec\nrounds: 1324"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2187.3818813756207,
            "unit": "iter/sec",
            "range": "stddev: 0.00004053464578178802",
            "extra": "mean: 457.16754285772487 usec\nrounds: 1295"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1091.1297027169792,
            "unit": "iter/sec",
            "range": "stddev: 0.0000936354515598292",
            "extra": "mean: 916.4813289473647 usec\nrounds: 912"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 725.4115260584144,
            "unit": "iter/sec",
            "range": "stddev: 0.00015439138719681948",
            "extra": "mean: 1.3785278618794292 msec\nrounds: 543"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1773.2603770585856,
            "unit": "iter/sec",
            "range": "stddev: 0.00013335648982509412",
            "extra": "mean: 563.932975065264 usec\nrounds: 762"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1784.1895233400942,
            "unit": "iter/sec",
            "range": "stddev: 0.00014479738677768787",
            "extra": "mean: 560.4785741191602 usec\nrounds: 1221"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1861.733670566482,
            "unit": "iter/sec",
            "range": "stddev: 0.00012605405479412213",
            "extra": "mean: 537.1337564603015 usec\nrounds: 1548"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2051.4414772847354,
            "unit": "iter/sec",
            "range": "stddev: 0.00003636725082504659",
            "extra": "mean: 487.46211435852837 usec\nrounds: 1574"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1936.107738518425,
            "unit": "iter/sec",
            "range": "stddev: 0.000025207267846843952",
            "extra": "mean: 516.5001823530924 usec\nrounds: 510"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1632.2015126979172,
            "unit": "iter/sec",
            "range": "stddev: 0.00005614723803687953",
            "extra": "mean: 612.669448116776 usec\nrounds: 1301"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2642.481528186786,
            "unit": "iter/sec",
            "range": "stddev: 0.000035807059221753144",
            "extra": "mean: 378.43216284890303 usec\nrounds: 2008"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 230.09658290328255,
            "unit": "iter/sec",
            "range": "stddev: 0.00011337375396118575",
            "extra": "mean: 4.346001089552616 msec\nrounds: 201"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "26a6434311434dd40200a6be663e2c77d4228082",
          "message": "feat(sdk/go): add read-only DbClient.Transport accessor (#528)\n\nThe Transport interface is exported but DbClient.transport is private\nwith no accessor, forcing external consumers to reach it via reflection\nand unsafe. Add a read-only DbClient.Transport() accessor returning the\nunderlying Transport so advanced consumers (custom tooling, tests) have\na supported, non-breaking path.\n\nCloses #509",
          "timestamp": "2026-05-17T11:48:08+01:00",
          "tree_id": "1aa3442de0f4b76c8ca78b08b94c173035f888a7",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/26a6434311434dd40200a6be663e2c77d4228082"
        },
        "date": 1779014985993,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3291.982177894492,
            "unit": "iter/sec",
            "range": "stddev: 0.00005152342543449466",
            "extra": "mean: 303.76835169854616 usec\nrounds: 1325"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2164.7815771010823,
            "unit": "iter/sec",
            "range": "stddev: 0.000040812847358193684",
            "extra": "mean: 461.9403687549518 usec\nrounds: 1261"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1023.1443583587782,
            "unit": "iter/sec",
            "range": "stddev: 0.00011423786646673893",
            "extra": "mean: 977.379185869818 usec\nrounds: 920"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 803.4261876147908,
            "unit": "iter/sec",
            "range": "stddev: 0.00010826330765438996",
            "extra": "mean: 1.2446694113479135 msec\nrounds: 705"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1927.4792039084086,
            "unit": "iter/sec",
            "range": "stddev: 0.00007802974345286887",
            "extra": "mean: 518.8123420331951 usec\nrounds: 1456"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1942.3536671152847,
            "unit": "iter/sec",
            "range": "stddev: 0.00008778991442647476",
            "extra": "mean: 514.8392988003903 usec\nrounds: 1834"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1967.7798342937099,
            "unit": "iter/sec",
            "range": "stddev: 0.00008868139848078591",
            "extra": "mean: 508.1869336052665 usec\nrounds: 1717"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2052.126605994382,
            "unit": "iter/sec",
            "range": "stddev: 0.000038250604653537425",
            "extra": "mean: 487.2993688980697 usec\nrounds: 1434"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1923.052364865258,
            "unit": "iter/sec",
            "range": "stddev: 0.00003357828240424426",
            "extra": "mean: 520.0066406252369 usec\nrounds: 384"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1625.3314306450104,
            "unit": "iter/sec",
            "range": "stddev: 0.00004851082225876556",
            "extra": "mean: 615.2591287815995 usec\nrounds: 1289"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2654.6374763030617,
            "unit": "iter/sec",
            "range": "stddev: 0.000029965529638691556",
            "extra": "mean: 376.69927021170287 usec\nrounds: 1843"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 155.80895631726713,
            "unit": "iter/sec",
            "range": "stddev: 0.00013454688164339355",
            "extra": "mean: 6.418116285714299 msec\nrounds: 140"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "39733193a9313dbd066e1889d00f25c24b9003c5",
          "message": "Clamp paginated reads to a max page size (SEC-4 #135) (#530)\n\nSeveral read RPCs applied only a default-when-zero limit with no upper\nceiling. A client passing limit=10_000_000 to QueryNodes, SearchNodes,\nListUsers, GetEdgesFrom or GetEdgesTo would make the server materialise\nthat many protos in memory — an amplification DoS. ListUsers was the\nworst: it documented \"No upper cap on limit\" and a negative limit fell\nthrough to SQLite as LIMIT -1 (full-table scan).\n\nAdd a single server-wide MaxPageSize ceiling (1000, matching the value\nGetConnectedNodes and ListSharedWithMe already enforce) and clamp at\nhandler entry for all five uncapped read RPCs, after the existing\n'<=0 => default' coercion so small/default page sizes are unaffected.\nWiden the ListUsers zero-coercion to <=0 so a negative limit can no\nlonger trigger an unbounded scan. Add a defence-in-depth clamp at the\nstore layer (store.QueryNodes) for any future in-process caller that\nbypasses the handler.\n\nTests: unit coverage of the clamp helper and the shared-ceiling\ninvariant; handler-level tests asserting a 10M limit returns exactly\nMaxPageSize for QueryNodes and ListUsers, that small/unset limits are\nuntouched, and that a negative ListUsers limit coerces to the default;\na store-layer test pinning the defence-in-depth clamp.\n\nCloses #135",
          "timestamp": "2026-05-17T11:48:19+01:00",
          "tree_id": "b514b3995788192d040ca77f903f8a854a5acbca",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/39733193a9313dbd066e1889d00f25c24b9003c5"
        },
        "date": 1779015097090,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3367.761732386273,
            "unit": "iter/sec",
            "range": "stddev: 0.000027776506774894865",
            "extra": "mean: 296.93312041153115 usec\nrounds: 1362"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2186.153061940984,
            "unit": "iter/sec",
            "range": "stddev: 0.00003711457974190007",
            "extra": "mean: 457.4245131363979 usec\nrounds: 1218"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1093.4708890353897,
            "unit": "iter/sec",
            "range": "stddev: 0.00009067648148214105",
            "extra": "mean: 914.5190878214914 usec\nrounds: 854"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 815.4849101434397,
            "unit": "iter/sec",
            "range": "stddev: 0.00007827938035551332",
            "extra": "mean: 1.2262642601493448 msec\nrounds: 665"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1922.9134800633892,
            "unit": "iter/sec",
            "range": "stddev: 0.00008004541920219739",
            "extra": "mean: 520.0441987473273 usec\nrounds: 1756"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1948.8514438644706,
            "unit": "iter/sec",
            "range": "stddev: 0.00007550850052340615",
            "extra": "mean: 513.1227437310728 usec\nrounds: 1635"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1982.6164736087005,
            "unit": "iter/sec",
            "range": "stddev: 0.00007089500382773929",
            "extra": "mean: 504.3839861674453 usec\nrounds: 1735"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1984.4697266003013,
            "unit": "iter/sec",
            "range": "stddev: 0.00005894407705014131",
            "extra": "mean: 503.91295296459487 usec\nrounds: 1552"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1828.575285434014,
            "unit": "iter/sec",
            "range": "stddev: 0.00008093864368628689",
            "extra": "mean: 546.8738465218014 usec\nrounds: 417"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1558.7025740768047,
            "unit": "iter/sec",
            "range": "stddev: 0.00007098695122825479",
            "extra": "mean: 641.5592151006003 usec\nrounds: 1139"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2564.106424022328,
            "unit": "iter/sec",
            "range": "stddev: 0.00002914240376766695",
            "extra": "mean: 389.99941290708773 usec\nrounds: 1906"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 148.6683317085263,
            "unit": "iter/sec",
            "range": "stddev: 0.0008848818695277273",
            "extra": "mean: 6.726382064746401 msec\nrounds: 139"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "d2151d75d4de332401c01953e6d7d31f3ef93296",
          "message": "Sanitize internal error responses (SEC-5, #136) (#533)\n\nHandlers wrapped underlying store/DB/driver errors with %v into\nclient-visible codes.Internal statuses, e.g.\n\n    errs.Errorf(codes.Internal, \"get user: %v\", err)\n\nwhich shipped raw SQLite driver text, table names, internal package\ncontext and the caller's own id back to the client. That lets an\nattacker fingerprint the schema/driver and confirm record existence,\ndefeating the generic 404 on GetUser.\n\nAdd an errs.Internal / errs.InternalNoCtx chokepoint: it logs the full\nop label + underlying error server-side via slog at ERROR level and\nreturns a fixed generic \"internal error\" message to the client. The\ngRPC code stays codes.Internal so SDK retry/observability behaviour is\nunchanged. Typed sentinels (NotFound / AlreadyExists / InvalidArgument\n/ PermissionDenied / FailedPrecondition / ...) and their contractful\nmessages are left untouched - only Internal/Unknown wrapped errors are\nsanitized.\n\nRoute all 32 internal-wrap call sites (31 errs.Errorf(codes.Internal)\nplus one status.Errorf(codes.Internal) in the freeze gate) through the\nsanitizer.\n\nTests: a handler-level regression (CreateUser against a closed\nglobalstore) proves the SQLite/driver detail is logged but never\nreaches the client, plus focused unit tests of the chokepoint itself.",
          "timestamp": "2026-05-17T11:48:31+01:00",
          "tree_id": "05e57bd8da22d67a5e825e546f323155aef2e405",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/d2151d75d4de332401c01953e6d7d31f3ef93296"
        },
        "date": 1779015198930,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3017.628892622742,
            "unit": "iter/sec",
            "range": "stddev: 0.000031847515351755115",
            "extra": "mean: 331.38601053453596 usec\nrounds: 1234"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2032.403892251721,
            "unit": "iter/sec",
            "range": "stddev: 0.000038279427125570424",
            "extra": "mean: 492.02818584060566 usec\nrounds: 1130"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 972.4153301570971,
            "unit": "iter/sec",
            "range": "stddev: 0.00009907616069859928",
            "extra": "mean: 1.028367168829441 msec\nrounds: 770"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 808.4968714083843,
            "unit": "iter/sec",
            "range": "stddev: 0.00008609502583480525",
            "extra": "mean: 1.2368631659118499 msec\nrounds: 663"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1891.148072048006,
            "unit": "iter/sec",
            "range": "stddev: 0.00008776706624440467",
            "extra": "mean: 528.7793244645602 usec\nrounds: 1541"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1890.9661470344395,
            "unit": "iter/sec",
            "range": "stddev: 0.00008096817753684498",
            "extra": "mean: 528.8301969700927 usec\nrounds: 1320"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1955.323517963405,
            "unit": "iter/sec",
            "range": "stddev: 0.00007831640855194581",
            "extra": "mean: 511.42431971644476 usec\nrounds: 1689"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2001.6752857133158,
            "unit": "iter/sec",
            "range": "stddev: 0.00005656409755010502",
            "extra": "mean: 499.58152910083044 usec\nrounds: 1512"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1730.9519432626746,
            "unit": "iter/sec",
            "range": "stddev: 0.00005506976041386185",
            "extra": "mean: 577.7167898232333 usec\nrounds: 452"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1447.1684439666312,
            "unit": "iter/sec",
            "range": "stddev: 0.00006820874584793064",
            "extra": "mean: 691.0045642365167 usec\nrounds: 1152"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2489.5576995679858,
            "unit": "iter/sec",
            "range": "stddev: 0.000031095198916439526",
            "extra": "mean: 401.67777600556536 usec\nrounds: 1692"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 138.2483604821771,
            "unit": "iter/sec",
            "range": "stddev: 0.001691386496436843",
            "extra": "mean: 7.233358837039659 msec\nrounds: 135"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "cf4b24a5e196f0c5e606c0eee19dae3f7e4135b2",
          "message": "fix(testseed): seed contract fixture through the real producer→applier path (#532)\n\nThe contract seed (SeedTenantContract) used to write the seed node with\nthe WAL-bypassing CreateNodeRaw, hand-record the seed-1 receipt, and\npre-bump the in-memory applied-offset tracker to 1. That pre-bump made\nWaitForOffset(target<=1) return before the applier had actually\nprocessed anything, leaving the seed and runtime write paths in two\ndifferent universes (issue #505).\n\nReroute the seed through the same path every real mutation takes:\nbuild a wal.Event (create_node, idempotency key \"seed-1\", actor\nuser:alice), append it via the shared producer, and block until the\nalready-running applier materialises it (poll CheckIdempotencyStatus\nfor the seed-1 receipt the applier writes itself). The applier now\nwrites the applied_events and applied_offsets rows, so there is no\nparallel-universe state and no offset pre-bump.\n\nIdempotency across repeated harness boots is preserved for free: the\nsame idempotency key dedupes at the producer (same StreamPos) and at\nthe applier's in-txn idempotency probe.\n\nSeedTenantContract / SeedTenant now take a wal.Producer + topic;\ncmd/entdb-server passes the shared producer (the applier goroutine is\nalready started before the seed runs). Dead helpers\n(isUniqueOrAlreadyExists, isIdempotencyViolation) removed. seed_test\nwires an in-memory WAL + running applier to mirror the server.\n\nThe #503 waitForIdempotencyRecord poll in execute_atomic.go is\ndeliberately KEPT: WaitForOffset's in-memory tracker is broadcast from\nUpdateAppliedOffsetTx before the applier batch txn commits, so the\nwait returning still does not guarantee the applied_events row is\nvisible on a fresh read connection. That pre-commit broadcast is an\nindependent source of the same race the seed pre-bump used to trigger\nand is not fixed here, so removing the poll would reintroduce the CAS\nflake. Its comment is updated to attribute the cause correctly.\n\nLocal CI green: go vet + go test (server, SDK), Python contract suite\n(95 passed), e2e docker stack (22 passed), ruff check/format.\n\nCloses #505",
          "timestamp": "2026-05-17T11:48:43+01:00",
          "tree_id": "1f3d076b8a16a2e1879e92f7ec24011d297f77bf",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/cf4b24a5e196f0c5e606c0eee19dae3f7e4135b2"
        },
        "date": 1779015250230,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3266.035180804496,
            "unit": "iter/sec",
            "range": "stddev: 0.000029333209691032294",
            "extra": "mean: 306.1816375638912 usec\nrounds: 1363"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2124.6432364708735,
            "unit": "iter/sec",
            "range": "stddev: 0.000047630568602274825",
            "extra": "mean: 470.6672550169149 usec\nrounds: 1196"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1112.9321438191357,
            "unit": "iter/sec",
            "range": "stddev: 0.00007912135295737903",
            "extra": "mean: 898.5273770316328 usec\nrounds: 923"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 825.9633186154747,
            "unit": "iter/sec",
            "range": "stddev: 0.00009437695369688784",
            "extra": "mean: 1.210707518678015 msec\nrounds: 696"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1910.8697128317551,
            "unit": "iter/sec",
            "range": "stddev: 0.0000880491195019692",
            "extra": "mean: 523.3219163425226 usec\nrounds: 1542"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1952.5273030514702,
            "unit": "iter/sec",
            "range": "stddev: 0.00007930889745281574",
            "extra": "mean: 512.1567306317146 usec\nrounds: 1678"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1987.3135252738894,
            "unit": "iter/sec",
            "range": "stddev: 0.00007045740777693079",
            "extra": "mean: 503.191865441655 usec\nrounds: 1687"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2072.910462153857,
            "unit": "iter/sec",
            "range": "stddev: 0.00003868593004021656",
            "extra": "mean: 482.4135042287114 usec\nrounds: 1537"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1799.0677142697443,
            "unit": "iter/sec",
            "range": "stddev: 0.000048504728396511254",
            "extra": "mean: 555.8434471744761 usec\nrounds: 407"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1534.090250132533,
            "unit": "iter/sec",
            "range": "stddev: 0.00005877911312461742",
            "extra": "mean: 651.8521318505271 usec\nrounds: 1259"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2678.0208515116637,
            "unit": "iter/sec",
            "range": "stddev: 0.000029813701685499615",
            "extra": "mean: 373.41008731710565 usec\nrounds: 2050"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 163.27167953798207,
            "unit": "iter/sec",
            "range": "stddev: 0.00011655352628716778",
            "extra": "mean: 6.124760906666419 msec\nrounds: 150"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "bb799b857f90eb3107f8357fa580c9a1442c0720",
          "message": "Add automatic per-tenant offset tracking to the Go SDK (#531)\n\nThe Go SDK only exposed the manual WaitForOffset RPC wrapper: its\nGetNode/GetNodeByKey/QueryNodes/GetNodes calls never set the proto\nafter_offset field and there was no per-tenant offset map, so\nread-after-write consistency required hand-written plumbing. The\nPython SDK has shipped automatic tracking since #74.\n\nMirror the Python semantics exactly:\n\n- offsetTracker: concurrency-safe per-tenant stream_position map\n  (RWMutex; the Go client is safe for concurrent use). Lives on\n  grpcTransport, the single boundary both Plan commits and Scope\n  reads cross. Nil-safe so bare-struct transports in tests degrade\n  gracefully.\n- ExecuteAtomic records receipt.stream_position per tenant on a\n  successful commit (empty position ignored, matching Python).\n- GetNode/GetNodeByKey/QueryNodes/GetNodes resolve and attach the\n  tracked offset as after_offset, with wait_timeout_ms=30000 when an\n  offset is present and 0 otherwise (Python's\n  \"30000 if resolved else 0\"). GetNodeByKey carries it as int64,\n  matching the wire field and Python's int() coercion.\n- Per-call overrides via the request context (Go has no kwargs):\n  WithAfterOffset(ctx, pos) pins an explicit offset, and\n  WithoutOffsetTracking(ctx) opts out — the Go analogues of the\n  Python explicit-string and after_offset=None paths.\n- DbClient.ClearOffsets() drops tracked state (Python clear_offsets),\n  exposed via an optional offsetClearer capability so non-tracking\n  test doubles are a safe no-op.\n\nTests cover the read-after-write contract end-to-end through the\nbufconn fake server for all four read RPCs, per-tenant isolation,\nexplicit override, opt-out, ClearOffsets, nil-safety, and a -race\nconcurrency stress of the tracker. Full Go SDK CI (vet + test -race)\nis green. README documents the feature and the parity table gains\nthe offset-tracking rows.",
          "timestamp": "2026-05-17T11:48:55+01:00",
          "tree_id": "92e3b245ad7ce455744a87c35e6b51fd148ef7d0",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/bb799b857f90eb3107f8357fa580c9a1442c0720"
        },
        "date": 1779015326078,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3026.6727502126582,
            "unit": "iter/sec",
            "range": "stddev: 0.000021175833556187243",
            "extra": "mean: 330.3958116812393 usec\nrounds: 1832"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2070.7108356378944,
            "unit": "iter/sec",
            "range": "stddev: 0.00003443726667386881",
            "extra": "mean: 482.9259512190384 usec\nrounds: 1599"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 922.8871297718365,
            "unit": "iter/sec",
            "range": "stddev: 0.00009938893784482672",
            "extra": "mean: 1.083556122672583 msec\nrounds: 913"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 714.3118955268031,
            "unit": "iter/sec",
            "range": "stddev: 0.0001668189527776364",
            "extra": "mean: 1.3999486866482918 msec\nrounds: 734"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1794.987716784209,
            "unit": "iter/sec",
            "range": "stddev: 0.00010053299198499539",
            "extra": "mean: 557.1068763587638 usec\nrounds: 1472"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1890.860982755643,
            "unit": "iter/sec",
            "range": "stddev: 0.00007634487543495454",
            "extra": "mean: 528.8596089928577 usec\nrounds: 1757"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1933.2081849220635,
            "unit": "iter/sec",
            "range": "stddev: 0.00008000069595285214",
            "extra": "mean: 517.274863514151 usec\nrounds: 1861"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1998.1964032099672,
            "unit": "iter/sec",
            "range": "stddev: 0.000031487675461135414",
            "extra": "mean: 500.45130618470125 usec\nrounds: 1633"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1728.1412046343407,
            "unit": "iter/sec",
            "range": "stddev: 0.000025182345725085635",
            "extra": "mean: 578.6564184213124 usec\nrounds: 380"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1492.4420324334988,
            "unit": "iter/sec",
            "range": "stddev: 0.000040689022134761475",
            "extra": "mean: 670.0427743712443 usec\nrounds: 1272"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2537.6959239536054,
            "unit": "iter/sec",
            "range": "stddev: 0.000025401763645216844",
            "extra": "mean: 394.05824415797196 usec\nrounds: 2097"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 159.35674882918363,
            "unit": "iter/sec",
            "range": "stddev: 0.0002770248400089896",
            "extra": "mean: 6.275228425197804 msec\nrounds: 127"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "b5f0244359b76b9895bcb845465113516b110f27",
          "message": "Harden transient-retry policy in both SDKs (PERF-7) (#534)\n\nThe retry layer had three problems that bite during an outage:\n\n- The exponential backoff was a fixed 0.1 * 2**attempt with no\n  randomness, so every client recovering from the same outage\n  retried in lockstep — a thundering herd.\n- DEADLINE_EXCEEDED was retried unconditionally, including for\n  ExecuteAtomic and the other mutating RPCs. A timeout means the\n  deadline elapsed mid-flight, so the write may already have\n  committed; a blind retry risks a duplicate mutation.\n- There was no ceiling on total time spent retrying a single call,\n  so a high max_retries plus exponential backoff could block a\n  caller for minutes past their own deadline.\n- The Go SDK configured WithMaxRetries but never consumed it —\n  there was no transient-retry loop at all.\n\nPython (_grpc_client.py):\n- Full-jitter backoff: uniform(0, min(cap, base*2**attempt)),\n  cap 5s. RNG is injectable for deterministic tests.\n- Per-call wall-clock budget (default 30s, configurable via\n  DbClient(retry_budget=...)): bail before the next sleep would\n  exceed it.\n- DEADLINE_EXCEEDED now only retries a read-method allowlist;\n  UNAVAILABLE still retries every method (request likely never\n  reached the server).\n\nGo (sdk/go/entdb): new retry.go installs a unary client\ninterceptor with identical semantics — same jitter, same\nallowlist, same budget — chained outside the redirect\ninterceptor so each attempt still follows tenant redirects.\nWithRetryBudget option added for parity.\n\nTests both sides drive the loop with a deterministic jitter\nsource and assert attempt counts, allowlist behaviour, and\nbudget cut-off.\n\nCloses #143",
          "timestamp": "2026-05-17T11:49:07+01:00",
          "tree_id": "c08340150e4cda1e7b7cdf95a8f5d3c69f2ebc86",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/b5f0244359b76b9895bcb845465113516b110f27"
        },
        "date": 1779015331201,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3028.724552286395,
            "unit": "iter/sec",
            "range": "stddev: 0.000025131841278906832",
            "extra": "mean: 330.17198584304947 usec\nrounds: 1554"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2061.614761201652,
            "unit": "iter/sec",
            "range": "stddev: 0.000038188573010967445",
            "extra": "mean: 485.05667441822663 usec\nrounds: 1290"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 966.6136272657674,
            "unit": "iter/sec",
            "range": "stddev: 0.00009076431564055479",
            "extra": "mean: 1.034539522092888 msec\nrounds: 860"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 809.6803393681931,
            "unit": "iter/sec",
            "range": "stddev: 0.00007519333873535956",
            "extra": "mean: 1.2350553068638377 msec\nrounds: 743"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1902.9301519755295,
            "unit": "iter/sec",
            "range": "stddev: 0.00007227960949263512",
            "extra": "mean: 525.5053628541482 usec\nrounds: 1626"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1884.378123707851,
            "unit": "iter/sec",
            "range": "stddev: 0.00007435498519494613",
            "extra": "mean: 530.6790539641381 usec\nrounds: 2094"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1951.5606831894029,
            "unit": "iter/sec",
            "range": "stddev: 0.00006681603340913764",
            "extra": "mean: 512.4104049717362 usec\nrounds: 1931"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1980.4372857365045,
            "unit": "iter/sec",
            "range": "stddev: 0.00004558371739803533",
            "extra": "mean: 504.9389885770153 usec\nrounds: 1138"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1835.565666774409,
            "unit": "iter/sec",
            "range": "stddev: 0.000024403040189050943",
            "extra": "mean: 544.7911878616 usec\nrounds: 346"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1550.0634644933225,
            "unit": "iter/sec",
            "range": "stddev: 0.00005625684063099884",
            "extra": "mean: 645.1348753819415 usec\nrounds: 1308"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2442.4727460806166,
            "unit": "iter/sec",
            "range": "stddev: 0.0000406656660589553",
            "extra": "mean: 409.42114977728136 usec\nrounds: 2023"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 121.24208931742508,
            "unit": "iter/sec",
            "range": "stddev: 0.002057399702420599",
            "extra": "mean: 8.247960799998179 msec\nrounds: 110"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "3a18313e374aeab1ba5b1b172c4cd552ef6091b7",
          "message": "Add production OIDC: network JWKS, discovery, ES256, server flags (#535)\n\nThe shipped MemoryOAuthValidator is in-memory/dev only and was never\nwired into the running server, so there was no production-grade OAuth\npath: no network JWKS fetch, no key rotation, no OIDC discovery, no\nES256, and no flags to turn auth on.\n\nAdd a network-backed JWKSValidator that fetches a JWKS document over\nHTTP, caches it in-process, and lazily refreshes on a kid miss (key\nrotation) with refreshes coalesced through a single-flight latch so a\nburst on an unknown kid triggers exactly one fetch. It performs OIDC\ndiscovery (<issuer>/.well-known/openid-configuration) when no explicit\nJWKS URL is given, verifies the discovered issuer matches, and warms\nthe cache at boot so a misconfigured issuer fails fast instead of\nsilently rejecting every request. Provider preset for Google;\nMicrosoft/Okta are recognised but tenant/org-specific so they require\nan explicit issuer and resolve via discovery.\n\nAdd ES256 (ECDSA P-256) verification alongside HS256/RS256, including\nthe JWS fixed-width R||S signature handling and a P-256 curve check.\nThe existing algorithm-confusion guards are preserved and extended:\nthe JWKS path rejects HS256 outright (a symmetric token at an\nasymmetric endpoint is the classic confusion attack) and the JWKS\nentry pins each kid's algorithm.\n\nWire it into entdb-server via -oauth-provider / -oauth-issuer /\n-jwks-url / -oauth-audience; the auth interceptor installs whenever any\n-oauth-* flag is set, independent of TLS. API-key/session managers stay\nnil here (tracked under sibling issues), and the no-flag path is\nunchanged so existing dev/test boots are unaffected.\n\nTests: JWKS RS256/ES256 verify, cache hit across requests, key\nrotation refetch, single-flight coalescing under concurrency, alg=none\nand HS256-at-JWKS rejection, expiry/issuer/audience enforcement, OIDC\ndiscovery + issuer-mismatch rejection, fail-closed boot, provider\npresets, and ES256 + alg-confusion cases for MemoryOAuthValidator.",
          "timestamp": "2026-05-17T11:49:19+01:00",
          "tree_id": "c01dc84ac9b372d1c3d052ea300a34896f35d7c5",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/3a18313e374aeab1ba5b1b172c4cd552ef6091b7"
        },
        "date": 1779015353367,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3355.5698861279966,
            "unit": "iter/sec",
            "range": "stddev: 0.000024824441880595907",
            "extra": "mean: 298.0119723132643 usec\nrounds: 939"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2184.7529413479015,
            "unit": "iter/sec",
            "range": "stddev: 0.00003837875886108691",
            "extra": "mean: 457.71765817284665 usec\nrounds: 1217"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1109.1595406617555,
            "unit": "iter/sec",
            "range": "stddev: 0.00008159802782319956",
            "extra": "mean: 901.5835534384639 usec\nrounds: 945"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 834.1175593726169,
            "unit": "iter/sec",
            "range": "stddev: 0.00007332500025380248",
            "extra": "mean: 1.1988717762423704 msec\nrounds: 724"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1939.9398654438717,
            "unit": "iter/sec",
            "range": "stddev: 0.00007366396083653916",
            "extra": "mean: 515.4798959560497 usec\nrounds: 1730"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1943.0015557660904,
            "unit": "iter/sec",
            "range": "stddev: 0.0000889603230277353",
            "extra": "mean: 514.6676270188152 usec\nrounds: 1177"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1983.4247097295167,
            "unit": "iter/sec",
            "range": "stddev: 0.00007172146475659294",
            "extra": "mean: 504.1784520958054 usec\nrounds: 1670"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2053.6993883647897,
            "unit": "iter/sec",
            "range": "stddev: 0.00003809611126839022",
            "extra": "mean: 486.92618095203636 usec\nrounds: 1155"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1925.0481104509681,
            "unit": "iter/sec",
            "range": "stddev: 0.00004616167612625349",
            "extra": "mean: 519.4675367182052 usec\nrounds: 531"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1624.3164708414895,
            "unit": "iter/sec",
            "range": "stddev: 0.00004803044858873841",
            "extra": "mean: 615.6435755908715 usec\nrounds: 1270"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2644.6377438056484,
            "unit": "iter/sec",
            "range": "stddev: 0.00002840082973123863",
            "extra": "mean: 378.12362102984827 usec\nrounds: 1921"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 166.1781612712345,
            "unit": "iter/sec",
            "range": "stddev: 0.00010105913810836392",
            "extra": "mean: 6.017637891466429 msec\nrounds: 129"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "e047de768bb974312f7d4294560ad6aff7ee05ee",
          "message": "audit: archive-lag metrics + WAL->S3 Object Lock e2e (#538)\n\nCloses two of the four remaining gaps on the S3 Object Lock archive\nEPIC (#511 / ADR-015); the archive foundation under\nserver/go/internal/audit/ already shipped in eda6ba9.\n\nMetrics\n- New entdb_archive_lag_events gauge ({topic,partition}) plus\n  entdb_archive_writes_total / entdb_archive_errors_total. Lag is the\n  count of WAL records polled by the archive sidecar but not yet\n  durably written to S3 + committed: it drains to 0 once caught up and\n  stays elevated when S3 is unavailable, which is the operator alert\n  signal ADR-015's failure-modes section names.\n- The archiver publishes lag per partition on every poll/commit cycle\n  and counts retryable failures.\n- Added a -metrics-addr flag exposing the Prometheus default registry\n  at /metrics on a separate HTTP listener (off by default; the gRPC\n  metrics were registered but never scrapable before this).\n\nMinIO + Redpanda e2e\n- docker-compose.archive.yml + Dockerfile.archive + run-archive-e2e.sh\n  boot Redpanda + MinIO (Object-Lock COMPLIANCE bucket) + the server\n  with -archive-enabled. Kept separate from the fast 22-case e2e stack\n  so it does not pay the MinIO/archiver startup cost.\n- test_archive_object_lock.py drives the SDK, then asserts the WAL->S3\n  round-trip: COMPLIANCE mode + retain-until on the object, gzip-JSONL\n  body decodes back to the written events, and the lag gauge drains to\n  0 while the writes counter advances. Skips cleanly when collected by\n  the main stack (no boto3 / no S3 endpoint).\n- Added -archive-s3-path-style so the archiver works against MinIO and\n  other non-AWS S3 endpoints (required for path-style addressing).\n\nLegal-hold lift on existing objects and deployment/ops docs remain\nopen on #511.",
          "timestamp": "2026-05-17T11:49:30+01:00",
          "tree_id": "dacd8691e9630372c2febc2f13e6d9161d3b59cc",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/e047de768bb974312f7d4294560ad6aff7ee05ee"
        },
        "date": 1779015415511,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3372.5924586256156,
            "unit": "iter/sec",
            "range": "stddev: 0.000026156036728932312",
            "extra": "mean: 296.50780883484384 usec\nrounds: 1313"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2212.551746626633,
            "unit": "iter/sec",
            "range": "stddev: 0.000032350984653483234",
            "extra": "mean: 451.9668303914926 usec\nrounds: 1303"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1109.8409084145032,
            "unit": "iter/sec",
            "range": "stddev: 0.00008442776982570886",
            "extra": "mean: 901.0300417098341 usec\nrounds: 959"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 852.127693119446,
            "unit": "iter/sec",
            "range": "stddev: 0.00007876666570408528",
            "extra": "mean: 1.1735330374479758 msec\nrounds: 721"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1948.2274340175945,
            "unit": "iter/sec",
            "range": "stddev: 0.00007034036855714018",
            "extra": "mean: 513.2870949968201 usec\nrounds: 1579"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1955.3058347745473,
            "unit": "iter/sec",
            "range": "stddev: 0.00008336284944660255",
            "extra": "mean: 511.4289448818134 usec\nrounds: 1778"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1998.3606513666598,
            "unit": "iter/sec",
            "range": "stddev: 0.00007062740056299126",
            "extra": "mean: 500.4101733669093 usec\nrounds: 1592"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2062.7249940014326,
            "unit": "iter/sec",
            "range": "stddev: 0.00003888548463990859",
            "extra": "mean: 484.79559946579354 usec\nrounds: 1498"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1868.1556631433155,
            "unit": "iter/sec",
            "range": "stddev: 0.00006445931591930448",
            "extra": "mean: 535.2872995162636 usec\nrounds: 414"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1582.798148734052,
            "unit": "iter/sec",
            "range": "stddev: 0.000054893824896625945",
            "extra": "mean: 631.7925003891472 usec\nrounds: 1285"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2580.708188486863,
            "unit": "iter/sec",
            "range": "stddev: 0.00003644124402649199",
            "extra": "mean: 387.4905363036517 usec\nrounds: 1818"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 150.76288546342866,
            "unit": "iter/sec",
            "range": "stddev: 0.0012695036609685008",
            "extra": "mean: 6.63293221621561 msec\nrounds: 148"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "079e789bda56ed748e4e0af1b2d96806de5507ae",
          "message": "docs: auto-generate API/SDK reference, tested examples, CI coverage guard (#537)\n\nRestructures documentation so it cannot drift from code (issue #40),\nre-scoped for the post Python-server reality (ADR-017): generate from\nthe proto contract + both SDK surfaces, not the deleted Python server.\n\n- scripts/generate_api_docs.py: extracts the 44-RPC gRPC contract from\n  proto/entdb/v1/entdb.proto, the entdb_sdk public surface + DbClient\n  methods via AST, and the Go SDK surface via 'go doc -all', writing\n  docs/generated/{api-reference,sdk-python,sdk-go}.md. Has a --check\n  mode so stale generated docs fail CI.\n- scripts/check_docs_coverage.py: fails if any public RPC or SDK symbol\n  is undocumented, and if the generated files are stale. Reuses the\n  generator's extractors so the two can't disagree on the surface.\n- examples/: four runnable, CI-tested examples (quickstart, ACL\n  sharing, graph edges, read-after-write) plus a shared harness that\n  boots the Go entdb-server with the contract seed profile. Each file\n  is both a standalone script and a pytest case; example_schema.proto\n  mirrors the server's contract registry so the high-level SDK works.\n- CI wiring: new docs-coverage job in ci.yml regenerates docs, fails\n  on staleness, runs the coverage guard, and executes the example\n  suite end-to-end against the Go server on PRs; docs.yml regenerates\n  the reference from source before the Astro build.\n\nDeferred (issue Layer 3, blocked on the Refraction-UI npm package):\nmoving Astro page content into docs/*.md thin wrappers.",
          "timestamp": "2026-05-17T11:49:42+01:00",
          "tree_id": "f5c627a76c365f58d1d7ee77e217cad504a3e20f",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/079e789bda56ed748e4e0af1b2d96806de5507ae"
        },
        "date": 1779015456926,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3298.470616859764,
            "unit": "iter/sec",
            "range": "stddev: 0.000029464435716400017",
            "extra": "mean: 303.17080737012225 usec\nrounds: 1194"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2178.8305773814,
            "unit": "iter/sec",
            "range": "stddev: 0.000038340768069395705",
            "extra": "mean: 458.9617983064279 usec\nrounds: 1299"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1101.737291218274,
            "unit": "iter/sec",
            "range": "stddev: 0.00008581805110534234",
            "extra": "mean: 907.6573952527508 usec\nrounds: 969"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 843.5352640037833,
            "unit": "iter/sec",
            "range": "stddev: 0.00008270286236549025",
            "extra": "mean: 1.1854868938775214 msec\nrounds: 735"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1948.4480067107436,
            "unit": "iter/sec",
            "range": "stddev: 0.00007102025643044752",
            "extra": "mean: 513.2289886904099 usec\nrounds: 1680"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1929.1052055300627,
            "unit": "iter/sec",
            "range": "stddev: 0.00009199337610933583",
            "extra": "mean: 518.3750461785876 usec\nrounds: 1884"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1989.3338000804554,
            "unit": "iter/sec",
            "range": "stddev: 0.00007111728560194329",
            "extra": "mean: 502.68084720601263 usec\nrounds: 1754"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1997.746985028451,
            "unit": "iter/sec",
            "range": "stddev: 0.00004145840308953087",
            "extra": "mean: 500.5638889680309 usec\nrounds: 1405"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1651.6923226089712,
            "unit": "iter/sec",
            "range": "stddev: 0.000071599911503395",
            "extra": "mean: 605.4396368570785 usec\nrounds: 369"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1420.3016753973716,
            "unit": "iter/sec",
            "range": "stddev: 0.0000724996487016432",
            "extra": "mean: 704.0757730009861 usec\nrounds: 1163"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2541.089854136154,
            "unit": "iter/sec",
            "range": "stddev: 0.00005440159309113384",
            "extra": "mean: 393.5319321244353 usec\nrounds: 1930"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 151.60318878403254,
            "unit": "iter/sec",
            "range": "stddev: 0.0006912393908496358",
            "extra": "mean: 6.596167323528778 msec\nrounds: 136"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "aaca588b836b6a71efb1095e388477d8d258ce87",
          "message": "auth: argon2id API-key hashing with persistent, rotatable keys (#542)\n\nMemoryAPIKeyManager hashed keys with SHA-256 and was in-memory only:\nno persistence, no rotation/multiple-active-keys migration window, and\nnot wired into entdb-server.\n\n- Hash API keys with argon2id (golang.org/x/crypto/argon2) in a\n  PHC-format string with a per-key random salt. Verification derives\n  the key and compares it with subtle.ConstantTimeCompare, so the\n  constant-time guarantee is preserved. PHC params are embedded per\n  hash so the defaults can be raised later without invalidating old\n  keys.\n- Add an api_keys table to global.db (hash, scopes, status, created_at,\n  expires_at, revoked_at) with CRUD: Put/Get/List/ListActive/Revoke/\n  Delete. Table is CREATE TABLE IF NOT EXISTS so existing global.db\n  files upgrade non-destructively.\n- Add PersistentAPIKeyManager: durable, scopeable, revocable, and\n  rotatable. Multiple keys can be active at once for a documented\n  migration window — issue the new key, flip clients over, then revoke\n  the old key_id; ListActiveAPIKeys returns every still-valid key so\n  the cutover never sees a gap. The manager depends on an APIKeyStore\n  interface so auth never imports globalstore.\n- Wire an --api-key-auth flag in entdb-server that installs the\n  API-key interceptor backed by global.db (opt-in; default off, so the\n  contract/e2e harnesses are unaffected). A globalStoreAPIKeys adapter\n  in main bridges the two packages without a cross-import.\n\nTests: argon2 round-trip + malformed-hash rejection, salt randomness,\npersistence round-trip + reopen durability, rotation overlap (memory,\npersistent, and end-to-end through global.db), scope enforcement,\nrevocation idempotency, expiry, and store-failure handling.\n\nCloses #87",
          "timestamp": "2026-05-17T12:49:54+01:00",
          "tree_id": "dcbd74ecaaab817ca5b0259f4572a7f8c5e23d4f",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/aaca588b836b6a71efb1095e388477d8d258ce87"
        },
        "date": 1779018685829,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3933.498822115957,
            "unit": "iter/sec",
            "range": "stddev: 0.000030859311765142725",
            "extra": "mean: 254.22659195358997 usec\nrounds: 1218"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2528.0878626272493,
            "unit": "iter/sec",
            "range": "stddev: 0.000042281962671386905",
            "extra": "mean: 395.55587239787474 usec\nrounds: 1489"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1173.894505157144,
            "unit": "iter/sec",
            "range": "stddev: 0.0003236316314308931",
            "extra": "mean: 851.8653044262563 usec\nrounds: 1107"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 922.4492462572231,
            "unit": "iter/sec",
            "range": "stddev: 0.00007843732634135095",
            "extra": "mean: 1.0840704830725745 msec\nrounds: 768"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2173.3828795577424,
            "unit": "iter/sec",
            "range": "stddev: 0.00006571331805202222",
            "extra": "mean: 460.1122100508531 usec\nrounds: 1771"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2156.8094308042887,
            "unit": "iter/sec",
            "range": "stddev: 0.00007167435993337925",
            "extra": "mean: 463.64782428973956 usec\nrounds: 1935"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2128.3221572866846,
            "unit": "iter/sec",
            "range": "stddev: 0.0001073083346531845",
            "extra": "mean: 469.85368102113887 usec\nrounds: 1881"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2233.3116104492515,
            "unit": "iter/sec",
            "range": "stddev: 0.00008332740909944887",
            "extra": "mean: 447.7655492951298 usec\nrounds: 426"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1954.1179396889945,
            "unit": "iter/sec",
            "range": "stddev: 0.000026168817859595617",
            "extra": "mean: 511.7398390801089 usec\nrounds: 261"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1650.6945074667997,
            "unit": "iter/sec",
            "range": "stddev: 0.0000876205457798237",
            "extra": "mean: 605.8056142287812 usec\nrounds: 1265"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 3075.630007011782,
            "unit": "iter/sec",
            "range": "stddev: 0.00003872023025555318",
            "extra": "mean: 325.1366379311598 usec\nrounds: 2030"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 152.00877987511905,
            "unit": "iter/sec",
            "range": "stddev: 0.00023238697980722115",
            "extra": "mean: 6.578567374999902 msec\nrounds: 152"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "90039ce7cac04962bf6bd3e4ac7e9cde83f9bc1e",
          "message": "Production session management: per-user cap, request-checked revocation, pluggable store (#529)\n\nMemorySessionManager had TTL, revocation and lazy expiry but explicitly\nlacked a per-user concurrent-session cap and a revocation list checked\non every request, and wasn't production-shaped.\n\nAdds:\n\n- Per-user concurrent-session cap (configurable; 0 = unlimited).\n  Reject-new on overflow with a ResourceExhausted error rather than\n  evicting the oldest session -- silent oldest-eviction would let\n  anyone who can mint sessions push a legitimate user off their own\n  login with no signal. Expired/revoked sessions are pruned during the\n  count so stale rows never permanently consume a user's quota.\n\n- Revocation list consulted on EVERY Validate (before the session\n  lookup and independent of TTL), so Revoke / RevokeUser take effect\n  on the next request. Revocation deliberately outlives the session\n  row: a token revoked then lazily evicted on expiry stays rejected as\n  revoked on replay, not downgraded to \"invalid\".\n\n- RevokeUser for \"log out everywhere\" / forced sign-out.\n\n- Pluggable SessionStore interface owning the session table and the\n  revocation set. In-memory implementation is the default; the\n  interface is the seam for a Redis adapter (or stateless signed\n  tokens) for multi-replica deployments -- mapping documented on the\n  interface. NewSessionManager is the full constructor;\n  NewMemorySessionManager kept for the existing unlimited/in-memory\n  call sites.\n\nAdds resourceExhaustedf error wrapper. Interceptor session-path\ncomment updated to record the per-request revocation guarantee; doc.go\nupdated. No behaviour change for existing callers\n(NewMemorySessionManager == unlimited cap + default store).\n\nTests: cap enforcement (reject-new, per-user isolation, revoke frees a\nslot, expired doesn't count, unlimited), immediate revocation on next\nrequest, revocation outliving the entry, RevokeUser, and a counting\nstore proving all persistence flows through the SessionStore seam.\n\nAdvances Epic #85.\n\nCloses #88",
          "timestamp": "2026-05-17T12:50:07+01:00",
          "tree_id": "3ef1972c2c167c6c81668bda7cb7759018be2a78",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/90039ce7cac04962bf6bd3e4ac7e9cde83f9bc1e"
        },
        "date": 1779018703165,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3350.284824108171,
            "unit": "iter/sec",
            "range": "stddev: 0.000024809062740941426",
            "extra": "mean: 298.48208510635953 usec\nrounds: 1363"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2200.347800296806,
            "unit": "iter/sec",
            "range": "stddev: 0.00003984250241343013",
            "extra": "mean: 454.47360633855686 usec\nrounds: 1199"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1108.0158320587377,
            "unit": "iter/sec",
            "range": "stddev: 0.00007978212812517971",
            "extra": "mean: 902.5141799119964 usec\nrounds: 906"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 793.8706861200353,
            "unit": "iter/sec",
            "range": "stddev: 0.00008337795621791838",
            "extra": "mean: 1.2596509954126174 msec\nrounds: 654"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1936.0503417803718,
            "unit": "iter/sec",
            "range": "stddev: 0.00007677646214661969",
            "extra": "mean: 516.5154946747977 usec\nrounds: 1690"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1941.9201438097812,
            "unit": "iter/sec",
            "range": "stddev: 0.00008593117295126824",
            "extra": "mean: 514.9542339254677 usec\nrounds: 1633"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1946.884406977346,
            "unit": "iter/sec",
            "range": "stddev: 0.00008622063987730739",
            "extra": "mean: 513.6411778820292 usec\nrounds: 1917"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1980.7520013275257,
            "unit": "iter/sec",
            "range": "stddev: 0.00007547460133184873",
            "extra": "mean: 504.85876037474003 usec\nrounds: 1494"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1702.4244226225521,
            "unit": "iter/sec",
            "range": "stddev: 0.000022858513711000422",
            "extra": "mean: 587.3975882345011 usec\nrounds: 357"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1447.5622342829195,
            "unit": "iter/sec",
            "range": "stddev: 0.00007877081692081243",
            "extra": "mean: 690.8165855096179 usec\nrounds: 1187"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2601.3612277287407,
            "unit": "iter/sec",
            "range": "stddev: 0.0000388917293004567",
            "extra": "mean: 384.41412493608357 usec\nrounds: 1953"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 154.56091676337448,
            "unit": "iter/sec",
            "range": "stddev: 0.00007762472775845895",
            "extra": "mean: 6.469940919999544 msec\nrounds: 125"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "92b13846901f39e4bd63420443139ecff27e828d",
          "message": "feat(server,sdk): DeleteWhere op for single-RPC sweeper pattern (#540)\n\nAdds a DeleteWhereOp to the ExecuteAtomic Operation oneof so a\npredicate-based sweep (the TTL-sweeper pattern) is one round trip\ninstead of a QueryNodes + DeleteNode loop.\n\nServer:\n- proto: DeleteWhereOp{type_id, where []FieldFilter, limit};\n  delete_where = 6 in the Operation oneof. Reuses the #501\n  FieldFilter/FilterOp types verbatim — no new wire concept.\n- handler: convertOperations resolves the developer-facing filter\n  field names to stable field ids via the existing QueryNodes\n  name->id path (#501), rejects an empty predicate\n  (INVALID_ARGUMENT — an implicit bulk delete is too dangerous),\n  and marks the op as a delete for the legal-hold gate.\n- applier: ops_delete_where.go compiles the predicate to the shared\n  injection-safe json_extract SQL (factored into\n  store.CompileQueryFilters, reused by QueryNodes), cascades\n  edges/visibility/access/acl_inherit per matched node exactly like\n  delete_node, and runs inside the event BatchTxn so idempotency is\n  the standard whole-batch applied_events memoization (#500).\n- limit is best-effort (Postgres DELETE ... LIMIT semantics) with a\n  server-side hard ceiling so a runaway predicate cannot pin the\n  single applier goroutine for a tenant.\n\nSDKs (shipped together):\n- Go: entdb.DeleteWhere[T](plan, []Filter, limit) free function\n  (type witness, mirrors Delete); wire encoder reuses the QueryWhere\n  Filter->FieldFilter lowering.\n- Python: Plan.delete_where(node_type, where, limit=) with the same\n  class-witness + non-empty-predicate contract; grpc client reuses\n  the query FieldFilter encoder.\n\nRegenerated server + SDK Go + Python proto stubs. Adds a proper\nsdk/go/entdb/buf.gen.yaml (the old protoc go:generate directive\npointed at deleted paths).\n\nTesting:\n- server/go applier: predicate match deletes only matching nodes,\n  idempotent re-apply, best-effort limit, edge cascade, empty\n  predicate is poison.\n- server/go handler: empty predicate / missing type_id rejected,\n  full round trip with name->id resolution.\n- Go SDK: builder + wire conversion.\n- Python SDK: builder unit tests + live-server e2e (sweep, empty\n  rejected, best-effort limit).\n\nCloses #504",
          "timestamp": "2026-05-17T12:50:21+01:00",
          "tree_id": "b76237f935b7caa5fdb63a4bacb26595077dd4fe",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/92b13846901f39e4bd63420443139ecff27e828d"
        },
        "date": 1779018772855,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3018.106087064222,
            "unit": "iter/sec",
            "range": "stddev: 0.000027506121951325442",
            "extra": "mean: 331.3336149070631 usec\nrounds: 1449"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2051.3085693353783,
            "unit": "iter/sec",
            "range": "stddev: 0.00003495611902381685",
            "extra": "mean: 487.49369790035973 usec\nrounds: 1238"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 949.6408155154375,
            "unit": "iter/sec",
            "range": "stddev: 0.00010407231726932593",
            "extra": "mean: 1.0530297178278178 msec\nrounds: 847"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 804.2611133996772,
            "unit": "iter/sec",
            "range": "stddev: 0.00008501361809884802",
            "extra": "mean: 1.2433772854849572 msec\nrounds: 627"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1893.3310958119082,
            "unit": "iter/sec",
            "range": "stddev: 0.00007977601202961892",
            "extra": "mean: 528.1696382698319 usec\nrounds: 1526"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1868.7014335156261,
            "unit": "iter/sec",
            "range": "stddev: 0.00007201372485623611",
            "extra": "mean: 535.1309642432711 usec\nrounds: 1678"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1941.5044322262872,
            "unit": "iter/sec",
            "range": "stddev: 0.0000790310695256385",
            "extra": "mean: 515.0644950386843 usec\nrounds: 1814"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1975.6143219909304,
            "unit": "iter/sec",
            "range": "stddev: 0.000043095725497343625",
            "extra": "mean: 506.1716696770286 usec\nrounds: 1550"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1721.573950589186,
            "unit": "iter/sec",
            "range": "stddev: 0.000048322787521598995",
            "extra": "mean: 580.8638075975551 usec\nrounds: 395"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1478.8034336651745,
            "unit": "iter/sec",
            "range": "stddev: 0.00005223238750563034",
            "extra": "mean: 676.2223952385118 usec\nrounds: 1260"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2507.8188282683595,
            "unit": "iter/sec",
            "range": "stddev: 0.00003339522604688078",
            "extra": "mean: 398.75288785932617 usec\nrounds: 2158"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 140.13797774597802,
            "unit": "iter/sec",
            "range": "stddev: 0.0016621893715394159",
            "extra": "mean: 7.135824393103889 msec\nrounds: 145"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "92b13846901f39e4bd63420443139ecff27e828d",
          "message": "feat(server,sdk): DeleteWhere op for single-RPC sweeper pattern (#540)\n\nAdds a DeleteWhereOp to the ExecuteAtomic Operation oneof so a\npredicate-based sweep (the TTL-sweeper pattern) is one round trip\ninstead of a QueryNodes + DeleteNode loop.\n\nServer:\n- proto: DeleteWhereOp{type_id, where []FieldFilter, limit};\n  delete_where = 6 in the Operation oneof. Reuses the #501\n  FieldFilter/FilterOp types verbatim — no new wire concept.\n- handler: convertOperations resolves the developer-facing filter\n  field names to stable field ids via the existing QueryNodes\n  name->id path (#501), rejects an empty predicate\n  (INVALID_ARGUMENT — an implicit bulk delete is too dangerous),\n  and marks the op as a delete for the legal-hold gate.\n- applier: ops_delete_where.go compiles the predicate to the shared\n  injection-safe json_extract SQL (factored into\n  store.CompileQueryFilters, reused by QueryNodes), cascades\n  edges/visibility/access/acl_inherit per matched node exactly like\n  delete_node, and runs inside the event BatchTxn so idempotency is\n  the standard whole-batch applied_events memoization (#500).\n- limit is best-effort (Postgres DELETE ... LIMIT semantics) with a\n  server-side hard ceiling so a runaway predicate cannot pin the\n  single applier goroutine for a tenant.\n\nSDKs (shipped together):\n- Go: entdb.DeleteWhere[T](plan, []Filter, limit) free function\n  (type witness, mirrors Delete); wire encoder reuses the QueryWhere\n  Filter->FieldFilter lowering.\n- Python: Plan.delete_where(node_type, where, limit=) with the same\n  class-witness + non-empty-predicate contract; grpc client reuses\n  the query FieldFilter encoder.\n\nRegenerated server + SDK Go + Python proto stubs. Adds a proper\nsdk/go/entdb/buf.gen.yaml (the old protoc go:generate directive\npointed at deleted paths).\n\nTesting:\n- server/go applier: predicate match deletes only matching nodes,\n  idempotent re-apply, best-effort limit, edge cascade, empty\n  predicate is poison.\n- server/go handler: empty predicate / missing type_id rejected,\n  full round trip with name->id resolution.\n- Go SDK: builder + wire conversion.\n- Python SDK: builder unit tests + live-server e2e (sweep, empty\n  rejected, best-effort limit).\n\nCloses #504",
          "timestamp": "2026-05-17T12:50:21+01:00",
          "tree_id": "b76237f935b7caa5fdb63a4bacb26595077dd4fe",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/92b13846901f39e4bd63420443139ecff27e828d"
        },
        "date": 1779018799960,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3010.2297975722086,
            "unit": "iter/sec",
            "range": "stddev: 0.000027609656769336493",
            "extra": "mean: 332.20055186700813 usec\nrounds: 964"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2044.4914837250074,
            "unit": "iter/sec",
            "range": "stddev: 0.00003486781323667572",
            "extra": "mean: 489.11918096035674 usec\nrounds: 1271"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 954.196869573811,
            "unit": "iter/sec",
            "range": "stddev: 0.00008527450775278833",
            "extra": "mean: 1.0480017613625652 msec\nrounds: 880"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 795.7216955504883,
            "unit": "iter/sec",
            "range": "stddev: 0.0000824929442155744",
            "extra": "mean: 1.2567207926989974 msec\nrounds: 685"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1893.6982252296266,
            "unit": "iter/sec",
            "range": "stddev: 0.00007024689863046178",
            "extra": "mean: 528.0672425400523 usec\nrounds: 1575"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1863.8133161839064,
            "unit": "iter/sec",
            "range": "stddev: 0.00008343498512006902",
            "extra": "mean: 536.5344218311873 usec\nrounds: 2162"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1946.6068749884487,
            "unit": "iter/sec",
            "range": "stddev: 0.00007178102087061202",
            "extra": "mean: 513.7144088253228 usec\nrounds: 1881"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1966.8584231548302,
            "unit": "iter/sec",
            "range": "stddev: 0.000038216533306492956",
            "extra": "mean: 508.425003156051 usec\nrounds: 1584"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1801.682143753469,
            "unit": "iter/sec",
            "range": "stddev: 0.000050044766470231655",
            "extra": "mean: 555.0368601182262 usec\nrounds: 336"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1533.9846805083087,
            "unit": "iter/sec",
            "range": "stddev: 0.00005391010523445277",
            "extra": "mean: 651.8969926535609 usec\nrounds: 1225"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2451.89665963631,
            "unit": "iter/sec",
            "range": "stddev: 0.000041387632784539904",
            "extra": "mean: 407.84753144870723 usec\nrounds: 2051"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 129.78727109175298,
            "unit": "iter/sec",
            "range": "stddev: 0.001689777242178299",
            "extra": "mean: 7.70491583333354 msec\nrounds: 114"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "52b019f8ad2f432767926cdd5f63c877f7fbb68a",
          "message": "feat(wal): port the 5 remaining cloud-native WAL backends (#539)\n\nADR-005 enumerates 7 swappable WAL backends; only in-memory and\nKafka/Redpanda were ported in Phase 4D. EPIC #518 tracks the rest.\nThis ports the remaining 5 from the retired Python source, each\nsatisfying the existing wal.Producer + wal.Consumer interface so\nbackend selection stays a deployment-config change, not a code change.\n\nBackends (all fully implemented + unit-tested with fakes, no live\ncloud calls):\n\n- Kinesis: PutRecord with explicit hash key for per-shard (per-tenant)\n  order; shard-iterator walk with ExpiredIteratorException recovery\n  from in-memory checkpoint; lossless sequence number carried in a\n  reserved record header (StreamPos.Offset is a best-effort int64\n  since Kinesis sequence numbers exceed int64).\n- SQS FIFO: MessageGroupId = tenant for per-tenant order;\n  MessageDeduplicationId from the WAL idempotency key for server-side\n  dedupe; DeleteMessage on Commit.\n- Pub/Sub: ordering-key delivery for per-tenant order; synchronous\n  apiv1 Pull/Publish/Acknowledge (bounded, unlike the streaming\n  Receive callback); ack id carried on the record so Commit is\n  stateless.\n- Service Bus: session id = tenant for per-tenant order; CompleteMessage\n  on Commit; sequence-number -> handle map in the adapter.\n- Event Hubs: partition-key hashing for per-tenant order; per-partition\n  fan-out poll resuming after an in-memory checkpoint.\n\nShared concerns (idempotency cache, header transport for backends\nwithout a native header map, AWS/gRPC error classification to the WAL\nsentinels) live in cloud_common.go. Each backend preserves the\nordering/offset and ErrConnection/ErrTimeout/ErrSerialization/ErrWal\ncontract documented on the interface and exercised against the\nKafka/in-memory backends.\n\nWired into cmd/entdb-server with backend-specific CLI flags following\nthe existing --wal-* pattern; -wal-backend now accepts\nkinesis|pubsub|sqs|servicebus|eventhubs.\n\nCloses #518",
          "timestamp": "2026-05-17T14:03:29+01:00",
          "tree_id": "16ae3540f8888ef9c0ac1883fc23d2b5e63a2171",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/52b019f8ad2f432767926cdd5f63c877f7fbb68a"
        },
        "date": 1779023120546,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3018.290535081497,
            "unit": "iter/sec",
            "range": "stddev: 0.00003235404051035858",
            "extra": "mean: 331.3133670788253 usec\nrounds: 1373"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2041.0556070985401,
            "unit": "iter/sec",
            "range": "stddev: 0.00004029115261435327",
            "extra": "mean: 489.94255547086675 usec\nrounds: 1289"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 952.236386379248,
            "unit": "iter/sec",
            "range": "stddev: 0.00009420685602776634",
            "extra": "mean: 1.050159408214138 msec\nrounds: 779"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 800.112651155419,
            "unit": "iter/sec",
            "range": "stddev: 0.00008834689309633334",
            "extra": "mean: 1.2498240073518765 msec\nrounds: 680"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1883.3737149594856,
            "unit": "iter/sec",
            "range": "stddev: 0.00008955527922283099",
            "extra": "mean: 530.9620666663661 usec\nrounds: 1635"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1897.2839017260367,
            "unit": "iter/sec",
            "range": "stddev: 0.00008904138301392989",
            "extra": "mean: 527.069248355641 usec\nrounds: 1824"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1972.5338719325553,
            "unit": "iter/sec",
            "range": "stddev: 0.00008257247047096143",
            "extra": "mean: 506.9621435804637 usec\nrounds: 1581"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1924.4638961569626,
            "unit": "iter/sec",
            "range": "stddev: 0.0000466903860695475",
            "extra": "mean: 519.6252327710274 usec\nrounds: 1538"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1594.5972235142342,
            "unit": "iter/sec",
            "range": "stddev: 0.0000986153561296963",
            "extra": "mean: 627.1176101737854 usec\nrounds: 236"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1368.071740941727,
            "unit": "iter/sec",
            "range": "stddev: 0.00007401062968870669",
            "extra": "mean: 730.955819109047 usec\nrounds: 1078"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2434.9599682193016,
            "unit": "iter/sec",
            "range": "stddev: 0.00004094255629917106",
            "extra": "mean: 410.6843697850627 usec\nrounds: 1774"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 124.01590280725677,
            "unit": "iter/sec",
            "range": "stddev: 0.0017635961949941017",
            "extra": "mean: 8.06348199999948 msec\nrounds: 105"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "52b019f8ad2f432767926cdd5f63c877f7fbb68a",
          "message": "feat(wal): port the 5 remaining cloud-native WAL backends (#539)\n\nADR-005 enumerates 7 swappable WAL backends; only in-memory and\nKafka/Redpanda were ported in Phase 4D. EPIC #518 tracks the rest.\nThis ports the remaining 5 from the retired Python source, each\nsatisfying the existing wal.Producer + wal.Consumer interface so\nbackend selection stays a deployment-config change, not a code change.\n\nBackends (all fully implemented + unit-tested with fakes, no live\ncloud calls):\n\n- Kinesis: PutRecord with explicit hash key for per-shard (per-tenant)\n  order; shard-iterator walk with ExpiredIteratorException recovery\n  from in-memory checkpoint; lossless sequence number carried in a\n  reserved record header (StreamPos.Offset is a best-effort int64\n  since Kinesis sequence numbers exceed int64).\n- SQS FIFO: MessageGroupId = tenant for per-tenant order;\n  MessageDeduplicationId from the WAL idempotency key for server-side\n  dedupe; DeleteMessage on Commit.\n- Pub/Sub: ordering-key delivery for per-tenant order; synchronous\n  apiv1 Pull/Publish/Acknowledge (bounded, unlike the streaming\n  Receive callback); ack id carried on the record so Commit is\n  stateless.\n- Service Bus: session id = tenant for per-tenant order; CompleteMessage\n  on Commit; sequence-number -> handle map in the adapter.\n- Event Hubs: partition-key hashing for per-tenant order; per-partition\n  fan-out poll resuming after an in-memory checkpoint.\n\nShared concerns (idempotency cache, header transport for backends\nwithout a native header map, AWS/gRPC error classification to the WAL\nsentinels) live in cloud_common.go. Each backend preserves the\nordering/offset and ErrConnection/ErrTimeout/ErrSerialization/ErrWal\ncontract documented on the interface and exercised against the\nKafka/in-memory backends.\n\nWired into cmd/entdb-server with backend-specific CLI flags following\nthe existing --wal-* pattern; -wal-backend now accepts\nkinesis|pubsub|sqs|servicebus|eventhubs.\n\nCloses #518",
          "timestamp": "2026-05-17T14:03:29+01:00",
          "tree_id": "16ae3540f8888ef9c0ac1883fc23d2b5e63a2171",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/52b019f8ad2f432767926cdd5f63c877f7fbb68a"
        },
        "date": 1779023686521,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3913.0435754757955,
            "unit": "iter/sec",
            "range": "stddev: 0.000028989346997101876",
            "extra": "mean: 255.5555492065809 usec\nrounds: 1575"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2473.8264728685217,
            "unit": "iter/sec",
            "range": "stddev: 0.0000428137344893672",
            "extra": "mean: 404.23207163776993 usec\nrounds: 1368"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1195.1868835429204,
            "unit": "iter/sec",
            "range": "stddev: 0.00008948521684843517",
            "extra": "mean: 836.6892356078044 usec\nrounds: 938"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 917.8780658932342,
            "unit": "iter/sec",
            "range": "stddev: 0.00007070833274573499",
            "extra": "mean: 1.0894693283980468 msec\nrounds: 743"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2120.844224677163,
            "unit": "iter/sec",
            "range": "stddev: 0.00006562449831871567",
            "extra": "mean: 471.5103487396492 usec\nrounds: 1190"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2093.8629460459742,
            "unit": "iter/sec",
            "range": "stddev: 0.00006534138152136583",
            "extra": "mean: 477.5861772082017 usec\nrounds: 1834"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2184.451545679,
            "unit": "iter/sec",
            "range": "stddev: 0.00006557359320614251",
            "extra": "mean: 457.7808109216572 usec\nrounds: 1703"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2388.40985401994,
            "unit": "iter/sec",
            "range": "stddev: 0.0000488345038371874",
            "extra": "mean: 418.6886092087155 usec\nrounds: 1694"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 2173.1698338555293,
            "unit": "iter/sec",
            "range": "stddev: 0.000032967129938043674",
            "extra": "mean: 460.157316939123 usec\nrounds: 366"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1776.9433913991784,
            "unit": "iter/sec",
            "range": "stddev: 0.00006670866686847385",
            "extra": "mean: 562.7641290320411 usec\nrounds: 1426"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 3089.4751438117746,
            "unit": "iter/sec",
            "range": "stddev: 0.000035326251284555596",
            "extra": "mean: 323.6795745073406 usec\nrounds: 2181"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 189.53933578636008,
            "unit": "iter/sec",
            "range": "stddev: 0.0000980597184259813",
            "extra": "mean: 5.275949690607513 msec\nrounds: 181"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "102ec1cc0068cdc0e37abcd96c1e725441090dda",
          "message": "docs: ADR-026 (read/write conn split) + ADR-027 (parallel WAL apply) (#544)\n\n* docs: ADR-026 (read/write conn split) + ADR-027 (parallel WAL apply)\n\nRecords the design decisions and blocking conditions from the deep\nreview of PR #536 (#137) and PR #541 (#140). ADR-027 amends ADR-016's\nno-fan-out-within-the-consumer clause. Neither implementation merges\nuntil the conditions in each ADR are met. Adds the two index pointers\nto CLAUDE.md per the ADR-019 convention.\n\n* docs: ADR-026/027 reflect dark-landing decision\n\nBoth perf changes land disabled by default: read/write split off\n(--read-pool-size=1) pending idle-tenant eviction; parallel apply\nserial (--apply-concurrency=1) pending staging soak + the\nmulti-replica rebalance open question. Conditions are met; the ADRs\nand CLAUDE.md pointers now state the actual rollout posture instead of\n'gated, do not merge'.",
          "timestamp": "2026-05-17T14:57:40+01:00",
          "tree_id": "13b1f20f1d1f0256102d6ea38cc3ad58c62f1754",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/102ec1cc0068cdc0e37abcd96c1e725441090dda"
        },
        "date": 1779026370163,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3003.9194333963965,
            "unit": "iter/sec",
            "range": "stddev: 0.000029741972100500633",
            "extra": "mean: 332.8984089527811 usec\nrounds: 1318"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2023.1381140987135,
            "unit": "iter/sec",
            "range": "stddev: 0.000045496018254616385",
            "extra": "mean: 494.28162765125376 usec\nrounds: 1273"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 932.9923840370531,
            "unit": "iter/sec",
            "range": "stddev: 0.00010796134885454579",
            "extra": "mean: 1.071820110334669 msec\nrounds: 716"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 796.5251151827805,
            "unit": "iter/sec",
            "range": "stddev: 0.00007973386982851418",
            "extra": "mean: 1.2554531940534326 msec\nrounds: 639"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1851.725222940096,
            "unit": "iter/sec",
            "range": "stddev: 0.00008967853074323081",
            "extra": "mean: 540.0369275157572 usec\nrounds: 1421"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1843.8821732977322,
            "unit": "iter/sec",
            "range": "stddev: 0.00009985045803815974",
            "extra": "mean: 542.3340029431098 usec\nrounds: 1359"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1908.6168667400825,
            "unit": "iter/sec",
            "range": "stddev: 0.00008179714289610591",
            "extra": "mean: 523.9396221558075 usec\nrounds: 1670"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1950.0073222421925,
            "unit": "iter/sec",
            "range": "stddev: 0.00003890008505165065",
            "extra": "mean: 512.8185871887712 usec\nrounds: 1405"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1680.950615645499,
            "unit": "iter/sec",
            "range": "stddev: 0.000050833960289030646",
            "extra": "mean: 594.9014746135131 usec\nrounds: 453"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1452.131692757898,
            "unit": "iter/sec",
            "range": "stddev: 0.00004915023619903207",
            "extra": "mean: 688.6427759873442 usec\nrounds: 1241"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2478.194512564193,
            "unit": "iter/sec",
            "range": "stddev: 0.000037970034716027325",
            "extra": "mean: 403.51957642150455 usec\nrounds: 1917"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 143.49490269092996,
            "unit": "iter/sec",
            "range": "stddev: 0.001477183834764813",
            "extra": "mean: 6.968888659089687 msec\nrounds: 132"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "8eb78e3309b3f0f483508c0784bf6b02822d2b0b",
          "message": "perf(store): split per-tenant read connections from the write connection (#536)\n\nPer-tenant *sql.DB used SetMaxOpenConns(1), so every same-tenant query\n(reads included) serialized onto a single connection. 50 parallel\nGetNode calls for one tenant ran strictly one at a time regardless of\nWAL mode (issue #137 / canonical-store.md open question 5). Design and\nblocking conditions: ADR-026.\n\nOpen a second, read-only pooled handle per tenant (mode=ro +\nPRAGMA query_only, SetMaxOpenConns(N)) and route the pure SELECT\nmethods (GetNode, QueryNodes, ACL/visibility/edges/fts/offset/\nidempotency reads) to it. The write/DDL handle is unchanged: still\nSetMaxOpenConns(1) + writeMu + BEGIN IMMEDIATE, so ADR-016\n(applier-only writes, single-writer) and write serialization are\npreserved verbatim. The read handle is opened SQLITE_OPEN_READONLY so\nit physically cannot write — concurrent readers cannot race\nINSERT-OR-REPLACE because they cannot write at all.\n\nThe split is ON BY DEFAULT (--read-pool-size=8): it is opt-OUT, not\nopt-in. Set --read-pool-size=0/1 to disable and revert to the single\nshared connection (exact pre-#137 behaviour). The split also requires\nWAL mode; with the rollback journal it falls back to the single write\nhandle regardless of pool size.\n\nADR-026 condition 1 (root-cause read-after-write fix). The split\nremoved an implicit connection-serialisation that was masking a\npre-existing latent race: UpdateAppliedOffsetTx broadcast offsetCond\n(waking WaitForOffset(N) waiters) BEFORE BatchTxn.Commit. With the\nsplit on, a woken reader runs on an independent read connection and its\nWAL snapshot can exclude the still-uncommitted write — the client reads\nits own confirmed write back as stale / Found=false. This is the\ndefault Python-SDK after_offset read-your-write path. Fix: the\napplied_offsets row is still written inside the applier's BatchTxn\n(atomic with the data), but the in-memory tracker bump + offsetCond\nbroadcast is deferred to AFTER the SQL COMMIT (offset.go stashes a\npending notify on the BatchTxn; txn.go BatchTxn.Commit publishes it via\nnotifyOffset post-COMMIT; Rollback / no-op commit publish nothing). The\nnon-txn / global apply path (UpdateAppliedOffset, used by\napply/global.go) already commits the offset row before notifying and is\npreserved. This also closes the pre-existing latent bug.\n\nADR-026 condition 2 (regression test).\nTestReadSplitWaitForOffsetObservesAppliedWrite drives a create_node\nthrough the real WAL->applier path with the split on (ReadPoolSize=8),\nfences a re-routed GetNode via WaitForOffset under concurrent apply,\nand asserts the write is observed. A test-only preCommitHook seam\n(export_test.go; nil in production) makes the broadcast->commit window\ndeterministic. Verified: fails on the pre-fix code (WaitForOffset\nreturns but the read-pool GetNode returns NotFound), passes with the\ncondition-1 fix.\n\nADR-026 condition 3 (narrative). Flag help text and code comments\n(pool.go, canonical.go, main.go) corrected: the split is on by default\n/ opt-out, and read-your-writes via bare WaitForOffset is weakened by\nthe split and then re-fixed by condition 1 — not \"preserved\".\n\nCorrectness: the applier reads its own uncommitted writes exclusively\nthrough tx.Conn() (the BEGIN IMMEDIATE write connection) — no store\nSELECT method observes in-flight writes. Handler-side ACL pre-flight\nand WaitForOffset reads see committed state because, with condition 1,\nthe offset broadcast fires only after COMMIT makes the data visible on\nevery connection including the read pool.\n\nBenchmark (Apple M-series, same-tenant parallel GetNode):\n  serialized-readpool=1   ~10.4 us/op\n  split-readpool=8         ~3.7 us/op   (~2.7x throughput)\n\nAdds BenchmarkSameTenantParallelReads plus concurrent read/write\ncorrectness tests (race-clean). Full Go suite (+ -race on store/apply),\nGo SDK, Python integration (101), and Docker e2e (22) all green.\n\nCloses #137",
          "timestamp": "2026-05-17T15:06:09+01:00",
          "tree_id": "b958e907dc11867c4cfc30da417c5023b826754a",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/8eb78e3309b3f0f483508c0784bf6b02822d2b0b"
        },
        "date": 1779026873559,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3801.163372818775,
            "unit": "iter/sec",
            "range": "stddev: 0.000027648210219799356",
            "extra": "mean: 263.07735340994935 usec\nrounds: 1129"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2419.418580750265,
            "unit": "iter/sec",
            "range": "stddev: 0.00004148602297681804",
            "extra": "mean: 413.3224436467288 usec\nrounds: 1393"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1152.664351544615,
            "unit": "iter/sec",
            "range": "stddev: 0.00009191191283669532",
            "extra": "mean: 867.5552416104142 usec\nrounds: 894"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 887.0861983137557,
            "unit": "iter/sec",
            "range": "stddev: 0.00011093999648956991",
            "extra": "mean: 1.1272861666666438 msec\nrounds: 726"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1952.8498555197907,
            "unit": "iter/sec",
            "range": "stddev: 0.00014028405788982592",
            "extra": "mean: 512.0721376369356 usec\nrounds: 1642"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2069.1203451708266,
            "unit": "iter/sec",
            "range": "stddev: 0.00011184447274025862",
            "extra": "mean: 483.2971665151937 usec\nrounds: 1099"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2034.344192096308,
            "unit": "iter/sec",
            "range": "stddev: 0.00012220629997318656",
            "extra": "mean: 491.5589032992206 usec\nrounds: 1758"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2309.010337177926,
            "unit": "iter/sec",
            "range": "stddev: 0.00005770517366244131",
            "extra": "mean: 433.08597796153686 usec\nrounds: 1452"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 2023.0784440886544,
            "unit": "iter/sec",
            "range": "stddev: 0.00004179711661589479",
            "extra": "mean: 494.2962063196095 usec\nrounds: 538"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1665.5040101638367,
            "unit": "iter/sec",
            "range": "stddev: 0.00006248074751678061",
            "extra": "mean: 600.418848527197 usec\nrounds: 1426"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2996.627946069684,
            "unit": "iter/sec",
            "range": "stddev: 0.00003583533929608733",
            "extra": "mean: 333.70842760496157 usec\nrounds: 2217"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 185.0937164997562,
            "unit": "iter/sec",
            "range": "stddev: 0.00014559395225923044",
            "extra": "mean: 5.402668544943919 msec\nrounds: 178"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "9273ead68e0a2815a5b9517c6d9605793f54f995",
          "message": "perf(apply): parallel WAL apply across distinct tenants (#140 PERF-4) (#541)\n\nThe applier was a single goroutine applying WAL records strictly\nserially regardless of tenant, so apply latency scaled with the\nnumber of distinct tenants in a poll window even though per-tenant\nSQLite files are independent (ADR-001, ADR-014).\n\nWithin a poll batch, records are now partitioned by tenant route key\n(scope + \"\\x00\" + tenant_id) and applied in parallel across distinct\nkeys, one worker goroutine per key, bounded by --apply-concurrency.\nThis amends ADR-016's \"no fan-out within the consumer\" clause exactly\nas ADR-027 records it. Invariants preserved:\n\n- Per-tenant ordering: a route key's records apply serially in offset\n  order on a single worker; two records for one tenant never run\n  concurrently.\n- Single-writer-per-tenant (ADR-016): workers only ever touch their\n  own tenant's DB; distinct tenants own independent SQLite files.\n- Gap-free monotonic offset commit: offsets are committed only in the\n  serial finalizeBatch loop, strictly in batch order, returning at the\n  first poisoned record. A faster later-tenant worker never advances\n  the consumer-group offset past an earlier un-applied record.\n\nHalt-on-poison semantics are unchanged: finalisation stops committing\nat the first failure in batch order; the speculative tail is left\nuncommitted and re-delivered after restart, where the in-txn\nidempotency probe SKIPs already-applied records (ADR-027 accepted\nconsequence).\n\nADR-027 blocking conditions:\n- cond 2: --apply-concurrency flag wired in cmd/entdb-server/main.go\n  (0 = runtime.GOMAXPROCS; 1 = strictly serial / pre-#140 behaviour)\n  so operators have a no-redeploy kill-switch; passed into\n  apply.Options.MaxApplyConcurrency.\n- cond 3: TestParallelApply_SamePartitionPoisonContiguousOffset —\n  two tenants forced onto the SAME WAL partition (collision asserted\n  at runtime via wal.InMemory.PartitionFor) with a tenant-B poison\n  sandwiched between two tenant-A records; asserts gap-free\n  contiguous-prefix commit and idempotent resume on the same group.\n- recommended #6: a load-bearing comment at the finalizeBatch commit\n  site stating that committing offsets from anywhere other than this\n  serial loop breaks the gap-free invariant.\n\nHonest benchmark (independently reproduced): ~2-3.7x at 256-1024\ntenant batches. The earlier \"~2.1x at 64 tenants\" claim did NOT\nreproduce (~1.0-1.3x, within noise) and is dropped per ADR-027.\n\nTests: per-tenant ordering under concurrency, single-writer proof\n(strict update-chain + race detector + serial-vs-parallel\nequivalence), same-partition and multi-partition poison\ncontiguous-offset gap-free commit, serial/parallel equivalence. Full\nGo suite + race detector green.",
          "timestamp": "2026-05-17T15:12:09+01:00",
          "tree_id": "20031841be74ab3518b81adbb375e95edc81e811",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/9273ead68e0a2815a5b9517c6d9605793f54f995"
        },
        "date": 1779027238475,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3050.4018378544597,
            "unit": "iter/sec",
            "range": "stddev: 0.00002793240761773283",
            "extra": "mean: 327.82566139002955 usec\nrounds: 1453"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2069.997957465307,
            "unit": "iter/sec",
            "range": "stddev: 0.00003743264437249788",
            "extra": "mean: 483.0922641220819 usec\nrounds: 1310"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 981.439255056284,
            "unit": "iter/sec",
            "range": "stddev: 0.00009075619318960556",
            "extra": "mean: 1.018911761322051 msec\nrounds: 817"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 812.0247295828543,
            "unit": "iter/sec",
            "range": "stddev: 0.00007205110392018571",
            "extra": "mean: 1.231489588394322 msec\nrounds: 741"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1929.659594003056,
            "unit": "iter/sec",
            "range": "stddev: 0.00007608388839267808",
            "extra": "mean: 518.2261177607558 usec\nrounds: 1554"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1898.8584378310989,
            "unit": "iter/sec",
            "range": "stddev: 0.00007771093897776417",
            "extra": "mean: 526.6322017886774 usec\nrounds: 1789"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1966.0375306305962,
            "unit": "iter/sec",
            "range": "stddev: 0.00008763228452709095",
            "extra": "mean: 508.6372891769036 usec\nrounds: 1774"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1972.4907545909361,
            "unit": "iter/sec",
            "range": "stddev: 0.00004045085335284098",
            "extra": "mean: 506.9732254371881 usec\nrounds: 1486"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1814.778726975287,
            "unit": "iter/sec",
            "range": "stddev: 0.000055170038404280246",
            "extra": "mean: 551.031365496945 usec\nrounds: 342"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1543.1344304398742,
            "unit": "iter/sec",
            "range": "stddev: 0.00005459847271535617",
            "extra": "mean: 648.0316816694625 usec\nrounds: 1222"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2463.4125131615533,
            "unit": "iter/sec",
            "range": "stddev: 0.000042299638427874344",
            "extra": "mean: 405.9409435720516 usec\nrounds: 2038"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 126.4426751822343,
            "unit": "iter/sec",
            "range": "stddev: 0.001985468309067924",
            "extra": "mean: 7.908722261362784 msec\nrounds: 88"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "9273ead68e0a2815a5b9517c6d9605793f54f995",
          "message": "perf(apply): parallel WAL apply across distinct tenants (#140 PERF-4) (#541)\n\nThe applier was a single goroutine applying WAL records strictly\nserially regardless of tenant, so apply latency scaled with the\nnumber of distinct tenants in a poll window even though per-tenant\nSQLite files are independent (ADR-001, ADR-014).\n\nWithin a poll batch, records are now partitioned by tenant route key\n(scope + \"\\x00\" + tenant_id) and applied in parallel across distinct\nkeys, one worker goroutine per key, bounded by --apply-concurrency.\nThis amends ADR-016's \"no fan-out within the consumer\" clause exactly\nas ADR-027 records it. Invariants preserved:\n\n- Per-tenant ordering: a route key's records apply serially in offset\n  order on a single worker; two records for one tenant never run\n  concurrently.\n- Single-writer-per-tenant (ADR-016): workers only ever touch their\n  own tenant's DB; distinct tenants own independent SQLite files.\n- Gap-free monotonic offset commit: offsets are committed only in the\n  serial finalizeBatch loop, strictly in batch order, returning at the\n  first poisoned record. A faster later-tenant worker never advances\n  the consumer-group offset past an earlier un-applied record.\n\nHalt-on-poison semantics are unchanged: finalisation stops committing\nat the first failure in batch order; the speculative tail is left\nuncommitted and re-delivered after restart, where the in-txn\nidempotency probe SKIPs already-applied records (ADR-027 accepted\nconsequence).\n\nADR-027 blocking conditions:\n- cond 2: --apply-concurrency flag wired in cmd/entdb-server/main.go\n  (0 = runtime.GOMAXPROCS; 1 = strictly serial / pre-#140 behaviour)\n  so operators have a no-redeploy kill-switch; passed into\n  apply.Options.MaxApplyConcurrency.\n- cond 3: TestParallelApply_SamePartitionPoisonContiguousOffset —\n  two tenants forced onto the SAME WAL partition (collision asserted\n  at runtime via wal.InMemory.PartitionFor) with a tenant-B poison\n  sandwiched between two tenant-A records; asserts gap-free\n  contiguous-prefix commit and idempotent resume on the same group.\n- recommended #6: a load-bearing comment at the finalizeBatch commit\n  site stating that committing offsets from anywhere other than this\n  serial loop breaks the gap-free invariant.\n\nHonest benchmark (independently reproduced): ~2-3.7x at 256-1024\ntenant batches. The earlier \"~2.1x at 64 tenants\" claim did NOT\nreproduce (~1.0-1.3x, within noise) and is dropped per ADR-027.\n\nTests: per-tenant ordering under concurrency, single-writer proof\n(strict update-chain + race detector + serial-vs-parallel\nequivalence), same-partition and multi-partition poison\ncontiguous-offset gap-free commit, serial/parallel equivalence. Full\nGo suite + race detector green.",
          "timestamp": "2026-05-17T15:12:09+01:00",
          "tree_id": "20031841be74ab3518b81adbb375e95edc81e811",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/9273ead68e0a2815a5b9517c6d9605793f54f995"
        },
        "date": 1779028140852,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3074.75436225123,
            "unit": "iter/sec",
            "range": "stddev: 0.00002468332823359966",
            "extra": "mean: 325.2292320573648 usec\nrounds: 836"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2085.6015251331855,
            "unit": "iter/sec",
            "range": "stddev: 0.000037894725361876805",
            "extra": "mean: 479.47797695254394 usec\nrounds: 781"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 986.5324982165862,
            "unit": "iter/sec",
            "range": "stddev: 0.00008517089432087959",
            "extra": "mean: 1.0136513513825038 msec\nrounds: 868"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 811.3162425723041,
            "unit": "iter/sec",
            "range": "stddev: 0.00008262982779343127",
            "extra": "mean: 1.232564994421248 msec\nrounds: 717"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1893.879866973595,
            "unit": "iter/sec",
            "range": "stddev: 0.00008664215216875389",
            "extra": "mean: 528.0165956872397 usec\nrounds: 1484"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1891.822643195714,
            "unit": "iter/sec",
            "range": "stddev: 0.00009403867694560745",
            "extra": "mean: 528.590776517388 usec\nrounds: 1763"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1946.9955870532694,
            "unit": "iter/sec",
            "range": "stddev: 0.00007742401991281316",
            "extra": "mean: 513.6118472222506 usec\nrounds: 1800"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2003.137538909933,
            "unit": "iter/sec",
            "range": "stddev: 0.000038241801844161255",
            "extra": "mean: 499.21684386394145 usec\nrounds: 1646"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1843.6493099146087,
            "unit": "iter/sec",
            "range": "stddev: 0.000022118484827152113",
            "extra": "mean: 542.402502809125 usec\nrounds: 356"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1560.174627194528,
            "unit": "iter/sec",
            "range": "stddev: 0.00005538525801701089",
            "extra": "mean: 640.9538923204886 usec\nrounds: 1263"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2529.9526417584575,
            "unit": "iter/sec",
            "range": "stddev: 0.000027002302881645837",
            "extra": "mean: 395.2643158193446 usec\nrounds: 1770"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 158.1085721883799,
            "unit": "iter/sec",
            "range": "stddev: 0.00020190272373644619",
            "extra": "mean: 6.324767760273876 msec\nrounds: 146"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "c95c5658c7182d13dfb2219173570a9683d76e62",
          "message": "fix: DeleteWhere usable in schemaless mode via numeric field id (#546)\n\nA server started without a schema cannot resolve a FieldFilter.field\nNAME to a payload field id, so entdb.DeleteWhere failed with\n\"VALIDATION_ERROR: payload: cannot translate filter key ... without a\nschema\". The pre-v1.14.0 QueryNodes path already worked schemaless\nbecause callers pass the numeric field id as the filter key; DeleteWhere\nhad no documented or tested escape hatch.\n\nThe DeleteWhere handler is in fact already schema-optional, exactly like\nQueryNodes: when the registry is nil it skips the registry block and\nroutes through fieldFiltersToStoreFilters -> payload.FilterNamesToIDs,\nwhose schemaless branch resolves digit-only keys and returns a clear\nINVALID_ARGUMENT for genuine name keys. Both SDKs already forward\nFilter.Field verbatim, so Filter{Field: \"4\"} reaches the wire\nunchanged. The real gap was that this contract was undocumented and\nunpinned, so callers did not know the numeric-id route was supported and\nnothing guarded it against regression.\n\n- server: spell out the schema-optional contract on the DeleteWhere\n  handler so it is not \"fixed\" into a hard reject that would diverge\n  from QueryNodes.\n- Go + Python SDKs: document the numeric-field-id escape hatch on\n  DeleteWhere and Filter; no API change (the field already suffices,\n  consistent with ADR-025's single-shape API).\n- tests: server unit tests for schemaless numeric (works) / schemaless\n  name (clear INVALID_ARGUMENT) / schema-mode unchanged; Go + Python SDK\n  tests proving the numeric id travels verbatim; live schemaless\n  integration coverage in tests/python/integration/test_delete_where.py.\n\nCloses #545",
          "timestamp": "2026-05-17T18:09:38+01:00",
          "tree_id": "85ed1611294ce7229d8801ab752c67bc19655b7f",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/c95c5658c7182d13dfb2219173570a9683d76e62"
        },
        "date": 1779037886631,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3165.218261139389,
            "unit": "iter/sec",
            "range": "stddev: 0.000028300309681022857",
            "extra": "mean: 315.93397911208444 usec\nrounds: 1532"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2131.211560567495,
            "unit": "iter/sec",
            "range": "stddev: 0.000035736035636735084",
            "extra": "mean: 469.2166739813113 usec\nrounds: 1276"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1007.2612028286865,
            "unit": "iter/sec",
            "range": "stddev: 0.00008295329660254609",
            "extra": "mean: 992.7911421503233 usec\nrounds: 809"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 814.5635257331526,
            "unit": "iter/sec",
            "range": "stddev: 0.00007844064568357639",
            "extra": "mean: 1.2276513352349583 msec\nrounds: 701"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1962.8613417268382,
            "unit": "iter/sec",
            "range": "stddev: 0.00007113403208593165",
            "extra": "mean: 509.46033667372774 usec\nrounds: 1497"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1939.9352018583675,
            "unit": "iter/sec",
            "range": "stddev: 0.0000690267195951921",
            "extra": "mean: 515.481135164745 usec\nrounds: 1820"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2026.377403114149,
            "unit": "iter/sec",
            "range": "stddev: 0.00007098557988138575",
            "extra": "mean: 493.49148804324113 usec\nrounds: 1840"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2037.4190569431105,
            "unit": "iter/sec",
            "range": "stddev: 0.000045328525218761126",
            "extra": "mean: 490.81704453102225 usec\nrounds: 1280"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1774.9832014068168,
            "unit": "iter/sec",
            "range": "stddev: 0.000055702967563335025",
            "extra": "mean: 563.385613569424 usec\nrounds: 339"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1530.785096107569,
            "unit": "iter/sec",
            "range": "stddev: 0.00006303826384414053",
            "extra": "mean: 653.2595610858557 usec\nrounds: 1326"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2609.2280969054304,
            "unit": "iter/sec",
            "range": "stddev: 0.000026739317404771424",
            "extra": "mean: 383.2551095038451 usec\nrounds: 1936"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 151.28958608399296,
            "unit": "iter/sec",
            "range": "stddev: 0.0010510458669694695",
            "extra": "mean: 6.609840279719055 msec\nrounds: 143"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "349de12d11cd8fc7040c23bdf5a181eaa43e333f",
          "message": "docs: document DeleteWhere + schema-less numeric-field-id requirement (#545) (#547)\n\nDocs-only change. No server or SDK behaviour is modified.\n\nDeleteWhere (the single-RPC predicate sweeper shipped in v1.14.0 via\n#504) and its schema-less numeric-field-id requirement (#545) were\nabsent from the user-facing docs; #546 only added code docstrings.\n\nRegenerated reference (docs/generated/):\n- Re-ran scripts/generate_api_docs.py — the reference was stale across\n  v1.14.0–v1.16.0. sdk-go.md now carries DeleteWhere (the Go free\n  function) plus the rest of the post-v1.14 Go SDK surface.\n\nClosed a #40 coverage-guard blind spot:\n- DeleteWhere is not a top-level RPC (it is an Operation-oneof member\n  on ExecuteAtomic) and delete_where is a Plan builder method, not a\n  DbClient method. The generator extracted only RPCs + DbClient\n  methods, and the guard enforced only those — so delete_where could\n  go undocumented undetected. The generator now also extracts the\n  Operation oneof and Plan public methods; the guard enforces both.\n  Verified the guard fails if delete_where regresses out of either\n  generated doc. Guard passes: 44 RPCs + 6 ExecuteAtomic ops, 46\n  public symbols + 50 DbClient/Plan methods.\n\nHand-written narrative:\n- docs/guides/schema-lockdown.md — primary home: dedicated section on\n  delete_where / QueryNodes filters in schema-less deployments, with\n  Go and Python numeric-field-id examples.\n- docs/api-reference.md — ExecuteAtomic operations table incl. the\n  schema-less caveat.\n- docs/sdk-reference.md — delete_where row + sweeper subsection in\n  Plan methods.\n- docs/getting-started.md — concise bulk-cleanup pointer.",
          "timestamp": "2026-05-17T19:13:09+01:00",
          "tree_id": "f5c2ea5386a02d4be778e3120267e0c6dc7f4151",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/349de12d11cd8fc7040c23bdf5a181eaa43e333f"
        },
        "date": 1779041685356,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 4318.232464209115,
            "unit": "iter/sec",
            "range": "stddev: 0.000021492276664788644",
            "extra": "mean: 231.5762313141588 usec\nrounds: 1552"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2884.0947314272703,
            "unit": "iter/sec",
            "range": "stddev: 0.00003272508371055134",
            "extra": "mean: 346.7292489054698 usec\nrounds: 1599"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1498.9014004693388,
            "unit": "iter/sec",
            "range": "stddev: 0.00006589179524762517",
            "extra": "mean: 667.1552909930421 usec\nrounds: 1299"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 1072.2901534153743,
            "unit": "iter/sec",
            "range": "stddev: 0.00005993335836621713",
            "extra": "mean: 932.5834027430715 usec\nrounds: 802"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2805.189291966366,
            "unit": "iter/sec",
            "range": "stddev: 0.00003855323759736751",
            "extra": "mean: 356.4821820986724 usec\nrounds: 972"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2711.668753192825,
            "unit": "iter/sec",
            "range": "stddev: 0.00005174938598340217",
            "extra": "mean: 368.77660622174477 usec\nrounds: 2443"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2717.9197717027596,
            "unit": "iter/sec",
            "range": "stddev: 0.00006126035512493332",
            "extra": "mean: 367.92844675231396 usec\nrounds: 2263"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 376.33776859894516,
            "unit": "iter/sec",
            "range": "stddev: 0.02096798758602176",
            "extra": "mean: 2.6571874614734132 msec\nrounds: 1181"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 382.3351292668751,
            "unit": "iter/sec",
            "range": "stddev: 0.019355378344745145",
            "extra": "mean: 2.6155064587381576 msec\nrounds: 412"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 622.8233488326401,
            "unit": "iter/sec",
            "range": "stddev: 0.012689594252253642",
            "extra": "mean: 1.6055917008800382 msec\nrounds: 1023"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 3411.865410744044,
            "unit": "iter/sec",
            "range": "stddev: 0.0000470193808348981",
            "extra": "mean: 293.0947970136737 usec\nrounds: 2478"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 209.68918928465698,
            "unit": "iter/sec",
            "range": "stddev: 0.00022089616360975063",
            "extra": "mean: 4.768963070587684 msec\nrounds: 170"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "17fee3937e4f923960c8659b377345ec01a1ca8b",
          "message": "fix(wal): Service Bus backend uses session receivers for per-tenant ordering (#548)\n\nThe Azure Service Bus WAL backend (shipped v1.15.0) created a\nnon-session receiver via Client.NewReceiverForQueue. The producer\nstamps every message with SessionId = tenant key, so a session-enabled\nqueue requires a *session* receiver: a plain receiver fails at runtime\n(\"entity requires sessions\") and cannot preserve per-session FIFO\n(= per-tenant ordering). The old unit fake had no session model, so\nthis shipped undetected.\n\nReshape the ServiceBusAPI seam to be session-scoped: the only consume\nentry point is AcceptNextSession, which returns a serviceBusSession\n(a per-session receiver) backed by Client.AcceptNextSessionForQueue.\nThere is no queue-wide Receive, so a non-session receiver is\nstructurally unrepresentable — a *azservicebus.Receiver cannot satisfy\nserviceBusSession, so reverting the bug fails to compile.\n\nConsumer semantics preserved against the wal.Consumer/Producer\ncontract the other backends honour:\n- Per-session FIFO == per-tenant order; cross-session order is not\n  promised (Service Bus doesn't, the WAL contract only needs per-key).\n- Fair rotation: accept-next loop services each tenant in turn,\n  releasing an idle/empty session lock so no tenant starves; a\n  visited set within a poll cycle prevents spinning on one tenant.\n- At-least-once: a received-but-uncommitted message stays locked\n  until Commit; on lock loss it redelivers and the applier dedupes.\n- Commit == Complete on the owning session's link (settlement is\n  link-scoped); once a session's in-flight set is fully settled the\n  lock is released so the queue stays fully drainable.\n- Connected/Close lifecycle and ErrConnection-before-Connect behaviour\n  unchanged; error classification now also maps *azservicebus.Error\n  codes (Timeout/ConnectionLost/Unauthorized/NotFound/Closed) before\n  the string-sniff fallback.\n\nThe fake now models sessions: no queue-wide read path, per-session\nFIFO, session locks, and an explicit lock-expiry seam for redelivery.\nNew/changed tests: multi-session (multi-tenant) FIFO, redelivery-\nbefore-commit then commit-stops-redelivery, commit-stops-redelivery\nacross lock expiry, accept-session error classification, not-connected\nguards for every consumer entry point, and an explicit session-based\nreceiver regression guard.\n\nAlso documents (ADR-005) the in-memory-checkpoint restart re-replay\nwindow and non-unique StreamPos-within-a-ms on the cloud backends.\nThe pubsub/v2 indirect-dep cleanup is deferred (go mod tidy reports no\nchurn; it is a legitimately-required transitive dep and removing it\nwould need code changes outside this fix's scope).\n\nCloses #543",
          "timestamp": "2026-05-17T20:00:50+01:00",
          "tree_id": "42b73d7abf7c6167149b8b3fad24d3cfaa6d92d4",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/17fee3937e4f923960c8659b377345ec01a1ca8b"
        },
        "date": 1779044554275,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3353.482931905069,
            "unit": "iter/sec",
            "range": "stddev: 0.000029976899188474502",
            "extra": "mean: 298.19743243240936 usec\nrounds: 1369"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2187.8995699348366,
            "unit": "iter/sec",
            "range": "stddev: 0.00003400578154518175",
            "extra": "mean: 457.05937043069287 usec\nrounds: 1231"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1106.4586979028431,
            "unit": "iter/sec",
            "range": "stddev: 0.00007946247273739894",
            "extra": "mean: 903.7843002141675 usec\nrounds: 936"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 810.521441340633,
            "unit": "iter/sec",
            "range": "stddev: 0.00007690580972642095",
            "extra": "mean: 1.233773653595101 msec\nrounds: 612"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1922.9846556604405,
            "unit": "iter/sec",
            "range": "stddev: 0.00007697747924244453",
            "extra": "mean: 520.024950306509 usec\nrounds: 1469"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1944.1964730251889,
            "unit": "iter/sec",
            "range": "stddev: 0.00009177294831881905",
            "extra": "mean: 514.3513085609038 usec\nrounds: 1682"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1976.9555401751854,
            "unit": "iter/sec",
            "range": "stddev: 0.00008580778251551927",
            "extra": "mean: 505.8282696187423 usec\nrounds: 1784"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2053.2857531900845,
            "unit": "iter/sec",
            "range": "stddev: 0.00003873184393280589",
            "extra": "mean: 487.02427241135405 usec\nrounds: 1439"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1922.007829779981,
            "unit": "iter/sec",
            "range": "stddev: 0.0000480269624863532",
            "extra": "mean: 520.2892436262727 usec\nrounds: 353"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1629.7477845038632,
            "unit": "iter/sec",
            "range": "stddev: 0.0000455609207547471",
            "extra": "mean: 613.5918756928548 usec\nrounds: 1263"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2592.201270835849,
            "unit": "iter/sec",
            "range": "stddev: 0.0000343854729844874",
            "extra": "mean: 385.7725135971222 usec\nrounds: 1949"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 144.36190857822308,
            "unit": "iter/sec",
            "range": "stddev: 0.001511637905736464",
            "extra": "mean: 6.9270350458005066 msec\nrounds: 131"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "f46b7705941a25ca6db4751891289f3ad2435456",
          "message": "feat(audit): durable queue + worker to lift S3 Object Lock legal hold on tenant release (#550)\n\nCloses #511\n\nWhen a tenant is under legal hold the archiver escalates archive writes\nto LegalHold=ON. Releasing the hold must then clear that hold on the\nobjects already written to S3, otherwise GDPR right-to-erasure-after-\nrelease is silently unfulfillable and the objects need manual S3 surgery.\n\nProblem with the first cut: the lift ran as a detached fire-and-forget\ngoroutine launched from the SetLegalHold OFF RPC path (30-min timeout).\nIt was not crash-durable — a server restart, S3 outage, or a run past\nthe timeout left a released tenant's objects stuck LegalHold=ON with no\nautomatic retry and no signal. Unacceptable for a compliance feature.\n\nRework to a durable queue + retrying worker, mirroring the GDPR\ndeletion-queue worker:\n\n- Durable enqueue. ApplyLegalHoldSet (the WAL-driven global apply step\n  that clears the legal_holds row and flips tenant_registry.status) now,\n  in that SAME transaction, upserts a row into a new\n  legal_hold_lift_queue table on an explicit release. A crash after the\n  release still has the pending lift recorded. The OFF RPC no longer\n  spawns anything — it just causes the enqueue via the apply path and\n  returns. The detached goroutine, its timeout const, the\n  WithLegalHoldLifter server seam and the main.go adapter are removed.\n\n- Background worker. internal/audit.LiftWorker drains the queue on an\n  interval (-legal-hold-lift-worker-enabled, default true;\n  -legal-hold-lift-worker-interval, default 1m), running the existing\n  idempotent/resumable paginated sweep per queued tenant. The queue row\n  is deleted only on full success; any failure / partial / per-run\n  timeout leaves the row so the next tick resumes. Only does work when\n  the archive sidecar is enabled.\n\n- Observability. New metrics close the no-signal gap:\n  entdb_legal_hold_lift_pending (gauge, queue depth),\n  entdb_legal_hold_lift_completed_total, entdb_legal_hold_lift_errors_total.\n\nSweep correctness is byte-for-byte preserved: per-partition shared-object\nsafety (a co-tenant still held keeps the object ON), COMPLIANCE\nretain-until never touched, GDPR never lifts a hold (legal hold\nsupersedes erasure; DeleteUser still refuses to queue while held), MinIO\nContent-MD5 middleware kept. Only the trigger + execution model changed.\n\nTests:\n\n- Durability (audit.TestLiftWorker_DurableRestart): a pre-populated\n  queue + a FRESH worker with zero in-memory state completes the lift\n  purely from the persisted queue + live S3 — models a crash/restart\n  after the release committed.\n- No goroutine (api.TestSetLegalHold_Release_SpawnsNoGoroutine): the OFF\n  RPC does not grow the live goroutine count and instead records the\n  lift durably in the queue.\n- Retry (audit.TestLiftWorker_RetryAfterTransientS3Error): a transient\n  S3 List error on tick 1 retains the queue row and bumps the error\n  metric; tick 2 succeeds from the same row and removes it.\n- Metrics (audit.TestLiftWorker_Metrics): pending gauge tracks queue\n  depth, completed counter advances per swept tenant.\n- Atomic enqueue (globalstore.TestApplyLegalHoldSet_*): release enqueues\n  in the same txn that clears the hold; ON never enqueues; re-release\n  keeps the earliest enqueued_at; queue CRUD round-trip.\n- All existing sweep correctness tests retained and passing\n  (shared-object, idempotent re-run, retention-untouched,\n  other-tenant-untouched, pagination, list-error, enable-no-trigger).\n- Archive e2e extended: release -> durable worker sweep (not a request\n  goroutine) -> LegalHold OFF while ObjectLockMode=COMPLIANCE and\n  retain-until unchanged; entdb_legal_hold_lift_pending exported and\n  drains to 0.\n\nADR-015 Gap-1 consequence updated to the durable design; deployment.md\nIAM comment updated (IAM actions unchanged).",
          "timestamp": "2026-05-18T13:12:45+01:00",
          "tree_id": "0549f2a3e7d02061ff1e3de252dba1f8ce83aa48",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/f46b7705941a25ca6db4751891289f3ad2435456"
        },
        "date": 1779106475629,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3047.7273010533195,
            "unit": "iter/sec",
            "range": "stddev: 0.00004177712623637992",
            "extra": "mean: 328.11334519804046 usec\nrounds: 1489"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2051.499789220617,
            "unit": "iter/sec",
            "range": "stddev: 0.00004560082527700696",
            "extra": "mean: 487.44825871023306 usec\nrounds: 1349"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 989.9327519047852,
            "unit": "iter/sec",
            "range": "stddev: 0.00010012440347842532",
            "extra": "mean: 1.0101696282660046 msec\nrounds: 842"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 787.0778305742443,
            "unit": "iter/sec",
            "range": "stddev: 0.00008068866349718243",
            "extra": "mean: 1.2705223818467986 msec\nrounds: 639"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1936.3322725086082,
            "unit": "iter/sec",
            "range": "stddev: 0.00007629785806443761",
            "extra": "mean: 516.440289818882 usec\nrounds: 1601"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1933.621022413567,
            "unit": "iter/sec",
            "range": "stddev: 0.00007213032446352869",
            "extra": "mean: 517.1644228152779 usec\nrounds: 1911"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2021.2225312136616,
            "unit": "iter/sec",
            "range": "stddev: 0.00007243882584924564",
            "extra": "mean: 494.7500755394513 usec\nrounds: 1668"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2012.865590499273,
            "unit": "iter/sec",
            "range": "stddev: 0.00004506529543719195",
            "extra": "mean: 496.80416055597584 usec\nrounds: 791"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1870.985717596795,
            "unit": "iter/sec",
            "range": "stddev: 0.000021963016206887252",
            "extra": "mean: 534.4776235301567 usec\nrounds: 340"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1587.805860233,
            "unit": "iter/sec",
            "range": "stddev: 0.00006000028178351909",
            "extra": "mean: 629.7999176380774 usec\nrounds: 1287"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2584.7808741859426,
            "unit": "iter/sec",
            "range": "stddev: 0.000029549922043321304",
            "extra": "mean: 386.8799904807956 usec\nrounds: 2101"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 150.1412262066805,
            "unit": "iter/sec",
            "range": "stddev: 0.0002405889202050042",
            "extra": "mean: 6.660395850393722 msec\nrounds: 127"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "f46b7705941a25ca6db4751891289f3ad2435456",
          "message": "feat(audit): durable queue + worker to lift S3 Object Lock legal hold on tenant release (#550)\n\nCloses #511\n\nWhen a tenant is under legal hold the archiver escalates archive writes\nto LegalHold=ON. Releasing the hold must then clear that hold on the\nobjects already written to S3, otherwise GDPR right-to-erasure-after-\nrelease is silently unfulfillable and the objects need manual S3 surgery.\n\nProblem with the first cut: the lift ran as a detached fire-and-forget\ngoroutine launched from the SetLegalHold OFF RPC path (30-min timeout).\nIt was not crash-durable — a server restart, S3 outage, or a run past\nthe timeout left a released tenant's objects stuck LegalHold=ON with no\nautomatic retry and no signal. Unacceptable for a compliance feature.\n\nRework to a durable queue + retrying worker, mirroring the GDPR\ndeletion-queue worker:\n\n- Durable enqueue. ApplyLegalHoldSet (the WAL-driven global apply step\n  that clears the legal_holds row and flips tenant_registry.status) now,\n  in that SAME transaction, upserts a row into a new\n  legal_hold_lift_queue table on an explicit release. A crash after the\n  release still has the pending lift recorded. The OFF RPC no longer\n  spawns anything — it just causes the enqueue via the apply path and\n  returns. The detached goroutine, its timeout const, the\n  WithLegalHoldLifter server seam and the main.go adapter are removed.\n\n- Background worker. internal/audit.LiftWorker drains the queue on an\n  interval (-legal-hold-lift-worker-enabled, default true;\n  -legal-hold-lift-worker-interval, default 1m), running the existing\n  idempotent/resumable paginated sweep per queued tenant. The queue row\n  is deleted only on full success; any failure / partial / per-run\n  timeout leaves the row so the next tick resumes. Only does work when\n  the archive sidecar is enabled.\n\n- Observability. New metrics close the no-signal gap:\n  entdb_legal_hold_lift_pending (gauge, queue depth),\n  entdb_legal_hold_lift_completed_total, entdb_legal_hold_lift_errors_total.\n\nSweep correctness is byte-for-byte preserved: per-partition shared-object\nsafety (a co-tenant still held keeps the object ON), COMPLIANCE\nretain-until never touched, GDPR never lifts a hold (legal hold\nsupersedes erasure; DeleteUser still refuses to queue while held), MinIO\nContent-MD5 middleware kept. Only the trigger + execution model changed.\n\nTests:\n\n- Durability (audit.TestLiftWorker_DurableRestart): a pre-populated\n  queue + a FRESH worker with zero in-memory state completes the lift\n  purely from the persisted queue + live S3 — models a crash/restart\n  after the release committed.\n- No goroutine (api.TestSetLegalHold_Release_SpawnsNoGoroutine): the OFF\n  RPC does not grow the live goroutine count and instead records the\n  lift durably in the queue.\n- Retry (audit.TestLiftWorker_RetryAfterTransientS3Error): a transient\n  S3 List error on tick 1 retains the queue row and bumps the error\n  metric; tick 2 succeeds from the same row and removes it.\n- Metrics (audit.TestLiftWorker_Metrics): pending gauge tracks queue\n  depth, completed counter advances per swept tenant.\n- Atomic enqueue (globalstore.TestApplyLegalHoldSet_*): release enqueues\n  in the same txn that clears the hold; ON never enqueues; re-release\n  keeps the earliest enqueued_at; queue CRUD round-trip.\n- All existing sweep correctness tests retained and passing\n  (shared-object, idempotent re-run, retention-untouched,\n  other-tenant-untouched, pagination, list-error, enable-no-trigger).\n- Archive e2e extended: release -> durable worker sweep (not a request\n  goroutine) -> LegalHold OFF while ObjectLockMode=COMPLIANCE and\n  retain-until unchanged; entdb_legal_hold_lift_pending exported and\n  drains to 0.\n\nADR-015 Gap-1 consequence updated to the durable design; deployment.md\nIAM comment updated (IAM actions unchanged).",
          "timestamp": "2026-05-18T13:12:45+01:00",
          "tree_id": "0549f2a3e7d02061ff1e3de252dba1f8ce83aa48",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/f46b7705941a25ca6db4751891289f3ad2435456"
        },
        "date": 1779121377077,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 4223.793351531388,
            "unit": "iter/sec",
            "range": "stddev: 0.000021897324436031928",
            "extra": "mean: 236.75400683071717 usec\nrounds: 1464"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2706.806658848972,
            "unit": "iter/sec",
            "range": "stddev: 0.00004175007283189169",
            "extra": "mean: 369.43902023103294 usec\nrounds: 1384"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1466.7360127227362,
            "unit": "iter/sec",
            "range": "stddev: 0.00007913229429177842",
            "extra": "mean: 681.7859460228816 usec\nrounds: 1056"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 1006.4585824901297,
            "unit": "iter/sec",
            "range": "stddev: 0.00006328764716662117",
            "extra": "mean: 993.5828631177745 usec\nrounds: 789"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2720.6772454981833,
            "unit": "iter/sec",
            "range": "stddev: 0.00005129926281022067",
            "extra": "mean: 367.55554215578775 usec\nrounds: 1874"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2594.000503516584,
            "unit": "iter/sec",
            "range": "stddev: 0.00006045721756907044",
            "extra": "mean: 385.50493673549386 usec\nrounds: 2371"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2770.1963478753655,
            "unit": "iter/sec",
            "range": "stddev: 0.00005399970648161878",
            "extra": "mean: 360.9852423518505 usec\nrounds: 2092"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 876.797274691899,
            "unit": "iter/sec",
            "range": "stddev: 0.003133040816263012",
            "extra": "mean: 1.1405144939021323 msec\nrounds: 328"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 792.7653080406415,
            "unit": "iter/sec",
            "range": "stddev: 0.003542426665971832",
            "extra": "mean: 1.261407367170934 msec\nrounds: 463"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 831.6493261316715,
            "unit": "iter/sec",
            "range": "stddev: 0.002910296403407527",
            "extra": "mean: 1.2024298806942992 msec\nrounds: 922"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 3415.92621075636,
            "unit": "iter/sec",
            "range": "stddev: 0.00002692998316126653",
            "extra": "mean: 292.7463704722645 usec\nrounds: 2181"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 161.64994511066914,
            "unit": "iter/sec",
            "range": "stddev: 0.0002189049288868196",
            "extra": "mean: 6.1862068639452845 msec\nrounds: 147"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "928aaf23b10d931eeb94f8690fe12cfd386098b8",
          "message": "docs: S3 Object Lock archive bucket requirements + ops runbook (#551)\n\nDocument the operator-facing surface of the S3 Object Lock WAL archive\n(EPIC #511): what the bucket must look like, the exact IAM the server\nneeds, retention vs legal-hold semantics, and the runbook for the archive\n+ legal-hold-lift metrics. EntDB ships no Terraform/IaC module — bucket\nprovisioning is the operator's choice — so this is docs-only, with a\nsingle clearly-labelled aws-cli/IAM example marked \"adapt to your own\ntooling\".\n\n- docs-site compliance/audit.astro: rewrite the stale page (it described\n  the retired Python server's YAML/env-var config and an IAM policy that\n  omitted the legal-hold actions) to the Go server reality — CLI flags,\n  per-partition wal/<topic>/<partition>/... object keys, COMPLIANCE WORM\n  implications, the verified IAM action set, the durable legal-hold-lift\n  worker, GDPR-supersedes-hold, and the lag + lift metric runbooks.\n- docs/operations.md: add an \"S3 Object Lock archive\" section (bucket\n  requirements, IAM table, retention vs legal hold, archive-lag and\n  legal-hold-lift runbooks); refresh the metrics table with the lift\n  gauges and the now-shipped -metrics-addr endpoint; extend the\n  production checklist.\n- docs/deployment.md: add s3:GetBucketObjectLockConfiguration to the\n  archive IAM block and point at the operations runbook + ADR-015.\n\nIAM set verified against server/go/internal/audit/{s3_lock,archiver,\nlegalhold_lift,legalhold_lift_worker}.go: s3:GetBucketObjectLockConfiguration,\ns3:PutObject, s3:PutObjectLegalHold, s3:GetObjectLegalHold, s3:ListBucket,\ns3:GetObject (+ SSE-KMS perms when -archive-kms-key-id is set).\n\nCloses #549",
          "timestamp": "2026-05-20T15:51:09+01:00",
          "tree_id": "b441f016da036747305ee171ebe3e9fa2bea4b8e",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/928aaf23b10d931eeb94f8690fe12cfd386098b8"
        },
        "date": 1779288779894,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 4283.408749617757,
            "unit": "iter/sec",
            "range": "stddev: 0.000023243371817076698",
            "extra": "mean: 233.4589245280965 usec\nrounds: 1325"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2861.6670548717134,
            "unit": "iter/sec",
            "range": "stddev: 0.000028849214609274106",
            "extra": "mean: 349.4466619719426 usec\nrounds: 1562"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1462.2367977367262,
            "unit": "iter/sec",
            "range": "stddev: 0.0000746020103100064",
            "extra": "mean: 683.883760515271 usec\nrounds: 1165"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 1014.7365049230281,
            "unit": "iter/sec",
            "range": "stddev: 0.00006525218680977237",
            "extra": "mean: 985.4775058830214 usec\nrounds: 850"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2740.708780175518,
            "unit": "iter/sec",
            "range": "stddev: 0.0000510139710817256",
            "extra": "mean: 364.86911970850076 usec\nrounds: 2197"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2720.610343573736,
            "unit": "iter/sec",
            "range": "stddev: 0.000053938417131152",
            "extra": "mean: 367.56458063245515 usec\nrounds: 2561"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2706.8639708258847,
            "unit": "iter/sec",
            "range": "stddev: 0.00007373084678862862",
            "extra": "mean: 369.43119816061255 usec\nrounds: 2392"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 411.2762454180193,
            "unit": "iter/sec",
            "range": "stddev: 0.017461546188735987",
            "extra": "mean: 2.431455770035064 msec\nrounds: 1148"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 607.591307774165,
            "unit": "iter/sec",
            "range": "stddev: 0.0070966917933396635",
            "extra": "mean: 1.6458431633319037 msec\nrounds: 600"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 438.67395545796705,
            "unit": "iter/sec",
            "range": "stddev: 0.017615228716661116",
            "extra": "mean: 2.2795973810572354 msec\nrounds: 908"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 3302.4288137227527,
            "unit": "iter/sec",
            "range": "stddev: 0.00005695662105326492",
            "extra": "mean: 302.8074354985786 usec\nrounds: 2124"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 145.43718631082092,
            "unit": "iter/sec",
            "range": "stddev: 0.0002045893445652248",
            "extra": "mean: 6.875820588710036 msec\nrounds: 124"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "95003ebb717a71e5e15b5d4fb76312e76737cd02",
          "message": "Expose SDK client interceptor hooks (#521)",
          "timestamp": "2026-05-20T17:09:04+01:00",
          "tree_id": "e7b37eadc23f7c5882d91d397d819fa4f0cb412e",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/95003ebb717a71e5e15b5d4fb76312e76737cd02"
        },
        "date": 1779293457054,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 4302.149415264806,
            "unit": "iter/sec",
            "range": "stddev: 0.0000213745618666107",
            "extra": "mean: 232.4419501684016 usec\nrounds: 1485"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2861.232841665074,
            "unit": "iter/sec",
            "range": "stddev: 0.00003047290021868346",
            "extra": "mean: 349.4996930826704 usec\nrounds: 1489"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1487.9832230723318,
            "unit": "iter/sec",
            "range": "stddev: 0.00006391257779660079",
            "extra": "mean: 672.0505880000701 usec\nrounds: 1250"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 1020.738432244341,
            "unit": "iter/sec",
            "range": "stddev: 0.00006534259552663773",
            "extra": "mean: 979.6829123022806 usec\nrounds: 821"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2708.7971572960664,
            "unit": "iter/sec",
            "range": "stddev: 0.00006032227667454416",
            "extra": "mean: 369.16754630612667 usec\nrounds: 1922"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2714.7924807189984,
            "unit": "iter/sec",
            "range": "stddev: 0.00004212476484077581",
            "extra": "mean: 368.35228000011085 usec\nrounds: 2550"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2855.0674104973705,
            "unit": "iter/sec",
            "range": "stddev: 0.000035417933977733033",
            "extra": "mean: 350.2544270314773 usec\nrounds: 2412"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1649.2176208627116,
            "unit": "iter/sec",
            "range": "stddev: 0.00018085351688443458",
            "extra": "mean: 606.3481176467764 usec\nrounds: 34"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 241.3855378745616,
            "unit": "iter/sec",
            "range": "stddev: 0.023626429812548814",
            "extra": "mean: 4.142750260869647 msec\nrounds: 46"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 299.27132008135476,
            "unit": "iter/sec",
            "range": "stddev: 0.021251943876132537",
            "extra": "mean: 3.3414494904762577 msec\nrounds: 840"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 3362.4774118911237,
            "unit": "iter/sec",
            "range": "stddev: 0.000041066111879129885",
            "extra": "mean: 297.3997673452266 usec\nrounds: 2407"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 88.05034801769887,
            "unit": "iter/sec",
            "range": "stddev: 0.02055276975261369",
            "extra": "mean: 11.357138529412643 msec\nrounds: 17"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "1b10ba4889bba7402c53662e214fb6a454ea24d5",
          "message": "docs: regenerate sdk-go reference after #521 (interceptor hooks) (#552)\n\nThe committed generated reference from #521 predated other merged SDK\nchanges, leaving docs/generated/sdk-go.md stale vs a fresh regen and the\ndocs-coverage guard red. Regenerated against current main.",
          "timestamp": "2026-05-20T17:10:56+01:00",
          "tree_id": "2ea168e3e7372a6e8d097e897a9fbba2072982ea",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/1b10ba4889bba7402c53662e214fb6a454ea24d5"
        },
        "date": 1779293603408,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3367.4712733971114,
            "unit": "iter/sec",
            "range": "stddev: 0.0000275362917256106",
            "extra": "mean: 296.9587321798288 usec\nrounds: 1445"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2213.995827233167,
            "unit": "iter/sec",
            "range": "stddev: 0.00004055117180712338",
            "extra": "mean: 451.67203465315527 usec\nrounds: 1212"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1130.146908328191,
            "unit": "iter/sec",
            "range": "stddev: 0.00008838668813846487",
            "extra": "mean: 884.8407163979102 usec\nrounds: 744"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 783.9298957052229,
            "unit": "iter/sec",
            "range": "stddev: 0.00008875878773550782",
            "extra": "mean: 1.2756242688007207 msec\nrounds: 625"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1940.6898107798618,
            "unit": "iter/sec",
            "range": "stddev: 0.00007838501175327834",
            "extra": "mean: 515.2806978453461 usec\nrounds: 1764"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1823.261091693296,
            "unit": "iter/sec",
            "range": "stddev: 0.00014556974150411586",
            "extra": "mean: 548.4678001170319 usec\nrounds: 1711"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1772.245377578424,
            "unit": "iter/sec",
            "range": "stddev: 0.00015075733438417932",
            "extra": "mean: 564.2559504747524 usec\nrounds: 1474"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2032.8484219229938,
            "unit": "iter/sec",
            "range": "stddev: 0.00005322566869198114",
            "extra": "mean: 491.9205924138897 usec\nrounds: 1450"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1956.7950265084341,
            "unit": "iter/sec",
            "range": "stddev: 0.000043698088508066735",
            "extra": "mean: 511.03972897167927 usec\nrounds: 428"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1654.7107789986255,
            "unit": "iter/sec",
            "range": "stddev: 0.000040350242325430655",
            "extra": "mean: 604.335218390954 usec\nrounds: 1305"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2658.132722826332,
            "unit": "iter/sec",
            "range": "stddev: 0.000033399001942677346",
            "extra": "mean: 376.2039387321197 usec\nrounds: 1877"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 149.41802993930358,
            "unit": "iter/sec",
            "range": "stddev: 0.0001089984948091857",
            "extra": "mean: 6.692632745902345 msec\nrounds: 122"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "1b10ba4889bba7402c53662e214fb6a454ea24d5",
          "message": "docs: regenerate sdk-go reference after #521 (interceptor hooks) (#552)\n\nThe committed generated reference from #521 predated other merged SDK\nchanges, leaving docs/generated/sdk-go.md stale vs a fresh regen and the\ndocs-coverage guard red. Regenerated against current main.",
          "timestamp": "2026-05-20T17:10:56+01:00",
          "tree_id": "2ea168e3e7372a6e8d097e897a9fbba2072982ea",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/1b10ba4889bba7402c53662e214fb6a454ea24d5"
        },
        "date": 1779293704052,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3643.5622811306694,
            "unit": "iter/sec",
            "range": "stddev: 0.00010619832788181599",
            "extra": "mean: 274.4566780644354 usec\nrounds: 1550"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2410.4874697125297,
            "unit": "iter/sec",
            "range": "stddev: 0.00004165947008687906",
            "extra": "mean: 414.8538470184448 usec\nrounds: 1425"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1128.5745673738443,
            "unit": "iter/sec",
            "range": "stddev: 0.00015483453967777723",
            "extra": "mean: 886.0734850041562 usec\nrounds: 967"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 885.4203419713923,
            "unit": "iter/sec",
            "range": "stddev: 0.0000885477763991389",
            "extra": "mean: 1.1294070766134598 msec\nrounds: 744"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1973.9791823306907,
            "unit": "iter/sec",
            "range": "stddev: 0.00011959988387841598",
            "extra": "mean: 506.5909554422419 usec\nrounds: 1773"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1925.5851364180235,
            "unit": "iter/sec",
            "range": "stddev: 0.0002012398026144044",
            "extra": "mean: 519.3226625441249 usec\nrounds: 1698"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1884.4734824420705,
            "unit": "iter/sec",
            "range": "stddev: 0.0001425036557752418",
            "extra": "mean: 530.6522003716974 usec\nrounds: 1612"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2224.5155285852325,
            "unit": "iter/sec",
            "range": "stddev: 0.00006112462822835487",
            "extra": "mean: 449.5360842169482 usec\nrounds: 1508"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 2159.3623054198933,
            "unit": "iter/sec",
            "range": "stddev: 0.000026955392417833097",
            "extra": "mean: 463.09968340655433 usec\nrounds: 458"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1749.4195475435108,
            "unit": "iter/sec",
            "range": "stddev: 0.000057656332334239404",
            "extra": "mean: 571.6181698119093 usec\nrounds: 636"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2965.6194890135225,
            "unit": "iter/sec",
            "range": "stddev: 0.00005030739060561431",
            "extra": "mean: 337.19767613634 usec\nrounds: 1936"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 158.63156685394443,
            "unit": "iter/sec",
            "range": "stddev: 0.0008594385464526726",
            "extra": "mean: 6.303915543623937 msec\nrounds: 149"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "aa3c9d7a4a2c6248e3098c18d38ca3cbce38d01d",
          "message": "chore(go): gofmt the three pre-existing unformatted files (#556)\n\napikey_persistent.go, error_sanitization_test.go, and apikeys_test.go\ncarried over-aligned end-of-line comments that gofmt rewrites, leaving\n`gofmt -l` dirty on main. Normalize the comment spacing — `gofmt -l` is\nnow clean across server/go. No logic change.",
          "timestamp": "2026-05-22T19:05:59+01:00",
          "tree_id": "5135f2b79564fc5e8752500d97728336d805b101",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/aa3c9d7a4a2c6248e3098c18d38ca3cbce38d01d"
        },
        "date": 1779473268642,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3950.0304499406952,
            "unit": "iter/sec",
            "range": "stddev: 0.00002764433489542929",
            "extra": "mean: 253.16260537055194 usec\nrounds: 1229"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2543.849574473409,
            "unit": "iter/sec",
            "range": "stddev: 0.000044965108276962226",
            "extra": "mean: 393.1050051208337 usec\nrounds: 1367"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1130.6511287012402,
            "unit": "iter/sec",
            "range": "stddev: 0.00012199000098939019",
            "extra": "mean: 884.4461165918466 usec\nrounds: 892"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 905.9855338204006,
            "unit": "iter/sec",
            "range": "stddev: 0.00008329028067035695",
            "extra": "mean: 1.1037703833781483 msec\nrounds: 746"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2083.0888215323785,
            "unit": "iter/sec",
            "range": "stddev: 0.00006808106591409095",
            "extra": "mean: 480.0563421315717 usec\nrounds: 2008"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2129.3374412441603,
            "unit": "iter/sec",
            "range": "stddev: 0.0000804556438381508",
            "extra": "mean: 469.62965128519295 usec\nrounds: 1712"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2166.869427695029,
            "unit": "iter/sec",
            "range": "stddev: 0.00006607096690087306",
            "extra": "mean: 461.49527388170003 usec\nrounds: 1811"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2367.1360377357587,
            "unit": "iter/sec",
            "range": "stddev: 0.00005419672901328088",
            "extra": "mean: 422.45142824851416 usec\nrounds: 1770"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 2221.098736262545,
            "unit": "iter/sec",
            "range": "stddev: 0.000023999261277483486",
            "extra": "mean: 450.22762098487584 usec\nrounds: 467"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1814.9785613963961,
            "unit": "iter/sec",
            "range": "stddev: 0.0000627961466429911",
            "extra": "mean: 550.9706953401294 usec\nrounds: 1395"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 3083.983612015561,
            "unit": "iter/sec",
            "range": "stddev: 0.000038435125245417296",
            "extra": "mean: 324.2559383596861 usec\nrounds: 1963"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 153.84009893814275,
            "unit": "iter/sec",
            "range": "stddev: 0.00013895837207400066",
            "extra": "mean: 6.500255829932143 msec\nrounds: 147"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "6d1bf0669a24871070aaf5ae6c09ea2eaa0a0cb2",
          "message": "chore(go): remove dangling citations to the retired Python server (#554)\n\nThe Python server was retired in EPIC #407 Phase 4D (ADR-017) and its\nsource deleted, but ~640 comments across server/go still cited it by\nfile:line (grpc_server.py, canonical_store.py, server/python/..., plus\nseveral deleted test files). These dangling references point at code\nthat no longer exists and only mislead readers.\n\nStrip the citations while preserving the substantive prose, and drop\nthe \"Ported from the retired Python source\" / \"(mirrors the Python\ndocstring)\" headers on the cloud-native WAL backends. Comment-only —\nno behavior change (go vet, go build, go test, Go SDK, ruff, and the\nPython contract suite all pass).\n\nCitations to files that still exist (tests/python/integration/\ntest_grpc_contract.py, conftest.py, test_e2e.py), docs/ADR references,\nand conceptual \"Python\" notes (no file:line) were left intact.\n\nCloses #553",
          "timestamp": "2026-05-22T19:05:55+01:00",
          "tree_id": "efd4c6aaceb29de8708f7352b513be4db88cf1f3",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/6d1bf0669a24871070aaf5ae6c09ea2eaa0a0cb2"
        },
        "date": 1779473269185,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3084.006467633858,
            "unit": "iter/sec",
            "range": "stddev: 0.00002816782394088625",
            "extra": "mean: 324.2535352940521 usec\nrounds: 1360"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2093.827934275491,
            "unit": "iter/sec",
            "range": "stddev: 0.00003194114852936413",
            "extra": "mean: 477.59416312593106 usec\nrounds: 1318"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 986.8449442994304,
            "unit": "iter/sec",
            "range": "stddev: 0.00008939918700734769",
            "extra": "mean: 1.0133304180930962 msec\nrounds: 818"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 789.8198044082267,
            "unit": "iter/sec",
            "range": "stddev: 0.00008273276852540656",
            "extra": "mean: 1.2661115793991151 msec\nrounds: 699"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1950.5588086959096,
            "unit": "iter/sec",
            "range": "stddev: 0.00007325958721788083",
            "extra": "mean: 512.6735966851329 usec\nrounds: 1629"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1881.575134133355,
            "unit": "iter/sec",
            "range": "stddev: 0.00010836775793699495",
            "extra": "mean: 531.469608552515 usec\nrounds: 2128"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1887.3379680836247,
            "unit": "iter/sec",
            "range": "stddev: 0.00010789262093007506",
            "extra": "mean: 529.846809056348 usec\nrounds: 1325"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1906.5560008435561,
            "unit": "iter/sec",
            "range": "stddev: 0.00009113545256351568",
            "extra": "mean: 524.5059675968342 usec\nrounds: 1111"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1806.1220324276042,
            "unit": "iter/sec",
            "range": "stddev: 0.000028869205184888825",
            "extra": "mean: 553.6724440794858 usec\nrounds: 304"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1502.6462606779046,
            "unit": "iter/sec",
            "range": "stddev: 0.0000860056113258425",
            "extra": "mean: 665.4926220285934 usec\nrounds: 1262"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2519.187355609954,
            "unit": "iter/sec",
            "range": "stddev: 0.00004397609819332899",
            "extra": "mean: 396.9534055389368 usec\nrounds: 2022"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 144.0479020368012,
            "unit": "iter/sec",
            "range": "stddev: 0.00016657552894478453",
            "extra": "mean: 6.942135122138199 msec\nrounds: 131"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "1afe64e0f1b71ed16fcdb52b36fea5f7dce72483",
          "message": "chore(go): drop conceptual Python references and port-era annotations (#557)\n\nWith the Python server retired (ADR-017) and the EPIC #407 Go port\ncomplete, comments across server/go still framed behavior against the\ndead Python implementation (\"Mirrors the Python enum\", \"the Python\nhandler does X\", `Python \"read\"`) and carried port-era tags (\"W2 —\",\n\"W2.03 —\", \"EPIC #407\", \"Phase 4A.2\"). Remove them.\n\nConcrete facts are preserved, re-attributed to the current system\n(e.g. `Python \"read\"` -> `Stored as \"read\"` for the stored\nnode_access.permission values), and behavioral-divergence rationale is\nkept, rephrased to describe the current invariant. Live references\n(tests/python/integration/*, docs/ADR paths, GDPR articles, CLAUDE.md\ninvariants) and the ADR-024 rate-limit \"Phase 1/2/3\" terms are left\nintact. Comment-only — no behavior change (go vet/build/test, Go SDK,\nruff, and the 103-case Python contract suite all pass).\n\nBuilds on the citation cleanup in #554.",
          "timestamp": "2026-05-22T19:07:23+01:00",
          "tree_id": "eaa6fe3c8dca97140b7ae81665ab0b4ddf99d22c",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/1afe64e0f1b71ed16fcdb52b36fea5f7dce72483"
        },
        "date": 1779473407057,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3040.8126092220587,
            "unit": "iter/sec",
            "range": "stddev: 0.000027530017427009355",
            "extra": "mean: 328.8594624237083 usec\nrounds: 1477"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2067.6999349291686,
            "unit": "iter/sec",
            "range": "stddev: 0.0000366499912726184",
            "extra": "mean: 483.62916838523586 usec\nrounds: 1164"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 965.9287704520381,
            "unit": "iter/sec",
            "range": "stddev: 0.000103407997112973",
            "extra": "mean: 1.0352730248753408 msec\nrounds: 603"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 779.8025314302226,
            "unit": "iter/sec",
            "range": "stddev: 0.00008811710396282906",
            "extra": "mean: 1.2823759345406547 msec\nrounds: 718"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1840.6328318917729,
            "unit": "iter/sec",
            "range": "stddev: 0.00010876262839542437",
            "extra": "mean: 543.2914064518865 usec\nrounds: 1705"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1780.2351731911995,
            "unit": "iter/sec",
            "range": "stddev: 0.00013095853156090215",
            "extra": "mean: 561.7235380241523 usec\nrounds: 1407"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1890.846352215692,
            "unit": "iter/sec",
            "range": "stddev: 0.00009916002632309857",
            "extra": "mean: 528.863701076611 usec\nrounds: 1579"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1912.2542059946934,
            "unit": "iter/sec",
            "range": "stddev: 0.00007162023243618813",
            "extra": "mean: 522.9430254958346 usec\nrounds: 1059"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1673.1345081031304,
            "unit": "iter/sec",
            "range": "stddev: 0.00003116054732947399",
            "extra": "mean: 597.6805780748149 usec\nrounds: 301"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1480.8311313068423,
            "unit": "iter/sec",
            "range": "stddev: 0.00005742966997554274",
            "extra": "mean: 675.2964459340438 usec\nrounds: 1193"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2519.727724164052,
            "unit": "iter/sec",
            "range": "stddev: 0.00003353283612533382",
            "extra": "mean: 396.8682768419994 usec\nrounds: 2077"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 156.58073746432103,
            "unit": "iter/sec",
            "range": "stddev: 0.00026304247060027983",
            "extra": "mean: 6.386481608108808 msec\nrounds: 148"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "5d26e7fe615579e38d3c73248e23a937be0821f9",
          "message": "docs: regenerate sdk-go reference with the pinned go 1.25 toolchain (#559)\n\nThe \"Docs Coverage & Examples\" CI gate regenerates docs/generated/ with\ngo 1.25 (pinned in go.mod and across the workflows) and diffs it against\nthe committed files. The committed sdk-go.md had been generated with a\nnewer go whose `go doc` renders the [TenantDetail.Region] doc link\nwithout brackets, so CI flagged it stale on every PR.\n\nRegenerate with go 1.25 so the committed reference matches the pinned\ntoolchain. Generated file only.",
          "timestamp": "2026-05-22T20:03:33+01:00",
          "tree_id": "864b906fe4d7a9e7c4ece19e5a289cf2fda271d6",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/5d26e7fe615579e38d3c73248e23a937be0821f9"
        },
        "date": 1779476724675,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3146.5781256175537,
            "unit": "iter/sec",
            "range": "stddev: 0.000026115911905347905",
            "extra": "mean: 317.8055525965172 usec\nrounds: 1502"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2119.142825609707,
            "unit": "iter/sec",
            "range": "stddev: 0.000038072180510437155",
            "extra": "mean: 471.88891089126383 usec\nrounds: 1212"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 991.1569050166552,
            "unit": "iter/sec",
            "range": "stddev: 0.00008880724399251386",
            "extra": "mean: 1.0089219930150175 msec\nrounds: 859"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 793.6268859725488,
            "unit": "iter/sec",
            "range": "stddev: 0.00007272122591016732",
            "extra": "mean: 1.260037956973385 msec\nrounds: 674"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1955.968378151008,
            "unit": "iter/sec",
            "range": "stddev: 0.0000795802739269622",
            "extra": "mean: 511.2557090239402 usec\nrounds: 1629"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1944.8136770402036,
            "unit": "iter/sec",
            "range": "stddev: 0.00007962042870284903",
            "extra": "mean: 514.188074572723 usec\nrounds: 1931"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2017.751314109715,
            "unit": "iter/sec",
            "range": "stddev: 0.00008667058716558327",
            "extra": "mean: 495.60121359218454 usec\nrounds: 2060"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2030.097575417791,
            "unit": "iter/sec",
            "range": "stddev: 0.000033185323965675685",
            "extra": "mean: 492.5871603950867 usec\nrounds: 1621"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1742.9742702615556,
            "unit": "iter/sec",
            "range": "stddev: 0.00003354534089571594",
            "extra": "mean: 573.7319345798129 usec\nrounds: 321"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1503.839778153533,
            "unit": "iter/sec",
            "range": "stddev: 0.00006155861215879332",
            "extra": "mean: 664.9644560059684 usec\nrounds: 1307"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2597.41607669034,
            "unit": "iter/sec",
            "range": "stddev: 0.00003931934603905144",
            "extra": "mean: 384.9980020429428 usec\nrounds: 1958"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 149.5180238128856,
            "unit": "iter/sec",
            "range": "stddev: 0.00014304754631441562",
            "extra": "mean: 6.688156882353197 msec\nrounds: 119"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "2985587bf96259833da2a03d31604b8a6fde82f1",
          "message": "test(api): de-flake TestSetLegalHold_Release_SpawnsNoGoroutine (#558)\n\nThe test snapshots runtime.NumGoroutine() before/after the OFF RPC to\nassert no detached lift goroutine is spawned, but it ran with\nt.Parallel(): the process-wide count was polluted by the concurrent\nparallel-test batch, so transient goroutines from unrelated tests\nintermittently failed it in CI (e.g. 297 -> 299).\n\nDrop t.Parallel() so the test runs isolated from the parallel batch,\nand poll for the count to settle back to baseline — a genuinely leaked\ngoroutine persists past the deadline while transient runtime/GC\ngoroutines clear. The durable-queue assertion is unchanged.",
          "timestamp": "2026-05-22T20:06:57+01:00",
          "tree_id": "ddec60a37da80cc9a47075ef3cb5f2c21c0a344d",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/2985587bf96259833da2a03d31604b8a6fde82f1"
        },
        "date": 1779476926830,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3925.4089199730834,
            "unit": "iter/sec",
            "range": "stddev: 0.000032601012062185454",
            "extra": "mean: 254.75052927908897 usec\nrounds: 1332"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2496.7077043895056,
            "unit": "iter/sec",
            "range": "stddev: 0.00003953585846915142",
            "extra": "mean: 400.52746192190716 usec\nrounds: 1405"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1184.3535513390696,
            "unit": "iter/sec",
            "range": "stddev: 0.00009410592249926854",
            "extra": "mean: 844.3424675590888 usec\nrounds: 971"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 906.1834693196165,
            "unit": "iter/sec",
            "range": "stddev: 0.00008079900726957017",
            "extra": "mean: 1.1035292894393927 msec\nrounds: 767"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2093.9106449926303,
            "unit": "iter/sec",
            "range": "stddev: 0.00007253497979794851",
            "extra": "mean: 477.5752978721399 usec\nrounds: 1598"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2128.4786543618548,
            "unit": "iter/sec",
            "range": "stddev: 0.00007638277664444057",
            "extra": "mean: 469.81913487866893 usec\nrounds: 1898"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2162.7394594964176,
            "unit": "iter/sec",
            "range": "stddev: 0.00006576305369702925",
            "extra": "mean: 462.3765454544602 usec\nrounds: 1386"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2387.2572893609763,
            "unit": "iter/sec",
            "range": "stddev: 0.00004255340282150799",
            "extra": "mean: 418.890751514966 usec\nrounds: 1650"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 2201.7806466226284,
            "unit": "iter/sec",
            "range": "stddev: 0.00005519760318280037",
            "extra": "mean: 454.1778498843322 usec\nrounds: 433"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1790.0192325206906,
            "unit": "iter/sec",
            "range": "stddev: 0.00006273025882952558",
            "extra": "mean: 558.6532154695388 usec\nrounds: 1448"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 3100.988882751463,
            "unit": "iter/sec",
            "range": "stddev: 0.00002809284813853948",
            "extra": "mean: 322.47777654485316 usec\nrounds: 1893"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 163.7541472306608,
            "unit": "iter/sec",
            "range": "stddev: 0.0004505397440987716",
            "extra": "mean: 6.1067155666684885 msec\nrounds: 150"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "2ab9934498f1367800e40950fc9962d2701db1c7",
          "message": "fix(api): lazy-open tenant on read RPCs; stop misreporting not-open tenants (#560)\n\nA per-tenant SQLite handle is a materialized view of the WAL (ADR-016);\n\"tenant not opened\" just means the applier hasn't materialized the\ntenant in-process yet, not a client error. Previously QueryNodes\nclobbered it to Internal, and GetEdgesFrom/GetEdgesTo/SearchNodes/\nGetNodes silently returned an empty result, so a valid tenant could be\nreported as empty or as an opaque internal error.\n\nLazy-open the tenant on every per-tenant read RPC (as GetNode already\ndid); genuine open failures (region pin / crypto-shred -> FailedPrecondition,\nIO -> Internal) surface their real typed code. Also wire QueryNodes'\nalready-declared after_offset/wait_timeout_ms read-your-writes fence,\nwhich the handler had been ignoring.\n\nTests rewritten to prove lazy-open reads persisted rows (seed -> close ->\nreopen fresh -> read) rather than asserting the old swallow-to-empty.",
          "timestamp": "2026-05-24T01:56:16+01:00",
          "tree_id": "8b7bfe166631866818d6acb927d5e6d82a9eba54",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/2ab9934498f1367800e40950fc9962d2701db1c7"
        },
        "date": 1779584283057,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3051.842008103216,
            "unit": "iter/sec",
            "range": "stddev: 0.000026521558590359313",
            "extra": "mean: 327.67095981535465 usec\nrounds: 1518"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2059.1753712944187,
            "unit": "iter/sec",
            "range": "stddev: 0.000034372393768990534",
            "extra": "mean: 485.63129393461503 usec\nrounds: 1286"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 941.5877101255691,
            "unit": "iter/sec",
            "range": "stddev: 0.00010768112746944173",
            "extra": "mean: 1.0620359518781752 msec\nrounds: 852"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 684.1602033095992,
            "unit": "iter/sec",
            "range": "stddev: 0.00018947135728703598",
            "extra": "mean: 1.4616459641507027 msec\nrounds: 530"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1774.435569284947,
            "unit": "iter/sec",
            "range": "stddev: 0.0001364025251374962",
            "extra": "mean: 563.5594874842228 usec\nrounds: 1598"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1779.9999358815762,
            "unit": "iter/sec",
            "range": "stddev: 0.00013700285975410908",
            "extra": "mean: 561.7977730458357 usec\nrounds: 1714"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1927.048052110062,
            "unit": "iter/sec",
            "range": "stddev: 0.0001114497073331111",
            "extra": "mean: 518.9284195093261 usec\nrounds: 1671"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1990.0812041721006,
            "unit": "iter/sec",
            "range": "stddev: 0.000044436305768551056",
            "extra": "mean: 502.49205806454154 usec\nrounds: 1550"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1831.423034784082,
            "unit": "iter/sec",
            "range": "stddev: 0.000024959384302356877",
            "extra": "mean: 546.0234915729868 usec\nrounds: 356"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1549.7083235727057,
            "unit": "iter/sec",
            "range": "stddev: 0.00004863157873170304",
            "extra": "mean: 645.2827185535112 usec\nrounds: 1272"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2525.400967430917,
            "unit": "iter/sec",
            "range": "stddev: 0.00003367841913758895",
            "extra": "mean: 395.97672325963237 usec\nrounds: 1767"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 152.5686106095823,
            "unit": "iter/sec",
            "range": "stddev: 0.0001437487977441936",
            "extra": "mean: 6.554428174999671 msec\nrounds: 120"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "2ab9934498f1367800e40950fc9962d2701db1c7",
          "message": "fix(api): lazy-open tenant on read RPCs; stop misreporting not-open tenants (#560)\n\nA per-tenant SQLite handle is a materialized view of the WAL (ADR-016);\n\"tenant not opened\" just means the applier hasn't materialized the\ntenant in-process yet, not a client error. Previously QueryNodes\nclobbered it to Internal, and GetEdgesFrom/GetEdgesTo/SearchNodes/\nGetNodes silently returned an empty result, so a valid tenant could be\nreported as empty or as an opaque internal error.\n\nLazy-open the tenant on every per-tenant read RPC (as GetNode already\ndid); genuine open failures (region pin / crypto-shred -> FailedPrecondition,\nIO -> Internal) surface their real typed code. Also wire QueryNodes'\nalready-declared after_offset/wait_timeout_ms read-your-writes fence,\nwhich the handler had been ignoring.\n\nTests rewritten to prove lazy-open reads persisted rows (seed -> close ->\nreopen fresh -> read) rather than asserting the old swallow-to-empty.",
          "timestamp": "2026-05-24T01:56:16+01:00",
          "tree_id": "8b7bfe166631866818d6acb927d5e6d82a9eba54",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/2ab9934498f1367800e40950fc9962d2701db1c7"
        },
        "date": 1779584503165,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3062.321991196873,
            "unit": "iter/sec",
            "range": "stddev: 0.000027949829788505712",
            "extra": "mean: 326.5495930456227 usec\nrounds: 1553"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2071.979662052507,
            "unit": "iter/sec",
            "range": "stddev: 0.00003955890193424334",
            "extra": "mean: 482.630219936328 usec\nrounds: 1264"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 952.7644517872502,
            "unit": "iter/sec",
            "range": "stddev: 0.0001001309902378523",
            "extra": "mean: 1.0495773620900137 msec\nrounds: 823"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 777.1635875856283,
            "unit": "iter/sec",
            "range": "stddev: 0.00008427208583037209",
            "extra": "mean: 1.2867303820893687 msec\nrounds: 670"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1932.4671079655513,
            "unit": "iter/sec",
            "range": "stddev: 0.00007882077033859205",
            "extra": "mean: 517.4732319520681 usec\nrounds: 1496"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1911.719739234994,
            "unit": "iter/sec",
            "range": "stddev: 0.00008039220633639286",
            "extra": "mean: 523.0892266667531 usec\nrounds: 1650"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2005.0920952445451,
            "unit": "iter/sec",
            "range": "stddev: 0.00007931563816875164",
            "extra": "mean: 498.7302091368716 usec\nrounds: 1970"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1922.1625995127777,
            "unit": "iter/sec",
            "range": "stddev: 0.00005714369643049355",
            "extra": "mean: 520.2473506942003 usec\nrounds: 576"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1782.4084528575813,
            "unit": "iter/sec",
            "range": "stddev: 0.00003164629246339223",
            "extra": "mean: 561.0386319683272 usec\nrounds: 269"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1479.6525243202973,
            "unit": "iter/sec",
            "range": "stddev: 0.00009096618801688412",
            "extra": "mean: 675.834348648421 usec\nrounds: 1110"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2490.081493499806,
            "unit": "iter/sec",
            "range": "stddev: 0.000042329207519849924",
            "extra": "mean: 401.5932822321013 usec\nrounds: 1846"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 133.45176124717295,
            "unit": "iter/sec",
            "range": "stddev: 0.0015948561250124884",
            "extra": "mean: 7.493344341464689 msec\nrounds: 123"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "5f0f0790e121ef882d9e57247a3c668681581689",
          "message": "test(sdk): de-flake test_wall_clock_budget_stops_early teardown (#567)\n\ntime.monotonic was patched with a finite iter([...]); an incidental\nmonotonic() call at teardown (surfaced under xdist) exhausted it and\nraised StopIteration -> RuntimeError (PEP 479), intermittently failing\nthe Unit Tests CI job. Hold the last tick after exhaustion instead.",
          "timestamp": "2026-05-24T02:01:28+01:00",
          "tree_id": "8bb51129d5668d6b7bf59d521ea5121d9ea69735",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/5f0f0790e121ef882d9e57247a3c668681581689"
        },
        "date": 1779584593540,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3333.0570153896338,
            "unit": "iter/sec",
            "range": "stddev: 0.000029342867225613798",
            "extra": "mean: 300.0248706765972 usec\nrounds: 1330"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2197.900938352307,
            "unit": "iter/sec",
            "range": "stddev: 0.000036385308094320304",
            "extra": "mean: 454.9795591559584 usec\nrounds: 1327"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1116.9280751954693,
            "unit": "iter/sec",
            "range": "stddev: 0.00008665151728205573",
            "extra": "mean: 895.3127978495785 usec\nrounds: 930"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 785.094266495085,
            "unit": "iter/sec",
            "range": "stddev: 0.00008040557023349055",
            "extra": "mean: 1.273732394536931 msec\nrounds: 659"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1922.498043805347,
            "unit": "iter/sec",
            "range": "stddev: 0.00008783678785568208",
            "extra": "mean: 520.1565760871329 usec\nrounds: 1380"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1830.325987690172,
            "unit": "iter/sec",
            "range": "stddev: 0.0001438955434145152",
            "extra": "mean: 546.3507630473938 usec\nrounds: 1667"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1804.6486953251363,
            "unit": "iter/sec",
            "range": "stddev: 0.0001429052475173433",
            "extra": "mean: 554.124468984161 usec\nrounds: 1467"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1989.9162866110432,
            "unit": "iter/sec",
            "range": "stddev: 0.0000616164730428458",
            "extra": "mean: 502.5337029142392 usec\nrounds: 1407"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1850.8878375434056,
            "unit": "iter/sec",
            "range": "stddev: 0.00006753251525267422",
            "extra": "mean: 540.2812529835692 usec\nrounds: 419"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1598.2232408838838,
            "unit": "iter/sec",
            "range": "stddev: 0.00006576293072645179",
            "extra": "mean: 625.6948181074869 usec\nrounds: 1226"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2669.1107806608006,
            "unit": "iter/sec",
            "range": "stddev: 0.00003209864499629653",
            "extra": "mean: 374.6566112000892 usec\nrounds: 1875"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 167.41879987527827,
            "unit": "iter/sec",
            "range": "stddev: 0.00016585451639953297",
            "extra": "mean: 5.973044847681195 msec\nrounds: 151"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "6b2547d15f37f1faab69c751adfa79be5325b721",
          "message": "chore: retire Python-server remnants and fix proto regen (#561)\n\nThe historical Python server (ADR-017) left untracked remnants under\nserver/python/ and dbaas/ plus a stale scripts/generate_proto.sh that\ngenerated into the dead server path.\n\n- generate_proto.sh is now Python-SDK-only (drops the dead server stubs)\n- `make proto` regenerates Go (buf: server + SDK) AND Python SDK stubs\n- drop dangling doc-comment citations to the retired Python server",
          "timestamp": "2026-05-24T02:01:56+01:00",
          "tree_id": "0250d70e6f37fa8307928b6fb869dc491ac48a49",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/6b2547d15f37f1faab69c751adfa79be5325b721"
        },
        "date": 1779584638033,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3055.567092477141,
            "unit": "iter/sec",
            "range": "stddev: 0.00003078299571647361",
            "extra": "mean: 327.27149158727923 usec\nrounds: 1367"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2086.07024696128,
            "unit": "iter/sec",
            "range": "stddev: 0.00003614883352457518",
            "extra": "mean: 479.37024242432483 usec\nrounds: 1188"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 980.262231980943,
            "unit": "iter/sec",
            "range": "stddev: 0.00008494065082315913",
            "extra": "mean: 1.0201351917630963 msec\nrounds: 777"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 787.1941022905164,
            "unit": "iter/sec",
            "range": "stddev: 0.00007380202407789197",
            "extra": "mean: 1.2703347206111903 msec\nrounds: 655"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1902.1954009791755,
            "unit": "iter/sec",
            "range": "stddev: 0.0000843480332940476",
            "extra": "mean: 525.7083470421804 usec\nrounds: 1775"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1898.2583014546515,
            "unit": "iter/sec",
            "range": "stddev: 0.00007950253708274572",
            "extra": "mean: 526.798697118138 usec\nrounds: 1839"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1954.5924509938714,
            "unit": "iter/sec",
            "range": "stddev: 0.00008133672172281868",
            "extra": "mean: 511.61560533579257 usec\nrounds: 1799"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1933.9235986640904,
            "unit": "iter/sec",
            "range": "stddev: 0.00009168717678902265",
            "extra": "mean: 517.0835087232903 usec\nrounds: 1433"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1793.3859830559215,
            "unit": "iter/sec",
            "range": "stddev: 0.000031352943603085026",
            "extra": "mean: 557.6044473683265 usec\nrounds: 380"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1501.6017760509558,
            "unit": "iter/sec",
            "range": "stddev: 0.00007121755333930328",
            "extra": "mean: 665.9555255920701 usec\nrounds: 1309"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2480.709569488704,
            "unit": "iter/sec",
            "range": "stddev: 0.00004690908241784901",
            "extra": "mean: 403.11046980244 usec\nrounds: 1871"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 145.20392839010387,
            "unit": "iter/sec",
            "range": "stddev: 0.0002514132959680802",
            "extra": "mean: 6.886866017243053 msec\nrounds: 116"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "3ce88da2cfa7bfe58212b22ffac41b1026fbf264",
          "message": "docs(adr): ADR-028 typed payload values + ADR-029 keyset cursor; land Bug C/A characterization tests (#562)\n\nADR-028 retires google.protobuf.Struct as the payload wire carrier\n(IEEE-754 doubles corrupt int64 >2^53) in favor of a typed,\nfield_id-keyed map<uint32, EntValue>. ADR-029 adopts AIP-158 keyset\ncursor pagination to replace the silent 100-row read truncation.\n\nCharacterization tests are landed RED-but-guarded so CI stays green and\nthey auto-activate when the fix lands:\n- TestPayload_Int64Spectrum_BugC (server, t.Skip)\n- TestPayloadRoundTrip_Int64Spectrum_BugC (Go SDK, t.Skip)\n- test_query_does_not_silently_truncate (Python integration, strict xfail)",
          "timestamp": "2026-05-24T02:07:27+01:00",
          "tree_id": "d23d243501d5c35475441006046200036b4991dc",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/3ce88da2cfa7bfe58212b22ffac41b1026fbf264"
        },
        "date": 1779584955740,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3304.6611316712747,
            "unit": "iter/sec",
            "range": "stddev: 0.000031954027284006136",
            "extra": "mean: 302.6028873024773 usec\nrounds: 1331"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2197.301394708568,
            "unit": "iter/sec",
            "range": "stddev: 0.000034736442093811226",
            "extra": "mean: 455.1037023906463 usec\nrounds: 1213"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1099.3889315821318,
            "unit": "iter/sec",
            "range": "stddev: 0.00009198760566785922",
            "extra": "mean: 909.5962050126328 usec\nrounds: 878"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 780.6605897952028,
            "unit": "iter/sec",
            "range": "stddev: 0.00007845759766062337",
            "extra": "mean: 1.2809664187894234 msec\nrounds: 628"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1926.305997983376,
            "unit": "iter/sec",
            "range": "stddev: 0.00008150811135236958",
            "extra": "mean: 519.1283217966858 usec\nrounds: 1358"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1941.728371103193,
            "unit": "iter/sec",
            "range": "stddev: 0.00008650704441755034",
            "extra": "mean: 515.005092824518 usec\nrounds: 1519"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1957.4116398510153,
            "unit": "iter/sec",
            "range": "stddev: 0.00007120887620513603",
            "extra": "mean: 510.87874397033477 usec\nrounds: 1617"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2042.6331238609616,
            "unit": "iter/sec",
            "range": "stddev: 0.000053400070683847945",
            "extra": "mean: 489.56417494582263 usec\nrounds: 1389"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1745.6048624880661,
            "unit": "iter/sec",
            "range": "stddev: 0.000027407009838571866",
            "extra": "mean: 572.8673318282742 usec\nrounds: 443"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1484.4665970847907,
            "unit": "iter/sec",
            "range": "stddev: 0.00005642826833873658",
            "extra": "mean: 673.6426417164315 usec\nrounds: 1189"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2608.6679494754076,
            "unit": "iter/sec",
            "range": "stddev: 0.0000376707181121785",
            "extra": "mean: 383.3374041342042 usec\nrounds: 1935"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 151.31236075761697,
            "unit": "iter/sec",
            "range": "stddev: 0.00027572439798312",
            "extra": "mean: 6.60884540425532 msec\nrounds: 141"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "59a57972e1456a9d826c6beb5e04f3a4ab0796bd",
          "message": "feat(adr-028): typed payload wire values — server (lossless int64) (#571)\n\n* proto(adr-028): add EntValue + typed payload map fields; regen stubs\n\nAdditive, non-breaking (buf breaking clean): EntValue oneof (int64/double/\nbool/string/bytes/json) + typed map fields — Node.typed_payload=11,\nCreateNodeOp.typed_data=12, UpdateNodeOp.typed_patch=8, Edge.typed_props=8,\nCreateEdgeOp.typed_props=6. Regenerated Go server + Go SDK + Python SDK\nstubs. Also fixes 'make proto' to call buf directly (matching CI) instead\nof the broken go:generate path.\n\nTranslation + dual-read/write wiring follows in subsequent commits.\n\n* payload(adr-028): lossless typed translation (TypedToPayload/PayloadToTyped)\n\nEntValue carries a real int64, so INTEGER/TIMESTAMP fields round-trip\nexactly with no float64 hop and no safe-integer guard. PayloadToTyped is\nschema-aware and handles json.Number (the store decodes payload_json with\nUseNumber so int64 survives at rest). Recast TestPayload_Int64Spectrum_BugC\nto assert the typed path is lossless (wire + at-rest) across the full\nint64 spectrum incl. MaxInt64/MinInt64; it now PASSES. Companion test\ndocuments the legacy Struct path stays lossy by design.\n\n* docs(adr-028): widen scope — canonical-number decode at all boundaries + scalar wire values (#572)\n\nThe int64 fix is systemic: a shared UseNumber+normalize decode must be\napplied consistently at wal.DecodeEvent, the applyUpdateNode merge read,\nand every payload_json egress (CAS compares store-decoded vs event-decoded\nvia reflect.DeepEqual). Scope also covers the scalar wire-value fields that\ncorrupt int64 today (#572): FieldFilter.value, GetNodeByKeyRequest.value,\nUpdateNodePrecondition.equals, and SDK toProtoValue.\n\n* feat(adr-028): canonical int64-preserving JSON decode at WAL + CAS boundaries\n\nAdd internal/jsonnum (json.Decoder.UseNumber + normalize json.Number ->\nint64-if-integral-else-float64) and apply it consistently at wal.DecodeEvent\nand the applyUpdateNode CAS merge. Integer payload values now survive the\nWAL as int64 instead of collapsing to float64, and update_node preconditions\ncompare on a single representation (reflect.DeepEqual int64==int64) — the\ninconsistency that previously broke CAS/DeleteWhere when only one boundary\nused UseNumber. Foundation for typed-payload egress/ingress and the #572\nscalar-value fields. jsonnum int64-spectrum tests added; full server suite green.\n\n* feat(adr-028): typed payload ingress — prefer typed_data/typed_patch (lossless int64 writes)\n\nexecute_atomic now reads CreateNodeOp.typed_data / UpdateNodeOp.typed_patch\nin preference to the legacy Struct data/patch (via TypedToPayload). Combined\nwith the canonical WAL decode, an int64 > 2^53 written through typed_data\npersists losslessly to payload_json — proven end-to-end by\nTestExecuteAtomic_CreateNodeTypedDataInt64. Legacy Struct path unchanged\n(backward-compatible). Egress (typed_payload) + #572 scalar fields + SDKs follow.\n\n* feat(adr-028): typed payload egress on primary node reads\n\nstoreNodeToProto (QueryNodes/GetNodes/GetConnectedNodes) and nodeToProto\n(GetNode) now populate Node.typed_payload alongside the legacy Struct\npayload, decoding payload_json via the canonical jsonnum path so int64 >2^53\nround-trips losslessly on reads. Regression test\nTestQueryNodes_TypedPayloadPreservesInt64. Legacy Struct payload unchanged\n(backward-compatible). GetNodeByKey, SearchNodes, and edge typed_props next.\n\n* feat(adr-028): complete typed payload egress (edges, GetNodeByKey, SearchNodes) + store canonical decode\n\nPopulate typed_payload on GetNodeByKey + SearchNodes and typed_props on\nedges (edgeToProto), all via the canonical jsonnum decode so int64 >2^53\nround-trips losslessly on every read surface. Route the store-layer\nUpdateNode merge and TenantExport decodes through jsonnum too, so int64\nsurvives read-modify-write and GDPR export. Adds payloadTypeName helper.\nLegacy Struct payload/props unchanged (backward-compatible). Full server\nsuite green.",
          "timestamp": "2026-05-24T05:02:06+01:00",
          "tree_id": "a9e7754b1010e8baada0315ca6046d77a8be5281",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/59a57972e1456a9d826c6beb5e04f3a4ab0796bd"
        },
        "date": 1779595416887,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 4278.341306131347,
            "unit": "iter/sec",
            "range": "stddev: 0.00003153374747226537",
            "extra": "mean: 233.73544288457933 usec\nrounds: 1567"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2792.73143384434,
            "unit": "iter/sec",
            "range": "stddev: 0.00003548182695519145",
            "extra": "mean: 358.0723831447867 usec\nrounds: 1412"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1417.7522705314484,
            "unit": "iter/sec",
            "range": "stddev: 0.00007121839679343473",
            "extra": "mean: 705.3418434132693 usec\nrounds: 1207"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 1020.8210158706112,
            "unit": "iter/sec",
            "range": "stddev: 0.00008720481786611044",
            "extra": "mean: 979.6036567165951 usec\nrounds: 871"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2707.0832720530284,
            "unit": "iter/sec",
            "range": "stddev: 0.000053482881090555165",
            "extra": "mean: 369.4012704831236 usec\nrounds: 1904"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2641.5438052997774,
            "unit": "iter/sec",
            "range": "stddev: 0.000057437411143232936",
            "extra": "mean: 378.56650266169424 usec\nrounds: 2254"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2772.2085086922416,
            "unit": "iter/sec",
            "range": "stddev: 0.00005003057833411691",
            "extra": "mean: 360.7232272985624 usec\nrounds: 1751"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 966.9056152536563,
            "unit": "iter/sec",
            "range": "stddev: 0.0028474332608938125",
            "extra": "mean: 1.034227109889792 msec\nrounds: 91"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 768.4839990322614,
            "unit": "iter/sec",
            "range": "stddev: 0.00401575523594694",
            "extra": "mean: 1.301263268017659 msec\nrounds: 444"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 655.5430012249434,
            "unit": "iter/sec",
            "range": "stddev: 0.008139243270593621",
            "extra": "mean: 1.525452942265277 msec\nrounds: 1351"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 3369.472165933771,
            "unit": "iter/sec",
            "range": "stddev: 0.00003595337618412666",
            "extra": "mean: 296.7823892745745 usec\nrounds: 1585"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 170.1374593414583,
            "unit": "iter/sec",
            "range": "stddev: 0.0001075970607301978",
            "extra": "mean: 5.877600405405399 msec\nrounds: 148"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "59a57972e1456a9d826c6beb5e04f3a4ab0796bd",
          "message": "feat(adr-028): typed payload wire values — server (lossless int64) (#571)\n\n* proto(adr-028): add EntValue + typed payload map fields; regen stubs\n\nAdditive, non-breaking (buf breaking clean): EntValue oneof (int64/double/\nbool/string/bytes/json) + typed map fields — Node.typed_payload=11,\nCreateNodeOp.typed_data=12, UpdateNodeOp.typed_patch=8, Edge.typed_props=8,\nCreateEdgeOp.typed_props=6. Regenerated Go server + Go SDK + Python SDK\nstubs. Also fixes 'make proto' to call buf directly (matching CI) instead\nof the broken go:generate path.\n\nTranslation + dual-read/write wiring follows in subsequent commits.\n\n* payload(adr-028): lossless typed translation (TypedToPayload/PayloadToTyped)\n\nEntValue carries a real int64, so INTEGER/TIMESTAMP fields round-trip\nexactly with no float64 hop and no safe-integer guard. PayloadToTyped is\nschema-aware and handles json.Number (the store decodes payload_json with\nUseNumber so int64 survives at rest). Recast TestPayload_Int64Spectrum_BugC\nto assert the typed path is lossless (wire + at-rest) across the full\nint64 spectrum incl. MaxInt64/MinInt64; it now PASSES. Companion test\ndocuments the legacy Struct path stays lossy by design.\n\n* docs(adr-028): widen scope — canonical-number decode at all boundaries + scalar wire values (#572)\n\nThe int64 fix is systemic: a shared UseNumber+normalize decode must be\napplied consistently at wal.DecodeEvent, the applyUpdateNode merge read,\nand every payload_json egress (CAS compares store-decoded vs event-decoded\nvia reflect.DeepEqual). Scope also covers the scalar wire-value fields that\ncorrupt int64 today (#572): FieldFilter.value, GetNodeByKeyRequest.value,\nUpdateNodePrecondition.equals, and SDK toProtoValue.\n\n* feat(adr-028): canonical int64-preserving JSON decode at WAL + CAS boundaries\n\nAdd internal/jsonnum (json.Decoder.UseNumber + normalize json.Number ->\nint64-if-integral-else-float64) and apply it consistently at wal.DecodeEvent\nand the applyUpdateNode CAS merge. Integer payload values now survive the\nWAL as int64 instead of collapsing to float64, and update_node preconditions\ncompare on a single representation (reflect.DeepEqual int64==int64) — the\ninconsistency that previously broke CAS/DeleteWhere when only one boundary\nused UseNumber. Foundation for typed-payload egress/ingress and the #572\nscalar-value fields. jsonnum int64-spectrum tests added; full server suite green.\n\n* feat(adr-028): typed payload ingress — prefer typed_data/typed_patch (lossless int64 writes)\n\nexecute_atomic now reads CreateNodeOp.typed_data / UpdateNodeOp.typed_patch\nin preference to the legacy Struct data/patch (via TypedToPayload). Combined\nwith the canonical WAL decode, an int64 > 2^53 written through typed_data\npersists losslessly to payload_json — proven end-to-end by\nTestExecuteAtomic_CreateNodeTypedDataInt64. Legacy Struct path unchanged\n(backward-compatible). Egress (typed_payload) + #572 scalar fields + SDKs follow.\n\n* feat(adr-028): typed payload egress on primary node reads\n\nstoreNodeToProto (QueryNodes/GetNodes/GetConnectedNodes) and nodeToProto\n(GetNode) now populate Node.typed_payload alongside the legacy Struct\npayload, decoding payload_json via the canonical jsonnum path so int64 >2^53\nround-trips losslessly on reads. Regression test\nTestQueryNodes_TypedPayloadPreservesInt64. Legacy Struct payload unchanged\n(backward-compatible). GetNodeByKey, SearchNodes, and edge typed_props next.\n\n* feat(adr-028): complete typed payload egress (edges, GetNodeByKey, SearchNodes) + store canonical decode\n\nPopulate typed_payload on GetNodeByKey + SearchNodes and typed_props on\nedges (edgeToProto), all via the canonical jsonnum decode so int64 >2^53\nround-trips losslessly on every read surface. Route the store-layer\nUpdateNode merge and TenantExport decodes through jsonnum too, so int64\nsurvives read-modify-write and GDPR export. Adds payloadTypeName helper.\nLegacy Struct payload/props unchanged (backward-compatible). Full server\nsuite green.",
          "timestamp": "2026-05-24T05:02:06+01:00",
          "tree_id": "a9e7754b1010e8baada0315ca6046d77a8be5281",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/59a57972e1456a9d826c6beb5e04f3a4ab0796bd"
        },
        "date": 1779595444786,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3956.2927848677296,
            "unit": "iter/sec",
            "range": "stddev: 0.00003191057256321321",
            "extra": "mean: 252.76187946070652 usec\nrounds: 1261"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2467.0330140910155,
            "unit": "iter/sec",
            "range": "stddev: 0.00004660012736064288",
            "extra": "mean: 405.3452038494315 usec\nrounds: 1143"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1116.0719447343813,
            "unit": "iter/sec",
            "range": "stddev: 0.00010704639372952899",
            "extra": "mean: 895.9995856163147 usec\nrounds: 876"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 856.5492147816959,
            "unit": "iter/sec",
            "range": "stddev: 0.0000916167182603717",
            "extra": "mean: 1.167475239884336 msec\nrounds: 692"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2045.2319206836487,
            "unit": "iter/sec",
            "range": "stddev: 0.00009623124899290623",
            "extra": "mean: 488.9421047495363 usec\nrounds: 1537"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2147.9870788152493,
            "unit": "iter/sec",
            "range": "stddev: 0.0000837244422708347",
            "extra": "mean: 465.5521487361848 usec\nrounds: 1701"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2110.033452213067,
            "unit": "iter/sec",
            "range": "stddev: 0.00007133862562922224",
            "extra": "mean: 473.9261356028122 usec\nrounds: 1792"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2329.2354044635003,
            "unit": "iter/sec",
            "range": "stddev: 0.00005131575765041842",
            "extra": "mean: 429.32543360954656 usec\nrounds: 482"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 2181.6539739461823,
            "unit": "iter/sec",
            "range": "stddev: 0.00002810525522249445",
            "extra": "mean: 458.36783098613796 usec\nrounds: 426"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1740.8899754910676,
            "unit": "iter/sec",
            "range": "stddev: 0.00008268645265045306",
            "extra": "mean: 574.4188398338738 usec\nrounds: 1205"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2982.6324263886368,
            "unit": "iter/sec",
            "range": "stddev: 0.00008582521408201129",
            "extra": "mean: 335.2743003638559 usec\nrounds: 1375"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 137.86527788462826,
            "unit": "iter/sec",
            "range": "stddev: 0.001636711605791911",
            "extra": "mean: 7.253457979730357 msec\nrounds: 148"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "839f458c1e17acb9e79a6864e2a9566d26de0d14",
          "message": "feat(adr-028): SDKs emit/read typed payload — end-to-end lossless int64 (#576)\n\n* feat(adr-028): Go SDK emits/reads typed payload (lossless int64)\n\nThe Go SDK now sends typed_data/typed_patch (EntValue map) alongside the\nlegacy Struct data/patch (dual-write — still works against pre-v1.20\nservers), and prefers typed_payload/typed_props on read, falling back to\nStruct for older servers. int64 >2^53 round-trips losslessly through the\nGo SDK (TestPayloadRoundTrip_Int64Spectrum_BugC, recast to the typed path).\nPython SDK follows in this same PR before release (both SDKs ship together).\n\n* feat(adr-028): Python SDK emits/reads typed payload (lossless int64)\n\nThe Python SDK now dual-writes typed_data/typed_patch (EntValue) alongside\nthe legacy Struct data/patch, and prefers typed_payload/typed_props on read\n(falling back to Struct for pre-v1.20 servers). int64 >2^53 round-trips\nlosslessly through the Python SDK; unit test test_typed_payload covers the\nfull spectrum. Pairs with the Go SDK so both ship together.",
          "timestamp": "2026-05-24T06:03:04+01:00",
          "tree_id": "edee9cf8b7a32c8003a32eeb8e18bbbac598f693",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/839f458c1e17acb9e79a6864e2a9566d26de0d14"
        },
        "date": 1779599094680,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3212.4215069346237,
            "unit": "iter/sec",
            "range": "stddev: 0.000030239995887648907",
            "extra": "mean: 311.2916526804809 usec\nrounds: 1287"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2087.537372748796,
            "unit": "iter/sec",
            "range": "stddev: 0.00005250500056526722",
            "extra": "mean: 479.03333997955457 usec\nrounds: 1003"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 948.7042125867335,
            "unit": "iter/sec",
            "range": "stddev: 0.0001055049171843447",
            "extra": "mean: 1.0540693155281808 msec\nrounds: 805"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 759.517311695483,
            "unit": "iter/sec",
            "range": "stddev: 0.00011415696469347983",
            "extra": "mean: 1.3166256839724737 msec\nrounds: 443"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1968.6743542217828,
            "unit": "iter/sec",
            "range": "stddev: 0.00008170420880536974",
            "extra": "mean: 507.95602525908873 usec\nrounds: 1544"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1935.7039796029783,
            "unit": "iter/sec",
            "range": "stddev: 0.00009521408640734822",
            "extra": "mean: 516.6079165705412 usec\nrounds: 1738"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2033.4975106952918,
            "unit": "iter/sec",
            "range": "stddev: 0.0000796107468277191",
            "extra": "mean: 491.7635722396733 usec\nrounds: 1585"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1870.695338624362,
            "unit": "iter/sec",
            "range": "stddev: 0.00021730546007906138",
            "extra": "mean: 534.5605879016953 usec\nrounds: 529"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1786.7405545886602,
            "unit": "iter/sec",
            "range": "stddev: 0.00006047960580399369",
            "extra": "mean: 559.6783469384104 usec\nrounds: 343"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1540.3887427484665,
            "unit": "iter/sec",
            "range": "stddev: 0.00005928718704055892",
            "extra": "mean: 649.1867749018548 usec\nrounds: 1275"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2552.325551024041,
            "unit": "iter/sec",
            "range": "stddev: 0.000056852295050966336",
            "extra": "mean: 391.79954908133925 usec\nrounds: 1905"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 122.65120165979295,
            "unit": "iter/sec",
            "range": "stddev: 0.001881887551354002",
            "extra": "mean: 8.153201815125927 msec\nrounds: 119"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "839f458c1e17acb9e79a6864e2a9566d26de0d14",
          "message": "feat(adr-028): SDKs emit/read typed payload — end-to-end lossless int64 (#576)\n\n* feat(adr-028): Go SDK emits/reads typed payload (lossless int64)\n\nThe Go SDK now sends typed_data/typed_patch (EntValue map) alongside the\nlegacy Struct data/patch (dual-write — still works against pre-v1.20\nservers), and prefers typed_payload/typed_props on read, falling back to\nStruct for older servers. int64 >2^53 round-trips losslessly through the\nGo SDK (TestPayloadRoundTrip_Int64Spectrum_BugC, recast to the typed path).\nPython SDK follows in this same PR before release (both SDKs ship together).\n\n* feat(adr-028): Python SDK emits/reads typed payload (lossless int64)\n\nThe Python SDK now dual-writes typed_data/typed_patch (EntValue) alongside\nthe legacy Struct data/patch, and prefers typed_payload/typed_props on read\n(falling back to Struct for pre-v1.20 servers). int64 >2^53 round-trips\nlosslessly through the Python SDK; unit test test_typed_payload covers the\nfull spectrum. Pairs with the Go SDK so both ship together.",
          "timestamp": "2026-05-24T06:03:04+01:00",
          "tree_id": "edee9cf8b7a32c8003a32eeb8e18bbbac598f693",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/839f458c1e17acb9e79a6864e2a9566d26de0d14"
        },
        "date": 1779599097077,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3300.376225735481,
            "unit": "iter/sec",
            "range": "stddev: 0.00004153299275585862",
            "extra": "mean: 302.9957591508078 usec\nrounds: 1366"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2155.2008568003084,
            "unit": "iter/sec",
            "range": "stddev: 0.00004717866103413981",
            "extra": "mean: 463.9938764151372 usec\nrounds: 1060"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1081.1809378045134,
            "unit": "iter/sec",
            "range": "stddev: 0.0000825086855711451",
            "extra": "mean: 924.9145679821524 usec\nrounds: 912"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 788.5099928441024,
            "unit": "iter/sec",
            "range": "stddev: 0.00008238792384437991",
            "extra": "mean: 1.268214745628102 msec\nrounds: 629"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1914.3223228050701,
            "unit": "iter/sec",
            "range": "stddev: 0.00009259541041972027",
            "extra": "mean: 522.3780698198686 usec\nrounds: 1776"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1947.70996415689,
            "unit": "iter/sec",
            "range": "stddev: 0.00009010391196844052",
            "extra": "mean: 513.4234657123975 usec\nrounds: 1677"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1984.1028911616268,
            "unit": "iter/sec",
            "range": "stddev: 0.00007981606536302669",
            "extra": "mean: 504.0061200730034 usec\nrounds: 1649"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2037.3899889451382,
            "unit": "iter/sec",
            "range": "stddev: 0.00004411950584765527",
            "extra": "mean: 490.8240471514988 usec\nrounds: 1527"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1914.5859728815442,
            "unit": "iter/sec",
            "range": "stddev: 0.000043185666968788515",
            "extra": "mean: 522.3061351979676 usec\nrounds: 429"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1595.6944852257939,
            "unit": "iter/sec",
            "range": "stddev: 0.00006002530392438851",
            "extra": "mean: 626.68637966653 usec\nrounds: 1259"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2659.2751896572777,
            "unit": "iter/sec",
            "range": "stddev: 0.00002673844296069458",
            "extra": "mean: 376.0423155487259 usec\nrounds: 1312"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 145.84347053639837,
            "unit": "iter/sec",
            "range": "stddev: 0.0004100663162495954",
            "extra": "mean: 6.856666234848193 msec\nrounds: 132"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "0fba2e1f600ead11a1ed20088828c0f1274ac1ea",
          "message": "feat(#572): typed scalar wire-values (filter/unique-key/CAS) — lossless int64 (#577)\n\n* feat(#572): typed scalar wire-values — proto + server decode + Go SDK filter/CAS\n\nAdds EntValue to FieldFilter.typed_value, GetNodeByKeyRequest.typed_value,\nUpdateNodePrecondition.typed_equals (additive, buf-breaking clean) so int64\nfilters / unique-key lookups / CAS stop corrupting through google.protobuf.Value's\ndouble. Server prefers the typed field at all three decode sites (query filter,\nGetNodeByKey, CAS equals) via payload.EntValueToGo. Go SDK dual-writes\ntyped_value on filters and typed_equals on preconditions. Regenerated all stubs.\nRegression: TestQueryNodes_TypedFilterInt64. Remaining in this PR: Go SDK\nGetNodeByKey (transport signature) + Python SDK encode.\n\n* feat(sdk): dual-write typed scalar wire-values for filters, unique-key lookups, and CAS (#572)\n\nThe legacy google.protobuf.Struct Value carries every number as an\nIEEE-754 double, so an int64 above 2^53 used as a query filter, a\nunique-key lookup value, or a CAS precondition silently mismatched the\nstored value — the same class of corruption ADR-028 fixed for payloads.\n\nBoth SDKs now dual-write the typed EntValue alongside the legacy Value\non the three remaining scalar surfaces:\n\n  - FieldFilter.typed_value      (QueryNodes + DeleteWhere where-clauses)\n  - GetNodeByKeyRequest.typed_value\n  - UpdateNodePrecondition.typed_equals\n\nGo SDK: the Transport.GetNodeByKey signature takes the raw value\n(value any) instead of a pre-built *structpb.Value, so the grpc\ntransport builds both the legacy Struct and the typed EntValue in one\nplace; scope.GetByKey forwards the raw value untouched. filterToProto\nand the precondition builder already dual-write on their scalar\nbranches.\n\nPython SDK: _grpc_client wires _value_to_entvalue into the filter,\nGetNodeByKey, and precondition builders.\n\nThe server prefers the typed branch when present (decode landed\nearlier this cycle), falling back to the legacy Value, so old clients\nkeep working.\n\nTests: 27 new big-int parametrized encode cases across the three\nsurfaces; an end-to-end integration test pins the server's\ntyped-over-legacy preference with a discriminating pair (legacy value\npoints at a missing row, typed value at a real one).\n\nCloses #572.\n\n* docs: refresh GetNodeByKey transport comment + regenerated sdk-go reference\n\nThe Transport.GetNodeByKey signature now takes the raw value and\ndual-writes the legacy google.protobuf.Value plus the typed EntValue;\nupdate the doc comment to match and regenerate docs/generated/sdk-go.md\nwith the go.mod-pinned toolchain (go 1.25) so the Docs Coverage gate\nstays green.",
          "timestamp": "2026-05-24T07:09:27+01:00",
          "tree_id": "3119f88c280f3396f8105aeaac0e5b306f6b41d9",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/0fba2e1f600ead11a1ed20088828c0f1274ac1ea"
        },
        "date": 1779603072062,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3333.777285229075,
            "unit": "iter/sec",
            "range": "stddev: 0.000028919246779243828",
            "extra": "mean: 299.96004965019335 usec\nrounds: 1430"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2167.2321158918853,
            "unit": "iter/sec",
            "range": "stddev: 0.000038929765708003526",
            "extra": "mean: 461.418042242544 usec\nrounds: 1160"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1070.315147654603,
            "unit": "iter/sec",
            "range": "stddev: 0.00010083111915428088",
            "extra": "mean: 934.304258134919 usec\nrounds: 922"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 714.0189862266524,
            "unit": "iter/sec",
            "range": "stddev: 0.0001817471642325663",
            "extra": "mean: 1.4005229822874319 msec\nrounds: 621"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1793.9792089499995,
            "unit": "iter/sec",
            "range": "stddev: 0.0001408043097531099",
            "extra": "mean: 557.4200609522624 usec\nrounds: 1575"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1821.3884488976514,
            "unit": "iter/sec",
            "range": "stddev: 0.00014403500865342773",
            "extra": "mean: 549.0317019443185 usec\nrounds: 1493"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1933.1744745506767,
            "unit": "iter/sec",
            "range": "stddev: 0.00009830568316050718",
            "extra": "mean: 517.2838836662313 usec\nrounds: 1745"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2035.5750271287875,
            "unit": "iter/sec",
            "range": "stddev: 0.00004045591972470543",
            "extra": "mean: 491.2616762696862 usec\nrounds: 1458"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1770.7883459776338,
            "unit": "iter/sec",
            "range": "stddev: 0.0000447711100436228",
            "extra": "mean: 564.7202288582437 usec\nrounds: 402"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1497.8301376641657,
            "unit": "iter/sec",
            "range": "stddev: 0.000046921043380507045",
            "extra": "mean: 667.6324470006183 usec\nrounds: 1217"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2658.7231167544237,
            "unit": "iter/sec",
            "range": "stddev: 0.00003641426527612448",
            "extra": "mean: 376.12039918648145 usec\nrounds: 1473"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 166.98121046059552,
            "unit": "iter/sec",
            "range": "stddev: 0.0001627901732786749",
            "extra": "mean: 5.988697753727097 msec\nrounds: 134"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "0fba2e1f600ead11a1ed20088828c0f1274ac1ea",
          "message": "feat(#572): typed scalar wire-values (filter/unique-key/CAS) — lossless int64 (#577)\n\n* feat(#572): typed scalar wire-values — proto + server decode + Go SDK filter/CAS\n\nAdds EntValue to FieldFilter.typed_value, GetNodeByKeyRequest.typed_value,\nUpdateNodePrecondition.typed_equals (additive, buf-breaking clean) so int64\nfilters / unique-key lookups / CAS stop corrupting through google.protobuf.Value's\ndouble. Server prefers the typed field at all three decode sites (query filter,\nGetNodeByKey, CAS equals) via payload.EntValueToGo. Go SDK dual-writes\ntyped_value on filters and typed_equals on preconditions. Regenerated all stubs.\nRegression: TestQueryNodes_TypedFilterInt64. Remaining in this PR: Go SDK\nGetNodeByKey (transport signature) + Python SDK encode.\n\n* feat(sdk): dual-write typed scalar wire-values for filters, unique-key lookups, and CAS (#572)\n\nThe legacy google.protobuf.Struct Value carries every number as an\nIEEE-754 double, so an int64 above 2^53 used as a query filter, a\nunique-key lookup value, or a CAS precondition silently mismatched the\nstored value — the same class of corruption ADR-028 fixed for payloads.\n\nBoth SDKs now dual-write the typed EntValue alongside the legacy Value\non the three remaining scalar surfaces:\n\n  - FieldFilter.typed_value      (QueryNodes + DeleteWhere where-clauses)\n  - GetNodeByKeyRequest.typed_value\n  - UpdateNodePrecondition.typed_equals\n\nGo SDK: the Transport.GetNodeByKey signature takes the raw value\n(value any) instead of a pre-built *structpb.Value, so the grpc\ntransport builds both the legacy Struct and the typed EntValue in one\nplace; scope.GetByKey forwards the raw value untouched. filterToProto\nand the precondition builder already dual-write on their scalar\nbranches.\n\nPython SDK: _grpc_client wires _value_to_entvalue into the filter,\nGetNodeByKey, and precondition builders.\n\nThe server prefers the typed branch when present (decode landed\nearlier this cycle), falling back to the legacy Value, so old clients\nkeep working.\n\nTests: 27 new big-int parametrized encode cases across the three\nsurfaces; an end-to-end integration test pins the server's\ntyped-over-legacy preference with a discriminating pair (legacy value\npoints at a missing row, typed value at a real one).\n\nCloses #572.\n\n* docs: refresh GetNodeByKey transport comment + regenerated sdk-go reference\n\nThe Transport.GetNodeByKey signature now takes the raw value and\ndual-writes the legacy google.protobuf.Value plus the typed EntValue;\nupdate the doc comment to match and regenerate docs/generated/sdk-go.md\nwith the go.mod-pinned toolchain (go 1.25) so the Docs Coverage gate\nstays green.",
          "timestamp": "2026-05-24T07:09:27+01:00",
          "tree_id": "3119f88c280f3396f8105aeaac0e5b306f6b41d9",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/0fba2e1f600ead11a1ed20088828c0f1274ac1ea"
        },
        "date": 1779603092151,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3343.89614561414,
            "unit": "iter/sec",
            "range": "stddev: 0.000028351186154434332",
            "extra": "mean: 299.0523498499203 usec\nrounds: 1332"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2169.193219030253,
            "unit": "iter/sec",
            "range": "stddev: 0.00004165718118803295",
            "extra": "mean: 461.0008879001817 usec\nrounds: 1124"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1043.813806776814,
            "unit": "iter/sec",
            "range": "stddev: 0.00010896001673974704",
            "extra": "mean: 958.0252661036296 usec\nrounds: 947"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 713.4440731788959,
            "unit": "iter/sec",
            "range": "stddev: 0.00017383633996656948",
            "extra": "mean: 1.4016515626015302 msec\nrounds: 615"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1823.5669663782749,
            "unit": "iter/sec",
            "range": "stddev: 0.00012861516997405436",
            "extra": "mean: 548.375803267629 usec\nrounds: 1469"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1864.7306053077953,
            "unit": "iter/sec",
            "range": "stddev: 0.00012759199370809035",
            "extra": "mean: 536.2704924526824 usec\nrounds: 1590"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1968.7153004088505,
            "unit": "iter/sec",
            "range": "stddev: 0.00008549118683650949",
            "extra": "mean: 507.94546057133107 usec\nrounds: 1750"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2042.586789917732,
            "unit": "iter/sec",
            "range": "stddev: 0.00004090303544089368",
            "extra": "mean: 489.5752801966748 usec\nrounds: 1424"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1918.0839462721121,
            "unit": "iter/sec",
            "range": "stddev: 0.0000398108260880021",
            "extra": "mean: 521.353615384534 usec\nrounds: 455"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1605.584331985701,
            "unit": "iter/sec",
            "range": "stddev: 0.00006706374970406915",
            "extra": "mean: 622.8262073056314 usec\nrounds: 1095"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2668.410362579949,
            "unit": "iter/sec",
            "range": "stddev: 0.000029802667740131726",
            "extra": "mean: 374.75495299499266 usec\nrounds: 1319"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 168.90020869297356,
            "unit": "iter/sec",
            "range": "stddev: 0.00013124749503651105",
            "extra": "mean: 5.9206557987018105 msec\nrounds: 154"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "db1d8462c5b17b659182be06e64a49af07c9c75f",
          "message": "feat(server): keyset cursor pagination for QueryNodes — no silent truncation (#564) (#578)\n\nQueryNodes silently capped results at the 100-row default with no way to\nfetch the rest: list/query helpers issue no limit, the server falls back\nto defaultQueryLimit=100, and there was no cursor. Callers got 100 of N\nwith no error and no continuation (Bug A).\n\nImplements ADR-029 (AIP-158 keyset pagination) for QueryNodes:\n\n  - Request gains page_size + page_token; response gains next_page_token.\n    page_size aliases (and wins over) the legacy limit; offset is\n    deprecated.\n  - The token is opaque and encodes a keyset anchor — the (order_by\n    value, node_id) tuple of the last row — plus the sort direction and a\n    fingerprint of the query (type_id + filters + order_by). A token\n    presented against a different query, or mixed with the deprecated\n    offset, is rejected with INVALID_ARGUMENT instead of returning wrong\n    rows.\n  - Continuation is a seek, not a skip: the store appends\n    `WHERE (order_by, node_id) > (anchor)` (or `<` descending) and always\n    sorts with node_id as the final, unique tiebreaker — a total order, so\n    pages never skip or duplicate a row under concurrent writes. This is\n    the discriminating win over OFFSET, pinned by a delete-stability test.\n  - next_page_token is minted from the last row the store returned\n    (before the ACL post-filter, which only drops rows), so a full page\n    always carries a cursor: a response that omits rows is never silent.\n\nThe previously-xfailed characterization test\ntest_query_does_not_silently_truncate now follows the cursor to retrieve\nall 150 rows and is green.\n\nStore: QueryNodes takes an optional QueryCursor and exposes\nEffectiveOrderBy / NodeOrderValue as the single source of truth for the\norder column so the cursor anchor always matches the SQL ORDER BY.\n\nScope: QueryNodes only. SDK helper auto-follow (both SDKs) and the same\ncursor on SearchNodes / GetEdgesFrom / GetEdgesTo / List* reuse this\npagetoken + keyset machinery and follow in tracked PRs.\n\nTests: token round-trip / fingerprint stability + discrimination /\nrejection paths; end-to-end paging of the full set, keyset stability\nunder a seen-region delete, page_size-over-limit precedence, and\ncross-query / token+offset rejection.\n\nRefs #564, ADR-029.",
          "timestamp": "2026-05-24T07:29:48+01:00",
          "tree_id": "5e4c60faa61f4fcccef92d5ae8fe384488241072",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/db1d8462c5b17b659182be06e64a49af07c9c75f"
        },
        "date": 1779604295975,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3106.3371723011596,
            "unit": "iter/sec",
            "range": "stddev: 0.00002785055264064168",
            "extra": "mean: 321.92255525796793 usec\nrounds: 1493"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2072.4607552052958,
            "unit": "iter/sec",
            "range": "stddev: 0.0000468481477826922",
            "extra": "mean: 482.5181839937621 usec\nrounds: 1212"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 943.5317005469896,
            "unit": "iter/sec",
            "range": "stddev: 0.0001018643608944108",
            "extra": "mean: 1.059847803121267 msec\nrounds: 833"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 462.9117906402987,
            "unit": "iter/sec",
            "range": "stddev: 0.00012900026415808743",
            "extra": "mean: 2.1602387759810613 msec\nrounds: 433"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1938.554792355385,
            "unit": "iter/sec",
            "range": "stddev: 0.0000781179972703772",
            "extra": "mean: 515.8481998772802 usec\nrounds: 1626"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1920.4463252318285,
            "unit": "iter/sec",
            "range": "stddev: 0.00008063237319451374",
            "extra": "mean: 520.7122880038232 usec\nrounds: 2059"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2012.3373964984135,
            "unit": "iter/sec",
            "range": "stddev: 0.0000750612012904139",
            "extra": "mean: 496.934560645774 usec\nrounds: 1921"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1992.8026712373085,
            "unit": "iter/sec",
            "range": "stddev: 0.00003731581903130632",
            "extra": "mean: 501.80583076954196 usec\nrounds: 1560"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1625.682897078938,
            "unit": "iter/sec",
            "range": "stddev: 0.00008402589929050316",
            "extra": "mean: 615.1261121076082 usec\nrounds: 223"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1390.2427534936642,
            "unit": "iter/sec",
            "range": "stddev: 0.0000721126642815673",
            "extra": "mean: 719.2988400673274 usec\nrounds: 1188"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2487.7045204077244,
            "unit": "iter/sec",
            "range": "stddev: 0.00004725526720952693",
            "extra": "mean: 401.9770000000258 usec\nrounds: 1187"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 114.30777699916435,
            "unit": "iter/sec",
            "range": "stddev: 0.0019914085945320365",
            "extra": "mean: 8.748311149531938 msec\nrounds: 107"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "db1d8462c5b17b659182be06e64a49af07c9c75f",
          "message": "feat(server): keyset cursor pagination for QueryNodes — no silent truncation (#564) (#578)\n\nQueryNodes silently capped results at the 100-row default with no way to\nfetch the rest: list/query helpers issue no limit, the server falls back\nto defaultQueryLimit=100, and there was no cursor. Callers got 100 of N\nwith no error and no continuation (Bug A).\n\nImplements ADR-029 (AIP-158 keyset pagination) for QueryNodes:\n\n  - Request gains page_size + page_token; response gains next_page_token.\n    page_size aliases (and wins over) the legacy limit; offset is\n    deprecated.\n  - The token is opaque and encodes a keyset anchor — the (order_by\n    value, node_id) tuple of the last row — plus the sort direction and a\n    fingerprint of the query (type_id + filters + order_by). A token\n    presented against a different query, or mixed with the deprecated\n    offset, is rejected with INVALID_ARGUMENT instead of returning wrong\n    rows.\n  - Continuation is a seek, not a skip: the store appends\n    `WHERE (order_by, node_id) > (anchor)` (or `<` descending) and always\n    sorts with node_id as the final, unique tiebreaker — a total order, so\n    pages never skip or duplicate a row under concurrent writes. This is\n    the discriminating win over OFFSET, pinned by a delete-stability test.\n  - next_page_token is minted from the last row the store returned\n    (before the ACL post-filter, which only drops rows), so a full page\n    always carries a cursor: a response that omits rows is never silent.\n\nThe previously-xfailed characterization test\ntest_query_does_not_silently_truncate now follows the cursor to retrieve\nall 150 rows and is green.\n\nStore: QueryNodes takes an optional QueryCursor and exposes\nEffectiveOrderBy / NodeOrderValue as the single source of truth for the\norder column so the cursor anchor always matches the SQL ORDER BY.\n\nScope: QueryNodes only. SDK helper auto-follow (both SDKs) and the same\ncursor on SearchNodes / GetEdgesFrom / GetEdgesTo / List* reuse this\npagetoken + keyset machinery and follow in tracked PRs.\n\nTests: token round-trip / fingerprint stability + discrimination /\nrejection paths; end-to-end paging of the full set, keyset stability\nunder a seen-region delete, page_size-over-limit precedence, and\ncross-query / token+offset rejection.\n\nRefs #564, ADR-029.",
          "timestamp": "2026-05-24T07:29:48+01:00",
          "tree_id": "5e4c60faa61f4fcccef92d5ae8fe384488241072",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/db1d8462c5b17b659182be06e64a49af07c9c75f"
        },
        "date": 1779604304657,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3334.652195779853,
            "unit": "iter/sec",
            "range": "stddev: 0.000026690035501847878",
            "extra": "mean: 299.88134932498906 usec\nrounds: 1334"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2158.913439233637,
            "unit": "iter/sec",
            "range": "stddev: 0.000042309317047470076",
            "extra": "mean: 463.1959678545409 usec\nrounds: 1151"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1084.9363493154297,
            "unit": "iter/sec",
            "range": "stddev: 0.00008660421047406121",
            "extra": "mean: 921.7130577576993 usec\nrounds: 883"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 476.42418424122087,
            "unit": "iter/sec",
            "range": "stddev: 0.00013998725461702148",
            "extra": "mean: 2.0989698530788368 msec\nrounds: 422"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1926.9977918402642,
            "unit": "iter/sec",
            "range": "stddev: 0.00007817246815795375",
            "extra": "mean: 518.9419542847579 usec\nrounds: 1750"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1923.664746593631,
            "unit": "iter/sec",
            "range": "stddev: 0.0000889139370254524",
            "extra": "mean: 519.8411010914301 usec\nrounds: 2107"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1980.5662925561746,
            "unit": "iter/sec",
            "range": "stddev: 0.00008013750549359805",
            "extra": "mean: 504.90609870441244 usec\nrounds: 1621"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2041.746770654096,
            "unit": "iter/sec",
            "range": "stddev: 0.000051462793009415725",
            "extra": "mean: 489.77670217136625 usec\nrounds: 1427"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1772.3593849548968,
            "unit": "iter/sec",
            "range": "stddev: 0.000047046718255837276",
            "extra": "mean: 564.219654596434 usec\nrounds: 359"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1496.3270029761154,
            "unit": "iter/sec",
            "range": "stddev: 0.00006238687891811366",
            "extra": "mean: 668.3031169062998 usec\nrounds: 1189"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2659.527140588762,
            "unit": "iter/sec",
            "range": "stddev: 0.00002806805977284434",
            "extra": "mean: 376.0066910911921 usec\nrounds: 1897"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 131.42946249529322,
            "unit": "iter/sec",
            "range": "stddev: 0.00016198444654608296",
            "extra": "mean: 7.608644066666652 msec\nrounds: 120"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "586d8d8cb77d49e8417bc6f46895ba36598fc8d6",
          "message": "feat(sdk): auto-follow the keyset cursor so query returns the complete set (#564) (#579)\n\nThe server gained keyset cursor pagination in v1.23.0, but the SDK\nhelpers still issued a single request and returned the first page — so a\nquery over more than the 100-row default still silently truncated for\ncallers. This closes the loop: both SDKs now follow next_page_token to\nexhaustion and return the complete result set by default (ADR-029\ninvariant 3).\n\nGo SDK:\n  - Transport.QueryNodes takes a limit (0 = complete set) and loops\n    next_page_token, accumulating pages; it fences read-after-write only\n    on the first page. WithLimit is now wired through as a total cap, so\n    a small limit no longer over-fetches a full page. A defensive\n    page-count ceiling guards against a server that never clears the\n    cursor.\n\nPython SDK:\n  - query_nodes auto-follows the cursor; client.query / scope.query\n    default `limit` changes from 100 to 0 (= the complete set). A\n    positive limit caps the total; the deprecated `offset` falls back to\n    a single non-cursor request for backward compatibility.\n\nThis is the standard-database contract — a query returns every matching\nrow, not a silent prefix. Pages are fetched at the server's MaxPageSize\n(1000) to minimise round-trips.\n\nBoth SDKs ship together (this PR).\n\nTests: Python unit tests (complete-set follow, limit cap with reduced\nfinal page_size, legacy offset single-shot, single-page) and Go\ntransport tests (multi-page auto-follow over a paginating fake server,\nlimit cap stops early with a bounded final page_size).\n\nCloses #564. Refs ADR-029.",
          "timestamp": "2026-05-24T11:50:49+01:00",
          "tree_id": "f08265428dc15ac88d95185dac8d702928745686",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/586d8d8cb77d49e8417bc6f46895ba36598fc8d6"
        },
        "date": 1779619957816,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2987.2094112965583,
            "unit": "iter/sec",
            "range": "stddev: 0.000033398179252712135",
            "extra": "mean: 334.76059502837575 usec\nrounds: 1247"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1995.9541168293879,
            "unit": "iter/sec",
            "range": "stddev: 0.00004350128928503827",
            "extra": "mean: 501.0135210866067 usec\nrounds: 1067"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 903.6431427160976,
            "unit": "iter/sec",
            "range": "stddev: 0.00011562691244735306",
            "extra": "mean: 1.1066315370847397 msec\nrounds: 782"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 449.78604291290947,
            "unit": "iter/sec",
            "range": "stddev: 0.00014657149718468683",
            "extra": "mean: 2.2232793030299223 msec\nrounds: 363"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1852.2136554641565,
            "unit": "iter/sec",
            "range": "stddev: 0.00009084375966408583",
            "extra": "mean: 539.8945186749552 usec\nrounds: 1419"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1837.5316659451169,
            "unit": "iter/sec",
            "range": "stddev: 0.00008863201616754209",
            "extra": "mean: 544.2083086419409 usec\nrounds: 1620"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1922.6101120057087,
            "unit": "iter/sec",
            "range": "stddev: 0.00007568488264458714",
            "extra": "mean: 520.1262563613474 usec\nrounds: 1572"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1910.8456620572924,
            "unit": "iter/sec",
            "range": "stddev: 0.000048456106419282334",
            "extra": "mean: 523.3285031106909 usec\nrounds: 1286"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1747.455268266058,
            "unit": "iter/sec",
            "range": "stddev: 0.000047548744582713695",
            "extra": "mean: 572.2607142855604 usec\nrounds: 371"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1471.3952598855185,
            "unit": "iter/sec",
            "range": "stddev: 0.00006608099882077924",
            "extra": "mean: 679.6270365026218 usec\nrounds: 1178"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2462.9049088656343,
            "unit": "iter/sec",
            "range": "stddev: 0.000033765539111981266",
            "extra": "mean: 406.02460793363736 usec\nrounds: 1084"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 134.75586936981892,
            "unit": "iter/sec",
            "range": "stddev: 0.0003998786593494015",
            "extra": "mean: 7.420827045801158 msec\nrounds: 131"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "586d8d8cb77d49e8417bc6f46895ba36598fc8d6",
          "message": "feat(sdk): auto-follow the keyset cursor so query returns the complete set (#564) (#579)\n\nThe server gained keyset cursor pagination in v1.23.0, but the SDK\nhelpers still issued a single request and returned the first page — so a\nquery over more than the 100-row default still silently truncated for\ncallers. This closes the loop: both SDKs now follow next_page_token to\nexhaustion and return the complete result set by default (ADR-029\ninvariant 3).\n\nGo SDK:\n  - Transport.QueryNodes takes a limit (0 = complete set) and loops\n    next_page_token, accumulating pages; it fences read-after-write only\n    on the first page. WithLimit is now wired through as a total cap, so\n    a small limit no longer over-fetches a full page. A defensive\n    page-count ceiling guards against a server that never clears the\n    cursor.\n\nPython SDK:\n  - query_nodes auto-follows the cursor; client.query / scope.query\n    default `limit` changes from 100 to 0 (= the complete set). A\n    positive limit caps the total; the deprecated `offset` falls back to\n    a single non-cursor request for backward compatibility.\n\nThis is the standard-database contract — a query returns every matching\nrow, not a silent prefix. Pages are fetched at the server's MaxPageSize\n(1000) to minimise round-trips.\n\nBoth SDKs ship together (this PR).\n\nTests: Python unit tests (complete-set follow, limit cap with reduced\nfinal page_size, legacy offset single-shot, single-page) and Go\ntransport tests (multi-page auto-follow over a paginating fake server,\nlimit cap stops early with a bounded final page_size).\n\nCloses #564. Refs ADR-029.",
          "timestamp": "2026-05-24T11:50:49+01:00",
          "tree_id": "f08265428dc15ac88d95185dac8d702928745686",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/586d8d8cb77d49e8417bc6f46895ba36598fc8d6"
        },
        "date": 1779619962890,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3037.36635415561,
            "unit": "iter/sec",
            "range": "stddev: 0.000025803740449467253",
            "extra": "mean: 329.23259277954327 usec\nrounds: 1385"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2034.8328437913524,
            "unit": "iter/sec",
            "range": "stddev: 0.000045575224978963346",
            "extra": "mean: 491.44085866865333 usec\nrounds: 1217"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 920.1424694265318,
            "unit": "iter/sec",
            "range": "stddev: 0.00009760489689119671",
            "extra": "mean: 1.0867882238097744 msec\nrounds: 840"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 460.72457650358353,
            "unit": "iter/sec",
            "range": "stddev: 0.00011461388313157718",
            "extra": "mean: 2.1704941542058633 msec\nrounds: 428"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1898.876021933987,
            "unit": "iter/sec",
            "range": "stddev: 0.00007724935842736173",
            "extra": "mean: 526.6273250327894 usec\nrounds: 1526"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1850.7035012495003,
            "unit": "iter/sec",
            "range": "stddev: 0.00009256676222768011",
            "extra": "mean: 540.3350668137015 usec\nrounds: 1811"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1816.4254536143214,
            "unit": "iter/sec",
            "range": "stddev: 0.00015950361691756472",
            "extra": "mean: 550.5318140142779 usec\nrounds: 1527"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1881.2498224783778,
            "unit": "iter/sec",
            "range": "stddev: 0.00006441955745721249",
            "extra": "mean: 531.561511954106 usec\nrounds: 1213"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1726.1228810554858,
            "unit": "iter/sec",
            "range": "stddev: 0.00009001036114024358",
            "extra": "mean: 579.3330306753841 usec\nrounds: 326"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1440.3608491608516,
            "unit": "iter/sec",
            "range": "stddev: 0.00008418285303868342",
            "extra": "mean: 694.2704674197413 usec\nrounds: 1151"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2453.4701677617713,
            "unit": "iter/sec",
            "range": "stddev: 0.000036024966652730794",
            "extra": "mean: 407.585962584689 usec\nrounds: 588"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 138.97834758295338,
            "unit": "iter/sec",
            "range": "stddev: 0.0001594721490173359",
            "extra": "mean: 7.195365446427689 msec\nrounds: 112"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "5b7242135ecbeac0cfaec17ad06a287b9398460e",
          "message": "fix(api): read RPCs surface genuine store faults instead of masking them as empty+OK (#573) (#581)\n\nSeveral read RPCs swallowed a genuine post-open store fault (SQLite IO\nerror, on-disk corruption, scan failure, per-id panic) into an empty\nresponse with codes.OK — so a caller could not tell \"no results\" from \"the\nstore is broken\", and downstream silently dropped data with no alert. The\ntenant is already lazy-opened before these reads, so a post-open error is\na real fault, never the not-open case.\n\nNow surfaced as a sanitized codes.Internal (errs.Internal — no path/schema\nleak), preserving typed sentinels; empty+OK is reserved for a genuinely\nempty result set:\n\n  - GetEdgesFrom / GetEdgesTo — store error → Internal.\n  - GetConnectedNodes — source-gate ACL fault, BFS traversal fault, and\n    per-row marshal failure → Internal (the intentional \"source not\n    accessible → empty, no existence leak\" path is preserved).\n  - SearchNodes — genuine FTS/scan fault → Internal; a malformed FTS5\n    MATCH query is a CLIENT error and now returns InvalidArgument (was\n    masked as empty+OK, which looked like \"no matches\").\n  - GetNodes — a per-id GetNode/CanAccess error or a fan-out panic is no\n    longer reported as a missing id (which masked data loss); it surfaces\n    as Internal. A real miss (ErrNodeNotFound) still flows to missing_ids,\n    and an explicit ACL denial still flows to missing_ids, unchanged.\n\nTests: a fault-injection harness drops the underlying table through a\nsecond raw connection to the tenant SQLite file (the store's own pool then\nfaults on its next read) and asserts codes.Internal for QueryNodes,\nGetNodes, GetEdgesFrom/To, GetConnectedNodes, and SearchNodes; the\nSearchNodes malformed-query test now asserts InvalidArgument.\n\nCloses #573.",
          "timestamp": "2026-05-24T12:10:01+01:00",
          "tree_id": "c8c902239757902095bbb62317821d1d04b25ffc",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/5b7242135ecbeac0cfaec17ad06a287b9398460e"
        },
        "date": 1779621103535,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3294.3990885452795,
            "unit": "iter/sec",
            "range": "stddev: 0.000022601304072137867",
            "extra": "mean: 303.5454943746885 usec\nrounds: 1600"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2181.2896291700636,
            "unit": "iter/sec",
            "range": "stddev: 0.00003797532582605355",
            "extra": "mean: 458.4443929990534 usec\nrounds: 1257"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1010.5097394053753,
            "unit": "iter/sec",
            "range": "stddev: 0.00007544309415160928",
            "extra": "mean: 989.5995664410324 usec\nrounds: 888"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 471.0494840885746,
            "unit": "iter/sec",
            "range": "stddev: 0.000197909582723294",
            "extra": "mean: 2.122919212903677 msec\nrounds: 465"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1842.2976376832748,
            "unit": "iter/sec",
            "range": "stddev: 0.00012625510514020292",
            "extra": "mean: 542.8004571821084 usec\nrounds: 1448"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1858.1700520109111,
            "unit": "iter/sec",
            "range": "stddev: 0.00012968592894815697",
            "extra": "mean: 538.1638773683819 usec\nrounds: 1003"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1909.1153954643596,
            "unit": "iter/sec",
            "range": "stddev: 0.00013318643253527063",
            "extra": "mean: 523.8028054122768 usec\nrounds: 1552"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2083.546709789472,
            "unit": "iter/sec",
            "range": "stddev: 0.00004120358230359949",
            "extra": "mean: 479.95084309918985 usec\nrounds: 1536"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1949.9102342018227,
            "unit": "iter/sec",
            "range": "stddev: 0.00004183284626949891",
            "extra": "mean: 512.844120954799 usec\nrounds: 587"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1625.4678319551015,
            "unit": "iter/sec",
            "range": "stddev: 0.00005305062489451871",
            "extra": "mean: 615.2074992448217 usec\nrounds: 1324"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2702.262791151809,
            "unit": "iter/sec",
            "range": "stddev: 0.00003406284732745247",
            "extra": "mean: 370.0602336953918 usec\nrounds: 1288"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 167.40500522155688,
            "unit": "iter/sec",
            "range": "stddev: 0.00025756242578395684",
            "extra": "mean: 5.973537043749211 msec\nrounds: 160"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "5b7242135ecbeac0cfaec17ad06a287b9398460e",
          "message": "fix(api): read RPCs surface genuine store faults instead of masking them as empty+OK (#573) (#581)\n\nSeveral read RPCs swallowed a genuine post-open store fault (SQLite IO\nerror, on-disk corruption, scan failure, per-id panic) into an empty\nresponse with codes.OK — so a caller could not tell \"no results\" from \"the\nstore is broken\", and downstream silently dropped data with no alert. The\ntenant is already lazy-opened before these reads, so a post-open error is\na real fault, never the not-open case.\n\nNow surfaced as a sanitized codes.Internal (errs.Internal — no path/schema\nleak), preserving typed sentinels; empty+OK is reserved for a genuinely\nempty result set:\n\n  - GetEdgesFrom / GetEdgesTo — store error → Internal.\n  - GetConnectedNodes — source-gate ACL fault, BFS traversal fault, and\n    per-row marshal failure → Internal (the intentional \"source not\n    accessible → empty, no existence leak\" path is preserved).\n  - SearchNodes — genuine FTS/scan fault → Internal; a malformed FTS5\n    MATCH query is a CLIENT error and now returns InvalidArgument (was\n    masked as empty+OK, which looked like \"no matches\").\n  - GetNodes — a per-id GetNode/CanAccess error or a fan-out panic is no\n    longer reported as a missing id (which masked data loss); it surfaces\n    as Internal. A real miss (ErrNodeNotFound) still flows to missing_ids,\n    and an explicit ACL denial still flows to missing_ids, unchanged.\n\nTests: a fault-injection harness drops the underlying table through a\nsecond raw connection to the tenant SQLite file (the store's own pool then\nfaults on its next read) and asserts codes.Internal for QueryNodes,\nGetNodes, GetEdgesFrom/To, GetConnectedNodes, and SearchNodes; the\nSearchNodes malformed-query test now asserts InvalidArgument.\n\nCloses #573.",
          "timestamp": "2026-05-24T12:10:01+01:00",
          "tree_id": "c8c902239757902095bbb62317821d1d04b25ffc",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/5b7242135ecbeac0cfaec17ad06a287b9398460e"
        },
        "date": 1779621112553,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3316.1175645939534,
            "unit": "iter/sec",
            "range": "stddev: 0.00003036199408907635",
            "extra": "mean: 301.55746306372174 usec\nrounds: 1110"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2144.495279086955,
            "unit": "iter/sec",
            "range": "stddev: 0.00004823498015662573",
            "extra": "mean: 466.31018951264014 usec\nrounds: 1087"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1073.0928572919224,
            "unit": "iter/sec",
            "range": "stddev: 0.00008309419771745322",
            "extra": "mean: 931.8858039215909 usec\nrounds: 714"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 457.5555907256289,
            "unit": "iter/sec",
            "range": "stddev: 0.00029301018526614863",
            "extra": "mean: 2.185526786841613 msec\nrounds: 380"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1775.3857010462468,
            "unit": "iter/sec",
            "range": "stddev: 0.00014083460749215326",
            "extra": "mean: 563.2578877990811 usec\nrounds: 1631"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1809.0248174671958,
            "unit": "iter/sec",
            "range": "stddev: 0.0001596909798561517",
            "extra": "mean: 552.7840139861064 usec\nrounds: 1144"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1804.7057532123524,
            "unit": "iter/sec",
            "range": "stddev: 0.00013845566001156003",
            "extra": "mean: 554.1069496897281 usec\nrounds: 1451"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1982.1622737576297,
            "unit": "iter/sec",
            "range": "stddev: 0.000059319840870022875",
            "extra": "mean: 504.49956254301895 usec\nrounds: 1447"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1732.0238305112578,
            "unit": "iter/sec",
            "range": "stddev: 0.00005471099954766589",
            "extra": "mean: 577.3592616822256 usec\nrounds: 535"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1444.7526354282381,
            "unit": "iter/sec",
            "range": "stddev: 0.00008185512392934137",
            "extra": "mean: 692.1600109790356 usec\nrounds: 1093"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2630.170263369783,
            "unit": "iter/sec",
            "range": "stddev: 0.00004935006354434037",
            "extra": "mean: 380.2035229152035 usec\nrounds: 1331"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 159.65979453773065,
            "unit": "iter/sec",
            "range": "stddev: 0.00013979553606370844",
            "extra": "mean: 6.263317592856359 msec\nrounds: 140"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "54474ab2e55b1226e422ad83737acc87d2289087",
          "message": "feat(sdk): explicit-fields update so a field can be set to its zero value (#574) (#583)\n\nThe typed update path sends only the SET (non-default) fields of the\nmessage, because proto3 omits zero-valued scalars on the wire. So a\nfield could never be updated TO its zero value (false / 0 / \"\") through\nthe typed API — the patch simply omitted it and the change was silently\ndropped. Confirmed downstream: a TotpCredential.verified true→false\nupdate was a no-op.\n\nBoth SDKs gain an explicit-fields update that names which fields to\ninclude, read off the message via reflection (so zeros are included):\n\n  - Go: Plan.UpdateFields(nodeID, msg, fields...) — builds the patch from\n    the named fields via protoreflect Get (returns the zero), not Range.\n  - Python: plan.update(node_id, msg, fields=[...]) — builds the patch\n    from the named fields directly, not ListFields.\n\nServer-side this needs no change: the applier's update merge applies\nevery patch entry verbatim (`merged[k] = v`), zeros included, and the\ntyped wire value (ADR-028) carries 0/\"\"/false losslessly. The fix is\npurely the SDK no longer dropping the field before it reaches the wire.\n\nBoth SDKs ship together.\n\nTests: Go (UpdateFields includes a zero-valued field in the patch while\nplain Update omits it; unknown-field and no-fields panic) and Python\n(update(fields=[...]) emits {\"3\": 0}; default update omits it; unknown\nfield raises UnknownFieldError).\n\nCloses #574.",
          "timestamp": "2026-05-24T12:29:45+01:00",
          "tree_id": "97efbde22d8eafde048a3613a933ac490c13641b",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/54474ab2e55b1226e422ad83737acc87d2289087"
        },
        "date": 1779622289821,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3124.0364836824433,
            "unit": "iter/sec",
            "range": "stddev: 0.00004641871515674111",
            "extra": "mean: 320.0986945009217 usec\nrounds: 1473"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2090.6626879948008,
            "unit": "iter/sec",
            "range": "stddev: 0.0000459419130494693",
            "extra": "mean: 478.31723679878814 usec\nrounds: 1212"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1001.0293946675085,
            "unit": "iter/sec",
            "range": "stddev: 0.00008232126446261984",
            "extra": "mean: 998.971663896193 usec\nrounds: 842"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 464.5744232314334,
            "unit": "iter/sec",
            "range": "stddev: 0.00016141296177317113",
            "extra": "mean: 2.152507650000004 msec\nrounds: 400"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2002.5250865403818,
            "unit": "iter/sec",
            "range": "stddev: 0.00007957359069952359",
            "extra": "mean: 499.3695243676712 usec\nrounds: 1621"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1987.180668739372,
            "unit": "iter/sec",
            "range": "stddev: 0.0000765299226788465",
            "extra": "mean: 503.225507238041 usec\nrounds: 1796"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2090.393071957573,
            "unit": "iter/sec",
            "range": "stddev: 0.00006103735890230132",
            "extra": "mean: 478.3789295013011 usec\nrounds: 1844"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2056.010380949419,
            "unit": "iter/sec",
            "range": "stddev: 0.00005215559958096997",
            "extra": "mean: 486.378867181703 usec\nrounds: 1295"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1908.1553711927686,
            "unit": "iter/sec",
            "range": "stddev: 0.000043615654016612976",
            "extra": "mean: 524.0663391969545 usec\nrounds: 398"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1606.5081227528772,
            "unit": "iter/sec",
            "range": "stddev: 0.00005511356164569143",
            "extra": "mean: 622.4680633960455 usec\nrounds: 1325"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2673.242468750641,
            "unit": "iter/sec",
            "range": "stddev: 0.00002763633753829756",
            "extra": "mean: 374.0775525189667 usec\nrounds: 1171"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 138.02847160003645,
            "unit": "iter/sec",
            "range": "stddev: 0.0002060266585972378",
            "extra": "mean: 7.2448820769217 msec\nrounds: 130"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "54474ab2e55b1226e422ad83737acc87d2289087",
          "message": "feat(sdk): explicit-fields update so a field can be set to its zero value (#574) (#583)\n\nThe typed update path sends only the SET (non-default) fields of the\nmessage, because proto3 omits zero-valued scalars on the wire. So a\nfield could never be updated TO its zero value (false / 0 / \"\") through\nthe typed API — the patch simply omitted it and the change was silently\ndropped. Confirmed downstream: a TotpCredential.verified true→false\nupdate was a no-op.\n\nBoth SDKs gain an explicit-fields update that names which fields to\ninclude, read off the message via reflection (so zeros are included):\n\n  - Go: Plan.UpdateFields(nodeID, msg, fields...) — builds the patch from\n    the named fields via protoreflect Get (returns the zero), not Range.\n  - Python: plan.update(node_id, msg, fields=[...]) — builds the patch\n    from the named fields directly, not ListFields.\n\nServer-side this needs no change: the applier's update merge applies\nevery patch entry verbatim (`merged[k] = v`), zeros included, and the\ntyped wire value (ADR-028) carries 0/\"\"/false losslessly. The fix is\npurely the SDK no longer dropping the field before it reaches the wire.\n\nBoth SDKs ship together.\n\nTests: Go (UpdateFields includes a zero-valued field in the patch while\nplain Update omits it; unknown-field and no-fields panic) and Python\n(update(fields=[...]) emits {\"3\": 0}; default update omits it; unknown\nfield raises UnknownFieldError).\n\nCloses #574.",
          "timestamp": "2026-05-24T12:29:45+01:00",
          "tree_id": "97efbde22d8eafde048a3613a933ac490c13641b",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/54474ab2e55b1226e422ad83737acc87d2289087"
        },
        "date": 1779622295780,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3334.9288582227086,
            "unit": "iter/sec",
            "range": "stddev: 0.000029189157492237663",
            "extra": "mean: 299.8564714609631 usec\nrounds: 1349"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2090.2565036028986,
            "unit": "iter/sec",
            "range": "stddev: 0.00005016008933597038",
            "extra": "mean: 478.4101847195962 usec\nrounds: 1034"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1021.757371093785,
            "unit": "iter/sec",
            "range": "stddev: 0.00012005831117137783",
            "extra": "mean: 978.705931849071 usec\nrounds: 763"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 464.0625205791205,
            "unit": "iter/sec",
            "range": "stddev: 0.00029389015456028286",
            "extra": "mean: 2.154882059322661 msec\nrounds: 354"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1925.9518118725214,
            "unit": "iter/sec",
            "range": "stddev: 0.00008182835514820371",
            "extra": "mean: 519.2237904580501 usec\nrounds: 1551"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1949.8243270844691,
            "unit": "iter/sec",
            "range": "stddev: 0.00009225565459812445",
            "extra": "mean: 512.866716303247 usec\nrounds: 1558"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2036.3846568816357,
            "unit": "iter/sec",
            "range": "stddev: 0.00007564578561757917",
            "extra": "mean: 491.0663595017082 usec\nrounds: 1847"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2038.5903908615094,
            "unit": "iter/sec",
            "range": "stddev: 0.00004600967287738996",
            "extra": "mean: 490.5350307166902 usec\nrounds: 1465"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1918.8219806512,
            "unit": "iter/sec",
            "range": "stddev: 0.00003678436738806529",
            "extra": "mean: 521.1530877192814 usec\nrounds: 399"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1587.085636589451,
            "unit": "iter/sec",
            "range": "stddev: 0.00005797247947378829",
            "extra": "mean: 630.0857225001029 usec\nrounds: 1200"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2651.82675561521,
            "unit": "iter/sec",
            "range": "stddev: 0.000047828510887169216",
            "extra": "mean: 377.0985408011713 usec\nrounds: 1348"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 136.41739497823949,
            "unit": "iter/sec",
            "range": "stddev: 0.0003806857391024168",
            "extra": "mean: 7.330443453780322 msec\nrounds: 119"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "f4e8df824cba481b8c7998ceb89ce0eaa7955e93",
          "message": "feat(wal): Kafka SASL (PLAIN/SCRAM) + TLS support (#569) (#584)\n\nThe Kafka WAL backend only spoke plaintext, so it could not connect to\nany authenticated broker — Confluent Cloud, MSK, self-managed Kafka with\nauth, or Azure Event Hubs over the Kafka protocol. Adds transport\nsecurity applied to both the producer and consumer connections:\n\n  - TLS (the \"SSL\" in SASL_SSL): system roots by default (managed\n    brokers), optional private-CA bundle, optional client certificate\n    for mutual TLS, and an insecure-skip-verify testing knob.\n  - SASL: PLAIN (Confluent Cloud / Event Hubs), and SCRAM-SHA-256 /\n    SCRAM-SHA-512 (self-managed, MSK SCRAM) via an xdg-go/scram client\n    adapter for sarama's SCRAMClient interface.\n\nKafkaConfig gains TLS*/SASL* fields; producerConfig/consumerConfig now\nreturn an error and route through a shared applySecurity so producer and\nconsumer auth can never drift. The server exposes --wal-kafka-tls[-*]\nand --wal-kafka-sasl-{mechanism,username,password} flags (documented in\ndocs/deployment.md). Default (no flags) is byte-for-byte the old\nplaintext behavior — applySecurity is a no-op — so existing deployments\nand the redpanda e2e stack are unaffected.\n\nTests: applySecurity maps PLAIN / SCRAM-256 / SCRAM-512 / default-PLAIN\ncorrectly, rejects an unknown mechanism, enables TLS with\ninsecure-skip-verify, is a no-op when nothing is requested; buildTLSConfig\nerrors on a missing/invalid CA file and on a half-specified client cert.\n\nCloses #569.",
          "timestamp": "2026-05-24T21:08:37+01:00",
          "tree_id": "474929715093a91cf235030609772023d43376bf",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/f4e8df824cba481b8c7998ceb89ce0eaa7955e93"
        },
        "date": 1779653421546,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3193.8280721513806,
            "unit": "iter/sec",
            "range": "stddev: 0.00002984183114990651",
            "extra": "mean: 313.10389207218486 usec\nrounds: 1501"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2123.1225487478077,
            "unit": "iter/sec",
            "range": "stddev: 0.000040412607654084337",
            "extra": "mean: 471.0043707037957 usec\nrounds: 1222"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 984.9702007435668,
            "unit": "iter/sec",
            "range": "stddev: 0.00009684002939962165",
            "extra": "mean: 1.0152591410837475 msec\nrounds: 886"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 466.10425930299215,
            "unit": "iter/sec",
            "range": "stddev: 0.00011631331288979661",
            "extra": "mean: 2.1454427417063093 msec\nrounds: 422"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1850.4650438884744,
            "unit": "iter/sec",
            "range": "stddev: 0.0001040149136012763",
            "extra": "mean: 540.404696269566 usec\nrounds: 1689"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1824.8150549665593,
            "unit": "iter/sec",
            "range": "stddev: 0.00012027455893130789",
            "extra": "mean: 548.0007397343209 usec\nrounds: 1583"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1907.4311506138442,
            "unit": "iter/sec",
            "range": "stddev: 0.00012754010702845016",
            "extra": "mean: 524.2653186607458 usec\nrounds: 1613"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1951.8657762373514,
            "unit": "iter/sec",
            "range": "stddev: 0.00007364739545304808",
            "extra": "mean: 512.3303109129353 usec\nrounds: 1457"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1760.5565879645312,
            "unit": "iter/sec",
            "range": "stddev: 0.000033093642824116186",
            "extra": "mean: 568.0021913729855 usec\nrounds: 371"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1504.6857598649071,
            "unit": "iter/sec",
            "range": "stddev: 0.000056502419109713144",
            "extra": "mean: 664.5905920514468 usec\nrounds: 1233"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2652.0024295809144,
            "unit": "iter/sec",
            "range": "stddev: 0.00003505598946186627",
            "extra": "mean: 377.073561036679 usec\nrounds: 1196"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 138.55554193523272,
            "unit": "iter/sec",
            "range": "stddev: 0.00014151507541375235",
            "extra": "mean: 7.2173222812512705 msec\nrounds: 128"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "f4e8df824cba481b8c7998ceb89ce0eaa7955e93",
          "message": "feat(wal): Kafka SASL (PLAIN/SCRAM) + TLS support (#569) (#584)\n\nThe Kafka WAL backend only spoke plaintext, so it could not connect to\nany authenticated broker — Confluent Cloud, MSK, self-managed Kafka with\nauth, or Azure Event Hubs over the Kafka protocol. Adds transport\nsecurity applied to both the producer and consumer connections:\n\n  - TLS (the \"SSL\" in SASL_SSL): system roots by default (managed\n    brokers), optional private-CA bundle, optional client certificate\n    for mutual TLS, and an insecure-skip-verify testing knob.\n  - SASL: PLAIN (Confluent Cloud / Event Hubs), and SCRAM-SHA-256 /\n    SCRAM-SHA-512 (self-managed, MSK SCRAM) via an xdg-go/scram client\n    adapter for sarama's SCRAMClient interface.\n\nKafkaConfig gains TLS*/SASL* fields; producerConfig/consumerConfig now\nreturn an error and route through a shared applySecurity so producer and\nconsumer auth can never drift. The server exposes --wal-kafka-tls[-*]\nand --wal-kafka-sasl-{mechanism,username,password} flags (documented in\ndocs/deployment.md). Default (no flags) is byte-for-byte the old\nplaintext behavior — applySecurity is a no-op — so existing deployments\nand the redpanda e2e stack are unaffected.\n\nTests: applySecurity maps PLAIN / SCRAM-256 / SCRAM-512 / default-PLAIN\ncorrectly, rejects an unknown mechanism, enables TLS with\ninsecure-skip-verify, is a no-op when nothing is requested; buildTLSConfig\nerrors on a missing/invalid CA file and on a half-specified client cert.\n\nCloses #569.",
          "timestamp": "2026-05-24T21:08:37+01:00",
          "tree_id": "474929715093a91cf235030609772023d43376bf",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/f4e8df824cba481b8c7998ceb89ce0eaa7955e93"
        },
        "date": 1779653425761,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3249.8844159461664,
            "unit": "iter/sec",
            "range": "stddev: 0.000026778968869363007",
            "extra": "mean: 307.70325095049924 usec\nrounds: 1578"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2132.6579477819273,
            "unit": "iter/sec",
            "range": "stddev: 0.00004138404753342494",
            "extra": "mean: 468.89844714200456 usec\nrounds: 1277"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 991.2957576480588,
            "unit": "iter/sec",
            "range": "stddev: 0.0000922366435297982",
            "extra": "mean: 1.0087806714442042 msec\nrounds: 907"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 468.7195321840575,
            "unit": "iter/sec",
            "range": "stddev: 0.00013177326873438434",
            "extra": "mean: 2.133472004762367 msec\nrounds: 420"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1996.736344209705,
            "unit": "iter/sec",
            "range": "stddev: 0.00007622886680586187",
            "extra": "mean: 500.81724755493104 usec\nrounds: 1636"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1974.5237645030493,
            "unit": "iter/sec",
            "range": "stddev: 0.00008378313839488921",
            "extra": "mean: 506.4512354712941 usec\nrounds: 2323"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2080.2753733504214,
            "unit": "iter/sec",
            "range": "stddev: 0.00007431484752493047",
            "extra": "mean: 480.7055896592352 usec\nrounds: 1818"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1794.0921345846753,
            "unit": "iter/sec",
            "range": "stddev: 0.00025076219745351076",
            "extra": "mean: 557.3849752323316 usec\nrounds: 323"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1730.6599969872143,
            "unit": "iter/sec",
            "range": "stddev: 0.00006527033133055113",
            "extra": "mean: 577.8142452826265 usec\nrounds: 318"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1422.1080458612319,
            "unit": "iter/sec",
            "range": "stddev: 0.00008246534002925442",
            "extra": "mean: 703.1814515854157 usec\nrounds: 1167"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2567.972809572352,
            "unit": "iter/sec",
            "range": "stddev: 0.00005988717795279644",
            "extra": "mean: 389.412222852364 usec\nrounds: 1234"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 111.11680797770413,
            "unit": "iter/sec",
            "range": "stddev: 0.002096275915121311",
            "extra": "mean: 8.99953857746393 msec\nrounds: 71"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "258f783159d5bed9dcd737a3d4fde08f552376d6",
          "message": "fix(apply): fail-stop on corrupt acl_blob instead of silently dropping all grants (#582) (#585)\n\nTransferOwnership read the node's existing acl_blob and unmarshaled it\nwith the error discarded. refreshVisibility then DELETEs the node's\nvisibility rows and re-inserts owner + each ACL entry — so a corrupt\nacl_blob rebuilt the index from an EMPTY ACL, silently revoking every\nshared grant on the node. Now the decode error is propagated, so the\napplier halts (halt-on-poison) on a corrupt blob rather than masking it\nas a mass-revoke. The transfer does not apply and the owner is unchanged.\n\nAudited the same class in acladapter.GrantsForNode: the typed-cap arrays\n(core_caps / ext_caps) were also decoded with `err == nil` ignores,\nsilently dropping caps and mis-evaluating access. Those now fail closed\n(return the error) too.\n\nTest: seed a node with an ACL grant, corrupt its acl_blob via a second\nconnection, append a TransferOwnership op, and assert the applier halts\n(non-nil) with the owner unchanged — i.e. grants are not silently dropped.\n\nCloses #582.",
          "timestamp": "2026-05-24T21:28:51+01:00",
          "tree_id": "41324655bdfed5628f88357ef95790e1ece59483",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/258f783159d5bed9dcd737a3d4fde08f552376d6"
        },
        "date": 1779654637469,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3320.0617334526846,
            "unit": "iter/sec",
            "range": "stddev: 0.00008398378546784192",
            "extra": "mean: 301.19921865430314 usec\nrounds: 1308"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2130.025486527792,
            "unit": "iter/sec",
            "range": "stddev: 0.00004922181089248073",
            "extra": "mean: 469.47795053388074 usec\nrounds: 1031"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 995.1720603518067,
            "unit": "iter/sec",
            "range": "stddev: 0.00011326979856490625",
            "extra": "mean: 1.0048513617298365 msec\nrounds: 763"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 468.55110510901415,
            "unit": "iter/sec",
            "range": "stddev: 0.0001361021056885842",
            "extra": "mean: 2.134238910326202 msec\nrounds: 368"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1914.5428077080328,
            "unit": "iter/sec",
            "range": "stddev: 0.00009229221786762134",
            "extra": "mean: 522.317911082456 usec\nrounds: 1597"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1922.6158807192007,
            "unit": "iter/sec",
            "range": "stddev: 0.00010996803388820407",
            "extra": "mean: 520.1246957483394 usec\nrounds: 1811"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1970.4473834237476,
            "unit": "iter/sec",
            "range": "stddev: 0.0000931230171765922",
            "extra": "mean: 507.49896110519416 usec\nrounds: 1594"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2009.7531196762823,
            "unit": "iter/sec",
            "range": "stddev: 0.00004604704310904394",
            "extra": "mean: 497.5735527959143 usec\nrounds: 1288"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1881.75226738573,
            "unit": "iter/sec",
            "range": "stddev: 0.00008105597228790595",
            "extra": "mean: 531.4195802135391 usec\nrounds: 374"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1583.67232560425,
            "unit": "iter/sec",
            "range": "stddev: 0.00006490466462077173",
            "extra": "mean: 631.4437550194924 usec\nrounds: 1245"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2646.8334961700502,
            "unit": "iter/sec",
            "range": "stddev: 0.00002809530956188083",
            "extra": "mean: 377.80993834594926 usec\nrounds: 1330"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 125.23080332163559,
            "unit": "iter/sec",
            "range": "stddev: 0.0008230536193814741",
            "extra": "mean: 7.98525581147681 msec\nrounds: 122"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "258f783159d5bed9dcd737a3d4fde08f552376d6",
          "message": "fix(apply): fail-stop on corrupt acl_blob instead of silently dropping all grants (#582) (#585)\n\nTransferOwnership read the node's existing acl_blob and unmarshaled it\nwith the error discarded. refreshVisibility then DELETEs the node's\nvisibility rows and re-inserts owner + each ACL entry — so a corrupt\nacl_blob rebuilt the index from an EMPTY ACL, silently revoking every\nshared grant on the node. Now the decode error is propagated, so the\napplier halts (halt-on-poison) on a corrupt blob rather than masking it\nas a mass-revoke. The transfer does not apply and the owner is unchanged.\n\nAudited the same class in acladapter.GrantsForNode: the typed-cap arrays\n(core_caps / ext_caps) were also decoded with `err == nil` ignores,\nsilently dropping caps and mis-evaluating access. Those now fail closed\n(return the error) too.\n\nTest: seed a node with an ACL grant, corrupt its acl_blob via a second\nconnection, append a TransferOwnership op, and assert the applier halts\n(non-nil) with the owner unchanged — i.e. grants are not silently dropped.\n\nCloses #582.",
          "timestamp": "2026-05-24T21:28:51+01:00",
          "tree_id": "41324655bdfed5628f88357ef95790e1ece59483",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/258f783159d5bed9dcd737a3d4fde08f552376d6"
        },
        "date": 1779654646256,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3221.3546592413486,
            "unit": "iter/sec",
            "range": "stddev: 0.00003255918145667465",
            "extra": "mean: 310.42840847443574 usec\nrounds: 1180"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2088.8544124750333,
            "unit": "iter/sec",
            "range": "stddev: 0.000049491314834709394",
            "extra": "mean: 478.7313055557204 usec\nrounds: 1152"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 938.392920301735,
            "unit": "iter/sec",
            "range": "stddev: 0.00011442797773817714",
            "extra": "mean: 1.0656516885042735 msec\nrounds: 809"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 448.61080514472803,
            "unit": "iter/sec",
            "range": "stddev: 0.00015935325755635826",
            "extra": "mean: 2.229103687498981 msec\nrounds: 368"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1868.497924714883,
            "unit": "iter/sec",
            "range": "stddev: 0.00011725626961447109",
            "extra": "mean: 535.1892484186684 usec\nrounds: 1739"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1771.9415310951188,
            "unit": "iter/sec",
            "range": "stddev: 0.0001733582113225558",
            "extra": "mean: 564.3527071584393 usec\nrounds: 1844"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1849.9093631829003,
            "unit": "iter/sec",
            "range": "stddev: 0.0001363264742551752",
            "extra": "mean: 540.5670244727174 usec\nrounds: 1185"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1819.5083208917601,
            "unit": "iter/sec",
            "range": "stddev: 0.00017216659939389063",
            "extra": "mean: 549.5990254718316 usec\nrounds: 1060"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1634.6827194463465,
            "unit": "iter/sec",
            "range": "stddev: 0.00009662199876215146",
            "extra": "mean: 611.7395064521706 usec\nrounds: 310"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1439.3899088750961,
            "unit": "iter/sec",
            "range": "stddev: 0.00006208479581403432",
            "extra": "mean: 694.7387874780326 usec\nrounds: 1134"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2601.138187879365,
            "unit": "iter/sec",
            "range": "stddev: 0.00003480705435452445",
            "extra": "mean: 384.4470873019138 usec\nrounds: 1008"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 126.31640992869438,
            "unit": "iter/sec",
            "range": "stddev: 0.00038311382162236403",
            "extra": "mean: 7.916627780701652 msec\nrounds: 114"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "e7b144bd343adbc5c7102fc8fd5f891d3797fae3",
          "message": "test(sdk-go): integration suite driving the Go SDK against a real entdb-server (#586)\n\nThe Go SDK had no end-to-end coverage: its unit tests run against an\nin-process fakeServer with canned responses, and the contract/integration\nsuite is Python-only (it boots the real server but drives it through the\nPython SDK). So every Go SDK wire change — typed int64, keyset\nauto-follow, unique-key lookup, zero-value patches — was only ever\nchecked against fakes that encode my understanding of the server, never\nthe server itself. And CI ran no Go SDK tests at all (only server/go).\n\nAdds (behind a `//go:build integration` tag so the fast unit path is\nunchanged) a TestMain that builds and boots the actual\nserver/go/cmd/entdb-server on a free port (e2e seed profile, memory WAL)\nand drives the Go SDK transport against it over real gRPC — the mirror of\nthe Python conftest. Tests:\n\n  - typed int64 > 2^53 round-trips losslessly (ADR-028 / #563 / #572);\n  - QueryNodes auto-follows the real keyset cursor over 250 rows with no\n    truncation (ADR-029 / #564);\n  - GetNodeByKey unique-key lookup (#572 typed value);\n  - an explicit zero-value patch sets the field (the wire shape #574's\n    UpdateFields emits), not a no-op.\n\nCI: new `go-sdk` job runs the SDK vet + unit tests AND this integration\nsuite, and is now a required check. Previously the Go SDK was untested in\nCI despite the `sdk/go/**` path filter.",
          "timestamp": "2026-05-24T21:39:00+01:00",
          "tree_id": "19b7c292be8d36013ef2afc1d2d93cef1965cbc3",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/e7b144bd343adbc5c7102fc8fd5f891d3797fae3"
        },
        "date": 1779655243174,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3333.300795656289,
            "unit": "iter/sec",
            "range": "stddev: 0.00003078605593146915",
            "extra": "mean: 300.00292841951915 usec\nrounds: 1411"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2163.6785055203345,
            "unit": "iter/sec",
            "range": "stddev: 0.00003840898417958441",
            "extra": "mean: 462.175871992366 usec\nrounds: 1164"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1070.2774284546658,
            "unit": "iter/sec",
            "range": "stddev: 0.0000891953943312915",
            "extra": "mean: 934.3371853070499 usec\nrounds: 912"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 472.3137845513703,
            "unit": "iter/sec",
            "range": "stddev: 0.00013806815963305304",
            "extra": "mean: 2.117236533652845 msec\nrounds: 416"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1873.61892945796,
            "unit": "iter/sec",
            "range": "stddev: 0.00011558312493288174",
            "extra": "mean: 533.7264607426341 usec\nrounds: 1643"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1813.077524259788,
            "unit": "iter/sec",
            "range": "stddev: 0.00015780155302694583",
            "extra": "mean: 551.5483958184649 usec\nrounds: 1435"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1771.9689502179663,
            "unit": "iter/sec",
            "range": "stddev: 0.0001548641013059287",
            "extra": "mean: 564.3439744680583 usec\nrounds: 1645"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1960.867065541529,
            "unit": "iter/sec",
            "range": "stddev: 0.00005923245029119282",
            "extra": "mean: 509.97847716098585 usec\nrounds: 1423"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1667.5311855518082,
            "unit": "iter/sec",
            "range": "stddev: 0.00006618035379911917",
            "extra": "mean: 599.6889345545204 usec\nrounds: 382"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1494.1758442057987,
            "unit": "iter/sec",
            "range": "stddev: 0.000050274812432589485",
            "extra": "mean: 669.2652701339389 usec\nrounds: 1192"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2650.8179829084834,
            "unit": "iter/sec",
            "range": "stddev: 0.000027330526378513837",
            "extra": "mean: 377.2420462089961 usec\nrounds: 1385"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 146.0676153121218,
            "unit": "iter/sec",
            "range": "stddev: 0.0001454548498035093",
            "extra": "mean: 6.846144491803806 msec\nrounds: 122"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "5a1291ed07f7d442a4cdbb1748a46b06e0e2d46a",
          "message": "feat(#580): keyset cursor for GetEdgesFrom/GetEdgesTo + SDK auto-follow (#587)\n\n* feat(edges): keyset cursor pagination for GetEdgesFrom/GetEdgesTo + SDK auto-follow (#580)\n\nEdge reads truncated at the page default with no cursor (Bug A class): a\nhigh-fan-out node's edges came back as a prefix with has_more=true and no\nway to fetch the rest. Extends the ADR-029 keyset cursor (shipped for\nQueryNodes in v1.23/v1.24) to both edge RPCs.\n\nServer: GetEdgesRequest gains page_size + page_token, GetEdgesResponse\ngains next_page_token (offset deprecated). The store seeks over the total\norder (created_at DESC, edge_type_id DESC, peer DESC) — peer is\nto_node_id outgoing / from_node_id incoming, which uniquely identifies an\nedge for a fixed source/target, so pages never skip or duplicate. The\ntoken is fingerprint-bound to (node_id, direction, edge_type_id);\nmismatched or offset-mixed tokens are INVALID_ARGUMENT. next_page_token is\nminted from the last edge, so a response that omits edges always carries\na cursor.\n\nSDKs (both, together): Go transport.GetEdgesFrom/To and Python\nget_edges_from/get_edges_to auto-follow the cursor to return the COMPLETE\nset by default; Python `limit` defaults to 0 (= all), a positive value\ncaps. Shared helpers (getEdgesPaged / _get_edges_paged) back both\ndirections.\n\nTests: server keyset paging (all edges, no dup), cross-query and\ntoken+offset rejection; a real-server Go integration test\n(TestIntegration_GetEdgesFromAutoFollowsRealCursor) seeds 150 edges and\nasserts auto-follow returns all of them.\n\nRefs #580, ADR-029.\n\n* style: ruff-format _grpc_client.py (edge auto-follow helper)",
          "timestamp": "2026-05-24T22:00:38+01:00",
          "tree_id": "feae7570f69750dd37280c6818299b1c9602a39a",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/5a1291ed07f7d442a4cdbb1748a46b06e0e2d46a"
        },
        "date": 1779656534423,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 4300.1062315467225,
            "unit": "iter/sec",
            "range": "stddev: 0.00002441706458569564",
            "extra": "mean: 232.55239432545503 usec\nrounds: 1339"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2808.0343870176175,
            "unit": "iter/sec",
            "range": "stddev: 0.00003873057627695785",
            "extra": "mean: 356.12099503599353 usec\nrounds: 1410"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1356.3988578329313,
            "unit": "iter/sec",
            "range": "stddev: 0.00009301151707608039",
            "extra": "mean: 737.2462710545653 usec\nrounds: 1140"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 519.7735584556526,
            "unit": "iter/sec",
            "range": "stddev: 0.00038104543107731327",
            "extra": "mean: 1.9239147196544448 msec\nrounds: 346"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2532.1625936587934,
            "unit": "iter/sec",
            "range": "stddev: 0.00010528566372304126",
            "extra": "mean: 394.9193477955425 usec\nrounds: 1498"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2618.5702074833534,
            "unit": "iter/sec",
            "range": "stddev: 0.00007614519970017921",
            "extra": "mean: 381.8877940114795 usec\nrounds: 2471"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2813.6466014180637,
            "unit": "iter/sec",
            "range": "stddev: 0.000052764811652359396",
            "extra": "mean: 355.4106615578534 usec\nrounds: 2219"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 884.8260741620192,
            "unit": "iter/sec",
            "range": "stddev: 0.003578204245469889",
            "extra": "mean: 1.1301656101704023 msec\nrounds: 59"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 675.3062433913644,
            "unit": "iter/sec",
            "range": "stddev: 0.005091731433808288",
            "extra": "mean: 1.480809647158058 msec\nrounds: 598"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 552.6997415390961,
            "unit": "iter/sec",
            "range": "stddev: 0.004930316920382222",
            "extra": "mean: 1.8093006470661857 msec\nrounds: 34"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 3308.189526991058,
            "unit": "iter/sec",
            "range": "stddev: 0.000034013430726758305",
            "extra": "mean: 302.28014200551064 usec\nrounds: 1507"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 132.29582610111206,
            "unit": "iter/sec",
            "range": "stddev: 0.00015403915534673644",
            "extra": "mean: 7.558817458350593 msec\nrounds: 24"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "5a1291ed07f7d442a4cdbb1748a46b06e0e2d46a",
          "message": "feat(#580): keyset cursor for GetEdgesFrom/GetEdgesTo + SDK auto-follow (#587)\n\n* feat(edges): keyset cursor pagination for GetEdgesFrom/GetEdgesTo + SDK auto-follow (#580)\n\nEdge reads truncated at the page default with no cursor (Bug A class): a\nhigh-fan-out node's edges came back as a prefix with has_more=true and no\nway to fetch the rest. Extends the ADR-029 keyset cursor (shipped for\nQueryNodes in v1.23/v1.24) to both edge RPCs.\n\nServer: GetEdgesRequest gains page_size + page_token, GetEdgesResponse\ngains next_page_token (offset deprecated). The store seeks over the total\norder (created_at DESC, edge_type_id DESC, peer DESC) — peer is\nto_node_id outgoing / from_node_id incoming, which uniquely identifies an\nedge for a fixed source/target, so pages never skip or duplicate. The\ntoken is fingerprint-bound to (node_id, direction, edge_type_id);\nmismatched or offset-mixed tokens are INVALID_ARGUMENT. next_page_token is\nminted from the last edge, so a response that omits edges always carries\na cursor.\n\nSDKs (both, together): Go transport.GetEdgesFrom/To and Python\nget_edges_from/get_edges_to auto-follow the cursor to return the COMPLETE\nset by default; Python `limit` defaults to 0 (= all), a positive value\ncaps. Shared helpers (getEdgesPaged / _get_edges_paged) back both\ndirections.\n\nTests: server keyset paging (all edges, no dup), cross-query and\ntoken+offset rejection; a real-server Go integration test\n(TestIntegration_GetEdgesFromAutoFollowsRealCursor) seeds 150 edges and\nasserts auto-follow returns all of them.\n\nRefs #580, ADR-029.\n\n* style: ruff-format _grpc_client.py (edge auto-follow helper)",
          "timestamp": "2026-05-24T22:00:38+01:00",
          "tree_id": "feae7570f69750dd37280c6818299b1c9602a39a",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/5a1291ed07f7d442a4cdbb1748a46b06e0e2d46a"
        },
        "date": 1779656542580,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3243.838726911229,
            "unit": "iter/sec",
            "range": "stddev: 0.000025109764468453628",
            "extra": "mean: 308.27673142437516 usec\nrounds: 1467"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2148.533887577648,
            "unit": "iter/sec",
            "range": "stddev: 0.000039169224955130665",
            "extra": "mean: 465.43366422181225 usec\nrounds: 1227"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 997.3568787214942,
            "unit": "iter/sec",
            "range": "stddev: 0.00009899230317488651",
            "extra": "mean: 1.002650125882617 msec\nrounds: 850"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 467.91124627873484,
            "unit": "iter/sec",
            "range": "stddev: 0.00011295582665981668",
            "extra": "mean: 2.137157437340798 msec\nrounds: 391"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1946.8925622198221,
            "unit": "iter/sec",
            "range": "stddev: 0.00011150743988807131",
            "extra": "mean: 513.6390263157679 usec\nrounds: 1596"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1817.8480005075553,
            "unit": "iter/sec",
            "range": "stddev: 0.0001379656263592861",
            "extra": "mean: 550.1009983897407 usec\nrounds: 1863"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1908.696677582389,
            "unit": "iter/sec",
            "range": "stddev: 0.00011031846834018031",
            "extra": "mean: 523.9177139799024 usec\nrounds: 1402"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1773.0371147014273,
            "unit": "iter/sec",
            "range": "stddev: 0.00008155394251985223",
            "extra": "mean: 564.003985990105 usec\nrounds: 1142"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1770.1081737023062,
            "unit": "iter/sec",
            "range": "stddev: 0.00006478567240440846",
            "extra": "mean: 564.9372252252977 usec\nrounds: 333"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1550.343543608067,
            "unit": "iter/sec",
            "range": "stddev: 0.00006479510710014122",
            "extra": "mean: 645.0183277912266 usec\nrounds: 1263"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2651.1723238393893,
            "unit": "iter/sec",
            "range": "stddev: 0.000028842252569108595",
            "extra": "mean: 377.19162613760784 usec\nrounds: 1209"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 136.47145795703432,
            "unit": "iter/sec",
            "range": "stddev: 0.00015622206503474565",
            "extra": "mean: 7.327539508772835 msec\nrounds: 114"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "b3c76aa918c3eee79fcb406446339f373568e732",
          "message": "feat(list-users): keyset cursor pagination for ListUsers + surface store faults (#580, #573) (#588)\n\nListUsers truncated at the 100-row default with NO has_more and NO cursor\n— the worst of the Bug A class — and swallowed genuine store faults into\nan empty list with codes.OK. This extends the ADR-029 keyset cursor to\nthe user registry and fixes the swallow.\n\nServer: ListUsersRequest gains page_size + page_token, ListUsersResponse\ngains next_page_token (offset deprecated). globalstore.ListUsersPaged\nseeks over the total order (created_at, user_id) — user_id is unique so\npages never skip or duplicate. The token is fingerprint-bound to the\nstatus filter; mismatched or offset-mixed tokens are INVALID_ARGUMENT.\nA genuine globalstore error now surfaces as sanitized codes.Internal\ninstead of empty+OK (#573-class swallow, called out as a parity wart in\nthe handler).\n\nPython SDK: list_users auto-follows the cursor to return the complete set\nby default; limit defaults to 0 (= all), a positive value caps, the\ndeprecated offset falls back to a single non-cursor request. The Go SDK\ndoes not expose ListUsers (admin RPC), so there is no Go-side change.\n\nTests: server keyset paging (all users, no dup), cross-filter token\nrejection, and the swallow test repointed to assert Internal; Python unit\ntests for auto-follow / limit cap / legacy offset.\n\nRefs #580, #573, ADR-029.",
          "timestamp": "2026-05-24T22:15:58+01:00",
          "tree_id": "bc8adfe809728ba66789cf6acf621541cc6050d3",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/b3c76aa918c3eee79fcb406446339f373568e732"
        },
        "date": 1779657467437,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3359.4422465846524,
            "unit": "iter/sec",
            "range": "stddev: 0.000023753188064038786",
            "extra": "mean: 297.6684600000614 usec\nrounds: 850"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2113.0944572032963,
            "unit": "iter/sec",
            "range": "stddev: 0.000048908205871079786",
            "extra": "mean: 473.2396115048785 usec\nrounds: 1130"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1024.1355960430192,
            "unit": "iter/sec",
            "range": "stddev: 0.00011385569212414714",
            "extra": "mean: 976.4332026576631 usec\nrounds: 903"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 474.8525661815876,
            "unit": "iter/sec",
            "range": "stddev: 0.00013877728490836185",
            "extra": "mean: 2.105916807065525 msec\nrounds: 368"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1912.9710213564686,
            "unit": "iter/sec",
            "range": "stddev: 0.0000896621011257884",
            "extra": "mean: 522.7470718771841 usec\nrounds: 1433"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1950.2150110930681,
            "unit": "iter/sec",
            "range": "stddev: 0.00008948027859264466",
            "extra": "mean: 512.7639743884005 usec\nrounds: 1718"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1978.055468228313,
            "unit": "iter/sec",
            "range": "stddev: 0.00008450073028938259",
            "extra": "mean: 505.54699605854387 usec\nrounds: 1776"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1672.2627014488933,
            "unit": "iter/sec",
            "range": "stddev: 0.00005201029730287332",
            "extra": "mean: 597.9921690136204 usec\nrounds: 1207"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1688.806216661046,
            "unit": "iter/sec",
            "range": "stddev: 0.00005342325020968568",
            "extra": "mean: 592.1342485208925 usec\nrounds: 338"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1443.2889173284052,
            "unit": "iter/sec",
            "range": "stddev: 0.00005145091347002828",
            "extra": "mean: 692.8619682405977 usec\nrounds: 1228"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2627.7935259246124,
            "unit": "iter/sec",
            "range": "stddev: 0.000031638360182679856",
            "extra": "mean: 380.54740227284077 usec\nrounds: 1320"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 148.7400202460793,
            "unit": "iter/sec",
            "range": "stddev: 0.00015239808651982018",
            "extra": "mean: 6.723140136363935 msec\nrounds: 132"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "b3c76aa918c3eee79fcb406446339f373568e732",
          "message": "feat(list-users): keyset cursor pagination for ListUsers + surface store faults (#580, #573) (#588)\n\nListUsers truncated at the 100-row default with NO has_more and NO cursor\n— the worst of the Bug A class — and swallowed genuine store faults into\nan empty list with codes.OK. This extends the ADR-029 keyset cursor to\nthe user registry and fixes the swallow.\n\nServer: ListUsersRequest gains page_size + page_token, ListUsersResponse\ngains next_page_token (offset deprecated). globalstore.ListUsersPaged\nseeks over the total order (created_at, user_id) — user_id is unique so\npages never skip or duplicate. The token is fingerprint-bound to the\nstatus filter; mismatched or offset-mixed tokens are INVALID_ARGUMENT.\nA genuine globalstore error now surfaces as sanitized codes.Internal\ninstead of empty+OK (#573-class swallow, called out as a parity wart in\nthe handler).\n\nPython SDK: list_users auto-follows the cursor to return the complete set\nby default; limit defaults to 0 (= all), a positive value caps, the\ndeprecated offset falls back to a single non-cursor request. The Go SDK\ndoes not expose ListUsers (admin RPC), so there is no Go-side change.\n\nTests: server keyset paging (all users, no dup), cross-filter token\nrejection, and the swallow test repointed to assert Internal; Python unit\ntests for auto-follow / limit cap / legacy offset.\n\nRefs #580, #573, ADR-029.",
          "timestamp": "2026-05-24T22:15:58+01:00",
          "tree_id": "bc8adfe809728ba66789cf6acf621541cc6050d3",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/b3c76aa918c3eee79fcb406446339f373568e732"
        },
        "date": 1779657470111,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3309.065800299459,
            "unit": "iter/sec",
            "range": "stddev: 0.000030735426436434846",
            "extra": "mean: 302.20009523821 usec\nrounds: 1239"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2137.0714119749814,
            "unit": "iter/sec",
            "range": "stddev: 0.000038249381824993904",
            "extra": "mean: 467.9300815108685 usec\nrounds: 1006"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1061.3959379718601,
            "unit": "iter/sec",
            "range": "stddev: 0.00009953133879172614",
            "extra": "mean: 942.1554805559396 usec\nrounds: 720"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 472.8718414656331,
            "unit": "iter/sec",
            "range": "stddev: 0.00014408053685976312",
            "extra": "mean: 2.1147378894471074 msec\nrounds: 398"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1914.6246142528935,
            "unit": "iter/sec",
            "range": "stddev: 0.0000859749050698461",
            "extra": "mean: 522.2955939016852 usec\nrounds: 1443"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1791.4272271768778,
            "unit": "iter/sec",
            "range": "stddev: 0.00017492252325974217",
            "extra": "mean: 558.2141349810267 usec\nrounds: 1578"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1811.5336482301843,
            "unit": "iter/sec",
            "range": "stddev: 0.00013743530664228645",
            "extra": "mean: 552.0184518664453 usec\nrounds: 1527"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1764.801110800991,
            "unit": "iter/sec",
            "range": "stddev: 0.000049874862080276655",
            "extra": "mean: 566.6360894039383 usec\nrounds: 604"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1767.0650788442945,
            "unit": "iter/sec",
            "range": "stddev: 0.00008803347002026764",
            "extra": "mean: 565.9101138787856 usec\nrounds: 281"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1431.7688345887896,
            "unit": "iter/sec",
            "range": "stddev: 0.00017995567325184038",
            "extra": "mean: 698.4367698485381 usec\nrounds: 1121"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2646.5185123161905,
            "unit": "iter/sec",
            "range": "stddev: 0.00003599772357427971",
            "extra": "mean: 377.8549046025059 usec\nrounds: 1195"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 142.86518243800342,
            "unit": "iter/sec",
            "range": "stddev: 0.00044713776896297284",
            "extra": "mean: 6.999606082706343 msec\nrounds: 133"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "871f96c09b9e7000ba9f6ccdd6efd91229ee15eb",
          "message": "feat(wal): durable Azure Blob checkpoint store for the Event Hubs backend (#570) (#589)\n\nThe Event Hubs backend kept per-partition checkpoints in memory, so a\nprocess restart replayed the hub from the configured start position\n(re-applying everything since). Adds an opt-in durable checkpoint store\nbacked by Azure Blob Storage: the per-partition sequence-number map is\npersisted on Commit and restored on Connect, so a restart resumes after\nthe last commit.\n\nDesign: a CheckpointStore interface (Load/Save the full map), wired into\nEventHubs — Connect loads, Commit saves under the existing lock. nil\nstore (the default) is byte-for-byte the prior in-memory behavior. The\nblob impl is split behind a tiny blobClient seam so its logic (marshal\nthe map, treat a missing blob as first-run) is unit-tested with a fake;\nonly the thin azblob upload/download wrapper is untested I/O, consistent\nwith the other cloud backends.\n\nServer flags (--wal-backend=eventhubs):\n  --wal-eventhubs-checkpoint-storage-connection-string (enables it)\n  --wal-eventhubs-checkpoint-container (default entdb-wal-checkpoints)\nOne blob per <hub>/<consumer-group> keeps distinct WALs from colliding.\nDocumented in docs/deployment.md.\n\nTests: blob store round-trip via a fake blobClient (incl. missing-blob\n⇒ empty map, and an int64 > 2^53 checkpoint); EventHubs restores\ncheckpoints on Connect and persists them on Commit via a fake store.\n\nCloses #570.",
          "timestamp": "2026-05-25T11:28:40+01:00",
          "tree_id": "9e78e54a76135d4a43192f7d958491f354ef53a2",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/871f96c09b9e7000ba9f6ccdd6efd91229ee15eb"
        },
        "date": 1779705028857,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3342.732942656115,
            "unit": "iter/sec",
            "range": "stddev: 0.00002986883546901769",
            "extra": "mean: 299.1564139746701 usec\nrounds: 1331"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2128.285203495533,
            "unit": "iter/sec",
            "range": "stddev: 0.00004590477656469232",
            "extra": "mean: 469.8618391734259 usec\nrounds: 1113"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1053.656935886613,
            "unit": "iter/sec",
            "range": "stddev: 0.00009392192342311202",
            "extra": "mean: 949.0755158922171 usec\nrounds: 818"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 403.11169027456725,
            "unit": "iter/sec",
            "range": "stddev: 0.00040492529263235166",
            "extra": "mean: 2.4807020588236486 msec\nrounds: 391"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1743.0525240735433,
            "unit": "iter/sec",
            "range": "stddev: 0.0001534594599790393",
            "extra": "mean: 573.7061770594171 usec\nrounds: 1299"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1810.1957446820709,
            "unit": "iter/sec",
            "range": "stddev: 0.00015346235385971585",
            "extra": "mean: 552.4264450061628 usec\nrounds: 1582"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1851.505636945465,
            "unit": "iter/sec",
            "range": "stddev: 0.0001425648435834014",
            "extra": "mean: 540.1009751446165 usec\nrounds: 1730"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1713.735982317507,
            "unit": "iter/sec",
            "range": "stddev: 0.0000523942128352048",
            "extra": "mean: 583.5204549114311 usec\nrounds: 1242"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1683.5164585265281,
            "unit": "iter/sec",
            "range": "stddev: 0.00005419443132129137",
            "extra": "mean: 593.9947868850861 usec\nrounds: 427"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1458.435709682973,
            "unit": "iter/sec",
            "range": "stddev: 0.00006994572516776433",
            "extra": "mean: 685.6661513159016 usec\nrounds: 1064"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2642.657105253645,
            "unit": "iter/sec",
            "range": "stddev: 0.00003603583820363483",
            "extra": "mean: 378.4070199694027 usec\nrounds: 1953"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 143.62681040894984,
            "unit": "iter/sec",
            "range": "stddev: 0.00023561343048858247",
            "extra": "mean: 6.962488390243378 msec\nrounds: 123"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "871f96c09b9e7000ba9f6ccdd6efd91229ee15eb",
          "message": "feat(wal): durable Azure Blob checkpoint store for the Event Hubs backend (#570) (#589)\n\nThe Event Hubs backend kept per-partition checkpoints in memory, so a\nprocess restart replayed the hub from the configured start position\n(re-applying everything since). Adds an opt-in durable checkpoint store\nbacked by Azure Blob Storage: the per-partition sequence-number map is\npersisted on Commit and restored on Connect, so a restart resumes after\nthe last commit.\n\nDesign: a CheckpointStore interface (Load/Save the full map), wired into\nEventHubs — Connect loads, Commit saves under the existing lock. nil\nstore (the default) is byte-for-byte the prior in-memory behavior. The\nblob impl is split behind a tiny blobClient seam so its logic (marshal\nthe map, treat a missing blob as first-run) is unit-tested with a fake;\nonly the thin azblob upload/download wrapper is untested I/O, consistent\nwith the other cloud backends.\n\nServer flags (--wal-backend=eventhubs):\n  --wal-eventhubs-checkpoint-storage-connection-string (enables it)\n  --wal-eventhubs-checkpoint-container (default entdb-wal-checkpoints)\nOne blob per <hub>/<consumer-group> keeps distinct WALs from colliding.\nDocumented in docs/deployment.md.\n\nTests: blob store round-trip via a fake blobClient (incl. missing-blob\n⇒ empty map, and an int64 > 2^53 checkpoint); EventHubs restores\ncheckpoints on Connect and persists them on Commit via a fake store.\n\nCloses #570.",
          "timestamp": "2026-05-25T11:28:40+01:00",
          "tree_id": "9e78e54a76135d4a43192f7d958491f354ef53a2",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/871f96c09b9e7000ba9f6ccdd6efd91229ee15eb"
        },
        "date": 1779705035527,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3063.5790505496516,
            "unit": "iter/sec",
            "range": "stddev: 0.00002941423412288518",
            "extra": "mean: 326.4156019804957 usec\nrounds: 1515"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2051.0104611979336,
            "unit": "iter/sec",
            "range": "stddev: 0.00004472283553557799",
            "extra": "mean: 487.5645536278396 usec\nrounds: 1268"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 944.9275953768998,
            "unit": "iter/sec",
            "range": "stddev: 0.00009872310097244535",
            "extra": "mean: 1.0582821423488364 msec\nrounds: 843"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 455.4016792460287,
            "unit": "iter/sec",
            "range": "stddev: 0.00012530715961012428",
            "extra": "mean: 2.195863664041859 msec\nrounds: 381"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1926.9306549834896,
            "unit": "iter/sec",
            "range": "stddev: 0.00008588571028997401",
            "extra": "mean: 518.9600349207005 usec\nrounds: 1575"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1908.1981920941585,
            "unit": "iter/sec",
            "range": "stddev: 0.00007321864642180683",
            "extra": "mean: 524.0545788918009 usec\nrounds: 1876"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2006.9159971423255,
            "unit": "iter/sec",
            "range": "stddev: 0.00006469442421274403",
            "extra": "mean: 498.27695898777694 usec\nrounds: 1146"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1580.3568335497428,
            "unit": "iter/sec",
            "range": "stddev: 0.0001377282270869214",
            "extra": "mean: 632.7684854273289 usec\nrounds: 995"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1625.5638012250222,
            "unit": "iter/sec",
            "range": "stddev: 0.00005209113840712374",
            "extra": "mean: 615.1711789142953 usec\nrounds: 313"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1303.3437513113795,
            "unit": "iter/sec",
            "range": "stddev: 0.00006301590011641117",
            "extra": "mean: 767.2572941665117 usec\nrounds: 1217"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2540.124732243882,
            "unit": "iter/sec",
            "range": "stddev: 0.00003168979936217537",
            "extra": "mean: 393.6814548144749 usec\nrounds: 2025"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 107.87123072368846,
            "unit": "iter/sec",
            "range": "stddev: 0.0023288768197365145",
            "extra": "mean: 9.270312327867051 msec\nrounds: 122"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "d16b67d814b6a79aab461c337a6318dec7e16b15",
          "message": "feat(uniqueness): composite (multi-field) unique constraints (#566) (#590)\n\nDeclare multi-field uniqueness via (entdb.node).composite_unique and\nenforce it atomically server-side, closing the racy query-then-create\nclients had to use for keys like (provider, provider_user_id).\n\nServer:\n- schema registry already carried CompositeUniqueDef; wire the applier\n  to ensure unique/composite/query expression indexes on the BatchTxn\n  connection before each CreateNode/UpdateNode (implements the\n  ensure-before-write hook ADR-023 described but the WAL path never did,\n  so single-field unique is now enforced through the applier too).\n- translate a SQLite UNIQUE-index violation into the structured\n  ALREADY_EXISTS detail by parsing the index name back to its\n  constraint coordinates; values rendered from the jsonnum-decoded\n  payload so int64 > 2^53 round-trips losslessly (ADR-028).\n- a constraint trip is a deterministic outcome: memoize it in the\n  idempotency cache (status UNIQUE_VIOLATION) and advance the WAL\n  offset without halting, mirroring the issue-#500 CAS-miss path;\n  ExecuteAtomic lifts it into a gRPC ALREADY_EXISTS, GetReceiptStatus\n  surfaces it on the poll path.\n- contract seed gains OAuthIdentity(201) with a composite + single\n  unique constraint for the integration suite.\n\nSDKs (ship together):\n- Go SDK schema extractor resolves composite_unique proto field names\n  to field_ids and emits them in the schema snapshot; error parsing\n  for the composite ALREADY_EXISTS detail already existed.\n- Python SDK declaration + parsing already existed; add the typed\n  composite-error unit test.\n\nDetail formats (wire contract, pinned by the SDK parsers):\n  Unique constraint violation: tenant=<t> type_id=<T> field_id=<F> value=<repr> already exists\n  Composite unique constraint violation: tenant=<t> type_id=<T> constraint='<name>' fields=[<F>, ...] values=[<repr>, ...] already exists\n\nDesign recorded in ADR-030 (no contradictions with ADR-022/023/025/028).",
          "timestamp": "2026-05-25T12:22:50+01:00",
          "tree_id": "39fcb3a9c38762a962047459c0294a88f8ed52f8",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/d16b67d814b6a79aab461c337a6318dec7e16b15"
        },
        "date": 1779708278068,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3291.9481008810294,
            "unit": "iter/sec",
            "range": "stddev: 0.000038913971944285876",
            "extra": "mean: 303.77149619472084 usec\nrounds: 1314"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2141.9797216643874,
            "unit": "iter/sec",
            "range": "stddev: 0.00004053923236834483",
            "extra": "mean: 466.85782777764473 usec\nrounds: 1080"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1057.3080781707881,
            "unit": "iter/sec",
            "range": "stddev: 0.00009421906021482374",
            "extra": "mean: 945.7981270038767 usec\nrounds: 811"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 454.8953649016213,
            "unit": "iter/sec",
            "range": "stddev: 0.00021076423292415767",
            "extra": "mean: 2.19830773658524 msec\nrounds: 410"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1773.6926126512246,
            "unit": "iter/sec",
            "range": "stddev: 0.0001421420012681895",
            "extra": "mean: 563.7955488269478 usec\nrounds: 1321"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1812.1844980118565,
            "unit": "iter/sec",
            "range": "stddev: 0.00013824998524402372",
            "extra": "mean: 551.8201933065301 usec\nrounds: 1733"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1918.958928533208,
            "unit": "iter/sec",
            "range": "stddev: 0.00010541761296181338",
            "extra": "mean: 521.1158952549174 usec\nrounds: 1728"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1692.0480148911868,
            "unit": "iter/sec",
            "range": "stddev: 0.00005373277068122442",
            "extra": "mean: 590.9997773108753 usec\nrounds: 1190"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1679.4671215715932,
            "unit": "iter/sec",
            "range": "stddev: 0.0000642000442485382",
            "extra": "mean: 595.4269584415746 usec\nrounds: 385"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1433.1958386697586,
            "unit": "iter/sec",
            "range": "stddev: 0.00006259913999786313",
            "extra": "mean: 697.7413504969177 usec\nrounds: 1107"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2634.271400642186,
            "unit": "iter/sec",
            "range": "stddev: 0.00004231138617401891",
            "extra": "mean: 379.61160712454256 usec\nrounds: 1965"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 148.76222877374929,
            "unit": "iter/sec",
            "range": "stddev: 0.00012370329333122158",
            "extra": "mean: 6.722136447154796 msec\nrounds: 123"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "d16b67d814b6a79aab461c337a6318dec7e16b15",
          "message": "feat(uniqueness): composite (multi-field) unique constraints (#566) (#590)\n\nDeclare multi-field uniqueness via (entdb.node).composite_unique and\nenforce it atomically server-side, closing the racy query-then-create\nclients had to use for keys like (provider, provider_user_id).\n\nServer:\n- schema registry already carried CompositeUniqueDef; wire the applier\n  to ensure unique/composite/query expression indexes on the BatchTxn\n  connection before each CreateNode/UpdateNode (implements the\n  ensure-before-write hook ADR-023 described but the WAL path never did,\n  so single-field unique is now enforced through the applier too).\n- translate a SQLite UNIQUE-index violation into the structured\n  ALREADY_EXISTS detail by parsing the index name back to its\n  constraint coordinates; values rendered from the jsonnum-decoded\n  payload so int64 > 2^53 round-trips losslessly (ADR-028).\n- a constraint trip is a deterministic outcome: memoize it in the\n  idempotency cache (status UNIQUE_VIOLATION) and advance the WAL\n  offset without halting, mirroring the issue-#500 CAS-miss path;\n  ExecuteAtomic lifts it into a gRPC ALREADY_EXISTS, GetReceiptStatus\n  surfaces it on the poll path.\n- contract seed gains OAuthIdentity(201) with a composite + single\n  unique constraint for the integration suite.\n\nSDKs (ship together):\n- Go SDK schema extractor resolves composite_unique proto field names\n  to field_ids and emits them in the schema snapshot; error parsing\n  for the composite ALREADY_EXISTS detail already existed.\n- Python SDK declaration + parsing already existed; add the typed\n  composite-error unit test.\n\nDetail formats (wire contract, pinned by the SDK parsers):\n  Unique constraint violation: tenant=<t> type_id=<T> field_id=<F> value=<repr> already exists\n  Composite unique constraint violation: tenant=<t> type_id=<T> constraint='<name>' fields=[<F>, ...] values=[<repr>, ...] already exists\n\nDesign recorded in ADR-030 (no contradictions with ADR-022/023/025/028).",
          "timestamp": "2026-05-25T12:22:50+01:00",
          "tree_id": "39fcb3a9c38762a962047459c0294a88f8ed52f8",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/d16b67d814b6a79aab461c337a6318dec7e16b15"
        },
        "date": 1779708280463,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3184.5603533865597,
            "unit": "iter/sec",
            "range": "stddev: 0.000026821979935322943",
            "extra": "mean: 314.01508812246846 usec\nrounds: 1566"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2113.961690097017,
            "unit": "iter/sec",
            "range": "stddev: 0.00004291372964405852",
            "extra": "mean: 473.0454694068305 usec\nrounds: 1095"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 974.5965682201261,
            "unit": "iter/sec",
            "range": "stddev: 0.00009874955954815348",
            "extra": "mean: 1.0260655871447069 msec\nrounds: 809"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 409.69775429096586,
            "unit": "iter/sec",
            "range": "stddev: 0.00041048988933123896",
            "extra": "mean: 2.440823728044659 msec\nrounds: 353"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1810.7527189006644,
            "unit": "iter/sec",
            "range": "stddev: 0.00013390918523487717",
            "extra": "mean: 552.2565226946699 usec\nrounds: 1410"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1816.476009771723,
            "unit": "iter/sec",
            "range": "stddev: 0.00012214801937527144",
            "extra": "mean: 550.51649161371 usec\nrounds: 1729"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1924.247611599739,
            "unit": "iter/sec",
            "range": "stddev: 0.00015540604484292304",
            "extra": "mean: 519.6836384110889 usec\nrounds: 1510"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1847.900688302098,
            "unit": "iter/sec",
            "range": "stddev: 0.000041389250857727575",
            "extra": "mean: 541.1546228270673 usec\nrounds: 1323"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1849.5911779743856,
            "unit": "iter/sec",
            "range": "stddev: 0.00006037277270338101",
            "extra": "mean: 540.6600182290925 usec\nrounds: 384"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1561.5568698344268,
            "unit": "iter/sec",
            "range": "stddev: 0.00007169571003882214",
            "extra": "mean: 640.3865394322979 usec\nrounds: 1268"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2631.6216447331785,
            "unit": "iter/sec",
            "range": "stddev: 0.00002709514918213661",
            "extra": "mean: 379.9938346005626 usec\nrounds: 2237"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 145.09043989478553,
            "unit": "iter/sec",
            "range": "stddev: 0.00016193747285755256",
            "extra": "mean: 6.892252864662652 msec\nrounds: 133"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "744c91c41a2476c9a4b3c38858c2c8ed5156d9c2",
          "message": "feat(#580): finish ADR-029 reads — SearchNodes, ListSharedWithMe, GetConnectedNodes (#591)\n\nCloses the last three reads from the keyset-pagination rollout, each with\nthe design decision recorded in ADR-029 rather than a forced cursor.\n\nListSharedWithMe — unified keyset cursor spanning BOTH merged sources\n(per-tenant node_access keyed on granted_at + global shared_index keyed on\nshared_at) over (timestamp, source_tenant, node_id) DESC. Each source is\nseeked with the same predicate, merged, deduped by (source_tenant,\nnode_id), top page_size returned; next_page_token is the last merged\ntuple. Exact has_more (page_size+1 probe per source). Token is\nfingerprint-bound to recipient+tenant; deprecated offset kept as a\nbackward-compatible single-request fallback, mutually exclusive with\npage_token. Both SDKs auto-follow to completion.\n\nSearchNodes — offset-paged ranked search (FTS carve-out): FTS5 rank is\ncomputed by MATCH and is not a stable keyset column, so search keeps\nlimit/offset with page_size as the AIP-158 alias (takes precedence) and an\nexact has_more (limit+1 probe, trimmed). No next_page_token. SDK search\nhelpers expose page_size+offset and do NOT auto-follow (top-N by design).\n\nGetConnectedNodes — documented as an intentionally bounded BFS traversal,\nnot cursor-paginated (keyset over a graph frontier is ill-defined). Made\nhas_more exact via a probe node beyond the page.\n\nAdds server keyset store methods (ListSharedWithMePaged,\nListSharedToUserPaged), the shared page-token codec, server tests\n(page_size + accurate has_more, keyset paging with no cross-page dups,\ncross-query token rejection, bounded-read assertions), and SDK tests\n(Go fake-server + Python autofollow) mirroring the existing QueryNodes\npatterns. Amends ADR-029 with the three resolutions and regenerates the\nproto stubs and Go SDK docs.",
          "timestamp": "2026-05-25T12:31:03+01:00",
          "tree_id": "100e11c40177129fd41cbcb092f17ac9f58fa77b",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/744c91c41a2476c9a4b3c38858c2c8ed5156d9c2"
        },
        "date": 1779708773318,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3022.742785461511,
            "unit": "iter/sec",
            "range": "stddev: 0.00002996421093979586",
            "extra": "mean: 330.8253698626628 usec\nrounds: 1387"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1986.4666236899322,
            "unit": "iter/sec",
            "range": "stddev: 0.00005211236552317874",
            "extra": "mean: 503.4063940840166 usec\nrounds: 1048"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 868.1370354813248,
            "unit": "iter/sec",
            "range": "stddev: 0.00013007111351745512",
            "extra": "mean: 1.1518918778134675 msec\nrounds: 622"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 452.69618489838996,
            "unit": "iter/sec",
            "range": "stddev: 0.00013260758345505955",
            "extra": "mean: 2.208987027855018 msec\nrounds: 359"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1907.221334386205,
            "unit": "iter/sec",
            "range": "stddev: 0.00008205936264762659",
            "extra": "mean: 524.3229938605037 usec\nrounds: 1303"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1883.2638917009294,
            "unit": "iter/sec",
            "range": "stddev: 0.00008037109324252685",
            "extra": "mean: 530.9930299236069 usec\nrounds: 1437"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1984.6409377158143,
            "unit": "iter/sec",
            "range": "stddev: 0.00007496483295451178",
            "extra": "mean: 503.86948137376 usec\nrounds: 1718"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1741.8940755514527,
            "unit": "iter/sec",
            "range": "stddev: 0.000056892967962461714",
            "extra": "mean: 574.0877209674289 usec\nrounds: 1240"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1748.8919244922727,
            "unit": "iter/sec",
            "range": "stddev: 0.00003409893873972293",
            "extra": "mean: 571.7906212474014 usec\nrounds: 433"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1469.7683591794823,
            "unit": "iter/sec",
            "range": "stddev: 0.00007803528238635771",
            "extra": "mean: 680.3793221935076 usec\nrounds: 1167"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2517.161077037715,
            "unit": "iter/sec",
            "range": "stddev: 0.00003158686403891838",
            "extra": "mean: 397.272947338291 usec\nrounds: 1747"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 146.4173375730708,
            "unit": "iter/sec",
            "range": "stddev: 0.0003617461037156299",
            "extra": "mean: 6.829792267605888 msec\nrounds: 142"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "744c91c41a2476c9a4b3c38858c2c8ed5156d9c2",
          "message": "feat(#580): finish ADR-029 reads — SearchNodes, ListSharedWithMe, GetConnectedNodes (#591)\n\nCloses the last three reads from the keyset-pagination rollout, each with\nthe design decision recorded in ADR-029 rather than a forced cursor.\n\nListSharedWithMe — unified keyset cursor spanning BOTH merged sources\n(per-tenant node_access keyed on granted_at + global shared_index keyed on\nshared_at) over (timestamp, source_tenant, node_id) DESC. Each source is\nseeked with the same predicate, merged, deduped by (source_tenant,\nnode_id), top page_size returned; next_page_token is the last merged\ntuple. Exact has_more (page_size+1 probe per source). Token is\nfingerprint-bound to recipient+tenant; deprecated offset kept as a\nbackward-compatible single-request fallback, mutually exclusive with\npage_token. Both SDKs auto-follow to completion.\n\nSearchNodes — offset-paged ranked search (FTS carve-out): FTS5 rank is\ncomputed by MATCH and is not a stable keyset column, so search keeps\nlimit/offset with page_size as the AIP-158 alias (takes precedence) and an\nexact has_more (limit+1 probe, trimmed). No next_page_token. SDK search\nhelpers expose page_size+offset and do NOT auto-follow (top-N by design).\n\nGetConnectedNodes — documented as an intentionally bounded BFS traversal,\nnot cursor-paginated (keyset over a graph frontier is ill-defined). Made\nhas_more exact via a probe node beyond the page.\n\nAdds server keyset store methods (ListSharedWithMePaged,\nListSharedToUserPaged), the shared page-token codec, server tests\n(page_size + accurate has_more, keyset paging with no cross-page dups,\ncross-query token rejection, bounded-read assertions), and SDK tests\n(Go fake-server + Python autofollow) mirroring the existing QueryNodes\npatterns. Amends ADR-029 with the three resolutions and regenerates the\nproto stubs and Go SDK docs.",
          "timestamp": "2026-05-25T12:31:03+01:00",
          "tree_id": "100e11c40177129fd41cbcb092f17ac9f58fa77b",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/744c91c41a2476c9a4b3c38858c2c8ed5156d9c2"
        },
        "date": 1779708774031,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3224.4897584317278,
            "unit": "iter/sec",
            "range": "stddev: 0.00002600550471215462",
            "extra": "mean: 310.12658588388973 usec\nrounds: 1601"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2131.1491706983006,
            "unit": "iter/sec",
            "range": "stddev: 0.000040132155105987334",
            "extra": "mean: 469.23041040451244 usec\nrounds: 1211"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 991.2196517079309,
            "unit": "iter/sec",
            "range": "stddev: 0.00009218374020633207",
            "extra": "mean: 1.0088581257211153 msec\nrounds: 867"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 471.98585678051904,
            "unit": "iter/sec",
            "range": "stddev: 0.00011074750994635156",
            "extra": "mean: 2.1187075536990423 msec\nrounds: 419"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1985.2884258956753,
            "unit": "iter/sec",
            "range": "stddev: 0.00007589717345168323",
            "extra": "mean: 503.70514780432654 usec\nrounds: 1617"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1967.8423939457346,
            "unit": "iter/sec",
            "range": "stddev: 0.00007937091652576031",
            "extra": "mean: 508.17077784105106 usec\nrounds: 1751"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1891.8591038236234,
            "unit": "iter/sec",
            "range": "stddev: 0.0001363613300559863",
            "extra": "mean: 528.5805893149795 usec\nrounds: 1853"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1781.6217569314279,
            "unit": "iter/sec",
            "range": "stddev: 0.00006158914856119438",
            "extra": "mean: 561.2863651386632 usec\nrounds: 1331"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1756.9256085845116,
            "unit": "iter/sec",
            "range": "stddev: 0.000055209472226956695",
            "extra": "mean: 569.176062500257 usec\nrounds: 352"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1488.7000693002833,
            "unit": "iter/sec",
            "range": "stddev: 0.00008859223630194173",
            "extra": "mean: 671.7269788736012 usec\nrounds: 1136"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2616.0022891966896,
            "unit": "iter/sec",
            "range": "stddev: 0.000032018639838875795",
            "extra": "mean: 382.26266243332515 usec\nrounds: 1890"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 139.73866786164788,
            "unit": "iter/sec",
            "range": "stddev: 0.00021708700152847294",
            "extra": "mean: 7.156215350428827 msec\nrounds: 117"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "f2610a4173bb7cc2e91670a9ad24620d9b2f3d43",
          "message": "fix(shared): stable total order on the ListSharedWithMe offset path (#593)\n\nThe deprecated offset path (store.ListSharedWithMe / globalstore.\nListSharedToUser) ordered by granted_at / shared_at DESC with no unique\ntiebreaker. With multiple grants sharing a timestamp, SQLite's row order\nwas unstable across successive offset windows, so a node could be skipped\n(or duplicated) between pages — TestListSharedWithMe_Pagination failed\nunder repeated runs. Append the node_id (and source_tenant) tiebreaker so\nthe offset order is a total order, matching the keyset path's ordering.\nShipped in v1.31.0; this is the follow-up fix.",
          "timestamp": "2026-05-25T12:51:01+01:00",
          "tree_id": "1c1bf0f69eccbc75a1ebb547a2f5e2b247821c9e",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/f2610a4173bb7cc2e91670a9ad24620d9b2f3d43"
        },
        "date": 1779709965345,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3766.138000905714,
            "unit": "iter/sec",
            "range": "stddev: 0.00003250025762476065",
            "extra": "mean: 265.52399294967717 usec\nrounds: 1560"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2503.947010079766,
            "unit": "iter/sec",
            "range": "stddev: 0.00003616701025013313",
            "extra": "mean: 399.36947386444245 usec\nrounds: 1167"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1240.2324189101976,
            "unit": "iter/sec",
            "range": "stddev: 0.00008412013699161123",
            "extra": "mean: 806.300484290443 usec\nrounds: 923"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 558.0857678931166,
            "unit": "iter/sec",
            "range": "stddev: 0.00011615460344647523",
            "extra": "mean: 1.791839279068514 msec\nrounds: 473"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2448.5993642790636,
            "unit": "iter/sec",
            "range": "stddev: 0.00006512854038616567",
            "extra": "mean: 408.3967408422603 usec\nrounds: 1802"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2611.6194003941064,
            "unit": "iter/sec",
            "range": "stddev: 0.00004469570882896071",
            "extra": "mean: 382.90418575122203 usec\nrounds: 2358"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2700.8379132419855,
            "unit": "iter/sec",
            "range": "stddev: 0.000051009527720400506",
            "extra": "mean: 370.25546594154446 usec\nrounds: 2114"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 849.9701405945383,
            "unit": "iter/sec",
            "range": "stddev: 0.0031325543907111857",
            "extra": "mean: 1.1765119175839738 msec\nrounds: 182"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 550.4154954066307,
            "unit": "iter/sec",
            "range": "stddev: 0.012313458230611122",
            "extra": "mean: 1.8168093164986745 msec\nrounds: 891"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 253.63379529880746,
            "unit": "iter/sec",
            "range": "stddev: 0.029657927512721415",
            "extra": "mean: 3.9426922536955065 msec\nrounds: 812"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 3064.160921522468,
            "unit": "iter/sec",
            "range": "stddev: 0.00004411921605748891",
            "extra": "mean: 326.35361706236273 usec\nrounds: 2063"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 117.3645844334882,
            "unit": "iter/sec",
            "range": "stddev: 0.0019453425772047084",
            "extra": "mean: 8.520457894746869 msec\nrounds: 19"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "f2610a4173bb7cc2e91670a9ad24620d9b2f3d43",
          "message": "fix(shared): stable total order on the ListSharedWithMe offset path (#593)\n\nThe deprecated offset path (store.ListSharedWithMe / globalstore.\nListSharedToUser) ordered by granted_at / shared_at DESC with no unique\ntiebreaker. With multiple grants sharing a timestamp, SQLite's row order\nwas unstable across successive offset windows, so a node could be skipped\n(or duplicated) between pages — TestListSharedWithMe_Pagination failed\nunder repeated runs. Append the node_id (and source_tenant) tiebreaker so\nthe offset order is a total order, matching the keyset path's ordering.\nShipped in v1.31.0; this is the follow-up fix.",
          "timestamp": "2026-05-25T12:51:01+01:00",
          "tree_id": "1c1bf0f69eccbc75a1ebb547a2f5e2b247821c9e",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/f2610a4173bb7cc2e91670a9ad24620d9b2f3d43"
        },
        "date": 1779709968486,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3335.243806669005,
            "unit": "iter/sec",
            "range": "stddev: 0.000028879827947973336",
            "extra": "mean: 299.82815589086607 usec\nrounds: 1392"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2153.18680198069,
            "unit": "iter/sec",
            "range": "stddev: 0.000041150078102850546",
            "extra": "mean: 464.4278885046632 usec\nrounds: 1157"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1073.889294666938,
            "unit": "iter/sec",
            "range": "stddev: 0.0000985041081710168",
            "extra": "mean: 931.1946817666578 usec\nrounds: 883"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 468.44427830405965,
            "unit": "iter/sec",
            "range": "stddev: 0.0001515587622642141",
            "extra": "mean: 2.1347256147953546 msec\nrounds: 392"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1822.749375327411,
            "unit": "iter/sec",
            "range": "stddev: 0.00013980609275139798",
            "extra": "mean: 548.6217762770459 usec\nrounds: 1703"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1789.3930966587036,
            "unit": "iter/sec",
            "range": "stddev: 0.000157704863244186",
            "extra": "mean: 558.8486967269959 usec\nrounds: 1375"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1831.8883670250552,
            "unit": "iter/sec",
            "range": "stddev: 0.0001336330024424682",
            "extra": "mean: 545.8847918904453 usec\nrounds: 1677"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1775.9219640552562,
            "unit": "iter/sec",
            "range": "stddev: 0.00005244263105128383",
            "extra": "mean: 563.0878046671232 usec\nrounds: 1157"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1828.0678624566144,
            "unit": "iter/sec",
            "range": "stddev: 0.0000710332517956158",
            "extra": "mean: 547.0256441443968 usec\nrounds: 444"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1565.8316674873447,
            "unit": "iter/sec",
            "range": "stddev: 0.00006528042208395078",
            "extra": "mean: 638.6382526064746 usec\nrounds: 1247"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2648.0552247965074,
            "unit": "iter/sec",
            "range": "stddev: 0.00002788370758488118",
            "extra": "mean: 377.635628832796 usec\nrounds: 1859"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 140.22980059445212,
            "unit": "iter/sec",
            "range": "stddev: 0.0006937443448724082",
            "extra": "mean: 7.131151836206511 msec\nrounds: 116"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "5a9c40680f8cdd821ebdcd566b239aa9adf7a715",
          "message": "feat(mailbox): expose USER_MAILBOX reads server-side + Go/Python SDK (#568) (#592)\n\nReimplement the mailbox read surface as a tenant-backed feature\n(ADR-014's sanctioned path): USER_MAILBOX nodes are stored in the\nper-tenant SQLite file, attributed by target_user_id, and reachable only\nthrough an explicit mailbox scope — never via tenant reads.\n\nStorage / write path (was the blocker):\n- nodes table gains storage_mode + target_user_id columns (additive\n  migration, partial index idx_nodes_mailbox); the applier now persists\n  both on create_node instead of dropping them.\n- The applier now maintains the FTS5 index transactionally on\n  create/update/delete (it never did before — FTSInsert was dead code),\n  so search works end-to-end through the WAL apply path. Fixes a latent\n  cross-connection deadlock by running the CREATE VIRTUAL TABLE DDL on\n  the open BatchTxn connection (EnsureFTSIndexConn).\n\nRead path:\n- Additive target_user field on GetNode/GetNodes/QueryNodes/SearchNodes.\n  Set => scope to that user's USER_MAILBOX nodes; empty => tenant read\n  that EXCLUDES every mailbox-private row (ADR-020 privacy boundary).\n- store: GetMailboxNode, QueryNodes MailboxUser, SearchMailboxNodes.\n\nSDKs (shipped together):\n- Go: GetInMailbox / QueryInMailbox / QueryWhereInMailbox /\n  SearchInMailbox plus the matching transport methods.\n- Python: ActorScope.get_in_mailbox / get_many_in_mailbox /\n  query_in_mailbox / search_in_mailbox plus target_user kwargs on the\n  client + grpc layers.\n\nTests: store + applier (FTS end-to-end) + api scoping (Go), SDK wire\nforwarding (Go + Python unit), live-server scoping + privacy (Python\nintegration). Regenerated proto stubs and docs/generated with go 1.25.",
          "timestamp": "2026-05-25T12:56:00+01:00",
          "tree_id": "c7d4d5002d25876f05fbfdb72995fc7bd19599f2",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/5a9c40680f8cdd821ebdcd566b239aa9adf7a715"
        },
        "date": 1779710266694,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3303.8214452889097,
            "unit": "iter/sec",
            "range": "stddev: 0.0000305868409129418",
            "extra": "mean: 302.6797956729628 usec\nrounds: 1248"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2041.360986267852,
            "unit": "iter/sec",
            "range": "stddev: 0.00005679000287623709",
            "extra": "mean: 489.8692620888502 usec\nrounds: 1034"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1003.1460371254567,
            "unit": "iter/sec",
            "range": "stddev: 0.00011091438739367443",
            "extra": "mean: 996.8638293837338 usec\nrounds: 633"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 452.79057868676955,
            "unit": "iter/sec",
            "range": "stddev: 0.00015471750895719376",
            "extra": "mean: 2.2085265177122375 msec\nrounds: 367"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1911.7794369206908,
            "unit": "iter/sec",
            "range": "stddev: 0.00008808875167084998",
            "extra": "mean: 523.0728925564255 usec\nrounds: 1666"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1934.5090601390134,
            "unit": "iter/sec",
            "range": "stddev: 0.0000874518214514504",
            "extra": "mean: 516.9270181283825 usec\nrounds: 1710"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1970.768262346763,
            "unit": "iter/sec",
            "range": "stddev: 0.00009902290995512547",
            "extra": "mean: 507.4163305274736 usec\nrounds: 1782"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1856.5358052564227,
            "unit": "iter/sec",
            "range": "stddev: 0.00004953101294646267",
            "extra": "mean: 538.6376051400102 usec\nrounds: 1284"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1835.9548968765848,
            "unit": "iter/sec",
            "range": "stddev: 0.000054528188116162645",
            "extra": "mean: 544.6756898555886 usec\nrounds: 345"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1553.005024587325,
            "unit": "iter/sec",
            "range": "stddev: 0.00006207460411353673",
            "extra": "mean: 643.9129198991013 usec\nrounds: 1186"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2640.5984775780507,
            "unit": "iter/sec",
            "range": "stddev: 0.000022757477587966315",
            "extra": "mean: 378.7020285330154 usec\nrounds: 736"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 132.5582415027689,
            "unit": "iter/sec",
            "range": "stddev: 0.00014386202166120354",
            "extra": "mean: 7.543853846153441 msec\nrounds: 117"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "5a9c40680f8cdd821ebdcd566b239aa9adf7a715",
          "message": "feat(mailbox): expose USER_MAILBOX reads server-side + Go/Python SDK (#568) (#592)\n\nReimplement the mailbox read surface as a tenant-backed feature\n(ADR-014's sanctioned path): USER_MAILBOX nodes are stored in the\nper-tenant SQLite file, attributed by target_user_id, and reachable only\nthrough an explicit mailbox scope — never via tenant reads.\n\nStorage / write path (was the blocker):\n- nodes table gains storage_mode + target_user_id columns (additive\n  migration, partial index idx_nodes_mailbox); the applier now persists\n  both on create_node instead of dropping them.\n- The applier now maintains the FTS5 index transactionally on\n  create/update/delete (it never did before — FTSInsert was dead code),\n  so search works end-to-end through the WAL apply path. Fixes a latent\n  cross-connection deadlock by running the CREATE VIRTUAL TABLE DDL on\n  the open BatchTxn connection (EnsureFTSIndexConn).\n\nRead path:\n- Additive target_user field on GetNode/GetNodes/QueryNodes/SearchNodes.\n  Set => scope to that user's USER_MAILBOX nodes; empty => tenant read\n  that EXCLUDES every mailbox-private row (ADR-020 privacy boundary).\n- store: GetMailboxNode, QueryNodes MailboxUser, SearchMailboxNodes.\n\nSDKs (shipped together):\n- Go: GetInMailbox / QueryInMailbox / QueryWhereInMailbox /\n  SearchInMailbox plus the matching transport methods.\n- Python: ActorScope.get_in_mailbox / get_many_in_mailbox /\n  query_in_mailbox / search_in_mailbox plus target_user kwargs on the\n  client + grpc layers.\n\nTests: store + applier (FTS end-to-end) + api scoping (Go), SDK wire\nforwarding (Go + Python unit), live-server scoping + privacy (Python\nintegration). Regenerated proto stubs and docs/generated with go 1.25.",
          "timestamp": "2026-05-25T12:56:00+01:00",
          "tree_id": "c7d4d5002d25876f05fbfdb72995fc7bd19599f2",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/5a9c40680f8cdd821ebdcd566b239aa9adf7a715"
        },
        "date": 1779710271809,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2991.029618681989,
            "unit": "iter/sec",
            "range": "stddev: 0.00002948947186065099",
            "extra": "mean: 334.33303159353346 usec\nrounds: 1456"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1980.9667635769406,
            "unit": "iter/sec",
            "range": "stddev: 0.00004808094349721248",
            "extra": "mean: 504.8040271984907 usec\nrounds: 1103"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 878.5591053264582,
            "unit": "iter/sec",
            "range": "stddev: 0.0000959680868036507",
            "extra": "mean: 1.138227347411551 msec\nrounds: 734"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 425.4800211446887,
            "unit": "iter/sec",
            "range": "stddev: 0.0002267484031138163",
            "extra": "mean: 2.3502866181816326 msec\nrounds: 385"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1730.599411813946,
            "unit": "iter/sec",
            "range": "stddev: 0.0001608838470955567",
            "extra": "mean: 577.8344735202697 usec\nrounds: 1284"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1724.8446420661132,
            "unit": "iter/sec",
            "range": "stddev: 0.00014325953489184007",
            "extra": "mean: 579.7623598158646 usec\nrounds: 1737"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1797.96119243212,
            "unit": "iter/sec",
            "range": "stddev: 0.00015784171512207097",
            "extra": "mean: 556.1855307050816 usec\nrounds: 1319"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1658.7716972172443,
            "unit": "iter/sec",
            "range": "stddev: 0.0000769695402907027",
            "extra": "mean: 602.855716478404 usec\nrounds: 1238"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1717.1541343373883,
            "unit": "iter/sec",
            "range": "stddev: 0.0000703475185239721",
            "extra": "mean: 582.3589041911358 usec\nrounds: 334"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1441.5934025716786,
            "unit": "iter/sec",
            "range": "stddev: 0.00007436288424064326",
            "extra": "mean: 693.6768704796277 usec\nrounds: 1189"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2462.1149990041004,
            "unit": "iter/sec",
            "range": "stddev: 0.00003409796800311843",
            "extra": "mean: 406.1548710781135 usec\nrounds: 1753"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 139.10285925511388,
            "unit": "iter/sec",
            "range": "stddev: 0.0005929879384087536",
            "extra": "mean: 7.188924838460764 msec\nrounds: 130"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "7c71502fee7c73736212f3e31ea93a002ba936ab",
          "message": "fix(mailbox): exclude USER_MAILBOX nodes from plain tenant by-id reads (#568) + cross-stack tests (#594)\n\nA new Go-SDK real-server integration test surfaced a privacy gap shipped\nin v1.32.0: QueryNodes/SearchNodes exclude mailbox-private nodes from a\nplain tenant read (no target_user), but GetNode/GetNodes by id returned\nthem — so a leaked or guessed node id exposed a user's private mailbox\ncontent. GetNode/GetNodes now treat a USER_MAILBOX node as not-found on a\nplain tenant read, consistent with the scan path; mailbox content is\nreachable only through the mailbox scope (target_user set).\n\nTest coverage added across the layers exercised by the v1.24–v1.32\nfeatures:\n  - E2E (Python SDK ↔ docker stack): query() + edges_out() keyset\n    auto-follow return the complete set over 150 / 120 rows; USER_MAILBOX\n    read via the mailbox scope + the tenant-read privacy exclusion.\n  - Go SDK integration (real server): mailbox round-trip + privacy (the\n    test that caught this bug).\n  - Server unit: GetNode/GetNodes by-id mailbox exclusion.\n\nCloses the privacy gap; refs #568.",
          "timestamp": "2026-05-25T14:13:25+01:00",
          "tree_id": "bcccf6d7f7ba3bb47e41690bff6dbd6df3ce1159",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/7c71502fee7c73736212f3e31ea93a002ba936ab"
        },
        "date": 1779714918957,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3025.8197565267383,
            "unit": "iter/sec",
            "range": "stddev: 0.00003065745339386496",
            "extra": "mean: 330.4889519089778 usec\nrounds: 1414"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2005.913724673138,
            "unit": "iter/sec",
            "range": "stddev: 0.00004578613746657298",
            "extra": "mean: 498.5259274612866 usec\nrounds: 1158"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 888.4481174798829,
            "unit": "iter/sec",
            "range": "stddev: 0.00009397060610494034",
            "extra": "mean: 1.1255581280722822 msec\nrounds: 773"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 433.06781956200416,
            "unit": "iter/sec",
            "range": "stddev: 0.00017293541072512436",
            "extra": "mean: 2.3091071532661545 msec\nrounds: 398"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1905.561216948111,
            "unit": "iter/sec",
            "range": "stddev: 0.00008171471440901213",
            "extra": "mean: 524.7797819907196 usec\nrounds: 1477"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1882.7712343146895,
            "unit": "iter/sec",
            "range": "stddev: 0.00009111352630713715",
            "extra": "mean: 531.13197279328 usec\nrounds: 1654"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1969.862243078388,
            "unit": "iter/sec",
            "range": "stddev: 0.00009246755406505623",
            "extra": "mean: 507.6497118078964 usec\nrounds: 1499"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1635.5840554206457,
            "unit": "iter/sec",
            "range": "stddev: 0.00006605704918836242",
            "extra": "mean: 611.4023896759107 usec\nrounds: 988"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1637.2308992375945,
            "unit": "iter/sec",
            "range": "stddev: 0.00004625897364500898",
            "extra": "mean: 610.7873974682909 usec\nrounds: 395"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1369.777772667205,
            "unit": "iter/sec",
            "range": "stddev: 0.00007387813686914327",
            "extra": "mean: 730.045427772433 usec\nrounds: 1073"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2455.884735885553,
            "unit": "iter/sec",
            "range": "stddev: 0.00003205645903851714",
            "extra": "mean: 407.18523365039607 usec\nrounds: 1682"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 116.72828430950548,
            "unit": "iter/sec",
            "range": "stddev: 0.0016209545588725977",
            "extra": "mean: 8.566903950618311 msec\nrounds: 81"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "7c71502fee7c73736212f3e31ea93a002ba936ab",
          "message": "fix(mailbox): exclude USER_MAILBOX nodes from plain tenant by-id reads (#568) + cross-stack tests (#594)\n\nA new Go-SDK real-server integration test surfaced a privacy gap shipped\nin v1.32.0: QueryNodes/SearchNodes exclude mailbox-private nodes from a\nplain tenant read (no target_user), but GetNode/GetNodes by id returned\nthem — so a leaked or guessed node id exposed a user's private mailbox\ncontent. GetNode/GetNodes now treat a USER_MAILBOX node as not-found on a\nplain tenant read, consistent with the scan path; mailbox content is\nreachable only through the mailbox scope (target_user set).\n\nTest coverage added across the layers exercised by the v1.24–v1.32\nfeatures:\n  - E2E (Python SDK ↔ docker stack): query() + edges_out() keyset\n    auto-follow return the complete set over 150 / 120 rows; USER_MAILBOX\n    read via the mailbox scope + the tenant-read privacy exclusion.\n  - Go SDK integration (real server): mailbox round-trip + privacy (the\n    test that caught this bug).\n  - Server unit: GetNode/GetNodes by-id mailbox exclusion.\n\nCloses the privacy gap; refs #568.",
          "timestamp": "2026-05-25T14:13:25+01:00",
          "tree_id": "bcccf6d7f7ba3bb47e41690bff6dbd6df3ce1159",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/7c71502fee7c73736212f3e31ea93a002ba936ab"
        },
        "date": 1779714927488,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3028.9072635541347,
            "unit": "iter/sec",
            "range": "stddev: 0.00003204040270776098",
            "extra": "mean: 330.152069042416 usec\nrounds: 1347"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1977.027085400612,
            "unit": "iter/sec",
            "range": "stddev: 0.00005248681966528732",
            "extra": "mean: 505.8099645596745 usec\nrounds: 1044"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 882.1985601943883,
            "unit": "iter/sec",
            "range": "stddev: 0.00010637904850662368",
            "extra": "mean: 1.1335316618286644 msec\nrounds: 689"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 421.2945618059465,
            "unit": "iter/sec",
            "range": "stddev: 0.00016830988899613748",
            "extra": "mean: 2.3736361459624353 msec\nrounds: 322"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1866.1422143177967,
            "unit": "iter/sec",
            "range": "stddev: 0.0000922893623869768",
            "extra": "mean: 535.8648404862159 usec\nrounds: 1398"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1724.7551287712686,
            "unit": "iter/sec",
            "range": "stddev: 0.000139693819642044",
            "extra": "mean: 579.792448979357 usec\nrounds: 1715"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1824.548812744457,
            "unit": "iter/sec",
            "range": "stddev: 0.00016398147430990705",
            "extra": "mean: 548.0807052214822 usec\nrounds: 1513"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1666.3501192358872,
            "unit": "iter/sec",
            "range": "stddev: 0.00007744720280655652",
            "extra": "mean: 600.1139787228838 usec\nrounds: 1269"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1666.810087093333,
            "unit": "iter/sec",
            "range": "stddev: 0.00003264264642986676",
            "extra": "mean: 599.9483730890123 usec\nrounds: 327"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1419.6643575924254,
            "unit": "iter/sec",
            "range": "stddev: 0.00008501814259479397",
            "extra": "mean: 704.3918477293295 usec\nrounds: 1123"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2469.1356978621434,
            "unit": "iter/sec",
            "range": "stddev: 0.000033715015514550444",
            "extra": "mean: 405.0000171581627 usec\nrounds: 1865"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 123.22405535924322,
            "unit": "iter/sec",
            "range": "stddev: 0.00027293152928451234",
            "extra": "mean: 8.115298567999844 msec\nrounds: 125"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "c983e77d34a4b46b476870ce0d81c553305ee6bc",
          "message": "feat(sdk): v2.1.0 — InsertIfNotExists helper (#599) + ADR-031 CI restoration (#613)\n\n* feat(sdk): InsertIfNotExists — atomic insert-or-resolve helper (#599)\n\nCloses the racy 'query-then-create' idiom under concurrent writers of\nthe same uniquely-keyed payload: exactly one writer wins and the rest\nlearn the existing canonical node id without a second user-visible\nround trip into client code.\n\nAPI (both SDKs):\n\n  Go:     created, existing, err :=\n            scope.InsertIfNotExists(ctx, idempotencyKey, msg)\n  Python: created, existing =\n            await scope.insert_if_not_exists(msg, idempotency_key='...')\n\nExactly one of (created, existing) is non-empty on success. The helper\nforces wait_applied=True so the applier's outcome surfaces\nsynchronously (otherwise the loser of a unique race would see a\nphantom success — #606). A typed UniqueConstraintError on commit\ntriggers a follow-up GetNodeByKey using the (type_id, field_id, value)\ntuple the server attached to the error; the resolved id is returned.\n\nv2.1.0 boundary (deliberate scope):\n\n- SINGLE-FIELD unique only. A composite-unique violation (ADR-030)\n  re-raises the typed error without a follow-up lookup — there is no\n  GetByCompositeKey RPC in v2.x, so callers needing composite upsert\n  query themselves. Tracked for v2.2.\n\n- This is still TWO round trips (commit + GetNodeByKey on conflict).\n  The single-round-trip server-side path (NodeConflictPolicy_SKIP +\n  ExecuteAtomicResponse.existing_node_ids threaded back through the\n  handler) is v2.2; doing it now would require either extending the\n  idempotency cache schema or adding a direct applier→handler result\n  channel, neither of which I want to land half-done.\n\nThis release closes the correctness gap (the racy idiom) NOW with the\nprimitives already shipped in v2.0.x.\n\nTests\n-----\n\nGo (sdk/go/entdb/insert_if_not_exists_test.go):\n  - HappyPath:        commit succeeds → (created, '', nil); WaitApplied\n                      is forced on (pinned via mock.lastWaitApplied*)\n  - SingleField:      *UniqueConstraintError → GetNodeByKey called with\n                      the UCE tuple → ('', existing, nil)\n  - Composite:        composite UCE → propagated; GetNodeByKey is NOT\n                      called\n  - Unrelated:        non-UCE error bubbles up unchanged\n  - NilMsg:           precondition short-circuits before any RPC\n\nPython (tests/python/unit/test_insert_if_not_exists.py): the same\nfive branches, using AsyncMock against the DbClient._grpc transport.\n\nCI: all four Go modules + python unit (454) + python integration (115)\ngreen locally.\n\n* docs(generated): refresh sdk-go.md (catches up v2.0.x + new InsertIfNotExists)\n\n`docs/generated/sdk-go.md` had been stale since the v2.0.x refactor\nlanded (WithWaitApplied / WithWaitTimeout / CommittedOffset were missing).\nv2.1.0 adds InsertIfNotExists, so regenerate end-to-end with the\ngo.mod-pinned toolchain (Go 1.25.0) per the regen-docs-with-pinned-go\nrule — newer local Go can drift the rendered output in ways the\n\"Docs Coverage & Examples\" CI gate flags as stale.\n\n* ci: restore green gates after ADR-031 (--seed-profile callers + stale snapshot)\n\nADR-031 removed the server's boot-seed flags (--seed-profile,\n--seed-tenant) but five call sites still passed them, so every CI run\nsince the ADR landed has been red on:\n\n  - Go SDK             (sdk/go/entdb/integration_test.go)\n  - Docs Coverage      (examples/_harness.py — runs runnable docs)\n  - Schema Compat      (.github/workflows/ci.yml + schema-compat.yml)\n  - Tier 1 benchmark   (tests/python/benchmarks/conftest.py +\n                        docker-compose.bench.yml)\n\nRecent PRs (#609, #610, #611) merged through despite the red gates.\nv2.1.0 deserves an actually-verified release.\n\nSites switched to empty-boot + API-driven bootstrap (mirror of\ntests/python/integration/conftest.py:_bootstrap_contract):\n\n* examples/_harness.py — provisions tenant + alice/bob via the gRPC\n  API after boot; lets the SDK auto-attach the schema on each\n  example's first ExecuteAtomic (hand-built descriptor fingerprints\n  differ from the SDK's canonicaliser → establish-or-reject would\n  reject the SDK write).\n\n* sdk/go/entdb/integration_test.go — adds bootstrapIntegrationContract\n  using a raw pb.EntDBServiceClient: CreateTenant + CreateUser +\n  AddTenantMember, then a self-describing ExecuteAtomic carrying the\n  itUserType (8001)/itProductType (8002)/edge 8101 SchemaDescriptor.\n\n* tests/python/benchmarks/conftest.py — adds _bootstrap_bench called\n  from the entdb_stub fixture (tenant + user + member). Tolerates\n  ALREADY_EXISTS so ENTDB_BENCH_ENDPOINT (an already-running container)\n  is a no-op.\n\n* .github/workflows/ci.yml + schema-compat.yml — drop the flags. The\n  step still runs entdb-schema check, just against an empty boot. With\n  the stale .schema-snapshot.json removed (see below) the boot+check\n  step's hashFiles() guard skips cleanly.\n\n* .schema-snapshot.json — DELETED. It carried names (pre-ADR-031\n  format) and was structurally incompatible with current GetSchema\n  output. Per ADR-032 the baseline is a customer artefact, not a\n  self-referential file in this repo — the workflows' `if:\n  hashFiles('.schema-snapshot.json') != ''` already guards on\n  presence, falling back to a 'No baseline — recommend committing\n  one' notice.\n\n* tests/python/benchmarks/docker-compose.bench.yml — drop the flags\n  from the EntDB service command. The bench corpus is still\n  name-keyed and will fail downstream of boot; that is a separate\n  pre-existing issue, tracked for a follow-up.\n\n* ci(bench): switch corpus to id-keyed payloads for EntDB seed (ADR-031)\n\nThe Postgres-vs-EntDB benchmark corpus generator produced\n`{title, description}` dict payloads consumed by BOTH backends. Per\nADR-031 (Bug C / CLAUDE.md invariant #6) the EntDB wire payload is\nfield-id-keyed, so the EntDB seed has been failing with\n`INVALID_ARGUMENT: payload contains non-field_id key \"title\"` ever\nsince name-keyed payloads were rejected.\n\n  - tests/python/benchmarks/conftest.py — corpus rows now carry a\n    sibling `payload_ids` ({'1': title, '2': description}) used by\n    `entdb_seeded`; `payload` (name-keyed) is retained verbatim\n    because the Postgres schema's JSONB expression-index keys on\n    `payload->>'title'`/`'description'` — both paths need their\n    historical shape.\n  - tests/python/benchmarks/bench_entdb.py — single-op + multi-op\n    write and update microbenchmarks now emit field-id-keyed\n    {'1': ..., '2': ...} maps to match the seed corpus.\n\nAll 12 bench_entdb cases pass locally with --benchmark-disable.",
          "timestamp": "2026-05-27T04:14:34+01:00",
          "tree_id": "96ef1a5d3d52d8e2d34387379dfdf7a1389458cb",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/c983e77d34a4b46b476870ce0d81c553305ee6bc"
        },
        "date": 1779851782110,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3862.9620929149637,
            "unit": "iter/sec",
            "range": "stddev: 0.00003172300084537396",
            "extra": "mean: 258.8687064349128 usec\nrounds: 1492"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2416.8658142994764,
            "unit": "iter/sec",
            "range": "stddev: 0.00004449461233959086",
            "extra": "mean: 413.75900725785556 usec\nrounds: 1240"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1101.5549863901128,
            "unit": "iter/sec",
            "range": "stddev: 0.00010907143195922997",
            "extra": "mean: 907.8076104735206 usec\nrounds: 611"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 487.9569390112183,
            "unit": "iter/sec",
            "range": "stddev: 0.00013422381539973235",
            "extra": "mean: 2.0493611629468185 msec\nrounds: 448"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2086.882523125414,
            "unit": "iter/sec",
            "range": "stddev: 0.00007168297861263901",
            "extra": "mean: 479.1836574022158 usec\nrounds: 1547"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2167.0909833562605,
            "unit": "iter/sec",
            "range": "stddev: 0.00008449509116755687",
            "extra": "mean: 461.4480922491127 usec\nrounds: 1832"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2153.979722048059,
            "unit": "iter/sec",
            "range": "stddev: 0.00008318254275303424",
            "extra": "mean: 464.2569239459573 usec\nrounds: 1946"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2109.6347191167315,
            "unit": "iter/sec",
            "range": "stddev: 0.00005916244472531727",
            "extra": "mean: 474.0157103684202 usec\nrounds: 1495"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 2070.3476093652744,
            "unit": "iter/sec",
            "range": "stddev: 0.00008054574300699711",
            "extra": "mean: 483.01067679479155 usec\nrounds: 362"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1665.423527911775,
            "unit": "iter/sec",
            "range": "stddev: 0.00007496622263753825",
            "extra": "mean: 600.4478640060227 usec\nrounds: 1353"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 3033.8889928769754,
            "unit": "iter/sec",
            "range": "stddev: 0.000042501541317244595",
            "extra": "mean: 329.60995024795557 usec\nrounds: 2010"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 141.6151475895077,
            "unit": "iter/sec",
            "range": "stddev: 0.00014740569569083987",
            "extra": "mean: 7.061391503814598 msec\nrounds: 131"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "c983e77d34a4b46b476870ce0d81c553305ee6bc",
          "message": "feat(sdk): v2.1.0 — InsertIfNotExists helper (#599) + ADR-031 CI restoration (#613)\n\n* feat(sdk): InsertIfNotExists — atomic insert-or-resolve helper (#599)\n\nCloses the racy 'query-then-create' idiom under concurrent writers of\nthe same uniquely-keyed payload: exactly one writer wins and the rest\nlearn the existing canonical node id without a second user-visible\nround trip into client code.\n\nAPI (both SDKs):\n\n  Go:     created, existing, err :=\n            scope.InsertIfNotExists(ctx, idempotencyKey, msg)\n  Python: created, existing =\n            await scope.insert_if_not_exists(msg, idempotency_key='...')\n\nExactly one of (created, existing) is non-empty on success. The helper\nforces wait_applied=True so the applier's outcome surfaces\nsynchronously (otherwise the loser of a unique race would see a\nphantom success — #606). A typed UniqueConstraintError on commit\ntriggers a follow-up GetNodeByKey using the (type_id, field_id, value)\ntuple the server attached to the error; the resolved id is returned.\n\nv2.1.0 boundary (deliberate scope):\n\n- SINGLE-FIELD unique only. A composite-unique violation (ADR-030)\n  re-raises the typed error without a follow-up lookup — there is no\n  GetByCompositeKey RPC in v2.x, so callers needing composite upsert\n  query themselves. Tracked for v2.2.\n\n- This is still TWO round trips (commit + GetNodeByKey on conflict).\n  The single-round-trip server-side path (NodeConflictPolicy_SKIP +\n  ExecuteAtomicResponse.existing_node_ids threaded back through the\n  handler) is v2.2; doing it now would require either extending the\n  idempotency cache schema or adding a direct applier→handler result\n  channel, neither of which I want to land half-done.\n\nThis release closes the correctness gap (the racy idiom) NOW with the\nprimitives already shipped in v2.0.x.\n\nTests\n-----\n\nGo (sdk/go/entdb/insert_if_not_exists_test.go):\n  - HappyPath:        commit succeeds → (created, '', nil); WaitApplied\n                      is forced on (pinned via mock.lastWaitApplied*)\n  - SingleField:      *UniqueConstraintError → GetNodeByKey called with\n                      the UCE tuple → ('', existing, nil)\n  - Composite:        composite UCE → propagated; GetNodeByKey is NOT\n                      called\n  - Unrelated:        non-UCE error bubbles up unchanged\n  - NilMsg:           precondition short-circuits before any RPC\n\nPython (tests/python/unit/test_insert_if_not_exists.py): the same\nfive branches, using AsyncMock against the DbClient._grpc transport.\n\nCI: all four Go modules + python unit (454) + python integration (115)\ngreen locally.\n\n* docs(generated): refresh sdk-go.md (catches up v2.0.x + new InsertIfNotExists)\n\n`docs/generated/sdk-go.md` had been stale since the v2.0.x refactor\nlanded (WithWaitApplied / WithWaitTimeout / CommittedOffset were missing).\nv2.1.0 adds InsertIfNotExists, so regenerate end-to-end with the\ngo.mod-pinned toolchain (Go 1.25.0) per the regen-docs-with-pinned-go\nrule — newer local Go can drift the rendered output in ways the\n\"Docs Coverage & Examples\" CI gate flags as stale.\n\n* ci: restore green gates after ADR-031 (--seed-profile callers + stale snapshot)\n\nADR-031 removed the server's boot-seed flags (--seed-profile,\n--seed-tenant) but five call sites still passed them, so every CI run\nsince the ADR landed has been red on:\n\n  - Go SDK             (sdk/go/entdb/integration_test.go)\n  - Docs Coverage      (examples/_harness.py — runs runnable docs)\n  - Schema Compat      (.github/workflows/ci.yml + schema-compat.yml)\n  - Tier 1 benchmark   (tests/python/benchmarks/conftest.py +\n                        docker-compose.bench.yml)\n\nRecent PRs (#609, #610, #611) merged through despite the red gates.\nv2.1.0 deserves an actually-verified release.\n\nSites switched to empty-boot + API-driven bootstrap (mirror of\ntests/python/integration/conftest.py:_bootstrap_contract):\n\n* examples/_harness.py — provisions tenant + alice/bob via the gRPC\n  API after boot; lets the SDK auto-attach the schema on each\n  example's first ExecuteAtomic (hand-built descriptor fingerprints\n  differ from the SDK's canonicaliser → establish-or-reject would\n  reject the SDK write).\n\n* sdk/go/entdb/integration_test.go — adds bootstrapIntegrationContract\n  using a raw pb.EntDBServiceClient: CreateTenant + CreateUser +\n  AddTenantMember, then a self-describing ExecuteAtomic carrying the\n  itUserType (8001)/itProductType (8002)/edge 8101 SchemaDescriptor.\n\n* tests/python/benchmarks/conftest.py — adds _bootstrap_bench called\n  from the entdb_stub fixture (tenant + user + member). Tolerates\n  ALREADY_EXISTS so ENTDB_BENCH_ENDPOINT (an already-running container)\n  is a no-op.\n\n* .github/workflows/ci.yml + schema-compat.yml — drop the flags. The\n  step still runs entdb-schema check, just against an empty boot. With\n  the stale .schema-snapshot.json removed (see below) the boot+check\n  step's hashFiles() guard skips cleanly.\n\n* .schema-snapshot.json — DELETED. It carried names (pre-ADR-031\n  format) and was structurally incompatible with current GetSchema\n  output. Per ADR-032 the baseline is a customer artefact, not a\n  self-referential file in this repo — the workflows' `if:\n  hashFiles('.schema-snapshot.json') != ''` already guards on\n  presence, falling back to a 'No baseline — recommend committing\n  one' notice.\n\n* tests/python/benchmarks/docker-compose.bench.yml — drop the flags\n  from the EntDB service command. The bench corpus is still\n  name-keyed and will fail downstream of boot; that is a separate\n  pre-existing issue, tracked for a follow-up.\n\n* ci(bench): switch corpus to id-keyed payloads for EntDB seed (ADR-031)\n\nThe Postgres-vs-EntDB benchmark corpus generator produced\n`{title, description}` dict payloads consumed by BOTH backends. Per\nADR-031 (Bug C / CLAUDE.md invariant #6) the EntDB wire payload is\nfield-id-keyed, so the EntDB seed has been failing with\n`INVALID_ARGUMENT: payload contains non-field_id key \"title\"` ever\nsince name-keyed payloads were rejected.\n\n  - tests/python/benchmarks/conftest.py — corpus rows now carry a\n    sibling `payload_ids` ({'1': title, '2': description}) used by\n    `entdb_seeded`; `payload` (name-keyed) is retained verbatim\n    because the Postgres schema's JSONB expression-index keys on\n    `payload->>'title'`/`'description'` — both paths need their\n    historical shape.\n  - tests/python/benchmarks/bench_entdb.py — single-op + multi-op\n    write and update microbenchmarks now emit field-id-keyed\n    {'1': ..., '2': ...} maps to match the seed corpus.\n\nAll 12 bench_entdb cases pass locally with --benchmark-disable.",
          "timestamp": "2026-05-27T04:14:34+01:00",
          "tree_id": "96ef1a5d3d52d8e2d34387379dfdf7a1389458cb",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/c983e77d34a4b46b476870ce0d81c553305ee6bc"
        },
        "date": 1779851795537,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3966.9301370628386,
            "unit": "iter/sec",
            "range": "stddev: 0.000032259840719754424",
            "extra": "mean: 252.0840966310568 usec\nrounds: 1573"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2463.604230655086,
            "unit": "iter/sec",
            "range": "stddev: 0.000043390413505227024",
            "extra": "mean: 405.90935327875064 usec\nrounds: 1220"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1127.2463568955873,
            "unit": "iter/sec",
            "range": "stddev: 0.00009575135801853794",
            "extra": "mean: 887.1175266017084 usec\nrounds: 921"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 487.19285104792874,
            "unit": "iter/sec",
            "range": "stddev: 0.00013969167204215134",
            "extra": "mean: 2.0525752745530794 msec\nrounds: 448"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2089.1496085080234,
            "unit": "iter/sec",
            "range": "stddev: 0.00007442308185678784",
            "extra": "mean: 478.66366100709985 usec\nrounds: 1708"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2149.5638808589465,
            "unit": "iter/sec",
            "range": "stddev: 0.000078331218582934",
            "extra": "mean: 465.2106452404703 usec\nrounds: 1618"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2163.4758338385536,
            "unit": "iter/sec",
            "range": "stddev: 0.00007734979948412602",
            "extra": "mean: 462.21916804392816 usec\nrounds: 1815"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 2165.711336650853,
            "unit": "iter/sec",
            "range": "stddev: 0.00005475625716963207",
            "extra": "mean: 461.742053558459 usec\nrounds: 1363"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 2117.87855618665,
            "unit": "iter/sec",
            "range": "stddev: 0.00006570111855177002",
            "extra": "mean: 472.1706053819025 usec\nrounds: 446"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1748.14841242476,
            "unit": "iter/sec",
            "range": "stddev: 0.00006559079619679378",
            "extra": "mean: 572.03381182777 usec\nrounds: 1302"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 3183.1002032280453,
            "unit": "iter/sec",
            "range": "stddev: 0.000042189517570069795",
            "extra": "mean: 314.15913296913493 usec\nrounds: 2196"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 117.42808558122806,
            "unit": "iter/sec",
            "range": "stddev: 0.0020234759151915097",
            "extra": "mean: 8.515850318519192 msec\nrounds: 135"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "da4b0a226fa2679a6f3bd462e88ca241a0cb3905",
          "message": "fix(sdk/go): typed coordinates on single-field UniqueConstraintError (#614)\n\nLive-server integration tests for InsertIfNotExists surfaced a real\nbug the mock unit tests couldn't have caught: the SDK's\nparseUniqueConstraintFromStatus only decoded composite-unique\nviolations into typed (TypeID, FieldIDs, Values), and for single-field\nviolations returned a *UniqueConstraintError with TypeID/FieldID/Value\nall zero. (The code itself even had a comment admitting this:\n\"intentionally left for future structured-error work\".)\n\nInsertIfNotExists's post-conflict resolution is\n\n    s.client.transport.GetNodeByKey(ctx, tenantID, actor,\n        uce.TypeID, uce.FieldID, uce.Value)\n\nso the zero-coordinates path silently sent GetNodeByKey(0, 0, nil),\ngot back no node, and surfaced the raw UCE instead of the existing\nid — meaning InsertIfNotExists was BROKEN for single-field conflicts\nin v2.1.0. Mock-based unit tests hand-built UCE.TypeID/FieldID/Value\nso the gap survived review.\n\nFix\n---\n\nAdded parseSingleFieldUniqueDetail + decodePyRepr to errors.go.\nparser branch is now:\n\n  composite? -> ConstraintName, FieldIDs, Values, TypeID\n  single?    -> TypeID, FieldID, Value (NEW)\n  fallback   -> Message only (last resort)\n\ndecodePyRepr round-trips the server's pyRepr() rendering:\n\n  'foo'       -> \"foo\"  (single-quoted strings with \\\\ / \\' escapes)\n  42          -> int64(42)\n  1.5         -> float64(1.5)\n  True/False  -> bool\n  None        -> nil\n\nBig ints (>2^53) round-trip losslessly through int64 — matches the\nADR-028 typed-value path.\n\nTests\n-----\n\n1) sdk/go/entdb/errors_test.go — two new cases:\n   - SingleFieldExtractsCoordinates: pins the wire-form parse.\n   - SingleFieldScalarTypes: 8 sub-tests covering string + escapes +\n     int + big int + float + True + False + None.\n\n2) sdk/go/entdb/integration_test.go — two new -tags=integration cases\n   against a real entdb-server (uses testpb.Product / type_id 201):\n   - InsertIfNotExists_Created: happy path, node is queryable.\n   - InsertIfNotExists_ResolvesConflict: second write with the same\n     unique sku returns the FIRST writer's id; first writer's payload\n     is preserved (no upsert overwrite).\n\n3) tests/python/integration/test_insert_if_not_exists.py — mirror\n   suite against the Python SDK. The Python parser already handled\n   single-field details so no Python fix is needed; the integration\n   test guards future regressions through the full applier round\n   trip.\n\nThese are the regression pins the user asked for after v2.1.0 — the\nmock unit tests prove the SDK BRANCHING logic; the integration tests\nprove the END-TO-END wire path (which is where the bug actually was).\n\nCI: all four Go modules + python unit (459) + python integration\n(115 → 117 with the new cases) green locally.",
          "timestamp": "2026-05-27T04:50:16+01:00",
          "tree_id": "dfe2ada6be2bd2b388f7774b8add0182546a0909",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/da4b0a226fa2679a6f3bd462e88ca241a0cb3905"
        },
        "date": 1779853920876,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3317.803176013108,
            "unit": "iter/sec",
            "range": "stddev: 0.000026256285279264754",
            "extra": "mean: 301.40425665686 usec\nrounds: 1352"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2120.836503672797,
            "unit": "iter/sec",
            "range": "stddev: 0.00004483868710891783",
            "extra": "mean: 471.5120652951003 usec\nrounds: 1118"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1028.8592907090472,
            "unit": "iter/sec",
            "range": "stddev: 0.0000946905090042784",
            "extra": "mean: 971.9502064376961 usec\nrounds: 901"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 404.76617296580065,
            "unit": "iter/sec",
            "range": "stddev: 0.00035665622540928776",
            "extra": "mean: 2.470562183279312 msec\nrounds: 311"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1773.3118515738995,
            "unit": "iter/sec",
            "range": "stddev: 0.00015377467660110164",
            "extra": "mean: 563.9166055944711 usec\nrounds: 1430"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1805.6386427301607,
            "unit": "iter/sec",
            "range": "stddev: 0.00015532861003761717",
            "extra": "mean: 553.8206683968508 usec\nrounds: 1734"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1860.014095536083,
            "unit": "iter/sec",
            "range": "stddev: 0.00015589088005148638",
            "extra": "mean: 537.630334307647 usec\nrounds: 1026"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1671.2541873938578,
            "unit": "iter/sec",
            "range": "stddev: 0.000057022491784646443",
            "extra": "mean: 598.3530258550275 usec\nrounds: 1199"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1708.993737832408,
            "unit": "iter/sec",
            "range": "stddev: 0.00005485543074504271",
            "extra": "mean: 585.1396513999777 usec\nrounds: 393"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1440.7126614833307,
            "unit": "iter/sec",
            "range": "stddev: 0.00007201273000786754",
            "extra": "mean: 694.1009312505234 usec\nrounds: 1120"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2632.778178675739,
            "unit": "iter/sec",
            "range": "stddev: 0.00003805734630321361",
            "extra": "mean: 379.82690987775885 usec\nrounds: 1964"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 137.22205246509998,
            "unit": "iter/sec",
            "range": "stddev: 0.0001915562159013942",
            "extra": "mean: 7.287458408001385 msec\nrounds: 125"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "da4b0a226fa2679a6f3bd462e88ca241a0cb3905",
          "message": "fix(sdk/go): typed coordinates on single-field UniqueConstraintError (#614)\n\nLive-server integration tests for InsertIfNotExists surfaced a real\nbug the mock unit tests couldn't have caught: the SDK's\nparseUniqueConstraintFromStatus only decoded composite-unique\nviolations into typed (TypeID, FieldIDs, Values), and for single-field\nviolations returned a *UniqueConstraintError with TypeID/FieldID/Value\nall zero. (The code itself even had a comment admitting this:\n\"intentionally left for future structured-error work\".)\n\nInsertIfNotExists's post-conflict resolution is\n\n    s.client.transport.GetNodeByKey(ctx, tenantID, actor,\n        uce.TypeID, uce.FieldID, uce.Value)\n\nso the zero-coordinates path silently sent GetNodeByKey(0, 0, nil),\ngot back no node, and surfaced the raw UCE instead of the existing\nid — meaning InsertIfNotExists was BROKEN for single-field conflicts\nin v2.1.0. Mock-based unit tests hand-built UCE.TypeID/FieldID/Value\nso the gap survived review.\n\nFix\n---\n\nAdded parseSingleFieldUniqueDetail + decodePyRepr to errors.go.\nparser branch is now:\n\n  composite? -> ConstraintName, FieldIDs, Values, TypeID\n  single?    -> TypeID, FieldID, Value (NEW)\n  fallback   -> Message only (last resort)\n\ndecodePyRepr round-trips the server's pyRepr() rendering:\n\n  'foo'       -> \"foo\"  (single-quoted strings with \\\\ / \\' escapes)\n  42          -> int64(42)\n  1.5         -> float64(1.5)\n  True/False  -> bool\n  None        -> nil\n\nBig ints (>2^53) round-trip losslessly through int64 — matches the\nADR-028 typed-value path.\n\nTests\n-----\n\n1) sdk/go/entdb/errors_test.go — two new cases:\n   - SingleFieldExtractsCoordinates: pins the wire-form parse.\n   - SingleFieldScalarTypes: 8 sub-tests covering string + escapes +\n     int + big int + float + True + False + None.\n\n2) sdk/go/entdb/integration_test.go — two new -tags=integration cases\n   against a real entdb-server (uses testpb.Product / type_id 201):\n   - InsertIfNotExists_Created: happy path, node is queryable.\n   - InsertIfNotExists_ResolvesConflict: second write with the same\n     unique sku returns the FIRST writer's id; first writer's payload\n     is preserved (no upsert overwrite).\n\n3) tests/python/integration/test_insert_if_not_exists.py — mirror\n   suite against the Python SDK. The Python parser already handled\n   single-field details so no Python fix is needed; the integration\n   test guards future regressions through the full applier round\n   trip.\n\nThese are the regression pins the user asked for after v2.1.0 — the\nmock unit tests prove the SDK BRANCHING logic; the integration tests\nprove the END-TO-END wire path (which is where the bug actually was).\n\nCI: all four Go modules + python unit (459) + python integration\n(115 → 117 with the new cases) green locally.",
          "timestamp": "2026-05-27T04:50:16+01:00",
          "tree_id": "dfe2ada6be2bd2b388f7774b8add0182546a0909",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/da4b0a226fa2679a6f3bd462e88ca241a0cb3905"
        },
        "date": 1779853951072,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3015.0386218011126,
            "unit": "iter/sec",
            "range": "stddev: 0.0000291812760038199",
            "extra": "mean: 331.6707098772166 usec\nrounds: 1296"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2023.541906988861,
            "unit": "iter/sec",
            "range": "stddev: 0.00003503135506662768",
            "extra": "mean: 494.1829949487202 usec\nrounds: 1188"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 897.6920467208414,
            "unit": "iter/sec",
            "range": "stddev: 0.00009775301438255421",
            "extra": "mean: 1.1139677617206 msec\nrounds: 789"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 431.2349111829191,
            "unit": "iter/sec",
            "range": "stddev: 0.0001632211736157477",
            "extra": "mean: 2.3189217154448447 msec\nrounds: 369"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1869.8893726803537,
            "unit": "iter/sec",
            "range": "stddev: 0.00008676743213029152",
            "extra": "mean: 534.7909959863407 usec\nrounds: 1495"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1869.6371849245968,
            "unit": "iter/sec",
            "range": "stddev: 0.00008903264986263312",
            "extra": "mean: 534.8631317687075 usec\nrounds: 1791"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1956.0084547976442,
            "unit": "iter/sec",
            "range": "stddev: 0.00009011715281545151",
            "extra": "mean: 511.2452339084871 usec\nrounds: 1740"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1766.1354173690163,
            "unit": "iter/sec",
            "range": "stddev: 0.000042370962255536587",
            "extra": "mean: 566.2079986424167 usec\nrounds: 1473"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1754.3713708836688,
            "unit": "iter/sec",
            "range": "stddev: 0.000051221720581191236",
            "extra": "mean: 570.0047416393398 usec\nrounds: 329"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1479.2536513813707,
            "unit": "iter/sec",
            "range": "stddev: 0.00006910402335507598",
            "extra": "mean: 676.0165838131753 usec\nrounds: 1223"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2513.0530391416423,
            "unit": "iter/sec",
            "range": "stddev: 0.00003536063617547562",
            "extra": "mean: 397.9223615358153 usec\nrounds: 1820"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 125.68397192162205,
            "unit": "iter/sec",
            "range": "stddev: 0.0011049346300317313",
            "extra": "mean: 7.956464016140511 msec\nrounds: 124"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "014843b80acdf62b6d66d103b0e7db0d33be21e0",
          "message": "chore(security): harden CI + supply chain — govulncheck, pip-audit, CodeQL, cosign image signing, SBOM, SECURITY.md (#615)\n\n* chore(security): harden CI + supply chain (items 3–7 of repo gap audit)\n\nBundles the code-side security hardening from the gap audit.\nRepo-settings items (branch protection on main, squash-only,\nsecret scanning + push protection + Dependabot security updates)\nwere applied directly via gh api in the same session.\n\nChanges\n-------\n\ndependabot.yml\n  - Add gomod entries for all four Go modules (server/go,\n    sdk/go/entdb, tests/contract, tools/protoc-gen-entdb-keys).\n    Each grouped so a transitive bump lands as one PR per module\n    instead of N.\n\nSECURITY.md (new)\n  - Supported versions: v2.1.x active, v2.0.x critical-only.\n  - Private vulnerability reporting via GitHub Security Advisories\n    + fallback email.\n  - 72h ack / 7d initial assessment / 30d (high+critical) or 90d\n    (med+low) coordinated disclosure target.\n  - Out-of-scope clauses for unsupported versions + the\n    documented '-oauth-*' / TLS-disabled dev modes that already\n    warn at boot.\n\nci.yml\n  - Pin trivy-action to @0.28.0 (was @master — moving target).\n  - New 'Go Vulnerability Scan' job: govulncheck against all four\n    Go modules. Unlike Trivy this uses CALL-GRAPH REACHABILITY —\n    a CVE in unused transitive symbols doesn't false-positive.\n  - New 'Python Vulnerability Scan' job: pip-audit --strict\n    against the installed SDK dep tree.\n  - Both new jobs wired into 'All Checks Passed' aggregator (now\n    a required status check per the branch-protection rule applied\n    today).\n\nrelease.yml\n  - Image signing with cosign (keyless, GitHub OIDC -> Fulcio\n    short-lived cert; no signing key to rotate).\n  - SBOM (SPDX-JSON) generated by syft and attached as release\n    asset.\n  - Build provenance attestation via actions/attest-build-\n    provenance — SLSA-3, verifiable with\n    'gh attestation verify oci://<image>@<digest>'.\n  - Added id-token: write + attestations: write permissions to\n    the merge job (needed for OIDC + the attestations API).\n  - New 'Resolve image digest' step pins everything below to the\n    content-addressable digest (tags are mutable; digests aren't).\n\ncodeql.yml (new)\n  - Semantic SAST for Go + Python.\n  - PRs touching *.go or *.py + weekly cron on main.\n  - security-extended query suite for taint-tracking +\n    security-quality coverage beyond the default pack.\n\nVerification (post-release)\n---------------------------\n\n  # image signature + identity\n  cosign verify ghcr.io/elloloop/tenant-shard-db@<digest> \\\n    --certificate-identity-regexp '^https://github.com/elloloop/tenant-shard-db/' \\\n    --certificate-oidc-issuer https://token.actions.githubusercontent.com\n\n  # build provenance\n  gh attestation verify oci://ghcr.io/elloloop/tenant-shard-db@<digest> \\\n    --repo elloloop/tenant-shard-db\n\n  # SBOM\n  gh release download v<version> --pattern 'sbom-*.spdx.json'\n\n* fix(deps): GO-2026-5026 — bump golang.org/x/net v0.54.0 -> v0.55.0\n\ngovulncheck (added in this PR) caught a REAL reachable CVE on first\nrun — exactly the kind of finding Trivy's manifest-only scan would\nhave missed:\n\n  Vulnerability: GO-2026-5026\n  Module:        golang.org/x/net\n  Found in:      v0.54.0\n  Fixed in:      v0.55.0\n  Reachable:     auth.discoverOIDC -> http.Client.Do -> idna.ToASCII\n                 (internal/auth/jwks.go:309 -- our OIDC discovery path)\n\nThe vulnerability is a failure to reject ASCII-only Punycode-encoded\nlabels in golang.org/x/net/idna, which our server reaches whenever\nit resolves the OIDC issuer URL during JWKS discovery.\n\nBumped in all four Go modules (server/go, sdk/go/entdb, tests/contract,\ntools/protoc-gen-entdb-keys); transitively pulled golang.org/x/sys +\ngolang.org/x/text minor bumps via 'go mod tidy'. All four modules\nreport 'No vulnerabilities found' under govulncheck locally; tests\npass on all four.\n\nAlso fixed in this commit:\n\n  - aquasecurity/trivy-action tag pin: '@0.28.0' did not exist;\n    the actual tags are 'v0.28.0' (with v prefix) and a non-v\n    '0.30.0'. Re-pinned to '@0.30.0', the canonical no-prefix form\n    matching the action's README quickstart.\n  - tools/protoc-gen-entdb-keys go.mod: 'go 1.22' -> 'go 1.25.0',\n    the floor x/net@v0.55.0 requires.\n\n* fix(ci): Trivy tag pin '@v0.36.0' — all real trivy-action tags use the v prefix\n\nThe /releases listing showed one non-v tag (0.35.0) that turned out\nto be a release without a matching git ref. Every actual /tags entry\nis v-prefixed. Re-pinned to the latest valid ref so the Security\nScan job stops resolving to a non-existent version.",
          "timestamp": "2026-05-27T05:29:07+01:00",
          "tree_id": "40fb3c7fcecd2b13b376a70d24f773a978be6faa",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/014843b80acdf62b6d66d103b0e7db0d33be21e0"
        },
        "date": 1779856255063,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3060.8714461234736,
            "unit": "iter/sec",
            "range": "stddev: 0.000026480169578960534",
            "extra": "mean: 326.70434469454045 usec\nrounds: 1555"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2022.938484965324,
            "unit": "iter/sec",
            "range": "stddev: 0.00004900745188804363",
            "extra": "mean: 494.3304047216944 usec\nrounds: 1186"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 869.70046080514,
            "unit": "iter/sec",
            "range": "stddev: 0.000137511400480059",
            "extra": "mean: 1.1498211683988682 msec\nrounds: 481"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 432.8090453581958,
            "unit": "iter/sec",
            "range": "stddev: 0.00020694985433149203",
            "extra": "mean: 2.310487756031977 msec\nrounds: 373"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1907.0796477728602,
            "unit": "iter/sec",
            "range": "stddev: 0.00012463501291115842",
            "extra": "mean: 524.3619484733254 usec\nrounds: 1572"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1891.7683256956877,
            "unit": "iter/sec",
            "range": "stddev: 0.00010684257230329978",
            "extra": "mean: 528.6059537085522 usec\nrounds: 1901"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1976.2890368816572,
            "unit": "iter/sec",
            "range": "stddev: 0.00010297490639946858",
            "extra": "mean: 505.9988601555357 usec\nrounds: 1802"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1795.190635072241,
            "unit": "iter/sec",
            "range": "stddev: 0.00005748000108774043",
            "extra": "mean: 557.043904119831 usec\nrounds: 1335"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1774.074585909126,
            "unit": "iter/sec",
            "range": "stddev: 0.0000618280296421432",
            "extra": "mean: 563.674158878472 usec\nrounds: 321"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1505.2133341045148,
            "unit": "iter/sec",
            "range": "stddev: 0.00006967588832037069",
            "extra": "mean: 664.3576543885206 usec\nrounds: 1276"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2540.2584051605522,
            "unit": "iter/sec",
            "range": "stddev: 0.00003770587752825322",
            "extra": "mean: 393.66073859592126 usec\nrounds: 1951"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 129.0262312908626,
            "unit": "iter/sec",
            "range": "stddev: 0.0006694637754018673",
            "extra": "mean: 7.7503620000006785 msec\nrounds: 122"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "014843b80acdf62b6d66d103b0e7db0d33be21e0",
          "message": "chore(security): harden CI + supply chain — govulncheck, pip-audit, CodeQL, cosign image signing, SBOM, SECURITY.md (#615)\n\n* chore(security): harden CI + supply chain (items 3–7 of repo gap audit)\n\nBundles the code-side security hardening from the gap audit.\nRepo-settings items (branch protection on main, squash-only,\nsecret scanning + push protection + Dependabot security updates)\nwere applied directly via gh api in the same session.\n\nChanges\n-------\n\ndependabot.yml\n  - Add gomod entries for all four Go modules (server/go,\n    sdk/go/entdb, tests/contract, tools/protoc-gen-entdb-keys).\n    Each grouped so a transitive bump lands as one PR per module\n    instead of N.\n\nSECURITY.md (new)\n  - Supported versions: v2.1.x active, v2.0.x critical-only.\n  - Private vulnerability reporting via GitHub Security Advisories\n    + fallback email.\n  - 72h ack / 7d initial assessment / 30d (high+critical) or 90d\n    (med+low) coordinated disclosure target.\n  - Out-of-scope clauses for unsupported versions + the\n    documented '-oauth-*' / TLS-disabled dev modes that already\n    warn at boot.\n\nci.yml\n  - Pin trivy-action to @0.28.0 (was @master — moving target).\n  - New 'Go Vulnerability Scan' job: govulncheck against all four\n    Go modules. Unlike Trivy this uses CALL-GRAPH REACHABILITY —\n    a CVE in unused transitive symbols doesn't false-positive.\n  - New 'Python Vulnerability Scan' job: pip-audit --strict\n    against the installed SDK dep tree.\n  - Both new jobs wired into 'All Checks Passed' aggregator (now\n    a required status check per the branch-protection rule applied\n    today).\n\nrelease.yml\n  - Image signing with cosign (keyless, GitHub OIDC -> Fulcio\n    short-lived cert; no signing key to rotate).\n  - SBOM (SPDX-JSON) generated by syft and attached as release\n    asset.\n  - Build provenance attestation via actions/attest-build-\n    provenance — SLSA-3, verifiable with\n    'gh attestation verify oci://<image>@<digest>'.\n  - Added id-token: write + attestations: write permissions to\n    the merge job (needed for OIDC + the attestations API).\n  - New 'Resolve image digest' step pins everything below to the\n    content-addressable digest (tags are mutable; digests aren't).\n\ncodeql.yml (new)\n  - Semantic SAST for Go + Python.\n  - PRs touching *.go or *.py + weekly cron on main.\n  - security-extended query suite for taint-tracking +\n    security-quality coverage beyond the default pack.\n\nVerification (post-release)\n---------------------------\n\n  # image signature + identity\n  cosign verify ghcr.io/elloloop/tenant-shard-db@<digest> \\\n    --certificate-identity-regexp '^https://github.com/elloloop/tenant-shard-db/' \\\n    --certificate-oidc-issuer https://token.actions.githubusercontent.com\n\n  # build provenance\n  gh attestation verify oci://ghcr.io/elloloop/tenant-shard-db@<digest> \\\n    --repo elloloop/tenant-shard-db\n\n  # SBOM\n  gh release download v<version> --pattern 'sbom-*.spdx.json'\n\n* fix(deps): GO-2026-5026 — bump golang.org/x/net v0.54.0 -> v0.55.0\n\ngovulncheck (added in this PR) caught a REAL reachable CVE on first\nrun — exactly the kind of finding Trivy's manifest-only scan would\nhave missed:\n\n  Vulnerability: GO-2026-5026\n  Module:        golang.org/x/net\n  Found in:      v0.54.0\n  Fixed in:      v0.55.0\n  Reachable:     auth.discoverOIDC -> http.Client.Do -> idna.ToASCII\n                 (internal/auth/jwks.go:309 -- our OIDC discovery path)\n\nThe vulnerability is a failure to reject ASCII-only Punycode-encoded\nlabels in golang.org/x/net/idna, which our server reaches whenever\nit resolves the OIDC issuer URL during JWKS discovery.\n\nBumped in all four Go modules (server/go, sdk/go/entdb, tests/contract,\ntools/protoc-gen-entdb-keys); transitively pulled golang.org/x/sys +\ngolang.org/x/text minor bumps via 'go mod tidy'. All four modules\nreport 'No vulnerabilities found' under govulncheck locally; tests\npass on all four.\n\nAlso fixed in this commit:\n\n  - aquasecurity/trivy-action tag pin: '@0.28.0' did not exist;\n    the actual tags are 'v0.28.0' (with v prefix) and a non-v\n    '0.30.0'. Re-pinned to '@0.30.0', the canonical no-prefix form\n    matching the action's README quickstart.\n  - tools/protoc-gen-entdb-keys go.mod: 'go 1.22' -> 'go 1.25.0',\n    the floor x/net@v0.55.0 requires.\n\n* fix(ci): Trivy tag pin '@v0.36.0' — all real trivy-action tags use the v prefix\n\nThe /releases listing showed one non-v tag (0.35.0) that turned out\nto be a release without a matching git ref. Every actual /tags entry\nis v-prefixed. Re-pinned to the latest valid ref so the Security\nScan job stops resolving to a non-existent version.",
          "timestamp": "2026-05-27T05:29:07+01:00",
          "tree_id": "40fb3c7fcecd2b13b376a70d24f773a978be6faa",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/014843b80acdf62b6d66d103b0e7db0d33be21e0"
        },
        "date": 1779856292055,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3350.2194537007413,
            "unit": "iter/sec",
            "range": "stddev: 0.00003252390957117978",
            "extra": "mean: 298.48790917125547 usec\nrounds: 1134"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2126.58726516618,
            "unit": "iter/sec",
            "range": "stddev: 0.000040478990498677945",
            "extra": "mean: 470.2369925655772 usec\nrounds: 1076"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1023.723686997266,
            "unit": "iter/sec",
            "range": "stddev: 0.00009694924200212903",
            "extra": "mean: 976.8260837386199 usec\nrounds: 824"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 450.5871003870472,
            "unit": "iter/sec",
            "range": "stddev: 0.00015269150988807576",
            "extra": "mean: 2.2193267386949507 msec\nrounds: 398"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1917.0053331780337,
            "unit": "iter/sec",
            "range": "stddev: 0.00008526300382878715",
            "extra": "mean: 521.6469577276494 usec\nrounds: 1514"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1923.308727413427,
            "unit": "iter/sec",
            "range": "stddev: 0.00009748672694051994",
            "extra": "mean: 519.9373276618236 usec\nrounds: 1822"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1955.2575550074635,
            "unit": "iter/sec",
            "range": "stddev: 0.00009544951598571923",
            "extra": "mean: 511.4415732285371 usec\nrounds: 1666"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1765.9945479192438,
            "unit": "iter/sec",
            "range": "stddev: 0.00009327406631481421",
            "extra": "mean: 566.2531637927392 usec\nrounds: 580"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1797.903075276228,
            "unit": "iter/sec",
            "range": "stddev: 0.00002663289444715588",
            "extra": "mean: 556.203509383486 usec\nrounds: 373"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1468.5319209577349,
            "unit": "iter/sec",
            "range": "stddev: 0.00010035738872702396",
            "extra": "mean: 680.9521711641299 usec\nrounds: 1186"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2538.744245631561,
            "unit": "iter/sec",
            "range": "stddev: 0.00005160657599906423",
            "extra": "mean: 393.89552599506953 usec\nrounds: 1808"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 111.83417296139454,
            "unit": "iter/sec",
            "range": "stddev: 0.0013868472834704514",
            "extra": "mean: 8.941810660550086 msec\nrounds: 109"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "d4e4eb771f6992379841a4a1e5de7a889d19ce79",
          "message": "fix(release): attach SBOMs to GitHub Release in post-create-release job (#620)\n\nv2.1.2 surfaced an ordering bug in the v2.1.2-shipped SBOM pipeline:\n\n  anchore/sbom-action's 'upload-release-assets: true' runs in the\n  'merge' job. But the 'create-release' job runs LATER (needs:\n  [merge]). At the time the SBOM step executes the release does not\n  exist yet, so the action silently falls back to artifact-only.\n  v2.1.2's SBOMs landed as workflow artifacts; the release page\n  ended up missing 'sbom-server-v2.1.2.spdx.json' and\n  'sbom-schema-v2.1.2.spdx.json' (retrofitted manually post-hoc).\n\nFix\n---\n\n1) merge.Generate SBOM step: turn off 'upload-release-assets' (it's\n   a no-op there anyway) and pin a deterministic 'artifact-name' so\n   the post-release job can resolve it across both matrix legs.\n\n2) New job 'attach-sboms-to-release' that needs both 'merge' and\n   'create-release', downloads the SBOM workflow artifacts, renames\n   them to 'sbom-<target>-<tag>.spdx.json' (matching the\n   entdb-schema-<tag>-... naming convention already on the release),\n   and uploads via 'gh release upload --clobber'.\n\nv2.1.3 is the first release where SBOMs land automatically on the\nrelease page; v2.1.2 was patched manually in the same session this\nfix was authored.\n\nVerification (post-v2.1.3):\n  gh release view v2.1.3 --json assets --jq '[.assets[] | select(.name | startswith(\"sbom-\")) | .name]'\n  # expect: ['sbom-schema-v2.1.3.spdx.json', 'sbom-server-v2.1.3.spdx.json']",
          "timestamp": "2026-05-27T05:49:41+01:00",
          "tree_id": "840e343c966109eda66f9d1766344837d9a5fa35",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/d4e4eb771f6992379841a4a1e5de7a889d19ce79"
        },
        "date": 1779857491714,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3027.5269978504357,
            "unit": "iter/sec",
            "range": "stddev: 0.00002889820224726261",
            "extra": "mean: 330.3025871313473 usec\nrounds: 1492"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2015.2675307447894,
            "unit": "iter/sec",
            "range": "stddev: 0.00004046375301372351",
            "extra": "mean: 496.2120337593225 usec\nrounds: 1096"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 903.8146844819253,
            "unit": "iter/sec",
            "range": "stddev: 0.00010355040328090559",
            "extra": "mean: 1.106421501187723 msec\nrounds: 421"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 380.17575851630687,
            "unit": "iter/sec",
            "range": "stddev: 0.00041309211864180547",
            "extra": "mean: 2.6303623458335443 msec\nrounds: 240"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1776.1465418674843,
            "unit": "iter/sec",
            "range": "stddev: 0.00012572590224725004",
            "extra": "mean: 563.0166072606686 usec\nrounds: 1515"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1769.4293017080595,
            "unit": "iter/sec",
            "range": "stddev: 0.0001293515438821527",
            "extra": "mean: 565.1539731113774 usec\nrounds: 1562"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1810.4966313368795,
            "unit": "iter/sec",
            "range": "stddev: 0.00013521712898073002",
            "extra": "mean: 552.3346371882476 usec\nrounds: 1323"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1772.7253315188811,
            "unit": "iter/sec",
            "range": "stddev: 0.000041071515505740424",
            "extra": "mean: 564.1031818184684 usec\nrounds: 1331"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1772.6777040583947,
            "unit": "iter/sec",
            "range": "stddev: 0.000050173653819227886",
            "extra": "mean: 564.1183378741579 usec\nrounds: 367"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1481.9679552704915,
            "unit": "iter/sec",
            "range": "stddev: 0.00006804474310133033",
            "extra": "mean: 674.7784231390335 usec\nrounds: 1236"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2527.7224487422704,
            "unit": "iter/sec",
            "range": "stddev: 0.0000376669997863469",
            "extra": "mean: 395.61305494500556 usec\nrounds: 1820"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 136.90263098861587,
            "unit": "iter/sec",
            "range": "stddev: 0.0002331596087170724",
            "extra": "mean: 7.304461519685147 msec\nrounds: 127"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "d4e4eb771f6992379841a4a1e5de7a889d19ce79",
          "message": "fix(release): attach SBOMs to GitHub Release in post-create-release job (#620)\n\nv2.1.2 surfaced an ordering bug in the v2.1.2-shipped SBOM pipeline:\n\n  anchore/sbom-action's 'upload-release-assets: true' runs in the\n  'merge' job. But the 'create-release' job runs LATER (needs:\n  [merge]). At the time the SBOM step executes the release does not\n  exist yet, so the action silently falls back to artifact-only.\n  v2.1.2's SBOMs landed as workflow artifacts; the release page\n  ended up missing 'sbom-server-v2.1.2.spdx.json' and\n  'sbom-schema-v2.1.2.spdx.json' (retrofitted manually post-hoc).\n\nFix\n---\n\n1) merge.Generate SBOM step: turn off 'upload-release-assets' (it's\n   a no-op there anyway) and pin a deterministic 'artifact-name' so\n   the post-release job can resolve it across both matrix legs.\n\n2) New job 'attach-sboms-to-release' that needs both 'merge' and\n   'create-release', downloads the SBOM workflow artifacts, renames\n   them to 'sbom-<target>-<tag>.spdx.json' (matching the\n   entdb-schema-<tag>-... naming convention already on the release),\n   and uploads via 'gh release upload --clobber'.\n\nv2.1.3 is the first release where SBOMs land automatically on the\nrelease page; v2.1.2 was patched manually in the same session this\nfix was authored.\n\nVerification (post-v2.1.3):\n  gh release view v2.1.3 --json assets --jq '[.assets[] | select(.name | startswith(\"sbom-\")) | .name]'\n  # expect: ['sbom-schema-v2.1.3.spdx.json', 'sbom-server-v2.1.3.spdx.json']",
          "timestamp": "2026-05-27T05:49:41+01:00",
          "tree_id": "840e343c966109eda66f9d1766344837d9a5fa35",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/d4e4eb771f6992379841a4a1e5de7a889d19ce79"
        },
        "date": 1779857543239,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3310.1634154139892,
            "unit": "iter/sec",
            "range": "stddev: 0.000025728720802453428",
            "extra": "mean: 302.0998888886982 usec\nrounds: 756"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2126.1733127210273,
            "unit": "iter/sec",
            "range": "stddev: 0.00004290686708561689",
            "extra": "mean: 470.3285447225481 usec\nrounds: 1118"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1029.9879114064454,
            "unit": "iter/sec",
            "range": "stddev: 0.00009675431685722973",
            "extra": "mean: 970.8851812003336 usec\nrounds: 883"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 453.31447978966077,
            "unit": "iter/sec",
            "range": "stddev: 0.00014946170941062688",
            "extra": "mean: 2.205974096534492 msec\nrounds: 404"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1928.202985536573,
            "unit": "iter/sec",
            "range": "stddev: 0.00008263902256816608",
            "extra": "mean: 518.617597577116 usec\nrounds: 1568"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1934.4960746825334,
            "unit": "iter/sec",
            "range": "stddev: 0.00009433405313289567",
            "extra": "mean: 516.9304880414959 usec\nrounds: 1547"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1967.709287798604,
            "unit": "iter/sec",
            "range": "stddev: 0.00008515064415711157",
            "extra": "mean: 508.20515317014167 usec\nrounds: 1593"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1845.9824118682304,
            "unit": "iter/sec",
            "range": "stddev: 0.00004567358941807732",
            "extra": "mean: 541.716970633511 usec\nrounds: 1294"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1844.996826804927,
            "unit": "iter/sec",
            "range": "stddev: 0.00005062734702080499",
            "extra": "mean: 542.0063522449249 usec\nrounds: 423"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1545.0762561709544,
            "unit": "iter/sec",
            "range": "stddev: 0.00007873322612209287",
            "extra": "mean: 647.2172464019507 usec\nrounds: 1181"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2647.2233660842344,
            "unit": "iter/sec",
            "range": "stddev: 0.000037438733120360856",
            "extra": "mean: 377.7542963740144 usec\nrounds: 1903"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 133.6799830292416,
            "unit": "iter/sec",
            "range": "stddev: 0.0005625324336095549",
            "extra": "mean: 7.480551518182469 msec\nrounds: 110"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "6c38ed1508bb71606db4c6f32c8400e1c6311fa7",
          "message": "fix(release): gh release upload needs --repo when actions/checkout is skipped (#621)\n\nv2.1.3's attach-sboms-to-release job failed at the last step:\n\n  failed to run git: fatal: not a git repository (or any of the\n  parent directories): .git\n  ##[error]Process completed with exit code 1.\n\n`gh release upload` infers the repo from the local git remote when\ncalled inside a checkout. This job intentionally skips\nactions/checkout (the SBOMs arrive via download-artifact, no source\nneeded), so the runner has no .git for gh to read.\n\nFix: pass --repo ${{ github.repository }} explicitly. v2.1.3 was\npatched manually post-hoc; v2.1.4 is the first release where this\nlands without a retrofit.",
          "timestamp": "2026-05-27T06:08:23+01:00",
          "tree_id": "800d11f4f1a64c30bb2c30b21a18c59a2a8e7d08",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/6c38ed1508bb71606db4c6f32c8400e1c6311fa7"
        },
        "date": 1779858612912,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3233.2076854502457,
            "unit": "iter/sec",
            "range": "stddev: 0.00002626658332317322",
            "extra": "mean: 309.290369591814 usec\nrounds: 1618"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2076.9125986238737,
            "unit": "iter/sec",
            "range": "stddev: 0.000048760697465083214",
            "extra": "mean: 481.48391061934075 usec\nrounds: 1130"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 895.4109069396999,
            "unit": "iter/sec",
            "range": "stddev: 0.0001297039340889008",
            "extra": "mean: 1.1168056947371352 msec\nrounds: 665"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 443.06469307700587,
            "unit": "iter/sec",
            "range": "stddev: 0.0001447869057995098",
            "extra": "mean: 2.2570067433159187 msec\nrounds: 374"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1968.977230824882,
            "unit": "iter/sec",
            "range": "stddev: 0.0000678154721667125",
            "extra": "mean: 507.87788926388987 usec\nrounds: 1481"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1964.8753917441095,
            "unit": "iter/sec",
            "range": "stddev: 0.00007261870334971346",
            "extra": "mean: 508.93812615381995 usec\nrounds: 1625"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2056.557061351202,
            "unit": "iter/sec",
            "range": "stddev: 0.00008149558465424474",
            "extra": "mean: 486.2495764367358 usec\nrounds: 1740"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1853.2054413906214,
            "unit": "iter/sec",
            "range": "stddev: 0.00005008997851899524",
            "extra": "mean: 539.6055815860399 usec\nrounds: 1097"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1847.0691524364267,
            "unit": "iter/sec",
            "range": "stddev: 0.00004629550456302428",
            "extra": "mean: 541.3982463411957 usec\nrounds: 410"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1545.4772239097279,
            "unit": "iter/sec",
            "range": "stddev: 0.00007314042276923181",
            "extra": "mean: 647.0493285369895 usec\nrounds: 1251"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2629.235599389174,
            "unit": "iter/sec",
            "range": "stddev: 0.00003266397792450516",
            "extra": "mean: 380.3386810342599 usec\nrounds: 1972"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 135.62698798413012,
            "unit": "iter/sec",
            "range": "stddev: 0.00022098559369458266",
            "extra": "mean: 7.373163813952804 msec\nrounds: 129"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "6c38ed1508bb71606db4c6f32c8400e1c6311fa7",
          "message": "fix(release): gh release upload needs --repo when actions/checkout is skipped (#621)\n\nv2.1.3's attach-sboms-to-release job failed at the last step:\n\n  failed to run git: fatal: not a git repository (or any of the\n  parent directories): .git\n  ##[error]Process completed with exit code 1.\n\n`gh release upload` infers the repo from the local git remote when\ncalled inside a checkout. This job intentionally skips\nactions/checkout (the SBOMs arrive via download-artifact, no source\nneeded), so the runner has no .git for gh to read.\n\nFix: pass --repo ${{ github.repository }} explicitly. v2.1.3 was\npatched manually post-hoc; v2.1.4 is the first release where this\nlands without a retrofit.",
          "timestamp": "2026-05-27T06:08:23+01:00",
          "tree_id": "800d11f4f1a64c30bb2c30b21a18c59a2a8e7d08",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/6c38ed1508bb71606db4c6f32c8400e1c6311fa7"
        },
        "date": 1779858643113,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3015.4401275471305,
            "unit": "iter/sec",
            "range": "stddev: 0.00003108581333049291",
            "extra": "mean: 331.62654793396166 usec\nrounds: 1210"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1962.8409722360584,
            "unit": "iter/sec",
            "range": "stddev: 0.00005106451749911422",
            "extra": "mean: 509.4656236265565 usec\nrounds: 1092"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 879.2603737993198,
            "unit": "iter/sec",
            "range": "stddev: 0.00011000639827914732",
            "extra": "mean: 1.137319535598948 msec\nrounds: 618"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 427.64676063268445,
            "unit": "iter/sec",
            "range": "stddev: 0.0002480203607084975",
            "extra": "mean: 2.3383785218448616 msec\nrounds: 412"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1885.7677058505765,
            "unit": "iter/sec",
            "range": "stddev: 0.00008737990152065776",
            "extra": "mean: 530.2880078482145 usec\nrounds: 1529"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1872.8954193057511,
            "unit": "iter/sec",
            "range": "stddev: 0.00009409331202065402",
            "extra": "mean: 533.9326423098852 usec\nrounds: 1697"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1978.2326248020074,
            "unit": "iter/sec",
            "range": "stddev: 0.00007983521999290182",
            "extra": "mean: 505.50172283205853 usec\nrounds: 1822"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1655.9097844930143,
            "unit": "iter/sec",
            "range": "stddev: 0.00005976508683728818",
            "extra": "mean: 603.8976334125397 usec\nrounds: 1263"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1656.182715306286,
            "unit": "iter/sec",
            "range": "stddev: 0.000052777367484614305",
            "extra": "mean: 603.7981140354221 usec\nrounds: 342"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1391.9062676626575,
            "unit": "iter/sec",
            "range": "stddev: 0.00007513698214250648",
            "extra": "mean: 718.4391817411946 usec\nrounds: 1183"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2508.959462213454,
            "unit": "iter/sec",
            "range": "stddev: 0.000031776096168589777",
            "extra": "mean: 398.5716051058792 usec\nrounds: 1841"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 132.49584887161535,
            "unit": "iter/sec",
            "range": "stddev: 0.00024670087050823286",
            "extra": "mean: 7.5474062660557095 msec\nrounds: 109"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "49699333+dependabot[bot]@users.noreply.github.com",
            "name": "dependabot[bot]",
            "username": "dependabot[bot]"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "87a789a6d36b2952bccf461981b16b38dee73a18",
          "message": "chore(deps): bump golang from 1.25-bookworm to 1.26-bookworm (#575)\n\nBumps golang from 1.25-bookworm to 1.26-bookworm.\n\n---\nupdated-dependencies:\n- dependency-name: golang\n  dependency-version: 1.26-bookworm\n  dependency-type: direct:production\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>\nCo-authored-by: dependabot[bot] <49699333+dependabot[bot]@users.noreply.github.com>",
          "timestamp": "2026-05-28T15:17:26Z",
          "tree_id": "db51a7859409bbd4b8254be68c6723191505f711",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/87a789a6d36b2952bccf461981b16b38dee73a18"
        },
        "date": 1779981567679,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3001.6344174450373,
            "unit": "iter/sec",
            "range": "stddev: 0.000028175548255375595",
            "extra": "mean: 333.1518302789153 usec\nrounds: 1255"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1987.4964361611835,
            "unit": "iter/sec",
            "range": "stddev: 0.00004553396797162324",
            "extra": "mean: 503.14555629165477 usec\nrounds: 755"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 894.3921609175827,
            "unit": "iter/sec",
            "range": "stddev: 0.00011456981209836331",
            "extra": "mean: 1.1180777780678122 msec\nrounds: 766"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 427.37599113025243,
            "unit": "iter/sec",
            "range": "stddev: 0.00019072360981019692",
            "extra": "mean: 2.339860031340009 msec\nrounds: 351"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1853.1356877523087,
            "unit": "iter/sec",
            "range": "stddev: 0.00011393204584472757",
            "extra": "mean: 539.6258928092375 usec\nrounds: 1474"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1845.5213213620987,
            "unit": "iter/sec",
            "range": "stddev: 0.00011831637255198938",
            "extra": "mean: 541.852314803897 usec\nrounds: 1763"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1923.2527355201887,
            "unit": "iter/sec",
            "range": "stddev: 0.00010172526291908584",
            "extra": "mean: 519.9524646611391 usec\nrounds: 2094"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1622.8677181533508,
            "unit": "iter/sec",
            "range": "stddev: 0.00010412248516434613",
            "extra": "mean: 616.1931676956965 usec\nrounds: 972"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1638.3330542265412,
            "unit": "iter/sec",
            "range": "stddev: 0.000025277430744203796",
            "extra": "mean: 610.3765027631096 usec\nrounds: 362"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1382.1423797115704,
            "unit": "iter/sec",
            "range": "stddev: 0.0000776694446454527",
            "extra": "mean: 723.5144618086907 usec\nrounds: 1139"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2469.6385175294076,
            "unit": "iter/sec",
            "range": "stddev: 0.000030074491041657992",
            "extra": "mean: 404.91755894720427 usec\nrounds: 1900"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 121.89547394894942,
            "unit": "iter/sec",
            "range": "stddev: 0.001512343527607807",
            "extra": "mean: 8.203750045869679 msec\nrounds: 109"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "8fdcef68fc0794aae93ce0c82f8c462f1e72b585",
          "message": "feat(v2.2.0): single-RTT InsertIfNotExists for single-field and composite unique (#622)\n\nv2.1.x InsertIfNotExists made two round trips on a unique-key\ncollision (commit -> typed UCE -> SDK-side GetNodeByKey) and could\nonly resolve SINGLE-FIELD collisions because composite UCE detail\nhas no companion GetByCompositeKey RPC. v2.2 closes both gaps with\na server-side conflict-resolution path:\n\n  proto/entdb/v1/entdb.proto:\n    - NodeConflictPolicy enum (ERROR=0 default, SKIP=1)\n    - CreateNodeOp.on_conflict (field 13)\n    - ExecuteAtomicResponse.existing_node_ids (field 8) — index-\n      aligned twin of created_node_ids; at any given index exactly\n      one is non-empty.\n\n  applier (server/go/internal/apply/):\n    - applyCreateNode reads op[\"on_conflict\"]. On a typed\n      UNIQUE_VIOLATION + skip, it calls\n      CanonicalStore.LookupNodeIDByUniqueViolation INSIDE THE SAME\n      TXN (no cross-connection race against the just-inserted row),\n      appends the colliding row's id to res.ExistingNodes, returns\n      nil. The batch keeps APPLIED.\n    - Result.ExistingNodes is the index-aligned twin of\n      CreatedNodes; the success branch pads ExistingNodes with \"\"\n      in lockstep so a mixed batch (some SKIPs swallowed, others\n      created) preserves alignment.\n    - AppliedResultEnvelope persists (CreatedNodes, ExistingNodes)\n      into applied_events.failure_json so a retry replay sees the\n      same surface as the first call. Empty envelope on APPLIED is\n      the legacy \"no SKIP happened\" signal; the failure_json\n      column is reused (no schema migration).\n\n  handler (server/go/internal/api/execute_atomic.go):\n    - On wait_applied with APPLIED status, the handler decodes\n      AppliedResultEnvelope and PREFERS the applier-reported\n      (CreatedNodeIDs, ExistingNodeIDs) over the pre-minted UUIDs.\n      Pre-v2.2 batches leave failure_json empty -> legacy path.\n    - Threads NodeConflictPolicy_SKIP from proto into the op map.\n    - Sets ExecuteAtomicResponse.ExistingNodeIds on the response.\n\n  store/unique_violation.go:\n    - LookupNodeIDByUniqueViolation: parses the violated index name\n      to recover the field_id tuple, projects the matching row out\n      via json_extract on payload_json. Works for single-field AND\n      composite (composite is degenerate as N>1 AND-ed clauses).\n\n  Go SDK (sdk/go/entdb/):\n    - NodeConflictPolicy + OnConflict(ConflictSkip) CreateOption.\n    - CommitResult.ExistingNodeIDs index-aligned with CreatedNodeIDs.\n    - Operation.OnConflict threaded to pb.CreateNodeOp.OnConflict.\n    - InsertIfNotExists rewritten: sends SKIP, reads\n      ExistingNodeIDs[0] on success. v2.1.x fallback unchanged\n      (catch UCE -> 2-RTT GetNodeByKey). Single binary works\n      against either server version.\n    - testpb.OAuthIdentity concrete wrapper added (composite-unique\n      fixture).\n\n  Python SDK (sdk/python/entdb_sdk/):\n    - Plan.create accepts on_conflict='skip' (or 'error').\n    - ScopedPlan.create forwards the kwarg.\n    - GrpcCommitResult / CommitResult carry existing_node_ids.\n    - _grpc_client maps on_conflict='skip' to the proto enum.\n    - insert_if_not_exists rewritten with the same v2.2 / v2.1.x\n      fallback semantics as Go.\n\nTests\n-----\n\n  Go integration (sdk/go/entdb/integration_test.go, +97 lines):\n    - InsertIfNotExists_SingleFieldSingleRTT: same sku twice ->\n      second returns first's id WITHOUT a GetNodeByKey round trip.\n    - InsertIfNotExists_CompositeSingleRTT: same (provider, uid)\n      tuple twice -> resolved server-side (the gap v2.1.x couldn't\n      close).\n\n  Python integration:\n    - test_composite_unique_insert_if_not_exists_skip: same composite\n      tuple + on_conflict=SKIP -> existing_node_ids=[first_id],\n      created_node_ids=[''] (index alignment proven on the wire).\n    - existing test_insert_if_not_exists.py cases now exercise the\n      single-RTT path automatically against a v2.2 server.\n\n  Local CI: all four Go modules + Python unit (454) + Python\n  integration (118 including 6 new) + ruff clean.\n\nBackwards compat\n----------------\n\n  - Pre-v2.2 SDKs against a v2.2 server: ignore on_conflict /\n    existing_node_ids, see legacy behaviour.\n  - v2.2 SDKs against a v2.1.x server: on_conflict is silently\n    dropped (unknown proto field), the legacy UCE surfaces, the\n    fallback path in InsertIfNotExists handles it.\n  - Wire shape for batches that never SKIP is byte-identical to\n    pre-v2.2 (existing_node_ids is the proto3-default empty).",
          "timestamp": "2026-05-28T15:42:32Z",
          "tree_id": "77b84832bef59c05f1a3c790183fd197f487dc73",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/8fdcef68fc0794aae93ce0c82f8c462f1e72b585"
        },
        "date": 1779983068209,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3014.025763810091,
            "unit": "iter/sec",
            "range": "stddev: 0.000029720467712729098",
            "extra": "mean: 331.7821672286835 usec\nrounds: 1483"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2004.7100198778412,
            "unit": "iter/sec",
            "range": "stddev: 0.00004311627141062021",
            "extra": "mean: 498.82526155126203 usec\nrounds: 1212"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 903.1727299067305,
            "unit": "iter/sec",
            "range": "stddev: 0.00010426095010021006",
            "extra": "mean: 1.1072079203534728 msec\nrounds: 791"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 439.0730220967125,
            "unit": "iter/sec",
            "range": "stddev: 0.00016071256285040683",
            "extra": "mean: 2.2775254904632574 msec\nrounds: 367"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1880.6416705629563,
            "unit": "iter/sec",
            "range": "stddev: 0.00010127236379032496",
            "extra": "mean: 531.7334054927419 usec\nrounds: 1238"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1836.6479809860532,
            "unit": "iter/sec",
            "range": "stddev: 0.00012095149404531707",
            "extra": "mean: 544.4701490718561 usec\nrounds: 1563"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1913.4080912475079,
            "unit": "iter/sec",
            "range": "stddev: 0.0001319197548843572",
            "extra": "mean: 522.6276634735133 usec\nrounds: 1566"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1753.1102662532421,
            "unit": "iter/sec",
            "range": "stddev: 0.0000543110941018455",
            "extra": "mean: 570.4147760980295 usec\nrounds: 1389"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1739.8582393598138,
            "unit": "iter/sec",
            "range": "stddev: 0.000050567044891859157",
            "extra": "mean: 574.7594702703785 usec\nrounds: 370"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1471.5079901926633,
            "unit": "iter/sec",
            "range": "stddev: 0.00008696395951394078",
            "extra": "mean: 679.5749711621144 usec\nrounds: 1179"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2487.325555613394,
            "unit": "iter/sec",
            "range": "stddev: 0.000046428486838367325",
            "extra": "mean: 402.03824454872864 usec\nrounds: 1926"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 150.0478351962684,
            "unit": "iter/sec",
            "range": "stddev: 0.0004135838709318488",
            "extra": "mean: 6.664541335714448 msec\nrounds: 140"
          }
        ]
      }
    ]
  }
}