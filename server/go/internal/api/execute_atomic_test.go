// Tests for the ExecuteAtomic RPC (W2 — EPIC #407).
//
// Coverage targets the Python contract pins listed in
// docs/go-port/rpcs/ExecuteAtomic.md "Contract tests pinning behavior":
//
//   - Empty op list -> INVALID_ARGUMENT (grpc_server.py:637).
//   - Required fields -> INVALID_ARGUMENT (tenant_id, actor, type_id, id).
//   - Missing actor -> INVALID_ARGUMENT (grpc_server.py:635).
//   - Single create_node -> receipt PENDING returns immediately, applier
//                           catches up and CheckIdempotency reports true.
//   - Multiple ops -> all created node IDs surface, in op order.
//   - Idempotent retry -> same idempotency_key returns the same receipt.
//   - Schema mismatch -> in-band success=false, error_code=SCHEMA_MISMATCH
//                           (test_grpc_schema_mismatch_metric.py).
//   - Field-id encoding -> wire payload arrives as id-keyed JSON in the
//                           WAL event (CLAUDE.md invariant #6, pinned by
//                           test_grpc_wire_format.py:172,202).
//   - Permission denied -> non-member actor cannot write
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

func newSchemalessXAFixture(t *testing.T) *xaFixture {
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

	srv := api.New(
		api.WithStore(cs),
		api.WithWALProducer(w),
		api.WithWALTopic(xaTopic),
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
	return &xaFixture{t: t, wal: w, store: cs, srv: srv, applier: a}
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

func newValue(t *testing.T, v any) *structpb.Value {
	t.Helper()
	out, err := structpb.NewValue(v)
	if err != nil {
		t.Fatalf("structpb.NewValue: %v", err)
	}
	return out
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

func TestExecuteAtomic_PreconditionFieldIDWorksWithoutSchemaRegistry(t *testing.T) {
	t.Parallel()
	f := newSchemalessXAFixture(t)
	f.runApplier(t)

	ctx := context.Background()
	if _, err := f.store.CreateNodeRaw(ctx, xaTenant, store.NodeInput{
		NodeID:     "cas-schemaless-ok",
		TypeID:     525,
		OwnerActor: xaActorStr,
		Payload:    map[string]any{"1": "old@x"},
	}); err != nil {
		t.Fatalf("CreateNodeRaw: %v", err)
	}

	resp, err := f.srv.ExecuteAtomic(ctx, &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "cas-schemaless-ok-1",
		Operations: []*pb.Operation{{
			Op: &pb.Operation_UpdateNode{UpdateNode: &pb.UpdateNodeOp{
				TypeId: 525,
				Id:     "cas-schemaless-ok",
				Patch:  newStruct(t, map[string]any{"1": "new@x"}),
				Precondition: &pb.UpdateNodePrecondition{
					Field:   "email",
					FieldId: 1,
					Equals:  newValue(t, "old@x"),
				},
			}},
		}},
		WaitApplied:   true,
		WaitTimeoutMs: 2000,
	})
	if err != nil || !resp.GetSuccess() {
		t.Fatalf("ExecuteAtomic: err=%v resp=%+v", err, resp)
	}
	if resp.GetAppliedStatus() != pb.ReceiptStatus_RECEIPT_STATUS_APPLIED {
		t.Fatalf("applied_status=%v want APPLIED", resp.GetAppliedStatus())
	}
	n, err := f.store.GetNode(ctx, xaTenant, "cas-schemaless-ok")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(n.PayloadJSON), &payload); err != nil {
		t.Fatalf("payload decode: %v", err)
	}
	if got := payload["1"]; got != "new@x" {
		t.Fatalf("payload[1]=%v want new@x", got)
	}
}

func TestExecuteAtomic_PreconditionFieldIDFailureWithoutSchemaRegistry(t *testing.T) {
	t.Parallel()
	f := newSchemalessXAFixture(t)
	f.runApplier(t)

	ctx := context.Background()
	if _, err := f.store.CreateNodeRaw(ctx, xaTenant, store.NodeInput{
		NodeID:     "cas-schemaless-miss",
		TypeID:     525,
		OwnerActor: xaActorStr,
		Payload:    map[string]any{"1": "old@x"},
	}); err != nil {
		t.Fatalf("CreateNodeRaw: %v", err)
	}

	resp, err := f.srv.ExecuteAtomic(ctx, &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "cas-schemaless-miss-1",
		Operations: []*pb.Operation{{
			Op: &pb.Operation_UpdateNode{UpdateNode: &pb.UpdateNodeOp{
				TypeId: 525,
				Id:     "cas-schemaless-miss",
				Patch:  newStruct(t, map[string]any{"1": "new@x"}),
				Precondition: &pb.UpdateNodePrecondition{
					Field:   "email",
					FieldId: 1,
					Equals:  newValue(t, "wrong@x"),
				},
			}},
		}},
		WaitApplied:   true,
		WaitTimeoutMs: 2000,
	})
	if err != nil {
		t.Fatalf("ExecuteAtomic: %v", err)
	}
	if resp.GetSuccess() {
		t.Fatalf("success=true want false")
	}
	if resp.GetAppliedStatus() != pb.ReceiptStatus_RECEIPT_STATUS_FAILED_PRECONDITION {
		t.Fatalf("applied_status=%v want FAILED_PRECONDITION", resp.GetAppliedStatus())
	}
	if resp.GetErrorCode() != "FAILED_PRECONDITION" {
		t.Fatalf("error_code=%q want FAILED_PRECONDITION", resp.GetErrorCode())
	}
	pf := resp.GetPreconditionFailure()
	if pf == nil {
		t.Fatalf("precondition_failure is nil")
	}
	if pf.GetField() != "email" {
		t.Fatalf("field=%q want email", pf.GetField())
	}
	if pf.GetExpected().GetStringValue() != "wrong@x" {
		t.Fatalf("expected=%q want wrong@x", pf.GetExpected().GetStringValue())
	}
	if pf.GetObserved().GetStringValue() != "old@x" {
		t.Fatalf("observed=%q want old@x", pf.GetObserved().GetStringValue())
	}
	n, err := f.store.GetNode(ctx, xaTenant, "cas-schemaless-miss")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(n.PayloadJSON), &payload); err != nil {
		t.Fatalf("payload decode: %v", err)
	}
	if got := payload["1"]; got != "old@x" {
		t.Fatalf("payload[1]=%v want old@x", got)
	}
}

