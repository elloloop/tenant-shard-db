// AddTenantMember implements entdb.v1.EntDBService/AddTenantMember.
//
// Source-of-truth Python: server/python/entdb_server/api/grpc_server.py:2442-2490.
// Port spec: docs/go-port/rpcs/AddTenantMember.md.
//
// # WAL bypass — explicit carve-out from CLAUDE.md §1
//
// Tenant-membership writes go DIRECTLY to the cross-tenant globalstore
// (the `tenant_members` table). They MUST NOT be appended to the
// per-tenant Kafka/Kinesis WAL. This is a deliberate exception to the
// "all writes go through the WAL" invariant: globalstore is a
// non-event-sourced control-plane store (see
// server/go/internal/globalstore/doc — package comment "Carve-out from
// invariant #1"). Adding a WAL event for membership would change
// rebuild semantics and break the cross-language contract suite. If
// you are tempted to "fix" this by routing through the Applier, stop
// and read the package doc on globalstore first.
//
// # Auth model — trusted-actor admin-only
//
// 1. Authentication is required (handler is NOT in
//    AuthInterceptor.UNAUTHENTICATED_METHODS).
// 2. The wire-claimed `actor` field is UNTRUSTED — the handler rebinds
//    to the interceptor-attested identity via auth.Authoritative on
//    entry. Every authorization branch consults the trusted actor,
//    never req.GetActor(). Privilege-escalation regression pinned by
//    commit fece3fb ("Fix privilege escalation: ignore client-claimed
//    actor in gRPC handlers").
// 3. Authorization succeeds iff EITHER:
//      a. trusted actor is system: / admin: prefixed, OR
//      b. trusted actor's role in tenant_members for tenant_id is
//         "owner" or "admin".
//
// # Side effects (intentionally minimal)
//
//   - Direct INSERT into globalstore.tenant_members. UNIQUE constraint
//     on (tenant_id, user_id).
//   - NO mailbox / notification fanout. The added member is silent —
//     discovery is via GetUserTenants. Matches Python parity.
//   - NO FK validation on tenant_id / user_id. Both can refer to rows
//     that were never created — preserved for parity (see the spec's
//     "Open questions / risks" section; harden in a separate ticket).
//
// # Error contract
//
//	UNIMPLEMENTED       globalstore not configured.
//	INVALID_ARGUMENT    actor / tenant_id / user_id empty.
//	PERMISSION_DENIED   trusted actor is neither admin/system nor an
//	                    owner/admin member of tenant_id.
//	OK + success=false  duplicate (tenant_id, user_id) row — soft
//	                    failure, NOT a gRPC error.
//	OK + success=false  any other AddTenantMember error.
//	OK + success=true   insert succeeded.
//
// Metrics: emits entdb_grpc_requests_total{method="AddTenantMember",
// status="ok"|"error"} via the shared chokepoint.

package api

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// AddTenantMember inserts a (tenant_id, user_id, role) row into the
// cross-tenant globalstore. See package-level doc for the WAL-bypass
// carve-out and auth model.
func (s *Server) AddTenantMember(
	ctx context.Context,
	req *pb.TenantMemberRequest,
) (*pb.TenantMemberResponse, error) {
	start := time.Now()
	status := "ok"
	defer func() {
		metrics.RecordGRPCRequest("AddTenantMember", status, time.Since(start))
	}()

	if s.global == nil {
		status = "error"
		return nil, errs.Errorf(codes.Unimplemented, "Tenant registry not configured")
	}

	// Mirror grpc_server.py:2456-2461: required-arg validation BEFORE
	// any identity work. Empty actor is INVALID_ARGUMENT even when the
	// interceptor has attested a stronger identity on ctx — Python pins
	// this in test_grpc_contract.py:519-523.
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

	// Trusted-actor rebind. From this point on, req.GetActor() is
	// strictly informational — every authorization branch consults
	// `trusted`. Mirrors grpc_server.py:2466 self._trusted_actor(...)
	// and the privilege-escalation regression fix in commit fece3fb.
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))

	// Admin / system bypass: same predicate as
	// grpc_server.py:2053-2069 _is_admin_or_system. group: actors are
	// not valid callers and never reach this branch.
	if !(trusted.IsAdmin() || trusted.IsSystem()) {
		// Membership-based admin: caller must be owner or admin of
		// the target tenant. Python uses _get_member_role
		// (grpc_server.py:2290-2296) which is an O(N) scan over
		// tenant_members; we mirror that with GetTenantMembers since
		// the dataset is tiny per tenant. Filing a hardening ticket
		// to expose a typed MemberRole helper in globalstore is
		// tracked in the spec's "Implementation outline" notes.
		role, err := s.lookupMemberRole(ctx, req.GetTenantId(), trusted.ID())
		if err != nil {
			status = "error"
			return nil, errs.Errorf(codes.Internal, "lookup member role: %v", err)
		}
		if role != "owner" && role != "admin" {
			status = "error"
			return nil, errs.Errorf(codes.PermissionDenied,
				"Only owner or admin can add members")
		}
	}

	role := req.GetRole()
	if role == "" {
		role = "member"
	}

	if err := s.global.AddTenantMember(ctx, req.GetTenantId(), req.GetUserId(), role); err != nil {
		// Soft-failure path: duplicate (tenant_id, user_id). Returns
		// gRPC OK with success=false so SDK retries on transient
		// network errors observe an idempotent-replay signal.
		// Pinned by grpc_server.py:2484-2487.
		if errors.Is(err, errs.ErrAlreadyExists) {
			status = "error"
			return &pb.TenantMemberResponse{
				Success: false,
				Error:   "Member already exists in this tenant",
			}, nil
		}
		// Any other globalstore error: same swallow shape — gRPC OK
		// with success=false. Mirrors grpc_server.py:2488-2490.
		status = "error"
		return &pb.TenantMemberResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.TenantMemberResponse{Success: true}, nil
}
