// SPDX-License-Identifier: AGPL-3.0-only

package acl_test

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/acl"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

// fakeNodes is an in-memory NodeMetaReader.
type fakeNodes struct {
	meta map[string]acl.NodeMeta // key = "tenant/node"
}

func (f *fakeNodes) NodeMeta(_ context.Context, tenant, node string) (acl.NodeMeta, error) {
	m, ok := f.meta[tenant+"/"+node]
	if !ok {
		return acl.NodeMeta{}, errs.Errorf(codes.NotFound,
			"acl_test: node %s/%s not found", tenant, node)
	}
	return m, nil
}

// fakeGrants is an in-memory GrantReader.
type fakeGrants struct {
	rows map[string][]acl.Grant // key = "tenant/node"
}

func (f *fakeGrants) GrantsForNode(_ context.Context, tenant, node string) ([]acl.Grant, error) {
	return f.rows[tenant+"/"+node], nil
}

// fakeXT is an in-memory CrossTenantGrantReader.
type fakeXT struct {
	rows map[string]acl.Permission // key = "tenant/node/actor"
}

func (f *fakeXT) CrossTenantGrant(_ context.Context, tenant, node, actor string) (acl.Permission, bool, error) {
	p, ok := f.rows[tenant+"/"+node+"/"+actor]
	return p, ok, nil
}

// newCheckerWithFixtures builds a Checker with empty fakes; callers
// populate them.
func newCheckerWithFixtures(t *testing.T, groups *fakeGroups, nodes *fakeNodes, grants *fakeGrants, xt *fakeXT, nowFn func() int64) *acl.Checker {
	t.Helper()
	r := acl.NewRegistry()
	res := acl.NewResolver(groups)
	c, err := acl.NewChecker(acl.CheckerOptions{
		Registry:    r,
		Resolver:    res,
		Nodes:       nodes,
		Grants:      grants,
		CrossTenant: xt,
		NowFn:       nowFn,
	})
	if err != nil {
		t.Fatalf("NewChecker: %v", err)
	}
	return c
}

func TestCheckerSystemBypass(t *testing.T) {
	c := newCheckerWithFixtures(t, &fakeGroups{}, &fakeNodes{}, &fakeGrants{}, nil, nil)
	// System actor returns nil even without any setup.
	err := c.Check(context.Background(), acl.CheckRequest{
		TenantID: "t1", Actor: acl.System("applier"), NodeID: "n1", OpName: "DeleteNode",
	})
	if err != nil {
		t.Fatalf("system bypass: %v", err)
	}
}

func TestCheckerOwnerAlwaysAllowed(t *testing.T) {
	nodes := &fakeNodes{meta: map[string]acl.NodeMeta{
		"t1/n1": {OwnerActor: "user:alice", TypeID: 0},
	}}
	c := newCheckerWithFixtures(t, &fakeGroups{}, nodes, &fakeGrants{}, nil, nil)
	err := c.Check(context.Background(), acl.CheckRequest{
		TenantID: "t1", Actor: acl.User("alice"), NodeID: "n1", OpName: "DeleteNode",
	})
	if err != nil {
		t.Fatalf("owner check: %v", err)
	}
}

func TestCheckerNonOwnerNoGrantsDenied(t *testing.T) {
	nodes := &fakeNodes{meta: map[string]acl.NodeMeta{
		"t1/n1": {OwnerActor: "user:alice", TypeID: 0},
	}}
	c := newCheckerWithFixtures(t, &fakeGroups{}, nodes, &fakeGrants{}, nil, nil)
	err := c.Check(context.Background(), acl.CheckRequest{
		TenantID: "t1", Actor: acl.User("bob"), NodeID: "n1", OpName: "GetNode",
	})
	if err == nil {
		t.Fatalf("non-owner with no grants should be denied")
	}
	if !errors.Is(err, errs.ErrPermission) {
		t.Fatalf("err = %v, want errs.ErrPermission", err)
	}
}

