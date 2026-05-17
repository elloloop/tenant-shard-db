window.BENCHMARK_DATA = {
  "lastUpdate": 1779018773165,
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
      }
    ]
  }
}