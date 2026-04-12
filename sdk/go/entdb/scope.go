package entdb

import "context"

// TenantScope captures a (client, tenantID) pair, allowing callers to
// avoid repeating the tenant on every call.
type TenantScope struct {
	client   *DbClient
	tenantID string
}

// Actor binds a typed Actor to this tenant scope, returning a fully-scoped
// ActorScope ready for reads and writes.
func (s *TenantScope) Actor(actor Actor) *ActorScope {
	return &ActorScope{
		client:   s.client,
		tenantID: s.tenantID,
		actor:    actor,
	}
}

// TenantID returns the tenant this scope is bound to.
func (s *TenantScope) TenantID() string { return s.tenantID }

// ActorScope is a fully-scoped handle with (client, tenantID, actor) pre-bound.
type ActorScope struct {
	client   *DbClient
	tenantID string
	actor    Actor
}

// TenantID returns the tenant this scope is bound to.
func (s *ActorScope) TenantID() string { return s.tenantID }

// ActorID returns the actor this scope is bound to.
func (s *ActorScope) ActorID() Actor { return s.actor }

// Get retrieves a single node by type and ID.
func (s *ActorScope) Get(ctx context.Context, typeID int, nodeID string) (*Node, error) {
	return s.client.Get(ctx, s.tenantID, s.actor.String(), typeID, nodeID)
}

// Query retrieves nodes matching a filter.
func (s *ActorScope) Query(ctx context.Context, typeID int, filter map[string]any) ([]*Node, error) {
	return s.client.Query(ctx, s.tenantID, s.actor.String(), typeID, filter)
}

// Share grants permission on a node to another actor.
func (s *ActorScope) Share(ctx context.Context, nodeID string, grantee Actor, perm Permission) error {
	return s.client.transport.Share(ctx, s.tenantID, s.actor.String(), nodeID, grantee.String(), string(perm))
}

// Plan creates a new Plan pre-bound to this scope's tenant and actor.
func (s *ActorScope) Plan() *Plan {
	return s.client.NewPlan(s.tenantID, s.actor.String())
}
