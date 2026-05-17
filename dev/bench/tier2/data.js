window.BENCHMARK_DATA = {
  "lastUpdate": 1779015226913,
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
      }
    ]
  }
}