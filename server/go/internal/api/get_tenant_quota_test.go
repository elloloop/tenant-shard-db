// Tests for GetTenantQuota (Wave 2 / EPIC #407). Behavioural pins:
//
//  1. Member happy path — tenant member sees configured quota + usage.
//  2. Admin happy path — admin role works the same as member.
//  3. Non-member → PERMISSION_DENIED, even with claimed actor on the wire.
//  4. Quota not configured → defaults (zero-valued cfg fields) but
//     period_end_ms is still computed and writes_used / period_start_ms
//     come from the synthesized usage row.
//  5. Empty tenant_id → INVALID_ARGUMENT (gate runs before tenant check).
//
// Spec: docs/go-port/rpcs/GetTenantQuota.md.
// Privilege-escalation pin: claim-on-wire is ignored, trusted Identity wins.

package api_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// withTrustedUser returns a ctx that carries the given subject as the
// trusted user identity, mirroring what the auth interceptor sets on
// every authenticated request.
func withTrustedUser(ctx context.Context, subject string) context.Context {
	return auth.WithIdentity(ctx, auth.Identity{
		Method:  auth.MethodSession,
		Subject: subject,
	})
}

// TestGetTenantQuota_Member_HappyPath: a tenant member with role=member
// can read the quota dashboard; configured cfg + usage round-trip into
// the response and period_end_ms is in the future.
func TestGetTenantQuota_Member_HappyPath(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()

	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := gs.AddTenantMember(ctx, "acme", "alice", "member"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}
	if _, err := gs.SetTenantQuota(ctx, globalstore.QuotaConfig{
		TenantID:               "acme",
		MaxWritesPerMonth:      1_000_000,
		HardEnforce:            true,
		MaxRPSSustained:        100,
		MaxRPSBurst:            200,
		MaxRPSPerUserSustained: 10,
		MaxRPSPerUserBurst:     20,
	}); err != nil {
		t.Fatalf("SetTenantQuota: %v", err)
	}
	if _, err := gs.IncrementUsage(ctx, "acme", 7); err != nil {
		t.Fatalf("IncrementUsage: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))
	ctx = withTrustedUser(ctx, "alice")

	resp, err := srv.GetTenantQuota(ctx, &pb.GetTenantQuotaRequest{TenantId: "acme"})
	if err != nil {
		t.Fatalf("GetTenantQuota: unexpected error: %v", err)
	}
	if resp.GetTenantId() != "acme" {
		t.Errorf("TenantId = %q, want %q", resp.GetTenantId(), "acme")
	}
	if resp.GetMaxWritesPerMonth() != 1_000_000 {
		t.Errorf("MaxWritesPerMonth = %d, want 1000000", resp.GetMaxWritesPerMonth())
	}
	if resp.GetWritesUsed() != 7 {
		t.Errorf("WritesUsed = %d, want 7", resp.GetWritesUsed())
	}
	if resp.GetPeriodStartMs() <= 0 {
		t.Errorf("PeriodStartMs = %d, want > 0", resp.GetPeriodStartMs())
	}
	if resp.GetPeriodEndMs() <= resp.GetPeriodStartMs() {
		t.Errorf("PeriodEndMs (%d) must be strictly after PeriodStartMs (%d)",
			resp.GetPeriodEndMs(), resp.GetPeriodStartMs())
	}
	if resp.GetPeriodEndMs() <= time.Now().UnixMilli() {
		t.Errorf("PeriodEndMs (%d) must be strictly after now", resp.GetPeriodEndMs())
	}
	if resp.GetMaxRpsSustained() != 100 || resp.GetMaxRpsBurst() != 200 {
		t.Errorf("rps sustained/burst = %d/%d, want 100/200",
			resp.GetMaxRpsSustained(), resp.GetMaxRpsBurst())
	}
	if resp.GetMaxRpsPerUserSustained() != 10 || resp.GetMaxRpsPerUserBurst() != 20 {
		t.Errorf("per-user rps sustained/burst = %d/%d, want 10/20",
			resp.GetMaxRpsPerUserSustained(), resp.GetMaxRpsPerUserBurst())
	}
	if !resp.GetHardEnforce() {
		t.Errorf("HardEnforce = false, want true")
	}
}

// TestGetTenantQuota_Admin_HappyPath: an admin role is treated identically
// to a member for read access (per W2 spec — read is gated by membership,
// admin is just one specific role).
func TestGetTenantQuota_Admin_HappyPath(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()

	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := gs.AddTenantMember(ctx, "acme", "boss", "admin"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}
	if _, err := gs.SetTenantQuota(ctx, globalstore.QuotaConfig{
		TenantID:          "acme",
		MaxWritesPerMonth: 42,
	}); err != nil {
		t.Fatalf("SetTenantQuota: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))
	ctx = withTrustedUser(ctx, "boss")

	resp, err := srv.GetTenantQuota(ctx, &pb.GetTenantQuotaRequest{TenantId: "acme"})
	if err != nil {
		t.Fatalf("GetTenantQuota: unexpected error: %v", err)
	}
	if resp.GetMaxWritesPerMonth() != 42 {
		t.Errorf("MaxWritesPerMonth = %d, want 42", resp.GetMaxWritesPerMonth())
	}
}

// TestGetTenantQuota_NonMember_PermissionDenied: a caller who is NOT a
// member of the tenant — even one who claims to be `system:root` on the
// wire actor field — must be rejected. Pins the privilege-escalation
// guard for this RPC.
func TestGetTenantQuota_NonMember_PermissionDenied(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()

	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	// "mallory" is authenticated but NOT a member of acme.
	srv := api.New(api.WithGlobalStore(gs))
	ctx = withTrustedUser(ctx, "mallory")

	_, err := srv.GetTenantQuota(ctx, &pb.GetTenantQuotaRequest{
		TenantId: "acme",
		Actor:    "system:root", // wire-claimed; MUST be ignored.
	})
	if err == nil {
		t.Fatalf("GetTenantQuota: expected PermissionDenied, got nil")
	}
	if got := errs.Code(err); got != codes.PermissionDenied {
		t.Fatalf("GetTenantQuota: code = %v, want PermissionDenied (err=%v)", got, err)
	}
}

// TestGetTenantQuota_NoQuotaConfigured_ReturnsDefaults: when no
// tenant_quotas row exists, all cfg-derived fields are zero but the
// response is still OK and period_end_ms is computed.
func TestGetTenantQuota_NoQuotaConfigured_ReturnsDefaults(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()

	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := gs.AddTenantMember(ctx, "acme", "alice", "member"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))
	ctx = withTrustedUser(ctx, "alice")

	resp, err := srv.GetTenantQuota(ctx, &pb.GetTenantQuotaRequest{TenantId: "acme"})
	if err != nil {
		t.Fatalf("GetTenantQuota: unexpected error: %v", err)
	}
	if resp.GetMaxWritesPerMonth() != 0 {
		t.Errorf("MaxWritesPerMonth = %d, want 0 (default)", resp.GetMaxWritesPerMonth())
	}
	if resp.GetWritesUsed() != 0 {
		t.Errorf("WritesUsed = %d, want 0 (default)", resp.GetWritesUsed())
	}
	if resp.GetPeriodStartMs() <= 0 {
		t.Errorf("PeriodStartMs = %d, want > 0 (synthesized current month)", resp.GetPeriodStartMs())
	}
	if resp.GetPeriodEndMs() <= time.Now().UnixMilli() {
		t.Errorf("PeriodEndMs (%d) must be strictly after now", resp.GetPeriodEndMs())
	}
	if resp.GetMaxRpsSustained() != 0 || resp.GetMaxRpsBurst() != 0 {
		t.Errorf("rps fields = %d/%d, want 0/0",
			resp.GetMaxRpsSustained(), resp.GetMaxRpsBurst())
	}
	if resp.GetHardEnforce() {
		t.Errorf("HardEnforce = true, want false (default)")
	}
}

// TestGetTenantQuota_EmptyTenantID_InvalidArgument: empty tenant_id is
// rejected before any DB read or auth check.
func TestGetTenantQuota_EmptyTenantID_InvalidArgument(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	ctx := withTrustedUser(context.Background(), "alice")
	_, err := srv.GetTenantQuota(ctx, &pb.GetTenantQuotaRequest{TenantId: ""})
	if err == nil {
		t.Fatalf("GetTenantQuota: expected InvalidArgument, got nil")
	}
	if got := errs.Code(err); got != codes.InvalidArgument {
		t.Fatalf("GetTenantQuota: code = %v, want InvalidArgument (err=%v)", got, err)
	}
}
