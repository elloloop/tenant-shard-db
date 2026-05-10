// Package api implements the EntDB gRPC service. Phase 0 of the
// Python → Go server port (see GitHub issue #407): every method
// returns codes.Unimplemented so the binary can boot, register
// against a grpc.Server, and pass `go vet` / `go test`. Methods
// land here one at a time as RPC sub-issues are picked up.
package api

import (
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// Server is the EntDBService implementation. Embedding
// UnimplementedEntDBServiceServer makes every RPC return
// codes.Unimplemented by default and keeps the type
// forward-compatible with new methods added to the proto.
type Server struct {
	pb.UnimplementedEntDBServiceServer
}

// New constructs a Server. The constructor is currently empty;
// dependencies (WAL handle, applier, auth interceptor wiring)
// will be added as the corresponding Python subsystems are
// ported.
func New() *Server {
	return &Server{}
}
