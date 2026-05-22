// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"testing"
)

// TestAuthoritative_TrustedWinsOverClaim is the privilege-escalation
// guard: when the interceptor has installed a trusted Identity, the
// claim from the request payload MUST be ignored, even if it tries to
// upgrade to system: or admin:.
func TestAuthoritative_TrustedWinsOverClaim(t *testing.T) {
	cases := []struct {
		name    string
		trusted Identity
		claimed Actor
		want    Actor
	}{
		{
			name:    "bare sub becomes user:",
			trusted: Identity{Method: MethodOAuth, Subject: "alice"},
			claimed: System("admin"), // attacker tries to claim system:admin
			want:    User("alice"),
		},
		{
			name:    "prefixed sub stays prefixed",
			trusted: Identity{Method: MethodSession, Subject: "user:alice"},
			claimed: Admin("root"),
			want:    User("alice"),
		},
		{
			name:    "system identity preserved",
			trusted: Identity{Method: MethodAPIKey, Subject: "system:gdpr-worker"},
			claimed: User("eve"),
			want:    System("gdpr-worker"),
		},
		{
			name:    "admin identity preserved",
			trusted: Identity{Method: MethodOAuth, Subject: "admin:root"},
			claimed: User("eve"),
			want:    Admin("root"),
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithIdentity(context.Background(), tt.trusted)
			got := Authoritative(ctx, tt.claimed)
			if got != tt.want {
				t.Errorf("Authoritative = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAuthoritative_FallbackToClaimed pins the documented fallback:
// when no interceptor ran (bare context), the claimed actor flows
// through unchanged. This is the no-auth dev-mode / unit-test path.
func TestAuthoritative_FallbackToClaimed(t *testing.T) {
	ctx := context.Background()
	got := Authoritative(ctx, User("alice"))
	if got != User("alice") {
		t.Errorf("fallback Authoritative = %v, want User(alice)", got)
	}
	// Claimed admin in a no-auth context is also passed through; this
	// is acceptable because no-auth mode is documented as for dev/test
	// only.
	got = Authoritative(ctx, Admin("root"))
	if got != Admin("root") {
		t.Errorf("fallback Authoritative = %v, want Admin(root)", got)
	}
}

// TestAuthoritative_GroupSubjectIsTreatedAsUser pins that a group:
// prefix coming through as a credential subject is NOT trusted as a
// caller "group" identity (groups are ACL subjects only). It falls
// through to the user:<subject> wrapping.
func TestAuthoritative_GroupSubjectIsTreatedAsUser(t *testing.T) {
	id := Identity{Method: MethodSession, Subject: "group:admins"}
	ctx := WithIdentity(context.Background(), id)
	got := Authoritative(ctx, User("eve"))
	want := User("group:admins")
	if got != want {
		t.Errorf("Authoritative for group: subject = %v, want %v", got, want)
	}
}

func TestIdentityFromContext(t *testing.T) {
	// Empty context -> not found.
	if _, ok := IdentityFromContext(context.Background()); ok {
		t.Errorf("expected no identity on bare context")
	}
	// Zero identity is treated as not-set.
	ctx := WithIdentity(context.Background(), Identity{})
	if _, ok := IdentityFromContext(ctx); ok {
		t.Errorf("expected zero Identity to read as absent")
	}
	// Populated identity round-trips.
	id := Identity{Method: MethodOAuth, Subject: "alice"}
	ctx = WithIdentity(context.Background(), id)
	got, ok := IdentityFromContext(ctx)
	if !ok {
		t.Fatalf("expected identity to round-trip")
	}
	if got.Subject != "alice" || got.Method != MethodOAuth {
		t.Errorf("round-trip = %+v, want %+v", got, id)
	}
}
