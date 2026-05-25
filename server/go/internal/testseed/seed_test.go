package testseed

import (
	"context"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// testTopic / testGroupID mirror the server defaults; the seed event
// is keyed by tenant_id so a single in-memory partition is enough.
const (
	testTopic   = "entdb-wal"
	testGroupID = "entdb-applier"
)

// newStores builds the globalstore + canonical store + a connected
// in-memory WAL with the applier already running, mirroring how
// cmd/entdb-server/main.go wires the seed (the applier consumes the
// seed event the seed appends — GitHub issue #505).
func newStores(t *testing.T) (*globalstore.GlobalStore, *store.CanonicalStore, *schema.Registry, wal.Producer) {
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

	w := wal.NewInMemory(1)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}

	applier, err := apply.New(apply.Options{
		Store:       s,
		Global:      g,
		Consumer:    w,
		Topic:       testTopic,
		GroupID:     testGroupID,
		PollTimeout: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- applier.Run(runCtx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	return g, s, reg, w
}

func TestSeedTenant_PopulatesContractFixture(t *testing.T) {
	ctx := context.Background()
	g, s, _, w := newStores(t)

	if err := SeedTenant(ctx, g, s, w, testTopic, "acme"); err != nil {
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

	// The applier (not a pre-bump) advanced the offset for the seed
	// event, which is the first in-memory record (offset 0). The
	// in-memory tracker is therefore populated honestly, so a
	// WaitForOffset for an offset we have already passed returns
	// without blocking — the GitHub-issue-#505 invariant.
	if err := s.WaitForOffset(ctx, "acme", 0); err != nil {
		t.Fatalf("WaitForOffset(0): %v", err)
	}
}

func TestSeedTenant_Idempotent(t *testing.T) {
	ctx := context.Background()
	g, s, _, w := newStores(t)

	if err := SeedTenant(ctx, g, s, w, testTopic, "acme"); err != nil {
		t.Fatalf("first SeedTenant: %v", err)
	}
	if err := SeedTenant(ctx, g, s, w, testTopic, "acme"); err != nil {
		t.Fatalf("second SeedTenant (should be idempotent): %v", err)
	}
}

func newE2EStores(t *testing.T) (*globalstore.GlobalStore, *store.CanonicalStore, *schema.Registry) {
	t.Helper()
	dir := t.TempDir()
	g, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: false})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })

	reg := schema.NewRegistry()
	if err := RegisterE2ESchema(reg); err != nil {
		t.Fatalf("RegisterE2ESchema: %v", err)
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

func TestSeedTenantE2E_PopulatesE2EFixture(t *testing.T) {
	ctx := context.Background()
	g, s, _ := newE2EStores(t)

	if err := SeedTenantE2E(ctx, g, s, "e2e-test"); err != nil {
		t.Fatalf("SeedTenantE2E: %v", err)
	}

	// Tenant present.
	tn, err := g.GetTenant(ctx, "e2e-test")
	if err != nil || tn == nil {
		t.Fatalf("GetTenant: %v, tenant=%v", err, tn)
	}

	// e2e-runner user present.
	u, err := g.GetUser(ctx, E2ERunnerUserID)
	if err != nil || u == nil {
		t.Fatalf("GetUser %q: %v, user=%v", E2ERunnerUserID, err, u)
	}

	// e2e-runner is owner of the tenant.
	members, err := g.GetTenantMembers(ctx, "e2e-test")
	if err != nil {
		t.Fatalf("GetTenantMembers: %v", err)
	}
	var role string
	for _, m := range members {
		if m.UserID == E2ERunnerUserID {
			role = m.Role
		}
	}
	if role != "owner" {
		t.Fatalf("e2e-runner role = %q, want owner", role)
	}
}

func TestSeedTenantE2E_Idempotent(t *testing.T) {
	ctx := context.Background()
	g, s, _ := newE2EStores(t)

	if err := SeedTenantE2E(ctx, g, s, "e2e-test"); err != nil {
		t.Fatalf("first SeedTenantE2E: %v", err)
	}
	if err := SeedTenantE2E(ctx, g, s, "e2e-test"); err != nil {
		t.Fatalf("second SeedTenantE2E (should be idempotent): %v", err)
	}
}

func TestRegisterE2ESchema_DefinesExpectedTypes(t *testing.T) {
	reg := schema.NewRegistry()
	if err := RegisterE2ESchema(reg); err != nil {
		t.Fatalf("RegisterE2ESchema: %v", err)
	}
	if nt := reg.NodeType("User"); nt == nil || nt.TypeID != E2EUserTypeID {
		t.Fatalf("User type missing or wrong id: %+v", nt)
	}
	if nt := reg.NodeType("Product"); nt == nil || nt.TypeID != E2EProductTypeID {
		t.Fatalf("Product type missing or wrong id: %+v", nt)
	}
	if nt := reg.NodeType("Order"); nt == nil || nt.TypeID != E2EOrderTypeID {
		t.Fatalf("Order type missing or wrong id: %+v", nt)
	}
	if et := reg.EdgeType("purchased"); et == nil || et.EdgeID != E2EPurchasedEdgeID {
		t.Fatalf("purchased edge missing or wrong id: %+v", et)
	}
	if et := reg.EdgeType("placed_order"); et == nil || et.EdgeID != E2EPlacedOrderEdgeID {
		t.Fatalf("placed_order edge missing or wrong id: %+v", et)
	}
	if et := reg.EdgeType("contains"); et == nil || et.EdgeID != E2EOrderContainsEdgeID {
		t.Fatalf("contains edge missing or wrong id: %+v", et)
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
	oauth := reg.NodeType("OAuthIdentity")
	if oauth == nil || oauth.TypeID != OAuthIdentityTypeID {
		t.Fatalf("OAuthIdentity type missing or wrong id: %+v", oauth)
	}
	cu := reg.CompositeUnique(OAuthIdentityTypeID)
	if len(cu) != 1 || cu[0].Name != "provider_user_id" ||
		len(cu[0].FieldIDs) != 2 || cu[0].FieldIDs[0] != 1 || cu[0].FieldIDs[1] != 2 {
		t.Fatalf("OAuthIdentity composite unique = %+v, want provider_user_id [1 2]", cu)
	}
}
