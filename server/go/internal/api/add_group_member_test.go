// SPDX-License-Identifier: AGPL-3.0-only

// Tests for AddGroupMember. The Go handler diverges from the Python
// source on two CLAUDE.md-flagged invariants — see
// add_group_member.go's package-level doc:
//
//  1. WAL-first restoration. The Python handler writes group_users
//     directly via canonical_store, bypassing the WAL. The Go handler
//     appends an `add_group_member` op to the per-tenant WAL and lets
//     the Applier materialize the row. Tests assert on what landed on
//     the WAL — they do NOT poke at SQLite, because the apply path is
//     covered separately in apply/ops_add_group_member.go's tests.
//
//  2. Trusted-actor substitution. The Python handler honours the
//     wire-claimed actor; the Go handler ignores it and resolves
//     identity via auth.Authoritative. The non-admin denial test below
//     pins the privilege-escalation regression (commit fece3fb): a
//     bob-authenticated session that claims admin:root in the request
//     body is rejected as user:bob.
//
// Coverage:
//
//   - TestAddGroupMember_Admin_HappyPath: trusted admin → OK + WAL
//     record observed with the full op shape.
//   - TestAddGroupMember_NonAdmin_PermissionDenied: bob (plain member)
//     claims admin:root in the request body → PERMISSION_DENIED.
//   - TestAddGroupMember_Idempotent: same logical request appended
//     twice → exactly one WAL record (idempotency-key dedupe at the
//     Append boundary).

package api_test

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

const addGroupMemberWALTopic = "entdb-wal"

// addGroupMemberFixture bundles the dependencies a single test case
// needs: a Server wired to a fresh in-memory WAL + globalstore, plus
// raw handles to both so assertions can introspect the WAL log and
// callers can seed tenant_members rows for the auth branches.
type addGroupMemberFixture struct {
	srv *api.Server
	w   *wal.InMemory
	gs  globalStoreLike
}

// globalStoreLike is the narrow interface this file uses to seed
// tenant_members rows. It exists so the test doesn't take a hard
// import on the globalstore package's public surface area beyond what
// it actually exercises.
type globalStoreLike interface {
	AddTenantMember(ctx context.Context, tenantID, userID, role string) error
}

func newAddGroupMemberFixture(t *testing.T) addGroupMemberFixture {
	t.Helper()
	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(context.Background(), "acme", "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	w := wal.NewInMemory(1)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}
	t.Cleanup(func() { _ = w.Close(context.Background()) })

	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithWALProducer(w),
	)
	return addGroupMemberFixture{srv: srv, w: w, gs: gs}
}

// TestAddGroupMember_Admin_HappyPath: a trusted admin: actor adds a
// user to a group. The handler returns OK + success=true and the WAL
// carries exactly one record whose Event.Ops[0] matches the expected
// op-shape (the contract apply/ops_add_group_member.go consumes).
func TestAddGroupMember_Admin_HappyPath(t *testing.T) {
	t.Parallel()

	f := newAddGroupMemberFixture(t)
	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodAPIKey,
		Subject: "admin:root",
	})

	resp, err := f.srv.AddGroupMember(ctx, &pb.GroupMemberRequest{
		Context: &pb.RequestContext{
			TenantId: "acme",
			Actor:    "admin:root",
		},
		GroupId:       "group:engineering",
		MemberActorId: "user:alice",
		Role:          "member",
	})
	if err != nil {
		t.Fatalf("AddGroupMember: unexpected error: %v", err)
	}
	if resp == nil || !resp.GetSuccess() {
		t.Fatalf("AddGroupMember: success=false, error=%q; want success=true",
			resp.GetError())
	}

	recs := f.w.GetAllRecords(addGroupMemberWALTopic)
	if len(recs) != 1 {
		t.Fatalf("WAL records = %d; want 1", len(recs))
	}
	if recs[0].Key != "acme" {
		t.Fatalf("WAL record key = %q; want %q (per-tenant partition key)",
			recs[0].Key, "acme")
	}

	var ev wal.Event
	if err := json.Unmarshal(recs[0].Value, &ev); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if ev.TenantID != "acme" {
		t.Fatalf("Event.TenantID = %q; want %q", ev.TenantID, "acme")
	}
	// Trusted-actor substitution pin: the WAL records the trusted
	// identity, never the wire-claimed one. (Wire claim and trusted
	// match in this test, but the assertion still pins the contract.)
	if ev.Actor != "admin:root" {
		t.Fatalf("Event.Actor = %q; want %q (trusted)", ev.Actor, "admin:root")
	}
	if len(ev.Ops) != 1 {
		t.Fatalf("Event.Ops length = %d; want 1", len(ev.Ops))
	}
	op := ev.Ops[0]
	if got := op["op"]; got != "add_group_member" {
		t.Fatalf("op[\"op\"] = %v; want %q", got, "add_group_member")
	}
	if got := op["group_id"]; got != "group:engineering" {
		t.Fatalf("op[\"group_id\"] = %v; want %q", got, "group:engineering")
	}
	if got := op["member_actor_id"]; got != "user:alice" {
		t.Fatalf("op[\"member_actor_id\"] = %v; want %q", got, "user:alice")
	}
	if got := op["role"]; got != "member" {
		t.Fatalf("op[\"role\"] = %v; want %q", got, "member")
	}
}

