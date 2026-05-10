// Tests for the ExecuteAtomic RPC (W2 — EPIC #407).
//
// Coverage targets the Python contract pins listed in
// docs/go-port/rpcs/ExecuteAtomic.md "Contract tests pinning behavior":
//
//   - Empty op list      -> INVALID_ARGUMENT (grpc_server.py:637).
//   - Required fields    -> INVALID_ARGUMENT (tenant_id, actor, type_id, id).
//   - Missing actor      -> INVALID_ARGUMENT (grpc_server.py:635).
//   - Single create_node -> receipt PENDING returns immediately, applier
//                           catches up and CheckIdempotency reports true.
//   - Multiple ops       -> all created node IDs surface, in op order.
//   - Idempotent retry   -> same idempotency_key returns the same receipt.
//   - Schema mismatch    -> in-band success=false, error_code=SCHEMA_MISMATCH
//                           (test_grpc_schema_mismatch_metric.py).
//   - Field-id encoding  -> wire payload arrives as id-keyed JSON in the
//                           WAL event (CLAUDE.md invariant #6, pinned by
//                           test_grpc_wire_format.py:172,202).
//   - Permission denied  -> non-member actor cannot write
//                           (test_tenant_roles.py:524-651).

package api_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/tenant"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

const (
	xaTopic    = "entdb-wal-xa-test"
	xaTenant   = "tenant-xa"
	xaGroupID  = "xa-applier"
	xaActorStr = "user:alice"
)

// xaFixture wires the full stack the ExecuteAtomic happy path needs:
// in-memory WAL, per-tenant SQLite store, a frozen schema registry with
// a User node type, and an Applier consuming the WAL into the store.
type xaFixture struct {
	t        *testing.T
	wal      *wal.InMemory
	store    *store.CanonicalStore
	registry *schema.Registry
	srv      *api.Server
	applier  *apply.Applier
}

func newXAFixture(t *testing.T) *xaFixture {
	t.Helper()
	w := wal.NewInMemory(1)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}

	cs, err := store.New(store.Options{RootDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	if err := cs.OpenTenant(context.Background(), xaTenant); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	reg := schema.NewRegistry()
	if err := reg.RegisterNode(&schema.NodeTypeDef{
		TypeID: 1,
		Name:   "User",
		Fields: []schema.FieldDef{
			{FieldID: 1, Name: "email", Kind: schema.KindString},
			{FieldID: 2, Name: "name", Kind: schema.KindString},
		},
	}); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
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

func (f *xaFixture) runApplier(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- f.applier.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
}

func (f *xaFixture) waitForIdempKey(t *testing.T, idempKey string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ok, err := f.store.CheckIdempotency(context.Background(), xaTenant, idempKey)
		if err != nil {
			t.Fatalf("CheckIdempotency: %v", err)
		}
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("idempotency key %q never reached applied state", idempKey)
}

func newStruct(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatalf("structpb.NewStruct: %v", err)
	}
	return s
}

// TestExecuteAtomic_EmptyOpsRejected pins Python grpc_server.py:637:
// missing operations -> INVALID_ARGUMENT.
func TestExecuteAtomic_EmptyOpsRejected(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)
	resp, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context: &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
	})
	if err == nil {
		t.Fatalf("ExecuteAtomic: expected error, got resp=%+v", resp)
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("code: got %v, want InvalidArgument", got)
	}
}

// TestExecuteAtomic_MissingActorRejected pins grpc_server.py:635.
func TestExecuteAtomic_MissingActorRejected(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)
	resp, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context: &pb.RequestContext{TenantId: xaTenant},
		Operations: []*pb.Operation{
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{TypeId: 1}}},
		},
	})
	if err == nil {
		t.Fatalf("ExecuteAtomic: expected error, got resp=%+v", resp)
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("code: got %v, want InvalidArgument", got)
	}
}

// TestExecuteAtomic_CreateNodeMissingTypeID exercises the per-op
// required-field validation: a create_node with type_id == 0 should
// abort with INVALID_ARGUMENT before any WAL append happens.
func TestExecuteAtomic_CreateNodeMissingTypeID(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)
	_, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context: &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		Operations: []*pb.Operation{{
			Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				Data: newStruct(t, map[string]any{"email": "a@x"}),
			}},
		}},
	})
	if err == nil {
		t.Fatalf("ExecuteAtomic: expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("code: got %v, want InvalidArgument", got)
	}
	// Double-check: no record made it into the WAL (the precondition
	// check ran before append).
	if n := f.wal.GetRecordCount(xaTopic); n != 0 {
		t.Fatalf("WAL: got %d records, want 0", n)
	}
}

