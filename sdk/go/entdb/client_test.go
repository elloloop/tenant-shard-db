// SPDX-License-Identifier: MIT
package entdb

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
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
	lastGetByKeyValue     any
	lastQueryTypeID       int
	lastQueryFilter       map[string]any
	lastQueryLimit        int
	lastCommitOperations  []Operation
	lastCommitIdempotency string
	// lastWaitAppliedSet records whether the caller passed
	// WithWaitApplied; lastWaitApplied is the value (issue #606).
	lastWaitApplied    bool
	lastWaitAppliedSet bool

	// WaitForOffset capture for the issue #600 WaitForCommit helper.
	waitOffsetCalls         int
	lastWaitOffsetStreamPos string
	lastWaitOffsetTimeoutMs int32
	waitOffsetReached       bool

	// Mailbox read tracking (#568).
	lastMailboxTargetUser string
	mailboxGetResp        *Node
	mailboxQueryResp      []*Node
	mailboxSearchResp     []*Node

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

func (m *mockTransport) GetNodeByKey(_ context.Context, _, _ string, typeID, fieldID int32, value any) (*Node, error) {
	m.getNodeByKeyCalls++
	m.lastGetByKeyTypeID = typeID
	m.lastGetByKeyFieldID = fieldID
	m.lastGetByKeyValue = value
	return m.getNodeByKeyResp, m.getNodeByKeyErr
}

func (m *mockTransport) QueryNodes(_ context.Context, _, _ string, typeID int, filter map[string]any, limit int) ([]*Node, error) {
	m.queryCalls++
	m.lastQueryTypeID = typeID
	m.lastQueryFilter = filter
	m.lastQueryLimit = limit
	return m.queryResp, m.queryErr
}

