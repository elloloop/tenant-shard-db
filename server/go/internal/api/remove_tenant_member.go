// SPDX-License-Identifier: AGPL-3.0-only

// RemoveTenantMember drops a (tenant_id, user_id) row from the
// cross-tenant tenant_members table. EPIC #407 Wave 2.
//
// Spec: docs/go-port/rpcs/RemoveTenantMember.md.
// Source-of-truth Python: server/python/entdb_server/api/grpc_server.py:2492-2541.
//
// # Behavioural pins
//
//   - Auth (Go HARDENS vs Python): the Python handler skips the
//     admin/owner role check entirely (a latent privilege escalation —
//     any authenticated caller can remove any member). The Go port
//     closes that gap mirroring AddTenantMember/ChangeMemberRole:
//     non-admin callers must be the tenant's "owner" or "admin" role,
//     otherwise PERMISSION_DENIED.
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
//     tenant" (NOT a gRPC error code — this is a domain-level "no-op"
//     per Python's contract). "admin" is NOT protected — only "owner".
//     Do NOT add an admin guard without an ADR (spec §"Last-admin
//     nuance").
//
//   - Idempotent removal of a non-member: success=false,
//     error="Member not found", gRPC code OK. Metric label = "ok"
//     (Python's distribution: dashboards depend on it).

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

	// Registry-less deployment guard. Mirrors grpc_server.py:2500-2504.
	if s.global == nil {
		status = "error"
		return nil, errs.Errorf(codes.Unimplemented, "Tenant registry not configured")
	}

	// Required-field validation. Mirrors grpc_server.py:2506-2511. Note:
	// `role` is silently ignored — the request type is shared with
	// AddTenantMember (proto/entdb/v1/entdb.proto:909-919).
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
	// unchanged — same behaviour as Python's
	// auth_interceptor.get_authoritative_actor.
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))

	// Admin/owner gate — Go HARDENS vs Python (closes the privilege-
	// escalation gap at grpc_server.py:2492-2541). Mirrors the pattern
	// in AddTenantMember (grpc_server.py:2466-2474) and
	// ChangeMemberRole (grpc_server.py:2627-2634).
	if !isAdminOrSystemActor(trusted) {
		role, err := s.getTenantMemberRole(ctx, req.GetTenantId(), trusted.ID())
		if err != nil {
			status = "error"
			return nil, errs.Errorf(codes.Internal, "RemoveTenantMember: lookup caller role: %v", err)
		}
		if role != "owner" && role != "admin" {
			status = "error"
			return nil, errs.Errorf(codes.PermissionDenied,
				"Only owner or admin can remove members")
		}
	}

	// Single-pass scan: count owners and locate the target row. Matches
	// grpc_server.py:2515-2521 — one read, no transaction wrapping.
	members, err := s.global.GetTenantMembers(ctx, req.GetTenantId())
	if err != nil {
		status = "error"
		return nil, errs.Errorf(codes.Internal, "RemoveTenantMember: list members: %v", err)
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

	// Not-a-member: idempotent no-op. NOT a gRPC error code; Python
	// records metric label "ok" (grpc_server.py:2523-2525).
	if target == nil {
		return &pb.TenantMemberResponse{
			Success: false,
			Error:   "Member not found",
		}, nil
	}

	// Last-owner protection. Only "owner" is protected — "admin" is
	// not (spec §"Last-admin nuance"). Python records metric label
	// "error" here (grpc_server.py:2527-2532).
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

// isAdminOrSystemActor mirrors grpc_server.py:2053-2069. The trusted
// actor is admin/system iff it carries the "system:" or "admin:"
// prefix.
func isAdminOrSystemActor(a auth.Actor) bool {
	return a.IsSystem() || a.IsAdmin()
}

// getTenantMemberRole returns the role of userID inside tenantID, or
// "" if the user is not a member. Mirrors grpc_server.py:2290-2296
// (`_get_member_role`).
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