func TestExecuteAtomic_NameOnlyPreconditionRejectedWithoutSchemaRegistry(t *testing.T) {
	t.Parallel()
	f := newSchemalessXAFixture(t)

	_, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "cas-schemaless-name-only-1",
		Operations: []*pb.Operation{{
			Op: &pb.Operation_UpdateNode{UpdateNode: &pb.UpdateNodeOp{
				TypeId: 525,
				Id:     "cas-schemaless-name-only",
				Patch:  newStruct(t, map[string]any{"1": "new@x"}),
				Precondition: &pb.UpdateNodePrecondition{
					Field:  "email",
					Equals: newValue(t, "old@x"),
				},
			}},
		}},
	})
	if err == nil {
		t.Fatalf("ExecuteAtomic: expected InvalidArgument, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("code=%v want InvalidArgument", got)
	}
	if !strings.Contains(err.Error(), "precondition.field_id is required") {
		t.Fatalf("error=%q want field_id guidance", err.Error())
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

// =============================================================================
// W7 — $alias resolution at gRPC ingress.
//
// The Python SDK emits {"alias_ref": "$charlie"} for edge endpoints
// declared with as_="charlie" on a sibling create_node. Before the W7
// fix the Go server forwarded that ref untouched to the WAL, the
// applier read op["from"] as an empty string (it's a map, not a
// string), and halted with "poison event: create_edge missing from/to"
// — which then took every subsequent RPC down with UNAVAILABLE because
// the applier's consumer loop was dead.
//
// These tests pin the new behavior: aliases are resolved against a
// per-transaction map at the handler, the WAL only ever sees bare
// node ids, and unresolved aliases fail in-band with
// INVALID_ARGUMENT instead of poisoning the applier.
// =============================================================================

// aliasRefEdge is the shape grpc_server.py:_convert_node_ref produces
// for an alias_ref-typed NodeRef. We hand-roll the op so tests don't
// need a registered edge type in the schema.
func aliasRefEdge(edgeID int32, fromAlias, toAlias string) *pb.Operation {
	return &pb.Operation{Op: &pb.Operation_CreateEdge{
		CreateEdge: &pb.CreateEdgeOp{
			EdgeId: edgeID,
			From:   &pb.NodeRef{Ref: &pb.NodeRef_AliasRef{AliasRef: fromAlias}},
			To:     &pb.NodeRef{Ref: &pb.NodeRef_AliasRef{AliasRef: toAlias}},
		},
	}}
}

// TestExecuteAtomic_AliasResolution_SingleCreateNodeAlias pins that a
// solo create_node with an "as" alias survives ingress untouched and
// the alias never leaks into the WAL payload (it is recorded only in
// the handler's per-transaction map).
func TestExecuteAtomic_AliasResolution_SingleCreateNodeAlias(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)

	resp, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "alias-solo-1",
		Operations: []*pb.Operation{{
			Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, As: "charlie",
				Data: newStruct(t, map[string]any{"email": "c@x"}),
			}},
		}},
	})
	if err != nil || !resp.GetSuccess() {
		t.Fatalf("ExecuteAtomic: err=%v resp=%+v", err, resp)
	}
	if len(resp.GetCreatedNodeIds()) != 1 {
		t.Fatalf("CreatedNodeIds: got %v want 1", resp.GetCreatedNodeIds())
	}
	uuidNodeID := resp.GetCreatedNodeIds()[0]
	if uuidNodeID == "" || strings.HasPrefix(uuidNodeID, "$") {
		t.Fatalf("CreatedNodeIds[0]: got %q want non-empty UUID", uuidNodeID)
	}

	recs := f.wal.GetAllRecords(xaTopic)
	if len(recs) != 1 {
		t.Fatalf("WAL records: got %d want 1", len(recs))
	}
	var ev wal.Event
	if err := json.Unmarshal(recs[0].Value, &ev); err != nil {
		t.Fatalf("decode WAL event: %v", err)
	}
	// The "as" tag is still on the create_node op — that is the
	// public alias contract for downstream readers — but the node
	// "id" itself MUST be the server-filled UUID, never "$charlie".
	if got, _ := ev.Ops[0]["id"].(string); got != uuidNodeID {
		t.Fatalf("WAL op id: got %q want %q", got, uuidNodeID)
	}
	if got, _ := ev.Ops[0]["as"].(string); got != "charlie" {
		t.Fatalf("WAL op as: got %q want %q", got, "charlie")
	}
}

