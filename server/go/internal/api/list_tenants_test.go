// Tests for the ListTenants RPC. Pinned by:
//   - tests/python/unit/test_listtenants_auth.py:99-258 (admin/user
//     visibility, nil identity → PERMISSION_DENIED, sharding still
//     applies, no globalstore → empty for users).
//   - tests/python/integration/test_privilege_escalation.py:386-417
//     (claimed admin actor on the wire MUST NOT bypass membership
//     filtering).
//   - tests/python/integration/test_grpc_contract.py:288-295
//     (over-the-wire empty request without auth interceptor →
//     PERMISSION_DENIED).
//
// The Go port maps these onto auth.WithIdentity(ctx, ...) instead of
// the Python ContextVar, but the visibility classes and error contract
// are identical.

package api_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/tenant"
)

// withIdentitySubject is a thin helper: wrap ctx with a verified
// identity carrying the given prefix-encoded subject ("user:alice",
// "system:admin", etc). Mirrors what the auth interceptor does in
// production.
func withIdentitySubject(ctx context.Context, subject string) context.Context {
	return auth.WithIdentity(ctx, auth.Identity{
		Method:  auth.MethodOAuth,
		Subject: subject,
	})
}

// tenantIDs flattens a *ListTenantsResponse into the stable string
// slice that test asserts compare against.
func tenantIDs(resp *pb.ListTenantsResponse) []string {
	if resp == nil {
		return nil
	}
	out := make([]string, 0, len(resp.GetTenants()))
	for _, t := range resp.GetTenants() {
		out = append(out, t.GetTenantId())
	}
	return out
}

// TestListTenants_Admin_SeesAll: every admin-class identity
// ("__system__", "system:*", "admin:*") sees every tenant on the
// node. Pinned by test_listtenants_auth.py:99-111.
func TestListTenants_Admin_SeesAll(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()

	for _, name := range []string{"acme", "beta", "carrot"} {
		if _, err := gs.CreateTenant(ctx, name, name, "us-east-1"); err != nil {
			t.Fatalf("CreateTenant(%q): %v", name, err)
		}
	}

	srv := api.New(api.WithGlobalStore(gs))

	cases := []string{
		"__system__",
		"system:admin",
		"system:gdpr-worker",
		"admin:root",
	}
	for _, subject := range cases {
		t.Run(subject, func(t *testing.T) {
			actx := withIdentitySubject(ctx, subject)
			resp, err := srv.ListTenants(actx, &pb.ListTenantsRequest{})
			if err != nil {
				t.Fatalf("ListTenants: unexpected err: %v", err)
			}
			got := tenantIDs(resp)
			want := []string{"acme", "beta", "carrot"}
			if len(got) != len(want) {
				t.Fatalf("tenants: got %v, want %v", got, want)
			}
			for i, g := range got {
				if g != want[i] {
					t.Fatalf("tenants[%d]: got %q, want %q (full %v)", i, g, want[i], got)
				}
			}
		})
	}
}

// TestListTenants_RegularUser_SeesOwnOnly: a "user:<id>" caller sees
// only the tenants they are a member of. Non-member tenants stay
// invisible — no enumeration leak. Pinned by
// test_listtenants_auth.py:119-137.
func TestListTenants_RegularUser_SeesOwnOnly(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()

	for _, name := range []string{"acme", "beta", "carrot"} {
		if _, err := gs.CreateTenant(ctx, name, name, "us-east-1"); err != nil {
			t.Fatalf("CreateTenant(%q): %v", name, err)
		}
	}
	if err := gs.AddTenantMember(ctx, "acme", "alice", "member"); err != nil {
		t.Fatalf("AddTenantMember(acme,alice): %v", err)
	}
	if err := gs.AddTenantMember(ctx, "carrot", "alice", "member"); err != nil {
		t.Fatalf("AddTenantMember(carrot,alice): %v", err)
	}
	if err := gs.AddTenantMember(ctx, "beta", "bob", "member"); err != nil {
		t.Fatalf("AddTenantMember(beta,bob): %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	actx := withIdentitySubject(ctx, "user:alice")
	resp, err := srv.ListTenants(actx, &pb.ListTenantsRequest{})
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}
	got := tenantIDs(resp)
	want := []string{"acme", "carrot"}
	if len(got) != len(want) {
		t.Fatalf("tenants: got %v, want %v", got, want)
	}
	for i, g := range got {
		if g != want[i] {
			t.Fatalf("tenants[%d]: got %q, want %q (full %v)", i, g, want[i], got)
		}
	}
}

// TestListTenants_RegularUser_NoMemberships_Empty: a user with zero
// memberships sees an empty list — no enumeration leak. Pinned by
// test_listtenants_auth.py:140-154.
func TestListTenants_RegularUser_NoMemberships_Empty(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()

	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	actx := withIdentitySubject(ctx, "user:eve")
	resp, err := srv.ListTenants(actx, &pb.ListTenantsRequest{})
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}
	if got := tenantIDs(resp); len(got) != 0 {
		t.Fatalf("tenants: got %v, want []", got)
	}
}

