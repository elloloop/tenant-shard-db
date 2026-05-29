// GetTenantQuota RPC.
// Spec: docs/go-port/rpcs/GetTenantQuota.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:142 (rpc), :1063-1068 (request),
// :1070-1085 (response).
//
// Semantics:
//
//   - Read-only. No WAL append, no per-tenant SQLite touch. Reads two rows
//     out of globalstore (tenant_quotas + tenant_usage); both fall back to
//     zero-valued defaults when the row is absent.
//   - Auth: trusted-actor. The proto `actor` field is a hint only and is
//     deliberately ignored for authorization when an interceptor-attested
//     Identity is on ctx (commit fece3fb, "Fix privilege escalation").
//     When no Identity is present (unit tests, no-auth deployments) we
//     fall through to the claimed actor — the documented Authoritative
//     contract. Mirrors get_authoritative_actor in
//   - Authorization gate: `_require_admin_or_owner` semantics. The
//     trusted actor passes if it is system:/admin:, or if it is a
//     user: whose tenant role is "owner" or "admin". Plain members and
//     non-members → PERMISSION_DENIED. Empty tenant_id →
//     INVALID_ARGUMENT.
//   - period_end_ms is computed locally from time.Now(), never read from
//     the DB — keeps dashboards correct for tenants with no writes this
//     period.

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

const grpcMethodGetTenantQuota = "GetTenantQuota"

// GetTenantQuota implements entdb.v1.EntDBService/GetTenantQuota.
func (s *Server) GetTenantQuota(
	ctx context.Context,
	req *pb.GetTenantQuotaRequest,
) (*pb.GetTenantQuotaResponse, error) {
	start := time.Now()
	outcome := "ok"
	defer func() {
		metrics.RecordGRPCRequest(ctx, grpcMethodGetTenantQuota, outcome, time.Since(start))
	}()

	tenantID := req.GetTenantId()
	if tenantID == "" {
		outcome = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "tenant_id is required")
	}

	// Tenant gate (sharding ownership / region pin / NotFound). Same
	// ingress contract as every other tenant-scoped RPC.
	if err := s.checkTenant(ctx, tenantID); err != nil {
		outcome = "error"
		return nil, err
	}

	// Trusted-actor rebind. When the auth interceptor has populated an
	// Identity on ctx, Authoritative returns that identity and the
	// claimed actor is ignored — this is the privilege-escalation guard
	// (commit fece3fb). When no interceptor ran (unit tests, contract
	// harness without auth), Authoritative falls through to the claimed
	// actor.
	claimed := auth.ParseActor(req.GetActor())
	trusted := auth.Authoritative(ctx, claimed)

	if s.global == nil {
		outcome = "error"
		return nil, errs.Errorf(codes.Unimplemented,
			"GetTenantQuota: quota registry not configured")
	}

	// Admin/owner gate:
	//   - system:/admin: actors bypass the membership check.
	//   - user: actors must have role "owner" or "admin" on the tenant.
	// Plain members and non-members are rejected.
	if !(trusted.IsSystem() || trusted.IsAdmin()) {
		role, err := s.lookupMemberRole(ctx, tenantID, trusted.ID())
		if err != nil {
			outcome = "error"
			return nil, errs.Internal(ctx, "GetTenantQuota: lookup member role", err)
		}
		if role != "owner" && role != "admin" {
			outcome = "error"
			return nil, errs.Errorf(codes.PermissionDenied,
				"GetTenantQuota requires admin or owner role")
		}
	}

	cfg, err := s.global.GetTenantQuota(ctx, tenantID)
	if err != nil {
		outcome = "error"
		return nil, errs.Internal(ctx, "GetTenantQuota: read quota config", err)
	}
	usage, err := s.global.GetUsage(ctx, tenantID)
	if err != nil {
		outcome = "error"
		return nil, errs.Internal(ctx, "GetTenantQuota: read usage", err)
	}

	// period_end_ms is computed locally — never read from the DB row.
	// See spec, "Phase 1 — monthly write quota": dashboards must show a
	// fresh upcoming-rollover even for tenants with no writes this
	// period.
	periodEnd := nextCalendarMonthStartMs(time.Now())

	resp := &pb.GetTenantQuotaResponse{
		TenantId:      tenantID,
		WritesUsed:    usage.WritesCount,
		PeriodStartMs: usage.PeriodStartMs,
		PeriodEndMs:   periodEnd,
	}
	if cfg != nil {
		resp.MaxWritesPerMonth = cfg.MaxWritesPerMonth
		resp.MaxRpsSustained = int32(cfg.MaxRPSSustained)
		resp.MaxRpsBurst = int32(cfg.MaxRPSBurst)
		resp.MaxRpsPerUserSustained = int32(cfg.MaxRPSPerUserSustained)
		resp.MaxRpsPerUserBurst = int32(cfg.MaxRPSPerUserBurst)
		resp.HardEnforce = cfg.HardEnforce
	}
	return resp, nil
}

// nextCalendarMonthStartMs returns the Unix-millisecond timestamp of the
// start of the UTC calendar month immediately following the one
// containing t. The equivalent for the start-of-current-month lives in
// the globalstore package as calendarMonthStartMs.
func nextCalendarMonthStartMs(t time.Time) int64 {
	t = t.UTC()
	year, month := t.Year(), t.Month()
	// time.Date normalises month=13 → next year's January, so we let
	// the stdlib do the rollover math.
	next := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)
	return next.UnixMilli()
}
