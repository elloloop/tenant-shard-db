// SPDX-License-Identifier: AGPL-3.0-only

// GetConnectedNodes RPC — Wave 2 of the Python -> Go server port (EPIC #407).
// Spec: docs/go-port/rpcs/GetConnectedNodes.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:91 (rpc), :707-718 (messages).
// Reference Python handler:
// server/python/entdb_server/api/grpc_server.py:1712-1744. Reference
// storage layer: server/python/entdb_server/apply/canonical_store.py:3267-3422.
//
// Behaviour preserved from the Python handler PLUS the Wave-2 enhancements
// requested by the task brief (BFS with bounded depth, cycle protection,
// per-step ACL filter via acl.Filter):
//
//   - Tenant gate runs first (s.checkTenant). PERMISSION_DENIED /
//     UNAVAILABLE / NOT_FOUND from the gate propagate unchanged.
//   - The wire actor in req.Context.Actor is UNTRUSTED. We resolve the
//     trusted identity via auth.Authoritative and ignore any wire claim
//     (CLAUDE.md / commit fece3fb).
//   - Source gate: if the trusted actor cannot read the source node, we
//     return an empty response (does NOT leak existence — same shape as
//     canonical_store.py:3291-3293).
//   - BFS traversal over GetEdgesFrom (outbound only — Python's storage
//     joins e.from_node_id = node_id only). Each frontier is run through
//     acl.Filter.FilterReadable BEFORE being yielded, so an ACL-denied
//     child is pruned both from the result set and from the next-level
//     frontier (it cannot leak its descendants either).
//   - Depth is bounded by MaxConnectedDepth. The proto has no `depth`
//     field today; mirroring the Python depth=1 contract, the default
//     traversal collects only direct children (depth 1), but the Go
//     traversal is structured so the constant can be raised in a follow-
//     up without rewriting the handler.
//   - Cycle protection via a visited set keyed on node_id (so a graph
//     containing back-edges or rings cannot cause unbounded work).
//   - Result-size limit honoured: BFS stops as soon as len(out) ==
//     limit. has_more = (len(out) == limit), matching Python's heuristic.
//   - Catch-all internal errors collapse to (nodes=[], OK) with
//     status="error" on the metric — matches the Python
//     `except Exception` swallow at grpc_server.py:1741-1744.
//
// No WAL append, no SQLite write, no audit-log entry, no quota debit.
// Read-only RPC.

package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/acl"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	"github.com/elloloop/tenant-shard-db/server/go/internal/payload"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

const getConnectedNodesMethod = "GetConnectedNodes"

// MaxConnectedDepth caps BFS traversal depth. The wire proto exposes no
// depth field today; this constant matches Python's effective depth=1
// contract (canonical_store.py joins one level only). Raising it is a
// behaviour change — track via EPIC #407 before flipping.
const MaxConnectedDepth = 1

// MaxConnectedResultLimit caps the per-call result size. The Python
// handler has no upper bound (open question 1 in the spec); the Go port
// clamps at 1000 to match the convention used elsewhere in the server.
// Limits ≤ 0 coerce to defaultConnectedLimit.
const MaxConnectedResultLimit = 1000

// defaultConnectedLimit mirrors the Python limit-or-100 default at
// grpc_server.py:1726.
const defaultConnectedLimit = 100