// TestListTenants_UserPrefix_Stripped: the "user:" prefix is stripped
// before the membership lookup, so a session subject "user:alice" and
// a bare-id subject "alice" both resolve to alice's memberships.
// Pinned by test_listtenants_auth.py:157-171.
func TestListTenants_UserPrefix_Stripped(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()

	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := gs.AddTenantMember(ctx, "acme", "alice", "member"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	prefixed, err := srv.ListTenants(withIdentitySubject(ctx, "user:alice"), &pb.ListTenantsRequest{})
	if err != nil {
		t.Fatalf("prefixed: %v", err)
	}
	bare, err := srv.ListTenants(withIdentitySubject(ctx, "alice"), &pb.ListTenantsRequest{})
	if err != nil {
		t.Fatalf("bare: %v", err)
	}
	if got := tenantIDs(prefixed); len(got) != 1 || got[0] != "acme" {
		t.Fatalf("prefixed: got %v, want [acme]", got)
	}
	if got := tenantIDs(bare); len(got) != 1 || got[0] != "acme" {
		t.Fatalf("bare: got %v, want [acme]", got)
	}
}

// TestListTenants_NoIdentity_PermissionDenied: with no trusted
// identity on the context (interceptor missing), the handler MUST
// return PERMISSION_DENIED and NOT fall open. Pinned by
// test_listtenants_auth.py:179-192 and test_grpc_contract.py:288-295.
func TestListTenants_NoIdentity_PermissionDenied(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()

	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	// Bare context — no auth.WithIdentity attached.
	resp, err := srv.ListTenants(ctx, &pb.ListTenantsRequest{})
	if err == nil {
		t.Fatalf("expected error, got resp=%+v", resp)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("err is not a grpc status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("code: got %v, want PermissionDenied", st.Code())
	}
}

// TestListTenants_EmptyIdentity_PermissionDenied: an Identity
// attached to ctx but with no Subject is treated the same as no
// identity at all (IdentityFromContext returns ok=false for the
// zero value). The handler MUST return PERMISSION_DENIED.
func TestListTenants_EmptyIdentity_PermissionDenied(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := api.New(api.WithGlobalStore(gs))

	ctx := auth.WithIdentity(context.Background(), auth.Identity{})
	resp, err := srv.ListTenants(ctx, &pb.ListTenantsRequest{})
	if err == nil {
		t.Fatalf("expected error, got resp=%+v", resp)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("err is not a grpc status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("code: got %v, want PermissionDenied", st.Code())
	}
}

// TestListTenants_NoGlobalStore_RegularUser_Empty: in the embedded
// harness (no globalstore wired), a regular user sees the empty
// list rather than the unfiltered inventory. Pinned by
// test_listtenants_auth.py:238-258.
func TestListTenants_NoGlobalStore_RegularUser_Empty(t *testing.T) {
	t.Parallel()
	srv := api.New() // no WithGlobalStore.

	ctx := withIdentitySubject(context.Background(), "user:alice")
	resp, err := srv.ListTenants(ctx, &pb.ListTenantsRequest{})
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}
	if got := tenantIDs(resp); len(got) != 0 {
		t.Fatalf("tenants: got %v, want []", got)
	}
}

// TestListTenants_Sharding_AppliesToAdmin: even an admin caller is
// filtered by the node's sharding view. Tenants the node does not
// own are stripped from the response. Pinned by
// test_listtenants_auth.py:200-230.
func TestListTenants_Sharding_AppliesToAdmin(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()

	for _, name := range []string{"acme", "beta", "carrot"} {
		if _, err := gs.CreateTenant(ctx, name, name, "us-east-1"); err != nil {
			t.Fatalf("CreateTenant(%q): %v", name, err)
		}
	}

	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithSharding(&tenant.Sharding{
			NodeID: "node-a",
			IsMine: func(tid string) bool { return tid == "acme" || tid == "carrot" },
		}),
	)

	actx := withIdentitySubject(ctx, "system:admin")
	resp, err := srv.ListTenants(actx, &pb.ListTenantsRequest{})
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}
	got := tenantIDs(resp)
	want := []string{"acme", "carrot"}
	if len(got) != len(want) {
		t.Fatalf("tenants: got %v, want %v", got, want)
	}
	for i, g := range got {
		if g != want[i] {
			t.Fatalf("tenants[%d]: got %q, want %q", i, g, want[i])
		}
	}
}