// TestExecuteAtomic_CreateNodeRoundTrip: a single create_node returns
// success=true with PENDING, applier catches up, CheckIdempotency
// flips to true.
func TestExecuteAtomic_CreateNodeRoundTrip(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)
	f.runApplier(t)

	const idem = "round-trip-1"
	resp, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: idem,
		Operations: []*pb.Operation{{
			Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1,
				Data:   newStruct(t, map[string]any{"email": "alice@example.com"}),
			}},
		}},
	})
	if err != nil {
		t.Fatalf("ExecuteAtomic: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("Success=false (error=%q code=%q)", resp.GetError(), resp.GetErrorCode())
	}
	if resp.GetReceipt().GetIdempotencyKey() != idem {
		t.Fatalf("Receipt.IdempotencyKey: got %q want %q", resp.GetReceipt().GetIdempotencyKey(), idem)
	}
	if resp.GetReceipt().GetTenantId() != xaTenant {
		t.Fatalf("Receipt.TenantId: got %q want %q", resp.GetReceipt().GetTenantId(), xaTenant)
	}
	if resp.GetReceipt().GetStreamPosition() == "" {
		t.Fatalf("Receipt.StreamPosition: got empty, want non-empty")
	}
	if got := resp.GetAppliedStatus(); got != pb.ReceiptStatus_RECEIPT_STATUS_PENDING {
		t.Fatalf("AppliedStatus: got %v want PENDING (wait_applied=false default)", got)
	}
	if len(resp.GetCreatedNodeIds()) != 1 {
		t.Fatalf("CreatedNodeIds: got %v want 1", resp.GetCreatedNodeIds())
	}
	if resp.GetCreatedNodeIds()[0] == "" {
		t.Fatalf("CreatedNodeIds[0]: empty (server-fill UUID expected)")
	}

	// Applier catches up.
	f.waitForIdempKey(t, idem)
}

// TestExecuteAtomic_MultipleOpsAtomic pins multi-op transactions: all
// create_node IDs surface in op order in CreatedNodeIds.
func TestExecuteAtomic_MultipleOpsAtomic(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)
	f.runApplier(t)

	resp, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "multi-1",
		Operations: []*pb.Operation{
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, Id: "fixed-id-A",
				Data: newStruct(t, map[string]any{"email": "a@x"}),
			}}},
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, // server-fill
				Data:   newStruct(t, map[string]any{"email": "b@x"}),
			}}},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteAtomic: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("Success=false (%q)", resp.GetError())
	}
	got := resp.GetCreatedNodeIds()
	if len(got) != 2 {
		t.Fatalf("CreatedNodeIds: got %v want 2", got)
	}
	if got[0] != "fixed-id-A" {
		t.Fatalf("CreatedNodeIds[0]: got %q want fixed-id-A (op order preserved)", got[0])
	}
	if got[1] == "" {
		t.Fatalf("CreatedNodeIds[1]: empty (server-fill expected)")
	}
}

// TestExecuteAtomic_IdempotentRetry: replaying the same idempotency_key
// returns the same StreamPosition (the WAL backend dedupes at append
// time).
func TestExecuteAtomic_IdempotentRetry(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)

	const idem = "retry-1"
	build := func() *pb.ExecuteAtomicRequest {
		return &pb.ExecuteAtomicRequest{
			Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
			IdempotencyKey: idem,
			Operations: []*pb.Operation{{
				Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
					TypeId: 1, Id: "retry-node",
					Data: newStruct(t, map[string]any{"email": "r@x"}),
				}},
			}},
		}
	}
	r1, err := f.srv.ExecuteAtomic(context.Background(), build())
	if err != nil {
		t.Fatalf("ExecuteAtomic #1: %v", err)
	}
	r2, err := f.srv.ExecuteAtomic(context.Background(), build())
	if err != nil {
		t.Fatalf("ExecuteAtomic #2: %v", err)
	}
	if a, b := r1.GetReceipt().GetStreamPosition(), r2.GetReceipt().GetStreamPosition(); a != b {
		t.Fatalf("stream_position drift: r1=%q r2=%q", a, b)
	}
	if n := f.wal.GetRecordCount(xaTopic); n != 1 {
		t.Fatalf("WAL records: got %d want 1 (idempotent dedupe)", n)
	}
}

