// Package api implements the EntDB gRPC service. Phase 0 of the
// Python → Go server port (see GitHub issue #407): every method
// returns codes.Unimplemented so the binary can boot, register
// against a grpc.Server, and pass `go vet` / `go test`. Methods
// land here one at a time as RPC sub-issues are picked up.
package api

import (
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// Server is the EntDBService implementation. Embedding
// UnimplementedEntDBServiceServer makes every RPC return
// codes.Unimplemented by default and keeps the type
// forward-compatible with new methods added to the proto.
//
// Wave-1 wiring: dependencies are injected through functional options
// (Option) rather than globals so cmd/entdb-server/main.go can hand
// off store + wal + globalstore handles without the api package
// growing import cycles. Wave-2 RPC methods will reach for these via
// the unexported fields on Server.
type Server struct {
	pb.UnimplementedEntDBServiceServer

	store    *store.CanonicalStore
	global   *globalstore.GlobalStore
	producer wal.Producer
}

// Option is a functional-options configurator for New.
type Option func(*Server)

// WithStore wires a *store.CanonicalStore (per-tenant SQLite). Wave-2
// read RPCs will need it; Wave-1 just stores the handle for forward
// compatibility.
func WithStore(s *store.CanonicalStore) Option {
	return func(srv *Server) { srv.store = s }
}

// WithGlobalStore wires the cross-tenant globalstore handle.
func WithGlobalStore(g *globalstore.GlobalStore) Option {
	return func(srv *Server) { srv.global = g }
}

// WithWALProducer wires the WAL producer (for ExecuteAtomic in Wave 2).
func WithWALProducer(p wal.Producer) Option {
	return func(srv *Server) { srv.producer = p }
}

// New constructs a Server. All RPCs return Unimplemented in Wave 1;
// dependencies wired via opts are stored for use by Wave-2 handlers as
// they land.
func New(opts ...Option) *Server {
	s := &Server{}
	for _, o := range opts {
		o(s)
	}
	return s
}
