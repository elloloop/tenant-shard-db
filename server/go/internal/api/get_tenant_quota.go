// GetTenantQuota RPC — Wave 2 of the Python → Go server port (EPIC #407).
// Spec: docs/go-port/rpcs/GetTenantQuota.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:142 (rpc), :1063-1068 (request),
// :1070-1085 (response). Reference Python:
// server/python/entdb_server/api/grpc_server.py:3106-3163.
//
// Semantics:
//
//   - Read-only. No WAL append, no per-tenant SQLite touch. Reads two rows
//     out of globalstore (tenant_quotas + tenant_usage); both fall back to
//     zero-valued defaults when the row is absent.
//   - Auth: trusted-actor. The proto `actor` field is a hint only and is
//     deliberately ignored for authorization (see commit fece3fb,
//     "Fix privilege escalation"). Authoritative actor is taken from the
//     interceptor-populated Identity on ctx.
//   - Authorization gate: caller must be a member of the tenant
//     (any role, including admin/owner). Non-members → PERMISSION_DENIED.
//     Empty tenant_id → INVALID_ARGUMENT.
//   - period_end_ms is computed locally from time.Now(), never read from
//     the DB — keeps dashboards correct for tenants with no writes this
//     period (parity with grpc_server.py:3137-3141).

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
		metrics.RecordGRPCRequest(grpcMethodGetTenantQuota, outcome, time.Since(start))
	}()

	tenantID := req.GetTenantId()
	if tenantID == "" {
		outcome = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "tenant_id is required")
	}

	// Tenant gate (sharding ownership / region pin / NotFound). Same
	// ingress contract as every other tenant-scoped RPC; matches the
	// Python `_check_tenant` call before the admin gate at
	// grpc_server.py:3122-3128.
	if err := s.checkTenant(ctx, tenantID); err != nil {
		outcome = "error"
		return nil, err
	}

	// Trusted actor: proto `actor` is ignored. The interceptor places
	// the verified Identity on ctx; auth.Authoritative normalises it to
	// an Actor. If the test/server is wired without an auth interceptor,
	// Authoritative falls through to the claimed actor — but we then
	// drop it on the floor and require a real trusted identity below.
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		outcome = "error"
		return nil, errs.Errorf(codes.PermissionDenied,
			"GetTenantQuota: no trusted identity")
	}
	trusted := auth.Authoritative(ctx, auth.User(id.Subject))

	// Membership gate: any member (including admin/owner) may read the
	// tenant's quota dashboard. Non-members are rejected. Mirrors
	// "_require_admin_or_owner" semantics relaxed to member-or-admin
	// per the W2 task spec for parity dashboards.
	if s.global == nil {
		outcome = "error"
		return nil, errs.Errorf(codes.Unimplemented,
			"GetTenantQuota: quota registry not configured")
	}
	member, err := s.global.IsMember(ctx, tenantID, trusted.ID())
	if err != nil {
		outcome = "error"
		return nil, errs.Errorf(codes.Internal,
			"GetTenantQuota: membership probe: %v", err)
	}
	if !member {
		outcome = "error"
		return nil, errs.Errorf(codes.PermissionDenied,
			"GetTenantQuota: caller is not a member of %q", tenantID)
	}

	cfg, err := s.global.GetTenantQuota(ctx, tenantID)
	if err != nil {
		outcome = "error"
		return nil, errs.Errorf(codes.Internal,
			"GetTenantQuota: read quota config: %v", err)
	}
	usage, err := s.global.GetUsage(ctx, tenantID)
	if err != nil {
		outcome = "error"
		return nil, errs.Errorf(codes.Internal,
			"GetTenantQuota: read usage: %v", err)
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
// containing t. Mirrors `_next_calendar_month_start_ms`
// (auth/quota_interceptor.py:65-71). Kept local to this file to avoid
// adding new exported helpers / fields per the W2 task constraints; the
// equivalent for the start-of-current-month lives in the globalstore
// package as calendarMonthStartMs.
func nextCalendarMonthStartMs(t time.Time) int64 {
	t = t.UTC()
	year, month := t.Year(), t.Month()
	// time.Date normalises month=13 → next year's January, so we let
	// the stdlib do the rollover math.
	next := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)
	return next.UnixMilli()
}
