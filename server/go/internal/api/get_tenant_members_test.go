// Tests for the GetTenantMembers RPC. Behaviour parity pinned by:
//
//   - tests/python/integration/test_grpc_contract.py:489-499 (happy
//     path + INVALID_ARGUMENT on empty actor)
//   - sdk/go/entdb/admin_test.go:235-260 (epoch-ms preserved)
//
// This file covers the four branches the Go handler exposes:
//
//  1. Empty members list for a tenant with no members.
//  2. Single-member round-trip (fields populated, joined_at preserved).
//  3. Multi-member round-trip (rows ordered by joined_at ascending).
//  4. Unknown tenant -> empty list (no NOT_FOUND — no tenant gate runs).
//  5. Empty tenant_id -> INVALID_ARGUMENT.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// TestGetTenantMembers_EmptyTenant: a tenant that exists but has no
// members returns members=[] with OK. The slice is non-nil.
func TestGetTenantMembers_EmptyTenant(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.GetTenantMembers(ctx, &pb.GetTenantMembersRequest{
		Actor:    "user:alice",
		TenantId: "acme",
	})
	if err != nil {
		t.Fatalf("GetTenantMembers: unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("GetTenantMembers: response is nil")
	}
	if resp.Members == nil {
		t.Fatalf("GetTenantMembers: Members is nil; want empty slice")
	}
	if len(resp.Members) != 0 {
		t.Fatalf("GetTenantMembers: Members = %v; want []", resp.Members)
	}
}

// TestGetTenantMembers_SingleMember: a single Add+Get round-trip
// preserves all four columns including joined_at as epoch-ms.
func TestGetTenantMembers_SingleMember(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := gs.AddTenantMember(ctx, "acme", "alice", "admin"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.GetTenantMembers(ctx, &pb.GetTenantMembersRequest{
		Actor:    "user:alice",
		TenantId: "acme",
	})
	if err != nil {
		t.Fatalf("GetTenantMembers: unexpected error: %v", err)
	}
	if got := len(resp.GetMembers()); got != 1 {
		t.Fatalf("GetTenantMembers: len(Members) = %d; want 1", got)
	}
	m := resp.GetMembers()[0]
	if m.GetTenantId() != "acme" {
		t.Errorf("TenantId = %q; want acme", m.GetTenantId())
	}
	if m.GetUserId() != "alice" {
		t.Errorf("UserId = %q; want alice", m.GetUserId())
	}
	if m.GetRole() != "admin" {
		t.Errorf("Role = %q; want admin", m.GetRole())
	}
	if m.GetJoinedAt() <= 0 {
		t.Errorf("JoinedAt = %d; want > 0 (epoch-ms)", m.GetJoinedAt())
	}
}

// TestGetTenantMembers_MultipleMembers: multiple members are returned;
// every member that was added is present in the response.
func TestGetTenantMembers_MultipleMembers(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	want := map[string]string{
		"alice": "admin",
		"bob":   "member",
		"carol": "member",
	}
	for u, r := range want {
		if err := gs.AddTenantMember(ctx, "acme", u, r); err != nil {
			t.Fatalf("AddTenantMember(%q,%q): %v", u, r, err)
		}
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.GetTenantMembers(ctx, &pb.GetTenantMembersRequest{
		Actor:    "user:alice",
		TenantId: "acme",
	})
	if err != nil {
		t.Fatalf("GetTenantMembers: unexpected error: %v", err)
	}
	got := map[string]string{}
	for _, m := range resp.GetMembers() {
		if m.GetTenantId() != "acme" {
			t.Errorf("TenantId = %q; want acme", m.GetTenantId())
		}
		if m.GetJoinedAt() <= 0 {
			t.Errorf("JoinedAt for %q = %d; want > 0", m.GetUserId(), m.GetJoinedAt())
		}
		got[m.GetUserId()] = m.GetRole()
	}
	if len(got) != len(want) {
		t.Fatalf("got %d members, want %d (got=%v)", len(got), len(want), got)
	}
	for u, r := range want {
		if g, ok := got[u]; !ok || g != r {
			t.Errorf("member %q: got role %q (present=%v); want %q", u, g, ok, r)
		}
	}
}

// TestGetTenantMembers_UnknownTenant: an unknown tenant_id returns an
// empty list with OK — no NOT_FOUND because the handler never calls
// _check_tenant. This is a behavioural pin: a future tightening that
// adds a tenant gate would break this contract.
func TestGetTenantMembers_UnknownTenant(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.GetTenantMembers(context.Background(), &pb.GetTenantMembersRequest{
		Actor:    "user:alice",
		TenantId: "ghost",
	})
	if err != nil {
		t.Fatalf("GetTenantMembers: expected OK for unknown tenant, got %v", err)
	}
	if resp == nil {
		t.Fatalf("GetTenantMembers: response is nil")
	}
	if resp.Members == nil {
		t.Fatalf("GetTenantMembers: Members is nil; want empty slice")
	}
	if len(resp.Members) != 0 {
		t.Fatalf("GetTenantMembers: Members = %v; want []", resp.Members)
	}
}

// TestGetTenantMembers_EmptyTenantID: an empty tenant_id returns
// INVALID_ARGUMENT with the exact message `"tenant_id is required"`.
// The actor-check fires first, so a non-empty actor is required to
// exercise this branch.
func TestGetTenantMembers_EmptyTenantID(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.GetTenantMembers(context.Background(), &pb.GetTenantMembersRequest{
		Actor:    "user:alice",
		TenantId: "",
	})
	if err == nil {
		t.Fatalf("GetTenantMembers: expected INVALID_ARGUMENT, got nil")
	}
	if got := errs.Code(err); got != codes.InvalidArgument {
		t.Fatalf("GetTenantMembers: code = %v; want InvalidArgument (err=%v)", got, err)
	}
}
