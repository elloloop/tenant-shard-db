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
// is created lazily on first data-plane use, NOT here. Wave 2 deliberately
// keeps this carve-out so the bring-up path doesn't depend on the WAL
// applier landing first; a follow-up may either (a) introduce a
// TenantCreated WAL op or (b) document the control plane formally.
//
// Three-step shape (non-atomic, mirrors Python):
//
//  1. validate (admin gate, non-empty fields)
//  2. globalstore.CreateTenant (registry row + region pin column)
//  3. globalstore.AddTenantMember (creator → "owner")
//
// A crash between steps 2 and 3 leaves an orphan tenant with no owner.
// This is the Python parity hazard flagged in the spec under "Open
// questions / risks: Non-atomic three-step write" — preserved here on
// purpose so the contract matches; do NOT wrap these in a single SQLite
// transaction without updating the spec and Python source together.
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

	// Step 2: globalstore write. UNIQUE on tenant_registry.tenant_id
	// surfaces as ALREADY_EXISTS; the handler maps that to a typed gRPC
	// error (NOT the Python success=false channel — Wave-2 decision per
	// task spec, gives SDKs a real status code to dispatch on).
	tenant, err := s.global.CreateTenant(ctx, req.GetTenantId(), req.GetName(), region)
	if err != nil {
		resultStatus = "error"
		if errors.Is(err, errs.ErrAlreadyExists) {
			return nil, err
		}
		return nil, errs.Errorf(codes.Internal,
			"create_tenant: %v", err)
	}

	// Step 3: record the creator as owner. Idempotent on retry — an
	// already-existing membership is swallowed so a crash-then-retry
	// after step 2 succeeds without surfacing a spurious error. The
	// step is non-atomic with step 2 by design (see top-of-file note).
	if err := s.global.AddTenantMember(ctx, tenant.TenantID, principal.ID(), "owner"); err != nil {
		if !errors.Is(err, errs.ErrAlreadyExists) {
			resultStatus = "error"
			return nil, errs.Errorf(codes.Internal,
				"add_tenant_member: %v", err)
		}
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
