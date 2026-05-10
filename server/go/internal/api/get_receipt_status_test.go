// Tests for the GetReceiptStatus RPC (W2.03 — EPIC #407).
//
// Pinning these three cases mirrors the Python contract tests at
// tests/python/integration/test_grpc_contract.py:313-326 (APPLIED for
// a recorded key, PENDING for an unknown key) and the spec error
// contract at docs/go-port/rpcs/GetReceiptStatus.md (any store fault
// collapses to status=UNKNOWN + error, with the RPC returning OK).

package api

import (
	"context"
	"strings"
	"testing"

	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// newTestStore returns a tmpdir-backed CanonicalStore with WAL on. The
// store is closed automatically on test teardown.
func newTestStore(t *testing.T) *store.CanonicalStore {
	t.Helper()
	cs, err := store.New(store.Options{
		RootDir: t.TempDir(),
		WALMode: true,
	})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// receiptReq is a small builder so each test case reads as the actual
// wire payload it exercises.
func receiptReq(tenantID, key string) *pb.GetReceiptStatusRequest {
	return &pb.GetReceiptStatusRequest{
		Context:        &pb.RequestContext{TenantId: tenantID},
		IdempotencyKey: key,
	}
}

// Case 1: receipt for a key that the applier has recorded returns
// APPLIED + empty error. Pinned by spec "Error contract" row 6.
func TestGetReceiptStatus_AppliedKeyReturnsApplied(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cs := newTestStore(t)
	const tenantID = "tenant-a"
	const key = "req-applied-1"

	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	if err := cs.RecordIdempotency(ctx, tenantID, key, "wal-topic:0:42"); err != nil {
		t.Fatalf("RecordIdempotency: %v", err)
	}

	srv := New(WithStore(cs))
	resp, err := srv.GetReceiptStatus(ctx, receiptReq(tenantID, key))
	if err != nil {
		t.Fatalf("GetReceiptStatus: unexpected gRPC error: %v", err)
	}
	if resp.GetStatus() != pb.ReceiptStatus_RECEIPT_STATUS_APPLIED {
		t.Fatalf("status: got %v, want APPLIED", resp.GetStatus())
	}
	if resp.GetError() != "" {
		t.Fatalf("error: got %q, want empty (mutually exclusive with non-UNKNOWN status)", resp.GetError())
	}
}

// Case 2: receipt for a key never recorded returns PENDING + empty
// error. Pinned by spec "Error contract" row 5 and the Python contract
// test's "never-issued" assertion (test_grpc_contract.py:325-326).
func TestGetReceiptStatus_UnknownKeyReturnsPending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cs := newTestStore(t)
	const tenantID = "tenant-b"

	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	srv := New(WithStore(cs))
	resp, err := srv.GetReceiptStatus(ctx, receiptReq(tenantID, "never-issued"))
	if err != nil {
		t.Fatalf("GetReceiptStatus: unexpected gRPC error: %v", err)
	}
	if resp.GetStatus() != pb.ReceiptStatus_RECEIPT_STATUS_PENDING {
		t.Fatalf("status: got %v, want PENDING", resp.GetStatus())
	}
	if resp.GetError() != "" {
		t.Fatalf("error: got %q, want empty", resp.GetError())
	}
}

// Case 3: a store-internal error (here: the tenant DB has never been
// opened, so the read-side pool lookup returns ErrTenantNotOpen) must
// surface as status=UNKNOWN + error=<msg> with NO gRPC error. This is
// the Python `except Exception` -> UNKNOWN collapse documented in the
// spec "Error contract" row 4.
func TestGetReceiptStatus_StoreErrorReturnsUnknownNotGRPC(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cs := newTestStore(t)
	const tenantID = "tenant-c"

	// Deliberately skip OpenTenant — CheckIdempotency will fail with
	// ErrTenantNotOpen and the handler must collapse that to UNKNOWN.
	srv := New(WithStore(cs))
	resp, err := srv.GetReceiptStatus(ctx, receiptReq(tenantID, "any-key"))
	if err != nil {
		t.Fatalf("GetReceiptStatus: must return OK + UNKNOWN body, not gRPC error %v", err)
	}
	if resp.GetStatus() != pb.ReceiptStatus_RECEIPT_STATUS_UNKNOWN {
		t.Fatalf("status: got %v, want UNKNOWN", resp.GetStatus())
	}
	if resp.GetError() == "" {
		t.Fatalf("error: got empty, want non-empty (Python populates `error` on UNKNOWN)")
	}
	// Sanity-check that the error message refers to the underlying
	// fault rather than being a generic placeholder. We don't pin the
	// exact string — the Python contract is just "non-empty".
	if !strings.Contains(resp.GetError(), tenantID) && !strings.Contains(resp.GetError(), "tenant") {
		t.Logf("error message: %q (no tenant context; allowed but worth noting)", resp.GetError())
	}
}
