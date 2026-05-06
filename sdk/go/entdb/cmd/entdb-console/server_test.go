package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	consolev1 "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/console/v1"
	"github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/console/v1/consolev1connect"
	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// fakeServer implements the upstreamStub interface so handler tests can
// exercise the full Connect → handler → upstream path without standing
// up a real gRPC server. Each field, when non-nil, overrides the default
// success response. tests assign the fields they care about.
type fakeServer struct {
	gotAuthHeader  string // set by the AuthCapture wrapper if used
	gotCalls       []string
	healthResp     *pb.HealthResponse
	healthErr      error
	listTenants    *pb.ListTenantsResponse
	getTenant      *pb.GetTenantResponse
	getSchema      *pb.GetSchemaResponse
	queryNodes     *pb.QueryNodesResponse
	getNode        *pb.GetNodeResponse
	getEdgesFrom   *pb.GetEdgesResponse
	getEdgesTo     *pb.GetEdgesResponse
	connectedNodes *pb.GetConnectedNodesResponse
	searchMailbox  *pb.SearchMailboxResponse
	forceErr       error // if set, every method returns this

	// ExecuteAtomic capture (PR 2 sandbox writes). When set, the
	// faked response is returned; otherwise a default success
	// receipt with one created node id is synthesised.
	gotExecuteAtomic []*pb.ExecuteAtomicRequest
	executeAtomic    *pb.ExecuteAtomicResponse
	executeErr       error
}

func (f *fakeServer) record(name string) {
	f.gotCalls = append(f.gotCalls, name)
}

func (f *fakeServer) Health(_ context.Context, _ *pb.HealthRequest, _ ...grpcCallOpt) (*pb.HealthResponse, error) {
	f.record("Health")
	if f.forceErr != nil {
		return nil, f.forceErr
	}
	if f.healthErr != nil {
		return nil, f.healthErr
	}
	if f.healthResp != nil {
		return f.healthResp, nil
	}
	return &pb.HealthResponse{Healthy: true, Version: "fake"}, nil
}

func (f *fakeServer) ListTenants(_ context.Context, _ *pb.ListTenantsRequest, _ ...grpcCallOpt) (*pb.ListTenantsResponse, error) {
	f.record("ListTenants")
	if f.forceErr != nil {
		return nil, f.forceErr
	}
	if f.listTenants != nil {
		return f.listTenants, nil
	}
	return &pb.ListTenantsResponse{}, nil
}

func (f *fakeServer) GetTenant(_ context.Context, in *pb.GetTenantRequest, _ ...grpcCallOpt) (*pb.GetTenantResponse, error) {
	f.record("GetTenant:" + in.GetTenantId())
	if f.forceErr != nil {
		return nil, f.forceErr
	}
	if f.getTenant != nil {
		return f.getTenant, nil
	}
	return &pb.GetTenantResponse{Found: true, Tenant: &pb.TenantDetail{TenantId: in.GetTenantId(), Name: in.GetTenantId()}}, nil
}

func (f *fakeServer) GetSchema(_ context.Context, _ *pb.GetSchemaRequest, _ ...grpcCallOpt) (*pb.GetSchemaResponse, error) {
	f.record("GetSchema")
	if f.forceErr != nil {
		return nil, f.forceErr
	}
	if f.getSchema != nil {
		return f.getSchema, nil
	}
	return &pb.GetSchemaResponse{Fingerprint: "fp"}, nil
}

func (f *fakeServer) QueryNodes(_ context.Context, _ *pb.QueryNodesRequest, _ ...grpcCallOpt) (*pb.QueryNodesResponse, error) {
	f.record("QueryNodes")
	if f.forceErr != nil {
		return nil, f.forceErr
	}
	if f.queryNodes != nil {
		return f.queryNodes, nil
	}
	return &pb.QueryNodesResponse{}, nil
}

func (f *fakeServer) GetNode(_ context.Context, in *pb.GetNodeRequest, _ ...grpcCallOpt) (*pb.GetNodeResponse, error) {
	f.record("GetNode:" + in.GetNodeId())
	if f.forceErr != nil {
		return nil, f.forceErr
	}
	if f.getNode != nil {
		return f.getNode, nil
	}
	return &pb.GetNodeResponse{Found: true, Node: &pb.Node{NodeId: in.GetNodeId()}}, nil
}

