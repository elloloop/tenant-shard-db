// SPDX-License-Identifier: AGPL-3.0-only

// W2 — ExecuteAtomic RPC (Python -> Go server port, EPIC #407).
//
// Spec: docs/go-port/rpcs/ExecuteAtomic.md. This is the central write
// path for the entire system: every mutation enters via this handler,
// is appended to the WAL, and is later materialised into per-tenant
// SQLite by the applier. Source-of-truth Python:
// server/python/entdb_server/api/grpc_server.py:620-828.
//
// CLAUDE.md invariants enforced here:
//
//	#1 — All writes go through the WAL. The handler MUST NOT touch
//	     SQLite for mutations; producer.Append is the single ingress.
//	#6 — Field IDs, not names, on disk. The wire-side Struct payloads
//	     for create_node.data / update_node.patch are translated to
//	     id-keyed maps via payload.StructToPayload before WAL append.
//
// Key behaviours (pinned by the Python contract tests, see spec §
// "Contract tests"):
//
//   - The wire-claimed actor (req.context.actor) is UNTRUSTED. The
//     handler resolves the trusted identity via auth.Authoritative and
//     immediately rebinds the local variable; the persisted WAL event
//     records the trusted actor only. Privilege-escalation pin.
//   - Schema-fingerprint mismatch is in-band: success=false,
//     error_code="SCHEMA_MISMATCH", gRPC status remains OK. NOT an
//     abort. Pinned by test_grpc_schema_mismatch_metric.py.
//   - WAL append failure is in-band: success=false,
//     error_code="INTERNAL", gRPC status remains OK.
//   - Empty create_node.id is server-filled with a UUIDv4 BEFORE WAL
//     append so replays read the same id off the log (determinism).
//   - The receipt is built post-append and returned with
//     applied_status=PENDING; if wait_applied is true, we block on
//     store.WaitForOffset until the applier catches up or the timeout
//     elapses, at which point applied_status flips to APPLIED.

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	"github.com/elloloop/tenant-shard-db/server/go/internal/payload"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

const (
	executeAtomicMethod    = "ExecuteAtomic"
	executeAtomicDefaultMs = 30_000

	// Internal op-string constants. Mirror the Python applier dispatch
	// keys at apply/applier.py:929-1248. Kept local rather than imported
	// from apply to avoid pulling that package's heavyweight deps into
	// the api binary just for two strings.
	opCreateNode  = "create_node"
	opUpdateNode  = "update_node"
	opDeleteNode  = "delete_node"
	opDeleteWhere = "delete_where"
	opCreateEdge  = "create_edge"
	opDeleteEdge  = "delete_edge"
)

