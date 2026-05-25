// SPDX-License-Identifier: AGPL-3.0-only

// ExecuteAtomic RPC.
//
// Spec: docs/go-port/rpcs/ExecuteAtomic.md. This is the central write
// path for the entire system: every mutation enters via this handler,
// is appended to the WAL, and is later materialised into per-tenant
// SQLite by the applier.
//
// CLAUDE.md invariants enforced here:
//
//	#1 — All writes go through the WAL. The handler MUST NOT touch
//	     SQLite for mutations; producer.Append is the single ingress.
//	#6 — Field IDs, not names, on disk. The wire-side Struct payloads
//	     for create_node.data / update_node.patch are translated to
//	     id-keyed maps via payload.StructToPayload before WAL append.
//
// Key behaviours (pinned by contract tests, see spec § "Contract tests"):
//
//   - The wire-claimed actor (req.context.actor) is UNTRUSTED. The
//     handler resolves the trusted identity via auth.Authoritative and
//     immediately rebinds the local variable; the persisted WAL event
//     records the trusted actor only. Privilege-escalation pin.
//   - Schema-fingerprint mismatch is in-band: success=false,
//     error_code="SCHEMA_MISMATCH", gRPC status remains OK. NOT an
//     abort.
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
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

const (
	executeAtomicMethod    = "ExecuteAtomic"
	executeAtomicDefaultMs = 30_000

	// Internal op-string constants. Kept local rather than imported from
	// apply to avoid pulling that package's heavyweight deps into the api
	// binary just for two strings.
	opRegisterSchema = "register_schema"
	opCreateNode     = "create_node"
	opUpdateNode     = "update_node"
	opDeleteNode     = "delete_node"
	opDeleteWhere    = "delete_where"
	opCreateEdge     = "create_edge"
	opDeleteEdge     = "delete_edge"
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

	// Required-field validation.
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
	// (PR #168 invariant.)
	trustedActor := auth.Authoritative(ctx, auth.ParseActor(rctx.GetActor()))

	// Idempotency key — server-generated UUIDv4 if empty. Generated here
	// so retries in the in-band INTERNAL path can still observe a stable
	// receipt.
	idem := req.GetIdempotencyKey()
	if idem == "" {
		idem = uuid.NewString()
	}

	// Schema fingerprint check + SELF-DESCRIBING WRITES negotiation.
	//
	// srvFP is the server's current registry fingerprint (empty in the
	// schema-less profile). When the request carries a SchemaDescriptor
	// (req.schema) and its fingerprint differs from the server's, the
	// write is self-describing: we materialize the carried types into a
	// leading register_schema WAL op (built below) instead of rejecting.
	// The applier registers establish-or-reject, so an identical schema is
	// a cheap idempotent no-op and a conflicting one fails the event
	// deterministically (FAILED_PRECONDITION) without halting.
	//
	// Only when the request claims a fingerprint that disagrees with the
	// server AND carries NO schema do we keep the legacy in-band
	// SCHEMA_MISMATCH (the client believes it matches a schema the server
	// no longer has, and gave us nothing to reconcile with).
	srvFP := ""
	if s.registry != nil {
		srvFP = s.registry.Fingerprint()
	}
	reqFP := req.GetSchemaFingerprint()
	carriesSchema := req.GetSchema() != nil &&
		(len(req.GetSchema().GetNodeTypes()) > 0 || len(req.GetSchema().GetEdgeTypes()) > 0)
	// includeSchemaOp: prepend a register_schema op whenever the client
	// rides types, EXCEPT the steady state where both fingerprints are
	// known and equal (the server already has exactly this schema). The
	// applier registers establish-or-reject and no-ops an identical type,
	// so appending the op is always safe — but skipping it in steady
	// state keeps the common write cheap. Note: an empty server
	// fingerprint (fresh selfdescribing registry) is NOT equal to a
	// non-empty client fingerprint, and two empty fingerprints still
	// trigger the op when a schema is carried (the client's very first
	// write before it has negotiated a fingerprint).
	fingerprintsKnownAndEqual := reqFP != "" && srvFP != "" && reqFP == srvFP
	includeSchemaOp := carriesSchema && !fingerprintsKnownAndEqual
	if !includeSchemaOp && reqFP != "" && srvFP != "" && reqFP != srvFP {
		outcome = "error"
		return &pb.ExecuteAtomicResponse{
			Success:   false,
			Error:     fmt.Sprintf("Schema mismatch: server has %s", srvFP),
			ErrorCode: "SCHEMA_MISMATCH",
		}, nil
	}

	// Validation-ordering refinement (ADR-031 "Known interactions"): the
	// handler validates the batch's data ops against the current registry
	// UNIONED WITH the schema attached to THIS request. Otherwise a
	// brand-new type's non-create op (e.g. delete_where) would be rejected
	// before the prepended register_schema op registers it in the applier.
	// validationReg is the union view; it falls back to s.registry when
	// nothing is attached.
	validationReg := s.registry
	if carriesSchema {
		view, verr := s.schemaValidationView(req.GetSchema())
		if verr != nil {
			outcome = "error"
			return nil, verr
		}
		validationReg = view
	}

	// Translate proto operations to internal id-keyed shape. This is the
	// CLAUDE.md invariant #6 / ADR-031 chokepoint: field_id-keyed Structs
	// become id-keyed payload maps before they reach the WAL, validated
	// against the union view.
	ops, err := s.convertOperations(validationReg, req.GetOperations())
	if err != nil {
		outcome = "error"
		return nil, err
	}

	// SELF-DESCRIBING WRITES: prepend the register_schema op so the
	// applier materializes the carried types (and their per-tenant
	// indexes) BEFORE the data ops, in the SAME WAL event / transaction.
	// Because the schema rides in the WAL, replaying the log
	// deterministically rebuilds the registry + indexes.
	if includeSchemaOp {
		schemaOp, serr := convertSchemaDescriptor(req.GetSchema())
		if serr != nil {
			outcome = "error"
			return nil, serr
		}
		ops = append([]map[string]any{schemaOp}, ops...)
	}

	// Tenant-membership + role + status check. Best-effort: skipped when
	// no globalstore is wired (single-node bring-up / no-auth tests).
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
	// Why resolve at ingress: the WAL is the audit-of-record; a bare
	// UUID is a stable, unambiguous identity, whereas "$alias" is
	// meaningful only in the context of one in-flight transaction.
	// Resolving here keeps the applier dispatch table free of map-shape
	// branching for the alias_ref payload and prevents alias bugs from
	// poisoning the consumer loop (the original symptom: "poison event:
	// create_edge missing from/to" halted the applier for every
	// subsequent RPC).
	//
	// Ordering: we walk ops in declared order. Each create_node with an
	// "as" field publishes its (server-filled) id into the alias map;
	// subsequent edge ops referencing that alias resolve against the
	// map. A reference to an alias that has not been previously defined
	// in the SAME transaction is an INVALID_ARGUMENT. Aliases never
	// span transactions.
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
	// not the wire claim — audit-truth invariant.
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
		// signals failure.
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

	// Optional apply-wait. Default 30s when wait_timeout_ms is zero.
	// Honour ctx cancellation.
	appliedStatus := pb.ReceiptStatus_RECEIPT_STATUS_PENDING
	var preFailure *pb.PreconditionFailure
	// uniqueViolationDetail, when non-empty, is the structured
	// ALREADY_EXISTS message the applier memoized for a tripped
	// single-field/composite unique constraint (issue #566). Unlike a
	// CAS miss (which is surfaced in-band), a unique violation surfaces
	// as a real gRPC ALREADY_EXISTS status so the SDKs raise a typed
	// UniqueConstraintError — see the SDK parsers.
	var uniqueViolationDetail string
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
			case rec.Present && rec.Status == store.IdempotencyStatusUniqueViolation:
				uniqueViolationDetail = decodeUniqueViolationDetailJSON(rec.FailureJSON)
			default:
				appliedStatus = pb.ReceiptStatus_RECEIPT_STATUS_APPLIED
			}
		}
		cancel()
	}

	// A tripped unique constraint surfaces as a gRPC ALREADY_EXISTS
	// status (issue #566) carrying the structured detail verbatim, so
	// the SDK error parsers produce a typed UniqueConstraintError. We
	// only have the detail when wait_applied raced the applier to the
	// memoized row; without it the client polls GetReceiptStatus.
	if uniqueViolationDetail != "" {
		outcome = "already_exists"
		return nil, errs.Errorf(codes.AlreadyExists, "%s", uniqueViolationDetail)
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

// decodeUniqueViolationDetailJSON parses the failure_json blob the
// applier memoizes for a UNIQUE_VIOLATION row (issue #566) and returns
// the structured ALREADY_EXISTS detail string. The blob is the
// JSON-encoded apply.UniqueViolation ({"Detail": "..."}); an empty or
// undecodable blob yields "" so the caller falls back to the generic
// path.
func decodeUniqueViolationDetailJSON(blob string) string {
	if blob == "" {
		return ""
	}
	var uv struct {
		Detail string `json:"Detail"`
	}
	if err := json.Unmarshal([]byte(blob), &uv); err != nil {
		return ""
	}
	return uv.Detail
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
// written).
func streamPosString(pos wal.StreamPos) string {
	if pos == (wal.StreamPos{}) {
		return ""
	}
	return pos.String()
}

// convertOperations translates wire Operations to internal id-keyed
// op dicts. The schema-aware translation lives in
// payload.StructToPayload; everything else is mechanical proto-to-map
// boilerplate. reg is the validation view (current registry unioned with
// the request's attached schema, per ADR-031) used for type-existence
// and field-kind resolution.
func (s *Server) convertOperations(reg *schema.Registry, operations []*pb.Operation) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(operations))

	for _, op := range operations {
		switch v := op.GetOp().(type) {
		case *pb.Operation_CreateNode:
			create := v.CreateNode
			if create.GetTypeId() == 0 {
				return nil, errs.Errorf(codes.InvalidArgument,
					"create_node: type_id is required")
			}
			data, err := s.ingressPayload(reg, create.GetTypeId(), create.GetTypedData(), create.GetData())
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
			patch, err := s.ingressPayload(reg, upd.GetTypeId(), upd.GetTypedPatch(), upd.GetPatch())
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
			// Resolve the FieldFilter field_ids using the EXACT same path
			// QueryNodes uses (issue #501) so the predicate is name-free
			// on the WAL (ADR-031).
			//
			// Schema-OPTIONAL, exactly like QueryNodes (issue #545): a
			// nil registry is the server's schema-less mode and is
			// tolerated here just as it is in query_nodes.go. When the
			// registry is nil, fieldFiltersToStoreFilters ->
			// payload.FilterToIDs takes its schema-less branch: digit-only
			// FieldFilter.field keys parse to raw payload field_ids, and a
			// non-digit key surfaces as INVALID_ARGUMENT with the same
			// wording as the QueryNodes path. Do NOT add a hard reject for
			// a missing schema here — that is the exact bug #545 fixes and
			// would diverge from the QueryNodes contract. Schema-mode
			// behaviour (registry configured) is unchanged: an unknown
			// type_id is still INVALID_ARGUMENT.
			if reg != nil {
				if nt := reg.NodeTypeByID(dw.GetTypeId()); nt == nil {
					return nil, errs.Errorf(codes.InvalidArgument,
						"delete_where: unknown type_id %d", dw.GetTypeId())
				}
			}
			storeFilters, err := s.fieldFiltersToStoreFilters(reg, dw.GetTypeId(), dw.GetWhere())
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
// payload consumed by the applier.
//
// NAME-FREE per ADR-031: the CAS precondition is keyed by field_id only
// (the proto field already calls field_id "the preferred and
// schema-less-safe coordinate"). A request without field_id is
// INVALID_ARGUMENT — the server never resolves a name. The "equals"
// value is unboxed from the *structpb.Value envelope at the boundary so
// the applier compares raw Go scalars rather than wrapped proto values;
// the wire-side nullability is preserved (nil → field-absence match).
//
// Error semantics:
//
//   - missing/invalid field_id → INVALID_ARGUMENT
//
// See GitHub issues #500 and #525.
func (s *Server) translatePrecondition(_ int32, pre *pb.UpdateNodePrecondition) (map[string]any, error) {
	// structpb.Value.AsInterface() unboxes scalars to Go-native types
	// (float64 for numbers, string, bool, nil for NullValue, []any /
	// map[string]any for compounds) — exactly the shape the applier's
	// reflect.DeepEqual comparator wants.
	var equals any
	if tv := pre.GetTypedEquals(); tv != nil {
		// Prefer the typed expected value (ADR-028 / #572) so a CAS on an
		// int64 >2^53 field compares the exact value; int64 flows through
		// the WAL canonical decode and matches the stored int64.
		gv, terr := payload.EntValueToGo(tv)
		if terr != nil {
			return nil, errs.Errorf(codes.InvalidArgument, "precondition.equals: %v", terr)
		}
		equals = gv
	} else if eq := pre.GetEquals(); eq != nil {
		equals = eq.AsInterface()
	}
	fid := pre.GetFieldId()
	if fid == 0 {
		return nil, errs.Errorf(codes.InvalidArgument,
			"update_node: precondition.field_id is required")
	}
	if fid < 0 || fid > 65535 {
		return nil, errs.Errorf(codes.InvalidArgument,
			"update_node: precondition.field_id must be 1-65535, got %d", fid)
	}
	return map[string]any{
		// "field" carries the decimal field_id as a stable, name-free
		// label surfaced on the failure detail (no field name exists).
		"field":    fmt.Sprintf("%d", fid),
		"field_id": fmt.Sprintf("%d", fid),
		"equals":   equals,
	}, nil
}

// translatePayload normalises a create_node.data / update_node.patch
// Struct into the id-keyed shape the WAL/applier consume. NAME-FREE per
// ADR-031: the Struct must already be field_id-keyed (the SDK
// pre-translates); payload.StructToPayload rejects a non-digit key and
// uses the registry only for field-kind coercion (resolved by type_id).
// The schema-less / unknown-type-id path is a digit-key passthrough.
//
// The result is stored under the op's "data"/"patch" key as
// map[string]any with stringified field-id keys — that's the shape the
// applier (which JSON-decodes events from the WAL) consumes.
func (s *Server) translatePayload(reg *schema.Registry, typeID int32, st *structpb.Struct) (map[string]any, error) {
	idKeyed, err := payload.StructToPayload(reg, typeID, st)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any, len(idKeyed))
	for fid, val := range idKeyed {
		out[fmt.Sprintf("%d", fid)] = val
	}
	return out, nil
}

// ingressPayload converts a write op's payload to the internal
// stringified id-keyed map. It PREFERS the typed EntValue map (ADR-028 —
// lossless int64) when present, falling back to the legacy
// google.protobuf.Struct. The typed map already carries concrete types,
// so no schema-driven coercion or safe-integer guard is needed; int64
// flows through the WAL (canonical jsonnum decode) to payload_json intact.
func (s *Server) ingressPayload(reg *schema.Registry, typeID int32, typed map[uint32]*pb.EntValue, st *structpb.Struct) (map[string]any, error) {
	if len(typed) > 0 {
		idKeyed, err := payload.TypedToPayload(typed)
		if err != nil {
			return nil, err
		}
		out := make(map[string]any, len(idKeyed))
		for fid, val := range idKeyed {
			out[fmt.Sprintf("%d", fid)] = val
		}
		return out, nil
	}
	return s.translatePayload(reg, typeID, st)
}

// convertSchemaDescriptor lowers a wire pb.SchemaDescriptor into the
// internal register_schema op map. The node_types / edge_types entries
// use the EXACT snake_case keys of the schema JSON contract
// (schema.NodeTypeDef / schema.FieldDef / schema.EdgeTypeDef struct
// tags) so the applier's decodeSchemaOp round-trips them straight into
// the typed schema structs via encoding/json — keeping this lowering in
// lockstep with schema.LoadFromJSON.
//
// Validation (type_id/edge_id positivity, field invariants,
// composite-unique sanity) is deferred to the applier's
// RegisterOrVerify* path, which runs the schema package's own Validate
// — the single source of truth. A structurally empty descriptor returns
// an error so we never emit a no-op schema op.
func convertSchemaDescriptor(desc *pb.SchemaDescriptor) (map[string]any, error) {
	if desc == nil {
		return nil, errs.Errorf(codes.InvalidArgument, "schema: nil descriptor")
	}
	nodeTypes := make([]any, 0, len(desc.GetNodeTypes()))
	for _, nt := range desc.GetNodeTypes() {
		nodeTypes = append(nodeTypes, schemaNodeToMap(nt))
	}
	edgeTypes := make([]any, 0, len(desc.GetEdgeTypes()))
	for _, et := range desc.GetEdgeTypes() {
		edgeTypes = append(edgeTypes, schemaEdgeToMap(et))
	}
	if len(nodeTypes) == 0 && len(edgeTypes) == 0 {
		return nil, errs.Errorf(codes.InvalidArgument,
			"schema: descriptor has no node_types or edge_types")
	}
	op := map[string]any{"op": opRegisterSchema}
	if len(nodeTypes) > 0 {
		op["node_types"] = nodeTypes
	}
	if len(edgeTypes) > 0 {
		op["edge_types"] = edgeTypes
	}
	return op, nil
}

// schemaValidationView builds the registry view the handler validates a
// batch's data ops against: the current registry's types UNIONED with the
// types carried on THIS request's SchemaDescriptor (ADR-031 "Known
// interactions"). Without this union, a brand-new type's non-create op
// (e.g. delete_where) would be rejected as an unknown type_id before the
// prepended register_schema op registers it in the applier.
//
// The view is a throwaway registry (never the process-wide one — the
// applier owns that, ADR-016): it seeds from the current registry's
// types, then layers ONLY the descriptor types whose id is not already
// present. It deliberately does NOT enforce establish-or-reject: a
// descriptor that conflicts with an already-registered type is left to
// the applier's deterministic, in-band conflict path (FAILED_PRECONDITION,
// surfaced via the register_schema op). The view exists solely to make
// data ops' type/field references resolvable at ingress, never to gate
// schema evolution.
//
// On a nil descriptor or empty registry the view degrades gracefully to
// whatever is available.
func (s *Server) schemaValidationView(desc *pb.SchemaDescriptor) (*schema.Registry, error) {
	view := schema.NewRegistry()
	seenNode := map[int32]struct{}{}
	seenEdge := map[int32]struct{}{}
	// Seed from the current process-wide registry (copy the type defs).
	if s.registry != nil {
		for _, nt := range s.registry.NodeTypes() {
			cp := *nt
			if err := view.RegisterNode(&cp); err != nil {
				return nil, errs.Internal(context.Background(), "schema view: seed node", err)
			}
			seenNode[nt.TypeID] = struct{}{}
		}
		for _, et := range s.registry.EdgeTypes() {
			cp := *et
			if err := view.RegisterEdge(&cp); err != nil {
				return nil, errs.Internal(context.Background(), "schema view: seed edge", err)
			}
			seenEdge[et.EdgeID] = struct{}{}
		}
	}
	if desc == nil {
		return view, nil
	}
	// Layer only descriptor types absent from the seed. A conflicting
	// redefinition of a present id is NOT merged here (the applier rejects
	// it deterministically in-band). Reuse the JSON contract lowering so
	// this view's type shape matches the WAL op the applier consumes.
	for _, ntMsg := range desc.GetNodeTypes() {
		if _, ok := seenNode[ntMsg.GetTypeId()]; ok {
			continue
		}
		nt, err := schemaNodeFromMsg(ntMsg)
		if err != nil {
			return nil, err
		}
		if err := view.RegisterNode(nt); err != nil {
			return nil, errs.Errorf(codes.InvalidArgument,
				"schema: attached node type_id %d: %v", nt.TypeID, err)
		}
		seenNode[nt.TypeID] = struct{}{}
	}
	for _, etMsg := range desc.GetEdgeTypes() {
		if _, ok := seenEdge[etMsg.GetEdgeId()]; ok {
			continue
		}
		et, err := schemaEdgeFromMsg(etMsg)
		if err != nil {
			return nil, err
		}
		if err := view.RegisterEdge(et); err != nil {
			return nil, errs.Errorf(codes.InvalidArgument,
				"schema: attached edge edge_id %d: %v", et.EdgeID, err)
		}
		seenEdge[et.EdgeID] = struct{}{}
	}
	return view, nil
}

// schemaNodeFromMsg lowers a wire SchemaNodeTypeDef into the typed
// schema.NodeTypeDef via the JSON contract (schemaNodeToMap) so the view
// and the WAL op share one decode path.
func schemaNodeFromMsg(msg *pb.SchemaNodeTypeDef) (*schema.NodeTypeDef, error) {
	b, err := json.Marshal(schemaNodeToMap(msg))
	if err != nil {
		return nil, errs.Errorf(codes.InvalidArgument, "schema: encode node: %v", err)
	}
	var nt schema.NodeTypeDef
	if err := json.Unmarshal(b, &nt); err != nil {
		return nil, errs.Errorf(codes.InvalidArgument, "schema: decode node: %v", err)
	}
	return &nt, nil
}

// schemaEdgeFromMsg is the edge counterpart of schemaNodeFromMsg.
func schemaEdgeFromMsg(msg *pb.SchemaEdgeTypeDef) (*schema.EdgeTypeDef, error) {
	b, err := json.Marshal(schemaEdgeToMap(msg))
	if err != nil {
		return nil, errs.Errorf(codes.InvalidArgument, "schema: encode edge: %v", err)
	}
	var et schema.EdgeTypeDef
	if err := json.Unmarshal(b, &et); err != nil {
		return nil, errs.Errorf(codes.InvalidArgument, "schema: decode edge: %v", err)
	}
	return &et, nil
}

// schemaFieldToMap lowers a wire SchemaFieldDef to the schema.FieldDef
// JSON shape. Only set keys are emitted so the JSON round-trip into
// schema.FieldDef leaves zero-value fields at their Go zero values
// (matching the omitempty-tagged struct). Name-free per ADR-031: no
// "name" key.
func schemaFieldToMap(f *pb.SchemaFieldDef) map[string]any {
	m := map[string]any{
		"field_id": f.GetFieldId(),
		"kind":     f.GetKind(),
	}
	if f.GetRequired() {
		m["required"] = true
	}
	if ev := f.GetEnumValues(); len(ev) > 0 {
		m["enum_values"] = toAnySlice(ev)
	}
	// ref_type_id is proto3-optional: only emit when explicitly present.
	if f.RefTypeId != nil {
		m["ref_type_id"] = f.GetRefTypeId()
	}
	if f.GetIndexed() {
		m["indexed"] = true
	}
	if f.GetSearchable() {
		m["searchable"] = true
	}
	if f.GetDeprecated() {
		m["deprecated"] = true
	}
	if d := f.GetDescription(); d != "" {
		m["description"] = d
	}
	if f.GetPii() {
		m["pii"] = true
	}
	if f.GetUnique() {
		m["unique"] = true
	}
	return m
}

// schemaNodeToMap lowers a wire SchemaNodeTypeDef to the
// schema.NodeTypeDef JSON shape.
func schemaNodeToMap(nt *pb.SchemaNodeTypeDef) map[string]any {
	fields := make([]any, 0, len(nt.GetFields()))
	for _, f := range nt.GetFields() {
		fields = append(fields, schemaFieldToMap(f))
	}
	m := map[string]any{
		"type_id": nt.GetTypeId(),
		"fields":  fields,
	}
	if nt.GetDeprecated() {
		m["deprecated"] = true
	}
	if d := nt.GetDescription(); d != "" {
		m["description"] = d
	}
	if dp := nt.GetDataPolicy(); dp != "" {
		m["data_policy"] = dp
	}
	// subject_field is a field_id (name-free, ADR-031).
	if nt.SubjectField != nil {
		m["subject_field"] = nt.GetSubjectField()
	}
	if nt.LegalBasis != nil {
		m["legal_basis"] = nt.GetLegalBasis()
	}
	if cu := nt.GetCompositeUnique(); len(cu) > 0 {
		out := make([]any, 0, len(cu))
		for _, c := range cu {
			// Composite constraint is identified by its field_ids tuple
			// (name-free, ADR-031).
			out = append(out, map[string]any{
				"field_ids": toAnyUint32Slice(c.GetFieldIds()),
			})
		}
		m["composite_unique"] = out
	}
	return m
}

// schemaEdgeToMap lowers a wire SchemaEdgeTypeDef to the
// schema.EdgeTypeDef JSON shape.
func schemaEdgeToMap(et *pb.SchemaEdgeTypeDef) map[string]any {
	props := make([]any, 0, len(et.GetProps()))
	for _, p := range et.GetProps() {
		props = append(props, schemaFieldToMap(p))
	}
	m := map[string]any{
		"edge_id":      et.GetEdgeId(),
		"from_type_id": et.GetFromTypeId(),
		"to_type_id":   et.GetToTypeId(),
		"props":        props,
	}
	if et.GetUniquePerFrom() {
		m["unique_per_from"] = true
	}
	if et.GetDeprecated() {
		m["deprecated"] = true
	}
	if d := et.GetDescription(); d != "" {
		m["description"] = d
	}
	if dp := et.GetDataPolicy(); dp != "" {
		m["data_policy"] = dp
	}
	// on_subject_exit is always emitted by the schema JSON contract
	// (defaults to "both"); the applier's RegisterOrVerifyEdge also
	// normalises an empty value, so emit only when the client set it.
	if ose := et.GetOnSubjectExit(); ose != "" {
		m["on_subject_exit"] = ose
	}
	return m
}

// toAnySlice widens a []string to []any so it survives the WAL JSON
// round-trip as the schema struct's []string field.
func toAnySlice(in []string) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

// toAnyUint32Slice widens a []uint32 to []any for the same reason.
func toAnyUint32Slice(in []uint32) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

// convertACL maps the wire AclEntry slice to the internal list-of-dicts
// the applier expects.
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
//     Supports "$a" and "$a.id"; only the dotted prefix before the
//     first "." is the alias name.
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
			// Bare "$alias" string is a short-form some SDKs emit
			// when bypassing NodeRef.
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
// transaction.
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

// convertNodeRef translates a proto NodeRef to the internal shape the
// applier's create_edge / delete_edge handlers consume: a bare id
// string, {"ref": <alias>}, or {"type_id": N, "id": "X"}.
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

// storageModeName maps a proto StorageMode enum to its internal string.
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
// gates. Skips when no globalstore is wired or the actor is system/admin
// (which bypass these checks).
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
		// Group / unknown actor kinds aren't valid callers — treat them as
		// non-members.
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
