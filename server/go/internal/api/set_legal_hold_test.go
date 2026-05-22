// SPDX-License-Identifier: AGPL-3.0-only

// Tests for the SetLegalHold RPC. Spec: docs/go-port/rpcs/SetLegalHold.md.
//
// Behavioural pins:
//
//   - tests/python/integration/test_grpc_contract.py:573-583 — wire-level sweep.
//
//  deviations (intentional, documented in set_legal_hold.go header):
//
//   - Unknown tenant -> NOT_FOUND (Python: OK + success=false). All
//     mutating RPCs converge on the status-code form.
//   - Owner role bypass deferred — admin-only. Mirrors ArchiveTenant.

package api_test

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// TestSetLegalHold_Release_DurablyEnqueuesLift: an explicit release
// (enabled=false) does NOT spawn any goroutine off the RPC path. It
// drives the durable global apply step which, in the SAME transaction
// that clears the legal_holds row + flips tenant_registry.status, upserts
// a legal_hold_lift_queue row. This is the EPIC #511 Gap 1 crash-durable
// contract: a server restart after the release still finds the pending
// lift recorded.
func TestSetLegalHold_Release_DurablyEnqueuesLift(t *testing.T) {
	t.Parallel()

	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	// Enable first so there is a hold to release; ON must NOT enqueue.
	if _, err := f.srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor: "admin:root", TenantId: "acme", Enabled: true,
	}); err != nil {
		t.Fatalf("SetLegalHold(enable): %v", err)
	}
	if q, err := gs.GetLegalHoldLiftQueue(ctx); err != nil {
		t.Fatalf("GetLegalHoldLiftQueue: %v", err)
	} else if len(q) != 0 {
		t.Fatalf("ON path enqueued %d lift rows; want 0", len(q))
	}

	// Release: the lift must be durably enqueued for "acme".
	resp, err := f.srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor: "admin:root", TenantId: "acme", Enabled: false,
	})
	if err != nil {
		t.Fatalf("SetLegalHold(release): %v", err)
	}
	if !resp.GetSuccess() || resp.GetStatus() != "active" {
		t.Fatalf("release resp: success=%v status=%q", resp.GetSuccess(), resp.GetStatus())
	}
	q, err := gs.GetLegalHoldLiftQueue(ctx)
	if err != nil {
		t.Fatalf("GetLegalHoldLiftQueue: %v", err)
	}
	if len(q) != 1 || q[0].TenantID != "acme" {
		t.Fatalf("lift queue = %+v; want exactly [acme]", q)
	}
	if q[0].EnqueuedAt == 0 {
		t.Fatalf("enqueued_at not set: %+v", q[0])
	}
	// The globalstore hold must be gone in the SAME durable step that
	// enqueued the lift (precedence: release first, then erasure may
	// proceed; the worker's stillHeld lookup sees the post-release state).
	if held, _ := gs.IsLegalHoldSet(ctx, "acme"); held {
		t.Fatal("globalstore hold still set after release; worker would see stale state")
	}
}

// TestSetLegalHold_Release_SpawnsNoGoroutine proves the detached
// fire-and-forget sweep goroutine is GONE. The OFF RPC now just causes
// the durable enqueue (via the apply path) and returns; the lift is done
// by the separate crash-durable worker (internal/audit.LiftWorker), not
// a request-scoped goroutine. We measure the live goroutine count across
// the OFF call: a detached `go func()` sweep would leave it elevated.
func TestSetLegalHold_Release_SpawnsNoGoroutine(t *testing.T) {
	t.Parallel()

	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	// Settle any transient goroutines, then snapshot the baseline.
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	before := runtime.NumGoroutine()

	if _, err := f.srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor: "admin:root", TenantId: "acme", Enabled: false,
	}); err != nil {
		t.Fatalf("SetLegalHold(release): %v", err)
	}

	// A detached sweep goroutine would still be alive here. Allow brief
	// settle for the synchronous applier-driven enqueue to finish.
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	after := runtime.NumGoroutine()
	// Allow tiny scheduler noise but not a leaked long-lived sweep.
	if after > before+1 {
		t.Fatalf("goroutine count grew %d -> %d across the OFF RPC; "+
			"the detached lift goroutine must be gone (lift is now the durable worker)",
			before, after)
	}

	// And the lift is instead recorded durably.
	q, err := gs.GetLegalHoldLiftQueue(ctx)
	if err != nil {
		t.Fatalf("GetLegalHoldLiftQueue: %v", err)
	}
	if len(q) != 1 || q[0].TenantID != "acme" {
		t.Fatalf("lift queue = %+v; want exactly [acme] (durable, not a goroutine)", q)
	}
}

// TestSetLegalHold_Enable_DoesNotEnqueueLift: the ON path must never
// enqueue a lift — legal hold is being ADDED, not lifted.
func TestSetLegalHold_Enable_DoesNotEnqueueLift(t *testing.T) {
	t.Parallel()

	f := newAdminWALFixture(t)
	ctx := context.Background()
	if _, err := f.gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if _, err := f.srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor: "admin:root", TenantId: "acme", Enabled: true,
	}); err != nil {
		t.Fatalf("SetLegalHold(enable): %v", err)
	}
	q, err := f.gs.GetLegalHoldLiftQueue(ctx)
	if err != nil {
		t.Fatalf("GetLegalHoldLiftQueue: %v", err)
	}
	if len(q) != 0 {
		t.Fatalf("ON path enqueued %d lift rows; want 0", len(q))
	}
}

