package entdb

import (
	"context"
	"fmt"
	"sync/atomic"
)

// aliasCounter provides unique alias suffixes across all Plans in a process.
var aliasCounter atomic.Int64

// Plan collects mutation operations to be committed atomically.
//
// Operations are accumulated locally and sent to the server as a single
// atomic batch on Commit. A Plan cannot be reused after Commit.
type Plan struct {
	client         Transport
	tenantID       string
	actor          string
	idempotencyKey string
	operations     []Operation
	committed      bool
}

// newPlan creates a new Plan bound to a tenant and actor.
func newPlan(client Transport, tenantID, actor, idempotencyKey string) *Plan {
	return &Plan{
		client:         client,
		tenantID:       tenantID,
		actor:          actor,
		idempotencyKey: idempotencyKey,
		operations:     make([]Operation, 0),
	}
}

// Create adds a create-node operation and returns a ref alias that can be
// used in subsequent operations (e.g., as edge endpoints).
func (p *Plan) Create(typeID int, data map[string]any) string {
	p.ensureNotCommitted()
	alias := fmt.Sprintf("_ref_%d", aliasCounter.Add(1))
	p.operations = append(p.operations, Operation{
		Type:   OpCreateNode,
		TypeID: typeID,
		Alias:  alias,
		Data:   data,
	})
	return alias
}

// CreateWithACL adds a create-node operation with explicit ACL entries.
func (p *Plan) CreateWithACL(typeID int, data map[string]any, acl []ACLEntry) string {
	p.ensureNotCommitted()
	alias := fmt.Sprintf("_ref_%d", aliasCounter.Add(1))
	p.operations = append(p.operations, Operation{
		Type:   OpCreateNode,
		TypeID: typeID,
		Alias:  alias,
		Data:   data,
		ACL:    acl,
	})
	return alias
}

// Update adds an update-node operation.
func (p *Plan) Update(nodeID string, typeID int, patch map[string]any) {
	p.ensureNotCommitted()
	p.operations = append(p.operations, Operation{
		Type:   OpUpdateNode,
		TypeID: typeID,
		NodeID: nodeID,
		Patch:  patch,
	})
}

// Delete adds a delete-node operation.
func (p *Plan) Delete(nodeID string) {
	p.ensureNotCommitted()
	p.operations = append(p.operations, Operation{
		Type:   OpDeleteNode,
		NodeID: nodeID,
	})
}

// CreateEdge adds a create-edge operation.
func (p *Plan) CreateEdge(edgeTypeID int, from, to string) {
	p.ensureNotCommitted()
	p.operations = append(p.operations, Operation{
		Type:       OpCreateEdge,
		EdgeTypeID: edgeTypeID,
		FromNodeID: from,
		ToNodeID:   to,
	})
}

// DeleteEdge adds a delete-edge operation.
func (p *Plan) DeleteEdge(edgeTypeID int, from, to string) {
	p.ensureNotCommitted()
	p.operations = append(p.operations, Operation{
		Type:       OpDeleteEdge,
		EdgeTypeID: edgeTypeID,
		FromNodeID: from,
		ToNodeID:   to,
	})
}

// Operations returns a copy of the accumulated operations (for inspection/testing).
func (p *Plan) Operations() []Operation {
	out := make([]Operation, len(p.operations))
	copy(out, p.operations)
	return out
}

// TenantID returns the tenant this plan is bound to.
func (p *Plan) TenantID() string { return p.tenantID }

// Actor returns the actor this plan is bound to.
func (p *Plan) Actor() string { return p.actor }

// IdempotencyKey returns the idempotency key for this plan.
func (p *Plan) IdempotencyKey() string { return p.idempotencyKey }

// Commit sends all accumulated operations to the server as a single atomic
// transaction. The Plan cannot be used after Commit is called.
func (p *Plan) Commit(ctx context.Context) (*CommitResult, error) {
	p.ensureNotCommitted()
	p.committed = true
	return p.client.ExecuteAtomic(ctx, p.tenantID, p.actor, p.idempotencyKey, p.operations)
}

// ensureNotCommitted panics if the Plan has already been committed.
// This mirrors the Python SDK's behaviour of raising RuntimeError.
func (p *Plan) ensureNotCommitted() {
	if p.committed {
		panic("entdb: Plan has already been committed; create a new Plan for additional operations")
	}
}
