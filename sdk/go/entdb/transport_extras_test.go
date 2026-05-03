package entdb

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

// Tests for the v1.4 surface added on top of v1.3:
//   - GetNodes (batch)
//   - WaitForOffset
//   - GetReceiptStatus
//   - GetConnectedNodes
//   - ListSharedWithMe
//   - RevokeAccess
//   - TransferOwnership
//   - AddGroupMember / RemoveGroupMember
//
// Each method gets a request-shape assertion (so wire breakages in
// future proto edits trip the test) plus a response-decoding
// assertion.

// ── GetNodes (batch) ───────────────────────────────────────────────────

func TestGrpcTransport_GetNodes_HappyPath(t *testing.T) {
	payload, _ := structpb.NewStruct(map[string]any{"1": "WIDGET-1"})
	svc := &fakeServer{
		getNodesResp: &pb.GetNodesResponse{
			Nodes: []*pb.Node{
				{TenantId: "acme", NodeId: "n1", TypeId: 201, Payload: payload},
				{TenantId: "acme", NodeId: "n2", TypeId: 201, Payload: payload},
			},
			MissingIds: []string{"n3"},
		},
	}
	tr := startFakeServer(t, svc)

	got, missing, err := tr.GetNodes(
		context.Background(), "acme", "user:alice", 201, []string{"n1", "n2", "n3"},
	)
	if err != nil {
		t.Fatalf("GetNodes: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(got))
	}
	if got[0].NodeID != "n1" || got[1].NodeID != "n2" {
		t.Errorf("ids mismatch: %v %v", got[0].NodeID, got[1].NodeID)
	}
	if len(missing) != 1 || missing[0] != "n3" {
		t.Errorf("missing mismatch: %v", missing)
	}
	if svc.getNodesReq.GetTypeId() != 201 ||
		svc.getNodesReq.GetContext().GetTenantId() != "acme" ||
		len(svc.getNodesReq.GetNodeIds()) != 3 {
		t.Errorf("request shape wrong: %+v", svc.getNodesReq)
	}
}

func TestGrpcTransport_GetNodes_TranslatesNotFound(t *testing.T) {
	svc := &fakeServer{
		getNodesErr: status.Error(codes.NotFound, "type unknown"),
	}
	tr := startFakeServer(t, svc)

	_, _, err := tr.GetNodes(context.Background(), "acme", "user:alice", 999, []string{"n1"})
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("want NotFoundError, got %T (%v)", err, err)
	}
}

// ── WaitForOffset ──────────────────────────────────────────────────────

func TestGrpcTransport_WaitForOffset_ReachesPosition(t *testing.T) {
	svc := &fakeServer{
		waitOffsetResp: &pb.WaitForOffsetResponse{
			Reached:         true,
			CurrentPosition: "kafka:42",
		},
	}
	tr := startFakeServer(t, svc)

	reached, pos, err := tr.WaitForOffset(
		context.Background(), "acme", "user:alice", "kafka:42", 5_000,
	)
	if err != nil {
		t.Fatalf("WaitForOffset: %v", err)
	}
	if !reached || pos != "kafka:42" {
		t.Errorf("want reached=true position=kafka:42, got %v %q", reached, pos)
	}
	if svc.waitOffsetReq.GetTimeoutMs() != 5_000 ||
		svc.waitOffsetReq.GetStreamPosition() != "kafka:42" {
		t.Errorf("request shape wrong: %+v", svc.waitOffsetReq)
	}
}

func TestGrpcTransport_WaitForOffset_TimesOutCleanly(t *testing.T) {
	svc := &fakeServer{
		waitOffsetResp: &pb.WaitForOffsetResponse{
			Reached:         false,
			CurrentPosition: "kafka:38",
		},
	}
	tr := startFakeServer(t, svc)

	reached, pos, err := tr.WaitForOffset(
		context.Background(), "acme", "user:alice", "kafka:42", 100,
	)
	if err != nil {
		t.Fatalf("WaitForOffset: %v", err)
	}
	if reached {
		t.Errorf("want reached=false")
	}
	if pos != "kafka:38" {
		t.Errorf("want current=kafka:38, got %q (callers use this to see how far behind)", pos)
	}
}

// ── GetReceiptStatus ───────────────────────────────────────────────────

