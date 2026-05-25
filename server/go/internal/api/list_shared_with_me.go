// ListSharedWithMe RPC.
// Spec: docs/go-port/rpcs/ListSharedWithMe.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:100 (rpc), :757-766
// (request/response).
//
// Semantics:
//
//   - Read-only. The caller is the IMPLICIT recipient — the actor
//     identity in the wire payload is UNTRUSTED and is overwritten by
//     the trusted Identity from the auth interceptor before any data
//     access. This is the privilege-escalation invariant: a malicious
//     `user:eve` MUST NOT be able to enumerate `user:alice`'s shares
//     by setting context.actor = "user:alice" on the wire.
//   - Aggregates two indexes:
//     1. Per-tenant `node_access`: same-tenant grants where the caller
//        (or any group they transitively belong to) is the grantee.
//        Filters expired (`expires_at`) and `deny` permission.
//     2. Cross-tenant `global_store.shared_index`: rows written by
//        ShareNode when the recipient lives in a different tenant from
//        the source node. The caller's BARE actor string only — group
//        fan-out happens at write time. Cross-tenant `shared_index` has
//        NO `expires_at` column (parity caveat — flagged in spec
//        "Open questions / risks" #3).
//   - Pagination (ADR-029): a UNIFIED keyset cursor spans BOTH merged
//     sources. The per-tenant node_access source (ordered by granted_at)
//     and the cross-tenant shared_index source (ordered by shared_at) are
//     both seeked with the same `(timestamp, source_tenant, node_id) <
//     cursor` predicate, ordered (timestamp DESC, source_tenant DESC,
//     node_id DESC), merged, deduped by (source_tenant, node_id), and the
//     top `page_size` are returned. `next_page_token` is the tuple of the
//     last merged row; following it resumes exactly after it in both
//     sources, so the stream never skips or duplicates a node. `page_size`
//     is the AIP-158 alias for `limit` and takes precedence. The
//     deprecated `offset` keeps the legacy independent-per-source
//     behaviour for backward compatibility and is mutually exclusive with
//     `page_token` (INVALID_ARGUMENT).
//   - `has_more` is EXACT under the keyset path: the merge fetches
//     page_size+1 candidates from each source and a probe row beyond the
//     page means more exist. Under the deprecated offset path `has_more`
//     stays the old `len(nodes) >= limit` over-report.
//   - Cross-tenant aggregation reads OTHER tenants' SQLite files via
//     store.GetNode(source_tenant, node_id). This is the ONE allowed
//     cross-tenant read in EntDB (CLAUDE.md invariant 4) and is only
//     legal because the `shared_index` row IS the authorisation token —
//     ShareNode wrote a `node_access` grant in the source tenant that
//     authorises this read.
//   - Stale entries (source tenant unloaded, node deleted, permission
//     revoked but global index lagging) are silently skipped. They are
//     eventual-consistency artefacts, not errors.
//   - Globalstore failures degrade gracefully: log + return per-tenant
//     results only.
//   - Per-tenant SQLite errors are also swallowed — the handler returns
//     `nodes=[]` with status OK rather than surfacing the fault. This
//     is hostile to debugging but load-bearing for parity; flagged for
//     tightening in a separate issue.
//   - Limit is clamped to [0, 1000]; default when zero is 100.
//     Negative limit/offset are clamped to 0 rather than passed
//     through to SQLite (where negative LIMIT means "unlimited" —
//     undefined behaviour).
//
// Note on group resolution: this implementation does NOT yet expand
// groups for the per-tenant query (group_users-backed
// `acl.Resolver.Expand`). The bare actor is passed as a single-element
// actor list, matching the "user has no groups" steady state. Group-
// share inclusion is covered by the cross-tenant path because ShareNode
// fans out group → members at write time and writes one `shared_index`
// row per (member, source_tenant, node). When the per-tenant group
// resolver lands, swap in `acl.Resolver.ExpandIDs(ctx, tenant, actor)`
// here.
//
// NOT modified by this PR: server/go/internal/api/server.go (per task statement).