// ExecuteAtomic implements entdb.v1.EntDBService/ExecuteAtomic.
func (s *Server) ExecuteAtomic(ctx context.Context, req *pb.ExecuteAtomicRequest) (*pb.ExecuteAtomicResponse, error) {
	start := time.Now()
	outcome := "ok"
	defer func() { metrics.RecordGRPCRequest(executeAtomicMethod, outcome, time.Since(start)) }()

	if s.producer == nil || s.topic == "" {
		outcome = "error"
		return nil, errs.Errorf(codes.Unimplemented, "ExecuteAtomic: WAL producer not configured")
	}

	rctx := req.GetContext()
	tenantID := rctx.GetTenantId()

	// Tenant gate: sharding ownership, existence, region pin.
	if err := s.checkTenant(ctx, tenantID); err != nil {
		outcome = "error"
		return nil, err
	}

	// Required-field validation. Order matches Python at grpc_server.py:632-637.
	if tenantID == "" {
		outcome = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "tenant_id is required")
	}
	if rctx.GetActor() == "" {
		outcome = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "actor is required")
	}
	if len(req.GetOperations()) == 0 {
		outcome = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "operations list is empty")
	}

	// Trusted-actor chokepoint. The wire-claimed actor is fed in as the
	// fallback for the no-interceptor path; when the AuthInterceptor
	// populated ctx, that identity wins regardless. Rebind the local var
	// — never re-read req.GetContext().GetActor() past this line.
	// (PR #168 invariant; pinned by test_privilege_escalation.py:266,287.)
	trustedActor := auth.Authoritative(ctx, auth.ParseActor(rctx.GetActor()))

	// Idempotency key — server-generated UUIDv4 if empty
	// (grpc_server.py:648). Generated here so retries in the in-band
	// INTERNAL path can still observe a stable receipt.
	idem := req.GetIdempotencyKey()
	if idem == "" {
		idem = uuid.NewString()
	}

	// Schema fingerprint check. In-band failure when the request claims a
	// fingerprint that disagrees with the server's. NOT an abort.
	// Pinned by test_grpc_schema_mismatch_metric.py:51-90.
	srvFP := ""
	if s.registry != nil {
		srvFP = s.registry.Fingerprint()
	}
	if reqFP := req.GetSchemaFingerprint(); reqFP != "" && srvFP != "" && reqFP != srvFP {
		outcome = "error"
		return &pb.ExecuteAtomicResponse{
			Success:   false,
			Error:     fmt.Sprintf("Schema mismatch: server has %s", srvFP),
			ErrorCode: "SCHEMA_MISMATCH",
		}, nil
	}

	// Translate proto operations to internal id-keyed shape. This is the
	// CLAUDE.md invariant #6 chokepoint: name-keyed Structs become
	// id-keyed payload maps before they reach the WAL.
	ops, err := s.convertOperations(req.GetOperations())
	if err != nil {
		outcome = "error"
		return nil, err
	}

	// Tenant-membership + role + status check. Best-effort: skipped when
	// no globalstore is wired (single-node bring-up / no-auth tests),
	// matching the Python `if self.global_store is not None` guard.
	hasDelete := false
	for _, op := range ops {
		switch op["op"].(string) {
		case opDeleteNode, opDeleteEdge, opDeleteWhere:
			hasDelete = true
		}
	}
	if err := s.checkTenantWriteAccess(ctx, tenantID, trustedActor, hasDelete); err != nil {
		outcome = "error"
		return nil, err
	}

	// Server-fill empty create_node IDs, collect created_node_ids in
	// op order, and resolve in-transaction "$alias" references on edge
	// from/to so the WAL event only ever carries concrete node ids.
	//
	// Why resolve at ingress (deviation from Python, which resolves at
	// apply time): the WAL is the audit-of-record; a bare UUID is a
	// stable, unambiguous identity, whereas "$alias" is meaningful only
	// in the context of one in-flight transaction. Resolving here keeps
	// the applier dispatch table free of map-shape branching for the
	// alias_ref payload and prevents alias bugs from poisoning the
	// consumer loop (the original symptom: "poison event: create_edge
	// missing from/to" halted the applier for every subsequent RPC).
	//
	// Ordering: we walk ops in declared order. Each create_node with an
	// "as" field publishes its (server-filled) id into the alias map;
	// subsequent edge ops referencing that alias resolve against the
	// map. A reference to an alias that has not been previously defined
	// in the SAME transaction is an INVALID_ARGUMENT — matches the
	// Python applier's lookup miss (which would surface as a poison
	// event today). Aliases never span transactions.
	createdNodeIDs := make([]string, 0)
	aliasMap := make(map[string]string)
	for _, op := range ops {
		switch op["op"].(string) {
		case opCreateNode:
			id, _ := op["id"].(string)
			if id == "" {
				id = uuid.NewString()
				op["id"] = id
			}
			createdNodeIDs = append(createdNodeIDs, id)
			if alias, _ := op["as"].(string); alias != "" {
				aliasMap[alias] = id
			}
		case opCreateEdge, opDeleteEdge:
			resolved, err := resolveEdgeRef(op["from"], aliasMap)
			if err != nil {
				outcome = "error"
				return nil, errs.Errorf(codes.InvalidArgument,
					"%s: from: %v", op["op"], err)
			}
			op["from"] = resolved
			resolved, err = resolveEdgeRef(op["to"], aliasMap)
			if err != nil {
				outcome = "error"
				return nil, errs.Errorf(codes.InvalidArgument,
					"%s: to: %v", op["op"], err)
			}
			op["to"] = resolved
		}
	}

	// Build the WAL event. The persisted actor is the trusted identity,
	// not the wire claim — audit-truth invariant (grpc_server.py:780).
	event := wal.Event{
		TenantID:          tenantID,
		Actor:             trustedActor.String(),
		IdempotencyKey:    idem,
		SchemaFingerprint: srvFP,
		TsMs:              time.Now().UnixMilli(),
		Ops:               ops,
	}
	payloadBytes, err := json.Marshal(event)
	if err != nil {
		outcome = "error"
		return &pb.ExecuteAtomicResponse{
			Success:   false,
			Error:     fmt.Sprintf("encode event: %v", err),
			ErrorCode: "INTERNAL",
		}, nil
	}

	// Append to WAL. Partition key = tenant_id so all of a tenant's
	// events land on one partition (in-order replay).
	headers := map[string][]byte{
		wal.HeaderIdempotencyKey: []byte(idem),
	}
	pos, err := s.producer.Append(ctx, s.topic, tenantID, payloadBytes, headers)
	if err != nil {
		// In-band INTERNAL channel — gRPC status stays OK, error_code
		// signals failure. Mirrors grpc_server.py:818-825.
		outcome = "error"
		return &pb.ExecuteAtomicResponse{
			Success:   false,
			Error:     err.Error(),
			ErrorCode: "INTERNAL",
		}, nil
	}

	posStr := streamPosString(pos)
	receipt := &pb.Receipt{
		TenantId:       tenantID,
		IdempotencyKey: idem,
		StreamPosition: posStr,
	}

	// Optional apply-wait. Default 30s when wait_timeout_ms is zero
	// (grpc_server.py:805). Honour ctx cancellation.
	appliedStatus := pb.ReceiptStatus_RECEIPT_STATUS_PENDING
	var preFailure *pb.PreconditionFailure
	if req.GetWaitApplied() && posStr != "" && s.store != nil {
		timeoutMs := int32(executeAtomicDefaultMs)
		if v := req.GetWaitTimeoutMs(); v > 0 {
			timeoutMs = v
		}
		waitCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		if err := s.store.WaitForOffset(waitCtx, tenantID, parseStreamOffset(posStr)); err == nil {
			// Poll briefly for the idempotency row to be present.
			// WaitForOffset's in-memory tracker is advanced from
			// UpdateAppliedOffsetTx BEFORE the applier's batch txn
			// commits (see store/offset.go: the cond broadcast is
			// pre-commit), so the wait returning does NOT guarantee
			// THIS event's applied_events row is committed/visible on
			// a fresh read connection. Polling the idempotency record
			// closes that residual race without changing the wait
			// semantics the rest of the codebase relies on.
			//
			// NOTE (GitHub issue #505): the original trigger — the
			// contract seed pre-bumping the in-memory offset via a
			// WAL-bypassing CreateNodeRaw — has been removed; the seed
			// now goes through the real producer→applier path. This
			// poll is retained because the pre-commit broadcast above
			// is an independent source of the same race and is not
			// addressed by the seed fix. GitHub issues #500, #503, #505.
			rec := waitForIdempotencyRecord(ctx, s.store, tenantID, idem, time.Duration(timeoutMs)*time.Millisecond)
			switch {
			case rec.Present && rec.Status == store.IdempotencyStatusFailedPrecondition:
				appliedStatus = pb.ReceiptStatus_RECEIPT_STATUS_FAILED_PRECONDITION
				preFailure = decodePreconditionFailureJSON(rec.FailureJSON)
			default:
				appliedStatus = pb.ReceiptStatus_RECEIPT_STATUS_APPLIED
			}
		}
		cancel()
	}

	resp := &pb.ExecuteAtomicResponse{
		Success:             appliedStatus != pb.ReceiptStatus_RECEIPT_STATUS_FAILED_PRECONDITION,
		Receipt:             receipt,
		CreatedNodeIds:      createdNodeIDs,
		AppliedStatus:       appliedStatus,
		PreconditionFailure: preFailure,
	}
	// On a CAS miss the response carries `success=false` so the SDK
	// can branch on the wire-level field without inspecting the
	// applied_status enum. The gRPC status code stays OK — the
	// failure is in-band so it composes with the existing
	// idempotency-cache path. The structured detail rides on the
	// precondition_failure field.
	if appliedStatus == pb.ReceiptStatus_RECEIPT_STATUS_FAILED_PRECONDITION {
		resp.Error = "precondition failed"
		resp.ErrorCode = "FAILED_PRECONDITION"
	}
	return resp, nil
}

