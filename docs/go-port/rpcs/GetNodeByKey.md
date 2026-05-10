# GetNodeByKey — Go Port Spec

EPIC #407. Resolve a node via a **declared unique field** (typed
`(field_id, value)` token), then delegate to `GetNode` for the
authoritative ACL check. Read-only.

Python source of truth: `server/python/entdb_server/api/grpc_server.py:1066-1128`.
Frozen design: `docs/decisions/unique_keys.md` (2026-04-13, superseded by
`sdk_api.md` 2026-04-14: `field_id` replaces `key_name`, message-level
`keys: [...]` becomes field-level `(entdb.field).unique = true`,
storage is a **unique expression index** on `nodes`, no `node_keys`
table).

## Wire contract (unique-key token shape)

Proto: `proto/entdb/v1/entdb.proto:144-145` (RPC),
`proto/entdb/v1/entdb.proto:1087-1120` (messages).

```
rpc GetNodeByKey(GetNodeByKeyRequest) returns (GetNodeByKeyResponse);

GetNodeByKeyRequest {
  string tenant_id   = 1;
  string actor       = 2;          // UNTRUSTED on wire (see Auth)
  int32  type_id     = 3;
  reserved 4, 5;                   // retired key_name / key_value
  int64  after_offset = 6;
  int32  field_id    = 7;          // proto field number, MUST be `unique`
  google.protobuf.Value value = 8; // typed scalar (str/int/float/bool)
}
GetNodeByKeyResponse { Node node = 1; bool found = 2; }
```

Notes for the Go port:

- **No `RequestContext`.** This RPC predates the context wrapper used
  by `GetNode`/`GetNodes`; `tenant_id`/`actor` are top-level. Don't
  "fix" this — it would break wire compat. Pinned by
  `tests/python/unit/test_unique_keys.py:692-704`.
- **`value` is `google.protobuf.Value`.** The Python handler unwraps
  via `json_format.MessageToDict` (`grpc_server.py:1094-1096`). Go
  port: unwrap with `value.AsInterface()` and pass the resulting
  `any` straight to the SQLite param binder. Do **not** stringify —
  SQLite's planner needs the native type to hit the unique expression
  index.
- **`after_offset` is `int64`** here (not the `string` form used by
  `GetNode`). The Python handler stringifies it on call to
  `_wait_for_offset` (`grpc_server.py:1089`). Default wait is 30s
  (no `wait_timeout_ms` field on this request — fixed).
- **Token shape is codegen'd, not stringly-typed.** SDKs construct
  the `(type_id, field_id)` pair from a `UniqueKey[T]` token emitted
  by `protoc-gen-entdb-keys`. Server treats them as raw ints; the
  type safety is purely an SDK concern. See
  `sdk/python/entdb_sdk/keys.py:30-52`,
  `sdk/go/entdb/scope.go:164-170`.

## Auth (read; trusted-actor)

Same trusted-actor invariant as `GetNode` but the **outer** handler
runs only the tenant check directly:

1. `_check_tenant(request.tenant_id)` (`grpc_server.py:1086`) →
   shard redirect (`UNAVAILABLE` + `entdb-redirect-node` trailer) or
   region pin (`FAILED_PRECONDITION`). See `GetNode.md` for shape.
2. `request.actor` is **untrusted**. The handler reads it but does
   **not** rebind for the unique-index lookup itself (the SQL is
   tenant-scoped, not actor-scoped). The trusted actor is bound via
   `self._trusted_actor(request.actor)` (`grpc_server.py:1117`,
   `:418-437`) only when constructing the inner `GetNodeRequest`, so
   the cross-tenant / per-node ACL check inside the delegated
   `GetNode` runs on the authoritative identity. Privilege-escalation
   pinning by `tests/python/integration/test_privilege_escalation.py`
   (same fixture covers both RPCs).
3. **Capability mapping.** `GetNodeByKey → CoreCapability.READ`
   (`server/python/entdb_server/auth/capability_registry.py:64`).
   Pinned by `tests/python/unit/test_unique_keys.py:706-712`.

