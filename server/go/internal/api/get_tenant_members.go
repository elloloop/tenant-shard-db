// GetTenantMembers RPC.
// Spec: docs/go-port/rpcs/GetTenantMembers.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:125 (rpc), :921-928
// (request/response), :902-907 (TenantMemberInfo).
//
// Semantics:
//
//   - Read-only. One SELECT against globalstore.tenant_members; no WAL
//     append, no per-tenant SQLite touch (CLAUDE.md invariant #4 —
//     globalstore is the legitimate cross-tenant path).
//   - No `_check_tenant` gate, no `_check_cross_tenant_read`, no
//     capability lookup; no membership gate beyond non-empty argument
//     validation. Flagged for follow-up in the spec's open-questions
//     section.
//   - Trusted-actor rebinding via auth.Authoritative is applied at the
//     top. This closes the privilege-escalation gap addressed by commit
//     fece3fb (issue #168) for free, and the rebinding has no observable
//     effect on the response because there is no member-only gate.
//   - INVALID_ARGUMENT messages and ordering: actor-check fires before
//     tenant-check, with the exact strings `"actor is required"` and
//     `"tenant_id is required"`. SDK callers that string-match these
//     errors stay green.
//   - Unknown tenant -> empty list + OK (the SELECT returns 0 rows).
//   - Internal globalstore failures are silently swallowed: the handler
//     returns an empty members slice with grpc.OK. This is hostile to
//     debugging but load-bearing for parity; flagged for tightening in
//     a separate issue.
//
// joined_at is epoch milliseconds (matching `int(time.time()*1000)`,
// Go SDK pin at sdk/go/entdb/admin_test.go:240). The
// globalstore.Member.JoinedAt field already carries ms; no conversion
// is required at the boundary.

package api

import (
	"context"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"google.golang.org/grpc/codes"
)

const getTenantMembersMethod = "GetTenantMembers"

// GetTenantMembers implements entdb.v1.EntDBService/GetTenantMembers.
// See file header for the parity contract.
func (s *Server) GetTenantMembers(
	ctx context.Context,
	req *pb.GetTenantMembersRequest,
) (*pb.GetTenantMembersResponse, error) {
	start := time.Now()

	if s.global == nil {
		metrics.RecordGRPCRequest(ctx, getTenantMembersMethod, "error", time.Since(start))
		return nil, errs.Errorf(codes.Unimplemented, "Tenant registry not configured")
	}

	// Argument validation. Order is load-bearing: actor is checked before
	// tenant_id, and the SDK contract tests rely on the exact
	// INVALID_ARGUMENT messages.
	if req.GetActor() == "" {
		metrics.RecordGRPCRequest(ctx, getTenantMembersMethod, "error", time.Since(start))
		return nil, errs.Errorf(codes.InvalidArgument, "actor is required")
	}
	if req.GetTenantId() == "" {
		metrics.RecordGRPCRequest(ctx, getTenantMembersMethod, "error", time.Since(start))
		return nil, errs.Errorf(codes.InvalidArgument, "tenant_id is required")
	}

	// Trusted-actor rebinding. No-op on the response today (no
	// membership gate), but consistent with the rest of the
	// surface and the privilege-escalation fix in #168. The rebound
	// actor is intentionally unused; we keep the call as documentation
	// and to match the canonical handler shape.
	_ = auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))

	rows, err := s.global.GetTenantMembers(ctx, req.GetTenantId())
	if err != nil {
		// Silent-swallow: the handler never surfaces an internal
		// globalstore failure; SDK callers cannot distinguish "no
		// members" from "DB blew up". Flagged in the EPIC's
		// open-questions for tightening.
		metrics.RecordGRPCRequest(ctx, getTenantMembersMethod, "error", time.Since(start))
		return &pb.GetTenantMembersResponse{Members: []*pb.TenantMemberInfo{}}, nil
	}

	out := make([]*pb.TenantMemberInfo, 0, len(rows))
	for _, m := range rows {
		out = append(out, &pb.TenantMemberInfo{
			TenantId: m.TenantID,
			UserId:   m.UserID,
			Role:     m.Role,
			JoinedAt: m.JoinedAt,
		})
	}
	metrics.RecordGRPCRequest(ctx, getTenantMembersMethod, "ok", time.Since(start))
	return &pb.GetTenantMembersResponse{Members: out}, nil
}
