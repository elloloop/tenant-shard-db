// Tests for the FreezeUser RPC and the freeze-gate interceptor that
// enforces it. The contract pins enumerated in
// docs/go-port/rpcs/FreezeUser.md ("Contract tests pinning behavior")
// reduce in the Go port to:
//
//   - admin happy path (sets status to "frozen", success=true);
//   - frozen user's mutating RPC → PERMISSION_DENIED via interceptor
//     (the load-bearing test for the freeze gate);
//   - frozen user's read RPC still allowed;
//
// plus the standard pre-check arms (UNIMPLEMENTED, INVALID_ARGUMENT,
// PERMISSION_DENIED for non-admin/non-self, in-band "User not found").

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// TestFreezeUser_Admin_HappyPath: admin freezes alice. Status is
// flipped to "frozen"; response carries success=true and status="frozen".
func TestFreezeUser_Admin_HappyPath(t *testing.T) {
	t.Parallel()
	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := f.srv

	resp, err := srv.FreezeUser(withTrustedUser(ctx, "admin:root"), &pb.FreezeUserRequest{
		Actor:   "admin:root",
		UserId:  "alice",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("FreezeUser: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("FreezeUser: success=false, error=%q", resp.GetError())
	}
	if resp.GetStatus() != "frozen" {
		t.Errorf("FreezeUser: status: got %q, want %q", resp.GetStatus(), "frozen")
	}

	// Verify the row was patched.
	got, err := gs.GetUser(ctx, "alice")
	if err != nil || got == nil {
		t.Fatalf("post-freeze GetUser: user=%v, err=%v", got, err)
	}
	if got.Status != "frozen" {
		t.Errorf("alice.Status: got %q, want %q", got.Status, "frozen")
	}
}

// TestFreezeUser_FrozenUserMutation_DeniedViaInterceptor: the
// load-bearing test for the freeze gate. After alice is frozen, her
// next mutating RPC (driven through the FreezeGateInterceptor) MUST
// abort PERMISSION_DENIED. The interceptor consults
// user_registry.status; FreezeUser only sets the bit.
func TestFreezeUser_FrozenUserMutation_DeniedViaInterceptor(t *testing.T) {
	t.Parallel()
	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := f.srv

	// Step 1: admin freezes alice.
	if _, err := srv.FreezeUser(withTrustedUser(ctx, "admin:root"), &pb.FreezeUserRequest{
		Actor:   "admin:root",
		UserId:  "alice",
		Enabled: true,
	}); err != nil {
		t.Fatalf("seed FreezeUser: %v", err)
	}

	// Step 2: alice attempts a mutating RPC. The FreezeGateInterceptor
	// is the layer that rejects her. Use ExecuteAtomic as the
	// representative mutating method (the canary the spec calls out).
	interceptor := srv.FreezeGateInterceptor()
	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return nil, nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/entdb.v1.EntDBService/ExecuteAtomic"}

	_, err := interceptor(withTrustedUser(ctx, "alice"), nil, info, handler)
	if err == nil {
		t.Fatalf("FreezeGateInterceptor: expected PERMISSION_DENIED, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("FreezeGateInterceptor: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("FreezeGateInterceptor: code: got %v, want PermissionDenied", st.Code())
	}
	if called {
		t.Errorf("FreezeGateInterceptor: handler was invoked despite freeze; gate failed open")
	}
}

// TestFreezeUser_FrozenUserRead_StillAllowed: reads bypass the freeze
// gate. The interceptor MUST admit a frozen user's read RPC (e.g.
// GetNode) so the user can still see her data.
func TestFreezeUser_FrozenUserRead_StillAllowed(t *testing.T) {
	t.Parallel()
	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := f.srv

	// Freeze alice.
	if _, err := srv.FreezeUser(withTrustedUser(ctx, "admin:root"), &pb.FreezeUserRequest{
		Actor:   "admin:root",
		UserId:  "alice",
		Enabled: true,
	}); err != nil {
		t.Fatalf("seed FreezeUser: %v", err)
	}

	// alice attempts a read RPC. The gate must NOT reject it.
	interceptor := srv.FreezeGateInterceptor()
	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/entdb.v1.EntDBService/GetNode"}

	resp, err := interceptor(withTrustedUser(ctx, "alice"), nil, info, handler)
	if err != nil {
		t.Fatalf("FreezeGateInterceptor on read: got error %v, want nil (reads must pass)", err)
	}
	if !called {
		t.Errorf("FreezeGateInterceptor: handler was NOT invoked on a read RPC; gate failed closed")
	}
	if resp != "ok" {
		t.Errorf("FreezeGateInterceptor: response: got %v, want %q", resp, "ok")
	}
}

// TestFreezeUser_Unfreeze_Idempotent: unfreezing returns success=true
// status="active" even when the user is already active — SetUserStatus
// UPDATEs unconditionally and rowcount > 0 iff the row existed.
func TestFreezeUser_Unfreeze_Idempotent(t *testing.T) {
	t.Parallel()
	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := f.srv

	// Unfreeze (alice is already active).
	resp, err := srv.FreezeUser(withTrustedUser(ctx, "admin:root"), &pb.FreezeUserRequest{
		Actor:   "admin:root",
		UserId:  "alice",
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("FreezeUser: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("FreezeUser: success=false, error=%q", resp.GetError())
	}
	if resp.GetStatus() != "active" {
		t.Errorf("FreezeUser: status: got %q, want %q", resp.GetStatus(), "active")
	}
}

// TestFreezeUser_Self_HappyPath: alice can freeze herself (the "self"
// arm of the self-or-admin gate). Spec flags this as debatable for
// GDPR Article 18.
func TestFreezeUser_Self_HappyPath(t *testing.T) {
	t.Parallel()
	f := newAdminWALFixture(t)
	gs := f.gs
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := f.srv

	resp, err := srv.FreezeUser(withTrustedUser(ctx, "alice"), &pb.FreezeUserRequest{
		Actor:   "user:alice",
		UserId:  "alice",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("FreezeUser: %v", err)
	}
	if !resp.GetSuccess() || resp.GetStatus() != "frozen" {
		t.Fatalf("FreezeUser self: got %+v, want success=true status=frozen", resp)
	}
}

// TestFreezeUser_NonAdminOther_Denied: alice trying to freeze bob is
// the privilege-escalation guard. The handler MUST consult ctx for the
// trusted identity and ignore the request payload's claim of
// `admin:root`.
func TestFreezeUser_NonAdminOther_Denied(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "bob", "bob@example.com", "Bob"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	// alice is the trusted caller; she claims admin:root in the wire
	// payload — must be ignored.
	resp, err := srv.FreezeUser(withTrustedUser(ctx, "alice"), &pb.FreezeUserRequest{
		Actor:   "admin:root",
		UserId:  "bob",
		Enabled: true,
	})
	if err == nil {
		t.Fatalf("FreezeUser: expected PERMISSION_DENIED, got resp=%+v", resp)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("FreezeUser: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("FreezeUser: code: got %v, want PermissionDenied", st.Code())
	}

	// Defence in depth: confirm the store was NOT mutated.
	got, _ := gs.GetUser(ctx, "bob")
	if got == nil || got.Status != "active" {
		t.Errorf("post-denied bob.Status: got %v, want unchanged 'active'", got)
	}
}

// TestFreezeUser_NotFound_InBandFailure: freezing a missing user_id
// returns success=false, error="User not found" in-band. No NOT_FOUND
// status code — clients pin the in-band shape.
func TestFreezeUser_NotFound_InBandFailure(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	// Seed a different user so the global store is non-empty but the
	// target row does not exist.
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.FreezeUser(withTrustedUser(ctx, "admin:root"), &pb.FreezeUserRequest{
		Actor:   "admin:root",
		UserId:  "ghost",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("FreezeUser: expected nil err for in-band not-found, got %v", err)
	}
	if resp.GetSuccess() {
		t.Fatalf("FreezeUser: expected success=false, got %+v", resp)
	}
	if resp.GetError() != "User not found" {
		t.Errorf("FreezeUser: error: got %q, want %q", resp.GetError(), "User not found")
	}
	if resp.GetStatus() != "" {
		t.Errorf("FreezeUser: status on not-found: got %q, want empty", resp.GetStatus())
	}
}

// TestFreezeUser_GlobalstoreUnset_Unimplemented: a Server with no
// globalstore wired aborts with codes.Unimplemented.
func TestFreezeUser_GlobalstoreUnset_Unimplemented(t *testing.T) {
	t.Parallel()
	srv := api.New() // no WithGlobalStore

	_, err := srv.FreezeUser(context.Background(), &pb.FreezeUserRequest{
		Actor:   "admin:root",
		UserId:  "alice",
		Enabled: true,
	})
	if err == nil {
		t.Fatalf("FreezeUser: expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("FreezeUser: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.Unimplemented {
		t.Fatalf("FreezeUser: code: got %v, want Unimplemented", st.Code())
	}
}

// TestFreezeUser_EmptyActor_InvalidArgument: actor is required.
func TestFreezeUser_EmptyActor_InvalidArgument(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.FreezeUser(context.Background(), &pb.FreezeUserRequest{
		UserId:  "alice",
		Enabled: true,
	})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("FreezeUser: code: got %v, want InvalidArgument", st.Code())
	}
}

// TestFreezeUser_EmptyUserID_InvalidArgument: user_id is required.
func TestFreezeUser_EmptyUserID_InvalidArgument(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.FreezeUser(context.Background(), &pb.FreezeUserRequest{
		Actor:   "admin:root",
		Enabled: true,
	})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("FreezeUser: code: got %v, want InvalidArgument", st.Code())
	}
}

// TestFreezeGate_PendingDeletion_BlocksMutations: pending_deletion is
// treated identically to frozen by the gate. DeleteUser writes
// pending_deletion; the gate must reject the user's mutating RPCs the
// same way.
func TestFreezeGate_PendingDeletion_BlocksMutations(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}
	if _, err := gs.SetUserStatus(ctx, "alice", "pending_deletion"); err != nil {
		t.Fatalf("seed SetUserStatus: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))
	interceptor := srv.FreezeGateInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/entdb.v1.EntDBService/ExecuteAtomic"}
	handler := func(ctx context.Context, req any) (any, error) { return nil, nil }

	_, err := interceptor(withTrustedUser(ctx, "alice"), nil, info, handler)
	if err == nil {
		t.Fatalf("FreezeGateInterceptor: expected PERMISSION_DENIED for pending_deletion, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.PermissionDenied {
		t.Errorf("FreezeGateInterceptor: code: got %v, want PermissionDenied", st.Code())
	}
}

// TestFreezeGate_AdminCaller_BypassesGate: admin/system callers
// bypass the freeze gate even when (hypothetically) the registry knows
// about them — the gate only consults user-kind identities.
func TestFreezeGate_AdminCaller_BypassesGate(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	interceptor := srv.FreezeGateInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/entdb.v1.EntDBService/ExecuteAtomic"}
	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return nil, nil
	}

	_, err := interceptor(withTrustedUser(context.Background(), "admin:root"), nil, info, handler)
	if err != nil {
		t.Fatalf("FreezeGateInterceptor on admin: got %v, want nil", err)
	}
	if !called {
		t.Error("FreezeGateInterceptor: handler not invoked for admin caller")
	}
}

// guard against the WithIdentity helper drifting away from the auth
// package — keeps the test file's imports honest.
var _ = auth.WithIdentity