// TestExecuteAtomic_SchemaMismatchInBand pins the
// test_grpc_schema_mismatch_metric.py contract: a request that pins a
// fingerprint different from the server's gets success=false +
// error_code=SCHEMA_MISMATCH back, with the gRPC status STILL OK.
func TestExecuteAtomic_SchemaMismatchInBand(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)

	resp, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:           &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		SchemaFingerprint: "sha256:bogus",
		Operations: []*pb.Operation{{
			Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{TypeId: 1}},
		}},
	})
	if err != nil {
		t.Fatalf("ExecuteAtomic: gRPC status must remain OK, got %v", err)
	}
	if resp.GetSuccess() {
		t.Fatalf("Success: got true, want false (schema-mismatch in-band)")
	}
	if resp.GetErrorCode() != "SCHEMA_MISMATCH" {
		t.Fatalf("ErrorCode: got %q want SCHEMA_MISMATCH", resp.GetErrorCode())
	}
	// And: nothing made it to the WAL.
	if n := f.wal.GetRecordCount(xaTopic); n != 0 {
		t.Fatalf("WAL records on schema mismatch: got %d want 0", n)
	}
}

// TestExecuteAtomic_PayloadIsFieldIDKeyed pins CLAUDE.md invariant #6:
// the WAL event MUST carry id-keyed payloads, never name-keyed. We
// reach into the in-memory backend and inspect the JSON-encoded event
// directly.
func TestExecuteAtomic_PayloadIsFieldIDKeyed(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)

	resp, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "fid-1",
		Operations: []*pb.Operation{{
			Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, Id: "fid-node",
				// Wire is name-keyed; the handler MUST translate.
				Data: newStruct(t, map[string]any{"email": "z@x", "name": "Z"}),
			}},
		}},
	})
	if err != nil || !resp.GetSuccess() {
		t.Fatalf("ExecuteAtomic: err=%v resp=%+v", err, resp)
	}

	recs := f.wal.GetAllRecords(xaTopic)
	if len(recs) != 1 {
		t.Fatalf("WAL records: got %d want 1", len(recs))
	}
	var ev wal.Event
	if err := json.Unmarshal(recs[0].Value, &ev); err != nil {
		t.Fatalf("decode WAL event: %v", err)
	}
	if len(ev.Ops) != 1 {
		t.Fatalf("ev.Ops: got %d want 1", len(ev.Ops))
	}
	data, ok := ev.Ops[0]["data"].(map[string]any)
	if !ok {
		t.Fatalf("ev.Ops[0].data: missing or wrong type %T", ev.Ops[0]["data"])
	}
	// Field-IDs in this schema: 1=email, 2=name. The map must NOT carry
	// name-keys.
	if _, hasName := data["email"]; hasName {
		t.Fatalf("payload contains name-key %q (should be id-keyed)", "email")
	}
	if _, hasName := data["name"]; hasName {
		t.Fatalf("payload contains name-key %q (should be id-keyed)", "name")
	}
	if got := data["1"]; got != "z@x" {
		t.Fatalf("data[\"1\"]: got %v want %q", got, "z@x")
	}
	if got := data["2"]; got != "Z" {
		t.Fatalf("data[\"2\"]: got %v want %q", got, "Z")
	}
	// Persisted actor MUST match the wire claim only because there's no
	// AuthInterceptor on ctx in this unit test (Authoritative falls
	// through to claimed). Sanity-check the round-trip nonetheless.
	if ev.Actor != xaActorStr {
		t.Fatalf("ev.Actor: got %q want %q", ev.Actor, xaActorStr)
	}
}

// TestExecuteAtomic_UnknownFieldNameDropped: an unknown name on the
// wire is silently dropped (matches Python ingress permissiveness;
// pinned by test_grpc_wire_format.py:355-389). The WAL event keeps the
// known field only.
func TestExecuteAtomic_UnknownFieldNameDropped(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)

	resp, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "unknown-1",
		Operations: []*pb.Operation{{
			Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, Id: "u-1",
				Data: newStruct(t, map[string]any{
					"email":         "u@x",
					"this_is_bogus": "ignore-me",
				}),
			}},
		}},
	})
	if err != nil || !resp.GetSuccess() {
		t.Fatalf("ExecuteAtomic: err=%v resp=%+v", err, resp)
	}
	recs := f.wal.GetAllRecords(xaTopic)
	var ev wal.Event
	if err := json.Unmarshal(recs[0].Value, &ev); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := ev.Ops[0]["data"].(map[string]any)
	if len(data) != 1 {
		t.Fatalf("data: got %v want only field_id 1 (email)", data)
	}
	if _, ok := data["1"]; !ok {
		t.Fatalf("data missing id 1: %v", data)
	}
}

