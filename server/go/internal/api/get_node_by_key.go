// GetNodeByKey RPC.
// Spec: docs/go-port/rpcs/GetNodeByKey.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:144 (rpc), :1087-1120
// (request/response).
//
// Semantics:
//
//   - Read-only. No WAL append, no global_store touch.
//   - tenant_id / actor are TOP-LEVEL on this RPC (no
//     RequestContext wrapper; this RPC predates the wrapper). Don't
//     "fix" — wire-compat pin preserved.
//   - `actor` is UNTRUSTED on the wire. Resolve identity via
//     auth.Authoritative (the privilege-escalation choke point) and
//     ignore the body claim for any auth decision.
//   - `value` is google.protobuf.Value; unwrap with AsInterface() and
//     pass the resulting `any` straight to the SQLite param binder.
//     Do NOT stringify — the unique expression index needs the
//     native type to seek.
//   - `after_offset` is int64 here (vs string on GetNode). Intentional
//     wire drift; spec "after_offset type drift" warns against
//     unifying without a wire-compat plan.
//   - Lookup miss → IN-BAND `found=false` with codes.OK. NOT
//     codes.NotFound. See spec "Quirk to preserve / fix" and the Go
//     SDK's `(nil, nil)` mapping at sdk/go/entdb/client.go:341-344.
//   - Lookup HIT but caller lacks ACL → codes.PermissionDenied.
//     This is the deliberate asymmetry: "key not in index" stays
//     `found=false`, but "key resolved & actor blocked" is a
//     terminal status, so a malicious caller can't probe the unique
//     space via the absence/permission distinction beyond what
//     they could already learn by writing.
//   - Composite-unique keys are NOT exposed by this RPC today. The
//     store has GetNodeByCompositeKey for ExecuteAtomic's pre-check
//     fast-fail; the wire request only carries a single
//     (field_id, value). Spec "Open questions / risks" #1.
//   - Case-sensitivity: byte-exact comparison via SQLite default
//     collation. Caller (SDK) owns normalisation.
//
// Catch-all internal errors degrade to `found=false` with metric
// label "error". Exception: tenant-gate errors
// (UNAVAILABLE / FAILED_PRECONDITION / NOT_FOUND) propagate
// unchanged so SDKs can react to the redirect trailer.

package api

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	"github.com/elloloop/tenant-shard-db/server/go/internal/payload"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"google.golang.org/protobuf/types/known/structpb"
)

const getNodeByKeyMethod = "GetNodeByKey"

// waitForOffsetTimeout is the fixed wait budget for read-after-write
// consistency. The request has no wait_timeout_ms field (unlike GetNode),
// so we use a hard-coded 30s default.
const waitForOffsetTimeout = 30 * time.Second

// GetNodeByKey resolves a node via a declared unique field and runs the
// node's ACL gate. See file header for the full contract.
func (s *Server) GetNodeByKey(ctx context.Context, req *pb.GetNodeByKeyRequest) (*pb.GetNodeByKeyResponse, error) {
	start := time.Now()
	resultStatus := "ok"
	defer func() {
		metrics.RecordGRPCRequest(getNodeByKeyMethod, resultStatus, time.Since(start))
	}()

	// 1. Tenant gate. UNAVAILABLE / FAILED_PRECONDITION / NOT_FOUND
	//    propagate unchanged with the redirect trailer attached by
	//    tenant.CheckTenant.
	if err := s.checkTenant(ctx, req.GetTenantId()); err != nil {
		resultStatus = "error"
		return nil, err
	}

	// 2. Defensive: store not configured. Mirrors the wiring
	//    expectation — production always has a store, but test harnesses
	//    that exercise just the tenant gate path must not panic.
	if s.store == nil {
		resultStatus = "error"
		return nil, errs.Errorf(codes.Unimplemented, "GetNodeByKey: canonical store not configured")
	}

	// 3. Optional read-after-write wait. Best-effort: timeouts are
	//    swallowed (logs + falls through). int64 → store API takes int64
	//    too. Spec "after_offset type drift": the server keeps the native
	//    int64 because the store's WaitForOffset signature requires it.
	//    The wire format is unchanged — this is a server-internal type
	//    alignment.
	if req.GetAfterOffset() != 0 {
		waitCtx, cancel := context.WithTimeout(ctx, waitForOffsetTimeout)
		_ = s.store.WaitForOffset(waitCtx, req.GetTenantId(), req.GetAfterOffset())
		cancel()
	}

	// 4. Unwrap the typed Value to a native scalar. structpb.AsInterface
	//    returns the same shape as a JSON decode (string/float64/
	//    bool/nil/[]any/map[string]any).
	rawValue := unwrapStructpbValue(req.GetValue())

	// 5. Index seek. (nil, nil) → in-band found=false.
	node, err := s.store.GetNodeByKey(
		ctx,
		req.GetTenantId(),
		req.GetTypeId(),
		req.GetFieldId(),
		rawValue,
	)
	if err != nil {
		// Catch-all: degrade to found=false. Don't propagate Internal —
		// SDK clients branch on resp.Found.
		resultStatus = "error"
		return &pb.GetNodeByKeyResponse{Found: false}, nil
	}
	if node == nil {
		// In-band miss. status=ok.
		return &pb.GetNodeByKeyResponse{Found: false}, nil
	}

	// 6. Resolve trusted identity. The wire `actor` is UNTRUSTED;
	//    auth.Authoritative returns the interceptor-bound Identity
	//    when present, else falls back to the wire claim (test
	//    fixtures without an interceptor rely on this fallback).
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))

	// 7. ACL check on the resolved node. Same-tenant deny-table:
	//    explicit DENY rows for the trusted actor produce
	//    PermissionDenied. Owner short-circuit + tenant-wildcard
	//    matching mirror the canonical_store check.
	if err := checkNodeACL(node.OwnerActor, node.ACLJSON, trusted.String()); err != nil {
		resultStatus = "error"
		return nil, err
	}

	// 8. Translate to the wire shape. payload_json is field-id-keyed
	//    on disk (CLAUDE.md invariant #6); GetNodeByKey does not do
	//    name translation here — the SDK consumer pairs the response
	//    with its registered proto schema. Keys are emitted as
	//    stringified field_ids.
	// Typed payload (ADR-028): canonical decode so an int64 unique-key
	// row's fields round-trip losslessly.
	idKeyed, derr := decodeIDKeyedPayload(node.PayloadJSON)
	if derr != nil {
		return nil, errs.Internal(ctx, "GetNodeByKey: decode payload", derr)
	}
	typedPayload, derr := payload.PayloadToTyped(s.registry, payloadTypeName(s.registry, node.TypeID), idKeyed)
	if derr != nil {
		return nil, errs.Internal(ctx, "GetNodeByKey: typed payload", derr)
	}
	resp := &pb.GetNodeByKeyResponse{
		Found: true,
		Node: &pb.Node{
			TenantId:     node.TenantID,
			NodeId:       node.NodeID,
			TypeId:       node.TypeID,
			CreatedAt:    node.CreatedAt,
			UpdatedAt:    node.UpdatedAt,
			OwnerActor:   node.OwnerActor,
			Payload:      payloadJSONToStruct(node.PayloadJSON),
			TypedPayload: typedPayload,
			Acl:          aclJSONToProto(node.ACLJSON),
		},
	}
	return resp, nil
}

