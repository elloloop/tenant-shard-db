// GetEdgesFrom RPC.
//
// Spec: docs/go-port/rpcs/GetEdgesFrom.md. Pure read of the per-tenant
// `edges` table indexed by (tenant_id, from_node_id), translated into
// the wire `Edge` shape.
//
// Semantics:
//
//   - Tenant gate (s.checkTenant) is the only ingress check. Its
//     UNAVAILABLE / FAILED_PRECONDITION / NotFound / InvalidArgument
//     errors propagate to the SDK so the redirect trailer
//     (`entdb-redirect-node`) lands before the status closes.
//   - request.context.actor is UNTRUSTED; resolve trusted identity via
//     auth.Authoritative (CLAUDE.md / commit fece3fb). The trusted
//     actor is NOT used for any authorization decision today — see the
//     ACL parity gap below.
//   - PARITY GAP (deliberate, pinned): no per-destination ACL filter.
//     The handler does NOT call the visibility check before fan-out, so
//     callers can enumerate `to_node_id`s they would not have READ on
//     through GetNode. Tracked as a follow-up: tightening this requires
//     a contract change because existing SDK clients depend on the full
//     edge fan-out shape.
//   - `offset` is declared on GetEdgesRequest (proto field 5) but the
//     handler ignores it; switching to honour it must update the
//     contract in lock-step.
//   - `edge_type_id == 0` means "no filter".
//   - `limit <= 0` defaults to 100. We pass 0 (no limit) to the store
//     so the slice + has_more accounting happens in this handler. NOT
//     in SQL: ordering is not contractually pinned.
//   - Side effects: none. No WAL append, no SQLite write, no metric
//     other than the standard request counter.
//   - Error contract (#573): the tenant is lazy-opened first, so a
//     genuine post-open store fault (IO error, corruption, scan failure)
//     is surfaced as a sanitized codes.Internal rather than masked as an
//     empty GetEdgesResponse with codes.OK — empty+OK is reserved for a
//     genuinely empty edge set. The metric outcome is "error" on faults.

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

const grpcMethodGetEdgesFrom = "GetEdgesFrom"

// defaultEdgesLimit: `limit or 100` substitutes 100 for any falsy value
// (0 / unset / negative).
const defaultEdgesLimit = 100

// GetEdgesFrom implements entdb.v1.EntDBService/GetEdgesFrom. See file
// header for the contract; the body is intentionally a thin translation
// over CanonicalStore.GetEdgesFrom.
func (s *Server) GetEdgesFrom(ctx context.Context, req *pb.GetEdgesRequest) (*pb.GetEdgesResponse, error) {
	start := time.Now()
	outcome := "ok"
	defer func() {
		metrics.RecordGRPCRequest(ctx, grpcMethodGetEdgesFrom, outcome, time.Since(start))
	}()

	tenantID := req.GetContext().GetTenantId()

	// Tenant gate — sharding ownership + region pinning. The redirect
	// trailer is set inside CheckTenant; the Go SDK redirect cache reads
	// it (sdk/go/entdb/redirect_cache.go).
	if err := s.checkTenant(ctx, tenantID); err != nil {
		outcome = "error"
		return nil, err
	}

	// Resolve trusted actor. The result is intentionally unused: this
	// RPC has no authorization decision today (see file-header parity
	// gap). The call exists so that the future tightening to ACL-filter
	// destinations has a single chokepoint to bind against and so we
	// honour the trusted-actor invariant explicitly.
	_ = auth.Authoritative(ctx, auth.ParseActor(req.GetContext().GetActor()))

	// Open the per-tenant view before reading. The SQLite store is a
	// materialized view of the WAL (ADR-016); "tenant not opened" means
	// the applier has not materialized this tenant in-process yet, not a
	// client error. Lazy-open it (as GetNode does) so a valid tenant is
	// not silently reported as an empty edge set. A genuine open failure
	// (region pin / crypto-shred -> FailedPrecondition; IO -> Internal)
	// surfaces its real typed code; only post-open query faults fall
	// through to the best-effort swallow below.
	if s.store != nil {
		if err := s.store.OpenTenant(ctx, tenantID); err != nil {
			outcome = "error"
			return nil, errs.Errorf(errs.Code(err), "GetEdgesFrom: open tenant: %v", err)
		}
	}

	// edge_type_id filter: falsy (== 0) means "no filter".
	var edgeTypeFilter *int32
	if etid := req.GetEdgeTypeId(); etid != 0 {
		v := etid
		edgeTypeFilter = &v
	}

	// Page size: prefer page_size, fall back to the legacy limit, then the
	// default. Clamp to MaxPageSize (SEC-4 #135).
	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = int(req.GetLimit())
	}
	if pageSize <= 0 {
		pageSize = defaultEdgesLimit
	}
	pageSize = clampPageSize(pageSize)

	// Keyset cursor (ADR-029): page_token is bound to (node_id, outgoing,
	// edge_type_id) by a fingerprint; mixing it with the deprecated offset
	// is INVALID_ARGUMENT.
	fingerprint := edgesFingerprint(req.GetNodeId(), true, req.GetEdgeTypeId())
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

	edges, err := s.store.GetEdgesFromPaged(ctx, tenantID, req.GetNodeId(), edgeTypeFilter, pageSize, cursor)
	if err != nil {
		// Surface genuine post-open faults (#573): the tenant is already
		// lazy-opened above, so this is a real IO/corruption error — not
		// "no edges". Masking it as edges=[]+OK is silent data loss.
		// Preserve typed sentinels; sanitize naked store errors.
		outcome = "error"
		if c := errs.Code(err); c != codes.Unknown {
			return nil, errs.Errorf(c, "GetEdgesFrom: %v", err)
		}
		return nil, errs.Internal(ctx, "GetEdgesFrom: store", err)
	}

	// A full page means more may exist — mint a cursor from the last edge
	// so a response that omits edges always carries a token (ADR-029).
	var nextPageToken string
	if len(edges) == pageSize && pageSize > 0 {
		ec := store.EdgeCursorFrom(edges[len(edges)-1], true)
		nextPageToken = encodeEdgePageToken(edgePageCursor{
			Fingerprint: fingerprint, CreatedAt: ec.CreatedAt,
			EdgeTypeID: ec.EdgeTypeID, PeerNodeID: ec.PeerNodeID,
		})
	}

	out := make([]*pb.Edge, 0, len(edges))
	for _, e := range edges {
		out = append(out, edgeToProto(e))
	}
	return &pb.GetEdgesResponse{Edges: out, HasMore: nextPageToken != "", NextPageToken: nextPageToken}, nil
}

// edgeToProto is shared with GetEdgesTo and lives in helpers.go (after
// the round-3 dedupe). The defensive empty-Struct shape is
// preserved there; behaviour is otherwise identical.
