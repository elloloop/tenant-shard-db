// Unit tests for the globalstore package. Each table covers a CRUD
// round-trip plus the parity-critical edge cases (UNIQUE violations,
// soft-delete semantics, calendar-month rollover, MaxOpenConns(1)).
//
// All tests use a tmpdir-backed global.db; modernc.org/sqlite is
// CGO-free so this works on every CI runner without preinstalling
// SQLite.

package globalstore_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"google.golang.org/grpc/codes"
)

// newStore opens a tmpdir-backed store. nowFn is optional; nil falls
// back to the production clock.
func newStore(t *testing.T, nowFn func() int64) *globalstore.GlobalStore {
	t.Helper()
	dir := t.TempDir()
	gs, err := globalstore.New(globalstore.Options{
		DataDir: dir,
		WALMode: true,
		NowFn:   nowFn,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })
	if got := filepath.Base(gs.Path()); got != "global.db" {
		t.Fatalf("unexpected db basename: %s", got)
	}
	return gs
}

// TestMaxOpenConns is the parity-critical assertion: the underlying
// pool must be pinned to exactly one connection. Catches a regression
// where someone bumps SetMaxOpenConns thinking SQLite "scales".
func TestMaxOpenConns(t *testing.T) {
	gs := newStore(t, nil)
	stats := gs.DB().Stats()
	if stats.MaxOpenConnections != 1 {
		t.Fatalf("MaxOpenConnections: got %d, want 1", stats.MaxOpenConnections)
	}
}

func TestUserCRUD(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()

	u, err := gs.CreateUser(ctx, "alice", "alice@example.com", "Alice")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.Status != "active" {
		t.Fatalf("status: got %q, want active", u.Status)
	}

	got, err := gs.GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got == nil || got.Email != "alice@example.com" {
		t.Fatalf("GetUser returned %+v", got)
	}

	// Update a field.
	newName := "Alice A."
	ok, err := gs.UpdateUser(ctx, "alice", globalstore.UserUpdates{Name: &newName})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if !ok {
		t.Fatalf("UpdateUser returned false")
	}
	got, _ = gs.GetUser(ctx, "alice")
	if got.Name != "Alice A." {
		t.Fatalf("Name not updated: %q", got.Name)
	}

	// List active users.
	users, err := gs.ListUsers(ctx, "active", 10, 0)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("ListUsers active: got %d, want 1", len(users))
	}

	// Soft delete and confirm Active list excludes.
	if _, err := gs.DeleteUser(ctx, "alice"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	users, _ = gs.ListUsers(ctx, "active", 10, 0)
	if len(users) != 0 {
		t.Fatalf("ListUsers post-delete active: got %d, want 0", len(users))
	}
	users, _ = gs.ListUsers(ctx, "deleted", 10, 0)
	if len(users) != 1 {
		t.Fatalf("ListUsers deleted: got %d, want 1", len(users))
	}

	// GetUser on missing returns (nil, nil).
	got, err = gs.GetUser(ctx, "ghost")
	if err != nil || got != nil {
		t.Fatalf("GetUser ghost: got (%v, %v)", got, err)
	}
}

