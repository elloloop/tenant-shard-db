window.BENCHMARK_DATA = {
  "lastUpdate": 1779023728905,
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
      }
    ]
  }
}