package entdb

import (
	"context"
	"testing"
	"time"
)

// mockTransport implements Transport for testing without a live gRPC server.
type mockTransport struct {
	connectCalled     bool
	closeCalled       bool
	getNodeCalls      int
	getNodeByKeyCalls int
	queryCalls        int
	commitCalls       int

	edgesFromCalls int
	edgesToCalls   int

	// Responses for stubbing
	getNodeResp      *Node
	getNodeErr       error
	getNodeByKeyResp *Node
	getNodeByKeyErr  error
	queryResp        []*Node
	queryErr         error
	commitResp       *CommitResult
	commitErr        error
	connectErr       error
	edgesFromResp    []*Edge
	edgesToResp      []*Edge
}

func (m *mockTransport) Connect(_ context.Context) error {
	m.connectCalled = true
	return m.connectErr
}

func (m *mockTransport) Close() error {
	m.closeCalled = true
	return nil
}

func (m *mockTransport) GetNode(_ context.Context, _, _ string, _ int, _ string) (*Node, error) {
	m.getNodeCalls++
	return m.getNodeResp, m.getNodeErr
}

func (m *mockTransport) GetNodeByKey(_ context.Context, _, _ string, _ int, _, _ string) (*Node, error) {
	m.getNodeByKeyCalls++
	return m.getNodeByKeyResp, m.getNodeByKeyErr
}

func (m *mockTransport) QueryNodes(_ context.Context, _, _ string, _ int, _ map[string]any) ([]*Node, error) {
	m.queryCalls++
	return m.queryResp, m.queryErr
}

func (m *mockTransport) ExecuteAtomic(_ context.Context, _, _, _ string, _ []Operation) (*CommitResult, error) {
	m.commitCalls++
	return m.commitResp, m.commitErr
}

func (m *mockTransport) Share(_ context.Context, _, _, _, _, _ string) error {
	return nil
}

func (m *mockTransport) GetEdgesFrom(_ context.Context, _, _, _ string, _ int) ([]*Edge, error) {
	m.edgesFromCalls++
	return m.edgesFromResp, nil
}

func (m *mockTransport) GetEdgesTo(_ context.Context, _, _, _ string, _ int) ([]*Edge, error) {
	m.edgesToCalls++
	return m.edgesToResp, nil
}

func newTestClient(t *testing.T, transport *mockTransport, opts ...ClientOption) *DbClient {
	t.Helper()
	return newClientWithTransport("localhost:50051", transport, opts...)
}

func TestNewClient_ValidAddress(t *testing.T) {
	client, err := NewClient("localhost:50051")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.Address() != "localhost:50051" {
		t.Errorf("expected address localhost:50051, got %s", client.Address())
	}
}

func TestNewClient_EmptyAddress(t *testing.T) {
	_, err := NewClient("")
	if err == nil {
		t.Fatal("expected error for empty address")
	}
	connErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("expected ConnectionError, got %T", err)
	}
	if connErr.Code != "CONNECTION_ERROR" {
		t.Errorf("expected code CONNECTION_ERROR, got %s", connErr.Code)
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	tests := []struct {
		name       string
		opts       []ClientOption
		wantSecure bool
		wantKey    string
		wantRetry  int
	}{
		{
			name:       "defaults",
			opts:       nil,
			wantSecure: false,
			wantKey:    "",
			wantRetry:  3,
		},
		{
			name:       "with secure",
			opts:       []ClientOption{WithSecure()},
			wantSecure: true,
			wantKey:    "",
			wantRetry:  3,
		},
		{
			name:       "with api key",
			opts:       []ClientOption{WithAPIKey("sk-test-123")},
			wantSecure: false,
			wantKey:    "sk-test-123",
			wantRetry:  3,
		},
		{
			name:       "with max retries",
			opts:       []ClientOption{WithMaxRetries(5)},
			wantSecure: false,
			wantKey:    "",
			wantRetry:  5,
		},
		{
			name:       "all options combined",
			opts:       []ClientOption{WithSecure(), WithAPIKey("key"), WithMaxRetries(10)},
			wantSecure: true,
			wantKey:    "key",
			wantRetry:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient("localhost:50051", tt.opts...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			cfg := client.Config()
			if cfg.secure != tt.wantSecure {
				t.Errorf("secure = %v, want %v", cfg.secure, tt.wantSecure)
			}
			if cfg.apiKey != tt.wantKey {
				t.Errorf("apiKey = %q, want %q", cfg.apiKey, tt.wantKey)
			}
			if cfg.maxRetries != tt.wantRetry {
				t.Errorf("maxRetries = %d, want %d", cfg.maxRetries, tt.wantRetry)
			}
		})
	}
}

func TestWithTimeout(t *testing.T) {
	client, err := NewClient("localhost:50051", WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := client.Config()
	if cfg.timeout != 5*time.Second {
		t.Errorf("timeout = %v, want %v", cfg.timeout, 5*time.Second)
	}
}

func TestClient_ConnectAndClose(t *testing.T) {
	mock := &mockTransport{}
	client := newTestClient(t, mock)

	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	if !mock.connectCalled {
		t.Error("expected Connect to be called on transport")
	}

	if err := client.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if !mock.closeCalled {
		t.Error("expected Close to be called on transport")
	}
}

func TestClient_ConnectError(t *testing.T) {
	mock := &mockTransport{
		connectErr: NewConnectionError("refused", "localhost:50051"),
	}
	client := newTestClient(t, mock)

	err := client.Connect(context.Background())
	if err == nil {
		t.Fatal("expected connect error")
	}
	connErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("expected *ConnectionError, got %T", err)
	}
	if connErr.Address != "localhost:50051" {
		t.Errorf("expected address localhost:50051, got %s", connErr.Address)
	}
}