package api

import (
	"context"
	"errors"
	"log"
	"sort"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

const (
	listSharedWithMeMethod       = "ListSharedWithMe"
	listSharedWithMeDefaultLimit = 100
	listSharedWithMeMaxLimit     = 1000
)

// ListSharedWithMe implements entdb.v1.EntDBService/ListSharedWithMe.
// See file header for the parity contract.
func (s *Server) ListSharedWithMe(
	ctx context.Context,
	req *pb.ListSharedWithMeRequest,
) (*pb.ListSharedWithMeResponse, error) {
	start := time.Now()
	statusLabel := "ok"
	defer func() {
		metrics.RecordGRPCRequest(listSharedWithMeMethod, statusLabel, time.Since(start))
	}()

	tenantID := req.GetContext().GetTenantId()
	if err := s.checkTenant(ctx, tenantID); err != nil {
		statusLabel = "error"
		return nil, err
	}

	// Trusted-actor rebinding. The wire-claimed actor is IGNORED: the
	// caller IS the implicit recipient. If the auth interceptor did not
	// run (unit tests, no-auth deployments), Authoritative falls
	// through to the wire claim — which is fine because tests opt into
	// a known actor and production MUST run the interceptor.
	claimed := auth.ParseActor(req.GetContext().GetActor())
	actor := auth.Authoritative(ctx, claimed)
	actorStr := actor.String()
	if actorStr == "" {
		// No identity available at all — treat as "no shares" rather
		// than an error (an empty actor string yields zero rows from
		// both indexes).
		return emptyListSharedWithMeResponse(), nil
	}

	// Page size: prefer the AIP-158 page_size, fall back to the legacy
	// limit, then the default. Clamp to [0, MaxLimit].
	pageSize := req.GetPageSize()
	if pageSize <= 0 {
		pageSize = req.GetLimit()
	}
	if pageSize <= 0 {
		pageSize = listSharedWithMeDefaultLimit
	}
	if pageSize > listSharedWithMeMaxLimit {
		pageSize = listSharedWithMeMaxLimit
	}

	// Deprecated offset path: when a caller still sends offset (and no
	// page_token), fall back to the legacy independent-per-source paging
	// for backward compatibility. New callers use the unified keyset below.
	if req.GetPageToken() == "" && req.GetOffset() > 0 {
		return s.listSharedWithMeLegacy(ctx, tenantID, actorStr, pageSize, req.GetOffset())
	}

	// Unified keyset cursor (ADR-029). The token is bound to the recipient
	// + tenant by a fingerprint; a token from a different recipient/tenant
	// or mixed with the deprecated offset is INVALID_ARGUMENT.
	fingerprint := sharedFingerprint(tenantID, actorStr)
	var cursor *sharedPageCursor
	if tok := req.GetPageToken(); tok != "" {
		if req.GetOffset() != 0 {
			statusLabel = "error"
			return nil, errs.Errorf(codes.InvalidArgument,
				"page_token and the deprecated offset are mutually exclusive; use one")
		}
		c, derr := decodeSharedPageToken(tok, fingerprint)
		if derr != nil {
			statusLabel = "error"
			return nil, derr
		}
		cursor = &c
	}

	merged, hasMore, err := s.mergeSharedWithMe(ctx, tenantID, actorStr, int(pageSize), cursor)
	if err != nil {
		statusLabel = "error"
		return nil, err
	}

	// Mint the next-page cursor from the last merged row when more exist,
	// so a response that omits rows always carries a token (ADR-029
	// invariant 3).
	var nextPageToken string
	if hasMore && len(merged) > 0 {
		last := merged[len(merged)-1]
		nextPageToken = encodeSharedPageToken(sharedPageCursor{
			Fingerprint:  fingerprint,
			Timestamp:    last.ts,
			SourceTenant: last.node.TenantID,
			NodeID:       last.node.NodeID,
		})
	}

	// Convert store.Node -> pb.Node. Payload translation lives in the
	// payload package and is gated on a schema.Registry; in its absence
	// we surface the field-id-keyed shape verbatim by leaving Payload
	// nil — callers that need a typed payload pull it via
	// GetNode anyway. ACL is similarly lifted into pb.AclEntry only
	// when consumers need it; this RPC's contract test
	// (test_grpc_contract.py:339-350) is permissive, asserting only
	// well-formed nodes with non-empty IDs.
	out := make([]*pb.Node, 0, len(merged))
	for _, m := range merged {
		out = append(out, &pb.Node{
			TenantId:   m.node.TenantID,
			NodeId:     m.node.NodeID,
			TypeId:     m.node.TypeID,
			CreatedAt:  m.node.CreatedAt,
			UpdatedAt:  m.node.UpdatedAt,
			OwnerActor: m.node.OwnerActor,
		})
	}
	return &pb.ListSharedWithMeResponse{
		Nodes:         out,
		HasMore:       hasMore,
		NextPageToken: nextPageToken,
	}, nil
}

// sharedMergeRow is one candidate in the unified shared-with-me stream:
// the resolved node plus the timestamp it sorts on (granted_at for the
// per-tenant source, shared_at for the cross-tenant source).
type sharedMergeRow struct {
	node *store.Node
	ts   int64
}

// mergeSharedWithMe runs the unified keyset (ADR-029) across both shared
// sources. It fetches up to pageSize+1 candidates from each source past
// the cursor, merges them in (timestamp DESC, source_tenant DESC, node_id
// DESC) order, dedupes by (source_tenant, node_id), and returns the top
// pageSize plus an EXACT has_more (a probe row beyond the page existed).
//
// Why pageSize+1 from EACH source is sufficient: the globally-next
// pageSize+1 distinct tuples are a subset of the union of each source's
// own next pageSize+1 tuples (each source is already sorted by the same
// key), so the merged top pageSize is exact and the (pageSize+1)th tuple
// is a sound existence probe. Dedup only removes rows; it never hides the
// existence of a further page.
func (s *Server) mergeSharedWithMe(ctx context.Context, tenantID, actorStr string, pageSize int, cursor *sharedPageCursor) ([]sharedMergeRow, bool, error) {
	fetch := pageSize + 1

	candidates := make([]sharedMergeRow, 0, 2*fetch)

	// 1. Per-tenant node_access source. Errors are swallowed for parity
	//    (file header) — keep going with the cross-tenant source.
	if s.store != nil {
		var sc *store.SharedCursor
		if cursor != nil {
			sc = &store.SharedCursor{Timestamp: cursor.Timestamp, SourceTenant: cursor.SourceTenant, NodeID: cursor.NodeID}
		}
		rows, err := s.store.ListSharedWithMePaged(ctx, tenantID, []string{actorStr}, fetch, sc)
		if err != nil {
			log.Printf("ListSharedWithMe: per-tenant query failed for %s/%s: %v", tenantID, actorStr, err)
		} else {
			for _, r := range rows {
				candidates = append(candidates, sharedMergeRow{node: r.Node, ts: r.GrantedAt})
			}
		}
	}

	// 2. Cross-tenant shared_index source. Resolve each entry through the
	//    (allowed) cross-tenant store.GetNode read. Stale entries are
	//    skipped silently. Errors are swallowed for parity.
	if s.global != nil && s.store != nil {
		var sc *globalstore.SharedCursor
		if cursor != nil {
			sc = &globalstore.SharedCursor{SharedAt: cursor.Timestamp, SourceTenant: cursor.SourceTenant, NodeID: cursor.NodeID}
		}
		entries, gerr := s.global.ListSharedToUserPaged(ctx, actorStr, fetch, sc)
		if gerr != nil {
			log.Printf("ListSharedWithMe: shared_index read failed for %s: %v", actorStr, gerr)
		} else {
			for _, e := range entries {
				n, err := s.store.GetNode(ctx, e.SourceTenant, e.NodeID)
				if err != nil || n == nil {
					if err != nil && !errors.Is(err, store.ErrNodeNotFound) {
						log.Printf("ListSharedWithMe: skip stale shared_index (%s/%s): %v",
							e.SourceTenant, e.NodeID, err)
					}
					continue
				}
				candidates = append(candidates, sharedMergeRow{node: n, ts: e.SharedAt})
			}
		}
	}

	// Merge: sort the combined candidates in the unified DESC order, then
	// dedupe by (source_tenant, node_id), keeping the first (highest-
	// sorting) occurrence. A node present in both sources collapses to one
	// row anchored at its higher timestamp.
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.ts != b.ts {
			return a.ts > b.ts
		}
		if a.node.TenantID != b.node.TenantID {
			return a.node.TenantID > b.node.TenantID
		}
		return a.node.NodeID > b.node.NodeID
	})
	seen := make(map[string]struct{}, len(candidates))
	deduped := candidates[:0]
	for _, c := range candidates {
		key := c.node.TenantID + "\x00" + c.node.NodeID
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, c)
	}

	hasMore := len(deduped) > pageSize
	if hasMore {
		deduped = deduped[:pageSize]
	}
	return deduped, hasMore, nil
}