// GetConnectedNodes implements entdb.v1.EntDBService/GetConnectedNodes.
// See file header for the full contract.
func (s *Server) GetConnectedNodes(ctx context.Context, req *pb.GetConnectedNodesRequest) (*pb.GetConnectedNodesResponse, error) {
	start := time.Now()
	resultStatus := "ok"
	defer func() {
		metrics.RecordGRPCRequest(getConnectedNodesMethod, resultStatus, time.Since(start))
	}()

	tenantID := req.GetContext().GetTenantId()
	if err := s.checkTenant(ctx, tenantID); err != nil {
		// Tighter than Python (which collapses every error to nodes=[]).
		// Spec "Error contract" recommends propagating PERMISSION_DENIED
		// from the tenant gate; we follow that recommendation.
		resultStatus = "error"
		return nil, err
	}

	// Resolve trusted actor (the wire actor is UNTRUSTED).
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetContext().GetActor()))
	aclActor := authActorToACLActor(trusted)

	// Limit / offset coercion.
	limit := req.GetLimit()
	if limit <= 0 {
		limit = defaultConnectedLimit
	}
	if limit > MaxConnectedResultLimit {
		limit = MaxConnectedResultLimit
	}
	offset := req.GetOffset()
	if offset < 0 {
		offset = 0
	}

	// Empty source short-circuits to []. Matches Python's "vacuous query"
	// semantics (an empty node_id never matches any edge row).
	if req.GetNodeId() == "" {
		return &pb.GetConnectedNodesResponse{Nodes: []*pb.Node{}, HasMore: false}, nil
	}

	// Without a store, we cannot resolve any traversal. Match the
	// Python swallow: empty response, status="error".
	if s.store == nil {
		resultStatus = "error"
		return &pb.GetConnectedNodesResponse{Nodes: []*pb.Node{}, HasMore: false}, nil
	}

	// Build a per-call ACL filter. The Resolver is created with a nil
	// GroupMembershipReader because the W2 CanonicalStore does not yet
	// expose GroupsContaining (group expansion lands in W1.10 alongside
	// the store-side ResolveActorGroups port). Trivial expansion —
	// returning [actor] — is the correct fallback for the unit cases
	// pinned today; once group expansion ships, swap in a reader-backed
	// Resolver via a Server option without rewriting this handler.
	resolver := acl.NewResolver(nil)
	filter := acl.NewFilter(resolver, storeVisibilityAdapter{s: s.store})

	// Source gate: if the actor cannot read the source node, return
	// empty. Mirrors canonical_store.py:3291-3293.
	if !aclActor.IsZero() && !aclActor.IsSystem() {
		visibleSrc, err := filter.FilterReadable(ctx, tenantID, aclActor, []string{req.GetNodeId()})
		if err != nil {
			resultStatus = "error"
			return &pb.GetConnectedNodesResponse{Nodes: []*pb.Node{}, HasMore: false}, nil
		}
		if len(visibleSrc) == 0 {
			// Source not accessible. Return empty — do NOT leak existence.
			return &pb.GetConnectedNodesResponse{Nodes: []*pb.Node{}, HasMore: false}, nil
		}
	}

	edgeTypeID := req.GetEdgeTypeId()
	collected, err := s.bfsConnected(ctx, tenantID, req.GetNodeId(), edgeTypeID, aclActor, filter, limit, offset)
	if err != nil {
		// Python `except Exception` -> nodes=[]. We mirror; the metric
		// is recorded as "error" so the swallow doesn't inflate the
		// success counter.
		resultStatus = "error"
		return &pb.GetConnectedNodesResponse{Nodes: []*pb.Node{}, HasMore: false}, nil
	}

	out := make([]*pb.Node, 0, len(collected))
	for _, n := range collected {
		pn, perr := storeNodeToProto(s.registry, n)
		if perr != nil {
			// A single bad row collapses the whole response to empty —
			// matches Python's broad except. Recording as error so the
			// metric reflects the swallow.
			resultStatus = "error"
			return &pb.GetConnectedNodesResponse{Nodes: []*pb.Node{}, HasMore: false}, nil
		}
		out = append(out, pn)
	}

	return &pb.GetConnectedNodesResponse{
		Nodes:   out,
		HasMore: int32(len(out)) == limit,
	}, nil
}

