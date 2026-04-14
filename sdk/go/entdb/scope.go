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
