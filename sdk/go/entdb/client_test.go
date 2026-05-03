package entdb

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

// mockTransport implements Transport for testing without a live gRPC server.
//
// It records the last-seen arguments for the operations the tests
// care about (typeID, fieldID, committed ops) so assertions can
// verify the SDK wrote the expected shape without a live server.
type mockTransport struct {
	connectCalled     bool
	closeCalled       bool
	getNodeCalls      int
	getNodeByKeyCalls int
	queryCalls        int
	commitCalls       int

	edgesFromCalls int
	edgesToCalls   int

	// Last-seen arguments.
	lastGetTypeID         int
	lastGetNodeID         string
	lastGetByKeyTypeID    int32
	lastGetByKeyFieldID   int32
	lastGetByKeyValue     *structpb.Value
	lastQueryTypeID       int
	lastQueryFilter       map[string]any
	lastCommitOperations  []Operation
	lastCommitIdempotency string

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

func (m *mockTransport) GetNode(_ context.Context, _, _ string, typeID int, nodeID string) (*Node, error) {
	m.getNodeCalls++
	m.lastGetTypeID = typeID
	m.lastGetNodeID = nodeID
	return m.getNodeResp, m.getNodeErr
}

func (m *mockTransport) GetNodeByKey(_ context.Context, _, _ string, typeID, fieldID int32, value *structpb.Value) (*Node, error) {
	m.getNodeByKeyCalls++
	m.lastGetByKeyTypeID = typeID
	m.lastGetByKeyFieldID = fieldID
	m.lastGetByKeyValue = value
	return m.getNodeByKeyResp, m.getNodeByKeyErr
}

func (m *mockTransport) QueryNodes(_ context.Context, _, _ string, typeID int, filter map[string]any) ([]*Node, error) {
	m.queryCalls++
	m.lastQueryTypeID = typeID
	m.lastQueryFilter = filter
	return m.queryResp, m.queryErr
}

func (m *mockTransport) ExecuteAtomic(_ context.Context, _, _, idempotencyKey string, ops []Operation) (*CommitResult, error) {
	m.commitCalls++
	m.lastCommitIdempotency = idempotencyKey
	m.lastCommitOperations = append([]Operation(nil), ops...)
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

func (m *mockTransport) SearchNodes(_ context.Context, _, _ string, _ int, _ string) ([]*Node, error) {
	return nil, nil
}

func (m *mockTransport) GetTenantQuota(_ context.Context, _, _ string) (*TenantQuota, error) {
	return nil, nil
}

func (m *mockTransport) GetNodes(_ context.Context, _, _ string, _ int, _ []string) ([]*Node, []string, error) {
	return nil, nil, nil
}

func (m *mockTransport) GetReceiptStatus(_ context.Context, _, _, _ string) (ReceiptStatus, string, error) {
	return ReceiptStatusUnknown, "", nil
}

func (m *mockTransport) WaitForOffset(_ context.Context, _, _, _ string, _ int32) (bool, string, error) {
	return false, "", nil
}

func (m *mockTransport) GetConnectedNodes(_ context.Context, _, _, _ string, _ int) ([]*Node, error) {
	return nil, nil
}

func (m *mockTransport) ListSharedWithMe(_ context.Context, _, _ string, _, _ int32) ([]*Node, error) {
	return nil, nil
}

func (m *mockTransport) RevokeAccess(_ context.Context, _, _, _, _ string) (bool, error) {
	return false, nil
}

func (m *mockTransport) TransferOwnership(_ context.Context, _, _, _, _ string) (bool, error) {
	return false, nil
}

func (m *mockTransport) AddGroupMember(_ context.Context, _, _, _, _, _ string) error {
	return nil
}

func (m *mockTransport) RemoveGroupMember(_ context.Context, _, _, _, _ string) error {
	return nil
}

func (m *mockTransport) CreateTenant(_ context.Context, _, _, _ string) (*TenantDetail, error) {
	return nil, nil
}

func (m *mockTransport) CreateUser(_ context.Context, _, _, _, _ string) (*UserInfo, error) {
	return nil, nil
}

func (m *mockTransport) AddTenantMember(_ context.Context, _, _, _, _ string) error {
	return nil
}

func (m *mockTransport) RemoveTenantMember(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *mockTransport) ChangeMemberRole(_ context.Context, _, _, _, _ string) error {
	return nil
}

func (m *mockTransport) GetTenantMembers(_ context.Context, _, _ string) ([]TenantMember, error) {
	return nil, nil
}

func (m *mockTransport) GetUserTenants(_ context.Context, _, _ string) ([]TenantMember, error) {
	return nil, nil
}

func (m *mockTransport) DelegateAccess(_ context.Context, _, _, _, _, _ string, _ int64) (*DelegateResult, error) {
	return nil, nil
}

func (m *mockTransport) TransferUserContent(_ context.Context, _, _, _, _ string) (int32, error) {
	return 0, nil
}

func (m *mockTransport) RevokeAllUserAccess(_ context.Context, _, _, _ string) (*RevokeAllResult, error) {
	return nil, nil
}

func (m *mockTransport) DeleteUser(_ context.Context, _, _ string, _ int32) (*DeletionScheduled, error) {
	return nil, nil
}

func (m *mockTransport) CancelUserDeletion(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockTransport) ExportUserData(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (m *mockTransport) FreezeUser(_ context.Context, _, _ string, _ bool) (string, error) {
	return "", nil
}

func (m *mockTransport) Health(_ context.Context) (*HealthStatus, error) {
	return nil, nil
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
