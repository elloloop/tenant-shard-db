// Tests for the CancelUserDeletion RPC. The four Python contract pins
// in docs/go-port/rpcs/CancelUserDeletion.md ("Contract tests pinning
// behavior") collapse to three behaviours in the Go port — happy
// within-grace cancel, past-no-return silent no-op, and the
// trusted-actor escalation guard. Each test below covers one arm.
//
// The auth helper withTrustedUser is defined in update_user_test.go
// (same package api_test) — DRY across the two GDPR RPCs.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// TestCancelUserDeletion_Self_HappyPath: alice cancels her own pending
// deletion within the grace window. Mirrors test_gdpr_engine.py:489-495
// (store-level pin) and test_grpc_contract.py:676-681 (wire pin).
//
// Verifies BOTH legs of the handler chain:
//  1. CancelDeletion removes the deletion_queue row (GetDeletionEntry
//     returns nil after).
//  2. SetUserStatus flips user_registry.status back to "active".
func TestCancelUserDeletion_Self_HappyPath(t *testing.T) {
	t.Parallel()
	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}
	// Simulate a prior DeleteUser: queue + flip status. The Python
	// DeleteUser handler does these two writes (grpc_server.py:2925-2981);
	// we reproduce them at the store layer here so this test does not
	// depend on the not-yet-ported DeleteUser RPC.
	if _, err := gs.QueueDeletion(ctx, "alice", 30); err != nil {
		t.Fatalf("seed QueueDeletion: %v", err)
	}
	if _, err := gs.SetUserStatus(ctx, "alice", "pending_deletion"); err != nil {
		t.Fatalf("seed SetUserStatus: %v", err)
	}

	srv := f.srv

	resp, err := srv.CancelUserDeletion(withTrustedUser(ctx, "alice"), &pb.CancelUserDeletionRequest{
		Actor:  "user:alice",
		UserId: "alice",
	})
	if err != nil {
		t.Fatalf("CancelUserDeletion: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("CancelUserDeletion: success=false, error=%q", resp.GetError())
	}

	// Step 1 verification: deletion_queue row gone.
	entry, err := gs.GetDeletionEntry(ctx, "alice")
	if err != nil {
		t.Fatalf("GetDeletionEntry: %v", err)
	}
	if entry != nil {
		t.Errorf("expected deletion_queue row removed, got %+v", entry)
	}

	// Step 2 verification: user_registry.status flipped to active.
	got, err := gs.GetUser(ctx, "alice")
	if err != nil || got == nil {
		t.Fatalf("post-cancel GetUser: user=%v, err=%v", got, err)
	}
	if got.Status != "active" {
		t.Errorf("Status: got %q, want %q", got.Status, "active")
	}
}

// TestCancelUserDeletion_PastNoReturn_SilentNoOp: when the GDPR worker
// has already swept the row to status='completed', the cancel is a
// no-op. The handler MUST surface this as success=false, error="" —
// NOT FAILED_PRECONDITION. This is the parity contract from the spec
// "Error contract" table (already-completed arm) and the Open
// Questions note. The user_registry.status MUST remain untouched (the
// step-2 flip is conditional on step 1).
//
// Flagged in the PR body as a v2 follow-up: a future proto change
// should distinguish "already executed" from "never queued" via either
// a distinct enum or FAILED_PRECONDITION + a `cancelled` tombstone.
func TestCancelUserDeletion_PastNoReturn_SilentNoOp(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}
	if _, err := gs.QueueDeletion(ctx, "alice", 0); err != nil {
		t.Fatalf("seed QueueDeletion: %v", err)
	}
	if _, err := gs.SetUserStatus(ctx, "alice", "pending_deletion"); err != nil {
		t.Fatalf("seed SetUserStatus: %v", err)
	}
	// Sweeper has run — flip pending → completed. Past point of no
	// return.
	if _, err := gs.MarkDeletionCompleted(ctx, "alice"); err != nil {
		t.Fatalf("seed MarkDeletionCompleted: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.CancelUserDeletion(withTrustedUser(ctx, "alice"), &pb.CancelUserDeletionRequest{
		Actor:  "user:alice",
		UserId: "alice",
	})
	// CRITICAL: NO grpc status error. Past-no-return is in-band.
	if err != nil {
		t.Fatalf("CancelUserDeletion: expected nil err for past-no-return, got %v", err)
	}
	if resp.GetSuccess() {
		t.Fatalf("CancelUserDeletion: expected success=false, got %+v", resp)
	}
	if resp.GetError() != "" {
		t.Errorf("CancelUserDeletion: error: got %q, want empty (parity v2 follow-up)", resp.GetError())
	}

	// Defence in depth: status flip MUST NOT fire when step 1 reported
	// nothing-to-cancel.
	got, err := gs.GetUser(ctx, "alice")
	if err != nil || got == nil {
		t.Fatalf("post-noop GetUser: user=%v, err=%v", got, err)
	}
	if got.Status != "pending_deletion" {
		t.Errorf("Status: got %q, want unchanged %q", got.Status, "pending_deletion")
	}

	// And the completed deletion_queue row was NOT touched.
	entry, err := gs.GetDeletionEntry(ctx, "alice")
	if err != nil {
		t.Fatalf("GetDeletionEntry: %v", err)
	}
	if entry == nil || entry.Status != "completed" {
		t.Errorf("deletion_queue: got %+v, want status=completed retained", entry)
	}
}

// TestCancelUserDeletion_NonAdmin_OtherDenied: alice trying to cancel
// bob's pending deletion is the trusted-actor escalation guard. The
// handler MUST consult ctx (not the wire `actor`) for the privilege
// decision — post-#168 invariant. Mirrors the auth gate at
// grpc_server.py:2997-3001.
func TestCancelUserDeletion_NonAdmin_OtherDenied(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "bob", "bob@example.com", "Bob"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}
	if _, err := gs.QueueDeletion(ctx, "bob", 30); err != nil {
		t.Fatalf("seed QueueDeletion: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	// alice is the trusted caller. She forges admin:root in the wire
	// payload — must be ignored.
	resp, err := srv.CancelUserDeletion(withTrustedUser(ctx, "alice"), &pb.CancelUserDeletionRequest{
		Actor:  "admin:root",
		UserId: "bob",
	})
	if err == nil {
		t.Fatalf("CancelUserDeletion: expected PERMISSION_DENIED, got resp=%+v", resp)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("CancelUserDeletion: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("CancelUserDeletion: code: got %v, want PermissionDenied", st.Code())
	}

	// Defence in depth: bob's deletion_queue row MUST still be pending.
	entry, err := gs.GetDeletionEntry(ctx, "bob")
	if err != nil {
		t.Fatalf("GetDeletionEntry: %v", err)
	}
	if entry == nil || entry.Status != "pending" {
		t.Errorf("deletion_queue: got %+v, want pending row retained", entry)
	}
}