// TestExecuteAtomic_AliasResolution_EdgeReferencesAlias pins the
// happy path that prompted this fix: create_node($alice) + create_edge
// referencing $alice. The WAL must carry the server-filled UUID on
// the edge's from/to as a bare string, NOT a {"ref":"$alice"} map.
func TestExecuteAtomic_AliasResolution_EdgeReferencesAlias(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)

	resp, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "alias-edge-1",
		Operations: []*pb.Operation{
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, As: "alice",
				Data: newStruct(t, map[string]any{"email": "a@x"}),
			}}},
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, As: "phone",
				Data: newStruct(t, map[string]any{"email": "p@x"}),
			}}},
			aliasRefEdge(99, "$alice", "$phone"),
		},
	})
	if err != nil {
		t.Fatalf("ExecuteAtomic: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("Success=false (error=%q code=%q)", resp.GetError(), resp.GetErrorCode())
	}
	if len(resp.GetCreatedNodeIds()) != 2 {
		t.Fatalf("CreatedNodeIds: got %v want 2", resp.GetCreatedNodeIds())
	}
	aliceUUID := resp.GetCreatedNodeIds()[0]
	phoneUUID := resp.GetCreatedNodeIds()[1]

	recs := f.wal.GetAllRecords(xaTopic)
	if len(recs) != 1 {
		t.Fatalf("WAL records: got %d want 1", len(recs))
	}
	var ev wal.Event
	if err := json.Unmarshal(recs[0].Value, &ev); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(ev.Ops) != 3 {
		t.Fatalf("ev.Ops: got %d want 3", len(ev.Ops))
	}
	edgeOp := ev.Ops[2]
	if got, _ := edgeOp["op"].(string); got != "create_edge" {
		t.Fatalf("op[2] kind: got %q want create_edge", got)
	}
	// The contract: from/to are bare strings, not maps. If we ever
	// regress and let {"ref":"$alice"} slip through, the applier's
	// stringField will return "" and halt the consumer.
	from, ok := edgeOp["from"].(string)
	if !ok {
		t.Fatalf("from: got %T want string (alias must be resolved at ingress)", edgeOp["from"])
	}
	if from != aliceUUID {
		t.Fatalf("from: got %q want %q (server-filled UUID for $alice)", from, aliceUUID)
	}
	to, ok := edgeOp["to"].(string)
	if !ok {
		t.Fatalf("to: got %T want string", edgeOp["to"])
	}
	if to != phoneUUID {
		t.Fatalf("to: got %q want %q (server-filled UUID for $phone)", to, phoneUUID)
	}
}

