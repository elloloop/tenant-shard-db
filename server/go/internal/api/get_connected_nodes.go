// SPDX-License-Identifier: AGPL-3.0-only

// GetConnectedNodes RPC.
// Spec: docs/go-port/rpcs/GetConnectedNodes.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:91 (rpc), :707-718 (messages).
//
// PAGINATION (ADR-029 — intentionally NOT cursor-paginated): connected-
// nodes is a BFS graph traversal bounded by `limit` + MaxConnectedDepth,
// not an ordered scan over a single index. A keyset cursor over a graph
// frontier is ill-defined — the "last row" of one page is not a stable
// seek anchor for the next, because the frontier is recomputed from the
// edge set, ACL-pruned per step, and deduplicated across fan-in. This RPC
// is therefore a deliberately BOUNDED READ: it returns at most `limit`
// reachable, ACL-visible nodes within MaxConnectedDepth, with an EXACT
// `has_more` (the traversal collects one probe node beyond `limit` to
// distinguish "page is full" from "more exist"). Callers that need the
// full neighbourhood raise `limit`; there is no `next_page_token`. See
// ADR-029 "GetConnectedNodes (bounded traversal, not cursor-paginated)".
//
// Behaviour: BFS with bounded depth, cycle protection, per-step ACL
// filter via acl.Filter. Notable behaviours:
//
//   - Tenant gate runs first (s.checkTenant). PERMISSION_DENIED /
//     UNAVAILABLE / NOT_FOUND from the gate propagate unchanged.
//   - The wire actor in req.Context.Actor is UNTRUSTED. We resolve the
//     trusted identity via auth.Authoritative and ignore any wire claim
//     (CLAUDE.md / commit fece3fb).
//   - Source gate: if the trusted actor cannot read the source node, we
//     return an empty response (does NOT leak existence).
//   - BFS traversal over GetEdgesFrom (outbound only). Each frontier is
//     run through acl.Filter.FilterReadable BEFORE being yielded, so an
//     ACL-denied child is pruned both from the result set and from the
//     next-level frontier (it cannot leak its descendants either).
//   - Depth is bounded by MaxConnectedDepth. The proto has no `depth`
//     field today; the default traversal collects only direct children
//     (depth 1), but the Go traversal is structured so the constant can
//     be raised in a follow-up without rewriting the handler.
//   - Cycle protection via a visited set keyed on node_id (so a graph
//     containing back-edges or rings cannot cause unbounded work).
//   - Result-size limit honoured: BFS collects up to limit+1 nodes (one
//     probe beyond the page) then stops. The probe row is trimmed before
//     egress and has_more = (a probe row was collected) — so has_more is
//     EXACT, not the old (len==limit) heuristic that over-reported true on
//     an exactly-full last page.
//   - Catch-all internal errors collapse to (nodes=[], OK) with
//     status="error" on the metric.
//
// No WAL append, no SQLite write, no audit-log entry, no quota debit.
// Read-only RPC.

package api

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/acl"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

const getConnectedNodesMethod = "GetConnectedNodes"

// MaxConnectedDepth caps BFS traversal depth. The wire proto exposes no
// depth field today; the effective depth=1 contract joins one level only.
// Raising it is a behaviour change — consider carefully before flipping.
const MaxConnectedDepth = 1

// MaxConnectedResultLimit caps the per-call result size at 1000, matching
// the convention used elsewhere in the server (open question 1 in the spec).
// Limits ≤ 0 coerce to defaultConnectedLimit.
const MaxConnectedResultLimit = 1000

