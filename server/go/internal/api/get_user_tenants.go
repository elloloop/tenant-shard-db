// GetUserTenants RPC.
//
// Spec: docs/go-port/rpcs/GetUserTenants.md.
//
// Contract pins (must not regress):
//
//   - actor / user_id presence-checked. Empty actor or user_id ->
//     INVALID_ARGUMENT. Validation order: actor first, then user_id.
//   - global_store == nil -> UNIMPLEMENTED with the exact message
//     "Tenant registry not configured".
//   - The trusted-actor invariant is honoured: the wire-claimed actor
//     is run through auth.Authoritative so the interceptor's identity
//     wins over any payload-claimed identity. The resolved actor is NOT
//     used for an authz decision (any authenticated caller may read any
//     user's memberships). The privilege-boundary gap is documented in
//     the spec and tracked as a follow-up; do NOT plug it here without
//     coordinated proto/contract-test changes (see spec "Auth" section).
//   - Memberships are returned in joined_at ASC order, sourced from
//     globalstore.GetUserTenants. Empty list when the user has no
//     memberships.
//   - Catch-all: any error from globalstore (or a panic inside the
//     handler body after validation) collapses to OK with
//     memberships=[]. We MUST NOT propagate codes.Internal — the
//     contract test relies on the empty-OK degradation.
//
// Metrics: emits entdb_grpc_requests_total{method="GetUserTenants",
// status="ok"|"error"} via the shared metrics chokepoint. The
// validation and UNIMPLEMENTED short-circuits do NOT emit metrics —
// only the post-validation success-or-degraded path does.

package api

import (
	"context"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grpcMethodGetUserTenants is the metric label.
const grpcMethodGetUserTenants = "GetUserTenants"

// GetUserTenants implements entdb.v1.EntDBService/GetUserTenants.
func (s *Server) GetUserTenants(
	ctx context.Context,
	req *pb.GetUserTenantsRequest,
) (resp *pb.GetUserTenantsResponse, err error) {
	// Pre-validation guards run BEFORE the metrics defer is armed,
	// short-circuiting before the record_grpc_request call site.
	if s.global == nil {
		return nil, status.Error(
			codes.Unimplemented, "Tenant registry not configured")
	}
	if req.GetActor() == "" {
		return nil, status.Error(codes.InvalidArgument, "actor is required")
	}
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	start := time.Now()
	outcome := "ok"
	// Empty-OK degradation on panic. The handler MUST NOT surface
	// codes.Internal; contract tests pin the empty-OK shape.
	defer func() {
		if r := recover(); r != nil {
			outcome = "error"
			resp = &pb.GetUserTenantsResponse{
				Memberships: []*pb.TenantMemberInfo{},
			}
			err = nil
		}
		metrics.RecordGRPCRequest(ctx, grpcMethodGetUserTenants, outcome, time.Since(start))
	}()

	// Trusted-actor resolution. The result is intentionally not used
	// for an authz decision; the call exists so the interceptor's
	// identity always wins over any payload-claimed actor (defence in
	// depth — see the spec's "privilege-boundary gap" note and the
	// comment block above).
	_ = auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))

	rows, qerr := s.global.GetUserTenants(ctx, req.GetUserId())
	if qerr != nil {
		// Silently swallows and returns memberships=[] OK.
		outcome = "error"
		return &pb.GetUserTenantsResponse{
			Memberships: []*pb.TenantMemberInfo{},
		}, nil
	}

	memberships := make([]*pb.TenantMemberInfo, 0, len(rows))
	for _, m := range rows {
		memberships = append(memberships, &pb.TenantMemberInfo{
			TenantId: m.TenantID,
			UserId:   m.UserID,
			Role:     m.Role,
			JoinedAt: m.JoinedAt,
		})
	}
	return &pb.GetUserTenantsResponse{Memberships: memberships}, nil
}
