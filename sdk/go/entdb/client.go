package entdb

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

// Transport is the low-level interface between the SDK and the
// gRPC layer. It is intentionally NOT part of the public single-
// shape API — user code goes through the typed [Scope] / [Plan]
// surface, which reaches the transport through internal calls.
//
// Transport methods take bare ints (“typeID“) and raw field ids
// because they sit below the proto-descriptor boundary; every
// public entry point above this layer is typed end-to-end.
type Transport interface {
	// Connect establishes the connection to the server.
	Connect(ctx context.Context) error
	// Close tears down the connection.
	Close() error
	// GetNode retrieves a single node.
	GetNode(ctx context.Context, tenantID, actor string, typeID int, nodeID string) (*Node, error)
	// GetNodeByKey resolves a node via a declared unique key.
	//
	// The signature matches the 2026-04-14 SDK v0.3 gRPC contract:
	// ``(type_id, field_id, value)``, with ``value`` carried as a
	// ``google.protobuf.Value`` so one RPC shape handles string,
	// int, float, and bool unique keys.
	GetNodeByKey(ctx context.Context, tenantID, actor string, typeID, fieldID int32, value *structpb.Value) (*Node, error)
	// QueryNodes retrieves nodes matching a filter.
	QueryNodes(ctx context.Context, tenantID, actor string, typeID int, filter map[string]any) ([]*Node, error)
	// ExecuteAtomic commits a batch of operations atomically.
	ExecuteAtomic(ctx context.Context, tenantID, actor, idempotencyKey string, ops []Operation) (*CommitResult, error)
	// Share grants permission on a node to another actor.
	Share(ctx context.Context, tenantID, actor, nodeID, grantee, permission string) error
	// GetEdgesFrom retrieves outgoing edges from a node.
	GetEdgesFrom(ctx context.Context, tenantID, actor, fromNodeID string, edgeTypeID int) ([]*Edge, error)
	// GetEdgesTo retrieves incoming edges to a node.
	GetEdgesTo(ctx context.Context, tenantID, actor, toNodeID string, edgeTypeID int) ([]*Edge, error)
	// SearchNodes performs full-text search across searchable fields.
	SearchNodes(ctx context.Context, tenantID, actor string, typeID int, query string) ([]*Node, error)
}

// grpcTransport is the production Transport backed by a gRPC connection.
type grpcTransport struct {
	address string
	config  clientConfig
	conn    *grpc.ClientConn
}

func newGRPCTransport(address string, cfg clientConfig) *grpcTransport {
	return &grpcTransport{
		address: address,
		config:  cfg,
	}
}

func (t *grpcTransport) Connect(_ context.Context) error {
	var creds credentials.TransportCredentials
	if t.config.secure {
		creds = credentials.NewTLS(nil)
	} else {
		creds = insecure.NewCredentials()
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}

	conn, err := grpc.NewClient(t.address, opts...)
	if err != nil {
		return NewConnectionError(err.Error(), t.address)
	}
	t.conn = conn
	return nil
}

func (t *grpcTransport) Close() error {
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}

func (t *grpcTransport) GetNode(_ context.Context, _, _ string, _ int, _ string) (*Node, error) {
	// Requires generated proto stubs to implement.
	return nil, &EntDBError{Message: "not implemented: requires proto stubs", Code: "NOT_IMPLEMENTED"}
}

func (t *grpcTransport) GetNodeByKey(_ context.Context, _, _ string, _, _ int32, _ *structpb.Value) (*Node, error) {
	// Requires generated proto stubs to implement.
	return nil, &EntDBError{Message: "not implemented: requires proto stubs", Code: "NOT_IMPLEMENTED"}
}

func (t *grpcTransport) QueryNodes(_ context.Context, _, _ string, _ int, _ map[string]any) ([]*Node, error) {
	// Requires generated proto stubs to implement.
	return nil, &EntDBError{Message: "not implemented: requires proto stubs", Code: "NOT_IMPLEMENTED"}
}