func TestGrpcTransport_GetReceiptStatus_AppliedDecodes(t *testing.T) {
	svc := &fakeServer{
		receiptResp: &pb.GetReceiptStatusResponse{
			Status: pb.ReceiptStatus_RECEIPT_STATUS_APPLIED,
		},
	}
	tr := startFakeServer(t, svc)

	st, msg, err := tr.GetReceiptStatus(
		context.Background(), "acme", "user:alice", "idempo-1",
	)
	if err != nil {
		t.Fatalf("GetReceiptStatus: %v", err)
	}
	if st != ReceiptStatusApplied || msg != "" {
		t.Errorf("want APPLIED/empty msg, got %v / %q", st, msg)
	}
	if svc.receiptReq.GetIdempotencyKey() != "idempo-1" {
		t.Errorf("request shape wrong: %+v", svc.receiptReq)
	}
}

func TestGrpcTransport_GetReceiptStatus_FailedSurfacesError(t *testing.T) {
	svc := &fakeServer{
		receiptResp: &pb.GetReceiptStatusResponse{
			Status: pb.ReceiptStatus_RECEIPT_STATUS_FAILED,
			Error:  "validation: missing required field 'sku'",
		},
	}
	tr := startFakeServer(t, svc)

	st, msg, err := tr.GetReceiptStatus(
		context.Background(), "acme", "user:alice", "idempo-2",
	)
	if err != nil {
		t.Fatalf("GetReceiptStatus: %v", err)
	}
	if st != ReceiptStatusFailed {
		t.Errorf("want FAILED, got %v", st)
	}
	if msg == "" {
		t.Error("want non-empty error string for FAILED receipt")
	}
}

// ── GetConnectedNodes ──────────────────────────────────────────────────

func TestGrpcTransport_GetConnectedNodes_HappyPath(t *testing.T) {
	payload, _ := structpb.NewStruct(map[string]any{"1": "WIDGET-1"})
	svc := &fakeServer{
		connectedResp: &pb.GetConnectedNodesResponse{
			Nodes: []*pb.Node{
				{TenantId: "acme", NodeId: "p-1", TypeId: 201, Payload: payload},
			},
		},
	}
	tr := startFakeServer(t, svc)

	got, err := tr.GetConnectedNodes(
		context.Background(), "acme", "user:alice", "user-1", 101,
	)
	if err != nil {
		t.Fatalf("GetConnectedNodes: %v", err)
	}
	if len(got) != 1 || got[0].NodeID != "p-1" {
		t.Errorf("got %+v", got)
	}
	if svc.connectedReq.GetEdgeTypeId() != 101 ||
		svc.connectedReq.GetNodeId() != "user-1" {
		t.Errorf("request shape wrong: %+v", svc.connectedReq)
	}
}

// ── ListSharedWithMe ───────────────────────────────────────────────────