// waitForIdempotencyRecord polls for the applied_events row keyed by
// (tenantID, idem) to become Present, with a short ceiling. Returns
// the (possibly still-absent) record at the end of the budget; the
// caller decides how to interpret absence. Used by ExecuteAtomic's
// wait_applied path to close a race between WaitForOffset returning
// and the applier finishing its write — see the issue #500 fix note
// at the call site.
func waitForIdempotencyRecord(ctx context.Context, st *store.CanonicalStore, tenantID, idem string, budget time.Duration) store.IdempotencyRecord {
	if budget <= 0 {
		budget = time.Second
	}
	deadline := time.Now().Add(budget)
	// Tight initial spin (≤5ms) for the common case where the
	// applier finished microseconds before the handler queried.
	delay := 1 * time.Millisecond
	for {
		rec, err := st.CheckIdempotencyStatus(ctx, tenantID, idem)
		if err == nil && rec.Present {
			return rec
		}
		if time.Now().After(deadline) {
			return rec
		}
		select {
		case <-ctx.Done():
			return rec
		case <-time.After(delay):
		}
		if delay < 25*time.Millisecond {
			delay *= 2
		}
	}
}

// decodePreconditionFailureJSON parses the failure_json column blob
// the applier writes and returns the wire-level proto message. Any
// decode failure collapses to nil — the typed status code still
// signals the CAS miss; the SDK will fall back to a non-detailed
// error when the detail is missing.
func decodePreconditionFailureJSON(blob string) *pb.PreconditionFailure {
	if blob == "" {
		return nil
	}
	var pf struct {
		OpIndex      int    `json:"OpIndex"`
		Field        string `json:"Field"`
		Expected     any    `json:"Expected"`
		Observed     any    `json:"Observed"`
		FieldPresent bool   `json:"FieldPresent"`
	}
	if err := json.Unmarshal([]byte(blob), &pf); err != nil {
		return nil
	}
	out := &pb.PreconditionFailure{
		OpIndex: int32(pf.OpIndex),
		Field:   pf.Field,
	}
	if v, err := structpb.NewValue(pf.Expected); err == nil {
		out.Expected = v
	}
	// Field-absent reads back as observed=nil with FieldPresent=false;
	// surface that as a NullValue Value so callers can distinguish the
	// two cases by inspecting the proto.
	if v, err := structpb.NewValue(pf.Observed); err == nil {
		out.Observed = v
	}
	return out
}

