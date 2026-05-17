// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// CreateTenant implements the entdb.v1.EntDBService/CreateTenant RPC.
// Spec: docs/go-port/rpcs/CreateTenant.md.
//
// WAL-first control-plane write: the handler appends a global-scope
// tenant_created event and waits for the applier to materialize both
// the tenant_registry row and the owner membership into globalstore.
// The per-tenant SQLite file is created lazily on first data-plane use,
// NOT here.
//
// Two-step shape:
//
//  1. validate (admin gate, non-empty fields)
//  2. append/wait for tenant_created on the global WAL scope
//
// The registry + member writes are applied atomically by globalstore's
// ApplyTenantCreated path, so a replay against a fresh global.db
// reconstructs both rows from the WAL.
//
// Auth contract:
//   - Wire actor (req.Actor) is UNTRUSTED. The trusted principal is
//     resolved via auth.Authoritative(ctx, claimed) — privilege-escalation
//     remediation per commit fece3fb.
//   - Admin-only: decision deviating from current Python (which
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
	if region == "" {
		region = globalstore.DefaultRegion
	}

	if existing, err := s.global.GetTenant(ctx, req.GetTenantId()); err != nil {
		resultStatus = "error"
		return nil, errs.Internal(ctx, "get tenant", err)
	} else if existing != nil {
		resultStatus = "error"
		return nil, errs.Errorf(codes.AlreadyExists,
			"globalstore: tenant %q already exists", req.GetTenantId())
	}

	createdAt := time.Now().Unix()
	_, _, err := s.appendGlobalAdminOp(ctx, principal.String(), map[string]any{
		"op":            string(apply.OpTenantCreated),
		"tenant_id":     req.GetTenantId(),
		"name":          req.GetName(),
		"region":        region,
		"owner_user_id": principal.ID(),
		"created_at":    createdAt,
	})
	if err != nil {
		resultStatus = "error"
		return nil, err
	}

	return &pb.CreateTenantResponse{
		Success: true,
		Tenant: &pb.TenantDetail{
			TenantId:  req.GetTenantId(),
			Name:      req.GetName(),
			Status:    "active",
			CreatedAt: createdAt,
			Region:    region,
		},
	}, nil
}
