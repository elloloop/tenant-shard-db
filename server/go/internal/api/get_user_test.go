// Tests for the GetUser RPC.
//
// Spec: docs/go-port/rpcs/GetUser.md. Three branches pinned here, one
// per contract test referenced by the spec:
//
//  1. Happy round-trip: an existing user -> found=true with
//     UserInfo populated (mirrors test_grpc_contract.py:396-400).
//
//  2. Missing user: unknown user_id -> found=false, no error
//     (mirrors test_grpc_contract.py:401-406). DO NOT upgrade to
//     NOT_FOUND.
//
//  3. Empty actor: -> codes.InvalidArgument (mirrors
//     test_grpc_contract.py:407-411).
//
// The shared globalstore helper lives in helpers_external_test.go;
// do NOT redeclare newGlobalStore here.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// TestGetUser_HappyRoundTrip seeds a user via globalstore.CreateUser
// (the same chokepoint the CreateUser RPC will write through) and
// asserts the GetUser response matches it field-for-field.
func TestGetUser_HappyRoundTrip(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()

	created, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.GetUser(ctx, &pb.GetUserRequest{
		Actor:  "user:alice",
		UserId: "alice",
	})
	if err != nil {
		t.Fatalf("GetUser: unexpected err: %v", err)
	}
	if resp == nil {
		t.Fatalf("GetUser: nil response")
	}
	if !resp.GetFound() {
		t.Fatalf("GetUser: found: got false, want true")
	}
	got := resp.GetUser()
	if got == nil {
		t.Fatalf("GetUser: user is nil despite found=true")
	}
	if got.GetUserId() != "alice" {
		t.Fatalf("GetUser: user_id: got %q, want %q", got.GetUserId(), "alice")
	}
	if got.GetEmail() != "alice@example.com" {
		t.Fatalf("GetUser: email: got %q, want %q", got.GetEmail(), "alice@example.com")
	}
	if got.GetName() != "Alice" {
		t.Fatalf("GetUser: name: got %q, want %q", got.GetName(), "Alice")
	}
	if got.GetStatus() != "active" {
		t.Fatalf("GetUser: status: got %q, want %q", got.GetStatus(), "active")
	}
	if got.GetCreatedAt() != created.CreatedAt {
		t.Fatalf("GetUser: created_at: got %d, want %d", got.GetCreatedAt(), created.CreatedAt)
	}
	if got.GetUpdatedAt() != created.UpdatedAt {
		t.Fatalf("GetUser: updated_at: got %d, want %d", got.GetUpdatedAt(), created.UpdatedAt)
	}
}

// TestGetUser_MissingUser pins the in-band absence signal: an unknown
// user_id returns found=false with codes.OK, NOT codes.NotFound. This
// is the deliberate asymmetry called out in the spec — clients branch
// on response.Found, so upgrading to a NOT_FOUND status code would be
// a contract break.
func TestGetUser_MissingUser(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.GetUser(context.Background(), &pb.GetUserRequest{
		Actor:  "user:alice",
		UserId: "ghost",
	})
	if err != nil {
		t.Fatalf("GetUser: unexpected err for missing user: %v", err)
	}
	if resp == nil {
		t.Fatalf("GetUser: nil response for missing user")
	}
	if resp.GetFound() {
		t.Fatalf("GetUser: found: got true, want false for missing user")
	}
	if resp.GetUser() != nil {
		t.Fatalf("GetUser: user populated despite found=false: %+v", resp.GetUser())
	}
}

// TestGetUser_EmptyActor pins the argument-validation branch: an
// empty wire actor produces codes.InvalidArgument. The wire field is
// UNTRUSTED for auth decisions, but the contract test suite — which
// runs without an auth interceptor — relies on this guard to drive
// the validation branch.
func TestGetUser_EmptyActor(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.GetUser(context.Background(), &pb.GetUserRequest{
		Actor:  "",
		UserId: "alice",
	})
	if err == nil {
		t.Fatalf("GetUser: expected error for empty actor, got resp=%+v", resp)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("GetUser: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("GetUser: code: got %v, want InvalidArgument", st.Code())
	}
}