// streamPosString renders a wal.StreamPos in the wire form the client
// expects. Returns "" when pos is the zero value (no record was
// written) — matches the Python `str(stream_pos) if stream_pos else ""`
// behaviour at grpc_server.py:799.
func streamPosString(pos wal.StreamPos) string {
	if pos == (wal.StreamPos{}) {
		return ""
	}
	return pos.String()
}

// convertOperations translates wire Operations to internal id-keyed
// op dicts. The schema-aware translation lives in
// payload.StructToPayload; everything else is mechanical proto-to-map
// boilerplate matching grpc_server.py:_convert_operations.
func (s *Server) convertOperations(operations []*pb.Operation) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(operations))

	for _, op := range operations {
		switch v := op.GetOp().(type) {
		case *pb.Operation_CreateNode:
			create := v.CreateNode
			if create.GetTypeId() == 0 {
				return nil, errs.Errorf(codes.InvalidArgument,
					"create_node: type_id is required")
			}
			data, err := s.translatePayload(create.GetTypeId(), create.GetData())
			if err != nil {
				return nil, err
			}
			internal := map[string]any{
				"op":      opCreateNode,
				"type_id": int(create.GetTypeId()),
				"data":    data,
			}
			if id := create.GetId(); id != "" {
				internal["id"] = id
			}
			if acl := convertACL(create.GetAcl()); len(acl) > 0 {
				internal["acl"] = acl
			}
			if alias := create.GetAs(); alias != "" {
				internal["as"] = alias
			}
			if fanout := create.GetFanoutTo(); len(fanout) > 0 {
				internal["fanout_to"] = fanout
			}
			if name := storageModeName(create.GetStorageMode()); name != "TENANT" {
				internal["storage_mode"] = name
			}
			if tgt := create.GetTargetUserId(); tgt != "" {
				internal["target_user_id"] = tgt
			}
			out = append(out, internal)

		case *pb.Operation_UpdateNode:
			upd := v.UpdateNode
			if upd.GetTypeId() == 0 {
				return nil, errs.Errorf(codes.InvalidArgument,
					"update_node: type_id is required")
			}
			if upd.GetId() == "" {
				return nil, errs.Errorf(codes.InvalidArgument,
					"update_node: id is required")
			}
			patch, err := s.translatePayload(upd.GetTypeId(), upd.GetPatch())
			if err != nil {
				return nil, err
			}
			internal := map[string]any{
				"op":      opUpdateNode,
				"type_id": int(upd.GetTypeId()),
				"id":      upd.GetId(),
				"patch":   patch,
			}
			// GitHub issue #500 / #525 — optional CAS precondition.
			// Modern SDKs send the stable field_id, with field kept
			// only for diagnostics. Legacy name-only requests can
			// still be resolved when a schema registry is configured.
			if pre := upd.GetPrecondition(); pre != nil {
				resolved, err := s.translatePrecondition(upd.GetTypeId(), pre)
				if err != nil {
					return nil, err
				}
				internal["precondition"] = resolved
			}
			out = append(out, internal)

		case *pb.Operation_DeleteNode:
			del := v.DeleteNode
			if del.GetTypeId() == 0 {
				return nil, errs.Errorf(codes.InvalidArgument,
					"delete_node: type_id is required")
			}
			if del.GetId() == "" {
				return nil, errs.Errorf(codes.InvalidArgument,
					"delete_node: id is required")
			}
			out = append(out, map[string]any{
				"op":      opDeleteNode,
				"type_id": int(del.GetTypeId()),
				"id":      del.GetId(),
			})

		case *pb.Operation_DeleteWhere:
			dw := v.DeleteWhere
			if dw.GetTypeId() == 0 {
				return nil, errs.Errorf(codes.InvalidArgument,
					"delete_where: type_id is required")
			}
			if len(dw.GetWhere()) == 0 {
				// An unconditional bulk delete is too dangerous to
				// express implicitly (issue #504). Callers that truly
				// want to clear a type pass an explicit predicate.
				return nil, errs.Errorf(codes.InvalidArgument,
					"delete_where: at least one `where` predicate is required")
			}
			// Resolve the developer-facing FieldFilter names to stable
			// field_ids using the EXACT same path QueryNodes uses
			// (issue #501) so the predicate is schema-less on the WAL.
			//
			// Schema-OPTIONAL, exactly like QueryNodes (issue #545): a
			// nil registry is the server's schema-less mode and is
			// tolerated here just as it is in query_nodes.go. When the
			// registry is nil, typeName stays "" and
			// fieldFiltersToStoreFilters -> payload.FilterNamesToIDs
			// takes its schema-less branch: digit-only FieldFilter.field
			// keys parse to raw payload field ids, and a non-digit field
			// NAME surfaces as INVALID_ARGUMENT with the same wording as
			// the QueryNodes path ("payload: cannot translate filter key
			// %q without a schema"). Do NOT add a hard reject for a
			// missing schema here — that is the exact bug #545 fixes and
			// would diverge from the QueryNodes contract. Schema-mode
			// behaviour (registry configured) is unchanged: an unknown
			// type_id is still INVALID_ARGUMENT.
			typeName := ""
			if s.registry != nil {
				nt := s.registry.NodeTypeByID(dw.GetTypeId())
				if nt == nil {
					return nil, errs.Errorf(codes.InvalidArgument,
						"delete_where: unknown type_id %d", dw.GetTypeId())
				}
				typeName = nt.Name
			}
			storeFilters, err := s.fieldFiltersToStoreFilters(typeName, dw.GetWhere())
			if err != nil {
				return nil, err
			}
			where := make([]any, 0, len(storeFilters))
			for _, f := range storeFilters {
				where = append(where, map[string]any{
					"field_id": int(f.FieldID),
					"op":       storeFilterOpToToken(f.Op),
					"value":    f.Value,
				})
			}
			out = append(out, map[string]any{
				"op":      opDeleteWhere,
				"type_id": int(dw.GetTypeId()),
				"where":   where,
				"limit":   int(dw.GetLimit()),
			})

		case *pb.Operation_CreateEdge:
			ce := v.CreateEdge
			if ce.GetEdgeId() == 0 {
				return nil, errs.Errorf(codes.InvalidArgument,
					"create_edge: edge_id is required")
			}
			out = append(out, map[string]any{
				"op":      opCreateEdge,
				"edge_id": int(ce.GetEdgeId()),
				"from":    convertNodeRef(ce.GetFrom()),
				"to":      convertNodeRef(ce.GetTo()),
				"props":   structToMap(ce.GetProps()),
			})

		case *pb.Operation_DeleteEdge:
			de := v.DeleteEdge
			if de.GetEdgeId() == 0 {
				return nil, errs.Errorf(codes.InvalidArgument,
					"delete_edge: edge_id is required")
			}
			out = append(out, map[string]any{
				"op":      opDeleteEdge,
				"edge_id": int(de.GetEdgeId()),
				"from":    convertNodeRef(de.GetFrom()),
				"to":      convertNodeRef(de.GetTo()),
			})
		}
	}
	return out, nil
}

