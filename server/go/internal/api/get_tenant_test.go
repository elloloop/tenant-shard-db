// Tests for the GetTenant RPC. Spec: docs/go-port/rpcs/GetTenant.md.
//
// Coverage matches the Python contract pins:
//
//   - test_grpc_contract.py:460-465 / test_tenant_registry.py:307-321
//     happy path: existing tenant round-trips with tenant_id, name,
//     status, created_at, region.
//   - test_grpc_contract.py:466-471 / test_tenant_registry.py:323-334
//     not-found: an unknown tenant_id yields `found=false` with OK
//     status, NOT a NOT_FOUND gRPC error.
//   - test_grpc_contract.py:472-476: empty actor / empty tenant_id
//     yields INVALID_ARGUMENT. The Go port returns this cleanly
//     (Python's wart at grpc_server.py:2392-2395 is fixed; the contract
//     test accepts either form — see spec "Error contract").
//   - test_region_pinning.py:97-106: cross-tenant metadata reads are
//     allowed. Any authenticated caller can fetch any tenant's
//     metadata; the region-pinning test pins this behaviour and we
//     preserve it for parity.

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
// TenantDetail fields populated. Mirrors
// tests/python/unit/test_tenant_registry.py:307-321 plus the SDK-side
// region pin at :804-823.
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
// handler MUST NOT return NOT_FOUND. Mirrors
// tests/python/unit/test_tenant_registry.py:323-334.
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
// INVALID_ARGUMENT. The Go port returns this cleanly via status.Error
// (Python's argument-validation-vs-catch-all wart is fixed here; spec
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
// other side: empty `actor` -> INVALID_ARGUMENT. Python validates
// actor before tenant_id (grpc_server.py:2376-2379) and so do we.
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
// behaviour at tests/python/integration/test_region_pinning.py:97-106:
// any authenticated caller can read any tenant's metadata regardless
// of membership. This is a known information leak preserved for parity
// with Python (spec "Open questions / risks"). The trusted identity on
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
		t.Fatalf("GetTenant cross-tenant: Found = false; want true (parity with test_region_pinning.py:97-106)")
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
// UNIMPLEMENTED path (grpc_server.py:2370-2374). No Python test pins
// this — preserved per spec for partial-deployment safety.
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