// TestExecuteAtomic_AliasResolution_MixedRefShapes pins that bare-id
// and alias_ref refs can be combined in the same transaction. The
// bare-id endpoint passes through unchanged, the $alias endpoint
// resolves against the in-flight alias map.
func TestExecuteAtomic_AliasResolution_MixedRefShapes(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)

	resp, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "alias-mix-1",
		Operations: []*pb.Operation{
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, As: "alice", Id: "alice-fixed",
				Data: newStruct(t, map[string]any{"email": "a@x"}),
			}}},
			{Op: &pb.Operation_CreateEdge{CreateEdge: &pb.CreateEdgeOp{
				EdgeId: 99,
				From:   &pb.NodeRef{Ref: &pb.NodeRef_AliasRef{AliasRef: "$alice"}},
				To:     &pb.NodeRef{Ref: &pb.NodeRef_Id{Id: "real-bob"}},
			}}},
		},
	})
	if err != nil || !resp.GetSuccess() {
		t.Fatalf("ExecuteAtomic: err=%v resp=%+v", err, resp)
	}
	var ev wal.Event
	if err := json.Unmarshal(f.wal.GetAllRecords(xaTopic)[0].Value, &ev); err != nil {
		t.Fatalf("decode: %v", err)
	}
	from, _ := ev.Ops[1]["from"].(string)
	to, _ := ev.Ops[1]["to"].(string)
	if from != "alice-fixed" {
		t.Fatalf("from: got %q want %q", from, "alice-fixed")
	}
	if to != "real-bob" {
		t.Fatalf("to: got %q want %q", to, "real-bob")
	}
}

// TestExecuteAtomic_AliasResolution_UnresolvedAliasRejected pins the
// failure mode: a create_edge that references "$ghost" with no prior
// create_node defining "ghost" gets INVALID_ARGUMENT, NOT a poisoned
// applier. The fix's reason for being.
func TestExecuteAtomic_AliasResolution_UnresolvedAliasRejected(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)

	_, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context: &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		Operations: []*pb.Operation{
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, Id: "alice-fixed",
				Data: newStruct(t, map[string]any{"email": "a@x"}),
			}}},
			aliasRefEdge(99, "alice-fixed", "$ghost"),
		},
	})
	if err == nil {
		t.Fatalf("ExecuteAtomic: expected InvalidArgument, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("code: got %v want InvalidArgument (msg=%v)", got, err)
	}
	if n := f.wal.GetRecordCount(xaTopic); n != 0 {
		t.Fatalf("WAL records: got %d want 0 (handler aborted before append)", n)
	}
}

