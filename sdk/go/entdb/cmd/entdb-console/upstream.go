package main

import (
	"context"
	"fmt"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// upstreamClient bundles the gRPC connection + EntDBService stub plus
// the API key the console attaches to every outgoing request. It's a
// thin wrapper — there's no caching, retry, or pooling here. The Go
// gRPC stub already handles connection multiplexing; we just need a
// place to stamp Authorization metadata before each call.
type upstreamClient struct {
	conn   *grpc.ClientConn
	stub   pb.EntDBServiceClient
	apiKey string
}

// dialUpstream opens a gRPC connection to entdb-server. `addr` is in
// `host:port` form. Insecure transport is used because the upstream
// connection is in-cluster (sidecar/loopback in production, dev compose
// network locally); TLS termination for browser-facing traffic is the
// console's HTTP layer, not this gRPC hop.
func dialUpstream(ctx context.Context, addr, apiKey string) (*upstreamClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial upstream %q: %w", addr, err)
	}
	return &upstreamClient{
		conn:   conn,
		stub:   pb.NewEntDBServiceClient(conn),
		apiKey: apiKey,
	}, nil
}

// authedCtx returns ctx with the console's API key attached as a
// `authorization: Bearer …` gRPC metadata header. If no key is
// configured the original ctx is returned unchanged so local dev
// against an auth-disabled server still works.
func (u *upstreamClient) authedCtx(ctx context.Context) context.Context {
	if u.apiKey == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+u.apiKey)
}

// Close releases the gRPC connection.
func (u *upstreamClient) Close() error {
	if u.conn == nil {
		return nil
	}
	return u.conn.Close()
}
