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
	// GetNodes batch-fetches nodes by id. Returns the slice of
	// found nodes plus the slice of ids that were missing — both in
	// input order is NOT guaranteed.
	GetNodes(ctx context.Context, tenantID, actor string, typeID int, nodeIDs []string) ([]*Node, []string, error)
	// GetReceiptStatus polls a transaction by its idempotency key.
	GetReceiptStatus(ctx context.Context, tenantID, actor, idempotencyKey string) (ReceiptStatus, string, error)
	// WaitForOffset blocks (server-side) until the applier has
	// reached ``streamPosition`` for the tenant, or ``timeoutMs``
	// elapses. Returns whether the offset was reached and the
	// current position (so callers can detect "still behind by N").
	WaitForOffset(ctx context.Context, tenantID, actor, streamPosition string, timeoutMs int32) (bool, string, error)
	// GetConnectedNodes returns nodes reachable from ``nodeID`` via
	// edges of ``edgeTypeID`` — server-side ACL filtered.
	GetConnectedNodes(ctx context.Context, tenantID, actor, nodeID string, edgeTypeID int) ([]*Node, error)
	// ListSharedWithMe returns nodes other actors have shared with
	// the calling actor (cross-tenant included).
	ListSharedWithMe(ctx context.Context, tenantID, actor string, limit, offset int32) ([]*Node, error)
	// RevokeAccess removes a previously-shared grant from a node.
	RevokeAccess(ctx context.Context, tenantID, actor, nodeID, granteeActor string) (bool, error)
	// TransferOwnership reassigns ``nodeID``'s ``owner_actor`` to
	// ``newOwner``. Returns false if the node was not found.
	TransferOwnership(ctx context.Context, tenantID, actor, nodeID, newOwner string) (bool, error)
	// AddGroupMember / RemoveGroupMember manage the membership of an
	// ACL group node — ``groupID`` is the node id of the group,
	// ``memberActor`` is the principal being added or removed.
	AddGroupMember(ctx context.Context, tenantID, actor, groupID, memberActor, role string) error
	RemoveGroupMember(ctx context.Context, tenantID, actor, groupID, memberActor string) error

	// ── Admin / cross-tenant identity (``client.Admin()``) ──────────
	// Cross-tenant identity operations grouped under DbClient.Admin().
	// They take ``actor`` as a string (typically ``"system:admin"`` or
	// a tenant owner/admin actor) rather than a scoped Actor because
	// they either operate above a single tenant (CreateTenant,
	// CreateUser, GetUserTenants) or want raw ``user_id`` form to
	// match the global registry's identifier shape.

	CreateTenant(ctx context.Context, actor, tenantID, name string, opts ...CreateTenantOption) (*TenantDetail, error)
	CreateUser(ctx context.Context, actor, userID, email, name string) (*UserInfo, error)
	AddTenantMember(ctx context.Context, actor, tenantID, userID, role string) error
	RemoveTenantMember(ctx context.Context, actor, tenantID, userID string) error
	ChangeMemberRole(ctx context.Context, actor, tenantID, userID, newRole string) error
	GetTenantMembers(ctx context.Context, actor, tenantID string) ([]TenantMember, error)
	GetUserTenants(ctx context.Context, actor, userID string) ([]TenantMember, error)
	DelegateAccess(ctx context.Context, actor, tenantID, fromUser, toUser, permission string, expiresAtMs int64) (*DelegateResult, error)
	TransferUserContent(ctx context.Context, actor, tenantID, fromUser, toUser string) (int32, error)
	RevokeAllUserAccess(ctx context.Context, actor, tenantID, userID string) (*RevokeAllResult, error)

	// ── GDPR user lifecycle (right-to-be-forgotten + deletion grace) ─

	// DeleteUser requests deletion of ``userID``. The deletion is
	// scheduled — actually executed by the GDPR worker after a
	// grace period. Pass ``graceDays = 0`` to use the server
	// default (30 days). The user enters the ``"deletion_pending"``
	// status immediately; reads continue to succeed until the grace
	// expires.
	DeleteUser(ctx context.Context, actor, userID string, graceDays int32) (*DeletionScheduled, error)
	// CancelUserDeletion lifts a pending deletion. Idempotent on a
	// user with no pending deletion (returns success but no
	// observable change).
	CancelUserDeletion(ctx context.Context, actor, userID string) error
	// ExportUserData returns a JSON-serialized bundle of every node
	// the user owns across all tenants — the GDPR Article 20
	// portability dump. The shape is documented on the server side.
	ExportUserData(ctx context.Context, actor, userID string) (string, error)
	// FreezeUser flips ``userID`` between ``"active"`` and
	// ``"frozen"``. A frozen user cannot read or write, but their
	// data is preserved (no deletion clock starts). Pass
	// ``enabled = true`` to freeze, ``false`` to unfreeze.
	FreezeUser(ctx context.Context, actor, userID string, enabled bool) (string, error)

	// Health returns a snapshot of server health — useful as a
	// Kubernetes liveness/readiness probe target. Tenant-agnostic.
	Health(ctx context.Context) (*HealthStatus, error)
}

