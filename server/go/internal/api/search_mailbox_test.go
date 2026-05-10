// Tests for the SearchMailbox stub handler. Two cases pin the contract
// for Wave 2:
//
//  1. Happy path: known tenant -> empty SearchMailboxResponse, nil error.
//     Mirrors tests/python/integration/test_grpc_contract.py:267-273.
//
//  2. Unknown tenant: globalstore returns no row -> codes.NotFound. This
//     comes straight from tenant.CheckTenant; the handler just propagates
//     the gate's error.
//
// A real FTS5 implementation is deferred (see
// docs/go-port/rpcs/SearchMailbox.md). When it lands the empty-response
// assertion will be expanded to cover the hit shape.

package api

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/tenant"
)

func newGlobalStore(t *testing.T) *globalstore.GlobalStore {
	t.Helper()
	gs, err := globalstore.New(globalstore.Options{DataDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })
	return gs
}

// TestSearchMailbox_HappyPath: existing tenant, sharding owns it,
// region matches => stub returns SearchMailboxResponse{} with no error.
func TestSearchMailbox_HappyPath(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	srv := New(
		WithGlobalStore(gs),
		WithSharding(&tenant.Sharding{NodeID: "n1"}),
		WithRegion("us-east-1"),
	)

	req := &pb.SearchMailboxRequest{
		Context: &pb.RequestContext{TenantId: "acme"},
		UserId:  "alice",
		Query:   "hi",
	}
	resp, err := srv.SearchMailbox(ctx, req)
	if err != nil {
		t.Fatalf("SearchMailbox: unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("SearchMailbox: nil response")
	}
	if len(resp.GetResults()) != 0 {
		t.Fatalf("SearchMailbox: expected empty Results, got %d", len(resp.GetResults()))
	}
	if resp.GetHasMore() {
		t.Fatalf("SearchMailbox: expected HasMore=false")
	}
}

// TestSearchMailbox_UnknownTenant: tenant not in globalstore => the
// tenant gate returns codes.NotFound and the handler propagates it.
func TestSearchMailbox_UnknownTenant(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	srv := New(
		WithGlobalStore(gs),
		WithSharding(&tenant.Sharding{NodeID: "n1"}),
	)

	req := &pb.SearchMailboxRequest{
		Context: &pb.RequestContext{TenantId: "ghost"},
		UserId:  "alice",
		Query:   "hi",
	}
	resp, err := srv.SearchMailbox(context.Background(), req)
	if err == nil {
		t.Fatalf("SearchMailbox: expected error, got resp=%v", resp)
	}
	if got := errs.Code(err); got != codes.NotFound {
		t.Fatalf("SearchMailbox: expected NotFound, got %v (err=%v)", got, err)
	}
}
