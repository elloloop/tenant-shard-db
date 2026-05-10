package testseed

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

func newStores(t *testing.T) (*globalstore.GlobalStore, *store.CanonicalStore, *schema.Registry) {
	t.Helper()
	dir := t.TempDir()
	g, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: false})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })

	reg := schema.NewRegistry()
	if err := RegisterContractSchema(reg); err != nil {
		t.Fatalf("RegisterContractSchema: %v", err)
	}
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}

	s, err := store.New(store.Options{RootDir: dir, WALMode: false, Registry: reg})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	return g, s, reg
}

func TestSeedTenant_PopulatesContractFixture(t *testing.T) {
	ctx := context.Background()
	g, s, _ := newStores(t)

	if err := SeedTenant(ctx, g, s, "acme"); err != nil {
		t.Fatalf("SeedTenant: %v", err)
	}

	// Tenant present.
	tn, err := g.GetTenant(ctx, "acme")
	if err != nil || tn == nil {
		t.Fatalf("GetTenant: %v, tenant=%v", err, tn)
	}
	if tn.Status != "active" {
		t.Fatalf("tenant status = %q, want active", tn.Status)
	}

	// Users present.
	for _, uid := range []string{"alice", "bob"} {
		u, err := g.GetUser(ctx, uid)
		if err != nil || u == nil {
			t.Fatalf("GetUser %q: %v, user=%v", uid, err, u)
		}
	}

	// Memberships: alice=owner, bob=member.
	members, err := g.GetTenantMembers(ctx, "acme")
	if err != nil {
		t.Fatalf("GetTenantMembers: %v", err)
	}
	roles := map[string]string{}
	for _, m := range members {
		roles[m.UserID] = m.Role
	}
	if roles["alice"] != "owner" {
		t.Fatalf("alice role = %q, want owner", roles["alice"])
	}
	if roles["bob"] != "member" {
		t.Fatalf("bob role = %q, want member", roles["bob"])
	}

	// Seed node present.
	n, err := s.GetNode(ctx, "acme", SeedNodeID)
	if err != nil || n == nil {
		t.Fatalf("GetNode seeded-node: %v, node=%v", err, n)
	}
	if n.OwnerActor != AliceActor {
		t.Fatalf("seed node owner = %q, want %q", n.OwnerActor, AliceActor)
	}

	// Receipt applied.
	applied, err := s.CheckIdempotency(ctx, "acme", SeedReceipt)
	if err != nil {
		t.Fatalf("CheckIdempotency: %v", err)
	}
	if !applied {
		t.Fatalf("CheckIdempotency seed-1 = false, want true")
	}

	// WaitForOffset returns immediately for target=0 since the applied
	// offset map is populated.
	if err := s.WaitForOffset(ctx, "acme", 0); err != nil {
		t.Fatalf("WaitForOffset(0): %v", err)
	}
}

func TestSeedTenant_Idempotent(t *testing.T) {
	ctx := context.Background()
	g, s, _ := newStores(t)

	if err := SeedTenant(ctx, g, s, "acme"); err != nil {
		t.Fatalf("first SeedTenant: %v", err)
	}
	if err := SeedTenant(ctx, g, s, "acme"); err != nil {
		t.Fatalf("second SeedTenant (should be idempotent): %v", err)
	}
}

func TestRegisterContractSchema_DefinesExpectedTypes(t *testing.T) {
	reg := schema.NewRegistry()
	if err := RegisterContractSchema(reg); err != nil {
		t.Fatalf("RegisterContractSchema: %v", err)
	}
	if nt := reg.NodeType("User"); nt == nil || nt.TypeID != UserTypeID {
		t.Fatalf("User type missing or wrong id: %+v", nt)
	}
	if nt := reg.NodeType("Task"); nt == nil || nt.TypeID != TaskTypeID {
		t.Fatalf("Task type missing or wrong id: %+v", nt)
	}
	if et := reg.EdgeType("AssignedTo"); et == nil || et.EdgeID != AssignedToID {
		t.Fatalf("AssignedTo edge missing or wrong id: %+v", et)
	}
}