func TestClient_Get_DelegatesToTransport(t *testing.T) {
	expected := &Node{
		TenantID: "t1",
		NodeID:   "n1",
		TypeID:   1,
		Payload:  map[string]any{"title": "Hello"},
	}
	mock := &mockTransport{getNodeResp: expected}
	client := newTestClient(t, mock)

	node, err := client.Get(context.Background(), "t1", "user:alice", 1, "n1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.NodeID != "n1" {
		t.Errorf("expected node ID n1, got %s", node.NodeID)
	}
	if mock.getNodeCalls != 1 {
		t.Errorf("expected 1 GetNode call, got %d", mock.getNodeCalls)
	}
}

func TestClient_Query_DelegatesToTransport(t *testing.T) {
	expected := []*Node{
		{NodeID: "n1", TypeID: 1},
		{NodeID: "n2", TypeID: 1},
	}
	mock := &mockTransport{queryResp: expected}
	client := newTestClient(t, mock)

	nodes, err := client.Query(context.Background(), "t1", "user:alice", 1, map[string]any{"status": "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
	if mock.queryCalls != 1 {
		t.Errorf("expected 1 QueryNodes call, got %d", mock.queryCalls)
	}
}

func TestClient_Tenant_ReturnsTenantScope(t *testing.T) {
	mock := &mockTransport{}
	client := newTestClient(t, mock)

	scope := client.Tenant("acme")
	if scope.TenantID() != "acme" {
		t.Errorf("expected tenant ID acme, got %s", scope.TenantID())
	}
}

func TestClient_NewPlan_ReturnsPlan(t *testing.T) {
	mock := &mockTransport{}
	client := newTestClient(t, mock)

	plan := client.NewPlan("t1", "user:alice")
	if plan.TenantID() != "t1" {
		t.Errorf("expected tenant ID t1, got %s", plan.TenantID())
	}
	if plan.Actor() != "user:alice" {
		t.Errorf("expected actor user:alice, got %s", plan.Actor())
	}
}

func TestClient_NewPlanWithKey(t *testing.T) {
	mock := &mockTransport{}
	client := newTestClient(t, mock)

	plan := client.NewPlanWithKey("t1", "user:alice", "idem-key-1")
	if plan.IdempotencyKey() != "idem-key-1" {
		t.Errorf("expected idempotency key idem-key-1, got %s", plan.IdempotencyKey())
	}
}

func TestClient_EdgesFrom_DelegatesToTransport(t *testing.T) {
	expected := []*Edge{
		{EdgeTypeID: 201, FromNodeID: "n1", ToNodeID: "n2"},
		{EdgeTypeID: 201, FromNodeID: "n1", ToNodeID: "n3"},
	}
	mock := &mockTransport{edgesFromResp: expected}
	client := newTestClient(t, mock)

	edges, err := client.EdgesFrom(context.Background(), "t1", "user:alice", "n1", 201)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(edges))
	}
	if mock.edgesFromCalls != 1 {
		t.Errorf("expected 1 EdgesFrom call, got %d", mock.edgesFromCalls)
	}
}

func TestClient_EdgesTo_DelegatesToTransport(t *testing.T) {
	expected := []*Edge{
		{EdgeTypeID: 201, FromNodeID: "n5", ToNodeID: "n1"},
	}
	mock := &mockTransport{edgesToResp: expected}
	client := newTestClient(t, mock)

	edges, err := client.EdgesTo(context.Background(), "t1", "user:alice", "n1", 201)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(edges))
	}
	if mock.edgesToCalls != 1 {
		t.Errorf("expected 1 EdgesTo call, got %d", mock.edgesToCalls)
	}
}

func TestScope_EdgesFromTo(t *testing.T) {
	mock := &mockTransport{
		edgesFromResp: []*Edge{{EdgeTypeID: 201, FromNodeID: "n1", ToNodeID: "n2"}},
		edgesToResp:   []*Edge{{EdgeTypeID: 201, FromNodeID: "n5", ToNodeID: "n1"}},
	}
	client := newTestClient(t, mock)
	scope := client.Tenant("acme").Actor(UserActor("alice"))

	out, err := scope.EdgesFrom(context.Background(), "n1", 201)
	if err != nil {
		t.Fatalf("EdgesFrom: %v", err)
	}
	if len(out) != 1 || out[0].ToNodeID != "n2" {
		t.Errorf("unexpected EdgesFrom result: %+v", out)
	}

	in, err := scope.EdgesTo(context.Background(), "n1", 201)
	if err != nil {
		t.Fatalf("EdgesTo: %v", err)
	}
	if len(in) != 1 || in[0].FromNodeID != "n5" {
		t.Errorf("unexpected EdgesTo result: %+v", in)
	}

	if mock.edgesFromCalls != 1 || mock.edgesToCalls != 1 {
		t.Errorf("expected 1 call each, got from=%d to=%d", mock.edgesFromCalls, mock.edgesToCalls)
	}
}