// translatePrecondition resolves the wire-side UpdateNodePrecondition
// to the stable field_id used on disk and returns the internal op-dict
// payload consumed by the applier. Modern SDKs send field_id directly;
// the legacy name-only path is retained only for registry-backed direct
// wire clients. The "equals" value is unboxed from the *structpb.Value
// envelope at the boundary so the applier compares raw Go scalars
// rather than wrapped proto values; the wire-side nullability is
// preserved (nil → field-absence match).
//
// Error semantics:
//
//   - invalid field_id          → INVALID_ARGUMENT
//   - name-only without registry → INVALID_ARGUMENT
//   - unknown type_id/name       → INVALID_ARGUMENT (registry-backed fallback)
//
// See GitHub issues #500 and #525.
func (s *Server) translatePrecondition(typeID int32, pre *pb.UpdateNodePrecondition) (map[string]any, error) {
	// structpb.Value.AsInterface() unboxes scalars to Go-native types
	// (float64 for numbers, string, bool, nil for NullValue, []any /
	// map[string]any for compounds) — exactly the shape the applier's
	// reflect.DeepEqual comparator wants.
	var equals any
	if eq := pre.GetEquals(); eq != nil {
		equals = eq.AsInterface()
	}
	if fid := pre.GetFieldId(); fid != 0 {
		if fid < 0 || fid > 65535 {
			return nil, errs.Errorf(codes.InvalidArgument,
				"update_node: precondition.field_id must be 1-65535, got %d", fid)
		}
		name := pre.GetField()
		if name == "" {
			name = fmt.Sprintf("%d", fid)
		}
		return map[string]any{
			"field":    name, // human-readable, surfaced on failure detail
			"field_id": fmt.Sprintf("%d", fid),
			"equals":   equals,
		}, nil
	}
	name := pre.GetField()
	if name == "" {
		return nil, errs.Errorf(codes.InvalidArgument,
			"update_node: precondition.field_id is required")
	}
	if s.registry == nil {
		return nil, errs.Errorf(codes.InvalidArgument,
			"update_node: precondition.field_id is required when schema registry is not configured")
	}
	fid, ok := s.registry.FieldIDByNameForType(typeID, name)
	if !ok {
		return nil, errs.Errorf(codes.InvalidArgument,
			"update_node: precondition references unknown field %q on type_id=%d",
			name, typeID)
	}
	return map[string]any{
		"field":    name, // human-readable, surfaced on failure detail
		"field_id": fmt.Sprintf("%d", fid),
		"equals":   equals,
	}, nil
}

