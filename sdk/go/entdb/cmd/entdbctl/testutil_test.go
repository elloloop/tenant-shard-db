package main

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

// fakeCLIServer is a tiny pb.UnimplementedEntDBServiceServer
// that records the last request seen for each RPC the CLI
// touches and hands back canned responses. It mirrors the
// grpc_transport_test.go pattern but stays scoped to the
// commands shipped by this CLI.
type fakeCLIServer struct {
	pb.UnimplementedEntDBServiceServer

	healthReq    *pb.HealthRequest
	healthResp   *pb.HealthResponse
	listTReq     *pb.ListTenantsRequest
	listTResp    *pb.ListTenantsResponse
	getTReq      *pb.GetTenantRequest
	getTResp     *pb.GetTenantResponse
	schemaReq    *pb.GetSchemaRequest
	schemaResp   *pb.GetSchemaResponse
	queryReq     *pb.QueryNodesRequest
	queryResp    *pb.QueryNodesResponse
	getNodeReq   *pb.GetNodeRequest
	getNodeResp  *pb.GetNodeResponse
	edgesFromReq *pb.GetEdgesRequest
	edgesFromRsp *pb.GetEdgesResponse
	edgesToReq   *pb.GetEdgesRequest
	edgesToRsp   *pb.GetEdgesResponse
	connReq      *pb.GetConnectedNodesRequest
	connResp     *pb.GetConnectedNodesResponse
	searchReq    *pb.SearchMailboxRequest
	searchResp   *pb.SearchMailboxResponse
}

func (s *fakeCLIServer) Health(_ context.Context, r *pb.HealthRequest) (*pb.HealthResponse, error) {
	s.healthReq = r
	return s.healthResp, nil
}
func (s *fakeCLIServer) ListTenants(_ context.Context, r *pb.ListTenantsRequest) (*pb.ListTenantsResponse, error) {
	s.listTReq = r
	return s.listTResp, nil
}
func (s *fakeCLIServer) GetTenant(_ context.Context, r *pb.GetTenantRequest) (*pb.GetTenantResponse, error) {
	s.getTReq = r
	return s.getTResp, nil
}
func (s *fakeCLIServer) GetSchema(_ context.Context, r *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
	s.schemaReq = r
	return s.schemaResp, nil
}
func (s *fakeCLIServer) QueryNodes(_ context.Context, r *pb.QueryNodesRequest) (*pb.QueryNodesResponse, error) {
	s.queryReq = r
	return s.queryResp, nil
}
func (s *fakeCLIServer) GetNode(_ context.Context, r *pb.GetNodeRequest) (*pb.GetNodeResponse, error) {
	s.getNodeReq = r
	return s.getNodeResp, nil
}
func (s *fakeCLIServer) GetEdgesFrom(_ context.Context, r *pb.GetEdgesRequest) (*pb.GetEdgesResponse, error) {
	s.edgesFromReq = r
	return s.edgesFromRsp, nil
}
func (s *fakeCLIServer) GetEdgesTo(_ context.Context, r *pb.GetEdgesRequest) (*pb.GetEdgesResponse, error) {
	s.edgesToReq = r
	return s.edgesToRsp, nil
}
func (s *fakeCLIServer) GetConnectedNodes(_ context.Context, r *pb.GetConnectedNodesRequest) (*pb.GetConnectedNodesResponse, error) {
	s.connReq = r
	return s.connResp, nil
}
func (s *fakeCLIServer) SearchMailbox(_ context.Context, r *pb.SearchMailboxRequest) (*pb.SearchMailboxResponse, error) {
	s.searchReq = r
	return s.searchResp, nil
}

// startFake spins up an in-process gRPC server over bufconn,
// rewires dial() to it, and returns the fake. The cleanup is
// registered on t — individual tests stay terse.
func startFake(t *testing.T, svc *fakeCLIServer) *fakeCLIServer {
	t.Helper()
	lis := bufconn.Listen(1 << 16)
	srv := grpc.NewServer()
	pb.RegisterEntDBServiceServer(srv, svc)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	// Override the package-level dialer so newClient() picks
	// up the bufconn listener instead of trying to reach the
	// real address. Restored on test cleanup.
	prev := dialOverride
	dialOverride = func(_ string) (*grpc.ClientConn, error) {
		dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
		return grpc.NewClient(
			"passthrough:///bufnet",
			grpc.WithContextDialer(dialer),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
	}
	t.Cleanup(func() { dialOverride = prev })

	return svc
}

// captureOutput rewires stdout/stderr to byte buffers and
// returns both plus a restore func. Tests use it to assert
// on the human-readable table output.
func captureOutput() (*bytes.Buffer, *bytes.Buffer, func()) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	restore := setOutputs(out, errBuf)
	return out, errBuf, restore
}

// resetFlags returns the global flags to a known baseline
// before each test. Cobra's persistent-flag plumbing only
// runs in Execute(), so unit tests bypass it and write
// directly to the struct.
func resetFlags() {
	flags.addr = "bufnet"
	flags.apiKey = ""
	flags.actor = "user:alice"
	flags.format = "table"
	flags.timeout = 5 * time.Second
}
