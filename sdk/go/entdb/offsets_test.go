// SPDX-License-Identifier: MIT
package entdb

import (
	"context"
	"fmt"
	"sync"
	"testing"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

// trackingServer is a fakeServer wired with canned read responses so
// the read-after-write flow can be exercised end-to-end: commit,
// then read, and assert the recorded stream_position rode along as
// the proto after_offset.
func newTrackingServer() *fakeServer {
	return &fakeServer{
		execResp: &pb.ExecuteAtomicResponse{
			Success: true,
			Receipt: &pb.Receipt{
				TenantId:       "acme",
				IdempotencyKey: "idem-1",
				StreamPosition: "42",
			},
			CreatedNodeIds: []string{"node-1"},
			AppliedStatus:  pb.ReceiptStatus_RECEIPT_STATUS_APPLIED,
		},
		getNodeResp: &pb.GetNodeResponse{
			Found: true,
			Node:  &pb.Node{TenantId: "acme", NodeId: "node-1", TypeId: 201},
		},
		getNodeByKeyResp: &pb.GetNodeByKeyResponse{
			Found: true,
			Node:  &pb.Node{TenantId: "acme", NodeId: "node-1", TypeId: 201},
		},
		queryResp:    &pb.QueryNodesResponse{Nodes: []*pb.Node{{NodeId: "node-1", TypeId: 201}}},
		getNodesResp: &pb.GetNodesResponse{Nodes: []*pb.Node{{NodeId: "node-1", TypeId: 201}}},
	}
}

// trackingTransport returns a bufconn-backed grpcTransport with a
// live offsetTracker (startFakeServer builds the struct literal
// without one).
func trackingTransport(t *testing.T, svc *fakeServer) *grpcTransport {
	t.Helper()
	tr := startFakeServer(t, svc)
	tr.offsets = newOffsetTracker()
	return tr
}

// TestReadAfterWrite_AutoAttachesOffset is the core E10.2 contract:
// a read issued right after a Commit must carry the receipt's
// stream_position as after_offset (and the 30s wait), with zero
// caller intervention — mirroring the Python SDK.
func TestReadAfterWrite_AutoAttachesOffset(t *testing.T) {
	svc := newTrackingServer()
	tr := trackingTransport(t, svc)
	ctx := context.Background()

	if _, err := tr.ExecuteAtomic(ctx, "acme", "user:alice", "idem-1",
		[]Operation{{Type: OpCreateNode, TypeID: 201, Alias: "a", Data: map[string]any{"1": "x"}}}); err != nil {
		t.Fatalf("ExecuteAtomic: %v", err)
	}

	if _, err := tr.GetNode(ctx, "acme", "user:alice", 201, "node-1"); err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got := svc.getNodeReq.GetAfterOffset(); got != "42" {
		t.Errorf("GetNode after_offset = %q, want \"42\"", got)
	}
	if got := svc.getNodeReq.GetWaitTimeoutMs(); got != 30000 {
		t.Errorf("GetNode wait_timeout_ms = %d, want 30000", got)
	}

	if _, err := tr.QueryNodes(ctx, "acme", "user:alice", 201, nil, 0); err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	if got := svc.queryReq.GetAfterOffset(); got != "42" {
		t.Errorf("QueryNodes after_offset = %q, want \"42\"", got)
	}
	if got := svc.queryReq.GetWaitTimeoutMs(); got != 30000 {
		t.Errorf("QueryNodes wait_timeout_ms = %d, want 30000", got)
	}

	if _, _, err := tr.GetNodes(ctx, "acme", "user:alice", 201, []string{"node-1"}); err != nil {
		t.Fatalf("GetNodes: %v", err)
	}
	if got := svc.getNodesReq.GetAfterOffset(); got != "42" {
		t.Errorf("GetNodes after_offset = %q, want \"42\"", got)
	}
	if got := svc.getNodesReq.GetWaitTimeoutMs(); got != 30000 {
		t.Errorf("GetNodes wait_timeout_ms = %d, want 30000", got)
	}

	if _, err := tr.GetNodeByKey(ctx, "acme", "user:alice", 201, 5, nil); err != nil {
		t.Fatalf("GetNodeByKey: %v", err)
	}
	// GetNodeByKey carries after_offset as int64 (mirrors Python's
	// int() coercion of the string offset).
	if got := svc.getNodeByKeyReq.GetAfterOffset(); got != 42 {
		t.Errorf("GetNodeByKey after_offset = %d, want 42", got)
	}
}

// TestRead_NoOffsetBeforeAnyWrite verifies a read with no prior
// write for the tenant sends no after_offset and no wait — matching
// the Python ``_last_offsets.get(tenant_id)`` -> None path.
func TestRead_NoOffsetBeforeAnyWrite(t *testing.T) {
	svc := newTrackingServer()
	tr := trackingTransport(t, svc)

	if _, err := tr.GetNode(context.Background(), "acme", "user:alice", 201, "node-1"); err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got := svc.getNodeReq.GetAfterOffset(); got != "" {
		t.Errorf("after_offset = %q, want empty (no prior write)", got)
	}
	if got := svc.getNodeReq.GetWaitTimeoutMs(); got != 0 {
		t.Errorf("wait_timeout_ms = %d, want 0", got)
	}
}

// TestOffsetTracking_PerTenantIsolation: a write for tenant A must
// not leak into reads for tenant B.
func TestOffsetTracking_PerTenantIsolation(t *testing.T) {
	svc := newTrackingServer()
	svc.execResp.Receipt.TenantId = "tenant-a"
	tr := trackingTransport(t, svc)
	ctx := context.Background()

	if _, err := tr.ExecuteAtomic(ctx, "tenant-a", "user:alice", "idem-1",
		[]Operation{{Type: OpCreateNode, TypeID: 201, Alias: "a", Data: map[string]any{"1": "x"}}}); err != nil {
		t.Fatalf("ExecuteAtomic: %v", err)
	}

	if _, err := tr.GetNode(ctx, "tenant-b", "user:alice", 201, "node-1"); err != nil {
		t.Fatalf("GetNode tenant-b: %v", err)
	}
	if got := svc.getNodeReq.GetAfterOffset(); got != "" {
		t.Errorf("tenant-b read carried tenant-a offset %q", got)
	}

	if _, err := tr.GetNode(ctx, "tenant-a", "user:alice", 201, "node-1"); err != nil {
		t.Fatalf("GetNode tenant-a: %v", err)
	}
	if got := svc.getNodeReq.GetAfterOffset(); got != "42" {
		t.Errorf("tenant-a read after_offset = %q, want \"42\"", got)
	}
}

// TestWithAfterOffset_ExplicitOverride: an explicit per-call offset
// wins over the tracked one (Python explicit-string path).
func TestWithAfterOffset_ExplicitOverride(t *testing.T) {
	svc := newTrackingServer()
	tr := trackingTransport(t, svc)
	ctx := context.Background()

	if _, err := tr.ExecuteAtomic(ctx, "acme", "user:alice", "idem-1",
		[]Operation{{Type: OpCreateNode, TypeID: 201, Alias: "a", Data: map[string]any{"1": "x"}}}); err != nil {
		t.Fatalf("ExecuteAtomic: %v", err)
	}

	octx := WithAfterOffset(ctx, "99")
	if _, err := tr.GetNode(octx, "acme", "user:alice", 201, "node-1"); err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got := svc.getNodeReq.GetAfterOffset(); got != "99" {
		t.Errorf("after_offset = %q, want \"99\" (explicit override)", got)
	}
}

// TestWithoutOffsetTracking_OptOut: opting out suppresses the
// after_offset even when a write was recorded (Python
// ``after_offset=None`` path).
func TestWithoutOffsetTracking_OptOut(t *testing.T) {
	svc := newTrackingServer()
	tr := trackingTransport(t, svc)
	ctx := context.Background()

	if _, err := tr.ExecuteAtomic(ctx, "acme", "user:alice", "idem-1",
		[]Operation{{Type: OpCreateNode, TypeID: 201, Alias: "a", Data: map[string]any{"1": "x"}}}); err != nil {
		t.Fatalf("ExecuteAtomic: %v", err)
	}

	octx := WithoutOffsetTracking(ctx)
	if _, err := tr.GetNode(octx, "acme", "user:alice", 201, "node-1"); err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got := svc.getNodeReq.GetAfterOffset(); got != "" {
		t.Errorf("after_offset = %q, want empty (opt-out)", got)
	}
	if got := svc.getNodeReq.GetWaitTimeoutMs(); got != 0 {
		t.Errorf("wait_timeout_ms = %d, want 0 (opt-out)", got)
	}
}

// TestClearOffsets drops tracked state so the next read no longer
// pins to the prior write — mirrors Python ``clear_offsets()``.
func TestClearOffsets(t *testing.T) {
	svc := newTrackingServer()
	tr := trackingTransport(t, svc)
	client := newClientWithTransport("bufnet", tr)
	ctx := context.Background()

	if _, err := tr.ExecuteAtomic(ctx, "acme", "user:alice", "idem-1",
		[]Operation{{Type: OpCreateNode, TypeID: 201, Alias: "a", Data: map[string]any{"1": "x"}}}); err != nil {
		t.Fatalf("ExecuteAtomic: %v", err)
	}

	client.ClearOffsets()

	if _, err := tr.GetNode(ctx, "acme", "user:alice", 201, "node-1"); err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got := svc.getNodeReq.GetAfterOffset(); got != "" {
		t.Errorf("after_offset = %q, want empty after ClearOffsets", got)
	}
}

// TestClearOffsets_NoTrackerIsNoOp: ClearOffsets must be safe on a
// client whose transport does not implement offsetClearer (bare
// test doubles).
func TestClearOffsets_NoTrackerIsNoOp(t *testing.T) {
	client := newClientWithTransport("x", &mockTransport{})
	client.ClearOffsets() // must not panic
}

// TestOffsetTracker_Concurrency exercises the RWMutex under racing
// record/get/clear goroutines — run with -race in CI.
func TestOffsetTracker_Concurrency(t *testing.T) {
	tr := newOffsetTracker()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func(n int) { defer wg.Done(); tr.record(fmt.Sprintf("t%d", n%5), fmt.Sprintf("%d", n)) }(i)
		go func(n int) { defer wg.Done(); _ = tr.get(fmt.Sprintf("t%d", n%5)) }(i)
		go func() { defer wg.Done(); tr.clear() }()
	}
	wg.Wait()
}

// TestOffsetTracker_NilSafe: methods on a nil tracker degrade
// gracefully (transports built without newGRPCTransport).
func TestOffsetTracker_NilSafe(t *testing.T) {
	var tr *offsetTracker
	tr.record("t", "1") // no panic
	if got := tr.get("t"); got != "" {
		t.Errorf("nil get = %q, want empty", got)
	}
	tr.clear() // no panic
	if got := tr.resolve(context.Background(), "t"); got != "" {
		t.Errorf("nil resolve = %q, want empty", got)
	}
}

func TestOffsetAsInt64(t *testing.T) {
	cases := map[string]int64{"": 0, "42": 42, "not-a-number": 0, "-7": -7}
	for in, want := range cases {
		if got := offsetAsInt64(in); got != want {
			t.Errorf("offsetAsInt64(%q) = %d, want %d", in, got, want)
		}
	}
}