// listSharedWithMeLegacy is the pre-ADR-029 independent-per-source paging
// kept for callers still sending the deprecated `offset`. limit/offset are
// applied to each source separately then merged + deduped (page boundaries
// do not line up). has_more stays the old `len >= limit` over-report.
func (s *Server) listSharedWithMeLegacy(ctx context.Context, tenantID, actorStr string, limit, offset int32) (*pb.ListSharedWithMeResponse, error) {
	var nodes []*store.Node
	if s.store != nil {
		got, err := s.store.ListSharedWithMe(ctx, tenantID, []string{actorStr}, int(limit), int(offset))
		if err != nil {
			log.Printf("ListSharedWithMe: per-tenant query failed for %s/%s: %v", tenantID, actorStr, err)
		} else {
			nodes = got
		}
	}
	seen := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		seen[n.TenantID+"\x00"+n.NodeID] = struct{}{}
	}
	if s.global != nil && s.store != nil {
		entries, gerr := s.global.ListSharedToUser(ctx, actorStr, int(limit), int(offset))
		if gerr != nil {
			log.Printf("ListSharedWithMe: shared_index read failed for %s: %v", actorStr, gerr)
		} else {
			for _, e := range entries {
				key := e.SourceTenant + "\x00" + e.NodeID
				if _, dup := seen[key]; dup {
					continue
				}
				n, err := s.store.GetNode(ctx, e.SourceTenant, e.NodeID)
				if err != nil || n == nil {
					if err != nil && !errors.Is(err, store.ErrNodeNotFound) {
						log.Printf("ListSharedWithMe: skip stale shared_index (%s/%s): %v",
							e.SourceTenant, e.NodeID, err)
					}
					continue
				}
				nodes = append(nodes, n)
				seen[key] = struct{}{}
			}
		}
	}
	out := make([]*pb.Node, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, &pb.Node{
			TenantId:   n.TenantID,
			NodeId:     n.NodeID,
			TypeId:     n.TypeID,
			CreatedAt:  n.CreatedAt,
			UpdatedAt:  n.UpdatedAt,
			OwnerActor: n.OwnerActor,
		})
	}
	return &pb.ListSharedWithMeResponse{
		Nodes:   out,
		HasMore: int32(len(out)) >= limit,
	}, nil
}

// emptyListSharedWithMeResponse is the canned empty response. Nodes is
// a non-nil empty slice (non-nil repeated-field default).
func emptyListSharedWithMeResponse() *pb.ListSharedWithMeResponse {
	return &pb.ListSharedWithMeResponse{
		Nodes:   []*pb.Node{},
		HasMore: false,
	}
}