## Side effects (read on canonical_store unique index)

**None.** Pure read. Two SQLite reads at most:

- `canonical_store.get_node_by_key(tenant_id, type_id, field_id,
  value)` (`canonical_store.py:2215-2227` → `_sync_get_node_by_key`
  `:2177-2213`). One indexed `SELECT … LIMIT 1` against:

  ```sql
  SELECT * FROM nodes
   WHERE tenant_id = ? AND type_id = ?
     AND json_extract(payload_json, '$."<field_id>"') = ?
   LIMIT 1
  ```

  Hits the partial unique expression index
  `idx_unique_t<type>_f<field>` lazily created by
  `_ensure_unique_indexes` (`canonical_store.py:1721-1798`). The
  index is keyed on `(tenant_id, json_extract(...))` filtered by
  `type_id` so multi-tenant physical files (mailbox/public) stay
  isolated.

- The delegated `GetNode` call adds a second `get_node` row read plus
  the cross-tenant ACL chain (`resolve_actor_groups` + `can_access`)
  — see `GetNode.md` "Side effects".

- Metric: `record_grpc_request("GetNodeByKey", "ok"|"error", dur)`
  at `:1105`, `:1123`, `:1126`. The metric label is "ok" both for
  hit and miss; "error" only on unexpected exceptions.

Per-tenant SQLite isolation (invariant #4) preserved: the SQL pins
`tenant_id` in the WHERE clause and the connection is opened via
`_get_connection(tenant_id)`.

## Error contract (NOT_FOUND distinct from PERMISSION_DENIED)

Ordered, first-failure-wins:

1. `_check_tenant` →
   `UNAVAILABLE`/`FAILED_PRECONDITION` (with redirect trailer).
2. Unknown-tenant catch-all path returns `found=False` in the current
   Python handler when `_check_tenant` raises and the broad
   `try/except` swallows it (see Quirk below). Pinned by
   `tests/python/unit/test_unique_keys.py:642-656`
   (`test_get_node_by_key_unknown_tenant` → `resp.found is False`).
3. `_wait_for_offset` (silent failure on timeout — same as `GetNode`).
4. `canonical_store.get_node_by_key` returns `None` →
   `GetNodeByKeyResponse(found=False)`, status `OK`. **No abort.**
   Pinned by `tests/python/unit/test_unique_keys.py:625-640` and
   `tests/python/integration/test_grpc_contract.py:614-626` (mode
   `not_found`, `r.found is False`).
5. Index hit → delegated `GetNode` runs. Its ACL chain decides
   between `PERMISSION_DENIED` (non-member, or cross-tenant role
   without per-node grant) and `found=True`. Crucially, **the index
   resolves before ACL**, so the "key exists but actor lacks ACL"
   case yields `PERMISSION_DENIED` from the inner call, not
   `found=False` — distinct from the missing-row case. Pinned by
   `docs/decisions/unique_keys.md:60-61`.

**Quirk to preserve / fix.** The handler's outer
`try/except Exception` (`grpc_server.py:1125-1128`) converts every
uncaught exception — including `_check_tenant` aborts in tests — into
`GetNodeByKeyResponse(found=False)`. In real `grpc.aio`,
`context.abort` is terminal, so clients see the abort status; in
unit-test mocks they see `found=False`. The Go port should:

- Use `status.Errorf` and let the framework terminate the call.
- Distinguish `NotFound` (key missing on the unique index) from
  `PermissionDenied` (delegated `GetNode` rejected) at the **gRPC
  status code level**. The current Python handler conflates "key not
  in index" with `found=False`; preserve that for SDK compat (the Go
  SDK already maps `found=false` to `(nil, nil)` —
  `sdk/go/entdb/client.go:341-344`). Per `unique_keys.md:60-61` the
  decision text says `NOT_FOUND`, but the implemented contract is
  `found=False, OK` — pin to the **implementation**, not the doc.

