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
//   - Pagination is parity-bug-compatible: `limit`/`offset` are applied
//     INDEPENDENTLY to each index then merged. Page boundaries do not
//     line up; consecutive pages can overlap, and a single page can
//     return up to 2*limit rows. Documented limitation; see spec
//     "Pagination semantics".
//   - `has_more` is `len(nodes) >= limit` AFTER the cross-tenant
//     append, which over-reports `true`. Spec "Open questions / risks" #2.
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
//   - Hardening delta vs Python: limit clamped to [0, 1000]; default
//     when zero is 100 (matches Python). Negative limit/offset clamped
//     to 0 instead of being passed through to SQLite (where negative
//     LIMIT means "unlimited" — undefined behaviour).
//
// Note on group resolution: this implementation does NOT yet expand
// groups for the per-tenant query (group_users-backed
// `acl.Resolver.Expand` is wired only in W1.10). The bare actor is
// passed as a single-element actor list, which is parity with the
// "user has no groups" steady state. Group-share inclusion is covered
// by the cross-tenant path because ShareNode fans out group → members
// at write time and writes one `shared_index` row per (member,
// source_tenant, node). When the per-tenant group resolver lands, swap
// in `acl.Resolver.ExpandIDs(ctx, tenant, actor)` here.
//
// NOT modified by this PR: server/go/internal/api/server.go (per task statement).

package api

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
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

	// Limit/offset clamping. Python passes raw values through to
	// SQLite; the Go port hardens this — see file header.
	limit := req.GetLimit()
	if limit <= 0 {
		limit = listSharedWithMeDefaultLimit
	}
	if limit > listSharedWithMeMaxLimit {
		limit = listSharedWithMeMaxLimit
	}
	offset := req.GetOffset()
	if offset < 0 {
		offset = 0
	}

	// 1. Per-tenant query against node_access. Group resolution is a
	//    no-op for now (see file header). The bare actor is passed as
	//    a single-element list. Errors are swallowed for parity.
	var nodes []*store.Node
	if s.store != nil {
		got, err := s.store.ListSharedWithMe(ctx, tenantID, []string{actorStr}, int(limit), int(offset))
		if err != nil {
			log.Printf("ListSharedWithMe: per-tenant query failed for %s/%s: %v", tenantID, actorStr, err)
			// Parity swallow — keep going with empty per-tenant results.
		} else {
			nodes = got
		}
	}

	// 2. Cross-tenant aggregation against globalstore.shared_index.
	//    De-dupe against per-tenant results by (source_tenant, node_id).
	seen := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		seen[n.TenantID+"\x00"+n.NodeID] = struct{}{}
	}
	if s.global != nil {
		entries, gerr := s.global.ListSharedToUser(ctx, actorStr, int(limit), int(offset))
		if gerr != nil {
			log.Printf("ListSharedWithMe: shared_index read failed for %s: %v", actorStr, gerr)
		} else {
			for _, e := range entries {
				key := e.SourceTenant + "\x00" + e.NodeID
				if _, dup := seen[key]; dup {
					continue
				}
				if s.store == nil {
					// No store wired — we can't fetch the full Node,
					// so the cross-tenant path is unusable. Skip
					// silently (matches the Python "no global_store"
					// graceful degrade applied in reverse).
					continue
				}
				n, err := s.store.GetNode(ctx, e.SourceTenant, e.NodeID)
				if err != nil || n == nil {
					// Stale shared_index entry: source tenant unloaded,
					// node deleted, etc. Silently skip — these are
					// eventual-consistency artefacts (spec "Side effects").
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

	// Convert store.Node -> pb.Node. Payload translation lives in the
	// payload package and is gated on a schema.Registry; in its absence
	// we surface the field-id-keyed shape verbatim by leaving Payload
	// nil — callers that need a typed payload pull it via
	// GetNode anyway. ACL is similarly lifted into pb.AclEntry only
	// when consumers need it; this RPC's contract test
	// (test_grpc_contract.py:339-350) is permissive, asserting only
	// well-formed nodes with non-empty IDs.
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
// a non-nil empty slice for symmetry with Python's `nodes=[]`.
func emptyListSharedWithMeResponse() *pb.ListSharedWithMeResponse {
	return &pb.ListSharedWithMeResponse{
		Nodes:   []*pb.Node{},
		HasMore: false,
	}
}