// HealthStatus mirrors ``pb.HealthResponse`` — the boolean
// healthy flag, the server version string, and a map of named
// components to their status (e.g. "wal" → "ok",
// "global_store" → "ok").
type HealthStatus struct {
	Healthy    bool
	Version    string
	Components map[string]string
}

// DeletionScheduled is what ``Admin.DeleteUser`` returns — the
// GDPR-mandated timestamps so callers can show "your data will be
// permanently removed at <date>" UI without a follow-up read.
type DeletionScheduled struct {
	RequestedAtMs int64
	ExecuteAtMs   int64
	Status        string // typically "deletion_pending"
}

// TenantDetail mirrors ``pb.TenantDetail`` — the identity-layer
// description of a tenant (not its quota, not its data).
//
// ``Region`` is the geographic region the tenant's data is pinned to
// (e.g. ``"us-east-1"``, ``"eu-west-1"``). It is set when the tenant
// is created — either explicitly via [WithRegion] on
// [Admin.CreateTenant] or, when omitted, defaulted server-side to
// the serving region. Cross-region requests are rejected with
// FAILED_PRECONDITION; clients that need to talk to a tenant in
// another region must connect to a server in that region.
type TenantDetail struct {
	TenantID  string
	Name      string
	Status    string
	Region    string
	CreatedAt int64
}

// UserInfo mirrors ``pb.UserInfo`` from the global user registry.
type UserInfo struct {
	UserID    string
	Email     string
	Name      string
	Status    string
	CreatedAt int64
	UpdatedAt int64
}

// TenantMember describes one ``(tenant_id, user_id, role)`` row
// from ``tenant_members`` — used by both ``GetTenantMembers`` (rows
// for one tenant) and ``GetUserTenants`` (rows for one user).
type TenantMember struct {
	TenantID string
	UserID   string
	Role     string
	JoinedAt int64
}

// DelegateResult is returned by ``Admin.DelegateAccess`` — how many
// nodes were affected and the absolute expiry timestamp.
type DelegateResult struct {
	Delegated   int32
	ExpiresAtMs int64
}

// RevokeAllResult tallies what ``Admin.RevokeAllUserAccess`` removed.
type RevokeAllResult struct {
	RevokedGrants int32
	RevokedGroups int32
	RevokedShared int32
}

// ReceiptStatus mirrors the wire enum (UNKNOWN / PENDING / APPLIED / FAILED).
type ReceiptStatus int32

const (
	ReceiptStatusUnknown ReceiptStatus = 0
	ReceiptStatusPending ReceiptStatus = 1
	ReceiptStatusApplied ReceiptStatus = 2
	ReceiptStatusFailed  ReceiptStatus = 3
)