func (m *mockTransport) ExecuteAtomic(_ context.Context, _, _, idempotencyKey string, ops []Operation, opts ...CommitOption) (*CommitResult, error) {
	m.commitCalls++
	m.lastCommitIdempotency = idempotencyKey
	m.lastCommitOperations = append([]Operation(nil), ops...)
	// Record the resolved commit options so race tests can inspect what
	// Plan.Commit propagated (issue #606).
	mco := commitOpts{}
	for _, o := range opts {
		o(&mco)
	}
	if mco.waitApplied != nil {
		m.lastWaitApplied = *mco.waitApplied
		m.lastWaitAppliedSet = true
	} else {
		m.lastWaitApplied = false
		m.lastWaitAppliedSet = false
	}
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

func (m *mockTransport) SearchNodes(_ context.Context, _, _ string, _ int, _ string, _, _ int32) ([]*Node, bool, error) {
	return nil, false, nil
}

func (m *mockTransport) GetTenantQuota(_ context.Context, _, _ string) (*TenantQuota, error) {
	return nil, nil
}

func (m *mockTransport) GetNodes(_ context.Context, _, _ string, _ int, _ []string) ([]*Node, []string, error) {
	return nil, nil, nil
}

func (m *mockTransport) GetMailboxNode(_ context.Context, _, _, targetUser string, typeID int, nodeID string) (*Node, error) {
	m.lastMailboxTargetUser = targetUser
	m.lastGetTypeID = typeID
	m.lastGetNodeID = nodeID
	return m.mailboxGetResp, nil
}

func (m *mockTransport) QueryMailboxNodes(_ context.Context, _, _, targetUser string, typeID int, filter map[string]any, limit int) ([]*Node, error) {
	m.lastMailboxTargetUser = targetUser
	m.lastQueryTypeID = typeID
	m.lastQueryFilter = filter
	m.lastQueryLimit = limit
	return m.mailboxQueryResp, nil
}

func (m *mockTransport) SearchMailboxNodes(_ context.Context, _, _, targetUser string, _ int, _ string) ([]*Node, error) {
	m.lastMailboxTargetUser = targetUser
	return m.mailboxSearchResp, nil
}

func (m *mockTransport) GetReceiptStatus(_ context.Context, _, _, _ string) (ReceiptStatus, string, error) {
	return ReceiptStatusUnknown, "", nil
}

func (m *mockTransport) WaitForOffset(_ context.Context, _, _, streamPosition string, timeoutMs int32) (bool, string, error) {
	m.lastWaitOffsetStreamPos = streamPosition
	m.lastWaitOffsetTimeoutMs = timeoutMs
	m.waitOffsetCalls++
	return m.waitOffsetReached, streamPosition, nil
}

func (m *mockTransport) GetConnectedNodes(_ context.Context, _, _, _ string, _ int) ([]*Node, error) {
	return nil, nil
}

func (m *mockTransport) ListSharedWithMe(_ context.Context, _, _ string, _ int32) ([]*Node, error) {
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

func (m *mockTransport) CreateTenant(_ context.Context, _, _, _ string, _ ...CreateTenantOption) (*TenantDetail, error) {
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

func TestClient_Transport_ReturnsUnderlyingTransport(t *testing.T) {
	mt := &mockTransport{}
	client := newTestClient(t, mt)

	got := client.Transport()
	if got == nil {
		t.Fatal("expected non-nil Transport, got nil")
	}
	if got != Transport(mt) {
		t.Errorf("expected Transport to return the injected transport %p, got %p", mt, got)
	}
}

func TestClient_Transport_RealClient(t *testing.T) {
	client, err := NewClient("localhost:50051")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.Transport() == nil {
		t.Fatal("expected non-nil Transport for a real client, got nil")
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

func TestClientInterceptorOptions(t *testing.T) {
	unary := func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	stream := func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		return streamer(ctx, desc, cc, method, opts...)
	}

	client, err := NewClient(
		"localhost:50051",
		WithUnaryClientInterceptors(nil, unary, unary),
		WithStreamClientInterceptors(nil, stream),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := client.Config()
	if got := len(cfg.unaryClientInterceptors); got != 2 {
		t.Fatalf("unaryClientInterceptors len = %d, want 2", got)
	}
	if got := len(cfg.streamClientInterceptors); got != 1 {
		t.Fatalf("streamClientInterceptors len = %d, want 1", got)
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

// ── Issue #607: serverFingerprint race ──────────────────────────────

// TestGrpcTransport_ServerFingerprintConcurrentAccess is the regression
// guard for issue #607. The grpcTransport's serverFingerprint was a
// plain string mutated without synchronisation; multiple goroutines
// hitting their first ExecuteAtomic concurrently raced on the field
// and tripped Go's -race detector even though the logical outcome was
// fine. The fix wraps reads/writes in an RWMutex (loadServerFingerprint
// / storeServerFingerprint). This test fans out N concurrent reader +
// writer goroutines against those accessors; the test target is
// `go test -race` running clean.
func TestGrpcTransport_ServerFingerprintConcurrentAccess(t *testing.T) {
	tr := &grpcTransport{}
	const goroutines = 32
	const iterations = 1000
	done := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		// Half readers, half writers — exactly the shape that bit a
		// real consumer (their conformance suite fans out N goroutines
		// doing first-time writes, every one of which reads the
		// fingerprint to decide on schema attach and then writes it
		// after the server confirms).
		if i%2 == 0 {
			go func() {
				for j := 0; j < iterations; j++ {
					_ = tr.loadServerFingerprint()
				}
				done <- struct{}{}
			}()
		} else {
			go func(seed int) {
				for j := 0; j < iterations; j++ {
					tr.storeServerFingerprint("sha256:fp")
				}
				done <- struct{}{}
			}(i)
		}
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
	if got := tr.loadServerFingerprint(); got != "sha256:fp" {
		t.Errorf("final fingerprint = %q; want sha256:fp", got)
	}
}

// ── Issue #600: offset-based read-after-write primitive ─────────────

// TestDbClient_WaitForCommit_PassesCommittedOffsetThroughToTransport
// pins the wiring so a future refactor cannot silently regress the
// offset-based wait into a no-op. Issue #600.
func TestDbClient_WaitForCommit_PassesCommittedOffsetThroughToTransport(t *testing.T) {
	mock := &mockTransport{waitOffsetReached: true}
	client := newTestClient(t, mock)
	result := &CommitResult{
		Success:         true,
		CommittedOffset: "wal:0:42",
	}
	ok, pos, err := client.WaitForCommit(context.Background(), "acme", "user:alice", result, 5000)
	if err != nil {
		t.Fatalf("WaitForCommit: %v", err)
	}
	if !ok {
		t.Errorf("ok = false; want true (mock said reached)")
	}
	if pos != "wal:0:42" {
		t.Errorf("returned position = %q, want wal:0:42 (the echoed offset)", pos)
	}
	if mock.waitOffsetCalls != 1 {
		t.Errorf("WaitForOffset calls = %d, want 1", mock.waitOffsetCalls)
	}
	if mock.lastWaitOffsetStreamPos != "wal:0:42" {
		t.Errorf("transport saw streamPosition = %q, want wal:0:42 — the CommittedOffset shortcut didn't propagate", mock.lastWaitOffsetStreamPos)
	}
	if mock.lastWaitOffsetTimeoutMs != 5000 {
		t.Errorf("transport saw timeoutMs = %d, want 5000", mock.lastWaitOffsetTimeoutMs)
	}
}

// TestDbClient_WaitForCommit_NilReceiptIsNoOp pins the safety guard:
// passing a nil / empty-offset CommitResult must not call the server.
func TestDbClient_WaitForCommit_NilReceiptIsNoOp(t *testing.T) {
	mock := &mockTransport{}
	client := newTestClient(t, mock)
	ok, _, err := client.WaitForCommit(context.Background(), "acme", "user:alice", nil, 5000)
	if err != nil || ok {
		t.Errorf("nil CommitResult: ok=%v err=%v; want (false, nil)", ok, err)
	}
	if mock.waitOffsetCalls != 0 {
		t.Errorf("server should not have been called; got %d calls", mock.waitOffsetCalls)
	}
	// Empty CommittedOffset must also short-circuit.
	ok, _, err = client.WaitForCommit(context.Background(), "acme", "user:alice", &CommitResult{Success: true}, 5000)
	if err != nil || ok {
		t.Errorf("empty CommittedOffset: ok=%v err=%v; want (false, nil)", ok, err)
	}
	if mock.waitOffsetCalls != 0 {
		t.Errorf("server still should not have been called; got %d calls", mock.waitOffsetCalls)
	}
}

// TestDbClient_WaitForCommit_DefaultTimeout pins that timeoutMs <= 0
// falls back to the 30s server default.
func TestDbClient_WaitForCommit_DefaultTimeout(t *testing.T) {
	mock := &mockTransport{waitOffsetReached: true}
	client := newTestClient(t, mock)
	_, _, err := client.WaitForCommit(context.Background(), "acme", "user:alice",
		&CommitResult{CommittedOffset: "wal:0:1"}, 0)
	if err != nil {
		t.Fatalf("WaitForCommit: %v", err)
	}
	if mock.lastWaitOffsetTimeoutMs != 30000 {
		t.Errorf("timeoutMs = %d, want 30000 (default fallback)", mock.lastWaitOffsetTimeoutMs)
	}
}