// translatePayload runs name->id translation for a create_node.data /
// update_node.patch struct. The schema-less / unknown-type-id path is a
// digit-key passthrough (matches Python's lazy schema fallthrough).
//
// The result is stored under the op's "data"/"patch" key as
// map[string]any with stringified field-id keys — that's the shape the
// applier (which JSON-decodes events from the WAL) consumes.
func (s *Server) translatePayload(typeID int32, st *structpb.Struct) (map[string]any, error) {
	typeName := ""
	if s.registry != nil {
		if nt := s.registry.NodeTypeByID(typeID); nt != nil {
			typeName = nt.Name
		}
	}
	idKeyed, err := payload.StructToPayload(s.registry, typeName, st)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any, len(idKeyed))
	for fid, val := range idKeyed {
		out[fmt.Sprintf("%d", fid)] = val
	}
	return out, nil
}

// convertACL maps the wire AclEntry slice to the internal list-of-dicts
// the applier expects. Mirrors _acl_proto_to_list in the Python handler.
func convertACL(in []*pb.AclEntry) []any {
	if len(in) == 0 {
		return nil
	}
	out := make([]any, 0, len(in))
	for _, e := range in {
		principal := e.GetGrantee()
		if principal == "" {
			principal = e.GetPrincipal()
		}
		entry := map[string]any{
			"principal":  principal,
			"permission": e.GetPermission(),
		}
		out = append(out, entry)
	}
	return out
}