func (t *grpcTransport) ExecuteAtomic(_ context.Context, _, _, _ string, _ []Operation) (*CommitResult, error) {
	return nil, &EntDBError{Message: "not implemented: requires proto stubs", Code: "NOT_IMPLEMENTED"}
}

func (t *grpcTransport) Share(_ context.Context, _, _, _, _, _ string) error {
	return &EntDBError{Message: "not implemented: requires proto stubs", Code: "NOT_IMPLEMENTED"}
}

func (t *grpcTransport) GetEdgesFrom(_ context.Context, _, _, _ string, _ int) ([]*Edge, error) {
	return nil, &EntDBError{Message: "not implemented: requires proto stubs", Code: "NOT_IMPLEMENTED"}
}

func (t *grpcTransport) GetEdgesTo(_ context.Context, _, _, _ string, _ int) ([]*Edge, error) {
	return nil, &EntDBError{Message: "not implemented: requires proto stubs", Code: "NOT_IMPLEMENTED"}
}

func (t *grpcTransport) SearchNodes(_ context.Context, _, _ string, _ int, _ string) ([]*Node, error) {
	return nil, &EntDBError{Message: "not implemented: requires proto stubs", Code: "NOT_IMPLEMENTED"}
}

// DbClient is the main entry point for the EntDB Go SDK.
//
// DbClient owns the gRPC transport and hands out [TenantScope] /
// [Scope] handles for typed access. It deliberately does NOT expose
// a flat “Get(tenantID, actor, typeID, nodeID)“ API — every
// user-facing read or write goes through a typed scope so that
// type_id and field_id only appear inside the SDK, never in user
// code.
//
//	client, _ := entdb.NewClient("localhost:50051",
//	    entdb.WithSecure(), entdb.WithAPIKey("sk-..."))
//	defer client.Close()
//	_ = client.Connect(ctx)
//	scope := client.Tenant("acme").Actor(entdb.UserActor("bob"))
//	product, err := entdb.Get[*shop.Product](ctx, scope, "node-42")
type DbClient struct {
	address   string
	config    clientConfig
	transport Transport
}

// NewClient creates a new DbClient targeting the given server address.
//
// The client is not connected until Connect is called.
func NewClient(address string, opts ...ClientOption) (*DbClient, error) {
	if address == "" {
		return nil, NewConnectionError("address must not be empty", "")
	}

	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	transport := newGRPCTransport(address, cfg)

	return &DbClient{
		address:   address,
		config:    cfg,
		transport: transport,
	}, nil
}

// newClientWithTransport creates a DbClient using a custom
// Transport (for testing — see the fake transport in
// v3_shape_test.go).
func newClientWithTransport(address string, transport Transport, opts ...ClientOption) *DbClient {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &DbClient{
		address:   address,
		config:    cfg,
		transport: transport,
	}
}

// Connect establishes the gRPC connection to the server.
func (c *DbClient) Connect(ctx context.Context) error {
	return c.transport.Connect(ctx)
}

// Close tears down the gRPC connection.
func (c *DbClient) Close() error {
	return c.transport.Close()
}

// Address returns the server address this client targets.
func (c *DbClient) Address() string { return c.address }

// Config returns a copy of the client configuration (for inspection/testing).
func (c *DbClient) Config() clientConfig { return c.config }

// Tenant returns a TenantScope for the given tenant.
//
//	scope := client.Tenant("acme").Actor(entdb.UserActor("bob"))
//	product, err := entdb.Get[*shop.Product](ctx, scope, "node-42")
func (c *DbClient) Tenant(tenantID string) *TenantScope {
	return &TenantScope{
		client:   c,
		tenantID: tenantID,
	}
}

// NewPlan creates a new Plan for batching operations atomically.
func (c *DbClient) NewPlan(tenantID, actor string) *Plan {
	return newPlan(c.transport, tenantID, actor, "")
}

// NewPlanWithKey creates a new Plan with an explicit idempotency key.
func (c *DbClient) NewPlanWithKey(tenantID, actor, idempotencyKey string) *Plan {
	return newPlan(c.transport, tenantID, actor, idempotencyKey)
}
