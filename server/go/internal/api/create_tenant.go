// SPDX-License-Identifier: AGPL-3.0-only

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

// CreateTenant implements the entdb.v1.EntDBService/CreateTenant RPC.
// Spec: docs/go-port/rpcs/CreateTenant.md.
//
// Carve-out from CLAUDE.md invariant #1 (intentional, scoped to the
// tenant-registry control plane): the write goes directly into the
// globalstore SQLite without a WAL append. tenant_registry is a
// non-WAL-backed control plane in this port; the per-tenant SQLite file
// is created lazily on first data-plane use, NOT here.
//
// Two-step shape:
//
//  1. validate (admin gate, non-empty fields)
//  2. globalstore.CreateTenantWithOwner (registry row + owner member,
//     atomic in a single SQLite transaction)
//
// The registry + member writes are now atomic — the Python-parity orphan-
// tenant hazard (crash between the registry INSERT and the membership
// INSERT) was closed when the Python source was retired in Phase 4D.
//
// Auth contract:
//   - Wire actor (req.Actor) is UNTRUSTED. The trusted principal is
//     resolved via auth.Authoritative(ctx, claimed) — privilege-escalation
//     remediation per commit fece3fb.
//   - Admin-only: Wave-2 decision deviating from current Python (which
//     has no admin gate today). Any caller whose trusted actor is not
//     admin: or system: gets PERMISSION_DENIED. This closes the multi-
//     tenant abuse vector flagged in the spec ("Admin gate" open
//     question).
func (s *Server) CreateTenant(ctx context.Context, req *pb.CreateTenantRequest) (*pb.CreateTenantResponse, error) {
	start := time.Now()
	resultStatus := "ok"
	defer func() {
		metrics.RecordGRPCRequest("CreateTenant", resultStatus, time.Since(start))
	}()

	// Step 1a: globalstore must be wired. Mirrors Python's
	// `if not self.global_store: abort(UNIMPLEMENTED, ...)`
	// (grpc_server.py:2312-2316).
	if s.global == nil {
		resultStatus = "error"
		return nil, errs.Errorf(codes.Unimplemented, "Tenant registry not configured")
	}

	// Step 1b: required-field validation. Matches the three INVALID_ARGUMENT
	// aborts in Python (grpc_server.py:2318-2323). The wire `actor` MUST be
	// non-empty even though we ignore it for the auth decision — this keeps
	// the wire contract identical so existing SDK callers don't break.
	if req.GetActor() == "" {
		resultStatus = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "actor is required")
	}
	if req.GetTenantId() == "" {
		resultStatus = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "tenant_id is required")
	}
	if req.GetName() == "" {
		resultStatus = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "name is required")
	}

	// Step 1c: trusted-actor + admin gate. Wire `actor` is rebound; the
	// authoritative principal comes from the auth interceptor's per-RPC
	// context. Anything that isn't admin: or system: is rejected.
	claimed := auth.ParseActor(req.GetActor())
	principal := auth.Authoritative(ctx, claimed)
	if !(principal.IsAdmin() || principal.IsSystem()) {
		resultStatus = "error"
		return nil, errs.Errorf(codes.PermissionDenied,
			"CreateTenant requires admin or system actor")
	}

	// Step 1d: resolve region. Empty → server's served region → final
	// fallback "us-east-1" (globalstore.DefaultRegion). Mirrors Python's
	// `request.region or self.served_region or "us-east-1"`
	// (grpc_server.py:2334).
	region := req.GetRegion()
	if region == "" {
		region = s.region
	}

	// Step 2: atomic globalstore write. The registry row and the owner
	// membership row land (or don't) in a single SQLite transaction.
	// UNIQUE on tenant_registry.tenant_id surfaces as ALREADY_EXISTS;
	// the handler maps that to a typed gRPC error.
	tenant, err := s.global.CreateTenantWithOwner(ctx, req.GetTenantId(), req.GetName(), region, principal.ID())
	if err != nil {
		resultStatus = "error"
		if errors.Is(err, errs.ErrAlreadyExists) {
			return nil, err
		}
		return nil, errs.Errorf(codes.Internal,
			"create_tenant: %v", err)
	}

	return &pb.CreateTenantResponse{
		Success: true,
		Tenant: &pb.TenantDetail{
			TenantId:  tenant.TenantID,
			Name:      tenant.Name,
			Status:    tenant.Status,
			CreatedAt: tenant.CreatedAt,
			Region:    tenant.Region,
		},
	}, nil
}
