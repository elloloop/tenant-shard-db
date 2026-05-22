// SPDX-License-Identifier: AGPL-3.0-only

// RemoveTenantMember drops a (tenant_id, user_id) row from the
// cross-tenant tenant_members table.
//
// Spec: docs/go-port/rpcs/RemoveTenantMember.md.
//
// # Behavioural pins
//
//   - Auth: the handler gates on admin/owner role, closing the latent
//     privilege escalation gap (any authenticated caller removing any
//     member). Non-admin callers must be the tenant's "owner" or
//     "admin" role, otherwise PERMISSION_DENIED. Mirrors
//     AddTenantMember/ChangeMemberRole.
//
//     The actor on the wire is UNTRUSTED. We rebind to the trusted
//     actor returned by auth.Authoritative before any privilege check,
//     so a forged `actor="system:admin"` from a regular user loses.
//
//   - WAL: this RPC appends a global `member_removed` op and waits for
//     the applier to delete the tenant_members row. The handler does
//     not write globalstore directly.
//
//   - Tenant gate: NOT called. The tenant registry is global, not
//     region-pinned. (spec §"Side effects" step 2 only checks
//     globalstore != nil; cross-region pinning explicitly out of
//     scope, spec "Open questions" §6.)
//
//   - ACL/mailbox cascade: NOT performed. Direct grants, group
//     memberships, shared_index rows, and owned nodes survive. The
//     caller is responsible for chaining TransferUserContent →
//     RevokeAllUserAccess → RemoveTenantMember for a complete
//     off-board (sdk/go/entdb/admin.go:68-71).
//
//   - Last-OWNER protection: removing the only "owner" returns
//     success=false with error="Cannot remove the last owner of a
//     tenant" (NOT a gRPC error code — domain-level "no-op").
//     "admin" is NOT protected — only "owner".
//     Do NOT add an admin guard without an ADR (spec §"Last-admin
//     nuance").
//
//   - Idempotent removal of a non-member: success=false,
//     error="Member not found", gRPC code OK. Metric label = "ok"
//     (dashboards depend on it).

package api

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

const removeTenantMemberMethod = "RemoveTenantMember"

// RemoveTenantMember implements entdb.v1.EntDBService/RemoveTenantMember.
func (s *Server) RemoveTenantMember(
	ctx context.Context, req *pb.TenantMemberRequest,
) (*pb.TenantMemberResponse, error) {
	start := time.Now()
	status := "ok"
	defer func() {
		metrics.RecordGRPCRequest(removeTenantMemberMethod, status, time.Since(start))
	}()

	// Registry-less deployment guard.
	if s.global == nil {
		status = "error"
		return nil, errs.Errorf(codes.Unimplemented, "Tenant registry not configured")
	}

	// Required-field validation. Note: `role` is silently ignored — the
	// request type is shared with AddTenantMember
	// (proto/entdb/v1/entdb.proto:909-919).
	if req.GetActor() == "" {
		status = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "actor is required")
	}
	if req.GetTenantId() == "" {
		status = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "tenant_id is required")
	}
	if req.GetUserId() == "" {
		status = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "user_id is required")
	}

	// Trusted-actor rebinding. The wire `actor` is UNTRUSTED. The
	// auth interceptor (when enabled) installs a verified Identity on
	// ctx; auth.Authoritative substitutes it for the wire claim. In
	// no-auth dev/test mode (no interceptor) the claim flows through
	// unchanged.
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))

	// Admin/owner gate. Mirrors the pattern in AddTenantMember and
	// ChangeMemberRole.
	if !isAdminOrSystemActor(trusted) {
		role, err := s.getTenantMemberRole(ctx, req.GetTenantId(), trusted.ID())
		if err != nil {
			status = "error"
			return nil, errs.Internal(ctx, "RemoveTenantMember: lookup caller role", err)
		}
		if role != "owner" && role != "admin" {
			status = "error"
			return nil, errs.Errorf(codes.PermissionDenied,
				"Only owner or admin can remove members")
		}
	}

	// Single-pass scan: count owners and locate the target row.
	// One read, no transaction wrapping.
	members, err := s.global.GetTenantMembers(ctx, req.GetTenantId())
	if err != nil {
		status = "error"
		return nil, errs.Internal(ctx, "RemoveTenantMember: list members", err)
	}
	var (
		target     *string // role of the target user, nil if not a member
		ownerCount int
	)
	for _, m := range members {
		if m.Role == "owner" {
			ownerCount++
		}
		if m.UserID == req.GetUserId() {
			role := m.Role
			target = &role
		}
	}

	// Not-a-member: idempotent no-op. NOT a gRPC error code; metric
	// label is "ok".
	if target == nil {
		return &pb.TenantMemberResponse{
			Success: false,
			Error:   "Member not found",
		}, nil
	}

	// Last-owner protection. Only "owner" is protected — "admin" is
	// not (spec §"Last-admin nuance"). Metric label is "error" here.
	if *target == "owner" && ownerCount <= 1 {
		status = "error"
		return &pb.TenantMemberResponse{
			Success: false,
			Error:   "Cannot remove the last owner of a tenant",
		}, nil
	}

	_, _, err = s.appendGlobalAdminOp(ctx, trusted.String(), map[string]any{
		"op":        string(apply.OpMemberRemoved),
		"tenant_id": req.GetTenantId(),
		"user_id":   req.GetUserId(),
	})
	if err != nil {
		status = "error"
		return nil, err
	}
	return &pb.TenantMemberResponse{Success: true}, nil
}

// isAdminOrSystemActor returns true when the trusted actor carries the
// "system:" or "admin:" prefix.
func isAdminOrSystemActor(a auth.Actor) bool {
	return a.IsSystem() || a.IsAdmin()
}

// getTenantMemberRole returns the role of userID inside tenantID, or
// "" if the user is not a member.
func (s *Server) getTenantMemberRole(ctx context.Context, tenantID, userID string) (string, error) {
	members, err := s.global.GetTenantMembers(ctx, tenantID)
	if err != nil {
		return "", err
	}
	for _, m := range members {
		if m.UserID == userID {
			return m.Role, nil
		}
	}
	return "", nil
}
