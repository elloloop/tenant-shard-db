// SPDX-License-Identifier: AGPL-3.0-only

// Tests for the DeleteUser RPC. Behaviours pinned (per
// docs/go-port/rpcs/DeleteUser.md "Contract tests pinning behavior"):
//
//   1. Self-deletion happy path — alice queues her own erasure with
//      grace_days=7; status="pending", user.Status="pending_deletion",
//      execute_at = requested_at + 7d.
//   2. Admin happy path — admin:root queues bob's erasure; same
//      assertions modulo the actor.
//   3. Legal-hold gate (NEW Go-port behaviour, opt-in via
//      WithLegalHoldOnDelete(true)) — a user with a held tenant gets
//      FAILED_PRECONDITION.
//   4. Already-pending → idempotent: re-queueing returns the EXISTING
//      requested_at / execute_at (does NOT push the timestamps forward).
//
// Plus the standard config / required-arg gates: Unimplemented when
// globalstore is not wired, InvalidArgument for empty actor / user_id,
// PermissionDenied when a non-admin tries to delete another user, and
// OK + success=false + error="User not found" for an unknown user.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

const secondsPerDay = int64(86400)

// TestDeleteUser_Self_HappyPath: alice queues her own erasure with a
// 7-day grace.
func TestDeleteUser_Self_HappyPath(t *testing.T) {
	t.Parallel()
	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := f.srv

	resp, err := srv.DeleteUser(withTrustedUser(ctx, "alice"), &pb.DeleteUserRequest{
		Actor:     "user:alice",
		UserId:    "alice",
		GraceDays: 7,
	})
	if err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("DeleteUser: success=false, error=%q", resp.GetError())
	}
	if resp.GetStatus() != "pending" {
		t.Errorf("Status: got %q, want %q", resp.GetStatus(), "pending")
	}
	if got, want := resp.GetExecuteAt()-resp.GetRequestedAt(), 7*secondsPerDay; got != want {
		t.Errorf("ExecuteAt-RequestedAt: got %d, want %d", got, want)
	}

	// Queue row was inserted as 'pending'.
	entry, err := gs.GetDeletionEntry(ctx, "alice")
	if err != nil || entry == nil {
		t.Fatalf("post-delete GetDeletionEntry: entry=%v err=%v", entry, err)
	}
	if entry.Status != "pending" {
		t.Errorf("entry.Status: got %q, want %q", entry.Status, "pending")
	}

	// User status flipped.
	got, err := gs.GetUser(ctx, "alice")
	if err != nil || got == nil {
		t.Fatalf("post-delete GetUser: user=%v err=%v", got, err)
	}
	if got.Status != "pending_deletion" {
		t.Errorf("user.Status: got %q, want %q", got.Status, "pending_deletion")
	}
}

// TestDeleteUser_Admin_HappyPath: admin:root queues bob's erasure.
// Default grace (grace_days=0) normalizes to 30 days.
func TestDeleteUser_Admin_HappyPath(t *testing.T) {
	t.Parallel()
	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "bob", "bob@example.com", "Bob"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := f.srv

	resp, err := srv.DeleteUser(withTrustedUser(ctx, "admin:root"), &pb.DeleteUserRequest{
		Actor:  "admin:root",
		UserId: "bob",
		// grace_days unset -> normalized to 30
	})
	if err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("DeleteUser: success=false, error=%q", resp.GetError())
	}
	if resp.GetStatus() != "pending" {
		t.Errorf("Status: got %q, want %q", resp.GetStatus(), "pending")
	}
	if got, want := resp.GetExecuteAt()-resp.GetRequestedAt(), 30*secondsPerDay; got != want {
		t.Errorf("ExecuteAt-RequestedAt (default grace): got %d, want %d", got, want)
	}

	// User status flipped.
	got, err := gs.GetUser(ctx, "bob")
	if err != nil || got == nil {
		t.Fatalf("post-delete GetUser: user=%v err=%v", got, err)
	}
	if got.Status != "pending_deletion" {
		t.Errorf("user.Status: got %q, want %q", got.Status, "pending_deletion")
	}
}

