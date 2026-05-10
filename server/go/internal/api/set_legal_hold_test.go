// SPDX-License-Identifier: AGPL-3.0-only

// Tests for the SetLegalHold RPC. Spec: docs/go-port/rpcs/SetLegalHold.md.
//
// Behavioural pins (mirror Python):
//
//   - tests/python/unit/test_admin_operations.py:594-609 — happy-path
//     enable; status flips to "legal_hold".
//   - :611-624                                            — disable returns to "active".
//   - :626-638                                            — non-admin / non-owner -> PERMISSION_DENIED.
//   - :860-868                                            — empty tenant_id -> INVALID_ARGUMENT.
//   - tests/python/integration/test_grpc_contract.py:573-583 — wire-level sweep.
//
// Wave-2 deviations (intentional, documented in set_legal_hold.go header):
//
//   - Unknown tenant -> NOT_FOUND (Python: OK + success=false). All
//     mutating Wave-2 RPCs converge on the status-code form.
//   - Owner role bypass deferred — admin-only. Mirrors ArchiveTenant.

package api_test

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// TestSetLegalHold_AdminEnable_HappyPath: admin actor sets the hold; the
// response carries success=true + status="legal_hold" and the
// tenant_registry row is flipped accordingly. Pinned by
// test_admin_operations.py:594-609.
func TestSetLegalHold_AdminEnable_HappyPath(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor:    "admin:root",
		TenantId: "acme",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("SetLegalHold: unexpected error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("SetLegalHold: success=false, error=%q; want true", resp.GetError())
	}
	if got, want := resp.GetStatus(), "legal_hold"; got != want {
		t.Errorf("response.status = %q; want %q", got, want)
	}
	if resp.GetError() != "" {
		t.Errorf("response.error = %q; want empty", resp.GetError())
	}

	// Side effect: tenant_registry.status flipped.
	tnt, gerr := gs.GetTenant(ctx, "acme")
	if gerr != nil {
		t.Fatalf("GetTenant: %v", gerr)
	}
	if tnt == nil {
		t.Fatalf("GetTenant: tenant disappeared")
	}
	if tnt.Status != "legal_hold" {
		t.Errorf("post-set status = %q; want %q", tnt.Status, "legal_hold")
	}
}

// TestSetLegalHold_SystemEnable_HappyPath: system: actor (e.g. internal
// compliance worker) is treated the same as admin: by the gate.
func TestSetLegalHold_SystemEnable_HappyPath(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor:    "system:compliance",
		TenantId: "acme",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("SetLegalHold (system): unexpected error: %v", err)
	}
	if !resp.GetSuccess() || resp.GetStatus() != "legal_hold" {
		t.Errorf("system response: success=%v status=%q; want true/legal_hold",
			resp.GetSuccess(), resp.GetStatus())
	}
}

// TestSetLegalHold_Clear_TogglesBackToActive: enable then clear leaves
// the registry status back at "active". Pinned by
// test_admin_operations.py:611-624.
func TestSetLegalHold_Clear_TogglesBackToActive(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	// Step 1: enable.
	if _, err := srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor: "admin:root", TenantId: "acme", Enabled: true,
	}); err != nil {
		t.Fatalf("SetLegalHold(enable): %v", err)
	}

	// Step 2: clear.
	resp, err := srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor: "admin:root", TenantId: "acme", Enabled: false,
	})
	if err != nil {
		t.Fatalf("SetLegalHold(clear): %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("clear: success=false, error=%q", resp.GetError())
	}
	if resp.GetStatus() != "active" {
		t.Errorf("clear response.status = %q; want %q", resp.GetStatus(), "active")
	}

	tnt, _ := gs.GetTenant(ctx, "acme")
	if tnt == nil || tnt.Status != "active" {
		t.Errorf("post-clear registry status = %v; want active", tnt)
	}
}

// TestSetLegalHold_NonAdminPermissionDenied: a user: actor (no
// admin/system prefix) is rejected. Pinned by
// test_admin_operations.py:626-638.
func TestSetLegalHold_NonAdminPermissionDenied(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor: "user:bob", TenantId: "acme", Enabled: true,
	})
	if err == nil {
		t.Fatalf("SetLegalHold: expected PERMISSION_DENIED, got nil")
	}
	if got := errs.Code(err); got != codes.PermissionDenied {
		t.Fatalf("SetLegalHold: code = %v; want PermissionDenied (err=%v)", got, err)
	}

	// Side-effect probe: tenant must remain active.
	tnt, _ := gs.GetTenant(ctx, "acme")
	if tnt.Status != "active" {
		t.Errorf("post-deny status = %q; want %q (no side effect on permission denial)",
			tnt.Status, "active")
	}
}

