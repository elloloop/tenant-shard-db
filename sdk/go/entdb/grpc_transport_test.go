package entdb

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

// fakeServer is an in-process EntDBServiceServer used by the
// transport-layer tests. It records the last request seen for
// each of the in-scope RPCs and hands back a canned response
// (optionally an error) so the Go SDK's wire translation layer
// can be exercised without a real Python server.
//
// Every field named “*Err“ takes precedence over the “*Resp“
// field for that RPC. Tests use the error path to probe the
// “translateGRPCError“ mapping on each status code.
type fakeServer struct {
	pb.UnimplementedEntDBServiceServer

	getNodeReq      *pb.GetNodeRequest
	getNodeByKeyReq *pb.GetNodeByKeyRequest
	queryReq        *pb.QueryNodesRequest
	searchReq       *pb.SearchNodesRequest
	edgesFromReq    *pb.GetEdgesRequest
	edgesToReq      *pb.GetEdgesRequest
	shareReq        *pb.ShareNodeRequest
	quotaReq        *pb.GetTenantQuotaRequest
	execReq         *pb.ExecuteAtomicRequest

	getNodeResp      *pb.GetNodeResponse
	getNodeErr       error
	getNodeByKeyResp *pb.GetNodeByKeyResponse
	getNodeByKeyErr  error
	queryResp        *pb.QueryNodesResponse
	queryErr         error
	searchResp       *pb.SearchNodesResponse
	searchErr        error
	edgesFromResp    *pb.GetEdgesResponse
	edgesFromErr     error
	edgesToResp      *pb.GetEdgesResponse
	edgesToErr       error
	shareResp        *pb.ShareNodeResponse
	shareErr         error
	quotaResp        *pb.GetTenantQuotaResponse
	quotaErr         error
	execResp         *pb.ExecuteAtomicResponse
	execErr          error

	// headerMeta is sent as a trailing metadata entry on every
	// RPC when non-empty. Used to test that rate-limit
	// translation picks up the ``retry-after`` header.
	trailerKV [2]string
}

func (s *fakeServer) sendTrailer(ctx context.Context) {
	if s.trailerKV[0] == "" {
		return
	}
	_ = grpc.SetTrailer(ctx, metadata.Pairs(s.trailerKV[0], s.trailerKV[1]))
}

func (s *fakeServer) GetNode(ctx context.Context, req *pb.GetNodeRequest) (*pb.GetNodeResponse, error) {
	s.getNodeReq = req
	s.sendTrailer(ctx)
	if s.getNodeErr != nil {
		return nil, s.getNodeErr
	}
	return s.getNodeResp, nil
}

func (s *fakeServer) GetNodeByKey(ctx context.Context, req *pb.GetNodeByKeyRequest) (*pb.GetNodeByKeyResponse, error) {
	s.getNodeByKeyReq = req
	s.sendTrailer(ctx)
	if s.getNodeByKeyErr != nil {
		return nil, s.getNodeByKeyErr
	}
	return s.getNodeByKeyResp, nil
}

func (s *fakeServer) QueryNodes(ctx context.Context, req *pb.QueryNodesRequest) (*pb.QueryNodesResponse, error) {
	s.queryReq = req
	s.sendTrailer(ctx)
	if s.queryErr != nil {
		return nil, s.queryErr
	}
	return s.queryResp, nil
}

func (s *fakeServer) SearchNodes(ctx context.Context, req *pb.SearchNodesRequest) (*pb.SearchNodesResponse, error) {
	s.searchReq = req
	s.sendTrailer(ctx)
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return s.searchResp, nil
}

func (s *fakeServer) GetEdgesFrom(ctx context.Context, req *pb.GetEdgesRequest) (*pb.GetEdgesResponse, error) {
	s.edgesFromReq = req
	s.sendTrailer(ctx)
	if s.edgesFromErr != nil {
		return nil, s.edgesFromErr
	}
	return s.edgesFromResp, nil
}

