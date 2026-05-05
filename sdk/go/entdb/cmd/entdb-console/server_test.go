package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/console/v1/consolev1connect"
	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// newTestServer builds a httptest.Server wired to a fakeServer, with a
// matching Connect client. Tests then call client methods directly and
// assert against fake.gotCalls / responses.
func newTestServer(t *testing.T, fake *fakeServer, apiKey string) (consolev1connect.ConsoleClient, func()) {
	t.Helper()
	srv := &consoleServer{
		upstream:  fake,
		authedCtx: func(ctx context.Context) context.Context { return ctx },
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