func TestCheckerDirectGrantAllows(t *testing.T) {
	nodes := &fakeNodes{meta: map[string]acl.NodeMeta{
		"t1/n1": {OwnerActor: "user:alice", TypeID: 0},
	}}
	grants := &fakeGrants{rows: map[string][]acl.Grant{
		"t1/n1": {
			{Subject: acl.User("bob"), NodeID: "n1", Permission: acl.PermRead},
		},
	}}
	c := newCheckerWithFixtures(t, &fakeGroups{}, nodes, grants, nil, nil)
	err := c.Check(context.Background(), acl.CheckRequest{
		TenantID: "t1", Actor: acl.User("bob"), NodeID: "n1", OpName: "GetNode",
	})
	if err != nil {
		t.Fatalf("direct READ grant: %v", err)
	}
	// Same READ grant should NOT permit DeleteNode (DELETE).
	err = c.Check(context.Background(), acl.CheckRequest{
		TenantID: "t1", Actor: acl.User("bob"), NodeID: "n1", OpName: "DeleteNode",
	})
	if err == nil {
		t.Fatalf("READ grant should not satisfy DELETE")
	}
	if !errors.Is(err, errs.ErrPermission) {
		t.Fatalf("err = %v, want errs.ErrPermission", err)
	}
}

func TestCheckerExplicitDenyOverrides(t *testing.T) {
	nodes := &fakeNodes{meta: map[string]acl.NodeMeta{
		"t1/n1": {OwnerActor: "user:alice", TypeID: 0},
	}}
	grants := &fakeGrants{rows: map[string][]acl.Grant{
		"t1/n1": {
			// Allow grant first.
			{Subject: acl.User("bob"), Permission: acl.PermAdmin},
			// Explicit deny — wins regardless of position.
			{Subject: acl.User("bob"), Permission: acl.PermDeny},
		},
	}}
	c := newCheckerWithFixtures(t, &fakeGroups{}, nodes, grants, nil, nil)
	err := c.Check(context.Background(), acl.CheckRequest{
		TenantID: "t1", Actor: acl.User("bob"), NodeID: "n1", OpName: "GetNode",
	})
	if !errors.Is(err, errs.ErrPermission) {
		t.Fatalf("DENY should win, got %v", err)
	}
}

func TestCheckerGroupGrantInherited(t *testing.T) {
	// Grant on group:eng. user:bob ∈ eng.
	groups := &fakeGroups{parents: map[string][]string{
		"user:bob": {"eng"},
	}}
	nodes := &fakeNodes{meta: map[string]acl.NodeMeta{
		"t1/n1": {OwnerActor: "user:alice", TypeID: 0},
	}}
	grants := &fakeGrants{rows: map[string][]acl.Grant{
		"t1/n1": {
			{Subject: acl.Group("eng"), Permission: acl.PermRead},
		},
	}}
	c := newCheckerWithFixtures(t, groups, nodes, grants, nil, nil)
	err := c.Check(context.Background(), acl.CheckRequest{
		TenantID: "t1", Actor: acl.User("bob"), NodeID: "n1", OpName: "GetNode",
	})
	if err != nil {
		t.Fatalf("group-inherited grant: %v", err)
	}
	// alice (not in eng) is also denied (she's the owner — owner short-circuit).
	// Use carol who is unrelated.
	err = c.Check(context.Background(), acl.CheckRequest{
		TenantID: "t1", Actor: acl.User("carol"), NodeID: "n1", OpName: "GetNode",
	})
	if !errors.Is(err, errs.ErrPermission) {
		t.Fatalf("non-member should be denied, got %v", err)
	}
}

func TestCheckerExpiredGrantIgnored(t *testing.T) {
	pastMs := int64(1000)
	nodes := &fakeNodes{meta: map[string]acl.NodeMeta{
		"t1/n1": {OwnerActor: "user:alice", TypeID: 0},
	}}
	grants := &fakeGrants{rows: map[string][]acl.Grant{
		"t1/n1": {
			{Subject: acl.User("bob"), Permission: acl.PermAdmin, ExpiresAt: &pastMs},
		},
	}}
	c := newCheckerWithFixtures(t, &fakeGroups{}, nodes, grants, nil, func() int64 { return 9999 })
	err := c.Check(context.Background(), acl.CheckRequest{
		TenantID: "t1", Actor: acl.User("bob"), NodeID: "n1", OpName: "GetNode",
	})
	if !errors.Is(err, errs.ErrPermission) {
		t.Fatalf("expired grant should not allow, got %v", err)
	}
}