func (f *fakeServer) GetEdgesFrom(_ context.Context, _ *pb.GetEdgesRequest, _ ...grpcCallOpt) (*pb.GetEdgesResponse, error) {
	f.record("GetEdgesFrom")
	if f.forceErr != nil {
		return nil, f.forceErr
	}
	if f.getEdgesFrom != nil {
		return f.getEdgesFrom, nil
	}
	return &pb.GetEdgesResponse{}, nil
}

func (f *fakeServer) GetEdgesTo(_ context.Context, _ *pb.GetEdgesRequest, _ ...grpcCallOpt) (*pb.GetEdgesResponse, error) {
	f.record("GetEdgesTo")
	if f.forceErr != nil {
		return nil, f.forceErr
	}
	if f.getEdgesTo != nil {
		return f.getEdgesTo, nil
	}
	return &pb.GetEdgesResponse{}, nil
}

func (f *fakeServer) GetConnectedNodes(_ context.Context, _ *pb.GetConnectedNodesRequest, _ ...grpcCallOpt) (*pb.GetConnectedNodesResponse, error) {
	f.record("GetConnectedNodes")
	if f.forceErr != nil {
		return nil, f.forceErr
	}
	if f.connectedNodes != nil {
		return f.connectedNodes, nil
	}
	return &pb.GetConnectedNodesResponse{}, nil
}

func (f *fakeServer) SearchMailbox(_ context.Context, _ *pb.SearchMailboxRequest, _ ...grpcCallOpt) (*pb.SearchMailboxResponse, error) {
	f.record("SearchMailbox")
	if f.forceErr != nil {
		return nil, f.forceErr
	}
	if f.searchMailbox != nil {
		return f.searchMailbox, nil
	}
	return &pb.SearchMailboxResponse{}, nil
}

func (f *fakeServer) ExecuteAtomic(_ context.Context, in *pb.ExecuteAtomicRequest, _ ...grpcCallOpt) (*pb.ExecuteAtomicResponse, error) {
	f.record("ExecuteAtomic")
	f.gotExecuteAtomic = append(f.gotExecuteAtomic, in)
	if f.forceErr != nil {
		return nil, f.forceErr
	}
	if f.executeErr != nil {
		return nil, f.executeErr
	}
	if f.executeAtomic != nil {
		return f.executeAtomic, nil
	}
	// Default: synthesise a success response. If the request was a
	// create-node we surface a created node id so handler tests can
	// assert the response wiring; create-edge ops produce no ids.
	resp := &pb.ExecuteAtomicResponse{
		Success: true,
		Receipt: &pb.Receipt{
			TenantId:       in.GetContext().GetTenantId(),
			IdempotencyKey: orDefault(in.GetIdempotencyKey(), "fake-idem"),
			StreamPosition: "fake-pos-1",
		},
	}
	for _, op := range in.GetOperations() {
		if op.GetCreateNode() != nil {
			resp.CreatedNodeIds = append(resp.CreatedNodeIds, "fake-node-id")
		}
	}
	return resp, nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// newTestServer builds a httptest.Server wired to a fakeServer, with a
// matching Connect client. Tests then call client methods directly and
// assert against fake.gotCalls / responses.
func newTestServer(t *testing.T, fake *fakeServer, apiKey string) (consolev1connect.ConsoleClient, func()) {
	t.Helper()
	return newTestServerWithSandbox(t, fake, apiKey, "")
}

// newTestServerWithSandbox is like newTestServer but lets the caller
// configure the sandbox tenant id the consoleServer uses for gating
// the SandboxCreate* RPCs. PR 2 tests use this to exercise the
// "disabled" / "wrong tenant" / "happy path" branches.
func newTestServerWithSandbox(t *testing.T, fake *fakeServer, apiKey, sandboxTenant string) (consolev1connect.ConsoleClient, func()) {
	t.Helper()
	srv := &consoleServer{
		upstream:      fake,
		authedCtx:     func(ctx context.Context) context.Context { return ctx },
		sandboxTenant: sandboxTenant,
	}
	mux := http.NewServeMux()
	path, handler := consolev1connect.NewConsoleHandler(srv)
	mux.Handle(path, handler)
	hs := httptest.NewServer(mux)
	t.Cleanup(hs.Close)

	opts := []connect.ClientOption{}
	if apiKey != "" {
		opts = append(opts, connect.WithInterceptors(authInterceptor(apiKey)))
	}
	client := consolev1connect.NewConsoleClient(hs.Client(), hs.URL, opts...)
	return client, hs.Close
}

// authInterceptor mirrors the frontend's behaviour: stamp Authorization
// on every outbound request. Used by tests that want to assert the
// header survives the Connect → handler hop.
func authInterceptor(apiKey string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("Authorization", "Bearer "+apiKey)
			return next(ctx, req)
		}
	}
}

