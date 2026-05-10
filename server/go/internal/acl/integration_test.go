// SPDX-License-Identifier: AGPL-3.0-only

// Integration tests that exercise the acl package against a real
// store.CanonicalStore + globalstore.GlobalStore. The acl package
// itself does not depend on store/globalstore — these tests prove the
// production wiring shape works.
//
// Production binding pattern: a thin adapter in the api package wraps
// store.CanonicalStore and exposes NodeMetaReader / GrantReader /
// VisibilityReader / GroupMembershipReader on top of the existing
// public methods (GetNode, ListSharedWithMe, GetVisibleNodeIDs,
// AddGroupMember). The adapter is intentionally NOT in this package —
// W1.10 will land it alongside the applier.

package acl_test

import (
	"context"
	"sort"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/acl"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

func newRealStores(t *testing.T) (*store.CanonicalStore, *globalstore.GlobalStore) {
	t.Helper()
	cs, err := store.New(store.Options{RootDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	gs, err := globalstore.New(globalstore.Options{DataDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })
	return cs, gs
}

// storeVisibilityAdapter wraps store.CanonicalStore.GetVisibleNodeIDs
// to satisfy acl.VisibilityReader.
type storeVisibilityAdapter struct {
	s *store.CanonicalStore
}

func (a *storeVisibilityAdapter) VisibleNodeIDs(ctx context.Context, tenant string, actorIDs []string, nodeIDs []string) (map[string]struct{}, error) {
	return a.s.GetVisibleNodeIDs(ctx, tenant, actorIDs, nodeIDs)
}

// xtAdapter wraps globalstore.GlobalStore.ListSharedFromNode for the
// (small, single-node) cross-tenant lookup used by the Checker.
type xtAdapter struct {
	g *globalstore.GlobalStore
}

func (x *xtAdapter) CrossTenantGrant(ctx context.Context, tenant, node, actor string) (acl.Permission, bool, error) {
	rows, err := x.g.ListSharedFromNode(ctx, tenant, node)
	if err != nil {
		return acl.PermDeny, false, err
	}
	for _, r := range rows {
		if "user:"+r.UserID == actor {
			p, ok := acl.ParsePermission(r.Permission)
			if !ok {
				continue
			}
			return p, true, nil
		}
	}
	return acl.PermDeny, false, nil
}

func TestFilterReadableRealStore(t *testing.T) {
	cs, _ := newRealStores(t)
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, "t1"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	// alice owns n1, bob owns n2, carol owns n3.
	mustCreate(t, cs, "t1", "n1", "user:alice")
	mustCreate(t, cs, "t1", "n2", "user:bob")
	mustCreate(t, cs, "t1", "n3", "user:carol")
	// Share n2 with bob (a no-op since bob already owns it) and n1 with bob.
	if err := cs.AddVisibility(ctx, "t1", "n1", "user:bob"); err != nil {
		t.Fatalf("AddVisibility: %v", err)
	}

	// Use the resolver with a no-op group reader (no groups seeded).
	r := acl.NewResolver(nil)
	f := acl.NewFilter(r, &storeVisibilityAdapter{s: cs})

	got, err := f.FilterReadable(ctx, "t1", acl.User("bob"), []string{"n1", "n2", "n3"})
	if err != nil {
		t.Fatalf("FilterReadable: %v", err)
	}
	sort.Strings(got)
	if len(got) != 2 || got[0] != "n1" || got[1] != "n2" {
		t.Fatalf("FilterReadable bob = %v, want [n1 n2]", got)
	}
}

func TestCrossTenantGrantRealStore(t *testing.T) {
	_, gs := newRealStores(t)
	ctx := context.Background()
	// Seed the globalstore shared_index with a cross-tenant grant.
	if err := gs.AddShared(ctx, "carol", "acme", "n1", "read"); err != nil {
		t.Fatalf("AddShared: %v", err)
	}
	xt := &xtAdapter{g: gs}
	perm, ok, err := xt.CrossTenantGrant(ctx, "acme", "n1", "user:carol")
	if err != nil {
		t.Fatalf("CrossTenantGrant: %v", err)
	}
	if !ok {
		t.Fatalf("CrossTenantGrant: row should exist")
	}
	if perm != acl.PermRead {
		t.Fatalf("CrossTenantGrant perm = %v, want PermRead", perm)
	}
	// Absent row.
	_, ok, _ = xt.CrossTenantGrant(ctx, "acme", "n1", "user:dan")
	if ok {
		t.Fatalf("CrossTenantGrant absent row should not exist")
	}
}

func mustCreate(t *testing.T, cs *store.CanonicalStore, tenant, node, owner string) {
	t.Helper()
	_, err := cs.CreateNodeRaw(context.Background(), tenant, store.NodeInput{
		NodeID:     node,
		TypeID:     1,
		OwnerActor: owner,
	})
	if err != nil {
		t.Fatalf("CreateNodeRaw %s: %v", node, err)
	}
}