// TestUserUniqueEmail asserts that a duplicate user_id or email raises
// errs.ErrAlreadyExists (codes.AlreadyExists) — the gRPC layer relies on
// this rather than parsing SQLite errors.
func TestUserUniqueEmail(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()
	if _, err := gs.CreateUser(ctx, "alice", "a@x.com", "A"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// Same user_id.
	_, err := gs.CreateUser(ctx, "alice", "b@x.com", "A2")
	if errs.Code(err) != codes.AlreadyExists {
		t.Fatalf("dup user_id: code=%v err=%v", errs.Code(err), err)
	}
	// Same email, different user_id.
	_, err = gs.CreateUser(ctx, "alice2", "a@x.com", "A2")
	if errs.Code(err) != codes.AlreadyExists {
		t.Fatalf("dup email: code=%v err=%v", errs.Code(err), err)
	}
}

func TestTenantCRUD(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()

	tn, err := gs.CreateTenant(ctx, "acme", "Acme Corp", "")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if tn.Region != globalstore.DefaultRegion {
		t.Fatalf("default region: got %q, want %q", tn.Region, globalstore.DefaultRegion)
	}

	tn2, err := gs.CreateTenant(ctx, "globex", "Globex", "eu-west-1")
	if err != nil {
		t.Fatalf("CreateTenant eu: %v", err)
	}
	if tn2.Region != "eu-west-1" {
		t.Fatalf("region: got %q, want eu-west-1", tn2.Region)
	}

	got, err := gs.GetTenant(ctx, "acme")
	if err != nil || got == nil || got.Status != "active" {
		t.Fatalf("GetTenant: %+v err=%v", got, err)
	}

	tenants, err := gs.ListTenants(ctx, "active")
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}
	if len(tenants) != 2 {
		t.Fatalf("ListTenants: got %d, want 2", len(tenants))
	}

	if _, err := gs.ArchiveTenant(ctx, "acme"); err != nil {
		t.Fatalf("ArchiveTenant: %v", err)
	}
	tenants, _ = gs.ListTenants(ctx, "active")
	if len(tenants) != 1 {
		t.Fatalf("ListTenants post-archive: got %d, want 1", len(tenants))
	}
	tenants, _ = gs.ListTenants(ctx, "archived")
	if len(tenants) != 1 {
		t.Fatalf("ListTenants archived: got %d, want 1", len(tenants))
	}
}

// TestTenantIdempotentCreate documents the Python contract: a duplicate
// CreateTenant raises an integrity error which the gRPC layer
// translates to ALREADY_EXISTS. We surface that directly via
// errs.ErrAlreadyExists.
func TestTenantIdempotentCreate(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	_, err := gs.CreateTenant(ctx, "acme", "Acme2", "")
	if errs.Code(err) != codes.AlreadyExists {
		t.Fatalf("dup tenant: code=%v err=%v", errs.Code(err), err)
	}
}

// TestLegalHoldStatus toggles tenant_registry.status between active and
// legal_hold.
func TestLegalHoldStatus(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "A", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if _, err := gs.SetLegalHoldStatus(ctx, "acme", true); err != nil {
		t.Fatalf("SetLegalHoldStatus(true): %v", err)
	}
	got, _ := gs.GetTenant(ctx, "acme")
	if got.Status != "legal_hold" {
		t.Fatalf("status: got %q, want legal_hold", got.Status)
	}
	if _, err := gs.SetLegalHoldStatus(ctx, "acme", false); err != nil {
		t.Fatalf("SetLegalHoldStatus(false): %v", err)
	}
	got, _ = gs.GetTenant(ctx, "acme")
	if got.Status != "active" {
		t.Fatalf("status: got %q, want active", got.Status)
	}
}

func TestMembersCRUD(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()
	must(t, "create tenant", func() error { _, e := gs.CreateTenant(ctx, "acme", "A", ""); return e })
	must(t, "create user", func() error { _, e := gs.CreateUser(ctx, "alice", "a@x.com", "A"); return e })
	must(t, "add member", func() error { return gs.AddTenantMember(ctx, "acme", "alice", "owner") })

	// Composite-key uniqueness: second AddTenantMember for the same
	// (tenant_id, user_id) MUST fail with ALREADY_EXISTS.
	err := gs.AddTenantMember(ctx, "acme", "alice", "member")
	if errs.Code(err) != codes.AlreadyExists {
		t.Fatalf("dup member: code=%v err=%v", errs.Code(err), err)
	}

	members, err := gs.GetTenantMembers(ctx, "acme")
	if err != nil {
		t.Fatalf("GetTenantMembers: %v", err)
	}
	if len(members) != 1 || members[0].Role != "owner" {
		t.Fatalf("GetTenantMembers: %+v", members)
	}

	tenants, err := gs.GetUserTenants(ctx, "alice")
	if err != nil || len(tenants) != 1 {
		t.Fatalf("GetUserTenants: %+v err=%v", tenants, err)
	}

	if ok, _ := gs.ChangeMemberRole(ctx, "acme", "alice", "admin"); !ok {
		t.Fatalf("ChangeMemberRole: returned false")
	}
	members, _ = gs.GetTenantMembers(ctx, "acme")
	if members[0].Role != "admin" {
		t.Fatalf("role after ChangeMemberRole: %q", members[0].Role)
	}

	if isMember, _ := gs.IsMember(ctx, "acme", "alice"); !isMember {
		t.Fatalf("IsMember(alice): got false")
	}
	if isMember, _ := gs.IsMember(ctx, "acme", "ghost"); isMember {
		t.Fatalf("IsMember(ghost): got true")
	}

	if ok, _ := gs.RemoveTenantMember(ctx, "acme", "alice"); !ok {
		t.Fatalf("RemoveTenantMember: returned false")
	}
}

