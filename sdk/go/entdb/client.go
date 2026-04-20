package entdb

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
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
	// GetTenantQuota retrieves the tenant's quota configuration
	// and current usage (monthly writes + per-tenant / per-user
	// RPS buckets).
	GetTenantQuota(ctx context.Context, actor, tenantID string) (*TenantQuota, error)
}

// grpcTransport is the production Transport backed by a gRPC connection.
type grpcTransport struct {
	address string
	config  clientConfig
	conn    *grpc.ClientConn
	client  pb.EntDBServiceClient
}

func newGRPCTransport(address string, cfg clientConfig) *grpcTransport {
	return &grpcTransport{
		address: address,
		config:  cfg,
	}
}

// callContext appends SDK-wide metadata (e.g. Authorization for
// API keys) to the outgoing context. Done on every call rather
// than on the dial to match the Python SDK's behaviour — the
// metadata is per-call, not per-channel.
func (t *grpcTransport) callContext(ctx context.Context) context.Context {
	if t.config.apiKey == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+t.config.apiKey)
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
	t.client = pb.NewEntDBServiceClient(conn)
	return nil
}

func (t *grpcTransport) Close() error {
	if t.conn != nil {
		err := t.conn.Close()
		t.conn = nil
		t.client = nil
		return err
	}
	return nil
}

// ensureReady returns a connected gRPC stub or a typed
// ConnectionError when Connect has not been called (or Close was
// called). Every read method funnels through this guard so user
// code gets a precise error instead of a generic nil-pointer
// dereference.
func (t *grpcTransport) ensureReady() (pb.EntDBServiceClient, error) {
	if t.client == nil {
		return nil, NewConnectionError("client not connected: call Connect first", t.address)
	}
	return t.client, nil
}

func (t *grpcTransport) GetNode(ctx context.Context, tenantID, actor string, typeID int, nodeID string) (*Node, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.GetNode(t.callContext(ctx), &pb.GetNodeRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: actor},
		TypeId:  int32(typeID),
		NodeId:  nodeID,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	if !resp.GetFound() {
		return nil, nil
	}
	return nodeFromProto(resp.GetNode()), nil
}

func (t *grpcTransport) GetNodeByKey(ctx context.Context, tenantID, actor string, typeID, fieldID int32, value *structpb.Value) (*Node, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.GetNodeByKey(t.callContext(ctx), &pb.GetNodeByKeyRequest{
		TenantId: tenantID,
		Actor:    actor,
		TypeId:   typeID,
		FieldId:  fieldID,
		Value:    value,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	if !resp.GetFound() {
		return nil, nil
	}
	return nodeFromProto(resp.GetNode()), nil
}

func (t *grpcTransport) QueryNodes(ctx context.Context, tenantID, actor string, typeID int, filter map[string]any) ([]*Node, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	filters, err := filterToProto(filter)
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.QueryNodes(t.callContext(ctx), &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: actor},
		TypeId:  int32(typeID),
		Filters: filters,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	nodes := resp.GetNodes()
	out := make([]*Node, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, nodeFromProto(n))
	}
	return out, nil
}

func (t *grpcTransport) ExecuteAtomic(ctx context.Context, tenantID, actor, idempotencyKey string, ops []Operation) (*CommitResult, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	protoOps, err := operationsToProto(ops)
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.ExecuteAtomic(t.callContext(ctx), &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: tenantID, Actor: actor},
		IdempotencyKey: idempotencyKey,
		Operations:     protoOps,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	var receipt *Receipt
	if r := resp.GetReceipt(); r != nil && r.GetIdempotencyKey() != "" {
		receipt = &Receipt{
			TenantID:       r.GetTenantId(),
			IdempotencyKey: r.GetIdempotencyKey(),
			StreamPosition: r.GetStreamPosition(),
		}
	}
	return &CommitResult{
		Success:        resp.GetSuccess(),
		Receipt:        receipt,
		CreatedNodeIDs: resp.GetCreatedNodeIds(),
		Applied:        resp.GetAppliedStatus() == pb.ReceiptStatus_RECEIPT_STATUS_APPLIED,
		Error:          resp.GetError(),
	}, nil
}

func (t *grpcTransport) Share(ctx context.Context, tenantID, actor, nodeID, grantee, permission string) error {
	client, err := t.ensureReady()
	if err != nil {
		return err
	}
	var trailer metadata.MD
	_, err = client.ShareNode(t.callContext(ctx), &pb.ShareNodeRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: actor},
		NodeId:     nodeID,
		ActorId:    grantee,
		Permission: permission,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	return nil
}

func (t *grpcTransport) GetEdgesFrom(ctx context.Context, tenantID, actor, fromNodeID string, edgeTypeID int) ([]*Edge, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.GetEdgesFrom(t.callContext(ctx), &pb.GetEdgesRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: actor},
		NodeId:     fromNodeID,
		EdgeTypeId: int32(edgeTypeID),
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	edges := resp.GetEdges()
	out := make([]*Edge, 0, len(edges))
	for _, e := range edges {
		out = append(out, edgeFromProto(e))
	}
	return out, nil
}

func (t *grpcTransport) GetEdgesTo(ctx context.Context, tenantID, actor, toNodeID string, edgeTypeID int) ([]*Edge, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.GetEdgesTo(t.callContext(ctx), &pb.GetEdgesRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: actor},
		NodeId:     toNodeID,
		EdgeTypeId: int32(edgeTypeID),
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	edges := resp.GetEdges()
	out := make([]*Edge, 0, len(edges))
	for _, e := range edges {
		out = append(out, edgeFromProto(e))
	}
	return out, nil
}

func (t *grpcTransport) SearchNodes(ctx context.Context, tenantID, actor string, typeID int, query string) ([]*Node, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.SearchNodes(t.callContext(ctx), &pb.SearchNodesRequest{
		TenantId: tenantID,
		Actor:    actor,
		TypeId:   int32(typeID),
		Query:    query,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	nodes := resp.GetNodes()
	out := make([]*Node, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, nodeFromProto(n))
	}
	return out, nil
}

func (t *grpcTransport) GetTenantQuota(ctx context.Context, actor, tenantID string) (*TenantQuota, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.GetTenantQuota(t.callContext(ctx), &pb.GetTenantQuotaRequest{
		Actor:    actor,
		TenantId: tenantID,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	return &TenantQuota{
		TenantID:               resp.GetTenantId(),
		MaxWritesPerMonth:      resp.GetMaxWritesPerMonth(),
		WritesUsed:             resp.GetWritesUsed(),
		PeriodStartMs:          resp.GetPeriodStartMs(),
		PeriodEndMs:            resp.GetPeriodEndMs(),
		MaxRPSSustained:        resp.GetMaxRpsSustained(),
		MaxRPSBurst:            resp.GetMaxRpsBurst(),
		MaxRPSPerUserSustained: resp.GetMaxRpsPerUserSustained(),
		MaxRPSPerUserBurst:     resp.GetMaxRpsPerUserBurst(),
		HardEnforce:            resp.GetHardEnforce(),
	}, nil
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

// GetTenantQuota fetches the tenant's quota configuration and
// current usage in a single round-trip. It is a direct client-
// level call (rather than scoped) because quota data is admin-
// oriented — there is no typed node behind it.
func (c *DbClient) GetTenantQuota(ctx context.Context, actor, tenantID string) (*TenantQuota, error) {
	return c.transport.GetTenantQuota(ctx, actor, tenantID)
}