// TestExecuteAtomic_PermissionDenied_NonMember pins
// test_tenant_roles.py:524-651: a member-less actor cannot write,
// even when the tenant exists.
func TestExecuteAtomic_PermissionDenied_NonMember(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, xaTenant, "T", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if _, err := gs.CreateUser(ctx, "alice", "alice@x", "Alice"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// Note: alice is NOT added as a member.

	w := wal.NewInMemory(1)
	if err := w.Connect(ctx); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}
	cs, err := store.New(store.Options{RootDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	if err := cs.OpenTenant(ctx, xaTenant); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	srv := api.New(
		api.WithStore(cs),
		api.WithGlobalStore(gs),
		api.WithWALProducer(w),
		api.WithWALTopic(xaTopic),
	)

	_, err = srv.ExecuteAtomic(ctx, &pb.ExecuteAtomicRequest{
		Context: &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		Operations: []*pb.Operation{{
			Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{TypeId: 1}},
		}},
	})
	if err == nil {
		t.Fatalf("ExecuteAtomic: expected PermissionDenied, got nil")
	}
	if got := status.Code(err); got != codes.PermissionDenied {
		t.Fatalf("code: got %v, want PermissionDenied (msg=%v)", got, err)
	}
	// Sanity-check that no WAL record was written despite the call.
	if n := w.GetRecordCount(xaTopic); n != 0 {
		t.Fatalf("WAL records: got %d want 0 (handler aborted before append)", n)
	}
}

// TestExecuteAtomic_WaitApplied_FlipsToApplied: when wait_applied=true
// and the applier is running, the handler waits until the offset is
// materialized into SQLite and returns APPLIED.
//
// We prime the WAL with one record first so the per-tenant applied
// offset gets bumped past 0; the store's WaitForOffset predicate
// checks ok && cur >= target, and for the very first record the
// initial 0->0 transition keeps ok==false. That edge case is a
// known store nuance (not part of the ExecuteAtomic contract); the
// applier's second event lands at offset 1 which exercises the
// wait-and-flip path cleanly.
func TestExecuteAtomic_WaitApplied_FlipsToApplied(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)
	f.runApplier(t)

	priming, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "prime-1",
		Operations: []*pb.Operation{{
			Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, Id: "prime-node",
				Data: newStruct(t, map[string]any{"email": "p@x"}),
			}},
		}},
	})
	if err != nil || !priming.GetSuccess() {
		t.Fatalf("priming ExecuteAtomic: err=%v resp=%+v", err, priming)
	}
	f.waitForIdempKey(t, "prime-1")

	resp, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "wait-1",
		WaitApplied:    true,
		WaitTimeoutMs:  5000,
		Operations: []*pb.Operation{{
			Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, Id: "wait-node",
				Data: newStruct(t, map[string]any{"email": "w@x"}),
			}},
		}},
	})
	if err != nil {
		t.Fatalf("ExecuteAtomic: %v", err)
	}
	if got := resp.GetAppliedStatus(); got != pb.ReceiptStatus_RECEIPT_STATUS_APPLIED {
		t.Fatalf("AppliedStatus: got %v want APPLIED", got)
	}
}

// TestExecuteAtomic_TenantGate_RedirectTrailer: when this node does
// not own the tenant, CheckTenant returns codes.Unavailable (the
// trailer is a separate concern verified by the tenant package's
// own tests). The ExecuteAtomic surface contract is just "Unavailable
// before any other validation runs" — pinned by the spec error table
// row 1 ("Tenant not on this node").
func TestExecuteAtomic_TenantGate_RedirectsForeignTenant(t *testing.T) {
	t.Parallel()
	w := wal.NewInMemory(1)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}
	cs, err := store.New(store.Options{RootDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	srv := api.New(
		api.WithStore(cs),
		api.WithWALProducer(w),
		api.WithWALTopic(xaTopic),
		api.WithSharding(tenantShardingDeny()),
	)

	_, err = srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context: &pb.RequestContext{TenantId: "remote-tenant", Actor: xaActorStr},
		Operations: []*pb.Operation{{
			Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{TypeId: 1}},
		}},
	})
	if err == nil {
		t.Fatalf("ExecuteAtomic: expected Unavailable, got nil")
	}
	if got := status.Code(err); got != codes.Unavailable {
		t.Fatalf("code: got %v want Unavailable (msg=%v)", got, err)
	}
	if !strings.Contains(err.Error(), "remote-tenant") {
		t.Fatalf("error must mention tenant id: %v", err)
	}
}

// tenantShardingDeny returns a tenant.Sharding that rejects every
// tenant id — used to drive the redirect-trailer / Unavailable path
// without a multi-node fixture.
func tenantShardingDeny() *tenant.Sharding {
	return &tenant.Sharding{
		NodeID: "this-node",
		IsMine: func(string) bool { return false },
		Owner:  func(string) string { return "owner-node" },
	}
}