// TestSetLegalHold_GroupActorPermissionDenied: a group: actor (which
// auth.Authoritative maps to a User with the group: prefix on the
// subject) is rejected — only admin: / system: pass the gate.
func TestSetLegalHold_GroupActorPermissionDenied(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor: "group:legal", TenantId: "acme", Enabled: true,
	})
	if err == nil || errs.Code(err) != codes.PermissionDenied {
		t.Fatalf("group: actor: want PERMISSION_DENIED, got %v", err)
	}
}

// TestSetLegalHold_EmptyActor: empty wire actor surfaces as
// INVALID_ARGUMENT before the auth decision. Spec §"Error contract".
func TestSetLegalHold_EmptyActor(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.SetLegalHold(context.Background(), &pb.LegalHoldRequest{
		Actor: "", TenantId: "acme", Enabled: true,
	})
	if err == nil || errs.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty actor: want INVALID_ARGUMENT, got %v", err)
	}
}

// TestSetLegalHold_EmptyTenantID: empty tenant_id surfaces as
// INVALID_ARGUMENT. Pinned by test_admin_operations.py:860-868.
func TestSetLegalHold_EmptyTenantID(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.SetLegalHold(context.Background(), &pb.LegalHoldRequest{
		Actor: "admin:root", TenantId: "", Enabled: true,
	})
	if err == nil || errs.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty tenant_id: want INVALID_ARGUMENT, got %v", err)
	}
}

// TestSetLegalHold_GlobalStoreNotConfigured: when global_store is
// absent, the handler returns UNIMPLEMENTED before any auth/validation
// (spec §"Auth": ordering preserved).
func TestSetLegalHold_GlobalStoreNotConfigured(t *testing.T) {
	t.Parallel()

	srv := api.New() // no WithGlobalStore

	_, err := srv.SetLegalHold(context.Background(), &pb.LegalHoldRequest{
		Actor: "admin:root", TenantId: "acme", Enabled: true,
	})
	if err == nil || errs.Code(err) != codes.Unimplemented {
		t.Fatalf("missing globalstore: want UNIMPLEMENTED, got %v", err)
	}
}

// TestSetLegalHold_UnknownTenantNotFound: an admin call against a tenant
// not in the registry returns NOT_FOUND (via checkTenant). This is the
// Wave-2 deviation from Python's OK + success=false response — see
// set_legal_hold.go header note.
func TestSetLegalHold_UnknownTenantNotFound(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.SetLegalHold(context.Background(), &pb.LegalHoldRequest{
		Actor: "admin:root", TenantId: "ghost", Enabled: true,
	})
	if err == nil {
		t.Fatalf("ghost tenant: expected NOT_FOUND, got nil")
	}
	if got := errs.Code(err); got != codes.NotFound {
		t.Fatalf("ghost tenant: code = %v; want NotFound (err=%v)", got, err)
	}
}

// TestSetLegalHold_AppendsWALEvent_WhenProducerWired: when the WAL
// producer is wired, the handler appends a `set_legal_hold` op to the
// configured topic. This is the WAL-first restoration the Go port
// adds on top of Python parity (CLAUDE.md invariant #1).
//
// The applier (apply/ops_set_legal_hold.go) consumes the op and writes
// a globalstore.legal_holds row; that side of the contract is covered
// by the applier's own test suite. Here we only assert that the wire
// payload is shaped correctly so the applier can dispatch it.
func TestSetLegalHold_AppendsWALEvent_WhenProducerWired(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	mem := wal.NewInMemory(0)
	if err := mem.Connect(ctx); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}
	t.Cleanup(func() { _ = mem.Close(context.Background()) })

	srv := api.New(api.WithGlobalStore(gs), api.WithWALProducer(mem))

	if _, err := srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor: "admin:root", TenantId: "acme", Enabled: true,
	}); err != nil {
		t.Fatalf("SetLegalHold: %v", err)
	}

	recs := mem.GetAllRecords("entdb-wal")
	if len(recs) != 1 {
		t.Fatalf("WAL records = %d; want 1", len(recs))
	}
	rec := recs[0]
	if rec.Key != "acme" {
		t.Errorf("record.Key = %q; want %q (per-tenant partition)", rec.Key, "acme")
	}

	ev, err := wal.DecodeEvent(rec.Value)
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	if ev.TenantID != "acme" {
		t.Errorf("event.TenantID = %q; want acme", ev.TenantID)
	}
	if ev.Actor != "admin:root" {
		t.Errorf("event.Actor = %q; want admin:root", ev.Actor)
	}
	if len(ev.Ops) != 1 {
		t.Fatalf("event.Ops = %d; want 1", len(ev.Ops))
	}
	op := ev.Ops[0]
	if got, _ := op["op"].(string); got != "set_legal_hold" {
		t.Errorf("op.op = %v; want set_legal_hold", op["op"])
	}
	if got, _ := op["held_by"].(string); got != "admin:root" {
		t.Errorf("op.held_by = %v; want admin:root", op["held_by"])
	}
	// `clear` is the inverse of `enabled`; for enable=true we expect false.
	if got, _ := op["clear"].(bool); got {
		t.Errorf("op.clear = true; want false (enable path)")
	}
	if got, _ := op["reason"].(string); got != "SetLegalHold RPC" {
		t.Errorf("op.reason = %q; want non-empty for enable path", got)
	}

	// Sanity: the JSON shape round-trips to map[string]any (mirrors what
	// the applier sees after wal.DecodeEvent + dispatch).
	raw, _ := json.Marshal(op)
	if len(raw) == 0 {
		t.Errorf("op JSON: empty")
	}
}

