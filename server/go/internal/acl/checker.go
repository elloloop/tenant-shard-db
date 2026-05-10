// SPDX-License-Identifier: AGPL-3.0-only

package acl

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

// NodeMeta is the minimal node metadata the Checker needs to evaluate
// a same-tenant grant: owner + type id. Returned by NodeMetaReader.
type NodeMeta struct {
	OwnerActor string // canonical "kind:id" form, e.g. "user:alice"
	TypeID     int32
}

// NodeMetaReader resolves a (tenant, node) to its owner + type id. The
// production binding wraps store.CanonicalStore.GetNode; tests provide
// an in-memory shim. NotFound is signalled by returning errs.ErrNotFound.
type NodeMetaReader interface {
	NodeMeta(ctx context.Context, tenantID, nodeID string) (NodeMeta, error)
}

// GrantReader returns the grants on a single node. It MUST include
// expired rows (the Checker drops them) so a future migration can
// surface "expired but not yet swept" rows as a metric. Cross-tenant
// callers can supply a CrossTenantGrantReader in addition.
type GrantReader interface {
	GrantsForNode(ctx context.Context, tenantID, nodeID string) ([]Grant, error)
}

// CrossTenantGrantReader is the cross-tenant slice. It is consulted
// only when the same-tenant Checker.Check fails AND the actor is
// foreign (different tenant than nodeTenant). Mirrors the
// global_store.shared_index lookup at grpc_server.py:338,1892-1933.
//
// Returns the typed permission for (foreignActor, sourceTenant, nodeID)
// or (PermDeny, false) when the row is absent. The Checker treats a
// hit as "allow READ"; richer cross-tenant typed-cap grants are a
// future concern (decision doc 2026-04-13).
type CrossTenantGrantReader interface {
	CrossTenantGrant(ctx context.Context, sourceTenant, nodeID, foreignActor string) (Permission, bool, error)
}

// Checker answers single-node authorization questions. Mirrors the
// per-handler _check_capability call site at
// server/python/entdb_server/api/grpc_server.py:299-360 plus the
// _sync_can_access core at canonical_store.py:2867-2946.
//
// A Checker is built once per server and shared across handlers; it
// holds no per-request state. All accessors are concurrency-safe to
// the extent that the underlying readers are.
type Checker struct {
	registry *Registry
	resolver *Resolver
	nodes    NodeMetaReader
	grants   GrantReader
	xt       CrossTenantGrantReader

	nowFn func() int64
}

// CheckerOptions configures a Checker. All non-nil readers are
// required for production wiring; tests can omit cross-tenant by
// leaving XT nil.
type CheckerOptions struct {
	Registry    *Registry
	Resolver    *Resolver
	Nodes       NodeMetaReader
	Grants      GrantReader
	CrossTenant CrossTenantGrantReader // optional
	NowFn       func() int64           // optional, defaults to nil (no expiry filtering)
}

// NewChecker constructs a Checker. Required: Registry, Resolver,
// Nodes, Grants. Returns an error if any required reader is nil.
func NewChecker(opts CheckerOptions) (*Checker, error) {
	if opts.Registry == nil {
		return nil, fmt.Errorf("acl: NewChecker: Registry is required")
	}
	if opts.Resolver == nil {
		return nil, fmt.Errorf("acl: NewChecker: Resolver is required")
	}
	if opts.Nodes == nil {
		return nil, fmt.Errorf("acl: NewChecker: Nodes reader is required")
	}
	if opts.Grants == nil {
		return nil, fmt.Errorf("acl: NewChecker: Grants reader is required")
	}
	return &Checker{
		registry: opts.Registry,
		resolver: opts.Resolver,
		nodes:    opts.Nodes,
		grants:   opts.Grants,
		xt:       opts.CrossTenant,
		nowFn:    opts.NowFn,
	}, nil
}

// CheckRequest is the input to Checker.Check. TenantID is the tenant
// the node lives in (the OWNER tenant, not necessarily the caller's).
// ActorTenantID is the caller's tenant — used to determine same-tenant
// vs cross-tenant. When equal to TenantID, only the same-tenant path
// runs; otherwise the cross-tenant fallback is consulted.
type CheckRequest struct {
	TenantID      string
	ActorTenantID string
	Actor         Actor
	NodeID        string
	OpName        string
	Field         string
	FieldValue    string
	ChildType     string
}

