// Tests for the GetTenant RPC. Spec: docs/go-port/rpcs/GetTenant.md.
//
// Coverage matches the contract pins:
//
//   - test_grpc_contract.py:460-465: happy path: existing tenant
//     round-trips with tenant_id, name, status, created_at, region.
//   - test_grpc_contract.py:466-471: not-found: an unknown tenant_id
//     yields `found=false` with OK status, NOT a NOT_FOUND gRPC error.
//   - test_grpc_contract.py:472-476: empty actor / empty tenant_id
//     yields INVALID_ARGUMENT (see spec "Error contract").
//   - Cross-tenant metadata reads are allowed. Any authenticated caller
//     can fetch any tenant's metadata; preserved for parity.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// TestGetTenant_Roundtrip pins the happy path: a tenant created via
// globalstore.CreateTenant round-trips through GetTenant with all
// TenantDetail fields populated.
func TestGetTenant_Roundtrip(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()

	created, err := gs.CreateTenant(ctx, "acme", "Acme Corp", "us-east-1")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.GetTenant(ctx, &pb.GetTenantRequest{
		Actor:    "user:alice",
		TenantId: "acme",
	})
	if err != nil {
		t.Fatalf("GetTenant: unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("GetTenant: response is nil")
	}
	if !resp.GetFound() {
		t.Fatalf("GetTenant: Found = false, want true")
	}
	td := resp.GetTenant()
	if td == nil {
		t.Fatalf("GetTenant: Tenant is nil; want populated TenantDetail")
	}
	if got, want := td.GetTenantId(), "acme"; got != want {
		t.Errorf("TenantId = %q, want %q", got, want)
	}
	if got, want := td.GetName(), "Acme Corp"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
	if got, want := td.GetStatus(), "active"; got != want {
		t.Errorf("Status = %q, want %q", got, want)
	}
	if got, want := td.GetRegion(), "us-east-1"; got != want {
		t.Errorf("Region = %q, want %q", got, want)
	}
	if got, want := td.GetCreatedAt(), created.CreatedAt; got != want {
		t.Errorf("CreatedAt = %d, want %d", got, want)
	}
}

// TestGetTenant_UnknownTenantInBandFalse pins the not-found contract:
// an unknown tenant_id yields `found=false` with no gRPC error. The
// handler MUST NOT return NOT_FOUND.
func TestGetTenant_UnknownTenantInBandFalse(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.GetTenant(context.Background(), &pb.GetTenantRequest{
		Actor:    "user:alice",
		TenantId: "ghost",
	})
	if err != nil {
		t.Fatalf("GetTenant: unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("GetTenant: response is nil")
	}
	if resp.GetFound() {
		t.Errorf("GetTenant: Found = true, want false for unknown tenant")
	}
	if resp.GetTenant() != nil {
		t.Errorf("GetTenant: Tenant = %v, want nil for unknown tenant", resp.GetTenant())
	}
}

// TestGetTenant_EmptyTenantIDInvalidArgument pins
// test_grpc_contract.py:472-476: an empty tenant_id yields
// INVALID_ARGUMENT returned cleanly via status.Error (spec
// "Error contract").
func TestGetTenant_EmptyTenantIDInvalidArgument(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.GetTenant(context.Background(), &pb.GetTenantRequest{
		Actor:    "user:alice",
		TenantId: "",
	})
	if err == nil {
		t.Fatalf("GetTenant: want INVALID_ARGUMENT, got nil")
	}
	if got := errs.Code(err); got != codes.InvalidArgument {
		t.Fatalf("GetTenant: code = %v, want InvalidArgument (err=%v)", got, err)
	}
}

// TestGetTenant_EmptyActorInvalidArgument pins the same wart from the
// other side: empty `actor` -> INVALID_ARGUMENT. Actor is validated
// before tenant_id.
func TestGetTenant_EmptyActorInvalidArgument(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.GetTenant(context.Background(), &pb.GetTenantRequest{
		Actor:    "",
		TenantId: "acme",
	})
	if err == nil {
		t.Fatalf("GetTenant: want INVALID_ARGUMENT, got nil")
	}
	if got := errs.Code(err); got != codes.InvalidArgument {
		t.Fatalf("GetTenant: code = %v, want InvalidArgument (err=%v)", got, err)
	}
}

// TestGetTenant_CrossTenantMetadataLeak pins the region-pinning
// behaviour: any authenticated caller can read any tenant's metadata
// regardless of membership. This is a known information leak preserved
// for parity (spec "Open questions / risks"). The trusted identity on
// ctx is bob, but bob is not a member of t-us — the lookup MUST still
// succeed. Flagging this with a dedicated test ensures a future
// hardening pass cannot silently change behaviour without breaking
// this pin.
func TestGetTenant_CrossTenantMetadataLeak(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()

	// Create a US-pinned tenant; the caller (bob) is not associated
	// with it in any way.
	if _, err := gs.CreateTenant(ctx, "t-us", "US Tenant", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	// Trusted identity: bob (a user who is NOT a member of t-us).
	authed := auth.WithIdentity(ctx, auth.Identity{
		Method:  auth.MethodSession,
		Subject: "user:bob",
	})

	resp, err := srv.GetTenant(authed, &pb.GetTenantRequest{
		Actor:    "user:bob",
		TenantId: "t-us",
	})
	if err != nil {
		t.Fatalf("GetTenant cross-tenant: unexpected error: %v", err)
	}
	if !resp.GetFound() {
		t.Fatalf("GetTenant cross-tenant: Found = false; want true (cross-tenant metadata leak preserved for parity)")
	}
	td := resp.GetTenant()
	if td == nil {
		t.Fatalf("GetTenant cross-tenant: Tenant is nil")
	}
	if got, want := td.GetTenantId(), "t-us"; got != want {
		t.Errorf("TenantId = %q, want %q", got, want)
	}
	// Region leak is the specific bit the region-pinning test pins —
	// non-members can observe data-residency choices.
	if got, want := td.GetRegion(), "us-east-1"; got != want {
		t.Errorf("Region = %q, want %q (region leak preserved for parity)", got, want)
	}
}

// TestGetTenant_NoGlobalStoreUnimplemented pins the defensive
// UNIMPLEMENTED path. Preserved per spec for partial-deployment safety.
func TestGetTenant_NoGlobalStoreUnimplemented(t *testing.T) {
	t.Parallel()

	srv := api.New() // no WithGlobalStore

	_, err := srv.GetTenant(context.Background(), &pb.GetTenantRequest{
		Actor:    "user:alice",
		TenantId: "acme",
	})
	if err == nil {
		t.Fatalf("GetTenant: want UNIMPLEMENTED, got nil")
	}
	if got := errs.Code(err); got != codes.Unimplemented {
		t.Fatalf("GetTenant: code = %v, want Unimplemented (err=%v)", got, err)
	}
}
