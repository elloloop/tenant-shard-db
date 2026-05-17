package store

import (
	"context"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

// Notification stubs.
//
// SearchMailbox / GetMailbox / ListMailboxUsers are deprecated and
// return empty results (see docs/go-port/rpcs/SearchMailbox.md). The
// Go server keeps the surface for forward-compat with existing gRPC
// handlers but does not write to or read from the deprecated
// notifications / read_cursors tables — the DDL for them is dropped
// (see schema.go).
//
// Unimplemented mailbox writes intentionally return ErrUnimplemented so
// any caller that accidentally exercises them surfaces a clear gRPC
// status rather than silently succeeding.

// MailboxItem is the placeholder shape for a mailbox / notifications
// entry. Carried only so the gRPC handler signatures compile when the
// console schema references it. No fields are populated.
type MailboxItem struct{}

// SearchMailbox is a deprecated stub. Always returns nil, nil.
func (s *CanonicalStore) SearchMailbox(ctx context.Context, tenantID, userID, query string, limit int) ([]MailboxItem, error) {
	return nil, nil
}

// GetMailbox is a deprecated stub. Always returns nil, nil.
func (s *CanonicalStore) GetMailbox(ctx context.Context, tenantID, userID string, limit, offset int) ([]MailboxItem, error) {
	return nil, nil
}

// ListMailboxUsers is a deprecated stub. Always returns nil, nil.
func (s *CanonicalStore) ListMailboxUsers(ctx context.Context, tenantID string) ([]string, error) {
	return nil, nil
}

// CreateNotification is intentionally unimplemented. Returning a
// concrete sentinel rather than silently succeeding makes the
// deprecation visible at the gRPC boundary.
func (s *CanonicalStore) CreateNotification(ctx context.Context, tenantID, userID, body string) error {
	return errs.ErrUnimplemented
}