// resolveEdgeRef collapses the three on-wire ref shapes — bare id
// string, {"ref": "$alias"}, and {"type_id":N, "id":"X"} — down to a
// single bare-string node id, using the per-transaction alias map.
//
// Shape contract (must match convertNodeRef output):
//
//   - string ""          → missing ref; surfaced as INVALID_ARGUMENT.
//   - string "id"        → returned as-is (bare-id; no $-prefix).
//   - {"ref":"$a"}       → resolved against aliasMap; unresolved → error.
//     Supports "$a" and "$a.id" (Python parity, see
//     applier.py:_resolve_ref); only the dotted
//     prefix before the first "." is the alias name.
//   - {"type_id":N,"id"} → returned as the id string (typed lookup is a
//     read-time concern, not a write-time one).
//
// Any other shape is treated as a missing ref. The caller wraps the
// returned error in an InvalidArgument so the gRPC client sees a
// stable status code instead of a poisoned applier loop.
func resolveEdgeRef(raw any, aliasMap map[string]string) (string, error) {
	switch v := raw.(type) {
	case nil:
		return "", fmt.Errorf("missing node reference")
	case string:
		if v == "" {
			return "", fmt.Errorf("missing node reference")
		}
		if strings.HasPrefix(v, "$") {
			// Bare "$alias" string is the Python on-the-wire
			// short-form some SDKs emit when bypassing NodeRef.
			// Treat it identically to {"ref":"$alias"}.
			return resolveAlias(v, aliasMap)
		}
		return v, nil
	case map[string]any:
		if refStr, ok := v["ref"].(string); ok && refStr != "" {
			return resolveAlias(refStr, aliasMap)
		}
		if idStr, ok := v["id"].(string); ok && idStr != "" {
			// Typed-ref carries an explicit id; the applier will
			// look it up by (type_id, id). We forward only the id
			// string downstream because the legacy edge op shape
			// expects a bare string. (Typed-ref+missing-id is a
			// separate validation in the applier.)
			return idStr, nil
		}
		return "", fmt.Errorf("malformed node reference")
	default:
		return "", fmt.Errorf("malformed node reference (type %T)", raw)
	}
}

// resolveAlias looks up "$alias" or "$alias.id" against the per-
// transaction alias map. Returns INVALID_ARGUMENT-shaped error when
// the alias was never published by an earlier create_node in this
// transaction. Mirrors applier.py:_resolve_ref's "." split.
func resolveAlias(ref string, aliasMap map[string]string) (string, error) {
	if !strings.HasPrefix(ref, "$") {
		// Defensive: caller already type-switched, but if we ever
		// reach here with a bare id, pass it through.
		return ref, nil
	}
	name := ref[1:]
	if i := strings.IndexByte(name, '.'); i >= 0 {
		name = name[:i]
	}
	if name == "" {
		return "", fmt.Errorf("alias reference %q has empty name", ref)
	}
	id, ok := aliasMap[name]
	if !ok {
		return "", fmt.Errorf("unresolved alias %q (no create_node with as=%q earlier in this transaction)", ref, name)
	}
	return id, nil
}