// TestAddGroupMember_NonAdmin_PermissionDenied: bob is authenticated
// as a plain tenant member but claims actor=admin:root in the request
// body. The handler MUST ignore the wire claim (privilege-escalation
// regression pinned by commit fece3fb) and reject as user:bob with
// PERMISSION_DENIED. The WAL MUST stay empty — denials never append.
func TestAddGroupMember_NonAdmin_PermissionDenied(t *testing.T) {
	t.Parallel()

	f := newAddGroupMemberFixture(t)
	// Bob is a plain member of acme — not owner, not admin.
	ctx := context.Background()
	if err := f.gs.AddTenantMember(ctx, "acme", "bob", "member"); err != nil {
		t.Fatalf("seed bob: %v", err)
	}

	authedCtx := auth.WithIdentity(ctx, auth.Identity{
		Method:  auth.MethodSession,
		Subject: "user:bob",
	})

	_, err := f.srv.AddGroupMember(authedCtx, &pb.GroupMemberRequest{
		Context: &pb.RequestContext{
			TenantId: "acme",
			Actor:    "admin:root", // wire-claimed escalation attempt
		},
		GroupId:       "group:engineering",
		MemberActorId: "user:carol",
		Role:          "member",
	})
	if err == nil {
		t.Fatalf("AddGroupMember: expected PERMISSION_DENIED, got nil")
	}
	if got := errs.Code(err); got != codes.PermissionDenied {
		t.Fatalf("AddGroupMember: code = %v, want PermissionDenied (err=%v)", got, err)
	}

	if n := len(f.w.GetAllRecords(addGroupMemberWALTopic)); n != 0 {
		t.Fatalf("WAL records after denial = %d; want 0 (denial must not append)", n)
	}
}

// TestAddGroupMember_Idempotent: the same logical request issued twice
// (same tenant + group + member + role) lands exactly one WAL record.
// The idempotency key derived from the tuple flows through the Append
// header so the in-memory WAL's dedupe path returns the original
// StreamPos on the second call.
func TestAddGroupMember_Idempotent(t *testing.T) {
	t.Parallel()

	f := newAddGroupMemberFixture(t)
	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodAPIKey,
		Subject: "admin:root",
	})

	req := &pb.GroupMemberRequest{
		Context: &pb.RequestContext{
			TenantId: "acme",
			Actor:    "admin:root",
		},
		GroupId:       "group:engineering",
		MemberActorId: "user:alice",
		Role:          "member",
	}

	resp1, err := f.srv.AddGroupMember(ctx, req)
	if err != nil {
		t.Fatalf("AddGroupMember (first): %v", err)
	}
	if !resp1.GetSuccess() {
		t.Fatalf("AddGroupMember (first): success=false, error=%q", resp1.GetError())
	}

	resp2, err := f.srv.AddGroupMember(ctx, req)
	if err != nil {
		t.Fatalf("AddGroupMember (re-add): %v", err)
	}
	if !resp2.GetSuccess() {
		t.Fatalf("AddGroupMember (re-add): success=false, error=%q", resp2.GetError())
	}

	if n := len(f.w.GetAllRecords(addGroupMemberWALTopic)); n != 1 {
		t.Fatalf("WAL records after idempotent re-add = %d; want 1", n)
	}
}