func TestGrpcTransport_ListSharedWithMe_HappyPath(t *testing.T) {
	payload, _ := structpb.NewStruct(map[string]any{"1": "doc"})
	svc := &fakeServer{
		sharedResp: &pb.ListSharedWithMeResponse{
			Nodes: []*pb.Node{
				{TenantId: "globex", NodeId: "doc-1", TypeId: 301, Payload: payload},
				{TenantId: "globex", NodeId: "doc-2", TypeId: 301, Payload: payload},
			},
			HasMore: false,
		},
	}
	tr := startFakeServer(t, svc)

	got, err := tr.ListSharedWithMe(
		context.Background(), "acme", "user:alice", 50, 0,
	)
	if err != nil {
		t.Fatalf("ListSharedWithMe: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	// Cross-tenant share — the returned tenant differs from the caller's.
	if got[0].TenantID != "globex" {
		t.Errorf("expected cross-tenant tenant_id, got %q", got[0].TenantID)
	}
	if svc.sharedReq.GetLimit() != 50 || svc.sharedReq.GetOffset() != 0 {
		t.Errorf("paging shape wrong: %+v", svc.sharedReq)
	}
}

// ── RevokeAccess ───────────────────────────────────────────────────────

func TestGrpcTransport_RevokeAccess_RemovesGrant(t *testing.T) {
	svc := &fakeServer{
		revokeResp: &pb.RevokeAccessResponse{Found: true},
	}
	tr := startFakeServer(t, svc)

	found, err := tr.RevokeAccess(
		context.Background(), "acme", "user:alice", "node-1", "user:bob",
	)
	if err != nil {
		t.Fatalf("RevokeAccess: %v", err)
	}
	if !found {
		t.Error("want found=true")
	}
	if svc.revokeReq.GetNodeId() != "node-1" || svc.revokeReq.GetActorId() != "user:bob" {
		t.Errorf("request shape wrong: %+v", svc.revokeReq)
	}
}

func TestGrpcTransport_RevokeAccess_IsIdempotent(t *testing.T) {
	// found=false on a re-revoke: caller can safely retry without
	// special-casing the second call.
	svc := &fakeServer{
		revokeResp: &pb.RevokeAccessResponse{Found: false},
	}
	tr := startFakeServer(t, svc)

	found, err := tr.RevokeAccess(
		context.Background(), "acme", "user:alice", "node-1", "user:nobody",
	)
	if err != nil {
		t.Fatalf("RevokeAccess: %v", err)
	}
	if found {
		t.Error("want found=false for missing grant")
	}
}

// ── TransferOwnership ──────────────────────────────────────────────────

func TestGrpcTransport_TransferOwnership_HappyPath(t *testing.T) {
	svc := &fakeServer{
		xferResp: &pb.TransferOwnershipResponse{Found: true},
	}
	tr := startFakeServer(t, svc)

	found, err := tr.TransferOwnership(
		context.Background(), "acme", "user:alice", "node-1", "user:carol",
	)
	if err != nil {
		t.Fatalf("TransferOwnership: %v", err)
	}
	if !found {
		t.Error("want found=true")
	}
	if svc.xferReq.GetNewOwner() != "user:carol" {
		t.Errorf("request shape wrong: %+v", svc.xferReq)
	}
}

func TestGrpcTransport_TransferOwnership_NotFound(t *testing.T) {
	svc := &fakeServer{
		xferResp: &pb.TransferOwnershipResponse{Found: false},
	}
	tr := startFakeServer(t, svc)

	found, err := tr.TransferOwnership(
		context.Background(), "acme", "user:alice", "ghost", "user:carol",
	)
	if err != nil {
		t.Fatalf("TransferOwnership: %v", err)
	}
	if found {
		t.Error("want found=false for missing node")
	}
}

// ── AddGroupMember / RemoveGroupMember ─────────────────────────────────

func TestGrpcTransport_AddRemoveGroupMember_HappyPath(t *testing.T) {
	svc := &fakeServer{
		addGroupResp: &pb.GroupMemberResponse{Success: true},
		rmGroupResp:  &pb.GroupMemberResponse{Success: true},
	}
	tr := startFakeServer(t, svc)

	if err := tr.AddGroupMember(
		context.Background(), "acme", "user:alice", "grp-1", "user:bob", "admin",
	); err != nil {
		t.Fatalf("AddGroupMember: %v", err)
	}
	if svc.addGroupReq.GetRole() != "admin" ||
		svc.addGroupReq.GetGroupId() != "grp-1" ||
		svc.addGroupReq.GetMemberActorId() != "user:bob" {
		t.Errorf("add request shape wrong: %+v", svc.addGroupReq)
	}

	if err := tr.RemoveGroupMember(
		context.Background(), "acme", "user:alice", "grp-1", "user:bob",
	); err != nil {
		t.Fatalf("RemoveGroupMember: %v", err)
	}
	if svc.rmGroupReq.GetGroupId() != "grp-1" || svc.rmGroupReq.GetMemberActorId() != "user:bob" {
		t.Errorf("remove request shape wrong: %+v", svc.rmGroupReq)
	}
}

// ── DbClient and Scope wrappers route through the transport ───────────

func TestDbClient_WaitForOffset_DelegatesToTransport(t *testing.T) {
	mt := &mockTransport{}
	c := newClientWithTransport("addr", mt)
	_, _, _ = c.WaitForOffset(context.Background(), "acme", "user:alice", "pos", 100)
	// Smoke test: just confirm the wrapper exists and forwards
	// without panicking. The transport-level behaviour is covered
	// above against the bufconn server.
}

func TestScope_RevokeAccess_RoutesViaTransport(t *testing.T) {
	mt := &mockTransport{}
	c := newClientWithTransport("addr", mt)
	scope := c.Tenant("acme").Actor(UserActor("alice"))
	_, _ = scope.RevokeAccess(context.Background(), "node-1", UserActor("bob"))
}

func TestScope_AddGroupMember_RoutesViaTransport(t *testing.T) {
	mt := &mockTransport{}
	c := newClientWithTransport("addr", mt)
	scope := c.Tenant("acme").Actor(UserActor("alice"))
	_ = scope.AddGroupMember(context.Background(), "grp-1", UserActor("bob"), "member")
}