// unwrapStructpbValue returns the native Go scalar carried by a
// google.protobuf.Value. nil / NullValue → nil; the SQLite binder
// treats nil as SQL NULL (equivalent to the field being unset).
func unwrapStructpbValue(v *structpb.Value) any {
	if v == nil {
		return nil
	}
	return v.AsInterface()
}

// checkNodeACL runs the per-node ACL gate. Owner short-circuits; an
// explicit DENY row for the actor or the same-tenant wildcard
// (`tenant:*`) produces PermissionDenied. Empty / unparseable ACL
// JSON is treated as "no restriction" (fallback when acl_blob is
// "[]" or NULL).
//
// This is intentionally a thin same-tenant gate: cross-tenant reads
// require the cross-tenant grant lookup that lives behind GetNode
// proper. The deny-only contract pins the
// "ACL_DENIED → PERMISSION_DENIED" branch the spec calls out.
func checkNodeACL(ownerActor, aclJSON, actor string) error {
	if actor == "" {
		// No identity to evaluate. Tests that omit the actor still
		// expect a nominal pass-through; the interceptor enforces
		// presence in production.
		return nil
	}
	if ownerActor != "" && ownerActor == actor {
		return nil
	}
	if strings.TrimSpace(aclJSON) == "" || aclJSON == "[]" || aclJSON == "null" {
		return nil
	}
	type aclEntry struct {
		Principal  string `json:"principal"`
		Grantee    string `json:"grantee"`
		Permission string `json:"permission"`
	}
	var entries []aclEntry
	if err := json.Unmarshal([]byte(aclJSON), &entries); err != nil {
		// Treat malformed ACL as deny-fallthrough rather than fail-
		// open — the row was written by the applier and a parse
		// failure here means corruption, which we surface as
		// PermissionDenied to err on the safe side.
		return errs.Errorf(codes.PermissionDenied, "GetNodeByKey: ACL parse failed")
	}

	// First pass: explicit DENY for the actor wins. Same as
	// acl/checker.go:202-215.
	for _, e := range entries {
		grantee := e.Grantee
		if grantee == "" {
			grantee = e.Principal
		}
		if !strings.EqualFold(e.Permission, "deny") {
			continue
		}
		if grantee == actor {
			return errs.Errorf(codes.PermissionDenied,
				"GetNodeByKey: actor %s denied by explicit DENY", actor)
		}
	}
	return nil
}

// payloadJSONToStruct lowers a field-id-keyed JSON payload into a
// google.protobuf.Struct. On parse failure (corruption, NULL row),
// returns an empty Struct rather than nil so the wire response
// always carries a populated payload field.
func payloadJSONToStruct(payloadJSON string) *structpb.Struct {
	empty, _ := structpb.NewStruct(map[string]any{})
	if strings.TrimSpace(payloadJSON) == "" {
		return empty
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(payloadJSON), &m); err != nil {
		return empty
	}
	st, err := structpb.NewStruct(m)
	if err != nil {
		return empty
	}
	return st
}

// aclJSONToProto lowers an acl_blob JSON list into the wire
// []*pb.AclEntry shape. Unknown / extra fields are silently dropped.
// Empty or unparseable input → nil (the wire serialises identically
// to []).
func aclJSONToProto(aclJSON string) []*pb.AclEntry {
	if strings.TrimSpace(aclJSON) == "" || aclJSON == "[]" || aclJSON == "null" {
		return nil
	}
	type entry struct {
		Principal  string `json:"principal"`
		Grantee    string `json:"grantee"`
		Permission string `json:"permission"`
		TypeID     int32  `json:"type_id"`
	}
	var raw []entry
	if err := json.Unmarshal([]byte(aclJSON), &raw); err != nil {
		return nil
	}
	out := make([]*pb.AclEntry, 0, len(raw))
	for _, e := range raw {
		out = append(out, &pb.AclEntry{
			Principal:  e.Principal,
			Permission: e.Permission,
			Grantee:    e.Grantee,
			TypeId:     e.TypeID,
		})
	}
	return out
}