// grpcTransport is the production Transport backed by a gRPC connection.
type grpcTransport struct {
	address string
	config  clientConfig
	conn    *grpc.ClientConn
	client  pb.EntDBServiceClient
	// redirectCache is non-nil when the client was constructed
	// with a [NodeResolver] (via [WithNodeResolver] or
	// [WithBaseDomain]). It caches per-tenant sub-channels for
	// nodes the SDK has been redirected to.
	redirectCache *tenantEndpointCache
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
//
// When ``tenantID`` is non-empty it is also stamped onto the
// outgoing context as ``entdb-tenant-id``. The redirect
// interceptor (see ``redirect_cache.go``) reads this header to
// route the call against a cached sub-channel without having to
// reflect over every request type.
func (t *grpcTransport) callContext(ctx context.Context, tenantID string) context.Context {
	if t.config.apiKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+t.config.apiKey)
	}
	if tenantID != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, outgoingTenantHeader, tenantID)
	}
	return ctx
}

func (t *grpcTransport) Connect(_ context.Context) error {
	// When a NodeResolver is configured, the redirect cache holds
	// per-tenant sub-channels and a unary interceptor routes calls
	// onto them transparently. The cache uses the same dial
	// settings (TLS / insecure / explicit dial options) as the
	// primary connection.
	if t.config.nodeResolver != nil && t.redirectCache == nil {
		t.redirectCache = newTenantEndpointCache(dialerFromConfig(t.config))
	}

	opts := append([]grpc.DialOption(nil), t.config.dialOptions...)
	if !hasTransportCreds(opts) {
		var creds credentials.TransportCredentials
		if t.config.secure {
			creds = credentials.NewTLS(nil)
		} else {
			creds = insecure.NewCredentials()
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}
	if interceptor := redirectInterceptor(t.config.nodeResolver, t.redirectCache); interceptor != nil {
		opts = append(opts, grpc.WithUnaryInterceptor(interceptor))
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
	if t.redirectCache != nil {
		t.redirectCache.closeAll()
	}
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
	resp, err := client.GetNode(t.callContext(ctx, tenantID), &pb.GetNodeRequest{
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
	resp, err := client.GetNodeByKey(t.callContext(ctx, tenantID), &pb.GetNodeByKeyRequest{
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
	resp, err := client.QueryNodes(t.callContext(ctx, tenantID), &pb.QueryNodesRequest{
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
	resp, err := client.ExecuteAtomic(t.callContext(ctx, tenantID), &pb.ExecuteAtomicRequest{
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
	_, err = client.ShareNode(t.callContext(ctx, tenantID), &pb.ShareNodeRequest{
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
	resp, err := client.GetEdgesFrom(t.callContext(ctx, tenantID), &pb.GetEdgesRequest{
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
	resp, err := client.GetEdgesTo(t.callContext(ctx, tenantID), &pb.GetEdgesRequest{
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
	resp, err := client.SearchNodes(t.callContext(ctx, tenantID), &pb.SearchNodesRequest{
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
	resp, err := client.GetTenantQuota(t.callContext(ctx, tenantID), &pb.GetTenantQuotaRequest{
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

func (t *grpcTransport) GetNodes(ctx context.Context, tenantID, actor string, typeID int, nodeIDs []string) ([]*Node, []string, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, nil, err
	}
	var trailer metadata.MD
	resp, err := client.GetNodes(t.callContext(ctx, tenantID), &pb.GetNodesRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: actor},
		TypeId:  int32(typeID),
		NodeIds: nodeIDs,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, nil, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	nodes := resp.GetNodes()
	out := make([]*Node, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, nodeFromProto(n))
	}
	return out, resp.GetMissingIds(), nil
}

func (t *grpcTransport) GetReceiptStatus(ctx context.Context, tenantID, actor, idempotencyKey string) (ReceiptStatus, string, error) {
	client, err := t.ensureReady()
	if err != nil {
		return ReceiptStatusUnknown, "", err
	}
	var trailer metadata.MD
	resp, err := client.GetReceiptStatus(t.callContext(ctx, tenantID), &pb.GetReceiptStatusRequest{
		Context:        &pb.RequestContext{TenantId: tenantID, Actor: actor},
		IdempotencyKey: idempotencyKey,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return ReceiptStatusUnknown, "", translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	return ReceiptStatus(resp.GetStatus()), resp.GetError(), nil
}

func (t *grpcTransport) WaitForOffset(ctx context.Context, tenantID, actor, streamPosition string, timeoutMs int32) (bool, string, error) {
	client, err := t.ensureReady()
	if err != nil {
		return false, "", err
	}
	var trailer metadata.MD
	resp, err := client.WaitForOffset(t.callContext(ctx, tenantID), &pb.WaitForOffsetRequest{
		Context:        &pb.RequestContext{TenantId: tenantID, Actor: actor},
		StreamPosition: streamPosition,
		TimeoutMs:      timeoutMs,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return false, "", translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	return resp.GetReached(), resp.GetCurrentPosition(), nil
}

func (t *grpcTransport) GetConnectedNodes(ctx context.Context, tenantID, actor, nodeID string, edgeTypeID int) ([]*Node, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.GetConnectedNodes(t.callContext(ctx, tenantID), &pb.GetConnectedNodesRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: actor},
		NodeId:     nodeID,
		EdgeTypeId: int32(edgeTypeID),
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

func (t *grpcTransport) ListSharedWithMe(ctx context.Context, tenantID, actor string, limit, offset int32) ([]*Node, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.ListSharedWithMe(t.callContext(ctx, tenantID), &pb.ListSharedWithMeRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: actor},
		Limit:   limit,
		Offset:  offset,
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

func (t *grpcTransport) RevokeAccess(ctx context.Context, tenantID, actor, nodeID, granteeActor string) (bool, error) {
	client, err := t.ensureReady()
	if err != nil {
		return false, err
	}
	var trailer metadata.MD
	resp, err := client.RevokeAccess(t.callContext(ctx, tenantID), &pb.RevokeAccessRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: actor},
		NodeId:  nodeID,
		ActorId: granteeActor,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return false, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	return resp.GetFound(), nil
}

func (t *grpcTransport) TransferOwnership(ctx context.Context, tenantID, actor, nodeID, newOwner string) (bool, error) {
	client, err := t.ensureReady()
	if err != nil {
		return false, err
	}
	var trailer metadata.MD
	resp, err := client.TransferOwnership(t.callContext(ctx, tenantID), &pb.TransferOwnershipRequest{
		Context:  &pb.RequestContext{TenantId: tenantID, Actor: actor},
		NodeId:   nodeID,
		NewOwner: newOwner,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return false, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	return resp.GetFound(), nil
}

func (t *grpcTransport) AddGroupMember(ctx context.Context, tenantID, actor, groupID, memberActor, role string) error {
	client, err := t.ensureReady()
	if err != nil {
		return err
	}
	var trailer metadata.MD
	_, err = client.AddGroupMember(t.callContext(ctx, tenantID), &pb.GroupMemberRequest{
		Context:       &pb.RequestContext{TenantId: tenantID, Actor: actor},
		GroupId:       groupID,
		MemberActorId: memberActor,
		Role:          role,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	return nil
}

func (t *grpcTransport) RemoveGroupMember(ctx context.Context, tenantID, actor, groupID, memberActor string) error {
	client, err := t.ensureReady()
	if err != nil {
		return err
	}
	var trailer metadata.MD
	_, err = client.RemoveGroupMember(t.callContext(ctx, tenantID), &pb.GroupMemberRequest{
		Context:       &pb.RequestContext{TenantId: tenantID, Actor: actor},
		GroupId:       groupID,
		MemberActorId: memberActor,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	return nil
}

// ── Admin (cross-tenant identity) wire methods ───────────────────────

func (t *grpcTransport) CreateTenant(ctx context.Context, actor, tenantID, name string, opts ...CreateTenantOption) (*TenantDetail, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	cfg := createTenantConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	var trailer metadata.MD
	resp, err := client.CreateTenant(t.callContext(ctx, tenantID), &pb.CreateTenantRequest{
		Actor:    actor,
		TenantId: tenantID,
		Name:     name,
		Region:   cfg.region,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	if !resp.GetSuccess() {
		return nil, &EntDBError{Message: resp.GetError(), Code: "ADMIN_ERROR"}
	}
	td := resp.GetTenant()
	return &TenantDetail{
		TenantID:  td.GetTenantId(),
		Name:      td.GetName(),
		Status:    td.GetStatus(),
		Region:    td.GetRegion(),
		CreatedAt: td.GetCreatedAt(),
	}, nil
}

func (t *grpcTransport) CreateUser(ctx context.Context, actor, userID, email, name string) (*UserInfo, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.CreateUser(t.callContext(ctx, ""), &pb.CreateUserRequest{
		Actor:  actor,
		UserId: userID,
		Email:  email,
		Name:   name,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, "", t.address)
	}
	if !resp.GetSuccess() {
		return nil, &EntDBError{Message: resp.GetError(), Code: "ADMIN_ERROR"}
	}
	u := resp.GetUser()
	return &UserInfo{
		UserID:    u.GetUserId(),
		Email:     u.GetEmail(),
		Name:      u.GetName(),
		Status:    u.GetStatus(),
		CreatedAt: u.GetCreatedAt(),
		UpdatedAt: u.GetUpdatedAt(),
	}, nil
}

func (t *grpcTransport) AddTenantMember(ctx context.Context, actor, tenantID, userID, role string) error {
	client, err := t.ensureReady()
	if err != nil {
		return err
	}
	var trailer metadata.MD
	resp, err := client.AddTenantMember(t.callContext(ctx, tenantID), &pb.TenantMemberRequest{
		Actor:    actor,
		TenantId: tenantID,
		UserId:   userID,
		Role:     role,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	if !resp.GetSuccess() {
		return &EntDBError{Message: resp.GetError(), Code: "ADMIN_ERROR"}
	}
	return nil
}

func (t *grpcTransport) RemoveTenantMember(ctx context.Context, actor, tenantID, userID string) error {
	client, err := t.ensureReady()
	if err != nil {
		return err
	}
	var trailer metadata.MD
	resp, err := client.RemoveTenantMember(t.callContext(ctx, tenantID), &pb.TenantMemberRequest{
		Actor:    actor,
		TenantId: tenantID,
		UserId:   userID,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	if !resp.GetSuccess() {
		return &EntDBError{Message: resp.GetError(), Code: "ADMIN_ERROR"}
	}
	return nil
}

func (t *grpcTransport) ChangeMemberRole(ctx context.Context, actor, tenantID, userID, newRole string) error {
	client, err := t.ensureReady()
	if err != nil {
		return err
	}
	var trailer metadata.MD
	resp, err := client.ChangeMemberRole(t.callContext(ctx, tenantID), &pb.ChangeMemberRoleRequest{
		Actor:    actor,
		TenantId: tenantID,
		UserId:   userID,
		NewRole:  newRole,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	if !resp.GetSuccess() {
		return &EntDBError{Message: resp.GetError(), Code: "ADMIN_ERROR"}
	}
	return nil
}

func (t *grpcTransport) GetTenantMembers(ctx context.Context, actor, tenantID string) ([]TenantMember, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.GetTenantMembers(t.callContext(ctx, tenantID), &pb.GetTenantMembersRequest{
		Actor:    actor,
		TenantId: tenantID,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	out := make([]TenantMember, 0, len(resp.GetMembers()))
	for _, m := range resp.GetMembers() {
		out = append(out, TenantMember{
			TenantID: m.GetTenantId(),
			UserID:   m.GetUserId(),
			Role:     m.GetRole(),
			JoinedAt: m.GetJoinedAt(),
		})
	}
	return out, nil
}

func (t *grpcTransport) GetUserTenants(ctx context.Context, actor, userID string) ([]TenantMember, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.GetUserTenants(t.callContext(ctx, ""), &pb.GetUserTenantsRequest{
		Actor:  actor,
		UserId: userID,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, "", t.address)
	}
	out := make([]TenantMember, 0, len(resp.GetMemberships()))
	for _, m := range resp.GetMemberships() {
		out = append(out, TenantMember{
			TenantID: m.GetTenantId(),
			UserID:   m.GetUserId(),
			Role:     m.GetRole(),
			JoinedAt: m.GetJoinedAt(),
		})
	}
	return out, nil
}

func (t *grpcTransport) DelegateAccess(ctx context.Context, actor, tenantID, fromUser, toUser, permission string, expiresAtMs int64) (*DelegateResult, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.DelegateAccess(t.callContext(ctx, tenantID), &pb.DelegateAccessRequest{
		Actor:      actor,
		TenantId:   tenantID,
		FromUser:   fromUser,
		ToUser:     toUser,
		Permission: permission,
		ExpiresAt:  expiresAtMs,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	if !resp.GetSuccess() {
		return nil, &EntDBError{Message: resp.GetError(), Code: "ADMIN_ERROR"}
	}
	return &DelegateResult{
		Delegated:   resp.GetDelegated(),
		ExpiresAtMs: resp.GetExpiresAt(),
	}, nil
}

func (t *grpcTransport) TransferUserContent(ctx context.Context, actor, tenantID, fromUser, toUser string) (int32, error) {
	client, err := t.ensureReady()
	if err != nil {
		return 0, err
	}
	var trailer metadata.MD
	resp, err := client.TransferUserContent(t.callContext(ctx, tenantID), &pb.TransferUserContentRequest{
		Actor:    actor,
		TenantId: tenantID,
		FromUser: fromUser,
		ToUser:   toUser,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return 0, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	if !resp.GetSuccess() {
		return 0, &EntDBError{Message: resp.GetError(), Code: "ADMIN_ERROR"}
	}
	return resp.GetTransferred(), nil
}

func (t *grpcTransport) RevokeAllUserAccess(ctx context.Context, actor, tenantID, userID string) (*RevokeAllResult, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.RevokeAllUserAccess(t.callContext(ctx, tenantID), &pb.RevokeAllUserAccessRequest{
		Actor:    actor,
		TenantId: tenantID,
		UserId:   userID,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, tenantID, t.address)
	}
	if !resp.GetSuccess() {
		return nil, &EntDBError{Message: resp.GetError(), Code: "ADMIN_ERROR"}
	}
	return &RevokeAllResult{
		RevokedGrants: resp.GetRevokedGrants(),
		RevokedGroups: resp.GetRevokedGroups(),
		RevokedShared: resp.GetRevokedShared(),
	}, nil
}

// ── GDPR user-lifecycle wire methods ─────────────────────────────────

func (t *grpcTransport) DeleteUser(ctx context.Context, actor, userID string, graceDays int32) (*DeletionScheduled, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.DeleteUser(t.callContext(ctx, ""), &pb.DeleteUserRequest{
		Actor:     actor,
		UserId:    userID,
		GraceDays: graceDays,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, "", t.address)
	}
	if !resp.GetSuccess() {
		return nil, &EntDBError{Message: resp.GetError(), Code: "ADMIN_ERROR"}
	}
	return &DeletionScheduled{
		RequestedAtMs: resp.GetRequestedAt(),
		ExecuteAtMs:   resp.GetExecuteAt(),
		Status:        resp.GetStatus(),
	}, nil
}

func (t *grpcTransport) CancelUserDeletion(ctx context.Context, actor, userID string) error {
	client, err := t.ensureReady()
	if err != nil {
		return err
	}
	var trailer metadata.MD
	resp, err := client.CancelUserDeletion(t.callContext(ctx, ""), &pb.CancelUserDeletionRequest{
		Actor:  actor,
		UserId: userID,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return translateGRPCStatusWithTrailer(err, trailer, "", t.address)
	}
	if !resp.GetSuccess() {
		return &EntDBError{Message: resp.GetError(), Code: "ADMIN_ERROR"}
	}
	return nil
}

func (t *grpcTransport) ExportUserData(ctx context.Context, actor, userID string) (string, error) {
	client, err := t.ensureReady()
	if err != nil {
		return "", err
	}
	var trailer metadata.MD
	resp, err := client.ExportUserData(t.callContext(ctx, ""), &pb.ExportUserDataRequest{
		Actor:  actor,
		UserId: userID,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return "", translateGRPCStatusWithTrailer(err, trailer, "", t.address)
	}
	if !resp.GetSuccess() {
		return "", &EntDBError{Message: resp.GetError(), Code: "ADMIN_ERROR"}
	}
	return resp.GetExportJson(), nil
}

func (t *grpcTransport) Health(ctx context.Context) (*HealthStatus, error) {
	client, err := t.ensureReady()
	if err != nil {
		return nil, err
	}
	var trailer metadata.MD
	resp, err := client.Health(t.callContext(ctx, ""), &pb.HealthRequest{}, grpc.Trailer(&trailer))
	if err != nil {
		return nil, translateGRPCStatusWithTrailer(err, trailer, "", t.address)
	}
	return &HealthStatus{
		Healthy:    resp.GetHealthy(),
		Version:    resp.GetVersion(),
		Components: resp.GetComponents(),
	}, nil
}

func (t *grpcTransport) FreezeUser(ctx context.Context, actor, userID string, enabled bool) (string, error) {
	client, err := t.ensureReady()
	if err != nil {
		return "", err
	}
	var trailer metadata.MD
	resp, err := client.FreezeUser(t.callContext(ctx, ""), &pb.FreezeUserRequest{
		Actor:   actor,
		UserId:  userID,
		Enabled: enabled,
	}, grpc.Trailer(&trailer))
	if err != nil {
		return "", translateGRPCStatusWithTrailer(err, trailer, "", t.address)
	}
	if !resp.GetSuccess() {
		return "", &EntDBError{Message: resp.GetError(), Code: "ADMIN_ERROR"}
	}
	return resp.GetStatus(), nil
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

// WaitForOffset blocks server-side until the WAL applier has reached
// ``streamPosition`` for ``tenantID``, or ``timeoutMs`` elapses.
// Returns ``(reached, currentPosition, err)``.  ``reached`` is true
// when the offset was met; ``currentPosition`` lets the caller see
// how far behind the applier is when it wasn't.
//
// This is the explicit form of the ``wait_applied`` option callers
// can pass to [Plan.Commit] — useful for read-after-write across
// independent processes that share an idempotency key but not a
// commit chain.
func (c *DbClient) WaitForOffset(ctx context.Context, tenantID, actor, streamPosition string, timeoutMs int32) (bool, string, error) {
	return c.transport.WaitForOffset(ctx, tenantID, actor, streamPosition, timeoutMs)
}

// Health returns a snapshot of server health for liveness /
// readiness probes. The response includes a boolean healthy flag,
// the server version string, and a per-component status map (e.g.
// ``"wal"`` → ``"ok"``).
func (c *DbClient) Health(ctx context.Context) (*HealthStatus, error) {
	return c.transport.Health(ctx)
}

// GetReceiptStatus polls the status of a transaction by its
// idempotency key. Returns the status enum, an optional error
// message (when status is FAILED), and any transport error.
//
// Useful when a network blip dropped the original commit response
// — re-issuing the same idempotency_key on a new ExecuteAtomic call
// is also safe (the WAL deduplicates) but ``GetReceiptStatus`` is
// the cheap "did it actually land?" probe.
func (c *DbClient) GetReceiptStatus(ctx context.Context, tenantID, actor, idempotencyKey string) (ReceiptStatus, string, error) {
	return c.transport.GetReceiptStatus(ctx, tenantID, actor, idempotencyKey)
}
