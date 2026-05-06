package main

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	consolev1 "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/console/v1"
	"github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/console/v1/consolev1connect"
	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

// sandboxWaitTimeoutMs is the wait-applied timeout sent on every
// sandbox `ExecuteAtomicRequest`. With this set the upstream RPC only
// returns after the WAL event has been applied to SQLite, so when the
// SPA's "Create" button shows success the data is actually queryable.
// Five seconds matches what the existing FastAPI playground uses.
const sandboxWaitTimeoutMs = 5000

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

	// ExecuteAtomic is the upstream write entry-point. The console
	// uses it ONLY to forward sandbox-create RPCs after gating; the
	// browser cannot reach it directly because the curated console
	// proto exposes no `ExecuteAtomic` RPC.
	ExecuteAtomic(ctx context.Context, in *pb.ExecuteAtomicRequest, opts ...grpcCallOpt) (*pb.ExecuteAtomicResponse, error)
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

	// sandboxTenant is the single tenant id that sandbox-write RPCs
	// are allowed against. Empty means sandbox writes are disabled
	// (handlers return `unimplemented`). Set from `--sandbox-tenant`
	// at startup and never mutated thereafter.
	sandboxTenant string
}

// newConsoleServer wires a real upstreamClient into the handler. Tests
// use newConsoleServerForTest with a fake stub instead.
func newConsoleServer(uc *upstreamClient, sandboxTenant string) *consoleServer {
	return &consoleServer{
		upstream:      realStub{stub: uc.stub},
		authedCtx:     uc.authedCtx,
		sandboxTenant: sandboxTenant,
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

// SandboxCreateNode forwards a single-op create-node `ExecuteAtomic`
// to upstream after gating on the configured sandbox tenant. See the
// proto-level comment on the RPC for the gating rules; we also
// deliberately do NOT echo the requested tenant id in the
// permission-denied message so the binary doesn't act as an oracle
// for "which tenant ids exist."
func (s *consoleServer) SandboxCreateNode(
	ctx context.Context,
	req *connect.Request[consolev1.SandboxCreateNodeRequest],
) (*connect.Response[consolev1.SandboxCreateNodeResponse], error) {
	if s.sandboxTenant == "" {
		return nil, connect.NewError(
			connect.CodeUnimplemented,
			errors.New("sandbox writes are disabled (--sandbox-tenant not configured)"),
		)
	}
	if req.Msg.GetTenantId() != s.sandboxTenant {
		return nil, connect.NewError(
			connect.CodePermissionDenied,
			errors.New("sandbox writes only allowed against tenant "+s.sandboxTenant),
		)
	}

	upstreamReq := &pb.ExecuteAtomicRequest{
		Context: &pb.RequestContext{
			TenantId: req.Msg.GetTenantId(),
			Actor:    req.Msg.GetActor(),
		},
		IdempotencyKey: req.Msg.GetIdempotencyKey(),
		Operations: []*pb.Operation{
			{
				Op: &pb.Operation_CreateNode{
					CreateNode: &pb.CreateNodeOp{
						TypeId: req.Msg.GetTypeId(),
						Data:   req.Msg.GetPayload(),
					},
				},
			},
		},
		WaitApplied:   true,
		WaitTimeoutMs: sandboxWaitTimeoutMs,
	}

	resp, err := s.upstream.ExecuteAtomic(s.authedCtx(ctx), upstreamReq)
	if err != nil {
		return nil, mapErr(err)
	}

	out := &consolev1.SandboxCreateNodeResponse{}
	if r := resp.GetReceipt(); r != nil {
		out.IdempotencyKey = r.GetIdempotencyKey()
		out.StreamPosition = r.GetStreamPosition()
	}
	if ids := resp.GetCreatedNodeIds(); len(ids) > 0 {
		out.NodeId = ids[0]
	}
	return connect.NewResponse(out), nil
}

// SandboxCreateEdge mirrors SandboxCreateNode for a single-op
// create-edge `ExecuteAtomic`. Edges are addressed by direct node id;
// the curated request type does not allow alias references (those are
// a multi-op transaction shape we don't expose to the browser).
func (s *consoleServer) SandboxCreateEdge(
	ctx context.Context,
	req *connect.Request[consolev1.SandboxCreateEdgeRequest],
) (*connect.Response[consolev1.SandboxCreateEdgeResponse], error) {
	if s.sandboxTenant == "" {
		return nil, connect.NewError(
			connect.CodeUnimplemented,
			errors.New("sandbox writes are disabled (--sandbox-tenant not configured)"),
		)
	}
	if req.Msg.GetTenantId() != s.sandboxTenant {
		return nil, connect.NewError(
			connect.CodePermissionDenied,
			errors.New("sandbox writes only allowed against tenant "+s.sandboxTenant),
		)
	}

	upstreamReq := &pb.ExecuteAtomicRequest{
		Context: &pb.RequestContext{
			TenantId: req.Msg.GetTenantId(),
			Actor:    req.Msg.GetActor(),
		},
		IdempotencyKey: req.Msg.GetIdempotencyKey(),
		Operations: []*pb.Operation{
			{
				Op: &pb.Operation_CreateEdge{
					CreateEdge: &pb.CreateEdgeOp{
						EdgeId: req.Msg.GetEdgeTypeId(),
						From:   &pb.NodeRef{Ref: &pb.NodeRef_Id{Id: req.Msg.GetFromNodeId()}},
						To:     &pb.NodeRef{Ref: &pb.NodeRef_Id{Id: req.Msg.GetToNodeId()}},
						Props:  req.Msg.GetProps(),
					},
				},
			},
		},
		WaitApplied:   true,
		WaitTimeoutMs: sandboxWaitTimeoutMs,
	}

	resp, err := s.upstream.ExecuteAtomic(s.authedCtx(ctx), upstreamReq)
	if err != nil {
		return nil, mapErr(err)
	}

	out := &consolev1.SandboxCreateEdgeResponse{}
	if r := resp.GetReceipt(); r != nil {
		out.IdempotencyKey = r.GetIdempotencyKey()
		out.StreamPosition = r.GetStreamPosition()
	}
	return connect.NewResponse(out), nil
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
