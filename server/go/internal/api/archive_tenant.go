// SPDX-License-Identifier: AGPL-3.0-only

// ArchiveTenant RPC.
// Spec: docs/go-port/rpcs/ArchiveTenant.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:120 (rpc), :890-898 (messages).
//
// Behavioural parity with Python is preserved deliberately, including the
// known gap below. The handler:
//
//   - Returns UNIMPLEMENTED when global_store is not wired.
//   - Returns INVALID_ARGUMENT for empty actor / tenant_id.
//   - Resolves the trusted actor via auth.Authoritative (CLAUDE.md
//     trusted-actor invariant — see docs/go-port/shared/auth-interceptor.md).
//   - narrowing: only system: / admin: actors may archive. The
//     Python handler additionally lets the tenant "owner" archive after a
//     member-role lookup; the Go port restricts this to admin-only for now.
//     The owner branch will land alongside the WAL-first restoration (see
//     "Known gaps" below).
//   - Appends a global `tenant_archived` WAL op and waits for the applier
//     to set tenant_registry.status = "archived". Re-archiving an already-
//     archived tenant is idempotent and returns success=true.
//   - Returns OK + success=false + error="Tenant not found" when the row is
//     missing. Asymmetric error contract preserved.
//
// Known gaps:
//
//  1. No archiver / cold-storage handoff, no member or API-key revocation.
//     Mirrors Python's no-op-after-flag behaviour.
//  2. Legal-hold collision: `archived` and `legal_hold` share one status
//     column. Spec "Open questions" item 1 flags this. We do not gate on
//     current status here — Python doesn't either.

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

const archiveTenantMethod = "ArchiveTenant"

// ArchiveTenant flips tenant_registry.status to 'archived' for a given
// tenant. See file header for the full contract.
func (s *Server) ArchiveTenant(
	ctx context.Context,
	req *pb.ArchiveTenantRequest,
) (*pb.ArchiveTenantResponse, error) {
	start := time.Now()
	status := "ok"
	defer func() {
		metrics.RecordGRPCRequest(archiveTenantMethod, status, time.Since(start))
	}()

	// Optional-store guard. Aborts with UNIMPLEMENTED when the global_store
	// dep is not configured.
	if s.global == nil {
		status = "error"
		return nil, errs.Errorf(codes.Unimplemented, "Tenant registry not configured")
	}

	// Required-arg validation. Empty actor / tenant_id surface as
	// INVALID_ARGUMENT before any privilege decision. We MUST validate the
	// wire actor non-empty even though the trusted actor below comes from
	// the interceptor — wire-contract pin (test_grpc_contract.py:690-693).
	if req.GetActor() == "" {
		status = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "actor is required")
	}
	if req.GetTenantId() == "" {
		status = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "tenant_id is required")
	}

	// Resolve the trusted actor. The wire `actor` field is UNTRUSTED — we
	// rebind to the interceptor-bound identity before any authz decision.
	// In no-auth deployments / unit tests with no Identity on ctx, the
	// claimed actor is returned as-is (auth.Authoritative documented
	// fallback) — matching the Python ContextVar fall-through.
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))

	//  narrowing: admin-only. The Python handler also allows the
	// tenant owner via a member-role lookup; owner-allowed archive remains
	// a separate authz follow-up.
	if !trusted.IsAdmin() && !trusted.IsSystem() {
		status = "error"
		return nil, errs.Errorf(codes.PermissionDenied,
			"Only tenant owner can archive a tenant")
	}

	if existing, err := s.global.GetTenant(ctx, req.GetTenantId()); err != nil {
		status = "error"
		return &pb.ArchiveTenantResponse{Success: false, Error: err.Error()}, nil
	} else if existing == nil {
		// Tenant id not found in registry. Asymmetric contract: OK +
		// success=false rather than a NOT_FOUND status code.
		return &pb.ArchiveTenantResponse{Success: false, Error: "Tenant not found"}, nil
	}

	_, _, err := s.appendGlobalAdminOp(ctx, trusted.String(), map[string]any{
		"op":        string(apply.OpTenantArchived),
		"tenant_id": req.GetTenantId(),
	})
	if err != nil {
		status = "error"
		return nil, err
	}
	return &pb.ArchiveTenantResponse{Success: true}, nil
}
