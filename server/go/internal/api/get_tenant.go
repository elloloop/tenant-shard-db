// GetTenant RPC.
// Spec: docs/go-port/rpcs/GetTenant.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:119 (rpc), :880-883
// (request), :885-888 (response), :853-863 (TenantDetail).
//
// Semantics:
//
//   - Read-only on globalstore.tenant_registry. No WAL append, no
//     per-tenant SQLite touch (CLAUDE.md invariants #1 / #4 satisfied
//     trivially — the registry is the cross-tenant exception).
//   - NO membership / admin gate. Any authenticated caller can read any
//     tenant's metadata. This cross-tenant metadata leak is preserved
//     and is REQUIRED, not a bug we're free to fix here. Flagged as a
//     follow-up in the spec "Open questions / risks" section.
//   - Authoritative actor is still resolved via auth.Authoritative so
//     the privilege-escalation fix (commit fece3fb) holds: a malicious
//     caller cannot claim `actor: "system:admin"` and expect the
//     handler to honour the body claim. The actor is then NOT consulted
//     for any authorization decision.
//   - Argument validation (`actor == ""` / `tenant_id == ""`) returns
//     `INVALID_ARGUMENT` cleanly via status.Error, BEFORE the recover
//     block. The contract test at test_grpc_contract.py:472-476
//     accepts either form. See spec "Error contract".
//   - All other unexpected errors / panics degrade to
//     `&GetTenantResponse{Found: false}, nil` with metric label "error".
//   - `s.global == nil` returns `UNIMPLEMENTED` (defensive — no test
//     pins this path but it guards against partial deployments).

package api

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

const getTenantMethod = "GetTenant"

// GetTenant returns the tenant_registry row for `tenant_id`, or
// `found=false` when the tenant does not exist. See file header for the
// auth posture (intentionally-no-permission-check, cross-tenant
// metadata reads allowed) and the swallow-errors-as-OK contract.
func (s *Server) GetTenant(ctx context.Context, req *pb.GetTenantRequest) (resp *pb.GetTenantResponse, err error) {
	start := time.Now()
	status := "ok"
	defer func() {
		metrics.RecordGRPCRequest(ctx, getTenantMethod, status, time.Since(start))
	}()

	// Defensive: registry not configured. No test pins this path but it
	// guards partial deployments. Emit BEFORE arg validation so the
	// signal is useful in monitoring (a misconfigured node should not
	// be masked by a bad request).
	if s.global == nil {
		status = "error"
		return nil, errs.Errorf(codes.Unimplemented, "GetTenant: tenant registry not configured")
	}

	// Argument validation BEFORE the recover block so INVALID_ARGUMENT
	// surfaces cleanly to the client (spec "Error contract" wart).
	if req.GetActor() == "" {
		status = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "GetTenant: actor is required")
	}
	if req.GetTenantId() == "" {
		status = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "GetTenant: tenant_id is required")
	}

	// Resolve the trusted actor. The result is intentionally NOT used
	// for any membership/admin check — any authenticated caller may
	// read any tenant's row. We still call Authoritative so the call
	// shape matches every other registry RPC (defence in depth: if a
	// future hardening pass adds a gate here, it gets the trusted
	// identity, not the wire claim).
	_ = auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))

	// Defer recover AFTER input validation so a panic in the lookup
	// degrades to `found=false` without swallowing INVALID_ARGUMENT.
	defer func() {
		if r := recover(); r != nil {
			status = "error"
			resp = &pb.GetTenantResponse{Found: false}
			err = nil
			_ = r
		}
	}()

	tenant, lookupErr := s.global.GetTenant(ctx, req.GetTenantId())
	if lookupErr != nil {
		// Return found=false with OK status. Do NOT propagate as
		// codes.Internal — the contract test at
		// test_grpc_contract.py:466-471 only accepts `r.found is False`
		// on the negative path.
		status = "error"
		return &pb.GetTenantResponse{Found: false}, nil
	}
	if tenant == nil {
		// Genuine not-found. Metric label stays "ok" — this is the
		// success path for a well-formed lookup.
		return &pb.GetTenantResponse{Found: false}, nil
	}

	return &pb.GetTenantResponse{
		Found: true,
		Tenant: &pb.TenantDetail{
			TenantId:  tenant.TenantID,
			Name:      tenant.Name,
			Status:    tenant.Status,
			CreatedAt: tenant.CreatedAt,
			Region:    tenant.Region,
		},
	}, nil
}