// TestSetLegalHold_ClearAppendsWALEventWithClearTrue: clearing the hold
// emits a `set_legal_hold` op with `clear=true`. The applier branches
// on `clear` to call ClearLegalHold instead of SetLegalHold.
func TestSetLegalHold_ClearAppendsWALEventWithClearTrue(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	mem := wal.NewInMemory(0)
	if err := mem.Connect(ctx); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}
	t.Cleanup(func() { _ = mem.Close(context.Background()) })

	srv := api.New(api.WithGlobalStore(gs), api.WithWALProducer(mem))

	if _, err := srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor: "admin:root", TenantId: "acme", Enabled: false,
	}); err != nil {
		t.Fatalf("SetLegalHold(clear): %v", err)
	}

	recs := mem.GetAllRecords("entdb-wal")
	if len(recs) != 1 {
		t.Fatalf("WAL records = %d; want 1", len(recs))
	}
	ev, err := wal.DecodeEvent(recs[0].Value)
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	op := ev.Ops[0]
	if got, _ := op["clear"].(bool); !got {
		t.Errorf("op.clear = false; want true (clear path)")
	}
}

// TestSetLegalHold_NoProducer_FlagStillFlips: when the producer is not
// wired, the synchronous tenant_registry status flip still happens —
// the WAL append is best-effort. This is the "globalstore is the
// authoritative gate today" trade-off documented in the spec.
func TestSetLegalHold_NoProducer_FlagStillFlips(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs)) // no producer

	resp, err := srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor: "admin:root", TenantId: "acme", Enabled: true,
	})
	if err != nil {
		t.Fatalf("SetLegalHold: %v", err)
	}
	if !resp.GetSuccess() || resp.GetStatus() != "legal_hold" {
		t.Errorf("response: success=%v status=%q; want true/legal_hold",
			resp.GetSuccess(), resp.GetStatus())
	}
	tnt, _ := gs.GetTenant(ctx, "acme")
	if tnt.Status != "legal_hold" {
		t.Errorf("registry status = %q; want legal_hold", tnt.Status)
	}
}

// TestSetLegalHold_PrivilegeEscalation_ClaimedActorIgnored: an
// authenticated user who claims `actor: "admin:root"` on the wire is
// rebound by auth.Authoritative to the trusted Identity (here we
// simulate by NOT installing an Identity, so claimed wins as
// documented). When a user: identity is on context, the claim is
// downgraded — pinned by the auth package's Authoritative tests.
//
// This test specifically exercises the documented fallback: no
// Identity on ctx => claim is honoured. It pins that the handler is
// honest about the rebind in deployments without auth.
func TestSetLegalHold_PrivilegeEscalation_NoIdentityFallback(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	srv := api.New(api.WithGlobalStore(gs))

	// No Identity on ctx -> Authoritative falls back to claim. Claim
	// is admin:root => allowed.
	resp, err := srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor: "admin:root", TenantId: "acme", Enabled: true,
	})
	if err != nil {
		t.Fatalf("SetLegalHold(no identity): %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("SetLegalHold: success=false, error=%q", resp.GetError())
	}
}
