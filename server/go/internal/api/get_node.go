// GetNode RPC — Wave 2 of the Python -> Go server port (EPIC #407).
// Spec: docs/go-port/rpcs/GetNode.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:55 (rpc), :394-407
// (request/response), :454-472 (Node). Reference Python handler:
// server/python/entdb_server/api/grpc_server.py:994-1064.
//
// Semantics (preserved from the Python handler):
//
//   - Read-only single-node lookup keyed on (tenant_id, node_id). Per
//     the spec, type_id on the request is ACCEPTED but NOT used to
//     filter the lookup — bug-for-bug parity with Python's
//     `canonical_store.get_node(tenant, node_id)` chokepoint
//     (`grpc_server.py:1029-1031`). Flagged as a follow-up in the spec
//     "Open questions / risks" section.
//
//   - Trusted-actor pattern (CLAUDE.md, commit fece3fb): the wire
//     `request.context.actor` is UNTRUSTED. We resolve identity via
//     `auth.Authoritative` and ignore the wire claim for any auth
//     decision. Pinned by
//     tests/python/integration/test_privilege_escalation.py:186-209.
//
//   - NOT_FOUND wins over PERMISSION_DENIED. The handler runs the
//     tenant gate, then determines the caller's role
//     (local/member/cross_tenant), then looks up the row. If the row
//     does not exist we return `&pb.GetNodeResponse{Found: false}` with
//     gRPC status OK — Python contract pin
//     test_grpc_contract.py:217-222 (`r.found is False`).
//     PERMISSION_DENIED is reserved for the case where the caller is
//     not a tenant member at all (fires before lookup; spec
//     "Error contract" — non-members cannot probe for node existence)
//     OR where the row EXISTS and a cross-tenant caller lacks an
//     explicit per-node grant.
//
//   - Cross-tenant role check. When the caller is not a tenant member
//     but DOES have at least one node_access row in the tenant, role
//     becomes "cross_tenant" (Python `_check_cross_tenant_read`). The
//     per-node ACL is then re-checked against the specific node_id
//     post-lookup, mirroring Python's two-step at
//     `grpc_server.py:561-618` + `:1031-1045`.
//
//   - Payload egress is id-keyed (CLAUDE.md invariant #6). The wire
//     Struct has stringified field_id keys ("1", "2", ...); SDK
//     translates id -> name on the client side. Pinned by
//     test_payload_wire_format.py:158-189.
//
//   - `after_offset` + `wait_timeout_ms` provide read-after-write
//     consistency by waiting for the WAL applier to reach a stream
//     position before reading. Default 30s when wait_timeout_ms == 0.
//     Wait failures are silent (Python parity at
//     `grpc_server.py:1015-1020`).
//
//   - Unlike Python, we do NOT swallow uncaught exceptions into
//     found=false. The Python handler's outer try/except is a
//     defensive artifact unreachable in real grpc.aio runtime (where
//     context.abort is terminal). In Go, gRPC errors are returned as
//     real status codes. See spec "Quirk to preserve".
//
// No WAL append, no SQLite write.

package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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
// declared in helpers.go (consolidated in the round-3 Wave-2 dedupe).

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

	// 3. Make sure the per-tenant DB exists. Mirrors Python's
	//    auto-open via `_get_connection(tenant_id)` at
	//    canonical_store.py:2306. CheckTenant has already vetted the
	//    tenant id; this just lazy-creates the tenant_<id>.db file
	//    and is a precondition for both the cross-tenant ACL probe
	//    and the GetNode read below.
	if err := s.store.OpenTenant(ctx, tenantID); err != nil {
		resultStatus = "error"
		return nil, errs.Errorf(errs.Code(err), "GetNode: open tenant: %v", err)
	}

	// 4. Cross-tenant read membership check. PERMISSION_DENIED here
	//    fires BEFORE the row lookup, so non-members cannot probe for
	//    node existence (spec "Error contract" / Python `:1024-1025`).
	role, err := s.checkCrossTenantRead(ctx, tenantID, actor)
	if err != nil {
		resultStatus = "error"
		return nil, err
	}

	// 5. Optional read-after-write barrier. Failures are silent — the
	//    handler proceeds with a stale read. Mirrors Python's
	//    `_wait_for_offset` returning a bool that GetNode discards
	//    (`grpc_server.py:1015-1020`).
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
	//    spec, type_id is NOT used as a filter (Python parity).
	n, err := s.store.GetNode(ctx, tenantID, nodeID)
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
	//    found=false signal a non-member would see — matching Python
	//    `:1041-1045`.
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
// round-3 Wave-2 dedupe). It now uses store.ResolveActorGroups +
// store.HasNodeAccess for group-aware grant resolution rather than the
// per-handler hasAnyNodeAccess shortcut — the per-node hasNodeAccess
// below is still consulted by step 7 of GetNode for the specific
// node_id post-lookup ACL re-check.

// hasNodeAccess returns true if actorID has a non-deny, non-expired
// node_access grant on (tenantID, nodeID). Mirrors a slim slice of
// `canonical_store.can_access` (`canonical_store.py:2867-2946`) — for
// W2 GetNode parity the read predicate "any non-deny grant" is
// sufficient (Python `:1041-1045` calls `can_access(node_id, actor,
// "read")` which collapses to the same SQL when no typed caps are
// registered for the type).
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

	rawPayload := map[string]any{}
	if n.PayloadJSON != "" {
		if err := json.Unmarshal([]byte(n.PayloadJSON), &rawPayload); err != nil {
			return nil, fmt.Errorf("decode payload_json: %w", err)
		}
	}
	idKeyed := make(map[uint32]any, len(rawPayload))
	for k, v := range rawPayload {
		id, perr := strconv.ParseUint(k, 10, 32)
		if perr != nil {
			// Non-digit key on disk is unexpected; skip silently
			// rather than corrupting the wire response. Matches
			// Python's id_to_name_keys passthrough where an
			// unparseable key falls into the decimal-string bucket.
			continue
		}
		idKeyed[uint32(id)] = v
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
