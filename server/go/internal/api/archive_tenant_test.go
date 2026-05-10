// SPDX-License-Identifier: AGPL-3.0-only

// Tests for the ArchiveTenant RPC. Spec: docs/go-port/rpcs/ArchiveTenant.md.
//
// Behavioural pins (mirror Python):
//   - tests/python/integration/test_grpc_contract.py:683-693 — admin actor
//     happy path returns success=true; empty actor returns INVALID_ARGUMENT.
//   - tests/python/unit/test_tenant_registry.py:347-391 — admin succeeds
//     without membership; non-admin / non-owner is PERMISSION_DENIED.
//   - Spec "Open questions" item 6 — re-archiving is idempotent
//     (UPDATE still matches the row → success=true).
//   - Python `:2431-2433` — unknown tenant returns OK + success=false
//     with error="Tenant not found"; this is an intentional asymmetry
//     vs. validation/auth which abort with a status code.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// TestArchiveTenant_AdminHappyPath: admin-prefixed trusted actor archives
// an existing tenant; response is success=true; status flips to "archived"
// in the global store.
func TestArchiveTenant_AdminHappyPath(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.ArchiveTenant(ctx, &pb.ArchiveTenantRequest{
		Actor:    "admin:root",
		TenantId: "acme",
	})
	if err != nil {
		t.Fatalf("ArchiveTenant: unexpected error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("ArchiveTenant: success=false, error=%q; want success=true", resp.GetError())
	}
	if resp.GetError() != "" {
		t.Errorf("ArchiveTenant: unexpected error message %q", resp.GetError())
	}

	// Confirm side effect: tenant_registry.status flipped to "archived".
	tnt, gerr := gs.GetTenant(ctx, "acme")
	if gerr != nil {
		t.Fatalf("GetTenant after archive: %v", gerr)
	}
	if tnt == nil {
		t.Fatalf("GetTenant after archive: tenant disappeared")
	}
	if tnt.Status != "archived" {
		t.Errorf("post-archive status = %q; want %q", tnt.Status, "archived")
	}
}

// TestArchiveTenant_SystemHappyPath: system: actor (e.g. internal control
// plane caller) is treated the same as admin: by IsAdminOrSystem; pinned
// here so we don't regress to admin-only.
func TestArchiveTenant_SystemHappyPath(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.ArchiveTenant(ctx, &pb.ArchiveTenantRequest{
		Actor:    "system:control-plane",
		TenantId: "acme",
	})
	if err != nil {
		t.Fatalf("ArchiveTenant: unexpected error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("ArchiveTenant (system): success=false, error=%q", resp.GetError())
	}
}

// TestArchiveTenant_NonAdminPermissionDenied: a user: actor with no
// member-role bypass MUST be rejected. Wave-2 is admin-only — the
// owner-can-archive branch is deferred to the WAL-first restoration PR.
func TestArchiveTenant_NonAdminPermissionDenied(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.ArchiveTenant(ctx, &pb.ArchiveTenantRequest{
		Actor:    "user:bob",
		TenantId: "acme",
	})
	if err == nil {
		t.Fatalf("ArchiveTenant: expected PERMISSION_DENIED, got nil error")
	}
	if got := errs.Code(err); got != codes.PermissionDenied {
		t.Fatalf("ArchiveTenant: code = %v; want PermissionDenied (err=%v)", got, err)
	}

	// Side-effect probe: tenant must remain active.
	tnt, gerr := gs.GetTenant(ctx, "acme")
	if gerr != nil {
		t.Fatalf("GetTenant: %v", gerr)
	}
	if tnt.Status != "active" {
		t.Errorf("post-deny status = %q; want %q (no side effect on permission denial)",
			tnt.Status, "active")
	}
}

// TestArchiveTenant_UnknownTenant: an admin call against a tenant id that
// is not in the registry returns OK + success=false + error="Tenant not
// found". Python parity (`grpc_server.py:2431-2433`) — this is asymmetric
// vs. validation/auth which abort with a status code.
func TestArchiveTenant_UnknownTenant(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.ArchiveTenant(context.Background(), &pb.ArchiveTenantRequest{
		Actor:    "admin:root",
		TenantId: "ghost",
	})
	if err != nil {
		t.Fatalf("ArchiveTenant: unexpected error: %v", err)
	}
	if resp.GetSuccess() {
		t.Errorf("ArchiveTenant (ghost): success=true; want false")
	}
	if resp.GetError() != "Tenant not found" {
		t.Errorf("ArchiveTenant (ghost): error=%q; want %q", resp.GetError(), "Tenant not found")
	}
}

// TestArchiveTenant_AlreadyArchivedIsIdempotent: re-archiving an already-
// archived tenant succeeds (the UPDATE still matches the row). Pinned by
// spec "Open questions" item 6.
func TestArchiveTenant_AlreadyArchivedIsIdempotent(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	first, err := srv.ArchiveTenant(ctx, &pb.ArchiveTenantRequest{
		Actor:    "admin:root",
		TenantId: "acme",
	})
	if err != nil {
		t.Fatalf("first ArchiveTenant: %v", err)
	}
	if !first.GetSuccess() {
		t.Fatalf("first ArchiveTenant: success=false, error=%q", first.GetError())
	}

	second, err := srv.ArchiveTenant(ctx, &pb.ArchiveTenantRequest{
		Actor:    "admin:root",
		TenantId: "acme",
	})
	if err != nil {
		t.Fatalf("second ArchiveTenant: %v", err)
	}
	if !second.GetSuccess() {
		t.Errorf("second ArchiveTenant: success=false, error=%q; want success=true (idempotent)",
			second.GetError())
	}
}

// TestArchiveTenant_EmptyActorInvalidArgument: empty wire actor must
// surface as INVALID_ARGUMENT before any auth decision. Pinned by
// test_grpc_contract.py:690-693.
func TestArchiveTenant_EmptyActorInvalidArgument(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.ArchiveTenant(context.Background(), &pb.ArchiveTenantRequest{
		Actor:    "",
		TenantId: "acme",
	})
	if err == nil {
		t.Fatalf("ArchiveTenant: expected INVALID_ARGUMENT, got nil")
	}
	if got := errs.Code(err); got != codes.InvalidArgument {
		t.Fatalf("ArchiveTenant: code = %v; want InvalidArgument (err=%v)", got, err)
	}
}

// TestArchiveTenant_EmptyTenantIDInvalidArgument: empty tenant_id must
// also surface INVALID_ARGUMENT. Pinned by Python `:2413-2414`.
func TestArchiveTenant_EmptyTenantIDInvalidArgument(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.ArchiveTenant(context.Background(), &pb.ArchiveTenantRequest{
		Actor:    "admin:root",
		TenantId: "",
	})
	if err == nil {
		t.Fatalf("ArchiveTenant: expected INVALID_ARGUMENT, got nil")
	}
	if got := errs.Code(err); got != codes.InvalidArgument {
		t.Fatalf("ArchiveTenant: code = %v; want InvalidArgument (err=%v)", got, err)
	}
}

// TestArchiveTenant_GlobalStoreNotConfigured: when the optional global_store
// dep is absent, the handler returns UNIMPLEMENTED. Pinned by Python
// `:2406-2409`.
func TestArchiveTenant_GlobalStoreNotConfigured(t *testing.T) {
	t.Parallel()

	srv := api.New() // no WithGlobalStore

	_, err := srv.ArchiveTenant(context.Background(), &pb.ArchiveTenantRequest{
		Actor:    "admin:root",
		TenantId: "acme",
	})
	if err == nil {
		t.Fatalf("ArchiveTenant: expected UNIMPLEMENTED, got nil")
	}
	if got := errs.Code(err); got != codes.Unimplemented {
		t.Fatalf("ArchiveTenant: code = %v; want Unimplemented (err=%v)", got, err)
	}
}