func TestSharedIndex(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()
	if err := gs.AddShared(ctx, "alice", "acme", "n1", "read"); err != nil {
		t.Fatalf("AddShared: %v", err)
	}
	// Re-share with a new permission — INSERT OR REPLACE.
	if err := gs.AddShared(ctx, "alice", "acme", "n1", "write"); err != nil {
		t.Fatalf("AddShared replace: %v", err)
	}
	rows, err := gs.ListSharedToUser(ctx, "alice", 10, 0)
	if err != nil || len(rows) != 1 || rows[0].Permission != "write" {
		t.Fatalf("ListSharedToUser: %+v err=%v", rows, err)
	}

	rows, _ = gs.ListSharedFromNode(ctx, "acme", "n1")
	if len(rows) != 1 {
		t.Fatalf("ListSharedFromNode: %+v", rows)
	}

	if n, _ := gs.CleanupStaleShared(ctx, "acme", "n1"); n != 1 {
		t.Fatalf("CleanupStaleShared: %d", n)
	}

	if err := gs.AddShared(ctx, "alice", "acme", "n2", "read"); err != nil {
		t.Fatalf("AddShared n2: %v", err)
	}
	if n, _ := gs.RemoveAllSharedForUser(ctx, "alice"); n != 1 {
		t.Fatalf("RemoveAllSharedForUser: %d", n)
	}
}

// TestRevokeUserAccess covers the atomicity invariant: the membership
// delete and shared_index cleanup must succeed or fail together.
func TestRevokeUserAccess(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()
	must(t, "create tenant", func() error { _, e := gs.CreateTenant(ctx, "acme", "A", ""); return e })
	must(t, "add member", func() error { return gs.AddTenantMember(ctx, "acme", "alice", "member") })
	must(t, "add shared", func() error { return gs.AddShared(ctx, "alice", "acme", "n1", "read") })
	must(t, "add unrelated shared", func() error { return gs.AddShared(ctx, "alice", "globex", "n9", "read") })

	res, err := gs.RevokeUserAccess(ctx, "acme", "alice")
	if err != nil {
		t.Fatalf("RevokeUserAccess: %v", err)
	}
	if !res.MembershipRemoved || res.SharedRemoved != 1 {
		t.Fatalf("RevokeUserAccess result: %+v", res)
	}
	rows, _ := gs.ListSharedToUser(ctx, "alice", 10, 0)
	if len(rows) != 1 || rows[0].SourceTenant != "globex" {
		t.Fatalf("unrelated shared was clobbered: %+v", rows)
	}
}