// defaultConnectedLimit is the limit-or-100 default for connected-node queries.
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

	// Empty source short-circuits to [] — an empty node_id never matches
	// any edge row.
	if req.GetNodeId() == "" {
		return &pb.GetConnectedNodesResponse{Nodes: []*pb.Node{}, HasMore: false}, nil
	}

	// Without a store, we cannot resolve any traversal: return empty
	// response, status="error".
	if s.store == nil {
		resultStatus = "error"
		return &pb.GetConnectedNodesResponse{Nodes: []*pb.Node{}, HasMore: false}, nil
	}

	// Build a per-call ACL filter. The Resolver is created with a nil
	// GroupMembershipReader because the CanonicalStore does not yet
	// expose GroupsContaining. Trivial expansion — returning [actor] —
	// is the correct fallback; once group expansion ships, swap in a
	// reader-backed Resolver via a Server option without rewriting this
	// handler.
	resolver := acl.NewResolver(nil)
	filter := acl.NewFilter(resolver, storeVisibilityAdapter{s: s.store})

	// Source gate: if the actor cannot read the source node, return empty.
	if !aclActor.IsZero() && !aclActor.IsSystem() {
		visibleSrc, err := filter.FilterReadable(ctx, tenantID, aclActor, []string{req.GetNodeId()})
		if err != nil {
			// Genuine ACL-store fault (#573) — not "not visible". Surface it.
			resultStatus = "error"
			if c := errs.Code(err); c != codes.Unknown {
				return nil, errs.Errorf(c, "GetConnectedNodes: %v", err)
			}
			return nil, errs.Internal(ctx, "GetConnectedNodes: acl", err)
		}
		if len(visibleSrc) == 0 {
			// Source not accessible. Return empty — do NOT leak existence.
			// This is an intentional empty result, not a fault.
			return &pb.GetConnectedNodesResponse{Nodes: []*pb.Node{}, HasMore: false}, nil
		}
	}

	edgeTypeID := req.GetEdgeTypeId()
	// Collect one probe node beyond the page so has_more is exact. The
	// probe is trimmed below before egress.
	collected, err := s.bfsConnected(ctx, tenantID, req.GetNodeId(), edgeTypeID, aclActor, filter, limit+1, offset)
	if err != nil {
		// Surface genuine post-open faults (#573) instead of masking the
		// traversal failure as an empty graph. Preserve typed sentinels.
		resultStatus = "error"
		if c := errs.Code(err); c != codes.Unknown {
			return nil, errs.Errorf(c, "GetConnectedNodes: %v", err)
		}
		return nil, errs.Internal(ctx, "GetConnectedNodes: traverse", err)
	}

	// has_more is exact: the traversal was asked for limit+1; a probe row
	// beyond the page means more reachable nodes exist. Trim it off.
	hasMore := int32(len(collected)) > limit
	if hasMore {
		collected = collected[:limit]
	}

	out := make([]*pb.Node, 0, len(collected))
	for _, n := range collected {
		pn, perr := s.storeNodeToProto(0, n)
		if perr != nil {
			// A row that will not marshal is corrupt stored state, not an
			// absent result — surface it rather than dropping rows.
			resultStatus = "error"
			return nil, errs.Internal(ctx, "GetConnectedNodes: marshal row", perr)
		}
		out = append(out, pn)
	}

	return &pb.GetConnectedNodesResponse{
		Nodes:   out,
		HasMore: hasMore,
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
	// (the SQL JOIN matches `e.from_node_id = node_id`; the returned
	// rows are the `to_node_id` side — the source never appears in the
	// result).
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

		// Per-step ACL filter. System actor bypasses. For zero actor
		// (uncommon — usually the auth interceptor fills one in) we also
		// bypass to preserve the "no auth context, no filter" fallback
		// used by the Applier replay path.
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

		// Materialise the allowed node rows ordered by created_at DESC.
		// GetNodes itself does not order; we sort post-fetch.
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
// stable on (created_at, node_id).
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

// authActorToACLActor / storeVisibilityAdapter /
// (*Server).storeNodeToProto / decodeIDKeyedPayload / decodeACLEntries
// /parsePayloadFieldID — consolidated in helpers.go (round-3
// dedupe). The shared method form takes typeName (passed "" here
// because GetConnectedNodes operates on heterogeneous node sets).
