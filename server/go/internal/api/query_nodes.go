// SPDX-License-Identifier: AGPL-3.0-only

// QueryNodes RPC — Wave 2 of the Python -> Go server port (EPIC #407).
// Spec: docs/go-port/rpcs/QueryNodes.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:61 (rpc), :424-452
// (request/response), :675-690 (FieldFilter / FilterOp). Reference
// Python handler: server/python/entdb_server/api/grpc_server.py:1293-1382.
//
// Behavior parity with the Python handler, with one DELIBERATE behavior
// fix flagged by the spec:
//
//  1. Tenant gate runs first via s.checkTenant (sharding + region +
//     globalstore existence). Same call shape as every other Wave-2
//     handler.
//  2. Trusted-actor: the wire-claimed actor is UNTRUSTED. We rebind via
//     auth.Authoritative(ctx, claimed) — mirrors commit fece3fb.
//  3. order_by, limit defaults are applied per spec (created_at, 100).
//  4. filters: name-keyed FieldFilter entries are translated to id-keyed
//     equality filters via payload.FilterNamesToIDs. Wave 2 supports
//     EQ only — every other operator surfaces as INVALID_ARGUMENT
//     (the full MongoDB-operator allow-list is W1.10's queryfilter
//     package; QueryNodes inherits it once it lands).
//  5. ACL post-filter on cross-tenant reads via acl.Filter.FilterReadable.
//     The Python handler iterates can_access; the equivalent Go path is
//     the bulk VisibleNodeIDs JOIN already implemented in
//     store.GetVisibleNodeIDs and exposed through acl.Filter.
//  6. Response: id-keyed payload Struct (PayloadToStruct) — pinned by
//     tests/python/unit/test_payload_wire_format.py:219. has_more is
//     the cheap (len == limit) heuristic Python uses; we preserve it
//     verbatim for parity (the cross-tenant ACL post-filter makes it
//     leakier but EPIC #407 explicitly defers a fix).
//
// BEHAVIOR CHANGE flagged in the spec ("Error contract" / "Open
// questions" §5): Python's handler swallows ALL exceptions and returns
// an empty QueryNodesResponse with codes.OK (grpc_server.py:1379-1382).
// The Go port MUST surface gRPC errors so unsupported filters,
// missing tenants, etc. reach the SDK. Verified by
// TestQueryNodes_UnsupportedFilterRejected.

package api

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/acl"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	"github.com/elloloop/tenant-shard-db/server/go/internal/payload"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

const queryNodesMethod = "QueryNodes"

// defaultQueryLimit mirrors the Python `request.limit or 100` fallback
// at grpc_server.py:1337.
const defaultQueryLimit = 100

// QueryNodes implements entdb.v1.EntDBService/QueryNodes.
func (s *Server) QueryNodes(ctx context.Context, req *pb.QueryNodesRequest) (*pb.QueryNodesResponse, error) {
	start := time.Now()
	resultStatus := "ok"
	defer func() {
		metrics.RecordGRPCRequest(queryNodesMethod, resultStatus, time.Since(start))
	}()

	tenantID := req.GetContext().GetTenantId()
	if err := s.checkTenant(ctx, tenantID); err != nil {
		resultStatus = "error"
		return nil, err
	}

	// Trusted-actor: wire field is UNTRUSTED. Rebind via
	// auth.Authoritative so the cross-tenant gate has the right
	// identity. Even though the Wave-2 cut does not yet enforce
	// cross-tenant role distinctions (no global membership reader is
	// wired up here), we still honour the privilege-escalation guard
	// so the chokepoint exists.
	claimedActor := auth.ParseActor(req.GetContext().GetActor())
	trustedAuthActor := auth.Authoritative(ctx, claimedActor)

	if s.store == nil {
		resultStatus = "error"
		return nil, errs.Errorf(codes.Unimplemented, "canonical store not configured")
	}

	typeID := req.GetTypeId()
	if typeID == 0 {
		resultStatus = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "type_id is required")
	}

	// Resolve the type so we can translate filter field names. A nil
	// registry is tolerated for the schema-less unit-test path: in
	// that case digit-only filter keys parse to ids, and string keys
	// surface INVALID_ARGUMENT (per payload.FilterNamesToIDs schema-
	// less path).
	typeName := ""
	if s.registry != nil {
		nt := s.registry.NodeTypeByID(typeID)
		if nt == nil {
			resultStatus = "error"
			return nil, errs.Errorf(codes.InvalidArgument, "unknown type_id %d", typeID)
		}
		typeName = nt.Name
	}

	// Translate FieldFilter wire shape -> id-keyed equality filters.
	// Wave 2 supports EQ only; non-EQ operators surface as
	// INVALID_ARGUMENT (parity-fix per spec — see file header).
	nameFilter, err := fieldFiltersToNameDict(req.GetFilters())
	if err != nil {
		resultStatus = "error"
		return nil, err
	}
	idFilter, err := payload.FilterNamesToIDs(s.registry, typeName, nameFilter)
	if err != nil {
		resultStatus = "error"
		return nil, err
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = defaultQueryLimit
	}

	nodes, err := s.store.QueryNodes(ctx, store.QueryNodesArgs{
		TenantID:        tenantID,
		TypeID:          typeID,
		EqualityFilters: idFilter,
		OrderBy:         req.GetOrderBy(),
		Descending:      req.GetDescending(),
		Limit:           limit,
		Offset:          int(req.GetOffset()),
	})
	if err != nil {
		resultStatus = "error"
		return nil, errs.Errorf(codes.Internal, "store: query nodes: %v", err)
	}

	// ACL post-filter (cross-tenant readers). Same-tenant member access
	// flows through the same code path: VisibleNodeIDs honours owner_actor
	// matches, tenant:* grants, and direct user/group node_visibility
	// rows.
	nodes, err = s.applyQueryACLFilter(ctx, tenantID, trustedAuthActor, nodes)
	if err != nil {
		resultStatus = "error"
		return nil, err
	}

	// Convert store.Node -> pb.Node. Payload stays id-keyed on the wire
	// (test_payload_wire_format.py:219 pin); ACL JSON unmarshalled to
	// repeated AclEntry.
	protoNodes := make([]*pb.Node, 0, len(nodes))
	for _, n := range nodes {
		pn, err := s.storeNodeToProto(typeName, n)
		if err != nil {
			resultStatus = "error"
			return nil, err
		}
		protoNodes = append(protoNodes, pn)
	}

	return &pb.QueryNodesResponse{
		Nodes:   protoNodes,
		HasMore: len(nodes) == limit,
	}, nil
}

