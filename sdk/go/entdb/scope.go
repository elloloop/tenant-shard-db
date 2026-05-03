package entdb

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"
)

// TenantScope captures a (client, tenantID) pair, allowing callers to
// avoid repeating the tenant on every call.
type TenantScope struct {
	client   *DbClient
	tenantID string
}

// Actor binds a typed Actor to this tenant scope, returning a fully-scoped
// Scope ready for reads and writes.
func (s *TenantScope) Actor(actor Actor) *Scope {
	return &Scope{
		client:   s.client,
		tenantID: s.tenantID,
		actor:    actor,
	}
}

// TenantID returns the tenant this scope is bound to.
func (s *TenantScope) TenantID() string { return s.tenantID }

// Scope is a fully-scoped handle with (client, tenantID, actor) pre-bound.
//
// The typed accessors [Get], [GetByKey], and [Query] are package-
// level free functions rather than methods on Scope because Go
// does not allow generic methods. The canonical call shape is:
//
//	scope := client.Tenant("acme").Actor(entdb.UserActor("bob"))
//	product, err := entdb.Get[*shop.Product](ctx, scope, "node-42")
//	match, err := entdb.GetByKey(ctx, scope, shop.ProductSKU, "WIDGET-1")
//	results, err := entdb.Query[*shop.Product](ctx, scope,
//	    map[string]any{"price_cents": map[string]any{"$gte": 1000}})
//
// The free-function-with-scope-as-first-arg pattern is the canonical
// Go workaround for generic methods.
type Scope struct {
	client   *DbClient
	tenantID string
	actor    Actor
}

// TenantID returns the tenant this scope is bound to.
func (s *Scope) TenantID() string { return s.tenantID }

// ActorID returns the actor this scope is bound to.
func (s *Scope) ActorID() Actor { return s.actor }

// Share grants permission on a node to another actor.
//
// Share stays a regular method because it does not need a type
// parameter — the node_id fully identifies the row.
func (s *Scope) Share(ctx context.Context, nodeID string, grantee Actor, perm Permission) error {
	return s.client.transport.Share(ctx, s.tenantID, s.actor.String(), nodeID, grantee.String(), string(perm))
}

// EdgesFrom retrieves outgoing edges from a node via this scope.
//
// Kept non-generic because the caller passes the edge_type_id as
// an int — a typed “EdgeType[*shop.PurchaseEdge]“ overload can
// be added as a strict replacement when the need is concrete.
func (s *Scope) EdgesFrom(ctx context.Context, fromNodeID string, edgeTypeID int) ([]*Edge, error) {
	return s.client.transport.GetEdgesFrom(ctx, s.tenantID, s.actor.String(), fromNodeID, edgeTypeID)
}

// EdgesTo retrieves incoming edges to a node via this scope.
func (s *Scope) EdgesTo(ctx context.Context, toNodeID string, edgeTypeID int) ([]*Edge, error) {
	return s.client.transport.GetEdgesTo(ctx, s.tenantID, s.actor.String(), toNodeID, edgeTypeID)
}

// RevokeAccess removes a previously-shared grant on a node.
//
// Returns true if the grant existed and was removed, false if no
// matching grant was found (idempotent — re-revoke is a no-op).
func (s *Scope) RevokeAccess(ctx context.Context, nodeID string, grantee Actor) (bool, error) {
	return s.client.transport.RevokeAccess(ctx, s.tenantID, s.actor.String(), nodeID, grantee.String())
}

// TransferOwnership reassigns ``nodeID``'s owner to ``newOwner``.
// Returns false if the node was not found.
func (s *Scope) TransferOwnership(ctx context.Context, nodeID string, newOwner Actor) (bool, error) {
	return s.client.transport.TransferOwnership(ctx, s.tenantID, s.actor.String(), nodeID, newOwner.String())
}

// AddGroupMember adds ``member`` to the ACL group identified by
// ``groupID`` (a node id of the group node). ``role`` is an
// optional free-form label like ``"admin"`` or ``"member"``.
func (s *Scope) AddGroupMember(ctx context.Context, groupID string, member Actor, role string) error {
	return s.client.transport.AddGroupMember(ctx, s.tenantID, s.actor.String(), groupID, member.String(), role)
}

// RemoveGroupMember removes ``member`` from the ACL group.
func (s *Scope) RemoveGroupMember(ctx context.Context, groupID string, member Actor) error {
	return s.client.transport.RemoveGroupMember(ctx, s.tenantID, s.actor.String(), groupID, member.String())
}

// SharedWithMe returns nodes other actors have shared with this
// scope's actor — including cross-tenant shares routed through the
// global shared_index.
//
// ``limit``/``offset`` of zero use the server defaults.
func (s *Scope) SharedWithMe(ctx context.Context, limit, offset int32) ([]*Node, error) {
	return s.client.transport.ListSharedWithMe(ctx, s.tenantID, s.actor.String(), limit, offset)
}

// Connected returns nodes reachable from ``nodeID`` via edges of
// ``edgeTypeID``. Server-side ACL filtering is applied.
func (s *Scope) Connected(ctx context.Context, nodeID string, edgeTypeID int) ([]*Node, error) {
	return s.client.transport.GetConnectedNodes(ctx, s.tenantID, s.actor.String(), nodeID, edgeTypeID)
}

// Plan creates a new Plan pre-bound to this scope's tenant and actor.
func (s *Scope) Plan() *Plan {
	return s.client.NewPlan(s.tenantID, s.actor.String())
}

