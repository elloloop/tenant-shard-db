// SPDX-License-Identifier: MIT
package entdb

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

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
		NodeID:       cfg.id,
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

// UpdateFields accumulates an update-node operation for an EXPLICIT set
// of fields, including fields whose value is the proto3 zero value
// (false / 0 / ""). Use it when you need to set a field BACK to its zero
// value — the default [Plan.Update] sends only non-default fields (proto3
// cannot tell "set to zero" from "unset" for scalars), so a true→false /
// non-empty→"" / n→0 update is otherwise silently dropped (#574).
//
//	// clear the verified flag (Update would drop the false)
//	plan.UpdateFields("node-1", &auth.TotpCredential{}, "verified")
//
// field names are proto field names (the SDK resolves them to field ids).
// Naming a field reads its current value off msg — set the ones you want
// to change on msg first, then list them here. At least one field is
// required; an unknown field name panics at call time.
func (p *Plan) UpdateFields(nodeID string, msg proto.Message, fields ...string) {
	p.ensureNotCommitted()
	typeID, patch, err := marshalExplicitFieldsForWire(msg, fields)
	if err != nil {
		panic(fmt.Errorf("entdb: Plan.UpdateFields: %w", err))
	}
	p.operations = append(p.operations, Operation{
		Type:   OpUpdateNode,
		TypeID: int(typeID),
		NodeID: nodeID,
		Patch:  patch,
	})
}

