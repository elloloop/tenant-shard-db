// GetNode RPC.
// Spec: docs/go-port/rpcs/GetNode.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:55 (rpc), :394-407
// (request/response), :454-472 (Node).
//
// Semantics:
//
//   - Read-only single-node lookup keyed on (tenant_id, node_id). Per
//     the spec, type_id on the request is ACCEPTED but NOT used to
//     filter the lookup — bug-for-bug parity with the storage chokepoint.
//     Flagged as a follow-up in the spec "Open questions / risks" section.
//
//   - Trusted-actor pattern (CLAUDE.md, commit fece3fb): the wire
//     `request.context.actor` is UNTRUSTED. We resolve identity via
//     `auth.Authoritative` and ignore the wire claim for any auth
//     decision.
//
//   - NOT_FOUND wins over PERMISSION_DENIED. The handler runs the
//     tenant gate, then determines the caller's role
//     (local/member/cross_tenant), then looks up the row. If the row
//     does not exist we return `&pb.GetNodeResponse{Found: false}` with
//     gRPC status OK — pin test_grpc_contract.py:217-222 (`r.found is
//     False`). PERMISSION_DENIED is reserved for the case where the
//     caller is not a tenant member at all (fires before lookup; spec
//     "Error contract" — non-members cannot probe for node existence)
//     OR where the row EXISTS and a cross-tenant caller lacks an
//     explicit per-node grant.
//
//   - Cross-tenant role check. When the caller is not a tenant member
//     but DOES have at least one node_access row in the tenant, role
//     becomes "cross_tenant". The per-node ACL is then re-checked
//     against the specific node_id post-lookup.
//
//   - Payload egress is id-keyed (CLAUDE.md invariant #6). The wire
//     Struct has stringified field_id keys ("1", "2", ...); SDK
//     translates id -> name on the client side.
//
//   - `after_offset` + `wait_timeout_ms` provide read-after-write
//     consistency by waiting for the WAL applier to reach a stream
//     position before reading. Default 30s when wait_timeout_ms == 0.
//     Wait failures are silent.
//
//   - Uncaught exceptions are NOT swallowed into found=false. In Go,
//     gRPC errors are returned as real status codes. See spec "Quirk
//     to preserve".
//
// No WAL append, no SQLite write.

package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	"github.com/elloloop/tenant-shard-db/server/go/internal/payload"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

const getNodeMethod = "GetNode"

// readRole / roleLocal / roleMember / roleCrossTenant — shared with
// the other read RPCs (GetNodes, QueryNodes, GetConnectedNodes) and
// declared in helpers.go (consolidated in the round-3 dedupe).

// GetNode implements entdb.v1.EntDBService/GetNode. See file header
// for the full contract.
func (s *Server) GetNode(ctx context.Context, req *pb.GetNodeRequest) (*pb.GetNodeResponse, error) {
	start := time.Now()
	resultStatus := "ok"
	defer func() {
		metrics.RecordGRPCRequest(getNodeMethod, resultStatus, time.Since(start))
	}()

	if s.store == nil {
		resultStatus = "error"
		return nil, errs.Errorf(codes.Unimplemented, "GetNode: store not wired")
	}

	tenantID := req.GetContext().GetTenantId()
	nodeID := req.GetNodeId()
	if nodeID == "" {
		resultStatus = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "GetNode: node_id is required")
	}

	// 1. Tenant gate — sharding redirect, region pin, existence.
	if err := s.checkTenant(ctx, tenantID); err != nil {
		resultStatus = "error"
		return nil, err
	}

	// 2. Trusted actor — wire claim is ignored for any auth decision
	//    (privilege-escalation guard, commit fece3fb).
	actor := auth.Authoritative(ctx, auth.ParseActor(req.GetContext().GetActor()))

	// 3. Make sure the per-tenant DB exists. CheckTenant has already
	//    vetted the tenant id; this just lazy-creates the
	//    tenant_<id>.db file and is a precondition for both the
	//    cross-tenant ACL probe and the GetNode read below.
	if err := s.store.OpenTenant(ctx, tenantID); err != nil {
		resultStatus = "error"
		return nil, errs.Errorf(errs.Code(err), "GetNode: open tenant: %v", err)
	}

	// 4. Cross-tenant read membership check. PERMISSION_DENIED here
	//    fires BEFORE the row lookup, so non-members cannot probe for
	//    node existence (spec "Error contract").
	role, err := s.checkCrossTenantRead(ctx, tenantID, actor)
	if err != nil {
		resultStatus = "error"
		return nil, err
	}

	// 5. Optional read-after-write barrier. Failures are silent — the
	//    handler proceeds with a stale read.
	if off := req.GetAfterOffset(); off != "" {
		timeout := 30 * time.Second
		if ms := req.GetWaitTimeoutMs(); ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
		waitCtx, cancel := context.WithTimeout(ctx, timeout)
		_ = s.store.WaitForOffset(waitCtx, tenantID, parseStreamOffset(off))
		cancel()
	}

	// 6. The actual read. Missing -> Found=false with status OK. Per
	//    spec, type_id is NOT used as a filter.
	//
	//    Mailbox scope (#568): when target_user is set, the read is
	//    confined to that user's USER_MAILBOX nodes — a node that is not a
	//    mailbox node owned by this user reads as Found=false, the same
	//    not-found signal a missing id produces, which is the intended
	//    privacy boundary.
	var n *store.Node
	if tu := req.GetTargetUser(); tu != "" {
		n, err = s.store.GetMailboxNode(ctx, tenantID, tu, nodeID)
	} else {
		n, err = s.store.GetNode(ctx, tenantID, nodeID)
	}
	if err != nil {
		if errors.Is(err, store.ErrNodeNotFound) {
			return &pb.GetNodeResponse{Found: false}, nil
		}
		resultStatus = "error"
		return nil, errs.Errorf(errs.Code(err), "GetNode: %v", err)
	}

	// 7. Cross-tenant per-node ACL re-check. The role test above only
	//    confirmed the actor has SOME access in the tenant; now we
	//    confirm the specific node grants them read. This fires AFTER
	//    the row lookup, so it returns PERMISSION_DENIED, not the
	//    found=false signal a non-member would see.
	if role == roleCrossTenant {
		ok, err := s.hasNodeAccess(ctx, tenantID, nodeID, actor.String())
		if err != nil {
			resultStatus = "error"
			return nil, errs.Errorf(errs.Code(err), "GetNode: %v", err)
		}
		if !ok {
			resultStatus = "error"
			return nil, errs.Errorf(codes.PermissionDenied,
				"Actor does not have access to this node")
		}
	}

	// 8. Translate the storage row into the wire Node. The payload
	//    stays id-keyed (CLAUDE.md invariant #6) — translation is a
	//    zero-translation wrap via payload.PayloadToStruct.
	wire, err := nodeToProto(s.registry, n)
	if err != nil {
		resultStatus = "error"
		return nil, errs.Errorf(errs.Code(err), "GetNode: encode payload: %v", err)
	}
	return &pb.GetNodeResponse{Found: true, Node: wire}, nil
}