// bfsConnected runs a depth-bounded BFS from sourceID over GetEdgesFrom,
// returning up to `limit` connected nodes (after applying offset and
// per-step ACL filtering). Cycles are tolerated via the visited set.
//
// The traversal is intentionally structured around acl.Filter rather
// than baking ACL into SQL: the spec calls out factoring ACL primitives
// into the acl package (canonicalstore/acl.go in the spec); we do the
// per-step filter here.
func (s *Server) bfsConnected(
	ctx context.Context,
	tenantID, sourceID string,
	edgeTypeID int32,
	aclActor acl.Actor,
	filter *acl.Filter,
	limit, offset int32,
) ([]*store.Node, error) {
	// Visited starts with the source so we never yield the source itself
	// (Python's SQL JOIN matches `e.from_node_id = node_id` and the
	// returned rows are the `to_node_id` side — the source never appears
	// in the result).
	visited := map[string]struct{}{sourceID: {}}
	frontier := []string{sourceID}

	// `seen` counts how many connected nodes we have produced. We apply
	// `offset` by skipping the first N before adding to `out`.
	var seen int32
	out := make([]*store.Node, 0, limit)

	systemBypass := aclActor.IsSystem()
	etID := edgeTypeID

	for depth := 0; depth < MaxConnectedDepth && len(frontier) > 0 && int32(len(out)) < limit; depth++ {
		// Collect outgoing edges from every node in the current
		// frontier. We deliberately deduplicate target ids before
		// fetching nodes so a fan-in pattern doesn't double-count.
		nextIDs := make([]string, 0, len(frontier))
		nextSeen := map[string]struct{}{}
		for _, src := range frontier {
			edges, err := s.store.GetEdgesFrom(ctx, tenantID, src, &etID, 0)
			if err != nil {
				return nil, err
			}
			for _, e := range edges {
				if _, ok := visited[e.ToNodeID]; ok {
					continue // cycle protection
				}
				if _, ok := nextSeen[e.ToNodeID]; ok {
					continue
				}
				nextSeen[e.ToNodeID] = struct{}{}
				nextIDs = append(nextIDs, e.ToNodeID)
			}
		}

		if len(nextIDs) == 0 {
			break
		}

		// Per-step ACL filter. System actor bypasses (matches
		// canonical_store.py:3281-3289). For zero actor (uncommon —
		// usually the auth interceptor fills one in) we also bypass to
		// preserve the Python "no auth context, no filter" fallback used
		// by the Applier replay path.
		var allowed []string
		if systemBypass || aclActor.IsZero() {
			allowed = nextIDs
		} else {
			filtered, err := filter.FilterReadable(ctx, tenantID, aclActor, nextIDs)
			if err != nil {
				return nil, err
			}
			allowed = filtered
		}

		// Mark every node in the original next set as visited so a
		// subsequent depth doesn't revisit ACL-denied nodes either —
		// the visited set is for cycle protection, not for re-trying.
		for _, id := range nextIDs {
			visited[id] = struct{}{}
		}

		if len(allowed) == 0 {
			// Nothing visible at this depth — the BFS frontier becomes
			// empty next iteration. Break early to skip the GetNodes
			// round-trip.
			frontier = nil
			continue
		}

		// Materialise the allowed node rows ordered by created_at DESC
		// to match the Python contract (canonical_store.py:3328, 3358,
		// 3405). GetNodes itself does not order; we sort post-fetch.
		nodes, _, err := s.store.GetNodes(ctx, tenantID, allowed)
		if err != nil {
			return nil, err
		}
		sortNodesByCreatedAtDesc(nodes)

		for _, n := range nodes {
			if seen < offset {
				seen++
				continue
			}
			out = append(out, n)
			seen++
			if int32(len(out)) >= limit {
				break
			}
		}

		// The next BFS frontier is every visible node we just yielded
		// (or would have yielded had limit/offset not stopped us). This
		// keeps ACL pruning sticky: a denied node prunes its sub-tree.
		frontier = allowed
	}

	return out, nil
}

// sortNodesByCreatedAtDesc orders a node slice by created_at DESC,
// stable on (created_at, node_id). Mirrors the ORDER BY clause Python
// uses on the connected-nodes SQL.
func sortNodesByCreatedAtDesc(nodes []*store.Node) {
	// In-place insertion sort: result sets here are bounded by the BFS
	// frontier size (typically small; the limit ceiling is 1000). Avoids
	// pulling in `sort` for a tight, allocation-free sort.
	for i := 1; i < len(nodes); i++ {
		j := i
		for j > 0 {
			a, b := nodes[j-1], nodes[j]
			if a.CreatedAt > b.CreatedAt {
				break
			}
			if a.CreatedAt == b.CreatedAt && a.NodeID <= b.NodeID {
				break
			}
			nodes[j-1], nodes[j] = nodes[j], nodes[j-1]
			j--
		}
	}
}

