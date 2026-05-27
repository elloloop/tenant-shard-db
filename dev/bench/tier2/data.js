window.BENCHMARK_DATA = {
  "lastUpdate": 1779856385645,
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
        "date": 1779595474424,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2513.482678063081,
            "unit": "iter/sec",
            "range": "stddev: 0.000036980445303675907",
            "extra": "mean: 397.8543431899087 usec\nrounds: 1116"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1749.350768831454,
            "unit": "iter/sec",
            "range": "stddev: 0.0000612494385153406",
            "extra": "mean: 571.6406439561508 usec\nrounds: 910"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 942.9839822855294,
            "unit": "iter/sec",
            "range": "stddev: 0.000336235573855208",
            "extra": "mean: 1.0604633999999447 msec\nrounds: 640"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 739.1863799686848,
            "unit": "iter/sec",
            "range": "stddev: 0.00008471815995321449",
            "extra": "mean: 1.352838779364907 msec\nrounds: 630"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1184.1659017415213,
            "unit": "iter/sec",
            "range": "stddev: 0.0021069817660651297",
            "extra": "mean: 844.4762668215042 usec\nrounds: 1293"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1122.4175738217355,
            "unit": "iter/sec",
            "range": "stddev: 0.0025934355302932254",
            "extra": "mean: 890.9340189632687 usec\nrounds: 1582"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1226.0070321217784,
            "unit": "iter/sec",
            "range": "stddev: 0.0018409489492258765",
            "extra": "mean: 815.6560066946424 usec\nrounds: 1195"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1684.4870934378735,
            "unit": "iter/sec",
            "range": "stddev: 0.00003740946514057964",
            "extra": "mean: 593.6525152941944 usec\nrounds: 1275"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1580.8348235819253,
            "unit": "iter/sec",
            "range": "stddev: 0.000032557238893713093",
            "extra": "mean: 632.5771580196822 usec\nrounds: 424"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1355.4729978536789,
            "unit": "iter/sec",
            "range": "stddev: 0.00006525413020082131",
            "extra": "mean: 737.7498493761573 usec\nrounds: 1122"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2092.3617521051556,
            "unit": "iter/sec",
            "range": "stddev: 0.00002464485059515785",
            "extra": "mean: 477.92882803075787 usec\nrounds: 1163"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 174.10010817528197,
            "unit": "iter/sec",
            "range": "stddev: 0.0003437230794889037",
            "extra": "mean: 5.7438218188423615 msec\nrounds: 138"
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
        "date": 1779595476585,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2382.7194092377204,
            "unit": "iter/sec",
            "range": "stddev: 0.000030211830083378447",
            "extra": "mean: 419.68852737046365 usec\nrounds: 1297"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1660.89395712601,
            "unit": "iter/sec",
            "range": "stddev: 0.0000842108641432892",
            "extra": "mean: 602.0853984744381 usec\nrounds: 1049"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 801.4116773327626,
            "unit": "iter/sec",
            "range": "stddev: 0.0004960655896695111",
            "extra": "mean: 1.2477981395631443 msec\nrounds: 824"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 687.0581593085955,
            "unit": "iter/sec",
            "range": "stddev: 0.00020961632769334718",
            "extra": "mean: 1.4554808591551056 msec\nrounds: 497"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1182.777446186677,
            "unit": "iter/sec",
            "range": "stddev: 0.0018496244335357542",
            "extra": "mean: 845.467592592369 usec\nrounds: 1215"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1074.181707998646,
            "unit": "iter/sec",
            "range": "stddev: 0.0025118595758716784",
            "extra": "mean: 930.941192308276 usec\nrounds: 1274"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1229.8485085491907,
            "unit": "iter/sec",
            "range": "stddev: 0.0016811743445453316",
            "extra": "mean: 813.1082755710011 usec\nrounds: 1183"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1613.482635460702,
            "unit": "iter/sec",
            "range": "stddev: 0.000039717609284497325",
            "extra": "mean: 619.7773549105889 usec\nrounds: 1344"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1419.2175292772426,
            "unit": "iter/sec",
            "range": "stddev: 0.00007527697363204375",
            "extra": "mean: 704.6136193859335 usec\nrounds: 423"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1263.4507038221486,
            "unit": "iter/sec",
            "range": "stddev: 0.00005236668219276953",
            "extra": "mean: 791.4831951692563 usec\nrounds: 1035"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1941.1487282423473,
            "unit": "iter/sec",
            "range": "stddev: 0.00002855770045147275",
            "extra": "mean: 515.1588775505473 usec\nrounds: 1029"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 175.69143733300152,
            "unit": "iter/sec",
            "range": "stddev: 0.001188030468333051",
            "extra": "mean: 5.69179702312198 msec\nrounds: 173"
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
        "date": 1779599124186,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2521.2072948619916,
            "unit": "iter/sec",
            "range": "stddev: 0.000027701472244147465",
            "extra": "mean: 396.6353746627324 usec\nrounds: 1113"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1769.1871169941614,
            "unit": "iter/sec",
            "range": "stddev: 0.00004085071935139802",
            "extra": "mean: 565.2313372589973 usec\nrounds: 934"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 938.600510308596,
            "unit": "iter/sec",
            "range": "stddev: 0.00031490073081641347",
            "extra": "mean: 1.0654159986246088 msec\nrounds: 727"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 722.8461207518942,
            "unit": "iter/sec",
            "range": "stddev: 0.00019572748603285167",
            "extra": "mean: 1.3834203038397914 msec\nrounds: 599"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1183.0972471620441,
            "unit": "iter/sec",
            "range": "stddev: 0.002085975398248153",
            "extra": "mean: 845.2390557063261 usec\nrounds: 1472"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1092.4904254532478,
            "unit": "iter/sec",
            "range": "stddev: 0.002647967089424473",
            "extra": "mean: 915.3398297153261 usec\nrounds: 1474"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1213.0248993710652,
            "unit": "iter/sec",
            "range": "stddev: 0.001993995315174941",
            "extra": "mean: 824.3853860860437 usec\nrounds: 1308"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1689.531226657556,
            "unit": "iter/sec",
            "range": "stddev: 0.00003144115372692916",
            "extra": "mean: 591.8801524481594 usec\nrounds: 1266"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1546.551918088635,
            "unit": "iter/sec",
            "range": "stddev: 0.00007501993074689683",
            "extra": "mean: 646.5996959454733 usec\nrounds: 444"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1362.0428932413743,
            "unit": "iter/sec",
            "range": "stddev: 0.000043134954924293086",
            "extra": "mean: 734.1912688375116 usec\nrounds: 1075"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2089.3434143107365,
            "unit": "iter/sec",
            "range": "stddev: 0.000029400187393550587",
            "extra": "mean: 478.6192605536294 usec\nrounds: 1516"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 163.541342763623,
            "unit": "iter/sec",
            "range": "stddev: 0.0005637059869421035",
            "extra": "mean: 6.114661791944349 msec\nrounds: 149"
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
        "date": 1779599139388,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2500.6132399287276,
            "unit": "iter/sec",
            "range": "stddev: 0.00002679421413933781",
            "extra": "mean: 399.9019056735467 usec\nrounds: 1410"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1749.788779285641,
            "unit": "iter/sec",
            "range": "stddev: 0.00004887595901449243",
            "extra": "mean: 571.4975497832683 usec\nrounds: 1155"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 850.9247579036396,
            "unit": "iter/sec",
            "range": "stddev: 0.0005473963912610593",
            "extra": "mean: 1.1751920375000324 msec\nrounds: 800"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 737.9722735411231,
            "unit": "iter/sec",
            "range": "stddev: 0.00014600877555057533",
            "extra": "mean: 1.355064459537958 msec\nrounds: 692"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1222.401224772287,
            "unit": "iter/sec",
            "range": "stddev: 0.0019057521731253271",
            "extra": "mean: 818.062007575527 usec\nrounds: 1320"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1128.5885853619006,
            "unit": "iter/sec",
            "range": "stddev: 0.0023720157545440977",
            "extra": "mean: 886.0624792508719 usec\nrounds: 1494"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1276.8609828141596,
            "unit": "iter/sec",
            "range": "stddev: 0.0015626487097775935",
            "extra": "mean: 783.1706140758039 usec\nrounds: 1293"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1620.7113735942062,
            "unit": "iter/sec",
            "range": "stddev: 0.00008027856444696423",
            "extra": "mean: 617.0130081720399 usec\nrounds: 1346"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1568.3511199621214,
            "unit": "iter/sec",
            "range": "stddev: 0.00003861724605126989",
            "extra": "mean: 637.6123224397302 usec\nrounds: 459"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1355.9226303604826,
            "unit": "iter/sec",
            "range": "stddev: 0.00004636213353511103",
            "extra": "mean: 737.5052068672547 usec\nrounds: 1194"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2018.5657903685715,
            "unit": "iter/sec",
            "range": "stddev: 0.000027607932510387307",
            "extra": "mean: 495.4012421945431 usec\nrounds: 1057"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 170.60222533218214,
            "unit": "iter/sec",
            "range": "stddev: 0.0006772668060863908",
            "extra": "mean: 5.8615882533354124 msec\nrounds: 150"
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
        "date": 1779603107870,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2523.3841909379375,
            "unit": "iter/sec",
            "range": "stddev: 0.00003048885124884845",
            "extra": "mean: 396.29320164215727 usec\nrounds: 1096"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1750.153167540668,
            "unit": "iter/sec",
            "range": "stddev: 0.000054677300397363694",
            "extra": "mean: 571.3785619147892 usec\nrounds: 961"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 910.4834337691345,
            "unit": "iter/sec",
            "range": "stddev: 0.00025634923221670383",
            "extra": "mean: 1.0983176221672624 msec\nrounds: 794"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 709.1713539062915,
            "unit": "iter/sec",
            "range": "stddev: 0.00014753901867423414",
            "extra": "mean: 1.4100964378943852 msec\nrounds: 475"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1188.058957716716,
            "unit": "iter/sec",
            "range": "stddev: 0.002011229168739784",
            "extra": "mean: 841.7090696591866 usec\nrounds: 1292"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1122.9795646042992,
            "unit": "iter/sec",
            "range": "stddev: 0.002603793154895419",
            "extra": "mean: 890.4881544771181 usec\nrounds: 1463"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1226.1272620903792,
            "unit": "iter/sec",
            "range": "stddev: 0.0019137906519121593",
            "extra": "mean: 815.5760261745888 usec\nrounds: 1490"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1662.5504752347988,
            "unit": "iter/sec",
            "range": "stddev: 0.00026178726756184175",
            "extra": "mean: 601.4854976711441 usec\nrounds: 1288"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1426.3157595868017,
            "unit": "iter/sec",
            "range": "stddev: 0.00007231334465315408",
            "extra": "mean: 701.1070257610393 usec\nrounds: 427"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1268.0779147741193,
            "unit": "iter/sec",
            "range": "stddev: 0.000051771883252046076",
            "extra": "mean: 788.5950763349807 usec\nrounds: 1048"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2087.943356543695,
            "unit": "iter/sec",
            "range": "stddev: 0.0000314689449034617",
            "extra": "mean: 478.9401957988763 usec\nrounds: 1190"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 169.66697617459036,
            "unit": "iter/sec",
            "range": "stddev: 0.0006000952767164544",
            "extra": "mean: 5.893898874999588 msec\nrounds: 136"
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
        "date": 1779603124588,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2481.2007467153903,
            "unit": "iter/sec",
            "range": "stddev: 0.00010025765157735304",
            "extra": "mean: 403.03067026067856 usec\nrounds: 1113"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1734.5168591817464,
            "unit": "iter/sec",
            "range": "stddev: 0.0000747161301263396",
            "extra": "mean: 576.5294206893712 usec\nrounds: 1015"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 949.8742529100151,
            "unit": "iter/sec",
            "range": "stddev: 0.0002924809809235475",
            "extra": "mean: 1.052770929348196 msec\nrounds: 920"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 720.1651871264509,
            "unit": "iter/sec",
            "range": "stddev: 0.00010171701519411544",
            "extra": "mean: 1.3885703139721666 msec\nrounds: 637"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1193.1584226206598,
            "unit": "iter/sec",
            "range": "stddev: 0.0020273228704373093",
            "extra": "mean: 838.1116715444999 usec\nrounds: 1230"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1079.3403093617644,
            "unit": "iter/sec",
            "range": "stddev: 0.0026785880856696324",
            "extra": "mean: 926.4918499998578 usec\nrounds: 1240"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1249.124908536873,
            "unit": "iter/sec",
            "range": "stddev: 0.0018084397968369367",
            "extra": "mean: 800.560450893035 usec\nrounds: 896"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1666.9252045884002,
            "unit": "iter/sec",
            "range": "stddev: 0.00012739365425357734",
            "extra": "mean: 599.9069407837778 usec\nrounds: 1199"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1583.4451476192105,
            "unit": "iter/sec",
            "range": "stddev: 0.00003429751766381377",
            "extra": "mean: 631.5343486975538 usec\nrounds: 499"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1343.9854425256544,
            "unit": "iter/sec",
            "range": "stddev: 0.0000565246438930003",
            "extra": "mean: 744.0556782525654 usec\nrounds: 1122"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2074.435682386578,
            "unit": "iter/sec",
            "range": "stddev: 0.000025464403956646517",
            "extra": "mean: 482.0588117003122 usec\nrounds: 1094"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 184.55006798485397,
            "unit": "iter/sec",
            "range": "stddev: 0.00015180935463914442",
            "extra": "mean: 5.418583753012055 msec\nrounds: 166"
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
        "date": 1779604324542,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2502.8452939745994,
            "unit": "iter/sec",
            "range": "stddev: 0.000025672279294919353",
            "extra": "mean: 399.545270499707 usec\nrounds: 1061"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1748.49501781286,
            "unit": "iter/sec",
            "range": "stddev: 0.000048351187482650436",
            "extra": "mean: 571.9204171658837 usec\nrounds: 1002"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 941.3176308214083,
            "unit": "iter/sec",
            "range": "stddev: 0.0003277839793845541",
            "extra": "mean: 1.0623406672276865 msec\nrounds: 595"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 436.6325739409206,
            "unit": "iter/sec",
            "range": "stddev: 0.000516363425627629",
            "extra": "mean: 2.2902551474212887 msec\nrounds: 407"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1172.18163702083,
            "unit": "iter/sec",
            "range": "stddev: 0.0021358565047905082",
            "extra": "mean: 853.1101054795229 usec\nrounds: 1460"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1099.4315122436385,
            "unit": "iter/sec",
            "range": "stddev: 0.0026812711298341975",
            "extra": "mean: 909.5609766171556 usec\nrounds: 1283"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1203.745283338307,
            "unit": "iter/sec",
            "range": "stddev: 0.0019854493115331485",
            "extra": "mean: 830.7405344316143 usec\nrounds: 1336"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1667.9721640234707,
            "unit": "iter/sec",
            "range": "stddev: 0.00018378388091250426",
            "extra": "mean: 599.5303887972609 usec\nrounds: 1214"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1561.7514693610237,
            "unit": "iter/sec",
            "range": "stddev: 0.000046546801044114596",
            "extra": "mean: 640.3067450989118 usec\nrounds: 459"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1355.482794822065,
            "unit": "iter/sec",
            "range": "stddev: 0.00007093969362435073",
            "extra": "mean: 737.7445171712936 usec\nrounds: 990"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2058.7967345080247,
            "unit": "iter/sec",
            "range": "stddev: 0.00003889451104286935",
            "extra": "mean: 485.7206072065014 usec\nrounds: 1138"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 154.5614039952464,
            "unit": "iter/sec",
            "range": "stddev: 0.0005194848238562469",
            "extra": "mean: 6.4699205244716556 msec\nrounds: 143"
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
        "date": 1779604336980,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2508.2318044326576,
            "unit": "iter/sec",
            "range": "stddev: 0.00002889392085307857",
            "extra": "mean: 398.68723386441235 usec\nrounds: 1069"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1741.5534174613001,
            "unit": "iter/sec",
            "range": "stddev: 0.00007117532922828599",
            "extra": "mean: 574.2000159017354 usec\nrounds: 1069"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 943.6998519623681,
            "unit": "iter/sec",
            "range": "stddev: 0.000362991121420536",
            "extra": "mean: 1.0596589560977032 msec\nrounds: 820"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 444.8531927225502,
            "unit": "iter/sec",
            "range": "stddev: 0.0004746453330778619",
            "extra": "mean: 2.247932613183892 msec\nrounds: 349"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1175.0808260553108,
            "unit": "iter/sec",
            "range": "stddev: 0.0020860455184585156",
            "extra": "mean: 851.0052907228105 usec\nrounds: 1455"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1118.3784443779148,
            "unit": "iter/sec",
            "range": "stddev: 0.0026266300581280883",
            "extra": "mean: 894.1517113701512 usec\nrounds: 1029"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1244.231472731152,
            "unit": "iter/sec",
            "range": "stddev: 0.001786397504032012",
            "extra": "mean: 803.7089737048273 usec\nrounds: 1331"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1682.4554633102123,
            "unit": "iter/sec",
            "range": "stddev: 0.00003347650999466921",
            "extra": "mean: 594.3693736965322 usec\nrounds: 1247"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1486.9246568940375,
            "unit": "iter/sec",
            "range": "stddev: 0.000040418263184856875",
            "extra": "mean: 672.5290318938216 usec\nrounds: 533"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1276.1833222393154,
            "unit": "iter/sec",
            "range": "stddev: 0.00006228382459072316",
            "extra": "mean: 783.5864821092495 usec\nrounds: 1062"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2078.36839735716,
            "unit": "iter/sec",
            "range": "stddev: 0.000028398856154746865",
            "extra": "mean: 481.1466539192924 usec\nrounds: 1569"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 166.1813934916995,
            "unit": "iter/sec",
            "range": "stddev: 0.00039843246944336493",
            "extra": "mean: 6.017520848686038 msec\nrounds: 152"
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
        "date": 1779619992897,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3297.619661269688,
            "unit": "iter/sec",
            "range": "stddev: 0.000027954604861788897",
            "extra": "mean: 303.24904104161254 usec\nrounds: 1267"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2296.5231788312026,
            "unit": "iter/sec",
            "range": "stddev: 0.00002861494999643714",
            "extra": "mean: 435.44084780757237 usec\nrounds: 1163"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1224.5148791562067,
            "unit": "iter/sec",
            "range": "stddev: 0.0001384920623265785",
            "extra": "mean: 816.6499378832242 usec\nrounds: 1143"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 579.0850087038314,
            "unit": "iter/sec",
            "range": "stddev: 0.00024518204165264833",
            "extra": "mean: 1.726862179075063 msec\nrounds: 497"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2035.677936537836,
            "unit": "iter/sec",
            "range": "stddev: 0.0007973788336247332",
            "extra": "mean: 491.23684157069687 usec\nrounds: 1477"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1972.223879522451,
            "unit": "iter/sec",
            "range": "stddev: 0.0009348543119098",
            "extra": "mean: 507.0418274431082 usec\nrounds: 1924"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1935.2016561406626,
            "unit": "iter/sec",
            "range": "stddev: 0.0010576860582732159",
            "extra": "mean: 516.7420133332677 usec\nrounds: 2025"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 943.8605699088241,
            "unit": "iter/sec",
            "range": "stddev: 0.0030114506322989737",
            "extra": "mean: 1.059478520324881 msec\nrounds: 123"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 953.6812549158475,
            "unit": "iter/sec",
            "range": "stddev: 0.0024735831809046594",
            "extra": "mean: 1.0485683710835227 msec\nrounds: 415"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1209.0338173536572,
            "unit": "iter/sec",
            "range": "stddev: 0.0011827183250573514",
            "extra": "mean: 827.1067241020668 usec\nrounds: 946"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2688.19600848649,
            "unit": "iter/sec",
            "range": "stddev: 0.000034023022047542214",
            "extra": "mean: 371.99668359117186 usec\nrounds: 1359"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 173.0929401781961,
            "unit": "iter/sec",
            "range": "stddev: 0.00036447180867332297",
            "extra": "mean: 5.77724313291182 msec\nrounds: 158"
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
        "date": 1779620003951,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2316.9744360948685,
            "unit": "iter/sec",
            "range": "stddev: 0.00009252302031227516",
            "extra": "mean: 431.59733850384833 usec\nrounds: 1096"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1657.6540377853094,
            "unit": "iter/sec",
            "range": "stddev: 0.00007645831080213445",
            "extra": "mean: 603.2621869253485 usec\nrounds: 979"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 799.0172730593594,
            "unit": "iter/sec",
            "range": "stddev: 0.0006337795650032195",
            "extra": "mean: 1.2515373993995116 msec\nrounds: 666"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 425.7704062493843,
            "unit": "iter/sec",
            "range": "stddev: 0.0007489079565506192",
            "extra": "mean: 2.348683669231523 msec\nrounds: 390"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1145.1354937947212,
            "unit": "iter/sec",
            "range": "stddev: 0.0020460032249455615",
            "extra": "mean: 873.2591081307112 usec\nrounds: 1193"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1076.9163491410445,
            "unit": "iter/sec",
            "range": "stddev: 0.002530396126643851",
            "extra": "mean: 928.577229603401 usec\nrounds: 1189"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1188.359900663728,
            "unit": "iter/sec",
            "range": "stddev: 0.001831012538029306",
            "extra": "mean: 841.4959133520709 usec\nrounds: 1408"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1604.3018810321403,
            "unit": "iter/sec",
            "range": "stddev: 0.00003975333291868212",
            "extra": "mean: 623.3240837171131 usec\nrounds: 1302"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1469.6266262259996,
            "unit": "iter/sec",
            "range": "stddev: 0.0000711391897853252",
            "extra": "mean: 680.4449389760986 usec\nrounds: 508"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1285.3941900980146,
            "unit": "iter/sec",
            "range": "stddev: 0.000052298820992066634",
            "extra": "mean: 777.9714640874076 usec\nrounds: 1086"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1936.1428437643099,
            "unit": "iter/sec",
            "range": "stddev: 0.000030700132750071455",
            "extra": "mean: 516.4908174108521 usec\nrounds: 942"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 158.86075843051017,
            "unit": "iter/sec",
            "range": "stddev: 0.0009969180194838595",
            "extra": "mean: 6.294820759258971 msec\nrounds: 162"
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
        "date": 1779621145984,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2516.338933427429,
            "unit": "iter/sec",
            "range": "stddev: 0.00003463033892961068",
            "extra": "mean: 397.40274520091396 usec\nrounds: 1146"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1740.7674389985407,
            "unit": "iter/sec",
            "range": "stddev: 0.00006346213969421581",
            "extra": "mean: 574.4592744538567 usec\nrounds: 1053"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 948.0808729900747,
            "unit": "iter/sec",
            "range": "stddev: 0.00030550233651394874",
            "extra": "mean: 1.0547623398900368 msec\nrounds: 915"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 444.4954809215713,
            "unit": "iter/sec",
            "range": "stddev: 0.0004300239875379293",
            "extra": "mean: 2.24974165750055 msec\nrounds: 400"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1190.3536306467627,
            "unit": "iter/sec",
            "range": "stddev: 0.002080491844816251",
            "extra": "mean: 840.0864871195154 usec\nrounds: 1281"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1103.2447551291382,
            "unit": "iter/sec",
            "range": "stddev: 0.0026239203080118227",
            "extra": "mean: 906.4171801867727 usec\nrounds: 1393"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1208.4145231314697,
            "unit": "iter/sec",
            "range": "stddev: 0.001994443304378497",
            "extra": "mean: 827.5306038267506 usec\nrounds: 1411"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1672.6462975991344,
            "unit": "iter/sec",
            "range": "stddev: 0.000040365894389047026",
            "extra": "mean: 597.8550285469017 usec\nrounds: 1191"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1581.2149130442308,
            "unit": "iter/sec",
            "range": "stddev: 0.000039732522515882875",
            "extra": "mean: 632.4251003140059 usec\nrounds: 319"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1364.873947041745,
            "unit": "iter/sec",
            "range": "stddev: 0.000044860879283703655",
            "extra": "mean: 732.6683919547443 usec\nrounds: 1069"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2085.553323275175,
            "unit": "iter/sec",
            "range": "stddev: 0.000026166019639821128",
            "extra": "mean: 479.48905877390337 usec\nrounds: 1191"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 158.2997134893915,
            "unit": "iter/sec",
            "range": "stddev: 0.0004658583019513226",
            "extra": "mean: 6.317130827068839 msec\nrounds: 133"
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
        "date": 1779621150204,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2362.728451212938,
            "unit": "iter/sec",
            "range": "stddev: 0.00003249620893342698",
            "extra": "mean: 423.2394964756262 usec\nrounds: 1277"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1671.5410050028254,
            "unit": "iter/sec",
            "range": "stddev: 0.00005743792162747955",
            "extra": "mean: 598.2503552153719 usec\nrounds: 1112"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 811.2876932504253,
            "unit": "iter/sec",
            "range": "stddev: 0.000528242224116381",
            "extra": "mean: 1.232608368547412 msec\nrounds: 833"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 416.04327278072736,
            "unit": "iter/sec",
            "range": "stddev: 0.0006009980348604332",
            "extra": "mean: 2.403596129114778 msec\nrounds: 395"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1179.6804106719405,
            "unit": "iter/sec",
            "range": "stddev: 0.001957190696590741",
            "extra": "mean: 847.6872133787527 usec\nrounds: 1181"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1106.0731981878537,
            "unit": "iter/sec",
            "range": "stddev: 0.002427021989402498",
            "extra": "mean: 904.0992961752986 usec\nrounds: 1229"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1180.1203676109305,
            "unit": "iter/sec",
            "range": "stddev: 0.0019163996566802066",
            "extra": "mean: 847.3711897917909 usec\nrounds: 1391"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1589.9503433770403,
            "unit": "iter/sec",
            "range": "stddev: 0.0000505848183154465",
            "extra": "mean: 628.9504600980235 usec\nrounds: 1228"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1422.2603455894373,
            "unit": "iter/sec",
            "range": "stddev: 0.000042506814851739614",
            "extra": "mean: 703.1061528932405 usec\nrounds: 484"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1236.2050452611086,
            "unit": "iter/sec",
            "range": "stddev: 0.000050653741547369114",
            "extra": "mean: 808.9272923075492 usec\nrounds: 1105"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1952.9155695878485,
            "unit": "iter/sec",
            "range": "stddev: 0.00002532223362158897",
            "extra": "mean: 512.0549068135313 usec\nrounds: 998"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 164.15532056935265,
            "unit": "iter/sec",
            "range": "stddev: 0.0009358472402917298",
            "extra": "mean: 6.091791582092023 msec\nrounds: 134"
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
        "date": 1779622323406,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2926.5295240520263,
            "unit": "iter/sec",
            "range": "stddev: 0.00003142222566920035",
            "extra": "mean: 341.7016612275334 usec\nrounds: 1287"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1983.763342654083,
            "unit": "iter/sec",
            "range": "stddev: 0.00005289885949039742",
            "extra": "mean: 504.09238768476126 usec\nrounds: 1153"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1020.2818277973956,
            "unit": "iter/sec",
            "range": "stddev: 0.00010319127288007655",
            "extra": "mean: 980.1213476072778 usec\nrounds: 794"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 470.5690544066356,
            "unit": "iter/sec",
            "range": "stddev: 0.00014224840190691328",
            "extra": "mean: 2.125086617225501 msec\nrounds: 418"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1421.100312579671,
            "unit": "iter/sec",
            "range": "stddev: 0.0012647588306672211",
            "extra": "mean: 703.6800929166899 usec\nrounds: 1313"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1270.853521907425,
            "unit": "iter/sec",
            "range": "stddev: 0.0019541501528468888",
            "extra": "mean: 786.8727455695283 usec\nrounds: 1580"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1458.9291900376472,
            "unit": "iter/sec",
            "range": "stddev: 0.0011601006103480226",
            "extra": "mean: 685.4342258887802 usec\nrounds: 1576"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1947.8534419398432,
            "unit": "iter/sec",
            "range": "stddev: 0.000040939340584338336",
            "extra": "mean: 513.3856472302723 usec\nrounds: 1372"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1823.7760412015273,
            "unit": "iter/sec",
            "range": "stddev: 0.00005039098070990676",
            "extra": "mean: 548.3129383261264 usec\nrounds: 454"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1487.010467500338,
            "unit": "iter/sec",
            "range": "stddev: 0.00007611175380630659",
            "extra": "mean: 672.4902223997105 usec\nrounds: 1250"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2491.77430193641,
            "unit": "iter/sec",
            "range": "stddev: 0.00003319868853409393",
            "extra": "mean: 401.3204563603048 usec\nrounds: 1352"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 159.28475788409727,
            "unit": "iter/sec",
            "range": "stddev: 0.0004374108711996326",
            "extra": "mean: 6.278064601307583 msec\nrounds: 153"
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
        "date": 1779622336799,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2547.1541545401774,
            "unit": "iter/sec",
            "range": "stddev: 0.000029369267448613884",
            "extra": "mean: 392.5950057704788 usec\nrounds: 1213"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1754.9343776159374,
            "unit": "iter/sec",
            "range": "stddev: 0.00005661957230455674",
            "extra": "mean: 569.8218763931737 usec\nrounds: 987"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 963.7804845585298,
            "unit": "iter/sec",
            "range": "stddev: 0.0002453498895736353",
            "extra": "mean: 1.0375806690649696 msec\nrounds: 834"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 448.34592669078535,
            "unit": "iter/sec",
            "range": "stddev: 0.0004313976236102274",
            "extra": "mean: 2.230420620481467 msec\nrounds: 332"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1155.6111161066226,
            "unit": "iter/sec",
            "range": "stddev: 0.002274116606777991",
            "extra": "mean: 865.3430086144438 usec\nrounds: 1393"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1129.2990219346907,
            "unit": "iter/sec",
            "range": "stddev: 0.0025743161568795893",
            "extra": "mean: 885.5050616149667 usec\nrounds: 1412"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1214.6752499575384,
            "unit": "iter/sec",
            "range": "stddev: 0.0019506380936827991",
            "extra": "mean: 823.2653131237811 usec\nrounds: 1303"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1693.7295394985863,
            "unit": "iter/sec",
            "range": "stddev: 0.00003186189990692154",
            "extra": "mean: 590.4130362490113 usec\nrounds: 1269"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1546.1759983737174,
            "unit": "iter/sec",
            "range": "stddev: 0.00006108797615019723",
            "extra": "mean: 646.7569028699253 usec\nrounds: 453"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1365.0739339250947,
            "unit": "iter/sec",
            "range": "stddev: 0.00004534003965297963",
            "extra": "mean: 732.561054128862 usec\nrounds: 1090"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2097.5084747358173,
            "unit": "iter/sec",
            "range": "stddev: 0.000035310811389832254",
            "extra": "mean: 476.75611900731445 usec\nrounds: 1168"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 151.7406287344721,
            "unit": "iter/sec",
            "range": "stddev: 0.000493976371408077",
            "extra": "mean: 6.590192806897354 msec\nrounds: 145"
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
        "date": 1779653466299,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2490.823521109546,
            "unit": "iter/sec",
            "range": "stddev: 0.00003720784211981792",
            "extra": "mean: 401.4736457742082 usec\nrounds: 1053"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1761.673048018167,
            "unit": "iter/sec",
            "range": "stddev: 0.00003661799118602947",
            "extra": "mean: 567.6422200617601 usec\nrounds: 977"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 936.6087071608828,
            "unit": "iter/sec",
            "range": "stddev: 0.0003435885933379907",
            "extra": "mean: 1.0676817248808987 msec\nrounds: 836"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 443.2875686304043,
            "unit": "iter/sec",
            "range": "stddev: 0.00042956363659158223",
            "extra": "mean: 2.2558719683695 msec\nrounds: 411"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1190.3596849556739,
            "unit": "iter/sec",
            "range": "stddev: 0.001983693365117222",
            "extra": "mean: 840.0822143411532 usec\nrounds: 1297"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1098.8452428497128,
            "unit": "iter/sec",
            "range": "stddev: 0.002684067035322346",
            "extra": "mean: 910.0462567473375 usec\nrounds: 1445"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1208.7376012975747,
            "unit": "iter/sec",
            "range": "stddev: 0.001499530591574377",
            "extra": "mean: 827.3094168051895 usec\nrounds: 1202"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1548.0893625739056,
            "unit": "iter/sec",
            "range": "stddev: 0.0003686226123455339",
            "extra": "mean: 645.9575423587733 usec\nrounds: 1204"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1419.28332400226,
            "unit": "iter/sec",
            "range": "stddev: 0.000056002577276178815",
            "extra": "mean: 704.5809551119671 usec\nrounds: 401"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1197.9996654862912,
            "unit": "iter/sec",
            "range": "stddev: 0.000053226296977467617",
            "extra": "mean: 834.7247739790317 usec\nrounds: 1053"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2088.486874103192,
            "unit": "iter/sec",
            "range": "stddev: 0.000026664721919226226",
            "extra": "mean: 478.815554169765 usec\nrounds: 1283"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 156.95330511091632,
            "unit": "iter/sec",
            "range": "stddev: 0.0004477482258860363",
            "extra": "mean: 6.371321708028489 msec\nrounds: 137"
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
        "date": 1779653473387,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2511.4970310762983,
            "unit": "iter/sec",
            "range": "stddev: 0.00003728280074597398",
            "extra": "mean: 398.168895931942 usec\nrounds: 1057"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1739.4477007825483,
            "unit": "iter/sec",
            "range": "stddev: 0.00005857056909750645",
            "extra": "mean: 574.8951230612548 usec\nrounds: 1032"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 942.9033839173769,
            "unit": "iter/sec",
            "range": "stddev: 0.0003436486788885861",
            "extra": "mean: 1.0605540472719592 msec\nrounds: 825"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 472.91072877114556,
            "unit": "iter/sec",
            "range": "stddev: 0.0003316677747486483",
            "extra": "mean: 2.1145639951931128 msec\nrounds: 416"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1180.7031046270674,
            "unit": "iter/sec",
            "range": "stddev: 0.002018774485631243",
            "extra": "mean: 846.9529690242123 usec\nrounds: 1485"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1118.746930193947,
            "unit": "iter/sec",
            "range": "stddev: 0.002549949693239074",
            "extra": "mean: 893.8572013123995 usec\nrounds: 1371"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1192.127641704511,
            "unit": "iter/sec",
            "range": "stddev: 0.0020639725112036795",
            "extra": "mean: 838.8363502503761 usec\nrounds: 1399"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1653.4025862339024,
            "unit": "iter/sec",
            "range": "stddev: 0.000053350249833318186",
            "extra": "mean: 604.8133759593217 usec\nrounds: 1173"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1579.7084121805951,
            "unit": "iter/sec",
            "range": "stddev: 0.0000366619745625309",
            "extra": "mean: 633.0282172895578 usec\nrounds: 428"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1358.1760979387777,
            "unit": "iter/sec",
            "range": "stddev: 0.000051362952079932376",
            "extra": "mean: 736.2815481126784 usec\nrounds: 1060"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2081.5451960120618,
            "unit": "iter/sec",
            "range": "stddev: 0.00002579326985818334",
            "extra": "mean: 480.41234075332824 usec\nrounds: 1168"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 157.35557257790632,
            "unit": "iter/sec",
            "range": "stddev: 0.0004966067264384901",
            "extra": "mean: 6.355033912160326 msec\nrounds: 148"
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
        "date": 1779654676354,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2314.4404200402405,
            "unit": "iter/sec",
            "range": "stddev: 0.000029098756676499624",
            "extra": "mean: 432.0698823530801 usec\nrounds: 1241"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1632.6529434387805,
            "unit": "iter/sec",
            "range": "stddev: 0.00007108474354868455",
            "extra": "mean: 612.5000441880482 usec\nrounds: 1041"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 790.8689161268162,
            "unit": "iter/sec",
            "range": "stddev: 0.0006156499686407095",
            "extra": "mean: 1.2644320438049048 msec\nrounds: 799"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 376.4743712861101,
            "unit": "iter/sec",
            "range": "stddev: 0.0008298187012748525",
            "extra": "mean: 2.656223308332528 msec\nrounds: 360"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1159.1817892228098,
            "unit": "iter/sec",
            "range": "stddev: 0.0019453194322026567",
            "extra": "mean: 862.6774586154123 usec\nrounds: 1184"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1055.9367491454425,
            "unit": "iter/sec",
            "range": "stddev: 0.0024698832199109865",
            "extra": "mean: 947.0264206727236 usec\nrounds: 1248"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1182.0910234715323,
            "unit": "iter/sec",
            "range": "stddev: 0.001791752975379686",
            "extra": "mean: 845.9585430766809 usec\nrounds: 1300"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1560.1797827261005,
            "unit": "iter/sec",
            "range": "stddev: 0.000049318480284769",
            "extra": "mean: 640.9517743222522 usec\nrounds: 1254"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1413.9000613609026,
            "unit": "iter/sec",
            "range": "stddev: 0.000054823267155001483",
            "extra": "mean: 707.2635664485956 usec\nrounds: 459"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1224.4346629769857,
            "unit": "iter/sec",
            "range": "stddev: 0.0000512617209035533",
            "extra": "mean: 816.703438931307 usec\nrounds: 1048"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1890.0686097739938,
            "unit": "iter/sec",
            "range": "stddev: 0.000031352037521359894",
            "extra": "mean: 529.0813226719721 usec\nrounds: 1063"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 161.63725617979003,
            "unit": "iter/sec",
            "range": "stddev: 0.0005671406617031396",
            "extra": "mean: 6.1866924967328965 msec\nrounds: 153"
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
        "date": 1779654692541,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2340.7143265312416,
            "unit": "iter/sec",
            "range": "stddev: 0.00003199471108839927",
            "extra": "mean: 427.2200108596434 usec\nrounds: 1105"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1653.66454018288,
            "unit": "iter/sec",
            "range": "stddev: 0.00005979948942237345",
            "extra": "mean: 604.7175685882514 usec\nrounds: 1006"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 783.8438898346462,
            "unit": "iter/sec",
            "range": "stddev: 0.0007553438855908753",
            "extra": "mean: 1.2757642343948774 msec\nrounds: 785"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 416.43746063901347,
            "unit": "iter/sec",
            "range": "stddev: 0.000596637365666262",
            "extra": "mean: 2.4013209533684208 msec\nrounds: 386"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1151.8659966512878,
            "unit": "iter/sec",
            "range": "stddev: 0.001933349829721744",
            "extra": "mean: 868.156541565778 usec\nrounds: 1239"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1077.0312965827318,
            "unit": "iter/sec",
            "range": "stddev: 0.0025389763383202204",
            "extra": "mean: 928.4781260979684 usec\nrounds: 1594"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1225.636146108588,
            "unit": "iter/sec",
            "range": "stddev: 0.0016316375007565404",
            "extra": "mean: 815.9028298692185 usec\nrounds: 1299"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1544.134789209782,
            "unit": "iter/sec",
            "range": "stddev: 0.0005611703227318363",
            "extra": "mean: 647.6118581019437 usec\nrounds: 1191"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1445.4064550957696,
            "unit": "iter/sec",
            "range": "stddev: 0.000026873008757496252",
            "extra": "mean: 691.846917159189 usec\nrounds: 338"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1239.671587230071,
            "unit": "iter/sec",
            "range": "stddev: 0.00005517836886911596",
            "extra": "mean: 806.6652573964411 usec\nrounds: 1014"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1921.4846844649246,
            "unit": "iter/sec",
            "range": "stddev: 0.00003174334302731099",
            "extra": "mean: 520.4308980888233 usec\nrounds: 942"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 149.42744086364863,
            "unit": "iter/sec",
            "range": "stddev: 0.0005891342927162455",
            "extra": "mean: 6.692211244603272 msec\nrounds: 139"
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
        "date": 1779655285233,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2201.1140205528714,
            "unit": "iter/sec",
            "range": "stddev: 0.00005712601987489141",
            "extra": "mean: 454.315401502382 usec\nrounds: 1198"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1615.0140270423287,
            "unit": "iter/sec",
            "range": "stddev: 0.00006301704747458898",
            "extra": "mean: 619.1896684831645 usec\nrounds: 917"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 759.8653383274902,
            "unit": "iter/sec",
            "range": "stddev: 0.0007370908159709704",
            "extra": "mean: 1.3160226550155067 msec\nrounds: 658"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 400.6860529454177,
            "unit": "iter/sec",
            "range": "stddev: 0.0006341614960764067",
            "extra": "mean: 2.495719510696875 msec\nrounds: 374"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1093.3775433882458,
            "unit": "iter/sec",
            "range": "stddev: 0.00211797251295614",
            "extra": "mean: 914.5971636669251 usec\nrounds: 1167"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1048.2373744542058,
            "unit": "iter/sec",
            "range": "stddev: 0.0025408574846385",
            "extra": "mean: 953.9823940361581 usec\nrounds: 1241"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1143.1376778000554,
            "unit": "iter/sec",
            "range": "stddev: 0.0019563975156664027",
            "extra": "mean: 874.785268144148 usec\nrounds: 1309"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1537.2908103531765,
            "unit": "iter/sec",
            "range": "stddev: 0.00017788646718907875",
            "extra": "mean: 650.4950093146399 usec\nrounds: 1181"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1398.827061986308,
            "unit": "iter/sec",
            "range": "stddev: 0.000041084251905541086",
            "extra": "mean: 714.884653847073 usec\nrounds: 468"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1211.8823574178202,
            "unit": "iter/sec",
            "range": "stddev: 0.000052638343112813755",
            "extra": "mean: 825.1626025241577 usec\nrounds: 951"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1892.6328549185084,
            "unit": "iter/sec",
            "range": "stddev: 0.00003277689772633479",
            "extra": "mean: 528.364493621272 usec\nrounds: 1019"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 149.8087605985629,
            "unit": "iter/sec",
            "range": "stddev: 0.0006203645685002902",
            "extra": "mean: 6.67517704575144 msec\nrounds: 153"
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
        "date": 1779656588138,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2519.6774321050966,
            "unit": "iter/sec",
            "range": "stddev: 0.00002605998982500495",
            "extra": "mean: 396.87619822214197 usec\nrounds: 1125"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1722.7096769142836,
            "unit": "iter/sec",
            "range": "stddev: 0.00007581925933825888",
            "extra": "mean: 580.4808630269026 usec\nrounds: 1044"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 941.6065055578371,
            "unit": "iter/sec",
            "range": "stddev: 0.0003761268408790972",
            "extra": "mean: 1.0620147525505559 msec\nrounds: 784"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 445.7556088588339,
            "unit": "iter/sec",
            "range": "stddev: 0.00041142370997007346",
            "extra": "mean: 2.2433817547693256 msec\nrounds: 367"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1177.3074224444913,
            "unit": "iter/sec",
            "range": "stddev: 0.0020255420205743065",
            "extra": "mean: 849.3958170446758 usec\nrounds: 1279"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1099.070872924515,
            "unit": "iter/sec",
            "range": "stddev: 0.0026923224873524726",
            "extra": "mean: 909.8594318481961 usec\nrounds: 1526"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1211.678510777891,
            "unit": "iter/sec",
            "range": "stddev: 0.0018681251109441957",
            "extra": "mean: 825.3014236903527 usec\nrounds: 1317"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1429.6406271080664,
            "unit": "iter/sec",
            "range": "stddev: 0.00003538915609652061",
            "extra": "mean: 699.4764845364249 usec\nrounds: 1067"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1383.9607299813758,
            "unit": "iter/sec",
            "range": "stddev: 0.00007403322930060258",
            "extra": "mean: 722.5638548381768 usec\nrounds: 372"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1254.0666777896752,
            "unit": "iter/sec",
            "range": "stddev: 0.00005232379598177704",
            "extra": "mean: 797.4057661451668 usec\nrounds: 1022"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2076.347917874784,
            "unit": "iter/sec",
            "range": "stddev: 0.000026570341798159157",
            "extra": "mean: 481.6148543272727 usec\nrounds: 1167"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 158.06417977526158,
            "unit": "iter/sec",
            "range": "stddev: 0.0005005277952433671",
            "extra": "mean: 6.326544074829714 msec\nrounds: 147"
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
        "date": 1779656588643,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2327.991912016608,
            "unit": "iter/sec",
            "range": "stddev: 0.00003152914210228816",
            "extra": "mean: 429.5547569723971 usec\nrounds: 1004"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1648.599941612866,
            "unit": "iter/sec",
            "range": "stddev: 0.00005542414188137519",
            "extra": "mean: 606.5752974743377 usec\nrounds: 1069"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 792.3144688520592,
            "unit": "iter/sec",
            "range": "stddev: 0.0005615993804946508",
            "extra": "mean: 1.2621251274747576 msec\nrounds: 808"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 421.70745485745374,
            "unit": "iter/sec",
            "range": "stddev: 0.0005317923108243245",
            "extra": "mean: 2.3713121228507132 msec\nrounds: 407"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1155.060487344828,
            "unit": "iter/sec",
            "range": "stddev: 0.002020552288261932",
            "extra": "mean: 865.755526187836 usec\nrounds: 1241"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1091.8233697247692,
            "unit": "iter/sec",
            "range": "stddev: 0.002494377366231982",
            "extra": "mean: 915.8990618162749 usec\nrounds: 1553"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1187.4835232680896,
            "unit": "iter/sec",
            "range": "stddev: 0.0018282724579519918",
            "extra": "mean: 842.1169476506811 usec\nrounds: 1490"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1445.9770329742712,
            "unit": "iter/sec",
            "range": "stddev: 0.00005704674544133718",
            "extra": "mean: 691.5739165946998 usec\nrounds: 1151"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1437.3951571354812,
            "unit": "iter/sec",
            "range": "stddev: 0.00003921051129001143",
            "extra": "mean: 695.7029144253235 usec\nrounds: 409"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1262.2638495593164,
            "unit": "iter/sec",
            "range": "stddev: 0.000049945543257944784",
            "extra": "mean: 792.2273939391686 usec\nrounds: 1089"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1906.79289789057,
            "unit": "iter/sec",
            "range": "stddev: 0.0000449021133643762",
            "extra": "mean: 524.4408037738505 usec\nrounds: 1060"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 155.16540025282794,
            "unit": "iter/sec",
            "range": "stddev: 0.000515053409805086",
            "extra": "mean: 6.444735735999075 msec\nrounds: 125"
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
        "date": 1779657503759,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2534.262226891419,
            "unit": "iter/sec",
            "range": "stddev: 0.000025830716201464913",
            "extra": "mean: 394.59215758687367 usec\nrounds: 1028"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1754.6434374664275,
            "unit": "iter/sec",
            "range": "stddev: 0.00003969604652356279",
            "extra": "mean: 569.9163594421921 usec\nrounds: 932"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 937.1100979620961,
            "unit": "iter/sec",
            "range": "stddev: 0.00029767818963389534",
            "extra": "mean: 1.0671104731180132 msec\nrounds: 651"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 452.78241956569445,
            "unit": "iter/sec",
            "range": "stddev: 0.0004430483229215466",
            "extra": "mean: 2.208566315271676 msec\nrounds: 406"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1183.1214334236795,
            "unit": "iter/sec",
            "range": "stddev: 0.0021028138598652847",
            "extra": "mean: 845.2217766913675 usec\nrounds: 1227"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1102.550709788581,
            "unit": "iter/sec",
            "range": "stddev: 0.002654275882040949",
            "extra": "mean: 906.9877613082799 usec\nrounds: 1437"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1180.811917016887,
            "unit": "iter/sec",
            "range": "stddev: 0.0021589673650569993",
            "extra": "mean: 846.8749218980814 usec\nrounds: 1306"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1536.7037071708633,
            "unit": "iter/sec",
            "range": "stddev: 0.0002333046511802341",
            "extra": "mean: 650.7435332742461 usec\nrounds: 1112"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1552.5485216023258,
            "unit": "iter/sec",
            "range": "stddev: 0.00003208446046342313",
            "extra": "mean: 644.102252577548 usec\nrounds: 388"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1330.531085134605,
            "unit": "iter/sec",
            "range": "stddev: 0.00006694535178129455",
            "extra": "mean: 751.5795844024444 usec\nrounds: 1090"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2079.2036830722727,
            "unit": "iter/sec",
            "range": "stddev: 0.000030411124285530046",
            "extra": "mean: 480.95336120335264 usec\nrounds: 1196"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 164.97736827078884,
            "unit": "iter/sec",
            "range": "stddev: 0.0004377063955604815",
            "extra": "mean: 6.061437459461897 msec\nrounds: 148"
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
        "date": 1779657524651,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3263.966150882102,
            "unit": "iter/sec",
            "range": "stddev: 0.000022834789771519967",
            "extra": "mean: 306.37572627085774 usec\nrounds: 1180"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2281.8434828866343,
            "unit": "iter/sec",
            "range": "stddev: 0.000030433253018589385",
            "extra": "mean: 438.24215267164385 usec\nrounds: 1179"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1240.1374208461687,
            "unit": "iter/sec",
            "range": "stddev: 0.00010780299916116289",
            "extra": "mean: 806.362249207577 usec\nrounds: 947"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 584.4118660890758,
            "unit": "iter/sec",
            "range": "stddev: 0.00022186658051263601",
            "extra": "mean: 1.711121998073154 msec\nrounds: 519"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2137.1576833675767,
            "unit": "iter/sec",
            "range": "stddev: 0.00037986573076809123",
            "extra": "mean: 467.9111924134082 usec\nrounds: 1819"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2211.7298301787437,
            "unit": "iter/sec",
            "range": "stddev: 0.00009649098166401373",
            "extra": "mean: 452.13478895800927 usec\nrounds: 1938"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2379.4836109409694,
            "unit": "iter/sec",
            "range": "stddev: 0.00004197882710786488",
            "extra": "mean: 420.2592509576264 usec\nrounds: 2088"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 310.0241471598642,
            "unit": "iter/sec",
            "range": "stddev: 0.005360418272746278",
            "extra": "mean: 3.2255552000094667 msec\nrounds: 5"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 562.1964925898633,
            "unit": "iter/sec",
            "range": "stddev: 0.006855294114024144",
            "extra": "mean: 1.7787375289257195 msec\nrounds: 847"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 568.9302639709084,
            "unit": "iter/sec",
            "range": "stddev: 0.005386741786218023",
            "extra": "mean: 1.7576846642335306 msec\nrounds: 822"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2682.3982322612287,
            "unit": "iter/sec",
            "range": "stddev: 0.000046922796033272796",
            "extra": "mean: 372.8007228654533 usec\nrounds: 1382"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 168.0489908572892,
            "unit": "iter/sec",
            "range": "stddev: 0.00030799199627434",
            "extra": "mean: 5.950645671232988 msec\nrounds: 146"
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
        "date": 1779705065447,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2537.6877012443033,
            "unit": "iter/sec",
            "range": "stddev: 0.000026790865205871485",
            "extra": "mean: 394.05952100003105 usec\nrounds: 1000"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1768.5499131041793,
            "unit": "iter/sec",
            "range": "stddev: 0.000041518427114595195",
            "extra": "mean: 565.434988625675 usec\nrounds: 1055"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 935.8940323211046,
            "unit": "iter/sec",
            "range": "stddev: 0.0003210629007921116",
            "extra": "mean: 1.0684970364859647 msec\nrounds: 740"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 426.8415971889431,
            "unit": "iter/sec",
            "range": "stddev: 0.0005630232484076498",
            "extra": "mean: 2.34278947175185 msec\nrounds: 354"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1196.4751372106412,
            "unit": "iter/sec",
            "range": "stddev: 0.002065103044452677",
            "extra": "mean: 835.7883660928497 usec\nrounds: 1221"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1131.782163716859,
            "unit": "iter/sec",
            "range": "stddev: 0.002579144874181906",
            "extra": "mean: 883.5622543440016 usec\nrounds: 1266"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1229.7653758914314,
            "unit": "iter/sec",
            "range": "stddev: 0.0017857700730420606",
            "extra": "mean: 813.1632420331567 usec\nrounds: 1318"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1545.7579477459042,
            "unit": "iter/sec",
            "range": "stddev: 0.000052806227116475755",
            "extra": "mean: 646.9318184378391 usec\nrounds: 1063"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1548.6845644624532,
            "unit": "iter/sec",
            "range": "stddev: 0.000039007635362353964",
            "extra": "mean: 645.7092831858236 usec\nrounds: 452"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1329.3015545931141,
            "unit": "iter/sec",
            "range": "stddev: 0.0000624822607951",
            "extra": "mean: 752.2747540200463 usec\nrounds: 1057"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2089.7896305101317,
            "unit": "iter/sec",
            "range": "stddev: 0.000028803990196561243",
            "extra": "mean: 478.51706478029246 usec\nrounds: 1590"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 166.96457998735232,
            "unit": "iter/sec",
            "range": "stddev: 0.00024825759503210675",
            "extra": "mean: 5.989294256756437 msec\nrounds: 148"
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
        "date": 1779705068093,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2313.800250432627,
            "unit": "iter/sec",
            "range": "stddev: 0.00002805280587479546",
            "extra": "mean: 432.18942508672615 usec\nrounds: 1148"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1647.4390469864916,
            "unit": "iter/sec",
            "range": "stddev: 0.00005221264885402529",
            "extra": "mean: 607.0027305891576 usec\nrounds: 1069"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 793.3529902524552,
            "unit": "iter/sec",
            "range": "stddev: 0.0006123050618133201",
            "extra": "mean: 1.2604729701488704 msec\nrounds: 804"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 412.5328389575051,
            "unit": "iter/sec",
            "range": "stddev: 0.0006275306990832651",
            "extra": "mean: 2.4240494466502573 msec\nrounds: 403"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1135.577948249105,
            "unit": "iter/sec",
            "range": "stddev: 0.002032396099473388",
            "extra": "mean: 880.6088578435797 usec\nrounds: 1224"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1047.8630568068734,
            "unit": "iter/sec",
            "range": "stddev: 0.0026395703517728473",
            "extra": "mean: 954.3231756325818 usec\nrounds: 1264"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1180.8713739334812,
            "unit": "iter/sec",
            "range": "stddev: 0.0018806662288827394",
            "extra": "mean: 846.8322817149858 usec\nrounds: 1143"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1398.5828260642625,
            "unit": "iter/sec",
            "range": "stddev: 0.00005487191168543816",
            "extra": "mean: 715.0094948713833 usec\nrounds: 780"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1355.5721168606794,
            "unit": "iter/sec",
            "range": "stddev: 0.00010084515611030487",
            "extra": "mean: 737.6959053391154 usec\nrounds: 412"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1223.5682188367946,
            "unit": "iter/sec",
            "range": "stddev: 0.00005021818636140648",
            "extra": "mean: 817.281770321451 usec\nrounds: 1058"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1908.8477584531947,
            "unit": "iter/sec",
            "range": "stddev: 0.000029429724184396013",
            "extra": "mean: 523.8762471085355 usec\nrounds: 951"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 150.48518144493366,
            "unit": "iter/sec",
            "range": "stddev: 0.0010810736062834574",
            "extra": "mean: 6.645172570469507 msec\nrounds: 149"
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
        "date": 1779708322290,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3291.0412063844265,
            "unit": "iter/sec",
            "range": "stddev: 0.000022190481450935098",
            "extra": "mean: 303.85520486952845 usec\nrounds: 1191"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2280.310818981579,
            "unit": "iter/sec",
            "range": "stddev: 0.000034779254421422075",
            "extra": "mean: 438.53670809956293 usec\nrounds: 1247"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1184.499664175868,
            "unit": "iter/sec",
            "range": "stddev: 0.00011826721565739628",
            "extra": "mean: 844.2383144918523 usec\nrounds: 973"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 579.3706036697423,
            "unit": "iter/sec",
            "range": "stddev: 0.00017545309519252265",
            "extra": "mean: 1.7260109395712948 msec\nrounds: 513"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2153.681070684763,
            "unit": "iter/sec",
            "range": "stddev: 0.00033712155691050514",
            "extra": "mean: 464.32130254181504 usec\nrounds: 1613"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2263.644926023009,
            "unit": "iter/sec",
            "range": "stddev: 0.00006431181608510088",
            "extra": "mean: 441.7653972599391 usec\nrounds: 146"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2300.3232795019603,
            "unit": "iter/sec",
            "range": "stddev: 0.0002114790379990356",
            "extra": "mean: 434.7215058469993 usec\nrounds: 1967"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1472.6275614920703,
            "unit": "iter/sec",
            "range": "stddev: 0.00012305398294669434",
            "extra": "mean: 679.0583214311141 usec\nrounds: 56"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1513.9814251901896,
            "unit": "iter/sec",
            "range": "stddev: 0.00008286691316893766",
            "extra": "mean: 660.5100851051576 usec\nrounds: 47"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 716.1079138269334,
            "unit": "iter/sec",
            "range": "stddev: 0.005618563536438027",
            "extra": "mean: 1.3964375769231852 msec\nrounds: 910"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2721.1731356968166,
            "unit": "iter/sec",
            "range": "stddev: 0.000020606685170284454",
            "extra": "mean: 367.4885610481113 usec\nrounds: 2064"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 234.33215984435262,
            "unit": "iter/sec",
            "range": "stddev: 0.00031374180198990307",
            "extra": "mean: 4.267446690476532 msec\nrounds: 210"
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
        "date": 1779708324005,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2364.128762407043,
            "unit": "iter/sec",
            "range": "stddev: 0.000031234541040402726",
            "extra": "mean: 422.9888049675635 usec\nrounds: 1087"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1653.180717356917,
            "unit": "iter/sec",
            "range": "stddev: 0.00005405097844539548",
            "extra": "mean: 604.8945463135975 usec\nrounds: 1058"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 812.8677758631554,
            "unit": "iter/sec",
            "range": "stddev: 0.00047578185476099017",
            "extra": "mean: 1.2302123785607513 msec\nrounds: 737"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 424.15898273857033,
            "unit": "iter/sec",
            "range": "stddev: 0.0004690539152191641",
            "extra": "mean: 2.3576065595582314 msec\nrounds: 361"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1140.6558464969455,
            "unit": "iter/sec",
            "range": "stddev: 0.001996234719598472",
            "extra": "mean: 876.6886200347703 usec\nrounds: 1158"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1096.6027671006782,
            "unit": "iter/sec",
            "range": "stddev: 0.0024609946730405857",
            "extra": "mean: 911.9072375167469 usec\nrounds: 1482"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1187.4359124528735,
            "unit": "iter/sec",
            "range": "stddev: 0.0018927524405051362",
            "extra": "mean: 842.1507127355706 usec\nrounds: 1382"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1454.9852555965106,
            "unit": "iter/sec",
            "range": "stddev: 0.00005073258904006704",
            "extra": "mean: 687.2921881191318 usec\nrounds: 1111"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1466.2120909827547,
            "unit": "iter/sec",
            "range": "stddev: 0.0000436716079388671",
            "extra": "mean: 682.029568675656 usec\nrounds: 415"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1270.9969253092127,
            "unit": "iter/sec",
            "range": "stddev: 0.00009376933571240554",
            "extra": "mean: 786.7839646871817 usec\nrounds: 623"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1942.0789762166062,
            "unit": "iter/sec",
            "range": "stddev: 0.000026145522901627056",
            "extra": "mean: 514.9121185319226 usec\nrounds: 1662"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 150.2854467673303,
            "unit": "iter/sec",
            "range": "stddev: 0.0011781464574800128",
            "extra": "mean: 6.654004239999267 msec\nrounds: 125"
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
        "date": 1779708805357,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2956.198960139869,
            "unit": "iter/sec",
            "range": "stddev: 0.000029643510729804518",
            "extra": "mean: 338.27222507130784 usec\nrounds: 1053"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2036.9668627352587,
            "unit": "iter/sec",
            "range": "stddev: 0.00004015600283546748",
            "extra": "mean: 490.9260029184718 usec\nrounds: 1028"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1022.1581293866417,
            "unit": "iter/sec",
            "range": "stddev: 0.0001221292362979979",
            "extra": "mean: 978.3222098914011 usec\nrounds: 829"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 471.36338940819854,
            "unit": "iter/sec",
            "range": "stddev: 0.00012946750786636607",
            "extra": "mean: 2.121505450933536 msec\nrounds: 428"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1418.9569477902053,
            "unit": "iter/sec",
            "range": "stddev: 0.0013587430458841167",
            "extra": "mean: 704.743016733057 usec\nrounds: 1255"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1294.1635825447963,
            "unit": "iter/sec",
            "range": "stddev: 0.0019247578790435053",
            "extra": "mean: 772.6998452804833 usec\nrounds: 1409"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1483.4271077985554,
            "unit": "iter/sec",
            "range": "stddev: 0.0012125915364234975",
            "extra": "mean: 674.1146866892747 usec\nrounds: 1465"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1771.6276268635975,
            "unit": "iter/sec",
            "range": "stddev: 0.00004411536183426204",
            "extra": "mean: 564.452701480136 usec\nrounds: 1216"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1816.5353274244076,
            "unit": "iter/sec",
            "range": "stddev: 0.00004089635811040547",
            "extra": "mean: 550.4985148942078 usec\nrounds: 470"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1468.236390962352,
            "unit": "iter/sec",
            "range": "stddev: 0.00007334042125181563",
            "extra": "mean: 681.0892347822495 usec\nrounds: 1150"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2501.107878734812,
            "unit": "iter/sec",
            "range": "stddev: 0.00003177405834920416",
            "extra": "mean: 399.8228179209332 usec\nrounds: 1741"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 159.6217918955002,
            "unit": "iter/sec",
            "range": "stddev: 0.00019194118390248194",
            "extra": "mean: 6.264808759036305 msec\nrounds: 166"
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
        "date": 1779708809642,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2323.2522315359724,
            "unit": "iter/sec",
            "range": "stddev: 0.00003079997551012824",
            "extra": "mean: 430.4310941472204 usec\nrounds: 1179"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1648.1845924871707,
            "unit": "iter/sec",
            "range": "stddev: 0.00005145947426010789",
            "extra": "mean: 606.7281568813621 usec\nrounds: 1039"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 789.0116640855997,
            "unit": "iter/sec",
            "range": "stddev: 0.0006099085458100995",
            "extra": "mean: 1.2674083863626004 msec\nrounds: 660"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 407.0151421325505,
            "unit": "iter/sec",
            "range": "stddev: 0.0006431838020444305",
            "extra": "mean: 2.45691104945264 msec\nrounds: 364"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1152.7130649540034,
            "unit": "iter/sec",
            "range": "stddev: 0.0018563807710248433",
            "extra": "mean: 867.5185789100975 usec\nrounds: 1375"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1055.2208919378086,
            "unit": "iter/sec",
            "range": "stddev: 0.0025798534634916834",
            "extra": "mean: 947.6688792273617 usec\nrounds: 1449"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1201.1059206372474,
            "unit": "iter/sec",
            "range": "stddev: 0.0017462115980579965",
            "extra": "mean: 832.5660400287175 usec\nrounds: 1449"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1442.2469703011107,
            "unit": "iter/sec",
            "range": "stddev: 0.00005466771525818175",
            "extra": "mean: 693.3625243055433 usec\nrounds: 1152"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1456.8084500815746,
            "unit": "iter/sec",
            "range": "stddev: 0.00003506501115729722",
            "extra": "mean: 686.4320425543966 usec\nrounds: 376"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1262.9640569085366,
            "unit": "iter/sec",
            "range": "stddev: 0.00005357536636824207",
            "extra": "mean: 791.7881704787262 usec\nrounds: 1050"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1892.1921402662895,
            "unit": "iter/sec",
            "range": "stddev: 0.00004141160999560734",
            "extra": "mean: 528.4875561629113 usec\nrounds: 1647"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 149.21115939525288,
            "unit": "iter/sec",
            "range": "stddev: 0.00022247626634041269",
            "extra": "mean: 6.701911599996687 msec\nrounds: 140"
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
        "date": 1779710003180,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2384.995824701704,
            "unit": "iter/sec",
            "range": "stddev: 0.00003956568712444548",
            "extra": "mean: 419.2879457661407 usec\nrounds: 1051"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1724.8723107202109,
            "unit": "iter/sec",
            "range": "stddev: 0.000054677293418909854",
            "extra": "mean: 579.75305985546 usec\nrounds: 969"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 912.1013058485154,
            "unit": "iter/sec",
            "range": "stddev: 0.00043821742092361017",
            "extra": "mean: 1.0963694422843893 msec\nrounds: 823"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 438.207501742026,
            "unit": "iter/sec",
            "range": "stddev: 0.0005370752267734388",
            "extra": "mean: 2.2820239179490427 msec\nrounds: 390"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1197.9171037727658,
            "unit": "iter/sec",
            "range": "stddev: 0.0019896499879654386",
            "extra": "mean: 834.7823040931313 usec\nrounds: 1368"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1081.1067280309237,
            "unit": "iter/sec",
            "range": "stddev: 0.002761825043342787",
            "extra": "mean: 924.9780563491196 usec\nrounds: 1331"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1224.984048666936,
            "unit": "iter/sec",
            "range": "stddev: 0.0019180317283029966",
            "extra": "mean: 816.3371605436249 usec\nrounds: 1470"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1443.2279754046788,
            "unit": "iter/sec",
            "range": "stddev: 0.000056898747163258756",
            "extra": "mean: 692.8912251161163 usec\nrounds: 1075"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1461.4906379678425,
            "unit": "iter/sec",
            "range": "stddev: 0.0000329120616117989",
            "extra": "mean: 684.2329153681538 usec\nrounds: 449"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1266.6269039078088,
            "unit": "iter/sec",
            "range": "stddev: 0.00004917435165039902",
            "extra": "mean: 789.498467871471 usec\nrounds: 996"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2089.79081795728,
            "unit": "iter/sec",
            "range": "stddev: 0.000030181449176967795",
            "extra": "mean: 478.51679288048354 usec\nrounds: 1545"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 151.7295950973124,
            "unit": "iter/sec",
            "range": "stddev: 0.0007995479151508988",
            "extra": "mean: 6.590672039681158 msec\nrounds: 126"
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
        "date": 1779710018899,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2354.262761051523,
            "unit": "iter/sec",
            "range": "stddev: 0.00003457066873082069",
            "extra": "mean: 424.761422787554 usec\nrounds: 1062"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1664.0991472857158,
            "unit": "iter/sec",
            "range": "stddev: 0.00005923507563824117",
            "extra": "mean: 600.9257330797165 usec\nrounds: 1049"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 792.2223498580414,
            "unit": "iter/sec",
            "range": "stddev: 0.0005899215951644523",
            "extra": "mean: 1.262271886395518 msec\nrounds: 713"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 415.81792523081634,
            "unit": "iter/sec",
            "range": "stddev: 0.0006625661770316966",
            "extra": "mean: 2.4048987292813555 msec\nrounds: 362"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1169.744908563914,
            "unit": "iter/sec",
            "range": "stddev: 0.0019212497782067624",
            "extra": "mean: 854.8872430893431 usec\nrounds: 1230"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1068.306983567312,
            "unit": "iter/sec",
            "range": "stddev: 0.002623526703060546",
            "extra": "mean: 936.0605288386115 usec\nrounds: 1231"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1226.3050433892424,
            "unit": "iter/sec",
            "range": "stddev: 0.0016103355195067905",
            "extra": "mean: 815.4577895530919 usec\nrounds: 1321"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1470.4347667440893,
            "unit": "iter/sec",
            "range": "stddev: 0.00005170805879623585",
            "extra": "mean: 680.0709712639958 usec\nrounds: 696"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1463.8206951137456,
            "unit": "iter/sec",
            "range": "stddev: 0.000039178324962768605",
            "extra": "mean: 683.1437780173584 usec\nrounds: 464"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1263.604153671226,
            "unit": "iter/sec",
            "range": "stddev: 0.00006286313219798908",
            "extra": "mean: 791.3870788526922 usec\nrounds: 1116"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1908.5633101984643,
            "unit": "iter/sec",
            "range": "stddev: 0.000033240672132784556",
            "extra": "mean: 523.9543245206855 usec\nrounds: 1513"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 153.59578567339412,
            "unit": "iter/sec",
            "range": "stddev: 0.0011229723578866876",
            "extra": "mean: 6.510595298014222 msec\nrounds: 151"
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
        "date": 1779710302477,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2488.4534938544953,
            "unit": "iter/sec",
            "range": "stddev: 0.00003355790169599965",
            "extra": "mean: 401.8560131702714 usec\nrounds: 1063"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1710.5302681961748,
            "unit": "iter/sec",
            "range": "stddev: 0.00005559365181618914",
            "extra": "mean: 584.6140337841209 usec\nrounds: 1036"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 892.5053787875212,
            "unit": "iter/sec",
            "range": "stddev: 0.0005051154711147287",
            "extra": "mean: 1.120441426760376 msec\nrounds: 710"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 420.31505167660697,
            "unit": "iter/sec",
            "range": "stddev: 0.00048277034549886594",
            "extra": "mean: 2.379167712436352 msec\nrounds: 386"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1174.9447509945737,
            "unit": "iter/sec",
            "range": "stddev: 0.002113266003378546",
            "extra": "mean: 851.1038490563191 usec\nrounds: 1484"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1112.2729556342276,
            "unit": "iter/sec",
            "range": "stddev: 0.002543629582992833",
            "extra": "mean: 899.059888972839 usec\nrounds: 1324"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1240.89591361329,
            "unit": "iter/sec",
            "range": "stddev: 0.0018582501845141517",
            "extra": "mean: 805.8693634409354 usec\nrounds: 1395"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1552.1333980382653,
            "unit": "iter/sec",
            "range": "stddev: 0.000019884976523982636",
            "extra": "mean: 644.2745200018862 usec\nrounds: 50"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1542.6516043322235,
            "unit": "iter/sec",
            "range": "stddev: 0.00003293999262054844",
            "extra": "mean: 648.2345055693089 usec\nrounds: 449"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1301.2168203042015,
            "unit": "iter/sec",
            "range": "stddev: 0.00007358054990724971",
            "extra": "mean: 768.511430528709 usec\nrounds: 1022"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2066.1002199389836,
            "unit": "iter/sec",
            "range": "stddev: 0.000026383294234267496",
            "extra": "mean: 484.00362690515186 usec\nrounds: 1509"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 145.1997090778576,
            "unit": "iter/sec",
            "range": "stddev: 0.00027240851881861196",
            "extra": "mean: 6.887066140496119 msec\nrounds: 121"
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
        "date": 1779710307625,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2294.0709081133664,
            "unit": "iter/sec",
            "range": "stddev: 0.00004067108815131328",
            "extra": "mean: 435.9063167852974 usec\nrounds: 1269"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1629.2246360427168,
            "unit": "iter/sec",
            "range": "stddev: 0.00006585760017497182",
            "extra": "mean: 613.7889017127414 usec\nrounds: 1109"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 788.371162042753,
            "unit": "iter/sec",
            "range": "stddev: 0.0006253790763125309",
            "extra": "mean: 1.2684380760565803 msec\nrounds: 710"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 411.19217002691664,
            "unit": "iter/sec",
            "range": "stddev: 0.0005378488436009399",
            "extra": "mean: 2.4319529234580024 msec\nrounds: 405"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1183.4847371827645,
            "unit": "iter/sec",
            "range": "stddev: 0.0019456149427380318",
            "extra": "mean: 844.9623122140619 usec\nrounds: 1310"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1084.1149963529306,
            "unit": "iter/sec",
            "range": "stddev: 0.0025227695434381463",
            "extra": "mean: 922.4113709007793 usec\nrounds: 1189"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1203.163569625571,
            "unit": "iter/sec",
            "range": "stddev: 0.0018514762816134423",
            "extra": "mean: 831.1421865201619 usec\nrounds: 1276"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1472.6019071666944,
            "unit": "iter/sec",
            "range": "stddev: 0.000050819714068501834",
            "extra": "mean: 679.0701513649492 usec\nrounds: 1209"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1485.1381015077634,
            "unit": "iter/sec",
            "range": "stddev: 0.00003104226251028613",
            "extra": "mean: 673.3380545450725 usec\nrounds: 440"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1284.217124641303,
            "unit": "iter/sec",
            "range": "stddev: 0.000042941407957113574",
            "extra": "mean: 778.6845236776545 usec\nrounds: 1077"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1945.0038228382584,
            "unit": "iter/sec",
            "range": "stddev: 0.00003466594234478778",
            "extra": "mean: 514.13780695852 usec\nrounds: 1782"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 155.729704455167,
            "unit": "iter/sec",
            "range": "stddev: 0.0005794289670566908",
            "extra": "mean: 6.4213825069442025 msec\nrounds: 144"
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
        "date": 1779714952316,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2526.2440206829488,
            "unit": "iter/sec",
            "range": "stddev: 0.00002590314028533497",
            "extra": "mean: 395.8445786759976 usec\nrounds: 1163"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1723.4861383901412,
            "unit": "iter/sec",
            "range": "stddev: 0.00005919560198855018",
            "extra": "mean: 580.2193459670475 usec\nrounds: 1029"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 886.4954191207406,
            "unit": "iter/sec",
            "range": "stddev: 0.00046301743514737937",
            "extra": "mean: 1.1280374138783902 msec\nrounds: 807"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 425.05085119670923,
            "unit": "iter/sec",
            "range": "stddev: 0.0004326165333806734",
            "extra": "mean: 2.3526596810347526 msec\nrounds: 348"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1176.0041153591171,
            "unit": "iter/sec",
            "range": "stddev: 0.0021003709829834107",
            "extra": "mean: 850.3371603377675 usec\nrounds: 1422"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1092.540305808035,
            "unit": "iter/sec",
            "range": "stddev: 0.002738338176166966",
            "extra": "mean: 915.2980395175508 usec\nrounds: 1493"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1242.207318869086,
            "unit": "iter/sec",
            "range": "stddev: 0.0018528683276665045",
            "extra": "mean: 805.0186026197357 usec\nrounds: 1374"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1419.6254548345066,
            "unit": "iter/sec",
            "range": "stddev: 0.00023527894726460915",
            "extra": "mean: 704.4111505570146 usec\nrounds: 1076"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1450.0678605293551,
            "unit": "iter/sec",
            "range": "stddev: 0.00003255846291334264",
            "extra": "mean: 689.6228978104132 usec\nrounds: 411"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1255.1226891212618,
            "unit": "iter/sec",
            "range": "stddev: 0.00004930002995599919",
            "extra": "mean: 796.7348600001179 usec\nrounds: 1050"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2079.6513372719464,
            "unit": "iter/sec",
            "range": "stddev: 0.00002720173570839824",
            "extra": "mean: 480.8498338532959 usec\nrounds: 1601"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 142.75641522834349,
            "unit": "iter/sec",
            "range": "stddev: 0.00036833938911079396",
            "extra": "mean: 7.004939136363629 msec\nrounds: 132"
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
        "date": 1779714959311,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2275.9142813815706,
            "unit": "iter/sec",
            "range": "stddev: 0.00003419176885594248",
            "extra": "mean: 439.38385912889487 usec\nrounds: 1079"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1619.723854336524,
            "unit": "iter/sec",
            "range": "stddev: 0.00006093379292054273",
            "extra": "mean: 617.389190955407 usec\nrounds: 995"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 774.8449791681477,
            "unit": "iter/sec",
            "range": "stddev: 0.0006163008125473702",
            "extra": "mean: 1.290580731482022 msec\nrounds: 648"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 400.03366529301917,
            "unit": "iter/sec",
            "range": "stddev: 0.0006686723310914504",
            "extra": "mean: 2.4997896096257644 msec\nrounds: 374"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1130.6015161167516,
            "unit": "iter/sec",
            "range": "stddev: 0.0019811146889816673",
            "extra": "mean: 884.4849274876923 usec\nrounds: 1186"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1033.6290672625678,
            "unit": "iter/sec",
            "range": "stddev: 0.0026917415162365597",
            "extra": "mean: 967.4650526695907 usec\nrounds: 1405"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1176.7086570363906,
            "unit": "iter/sec",
            "range": "stddev: 0.0018479313431175425",
            "extra": "mean: 849.8280300907774 usec\nrounds: 1429"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1367.4512915666103,
            "unit": "iter/sec",
            "range": "stddev: 0.00006556239593181474",
            "extra": "mean: 731.287473394652 usec\nrounds: 1090"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1388.9486707515757,
            "unit": "iter/sec",
            "range": "stddev: 0.00002917223040974927",
            "extra": "mean: 719.969010416266 usec\nrounds: 384"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1204.113498685626,
            "unit": "iter/sec",
            "range": "stddev: 0.00005221853576945647",
            "extra": "mean: 830.4864957427768 usec\nrounds: 1057"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1900.812484082218,
            "unit": "iter/sec",
            "range": "stddev: 0.000036629461175048654",
            "extra": "mean: 526.090820832775 usec\nrounds: 1680"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 154.46879196682443,
            "unit": "iter/sec",
            "range": "stddev: 0.0002005414523288641",
            "extra": "mean: 6.473799576388038 msec\nrounds: 144"
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
        "date": 1779851815428,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2941.124386341798,
            "unit": "iter/sec",
            "range": "stddev: 0.00002969200019828766",
            "extra": "mean: 340.00602104551274 usec\nrounds: 1378"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1941.4705476033396,
            "unit": "iter/sec",
            "range": "stddev: 0.0000583324372898449",
            "extra": "mean: 515.0734844957891 usec\nrounds: 1032"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 956.5392477878191,
            "unit": "iter/sec",
            "range": "stddev: 0.00012672514882572158",
            "extra": "mean: 1.0454354092764016 msec\nrounds: 733"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 471.3422817208703,
            "unit": "iter/sec",
            "range": "stddev: 0.0001323823198176209",
            "extra": "mean: 2.121600456358383 msec\nrounds: 401"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1400.9789901751399,
            "unit": "iter/sec",
            "range": "stddev: 0.0013896315873151031",
            "extra": "mean: 713.7865785374751 usec\nrounds: 1286"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1281.4515011862234,
            "unit": "iter/sec",
            "range": "stddev: 0.0019183917125374515",
            "extra": "mean: 780.3650774721577 usec\nrounds: 1678"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1485.0324776142947,
            "unit": "iter/sec",
            "range": "stddev: 0.0011384453978568907",
            "extra": "mean: 673.3859461488009 usec\nrounds: 1337"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1609.8405710745794,
            "unit": "iter/sec",
            "range": "stddev: 0.00004465415365434234",
            "extra": "mean: 621.1795242136885 usec\nrounds: 1177"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1592.8324051532793,
            "unit": "iter/sec",
            "range": "stddev: 0.00004991274837967773",
            "extra": "mean: 627.8124407594341 usec\nrounds: 422"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1354.209648544248,
            "unit": "iter/sec",
            "range": "stddev: 0.00007317442998133102",
            "extra": "mean: 738.4381000940162 usec\nrounds: 1079"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2489.9674079102097,
            "unit": "iter/sec",
            "range": "stddev: 0.000027678224344843435",
            "extra": "mean: 401.6116824755084 usec\nrounds: 1729"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 156.6050846902274,
            "unit": "iter/sec",
            "range": "stddev: 0.000485136889254912",
            "extra": "mean: 6.385488708607703 msec\nrounds: 151"
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
        "date": 1779851865046,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3287.4822846946613,
            "unit": "iter/sec",
            "range": "stddev: 0.00003350796449604743",
            "extra": "mean: 304.1841486585772 usec\nrounds: 1305"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2228.61905373673,
            "unit": "iter/sec",
            "range": "stddev: 0.000058491178326721844",
            "extra": "mean: 448.70835970072943 usec\nrounds: 1201"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1212.4107545133484,
            "unit": "iter/sec",
            "range": "stddev: 0.0001253346557917357",
            "extra": "mean: 824.8029772726585 usec\nrounds: 616"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 551.4853605051911,
            "unit": "iter/sec",
            "range": "stddev: 0.0003223763506520355",
            "extra": "mean: 1.813284760784846 msec\nrounds: 510"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2258.1037207052063,
            "unit": "iter/sec",
            "range": "stddev: 0.0001008239637875867",
            "extra": "mean: 442.8494540931449 usec\nrounds: 1808"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2268.0310272238135,
            "unit": "iter/sec",
            "range": "stddev: 0.00009164802915404596",
            "extra": "mean: 440.9110757289998 usec\nrounds: 1611"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2278.545684441074,
            "unit": "iter/sec",
            "range": "stddev: 0.00014033761133755882",
            "extra": "mean: 438.87643194009485 usec\nrounds: 1866"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 431.04684719906425,
            "unit": "iter/sec",
            "range": "stddev: 0.011944443815607915",
            "extra": "mean: 2.3199334515447325 msec\nrounds: 939"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 554.4141572490827,
            "unit": "iter/sec",
            "range": "stddev: 0.007387597370304406",
            "extra": "mean: 1.803705744026894 msec\nrounds: 586"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 580.9982061807846,
            "unit": "iter/sec",
            "range": "stddev: 0.010461594546877331",
            "extra": "mean: 1.7211757099450287 msec\nrounds: 724"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2730.4674460609967,
            "unit": "iter/sec",
            "range": "stddev: 0.000023036815128969515",
            "extra": "mean: 366.2376570145933 usec\nrounds: 2003"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 161.52098279377836,
            "unit": "iter/sec",
            "range": "stddev: 0.000223811847397937",
            "extra": "mean: 6.191146083334252 msec\nrounds: 144"
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
        "date": 1779853962537,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2277.8491000543568,
            "unit": "iter/sec",
            "range": "stddev: 0.00003479983789799061",
            "extra": "mean: 439.0106438464852 usec\nrounds: 1373"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1626.4812323769186,
            "unit": "iter/sec",
            "range": "stddev: 0.00005661824066210354",
            "extra": "mean: 614.8241861595986 usec\nrounds: 1026"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 780.6004488122348,
            "unit": "iter/sec",
            "range": "stddev: 0.0005745001603980085",
            "extra": "mean: 1.2810651102258581 msec\nrounds: 753"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 400.7684589964575,
            "unit": "iter/sec",
            "range": "stddev: 0.0007356301110035976",
            "extra": "mean: 2.4952063405988723 msec\nrounds: 367"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1182.565346460823,
            "unit": "iter/sec",
            "range": "stddev: 0.0018625663465669145",
            "extra": "mean: 845.6192319458675 usec\nrounds: 1177"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1067.16087144682,
            "unit": "iter/sec",
            "range": "stddev: 0.002552586739349965",
            "extra": "mean: 937.0658414829569 usec\nrounds: 1268"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1197.4465072662326,
            "unit": "iter/sec",
            "range": "stddev: 0.0017541319889254625",
            "extra": "mean: 835.1103735589805 usec\nrounds: 1301"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1431.656166540403,
            "unit": "iter/sec",
            "range": "stddev: 0.00010267956246438019",
            "extra": "mean: 698.4917352163543 usec\nrounds: 1133"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1450.2076786672135,
            "unit": "iter/sec",
            "range": "stddev: 0.00004047258377807001",
            "extra": "mean: 689.5564095475149 usec\nrounds: 398"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1252.7341143961187,
            "unit": "iter/sec",
            "range": "stddev: 0.000048250357973137204",
            "extra": "mean: 798.2539858284698 usec\nrounds: 1129"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1903.0693219184357,
            "unit": "iter/sec",
            "range": "stddev: 0.00003205889824441592",
            "extra": "mean: 525.4669330657516 usec\nrounds: 1494"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 152.94457456520905,
            "unit": "iter/sec",
            "range": "stddev: 0.0008661900949651268",
            "extra": "mean: 6.538316268117392 msec\nrounds: 138"
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
        "date": 1779854002632,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 1475.9396347012196,
            "unit": "iter/sec",
            "range": "stddev: 0.0001752862482547893",
            "extra": "mean: 677.5344848046134 usec\nrounds: 691"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1687.2265833454558,
            "unit": "iter/sec",
            "range": "stddev: 0.00008976181662148315",
            "extra": "mean: 592.6886227795121 usec\nrounds: 957"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 923.1103446038304,
            "unit": "iter/sec",
            "range": "stddev: 0.00023471682871757107",
            "extra": "mean: 1.0832941108781187 msec\nrounds: 478"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 395.4051734440929,
            "unit": "iter/sec",
            "range": "stddev: 0.0007603341961288733",
            "extra": "mean: 2.5290513811180366 msec\nrounds: 286"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1186.5733645176695,
            "unit": "iter/sec",
            "range": "stddev: 0.0015782363957661646",
            "extra": "mean: 842.7628917883981 usec\nrounds: 1303"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1107.317010605791,
            "unit": "iter/sec",
            "range": "stddev: 0.0024794488178070608",
            "extra": "mean: 903.0837514660052 usec\nrounds: 1364"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1209.0081960907298,
            "unit": "iter/sec",
            "range": "stddev: 0.001827460913454358",
            "extra": "mean: 827.1242521212447 usec\nrounds: 1297"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1456.6015401152638,
            "unit": "iter/sec",
            "range": "stddev: 0.0000555852224700928",
            "extra": "mean: 686.5295500929293 usec\nrounds: 1078"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1458.7764974772283,
            "unit": "iter/sec",
            "range": "stddev: 0.00004033242303120261",
            "extra": "mean: 685.5059714283683 usec\nrounds: 385"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1248.9960960392332,
            "unit": "iter/sec",
            "range": "stddev: 0.0001018752595705863",
            "extra": "mean: 800.643014955099 usec\nrounds: 1003"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2081.446138605323,
            "unit": "iter/sec",
            "range": "stddev: 0.00002651201220757732",
            "extra": "mean: 480.4352038962929 usec\nrounds: 1540"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 148.27096399183384,
            "unit": "iter/sec",
            "range": "stddev: 0.0003278523429981937",
            "extra": "mean: 6.744408838234004 msec\nrounds: 136"
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
        "date": 1779856295538,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2340.919694214352,
            "unit": "iter/sec",
            "range": "stddev: 0.000029620476731164112",
            "extra": "mean: 427.1825310673953 usec\nrounds: 1030"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1640.7024622573708,
            "unit": "iter/sec",
            "range": "stddev: 0.000049886502419344826",
            "extra": "mean: 609.4950321608853 usec\nrounds: 995"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 763.8771772946744,
            "unit": "iter/sec",
            "range": "stddev: 0.0006945043104176472",
            "extra": "mean: 1.309110979780246 msec\nrounds: 544"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 392.9117103550665,
            "unit": "iter/sec",
            "range": "stddev: 0.0009206731179929647",
            "extra": "mean: 2.545101033248207 msec\nrounds: 391"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1155.0897387110429,
            "unit": "iter/sec",
            "range": "stddev: 0.001983853827095671",
            "extra": "mean: 865.7336018895758 usec\nrounds: 1482"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1072.3887010122644,
            "unit": "iter/sec",
            "range": "stddev: 0.002528191653261502",
            "extra": "mean: 932.4977026110643 usec\nrounds: 1187"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1200.5242814220146,
            "unit": "iter/sec",
            "range": "stddev: 0.0018844681061900926",
            "extra": "mean: 832.9694080118941 usec\nrounds: 1348"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1391.5836264938475,
            "unit": "iter/sec",
            "range": "stddev: 0.00043634107873395255",
            "extra": "mean: 718.6057531587529 usec\nrounds: 1029"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1380.3892888442133,
            "unit": "iter/sec",
            "range": "stddev: 0.00007990136028017803",
            "extra": "mean: 724.433323325256 usec\nrounds: 433"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1226.820306904514,
            "unit": "iter/sec",
            "range": "stddev: 0.0000508912937695699",
            "extra": "mean: 815.1152979552304 usec\nrounds: 1027"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1926.2422542745242,
            "unit": "iter/sec",
            "range": "stddev: 0.00003982009590388475",
            "extra": "mean: 519.145500925909 usec\nrounds: 1619"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 145.9236896167869,
            "unit": "iter/sec",
            "range": "stddev: 0.0007218906469566355",
            "extra": "mean: 6.852896898551015 msec\nrounds: 138"
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
        "date": 1779856384914,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3267.2928231867663,
            "unit": "iter/sec",
            "range": "stddev: 0.00002409219993048575",
            "extra": "mean: 306.0637825001085 usec\nrounds: 1200"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2257.256643644161,
            "unit": "iter/sec",
            "range": "stddev: 0.000033108537704462175",
            "extra": "mean: 443.0156414937291 usec\nrounds: 1205"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1176.492486813458,
            "unit": "iter/sec",
            "range": "stddev: 0.0001728471780875846",
            "extra": "mean: 849.9841785717733 usec\nrounds: 952"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 542.2038593761877,
            "unit": "iter/sec",
            "range": "stddev: 0.0003125069382608043",
            "extra": "mean: 1.8443247548081132 msec\nrounds: 416"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 2114.638759683521,
            "unit": "iter/sec",
            "range": "stddev: 0.00038273938102427034",
            "extra": "mean: 472.8940086909506 usec\nrounds: 1841"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 2111.9649568991135,
            "unit": "iter/sec",
            "range": "stddev: 0.0003125875147477801",
            "extra": "mean: 473.4927048544627 usec\nrounds: 1545"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 2236.035881712593,
            "unit": "iter/sec",
            "range": "stddev: 0.00021218498923436974",
            "extra": "mean: 447.2200147495371 usec\nrounds: 2034"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 414.16904999630725,
            "unit": "iter/sec",
            "range": "stddev: 0.010951750822010713",
            "extra": "mean: 2.4144730274000823 msec\nrounds: 73"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 671.5720637359439,
            "unit": "iter/sec",
            "range": "stddev: 0.004696175239750204",
            "extra": "mean: 1.489043475747066 msec\nrounds: 536"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 638.2757793514113,
            "unit": "iter/sec",
            "range": "stddev: 0.009727622035765987",
            "extra": "mean: 1.5667208945577686 msec\nrounds: 882"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2683.3511982551986,
            "unit": "iter/sec",
            "range": "stddev: 0.00002162908813285862",
            "extra": "mean: 372.66832632651 usec\nrounds: 1998"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 151.3799599394736,
            "unit": "iter/sec",
            "range": "stddev: 0.0002005073123126832",
            "extra": "mean: 6.605894204225123 msec\nrounds: 142"
          }
        ]
      }
    ]
  }
}