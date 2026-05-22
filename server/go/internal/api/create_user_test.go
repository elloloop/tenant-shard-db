// Tests for the CreateUser RPC. Spec: docs/go-port/rpcs/CreateUser.md.
//
// Behavioural pins:
//
//   - Happy path with actor="system:admin".
//   - Non-admin caller returns PERMISSION_DENIED and the store is NOT touched.
//   - Duplicate user surfaces as a uniqueness collision.
//   - Empty required field aborts with INVALID_ARGUMENT.
//
// The Go port deliberately upgrades the duplicate-key path to
// codes.AlreadyExists. The spec calls this out as an acceptable
// deviation — the upgraded code is a strict superset of information.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// TestCreateUser_HappyPath_AdminActor pins the canonical success case:
// an admin caller creates a user and the response carries the populated
// UserInfo with status="active" plus non-zero timestamps.
func TestCreateUser_HappyPath_AdminActor(t *testing.T) {
	t.Parallel()

	f := newAdminWALFixture(t)
	srv := f.srv

	resp, err := srv.CreateUser(context.Background(), &pb.CreateUserRequest{
		Actor:  "admin:root",
		UserId: "alice",
		Email:  "alice@example.com",
		Name:   "Alice",
	})
	if err != nil {
		t.Fatalf("CreateUser: unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("CreateUser: response is nil")
	}
	if !resp.GetSuccess() {
		t.Errorf("Success: want true, got false (error=%q)", resp.GetError())
	}
	u := resp.GetUser()
	if u == nil {
		t.Fatalf("User: want non-nil UserInfo")
	}
	if u.GetUserId() != "alice" {
		t.Errorf("UserId: want %q, got %q", "alice", u.GetUserId())
	}
	if u.GetEmail() != "alice@example.com" {
		t.Errorf("Email: want %q, got %q", "alice@example.com", u.GetEmail())
	}
	if u.GetName() != "Alice" {
		t.Errorf("Name: want %q, got %q", "Alice", u.GetName())
	}
	if u.GetStatus() != "active" {
		t.Errorf("Status: want %q, got %q", "active", u.GetStatus())
	}
	if u.GetCreatedAt() == 0 {
		t.Errorf("CreatedAt: want non-zero unix-seconds value")
	}
	if u.GetUpdatedAt() == 0 {
		t.Errorf("UpdatedAt: want non-zero unix-seconds value")
	}
}

// TestCreateUser_HappyPath_SystemActor pins that system:* trusted actors
// are equally privileged.
func TestCreateUser_HappyPath_SystemActor(t *testing.T) {
	t.Parallel()

	f := newAdminWALFixture(t)
	srv := f.srv

	resp, err := srv.CreateUser(context.Background(), &pb.CreateUserRequest{
		Actor:  "system:bootstrap",
		UserId: "bob",
		Email:  "bob@example.com",
		Name:   "Bob",
	})
	if err != nil {
		t.Fatalf("CreateUser: unexpected error: %v", err)
	}
	if !resp.GetSuccess() || resp.GetUser().GetUserId() != "bob" {
		t.Fatalf("CreateUser: unexpected response: %+v", resp)
	}
}

// TestCreateUser_DuplicateUserID pins the AlreadyExists path. The second
// call collides on the user_id PRIMARY KEY; globalstore detects this via
// the modernc.org/sqlite driver sentinel and surfaces an
// errs.ErrAlreadyExists which the handler propagates verbatim.
func TestCreateUser_DuplicateUserID(t *testing.T) {
	t.Parallel()

	f := newAdminWALFixture(t)
	srv := f.srv
	ctx := context.Background()

	if _, err := srv.CreateUser(ctx, &pb.CreateUserRequest{
		Actor:  "admin:root",
		UserId: "alice",
		Email:  "alice@example.com",
		Name:   "Alice",
	}); err != nil {
		t.Fatalf("CreateUser (first): unexpected error: %v", err)
	}

	_, err := srv.CreateUser(ctx, &pb.CreateUserRequest{
		Actor:  "admin:root",
		UserId: "alice",
		Email:  "alice2@example.com",
		Name:   "Alice the Second",
	})
	if err == nil {
		t.Fatalf("CreateUser (duplicate): expected ALREADY_EXISTS, got nil")
	}
	if got := errs.Code(err); got != codes.AlreadyExists {
		t.Fatalf("CreateUser (duplicate): code = %v, want AlreadyExists (err=%v)", got, err)
	}
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	t.Parallel()

	f := newAdminWALFixture(t)
	srv := f.srv
	ctx := context.Background()

	if _, err := srv.CreateUser(ctx, &pb.CreateUserRequest{
		Actor:  "admin:root",
		UserId: "alice",
		Email:  "alice@example.com",
		Name:   "Alice",
	}); err != nil {
		t.Fatalf("CreateUser (first): unexpected error: %v", err)
	}

	_, err := srv.CreateUser(ctx, &pb.CreateUserRequest{
		Actor:  "admin:root",
		UserId: "alice2",
		Email:  "alice@example.com",
		Name:   "Alice the Second",
	})
	if err == nil {
		t.Fatalf("CreateUser (duplicate email): expected ALREADY_EXISTS, got nil")
	}
	if got := errs.Code(err); got != codes.AlreadyExists {
		t.Fatalf("CreateUser (duplicate email): code = %v, want AlreadyExists (err=%v)", got, err)
	}
}

// TestCreateUser_NonAdminActor_PermissionDenied pins the privilege gate.
// Here we cover the simpler path where the wire itself does not claim
// admin (no interceptor on ctx, so claimed passes through Authoritative
// unchanged).
func TestCreateUser_NonAdminActor_PermissionDenied(t *testing.T) {
	t.Parallel()

	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	_, err := srv.CreateUser(context.Background(), &pb.CreateUserRequest{
		Actor:  "user:eve",
		UserId: "alice",
		Email:  "alice@example.com",
		Name:   "Alice",
	})
	if err == nil {
		t.Fatalf("CreateUser: expected PERMISSION_DENIED, got nil")
	}
	if got := errs.Code(err); got != codes.PermissionDenied {
		t.Fatalf("CreateUser: code = %v, want PermissionDenied (err=%v)", got, err)
	}

	// And nothing was written to the store: the row does not exist.
	got, gerr := gs.GetUser(context.Background(), "alice")
	if gerr != nil {
		t.Fatalf("GetUser: %v", gerr)
	}
	if got != nil {
		t.Errorf("GetUser: unexpected row %+v — handler must not write on PERMISSION_DENIED", got)
	}
}

// TestCreateUser_EmptyRequiredField pins validation. Each empty
// non-optional field aborts with INVALID_ARGUMENT before the store is
// touched.
func TestCreateUser_EmptyRequiredField(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		req  *pb.CreateUserRequest
	}{
		{
			name: "empty_actor",
			req:  &pb.CreateUserRequest{UserId: "alice", Email: "a@x", Name: "A"},
		},
		{
			name: "empty_user_id",
			req:  &pb.CreateUserRequest{Actor: "admin:root", Email: "a@x", Name: "A"},
		},
		{
			name: "empty_email",
			req:  &pb.CreateUserRequest{Actor: "admin:root", UserId: "alice", Name: "A"},
		},
		{
			name: "empty_name",
			req:  &pb.CreateUserRequest{Actor: "admin:root", UserId: "alice", Email: "a@x"},
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			gs := newGlobalStore(t)
			srv := api.New(api.WithGlobalStore(gs))

			_, err := srv.CreateUser(context.Background(), c.req)
			if err == nil {
				t.Fatalf("CreateUser: expected INVALID_ARGUMENT, got nil")
			}
			if got := errs.Code(err); got != codes.InvalidArgument {
				t.Fatalf("CreateUser: code = %v, want InvalidArgument (err=%v)", got, err)
			}
		})
	}
}

// TestCreateUser_NoGlobalStore pins the UNIMPLEMENTED guard. A Server
// constructed without WithGlobalStore must reject CreateUser before
// touching auth or validation.
func TestCreateUser_NoGlobalStore(t *testing.T) {
	t.Parallel()

	srv := api.New()
	_, err := srv.CreateUser(context.Background(), &pb.CreateUserRequest{
		Actor:  "admin:root",
		UserId: "alice",
		Email:  "alice@example.com",
		Name:   "Alice",
	})
	if err == nil {
		t.Fatalf("CreateUser: expected UNIMPLEMENTED, got nil")
	}
	if got := errs.Code(err); got != codes.Unimplemented {
		t.Fatalf("CreateUser: code = %v, want Unimplemented (err=%v)", got, err)
	}
}