// TestSetLegalHold_Release_ReEnqueueKeepsEarliestAge: re-releasing an
// already-queued tenant (idempotent re-issue) must not reset
// enqueued_at — the age drives operator alerting on a stuck lift.
func TestSetLegalHold_Release_ReEnqueueKeepsEarliestAge(t *testing.T) {
	t.Parallel()

	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	rel := func() {
		if _, err := f.srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
			Actor: "admin:root", TenantId: "acme", Enabled: false,
		}); err != nil {
			t.Fatalf("SetLegalHold(release): %v", err)
		}
	}
	rel()
	q1, err := gs.GetLegalHoldLiftQueue(ctx)
	if err != nil || len(q1) != 1 {
		t.Fatalf("first release queue=%+v err=%v", q1, err)
	}
	first := q1[0].EnqueuedAt
	rel()
	q2, err := gs.GetLegalHoldLiftQueue(ctx)
	if err != nil || len(q2) != 1 {
		t.Fatalf("second release queue=%+v err=%v", q2, err)
	}
	if q2[0].EnqueuedAt != first {
		t.Fatalf("re-release reset enqueued_at: was %d now %d (want unchanged)",
			first, q2[0].EnqueuedAt)
	}
}

// TestSetLegalHold_AdminEnable_HappyPath: admin actor sets the hold; the
// response carries success=true + status="legal_hold" and the
// tenant_registry row is flipped accordingly.
func TestSetLegalHold_AdminEnable_HappyPath(t *testing.T) {
	t.Parallel()

	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := f.srv

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

	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := f.srv

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
// the registry status back at "active".
func TestSetLegalHold_Clear_TogglesBackToActive(t *testing.T) {
	t.Parallel()

	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := f.srv

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
// admin/system prefix) is rejected.
func TestSetLegalHold_NonAdminPermissionDenied(t *testing.T) {
	t.Parallel()

	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := f.srv

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

	f := newAdminWALFixture(t)
	gs := f.gs
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

	f := newAdminWALFixture(t)
	gs := f.gs
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.SetLegalHold(context.Background(), &pb.LegalHoldRequest{
		Actor: "", TenantId: "acme", Enabled: true,
	})
	if err == nil || errs.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty actor: want INVALID_ARGUMENT, got %v", err)
	}
}

// TestSetLegalHold_EmptyTenantID: empty tenant_id surfaces as
// INVALID_ARGUMENT.
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
//
//	deviation from Python's OK + success=false response — see
//
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

	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := f.srv

	if _, err := srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor: "admin:root", TenantId: "acme", Enabled: true,
	}); err != nil {
		t.Fatalf("SetLegalHold: %v", err)
	}

	recs := f.wal.GetAllRecords("entdb-wal")
	if len(recs) != 1 {
		t.Fatalf("WAL records = %d; want 1", len(recs))
	}
	rec := recs[0]
	if rec.Key != wal.GlobalTenantID {
		t.Errorf("record.Key = %q; want %q", rec.Key, wal.GlobalTenantID)
	}

	ev, err := wal.DecodeEvent(rec.Value)
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	if ev.TenantID != wal.GlobalTenantID {
		t.Errorf("event.TenantID = %q; want %s", ev.TenantID, wal.GlobalTenantID)
	}
	if ev.Scope != wal.ScopeGlobal {
		t.Errorf("event.Scope = %q; want %q", ev.Scope, wal.ScopeGlobal)
	}
	if ev.Actor != "admin:root" {
		t.Errorf("event.Actor = %q; want admin:root", ev.Actor)
	}
	if len(ev.Ops) != 1 {
		t.Fatalf("event.Ops = %d; want 1", len(ev.Ops))
	}
	op := ev.Ops[0]
	if got, _ := op["op"].(string); got != "legal_hold_set" {
		t.Errorf("op.op = %v; want legal_hold_set", op["op"])
	}
	if got, _ := op["tenant_id"].(string); got != "acme" {
		t.Errorf("op.tenant_id = %v; want acme", op["tenant_id"])
	}
	if got, _ := op["held_by"].(string); got != "admin:root" {
		t.Errorf("op.held_by = %v; want admin:root", op["held_by"])
	}
	if got, _ := op["enabled"].(bool); !got {
		t.Errorf("op.enabled = false; want true")
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

	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := f.srv

	if _, err := srv.SetLegalHold(ctx, &pb.LegalHoldRequest{
		Actor: "admin:root", TenantId: "acme", Enabled: false,
	}); err != nil {
		t.Fatalf("SetLegalHold(clear): %v", err)
	}

	recs := f.wal.GetAllRecords("entdb-wal")
	if len(recs) != 1 {
		t.Fatalf("WAL records = %d; want 1", len(recs))
	}
	ev, err := wal.DecodeEvent(recs[0].Value)
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	op := ev.Ops[0]
	if got, _ := op["enabled"].(bool); got {
		t.Errorf("op.enabled = true; want false (clear path)")
	}
}

// TestSetLegalHold_NoProducer_Unimplemented: global admin mutations now
// require a durable WAL producer.
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
	if err == nil {
		t.Fatalf("SetLegalHold: expected error, got resp=%+v", resp)
	}
	if got := errs.Code(err); got != codes.Unimplemented {
		t.Fatalf("SetLegalHold: code = %v; want Unimplemented", got)
	}
	tnt, _ := gs.GetTenant(ctx, "acme")
	if tnt.Status != "active" {
		t.Errorf("registry status = %q; want active", tnt.Status)
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

	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	srv := f.srv

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