func (s *fakeServer) GetEdgesTo(ctx context.Context, req *pb.GetEdgesRequest) (*pb.GetEdgesResponse, error) {
	s.edgesToReq = req
	s.sendTrailer(ctx)
	if s.edgesToErr != nil {
		return nil, s.edgesToErr
	}
	return s.edgesToResp, nil
}

func (s *fakeServer) ShareNode(ctx context.Context, req *pb.ShareNodeRequest) (*pb.ShareNodeResponse, error) {
	s.shareReq = req
	s.sendTrailer(ctx)
	if s.shareErr != nil {
		return nil, s.shareErr
	}
	return s.shareResp, nil
}

func (s *fakeServer) GetTenantQuota(ctx context.Context, req *pb.GetTenantQuotaRequest) (*pb.GetTenantQuotaResponse, error) {
	s.quotaReq = req
	s.sendTrailer(ctx)
	if s.quotaErr != nil {
		return nil, s.quotaErr
	}
	return s.quotaResp, nil
}

func (s *fakeServer) ExecuteAtomic(ctx context.Context, req *pb.ExecuteAtomicRequest) (*pb.ExecuteAtomicResponse, error) {
	s.execReq = req
	s.sendTrailer(ctx)
	if s.execErr != nil {
		return nil, s.execErr
	}
	return s.execResp, nil
}

// startFakeServer spins up an in-process gRPC server over
// bufconn and returns a connected grpcTransport pointing at it.
// Cleanup is registered on the *testing.T so individual tests
// stay terse.
func startFakeServer(t *testing.T, svc *fakeServer) *grpcTransport {
	t.Helper()
	lis := bufconn.Listen(1 << 16)
	srv := grpc.NewServer()
	pb.RegisterEntDBServiceServer(srv, svc)

	go func() {
		_ = srv.Serve(lis)
	}()
	t.Cleanup(func() { srv.Stop() })

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return &grpcTransport{
		address: "bufnet",
		config:  defaultConfig(),
		conn:    conn,
		client:  pb.NewEntDBServiceClient(conn),
	}
}

// ── Happy-path tests: one per wired RPC ─────────────────────────────

func TestGrpcTransport_GetNode_HappyPath(t *testing.T) {
	payload, err := structpb.NewStruct(map[string]any{
		"1": "WIDGET-1",
		"2": "Widget",
	})
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	svc := &fakeServer{
		getNodeResp: &pb.GetNodeResponse{
			Found: true,
			Node: &pb.Node{
				TenantId:   "acme",
				NodeId:     "node-1",
				TypeId:     201,
				OwnerActor: "user:alice",
				CreatedAt:  1000,
				UpdatedAt:  2000,
				Payload:    payload,
				Acl: []*pb.AclEntry{
					{Grantee: "user:bob", Permission: "read"},
				},
			},
		},
	}
	tr := startFakeServer(t, svc)

	got, err := tr.GetNode(context.Background(), "acme", "user:alice", 201, "node-1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got == nil {
		t.Fatal("GetNode returned nil")
	}
	if got.NodeID != "node-1" || got.TypeID != 201 || got.TenantID != "acme" {
		t.Errorf("Node mismatch: %+v", got)
	}
	if got.Payload["1"] != "WIDGET-1" || got.Payload["2"] != "Widget" {
		t.Errorf("payload mismatch: %+v", got.Payload)
	}
	if got.OwnerActor.Kind() != "user" || got.OwnerActor.ID() != "alice" {
		t.Errorf("owner actor mismatch: %+v", got.OwnerActor)
	}
	if len(got.ACL) != 1 || got.ACL[0].Permission != PermissionRead {
		t.Errorf("acl mismatch: %+v", got.ACL)
	}
	if svc.getNodeReq.GetContext().GetTenantId() != "acme" {
		t.Errorf("server saw tenant_id = %q", svc.getNodeReq.GetContext().GetTenantId())
	}
	if svc.getNodeReq.GetTypeId() != 201 {
		t.Errorf("server saw type_id = %d", svc.getNodeReq.GetTypeId())
	}
}

