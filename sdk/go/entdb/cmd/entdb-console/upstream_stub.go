package main

import (
	"context"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
	"google.golang.org/grpc"
)

// grpcCallOpt is just an alias for grpc.CallOption — defining it here
// keeps `server.go` independent of the gRPC import path so handler
// tests can supply a fake without dragging the gRPC client into the
// test file.
type grpcCallOpt = grpc.CallOption

// realStub adapts the generated pb.EntDBServiceClient to the small
// upstreamStub interface used by consoleServer. The methods are pure
// forwarding — there is no behaviour of our own here.
type realStub struct {
	stub pb.EntDBServiceClient
}

func (r realStub) Health(ctx context.Context, in *pb.HealthRequest, opts ...grpcCallOpt) (*pb.HealthResponse, error) {
	return r.stub.Health(ctx, in, opts...)
}

func (r realStub) ListTenants(ctx context.Context, in *pb.ListTenantsRequest, opts ...grpcCallOpt) (*pb.ListTenantsResponse, error) {
	return r.stub.ListTenants(ctx, in, opts...)
}

func (r realStub) GetTenant(ctx context.Context, in *pb.GetTenantRequest, opts ...grpcCallOpt) (*pb.GetTenantResponse, error) {
	return r.stub.GetTenant(ctx, in, opts...)
}

func (r realStub) GetSchema(ctx context.Context, in *pb.GetSchemaRequest, opts ...grpcCallOpt) (*pb.GetSchemaResponse, error) {
	return r.stub.GetSchema(ctx, in, opts...)
}

func (r realStub) QueryNodes(ctx context.Context, in *pb.QueryNodesRequest, opts ...grpcCallOpt) (*pb.QueryNodesResponse, error) {
	return r.stub.QueryNodes(ctx, in, opts...)
}

func (r realStub) GetNode(ctx context.Context, in *pb.GetNodeRequest, opts ...grpcCallOpt) (*pb.GetNodeResponse, error) {
	return r.stub.GetNode(ctx, in, opts...)
}

func (r realStub) GetEdgesFrom(ctx context.Context, in *pb.GetEdgesRequest, opts ...grpcCallOpt) (*pb.GetEdgesResponse, error) {
	return r.stub.GetEdgesFrom(ctx, in, opts...)
}

func (r realStub) GetEdgesTo(ctx context.Context, in *pb.GetEdgesRequest, opts ...grpcCallOpt) (*pb.GetEdgesResponse, error) {
	return r.stub.GetEdgesTo(ctx, in, opts...)
}

func (r realStub) GetConnectedNodes(ctx context.Context, in *pb.GetConnectedNodesRequest, opts ...grpcCallOpt) (*pb.GetConnectedNodesResponse, error) {
	return r.stub.GetConnectedNodes(ctx, in, opts...)
}

func (r realStub) SearchMailbox(ctx context.Context, in *pb.SearchMailboxRequest, opts ...grpcCallOpt) (*pb.SearchMailboxResponse, error) {
	return r.stub.SearchMailbox(ctx, in, opts...)
}
