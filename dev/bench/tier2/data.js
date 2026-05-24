window.BENCHMARK_DATA = {
  "lastUpdate": 1779584991683,
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
        "date": 1778976785932,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2332.5457288884522,
            "unit": "iter/sec",
            "range": "stddev: 0.00003223363202096383",
            "extra": "mean: 428.7161394587271 usec\nrounds: 1219"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1663.7058834677086,
            "unit": "iter/sec",
            "range": "stddev: 0.000053223188433165015",
            "extra": "mean: 601.0677788285944 usec\nrounds: 1058"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 824.4005595738923,
            "unit": "iter/sec",
            "range": "stddev: 0.0005251918185447465",
            "extra": "mean: 1.2130025730657799 msec\nrounds: 698"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 748.4048135570138,
            "unit": "iter/sec",
            "range": "stddev: 0.00012852822998794962",
            "extra": "mean: 1.3361752648906762 msec\nrounds: 638"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1186.8472106510303,
            "unit": "iter/sec",
            "range": "stddev: 0.0018240905328235826",
            "extra": "mean: 842.5684376436815 usec\nrounds: 1307"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1101.7208370363228,
            "unit": "iter/sec",
            "range": "stddev: 0.002408550506953024",
            "extra": "mean: 907.6709511005018 usec\nrounds: 1227"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1221.7320251136225,
            "unit": "iter/sec",
            "range": "stddev: 0.001621203101784329",
            "extra": "mean: 818.5100983229107 usec\nrounds: 1312"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1605.9848473268767,
            "unit": "iter/sec",
            "range": "stddev: 0.00003856268083331603",
            "extra": "mean: 622.6708811508876 usec\nrounds: 1321"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1472.8515314187578,
            "unit": "iter/sec",
            "range": "stddev: 0.00006280466293332633",
            "extra": "mean: 678.9550600777305 usec\nrounds: 516"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1308.9741096403554,
            "unit": "iter/sec",
            "range": "stddev: 0.000043033250511987144",
            "extra": "mean: 763.9570505139731 usec\nrounds: 1069"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1913.7718723023613,
            "unit": "iter/sec",
            "range": "stddev: 0.00002910580698385685",
            "extra": "mean: 522.5283193220679 usec\nrounds: 1475"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 182.8828916410737,
            "unit": "iter/sec",
            "range": "stddev: 0.000935151374642914",
            "extra": "mean: 5.467980033707045 msec\nrounds: 178"
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
        "date": 1778977783320,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2512.3262022963686,
            "unit": "iter/sec",
            "range": "stddev: 0.0000276893543337289",
            "extra": "mean: 398.0374837813494 usec\nrounds: 1079"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1763.7797425581098,
            "unit": "iter/sec",
            "range": "stddev: 0.00005888943032124892",
            "extra": "mean: 566.9642166031703 usec\nrounds: 1048"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 961.3991305639422,
            "unit": "iter/sec",
            "range": "stddev: 0.0003060729665692973",
            "extra": "mean: 1.0401507222223252 msec\nrounds: 792"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 737.8064681235234,
            "unit": "iter/sec",
            "range": "stddev: 0.00007882924030089977",
            "extra": "mean: 1.3553689798129829 msec\nrounds: 644"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1216.1362055289424,
            "unit": "iter/sec",
            "range": "stddev: 0.0019092160983738957",
            "extra": "mean: 822.2763169566712 usec\nrounds: 1262"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1143.303759252838,
            "unit": "iter/sec",
            "range": "stddev: 0.0023822986359390867",
            "extra": "mean: 874.6581928966203 usec\nrounds: 1633"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1240.575786316372,
            "unit": "iter/sec",
            "range": "stddev: 0.0019267477977788507",
            "extra": "mean: 806.0773158964266 usec\nrounds: 1472"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1676.3100532719268,
            "unit": "iter/sec",
            "range": "stddev: 0.00004485731043698995",
            "extra": "mean: 596.5483521667949 usec\nrounds: 1292"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1467.3434201112552,
            "unit": "iter/sec",
            "range": "stddev: 0.000026989558988029454",
            "extra": "mean: 681.5037204611441 usec\nrounds: 347"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1281.5385238418282,
            "unit": "iter/sec",
            "range": "stddev: 0.00006463596371799978",
            "extra": "mean: 780.312086914231 usec\nrounds: 1024"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2085.005974066773,
            "unit": "iter/sec",
            "range": "stddev: 0.00002675100074484585",
            "extra": "mean: 479.61493273302943 usec\nrounds: 1665"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 170.70669688749686,
            "unit": "iter/sec",
            "range": "stddev: 0.00045596845144396496",
            "extra": "mean: 5.858000993710536 msec\nrounds: 159"
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
        "date": 1778978592742,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2519.589223794166,
            "unit": "iter/sec",
            "range": "stddev: 0.00003220751717964998",
            "extra": "mean: 396.89009246282336 usec\nrounds: 995"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1760.0974503717873,
            "unit": "iter/sec",
            "range": "stddev: 0.00005155509944038064",
            "extra": "mean: 568.1503599637446 usec\nrounds: 1089"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 945.7730853081172,
            "unit": "iter/sec",
            "range": "stddev: 0.00038349914991607736",
            "extra": "mean: 1.0573360730330115 msec\nrounds: 712"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 718.3247076882803,
            "unit": "iter/sec",
            "range": "stddev: 0.00007853807438859125",
            "extra": "mean: 1.392128085386635 msec\nrounds: 609"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1198.0594061127279,
            "unit": "iter/sec",
            "range": "stddev: 0.0020091334558843656",
            "extra": "mean: 834.683150850291 usec\nrounds: 1412"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1095.4684244208452,
            "unit": "iter/sec",
            "range": "stddev: 0.002627836960723531",
            "extra": "mean: 912.8515050798313 usec\nrounds: 1378"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1245.762321958262,
            "unit": "iter/sec",
            "range": "stddev: 0.0018063556636978997",
            "extra": "mean: 802.7213396758231 usec\nrounds: 1419"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1674.2124363964826,
            "unit": "iter/sec",
            "range": "stddev: 0.0003251288365385096",
            "extra": "mean: 597.2957662125397 usec\nrounds: 1249"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1545.8816837818622,
            "unit": "iter/sec",
            "range": "stddev: 0.0000926365570338141",
            "extra": "mean: 646.8800364809218 usec\nrounds: 466"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1385.032473568812,
            "unit": "iter/sec",
            "range": "stddev: 0.000040346057737001",
            "extra": "mean: 722.0047320791698 usec\nrounds: 1116"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2092.581288352703,
            "unit": "iter/sec",
            "range": "stddev: 0.00002414981284554958",
            "extra": "mean: 477.87868770785394 usec\nrounds: 1505"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 168.28250829958498,
            "unit": "iter/sec",
            "range": "stddev: 0.00043377823242799884",
            "extra": "mean: 5.942388250000111 msec\nrounds: 140"
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
        "date": 1779015030770,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2472.818502768405,
            "unit": "iter/sec",
            "range": "stddev: 0.000027770691831852288",
            "extra": "mean: 404.39684468571625 usec\nrounds: 1101"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1697.96784303318,
            "unit": "iter/sec",
            "range": "stddev: 0.00008364991095688993",
            "extra": "mean: 588.9393041823696 usec\nrounds: 1052"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 860.4021360739447,
            "unit": "iter/sec",
            "range": "stddev: 0.0005194412268465001",
            "extra": "mean: 1.162247230769378 msec\nrounds: 819"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 770.7316427595782,
            "unit": "iter/sec",
            "range": "stddev: 0.00007768251982664789",
            "extra": "mean: 1.2974684631080338 msec\nrounds: 637"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1233.6319700036706,
            "unit": "iter/sec",
            "range": "stddev: 0.0017094336793709126",
            "extra": "mean: 810.6145303586973 usec\nrounds: 1367"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1140.275597351977,
            "unit": "iter/sec",
            "range": "stddev: 0.0019295489967619537",
            "extra": "mean: 876.9809704971901 usec\nrounds: 1288"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1276.7213840093373,
            "unit": "iter/sec",
            "range": "stddev: 0.0016030457759104448",
            "extra": "mean: 783.2562472320011 usec\nrounds: 542"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1646.911416866019,
            "unit": "iter/sec",
            "range": "stddev: 0.00032641806153949077",
            "extra": "mean: 607.1971994115777 usec\nrounds: 1359"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1485.4336118877407,
            "unit": "iter/sec",
            "range": "stddev: 0.00003434797267403862",
            "extra": "mean: 673.204101480621 usec\nrounds: 473"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1280.0818995728787,
            "unit": "iter/sec",
            "range": "stddev: 0.00006890104886302014",
            "extra": "mean: 781.2000156659252 usec\nrounds: 1149"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1979.0837660086486,
            "unit": "iter/sec",
            "range": "stddev: 0.000029725741741433236",
            "extra": "mean: 505.28432256142816 usec\nrounds: 1671"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 166.38977259307154,
            "unit": "iter/sec",
            "range": "stddev: 0.0007929586397265875",
            "extra": "mean: 6.009984774999566 msec\nrounds: 160"
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
        "date": 1779015209055,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2296.2636992436765,
            "unit": "iter/sec",
            "range": "stddev: 0.0000324517633860186",
            "extra": "mean: 435.49005296272 usec\nrounds: 1114"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1650.5166144484115,
            "unit": "iter/sec",
            "range": "stddev: 0.00006266021716727039",
            "extra": "mean: 605.870908081826 usec\nrounds: 990"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 797.4996351099957,
            "unit": "iter/sec",
            "range": "stddev: 0.0005696646790857224",
            "extra": "mean: 1.2539190690188522 msec\nrounds: 652"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 712.9118254916589,
            "unit": "iter/sec",
            "range": "stddev: 0.00008367043180042998",
            "extra": "mean: 1.4026980115112426 msec\nrounds: 608"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1144.7883556135635,
            "unit": "iter/sec",
            "range": "stddev: 0.0020067570821242827",
            "extra": "mean: 873.5239095474882 usec\nrounds: 1393"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1075.4068012215198,
            "unit": "iter/sec",
            "range": "stddev: 0.002299991997575292",
            "extra": "mean: 929.8806729361693 usec\nrounds: 1223"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1139.1331282200338,
            "unit": "iter/sec",
            "range": "stddev: 0.0019650210756874216",
            "extra": "mean: 877.8605197467675 usec\nrounds: 1266"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1511.4510851029,
            "unit": "iter/sec",
            "range": "stddev: 0.0006772550116052778",
            "extra": "mean: 661.6158537025495 usec\nrounds: 540"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1482.083930257608,
            "unit": "iter/sec",
            "range": "stddev: 0.00003864710901752968",
            "extra": "mean: 674.7256208534595 usec\nrounds: 422"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1277.9280285766386,
            "unit": "iter/sec",
            "range": "stddev: 0.00006414962004374817",
            "extra": "mean: 782.5166814079538 usec\nrounds: 1108"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1893.4216273323698,
            "unit": "iter/sec",
            "range": "stddev: 0.000032007802431160764",
            "extra": "mean: 528.144384517723 usec\nrounds: 1576"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 151.70454851452877,
            "unit": "iter/sec",
            "range": "stddev: 0.0012243847178538636",
            "extra": "mean: 6.591760166665206 msec\nrounds: 138"
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
        "date": 1779015226108,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3310.894787033285,
            "unit": "iter/sec",
            "range": "stddev: 0.000029328496740893056",
            "extra": "mean: 302.03315548303675 usec\nrounds: 1222"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2320.1655846545373,
            "unit": "iter/sec",
            "range": "stddev: 0.00003754540566579567",
            "extra": "mean: 431.00372086111076 usec\nrounds: 1347"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1279.951259868004,
            "unit": "iter/sec",
            "range": "stddev: 0.00011185209871105872",
            "extra": "mean: 781.279749748538 usec\nrounds: 995"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 980.6270873913192,
            "unit": "iter/sec",
            "range": "stddev: 0.0000720745637497721",
            "extra": "mean: 1.0197556368346066 msec\nrounds: 771"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2207.6603388139197,
            "unit": "iter/sec",
            "range": "stddev: 0.00023677494257182913",
            "extra": "mean: 452.96823176035167 usec\nrounds: 1864"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2263.3616952541347,
            "unit": "iter/sec",
            "range": "stddev: 0.00007830026289811634",
            "extra": "mean: 441.8206785494433 usec\nrounds: 1627"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2265.6562157232274,
            "unit": "iter/sec",
            "range": "stddev: 0.00033071541428686055",
            "extra": "mean: 441.37322911578036 usec\nrounds: 2047"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1408.892024764832,
            "unit": "iter/sec",
            "range": "stddev: 0.00010150951619038528",
            "extra": "mean: 709.7776000023259 usec\nrounds: 5"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 223.74058565676495,
            "unit": "iter/sec",
            "range": "stddev: 0.029106671932409164",
            "extra": "mean: 4.469461796860029 msec\nrounds: 1083"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 469.2007079520727,
            "unit": "iter/sec",
            "range": "stddev: 0.013021875302553446",
            "extra": "mean: 2.1312840817412977 msec\nrounds: 942"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2741.1987025506555,
            "unit": "iter/sec",
            "range": "stddev: 0.00002070078238789209",
            "extra": "mean: 364.8039082571836 usec\nrounds: 2071"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 183.804716126791,
            "unit": "iter/sec",
            "range": "stddev: 0.00047113723410750346",
            "extra": "mean: 5.440556809816493 msec\nrounds: 163"
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
        "date": 1779015328740,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2524.3649779326224,
            "unit": "iter/sec",
            "range": "stddev: 0.000029502640596815268",
            "extra": "mean: 396.1392305557057 usec\nrounds: 1080"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1774.1528047349925,
            "unit": "iter/sec",
            "range": "stddev: 0.000055881812160110346",
            "extra": "mean: 563.6493076194591 usec\nrounds: 1063"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 962.2587247134261,
            "unit": "iter/sec",
            "range": "stddev: 0.0003077207186331785",
            "extra": "mean: 1.0392215464690266 msec\nrounds: 807"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 737.2927304854484,
            "unit": "iter/sec",
            "range": "stddev: 0.00006978240238865194",
            "extra": "mean: 1.3563133863283583 msec\nrounds: 629"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1210.609114774615,
            "unit": "iter/sec",
            "range": "stddev: 0.0020000014703637618",
            "extra": "mean: 826.0304567310109 usec\nrounds: 1248"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1155.3094061084657,
            "unit": "iter/sec",
            "range": "stddev: 0.0023722669671859273",
            "extra": "mean: 865.5689936502737 usec\nrounds: 945"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1218.3455315018439,
            "unit": "iter/sec",
            "range": "stddev: 0.0020323034062979515",
            "extra": "mean: 820.7852158059862 usec\nrounds: 1316"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1710.2720910168648,
            "unit": "iter/sec",
            "range": "stddev: 0.000032957400375474083",
            "extra": "mean: 584.7022852401437 usec\nrounds: 1206"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1485.1217046672966,
            "unit": "iter/sec",
            "range": "stddev: 0.00003295529617803262",
            "extra": "mean: 673.3454886944934 usec\nrounds: 575"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1296.2643046888525,
            "unit": "iter/sec",
            "range": "stddev: 0.000060147674633010675",
            "extra": "mean: 771.4476101693119 usec\nrounds: 1180"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2098.4038589648376,
            "unit": "iter/sec",
            "range": "stddev: 0.000033307194319068274",
            "extra": "mean: 476.552688238626 usec\nrounds: 1607"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 200.20093103080342,
            "unit": "iter/sec",
            "range": "stddev: 0.00032880109761315783",
            "extra": "mean: 4.994981765824743 msec\nrounds: 158"
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
        "date": 1779015349447,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2551.170640195124,
            "unit": "iter/sec",
            "range": "stddev: 0.00002800423508544716",
            "extra": "mean: 391.97691610448913 usec\nrounds: 1037"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1809.5515090165115,
            "unit": "iter/sec",
            "range": "stddev: 0.00003314054392693235",
            "extra": "mean: 552.6231196057516 usec\nrounds: 811"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 974.1278365745634,
            "unit": "iter/sec",
            "range": "stddev: 0.00025289009446545724",
            "extra": "mean: 1.0265593102404442 msec\nrounds: 664"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 730.2505890286969,
            "unit": "iter/sec",
            "range": "stddev: 0.0000670349869933001",
            "extra": "mean: 1.3693929385666028 msec\nrounds: 586"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1244.9821046878496,
            "unit": "iter/sec",
            "range": "stddev: 0.0019108741907302453",
            "extra": "mean: 803.2243967480375 usec\nrounds: 1230"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1144.2076499849939,
            "unit": "iter/sec",
            "range": "stddev: 0.0024774730569159117",
            "extra": "mean: 873.9672383881675 usec\nrounds: 1464"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1216.236778604918,
            "unit": "iter/sec",
            "range": "stddev: 0.0020404394959830673",
            "extra": "mean: 822.2083212670544 usec\nrounds: 1326"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1719.0389795592343,
            "unit": "iter/sec",
            "range": "stddev: 0.000028721625256850908",
            "extra": "mean: 581.7203750995817 usec\nrounds: 1261"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1493.7670290915075,
            "unit": "iter/sec",
            "range": "stddev: 0.00003350470826217732",
            "extra": "mean: 669.4484350803946 usec\nrounds: 439"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1294.4169684609362,
            "unit": "iter/sec",
            "range": "stddev: 0.00006660891071381897",
            "extra": "mean: 772.5485870206118 usec\nrounds: 1017"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2111.7959038867907,
            "unit": "iter/sec",
            "range": "stddev: 0.00002627874566244997",
            "extra": "mean: 473.5306087863347 usec\nrounds: 1434"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 182.091766317346,
            "unit": "iter/sec",
            "range": "stddev: 0.00009738178593376766",
            "extra": "mean: 5.491736503105908 msec\nrounds: 161"
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
        "date": 1779015379980,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2519.230528836571,
            "unit": "iter/sec",
            "range": "stddev: 0.00003009514399644825",
            "extra": "mean: 396.94660276359036 usec\nrounds: 1158"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1779.5901449273479,
            "unit": "iter/sec",
            "range": "stddev: 0.000056788929902206065",
            "extra": "mean: 561.9271397127372 usec\nrounds: 1045"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 972.485792174705,
            "unit": "iter/sec",
            "range": "stddev: 0.0003086690570171114",
            "extra": "mean: 1.0282926578945353 msec\nrounds: 836"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 740.1713159813934,
            "unit": "iter/sec",
            "range": "stddev: 0.00008567288424776318",
            "extra": "mean: 1.3510385750008425 msec\nrounds: 600"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1218.9081157615967,
            "unit": "iter/sec",
            "range": "stddev: 0.00198884909447166",
            "extra": "mean: 820.4063842623455 usec\nrounds: 1309"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1140.452053185001,
            "unit": "iter/sec",
            "range": "stddev: 0.002468394647762595",
            "extra": "mean: 876.8452800863017 usec\nrounds: 1396"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1586.9298251766097,
            "unit": "iter/sec",
            "range": "stddev: 0.00009614115461832745",
            "extra": "mean: 630.1475869537645 usec\nrounds: 46"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1711.0290707827423,
            "unit": "iter/sec",
            "range": "stddev: 0.00003171167681203732",
            "extra": "mean: 584.4436059420844 usec\nrounds: 1279"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1519.3777360582822,
            "unit": "iter/sec",
            "range": "stddev: 0.00002923633303740138",
            "extra": "mean: 658.1641788396198 usec\nrounds: 397"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1319.7706990427146,
            "unit": "iter/sec",
            "range": "stddev: 0.00004398138138388954",
            "extra": "mean: 757.7073810816852 usec\nrounds: 1110"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2059.5370693123837,
            "unit": "iter/sec",
            "range": "stddev: 0.000039229489185149236",
            "extra": "mean: 485.54600686739246 usec\nrounds: 1602"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 164.89352894262984,
            "unit": "iter/sec",
            "range": "stddev: 0.00013645400126839806",
            "extra": "mean: 6.064519368421804 msec\nrounds: 133"
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
        "date": 1779015418503,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2462.0572271990286,
            "unit": "iter/sec",
            "range": "stddev: 0.00009449296273618136",
            "extra": "mean: 406.1644014414949 usec\nrounds: 1248"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1745.1916602412546,
            "unit": "iter/sec",
            "range": "stddev: 0.00004757856113504591",
            "extra": "mean: 573.0029674000164 usec\nrounds: 1135"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 869.1806705221311,
            "unit": "iter/sec",
            "range": "stddev: 0.00041754357771314824",
            "extra": "mean: 1.150508788235343 msec\nrounds: 850"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 730.8697139138862,
            "unit": "iter/sec",
            "range": "stddev: 0.00010679788780003569",
            "extra": "mean: 1.368232916158055 msec\nrounds: 656"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1251.2132847288196,
            "unit": "iter/sec",
            "range": "stddev: 0.00169895687704267",
            "extra": "mean: 799.2242507373425 usec\nrounds: 1356"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1155.4357049491289,
            "unit": "iter/sec",
            "range": "stddev: 0.0022259245057991007",
            "extra": "mean: 865.4743796791598 usec\nrounds: 1496"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1240.2637051448246,
            "unit": "iter/sec",
            "range": "stddev: 0.0017427501478952127",
            "extra": "mean: 806.2801449819342 usec\nrounds: 1345"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1663.1049026311725,
            "unit": "iter/sec",
            "range": "stddev: 0.00006094540517145936",
            "extra": "mean: 601.2849811325284 usec\nrounds: 1325"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1571.4557264380164,
            "unit": "iter/sec",
            "range": "stddev: 0.000027075461977702977",
            "extra": "mean: 636.352639896943 usec\nrounds: 386"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1362.9889684609063,
            "unit": "iter/sec",
            "range": "stddev: 0.000043330770555094504",
            "extra": "mean: 733.681653439356 usec\nrounds: 1134"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1977.4002932123624,
            "unit": "iter/sec",
            "range": "stddev: 0.000036076805364271554",
            "extra": "mean: 505.714499705804 usec\nrounds: 1697"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 160.98761145761588,
            "unit": "iter/sec",
            "range": "stddev: 0.000808926679663388",
            "extra": "mean: 6.211658095587533 msec\nrounds: 136"
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
        "date": 1779015457818,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2521.108992766195,
            "unit": "iter/sec",
            "range": "stddev: 0.000030988661813079554",
            "extra": "mean: 396.65084011413023 usec\nrounds: 1057"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1767.784095312523,
            "unit": "iter/sec",
            "range": "stddev: 0.00005190446095181383",
            "extra": "mean: 565.6799394516626 usec\nrounds: 1057"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 960.9082208410987,
            "unit": "iter/sec",
            "range": "stddev: 0.00032991571802961235",
            "extra": "mean: 1.0406821154310488 msec\nrounds: 823"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 732.1585171040077,
            "unit": "iter/sec",
            "range": "stddev: 0.00008649684090184756",
            "extra": "mean: 1.3658244446235728 msec\nrounds: 623"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1212.4766757336354,
            "unit": "iter/sec",
            "range": "stddev: 0.0019467493197114747",
            "extra": "mean: 824.7581335079525 usec\nrounds: 1146"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1141.5032519049337,
            "unit": "iter/sec",
            "range": "stddev: 0.002481359247767411",
            "extra": "mean: 876.0378021974149 usec\nrounds: 1183"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1210.8844512500111,
            "unit": "iter/sec",
            "range": "stddev: 0.0020194003060724362",
            "extra": "mean: 825.8426301268362 usec\nrounds: 1414"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1701.768006303704,
            "unit": "iter/sec",
            "range": "stddev: 0.00003304906072687438",
            "extra": "mean: 587.624162809379 usec\nrounds: 1253"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1597.6271660777686,
            "unit": "iter/sec",
            "range": "stddev: 0.000031245255150651526",
            "extra": "mean: 625.9282648873801 usec\nrounds: 487"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1376.5007953341765,
            "unit": "iter/sec",
            "range": "stddev: 0.00005858154363008561",
            "extra": "mean: 726.4797836584087 usec\nrounds: 1077"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2087.9495924532666,
            "unit": "iter/sec",
            "range": "stddev: 0.000024784169177479482",
            "extra": "mean: 478.9387653870683 usec\nrounds: 1641"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 194.89196081029723,
            "unit": "iter/sec",
            "range": "stddev: 0.0003978000364416739",
            "extra": "mean: 5.131047970590096 msec\nrounds: 170"
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
        "date": 1779015502435,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3302.7649353463594,
            "unit": "iter/sec",
            "range": "stddev: 0.000026673386983589552",
            "extra": "mean: 302.77661885590123 usec\nrounds: 1241"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2313.4702756978404,
            "unit": "iter/sec",
            "range": "stddev: 0.00003313166787068839",
            "extra": "mean: 432.2510690993675 usec\nrounds: 1288"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1264.8482082451621,
            "unit": "iter/sec",
            "range": "stddev: 0.00008952958586433418",
            "extra": "mean: 790.6087018832006 usec\nrounds: 956"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 956.0810795045371,
            "unit": "iter/sec",
            "range": "stddev: 0.0000715691587772001",
            "extra": "mean: 1.0459363974844296 msec\nrounds: 795"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2154.500551711504,
            "unit": "iter/sec",
            "range": "stddev: 0.00041570981145671297",
            "extra": "mean: 464.1446943263089 usec\nrounds: 1410"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2240.4872502466587,
            "unit": "iter/sec",
            "range": "stddev: 0.00016250129031427258",
            "extra": "mean: 446.33148431882773 usec\nrounds: 2232"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2149.648518586939,
            "unit": "iter/sec",
            "range": "stddev: 0.0004927924632738778",
            "extra": "mean: 465.19232858464926 usec\nrounds: 2106"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 361.5508363231031,
            "unit": "iter/sec",
            "range": "stddev: 0.027780618435597083",
            "extra": "mean: 2.765862776504107 msec\nrounds: 698"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 258.2021833208969,
            "unit": "iter/sec",
            "range": "stddev: 0.03898972920060246",
            "extra": "mean: 3.872933943231562 msec\nrounds: 458"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 843.3301405208015,
            "unit": "iter/sec",
            "range": "stddev: 0.005280553685745981",
            "extra": "mean: 1.1857752402664592 msec\nrounds: 1053"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2692.944746158904,
            "unit": "iter/sec",
            "range": "stddev: 0.00002391965580462917",
            "extra": "mean: 371.3407047903063 usec\nrounds: 1670"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 184.40547454718134,
            "unit": "iter/sec",
            "range": "stddev: 0.0003204837484175513",
            "extra": "mean: 5.422832497004548 msec\nrounds: 167"
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
        "date": 1779018726887,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2379.410978172554,
            "unit": "iter/sec",
            "range": "stddev: 0.00002781862571129625",
            "extra": "mean: 420.2720795917418 usec\nrounds: 980"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1713.0003092314926,
            "unit": "iter/sec",
            "range": "stddev: 0.0000386136160497295",
            "extra": "mean: 583.7710563220111 usec\nrounds: 870"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 823.7308930660491,
            "unit": "iter/sec",
            "range": "stddev: 0.0005365301325542573",
            "extra": "mean: 1.2139887048279214 msec\nrounds: 725"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 733.9860522458076,
            "unit": "iter/sec",
            "range": "stddev: 0.00012850986695768225",
            "extra": "mean: 1.3624237094700349 msec\nrounds: 623"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1195.3258360548086,
            "unit": "iter/sec",
            "range": "stddev: 0.0017735456293127626",
            "extra": "mean: 836.5919733656185 usec\nrounds: 1239"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1117.2623196416441,
            "unit": "iter/sec",
            "range": "stddev: 0.002276660104316685",
            "extra": "mean: 895.0449526667512 usec\nrounds: 1331"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1228.7730551839725,
            "unit": "iter/sec",
            "range": "stddev: 0.0015828373791584103",
            "extra": "mean: 813.8199285711711 usec\nrounds: 1260"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1622.1046489249863,
            "unit": "iter/sec",
            "range": "stddev: 0.00011565721930628092",
            "extra": "mean: 616.4830368143805 usec\nrounds: 1331"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1444.0251520834431,
            "unit": "iter/sec",
            "range": "stddev: 0.00004241140243831156",
            "extra": "mean: 692.5087132708163 usec\nrounds: 422"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1256.606189348061,
            "unit": "iter/sec",
            "range": "stddev: 0.00007316762276751219",
            "extra": "mean: 795.7942659177967 usec\nrounds: 1068"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1941.4047111629918,
            "unit": "iter/sec",
            "range": "stddev: 0.000026133812987548886",
            "extra": "mean: 515.0909515414503 usec\nrounds: 1589"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 179.1162714264927,
            "unit": "iter/sec",
            "range": "stddev: 0.0009032262781547598",
            "extra": "mean: 5.582965701753058 msec\nrounds: 171"
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
        "date": 1779018749788,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2291.833100981483,
            "unit": "iter/sec",
            "range": "stddev: 0.000043653669998410576",
            "extra": "mean: 436.3319473707521 usec\nrounds: 1102"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1618.2233963492142,
            "unit": "iter/sec",
            "range": "stddev: 0.0000621532443601949",
            "extra": "mean: 617.9616499526861 usec\nrounds: 1057"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 816.4462567565675,
            "unit": "iter/sec",
            "range": "stddev: 0.0005403626452065891",
            "extra": "mean: 1.2248203623011538 msec\nrounds: 748"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 723.468065176822,
            "unit": "iter/sec",
            "range": "stddev: 0.00012909671018230612",
            "extra": "mean: 1.3822310176961177 msec\nrounds: 678"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1183.4388126591127,
            "unit": "iter/sec",
            "range": "stddev: 0.0017914860475232019",
            "extra": "mean: 844.9951018194704 usec\nrounds: 1375"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1096.2484407288164,
            "unit": "iter/sec",
            "range": "stddev: 0.0023631416472570397",
            "extra": "mean: 912.2019816376408 usec\nrounds: 1307"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1195.6853574298311,
            "unit": "iter/sec",
            "range": "stddev: 0.0018030900267698808",
            "extra": "mean: 836.3404250007177 usec\nrounds: 1400"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1592.0634499061944,
            "unit": "iter/sec",
            "range": "stddev: 0.00005039573041418725",
            "extra": "mean: 628.1156696731658 usec\nrounds: 1220"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1498.1633228802161,
            "unit": "iter/sec",
            "range": "stddev: 0.000028744862823734834",
            "extra": "mean: 667.4839683550002 usec\nrounds: 474"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1307.7405724577536,
            "unit": "iter/sec",
            "range": "stddev: 0.00004638134646482079",
            "extra": "mean: 764.6776593622164 usec\nrounds: 1095"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1900.640433168291,
            "unit": "iter/sec",
            "range": "stddev: 0.000036524609617516005",
            "extra": "mean: 526.1384439417824 usec\nrounds: 1766"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 164.59568473506968,
            "unit": "iter/sec",
            "range": "stddev: 0.0013951163944286183",
            "extra": "mean: 6.075493422622728 msec\nrounds: 168"
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
        "date": 1779018827793,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2354.7963107370824,
            "unit": "iter/sec",
            "range": "stddev: 0.00003105152921952267",
            "extra": "mean: 424.6651803556575 usec\nrounds: 1181"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1686.8291133665955,
            "unit": "iter/sec",
            "range": "stddev: 0.00004772112081518853",
            "extra": "mean: 592.8282788552225 usec\nrounds: 1083"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 825.3254600044141,
            "unit": "iter/sec",
            "range": "stddev: 0.0005011605761266948",
            "extra": "mean: 1.2116432225350853 msec\nrounds: 710"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 712.9568513212147,
            "unit": "iter/sec",
            "range": "stddev: 0.00014668665308465923",
            "extra": "mean: 1.402609426007832 msec\nrounds: 669"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1213.907644681596,
            "unit": "iter/sec",
            "range": "stddev: 0.0016922549997131907",
            "extra": "mean: 823.785898689432 usec\nrounds: 1145"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1091.2855125078913,
            "unit": "iter/sec",
            "range": "stddev: 0.0023611645974328708",
            "extra": "mean: 916.3504770643317 usec\nrounds: 1417"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1158.6824471347618,
            "unit": "iter/sec",
            "range": "stddev: 0.0020482606741157507",
            "extra": "mean: 863.0492353386743 usec\nrounds: 1313"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1615.0931490786534,
            "unit": "iter/sec",
            "range": "stddev: 0.00003635775443556728",
            "extra": "mean: 619.1593349092344 usec\nrounds: 1269"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1421.7313489131109,
            "unit": "iter/sec",
            "range": "stddev: 0.00006628388877738302",
            "extra": "mean: 703.367764074755 usec\nrounds: 373"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1262.5163164577689,
            "unit": "iter/sec",
            "range": "stddev: 0.00004698355106982815",
            "extra": "mean: 792.0689712792713 usec\nrounds: 1149"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1925.762919804691,
            "unit": "iter/sec",
            "range": "stddev: 0.000031164007051258505",
            "extra": "mean: 519.2747194973612 usec\nrounds: 1672"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 167.85586493155012,
            "unit": "iter/sec",
            "range": "stddev: 0.0011028639511512998",
            "extra": "mean: 5.957492163933561 msec\nrounds: 122"
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
        "date": 1779018836898,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2387.62358139562,
            "unit": "iter/sec",
            "range": "stddev: 0.000029249981195137012",
            "extra": "mean: 418.8264883091318 usec\nrounds: 1112"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1708.1444997137153,
            "unit": "iter/sec",
            "range": "stddev: 0.00005204035678361821",
            "extra": "mean: 585.4305652522955 usec\nrounds: 1249"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 820.1111858891883,
            "unit": "iter/sec",
            "range": "stddev: 0.00030147873933963025",
            "extra": "mean: 1.2193468607744584 msec\nrounds: 826"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 738.0662841086829,
            "unit": "iter/sec",
            "range": "stddev: 0.0001576316685969632",
            "extra": "mean: 1.3548918593505979 msec\nrounds: 647"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1222.8281014659692,
            "unit": "iter/sec",
            "range": "stddev: 0.0017409358229547652",
            "extra": "mean: 817.7764305556643 usec\nrounds: 1296"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1125.779302844077,
            "unit": "iter/sec",
            "range": "stddev: 0.0023588254269780113",
            "extra": "mean: 888.2735696718545 usec\nrounds: 1464"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1210.192423118427,
            "unit": "iter/sec",
            "range": "stddev: 0.0018319834887093119",
            "extra": "mean: 826.3148743099856 usec\nrounds: 1448"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1591.3614589384017,
            "unit": "iter/sec",
            "range": "stddev: 0.0005245246747329511",
            "extra": "mean: 628.3927478469288 usec\nrounds: 1277"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1529.5374290845932,
            "unit": "iter/sec",
            "range": "stddev: 0.000033029553748391215",
            "extra": "mean: 653.7924348791425 usec\nrounds: 453"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1332.4587404728773,
            "unit": "iter/sec",
            "range": "stddev: 0.00004282050932005918",
            "extra": "mean: 750.4922813933505 usec\nrounds: 1091"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1940.0327173159658,
            "unit": "iter/sec",
            "range": "stddev: 0.00004045116825666223",
            "extra": "mean: 515.4552245817274 usec\nrounds: 1554"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 174.10003337596692,
            "unit": "iter/sec",
            "range": "stddev: 0.0009106711636464809",
            "extra": "mean: 5.743824286584208 msec\nrounds: 164"
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
        "date": 1779023164864,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2329.3598226897407,
            "unit": "iter/sec",
            "range": "stddev: 0.000028109878545953463",
            "extra": "mean: 429.30250202619516 usec\nrounds: 1233"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1671.4144020681424,
            "unit": "iter/sec",
            "range": "stddev: 0.00005608685604172712",
            "extra": "mean: 598.2956702793989 usec\nrounds: 1107"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 799.892966128238,
            "unit": "iter/sec",
            "range": "stddev: 0.00043446624214404825",
            "extra": "mean: 1.25016726280311 msec\nrounds: 742"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 723.270146097242,
            "unit": "iter/sec",
            "range": "stddev: 0.00021074738112935497",
            "extra": "mean: 1.3826092579598224 msec\nrounds: 628"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1176.0252182417898,
            "unit": "iter/sec",
            "range": "stddev: 0.0018211683151979682",
            "extra": "mean: 850.3219016808538 usec\nrounds: 1251"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1082.1254502006539,
            "unit": "iter/sec",
            "range": "stddev: 0.0024524193025292746",
            "extra": "mean: 924.1072740822927 usec\nrounds: 1609"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1235.7560457993131,
            "unit": "iter/sec",
            "range": "stddev: 0.0015710181046698158",
            "extra": "mean: 809.2212078583673 usec\nrounds: 1400"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1587.565863764221,
            "unit": "iter/sec",
            "range": "stddev: 0.00004873955170377759",
            "extra": "mean: 629.8951261328683 usec\nrounds: 1324"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1419.4578853892876,
            "unit": "iter/sec",
            "range": "stddev: 0.000044516522501388996",
            "extra": "mean: 704.4943075051143 usec\nrounds: 413"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1244.3799920280483,
            "unit": "iter/sec",
            "range": "stddev: 0.00004529162209710187",
            "extra": "mean: 803.6130493951723 usec\nrounds: 1073"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1874.151038253024,
            "unit": "iter/sec",
            "range": "stddev: 0.00006937964776858762",
            "extra": "mean: 533.5749251736629 usec\nrounds: 1577"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 165.11205653677274,
            "unit": "iter/sec",
            "range": "stddev: 0.00117550392153346",
            "extra": "mean: 6.056492911390067 msec\nrounds: 158"
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
        "date": 1779023728595,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2374.172386398091,
            "unit": "iter/sec",
            "range": "stddev: 0.000030259522277296155",
            "extra": "mean: 421.19940646648746 usec\nrounds: 1299"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1695.7790744060094,
            "unit": "iter/sec",
            "range": "stddev: 0.00005732821027204027",
            "extra": "mean: 589.6994573719905 usec\nrounds: 1126"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 834.0296827265765,
            "unit": "iter/sec",
            "range": "stddev: 0.0004953801001483773",
            "extra": "mean: 1.1989980940856206 msec\nrounds: 744"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 727.0968776695795,
            "unit": "iter/sec",
            "range": "stddev: 0.00013727073566828664",
            "extra": "mean: 1.3753325460633294 msec\nrounds: 597"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1211.3583983175367,
            "unit": "iter/sec",
            "range": "stddev: 0.0017009460321812176",
            "extra": "mean: 825.5195170883418 usec\nrounds: 1346"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1095.3320711780473,
            "unit": "iter/sec",
            "range": "stddev: 0.002369674102394636",
            "extra": "mean: 912.9651420910957 usec\nrounds: 1492"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1192.9695292328115,
            "unit": "iter/sec",
            "range": "stddev: 0.001914327090110978",
            "extra": "mean: 838.2443771578068 usec\nrounds: 1506"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1606.6554134981918,
            "unit": "iter/sec",
            "range": "stddev: 0.00004677767530178386",
            "extra": "mean: 622.4109983998915 usec\nrounds: 1250"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1514.6015077799987,
            "unit": "iter/sec",
            "range": "stddev: 0.000026756048318966456",
            "extra": "mean: 660.2396702124859 usec\nrounds: 376"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1325.5597430008593,
            "unit": "iter/sec",
            "range": "stddev: 0.00004729105154379144",
            "extra": "mean: 754.3982874254742 usec\nrounds: 1169"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1926.2763001686103,
            "unit": "iter/sec",
            "range": "stddev: 0.00003594677250817075",
            "extra": "mean: 519.1363253093382 usec\nrounds: 1537"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 158.89245969970898,
            "unit": "iter/sec",
            "range": "stddev: 0.0013609884344201078",
            "extra": "mean: 6.293564854429851 msec\nrounds: 158"
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
        "date": 1779026406101,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2343.2820363367864,
            "unit": "iter/sec",
            "range": "stddev: 0.00003236713882119313",
            "extra": "mean: 426.75187386460885 usec\nrounds: 1213"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1683.7590302257324,
            "unit": "iter/sec",
            "range": "stddev: 0.00005494989461857995",
            "extra": "mean: 593.9092126893808 usec\nrounds: 1119"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 815.9203158133013,
            "unit": "iter/sec",
            "range": "stddev: 0.0005595937317097187",
            "extra": "mean: 1.225609879566744 msec\nrounds: 739"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 730.4287088870105,
            "unit": "iter/sec",
            "range": "stddev: 0.00018098106803675245",
            "extra": "mean: 1.369059003066498 msec\nrounds: 652"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1172.4350469255983,
            "unit": "iter/sec",
            "range": "stddev: 0.001919756109736407",
            "extra": "mean: 852.9257144114179 usec\nrounds: 1138"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1112.7995978264125,
            "unit": "iter/sec",
            "range": "stddev: 0.0023587330384437413",
            "extra": "mean: 898.6344009768341 usec\nrounds: 1434"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1234.9759704337594,
            "unit": "iter/sec",
            "range": "stddev: 0.00175492342707717",
            "extra": "mean: 809.7323542649749 usec\nrounds: 1290"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1616.8188473138655,
            "unit": "iter/sec",
            "range": "stddev: 0.00004294815274744641",
            "extra": "mean: 618.4984803098814 usec\nrounds: 1295"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1442.75308046812,
            "unit": "iter/sec",
            "range": "stddev: 0.00009296817206139105",
            "extra": "mean: 693.1192963909923 usec\nrounds: 388"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1320.0732431841318,
            "unit": "iter/sec",
            "range": "stddev: 0.00004845568397378012",
            "extra": "mean: 757.5337241045147 usec\nrounds: 1033"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1936.0559239037461,
            "unit": "iter/sec",
            "range": "stddev: 0.000028397470629767795",
            "extra": "mean: 516.5140054341305 usec\nrounds: 1656"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 171.50728906991046,
            "unit": "iter/sec",
            "range": "stddev: 0.0012791506414984088",
            "extra": "mean: 5.830655976332157 msec\nrounds: 169"
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
        "date": 1779026920844,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2353.6917225211128,
            "unit": "iter/sec",
            "range": "stddev: 0.000027957882582134902",
            "extra": "mean: 424.8644758493983 usec\nrounds: 1118"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1671.608829341507,
            "unit": "iter/sec",
            "range": "stddev: 0.00007274644302337773",
            "extra": "mean: 598.2260816329426 usec\nrounds: 1078"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 826.7313562463875,
            "unit": "iter/sec",
            "range": "stddev: 0.0005366502392962183",
            "extra": "mean: 1.2095827652410633 msec\nrounds: 771"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 739.8390138272251,
            "unit": "iter/sec",
            "range": "stddev: 0.0001375372295657596",
            "extra": "mean: 1.3516454002972198 msec\nrounds: 677"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1197.572011613373,
            "unit": "iter/sec",
            "range": "stddev: 0.0017808065381038636",
            "extra": "mean: 835.0228548284097 usec\nrounds: 1481"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1104.8473685586885,
            "unit": "iter/sec",
            "range": "stddev: 0.0023680518138003174",
            "extra": "mean: 905.1023955503777 usec\nrounds: 1393"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1198.9910920102138,
            "unit": "iter/sec",
            "range": "stddev: 0.0018521585050859113",
            "extra": "mean: 834.0345534372673 usec\nrounds: 1469"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1567.7919959618707,
            "unit": "iter/sec",
            "range": "stddev: 0.00048790942643183653",
            "extra": "mean: 637.839715074244 usec\nrounds: 1267"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1453.3736590212877,
            "unit": "iter/sec",
            "range": "stddev: 0.00002780479326228729",
            "extra": "mean: 688.0543030299635 usec\nrounds: 429"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1267.2589349975083,
            "unit": "iter/sec",
            "range": "stddev: 0.000043353274242434375",
            "extra": "mean: 789.10471442205 usec\nrounds: 1040"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1923.4690824223385,
            "unit": "iter/sec",
            "range": "stddev: 0.000034078100843867675",
            "extra": "mean: 519.8939817325479 usec\nrounds: 1697"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 159.7319291638046,
            "unit": "iter/sec",
            "range": "stddev: 0.0010012970554464007",
            "extra": "mean: 6.260489090909952 msec\nrounds: 154"
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
        "date": 1779027269057,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2496.9530655845097,
            "unit": "iter/sec",
            "range": "stddev: 0.00003122136712143482",
            "extra": "mean: 400.48810439531064 usec\nrounds: 910"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1764.233187044784,
            "unit": "iter/sec",
            "range": "stddev: 0.000043693978606034664",
            "extra": "mean: 566.8184950511395 usec\nrounds: 1111"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 956.1045452368636,
            "unit": "iter/sec",
            "range": "stddev: 0.0002952255497554947",
            "extra": "mean: 1.0459107270034596 msec\nrounds: 674"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 765.0467215776453,
            "unit": "iter/sec",
            "range": "stddev: 0.00010317359056086857",
            "extra": "mean: 1.307109712120385 msec\nrounds: 660"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1196.1292112807328,
            "unit": "iter/sec",
            "range": "stddev: 0.0019346435204652",
            "extra": "mean: 836.0300798349945 usec\nrounds: 1215"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1110.4697412785883,
            "unit": "iter/sec",
            "range": "stddev: 0.002538927367195965",
            "extra": "mean: 900.519809615529 usec\nrounds: 1560"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1202.8638255402795,
            "unit": "iter/sec",
            "range": "stddev: 0.0019507376784412022",
            "extra": "mean: 831.3493005335321 usec\nrounds: 1311"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1652.486049861166,
            "unit": "iter/sec",
            "range": "stddev: 0.000142000448840046",
            "extra": "mean: 605.1488302028421 usec\nrounds: 1331"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1557.847360491759,
            "unit": "iter/sec",
            "range": "stddev: 0.00007668157010463733",
            "extra": "mean: 641.9114127357986 usec\nrounds: 424"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1384.4583573520035,
            "unit": "iter/sec",
            "range": "stddev: 0.00003650814075017505",
            "extra": "mean: 722.3041377081639 usec\nrounds: 1082"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2086.4985007550385,
            "unit": "iter/sec",
            "range": "stddev: 0.000028381244409576333",
            "extra": "mean: 479.2718516874712 usec\nrounds: 1571"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 174.61899877890156,
            "unit": "iter/sec",
            "range": "stddev: 0.000560483027725918",
            "extra": "mean: 5.726753715190959 msec\nrounds: 158"
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
        "date": 1779028178583,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2339.682101184618,
            "unit": "iter/sec",
            "range": "stddev: 0.00003006208840432029",
            "extra": "mean: 427.4084925869562 usec\nrounds: 1214"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1662.9301906391167,
            "unit": "iter/sec",
            "range": "stddev: 0.00005526357172879578",
            "extra": "mean: 601.3481537764784 usec\nrounds: 1112"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 823.7707759575503,
            "unit": "iter/sec",
            "range": "stddev: 0.0005156007843998893",
            "extra": "mean: 1.2139299295214754 msec\nrounds: 752"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 734.6139060752147,
            "unit": "iter/sec",
            "range": "stddev: 0.00018129109956623564",
            "extra": "mean: 1.3612592842717208 msec\nrounds: 693"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1170.6327170291893,
            "unit": "iter/sec",
            "range": "stddev: 0.001847900714573202",
            "extra": "mean: 854.238896156757 usec\nrounds: 1223"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1102.853889577368,
            "unit": "iter/sec",
            "range": "stddev: 0.002303962833096432",
            "extra": "mean: 906.7384260513574 usec\nrounds: 1474"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1192.382280188275,
            "unit": "iter/sec",
            "range": "stddev: 0.0017947472325739065",
            "extra": "mean: 838.6572130559521 usec\nrounds: 1394"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1589.2930921379643,
            "unit": "iter/sec",
            "range": "stddev: 0.00005078032205331215",
            "extra": "mean: 629.2105621970396 usec\nrounds: 1238"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1498.1005666554188,
            "unit": "iter/sec",
            "range": "stddev: 0.00002693613581446202",
            "extra": "mean: 667.5119296113397 usec\nrounds: 412"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1305.7893157105752,
            "unit": "iter/sec",
            "range": "stddev: 0.000046533365419420895",
            "extra": "mean: 765.8203264251914 usec\nrounds: 1158"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1905.4291086986798,
            "unit": "iter/sec",
            "range": "stddev: 0.00003735264753202159",
            "extra": "mean: 524.8161663085717 usec\nrounds: 1395"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 166.86213535807863,
            "unit": "iter/sec",
            "range": "stddev: 0.001138130995801803",
            "extra": "mean: 5.992971370371384 msec\nrounds: 162"
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
        "date": 1779037927724,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2313.2997346296133,
            "unit": "iter/sec",
            "range": "stddev: 0.0000313137658663227",
            "extra": "mean: 432.28293550991646 usec\nrounds: 1225"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1652.8539687265534,
            "unit": "iter/sec",
            "range": "stddev: 0.00005782764645814541",
            "extra": "mean: 605.0141264267002 usec\nrounds: 1139"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 821.5742158208949,
            "unit": "iter/sec",
            "range": "stddev: 0.0004847643842740631",
            "extra": "mean: 1.217175491566306 msec\nrounds: 830"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 740.6461867397196,
            "unit": "iter/sec",
            "range": "stddev: 0.00014682475412980782",
            "extra": "mean: 1.3501723466665512 msec\nrounds: 675"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1167.0520374869927,
            "unit": "iter/sec",
            "range": "stddev: 0.0019170137412515957",
            "extra": "mean: 856.8598210524485 usec\nrounds: 1235"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1088.1352627659003,
            "unit": "iter/sec",
            "range": "stddev: 0.0023819156440411464",
            "extra": "mean: 919.0033943557056 usec\nrounds: 1311"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1199.1437707870878,
            "unit": "iter/sec",
            "range": "stddev: 0.0017017999314191531",
            "extra": "mean: 833.9283615205082 usec\nrounds: 1289"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1587.5434033624351,
            "unit": "iter/sec",
            "range": "stddev: 0.000047171115470946044",
            "extra": "mean: 629.9040378247224 usec\nrounds: 1269"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1483.9560606551256,
            "unit": "iter/sec",
            "range": "stddev: 0.000047355921393033536",
            "extra": "mean: 673.8743999997733 usec\nrounds: 460"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1305.6045983516801,
            "unit": "iter/sec",
            "range": "stddev: 0.000054157344108246424",
            "extra": "mean: 765.9286749315187 usec\nrounds: 1089"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1891.7811727231797,
            "unit": "iter/sec",
            "range": "stddev: 0.000040254435894665806",
            "extra": "mean: 528.6023639618534 usec\nrounds: 1676"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 173.96543076048806,
            "unit": "iter/sec",
            "range": "stddev: 0.001141603488181207",
            "extra": "mean: 5.748268467065615 msec\nrounds: 167"
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
        "date": 1779041740155,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2386.838622766092,
            "unit": "iter/sec",
            "range": "stddev: 0.000028492586657518413",
            "extra": "mean: 418.96422760291443 usec\nrounds: 1239"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1707.8888366511328,
            "unit": "iter/sec",
            "range": "stddev: 0.000046585962250807256",
            "extra": "mean: 585.5182015012305 usec\nrounds: 799"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 839.4782523828543,
            "unit": "iter/sec",
            "range": "stddev: 0.0004957604607304997",
            "extra": "mean: 1.1912160882804357 msec\nrounds: 657"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 761.8277898274893,
            "unit": "iter/sec",
            "range": "stddev: 0.00009099985102822092",
            "extra": "mean: 1.3126326098270098 msec\nrounds: 692"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1195.607005711398,
            "unit": "iter/sec",
            "range": "stddev: 0.001785128534203126",
            "extra": "mean: 836.3952329009566 usec\nrounds: 1155"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1077.0150057897156,
            "unit": "iter/sec",
            "range": "stddev: 0.0025566067684915364",
            "extra": "mean: 928.4921701408934 usec\nrounds: 1487"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1203.8427935782222,
            "unit": "iter/sec",
            "range": "stddev: 0.0016876075852630605",
            "extra": "mean: 830.6732451565928 usec\nrounds: 1342"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1626.3728466898785,
            "unit": "iter/sec",
            "range": "stddev: 0.000033002258157692794",
            "extra": "mean: 614.8651596313098 usec\nrounds: 1303"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1393.9358817097311,
            "unit": "iter/sec",
            "range": "stddev: 0.00008537839124601575",
            "extra": "mean: 717.3931119223724 usec\nrounds: 411"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1271.8173120845336,
            "unit": "iter/sec",
            "range": "stddev: 0.000045358257301007244",
            "extra": "mean: 786.2764490608957 usec\nrounds: 1011"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1944.2355830518861,
            "unit": "iter/sec",
            "range": "stddev: 0.00003065699133116194",
            "extra": "mean: 514.3409619272012 usec\nrounds: 1681"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 176.12722667065864,
            "unit": "iter/sec",
            "range": "stddev: 0.0006807949831509457",
            "extra": "mean: 5.677713882759909 msec\nrounds: 145"
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
        "date": 1779044592150,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2500.5132666393515,
            "unit": "iter/sec",
            "range": "stddev: 0.00002601646997254382",
            "extra": "mean: 399.91789419457206 usec\nrounds: 1068"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1749.7601806619593,
            "unit": "iter/sec",
            "range": "stddev: 0.00004997679704605893",
            "extra": "mean: 571.5068905166682 usec\nrounds: 1160"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 962.5353728360269,
            "unit": "iter/sec",
            "range": "stddev: 0.00032074124031081057",
            "extra": "mean: 1.0389228575086928 msec\nrounds: 779"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 737.0872402634699,
            "unit": "iter/sec",
            "range": "stddev: 0.00008771358415979003",
            "extra": "mean: 1.3566915086503908 msec\nrounds: 578"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1199.9405506325709,
            "unit": "iter/sec",
            "range": "stddev: 0.001973493652616489",
            "extra": "mean: 833.374619661642 usec\nrounds: 1241"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1075.0233124143263,
            "unit": "iter/sec",
            "range": "stddev: 0.0027225688421988964",
            "extra": "mean: 930.21238558461 usec\nrounds: 1429"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1225.2794120134915,
            "unit": "iter/sec",
            "range": "stddev: 0.0019132166831271182",
            "extra": "mean: 816.1403759789845 usec\nrounds: 1532"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1662.2536030120084,
            "unit": "iter/sec",
            "range": "stddev: 0.00015678272730715719",
            "extra": "mean: 601.5929207119763 usec\nrounds: 1236"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1448.9024249337765,
            "unit": "iter/sec",
            "range": "stddev: 0.00005758006952222766",
            "extra": "mean: 690.1776011905744 usec\nrounds: 336"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1293.5045427332864,
            "unit": "iter/sec",
            "range": "stddev: 0.000041161456841380874",
            "extra": "mean: 773.093535401827 usec\nrounds: 1031"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2073.234098892876,
            "unit": "iter/sec",
            "range": "stddev: 0.00002743284798864021",
            "extra": "mean: 482.3381983414262 usec\nrounds: 1447"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 177.42446160474302,
            "unit": "iter/sec",
            "range": "stddev: 0.0004499539180877003",
            "extra": "mean: 5.636201406251118 msec\nrounds: 160"
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
        "date": 1779106507819,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2505.2886513992153,
            "unit": "iter/sec",
            "range": "stddev: 0.000028444291029454473",
            "extra": "mean: 399.1556020666502 usec\nrounds: 1161"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1753.958000491967,
            "unit": "iter/sec",
            "range": "stddev: 0.000056316999834564636",
            "extra": "mean: 570.1390795671906 usec\nrounds: 1018"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 957.6717932333893,
            "unit": "iter/sec",
            "range": "stddev: 0.00037276914231554185",
            "extra": "mean: 1.0441990743234673 msec\nrounds: 740"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 716.986621559875,
            "unit": "iter/sec",
            "range": "stddev: 0.00008154541723149192",
            "extra": "mean: 1.3947261635431938 msec\nrounds: 587"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1185.20888012124,
            "unit": "iter/sec",
            "range": "stddev: 0.001951246205718461",
            "extra": "mean: 843.7331315790561 usec\nrounds: 1216"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1154.262566592522,
            "unit": "iter/sec",
            "range": "stddev: 0.002568964346824465",
            "extra": "mean: 866.354007262041 usec\nrounds: 1377"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1217.7912784299106,
            "unit": "iter/sec",
            "range": "stddev: 0.0018459439338171905",
            "extra": "mean: 821.1587795975125 usec\nrounds: 1343"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1684.6152226506379,
            "unit": "iter/sec",
            "range": "stddev: 0.00003276796275723653",
            "extra": "mean: 593.6073630075371 usec\nrounds: 1303"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1591.6759848476852,
            "unit": "iter/sec",
            "range": "stddev: 0.000023627607188252784",
            "extra": "mean: 628.2685732019099 usec\nrounds: 403"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1373.2709800062235,
            "unit": "iter/sec",
            "range": "stddev: 0.00005407378683195821",
            "extra": "mean: 728.1884016768985 usec\nrounds: 1073"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2070.3482731712725,
            "unit": "iter/sec",
            "range": "stddev: 0.000027671077466848403",
            "extra": "mean: 483.0105219293574 usec\nrounds: 1596"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 178.3793844943274,
            "unit": "iter/sec",
            "range": "stddev: 0.0004520445504553905",
            "extra": "mean: 5.606028986111905 msec\nrounds: 144"
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
        "date": 1779121430221,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2283.717014834355,
            "unit": "iter/sec",
            "range": "stddev: 0.00003015049786317123",
            "extra": "mean: 437.88262446892224 usec\nrounds: 1177"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1658.219871085056,
            "unit": "iter/sec",
            "range": "stddev: 0.0000470585278728974",
            "extra": "mean: 603.0563361574301 usec\nrounds: 1062"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 790.6755540269577,
            "unit": "iter/sec",
            "range": "stddev: 0.00048064804898624304",
            "extra": "mean: 1.2647412644882474 msec\nrounds: 673"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 690.4614140156989,
            "unit": "iter/sec",
            "range": "stddev: 0.0001592737787127494",
            "extra": "mean: 1.4483068564020627 msec\nrounds: 578"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1168.3920768467449,
            "unit": "iter/sec",
            "range": "stddev: 0.0018272332487061652",
            "extra": "mean: 855.8770808330014 usec\nrounds: 1200"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1041.159393552439,
            "unit": "iter/sec",
            "range": "stddev: 0.0026264691331975305",
            "extra": "mean: 960.4677306785823 usec\nrounds: 1281"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1196.873716114279,
            "unit": "iter/sec",
            "range": "stddev: 0.0017703340115138397",
            "extra": "mean: 835.5100346313552 usec\nrounds: 1155"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1567.9537182407234,
            "unit": "iter/sec",
            "range": "stddev: 0.00025866242876124314",
            "extra": "mean: 637.7739268490786 usec\nrounds: 1244"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1458.1585838752724,
            "unit": "iter/sec",
            "range": "stddev: 0.00007264820764525646",
            "extra": "mean: 685.7964634699416 usec\nrounds: 438"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1291.6717918993963,
            "unit": "iter/sec",
            "range": "stddev: 0.00004561128657527399",
            "extra": "mean: 774.1904764595853 usec\nrounds: 1062"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1887.129788257979,
            "unit": "iter/sec",
            "range": "stddev: 0.000029652990786589294",
            "extra": "mean: 529.9052594167919 usec\nrounds: 1646"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 171.1187454577439,
            "unit": "iter/sec",
            "range": "stddev: 0.0011350087666591388",
            "extra": "mean: 5.843895111111251 msec\nrounds: 144"
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
        "date": 1779288829978,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2478.563787717561,
            "unit": "iter/sec",
            "range": "stddev: 0.000029123296724187652",
            "extra": "mean: 403.4594570272777 usec\nrounds: 989"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1708.04603423257,
            "unit": "iter/sec",
            "range": "stddev: 0.00005710503914106029",
            "extra": "mean: 585.4643141683841 usec\nrounds: 974"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 895.6573073742103,
            "unit": "iter/sec",
            "range": "stddev: 0.0004143901170953137",
            "extra": "mean: 1.1164984551197268 msec\nrounds: 791"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 608.9763800473479,
            "unit": "iter/sec",
            "range": "stddev: 0.000308852729606613",
            "extra": "mean: 1.642099813333072 msec\nrounds: 525"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1092.9100848070066,
            "unit": "iter/sec",
            "range": "stddev: 0.001760466397609357",
            "extra": "mean: 914.9883543956745 usec\nrounds: 1092"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 972.6189464143191,
            "unit": "iter/sec",
            "range": "stddev: 0.002518236239588968",
            "extra": "mean: 1.028151881768934 msec\nrounds: 1108"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1234.4446981070948,
            "unit": "iter/sec",
            "range": "stddev: 0.0014812321424449936",
            "extra": "mean: 810.0808416394887 usec\nrounds: 903"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1529.6488876806875,
            "unit": "iter/sec",
            "range": "stddev: 0.00034474356084728127",
            "extra": "mean: 653.7447959814087 usec\nrounds: 1294"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1463.4040312931186,
            "unit": "iter/sec",
            "range": "stddev: 0.0004326630663558408",
            "extra": "mean: 683.3382843126122 usec\nrounds: 408"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1295.7869279640959,
            "unit": "iter/sec",
            "range": "stddev: 0.00022541495274937002",
            "extra": "mean: 771.7318167201856 usec\nrounds: 933"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1906.57146974939,
            "unit": "iter/sec",
            "range": "stddev: 0.00018475591204566595",
            "extra": "mean: 524.5017120346637 usec\nrounds: 1396"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 168.95058656992845,
            "unit": "iter/sec",
            "range": "stddev: 0.0005726364264713364",
            "extra": "mean: 5.918890370860601 msec\nrounds: 151"
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
        "date": 1779293502393,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2356.1598872017007,
            "unit": "iter/sec",
            "range": "stddev: 0.00003270228222314316",
            "extra": "mean: 424.4194145872047 usec\nrounds: 1042"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1692.824259772959,
            "unit": "iter/sec",
            "range": "stddev: 0.00005471868564107927",
            "extra": "mean: 590.7287742521598 usec\nrounds: 1103"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 816.5884768436952,
            "unit": "iter/sec",
            "range": "stddev: 0.000491148594517576",
            "extra": "mean: 1.2246070430300866 msec\nrounds: 581"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 623.7132469188329,
            "unit": "iter/sec",
            "range": "stddev: 0.0002515189636397312",
            "extra": "mean: 1.6033008837635532 msec\nrounds: 542"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1096.2377544172512,
            "unit": "iter/sec",
            "range": "stddev: 0.0018498402749688389",
            "extra": "mean: 912.2108739372783 usec\nrounds: 706"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1107.158654856082,
            "unit": "iter/sec",
            "range": "stddev: 0.002391098128653218",
            "extra": "mean: 903.2129185947506 usec\nrounds: 1167"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1158.5841690688683,
            "unit": "iter/sec",
            "range": "stddev: 0.0019474022951481782",
            "extra": "mean: 863.1224443569608 usec\nrounds: 1267"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1609.402732259576,
            "unit": "iter/sec",
            "range": "stddev: 0.0002920466198691575",
            "extra": "mean: 621.3485164126793 usec\nrounds: 1249"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1439.764848997675,
            "unit": "iter/sec",
            "range": "stddev: 0.00003302227198538554",
            "extra": "mean: 694.5578652626314 usec\nrounds: 475"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1255.3050751472595,
            "unit": "iter/sec",
            "range": "stddev: 0.00007370056295654605",
            "extra": "mean: 796.6191006458652 usec\nrounds: 1083"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1915.2815281632384,
            "unit": "iter/sec",
            "range": "stddev: 0.00003830794435276163",
            "extra": "mean: 522.1164540541481 usec\nrounds: 925"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 213.5812969744396,
            "unit": "iter/sec",
            "range": "stddev: 0.0009278506191819692",
            "extra": "mean: 4.682057905658636 msec\nrounds: 212"
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
        "date": 1779293678300,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2382.6073722458145,
            "unit": "iter/sec",
            "range": "stddev: 0.00003568658847425171",
            "extra": "mean: 419.7082623216317 usec\nrounds: 1258"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1701.8487058392009,
            "unit": "iter/sec",
            "range": "stddev: 0.00005522590958531917",
            "extra": "mean: 587.5962984070835 usec\nrounds: 1193"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 837.3545481498737,
            "unit": "iter/sec",
            "range": "stddev: 0.000545300950841288",
            "extra": "mean: 1.1942372585298424 msec\nrounds: 762"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 719.5722100304014,
            "unit": "iter/sec",
            "range": "stddev: 0.00012221136764315412",
            "extra": "mean: 1.3897145916151359 msec\nrounds: 644"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1213.7740754284473,
            "unit": "iter/sec",
            "range": "stddev: 0.001743346955853729",
            "extra": "mean: 823.8765518591359 usec\nrounds: 1022"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1078.1581078325676,
            "unit": "iter/sec",
            "range": "stddev: 0.0025251488620187767",
            "extra": "mean: 927.5077493136052 usec\nrounds: 1456"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1256.0657152133,
            "unit": "iter/sec",
            "range": "stddev: 0.001573450014115707",
            "extra": "mean: 796.1366892576825 usec\nrounds: 1387"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1620.059186322061,
            "unit": "iter/sec",
            "range": "stddev: 0.00004505581713579305",
            "extra": "mean: 617.2613991160717 usec\nrounds: 1358"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1524.2754860021503,
            "unit": "iter/sec",
            "range": "stddev: 0.0000394797970639594",
            "extra": "mean: 656.0493881737788 usec\nrounds: 389"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1328.5485954466571,
            "unit": "iter/sec",
            "range": "stddev: 0.00004444117471799925",
            "extra": "mean: 752.701108132067 usec\nrounds: 1119"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1945.0738714125002,
            "unit": "iter/sec",
            "range": "stddev: 0.00002676741239650691",
            "extra": "mean: 514.1192911474392 usec\nrounds: 1604"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 183.64484517361132,
            "unit": "iter/sec",
            "range": "stddev: 0.0009784604200454468",
            "extra": "mean: 5.445293054943282 msec\nrounds: 182"
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
        "date": 1779293876104,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2952.118298288865,
            "unit": "iter/sec",
            "range": "stddev: 0.000031454705860268185",
            "extra": "mean: 338.7398128928741 usec\nrounds: 1272"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1978.3774098034712,
            "unit": "iter/sec",
            "range": "stddev: 0.00006588447923548304",
            "extra": "mean: 505.4647283398461 usec\nrounds: 1108"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1003.5735842804198,
            "unit": "iter/sec",
            "range": "stddev: 0.00010568013948763908",
            "extra": "mean: 996.4391407502201 usec\nrounds: 881"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 803.6095118325003,
            "unit": "iter/sec",
            "range": "stddev: 0.00014147127731759204",
            "extra": "mean: 1.2443854699027432 msec\nrounds: 515"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1432.720578832872,
            "unit": "iter/sec",
            "range": "stddev: 0.0012622716150109332",
            "extra": "mean: 697.9728041699685 usec\nrounds: 1583"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1303.2576360216476,
            "unit": "iter/sec",
            "range": "stddev: 0.0017334504652698325",
            "extra": "mean: 767.3079921884222 usec\nrounds: 1408"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1409.2449226877168,
            "unit": "iter/sec",
            "range": "stddev: 0.0014081889956647498",
            "extra": "mean: 709.5998601100486 usec\nrounds: 915"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1881.284319325343,
            "unit": "iter/sec",
            "range": "stddev: 0.00003511767153323902",
            "extra": "mean: 531.5517647851417 usec\nrounds: 1471"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1785.8352660426315,
            "unit": "iter/sec",
            "range": "stddev: 0.00003707672848590241",
            "extra": "mean: 559.9620631392146 usec\nrounds: 491"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1497.4109636357139,
            "unit": "iter/sec",
            "range": "stddev: 0.0000624043083962287",
            "extra": "mean: 667.8193390356913 usec\nrounds: 1286"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2451.0241794395106,
            "unit": "iter/sec",
            "range": "stddev: 0.000030299443000518334",
            "extra": "mean: 407.99271112399856 usec\nrounds: 1717"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 178.17587193245228,
            "unit": "iter/sec",
            "range": "stddev: 0.00014416761475987875",
            "extra": "mean: 5.612432194966932 msec\nrounds: 159"
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
        "date": 1779473299682,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2345.3925667397416,
            "unit": "iter/sec",
            "range": "stddev: 0.000027576106970142103",
            "extra": "mean: 426.3678559321391 usec\nrounds: 1180"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1684.6201248373466,
            "unit": "iter/sec",
            "range": "stddev: 0.00004895846850457558",
            "extra": "mean: 593.6056356304967 usec\nrounds: 1117"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 841.0208763688439,
            "unit": "iter/sec",
            "range": "stddev: 0.00039951780689911936",
            "extra": "mean: 1.1890311264537898 msec\nrounds: 688"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 722.7450260028987,
            "unit": "iter/sec",
            "range": "stddev: 0.00008808981326191434",
            "extra": "mean: 1.3836138112640421 msec\nrounds: 657"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1181.8036987803118,
            "unit": "iter/sec",
            "range": "stddev: 0.0018424719081445709",
            "extra": "mean: 846.164215793246 usec\nrounds: 1469"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1102.9389303456232,
            "unit": "iter/sec",
            "range": "stddev: 0.002275994448909158",
            "extra": "mean: 906.6685130849758 usec\nrounds: 1261"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1192.0985151231234,
            "unit": "iter/sec",
            "range": "stddev: 0.0018093609120807914",
            "extra": "mean: 838.8568455659196 usec\nrounds: 1308"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1588.6420559242345,
            "unit": "iter/sec",
            "range": "stddev: 0.00019206187275023896",
            "extra": "mean: 629.4684169230453 usec\nrounds: 1300"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1510.9387554387154,
            "unit": "iter/sec",
            "range": "stddev: 0.000027464420984639943",
            "extra": "mean: 661.8401946474928 usec\nrounds: 411"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1321.4534401182505,
            "unit": "iter/sec",
            "range": "stddev: 0.00004303699138869021",
            "extra": "mean: 756.7425152039522 usec\nrounds: 1151"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1919.0975092653289,
            "unit": "iter/sec",
            "range": "stddev: 0.000034054986597876905",
            "extra": "mean: 521.0782647426921 usec\nrounds: 1594"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 162.32175569748733,
            "unit": "iter/sec",
            "range": "stddev: 0.0010192184947409426",
            "extra": "mean: 6.1606036461536355 msec\nrounds: 130"
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
        "date": 1779473302116,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2520.919891581957,
            "unit": "iter/sec",
            "range": "stddev: 0.000031873740618098626",
            "extra": "mean: 396.680593992405 usec\nrounds: 1032"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1771.0934536769628,
            "unit": "iter/sec",
            "range": "stddev: 0.00005094742363392682",
            "extra": "mean: 564.6229440484365 usec\nrounds: 983"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 964.1229261445196,
            "unit": "iter/sec",
            "range": "stddev: 0.0003087434477894927",
            "extra": "mean: 1.0372121364222207 msec\nrounds: 777"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 713.8023717504998,
            "unit": "iter/sec",
            "range": "stddev: 0.00008703547482593047",
            "extra": "mean: 1.400947992856399 msec\nrounds: 420"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1176.107529327711,
            "unit": "iter/sec",
            "range": "stddev: 0.0021171194549807677",
            "extra": "mean: 850.2623910346208 usec\nrounds: 1450"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1105.9150151661238,
            "unit": "iter/sec",
            "range": "stddev: 0.002578694363304783",
            "extra": "mean: 904.2286127653183 usec\nrounds: 1645"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1202.5015456233882,
            "unit": "iter/sec",
            "range": "stddev: 0.0020099414994117727",
            "extra": "mean: 831.5997627109831 usec\nrounds: 1416"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1697.912794014563,
            "unit": "iter/sec",
            "range": "stddev: 0.000026531624752145952",
            "extra": "mean: 588.958398526222 usec\nrounds: 1222"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1428.2747618117296,
            "unit": "iter/sec",
            "range": "stddev: 0.00008454205690679279",
            "extra": "mean: 700.1453969063529 usec\nrounds: 388"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1300.4815022308846,
            "unit": "iter/sec",
            "range": "stddev: 0.000041012241297375255",
            "extra": "mean: 768.9459621567629 usec\nrounds: 1057"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2052.7682220204038,
            "unit": "iter/sec",
            "range": "stddev: 0.00005104028712634524",
            "extra": "mean: 487.14705794488884 usec\nrounds: 1605"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 155.7845073726032,
            "unit": "iter/sec",
            "range": "stddev: 0.00039359806165758994",
            "extra": "mean: 6.419123549995983 msec\nrounds: 140"
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
        "date": 1779473417105,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2180.845272778753,
            "unit": "iter/sec",
            "range": "stddev: 0.00017672338629962472",
            "extra": "mean: 458.53780297115554 usec\nrounds: 1010"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1487.3755364519973,
            "unit": "iter/sec",
            "range": "stddev: 0.00022158069280262856",
            "extra": "mean: 672.3251630085375 usec\nrounds: 957"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 828.4334439297326,
            "unit": "iter/sec",
            "range": "stddev: 0.0003608849495319194",
            "extra": "mean: 1.2070975735315916 msec\nrounds: 408"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 620.9218093918468,
            "unit": "iter/sec",
            "range": "stddev: 0.00026143021323558646",
            "extra": "mean: 1.6105087385792363 msec\nrounds: 394"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1091.778674084263,
            "unit": "iter/sec",
            "range": "stddev: 0.001807808604761387",
            "extra": "mean: 915.9365572319473 usec\nrounds: 1127"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1044.5229772786547,
            "unit": "iter/sec",
            "range": "stddev: 0.0021907480901595523",
            "extra": "mean: 957.3748225293689 usec\nrounds: 941"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1102.0849921247636,
            "unit": "iter/sec",
            "range": "stddev: 0.0016808966988590057",
            "extra": "mean: 907.3710350342863 usec\nrounds: 1313"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1572.0195416120616,
            "unit": "iter/sec",
            "range": "stddev: 0.00016083279618100133",
            "extra": "mean: 636.1244078267172 usec\nrounds: 971"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1395.7561870659147,
            "unit": "iter/sec",
            "range": "stddev: 0.00018410134801533494",
            "extra": "mean: 716.4575083146487 usec\nrounds: 421"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1209.3674413471356,
            "unit": "iter/sec",
            "range": "stddev: 0.00020280564029558526",
            "extra": "mean: 826.8785530442943 usec\nrounds: 1018"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1799.7615668058568,
            "unit": "iter/sec",
            "range": "stddev: 0.00016839982359303132",
            "extra": "mean: 555.6291557968754 usec\nrounds: 1380"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 170.37414297899386,
            "unit": "iter/sec",
            "range": "stddev: 0.0003487949725941345",
            "extra": "mean: 5.869435247127225 msec\nrounds: 174"
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
        "date": 1779476757219,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2521.473105976459,
            "unit": "iter/sec",
            "range": "stddev: 0.000026663958860689747",
            "extra": "mean: 396.5935617674346 usec\nrounds: 1109"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1782.4731469984174,
            "unit": "iter/sec",
            "range": "stddev: 0.00004378363834809961",
            "extra": "mean: 561.0182692984423 usec\nrounds: 1140"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 884.2332922737413,
            "unit": "iter/sec",
            "range": "stddev: 0.00046133334952001956",
            "extra": "mean: 1.1309232628287191 msec\nrounds: 799"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 740.9485717095354,
            "unit": "iter/sec",
            "range": "stddev: 0.00009531752930943407",
            "extra": "mean: 1.349621334302291 msec\nrounds: 688"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1234.6641099303456,
            "unit": "iter/sec",
            "range": "stddev: 0.001768738362428726",
            "extra": "mean: 809.9368823933951 usec\nrounds: 1437"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1141.3503699459507,
            "unit": "iter/sec",
            "range": "stddev: 0.0022846712123135867",
            "extra": "mean: 876.1551459849753 usec\nrounds: 1370"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1222.664231625383,
            "unit": "iter/sec",
            "range": "stddev: 0.0018547095550809946",
            "extra": "mean: 817.8860345580093 usec\nrounds: 1389"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1684.9802064697171,
            "unit": "iter/sec",
            "range": "stddev: 0.000053888238336878645",
            "extra": "mean: 593.4787816262531 usec\nrounds: 1328"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1514.0112419603327,
            "unit": "iter/sec",
            "range": "stddev: 0.00004037318129033278",
            "extra": "mean: 660.4970770925096 usec\nrounds: 454"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1321.5133223195544,
            "unit": "iter/sec",
            "range": "stddev: 0.00005001019097715826",
            "extra": "mean: 756.7082246622941 usec\nrounds: 1184"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2010.7653304308199,
            "unit": "iter/sec",
            "range": "stddev: 0.00003885871839738671",
            "extra": "mean: 497.32307637597034 usec\nrounds: 1545"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 161.61490656086445,
            "unit": "iter/sec",
            "range": "stddev: 0.0011476271622163354",
            "extra": "mean: 6.187548050361296 msec\nrounds: 139"
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
        "date": 1779476971204,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2305.027769469732,
            "unit": "iter/sec",
            "range": "stddev: 0.00003453706957659505",
            "extra": "mean: 433.83425277780856 usec\nrounds: 1080"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1659.1084828388045,
            "unit": "iter/sec",
            "range": "stddev: 0.000052770131569141844",
            "extra": "mean: 602.7333416371653 usec\nrounds: 1124"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 810.3316674373547,
            "unit": "iter/sec",
            "range": "stddev: 0.0006106554340495511",
            "extra": "mean: 1.2340625945947103 msec\nrounds: 740"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 702.2826226786124,
            "unit": "iter/sec",
            "range": "stddev: 0.00009326438597036323",
            "extra": "mean: 1.4239281561401143 msec\nrounds: 570"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1164.3684140213943,
            "unit": "iter/sec",
            "range": "stddev: 0.0017856842420919902",
            "extra": "mean: 858.8347021079755 usec\nrounds: 1044"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1052.883906266516,
            "unit": "iter/sec",
            "range": "stddev: 0.002556331071265542",
            "extra": "mean: 949.772329169661 usec\nrounds: 1121"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1173.3443993461185,
            "unit": "iter/sec",
            "range": "stddev: 0.0018401518941653294",
            "extra": "mean: 852.2646893420894 usec\nrounds: 1323"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1588.4406802373444,
            "unit": "iter/sec",
            "range": "stddev: 0.00010335784983568093",
            "extra": "mean: 629.5482182252977 usec\nrounds: 1251"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1469.4551009847687,
            "unit": "iter/sec",
            "range": "stddev: 0.00006214332083294602",
            "extra": "mean: 680.5243653445696 usec\nrounds: 479"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1300.0134649196198,
            "unit": "iter/sec",
            "range": "stddev: 0.000047624512282671826",
            "extra": "mean: 769.2228019052328 usec\nrounds: 1050"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1898.9383636711834,
            "unit": "iter/sec",
            "range": "stddev: 0.00003168189691660661",
            "extra": "mean: 526.610035971214 usec\nrounds: 1529"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 189.45517833656248,
            "unit": "iter/sec",
            "range": "stddev: 0.0007987186833582156",
            "extra": "mean: 5.278293308106493 msec\nrounds: 185"
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
        "date": 1779584326819,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2291.6728359004305,
            "unit": "iter/sec",
            "range": "stddev: 0.00004170561014402355",
            "extra": "mean: 436.3624616631134 usec\nrounds: 1226"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1646.5255375830047,
            "unit": "iter/sec",
            "range": "stddev: 0.00005254320300548057",
            "extra": "mean: 607.3395019842429 usec\nrounds: 1008"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 802.8049135782713,
            "unit": "iter/sec",
            "range": "stddev: 0.0005997759041964945",
            "extra": "mean: 1.245632635135214 msec\nrounds: 814"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 684.8956436494456,
            "unit": "iter/sec",
            "range": "stddev: 0.00013103941533986222",
            "extra": "mean: 1.4600764499997847 msec\nrounds: 600"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1126.200944945468,
            "unit": "iter/sec",
            "range": "stddev: 0.002096659114963615",
            "extra": "mean: 887.9410059884306 usec\nrounds: 1169"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1064.1242288116275,
            "unit": "iter/sec",
            "range": "stddev: 0.0024877787970900957",
            "extra": "mean: 939.7399034103012 usec\nrounds: 1408"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1135.3336003096736,
            "unit": "iter/sec",
            "range": "stddev: 0.002018383670690798",
            "extra": "mean: 880.7983836004148 usec\nrounds: 1439"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1570.1468358155398,
            "unit": "iter/sec",
            "range": "stddev: 0.0000694582436870007",
            "extra": "mean: 636.8831100312963 usec\nrounds: 1236"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1479.430094440844,
            "unit": "iter/sec",
            "range": "stddev: 0.000031094483266441354",
            "extra": "mean: 675.9359592302694 usec\nrounds: 417"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1289.7953012503617,
            "unit": "iter/sec",
            "range": "stddev: 0.00004869426255051865",
            "extra": "mean: 775.3168266550308 usec\nrounds: 1148"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1872.244183044028,
            "unit": "iter/sec",
            "range": "stddev: 0.000039665360156953184",
            "extra": "mean: 534.1183639700934 usec\nrounds: 1632"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 161.49076862856256,
            "unit": "iter/sec",
            "range": "stddev: 0.0011314065853995057",
            "extra": "mean: 6.192304417722189 msec\nrounds: 158"
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
        "date": 1779584530938,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2882.389889522627,
            "unit": "iter/sec",
            "range": "stddev: 0.00008256667115481908",
            "extra": "mean: 346.9343282235899 usec\nrounds: 1109"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1983.4424812326465,
            "unit": "iter/sec",
            "range": "stddev: 0.00015326421733169",
            "extra": "mean: 504.17393469284366 usec\nrounds: 1179"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1053.424888456135,
            "unit": "iter/sec",
            "range": "stddev: 0.00010028531535951727",
            "extra": "mean: 949.2845773423553 usec\nrounds: 918"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 831.8522179039447,
            "unit": "iter/sec",
            "range": "stddev: 0.00008455485646844688",
            "extra": "mean: 1.2021366036863432 msec\nrounds: 651"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1482.0752170337614,
            "unit": "iter/sec",
            "range": "stddev: 0.0011011175894812125",
            "extra": "mean: 674.7295876125701 usec\nrounds: 1324"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1362.6155538488883,
            "unit": "iter/sec",
            "range": "stddev: 0.0015870036666399505",
            "extra": "mean: 733.8827134149228 usec\nrounds: 1476"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1495.1659043546197,
            "unit": "iter/sec",
            "range": "stddev: 0.001137239487457321",
            "extra": "mean: 668.8221000007652 usec\nrounds: 1510"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1932.8375161345741,
            "unit": "iter/sec",
            "range": "stddev: 0.00004492466186417347",
            "extra": "mean: 517.3740635994437 usec\nrounds: 456"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1800.6900761005147,
            "unit": "iter/sec",
            "range": "stddev: 0.0000441080667377292",
            "extra": "mean: 555.3426507272981 usec\nrounds: 481"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1524.7475377942815,
            "unit": "iter/sec",
            "range": "stddev: 0.000055507067687039766",
            "extra": "mean: 655.8462796054829 usec\nrounds: 1216"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2447.3186666385504,
            "unit": "iter/sec",
            "range": "stddev: 0.00003605404341108159",
            "extra": "mean: 408.6104574904107 usec\nrounds: 1729"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 188.07495575072454,
            "unit": "iter/sec",
            "range": "stddev: 0.00017657883361036673",
            "extra": "mean: 5.31702903243218 msec\nrounds: 185"
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
        "date": 1779584633274,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2301.186104957465,
            "unit": "iter/sec",
            "range": "stddev: 0.000026864684232797583",
            "extra": "mean: 434.5585078258953 usec\nrounds: 1150"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1646.5114474621703,
            "unit": "iter/sec",
            "range": "stddev: 0.000057062721885780614",
            "extra": "mean: 607.3446993285941 usec\nrounds: 745"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 817.4126386792078,
            "unit": "iter/sec",
            "range": "stddev: 0.0005622760017252439",
            "extra": "mean: 1.2233723246753567 msec\nrounds: 770"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 638.5085690055726,
            "unit": "iter/sec",
            "range": "stddev: 0.0002715733115466526",
            "extra": "mean: 1.5661496940556683 msec\nrounds: 572"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1152.0384003503832,
            "unit": "iter/sec",
            "range": "stddev: 0.0019216736369841933",
            "extra": "mean: 868.0266210708412 usec\nrounds: 1177"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1063.9211213943074,
            "unit": "iter/sec",
            "range": "stddev: 0.0024788392292297593",
            "extra": "mean: 939.9193040640678 usec\nrounds: 1378"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1163.9745605309101,
            "unit": "iter/sec",
            "range": "stddev: 0.0019208029288223423",
            "extra": "mean: 859.1253055770237 usec\nrounds: 1273"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1560.7767928284711,
            "unit": "iter/sec",
            "range": "stddev: 0.00038535789908266267",
            "extra": "mean: 640.7066049385447 usec\nrounds: 1215"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1401.8147802876442,
            "unit": "iter/sec",
            "range": "stddev: 0.00006619263589004942",
            "extra": "mean: 713.3610046505615 usec\nrounds: 430"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1255.168938056964,
            "unit": "iter/sec",
            "range": "stddev: 0.000053556430855236044",
            "extra": "mean: 796.705502884757 usec\nrounds: 1040"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1885.688123534343,
            "unit": "iter/sec",
            "range": "stddev: 0.00002729947537682537",
            "extra": "mean: 530.3103877674645 usec\nrounds: 1684"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 166.51262676066995,
            "unit": "iter/sec",
            "range": "stddev: 0.001230842909760516",
            "extra": "mean: 6.005550566668489 msec\nrounds: 150"
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
        "date": 1779584670765,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2275.540343563021,
            "unit": "iter/sec",
            "range": "stddev: 0.000037634380250277",
            "extra": "mean: 439.4560627451715 usec\nrounds: 1275"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1608.198419695192,
            "unit": "iter/sec",
            "range": "stddev: 0.00008591186686366022",
            "extra": "mean: 621.8138183406086 usec\nrounds: 1145"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 830.9790641499156,
            "unit": "iter/sec",
            "range": "stddev: 0.0005653421046640469",
            "extra": "mean: 1.2033997523427276 msec\nrounds: 747"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 710.5779490014415,
            "unit": "iter/sec",
            "range": "stddev: 0.00010307257915258615",
            "extra": "mean: 1.407305139999456 msec\nrounds: 600"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1161.0881296974012,
            "unit": "iter/sec",
            "range": "stddev: 0.0019104785000442265",
            "extra": "mean: 861.2610657389259 usec\nrounds: 1293"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1090.2870018041374,
            "unit": "iter/sec",
            "range": "stddev: 0.0023863278953374033",
            "extra": "mean: 917.1896925720143 usec\nrounds: 1454"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1169.224883034942,
            "unit": "iter/sec",
            "range": "stddev: 0.0018719598793919922",
            "extra": "mean: 855.2674635219128 usec\nrounds: 1357"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1584.2263707158857,
            "unit": "iter/sec",
            "range": "stddev: 0.00004913327296994815",
            "extra": "mean: 631.2229227368034 usec\nrounds: 1359"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1493.2571764808365,
            "unit": "iter/sec",
            "range": "stddev: 0.000027502699605119752",
            "extra": "mean: 669.677009258849 usec\nrounds: 432"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1303.2757474154869,
            "unit": "iter/sec",
            "range": "stddev: 0.00004829360097990605",
            "extra": "mean: 767.2973290442103 usec\nrounds: 1088"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1893.9102868654295,
            "unit": "iter/sec",
            "range": "stddev: 0.00003697506263707876",
            "extra": "mean: 528.0081147112193 usec\nrounds: 1543"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 160.71544574966862,
            "unit": "iter/sec",
            "range": "stddev: 0.001403536943233973",
            "extra": "mean: 6.222177310559224 msec\nrounds: 161"
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
        "date": 1779584990720,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2379.1431444519308,
            "unit": "iter/sec",
            "range": "stddev: 0.000027923603502934594",
            "extra": "mean: 420.31939201807216 usec\nrounds: 1278"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1701.8325929990604,
            "unit": "iter/sec",
            "range": "stddev: 0.000060588294225251364",
            "extra": "mean: 587.6018617305634 usec\nrounds: 1121"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 847.071464030266,
            "unit": "iter/sec",
            "range": "stddev: 0.00047471386184221704",
            "extra": "mean: 1.180537938608058 msec\nrounds: 733"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 729.1150270852016,
            "unit": "iter/sec",
            "range": "stddev: 0.00017350614023679142",
            "extra": "mean: 1.3715257028753347 msec\nrounds: 626"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1187.8470045092374,
            "unit": "iter/sec",
            "range": "stddev: 0.0018407157653289093",
            "extra": "mean: 841.8592598237456 usec\nrounds: 1247"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1136.2034202037628,
            "unit": "iter/sec",
            "range": "stddev: 0.0022672647344670438",
            "extra": "mean: 880.124088889526 usec\nrounds: 1260"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1270.842937437186,
            "unit": "iter/sec",
            "range": "stddev: 0.0015008868049989363",
            "extra": "mean: 786.8792991969764 usec\nrounds: 1494"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1637.1846579915414,
            "unit": "iter/sec",
            "range": "stddev: 0.000030807718373561746",
            "extra": "mean: 610.804648772348 usec\nrounds: 1304"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1499.067256930408,
            "unit": "iter/sec",
            "range": "stddev: 0.00006191689392881886",
            "extra": "mean: 667.0814770830684 usec\nrounds: 480"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1339.5464350558511,
            "unit": "iter/sec",
            "range": "stddev: 0.000037657203303864726",
            "extra": "mean: 746.5213402313343 usec\nrounds: 1208"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1933.5486076866164,
            "unit": "iter/sec",
            "range": "stddev: 0.00003666223643265442",
            "extra": "mean: 517.1837915140103 usec\nrounds: 1909"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 179.90505955510847,
            "unit": "iter/sec",
            "range": "stddev: 0.0006275717219017595",
            "extra": "mean: 5.558487362572926 msec\nrounds: 171"
          }
        ]
      }
    ]
  }
}