// UpdateIf accumulates a conditional update-node operation with a
// single-field equality precondition (CAS). The applier compares the
// node's current “field“ against “equals“ before applying the
// patch; on mismatch the whole batch aborts and Commit returns a
// [*PreconditionFailure] error matching [ErrPreconditionFailed].
//
// Use this for state-machine transitions and single-use tokens —
// anywhere two concurrent writers must not both win. See GitHub
// issue #500 for the design.
//
// The idempotency cache memoizes both success and CAS-miss outcomes,
// so a retry with the SAME idempotency key returns the cached
// result without re-evaluating. To re-evaluate against current
// state, mint a new idempotency key.
func (p *Plan) UpdateIf(nodeID string, msg proto.Message, field string, equals any) {
	p.ensureNotCommitted()
	typeID, patch, err := marshalSetFieldsForWire(msg)
	if err != nil {
		panic(fmt.Errorf("entdb: Plan.UpdateIf: %w", err))
	}
	fieldID, err := fieldIDFromMessage(msg, field)
	if err != nil {
		panic(fmt.Errorf("entdb: Plan.UpdateIf: %w", err))
	}
	p.operations = append(p.operations, Operation{
		Type:         OpUpdateNode,
		TypeID:       int(typeID),
		NodeID:       nodeID,
		Patch:        patch,
		Precondition: &Precondition{Field: field, FieldID: int(fieldID), Equals: equals},
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

// DeleteWhere accumulates a single-RPC predicate-based sweeper op
// (GitHub issue #504). It deletes every node of type T whose payload
// matches ALL of the AND-ed [Filter] predicates, capped best-effort
// by limit, in one round trip — collapsing the QueryWhere + Delete
// loop the TTL-sweeper pattern otherwise needs.
//
// Like [Delete], DeleteWhere is a free function (Go has no generic
// methods) and T is a compile-time type witness: the SDK reads
// "(entdb.node).type_id" from T's descriptor without a message
// instance. The Filter field names are resolved to payload field ids
// server-side, exactly like [QueryWhere], so no client registry is
// needed for the predicate.
//
// limit is best-effort (Postgres DELETE … LIMIT semantics): at most
// limit nodes are deleted by this op. Pass 0 for the server default;
// the server clamps to its own hard ceiling so a runaway predicate
// cannot pin the single applier goroutine for a tenant. To drain a
// large backlog, sweep in a loop until a sweep deletes nothing.
//
// At least one filter is required — an unconditional bulk delete is
// rejected server-side. Passing T with no "(entdb.node)" option
// panics at call time.
//
//	entdb.DeleteWhere[*auth.WebAuthnChallenge](plan,
//	    []entdb.Filter{{Field: "expires_at", Op: entdb.FilterLt, Value: now}},
//	    1000,
//	)
//
// Schema-less servers (issue #545): a server started without a schema
// cannot resolve a field NAME to a payload field id. Pass the numeric
// payload field id as the [Filter].Field string instead — exactly the
// schema-optional escape hatch QueryWhere already accepts for numeric
// filter keys. The SDK forwards Field verbatim; the server treats a
// digit-only key as a raw field id with no schema lookup:
//
//	// "expires_at" is proto field 4 on the caller's own schema.
//	entdb.DeleteWhere[*auth.WebAuthnChallenge](plan,
//	    []entdb.Filter{{Field: "4", Op: entdb.FilterLt, Value: now}},
//	    1000,
//	)
//
// Against a schema-configured server both forms work; against a
// schema-less server only the numeric-id form does (a name key
// returns INVALID_ARGUMENT: "cannot translate filter key … without a
// schema").
func DeleteWhere[T proto.Message](p *Plan, where []Filter, limit int) {
	p.ensureNotCommitted()
	msg := newZeroMessage[T]()
	typeID, err := typeIDFromMessage(msg)
	if err != nil {
		panic(fmt.Errorf("entdb: DeleteWhere: %w", err))
	}
	p.operations = append(p.operations, Operation{
		Type:   OpDeleteWhere,
		TypeID: int(typeID),
		// NAME-FREE (ADR-031): resolve filter field names to decimal
		// field_ids from T's descriptor before the WAL ever sees the
		// predicate (the server rejects a non-digit FieldFilter.field).
		Where: resolveFilterFields(msg, where),
		Limit: limit,
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

// commitOpts is the resolved options pack for a single Commit /
// ExecuteAtomic call. Callers configure it via CommitOption values.
type commitOpts struct {
	// waitApplied, when non-nil, OVERRIDES the auto-default
	// (precondition-driven). nil = auto-default.
	waitApplied   *bool
	waitTimeoutMs int32
}

// CommitOption configures a single Plan.Commit / ExecuteAtomic call.
// Pass values to [Plan.Commit] as variadic args:
//
//	receipt, err := plan.Commit(ctx, entdb.WithWaitApplied(true))
//
// Options are independent and can be combined. Issue #606.
type CommitOption func(*commitOpts)

// WithWaitApplied controls whether [Plan.Commit] blocks until the WAL
// applier has processed every op in the batch BEFORE returning. When
// true, the applier's outcome (success, ALREADY_EXISTS on a unique-
// constraint violation, FAILED_PRECONDITION on a CAS miss, …) is
// surfaced synchronously. When false (the default), the call returns
// as soon as the WAL has accepted the event, and an applier-side
// rejection only shows up via a follow-up read.
//
// Prior to issue #606 the Plan API auto-enabled this ONLY for ops
// carrying a Precondition. For unique-constraint races (no
// precondition) the LOSER otherwise got a phantom success: a UUID
// returned by Commit, then a silent ALREADY_EXISTS at apply time.
// Set this explicitly to true on writes whose uniqueness outcome the
// caller needs to react to.
func WithWaitApplied(b bool) CommitOption {
	return func(o *commitOpts) { o.waitApplied = &b }
}

// WithWaitTimeout sets the upper bound on how long the server will
// block waiting for the applier when WaitApplied is true. Zero (the
// default) selects the SDK's default of 30s. Values are clamped to
// >= 1ms.
func WithWaitTimeout(d time.Duration) CommitOption {
	return func(o *commitOpts) {
		ms := d / time.Millisecond
		if ms > 0 {
			o.waitTimeoutMs = int32(ms)
		}
	}
}

// Commit sends all accumulated operations to the server as a single atomic
// transaction. The Plan cannot be used after Commit is called.
//
// Pass [WithWaitApplied] / [WithWaitTimeout] to block until the applier
// has processed the batch — useful when racing on a unique constraint
// where the loser otherwise sees a phantom success (issue #606).
func (p *Plan) Commit(ctx context.Context, opts ...CommitOption) (*CommitResult, error) {
	p.ensureNotCommitted()
	p.committed = true
	return p.client.ExecuteAtomic(ctx, p.tenantID, p.actor, p.idempotencyKey, p.operations, opts...)
}

// ensureNotCommitted panics if the Plan has already been committed.
// This mirrors the Python SDK's behaviour of raising RuntimeError.
func (p *Plan) ensureNotCommitted() {
	if p.committed {
		panic("entdb: Plan has already been committed; create a new Plan for additional operations")
	}
}