// checkCrossTenantRead lives in helpers.go (consolidated in the
// round-3 dedupe). It now uses store.ResolveActorGroups +
// store.HasNodeAccess for group-aware grant resolution rather than the
// per-handler hasAnyNodeAccess shortcut — the per-node hasNodeAccess
// below is still consulted by step 7 of GetNode for the specific
// node_id post-lookup ACL re-check.

// hasNodeAccess returns true if actorID has a non-deny, non-expired
// node_access grant on (tenantID, nodeID). For GetNode parity the read
// predicate "any non-deny grant" is sufficient.
func (s *Server) hasNodeAccess(ctx context.Context, tenantID, nodeID, actorID string) (bool, error) {
	db, err := s.store.AdminDB(tenantID)
	if err != nil {
		return false, fmt.Errorf("GetNode: admin db: %w", err)
	}
	now := time.Now().UnixMilli()
	var one int
	err = db.QueryRowContext(ctx, `
		SELECT 1 FROM node_access
		WHERE node_id = ? AND actor_id = ?
		  AND permission != 'deny'
		  AND (expires_at IS NULL OR expires_at > ?)
		LIMIT 1`, nodeID, actorID, now).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("GetNode: query node_access: %w", err)
	}
	return true, nil
}

// nodeToProto translates a store.Node into the wire pb.Node, including
// id-keyed payload egress per CLAUDE.md invariant #6. The payload is
// wrapped via payload.PayloadToStruct so kind-aware coercions
// (BYTES -> base64, INTEGER -> float64) are handled centrally.
//
// The on-disk payload_json is keyed by string field_id ("1", "2"); we
// re-key to uint32 before handing it to PayloadToStruct (which expects
// a uint32-keyed map). When reg is nil OR has no entry for type_id
// the Struct round-trip is a schema-less passthrough — that's the
// path unit tests with no Registry exercise.
func nodeToProto(reg *schema.Registry, n *store.Node) (*pb.Node, error) {
	out := &pb.Node{
		TenantId:   n.TenantID,
		NodeId:     n.NodeID,
		TypeId:     n.TypeID,
		CreatedAt:  n.CreatedAt,
		UpdatedAt:  n.UpdatedAt,
		OwnerActor: n.OwnerActor,
	}

	// Canonical decode (jsonnum via the shared helper): integers survive
	// as int64 so typed_payload is lossless (ADR-028).
	idKeyed, err := decodeIDKeyedPayload(n.PayloadJSON)
	if err != nil {
		return nil, fmt.Errorf("decode payload_json: %w", err)
	}

	typeName := ""
	if reg != nil {
		if nt := reg.NodeTypeByID(n.TypeID); nt != nil {
			typeName = nt.Name
		}
	}
	wire, err := payload.PayloadToStruct(reg, typeName, idKeyed)
	if err != nil {
		return nil, fmt.Errorf("encode payload: %w", err)
	}
	out.Payload = wire
	typed, err := payload.PayloadToTyped(reg, typeName, idKeyed)
	if err != nil {
		return nil, fmt.Errorf("encode typed payload: %w", err)
	}
	out.TypedPayload = typed

	if n.ACLJSON != "" && n.ACLJSON != "[]" {
		var entries []store.ACLEntry
		if err := json.Unmarshal([]byte(n.ACLJSON), &entries); err == nil {
			for _, e := range entries {
				out.Acl = append(out.Acl, &pb.AclEntry{
					Principal:  e.Principal,
					Permission: e.Permission,
				})
			}
		}
	}
	return out, nil
}