func TestTransferUserContent(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()
	must(t, "create tenant", func() error { _, e := gs.CreateTenant(ctx, "acme", "A", ""); return e })
	must(t, "add from", func() error { return gs.AddTenantMember(ctx, "acme", "alice", "owner") })

	res, err := gs.TransferUserContent(ctx, "acme", "alice", "bob")
	if err != nil {
		t.Fatalf("TransferUserContent: %v", err)
	}
	if !res.MembershipCreated {
		t.Fatalf("expected membership_created=true")
	}
	// Second call: bob already a member, no-op insert.
	res, err = gs.TransferUserContent(ctx, "acme", "alice", "bob")
	if err != nil {
		t.Fatalf("TransferUserContent (idempotent): %v", err)
	}
	if res.MembershipCreated {
		t.Fatalf("expected membership_created=false on second call")
	}
}

func TestDeletionQueue(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()

	d, err := gs.QueueDeletion(ctx, "alice", 30)
	if err != nil {
		t.Fatalf("QueueDeletion: %v", err)
	}
	if d.ExecuteAt-d.RequestedAt != 30*86400 {
		t.Fatalf("execute_at gap: got %d, want %d", d.ExecuteAt-d.RequestedAt, 30*86400)
	}

	// Duplicate => ALREADY_EXISTS.
	_, err = gs.QueueDeletion(ctx, "alice", 30)
	if errs.Code(err) != codes.AlreadyExists {
		t.Fatalf("dup queue: code=%v err=%v", errs.Code(err), err)
	}

	pending, _ := gs.GetDeletionQueue(ctx)
	if len(pending) != 1 {
		t.Fatalf("pending: %+v", pending)
	}

	// Far future: not executable now.
	exec, _ := gs.GetExecutableDeletions(ctx, 0)
	if len(exec) != 0 {
		t.Fatalf("executable: %+v", exec)
	}
	// Force it executable by passing a now beyond execute_at.
	exec, _ = gs.GetExecutableDeletions(ctx, d.ExecuteAt+1)
	if len(exec) != 1 {
		t.Fatalf("executable forced: %+v", exec)
	}

	if ok, _ := gs.MarkDeletionCompleted(ctx, "alice"); !ok {
		t.Fatalf("MarkDeletionCompleted: false")
	}
	got, _ := gs.GetDeletionEntry(ctx, "alice")
	if got.Status != "completed" {
		t.Fatalf("status: %q", got.Status)
	}

	// Cancel deletes only pending; completed row is immutable.
	if ok, _ := gs.CancelDeletion(ctx, "alice"); ok {
		t.Fatalf("CancelDeletion of completed row: returned true")
	}
}

func TestLegalHolds(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()
	if _, err := gs.SetLegalHold(ctx, "acme", "fbi", "court order #1"); err != nil {
		t.Fatalf("SetLegalHold: %v", err)
	}
	// Idempotent — second call doesn't error.
	if _, err := gs.SetLegalHold(ctx, "acme", "fbi", "court order #1"); err != nil {
		t.Fatalf("SetLegalHold idempotent: %v", err)
	}
	holds, _ := gs.GetLegalHold(ctx, "acme")
	if len(holds) != 1 {
		t.Fatalf("GetLegalHold: %+v", holds)
	}
	if set, _ := gs.IsLegalHoldSet(ctx, "acme"); !set {
		t.Fatalf("IsLegalHoldSet: false")
	}
	if ok, _ := gs.ClearLegalHold(ctx, "acme", "fbi"); !ok {
		t.Fatalf("ClearLegalHold: false")
	}
	if set, _ := gs.IsLegalHoldSet(ctx, "acme"); set {
		t.Fatalf("IsLegalHoldSet post-clear: true")
	}
}

func TestQuotaCRUD(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()
	cfg := globalstore.QuotaConfig{
		TenantID:               "acme",
		MaxWritesPerMonth:      1000,
		HardEnforce:            true,
		MaxRPSSustained:        100,
		MaxRPSBurst:            200,
		MaxRPSPerUserSustained: 10,
		MaxRPSPerUserBurst:     20,
	}
	if _, err := gs.SetTenantQuota(ctx, cfg); err != nil {
		t.Fatalf("SetTenantQuota: %v", err)
	}
	got, err := gs.GetTenantQuota(ctx, "acme")
	if err != nil || got == nil {
		t.Fatalf("GetTenantQuota: %+v err=%v", got, err)
	}
	if got.MaxWritesPerMonth != 1000 || !got.HardEnforce || got.MaxRPSBurst != 200 {
		t.Fatalf("GetTenantQuota: %+v", got)
	}

	// Missing tenant => (nil, nil).
	none, err := gs.GetTenantQuota(ctx, "ghost")
	if err != nil || none != nil {
		t.Fatalf("GetTenantQuota ghost: %+v err=%v", none, err)
	}
}

