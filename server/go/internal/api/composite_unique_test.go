// SPDX-License-Identifier: AGPL-3.0-only

package api_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// newCompositeFixture wires the standard ExecuteAtomic harness with an
// OAuthIdentity(type_id=201) node type carrying a (provider,
// provider_user_id) composite unique constraint (issue #566).
func newCompositeFixture(t *testing.T) *xaFixture {
	t.Helper()
	w := wal.NewInMemory(1)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}

	reg := schema.NewRegistry()
	if err := reg.RegisterNode(&schema.NodeTypeDef{
		TypeID: 201,
		Fields: []schema.FieldDef{
			{FieldID: 1, Kind: schema.KindString},
			{FieldID: 2, Kind: schema.KindString},
		},
		CompositeUnique: []schema.CompositeUniqueDef{
			{FieldIDs: []uint32{1, 2}},
		},
	}); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}

	cs, err := store.New(store.Options{RootDir: t.TempDir(), WALMode: true, Registry: reg})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	if err := cs.OpenTenant(context.Background(), xaTenant); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	srv := api.New(
		api.WithStore(cs),
		api.WithWALProducer(w),
		api.WithWALTopic(xaTopic),
		api.WithSchemaRegistry(reg),
	)
	a, err := apply.New(apply.Options{
		Store:       cs,
		Consumer:    w,
		Topic:       xaTopic,
		GroupID:     xaGroupID,
		PollTimeout: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}
	return &xaFixture{t: t, wal: w, store: cs, registry: reg, srv: srv, applier: a}
}

func createOAuthIdentity(idem, provider, providerUserID string) *pb.ExecuteAtomicRequest {
	return &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: idem,
		WaitApplied:    true,
		WaitTimeoutMs:  2000,
		Operations: []*pb.Operation{{
			Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 201,
				TypedData: map[uint32]*pb.EntValue{
					1: {V: &pb.EntValue_StringValue{StringValue: provider}},
					2: {V: &pb.EntValue_StringValue{StringValue: providerUserID}},
				},
			}},
		}},
	}
}

// TestExecuteAtomic_CompositeUniqueViolation is the end-to-end handler
// test: the second create of the same (provider, provider_user_id)
// tuple surfaces as a gRPC ALREADY_EXISTS carrying the verbatim
// structured detail the SDK parsers consume (issue #566).
func TestExecuteAtomic_CompositeUniqueViolation(t *testing.T) {
	t.Parallel()
	f := newCompositeFixture(t)
	f.runApplier(t)

	// First create wins.
	resp, err := f.srv.ExecuteAtomic(context.Background(), createOAuthIdentity("oa-1", "google", "uid-1"))
	if err != nil {
		t.Fatalf("first ExecuteAtomic: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("first create Success=false: %q / %q", resp.GetError(), resp.GetErrorCode())
	}

	// Second create of the same tuple must surface ALREADY_EXISTS.
	_, err = f.srv.ExecuteAtomic(context.Background(), createOAuthIdentity("oa-2", "google", "uid-1"))
	if err == nil {
		t.Fatalf("duplicate ExecuteAtomic: expected ALREADY_EXISTS error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error is not a gRPC status: %v", err)
	}
	if st.Code() != codes.AlreadyExists {
		t.Fatalf("code: got %v, want AlreadyExists; msg=%q", st.Code(), st.Message())
	}
	want := "Composite unique constraint violation: tenant=" + xaTenant + " type_id=201 " +
		"constraint='(1,2)' fields=[1, 2] values=['google', 'uid-1'] already exists"
	if st.Message() != want {
		t.Fatalf("detail mismatch:\n got=%q\nwant=%q", st.Message(), want)
	}
}
