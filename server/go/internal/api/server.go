// Package api implements the EntDB gRPC service. Phase 0 of the
// Python → Go server port (see GitHub issue #407): every method
// returns codes.Unimplemented so the binary can boot, register
// against a grpc.Server, and pass `go vet` / `go test`. Methods
// land here one at a time as RPC sub-issues are picked up.
package api

import (
	"context"

	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/tenant"
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
	topic    string
	sharding *tenant.Sharding
	region   string
	registry *schema.Registry

	// legalHoldOnDelete gates the Go-port addition of a legal-hold
	// precondition check at GDPR queue time (DeleteUser). Off by
	// default to keep day-zero parity with the Python handler, which
	// has no such gate. See docs/go-port/rpcs/DeleteUser.md "Side
	// effects" / "Open questions" §4.
	legalHoldOnDelete bool
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

// WithWALTopic configures the WAL topic name handlers append to. When
// unset (or empty), Wave-2 write RPCs default to "entdb-wal" — the
// same default carried by cmd/entdb-server/main.go's --wal-topic flag.
func WithWALTopic(topic string) Option {
	return func(srv *Server) { srv.topic = topic }
}

// WithSharding wires the per-node tenant-ownership/redirect config the
// tenant gate consults on every RPC. A nil value is the single-node
// default — every tenant is owned by this node (see tenant.Sharding).
func WithSharding(sh *tenant.Sharding) Option {
	return func(srv *Server) { srv.sharding = sh }
}

// WithRegion wires the region this node is configured to serve. Empty
// disables region pinning (tenant.Options{}.ServedRegion == "").
func WithRegion(region string) Option {
	return func(srv *Server) { srv.region = region }
}

// WithSchemaRegistry wires the process-wide *schema.Registry. Read-only
// RPCs in Wave 2 (GetSchema, ExecuteAtomic schema_fingerprint check,
// query handlers translating field names) all dereference this handle.
// The registry is typically Freeze()'d before the server starts serving
// (CLAUDE.md schema invariant), so post-boot reads are lock-free.
func WithSchemaRegistry(r *schema.Registry) Option {
	return func(srv *Server) { srv.registry = r }
}

// WithLegalHoldOnDelete enables the Go-port-only legal-hold gate at
// DeleteUser queue time. When true, the handler walks the user's
// tenants and rejects with codes.FailedPrecondition if any tenant has
// a legal_holds row. Off by default for byte-for-byte parity with the
// Python handler at HEAD. See docs/go-port/rpcs/DeleteUser.md.
func WithLegalHoldOnDelete(enabled bool) Option {
	return func(srv *Server) { srv.legalHoldOnDelete = enabled }
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

// checkTenant is the per-handler tenant gate wrapper. It exists so
// Wave-2 RPCs can write a single line at the top of their handler
// instead of repeating the dependency-passing dance, and so the gate
// has exactly one call shape across the server (mirrors the Python
// `await self._check_tenant(...)` convention at
// api/grpc_server.py:362).
func (s *Server) checkTenant(ctx context.Context, tenantID string) error {
	return tenant.CheckTenant(ctx, tenantID, s.global, s.sharding, tenant.Options{ServedRegion: s.region})
}

// walTopic returns the configured WAL topic, falling back to the
// process-wide default ("entdb-wal", same as cmd/entdb-server/main.go's
// --wal-topic flag default). Centralised here so every Wave-2 writer
// reaches for the same string.
func (s *Server) walTopic() string {
	if s.topic == "" {
		return "entdb-wal"
	}
	return s.topic
}