// fieldFiltersToNameDict mirrors _field_filters_to_filter_dict in the
// Python servicer (grpc_server.py:190-228) but constrained to the
// Wave-2 supported subset: FilterOp.EQ only. Non-EQ operators surface
// as INVALID_ARGUMENT — the spec calls this out as a behaviour fix
// over Python's silent exception swallow (grpc_server.py:1379-1382).
//
// Multiple EQ filters on the same field collapse to the last value
// (mirrors the Python merge behaviour where equality on the same field
// is rewritten to $eq and merged; in the EQ-only subset this is
// equivalent to "last write wins").
func fieldFiltersToNameDict(filters []*pb.FieldFilter) (map[string]any, error) {
	out := map[string]any{}
	for _, f := range filters {
		if f == nil {
			continue
		}
		if f.GetOp() != pb.FilterOp_EQ {
			return nil, errs.Errorf(codes.InvalidArgument,
				"QueryNodes: filter operator %s on field %q is not yet supported (Wave 2 supports EQ only)",
				f.GetOp(), f.GetField())
		}
		// Reject the legacy inlined-operator shape: a Struct value
		// carrying $-prefixed keys is the Python SDK's way of smuggling
		// a non-EQ operator through Op=EQ. The Wave-2 cut treats this
		// as INVALID_ARGUMENT for the same reason — surface it instead
		// of silently dropping rows.
		if iv, ok := inlinedOperatorKey(f.GetValue().AsInterface()); ok {
			return nil, errs.Errorf(codes.InvalidArgument,
				"QueryNodes: inlined operator %q on field %q is not yet supported (Wave 2 supports EQ only)",
				iv, f.GetField())
		}
		out[f.GetField()] = f.GetValue().AsInterface()
	}
	return out, nil
}

// inlinedOperatorKey detects the Python SDK's inlined-operator shape
// (Op=EQ, Value=Struct{"$gte": …}). Returns the offending key and true
// when the value is a map whose first key starts with "$".
func inlinedOperatorKey(v any) (string, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return "", false
	}
	for k := range m {
		if len(k) > 0 && k[0] == '$' {
			return k, true
		}
	}
	return "", false
}

