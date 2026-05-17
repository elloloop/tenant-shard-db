// SPDX-License-Identifier: AGPL-3.0-only

// ChangeMemberRole implements entdb.v1.EntDBService/ChangeMemberRole.
//
// Port spec: docs/go-port/rpcs/ChangeMemberRole.md.
//
// Behaviour parity vs. the Python handler is preserved on the wire shape
// (request/response, error code asymmetry between auth-failure and
// missing-row), but two PLAN.md §6 drifts are folded in here:
//
//  1. WAL-first global mutation. The handler appends a global
//     `member_role_changed` op and waits for the applier to update the
//     tenant_members row; it does not write globalstore directly.
//
//  2. Last-owner demotion protection. The Python handler (spec §"Open
//     questions" item 2) lets the sole owner demote themselves and
//     brick the tenant. The Go port adds an explicit check: if the
//     target user is the only "owner" row in tenant_members and
//     new_role != "owner", reject with FAILED_PRECONDITION. This is
//     the §6 drift the spec asked us to add on the Go side.
//
//  3. Region pin gap (spec §"Open questions" item 5). Not fixed here —
//     parity preserved; the Go handler does NOT call s.checkTenant.
//     Filed as a follow-up.

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

const grpcMethodChangeMemberRole = "ChangeMemberRole"

// ChangeMemberRole updates a tenant member's role.
//
// Authorization (admin-only via trusted-actor):
//
//   - Trusted actor (auth.Authoritative) is system: or admin: → allowed.
//   - Trusted actor is a user: with tenant role "owner" or "admin"
//     in the tenant_members row for tenantID → allowed.
//   - Otherwise → codes.PermissionDenied.
//
// Last-owner protection: if the target user is the sole "owner" of the
// tenant and the new role is not "owner", the call is rejected with
// codes.FailedPrecondition. This is a Go-side improvement over the
// Python handler's silent self-brick.
func (s *Server) ChangeMemberRole(
	ctx context.Context,
	req *pb.ChangeMemberRoleRequest,
) (*pb.ChangeMemberRoleResponse, error) {
	start := time.Now()

	if s.global == nil {
		metrics.RecordGRPCRequest(grpcMethodChangeMemberRole, "error", time.Since(start))
		return nil, errs.Errorf(codes.Unimplemented, "Tenant registry not configured")
	}
	if req.GetActor() == "" {
		metrics.RecordGRPCRequest(grpcMethodChangeMemberRole, "error", time.Since(start))
		return nil, errs.Errorf(codes.InvalidArgument, "actor is required")
	}
	if req.GetTenantId() == "" {
		metrics.RecordGRPCRequest(grpcMethodChangeMemberRole, "error", time.Since(start))
		return nil, errs.Errorf(codes.InvalidArgument, "tenant_id is required")
	}
	if req.GetUserId() == "" {
		metrics.RecordGRPCRequest(grpcMethodChangeMemberRole, "error", time.Since(start))
		return nil, errs.Errorf(codes.InvalidArgument, "user_id is required")
	}
	if req.GetNewRole() == "" {
		metrics.RecordGRPCRequest(grpcMethodChangeMemberRole, "error", time.Since(start))
		return nil, errs.Errorf(codes.InvalidArgument, "new_role is required")
	}

	// Trusted-actor rebind. Privilege checks below MUST consult `trusted`
	// (never `req.Actor`); honouring the request payload is the
	// privilege-escalation hole fixed by commit fece3fb.
	claimed := auth.ParseActor(req.GetActor())
	trusted := auth.Authoritative(ctx, claimed)

	if !(trusted.IsSystem() || trusted.IsAdmin()) {
		// Caller must be a tenant-level admin/owner to mutate roles.
		callerRole, err := s.lookupMemberRole(ctx, req.GetTenantId(), trusted.ID())
		if err != nil {
			metrics.RecordGRPCRequest(grpcMethodChangeMemberRole, "error", time.Since(start))
			return nil, errs.Internal(ctx, "list tenant members", err)
		}
		if callerRole != "owner" && callerRole != "admin" {
			metrics.RecordGRPCRequest(grpcMethodChangeMemberRole, "error", time.Since(start))
			return nil, errs.Errorf(codes.PermissionDenied,
				"Only tenant admins can change member roles")
		}
	}

	members, err := s.global.GetTenantMembers(ctx, req.GetTenantId())
	if err != nil {
		metrics.RecordGRPCRequest(grpcMethodChangeMemberRole, "error", time.Since(start))
		return nil, errs.Internal(ctx, "list tenant members", err)
	}
	var targetJoinedAt int64
	targetFound := false

	// Last-owner demotion guard. We compute this before appending so
	// the caller sees a deterministic FAILED_PRECONDITION rather than a
	// post-hoc "tenant has no owners" surprise.
	if req.GetNewRole() != "owner" {
		ownerCount := 0
		targetIsOwner := false
		for _, m := range members {
			if m.UserID == req.GetUserId() {
				targetJoinedAt = m.JoinedAt
				targetFound = true
			}
			if m.Role == "owner" {
				ownerCount++
				if m.UserID == req.GetUserId() {
					targetIsOwner = true
				}
			}
		}
		if targetIsOwner && ownerCount == 1 {
			metrics.RecordGRPCRequest(grpcMethodChangeMemberRole, "error", time.Since(start))
			return nil, errs.Errorf(codes.FailedPrecondition,
				"cannot demote the last owner of tenant %q", req.GetTenantId())
		}
	} else {
		for _, m := range members {
			if m.UserID == req.GetUserId() {
				targetJoinedAt = m.JoinedAt
				targetFound = true
				break
			}
		}
	}

	if !targetFound {
		// Soft failure (matches Python: gRPC OK, response.success=false).
		metrics.RecordGRPCRequest(grpcMethodChangeMemberRole, "ok", time.Since(start))
		return &pb.ChangeMemberRoleResponse{Success: false, Error: "Member not found"}, nil
	}
	if targetJoinedAt == 0 {
		targetJoinedAt = time.Now().Unix()
	}

	_, _, err = s.appendGlobalAdminOp(ctx, trusted.String(), map[string]any{
		"op":        string(apply.OpMemberRoleChanged),
		"tenant_id": req.GetTenantId(),
		"user_id":   req.GetUserId(),
		"role":      req.GetNewRole(),
		"joined_at": targetJoinedAt,
	})
	if err != nil {
		metrics.RecordGRPCRequest(grpcMethodChangeMemberRole, "error", time.Since(start))
		return nil, err
	}

	metrics.RecordGRPCRequest(grpcMethodChangeMemberRole, "ok", time.Since(start))
	return &pb.ChangeMemberRoleResponse{Success: true}, nil
}