// TestExecuteAtomic_AliasResolution_ForwardReferenceRejected pins that
// aliases are resolved in op order: an edge that references an alias
// defined LATER in the same transaction must fail, not silently
// half-work. (Python parity: the applier's alias map is populated
// linearly too.)
func TestExecuteAtomic_AliasResolution_ForwardReferenceRejected(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)

	_, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context: &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		Operations: []*pb.Operation{
			// edge first — references an alias that doesn't exist yet.
			aliasRefEdge(99, "$alice", "$bob"),
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, As: "alice",
				Data: newStruct(t, map[string]any{"email": "a@x"}),
			}}},
		},
	})
	if err == nil {
		t.Fatalf("ExecuteAtomic: expected InvalidArgument, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("code: got %v want InvalidArgument (msg=%v)", got, err)
	}
}

// TestExecuteAtomic_AliasResolution_TransactionIsolation pins that
// aliases never span transactions: an alias defined in transaction A
// is NOT visible to transaction B, even on the same handler.
func TestExecuteAtomic_AliasResolution_TransactionIsolation(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)

	// txn A defines $alice.
	_, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "iso-A",
		Operations: []*pb.Operation{
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, As: "alice",
				Data: newStruct(t, map[string]any{"email": "a@x"}),
			}}},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteAtomic A: %v", err)
	}

	// txn B tries to reference $alice with no local create_node.
	_, err = f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "iso-B",
		Operations: []*pb.Operation{
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, Id: "bob-fixed",
				Data: newStruct(t, map[string]any{"email": "b@x"}),
			}}},
			aliasRefEdge(99, "bob-fixed", "$alice"),
		},
	})
	if err == nil {
		t.Fatalf("ExecuteAtomic B: expected InvalidArgument (alias must not leak across txns), got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("code: got %v want InvalidArgument", got)
	}
}

// TestExecuteAtomic_AliasResolution_MultipleAliasesInOneTxn covers
// the original e2e failure shape: three aliased creates feeding three
// edges. Every edge endpoint resolves to a distinct server-filled
// UUID, all on the same WAL record.
func TestExecuteAtomic_AliasResolution_MultipleAliasesInOneTxn(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)

	resp, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "alias-multi-1",
		Operations: []*pb.Operation{
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, As: "charlie",
				Data: newStruct(t, map[string]any{"email": "c@x"}),
			}}},
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, As: "phone",
				Data: newStruct(t, map[string]any{"email": "p@x"}),
			}}},
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, As: "order1",
				Data: newStruct(t, map[string]any{"email": "o@x"}),
			}}},
			aliasRefEdge(101, "$charlie", "$phone"),
			aliasRefEdge(102, "$charlie", "$order1"),
			aliasRefEdge(103, "$order1", "$phone"),
		},
	})
	if err != nil || !resp.GetSuccess() {
		t.Fatalf("ExecuteAtomic: err=%v resp=%+v", err, resp)
	}
	ids := resp.GetCreatedNodeIds()
	if len(ids) != 3 {
		t.Fatalf("CreatedNodeIds: got %v want 3", ids)
	}
	charlie, phone, order1 := ids[0], ids[1], ids[2]

	var ev wal.Event
	if err := json.Unmarshal(f.wal.GetAllRecords(xaTopic)[0].Value, &ev); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i, want := range []struct{ from, to string }{
		{charlie, phone},
		{charlie, order1},
		{order1, phone},
	} {
		got := ev.Ops[3+i]
		if f, _ := got["from"].(string); f != want.from {
			t.Errorf("op[%d].from: got %q want %q", 3+i, f, want.from)
		}
		if to, _ := got["to"].(string); to != want.to {
			t.Errorf("op[%d].to: got %q want %q", 3+i, to, want.to)
		}
	}
}

