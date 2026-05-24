// GetEdgesTo RPC.
// Spec: docs/go-port/rpcs/GetEdgesTo.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:67 (rpc), :476-491 (request /
// response), :493-507 (Edge).
//
// Symmetric inverse of GetEdgesFrom: returns edges whose `to_node_id`
// matches the requested node, scoped to a single tenant. Cross-tenant
// fan-in is structurally impossible because the edges PRIMARY KEY
// includes tenant_id, so even a forged node_id that happens to exist
// in another tenant cannot leak through the
// `WHERE tenant_id = ? AND to_node_id = ?` filter.
//
// Semantics:
//
//   - Read-only. No WAL append, no SQLite mutation, no global_store write.
//   - Tenant gate runs first via s.checkTenant; UNAVAILABLE /
//     FAILED_PRECONDITION / INVALID_ARGUMENT propagate untouched.
//   - Trusted-actor invariant (CLAUDE.md, commit fece3fb): we resolve
//     identity via auth.Authoritative and explicitly drop the wire
//     `actor` claim. There is no per-edge ACL filter today; the call
//     pins the trust boundary so a future tightening has a single
//     chokepoint.
//   - limit==0 ⇒ default 100; has_more is computed AFTER fetching
//     limit+1 rows (we ask SQLite for limit+1 to bound memory at the
//     storage layer while still emitting the same wire shape).
//   - offset is currently UNUSED (flagged in the spec "Open questions").
//   - Genuine post-open storage faults are surfaced as a sanitized
//     codes.Internal (#573); empty+OK is reserved for a genuinely empty
//     incoming-edge set. Metric label is "error" on faults.

package api

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

const grpcMethodGetEdgesTo = "GetEdgesTo"

// GetEdgesTo implements entdb.v1.EntDBService/GetEdgesTo. See file
// header for the full contract; the body is a thin translation layer
// over store.GetEdgesTo.
func (s *Server) GetEdgesTo(
	ctx context.Context,
	req *pb.GetEdgesRequest,
) (*pb.GetEdgesResponse, error) {
	start := time.Now()
	outcome := "ok"
	defer func() {
		metrics.RecordGRPCRequest(grpcMethodGetEdgesTo, outcome, time.Since(start))
	}()

	tenantID := req.GetContext().GetTenantId()

	// Ingress checks — may abort with INVALID_ARGUMENT (empty tenant) /
	// UNAVAILABLE (sharding) / FAILED_PRECONDITION (region) / NOT_FOUND
	// (unknown tenant). Same gate every tenant-scoped RPC uses.
	if err := s.checkTenant(ctx, tenantID); err != nil {
		outcome = "error"
		return nil, err
	}

	// Trusted actor: pin the trust boundary even though no per-edge ACL
	// filter exists today. The wire `actor` is treated as a hint only;
	// auth.Authoritative replaces it with the interceptor-attached
	// identity when present (commit fece3fb).
	_ = auth.Authoritative(ctx, auth.ParseActor(req.GetContext().GetActor()))

	// Storage path requires the per-tenant CanonicalStore. If no store
	// is wired, fall through to the swallow-and-empty path.
	if s.store == nil {
		outcome = "error"
		return &pb.GetEdgesResponse{}, nil
	}

	// Open the per-tenant view before reading. The SQLite store is a
	// materialized view of the WAL (ADR-016); "tenant not opened" means
	// the applier has not materialized this tenant in-process yet, not a
	// client error. Lazy-open it (as GetNode does) so a valid tenant is
	// not silently reported as an empty edge set. A genuine open failure
	// (region pin / crypto-shred -> FailedPrecondition; IO -> Internal)
	// surfaces its real typed code; only post-open query faults fall
	// through to the best-effort swallow below.
	if err := s.store.OpenTenant(ctx, tenantID); err != nil {
		outcome = "error"
		return nil, errs.Errorf(errs.Code(err), "GetEdgesTo: open tenant: %v", err)
	}

	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = int(req.GetLimit())
	}
	if pageSize <= 0 {
		pageSize = 100
	}
	// SEC-4 (#135): cap oversized page requests.
	pageSize = clampPageSize(pageSize)

	var edgeType *int32
	if t := req.GetEdgeTypeId(); t != 0 {
		edgeType = &t
	}

	// Keyset cursor (ADR-029): page_token bound to (node_id, incoming,
	// edge_type_id); mutually exclusive with the deprecated offset.
	fingerprint := edgesFingerprint(req.GetNodeId(), false, req.GetEdgeTypeId())
	var cursor *store.EdgeCursor
	if tok := req.GetPageToken(); tok != "" {
		if req.GetOffset() != 0 {
			outcome = "error"
			return nil, errs.Errorf(codes.InvalidArgument,
				"page_token and the deprecated offset are mutually exclusive; use one")
		}
		c, derr := decodeEdgePageToken(tok, fingerprint)
		if derr != nil {
			outcome = "error"
			return nil, derr
		}
		cursor = &store.EdgeCursor{CreatedAt: c.CreatedAt, EdgeTypeID: c.EdgeTypeID, PeerNodeID: c.PeerNodeID}
	}

	rows, err := s.store.GetEdgesToPaged(ctx, tenantID, req.GetNodeId(), edgeType, pageSize, cursor)
	if err != nil {
		// Surface genuine post-open faults (#573): the tenant is already
		// lazy-opened above, so this is a real IO/corruption error — not
		// "no edges". Preserve typed sentinels; sanitize naked store errors.
		outcome = "error"
		if c := errs.Code(err); c != codes.Unknown {
			return nil, errs.Errorf(c, "GetEdgesTo: %v", err)
		}
		return nil, errs.Internal(ctx, "GetEdgesTo: store", err)
	}

	var nextPageToken string
	if len(rows) == pageSize && pageSize > 0 {
		ec := store.EdgeCursorFrom(rows[len(rows)-1], false)
		nextPageToken = encodeEdgePageToken(edgePageCursor{
			Fingerprint: fingerprint, CreatedAt: ec.CreatedAt,
			EdgeTypeID: ec.EdgeTypeID, PeerNodeID: ec.PeerNodeID,
		})
	}

	out := make([]*pb.Edge, 0, len(rows))
	for _, e := range rows {
		out = append(out, edgeToProto(e))
	}
	return &pb.GetEdgesResponse{Edges: out, HasMore: nextPageToken != "", NextPageToken: nextPageToken}, nil
}

// edgeToProto and edgePropsToStruct live in helpers.go (consolidated
// in the round-3 dedupe). The defensive empty-Struct semantics
// — never nil for `props` even on malformed legacy rows — are
// preserved there.