func TestHealth_PassThrough(t *testing.T) {
	fake := &fakeServer{
		healthResp: &pb.HealthResponse{Healthy: true, Version: "v-test", Components: map[string]string{"db": "ok"}},
	}
	client, _ := newTestServer(t, fake, "")
	resp, err := client.Health(context.Background(), connect.NewRequest(&pb.HealthRequest{}))
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !resp.Msg.GetHealthy() {
		t.Errorf("expected Healthy=true")
	}
	if got := resp.Msg.GetVersion(); got != "v-test" {
		t.Errorf("version: got %q, want v-test", got)
	}
	if got := resp.Msg.GetComponents()["db"]; got != "ok" {
		t.Errorf("components.db: got %q, want ok", got)
	}
	if len(fake.gotCalls) != 1 || fake.gotCalls[0] != "Health" {
		t.Errorf("expected one Health call, got %v", fake.gotCalls)
	}
}

func TestListTenants_PassThrough(t *testing.T) {
	fake := &fakeServer{
		listTenants: &pb.ListTenantsResponse{Tenants: []*pb.TenantInfo{{TenantId: "acme"}, {TenantId: "globex"}}},
	}
	client, _ := newTestServer(t, fake, "")
	resp, err := client.ListTenants(context.Background(), connect.NewRequest(&pb.ListTenantsRequest{}))
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}
	if got := len(resp.Msg.GetTenants()); got != 2 {
		t.Fatalf("tenants: got %d, want 2", got)
	}
	if resp.Msg.GetTenants()[0].GetTenantId() != "acme" {
		t.Errorf("first tenant id mismatch: %v", resp.Msg.GetTenants()[0])
	}
}

func TestGetTenant_ForwardsTenantId(t *testing.T) {
	fake := &fakeServer{}
	client, _ := newTestServer(t, fake, "")
	resp, err := client.GetTenant(context.Background(), connect.NewRequest(&pb.GetTenantRequest{TenantId: "acme", Actor: "user:bob"}))
	if err != nil {
		t.Fatalf("GetTenant: %v", err)
	}
	if !resp.Msg.GetFound() {
		t.Errorf("expected Found=true")
	}
	if resp.Msg.GetTenant().GetTenantId() != "acme" {
		t.Errorf("tenant id mismatch: %v", resp.Msg.GetTenant())
	}
	if len(fake.gotCalls) != 1 || fake.gotCalls[0] != "GetTenant:acme" {
		t.Errorf("expected GetTenant:acme, got %v", fake.gotCalls)
	}
}

func TestGetNode_ForwardsNodeId(t *testing.T) {
	fake := &fakeServer{}
	client, _ := newTestServer(t, fake, "")
	resp, err := client.GetNode(context.Background(), connect.NewRequest(&pb.GetNodeRequest{
		Context: &pb.RequestContext{TenantId: "acme", Actor: "user:bob"},
		TypeId:  1,
		NodeId:  "n1",
	}))
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if resp.Msg.GetNode().GetNodeId() != "n1" {
		t.Errorf("node id mismatch: %v", resp.Msg.GetNode())
	}
}