// TestListTenants_PrivilegeEscalation_BodyClaimIgnored: the request
// payload has no actor field, so there's nothing for a malicious
// caller to spoof at this RPC. The handler MUST derive visibility
// purely from the trusted Identity on ctx, even if a sibling RPC
// idiom would have looked at a request-context actor. Mirrors
// test_privilege_escalation.py:386-417 (eve cannot claim admin and
// see everything).
func TestListTenants_PrivilegeEscalation_BodyClaimIgnored(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()

	for _, name := range []string{"acme", "beta", "carrot"} {
		if _, err := gs.CreateTenant(ctx, name, name, "us-east-1"); err != nil {
			t.Fatalf("CreateTenant(%q): %v", name, err)
		}
	}
	if err := gs.AddTenantMember(ctx, "beta", "eve", "member"); err != nil {
		t.Fatalf("AddTenantMember(beta,eve): %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	// Trusted subject is user:eve — eve cannot escalate to admin via
	// any wire path. ListTenantsRequest is empty; there is no actor
	// field to spoof.
	actx := withIdentitySubject(ctx, "user:eve")
	resp, err := srv.ListTenants(actx, &pb.ListTenantsRequest{})
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}
	got := tenantIDs(resp)
	want := []string{"beta"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("tenants: got %v, want %v", got, want)
	}
}

// TestListTenants_EmptyResponse_Wire: the response has a non-nil
// empty Tenants slice. protobuf-go marshals nil and []*TenantInfo{}
// identically, but the explicit empty slice is the documented Go
// shape (see GetMailbox.go for the same pattern).
func TestListTenants_EmptyResponse_Wire(t *testing.T) {
	t.Parallel()
	srv := api.New() // no globalstore — admin still gets [] because the
	// inventory is empty.

	ctx := withIdentitySubject(context.Background(), "system:admin")
	resp, err := srv.ListTenants(ctx, &pb.ListTenantsRequest{})
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}
	if resp == nil {
		t.Fatalf("nil response")
	}
	if resp.GetTenants() == nil {
		t.Fatalf("Tenants slice should be non-nil even when empty")
	}
	if len(resp.GetTenants()) != 0 {
		t.Fatalf("expected empty tenants, got %v", tenantIDs(resp))
	}
}

// TestListTenants_GlobalStoreError_SwallowToEmpty: an internal error
// from globalstore.GetUserTenants MUST be swallowed to an empty,
// OK-coded response — matches Python's `except Exception` at
// grpc_server.py:1600-1603. Surfacing codes.Internal is a contract
// break.
//
// We trigger the error by closing the globalstore before the RPC so
// the underlying *sql.DB returns "database is closed" on every query.
func TestListTenants_GlobalStoreError_SwallowToEmpty(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()

	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs))

	// Close the underlying store to force every subsequent query to
	// error. The Cleanup in newGlobalStore is idempotent (uses _ =
	// gs.Close()).
	if err := gs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	actx := withIdentitySubject(ctx, "user:alice")
	resp, err := srv.ListTenants(actx, &pb.ListTenantsRequest{})
	if err != nil {
		t.Fatalf("ListTenants: expected swallow-to-OK, got err=%v", err)
	}
	if resp == nil {
		t.Fatalf("expected non-nil empty response")
	}
	if got := tenantIDs(resp); len(got) != 0 {
		t.Fatalf("tenants: got %v, want []", got)
	}
}
