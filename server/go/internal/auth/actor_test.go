// SPDX-License-Identifier: AGPL-3.0-only

package auth

import "testing"

func TestActorConstructors(t *testing.T) {
	tests := []struct {
		name string
		a    Actor
		want string
		kind Kind
	}{
		{"user", User("alice"), "user:alice", KindUser},
		{"system", System("gdpr-worker"), "system:gdpr-worker", KindSystem},
		{"admin", Admin("root"), "admin:root", KindAdmin},
		{"group", Group("admins"), "group:admins", KindGroup},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
			if tt.a.Kind() != tt.kind {
				t.Errorf("Kind() = %v, want %v", tt.a.Kind(), tt.kind)
			}
		})
	}
}

func TestActorPredicates(t *testing.T) {
	cases := []struct {
		a        Actor
		isUser   bool
		isSystem bool
		isAdmin  bool
		isGroup  bool
	}{
		{User("a"), true, false, false, false},
		{System("s"), false, true, false, false},
		{Admin("r"), false, false, true, false},
		{Group("g"), false, false, false, true},
		{Actor{}, false, false, false, false},
	}
	for _, c := range cases {
		if got := c.a.IsUser(); got != c.isUser {
			t.Errorf("%v.IsUser()=%v want %v", c.a, got, c.isUser)
		}
		if got := c.a.IsSystem(); got != c.isSystem {
			t.Errorf("%v.IsSystem()=%v want %v", c.a, got, c.isSystem)
		}
		if got := c.a.IsAdmin(); got != c.isAdmin {
			t.Errorf("%v.IsAdmin()=%v want %v", c.a, got, c.isAdmin)
		}
		if got := c.a.IsGroup(); got != c.isGroup {
			t.Errorf("%v.IsGroup()=%v want %v", c.a, got, c.isGroup)
		}
	}
}

func TestActorZeroValue(t *testing.T) {
	var z Actor
	if !z.IsZero() {
		t.Fatalf("zero Actor should report IsZero")
	}
	if z.String() != "" {
		t.Errorf("zero Actor String() = %q, want empty", z.String())
	}
	if User("alice").IsZero() {
		t.Errorf("User actor should not be zero")
	}
}

func TestParseActor(t *testing.T) {
	tests := []struct {
		in       string
		wantKind Kind
		wantID   string
	}{
		{"user:alice", KindUser, "alice"},
		{"system:gdpr-worker", KindSystem, "gdpr-worker"},
		{"admin:root", KindAdmin, "root"},
		{"group:admins", KindGroup, "admins"},
		{"", KindUnknown, ""},           // zero value
		{"alice", KindUnknown, "alice"}, // bare id, no prefix
		{"user:", KindUnknown, "user:"}, // empty id is rejected
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := ParseActor(tt.in)
			if got.Kind() != tt.wantKind {
				t.Errorf("Kind = %v, want %v", got.Kind(), tt.wantKind)
			}
			if got.ID() != tt.wantID {
				t.Errorf("ID = %q, want %q", got.ID(), tt.wantID)
			}
		})
	}
}

func TestKindString(t *testing.T) {
	cases := []struct {
		k    Kind
		want string
	}{
		{KindUser, "user"},
		{KindSystem, "system"},
		{KindAdmin, "admin"},
		{KindGroup, "group"},
		{KindUnknown, "unknown"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Errorf("Kind(%d).String() = %q, want %q", c.k, got, c.want)
		}
	}
}
