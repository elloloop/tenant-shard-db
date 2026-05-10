// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

func TestMemoryAPIKeyManager_ValidateRoundTrip(t *testing.T) {
	m := NewMemoryAPIKeyManager()
	keyID := m.Add("plaintext-secret", "ci-bot", []string{"read", "write"})
	if keyID == "" {
		t.Fatal("expected non-empty keyID")
	}
	info, err := m.Validate(context.Background(), "plaintext-secret")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if info.Name != "ci-bot" {
		t.Errorf("Name = %q, want ci-bot", info.Name)
	}
	if len(info.Scopes) != 2 || info.Scopes[0] != "read" || info.Scopes[1] != "write" {
		t.Errorf("Scopes = %v, want [read write]", info.Scopes)
	}
}

func TestMemoryAPIKeyManager_RejectsUnknown(t *testing.T) {
	m := NewMemoryAPIKeyManager()
	_, err := m.Validate(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	if errs.Code(err) != codes.Unauthenticated {
		t.Errorf("code = %v, want Unauthenticated", errs.Code(err))
	}
}

func TestMemoryAPIKeyManager_RevokedKeyFails(t *testing.T) {
	m := NewMemoryAPIKeyManager()
	id := m.Add("s", "n", nil)
	m.Revoke(id)
	if _, err := m.Validate(context.Background(), "s"); err == nil {
		t.Fatal("expected revoked key to fail")
	}
}

func TestMemoryAPIKeyManager_EmptyKeyFails(t *testing.T) {
	m := NewMemoryAPIKeyManager()
	if _, err := m.Validate(context.Background(), ""); err == nil {
		t.Fatal("expected empty key to fail")
	}
}