// PlanWithKey creates a new Plan with an explicit idempotency key,
// pre-bound to this scope's tenant and actor.
func (s *Scope) PlanWithKey(idempotencyKey string) *Plan {
	return s.client.NewPlanWithKey(s.tenantID, s.actor.String(), idempotencyKey)
}

// ── Generic free functions ──────────────────────────────────────────

// Get fetches a node by id.
//
// T is the proto message type; the SDK reads the “(entdb.node).type_id“
// from its descriptor:
//
//	product, err := entdb.Get[*shop.Product](ctx, scope, "node-42")
func Get[T proto.Message](ctx context.Context, s *Scope, nodeID string) (T, error) {
	var zero T
	witness := newZeroMessage[T]()
	typeID, err := typeIDFromMessage(witness)
	if err != nil {
		return zero, fmt.Errorf("entdb: Get: %w", err)
	}
	node, err := s.client.transport.GetNode(ctx, s.tenantID, s.actor.String(), int(typeID), nodeID)
	if err != nil {
		return zero, err
	}
	if node == nil {
		return zero, nil
	}
	return unmarshalFromWire[T](node.Payload)
}

// GetByKey fetches a node by a unique key token. The token is
// produced by the protoc-gen-entdb-keys codegen plugin — user code
// cannot construct one by hand.
//
//	product, err := entdb.GetByKey(ctx, scope, shop.ProductSKU, "WIDGET-1")
//
// The generic T on UniqueKey[T] constrains the value argument at
// compile time: passing an int where the key declares
// UniqueKey[string] does not type-check.
func GetByKey[T any](ctx context.Context, s *Scope, key UniqueKey[T], value T) (*Node, error) {
	pv, err := toProtoValue(value)
	if err != nil {
		return nil, fmt.Errorf("entdb: GetByKey: %w", err)
	}
	return s.client.transport.GetNodeByKey(ctx, s.tenantID, s.actor.String(), key.TypeID, key.FieldID, pv)
}

// Query fetches nodes by filter.
//
// T determines the type_id and the result type. The filter map
// accepts MongoDB-style operators described in the 2026-04-14 SDK
// v0.3 decision (sdk_api.md):
//
//	$eq (default), $ne, $gt, $gte, $lt, $lte, $in, $nin, $like,
//	$between, plus top-level $and / $or.
//
// Field names are proto field names; the server resolves them to
// field ids at the ingress boundary.
func Query[T proto.Message](ctx context.Context, s *Scope, filter map[string]any, opts ...QueryOption) ([]T, error) {
	witness := newZeroMessage[T]()
	typeID, err := typeIDFromMessage(witness)
	if err != nil {
		return nil, fmt.Errorf("entdb: Query: %w", err)
	}
	// queryConfig is unused on the wire today — limit/offset will
	// land when the transport supports them. Building the config
	// keeps the public signature forward-compatible.
	var cfg queryConfig
	for _, o := range opts {
		o(&cfg)
	}
	nodes, err := s.client.transport.QueryNodes(ctx, s.tenantID, s.actor.String(), int(typeID), filter)
	if err != nil {
		return nil, err
	}
	out := make([]T, 0, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		conv, err := unmarshalFromWire[T](n.Payload)
		if err != nil {
			return nil, err
		}
		out = append(out, conv)
	}
	return out, nil
}

// GetMany batch-fetches nodes by id. Returns the list of unmarshaled
// messages plus the list of ids the server reported as missing.
//
//	products, missing, err := entdb.GetMany[*shop.Product](
//	    ctx, scope, []string{"node-1", "node-2", "node-3"})
//
// Order is not guaranteed; callers needing input-order alignment
// should index by ``Node.NodeID``.
func GetMany[T proto.Message](ctx context.Context, s *Scope, nodeIDs []string) ([]T, []string, error) {
	witness := newZeroMessage[T]()
	typeID, err := typeIDFromMessage(witness)
	if err != nil {
		return nil, nil, fmt.Errorf("entdb: GetMany: %w", err)
	}
	nodes, missing, err := s.client.transport.GetNodes(
		ctx, s.tenantID, s.actor.String(), int(typeID), nodeIDs,
	)
	if err != nil {
		return nil, nil, err
	}
	out := make([]T, 0, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		conv, err := unmarshalFromWire[T](n.Payload)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, conv)
	}
	return out, missing, nil
}

// Search performs full-text search across searchable fields of a node
// type.
//
// T determines the type_id and the result type. The query string is
// an FTS5 match expression supporting AND, OR, NOT, phrase ("..."),
// and prefix (word*) syntax. Only fields declared with
// (entdb.field).searchable = true are searched.
//
//	results, err := entdb.Search[*shop.Product](ctx, scope, "widget")
func Search[T proto.Message](ctx context.Context, s *Scope, query string, opts ...QueryOption) ([]T, error) {
	witness := newZeroMessage[T]()
	typeID, err := typeIDFromMessage(witness)
	if err != nil {
		return nil, fmt.Errorf("entdb: Search: %w", err)
	}
	var cfg queryConfig
	for _, o := range opts {
		o(&cfg)
	}
	nodes, err := s.client.transport.SearchNodes(ctx, s.tenantID, s.actor.String(), int(typeID), query)
	if err != nil {
		return nil, err
	}
	out := make([]T, 0, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		conv, err := unmarshalFromWire[T](n.Payload)
		if err != nil {
			return nil, err
		}
		out = append(out, conv)
	}
	return out, nil
}