// TestExecuteAtomic_AliasResolution_DeleteEdgeAlias pins the symmetric
// case on the destructive side: delete_edge with $alias refs resolves
// the same way create_edge does.
func TestExecuteAtomic_AliasResolution_DeleteEdgeAlias(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)

	resp, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "alias-del-1",
		Operations: []*pb.Operation{
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, As: "u",
				Data: newStruct(t, map[string]any{"email": "u@x"}),
			}}},
			{Op: &pb.Operation_DeleteEdge{DeleteEdge: &pb.DeleteEdgeOp{
				EdgeId: 99,
				From:   &pb.NodeRef{Ref: &pb.NodeRef_AliasRef{AliasRef: "$u"}},
				To:     &pb.NodeRef{Ref: &pb.NodeRef_Id{Id: "real-target"}},
			}}},
		},
	})
	if err != nil || !resp.GetSuccess() {
		t.Fatalf("ExecuteAtomic: err=%v resp=%+v", err, resp)
	}
	uUUID := resp.GetCreatedNodeIds()[0]

	var ev wal.Event
	if err := json.Unmarshal(f.wal.GetAllRecords(xaTopic)[0].Value, &ev); err != nil {
		t.Fatalf("decode: %v", err)
	}
	from, _ := ev.Ops[1]["from"].(string)
	to, _ := ev.Ops[1]["to"].(string)
	if from != uUUID {
		t.Fatalf("delete_edge.from: got %q want %q", from, uUUID)
	}
	if to != "real-target" {
		t.Fatalf("delete_edge.to: got %q want %q", to, "real-target")
	}
}

// TestExecuteAtomic_AliasResolution_DottedFormSupported pins the
// Python-parity dotted-alias form: "$alice.id" resolves the same as
// "$alice". applier.py:_resolve_ref splits on "." for legacy reasons.
func TestExecuteAtomic_AliasResolution_DottedFormSupported(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)

	resp, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "alias-dot-1",
		Operations: []*pb.Operation{
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, As: "alice",
				Data: newStruct(t, map[string]any{"email": "a@x"}),
			}}},
			aliasRefEdge(99, "$alice.id", "$alice.id"),
		},
	})
	if err != nil || !resp.GetSuccess() {
		t.Fatalf("ExecuteAtomic: err=%v resp=%+v", err, resp)
	}
	aliceUUID := resp.GetCreatedNodeIds()[0]
	var ev wal.Event
	if err := json.Unmarshal(f.wal.GetAllRecords(xaTopic)[0].Value, &ev); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if from, _ := ev.Ops[1]["from"].(string); from != aliceUUID {
		t.Fatalf("from: got %q want %q", from, aliceUUID)
	}
	if to, _ := ev.Ops[1]["to"].(string); to != aliceUUID {
		t.Fatalf("to: got %q want %q", to, aliceUUID)
	}
}

// TestExecuteAtomic_DeleteWhereEmptyPredicateRejected pins the
// issue-#504 safety rail: an unconditional bulk delete is too
// dangerous to express implicitly, so a DeleteWhereOp with no `where`
// predicate is INVALID_ARGUMENT before any WAL append.
func TestExecuteAtomic_DeleteWhereEmptyPredicateRejected(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)
	_, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context: &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		Operations: []*pb.Operation{{
			Op: &pb.Operation_DeleteWhere{DeleteWhere: &pb.DeleteWhereOp{
				TypeId: 1,
			}},
		}},
	})
	if err == nil {
		t.Fatalf("ExecuteAtomic: expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("code: got %v, want InvalidArgument", got)
	}
	if n := f.wal.GetRecordCount(xaTopic); n != 0 {
		t.Fatalf("WAL: got %d records, want 0 (rejected before append)", n)
	}
}