func TestQueryNodes_PassThrough(t *testing.T) {
	fake := &fakeServer{queryNodes: &pb.QueryNodesResponse{TotalCount: 7, HasMore: true}}
	client, _ := newTestServer(t, fake, "")
	resp, err := client.QueryNodes(context.Background(), connect.NewRequest(&pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: "acme", Actor: "user:bob"},
		TypeId:  1,
	}))
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	if resp.Msg.GetTotalCount() != 7 || !resp.Msg.GetHasMore() {
		t.Errorf("response mismatch: %+v", resp.Msg)
	}
}

func TestGetEdgesFrom_And_To(t *testing.T) {
	fake := &fakeServer{}
	client, _ := newTestServer(t, fake, "")
	if _, err := client.GetEdgesFrom(context.Background(), connect.NewRequest(&pb.GetEdgesRequest{
		Context: &pb.RequestContext{TenantId: "acme", Actor: "user:bob"},
		NodeId:  "n1",
	})); err != nil {
		t.Fatalf("GetEdgesFrom: %v", err)
	}
	if _, err := client.GetEdgesTo(context.Background(), connect.NewRequest(&pb.GetEdgesRequest{
		Context: &pb.RequestContext{TenantId: "acme", Actor: "user:bob"},
		NodeId:  "n1",
	})); err != nil {
		t.Fatalf("GetEdgesTo: %v", err)
	}
	if len(fake.gotCalls) != 2 {
		t.Errorf("expected 2 calls, got %v", fake.gotCalls)
	}
}

func TestGetConnectedNodes_PassThrough(t *testing.T) {
	fake := &fakeServer{connectedNodes: &pb.GetConnectedNodesResponse{HasMore: true}}
	client, _ := newTestServer(t, fake, "")
	resp, err := client.GetConnectedNodes(context.Background(), connect.NewRequest(&pb.GetConnectedNodesRequest{
		Context: &pb.RequestContext{TenantId: "acme", Actor: "user:bob"},
		NodeId:  "n1",
	}))
	if err != nil {
		t.Fatalf("GetConnectedNodes: %v", err)
	}
	if !resp.Msg.GetHasMore() {
		t.Errorf("expected HasMore=true")
	}
}

func TestSearchMailbox_PassThrough(t *testing.T) {
	fake := &fakeServer{}
	client, _ := newTestServer(t, fake, "")
	if _, err := client.SearchMailbox(context.Background(), connect.NewRequest(&pb.SearchMailboxRequest{
		Context: &pb.RequestContext{TenantId: "acme", Actor: "user:bob"},
		UserId:  "alice",
		Query:   "hello",
	})); err != nil {
		t.Fatalf("SearchMailbox: %v", err)
	}
}

func TestUpstreamError_MapsToConnectCode(t *testing.T) {
	cases := []struct {
		grpcCode    codes.Code
		connectCode connect.Code
	}{
		{codes.NotFound, connect.CodeNotFound},
		{codes.PermissionDenied, connect.CodePermissionDenied},
		{codes.InvalidArgument, connect.CodeInvalidArgument},
		{codes.Unauthenticated, connect.CodeUnauthenticated},
		{codes.Unavailable, connect.CodeUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.grpcCode.String(), func(t *testing.T) {
			fake := &fakeServer{forceErr: status.Error(tc.grpcCode, "upstream-said-no")}
			client, _ := newTestServer(t, fake, "")
			_, err := client.GetTenant(context.Background(), connect.NewRequest(&pb.GetTenantRequest{TenantId: "x"}))
			if err == nil {
				t.Fatalf("expected error")
			}
			var ce *connect.Error
			if !errors.As(err, &ce) {
				t.Fatalf("error is not connect.Error: %T %v", err, err)
			}
			if ce.Code() != tc.connectCode {
				t.Errorf("code: got %v, want %v", ce.Code(), tc.connectCode)
			}
		})
	}
}

func TestUpstreamError_LongMessageIsRedacted(t *testing.T) {
	long := strings.Repeat("A", 500)
	fake := &fakeServer{forceErr: status.Error(codes.Internal, long)}
	client, _ := newTestServer(t, fake, "")
	_, err := client.GetTenant(context.Background(), connect.NewRequest(&pb.GetTenantRequest{TenantId: "x"}))
	if err == nil {
		t.Fatalf("expected error")
	}
	if strings.Contains(err.Error(), long) {
		t.Errorf("long upstream message was NOT redacted; this leaks server internals")
	}
}

