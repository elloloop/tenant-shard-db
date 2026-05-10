// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

func TestMemorySessionManager_RoundTrip(t *testing.T) {
	m := NewMemorySessionManager(time.Hour)
	tok, err := m.Create("user:alice", map[string]any{"ip": "127.0.0.1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	info, err := m.Validate(context.Background(), tok)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if info.UserID != "user:alice" {
		t.Errorf("UserID = %q", info.UserID)
	}
	if info.Metadata["ip"] != "127.0.0.1" {
		t.Errorf("Metadata = %v", info.Metadata)
	}
}

func TestMemorySessionManager_RejectsUnknown(t *testing.T) {
	m := NewMemorySessionManager(time.Hour)
	_, err := m.Validate(context.Background(), "nope")
	if err == nil || errs.Code(err) != codes.Unauthenticated {
		t.Fatalf("want UNAUTHENTICATED, got %v", err)
	}
}

func TestMemorySessionManager_Revoke(t *testing.T) {
	m := NewMemorySessionManager(time.Hour)
	tok, _ := m.Create("u", nil)
	m.Revoke(tok)
	if _, err := m.Validate(context.Background(), tok); err == nil {
		t.Fatal("expected revoked session to fail")
	}
}

func TestMemorySessionManager_Expired(t *testing.T) {
	m := NewMemorySessionManager(time.Hour)
	tok, err := m.CreateWithExpiry("u", nil, time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("CreateWithExpiry: %v", err)
	}
	if _, err := m.Validate(context.Background(), tok); err == nil {
		t.Fatal("expected expired session to fail")
	}
}