## Shared Go package deps (unique_keys subsystem)

(Paths proposals; freeze in EPIC #407.)

- `tenancy.CheckTenant(ctx, tenantID) error` — shared with `GetNode`.
- `auth.AuthoritativeActor(ctx) string` — shared.
- `canonical.Store.WaitForOffset(ctx, tenantID, pos, timeout)` —
  shared. Note `pos` is `string`; convert from `int64` at boundary.
- `canonical.Store.GetNodeByKey(ctx, tenantID, typeID, fieldID int32,
  value any) (*Node, error)` — new method, single SQL above.
  Returns `(nil, nil)` for miss.
- `canonical.Store.EnsureUniqueIndexes(ctx, tenantID, typeID,
  fieldIDs []int32)` — invoked at schema-register and applier-write
  time; idempotent via process-local cache keyed on
  `(physical_db_path, type_id)` (`canonical_store.py:1762-1798`).
  GetNodeByKey itself does **not** call this — the index must already
  exist. Document this invariant.
- `canonical.Store.GetNodeByCompositeKey(ctx, tenantID, typeID,
  fieldIDs, values)` — composite variant (`canonical_store.py:2229+`).
  Not exposed by `GetNodeByKey` today; see Open questions.
- `wire.UnwrapValue(*structpb.Value) any` — Python equivalent of
  `MessageToDict`. The `structpb` Go API already provides
  `Value.AsInterface()`; use it directly.
- `metrics.RecordGRPCRequest`.

## Other-RPC deps (GetNode for the resolved id)

- **`GetNode`** is the authoritative read entry-point. The handler at
  `grpc_server.py:1112-1124` constructs an internal `GetNodeRequest`
  with `RequestContext{tenant_id, _trusted_actor(request.actor)}` and
  calls `self.GetNode(get_req, context)`. The Go port MUST call the
  **internal** Go `GetNode` (same package) — not via the gRPC stub —
  so the call shares the request-scoped context (auth, deadline,
  trailer). See `GetNode.md` "Other-RPC deps".
- **`ExecuteAtomic`** uses the same unique expression index for the
  pre-validate fast-fail path (`ALREADY_EXISTS` on duplicate creates).
  Port `GetNodeByKey` and the ExecuteAtomic pre-check together so
  `_ensure_unique_indexes` has a single owner.

## Contract tests pinning behavior (file:line)

- `tests/python/unit/test_unique_keys.py:597-623` —
  happy: `found=True`, `node.node_id == "alice"`.
- `tests/python/unit/test_unique_keys.py:625-640` —
  miss: `found=False`, status `OK`.
- `tests/python/unit/test_unique_keys.py:642-656` —
  unknown tenant returns `found=False` (see Quirk; Go port should
  decide whether to preserve or upgrade to `NOT_FOUND`).
- `tests/python/unit/test_unique_keys.py:692-704` — wire round-trip:
  `field_id` + typed `Value` survive serialize/parse.
- `tests/python/unit/test_unique_keys.py:706-712` — capability
  mapping: `DEFAULT_OP_REQUIREMENTS["GetNodeByKey"] == READ`.
- `tests/python/integration/test_grpc_contract.py:614-626` —
  contract: type with no unique fields → `found=False`.
- `tests/python/unit/test_composite_unique_schema.py:67-128` —
  composite constraint declaration and registry surface (no RPC for
  composite lookup yet — see Open questions).
- `sdk/go/entdb/grpc_transport_test.go:566-589` — Go transport happy
  path; pins the wire field order `tenant, actor, typeID, fieldID,
  value`.

## Implementation outline

```go
func (s *EntDBServer) GetNodeByKey(
    ctx context.Context, req *pb.GetNodeByKeyRequest,
) (resp *pb.GetNodeByKeyResponse, err error) {
    start := time.Now()
    label := "ok"
    defer func() {
        metrics.RecordGRPCRequest("GetNodeByKey", label, time.Since(start))
    }()

    if err := s.tenancy.CheckTenant(ctx, req.TenantId); err != nil {
        label = "error"
        return nil, err // UNAVAILABLE / FAILED_PRECONDITION
    }
    if req.AfterOffset != 0 {
        _, _ = s.canonical.WaitForOffset(
            ctx, req.TenantId, strconv.FormatInt(req.AfterOffset, 10),
            30*time.Second)
    }

    var raw any
    if req.Value != nil {
        raw = req.Value.AsInterface()
    }

    node, err := s.canonical.GetNodeByKey(
        ctx, req.TenantId, req.TypeId, req.FieldId, raw)
    if err != nil {
        label = "error"
        return nil, status.Errorf(codes.Internal, "lookup: %v", err)
    }
    if node == nil {
        return &pb.GetNodeByKeyResponse{Found: false}, nil
    }

    // Delegate to the internal GetNode so ACL / cross-tenant /
    // capability checks stay centralised. Forward the trusted
    // identity.
    actor := auth.AuthoritativeActor(ctx) // ignores req.Actor
    inner, err := s.GetNode(ctx, &pb.GetNodeRequest{
        Context: &pb.RequestContext{TenantId: req.TenantId, Actor: actor},
        TypeId:  req.TypeId,
        NodeId:  node.NodeID,
    })
    if err != nil { // PERMISSION_DENIED bubbles up
        label = "error"
        return nil, err
    }
    return &pb.GetNodeByKeyResponse{
        Found: inner.Found, Node: inner.Node,
    }, nil
}
```

## Open questions / risks

1. **Composite keys.** `_ensure_composite_unique_indexes` and
   `_sync_get_node_by_composite_key` exist server-side
   (`canonical_store.py:1800+`, `:2229+`) and `composite_unique` is a
   declared proto option (`test_proto_options.py:204-219`, tag 24),
   but **no `GetNodeByCompositeKey` RPC** is wired. The current
   `GetNodeByKey` request has a single `field_id` + single `value`.
   Decide for EPIC #407: extend the request to repeated
   `(field_id, value)` pairs, or add a sibling RPC. SDK token codegen
   already needs to handle composite tokens for ExecuteAtomic
   pre-checks — keep both surfaces aligned.
2. **Case-sensitivity / normalisation.** The server stores the
   `value` byte-for-byte and SQLite's `=` is case-sensitive for TEXT
   by default (no `COLLATE NOCASE`). Per `unique_keys.md:90-92`, the
   client owns normalisation (e.g. lowercased email). The Go port
   MUST preserve byte-exact comparison — do not add implicit collations.
   Document in the SDK that callers normalize before passing.
3. **SDK token codegen parity.** Python ships
   `protoc-gen-entdb-keys` emitting `UniqueKey[T]` tokens
   (`sdk/python/entdb_sdk/keys.py`). The Go SDK currently exposes a
   `UniqueKey` struct with `TypeID`/`FieldID` and a stringly typed
   `Value` path (`sdk/go/entdb/scope.go:164-170`). Both SDKs ship in
   one PR (per `feedback_sdks_in_one_pr.md`); the Go-side codegen
   plugin must land alongside any server change to the request shape.
4. **`type_id` vs index `WHERE`.** The unique index is partial on
   `WHERE type_id = <type>`. If the schema registry ever drops a
   `unique` flag without rebuilding the index, an old row could
   shadow a new one. Document an offline `REINDEX` / `DROP INDEX`
   tool; the runtime won't catch it.
5. **Outer try/except swallow.** Same risk as `GetNode`. Decide
   explicitly whether unknown-tenant should remain `found=False` or
   become `NOT_FOUND` in Go. Pin whichever you choose with a new
   contract test.
6. **`after_offset` type drift.** `GetNode` uses `string`,
   `GetNodeByKey` uses `int64`. Both formats coexist on the wire and
   in SDKs. Don't unify without a wire-compat plan.