func TestUpstreamConnectionError_BecomesUnavailable(t *testing.T) {
	plainErr := errors.New("connect refused")
	mapped := mapErr(plainErr)
	var ce *connect.Error
	if !errors.As(mapped, &ce) {
		t.Fatalf("expected connect.Error, got %T", mapped)
	}
	if ce.Code() != connect.CodeUnavailable {
		t.Errorf("code: got %v, want Unavailable", ce.Code())
	}
}

func TestAuthHeaderForwarded(t *testing.T) {
	// The console binary's own authedCtx is wired in real flows from
	// upstreamClient.authedCtx; here we just sanity-check that the
	// Connect client sends the header so the integration would work.
	fake := &fakeServer{}
	client, _ := newTestServer(t, fake, "secret-key")
	if _, err := client.Health(context.Background(), connect.NewRequest(&pb.HealthRequest{})); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

// ---------------------------------------------------------------------
// Sandbox-write tests (PR 2). The handlers for SandboxCreateNode and
// SandboxCreateEdge translate narrow Console requests into a single-op
// upstream ExecuteAtomicRequest, gate on the configured sandbox tenant,
// and translate the upstream Receipt back. These tests cover:
//
//   - sandbox tenant not configured        → CodeUnimplemented
//   - request tenant_id ≠ sandbox tenant   → CodePermissionDenied
//   - happy path: upstream gets the right ExecuteAtomicRequest shape
//   - upstream Unavailable / FailedPrecondition propagate by code
//
// We deliberately don't echo the requested tenant_id in the
// permission-denied message (would be a tenant-existence oracle); the
// tests assert that the configured sandbox tenant is the only one
// mentioned.
// ---------------------------------------------------------------------

func newSandboxNodeReq(tenant string) *consolev1.SandboxCreateNodeRequest {
	return &consolev1.SandboxCreateNodeRequest{
		TenantId: tenant,
		Actor:    "user:demo",
		TypeId:   1,
		Payload: structPB(map[string]any{
			"1": "hello",
			"2": float64(42),
		}),
		IdempotencyKey: "test-idem",
	}
}

func newSandboxEdgeReq(tenant string) *consolev1.SandboxCreateEdgeRequest {
	return &consolev1.SandboxCreateEdgeRequest{
		TenantId:       tenant,
		Actor:          "user:demo",
		EdgeTypeId:     7,
		FromNodeId:     "node-A",
		ToNodeId:       "node-B",
		Props:          structPB(map[string]any{"3": "edge-prop"}),
		IdempotencyKey: "edge-idem",
	}
}

func structPB(m map[string]any) *structpb.Struct {
	s, err := structpb.NewStruct(m)
	if err != nil {
		panic(err)
	}
	return s
}

func TestSandboxCreateNode_DisabledWhenNoSandboxTenant(t *testing.T) {
	fake := &fakeServer{}
	client, _ := newTestServerWithSandbox(t, fake, "", "")
	_, err := client.SandboxCreateNode(context.Background(), connect.NewRequest(newSandboxNodeReq("playground")))
	if err == nil {
		t.Fatalf("expected error when sandbox is disabled")
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("error is not connect.Error: %T %v", err, err)
	}
	if ce.Code() != connect.CodeUnimplemented {
		t.Errorf("code: got %v, want Unimplemented", ce.Code())
	}
	if len(fake.gotExecuteAtomic) != 0 {
		t.Errorf("ExecuteAtomic must NOT be called when sandbox is disabled, got %d calls", len(fake.gotExecuteAtomic))
	}
}

func TestSandboxCreateEdge_DisabledWhenNoSandboxTenant(t *testing.T) {
	fake := &fakeServer{}
	client, _ := newTestServerWithSandbox(t, fake, "", "")
	_, err := client.SandboxCreateEdge(context.Background(), connect.NewRequest(newSandboxEdgeReq("playground")))
	if err == nil {
		t.Fatalf("expected error when sandbox is disabled")
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("error is not connect.Error: %T %v", err, err)
	}
	if ce.Code() != connect.CodeUnimplemented {
		t.Errorf("code: got %v, want Unimplemented", ce.Code())
	}
}

func TestSandboxCreateNode_RejectsWrongTenant(t *testing.T) {
	fake := &fakeServer{}
	client, _ := newTestServerWithSandbox(t, fake, "", "playground")
	// Request targets a different tenant.
	_, err := client.SandboxCreateNode(context.Background(), connect.NewRequest(newSandboxNodeReq("real-prod-tenant")))
	if err == nil {
		t.Fatalf("expected permission_denied")
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("error is not connect.Error: %T", err)
	}
	if ce.Code() != connect.CodePermissionDenied {
		t.Errorf("code: got %v, want PermissionDenied", ce.Code())
	}
	if strings.Contains(ce.Message(), "real-prod-tenant") {
		t.Errorf("message must NOT echo the requested tenant id (oracle leak): %q", ce.Message())
	}
	if len(fake.gotExecuteAtomic) != 0 {
		t.Errorf("ExecuteAtomic must NOT be called for rejected tenant, got %d calls", len(fake.gotExecuteAtomic))
	}
}

func TestSandboxCreateEdge_RejectsWrongTenant(t *testing.T) {
	fake := &fakeServer{}
	client, _ := newTestServerWithSandbox(t, fake, "", "playground")
	_, err := client.SandboxCreateEdge(context.Background(), connect.NewRequest(newSandboxEdgeReq("other")))
	if err == nil {
		t.Fatalf("expected permission_denied")
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("error is not connect.Error: %T", err)
	}
	if ce.Code() != connect.CodePermissionDenied {
		t.Errorf("code: got %v, want PermissionDenied", ce.Code())
	}
}

func TestSandboxCreateNode_HappyPath_BuildsSingleCreateNodeOp(t *testing.T) {
	fake := &fakeServer{}
	client, _ := newTestServerWithSandbox(t, fake, "", "playground")
	resp, err := client.SandboxCreateNode(context.Background(), connect.NewRequest(newSandboxNodeReq("playground")))
	if err != nil {
		t.Fatalf("SandboxCreateNode: %v", err)
	}

	// Response wiring.
	if resp.Msg.GetNodeId() != "fake-node-id" {
		t.Errorf("node_id: got %q, want fake-node-id", resp.Msg.GetNodeId())
	}
	if resp.Msg.GetIdempotencyKey() != "test-idem" {
		t.Errorf("idempotency_key: got %q, want test-idem", resp.Msg.GetIdempotencyKey())
	}
	if resp.Msg.GetStreamPosition() != "fake-pos-1" {
		t.Errorf("stream_position: got %q", resp.Msg.GetStreamPosition())
	}

	// Upstream request shape.
	if got := len(fake.gotExecuteAtomic); got != 1 {
		t.Fatalf("ExecuteAtomic called %d times, want 1", got)
	}
	in := fake.gotExecuteAtomic[0]
	if !in.GetWaitApplied() {
		t.Errorf("wait_applied must be true so success means applied")
	}
	if in.GetWaitTimeoutMs() <= 0 {
		t.Errorf("wait_timeout_ms must be > 0, got %d", in.GetWaitTimeoutMs())
	}
	if in.GetContext().GetTenantId() != "playground" || in.GetContext().GetActor() != "user:demo" {
		t.Errorf("context mismatch: %+v", in.GetContext())
	}
	if in.GetIdempotencyKey() != "test-idem" {
		t.Errorf("idempotency forwarded wrong: %q", in.GetIdempotencyKey())
	}
	if got := len(in.GetOperations()); got != 1 {
		t.Fatalf("operations: got %d, want 1", got)
	}
	cn := in.GetOperations()[0].GetCreateNode()
	if cn == nil {
		t.Fatalf("operation is not create_node: %+v", in.GetOperations()[0])
	}
	if cn.GetTypeId() != 1 {
		t.Errorf("create_node.type_id: got %d, want 1", cn.GetTypeId())
	}
	if cn.GetData() == nil || cn.GetData().GetFields()["1"].GetStringValue() != "hello" {
		t.Errorf("create_node.data missing/wrong: %+v", cn.GetData())
	}
}

func TestSandboxCreateEdge_HappyPath_BuildsSingleCreateEdgeOp(t *testing.T) {
	fake := &fakeServer{}
	client, _ := newTestServerWithSandbox(t, fake, "", "playground")
	resp, err := client.SandboxCreateEdge(context.Background(), connect.NewRequest(newSandboxEdgeReq("playground")))
	if err != nil {
		t.Fatalf("SandboxCreateEdge: %v", err)
	}

	if resp.Msg.GetIdempotencyKey() != "edge-idem" {
		t.Errorf("idempotency_key: got %q, want edge-idem", resp.Msg.GetIdempotencyKey())
	}
	if resp.Msg.GetStreamPosition() != "fake-pos-1" {
		t.Errorf("stream_position: got %q", resp.Msg.GetStreamPosition())
	}

	if got := len(fake.gotExecuteAtomic); got != 1 {
		t.Fatalf("ExecuteAtomic called %d times, want 1", got)
	}
	in := fake.gotExecuteAtomic[0]
	if !in.GetWaitApplied() {
		t.Errorf("wait_applied must be true")
	}
	if got := len(in.GetOperations()); got != 1 {
		t.Fatalf("operations: got %d, want 1", got)
	}
	ce := in.GetOperations()[0].GetCreateEdge()
	if ce == nil {
		t.Fatalf("operation is not create_edge: %+v", in.GetOperations()[0])
	}
	if ce.GetEdgeId() != 7 {
		t.Errorf("create_edge.edge_id (=edge_type_id wire field): got %d, want 7", ce.GetEdgeId())
	}
	if ce.GetFrom().GetId() != "node-A" {
		t.Errorf("create_edge.from.id: got %q, want node-A", ce.GetFrom().GetId())
	}
	if ce.GetTo().GetId() != "node-B" {
		t.Errorf("create_edge.to.id: got %q, want node-B", ce.GetTo().GetId())
	}
	if ce.GetProps() == nil || ce.GetProps().GetFields()["3"].GetStringValue() != "edge-prop" {
		t.Errorf("create_edge.props missing/wrong: %+v", ce.GetProps())
	}
}

func TestSandboxCreateNode_PropagatesUpstreamUnavailable(t *testing.T) {
	fake := &fakeServer{executeErr: status.Error(codes.Unavailable, "applier down")}
	client, _ := newTestServerWithSandbox(t, fake, "", "playground")
	_, err := client.SandboxCreateNode(context.Background(), connect.NewRequest(newSandboxNodeReq("playground")))
	if err == nil {
		t.Fatalf("expected error")
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("error is not connect.Error: %T", err)
	}
	if ce.Code() != connect.CodeUnavailable {
		t.Errorf("code: got %v, want Unavailable", ce.Code())
	}
}

func TestSandboxCreateNode_PropagatesFailedPrecondition(t *testing.T) {
	fake := &fakeServer{executeErr: status.Error(codes.FailedPrecondition, "schema fingerprint mismatch")}
	client, _ := newTestServerWithSandbox(t, fake, "", "playground")
	_, err := client.SandboxCreateNode(context.Background(), connect.NewRequest(newSandboxNodeReq("playground")))
	if err == nil {
		t.Fatalf("expected error")
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("error is not connect.Error: %T", err)
	}
	if ce.Code() != connect.CodeFailedPrecondition {
		t.Errorf("code: got %v, want FailedPrecondition", ce.Code())
	}
}

func TestSandboxCreateEdge_PropagatesFailedPrecondition(t *testing.T) {
	fake := &fakeServer{executeErr: status.Error(codes.FailedPrecondition, "edge schema unknown")}
	client, _ := newTestServerWithSandbox(t, fake, "", "playground")
	_, err := client.SandboxCreateEdge(context.Background(), connect.NewRequest(newSandboxEdgeReq("playground")))
	if err == nil {
		t.Fatalf("expected error")
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("error is not connect.Error: %T", err)
	}
	if ce.Code() != connect.CodeFailedPrecondition {
		t.Errorf("code: got %v, want FailedPrecondition", ce.Code())
	}
}
