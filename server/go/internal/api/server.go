// Package api implements the EntDB gRPC service. Embedding
// UnimplementedEntDBServiceServer keeps the implementation
// forward-compatible with future RPCs added to the proto without
// breaking the build.
package api

import (
	"context"

	"github.com/elloloop/tenant-shard-db/server/go/internal/acl"
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
// Dependencies are injected through functional options (Option)
// rather than globals so cmd/entdb-server/main.go can hand off
// store + wal + globalstore handles without the api package
// growing import cycles. Handlers reach for these via the
// unexported fields on Server.
type Server struct {
	pb.UnimplementedEntDBServiceServer

	store    *store.CanonicalStore
	global   *globalstore.GlobalStore
	producer wal.Producer
	topic    string
	sharding *tenant.Sharding
	region   string
	registry *schema.Registry

	// legalHoldOnDelete gates the legal-hold precondition check at GDPR
	// queue time (DeleteUser). Off by default; see
	// docs/go-port/rpcs/DeleteUser.md "Side effects" / "Open
	// questions" §4.
	legalHoldOnDelete bool

	// aclEnforcer is the optional ACL Enforcer wired by main.go (or
	// by tests via WithEnforcer). When nil, handlers that need ACL
	// pre-checks fall back to the lightweight per-call construction
	// path (see Server.aclChecker). The shared Enforcer holds the
	// Registry + Resolver + Checker + Filter so per-server wiring is
	// in one place; handlers prefer it when present.
	aclEnforcer *acl.Enforcer

	// applierReadiness, when wired, lets the Health RPC report the WAL
	// applier's liveness (issue #653). It returns the applier state
	// string ("applying"/"idle"/"stalled"/"exited") and whether that
	// state is healthy. main.go builds the closure from the applier's
	// progress signal + the (opt-in) stall threshold; the api package
	// stays decoupled from the apply package. nil => component "unknown",
	// never gating healthy.
	applierReadiness func() (state string, healthy bool)
}

// Option is a functional-options configurator for New.
type Option func(*Server)

// WithStore wires a *store.CanonicalStore (per-tenant SQLite). Read
// RPCs reach for it through s.store.
func WithStore(s *store.CanonicalStore) Option {
	return func(srv *Server) { srv.store = s }
}

// WithGlobalStore wires the cross-tenant globalstore handle.
func WithGlobalStore(g *globalstore.GlobalStore) Option {
	return func(srv *Server) { srv.global = g }
}

// WithWALProducer wires the WAL producer (for ExecuteAtomic).
func WithWALProducer(p wal.Producer) Option {
	return func(srv *Server) { srv.producer = p }
}

// WithWALTopic configures the WAL topic name handlers append to. When
// unset (or empty), write RPCs default to "entdb-wal" — the
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
// RPCs (GetSchema, ExecuteAtomic schema_fingerprint check,
// query handlers translating field names) all dereference this handle.
// The registry is typically Freeze()'d before the server starts serving
// (CLAUDE.md schema invariant), so post-boot reads are lock-free.
func WithSchemaRegistry(r *schema.Registry) Option {
	return func(srv *Server) { srv.registry = r }
}

// WithEnforcer wires a process-wide *acl.Enforcer. Handlers that
// need ACL pre-checks (ShareNode, RevokeAccess, TransferOwnership)
// reach for it via s.aclEnforcer. When unset, handlers construct an
// ad-hoc Checker on the fly using s.store + s.global as reader sources
// — preserves test fixtures that wire only the stores.
func WithEnforcer(e *acl.Enforcer) Option {
	return func(srv *Server) { srv.aclEnforcer = e }
}

// WithLegalHoldOnDelete enables the legal-hold gate at DeleteUser queue
// time. When true, the handler walks the user's tenants and rejects
// with codes.FailedPrecondition if any tenant has a legal_holds row.
// Off by default. See docs/go-port/rpcs/DeleteUser.md.
func WithLegalHoldOnDelete(enabled bool) Option {
	return func(srv *Server) { srv.legalHoldOnDelete = enabled }
}

// WithApplierReadiness wires a readiness probe for the WAL applier
// (issue #653). The closure returns the applier's state string and
// whether it is healthy; the Health RPC surfaces the state in
// components["applier"] and, when unhealthy, flips healthy=false so a
// stalled (or exited) applier becomes an alertable / restart-triggering
// condition instead of a silent stall. Off by default (component
// "unknown", never gating) so the api package needs no apply dependency.
func WithApplierReadiness(fn func() (state string, healthy bool)) Option {
	return func(srv *Server) { srv.applierReadiness = fn }
}

// New constructs a Server. Dependencies wired via opts are stored
// for use by RPC handlers.
func New(opts ...Option) *Server {
	s := &Server{}
	for _, o := range opts {
		o(s)
	}
	return s
}

// checkTenant is the per-handler tenant gate wrapper. It exists so
// RPCs can write a single line at the top of their handler instead
// of repeating the dependency-passing dance, and so the gate has
// exactly one call shape across the server.
func (s *Server) checkTenant(ctx context.Context, tenantID string) error {
	return tenant.CheckTenant(ctx, tenantID, s.global, s.sharding, tenant.Options{ServedRegion: s.region})
}

// walTopic returns the configured WAL topic, falling back to the
// process-wide default ("entdb-wal", same as cmd/entdb-server/main.go's
// --wal-topic flag default). Centralised here so every writer reaches
// for the same string.
func (s *Server) walTopic() string {
	if s.topic == "" {
		return "entdb-wal"
	}
	return s.topic
}
