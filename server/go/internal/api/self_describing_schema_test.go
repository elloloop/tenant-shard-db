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

// newSelfDescribingFixture wires the production-realistic SELF-DESCRIBING
// WRITES path: an EMPTY, NON-frozen process-wide registry shared by the
// store, the api server, and the applier. No types are pre-registered —
// they ride in with the writes via the register_schema WAL op and the
// applier materializes them (registry + per-tenant indexes) on apply.
func newSelfDescribingFixture(t *testing.T) *xaFixture {
	t.Helper()
	w := wal.NewInMemory(1)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}

	// One empty, runtime-mutable registry shared everywhere (same pointer).
	reg := schema.NewRegistry()

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

// userSchemaDescriptor returns a SchemaDescriptor for a node type
// (type_id=1) whose field_id=1 is unique. emailUnique toggles the
// unique flag so the conflict test can supply a divergent definition.
// Name-free per ADR-031: types/fields are id-only on the wire.
func userSchemaDescriptor(emailUnique bool) *pb.SchemaDescriptor {
	return &pb.SchemaDescriptor{
		NodeTypes: []*pb.SchemaNodeTypeDef{{
			TypeId: 1,
			Fields: []*pb.SchemaFieldDef{
				{FieldId: 1, Kind: "str", Unique: emailUnique},
				{FieldId: 2, Kind: "str"},
			},
		}},
	}
}

// createUserWithSchema builds an ExecuteAtomic request that rides the
// User schema and creates one User node. When schema is nil the request
// carries no schema (steady state).
func createUserWithSchema(idem, email string, schema *pb.SchemaDescriptor) *pb.ExecuteAtomicRequest {
	return &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: idem,
		WaitApplied:    true,
		WaitTimeoutMs:  2000,
		Schema:         schema,
		Operations: []*pb.Operation{{
			Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1,
				TypedData: map[uint32]*pb.EntValue{
					1: {V: &pb.EntValue_StringValue{StringValue: email}},
				},
			}},
		}},
	}
}