// TestExecuteAtomic_DeleteWhereMissingTypeIDRejected pins the per-op
// required-field validation parity with the other ops.
func TestExecuteAtomic_DeleteWhereMissingTypeIDRejected(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)
	_, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context: &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		Operations: []*pb.Operation{{
			Op: &pb.Operation_DeleteWhere{DeleteWhere: &pb.DeleteWhereOp{
				Where: []*pb.FieldFilter{
					{Field: "name", Op: pb.FilterOp_EQ, Value: newValue(t, "x")},
				},
			}},
		}},
	})
	if err == nil {
		t.Fatalf("ExecuteAtomic: expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("code: got %v, want InvalidArgument", got)
	}
}

// TestExecuteAtomic_DeleteWhereRoundTrip is the full #504 contract end
// to end: the handler resolves the developer-facing field NAME to a
// stable field_id (reusing the QueryNodes #501 path), appends a single
// predicate op, and the applier sweeps exactly the matching nodes —
// one round trip, no QueryNodes+DeleteNode loop. The handler's
// existing write-access gate is the ACL chokepoint (ADR-016: handlers
// gate, applier just materialises).
func TestExecuteAtomic_DeleteWhereRoundTrip(t *testing.T) {
	t.Parallel()
	f := newXAFixture(t)
	f.runApplier(t)

	// Seed: three Users. Two have name="sweep-me", one does not.
	seed, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "dw-seed",
		Operations: []*pb.Operation{
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, Id: "u-stale-1",
				Data: newStruct(t, map[string]any{"email": "a@x", "name": "sweep-me"}),
			}}},
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, Id: "u-stale-2",
				Data: newStruct(t, map[string]any{"email": "b@x", "name": "sweep-me"}),
			}}},
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: 1, Id: "u-keep",
				Data: newStruct(t, map[string]any{"email": "c@x", "name": "keep"}),
			}}},
		},
	})
	if err != nil || !seed.GetSuccess() {
		t.Fatalf("seed ExecuteAtomic: err=%v resp=%+v", err, seed)
	}
	f.waitForIdempKey(t, "dw-seed")

	// Single-RPC sweep by NAME predicate (handler resolves name->id).
	resp, err := f.srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: xaTenant, Actor: xaActorStr},
		IdempotencyKey: "dw-sweep",
		WaitApplied:    true,
		Operations: []*pb.Operation{{
			Op: &pb.Operation_DeleteWhere{DeleteWhere: &pb.DeleteWhereOp{
				TypeId: 1,
				Where: []*pb.FieldFilter{
					{Field: "name", Op: pb.FilterOp_EQ, Value: newValue(t, "sweep-me")},
				},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("sweep ExecuteAtomic: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("sweep Success=false (error=%q code=%q)", resp.GetError(), resp.GetErrorCode())
	}
	f.waitForIdempKey(t, "dw-sweep")

	// Matching nodes swept; the non-matching one survives.
	for _, id := range []string{"u-stale-1", "u-stale-2"} {
		if _, err := f.store.GetNode(context.Background(), xaTenant, id); err == nil {
			t.Fatalf("node %q should have been swept", id)
		}
	}
	if _, err := f.store.GetNode(context.Background(), xaTenant, "u-keep"); err != nil {
		t.Fatalf("u-keep must survive the predicate sweep: %v", err)
	}

	// The WAL carries exactly one id-keyed predicate (schema-less on
	// the log) — field NAME "name" resolved to field_id 2.
	recs := f.wal.GetAllRecords(xaTopic)
	var ev wal.Event
	if err := json.Unmarshal(recs[len(recs)-1].Value, &ev); err != nil {
		t.Fatalf("decode sweep event: %v", err)
	}
	if got := ev.Ops[0]["op"]; got != "delete_where" {
		t.Fatalf("op: got %v want delete_where", got)
	}
	where, ok := ev.Ops[0]["where"].([]any)
	if !ok || len(where) != 1 {
		t.Fatalf("where: got %#v want one predicate", ev.Ops[0]["where"])
	}
	pm, _ := where[0].(map[string]any)
	if fid, _ := pm["field_id"].(float64); int(fid) != 2 {
		t.Fatalf("predicate field_id: got %v want 2 (name->id resolved)", pm["field_id"])
	}
}
