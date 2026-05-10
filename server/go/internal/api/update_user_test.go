// Tests for the UpdateUser RPC. The seven Python contract pins enumerated
// in docs/go-port/rpcs/UpdateUser.md ("Contract tests pinning behavior")
// reduce to five behaviours in the Go port — happy self-update, happy
// admin-other, non-admin-other denied, no-fields in-band failure, and
// not-found in-band failure. Each test below covers one of those arms.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// withTrustedUser stamps a verified user identity onto ctx so the
// handler's auth.Authoritative call returns the trusted Actor — the
// only way a unit test can drive the post-#168 trusted-actor pattern.
func withTrustedUser(ctx context.Context, subject string) context.Context {
	return auth.WithIdentity(ctx, auth.Identity{
		Method:  auth.MethodOAuth,
		Subject: subject,
	})
}

// TestUpdateUser_Self_HappyPath: alice updates her own row (only
// `name` set; truthy-only gating means `email` and `status` are not
// forwarded). Mirrors test_user_registry.py:255-271 and the contract
// pin at test_grpc_contract.py:448-453.
func TestUpdateUser_Self_HappyPath(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.UpdateUser(withTrustedUser(ctx, "alice"), &pb.UpdateUserRequest{
		Actor:  "user:alice",
		UserId: "alice",
		Name:   "Alice Updated",
	})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("UpdateUser: success=false, error=%q", resp.GetError())
	}

	// Verify the row was patched and untouched fields kept their
	// values — proves truthy-only gating.
	got, err := gs.GetUser(ctx, "alice")
	if err != nil || got == nil {
		t.Fatalf("post-update GetUser: user=%v, err=%v", got, err)
	}
	if got.Name != "Alice Updated" {
		t.Errorf("Name: got %q, want %q", got.Name, "Alice Updated")
	}
	if got.Email != "alice@example.com" {
		t.Errorf("Email: got %q, want unchanged %q", got.Email, "alice@example.com")
	}
}

// TestUpdateUser_Admin_UpdatesOther: an admin: trusted identity can
// update any user. Only `status` is set (truthy-only gating again).
// Mirrors test_user_registry.py:273-290.
func TestUpdateUser_Admin_UpdatesOther(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "bob", "bob@example.com", "Bob"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.UpdateUser(withTrustedUser(ctx, "admin:root"), &pb.UpdateUserRequest{
		Actor:  "admin:root",
		UserId: "bob",
		Status: "suspended",
	})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("UpdateUser: success=false, error=%q", resp.GetError())
	}

	got, err := gs.GetUser(ctx, "bob")
	if err != nil || got == nil {
		t.Fatalf("post-update GetUser: user=%v, err=%v", got, err)
	}
	if got.Status != "suspended" {
		t.Errorf("Status: got %q, want %q", got.Status, "suspended")
	}
}

// TestUpdateUser_NonAdmin_OtherDenied: alice trying to update bob is
// the privilege-escalation guard. The handler MUST consult ctx for
// the trusted identity and ignore the request payload's claim of
// `admin:root`. Mirrors test_user_registry.py:292-309 and the contract
// pin at test_grpc_contract.py:454-458.
func TestUpdateUser_NonAdmin_OtherDenied(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "bob", "bob@example.com", "Bob"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	// alice is the trusted caller. She tries to claim admin:root in
	// the wire payload — must be ignored.
	resp, err := srv.UpdateUser(withTrustedUser(ctx, "alice"), &pb.UpdateUserRequest{
		Actor:  "admin:root",
		UserId: "bob",
		Status: "suspended",
	})
	if err == nil {
		t.Fatalf("UpdateUser: expected PERMISSION_DENIED, got resp=%+v", resp)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("UpdateUser: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("UpdateUser: code: got %v, want PermissionDenied", st.Code())
	}

	// Defence in depth: confirm the store was NOT mutated.
	got, _ := gs.GetUser(ctx, "bob")
	if got == nil || got.Status != "active" {
		t.Errorf("post-denied bob.Status: got %v, want unchanged 'active'", got)
	}
}

// TestUpdateUser_NoFields_InBandFailure: omitting all three mutable
// fields short-circuits to success=false, error="No fields to update"
// IN-BAND — no codes.InvalidArgument abort. Mirrors
// test_user_registry.py:330-345.
func TestUpdateUser_NoFields_InBandFailure(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.UpdateUser(withTrustedUser(ctx, "alice"), &pb.UpdateUserRequest{
		Actor:  "user:alice",
		UserId: "alice",
		// email/name/status all empty
	})
	if err != nil {
		t.Fatalf("UpdateUser: expected nil err for in-band no-fields failure, got %v", err)
	}
	if resp.GetSuccess() {
		t.Fatalf("UpdateUser: expected success=false, got %+v", resp)
	}
	if resp.GetError() != "No fields to update" {
		t.Errorf("UpdateUser: error: got %q, want %q", resp.GetError(), "No fields to update")
	}
}

// TestUpdateUser_NotFound_InBandFailure: trying to update a missing
// user_id returns success=false, error="User not found" in-band. No
// NOT_FOUND status code — mirrors test_user_registry.py:311-328.
func TestUpdateUser_NotFound_InBandFailure(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	// Seed a different user so the global store is non-empty but the
	// target row does not exist.
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.UpdateUser(withTrustedUser(ctx, "admin:root"), &pb.UpdateUserRequest{
		Actor:  "admin:root",
		UserId: "ghost",
		Name:   "Phantom",
	})
	if err != nil {
		t.Fatalf("UpdateUser: expected nil err for in-band not-found, got %v", err)
	}
	if resp.GetSuccess() {
		t.Fatalf("UpdateUser: expected success=false, got %+v", resp)
	}
	if resp.GetError() != "User not found" {
		t.Errorf("UpdateUser: error: got %q, want %q", resp.GetError(), "User not found")
	}
}

// TestUpdateUser_GlobalstoreUnset_Unimplemented: a Server with no
// globalstore wired aborts with codes.Unimplemented. Mirrors the
// configuration gate at grpc_server.py:2192-2196.
func TestUpdateUser_GlobalstoreUnset_Unimplemented(t *testing.T) {
	t.Parallel()
	srv := api.New() // no WithGlobalStore

	_, err := srv.UpdateUser(context.Background(), &pb.UpdateUserRequest{
		Actor:  "admin:root",
		UserId: "alice",
		Name:   "x",
	})
	if err == nil {
		t.Fatalf("UpdateUser: expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("UpdateUser: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.Unimplemented {
		t.Fatalf("UpdateUser: code: got %v, want Unimplemented", st.Code())
	}
}

// TestUpdateUser_EmptyActor_InvalidArgument: actor required.
func TestUpdateUser_EmptyActor_InvalidArgument(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.UpdateUser(context.Background(), &pb.UpdateUserRequest{
		UserId: "alice",
		Name:   "x",
	})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("UpdateUser: code: got %v, want InvalidArgument", st.Code())
	}
}

// TestUpdateUser_EmptyUserID_InvalidArgument: user_id required.
func TestUpdateUser_EmptyUserID_InvalidArgument(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.UpdateUser(context.Background(), &pb.UpdateUserRequest{
		Actor: "admin:root",
		Name:  "x",
	})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("UpdateUser: code: got %v, want InvalidArgument", st.Code())
	}
}
