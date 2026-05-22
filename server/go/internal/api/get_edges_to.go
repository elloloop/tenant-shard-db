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
//   - Internal storage errors are SWALLOWED into an empty
//     GetEdgesResponse with codes.OK. Metric label is "error" so the
//     swallow does not inflate the ok counter.

package api

import (
	"context"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
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

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 100
	}
	// SEC-4 (#135): cap oversized page requests before the store
	// fetches limit+1 rows.
	limit = clampPageSize(limit)

	var edgeType *int32
	if t := req.GetEdgeTypeId(); t != 0 {
		edgeType = &t
	}

	// Fetch limit+1 so a single trailing row distinguishes
	// "exactly limit" from "more available", without materialising the
	// full result set in memory for high-fan-in targets.
	rows, err := s.store.GetEdgesTo(ctx, tenantID, req.GetNodeId(), edgeType, limit+1)
	if err != nil {
		// Swallow ALL internal errors into edges=[] with grpc.OK.
		// Operators can still tell the "no edges" case apart from
		// "store fault" via the metric label, which is "error" here.
		outcome = "error"
		return &pb.GetEdgesResponse{}, nil
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	out := make([]*pb.Edge, 0, len(rows))
	for _, e := range rows {
		out = append(out, edgeToProto(e))
	}
	return &pb.GetEdgesResponse{Edges: out, HasMore: hasMore}, nil
}

// edgeToProto and edgePropsToStruct live in helpers.go (consolidated
// in the round-3 dedupe). The defensive empty-Struct semantics
// — never nil for `props` even on malformed legacy rows — are
// preserved there.