// applyQueryACLFilter is the cross-tenant post-filter. It mirrors the
// canonical_store.can_access loop at grpc_server.py:1344-1358 but uses
// the bulk acl.Filter pattern (single VisibleNodeIDs JOIN, not N+1
// per-row checks).
//
// The filter is a no-op when the actor is the system bypass, when the
// actor is empty (no auth), or when no canonical store is wired up.
// Same-tenant owner access is honoured by VisibleNodeIDs — there is no
// dedicated short-circuit for "tenant member" because in the Wave-2 cut
// we don't yet have a global tenant-membership reader at this layer.
func (s *Server) applyQueryACLFilter(ctx context.Context, tenantID string, a auth.Actor, nodes []*store.Node) ([]*store.Node, error) {
	if len(nodes) == 0 {
		return nodes, nil
	}
	// System / admin actors bypass ACL.
	if a.IsSystem() || a.IsAdmin() {
		return nodes, nil
	}
	if a.IsZero() || a.ID() == "" {
		return nodes, nil
	}

	aclActor := authActorToACLActor(a)
	if aclActor.IsZero() {
		return nodes, nil
	}

	resolver := acl.NewResolver(nil)
	filter := acl.NewFilter(resolver, storeVisibilityAdapter{s.store})

	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.NodeID
	}
	visible, err := filter.FilterReadable(ctx, tenantID, aclActor, ids)
	if err != nil {
		return nil, errs.Errorf(codes.Internal, "QueryNodes: acl filter: %v", err)
	}
	visibleSet := make(map[string]struct{}, len(visible))
	for _, id := range visible {
		visibleSet[id] = struct{}{}
	}
	out := make([]*store.Node, 0, len(visible))
	for _, n := range nodes {
		if _, ok := visibleSet[n.NodeID]; ok {
			out = append(out, n)
		}
	}
	return out, nil
}

// authActorToACLActor coerces an auth.Actor into an acl.Actor.
func authActorToACLActor(a auth.Actor) acl.Actor {
	switch a.Kind() {
	case auth.KindUser:
		return acl.User(a.ID())
	case auth.KindSystem:
		return acl.System(a.ID())
	case auth.KindAdmin:
		// admin: actors are not ACL grant subjects; map to user: so
		// owner-by-actor checks still resolve cleanly.
		return acl.User(a.ID())
	default:
		return acl.Actor{}
	}
}

// storeNodeToProto translates a store.Node into the wire pb.Node.
// Payload stays id-keyed on the wire; ACL is the JSON-stored shape
// unmarshalled to repeated AclEntry.
func (s *Server) storeNodeToProto(typeName string, n *store.Node) (*pb.Node, error) {
	rawPayload := map[string]any{}
	if n.PayloadJSON != "" {
		if err := json.Unmarshal([]byte(n.PayloadJSON), &rawPayload); err != nil {
			return nil, errs.Errorf(codes.Internal,
				"QueryNodes: parse payload for %s: %v", n.NodeID, err)
		}
	}
	idPayload := make(map[uint32]any, len(rawPayload))
	for k, v := range rawPayload {
		fid, ok := parsePayloadFieldID(k)
		if !ok {
			// Non-digit keys can't appear in on-disk payloads (the
			// applier writes id-keyed JSON only); drop silently to
			// avoid blowing up egress on legacy data.
			continue
		}
		idPayload[fid] = v
	}

	pStruct, err := payload.PayloadToStruct(s.registry, typeName, idPayload)
	if err != nil {
		return nil, err
	}

	var aclList []store.ACLEntry
	if n.ACLJSON != "" && n.ACLJSON != "null" {
		if err := json.Unmarshal([]byte(n.ACLJSON), &aclList); err != nil {
			return nil, errs.Errorf(codes.Internal,
				"QueryNodes: parse acl for %s: %v", n.NodeID, err)
		}
	}
	protoACL := make([]*pb.AclEntry, 0, len(aclList))
	for _, e := range aclList {
		protoACL = append(protoACL, &pb.AclEntry{
			Principal:  e.Principal,
			Permission: e.Permission,
			Grantee:    e.Principal,
		})
	}

	return &pb.Node{
		TenantId:   n.TenantID,
		NodeId:     n.NodeID,
		TypeId:     n.TypeID,
		CreatedAt:  n.CreatedAt,
		UpdatedAt:  n.UpdatedAt,
		OwnerActor: n.OwnerActor,
		Payload:    pStruct,
		Acl:        protoACL,
	}, nil
}

// storeVisibilityAdapter adapts *store.CanonicalStore to the
// acl.VisibilityReader interface. The store method is GetVisibleNodeIDs
// (renamed from the Python _get_visible_nodes); the acl interface uses
// the unprefixed name. Both signatures are otherwise identical.
type storeVisibilityAdapter struct {
	store *store.CanonicalStore
}

// VisibleNodeIDs satisfies acl.VisibilityReader.
func (a storeVisibilityAdapter) VisibleNodeIDs(ctx context.Context, tenantID string, actorIDs []string, nodeIDs []string) (map[string]struct{}, error) {
	return a.store.GetVisibleNodeIDs(ctx, tenantID, actorIDs, nodeIDs)
}

// parsePayloadFieldID parses a digit-only string into a uint32 in the
// FieldDef bound (1-65535). Mirrors payload.parseFieldID — duplicated
// here because that helper is unexported.
func parsePayloadFieldID(s string) (uint32, bool) {
	if s == "" {
		return 0, false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil || n == 0 || n > 65535 {
		return 0, false
	}
	return uint32(n), true
}