func TestGrpcTransport_GetNode_NotFoundIsNil(t *testing.T) {
	// Server returns Found=false — the SDK convention is to
	// return (nil, nil), matching the Python SDK.
	svc := &fakeServer{getNodeResp: &pb.GetNodeResponse{Found: false}}
	tr := startFakeServer(t, svc)

	got, err := tr.GetNode(context.Background(), "acme", "user:alice", 201, "missing")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for not-found, got %+v", got)
	}
}

func TestGrpcTransport_GetNodeByKey_HappyPath(t *testing.T) {
	payload, _ := structpb.NewStruct(map[string]any{"1": "alice@example.com"})
	svc := &fakeServer{
		getNodeByKeyResp: &pb.GetNodeByKeyResponse{
			Found: true,
			Node: &pb.Node{
				TenantId: "acme",
				NodeId:   "node-email-1",
				TypeId:   202,
				Payload:  payload,
			},
		},
	}
	tr := startFakeServer(t, svc)

	val := structpb.NewStringValue("alice@example.com")
	got, err := tr.GetNodeByKey(context.Background(), "acme", "user:alice", 202, 1, val)
	if err != nil {
		t.Fatalf("GetNodeByKey: %v", err)
	}
	if got == nil || got.NodeID != "node-email-1" {
		t.Errorf("GetNodeByKey result mismatch: %+v", got)
	}
	if svc.getNodeByKeyReq.GetFieldId() != 1 {
		t.Errorf("server saw field_id = %d", svc.getNodeByKeyReq.GetFieldId())
	}
	if svc.getNodeByKeyReq.GetValue().GetStringValue() != "alice@example.com" {
		t.Errorf("server saw value = %v", svc.getNodeByKeyReq.GetValue())
	}
}

func TestGrpcTransport_QueryNodes_HappyPath(t *testing.T) {
	p1, _ := structpb.NewStruct(map[string]any{"1": "WIDGET-1", "3": float64(999)})
	p2, _ := structpb.NewStruct(map[string]any{"1": "WIDGET-2", "3": float64(1999)})
	svc := &fakeServer{
		queryResp: &pb.QueryNodesResponse{
			Nodes: []*pb.Node{
				{TenantId: "acme", NodeId: "n1", TypeId: 201, Payload: p1},
				{TenantId: "acme", NodeId: "n2", TypeId: 201, Payload: p2},
			},
			HasMore: false,
		},
	}
	tr := startFakeServer(t, svc)

	filter := map[string]any{
		"price_cents": map[string]any{"$gte": 500.0},
		"sku":         "WIDGET-1",
	}
	nodes, err := tr.QueryNodes(context.Background(), "acme", "user:alice", 201, filter)
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(nodes))
	}
	if svc.queryReq.GetTypeId() != 201 {
		t.Errorf("server saw type_id = %d", svc.queryReq.GetTypeId())
	}
	if len(svc.queryReq.GetFilters()) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(svc.queryReq.GetFilters()))
	}
	// Find the price_cents filter and confirm it came through as $gte.
	foundGte := false
	for _, f := range svc.queryReq.GetFilters() {
		if f.GetField() == "price_cents" && f.GetOp() == pb.FilterOp_GTE {
			foundGte = true
			if f.GetValue().GetNumberValue() != 500 {
				t.Errorf("price_cents filter value = %v", f.GetValue())
			}
		}
	}
	if !foundGte {
		t.Errorf("expected a $gte filter on price_cents, got %+v", svc.queryReq.GetFilters())
	}
}

