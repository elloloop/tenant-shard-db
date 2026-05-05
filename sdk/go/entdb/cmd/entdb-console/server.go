package main

import (
	"context"

	"connectrpc.com/connect"
	consolev1 "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/console/v1"
	"github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/console/v1/consolev1connect"
	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

// upstreamStub is the subset of pb.EntDBServiceClient the console
// actually calls. Defined as a private interface so handler tests can
// supply a fake upstream without standing up a real gRPC server.
//
// IMPORTANT: this interface MUST stay a strict subset of the upstream
// EntDBServiceClient that maps 1:1 to the curated console.proto. The
// proto is the access policy; this interface is the in-process mirror.
// Adding a method here without a corresponding `rpc` in console.proto
// is a security regression — review accordingly.
type upstreamStub interface {
	Health(ctx context.Context, in *pb.HealthRequest, opts ...grpcCallOpt) (*pb.HealthResponse, error)
	ListTenants(ctx context.Context, in *pb.ListTenantsRequest, opts ...grpcCallOpt) (*pb.ListTenantsResponse, error)
	GetTenant(ctx context.Context, in *pb.GetTenantRequest, opts ...grpcCallOpt) (*pb.GetTenantResponse, error)
	GetSchema(ctx context.Context, in *pb.GetSchemaRequest, opts ...grpcCallOpt) (*pb.GetSchemaResponse, error)
	QueryNodes(ctx context.Context, in *pb.QueryNodesRequest, opts ...grpcCallOpt) (*pb.QueryNodesResponse, error)
	GetNode(ctx context.Context, in *pb.GetNodeRequest, opts ...grpcCallOpt) (*pb.GetNodeResponse, error)
	GetEdgesFrom(ctx context.Context, in *pb.GetEdgesRequest, opts ...grpcCallOpt) (*pb.GetEdgesResponse, error)
	GetEdgesTo(ctx context.Context, in *pb.GetEdgesRequest, opts ...grpcCallOpt) (*pb.GetEdgesResponse, error)
	GetConnectedNodes(ctx context.Context, in *pb.GetConnectedNodesRequest, opts ...grpcCallOpt) (*pb.GetConnectedNodesResponse, error)
	SearchMailbox(ctx context.Context, in *pb.SearchMailboxRequest, opts ...grpcCallOpt) (*pb.SearchMailboxResponse, error)
}

// consoleServer implements the ConnectRPC Console handler. Every
// method is a straight pass-through: build the upstream context (with
// auth metadata), call the upstream, map errors. No business logic
// lives here — that all belongs to entdb-server. The point of this
// binary is only to be a curated, browser-safe surface.
type consoleServer struct {
	consolev1connect.UnimplementedConsoleHandler

	upstream     upstreamStub
	authedCtx    func(ctx context.Context) context.Context
	binaryHealth func() *pb.HealthResponse // optional override for /Health
}

// newConsoleServer wires a real upstreamClient into the handler. Tests
// use newConsoleServerForTest with a fake stub instead.
func newConsoleServer(uc *upstreamClient) *consoleServer {
	return &consoleServer{
		upstream:  realStub{stub: uc.stub},
		authedCtx: uc.authedCtx,
	}
}

// Health forwards to the upstream so the response reflects upstream
// availability (the SPA uses this as its readiness probe). If the
// caller injected a binaryHealth override (tests) we return that
// instead so the test isn't coupled to the upstream stub.
func (s *consoleServer) Health(
	ctx context.Context,
	req *connect.Request[pb.HealthRequest],
) (*connect.Response[pb.HealthResponse], error) {
	if s.binaryHealth != nil {
		return connect.NewResponse(s.binaryHealth()), nil
	}
	resp, err := s.upstream.Health(s.authedCtx(ctx), req.Msg)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (s *consoleServer) ListTenants(
	ctx context.Context,
	req *connect.Request[pb.ListTenantsRequest],
) (*connect.Response[pb.ListTenantsResponse], error) {
	resp, err := s.upstream.ListTenants(s.authedCtx(ctx), req.Msg)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (s *consoleServer) GetTenant(
	ctx context.Context,
	req *connect.Request[pb.GetTenantRequest],
) (*connect.Response[pb.GetTenantResponse], error) {
	resp, err := s.upstream.GetTenant(s.authedCtx(ctx), req.Msg)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (s *consoleServer) GetSchema(
	ctx context.Context,
	req *connect.Request[pb.GetSchemaRequest],
) (*connect.Response[pb.GetSchemaResponse], error) {
	resp, err := s.upstream.GetSchema(s.authedCtx(ctx), req.Msg)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (s *consoleServer) QueryNodes(
	ctx context.Context,
	req *connect.Request[pb.QueryNodesRequest],
) (*connect.Response[pb.QueryNodesResponse], error) {
	resp, err := s.upstream.QueryNodes(s.authedCtx(ctx), req.Msg)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (s *consoleServer) GetNode(
	ctx context.Context,
	req *connect.Request[pb.GetNodeRequest],
) (*connect.Response[pb.GetNodeResponse], error) {
	resp, err := s.upstream.GetNode(s.authedCtx(ctx), req.Msg)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (s *consoleServer) GetEdgesFrom(
	ctx context.Context,
	req *connect.Request[pb.GetEdgesRequest],
) (*connect.Response[pb.GetEdgesResponse], error) {
	resp, err := s.upstream.GetEdgesFrom(s.authedCtx(ctx), req.Msg)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (s *consoleServer) GetEdgesTo(
	ctx context.Context,
	req *connect.Request[pb.GetEdgesRequest],
) (*connect.Response[pb.GetEdgesResponse], error) {
	resp, err := s.upstream.GetEdgesTo(s.authedCtx(ctx), req.Msg)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (s *consoleServer) GetConnectedNodes(
	ctx context.Context,
	req *connect.Request[pb.GetConnectedNodesRequest],
) (*connect.Response[pb.GetConnectedNodesResponse], error) {
	resp, err := s.upstream.GetConnectedNodes(s.authedCtx(ctx), req.Msg)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (s *consoleServer) SearchMailbox(
	ctx context.Context,
	req *connect.Request[pb.SearchMailboxRequest],
) (*connect.Response[pb.SearchMailboxResponse], error) {
	resp, err := s.upstream.SearchMailbox(s.authedCtx(ctx), req.Msg)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(resp), nil
}

// Compile-time assertion: consoleServer must implement the generated
// ConsoleHandler interface. This is the *literal* enforcement that
// console.proto is the surface — if a new RPC is added to the proto,
// the build fails until consoleServer grows the matching method.
var _ consolev1connect.ConsoleHandler = (*consoleServer)(nil)

// _ = consolev1 keeps the import live so generated `MethodConsoleX`
// constants remain reachable for tests/diagnostics that may want to
// reference them by name.
var _ = consolev1.File_console_proto