// TestDeleteUser_LegalHold_Blocks: when WithLegalHoldOnDelete(true), a
// user belonging to a tenant with a legal_holds row gets
// FAILED_PRECONDITION. See DeleteUser.md "Side effects" §4.
func TestDeleteUser_LegalHold_Blocks(t *testing.T) {
	t.Parallel()
	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}
	if _, err := gs.CreateTenant(ctx, "t1", "Alice's Tenant", "us-east-1"); err != nil {
		t.Fatalf("seed CreateTenant: %v", err)
	}
	if err := gs.AddTenantMember(ctx, "t1", "alice", "owner"); err != nil {
		t.Fatalf("seed AddTenantMember: %v", err)
	}
	if _, err := gs.SetLegalHold(ctx, "t1", "compliance:legal", "litigation hold"); err != nil {
		t.Fatalf("seed SetLegalHold: %v", err)
	}

	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithLegalHoldOnDelete(true),
	)

	_, err := srv.DeleteUser(withTrustedUser(ctx, "alice"), &pb.DeleteUserRequest{
		Actor:     "user:alice",
		UserId:    "alice",
		GraceDays: 30,
	})
	if err == nil {
		t.Fatalf("DeleteUser: expected FAILED_PRECONDITION, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("DeleteUser: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.FailedPrecondition {
		t.Fatalf("DeleteUser: code: got %v, want FailedPrecondition", st.Code())
	}

	// Defence in depth: no queue row was created.
	entry, _ := gs.GetDeletionEntry(ctx, "alice")
	if entry != nil {
		t.Errorf("expected no deletion_queue row after legal-hold block, got %+v", entry)
	}
	// User status NOT flipped.
	got, _ := gs.GetUser(ctx, "alice")
	if got == nil || got.Status != "active" {
		t.Errorf("user.Status: got %v, want unchanged 'active'", got)
	}
}

// TestDeleteUser_Idempotent_ReturnsExisting: re-queueing returns the
// existing requested_at / execute_at unchanged.
func TestDeleteUser_Idempotent_ReturnsExisting(t *testing.T) {
	t.Parallel()
	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := f.srv

	first, err := srv.DeleteUser(withTrustedUser(ctx, "alice"), &pb.DeleteUserRequest{
		Actor:     "user:alice",
		UserId:    "alice",
		GraceDays: 7,
	})
	if err != nil {
		t.Fatalf("first DeleteUser: %v", err)
	}
	if !first.GetSuccess() {
		t.Fatalf("first DeleteUser: success=false")
	}

	// Re-queue with a *different* grace period — the handler MUST
	// return the original timestamps, not push them forward.
	second, err := srv.DeleteUser(withTrustedUser(ctx, "alice"), &pb.DeleteUserRequest{
		Actor:     "user:alice",
		UserId:    "alice",
		GraceDays: 99,
	})
	if err != nil {
		t.Fatalf("second DeleteUser: %v", err)
	}
	if !second.GetSuccess() {
		t.Fatalf("second DeleteUser: success=false, error=%q", second.GetError())
	}
	if second.GetRequestedAt() != first.GetRequestedAt() {
		t.Errorf("RequestedAt: got %d, want unchanged %d", second.GetRequestedAt(), first.GetRequestedAt())
	}
	if second.GetExecuteAt() != first.GetExecuteAt() {
		t.Errorf("ExecuteAt: got %d, want unchanged %d", second.GetExecuteAt(), first.GetExecuteAt())
	}
	if second.GetStatus() != "pending" {
		t.Errorf("Status: got %q, want %q", second.GetStatus(), "pending")
	}
}

// TestDeleteUser_NonAdmin_OtherDenied: alice tries to delete bob with
// a forged admin:root claim on the wire — the trusted identity wins
// and the request is denied.
func TestDeleteUser_NonAdmin_OtherDenied(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "bob", "bob@example.com", "Bob"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	// alice is the trusted caller; she claims admin:root in the wire
	// payload. The handler MUST ignore the wire claim.
	_, err := srv.DeleteUser(withTrustedUser(ctx, "alice"), &pb.DeleteUserRequest{
		Actor:     "admin:root",
		UserId:    "bob",
		GraceDays: 30,
	})
	if err == nil {
		t.Fatalf("DeleteUser: expected PERMISSION_DENIED, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("DeleteUser: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("DeleteUser: code: got %v, want PermissionDenied", st.Code())
	}

	// No queue row created.
	entry, _ := gs.GetDeletionEntry(ctx, "bob")
	if entry != nil {
		t.Errorf("expected no deletion_queue row after denied call, got %+v", entry)
	}
}

// TestDeleteUser_NotFound_InBandFailure: unknown user_id returns
// success=false, error="User not found" IN-BAND (no NOT_FOUND code).
func TestDeleteUser_NotFound_InBandFailure(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.DeleteUser(withTrustedUser(context.Background(), "admin:root"), &pb.DeleteUserRequest{
		Actor:     "admin:root",
		UserId:    "ghost",
		GraceDays: 30,
	})
	if err != nil {
		t.Fatalf("DeleteUser: expected nil err for in-band not-found, got %v", err)
	}
	if resp.GetSuccess() {
		t.Fatalf("DeleteUser: expected success=false, got %+v", resp)
	}
	if resp.GetError() != "User not found" {
		t.Errorf("DeleteUser: error: got %q, want %q", resp.GetError(), "User not found")
	}
}

// TestDeleteUser_GlobalstoreUnset_Unimplemented: a Server with no
// globalstore wired aborts with codes.Unimplemented.
func TestDeleteUser_GlobalstoreUnset_Unimplemented(t *testing.T) {
	t.Parallel()
	srv := api.New() // no WithGlobalStore

	_, err := srv.DeleteUser(context.Background(), &pb.DeleteUserRequest{
		Actor:  "admin:root",
		UserId: "alice",
	})
	if err == nil {
		t.Fatalf("DeleteUser: expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("DeleteUser: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.Unimplemented {
		t.Fatalf("DeleteUser: code: got %v, want Unimplemented", st.Code())
	}
}

// TestDeleteUser_EmptyActor_InvalidArgument: actor required.
func TestDeleteUser_EmptyActor_InvalidArgument(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.DeleteUser(context.Background(), &pb.DeleteUserRequest{
		UserId: "alice",
	})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("DeleteUser: code: got %v, want InvalidArgument", st.Code())
	}
}

// TestDeleteUser_EmptyUserID_InvalidArgument: user_id required.
func TestDeleteUser_EmptyUserID_InvalidArgument(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.DeleteUser(context.Background(), &pb.DeleteUserRequest{
		Actor: "admin:root",
	})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("DeleteUser: code: got %v, want InvalidArgument", st.Code())
	}
}
