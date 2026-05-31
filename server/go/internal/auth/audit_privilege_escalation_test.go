// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
)

// FINDING #2 (HIGH / security): Privilege escalation from credential subject prefix.
//
// File: internal/auth/authoritative.go:47-64 (Authoritative) and
//       internal/auth/mtls.go:29-60 (IdentityFromCertificate).
//
// WHY THIS IS WRONG:
//   Authoritative() ignores Identity.Method entirely. It feeds the
//   interceptor-attested Subject straight into ParseActor; if that string
//   carries a "system:" / "admin:" prefix it is returned VERBATIM as a
//   KindSystem / KindAdmin actor — regardless of how the subject was
//   attested. A JWT `sub` (or the email fallback) of "admin:root" delivered
//   over MethodOAuth therefore resolves to Admin("root"), and "system:x"
//   resolves to System("x"). JWT sub / email is attacker-influenceable at
//   many IdPs (self-service signup, email-as-subject, etc.), so a tenant
//   user can mint a god-mode cross-tenant identity simply by choosing the
//   right subject string. The privilege class of an actor must be derived
//   from HOW it was authenticated (the Method / trust anchor), never from a
//   string the credential itself can choose.
//
//   Compounding it, IdentityFromCertificate() hardcodes
//   Subject = "system:" + <cert subject> for EVERY verified client cert, so
//   any leaf chaining to the trusted CA — even one minted for a low-trust
//   workload — is promoted to a system actor by Authoritative().
//
// FIXED (PR for finding #2): privilege is now derived from the trust anchor
// (Identity.Method) via methodAttestsPrefix, not from the subject string.
// The two tests below — previously skipped regression gates — are now active
// and assert the secure behaviour. The earlier "_DemonstratesFinding2_*"
// tests that pinned the buggy escalation were removed when the fix landed;
// their negative is now covered here.

// Test_Authoritative_SecureBehavior_Finding2 documents the correct behaviour:
// a credential whose Method is MethodOAuth / MethodMTLS and whose Subject
// carries a privileged (admin:/system:/group:) prefix MUST resolve to a
// KindUser actor — the privilege class must come from the trust anchor
// (Method), never from an attacker-influenceable subject string. Only a
// genuinely privileged authentication path may yield KindSystem / KindAdmin.
func Test_Authoritative_SecureBehavior_Finding2(t *testing.T) {
	// Fixed in authoritative.go: privilege is derived from Identity.Method,
	// not from a prefix the credential can choose.

	// OAuth-attested subjects must NOT be able to self-elevate via prefix.
	escalationCases := []struct {
		name    string
		method  string
		subject string
	}{
		{name: "oauth admin prefix", method: MethodOAuth, subject: "admin:root"},
		{name: "oauth system prefix", method: MethodOAuth, subject: "system:x"},
		{name: "oauth group prefix", method: MethodOAuth, subject: "group:admins"},
		{name: "mtls admin prefix", method: MethodMTLS, subject: "admin:root"},
		{name: "mtls system prefix", method: MethodMTLS, subject: "system:x"},
	}
	for _, tc := range escalationCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := WithIdentity(context.Background(), Identity{
				Method:  tc.method,
				Subject: tc.subject,
			})
			got := Authoritative(ctx, User("eve"))
			if got.Kind() != KindUser {
				t.Errorf("Authoritative(%s %q) kind = %v, want KindUser (no self-elevation)",
					tc.method, tc.subject, got.Kind())
			}
			if got.IsAdmin() || got.IsSystem() {
				t.Errorf("Authoritative(%s %q) = %v escalated to privileged actor; must resolve to a user",
					tc.method, tc.subject, got)
			}
		})
	}
}

// Test_IdentityFromCertificate_SecureBehavior_Finding2 documents that a plain
// verified client cert must NOT be promoted to a system actor by default. The
// derived Identity must resolve through Authoritative to a KindUser actor
// unless an explicit, configured privilege mapping (out of scope for this
// gate) elevates it.
func Test_IdentityFromCertificate_SecureBehavior_Finding2(t *testing.T) {
	// Fixed in mtls.go + authoritative.go: a client cert resolves to a plain
	// user actor, never system/admin by default.

	cert := &x509.Certificate{
		Subject:      pkix.Name{CommonName: "some-svc"},
		SerialNumber: big.NewInt(1),
	}
	id, ok := IdentityFromCertificate(cert)
	if !ok {
		t.Fatalf("IdentityFromCertificate ok=false, want true")
	}
	got := Authoritative(WithIdentity(context.Background(), id), User("eve"))
	if got.Kind() != KindUser {
		t.Errorf("Authoritative(default cert identity) kind = %v, want KindUser by default", got.Kind())
	}
	if got.IsSystem() || got.IsAdmin() {
		t.Errorf("Authoritative(default cert identity) = %v elevated to privileged actor; a cert must not grant system/admin by default", got)
	}
}