// authActorToACLActor bridges the auth.Actor (caller identity admitted
// by the interceptor) and the acl.Actor (ACL grant subject). user/system
// translate directly; admin maps to system because admins bypass ACL on
// the read path (matches Python's "admin can read everything" handler-
// level convention).
func authActorToACLActor(a auth.Actor) acl.Actor {
	if a.IsZero() {
		return acl.Actor{}
	}
	switch a.Kind() {
	case auth.KindUser:
		return acl.User(a.ID())
	case auth.KindSystem:
		return acl.System(a.ID())
	case auth.KindAdmin:
		// Admins bypass ACL for read on the Python side; expressing them
		// as system: actors here picks up the same bypass in
		// acl.Filter.FilterReadable.
		return acl.System(a.ID())
	default:
		// Service / unknown — fall through to user: form so the filter
		// has a non-system, non-empty actor.
		return acl.User(a.ID())
	}
}

// storeVisibilityAdapter satisfies acl.VisibilityReader by forwarding to
// CanonicalStore.GetVisibleNodeIDs. Mirrors the same adapter used in
// acl/integration_test.go.
type storeVisibilityAdapter struct {
	s *store.CanonicalStore
}

func (a storeVisibilityAdapter) VisibleNodeIDs(ctx context.Context, tenantID string, actorIDs []string, nodeIDs []string) (map[string]struct{}, error) {
	return a.s.GetVisibleNodeIDs(ctx, tenantID, actorIDs, nodeIDs)
}

// storeNodeToProto converts a store.Node to a wire pb.Node. Payload
// stays field-id-keyed per CLAUDE.md invariant #6 — the SDK names the
// fields client-side. Mirrors Python's _node_to_proto at
// grpc_server.py:1693-1710.
//
// The schema registry is currently unused on egress (the wire stays
// id-keyed and structpb encodes scalars without help). It is accepted as
// a parameter so a future kind-aware encoding can be plumbed in without
// changing call sites.
func storeNodeToProto(_ interface{}, n *store.Node) (*pb.Node, error) {
	idPayload, err := decodeIDKeyedPayload(n.PayloadJSON)
	if err != nil {
		return nil, err
	}
	pStruct, err := payload.PayloadToStruct(nil, "", idPayload)
	if err != nil {
		return nil, err
	}
	aclEntries, err := decodeACLEntries(n.ACLJSON)
	if err != nil {
		return nil, err
	}
	return &pb.Node{
		TenantId:   n.TenantID,
		NodeId:     n.NodeID,
		TypeId:     n.TypeID,
		CreatedAt:  n.CreatedAt,
		UpdatedAt:  n.UpdatedAt,
		OwnerActor: n.OwnerActor,
		Payload:    pStruct,
		Acl:        aclEntries,
	}, nil
}

// decodeIDKeyedPayload parses a raw JSON object whose keys are
// stringified field_ids ({"1": ...}) into the uint32-keyed map the
// payload package consumes. Empty / null JSON yields an empty map.
func decodeIDKeyedPayload(raw string) (map[uint32]any, error) {
	out := map[uint32]any{}
	if raw == "" || raw == "null" {
		return out, nil
	}
	tmp := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &tmp); err != nil {
		return nil, err
	}
	for k, v := range tmp {
		id, ok := parsePositiveDigits(k)
		if !ok {
			// A non-digit key on disk would violate invariant #6. We
			// drop silently (the read path can't fix old rows; the
			// applier-side guard prevents new ones).
			continue
		}
		out[id] = v
	}
	return out, nil
}

// decodeACLEntries parses the on-disk acl_blob ([{principal, permission}])
// into the wire pb.AclEntry slice. Empty / null yields nil.
func decodeACLEntries(raw string) ([]*pb.AclEntry, error) {
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var tmp []store.ACLEntry
	if err := json.Unmarshal([]byte(raw), &tmp); err != nil {
		return nil, err
	}
	if len(tmp) == 0 {
		return nil, nil
	}
	out := make([]*pb.AclEntry, 0, len(tmp))
	for _, e := range tmp {
		out = append(out, &pb.AclEntry{
			Grantee:    e.Principal,
			Permission: e.Permission,
		})
	}
	return out, nil
}

// parsePositiveDigits parses a non-empty digit-only string into a
// uint32 in [1, 65535]. Mirrors payload.parseFieldID (unexported there).
func parsePositiveDigits(s string) (uint32, bool) {
	if s == "" {
		return 0, false
	}
	var n uint32
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		d := uint32(c - '0')
		next := n*10 + d
		if next < n {
			return 0, false
		}
		n = next
	}
	if n == 0 || n > 65535 {
		return 0, false
	}
	return n, true
}
