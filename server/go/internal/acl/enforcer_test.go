// SPDX-License-Identifier: AGPL-3.0-only

package acl_test

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/acl"
)

func TestEnforcerWiring(t *testing.T) {
	r := acl.NewRegistry()
	res := acl.NewResolver(nil)
	nodes := &fakeNodes{meta: map[string]acl.NodeMeta{
		"t1/n1": {OwnerActor: "user:alice", TypeID: 0},
	}}
	c, err := acl.NewChecker(acl.CheckerOptions{
		Registry: r, Resolver: res, Nodes: nodes, Grants: &fakeGrants{},
	})
	if err != nil {
		t.Fatalf("NewChecker: %v", err)
	}
	f := acl.NewFilter(res, &fakeVis{
		owners: map[string]string{"n1": "user:alice"},
	})
	enf, err := acl.NewEnforcer(acl.EnforcerOptions{
		Registry: r, Resolver: res, Checker: c, Filter: f,
	})
	if err != nil {
		t.Fatalf("NewEnforcer: %v", err)
	}
	// Check delegation.
	if err := enf.Check(context.Background(), acl.CheckRequest{
		TenantID: "t1", Actor: acl.User("alice"), NodeID: "n1", OpName: "DeleteNode",
	}); err != nil {
		t.Fatalf("Enforcer.Check: %v", err)
	}
	// FilterReadable delegation.
	got, err := enf.FilterReadable(context.Background(), "t1", acl.User("alice"), []string{"n1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("Enforcer.FilterReadable = %v err=%v", got, err)
	}
	// Expand delegation.
	exp, err := enf.Expand(context.Background(), "t1", acl.User("alice"))
	if err != nil || len(exp) != 1 {
		t.Fatalf("Enforcer.Expand = %v err=%v", exp, err)
	}
	// RequiredForOp delegation.
	core, ext := enf.RequiredForOp(0, "GetNode", acl.OpQuery{})
	if core != acl.CoreCapRead || ext != 0 {
		t.Fatalf("Enforcer.RequiredForOp = (%v,%v), want (READ,0)", core, ext)
	}
	// LegacyToCoreCaps delegation.
	caps := enf.LegacyToCoreCaps(acl.PermAdmin)
	if len(caps) != 1 || caps[0] != acl.CoreCapAdmin {
		t.Fatalf("Enforcer.LegacyToCoreCaps(ADMIN) = %v", caps)
	}
}

func TestNewEnforcerValidates(t *testing.T) {
	r := acl.NewRegistry()
	res := acl.NewResolver(nil)
	c, _ := acl.NewChecker(acl.CheckerOptions{
		Registry: r, Resolver: res, Nodes: &fakeNodes{}, Grants: &fakeGrants{},
	})
	f := acl.NewFilter(res, &fakeVis{})
	cases := []struct {
		name string
		opts acl.EnforcerOptions
	}{
		{"missing registry", acl.EnforcerOptions{Resolver: res, Checker: c, Filter: f}},
		{"missing resolver", acl.EnforcerOptions{Registry: r, Checker: c, Filter: f}},
		{"missing checker", acl.EnforcerOptions{Registry: r, Resolver: res, Filter: f}},
		{"missing filter", acl.EnforcerOptions{Registry: r, Resolver: res, Checker: c}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := acl.NewEnforcer(tc.opts); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}