// Check returns nil if Actor has the capability required by OpName on
// NodeID, or errs.ErrPermission otherwise. NotFound from the underlying
// node lookup propagates unchanged.
//
// Algorithm (mirrors _check_capability + _sync_can_access):
//
//  1. System actors short-circuit allow.
//  2. Resolve the node's type_id and owner. Owner is always allowed.
//  3. Resolve the actor's group memberships (depth-bounded).
//  4. Look up RequiredForOp(typeID, op, ...). No requirement → allow.
//  5. Walk node grants:
//     a. If any DENY row matches the resolved actor set → deny.
//     b. If any allow row's typed caps satisfy the requirement → allow.
//  6. Same-tenant fall-through is a deny.
//  7. Cross-tenant: if ActorTenantID != TenantID and a CrossTenantGrantReader
//     is configured, ask it for an allow row keyed on the caller's
//     Actor (string form). A hit with READ-implying perm satisfies
//     CoreCapRead requirements only; ADMIN cross-tenant grants
//     satisfy everything via LegacyToCoreCaps + CheckGrant.
func (c *Checker) Check(ctx context.Context, req CheckRequest) error {
	if req.Actor.IsSystem() {
		return nil
	}
	if req.Actor.IsZero() || req.NodeID == "" {
		return errs.Errorf(codes.InvalidArgument, "acl: Check: actor and node_id required")
	}
	meta, err := c.nodes.NodeMeta(ctx, req.TenantID, req.NodeID)
	if err != nil {
		return err
	}

	// Resolve actor + groups, but only for same-tenant. Cross-tenant
	// callers don't (yet) get group-membership lookups against the
	// owner tenant.
	var resolved []Actor
	sameTenant := req.ActorTenantID == "" || req.ActorTenantID == req.TenantID
	if sameTenant {
		resolved, err = c.resolver.Expand(ctx, req.TenantID, req.Actor)
		if err != nil {
			return err
		}
	} else {
		resolved = []Actor{req.Actor, TenantWildcardForTenant(req.ActorTenantID)}
	}

	// Owner short-circuit (canonical_store.py:2920) — owner_actor in
	// the resolved set means allow. Compare via canonical string form.
	if meta.OwnerActor != "" {
		for _, a := range resolved {
			if a.String() == meta.OwnerActor {
				return nil
			}
		}
	}

	requiredCore, requiredExt := c.registry.RequiredForOp(meta.TypeID, req.OpName, OpQuery{
		Field:      req.Field,
		FieldValue: req.FieldValue,
		ChildType:  req.ChildType,
	})
	if requiredCore == CoreCapUnspecified && requiredExt == 0 {
		// No requirement registered — allow. This matches the Python
		// (None, None) → allow contract at capability_registry.py:267.
		return nil
	}

	grants, err := c.grants.GrantsForNode(ctx, req.TenantID, req.NodeID)
	if err != nil {
		return err
	}

	resolvedSet := map[string]struct{}{}
	for _, a := range resolved {
		resolvedSet[a.String()] = struct{}{}
	}
	// tenant:* matches any same-tenant caller (acl.py:127-129).
	if sameTenant {
		resolvedSet[TenantWildcard().String()] = struct{}{}
	}

	now := c.now()
	// First pass: explicit DENY wins (acl.py:243-264, canonical_store.py:2893).
	for _, g := range grants {
		if !g.IsDeny() {
			continue
		}
		if _, ok := resolvedSet[g.Subject.String()]; !ok {
			continue
		}
		if g.IsExpired(now) {
			continue
		}
		return errs.Errorf(codes.PermissionDenied,
			"acl: actor %s denied on node %s/%s by explicit DENY",
			req.Actor, req.TenantID, req.NodeID)
	}
	// Second pass: any matching grant whose typed caps satisfy req → allow.
	for _, g := range grants {
		if g.IsDeny() {
			continue
		}
		if g.IsExpired(now) {
			continue
		}
		if _, ok := resolvedSet[g.Subject.String()]; !ok {
			continue
		}
		grantCore := g.EffectiveCoreCaps()
		grantTypeID := g.TypeID
		if grantTypeID == 0 {
			grantTypeID = meta.TypeID
		}
		if c.registry.CheckGrant(grantCore, g.ExtCapIDs, requiredCore, requiredExt, grantTypeID) {
			return nil
		}
	}

	// Cross-tenant fallback.
	if !sameTenant && c.xt != nil {
		perm, ok, err := c.xt.CrossTenantGrant(ctx, req.TenantID, req.NodeID, req.Actor.String())
		if err != nil {
			return err
		}
		if ok {
			grantCore := LegacyToCoreCaps(perm)
			if c.registry.CheckGrant(grantCore, nil, requiredCore, requiredExt, meta.TypeID) {
				return nil
			}
		}
	}

	return errs.Errorf(codes.PermissionDenied,
		"acl: actor %s lacks %s on node %s/%s",
		req.Actor, capabilityLabel(requiredCore, requiredExt),
		req.TenantID, req.NodeID)
}

// TenantWildcardForTenant returns the cross-tenant grantee for a given
// tenant id. Per docs/decisions/acl.md (2026-04-13): a grant to
// tenant:<id> matches any authenticated caller from tenant <id>.
func TenantWildcardForTenant(tenantID string) Actor {
	return Actor{Kind: ActorKindTenant, ID: tenantID}
}

func capabilityLabel(core CoreCapability, ext ExtCapID) string {
	if core != CoreCapUnspecified {
		return core.String()
	}
	if ext != 0 {
		return fmt.Sprintf("ext:%d", ext)
	}
	return "ANY"
}

func (c *Checker) now() int64 {
	if c.nowFn == nil {
		return 0
	}
	return c.nowFn()
}