// TestUsageRollover injects a clock that crosses a calendar-month
// boundary and asserts IncrementUsage resets the counter.
func TestUsageRollover(t *testing.T) {
	// Pick two timestamps that straddle a month boundary in UTC.
	tFirst := time.Date(2025, time.March, 15, 12, 0, 0, 0, time.UTC)
	tSecond := time.Date(2025, time.April, 2, 12, 0, 0, 0, time.UTC)

	clockNow := tFirst
	gs := newStore(t, func() int64 { return clockNow.Unix() })
	ctx := context.Background()

	u, err := gs.IncrementUsage(ctx, "acme", 5)
	if err != nil {
		t.Fatalf("IncrementUsage 1: %v", err)
	}
	if u.WritesCount != 5 {
		t.Fatalf("WritesCount #1: got %d, want 5", u.WritesCount)
	}

	// Same month — counter accumulates.
	u, err = gs.IncrementUsage(ctx, "acme", 7)
	if err != nil {
		t.Fatalf("IncrementUsage 2: %v", err)
	}
	if u.WritesCount != 12 {
		t.Fatalf("WritesCount #2: got %d, want 12", u.WritesCount)
	}

	// Advance to the next month — counter resets.
	clockNow = tSecond
	u, err = gs.IncrementUsage(ctx, "acme", 3)
	if err != nil {
		t.Fatalf("IncrementUsage 3: %v", err)
	}
	if u.WritesCount != 3 {
		t.Fatalf("WritesCount post-rollover: got %d, want 3", u.WritesCount)
	}

	// Reset blanks the counter regardless.
	r, err := gs.ResetUsage(ctx, "acme")
	if err != nil {
		t.Fatalf("ResetUsage: %v", err)
	}
	if r.WritesCount != 0 {
		t.Fatalf("ResetUsage: got %d, want 0", r.WritesCount)
	}
}

// TestUsageNegativeIsNoop confirms n <= 0 returns the existing usage
// without mutation, matching the Python guard at global_store.py:1269.
func TestUsageNegativeIsNoop(t *testing.T) {
	gs := newStore(t, nil)
	ctx := context.Background()
	u, err := gs.IncrementUsage(ctx, "acme", 0)
	if err != nil {
		t.Fatalf("IncrementUsage 0: %v", err)
	}
	if u.WritesCount != 0 {
		t.Fatalf("WritesCount: %d", u.WritesCount)
	}
}

// TestSchemaIdempotent re-opens an existing global.db and asserts the
// migrations are no-ops on the second pass.
func TestSchemaIdempotent(t *testing.T) {
	dir := t.TempDir()
	gs1, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("New 1: %v", err)
	}
	ctx := context.Background()
	if _, err := gs1.CreateTenant(ctx, "acme", "A", "eu-west-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	_ = gs1.Close()

	gs2, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("New 2: %v", err)
	}
	defer gs2.Close()
	got, err := gs2.GetTenant(ctx, "acme")
	if err != nil || got == nil || got.Region != "eu-west-1" {
		t.Fatalf("GetTenant after reopen: %+v err=%v", got, err)
	}
}

// must runs fn and fatals with a labelled error. Cuts repetition in
// the multi-step setup blocks above without hiding the failing call.
func must(t *testing.T, label string, fn func() error) {
	t.Helper()
	if err := fn(); err != nil {
		t.Fatalf("%s: %v", label, err)
	}
}
