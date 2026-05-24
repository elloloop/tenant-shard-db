// Behavioural tests for ListUsers. Spec is docs/go-port/rpcs/ListUsers.md.
//
// The four cases below cover the contract shape:
//
//  1. Empty registry -> users=[], OK.
//  2. Multi-row round-trip -> proto fields preserved, ordered by
//     created_at (the underlying globalstore.ListUsers contract).
//  3. Default limit=100 + status="active" applied when the request
//     omits them.
//  4. Internal globalstore error -> codes.OK with users=[] (silent
//     swallow for parity).
//
// Wart-tracking: cases 1 and 2 also exercise the "no admin scope"
// behaviour by passing a plain user:<id> actor; today this is allowed.
// A follow-up ticket should gate on a users.list capability and tighten
// the test to expect PERMISSION_DENIED for non-admin callers.

package api_test

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// TestListUsers_EmptyRegistry: a fresh globalstore with no user_registry
// rows returns ListUsersResponse{users: []} and codes.OK. The Users
// slice MUST be non-nil (non-nil repeated-field default).
func TestListUsers_EmptyRegistry(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.ListUsers(context.Background(), &pb.ListUsersRequest{
		Actor: "user:u1",
	})
	if err != nil {
		t.Fatalf("ListUsers: unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("ListUsers: response is nil")
	}
	if resp.Users == nil {
		t.Fatalf("ListUsers: Users is nil; want empty slice")
	}
	if len(resp.Users) != 0 {
		t.Fatalf("ListUsers: len(Users) = %d; want 0", len(resp.Users))
	}
}

// TestListUsers_RoundTripsMultipleUsers: rows seeded via
// globalstore.CreateUser come back through the gRPC handler with their
// fields preserved (user_id, email, name, status). Order is
// created_at-ASC per the SQL in globalstore.ListUsers.
func TestListUsers_RoundTripsMultipleUsers(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	for _, u := range []struct{ id, email, name string }{
		{"alice", "alice@example.com", "Alice"},
		{"bob", "bob@example.com", "Bob"},
		{"carol", "carol@example.com", "Carol"},
	} {
		if _, err := gs.CreateUser(ctx, u.id, u.email, u.name); err != nil {
			t.Fatalf("CreateUser(%q): %v", u.id, err)
		}
	}

	srv := api.New(api.WithGlobalStore(gs))

	resp, err := srv.ListUsers(ctx, &pb.ListUsersRequest{
		Actor: "system:admin",
	})
	if err != nil {
		t.Fatalf("ListUsers: unexpected error: %v", err)
	}
	if got := len(resp.GetUsers()); got != 3 {
		t.Fatalf("ListUsers: got %d users, want 3", got)
	}
	wantIDs := map[string]struct {
		email, name string
	}{
		"alice": {"alice@example.com", "Alice"},
		"bob":   {"bob@example.com", "Bob"},
		"carol": {"carol@example.com", "Carol"},
	}
	for _, u := range resp.GetUsers() {
		want, ok := wantIDs[u.GetUserId()]
		if !ok {
			t.Errorf("ListUsers: unexpected user_id %q", u.GetUserId())
			continue
		}
		if u.GetEmail() != want.email {
			t.Errorf("ListUsers[%s]: email = %q, want %q",
				u.GetUserId(), u.GetEmail(), want.email)
		}
		if u.GetName() != want.name {
			t.Errorf("ListUsers[%s]: name = %q, want %q",
				u.GetUserId(), u.GetName(), want.name)
		}
		if u.GetStatus() != "active" {
			t.Errorf("ListUsers[%s]: status = %q, want %q",
				u.GetUserId(), u.GetStatus(), "active")
		}
		if u.GetCreatedAt() == 0 {
			t.Errorf("ListUsers[%s]: created_at = 0; want non-zero (CreateUser stamps it)",
				u.GetUserId())
		}
	}
}

// TestListUsers_DefaultStatusApplied: when the request omits status,
// the handler coerces to status="active". We pin the behaviour by
// seeding a "deleted" row alongside an "active" one and asserting the
// deleted row is filtered out by the default-status branch.
func TestListUsers_DefaultStatusApplied(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice"); err != nil {
		t.Fatalf("CreateUser(alice): %v", err)
	}
	if _, err := gs.CreateUser(ctx, "ghost", "ghost@example.com", "Ghost"); err != nil {
		t.Fatalf("CreateUser(ghost): %v", err)
	}
	if _, err := gs.SetUserStatus(ctx, "ghost", "deleted"); err != nil {
		t.Fatalf("SetUserStatus(ghost, deleted): %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	// No status -> default: status="active".
	resp, err := srv.ListUsers(ctx, &pb.ListUsersRequest{Actor: "user:u1"})
	if err != nil {
		t.Fatalf("ListUsers: unexpected error: %v", err)
	}
	if got := len(resp.GetUsers()); got != 1 {
		t.Fatalf("ListUsers: got %d users, want 1 (active default filters out 'deleted')", got)
	}
	if id := resp.GetUsers()[0].GetUserId(); id != "alice" {
		t.Fatalf("ListUsers: user_id = %q, want \"alice\"", id)
	}

	// Explicit status="deleted" should now surface ghost.
	resp, err = srv.ListUsers(ctx, &pb.ListUsersRequest{
		Actor:  "user:u1",
		Status: "deleted",
	})
	if err != nil {
		t.Fatalf("ListUsers(status=deleted): unexpected error: %v", err)
	}
	if got := len(resp.GetUsers()); got != 1 {
		t.Fatalf("ListUsers(status=deleted): got %d users, want 1", got)
	}
	if id := resp.GetUsers()[0].GetUserId(); id != "ghost" {
		t.Fatalf("ListUsers(status=deleted): user_id = %q, want \"ghost\"", id)
	}
}

// TestListUsers_DefaultLimitApplied pins the limit=0 -> 100 coercion.
// We seed 101 active users and assert the default-limit response caps
// at 100; a follow-up explicit limit=200 confirms the cap was the
// default, not a hard ceiling.
func TestListUsers_DefaultLimitApplied(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	ctx := context.Background()
	const seeded = 101
	for i := 0; i < seeded; i++ {
		id := userIDFromIndex(i)
		if _, err := gs.CreateUser(ctx, id, id+"@example.com", id); err != nil {
			t.Fatalf("CreateUser(%q): %v", id, err)
		}
	}

	srv := api.New(api.WithGlobalStore(gs))

	// limit unset (zero) -> default 100.
	resp, err := srv.ListUsers(ctx, &pb.ListUsersRequest{Actor: "user:u1"})
	if err != nil {
		t.Fatalf("ListUsers: unexpected error: %v", err)
	}
	if got := len(resp.GetUsers()); got != 100 {
		t.Fatalf("ListUsers: got %d users, want 100 (default limit)", got)
	}

	// Explicit larger limit returns all rows — proves no hidden cap and
	// that the previous result was bounded by the default, not the data.
	resp, err = srv.ListUsers(ctx, &pb.ListUsersRequest{
		Actor: "user:u1",
		Limit: 200,
	})
	if err != nil {
		t.Fatalf("ListUsers(limit=200): unexpected error: %v", err)
	}
	if got := len(resp.GetUsers()); got != seeded {
		t.Fatalf("ListUsers(limit=200): got %d users, want %d", got, seeded)
	}
}

// userIDFromIndex returns a stable, lex-sortable id like "u000". Three
// digits are enough for the 101-row test below and keep the IDs short
// in failure messages.
func userIDFromIndex(i int) string {
	const digits = "0123456789"
	b := []byte{'u', '0', '0', '0'}
	b[1] = digits[(i/100)%10]
	b[2] = digits[(i/10)%10]
	b[3] = digits[i%10]
	return string(b)
}

// TestListUsers_EmptyActorInvalidArgument pins the wire-validation
// branch: actor=="" -> codes.InvalidArgument. Contract pin:
// tests/python/integration/test_grpc_contract.py:423-427.
func TestListUsers_EmptyActorInvalidArgument(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.ListUsers(context.Background(), &pb.ListUsersRequest{Actor: ""})
	if err == nil {
		t.Fatalf("ListUsers: expected INVALID_ARGUMENT, got nil")
	}
	if got := errs.Code(err); got != codes.InvalidArgument {
		t.Fatalf("ListUsers: code = %v, want InvalidArgument (err=%v)", got, err)
	}
}

// TestListUsers_GlobalStoreUnconfigured pins the UNIMPLEMENTED branch:
// when the server boots without a globalstore handle wired, ListUsers
// MUST fail with codes.Unimplemented and the message
// "User registry not configured".
func TestListUsers_GlobalStoreUnconfigured(t *testing.T) {
	t.Parallel()

	srv := api.New() // no WithGlobalStore — global == nil.

	_, err := srv.ListUsers(context.Background(), &pb.ListUsersRequest{
		Actor: "user:u1",
	})
	if err == nil {
		t.Fatalf("ListUsers: expected UNIMPLEMENTED, got nil")
	}
	if got := errs.Code(err); got != codes.Unimplemented {
		t.Fatalf("ListUsers: code = %v, want Unimplemented (err=%v)", got, err)
	}
}

// TestListUsers_InternalErrorSurfaced pins the #573 fix: a genuine
// internal error from globalstore must surface as a non-OK status, not
// be masked as an empty list with codes.OK (which hid DB outages and
// silently broke pagination clients). We force the error by closing the
// underlying SQLite handle before calling.
func TestListUsers_InternalErrorSurfaced(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	// Force every subsequent query to fail with "database is closed".
	if err := gs.Close(); err != nil {
		t.Fatalf("gs.Close: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.ListUsers(context.Background(), &pb.ListUsersRequest{
		Actor: "user:u1",
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("ListUsers on broken store: code = %s, want Internal (%v)", status.Code(err), err)
	}
}

// TestListUsers_KeysetPagesAllUsers pins the ADR-029 cursor (#580): the
// user registry pages through completely with page_size + page_token,
// returning every active user exactly once.
func TestListUsers_KeysetPagesAllUsers(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	const n = 25
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("u%03d", i)
		if _, err := gs.CreateUser(ctx, id, id+"@x", id); err != nil {
			t.Fatalf("CreateUser(%s): %v", id, err)
		}
	}
	srv := api.New(api.WithGlobalStore(gs))

	seen := map[string]bool{}
	token := ""
	for pages := 0; ; pages++ {
		if pages > 1000 {
			t.Fatal("pagination did not terminate")
		}
		resp, err := srv.ListUsers(ctx, &pb.ListUsersRequest{
			Actor: "user:admin", PageSize: 10, PageToken: token,
		})
		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		for _, u := range resp.GetUsers() {
			if seen[u.GetUserId()] {
				t.Fatalf("user %s returned on more than one page", u.GetUserId())
			}
			seen[u.GetUserId()] = true
		}
		token = resp.GetNextPageToken()
		if token == "" {
			break
		}
	}
	if len(seen) != n {
		t.Fatalf("paged %d users, want %d — silent truncation (Bug A class)", len(seen), n)
	}
}

// TestListUsers_KeysetRejectsCrossFilterToken pins that a token minted for one
// status filter is rejected against another.
func TestListUsers_KeysetRejectsCrossFilterToken(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("u%03d", i)
		if _, err := gs.CreateUser(ctx, id, id+"@x", id); err != nil {
			t.Fatalf("CreateUser(%s): %v", id, err)
		}
	}
	srv := api.New(api.WithGlobalStore(gs))
	first, err := srv.ListUsers(ctx, &pb.ListUsersRequest{Actor: "user:admin", Status: "active", PageSize: 5})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if first.GetNextPageToken() == "" {
		t.Fatal("expected a next_page_token")
	}
	_, err = srv.ListUsers(ctx, &pb.ListUsersRequest{
		Actor: "user:admin", Status: "deleted", PageSize: 5, PageToken: first.GetNextPageToken(),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("cross-filter token: code = %s, want InvalidArgument (%v)", status.Code(err), err)
	}
}