// TestSelfDescribingWrite_RegistersSchemaAndEnforcesUnique is the core
// foundation test: an ExecuteAtomic carrying a User schema (email unique)
// plus a create_node registers the type, creates the unique index, and a
// later duplicate-email create returns ALREADY_EXISTS — proving the
// unique constraint declared client-side now bites on a schema-less
// server.
func TestSelfDescribingWrite_RegistersSchemaAndEnforcesUnique(t *testing.T) {
	t.Parallel()
	f := newSelfDescribingFixture(t)
	f.runApplier(t)

	// Pre-state: the server registry is empty (schema-less); no fingerprint.
	if f.registry.NodeTypeByID(1) != nil {
		t.Fatalf("precondition: type 1 already registered")
	}

	// First write rides the schema and a create_node.
	resp, err := f.srv.ExecuteAtomic(context.Background(),
		createUserWithSchema("u-1", "alice@example.com", userSchemaDescriptor(true)))
	if err != nil {
		t.Fatalf("first ExecuteAtomic: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("first create Success=false: %q / %q", resp.GetError(), resp.GetErrorCode())
	}

	// The type is now registered with the unique email field, and the
	// server advertises a non-empty fingerprint.
	nt := f.registry.NodeTypeByID(1)
	if nt == nil {
		t.Fatalf("type 1 was not registered after the self-describing write")
	}
	if got := f.registry.UniqueFieldIDs(1); len(got) != 1 || got[0] != 1 {
		t.Fatalf("UniqueFieldIDs(1) = %v, want [1]", got)
	}
	if f.registry.Fingerprint() == "" {
		t.Fatalf("server fingerprint empty after registration")
	}

	// A duplicate email must now trip the unique constraint -> ALREADY_EXISTS.
	_, err = f.srv.ExecuteAtomic(context.Background(),
		createUserWithSchema("u-2", "alice@example.com", userSchemaDescriptor(true)))
	if err == nil {
		t.Fatalf("duplicate-email create: expected ALREADY_EXISTS, got nil")
	}
	if got := status.Code(err); got != codes.AlreadyExists {
		t.Fatalf("duplicate-email code: got %v, want AlreadyExists; msg=%q", got, status.Convert(err).Message())
	}

	// A distinct email still succeeds (the constraint is per-value, the
	// consumer did not halt).
	resp, err = f.srv.ExecuteAtomic(context.Background(),
		createUserWithSchema("u-3", "bob@example.com", userSchemaDescriptor(true)))
	if err != nil {
		t.Fatalf("distinct-email create: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("distinct-email create Success=false: %q / %q", resp.GetError(), resp.GetErrorCode())
	}
}

// TestSelfDescribingWrite_IdenticalSchemaIsNoOp pins that re-sending the
// SAME schema on a second write is an idempotent no-op: the write
// succeeds and the fingerprint is unchanged.
func TestSelfDescribingWrite_IdenticalSchemaIsNoOp(t *testing.T) {
	t.Parallel()
	f := newSelfDescribingFixture(t)
	f.runApplier(t)

	if _, err := f.srv.ExecuteAtomic(context.Background(),
		createUserWithSchema("u-1", "a@example.com", userSchemaDescriptor(true))); err != nil {
		t.Fatalf("first ExecuteAtomic: %v", err)
	}
	fp1 := f.registry.Fingerprint()
	if fp1 == "" {
		t.Fatalf("fingerprint empty after first write")
	}

	// Second write rides the byte-identical schema again.
	resp, err := f.srv.ExecuteAtomic(context.Background(),
		createUserWithSchema("u-2", "b@example.com", userSchemaDescriptor(true)))
	if err != nil {
		t.Fatalf("second ExecuteAtomic: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("second create Success=false: %q / %q", resp.GetError(), resp.GetErrorCode())
	}
	if fp2 := f.registry.Fingerprint(); fp2 != fp1 {
		t.Fatalf("fingerprint changed after identical schema: %q -> %q", fp1, fp2)
	}
}

// TestSelfDescribingWrite_ConflictingSchemaRejected pins establish-or-
// reject: a second write that redefines type_id=1 with a DIFFERENT
// definition (email no longer unique) fails the event deterministically
// (FAILED_PRECONDITION) and does NOT mutate the registered type.
func TestSelfDescribingWrite_ConflictingSchemaRejected(t *testing.T) {
	t.Parallel()
	f := newSelfDescribingFixture(t)
	f.runApplier(t)

	if _, err := f.srv.ExecuteAtomic(context.Background(),
		createUserWithSchema("u-1", "a@example.com", userSchemaDescriptor(true))); err != nil {
		t.Fatalf("first ExecuteAtomic: %v", err)
	}
	fpBefore := f.registry.Fingerprint()

	// Conflicting redefinition: same type_id/name, email NOT unique.
	resp, err := f.srv.ExecuteAtomic(context.Background(),
		createUserWithSchema("u-2", "c@example.com", userSchemaDescriptor(false)))
	if err != nil {
		// A schema conflict is surfaced in-band (success=false), not as a
		// gRPC error, mirroring the CAS-miss FAILED_PRECONDITION contract.
		t.Fatalf("conflicting ExecuteAtomic returned a transport error: %v", err)
	}
	if resp.GetSuccess() {
		t.Fatalf("conflicting schema write Success=true, want false")
	}
	if resp.GetErrorCode() != "FAILED_PRECONDITION" {
		t.Fatalf("conflicting schema error_code = %q, want FAILED_PRECONDITION", resp.GetErrorCode())
	}

	// The registered type is unchanged (email still unique) and the
	// fingerprint did not move.
	if got := f.registry.UniqueFieldIDs(1); len(got) != 1 || got[0] != 1 {
		t.Fatalf("after conflict UniqueFieldIDs(1) = %v, want [1] (unchanged)", got)
	}
	if fpAfter := f.registry.Fingerprint(); fpAfter != fpBefore {
		t.Fatalf("fingerprint changed after a rejected conflict: %q -> %q", fpBefore, fpAfter)
	}

	// The whole event aborted: the create_node that followed the rejected
	// register_schema op left no node behind (the schema op fails before
	// any data op runs, and the batch rolls back).
	if ids := resp.GetCreatedNodeIds(); len(ids) > 0 {
		if _, gerr := f.store.GetNode(context.Background(), xaTenant, ids[0]); gerr == nil {
			t.Fatalf("a node (%q) leaked from the rejected conflict event", ids[0])
		}
	}
}

// TestSelfDescribingWrite_RegistryRebuiltFromWALReplay is the crash-
// recovery property: a fresh applier + fresh EMPTY registry replaying the
// SAME WAL rebuilds the registry (types + unique index) from the
// register_schema ops on the log — proving the schema is reconstructed
// from the WAL, not a boot-loaded snapshot.
func TestSelfDescribingWrite_RegistryRebuiltFromWALReplay(t *testing.T) {
	t.Parallel()
	f := newSelfDescribingFixture(t)
	f.runApplier(t)

	// Write through the live stack so the WAL holds a register_schema +
	// create_node event.
	if _, err := f.srv.ExecuteAtomic(context.Background(),
		createUserWithSchema("u-1", "alice@example.com", userSchemaDescriptor(true))); err != nil {
		t.Fatalf("seed ExecuteAtomic: %v", err)
	}
	if f.registry.NodeTypeByID(1) == nil {
		t.Fatalf("precondition: type 1 not registered on the live stack")
	}

	// Simulate a restart: a BRAND-NEW empty registry + store + applier,
	// replaying the SAME WAL from a fresh consumer group. The registry
	// starts empty; replay must reconstruct it.
	reg2 := schema.NewRegistry()
	cs2, err := store.New(store.Options{RootDir: t.TempDir(), WALMode: true, Registry: reg2})
	if err != nil {
		t.Fatalf("store.New (replay): %v", err)
	}
	t.Cleanup(func() { _ = cs2.Close() })
	if err := cs2.OpenTenant(context.Background(), xaTenant); err != nil {
		t.Fatalf("OpenTenant (replay): %v", err)
	}
	a2, err := apply.New(apply.Options{
		Store:       cs2,
		Consumer:    f.wal, // same in-memory WAL, fresh group_id => full replay
		Topic:       xaTopic,
		GroupID:     "xa-applier-replay",
		PollTimeout: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("apply.New (replay): %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a2.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })

	// Wait for the replayed event to apply on the new store.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ok, _ := cs2.CheckIdempotency(context.Background(), xaTenant, "u-1"); ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// The replayed registry must hold the User type with the unique email
	// field — rebuilt purely from the WAL register_schema op.
	if reg2.NodeTypeByID(1) == nil {
		t.Fatalf("registry NOT rebuilt from WAL replay: type 1 missing")
	}
	if got := reg2.UniqueFieldIDs(1); len(got) != 1 || got[0] != 1 {
		t.Fatalf("replayed UniqueFieldIDs(1) = %v, want [1]", got)
	}
	if reg2.Fingerprint() == "" {
		t.Fatalf("replayed registry fingerprint empty")
	}
	// And the unique index it created bites: a duplicate create against the
	// replayed store trips ALREADY_EXISTS.
	dup := &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "u-dup",
		Operations: []*pb.Operation{{
			Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId:    1,
				TypedData: map[uint32]*pb.EntValue{1: {V: &pb.EntValue_StringValue{StringValue: "alice@example.com"}}},
			}},
		}},
	}
	// Drive the dup straight through a server bound to the replayed stack.
	srv2 := api.New(
		api.WithStore(cs2),
		api.WithWALProducer(f.wal),
		api.WithWALTopic(xaTopic),
		api.WithSchemaRegistry(reg2),
	)
	dup.WaitApplied = true
	dup.WaitTimeoutMs = 2000
	_, derr := srv2.ExecuteAtomic(context.Background(), dup)
	if derr == nil {
		t.Fatalf("duplicate on replayed store: expected ALREADY_EXISTS, got nil")
	}
	if got := status.Code(derr); got != codes.AlreadyExists {
		t.Fatalf("duplicate on replayed store code: got %v, want AlreadyExists", got)
	}
}