// convertNodeRef mirrors grpc_server.py:_convert_node_ref. The
// applier's create_edge / delete_edge handlers consume one of three
// shapes: a bare id string, {"ref": <alias>}, or
// {"type_id": N, "id": "X"}.
func convertNodeRef(ref *pb.NodeRef) any {
	if ref == nil {
		return nil
	}
	switch r := ref.GetRef().(type) {
	case *pb.NodeRef_Id:
		return r.Id
	case *pb.NodeRef_AliasRef:
		return map[string]any{"ref": r.AliasRef}
	case *pb.NodeRef_Typed:
		return map[string]any{
			"type_id": int(r.Typed.GetTypeId()),
			"id":      r.Typed.GetId(),
		}
	}
	return nil
}

// storageModeName mirrors the proto-enum -> internal-string map at
// grpc_server.py:865-871.
func storageModeName(m pb.StorageMode) string {
	switch m {
	case pb.StorageMode_STORAGE_MODE_USER_MAILBOX:
		return "USER_MAILBOX"
	case pb.StorageMode_STORAGE_MODE_PUBLIC:
		return "PUBLIC"
	default:
		return "TENANT"
	}
}

// structToMap is the lightweight egress for edge props (which are NOT
// field-id-translated — edges store free-form properties, not declared
// schema fields). Returns nil for a nil Struct so the JSON-encoded WAL
// event omits the field cleanly.
func structToMap(s *structpb.Struct) map[string]any {
	if s == nil || len(s.GetFields()) == 0 {
		return nil
	}
	return s.AsMap()
}

// checkTenantWriteAccess enforces the membership + role + tenant-status
// gates the Python handler runs at grpc_server.py:758. Skips when no
// globalstore is wired or the actor is system/admin (which bypass
// per grpc_server.py:498-499).
//
// The gate is finer-grained than CheckTenant: where CheckTenant only
// asserts existence + ownership + region, this asserts the caller is
// allowed to WRITE to this tenant in its current status.
func (s *Server) checkTenantWriteAccess(ctx context.Context, tenantID string, actor auth.Actor, hasDelete bool) error {
	if s.global == nil {
		return nil
	}
	// system: / admin: actors bypass tenant-membership checks.
	if actor.IsSystem() || actor.IsAdmin() {
		return nil
	}

	t, err := s.global.GetTenant(ctx, tenantID)
	if err != nil {
		return errs.Internal(ctx, "tenant lookup", err)
	}
	if t == nil {
		return errs.Errorf(codes.NotFound, "tenant %q does not exist", tenantID)
	}
	switch t.Status {
	case "deleted":
		return errs.Errorf(codes.NotFound, "tenant %q does not exist", tenantID)
	case "archived":
		return errs.Errorf(codes.FailedPrecondition,
			"tenant %q is archived; writes are disabled", tenantID)
	case "legal_hold":
		if hasDelete {
			return errs.Errorf(codes.FailedPrecondition,
				"tenant %q is on legal hold; deletes are disabled", tenantID)
		}
	}

	if !actor.IsUser() {
		// Group / unknown actor kinds aren't valid callers — the Python
		// gate rejects these at the auth layer; here we treat them as
		// non-members for defensive parity.
		return errs.Errorf(codes.PermissionDenied,
			"actor %q is not a member of tenant %q", actor.String(), tenantID)
	}

	members, err := s.global.GetTenantMembers(ctx, tenantID)
	if err != nil {
		return errs.Internal(ctx, "membership lookup", err)
	}
	var role string
	for _, m := range members {
		if m.UserID == actor.ID() {
			role = m.Role
			break
		}
	}
	if role == "" {
		return errs.Errorf(codes.PermissionDenied,
			"actor %q is not a member of tenant %q", actor.String(), tenantID)
	}
	switch role {
	case "viewer", "guest":
		return errs.Errorf(codes.PermissionDenied,
			"actor %q has role %q; writes require member/admin/owner",
			actor.String(), role)
	}
	return nil
}
