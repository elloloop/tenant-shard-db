// SPDX-License-Identifier: AGPL-3.0-only

package acl_test

import (
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/acl"
)

func TestActorString(t *testing.T) {
	cases := []struct {
		a    acl.Actor
		want string
	}{
		{acl.User("alice"), "user:alice"},
		{acl.Group("eng"), "group:eng"},
		{acl.Service("svc1"), "service:svc1"},
		{acl.Role("admin"), "role:admin"},
		{acl.Tenant("acme"), "tenant:acme"},
		{acl.TenantWildcard(), "tenant:*"},
		{acl.System("applier"), "system:applier"},
		{acl.Actor{}, ""},
	}
	for _, c := range cases {
		if got := c.a.String(); got != c.want {
			t.Fatalf("String(%+v) = %q, want %q", c.a, got, c.want)
		}
	}
}

func TestParseActor(t *testing.T) {
	cases := []struct {
		in   string
		want acl.Actor
	}{
		{"", acl.Actor{}},
		{"user:alice", acl.User("alice")},
		{"group:eng", acl.Group("eng")},
		{"tenant:*", acl.TenantWildcard()},
		{"tenant:acme", acl.Tenant("acme")},
		{"system:applier", acl.System("applier")},
		{"service:svc1", acl.Service("svc1")},
		{"role:admin", acl.Role("admin")},
	}
	for _, c := range cases {
		got := acl.ParseActor(c.in)
		if got != c.want {
			t.Fatalf("ParseActor(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
	// Malformed inputs preserve the raw string.
	got := acl.ParseActor("garbage")
	if got.Kind != acl.ActorKindUnknown || got.ID != "garbage" {
		t.Fatalf("ParseActor(garbage) = %+v, want unknown/garbage", got)
	}
	// Trailing colon = malformed.
	got = acl.ParseActor("user:")
	if got.Kind != acl.ActorKindUnknown {
		t.Fatalf("ParseActor(user:) classified as %v, want Unknown", got.Kind)
	}
	// Unknown prefix.
	got = acl.ParseActor("alien:42")
	if got.Kind != acl.ActorKindUnknown {
		t.Fatalf("ParseActor(alien:42) classified as %v, want Unknown", got.Kind)
	}
}

func TestActorPredicates(t *testing.T) {
	if !acl.User("a").IsUser() {
		t.Fatalf("User.IsUser")
	}
	if !acl.Group("g").IsGroup() {
		t.Fatalf("Group.IsGroup")
	}
	if !acl.System("s").IsSystem() {
		t.Fatalf("System.IsSystem")
	}
	if !acl.TenantWildcard().IsTenantWildcard() {
		t.Fatalf("TenantWildcard.IsTenantWildcard")
	}
	if acl.Tenant("acme").IsTenantWildcard() {
		t.Fatalf("tenant:acme should not be tenant wildcard")
	}
	if !(acl.Actor{}).IsZero() {
		t.Fatalf("zero Actor should IsZero()")
	}
}
