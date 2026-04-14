package entdb

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"

	"google.golang.org/protobuf/proto"
)

// aliasCounter provides unique alias suffixes across all Plans in a process.
var aliasCounter atomic.Int64

// nextAlias returns a fresh internal alias like "_ref_42".
func nextAlias() string {
	return fmt.Sprintf("_ref_%d", aliasCounter.Add(1))
}

// Plan collects mutation operations to be committed atomically.
//
// The 2026-04-14 SDK v0.3 API has exactly one way to perform each
// mutation: [Plan.Create] for nodes (options go through
// [CreateOption]), [Plan.Update] for partial updates, and the free
// generic functions [Delete], [EdgeCreate], [EdgeDelete] for the
// ops that need a compile-time type witness without a message
// instance.
//
// Go does not support generic methods, so the three type-witness
// ops are package-level functions that take the *Plan as their
// first argument:
//
//	plan := scope.Plan()
//	plan.Create(&shop.Product{Sku: "WIDGET-1", Name: "Widget"})
//	plan.Update("node-42", &shop.Product{PriceCents: 1999})
//	entdb.Delete[*shop.Product](plan, "node-old")
//	entdb.EdgeCreate[*shop.PurchaseEdge](plan, "user-1", "node-42")
//	entdb.EdgeDelete[*shop.PurchaseEdge](plan, "user-1", "node-42")
//	_, err := plan.Commit(ctx)
//
// A Plan cannot be reused after Commit.
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

// Create accumulates a create-node operation.
//
// The type_id is read from the message's “(entdb.node).type_id“
// proto option; field values are encoded by proto field id. Returns
// the alias usable as an edge endpoint later in the same plan.
//
// Options configure storage mode, ACL, and the alias string:
//
//	alias := plan.Create(&shop.Product{Sku: "WIDGET-1"},
//	    entdb.WithACL(entdb.ACLEntry{Grantee: entdb.UserActor("bob"),
//	                                 Permission: entdb.PermissionRead}),
//	    entdb.As("first-product"),
//	)
//
// If “msg“ is not an EntDB node type (no “(entdb.node)“ option)
// Create panics — the failure is at compile-time-adjacent developer
// speed, surfaced immediately in unit tests.
func (p *Plan) Create(msg proto.Message, opts ...CreateOption) string {
	p.ensureNotCommitted()
	cfg := createConfig{
		storage: StorageModeTenant,
		alias:   nextAlias(),
	}
	for _, o := range opts {
		o(&cfg)
	}
	typeID, payload, err := marshalForWire(msg)
	if err != nil {
		panic(fmt.Errorf("entdb: Plan.Create: %w", err))
	}
	p.operations = append(p.operations, Operation{
		Type:         OpCreateNode,
		TypeID:       int(typeID),
		Alias:        cfg.alias,
		Data:         payload,
		ACL:          cfg.acl,
		StorageMode:  cfg.storage,
		TargetUserID: cfg.targetUserID,
	})
	return cfg.alias
}

// Update accumulates an update-node operation.
//
// Only fields set on “msg“ are sent: proto3 “Range“ iterates
// only non-default scalars and explicitly-set “optional“ fields
// and submessages, which matches the intuitive "patch = fields I
// actually touched" semantics. The type_id is read from the
// message's “(entdb.node)“ option — callers do not pass it
// separately.
func (p *Plan) Update(nodeID string, msg proto.Message) {
	p.ensureNotCommitted()
	typeID, patch, err := marshalSetFieldsForWire(msg)
	if err != nil {
		panic(fmt.Errorf("entdb: Plan.Update: %w", err))
	}
	p.operations = append(p.operations, Operation{
		Type:   OpUpdateNode,
		TypeID: int(typeID),
		NodeID: nodeID,
		Patch:  patch,
	})
}

// Delete accumulates a delete-node operation.
//
// Delete is a free function rather than a method on Plan because
// Go does not support generic methods. The type parameter T is a
// compile-time witness — the SDK reads the “(entdb.node).type_id“
// from T's descriptor without needing a message instance:
//
//	entdb.Delete[*shop.Product](plan, "node-42")
//
// Passing a non-pointer or non-proto T is a compile-time error;
// passing a T with no “(entdb.node)“ option panics at call time.
func Delete[T proto.Message](p *Plan, nodeID string) {
	p.ensureNotCommitted()
	msg := newZeroMessage[T]()
	typeID, err := typeIDFromMessage(msg)
	if err != nil {
		panic(fmt.Errorf("entdb: Delete: %w", err))
	}
	p.operations = append(p.operations, Operation{
		Type:   OpDeleteNode,
		TypeID: int(typeID),
		NodeID: nodeID,
	})
}

// EdgeCreate accumulates a create-edge operation.
//
// The type parameter T is an edge proto message type (annotated
// with “(entdb.edge)“), used purely as a compile-time witness
// for the edge_id:
//
//	entdb.EdgeCreate[*shop.PurchaseEdge](plan, "user-1", "product-42")
func EdgeCreate[T proto.Message](p *Plan, from, to string) {
	p.ensureNotCommitted()
	msg := newZeroMessage[T]()
	edgeID, err := edgeIDFromMessage(msg)
	if err != nil {
		panic(fmt.Errorf("entdb: EdgeCreate: %w", err))
	}
	p.operations = append(p.operations, Operation{
		Type:       OpCreateEdge,
		EdgeTypeID: int(edgeID),
		FromNodeID: from,
		ToNodeID:   to,
	})
}

// EdgeDelete accumulates a delete-edge operation. See [EdgeCreate]
// for the type-witness pattern.
func EdgeDelete[T proto.Message](p *Plan, from, to string) {
	p.ensureNotCommitted()
	msg := newZeroMessage[T]()
	edgeID, err := edgeIDFromMessage(msg)
	if err != nil {
		panic(fmt.Errorf("entdb: EdgeDelete: %w", err))
	}
	p.operations = append(p.operations, Operation{
		Type:       OpDeleteEdge,
		EdgeTypeID: int(edgeID),
		FromNodeID: from,
		ToNodeID:   to,
	})
}

// newZeroMessage builds a fresh, writable instance of a proto
// message type T. The caller passes T as a compile-time witness;
// this helper handles the "nil pointer receiver is safe for
// ProtoReflect but not for actual use" dance that T-parameterised
// code otherwise has to repeat at every call site.
//
// For generated proto types (where T is “*ConcreteStruct“ and
// calling ProtoReflect on a nil pointer returns a valid package-
// level descriptor), “var zero T“ is enough to read the
// descriptor — but the returned writable instance from
// “.New().Interface()“ has a different concrete Go type than T
// (e.g. “*dynamicpb.Message“), so we use reflection to construct
// a fresh “new(Elem)“ instance of T's pointed-to type.
func newZeroMessage[T proto.Message]() T {
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil || t.Kind() != reflect.Ptr {
		// Non-pointer proto message types don't exist in
		// practice, but fall back to the
		// ProtoReflect().New() path for completeness.
		return zero.ProtoReflect().New().Interface().(T)
	}
	return reflect.New(t.Elem()).Interface().(T)
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
