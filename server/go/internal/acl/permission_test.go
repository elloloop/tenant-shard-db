// SPDX-License-Identifier: AGPL-3.0-only

package acl_test

import (
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/acl"
)

func TestPermissionString(t *testing.T) {
	cases := []struct {
		p    acl.Permission
		want string
	}{
		{acl.PermDeny, "deny"},
		{acl.PermRead, "read"},
		{acl.PermComment, "comment"},
		{acl.PermWrite, "write"},
		{acl.PermShare, "share"},
		{acl.PermDelete, "delete"},
		{acl.PermAdmin, "admin"},
	}
	for _, c := range cases {
		if got := c.p.String(); got != c.want {
			t.Fatalf("String(%v) = %q, want %q", c.p, got, c.want)
		}
	}
}

func TestParsePermission(t *testing.T) {
	for _, s := range []string{"deny", "read", "comment", "write", "share", "delete", "admin"} {
		p, ok := acl.ParsePermission(s)
		if !ok {
			t.Fatalf("ParsePermission(%q) failed", s)
		}
		if p.String() != s {
			t.Fatalf("round-trip %q -> %v -> %q", s, p, p.String())
		}
	}
	if _, ok := acl.ParsePermission("garbage"); ok {
		t.Fatalf("ParsePermission(garbage) should fail")
	}
	if _, ok := acl.ParsePermission(""); ok {
		t.Fatalf("ParsePermission empty should fail")
	}
}

// TestPermissionImplies pins the permission implication hierarchy.
func TestPermissionImplies(t *testing.T) {
	cases := []struct {
		grant, need acl.Permission
		want        bool
	}{
		// Read implies only Read.
		{acl.PermRead, acl.PermRead, true},
		{acl.PermRead, acl.PermComment, false},
		{acl.PermRead, acl.PermWrite, false},
		// Comment implies Read+Comment.
		{acl.PermComment, acl.PermRead, true},
		{acl.PermComment, acl.PermComment, true},
		{acl.PermComment, acl.PermWrite, false},
		// Write implies Read, Comment, Write — NOT Share/Delete.
		{acl.PermWrite, acl.PermRead, true},
		{acl.PermWrite, acl.PermComment, true},
		{acl.PermWrite, acl.PermWrite, true},
		{acl.PermWrite, acl.PermShare, false},
		{acl.PermWrite, acl.PermDelete, false},
		// Share implies Read/Comment/Write/Share — NOT Delete.
		{acl.PermShare, acl.PermShare, true},
		{acl.PermShare, acl.PermDelete, false},
		// Delete implies Read/Comment/Write/Delete — NOT Share.
		{acl.PermDelete, acl.PermDelete, true},
		{acl.PermDelete, acl.PermShare, false},
		// Admin implies everything positive.
		{acl.PermAdmin, acl.PermRead, true},
		{acl.PermAdmin, acl.PermComment, true},
		{acl.PermAdmin, acl.PermWrite, true},
		{acl.PermAdmin, acl.PermShare, true},
		{acl.PermAdmin, acl.PermDelete, true},
		{acl.PermAdmin, acl.PermAdmin, true},
		// Deny implies nothing.
		{acl.PermDeny, acl.PermRead, false},
		{acl.PermDeny, acl.PermAdmin, false},
	}
	for _, c := range cases {
		if got := c.grant.Implies(c.need); got != c.want {
			t.Fatalf("Implies(%v→%v) = %v, want %v", c.grant, c.need, got, c.want)
		}
	}
}

// TestLegacyToCoreCaps pins the migration table in docs/decisions/acl.md.
func TestLegacyToCoreCaps(t *testing.T) {
	cases := []struct {
		in   acl.Permission
		want []acl.CoreCapability
	}{
		{acl.PermDeny, nil},
		{acl.PermRead, []acl.CoreCapability{acl.CoreCapRead}},
		{acl.PermComment, []acl.CoreCapability{acl.CoreCapRead, acl.CoreCapComment}},
		{acl.PermWrite, []acl.CoreCapability{acl.CoreCapRead, acl.CoreCapComment, acl.CoreCapEdit}},
		{acl.PermShare, []acl.CoreCapability{acl.CoreCapAdmin}},
		{acl.PermDelete, []acl.CoreCapability{acl.CoreCapRead, acl.CoreCapComment, acl.CoreCapEdit, acl.CoreCapDelete}},
		{acl.PermAdmin, []acl.CoreCapability{acl.CoreCapAdmin}},
	}
	for _, c := range cases {
		got := acl.LegacyToCoreCaps(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("LegacyToCoreCaps(%v) = %v (len=%d), want %v", c.in, got, len(got), c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("LegacyToCoreCaps(%v)[%d] = %v, want %v", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestCoreCapabilityString(t *testing.T) {
	cases := []struct {
		c    acl.CoreCapability
		want string
	}{
		{acl.CoreCapUnspecified, "CORE_CAP_UNSPECIFIED"},
		{acl.CoreCapRead, "CORE_CAP_READ"},
		{acl.CoreCapComment, "CORE_CAP_COMMENT"},
		{acl.CoreCapEdit, "CORE_CAP_EDIT"},
		{acl.CoreCapDelete, "CORE_CAP_DELETE"},
		{acl.CoreCapAdmin, "CORE_CAP_ADMIN"},
	}
	for _, c := range cases {
		if got := c.c.String(); got != c.want {
			t.Fatalf("CoreCapability(%v).String = %q, want %q", c.c, got, c.want)
		}
	}
}
