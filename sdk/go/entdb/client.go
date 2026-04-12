package entdb

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Transport is the interface for the gRPC transport layer.
// This allows testing without a live server and decouples the SDK
// logic from the generated proto stubs.
type Transport interface {
	// Connect establishes the connection to the server.
	Connect(ctx context.Context) error
	// Close tears down the connection.
	Close() error
	// GetNode retrieves a single node.
	GetNode(ctx context.Context, tenantID, actor string, typeID int, nodeID string) (*Node, error)
	// QueryNodes retrieves nodes matching a filter.
	QueryNodes(ctx context.Context, tenantID, actor string, typeID int, filter map[string]any) ([]*Node, error)
	// ExecuteAtomic commits a batch of operations atomically.
	ExecuteAtomic(ctx context.Context, tenantID, actor, idempotencyKey string, ops []Operation) (*CommitResult, error)
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

func (t *grpcTransport) Connect(ctx context.Context) error {
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

func (t *grpcTransport) QueryNodes(_ context.Context, _, _ string, _ int, _ map[string]any) ([]*Node, error) {
	// Requires generated proto stubs to implement.
	return nil, &EntDBError{Message: "not implemented: requires proto stubs", Code: "NOT_IMPLEMENTED"}
}

func (t *grpcTransport) ExecuteAtomic(_ context.Context, _, _, _ string, _ []Operation) (*CommitResult, error) {
	// Requires generated proto stubs to implement.
	return nil, &EntDBError{Message: "not implemented: requires proto stubs", Code: "NOT_IMPLEMENTED"}
}

// DbClient is the main entry point for the EntDB Go SDK.
//
// It manages a gRPC connection and provides both a flat API (with explicit
// tenant/actor parameters) and a hierarchical scope API.
type DbClient struct {
	address   string
	config    clientConfig
	transport Transport
}

// NewClient creates a new DbClient targeting the given server address.
//
// The client is not connected until Connect is called.
//
//	client, err := entdb.NewClient("localhost:50051", entdb.WithSecure(), entdb.WithAPIKey("key"))
//	if err != nil { ... }
//	defer client.Close()
//	if err := client.Connect(ctx); err != nil { ... }
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

// newClientWithTransport creates a DbClient using a custom Transport (for testing).
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

// Tenant returns a TenantScope for the given tenant, allowing callers to
// chain .Actor(actor) to get a fully-scoped handle.
//
//	scope := client.Tenant("acme").Actor("user:bob")
//	node, err := scope.Get(ctx, 1, "node-123")
func (c *DbClient) Tenant(tenantID string) *TenantScope {
	return &TenantScope{
		client:   c,
		tenantID: tenantID,
	}
}

// Get retrieves a single node by type and ID (flat API).
func (c *DbClient) Get(ctx context.Context, tenantID, actor string, typeID int, nodeID string) (*Node, error) {
	return c.transport.GetNode(ctx, tenantID, actor, typeID, nodeID)
}

// Query retrieves nodes matching a filter (flat API).
func (c *DbClient) Query(ctx context.Context, tenantID, actor string, typeID int, filter map[string]any) ([]*Node, error) {
	return c.transport.QueryNodes(ctx, tenantID, actor, typeID, filter)
}

// NewPlan creates a new Plan for batching operations atomically (flat API).
func (c *DbClient) NewPlan(tenantID, actor string) *Plan {
	return newPlan(c.transport, tenantID, actor, "")
}

// NewPlanWithKey creates a new Plan with an explicit idempotency key.
func (c *DbClient) NewPlanWithKey(tenantID, actor, idempotencyKey string) *Plan {
	return newPlan(c.transport, tenantID, actor, idempotencyKey)
}