func TestGrpcTransport_SearchNodes_HappyPath(t *testing.T) {
	p, _ := structpb.NewStruct(map[string]any{"2": "Widget Deluxe"})
	svc := &fakeServer{
		searchResp: &pb.SearchNodesResponse{
			Nodes: []*pb.Node{
				{NodeId: "n1", TypeId: 201, Payload: p},
			},
		},
	}
	tr := startFakeServer(t, svc)

	nodes, err := tr.SearchNodes(context.Background(), "acme", "user:alice", 201, "deluxe")
	if err != nil {
		t.Fatalf("SearchNodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(nodes))
	}
	if svc.searchReq.GetQuery() != "deluxe" {
		t.Errorf("server saw query = %q", svc.searchReq.GetQuery())
	}
	if svc.searchReq.GetTypeId() != 201 {
		t.Errorf("server saw type_id = %d", svc.searchReq.GetTypeId())
	}
}

func TestGrpcTransport_GetEdgesFrom_HappyPath(t *testing.T) {
	svc := &fakeServer{
		edgesFromResp: &pb.GetEdgesResponse{
			Edges: []*pb.Edge{
				{TenantId: "acme", EdgeTypeId: 301, FromNodeId: "user-1", ToNodeId: "node-1", CreatedAt: 1000},
			},
		},
	}
	tr := startFakeServer(t, svc)

	edges, err := tr.GetEdgesFrom(context.Background(), "acme", "user:alice", "user-1", 301)
	if err != nil {
		t.Fatalf("GetEdgesFrom: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(edges))
	}
	if edges[0].EdgeTypeID != 301 || edges[0].FromNodeID != "user-1" || edges[0].ToNodeID != "node-1" {
		t.Errorf("edge mismatch: %+v", edges[0])
	}
	if svc.edgesFromReq.GetNodeId() != "user-1" || svc.edgesFromReq.GetEdgeTypeId() != 301 {
		t.Errorf("server saw node_id=%q edge_type_id=%d", svc.edgesFromReq.GetNodeId(), svc.edgesFromReq.GetEdgeTypeId())
	}
}

func TestGrpcTransport_GetEdgesTo_HappyPath(t *testing.T) {
	svc := &fakeServer{
		edgesToResp: &pb.GetEdgesResponse{
			Edges: []*pb.Edge{
				{EdgeTypeId: 301, FromNodeId: "user-5", ToNodeId: "node-1"},
			},
		},
	}
	tr := startFakeServer(t, svc)

	edges, err := tr.GetEdgesTo(context.Background(), "acme", "user:alice", "node-1", 301)
	if err != nil {
		t.Fatalf("GetEdgesTo: %v", err)
	}
	if len(edges) != 1 || edges[0].FromNodeID != "user-5" {
		t.Errorf("edges mismatch: %+v", edges)
	}
	if svc.edgesToReq.GetNodeId() != "node-1" {
		t.Errorf("server saw node_id = %q", svc.edgesToReq.GetNodeId())
	}
}

func TestGrpcTransport_Share_HappyPath(t *testing.T) {
	svc := &fakeServer{shareResp: &pb.ShareNodeResponse{Success: true}}
	tr := startFakeServer(t, svc)

	err := tr.Share(context.Background(), "acme", "user:alice", "node-1", "user:bob", "read")
	if err != nil {
		t.Fatalf("Share: %v", err)
	}
	if svc.shareReq.GetNodeId() != "node-1" || svc.shareReq.GetActorId() != "user:bob" {
		t.Errorf("server request mismatch: %+v", svc.shareReq)
	}
	if svc.shareReq.GetPermission() != "read" {
		t.Errorf("permission = %q, want read", svc.shareReq.GetPermission())
	}
}

func TestGrpcTransport_GetTenantQuota_HappyPath(t *testing.T) {
	svc := &fakeServer{
		quotaResp: &pb.GetTenantQuotaResponse{
			TenantId:               "acme",
			MaxWritesPerMonth:      1_000_000,
			WritesUsed:             42_000,
			PeriodStartMs:          100,
			PeriodEndMs:            200,
			MaxRpsSustained:        50,
			MaxRpsBurst:            100,
			MaxRpsPerUserSustained: 5,
			MaxRpsPerUserBurst:     10,
			HardEnforce:            true,
		},
	}
	tr := startFakeServer(t, svc)

	q, err := tr.GetTenantQuota(context.Background(), "system:admin", "acme")
	if err != nil {
		t.Fatalf("GetTenantQuota: %v", err)
	}
	if q.TenantID != "acme" {
		t.Errorf("tenant id = %q", q.TenantID)
	}
	if q.MaxWritesPerMonth != 1_000_000 || q.WritesUsed != 42_000 {
		t.Errorf("monthly quota mismatch: %+v", q)
	}
	if q.MaxRPSSustained != 50 || q.MaxRPSBurst != 100 {
		t.Errorf("rps quota mismatch: %+v", q)
	}
	if !q.HardEnforce {
		t.Errorf("hard_enforce = false")
	}
}

func TestGrpcTransport_ExecuteAtomic_HappyPath(t *testing.T) {
	svc := &fakeServer{
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
	}
	tr := startFakeServer(t, svc)

	ops := []Operation{
		{
			Type:   OpCreateNode,
			TypeID: 201,
			Alias:  "first",
			Data:   map[string]any{"1": "WIDGET-1", "2": "Widget"},
		},
		{
			Type:   OpUpdateNode,
			TypeID: 201,
			NodeID: "node-1",
			Patch:  map[string]any{"3": float64(1299)},
		},
		{
			Type:   OpDeleteNode,
			TypeID: 201,
			NodeID: "old-node",
		},
		{
			Type:       OpCreateEdge,
			EdgeTypeID: 301,
			FromNodeID: "user-1",
			ToNodeID:   "node-1",
		},
		{
			Type:       OpDeleteEdge,
			EdgeTypeID: 301,
			FromNodeID: "user-1",
			ToNodeID:   "stale",
		},
	}
	result, err := tr.ExecuteAtomic(context.Background(), "acme", "user:alice", "idem-1", ops)
	if err != nil {
		t.Fatalf("ExecuteAtomic: %v", err)
	}
	if !result.Success {
		t.Error("result.Success = false")
	}
	if result.Receipt == nil || result.Receipt.IdempotencyKey != "idem-1" {
		t.Errorf("receipt mismatch: %+v", result.Receipt)
	}
	if !result.Applied {
		t.Error("result.Applied = false")
	}
	if len(result.CreatedNodeIDs) != 1 || result.CreatedNodeIDs[0] != "node-1" {
		t.Errorf("created ids = %v", result.CreatedNodeIDs)
	}
	if len(svc.execReq.GetOperations()) != 5 {
		t.Fatalf("server saw %d ops, want 5", len(svc.execReq.GetOperations()))
	}
	// Verify the first op was a CreateNode with the right shape.
	cn := svc.execReq.GetOperations()[0].GetCreateNode()
	if cn == nil {
		t.Fatal("op[0] is not a CreateNode")
	}
	if cn.GetTypeId() != 201 || cn.GetAs() != "first" {
		t.Errorf("CreateNode mismatch: %+v", cn)
	}
	if got := cn.GetData().AsMap()["1"]; got != "WIDGET-1" {
		t.Errorf("data[1] = %v", got)
	}
	// Verify the edge ops used NodeRef_Id.
	ce := svc.execReq.GetOperations()[3].GetCreateEdge()
	if ce == nil || ce.GetFrom().GetId() != "user-1" || ce.GetTo().GetId() != "node-1" {
		t.Errorf("CreateEdge mismatch: %+v", ce)
	}
}

// ── Error-translation tests: one per mapped status code ─────────────

func TestGrpcTransport_NotFoundError(t *testing.T) {
	svc := &fakeServer{getNodeErr: status.Error(codes.NotFound, "no such node")}
	tr := startFakeServer(t, svc)

	_, err := tr.GetNode(context.Background(), "acme", "user:alice", 201, "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("expected *NotFoundError, got %T: %v", err, err)
	}
	if nf.Message != "no such node" {
		t.Errorf("message = %q", nf.Message)
	}
}

func TestGrpcTransport_PermissionDeniedError(t *testing.T) {
	svc := &fakeServer{getNodeErr: status.Error(codes.PermissionDenied, "insufficient capability")}
	tr := startFakeServer(t, svc)

	_, err := tr.GetNode(context.Background(), "acme", "user:alice", 201, "n1")
	if err == nil {
		t.Fatal("expected error")
	}
	var ad *AccessDeniedError
	if !errors.As(err, &ad) {
		t.Fatalf("expected *AccessDeniedError, got %T: %v", err, err)
	}
	if ad.Code != "ACCESS_DENIED" {
		t.Errorf("code = %q", ad.Code)
	}
}

func TestGrpcTransport_InvalidArgumentError(t *testing.T) {
	svc := &fakeServer{queryErr: status.Error(codes.InvalidArgument, "bad filter")}
	tr := startFakeServer(t, svc)

	_, err := tr.QueryNodes(context.Background(), "acme", "user:alice", 201, map[string]any{"x": "y"})
	if err == nil {
		t.Fatal("expected error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	if ve.Code != "VALIDATION_ERROR" {
		t.Errorf("code = %q", ve.Code)
	}
}

func TestGrpcTransport_ResourceExhaustedError(t *testing.T) {
	svc := &fakeServer{
		searchErr: status.Error(codes.ResourceExhausted, "monthly quota exceeded"),
		trailerKV: [2]string{"retry-after", "60"},
	}
	tr := startFakeServer(t, svc)

	_, err := tr.SearchNodes(context.Background(), "acme", "user:alice", 201, "widget")
	if err == nil {
		t.Fatal("expected error")
	}
	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}
	if rle.RetryAfterMs != 60_000 {
		t.Errorf("retry_after_ms = %d, want 60000", rle.RetryAfterMs)
	}
}

func TestGrpcTransport_AlreadyExistsError(t *testing.T) {
	msg := "Unique constraint violation: type_id=201 field_id=1 value='WIDGET-1' already exists"
	svc := &fakeServer{execErr: status.Error(codes.AlreadyExists, msg)}
	tr := startFakeServer(t, svc)

	_, err := tr.ExecuteAtomic(context.Background(), "acme", "user:alice", "idem-1",
		[]Operation{{Type: OpCreateNode, TypeID: 201, Data: map[string]any{"1": "WIDGET-1"}}})
	if err == nil {
		t.Fatal("expected error")
	}
	var uce *UniqueConstraintError
	if !errors.As(err, &uce) {
		t.Fatalf("expected *UniqueConstraintError, got %T: %v", err, err)
	}
	if uce.TenantID != "acme" {
		t.Errorf("tenant_id = %q", uce.TenantID)
	}
}

func TestGrpcTransport_UnavailableError(t *testing.T) {
	svc := &fakeServer{getNodeErr: status.Error(codes.Unavailable, "server down")}
	tr := startFakeServer(t, svc)

	_, err := tr.GetNode(context.Background(), "acme", "user:alice", 201, "n1")
	if err == nil {
		t.Fatal("expected error")
	}
	var ce *ConnectionError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConnectionError, got %T: %v", err, err)
	}
}

func TestGrpcTransport_NotConnected(t *testing.T) {
	// Direct struct — no Connect call, so client is nil and we
	// should get a ConnectionError instead of a nil-deref.
	tr := &grpcTransport{address: "localhost:50051", config: defaultConfig()}
	_, err := tr.GetNode(context.Background(), "acme", "user:alice", 201, "n1")
	if err == nil {
		t.Fatal("expected error from disconnected transport")
	}
	var ce *ConnectionError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConnectionError, got %T: %v", err, err)
	}
}

func TestGrpcTransport_ContextDeadline(t *testing.T) {
	// Blocking server — the test cancels via its context.
	svc := &fakeServer{}
	tr := startFakeServer(t, svc)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Server never populates a response so the client's context
	// deadline fires. We verify the mapper turns codes.DeadlineExceeded
	// into a ConnectionError. To force a deadline we set up a
	// channel-based blocker; simpler path — call a real RPC against
	// an already-cancelled ctx so the gRPC client surfaces
	// DeadlineExceeded without involving the server.
	immediateCtx, cancel2 := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel2()
	_, err := tr.GetNode(immediateCtx, "acme", "user:alice", 201, "n1")
	if err == nil {
		t.Fatal("expected deadline error")
	}
	var ce *ConnectionError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConnectionError, got %T: %v", err, err)
	}
	_ = ctx
}