func TestCheckerTenantWildcardGrant(t *testing.T) {
	nodes := &fakeNodes{meta: map[string]acl.NodeMeta{
		"t1/n1": {OwnerActor: "user:alice", TypeID: 0},
	}}
	grants := &fakeGrants{rows: map[string][]acl.Grant{
		"t1/n1": {
			{Subject: acl.TenantWildcard(), Permission: acl.PermRead},
		},
	}}
	c := newCheckerWithFixtures(t, &fakeGroups{}, nodes, grants, nil, nil)
	err := c.Check(context.Background(), acl.CheckRequest{
		TenantID: "t1", Actor: acl.User("bob"), NodeID: "n1", OpName: "GetNode",
	})
	if err != nil {
		t.Fatalf("tenant:* should grant READ to any same-tenant user: %v", err)
	}
}

func TestCheckerCrossTenantGrant(t *testing.T) {
	nodes := &fakeNodes{meta: map[string]acl.NodeMeta{
		"acme/n1": {OwnerActor: "user:alice", TypeID: 0},
	}}
	grants := &fakeGrants{rows: map[string][]acl.Grant{}}
	xt := &fakeXT{rows: map[string]acl.Permission{
		"acme/n1/user:carol": acl.PermRead,
	}}
	c := newCheckerWithFixtures(t, &fakeGroups{}, nodes, grants, xt, nil)
	// carol is in tenant "beta" reading from "acme".
	err := c.Check(context.Background(), acl.CheckRequest{
		TenantID:      "acme",
		ActorTenantID: "beta",
		Actor:         acl.User("carol"),
		NodeID:        "n1",
		OpName:        "GetNode",
	})
	if err != nil {
		t.Fatalf("cross-tenant READ: %v", err)
	}
	// But cross-tenant READ does not allow DELETE.
	err = c.Check(context.Background(), acl.CheckRequest{
		TenantID:      "acme",
		ActorTenantID: "beta",
		Actor:         acl.User("carol"),
		NodeID:        "n1",
		OpName:        "DeleteNode",
	})
	if !errors.Is(err, errs.ErrPermission) {
		t.Fatalf("cross-tenant READ should not satisfy DELETE, got %v", err)
	}
}

func TestCheckerOpWithoutDefaultAllows(t *testing.T) {
	// CreateNode is intentionally absent from DefaultOpRequirements —
	// no per-node check applies (capability_registry.py:60-61).
	nodes := &fakeNodes{meta: map[string]acl.NodeMeta{
		"t1/n1": {OwnerActor: "user:alice", TypeID: 0},
	}}
	c := newCheckerWithFixtures(t, &fakeGroups{}, nodes, &fakeGrants{}, nil, nil)
	err := c.Check(context.Background(), acl.CheckRequest{
		TenantID: "t1", Actor: acl.User("carol"), NodeID: "n1", OpName: "CreateNode",
	})
	if err != nil {
		t.Fatalf("CreateNode (no requirement) should allow: %v", err)
	}
}

func TestCheckerEmptyActorRejected(t *testing.T) {
	c := newCheckerWithFixtures(t, &fakeGroups{}, &fakeNodes{}, &fakeGrants{}, nil, nil)
	err := c.Check(context.Background(), acl.CheckRequest{
		TenantID: "t1", NodeID: "n1", OpName: "GetNode",
	})
	if !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("empty actor should err InvalidArgument, got %v", err)
	}
}

func TestCheckerNodeNotFoundPropagates(t *testing.T) {
	c := newCheckerWithFixtures(t, &fakeGroups{}, &fakeNodes{meta: map[string]acl.NodeMeta{}}, &fakeGrants{}, nil, nil)
	err := c.Check(context.Background(), acl.CheckRequest{
		TenantID: "t1", Actor: acl.User("alice"), NodeID: "missing", OpName: "GetNode",
	})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("missing node should propagate NotFound, got %v", err)
	}
}

func TestNewCheckerValidatesArgs(t *testing.T) {
	r := acl.NewRegistry()
	res := acl.NewResolver(nil)
	for _, tc := range []struct {
		name string
		opts acl.CheckerOptions
	}{
		{"missing registry", acl.CheckerOptions{Resolver: res, Nodes: &fakeNodes{}, Grants: &fakeGrants{}}},
		{"missing resolver", acl.CheckerOptions{Registry: r, Nodes: &fakeNodes{}, Grants: &fakeGrants{}}},
		{"missing nodes", acl.CheckerOptions{Registry: r, Resolver: res, Grants: &fakeGrants{}}},
		{"missing grants", acl.CheckerOptions{Registry: r, Resolver: res, Nodes: &fakeNodes{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := acl.NewChecker(tc.opts); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}
