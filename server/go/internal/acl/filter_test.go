// SPDX-License-Identifier: AGPL-3.0-only

package acl_test

import (
	"context"
	"sort"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/acl"
)

// fakeVis is an in-memory VisibilityReader. It treats actorIDs union
// {"tenant:*"} as the principal set and reports any node it has been
// pre-told the actor can see.
type fakeVis struct {
	// owners maps node_id -> owner principal.
	owners map[string]string
	// vis maps node_id -> set of principals (canonical "kind:id" strings).
	vis map[string]map[string]struct{}
}

func (f *fakeVis) VisibleNodeIDs(_ context.Context, _ string, actorIDs []string, nodeIDs []string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	allowed := map[string]struct{}{}
	for _, a := range actorIDs {
		allowed[a] = struct{}{}
	}
	allowed["tenant:*"] = struct{}{} // wildcard matches every authenticated user.
	for _, id := range nodeIDs {
		if _, ok := allowed[f.owners[id]]; ok {
			out[id] = struct{}{}
			continue
		}
		for p := range f.vis[id] {
			if _, ok := allowed[p]; ok {
				out[id] = struct{}{}
				break
			}
		}
	}
	return out, nil
}

func TestFilterReadableEmptyInputs(t *testing.T) {
	r := acl.NewResolver(nil)
	f := acl.NewFilter(r, &fakeVis{})
	got, err := f.FilterReadable(context.Background(), "t1", acl.User("a"), nil)
	if err != nil || got != nil {
		t.Fatalf("empty nodeIDs = %v, err=%v", got, err)
	}
	got, err = f.FilterReadable(context.Background(), "t1", acl.Actor{}, []string{"n1"})
	if err != nil || got != nil {
		t.Fatalf("empty actor = %v, err=%v", got, err)
	}
}

func TestFilterReadableSubset(t *testing.T) {
	vis := &fakeVis{
		owners: map[string]string{"n1": "user:alice", "n2": "user:bob", "n3": "user:carol"},
		vis: map[string]map[string]struct{}{
			"n1": {"user:alice": {}},
			"n2": {"user:bob": {}, "user:alice": {}}, // alice was shared n2.
			"n3": {"user:carol": {}},
		},
	}
	r := acl.NewResolver(nil)
	f := acl.NewFilter(r, vis)
	got, err := f.FilterReadable(context.Background(), "t1", acl.User("alice"),
		[]string{"n1", "n2", "n3"})
	if err != nil {
		t.Fatalf("FilterReadable: %v", err)
	}
	sort.Strings(got)
	if len(got) != 2 || got[0] != "n1" || got[1] != "n2" {
		t.Fatalf("FilterReadable = %v, want [n1 n2]", got)
	}
}

func TestFilterReadableSystemBypass(t *testing.T) {
	vis := &fakeVis{} // empty — would deny everything for normal user.
	r := acl.NewResolver(nil)
	f := acl.NewFilter(r, vis)
	got, err := f.FilterReadable(context.Background(), "t1", acl.System("applier"),
		[]string{"n1", "n2"})
	if err != nil {
		t.Fatalf("FilterReadable: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("system bypass should return all nodes, got %v", got)
	}
}

func TestFilterReadableTenantWildcardSeesAll(t *testing.T) {
	vis := &fakeVis{
		owners: map[string]string{"n1": "user:alice", "n2": "user:bob"},
		vis: map[string]map[string]struct{}{
			"n1": {"tenant:*": {}}, // all tenant readable.
			"n2": {"user:bob": {}},
		},
	}
	r := acl.NewResolver(nil)
	f := acl.NewFilter(r, vis)
	got, err := f.FilterReadable(context.Background(), "t1", acl.User("carol"), []string{"n1", "n2"})
	if err != nil {
		t.Fatalf("FilterReadable: %v", err)
	}
	if len(got) != 1 || got[0] != "n1" {
		t.Fatalf("FilterReadable carol = %v, want [n1]", got)
	}
}

func TestFilterReadableGroupExpansion(t *testing.T) {
	groups := &fakeGroups{parents: map[string][]string{
		"user:bob": {"eng"},
	}}
	vis := &fakeVis{
		owners: map[string]string{"n1": "user:alice"},
		vis: map[string]map[string]struct{}{
			"n1": {"group:eng": {}}, // shared with eng.
		},
	}
	r := acl.NewResolver(groups)
	f := acl.NewFilter(r, vis)
	got, err := f.FilterReadable(context.Background(), "t1", acl.User("bob"), []string{"n1"})
	if err != nil {
		t.Fatalf("FilterReadable: %v", err)
	}
	if len(got) != 1 || got[0] != "n1" {
		t.Fatalf("group-shared filter = %v, want [n1]", got)
	}
}
