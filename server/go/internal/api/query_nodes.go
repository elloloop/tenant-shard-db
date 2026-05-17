// SPDX-License-Identifier: AGPL-3.0-only

// QueryNodes RPC.
// Spec: docs/go-port/rpcs/QueryNodes.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:61 (rpc), :424-452
// (request/response), :675-690 (FieldFilter / FilterOp). Reference
// Python handler.
//
// Behavior parity with the Python handler, with one DELIBERATE behavior
// fix flagged by the spec:
//
//  1. Tenant gate runs first via s.checkTenant (sharding + region +
//     globalstore existence). Same call shape as every other
//     handler.
//  2. Trusted-actor: the wire-claimed actor is UNTRUSTED. We rebind via
//     auth.Authoritative(ctx, claimed) — mirrors commit fece3fb.
//  3. order_by, limit defaults are applied per spec (created_at, 100).
//  4. filters: name-keyed FieldFilter entries are translated to id-keyed
//     comparison filters via payload.FilterNamesToIDs. Eq/Ne/Lt/Le/Gt/Ge
//     are supported as AND-ed predicates per issue #501. CONTAINS and IN
//     remain reserved (the wire enum declares them but the server still
//     surfaces them as INVALID_ARGUMENT pending the full MongoDB-operator
//     allow-list in W1.10's queryfilter package).
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
	// identity. Even though the cut does not yet enforce
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

	// Translate FieldFilter wire shape -> id-keyed comparison filters.
	// Eq/Ne/Lt/Le/Gt/Ge are supported (issue #501); CONTAINS / IN and
	// any unknown operator value surface as INVALID_ARGUMENT.
	storeFilters, err := s.fieldFiltersToStoreFilters(typeName, req.GetFilters())
	if err != nil {
		resultStatus = "error"
		return nil, err
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = defaultQueryLimit
	}

	nodes, err := s.store.QueryNodes(ctx, store.QueryNodesArgs{
		TenantID:   tenantID,
		TypeID:     typeID,
		Filters:    storeFilters,
		OrderBy:    req.GetOrderBy(),
		Descending: req.GetDescending(),
		Limit:      limit,
		Offset:     int(req.GetOffset()),
	})
	if err != nil {
		resultStatus = "error"
		return nil, errs.Internal(ctx, "store: query nodes", err)
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

// fieldFiltersToStoreFilters translates the wire-shape FieldFilter
// slice into the canonical store's id-keyed [store.QueryFilter]
// list, applying name-to-id resolution against the schema registry.
//
// Supported operators (issue #501):
//
//   - EQ, NEQ, LT, LTE, GT, GTE
//
// CONTAINS and IN remain unsupported — they need a separate compiler
// (LIKE-with-wildcards, parameterised IN-list) and the issue defers
// them. Both surface as INVALID_ARGUMENT.
//
// Inlined operator shape: the Python SDK historically smuggled non-EQ
// operators through “Op=EQ, Value=Struct{"$gt": v}“. We honour that
// shape so the existing SDK call site keeps working — the inlined
// operator overrides the wire “op“ field. Unknown inlined keys (e.g.
// “$nin“, “$between“) still surface as INVALID_ARGUMENT.
func (s *Server) fieldFiltersToStoreFilters(typeName string, filters []*pb.FieldFilter) ([]store.QueryFilter, error) {
	if len(filters) == 0 {
		return nil, nil
	}
	// Resolve field name -> id via a single-entry call through
	// payload.FilterNamesToIDs. We re-use that helper per distinct
	// name so the unknown-field error shape stays identical to the
	// equality path and the schema-less unit-test path keeps working.
	resolveField := func(name string) (uint32, error) {
		ids, err := payload.FilterNamesToIDs(s.registry, typeName, map[string]any{name: nil})
		if err != nil {
			return 0, err
		}
		for id := range ids {
			return id, nil
		}
		// Defensive: FilterNamesToIDs always returns one id on success.
		return 0, errs.Errorf(codes.InvalidArgument,
			"QueryNodes: filter references unknown field %q", name)
	}

	nameToID := map[string]uint32{}
	out := make([]store.QueryFilter, 0, len(filters))
	for _, f := range filters {
		if f == nil {
			continue
		}
		field := f.GetField()
		raw := f.GetValue().AsInterface()

		fid, ok := nameToID[field]
		if !ok {
			id, err := resolveField(field)
			if err != nil {
				return nil, err
			}
			nameToID[field] = id
			fid = id
		}

		// Inlined operator detection: a Struct value carrying
		// ``$gt``/``$lt``/... keys. The Python SDK uses this shape to
		// fan a single FieldFilter into a multi-op predicate (e.g.
		// ``{"price": {"$gte": 100, "$lt": 200}}``). Emit one store
		// filter per inlined entry.
		if subs, ok := inlinedFilterOps(raw); ok {
			for _, sub := range subs {
				op, ok := storeFilterOpFromInlineKey(sub.key)
				if !ok {
					return nil, errs.Errorf(codes.InvalidArgument,
						"QueryNodes: inlined operator %q on field %q is not supported", sub.key, field)
				}
				out = append(out, store.QueryFilter{FieldID: fid, Op: op, Value: sub.value})
			}
			continue
		}

		op, err := storeFilterOpFromWire(field, f.GetOp())
		if err != nil {
			return nil, err
		}
		out = append(out, store.QueryFilter{FieldID: fid, Op: op, Value: raw})
	}
	return out, nil
}

// storeFilterOpFromWire maps the wire FilterOp enum to the store's
// comparison-operator type. CONTAINS / IN / unknown values surface as
// INVALID_ARGUMENT — the issue #501 cut covers Eq/Ne/Lt/Le/Gt/Ge only.
func storeFilterOpFromWire(field string, op pb.FilterOp) (store.QueryFilterOp, error) {
	switch op {
	case pb.FilterOp_EQ:
		return store.QueryFilterEq, nil
	case pb.FilterOp_NEQ:
		return store.QueryFilterNe, nil
	case pb.FilterOp_LT:
		return store.QueryFilterLt, nil
	case pb.FilterOp_LTE:
		return store.QueryFilterLe, nil
	case pb.FilterOp_GT:
		return store.QueryFilterGt, nil
	case pb.FilterOp_GTE:
		return store.QueryFilterGe, nil
	default:
		return 0, errs.Errorf(codes.InvalidArgument,
			"QueryNodes: filter operator %s on field %q is not supported (Eq/Ne/Lt/Le/Gt/Ge only)",
			op, field)
	}
}

// storeFilterOpFromInlineKey maps the MongoDB-style “$gt“ keys used
// by the Python SDK when smuggling non-EQ operators through Op=EQ.
func storeFilterOpFromInlineKey(key string) (store.QueryFilterOp, bool) {
	switch key {
	case "$eq":
		return store.QueryFilterEq, true
	case "$ne":
		return store.QueryFilterNe, true
	case "$lt":
		return store.QueryFilterLt, true
	case "$lte":
		return store.QueryFilterLe, true
	case "$gt":
		return store.QueryFilterGt, true
	case "$gte":
		return store.QueryFilterGe, true
	}
	return 0, false
}

// inlinedFilterOps reports whether the FieldFilter value carries the
// inlined operator shape (a JSON object whose first key starts with
// “$“) and, if so, returns the (op-key, value) pairs in iteration
// order. The shape historically lets callers express “{"price":
// {"$gt": 10, "$lt": 100}}“.
type inlinedFilterOp struct {
	key   string
	value any
}

func inlinedFilterOps(v any) ([]inlinedFilterOp, bool) {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return nil, false
	}
	for k := range m {
		if len(k) == 0 || k[0] != '$' {
			return nil, false
		}
	}
	out := make([]inlinedFilterOp, 0, len(m))
	for k, val := range m {
		out = append(out, inlinedFilterOp{key: k, value: val})
	}
	return out, true
}

// applyQueryACLFilter is the cross-tenant post-filter. It mirrors the
// canonical_store.can_access loop at grpc_server.py:1344-1358 but uses
// the bulk acl.Filter pattern (single VisibleNodeIDs JOIN, not N+1
// per-row checks).
//
// The filter is a no-op when the actor is the system bypass, when the
// actor is empty (no auth), or when no canonical store is wired up.
// Same-tenant owner access is honoured by VisibleNodeIDs — there is no
// dedicated short-circuit for "tenant member" because in the cut
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
		return nil, errs.Internal(ctx, "QueryNodes: acl filter", err)
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

// authActorToACLActor / (*Server).storeNodeToProto /
// storeVisibilityAdapter / parsePayloadFieldID — consolidated in
// helpers.go (round-3 dedupe). The shared authActorToACLActor
// maps admin -> system (defensive: picks up the read-path bypass in
// acl.Filter.FilterReadable). The shared storeNodeToProto preserves
// the typeName-aware payload.PayloadToStruct path used here.
